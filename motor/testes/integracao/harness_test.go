// Harness de integração: motor montado em pasta temporária com WhatsApp falso e relógio
// controlável, servido por httptest em 127.0.0.1; cliente HTTP com token; esperarEvento via WS;
// reiniciar() na mesma pasta.
package integracao

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/rs/zerolog"

	"zapdesk/motor/internal/aplicacao"
	"zapdesk/motor/internal/relogio"
	"zapdesk/motor/internal/whatsapp"
)

const tokenTeste = "token-de-teste-com-mais-de-32-caracteres!!"

// inicioPadrao é uma segunda-feira às 10h (dentro de janelas comerciais).
var inicioPadrao = time.Date(2026, 9, 28, 10, 0, 0, 0, time.Local)

type opcoesMotor struct {
	modo      string // falso (padrão) | real
	corrente  bool   // relógio andando em tempo real
	religado  bool
	fabrica   whatsapp.Fabrica
	pasta     string
	relogioIn *relogio.Controlavel
	log       *zerolog.Logger
	// Feature 002.
	runnerExec, runnerScript string
	aguardarSegredos         bool
	iaReal                   bool
	urlClaude                string
}

type Evento struct {
	Seq     int64           `json:"seq"`
	Tipo    string          `json:"tipo"`
	ContaID *string         `json:"conta_id"`
	Em      string          `json:"em"`
	Dados   json.RawMessage `json:"dados"`
}

type Motor struct {
	t       *testing.T
	o       opcoesMotor
	Pasta   string
	App     *aplicacao.App
	Relogio *relogio.Controlavel
	srv     *httptest.Server
	Base    string
	cli     *http.Client

	mu      sync.Mutex
	eventos []Evento
	novo    chan struct{}
	wsFim   context.CancelFunc
}

func novoMotor(t *testing.T, opcoes ...func(*opcoesMotor)) *Motor {
	t.Helper()
	o := opcoesMotor{modo: "falso"}
	for _, f := range opcoes {
		f(&o)
	}
	if o.pasta == "" {
		o.pasta = t.TempDir()
	}
	m := &Motor{t: t, o: o, Pasta: o.pasta, cli: &http.Client{Timeout: 30 * time.Second}}
	m.subir()
	t.Cleanup(m.parar)
	return m
}

func comRelogioCorrente(o *opcoesMotor) { o.corrente = true }
func modoReal(f whatsapp.Fabrica) func(*opcoesMotor) {
	return func(o *opcoesMotor) { o.modo = "real"; o.fabrica = f }
}

func (m *Motor) subir() {
	t := m.t
	t.Helper()
	if m.o.modo == "falso" {
		if m.Relogio == nil {
			if m.o.corrente {
				m.Relogio = relogio.NovoControlavel(inicioPadrao)
			} else {
				m.Relogio = relogio.NovoCongelado(inicioPadrao)
			}
		}
	}
	opc := aplicacao.Opcoes{
		Versao: "0.0.0-teste", PastaDados: m.Pasta, CaminhoLogs: m.Pasta + "/logs/motor.log",
		ModoWhatsApp: m.o.modo, Token: tokenTeste, Religado: m.o.religado, Log: logTeste(),
		IA: map[bool]string{true: "real", false: "falsa"}[m.o.iaReal], RunnerExec: m.o.runnerExec, RunnerScript: m.o.runnerScript, AguardarSegredos: m.o.aguardarSegredos,
		URLClaude: m.o.urlClaude,
	}
	if m.Relogio != nil {
		opc.Relogio = m.Relogio
	}
	if m.o.log != nil {
		opc.Log = *m.o.log
	}
	if m.o.modo == "real" {
		f := m.o.fabrica
		opc.FabricaReal = func(string, zerolog.Logger) (whatsapp.Fabrica, error) { return f, nil }
	}
	app, err := aplicacao.Montar(context.Background(), opc)
	if err != nil {
		t.Fatalf("montar motor: %v", err)
	}
	m.App = app
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	m.srv = httptest.NewUnstartedServer(app.Handler())
	m.srv.Listener.Close()
	m.srv.Listener = ln
	m.srv.Start()
	app.Servidor.DefinirPorta(ln.Addr().(*net.TCPAddr).Port)
	m.Base = m.srv.URL

	m.mu.Lock()
	m.eventos = nil
	m.novo = make(chan struct{})
	m.mu.Unlock()
	m.conectarWS()
	app.Iniciar()
}

