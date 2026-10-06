package esperas

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/automacoes/execucoes"
	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/automacoes/testeapoio"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/ids"
)

type chamada struct {
	id, tipo, motivo string // motivo vazio = executada
}

type manip struct {
	mu sync.Mutex
	l  []chamada
	c  chan chamada
}

func novoManip() *manip { return &manip{c: make(chan chamada, 100)} }

func (m *manip) Executar(_ context.Context, e dominio.Espera) error {
	m.registrar(chamada{id: e.ID, tipo: e.Tipo})
	return nil
}

func (m *manip) Abortar(_ context.Context, e dominio.Espera, motivo string) {
	m.registrar(chamada{id: e.ID, tipo: e.Tipo, motivo: motivo})
}

func (m *manip) registrar(c chamada) {
	m.mu.Lock()
	m.l = append(m.l, c)
	m.mu.Unlock()
	m.c <- c
}

func (m *manip) esperar(t *testing.T) chamada {
	t.Helper()
	select {
	case c := <-m.c:
		return c
	case <-time.After(5 * time.Second):
		t.Fatal("espera não foi tratada")
	}
	return chamada{}
}

func (m *manip) nada(t *testing.T) {
	t.Helper()
	select {
	case c := <-m.c:
		t.Fatalf("não deveria tratar: %+v", c)
	case <-time.After(50 * time.Millisecond):
	}
}

type verif struct {
	mu         sync.Mutex
	pausadas   map[string]bool
	desligadas map[string]bool
}

func (v *verif) ConversaPausada(_ context.Context, c string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.pausadas[c]
}
func (v *verif) ContaConectada(_ context.Context, c string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return !v.desligadas[c]
}

func montar(t *testing.T) (*testeapoio.Ambiente, *Agendador, *manip, *verif, string) {
	amb := testeapoio.Novo(t)
	m := novoManip()
	v := &verif{pausadas: map[string]bool{}, desligadas: map[string]bool{}}
	reg := execucoes.Novo(amb.Banco, amb.Barramento, amb.Relogio)
	a := Novo(amb.Banco, amb.Relogio, zerolog.Nop(), m, v, nil, reg)
	aut := amb.Automacao("fluxo", "F", []any{}, nil)
	return amb, a, m, v, aut
}

func (a *Agendador) gravarTeste(t *testing.T, amb *testeapoio.Ambiente, tipo, aut, conversa string, em time.Time) string {
	t.Helper()
	e := dominio.Espera{ID: ids.NovoEm(amb.Relogio.Agora()), Tipo: tipo, AutomacaoID: aut, RetomarEm: em}
	if conversa != "" {
		e.ConversaID = &conversa
	}
	if err := a.Gravar(amb.Ctx, e); err != nil {
		t.Fatal(err)
	}
	return e.ID
}

func TestAcordaNaMenorEReacordaComNovaMenor(t *testing.T) {
	amb, a, m, _, aut := montar(t)
	agora := amb.Relogio.Agora()
	e10 := a.gravarTeste(t, amb, dominio.EsperaAgendar, aut, "", agora.Add(10*time.Minute))
	e60 := a.gravarTeste(t, amb, dominio.EsperaAgendar, aut, "", agora.Add(60*time.Minute))
	ctx, cancelar := context.WithCancel(amb.Ctx)
	defer cancelar()
	a.Iniciar(ctx)
	defer a.Encerrar()
	m.nada(t)
	// Espera nova e menor reacorda o agendador.
	e5 := a.gravarTeste(t, amb, dominio.EsperaAgendar, aut, "", agora.Add(5*time.Minute))
	amb.Relogio.Avancar(5 * time.Minute)
	if c := m.esperar(t); c.id != e5 || c.motivo != "" {
		t.Fatalf("primeira: %+v", c)
	}
	m.nada(t)
	amb.Relogio.Avancar(5 * time.Minute)
	if c := m.esperar(t); c.id != e10 {
		t.Fatalf("segunda: %+v", c)
	}
	amb.Relogio.Avancar(50 * time.Minute)
	if c := m.esperar(t); c.id != e60 {
		t.Fatalf("terceira: %+v", c)
	}
	var n int
	amb.Banco.L().QueryRow(`SELECT count(*) FROM esperas`).Scan(&n)
	if n != 0 {
		t.Fatalf("esperas restantes: %d", n)
	}
}

