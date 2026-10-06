package integracao

import (
	"strings"
	"testing"
)

func TestAutomacoesConfiguracaoEPausas(t *testing.T) {
	m := novoMotor(t)
	c := m.contaConectada("+5511988880000")

	var cfg map[string]any
	m.json("GET", "/v1/automacoes/configuracao", nil, 200, &cfg)
	if cfg["anti_loop_mensagens"] != float64(10) || cfg["anti_loop_janela_min"] != float64(10) || cfg["pausa_humana_min"] != float64(30) ||
		cfg["primeiros_contatos_hora"] != float64(20) || cfg["pausa_geral"] != false || cfg["processos_ia_max"] != float64(4) {
		t.Fatalf("padrões: %v", cfg)
	}
	marca := m.marca()
	m.json("PATCH", "/v1/automacoes/configuracao", map[string]any{"pausa_geral": true, "pausa_humana_min": 45}, 200, &cfg)
	if cfg["pausa_geral"] != true || cfg["pausa_humana_min"] != float64(45) {
		t.Fatalf("patch: %v", cfg)
	}
	m.esperarEvento(marca, "automacoes.configuracao", nil)
	e := m.erro("PATCH", "/v1/automacoes/configuracao", map[string]any{"anti_loop_mensagens": 0, "tempo_ia_s": 999}, 422, "validacao")
	campos := e["detalhes"].(map[string]any)["campos"].(map[string]any)
	if campos["anti_loop_mensagens"] == nil || campos["tempo_ia_s"] == nil {
		t.Fatalf("faixas: %v", e)
	}
	m.json("PATCH", "/v1/automacoes/configuracao", map[string]any{"pausa_geral": false}, 200, nil)

	// Conversa: estado, "Assumir" sem prazo, lista de pausas, "Devolver".
	m.injetar(c.ID, "+5511977770001", "oi")
	conv := m.conversas(c.ID, "")[0]
	var est map[string]any
	m.json("GET", "/v1/conversas/"+conv.ID+"/automacoes", nil, 200, &est)
	if est["conversa_id"] != conv.ID || est["pausa"] != nil || est["sessao"] != nil || est["pausa_geral"] != false {
		t.Fatalf("estado: %v", est)
	}
	marca = m.marca()
	var p Pausa
	m.json("POST", "/v1/conversas/"+conv.ID+"/pausa", map[string]any{"motivo": "humano"}, 200, &p)
	if p.Motivo != "humano" || p.Ate != nil {
		t.Fatalf("assumir: %+v", p)
	}
	ev := m.esperarEvento(marca, "conversa.pausa", nil)
	if ev.ContaID == nil || *ev.ContaID != c.ID {
		t.Fatalf("conversa.pausa sem conta: %+v", ev)
	}
	var lista []Pausa
	m.json("GET", "/v1/pausas?motivo=humano", nil, 200, &lista)
	if len(lista) != 1 || lista[0].ConversaID != conv.ID {
		t.Fatalf("pausas: %+v", lista)
	}
	m.erro("POST", "/v1/conversas/"+conv.ID+"/pausa", map[string]any{"motivo": "anti_loop"}, 422, "validacao")
	if st, _ := m.req("DELETE", "/v1/conversas/"+conv.ID+"/pausa", nil); st != 204 {
		t.Fatalf("devolver: %d", st)
	}
	m.json("GET", "/v1/pausas", nil, 200, &lista)
	if len(lista) != 0 {
		t.Fatalf("pausa não removida: %+v", lista)
	}
	m.erro("GET", "/v1/conversas/nao-existe/automacoes", nil, 404, "nao_encontrado")

	// Resposta manual pela API → pausa humano de 30 min (padrão).
	m.json("POST", "/v1/conversas/"+conv.ID+"/mensagens", map[string]any{"texto": "Oi, aqui é a Ana"}, 202, nil)
	m.ociosoAut()
	m.json("GET", "/v1/conversas/"+conv.ID+"/automacoes", nil, 200, &est)
	pz, _ := est["pausa"].(map[string]any)
	if pz == nil || pz["motivo"] != "humano" || pz["ate"] == nil {
		t.Fatalf("resposta manual deveria pausar: %v", est)
	}

	// Sistema e segredos (só nomes).
	var sis map[string]any
	m.json("GET", "/v1/sistema", nil, 200, &sis)
	if sis["automacoes_ativas"] != float64(0) || sis["processos_ia"] != float64(0) || sis["runner_disponivel"] != false {
		t.Fatalf("sistema: %v", sis)
	}
	marca = m.marca()
	m.json("POST", "/v1/falso/segredos", map[string]any{"valores": map[string]string{"ANTHROPIC_API_KEY": "sk-ant-segredo-123", "OUTRO": "x"}}, 200, nil)
	ev = m.esperarEvento(marca, "segredos.alterados", nil)
	if string(ev.Dados) != `{"nomes":["ANTHROPIC_API_KEY","OUTRO"]}` {
		t.Fatalf("segredos.alterados: %s", ev.Dados)
	}
	st, corpo := m.req("GET", "/v1/segredos", nil)
	if st != 200 || contem(string(corpo), "sk-ant-segredo") || !contem(string(corpo), `"reservado":true`) {
		t.Fatalf("segredos: %d %s", st, corpo)
	}
	var ia map[string]any
	m.json("GET", "/v1/ia/configuracao", nil, 200, &ia)
	if ia["modelo_padrao"] != "claude-sonnet-5" || ia["chave_configurada"] != true || len(ia["modelos"].([]any)) < 3 {
		t.Fatalf("ia: %v", ia)
	}
	m.json("POST", "/v1/ia/testar-chave", map[string]any{}, 200, nil)
	m.json("POST", "/v1/falso/segredos", map[string]any{"valores": map[string]string{}}, 200, nil)
	d := m.erro("POST", "/v1/ia/testar-chave", map[string]any{}, 409, "ia_nao_configurada")
	if d["detalhes"].(map[string]any)["segredo"] != "ANTHROPIC_API_KEY" {
		t.Fatalf("ia_nao_configurada: %v", d)
	}
	m.json("PATCH", "/v1/ia/configuracao", map[string]any{"modelo_padrao": "claude-opus-5-5"}, 200, &ia)
	if ia["modelo_padrao"] != "claude-opus-5-5" {
		t.Fatal(ia)
	}
	m.erro("PATCH", "/v1/ia/configuracao", map[string]any{"modelo_padrao": "gpt-5"}, 422, "validacao")

	// IA simulada: configurar e ler chamadas; processar esperas responde.
	m.json("PUT", "/v1/falso/ia", map[string]any{"respostas": []map[string]string{{"contem": "preço", "texto": "R$ 10"}}}, 200, nil)
	var chamadas []any
	m.json("GET", "/v1/falso/ia/chamadas", nil, 200, &chamadas)
	m.processarEsperas()

	// Persistência da configuração entre reinícios.
	m.reiniciar()
	m.json("GET", "/v1/automacoes/configuracao", nil, 200, &cfg)
	if cfg["pausa_geral"] != false || cfg["pausa_humana_min"] != float64(45) {
		t.Fatalf("configuração não persistiu: %v", cfg)
	}
}

func contem(s, sub string) bool { return strings.Contains(s, sub) }
