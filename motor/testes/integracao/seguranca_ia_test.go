package integracao

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"zapdesk/motor/internal/logs"
)

// T129 — segurança das automações: log sem mensagens nem ctx.log, segredos só por nome, runner
// sem token/chave e nenhuma conexão com a Claude API sem chave ou sem automação ativa.

func TestSegurancaAutomacoesLogSegredosERunner(t *testing.T) {
	pasta := t.TempDir()
	l, err := logs.Novo(logs.Opcoes{PastaDados: pasta, Nivel: "debug", SemStderr: true})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Fechar()
	m := novoMotor(t, comRunner(t), func(o *opcoesMotor) { o.pasta = pasta; o.log = &l.Logger })
	c := m.contaConectada("+5511900000030")
	m.json("POST", "/v1/falso/segredos", map[string]any{"valores": map[string]string{"ANTHROPIC_API_KEY": "sk-ant-valor-secreto-999", "OUTRA_CHAVE": "valor-outra-777"}}, 200, nil)

	a := m.criarIA("Espiã", "em_branco")
	m.escrever(a.ID, "automacao.json", manifesto("Espiã", `[{"tipo":"mensagem_recebida"},{"tipo":"manual"}]`, `["enviar"]`, "10"))
	m.escrever(a.ID, "index.ts", `import { definirAutomacao } from '@zapdesk/automacao';
export default definirAutomacao({
  async aoReceberMensagem(ctx, msg) {
    ctx.log.info('MARCADOR-CTX-LOG-555', msg.texto);
    console.log('MARCADOR-CONSOLE-666');
    await ctx.responder('ok');
  },
  aoExecutar() {
    return { env: Object.keys(process.env), fetch: typeof (globalThis as any).fetch };
  },
});
`)
	m.json("POST", "/v1/automacoes/"+a.ID+"/ativar", map[string]any{}, 200, nil)
	m.injetar(c.ID, "+5511955554444", "texto-secreto-da-mensagem-321")
	ex := m.execucoesDe(a.ID)
	var det Execucao
	m.json("GET", "/v1/execucoes/"+ex[0].ID, nil, 200, &det)
	if det.Estado != "ok" || !strings.Contains(det.Log, "MARCADOR-CTX-LOG-555") || !strings.Contains(det.Log, "MARCADOR-CONSOLE-666") {
		t.Fatalf("ctx.log deveria ficar na execução: %+v", det)
	}

	// Ambiente do runner: sem token, sem chave, sem fetch global.
	var e Execucao
	m.json("POST", "/v1/automacoes/"+a.ID+"/executar", map[string]any{}, 202, &e)
	m.ociosoAut()
	m.json("GET", "/v1/execucoes/"+e.ID, nil, 200, &det)
	ret := string(det.Retorno)
	if det.Estado != "ok" || strings.Contains(ret, "ZAPDESK") || strings.Contains(ret, "ANTHROPIC") || !strings.Contains(ret, `"fetch":"undefined"`) {
		t.Fatalf("ambiente do runner: %s %+v", ret, det)
	}

	// Segredos: só nomes na API.
	_, corpo := m.req("GET", "/v1/segredos", nil)
	if strings.Contains(string(corpo), "sk-ant-valor-secreto-999") || strings.Contains(string(corpo), "valor-outra-777") {
		t.Fatalf("GET /v1/segredos vazou valores: %s", corpo)
	}

	m.parar()
	dados, err := os.ReadFile(logs.CaminhoLog(pasta))
	if err != nil {
		t.Fatal(err)
	}
	texto := string(dados)
	for _, s := range []string{"texto-secreto-da-mensagem-321", "MARCADOR-CTX-LOG-555", "MARCADOR-CONSOLE-666", "sk-ant-valor-secreto-999", "valor-outra-777"} {
		if strings.Contains(texto, s) {
			t.Fatalf("log do motor vazou %q", s)
		}
	}
}

func TestSegurancaSemConexaoClaudeSemChaveOuSemAtiva(t *testing.T) {
	var chamadas atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chamadas.Add(1)
		w.Header().Set("request-id", "req_teste")
		w.Write([]byte(`{"content":[{"type":"text","text":"resposta real"}],"model":"claude-sonnet-5","stop_reason":"end_turn","usage":{"input_tokens":3,"output_tokens":2}}`))
	}))
	defer srv.Close()
	m := novoMotor(t, comRunner(t), func(o *opcoesMotor) { o.iaReal = true; o.urlClaude = srv.URL })
	c := m.contaConectada("+5511900000031")

	a := m.criarIA("Responder", "responder_historico")
	// Inativa e sem chave: nada.
	m.injetar(c.ID, "+5511955553333", "Olá")
	m.erro("POST", "/v1/ia/testar-chave", map[string]any{}, 409, "ia_nao_configurada")
	// Ativa e sem chave: executa, falha sem abrir conexão.
	m.json("POST", "/v1/automacoes/"+a.ID+"/ativar", map[string]any{}, 200, nil)
	m.injetar(c.ID, "+5511955553333", "Olá de novo")
	if n := chamadas.Load(); n != 0 {
		t.Fatalf("conexões à Claude API sem chave: %d", n)
	}
	// Com chave mas desativada: nada.
	m.json("POST", "/v1/automacoes/"+a.ID+"/desativar", map[string]any{}, 200, nil)
	m.json("POST", "/v1/falso/segredos", map[string]any{"valores": map[string]string{"ANTHROPIC_API_KEY": "sk-ant-x"}}, 200, nil)
	m.injetar(c.ID, "+5511955553333", "E agora?")
	if n := chamadas.Load(); n != 0 {
		t.Fatalf("conexões à Claude API com automação inativa: %d", n)
	}
	// Com chave e ativa: exatamente a chamada do helper (prova que o servidor falso está ligado).
	m.json("POST", "/v1/automacoes/"+a.ID+"/ativar", map[string]any{}, 200, nil)
	m.injetar(c.ID, "+5511955553333", "Quanto custa?")
	if n := chamadas.Load(); n != 1 {
		t.Fatalf("esperava 1 chamada com chave e automação ativa: %d", n)
	}
	if env := m.enviadasPara("+5511955553333"); len(env) != 1 || env[0]["texto"] != "resposta real" {
		t.Fatalf("resposta: %v", env)
	}
}
