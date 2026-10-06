package acoes

import (
	"context"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/funil"
)

// funil executa mover_etapa e remover_do_funil (origem "automacao" no histórico; T066).
func (x *Executores) funil(ctx context.Context, c *Contexto, a modelo.Acao) Resultado {
	f, err := armazenamento.ObterFunil(ctx, x.d.Banco.L(), a.FunilID)
	if err != nil {
		return x.falhou(a.Tipo, "", "Funil não encontrado.")
	}
	origem := funil.Origem{Tipo: dominio.OrigemAutomacao, AutomacaoID: c.Automacao.ID}
	if c.Exec != nil {
		origem.ExecucaoID = c.Exec.ID()
	}
	if a.Tipo == modelo.AcaoRemoverDoFunil {
		if c.Alvo.LeadID == "" {
			return x.ok(a.Tipo, f.Nome, "O contato não estava no funil.")
		}
		if c.Simulacao {
			return x.simulada(a.Tipo, f.Nome, "Tiraria o lead do funil "+f.Nome+".")
		}
		if err := x.d.Funil.Remover(ctx, f.ID, c.Alvo.LeadID, origem); err != nil {
			return x.falhaOuFatal(a.Tipo, f.Nome, err)
		}
		return x.ok(a.Tipo, f.Nome, "")
	}
	var etapa *dominio.Etapa
	for i := range f.Etapas {
		if f.Etapas[i].ID == a.EtapaID {
			etapa = &f.Etapas[i]
		}
	}
	if etapa == nil {
		return x.falhou(a.Tipo, f.Nome, "Etapa não encontrada no funil "+f.Nome+".")
	}
	alvo := f.Nome + " › " + etapa.Nome
	if c.Simulacao {
		return x.simulada(a.Tipo, alvo, "Moveria o lead para "+alvo+".")
	}
	lead, err := x.garantirLead(ctx, c)
	if err != nil {
		return x.falhaOuFatal(a.Tipo, alvo, err)
	}
	if _, _, err := x.d.Funil.Mover(ctx, f.ID, funil.AlvoCard{LeadID: lead}, etapa.ID, origem); err != nil {
		return x.falhaOuFatal(a.Tipo, alvo, err)
	}
	return x.ok(a.Tipo, alvo, "")
}
