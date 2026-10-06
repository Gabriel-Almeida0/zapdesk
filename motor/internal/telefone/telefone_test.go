package telefone

import "testing"

func TestNormalizar(t *testing.T) {
	casos := []struct {
		bruto, ddi, e164, motivo string
	}{
		{"(11) 99999-0000", "", "+5511999990000", ""},
		{"11999990000", "", "+5511999990000", ""},
		{"5511999990000", "", "+5511999990000", ""},
		{"+55 11 99999-0000", "", "+5511999990000", ""},
		{"0055 11 99999-0000", "", "+5511999990000", ""},
		{"1133334444", "", "+551133334444", ""},
		{"+1 415 555 2671", "", "+14155552671", ""},
		{"+1 415 555 2671", "55", "+14155552671", ""},
		{"", "", "", MotivoVazio},
		{"   ", "", "", MotivoVazio},
		{"abc", "", "", MotivoFormatoInvalido},
		{"+55 11 1234", "", "", MotivoNumeroInvalido},
		{"21 98888-7777", "", "+5521988887777", ""},
		{"4155552671", "1", "+14155552671", ""},
		{"+55 (21) 3333-4444", "", "+552133334444", ""},
	}
	for _, c := range casos {
		e164, motivo := Normalizar(c.bruto, c.ddi)
		if e164 != c.e164 || motivo != c.motivo {
			t.Errorf("Normalizar(%q, %q) = (%q, %q); esperado (%q, %q)", c.bruto, c.ddi, e164, motivo, c.e164, c.motivo)
		}
	}
}
