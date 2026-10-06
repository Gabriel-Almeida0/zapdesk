package variaveis

import (
	"reflect"
	"testing"

	"zapdesk/motor/internal/dominio"
)

func str(s string) *string { return &s }

func TestExtrair(t *testing.T) {
	casos := map[string][]string{
		"Oi {nome}, tudo bem em {cidade}?":       {"nome", "cidade"},
		"{Nome} e {nome} de novo":                {"nome"},
		"Sem variáveis":                          {},
		"Chaves {{literais}} e {x}":              {"x"},
		"{Razão Social} / {  E-mail }":           {"razao_social", "e_mail"},
		"quebrada { } solta {} {\n}":             {},
		"fechamento }} e abertura {{ e {cidade}": {"cidade"},
	}
	for texto, esperado := range casos {
		if v := Extrair(texto); !reflect.DeepEqual(v, esperado) {
			t.Errorf("Extrair(%q) = %v; esperado %v", texto, v, esperado)
		}
	}
}

func TestResolverEscapes(t *testing.T) {
	v := map[string]string{"nome": "Ana", "cidade": "Recife"}
	casos := map[string]string{
		"Oi {nome}!":                "Oi Ana!",
		"{{nome}} é literal":        "{nome} é literal",
		"Em {Cidade}, {{ok}} }}":    "Em Recife, {ok} }",
		"{desconhecida} fica igual": "{desconhecida} fica igual",
		"chave {aberta":             "chave {aberta",
	}
	for texto, esperado := range casos {
		if r := Resolver(texto, v); r != esperado {
			t.Errorf("Resolver(%q) = %q; esperado %q", texto, r, esperado)
		}
	}
}

func TestValoresDoLead(t *testing.T) {
	lead := dominio.Lead{Telefone: "+5511999990000", Nome: str("Ana"), Campos: map[string]string{"cidade": "Recife", "empresa": "  "}}
	vals, faltando := ValoresDoLead([]string{"nome", "cidade", "empresa", "cargo"}, lead, map[string]string{"empresa": "sua empresa"})
	if vals["nome"] != "Ana" || vals["cidade"] != "Recife" || vals["empresa"] != "sua empresa" {
		t.Fatalf("valores: %v", vals)
	}
	if !reflect.DeepEqual(faltando, []string{"cargo"}) {
		t.Fatalf("faltando: %v", faltando)
	}
	semNome := dominio.Lead{Telefone: "+5511988887777", Campos: map[string]string{}}
	_, faltando = ValoresDoLead([]string{"nome"}, semNome, nil)
	if !reflect.DeepEqual(faltando, []string{"nome"}) {
		t.Fatalf("nome ausente: %v", faltando)
	}
	vals, faltando = ValoresDoLead([]string{"nome", "telefone"}, semNome, map[string]string{"nome": "tudo bem"})
	if vals["nome"] != "tudo bem" || vals["telefone"] != "+5511988887777" || len(faltando) != 0 {
		t.Fatalf("valor padrão: %v %v", vals, faltando)
	}
}

func TestFaltandoListaIncompletosEBloqueiaVariavelDesconhecida(t *testing.T) {
	leads := []dominio.Lead{
		{Telefone: "+5511900000001", Nome: str("Ana"), Campos: map[string]string{}},
		{Telefone: "+5511900000002", Campos: map[string]string{}},
		{Telefone: "+5511900000003", Nome: str("Caio"), Campos: map[string]string{}},
	}
	f := Faltando("Oi {nome}", leads, nil)
	if len(f) != 1 || f[0].DestinatarioLinha != 2 || f[0].Telefone != "+5511900000002" || !reflect.DeepEqual(f[0].Variaveis, []string{"nome"}) {
		t.Fatalf("incompletos: %+v", f)
	}
	f = Faltando("Oi {nome}", leads, map[string]string{"nome": "cliente"})
	if len(f) != 0 {
		t.Fatalf("valor padrão deveria cobrir: %+v", f)
	}
	f = Faltando("Oferta para {cargo}", leads, nil)
	if len(f) != 3 {
		t.Fatalf("variável ausente em todos os leads deve bloquear todos: %+v", f)
	}
}
