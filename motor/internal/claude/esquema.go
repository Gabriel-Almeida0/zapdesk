package claude

import (
	"fmt"
	"strings"
)

// palavras de esquema não suportadas pelas saídas estruturadas.
var proibidas = []string{"minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "minLength", "maxLength",
	"minItems", "maxItems", "pattern", "multipleOf"}

// prepararEsquema copia o esquema do usuário forçando additionalProperties:false em todo objeto e
// rejeitando restrições numéricas/de tamanho e recursão ($ref/$defs).
func prepararEsquema(e map[string]any) (map[string]any, error) {
	if e == nil {
		return nil, &Erro{Codigo: CodigoValidacao, Mensagem: "Informe o esquema JSON para extrair os dados."}
	}
	v, err := copiarEsquema(e, "$")
	if err != nil {
		return nil, err
	}
	return v.(map[string]any), nil
}

func copiarEsquema(v any, caminho string) (any, error) {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range x {
			if k == "$ref" || k == "$defs" || k == "definitions" {
				return nil, &Erro{Codigo: CodigoValidacao, Mensagem: fmt.Sprintf("Esquemas recursivos ($ref/$defs) não são suportados (em %s).", caminho)}
			}
			for _, p := range proibidas {
				if k == p {
					return nil, &Erro{Codigo: CodigoValidacao, Mensagem: fmt.Sprintf("A restrição %q não é suportada pela IA (em %s); valide o resultado no código.", k, caminho)}
				}
			}
			c, err := copiarEsquema(val, caminho+"."+k)
			if err != nil {
				return nil, err
			}
			out[k] = c
		}
		if ehObjeto(out) {
			out["additionalProperties"] = false
		}
		return out, nil
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			c, err := copiarEsquema(val, fmt.Sprintf("%s[%d]", caminho, i))
			if err != nil {
				return nil, err
			}
			out[i] = c
		}
		return out, nil
	}
	return v, nil
}

func tipos(e map[string]any) []string {
	switch t := e["type"].(type) {
	case string:
		return []string{t}
	case []any:
		var l []string
		for _, x := range t {
			if s, ok := x.(string); ok {
				l = append(l, s)
			}
		}
		return l
	}
	return nil
}

func ehObjeto(e map[string]any) bool {
	for _, t := range tipos(e) {
		if t == "object" {
			return true
		}
	}
	_, temProps := e["properties"]
	return temProps && len(tipos(e)) == 0
}

// esquemaClassificar devolve o esquema {"categoria": enum}.
func esquemaClassificar(cats []string) map[string]any {
	enum := make([]any, len(cats))
	for i, c := range cats {
		enum[i] = c
	}
	return map[string]any{
		"type":                 "object",
		"properties":           map[string]any{"categoria": map[string]any{"type": "string", "enum": enum}},
		"required":             []any{"categoria"},
		"additionalProperties": false,
	}
}

// promptClassificar monta o texto enviado à IA.
func promptClassificar(p PedidoClassificar) string {
	var b strings.Builder
	b.WriteString("Classifique o texto abaixo em exatamente uma das categorias.\n\nCategorias:\n")
	for _, c := range p.Categorias {
		b.WriteString("- " + c)
		if d := p.Descricoes[c]; d != "" {
			b.WriteString(": " + d)
		}
		b.WriteString("\n")
	}
	if p.Instrucoes != "" {
		b.WriteString("\nInstruções: " + p.Instrucoes + "\n")
	}
	b.WriteString("\nTexto:\n" + p.Texto)
	return b.String()
}

func promptExtrair(p PedidoExtrair) string {
	var b strings.Builder
	b.WriteString("Extraia do texto abaixo os dados pedidos no esquema. Use null quando a informação não estiver no texto; não invente.\n")
	if p.Instrucoes != "" {
		b.WriteString("\nInstruções: " + p.Instrucoes + "\n")
	}
	b.WriteString("\nTexto:\n" + p.Texto)
	return b.String()
}

func validarCategorias(cats []string) error {
	if len(cats) == 0 {
		return &Erro{Codigo: CodigoValidacao, Mensagem: "Informe pelo menos uma categoria."}
	}
	return nil
}
