package eventos

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSeqCrescenteEEntrega(t *testing.T) {
	b := Novo(nil)
	a1 := b.Assinar(10)
	a2 := b.Assinar(10)
	b.Publicar(ContaAtualizada, "c1", map[string]string{"id": "c1"})
	b.Publicar(EtiquetasAlteradas, "", nil)
	for _, a := range []*Assinatura{a1, a2} {
		e1, e2 := <-a.C, <-a.C
		if e1.Seq != 1 || e2.Seq != 2 {
			t.Fatalf("seq: %d %d", e1.Seq, e2.Seq)
		}
		if e1.ContaID == nil || *e1.ContaID != "c1" || e2.ContaID != nil {
			t.Fatal("conta_id")
		}
	}
	j, _ := json.Marshal(<-func() chan Evento { c := make(chan Evento, 1); c <- b.Publicar(EtiquetasAlteradas, "", nil); return c }())
	if !strings.Contains(string(j), `"conta_id":null`) || !strings.Contains(string(j), `"dados":{}`) {
		t.Fatalf("envelope: %s", j)
	}
}

func TestAssinanteLentoPerdeEventos(t *testing.T) {
	b := Novo(nil)
	lento := b.Assinar(2)
	rapido := b.Assinar(100)
	for i := 0; i < 5; i++ {
		b.Publicar(MensagemNova, "c", nil)
	}
	if lento.Perdidos() != 3 {
		t.Fatalf("perdidos = %d", lento.Perdidos())
	}
	if rapido.Perdidos() != 0 || len(rapido.C) != 5 {
		t.Fatal("rápido não deveria perder")
	}
	<-lento.C
	<-lento.C
	b.Publicar(MensagemNova, "c", nil)
	ev := <-lento.C
	if ev.Seq != 6 {
		t.Fatalf("lacuna esperada: seq %d", ev.Seq)
	}
}

func TestCancelar(t *testing.T) {
	b := Novo(nil)
	a := b.Assinar(1)
	a.Cancelar()
	a.Cancelar()
	b.Publicar(MensagemNova, "", nil)
	if _, ok := <-a.C; ok {
		t.Fatal("canal deveria estar fechado")
	}
}

func TestTodosOsTipos(t *testing.T) {
	// 19 do MVP + 15 da feature 002 (specs/002-automacoes/contracts/eventos-ws.md).
	if len(Todos) != 34 {
		t.Fatalf("contrato tem 34 tipos, lista tem %d", len(Todos))
	}
	vistos := map[string]bool{}
	for _, t2 := range Todos {
		if vistos[t2] {
			t.Fatalf("tipo repetido: %s", t2)
		}
		vistos[t2] = true
	}
}