func (m *Motor) conectarWS() {
	ctx, cancelar := context.WithCancel(context.Background())
	m.wsFim = cancelar
	url := strings.Replace(m.Base, "http://", "ws://", 1) + "/v1/eventos?token=" + tokenTeste
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		m.t.Fatalf("ws: %v", err)
	}
	c.SetReadLimit(64 << 20)
	pronto := make(chan struct{})
	go func() {
		defer c.CloseNow()
		primeiro := true
		for {
			_, dados, err := c.Read(ctx)
			if err != nil {
				return
			}
			var ev Evento
			json.Unmarshal(dados, &ev)
			m.mu.Lock()
			m.eventos = append(m.eventos, ev)
			close(m.novo)
			m.novo = make(chan struct{})
			m.mu.Unlock()
			if primeiro {
				primeiro = false
				close(pronto)
			}
		}
	}()
	select {
	case <-pronto:
	case <-time.After(5 * time.Second):
		m.t.Fatal("ws não recebeu motor.pronto")
	}
}

func (m *Motor) parar() {
	if m.App == nil {
		return
	}
	m.wsFim()
	m.srv.Close()
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	m.App.Encerrar(ctx)
	m.App = nil
}

// reiniciar encerra o motor e sobe outro na mesma pasta (mesmo relógio).
func (m *Motor) reiniciar(religado ...bool) {
	m.t.Helper()
	m.parar()
	m.o.religado = len(religado) > 0 && religado[0]
	m.subir()
}

// esperarEvento espera um evento do tipo (e que satisfaça o filtro) recebido após o índice
// `desde` (use m.marca()).
func (m *Motor) esperarEvento(desde int, tipo string, filtro func(Evento) bool) Evento {
	m.t.Helper()
	limite := time.After(10 * time.Second)
	for i := desde; ; {
		m.mu.Lock()
		for ; i < len(m.eventos); i++ {
			ev := m.eventos[i]
			if ev.Tipo == tipo && (filtro == nil || filtro(ev)) {
				m.mu.Unlock()
				return ev
			}
		}
		novo := m.novo
		m.mu.Unlock()
		select {
		case <-novo:
		case <-limite:
			m.t.Fatalf("evento %s não chegou", tipo)
		}
	}
}

// marca devolve a posição atual do log de eventos.
func (m *Motor) marca() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.eventos)
}

func (m *Motor) eventosDoTipo(tipo string) []Evento {
	m.mu.Lock()
	defer m.mu.Unlock()
	var l []Evento
	for _, e := range m.eventos {
		if e.Tipo == tipo {
			l = append(l, e)
		}
	}
	return l
}

// req faz uma requisição com token e devolve status e corpo.
func (m *Motor) req(metodo, caminho string, corpo any, cabecalhos ...string) (int, []byte) {
	m.t.Helper()
	var leitor io.Reader
	if corpo != nil {
		if b, ok := corpo.([]byte); ok {
			leitor = bytes.NewReader(b)
		} else {
			j, _ := json.Marshal(corpo)
			leitor = bytes.NewReader(j)
		}
	}
	r, _ := http.NewRequest(metodo, m.Base+caminho, leitor)
	r.Header.Set("Authorization", "Bearer "+tokenTeste)
	if corpo != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(cabecalhos); i += 2 {
		if cabecalhos[i] == "Host" {
			r.Host = cabecalhos[i+1]
		} else {
			r.Header.Set(cabecalhos[i], cabecalhos[i+1])
		}
	}
	resp, err := m.cli.Do(r)
	if err != nil {
		m.t.Fatalf("%s %s: %v", metodo, caminho, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// json faz a requisição, confere o status esperado e decodifica em `saida` (pode ser nil).
func (m *Motor) json(metodo, caminho string, corpo any, esperado int, saida any) {
	m.t.Helper()
	status, b := m.req(metodo, caminho, corpo)
	if status != esperado {
		m.t.Fatalf("%s %s: status %d (esperado %d): %s", metodo, caminho, status, esperado, b)
	}
	if saida != nil {
		if err := json.Unmarshal(b, saida); err != nil {
			m.t.Fatalf("%s %s: json: %v: %s", metodo, caminho, err, b)
		}
	}
}

// erro faz a requisição e confere status e código de erro.
func (m *Motor) erro(metodo, caminho string, corpo any, status int, codigo string) map[string]any {
	m.t.Helper()
	st, b := m.req(metodo, caminho, corpo)
	var c struct {
		Erro struct {
			Codigo   string         `json:"codigo"`
			Mensagem string         `json:"mensagem"`
			Detalhes map[string]any `json:"detalhes"`
		} `json:"erro"`
	}
	json.Unmarshal(b, &c)
	if st != status || c.Erro.Codigo != codigo {
		m.t.Fatalf("%s %s: esperado %d %s, veio %d %s", metodo, caminho, status, codigo, st, b)
	}
	if c.Erro.Mensagem == "" {
		m.t.Fatalf("erro sem mensagem: %s", b)
	}
	return map[string]any{"mensagem": c.Erro.Mensagem, "detalhes": c.Erro.Detalhes}
}

func (m *Motor) url(caminho string) string { return fmt.Sprintf("%s%s", m.Base, caminho) }
