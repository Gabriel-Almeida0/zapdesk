package api

import (
	"net/http"

	"zapdesk/motor/internal/armazenamento"
)

func (s *Servidor) registrarRotasContatos() {
	s.rota("GET /v1/contas/{id}/contatos", s.listarContatos, false)
	s.rota("GET /v1/contatos/{id}", s.obterContato, false)
}

func (s *Servidor) listarContatos(w http.ResponseWriter, r *http.Request) error {
	p, err := lerPagina(r)
	if err != nil {
		return err
	}
	id := r.PathValue("id")
	banco := s.o.Servicos.Contas.Banco()
	if _, err := armazenamento.ObterConta(r.Context(), banco.L(), id); err != nil {
		return err
	}
	q := r.URL.Query()
	lista, prox, err := armazenamento.ListarContatos(r.Context(), banco.L(), id,
		armazenamento.FiltroContatos{Busca: q.Get("busca"), EtiquetaID: q.Get("etiqueta_id")}, p)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, NovaPagina(lista, prox))
}

func (s *Servidor) obterContato(w http.ResponseWriter, r *http.Request) error {
	c, err := armazenamento.ObterContato(r.Context(), s.o.Servicos.Contas.Banco().L(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, c)
}
