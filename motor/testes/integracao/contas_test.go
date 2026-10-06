package integracao

import (
	"strings"
	"testing"
)

func TestContasCicloCompleto(t *testing.T) {
	m := novoMotor(t)

	var c Conta
	marca := m.marca()
	m.json("POST", "/v1/contas", map[string]any{}, 201, &c)
	if c.Estado != "conectando" || c.ID == "" || c.Nome == "" {
		t.Fatalf("conta criada: %+v", c)
	}
	qr := m.esperarEvento(marca, "conta.qr", nil)
	dq := dados[map[string]string](t, qr)
	if !strings.HasPrefix(dq["codigo"], "2@") || dq["expira_em"] == "" || ptr(qr.ContaID) != c.ID {
		t.Fatalf("conta.qr: %s", qr.Dados)
	}
	var q map[string]string
	m.json("GET", "/v1/contas/"+c.ID+"/qr", nil, 200, &q)
	if q["codigo"] != dq["codigo"] {
		t.Fatalf("GET qr: %v", q)
	}

	marca = m.marca()
	m.json("POST", "/v1/falso/contas/"+c.ID+"/escanear-qr", map[string]string{"telefone": "+5511900000001", "nome": "Loja"}, 200, nil)
	m.json("GET", "/v1/contas/"+c.ID, nil, 200, &c)
	if c.Estado != "conectada" || ptr(c.Telefone) != "+5511900000001" || !c.Online || c.Nome != "Loja" {
		t.Fatalf("após QR: %+v", c)
	}
	m.esperarEvento(marca, "conta.atualizada", func(e Evento) bool { return dados[Conta](t, e).Estado == "conectada" })
	m.erro("GET", "/v1/contas/"+c.ID+"/qr", nil, 404, "nao_encontrado")

	var lista []Conta
	m.json("GET", "/v1/contas", nil, 200, &lista)
	if len(lista) != 1 || lista[0].ID != c.ID {
		t.Fatalf("lista: %+v", lista)
	}

	// sessão persistida: reinicia e volta conectada sem QR
	m.reiniciar()
	m.esperarEvento(0, "conta.atualizada", func(e Evento) bool { return dados[Conta](t, e).Estado == "conectada" })
	if len(m.eventosDoTipo("conta.qr")) != 0 {
		t.Fatal("não deveria pedir QR após reiniciar")
	}
	m.json("GET", "/v1/contas/"+c.ID, nil, 200, &c)
	if c.Estado != "conectada" {
		t.Fatalf("após reiniciar: %+v", c)
	}

	// queda de rede: continua conectada, online=false
	marca = m.marca()
	m.json("POST", "/v1/falso/contas/"+c.ID+"/estado", map[string]string{"evento": "queda_rede"}, 200, nil)
	m.json("GET", "/v1/contas/"+c.ID, nil, 200, &c)
	if c.Estado != "conectada" || c.Online {
		t.Fatalf("queda de rede: %+v", c)
	}
	ev := m.esperarEvento(marca, "conexao.rede", nil)
	if dados[map[string]any](t, ev)["online"] != false {
		t.Fatalf("conexao.rede: %s", ev.Dados)
	}
	m.json("POST", "/v1/falso/contas/"+c.ID+"/estado", map[string]string{"evento": "reconectou"}, 200, nil)
	m.json("GET", "/v1/contas/"+c.ID, nil, 200, &c)
	if !c.Online {
		t.Fatal("deveria voltar online")
	}

	// logout → desconectada
	m.json("POST", "/v1/falso/contas/"+c.ID+"/estado", map[string]string{"evento": "logout"}, 200, nil)
	m.json("GET", "/v1/contas/"+c.ID, nil, 200, &c)
	if c.Estado != "desconectada" {
		t.Fatalf("logout: %+v", c)
	}

	// reconectar → novo QR (sessão perdida) → conectada → ban → banida
	marca = m.marca()
	m.json("POST", "/v1/contas/"+c.ID+"/reconectar", nil, 202, &c)
	if c.Estado != "conectando" {
		t.Fatalf("reconectar: %+v", c)
	}
	m.esperarEvento(marca, "conta.qr", nil)
	m.json("POST", "/v1/falso/contas/"+c.ID+"/escanear-qr", map[string]string{"telefone": "+5511900000001"}, 200, nil)
	m.json("POST", "/v1/falso/contas/"+c.ID+"/estado", map[string]string{"evento": "ban"}, 200, nil)
	m.json("GET", "/v1/contas/"+c.ID, nil, 200, &c)
	if c.Estado != "banida" {
		t.Fatalf("ban: %+v", c)
	}

	// renomear
	m.erro("PATCH", "/v1/contas/"+c.ID, map[string]string{"nome": ""}, 422, "validacao")
	m.erro("PATCH", "/v1/contas/"+c.ID, map[string]string{"nome": strings.Repeat("a", 61)}, 422, "validacao")
	m.json("PATCH", "/v1/contas/"+c.ID, map[string]string{"nome": "Vendas"}, 200, &c)
	if c.Nome != "Vendas" {
		t.Fatalf("renomear: %+v", c)
	}

	// reconectar após ban (sessão ainda válida no falso) → conectada sem QR
	m.json("POST", "/v1/contas/"+c.ID+"/reconectar", nil, 202, nil)
	m.json("POST", "/v1/falso/contas/"+c.ID+"/estado", map[string]string{"evento": "reconectou"}, 200, nil)
	m.json("GET", "/v1/contas/"+c.ID, nil, 200, &c)
	if c.Estado != "conectada" {
		t.Fatalf("reconectar após ban: %+v", c)
	}

	// remover apaga dados
	m.receber(c.ID, map[string]any{"de": "+5511911112222", "texto": "oi"})
	if len(m.conversas(c.ID, "")) != 1 {
		t.Fatal("deveria ter conversa")
	}
	marca = m.marca()
	if st, b := m.req("DELETE", "/v1/contas/"+c.ID, nil); st != 204 {
		t.Fatalf("remover: %d %s", st, b)
	}
	m.esperarEvento(marca, "conta.removida", nil)
	m.erro("GET", "/v1/contas/"+c.ID, nil, 404, "nao_encontrado")
	m.erro("GET", "/v1/contas/"+c.ID+"/conversas", nil, 404, "nao_encontrado")
	m.erro("DELETE", "/v1/contas/"+c.ID, nil, 404, "nao_encontrado")
}

