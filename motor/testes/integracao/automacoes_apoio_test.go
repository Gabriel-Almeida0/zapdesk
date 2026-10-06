package integracao

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Apoio dos testes de integração da feature 002 (T046): runner real via `node` +
// automacao/runner/dist quando presente (pula testes de IA com aviso; com ZAPDESK_CI=1 falha),
// injeção de mensagens e passada das esperas.

// runnerDisponivel devolve (node, script) ou pula/falha o teste.
func runnerDisponivel(t *testing.T) (string, string) {
	t.Helper()
	_, arq, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(arq), "..", "..", "..", "automacao", "runner", "dist", "zapdesk-runner.mjs")
	script, _ = filepath.Abs(script)
	node, err := exec.LookPath("node")
	_, errScript := os.Stat(script)
	if err != nil || errScript != nil {
		if os.Getenv("ZAPDESK_CI") == "1" {
			t.Fatalf("runner indisponível no CI (node=%v, script=%v): rode npm run runner:compilar", err, errScript)
		}
		t.Skipf("aviso: runner indisponível (node=%v, script=%v); teste de IA pulado", err, errScript)
	}
	return node, script
}

func comRunner(t *testing.T) func(*opcoesMotor) {
	node, script := runnerDisponivel(t)
	return func(o *opcoesMotor) { o.runnerExec, o.runnerScript = node, script }
}

// processarEsperas força uma passada do agendador e espera os despachos.
func (m *Motor) processarEsperas() {
	m.t.Helper()
	m.json("POST", "/v1/falso/processar-esperas", map[string]any{}, 200, nil)
}

// ocioso espera o despachante de automações terminar tudo o que está na fila.
func (m *Motor) ociosoAut() {
	m.t.Helper()
	ctx, cancelar := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelar()
	if err := m.App.AguardarAutomacoes(ctx); err != nil {
		m.t.Fatalf("automações não ficaram ociosas: %v", err)
	}
}

// avancar move o relógio controlável (acorda o agendador) e processa as esperas vencidas.
func (m *Motor) avancarAut(d time.Duration) {
	m.t.Helper()
	m.json("PUT", "/v1/falso/relogio", map[string]any{"avancar_s": d.Seconds()}, 200, nil)
	m.processarEsperas()
}

// injetar recebe uma mensagem na conta e espera as automações processarem.
func (m *Motor) injetar(contaID, de, texto string) string {
	m.t.Helper()
	w := m.receber(contaID, map[string]any{"de": de, "texto": texto, "nome": "Contato " + de[len(de)-4:]})
	m.ociosoAut()
	return w
}

// Tipos mínimos da feature 002 usados nos testes.
type Automacao struct {
	ID               string          `json:"id"`
	Tipo             string          `json:"tipo"`
	Nome             string          `json:"nome"`
	Ativa            bool            `json:"ativa"`
	Versao           int             `json:"versao"`
	Gatilhos         json.RawMessage `json:"gatilhos"`
	Definicao        json.RawMessage `json:"definicao"`
	Avisos           []ErroDef       `json:"avisos"`
	ErrosSeguidos    int             `json:"erros_seguidos"`
	DesativadaMotivo *string         `json:"desativada_motivo"`
	SessoesAtivas    int             `json:"sessoes_ativas"`
	Estatisticas     struct {
		OK, Erro, Abortada int
	} `json:"estatisticas_24h"`
	IA *struct {
		CompilacaoOK          bool      `json:"compilacao_ok"`
		ErrosCompilacao       []ErroDef `json:"erros_compilacao"`
		RodandoVersaoAnterior bool      `json:"rodando_versao_anterior"`
		HashCompilado         *string   `json:"hash_compilado"`
	} `json:"ia"`
}

type ErroDef struct {
	Caminho  string  `json:"caminho"`
	NoID     *string `json:"no_id"`
	AcaoID   *string `json:"acao_id"`
	Mensagem string  `json:"mensagem"`
	Arquivo  string  `json:"arquivo"`
	Linha    int     `json:"linha"`
	Tipo     string  `json:"tipo"`
}

type AcaoReg struct {
	Tipo      string  `json:"tipo"`
	Alvo      *string `json:"alvo"`
	Resultado string  `json:"resultado"`
	Detalhe   *string `json:"detalhe"`
}

type Execucao struct {
	ID          string          `json:"id"`
	AutomacaoID string          `json:"automacao_id"`
	Estado      string          `json:"estado"`
	Origem      string          `json:"origem"`
	Simulacao   bool            `json:"simulacao"`
	Motivo      *string         `json:"motivo"`
	Erro        *string         `json:"erro"`
	ConversaID  *string         `json:"conversa_id"`
	LeadID      *string         `json:"lead_id"`
	Acoes       []AcaoReg       `json:"acoes"`
	RetomarEm   *string         `json:"retomar_em"`
	Log         string          `json:"log"`
	Retorno     json.RawMessage `json:"retorno"`
	Gatilho     struct {
		Tipo  string         `json:"tipo"`
		Dados map[string]any `json:"dados"`
	} `json:"gatilho"`
	Tokens struct {
		Entrada int `json:"entrada"`
		Saida   int `json:"saida"`
	} `json:"tokens"`
}

type Pausa struct {
	ConversaID  string  `json:"conversa_id"`
	Motivo      string  `json:"motivo"`
	Ate         *string `json:"ate"`
	AutomacaoID *string `json:"automacao_id"`
}

// execucoes lista as execuções de uma automação (mais recentes primeiro).
func (m *Motor) execucoesDe(automacaoID string) []Execucao {
	m.t.Helper()
	var p Pagina[Execucao]
	m.json("GET", "/v1/automacoes/"+automacaoID+"/execucoes?limite=200", nil, 200, &p)
	return p.Itens
}

// criarAutomacao cria e (opcionalmente) ativa um fluxo/chatbot.
func (m *Motor) criarAutomacao(corpo map[string]any, ativar bool) Automacao {
	m.t.Helper()
	var a Automacao
	m.json("POST", "/v1/automacoes", corpo, 201, &a)
	if ativar {
		m.json("POST", "/v1/automacoes/"+a.ID+"/ativar", map[string]any{}, 200, &a)
	}
	return a
}

// enviadasPara filtra /v1/falso/enviadas por telefone.
func (m *Motor) enviadasPara(tel string) []map[string]any {
	m.t.Helper()
	var l []map[string]any
	m.json("GET", "/v1/falso/enviadas", nil, 200, &l)
	var r []map[string]any
	for _, e := range l {
		if e["telefone"] == tel || e["para"] == tel[1:]+"@s.whatsapp.net" {
			r = append(r, e)
		}
	}
	return r
}
