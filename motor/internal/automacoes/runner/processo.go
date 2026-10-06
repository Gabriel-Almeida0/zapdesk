package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Prazos do ciclo de vida (runner-protocolo.md › Ciclo de vida).
const (
	PrazoPronto      = 5 * time.Second
	PrazoInicializar = 10 * time.Second
	PrazoEncerrar    = 2 * time.Second
	FolgaMorte       = 2 * time.Second
	LinhasStderr     = 200
)

// execucao em andamento num processo.
type execucao struct {
	id    string
	ponte Ponte
	ctx   context.Context
}

// processo é um runner vivo (ou iniciando) de uma (automação, hash).
type processo struct {
	pool  *Pool
	chave string
	aut   Automacao

	cmd   *exec.Cmd
	stdin io.WriteCloser
	pid   int
	esc   sync.Mutex

	iniciado chan struct{} // fechado ao fim de iniciar (ok ou erro)
	erroInit error
	pronto   chan struct{}
	umPronto sync.Once
	morto    chan struct{} // fechado depois do Wait
	codigo   int

	mu        sync.Mutex
	proxID    int64
	pendentes map[int64]chan *mensagem
	execs     map[string]*execucao
	stderr    []string
	estourou  string // execução que excedeu o prazo (processo morto por ela)
	fechado   bool   // encerrado pelo pool

	// Protegidos por pool.mu.
	ocupado       int
	obsoleto      bool
	fechando      bool
	geracaoOcioso int
	cancelOcioso  context.CancelFunc
}

func novoProcesso(p *Pool, a Automacao) *processo {
	return &processo{pool: p, chave: chaveDe(a), aut: a, iniciado: make(chan struct{}), pronto: make(chan struct{}),
		morto: make(chan struct{}), pendentes: map[int64]chan *mensagem{}, execs: map[string]*execucao{}}
}

func chaveDe(a Automacao) string { return a.ID + "|" + a.Hash }

// fusoLocal devolve o nome IANA do fuso do motor.
func fusoLocal() string {
	if n := time.Local.String(); n != "" && n != "Local" {
		return n
	}
	if tz := os.Getenv("TZ"); tz != "" {
		return strings.TrimPrefix(tz, ":")
	}
	if alvo, err := os.Readlink("/etc/localtime"); err == nil {
		if i := strings.Index(alvo, "zoneinfo/"); i >= 0 {
			return alvo[i+len("zoneinfo/"):]
		}
	}
	return "UTC"
}

// ambiente limpo do runner: nada além de TZ, LANG e (Electron) ELECTRON_RUN_AS_NODE.
func (pr *processo) ambiente() []string {
	fuso := pr.pool.o.Fuso
	if fuso == "" {
		fuso = fusoLocal()
	}
	env := []string{"TZ=" + fuso, "LANG=pt_BR.UTF-8"}
	base := strings.ToLower(filepath.Base(pr.pool.o.Exec))
	if base != "node" && base != "node.exe" {
		env = append(env, "ELECTRON_RUN_AS_NODE=1")
	}
	return env
}

func (pr *processo) memoriaMB() int {
	if pr.aut.MemoriaMB > 0 {
		return pr.aut.MemoriaMB
	}
	return 256
}

