package acoes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/automacoes/condicoes"
	"zapdesk/motor/internal/automacoes/execucoes"
	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/automacoes/seguranca"
	"zapdesk/motor/internal/chat"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/fatos"
	"zapdesk/motor/internal/funil"
	"zapdesk/motor/internal/leads"
	"zapdesk/motor/internal/organizacao"
	"zapdesk/motor/internal/relogio"
)

// Disparos acrescenta leads a um disparo (ação adicionar_a_disparo; preenchido na US2).
type Disparos interface {
	AdicionarLeads(ctx context.Context, disparoID string, leadIDs []string) (adicionados, jaExistiam int, err error)
}

// Chatbots inicia um chatbot numa conversa (ação iniciar_chatbot; preenchido na US4). Devolve
// (false, nil) se a conversa já tem sessão ativa (a ação é ignorada).
type Chatbots interface {
	IniciarPorAcao(ctx context.Context, automacaoID string, c *Contexto) (bool, error)
}

// IA executa o handler aoExecutar de uma automação de IA (ação executar_ia e nó ia; US3).
type IA interface {
	ExecutarPorAcao(ctx context.Context, automacaoID string, entrada map[string]any, c *Contexto) (json.RawMessage, error)
}

// Deps são os serviços usados pelas ações.
type Deps struct {
	Banco       *armazenamento.Banco
	Barramento  *eventos.Barramento
	Relogio     relogio.Relogio
	Chat        *chat.Servico
	Organizacao *organizacao.Servico
	Leads       *leads.Servico
	Funil       *funil.Servico
	Portao      *seguranca.Portao
	Disparos    Disparos
	Chatbots    Chatbots
	IA          IA
}

// Executores das ações.
type Executores struct{ d Deps }

// Novo cria os executores.
func Novo(d Deps) *Executores { return &Executores{d: d} }

// DefinirDisparos, DefinirChatbots e DefinirIA ligam as dependências das histórias seguintes.
func (x *Executores) DefinirDisparos(d Disparos) { x.d.Disparos = d }

// DefinirChatbots liga o serviço de sessões.
func (x *Executores) DefinirChatbots(c Chatbots) { x.d.Chatbots = c }

// DefinirIA liga o serviço de IA.
func (x *Executores) DefinirIA(i IA) { x.d.IA = i }

// Alvo de uma execução.
type Alvo struct {
	ContaID, ConversaID, ContatoID, LeadID string
}

// Contexto de uma execução de ações.
type Contexto struct {
	Exec             *execucoes.Exec // nil no simulador do chatbot
	Automacao        dominio.Automacao
	Alvo             Alvo
	Simulacao        bool
	Cadeia           []string          // inclui a automação atual
	Variaveis        map[string]string // variáveis da execução/sessão
	UltimaMensagem   *string           // texto da mensagem do gatilho
	Origem           string            // "fluxo" | "chatbot" (origem das execuções filhas)
	ConversaFicticia bool
	IASimulada       bool
}

// Resultado de uma ação.
type Resultado struct {
	Registro dominio.AcaoRegistrada
	Fatal    error           // erro interno: a execução termina em "erro"
	Bloqueio string          // motivo do portão (anti_loop, pausa…) quando bloqueada
	Texto    string          // texto enviado (ou que seria enviado)
	Retorno  json.RawMessage // executar_ia
}

func (x *Executores) agora() dominio.AcaoRegistrada {
	return dominio.AcaoRegistrada{Em: x.d.Relogio.Agora()}
}

func reg(tipo, resultado, alvo, detalhe string, em dominio.AcaoRegistrada) dominio.AcaoRegistrada {
	em.Tipo, em.Resultado = tipo, resultado
	if alvo != "" {
		em.Alvo = &alvo
	}
	if detalhe != "" {
		d := execucoes.Resumo(detalhe, 500)
		em.Detalhe = &d
	}
	return em
}

func (x *Executores) ok(tipo, alvo, detalhe string) Resultado {
	return Resultado{Registro: reg(tipo, dominio.AcaoOK, alvo, detalhe, x.agora())}
}

func (x *Executores) falhou(tipo, alvo, detalhe string) Resultado {
	return Resultado{Registro: reg(tipo, dominio.AcaoFalhou, alvo, detalhe, x.agora())}
}

