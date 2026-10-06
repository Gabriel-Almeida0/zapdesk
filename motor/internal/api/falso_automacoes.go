package api

import (
	"context"
	"net/http"
	"time"

	"zapdesk/motor/internal/claude"
	"zapdesk/motor/internal/erros"
)

// Rotas do modo falso da feature 002 (contracts/runtime.md › Modo falso — acréscimos).
func (s *Servidor) registrarRotasFalsoAutomacoes() {
	s.rota("POST /v1/falso/segredos", s.falsoSegredos, false)
	s.rota("POST /v1/falso/processar-esperas", s.falsoProcessarEsperas, false)
	s.rota("PUT /v1/falso/ia", s.falsoIA, false)
	s.rota("GET /v1/falso/ia/chamadas", s.falsoIAChamadas, false)
}

func (s *Servidor) falsoSegredos(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Valores map[string]string `json:"valores"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	if c.Valores == nil {
		return erros.Campo("valores", "Envie {\"valores\": {NOME: valor}}.")
	}
	s.o.Servicos.Cofre.Substituir(c.Valores)
	return responderJSON(w, 200, map[string]any{"ok": true, "nomes": s.o.Servicos.Cofre.Nomes()})
}

func (s *Servidor) falsoProcessarEsperas(w http.ResponseWriter, r *http.Request) error {
	ctx, cancelar := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancelar()
	if err := s.o.Servicos.ProcessarEsperas(ctx); err != nil {
		return err
	}
	return responderJSON(w, 200, map[string]any{"ok": true})
}

func (s *Servidor) iaFalsa() (*claude.Falsa, error) {
	if s.o.Servicos.IAFalsa == nil {
		return nil, erros.Novo(erros.NaoEncontrado, "IA simulada indisponível (inicie o motor com --ia=falsa).")
	}
	return s.o.Servicos.IAFalsa, nil
}

func (s *Servidor) falsoIA(w http.ResponseWriter, r *http.Request) error {
	f, err := s.iaFalsa()
	if err != nil {
		return err
	}
	var c struct {
		Respostas []claude.RespostaFalsa `json:"respostas"`
		Erro      *claude.ErroFalso      `json:"erro"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	f.Configurar(c.Respostas, c.Erro)
	return responderJSON(w, 200, map[string]any{"ok": true})
}

func (s *Servidor) falsoIAChamadas(w http.ResponseWriter, r *http.Request) error {
	f, err := s.iaFalsa()
	if err != nil {
		return err
	}
	return responderJSON(w, 200, f.Chamadas())
}
