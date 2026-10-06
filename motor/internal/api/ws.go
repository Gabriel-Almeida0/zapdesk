package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"zapdesk/motor/internal/eventos"
)

// IntervaloPing do WebSocket (contracts/eventos-ws.md).
var IntervaloPing = 20 * time.Second

// eventosWS atende GET /v1/eventos: primeiro envia motor.pronto e depois retransmite o
// barramento. Mensagens do cliente são ignoradas.
func (s *Servidor) eventosWS(w http.ResponseWriter, r *http.Request) error {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// O token e o Host já foram conferidos; a origem do renderer (file:// ou dev server)
		// não bate com o Host, por isso a checagem de Origin fica desligada.
		InsecureSkipVerify: true,
	})
	if err != nil {
		return nil // Accept já respondeu
	}
	defer c.CloseNow()
	c.SetReadLimit(64 << 10)

	ctx := c.CloseRead(r.Context())
	assinatura, seq := s.o.Barramento.AssinarComSeq(1024)
	defer assinatura.Cancelar()

	pronto := eventos.Evento{Seq: seq, Tipo: eventos.MotorPronto, Em: s.o.Relogio.Agora(),
		Dados: map[string]string{"versao": s.o.Versao}}
	if err := escreverEvento(ctx, c, pronto); err != nil {
		return nil
	}

	ping := time.NewTicker(IntervaloPing)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			c.Close(websocket.StatusNormalClosure, "")
			return nil
		case ev, ok := <-assinatura.C:
			if !ok {
				c.Close(websocket.StatusGoingAway, "motor encerrando")
				return nil
			}
			if err := escreverEvento(ctx, c, ev); err != nil {
				return nil
			}
		case <-ping.C:
			pctx, cancelar := context.WithTimeout(ctx, 10*time.Second)
			err := c.Ping(pctx)
			cancelar()
			if err != nil {
				return nil
			}
		}
	}
}

func escreverEvento(ctx context.Context, c *websocket.Conn, ev eventos.Evento) error {
	dados, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	wctx, cancelar := context.WithTimeout(ctx, 10*time.Second)
	defer cancelar()
	return c.Write(wctx, websocket.MessageText, dados)
}
