package claude

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type chaves map[string]string

func (c chaves) Obter(n string) (string, bool) { v, ok := c[n]; return v, ok }

type servidor struct {
	mu      sync.Mutex
	corpos  []map[string]any
	headers []http.Header
	resps   []func(w http.ResponseWriter)
}

func (s *servidor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	var m map[string]any
	json.Unmarshal(b, &m)
	s.mu.Lock()
	s.corpos = append(s.corpos, m)
	s.headers = append(s.headers, r.Header.Clone())
	i := len(s.corpos) - 1
	if i >= len(s.resps) {
		i = len(s.resps) - 1
	}
	f := s.resps[i]
	s.mu.Unlock()
	f(w)
}

func ok(texto, parada string) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Set("request-id", "req_123")
		json.NewEncoder(w).Encode(map[string]any{"model": "claude-sonnet-5", "stop_reason": parada,
			"content": []any{map[string]any{"type": "text", "text": texto}},
			"usage":   map[string]any{"input_tokens": 11, "output_tokens": 7}})
	}
}

func status(st int, retry string) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		if retry != "" {
			w.Header().Set("retry-after", retry)
		}
		w.Header().Set("request-id", "req_err")
		w.WriteHeader(st)
		w.Write([]byte(`{"type":"error","error":{"type":"x","message":"falhou"}}`))
	}
}

func montar(t *testing.T, resps ...func(w http.ResponseWriter)) (*servidor, Cliente, *[]time.Duration) {
	s := &servidor{resps: resps}
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	var esperas []time.Duration
	c := NovoReal(OpcoesReal{URLBase: ts.URL, Chave: chaves{NomeChave: "sk-ant-teste"},
		Esperar: func(_ context.Context, d time.Duration) error { esperas = append(esperas, d); return nil }})
	return s, c, &esperas
}

func codigo(err error) *Erro {
	var e *Erro
	errors.As(err, &e)
	return e
}