func TestVencimento24h(t *testing.T) {
	amb, a, m, _, aut := montar(t)
	agora := amb.Relogio.Agora()
	ok := a.gravarTeste(t, amb, dominio.EsperaAgendar, aut, "", agora.Add(-23*time.Hour))
	velha := a.gravarTeste(t, amb, dominio.EsperaAguardar, aut, "", agora.Add(-25*time.Hour))
	a.Passada(amb.Ctx)
	c1, c2 := m.esperar(t), m.esperar(t)
	res := map[string]string{c1.id: c1.motivo, c2.id: c2.motivo}
	if res[velha] != MotivoExpirada || res[ok] != "" {
		t.Fatalf("vencimento: %+v", res)
	}
}

func TestConversaPausadaEContaDesconectada(t *testing.T) {
	amb, a, m, v, aut := montar(t)
	agora := amb.Relogio.Agora()
	pz := a.gravarTeste(t, amb, dominio.EsperaSemResposta, aut, "conv-pausada", agora)
	v.pausadas["conv-pausada"] = true
	a.Passada(amb.Ctx)
	if c := m.esperar(t); c.id != pz || c.motivo != MotivoConversaPausada {
		t.Fatalf("pausada: %+v", c)
	}

	dz := a.gravarTeste(t, amb, dominio.EsperaAguardar, aut, "conv-off", agora)
	v.desligadas["conv-off"] = true
	a.Passada(amb.Ctx)
	m.nada(t)
	e, ok, _ := armazenamento.ObterEspera(amb.Ctx, amb.Banco.L(), dz)
	if !ok || !e.RetomarEm.Equal(agora.Add(time.Minute)) {
		t.Fatalf("reprogramada para +1 min: %+v", e)
	}
	// Ainda desconectada depois de 1 min: reprograma de novo mantendo o vencimento original.
	amb.Relogio.Avancar(time.Minute)
	a.Passada(amb.Ctx)
	m.nada(t)
	// Reconectou: executa.
	amb.Relogio.Avancar(time.Minute)
	v.desligadas["conv-off"] = false
	a.Passada(amb.Ctx)
	if c := m.esperar(t); c.id != dz || c.motivo != "" {
		t.Fatalf("reconectou: %+v", c)
	}
	// Desconectada por mais de 24 h: expira.
	d2 := a.gravarTeste(t, amb, dominio.EsperaAguardar, aut, "conv-off2", amb.Relogio.Agora())
	v.desligadas["conv-off2"] = true
	a.Passada(amb.Ctx)
	amb.Relogio.Avancar(24*time.Hour + 2*time.Minute)
	a.Passada(amb.Ctx)
	if c := m.esperar(t); c.id != d2 || c.motivo != MotivoExpirada {
		t.Fatalf("desconectada 24 h: %+v", c)
	}
}

func TestSemRespostaSubstituidaEApagada(t *testing.T) {
	amb, a, m, _, aut := montar(t)
	agora := amb.Relogio.Agora()
	a.ProgramarSemResposta(amb.Ctx, aut, "cv", "m1", agora, 7200, 0)
	amb.Relogio.Avancar(time.Hour)
	a.ProgramarSemResposta(amb.Ctx, aut, "cv", "m2", amb.Relogio.Agora(), 7200, 0)
	l, _ := armazenamento.EsperasDaAutomacao(amb.Ctx, amb.Banco.L(), aut, dominio.EsperaSemResposta)
	if len(l) != 1 || *l[0].Referencia != "m2" || !l[0].RetomarEm.Equal(agora.Add(3*time.Hour)) {
		t.Fatalf("substituição: %+v", l)
	}
	amb.Relogio.Avancar(90 * time.Minute)
	a.CancelarSemResposta(amb.Ctx, "cv")
	amb.Relogio.Avancar(2 * time.Hour)
	a.Passada(amb.Ctx)
	m.nada(t)
}

