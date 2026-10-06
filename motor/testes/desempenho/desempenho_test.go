// Testes de desempenho com o motor falso (metas de plan.md / spec SC-001, SC-005, SC-008).
package desempenho

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"zapdesk/motor/internal/aplicacao"
	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/disparos"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/leads"
	"zapdesk/motor/internal/pagina"
	"zapdesk/motor/internal/relogio"
	"zapdesk/motor/internal/whatsapp/falso"
)

const token = "token-de-teste-com-mais-de-32-caracteres!!"

var ctx = context.Background()

func montar(t *testing.T, pasta string) *aplicacao.App {
	t.Helper()
	app, err := aplicacao.Montar(ctx, aplicacao.Opcoes{Versao: "t", PastaDados: pasta, ModoWhatsApp: "falso", Token: token,
		Log: zerolog.Nop(), Relogio: relogio.NovoCongelado(time.Date(2026, 9, 28, 10, 0, 0, 0, time.Local))})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func TestSubidaAtePronto(t *testing.T) {
	pasta := t.TempDir()
	app := montar(t, pasta)
	app.Encerrar(ctx)
	inicio := time.Now()
	app = montar(t, pasta)
	if err := app.Servidor.Escutar(0); err != nil {
		t.Fatal(err)
	}
	go app.Servidor.Servir()
	d := time.Since(inicio)
	defer app.Encerrar(ctx)
	t.Logf("montagem + migração + escuta: %v", d)
	if d > 2*time.Second {
		t.Fatalf("subida levou %v (meta < 2 s)", d)
	}
}

func TestAbrirConversaCom50MilMensagens(t *testing.T) {
	if testing.Short() || comRace {
		t.Skip("teste de volume: rode sem -race")
	}
	app := montar(t, t.TempDir())
	defer app.Encerrar(ctx)
	app.Iniciar()
	sv := app.Servicos()
	c, _ := sv.Contas.Criar(ctx, nil)
	sv.Contas.AguardarEventos(ctx, c.ID)
	app.Falso.EscanearQR(c.ID, "+5511900000001", "Loja")
	sv.Contas.AguardarEventos(ctx, c.ID)
	msgs := make([]falso.MensagemHistorico, 50000)
	for i := range msgs {
		msgs[i] = falso.MensagemHistorico{Texto: fmt.Sprintf("mensagem de histórico número %d com algum texto", i), DeMim: i%2 == 0}
	}
	inicio := time.Now()
	if err := app.Falso.InjetarHistorico(c.ID, []falso.ConversaHistorico{{JID: "+5511988887777", Mensagens: msgs}}); err != nil {
		t.Fatal(err)
	}
	sv.Contas.AguardarEventos(ctx, c.ID)
	t.Logf("history sync de 50.000 mensagens: %v", time.Since(inicio))

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	porta := ln.Addr().(*net.TCPAddr).Port
	app.Servidor.DefinirPorta(porta)
	srv := &http.Server{Handler: app.Handler()}
	go srv.Serve(ln)
	defer srv.Close()
	get := func(caminho string) time.Duration {
		req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d%s", porta, caminho), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		inicio := time.Now()
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("GET %s: %v %v", caminho, err, resp)
		}
		resp.Body.Close()
		return time.Since(inicio)
	}
	convs, _, _ := armazenamento.ListarConversas(ctx, app.Banco.L(), c.ID, armazenamento.FiltroConversas{}, pagina.Params{Limite: 1})
	d1 := get("/v1/contas/" + c.ID + "/conversas")
	d2 := get("/v1/conversas/" + convs[0].ID + "/mensagens?limite=50")
	d3 := get("/v1/contas/" + c.ID + "/mensagens/busca?q=numero%2049999")
	t.Logf("listar conversas %v; abrir conversa (50 mais recentes de 50.000) %v; busca FTS %v", d1, d2, d3)
	if d2 > 500*time.Millisecond || d1 > 500*time.Millisecond {
		t.Fatalf("abrir conversa acima de 500 ms: %v", d2)
	}
}

func TestDisparoCom50MilDestinatarios(t *testing.T) {
	if testing.Short() || comRace {
		t.Skip("teste de volume: rode sem -race")
	}
	app := montar(t, t.TempDir())
	defer app.Encerrar(ctx)
	sv := app.Servicos()
	c, _ := sv.Contas.Criar(ctx, nil)
	var b strings.Builder
	for i := 0; i < 50000; i++ {
		fmt.Fprintf(&b, "+55119%08d\n", 70000000+i)
	}
	texto := b.String()
	inicio := time.Now()
	rel, err := sv.Leads.Importar(ctx, leads.CorpoImportacao{TextoColado: &texto})
	if err != nil || rel.TotalNovos != 50000 {
		t.Fatalf("importar: %v %d", err, rel.TotalNovos)
	}
	t.Logf("importar 50.000 números: %v", time.Since(inicio))
	msg := "Oi!"
	inicio = time.Now()
	d, err := sv.Disparos.Criar(ctx, disparos.NovoDisparo{ContaID: c.ID, Mensagem: &msg,
		Destinatarios: disparos.Destinatarios{LeadIDs: rel.LeadIDs}, Ritmo: &dominio.Ritmo{IntervaloMinS: 30, IntervaloMaxS: 60}})
	if err != nil {
		t.Fatal(err)
	}
	criar := time.Since(inicio)
	inicio = time.Now()
	d, _ = sv.Disparos.Obter(ctx, d.ID)
	obter := time.Since(inicio)
	inicio = time.Now()
	lista, _, _ := sv.Disparos.Listar(ctx, "", "", pagina.Params{Limite: 50})
	listar := time.Since(inicio)
	inicio = time.Now()
	dests, _, _ := sv.Disparos.Destinatarios(ctx, d.ID, armazenamento.FiltroDestinatarios{}, pagina.Params{Limite: 50, Cursor: &pagina.Chave{N: 40000}})
	pag := time.Since(inicio)
	t.Logf("criar disparo com 50.000: %v; obter: %v; listar: %v; página de destinatários: %v", criar, obter, listar, pag)
	if d.Contadores.Total != 50000 || len(lista) != 1 || len(dests) != 50 || dests[0].Ordem != 40001 {
		t.Fatalf("resultado: %+v %d %d", d.Contadores, len(lista), len(dests))
	}
	if criar > 15*time.Second || obter > time.Second || pag > 500*time.Millisecond {
		t.Fatalf("lento demais: criar %v, obter %v, página %v", criar, obter, pag)
	}
}
