package condicoes

import (
	"testing"
	"time"

	"zapdesk/motor/internal/automacoes/modelo"
)

func s(v string) *string { return &v }

// segunda 28/09/2026 10:00 local.
var seg10 = time.Date(2026, 9, 28, 10, 0, 0, 0, time.Local)

func base() Dados {
	return Dados{
		Agora:      seg10,
		ContaID:    "c1",
		Etiquetas:  map[string]bool{"e1": true},
		Posicoes:   map[string]string{"f1": "s1"},
		TemLead:    true,
		CamposLead: map[string]string{"empresa": "ACME Ltda", "idade": "42", "valor": "1.234,50"},
		Texto:      s("Quero o BOLETO"),
		Variaveis:  map[string]string{"email": "a@empresa.com"},
	}
}

func avaliar(d Dados, modo string, rs ...modelo.Regra) bool {
	return Avaliar(&modelo.Condicoes{Modo: modo, Regras: rs}, d)
}

func TestVazioEVerdadeiro(t *testing.T) {
	if !Avaliar(nil, base()) || !avaliar(base(), "todas") || !avaliar(base(), "alguma") {
		t.Fatal("condições vazias = verdadeiro")
	}
}

func TestTodasEAlguma(t *testing.T) {
	v := modelo.Regra{Tipo: modelo.RegraEtiqueta, Operador: "tem", EtiquetaID: "e1"}
	f := modelo.Regra{Tipo: modelo.RegraEtiqueta, Operador: "tem", EtiquetaID: "e2"}
	if avaliar(base(), "todas", v, f) || !avaliar(base(), "alguma", v, f) || avaliar(base(), "alguma", f, f) {
		t.Fatal("todas/alguma")
	}
}

func TestOperadores(t *testing.T) {
	d := base()
	casos := []struct {
		r    modelo.Regra
		quer bool
	}{
		{modelo.Regra{Tipo: modelo.RegraEtiqueta, Operador: "nao_tem", EtiquetaID: "e1"}, false},
		{modelo.Regra{Tipo: modelo.RegraEtiqueta, Operador: "nao_tem", EtiquetaID: "e9"}, true},
		{modelo.Regra{Tipo: modelo.RegraEtapa, Operador: "esta", FunilID: "f1", EtapaID: s("s1")}, true},
		{modelo.Regra{Tipo: modelo.RegraEtapa, Operador: "esta", FunilID: "f1", EtapaID: s("s2")}, false},
		{modelo.Regra{Tipo: modelo.RegraEtapa, Operador: "esta", FunilID: "f1"}, true},
		{modelo.Regra{Tipo: modelo.RegraEtapa, Operador: "esta", FunilID: "f2"}, false},
		{modelo.Regra{Tipo: modelo.RegraEtapa, Operador: "nao_esta", FunilID: "f2"}, true},
		{modelo.Regra{Tipo: modelo.RegraEtapa, Operador: "nao_esta", FunilID: "f1", EtapaID: s("s2")}, true},
		{modelo.Regra{Tipo: modelo.RegraCampoLead, Campo: "empresa", Operador: "igual", Valor: s("acme ltda")}, true},
		{modelo.Regra{Tipo: modelo.RegraCampoLead, Campo: "empresa", Operador: "diferente", Valor: s("ACME Ltda")}, false},
		{modelo.Regra{Tipo: modelo.RegraCampoLead, Campo: "empresa", Operador: "contem", Valor: s("acme")}, true},
		{modelo.Regra{Tipo: modelo.RegraCampoLead, Campo: "empresa", Operador: "existe"}, true},
		{modelo.Regra{Tipo: modelo.RegraCampoLead, Campo: "cidade", Operador: "existe"}, false},
		{modelo.Regra{Tipo: modelo.RegraCampoLead, Campo: "cidade", Operador: "nao_existe"}, true},
		{modelo.Regra{Tipo: modelo.RegraCampoLead, Campo: "idade", Operador: "maior", Valor: s("40")}, true},
		{modelo.Regra{Tipo: modelo.RegraCampoLead, Campo: "idade", Operador: "menor", Valor: s("40")}, false},
		{modelo.Regra{Tipo: modelo.RegraCampoLead, Campo: "valor", Operador: "maior", Valor: s("1000")}, true},
		{modelo.Regra{Tipo: modelo.RegraCampoLead, Campo: "empresa", Operador: "maior", Valor: s("1")}, false},
		{modelo.Regra{Tipo: modelo.RegraTexto, Operador: "contem", Valor: s("boleto")}, true},
		{modelo.Regra{Tipo: modelo.RegraTexto, Operador: "igual", Valor: s("quero o boleto")}, true},
		{modelo.Regra{Tipo: modelo.RegraTexto, Operador: "regex", Valor: s("^Quero")}, true},
		{modelo.Regra{Tipo: modelo.RegraTexto, Operador: "regex", Valor: s("^boleto")}, false},
		{modelo.Regra{Tipo: modelo.RegraConta, ContaIDs: []string{"c2", "c1"}}, true},
		{modelo.Regra{Tipo: modelo.RegraConta, ContaIDs: []string{"c2"}}, false},
		{modelo.Regra{Tipo: modelo.RegraVariavel, Variavel: "email", Operador: "contem", Valor: s("@empresa.com")}, true},
		{modelo.Regra{Tipo: modelo.RegraVariavel, Variavel: "email", Operador: "regex", Valor: s(`@empresa\.com$`)}, true},
		{modelo.Regra{Tipo: modelo.RegraVariavel, Variavel: "cpf", Operador: "nao_existe"}, true},
	}
	for i, c := range casos {
		if got := avaliar(d, "todas", c.r); got != c.quer {
			t.Errorf("caso %d (%s %s): %v, esperado %v", i, c.r.Tipo, c.r.Operador, got, c.quer)
		}
	}
	// Sem mensagem no gatilho, regra de texto é falsa; sem lead, campo existe é falso.
	d.Texto = nil
	d.TemLead = false
	d.CamposLead = nil
	if avaliar(d, "alguma", modelo.Regra{Tipo: modelo.RegraTexto, Operador: "contem", Valor: s("x")}) {
		t.Fatal("texto sem mensagem")
	}
	if avaliar(d, "todas", modelo.Regra{Tipo: modelo.RegraCampoLead, Campo: "empresa", Operador: "existe"}) {
		t.Fatal("campo sem lead")
	}
}

