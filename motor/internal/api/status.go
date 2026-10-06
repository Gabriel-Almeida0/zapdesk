package api

import "net/http"

func (s *Servidor) registrarRotasStatus() {
	s.rota("GET /v1/contas/{id}/status", s.listarStatus, false)
	s.rota("GET /v1/status/{id}/midia", s.midiaStatus, true)
}

func (s *Servidor) listarStatus(w http.ResponseWriter, r *http.Request) error {
	lista, err := s.o.Servicos.Chat.ListarStatus(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, lista)
}

func (s *Servidor) midiaStatus(w http.ResponseWriter, r *http.Request) error {
	caminho, mt, err := s.o.Servicos.Chat.ConteudoStatus(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return servirArquivo(w, r, caminho, mt, "")
}
