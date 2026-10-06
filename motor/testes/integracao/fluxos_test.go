package integracao

import (
	"strings"
	"testing"
	"time"
)

// T073 — fluxos ponta a ponta com WhatsApp falso e relógio controlável.

func (m *Motor) etiqueta(nome string) Etiqueta {
	m.t.Helper()
	var e Etiqueta
	m.json("POST", "/v1/etiquetas", map[string]any{"nome": nome, "cor": "#FF8800"}, 201, &e)
	return e
}

func (m *Motor) funilVendas() FunilT {
	m.t.Helper()
	var f FunilT
	m.json("POST", "/v1/funis", map[string]any{"nome": "Vendas", "etapas": []map[string]any{{"nome": "Novo"}, {"nome": "Qualificando"}, {"nome": "Proposta"}}}, 201, &f)
	return f
}

func fluxo(nome string, gatilhos []map[string]any, acoes ...map[string]any) map[string]any {
	return map[string]any{"tipo": "fluxo", "nome": nome, "gatilhos": gatilhos,
		"definicao": map[string]any{"versao": 1, "condicoes": nil, "acoes": acoes}}
}

func TestFluxoRespondeuDisparoEtiquetaEEtapa(t *testing.T) {
	m := novoMotor(t)
	c := m.contaConectada("+5511900000001")
	quente := m.etiqueta("Quente")
	f := m.funilVendas()
	a := m.criarAutomacao(fluxo("Respondeu → quente", []map[string]any{{"tipo": "disparo_respondeu"}},
		map[string]any{"tipo": "adicionar_etiqueta", "etiqueta_id": quente.ID},
		map[string]any{"tipo": "mover_etapa", "funil_id": f.ID, "etapa_id": f.Etapas[1].ID}), true)
	if !a.Ativa || a.Versao != 1 {
		t.Fatalf("ativa: %+v", a)
	}
	var rel Relatorio
	m.json("POST", "/v1/leads/importar", map[string]any{"leads": []map[string]any{{"telefone": "11 91234-0001", "nome": "Ana"}}}, 200, &rel)
	var d Disparo
	m.json("POST", "/v1/disparos", map[string]any{"conta_id": c.ID, "mensagem": "Oi {nome}!", "iniciar": true,
		"destinatarios": map[string]any{"lead_ids": rel.LeadIDs}, "ritmo": map[string]any{"intervalo_min_s": 30, "intervalo_max_s": 60}}, 201, &d)
	m.ocioso()
	m.injetar(c.ID, "+5511912340001", "Tenho interesse!")

	ex := m.execucoesDe(a.ID)
	if len(ex) != 1 || ex[0].Estado != "ok" || ex[0].Gatilho.Tipo != "disparo_respondeu" || ex[0].Gatilho.Dados["disparo_id"] != d.ID {
		t.Fatalf("execução: %+v", ex)
	}
	conv := m.conversas(c.ID, "")[0]
	if len(conv.Etiquetas) != 1 || conv.Etiquetas[0].ID != quente.ID {
		t.Fatalf("etiqueta: %+v", conv.Etiquetas)
	}
	var cards Pagina[CardT]
	m.json("GET", "/v1/funis/"+f.ID+"/cards?etapa_id="+f.Etapas[1].ID, nil, 200, &cards)
	if len(cards.Itens) != 1 || cards.Itens[0].LeadID != rel.LeadIDs[0] {
		t.Fatalf("card: %+v", cards)
	}
	var h Pagina[map[string]any]
	m.json("GET", "/v1/funis/"+f.ID+"/historico", nil, 200, &h)
	if h.Itens[0]["origem"] != "automacao" || h.Itens[0]["automacao_id"] != a.ID || h.Itens[0]["execucao_id"] != ex[0].ID {
		t.Fatalf("histórico com origem automação: %v", h.Itens[0])
	}
}

