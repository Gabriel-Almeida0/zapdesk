package fluxo

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/automacoes/acoes"
	"zapdesk/motor/internal/automacoes/execucoes"
	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/automacoes/seguranca"
	"zapdesk/motor/internal/automacoes/testeapoio"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/funil"
	"zapdesk/motor/internal/leads"
)

type cfg struct {
	c dominio.ConfiguracaoAutomacoes
}

func (c *cfg) Configuracao() dominio.ConfiguracaoAutomacoes { return c.c }

type esperasTeste struct{ l []dominio.Espera }

func (e *esperasTeste) Gravar(_ context.Context, x dominio.Espera) error {
	e.l = append(e.l, x)
	return nil
}

type iaTeste struct{ ret string }

func (i iaTeste) ExecutarPorAcao(context.Context, string, map[string]any, *acoes.Contexto) (json.RawMessage, error) {
	return json.RawMessage(i.ret), nil
}

type amb struct {
	*testeapoio.Ambiente
	x     *Executor
	reg   *execucoes.Registro
	esp   *esperasTeste
	cfg   *cfg
	funil *funil.Servico
	conta string
	conv  string
	ct    string
}

func montar(t *testing.T) *amb {
	a := testeapoio.Novo(t)
	c := &cfg{c: dominio.ConfiguracaoPadrao()}
	reg := execucoes.Novo(a.Banco, a.Barramento, a.Relogio)
	portao := seguranca.NovoPortao(a.Banco, a.Barramento, a.Relogio, c)
	ls := leads.NovoServico(a.Banco, a.Barramento, a.Relogio)
	fs := funil.Novo(a.Banco, a.Barramento, a.Relogio, ls)
	ac := acoes.Novo(acoes.Deps{Banco: a.Banco, Barramento: a.Barramento, Relogio: a.Relogio, Leads: ls, Funil: fs, Portao: portao})
	ac.DefinirIA(iaTeste{ret: `{"nota":10}`})
	esp := &esperasTeste{}
	conta := a.Conta("Loja")
	conv, ct := a.Conversa(conta, "+5511955550000")
	return &amb{Ambiente: a, x: Novo(ac, esp, a.Relogio), reg: reg, esp: esp, cfg: c, funil: fs, conta: conta, conv: conv, ct: ct}
}

func (a *amb) automacao(t *testing.T, def modelo.DefinicaoFluxo) dominio.Automacao {
	modelo.NormalizarFluxo(&def)
	id := a.Automacao("fluxo", "Fluxo", []modelo.Gatilho{{Tipo: "manual"}}, def)
	au, err := armazenamento.ObterAutomacao(a.Ctx, a.Banco.L(), id)
	if err != nil {
		t.Fatal(err)
	}
	return au
}

func (a *amb) rodar(t *testing.T, au dominio.Automacao, sim bool, texto string) (*execucoes.Exec, *acoes.Contexto) {
	e, err := a.reg.Criar(a.Ctx, execucoes.Nova{Automacao: au, Origem: "gatilho", Simulacao: sim,
		Alvo: execucoes.Alvo{ContaID: a.conta, ConversaID: a.conv, ContatoID: a.ct}})
	if err != nil {
		t.Fatal(err)
	}
	c := &acoes.Contexto{Exec: e, Automacao: au, Alvo: acoes.Alvo{ContaID: a.conta, ConversaID: a.conv, ContatoID: a.ct},
		Simulacao: sim, Cadeia: []string{au.ID}, Variaveis: map[string]string{}, UltimaMensagem: &texto}
	if err := a.x.Executar(a.Ctx, e, au, c); err != nil {
		t.Fatal(err)
	}
	return e, c
}

