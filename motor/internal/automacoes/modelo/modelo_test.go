package modelo

import (
	"encoding/json"
	"strings"
	"testing"
)

// chatbotExemplo é o bot de formatos.md › DefinicaoChatbot.
const chatbotExemplo = `{ "versao": 1, "inicio": "n1",
  "nao_entendi": "Não entendi. Responda com uma das opções.", "max_tentativas": 3, "inatividade_min": 30,
  "nos": [
    { "id": "n1", "tipo": "inicio", "proximo": "n2", "posicao": { "x": 0, "y": 0 } },
    { "id": "n2", "tipo": "mensagem", "texto": "Olá, {nome}!", "template_id": null, "proximo": "n3" },
    { "id": "n3", "tipo": "menu", "texto": "Como posso ajudar?",
      "opcoes": [ { "rotulo": "Preços", "valores": ["preco", "valores"], "proximo": "n4" },
                  { "rotulo": "Falar com vendedor", "valores": [], "proximo": "n8" } ],
      "mostrar_numeros": true, "ao_esgotar": null },
    { "id": "n4", "tipo": "pergunta", "texto": "Qual seu e-mail?", "variavel": "email",
      "validacao": { "tipo": "email", "padrao": null, "mensagem_erro": "E-mail inválido. Tente de novo." },
      "proximo": "n5", "ao_esgotar": null },
    { "id": "n5", "tipo": "condicao",
      "ramos": [ { "condicoes": { "modo": "todas", "regras": [ { "tipo": "variavel", "variavel": "email", "operador": "contem", "valor": "@empresa.com" } ] }, "proximo": "n6" } ],
      "senao": "n7" },
    { "id": "n6", "tipo": "acao", "acao": { "tipo": "atualizar_campo_lead", "campo": "email", "valor": "{email}" }, "proximo": "n7" },
    { "id": "n7", "tipo": "ia", "automacao_id": "01JIA", "entrada": { "email": "{email}" },
      "modo": "responder", "variavel": null, "proximo": "n9", "em_erro": "n8" },
    { "id": "n8", "tipo": "humano", "mensagem": "Vou chamar um atendente, só um instante." },
    { "id": "n9", "tipo": "fim", "mensagem": "Obrigado!" }
  ] }`

const fluxoExemplo = `{ "versao": 1,
  "condicoes": { "modo": "todas", "regras": [
    { "tipo": "etiqueta", "operador": "tem", "etiqueta_id": "01E" },
    { "tipo": "etapa", "operador": "esta", "funil_id": "01F", "etapa_id": null },
    { "tipo": "campo_lead", "campo": "empresa", "operador": "igual", "valor": "ACME" },
    { "tipo": "horario", "inicio": "22:00", "fim": "06:00", "dias": [1,2,3,4,5] },
    { "tipo": "texto", "operador": "regex", "valor": "(?i)boleto" },
    { "tipo": "conta", "conta_ids": ["01C"] },
    { "tipo": "variavel", "variavel": "email", "operador": "existe", "valor": null } ] },
  "acoes": [
    { "tipo": "enviar_texto", "texto": "Oi {primeiro_nome}!", "valores_padrao": { "primeiro_nome": "tudo bem" } },
    { "tipo": "enviar_template", "template_id": "01T", "valores_padrao": {} },
    { "tipo": "aguardar", "duracao_s": 7200 },
    { "tipo": "adicionar_etiqueta", "etiqueta_id": "01E" },
    { "tipo": "remover_etiqueta", "etiqueta_id": "01E" },
    { "tipo": "mover_etapa", "funil_id": "01F", "etapa_id": "01S" },
    { "tipo": "remover_do_funil", "funil_id": "01F" },
    { "tipo": "atualizar_nota", "texto": "Interesse: {interesse}", "modo": "acrescentar" },
    { "tipo": "atualizar_campo_lead", "campo": "interesse", "valor": "" },
    { "tipo": "iniciar_chatbot", "automacao_id": "01B" },
    { "tipo": "executar_ia", "automacao_id": "01I", "entrada": { "pergunta": "{ultima_mensagem}" }, "salvar_em": "resposta" },
    { "tipo": "adicionar_a_disparo", "disparo_id": "01D" },
    { "tipo": "pausar_automacoes", "duracao_min": 60 },
    { "tipo": "notificar", "titulo": "Lead quente", "texto": "{nome} respondeu" } ] }`