func TestHorario(t *testing.T) {
	r := modelo.Regra{Tipo: modelo.RegraHorario, Inicio: "09:00", Fim: "18:00", Dias: []int{1, 2, 3, 4, 5}}
	em := func(dia, h, m int) Dados {
		d := base()
		d.Agora = time.Date(2026, 9, 27+dia, h, m, 0, 0, time.Local) // 28/09 = segunda (dia 1)
		return d
	}
	if !avaliar(em(1, 9, 0), "todas", r) || !avaliar(em(1, 17, 59), "todas", r) || avaliar(em(1, 18, 0), "todas", r) || avaliar(em(1, 8, 59), "todas", r) {
		t.Fatal("janela comercial")
	}
	if avaliar(em(6, 10, 0), "todas", r) || avaliar(em(7, 10, 0), "todas", r) {
		t.Fatal("fim de semana (6 = sábado, 7 = domingo)")
	}
	// Atravessa a meia-noite: sexta 22:00 → sábado 06:00 conta como sexta.
	n := modelo.Regra{Tipo: modelo.RegraHorario, Inicio: "22:00", Fim: "06:00", Dias: []int{5}}
	if !avaliar(em(5, 23, 0), "todas", n) || !avaliar(em(6, 5, 59), "todas", n) || avaliar(em(6, 6, 0), "todas", n) || avaliar(em(6, 23, 0), "todas", n) || avaliar(em(5, 21, 0), "todas", n) {
		t.Fatal("atravessando a meia-noite")
	}
	// Sem dias = todos os dias.
	if !avaliar(em(7, 23, 30), "todas", modelo.Regra{Tipo: modelo.RegraHorario, Inicio: "22:00", Fim: "06:00"}) {
		t.Fatal("sem dias")
	}
}
