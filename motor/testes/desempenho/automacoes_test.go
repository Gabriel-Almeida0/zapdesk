package desempenho

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"zapdesk/motor/internal/aplicacao"
	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/automacoes"
	"zapdesk/motor/internal/automacoes/compilador"
	"zapdesk/motor/internal/automacoes/projetos"
	"zapdesk/motor/internal/funil"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/pagina"
	"zapdesk/motor/internal/relogio"
	"zapdesk/motor/internal/whatsapp/falso"
)

// T130 — metas de desempenho da feature 002 (plan.md › Performance Goals).

func TestKanbanCom5000Cards(t *testing.T) {
	if testing.Short() || comRace {
		t.Skip("teste de volume: rode sem -race")
	}
	app := montar(t, t.TempDir())
	defer app.Encerrar(ctx)
	sv := app.Servicos()
	f, err := sv.Funil.Criar(ctx, "Vendas", []funil.NovaEtapa{{Nome: "Novo"}, {Nome: "Qualificando"}, {Nome: "Proposta"}, {Nome: "Fechado"}})
	if err != nil {
		t.Fatal(err)
	}
	agora := time.Now()
	err = app.Banco.Transacao(ctx, func(tx *sql.Tx) error {
		for i := 0; i < 5000; i++ {
			id := ids.NovoEm(agora)
			tel := fmt.Sprintf("+55119%08d", i)
			if _, err := tx.ExecContext(ctx, `INSERT INTO leads (id, telefone, nome, campos, origem, importado_em, atualizado_em) VALUES (?, ?, ?, '{}', 'csv', ?, ?)`,
				id, tel, fmt.Sprintf("Lead %d", i), armazenamento.Ms(agora), armazenamento.Ms(agora)); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO posicoes_funil (lead_id, funil_id, etapa_id, desde) VALUES (?, ?, ?, ?)`,
				id, f.ID, f.Etapas[i%4].ID, armazenamento.Ms(agora.Add(time.Duration(i)*time.Second))); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, filtro := range []armazenamento.FiltroCards{{}, {EtapaID: f.Etapas[2].ID}, {Busca: "Lead 49"}} {
		inicio := time.Now()
		cards, prox, err := sv.Funil.Cards(ctx, f.ID, filtro, pagina.Params{Limite: 50})
		d := time.Since(inicio)
		if err != nil || len(cards) == 0 {
			t.Fatalf("cards: %v %d", err, len(cards))
		}
		t.Logf("página de cards %+v: %v (próximo=%v)", filtro, d, prox != "")
		if d > 200*time.Millisecond {
			t.Fatalf("consulta paginada levou %v (meta < 200 ms)", d)
		}
	}
	inicio := time.Now()
	fl, err := sv.Funil.Obter(ctx, f.ID)
	if err != nil || fl.TotalCards != 5000 {
		t.Fatalf("funil: %v %d", err, fl.TotalCards)
	}
	t.Logf("funil com contagens: %v", time.Since(inicio))
}

func TestCompilacaoMenorQue1s(t *testing.T) {
	if comRace {
		t.Skip("medição de tempo: rode sem -race")
	}
	pasta := t.TempDir()
	p := projetos.Novo(pasta, relogio.Real{})
	if err := p.Criar("a1", "classificar_funil", "Classificar", nil); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 7; i++ {
		p.Escrever("a1", fmt.Sprintf("util%d.ts", i), fmt.Sprintf("export const valor%d = %d;\nexport function dobro%d(x: number) { return x * 2 + %d; }\n", i, i, i, i), false, nil)
	}
	inicio := time.Now()
	res := compilador.Compilar(ctx, p.Pasta("a1"), p.PastaCompiladas("a1"), nada{})
	d := time.Since(inicio)
	if !res.OK {
		t.Fatalf("compilação: %+v", res.Erros)
	}
	t.Logf("compilação de 10 arquivos: %v", d)
	if d > time.Second {
		t.Fatalf("compilação levou %v (meta < 1 s)", d)
	}
}

type nada struct{}

func (nada) Etiqueta(_ contexto, _ string) (string, bool) { return "", false }
func (nada) Funil(_ contexto, _ string) (string, bool)    { return "", false }
func (nada) Etapa(_ contexto, _, _ string) (string, bool) { return "", false }

func TestFluxoPontaAPontaMenorQue3s(t *testing.T) {
	app := montar(t, t.TempDir())
	defer app.Encerrar(ctx)
	app.Iniciar()
	sv := app.Servicos()
	c, _ := sv.Contas.Criar(ctx, nil)
	sv.Contas.AguardarEventos(ctx, c.ID)
	app.Falso.EscanearQR(c.ID, "+5511900000001", "Loja")
	sv.Contas.AguardarEventos(ctx, c.ID)
	e, _ := automacoes.LerEntrada([]byte(`{"tipo":"fluxo","nome":"Resposta","gatilhos":[{"tipo":"mensagem_recebida"}],
		"definicao":{"versao":1,"acoes":[{"tipo":"enviar_texto","texto":"Recebemos sua mensagem, {primeiro_nome}!"}]}}`))
	a, err := sv.Automacoes.Criar(ctx, e)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Automacoes.Ativar(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	inicio := time.Now()
	app.Falso.InjetarMensagem(c.ID, falsoRecebida("+5511977776666", "Olá"))
	sv.Contas.AguardarEventos(ctx, c.ID)
	if err := app.AguardarAutomacoes(ctx); err != nil {
		t.Fatal(err)
	}
	for len(app.Falso.Enviadas()) == 0 {
		if time.Since(inicio) > 3*time.Second {
			t.Fatal("fluxo não respondeu em 3 s")
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Logf("fluxo ponta a ponta: %v", time.Since(inicio))
}

func TestIAProcessoFrioMenorQue10s(t *testing.T) {
	_, arq, _, _ := runtime.Caller(0)
	script, _ := filepath.Abs(filepath.Join(filepath.Dir(arq), "..", "..", "..", "automacao", "runner", "dist", "zapdesk-runner.mjs"))
	node, err := exec.LookPath("node")
	if _, e2 := os.Stat(script); err != nil || e2 != nil {
		if os.Getenv("ZAPDESK_CI") == "1" {
			t.Fatal("runner indisponível no CI")
		}
		t.Skip("aviso: runner indisponível; teste de IA pulado")
	}
	app, err := aplicacao.Montar(ctx, aplicacao.Opcoes{Versao: "t", PastaDados: t.TempDir(), ModoWhatsApp: "falso", Token: token,
		Log: zerolog.Nop(), Relogio: relogio.NovoControlavel(time.Now()), IA: "falsa", RunnerExec: node, RunnerScript: script})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Encerrar(ctx)
	app.Iniciar()
	sv := app.Servicos()
	c, _ := sv.Contas.Criar(ctx, nil)
	sv.Contas.AguardarEventos(ctx, c.ID)
	app.Falso.EscanearQR(c.ID, "+5511900000001", "Loja")
	sv.Contas.AguardarEventos(ctx, c.ID)
	a, err := sv.AutomacoesIA.CriarPeloModelo(ctx, "Responder", "responder_historico", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Automacoes.Ativar(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	inicio := time.Now()
	app.Falso.InjetarMensagem(c.ID, falsoRecebida("+5511977775555", "Quanto custa?"))
	sv.Contas.AguardarEventos(ctx, c.ID)
	for len(app.Falso.Enviadas()) == 0 {
		if time.Since(inicio) > 10*time.Second {
			t.Fatal("IA com processo frio não respondeu em 10 s")
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Logf("IA (processo frio, IA simulada): %v", time.Since(inicio))
}

type contexto = context.Context

func falsoRecebida(de, texto string) falso.Recebida {
	return falso.Recebida{De: de, Nome: "Contato", Texto: texto}
}
