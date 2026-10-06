package ciclo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLinhasControle(t *testing.T) {
	var buf bytes.Buffer
	s := NovaSaida(&buf)
	s.Pronto(1234, "0.1.0", 99, "falso")
	s.ErroFatal(FatalPortaOcupada, "x")
	s.Encerrando(MotivoSinal)
	linhas := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(linhas) != 3 || !strings.Contains(linhas[0], `"evento":"pronto"`) || !strings.Contains(linhas[0], `"porta":1234`) ||
		!strings.Contains(linhas[1], `"codigo":"porta_ocupada"`) || !strings.Contains(linhas[2], `"motivo":"sinal"`) {
		t.Fatalf("linhas: %v", linhas)
	}
}

func TestTravaExclusiva(t *testing.T) {
	pasta := t.TempDir()
	t1, err := Travar(pasta)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Travar(pasta); !errors.Is(err, ErrInstanciaDuplicada) {
		t.Fatalf("esperado instância duplicada: %v", err)
	}
	t1.Liberar()
	t2, err := Travar(pasta)
	if err != nil {
		t.Fatalf("após liberar: %v", err)
	}
	t2.Liberar()
}

func TestStdinEPedido(t *testing.T) {
	r, w := io.Pipe()
	fim := VigiarStdin(r)
	w.Close()
	if m := AguardarFim(context.Background(), fim, nil); m != MotivoStdinFechado {
		t.Fatalf("motivo %s", m)
	}
	pedido := make(chan struct{})
	close(pedido)
	if m := AguardarFim(context.Background(), nil, pedido); m != MotivoPedido {
		t.Fatalf("motivo %s", m)
	}
	_ = time.Second
}

func TestStdinLinhasDeControle(t *testing.T) {
	r, w := io.Pipe()
	var mu sync.Mutex
	var comandos []ComandoControle
	invalidas := 0
	fim := VigiarStdinControle(r, func(c ComandoControle) {
		mu.Lock()
		comandos = append(comandos, c)
		mu.Unlock()
	}, func() {
		mu.Lock()
		invalidas++
		mu.Unlock()
	})
	io.WriteString(w, `{"comando":"segredos","valores":{"ANTHROPIC_API_KEY":"sk-ant-x"}}`+"\n")
	io.WriteString(w, "isto não é json\n")
	io.WriteString(w, `{"comando":"outro"}`+"\n")
	io.WriteString(w, "\n")
	io.WriteString(w, `{"comando":"segredos","valores":{}}`+"\n")
	grande := `{"comando":"segredos","valores":{"X":"` + strings.Repeat("a", TamanhoMaximoLinhaControle) + `"}}` + "\n"
	io.WriteString(w, grande)
	w.Close()
	select {
	case <-fim:
	case <-time.After(5 * time.Second):
		t.Fatal("EOF não encerrou")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(comandos) != 2 || comandos[0].Valores["ANTHROPIC_API_KEY"] != "sk-ant-x" || len(comandos[1].Valores) != 0 {
		t.Fatalf("comandos: %+v", comandos)
	}
	if invalidas != 3 {
		t.Fatalf("inválidas: %d", invalidas)
	}
}