const gatilhosExemplo = `[
  { "tipo": "mensagem_recebida", "contem": "preço", "regex": "^\\d{5}-?\\d{3}$", "primeira_mensagem": false, "tipo_conversa": "individual" },
  { "tipo": "palavra_chave", "palavras": ["orçamento", "preço"], "modo": "palavra" },
  { "tipo": "lead_importado", "origens": ["csv", "colado", "contatos", "mcp"] },
  { "tipo": "etiqueta", "evento": "adicionada", "etiqueta_id": "01J" },
  { "tipo": "entrou_etapa", "funil_id": "01F", "etapa_id": "01E" },
  { "tipo": "disparo_respondeu", "disparo_id": null },
  { "tipo": "sem_resposta", "apos_s": 7200, "origem_mensagem": "qualquer" },
  { "tipo": "agendamento", "cron": "0 9 * * 1-5" },
  { "tipo": "agendamento", "intervalo_s": 3600 },
  { "tipo": "manual" } ]`

func caminhos(es []ErroDefinicao) []string {
	var l []string
	for _, e := range es {
		l = append(l, e.Caminho)
	}
	return l
}

func contem(es []ErroDefinicao, caminho string) bool {
	for _, e := range es {
		if e.Caminho == caminho {
			return true
		}
	}
	return false
}

func TestExemplosValidos(t *testing.T) {
	g, errs := DecodificarGatilhos(json.RawMessage(gatilhosExemplo))
	if errs != nil {
		t.Fatal(errs)
	}
	if es := ValidarGatilhos(g, OpcoesGatilhos{}); len(es) != 0 {
		t.Fatalf("gatilhos válidos rejeitados: %+v", es)
	}
	f, errs := DecodificarFluxo(json.RawMessage(fluxoExemplo))
	if errs != nil {
		t.Fatal(errs)
	}
	if es := ValidarFluxo(f); len(es) != 0 {
		t.Fatalf("fluxo válido rejeitado: %+v", es)
	}
	cb, errs := DecodificarChatbot(json.RawMessage(chatbotExemplo))
	if errs != nil {
		t.Fatal(errs)
	}
	if es := ValidarChatbot(cb); len(es) != 0 {
		t.Fatalf("chatbot válido rejeitado: %+v", es)
	}
}

func TestIdsGeradosEUnicos(t *testing.T) {
	d := &DefinicaoFluxo{Acoes: []Acao{{Tipo: AcaoNotificar, Titulo: "t", Texto: "x"}, {ID: "a1", Tipo: AcaoNotificar, Titulo: "t", Texto: "x"}, {Tipo: AcaoNotificar, Titulo: "t", Texto: "x"}}}
	NormalizarFluxo(d)
	if d.Versao != 1 || d.Acoes[0].ID != "a2" || d.Acoes[2].ID != "a3" {
		t.Fatalf("ids: %+v", d.Acoes)
	}
	d.Acoes[2].ID = "a1"
	es := ValidarFluxo(d)
	if !contem(es, "definicao.acoes[2].id") || es[0].AcaoID == nil || *es[0].AcaoID != "a1" {
		t.Fatalf("id repetido: %+v", es)
	}
}

