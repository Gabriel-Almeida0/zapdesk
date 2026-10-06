// Pacote simulador roda chatbots em memória para o chat simulado do editor
// (contracts/api-http.md › Simulador de chatbot): mesma máquina do chatbot, ações em simulação,
// nada gravado além da execução "simulacao" de cada rodada; sessões expiram após 30 min ociosas.
package simulador

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/automacoes/acoes"
	"zapdesk/motor/internal/automacoes/chatbot"
	"zapdesk/motor/internal/automacoes/condicoes"
	"zapdesk/motor/internal/automacoes/execucoes"
	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/ids"
)

// Ociosidade máxima de uma simulação.
const Ociosidade = 30 * time.Minute

// Saida é uma linha do chat simulado.
type Saida struct {
	Tipo  string  `json:"tipo"` // mensagem | acao | aviso
	Texto string  `json:"texto"`
	NoID  *string `json:"no_id"`
}

// Estado devolvido ao editor.
type Estado struct {
	SimulacaoID string                   `json:"simulacao_id,omitempty"`
	Saidas      []Saida                  `json:"saidas"`
	NoAtual     *string                  `json:"no_atual"`
	Variaveis   map[string]string        `json:"variaveis"`
	Estado      string                   `json:"estado"`
	Acoes       []dominio.AcaoRegistrada `json:"acoes,omitempty"`
}

type sessao struct {
	aut    dominio.Automacao
	def    *modelo.DefinicaoChatbot
	est    chatbot.Estado
	alvo   acoes.Alvo
	iaSim  bool
	ultimo time.Time
	mu     sync.Mutex
}

// Deps do simulador.
type Deps struct {
	Banco    *armazenamento.Banco
	Registro *execucoes.Registro
	Acoes    *acoes.Executores
	IA       acoes.IA
	// Alvo resolve o AlvoExecucao do pedido.
	Alvo func(ctx context.Context, conversaID, contatoID, leadID, telefone, contaID string) (acoes.Alvo, error)
}

// Simulador guarda as sessões simuladas.
type Simulador struct {
	d       Deps
	mu      sync.Mutex
	sessoes map[string]*sessao
	agora   func() time.Time
}

// Novo cria o simulador (`agora` do relógio real: a ociosidade é da interface, não do domínio).
func Novo(d Deps) *Simulador {
	return &Simulador{d: d, sessoes: map[string]*sessao{}, agora: time.Now}
}

// DefinirIA liga a IA (nó ia).
func (s *Simulador) DefinirIA(ia acoes.IA) { s.d.IA = ia }

func (s *Simulador) limpar() {
	lim := s.agora().Add(-Ociosidade)
	for id, se := range s.sessoes {
		if se.ultimo.Before(lim) {
			delete(s.sessoes, id)
		}
	}
}

// Pedido de início.
type Pedido struct {
	Definicao json.RawMessage `json:"definicao"`
	Alvo      *struct {
		ConversaID string `json:"conversa_id"`
		ContatoID  string `json:"contato_id"`
		LeadID     string `json:"lead_id"`
		Telefone   string `json:"telefone"`
		ContaID    string `json:"conta_id"`
	} `json:"alvo"`
	IASimulada bool `json:"ia_simulada"`
}

