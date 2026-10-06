package ids

import "testing"

func TestNovoUnicoEOrdenado(t *testing.T) {
	vistos := map[string]bool{}
	ant := ""
	for i := 0; i < 1000; i++ {
		id := Novo()
		if len(id) != 26 || vistos[id] {
			t.Fatalf("id inválido/repetido: %s", id)
		}
		if id <= ant {
			t.Fatalf("fora de ordem: %s <= %s", id, ant)
		}
		vistos[id] = true
		ant = id
	}
}
