package acoes

import (
	"errors"
	"testing"
)

func TestResolverVariaveis(t *testing.T) {
	f := FonteVariaveis{
		Execucao:   map[string]string{"email": "a@b.com", "nome": ""},
		CamposLead: map[string]string{"empresa": "ACME", "cidade": ""},
		NomeLead:   "", NomeContato: "Maria Souza", NomePush: "Mari",
		Telefone: "+5511999990000", UltimaMensagem: "Quanto custa?", Conta: "Loja",
	}
	txt, err := Resolver("Oi {primeiro_nome} ({Nome}) da {empresa}, {telefone} — '{ultima_mensagem}' em {conta}: {email}", f, nil)
	if err != nil || txt != "Oi Maria (Maria Souza) da ACME, +5511999990000 — 'Quanto custa?' em Loja: a@b.com" {
		t.Fatalf("%q %v", txt, err)
	}
	// Precedência: execução > campos do lead.
	f.Execucao["empresa"] = "Outra"
	if txt, _ := Resolver("{empresa}", f, nil); txt != "Outra" {
		t.Fatal(txt)
	}
	// Nome: lead → contato → push.
	f.NomeLead = "João"
	if txt, _ := Resolver("{nome}", f, nil); txt != "João" {
		t.Fatal(txt)
	}
	f.NomeLead, f.NomeContato = "", ""
	if txt, _ := Resolver("{primeiro_nome}", f, nil); txt != "Mari" {
		t.Fatal(txt)
	}
	// Sem valor: usa o padrão; sem padrão falha.
	if txt, _ := Resolver("Olá {cidade}", f, map[string]string{"cidade": "tudo bem"}); txt != "Olá tudo bem" {
		t.Fatal(txt)
	}
	_, err = Resolver("Olá {cidade}", f, nil)
	var ev *ErroVariavel
	if !errors.As(err, &ev) || ev.Nome != "cidade" || err.Error() != "Variável {cidade} sem valor" {
		t.Fatalf("%v", err)
	}
	// Chaves literais.
	if txt, _ := Resolver("{{nome}}", f, nil); txt != "{nome}" {
		t.Fatal(txt)
	}
	// Entrada JSON.
	m, err := ResolverEntrada(map[string]any{"p": "{email}", "n": 3, "l": []any{"{telefone}"}}, f)
	if err != nil || m["p"] != "a@b.com" || m["n"] != 3 || m["l"].([]any)[0] != "+5511999990000" {
		t.Fatalf("%v %v", m, err)
	}
}
