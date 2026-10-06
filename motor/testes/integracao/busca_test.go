package integracao

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestHistoricoPaginacaoEBusca(t *testing.T) {
	m := novoMotor(t)
	c := m.contaConectada("+5511900000001")

	var msgs []map[string]any
	for i := 0; i < 1000; i++ {
		texto := fmt.Sprintf("mensagem antiga %d", i)
		if i == 10 {
			texto = "Promoção de ação especial só hoje"
		}
		msgs = append(msgs, map[string]any{"texto": texto, "de_mim": i%3 == 0})
	}
	marca := m.marca()
	m.json("POST", "/v1/falso/contas/"+c.ID+"/historico", map[string]any{
		"conversas": []map[string]any{{"jid": "+5511988887777", "nome": "Cliente Antigo", "mensagens": msgs}},
	}, 200, nil)

	progresso := m.eventosDoTipo("sincronizacao.progresso")[0:]
	if len(progresso) < 2 {
		t.Fatalf("esperava ≥2 eventos de progresso, veio %d", len(progresso))
	}
	fim := m.esperarEvento(marca, "sincronizacao.progresso", func(e Evento) bool {
		return dados[map[string]any](t, e)["concluida"] == true
	})
	if d := dados[map[string]any](t, fim); d["mensagens"] != float64(1000) || d["conversas"] != float64(1) {
		t.Fatalf("progresso final: %v", d)
	}
	var conta Conta
	m.json("GET", "/v1/contas/"+c.ID, nil, 200, &conta)
	if conta.Sincronizando {
		t.Fatal("deveria ter terminado de sincronizar")
	}

	convs := m.conversas(c.ID, "")
	if len(convs) != 1 || convs[0].Nome != "Cliente Antigo" {
		t.Fatalf("conversa do histórico: %+v", convs)
	}
	cv := convs[0]

	// paginação: 50 por página, mais recentes primeiro, sem repetir
	vistos := map[string]bool{}
	antes := ""
	paginas := 0
	for {
		var p Pagina[Mensagem]
		url := "/v1/conversas/" + cv.ID + "/mensagens?limite=200"
		if antes != "" {
			url += "&antes=" + antes
		}
		m.json("GET", url, nil, 200, &p)
		paginas++
		for i, msg := range p.Itens {
			if vistos[msg.ID] {
				t.Fatalf("mensagem repetida na paginação: %s", msg.ID)
			}
			vistos[msg.ID] = true
			if i > 0 && msg.EnviadaEm > p.Itens[i-1].EnviadaEm {
				t.Fatal("ordem deveria ser da mais recente para a mais antiga")
			}
		}
		if p.ProximoCursor == nil {
			break
		}
		antes = *p.ProximoCursor
	}
	if len(vistos) != 1000 || paginas != 5 {
		t.Fatalf("paginação: %d mensagens em %d páginas", len(vistos), paginas)
	}
	var p50 Pagina[Mensagem]
	m.json("GET", "/v1/conversas/"+cv.ID+"/mensagens", nil, 200, &p50)
	if len(p50.Itens) != 50 || ptr(p50.Itens[0].Texto) != "mensagem antiga 999" {
		t.Fatalf("página padrão: %d %s", len(p50.Itens), ptr(p50.Itens[0].Texto))
	}
	m.erro("GET", "/v1/conversas/"+cv.ID+"/mensagens?limite=500", nil, 422, "validacao")

	// busca sem acento encontra com acento
	var b Pagina[struct {
		Mensagem Mensagem `json:"mensagem"`
		Conversa struct {
			ID   string `json:"id"`
			Nome string `json:"nome"`
		} `json:"conversa"`
		Trecho string `json:"trecho"`
	}]
	m.json("GET", "/v1/contas/"+c.ID+"/mensagens/busca?q=acao", nil, 200, &b)
	if len(b.Itens) != 1 || b.Itens[0].Conversa.ID != cv.ID || b.Itens[0].Conversa.Nome != "Cliente Antigo" ||
		!strings.Contains(strings.ToLower(b.Itens[0].Trecho), "ação") {
		t.Fatalf("busca: %+v", b.Itens)
	}
	m.json("GET", "/v1/contas/"+c.ID+"/mensagens/busca?q=antiga&limite=20", nil, 200, &b)
	if len(b.Itens) != 20 || b.ProximoCursor == nil {
		t.Fatalf("busca paginada: %d", len(b.Itens))
	}
	m.json("GET", "/v1/contas/"+c.ID+"/mensagens/busca?q=antiga&limite=20&cursor="+*b.ProximoCursor, nil, 200, &b)
	if len(b.Itens) != 20 {
		t.Fatalf("busca 2ª página: %d", len(b.Itens))
	}
	m.erro("GET", "/v1/contas/"+c.ID+"/mensagens/busca?q=", nil, 422, "validacao")

	// reenviar o mesmo histórico é idempotente
	recente := inicioPadrao.Add(5 * time.Minute).Format(time.RFC3339)
	m.json("POST", "/v1/falso/contas/"+c.ID+"/historico", map[string]any{
		"conversas": []map[string]any{{"jid": "+5511988887777", "mensagens": []map[string]any{{"texto": "x", "wa_id": "fixo", "em": recente}}}},
	}, 200, nil)
	m.json("POST", "/v1/falso/contas/"+c.ID+"/historico", map[string]any{
		"conversas": []map[string]any{{"jid": "+5511988887777", "mensagens": []map[string]any{{"texto": "x", "wa_id": "fixo", "em": recente}}}},
	}, 200, nil)
	var todas Pagina[Mensagem]
	m.json("GET", "/v1/conversas/"+cv.ID+"/mensagens?limite=5", nil, 200, &todas)
	n := 0
	for _, msg := range todas.Itens {
		if msg.WaID == "fixo" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("mensagem duplicada no histórico: %d", n)
	}
}
