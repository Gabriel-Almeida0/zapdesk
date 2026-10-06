package integracao

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// T110 — chatbots ponta a ponta com WhatsApp falso.

func definicaoBot(boasVindas string) map[string]any {
	var d map[string]any
	json.Unmarshal([]byte(`{ "versao": 1, "inicio": "n1", "nao_entendi": "Não entendi. Responda com uma das opções.",
	  "max_tentativas": 3, "inatividade_min": 30,
	  "nos": [
	    { "id": "n1", "tipo": "inicio", "proximo": "n2" },
	    { "id": "n2", "tipo": "mensagem", "texto": "`+boasVindas+`", "proximo": "n3" },
	    { "id": "n3", "tipo": "menu", "texto": "Como posso ajudar?",
	      "opcoes": [ { "rotulo": "Preços", "valores": ["preco"], "proximo": "n4" },
	                  { "rotulo": "Falar com vendedor", "valores": [], "proximo": "n8" } ], "mostrar_numeros": true },
	    { "id": "n4", "tipo": "pergunta", "texto": "Qual seu e-mail?", "variavel": "email",
	      "validacao": { "tipo": "email", "mensagem_erro": "E-mail inválido. Tente de novo." }, "proximo": "n6" },
	    { "id": "n6", "tipo": "acao", "acao": { "tipo": "atualizar_campo_lead", "campo": "email", "valor": "{email}" }, "proximo": "n9" },
	    { "id": "n8", "tipo": "humano", "mensagem": "Vou chamar um atendente." },
	    { "id": "n9", "tipo": "fim", "mensagem": "Obrigado! Enviaremos para {email}." }
	  ] }`), &d)
	return d
}

type SessaoT struct {
	ID         string            `json:"id"`
	Estado     string            `json:"estado"`
	NoAtual    string            `json:"no_atual"`
	Variaveis  map[string]string `json:"variaveis"`
	Motivo     *string           `json:"motivo"`
	Versao     int               `json:"versao"`
	ConversaID string            `json:"conversa_id"`
}

func (m *Motor) sessoes(automacaoID string) []SessaoT {
	m.t.Helper()
	var p Pagina[SessaoT]
	m.json("GET", "/v1/automacoes/"+automacaoID+"/sessoes", nil, 200, &p)
	return p.Itens
}

func textos(l []map[string]any) []string {
	var r []string
	for _, e := range l {
		if t, ok := e["texto"].(string); ok {
			r = append(r, t)
		}
	}
	return r
}

func TestChatbotPalavraChaveSessaoEFim(t *testing.T) {
	m := novoMotor(t)
	c := m.contaConectada("+5511900000020")
	bot := m.criarAutomacao(map[string]any{"tipo": "chatbot", "nome": "Atendimento", "gatilhos": []map[string]any{{"tipo": "palavra_chave", "palavras": []string{"oi"}}},
		"definicao": definicaoBot("Olá, {nome}!")}, true)
	outro := m.criarAutomacao(fluxo("Eco", []map[string]any{{"tipo": "mensagem_recebida"}}, map[string]any{"tipo": "notificar", "titulo": "t", "texto": "x"}), true)
	tel := "+5511933331111"
	m.injetar(c.ID, tel, "Oi!")
	env := textos(m.enviadasPara(tel))
	if len(env) != 2 || env[0] != "Olá, Contato 1111!" || env[1] != "Como posso ajudar?\n1 - Preços\n2 - Falar com vendedor" {
		t.Fatalf("início: %q", env)
	}
	ss := m.sessoes(bot.ID)
	if len(ss) != 1 || ss[0].Estado != "ativa" || ss[0].NoAtual != "n3" {
		t.Fatalf("sessão: %+v", ss)
	}
	var conv Conversa
	for _, cv := range m.conversas(c.ID, "") {
		if cv.Telefone != nil && *cv.Telefone == tel {
			conv = cv
		}
	}
	var est map[string]any
	m.json("GET", "/v1/conversas/"+conv.ID+"/automacoes", nil, 200, &est)
	if est["sessao"] == nil {
		t.Fatalf("estado da conversa sem sessão: %v", est)
	}
	execOutro := len(m.execucoesDe(outro.ID))

	// Sessão ativa consome as mensagens: a outra automação não dispara.
	m.injetar(c.ID, tel, "quanto custa?") // inválida
	m.injetar(c.ID, tel, "1")
	m.injetar(c.ID, tel, "email-ruim")
	m.injetar(c.ID, tel, "ana@exemplo.com")
	env = textos(m.enviadasPara(tel))
	quer := []string{"Não entendi. Responda com uma das opções.", "Como posso ajudar?\n1 - Preços\n2 - Falar com vendedor",
		"Qual seu e-mail?", "E-mail inválido. Tente de novo.", "Obrigado! Enviaremos para ana@exemplo.com."}
	if len(env) != 2+len(quer) || strings.Join(env[2:], "|") != strings.Join(quer, "|") {
		t.Fatalf("conversa com o bot: %q", env)
	}
	if n := len(m.execucoesDe(outro.ID)); n != execOutro {
		t.Fatalf("outra automação disparou durante a sessão: %d → %d", execOutro, n)
	}
	ss = m.sessoes(bot.ID)
	if ss[0].Estado != "concluida" || ss[0].Variaveis["email"] != "ana@exemplo.com" {
		t.Fatalf("fim: %+v", ss[0])
	}
	var ct Contato
	m.json("GET", "/v1/contatos/"+*conv.ContatoID, nil, 200, &ct)
	var lead Lead
	m.json("GET", "/v1/leads/"+ct.Lead.ID, nil, 200, &lead)
	if lead.Campos["email"] != "ana@exemplo.com" {
		t.Fatalf("ação do bot: %+v", lead)
	}
	// Depois do fim, mensagens voltam às automações normais.
	m.injetar(c.ID, tel, "valeu")
	if len(m.execucoesDe(outro.ID)) != execOutro+1 {
		t.Fatal("depois da sessão a outra automação deveria disparar")
	}
}

