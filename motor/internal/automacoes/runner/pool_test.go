package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"zapdesk/motor/internal/relogio"
)

// nodeOuPular devolve o node do PATH; sem node o teste é pulado (com ZAPDESK_CI=1, falha).
func nodeOuPular(t *testing.T) string {
	t.Helper()
	n, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ZAPDESK_CI") == "1" {
			t.Fatal("node não encontrado no PATH (obrigatório com ZAPDESK_CI=1)")
		}
		t.Skip("node não encontrado no PATH: testes do pool de runners pulados")
	}
	return n
}

func novoPoolTeste(t *testing.T, rel relogio.Relogio, max int) *Pool {
	t.Helper()
	node := nodeOuPular(t)
	script, _ := filepath.Abs("testdata/runner-teste.mjs")
	p := NovoPool(Opcoes{Exec: node, Script: script, Relogio: rel, Log: zerolog.Nop(), MaxProcessos: max})
	t.Cleanup(func() {
		ctx, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		p.EncerrarTodos(ctx)
	})
	return p
}

func aut(id, hash string) Automacao {
	b, _ := filepath.Abs("testdata/bundle.mjs")
	return Automacao{ID: id, Nome: "Automação " + id, Versao: 1, Hash: hash, Bundle: b, MemoriaMB: 64}
}

var seq atomic.Int64

func pedido(arg map[string]any, prazo time.Duration) Pedido {
	a, _ := json.Marshal(arg)
	return Pedido{ExecucaoID: fmt.Sprintf("x%d", seq.Add(1)), Handler: "aoExecutar", Prazo: prazo, Argumento: a}
}

type ponteTeste struct {
	mu       sync.Mutex
	chamadas []string
	logs     []string
	http     int
}

func (p *ponteTeste) Chamar(_ context.Context, ex, metodo string, params json.RawMessage) (json.RawMessage, *ErroRPC) {
	p.mu.Lock()
	p.chamadas = append(p.chamadas, metodo)
	p.mu.Unlock()
	if strings.Contains(string(params), `"negar"`) {
		return nil, &ErroRPC{Codigo: 1001, Mensagem: "Permissão 'enviar' não declarada em automacao.json", Dados: map[string]any{"codigo": "permissao_negada"}}
	}
	return json.RawMessage(`{"eco":"` + ex + `"}`), nil
}
func (p *ponteTeste) Log(_ *string, nivel, texto string, _ time.Time) {
	p.mu.Lock()
	p.logs = append(p.logs, nivel+":"+texto)
	p.mu.Unlock()
}
func (p *ponteTeste) HTTP(string, string, string, *int, int64, *string) {
	p.mu.Lock()
	p.http++
	p.mu.Unlock()
}

func rodar(t *testing.T, p *Pool, a Automacao, arg map[string]any, prazo time.Duration) (json.RawMessage, error) {
	t.Helper()
	return p.Executar(context.Background(), a, pedido(arg, prazo), &ponteTeste{})
}

