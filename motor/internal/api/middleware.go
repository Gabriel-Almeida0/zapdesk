package api

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"
	"time"

	"zapdesk/motor/internal/erros"
)

// conferirHost aceita só Host = 127.0.0.1:<porta> (ou localhost:<porta>) contra DNS rebinding.
func (s *Servidor) conferirHost(prox http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		porta := strconv.Itoa(s.Porta())
		if r.Host != "127.0.0.1:"+porta && r.Host != "localhost:"+porta {
			responderErroCodigo(w, erros.HostInvalido, "", nil)
			return
		}
		prox.ServeHTTP(w, r)
	})
}

// autenticar rejeita rotas não registradas sem token (as registradas conferem em rota()).
func (s *Servidor) autenticar(prox http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// O token é conferido por rota (para saber se ?token= é aceito). Aqui só garantimos que
		// nenhuma resposta (nem 404) sai sem algum token válido.
		if !autorizado(r, s.o.Token, true) {
			responderErroCodigo(w, erros.NaoAutorizado, "", nil)
			return
		}
		prox.ServeHTTP(w, r)
	})
}

// autorizado compara o token em tempo constante.
func autorizado(r *http.Request, token string, aceitaQuery bool) bool {
	recebido := ""
	if cab := r.Header.Get("Authorization"); strings.HasPrefix(cab, "Bearer ") {
		recebido = strings.TrimPrefix(cab, "Bearer ")
	} else if aceitaQuery && r.Method == http.MethodGet {
		recebido = r.URL.Query().Get("token")
	}
	if recebido == "" || token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(recebido), []byte(token)) == 1
}

type gravadorStatus struct {
	http.ResponseWriter
	status int
}

func (g *gravadorStatus) WriteHeader(s int) {
	g.status = s
	g.ResponseWriter.WriteHeader(s)
}

// Unwrap permite ao http.ResponseController (e ao WebSocket) achar o writer original.
func (g *gravadorStatus) Unwrap() http.ResponseWriter { return g.ResponseWriter }

// registroAcesso loga método, caminho (SEM query string: o ?token= nunca vai para o log),
// status e duração.
func (s *Servidor) registroAcesso(prox http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		g := &gravadorStatus{ResponseWriter: w, status: 200}
		prox.ServeHTTP(g, r)
		s.o.Log.Debug().Str("metodo", r.Method).Str("caminho", r.URL.Path).Int("status", g.status).
			Dur("duracao", time.Since(inicio)).Msg("http")
	})
}