func TestErrosDeGatilho(t *testing.T) {
	longo := strings.Repeat("a", 501)
	casos := []struct {
		g       Gatilho
		o       OpcoesGatilhos
		caminho string
	}{
		{Gatilho{Tipo: "x"}, OpcoesGatilhos{}, "gatilhos[0].tipo"},
		{Gatilho{Tipo: GatilhoMensagemRecebida, Regex: &longo}, OpcoesGatilhos{}, "gatilhos[0].regex"},
		{Gatilho{Tipo: GatilhoMensagemRecebida, Regex: ptr("a(")}, OpcoesGatilhos{}, "gatilhos[0].regex"},
		{Gatilho{Tipo: GatilhoMensagemRecebida, TipoConversa: "grupo"}, OpcoesGatilhos{}, "gatilhos[0].tipo_conversa"},
		{Gatilho{Tipo: GatilhoPalavraChave}, OpcoesGatilhos{}, "gatilhos[0].palavras"},
		{Gatilho{Tipo: GatilhoPalavraChave, Palavras: []string{"!!"}}, OpcoesGatilhos{}, "gatilhos[0].palavras[0]"},
		{Gatilho{Tipo: GatilhoLeadImportado, Origens: []string{"x"}}, OpcoesGatilhos{}, "gatilhos[0].origens[0]"},
		{Gatilho{Tipo: GatilhoEtiqueta, Evento: "adicionada"}, OpcoesGatilhos{}, "gatilhos[0].etiqueta_id"},
		{Gatilho{Tipo: GatilhoEntrouEtapa, FunilID: "f"}, OpcoesGatilhos{}, "gatilhos[0].etapa_id"},
		{Gatilho{Tipo: GatilhoSemResposta, AposS: 59}, OpcoesGatilhos{}, "gatilhos[0].apos_s"},
		{Gatilho{Tipo: GatilhoSemResposta, AposS: 2592001}, OpcoesGatilhos{}, "gatilhos[0].apos_s"},
		{Gatilho{Tipo: GatilhoAgendamento}, OpcoesGatilhos{}, "gatilhos[0]"},
		{Gatilho{Tipo: GatilhoAgendamento, Cron: "0 9 * *"}, OpcoesGatilhos{}, "gatilhos[0].cron"},
		{Gatilho{Tipo: GatilhoAgendamento, Cron: "99 9 * * *"}, OpcoesGatilhos{}, "gatilhos[0].cron"},
		{Gatilho{Tipo: GatilhoAgendamento, IntervaloS: 59}, OpcoesGatilhos{}, "gatilhos[0].intervalo_s"},
		{Gatilho{Tipo: GatilhoAgendamento, IntervaloS: 60, Cron: "* * * * *"}, OpcoesGatilhos{}, "gatilhos[0]"},
	}
	for i, c := range casos {
		es := ValidarGatilhos([]Gatilho{c.g}, c.o)
		if !contem(es, c.caminho) {
			t.Errorf("caso %d: esperava erro em %s, veio %v", i, c.caminho, caminhos(es))
		}
	}
	// Com incluir_grupos, gatilho de grupo é aceito; no manifesto, nomes substituem ids.
	if es := ValidarGatilhos([]Gatilho{{Tipo: GatilhoMensagemRecebida, TipoConversa: "grupo"}}, OpcoesGatilhos{IncluirGrupos: true}); len(es) != 0 {
		t.Fatal(es)
	}
	if es := ValidarGatilhos([]Gatilho{{Tipo: GatilhoEntrouEtapa, Funil: "Vendas", Etapa: "Novo"}}, OpcoesGatilhos{Manifesto: true}); len(es) != 0 {
		t.Fatal(es)
	}
}

func ptr(s string) *string { return &s }
func iptr(i int) *int      { return &i }

