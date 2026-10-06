// Pacote fatos define os fatos de domínio que alimentam o despachante de automações
// (specs/002-automacoes/research.md › R2) sem dependência circular: chat, organização, leads,
// disparos e funil publicam aqui; o despachante (automacoes/despacho) implementa Receptor.
// Receber NUNCA pode bloquear (só enfileira).
package fatos

import (
	"context"
	"time"
)

// Tipos de fato.
const (
	MensagemRecebida = "mensagem_recebida"
	MensagemEnviada  = "mensagem_enviada"
	Etiqueta         = "etiqueta"
	LeadImportado    = "lead_importado"
	DisparoRespondeu = "disparo_respondeu"
	EntrouEtapa      = "entrou_etapa"
)

// Origens de mensagem enviada.
const (
	OrigemManual    = "manual"
	OrigemDisparo   = "disparo"
	OrigemAutomacao = "automacao"
)

// Fato é um acontecimento que pode disparar automações.
type Fato struct {
	Tipo       string
	ContaID    string
	ConversaID string
	ContatoID  string
	LeadID     string
	MensagemID string
	Grupo      bool
	Texto      string
	Em         time.Time

	// mensagem_recebida
	Primeira bool
	TemMidia bool
	// mensagem_enviada
	Origem      string // manual | disparo | automacao
	AutomacaoID string // automação que enviou (origem automacao)

	// etiqueta
	EtiquetaID string
	Evento     string // adicionada | removida
	// lead_importado
	LeadIDs    []string
	OrigemLead string
	// disparo_respondeu
	DisparoID      string
	DestinatarioID string
	// entrou_etapa
	FunilID         string
	EtapaID         string
	EtapaAnteriorID string

	// Cadeia de automações que causaram o fato (R11).
	Cadeia []string
}

// Receptor recebe fatos (implementado pelo despachante).
type Receptor interface {
	Receber(f Fato)
}

// ReceptorFunc adapta uma função.
type ReceptorFunc func(Fato)

// Receber chama a função.
func (f ReceptorFunc) Receber(x Fato) { f(x) }

type chaveCadeia struct{}

// ComCadeia marca o contexto de uma ação de automação com a cadeia causal; fatos emitidos pelos
// serviços chamados com esse contexto herdam a cadeia.
func ComCadeia(ctx context.Context, cadeia []string) context.Context {
	return context.WithValue(ctx, chaveCadeia{}, append([]string{}, cadeia...))
}

// CadeiaDe devolve a cadeia do contexto (nil fora de automação).
func CadeiaDe(ctx context.Context) []string {
	c, _ := ctx.Value(chaveCadeia{}).([]string)
	return c
}

// Emissor é embutido nos serviços que publicam fatos.
type Emissor struct{ r Receptor }

// DefinirReceptor liga o despachante (nil desliga).
func (e *Emissor) DefinirReceptor(r Receptor) { e.r = r }

// Emitir entrega o fato (com a cadeia do contexto) se houver receptor.
func (e *Emissor) Emitir(ctx context.Context, f Fato) {
	if e == nil || e.r == nil {
		return
	}
	if f.Cadeia == nil {
		f.Cadeia = CadeiaDe(ctx)
	}
	e.r.Receber(f)
}