// iniciar sobe o processo, espera "pronto" e faz "inicializar".
func (pr *processo) iniciar() error {
	o := pr.pool.o
	// O Permission Model do Node compara caminhos reais (no macOS /var → /private/var): passe os
	// caminhos já resolvidos nas flags e no "inicializar".
	script := caminhoReal(o.Script)
	pr.aut.Bundle = caminhoReal(pr.aut.Bundle)
	pr.cmd = exec.Command(o.Exec, "--permission", "--allow-fs-read="+script, "--allow-fs-read="+pr.aut.Bundle,
		fmt.Sprintf("--max-old-space-size=%d", pr.memoriaMB()), script)
	pr.cmd.Env = pr.ambiente()
	pr.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdin, err := pr.cmd.StdinPipe()
	if err != nil {
		return pr.falhaInicio(err)
	}
	stdout, err := pr.cmd.StdoutPipe()
	if err != nil {
		return pr.falhaInicio(err)
	}
	stderr, err := pr.cmd.StderrPipe()
	if err != nil {
		return pr.falhaInicio(err)
	}
	if err := pr.cmd.Start(); err != nil {
		return pr.falhaInicio(err)
	}
	pr.stdin, pr.pid = stdin, pr.cmd.Process.Pid
	if !pr.pool.registrarVivo(pr) {
		pr.matar()
	}
	fimErr := make(chan struct{})
	go func() { pr.lerStderr(stderr); close(fimErr) }()
	go func() {
		pr.lerStdout(stdout)
		<-fimErr
		err := pr.cmd.Wait()
		pr.mu.Lock()
		pr.codigo = pr.cmd.ProcessState.ExitCode()
		if err != nil && pr.codigo == -1 {
			if st, ok := pr.cmd.ProcessState.Sys().(syscall.WaitStatus); ok && st.Signaled() {
				pr.codigo = 128 + int(st.Signal())
			}
		}
		pr.mu.Unlock()
		close(pr.morto)
		pr.pool.aoMorrer(pr)
	}()

	select {
	case <-pr.pronto:
	case <-pr.morto:
		return pr.falhaInicio(errors.New("o processo saiu antes de ficar pronto"))
	case <-time.After(PrazoPronto):
		pr.matar()
		return pr.falhaInicio(errors.New("sem sinal de pronto em 5 s"))
	}
	params := map[string]any{"protocolo": 1,
		"automacao": map[string]any{"id": pr.aut.ID, "nome": pr.aut.Nome, "versao": pr.aut.Versao},
		"bundle":    pr.aut.Bundle, "hash": pr.aut.Hash, "permissoes": naoNulo(pr.aut.Permissoes),
		"segredos": segredosNaoNulos(pr.aut.Segredos), "fuso": pr.ambienteFuso()}
	_, ch, err := pr.requisitar("inicializar", params)
	if err != nil {
		pr.matar()
		return pr.falhaInicio(err)
	}
	select {
	case m := <-ch:
		if m.Error != nil {
			pr.matar()
			return pr.falhaInicio(fmt.Errorf("%s", m.Error.Mensagem))
		}
		return nil
	case <-pr.morto:
		return pr.falhaInicio(errors.New("o processo saiu durante a inicialização"))
	case <-time.After(PrazoInicializar):
		pr.matar()
		return pr.falhaInicio(errors.New("inicialização passou de 10 s"))
	}
}

func (pr *processo) ambienteFuso() string {
	for _, e := range pr.ambiente() {
		if strings.HasPrefix(e, "TZ=") {
			return e[3:]
		}
	}
	return "UTC"
}

func naoNulo(l []string) []string {
	if l == nil {
		return []string{}
	}
	return l
}

