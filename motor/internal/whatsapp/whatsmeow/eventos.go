package whatsmeow

import (
	"context"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"zapdesk/motor/internal/whatsapp"
)

// fontes resolve chat e remetente (LID → telefone).
func (c *Cliente) fontes(ctx context.Context, info types.MessageSource) (chat, remetente types.JID) {
	remetente = c.resolver(ctx, info.Sender, info.SenderAlt)
	chat = info.Chat.ToNonAD()
	if !info.IsGroup && chat.Server == types.HiddenUserServer {
		alt := info.SenderAlt
		if info.IsFromMe {
			alt = info.RecipientAlt
		}
		chat = c.resolver(ctx, chat, alt)
	}
	return chat, remetente
}

func (c *Cliente) tratarMensagem(e *events.Message) {
	if e.Info.Chat.Server == types.NewsletterServer || e.Message == nil {
		return
	}
	for _, ev := range c.converterMensagem(context.Background(), e) {
		c.emitir(ev)
	}
}

// converterMensagem transforma um events.Message em zero ou mais eventos do domínio.
func (c *Cliente) converterMensagem(ctx context.Context, e *events.Message) []whatsapp.Evento {
	chat, remetente := c.fontes(ctx, e.Info.MessageSource)
	msg := e.Message
	em := e.Info.Timestamp

	if p := msg.GetProtocolMessage(); p != nil {
		alvo := p.GetKey().GetID()
		switch p.GetType() {
		case waE2E.ProtocolMessage_REVOKE:
			return []whatsapp.Evento{{Revogacao: &whatsapp.Revogacao{Chat: paraJID(chat), Remetente: paraJID(remetente), WaIDAlvo: alvo, Em: em}}}
		case waE2E.ProtocolMessage_MESSAGE_EDIT:
			tipo, texto, _, _ := converterConteudo(p.GetEditedMessage())
			if tipo == "" {
				return nil
			}
			return []whatsapp.Evento{{Edicao: &whatsapp.Edicao{Chat: paraJID(chat), Remetente: paraJID(remetente), WaIDAlvo: alvo, NovoTexto: texto, Em: em}}}
		}
		return nil
	}
	if r := msg.GetReactionMessage(); r != nil {
		return []whatsapp.Evento{{Reacao: &whatsapp.Reacao{Chat: paraJID(chat), Remetente: paraJID(remetente), DeMim: e.Info.IsFromMe,
			WaIDAlvo: r.GetKey().GetID(), Emoji: r.GetText(), Em: em}}}
	}
	if e.IsEdit {
		tipo, texto, _, _ := converterConteudo(msg)
		if tipo == "" {
			return nil
		}
		return []whatsapp.Evento{{Edicao: &whatsapp.Edicao{Chat: paraJID(chat), Remetente: paraJID(remetente), WaIDAlvo: e.Info.ID, NovoTexto: texto, Em: em}}}
	}

	tipo, texto, midia, citacao := converterConteudo(msg)
	if tipo == "" {
		return nil
	}
	if midia != nil {
		midia.Chave.WaID = e.Info.ID
		midia.Chave.Chat = e.Info.Chat.String()
		midia.Chave.Remetente = e.Info.Sender.String()
		midia.Chave.DeMim = e.Info.IsFromMe
		midia.Chave.Grupo = e.Info.IsGroup
	}
	if citacao != nil && citacao.Remetente.Servidor == types.HiddenUserServer {
		citacao.Remetente = paraJID(c.resolver(ctx, deJID(citacao.Remetente), types.EmptyJID))
	}
	m := &whatsapp.MensagemRecebida{
		WaID: e.Info.ID, TipoMsg: tipo, Texto: texto, Chat: paraJID(chat), Remetente: paraJID(remetente),
		NomeRemetente: e.Info.PushName, DeMim: e.Info.IsFromMe, Grupo: e.Info.IsGroup, Midia: midia, Citacao: citacao, Em: em,
	}
	if e.Info.Chat == types.StatusBroadcastJID {
		m.Chat = whatsapp.JIDStatus
	}
	if m.Grupo {
		c.mu.Lock()
		m.NomeChat = c.grupos[chat.String()]
		c.mu.Unlock()
	}
	return []whatsapp.Evento{{Mensagem: m}}
}

