// Pacote execucoes registra as execuções das automações (data-model.md › Execução): transições
// de estado, ações registradas (≤ 200), log limitado a 64 KB, tokens por modelo, retenção de 500
// por automação e erros seguidos (5 → desativa a automação e notifica). Publica
// automacao.execucao.{iniciada,atualizada,finalizada} (atualizada no máx. 1/s por execução,
// exceto mudança de estado).
package execucoes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/relogio"
)

// Limites.
const (
	LimiteLog          = 64 << 10
	LimiteAcoes        = 200
	LimiteRetorno      = 64 << 10
	ErrosParaDesativar = 5
	IntervaloPublicar  = time.Second
)

// Registro cria e persiste execuções.
type Registro struct {
	banco      *armazenamento.Banco
	barramento *eventos.Barramento
	relogio    relogio.Relogio

	mu          sync.Mutex
	aoDesativar func(ctx context.Context, automacaoID string)
}

// Novo cria o registro.
func Novo(b *armazenamento.Banco, bar *eventos.Barramento, r relogio.Relogio) *Registro {
	return &Registro{banco: b, barramento: bar, relogio: r}
}

// AoDesativar registra quem publica a automação desativada por erros seguidos.
func (r *Registro) AoDesativar(fn func(ctx context.Context, automacaoID string)) {
	r.mu.Lock()
	r.aoDesativar = fn
	r.mu.Unlock()
}

// Alvo da execução.
type Alvo struct {
	ContaID, ConversaID, ContatoID, LeadID string
}

// Nova descreve uma execução a criar.
type Nova struct {
	Automacao        dominio.Automacao
	Gatilho          dominio.GatilhoExecucao
	Origem           string
	OrigemExecucaoID string
	Cadeia           []string
	Alvo             Alvo
	Simulacao        bool
	Variaveis        map[string]any
}

// Exec é uma execução em andamento (seguro para uso concorrente).
type Exec struct {
	r           *Registro
	mu          sync.Mutex
	d           dominio.ExecucaoDetalhe
	ultimaPub   time.Time
	inicioAtivo time.Time
	ativoMs     int64
	avisouAcoes bool
}

func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Criar grava a execução em "na_fila".
func (r *Registro) Criar(ctx context.Context, n Nova) (*Exec, error) {
	agora := r.relogio.Agora()
	gat := n.Gatilho
	if gat.Dados == nil {
		gat.Dados = map[string]any{}
	}
	vars := n.Variaveis
	if vars == nil {
		vars = map[string]any{}
	}
	d := dominio.ExecucaoDetalhe{Execucao: dominio.Execucao{
		ID: ids.NovoEm(agora), AutomacaoID: n.Automacao.ID, AutomacaoNome: n.Automacao.Nome, TipoAutomacao: n.Automacao.Tipo,
		AutomacaoVersao: n.Automacao.Versao, Gatilho: gat, Origem: n.Origem, OrigemExecucaoID: ptr(n.OrigemExecucaoID),
		ContaID: ptr(n.Alvo.ContaID), ConversaID: ptr(n.Alvo.ConversaID), ContatoID: ptr(n.Alvo.ContatoID), LeadID: ptr(n.Alvo.LeadID),
		Estado: dominio.ExecNaFila, Simulacao: n.Simulacao, Acoes: []dominio.AcaoRegistrada{},
		Tokens: dominio.Tokens{PorModelo: map[string]dominio.TokensModelo{}}, Retorno: json.RawMessage("null"), IniciadaEm: agora,
		Cadeia: append([]string{}, n.Cadeia...)}, Variaveis: vars}
	if err := armazenamento.InserirExecucao(ctx, r.banco.E(), d); err != nil {
		return nil, err
	}
	return &Exec{r: r, d: d}, nil
}

