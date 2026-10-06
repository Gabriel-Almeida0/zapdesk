// Pacote runner é o lado motor do protocolo motor ↔ runner (specs/002-automacoes/contracts/
// runner-protocolo.md e runtime.md › Runner): um pool de processos Node isolados, um por
// (automação, hash compilado), falando JSON-RPC 2.0 em NDJSON pelo stdio, com prazo por execução
// (SIGKILL do grupo), detecção de falhas/OOM, ociosidade, limite de processos com fila e
// encerramento sem deixar processos filhos.
package runner

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Códigos de erro do protocolo.
const (
	CodigoRequisicaoInvalida = -32600
	CodigoMetodoDesconhecido = -32601
	CodigoExecucaoEncerrada  = 1007
	CodigoBundleInvalido     = 2001
	CodigoErroUsuario        = 2002
	CodigoHandlerAusente     = 2003
)

// LimiteLinha é o tamanho máximo de uma linha JSON-RPC.
const LimiteLinha = 1 << 20

// ErroRPC é o objeto `error` do JSON-RPC.
type ErroRPC struct {
	Codigo   int    `json:"code"`
	Mensagem string `json:"message"`
	Dados    any    `json:"data,omitempty"`
}

func (e *ErroRPC) Error() string { return fmt.Sprintf("erro %d: %s", e.Codigo, e.Mensagem) }

// ErroUsuario é a exceção lançada pelo código da automação (2002 erro_usuario).
type ErroUsuario struct {
	Nome, Mensagem, Stack string
}

func (e *ErroUsuario) Error() string {
	if e.Nome != "" && !strings.HasPrefix(e.Mensagem, e.Nome) {
		return e.Nome + ": " + e.Mensagem
	}
	return e.Mensagem
}

// ErroRunner é uma falha do processo (prazo, OOM, saída inesperada, handler ausente, bundle
// inválido). Stderr traz as últimas linhas do stderr do processo quando relevante.
type ErroRunner struct {
	Mensagem string
	Stderr   []string
}

func (e *ErroRunner) Error() string { return e.Mensagem }

// mensagem é uma linha JSON-RPC (requisição, notificação ou resposta).
type mensagem struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Metodo  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ErroRPC        `json:"error,omitempty"`
}