func TestFluxoSemRespostaComReinicioEExpiracao(t *testing.T) {
	m := novoMotor(t)
	c := m.contaConectada("+5511900000002")
	a := m.criarAutomacao(fluxo("Sem resposta 2 h", []map[string]any{{"tipo": "sem_resposta", "apos_s": 7200}},
		map[string]any{"tipo": "enviar_texto", "texto": "Oi {primeiro_nome}, ainda tem interesse?"}), true)
	m.injetar(c.ID, "+5511955550001", "Quanto custa?")
	conv := m.conversas(c.ID, "")[0]
	m.json("POST", "/v1/conversas/"+conv.ID+"/mensagens", map[string]any{"texto": "Custa R$ 50."}, 202, nil)
	m.ociosoAut()
	antes := len(m.enviadasPara("+5511955550001"))

	// Motor reiniciado no meio da espera: a espera é persistida e dispara uma única vez.
	m.avancarAut(time.Hour)
	m.reiniciar()
	m.aguardarConta(c.ID)
	m.avancarAut(time.Hour + time.Minute)
	m.processarEsperas()
	depois := m.enviadasPara("+5511955550001")
	if len(depois) != antes+1 || !strings.Contains(depois[len(depois)-1]["texto"].(string), "ainda tem interesse?") {
		t.Fatalf("follow-up: %d → %v", antes, depois)
	}
	m.avancarAut(3 * time.Hour)
	if len(m.enviadasPara("+5511955550001")) != antes+1 {
		t.Fatal("follow-up repetido")
	}
	ex := m.execucoesDe(a.ID)
	if len(ex) != 1 || ex[0].Estado != "ok" || ex[0].Gatilho.Tipo != "sem_resposta" {
		t.Fatalf("execução: %+v", ex)
	}

	// Motor parado 30 h: a espera vence há mais de 24 h → abortada "expirada", nada enviado.
	m.json("POST", "/v1/conversas/"+conv.ID+"/mensagens", map[string]any{"texto": "Posso ajudar?"}, 202, nil)
	m.ociosoAut()
	n := len(m.enviadasPara("+5511955550001"))
	m.reiniciar()
	m.aguardarConta(c.ID)
	m.avancarAut(30 * time.Hour)
	m.processarEsperas()
	if len(m.enviadasPara("+5511955550001")) != n {
		t.Fatal("espera vencida há 30 h não deveria enviar")
	}
	ex = m.execucoesDe(a.ID)
	if len(ex) != 2 || ex[0].Estado != "abortada" || ex[0].Motivo == nil || *ex[0].Motivo != "expirada" {
		t.Fatalf("expirada: %+v", ex[0])
	}
	// Resposta do contato cancela a espera.
	m.json("POST", "/v1/conversas/"+conv.ID+"/mensagens", map[string]any{"texto": "E aí?"}, 202, nil)
	m.ociosoAut()
	m.injetar(c.ID, "+5511955550001", "Respondi")
	m.avancarAut(3 * time.Hour)
	if len(m.execucoesDe(a.ID)) != 2 {
		t.Fatal("resposta deveria cancelar o sem resposta")
	}
}