// Carregar reabre uma execução gravada (retomada de espera).
func (r *Registro) Carregar(ctx context.Context, id string) (*Exec, error) {
	d, err := armazenamento.ObterExecucao(ctx, r.banco.L(), id)
	if err != nil {
		return nil, err
	}
	var ms int64
	if d.DuracaoMs != nil {
		ms = *d.DuracaoMs
	}
	return &Exec{r: r, d: d, ativoMs: ms}, nil
}

// ID da execução.
func (e *Exec) ID() string { return e.d.ID }

// Detalhe devolve uma cópia do estado atual.
func (e *Exec) Detalhe() dominio.ExecucaoDetalhe {
	e.mu.Lock()
	defer e.mu.Unlock()
	return copiar(e.d)
}

func copiar(d dominio.ExecucaoDetalhe) dominio.ExecucaoDetalhe {
	c := d
	c.Acoes = append([]dominio.AcaoRegistrada{}, d.Acoes...)
	c.Variaveis = map[string]any{}
	for k, v := range d.Variaveis {
		c.Variaveis[k] = v
	}
	c.Tokens.PorModelo = map[string]dominio.TokensModelo{}
	for k, v := range d.Tokens.PorModelo {
		c.Tokens.PorModelo[k] = v
	}
	return c
}

// Estado atual.
func (e *Exec) Estado() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.d.Estado
}

// Simulacao indica se é teste.
func (e *Exec) Simulacao() bool { return e.d.Simulacao }

func (e *Exec) contaSemTrava() string {
	if e.d.ContaID != nil {
		return *e.d.ContaID
	}
	return ""
}

// salvarEPublicar persiste e publica (tipo vazio = atualizada com limite de 1/s).
func (e *Exec) salvarEPublicar(ctx context.Context, tipo string, forcar bool) error {
	e.d.DuracaoMs = e.duracaoSemTrava()
	if err := armazenamento.GravarExecucao(ctx, e.r.banco.E(), e.d); err != nil {
		return err
	}
	agora := time.Now()
	if tipo == "" {
		if !forcar && agora.Sub(e.ultimaPub) < IntervaloPublicar {
			return nil
		}
		tipo = eventos.AutomacaoExecucaoAtualizada
	}
	e.ultimaPub = agora
	e.r.barramento.Publicar(tipo, e.contaSemTrava(), copiar(e.d).Execucao)
	return nil
}

func (e *Exec) duracaoSemTrava() *int64 {
	ms := e.ativoMs
	if !e.inicioAtivo.IsZero() {
		ms += e.r.relogio.Agora().Sub(e.inicioAtivo).Milliseconds()
	}
	if ms < 0 {
		ms = 0
	}
	return &ms
}

// Iniciar passa a "rodando" (publica iniciada).
func (e *Exec) Iniciar(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.d.Estado = dominio.ExecRodando
	e.d.RetomarEm = nil
	e.inicioAtivo = e.r.relogio.Agora()
	return e.salvarEPublicar(ctx, eventos.AutomacaoExecucaoIniciada, true)
}

// Retomar volta de "aguardando" para "rodando".
func (e *Exec) Retomar(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.d.Estado = dominio.ExecRodando
	e.d.RetomarEm = nil
	e.inicioAtivo = e.r.relogio.Agora()
	return e.salvarEPublicar(ctx, "", true)
}

// Aguardar grava "aguardando" com a hora de retomada (a espera é gravada por quem chama).
func (e *Exec) Aguardar(ctx context.Context, retomarEm time.Time, proximoPasso int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.inicioAtivo.IsZero() {
		e.ativoMs += e.r.relogio.Agora().Sub(e.inicioAtivo).Milliseconds()
		e.inicioAtivo = time.Time{}
	}
	e.d.Estado = dominio.ExecAguardando
	r := retomarEm
	e.d.RetomarEm = &r
	p := proximoPasso
	e.d.PassoAtual = &p
	return e.salvarEPublicar(ctx, "", true)
}

