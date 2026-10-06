// Pacote chatbot executa chatbots de nós (data-model.md › Máquina do chatbot): a máquina é pura
// sobre a definição (efeitos por interface), o serviço de sessões guarda o estado por conversa e
// o simulador roda a mesma máquina em memória para o editor.
package chatbot

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/telefone"
)

// MaxPassos por avanço (proteção contra laços sem nó de espera).
const MaxPassos = 200

// Envio é o resultado de um envio pelo portão.
type Envio struct {
	Bloqueio string // motivo do portão (vazio = enviado/simulado)
	Erro     string // falha não relacionada ao portão
}

// Efeitos são as operações com efeito colateral usadas pela máquina.
type Efeitos interface {
	// Enviar manda o texto (já com variáveis resolvidas) ou o template do nó.
	Enviar(ctx context.Context, noID, texto string, templateID *string) Envio
	// Acao executa a ação de um nó "acao" (variáveis da sessão disponíveis).
	Acao(ctx context.Context, noID string, a modelo.Acao, vars map[string]string) Envio
	// IA executa a automação de IA do nó (aoExecutar) e devolve o retorno.
	IA(ctx context.Context, no modelo.No, vars map[string]string) (json.RawMessage, error)
	// Condicao avalia um ramo com as variáveis da sessão.
	Condicao(ctx context.Context, c modelo.Condicoes, vars map[string]string) bool
	// Resolver aplica as variáveis ao texto.
	Resolver(ctx context.Context, texto string, vars map[string]string) (string, error)
	// Humano coloca a conversa em atendimento humano e notifica (depois da mensagem do nó).
	Humano(ctx context.Context, noID string)
}

// Estado da sessão.
type Estado struct {
	NoAtual    string
	Variaveis  map[string]string
	Tentativas int
	Final      string // "" (ativa) | concluida | humano | abortada
	Motivo     string
}

// Resposta do contato.
type Resposta struct {
	Texto    string
	TemMidia bool
}

func copiar(e Estado) Estado {
	v := map[string]string{}
	for k, x := range e.Variaveis {
		v[k] = x
	}
	e.Variaveis = v
	return e
}

// Iniciar roda a partir do nó de início até o primeiro nó que espera resposta (ou o fim).
func Iniciar(ctx context.Context, d *modelo.DefinicaoChatbot, ef Efeitos, vars map[string]string) Estado {
	e := Estado{Variaveis: vars}
	if e.Variaveis == nil {
		e.Variaveis = map[string]string{}
	}
	return avancar(ctx, d, ef, e, d.Inicio)
}

// Responder trata a resposta do contato no nó atual.
func Responder(ctx context.Context, d *modelo.DefinicaoChatbot, ef Efeitos, est Estado, r Resposta) Estado {
	e := copiar(est)
	no, ok := d.No(e.NoAtual)
	if !ok || e.Final != "" {
		e.Final, e.Motivo = dominio.SessaoAbortada, "nó atual inexistente"
		return e
	}
	texto := strings.TrimSpace(r.Texto)
	switch no.Tipo {
	case modelo.NoMenu:
		if !r.TemMidia || texto != "" {
			if prox, ok := escolherOpcao(no, texto); ok {
				e.Tentativas = 0
				return avancar(ctx, d, ef, e, prox)
			}
		}
		return invalida(ctx, d, ef, e, no, d.NaoEntendi, true)
	case modelo.NoPergunta:
		if texto != "" {
			if v, ok := validar(no.Validacao, texto); ok {
				e.Variaveis[*no.Variavel] = v
				e.Tentativas = 0
				return avancar(ctx, d, ef, e, no.Proximo)
			}
		}
		msg := d.NaoEntendi
		if no.Validacao != nil && no.Validacao.MensagemErro != nil && strings.TrimSpace(*no.Validacao.MensagemErro) != "" {
			msg = *no.Validacao.MensagemErro
		}
		return invalida(ctx, d, ef, e, no, msg, false)
	}
	// Nó atual não espera resposta: continua dele.
	return avancar(ctx, d, ef, e, no.ID)
}

