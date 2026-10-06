package aplicacao

import (
	"context"
	"time"

	"zapdesk/motor/internal/api"
	"zapdesk/motor/internal/automacoes"
	"zapdesk/motor/internal/automacoes/acoes"
	"zapdesk/motor/internal/automacoes/chatbot"
	"zapdesk/motor/internal/automacoes/despacho"
	"zapdesk/motor/internal/automacoes/esperas"
	"zapdesk/motor/internal/automacoes/execucoes"
	"zapdesk/motor/internal/automacoes/fluxo"
	"zapdesk/motor/internal/automacoes/seguranca"
	"zapdesk/motor/internal/automacoes/simulador"
	"zapdesk/motor/internal/claude"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/funil"
	"zapdesk/motor/internal/segredos"
)

// PrazoSegredos é quanto o motor espera o primeiro comando "segredos" com --aguardar-segredos.
const PrazoSegredos = 2 * time.Second

// pecasAutomacoes guarda as peças do motor de automações (feature 002).
type pecasAutomacoes struct {
	cofre      *segredos.Cofre
	config     *automacoes.Config
	registro   *execucoes.Registro
	portao     *seguranca.Portao
	funil      *funil.Servico
	acoes      *acoes.Executores
	fluxo      *fluxo.Executor
	despacho   *despacho.Despachante
	agendador  *esperas.Agendador
	servico    *automacoes.Servico
	chatbot    *chatbot.Servico
	simulador  *simulador.Simulador
	claude     claude.Cliente
	iaFalsa    *claude.Falsa
	encerrarIA func(ctx context.Context)
	iniciado   chan struct{}
	parar      context.CancelFunc
}

// verificador informa ao agendador o estado da conversa de uma espera.
type verificador struct{ a *App }

func (v verificador) ConversaPausada(ctx context.Context, conversaID string) bool {
	_, ok := v.a.aut.portao.Pausas().Obter(ctx, conversaID)
	return ok
}

func (v verificador) ContaConectada(ctx context.Context, conversaID string) bool {
	var conta string
	if v.a.Banco.L().QueryRowContext(ctx, `SELECT conta_id FROM conversas WHERE id = ?`, conversaID).Scan(&conta) != nil {
		return true // conversa sumiu: deixa a execução tratar
	}
	_, err := v.a.sv.contas.Cliente(ctx, conta)
	return err == nil
}

// montarAutomacoes monta funil, portão, execuções, ações, fluxo, despachante, agendador e a
// fachada; liga os ouvintes de fatos dos serviços do MVP. Nada executa até iniciarAutomacoes.
func (a *App) montarAutomacoes(ctx context.Context, sv *api.Servicos) error {
	p := &a.aut
	p.iniciado = make(chan struct{})
	p.cofre = segredos.Novo()
	p.config = automacoes.CarregarConfig(ctx, a.Banco, a.Barramento)
	p.registro = execucoes.Novo(a.Banco, a.Barramento, a.Relogio)
	p.portao = seguranca.NovoPortao(a.Banco, a.Barramento, a.Relogio, p.config)
	p.funil = funil.Novo(a.Banco, a.Barramento, a.Relogio, a.sv.leads)
	p.acoes = acoes.Novo(acoes.Deps{Banco: a.Banco, Barramento: a.Barramento, Relogio: a.Relogio, Chat: a.sv.chat,
		Organizacao: sv.Organizacao, Leads: a.sv.leads, Funil: p.funil, Portao: p.portao})
	p.despacho = despacho.Novo(despacho.Deps{Banco: a.Banco, Relogio: a.Relogio, Log: a.o.Log, Registro: p.registro,
		Acoes: p.acoes, Pausas: p.portao.Pausas(), Config: p.config})
	p.agendador = esperas.Novo(a.Banco, a.Relogio, a.o.Log, p.despacho, verificador{a}, p.portao.Pausas(), p.registro)
	p.fluxo = fluxo.Novo(p.acoes, p.agendador, a.Relogio)
	p.despacho.DefinirExecutor("fluxo", p.fluxo)
	p.despacho.DefinirRetomador(p.fluxo)
	p.despacho.DefinirEsperas(p.agendador)
	p.portao.Pausas().AoMudar(p.agendador.Acordar)
	p.acoes.DefinirDisparos(a.sv.disparos)

	// Fontes de fatos (research.md › R2).
	a.sv.chat.DefinirReceptorFatos(p.despacho)
	a.sv.leads.DefinirReceptorFatos(p.despacho)
	a.sv.disparos.DefinirReceptorFatos(p.despacho)
	sv.Organizacao.DefinirReceptorFatos(p.despacho)
	p.funil.DefinirReceptorFatos(p.despacho)

	// IA (Claude): simulada com --ia=falsa; real só com a chave do usuário (Constituição I).
	if a.o.IA == "falsa" {
		p.iaFalsa = claude.NovaFalsa(a.Relogio.Agora)
		p.claude = p.iaFalsa
	} else {
		p.claude = claude.NovoReal(claude.OpcoesReal{Chave: p.cofre, URLBase: a.o.URLClaude})
	}

	p.servico = automacoes.Novo(automacoes.Deps{Banco: a.Banco, Barramento: a.Barramento, Relogio: a.Relogio, Log: a.o.Log,
		Config: p.config, Registro: p.registro, Portao: p.portao, Acoes: p.acoes, Fluxo: p.fluxo, Despacho: p.despacho,
		Agendador: p.agendador, Cofre: p.cofre, Chat: a.sv.chat})
	// Chatbots (US4): sessões por conversa, gancho de mensagens e simulador.
	p.chatbot = chatbot.Novo(chatbot.Deps{Banco: a.Banco, Barramento: a.Barramento, Relogio: a.Relogio, Log: a.o.Log,
		Registro: p.registro, Acoes: p.acoes, Pausas: p.portao.Pausas(), Fila: p.despacho, Esperas: p.agendador})
	p.despacho.DefinirChatbot(p.chatbot)
	p.despacho.DefinirExecutor("chatbot", p.chatbot)
	p.acoes.DefinirChatbots(p.chatbot)
	p.servico.AdicionarGanchos(automacoes.Ganchos{
		AoDesativar: []func(context.Context, dominio.Automacao){func(ctx context.Context, x dominio.Automacao) {
			p.chatbot.EncerrarDaAutomacao(ctx, x, "chatbot desativado")
		}},
		AoExcluir: []func(context.Context, dominio.Automacao){func(ctx context.Context, x dominio.Automacao) {
			p.chatbot.EncerrarDaAutomacao(ctx, x, "chatbot excluído")
		}},
	})
	p.simulador = simulador.Novo(simulador.Deps{Banco: a.Banco, Registro: p.registro, Acoes: p.acoes,
		Alvo: func(ctx context.Context, conversa, contato, lead, tel, conta string) (acoes.Alvo, error) {
			return p.servico.ResolverAlvo(ctx, automacoes.AlvoExecucao{ConversaID: conversa, ContatoID: contato, LeadID: lead, Telefone: tel, ContaID: conta}, "alvo")
		}})
	sv.Chatbot, sv.Simulador = p.chatbot, p.simulador

	p.cofre.Assinar(func(nomes []string) {
		a.Barramento.Publicar(eventos.SegredosAlterados, "", map[string]any{"nomes": p.cofre.Nomes()})
	})
	if err := a.montarIA(ctx, sv); err != nil {
		return err
	}
	sv.Automacoes = p.servico
	sv.Funil = p.funil
	sv.Claude = p.claude
	sv.IAFalsa = p.iaFalsa
	sv.Cofre = p.cofre
	sv.ProcessarEsperas = a.processarEsperas
	return nil
}