func TestGerarCabecalhosCorpoETokens(t *testing.T) {
	s, c, _ := montar(t, ok("Olá!", "end_turn"))
	r, err := c.Gerar(context.Background(), PedidoGerar{Sistema: "sis", Mensagens: []Mensagem{{Papel: "user", Texto: "oi"}, {Papel: "assistant", Texto: "e aí"}}, Prompt: "tudo?"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Texto != "Olá!" || r.RequestID != "req_123" || r.Uso.Entrada != 11 || r.Uso.Saida != 7 || r.MotivoParada != "end_turn" || r.Modelo != "claude-sonnet-5" {
		t.Fatalf("resposta: %+v", r)
	}
	h := s.headers[0]
	if h.Get("x-api-key") != "sk-ant-teste" || h.Get("anthropic-version") != "2023-06-01" || !strings.HasPrefix(h.Get("content-type"), "application/json") {
		t.Fatalf("cabeçalhos: %v", h)
	}
	b := s.corpos[0]
	for _, k := range []string{"temperature", "top_p", "top_k", "output_config"} {
		if _, tem := b[k]; tem {
			t.Fatalf("corpo não deveria ter %s: %v", k, b)
		}
	}
	if b["model"] != ModeloPadrao || b["max_tokens"].(float64) != 1024 || b["system"] != "sis" || len(b["messages"].([]any)) != 3 {
		t.Fatalf("corpo: %v", b)
	}
	// max_tokens limitado e max_tokens como motivo em gerar não é erro.
	s2, c2, _ := montar(t, ok("cortado", "max_tokens"))
	r, err = c2.Gerar(context.Background(), PedidoGerar{Prompt: "x", MaxTokens: 99999})
	if err != nil || r.MotivoParada != "max_tokens" || s2.corpos[0]["max_tokens"].(float64) != 8192 {
		t.Fatalf("max_tokens: %+v %v", r, err)
	}
}

func TestSemChaveNaoChamaRede(t *testing.T) {
	s := &servidor{resps: []func(http.ResponseWriter){ok("x", "end_turn")}}
	ts := httptest.NewServer(s)
	defer ts.Close()
	c := NovoReal(OpcoesReal{URLBase: ts.URL, Chave: chaves{}})
	_, err := c.Gerar(context.Background(), PedidoGerar{Prompt: "oi"})
	if e := codigo(err); e == nil || e.Codigo != CodigoNaoConfigurada || e.Mensagem != "Configure a chave da Anthropic em Ajustes → IA" {
		t.Fatalf("erro: %v", err)
	}
	_, err = c.Classificar(context.Background(), PedidoClassificar{Texto: "x", Categorias: []string{"a"}})
	if codigo(err).Codigo != CodigoNaoConfigurada {
		t.Fatal(err)
	}
	if len(s.corpos) != 0 {
		t.Fatal("fez requisição sem chave")
	}
}

func TestClassificarEExtrairEstruturados(t *testing.T) {
	s, c, _ := montar(t, ok(`{"categoria":"quente"}`, "end_turn"))
	r, err := c.Classificar(context.Background(), PedidoClassificar{Texto: "quero comprar", Categorias: []string{"frio", "quente"}, Descricoes: map[string]string{"quente": "quer comprar"}})
	if err != nil || r.Categoria != "quente" || r.Uso.Entrada != 11 {
		t.Fatalf("classificar: %+v %v", r, err)
	}
	of := s.corpos[0]["output_config"].(map[string]any)["format"].(map[string]any)
	sch := of["schema"].(map[string]any)
	if of["type"] != "json_schema" || sch["additionalProperties"] != false ||
		len(sch["properties"].(map[string]any)["categoria"].(map[string]any)["enum"].([]any)) != 2 {
		t.Fatalf("output_config: %v", of)
	}

	s, c, _ = montar(t, ok(`{"nome":"Ana","endereco":{"cidade":"SP"}}`, "end_turn"))
	esq := map[string]any{"type": "object", "properties": map[string]any{
		"nome":     map[string]any{"type": "string"},
		"endereco": map[string]any{"type": "object", "properties": map[string]any{"cidade": map[string]any{"type": "string"}}},
		"itens":    map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{"x": map[string]any{"type": "number"}}}},
	}}
	re, err := c.Extrair(context.Background(), PedidoExtrair{Texto: "Ana de SP", Esquema: esq})
	if err != nil || !strings.Contains(string(re.Dados), "Ana") {
		t.Fatalf("extrair: %+v %v", re, err)
	}
	sch = s.corpos[0]["output_config"].(map[string]any)["format"].(map[string]any)["schema"].(map[string]any)
	props := sch["properties"].(map[string]any)
	item := props["itens"].(map[string]any)["items"].(map[string]any)
	if sch["additionalProperties"] != false || props["endereco"].(map[string]any)["additionalProperties"] != false || item["additionalProperties"] != false {
		t.Fatalf("additionalProperties não forçado: %v", sch)
	}
	if _, tem := esq["additionalProperties"]; tem {
		t.Fatal("esquema do usuário foi alterado")
	}
	// Restrições não suportadas e recursão são rejeitadas antes da rede.
	for _, ruim := range []map[string]any{
		{"type": "object", "properties": map[string]any{"idade": map[string]any{"type": "integer", "minimum": 0}}},
		{"type": "object", "properties": map[string]any{"n": map[string]any{"type": "string", "maxLength": 3}}},
		{"type": "object", "properties": map[string]any{"filho": map[string]any{"$ref": "#"}}},
		{"$defs": map[string]any{}, "type": "object"},
	} {
		antes := len(s.corpos)
		_, err := c.Extrair(context.Background(), PedidoExtrair{Texto: "x", Esquema: ruim})
		if e := codigo(err); e == nil || e.Codigo != CodigoValidacao {
			t.Fatalf("esquema %v aceito: %v", ruim, err)
		}
		if len(s.corpos) != antes {
			t.Fatal("chamou a rede com esquema inválido")
		}
	}
	// max_tokens em estruturado é erro.
	_, c, _ = montar(t, ok(`{"categ`, "max_tokens"))
	_, err = c.Classificar(context.Background(), PedidoClassificar{Texto: "x", Categorias: []string{"a"}})
	if e := codigo(err); e == nil || e.Codigo != CodigoErro {
		t.Fatalf("max_tokens: %v", err)
	}
	// Recusa.
	_, c, _ = montar(t, ok("", "refusal"))
	_, err = c.Gerar(context.Background(), PedidoGerar{Prompt: "x"})
	if e := codigo(err); e == nil || e.Mensagem != "A IA recusou o pedido." {
		t.Fatalf("refusal: %v", err)
	}
}

