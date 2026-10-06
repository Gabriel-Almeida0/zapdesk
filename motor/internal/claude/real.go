package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// OpcoesReal do cliente HTTP.
type OpcoesReal struct {
	URLBase string // padrão https://api.anthropic.com
	HTTP    *http.Client
	Chave   FonteChave
	// Esperar é o sono entre retentativas (injetável nos testes); nil = timer real respeitando ctx.
	Esperar func(ctx context.Context, d time.Duration) error
}

type real struct{ o OpcoesReal }

// NovoReal cria o cliente da Claude API.
func NovoReal(o OpcoesReal) Cliente {
	if o.URLBase == "" {
		o.URLBase = "https://api.anthropic.com"
	}
	o.URLBase = strings.TrimRight(o.URLBase, "/")
	if o.HTTP == nil {
		o.HTTP = &http.Client{Timeout: 5 * time.Minute}
	}
	if o.Esperar == nil {
		o.Esperar = esperarReal
	}
	return &real{o: o}
}

func esperarReal(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// MaxRetentativas depois da primeira tentativa.
const MaxRetentativas = 3

type mensagemAPI struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type corpoAPI struct {
	Model        string         `json:"model"`
	MaxTokens    int            `json:"max_tokens"`
	System       string         `json:"system,omitempty"`
	Messages     []mensagemAPI  `json:"messages"`
	OutputConfig map[string]any `json:"output_config,omitempty"`
}

type respostaAPI struct {
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
	Content    []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type resultado struct {
	texto, modelo, parada, requestID string
	uso                              Uso
}

func mensagensAPI(prompt string, ms []Mensagem) []mensagemAPI {
	var l []mensagemAPI
	for _, m := range ms {
		role := "user"
		if m.Papel == "assistant" {
			role = "assistant"
		}
		l = append(l, mensagemAPI{Role: role, Content: m.Texto})
	}
	if prompt != "" {
		l = append(l, mensagemAPI{Role: "user", Content: prompt})
	}
	return l
}

func (c *real) chave() (string, error) {
	if c.o.Chave != nil {
		if k, ok := c.o.Chave.Obter(NomeChave); ok && k != "" {
			return k, nil
		}
	}
	return "", &Erro{Codigo: CodigoNaoConfigurada, Mensagem: "Configure a chave da Anthropic em Ajustes → IA"}
}

// chamar faz a requisição com retentativas.
func (c *real) chamar(ctx context.Context, corpo corpoAPI) (resultado, error) {
	chave, err := c.chave()
	if err != nil {
		return resultado{}, err
	}
	if len(corpo.Messages) == 0 {
		return resultado{}, &Erro{Codigo: CodigoValidacao, Mensagem: "Informe o prompt ou as mensagens."}
	}
	dados, _ := json.Marshal(corpo)
	espera := time.Second
	semRetryAfter := 0
	for tentativa := 0; ; tentativa++ {
		r, status, reqID, retryAfter, temRetry, err := c.uma(ctx, chave, dados)
		if err == nil {
			return r, nil
		}
		if status == 0 {
			return resultado{}, err // erro de rede/contexto: sem retentativa
		}
		repetivel := status == 429 || status == 500 || status == 504 || status == 529
		if status == 429 && !temRetry {
			semRetryAfter++
			if semRetryAfter >= 2 {
				return resultado{}, &Erro{Codigo: CodigoErro, Status: 429, RequestID: reqID,
					Mensagem: "Limite de gasto da Anthropic atingido"}
			}
		}
		if !repetivel || tentativa >= MaxRetentativas {
			return resultado{}, err
		}
		d := espera
		if status == 429 && temRetry {
			d = retryAfter
		}
		if dl, ok := ctx.Deadline(); ok && time.Now().Add(d).After(dl) {
			return resultado{}, err
		}
		if e := c.o.Esperar(ctx, d); e != nil {
			return resultado{}, err
		}
		espera *= 2
	}
}

func (c *real) uma(ctx context.Context, chave string, dados []byte) (r resultado, status int, reqID string, retryAfter time.Duration, temRetry bool, err error) {
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, c.o.URLBase+"/v1/messages", bytes.NewReader(dados))
	if e != nil {
		return r, 0, "", 0, false, e
	}
	req.Header.Set("x-api-key", chave)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")
	resp, e := c.o.HTTP.Do(req)
	if e != nil {
		return r, 0, "", 0, false, &Erro{Codigo: CodigoErro, Mensagem: "Não foi possível falar com a Claude API: " + e.Error()}
	}
	defer resp.Body.Close()
	corpo, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	reqID = resp.Header.Get("request-id")
	if ra := resp.Header.Get("retry-after"); ra != "" {
		if s, e := strconv.ParseFloat(ra, 64); e == nil && s >= 0 {
			retryAfter, temRetry = time.Duration(s*float64(time.Second)), true
		}
	}
	if resp.StatusCode != http.StatusOK {
		return r, resp.StatusCode, reqID, retryAfter, temRetry, erroHTTP(resp.StatusCode, reqID, corpo)
	}
	var ra respostaAPI
	if e := json.Unmarshal(corpo, &ra); e != nil {
		return r, 0, reqID, 0, false, &Erro{Codigo: CodigoErro, Status: 200, RequestID: reqID, Mensagem: "Resposta inválida da Claude API."}
	}
	var b strings.Builder
	for _, bl := range ra.Content {
		if bl.Type == "text" {
			b.WriteString(bl.Text)
		}
	}
	r = resultado{texto: b.String(), modelo: ra.Model, parada: ra.StopReason, requestID: reqID,
		uso: Uso{Entrada: ra.Usage.InputTokens, Saida: ra.Usage.OutputTokens}}
	if ra.StopReason == "refusal" {
		return r, 0, reqID, 0, false, &Erro{Codigo: CodigoErro, Status: 200, RequestID: reqID, Mensagem: "A IA recusou o pedido."}
	}
	return r, 200, reqID, 0, false, nil
}

func erroHTTP(status int, reqID string, corpo []byte) *Erro {
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(corpo, &e)
	msg := ""
	switch status {
	case 401:
		msg = "Chave da Anthropic inválida. Configure em Ajustes → IA"
	case 403:
		msg = "A chave da Anthropic não tem permissão para esta chamada."
	case 404:
		msg = "Modelo indisponível; escolha outro em Ajustes → IA"
	case 413:
		msg = "O pedido à IA ficou grande demais."
	case 429:
		msg = "Limite de uso da Claude API atingido (429). Tente mais tarde."
	case 529:
		msg = "A Claude API está sobrecarregada (529). Tente mais tarde."
	case 500, 502, 503, 504:
		msg = fmt.Sprintf("A Claude API falhou (%d). Tente mais tarde.", status)
	default:
		msg = fmt.Sprintf("A Claude API respondeu com erro (%d).", status)
		if e.Error.Message != "" {
			msg += " " + e.Error.Message
		}
	}
	return &Erro{Codigo: CodigoErro, Status: status, RequestID: reqID, Mensagem: msg}
}

func (c *real) Gerar(ctx context.Context, p PedidoGerar) (RespostaGerar, error) {
	r, err := c.chamar(ctx, corpoAPI{Model: modeloOu(p.Modelo), MaxTokens: maxTokens(p.MaxTokens), System: p.Sistema,
		Messages: mensagensAPI(p.Prompt, p.Mensagens)})
	if err != nil {
		return RespostaGerar{}, err
	}
	return RespostaGerar{Texto: r.texto, Modelo: modeloOu(r.modelo), MotivoParada: r.parada, RequestID: r.requestID, Uso: r.uso}, nil
}

func formato(esquema map[string]any) map[string]any {
	return map[string]any{"format": map[string]any{"type": "json_schema", "schema": esquema}}
}

func (c *real) estruturado(ctx context.Context, modelo, sistema, prompt string, max int, esquema map[string]any) (resultado, error) {
	r, err := c.chamar(ctx, corpoAPI{Model: modeloOu(modelo), MaxTokens: maxTokens(max), System: sistema,
		Messages: mensagensAPI(prompt, nil), OutputConfig: formato(esquema)})
	if err != nil {
		return r, err
	}
	if r.parada == "max_tokens" {
		return r, &Erro{Codigo: CodigoErro, Status: 200, RequestID: r.requestID,
			Mensagem: "A resposta da IA foi cortada (max_tokens) e o JSON ficou incompleto. Aumente maxTokens."}
	}
	return r, nil
}

func (c *real) Classificar(ctx context.Context, p PedidoClassificar) (RespostaClassificar, error) {
	if err := validarCategorias(p.Categorias); err != nil {
		return RespostaClassificar{}, err
	}
	if _, err := c.chave(); err != nil {
		return RespostaClassificar{}, err
	}
	r, err := c.estruturado(ctx, p.Modelo, p.Sistema, promptClassificar(p), p.MaxTokens, esquemaClassificar(p.Categorias))
	if err != nil {
		return RespostaClassificar{}, err
	}
	var v struct {
		Categoria string `json:"categoria"`
	}
	if json.Unmarshal([]byte(r.texto), &v) != nil || !contem(p.Categorias, v.Categoria) {
		return RespostaClassificar{}, &Erro{Codigo: CodigoErro, Status: 200, RequestID: r.requestID, Mensagem: "A IA não devolveu uma categoria válida."}
	}
	return RespostaClassificar{Categoria: v.Categoria, Modelo: modeloOu(r.modelo), RequestID: r.requestID, Uso: r.uso}, nil
}

func contem(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func (c *real) Extrair(ctx context.Context, p PedidoExtrair) (RespostaExtrair, error) {
	esq, err := prepararEsquema(p.Esquema)
	if err != nil {
		return RespostaExtrair{}, err
	}
	if _, err := c.chave(); err != nil {
		return RespostaExtrair{}, err
	}
	r, err := c.estruturado(ctx, p.Modelo, p.Sistema, promptExtrair(p), p.MaxTokens, esq)
	if err != nil {
		return RespostaExtrair{}, err
	}
	if !json.Valid([]byte(r.texto)) {
		return RespostaExtrair{}, &Erro{Codigo: CodigoErro, Status: 200, RequestID: r.requestID, Mensagem: "A IA não devolveu um JSON válido."}
	}
	return RespostaExtrair{Dados: json.RawMessage(r.texto), Modelo: modeloOu(r.modelo), RequestID: r.requestID, Uso: r.uso}, nil
}

func (c *real) TestarChave(ctx context.Context, modelo string) (time.Duration, error) {
	inicio := time.Now()
	_, err := c.Gerar(ctx, PedidoGerar{Modelo: modelo, Prompt: "ping", MaxTokens: 1})
	return time.Since(inicio), err
}