// DefinirPasso persiste o índice da próxima ação (antes de executá-la).
func (e *Exec) DefinirPasso(ctx context.Context, passo int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	p := passo
	e.d.PassoAtual = &p
	return armazenamento.GravarExecucao(ctx, e.r.banco.E(), e.d)
}

// Passo devolve o índice gravado (0 se nenhum).
func (e *Exec) Passo() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.d.PassoAtual == nil {
		return 0
	}
	return *e.d.PassoAtual
}

// RegistrarAcao acrescenta uma ação (máx. 200; as seguintes só aparecem no log).
func (e *Exec) RegistrarAcao(ctx context.Context, a dominio.AcaoRegistrada) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if a.Em.IsZero() {
		a.Em = e.r.relogio.Agora()
	}
	if len(e.d.Acoes) >= LimiteAcoes {
		if !e.avisouAcoes {
			e.avisouAcoes = true
			e.logarSemTrava("aviso", fmt.Sprintf("Mais de %d ações: as seguintes não foram registradas.", LimiteAcoes))
		}
		return
	}
	e.d.Acoes = append(e.d.Acoes, a)
	e.salvarEPublicar(ctx, "", false)
}

// Logar acrescenta uma linha ao log (≤ 64 KB; o excedente é descartado e marca log_truncado).
func (e *Exec) Logar(nivel, texto string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.logarSemTrava(nivel, texto)
}

func (e *Exec) logarSemTrava(nivel, texto string) {
	if e.d.LogTruncado {
		return
	}
	linha := fmt.Sprintf("%s [%s] %s\n", e.r.relogio.Agora().Format("15:04:05"), nivel, texto)
	if len(e.d.Log)+len(linha) > LimiteLog {
		resto := LimiteLog - len(e.d.Log)
		if resto > 0 {
			corte := linha[:resto]
			for !utf8.ValidString(corte) && len(corte) > 0 {
				corte = corte[:len(corte)-1]
			}
			e.d.Log += corte
		}
		e.d.LogTruncado = true
		return
	}
	e.d.Log += linha
}

// SalvarLog persiste o log (chamado periodicamente por quem loga muito).
func (e *Exec) SalvarLog(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.salvarEPublicar(ctx, "", false)
}

// SomarTokens de uma chamada de IA.
func (e *Exec) SomarTokens(modelo string, entrada, saida int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.d.Tokens.Somar(modelo, entrada, saida)
}

// DefinirVariavel grava uma variável da execução.
func (e *Exec) DefinirVariavel(ctx context.Context, nome string, valor any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.d.Variaveis == nil {
		e.d.Variaveis = map[string]any{}
	}
	e.d.Variaveis[nome] = valor
	armazenamento.GravarExecucao(ctx, e.r.banco.E(), e.d)
}

// Variaveis devolve cópia das variáveis como texto (strings e JSON do resto).
func (e *Exec) Variaveis() map[string]string {
	e.mu.Lock()
	defer e.mu.Unlock()
	m := map[string]string{}
	for k, v := range e.d.Variaveis {
		switch x := v.(type) {
		case string:
			m[k] = x
		default:
			b, _ := json.Marshal(x)
			m[k] = string(b)
		}
	}
	return m
}

// DefinirAlvo completa o alvo (ex.: conversa aberta pela ação).
func (e *Exec) DefinirAlvo(ctx context.Context, a Alvo) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if a.ContaID != "" {
		e.d.ContaID = ptr(a.ContaID)
	}
	if a.ConversaID != "" {
		e.d.ConversaID = ptr(a.ConversaID)
	}
	if a.ContatoID != "" {
		e.d.ContatoID = ptr(a.ContatoID)
	}
	if a.LeadID != "" {
		e.d.LeadID = ptr(a.LeadID)
	}
	armazenamento.GravarExecucao(ctx, e.r.banco.E(), e.d)
}