func TestAgendamentoNaoRecuperaVarias(t *testing.T) {
	amb, a, m, _, aut := montar(t)
	g := modelo.Gatilho{Tipo: modelo.GatilhoAgendamento, IntervaloS: 3600}
	a.ProgramarAgendamento(amb.Ctx, aut, 0, g)
	// Motor "parado" por 5 h: ao voltar, executa uma vez e agenda a próxima a partir de agora.
	amb.Relogio.Avancar(5 * time.Hour)
	a.Passada(amb.Ctx)
	if c := m.esperar(t); c.tipo != dominio.EsperaAgendamento || c.motivo != "" {
		t.Fatalf("agendamento: %+v", c)
	}
	m.nada(t)
	l, _ := armazenamento.EsperasDaAutomacao(amb.Ctx, amb.Banco.L(), aut, dominio.EsperaAgendamento)
	if len(l) != 1 || !l[0].RetomarEm.Equal(amb.Relogio.Agora().Add(time.Hour)) {
		t.Fatalf("próxima: %+v", l)
	}
	// Cron: dias úteis às 9h a partir de segunda 10h → terça 9h.
	c := modelo.Gatilho{Tipo: modelo.GatilhoAgendamento, Cron: "0 9 * * 1-5"}
	prox, err := ProximaOcorrencia(c, time.Date(2026, 9, 28, 10, 0, 0, 0, time.Local))
	if err != nil || !prox.Equal(time.Date(2026, 9, 29, 9, 0, 0, 0, time.Local)) {
		t.Fatalf("cron: %v %v", prox, err)
	}
	prox, _ = ProximaOcorrencia(c, time.Date(2026, 10, 2, 9, 30, 0, 0, time.Local)) // sexta 9h30 → segunda
	if !prox.Equal(time.Date(2026, 10, 5, 9, 0, 0, 0, time.Local)) {
		t.Fatalf("cron fim de semana: %v", prox)
	}
	// Agendamento vencido há mais de 24 h: aborta "expirada" e agenda a próxima.
	amb.Relogio.Avancar(30 * time.Hour)
	a.Passada(amb.Ctx)
	if c := m.esperar(t); c.motivo != MotivoExpirada {
		t.Fatalf("agendamento velho: %+v", c)
	}
	l, _ = armazenamento.EsperasDaAutomacao(amb.Ctx, amb.Banco.L(), aut, dominio.EsperaAgendamento)
	if len(l) != 1 || !l[0].RetomarEm.After(amb.Relogio.Agora()) {
		t.Fatalf("reagendou: %+v", l)
	}
}

func TestRecuperacaoNaSubida(t *testing.T) {
	amb, a, _, _, autID := montar(t)
	reg := execucoes.Novo(amb.Banco, amb.Barramento, amb.Relogio)
	aut, _ := armazenamento.ObterAutomacao(amb.Ctx, amb.Banco.L(), autID)
	e1, _ := reg.Criar(amb.Ctx, execucoes.Nova{Automacao: aut, Origem: "gatilho"})
	e2, _ := reg.Criar(amb.Ctx, execucoes.Nova{Automacao: aut, Origem: "gatilho"})
	e2.Iniciar(amb.Ctx)
	if err := a.Recuperar(amb.Ctx); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{e1.ID(), e2.ID()} {
		d, _ := armazenamento.ObterExecucao(amb.Ctx, amb.Banco.L(), id)
		if d.Estado != dominio.ExecAbortada || *d.Motivo != MotivoInterrompida {
			t.Fatalf("recuperação: %+v", d.Execucao)
		}
	}
}
