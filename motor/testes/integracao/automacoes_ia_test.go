package integracao

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// T092 — automações de IA no motor real com o runner real (node + automacao/runner/dist) e IA
// simulada.

type ArquivoT struct {
	Caminho string `json:"caminho"`
	Hash    string `json:"hash"`
}

func (m *Motor) escrever(id, caminho, conteudo string) {
	m.t.Helper()
	st, b := m.req("PUT", "/v1/automacoes/"+id+"/arquivos/"+caminho, map[string]any{"conteudo": conteudo})
	if st != 200 && st != 201 {
		m.t.Fatalf("escrever %s: %d %s", caminho, st, b)
	}
}

func (m *Motor) criarIA(nome, modelo string) Automacao {
	m.t.Helper()
	var a Automacao
	m.json("POST", "/v1/automacoes/ia", map[string]any{"nome": nome, "modelo": modelo}, 201, &a)
	return a
}

func manifesto(nome string, gatilhos string, permissoes string, extra string) string {
	return `{"versao_manifesto":1,"nome":"` + nome + `","entrada":"index.ts","gatilhos":` + gatilhos +
		`,"permissoes":` + permissoes + `,"segredos":[],"contas":"todas","incluir_grupos":false,"prioridade":100,` +
		`"conta_envio":null,"limites":{"tempo_s":` + extra + `,"memoria_mb":256,"anti_loop":null},"ia":{"modelo":null}}`
}

func filhosDoTeste(t *testing.T) []string {
	out, err := exec.Command("ps", "-axo", "ppid=,pid=,command=").Output()
	if err != nil {
		t.Fatal(err)
	}
	eu := strconv.Itoa(os.Getpid())
	var l []string
	for _, linha := range strings.Split(string(out), "\n") {
		c := strings.Fields(linha)
		if len(c) >= 3 && c[0] == eu && strings.Contains(linha, "zapdesk-runner") {
			l = append(l, linha)
		}
	}
	return l
}

