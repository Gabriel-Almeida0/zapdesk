// Pacote despacho é o despachante de automações (research.md › R2/R11; plan.md › Fluxos-chave):
// recebe fatos dos serviços (sem bloquear), casa gatilhos das automações ativas em ordem de
// prioridade, cria as execuções e as roda numa fila por (automação, conversa) — em ordem dentro da
// fila, filas diferentes em paralelo. Também trata a manutenção ligada às mensagens (pausa
// "humano" por resposta manual, esperas "sem resposta"), o gancho da sessão de chatbot ativa e as
// esperas vencidas entregues pelo agendador.
package despacho

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/automacoes/acoes"
	"zapdesk/motor/internal/automacoes/execucoes"
	"zapdesk/motor/internal/automacoes/gatilhos"
	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/automacoes/seguranca"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/fatos"
	"zapdesk/motor/internal/relogio"
)

// Executor roda uma execução nova de um tipo de automação.
type Executor interface {
	Executar(ctx context.Context, e *execucoes.Exec, a dominio.Automacao, c *acoes.Contexto) error
}

// Retomador continua uma execução de fluxo que estava aguardando.
type Retomador interface {
	Retomar(ctx context.Context, e *execucoes.Exec, a dominio.Automacao, c *acoes.Contexto) error
}

// Chatbot é o gancho das sessões de chatbot (US4).
type Chatbot interface {
	// Consumir entrega a mensagem recebida à sessão ativa da conversa (true = consumiu; as demais
	// automações não recebem gatilhos de mensagem dessa mensagem).
	Consumir(ctx context.Context, f fatos.Fato) bool
	// ExpirarSessao trata a espera expirar_sessao (motivo vazio = expirou normalmente).
	ExpirarSessao(ctx context.Context, e dominio.Espera, motivo string)
}

// Esperas é o que o despachante usa do agendador.
type Esperas interface {
	ProgramarSemResposta(ctx context.Context, automacaoID, conversaID, referencia string, enviadaEm time.Time, aposS int, gatilhoIndice int) error
	CancelarSemResposta(ctx context.Context, conversaID string) error
}

// Deps do despachante.
type Deps struct {
	Banco      *armazenamento.Banco
	Relogio    relogio.Relogio
	Log        zerolog.Logger
	Registro   *execucoes.Registro
	Acoes      *acoes.Executores
	Pausas     *seguranca.Pausas
	Config     seguranca.Configuracao
	Executores map[string]Executor // por tipo de automação
	Retomador  Retomador           // fluxo
	Esperas    Esperas
	Chatbot    Chatbot
}

// Despachante de fatos e execuções.
type Despachante struct {
	d Deps

	mu        sync.Mutex
	fatos     []fatos.Fato
	sinal     chan struct{}
	filas     map[string]*fila
	pendentes int // fatos na fila + tarefas não terminadas
	ocioso    chan struct{}
	fechado   bool

	ctx      context.Context
	cancelar context.CancelFunc
	wg       sync.WaitGroup
}

type fila struct {
	tarefas []func(ctx context.Context)
}

// Novo cria o despachante (parado até Iniciar).
func Novo(d Deps) *Despachante {
	if d.Executores == nil {
		d.Executores = map[string]Executor{}
	}
	return &Despachante{d: d, sinal: make(chan struct{}, 1), filas: map[string]*fila{}, ocioso: make(chan struct{})}
}

// DefinirExecutor liga o executor de um tipo (chatbot/ia chegam nas histórias seguintes).
func (x *Despachante) DefinirExecutor(tipo string, e Executor) {
	x.mu.Lock()
	x.d.Executores[tipo] = e
	x.mu.Unlock()
}

// DefinirChatbot liga o gancho de sessões.
func (x *Despachante) DefinirChatbot(c Chatbot) {
	x.mu.Lock()
	x.d.Chatbot = c
	x.mu.Unlock()
}

