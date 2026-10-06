package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/disparos"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
)

func (s *Servidor) registrarRotasDisparos() {
	s.rota("POST /v1/disparos/validar", s.validarDisparo, false)
	s.rota("POST /v1/disparos", s.criarDisparo, false)
	s.rota("GET /v1/disparos", s.listarDisparos, false)
	s.rota("GET /v1/disparos/{id}", s.obterDisparo, false)
	s.rota("PATCH /v1/disparos/{id}", s.editarDisparo, false)
	s.rota("DELETE /v1/disparos/{id}", s.excluirDisparo, false)
	s.rota("POST /v1/disparos/{id}/iniciar", s.iniciarDisparo, false)
	s.rota("POST /v1/disparos/{id}/pausar", s.acaoDisparo((*disparos.Servico).Pausar), false)
	s.rota("POST /v1/disparos/{id}/retomar", s.acaoDisparo((*disparos.Servico).Retomar), false)
	s.rota("POST /v1/disparos/{id}/cancelar", s.acaoDisparo((*disparos.Servico).Cancelar), false)
	s.rota("GET /v1/disparos/{id}/destinatarios", s.listarDestinatarios, false)
	s.rota("GET /v1/disparos/{id}/relatorio.csv", s.relatorioDisparo, true)
	s.rota("POST /v1/disparos/{id}/destinatarios", s.adicionarDestinatarios, false)
}

// adicionarDestinatarios acrescenta leads a um disparo (feature 002).
func (s *Servidor) adicionarDestinatarios(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		LeadIDs []string `json:"lead_ids"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	res, err := s.o.Servicos.Disparos.AdicionarDestinatarios(r.Context(), r.PathValue("id"), c.LeadIDs)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, res)
}

func (s *Servidor) validarDisparo(w http.ResponseWriter, r *http.Request) error {
	var n disparos.NovoDisparo
	if err := lerJSON(w, r, &n); err != nil {
		return err
	}
	resp, err := s.o.Servicos.Disparos.Validar(r.Context(), n)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, resp)
}

func (s *Servidor) criarDisparo(w http.ResponseWriter, r *http.Request) error {
	var n disparos.NovoDisparo
	if err := lerJSON(w, r, &n); err != nil {
		return err
	}
	d, err := s.o.Servicos.Disparos.Criar(r.Context(), n)
	if err != nil {
		return err
	}
	return responderJSON(w, 201, d)
}

var estadosDisparo = map[string]bool{dominio.DisparoRascunho: true, dominio.DisparoAgendado: true, dominio.DisparoEnviando: true,
	dominio.DisparoForaDaJanela: true, dominio.DisparoPausado: true, dominio.DisparoConcluido: true, dominio.DisparoCancelado: true}

func (s *Servidor) listarDisparos(w http.ResponseWriter, r *http.Request) error {
	p, err := lerPagina(r)
	if err != nil {
		return err
	}
	q := r.URL.Query()
	if e := q.Get("estado"); e != "" && !estadosDisparo[e] {
		return erros.Campo("estado", "Estado de disparo inválido.")
	}
	lista, prox, err := s.o.Servicos.Disparos.Listar(r.Context(), q.Get("conta_id"), q.Get("estado"), p)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, NovaPagina(lista, prox))
}

func (s *Servidor) obterDisparo(w http.ResponseWriter, r *http.Request) error {
	d, err := s.o.Servicos.Disparos.Obter(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, d)
}

func (s *Servidor) editarDisparo(w http.ResponseWriter, r *http.Request) error {
	var corpo map[string]json.RawMessage
	if err := lerJSON(w, r, &corpo); err != nil {
		return err
	}
	d, err := s.o.Servicos.Disparos.Editar(r.Context(), r.PathValue("id"), corpo)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, d)
}

func (s *Servidor) excluirDisparo(w http.ResponseWriter, r *http.Request) error {
	if err := s.o.Servicos.Disparos.Excluir(r.Context(), r.PathValue("id")); err != nil {
		return err
	}
	return semConteudo(w)
}

func (s *Servidor) iniciarDisparo(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		ValoresPadrao map[string]string `json:"valores_padrao"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	d, err := s.o.Servicos.Disparos.Iniciar(r.Context(), r.PathValue("id"), c.ValoresPadrao)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, d)
}

func (s *Servidor) acaoDisparo(f func(*disparos.Servico, contexto, string) (dominio.Disparo, error)) func(http.ResponseWriter, *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		d, err := f(s.o.Servicos.Disparos, r.Context(), r.PathValue("id"))
		if err != nil {
			return err
		}
		return responderJSON(w, 200, d)
	}
}

var estadosDest = map[string]bool{dominio.DestPendente: true, dominio.DestEnviando: true, dominio.DestEnviado: true,
	dominio.DestEntregue: true, dominio.DestLido: true, dominio.DestFalhou: true, dominio.DestRespondeu: true}

func (s *Servidor) listarDestinatarios(w http.ResponseWriter, r *http.Request) error {
	p, err := lerPagina(r)
	if err != nil {
		return err
	}
	q := r.URL.Query()
	if e := q.Get("estado"); e != "" && !estadosDest[e] {
		return erros.Campo("estado", "Estado de destinatário inválido.")
	}
	lista, prox, err := s.o.Servicos.Disparos.Destinatarios(r.Context(), r.PathValue("id"),
		armazenamento.FiltroDestinatarios{Estado: q.Get("estado"), Busca: q.Get("busca")}, p)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, NovaPagina(lista, prox))
}

var naoSeguro = regexp.MustCompile(`[^\p{L}\p{N}._-]+`)

func (s *Servidor) relatorioDisparo(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	d, err := s.o.Servicos.Disparos.Obter(r.Context(), id)
	if err != nil {
		return err
	}
	nome := strings.Trim(naoSeguro.ReplaceAllString(d.Nome, "-"), "-")
	if nome == "" {
		nome = "disparo"
	}
	arquivo := fmt.Sprintf("zapdesk-%s.csv", nome)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(arquivo))
	pr, pw := io.Pipe()
	go func() {
		_, err := s.o.Servicos.Disparos.RelatorioCSV(r.Context(), id, pw)
		pw.CloseWithError(err)
	}()
	w.WriteHeader(200)
	_, err = io.Copy(w, pr)
	return nil
}
