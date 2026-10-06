// Pacote testeapoio monta um banco temporário migrado e fixtures (conta, conversa, mensagens,
// automações) para os testes unitários do motor de automações. Só é importado por testes.
package testeapoio

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/relogio"
)

// Inicio padrão dos testes: segunda-feira 28/09/2026 10:00 no fuso local.
var Inicio = time.Date(2026, 9, 28, 10, 0, 0, 0, time.Local)

// Ambiente de teste.
type Ambiente struct {
	T          testing.TB
	Ctx        context.Context
	Banco      *armazenamento.Banco
	Relogio    *relogio.Controlavel
	Barramento *eventos.Barramento
	eventos    *eventos.Assinatura
	mu         sync.Mutex
	recebidos  []eventos.Evento
}

// Novo cria banco migrado, relógio congelado em Inicio e barramento com assinatura.
func Novo(t testing.TB) *Ambiente {
	t.Helper()
	ctx := context.Background()
	b, err := armazenamento.Abrir(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Migrar(ctx, armazenamento.MigracoesPadrao()); err != nil {
		t.Fatal(err)
	}
	a := &Ambiente{T: t, Ctx: ctx, Banco: b, Relogio: relogio.NovoCongelado(Inicio)}
	a.Barramento = eventos.Novo(a.Relogio.Agora)
	a.eventos = a.Barramento.Assinar(10000)
	t.Cleanup(func() { b.Fechar() })
	return a
}

// Exec executa SQL e falha o teste em erro.
func (a *Ambiente) Exec(q string, args ...any) {
	a.T.Helper()
	if _, err := a.Banco.E().ExecContext(a.Ctx, q, args...); err != nil {
		a.T.Fatalf("%s: %v", q, err)
	}
}

// Conta cria uma conta conectada.
func (a *Ambiente) Conta(nome string) string {
	id := ids.NovoEm(a.Relogio.Agora())
	a.Exec(`INSERT INTO contas (id, nome, estado, criada_em, atualizada_em) VALUES (?, ?, 'conectada', 0, 0)`, id, nome)
	return id
}

// Conversa cria contato + conversa individual para o telefone (E.164) e devolve (conversa, contato).
func (a *Ambiente) Conversa(contaID, telefone string) (string, string) {
	a.T.Helper()
	jid := telefone[1:] + "@s.whatsapp.net"
	agora := a.Relogio.Agora()
	ct, _, err := armazenamento.GarantirContato(a.Ctx, a.Banco.E(), contaID, armazenamento.DadosContato{JID: jid, Telefone: telefone, NomePush: "Contato " + telefone[len(telefone)-4:]}, agora)
	if err != nil {
		a.T.Fatal(err)
	}
	cv, _, err := armazenamento.GarantirConversa(a.Ctx, a.Banco.E(), contaID, jid, "individual", "", ct, agora)
	if err != nil {
		a.T.Fatal(err)
	}
	return cv, ct
}

// Grupo cria uma conversa de grupo.
func (a *Ambiente) Grupo(contaID, nome string) string {
	cv, _, err := armazenamento.GarantirConversa(a.Ctx, a.Banco.E(), contaID, fmt.Sprintf("%d@g.us", time.Now().UnixNano()), "grupo", nome, "", a.Relogio.Agora())
	if err != nil {
		a.T.Fatal(err)
	}
	return cv
}

// Mensagem grava uma mensagem na conversa (de_mim, automação e primeiro contato opcionais).
func (a *Ambiente) Mensagem(contaID, conversaID string, deMim bool, texto, automacaoID string, primeiro bool, em time.Time) string {
	a.T.Helper()
	id := ids.NovoEm(em)
	estado := dominio.MsgRecebida
	if deMim {
		estado = dominio.MsgEnviada
	}
	if _, err := armazenamento.InserirMensagem(a.Ctx, a.Banco.E(), armazenamento.NovaMensagem{ID: id, ContaID: contaID, ConversaID: conversaID,
		WaID: "w" + id, RemetenteJID: "x", DeMim: deMim, Tipo: "texto", Texto: texto, Estado: estado, AutomacaoID: automacaoID,
		PrimeiroContato: primeiro, EnviadaEm: em}); err != nil {
		a.T.Fatal(err)
	}
	return id
}

// Automacao grava uma automação mínima (ativa) e devolve o id.
func (a *Ambiente) Automacao(tipo, nome string, gatilhos any, definicao any) string {
	a.T.Helper()
	agora := a.Relogio.Agora()
	id := ids.NovoEm(agora)
	g, _ := json.Marshal(gatilhos)
	var d json.RawMessage
	if definicao != nil {
		d, _ = json.Marshal(definicao)
	}
	if err := armazenamento.InserirAutomacao(a.Ctx, a.Banco.E(), dominio.Automacao{ID: id, Tipo: tipo, Nome: nome, Ativa: true,
		Prioridade: 100, Gatilhos: g, Definicao: d, Versao: 1, CriadaEm: agora, AtualizadaEm: agora}); err != nil {
		a.T.Fatal(err)
	}
	return id
}

// Eventos devolve (drenando) os eventos publicados até agora, acumulados.
func (a *Ambiente) Eventos() []eventos.Evento {
	a.mu.Lock()
	defer a.mu.Unlock()
	for {
		select {
		case ev := <-a.eventos.C:
			a.recebidos = append(a.recebidos, ev)
		default:
			return append([]eventos.Evento{}, a.recebidos...)
		}
	}
}

// EventosDoTipo filtra Eventos().
func (a *Ambiente) EventosDoTipo(tipo string) []eventos.Evento {
	var l []eventos.Evento
	for _, e := range a.Eventos() {
		if e.Tipo == tipo {
			l = append(l, e)
		}
	}
	return l
}