func pidDe(t *testing.T, p *Pool, a Automacao) int {
	t.Helper()
	r, err := rodar(t, p, a, map[string]any{"acao": "pid"}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var pid int
	json.Unmarshal(r, &pid)
	return pid
}

func vivo(pid int) bool { return syscall.Kill(pid, 0) == nil }

func eventualmente(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	limite := time.Now().Add(5 * time.Second)
	for time.Now().Before(limite) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(msg)
}

func TestRetornoCtxENotificacoes(t *testing.T) {
	p := novoPoolTeste(t, nil, 0)
	a := aut("a1", "h1")
	r, err := rodar(t, p, a, map[string]any{"acao": "retornar", "valor": map[string]any{"ok": true}}, 5*time.Second)
	if err != nil || string(r) != `{"ok":true}` {
		t.Fatalf("retorno: %s %v", r, err)
	}
	ponte := &ponteTeste{}
	pd := pedido(map[string]any{"acao": "ctx"}, 5*time.Second)
	r, err = p.Executar(context.Background(), a, pd, ponte)
	if err != nil || string(r) != `{"eco":"`+pd.ExecucaoID+`"}` {
		t.Fatalf("ctx: %s %v", r, err)
	}
	if len(ponte.chamadas) != 1 || ponte.chamadas[0] != "ctx.teste" || len(ponte.logs) != 1 || ponte.http != 1 {
		t.Fatalf("ponte: %+v", ponte)
	}
	if p.Vivos() != 1 {
		t.Fatalf("vivos: %d", p.Vivos())
	}
	// Chamada ctx.* depois do fim da execução → 1007.
	rodar(t, p, a, map[string]any{"acao": "ctx_tarde"}, 5*time.Second)
	time.Sleep(200 * time.Millisecond)
	r, _ = rodar(t, p, a, map[string]any{"acao": "ultimo_erro"}, 5*time.Second)
	if string(r) != "1007" {
		t.Fatalf("execução encerrada: %s", r)
	}
	// Linha maior que 1 MB → -32600 e o processo segue.
	r, _ = rodar(t, p, a, map[string]any{"acao": "linha_grande"}, 5*time.Second)
	if string(r) != `"rejeitado:-32600"` {
		t.Fatalf("linha grande: %s", r)
	}
}

func TestFlagsEAmbienteLimpo(t *testing.T) {
	t.Setenv("ZAPDESK_TOKEN", "segredo-do-motor")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-x")
	p := novoPoolTeste(t, nil, 0)
	a := aut("a1", "h1")
	r, err := rodar(t, p, a, map[string]any{"acao": "ambiente"}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var amb struct {
		ExecArgv []string `json:"execArgv"`
		Env      []string `json:"env"`
		TZ       string   `json:"tz"`
	}
	json.Unmarshal(r, &amb)
	args := strings.Join(amb.ExecArgv, " ")
	for _, f := range []string{"--permission", "--allow-fs-read=" + p.o.Script, "--allow-fs-read=" + a.Bundle, "--max-old-space-size=64"} {
		if !strings.Contains(args, f) {
			t.Errorf("flag ausente %q em %v", f, amb.ExecArgv)
		}
	}
	// __CF_USER_TEXT_ENCODING é criado pelo próprio macOS no processo (não vem do motor).
	permitidas := map[string]bool{"TZ": true, "LANG": true, "__CF_USER_TEXT_ENCODING": true}
	for _, k := range amb.Env {
		if !permitidas[k] {
			t.Errorf("variável de ambiente inesperada no runner: %s", k)
		}
	}
	if amb.TZ == "" || amb.TZ == "Local" {
		t.Fatalf("TZ: %q", amb.TZ)
	}
}

func TestErrosDoHandler(t *testing.T) {
	p := novoPoolTeste(t, nil, 0)
	a := aut("a1", "h1")
	pid := pidDe(t, p, a)
	_, err := rodar(t, p, a, map[string]any{"acao": "erro"}, 5*time.Second)
	var eu *ErroUsuario
	if !errors.As(err, &eu) || eu.Mensagem != "falhou" || eu.Nome != "TypeError" || !strings.Contains(eu.Stack, "index.ts:3") {
		t.Fatalf("erro_usuario: %#v", err)
	}
	pd := pedido(map[string]any{"acao": "retornar"}, 5*time.Second)
	pd.Handler = "aoEvento"
	_, err = p.Executar(context.Background(), a, pd, &ponteTeste{})
	var er *ErroRunner
	if !errors.As(err, &er) || !strings.Contains(er.Mensagem, "aoEvento não exportado") {
		t.Fatalf("handler ausente: %#v", err)
	}
	if pidDe(t, p, a) != pid {
		t.Fatal("erro do handler não deveria reiniciar o processo")
	}
	// Bundle inválido: não inicia.
	b := aut("a2", "h1")
	b.Bundle = filepath.Join(filepath.Dir(b.Bundle), "invalido.mjs")
	_, err = rodar(t, p, b, map[string]any{"acao": "retornar"}, 5*time.Second)
	if err == nil || !strings.Contains(err.Error(), "não iniciou") {
		t.Fatalf("bundle inválido: %v", err)
	}
}

func TestPrazoMataEInterrompeAsOutras(t *testing.T) {
	p := novoPoolTeste(t, nil, 0)
	a := aut("a1", "h1")
	pid := pidDe(t, p, a)
	var errDormir error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, errDormir = rodar(t, p, a, map[string]any{"acao": "dormir", "ms": 20000}, 20*time.Second)
	}()
	time.Sleep(100 * time.Millisecond)
	inicio := time.Now()
	_, err := rodar(t, p, a, map[string]any{"acao": "loop"}, 300*time.Millisecond)
	if err == nil || err.Error() != "Tempo limite de 2 s excedido" {
		t.Fatalf("prazo: %v", err)
	}
	if d := time.Since(inicio); d < 2*time.Second || d > 5*time.Second {
		t.Fatalf("SIGKILL em prazo+2 s: %v", d)
	}
	wg.Wait()
	if errDormir == nil || errDormir.Error() != "Interrompida porque outra execução desta automação excedeu o tempo" {
		t.Fatalf("outra execução: %v", errDormir)
	}
	eventualmente(t, func() bool { return !vivo(pid) }, "processo que estourou continua vivo")
	// Reinicia sob demanda.
	if novo := pidDe(t, p, a); novo == pid {
		t.Fatal("deveria subir outro processo")
	}
}

func TestSaidaInesperadaEOOM(t *testing.T) {
	p := novoPoolTeste(t, nil, 0)
	_, err := rodar(t, p, aut("a1", "h1"), map[string]any{"acao": "sair"}, 5*time.Second)
	if err == nil || err.Error() != "O processo da automação terminou inesperadamente (código 3)" {
		t.Fatalf("saída: %v", err)
	}
	_, err = rodar(t, p, aut("a2", "h1"), map[string]any{"acao": "stderr"}, 5*time.Second)
	var er *ErroRunner
	if !errors.As(err, &er) || !strings.Contains(strings.Join(er.Stderr, "\n"), "linha de erro 2") {
		t.Fatalf("stderr no erro: %#v", err)
	}
	b := aut("a3", "h1")
	b.MemoriaMB = 32
	_, err = rodar(t, p, b, map[string]any{"acao": "oom"}, 20*time.Second)
	if err == nil || err.Error() != "Memória excedida (32 MB)" {
		t.Fatalf("OOM: %v", err)
	}
	eventualmente(t, func() bool { return p.Vivos() == 0 }, "processos mortos ainda contados")
}

func TestLimiteDeProcessosComFila(t *testing.T) {
	p := novoPoolTeste(t, nil, 4)
	var wg sync.WaitGroup
	var maxVivos atomic.Int64
	parar := make(chan struct{})
	go func() {
		for {
			select {
			case <-parar:
				return
			default:
			}
			p.mu.Lock()
			n := int64(len(p.procs))
			p.mu.Unlock()
			if n > maxVivos.Load() {
				maxVivos.Store(n)
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()
	inicio := time.Now()
	erros := make([]error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, erros[i] = rodar(t, p, aut(fmt.Sprintf("a%d", i), "h"), map[string]any{"acao": "dormir", "ms": 600}, 10*time.Second)
		}(i)
	}
	wg.Wait()
	close(parar)
	for i, e := range erros {
		if e != nil {
			t.Fatalf("execução %d: %v", i, e)
		}
	}
	if maxVivos.Load() > 4 {
		t.Fatalf("mais de 4 processos: %d", maxVivos.Load())
	}
	if d := time.Since(inicio); d < 1200*time.Millisecond {
		t.Fatalf("a 5ª deveria esperar na fila: %v", d)
	}
}

func TestOciosidadeComRelogioControlavel(t *testing.T) {
	rel := relogio.NovoCongelado(time.Date(2026, 9, 28, 10, 0, 0, 0, time.Local))
	p := novoPoolTeste(t, rel, 0)
	pid := pidDe(t, p, aut("a1", "h1"))
	rel.Avancar(4 * time.Minute)
	time.Sleep(100 * time.Millisecond)
	if p.Vivos() != 1 {
		t.Fatal("fechou antes da ociosidade")
	}
	rel.Avancar(time.Minute + time.Second)
	eventualmente(t, func() bool { return p.Vivos() == 0 && !vivo(pid) }, "não fechou após 5 min ocioso")
}

func TestHashNovoESegredosReiniciam(t *testing.T) {
	p := novoPoolTeste(t, nil, 0)
	pid1 := pidDe(t, p, aut("a1", "h1"))
	pid2 := pidDe(t, p, aut("a1", "h2"))
	if pid1 == pid2 {
		t.Fatal("hash novo deveria usar outro processo")
	}
	eventualmente(t, func() bool { return !vivo(pid1) && p.Vivos() == 1 }, "processo do hash antigo continua vivo")

	a := aut("s1", "h1")
	a.NomesSegredos = []string{"OPENAI_KEY"}
	a.Segredos = map[string]string{"OPENAI_KEY": "v1"}
	r, _ := rodar(t, p, a, map[string]any{"acao": "segredos"}, 5*time.Second)
	if string(r) != `{"OPENAI_KEY":"v1"}` {
		t.Fatalf("segredos: %s", r)
	}
	pidS := pidDe(t, p, a)
	p.SegredosAlterados([]string{"OUTRO"})
	time.Sleep(100 * time.Millisecond)
	if !vivo(pidS) {
		t.Fatal("segredo não declarado não deveria reiniciar")
	}
	p.SegredosAlterados([]string{"OPENAI_KEY"})
	eventualmente(t, func() bool { return !vivo(pidS) }, "segredo alterado não encerrou o processo")
	a.Segredos = map[string]string{"OPENAI_KEY": "v2"}
	r, _ = rodar(t, p, a, map[string]any{"acao": "segredos"}, 5*time.Second)
	if string(r) != `{"OPENAI_KEY":"v2"}` {
		t.Fatalf("segredos novos: %s", r)
	}
}

func TestEncerrarAutomacaoECancelar(t *testing.T) {
	p := novoPoolTeste(t, nil, 0)
	a := aut("a1", "h1")
	pd := pedido(map[string]any{"acao": "dormir", "ms": 300}, 5*time.Second)
	fim := make(chan struct{})
	go func() {
		// Repete até a execução terminar: o processo pode demorar a subir sob carga.
		for {
			select {
			case <-fim:
				return
			case <-time.After(20 * time.Millisecond):
				p.Cancelar(pd.ExecucaoID)
			}
		}
	}()
	_, err := p.Executar(context.Background(), a, pd, &ponteTeste{})
	close(fim)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := rodar(t, p, a, map[string]any{"acao": "cancelados"}, 5*time.Second)
	if !strings.Contains(string(r), pd.ExecucaoID) {
		t.Fatalf("cancelar não chegou: %s", r)
	}
	pid := pidDe(t, p, a)
	p.EncerrarAutomacao("a1")
	if vivo(pid) || p.Vivos() != 0 {
		t.Fatal("EncerrarAutomacao deveria esperar o processo sair")
	}
}

func TestEncerrarTodosNaoDeixaProcessos(t *testing.T) {
	p := novoPoolTeste(t, nil, 0)
	var pids []int
	for i := 0; i < 3; i++ {
		pids = append(pids, pidDe(t, p, aut(fmt.Sprintf("a%d", i), "h")))
	}
	ocupado := make(chan error, 1)
	go func() {
		_, err := rodar(t, p, aut("a0", "h"), map[string]any{"acao": "loop"}, 30*time.Second)
		ocupado <- err
	}()
	time.Sleep(200 * time.Millisecond)
	inicio := time.Now()
	p.EncerrarTodos(context.Background())
	if d := time.Since(inicio); d > 5*time.Second {
		t.Fatalf("encerramento lento: %v", d)
	}
	for _, pid := range pids {
		if vivo(pid) {
			t.Fatalf("processo %d sobreviveu ao encerramento", pid)
		}
	}
	if p.Vivos() != 0 {
		t.Fatalf("vivos: %d", p.Vivos())
	}
	select {
	case err := <-ocupado:
		if err == nil {
			t.Fatal("execução em andamento deveria falhar")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("execução em andamento não terminou")
	}
	if _, err := rodar(t, p, aut("a9", "h"), map[string]any{"acao": "pid"}, time.Second); err == nil {
		t.Fatal("pool encerrado não deveria executar")
	}
}
