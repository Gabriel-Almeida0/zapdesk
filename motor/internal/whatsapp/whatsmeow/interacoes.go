package whatsmeow

import (
	"context"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"

	"zapdesk/motor/internal/whatsapp"
)

// Reagir envia (ou remove, com emoji vazio) uma reação.
func (c *Cliente) Reagir(ctx context.Context, chat, remetente whatsapp.JID, waID, emoji string) error {
	_, err := c.enviar(ctx, chat, c.cli.BuildReaction(deJID(chat), deJID(remetente), waID, emoji))
	return err
}

// Editar troca o texto de uma mensagem minha.
func (c *Cliente) Editar(ctx context.Context, chat whatsapp.JID, waID, novoTexto string) error {
	_, err := c.enviar(ctx, chat, c.cli.BuildEdit(deJID(chat), waID, &waE2E.Message{Conversation: proto.String(novoTexto)}))
	return err
}

// Apagar apaga para todos (mensagem minha: remetente vazio ou o meu JID).
func (c *Cliente) Apagar(ctx context.Context, chat, remetente whatsapp.JID, waID string) error {
	_, err := c.enviar(ctx, chat, c.cli.BuildRevoke(deJID(chat), deJID(remetente), waID))
	return err
}
