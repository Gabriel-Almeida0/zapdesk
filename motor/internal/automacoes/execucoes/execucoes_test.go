package execucoes

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/automacoes/testeapoio"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/eventos"
)

func montar(t *testing.T) (*testeapoio.Ambiente, *Registro, dominio.Automacao) {
	amb := testeapoio.Novo(t)
	r := Novo(amb.Banco, amb.Barramento, amb.Relogio)
	id := amb.Automacao("fluxo", "Minha automação", []any{}, nil)
	a, err := armazenamento.ObterAutomacao(amb.Ctx, amb.Banco.L(), id)
	if err != nil {
		t.Fatal(err)
	}
	return amb, r, a
}

func TestTransicoesEEventos(t *testing.T) {
	amb, r, a := montar(t)
	e, err := r.Criar(amb.Ctx, Nova{Automacao: a, Origem: "gatilho", Gatilho: dominio.GatilhoExecucao{Tipo: "manual"}})
	if err != nil {
		t.Fatal(err)
	}
	if e.Estado() != dominio.ExecNaFila {
		t.Fatal("começa na fila")
	}
	e.Iniciar(amb.Ctx)
	amb.Relogio.Avancar(2 * time.Second)
	e.RegistrarAcao(amb.Ctx, dominio.AcaoRegistrada{Tipo: "enviar_texto", Resultado: dominio.AcaoOK})
	retomar := amb.Relogio.Agora().Add(2 * time.Hour)
	e.Aguardar(amb.Ctx, retomar, 1)
	d, _ := armazenamento.ObterExecucao(amb.Ctx, amb.Banco.L(), e.ID())
	if d.Estado != dominio.ExecAguardando || d.RetomarEm == nil || !d.RetomarEm.Equal(retomar) || d.PassoAtual == nil || *d.PassoAtual != 1 {
		t.Fatalf("aguardando: %+v", d.Execucao)
	}
	amb.Relogio.Avancar(2 * time.Hour) // espera não conta na duração
	e.Retomar(amb.Ctx)
	amb.Relogio.Avancar(3 * time.Second)
	e.Finalizar(amb.Ctx, Fim{Estado: dominio.ExecOK})
	d, _ = armazenamento.ObterExecucao(amb.Ctx, amb.Banco.L(), e.ID())
	if d.Estado != dominio.ExecOK || d.FinalizadaEm == nil || d.DuracaoMs == nil || *d.DuracaoMs != 5000 || len(d.Acoes) != 1 || d.AutomacaoNome != "Minha automação" {
		t.Fatalf("final: %+v dur=%v", d.Execucao, d.DuracaoMs)
	}
	if len(amb.EventosDoTipo(eventos.AutomacaoExecucaoIniciada)) != 1 || len(amb.EventosDoTipo(eventos.AutomacaoExecucaoFinalizada)) != 1 {
		t.Fatal("eventos iniciada/finalizada")
	}
	// Finalizar de novo não muda nada.
	e.Finalizar(amb.Ctx, Fim{Estado: dominio.ExecErro, Erro: "x"})
	if d2, _ := armazenamento.ObterExecucao(amb.Ctx, amb.Banco.L(), e.ID()); d2.Estado != dominio.ExecOK {
		t.Fatal("estado final alterado")
	}
}

func TestLogLimitadoEAcoesLimitadas(t *testing.T) {
	amb, r, a := montar(t)
	e, _ := r.Criar(amb.Ctx, Nova{Automacao: a, Origem: "teste", Simulacao: true})
	e.Iniciar(amb.Ctx)
	linha := strings.Repeat("x", 1000)
	for i := 0; i < 80; i++ {
		e.Logar("info", linha)
	}
	for i := 0; i < 210; i++ {
		e.RegistrarAcao(amb.Ctx, dominio.AcaoRegistrada{Tipo: "http", Resultado: dominio.AcaoOK})
	}
	e.Finalizar(amb.Ctx, Fim{Estado: dominio.ExecSimulacao})
	d, _ := armazenamento.ObterExecucao(amb.Ctx, amb.Banco.L(), e.ID())
	if len(d.Log) > LimiteLog || !d.LogTruncado {
		t.Fatalf("log %d truncado=%v", len(d.Log), d.LogTruncado)
	}
	if len(d.Acoes) != LimiteAcoes {
		t.Fatalf("ações: %d", len(d.Acoes))
	}
}

