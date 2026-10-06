package gatilhos

import (
	"testing"
	"time"

	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/fatos"
)

func s(v string) *string { return &v }

func recebida(texto string) fatos.Fato {
	return fatos.Fato{Tipo: fatos.MensagemRecebida, ContaID: "c1", ConversaID: "v1", Texto: texto}
}

func cand(id string, gs ...modelo.Gatilho) Candidata {
	return Candidata{ID: id, Prioridade: 100, Gatilhos: gs}
}

func casa(g modelo.Gatilho, f fatos.Fato) bool { return len(Casar([]Candidata{cand("a", g)}, f)) == 1 }

func TestMensagemRecebida(t *testing.T) {
	g := modelo.Gatilho{Tipo: modelo.GatilhoMensagemRecebida}
	if !casa(g, recebida("qualquer coisa")) || !casa(g, recebida("")) {
		t.Fatal("sem filtros casa qualquer mensagem")
	}
	g.Contem = s("Preço")
	if !casa(g, recebida("qual o PRECO disso?")) || casa(g, recebida("qual o valor?")) {
		t.Fatal("contem sem maiúsculas/acentos")
	}
	g = modelo.Gatilho{Tipo: modelo.GatilhoMensagemRecebida, Regex: s(`^\d{5}-?\d{3}$`)}
	if !casa(g, recebida("01310-100")) || casa(g, recebida("cep 01310-100")) {
		t.Fatal("regex")
	}
	g = modelo.Gatilho{Tipo: modelo.GatilhoMensagemRecebida, Regex: s(`(?i)boleto`)}
	if !casa(g, recebida("Manda o BOLETO")) {
		t.Fatal("regex (?i)")
	}
	g = modelo.Gatilho{Tipo: modelo.GatilhoMensagemRecebida, PrimeiraMensagem: true}
	f := recebida("oi")
	if casa(g, f) {
		t.Fatal("primeira_mensagem com mensagem que não é a primeira")
	}
	f.Primeira = true
	if !casa(g, f) {
		t.Fatal("primeira_mensagem")
	}
}

func TestPalavraChave(t *testing.T) {
	g := modelo.Gatilho{Tipo: modelo.GatilhoPalavraChave, Palavras: []string{"orçamento", "preço", "bom dia"}}
	for _, txt := range []string{"quero um ORCAMENTO!", "Preço?", "bom   dia, tudo bem?"} {
		if !casa(g, recebida(txt)) {
			t.Errorf("palavra: %q deveria casar", txt)
		}
	}
	for _, txt := range []string{"precos", "orçamentos", "bomdia"} {
		if casa(g, recebida(txt)) {
			t.Errorf("palavra isolada: %q não deveria casar", txt)
		}
	}
	g.Modo = "mensagem_inteira"
	if !casa(g, recebida("  Orçamento! ")) || casa(g, recebida("quero orçamento")) {
		t.Fatal("mensagem_inteira")
	}
}

func TestGruposEContas(t *testing.T) {
	g := modelo.Gatilho{Tipo: modelo.GatilhoMensagemRecebida}
	fg := recebida("oi")
	fg.Grupo = true
	if casa(g, fg) {
		t.Fatal("grupo sem incluir_grupos")
	}
	c := cand("a", modelo.Gatilho{Tipo: modelo.GatilhoMensagemRecebida, TipoConversa: "qualquer"})
	c.IncluirGrupos = true
	if len(Casar([]Candidata{c}, fg)) != 1 {
		t.Fatal("grupo com incluir_grupos e tipo qualquer")
	}
	c.Gatilhos[0].TipoConversa = ""
	if len(Casar([]Candidata{c}, fg)) != 0 {
		t.Fatal("tipo_conversa padrão é individual")
	}
	c.Gatilhos[0].TipoConversa = "grupo"
	if len(Casar([]Candidata{c}, fg)) != 1 || len(Casar([]Candidata{c}, recebida("x"))) != 0 {
		t.Fatal("tipo_conversa grupo")
	}
	pc := cand("p", modelo.Gatilho{Tipo: modelo.GatilhoPalavraChave, Palavras: []string{"oi"}})
	if len(Casar([]Candidata{pc}, fg)) != 0 {
		t.Fatal("palavra-chave em grupo sem incluir_grupos")
	}
	c2 := cand("a", modelo.Gatilho{Tipo: modelo.GatilhoMensagemRecebida})
	c2.Contas = []string{"c2"}
	if len(Casar([]Candidata{c2}, recebida("x"))) != 0 {
		t.Fatal("conta fora da lista")
	}
	c2.Contas = []string{"c2", "c1"}
	if len(Casar([]Candidata{c2}, recebida("x"))) != 1 {
		t.Fatal("conta na lista")
	}
}

func TestNuncaDeMim(t *testing.T) {
	f := fatos.Fato{Tipo: fatos.MensagemEnviada, ContaID: "c1", ConversaID: "v1", Texto: "oi", Origem: fatos.OrigemManual}
	if casa(modelo.Gatilho{Tipo: modelo.GatilhoMensagemRecebida}, f) || casa(modelo.Gatilho{Tipo: modelo.GatilhoPalavraChave, Palavras: []string{"oi"}}, f) {
		t.Fatal("mensagem minha nunca dispara gatilho de mensagem")
	}
}

