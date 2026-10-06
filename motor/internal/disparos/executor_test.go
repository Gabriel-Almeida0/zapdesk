package disparos_test

import (
	"context"
	"fmt"
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

var ctx = context.Background()

type ambiente struct {
	t       *testing.T
	app     *aplicacao.App
	relogio *relogio.Controlavel
	d       *disparos.Servico
	falso   *falso.Controle
}

func novoAmbiente(t *testing.T) *ambiente {
	t.Helper()
	r := relogio.NovoCongelado(time.Date(2026, 9, 28, 10, 0, 0, 0, time.Local))
	aplicacao.SementeDisparos = 1
	app, err := aplicacao.Montar(ctx, aplicacao.Opcoes{Versao: "t", PastaDados: t.TempDir(), ModoWhatsApp: "falso",
		Token: "x", Log: zerolog.Nop(), Relogio: r})
	if err != nil {
		t.Fatal(err)
	}
	app.Iniciar()
	t.Cleanup(func() {
		c, cancelar := context.WithTimeout(ctx, 5*time.Second)
		defer cancelar()
		app.Encerrar(c)
	})
	return &ambiente{t: t, app: app, relogio: r, d: app.Servicos().Disparos, falso: app.Falso}
}

func (a *ambiente) conta(tel string) string {
	a.t.Helper()
	c, err := a.app.Servicos().Contas.Criar(ctx, nil)
	if err != nil {
		a.t.Fatal(err)
	}
	a.app.Servicos().Contas.AguardarEventos(ctx, c.ID)
	if err := a.falso.EscanearQR(c.ID, tel, "Loja"); err != nil {
		a.t.Fatal(err)
	}
	a.app.Servicos().Contas.AguardarEventos(ctx, c.ID)
	return c.ID
}

func (a *ambiente) leads(tels ...string) []string {
	a.t.Helper()
	var ls []leads.LeadEntrada
	for i, tel := range tels {
		nome := fmt.Sprintf("Lead %d", i+1)
		ls = append(ls, leads.LeadEntrada{Telefone: tel, Nome: &nome})
	}
	rel, err := a.app.Servicos().Leads.Importar(ctx, leads.CorpoImportacao{Leads: &ls})
	if err != nil {
		a.t.Fatal(err)
	}
	return rel.LeadIDs
}

func tels(prefixo, n int) []string {
	var l []string
	for i := 0; i < n; i++ {
		l = append(l, fmt.Sprintf("+55119%04d%04d", prefixo, i))
	}
	return l
}

func ip(n int) *int { return &n }

func (a *ambiente) disparo(conta string, leadIDs []string, ritmo dominio.Ritmo, max *int) dominio.Disparo {
	a.t.Helper()
	msg := "Oi {nome}!"
	d, err := a.d.Criar(ctx, disparos.NovoDisparo{ContaID: conta, Mensagem: &msg, Destinatarios: disparos.Destinatarios{LeadIDs: leadIDs},
		Ritmo: &ritmo, FalhasSeguidasMax: max, Iniciar: true})
	if err != nil {
		a.t.Fatal(err)
	}
	return d
}

func (a *ambiente) ocioso() {
	a.t.Helper()
	c, cancelar := context.WithTimeout(ctx, 10*time.Second)
	defer cancelar()
	if err := a.d.AguardarOcioso(c); err != nil {
		a.t.Fatal("executor não ficou ocioso")
	}
}

func (a *ambiente) avancar(d time.Duration) {
	a.relogio.Avancar(d)
	a.ocioso()
}

func (a *ambiente) obter(id string) dominio.Disparo {
	a.t.Helper()
	d, err := a.d.Obter(ctx, id)
	if err != nil {
		a.t.Fatal(err)
	}
	return d
}

func (a *ambiente) enviadasPara() map[string]int {
	m := map[string]int{}
	for _, e := range a.falso.Enviadas() {
		if e.Tipo == "texto" {
			m[e.Telefone]++
		}
	}
	return m
}

var ritmo1s = dominio.Ritmo{IntervaloMinS: 1, IntervaloMaxS: 1}

func TestFilaPorConta(t *testing.T) {
	a := novoAmbiente(t)
	c := a.conta("+5511900000001")
	d1 := a.disparo(c, a.leads(tels(1, 3)...), dominio.Ritmo{IntervaloMinS: 60, IntervaloMaxS: 60}, nil)
	d2 := a.disparo(c, a.leads(tels(2, 2)...), ritmo1s, nil)
	a.ocioso()
	x1, x2 := a.obter(d1.ID), a.obter(d2.ID)
	if x1.Estado != dominio.DisparoEnviando || x1.Contadores.Enviado != 1 || x2.Estado != dominio.DisparoAgendado || !x2.NaFila {
		t.Fatalf("fila: d1=%s(%d) d2=%s na_fila=%v", x1.Estado, x1.Contadores.Enviado, x2.Estado, x2.NaFila)
	}
	// pausar o ativo libera a fila
	if _, err := a.d.Pausar(ctx, d1.ID); err != nil {
		t.Fatal(err)
	}
	a.ocioso()
	x2 = a.obter(d2.ID)
	if x2.Estado != dominio.DisparoEnviando || x2.NaFila || x2.Contadores.Enviado != 1 {
		t.Fatalf("d2 deveria assumir: %s %v %d", x2.Estado, x2.NaFila, x2.Contadores.Enviado)
	}
	// retomar entra no fim da fila
	x1, err := a.d.Retomar(ctx, d1.ID)
	if err != nil {
		t.Fatal(err)
	}
	a.ocioso()
	if x1 = a.obter(d1.ID); x1.Estado != dominio.DisparoAgendado || !x1.NaFila {
		t.Fatalf("retomado deveria esperar na fila: %s %v", x1.Estado, x1.NaFila)
	}
	a.avancar(time.Second)
	if x2 = a.obter(d2.ID); x2.Estado != dominio.DisparoConcluido {
		t.Fatalf("d2 deveria concluir: %s %+v", x2.Estado, x2.Contadores)
	}
	for i := 0; i < 5; i++ {
		a.avancar(61 * time.Second)
	}
	if x1 = a.obter(d1.ID); x1.Estado != dominio.DisparoConcluido || x1.Contadores.Enviado != 3 {
		t.Fatalf("d1 deveria concluir com 3: %s %+v", x1.Estado, x1.Contadores)
	}
	for tel, n := range a.enviadasPara() {
		if n != 1 {
			t.Fatalf("%s recebeu %d mensagens", tel, n)
		}
	}
	if len(a.enviadasPara()) != 5 {
		t.Fatalf("esperava 5 envios: %v", a.enviadasPara())
	}
}

func TestFalhasSeguidas(t *testing.T) {
	a := novoAmbiente(t)
	c := a.conta("+5511900000001")
	f := tels(3, 5)
	sem := tels(4, 2)
	ok := tels(5, 1)
	a.falso.DefinirFalhas(f, "servidor recusou")
	a.falso.DefinirSemWhatsApp(sem)
	ordem := []string{f[0], f[1], sem[0], ok[0], f[2], f[3], sem[1], f[4], tels(6, 1)[0]}
	d := a.disparo(c, a.leads(ordem...), ritmo1s, ip(3))
	a.ocioso()
	for i := 0; i < 12; i++ {
		a.avancar(time.Second)
	}
	x := a.obter(d.ID)
	if x.Estado != dominio.DisparoPausado || x.MotivoPausa == nil || *x.MotivoPausa != dominio.PausaFalhasSeguidas {
		t.Fatalf("deveria pausar após 3 falhas seguidas: %s %v %+v", x.Estado, x.MotivoPausa, x.Contadores)
	}
	if x.Contadores.Falhou != 7 || x.Contadores.Enviado != 1 || x.Contadores.Pendente != 1 {
		t.Fatalf("contadores: %+v", x.Contadores)
	}
	var semWA int
	lista, _, _ := a.d.Destinatarios(ctx, d.ID, armazenamento.FiltroDestinatarios{Estado: "falhou"}, pagina.Params{Limite: 50})
	for _, dest := range lista {
		if *dest.MotivoFalha == disparos.MotivoSemWhatsApp {
			semWA++
		}
	}
	if semWA != 2 {
		t.Fatalf("\"Sem WhatsApp\" deveria aparecer 2 vezes: %d", semWA)
	}

	// 0 desliga a pausa automática
	d2 := a.disparo(c, a.leads(tels(7, 4)...), ritmo1s, ip(0))
	a.falso.DefinirFalhas(tels(7, 4), "x")
	a.ocioso()
	for i := 0; i < 6; i++ {
		a.avancar(time.Second)
	}
	if x := a.obter(d2.ID); x.Estado != dominio.DisparoConcluido || x.Contadores.Falhou != 4 {
		t.Fatalf("com 0 nunca pausa: %s %+v", x.Estado, x.Contadores)
	}
}

func TestContaDesconectadaEBanida(t *testing.T) {
	a := novoAmbiente(t)
	c := a.conta("+5511900000001")
	d := a.disparo(c, a.leads(tels(1, 5)...), ritmo1s, nil)
	a.ocioso()
	a.falso.InjetarEstado(c, falso.EventoLogout)
	a.app.Servicos().Contas.AguardarEventos(ctx, c)
	a.ocioso()
	x := a.obter(d.ID)
	if x.Estado != dominio.DisparoPausado || *x.MotivoPausa != dominio.PausaContaDesconectada {
		t.Fatalf("logout: %s %v", x.Estado, x.MotivoPausa)
	}
	if _, err := a.d.Retomar(ctx, d.ID); err == nil {
		t.Fatal("retomar com conta desconectada deveria falhar")
	}

	c2 := a.conta("+5511900000002")
	d2 := a.disparo(c2, a.leads(tels(2, 5)...), ritmo1s, nil)
	a.ocioso()
	a.falso.InjetarEstado(c2, falso.EventoBan)
	a.app.Servicos().Contas.AguardarEventos(ctx, c2)
	a.ocioso()
	x = a.obter(d2.ID)
	if x.Estado != dominio.DisparoPausado || *x.MotivoPausa != dominio.PausaContaBanida {
		t.Fatalf("ban: %s %v", x.Estado, x.MotivoPausa)
	}
	if _, err := a.d.Retomar(ctx, d2.ID); err == nil {
		t.Fatal("não pode retomar com conta banida")
	}
}

func TestQuedaDeRedeNaoMarcaFalhou(t *testing.T) {
	a := novoAmbiente(t)
	c := a.conta("+5511900000001")
	d := a.disparo(c, a.leads(tels(1, 3)...), ritmo1s, nil)
	a.ocioso()
	a.falso.InjetarEstado(c, falso.EventoQuedaRede)
	a.app.Servicos().Contas.AguardarEventos(ctx, c)
	a.avancar(time.Second)
	a.avancar(disparos.EsperaSemConexao)
	x := a.obter(d.ID)
	if x.Contadores.Falhou != 0 || x.Contadores.Enviado != 1 || x.Contadores.Pendente != 2 || x.Estado != dominio.DisparoEnviando {
		t.Fatalf("queda de rede: %s %+v", x.Estado, x.Contadores)
	}
	a.falso.InjetarEstado(c, falso.EventoReconectou)
	a.app.Servicos().Contas.AguardarEventos(ctx, c)
	a.ocioso()
	for i := 0; i < 3; i++ {
		a.avancar(time.Second)
	}
	if x = a.obter(d.ID); x.Estado != dominio.DisparoConcluido || x.Contadores.Enviado != 3 {
		t.Fatalf("após reconectar: %s %+v", x.Estado, x.Contadores)
	}
}

func TestRespondeuERecibos(t *testing.T) {
	a := novoAmbiente(t)
	c := a.conta("+5511900000001")
	ts := tels(1, 2)
	d := a.disparo(c, a.leads(ts...), ritmo1s, nil)
	a.ocioso()
	a.avancar(time.Second)
	lista, _, _ := a.d.Destinatarios(ctx, d.ID, armazenamento.FiltroDestinatarios{}, pagina.Params{Limite: 10})
	if lista[0].MensagemWaID == nil || lista[1].MensagemWaID == nil {
		t.Fatalf("wa_id: %+v", lista)
	}
	// lido antes de entregue preenche ambos
	a.falso.InjetarRecibo(c, *lista[0].MensagemWaID, "lido")
	a.falso.InjetarRecibo(c, *lista[0].MensagemWaID, "entregue")
	a.app.Servicos().Contas.AguardarEventos(ctx, c)
	lista, _, _ = a.d.Destinatarios(ctx, d.ID, armazenamento.FiltroDestinatarios{}, pagina.Params{Limite: 10})
	if lista[0].Estado != dominio.DestLido || lista[0].EntregueEm == nil || lista[0].LidoEm == nil {
		t.Fatalf("recibos: %+v", lista[0])
	}
	a.relogio.Avancar(time.Minute)
	a.falso.InjetarMensagem(c, falso.Recebida{De: ts[1], Texto: "quero saber mais"})
	a.app.Servicos().Contas.AguardarEventos(ctx, c)
	lista, _, _ = a.d.Destinatarios(ctx, d.ID, armazenamento.FiltroDestinatarios{}, pagina.Params{Limite: 10})
	if lista[1].Estado != dominio.DestRespondeu || lista[1].RespondeuEm == nil {
		t.Fatalf("respondeu: %+v", lista[1])
	}
	// recibo depois de respondeu só preenche datas
	a.falso.InjetarRecibo(c, *lista[1].MensagemWaID, "lido")
	a.app.Servicos().Contas.AguardarEventos(ctx, c)
	lista, _, _ = a.d.Destinatarios(ctx, d.ID, armazenamento.FiltroDestinatarios{}, pagina.Params{Limite: 10})
	if lista[1].Estado != dominio.DestRespondeu || lista[1].LidoEm == nil {
		t.Fatalf("respondeu é final: %+v", lista[1])
	}
	if x := a.obter(d.ID); x.Contadores.Respondeu != 1 || x.Contadores.Lido != 1 {
		t.Fatalf("contadores: %+v", x.Contadores)
	}
}

func TestBootPausaEReconcilia(t *testing.T) {
	a := novoAmbiente(t)
	c := a.conta("+5511900000001")
	d := a.disparo(c, a.leads(tels(1, 4)...), ritmo1s, nil)
	a.ocioso()
	banco := a.app.Banco
	// simula queda no meio: um destinatário ficou em "enviando" sem wa_id e o disparo "enviando"
	lista, _, _ := a.d.Destinatarios(ctx, d.ID, armazenamento.FiltroDestinatarios{Estado: "pendente"}, pagina.Params{Limite: 1})
	banco.E().Exec(`UPDATE destinatarios SET estado = 'enviando', enviando_em = 1 WHERE id = ?`, lista[0].ID)
	if err := a.d.Preparar(ctx, true); err != nil {
		t.Fatal(err)
	}
	x := a.obter(d.ID)
	if x.Estado != dominio.DisparoPausado || *x.MotivoPausa != dominio.PausaMotorReiniciado {
		t.Fatalf("boot: %s %v", x.Estado, x.MotivoPausa)
	}
	dest, _ := armazenamento.ObterDestinatario(ctx, banco.L(), lista[0].ID)
	if dest.Estado != dominio.DestFalhou || *dest.MotivoFalha != disparos.MotivoEstadoIncerto {
		t.Fatalf("reconciliação: %+v", dest)
	}
	// o que foi realmente enviado (achado no histórico) volta como enviado
	enviado, _, _ := a.d.Destinatarios(ctx, d.ID, armazenamento.FiltroDestinatarios{Estado: "enviado"}, pagina.Params{Limite: 1})
	banco.E().Exec(`UPDATE destinatarios SET estado = 'enviando', mensagem_wa_id = NULL WHERE id = ?`, enviado[0].ID)
	a.d.Preparar(ctx, false)
	dest, _ = armazenamento.ObterDestinatario(ctx, banco.L(), enviado[0].ID)
	if dest.Estado != dominio.DestEnviado || dest.MensagemWaID == nil {
		t.Fatalf("achado no histórico deveria voltar a enviado: %+v", dest)
	}
}