func TestTokensPorModelo(t *testing.T) {
	amb, r, a := montar(t)
	e, _ := r.Criar(amb.Ctx, Nova{Automacao: a, Origem: "gatilho"})
	e.Iniciar(amb.Ctx)
	e.SomarTokens("claude-sonnet-5", 100, 20)
	e.SomarTokens("claude-sonnet-5", 50, 5)
	e.SomarTokens("claude-haiku-4-5", 10, 1)
	e.Finalizar(amb.Ctx, Fim{Estado: dominio.ExecOK, Retorno: json.RawMessage(`{"ok":true}`)})
	d, _ := armazenamento.ObterExecucao(amb.Ctx, amb.Banco.L(), e.ID())
	tk := d.Tokens
	if tk.Entrada != 160 || tk.Saida != 26 || tk.PorModelo["claude-sonnet-5"].Chamadas != 2 || tk.PorModelo["claude-haiku-4-5"].Entrada != 10 {
		t.Fatalf("tokens: %+v", tk)
	}
	if string(d.Retorno) != `{"ok":true}` {
		t.Fatalf("retorno: %s", d.Retorno)
	}
}

func TestRetencao500(t *testing.T) {
	amb, r, a := montar(t)
	var primeira string
	for i := 0; i < RetencaoTeste+3; i++ {
		e, _ := r.Criar(amb.Ctx, Nova{Automacao: a, Origem: "gatilho"})
		if i == 0 {
			primeira = e.ID()
			continue // a primeira fica "na_fila" e não é apagada pela retenção
		}
		e.Finalizar(amb.Ctx, Fim{Estado: dominio.ExecOK})
		amb.Relogio.Avancar(time.Millisecond)
	}
	var n int
	amb.Banco.L().QueryRow(`SELECT count(*) FROM execucoes WHERE automacao_id = ?`, a.ID).Scan(&n)
	if n != armazenamento.RetencaoExecucoes+1 {
		t.Fatalf("retenção: %d", n)
	}
	if _, err := armazenamento.ObterExecucao(amb.Ctx, amb.Banco.L(), primeira); err != nil {
		t.Fatal("execução não final foi apagada pela retenção")
	}
}

// RetencaoTeste cria execuções além do limite.
const RetencaoTeste = armazenamento.RetencaoExecucoes + 2

func TestErrosSeguidosDesativam(t *testing.T) {
	amb, r, a := montar(t)
	var desativada string
	r.AoDesativar(func(_ context.Context, id string) { desativada = id })
	final := func(estado string, sim bool) {
		e, _ := r.Criar(amb.Ctx, Nova{Automacao: a, Origem: "gatilho", Simulacao: sim})
		e.Iniciar(amb.Ctx)
		e.Finalizar(amb.Ctx, Fim{Estado: estado, Erro: "falhou"})
	}
	seguidos := func() int {
		x, _ := armazenamento.ObterAutomacao(amb.Ctx, amb.Banco.L(), a.ID)
		return x.ErrosSeguidos
	}
	for i := 0; i < 4; i++ {
		final(dominio.ExecErro, false)
	}
	final(dominio.ExecErro, true)      // simulação não conta
	final(dominio.ExecAbortada, false) // abortada não conta
	if seguidos() != 4 {
		t.Fatalf("seguidos: %d", seguidos())
	}
	final(dominio.ExecOK, false)
	if seguidos() != 0 {
		t.Fatal("ok não zerou")
	}
	for i := 0; i < 5; i++ {
		final(dominio.ExecErro, false)
	}
	x, _ := armazenamento.ObterAutomacao(amb.Ctx, amb.Banco.L(), a.ID)
	if x.Ativa || x.DesativadaMotivo == nil || *x.DesativadaMotivo != "erros_seguidos" || desativada != a.ID {
		t.Fatalf("não desativou: %+v", x)
	}
	n := amb.EventosDoTipo(eventos.Notificacao)
	if len(n) != 1 || n[0].Dados.(dominio.Notificacao).Tipo != "desativada" {
		t.Fatalf("notificação: %+v", n)
	}
}

func TestAbortarPendentes(t *testing.T) {
	amb, r, a := montar(t)
	e1, _ := r.Criar(amb.Ctx, Nova{Automacao: a, Origem: "gatilho"})
	e2, _ := r.Criar(amb.Ctx, Nova{Automacao: a, Origem: "gatilho"})
	e2.Iniciar(amb.Ctx)
	e3, _ := r.Criar(amb.Ctx, Nova{Automacao: a, Origem: "gatilho"})
	e3.Iniciar(amb.Ctx)
	e3.Aguardar(amb.Ctx, amb.Relogio.Agora().Add(time.Hour), 1)
	n, _ := r.AbortarPendentes(amb.Ctx, "interrompida", dominio.ExecNaFila, dominio.ExecRodando)
	if n != 2 {
		t.Fatalf("abortadas: %d", n)
	}
	for _, id := range []string{e1.ID(), e2.ID()} {
		d, _ := armazenamento.ObterExecucao(amb.Ctx, amb.Banco.L(), id)
		if d.Estado != dominio.ExecAbortada || d.Motivo == nil || *d.Motivo != "interrompida" {
			t.Fatalf("não abortou: %+v", d.Execucao)
		}
	}
	if d, _ := armazenamento.ObterExecucao(amb.Ctx, amb.Banco.L(), e3.ID()); d.Estado != dominio.ExecAguardando {
		t.Fatal("aguardando não deveria ser abortada aqui")
	}
}
