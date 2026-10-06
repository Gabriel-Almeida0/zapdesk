package aplicacao

import (
	"context"

	"zapdesk/motor/internal/api"
	"zapdesk/motor/internal/arquivos"
	"zapdesk/motor/internal/chat"
	"zapdesk/motor/internal/contas"
	"zapdesk/motor/internal/disparos"
	"zapdesk/motor/internal/leads"
)

// servicos guarda os serviços montados.
type servicos struct {
	contas   *contas.Gerenciador
	chat     *chat.Servico
	arquivos *arquivos.Servico
	leads    *leads.Servico
	disparos *disparos.Servico
}

type contadores struct{ a *App }

func (c contadores) DisparosAtivos() int   { return c.a.disparosAtivos() }
func (c contadores) ContasConectadas() int { return c.a.sv.contas.ContasConectadas() }

// Contadores da feature 002 (GET /v1/sistema).
func (c contadores) AutomacoesAtivas() int  { return c.a.aut.servico.AutomacoesAtivas() }
func (c contadores) ProcessosIA() int       { return c.a.aut.servico.ProcessosIA() }
func (c contadores) RunnerDisponivel() bool { return c.a.aut.servico.RunnerDisponivel() }

// AguardarEventos permite às rotas /v1/falso/* esperarem o processamento.
func (c contadores) AguardarEventos(ctx context.Context, contaID string) error {
	return c.a.sv.contas.AguardarEventos(ctx, contaID)
}

// montarServicos cria os serviços de domínio.
func (a *App) montarServicos(ctx context.Context) (api.Servicos, error) {
	log := a.o.Log
	a.sv.arquivos = arquivos.Novo(a.Banco, a.o.PastaDados, a.Relogio)
	a.sv.contas = contas.Novo(ctx, a.Banco, a.Fabrica, a.Barramento, a.Relogio, log, a.o.PastaDados)
	a.sv.chat = chat.Novo(ctx, a.Banco, a.Barramento, a.Relogio, log, a.o.PastaDados, a.sv.contas)
	a.sv.chat.DefinirArquivos(a.sv.arquivos)
	a.sv.contas.DefinirConsumidor(a.sv.chat)
	if err := a.sv.chat.Iniciar(ctx); err != nil {
		return api.Servicos{}, err
	}
	a.sv.leads = leads.NovoServico(a.Banco, a.Barramento, a.Relogio)
	sv := api.Servicos{
		Contadores: contadores{a},
		Contas:     a.sv.contas,
		Chat:       a.sv.chat,
		Arquivos:   a.sv.arquivos,
		Leads:      a.sv.leads,
	}
	if err := a.montarDominio(ctx, &sv); err != nil {
		return api.Servicos{}, err
	}
	if err := a.montarAutomacoes(ctx, &sv); err != nil {
		return api.Servicos{}, err
	}
	a.apiServicos = sv
	return sv, nil
}

func (a *App) iniciarServicos() {
	a.sv.contas.Iniciar()
	a.iniciarDominio()
	a.iniciarAutomacoes(a.ctxApp)
}

func (a *App) encerrarServicos(ctx context.Context) {
	a.encerrarAutomacoes(ctx)
	a.encerrarDominio(ctx)
	a.sv.chat.Encerrar()
	a.sv.contas.Encerrar()
}

// Servicos expõe os serviços montados (testes e ferramentas internas).
func (a *App) Servicos() api.Servicos { return a.apiServicos }
