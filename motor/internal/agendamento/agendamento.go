// Pacote agendamento calcula quando enviar a próxima mensagem de um disparo (data-model.md ›
// Cálculo do agendamento). Funções puras: relógio e aleatório vêm de fora.
package agendamento

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"time"
)

// Motivos do próximo envio.
const (
	MotivoIntervalo  = "intervalo"
	MotivoInicio     = "inicio"
	MotivoPausa      = "pausa"
	MotivoLimiteHora = "limite_hora"
	MotivoLimiteDia  = "limite_dia"
	MotivoJanela     = "janela"
)

// Janela em minutos desde a meia-noite local; Fim < Inicio = cruza a meia-noite.
type Janela struct {
	Inicio, Fim int
}

// Config do ritmo.
type Config struct {
	IntervaloMin, IntervaloMax time.Duration
	LimitePorHora              int // 0 = sem limite
	LimitePorDia               int
	PausaACada                 int
	PausaDuracao               time.Duration
	InicioEm                   time.Time // zero = agora
	Janela                     *Janela
}

// Estado do disparo relevante ao agendamento.
type Estado struct {
	UltimoEnvio        time.Time   // zero = nenhum envio ainda
	EnviadosDesdePausa int         // envios desde a última pausa longa
	EnviosRecentes     []time.Time // envios das últimas ~24 h (limites)
}

// ParseJanela lê "HH:MM" + "HH:MM".
func ParseJanela(inicio, fim string) (*Janela, error) {
	i, err := parseHora(inicio)
	if err != nil {
		return nil, err
	}
	f, err := parseHora(fim)
	if err != nil {
		return nil, err
	}
	if i == f {
		return nil, errors.New("início e fim da janela devem ser diferentes")
	}
	return &Janela{Inicio: i, Fim: f}, nil
}

func parseHora(s string) (int, error) {
	var h, m int
	if len(s) != 5 || s[2] != ':' {
		return 0, fmt.Errorf("horário inválido %q (use HH:MM)", s)
	}
	if _, err := fmt.Sscanf(s, "%02d:%02d", &h, &m); err != nil || h > 23 || m > 59 || h < 0 || m < 0 {
		return 0, fmt.Errorf("horário inválido %q (use HH:MM)", s)
	}
	return h*60 + m, nil
}

// Formatar devolve "HH:MM".
func Formatar(min int) string { return fmt.Sprintf("%02d:%02d", min/60, min%60) }

func minutoDoDia(t time.Time) int { return t.Hour()*60 + t.Minute() }

// DentroDaJanela indica se t está dentro da janela (nil = sempre).
func DentroDaJanela(t time.Time, j *Janela) bool {
	if j == nil {
		return true
	}
	m := minutoDoDia(t)
	if j.Inicio < j.Fim {
		return m >= j.Inicio && m < j.Fim
	}
	return m >= j.Inicio || m < j.Fim
}

func noDia(t time.Time, minutos int) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), minutos/60, minutos%60, 0, 0, t.Location())
}

// ProximoInicioJanela devolve o primeiro instante >= t dentro da janela.
func ProximoInicioJanela(t time.Time, j *Janela) time.Time {
	if DentroDaJanela(t, j) {
		return t
	}
	hoje := noDia(t, j.Inicio)
	if !hoje.Before(t) {
		return hoje
	}
	amanha := t.AddDate(0, 0, 1)
	return noDia(amanha, j.Inicio)
}

func proximaMeiaNoite(t time.Time) time.Time {
	d := t.AddDate(0, 0, 1)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, t.Location())
}