func TestErrosDeCondicaoEAcao(t *testing.T) {
	regras := make([]Regra, 21)
	for i := range regras {
		regras[i] = Regra{Tipo: RegraConta, ContaIDs: []string{"c"}}
	}
	d := &DefinicaoFluxo{Versao: 1, Condicoes: &Condicoes{Modo: "cada", Regras: regras}, Acoes: []Acao{{Tipo: AcaoNotificar, Titulo: "t", Texto: "x"}}}
	es := ValidarFluxo(d)
	if !contem(es, "definicao.condicoes.modo") || !contem(es, "definicao.condicoes.regras") {
		t.Fatalf("condições: %v", caminhos(es))
	}
	regrasRuins := []Regra{
		{Tipo: RegraEtiqueta, Operador: "x"},
		{Tipo: RegraCampoLead, Campo: "a", Operador: "igual"},
		{Tipo: RegraHorario, Inicio: "25:00", Fim: "09:00", Dias: []int{0}},
		{Tipo: RegraTexto, Operador: "regex", Valor: ptr("(")},
		{Tipo: RegraConta},
		{Tipo: RegraVariavel, Variavel: "Email", Operador: "existe"},
		{Tipo: "?"},
	}
	d = &DefinicaoFluxo{Versao: 1, Condicoes: &Condicoes{Modo: ModoAlguma, Regras: regrasRuins}, Acoes: []Acao{{Tipo: AcaoNotificar, Titulo: "t", Texto: "x"}}}
	es = ValidarFluxo(d)
	for _, c := range []string{"definicao.condicoes.regras[0].operador", "definicao.condicoes.regras[0].etiqueta_id",
		"definicao.condicoes.regras[1].valor", "definicao.condicoes.regras[2].inicio", "definicao.condicoes.regras[2].dias[0]",
		"definicao.condicoes.regras[3].valor", "definicao.condicoes.regras[4].conta_ids", "definicao.condicoes.regras[5].variavel",
		"definicao.condicoes.regras[6].tipo"} {
		if !contem(es, c) {
			t.Errorf("faltou %s em %v", c, caminhos(es))
		}
	}

	acoes := []Acao{
		{Tipo: AcaoEnviarTexto, Texto: strings.Repeat("x", 4097)},
		{Tipo: AcaoEnviarTemplate},
		{Tipo: AcaoAguardar, DuracaoS: 30},
		{Tipo: AcaoAdicionarEtiqueta},
		{Tipo: AcaoMoverEtapa, FunilID: "f"},
		{Tipo: AcaoAtualizarNota, Texto: "x", Modo: "juntar"},
		{Tipo: AcaoAtualizarCampoLead, Campo: "a"},
		{Tipo: AcaoExecutarIA, AutomacaoID: "i", SalvarEm: ptr("1x")},
		{Tipo: AcaoAdicionarADisparo},
		{Tipo: AcaoPausarAutomacoes, DuracaoMin: iptr(0)},
		{Tipo: AcaoNotificar, Titulo: strings.Repeat("t", 61), Texto: strings.Repeat("x", 241)},
		{Tipo: AcaoNotificar, ID: "id com espaço", Titulo: "t", Texto: "x"},
	}
	es = ValidarFluxo(&DefinicaoFluxo{Versao: 1, Acoes: acoes})
	for _, c := range []string{"definicao.acoes[0].texto", "definicao.acoes[1].template_id", "definicao.acoes[2].duracao_s",
		"definicao.acoes[3].etiqueta_id", "definicao.acoes[4].etapa_id", "definicao.acoes[5].modo", "definicao.acoes[6].valor",
		"definicao.acoes[7].salvar_em", "definicao.acoes[8].disparo_id", "definicao.acoes[9].duracao_min",
		"definicao.acoes[10].titulo", "definicao.acoes[10].texto", "definicao.acoes[11].id"} {
		if !contem(es, c) {
			t.Errorf("faltou %s em %v", c, caminhos(es))
		}
	}
	muitas := make([]Acao, 51)
	for i := range muitas {
		muitas[i] = Acao{Tipo: AcaoNotificar, Titulo: "t", Texto: "x"}
	}
	if !contem(ValidarFluxo(&DefinicaoFluxo{Versao: 1, Acoes: muitas}), "definicao.acoes") {
		t.Fatal("51 ações aceitas")
	}
	if !contem(ValidarFluxo(&DefinicaoFluxo{Versao: 1}), "definicao.acoes") {
		t.Fatal("fluxo sem ações aceito")
	}
}

