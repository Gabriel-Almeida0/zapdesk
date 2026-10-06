package importacao

import (
	"testing"
	"time"

	"zapdesk/motor/internal/relogio"
)

func TestPreviaExpiraEm30Min(t *testing.T) {
	r := relogio.NovoCongelado(time.Date(2026, 9, 28, 10, 0, 0, 0, time.Local))
	p := NovasPrevias(r)
	pv := p.Guardar("a.csv", Planilha{Colunas: []string{"x"}})
	r.Avancar(29 * time.Minute)
	if _, err := p.Obter(pv.ID); err != nil {
		t.Fatal("ainda válida")
	}
	r.Avancar(time.Minute)
	if _, err := p.Obter(pv.ID); err == nil {
		t.Fatal("deveria ter expirado")
	}
}
