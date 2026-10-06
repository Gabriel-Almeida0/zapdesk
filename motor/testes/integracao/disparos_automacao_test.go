package integracao

import (
	"testing"
)

// T074 — POST /v1/disparos/{id}/destinatarios (ação "adicionar a um disparo").
func TestDisparoAdicionarDestinatarios(t *testing.T) {
	m := novoMotor(t)
	c := m.contaConectada("+5511900000009")
	var rel Relatorio
	m.json("POST", "/v1/leads/importar", map[string]any{"leads": []map[string]any{
		{"telefone": "11 94444-0001", "nome": "Ana"}, {"telefone": "11 94444-0002", "nome": "Beto"}, {"telefone": "11 94444-0003"},
	}}, 200, &rel)
	var d Disparo
	m.json("POST", "/v1/disparos", map[string]any{"conta_id": c.ID, "mensagem": "Oi {nome}", "destinatarios": map[string]any{"lead_ids": rel.LeadIDs[:1]},
		"ritmo": map[string]any{"intervalo_min_s": 30, "intervalo_max_s": 60}}, 201, &d)

	var r struct {
		Adicionados int     `json:"adicionados"`
		JaExistiam  int     `json:"ja_existiam"`
		Disparo     Disparo `json:"disparo"`
	}
	m.json("POST", "/v1/disparos/"+d.ID+"/destinatarios", map[string]any{"lead_ids": rel.LeadIDs[:2]}, 200, &r)
	if r.Adicionados != 1 || r.JaExistiam != 1 || r.Disparo.Contadores.Total != 2 {
		t.Fatalf("adicionar: %+v", r)
	}
	// Idempotente.
	m.json("POST", "/v1/disparos/"+d.ID+"/destinatarios", map[string]any{"lead_ids": rel.LeadIDs[:2]}, 200, &r)
	if r.Adicionados != 0 || r.JaExistiam != 2 {
		t.Fatalf("idempotente: %+v", r)
	}
	// Variável sem valor e sem padrão.
	e := m.erro("POST", "/v1/disparos/"+d.ID+"/destinatarios", map[string]any{"lead_ids": rel.LeadIDs[2:]}, 422, "variaveis_faltando")
	if e["detalhes"].(map[string]any)["total"] != float64(1) {
		t.Fatalf("faltando: %v", e)
	}
	m.erro("POST", "/v1/disparos/"+d.ID+"/destinatarios", map[string]any{"lead_ids": []string{}}, 422, "validacao")
	m.erro("POST", "/v1/disparos/"+d.ID+"/destinatarios", map[string]any{"lead_ids": []string{"x"}}, 422, "validacao")
	// Estados permitidos: enviando/pausado aceitam; cancelado não.
	m.json("POST", "/v1/disparos/"+d.ID+"/iniciar", map[string]any{"valores_padrao": map[string]string{"nome": "tudo bem"}}, 200, nil)
	m.ocioso()
	m.json("POST", "/v1/disparos/"+d.ID+"/pausar", nil, 200, nil)
	m.json("POST", "/v1/disparos/"+d.ID+"/destinatarios", map[string]any{"lead_ids": rel.LeadIDs[2:]}, 200, &r)
	if r.Adicionados != 1 || r.Disparo.Contadores.Total != 3 {
		t.Fatalf("pausado aceita: %+v", r)
	}
	m.json("POST", "/v1/disparos/"+d.ID+"/cancelar", nil, 200, nil)
	m.erro("POST", "/v1/disparos/"+d.ID+"/destinatarios", map[string]any{"lead_ids": rel.LeadIDs[:1]}, 409, "transicao_invalida")
	m.erro("POST", "/v1/disparos/nao-existe/destinatarios", map[string]any{"lead_ids": rel.LeadIDs[:1]}, 404, "nao_encontrado")
}
