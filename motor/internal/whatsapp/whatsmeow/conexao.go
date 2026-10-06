package whatsmeow

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"zapdesk/motor/internal/whatsapp"
)

// Cliente adapta *whatsmeow.Client para whatsapp.Cliente.
type Cliente struct {
	contaID   string
	container *sqlstore.Container
	cli       *whatsmeow.Client
	log       zerolog.Logger

	mu         sync.Mutex
	eventos    chan whatsapp.Evento
	fechado    bool
	jaConectou bool
	caiu       bool
	grupos     map[string]string
}

var _ whatsapp.Cliente = (*Cliente)(nil)

// emitir entrega um evento normalizado (descarta depois de Desconectar).
func (c *Cliente) emitir(e whatsapp.Evento) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fechado {
		return
	}
	select {
	case c.eventos <- e:
	case <-time.After(30 * time.Second):
		c.log.Error().Msg("fila de eventos cheia; evento descartado")
	}
}

// Conectar restaura a sessão ou, sem sessão, inicia o pareamento (chame CanalQR antes).
func (c *Cliente) Conectar(ctx context.Context) error {
	return traduzirErro(c.cli.ConnectContext(ctx))
}

// Desconectar fecha a conexão, o banco da sessão e o canal de eventos.
func (c *Cliente) Desconectar() {
	c.cli.Disconnect()
	c.mu.Lock()
	if !c.fechado {
		c.fechado = true
		close(c.eventos)
	}
	c.mu.Unlock()
	c.container.Close()
}

// Pareado indica sessão salva.
func (c *Cliente) Pareado() bool { return c.cli.Store.ID != nil }

// CanalQR converte o canal de QR do whatsmeow.
func (c *Cliente) CanalQR(ctx context.Context) (<-chan whatsapp.EventoQR, error) {
	origem, err := c.cli.GetQRChannel(ctx)
	if err != nil {
		return nil, err
	}
	destino := make(chan whatsapp.EventoQR, 8)
	go func() {
		defer close(destino)
		for item := range origem {
			switch item.Event {
			case whatsmeow.QRChannelEventCode:
				destino <- whatsapp.EventoQR{Tipo: whatsapp.QRCodigo, Codigo: item.Code, Expira: item.Timeout}
			case whatsmeow.QRChannelSuccess.Event:
				destino <- whatsapp.EventoQR{Tipo: whatsapp.QRSucesso}
			case whatsmeow.QRChannelTimeout.Event:
				destino <- whatsapp.EventoQR{Tipo: whatsapp.QRExpirado}
			case whatsmeow.QRChannelEventPasskeyRequest, whatsmeow.QRChannelEventPasskeyResponse:
				// pareamento por passkey fica fora do MVP
			default:
				erro := item.Error
				if erro == nil {
					erro = errors.New(item.Event)
				}
				destino <- whatsapp.EventoQR{Tipo: whatsapp.QRErro, Erro: erro}
			}
		}
	}()
	return destino, nil
}

// Eventos devolve o canal de eventos normalizados.
func (c *Cliente) Eventos() <-chan whatsapp.Evento { return c.eventos }

// Proprio devolve o JID (telefone) da conta.
func (c *Cliente) Proprio() (whatsapp.JID, bool) {
	if c.cli.Store.ID == nil {
		return whatsapp.JID{}, false
	}
	return paraJID(c.cli.Store.ID.ToNonAD()), true
}