// invalida conta a tentativa: abaixo do máximo avisa (e repete o menu); no máximo segue ao_esgotar
// (padrão: atendimento humano).
func invalida(ctx context.Context, d *modelo.DefinicaoChatbot, ef Efeitos, e Estado, no modelo.No, msg string, repetirMenu bool) Estado {
	e.Tentativas++
	if e.Tentativas >= d.MaxTentativas {
		e.Tentativas = 0
		if no.AoEsgotar != nil && *no.AoEsgotar != "" {
			return avancar(ctx, d, ef, e, *no.AoEsgotar)
		}
		ef.Humano(ctx, no.ID)
		e.Final, e.Motivo = dominio.SessaoHumano, "tentativas esgotadas"
		return e
	}
	if env := enviar(ctx, ef, no.ID, msg, nil, e.Variaveis); env.Bloqueio != "" {
		return bloqueado(e, env)
	}
	if repetirMenu {
		if env := enviar(ctx, ef, no.ID, textoMenu(no), nil, e.Variaveis); env.Bloqueio != "" {
			return bloqueado(e, env)
		}
	}
	return e
}

func bloqueado(e Estado, env Envio) Estado {
	e.Final = dominio.SessaoAbortada
	e.Motivo = "envio bloqueado (" + env.Bloqueio + ")"
	if env.Bloqueio == "anti_loop" {
		e.Motivo = "anti-loop"
	}
	return e
}

func enviar(ctx context.Context, ef Efeitos, noID, texto string, tpl *string, vars map[string]string) Envio {
	if tpl == nil || *tpl == "" {
		tpl = nil
		t, err := ef.Resolver(ctx, texto, vars)
		if err != nil {
			return Envio{Erro: err.Error()}
		}
		texto = t
	}
	return ef.Enviar(ctx, noID, texto, tpl)
}

func textoMenu(no modelo.No) string {
	if !no.MostraNumeros() {
		return no.Texto
	}
	var b strings.Builder
	b.WriteString(no.Texto)
	for i, o := range no.Opcoes {
		fmt.Fprintf(&b, "\n%d - %s", i+1, o.Rotulo)
	}
	return b.String()
}

var reNumeroOpcao = regexp.MustCompile(`^\s*(\d{1,2})\s*[.)\-]?\s*$`)

// escolherOpcao aceita o número ("2", "2.", "2)"), o rótulo normalizado ou um dos valores.
func escolherOpcao(no modelo.No, texto string) (string, bool) {
	if m := reNumeroOpcao.FindStringSubmatch(texto); m != nil {
		if n, _ := strconv.Atoi(m[1]); n >= 1 && n <= len(no.Opcoes) {
			return no.Opcoes[n-1].Proximo, true
		}
	}
	t := modelo.NormalizarTexto(texto)
	for _, o := range no.Opcoes {
		if modelo.NormalizarTexto(o.Rotulo) == t {
			return o.Proximo, true
		}
		for _, v := range o.Valores {
			if n := modelo.NormalizarTexto(v); n != "" && n == t {
				return o.Proximo, true
			}
		}
	}
	return "", false
}

var reNumero = regexp.MustCompile(`^-?\d+([.,]\d+)?$`)

// validar aplica a validação da pergunta e devolve o valor a guardar (telefone normalizado).
func validar(v *modelo.ValidacaoPergunta, texto string) (string, bool) {
	if v == nil {
		return texto, true
	}
	switch v.Tipo {
	case "email":
		return strings.ToLower(texto), modelo.EmailValido(texto)
	case "numero":
		t := strings.ReplaceAll(strings.ReplaceAll(texto, " ", ""), ".", "")
		if strings.Count(texto, ".") == 1 && !strings.Contains(texto, ",") {
			t = strings.ReplaceAll(texto, " ", "") // "12.5" é decimal
		}
		return texto, reNumero.MatchString(t)
	case "telefone":
		e164, motivo := telefone.Normalizar(texto, "")
		return e164, motivo == ""
	case "regex":
		if v.Padrao == nil {
			return texto, false
		}
		re, err := regexp.Compile(*v.Padrao)
		return texto, err == nil && re.MatchString(texto)
	}
	return texto, true
}

