package automacoes

import (
	"context"
	"fmt"

	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/dominio"
)

// avisos lista referências quebradas (ids que não existem mais) sem impedir salvar (FR-021): a
// ação correspondente falha em execução e o editor mostra o aviso.
func (s *Servico) avisos(ctx context.Context, a dominio.Automacao) []dominio.ErroDefinicao {
	r := &refs{s: s, ctx: ctx, l: []dominio.ErroDefinicao{}}
	for i, c := range a.Contas {
		r.existe(fmt.Sprintf("contas[%d]", i), "contas", c, "Conta não encontrada.")
	}
	if a.ContaEnvioID != nil {
		r.existe("conta_envio_id", "contas", *a.ContaEnvioID, "Conta de envio não encontrada.")
	}
	gs, _ := modelo.DecodificarGatilhos(a.Gatilhos)
	for i, g := range gs {
		p := fmt.Sprintf("gatilhos[%d]", i)
		switch g.Tipo {
		case modelo.GatilhoEtiqueta:
			r.existe(p+".etiqueta_id", "etiquetas", g.EtiquetaID, "Etiqueta não encontrada.")
		case modelo.GatilhoEntrouEtapa:
			r.etapa(p, g.FunilID, g.EtapaID)
		case modelo.GatilhoDisparoRespondeu:
			if g.DisparoID != nil && *g.DisparoID != "" {
				r.existe(p+".disparo_id", "disparos", *g.DisparoID, "Disparo não encontrado.")
			}
		}
	}
	switch a.Tipo {
	case modelo.TipoFluxo:
		if d, errs := modelo.DecodificarFluxo(a.Definicao); errs == nil {
			r.condicoes("definicao.condicoes", d.Condicoes)
			for i, ac := range d.Acoes {
				id := ac.ID
				r.acao(fmt.Sprintf("definicao.acoes[%d]", i), ac, nil, &id)
			}
		}
	case modelo.TipoChatbot:
		if d, errs := modelo.DecodificarChatbot(a.Definicao); errs == nil {
			for i, n := range d.Nos {
				p := fmt.Sprintf("definicao.nos[%d]", i)
				id := n.ID
				if n.TemplateID != nil && *n.TemplateID != "" {
					r.com(&id, nil).existe(p+".template_id", "templates", *n.TemplateID, "Template não encontrado.")
				}
				if n.Acao != nil {
					r.acao(p+".acao", *n.Acao, &id, nil)
				}
				if n.Tipo == modelo.NoIA {
					r.com(&id, nil).automacao(p+".automacao_id", n.AutomacaoID, modelo.TipoIA)
				}
				for j, rm := range n.Ramos {
					cd := rm.Condicoes
					r.com(&id, nil).condicoes(fmt.Sprintf("%s.ramos[%d].condicoes", p, j), &cd)
				}
			}
		}
	}
	return r.l
}

type refs struct {
	s      *Servico
	ctx    context.Context
	l      []dominio.ErroDefinicao
	noID   *string
	acaoID *string
	pai    *refs
}

// com devolve um coletor que grava no mesmo resultado com nó/ação de contexto.
func (r *refs) com(no, acao *string) *refs {
	raiz := r
	for raiz.pai != nil {
		raiz = raiz.pai
	}
	return &refs{s: r.s, ctx: r.ctx, noID: no, acaoID: acao, pai: raiz}
}

func (r *refs) add(caminho, msg string) {
	e := dominio.ErroDefinicao{Caminho: caminho, NoID: r.noID, AcaoID: r.acaoID, Mensagem: msg}
	if r.pai != nil {
		r.pai.l = append(r.pai.l, e)
		return
	}
	r.l = append(r.l, e)
}

func (r *refs) existe(caminho, tabela, id, msg string) bool {
	if id == "" {
		return false
	}
	var n int
	r.s.d.Banco.L().QueryRowContext(r.ctx, `SELECT count(*) FROM `+tabela+` WHERE id = ?`, id).Scan(&n)
	if n == 0 {
		r.add(caminho, msg)
		return false
	}
	return true
}

func (r *refs) etapa(p, funilID, etapaID string) {
	if !r.existe(p+".funil_id", "funis", funilID, "Funil não encontrado.") {
		return
	}
	if etapaID == "" {
		return
	}
	var f string
	r.s.d.Banco.L().QueryRowContext(r.ctx, `SELECT funil_id FROM etapas WHERE id = ?`, etapaID).Scan(&f)
	if f != funilID {
		r.add(p+".etapa_id", "Etapa não encontrada neste funil.")
	}
}

func (r *refs) automacao(caminho, id, tipo string) {
	if id == "" {
		return
	}
	var t string
	r.s.d.Banco.L().QueryRowContext(r.ctx, `SELECT tipo FROM automacoes WHERE id = ?`, id).Scan(&t)
	switch {
	case t == "":
		r.add(caminho, "Automação não encontrada.")
	case t != tipo:
		nomes := map[string]string{modelo.TipoChatbot: "um chatbot", modelo.TipoIA: "uma automação de IA"}
		r.add(caminho, "Escolha "+nomes[tipo]+".")
	}
}

func (r *refs) condicoes(p string, c *modelo.Condicoes) {
	if c == nil {
		return
	}
	for i, rg := range c.Regras {
		pr := fmt.Sprintf("%s.regras[%d]", p, i)
		switch rg.Tipo {
		case modelo.RegraEtiqueta:
			r.existe(pr+".etiqueta_id", "etiquetas", rg.EtiquetaID, "Etiqueta não encontrada.")
		case modelo.RegraEtapa:
			e := ""
			if rg.EtapaID != nil {
				e = *rg.EtapaID
			}
			r.etapa(pr, rg.FunilID, e)
		case modelo.RegraConta:
			for j, c := range rg.ContaIDs {
				r.existe(fmt.Sprintf("%s.conta_ids[%d]", pr, j), "contas", c, "Conta não encontrada.")
			}
		}
	}
}

func (r *refs) acao(p string, a modelo.Acao, noID, acaoID *string) {
	x := r.com(noID, acaoID)
	switch a.Tipo {
	case modelo.AcaoEnviarTemplate:
		x.existe(p+".template_id", "templates", a.TemplateID, "Template não encontrado.")
	case modelo.AcaoAdicionarEtiqueta, modelo.AcaoRemoverEtiqueta:
		x.existe(p+".etiqueta_id", "etiquetas", a.EtiquetaID, "Etiqueta não encontrada.")
	case modelo.AcaoMoverEtapa:
		x.etapa(p, a.FunilID, a.EtapaID)
	case modelo.AcaoRemoverDoFunil:
		x.existe(p+".funil_id", "funis", a.FunilID, "Funil não encontrado.")
	case modelo.AcaoIniciarChatbot:
		x.automacao(p+".automacao_id", a.AutomacaoID, modelo.TipoChatbot)
	case modelo.AcaoExecutarIA:
		x.automacao(p+".automacao_id", a.AutomacaoID, modelo.TipoIA)
	case modelo.AcaoAdicionarADisparo:
		x.existe(p+".disparo_id", "disparos", a.DisparoID, "Disparo não encontrado.")
	}
}
