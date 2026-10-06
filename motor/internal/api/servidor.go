// Pacote api expõe o motor por HTTP/JSON e WebSocket em 127.0.0.1 (contracts/api-http.md,
// eventos-ws.md, runtime.md › Modo falso). Todas as rotas exigem token; o Host é conferido
// contra DNS rebinding.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/relogio"
	"zapdesk/motor/internal/whatsapp/falso"
)

// ErrPortaOcupada indica que a porta pedida já está em uso.
var ErrPortaOcupada = errors.New("porta ocupada")

// Opcoes reúne as dependências do servidor.
type Opcoes struct {
	Versao       string
	PastaDados   string
	CaminhoLogs  string
	ModoWhatsApp string // real | falso
	Token        string
	Log          zerolog.Logger
	Barramento   *eventos.Barramento
	Relogio      relogio.Relogio

	// Somente no modo falso.
	Falso              *falso.Controle
	RelogioControlavel *relogio.Controlavel

	// Encerrar pede o encerramento gracioso do motor (POST /v1/sistema/encerrar).
	Encerrar func()
	// Energia recebe "suspender" | "retomar" (POST /v1/sistema/energia).
	Energia func(evento string)

	Servicos Servicos
}

// Servidor HTTP do motor.
type Servidor struct {
	o     Opcoes
	mux   *http.ServeMux
	porta atomic.Int64
	srv   *http.Server
	ln    net.Listener
}

// Novo monta o servidor e registra as rotas.
func Novo(o Opcoes) *Servidor {
	if o.Relogio == nil {
		o.Relogio = relogio.Real{}
	}
	s := &Servidor{o: o, mux: http.NewServeMux()}
	s.registrarRotas()
	return s
}

// Escutar abre 127.0.0.1:<porta> (0 = aleatória). Nunca escuta em outra interface.
func (s *Servidor) Escutar(porta int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", porta))
	if err != nil {
		if strings.Contains(err.Error(), "address already in use") {
			return ErrPortaOcupada
		}
		return err
	}
	s.ln = ln
	s.DefinirPorta(ln.Addr().(*net.TCPAddr).Port)
	s.srv = &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	return nil
}

// DefinirPorta informa a porta usada na checagem de Host (útil com httptest).
func (s *Servidor) DefinirPorta(p int) { s.porta.Store(int64(p)) }

// Porta devolve a porta em uso.
func (s *Servidor) Porta() int { return int(s.porta.Load()) }

// Handler devolve o handler completo (middlewares + rotas).
func (s *Servidor) Handler() http.Handler {
	return s.registroAcesso(s.conferirHost(s.autenticar(s.mux)))
}

// Servir atende no listener aberto por Escutar (bloqueia).
func (s *Servidor) Servir() error {
	err := s.srv.Serve(s.ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Encerrar fecha o servidor (conexões WS são derrubadas).
func (s *Servidor) Encerrar(ctx context.Context) error {
	if s.srv == nil {
		if s.ln != nil {
			return s.ln.Close()
		}
		return nil
	}
	err := s.srv.Shutdown(ctx)
	if err != nil {
		s.srv.Close()
	}
	return err
}

// rota registra um padrão do ServeMux. `tokenNaQuery` libera ?token= (GET binário e WS).
func (s *Servidor) rota(padrao string, h func(w http.ResponseWriter, r *http.Request) error, tokenNaQuery bool) {
	s.mux.HandleFunc(padrao, func(w http.ResponseWriter, r *http.Request) {
		if !autorizado(r, s.o.Token, tokenNaQuery) {
			responderErroCodigo(w, erros.NaoAutorizado, "", nil)
			return
		}
		if err := h(w, r); err != nil {
			s.responderErro(w, r, err)
		}
	})
}

// ---------------------------------------------------------------------------
// Utilidades de requisição/resposta
// ---------------------------------------------------------------------------

// LimiteCorpoJSON é o tamanho máximo de corpo JSON.
const LimiteCorpoJSON = 8 << 20

func responderJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func semConteudo(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// lerJSON decodifica o corpo em v. Corpo vazio é aceito (v fica com os zeros).
func lerJSON(w http.ResponseWriter, r *http.Request, v any) error {
	corpo := http.MaxBytesReader(w, r.Body, LimiteCorpoJSON)
	dados, err := io.ReadAll(corpo)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(dados))) == 0 {
		return nil
	}
	if err := json.Unmarshal(dados, v); err != nil {
		return erros.Novo(erros.Validacao, "JSON inválido: "+err.Error())
	}
	return nil
}
