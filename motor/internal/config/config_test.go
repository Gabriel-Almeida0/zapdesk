package config

import (
	"errors"
	"strings"
	"testing"
)

const tokenValido = "abcdefghijklmnopqrstuvwxyz0123456789"

func amb(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestPadroes(t *testing.T) {
	c, err := Carregar(nil, amb(map[string]string{"ZAPDESK_TOKEN": tokenValido}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.WhatsApp != WhatsAppReal || c.NivelLog != "info" || c.Porta != 0 || c.LogConteudo || c.SemStdin || c.Religado {
		t.Fatalf("padrões errados: %+v", c)
	}
	if !strings.HasSuffix(c.PastaDados, "Library/Application Support/ZapDesk") {
		t.Fatalf("pasta padrão: %s", c.PastaDados)
	}
}

func TestFlagsEAmbiente(t *testing.T) {
	c, err := Carregar(
		[]string{"--pasta-dados", "/tmp/x", "--porta", "7788", "--whatsapp=falso", "--nivel-log", "debug", "--log-conteudo", "--sem-stdin", "--religado"},
		amb(map[string]string{"ZAPDESK_TOKEN": tokenValido}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.PastaDados != "/tmp/x" || c.Porta != 7788 || c.WhatsApp != "falso" || c.NivelLog != "debug" || !c.LogConteudo || !c.SemStdin || !c.Religado {
		t.Fatalf("flags não aplicadas: %+v", c)
	}

	c, err = Carregar(nil, amb(map[string]string{
		"ZAPDESK_TOKEN": tokenValido, "ZAPDESK_PASTA_DADOS": "/tmp/y", "ZAPDESK_PORTA": "9000",
		"ZAPDESK_WHATSAPP": "falso", "ZAPDESK_NIVEL_LOG": "erro", "ZAPDESK_LOG_CONTEUDO": "1",
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.PastaDados != "/tmp/y" || c.Porta != 9000 || c.WhatsApp != "falso" || c.NivelLog != "erro" || !c.LogConteudo {
		t.Fatalf("ambiente não aplicado: %+v", c)
	}

	// flag vence ambiente
	c, _ = Carregar([]string{"--porta", "1"}, amb(map[string]string{"ZAPDESK_TOKEN": tokenValido, "ZAPDESK_PORTA": "9000"}), nil)
	if c.Porta != 1 {
		t.Fatalf("flag deveria vencer: %d", c.Porta)
	}
}

func TestToken(t *testing.T) {
	for _, tok := range []string{"", "curto", strings.Repeat("a", 31)} {
		_, err := Carregar(nil, amb(map[string]string{"ZAPDESK_TOKEN": tok}), nil)
		var ec *ErroConfig
		if !errors.As(err, &ec) || ec.Codigo != "token_ausente" {
			t.Fatalf("token %q: esperado token_ausente, veio %v", tok, err)
		}
	}
	if _, err := Carregar(nil, amb(map[string]string{"ZAPDESK_TOKEN": strings.Repeat("a", 32)}), nil); err != nil {
		t.Fatalf("32 caracteres deveria passar: %v", err)
	}
}

func TestVersaoDispensaToken(t *testing.T) {
	c, err := Carregar([]string{"--versao"}, amb(nil), nil)
	if err != nil || !c.MostrarVersao {
		t.Fatalf("--versao: %v %+v", err, c)
	}
}

func TestInvalidos(t *testing.T) {
	casos := [][]string{{"--whatsapp=outro"}, {"--nivel-log=trace"}, {"--porta=70000"}, {"--desconhecida"}, {"sobra"}}
	for _, args := range casos {
		_, err := Carregar(args, amb(map[string]string{"ZAPDESK_TOKEN": tokenValido}), nil)
		var ec *ErroConfig
		if !errors.As(err, &ec) || ec.Codigo != "config_invalida" {
			t.Fatalf("%v: esperado config_invalida, veio %v", args, err)
		}
	}
}

func TestFlagsAutomacoes(t *testing.T) {
	c, err := Carregar(nil, amb(map[string]string{"ZAPDESK_TOKEN": tokenValido}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.IA != IAReal || c.AguardarSegredos || c.RunnerDisponivel() {
		t.Fatalf("padrões de automação errados: %+v", c)
	}
	c, err = Carregar([]string{"--runner-exec", "/usr/bin/node", "--runner-script", "/x/zapdesk-runner.mjs", "--ia=falsa", "--aguardar-segredos"},
		amb(map[string]string{"ZAPDESK_TOKEN": tokenValido}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.RunnerExec != "/usr/bin/node" || c.RunnerScript != "/x/zapdesk-runner.mjs" || c.IA != IAFalsa || !c.AguardarSegredos || !c.RunnerDisponivel() {
		t.Fatalf("flags de automação não aplicadas: %+v", c)
	}
	c, err = Carregar(nil, amb(map[string]string{"ZAPDESK_TOKEN": tokenValido, "ZAPDESK_RUNNER_EXEC": "node",
		"ZAPDESK_RUNNER_SCRIPT": "/r.mjs", "ZAPDESK_IA": "falsa"}), nil)
	if err != nil || c.RunnerExec != "node" || c.RunnerScript != "/r.mjs" || c.IA != IAFalsa {
		t.Fatalf("ambiente de automação não aplicado: %+v %v", c, err)
	}
	if _, err := Carregar([]string{"--ia=talvez"}, amb(map[string]string{"ZAPDESK_TOKEN": tokenValido}), nil); err == nil {
		t.Fatal("--ia inválida aceita")
	}
	if _, err := Carregar([]string{"--runner-exec", "node"}, amb(map[string]string{"ZAPDESK_TOKEN": tokenValido}), nil); err == nil {
		t.Fatal("--runner-exec sem --runner-script aceito")
	}
}
