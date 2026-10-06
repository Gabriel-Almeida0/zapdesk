package chat

import (
	"context"
	"database/sql"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/fatos"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/whatsapp"
)

// EnviadaDisparo descreve uma mensagem enviada por um disparo.
type EnviadaDisparo struct {
	ContaID   string
	Para      whatsapp.JID
	Envio     whatsapp.Enviada
	Texto     string
	Tipo      string
	Midia     *whatsapp.MidiaInfo
	ArquivoID string
	DisparoID string
}

// GravarEnviadaTx grava a mensagem enviada por um disparo na conversa, dentro da transação do
// executor (junto com "enviado" do destinatário). Devolve ids para publicar depois do commit.
func (s *Servico) GravarEnviadaTx(ctx context.Context, tx *sql.Tx, e EnviadaDisparo) (mensagemID, conversaID string, err error) {
	agora := s.relogio.Agora()
	tel, _ := e.Para.Telefone()
	cid, _, err := armazenamento.GarantirContato(ctx, tx, e.ContaID, armazenamento.DadosContato{JID: e.Para.String(), Telefone: tel}, agora)
	if err != nil {
		return "", "", err
	}
	conversaID, _, err = armazenamento.GarantirConversa(ctx, tx, e.ContaID, e.Para.String(), "individual", "", cid, agora)
	if err != nil {
		return "", "", err
	}
	tipo := e.Tipo
	if tipo == "" {
		tipo = whatsapp.TipoTexto
	}
	nova := armazenamento.NovaMensagem{ID: ids.NovoEm(e.Envio.Em), ContaID: e.ContaID, ConversaID: conversaID, WaID: e.Envio.WaID,
		DeMim: true, Tipo: tipo, Texto: e.Texto, Midia: e.Midia, ArquivoID: e.ArquivoID, Estado: dominio.MsgEnviada,
		DisparoID: e.DisparoID, EnviadaEm: e.Envio.Em}
	if p, ok := s.contas.Proprio(e.ContaID); ok {
		nova.RemetenteJID = p.String()
	}
	inserida, err := armazenamento.InserirMensagem(ctx, tx, nova)
	if err != nil || !inserida {
		return "", conversaID, err
	}
	return nova.ID, conversaID, armazenamento.AtualizarUltimaMensagem(ctx, tx, conversaID, e.Envio.Em, resumo(tipo, e.Texto), 0)
}

// PublicarEnviada emite mensagem.nova e conversa.atualizada após o commit (e o fato de mensagem
// enviada por disparo, para o gatilho "sem resposta").
func (s *Servico) PublicarEnviada(ctx context.Context, mensagemID, conversaID string) {
	if mensagemID != "" {
		s.publicarMensagem(ctx, eventos.MensagemNova, mensagemID)
		if b, err := armazenamento.ObterMensagem(ctx, s.banco.L(), mensagemID); err == nil {
			f := fatos.Fato{Tipo: fatos.MensagemEnviada, ContaID: b.ContaID, ConversaID: b.ConversaID, MensagemID: b.ID,
				Texto: ptrStr(b.Texto), Em: b.EnviadaEm, Origem: fatos.OrigemDisparo}
			if b.DisparoID != nil {
				f.DisparoID = *b.DisparoID
			}
			s.fatos.Emitir(ctx, f)
		}
	}
	if conversaID != "" {
		s.publicarConversa(ctx, conversaID)
	}
}

// AchadaNoHistorico procura a mensagem de um disparo para o JID (reconciliação de "enviando").
func (s *Servico) AchadaNoHistorico(ctx context.Context, contaID, disparoID, jid string) (string, bool) {
	var waID string
	err := s.banco.L().QueryRowContext(ctx, `SELECT m.wa_id FROM mensagens m JOIN conversas cv ON cv.id = m.conversa_id
		WHERE m.conta_id = ? AND m.disparo_id = ? AND m.de_mim = 1 AND cv.jid = ? AND m.wa_id NOT LIKE 'pendente:%' LIMIT 1`,
		contaID, disparoID, jid).Scan(&waID)
	return waID, err == nil
}