func (x *Executores) simulada(tipo, alvo, detalhe string) Resultado {
	return Resultado{Registro: reg(tipo, dominio.AcaoSimulada, alvo, detalhe, x.agora())}
}

// mensagemErro traduz erros de domínio em texto exibível; erros internos viram Fatal.
func mensagemErro(err error) (string, bool) {
	if e, ok := erros.Como(err); ok {
		return e.Mensagem, true
	}
	var ev *ErroVariavel
	if errors.As(err, &ev) {
		return ev.Error(), true
	}
	return "", false
}

func (x *Executores) falhaOuFatal(tipo, alvo string, err error) Resultado {
	if msg, ok := mensagemErro(err); ok {
		return x.falhou(tipo, alvo, msg)
	}
	r := x.falhou(tipo, alvo, "Erro interno: "+err.Error())
	r.Fatal = err
	return r
}

// CompletarAlvo preenche contato/lead/conta a partir do que já se sabe (conversa → contato → lead).
func (x *Executores) CompletarAlvo(ctx context.Context, c *Contexto) {
	l := x.d.Banco.L()
	if c.Alvo.ConversaID != "" && (c.Alvo.ContatoID == "" || c.Alvo.ContaID == "") {
		if cv, err := armazenamento.ObterConversa(ctx, l, c.Alvo.ConversaID); err == nil {
			c.Alvo.ContaID = cv.ContaID
			if cv.ContatoID != nil && c.Alvo.ContatoID == "" {
				c.Alvo.ContatoID = *cv.ContatoID
			}
		}
	}
	if c.Alvo.ContatoID != "" && c.Alvo.LeadID == "" {
		if ct, err := armazenamento.ObterContato(ctx, l, c.Alvo.ContatoID); err == nil && ct.Lead != nil {
			c.Alvo.LeadID = ct.Lead.ID
		}
	}
	if c.Alvo.LeadID != "" && c.Alvo.ContatoID == "" {
		if ld, err := armazenamento.ObterLead(ctx, l, c.Alvo.LeadID); err == nil {
			var ct string
			if l.QueryRowContext(ctx, `SELECT id FROM contatos WHERE telefone = ? ORDER BY atualizado_em DESC LIMIT 1`, ld.Telefone).Scan(&ct) == nil {
				c.Alvo.ContatoID = ct
			}
		}
	}
}

// Fonte monta os valores de variáveis da execução.
func (x *Executores) Fonte(ctx context.Context, c *Contexto) FonteVariaveis {
	l := x.d.Banco.L()
	f := FonteVariaveis{Execucao: c.Variaveis}
	if c.ConversaFicticia {
		f.NomeContato, f.Telefone = "Contato de teste", "+5500000000000"
	}
	if c.Alvo.LeadID != "" {
		if ld, err := armazenamento.ObterLead(ctx, l, c.Alvo.LeadID); err == nil {
			f.CamposLead, f.Telefone = ld.Campos, ld.Telefone
			if ld.Nome != nil {
				f.NomeLead = *ld.Nome
			}
		}
	}
	if c.Alvo.ContatoID != "" {
		if ct, err := armazenamento.ObterContato(ctx, l, c.Alvo.ContatoID); err == nil {
			if ct.Nome != nil {
				f.NomeContato = *ct.Nome
			}
			if ct.NomePush != nil {
				f.NomePush = *ct.NomePush
			}
			if f.Telefone == "" && ct.Telefone != nil {
				f.Telefone = *ct.Telefone
			}
		}
	}
	if c.Alvo.ContaID != "" {
		if ca, err := armazenamento.ObterConta(ctx, l, c.Alvo.ContaID); err == nil {
			f.Conta = ca.Nome
		}
	}
	if c.UltimaMensagem != nil {
		f.UltimaMensagem = *c.UltimaMensagem
	} else if c.Alvo.ConversaID != "" {
		var t *string
		l.QueryRowContext(ctx, `SELECT texto FROM mensagens WHERE conversa_id = ? AND de_mim = 0 ORDER BY enviada_em DESC, rid DESC LIMIT 1`,
			c.Alvo.ConversaID).Scan(&t)
		if t != nil {
			f.UltimaMensagem = *t
		}
	}
	return f
}

