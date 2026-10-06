// Pacote logs configura o log estruturado do motor: zerolog em <pasta-dados>/logs/motor.log com
// rotação (lumberjack, 10 MB × 5) e cópia no stderr. O campo "conteudo" (texto de mensagens) é
// removido de toda linha, salvo com --log-conteudo (Constituição I).
package logs

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

// CampoConteudo é o nome do campo que carrega conteúdo de mensagens.
const CampoConteudo = "conteudo"

// Opcoes de criação do log.
type Opcoes struct {
	PastaDados  string
	Nivel       string // debug | info | aviso | erro
	LogConteudo bool
	Stderr      io.Writer // padrão os.Stderr; nil desliga com SemStderr
	SemStderr   bool
}

// Log reúne o logger e o fechamento do arquivo.
type Log struct {
	zerolog.Logger
	Caminho string
	arquivo io.Closer
}

// Fechar fecha o arquivo de log.
func (l *Log) Fechar() error {
	if l.arquivo != nil {
		return l.arquivo.Close()
	}
	return nil
}

// CaminhoLog devolve <pasta>/logs/motor.log.
func CaminhoLog(pastaDados string) string {
	return filepath.Join(pastaDados, "logs", "motor.log")
}

// Novo cria o log.
func Novo(o Opcoes) (*Log, error) {
	caminho := CaminhoLog(o.PastaDados)
	if err := os.MkdirAll(filepath.Dir(caminho), 0o700); err != nil {
		return nil, err
	}
	arq := &lumberjack.Logger{Filename: caminho, MaxSize: 10, MaxBackups: 5}
	var destino io.Writer = arq
	if !o.SemStderr {
		stderr := o.Stderr
		if stderr == nil {
			stderr = os.Stderr
		}
		destino = io.MultiWriter(arq, stderr)
	}
	if !o.LogConteudo {
		destino = &filtro{destino: destino}
	}
	zerolog.TimeFieldFormat = "2006-01-02T15:04:05.000Z07:00"
	l := zerolog.New(destino).Level(Nivel(o.Nivel)).With().Timestamp().Logger()
	return &Log{Logger: l, Caminho: caminho, arquivo: arq}, nil
}

// Nivel traduz o nível em português para o do zerolog.
func Nivel(n string) zerolog.Level {
	switch n {
	case "debug":
		return zerolog.DebugLevel
	case "aviso":
		return zerolog.WarnLevel
	case "erro":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

// NovoFiltro devolve um writer que remove o campo "conteudo" de cada linha JSON.
func NovoFiltro(destino io.Writer) io.Writer { return &filtro{destino: destino} }

type filtro struct {
	mu      sync.Mutex
	destino io.Writer
}

var marcaConteudo = []byte(`"` + CampoConteudo + `"`)

func (f *filtro) Write(p []byte) (int, error) {
	if !bytes.Contains(p, marcaConteudo) {
		return f.destino.Write(p)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(p, &m); err != nil {
		return f.destino.Write(p)
	}
	delete(m, CampoConteudo)
	saida, err := json.Marshal(m)
	if err != nil {
		return 0, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, err := f.destino.Write(append(saida, '\n')); err != nil {
		return 0, err
	}
	return len(p), nil
}
