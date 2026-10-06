package chatbot

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"zapdesk/motor/internal/automacoes/condicoes"
	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/variaveis"
)

// efeitosTeste grava tudo o que a máquina faria.
type efeitosTeste struct {
	enviados []string
	acoes    []string
	humano   int
	iaRet    json.RawMessage
	iaErro   error
	bloquear string // bloqueio a devolver a partir do próximo envio
}

func (e *efeitosTeste) Enviar(_ context.Context, noID, texto string, tpl *string) Envio {
	if e.bloquear != "" {
		return Envio{Bloqueio: e.bloquear}
	}
	if tpl != nil {
		texto = "[template " + *tpl + "]"
	}
	e.enviados = append(e.enviados, texto)
	return Envio{}
}
func (e *efeitosTeste) Acao(_ context.Context, noID string, a modelo.Acao, vars map[string]string) Envio {
	v := ""
	if a.Valor != nil {
		v = variaveis.Resolver(*a.Valor, vars)
	}
	e.acoes = append(e.acoes, a.Tipo+":"+a.Campo+"="+v)
	return Envio{}
}
func (e *efeitosTeste) IA(context.Context, modelo.No, map[string]string) (json.RawMessage, error) {
	return e.iaRet, e.iaErro
}
func (e *efeitosTeste) Condicao(_ context.Context, c modelo.Condicoes, vars map[string]string) bool {
	return condicoes.Avaliar(&c, condicoes.Dados{Variaveis: vars})
}
func (e *efeitosTeste) Resolver(_ context.Context, t string, vars map[string]string) (string, error) {
	for _, v := range variaveis.Extrair(t) {
		if vars[v] == "" && v != "nome" {
			return "", errors.New("Variável {" + v + "} sem valor")
		}
	}
	vs := map[string]string{"nome": "Ana"}
	for k, v := range vars {
		vs[k] = v
	}
	return variaveis.Resolver(t, vs), nil
}
func (e *efeitosTeste) Humano(context.Context, string) { e.humano++ }

const botJSON = `{ "versao": 1, "inicio": "n1", "nao_entendi": "Não entendi.", "max_tentativas": 3, "inatividade_min": 30,
  "nos": [
    { "id": "n1", "tipo": "inicio", "proximo": "n2" },
    { "id": "n2", "tipo": "mensagem", "texto": "Olá, {nome}!", "proximo": "n3" },
    { "id": "n3", "tipo": "menu", "texto": "Como posso ajudar?",
      "opcoes": [ { "rotulo": "Preços", "valores": ["preco", "valores"], "proximo": "n4" },
                  { "rotulo": "Falar com vendedor", "valores": [], "proximo": "n8" } ], "mostrar_numeros": true },
    { "id": "n4", "tipo": "pergunta", "texto": "Qual seu e-mail?", "variavel": "email",
      "validacao": { "tipo": "email", "mensagem_erro": "E-mail inválido. Tente de novo." }, "proximo": "n5" },
    { "id": "n5", "tipo": "condicao",
      "ramos": [ { "condicoes": { "modo": "todas", "regras": [ { "tipo": "variavel", "variavel": "email", "operador": "contem", "valor": "@empresa.com" } ] }, "proximo": "n6" } ],
      "senao": "n7" },
    { "id": "n6", "tipo": "acao", "acao": { "tipo": "atualizar_campo_lead", "campo": "email", "valor": "{email}" }, "proximo": "n7" },
    { "id": "n7", "tipo": "ia", "automacao_id": "01JIA", "modo": "responder", "proximo": "n9", "em_erro": "n8" },
    { "id": "n8", "tipo": "humano", "mensagem": "Vou chamar um atendente." },
    { "id": "n9", "tipo": "fim", "mensagem": "Obrigado, {email}!" }
  ] }`

func bot(t *testing.T) *modelo.DefinicaoChatbot {
	d, errs := modelo.DecodificarChatbot(json.RawMessage(botJSON))
	if errs != nil || len(modelo.ValidarChatbot(d)) != 0 {
		t.Fatalf("bot inválido: %v %v", errs, modelo.ValidarChatbot(d))
	}
	return d
}