// Executar roda uma ação (exceto "aguardar", tratada pelo executor de fluxo).
func (x *Executores) Executar(ctx context.Context, c *Contexto, a modelo.Acao) Resultado {
	ctx = fatos.ComCadeia(ctx, c.Cadeia)
	x.CompletarAlvo(ctx, c)
	switch a.Tipo {
	case modelo.AcaoEnviarTexto:
		txt, err := Resolver(a.Texto, x.Fonte(ctx, c), a.ValoresPadrao)
		if err != nil {
			return x.falhaOuFatal(a.Tipo, "", err)
		}
		return x.Enviar(ctx, c, a.Tipo, txt, "")
	case modelo.AcaoEnviarTemplate:
		t, err := armazenamento.ObterTemplate(ctx, x.d.Banco.L(), a.TemplateID)
		if err != nil {
			return x.falhou(a.Tipo, "", "Template não encontrado.")
		}
		txt, err := Resolver(t.Texto, x.Fonte(ctx, c), a.ValoresPadrao)
		if err != nil {
			return x.falhaOuFatal(a.Tipo, t.Nome, err)
		}
		arq := ""
		if t.ArquivoID != nil {
			arq = *t.ArquivoID
		}
		r := x.Enviar(ctx, c, a.Tipo, txt, arq)
		if r.Registro.Alvo == nil {
			n := t.Nome
			r.Registro.Alvo = &n
		}
		return r
	case modelo.AcaoAdicionarEtiqueta, modelo.AcaoRemoverEtiqueta:
		return x.etiqueta(ctx, c, a)
	case modelo.AcaoAtualizarNota:
		return x.nota(ctx, c, a)
	case modelo.AcaoAtualizarCampoLead:
		return x.campoLead(ctx, c, a)
	case modelo.AcaoMoverEtapa, modelo.AcaoRemoverDoFunil:
		return x.funil(ctx, c, a)
	case modelo.AcaoPausarAutomacoes:
		if c.Alvo.ConversaID == "" {
			return x.falhou(a.Tipo, "", "Esta execução não tem conversa.")
		}
		prazo := "sem prazo"
		if a.DuracaoMin != nil {
			prazo = fmt.Sprintf("%d min", *a.DuracaoMin)
		}
		if c.Simulacao {
			return x.simulada(a.Tipo, "", "Pausaria as automações nesta conversa ("+prazo+").")
		}
		x.d.Portao.Pausas().PausarPorAutomacao(ctx, c.Alvo.ConversaID, c.Automacao.ID, a.DuracaoMin)
		return x.ok(a.Tipo, "", "Automações pausadas nesta conversa ("+prazo+").")
	case modelo.AcaoNotificar:
		fonte := x.Fonte(ctx, c)
		titulo, err := Resolver(a.Titulo, fonte, nil)
		if err != nil {
			return x.falhaOuFatal(a.Tipo, "", err)
		}
		texto, err := Resolver(a.Texto, fonte, nil)
		if err != nil {
			return x.falhaOuFatal(a.Tipo, "", err)
		}
		titulo, texto = execucoes.Resumo(titulo, modelo.MaxTituloNotificar), execucoes.Resumo(texto, modelo.MaxTextoNotificar)
		if c.Simulacao {
			return x.simulada(a.Tipo, titulo, "Notificaria: "+texto)
		}
		n := dominio.Notificacao{Titulo: titulo, Corpo: texto, Tipo: "acao"}
		aut := c.Automacao.ID
		n.AutomacaoID = &aut
		if c.Alvo.ConversaID != "" {
			cv := c.Alvo.ConversaID
			n.ConversaID = &cv
		}
		x.d.Barramento.Publicar(eventos.Notificacao, c.Alvo.ContaID, n)
		return x.ok(a.Tipo, titulo, texto)
	case modelo.AcaoIniciarChatbot:
		if x.d.Chatbots == nil {
			return x.falhou(a.Tipo, "", "Chatbots indisponíveis.")
		}
		if c.Alvo.ConversaID == "" {
			return x.falhou(a.Tipo, "", "Esta execução não tem conversa.")
		}
		if c.Simulacao {
			return x.simulada(a.Tipo, a.AutomacaoID, "Iniciaria o chatbot nesta conversa.")
		}
		iniciou, err := x.d.Chatbots.IniciarPorAcao(ctx, a.AutomacaoID, c)
		if err != nil {
			return x.falhaOuFatal(a.Tipo, a.AutomacaoID, err)
		}
		if !iniciou {
			return x.ok(a.Tipo, a.AutomacaoID, "Ignorado: a conversa já tem um chatbot ativo.")
		}
		return x.ok(a.Tipo, a.AutomacaoID, "Chatbot iniciado.")
	case modelo.AcaoExecutarIA:
		if x.d.IA == nil {
			return x.falhou(a.Tipo, a.AutomacaoID, "Automações de IA indisponíveis.")
		}
		entrada, err := ResolverEntrada(a.Entrada, x.Fonte(ctx, c))
		if err != nil {
			return x.falhaOuFatal(a.Tipo, a.AutomacaoID, err)
		}
		ret, err := x.d.IA.ExecutarPorAcao(ctx, a.AutomacaoID, entrada, c)
		if err != nil {
			return x.falhaOuFatal(a.Tipo, a.AutomacaoID, err)
		}
		r := x.ok(a.Tipo, a.AutomacaoID, execucoes.Resumo(string(ret), 200))
		if c.Simulacao {
			r.Registro.Resultado = dominio.AcaoSimulada
		}
		r.Retorno = ret
		return r
	case modelo.AcaoAdicionarADisparo:
		if x.d.Disparos == nil {
			return x.falhou(a.Tipo, a.DisparoID, "Disparos indisponíveis.")
		}
		lead, err := x.garantirLead(ctx, c)
		if err != nil {
			return x.falhaOuFatal(a.Tipo, a.DisparoID, err)
		}
		if c.Simulacao {
			return x.simulada(a.Tipo, a.DisparoID, "Adicionaria o lead ao disparo.")
		}
		novos, _, err := x.d.Disparos.AdicionarLeads(ctx, a.DisparoID, []string{lead})
		if err != nil {
			return x.falhaOuFatal(a.Tipo, a.DisparoID, err)
		}
		if novos == 0 {
			return x.ok(a.Tipo, a.DisparoID, "O lead já estava no disparo.")
		}
		return x.ok(a.Tipo, a.DisparoID, "Lead adicionado ao disparo.")
	case modelo.AcaoAguardar:
		return x.falhou(a.Tipo, "", "Aguardar não é permitido aqui.")
	}
	return x.falhou(a.Tipo, "", "Ação desconhecida.")
}

