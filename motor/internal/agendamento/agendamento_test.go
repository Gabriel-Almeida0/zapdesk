package agendamento

import (
	"math/rand"
	"testing"
	"time"
)

func dt(dia, h, m, s int) time.Time { return time.Date(2026, 9, dia, h, m, s, 0, time.Local) }

var agora = dt(28, 10, 0, 0) // segunda-feira 10:00

func rnd() *rand.Rand { return rand.New(rand.NewSource(7)) }

func TestPrimeiroEnvioAgoraOuInicio(t *testing.T) {
	c := Config{IntervaloMin: 30 * time.Second, IntervaloMax: 90 * time.Second}
	if p, _ := ProximoEnvio(Estado{}, agora, c, rnd()); !p.Equal(agora) {
		t.Fatalf("primeiro envio deveria ser agora: %v", p)
	}
	c.InicioEm = dt(28, 14, 0, 0)
	if p, m := ProximoEnvio(Estado{}, agora, c, rnd()); !p.Equal(c.InicioEm) || m != MotivoInicio {
		t.Fatalf("início futuro: %v %s", p, m)
	}
	c.InicioEm = dt(27, 14, 0, 0)
	if p, _ := ProximoEnvio(Estado{}, agora, c, rnd()); !p.Equal(agora) {
		t.Fatalf("início no passado: %v", p)
	}
}

func TestIntervaloUniforme(t *testing.T) {
	c := Config{IntervaloMin: 30 * time.Second, IntervaloMax: 90 * time.Second}
	r := rnd()
	vistos := map[int]bool{}
	for i := 0; i < 2000; i++ {
		e := Estado{UltimoEnvio: agora}
		p, m := ProximoEnvio(e, agora, c, r)
		d := p.Sub(agora)
		if d < 30*time.Second || d > 90*time.Second || m != MotivoIntervalo {
			t.Fatalf("fora do intervalo: %v %s", d, m)
		}
		vistos[int(d/(10*time.Second))] = true
	}
	for faixa := 3; faixa < 9; faixa++ {
		if !vistos[faixa] {
			t.Fatalf("distribuição não cobre a faixa %d0 s", faixa)
		}
	}
	c.IntervaloMax = c.IntervaloMin
	if p, _ := ProximoEnvio(Estado{UltimoEnvio: agora}, agora, c, rnd()); p.Sub(agora) != 30*time.Second {
		t.Fatal("min == max deve ser exato")
	}
}

func TestPausaACadaN(t *testing.T) {
	c := Config{IntervaloMin: 10 * time.Second, IntervaloMax: 10 * time.Second, PausaACada: 5, PausaDuracao: 10 * time.Minute}
	e := Estado{UltimoEnvio: agora, EnviadosDesdePausa: 4}
	if p, _ := ProximoEnvio(e, agora, c, rnd()); p.Sub(agora) != 10*time.Second {
		t.Fatal("antes da pausa: intervalo normal")
	}
	e.EnviadosDesdePausa = 5
	if p, m := ProximoEnvio(e, agora, c, rnd()); p.Sub(agora) != 10*time.Minute || m != MotivoPausa {
		t.Fatalf("pausa: %v %s", p.Sub(agora), m)
	}
}

func TestLimitePorHoraJanelaMovel(t *testing.T) {
	c := Config{IntervaloMin: time.Second, IntervaloMax: time.Second, LimitePorHora: 3}
	envios := []time.Time{dt(28, 9, 10, 0), dt(28, 9, 30, 0), dt(28, 9, 59, 0)}
	e := Estado{UltimoEnvio: envios[2], EnviosRecentes: envios}
	p, m := ProximoEnvio(e, dt(28, 9, 59, 0), c, rnd())
	if !p.Equal(dt(28, 10, 10, 0)) || m != MotivoLimiteHora {
		t.Fatalf("janela móvel de 60 min: %v %s", p, m)
	}
	// nunca mais de 3 envios em qualquer período de 60 min numa simulação longa
	var todos []time.Time
	est := Estado{}
	t0 := dt(28, 8, 0, 0)
	for i := 0; i < 30; i++ {
		prox, _ := ProximoEnvio(est, t0, c, rnd())
		todos = append(todos, prox)
		est = Registrar(est, prox, c)
		t0 = prox
	}
	for i := range todos {
		n := 0
		for _, x := range todos {
			if !x.Before(todos[i]) && x.Before(todos[i].Add(time.Hour)) {
				n++
			}
		}
		if n > 3 {
			t.Fatalf("mais de 3 envios em 60 min a partir de %v", todos[i])
		}
	}
}

func TestLimitePorDiaZeraAMeiaNoite(t *testing.T) {
	c := Config{IntervaloMin: time.Minute, IntervaloMax: time.Minute, LimitePorDia: 2}
	e := Estado{UltimoEnvio: dt(28, 22, 0, 0), EnviosRecentes: []time.Time{dt(28, 21, 0, 0), dt(28, 22, 0, 0)}}
	p, m := ProximoEnvio(e, dt(28, 22, 0, 0), c, rnd())
	if !p.Equal(dt(29, 0, 0, 0)) || m != MotivoLimiteDia {
		t.Fatalf("limite por dia: %v %s", p, m)
	}
	// envios de ontem não contam
	e = Estado{UltimoEnvio: dt(28, 23, 59, 0), EnviosRecentes: []time.Time{dt(28, 23, 0, 0), dt(28, 23, 59, 0)}}
	if p, _ := ProximoEnvio(e, dt(29, 0, 0, 30), c, rnd()); !p.Equal(dt(29, 0, 1, 30)) && !p.Equal(dt(29, 0, 0, 59)) && p.Day() != 29 {
		t.Fatalf("novo dia: %v", p)
	}
}