var ctx = context.Background()

func TestCaminhoFeliz(t *testing.T) {
	d, ef := bot(t), &efeitosTeste{iaRet: json.RawMessage(`"Nosso plano custa R$ 50."`)}
	e := Iniciar(ctx, d, ef, nil)
	if e.NoAtual != "n3" || e.Final != "" || len(ef.enviados) != 2 || ef.enviados[0] != "Olá, Ana!" ||
		ef.enviados[1] != "Como posso ajudar?\n1 - Preços\n2 - Falar com vendedor" {
		t.Fatalf("início: %+v %q", e, ef.enviados)
	}
	for _, r := range []string{"1", "1.", "1)", "preços", "VALORES"} {
		if prox, ok := escolherOpcao(mustNo(d, "n3"), r); !ok || prox != "n4" {
			t.Errorf("menu não reconheceu %q", r)
		}
	}
	e = Responder(ctx, d, ef, e, Resposta{Texto: "2)"})
	if e.Final != dominio.SessaoHumano || ef.humano != 1 || ef.enviados[len(ef.enviados)-1] != "Vou chamar um atendente." {
		t.Fatalf("opção humano: %+v", e)
	}
	ef = &efeitosTeste{iaRet: json.RawMessage(`"Nosso plano custa R$ 50."`)}
	e = Iniciar(ctx, d, ef, nil)
	e = Responder(ctx, d, ef, e, Resposta{Texto: "Preços"})
	if e.NoAtual != "n4" || ef.enviados[len(ef.enviados)-1] != "Qual seu e-mail?" {
		t.Fatalf("pergunta: %+v", e)
	}
	e = Responder(ctx, d, ef, e, Resposta{Texto: "Joao@Empresa.com"})
	if e.Final != dominio.SessaoConcluida || e.Variaveis["email"] != "joao@empresa.com" ||
		len(ef.acoes) != 1 || ef.acoes[0] != "atualizar_campo_lead:email=joao@empresa.com" {
		t.Fatalf("fim: %+v %v", e, ef.acoes)
	}
	n := len(ef.enviados)
	if ef.enviados[n-2] != "Nosso plano custa R$ 50." || ef.enviados[n-1] != "Obrigado, joao@empresa.com!" {
		t.Fatalf("IA responder + fim: %q", ef.enviados)
	}
}

func mustNo(d *modelo.DefinicaoChatbot, id string) modelo.No {
	n, _ := d.No(id)
	return n
}

func TestNaoEntendiETentativas(t *testing.T) {
	d, ef := bot(t), &efeitosTeste{}
	e := Iniciar(ctx, d, ef, nil)
	e = Responder(ctx, d, ef, e, Resposta{Texto: "hein?"})
	if e.Tentativas != 1 || e.NoAtual != "n3" || ef.enviados[len(ef.enviados)-2] != "Não entendi." || !strings.HasPrefix(ef.enviados[len(ef.enviados)-1], "Como posso ajudar?") {
		t.Fatalf("não entendi: %+v %q", e, ef.enviados)
	}
	e = Responder(ctx, d, ef, e, Resposta{TemMidia: true}) // mídia sem texto = inválida
	if e.Tentativas != 2 {
		t.Fatalf("mídia: %+v", e)
	}
	e = Responder(ctx, d, ef, e, Resposta{Texto: "7"})
	if e.Final != dominio.SessaoHumano || ef.humano != 1 {
		t.Fatalf("esgotou sem destino → humano: %+v", e)
	}
	// Com ao_esgotar definido, segue para o nó.
	d2 := bot(t)
	for i := range d2.Nos {
		if d2.Nos[i].ID == "n4" {
			alvo := "n9"
			d2.Nos[i].AoEsgotar = &alvo
		}
	}
	d2.MaxTentativas = 1
	ef = &efeitosTeste{}
	e = Iniciar(ctx, d2, ef, nil)
	e = Responder(ctx, d2, ef, e, Resposta{Texto: "1"})
	e = Responder(ctx, d2, ef, e, Resposta{Texto: "email-ruim"})
	// fim usa {email}, que não foi capturado: o envio falha mas a sessão conclui.
	if e.Final != dominio.SessaoConcluida {
		t.Fatalf("ao_esgotar: %+v", e)
	}
}