func TestContaNomePadraoEPushName(t *testing.T) {
	m := novoMotor(t)
	var c Conta
	m.json("POST", "/v1/contas", map[string]any{"nome": "Minha loja"}, 201, &c)
	if c.Nome != "Minha loja" {
		t.Fatalf("nome: %+v", c)
	}
	m.erro("POST", "/v1/contas", map[string]any{"nome": strings.Repeat("x", 61)}, 422, "validacao")

	var d Conta
	marca := m.marca()
	m.json("POST", "/v1/contas", map[string]any{}, 201, &d)
	m.esperarEvento(marca, "conta.qr", func(e Evento) bool { return ptr(e.ContaID) == d.ID })
	m.json("POST", "/v1/falso/contas/"+d.ID+"/escanear-qr", map[string]string{"telefone": "+5511900000002", "nome": "Push Name"}, 200, nil)
	m.json("GET", "/v1/contas/"+d.ID, nil, 200, &d)
	if d.Nome != "Push Name" {
		t.Fatalf("nome padrão deveria virar o pushname: %+v", d)
	}

	// QR expirado → desconectada + conta.qr_expirado
	var e Conta
	marca = m.marca()
	m.json("POST", "/v1/contas", map[string]any{}, 201, &e)
	m.esperarEvento(marca, "conta.qr", func(ev Evento) bool { return ptr(ev.ContaID) == e.ID })
	m.json("POST", "/v1/falso/contas/"+e.ID+"/expirar-qr", nil, 200, nil)
	m.esperarEvento(marca, "conta.qr_expirado", func(ev Evento) bool { return ptr(ev.ContaID) == e.ID })
	m.json("GET", "/v1/contas/"+e.ID, nil, 200, &e)
	if e.Estado != "desconectada" {
		t.Fatalf("QR expirado: %+v", e)
	}
}

// Após reiniciar, "conectada" só aparece quando a sessão já reabriu: se a API disser conectada,
// o envio tem de ser aceito (antes o estado gravado da execução anterior vazava por alguns ms).
func TestContaAposReiniciarNaoMenteConectada(t *testing.T) {
	m := novoMotor(t)
	c := m.contaConectada("+5511900000077")
	m.injetar(c.ID, "+5511955557777", "oi")
	conv := m.conversas(c.ID, "")[0]
	for i := 0; i < 10; i++ {
		m.reiniciar()
		var atual Conta
		m.json("GET", "/v1/contas/"+c.ID, nil, 200, &atual)
		if atual.Estado == "conectada" {
			m.json("POST", "/v1/conversas/"+conv.ID+"/mensagens", map[string]any{"texto": "ainda aí?"}, 202, nil)
		} else if atual.Estado != "conectando" {
			t.Fatalf("estado logo após reiniciar: %s", atual.Estado)
		}
		m.aguardarConta(c.ID)
	}
}
