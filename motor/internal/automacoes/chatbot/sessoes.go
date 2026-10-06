package chatbot

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/automacoes/acoes"
	"zapdesk/motor/internal/automacoes/condicoes"
	"zapdesk/motor/internal/automacoes/execucoes"
	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/automacoes/seguranca"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/fatos"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/relogio"
)

// Enfileirador põe tarefas na fila do despachante (ordem por conversa).
type Enfileirador interface {
	Enfileirar(chave string, t func(ctx context.Context))
}

// Esperas grava a espera de expiração da sessão.
type Esperas interface {
	Gravar(ctx context.Context, e dominio.Espera) error
}

// Deps do serviço de sessões.
type Deps struct {
	Banco      *armazenamento.Banco
	Barramento *eventos.Barramento
	Relogio    relogio.Relogio
	Log        zerolog.Logger
	Registro   *execucoes.Registro
	Acoes      *acoes.Executores
	Pausas     *seguranca.Pausas
	Fila       Enfileirador
	Esperas    Esperas
	IA         acoes.IA
}

// Servico de sessões de chatbot.
type Servico struct {
	d           Deps
	mu          sync.Mutex
	processando map[string]bool // conversas com rodada em andamento
}

// Novo cria o serviço (e se liga às pausas: atendimento humano encerra a sessão).
func Novo(d Deps) *Servico {
	s := &Servico{d: d, processando: map[string]bool{}}
	d.Pausas.AoAtendimentoHumano(func(ctx context.Context, conversaID, motivo string) {
		s.mu.Lock()
		ocupada := s.processando[conversaID]
		s.mu.Unlock()
		if !ocupada {
			s.EncerrarDaConversa(ctx, conversaID, dominio.SessaoHumano, "atendimento humano")
		}
	})
	return s
}

// DefinirIA liga o serviço de IA (nó ia).
func (s *Servico) DefinirIA(ia acoes.IA) { s.d.IA = ia }

func (s *Servico) publicar(tipo string, se dominio.SessaoChatbot) {
	s.d.Barramento.Publicar(tipo, se.ContaID, se)
}

// Chave da fila das rodadas de chatbot de uma conversa.
func Chave(conversaID string) string { return "chatbot|c:" + conversaID }

// ---------------------------------------------------------------------------
// Efeitos reais
// ---------------------------------------------------------------------------

type efeitos struct {
	s    *Servico
	c    *acoes.Contexto
	nome string
}

func (e *efeitos) comVars(vars map[string]string) *acoes.Contexto {
	e.c.Variaveis = vars
	return e.c
}

func (e *efeitos) registrar(ctx context.Context, r acoes.Resultado, noID string) {
	if r.Registro.Alvo == nil && noID != "" {
		a := "nó " + noID
		r.Registro.Alvo = &a
	}
	if e.c.Exec != nil {
		e.c.Exec.RegistrarAcao(ctx, r.Registro)
	}
}

func (e *efeitos) Enviar(ctx context.Context, noID, texto string, tpl *string) Envio {
	var r acoes.Resultado
	if tpl != nil {
		r = e.s.d.Acoes.Executar(ctx, e.c, modelo.Acao{Tipo: modelo.AcaoEnviarTemplate, TemplateID: *tpl})
	} else {
		r = e.s.d.Acoes.Enviar(ctx, e.c, "mensagem", texto, "")
	}
	e.registrar(ctx, r, noID)
	switch r.Registro.Resultado {
	case dominio.AcaoBloqueada:
		return Envio{Bloqueio: r.Bloqueio}
	case dominio.AcaoFalhou:
		return Envio{Erro: detalhe(r)}
	}
	return Envio{}
}

func detalhe(r acoes.Resultado) string {
	if r.Registro.Detalhe != nil {
		return *r.Registro.Detalhe
	}
	return "falhou"
}

func (e *efeitos) Acao(ctx context.Context, noID string, a modelo.Acao, vars map[string]string) Envio {
	r := e.s.d.Acoes.Executar(ctx, e.comVars(vars), a)
	e.registrar(ctx, r, noID)
	if r.Registro.Resultado == dominio.AcaoBloqueada {
		return Envio{Bloqueio: r.Bloqueio}
	}
	return Envio{}
}

