package integracao

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"zapdesk/motor/internal/whatsapp"
)

func TestFundacaoAutenticacaoEHost(t *testing.T) {
	m := novoMotor(t)

	r, _ := http.NewRequest("GET", m.url("/v1/saude"), nil)
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("sem token: %d", resp.StatusCode)
	}

	// token errado
	r, _ = http.NewRequest("GET", m.url("/v1/saude"), nil)
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("x", 40))
	resp, _ = http.DefaultClient.Do(r)
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("token errado: %d", resp.StatusCode)
	}

	// ?token= só em GET binário/WS: /v1/saude não aceita
	resp, _ = http.Get(m.url("/v1/saude?token=" + tokenTeste))
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("?token= em rota JSON deveria ser 401: %d", resp.StatusCode)
	}

	st, _ := m.req("GET", "/v1/saude", nil, "Host", "evil.example:80")
	if st != 403 {
		t.Fatalf("host errado: %d", st)
	}

	var saude map[string]any
	m.json("GET", "/v1/saude", nil, 200, &saude)
	if saude["ok"] != true || saude["whatsapp"] != "falso" || saude["versao"] == "" {
		t.Fatalf("saude: %v", saude)
	}

	var sis map[string]any
	m.json("GET", "/v1/sistema", nil, 200, &sis)
	for _, k := range []string{"versao", "pasta_dados", "caminho_logs", "whatsapp", "disparos_ativos", "contas_conectadas"} {
		if _, ok := sis[k]; !ok {
			t.Fatalf("/v1/sistema sem %s: %v", k, sis)
		}
	}

	m.erro("GET", "/v1/nao-existe", nil, 404, "nao_encontrado")
	m.erro("POST", "/v1/sistema/energia", map[string]string{"evento": "x"}, 422, "validacao")
	if st, _ := m.req("POST", "/v1/sistema/energia", map[string]string{"evento": "suspender"}); st != 204 {
		t.Fatalf("energia: %d", st)
	}
}

func TestFundacaoWSMotorPronto(t *testing.T) {
	m := novoMotor(t)
	ev := m.esperarEvento(0, "motor.pronto", nil)
	var dados map[string]string
	json.Unmarshal(ev.Dados, &dados)
	if dados["versao"] != "0.0.0-teste" || ev.ContaID != nil {
		t.Fatalf("motor.pronto: %+v %s", ev, ev.Dados)
	}
}

func TestFundacaoFalsoRelogioEEnviadas(t *testing.T) {
	m := novoMotor(t)
	var r map[string]string
	m.json("PUT", "/v1/falso/relogio", map[string]any{"agora": "2026-09-28T09:00:00-03:00"}, 200, &r)
	if !strings.HasPrefix(r["agora"], "2026-09-28T") {
		t.Fatalf("relógio: %v", r)
	}
	m.json("PUT", "/v1/falso/relogio", map[string]any{"avancar_s": 3600}, 200, &r)
	esperado, _ := parseRFC("2026-09-28T10:00:00-03:00")
	if !m.Relogio.Agora().Equal(esperado) {
		t.Fatalf("avançar: %v", m.Relogio.Agora())
	}
	var enviadas []any
	m.json("GET", "/v1/falso/enviadas", nil, 200, &enviadas)
	if len(enviadas) != 0 {
		t.Fatal("enviadas deveria estar vazio")
	}
	m.json("PUT", "/v1/falso/numeros-sem-whatsapp", map[string]any{"telefones": []string{"11 99999-0000"}}, 200, nil)
	m.json("PUT", "/v1/falso/falhas-envio", map[string]any{"telefones": []string{"+5511988887777"}, "erro": "x"}, 200, nil)
	m.erro("POST", "/v1/falso/contas/inexistente/escanear-qr", map[string]string{"telefone": "+5511900000001"}, 404, "nao_encontrado")
}

type fabricaNula struct{}

func (fabricaNula) Abrir(context.Context, string) (whatsapp.Cliente, error) {
	return nil, errors.New("sem cliente")
}
func (fabricaNula) Remover(context.Context, string) error { return nil }

func TestFundacaoFalsoNaoExisteNoModoReal(t *testing.T) {
	m := novoMotor(t, modoReal(fabricaNula{}))
	var saude map[string]any
	m.json("GET", "/v1/saude", nil, 200, &saude)
	if saude["whatsapp"] != "real" {
		t.Fatalf("modo: %v", saude)
	}
	m.erro("GET", "/v1/falso/enviadas", nil, 404, "nao_encontrado")
	m.erro("PUT", "/v1/falso/relogio", map[string]any{"avancar_s": 1}, 404, "nao_encontrado")
	m.erro("POST", "/v1/falso/contas/x/escanear-qr", map[string]any{}, 404, "nao_encontrado")
}