func segredosNaoNulos(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func (pr *processo) falhaInicio(causa error) error {
	msg := "O processo da automação não iniciou"
	if causa != nil {
		msg += ": " + causa.Error()
	}
	return &ErroRunner{Mensagem: msg, Stderr: pr.ultimasStderr(20)}
}

// escrever envia uma linha JSON.
func (pr *processo) escrever(m *mensagem) error {
	m.JSONRPC = "2.0"
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	pr.esc.Lock()
	defer pr.esc.Unlock()
	if pr.stdin == nil {
		return errors.New("processo sem stdin")
	}
	_, err = pr.stdin.Write(append(b, '\n'))
	return err
}

// requisitar envia uma requisição e devolve o canal da resposta.
func (pr *processo) requisitar(metodo string, params any) (int64, chan *mensagem, error) {
	p, err := json.Marshal(params)
	if err != nil {
		return 0, nil, err
	}
	ch := make(chan *mensagem, 1)
	pr.mu.Lock()
	pr.proxID++
	id := pr.proxID
	pr.pendentes[id] = ch
	pr.mu.Unlock()
	if err := pr.escrever(&mensagem{ID: &id, Metodo: metodo, Params: p}); err != nil {
		pr.mu.Lock()
		delete(pr.pendentes, id)
		pr.mu.Unlock()
		return 0, nil, err
	}
	return id, ch, nil
}

func (pr *processo) esquecer(id int64) {
	pr.mu.Lock()
	delete(pr.pendentes, id)
	pr.mu.Unlock()
}

// notificar envia uma notificação (sem resposta).
func (pr *processo) notificar(metodo string, params any) {
	p, _ := json.Marshal(params)
	pr.escrever(&mensagem{Metodo: metodo, Params: p})
}

func (pr *processo) responder(id int64, resultado json.RawMessage, erro *ErroRPC) {
	m := &mensagem{ID: &id}
	if erro != nil {
		m.Error = erro
	} else {
		if len(resultado) == 0 {
			resultado = json.RawMessage("null")
		}
		m.Result = resultado
	}
	pr.escrever(m)
}

// lerLinha lê uma linha; linhas maiores que o limite são descartadas (devolve o começo em
// `prefixo` e grande=true).
func lerLinha(r *bufio.Reader, limite int) (linha []byte, grande bool, err error) {
	var buf []byte
	for {
		parte, e := r.ReadSlice('\n')
		if !grande {
			if len(buf)+len(parte) > limite {
				grande = true
				if len(buf) < 512 {
					buf = append(buf, parte[:min(len(parte), 512)]...)
				}
			} else {
				buf = append(buf, parte...)
			}
		}
		if e == bufio.ErrBufferFull {
			continue
		}
		return buf, grande, e
	}
}

var reID = regexp.MustCompile(`"id"\s*:\s*(\d+)`)

func (pr *processo) lerStdout(r io.Reader) {
	leitor := bufio.NewReaderSize(r, 64<<10)
	for {
		linha, grande, err := lerLinha(leitor, LimiteLinha)
		if grande {
			pr.linhaGrande(linha)
		} else if len(bytes.TrimSpace(linha)) > 0 {
			pr.tratar(linha)
		}
		if err != nil {
			return
		}
	}
}

// linhaGrande: requisição → responde -32600; resposta a uma requisição nossa → falha com erro.
func (pr *processo) linhaGrande(prefixo []byte) {
	sub := reID.FindSubmatch(prefixo)
	if sub == nil {
		return
	}
	id, _ := strconv.ParseInt(string(sub[1]), 10, 64)
	erro := &ErroRPC{Codigo: CodigoRequisicaoInvalida, Mensagem: "Linha maior que 1 MB."}
	if bytes.Contains(prefixo, []byte(`"method"`)) {
		pr.responder(id, nil, erro)
		return
	}
	pr.mu.Lock()
	ch := pr.pendentes[id]
	delete(pr.pendentes, id)
	pr.mu.Unlock()
	if ch != nil {
		ch <- &mensagem{ID: &id, Error: erro}
	}
}

func (pr *processo) tratar(linha []byte) {
	var m mensagem
	if err := json.Unmarshal(linha, &m); err != nil {
		pr.pool.o.Log.Warn().Str("automacao", pr.aut.ID).Msg("linha inválida do runner")
		return
	}
	switch {
	case m.Metodo != "" && m.ID != nil:
		go pr.atenderRequisicao(*m.ID, m.Metodo, m.Params)
	case m.Metodo != "":
		pr.atenderNotificacao(m.Metodo, m.Params)
	case m.ID != nil:
		pr.mu.Lock()
		ch := pr.pendentes[*m.ID]
		delete(pr.pendentes, *m.ID)
		pr.mu.Unlock()
		if ch != nil {
			ch <- &m
		}
	}
}

func (pr *processo) execucao(id string) *execucao {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	return pr.execs[id]
}

func (pr *processo) atenderRequisicao(id int64, metodo string, params json.RawMessage) {
	if !strings.HasPrefix(metodo, "ctx.") {
		pr.responder(id, nil, &ErroRPC{Codigo: CodigoMetodoDesconhecido, Mensagem: "Método desconhecido: " + metodo})
		return
	}
	var p struct {
		ExecucaoID string `json:"execucao_id"`
	}
	json.Unmarshal(params, &p)
	ex := pr.execucao(p.ExecucaoID)
	if ex == nil {
		pr.responder(id, nil, &ErroRPC{Codigo: CodigoExecucaoEncerrada, Mensagem: "A execução já terminou.",
			Dados: map[string]any{"codigo": "execucao_encerrada"}})
		return
	}
	res, erro := ex.ponte.Chamar(ex.ctx, p.ExecucaoID, metodo, params)
	if erro != nil {
		pr.responder(id, nil, erro)
		return
	}
	pr.responder(id, res, nil)
}

func (pr *processo) atenderNotificacao(metodo string, params json.RawMessage) {
	switch metodo {
	case "pronto":
		pr.umPronto.Do(func() { close(pr.pronto) })
	case "log":
		var p struct {
			ExecucaoID *string `json:"execucao_id"`
			Nivel      string  `json:"nivel"`
			Texto      string  `json:"texto"`
			Em         string  `json:"em"`
		}
		if json.Unmarshal(params, &p) != nil || p.ExecucaoID == nil {
			return
		}
		if ex := pr.execucao(*p.ExecucaoID); ex != nil {
			em, err := time.Parse(time.RFC3339Nano, p.Em)
			if err != nil {
				em = time.Now()
			}
			ex.ponte.Log(p.ExecucaoID, p.Nivel, p.Texto, em)
		}
	case "http":
		var p struct {
			ExecucaoID  string  `json:"execucao_id"`
			Metodo      string  `json:"metodo"`
			URLSemQuery string  `json:"url_sem_query"`
			Status      *int    `json:"status"`
			DuracaoMs   int64   `json:"duracao_ms"`
			Erro        *string `json:"erro"`
		}
		if json.Unmarshal(params, &p) != nil {
			return
		}
		if ex := pr.execucao(p.ExecucaoID); ex != nil {
			ex.ponte.HTTP(p.ExecucaoID, p.Metodo, p.URLSemQuery, p.Status, p.DuracaoMs, p.Erro)
		}
	}
}

func (pr *processo) lerStderr(r io.Reader) {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 64<<10), 1<<20)
	for s.Scan() {
		pr.mu.Lock()
		pr.stderr = append(pr.stderr, s.Text())
		if len(pr.stderr) > LinhasStderr {
			pr.stderr = pr.stderr[len(pr.stderr)-LinhasStderr:]
		}
		pr.mu.Unlock()
	}
	io.Copy(io.Discard, r)
}