func (e *efeitos) IA(ctx context.Context, no modelo.No, vars map[string]string) (json.RawMessage, error) {
	if e.s.d.IA == nil {
		return nil, errors.New("automações de IA indisponíveis")
	}
	c := e.comVars(vars)
	entrada, err := acoes.ResolverEntrada(no.Entrada, e.s.d.Acoes.Fonte(ctx, c))
	if err != nil {
		return nil, err
	}
	ret, err := e.s.d.IA.ExecutarPorAcao(ctx, no.AutomacaoID, entrada, c)
	res := dominio.AcaoOK
	det := string(ret)
	if err != nil {
		res, det = dominio.AcaoFalhou, err.Error()
	} else if c.Simulacao {
		res = dominio.AcaoSimulada
	}
	d := execucoes.Resumo(det, 300)
	alvo := "nó " + no.ID
	if c.Exec != nil {
		c.Exec.RegistrarAcao(ctx, dominio.AcaoRegistrada{Tipo: "ia", Alvo: &alvo, Resultado: res, Detalhe: &d})
	}
	return ret, err
}

func (e *efeitos) Condicao(ctx context.Context, cd modelo.Condicoes, vars map[string]string) bool {
	return condicoes.Avaliar(&cd, e.s.d.Acoes.DadosCondicoes(ctx, e.comVars(vars)))
}

func (e *efeitos) Resolver(ctx context.Context, texto string, vars map[string]string) (string, error) {
	return acoes.Resolver(texto, e.s.d.Acoes.Fonte(ctx, e.comVars(vars)), nil)
}

func (e *efeitos) Humano(ctx context.Context, noID string) {
	if e.c.Alvo.ConversaID == "" {
		return
	}
	e.s.d.Pausas.PausarHumano(ctx, e.c.Alvo.ConversaID)
	aut, cv := e.c.Automacao.ID, e.c.Alvo.ConversaID
	e.s.d.Barramento.Publicar(eventos.Notificacao, e.c.Alvo.ContaID, dominio.Notificacao{Titulo: "Atendimento humano",
		Corpo: "O chatbot \"" + e.nome + "\" passou a conversa para você.", AutomacaoID: &aut, ConversaID: &cv, Tipo: "humano"})
	alvo := "nó " + noID
	d := "Conversa transferida para atendimento humano."
	if e.c.Exec != nil {
		e.c.Exec.RegistrarAcao(ctx, dominio.AcaoRegistrada{Tipo: "humano", Alvo: &alvo, Resultado: dominio.AcaoOK, Detalhe: &d})
	}
}

// ---------------------------------------------------------------------------
// Início
// ---------------------------------------------------------------------------

// Executar inicia a sessão (executor do despachante para automações "chatbot").
func (s *Servico) Executar(ctx context.Context, e *execucoes.Exec, a dominio.Automacao, c *acoes.Contexto) error {
	if err := e.Iniciar(ctx); err != nil {
		return err
	}
	if c.Alvo.ConversaID == "" {
		return e.Finalizar(ctx, execucoes.Fim{Estado: dominio.ExecErro, Erro: "O chatbot precisa de uma conversa."})
	}
	iniciou, err := s.iniciar(ctx, a, c)
	if err != nil {
		return e.Finalizar(ctx, execucoes.Fim{Estado: dominio.ExecErro, Erro: err.Error()})
	}
	fim := execucoes.Fim{Estado: dominio.ExecOK}
	if !iniciou {
		fim.Motivo = "a conversa já tem um chatbot ativo"
	}
	return e.Finalizar(ctx, fim)
}

// IniciarPorAcao é a ação iniciar_chatbot (false = a conversa já tem sessão ativa).
func (s *Servico) IniciarPorAcao(ctx context.Context, automacaoID string, c *acoes.Contexto) (bool, error) {
	a, err := armazenamento.ObterAutomacao(ctx, s.d.Banco.L(), automacaoID)
	if err != nil || a.Tipo != modelo.TipoChatbot {
		return false, erros.Novo(erros.NaoEncontrado, "Chatbot não encontrado.")
	}
	pai := ""
	if c.Exec != nil {
		pai = c.Exec.ID()
	}
	e, err := s.d.Registro.Criar(ctx, execucoes.Nova{Automacao: a, Origem: "fluxo", OrigemExecucaoID: pai, Cadeia: c.Cadeia,
		Gatilho: dominio.GatilhoExecucao{Tipo: modelo.GatilhoManual, Dados: map[string]any{"pedido_por": "fluxo"}},
		Alvo:    execucoes.Alvo{ContaID: c.Alvo.ContaID, ConversaID: c.Alvo.ConversaID, ContatoID: c.Alvo.ContatoID, LeadID: c.Alvo.LeadID}})
	if err != nil {
		return false, err
	}
	e.Iniciar(ctx)
	filho := &acoes.Contexto{Exec: e, Automacao: a, Alvo: c.Alvo, Cadeia: append(append([]string{}, c.Cadeia...), a.ID),
		Variaveis: map[string]string{}, UltimaMensagem: c.UltimaMensagem, Origem: modelo.TipoChatbot}
	iniciou, err := s.iniciar(ctx, a, filho)
	fim := execucoes.Fim{Estado: dominio.ExecOK}
	if err != nil {
		fim = execucoes.Fim{Estado: dominio.ExecErro, Erro: err.Error()}
	} else if !iniciou {
		fim.Motivo = "a conversa já tem um chatbot ativo"
	}
	e.Finalizar(ctx, fim)
	return iniciou, err
}

