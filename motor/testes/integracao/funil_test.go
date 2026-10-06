package integracao

import (
	"encoding/json"
	"strings"
	"testing"
)

type EtapaT struct {
	ID         string `json:"id"`
	Nome       string `json:"nome"`
	Cor        string `json:"cor"`
	Ordem      int    `json:"ordem"`
	TotalCards int    `json:"total_cards"`
}

type FunilT struct {
	ID         string   `json:"id"`
	Nome       string   `json:"nome"`
	Etapas     []EtapaT `json:"etapas"`
	TotalCards int      `json:"total_cards"`
}

type CardT struct {
	LeadID  string `json:"lead_id"`
	EtapaID string `json:"etapa_id"`
	Lead    struct {
		ID       string            `json:"id"`
		Telefone string            `json:"telefone"`
		Nome     *string           `json:"nome"`
		Campos   map[string]string `json:"campos"`
	} `json:"lead"`
	Etiquetas  []Etiqueta `json:"etiquetas"`
	ConversaID *string    `json:"conversa_id"`
	ContaID    *string    `json:"conta_id"`
}

func TestFunilRotasCardsHistoricoEEventos(t *testing.T) {
	m := novoMotor(t)
	c := m.contaConectada("+5511988880001")

	marca := m.marca()
	var f FunilT
	m.json("POST", "/v1/funis", map[string]any{"nome": "Vendas", "etapas": []map[string]any{{"nome": "Novo"}, {"nome": "Qualificando", "cor": "#22AA44"}, {"nome": "Proposta"}, {"nome": "Fechado"}}}, 201, &f)
	if len(f.Etapas) != 4 || f.Etapas[1].Cor != "#22AA44" {
		t.Fatalf("funil: %+v", f)
	}
	ev := m.esperarEvento(marca, "funil.alterado", nil)
	if string(ev.Dados) != `{"funil_id":null}` {
		t.Fatalf("funil.alterado: %s", ev.Dados)
	}
	m.erro("POST", "/v1/funis", map[string]any{"nome": "vendas"}, 409, "conflito")
	m.erro("POST", "/v1/funis", map[string]any{"nome": ""}, 422, "validacao")

	// Lead por telefone (cria lead), por contato sem lead (cria lead origem contatos) e por lead_id.
	m.injetar(c.ID, "+5511977771111", "oi")
	conv := m.conversas(c.ID, "")[0]
	var ct Contato
	m.json("GET", "/v1/contatos/"+*conv.ContatoID, nil, 200, &ct)

	marca = m.marca()
	var card CardT
	m.json("PUT", "/v1/funis/"+f.ID+"/cards", map[string]any{"contato_id": ct.ID, "etapa_id": f.Etapas[0].ID}, 201, &card)
	if card.Lead.Telefone != "+5511977771111" || card.ConversaID == nil || *card.ConversaID != conv.ID || card.ContaID == nil {
		t.Fatalf("card por contato: %+v", card)
	}
	ev = m.esperarEvento(marca, "funil.movido", nil)
	var mov struct {
		Movimento struct {
			Origem           string  `json:"origem"`
			EtapaOrigemID    *string `json:"etapa_origem_id"`
			EtapaDestinoNome *string `json:"etapa_destino_nome"`
		} `json:"movimento"`
		Card *CardT `json:"card"`
	}
	json.Unmarshal(ev.Dados, &mov)
	if mov.Movimento.Origem != "app" || mov.Movimento.EtapaOrigemID != nil || *mov.Movimento.EtapaDestinoNome != "Novo" || mov.Card == nil {
		t.Fatalf("funil.movido: %s", ev.Dados)
	}
	var tel CardT
	m.json("PUT", "/v1/funis/"+f.ID+"/cards", map[string]any{"telefone": "(11) 96666-2222", "etapa_id": f.Etapas[0].ID, "origem": "mcp"}, 201, &tel)
	var lead Lead
	m.json("GET", "/v1/leads/"+tel.LeadID, nil, 200, &lead)
	if lead.Origem != "mcp" {
		t.Fatalf("lead por telefone via mcp: %+v", lead)
	}
	// Mover (200) e mesma etapa (200 sem histórico).
	m.json("PUT", "/v1/funis/"+f.ID+"/cards", map[string]any{"lead_id": card.LeadID, "etapa_id": f.Etapas[1].ID}, 200, &card)
	m.json("PUT", "/v1/funis/"+f.ID+"/cards", map[string]any{"lead_id": card.LeadID, "etapa_id": f.Etapas[1].ID}, 200, nil)
	var h Pagina[map[string]any]
	m.json("GET", "/v1/funis/"+f.ID+"/historico?lead_id="+card.LeadID, nil, 200, &h)
	if len(h.Itens) != 2 {
		t.Fatalf("histórico: %+v", h.Itens)
	}
	m.erro("PUT", "/v1/funis/"+f.ID+"/cards", map[string]any{"telefone": "12", "etapa_id": f.Etapas[0].ID}, 422, "validacao")
	m.erro("PUT", "/v1/funis/"+f.ID+"/cards", map[string]any{"lead_id": card.LeadID, "telefone": "+5511977771111", "etapa_id": f.Etapas[0].ID}, 422, "validacao")

	// Cards paginados, com filtro de etapa e busca; contagens no funil.
	var cards Pagina[CardT]
	m.json("GET", "/v1/funis/"+f.ID+"/cards?etapa_id="+f.Etapas[1].ID, nil, 200, &cards)
	if len(cards.Itens) != 1 || cards.Itens[0].LeadID != card.LeadID {
		t.Fatalf("cards por etapa: %+v", cards)
	}
	m.json("GET", "/v1/funis/"+f.ID+"/cards?busca=96666", nil, 200, &cards)
	if len(cards.Itens) != 1 || cards.Itens[0].LeadID != tel.LeadID {
		t.Fatalf("busca: %+v", cards)
	}
	m.json("GET", "/v1/funis/"+f.ID, nil, 200, &f)
	if f.TotalCards != 2 || f.Etapas[0].TotalCards != 1 || f.Etapas[1].TotalCards != 1 {
		t.Fatalf("contagens: %+v", f)
	}
	var dosLeads []map[string]any
	m.json("GET", "/v1/leads/"+card.LeadID+"/funis", nil, 200, &dosLeads)
	if len(dosLeads) != 1 || dosLeads[0]["card"] == nil {
		t.Fatalf("funis do lead: %v", dosLeads)
	}

	// PATCH /leads/{id}: merge, null remove, chaves normalizadas, limite de 1.000.
	m.json("PATCH", "/v1/leads/"+card.LeadID, map[string]any{"nome": "Maria", "campos": map[string]any{"Empresa X": "ACME", "cidade": "SP"}}, 200, &lead)
	if *lead.Nome != "Maria" || lead.Campos["empresa_x"] != "ACME" || lead.Campos["cidade"] != "SP" {
		t.Fatalf("patch lead: %+v", lead)
	}
	lead = Lead{}
	m.json("PATCH", "/v1/leads/"+card.LeadID, map[string]any{"campos": map[string]any{"cidade": nil}}, 200, &lead)
	if _, ok := lead.Campos["cidade"]; ok || lead.Campos["empresa_x"] != "ACME" || *lead.Nome != "Maria" {
		t.Fatalf("merge/remover: %+v", lead)
	}
	m.json("PATCH", "/v1/leads/"+card.LeadID, map[string]any{"nome": nil}, 200, &lead)
	if lead.Nome != nil {
		t.Fatal("nome null deveria remover")
	}
	m.erro("PATCH", "/v1/leads/"+card.LeadID, map[string]any{"campos": map[string]any{"x": strings.Repeat("a", 1001)}}, 422, "validacao")

	// Etapas: criar, editar, reordenar, excluir com destino obrigatório.
	var e EtapaT
	m.json("POST", "/v1/funis/"+f.ID+"/etapas", map[string]any{"nome": "Perdido", "cor": "#FF0000"}, 201, &e)
	m.json("PATCH", "/v1/etapas/"+e.ID, map[string]any{"nome": "Perdidos"}, 200, &e)
	if e.Nome != "Perdidos" || e.Cor != "#FF0000" {
		t.Fatalf("etapa: %+v", e)
	}
	ordem := []string{e.ID, f.Etapas[0].ID, f.Etapas[1].ID, f.Etapas[2].ID, f.Etapas[3].ID}
	m.json("PUT", "/v1/funis/"+f.ID+"/etapas/ordem", map[string]any{"etapa_ids": ordem}, 200, &f)
	if f.Etapas[0].ID != e.ID {
		t.Fatalf("ordem: %+v", f.Etapas)
	}
	m.erro("PUT", "/v1/funis/"+f.ID+"/etapas/ordem", map[string]any{"etapa_ids": ordem[:2]}, 422, "validacao")
	m.erro("DELETE", "/v1/etapas/"+f.Etapas[1].ID, nil, 422, "validacao")
	if st, _ := m.req("DELETE", "/v1/etapas/"+f.Etapas[1].ID+"?destino_etapa_id="+f.Etapas[2].ID, nil); st != 204 {
		t.Fatalf("excluir etapa com destino: %d", st)
	}
	if st, _ := m.req("DELETE", "/v1/funis/"+f.ID+"/cards/"+tel.LeadID+"?origem=mcp", nil); st != 204 {
		t.Fatal("remover card")
	}
	if st, _ := m.req("DELETE", "/v1/funis/"+f.ID+"/cards/"+tel.LeadID, nil); st != 204 {
		t.Fatal("remover card é idempotente")
	}
	var lista []FunilT
	m.json("GET", "/v1/funis", nil, 200, &lista)
	if len(lista) != 1 || lista[0].TotalCards != 1 {
		t.Fatalf("lista: %+v", lista)
	}
	if st, _ := m.req("DELETE", "/v1/funis/"+f.ID, nil); st != 204 {
		t.Fatal("excluir funil")
	}
	m.erro("GET", "/v1/funis/"+f.ID, nil, 404, "nao_encontrado")
}
