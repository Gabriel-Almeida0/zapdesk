package api

import (
	"encoding/json"
	"net/http"

	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/organizacao"
)

func (s *Servidor) registrarRotasOrganizacao() {
	s.rota("GET /v1/etiquetas", s.listarEtiquetas, false)
	s.rota("POST /v1/etiquetas", s.criarEtiqueta, false)
	s.rota("PATCH /v1/etiquetas/{id}", s.editarEtiqueta, false)
	s.rota("DELETE /v1/etiquetas/{id}", s.excluirEtiqueta, false)
	s.rota("PATCH /v1/contatos/{id}", s.editarContato, false)
	s.rota("PUT /v1/contatos/{id}/etiquetas", s.etiquetasContato, false)
	s.rota("GET /v1/templates", s.listarTemplates, false)
	s.rota("POST /v1/templates", s.criarTemplate, false)
	s.rota("GET /v1/templates/{id}", s.obterTemplate, false)
	s.rota("PATCH /v1/templates/{id}", s.editarTemplate, false)
	s.rota("DELETE /v1/templates/{id}", s.excluirTemplate, false)
}

func (s *Servidor) listarEtiquetas(w http.ResponseWriter, r *http.Request) error {
	l, err := s.o.Servicos.Organizacao.ListarEtiquetas(r.Context())
	if err != nil {
		return err
	}
	return responderJSON(w, 200, l)
}

func (s *Servidor) criarEtiqueta(w http.ResponseWriter, r *http.Request) error {
	var c struct{ Nome, Cor string }
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	e, err := s.o.Servicos.Organizacao.CriarEtiqueta(r.Context(), c.Nome, c.Cor)
	if err != nil {
		return err
	}
	return responderJSON(w, 201, e)
}

func (s *Servidor) editarEtiqueta(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Nome *string `json:"nome"`
		Cor  *string `json:"cor"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	e, err := s.o.Servicos.Organizacao.EditarEtiqueta(r.Context(), r.PathValue("id"), c.Nome, c.Cor)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, e)
}

func (s *Servidor) excluirEtiqueta(w http.ResponseWriter, r *http.Request) error {
	if err := s.o.Servicos.Organizacao.ExcluirEtiqueta(r.Context(), r.PathValue("id")); err != nil {
		return err
	}
	return semConteudo(w)
}

func (s *Servidor) editarContato(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Notas *string `json:"notas"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	ct, err := s.o.Servicos.Organizacao.DefinirNotas(r.Context(), r.PathValue("id"), c.Notas)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, ct)
}

func (s *Servidor) etiquetasContato(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		EtiquetaIDs *[]string `json:"etiqueta_ids"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	if c.EtiquetaIDs == nil {
		return erros.Campo("etiqueta_ids", "Envie a lista de etiquetas (pode ser vazia).")
	}
	ct, err := s.o.Servicos.Organizacao.DefinirEtiquetas(r.Context(), r.PathValue("id"), *c.EtiquetaIDs)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, ct)
}

func (s *Servidor) listarTemplates(w http.ResponseWriter, r *http.Request) error {
	l, err := s.o.Servicos.Organizacao.ListarTemplates(r.Context(), r.URL.Query().Get("busca"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, l)
}

func (s *Servidor) criarTemplate(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Nome      string  `json:"nome"`
		Texto     string  `json:"texto"`
		ArquivoID *string `json:"arquivo_id"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	t, err := s.o.Servicos.Organizacao.CriarTemplate(r.Context(), c.Nome, c.Texto, c.ArquivoID)
	if err != nil {
		return err
	}
	return responderJSON(w, 201, t)
}

func (s *Servidor) obterTemplate(w http.ResponseWriter, r *http.Request) error {
	t, err := s.o.Servicos.Organizacao.ObterTemplate(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, t)
}

func (s *Servidor) editarTemplate(w http.ResponseWriter, r *http.Request) error {
	var bruto map[string]json.RawMessage
	if err := lerJSON(w, r, &bruto); err != nil {
		return err
	}
	var a organizacao.AlteracaoTemplate
	ler := func(chave string, destino **string) error {
		if v, ok := bruto[chave]; ok {
			if err := json.Unmarshal(v, destino); err != nil {
				return erros.Campo(chave, "Valor inválido.")
			}
		}
		return nil
	}
	if err := ler("nome", &a.Nome); err != nil {
		return err
	}
	if err := ler("texto", &a.Texto); err != nil {
		return err
	}
	if _, ok := bruto["arquivo_id"]; ok {
		a.ArquivoDefinido = true
		if err := ler("arquivo_id", &a.ArquivoID); err != nil {
			return err
		}
	}
	t, err := s.o.Servicos.Organizacao.EditarTemplate(r.Context(), r.PathValue("id"), a)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, t)
}

func (s *Servidor) excluirTemplate(w http.ResponseWriter, r *http.Request) error {
	if err := s.o.Servicos.Organizacao.ExcluirTemplate(r.Context(), r.PathValue("id")); err != nil {
		return err
	}
	return semConteudo(w)
}
