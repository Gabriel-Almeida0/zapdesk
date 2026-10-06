// Pacote gatilhos casa fatos (fatos.Fato) com os gatilhos das automações ativas — funções puras
// (formatos.md › Gatilho; research.md › R11 para a cadeia).
package gatilhos

import (
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/fatos"
)

// ProfundidadeMaxima da cadeia causal: com 5 automações na cadeia, nada mais dispara.
const ProfundidadeMaxima = 5

// Candidata é a parte de uma automação ativa usada no casamento.
type Candidata struct {
	ID            string
	Prioridade    int
	CriadaEm      time.Time
	Contas        []string // nil = todas
	IncluirGrupos bool
	Gatilhos      []modelo.Gatilho
}

// Casamento de uma automação com o fato (primeiro gatilho que casou).
type Casamento struct {
	Automacao Candidata
	Indice    int
	Gatilho   modelo.Gatilho
}

// Casar devolve as automações cujo algum gatilho casa com o fato, em ordem de prioridade (menor
// primeiro) e criação. Automações já presentes na cadeia do fato são ignoradas; com a cadeia na
// profundidade máxima nada casa.
func Casar(cands []Candidata, f fatos.Fato) []Casamento {
	if len(f.Cadeia) >= ProfundidadeMaxima {
		return nil
	}
	naCadeia := map[string]bool{}
	for _, id := range f.Cadeia {
		naCadeia[id] = true
	}
	var r []Casamento
	for _, c := range cands {
		if naCadeia[c.ID] || !contaPermitida(c, f) || (f.Grupo && !c.IncluirGrupos) {
			continue
		}
		for i, g := range c.Gatilhos {
			if casaGatilho(g, f) {
				r = append(r, Casamento{Automacao: c, Indice: i, Gatilho: g})
				break
			}
		}
	}
	sort.SliceStable(r, func(i, j int) bool {
		a, b := r[i].Automacao, r[j].Automacao
		if a.Prioridade != b.Prioridade {
			return a.Prioridade < b.Prioridade
		}
		if !a.CriadaEm.Equal(b.CriadaEm) {
			return a.CriadaEm.Before(b.CriadaEm)
		}
		return a.ID < b.ID
	})
	return r
}

func contaPermitida(c Candidata, f fatos.Fato) bool {
	if c.Contas == nil || f.ContaID == "" {
		return true
	}
	for _, id := range c.Contas {
		if id == f.ContaID {
			return true
		}
	}
	return false
}

func casaGatilho(g modelo.Gatilho, f fatos.Fato) bool {
	switch g.Tipo {
	case modelo.GatilhoMensagemRecebida:
		if f.Tipo != fatos.MensagemRecebida || !tipoConversaOK(g.TipoConversa, f.Grupo) {
			return false
		}
		if g.Contem != nil && !strings.Contains(modelo.NormalizarTexto(f.Texto), modelo.NormalizarTexto(*g.Contem)) {
			return false
		}
		if g.Regex != nil {
			re := compilar(*g.Regex)
			if re == nil || !re.MatchString(f.Texto) {
				return false
			}
		}
		return !g.PrimeiraMensagem || f.Primeira
	case modelo.GatilhoPalavraChave:
		if f.Tipo != fatos.MensagemRecebida {
			return false
		}
		return CasaPalavras(g.Palavras, g.Modo, f.Texto)
	case modelo.GatilhoEtiqueta:
		return f.Tipo == fatos.Etiqueta && f.Evento == g.Evento && f.EtiquetaID == g.EtiquetaID
	case modelo.GatilhoEntrouEtapa:
		return f.Tipo == fatos.EntrouEtapa && f.FunilID == g.FunilID && f.EtapaID == g.EtapaID
	case modelo.GatilhoDisparoRespondeu:
		return f.Tipo == fatos.DisparoRespondeu && (g.DisparoID == nil || *g.DisparoID == "" || *g.DisparoID == f.DisparoID)
	case modelo.GatilhoLeadImportado:
		if f.Tipo != fatos.LeadImportado {
			return false
		}
		if len(g.Origens) == 0 {
			return true
		}
		for _, o := range g.Origens {
			if o == f.OrigemLead {
				return true
			}
		}
	}
	return false
}

func tipoConversaOK(tipo string, grupo bool) bool {
	switch tipo {
	case "qualquer":
		return true
	case "grupo":
		return grupo
	default:
		return !grupo
	}
}

// CasaPalavras aplica o gatilho palavra_chave: "palavra" (padrão) casa se alguma palavra/frase da
// lista aparece isolada; "mensagem_inteira" exige a mensagem normalizada igual à palavra.
func CasaPalavras(palavras []string, modo, texto string) bool {
	if modo == "mensagem_inteira" {
		t := modelo.NormalizarTexto(texto)
		for _, p := range palavras {
			if n := modelo.NormalizarTexto(p); n != "" && n == t {
				return true
			}
		}
		return false
	}
	tokens := modelo.Palavras(texto)
	for _, p := range palavras {
		alvo := modelo.Palavras(p)
		if len(alvo) == 0 {
			continue
		}
		for i := 0; i+len(alvo) <= len(tokens); i++ {
			ok := true
			for j := range alvo {
				if tokens[i+j] != alvo[j] {
					ok = false
					break
				}
			}
			if ok {
				return true
			}
		}
	}
	return false
}

var cacheRegex sync.Map // padrão → *regexp.Regexp (nil se inválido)

func compilar(padrao string) *regexp.Regexp {
	if v, ok := cacheRegex.Load(padrao); ok {
		re, _ := v.(*regexp.Regexp)
		return re
	}
	re, err := regexp.Compile(padrao)
	if err != nil {
		re = nil
	}
	cacheRegex.Store(padrao, re)
	return re
}