// iniciarAutomacoes roda depois do "pronto", em segundo plano: espera os segredos (se pedido),
// recupera execuções interrompidas e liga despachante e agendador.
func (a *App) iniciarAutomacoes(pai context.Context) {
	p := &a.aut
	ctx, parar := context.WithCancel(pai)
	p.parar = parar
	go func() {
		defer close(p.iniciado)
		if a.o.AguardarSegredos {
			c, cancelar := context.WithTimeout(ctx, PrazoSegredos)
			if !p.cofre.AguardarPrimeiro(c) {
				a.o.Log.Warn().Msg("segredos não chegaram em 2 s; automações seguem sem segredos")
			}
			cancelar()
		}
		if ctx.Err() != nil {
			return
		}
		if err := p.agendador.Recuperar(ctx); err != nil {
			a.o.Log.Warn().Err(err).Msg("recuperar execuções")
		}
		p.despacho.Iniciar(ctx)
		p.agendador.Iniciar(ctx)
	}()
}

// encerrarAutomacoes: despachante (rodando → abortada "app fechado") → runners → agendador.
func (a *App) encerrarAutomacoes(ctx context.Context) {
	p := &a.aut
	if p.despacho == nil {
		return
	}
	if p.parar != nil {
		p.parar()
		select {
		case <-p.iniciado:
		case <-ctx.Done():
		}
	}
	p.despacho.Encerrar(ctx)
	if p.encerrarIA != nil {
		p.encerrarIA(ctx)
	}
	p.agendador.Encerrar()
}

// processarEsperas é o POST /v1/falso/processar-esperas: uma passada síncrona do agendador e
// espera os despachos e os envios que eles enfileiraram terminarem.
func (a *App) processarEsperas(ctx context.Context) error {
	p := &a.aut
	select {
	case <-p.iniciado:
	case <-ctx.Done():
		return ctx.Err()
	}
	p.agendador.Passada(ctx)
	return a.AguardarAutomacoes(ctx)
}

// Cofre devolve o cofre de segredos (o main entrega nele as linhas "segredos" do stdin).
func (a *App) Cofre() *segredos.Cofre { return a.aut.cofre }

// AguardarAutomacoes espera o despachante ficar ocioso (testes de integração).
func (a *App) AguardarAutomacoes(ctx context.Context) error {
	select {
	case <-a.aut.iniciado:
	case <-ctx.Done():
		return ctx.Err()
	}
	// Envios automáticos saem pela fila do chat (assíncrona) e podem gerar novos fatos só ao
	// gravar; esperar despachante → filas → despachante até tudo parar.
	for i := 0; i < 3; i++ {
		if err := a.aut.despacho.AguardarOcioso(ctx); err != nil {
			return err
		}
		if err := a.apiServicos.Chat.AguardarEnvios(ctx); err != nil {
			return err
		}
	}
	return a.aut.despacho.AguardarOcioso(ctx)
}
