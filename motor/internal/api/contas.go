package api

import (
	"net/http"
	"time"
)

func (s *Servidor) registrarRotasContas() {
	s.rota("GET /v1/contas", s.listarContas, false)
	s.rota("POST /v1/contas", s.criarConta, false)
	s.rota("GET /v1/contas/{id}", s.obterConta, false)
	s.rota("PATCH /v1/contas/{id}", s.renomearConta, false)
	s.rota("DELETE /v1/contas/{id}", s.removerConta, false)
	s.rota("GET /v1/contas/{id}/qr", s.qrConta, false)
	s.rota("POST /v1/contas/{id}/reconectar", s.reconectarConta, false)
}

func (s *Servidor) listarContas(w http.ResponseWriter, r *http.Request) error {
	lista, err := s.o.Servicos.Contas.Listar(r.Context())
	if err != nil {
		return err
	}
	return responderJSON(w, 200, lista)
}

func (s *Servidor) criarConta(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Nome *string `json:"nome"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	conta, err := s.o.Servicos.Contas.Criar(r.Context(), c.Nome)
	if err != nil {
		return err
	}
	return responderJSON(w, 201, conta)
}

func (s *Servidor) obterConta(w http.ResponseWriter, r *http.Request) error {
	conta, err := s.o.Servicos.Contas.Obter(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, conta)
}

func (s *Servidor) renomearConta(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Nome string `json:"nome"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	conta, err := s.o.Servicos.Contas.Renomear(r.Context(), r.PathValue("id"), c.Nome)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, conta)
}

func (s *Servidor) removerConta(w http.ResponseWriter, r *http.Request) error {
	if err := s.o.Servicos.Contas.Remover(r.Context(), r.PathValue("id")); err != nil {
		return err
	}
	return semConteudo(w)
}

func (s *Servidor) qrConta(w http.ResponseWriter, r *http.Request) error {
	codigo, expira, err := s.o.Servicos.Contas.QR(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, map[string]any{"codigo": codigo, "expira_em": expira.Format(time.RFC3339Nano)})
}

func (s *Servidor) reconectarConta(w http.ResponseWriter, r *http.Request) error {
	conta, err := s.o.Servicos.Contas.Reconectar(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return responderJSON(w, 202, conta)
}