// avancar executa nós a partir de `id` até um nó que espera resposta ou o fim da sessão.
func avancar(ctx context.Context, d *modelo.DefinicaoChatbot, ef Efeitos, e Estado, id string) Estado {
	for passos := 0; passos < MaxPassos; passos++ {
		no, ok := d.No(id)
		if !ok {
			e.Final, e.Motivo = dominio.SessaoAbortada, fmt.Sprintf("nó %q não existe", id)
			return e
		}
		e.NoAtual = no.ID
		switch no.Tipo {
		case modelo.NoInicio:
			id = no.Proximo
		case modelo.NoMensagem:
			if env := enviar(ctx, ef, no.ID, no.Texto, no.TemplateID, e.Variaveis); env.Bloqueio != "" {
				return bloqueado(e, env)
			}
			id = no.Proximo
		case modelo.NoMenu:
			if env := enviar(ctx, ef, no.ID, textoMenu(no), nil, e.Variaveis); env.Bloqueio != "" {
				return bloqueado(e, env)
			}
			return e
		case modelo.NoPergunta:
			if env := enviar(ctx, ef, no.ID, no.Texto, nil, e.Variaveis); env.Bloqueio != "" {
				return bloqueado(e, env)
			}
			return e
		case modelo.NoCondicao:
			id = no.Senao
			for _, r := range no.Ramos {
				if ef.Condicao(ctx, r.Condicoes, e.Variaveis) {
					id = r.Proximo
					break
				}
			}
		case modelo.NoAcao:
			if no.Acao != nil {
				if env := ef.Acao(ctx, no.ID, *no.Acao, e.Variaveis); env.Bloqueio != "" {
					return bloqueado(e, env)
				}
			}
			id = no.Proximo
		case modelo.NoIA:
			ret, err := ef.IA(ctx, no, e.Variaveis)
			if err != nil {
				if no.EmErro != nil && *no.EmErro != "" {
					id = *no.EmErro
					continue
				}
				ef.Humano(ctx, no.ID)
				e.Final, e.Motivo = dominio.SessaoHumano, "erro na IA"
				return e
			}
			texto := textoRetorno(ret)
			if no.ModoIA == "variavel" && no.Variavel != nil {
				e.Variaveis[*no.Variavel] = texto
			} else if strings.TrimSpace(texto) != "" {
				if env := ef.Enviar(ctx, no.ID, texto, nil); env.Bloqueio != "" {
					return bloqueado(e, env)
				}
			}
			id = no.Proximo
		case modelo.NoHumano:
			if no.MensagemHumFim != nil && strings.TrimSpace(*no.MensagemHumFim) != "" {
				enviar(ctx, ef, no.ID, *no.MensagemHumFim, nil, e.Variaveis)
			}
			ef.Humano(ctx, no.ID)
			e.Final, e.Motivo = dominio.SessaoHumano, "transferido para atendimento humano"
			return e
		case modelo.NoFim:
			if no.MensagemHumFim != nil && strings.TrimSpace(*no.MensagemHumFim) != "" {
				if env := enviar(ctx, ef, no.ID, *no.MensagemHumFim, nil, e.Variaveis); env.Bloqueio != "" {
					return bloqueado(e, env)
				}
			}
			e.Final = dominio.SessaoConcluida
			return e
		default:
			e.Final, e.Motivo = dominio.SessaoAbortada, "tipo de nó desconhecido"
			return e
		}
	}
	e.Final, e.Motivo = dominio.SessaoAbortada, "laço no chatbot"
	return e
}

func textoRetorno(r json.RawMessage) string {
	var s string
	if json.Unmarshal(r, &s) == nil {
		return s
	}
	if string(r) == "null" {
		return ""
	}
	return string(r)
}