func TestOutrosTipos(t *testing.T) {
	et := fatos.Fato{Tipo: fatos.Etiqueta, ContaID: "c1", EtiquetaID: "e1", Evento: "adicionada"}
	if !casa(modelo.Gatilho{Tipo: modelo.GatilhoEtiqueta, Evento: "adicionada", EtiquetaID: "e1"}, et) ||
		casa(modelo.Gatilho{Tipo: modelo.GatilhoEtiqueta, Evento: "removida", EtiquetaID: "e1"}, et) ||
		casa(modelo.Gatilho{Tipo: modelo.GatilhoEtiqueta, Evento: "adicionada", EtiquetaID: "e2"}, et) {
		t.Fatal("etiqueta")
	}
	ee := fatos.Fato{Tipo: fatos.EntrouEtapa, FunilID: "f1", EtapaID: "s1"}
	if !casa(modelo.Gatilho{Tipo: modelo.GatilhoEntrouEtapa, FunilID: "f1", EtapaID: "s1"}, ee) ||
		casa(modelo.Gatilho{Tipo: modelo.GatilhoEntrouEtapa, FunilID: "f1", EtapaID: "s2"}, ee) {
		t.Fatal("entrou_etapa")
	}
	dr := fatos.Fato{Tipo: fatos.DisparoRespondeu, ContaID: "c1", DisparoID: "d1"}
	if !casa(modelo.Gatilho{Tipo: modelo.GatilhoDisparoRespondeu}, dr) || !casa(modelo.Gatilho{Tipo: modelo.GatilhoDisparoRespondeu, DisparoID: s("d1")}, dr) ||
		casa(modelo.Gatilho{Tipo: modelo.GatilhoDisparoRespondeu, DisparoID: s("d2")}, dr) {
		t.Fatal("disparo_respondeu")
	}
	li := fatos.Fato{Tipo: fatos.LeadImportado, LeadIDs: []string{"l1"}, OrigemLead: "csv"}
	if !casa(modelo.Gatilho{Tipo: modelo.GatilhoLeadImportado}, li) || !casa(modelo.Gatilho{Tipo: modelo.GatilhoLeadImportado, Origens: []string{"csv"}}, li) ||
		casa(modelo.Gatilho{Tipo: modelo.GatilhoLeadImportado, Origens: []string{"mcp"}}, li) {
		t.Fatal("lead_importado")
	}
	// Gatilhos sem fato (sem_resposta, agendamento, manual) nunca casam com fatos.
	for _, g := range []modelo.Gatilho{{Tipo: modelo.GatilhoManual}, {Tipo: modelo.GatilhoAgendamento, IntervaloS: 60}, {Tipo: modelo.GatilhoSemResposta, AposS: 60}} {
		if casa(g, recebida("oi")) {
			t.Fatalf("%s casou com mensagem", g.Tipo)
		}
	}
}

func TestCadeiaEOrdem(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	g := modelo.Gatilho{Tipo: modelo.GatilhoEtiqueta, Evento: "adicionada", EtiquetaID: "e1"}
	a := Candidata{ID: "a", Prioridade: 200, CriadaEm: t0, Gatilhos: []modelo.Gatilho{g}}
	b := Candidata{ID: "b", Prioridade: 100, CriadaEm: t0.Add(time.Hour), Gatilhos: []modelo.Gatilho{g}}
	c := Candidata{ID: "c", Prioridade: 100, CriadaEm: t0, Gatilhos: []modelo.Gatilho{{Tipo: modelo.GatilhoManual}, g}}
	f := fatos.Fato{Tipo: fatos.Etiqueta, EtiquetaID: "e1", Evento: "adicionada"}
	r := Casar([]Candidata{a, b, c}, f)
	if len(r) != 3 || r[0].Automacao.ID != "c" || r[1].Automacao.ID != "b" || r[2].Automacao.ID != "a" || r[0].Indice != 1 {
		t.Fatalf("ordem por prioridade e criação: %+v", r)
	}
	// Auto-disparo proibido: quem está na cadeia não roda de novo.
	f.Cadeia = []string{"b"}
	r = Casar([]Candidata{a, b, c}, f)
	if len(r) != 2 || r[0].Automacao.ID != "c" || r[1].Automacao.ID != "a" {
		t.Fatalf("cadeia: %+v", r)
	}
	// Profundidade > 5 interrompe.
	f.Cadeia = []string{"x1", "x2", "x3", "x4", "x5"}
	if r := Casar([]Candidata{a, b, c}, f); len(r) != 0 {
		t.Fatalf("profundidade 6 deveria parar: %+v", r)
	}
	f.Cadeia = []string{"x1", "x2", "x3", "x4"}
	if r := Casar([]Candidata{a, b, c}, f); len(r) != 3 {
		t.Fatalf("profundidade 5 ainda roda: %+v", r)
	}
}
