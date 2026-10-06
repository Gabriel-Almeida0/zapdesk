package funil

import (
	"context"
	"strings"
	"sync"
	"testing"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/automacoes/testeapoio"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/fatos"
	"zapdesk/motor/internal/leads"
	"zapdesk/motor/internal/pagina"
)

type coletor struct {
	mu sync.Mutex
	l  []fatos.Fato
}

func (c *coletor) Receber(f fatos.Fato) {
	c.mu.Lock()
	c.l = append(c.l, f)
	c.mu.Unlock()
}

func montar(t *testing.T) (*testeapoio.Ambiente, *Servico, *coletor) {
	amb := testeapoio.Novo(t)
	ls := leads.NovoServico(amb.Banco, amb.Barramento, amb.Relogio)
	s := Novo(amb.Banco, amb.Barramento, amb.Relogio, ls)
	c := &coletor{}
	s.DefinirReceptorFatos(c)
	return amb, s, c
}

func cor(s string) *string { return &s }

var app = Origem{Tipo: dominio.OrigemApp}

func TestNomesEtapasECores(t *testing.T) {
	amb, s, _ := montar(t)
	ctx := amb.Ctx
	f, err := s.Criar(ctx, "  Vendas ", []NovaEtapa{{Nome: "Novo"}, {Nome: "Qualificando", Cor: cor("#00aa00")}, {Nome: "Proposta"}, {Nome: "Fechado"}})
	if err != nil {
		t.Fatal(err)
	}
	if f.Nome != "Vendas" || len(f.Etapas) != 4 || f.Etapas[0].Cor != CorPadrao || f.Etapas[1].Cor != "#00AA00" || f.Etapas[3].Ordem != 3 {
		t.Fatalf("funil: %+v", f)
	}
	if len(amb.EventosDoTipo(eventos.FunilAlterado)) != 1 {
		t.Fatal("funil.alterado")
	}
	if _, err := s.Criar(ctx, "VENDAS", nil); !erros.Eh(err, erros.Conflito) {
		t.Fatalf("nome duplicado sem diferenciar maiúsculas: %v", err)
	}
	for _, n := range []string{"", strings.Repeat("x", 61)} {
		if _, err := s.Criar(ctx, n, nil); !erros.Eh(err, erros.Validacao) {
			t.Fatalf("nome %q: %v", n, err)
		}
	}
	if _, err := s.Criar(ctx, "Outro", []NovaEtapa{{Nome: "A"}, {Nome: "a"}}); !erros.Eh(err, erros.Validacao) {
		t.Fatalf("etapas repetidas: %v", err)
	}
	if _, err := s.CriarEtapa(ctx, f.ID, "novo", nil, nil); !erros.Eh(err, erros.Conflito) {
		t.Fatalf("etapa repetida: %v", err)
	}
	if _, err := s.CriarEtapa(ctx, f.ID, strings.Repeat("e", 41), nil, nil); !erros.Eh(err, erros.Validacao) {
		t.Fatalf("etapa longa: %v", err)
	}
	if _, err := s.CriarEtapa(ctx, f.ID, "X", cor("azul"), nil); !erros.Eh(err, erros.Validacao) {
		t.Fatalf("cor inválida: %v", err)
	}
	pos := 1
	e, err := s.CriarEtapa(ctx, f.ID, "Contato feito", nil, &pos)
	if err != nil || e.Ordem != 1 {
		t.Fatalf("etapa na posição: %+v %v", e, err)
	}
	f, _ = s.Obter(ctx, f.ID)
	if f.Etapas[2].Nome != "Qualificando" || f.Etapas[2].Ordem != 2 {
		t.Fatalf("ordem após inserir: %+v", f.Etapas)
	}
	for i := len(f.Etapas); i < MaxEtapas; i++ {
		if _, err := s.CriarEtapa(ctx, f.ID, "E"+itoa(i), nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.CriarEtapa(ctx, f.ID, "Demais", nil, nil); !erros.Eh(err, erros.Validacao) {
		t.Fatalf("máximo de 30 etapas: %v", err)
	}
	// Reordenação exige todas as etapas.
	f, _ = s.Obter(ctx, f.ID)
	idsE := make([]string, len(f.Etapas))
	for i, e := range f.Etapas {
		idsE[len(f.Etapas)-1-i] = e.ID
	}
	if _, err := s.ReordenarEtapas(ctx, f.ID, idsE[1:]); !erros.Eh(err, erros.Validacao) {
		t.Fatalf("lista incompleta: %v", err)
	}
	f, err = s.ReordenarEtapas(ctx, f.ID, idsE)
	if err != nil || f.Etapas[0].ID != idsE[0] {
		t.Fatalf("reordenar: %v", err)
	}
}

func TestMoverEHistorico(t *testing.T) {
	amb, s, col := montar(t)
	ctx := amb.Ctx
	f, _ := s.Criar(ctx, "Vendas", []NovaEtapa{{Nome: "Novo"}, {Nome: "Quente"}})
	novo, quente := f.Etapas[0].ID, f.Etapas[1].ID
	conta := amb.Conta("Loja")
	conv, contato := amb.Conversa(conta, "+5511911110000")

	// Contato sem lead: cria o lead (origem contatos) e entra no funil (201).
	card, entrou, err := s.Mover(ctx, f.ID, AlvoCard{ContatoID: contato}, novo, app)
	if err != nil || !entrou || card.EtapaID != novo || card.Lead.Telefone != "+5511911110000" || card.ConversaID == nil || *card.ConversaID != conv {
		t.Fatalf("entrar: %+v %v %v", card, entrou, err)
	}
	l, _ := armazenamento.ObterLead(ctx, amb.Banco.L(), card.LeadID)
	if l.Origem != dominio.OrigemContatos {
		t.Fatalf("origem do lead: %s", l.Origem)
	}
	// Mesma etapa: sem efeito e sem histórico.
	if _, entrou, err := s.Mover(ctx, f.ID, AlvoCard{LeadID: card.LeadID}, novo, app); err != nil || entrou {
		t.Fatal("mover para a mesma etapa")
	}
	// Mudar de etapa: nunca duplica.
	if _, entrou, err := s.Mover(ctx, f.ID, AlvoCard{Telefone: "11 91111-0000"}, quente, Origem{Tipo: dominio.OrigemAutomacao, AutomacaoID: "a1", ExecucaoID: "x1"}); err != nil || entrou {
		t.Fatalf("mudar: %v", err)
	}
	cards, _, _ := s.Cards(ctx, f.ID, armazenamento.FiltroCards{}, pagina.Params{Limite: 50})
	if len(cards) != 1 || cards[0].EtapaID != quente {
		t.Fatalf("cards: %+v", cards)
	}
	h, _, _ := s.Historico(ctx, f.ID, "", pagina.Params{Limite: 50})
	if len(h) != 2 || h[0].Origem != "automacao" || *h[0].EtapaOrigemNome != "Novo" || *h[0].EtapaDestinoNome != "Quente" ||
		*h[0].AutomacaoID != "a1" || h[1].Origem != "app" || h[1].EtapaOrigemID != nil {
		t.Fatalf("histórico: %+v", h)
	}
	// Fatos entrou_etapa (um por movimento real) com a etapa anterior.
	col.mu.Lock()
	if len(col.l) != 2 || col.l[1].EtapaID != quente || col.l[1].EtapaAnteriorID != novo || col.l[0].EtapaAnteriorID != "" {
		t.Fatalf("fatos: %+v", col.l)
	}
	col.mu.Unlock()
	// Eventos funil.movido.
	if n := len(amb.EventosDoTipo(eventos.FunilMovido)); n != 2 {
		t.Fatalf("funil.movido: %d", n)
	}
	// Etapa de outro funil; grupo; telefone inválido; alvo ambíguo.
	g, _ := s.Criar(ctx, "Pós-venda", []NovaEtapa{{Nome: "Ativo"}})
	if _, _, err := s.Mover(ctx, f.ID, AlvoCard{LeadID: card.LeadID}, g.Etapas[0].ID, app); !erros.Eh(err, erros.Validacao) {
		t.Fatalf("etapa de outro funil: %v", err)
	}
	grupo := amb.Grupo(conta, "Turma")
	var ctGrupo string
	amb.Banco.L().QueryRow(`SELECT id FROM contatos LIMIT 0`).Scan(&ctGrupo)
	_ = grupo
	amb.Exec(`INSERT INTO contatos (id, conta_id, jid, criado_em, atualizado_em) VALUES ('ctg', ?, '123@g.us', 0, 0)`, conta)
	if _, _, err := s.Mover(ctx, f.ID, AlvoCard{ContatoID: "ctg"}, novo, app); !erros.Eh(err, erros.Validacao) || !strings.Contains(err.Error(), "Grupos") {
		t.Fatalf("grupo: %v", err)
	}
	if _, _, err := s.Mover(ctx, f.ID, AlvoCard{Telefone: "123"}, novo, app); !erros.Eh(err, erros.Validacao) {
		t.Fatalf("telefone inválido: %v", err)
	}
	if _, _, err := s.Mover(ctx, f.ID, AlvoCard{LeadID: card.LeadID, Telefone: "+5511911110000"}, novo, app); !erros.Eh(err, erros.Validacao) {
		t.Fatalf("dois alvos: %v", err)
	}
	// Telefone novo pela origem mcp cria lead mcp.
	c2, _, err := s.Mover(ctx, f.ID, AlvoCard{Telefone: "+5511922220000"}, novo, Origem{Tipo: dominio.OrigemMCPFunil})
	if err != nil {
		t.Fatal(err)
	}
	if l2, _ := armazenamento.ObterLead(ctx, amb.Banco.L(), c2.LeadID); l2.Origem != dominio.OrigemMCP {
		t.Fatalf("origem mcp: %s", l2.Origem)
	}
	// Remover (idempotente) gera histórico de saída e card null no evento.
	if err := s.Remover(ctx, f.ID, c2.LeadID, app); err != nil {
		t.Fatal(err)
	}
	if err := s.Remover(ctx, f.ID, c2.LeadID, app); err != nil {
		t.Fatal("remover de novo deveria ser idempotente")
	}
	h, _, _ = s.Historico(ctx, f.ID, c2.LeadID, pagina.Params{Limite: 50})
	if len(h) != 2 || h[0].EtapaDestinoID != nil {
		t.Fatalf("saída: %+v", h)
	}
	fl, _ := s.FunisDoLead(ctx, card.LeadID)
	if len(fl) != 2 || fl[0].Card == nil || fl[1].Card != nil {
		t.Fatalf("funis do lead: %+v", fl)
	}
}

func TestExcluirEtapaComCards(t *testing.T) {
	amb, s, _ := montar(t)
	ctx := amb.Ctx
	f, _ := s.Criar(ctx, "Vendas", []NovaEtapa{{Nome: "Novo"}, {Nome: "Quente"}, {Nome: "Frio"}})
	for _, tel := range []string{"+5511933330001", "+5511933330002"} {
		if _, _, err := s.Mover(ctx, f.ID, AlvoCard{Telefone: tel}, f.Etapas[0].ID, app); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ExcluirEtapa(ctx, f.Etapas[0].ID, "", false, app); !erros.Eh(err, erros.Validacao) || !strings.Contains(err.Error(), "Escolha para onde mover os cards.") {
		t.Fatalf("exige destino: %v", err)
	}
	if err := s.ExcluirEtapa(ctx, f.Etapas[0].ID, f.Etapas[1].ID, false, Origem{Tipo: dominio.OrigemMCPFunil}); err != nil {
		t.Fatal(err)
	}
	f2, _ := s.Obter(ctx, f.ID)
	if len(f2.Etapas) != 2 || f2.Etapas[0].Nome != "Quente" || f2.Etapas[0].Ordem != 0 || f2.Etapas[0].TotalCards != 2 || f2.TotalCards != 2 {
		t.Fatalf("depois de excluir com destino: %+v", f2)
	}
	h, _, _ := s.Historico(ctx, f.ID, "", pagina.Params{Limite: 50})
	if h[0].Origem != "mcp" || *h[0].EtapaOrigemNome != "Novo" {
		t.Fatalf("histórico copia o nome da etapa excluída: %+v", h[0])
	}
	if err := s.ExcluirEtapa(ctx, f2.Etapas[0].ID, "", true, app); err != nil {
		t.Fatal(err)
	}
	f3, _ := s.Obter(ctx, f.ID)
	if f3.TotalCards != 0 || len(f3.Etapas) != 1 {
		t.Fatalf("remover cards: %+v", f3)
	}
	if err := s.Excluir(ctx, f.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Excluir(ctx, f.ID); !erros.Eh(err, erros.NaoEncontrado) {
		t.Fatal("excluir de novo")
	}
	_ = context.Background
}
