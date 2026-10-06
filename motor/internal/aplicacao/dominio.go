package aplicacao

import (
	"context"

	"zapdesk/motor/internal/api"
	"zapdesk/motor/internal/disparos"
	"zapdesk/motor/internal/organizacao"
)

// SementeDisparos (testes): semente do aleatório dos intervalos; 0 = aleatória.
var SementeDisparos int64

// montarDominio monta disparos e organização (US3+). Disparos ativos viram "pausado" aqui,
// antes do "pronto" (contracts/runtime.md).
func (a *App) montarDominio(ctx context.Context, sv *api.Servicos) error {
	a.sv.disparos = disparos.Novo(ctx, a.Banco, a.Barramento, a.Relogio, a.o.Log, a.sv.contas, a.sv.chat, a.sv.leads,
		disparos.Opcoes{Semente: SementeDisparos})
	if err := a.sv.disparos.Preparar(ctx, a.o.Religado); err != nil {
		return err
	}
	sv.Disparos = a.sv.disparos
	sv.Organizacao = organizacao.Novo(a.Banco, a.Barramento, a.Relogio, a.sv.chat)
	return nil
}

func (a *App) iniciarDominio() {}

func (a *App) encerrarDominio(ctx context.Context) {
	a.sv.disparos.Encerrar(ctx)
}

func (a *App) disparosAtivos() int { return a.sv.disparos.DisparosAtivos() }

func (a *App) energia(evento string) { a.sv.disparos.Energia(evento) }
