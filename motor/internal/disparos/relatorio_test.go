package disparos

import (
	"strings"
	"testing"
)

// A variável {nome} não pode gerar uma segunda coluna "nome" no relatório.
func TestCabecalhoRelatorioSemColunaRepetida(t *testing.T) {
	cab := CabecalhoRelatorio([]string{"nome", "empresa"})
	vistos := map[string]bool{}
	for _, c := range cab {
		if vistos[c] {
			t.Fatalf("coluna repetida %q em %v", c, cab)
		}
		vistos[c] = true
	}
	if got := strings.Join(cab[len(cab)-2:], ","); got != "{nome},{empresa}" {
		t.Fatalf("colunas de variáveis: %s", got)
	}
	if strings.Join(CabecalhoRelatorio(nil), ",") != "telefone,nome,estado,motivo_falha,enviando_em,enviado_em,entregue_em,lido_em,respondeu_em,falhou_em" {
		t.Fatal("colunas fixas mudaram")
	}
}
