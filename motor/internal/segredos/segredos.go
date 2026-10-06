// Pacote segredos guarda em memória os segredos entregues pelo app no stdin (chave da Anthropic
// e segredos declarados pelas automações de IA). Valores nunca saem daqui para log, API ou MCP
// (Constituição I; specs/002-automacoes/contracts/runtime.md › Canal de controle).
package segredos

import (
	"context"
	"regexp"
	"sort"
	"sync"
	"unicode/utf8"
)

// ChaveAnthropic é o segredo reservado usado só pelo motor (nunca vai ao runner).
const ChaveAnthropic = "ANTHROPIC_API_KEY"

// TamanhoMaximoValor de um segredo.
const TamanhoMaximoValor = 4096

var nomeValido = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)

// NomeValido indica se o nome segue `[A-Z][A-Z0-9_]{0,63}`.
func NomeValido(nome string) bool { return nomeValido.MatchString(nome) }

// Reservado indica se o nome é de uso exclusivo do motor.
func Reservado(nome string) bool { return nome == ChaveAnthropic }

// Cofre é o conjunto atual de segredos.
type Cofre struct {
	mu         sync.RWMutex
	valores    map[string]string
	recebeu    bool
	primeiro   chan struct{}
	assinantes []func(alterados []string)
}

// Novo cria um cofre vazio que ainda não recebeu nenhum envio.
func Novo() *Cofre {
	return &Cofre{valores: map[string]string{}, primeiro: make(chan struct{})}
}

// Substituir troca o conjunto inteiro (ausente = removido). Entradas com nome ou valor inválidos
// são descartadas. Devolve os nomes cujo valor mudou (criados, alterados ou removidos), em ordem,
// e avisa os assinantes se houver mudança.
func (c *Cofre) Substituir(novos map[string]string) []string {
	validos := map[string]string{}
	for n, v := range novos {
		if !NomeValido(n) || v == "" || utf8.RuneCountInString(v) > TamanhoMaximoValor {
			continue
		}
		validos[n] = v
	}
	c.mu.Lock()
	var alterados []string
	for n, v := range validos {
		if antigo, ok := c.valores[n]; !ok || antigo != v {
			alterados = append(alterados, n)
		}
	}
	for n := range c.valores {
		if _, ok := validos[n]; !ok {
			alterados = append(alterados, n)
		}
	}
	c.valores = validos
	if !c.recebeu {
		c.recebeu = true
		close(c.primeiro)
	}
	assinantes := append([]func([]string){}, c.assinantes...)
	c.mu.Unlock()
	sort.Strings(alterados)
	if len(alterados) > 0 {
		for _, a := range assinantes {
			a(alterados)
		}
	}
	return alterados
}

// Assinar registra quem precisa saber das mudanças (ex.: pool de runners, evento WS).
func (c *Cofre) Assinar(fn func(alterados []string)) {
	c.mu.Lock()
	c.assinantes = append(c.assinantes, fn)
	c.mu.Unlock()
}

// Obter devolve o valor de um segredo.
func (c *Cofre) Obter(nome string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.valores[nome]
	return v, ok
}

// Tem indica se o segredo existe.
func (c *Cofre) Tem(nome string) bool {
	_, ok := c.Obter(nome)
	return ok
}

// Nomes devolve os nomes presentes (sem valores), em ordem.
func (c *Cofre) Nomes() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	lista := make([]string, 0, len(c.valores))
	for n := range c.valores {
		lista = append(lista, n)
	}
	sort.Strings(lista)
	return lista
}

// Declarados devolve os valores dos nomes pedidos que existem, nunca o reservado (usado no
// `inicializar` do runner).
func (c *Cofre) Declarados(nomes []string) map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	r := map[string]string{}
	for _, n := range nomes {
		if Reservado(n) {
			continue
		}
		if v, ok := c.valores[n]; ok {
			r[n] = v
		}
	}
	return r
}

// Recebeu indica se o primeiro envio já chegou.
func (c *Cofre) Recebeu() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.recebeu
}

// AguardarPrimeiro bloqueia até o primeiro envio ou o fim do contexto (o chamador define o prazo
// de 2 s de --aguardar-segredos). Devolve true se recebeu.
func (c *Cofre) AguardarPrimeiro(ctx context.Context) bool {
	select {
	case <-c.primeiro:
		return true
	default:
	}
	select {
	case <-c.primeiro:
		return true
	case <-ctx.Done():
		return false
	}
}
