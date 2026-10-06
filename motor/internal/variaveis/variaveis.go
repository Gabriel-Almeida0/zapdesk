// Pacote variaveis extrai e resolve variáveis de mensagem ({nome}, {cidade}...) conforme
// data-model.md › Resolução de variáveis. {{ e }} escapam chaves literais.
package variaveis

import (
	"strings"

	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/importacao"
)

type pedaco struct {
	literal  string
	variavel string // chave normalizada; vazio = literal
	bruto    string
}

// analisar quebra o texto em literais e variáveis.
func analisar(texto string) []pedaco {
	var partes []pedaco
	var lit strings.Builder
	r := []rune(texto)
	for i := 0; i < len(r); i++ {
		c := r[i]
		if c == '{' && i+1 < len(r) && r[i+1] == '{' {
			lit.WriteRune('{')
			i++
			continue
		}
		if c == '}' && i+1 < len(r) && r[i+1] == '}' {
			lit.WriteRune('}')
			i++
			continue
		}
		if c == '{' {
			fim := -1
			for j := i + 1; j < len(r); j++ {
				if r[j] == '}' {
					fim = j
					break
				}
				if r[j] == '{' || r[j] == '\n' {
					break
				}
			}
			if fim > i {
				nome := string(r[i+1 : fim])
				chave := importacao.NormalizarChave(nome)
				if chave != "" {
					if lit.Len() > 0 {
						partes = append(partes, pedaco{literal: lit.String()})
						lit.Reset()
					}
					partes = append(partes, pedaco{variavel: chave, bruto: string(r[i : fim+1])})
					i = fim
					continue
				}
			}
		}
		lit.WriteRune(c)
	}
	if lit.Len() > 0 {
		partes = append(partes, pedaco{literal: lit.String()})
	}
	return partes
}

// Extrair devolve as variáveis (chaves normalizadas, sem repetir, na ordem).
func Extrair(texto string) []string {
	vistas := map[string]bool{}
	lista := []string{}
	for _, p := range analisar(texto) {
		if p.variavel != "" && !vistas[p.variavel] {
			vistas[p.variavel] = true
			lista = append(lista, p.variavel)
		}
	}
	return lista
}

// Resolver substitui as variáveis conhecidas; desconhecidas ficam como estão.
func Resolver(texto string, valores map[string]string) string {
	var b strings.Builder
	for _, p := range analisar(texto) {
		if p.variavel == "" {
			b.WriteString(p.literal)
			continue
		}
		if v, ok := valores[p.variavel]; ok {
			b.WriteString(v)
		} else {
			b.WriteString(p.bruto)
		}
	}
	return b.String()
}

// ValoresDoLead resolve cada variável: {nome} → lead.nome; {telefone} → lead.telefone;
// {x} → lead.campos[x]; vazio → padrao[x]; senão fica em `faltando`.
func ValoresDoLead(vars []string, lead dominio.Lead, padrao map[string]string) (map[string]string, []string) {
	valores := map[string]string{}
	faltando := []string{}
	for _, v := range vars {
		val := ""
		switch v {
		case "nome":
			if lead.Nome != nil {
				val = strings.TrimSpace(*lead.Nome)
			}
		case "telefone":
			val = lead.Telefone
		default:
			val = strings.TrimSpace(lead.Campos[v])
		}
		if val == "" {
			val = strings.TrimSpace(padrao[v])
		}
		if val == "" {
			faltando = append(faltando, v)
			continue
		}
		valores[v] = val
	}
	return valores, faltando
}

// Faltante descreve um destinatário incompleto.
type Faltante struct {
	DestinatarioLinha int      `json:"destinatario_linha"`
	Telefone          string   `json:"telefone"`
	Variaveis         []string `json:"variaveis"`
}

// Faltando lista os destinatários (na ordem, linha 1-based) sem valor para alguma variável.
func Faltando(mensagem string, leads []dominio.Lead, padrao map[string]string) []Faltante {
	vars := Extrair(mensagem)
	lista := []Faltante{}
	if len(vars) == 0 {
		return lista
	}
	for i, l := range leads {
		if _, f := ValoresDoLead(vars, l, padrao); len(f) > 0 {
			lista = append(lista, Faltante{DestinatarioLinha: i + 1, Telefone: l.Telefone, Variaveis: f})
		}
	}
	return lista
}
