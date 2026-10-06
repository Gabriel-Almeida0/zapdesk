package integracao

import (
	"testing"
	"time"
)

type grupoStatus struct {
	ContatoJID  string  `json:"contato_jid"`
	ContatoNome *string `json:"contato_nome"`
	Itens       []struct {
		ID          string  `json:"id"`
		Tipo        string  `json:"tipo"`
		Texto       *string `json:"texto"`
		PublicadoEm string  `json:"publicado_em"`
	} `json:"itens"`
}

func TestStatusDosContatos(t *testing.T) {
	m := novoMotor(t)
	c := m.contaConectada("+5511900000001")
	m.receber(c.ID, map[string]any{"de": "+5511944445555", "texto": "oi", "nome": "Carla"})
	marca := m.marca()
	m.json("POST", "/v1/falso/contas/"+c.ID+"/status", map[string]string{"de": "+5511944445555", "texto": "Promoção hoje!"}, 200, nil)
	ev := m.esperarEvento(marca, "status.novo", nil)
	if d := dados[map[string]any](t, ev); d["texto"] != "Promoção hoje!" || d["tipo"] != "texto" {
		t.Fatalf("status.novo: %v", d)
	}
	m.Relogio.Avancar(time.Hour)
	m.json("POST", "/v1/falso/contas/"+c.ID+"/status", map[string]string{"de": "+5511944445555", "texto": "Segundo"}, 200, nil)
	m.json("POST", "/v1/falso/contas/"+c.ID+"/status", map[string]string{"de": "+5511933332222", "texto": "Outro"}, 200, nil)

	var gs []grupoStatus
	m.json("GET", "/v1/contas/"+c.ID+"/status", nil, 200, &gs)
	if len(gs) != 2 {
		t.Fatalf("grupos: %+v", gs)
	}
	var carla grupoStatus
	for _, g := range gs {
		if g.ContatoJID == "5511944445555@s.whatsapp.net" {
			carla = g
		}
	}
	if ptr(carla.ContatoNome) != "Carla" || len(carla.Itens) != 2 || ptr(carla.Itens[0].Texto) != "Segundo" {
		t.Fatalf("status da Carla (mais recente primeiro): %+v", carla)
	}
	// o status não vira conversa
	if len(m.conversas(c.ID, "")) != 1 {
		t.Fatal("status não pode criar conversa")
	}
	m.Relogio.Avancar(23*time.Hour + time.Minute)
	m.json("GET", "/v1/contas/"+c.ID+"/status", nil, 200, &gs)
	total := 0
	for _, g := range gs {
		total += len(g.Itens)
	}
	if total != 2 {
		t.Fatalf("o primeiro status deveria sumir após 24 h: %d", total)
	}
	m.Relogio.Avancar(2 * time.Hour)
	m.json("GET", "/v1/contas/"+c.ID+"/status", nil, 200, &gs)
	if len(gs) != 0 {
		t.Fatalf("tudo expirado: %+v", gs)
	}
	m.erro("GET", "/v1/status/x/midia", nil, 404, "nao_encontrado")
	m.erro("GET", "/v1/contas/x/status", nil, 404, "nao_encontrado")
}