// DefinirRetomador liga o executor que retoma fluxos em espera.
func (x *Despachante) DefinirRetomador(r Retomador) {
	x.mu.Lock()
	x.d.Retomador = r
	x.mu.Unlock()
}

// DefinirEsperas liga o agendador.
func (x *Despachante) DefinirEsperas(e Esperas) {
	x.mu.Lock()
	x.d.Esperas = e
	x.mu.Unlock()
}

// Iniciar põe o despachante para trabalhar.
func (x *Despachante) Iniciar(ctx context.Context) {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.ctx != nil {
		return
	}
	x.ctx, x.cancelar = context.WithCancel(ctx)
	x.wg.Add(1)
	go x.laco()
}

// Receber enfileira um fato (nunca bloqueia; implementa fatos.Receptor).
func (x *Despachante) Receber(f fatos.Fato) {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.fechado {
		return
	}
	x.fatos = append(x.fatos, f)
	x.pendentes++
	select {
	case x.sinal <- struct{}{}:
	default:
	}
}

func (x *Despachante) laco() {
	defer x.wg.Done()
	for {
		x.mu.Lock()
		if len(x.fatos) == 0 {
			x.mu.Unlock()
			select {
			case <-x.ctx.Done():
				return
			case <-x.sinal:
				continue
			}
		}
		f := x.fatos[0]
		x.fatos = x.fatos[1:]
		x.mu.Unlock()
		func() {
			defer x.concluir()
			defer func() {
				if r := recover(); r != nil {
					x.d.Log.Error().Interface("panico", r).Str("fato", f.Tipo).Msg("pânico ao processar fato de automação")
				}
			}()
			x.processar(x.ctx, f)
		}()
	}
}

func (x *Despachante) concluir() {
	x.mu.Lock()
	x.pendentes--
	if x.pendentes == 0 {
		close(x.ocioso)
		x.ocioso = make(chan struct{})
	}
	x.mu.Unlock()
}

// Enfileirar põe uma tarefa na fila da chave (em ordem dentro da chave).
func (x *Despachante) Enfileirar(chave string, t func(ctx context.Context)) {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.fechado || x.ctx == nil {
		return
	}
	x.pendentes++
	fl, existe := x.filas[chave]
	if !existe {
		fl = &fila{}
		x.filas[chave] = fl
	}
	fl.tarefas = append(fl.tarefas, t)
	if !existe {
		x.wg.Add(1)
		go x.trabalhar(chave, fl)
	}
}

func (x *Despachante) trabalhar(chave string, fl *fila) {
	defer x.wg.Done()
	for {
		x.mu.Lock()
		if len(fl.tarefas) == 0 {
			delete(x.filas, chave)
			x.mu.Unlock()
			return
		}
		t := fl.tarefas[0]
		fl.tarefas = fl.tarefas[1:]
		ctx := x.ctx
		x.mu.Unlock()
		func() {
			defer x.concluir()
			defer func() {
				if r := recover(); r != nil {
					x.d.Log.Error().Interface("panico", r).Msg("pânico numa execução de automação")
				}
			}()
			t(ctx)
		}()
	}
}

