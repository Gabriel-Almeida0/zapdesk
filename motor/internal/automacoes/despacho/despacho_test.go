package despacho

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/automacoes/acoes"
	"zapdesk/motor/internal/automacoes/execucoes"
	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/automacoes/seguranca"
	"zapdesk/motor/internal/automacoes/testeapoio"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/fatos"
)

type cfg struct {
	mu sync.Mutex
	c  dominio.ConfiguracaoAutomacoes
}

func (c *cfg) Configuracao() dominio.ConfiguracaoAutomacoes {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.c
}

// execTeste registra a ordem das execuções e pode segurar uma execução até ser liberada.
type execTeste struct {
	mu      sync.Mutex
	ordem   []string // "<automacao>:<texto>"
	segurar map[string]chan struct{}
	iniciou chan string
}

func (x *execTeste) Executar(ctx context.Context, e *execucoes.Exec, a dominio.Automacao, c *acoes.Contexto) error {
	e.Iniciar(ctx)
	txt := ""
	if c.UltimaMensagem != nil {
		txt = *c.UltimaMensagem
	}
	if c.Alvo.LeadID != "" && txt == "" {
		txt = c.Alvo.LeadID
	}
	x.mu.Lock()
	ch := x.segurar[txt]
	x.mu.Unlock()
	if x.iniciou != nil {
		x.iniciou <- txt
	}
	if ch != nil {
		select {
		case <-ch:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	x.mu.Lock()
	x.ordem = append(x.ordem, a.Nome+":"+txt)
	x.mu.Unlock()
	return e.Finalizar(ctx, execucoes.Fim{Estado: dominio.ExecOK})
}

func (x *execTeste) lista() []string {
	x.mu.Lock()
	defer x.mu.Unlock()
	return append([]string{}, x.ordem...)
}

type botTeste struct{ consome map[string]bool }

func (b *botTeste) Consumir(_ context.Context, f fatos.Fato) bool         { return b.consome[f.ConversaID] }
func (b *botTeste) ExpirarSessao(context.Context, dominio.Espera, string) {}

type ambiente struct {
	*testeapoio.Ambiente
	d     *Despachante
	x     *execTeste
	cfg   *cfg
	reg   *execucoes.Registro
	conta string
}

func montar(t *testing.T) *ambiente {
	amb := testeapoio.Novo(t)
	c := &cfg{c: dominio.ConfiguracaoPadrao()}
	reg := execucoes.Novo(amb.Banco, amb.Barramento, amb.Relogio)
	portao := seguranca.NovoPortao(amb.Banco, amb.Barramento, amb.Relogio, c)
	x := &execTeste{segurar: map[string]chan struct{}{}}
	d := Novo(Deps{Banco: amb.Banco, Relogio: amb.Relogio, Log: zerolog.Nop(), Registro: reg, Pausas: portao.Pausas(), Config: c,
		Executores: map[string]Executor{"fluxo": x}})
	ctx, cancelar := context.WithCancel(amb.Ctx)
	t.Cleanup(cancelar)
	d.Iniciar(ctx)
	return &ambiente{Ambiente: amb, d: d, x: x, cfg: c, reg: reg, conta: amb.Conta("Loja")}
}

func (a *ambiente) ocioso(t *testing.T) {
	t.Helper()
	ctx, cancelar := context.WithTimeout(a.Ctx, 5*time.Second)
	defer cancelar()
	if err := a.d.AguardarOcioso(ctx); err != nil {
		t.Fatal("despachante não ficou ocioso")
	}
}

var gMsg = []modelo.Gatilho{{Tipo: modelo.GatilhoMensagemRecebida}}

func TestFilaPorConversaEmOrdemEIndependente(t *testing.T) {
	a := montar(t)
	a.Automacao("fluxo", "A", gMsg, nil)
	cv1, ct1 := a.Conversa(a.conta, "+5511900000001")
	cv2, ct2 := a.Conversa(a.conta, "+5511900000002")
	// Segura a 1ª mensagem da conversa 1: a 2ª da mesma conversa espera; a da conversa 2 não.
	libera := make(chan struct{})
	a.x.segurar["c1-m1"] = libera
	a.x.iniciou = make(chan string, 10)
	a.d.Receber(fatos.Fato{Tipo: fatos.MensagemRecebida, ContaID: a.conta, ConversaID: cv1, ContatoID: ct1, Texto: "c1-m1"})
	a.d.Receber(fatos.Fato{Tipo: fatos.MensagemRecebida, ContaID: a.conta, ConversaID: cv1, ContatoID: ct1, Texto: "c1-m2"})
	a.d.Receber(fatos.Fato{Tipo: fatos.MensagemRecebida, ContaID: a.conta, ConversaID: cv2, ContatoID: ct2, Texto: "c2-m1"})
	vistos := map[string]bool{}
	for i := 0; i < 2; i++ {
		select {
		case s := <-a.x.iniciou:
			vistos[s] = true
		case <-time.After(5 * time.Second):
			t.Fatal("execuções não começaram")
		}
	}
	if !vistos["c1-m1"] || !vistos["c2-m1"] || vistos["c1-m2"] {
		t.Fatalf("filas: %v", vistos)
	}
	// A conversa 2 termina enquanto a 1 está presa.
	deadline := time.After(5 * time.Second)
	for len(a.x.lista()) < 1 {
		select {
		case <-deadline:
			t.Fatal("conversa 2 não terminou")
		case <-time.After(5 * time.Millisecond):
		}
	}
	close(libera)
	a.ocioso(t)
	l := a.x.lista()
	if len(l) != 3 || l[0] != "A:c2-m1" || l[1] != "A:c1-m1" || l[2] != "A:c1-m2" {
		t.Fatalf("ordem: %v", l)
	}
}

func TestPrioridadeUmaExecucaoPorLeadEPausaGeral(t *testing.T) {
	a := montar(t)
	g := []modelo.Gatilho{{Tipo: modelo.GatilhoLeadImportado}}
	a.Automacao("fluxo", "L", g, nil)
	a.d.Receber(fatos.Fato{Tipo: fatos.LeadImportado, LeadIDs: []string{"l1", "l2", "l3"}, OrigemLead: "csv"})
	a.ocioso(t)
	if l := a.x.lista(); len(l) != 3 {
		t.Fatalf("uma execução por lead: %v", l)
	}
	var n int
	a.Banco.L().QueryRow(`SELECT count(*) FROM execucoes WHERE lead_id IS NOT NULL`).Scan(&n)
	if n != 3 {
		t.Fatalf("execuções com lead: %d", n)
	}
	// Pausa geral: nada executa.
	a.cfg.mu.Lock()
	a.cfg.c.PausaGeral = true
	a.cfg.mu.Unlock()
	a.d.Receber(fatos.Fato{Tipo: fatos.LeadImportado, LeadIDs: []string{"l4"}, OrigemLead: "csv"})
	a.ocioso(t)
	if l := a.x.lista(); len(l) != 3 {
		t.Fatalf("pausa geral deveria bloquear: %v", l)
	}
}

func TestSessaoDeChatbotConsomeMensagem(t *testing.T) {
	a := montar(t)
	a.Automacao("fluxo", "A", gMsg, nil)
	cv1, ct1 := a.Conversa(a.conta, "+5511900000011")
	cv2, ct2 := a.Conversa(a.conta, "+5511900000012")
	a.d.DefinirChatbot(&botTeste{consome: map[string]bool{cv1: true}})
	a.d.Receber(fatos.Fato{Tipo: fatos.MensagemRecebida, ContaID: a.conta, ConversaID: cv1, ContatoID: ct1, Texto: "no bot"})
	a.d.Receber(fatos.Fato{Tipo: fatos.MensagemRecebida, ContaID: a.conta, ConversaID: cv2, ContatoID: ct2, Texto: "livre"})
	// Fato não-mensagem na conversa do bot continua disparando outras automações.
	a.Automacao("fluxo", "E", []modelo.Gatilho{{Tipo: modelo.GatilhoEtiqueta, Evento: "adicionada", EtiquetaID: "e1"}}, nil)
	a.d.Receber(fatos.Fato{Tipo: fatos.Etiqueta, ContaID: a.conta, ConversaID: cv1, ContatoID: ct1, EtiquetaID: "e1", Evento: "adicionada"})
	a.ocioso(t)
	l := a.x.lista()
	if len(l) != 2 || l[0] != "A:livre" && l[1] != "A:livre" {
		t.Fatalf("sessão deveria consumir a mensagem: %v", l)
	}
}

func TestConversaPausadaNaoDisparaERespostaManualPausa(t *testing.T) {
	a := montar(t)
	a.Automacao("fluxo", "A", gMsg, nil)
	cv, ct := a.Conversa(a.conta, "+5511900000021")
	a.d.Receber(fatos.Fato{Tipo: fatos.MensagemEnviada, ContaID: a.conta, ConversaID: cv, ContatoID: ct, Origem: fatos.OrigemManual, Em: a.Relogio.Agora()})
	a.d.Receber(fatos.Fato{Tipo: fatos.MensagemRecebida, ContaID: a.conta, ConversaID: cv, ContatoID: ct, Texto: "oi"})
	a.ocioso(t)
	if l := a.x.lista(); len(l) != 0 {
		t.Fatalf("conversa em atendimento humano não deveria disparar: %v", l)
	}
	p, ok, _ := armazenamento.ObterPausa(a.Ctx, a.Banco.L(), cv)
	if !ok || p.Motivo != dominio.PausaHumano {
		t.Fatalf("pausa humano: %+v %v", p, ok)
	}
	// Mensagem enviada por automação não pausa.
	cv2, ct2 := a.Conversa(a.conta, "+5511900000022")
	a.d.Receber(fatos.Fato{Tipo: fatos.MensagemEnviada, ContaID: a.conta, ConversaID: cv2, ContatoID: ct2, Origem: fatos.OrigemAutomacao, AutomacaoID: "x"})
	a.ocioso(t)
	if _, ok, _ := armazenamento.ObterPausa(a.Ctx, a.Banco.L(), cv2); ok {
		t.Fatal("mensagem automática não deveria pausar")
	}
}

func TestEncerramentoAbortaRodando(t *testing.T) {
	a := montar(t)
	a.Automacao("fluxo", "A", gMsg, nil)
	cv, ct := a.Conversa(a.conta, "+5511900000031")
	a.x.segurar["preso"] = make(chan struct{}) // nunca liberado
	a.x.iniciou = make(chan string, 10)
	a.d.Receber(fatos.Fato{Tipo: fatos.MensagemRecebida, ContaID: a.conta, ConversaID: cv, ContatoID: ct, Texto: "preso"})
	a.d.Receber(fatos.Fato{Tipo: fatos.MensagemRecebida, ContaID: a.conta, ConversaID: cv, ContatoID: ct, Texto: "na fila"})
	select {
	case <-a.x.iniciou:
	case <-time.After(5 * time.Second):
		t.Fatal("não começou")
	}
	for fim := time.Now().Add(5 * time.Second); ; {
		var n int
		a.Banco.L().QueryRow(`SELECT count(*) FROM execucoes`).Scan(&n)
		if n == 2 {
			break
		}
		if time.Now().After(fim) {
			t.Fatal("a segunda execução não entrou na fila")
		}
		time.Sleep(5 * time.Millisecond)
	}
	ctx, cancelar := context.WithTimeout(a.Ctx, 5*time.Second)
	defer cancelar()
	a.d.Encerrar(ctx)
	linhas, _ := a.Banco.L().Query(`SELECT estado, COALESCE(motivo, '') FROM execucoes`)
	defer linhas.Close()
	n := 0
	for linhas.Next() {
		var e, m string
		linhas.Scan(&e, &m)
		n++
		if e != dominio.ExecAbortada || m != MotivoAppFechado {
			t.Fatalf("execução %s (%s) deveria estar abortada por app fechado", e, m)
		}
	}
	if n != 2 {
		t.Fatalf("execuções: %d", n)
	}
	// Depois de encerrado, fatos são ignorados.
	a.d.Receber(fatos.Fato{Tipo: fatos.MensagemRecebida, ContaID: a.conta, ConversaID: cv, Texto: "depois"})
}
