package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/leads"
)

func (s *Servidor) registrarRotasLeads() {
	s.rota("POST /v1/importacoes/previa", s.previaImportacao, false)
	s.rota("POST /v1/leads/importar", s.importarLeads, false)
	s.rota("POST /v1/leads/importar-contatos", s.importarContatos, false)
	s.rota("GET /v1/leads", s.listarLeads, false)
	s.rota("GET /v1/leads/{id}", s.obterLead, false)
	s.rota("PATCH /v1/leads/{id}", s.alterarLead, false)
}

// alterarLead é o PATCH /v1/leads/{id} da feature 002 (nome e merge de campos; null remove).
func (s *Servidor) alterarLead(w http.ResponseWriter, r *http.Request) error {
	var corpo map[string]json.RawMessage
	if err := lerJSON(w, r, &corpo); err != nil {
		return err
	}
	a, err := decodificarAlteracaoLead(corpo)
	if err != nil {
		return err
	}
	l, err := s.o.Servicos.Leads.Atualizar(r.Context(), r.PathValue("id"), leads.AlteracaoLead{Nome: a.nome, NomeDefinido: a.nomeDefinido, Campos: a.campos})
	if err != nil {
		return err
	}
	return responderJSON(w, 200, l)
}

func (s *Servidor) previaImportacao(w http.ResponseWriter, r *http.Request) error {
	sv := s.o.Servicos.Leads
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, 51<<20)
		leitor, err := r.MultipartReader()
		if err != nil {
			return erros.Campo("arquivo", "Envie a planilha no campo \"arquivo\".")
		}
		for {
			parte, err := leitor.NextPart()
			if err != nil {
				return erros.Campo("arquivo", "Envie a planilha no campo \"arquivo\".")
			}
			if parte.FormName() != "arquivo" {
				parte.Close()
				continue
			}
			resp, err := sv.Previa(parte.FileName(), parte)
			parte.Close()
			if err != nil {
				return err
			}
			return responderJSON(w, 201, resp)
		}
	}
	var c struct {
		Caminho string `json:"caminho"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	if c.Caminho == "" {
		return erros.Campo("caminho", "Envie a planilha (multipart, campo \"arquivo\") ou o caminho absoluto.")
	}
	resp, err := sv.PreviaCaminho(c.Caminho)
	if err != nil {
		return err
	}
	return responderJSON(w, 201, resp)
}

func (s *Servidor) importarLeads(w http.ResponseWriter, r *http.Request) error {
	var c leads.CorpoImportacao
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	rel, err := s.o.Servicos.Leads.Importar(r.Context(), c)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, rel)
}

func (s *Servidor) importarContatos(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		ContaID     string   `json:"conta_id"`
		ContatoIDs  []string `json:"contato_ids"`
		EtiquetaIDs []string `json:"etiqueta_ids"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	rel, err := s.o.Servicos.Leads.ImportarContatos(r.Context(), c.ContaID, c.ContatoIDs, c.EtiquetaIDs)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, rel)
}

func (s *Servidor) listarLeads(w http.ResponseWriter, r *http.Request) error {
	p, err := lerPagina(r)
	if err != nil {
		return err
	}
	q := r.URL.Query()
	lista, prox, err := s.o.Servicos.Leads.Listar(r.Context(), q.Get("busca"), q.Get("origem"), p)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, NovaPagina(lista, prox))
}

func (s *Servidor) obterLead(w http.ResponseWriter, r *http.Request) error {
	l, err := s.o.Servicos.Leads.Obter(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, l)
}