func (m *Motor) aguardarConta(id string) {
	m.t.Helper()
	fim := time.Now().Add(10 * time.Second)
	for {
		var c Conta
		m.json("GET", "/v1/contas/"+id, nil, 200, &c)
		if c.Estado == "conectada" {
			return
		}
		if time.Now().After(fim) {
			m.t.Fatalf("conta não reconectou: %+v", c)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestFluxoAntiLoopHumanoEGrupos(t *testing.T) {
	m := novoMotor(t)
	c := m.contaConectada("+5511900000003")
	a := m.criarAutomacao(fluxo("Eco", []map[string]any{{"tipo": "mensagem_recebida"}},
		map[string]any{"tipo": "enviar_texto", "texto": "Recebido: {ultima_mensagem}"}), true)
	marca := m.marca()
	for i := 0; i < 12; i++ {
		m.injetar(c.ID, "+5511966660001", "msg "+string(rune('a'+i)))
	}
	if n := len(m.enviadasPara("+5511966660001")); n != 10 {
		t.Fatalf("anti-loop: %d mensagens automáticas (esperado 10)", n)
	}
	ex := m.execucoesDe(a.ID)
	bloqueadas := 0
	for _, e := range ex {
		if len(e.Acoes) == 1 && e.Acoes[0].Resultado == "bloqueada" {
			bloqueadas++
		}
	}
	if bloqueadas != 1 {
		// a 11ª é bloqueada (e cria a pausa); a 12ª nem dispara (conversa pausada)
		t.Fatalf("bloqueadas: %d de %d", bloqueadas, len(ex))
	}
	not := m.esperarEvento(marca, "notificacao", nil)
	if !strings.Contains(string(not.Dados), `"tipo":"anti_loop"`) {
		t.Fatalf("notificação: %s", not.Dados)
	}
	conv := m.conversas(c.ID, "")[0]
	var est map[string]any
	m.json("GET", "/v1/conversas/"+conv.ID+"/automacoes", nil, 200, &est)
	if p, _ := est["pausa"].(map[string]any); p == nil || p["motivo"] != "anti_loop" {
		t.Fatalf("pausa anti-loop: %v", est)
	}
	// Mensagens automáticas levam automacao_id.
	msgs := m.mensagens(conv.ID)
	auto := 0
	for _, x := range msgs {
		if x.DeMim && strings.HasPrefix(ptr(x.Texto), "Recebido:") {
			auto++
		}
	}
	if auto != 10 {
		t.Fatalf("mensagens automáticas: %d", auto)
	}

	// Resposta manual pelo celular (de_mim, wa_id inédito) → pausa humano.
	m.injetar(c.ID, "+5511966660002", "oi")
	m.receber(c.ID, map[string]any{"de": "+5511966660002", "texto": "Oi! Aqui é a Ana.", "de_mim": true})
	m.ociosoAut()
	conv2 := m.conversas(c.ID, "?busca=66660002")
	if len(conv2) == 0 {
		conv2 = m.conversas(c.ID, "")
	}
	var alvo string
	for _, cv := range m.conversas(c.ID, "") {
		if cv.Telefone != nil && *cv.Telefone == "+5511966660002" {
			alvo = cv.ID
		}
	}
	m.json("GET", "/v1/conversas/"+alvo+"/automacoes", nil, 200, &est)
	if p, _ := est["pausa"].(map[string]any); p == nil || p["motivo"] != "humano" {
		t.Fatalf("pausa humano pelo celular: %v", est)
	}
	antes := len(m.execucoesDe(a.ID))
	m.injetar(c.ID, "+5511966660002", "e agora?")
	if len(m.execucoesDe(a.ID)) != antes {
		t.Fatal("conversa em atendimento humano não deveria disparar")
	}

	// Grupo sem incluir_grupos: ignorado.
	m.receber(c.ID, map[string]any{"de": "+5511966660003", "texto": "oi grupo", "grupo_jid": "120363000000000001@g.us", "grupo_nome": "Turma"})
	m.ociosoAut()
	if len(m.execucoesDe(a.ID)) != antes {
		t.Fatal("grupo não deveria disparar")
	}
}

func TestFluxoAgendamentoManualCadeiaEPrimeiroContato(t *testing.T) {
	m := novoMotor(t)
	c := m.contaConectada("+5511900000004")
	// Agendamento por intervalo.
	ag := m.criarAutomacao(fluxo("De hora em hora", []map[string]any{{"tipo": "agendamento", "intervalo_s": 3600}},
		map[string]any{"tipo": "notificar", "titulo": "Lembrete", "texto": "Passou 1 h"}), true)
	m.avancarAut(time.Hour)
	if ex := m.execucoesDe(ag.ID); len(ex) != 1 || ex[0].Estado != "ok" || ex[0].Gatilho.Tipo != "agendamento" {
		t.Fatalf("agendamento: %+v", ex)
	}
	m.json("POST", "/v1/automacoes/"+ag.ID+"/desativar", map[string]any{}, 200, nil)
	m.avancarAut(2 * time.Hour)
	if len(m.execucoesDe(ag.ID)) != 1 {
		t.Fatal("desativada não deveria rodar")
	}

	// Executar manual (202 na_fila → ok).
	m.injetar(c.ID, "+5511977780001", "oi")
	conv := m.conversas(c.ID, "")[0]
	man := m.criarAutomacao(fluxo("Manual", []map[string]any{{"tipo": "manual"}},
		map[string]any{"tipo": "enviar_texto", "texto": "Olá {nome}!"}), false)
	var e Execucao
	m.json("POST", "/v1/automacoes/"+man.ID+"/executar", map[string]any{"conversa_id": conv.ID}, 202, &e)
	if e.Estado != "na_fila" || e.Origem != "manual_app" {
		t.Fatalf("executar: %+v", e)
	}
	m.ociosoAut()
	m.json("GET", "/v1/execucoes/"+e.ID, nil, 200, &e)
	if e.Estado != "ok" || len(e.Acoes) != 1 || e.Acoes[0].Resultado != "ok" {
		t.Fatalf("execução manual: %+v", e)
	}

	// Cadeia etiqueta ↔ etapa interrompida.
	x := m.etiqueta("X")
	f := m.funilVendas()
	aA := m.criarAutomacao(fluxo("A", []map[string]any{{"tipo": "etiqueta", "evento": "adicionada", "etiqueta_id": x.ID}},
		map[string]any{"tipo": "mover_etapa", "funil_id": f.ID, "etapa_id": f.Etapas[0].ID}), true)
	aB := m.criarAutomacao(fluxo("B", []map[string]any{{"tipo": "entrou_etapa", "funil_id": f.ID, "etapa_id": f.Etapas[0].ID}},
		map[string]any{"tipo": "remover_etiqueta", "etiqueta_id": x.ID},
		map[string]any{"tipo": "adicionar_etiqueta", "etiqueta_id": x.ID}), true)
	m.json("PUT", "/v1/contatos/"+*conv.ContatoID+"/etiquetas", map[string]any{"etiqueta_ids": []string{x.ID}}, 200, nil)
	m.ociosoAut()
	if la, lb := len(m.execucoesDe(aA.ID)), len(m.execucoesDe(aB.ID)); la != 1 || lb != 1 {
		t.Fatalf("cadeia: A=%d B=%d", la, lb)
	}

	// Primeiro contato acima do limite por hora é bloqueado.
	m.json("PATCH", "/v1/automacoes/configuracao", map[string]any{"primeiros_contatos_hora": 2}, 200, nil)
	boas := m.criarAutomacao(map[string]any{"tipo": "fluxo", "nome": "Boas-vindas", "conta_envio_id": c.ID,
		"gatilhos":  []map[string]any{{"tipo": "lead_importado"}},
		"definicao": map[string]any{"versao": 1, "acoes": []map[string]any{{"tipo": "enviar_texto", "texto": "Olá! Obrigado pelo contato."}}}}, true)
	m.json("POST", "/v1/leads/importar", map[string]any{"leads": []map[string]any{{"telefone": "11 93333-0001"}, {"telefone": "11 93333-0002"}, {"telefone": "11 93333-0003"}}}, 200, nil)
	m.ociosoAut()
	ex := m.execucoesDe(boas.ID)
	res := map[string]int{}
	for _, e := range ex {
		res[e.Acoes[0].Resultado]++
	}
	if len(ex) != 3 || res["ok"] != 2 || res["bloqueada"] != 1 {
		t.Fatalf("primeiro contato: %v %+v", res, ex)
	}
}

func TestAutomacoesCRUDValidacaoEAvisos(t *testing.T) {
	m := novoMotor(t)
	m.contaConectada("+5511900000005")
	// Validar sem gravar.
	var v struct {
		Erros  []ErroDef `json:"erros"`
		Avisos []ErroDef `json:"avisos"`
	}
	m.json("POST", "/v1/automacoes/validar", fluxo("X", []map[string]any{{"tipo": "manual"}}, map[string]any{"tipo": "aguardar", "duracao_s": 10}), 200, &v)
	if len(v.Erros) != 1 || v.Erros[0].Caminho != "definicao.acoes[0].duracao_s" || v.Erros[0].AcaoID == nil || *v.Erros[0].AcaoID != "a1" {
		t.Fatalf("validar: %+v", v)
	}
	m.json("POST", "/v1/automacoes/validar", fluxo("X", []map[string]any{{"tipo": "manual"}}, map[string]any{"tipo": "adicionar_etiqueta", "etiqueta_id": "sumiu"}), 200, &v)
	if len(v.Erros) != 0 || len(v.Avisos) != 1 || v.Avisos[0].Caminho != "definicao.acoes[0].etiqueta_id" {
		t.Fatalf("avisos: %+v", v)
	}
	e := m.erro("POST", "/v1/automacoes", fluxo("X", []map[string]any{{"tipo": "agendamento", "cron": "abc"}}, map[string]any{"tipo": "notificar", "titulo": "t", "texto": "x"}), 422, "definicao_invalida")
	if erros := e["detalhes"].(map[string]any)["erros"].([]any); len(erros) != 1 {
		t.Fatalf("definicao_invalida: %v", e)
	}
	m.erro("POST", "/v1/automacoes", map[string]any{"tipo": "fluxo", "nome": ""}, 422, "validacao")
	m.erro("POST", "/v1/automacoes", map[string]any{"tipo": "ia", "nome": "x"}, 422, "validacao")

	// Criar inativo; editar muda versão só com definição; referência quebrada vira aviso.
	a := m.criarAutomacao(fluxo("Y", []map[string]any{{"tipo": "manual"}}, map[string]any{"tipo": "adicionar_etiqueta", "etiqueta_id": "sumiu"}), false)
	if a.Ativa || a.Versao != 1 || len(a.Avisos) != 1 {
		t.Fatalf("criada: %+v", a)
	}
	m.json("PATCH", "/v1/automacoes/"+a.ID, map[string]any{"nome": "Y2", "prioridade": 5}, 200, &a)
	if a.Versao != 1 || a.Nome != "Y2" {
		t.Fatalf("patch sem definição: %+v", a)
	}
	m.json("PATCH", "/v1/automacoes/"+a.ID, map[string]any{"definicao": map[string]any{"versao": 1, "acoes": []map[string]any{{"tipo": "notificar", "titulo": "a", "texto": "b"}}}}, 200, &a)
	if a.Versao != 2 || len(a.Avisos) != 0 {
		t.Fatalf("patch com definição: %+v", a)
	}
	m.erro("PATCH", "/v1/automacoes/"+a.ID, map[string]any{"tipo": "chatbot"}, 422, "validacao")
	m.erro("PATCH", "/v1/automacoes/"+a.ID, map[string]any{"limites": map[string]any{"anti_loop": map[string]any{"mensagens": 50, "janela_min": 10}}}, 422, "definicao_invalida")
	// Sem gatilho não ativa.
	semG := m.criarAutomacao(fluxo("Z", []map[string]any{}, map[string]any{"tipo": "notificar", "titulo": "a", "texto": "b"}), false)
	m.erro("POST", "/v1/automacoes/"+semG.ID+"/ativar", map[string]any{}, 422, "validacao")
	// Testar: simulação sem alvo usa a conversa fictícia e não envia nada.
	tst := m.criarAutomacao(fluxo("T", []map[string]any{{"tipo": "mensagem_recebida"}},
		map[string]any{"tipo": "enviar_texto", "texto": "Oi {nome}! Você disse: {ultima_mensagem}"},
		map[string]any{"tipo": "aguardar", "duracao_s": 7200}), false)
	var r struct {
		Execucao Execucao `json:"execucao"`
	}
	m.json("POST", "/v1/automacoes/"+tst.ID+"/testar", map[string]any{"mensagem": map[string]any{"texto": "Quanto custa?"}}, 200, &r)
	if r.Execucao.Estado != "simulacao" || !r.Execucao.Simulacao || len(r.Execucao.Acoes) != 2 ||
		!strings.Contains(ptr(r.Execucao.Acoes[0].Detalhe), "Oi Contato de teste! Você disse: Quanto custa?") ||
		!strings.Contains(ptr(r.Execucao.Acoes[1].Detalhe), "Aguardaria 2 h") {
		t.Fatalf("testar: %+v", r.Execucao)
	}
	var env []any
	m.json("GET", "/v1/falso/enviadas", nil, 200, &env)
	if len(env) != 0 {
		t.Fatal("teste enviou mensagem")
	}
	// Listagem e exclusão.
	var lista []Automacao
	m.json("GET", "/v1/automacoes?tipo=fluxo&ativa=false", nil, 200, &lista)
	if len(lista) != 3 {
		t.Fatalf("lista: %d", len(lista))
	}
	marca := m.marca()
	if st, _ := m.req("DELETE", "/v1/automacoes/"+a.ID, nil); st != 204 {
		t.Fatal("excluir")
	}
	m.esperarEvento(marca, "automacao.removida", nil)
	m.erro("GET", "/v1/automacoes/"+a.ID, nil, 404, "nao_encontrado")
}
