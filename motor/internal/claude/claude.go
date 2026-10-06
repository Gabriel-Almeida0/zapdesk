// Pacote claude é o cliente da Claude API (Anthropic Messages API) usado pelo motor para os
// helpers ctx.ia das automações de IA (specs/002-automacoes/research.md › R7). A chave nunca sai
// do motor. Há uma implementação real (net/http, sem SDK) e uma falsa determinística (--ia=falsa).
package claude

import (
	"context"
	"encoding/json"
	"time"
)

// ModeloPadrao usado quando nem o manifesto nem Ajustes escolhem outro.
const ModeloPadrao = "claude-sonnet-5"

// Códigos de erro.
const (
	CodigoNaoConfigurada = "ia_nao_configurada"
	CodigoErro           = "ia_erro"
	CodigoValidacao      = "validacao"
)

// Limites de max_tokens.
const (
	MaxTokensPadrao = 1024
	MaxTokensLimite = 8192
)

// InfoModelo descreve um modelo oferecido na interface.
type InfoModelo struct {
	ID    string  `json:"id"`
	Nome  string  `json:"nome"`
	Aviso *string `json:"aviso"`
}

// Modelos devolve os modelos oferecidos em Ajustes → IA.
func Modelos() []InfoModelo {
	aviso := "Será aposentado pela Anthropic (não antes de 15/10/2026); prefira outro modelo."
	return []InfoModelo{
		{ID: "claude-sonnet-5", Nome: "Claude Sonnet 5"},
		{ID: "claude-opus-5-5", Nome: "Claude Opus 5.5"},
		{ID: "claude-haiku-4-5-20251001", Nome: "Claude Haiku 4.5", Aviso: &aviso},
	}
}

// Mensagem de uma conversa com a IA (papel "user" | "assistant").
type Mensagem struct {
	Papel string `json:"papel"`
	Texto string `json:"texto"`
}

// Uso de tokens de uma chamada.
type Uso struct{ Entrada, Saida int }

// PedidoGerar de ctx.ia.gerar.
type PedidoGerar struct {
	Modelo, Sistema, Prompt string
	Mensagens               []Mensagem
	MaxTokens               int
}

// RespostaGerar de ctx.ia.gerar.
type RespostaGerar struct {
	Texto, Modelo, MotivoParada, RequestID string
	Uso                                    Uso
}

// PedidoClassificar de ctx.ia.classificar.
type PedidoClassificar struct {
	Modelo, Sistema, Texto, Instrucoes string
	Categorias                         []string
	Descricoes                         map[string]string
	MaxTokens                          int
}

// RespostaClassificar de ctx.ia.classificar.
type RespostaClassificar struct {
	Categoria, Modelo, RequestID string
	Uso                          Uso
}

// PedidoExtrair de ctx.ia.extrair.
type PedidoExtrair struct {
	Modelo, Sistema, Texto, Instrucoes string
	Esquema                            map[string]any
	MaxTokens                          int
}

// RespostaExtrair de ctx.ia.extrair.
type RespostaExtrair struct {
	Dados             json.RawMessage
	Modelo, RequestID string
	Uso               Uso
}

// Erro exibível (mensagem em pt-BR).
type Erro struct {
	Codigo, Mensagem string
	Status           int
	RequestID        string
}

func (e *Erro) Error() string { return e.Codigo + ": " + e.Mensagem }

// Cliente da Claude API.
type Cliente interface {
	Gerar(ctx context.Context, p PedidoGerar) (RespostaGerar, error)
	Classificar(ctx context.Context, p PedidoClassificar) (RespostaClassificar, error)
	Extrair(ctx context.Context, p PedidoExtrair) (RespostaExtrair, error)
	TestarChave(ctx context.Context, modelo string) (time.Duration, error)
}

// FonteChave fornece a ANTHROPIC_API_KEY (satisfeita por *segredos.Cofre).
type FonteChave interface {
	Obter(nome string) (string, bool)
}

// NomeChave é o segredo reservado da Anthropic.
const NomeChave = "ANTHROPIC_API_KEY"

func modeloOu(m string) string {
	if m == "" {
		return ModeloPadrao
	}
	return m
}

func maxTokens(n int) int {
	if n <= 0 {
		return MaxTokensPadrao
	}
	if n > MaxTokensLimite {
		return MaxTokensLimite
	}
	return n
}