func mesmoDia(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

func aleatorio(c Config, r *rand.Rand) time.Duration {
	min, max := c.IntervaloMin, c.IntervaloMax
	if max <= min || r == nil {
		if r == nil {
			return (min + max) / 2
		}
		return min
	}
	passos := int64((max - min) / time.Millisecond)
	return min + time.Duration(r.Int63n(passos+1))*time.Millisecond
}

// aplicarRestricoes aplica limites e janela até estabilizar.
func aplicarRestricoes(base time.Time, e Estado, c Config) (time.Time, string) {
	motivo := ""
	for iter := 0; iter < 1000; iter++ {
		mudou := false
		if c.LimitePorHora > 0 {
			var janela []time.Time
			for _, x := range e.EnviosRecentes {
				if x.After(base.Add(-time.Hour)) && !x.After(base) {
					janela = append(janela, x)
				}
			}
			if len(janela) >= c.LimitePorHora {
				sort.Slice(janela, func(i, j int) bool { return janela[i].Before(janela[j]) })
				novo := janela[len(janela)-c.LimitePorHora].Add(time.Hour)
				if novo.After(base) {
					base, motivo, mudou = novo, MotivoLimiteHora, true
				}
			}
		}
		if c.LimitePorDia > 0 {
			n := 0
			for _, x := range e.EnviosRecentes {
				if mesmoDia(x, base) && !x.After(base) {
					n++
				}
			}
			if n >= c.LimitePorDia {
				base, motivo, mudou = proximaMeiaNoite(base), MotivoLimiteDia, true
			}
		}
		if c.Janela != nil && !DentroDaJanela(base, c.Janela) {
			base, motivo, mudou = ProximoInicioJanela(base, c.Janela), MotivoJanela, true
		}
		if !mudou {
			break
		}
	}
	return base, motivo
}

// ProximoEnvio calcula o instante do próximo envio e o motivo da espera.
func ProximoEnvio(e Estado, agora time.Time, c Config, r *rand.Rand) (time.Time, string) {
	var base time.Time
	motivo := MotivoIntervalo
	if e.UltimoEnvio.IsZero() {
		base = agora
		if c.InicioEm.After(agora) {
			base, motivo = c.InicioEm, MotivoInicio
		}
	} else {
		base = e.UltimoEnvio.Add(aleatorio(c, r))
		if c.PausaACada > 0 && e.EnviadosDesdePausa >= c.PausaACada {
			if fim := e.UltimoEnvio.Add(c.PausaDuracao); fim.After(base) {
				base, motivo = fim, MotivoPausa
			}
		}
		// regra 6: acordou do sono / retomou depois de muito tempo → nunca rajada
		if agora.After(base) {
			base, motivo = agora.Add(aleatorio(c, r)), MotivoIntervalo
		}
	}
	if b, m := aplicarRestricoes(base, e, c); m != "" {
		base, motivo = b, m
	}
	return base, motivo
}

// Registrar atualiza o estado após um envio em `em`.
func Registrar(e Estado, em time.Time, c Config) Estado {
	if c.PausaACada > 0 && e.EnviadosDesdePausa >= c.PausaACada {
		e.EnviadosDesdePausa = 0
	}
	e.EnviadosDesdePausa++
	e.UltimoEnvio = em
	limite := em.Add(-25 * time.Hour)
	recentes := e.EnviosRecentes[:0:0]
	for _, x := range e.EnviosRecentes {
		if x.After(limite) {
			recentes = append(recentes, x)
		}
	}
	e.EnviosRecentes = append(recentes, em)
	return e
}

// MaxSimulacao limita o custo da estimativa.
const MaxSimulacao = 100000

// Estimativa simula os envios restantes com o intervalo médio e devolve o instante do último.
func Estimativa(e Estado, agora time.Time, c Config, pendentes int) time.Time {
	if pendentes <= 0 {
		return time.Time{}
	}
	media := (c.IntervaloMin + c.IntervaloMax) / 2
	cm := c
	cm.IntervaloMin, cm.IntervaloMax = media, media
	// atalho sem restrições
	if c.LimitePorHora == 0 && c.LimitePorDia == 0 && c.PausaACada == 0 && c.Janela == nil {
		primeiro, _ := ProximoEnvio(e, agora, cm, nil)
		return primeiro.Add(time.Duration(pendentes-1) * media)
	}
	t := agora
	est := Estado{UltimoEnvio: e.UltimoEnvio, EnviadosDesdePausa: e.EnviadosDesdePausa, EnviosRecentes: append([]time.Time(nil), e.EnviosRecentes...)}
	n := min(pendentes, MaxSimulacao)
	for i := 0; i < n; i++ {
		prox, _ := ProximoEnvio(est, t, cm, nil)
		est = Registrar(est, prox, cm)
		t = prox
	}
	if pendentes > n {
		t = t.Add(time.Duration(pendentes-n) * media)
	}
	return t
}
