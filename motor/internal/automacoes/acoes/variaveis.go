// Pacote acoes executa as ações de fluxos e chatbots (formatos.md › Ação), reais e simuladas,
// sempre passando os envios pelo portão (seguranca.Portao) e registrando AcaoRegistrada.
package acoes

import (
	"fmt"
	"strings"

	"zapdesk/motor/internal/variaveis"
)

// FonteVariaveis reúne os valores disponíveis para `{variavel}` numa execução
// (data-model.md › Resolução de variáveis em automações).
type FonteVariaveis struct {
	Execucao       map[string]string // variáveis da execução/sessão (maior precedência)
	CamposLead     map[string]string
	NomeLead       string
	NomeContato    string
	NomePush       string
	Telefone       string
	UltimaMensagem string
	Conta          string
}

// ErroVariavel indica uma variável sem valor e sem padrão.
type ErroVariavel struct{ Nome string }

func (e *ErroVariavel) Error() string { return fmt.Sprintf("Variável {%s} sem valor", e.Nome) }

// Valor devolve o valor de uma variável (chave normalizada).
func (f FonteVariaveis) Valor(nome string) string {
	if v := strings.TrimSpace(f.Execucao[nome]); v != "" {
		return v
	}
	switch nome {
	case "nome":
		return f.nome()
	case "primeiro_nome":
		if campos := strings.Fields(f.nome()); len(campos) > 0 {
			return campos[0]
		}
		return ""
	case "telefone":
		return f.Telefone
	case "ultima_mensagem":
		return strings.TrimSpace(f.UltimaMensagem)
	case "conta":
		return f.Conta
	}
	return strings.TrimSpace(f.CamposLead[nome])
}

func (f FonteVariaveis) nome() string {
	for _, n := range []string{f.NomeLead, f.NomeContato, f.NomePush} {
		if strings.TrimSpace(n) != "" {
			return strings.TrimSpace(n)
		}
	}
	return ""
}

// Resolver substitui as variáveis do texto; sem valor usa `padrao`; sem ambos → *ErroVariavel.
func Resolver(texto string, f FonteVariaveis, padrao map[string]string) (string, error) {
	vals := map[string]string{}
	pad := map[string]string{}
	for k, v := range padrao {
		pad[variaveis.Extrair("{" + k + "}")[0]] = v
	}
	for _, v := range variaveis.Extrair(texto) {
		val := f.Valor(v)
		if val == "" {
			val = strings.TrimSpace(pad[v])
		}
		if val == "" {
			return "", &ErroVariavel{Nome: v}
		}
		vals[v] = val
	}
	return variaveis.Resolver(texto, vals), nil
}

// ResolverEntrada aplica Resolver a todas as strings de um mapa JSON (ação executar_ia, nó ia).
func ResolverEntrada(entrada map[string]any, f FonteVariaveis) (map[string]any, error) {
	if entrada == nil {
		return map[string]any{}, nil
	}
	out := map[string]any{}
	for k, v := range entrada {
		r, err := resolverValor(v, f)
		if err != nil {
			return nil, err
		}
		out[k] = r
	}
	return out, nil
}

func resolverValor(v any, f FonteVariaveis) (any, error) {
	switch x := v.(type) {
	case string:
		return Resolver(x, f, nil)
	case map[string]any:
		return ResolverEntrada(x, f)
	case []any:
		l := make([]any, len(x))
		for i, it := range x {
			r, err := resolverValor(it, f)
			if err != nil {
				return nil, err
			}
			l[i] = r
		}
		return l, nil
	}
	return v, nil
}
