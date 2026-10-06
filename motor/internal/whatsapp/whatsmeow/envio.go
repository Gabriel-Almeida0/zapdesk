package whatsmeow

import (
	"context"
	"errors"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	"zapdesk/motor/internal/whatsapp"
)

func contextoCitacao(citar *whatsapp.Citacao) *waE2E.ContextInfo {
	if citar == nil || citar.WaID == "" {
		return nil
	}
	ci := &waE2E.ContextInfo{
		StanzaID:      proto.String(citar.WaID),
		QuotedMessage: &waE2E.Message{Conversation: proto.String(citar.Texto)},
	}
	if !citar.Remetente.Vazio() {
		ci.Participant = proto.String(deJID(citar.Remetente).String())
	}
	return ci
}

// EnviarTexto envia texto (com citação opcional).
func (c *Cliente) EnviarTexto(ctx context.Context, para whatsapp.JID, texto string, citar *whatsapp.Citacao) (whatsapp.Enviada, error) {
	msg := &waE2E.Message{Conversation: proto.String(texto)}
	if ci := contextoCitacao(citar); ci != nil {
		msg = &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String(texto), ContextInfo: ci}}
	}
	return c.enviar(ctx, para, msg)
}

func (c *Cliente) enviar(ctx context.Context, para whatsapp.JID, msg *waE2E.Message) (whatsapp.Enviada, error) {
	if !c.cli.IsConnected() {
		return whatsapp.Enviada{}, whatsapp.ErrDesconectado
	}
	resp, err := c.cli.SendMessage(ctx, deJID(para), msg)
	if err != nil {
		// tempo esgotado esperando a confirmação: a mensagem pode ter saído
		if errors.Is(err, whatsmeow.ErrMessageTimedOut) || errors.Is(err, context.DeadlineExceeded) {
			return whatsapp.Enviada{}, errors.Join(whatsapp.ErrEstadoIncerto, err)
		}
		return whatsapp.Enviada{}, traduzirErro(err)
	}
	em := resp.Timestamp
	if em.IsZero() {
		em = time.Now()
	}
	return whatsapp.Enviada{WaID: resp.ID, Em: em}, nil
}

// TemWhatsApp consulta um único número (IsOnWhatsApp com 1 telefone).
func (c *Cliente) TemWhatsApp(ctx context.Context, telefoneE164 string) (whatsapp.JID, bool, error) {
	if !c.cli.IsConnected() {
		return whatsapp.JID{}, false, whatsapp.ErrDesconectado
	}
	resp, err := c.cli.IsOnWhatsApp(ctx, []string{telefoneE164})
	if err != nil {
		return whatsapp.JID{}, false, traduzirErro(err)
	}
	if len(resp) == 0 || !resp[0].IsIn {
		return whatsapp.JID{}, false, nil
	}
	r := resp[0]
	switch {
	case !r.PhoneNumber.IsEmpty() && r.PhoneNumber.Server == types.DefaultUserServer:
		return paraJID(r.PhoneNumber), true, nil
	case r.JID.Server == types.DefaultUserServer:
		return paraJID(r.JID), true, nil
	}
	return whatsapp.JIDDeTelefone(telefoneE164), true, nil
}

// Grupos lista os grupos da conta (e guarda os nomes para as mensagens).
func (c *Cliente) Grupos(ctx context.Context) ([]whatsapp.InfoGrupo, error) {
	grupos, err := c.cli.GetJoinedGroups(ctx)
	if err != nil {
		return nil, traduzirErro(err)
	}
	lista := make([]whatsapp.InfoGrupo, 0, len(grupos))
	c.mu.Lock()
	for _, g := range grupos {
		c.grupos[g.JID.String()] = g.Name
		lista = append(lista, whatsapp.InfoGrupo{JID: paraJID(g.JID), Nome: g.Name})
	}
	c.mu.Unlock()
	return lista, nil
}

// Contatos lista a agenda sincronizada (somente contatos com telefone conhecido).
func (c *Cliente) Contatos(ctx context.Context) ([]whatsapp.InfoContato, error) {
	todos, err := c.cli.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		return nil, err
	}
	lista := make([]whatsapp.InfoContato, 0, len(todos))
	for jid, info := range todos {
		jid = c.resolver(ctx, jid, types.EmptyJID)
		if jid.Server != types.DefaultUserServer {
			continue
		}
		nome := info.FullName
		if nome == "" {
			nome = info.FirstName
		}
		lista = append(lista, whatsapp.InfoContato{JID: paraJID(jid), Nome: nome, NomePush: info.PushName})
	}
	return lista, nil
}

// MarcarLida envia recibo de leitura.
func (c *Cliente) MarcarLida(ctx context.Context, chat, remetente whatsapp.JID, waIDs []string, em time.Time) error {
	ids := make([]types.MessageID, len(waIDs))
	copy(ids, waIDs)
	return traduzirErro(c.cli.MarkRead(ctx, ids, em, deJID(chat), deJID(remetente)))
}

var errTipoMidia = errors.New("tipo de mídia desconhecido")