func TestIAModeloTesteAtivacaoEResposta(t *testing.T) {
	m := novoMotor(t, comRunner(t))
	c := m.contaConectada("+5511900000010")
	var sis map[string]any
	m.json("GET", "/v1/sistema", nil, 200, &sis)
	if sis["runner_disponivel"] != true {
		t.Fatalf("runner_disponivel: %v", sis)
	}
	var modelos []map[string]any
	m.json("GET", "/v1/automacoes/modelos", nil, 200, &modelos)
	if len(modelos) != 4 {
		t.Fatalf("modelos: %v", modelos)
	}
	var sdk map[string]any
	m.json("GET", "/v1/automacoes/sdk", nil, 200, &sdk)
	if !strings.Contains(sdk["tipos"].(string), "definirAutomacao") || sdk["esquema_manifesto"] == nil {
		t.Fatal("sdk")
	}

	a := m.criarIA("Responder", "responder_historico")
	if a.Tipo != "ia" || a.IA == nil || !a.IA.CompilacaoOK || a.IA.HashCompilado == nil || a.Ativa {
		t.Fatalf("criada pelo modelo: %+v %+v", a, a.IA)
	}
	var arqs []ArquivoT
	m.json("GET", "/v1/automacoes/"+a.ID+"/arquivos", nil, 200, &arqs)
	if len(arqs) != 3 {
		t.Fatalf("arquivos: %+v", arqs)
	}

	// Teste com IA simulada: estado simulacao, nada enviado.
	m.json("PUT", "/v1/falso/ia", map[string]any{"respostas": []map[string]string{{"contem": "preço", "texto": "O valor é R$ 10"}}}, 200, nil)
	var r struct {
		Execucao Execucao `json:"execucao"`
	}
	m.json("POST", "/v1/automacoes/"+a.ID+"/testar", map[string]any{"mensagem": map[string]any{"texto": "Qual o preço?"}, "ia_simulada": true}, 200, &r)
	if r.Execucao.Estado != "simulacao" || len(r.Execucao.Acoes) == 0 || r.Execucao.Acoes[0].Resultado != "simulada" ||
		!strings.Contains(ptr(r.Execucao.Acoes[0].Detalhe), "O valor é R$ 10") {
		t.Fatalf("teste: %s %+v", ptr(r.Execucao.Erro), r.Execucao)
	}
	var env []any
	m.json("GET", "/v1/falso/enviadas", nil, 200, &env)
	if len(env) != 0 {
		t.Fatal("teste enviou")
	}

	// Erro de compilação bloqueia ativar (e o teste).
	m.escrever(a.ID, "index.ts", "export default definirAutomacao({ aoReceberMensagem( {")
	e := m.erro("POST", "/v1/automacoes/"+a.ID+"/ativar", map[string]any{}, 422, "compilacao_falhou")
	if erros, _ := e["detalhes"].(map[string]any)["erros"].([]any); len(erros) == 0 {
		t.Fatalf("erros de compilação: %v", e)
	}
	m.erro("POST", "/v1/automacoes/"+a.ID+"/testar", map[string]any{"mensagem": map[string]any{"texto": "oi"}}, 422, "compilacao_falhou")

	// Corrige e ativa; mensagem real recebe a resposta da IA.
	var orig ArquivoT
	m.json("GET", "/v1/automacoes/"+a.ID+"/arquivos/prompt.md", nil, 200, &orig)
	m.escrever(a.ID, "index.ts", `import { definirAutomacao } from '@zapdesk/automacao';
import prompt from './prompt.md';
export default definirAutomacao({
  async aoReceberMensagem(ctx, msg) {
    if (!msg.texto || !ctx.conversa) return;
    const historico = await ctx.conversa.historicoParaIA({ limite: 20 });
    const resposta = await ctx.ia.gerar({ sistema: prompt, mensagens: historico });
    await ctx.responder(resposta.texto.trim());
    ctx.log.info('respondeu');
  },
});
`)
	m.json("POST", "/v1/automacoes/"+a.ID+"/ativar", map[string]any{}, 200, &a)
	if !a.Ativa {
		t.Fatal("não ativou")
	}
	m.injetar(c.ID, "+5511988887777", "Qual o preço do plano?")
	env2 := m.enviadasPara("+5511988887777")
	if len(env2) != 1 || env2[0]["texto"] != "O valor é R$ 10" {
		t.Fatalf("resposta da IA: %v", env2)
	}
	ex := m.execucoesDe(a.ID)
	var ultima Execucao
	m.json("GET", "/v1/execucoes/"+ex[0].ID, nil, 200, &ultima)
	if ultima.Estado != "ok" || !strings.Contains(ultima.Log, "respondeu") {
		t.Fatalf("execução: %+v", ultima)
	}
	// O alvo registrado é o telefone (legível), não o id interno da conversa.
	if len(ultima.Acoes) != 1 || ptr(ultima.Acoes[0].Alvo) != "+5511988887777" {
		t.Fatalf("alvo da ação: %+v", ultima.Acoes)
	}
	conv := m.conversas(c.ID, "")[0]
	msgs := m.mensagens(conv.ID)
	if !msgs[0].DeMim || ptr(msgs[0].Texto) != "O valor é R$ 10" {
		t.Fatalf("mensagem automática: %+v", msgs[0])
	}
	var chamadas []map[string]any
	m.json("GET", "/v1/falso/ia/chamadas", nil, 200, &chamadas)
	if len(chamadas) < 2 || !strings.Contains(chamadas[len(chamadas)-1]["mensagens_resumo"].(string), "Qual o preço do plano?") {
		t.Fatalf("chamadas: %v", chamadas)
	}

	// Código alterado com erro: roda a versão anterior e avisa.
	m.escrever(a.ID, "index.ts", "isto não compila (")
	m.injetar(c.ID, "+5511988887777", "E o preço anual?")
	if n := len(m.enviadasPara("+5511988887777")); n != 2 {
		t.Fatalf("versão anterior não respondeu: %d", n)
	}
	m.json("GET", "/v1/automacoes/"+a.ID, nil, 200, &a)
	if a.IA == nil || !a.IA.RodandoVersaoAnterior || a.IA.CompilacaoOK {
		t.Fatalf("rodando_versao_anterior: %+v", a.IA)
	}

	// Encerrar o motor não deixa runner vivo.
	if len(filhosDoTeste(t)) == 0 {
		t.Fatal("deveria haver um runner vivo antes de encerrar")
	}
	m.parar()
	if l := filhosDoTeste(t); len(l) != 0 {
		t.Fatalf("processos de runner sobraram: %v", l)
	}
}

