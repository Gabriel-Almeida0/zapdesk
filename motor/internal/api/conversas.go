package api

import (
	"net/http"

	"zapdesk/motor/internal/armazenamento"
)

func (s *Servidor) registrarRotasConversas() {
	s.rota("GET /v1/contas/{id}/conversas", s.listarConversas, false)
	s.rota("POST /v1/contas/{id}/conversas", s.novaConversa, false)
	s.rota("GET /v1/conversas/{id}", s.obterConversa, false)
	s.rota("POST /v1/conversas/{id}/lida", s.marcarLida, false)
	s.rota("GET /v1/conversas/{id}/mensagens", s.listarMensagens, false)
	s.rota("POST /v1/conversas/{id}/mensagens", s.enviarMensagem, false)
	s.rota("GET /v1/contas/{id}/mensagens/busca", s.buscarMensagens, false)
}

func (s *Servidor) listarConversas(w http.ResponseWriter, r *http.Request) error {
	p, err := lerPagina(r)
	if err != nil {
		return err
	}
	id := r.PathValue("id")
	if _, err := armazenamento.ObterConta(r.Context(), s.o.Servicos.Contas.Banco().L(), id); err != nil {
		return err
	}
	q := r.URL.Query()
	f := armazenamento.FiltroConversas{Busca: q.Get("busca"), EtiquetaID: q.Get("etiqueta_id"), NaoLidas: q.Get("nao_lidas") == "true"}
	lista, prox, err := armazenamento.ListarConversas(r.Context(), s.o.Servicos.Contas.Banco().L(), id, f, p)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, NovaPagina(lista, prox))
}

func (s *Servidor) novaConversa(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Telefone string `json:"telefone"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	conv, criada, err := s.o.Servicos.Chat.NovaConversa(r.Context(), r.PathValue("id"), c.Telefone)
	if err != nil {
		return err
	}
	status := 200
	if criada {
		status = 201
	}
	return responderJSON(w, status, conv)
}

func (s *Servidor) obterConversa(w http.ResponseWriter, r *http.Request) error {
	c, err := armazenamento.ObterConversa(r.Context(), s.o.Servicos.Contas.Banco().L(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, c)
}

func (s *Servidor) marcarLida(w http.ResponseWriter, r *http.Request) error {
	if err := s.o.Servicos.Chat.MarcarLida(r.Context(), r.PathValue("id")); err != nil {
		return err
	}
	return semConteudo(w)
}

func (s *Servidor) listarMensagens(w http.ResponseWriter, r *http.Request) error {
	limite, err := lerLimite(r)
	if err != nil {
		return err
	}
	antes := r.URL.Query().Get("antes")
	if antes == "" {
		antes = r.URL.Query().Get("cursor")
	}
	lista, prox, err := s.o.Servicos.Chat.ListarMensagens(r.Context(), r.PathValue("id"), antes, limite)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, NovaPagina(lista, prox))
}

func (s *Servidor) buscarMensagens(w http.ResponseWriter, r *http.Request) error {
	p, err := lerPagina(r)
	if err != nil {
		return err
	}
	lista, prox, err := s.o.Servicos.Chat.Buscar(r.Context(), r.PathValue("id"), r.URL.Query().Get("q"), p)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, NovaPagina(lista, prox))
}