// tratar recebe os eventos brutos do whatsmeow e os normaliza.
func (c *Cliente) tratar(evt any) {
	switch e := evt.(type) {
	case *events.Connected:
		c.mu.Lock()
		primeiro := !c.jaConectou
		c.jaConectou, c.caiu = true, false
		c.mu.Unlock()
		go func() {
			ctx, cancelar := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancelar()
			// sem presença "disponível" o nome aparece como "-" para os contatos
			if err := c.cli.SendPresence(ctx, types.PresenceAvailable); err != nil {
				c.log.Debug().Err(err).Msg("enviar presença")
			}
		}()
		if primeiro {
			proprio, _ := c.Proprio()
			c.emitir(whatsapp.Evento{Estado: &whatsapp.EventoEstado{Estado: whatsapp.EstadoConectada, Proprio: proprio, PushName: c.cli.Store.PushName}})
		} else {
			c.emitir(whatsapp.Evento{Estado: &whatsapp.EventoEstado{Estado: whatsapp.EstadoRedeVoltou}})
		}
	case *events.KeepAliveRestored:
		c.mu.Lock()
		caiu := c.caiu
		c.caiu = false
		c.mu.Unlock()
		if caiu {
			c.emitir(whatsapp.Evento{Estado: &whatsapp.EventoEstado{Estado: whatsapp.EstadoRedeVoltou}})
		}
	case *events.Disconnected, *events.KeepAliveTimeout:
		c.mu.Lock()
		avisar := c.jaConectou && !c.caiu
		c.caiu = true
		c.mu.Unlock()
		if avisar {
			c.emitir(whatsapp.Evento{Estado: &whatsapp.EventoEstado{Estado: whatsapp.EstadoRedeCaiu}})
		}
	case *events.LoggedOut:
		c.emitir(whatsapp.Evento{Estado: &whatsapp.EventoEstado{Estado: whatsapp.EstadoDesconectada, Motivo: e.Reason.String()}})
	case *events.StreamReplaced:
		c.emitir(whatsapp.Evento{Estado: &whatsapp.EventoEstado{Estado: whatsapp.EstadoSubstituida, Motivo: "sessão aberta em outro lugar"}})
	case *events.TemporaryBan:
		c.emitir(whatsapp.Evento{Estado: &whatsapp.EventoEstado{Estado: whatsapp.EstadoBanida, Motivo: e.String()}})
	case *events.ConnectFailure:
		if e.Reason == events.ConnectFailureTempBanned {
			c.emitir(whatsapp.Evento{Estado: &whatsapp.EventoEstado{Estado: whatsapp.EstadoBanida, Motivo: e.Reason.String()}})
		}
	case *events.ClientOutdated:
		c.emitir(whatsapp.Evento{Estado: &whatsapp.EventoEstado{Estado: whatsapp.EstadoDesconectada, Motivo: "versão do cliente desatualizada"}})
	case *events.Message:
		c.tratarMensagem(e)
	case *events.Receipt:
		c.tratarRecibo(e)
	case *events.HistorySync:
		c.tratarHistorico(e)
	case *events.PushName:
		jid := c.resolver(context.Background(), e.JID, e.JIDAlt)
		if jid.Server == types.DefaultUserServer {
			c.emitir(whatsapp.Evento{Contato: &whatsapp.InfoContato{JID: paraJID(jid), NomePush: e.NewPushName}})
		}
	case *events.Contact:
		jid := c.resolver(context.Background(), e.JID, types.EmptyJID)
		if jid.Server == types.DefaultUserServer && e.Action != nil {
			nome := e.Action.GetFullName()
			if nome == "" {
				nome = e.Action.GetFirstName()
			}
			c.emitir(whatsapp.Evento{Contato: &whatsapp.InfoContato{JID: paraJID(jid), Nome: nome}})
		}
	}
}

// traduzirErro converte erros do whatsmeow nos sentinelas do domínio. Erros de rede e de tempo
// esgotado viram ErrDesconectado (o disparo aguarda em vez de marcar falhou).
func traduzirErro(err error) error {
	if err == nil {
		return nil
	}
	var desconectado *whatsmeow.DisconnectedError
	var erroRede net.Error
	switch {
	case errors.Is(err, whatsmeow.ErrNotConnected), errors.Is(err, whatsmeow.ErrNotLoggedIn),
		errors.Is(err, whatsmeow.ErrIQTimedOut), errors.Is(err, whatsmeow.ErrMessageTimedOut),
		errors.Is(err, context.DeadlineExceeded), errors.As(err, &desconectado), errors.As(err, &erroRede):
		return errors.Join(whatsapp.ErrDesconectado, err)
	case errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith404), errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith410),
		errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith403):
		return errors.Join(whatsapp.ErrMidiaExpirada, err)
	}
	return err
}

// paraJID converte o JID do whatsmeow no JID do domínio.
func paraJID(j types.JID) whatsapp.JID {
	j = j.ToNonAD()
	return whatsapp.JID{Usuario: j.User, Servidor: j.Server}
}

// deJID converte o JID do domínio no do whatsmeow.
func deJID(j whatsapp.JID) types.JID { return types.NewJID(j.Usuario, j.Servidor) }

// resolver troca LID pelo JID de telefone quando conhecido (via JID alternativo ou LIDStore);
// o domínio nunca vê LID quando o telefone é conhecido (research.md §1).
func (c *Cliente) resolver(ctx context.Context, jid, alt types.JID) types.JID {
	jid = jid.ToNonAD()
	if jid.Server != types.HiddenUserServer {
		return jid
	}
	if !alt.IsEmpty() && alt.Server == types.DefaultUserServer {
		return alt.ToNonAD()
	}
	if c.cli.Store != nil && c.cli.Store.LIDs != nil {
		if pn, err := c.cli.Store.LIDs.GetPNForLID(ctx, jid); err == nil && !pn.IsEmpty() {
			return pn.ToNonAD()
		}
	}
	return jid
}
