package api

import (
	"encoding/json"
	"net/http"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/automacoes"
	"zapdesk/motor/internal/erros"
)

// Rotas comuns das automações (specs/002-automacoes/contracts/api-http.md): configuração,
// estado da conversa, pausas, execuções, segredos e IA.
func (s *Servidor) registrarRotasAutomacoesComum() {
	s.rota("GET /v1/automacoes/configuracao", s.configuracaoAutomacoes, false)
	s.rota("PATCH /v1/automacoes/configuracao", s.alterarConfiguracaoAutomacoes, false)
	s.rota("GET /v1/conversas/{id}/automacoes", s.estadoConversaAutomacoes, false)
	s.rota("POST /v1/conversas/{id}/pausa", s.pausarConversa, false)
	s.rota("DELETE /v1/conversas/{id}/pausa", s.retomarConversa, false)
	s.rota("GET /v1/pausas", s.listarPausas, false)
	s.rota("GET /v1/execucoes", s.listarExecucoes, false)
	s.rota("GET /v1/execucoes/{id}", s.obterExecucao, false)
	s.rota("GET /v1/automacoes/{id}/execucoes", s.listarExecucoesAutomacao, false)
	s.rota("GET /v1/segredos", s.listarSegredos, false)
	s.rota("GET /v1/ia/configuracao", s.configuracaoIA, false)
	s.rota("PATCH /v1/ia/configuracao", s.alterarConfiguracaoIA, false)
	s.rota("POST /v1/ia/testar-chave", s.testarChaveIA, false)
}

func (s *Servidor) aut() *automacoes.Servico { return s.o.Servicos.Automacoes }

func (s *Servidor) configuracaoAutomacoes(w http.ResponseWriter, r *http.Request) error {
	return responderJSON(w, 200, s.aut().Config().Configuracao())
}

func (s *Servidor) alterarConfiguracaoAutomacoes(w http.ResponseWriter, r *http.Request) error {
	var corpo map[string]json.RawMessage
	if err := lerJSON(w, r, &corpo); err != nil {
		return err
	}
	c, err := s.aut().Config().Atualizar(r.Context(), corpo)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, c)
}

func (s *Servidor) estadoConversaAutomacoes(w http.ResponseWriter, r *http.Request) error {
	e, err := s.aut().EstadoConversa(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, e)
}

func (s *Servidor) pausarConversa(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Motivo     string `json:"motivo"`
		DuracaoMin *int   `json:"duracao_min"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	p, err := s.aut().Pausar(r.Context(), r.PathValue("id"), c.Motivo, c.DuracaoMin)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, p)
}

func (s *Servidor) retomarConversa(w http.ResponseWriter, r *http.Request) error {
	if err := s.aut().Retomar(r.Context(), r.PathValue("id")); err != nil {
		return err
	}
	return semConteudo(w)
}

func (s *Servidor) listarPausas(w http.ResponseWriter, r *http.Request) error {
	l, err := s.aut().Pausas(r.Context(), r.URL.Query().Get("motivo"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, l)
}

func (s *Servidor) execucoes(w http.ResponseWriter, r *http.Request, automacaoID string) error {
	p, err := lerPagina(r)
	if err != nil {
		return err
	}
	q := r.URL.Query()
	l, prox, err := s.aut().Execucoes(r.Context(), armazenamento.FiltroExecucoes{AutomacaoID: automacaoID, Estado: q.Get("estado"),
		ConversaID: q.Get("conversa_id")}, p)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, NovaPagina(l, prox))
}

func (s *Servidor) listarExecucoes(w http.ResponseWriter, r *http.Request) error {
	return s.execucoes(w, r, "")
}

func (s *Servidor) listarExecucoesAutomacao(w http.ResponseWriter, r *http.Request) error {
	return s.execucoes(w, r, r.PathValue("id"))
}

func (s *Servidor) obterExecucao(w http.ResponseWriter, r *http.Request) error {
	e, err := s.aut().Execucao(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, e)
}

func (s *Servidor) listarSegredos(w http.ResponseWriter, r *http.Request) error {
	l, err := s.aut().Segredos(r.Context())
	if err != nil {
		return err
	}
	return responderJSON(w, 200, l)
}

func (s *Servidor) configuracaoIA(w http.ResponseWriter, r *http.Request) error {
	return responderJSON(w, 200, s.aut().ConfiguracaoIA(r.Context()))
}

func (s *Servidor) alterarConfiguracaoIA(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		ModeloPadrao *string `json:"modelo_padrao"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	if c.ModeloPadrao == nil {
		return erros.Campo("modelo_padrao", "Informe o modelo padrão.")
	}
	cfg, err := s.aut().DefinirModeloPadrao(r.Context(), *c.ModeloPadrao)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, cfg)
}

func (s *Servidor) testarChaveIA(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Modelo string `json:"modelo"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	cfg := s.aut().ConfiguracaoIA(r.Context())
	if !cfg.ChaveConfigurada {
		return erros.ComDetalhes(erros.IANaoConfigurada, "Configure a chave da Anthropic em Ajustes → IA.", map[string]any{"segredo": "ANTHROPIC_API_KEY"})
	}
	modelo := c.Modelo
	if modelo == "" {
		modelo = cfg.ModeloPadrao
	}
	lat, err := s.o.Servicos.Claude.TestarChave(r.Context(), modelo)
	if err != nil {
		return automacoes.ErroDaIA(err)
	}
	return responderJSON(w, 200, map[string]any{"ok": true, "modelo": modelo, "latencia_ms": lat.Milliseconds()})
}
