package disparos

import (
	"context"
	"time"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/fatos"
)

// acompanhamento recebe do chat os recibos e as respostas dos contatos.
type acompanhamento struct{ s *Servico }

// Recibo: entregue/lido sem regressão (lido antes de entregue preenche ambos).
func (a acompanhamento) Recibo(contaID, waID, tipo string, em time.Time) {
	ctx := context.Background()
	idsA, err := armazenamento.AplicarReciboDest(ctx, a.s.banco.E(), contaID, waID, tipo, em)
	if err != nil {
		a.s.log.Warn().Err(err).Msg("aplicar recibo ao destinatário")
		return
	}
	a.s.publicarAlterados(contaID, idsA)
}

// MensagemDoContato: resposta do número na mesma conta depois do envio → respondeu.
func (a acompanhamento) MensagemDoContato(contaID, telefone string, em time.Time) {
	ctx := context.Background()
	idsA, err := armazenamento.MarcarRespondeu(ctx, a.s.banco.E(), contaID, telefone, em)
	if err != nil {
		a.s.log.Warn().Err(err).Msg("marcar respondeu")
		return
	}
	a.s.publicarAlterados(contaID, idsA)
	for _, id := range idsA {
		d, err := armazenamento.ObterDestinatario(ctx, a.s.banco.L(), id)
		if err != nil {
			continue
		}
		f := fatos.Fato{Tipo: fatos.DisparoRespondeu, ContaID: contaID, LeadID: d.LeadID, DisparoID: d.DisparoID,
			DestinatarioID: d.ID, Em: em}
		if d.JID != nil {
			if c, ok, _ := armazenamento.ConversaPorJID(ctx, a.s.banco.L(), contaID, *d.JID); ok {
				f.ConversaID = c.ID
				if c.ContatoID != nil {
					f.ContatoID = *c.ContatoID
				}
			}
		}
		a.s.fatos.Emitir(ctx, f)
	}
}

func (s *Servico) publicarAlterados(contaID string, idsA []string) {
	disparosTocados := map[string]bool{}
	for _, id := range idsA {
		d, err := armazenamento.ObterDestinatario(context.Background(), s.banco.L(), id)
		if err != nil {
			continue
		}
		s.barramento.Publicar(eventos.DestinatarioAtualizado, contaID, d)
		disparosTocados[d.DisparoID] = true
	}
	for id := range disparosTocados {
		s.publicarDisparo(id, false)
	}
}