func TestChatbotHumanoAssumirInatividadeVersaoEAntiLoop(t *testing.T) {
	m := novoMotor(t)
	c := m.contaConectada("+5511900000021")
	bot := m.criarAutomacao(map[string]any{"tipo": "chatbot", "nome": "Primeira", "gatilhos": []map[string]any{{"tipo": "mensagem_recebida", "primeira_mensagem": true}},
		"definicao": definicaoBot("Bem-vindo!")}, true)
	// Primeira mensagem inicia; segunda (de outro contato) também.
	m.injetar(c.ID, "+5511922220001", "boa tarde")
	m.injetar(c.ID, "+5511922220002", "boa tarde")
	if ss := m.sessoes(bot.ID); len(ss) != 2 {
		t.Fatalf("sessões: %+v", ss)
	}
	convDe := func(tel string) string {
		for _, cv := range m.conversas(c.ID, "") {
			if cv.Telefone != nil && *cv.Telefone == tel {
				return cv.ID
			}
		}
		t.Fatalf("conversa de %s", tel)
		return ""
	}
	// Resposta manual encerra como humano.
	m.json("POST", "/v1/conversas/"+convDe("+5511922220001")+"/mensagens", map[string]any{"texto": "Oi, sou a Ana."}, 202, nil)
	m.ociosoAut()
	// "Assumir" encerra como humano.
	m.json("POST", "/v1/conversas/"+convDe("+5511922220002")+"/pausa", map[string]any{"motivo": "humano"}, 200, nil)
	for _, s := range m.sessoes(bot.ID) {
		if s.Estado != "humano" {
			t.Fatalf("deveria estar em humano: %+v", s)
		}
	}
	// Opção "Falar com vendedor" → nó humano → pausa + notificação.
	marca := m.marca()
	m.injetar(c.ID, "+5511922220003", "olá")
	m.injetar(c.ID, "+5511922220003", "2")
	ev := m.esperarEvento(marca, "notificacao", func(e Evento) bool { return strings.Contains(string(e.Dados), `"tipo":"humano"`) })
	_ = ev
	var est map[string]any
	m.json("GET", "/v1/conversas/"+convDe("+5511922220003")+"/automacoes", nil, 200, &est)
	if p, _ := est["pausa"].(map[string]any); p == nil || p["motivo"] != "humano" {
		t.Fatalf("nó humano deveria pausar: %v", est)
	}

	// Inatividade de 30 min → expirada.
	m.injetar(c.ID, "+5511922220004", "olá")
	m.avancarAut(31 * time.Minute)
	var achou bool
	for _, s := range m.sessoes(bot.ID) {
		if s.ConversaID == convDe("+5511922220004") {
			achou = s.Estado == "expirada"
		}
	}
	if !achou {
		t.Fatalf("inatividade: %+v", m.sessoes(bot.ID))
	}

	// Edição com sessão ativa mantém a versão congelada.
	m.injetar(c.ID, "+5511922220005", "olá")
	nova := definicaoBot("Bem-vindo!")
	for _, n := range nova["nos"].([]any) {
		no := n.(map[string]any)
		if no["id"] == "n4" {
			no["texto"] = "Me passa seu e-mail?"
		}
	}
	var a Automacao
	m.json("PATCH", "/v1/automacoes/"+bot.ID, map[string]any{"definicao": nova}, 200, &a)
	if a.Versao != 2 {
		t.Fatalf("versão: %d", a.Versao)
	}
	m.injetar(c.ID, "+5511922220005", "1")
	env := textos(m.enviadasPara("+5511922220005"))
	if env[len(env)-1] != "Qual seu e-mail?" {
		t.Fatalf("sessão deveria usar a definição congelada: %q", env)
	}

	// Anti-loop: com limite global baixo, o envio barrado encerra a sessão como abortada.
	m.json("PATCH", "/v1/automacoes/configuracao", map[string]any{"anti_loop_mensagens": 3}, 200, nil)
	m.injetar(c.ID, "+5511922220006", "olá") // 2 mensagens do bot
	m.injetar(c.ID, "+5511922220006", "x")   // "não entendi" (3ª) + menu barrado
	vistas := 0
	for _, s := range m.sessoes(bot.ID) {
		if s.ConversaID == convDe("+5511922220006") {
			vistas++
			if s.Estado != "abortada" || s.Motivo == nil || *s.Motivo != "anti-loop" {
				t.Fatalf("anti-loop: %+v", s)
			}
		}
	}
	if vistas != 1 {
		t.Fatalf("sessão do anti-loop não encontrada (%d)", vistas)
	}
}