// Enviar manda um texto (e anexo opcional) na conversa do alvo, pelo portão. Sem conversa, abre a
// conversa com o telefone do lead/contato pela conta de envio (ou a conta do evento).
func (x *Executores) Enviar(ctx context.Context, c *Contexto, tipo, texto, arquivoID string) Resultado {
	if strings.TrimSpace(texto) == "" && arquivoID == "" {
		return x.falhou(tipo, "", "Texto vazio.")
	}
	if utf8.RuneCountInString(texto) > modelo.MaxTextoMensagem {
		return x.falhou(tipo, "", "A mensagem pode ter até 4.096 caracteres.")
	}
	fonte := x.Fonte(ctx, c)
	alvo := fonte.Telefone
	if c.Simulacao {
		r := x.simulada(tipo, alvo, "Enviaria: "+texto)
		r.Texto = texto
		return r
	}
	if c.Alvo.ConversaID == "" {
		conta := c.Alvo.ContaID
		if c.Automacao.ContaEnvioID != nil && *c.Automacao.ContaEnvioID != "" {
			conta = *c.Automacao.ContaEnvioID
		}
		if conta == "" || fonte.Telefone == "" {
			return x.falhou(tipo, alvo, "Esta execução não tem conversa nem conta de envio.")
		}
		cv, _, err := x.d.Chat.NovaConversa(ctx, conta, fonte.Telefone)
		if err != nil {
			return x.falhaOuFatal(tipo, alvo, err)
		}
		c.Alvo.ContaID, c.Alvo.ConversaID = cv.ContaID, cv.ID
		if cv.ContatoID != nil {
			c.Alvo.ContatoID = *cv.ContatoID
		}
		if c.Exec != nil {
			c.Exec.DefinirAlvo(ctx, execucoes.Alvo{ContaID: c.Alvo.ContaID, ConversaID: c.Alvo.ConversaID, ContatoID: c.Alvo.ContatoID})
		}
	}
	var arq *string
	if arquivoID != "" {
		arq = &arquivoID
	}
	txt := texto
	d, err := x.d.Portao.AutorizarEEnviar(ctx, seguranca.Pedido{ConversaID: c.Alvo.ConversaID, AutomacaoID: c.Automacao.ID,
		AutomacaoNome: c.Automacao.Nome, IncluirGrupos: c.Automacao.IncluirGrupos, AntiLoop: c.Automacao.Limites.AntiLoop},
		func(d seguranca.Decisao) error {
			_, err := x.d.Chat.Enviar(ctx, c.Alvo.ConversaID, chat.Envio{Texto: &txt, ArquivoID: arq,
				Automatico: &chat.EnvioAutomatico{AutomacaoID: c.Automacao.ID, PrimeiroContato: d.PrimeiroContato}})
			return err
		})
	if !d.Permitido {
		r := Resultado{Registro: reg(tipo, dominio.AcaoBloqueada, alvo, d.Mensagem, x.agora()), Bloqueio: d.Motivo}
		return r
	}
	if err != nil {
		return x.falhaOuFatal(tipo, alvo, err)
	}
	r := x.ok(tipo, alvo, texto)
	r.Texto = texto
	return r
}