func TestRetentativas(t *testing.T) {
	s, c, esperas := montar(t, status(529, ""), status(500, ""), status(504, ""), ok("enfim", "end_turn"))
	r, err := c.Gerar(context.Background(), PedidoGerar{Prompt: "x"})
	if err != nil || r.Texto != "enfim" || len(s.corpos) != 4 {
		t.Fatalf("retentativas: %v %d", err, len(s.corpos))
	}
	if len(*esperas) != 3 || (*esperas)[0] != time.Second || (*esperas)[1] != 2*time.Second || (*esperas)[2] != 4*time.Second {
		t.Fatalf("backoff: %v", *esperas)
	}
	// Mais de 3 retentativas: desiste com a mensagem do 529.
	s, c, _ = montar(t, status(529, ""))
	_, err = c.Gerar(context.Background(), PedidoGerar{Prompt: "x"})
	if e := codigo(err); e == nil || e.Status != 529 || e.Mensagem != "A Claude API está sobrecarregada (529). Tente mais tarde." || e.RequestID != "req_err" || len(s.corpos) != 4 {
		t.Fatalf("529: %v %d", err, len(s.corpos))
	}
	// 429 com retry-after respeita o cabeçalho.
	_, c, esperas = montar(t, status(429, "7"), ok("ok", "end_turn"))
	if _, err := c.Gerar(context.Background(), PedidoGerar{Prompt: "x"}); err != nil || (*esperas)[0] != 7*time.Second {
		t.Fatalf("retry-after: %v %v", err, *esperas)
	}
	// 429 sem retry-after duas vezes = limite de gasto.
	s, c, _ = montar(t, status(429, ""))
	_, err = c.Gerar(context.Background(), PedidoGerar{Prompt: "x"})
	if e := codigo(err); e == nil || e.Mensagem != "Limite de gasto da Anthropic atingido" || len(s.corpos) != 2 {
		t.Fatalf("gasto: %v %d", err, len(s.corpos))
	}
	// 401 e 404 não repetem.
	s, c, _ = montar(t, status(401, ""))
	_, err = c.Gerar(context.Background(), PedidoGerar{Prompt: "x"})
	if e := codigo(err); e == nil || e.Mensagem != "Chave da Anthropic inválida. Configure em Ajustes → IA" || e.Status != 401 || len(s.corpos) != 1 {
		t.Fatalf("401: %v", err)
	}
	s, c, _ = montar(t, status(404, ""))
	_, err = c.Gerar(context.Background(), PedidoGerar{Prompt: "x"})
	if e := codigo(err); e == nil || !strings.HasPrefix(e.Mensagem, "Modelo indisponível") || len(s.corpos) != 1 {
		t.Fatalf("404: %v", err)
	}
	// Prazo: espera que passaria do deadline não acontece.
	s, c, esperas = montar(t, status(500, ""))
	ctx, cancelar := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancelar()
	_, err = c.Gerar(ctx, PedidoGerar{Prompt: "x"})
	if err == nil || len(*esperas) != 0 || len(s.corpos) != 1 {
		t.Fatalf("prazo: %v %v %d", err, *esperas, len(s.corpos))
	}
}

func TestTestarChave(t *testing.T) {
	s, c, _ := montar(t, ok("p", "end_turn"))
	if _, err := c.TestarChave(context.Background(), "claude-opus-5-5"); err != nil {
		t.Fatal(err)
	}
	if s.corpos[0]["max_tokens"].(float64) != 1 || s.corpos[0]["model"] != "claude-opus-5-5" {
		t.Fatalf("testar chave: %v", s.corpos[0])
	}
}

func TestModelos(t *testing.T) {
	m := Modelos()
	if len(m) != 3 || m[0].ID != ModeloPadrao || m[2].Aviso == nil || !strings.Contains(*m[2].Aviso, "15/10/2026") {
		t.Fatalf("modelos: %+v", m)
	}
}

