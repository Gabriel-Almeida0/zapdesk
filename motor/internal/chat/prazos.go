package chat

import (
	"time"

	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/whatsapp"
)

// Prazos do WhatsApp (data-model.md › Mensagem; research.md §1). Únicos lugares a mudar.
var (
	PrazoEditar = 15 * time.Minute
	PrazoApagar = 48 * time.Hour
)

func enviadaComSucesso(m dominio.Mensagem) bool {
	switch m.Estado {
	case dominio.MsgEnviada, dominio.MsgEntregue, dominio.MsgLida:
		return true
	}
	return false
}

// PodeEditar: só mensagem de texto minha, não apagada, até 15 min após o envio.
func PodeEditar(m dominio.Mensagem, agora time.Time) bool {
	return m.DeMim && !m.Apagada && m.Tipo == whatsapp.TipoTexto && enviadaComSucesso(m) && agora.Sub(m.EnviadaEm) <= PrazoEditar
}

// PodeApagar: só mensagem minha, não apagada, até 48 h após o envio.
func PodeApagar(m dominio.Mensagem, agora time.Time) bool {
	return m.DeMim && !m.Apagada && enviadaComSucesso(m) && agora.Sub(m.EnviadaEm) <= PrazoApagar
}
