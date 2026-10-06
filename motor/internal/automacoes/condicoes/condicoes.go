// Pacote condicoes avalia as condições de fluxos e dos nós de condição do chatbot (formatos.md ›
// Condições). Puro: os dados (etiquetas, posições no funil, lead, mensagem, variáveis e o
// instante no fuso local) vêm prontos em Dados.
package condicoes

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/importacao"
)

// Dados de contexto para avaliar as regras.
type Dados struct {
	Agora      time.Time         // instante no fuso local
	ContaID    string            // conta da conversa/execução
	Etiquetas  map[string]bool   // etiquetas do contato
	Posicoes   map[string]string // funil_id → etapa_id do lead
	TemLead    bool
	CamposLead map[string]string // inclui "nome" se o lead tiver nome
	Texto      *string           // texto da mensagem do gatilho (nil = sem mensagem)
	Variaveis  map[string]string // variáveis da execução/sessão
}

// Avaliar devolve se as condições são atendidas (nil ou sem regras = verdadeiro).
func Avaliar(c *modelo.Condicoes, d Dados) bool {
	if c == nil || len(c.Regras) == 0 {
		return true
	}
	alguma := c.Modo == modelo.ModoAlguma
	for _, r := range c.Regras {
		v := regra(r, d)
		if alguma && v {
			return true
		}
		if !alguma && !v {
			return false
		}
	}
	return !alguma
}

func regra(r modelo.Regra, d Dados) bool {
	switch r.Tipo {
	case modelo.RegraEtiqueta:
		tem := d.Etiquetas[r.EtiquetaID]
		if r.Operador == "nao_tem" {
			return !tem
		}
		return tem
	case modelo.RegraEtapa:
		etapa, noFunil := d.Posicoes[r.FunilID]
		esta := noFunil && (r.EtapaID == nil || *r.EtapaID == "" || *r.EtapaID == etapa)
		if r.Operador == "nao_esta" {
			return !esta
		}
		return esta
	case modelo.RegraCampoLead:
		v, ok := "", false
		if d.TemLead {
			v, ok = d.CamposLead[importacaoChave(r.Campo)]
		}
		return comparar(r.Operador, v, ok && strings.TrimSpace(v) != "", r.Valor)
	case modelo.RegraHorario:
		return horario(r, d.Agora)
	case modelo.RegraTexto:
		if d.Texto == nil {
			return false
		}
		return comparar(r.Operador, *d.Texto, true, r.Valor)
	case modelo.RegraConta:
		for _, c := range r.ContaIDs {
			if c == d.ContaID {
				return true
			}
		}
		return false
	case modelo.RegraVariavel:
		v, ok := d.Variaveis[r.Variavel]
		return comparar(r.Operador, v, ok && strings.TrimSpace(v) != "", r.Valor)
	}
	return false
}

// importacaoChave normaliza o nome do campo como na importação (minúsculas, sem acento, "_").
func importacaoChave(s string) string { return importacao.NormalizarChave(s) }

// comparar aplica os operadores de campo/texto/variável. Texto: sem diferenciar maiúsculas e
// acentos; maior/menor: numérico (vírgula decimal aceita; não numérico = falso).
func comparar(op, v string, existe bool, alvo *string) bool {
	a := ""
	if alvo != nil {
		a = *alvo
	}
	switch op {
	case "existe":
		return existe
	case "nao_existe":
		return !existe
	case "igual":
		return existe && modelo.NormalizarTexto(v) == modelo.NormalizarTexto(a)
	case "diferente":
		return !existe || modelo.NormalizarTexto(v) != modelo.NormalizarTexto(a)
	case "contem":
		return existe && strings.Contains(modelo.NormalizarTexto(v), modelo.NormalizarTexto(a))
	case "maior", "menor":
		x, ok1 := numero(v)
		y, ok2 := numero(a)
		if !existe || !ok1 || !ok2 {
			return false
		}
		if op == "maior" {
			return x > y
		}
		return x < y
	case "regex":
		re := compilar(a)
		return existe && re != nil && re.MatchString(v)
	}
	return false
}

// numero lê "1.234,50", "1234.5", "42".
func numero(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, ",", ".")
	}
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

func minutos(hhmm string) (int, bool) {
	var h, m int
	if len(hhmm) != 5 || hhmm[2] != ':' {
		return 0, false
	}
	h, e1 := strconv.Atoi(hhmm[:2])
	m, e2 := strconv.Atoi(hhmm[3:])
	return h*60 + m, e1 == nil && e2 == nil
}

// diaSemana: 1 = segunda … 7 = domingo.
func diaSemana(t time.Time) int {
	d := int(t.Weekday())
	if d == 0 {
		return 7
	}
	return d
}

func horario(r modelo.Regra, agora time.Time) bool {
	ini, ok1 := minutos(r.Inicio)
	fim, ok2 := minutos(r.Fim)
	if !ok1 || !ok2 {
		return false
	}
	agora = agora.Local()
	m := agora.Hour()*60 + agora.Minute()
	dia := diaSemana(agora)
	dentro := false
	switch {
	case ini == fim:
		dentro = true
	case ini < fim:
		dentro = m >= ini && m < fim
	default: // atravessa a meia-noite: a parte da madrugada pertence ao dia anterior
		if m >= ini {
			dentro = true
		} else if m < fim {
			dentro = true
			dia = diaSemana(agora.AddDate(0, 0, -1))
		}
	}
	if !dentro {
		return false
	}
	if len(r.Dias) == 0 {
		return true
	}
	for _, d := range r.Dias {
		if d == dia {
			return true
		}
	}
	return false
}

var cache sync.Map

func compilar(p string) *regexp.Regexp {
	if v, ok := cache.Load(p); ok {
		re, _ := v.(*regexp.Regexp)
		return re
	}
	re, err := regexp.Compile(p)
	if err != nil {
		re = nil
	}
	cache.Store(p, re)
	return re
}