func TestErrosDeChatbot(t *testing.T) {
	base := func() *DefinicaoChatbot {
		d, _ := DecodificarChatbot(json.RawMessage(chatbotExemplo))
		return d
	}
	muda := func(f func(d *DefinicaoChatbot)) []ErroDefinicao {
		d := base()
		f(d)
		return ValidarChatbot(d)
	}
	casos := []struct {
		nome    string
		f       func(d *DefinicaoChatbot)
		caminho string
		noID    string
	}{
		{"dois inícios", func(d *DefinicaoChatbot) { d.Nos[1].Tipo = NoInicio }, "definicao.nos", ""},
		{"id repetido", func(d *DefinicaoChatbot) { d.Nos[2].ID = "n2" }, "definicao.nos[2].id", "n2"},
		{"id inválido", func(d *DefinicaoChatbot) { d.Nos[2].ID = "n 3" }, "definicao.nos[2].id", ""},
		{"próximo inexistente", func(d *DefinicaoChatbot) { d.Nos[1].Proximo = "zz" }, "definicao.nos[1].proximo", "n2"},
		{"mensagem sem saída", func(d *DefinicaoChatbot) { d.Nos[1].Proximo = "" }, "definicao.nos[1].proximo", "n2"},
		{"menu sem opções", func(d *DefinicaoChatbot) { d.Nos[2].Opcoes = nil }, "definicao.nos[2].opcoes", "n3"},
		{"variável inválida", func(d *DefinicaoChatbot) { d.Nos[3].Variavel = ptr("E-mail") }, "definicao.nos[3].variavel", "n4"},
		{"regex sem padrão", func(d *DefinicaoChatbot) { d.Nos[3].Validacao.Tipo = "regex" }, "definicao.nos[3].validacao.padrao", "n4"},
		{"condição sem senão", func(d *DefinicaoChatbot) { d.Nos[4].Senao = "" }, "definicao.nos[4].senao", "n5"},
		{"aguardar proibido", func(d *DefinicaoChatbot) { d.Nos[5].Acao = &Acao{Tipo: AcaoAguardar, DuracaoS: 60} }, "definicao.nos[5].acao.tipo", "n6"},
		{"iniciar_chatbot proibido", func(d *DefinicaoChatbot) { d.Nos[5].Acao = &Acao{Tipo: AcaoIniciarChatbot, AutomacaoID: "x"} }, "definicao.nos[5].acao.tipo", "n6"},
		{"ia modo variável sem variável", func(d *DefinicaoChatbot) { d.Nos[6].ModoIA = "variavel" }, "definicao.nos[6].variavel", "n7"},
		{"fim com saída", func(d *DefinicaoChatbot) { d.Nos[8].Proximo = "n1" }, "definicao.nos[8].proximo", "n9"},
		{"tentativas", func(d *DefinicaoChatbot) { d.MaxTentativas = 11 }, "definicao.max_tentativas", ""},
		{"inatividade", func(d *DefinicaoChatbot) { d.InatividadeMin = 0 }, "definicao.inatividade_min", ""},
		{"não entendi", func(d *DefinicaoChatbot) { d.NaoEntendi = "" }, "definicao.nao_entendi", ""},
		{"início errado", func(d *DefinicaoChatbot) { d.Inicio = "n2" }, "definicao.inicio", ""},
		{"inalcançável", func(d *DefinicaoChatbot) {
			d.Nos = append(d.Nos, No{ID: "solto", Tipo: NoFim})
		}, "definicao.nos[9]", "solto"},
		{"variável antes da pergunta", func(d *DefinicaoChatbot) { d.Nos[1].Texto = "Seu e-mail é {email}?" }, "definicao.nos[1]", "n2"},
		{"condição com variável não capturada", func(d *DefinicaoChatbot) {
			d.Nos[4].Ramos[0].Condicoes.Regras[0].Variavel = "cpf"
		}, "definicao.nos[4]", "n5"},
	}
	for _, c := range casos {
		es := muda(c.f)
		achou := false
		for _, e := range es {
			if e.Caminho == c.caminho && (c.noID == "" || (e.NoID != nil && *e.NoID == c.noID)) {
				achou = true
			}
		}
		if !achou {
			t.Errorf("%s: esperava %s (nó %q), veio %+v", c.nome, c.caminho, c.noID, es)
		}
	}
	// Texto com {empresa} (campo do lead, não capturado por pergunta) é aceito.
	if es := muda(func(d *DefinicaoChatbot) { d.Nos[1].Texto = "Olá, {empresa}!" }); len(es) != 0 {
		t.Fatalf("campo do lead rejeitado: %+v", es)
	}
	// 1 nó e 201 nós.
	if es := muda(func(d *DefinicaoChatbot) { d.Nos = d.Nos[:1] }); !contem(es, "definicao.nos") {
		t.Fatal("1 nó aceito")
	}
}

func TestSerializacaoEmiteNulos(t *testing.T) {
	b, _ := json.Marshal(Acao{ID: "a1", Tipo: AcaoPausarAutomacoes})
	if !strings.Contains(string(b), `"duracao_min":null`) {
		t.Fatalf("pausar sem duracao_min null: %s", b)
	}
	b, _ = json.Marshal(Regra{Tipo: RegraEtapa, Operador: "esta", FunilID: "f"})
	if !strings.Contains(string(b), `"etapa_id":null`) {
		t.Fatalf("regra etapa sem etapa_id null: %s", b)
	}
	b, _ = json.Marshal(Gatilho{Tipo: GatilhoManual})
	if string(b) != `{"tipo":"manual"}` {
		t.Fatalf("gatilho manual: %s", b)
	}
	d, _ := DecodificarChatbot(json.RawMessage(chatbotExemplo))
	b, _ = json.Marshal(d)
	var volta DefinicaoChatbot
	if err := json.Unmarshal(b, &volta); err != nil || len(ValidarChatbot(&volta)) != 0 {
		t.Fatalf("ida e volta do chatbot: %v %s", err, b)
	}
}

