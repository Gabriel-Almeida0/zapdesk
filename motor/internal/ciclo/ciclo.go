// Pacote ciclo cuida do ciclo de vida do processo do motor (contracts/runtime.md): linhas JSON
// de controle no stdout, trava de instância única (motor.lock com flock), vigia do stdin e
// sinais de encerramento.
package ciclo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
)

// Códigos de saída.
const (
	SaidaNormal             = 0
	SaidaErroInesperado     = 1
	SaidaConfigInvalida     = 2
	SaidaInstanciaDuplicada = 3
)

// Códigos de erro_fatal.
const (
	FatalTokenAusente       = "token_ausente"
	FatalPastaInacessivel   = "pasta_dados_inacessivel"
	FatalInstanciaDuplicada = "instancia_duplicada"
	FatalMigracaoFalhou     = "migracao_falhou"
	FatalPortaOcupada       = "porta_ocupada"
)

// Motivos de encerramento.
const (
	MotivoSinal        = "sinal"
	MotivoStdinFechado = "stdin_fechado"
	MotivoPedido       = "pedido"
)

// Saida escreve as linhas de controle no stdout (uma por linha, nunca logs).
type Saida struct {
	mu sync.Mutex
	w  io.Writer
}

// NovaSaida cria a saída de controle.
func NovaSaida(w io.Writer) *Saida { return &Saida{w: w} }

func (s *Saida) escrever(v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	linha, _ := json.Marshal(v)
	s.w.Write(append(linha, '\n'))
	if f, ok := s.w.(interface{ Sync() error }); ok {
		f.Sync()
	}
}

// Pronto emite {"evento":"pronto",...}.
func (s *Saida) Pronto(porta int, versao string, pid int, whatsapp string) {
	s.escrever(map[string]any{"evento": "pronto", "porta": porta, "versao": versao, "pid": pid, "whatsapp": whatsapp})
}

// ErroFatal emite {"evento":"erro_fatal",...}.
func (s *Saida) ErroFatal(codigo, mensagem string) {
	s.escrever(map[string]any{"evento": "erro_fatal", "codigo": codigo, "mensagem": mensagem})
}

// Encerrando emite {"evento":"encerrando","motivo":...}.
func (s *Saida) Encerrando(motivo string) {
	s.escrever(map[string]any{"evento": "encerrando", "motivo": motivo})
}

// ErrInstanciaDuplicada indica que outro motor segura a trava.
var ErrInstanciaDuplicada = errors.New("outro motor já está usando esta pasta de dados")

// Trava é o motor.lock.
type Trava struct{ arq *os.File }

// Travar obtém flock exclusivo em <pasta>/motor.lock.
func Travar(pastaDados string) (*Trava, error) {
	arq, err := os.OpenFile(filepath.Join(pastaDados, "motor.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(arq.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		arq.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrInstanciaDuplicada
		}
		return nil, err
	}
	return &Trava{arq: arq}, nil
}

// Liberar solta a trava.
func (t *Trava) Liberar() {
	if t == nil || t.arq == nil {
		return
	}
	syscall.Flock(int(t.arq.Fd()), syscall.LOCK_UN)
	t.arq.Close()
	t.arq = nil
}

// VigiarStdin fecha o canal devolvido quando o leitor chega ao fim (o app morreu). Linhas de
// controle são ignoradas (use VigiarStdinControle para recebê-las).
func VigiarStdin(r io.Reader) <-chan struct{} { return VigiarStdinControle(r, nil, nil) }

// ComandoControle é uma linha JSON de controle enviada pelo app no stdin
// (specs/002-automacoes/contracts/runtime.md › Canal de controle no stdin).
type ComandoControle struct {
	Comando string            `json:"comando"`
	Valores map[string]string `json:"valores"`
}

// ComandoSegredos substitui o conjunto de segredos em memória.
const ComandoSegredos = "segredos"

// TamanhoMaximoLinhaControle limita uma linha de controle (64 segredos × 4 KB cabem com folga).
const TamanhoMaximoLinhaControle = 4 << 20

// VigiarStdinControle lê linhas JSON de controle do stdin e entrega cada comando válido a
// `aoComando` (na ordem, na goroutine de leitura). Linhas inválidas ou desconhecidas chamam
// `invalida` (que NÃO recebe o conteúdo, para nunca logar segredos) e são ignoradas. O canal
// devolvido fecha no EOF (o app morreu), como no MVP.
func VigiarStdinControle(r io.Reader, aoComando func(ComandoControle), invalida func()) <-chan struct{} {
	fim := make(chan struct{})
	go func() {
		defer close(fim)
		leitor := bufio.NewReaderSize(r, 64<<10)
		for {
			linha, err := lerLinhaLimitada(leitor, TamanhoMaximoLinhaControle)
			if len(bytes.TrimSpace(linha)) > 0 {
				interpretarLinha(linha, aoComando, invalida)
			}
			if err != nil {
				return
			}
		}
	}()
	return fim
}

// lerLinhaLimitada lê até '\n'; linhas maiores que `limite` são descartadas inteiras (devolve
// um marcador inválido para que `invalida` seja chamado).
func lerLinhaLimitada(r *bufio.Reader, limite int) ([]byte, error) {
	var buf []byte
	grande := false
	for {
		parte, err := r.ReadSlice('\n')
		if !grande {
			if len(buf)+len(parte) > limite {
				grande, buf = true, nil
			} else {
				buf = append(buf, parte...)
			}
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if grande {
			return []byte("\x00"), err
		}
		return buf, err
	}
}

func interpretarLinha(linha []byte, aoComando func(ComandoControle), invalida func()) {
	var c ComandoControle
	if err := json.Unmarshal(linha, &c); err != nil || c.Comando != ComandoSegredos || c.Valores == nil {
		if invalida != nil {
			invalida()
		}
		return
	}
	if aoComando != nil {
		aoComando(c)
	}
}

// AguardarFim bloqueia até um motivo de encerramento: sinal (SIGTERM/SIGINT), stdin fechado
// (se stdin != nil) ou pedido (canal fechado/recebido). Devolve o motivo.
func AguardarFim(ctx context.Context, stdin <-chan struct{}, pedido <-chan struct{}) string {
	sinais := make(chan os.Signal, 1)
	signal.Notify(sinais, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sinais)
	select {
	case <-sinais:
		return MotivoSinal
	case <-stdin:
		return MotivoStdinFechado
	case <-pedido:
		return MotivoPedido
	case <-ctx.Done():
		return MotivoPedido
	}
}
