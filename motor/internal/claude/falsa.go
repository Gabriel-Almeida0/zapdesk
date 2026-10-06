package claude

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"
)

// RespostaFalsa: primeira cujo `Contem` aparece no prompt (sem diferenciar maiúsculas).
type RespostaFalsa struct {
	Contem string `json:"contem"`
	Texto  string `json:"texto"`
}

// ErroFalso injetado nas próximas `Vezes` chamadas.
type ErroFalso struct {
	Status int `json:"status"`
	Vezes  int `json:"vezes"`
}

// ChamadaFalsa registrada para asserts (GET /v1/falso/ia/chamadas).
type ChamadaFalsa struct {
	Modelo          string         `json:"modelo"`
	Sistema         *string        `json:"sistema"`
	MensagensResumo string         `json:"mensagens_resumo"`
	Esquema         map[string]any `json:"esquema,omitempty"`
	Em              time.Time      `json:"em"`
}

// Falsa é a IA simulada determinística (--ia=falsa): nunca abre rede, tokens 0.
type Falsa struct {
	agora     func() time.Time
	mu        sync.Mutex
	respostas []RespostaFalsa
	erro      *ErroFalso
	chamadas  []ChamadaFalsa
}

// NovaFalsa cria a IA simulada.
func NovaFalsa(agora func() time.Time) *Falsa {
	if agora == nil {
		agora = time.Now
	}
	return &Falsa{agora: agora}
}

// Configurar substitui as respostas e o erro injetado.
func (f *Falsa) Configurar(respostas []RespostaFalsa, erro *ErroFalso) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.respostas = append([]RespostaFalsa{}, respostas...)
	if erro != nil && erro.Vezes > 0 {
		e := *erro
		f.erro = &e
	} else {
		f.erro = nil
	}
}

// Chamadas feitas até agora (nunca nil).
func (f *Falsa) Chamadas() []ChamadaFalsa {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]ChamadaFalsa{}, f.chamadas...)
}

const limiteResumo = 500

func resumir(s string) string {
	r := []rune(s)
	if len(r) > limiteResumo {
		return string(r[:limiteResumo]) + "…"
	}
	return s
}

// registrar grava a chamada, aplica o erro injetado e procura resposta configurada.
// `busca` é o texto em que `Contem` é procurado (vazio = sistema + texto).
func (f *Falsa) registrar(modelo, sistema, texto, busca string, esquema map[string]any) (*RespostaFalsa, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := ChamadaFalsa{Modelo: modeloOu(modelo), MensagensResumo: resumir(texto), Esquema: esquema, Em: f.agora()}
	if sistema != "" {
		s := sistema
		c.Sistema = &s
	}
	f.chamadas = append(f.chamadas, c)
	if f.erro != nil {
		st := f.erro.Status
		f.erro.Vezes--
		if f.erro.Vezes <= 0 {
			f.erro = nil
		}
		return nil, erroHTTP(st, "req_falso", nil)
	}
	if busca == "" {
		busca = sistema + "\n" + texto
	}
	alvo := strings.ToLower(busca)
	for i := range f.respostas {
		if r := f.respostas[i]; r.Contem == "" || strings.Contains(alvo, strings.ToLower(r.Contem)) {
			return &r, nil
		}
	}
	return nil, nil
}

func textoMensagens(prompt string, ms []Mensagem) (tudo, ultimaUser string) {
	var partes []string
	for _, m := range ms {
		partes = append(partes, m.Texto)
		if m.Papel != "assistant" {
			ultimaUser = m.Texto
		}
	}
	if prompt != "" {
		partes = append(partes, prompt)
		ultimaUser = prompt
	}
	return strings.Join(partes, "\n"), ultimaUser
}

// Gerar devolve a resposta configurada ou "[IA simulada] …".
func (f *Falsa) Gerar(_ context.Context, p PedidoGerar) (RespostaGerar, error) {
	tudo, ultima := textoMensagens(p.Prompt, p.Mensagens)
	r, err := f.registrar(p.Modelo, p.Sistema, tudo, "", nil)
	if err != nil {
		return RespostaGerar{}, err
	}
	texto := ""
	if r != nil {
		texto = r.Texto
	} else {
		ru := []rune(ultima)
		if len(ru) > 60 {
			ru = ru[:60]
		}
		texto = "[IA simulada] " + string(ru)
	}
	return RespostaGerar{Texto: texto, Modelo: modeloOu(p.Modelo), MotivoParada: "end_turn", RequestID: "req_falso"}, nil
}

// Classificar devolve a primeira categoria (ou a configurada, se for uma das categorias).
func (f *Falsa) Classificar(_ context.Context, p PedidoClassificar) (RespostaClassificar, error) {
	if err := validarCategorias(p.Categorias); err != nil {
		return RespostaClassificar{}, err
	}
	r, err := f.registrar(p.Modelo, p.Sistema, promptClassificar(p), p.Sistema+"\n"+p.Instrucoes+"\n"+p.Texto, esquemaClassificar(p.Categorias))
	if err != nil {
		return RespostaClassificar{}, err
	}
	cat := p.Categorias[0]
	if r != nil && contem(p.Categorias, strings.TrimSpace(r.Texto)) {
		cat = strings.TrimSpace(r.Texto)
	}
	return RespostaClassificar{Categoria: cat, Modelo: modeloOu(p.Modelo), RequestID: "req_falso"}, nil
}

// Extrair devolve valores vazios válidos para o esquema (ou o JSON configurado).
func (f *Falsa) Extrair(_ context.Context, p PedidoExtrair) (RespostaExtrair, error) {
	esq, err := prepararEsquema(p.Esquema)
	if err != nil {
		return RespostaExtrair{}, err
	}
	r, err := f.registrar(p.Modelo, p.Sistema, promptExtrair(p), p.Sistema+"\n"+p.Instrucoes+"\n"+p.Texto, esq)
	if err != nil {
		return RespostaExtrair{}, err
	}
	var dados json.RawMessage
	if r != nil && json.Valid([]byte(r.Texto)) {
		dados = json.RawMessage(r.Texto)
	} else {
		dados, _ = json.Marshal(valorVazio(esq))
	}
	return RespostaExtrair{Dados: dados, Modelo: modeloOu(p.Modelo), RequestID: "req_falso"}, nil
}

// TestarChave na IA simulada sempre funciona.
func (f *Falsa) TestarChave(_ context.Context, modelo string) (time.Duration, error) {
	if _, err := f.registrar(modelo, "", "ping", "", nil); err != nil {
		return 0, err
	}
	return 0, nil
}

func valorVazio(e map[string]any) any {
	if enum, ok := e["enum"].([]any); ok && len(enum) > 0 {
		return enum[0]
	}
	ts := tipos(e)
	for _, t := range ts {
		if t == "null" {
			return nil
		}
	}
	if len(ts) == 0 {
		if ehObjeto(e) {
			ts = []string{"object"}
		} else {
			return nil
		}
	}
	switch ts[0] {
	case "object":
		out := map[string]any{}
		if props, ok := e["properties"].(map[string]any); ok {
			for k, v := range props {
				if m, ok := v.(map[string]any); ok {
					out[k] = valorVazio(m)
				} else {
					out[k] = nil
				}
			}
		}
		return out
	case "string":
		return ""
	case "number", "integer":
		return 0
	case "boolean":
		return false
	case "array":
		return []any{}
	}
	return nil
}