// garantirLead devolve o lead do alvo, criando-o a partir do contato se preciso.
func (x *Executores) garantirLead(ctx context.Context, c *Contexto) (string, error) {
	if c.Alvo.LeadID != "" {
		return c.Alvo.LeadID, nil
	}
	if c.Alvo.ContatoID == "" {
		return "", erros.Novo(erros.Validacao, "Esta execução não tem contato.")
	}
	if c.Simulacao {
		return "", erros.Novo(erros.Validacao, "O contato ainda não é um lead (seria criado).")
	}
	l, _, err := x.d.Leads.GarantirLeadDoContato(ctx, c.Alvo.ContatoID)
	if err != nil {
		return "", err
	}
	c.Alvo.LeadID = l.ID
	if c.Exec != nil {
		c.Exec.DefinirAlvo(ctx, execucoes.Alvo{LeadID: l.ID})
	}
	return l.ID, nil
}

func (x *Executores) etiqueta(ctx context.Context, c *Contexto, a modelo.Acao) Resultado {
	e, err := armazenamento.ObterEtiqueta(ctx, x.d.Banco.L(), a.EtiquetaID)
	if err != nil {
		return x.falhou(a.Tipo, "", "Etiqueta não encontrada.")
	}
	if c.Alvo.ContatoID == "" {
		return x.falhou(a.Tipo, e.Nome, "Esta execução não tem contato.")
	}
	adicionar := a.Tipo == modelo.AcaoAdicionarEtiqueta
	if c.Simulacao {
		verbo := "Removeria"
		if adicionar {
			verbo = "Adicionaria"
		}
		return x.simulada(a.Tipo, e.Nome, verbo+" a etiqueta "+e.Nome+".")
	}
	ct, err := armazenamento.ObterContato(ctx, x.d.Banco.L(), c.Alvo.ContatoID)
	if err != nil {
		return x.falhaOuFatal(a.Tipo, e.Nome, err)
	}
	var novas []string
	tinha := false
	for _, et := range ct.Etiquetas {
		if et.ID == e.ID {
			tinha = true
			if !adicionar {
				continue
			}
		}
		novas = append(novas, et.ID)
	}
	if adicionar == tinha {
		return x.ok(a.Tipo, e.Nome, "Sem mudança.")
	}
	if adicionar {
		novas = append(novas, e.ID)
	}
	if _, err := x.d.Organizacao.DefinirEtiquetas(ctx, ct.ID, novas); err != nil {
		return x.falhaOuFatal(a.Tipo, e.Nome, err)
	}
	return x.ok(a.Tipo, e.Nome, "")
}