func TestValidacoes(t *testing.T) {
	casos := []struct {
		tipo, padrao, texto string
		ok                  bool
		valor               string
	}{
		{"email", "", "a@b.co", true, "a@b.co"}, {"email", "", "a@b", false, ""},
		{"numero", "", "42", true, "42"}, {"numero", "", "3,5", true, "3,5"}, {"numero", "", "1.234,56", true, "1.234,56"}, {"numero", "", "abc", false, ""},
		{"telefone", "", "(11) 99999-0000", true, "+5511999990000"}, {"telefone", "", "123", false, ""},
		{"regex", `^\d{5}-?\d{3}$`, "01310-100", true, "01310-100"}, {"regex", `^\d{5}$`, "abc", false, ""},
		{"nenhuma", "", "qualquer", true, "qualquer"},
	}
	for _, c := range casos {
		v := &modelo.ValidacaoPergunta{Tipo: c.tipo}
		if c.padrao != "" {
			p := c.padrao
			v.Padrao = &p
		}
		got, ok := validar(v, c.texto)
		if ok != c.ok || (ok && got != c.valor) {
			t.Errorf("%s %q: %q %v", c.tipo, c.texto, got, ok)
		}
	}
}

func TestIAErroEModoVariavelEBloqueio(t *testing.T) {
	d := bot(t)
	ef := &efeitosTeste{iaErro: errors.New("falhou")}
	e := Iniciar(ctx, d, ef, nil)
	e = Responder(ctx, d, ef, e, Resposta{Texto: "1"})
	e = Responder(ctx, d, ef, e, Resposta{Texto: "x@outro.com"})
	if e.Final != dominio.SessaoHumano || len(ef.acoes) != 0 {
		t.Fatalf("erro da IA vai para em_erro (humano): %+v", e)
	}
	// Modo variável guarda o retorno.
	for i := range d.Nos {
		if d.Nos[i].ID == "n7" {
			v := "resposta"
			d.Nos[i].ModoIA, d.Nos[i].Variavel = "variavel", &v
		}
	}
	ef = &efeitosTeste{iaRet: json.RawMessage(`{"nota":9}`)}
	e = Iniciar(ctx, d, ef, nil)
	e = Responder(ctx, d, ef, e, Resposta{Texto: "1"})
	e = Responder(ctx, d, ef, e, Resposta{Texto: "x@outro.com"})
	if e.Final != dominio.SessaoConcluida || e.Variaveis["resposta"] != `{"nota":9}` {
		t.Fatalf("modo variável: %+v", e)
	}
	// Envio barrado por anti-loop encerra a sessão como abortada.
	ef = &efeitosTeste{}
	e = Iniciar(ctx, d, ef, nil)
	ef.bloquear = "anti_loop"
	e = Responder(ctx, d, ef, e, Resposta{Texto: "1"})
	if e.Final != dominio.SessaoAbortada || e.Motivo != "anti-loop" {
		t.Fatalf("anti-loop: %+v", e)
	}
}

func TestLacoSemEspera(t *testing.T) {
	d := &modelo.DefinicaoChatbot{Versao: 1, Inicio: "i", NaoEntendi: "x", MaxTentativas: 1, InatividadeMin: 1, Nos: []modelo.No{
		{ID: "i", Tipo: "inicio", Proximo: "a"},
		{ID: "a", Tipo: "acao", Acao: &modelo.Acao{Tipo: "notificar", Titulo: "t", Texto: "x"}, Proximo: "a"},
	}}
	e := Iniciar(ctx, d, &efeitosTeste{}, nil)
	if e.Final != dominio.SessaoAbortada || e.Motivo != "laço no chatbot" {
		t.Fatalf("laço: %+v", e)
	}
}