// Iniciar cria a simulação e roda até o primeiro nó que espera resposta.
func (s *Simulador) Iniciar(ctx context.Context, automacaoID string, p Pedido) (Estado, error) {
	a, err := armazenamento.ObterAutomacao(ctx, s.d.Banco.L(), automacaoID)
	if err != nil {
		return Estado{}, err
	}
	if a.Tipo != modelo.TipoChatbot {
		return Estado{}, erros.Campo("tipo", "O chat simulado é só para chatbots.")
	}
	bruto := a.Definicao
	if len(p.Definicao) > 0 && string(p.Definicao) != "null" {
		bruto = p.Definicao
	}
	def, errs := modelo.DecodificarChatbot(bruto)
	if errs == nil {
		modelo.NormalizarChatbot(def)
		errs = modelo.ValidarChatbot(def)
	}
	if len(errs) > 0 {
		msg := "O chatbot tem erros. Confira os nós destacados."
		if len(errs) == 1 {
			msg = errs[0].Mensagem
		}
		return Estado{}, erros.ComDetalhes(erros.DefinicaoInvalida, msg, map[string]any{"erros": errs})
	}
	var alvo acoes.Alvo
	if p.Alvo != nil && s.d.Alvo != nil {
		if alvo, err = s.d.Alvo(ctx, p.Alvo.ConversaID, p.Alvo.ContatoID, p.Alvo.LeadID, p.Alvo.Telefone, p.Alvo.ContaID); err != nil {
			return Estado{}, err
		}
	}
	se := &sessao{aut: a, def: def, alvo: alvo, iaSim: p.IASimulada, ultimo: s.agora()}
	id := ids.NovoEm(s.agora())
	se.mu.Lock()
	defer se.mu.Unlock()
	s.mu.Lock()
	s.limpar()
	s.sessoes[id] = se
	s.mu.Unlock()
	r, err := s.rodada(ctx, se, nil)
	r.SimulacaoID = id
	r.Acoes = nil
	return r, err
}

// Mensagem responde ao nó atual.
func (s *Simulador) Mensagem(ctx context.Context, id, texto string) (Estado, error) {
	s.mu.Lock()
	s.limpar()
	se, ok := s.sessoes[id]
	s.mu.Unlock()
	if !ok {
		return Estado{}, erros.NaoAchada("Simulação")
	}
	se.mu.Lock()
	defer se.mu.Unlock()
	se.ultimo = s.agora()
	if se.est.Final != "" {
		return Estado{}, erros.Transicao("A simulação terminou. Reinicie para testar de novo.", se.est.Final)
	}
	r, err := s.rodada(ctx, se, &texto)
	if r.Acoes == nil {
		r.Acoes = []dominio.AcaoRegistrada{}
	}
	return r, err
}

// Encerrar apaga a simulação.
func (s *Simulador) Encerrar(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessoes[id]; !ok {
		return erros.NaoAchada("Simulação")
	}
	delete(s.sessoes, id)
	return nil
}

// rodada roda a máquina (início ou resposta) com efeitos simulados e grava a execução "simulacao".
func (s *Simulador) rodada(ctx context.Context, se *sessao, texto *string) (Estado, error) {
	g := dominio.GatilhoExecucao{Tipo: modelo.GatilhoManual, Dados: map[string]any{"pedido_por": "simulador"}}
	if texto != nil {
		g = dominio.GatilhoExecucao{Tipo: modelo.GatilhoMensagemRecebida, Dados: map[string]any{"mensagem_id": nil, "texto": *texto}}
	}
	e, err := s.d.Registro.Criar(ctx, execucoes.Nova{Automacao: se.aut, Gatilho: g, Origem: "teste", Simulacao: true,
		Alvo: execucoes.Alvo{ContaID: se.alvo.ContaID, ConversaID: se.alvo.ConversaID, ContatoID: se.alvo.ContatoID, LeadID: se.alvo.LeadID}})
	if err != nil {
		return Estado{}, err
	}
	e.Iniciar(ctx)
	ficticia := se.alvo.ConversaID == "" && se.alvo.ContatoID == "" && se.alvo.LeadID == ""
	c := &acoes.Contexto{Exec: e, Automacao: se.aut, Alvo: se.alvo, Simulacao: true, Cadeia: []string{se.aut.ID},
		Variaveis: se.est.Variaveis, UltimaMensagem: texto, Origem: modelo.TipoChatbot, ConversaFicticia: ficticia, IASimulada: se.iaSim}
	ef := &efeitos{s: s, c: c}
	if texto == nil {
		se.est = chatbot.Iniciar(ctx, se.def, ef, nil)
	} else {
		se.est = chatbot.Responder(ctx, se.def, ef, se.est, chatbot.Resposta{Texto: *texto})
	}
	e.Finalizar(ctx, execucoes.Fim{Estado: dominio.ExecSimulacao})
	r := Estado{Saidas: ef.saidas, Variaveis: se.est.Variaveis, Estado: dominio.SessaoAtiva, Acoes: e.Detalhe().Acoes}
	if r.Saidas == nil {
		r.Saidas = []Saida{}
	}
	if r.Variaveis == nil {
		r.Variaveis = map[string]string{}
	}
	if se.est.Final != "" {
		r.Estado = se.est.Final
	} else {
		n := se.est.NoAtual
		r.NoAtual = &n
	}
	return r, nil
}

