package segredos

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSubstituirValidaEDevolveAlterados(t *testing.T) {
	c := Novo()
	var avisos [][]string
	c.Assinar(func(a []string) { avisos = append(avisos, a) })

	alt := c.Substituir(map[string]string{
		"ANTHROPIC_API_KEY": "sk-ant-1", "OPENAI_KEY": "x", "minusculo": "v", "VAZIO": "",
		"GRANDE": strings.Repeat("a", TamanhoMaximoValor+1), "A" + strings.Repeat("B", 64): "v",
	})
	if !reflect.DeepEqual(alt, []string{"ANTHROPIC_API_KEY", "OPENAI_KEY"}) {
		t.Fatalf("alterados: %v", alt)
	}
	if !reflect.DeepEqual(c.Nomes(), []string{"ANTHROPIC_API_KEY", "OPENAI_KEY"}) {
		t.Fatalf("nomes: %v", c.Nomes())
	}
	// Mesmo conjunto: nada muda, ninguém é avisado.
	if alt := c.Substituir(map[string]string{"ANTHROPIC_API_KEY": "sk-ant-1", "OPENAI_KEY": "x"}); len(alt) != 0 {
		t.Fatalf("sem mudança: %v", alt)
	}
	// Substitui o conjunto: ausente = removido; valor diferente = alterado.
	alt = c.Substituir(map[string]string{"OPENAI_KEY": "y", "NOVO": "z"})
	if !reflect.DeepEqual(alt, []string{"ANTHROPIC_API_KEY", "NOVO", "OPENAI_KEY"}) {
		t.Fatalf("alterados 2: %v", alt)
	}
	if c.Tem(ChaveAnthropic) {
		t.Fatal("chave removida continua")
	}
	if len(avisos) != 2 {
		t.Fatalf("avisos: %v", avisos)
	}
}

func TestDeclaradosNuncaEntregaReservado(t *testing.T) {
	c := Novo()
	c.Substituir(map[string]string{"ANTHROPIC_API_KEY": "sk", "A": "1", "B": "2"})
	d := c.Declarados([]string{"ANTHROPIC_API_KEY", "A", "C"})
	if !reflect.DeepEqual(d, map[string]string{"A": "1"}) {
		t.Fatalf("declarados: %v", d)
	}
	if !Reservado("ANTHROPIC_API_KEY") || Reservado("A") || NomeValido("a") || !NomeValido("A_1") {
		t.Fatal("regras de nome")
	}
}

func TestAguardarPrimeiro(t *testing.T) {
	c := Novo()
	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()
	if c.AguardarPrimeiro(ctx) || c.Recebeu() {
		t.Fatal("não recebeu nada ainda")
	}
	go c.Substituir(map[string]string{})
	ctx2, cancelar2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar2()
	if !c.AguardarPrimeiro(ctx2) || !c.Recebeu() {
		t.Fatal("envio vazio também conta como primeiro")
	}
	// Chamadas seguintes não bloqueiam.
	if !c.AguardarPrimeiro(ctx) {
		t.Fatal("segunda espera")
	}
}
