package disparos

import (
	"testing"

	"zapdesk/motor/internal/dominio"
)

func ip(n int) *int { return &n }

func TestValidacaoLiteral(t *testing.T) {
	ok := NovoDisparo{ContaID: "c", Ritmo: &dominio.Ritmo{IntervaloMinS: 30, IntervaloMaxS: 90, LimitePorHora: ip(40), PausaACada: ip(50), PausaDuracaoS: ip(600)},
		Janela: &dominio.Janela{Inicio: "09:00", Fim: "18:00"}}
	if c := ValidarCampos(ok, "Oi {nome}"); len(c) != 0 {
		t.Fatalf("válido: %v", c)
	}
	casos := []struct {
		mudar func(*NovoDisparo)
		msg   string
		campo string
	}{
		{func(n *NovoDisparo) { n.Ritmo.IntervaloMinS = 0 }, "x", "ritmo.intervalo_min_s"},
		{func(n *NovoDisparo) { n.Ritmo.IntervaloMaxS = 10 }, "x", "ritmo.intervalo_max_s"},
		{func(n *NovoDisparo) { n.Ritmo.LimitePorHora = ip(0) }, "x", "ritmo.limite_por_hora"},
		{func(n *NovoDisparo) { n.Ritmo.LimitePorDia = ip(-1) }, "x", "ritmo.limite_por_dia"},
		{func(n *NovoDisparo) { n.Ritmo.PausaDuracaoS = nil }, "x", "ritmo.pausa_duracao_s"},
		{func(n *NovoDisparo) { n.Janela = &dominio.Janela{Inicio: "09:00"} }, "x", "janela"},
		{func(n *NovoDisparo) { n.Janela = &dominio.Janela{Inicio: "9h", Fim: "18:00"} }, "x", "janela"},
		{func(n *NovoDisparo) { n.Janela = &dominio.Janela{Inicio: "09:00", Fim: "09:00"} }, "x", "janela"},
		{func(n *NovoDisparo) {}, "   ", "mensagem"},
		{func(n *NovoDisparo) {}, string(make([]rune, 4097)), "mensagem"},
		{func(n *NovoDisparo) { s := ""; n.Nome = &s }, "x", "nome"},
		{func(n *NovoDisparo) { n.FalhasSeguidasMax = ip(-1) }, "x", "falhas_seguidas_max"},
		{func(n *NovoDisparo) { n.Ritmo = nil }, "x", "ritmo.intervalo_min_s"},
	}
	for i, c := range casos {
		n := ok
		r := *ok.Ritmo
		n.Ritmo = &r
		c.mudar(&n)
		msg := c.msg
		if c.campo == "mensagem" && len(msg) > 100 {
			msg = ""
			for j := 0; j < 4097; j++ {
				msg += "a"
			}
		}
		if erros := ValidarCampos(n, msg); erros[c.campo] == "" {
			t.Errorf("caso %d: esperado erro em %s, veio %v", i, c.campo, erros)
		}
	}
}

func TestAvisoRitmoAgressivo(t *testing.T) {
	casos := []struct {
		r     dominio.Ritmo
		aviso bool
	}{
		{dominio.Ritmo{IntervaloMinS: 30, IntervaloMaxS: 60, LimitePorHora: ip(40)}, false},
		{dominio.Ritmo{IntervaloMinS: 5, IntervaloMaxS: 60, LimitePorHora: ip(40)}, true},
		{dominio.Ritmo{IntervaloMinS: 30, IntervaloMaxS: 60, LimitePorHora: ip(121)}, true},
		{dominio.Ritmo{IntervaloMinS: 30, IntervaloMaxS: 60, LimitePorDia: ip(1001)}, true},
		{dominio.Ritmo{IntervaloMinS: 30, IntervaloMaxS: 60}, true},
	}
	for i, c := range casos {
		if AvisoRitmoAgressivo(c.r) != c.aviso {
			t.Errorf("caso %d", i)
		}
	}
}