func (s *Servico) marcar(conversa string, v bool) {
	s.mu.Lock()
	if v {
		s.processando[conversa] = true
	} else {
		delete(s.processando, conversa)
	}
	s.mu.Unlock()
}

func (s *Servico) iniciar(ctx context.Context, a dominio.Automacao, c *acoes.Contexto) (bool, error) {
	if _, ativa, _ := armazenamento.SessaoAtivaDaConversa(ctx, s.d.Banco.L(), c.Alvo.ConversaID); ativa {
		return false, nil
	}
	def, errs := modelo.DecodificarChatbot(a.Definicao)
	if errs != nil {
		return false, errors.New("definição do chatbot inválida")
	}
	agora := s.d.Relogio.Agora()
	se := dominio.SessaoChatbot{ID: ids.NovoEm(agora), AutomacaoID: a.ID, AutomacaoNome: a.Nome, ConversaID: c.Alvo.ConversaID,
		Versao: a.Versao, NoAtual: def.Inicio, Variaveis: map[string]string{}, Estado: dominio.SessaoAtiva,
		ExpiraEm: agora.Add(time.Duration(def.InatividadeMin) * time.Minute), IniciadaEm: agora, AtualizadaEm: agora,
		Definicao: a.Definicao, ContaID: c.Alvo.ContaID}
	if err := armazenamento.InserirSessao(ctx, s.d.Banco.E(), se); err != nil {
		if erros.Eh(err, erros.Conflito) {
			return false, nil
		}
		return false, err
	}
	s.publicar(eventos.ChatbotSessaoIniciada, se)
	s.marcar(se.ConversaID, true)
	defer s.marcar(se.ConversaID, false)
	c.Origem = modelo.TipoChatbot
	ctx = fatos.ComCadeia(ctx, c.Cadeia)
	est := Iniciar(ctx, def, &efeitos{s: s, c: c, nome: a.Nome}, nil)
	s.aplicar(ctx, se, def, est)
	return true, nil
}

// aplicar grava o estado da rodada, reprograma a expiração e publica os eventos.
func (s *Servico) aplicar(ctx context.Context, se dominio.SessaoChatbot, def *modelo.DefinicaoChatbot, est Estado) dominio.SessaoChatbot {
	agora := s.d.Relogio.Agora()
	mudou := se.NoAtual != est.NoAtual || se.Tentativas != est.Tentativas || !mesmasVars(se.Variaveis, est.Variaveis)
	se.NoAtual, se.Variaveis, se.Tentativas, se.AtualizadaEm = est.NoAtual, est.Variaveis, est.Tentativas, agora
	chave := "sessao:" + se.ID
	if est.Final != "" {
		return s.finalizar(ctx, se, est.Final, est.Motivo)
	}
	se.ExpiraEm = agora.Add(time.Duration(def.InatividadeMin) * time.Minute)
	armazenamento.GravarSessao(ctx, s.d.Banco.E(), se)
	sid, cv := se.ID, se.ConversaID
	s.d.Esperas.Gravar(ctx, dominio.Espera{Tipo: dominio.EsperaExpirarSessao, AutomacaoID: se.AutomacaoID, SessaoID: &sid,
		ConversaID: &cv, Chave: &chave, RetomarEm: se.ExpiraEm})
	if mudou {
		s.publicar(eventos.ChatbotSessaoAtualizada, se)
	}
	return se
}