// AguardarOcioso espera todos os fatos e tarefas terminarem (testes e /falso/processar-esperas).
func (x *Despachante) AguardarOcioso(ctx context.Context) error {
	for {
		x.mu.Lock()
		if x.pendentes == 0 {
			x.mu.Unlock()
			return nil
		}
		c := x.ocioso
		x.mu.Unlock()
		select {
		case <-c:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Encerrar para de aceitar fatos, cancela as execuções em andamento, espera as filas (até o fim
// do ctx) e marca as que ficaram "rodando"/"na_fila" como "abortada" ("app fechado").
func (x *Despachante) Encerrar(ctx context.Context) {
	x.mu.Lock()
	x.fechado = true
	cancelar := x.cancelar
	x.mu.Unlock()
	if cancelar != nil {
		cancelar()
	}
	feito := make(chan struct{})
	go func() {
		x.wg.Wait()
		close(feito)
	}()
	select {
	case <-feito:
	case <-ctx.Done():
	}
	if x.d.Registro != nil {
		x.d.Registro.AbortarPendentes(context.WithoutCancel(ctx), MotivoAppFechado, dominio.ExecRodando, dominio.ExecNaFila)
	}
}

// MotivoAppFechado das execuções interrompidas no encerramento.
const MotivoAppFechado = "app fechado"

// ---------------------------------------------------------------------------
// Fatos
// ---------------------------------------------------------------------------

func (x *Despachante) pausaGeral() bool { return x.d.Config.Configuracao().PausaGeral }

func (x *Despachante) chatbot() Chatbot {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.d.Chatbot
}

func (x *Despachante) esperas() Esperas {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.d.Esperas
}

// ativas carrega as automações ativas com os gatilhos interpretados.
func (x *Despachante) ativas(ctx context.Context) ([]dominio.Automacao, []gatilhos.Candidata) {
	sim := true
	lista, err := armazenamento.ListarAutomacoes(ctx, x.d.Banco.L(), armazenamento.FiltroAutomacoes{Ativa: &sim})
	if err != nil {
		x.d.Log.Warn().Err(err).Msg("listar automações ativas")
		return nil, nil
	}
	cands := make([]gatilhos.Candidata, 0, len(lista))
	for _, a := range lista {
		gs, _ := modelo.DecodificarGatilhos(a.Gatilhos)
		cands = append(cands, gatilhos.Candidata{ID: a.ID, Prioridade: a.Prioridade, CriadaEm: a.CriadaEm, Contas: a.Contas,
			IncluirGrupos: a.IncluirGrupos, Gatilhos: gs})
	}
	return lista, cands
}

func (x *Despachante) processar(ctx context.Context, f fatos.Fato) {
	switch f.Tipo {
	case fatos.MensagemEnviada:
		x.mensagemEnviada(ctx, f)
		return
	case fatos.MensagemRecebida:
		if es := x.esperas(); es != nil && f.ConversaID != "" {
			es.CancelarSemResposta(ctx, f.ConversaID)
		}
	}
	if x.pausaGeral() {
		return
	}
	if f.ConversaID != "" {
		if _, pausada := x.d.Pausas.Obter(ctx, f.ConversaID); pausada {
			return
		}
	}
	if f.Tipo == fatos.MensagemRecebida {
		if cb := x.chatbot(); cb != nil && cb.Consumir(ctx, f) {
			return
		}
	}
	auts, cands := x.ativas(ctx)
	porID := map[string]dominio.Automacao{}
	for _, a := range auts {
		porID[a.ID] = a
	}
	for _, m := range gatilhos.Casar(cands, f) {
		a := porID[m.Automacao.ID]
		if f.Tipo == fatos.LeadImportado {
			for _, lead := range f.LeadIDs {
				fl := f
				fl.LeadID, fl.LeadIDs = lead, nil
				x.disparar(ctx, a, m.Gatilho, fl)
			}
			continue
		}
		x.disparar(ctx, a, m.Gatilho, f)
	}
}

// mensagemEnviada: resposta manual → pausa "humano"; toda mensagem minha (re)programa as esperas
// "sem resposta" das automações ativas com filtro de origem compatível.
func (x *Despachante) mensagemEnviada(ctx context.Context, f fatos.Fato) {
	if f.ConversaID == "" {
		return
	}
	if f.Origem == fatos.OrigemManual && !f.Grupo {
		x.d.Pausas.PausarHumano(ctx, f.ConversaID)
	}
	es := x.esperas()
	if es == nil {
		return
	}
	if len(f.Cadeia) >= gatilhos.ProfundidadeMaxima {
		return
	}
	naCadeia := map[string]bool{}
	for _, id := range f.Cadeia {
		naCadeia[id] = true
	}
	auts, _ := x.ativas(ctx)
	for _, a := range auts {
		// A própria mensagem de uma automação não reprograma o "sem resposta" dela (R11).
		if naCadeia[a.ID] || (f.Grupo && !a.IncluirGrupos) || !contaPermitida(a, f.ContaID) {
			continue
		}
		gs, _ := modelo.DecodificarGatilhos(a.Gatilhos)
		for i, g := range gs {
			if g.Tipo != modelo.GatilhoSemResposta {
				continue
			}
			if o := g.OrigemMensagem; o != "" && o != "qualquer" && o != f.Origem {
				continue
			}
			if err := es.ProgramarSemResposta(ctx, a.ID, f.ConversaID, f.MensagemID, f.Em, g.AposS, i); err != nil {
				x.d.Log.Warn().Err(err).Msg("programar sem resposta")
			}
			break
		}
	}
}

func contaPermitida(a dominio.Automacao, conta string) bool {
	if a.Contas == nil || conta == "" {
		return true
	}
	for _, c := range a.Contas {
		if c == conta {
			return true
		}
	}
	return false
}

// DadosDoFato monta gatilho.dados (formatos.md › Dados do fato entregues à execução).
func DadosDoFato(f fatos.Fato) map[string]any {
	d := map[string]any{}
	switch f.Tipo {
	case fatos.MensagemRecebida:
		d["mensagem_id"] = f.MensagemID
	case fatos.LeadImportado:
		d["lead_ids"] = []string{f.LeadID}
		d["origem"] = f.OrigemLead
	case fatos.Etiqueta:
		d["etiqueta_id"], d["contato_id"], d["evento"] = f.EtiquetaID, f.ContatoID, f.Evento
	case fatos.EntrouEtapa:
		d["funil_id"], d["etapa_id"], d["lead_id"] = f.FunilID, f.EtapaID, f.LeadID
		if f.EtapaAnteriorID != "" {
			d["etapa_anterior_id"] = f.EtapaAnteriorID
		} else {
			d["etapa_anterior_id"] = nil
		}
	case fatos.DisparoRespondeu:
		d["disparo_id"], d["destinatario_id"] = f.DisparoID, f.DestinatarioID
	}
	return d
}

// Chave da fila de uma execução: (automação, conversa) — ou lead/sem alvo.
func Chave(automacaoID string, alvo acoes.Alvo) string {
	switch {
	case alvo.ConversaID != "":
		return automacaoID + "|c:" + alvo.ConversaID
	case alvo.LeadID != "":
		return automacaoID + "|l:" + alvo.LeadID
	case alvo.ContatoID != "":
		return automacaoID + "|t:" + alvo.ContatoID
	}
	return automacaoID + "|-"
}

// disparar cria a execução do casamento e a enfileira.
func (x *Despachante) disparar(ctx context.Context, a dominio.Automacao, g modelo.Gatilho, f fatos.Fato) {
	alvo := acoes.Alvo{ContaID: f.ContaID, ConversaID: f.ConversaID, ContatoID: f.ContatoID, LeadID: f.LeadID}
	var ultima *string
	if f.Tipo == fatos.MensagemRecebida {
		t := f.Texto
		ultima = &t
	}
	x.CriarEEnfileirar(ctx, a, dominio.GatilhoExecucao{Tipo: g.Tipo, Dados: DadosDoFato(f)}, "gatilho", "", f.Cadeia, alvo, ultima, nil)
}

// CriarEEnfileirar cria a execução (na_fila) e a enfileira na fila da chave; devolve a execução.
func (x *Despachante) CriarEEnfileirar(ctx context.Context, a dominio.Automacao, g dominio.GatilhoExecucao, origem, origemExec string,
	cadeia []string, alvo acoes.Alvo, ultima *string, vars map[string]any) (*execucoes.Exec, error) {
	e, err := x.d.Registro.Criar(ctx, execucoes.Nova{Automacao: a, Gatilho: g, Origem: origem, OrigemExecucaoID: origemExec,
		Cadeia: cadeia, Alvo: execucoes.Alvo{ContaID: alvo.ContaID, ConversaID: alvo.ConversaID, ContatoID: alvo.ContatoID, LeadID: alvo.LeadID},
		Variaveis: vars})
	if err != nil {
		x.d.Log.Warn().Err(err).Msg("criar execução")
		return nil, err
	}
	c := &acoes.Contexto{Exec: e, Automacao: a, Alvo: alvo, Cadeia: append(append([]string{}, cadeia...), a.ID),
		Variaveis: map[string]string{}, UltimaMensagem: ultima, Origem: a.Tipo}
	x.Enfileirar(Chave(a.ID, alvo), func(ctx context.Context) { x.Rodar(ctx, e, a, c) })
	return e, nil
}

// Rodar executa uma execução já criada pelo executor do tipo da automação.
func (x *Despachante) Rodar(ctx context.Context, e *execucoes.Exec, a dominio.Automacao, c *acoes.Contexto) {
	x.mu.Lock()
	ex := x.d.Executores[a.Tipo]
	x.mu.Unlock()
	if ctx.Err() != nil {
		e.Finalizar(context.WithoutCancel(ctx), execucoes.Fim{Estado: dominio.ExecAbortada, Motivo: MotivoAppFechado})
		return
	}
	if ex == nil {
		e.Iniciar(ctx)
		e.Finalizar(ctx, execucoes.Fim{Estado: dominio.ExecErro, Erro: "Este tipo de automação ainda não pode ser executado."})
		return
	}
	if err := ex.Executar(ctx, e, a, c); err != nil {
		x.d.Log.Warn().Err(err).Str("automacao", a.ID).Msg("execução de automação")
		if !dominio.FinalExec(e.Estado()) && e.Estado() != dominio.ExecAguardando {
			fim := execucoes.Fim{Estado: dominio.ExecErro, Erro: "Erro interno: " + err.Error()}
			if ctx.Err() != nil {
				fim = execucoes.Fim{Estado: dominio.ExecAbortada, Motivo: MotivoAppFechado}
			}
			e.Finalizar(context.WithoutCancel(ctx), fim)
		}
	}
}

// ---------------------------------------------------------------------------
// Esperas vencidas (esperas.Manipulador)
// ---------------------------------------------------------------------------

// Executar trata uma espera vencida.
func (x *Despachante) Executar(ctx context.Context, esp dominio.Espera) error {
	switch esp.Tipo {
	case dominio.EsperaExpirarSessao:
		if cb := x.chatbot(); cb != nil {
			cb.ExpirarSessao(ctx, esp, "")
		}
		return nil
	case dominio.EsperaAguardar:
		return x.retomar(ctx, esp)
	}
	a, err := armazenamento.ObterAutomacao(ctx, x.d.Banco.L(), esp.AutomacaoID)
	if err != nil || !a.Ativa || x.pausaGeral() {
		return err
	}
	alvo := acoes.Alvo{}
	if esp.ConversaID != nil {
		alvo.ConversaID = *esp.ConversaID
	}
	g := dominio.GatilhoExecucao{Dados: map[string]any{}}
	switch esp.Tipo {
	case dominio.EsperaSemResposta:
		g.Tipo = modelo.GatilhoSemResposta
		if esp.Referencia != nil {
			g.Dados["mensagem_id"] = *esp.Referencia
		}
		var d struct {
			AposS int `json:"apos_s"`
		}
		json.Unmarshal(esp.Dados, &d)
		g.Dados["apos_s"] = d.AposS
	case dominio.EsperaAgendamento:
		g.Tipo = modelo.GatilhoAgendamento
		g.Dados["ocorrencia"] = esp.RetomarEm
		var d map[string]any
		json.Unmarshal(esp.Dados, &d)
		for _, k := range []string{"cron", "intervalo_s"} {
			if v, ok := d[k]; ok {
				g.Dados[k] = v
			}
		}
	case dominio.EsperaAgendar:
		g.Tipo = modelo.GatilhoAgendar
		g.Dados["agendamento_id"] = esp.ID
		g.Dados["previsto_para"] = esp.RetomarEm
		var d struct {
			Dados json.RawMessage `json:"dados"`
		}
		json.Unmarshal(esp.Dados, &d)
		if len(d.Dados) > 0 {
			g.Dados["dados"] = d.Dados
		}
	}
	x.CriarEEnfileirar(ctx, a, g, "gatilho", "", nil, alvo, nil, nil)
	return nil
}

func (x *Despachante) retomar(ctx context.Context, esp dominio.Espera) error {
	if esp.ExecucaoID == nil {
		return nil
	}
	e, err := x.d.Registro.Carregar(ctx, *esp.ExecucaoID)
	if err != nil {
		return err
	}
	if e.Estado() != dominio.ExecAguardando {
		return nil
	}
	a, err := armazenamento.ObterAutomacao(ctx, x.d.Banco.L(), esp.AutomacaoID)
	if err != nil {
		return e.Finalizar(ctx, execucoes.Fim{Estado: dominio.ExecAbortada, Motivo: "automação excluída"})
	}
	if x.pausaGeral() {
		return e.Finalizar(ctx, execucoes.Fim{Estado: dominio.ExecAbortada, Motivo: "automações pausadas"})
	}
	d := e.Detalhe()
	alvo := acoes.Alvo{ContaID: val(d.ContaID), ConversaID: val(d.ConversaID), ContatoID: val(d.ContatoID), LeadID: val(d.LeadID)}
	var ultima *string
	if mid, ok := d.Gatilho.Dados["mensagem_id"].(string); ok && mid != "" {
		if m, err := armazenamento.ObterMensagem(ctx, x.d.Banco.L(), mid); err == nil && m.Texto != nil {
			ultima = m.Texto
		}
	}
	c := &acoes.Contexto{Exec: e, Automacao: a, Alvo: alvo, Cadeia: append(append([]string{}, d.Cadeia...), a.ID),
		Variaveis: e.Variaveis(), UltimaMensagem: ultima, Origem: a.Tipo}
	x.Enfileirar(Chave(a.ID, alvo), func(ctx context.Context) {
		if x.d.Retomador == nil {
			return
		}
		if err := x.d.Retomador.Retomar(ctx, e, a, c); err != nil {
			x.d.Log.Warn().Err(err).Msg("retomar execução")
		}
	})
	return nil
}

func val(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Abortar trata uma espera que não será executada (expirada, conversa pausada).
func (x *Despachante) Abortar(ctx context.Context, esp dominio.Espera, motivo string) {
	switch esp.Tipo {
	case dominio.EsperaExpirarSessao:
		if cb := x.chatbot(); cb != nil {
			cb.ExpirarSessao(ctx, esp, motivo)
		}
		return
	case dominio.EsperaAguardar:
		if esp.ExecucaoID != nil {
			if e, err := x.d.Registro.Carregar(ctx, *esp.ExecucaoID); err == nil {
				e.Finalizar(ctx, execucoes.Fim{Estado: dominio.ExecAbortada, Motivo: motivo})
			}
		}
		return
	}
	a, err := armazenamento.ObterAutomacao(ctx, x.d.Banco.L(), esp.AutomacaoID)
	if err != nil || !a.Ativa {
		return
	}
	tipo := map[string]string{dominio.EsperaSemResposta: modelo.GatilhoSemResposta, dominio.EsperaAgendamento: modelo.GatilhoAgendamento,
		dominio.EsperaAgendar: modelo.GatilhoAgendar}[esp.Tipo]
	alvo := execucoes.Alvo{}
	if esp.ConversaID != nil {
		alvo.ConversaID = *esp.ConversaID
	}
	e, err := x.d.Registro.Criar(ctx, execucoes.Nova{Automacao: a, Origem: "gatilho", Alvo: alvo,
		Gatilho: dominio.GatilhoExecucao{Tipo: tipo, Dados: map[string]any{"previsto_para": esp.RetomarEm}}})
	if err == nil {
		e.Finalizar(ctx, execucoes.Fim{Estado: dominio.ExecAbortada, Motivo: motivo})
	}
}
