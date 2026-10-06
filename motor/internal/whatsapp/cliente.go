// Pacote whatsapp define a interface que o domínio usa para falar com o WhatsApp
// (contracts/cliente-whatsapp.md, Constituição IV). O domínio só conhece estes tipos; nunca
// types.JID, waE2E.* ou events.* do whatsmeow. Implementações: whatsapp/whatsmeow (real) e
// whatsapp/falso (memória).
package whatsapp

import (
	"context"
	"io"
	"time"
)

// Fabrica cria um cliente por conta; a real abre sessoes/<contaID>.db.
type Fabrica interface {
	Abrir(ctx context.Context, contaID string) (Cliente, error)
	Remover(ctx context.Context, contaID string) error // logout + apaga sessão
}

// Cliente é a conexão de uma conta.
type Cliente interface {
	// Conexão
	Conectar(ctx context.Context) error // restaura sessão ou inicia QR
	Desconectar()
	Pareado() bool // tem sessão salva
	CanalQR(ctx context.Context) (<-chan EventoQR, error)
	Eventos() <-chan Evento // todos os eventos normalizados (fechado ao desconectar)
	Proprio() (JID, bool)   // JID/telefone da conta

	// Consulta
	TemWhatsApp(ctx context.Context, telefoneE164 string) (JID, bool, error)
	Grupos(ctx context.Context) ([]InfoGrupo, error)
	Contatos(ctx context.Context) ([]InfoContato, error)

	// Envio (retornam o id do WhatsApp e o horário do servidor)
	EnviarTexto(ctx context.Context, para JID, texto string, citar *Citacao) (Enviada, error)
	EnviarMidia(ctx context.Context, para JID, m MidiaEnvio, citar *Citacao) (Enviada, error)
	Reagir(ctx context.Context, chat, remetente JID, waID, emoji string) error
	Editar(ctx context.Context, chat JID, waID, novoTexto string) error
	Apagar(ctx context.Context, chat, remetente JID, waID string) error
	MarcarLida(ctx context.Context, chat, remetente JID, waIDs []string, em time.Time) error

	// Mídia
	Baixar(ctx context.Context, chave ChaveDownload) (io.ReadCloser, error)
}