func mesmasVars(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func (s *Servico) finalizar(ctx context.Context, se dominio.SessaoChatbot, estado, motivo string) dominio.SessaoChatbot {
	agora := s.d.Relogio.Agora()
	se.Estado, se.AtualizadaEm, se.FinalizadaEm = estado, agora, &agora
	if motivo != "" {
		m := motivo
		se.Motivo = &m
	}
	armazenamento.GravarSessao(ctx, s.d.Banco.E(), se)
	s.d.Banco.E().ExecContext(ctx, `DELETE FROM esperas WHERE sessao_id = ?`, se.ID)
	s.publicar(eventos.ChatbotSessaoFinalizada, se)
	return se
}

// ---------------------------------------------------------------------------
// Mensagens, expiração e encerramentos
// ---------------------------------------------------------------------------

// Consumir entrega a mensagem recebida à sessão ativa (gancho do despachante).
func (s *Servico) Consumir(ctx context.Context, f fatos.Fato) bool {
	se, ativa, err := armazenamento.SessaoAtivaDaConversa(ctx, s.d.Banco.L(), f.ConversaID)
	if err != nil || !ativa {
		return false
	}
	if !se.ExpiraEm.After(s.d.Relogio.Agora()) {
		s.finalizar(ctx, se, dominio.SessaoExpirada, "inatividade")
		return false
	}
	s.d.Fila.Enfileirar(Chave(f.ConversaID), func(ctx context.Context) { s.responder(ctx, se.ID, f) })
	return true
}

func (s *Servico) responder(ctx context.Context, sessaoID string, f fatos.Fato) {
	se, err := armazenamento.ObterSessao(ctx, s.d.Banco.L(), sessaoID)
	if err != nil || se.Estado != dominio.SessaoAtiva {
		return
	}
	a, err := armazenamento.ObterAutomacao(ctx, s.d.Banco.L(), se.AutomacaoID)
	if err != nil {
		return
	}
	def, errs := modelo.DecodificarChatbot(se.Definicao) // cópia congelada
	if errs != nil {
		s.finalizar(ctx, se, dominio.SessaoAbortada, "definição congelada inválida")
		return
	}
	a.Versao = se.Versao
	e, err := s.d.Registro.Criar(ctx, execucoes.Nova{Automacao: a, Origem: "gatilho", Cadeia: f.Cadeia,
		Gatilho: dominio.GatilhoExecucao{Tipo: modelo.GatilhoMensagemRecebida, Dados: map[string]any{"mensagem_id": f.MensagemID, "sessao_id": se.ID}},
		Alvo:    execucoes.Alvo{ContaID: f.ContaID, ConversaID: f.ConversaID, ContatoID: f.ContatoID}})
	if err != nil {
		return
	}
	e.Iniciar(ctx)
	texto := f.Texto
	c := &acoes.Contexto{Exec: e, Automacao: a, Alvo: acoes.Alvo{ContaID: f.ContaID, ConversaID: f.ConversaID, ContatoID: f.ContatoID},
		Cadeia: append(append([]string{}, f.Cadeia...), a.ID), Variaveis: se.Variaveis, UltimaMensagem: &texto, Origem: modelo.TipoChatbot}
	s.marcar(se.ConversaID, true)
	est := Responder(fatos.ComCadeia(ctx, c.Cadeia), def, &efeitos{s: s, c: c, nome: a.Nome},
		Estado{NoAtual: se.NoAtual, Variaveis: se.Variaveis, Tentativas: se.Tentativas}, Resposta{Texto: f.Texto, TemMidia: f.TemMidia})
	s.marcar(se.ConversaID, false)
	s.aplicar(ctx, se, def, est)
	e.DefinirVariavel(ctx, "no_atual", est.NoAtual)
	e.Finalizar(ctx, execucoes.Fim{Estado: dominio.ExecOK})
}

// ExpirarSessao trata a espera expirar_sessao (motivo "expirada" do agendador = > 24 h → abortada).
func (s *Servico) ExpirarSessao(ctx context.Context, esp dominio.Espera, motivo string) {
	if esp.SessaoID == nil {
		return
	}
	se, err := armazenamento.ObterSessao(ctx, s.d.Banco.L(), *esp.SessaoID)
	if err != nil || se.Estado != dominio.SessaoAtiva {
		return
	}
	if se.ExpiraEm.After(s.d.Relogio.Agora()) {
		return // houve interação depois; a espera nova cuida
	}
	if motivo != "" {
		s.finalizar(ctx, se, dominio.SessaoAbortada, "sessão vencida há mais de 24 h")
		return
	}
	s.finalizar(ctx, se, dominio.SessaoExpirada, "inatividade")
}

// EncerrarDaConversa encerra a sessão ativa da conversa ("Assumir", resposta manual).
func (s *Servico) EncerrarDaConversa(ctx context.Context, conversaID, estado, motivo string) {
	if se, ativa, err := armazenamento.SessaoAtivaDaConversa(ctx, s.d.Banco.L(), conversaID); err == nil && ativa {
		s.finalizar(ctx, se, estado, motivo)
	}
}

// EncerrarDaAutomacao encerra as sessões ativas de um chatbot (desativado/excluído).
func (s *Servico) EncerrarDaAutomacao(ctx context.Context, a dominio.Automacao, motivo string) {
	if a.Tipo != modelo.TipoChatbot {
		return
	}
	l, _ := armazenamento.SessoesAtivasDaAutomacao(ctx, s.d.Banco.L(), a.ID)
	for _, se := range l {
		s.finalizar(ctx, se, dominio.SessaoAbortada, motivo)
	}
}