func TestJanelaNormalECruzandoMeiaNoite(t *testing.T) {
	c := Config{IntervaloMin: time.Minute, IntervaloMax: time.Minute, Janela: &Janela{Inicio: 9 * 60, Fim: 18 * 60}}
	if p, m := ProximoEnvio(Estado{}, dt(28, 7, 0, 0), c, rnd()); !p.Equal(dt(28, 9, 0, 0)) || m != MotivoJanela {
		t.Fatalf("antes da janela: %v %s", p, m)
	}
	if p, _ := ProximoEnvio(Estado{UltimoEnvio: dt(28, 17, 59, 30)}, dt(28, 17, 59, 30), c, rnd()); !p.Equal(dt(29, 9, 0, 0)) {
		t.Fatalf("depois da janela vai para o dia seguinte: %v", p)
	}
	if p, _ := ProximoEnvio(Estado{}, dt(28, 12, 0, 0), c, rnd()); !p.Equal(dt(28, 12, 0, 0)) {
		t.Fatalf("dentro da janela: %v", p)
	}
	noite := Config{IntervaloMin: time.Minute, IntervaloMax: time.Minute, Janela: &Janela{Inicio: 22 * 60, Fim: 2 * 60}}
	if !DentroDaJanela(dt(28, 23, 0, 0), noite.Janela) || !DentroDaJanela(dt(29, 1, 59, 0), noite.Janela) || DentroDaJanela(dt(29, 2, 0, 0), noite.Janela) {
		t.Fatal("janela que cruza a meia-noite")
	}
	if p, _ := ProximoEnvio(Estado{}, dt(28, 12, 0, 0), noite, rnd()); !p.Equal(dt(28, 22, 0, 0)) {
		t.Fatalf("próxima janela noturna: %v", p)
	}
	if p, _ := ProximoEnvio(Estado{UltimoEnvio: dt(29, 1, 59, 30)}, dt(29, 1, 59, 30), noite, rnd()); !p.Equal(dt(29, 22, 0, 0)) {
		t.Fatalf("saiu da janela noturna: %v", p)
	}
}

func TestRetomadaAposSonoSemRajada(t *testing.T) {
	c := Config{IntervaloMin: 30 * time.Second, IntervaloMax: 60 * time.Second}
	e := Estado{UltimoEnvio: dt(28, 8, 0, 0)}
	acordou := dt(28, 11, 0, 0)
	p, _ := ProximoEnvio(e, acordou, c, rnd())
	d := p.Sub(acordou)
	if d < 30*time.Second || d > 60*time.Second {
		t.Fatalf("após o sono deve esperar um intervalo a partir de agora: %v", d)
	}
	// com janela: a regra 6 não pode jogar o envio para fora da janela
	c.Janela = &Janela{Inicio: 9 * 60, Fim: 18 * 60}
	p, _ = ProximoEnvio(Estado{UltimoEnvio: dt(28, 17, 0, 0)}, dt(28, 17, 59, 50), c, rnd())
	if !p.Equal(dt(29, 9, 0, 0)) {
		t.Fatalf("regra 6 + janela: %v", p)
	}
}

func TestEstimativa(t *testing.T) {
	c := Config{IntervaloMin: 30 * time.Second, IntervaloMax: 90 * time.Second}
	fim := Estimativa(Estado{}, agora, c, 11)
	if !fim.Equal(agora.Add(10 * time.Minute)) {
		t.Fatalf("11 envios com média de 60 s: %v", fim.Sub(agora))
	}
	c.LimitePorHora = 5
	fim = Estimativa(Estado{}, agora, c, 11)
	if fim.Sub(agora) < 2*time.Hour {
		t.Fatalf("limite por hora deve alongar a estimativa: %v", fim.Sub(agora))
	}
	c = Config{IntervaloMin: time.Minute, IntervaloMax: time.Minute, Janela: &Janela{Inicio: 9 * 60, Fim: 10 * 60}}
	fim = Estimativa(Estado{}, dt(28, 9, 0, 0), c, 61)
	if !fim.Equal(dt(29, 9, 0, 0)) {
		t.Fatalf("estimativa com janela: %v", fim)
	}
	if !Estimativa(Estado{}, agora, c, 0).IsZero() {
		t.Fatal("sem pendentes: zero")
	}
}

func TestParseJanela(t *testing.T) {
	j, err := ParseJanela("09:00", "18:30")
	if err != nil || j.Inicio != 540 || j.Fim != 1110 {
		t.Fatalf("parse: %+v %v", j, err)
	}
	for _, par := range [][2]string{{"9:00", "18:00"}, {"25:00", "18:00"}, {"09:00", "09:00"}, {"09:60", "10:00"}} {
		if _, err := ParseJanela(par[0], par[1]); err == nil {
			t.Fatalf("deveria rejeitar %v", par)
		}
	}
}
