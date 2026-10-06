package api

import (
	"net/http"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/automacoes/simulador"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
)

// Rotas de chatbot: simulador e sessões (contracts/api-http.md › Simulador de chatbot).
func (s *Servidor) registrarRotasChatbot() {
	s.rota("POST /v1/automacoes/{id}/simulador", s.iniciarSimulador, false)
	s.rota("POST /v1/simulador/{id}/mensagens", s.mensagemSimulador, false)
	s.rota("DELETE /v1/simulador/{id}", s.encerrarSimulador, false)
	s.rota("GET /v1/automacoes/{id}/sessoes", s.listarSessoes, false)
}

func (s *Servidor) iniciarSimulador(w http.ResponseWriter, r *http.Request) error {
	var p simulador.Pedido
	if err := lerJSON(w, r, &p); err != nil {
		return err
	}
	e, err := s.o.Servicos.Simulador.Iniciar(r.Context(), r.PathValue("id"), p)
	if err != nil {
		return err
	}
	return responderJSON(w, 201, e)
}

func (s *Servidor) mensagemSimulador(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Texto string `json:"texto"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	e, err := s.o.Servicos.Simulador.Mensagem(r.Context(), r.PathValue("id"), c.Texto)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, e)
}

func (s *Servidor) encerrarSimulador(w http.ResponseWriter, r *http.Request) error {
	if err := s.o.Servicos.Simulador.Encerrar(r.PathValue("id")); err != nil {
		return err
	}
	return semConteudo(w)
}

func (s *Servidor) listarSessoes(w http.ResponseWriter, r *http.Request) error {
	p, err := lerPagina(r)
	if err != nil {
		return err
	}
	id := r.PathValue("id")
	if _, err := armazenamento.ObterAutomacao(r.Context(), s.o.Servicos.Chat.Banco(), id); err != nil {
		return err
	}
	estado := r.URL.Query().Get("estado")
	switch estado {
	case "", dominio.SessaoAtiva, dominio.SessaoConcluida, dominio.SessaoHumano, dominio.SessaoExpirada, dominio.SessaoAbortada:
	default:
		return erros.Campo("estado", "Estado de sessão inválido.")
	}
	l, prox, err := armazenamento.ListarSessoes(r.Context(), s.o.Servicos.Chat.Banco(), id, estado, p)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, NovaPagina(l, prox))
}