func (pr *processo) ultimasStderr(n int) []string {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	if len(pr.stderr) <= n {
		return append([]string{}, pr.stderr...)
	}
	return append([]string{}, pr.stderr[len(pr.stderr)-n:]...)
}

// matar envia SIGKILL ao grupo do processo.
func (pr *processo) matar() {
	if pr.pid > 0 {
		syscall.Kill(-pr.pid, syscall.SIGKILL)
		if pr.cmd != nil && pr.cmd.Process != nil {
			pr.cmd.Process.Kill()
		}
	}
}

// matarPorTempo registra a execução que estourou o prazo e mata o processo.
func (pr *processo) matarPorTempo(execucaoID string) {
	pr.mu.Lock()
	if pr.estourou == "" {
		pr.estourou = execucaoID
	}
	pr.mu.Unlock()
	pr.matar()
}

// erroMorte descreve o fim do processo para uma execução que estava nele.
func (pr *processo) erroMorte(execucaoID string, prazo time.Duration) error {
	pr.mu.Lock()
	estourou, fechado, codigo := pr.estourou, pr.fechado, pr.codigo
	stderr := strings.Join(pr.stderr, "\n")
	pr.mu.Unlock()
	ultimas := pr.ultimasStderr(20)
	switch {
	case estourou != "" && estourou == execucaoID:
		s := int(math.Round((prazo + FolgaMorte).Seconds()))
		return &ErroRunner{Mensagem: fmt.Sprintf("Tempo limite de %d s excedido", s)}
	case estourou != "":
		return &ErroRunner{Mensagem: "Interrompida porque outra execução desta automação excedeu o tempo"}
	case strings.Contains(stderr, "heap out of memory"):
		return &ErroRunner{Mensagem: fmt.Sprintf("Memória excedida (%d MB)", pr.memoriaMB()), Stderr: ultimas}
	case fechado:
		return &ErroRunner{Mensagem: "A execução foi interrompida porque o processo da automação foi encerrado"}
	}
	return &ErroRunner{Mensagem: fmt.Sprintf("O processo da automação terminou inesperadamente (código %d)", codigo), Stderr: ultimas}
}

// encerrar pede "encerrar", espera até 2 s e mata o grupo; volta depois que o processo saiu.
func (pr *processo) encerrar() {
	pr.mu.Lock()
	pr.fechado = true
	pr.mu.Unlock()
	select {
	case <-pr.iniciado:
	case <-time.After(PrazoPronto + PrazoInicializar):
	}
	if pr.pid == 0 {
		return
	}
	if id, ch, err := pr.requisitar("encerrar", map[string]any{}); err == nil {
		select {
		case <-ch:
		case <-pr.morto:
		case <-time.After(PrazoEncerrar):
		}
		pr.esquecer(id)
	}
	pr.esc.Lock()
	if pr.stdin != nil {
		pr.stdin.Close()
	}
	pr.esc.Unlock()
	select {
	case <-pr.morto:
	case <-time.After(200 * time.Millisecond):
	}
	pr.matar()
	select {
	case <-pr.morto:
	case <-time.After(3 * time.Second):
	}
}

// caminhoReal resolve links simbólicos (o original se não der).
func caminhoReal(c string) string {
	if r, err := filepath.EvalSymlinks(c); err == nil {
		return r
	}
	return c
}