func (a *amb) detalhe(t *testing.T, e *execucoes.Exec) dominio.ExecucaoDetalhe {
	d, err := armazenamento.ObterExecucao(a.Ctx, a.Banco.L(), e.ID())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func s(v string) *string { return &v }

func TestCondicoesNaoAtendidas(t *testing.T) {
	a := montar(t)
	au := a.automacao(t, modelo.DefinicaoFluxo{
		Condicoes: &modelo.Condicoes{Modo: "todas", Regras: []modelo.Regra{{Tipo: "texto", Operador: "contem", Valor: s("boleto")}}},
		Acoes:     []modelo.Acao{{Tipo: "notificar", Titulo: "t", Texto: "x"}}})
	e, _ := a.rodar(t, au, false, "quero o preço")
	d := a.detalhe(t, e)
	if d.Estado != dominio.ExecOK || d.Motivo == nil || *d.Motivo != MotivoCondicoes || len(d.Acoes) != 0 {
		t.Fatalf("condições: %+v", d.Execucao)
	}
	e, _ = a.rodar(t, au, false, "manda o BOLETO")
	if d := a.detalhe(t, e); d.Estado != dominio.ExecOK || d.Motivo != nil || len(d.Acoes) != 1 {
		t.Fatalf("condições atendidas: %+v", d.Execucao)
	}
}

func TestAguardarRetomaNoPassoSeguinteEFalhasNaoFatais(t *testing.T) {
	a := montar(t)
	f, _ := a.funil.Criar(a.Ctx, "Vendas", []funil.NovaEtapa{{Nome: "Novo"}, {Nome: "Quente"}})
	au := a.automacao(t, modelo.DefinicaoFluxo{Acoes: []modelo.Acao{
		{Tipo: "adicionar_etiqueta", EtiquetaID: "nao-existe"},        // falha não fatal
		{Tipo: "enviar_texto", Texto: "Oi {primeiro_nome}"},           // bloqueada (pausa geral)
		{Tipo: "aguardar", DuracaoS: 7200},                            // cede
		{Tipo: "mover_etapa", FunilID: f.ID, EtapaID: f.Etapas[1].ID}, // após retomar
		{Tipo: "executar_ia", AutomacaoID: "ia1", SalvarEm: s("resposta")},
		{Tipo: "notificar", Titulo: "Lead {primeiro_nome}", Texto: "Resposta: {resposta}"},
	}})
	a.cfg.c.PausaGeral = true
	e, c := a.rodar(t, au, false, "oi")
	d := a.detalhe(t, e)
	if d.Estado != dominio.ExecAguardando || d.PassoAtual == nil || *d.PassoAtual != 3 || d.RetomarEm == nil ||
		!d.RetomarEm.Equal(a.Relogio.Agora().Add(2*time.Hour)) {
		t.Fatalf("aguardando: %+v passo=%v", d.Execucao, d.PassoAtual)
	}
	if len(d.Acoes) != 3 || d.Acoes[0].Resultado != dominio.AcaoFalhou || d.Acoes[1].Resultado != dominio.AcaoBloqueada ||
		!strings.Contains(*d.Acoes[1].Detalhe, "pausadas") || d.Acoes[2].Tipo != "aguardar" {
		t.Fatalf("ações antes da espera: %+v", d.Acoes)
	}
	if len(a.esp.l) != 1 || a.esp.l[0].Tipo != dominio.EsperaAguardar || *a.esp.l[0].ExecucaoID != e.ID() || *a.esp.l[0].ConversaID != a.conv {
		t.Fatalf("espera: %+v", a.esp.l)
	}
	// Retomada (agendador): recarrega a execução e continua no passo 3.
	a.cfg.c.PausaGeral = false
	a.Relogio.Avancar(2 * time.Hour)
	e2, _ := a.reg.Carregar(a.Ctx, e.ID())
	c.Exec = e2
	if err := a.x.Retomar(a.Ctx, e2, au, c); err != nil {
		t.Fatal(err)
	}
	d = a.detalhe(t, e2)
	if d.Estado != dominio.ExecOK || len(d.Acoes) != 6 || d.Acoes[3].Resultado != dominio.AcaoOK || d.Acoes[4].Resultado != dominio.AcaoOK {
		t.Fatalf("depois de retomar: %+v %+v", d.Execucao, d.Acoes)
	}
	if d.Variaveis["resposta"] != `{"nota":10}` {
		t.Fatalf("salvar_em: %+v", d.Variaveis)
	}
	if not := amb2notificacao(a); not == nil || not.Corpo != `Resposta: {"nota":10}` || not.Titulo != "Lead Contato" {
		t.Fatalf("notificação com variáveis: %+v", not)
	}
	card, ok, _ := a.funil.Posicao(a.Ctx, f.ID, *d.LeadID)
	if !ok || card.EtapaID != f.Etapas[1].ID {
		t.Fatal("mover_etapa após retomar")
	}
}

func amb2notificacao(a *amb) *dominio.Notificacao {
	var ult *dominio.Notificacao
	for _, ev := range a.EventosDoTipo(eventos.Notificacao) {
		n := ev.Dados.(dominio.Notificacao)
		ult = &n
	}
	return ult
}

func TestSimulacaoNaoEscreveNemEspera(t *testing.T) {
	a := montar(t)
	f, _ := a.funil.Criar(a.Ctx, "Vendas", []funil.NovaEtapa{{Nome: "Novo"}})
	au := a.automacao(t, modelo.DefinicaoFluxo{Acoes: []modelo.Acao{
		{Tipo: "aguardar", DuracaoS: 7200},
		{Tipo: "mover_etapa", FunilID: f.ID, EtapaID: f.Etapas[0].ID},
		{Tipo: "atualizar_campo_lead", Campo: "interesse", Valor: s("{ultima_mensagem}")},
		{Tipo: "pausar_automacoes", DuracaoMin: nil},
		{Tipo: "notificar", Titulo: "t", Texto: "x"},
	}})
	e, _ := a.rodar(t, au, true, "quero")
	d := a.detalhe(t, e)
	if d.Estado != dominio.ExecSimulacao || len(d.Acoes) != 5 || !strings.Contains(*d.Acoes[0].Detalhe, "Aguardaria 2 h") {
		t.Fatalf("simulação: %+v %+v", d.Execucao, d.Acoes)
	}
	for _, ac := range d.Acoes {
		if ac.Resultado != dominio.AcaoSimulada {
			t.Fatalf("ação não simulada: %+v", ac)
		}
	}
	if len(a.esp.l) != 0 {
		t.Fatal("simulação gravou espera")
	}
	var n int
	a.Banco.L().QueryRow(`SELECT count(*) FROM posicoes_funil`).Scan(&n)
	a.Banco.L().QueryRow(`SELECT count(*) + (SELECT count(*) FROM pausas_conversa) + (SELECT count(*) FROM leads) FROM posicoes_funil`).Scan(&n)
	if n != 0 || len(a.EventosDoTipo(eventos.Notificacao)) != 0 {
		t.Fatalf("simulação escreveu algo (%d)", n)
	}
}

func TestDuracao(t *testing.T) {
	for d, s := range map[time.Duration]string{2 * time.Hour: "2 h", 90 * time.Minute: "1 h 30 min", 5 * time.Minute: "5 min", 45 * time.Second: "45 s"} {
		if Duracao(d) != s {
			t.Errorf("%v → %q", d, Duracao(d))
		}
	}
}