func (x *Executores) nota(ctx context.Context, c *Contexto, a modelo.Acao) Resultado {
	if c.Alvo.ContatoID == "" {
		return x.falhou(a.Tipo, "", "Esta execução não tem contato.")
	}
	txt, err := Resolver(a.Texto, x.Fonte(ctx, c), nil)
	if err != nil {
		return x.falhaOuFatal(a.Tipo, "", err)
	}
	ct, err := armazenamento.ObterContato(ctx, x.d.Banco.L(), c.Alvo.ContatoID)
	if err != nil {
		return x.falhaOuFatal(a.Tipo, "", err)
	}
	nova := txt
	if a.Modo == "acrescentar" && ct.Notas != nil && *ct.Notas != "" {
		nova = *ct.Notas + "\n" + txt
	}
	if utf8.RuneCountInString(nova) > modelo.MaxNota {
		return x.falhou(a.Tipo, "", "A nota passaria de 10.000 caracteres.")
	}
	if c.Simulacao {
		return x.simulada(a.Tipo, "", "Gravaria a nota: "+txt)
	}
	if _, err := x.d.Organizacao.DefinirNotas(ctx, ct.ID, &nova); err != nil {
		return x.falhaOuFatal(a.Tipo, "", err)
	}
	return x.ok(a.Tipo, "", txt)
}

func (x *Executores) campoLead(ctx context.Context, c *Contexto, a modelo.Acao) Resultado {
	valor := ""
	if a.Valor != nil {
		v, err := Resolver(*a.Valor, x.Fonte(ctx, c), nil)
		if err != nil {
			return x.falhaOuFatal(a.Tipo, a.Campo, err)
		}
		valor = v
	}
	if c.Simulacao {
		if valor == "" {
			return x.simulada(a.Tipo, a.Campo, "Removeria o campo.")
		}
		return x.simulada(a.Tipo, a.Campo, "Gravaria: "+valor)
	}
	lead, err := x.garantirLead(ctx, c)
	if err != nil {
		return x.falhaOuFatal(a.Tipo, a.Campo, err)
	}
	alt := leads.AlteracaoLead{}
	if strings.EqualFold(strings.TrimSpace(a.Campo), "nome") {
		alt.NomeDefinido = true
		if valor != "" {
			alt.Nome = &valor
		}
	} else {
		var v *string
		if valor != "" {
			v = &valor
		}
		alt.Campos = map[string]*string{a.Campo: v}
	}
	if _, err := x.d.Leads.Atualizar(ctx, lead, alt); err != nil {
		return x.falhaOuFatal(a.Tipo, a.Campo, err)
	}
	return x.ok(a.Tipo, a.Campo, valor)
}

// DadosCondicoes monta os dados para avaliar condições no contexto da execução.
func (x *Executores) DadosCondicoes(ctx context.Context, c *Contexto) condicoes.Dados {
	x.CompletarAlvo(ctx, c)
	l := x.d.Banco.L()
	d := condicoes.Dados{Agora: x.d.Relogio.Agora(), ContaID: c.Alvo.ContaID, Etiquetas: map[string]bool{},
		Posicoes: map[string]string{}, Texto: c.UltimaMensagem, Variaveis: c.Variaveis}
	if c.Alvo.ContatoID != "" {
		if ct, err := armazenamento.ObterContato(ctx, l, c.Alvo.ContatoID); err == nil {
			for _, e := range ct.Etiquetas {
				d.Etiquetas[e.ID] = true
			}
		}
	}
	if c.Alvo.LeadID != "" {
		if ld, err := armazenamento.ObterLead(ctx, l, c.Alvo.LeadID); err == nil {
			d.TemLead = true
			d.CamposLead = map[string]string{}
			for k, v := range ld.Campos {
				d.CamposLead[k] = v
			}
			if ld.Nome != nil {
				d.CamposLead["nome"] = *ld.Nome
			}
			linhas, err := l.QueryContext(ctx, `SELECT funil_id, etapa_id FROM posicoes_funil WHERE lead_id = ?`, ld.ID)
			if err == nil {
				for linhas.Next() {
					var f, e string
					if linhas.Scan(&f, &e) == nil {
						d.Posicoes[f] = e
					}
				}
				linhas.Close()
			}
		}
	}
	return d
}
