package api

import (
	"context"

	"zapdesk/motor/internal/arquivos"
	"zapdesk/motor/internal/automacoes"
	"zapdesk/motor/internal/automacoes/chatbot"
	"zapdesk/motor/internal/automacoes/simulador"
	"zapdesk/motor/internal/chat"
	"zapdesk/motor/internal/claude"
	"zapdesk/motor/internal/contas"
	"zapdesk/motor/internal/disparos"
	"zapdesk/motor/internal/funil"
	"zapdesk/motor/internal/leads"
	"zapdesk/motor/internal/organizacao"
	"zapdesk/motor/internal/segredos"
)

// Servicos são os serviços de domínio usados pelos handlers (nil desliga as rotas).
type Servicos struct {
	Contadores  Contadores
	Contas      *contas.Gerenciador
	Chat        *chat.Servico
	Arquivos    *arquivos.Servico
	Leads       *leads.Servico
	Disparos    *disparos.Servico
	Organizacao *organizacao.Servico
	Extras      []func(s *Servidor)

	// Feature 002.
	Automacoes       *automacoes.Servico
	Funil            *funil.Servico
	Claude           claude.Cliente
	IAFalsa          *claude.Falsa
	Cofre            *segredos.Cofre
	ProcessarEsperas func(ctx context.Context) error
	AutomacoesIA     *automacoes.IA
	Chatbot          *chatbot.Servico
	Simulador        *simulador.Simulador
}

// Contadores alimenta GET /v1/sistema.
type Contadores interface {
	DisparosAtivos() int
	ContasConectadas() int
}

// ContadoresAutomacoes alimenta os campos da feature 002 em GET /v1/sistema.
type ContadoresAutomacoes interface {
	AutomacoesAtivas() int
	ProcessosIA() int
	RunnerDisponivel() bool
}

type contexto = context.Context
