package aplicacao

import (
	"context"
	"time"

	"zapdesk/motor/internal/api"
	"zapdesk/motor/internal/automacoes"
	"zapdesk/motor/internal/automacoes/ponte"
	"zapdesk/motor/internal/automacoes/projetos"
	"zapdesk/motor/internal/automacoes/runner"
	"zapdesk/motor/internal/claude"
	"zapdesk/motor/internal/dominio"
)

// montarIA monta as peças das automações de IA (US3): projetos, pool de runners, ponte ctx.* e o
// serviço de IA, e os liga ao despachante, às ações, à fachada e ao cofre de segredos.
func (a *App) montarIA(ctx context.Context, sv *api.Servicos) error {
	p := &a.aut
	cfg := p.config.Configuracao()
	pool := runner.NovoPool(runner.Opcoes{Exec: a.o.RunnerExec, Script: a.o.RunnerScript, Relogio: a.Relogio, Log: a.o.Log,
		MaxProcessos: cfg.ProcessosIAMax, Ociosidade: time.Duration(cfg.OciosidadeIAMin) * time.Minute})
	proj := projetos.Novo(a.o.PastaDados, a.Relogio)
	iaSimulada := claude.Cliente(p.iaFalsa)
	if iaSimulada == nil {
		iaSimulada = claude.NovaFalsa(a.Relogio.Agora)
	}
	pt := ponte.Nova(ponte.Deps{Banco: a.Banco, Barramento: a.Barramento, Relogio: a.Relogio, Acoes: p.acoes, Chat: a.sv.chat,
		Organizacao: sv.Organizacao, Leads: a.sv.leads, Funil: p.funil, Portao: p.portao, Claude: p.claude, IASimulada: iaSimulada,
		ModeloPadrao: p.servico.ModeloPadrao, Esperas: p.agendador})
	ia := automacoes.NovaIA(p.servico, proj, pool, pt)
	p.despacho.DefinirExecutor("ia", ia)
	p.acoes.DefinirIA(ia)
	p.chatbot.DefinirIA(ia)
	p.simulador.DefinirIA(ia)
	p.cofre.Assinar(pool.SegredosAlterados)
	p.config.AoMudar(func(c dominio.ConfiguracaoAutomacoes) { ia.Configurar(c) })
	p.encerrarIA = ia.Encerrar
	sv.AutomacoesIA = ia
	return nil
}
