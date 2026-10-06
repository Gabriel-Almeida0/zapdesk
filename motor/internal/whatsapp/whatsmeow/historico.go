package whatsmeow

import (
	"context"

	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"zapdesk/motor/internal/whatsapp"
)

// tratarHistorico converte um lote do history sync (ParseWebMessage → mesmo caminho das
// mensagens ao vivo).
func (c *Cliente) tratarHistorico(e *events.HistorySync) {
	if e.Data == nil {
		return
	}
	ctx := context.Background()
	lote := whatsapp.LoteHistorico{Progresso: int(e.Data.GetProgress())}
	tipo := e.Data.GetSyncType()
	lote.Final = lote.Progresso >= 100 || tipo == waHistorySync.HistorySync_RECENT || tipo == waHistorySync.HistorySync_PUSH_NAME ||
		(tipo != waHistorySync.HistorySync_FULL && tipo != waHistorySync.HistorySync_INITIAL_BOOTSTRAP)

	for _, pn := range e.Data.GetPushnames() {
		jid, err := types.ParseJID(pn.GetID())
		if err != nil {
			continue
		}
		jid = c.resolver(ctx, jid, types.EmptyJID)
		if jid.Server == types.DefaultUserServer && pn.GetPushname() != "" {
			c.emitir(whatsapp.Evento{Contato: &whatsapp.InfoContato{JID: paraJID(jid), NomePush: pn.GetPushname()}})
		}
	}

	for _, conv := range e.Data.GetConversations() {
		chatBruto, err := types.ParseJID(conv.GetID())
		if err != nil || chatBruto.Server == types.NewsletterServer || chatBruto == types.StatusBroadcastJID {
			continue
		}
		chat := c.resolver(ctx, chatBruto, types.EmptyJID)
		ch := whatsapp.ConversaHistorico{Chat: paraJID(chat), Nome: conv.GetName(), NaoLidas: int(conv.GetUnreadCount())}
		if chat.Server == types.GroupServer && conv.GetName() != "" {
			c.mu.Lock()
			c.grupos[chat.String()] = conv.GetName()
			c.mu.Unlock()
		}
		for _, hm := range conv.GetMessages() {
			web := hm.GetMessage()
			if web == nil {
				continue
			}
			evt, err := c.cli.ParseWebMessage(chatBruto, web)
			if err != nil {
				continue
			}
			for _, ev := range c.converterMensagem(ctx, evt) {
				if ev.Mensagem != nil {
					if ev.Mensagem.Grupo && ev.Mensagem.NomeChat == "" {
						ev.Mensagem.NomeChat = conv.GetName()
					}
					ch.Mensagens = append(ch.Mensagens, *ev.Mensagem)
				}
			}
		}
		lote.Conversas = append(lote.Conversas, ch)
	}
	c.emitir(whatsapp.Evento{Historico: &lote})
}
