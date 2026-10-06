package automacoes

import (
	"context"
	"strings"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/claude"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/segredos"
)

// ConfiguracaoIA de GET /ia/configuracao.
type ConfiguracaoIA struct {
	ModeloPadrao     string              `json:"modelo_padrao"`
	Modelos          []claude.InfoModelo `json:"modelos"`
	ChaveConfigurada bool                `json:"chave_configurada"`
}

// ModeloPadrao lê "ia.modelo_padrao" (padrão claude-sonnet-5).
func (s *Servico) ModeloPadrao(ctx context.Context) string {
	var m string
	if ok, err := armazenamento.LerConfiguracao(ctx, s.d.Banco.L(), "ia.modelo_padrao", &m); ok && err == nil && strings.HasPrefix(m, "claude-") {
		return m
	}
	return claude.ModeloPadrao
}

// ConfiguracaoIA atual.
func (s *Servico) ConfiguracaoIA(ctx context.Context) ConfiguracaoIA {
	return ConfiguracaoIA{ModeloPadrao: s.ModeloPadrao(ctx), Modelos: claude.Modelos(), ChaveConfigurada: s.d.Cofre.Tem(segredos.ChaveAnthropic)}
}

// DefinirModeloPadrao grava o modelo (começa com "claude-").
func (s *Servico) DefinirModeloPadrao(ctx context.Context, modelo string) (ConfiguracaoIA, error) {
	modelo = strings.TrimSpace(modelo)
	if !strings.HasPrefix(modelo, "claude-") || len(modelo) > 100 {
		return s.ConfiguracaoIA(ctx), erros.Campo("modelo_padrao", "O modelo deve começar com \"claude-\".")
	}
	if err := armazenamento.GravarConfiguracao(ctx, s.d.Banco.E(), "ia.modelo_padrao", modelo); err != nil {
		return s.ConfiguracaoIA(ctx), err
	}
	return s.ConfiguracaoIA(ctx), nil
}

// ErroDaIA traduz erros do cliente Claude para erros da API (ia_nao_configurada / ia_erro).
func ErroDaIA(err error) error {
	e, ok := err.(*claude.Erro)
	if !ok {
		return err
	}
	switch e.Codigo {
	case claude.CodigoNaoConfigurada:
		return erros.ComDetalhes(erros.IANaoConfigurada, e.Mensagem, map[string]any{"segredo": segredos.ChaveAnthropic})
	case claude.CodigoValidacao:
		return erros.Novo(erros.Validacao, e.Mensagem)
	}
	d := map[string]any{"status": e.Status}
	if e.RequestID != "" {
		d["request_id"] = e.RequestID
	}
	return erros.ComDetalhes(erros.IAErro, e.Mensagem, d)
}
