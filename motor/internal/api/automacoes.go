package api

import (
	"io"
	"net/http"
	"strconv"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/automacoes"
	"zapdesk/motor/internal/erros"
)

// Rotas de automações (contracts/api-http.md › Automações (todas)).
func (s *Servidor) registrarRotasAutomacoes() {
	s.rota("GET /v1/automacoes", s.listarAutomacoes, false)
	s.rota("POST /v1/automacoes", s.criarAutomacao, false)
	s.rota("POST /v1/automacoes/validar", s.validarAutomacao, false)
	s.rota("GET /v1/automacoes/{id}", s.obterAutomacao, false)
	s.rota("PATCH /v1/automacoes/{id}", s.editarAutomacao, false)
	s.rota("DELETE /v1/automacoes/{id}", s.excluirAutomacao, false)
	s.rota("POST /v1/automacoes/{id}/ativar", s.ativarAutomacao, false)
	s.rota("POST /v1/automacoes/{id}/desativar", s.desativarAutomacao, false)
	s.rota("POST /v1/automacoes/{id}/executar", s.executarAutomacao, false)
	s.rota("POST /v1/automacoes/{id}/testar", s.testarAutomacao, false)
	if s.o.Servicos.AutomacoesIA != nil {
		s.registrarRotasAutomacoesIA()
	}
	if s.o.Servicos.Simulador != nil {
		s.registrarRotasChatbot()
	}
	for _, f := range registrosAutomacoesExtras {
		f(s)
	}
}

// registrosAutomacoesExtras permite às histórias seguintes (chatbot, IA) acrescentarem rotas.
var registrosAutomacoesExtras []func(s *Servidor)

func lerCorpo(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	return io.ReadAll(http.MaxBytesReader(w, r.Body, LimiteCorpoJSON))
}

func (s *Servidor) listarAutomacoes(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	f := armazenamento.FiltroAutomacoes{Tipo: q.Get("tipo"), Busca: q.Get("busca")}
	if v := q.Get("ativa"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return erros.Campo("ativa", "Use true ou false.")
		}
		f.Ativa = &b
	}
	l, err := s.aut().Listar(r.Context(), f)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, l)
}

func (s *Servidor) entrada(w http.ResponseWriter, r *http.Request) (automacoes.Entrada, error) {
	b, err := lerCorpo(w, r)
	if err != nil {
		return automacoes.Entrada{}, err
	}
	return automacoes.LerEntrada(b)
}

func (s *Servidor) criarAutomacao(w http.ResponseWriter, r *http.Request) error {
	e, err := s.entrada(w, r)
	if err != nil {
		return err
	}
	a, err := s.aut().Criar(r.Context(), e)
	if err != nil {
		return err
	}
	return responderJSON(w, 201, a)
}

func (s *Servidor) validarAutomacao(w http.ResponseWriter, r *http.Request) error {
	e, err := s.entrada(w, r)
	if err != nil {
		return err
	}
	errs, avisos, err := s.aut().Validar(r.Context(), e)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, map[string]any{"erros": errs, "avisos": avisos})
}

func (s *Servidor) obterAutomacao(w http.ResponseWriter, r *http.Request) error {
	a, err := s.aut().Obter(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, a)
}

func (s *Servidor) editarAutomacao(w http.ResponseWriter, r *http.Request) error {
	e, err := s.entrada(w, r)
	if err != nil {
		return err
	}
	a, err := s.aut().Editar(r.Context(), r.PathValue("id"), e)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, a)
}

func (s *Servidor) excluirAutomacao(w http.ResponseWriter, r *http.Request) error {
	if err := s.aut().Excluir(r.Context(), r.PathValue("id")); err != nil {
		return err
	}
	return semConteudo(w)
}

func (s *Servidor) ativarAutomacao(w http.ResponseWriter, r *http.Request) error {
	a, err := s.aut().Ativar(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, a)
}

func (s *Servidor) desativarAutomacao(w http.ResponseWriter, r *http.Request) error {
	a, err := s.aut().Desativar(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, a)
}

func (s *Servidor) executarAutomacao(w http.ResponseWriter, r *http.Request) error {
	var p automacoes.PedidoExecucao
	if err := lerJSON(w, r, &p); err != nil {
		return err
	}
	e, err := s.aut().Executar(r.Context(), r.PathValue("id"), p)
	if err != nil {
		return err
	}
	return responderJSON(w, 202, e)
}

func (s *Servidor) testarAutomacao(w http.ResponseWriter, r *http.Request) error {
	var p automacoes.PedidoTeste
	if err := lerJSON(w, r, &p); err != nil {
		return err
	}
	e, err := s.aut().Testar(r.Context(), r.PathValue("id"), p)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, map[string]any{"execucao": e})
}
