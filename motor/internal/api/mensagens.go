package api

import (
	"net/http"
	"strconv"

	"zapdesk/motor/internal/chat"
	"zapdesk/motor/internal/erros"
)

func (s *Servidor) registrarRotasMensagens() {
	s.rota("POST /v1/mensagens/{id}/reenviar", s.reenviarMensagem, false)
	s.rota("POST /v1/mensagens/{id}/reacao", s.reagirMensagem, false)
	s.rota("PATCH /v1/mensagens/{id}", s.editarMensagem, false)
	s.rota("DELETE /v1/mensagens/{id}", s.apagarMensagem, false)
	s.rota("GET /v1/mensagens/{id}/midia", s.midiaMensagem, true)
	s.rota("GET /v1/contas/{id}/figurinhas", s.figurinhas, false)
}

func (s *Servidor) enviarMensagem(w http.ResponseWriter, r *http.Request) error {
	var e chat.Envio
	if err := lerJSON(w, r, &e); err != nil {
		return err
	}
	m, err := s.o.Servicos.Chat.Enviar(r.Context(), r.PathValue("id"), e)
	if err != nil {
		return err
	}
	return responderJSON(w, 202, m)
}

func (s *Servidor) reenviarMensagem(w http.ResponseWriter, r *http.Request) error {
	m, err := s.o.Servicos.Chat.Reenviar(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return responderJSON(w, 202, m)
}

func (s *Servidor) reagirMensagem(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Emoji *string `json:"emoji"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	if c.Emoji == nil {
		return erros.Campo("emoji", "Informe o emoji (ou \"\" para remover).")
	}
	if err := s.o.Servicos.Chat.Reagir(r.Context(), r.PathValue("id"), *c.Emoji); err != nil {
		return err
	}
	return semConteudo(w)
}

func (s *Servidor) editarMensagem(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Texto string `json:"texto"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	m, err := s.o.Servicos.Chat.Editar(r.Context(), r.PathValue("id"), c.Texto)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, m)
}

func (s *Servidor) apagarMensagem(w http.ResponseWriter, r *http.Request) error {
	if err := s.o.Servicos.Chat.Apagar(r.Context(), r.PathValue("id")); err != nil {
		return err
	}
	return semConteudo(w)
}

func (s *Servidor) midiaMensagem(w http.ResponseWriter, r *http.Request) error {
	caminho, mt, err := s.o.Servicos.Chat.ConteudoMidia(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return servirArquivo(w, r, caminho, mt, "")
}

func (s *Servidor) figurinhas(w http.ResponseWriter, r *http.Request) error {
	limite := 30
	if v := r.URL.Query().Get("limite"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > LimiteMaximo {
			return erros.Campo("limite", "O limite deve ser um número de 1 a 200.")
		}
		limite = n
	}
	lista, err := s.o.Servicos.Chat.Figurinhas(r.Context(), r.PathValue("id"), limite)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, lista)
}
