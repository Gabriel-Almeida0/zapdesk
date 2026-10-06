package chat

import (
	"testing"
	"time"

	"zapdesk/motor/internal/dominio"
)

func TestPrazos(t *testing.T) {
	envio := time.Date(2026, 9, 28, 10, 0, 0, 0, time.Local)
	minha := dominio.Mensagem{DeMim: true, Tipo: "texto", Estado: dominio.MsgEnviada, EnviadaEm: envio}
	casos := []struct {
		m              dominio.Mensagem
		depois         time.Duration
		editar, apagar bool
	}{
		{minha, 0, true, true},
		{minha, 15 * time.Minute, true, true},
		{minha, 15*time.Minute + time.Second, false, true},
		{minha, 48 * time.Hour, false, true},
		{minha, 48*time.Hour + time.Second, false, false},
		{dominio.Mensagem{DeMim: false, Tipo: "texto", Estado: dominio.MsgRecebida, EnviadaEm: envio}, 0, false, false},
		{dominio.Mensagem{DeMim: true, Tipo: "imagem", Estado: dominio.MsgLida, EnviadaEm: envio}, 0, false, true},
		{dominio.Mensagem{DeMim: true, Tipo: "texto", Estado: dominio.MsgFalhou, EnviadaEm: envio}, 0, false, false},
		{dominio.Mensagem{DeMim: true, Tipo: "texto", Estado: dominio.MsgPendente, EnviadaEm: envio}, 0, false, false},
		{dominio.Mensagem{DeMim: true, Tipo: "texto", Estado: dominio.MsgEntregue, Apagada: true, EnviadaEm: envio}, 0, false, false},
	}
	for i, c := range casos {
		agora := envio.Add(c.depois)
		if PodeEditar(c.m, agora) != c.editar || PodeApagar(c.m, agora) != c.apagar {
			t.Errorf("caso %d: editar=%v apagar=%v", i, PodeEditar(c.m, agora), PodeApagar(c.m, agora))
		}
	}
}