func TestFalsa(t *testing.T) {
	agora := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	f := NovaFalsa(func() time.Time { return agora })
	ctx := context.Background()
	if len(f.Chamadas()) != 0 || f.Chamadas() == nil {
		t.Fatal("chamadas deve começar vazia e não nil")
	}
	r, _ := f.Gerar(ctx, PedidoGerar{Mensagens: []Mensagem{{Papel: "user", Texto: "Quanto custa o plano?"}}})
	if r.Texto != "[IA simulada] Quanto custa o plano?" || r.Uso.Entrada != 0 || r.Modelo != ModeloPadrao {
		t.Fatalf("padrão: %+v", r)
	}
	f.Configurar([]RespostaFalsa{{Contem: "PREÇO", Texto: "O valor é R$ 10"}, {Contem: "quente", Texto: "quente"}, {Contem: "extrair", Texto: `{"nome":"Ana"}`}}, nil)
	r, _ = f.Gerar(ctx, PedidoGerar{Sistema: "s", Prompt: "qual o preço?", Modelo: "claude-opus-5-5"})
	if r.Texto != "O valor é R$ 10" || r.Modelo != "claude-opus-5-5" {
		t.Fatalf("configurada: %+v", r)
	}
	cl, _ := f.Classificar(ctx, PedidoClassificar{Texto: "lead morno", Categorias: []string{"frio", "quente"}})
	if cl.Categoria != "frio" {
		t.Fatalf("classificar padrão: %+v", cl)
	}
	cl, _ = f.Classificar(ctx, PedidoClassificar{Texto: "lead quente", Categorias: []string{"frio", "quente"}})
	if cl.Categoria != "quente" {
		t.Fatalf("classificar configurado: %+v", cl)
	}
	esq := map[string]any{"type": "object", "properties": map[string]any{
		"nome": map[string]any{"type": "string"}, "idade": map[string]any{"type": "integer"},
		"ativo": map[string]any{"type": "boolean"}, "tags": map[string]any{"type": "array"},
		"email": map[string]any{"type": []any{"string", "null"}}, "nivel": map[string]any{"enum": []any{"a", "b"}},
		"sub": map[string]any{"type": "object", "properties": map[string]any{"x": map[string]any{"type": "number"}}}}}
	ex, _ := f.Extrair(ctx, PedidoExtrair{Texto: "nada", Esquema: esq})
	var v map[string]any
	json.Unmarshal(ex.Dados, &v)
	if v["nome"] != "" || v["idade"].(float64) != 0 || v["ativo"] != false || len(v["tags"].([]any)) != 0 || v["email"] != nil ||
		v["nivel"] != "a" || v["sub"].(map[string]any)["x"].(float64) != 0 {
		t.Fatalf("extrair vazio: %s", ex.Dados)
	}
	ex, _ = f.Extrair(ctx, PedidoExtrair{Texto: "extrair isto", Esquema: esq})
	if string(ex.Dados) != `{"nome":"Ana"}` {
		t.Fatalf("extrair configurado: %s", ex.Dados)
	}
	ch := f.Chamadas()
	if len(ch) != 6 || ch[1].Sistema == nil || *ch[1].Sistema != "s" || ch[2].Esquema == nil || !ch[0].Em.Equal(agora) || !strings.Contains(ch[1].MensagensResumo, "preço") {
		t.Fatalf("chamadas: %+v", ch)
	}
	// Erro injetado nas próximas 2 chamadas.
	f.Configurar(nil, &ErroFalso{Status: 529, Vezes: 2})
	for i := 0; i < 2; i++ {
		_, err := f.Gerar(ctx, PedidoGerar{Prompt: "x"})
		if e := codigo(err); e == nil || e.Status != 529 || e.Codigo != CodigoErro {
			t.Fatalf("erro injetado %d: %v", i, err)
		}
	}
	if _, err := f.Gerar(ctx, PedidoGerar{Prompt: "x"}); err != nil {
		t.Fatalf("depois do erro: %v", err)
	}
	var _ Cliente = f
}