func TestChatbotSimuladorSemEscrita(t *testing.T) {
	m := novoMotor(t)
	m.contaConectada("+5511900000022")
	bot := m.criarAutomacao(map[string]any{"tipo": "chatbot", "nome": "Sim", "gatilhos": []map[string]any{{"tipo": "palavra_chave", "palavras": []string{"oi"}}},
		"definicao": definicaoBot("Olá, {nome}!")}, false)
	var ini struct {
		SimulacaoID string            `json:"simulacao_id"`
		Saidas      []map[string]any  `json:"saidas"`
		NoAtual     *string           `json:"no_atual"`
		Estado      string            `json:"estado"`
		Variaveis   map[string]string `json:"variaveis"`
		Acoes       []map[string]any  `json:"acoes"`
	}
	m.json("POST", "/v1/automacoes/"+bot.ID+"/simulador", map[string]any{}, 201, &ini)
	if ini.SimulacaoID == "" || len(ini.Saidas) != 2 || ini.Saidas[0]["texto"] != "Olá, Contato de teste!" || *ini.NoAtual != "n3" || ini.Estado != "ativa" {
		t.Fatalf("início simulado: %+v", ini)
	}
	var r = ini
	for _, txt := range []string{"1", "ana@exemplo.com"} {
		m.json("POST", "/v1/simulador/"+ini.SimulacaoID+"/mensagens", map[string]any{"texto": txt}, 200, &r)
	}
	if r.Estado != "concluida" || r.Variaveis["email"] != "ana@exemplo.com" || len(r.Acoes) == 0 {
		t.Fatalf("rodada simulada: %+v", r)
	}
	tem := false
	for _, s := range r.Saidas {
		if s["tipo"] == "acao" {
			tem = true
		}
	}
	if !tem {
		t.Fatalf("saída de ação: %+v", r.Saidas)
	}
	m.erro("POST", "/v1/simulador/"+ini.SimulacaoID+"/mensagens", map[string]any{"texto": "de novo"}, 409, "transicao_invalida")
	// Definição com erro no simulador.
	ruim := definicaoBot("x")
	ruim["inicio"] = "nao-existe"
	m.erro("POST", "/v1/automacoes/"+bot.ID+"/simulador", map[string]any{"definicao": ruim}, 422, "definicao_invalida")
	// Nada gravado além das execuções "simulacao".
	var env []any
	m.json("GET", "/v1/falso/enviadas", nil, 200, &env)
	if len(env) != 0 || len(m.sessoes(bot.ID)) != 0 {
		t.Fatal("simulador escreveu")
	}
	for _, e := range m.execucoesDe(bot.ID) {
		if e.Estado != "simulacao" {
			t.Fatalf("execução não simulada: %+v", e)
		}
	}
	if st, _ := m.req("DELETE", "/v1/simulador/"+ini.SimulacaoID, nil); st != 204 {
		t.Fatal("encerrar simulação")
	}
	m.erro("POST", "/v1/automacoes/"+bot.ID+"/testar", map[string]any{}, 422, "validacao")
}
