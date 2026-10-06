// Pacote config lê as flags e variáveis de ambiente do motor (contracts/runtime.md › Flags).
package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Modos do cliente WhatsApp.
const (
	WhatsAppReal  = "real"
	WhatsAppFalso = "falso"
)

// TamanhoMinimoToken é o tamanho mínimo aceito para ZAPDESK_TOKEN.
const TamanhoMinimoToken = 32

// Config é a configuração final do motor.
type Config struct {
	PastaDados    string
	Porta         int
	WhatsApp      string // real | falso
	NivelLog      string // debug | info | aviso | erro
	LogConteudo   bool
	SemStdin      bool
	Religado      bool
	MostrarVersao bool
	Token         string

	// Feature 002 (specs/002-automacoes/contracts/runtime.md › Novas flags).
	RunnerExec       string // executável do runner (Electron ou node); vazio = IA não executa
	RunnerScript     string // zapdesk-runner.mjs
	IA               string // real | falsa
	AguardarSegredos bool   // espera o 1º comando "segredos" no stdin (máx. 2 s)
}

// Modos da IA.
const (
	IAReal  = "real"
	IAFalsa = "falsa"
)

// RunnerDisponivel indica se as automações de IA podem executar.
func (c Config) RunnerDisponivel() bool { return c.RunnerExec != "" && c.RunnerScript != "" }

// ErroConfig indica configuração inválida (o motor sai com código 2).
type ErroConfig struct {
	Codigo   string // token_ausente | config_invalida
	Mensagem string
}

func (e *ErroConfig) Error() string { return e.Mensagem }

// ErrAjuda é devolvido quando o usuário pediu -h/--help.
var ErrAjuda = flag.ErrHelp

// PastaDadosPadrao devolve ~/Library/Application Support/ZapDesk.
func PastaDadosPadrao() string {
	casa, err := os.UserHomeDir()
	if err != nil {
		return "ZapDesk"
	}
	return filepath.Join(casa, "Library", "Application Support", "ZapDesk")
}

// Carregar interpreta os argumentos (sem o nome do programa) e o ambiente. `ambiente` pode ser
// os.Getenv; `saida` recebe o texto de ajuda.
func Carregar(args []string, ambiente func(string) string, saida io.Writer) (Config, error) {
	if ambiente == nil {
		ambiente = os.Getenv
	}
	c := Config{
		PastaDados:  valorOu(ambiente("ZAPDESK_PASTA_DADOS"), PastaDadosPadrao()),
		WhatsApp:    valorOu(ambiente("ZAPDESK_WHATSAPP"), WhatsAppReal),
		NivelLog:    valorOu(ambiente("ZAPDESK_NIVEL_LOG"), "info"),
		LogConteudo: ambiente("ZAPDESK_LOG_CONTEUDO") == "1",

		RunnerExec:   ambiente("ZAPDESK_RUNNER_EXEC"),
		RunnerScript: ambiente("ZAPDESK_RUNNER_SCRIPT"),
		IA:           valorOu(ambiente("ZAPDESK_IA"), IAReal),
	}
	if p := ambiente("ZAPDESK_PORTA"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			return c, &ErroConfig{"config_invalida", "ZAPDESK_PORTA inválida: " + p}
		}
		c.Porta = n
	}

	fs := flag.NewFlagSet("zapdesk-motor", flag.ContinueOnError)
	if saida != nil {
		fs.SetOutput(saida)
	} else {
		fs.SetOutput(io.Discard)
	}
	fs.StringVar(&c.PastaDados, "pasta-dados", c.PastaDados, "pasta de dados")
	fs.IntVar(&c.Porta, "porta", c.Porta, "porta TCP em 127.0.0.1 (0 = aleatória)")
	fs.StringVar(&c.WhatsApp, "whatsapp", c.WhatsApp, "implementação do WhatsApp: real | falso")
	fs.StringVar(&c.NivelLog, "nivel-log", c.NivelLog, "nível de log: debug | info | aviso | erro")
	fs.BoolVar(&c.LogConteudo, "log-conteudo", c.LogConteudo, "inclui conteúdo de mensagens no log (depuração)")
	fs.BoolVar(&c.SemStdin, "sem-stdin", false, "não encerrar ao fechar o stdin")
	fs.BoolVar(&c.Religado, "religado", false, "o app religou o motor após uma queda")
	fs.BoolVar(&c.MostrarVersao, "versao", false, "imprime a versão e sai")
	fs.StringVar(&c.RunnerExec, "runner-exec", c.RunnerExec, "executável que roda o runner das automações de IA")
	fs.StringVar(&c.RunnerScript, "runner-script", c.RunnerScript, "caminho do zapdesk-runner.mjs")
	fs.StringVar(&c.IA, "ia", c.IA, "IA das automações: real | falsa (simulada, sem rede)")
	fs.BoolVar(&c.AguardarSegredos, "aguardar-segredos", false, "espera o primeiro comando de segredos no stdin (máx. 2 s)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return c, ErrAjuda
		}
		return c, &ErroConfig{"config_invalida", err.Error()}
	}
	if fs.NArg() > 0 {
		return c, &ErroConfig{"config_invalida", "argumentos inesperados: " + strings.Join(fs.Args(), " ")}
	}
	if c.MostrarVersao {
		return c, nil
	}

	switch c.WhatsApp {
	case WhatsAppReal, WhatsAppFalso:
	default:
		return c, &ErroConfig{"config_invalida", fmt.Sprintf("--whatsapp deve ser real ou falso (recebido %q)", c.WhatsApp)}
	}
	switch c.NivelLog {
	case "debug", "info", "aviso", "erro":
	default:
		return c, &ErroConfig{"config_invalida", fmt.Sprintf("--nivel-log inválido: %q", c.NivelLog)}
	}
	switch c.IA {
	case IAReal, IAFalsa:
	default:
		return c, &ErroConfig{"config_invalida", fmt.Sprintf("--ia deve ser real ou falsa (recebido %q)", c.IA)}
	}
	if (c.RunnerExec == "") != (c.RunnerScript == "") {
		return c, &ErroConfig{"config_invalida", "--runner-exec e --runner-script devem ser informados juntos"}
	}
	if c.RunnerScript != "" {
		if abs, err := filepath.Abs(c.RunnerScript); err == nil {
			c.RunnerScript = abs
		}
	}
	if c.Porta < 0 || c.Porta > 65535 {
		return c, &ErroConfig{"config_invalida", fmt.Sprintf("--porta fora do intervalo: %d", c.Porta)}
	}
	if c.PastaDados == "" {
		return c, &ErroConfig{"config_invalida", "--pasta-dados vazia"}
	}
	abs, err := filepath.Abs(c.PastaDados)
	if err == nil {
		c.PastaDados = abs
	}

	c.Token = ambiente("ZAPDESK_TOKEN")
	if len(c.Token) < TamanhoMinimoToken {
		return c, &ErroConfig{"token_ausente", "ZAPDESK_TOKEN ausente ou com menos de 32 caracteres"}
	}
	return c, nil
}

func valorOu(v, padrao string) string {
	if v == "" {
		return padrao
	}
	return v
}