// converterConteudo extrai tipo, texto, mídia e citação. Tipo vazio = mensagem ignorada.
func converterConteudo(msg *waE2E.Message) (tipo, texto string, midia *whatsapp.MidiaInfo, citacao *whatsapp.Citacao) {
	if msg == nil {
		return "", "", nil, nil
	}
	var ctxInfo *waE2E.ContextInfo
	switch {
	case msg.GetConversation() != "":
		tipo, texto = whatsapp.TipoTexto, msg.GetConversation()
	case msg.GetExtendedTextMessage() != nil:
		t := msg.GetExtendedTextMessage()
		tipo, texto, ctxInfo = whatsapp.TipoTexto, t.GetText(), t.GetContextInfo()
	case msg.GetImageMessage() != nil:
		m := msg.GetImageMessage()
		tipo, texto, ctxInfo = whatsapp.TipoImagem, m.GetCaption(), m.GetContextInfo()
		midia = &whatsapp.MidiaInfo{Mimetype: m.GetMimetype(), Tamanho: int64(m.GetFileLength()), Largura: int(m.GetWidth()),
			Altura: int(m.GetHeight()), Miniatura: m.GetJPEGThumbnail(),
			Chave: chave(tipo, m.GetURL(), m.GetDirectPath(), m.GetMediaKey(), m.GetFileSHA256(), m.GetFileEncSHA256(), m.GetFileLength(), m.GetMimetype())}
	case msg.GetVideoMessage() != nil:
		m := msg.GetVideoMessage()
		tipo, texto, ctxInfo = whatsapp.TipoVideo, m.GetCaption(), m.GetContextInfo()
		midia = &whatsapp.MidiaInfo{Mimetype: m.GetMimetype(), Tamanho: int64(m.GetFileLength()), DuracaoS: int(m.GetSeconds()),
			Largura: int(m.GetWidth()), Altura: int(m.GetHeight()), Miniatura: m.GetJPEGThumbnail(),
			Chave: chave(tipo, m.GetURL(), m.GetDirectPath(), m.GetMediaKey(), m.GetFileSHA256(), m.GetFileEncSHA256(), m.GetFileLength(), m.GetMimetype())}
	case msg.GetAudioMessage() != nil:
		m := msg.GetAudioMessage()
		tipo, ctxInfo = whatsapp.TipoAudio, m.GetContextInfo()
		midia = &whatsapp.MidiaInfo{Mimetype: m.GetMimetype(), Tamanho: int64(m.GetFileLength()), DuracaoS: int(m.GetSeconds()), PTT: m.GetPTT(),
			Chave: chave(tipo, m.GetURL(), m.GetDirectPath(), m.GetMediaKey(), m.GetFileSHA256(), m.GetFileEncSHA256(), m.GetFileLength(), m.GetMimetype())}
	case msg.GetDocumentMessage() != nil:
		m := msg.GetDocumentMessage()
		tipo, texto, ctxInfo = whatsapp.TipoDocumento, m.GetCaption(), m.GetContextInfo()
		nome := m.GetFileName()
		if nome == "" {
			nome = m.GetTitle()
		}
		midia = &whatsapp.MidiaInfo{Mimetype: m.GetMimetype(), Tamanho: int64(m.GetFileLength()), NomeArquivo: nome, Miniatura: m.GetJPEGThumbnail(),
			Chave: chave(tipo, m.GetURL(), m.GetDirectPath(), m.GetMediaKey(), m.GetFileSHA256(), m.GetFileEncSHA256(), m.GetFileLength(), m.GetMimetype())}
	case msg.GetStickerMessage() != nil:
		m := msg.GetStickerMessage()
		tipo, ctxInfo = whatsapp.TipoFigurinha, m.GetContextInfo()
		midia = &whatsapp.MidiaInfo{Mimetype: m.GetMimetype(), Tamanho: int64(m.GetFileLength()), Largura: int(m.GetWidth()), Altura: int(m.GetHeight()),
			Chave: chave(tipo, m.GetURL(), m.GetDirectPath(), m.GetMediaKey(), m.GetFileSHA256(), m.GetFileEncSHA256(), m.GetFileLength(), m.GetMimetype())}
	default:
		return "", "", nil, nil
	}
	if ctxInfo != nil && ctxInfo.GetStanzaID() != "" {
		_, resumo, _, _ := converterConteudo(ctxInfo.GetQuotedMessage())
		rem, _ := types.ParseJID(ctxInfo.GetParticipant())
		citacao = &whatsapp.Citacao{WaID: ctxInfo.GetStanzaID(), Remetente: paraJID(rem), Texto: resumo}
	}
	return tipo, texto, midia, citacao
}

func chave(tipo, url, caminho string, mediaKey, sha, encSha []byte, tamanho uint64, mime string) whatsapp.ChaveDownload {
	return whatsapp.ChaveDownload{Tipo: tipo, URL: url, DirectPath: caminho, MediaKey: mediaKey, FileSHA256: sha,
		FileEncSHA256: encSha, Tamanho: int64(tamanho), Mimetype: mime}
}

func (c *Cliente) tratarRecibo(e *events.Receipt) {
	if e.IsFromMe {
		return // recibos dos meus outros aparelhos (read-self etc.)
	}
	tipo := ""
	switch e.Type {
	case types.ReceiptTypeDelivered:
		tipo = whatsapp.ReciboEntregue
	case types.ReceiptTypeRead:
		tipo = whatsapp.ReciboLido
	case types.ReceiptTypePlayed:
		tipo = whatsapp.ReciboTocado
	default:
		return
	}
	chat, remetente := c.fontes(context.Background(), e.MessageSource)
	c.emitir(whatsapp.Evento{Recibo: &whatsapp.Recibo{Tipo: tipo, Chat: paraJID(chat), Remetente: paraJID(remetente),
		WaIDs: append([]string(nil), e.MessageIDs...), Em: e.Timestamp}})
}
