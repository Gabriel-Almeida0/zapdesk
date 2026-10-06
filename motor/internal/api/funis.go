package api

import (
	"encoding/json"
	"net/http"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/funil"
)

// Rotas de funis (contracts/api-http.md › Funis).
func (s *Servidor) registrarRotasFunis() {
	s.rota("GET /v1/funis", s.listarFunis, false)
	s.rota("POST /v1/funis", s.criarFunil, false)
	s.rota("GET /v1/funis/{id}", s.obterFunil, false)
	s.rota("PATCH /v1/funis/{id}", s.editarFunil, false)
	s.rota("DELETE /v1/funis/{id}", s.excluirFunil, false)
	s.rota("POST /v1/funis/{id}/etapas", s.criarEtapa, false)
	s.rota("PUT /v1/funis/{id}/etapas/ordem", s.ordenarEtapas, false)
	s.rota("PATCH /v1/etapas/{id}", s.editarEtapa, false)
	s.rota("DELETE /v1/etapas/{id}", s.excluirEtapa, false)
	s.rota("GET /v1/funis/{id}/cards", s.listarCards, false)
	s.rota("PUT /v1/funis/{id}/cards", s.moverCard, false)
	s.rota("DELETE /v1/funis/{id}/cards/{lead_id}", s.removerCard, false)
	s.rota("GET /v1/funis/{id}/historico", s.historicoFunil, false)
	s.rota("GET /v1/leads/{id}/funis", s.funisDoLead, false)
}

func (s *Servidor) fun() *funil.Servico { return s.o.Servicos.Funil }

func origemCliente(v string) (funil.Origem, error) {
	switch v {
	case "", dominio.OrigemApp:
		return funil.Origem{Tipo: dominio.OrigemApp}, nil
	case dominio.OrigemMCPFunil:
		return funil.Origem{Tipo: dominio.OrigemMCPFunil}, nil
	}
	return funil.Origem{}, erros.Campo("origem", "Use app ou mcp.")
}

func (s *Servidor) listarFunis(w http.ResponseWriter, r *http.Request) error {
	l, err := s.fun().Listar(r.Context())
	if err != nil {
		return err
	}
	return responderJSON(w, 200, l)
}

func (s *Servidor) criarFunil(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Nome   string            `json:"nome"`
		Etapas []funil.NovaEtapa `json:"etapas"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	f, err := s.fun().Criar(r.Context(), c.Nome, c.Etapas)
	if err != nil {
		return err
	}
	return responderJSON(w, 201, f)
}

func (s *Servidor) obterFunil(w http.ResponseWriter, r *http.Request) error {
	f, err := s.fun().Obter(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, f)
}

func (s *Servidor) editarFunil(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Nome  *string `json:"nome"`
		Ordem *int    `json:"ordem"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	f, err := s.fun().Editar(r.Context(), r.PathValue("id"), c.Nome, c.Ordem)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, f)
}

func (s *Servidor) excluirFunil(w http.ResponseWriter, r *http.Request) error {
	if err := s.fun().Excluir(r.Context(), r.PathValue("id")); err != nil {
		return err
	}
	return semConteudo(w)
}

func (s *Servidor) criarEtapa(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Nome    string  `json:"nome"`
		Cor     *string `json:"cor"`
		Posicao *int    `json:"posicao"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	e, err := s.fun().CriarEtapa(r.Context(), r.PathValue("id"), c.Nome, c.Cor, c.Posicao)
	if err != nil {
		return err
	}
	return responderJSON(w, 201, e)
}

func (s *Servidor) ordenarEtapas(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		EtapaIDs []string `json:"etapa_ids"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	f, err := s.fun().ReordenarEtapas(r.Context(), r.PathValue("id"), c.EtapaIDs)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, f)
}

func (s *Servidor) editarEtapa(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Nome *string `json:"nome"`
		Cor  *string `json:"cor"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	e, err := s.fun().EditarEtapa(r.Context(), r.PathValue("id"), c.Nome, c.Cor)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, e)
}

func (s *Servidor) excluirEtapa(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	remover := q.Get("remover_cards") == "true"
	o, err := origemCliente(q.Get("origem"))
	if err != nil {
		return err
	}
	if err := s.fun().ExcluirEtapa(r.Context(), r.PathValue("id"), q.Get("destino_etapa_id"), remover, o); err != nil {
		return err
	}
	return semConteudo(w)
}

func (s *Servidor) listarCards(w http.ResponseWriter, r *http.Request) error {
	p, err := lerPagina(r)
	if err != nil {
		return err
	}
	q := r.URL.Query()
	l, prox, err := s.fun().Cards(r.Context(), r.PathValue("id"), armazenamento.FiltroCards{EtapaID: q.Get("etapa_id"), Busca: q.Get("busca")}, p)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, NovaPagina(l, prox))
}

func (s *Servidor) moverCard(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		LeadID    string `json:"lead_id"`
		ContatoID string `json:"contato_id"`
		Telefone  string `json:"telefone"`
		EtapaID   string `json:"etapa_id"`
		Origem    string `json:"origem"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	if c.EtapaID == "" {
		return erros.Campo("etapa_id", "Escolha a etapa.")
	}
	o, err := origemCliente(c.Origem)
	if err != nil {
		return err
	}
	card, entrou, err := s.fun().Mover(r.Context(), r.PathValue("id"), funil.AlvoCard{LeadID: c.LeadID, ContatoID: c.ContatoID, Telefone: c.Telefone}, c.EtapaID, o)
	if err != nil {
		return err
	}
	status := 200
	if entrou {
		status = 201
	}
	return responderJSON(w, status, card)
}

func (s *Servidor) removerCard(w http.ResponseWriter, r *http.Request) error {
	o, err := origemCliente(r.URL.Query().Get("origem"))
	if err != nil {
		return err
	}
	if err := s.fun().Remover(r.Context(), r.PathValue("id"), r.PathValue("lead_id"), o); err != nil {
		return err
	}
	return semConteudo(w)
}

func (s *Servidor) historicoFunil(w http.ResponseWriter, r *http.Request) error {
	p, err := lerPagina(r)
	if err != nil {
		return err
	}
	l, prox, err := s.fun().Historico(r.Context(), r.PathValue("id"), r.URL.Query().Get("lead_id"), p)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, NovaPagina(l, prox))
}

func (s *Servidor) funisDoLead(w http.ResponseWriter, r *http.Request) error {
	l, err := s.fun().FunisDoLead(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, l)
}

// decodificarAlteracaoLead lê o PATCH /leads/{id} distinguindo nome ausente de null.
func decodificarAlteracaoLead(corpo map[string]json.RawMessage) (alteracao, error) {
	var a alteracao
	if b, ok := corpo["nome"]; ok {
		a.nomeDefinido = true
		if string(b) != "null" {
			var n string
			if err := json.Unmarshal(b, &n); err != nil {
				return a, erros.Campo("nome", "Use um texto ou null.")
			}
			a.nome = &n
		}
	}
	if b, ok := corpo["campos"]; ok && string(b) != "null" {
		if err := json.Unmarshal(b, &a.campos); err != nil {
			return a, erros.Campo("campos", "Use um objeto {campo: valor|null}.")
		}
	}
	for k := range corpo {
		if k != "nome" && k != "campos" {
			return a, erros.Campo(k, "Campo desconhecido.")
		}
	}
	return a, nil
}

type alteracao struct {
	nome         *string
	nomeDefinido bool
	campos       map[string]*string
}