func TestDecodificarErroDeTipo(t *testing.T) {
	_, es := DecodificarFluxo(json.RawMessage(`{"versao":1,"acoes":[{"tipo":"aguardar","duracao_s":"duas horas"}]}`))
	if len(es) != 1 || !strings.HasPrefix(es[0].Caminho, "definicao.acoes") {
		t.Fatalf("erro de tipo: %+v", es)
	}
	_, es = DecodificarFluxo(json.RawMessage(`null`))
	if len(es) != 1 || es[0].Caminho != "definicao" {
		t.Fatalf("definição nula: %+v", es)
	}
}

func TestManifesto(t *testing.T) {
	ok := `{"$schema":"./.zapdesk/automacao.schema.json","versao_manifesto":1,"nome":"Responder com IA",
	  "descricao":"x","entrada":"index.ts","gatilhos":[{"tipo":"mensagem_recebida"},{"tipo":"etiqueta","evento":"adicionada","etiqueta":"Quente"}],
	  "permissoes":["ler_conversas","enviar","ia"],"segredos":["OPENAI_KEY"],"contas":"todas","incluir_grupos":false,
	  "prioridade":100,"conta_envio":null,"limites":{"tempo_s":60,"memoria_mb":256,"anti_loop":null},"ia":{"modelo":null}}`
	m, es := DecodificarManifesto([]byte(ok))
	if es != nil {
		t.Fatal(es)
	}
	if es := ValidarManifesto(m); len(es) != 0 {
		t.Fatalf("manifesto válido rejeitado: %+v", es)
	}
	if m.Contas.Lista != nil || m.ArquivoEntrada() != "index.ts" || !m.TemPermissao("ia") || m.TemPermissao("rede") {
		t.Fatalf("manifesto lido: %+v", m)
	}
	ruim := `{"versao_manifesto":2,"nome":"","gatilhos":[],"permissoes":["voar","ia","ia"],"segredos":["ANTHROPIC_API_KEY","minusculo"],
	  "prioridade":0,"limites":{"tempo_s":1,"memoria_mb":10},"ia":{"modelo":"gpt-4"},"entrada":"../x.ts"}`
	m, es = DecodificarManifesto([]byte(ruim))
	if es != nil {
		t.Fatal(es)
	}
	es = ValidarManifesto(m)
	for _, c := range []string{"versao_manifesto", "nome", "gatilhos", "permissoes[0]", "permissoes[2]", "segredos[0]", "segredos[1]",
		"prioridade", "limites.tempo_s", "limites.memoria_mb", "ia.modelo", "entrada"} {
		if !contem(es, c) {
			t.Errorf("faltou %s em %v", c, caminhos(es))
		}
	}
	_, es = DecodificarManifesto([]byte(`{"versao_manifesto":1,"nomee":"x"}`))
	if len(es) != 1 || es[0].Caminho != "nomee" {
		t.Fatalf("campo desconhecido: %+v", es)
	}
	_, es = DecodificarManifesto([]byte(`{"contas":"algumas"}`))
	if len(es) != 1 {
		t.Fatalf("contas inválidas: %+v", es)
	}
	if HandlerDoGatilho("palavra_chave") != HandlerMensagem || HandlerDoGatilho("sem_resposta") != HandlerEvento ||
		HandlerDoGatilho("agendamento") != HandlerAgendar || HandlerDoGatilho("manual") != HandlerExecutar {
		t.Fatal("mapa gatilho → handler")
	}
}

func TestNormalizarTexto(t *testing.T) {
	casos := map[string]string{
		"  Orçamento!!  ":         "orcamento",
		"QUANTO   custa o PREÇO?": "quanto custa o preco",
		"¿Ação?":                  "acao",
		"2)":                      "2",
	}
	for e, s := range casos {
		if g := NormalizarTexto(e); g != s {
			t.Errorf("%q → %q (esperado %q)", e, g, s)
		}
	}
	if p := Palavras("Qual o preço, por favor?"); strings.Join(p, "|") != "qual|o|preco|por|favor" {
		t.Fatalf("palavras: %v", p)
	}
}
