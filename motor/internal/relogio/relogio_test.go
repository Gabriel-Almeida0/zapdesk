package relogio

import (
	"context"
	"errors"
	"testing"
	"time"
)

var t0 = time.Date(2026, 9, 28, 9, 0, 0, 0, time.Local)

func TestCongeladoDefinirAvancar(t *testing.T) {
	r := NovoCongelado(t0)
	if !r.Agora().Equal(t0) {
		t.Fatal("agora inicial")
	}
	r.Avancar(time.Hour)
	if !r.Agora().Equal(t0.Add(time.Hour)) {
		t.Fatal("avançar")
	}
	r.Definir(t0)
	if !r.Agora().Equal(t0) {
		t.Fatal("definir")
	}
}

func TestEsperarAcordaAoAvancar(t *testing.T) {
	r := NovoCongelado(t0)
	feito := make(chan error, 1)
	go func() { feito <- r.Esperar(context.Background(), t0.Add(10*time.Minute)) }()

	r.Avancar(5 * time.Minute)
	select {
	case <-feito:
		t.Fatal("não deveria acordar antes do prazo")
	case <-time.After(20 * time.Millisecond):
	}
	r.Avancar(5 * time.Minute)
	select {
	case err := <-feito:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("não acordou")
	}
}

func TestEsperarPassadoRetornaNaHora(t *testing.T) {
	r := NovoCongelado(t0)
	if err := r.Esperar(context.Background(), t0.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
}

func TestEsperarCancelado(t *testing.T) {
	r := NovoCongelado(t0)
	ctx, cancelar := context.WithCancel(context.Background())
	feito := make(chan error, 1)
	go func() { feito <- r.Esperar(ctx, t0.Add(time.Hour)) }()
	cancelar()
	select {
	case err := <-feito:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("esperado Canceled, veio %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("não cancelou")
	}
}

func TestCorrenteAnda(t *testing.T) {
	r := NovoControlavel(t0)
	if err := r.Esperar(context.Background(), t0.Add(15*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if r.Agora().Before(t0.Add(15 * time.Millisecond)) {
		t.Fatal("relógio corrente não andou")
	}
	r.Definir(t0.Add(24 * time.Hour))
	if r.Agora().Before(t0.Add(24 * time.Hour)) {
		t.Fatal("definir no corrente")
	}
}

func TestReal(t *testing.T) {
	var r Relogio = Real{}
	if time.Since(r.Agora()) > time.Second {
		t.Fatal("real")
	}
	if err := r.Esperar(context.Background(), time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
}