// Fim descreve o término.
type Fim struct {
	Estado    string // ok | erro | simulacao | abortada
	Erro      string
	ErroStack string
	Motivo    string
	Retorno   json.RawMessage
}

// Finalizar grava o estado final, publica finalizada e aplica a regra de erros seguidos
// (execuções reais: erro soma; ok zera; 5 seguidos desativam a automação e notificam).
func (e *Exec) Finalizar(ctx context.Context, f Fim) error {
	e.mu.Lock()
	if dominio.FinalExec(e.d.Estado) {
		e.mu.Unlock()
		return nil
	}
	agora := e.r.relogio.Agora()
	if !e.inicioAtivo.IsZero() {
		e.ativoMs += agora.Sub(e.inicioAtivo).Milliseconds()
		e.inicioAtivo = time.Time{}
	}
	e.d.Estado = f.Estado
	e.d.Erro, e.d.ErroStack, e.d.Motivo = ptr(f.Erro), ptr(f.ErroStack), ptr(f.Motivo)
	if len(f.Retorno) > 0 {
		if len(f.Retorno) > LimiteRetorno {
			e.d.Retorno = json.RawMessage("null")
			e.logarSemTrava("aviso", "Retorno maior que 64 KB descartado.")
		} else {
			e.d.Retorno = f.Retorno
		}
	}
	e.d.RetomarEm = nil
	e.d.FinalizadaEm = &agora
	err := e.salvarEPublicar(ctx, eventos.AutomacaoExecucaoFinalizada, true)
	d := e.d
	e.mu.Unlock()
	if err != nil || d.Simulacao {
		return err
	}
	switch f.Estado {
	case dominio.ExecOK:
		_, err = armazenamento.RegistrarResultadoErros(ctx, e.r.banco.E(), d.AutomacaoID, false)
	case dominio.ExecErro:
		var n int
		n, err = armazenamento.RegistrarResultadoErros(ctx, e.r.banco.E(), d.AutomacaoID, true)
		if err == nil && n >= ErrosParaDesativar {
			e.r.desativar(ctx, d)
		}
	}
	return err
}

func (r *Registro) desativar(ctx context.Context, d dominio.ExecucaoDetalhe) {
	ok, err := armazenamento.DesativarAutomacao(ctx, r.banco.E(), d.AutomacaoID, "erros_seguidos", r.relogio.Agora())
	if err != nil || !ok {
		return
	}
	aut := d.AutomacaoID
	conta := ""
	if d.ContaID != nil {
		conta = *d.ContaID
	}
	r.barramento.Publicar(eventos.Notificacao, conta, dominio.Notificacao{
		Titulo: "Automação desativada",
		Corpo: fmt.Sprintf("\"%s\" falhou %d vezes seguidas e foi desativada. Veja as execuções para o motivo.",
			d.AutomacaoNome, ErrosParaDesativar),
		AutomacaoID: &aut, Tipo: "desativada"})
	r.mu.Lock()
	fn := r.aoDesativar
	r.mu.Unlock()
	if fn != nil {
		fn(ctx, d.AutomacaoID)
	}
}

// AbortarPendentes marca como "abortada" (motivo dado) as execuções nos estados informados —
// usado na subida ("interrompida") e no encerramento ("app fechado").
func (r *Registro) AbortarPendentes(ctx context.Context, motivo string, estados ...string) (int, error) {
	idsL, err := armazenamento.ExecucoesNosEstados(ctx, r.banco.L(), estados...)
	if err != nil {
		return 0, err
	}
	for _, id := range idsL {
		e, err := r.Carregar(ctx, id)
		if err != nil {
			continue
		}
		e.Finalizar(ctx, Fim{Estado: dominio.ExecAbortada, Motivo: motivo})
	}
	return len(idsL), nil
}

// Resumo de texto curto para AcaoRegistrada.detalhe.
func Resumo(s string, max int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max-1]) + "…"
}