type efeitos struct {
	s      *Simulador
	c      *acoes.Contexto
	saidas []Saida
}

func ptr(s string) *string { return &s }

func (e *efeitos) saida(tipo, texto, no string) {
	e.saidas = append(e.saidas, Saida{Tipo: tipo, Texto: texto, NoID: ptr(no)})
}

func (e *efeitos) Enviar(ctx context.Context, noID, texto string, tpl *string) chatbot.Envio {
	if tpl != nil {
		t, err := armazenamento.ObterTemplate(ctx, e.s.d.Banco.L(), *tpl)
		if err != nil {
			e.saida("aviso", "Template não encontrado.", noID)
			return chatbot.Envio{Erro: "template"}
		}
		txt, err := acoes.Resolver(t.Texto, e.s.d.Acoes.Fonte(ctx, e.c), nil)
		if err != nil {
			e.saida("aviso", err.Error(), noID)
			return chatbot.Envio{Erro: err.Error()}
		}
		texto = txt
	}
	e.saida("mensagem", texto, noID)
	d := "Enviaria: " + texto
	e.c.Exec.RegistrarAcao(ctx, dominio.AcaoRegistrada{Tipo: "mensagem", Alvo: ptr("nó " + noID), Resultado: dominio.AcaoSimulada, Detalhe: &d})
	return chatbot.Envio{}
}

func (e *efeitos) Acao(ctx context.Context, noID string, a modelo.Acao, vars map[string]string) chatbot.Envio {
	e.c.Variaveis = vars
	r := e.s.d.Acoes.Executar(ctx, e.c, a)
	e.c.Exec.RegistrarAcao(ctx, r.Registro)
	txt := a.Tipo
	if r.Registro.Detalhe != nil {
		txt = *r.Registro.Detalhe
	}
	e.saida("acao", txt, noID)
	return chatbot.Envio{}
}

func (e *efeitos) IA(ctx context.Context, no modelo.No, vars map[string]string) (json.RawMessage, error) {
	e.c.Variaveis = vars
	if e.s.d.IA == nil {
		e.saida("aviso", "Automações de IA indisponíveis.", no.ID)
		return nil, erros.Novo(erros.Validacao, "Automações de IA indisponíveis.")
	}
	entrada, err := acoes.ResolverEntrada(no.Entrada, e.s.d.Acoes.Fonte(ctx, e.c))
	if err != nil {
		e.saida("aviso", err.Error(), no.ID)
		return nil, err
	}
	ret, err := e.s.d.IA.ExecutarPorAcao(ctx, no.AutomacaoID, entrada, e.c)
	if err != nil {
		e.saida("aviso", "IA falhou: "+err.Error(), no.ID)
	}
	return ret, err
}

func (e *efeitos) Condicao(ctx context.Context, cd modelo.Condicoes, vars map[string]string) bool {
	e.c.Variaveis = vars
	return condicoes.Avaliar(&cd, e.s.d.Acoes.DadosCondicoes(ctx, e.c))
}

func (e *efeitos) Resolver(ctx context.Context, texto string, vars map[string]string) (string, error) {
	e.c.Variaveis = vars
	return acoes.Resolver(texto, e.s.d.Acoes.Fonte(ctx, e.c), nil)
}

func (e *efeitos) Humano(ctx context.Context, noID string) {
	e.saida("aviso", "A conversa seria transferida para atendimento humano.", noID)
}
