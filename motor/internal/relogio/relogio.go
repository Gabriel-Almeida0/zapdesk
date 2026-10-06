// Pacote relogio define o relógio injetável do motor (Constituição VI: nada de sleep real em
// testes de tempo). Há duas implementações: Real (sistema) e Controlavel (modo falso e testes).
package relogio

import (
	"context"
	"sync"
	"time"
)

// Relogio é a fonte de tempo do domínio.
type Relogio interface {
	Agora() time.Time
	// Esperar bloqueia até Agora() >= ate ou o contexto terminar (devolve ctx.Err()).
	Esperar(ctx context.Context, ate time.Time) error
}

// Real usa o relógio do sistema.
type Real struct{}

// Agora devolve time.Now().
func (Real) Agora() time.Time { return time.Now() }

// Esperar dorme até `ate`.
func (Real) Esperar(ctx context.Context, ate time.Time) error {
	d := time.Until(ate)
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Controlavel é um relógio que pode ser ajustado (Definir/Avancar). Em modo "corrente" o tempo
// continua passando a partir do instante ajustado (usado no motor falso, para o app ver os
// disparos andarem); em modo "congelado" só muda quando ajustado (testes determinísticos).
type Controlavel struct {
	mu        sync.Mutex
	congelado bool
	base      time.Time // instante lógico na referência
	ref       time.Time // instante real correspondente a base (modo corrente)
	mudou     chan struct{}
}

// NovoControlavel cria um relógio que anda em tempo real a partir de `inicio`.
func NovoControlavel(inicio time.Time) *Controlavel {
	return &Controlavel{base: inicio, ref: time.Now(), mudou: make(chan struct{})}
}

// NovoCongelado cria um relógio parado em `inicio`.
func NovoCongelado(inicio time.Time) *Controlavel {
	return &Controlavel{congelado: true, base: inicio, mudou: make(chan struct{})}
}

func (c *Controlavel) agoraSemTrava() time.Time {
	if c.congelado {
		return c.base
	}
	return c.base.Add(time.Since(c.ref))
}

// Agora devolve o instante lógico atual.
func (c *Controlavel) Agora() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.agoraSemTrava()
}

// Definir ajusta o relógio para `t` e acorda quem espera.
func (c *Controlavel) Definir(t time.Time) {
	c.mu.Lock()
	c.base = t
	c.ref = time.Now()
	c.avisarSemTrava()
	c.mu.Unlock()
}

// Avancar soma `d` ao relógio e acorda quem espera.
func (c *Controlavel) Avancar(d time.Duration) {
	c.mu.Lock()
	c.base = c.agoraSemTrava().Add(d)
	c.ref = time.Now()
	c.avisarSemTrava()
	c.mu.Unlock()
}

func (c *Controlavel) avisarSemTrava() {
	close(c.mudou)
	c.mudou = make(chan struct{})
}

// Esperar bloqueia até o instante lógico alcançar `ate`.
func (c *Controlavel) Esperar(ctx context.Context, ate time.Time) error {
	for {
		c.mu.Lock()
		falta := ate.Sub(c.agoraSemTrava())
		mudou := c.mudou
		congelado := c.congelado
		c.mu.Unlock()
		if falta <= 0 {
			return ctx.Err()
		}
		var timer <-chan time.Time
		var t *time.Timer
		if !congelado {
			t = time.NewTimer(falta)
			timer = t.C
		}
		select {
		case <-ctx.Done():
			if t != nil {
				t.Stop()
			}
			return ctx.Err()
		case <-mudou:
		case <-timer:
		}
		if t != nil {
			t.Stop()
		}
	}
}