func TestIALaçoInfinitoPermissaoEChave(t *testing.T) {
	m := novoMotor(t, comRunner(t))
	c := m.contaConectada("+5511900000011")
	m.json("PATCH", "/v1/automacoes/configuracao", map[string]any{"tempo_ia_s": 5}, 200, nil)

	trava := m.criarIA("Trava", "em_branco")
	m.escrever(trava.ID, "automacao.json", manifesto("Trava", `[{"tipo":"palavra_chave","palavras":["trava"]}]`, `[]`, "5"))
	m.escrever(trava.ID, "index.ts", `import { definirAutomacao } from '@zapdesk/automacao';
export default definirAutomacao({ aoReceberMensagem() { while (true) {} } });
`)
	m.json("POST", "/v1/automacoes/"+trava.ID+"/ativar", map[string]any{}, 200, nil)

	eco := m.criarIA("Eco", "em_branco")
	m.escrever(eco.ID, "automacao.json", manifesto("Eco", `[{"tipo":"palavra_chave","palavras":["eco"]}]`, `["enviar"]`, "5"))
	m.escrever(eco.ID, "index.ts", `import { definirAutomacao } from '@zapdesk/automacao';
export default definirAutomacao({ async aoReceberMensagem(ctx, msg) { await ctx.responder('eco: ' + msg.texto); } });
`)
	m.json("POST", "/v1/automacoes/"+eco.ID+"/ativar", map[string]any{}, 200, nil)

	// 5 conversas travam a mesma automação ao mesmo tempo.
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.receber(c.ID, map[string]any{"de": "+55119555500" + strconv.Itoa(10+i), "texto": "trava"})
		}(i)
	}
	wg.Wait()
	time.Sleep(300 * time.Millisecond) // deixa o processo da "Trava" subir e prender
	inicio := time.Now()
	m.receber(c.ID, map[string]any{"de": "+5511955559999", "texto": "eco oi"})
	for len(m.enviadasPara("+5511955559999")) == 0 {
		if time.Since(inicio) > 3*time.Second {
			t.Fatal("a outra automação ficou presa atrás do laço infinito")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if d := time.Since(inicio); d > 3*time.Second {
		t.Fatalf("resposta demorou %v", d)
	}
	m.ociosoAut()
	ex := m.execucoesDe(trava.ID)
	erros := 0
	for _, e := range ex {
		if e.Estado == "erro" {
			erros++
		}
	}
	var a Automacao
	m.json("GET", "/v1/automacoes/"+trava.ID, nil, 200, &a)
	if erros != 5 || a.Ativa || a.DesativadaMotivo == nil || *a.DesativadaMotivo != "erros_seguidos" {
		t.Fatalf("laço infinito: %d erros, %+v", erros, a)
	}
	temPrazo := false
	for _, e := range ex {
		if strings.Contains(ptr(e.Erro), "Tempo limite de") {
			temPrazo = true
		}
	}
	if !temPrazo {
		t.Fatalf("mensagem de tempo limite ausente: %+v", ex)
	}

	// Sem a permissão "enviar": ErroPermissao no código → execução com erro.
	sem := m.criarIA("Sem permissão", "em_branco")
	m.escrever(sem.ID, "automacao.json", manifesto("Sem permissão", `[{"tipo":"palavra_chave","palavras":["perm"]}]`, `[]`, "10"))
	m.escrever(sem.ID, "index.ts", `import { definirAutomacao } from '@zapdesk/automacao';
export default definirAutomacao({ async aoReceberMensagem(ctx) { await ctx.responder('oi'); } });
`)
	m.json("POST", "/v1/automacoes/"+sem.ID+"/ativar", map[string]any{}, 200, nil)
	m.injetar(c.ID, "+5511955558888", "perm")
	ex = m.execucoesDe(sem.ID)
	if len(ex) != 1 || ex[0].Estado != "erro" || !strings.Contains(ptr(ex[0].Erro), "Permissão 'enviar' não declarada") {
		t.Fatalf("permissão: %+v", ex)
	}
}

func TestIAChaveAusenteComIAReal(t *testing.T) {
	m := novoMotor(t, comRunner(t), func(o *opcoesMotor) { o.iaReal = true })
	c := m.contaConectada("+5511900000012")
	a := m.criarIA("Responder", "responder_historico")
	m.json("POST", "/v1/automacoes/"+a.ID+"/ativar", map[string]any{}, 200, nil)
	m.injetar(c.ID, "+5511944443333", "Olá")
	ex := m.execucoesDe(a.ID)
	if len(ex) != 1 || ex[0].Estado != "erro" || !strings.Contains(ptr(ex[0].Erro), "Configure a chave da Anthropic em Ajustes → IA") {
		t.Fatalf("chave ausente: %s %+v", ptr(ex[0].Erro), ex)
	}
	if len(m.enviadasPara("+5511944443333")) != 0 {
		t.Fatal("não deveria responder")
	}
}
