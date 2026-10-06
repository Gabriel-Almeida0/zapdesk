package logs

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestRemoveConteudo(t *testing.T) {
	var buf bytes.Buffer
	pasta := t.TempDir()
	l, err := Novo(Opcoes{PastaDados: pasta, Nivel: "info", Stderr: &buf})
	if err != nil {
		t.Fatal(err)
	}
	l.Info().Str("conteudo", "segredo do cliente").Str("conta", "x").Msg("mensagem recebida")
	l.Fechar()
	if strings.Contains(buf.String(), "segredo") {
		t.Fatalf("conteúdo vazou no stderr: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "mensagem recebida") || !strings.Contains(buf.String(), `"conta":"x"`) {
		t.Fatalf("linha perdida: %s", buf.String())
	}
	arq, _ := os.ReadFile(CaminhoLog(pasta))
	if strings.Contains(string(arq), "segredo") || !strings.Contains(string(arq), "mensagem recebida") {
		t.Fatalf("arquivo: %s", arq)
	}
}

func TestMantemConteudoComFlag(t *testing.T) {
	var buf bytes.Buffer
	l, err := Novo(Opcoes{PastaDados: t.TempDir(), Nivel: "debug", LogConteudo: true, Stderr: &buf})
	if err != nil {
		t.Fatal(err)
	}
	l.Debug().Str("conteudo", "oi").Msg("x")
	l.Fechar()
	if !strings.Contains(buf.String(), `"conteudo":"oi"`) {
		t.Fatalf("esperado conteúdo: %s", buf.String())
	}
}

func TestNivel(t *testing.T) {
	var buf bytes.Buffer
	l, _ := Novo(Opcoes{PastaDados: t.TempDir(), Nivel: "aviso", Stderr: &buf})
	l.Info().Msg("oculto")
	l.Warn().Msg("visivel")
	l.Fechar()
	if strings.Contains(buf.String(), "oculto") || !strings.Contains(buf.String(), "visivel") {
		t.Fatalf("nível: %s", buf.String())
	}
}
