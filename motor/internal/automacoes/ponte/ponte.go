// Pacote ponte atende as chamadas ctx.* do runner (specs/002-automacoes/contracts/
// runner-protocolo.md › Runner → motor): cada método exige a permissão declarada no manifesto,
// a simulação não escreve (registra "simulada"), a memória simulada vive só na execução, envios
// passam pelo portão e a IA é chamada pelo motor (a chave nunca vai ao runner).
package ponte

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/automacoes/acoes"
	"zapdesk/motor/internal/automacoes/execucoes"
	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/automacoes/runner"
	"zapdesk/motor/internal/automacoes/seguranca"
	"zapdesk/motor/internal/chat"
	"zapdesk/motor/internal/claude"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/fatos"
	"zapdesk/motor/internal/funil"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/leads"
	"zapdesk/motor/internal/organizacao"
	"zapdesk/motor/internal/relogio"
	"zapdesk/motor/internal/variaveis"
)

// Códigos de erro de runner-protocolo.md.
const (
	CodPermissao         = 1001
	CodValidacao         = 1002
	CodNaoEncontrado     = 1003
	CodBloqueado         = 1004
	CodIANaoConfigurada  = 1005
	CodIAErro            = 1006
	CodExecucaoEncerrada = 1007
	CodLimite            = 1008
	CodContaIndisponivel = 1009
)

// Limites.
const (
	MaxNotificacoes    = 5
	MaxValorMemoria    = 64 << 10
	MaxDadosAgendar    = 16 << 10
	MaxAgendarPorAut   = 1000
	MinAgendar         = 60 * time.Second
	MaxAgendar         = 30 * 24 * time.Hour
	LimiteHistorico    = 200
	HistoricoPadrao    = 30
	ConversaFicticiaID = "conversa-ficticia"
)

// Permissão exigida por método (vazio = nenhuma).
var permissaoDoMetodo = map[string]string{
	"ctx.conversa.historico": modelo.PermLerConversas, "ctx.enviar": modelo.PermEnviar, "ctx.reagir": modelo.PermEnviar,
	"ctx.etiquetas.listar": modelo.PermEtiquetas, "ctx.etiquetas.do_contato": modelo.PermEtiquetas,
	"ctx.etiquetas.adicionar": modelo.PermEtiquetas, "ctx.etiquetas.remover": modelo.PermEtiquetas,
	"ctx.funil.listar": modelo.PermFunil, "ctx.funil.posicao": modelo.PermFunil, "ctx.funil.mover": modelo.PermFunil,
	"ctx.funil.remover": modelo.PermFunil, "ctx.leads.atual": modelo.PermLeads, "ctx.leads.obter": modelo.PermLeads,
	"ctx.leads.buscar_telefone": modelo.PermLeads, "ctx.leads.atualizar": modelo.PermLeads,
	"ctx.memoria.obter": "", "ctx.memoria.definir": "", "ctx.memoria.remover": "", "ctx.memoria.listar": "",
	"ctx.ia.gerar": modelo.PermIA, "ctx.ia.classificar": modelo.PermIA, "ctx.ia.extrair": modelo.PermIA,
	"ctx.agendar": modelo.PermAgendar, "ctx.cancelar_agendamento": modelo.PermAgendar,
	"ctx.humano.transferir": modelo.PermEnviar, "ctx.notificar": "",
}

// Esperas grava/apaga as esperas de ctx.agendar.
type Esperas interface {
	Gravar(ctx context.Context, e dominio.Espera) error
}

// Deps da ponte.
type Deps struct {
	Banco        *armazenamento.Banco
	Barramento   *eventos.Barramento
	Relogio      relogio.Relogio
	Acoes        *acoes.Executores
	Chat         *chat.Servico
	Organizacao  *organizacao.Servico
	Leads        *leads.Servico
	Funil        *funil.Servico
	Portao       *seguranca.Portao
	Claude       claude.Cliente
	IASimulada   claude.Cliente // usada no teste com "IA simulada"
	ModeloPadrao func(ctx context.Context) string
	Esperas      Esperas
}

// Ponte cria sessões por execução.
type Ponte struct{ d Deps }

// Nova cria a ponte.
func Nova(d Deps) *Ponte { return &Ponte{d: d} }

// Sessao atende as chamadas ctx.* de UMA execução (implementa runner.Ponte).
type Sessao struct {
	p            *Ponte
	exec         *execucoes.Exec
	aut          dominio.Automacao
	perms        map[string]bool
	c            *acoes.Contexto
	iaSimulada   bool
	modelo       string
	mu           sync.Mutex
	memSim       map[string]json.RawMessage // "escopo\x00chave" → valor (nil = removido)
	notificacoes int
	encerrada    bool
}

// Opcoes de uma sessão.
type Opcoes struct {
	Exec            *execucoes.Exec
	Automacao       dominio.Automacao
	Permissoes      []string
	Contexto        *acoes.Contexto
	IASimulada      bool
	ModeloManifesto string
}

// Sessao cria a sessão de uma execução.
func (p *Ponte) Sessao(o Opcoes) *Sessao {
	perms := map[string]bool{}
	for _, x := range o.Permissoes {
		perms[x] = true
	}
	return &Sessao{p: p, exec: o.Exec, aut: o.Automacao, perms: perms, c: o.Contexto, iaSimulada: o.IASimulada,
		modelo: o.ModeloManifesto, memSim: map[string]json.RawMessage{}}
}

// Encerrar faz as chamadas seguintes falharem com execucao_encerrada.
func (s *Sessao) Encerrar() {
	s.mu.Lock()
	s.encerrada = true
	s.mu.Unlock()
}

func erroRPC(cod int, nome, msg string, extra map[string]any) *runner.ErroRPC {
	d := map[string]any{"codigo": nome}
	for k, v := range extra {
		d[k] = v
	}
	return &runner.ErroRPC{Codigo: cod, Mensagem: msg, Dados: d}
}

func validacao(msg string) *runner.ErroRPC { return erroRPC(CodValidacao, "validacao", msg, nil) }
func naoAchado(msg string) *runner.ErroRPC {
	return erroRPC(CodNaoEncontrado, "nao_encontrado", msg, nil)
}

// deErro converte erros de domínio/IA em erros RPC.
func deErro(err error) *runner.ErroRPC {
	var ce *claude.Erro
	if errors.As(err, &ce) {
		switch ce.Codigo {
		case claude.CodigoNaoConfigurada:
			return erroRPC(CodIANaoConfigurada, "ia_nao_configurada", ce.Mensagem, nil)
		case claude.CodigoValidacao:
			return validacao(ce.Mensagem)
		}
		var st, rid any
		if ce.Status != 0 {
			st = ce.Status
		}
		if ce.RequestID != "" {
			rid = ce.RequestID
		}
		return erroRPC(CodIAErro, "ia_erro", ce.Mensagem, map[string]any{"status": st, "request_id": rid})
	}
	if e, ok := erros.Como(err); ok {
		switch e.Codigo {
		case erros.NaoEncontrado:
			return naoAchado(e.Mensagem)
		case erros.ContaIndisponivel:
			return erroRPC(CodContaIndisponivel, "conta_indisponivel", e.Mensagem, nil)
		}
		campos := map[string]any{}
		if c, ok := e.Detalhes["campos"]; ok {
			campos["campos"] = c
		}
		return erroRPC(CodValidacao, "validacao", e.Mensagem, campos)
	}
	var ev *acoes.ErroVariavel
	if errors.As(err, &ev) {
		return validacao(ev.Error())
	}
	return &runner.ErroRPC{Codigo: -32603, Mensagem: "Erro interno do motor: " + err.Error()}
}

func ok(v any) (json.RawMessage, *runner.ErroRPC) {
	if v == nil {
		return json.RawMessage("{}"), nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, &runner.ErroRPC{Codigo: -32603, Mensagem: err.Error()}
	}
	return b, nil
}

// Chamar atende uma requisição ctx.*.
func (s *Sessao) Chamar(ctx context.Context, execucaoID, metodo string, params json.RawMessage) (json.RawMessage, *runner.ErroRPC) {
	s.mu.Lock()
	encerrada := s.encerrada
	s.mu.Unlock()
	if encerrada || (s.exec != nil && execucaoID != s.exec.ID()) {
		return nil, erroRPC(CodExecucaoEncerrada, "execucao_encerrada", "A execução já terminou.", nil)
	}
	perm, conhecido := permissaoDoMetodo[metodo]
	if !conhecido {
		return nil, &runner.ErroRPC{Codigo: -32601, Mensagem: "Método desconhecido: " + metodo}
	}
	if perm != "" && !s.perms[perm] {
		return nil, erroRPC(CodPermissao, "permissao_negada", fmt.Sprintf("Permissão '%s' não declarada em automacao.json", perm), map[string]any{"permissao": perm})
	}
	ctx = fatos.ComCadeia(ctx, s.c.Cadeia)
	switch metodo {
	case "ctx.conversa.historico":
		return s.historico(ctx, params)
	case "ctx.enviar":
		return s.enviar(ctx, params)
	case "ctx.reagir":
		return s.reagir(ctx, params)
	case "ctx.etiquetas.listar", "ctx.etiquetas.do_contato", "ctx.etiquetas.adicionar", "ctx.etiquetas.remover":
		return s.etiquetas(ctx, metodo, params)
	case "ctx.funil.listar", "ctx.funil.posicao", "ctx.funil.mover", "ctx.funil.remover":
		return s.funil(ctx, metodo, params)
	case "ctx.leads.atual", "ctx.leads.obter", "ctx.leads.buscar_telefone", "ctx.leads.atualizar":
		return s.leads(ctx, metodo, params)
	case "ctx.memoria.obter", "ctx.memoria.definir", "ctx.memoria.remover", "ctx.memoria.listar":
		return s.memoria(ctx, metodo, params)
	case "ctx.ia.gerar", "ctx.ia.classificar", "ctx.ia.extrair":
		return s.ia(ctx, metodo, params)
	case "ctx.agendar":
		return s.agendar(ctx, params)
	case "ctx.cancelar_agendamento":
		return s.cancelarAgendamento(ctx, params)
	case "ctx.humano.transferir":
		return s.humano(ctx, params)
	case "ctx.notificar":
		return s.notificar(ctx, params)
	}
	return nil, &runner.ErroRPC{Codigo: -32601, Mensagem: "Método desconhecido: " + metodo}
}

// Log registra ctx.log/console no log da execução (nunca no log do motor).
func (s *Sessao) Log(execucaoID *string, nivel, texto string, em time.Time) {
	if s.exec != nil {
		s.exec.Logar(nivel, texto)
	}
}

// HTTP registra cada ctx.http.fetch como AcaoRegistrada "http".
func (s *Sessao) HTTP(execucaoID string, metodo, urlSemQuery string, status *int, duracaoMs int64, erro *string) {
	if s.exec == nil {
		return
	}
	res := dominio.AcaoOK
	det := fmt.Sprintf("%s %s", metodo, urlSemQuery)
	if status != nil {
		det += fmt.Sprintf(" → %d", *status)
		if *status >= 400 {
			res = dominio.AcaoFalhou
		}
	}
	det += fmt.Sprintf(" (%d ms)", duracaoMs)
	if erro != nil {
		res = dominio.AcaoFalhou
		det += ": " + *erro
	}
	alvo := urlSemQuery
	s.exec.RegistrarAcao(context.Background(), dominio.AcaoRegistrada{Tipo: "http", Alvo: &alvo, Resultado: res, Detalhe: &det})
}

func (s *Sessao) registrar(ctx context.Context, tipo, resultado, alvo, detalhe string) {
	if s.exec == nil {
		return
	}
	a := dominio.AcaoRegistrada{Tipo: tipo, Resultado: resultado}
	if alvo != "" {
		a.Alvo = &alvo
	}
	if detalhe != "" {
		d := execucoes.Resumo(detalhe, 500)
		a.Detalhe = &d
	}
	s.exec.RegistrarAcao(ctx, a)
}

func decodificar(params json.RawMessage, v any) *runner.ErroRPC {
	if len(params) == 0 {
		return nil
	}
	if err := json.Unmarshal(params, v); err != nil {
		return &runner.ErroRPC{Codigo: -32602, Mensagem: "Parâmetros inválidos: " + err.Error()}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Alvos
// ---------------------------------------------------------------------------

type alvoParam struct {
	ContatoID  string `json:"contato_id"`
	LeadID     string `json:"lead_id"`
	Telefone   string `json:"telefone"`
	ConversaID string `json:"conversa_id"`
}

// resolverAlvo devolve um Contexto de ações para o alvo (ausente = o da execução).
func (s *Sessao) resolverAlvo(ctx context.Context, a *alvoParam) (*acoes.Contexto, *runner.ErroRPC) {
	c := *s.c
	c.Cadeia = s.c.Cadeia
	if a == nil || (a.ContatoID == "" && a.LeadID == "" && a.Telefone == "" && a.ConversaID == "") {
		if c.Alvo.ContatoID == "" && c.Alvo.LeadID == "" && c.Alvo.ConversaID == "" && !c.ConversaFicticia {
			return nil, validacao("Esta execução não tem contato.")
		}
		s.p.d.Acoes.CompletarAlvo(ctx, &c)
		return &c, nil
	}
	c.Alvo = acoes.Alvo{}
	c.ConversaFicticia = false
	l := s.p.d.Banco.L()
	switch {
	case a.ConversaID != "":
		cv, err := armazenamento.ObterConversa(ctx, l, a.ConversaID)
		if err != nil {
			return nil, naoAchado("Conversa não encontrada.")
		}
		c.Alvo.ConversaID, c.Alvo.ContaID = cv.ID, cv.ContaID
	case a.ContatoID != "":
		if _, err := armazenamento.ObterContato(ctx, l, a.ContatoID); err != nil {
			return nil, naoAchado("Contato não encontrado.")
		}
		c.Alvo.ContatoID = a.ContatoID
	case a.LeadID != "":
		if _, err := armazenamento.ObterLead(ctx, l, a.LeadID); err != nil {
			return nil, naoAchado("Lead não encontrado.")
		}
		c.Alvo.LeadID = a.LeadID
	default:
		ld, err := armazenamento.LeadPorTelefone(ctx, l, normalizarTel(a.Telefone))
		if err == nil {
			c.Alvo.LeadID = ld.ID
		} else {
			var ct string
			if l.QueryRowContext(ctx, `SELECT id FROM contatos WHERE telefone = ? ORDER BY atualizado_em DESC LIMIT 1`, normalizarTel(a.Telefone)).Scan(&ct) != nil {
				return nil, naoAchado("Nenhum contato ou lead com esse telefone.")
			}
			c.Alvo.ContatoID = ct
		}
	}
	s.p.d.Acoes.CompletarAlvo(ctx, &c)
	return &c, nil
}

// ---------------------------------------------------------------------------
// Conversa e envio
// ---------------------------------------------------------------------------

type mensagemProt struct {
	ID            string  `json:"id"`
	ConversaID    string  `json:"conversa_id"`
	DeMim         bool    `json:"de_mim"`
	AutomacaoID   *string `json:"automacao_id"`
	Tipo          string  `json:"tipo"`
	Texto         *string `json:"texto"`
	RemetenteNome *string `json:"remetente_nome"`
	EnviadaEm     string  `json:"enviada_em"`
}

func (s *Sessao) historico(ctx context.Context, params json.RawMessage) (json.RawMessage, *runner.ErroRPC) {
	var p struct {
		ConversaID string `json:"conversa_id"`
		Limite     int    `json:"limite"`
		Antes      string `json:"antes"`
	}
	if e := decodificar(params, &p); e != nil {
		return nil, e
	}
	conv := p.ConversaID
	if conv == "" || conv == ConversaFicticiaID {
		if s.c.ConversaFicticia || conv == ConversaFicticiaID {
			// A conversa fictícia do teste só tem a mensagem que disparou o teste.
			l := []mensagemProt{}
			if s.c.UltimaMensagem != nil {
				nome := "Contato de teste"
				l = append(l, mensagemProt{ID: "mensagem-teste", ConversaID: ConversaFicticiaID, Tipo: "texto",
					Texto: s.c.UltimaMensagem, RemetenteNome: &nome, EnviadaEm: s.p.d.Relogio.Agora().Format(time.RFC3339)})
			}
			return ok(l)
		}
		conv = s.c.Alvo.ConversaID
	}
	if conv == "" {
		return nil, validacao("Esta execução não tem conversa.")
	}
	limite := p.Limite
	if limite == 0 {
		limite = HistoricoPadrao
	}
	if limite < 1 || limite > LimiteHistorico {
		return nil, validacao("O limite do histórico deve ficar entre 1 e 200.")
	}
	lista, _, err := armazenamento.ListarMensagens(ctx, s.p.d.Banco.L(), conv, p.Antes, limite)
	if err != nil {
		return nil, deErro(err)
	}
	r := make([]mensagemProt, 0, len(lista))
	for _, m := range lista {
		r = append(r, mensagemProt{ID: m.ID, ConversaID: m.ConversaID, DeMim: m.DeMim, AutomacaoID: m.AutomacaoID, Tipo: m.Tipo,
			Texto: m.Texto, RemetenteNome: m.RemetenteNome, EnviadaEm: m.EnviadaEm.Format(time.RFC3339)})
	}
	return ok(r)
}

type mensagemEnviada struct {
	ID         *string `json:"id"`
	ConversaID *string `json:"conversa_id"`
	Simulada   bool    `json:"simulada"`
	Texto      *string `json:"texto"`
}

func (s *Sessao) enviar(ctx context.Context, params json.RawMessage) (json.RawMessage, *runner.ErroRPC) {
	var p struct {
		Destino struct {
			ConversaID string `json:"conversa_id"`
			Telefone   string `json:"telefone"`
			ContaID    string `json:"conta_id"`
		} `json:"destino"`
		Conteudo struct {
			Texto     *string           `json:"texto"`
			Template  string            `json:"template"`
			Variaveis map[string]string `json:"variaveis"`
			ArquivoID string            `json:"arquivo_id"`
			Legenda   string            `json:"legenda"`
			Como      string            `json:"como"`
		} `json:"conteudo"`
		CitarMensagemID string `json:"citar_mensagem_id"`
	}
	if e := decodificar(params, &p); e != nil {
		return nil, e
	}
	// Conteúdo.
	texto, arquivo := "", ""
	switch {
	case p.Conteudo.Texto != nil:
		texto = *p.Conteudo.Texto
		if strings.TrimSpace(texto) == "" {
			return nil, validacao("Texto vazio.")
		}
	case p.Conteudo.Template != "":
		t, err := s.template(ctx, p.Conteudo.Template)
		if err != nil {
			return nil, naoAchado(fmt.Sprintf("Template '%s' não encontrado.", p.Conteudo.Template))
		}
		fonte := s.p.d.Acoes.Fonte(ctx, s.c)
		for k, v := range p.Conteudo.Variaveis {
			if fonte.Execucao == nil {
				fonte.Execucao = map[string]string{}
			}
			fonte.Execucao[variaveis.Extrair("{" + k + "}")[0]] = v
		}
		txt, err := acoes.Resolver(t.Texto, fonte, nil)
		if err != nil {
			return nil, deErro(err)
		}
		texto = txt
		if t.ArquivoID != nil {
			arquivo = *t.ArquivoID
		}
	case p.Conteudo.ArquivoID != "":
		arquivo, texto = p.Conteudo.ArquivoID, p.Conteudo.Legenda
	default:
		return nil, validacao("Informe o conteúdo: texto, template ou arquivo.")
	}
	if len([]rune(texto)) > modelo.MaxTextoMensagem {
		return nil, validacao("A mensagem pode ter até 4.096 caracteres.")
	}
	// Destino.
	conv := p.Destino.ConversaID
	alvoTxt := conv
	if p.Destino.Telefone != "" {
		alvoTxt = p.Destino.Telefone
	} else if conv != "" && conv != ConversaFicticiaID {
		// O registro da ação mostra o telefone (como nos fluxos), não o id interno da conversa.
		if c, err := armazenamento.ObterConversa(ctx, s.p.d.Banco.L(), conv); err == nil && c.Telefone != nil {
			alvoTxt = *c.Telefone
		}
	}
	if s.c.Simulacao {
		s.registrar(ctx, "enviar", dominio.AcaoSimulada, alvoTxt, "Enviaria: "+texto)
		var cv *string
		if conv != "" {
			cv = &conv
		}
		return ok(mensagemEnviada{ConversaID: cv, Simulada: true, Texto: &texto})
	}
	if conv == ConversaFicticiaID {
		return nil, validacao("A conversa fictícia só existe em teste.")
	}
	if conv == "" {
		if p.Destino.Telefone == "" {
			return nil, validacao("Informe o destino: conversa_id ou telefone.")
		}
		conta := p.Destino.ContaID
		if conta == "" && s.aut.ContaEnvioID != nil {
			conta = *s.aut.ContaEnvioID
		}
		if conta == "" {
			conta = s.c.Alvo.ContaID
		}
		if conta == "" {
			return nil, validacao("Informe a conta (destino.contaId) ou configure conta_envio no automacao.json.")
		}
		cv, _, err := s.p.d.Chat.NovaConversa(ctx, conta, p.Destino.Telefone)
		if err != nil {
			return nil, deErro(err)
		}
		conv = cv.ID
	}
	var m dominio.Mensagem
	d, err := s.p.d.Portao.AutorizarEEnviar(ctx, seguranca.Pedido{ConversaID: conv, AutomacaoID: s.aut.ID, AutomacaoNome: s.aut.Nome,
		IncluirGrupos: s.aut.IncluirGrupos, AntiLoop: s.aut.Limites.AntiLoop}, func(d seguranca.Decisao) error {
		env := chat.Envio{Texto: &texto, Como: p.Conteudo.Como, Automatico: &chat.EnvioAutomatico{AutomacaoID: s.aut.ID, PrimeiroContato: d.PrimeiroContato}}
		if arquivo != "" {
			env.ArquivoID = &arquivo
		}
		if p.CitarMensagemID != "" {
			env.CitarMensagemID = &p.CitarMensagemID
		}
		var err error
		m, err = s.p.d.Chat.Enviar(ctx, conv, env)
		return err
	})
	if !d.Permitido {
		s.registrar(ctx, "enviar", dominio.AcaoBloqueada, alvoTxt, d.Mensagem)
		return nil, erroRPC(CodBloqueado, "bloqueado", d.Mensagem, map[string]any{"motivo": d.Motivo})
	}
	if err != nil {
		s.registrar(ctx, "enviar", dominio.AcaoFalhou, alvoTxt, err.Error())
		return nil, deErro(err)
	}
	s.registrar(ctx, "enviar", dominio.AcaoOK, alvoTxt, texto)
	id := m.ID
	return ok(mensagemEnviada{ID: &id, ConversaID: &conv, Texto: m.Texto})
}

func (s *Sessao) template(ctx context.Context, nomeOuID string) (dominio.Template, error) {
	if t, err := armazenamento.ObterTemplate(ctx, s.p.d.Banco.L(), nomeOuID); err == nil {
		return t, nil
	}
	lista, err := armazenamento.ListarTemplates(ctx, s.p.d.Banco.L(), "")
	if err != nil {
		return dominio.Template{}, err
	}
	for _, t := range lista {
		if modelo.NormalizarTexto(t.Nome) == modelo.NormalizarTexto(nomeOuID) {
			return t, nil
		}
	}
	return dominio.Template{}, erros.NaoAchado("Template")
}

func (s *Sessao) reagir(ctx context.Context, params json.RawMessage) (json.RawMessage, *runner.ErroRPC) {
	var p struct {
		MensagemID string `json:"mensagem_id"`
		Emoji      string `json:"emoji"`
	}
	if e := decodificar(params, &p); e != nil {
		return nil, e
	}
	if s.c.Simulacao {
		s.registrar(ctx, "reagir", dominio.AcaoSimulada, p.MensagemID, "Reagiria com "+p.Emoji)
		return ok(nil)
	}
	if err := s.p.d.Chat.Reagir(ctx, p.MensagemID, p.Emoji); err != nil {
		return nil, deErro(err)
	}
	s.registrar(ctx, "reagir", dominio.AcaoOK, p.MensagemID, p.Emoji)
	return ok(nil)
}

func (s *Sessao) humano(ctx context.Context, params json.RawMessage) (json.RawMessage, *runner.ErroRPC) {
	var p struct {
		Motivo     string `json:"motivo"`
		Mensagem   string `json:"mensagem"`
		DuracaoMin *int   `json:"duracao_min"`
	}
	if e := decodificar(params, &p); e != nil {
		return nil, e
	}
	conv := s.c.Alvo.ConversaID
	if conv == "" && !s.c.ConversaFicticia {
		return nil, validacao("Esta execução não tem conversa.")
	}
	if p.DuracaoMin != nil && (*p.DuracaoMin < 1 || *p.DuracaoMin > 10080) {
		return nil, validacao("A duração deve ficar entre 1 e 10.080 minutos.")
	}
	if s.c.Simulacao {
		det := "Transferiria a conversa para atendimento humano."
		if p.Mensagem != "" {
			det += " Mensagem: " + p.Mensagem
		}
		s.registrar(ctx, "humano", dominio.AcaoSimulada, "", det)
		return ok(nil)
	}
	if p.Mensagem != "" {
		txt := p.Mensagem
		body, _ := json.Marshal(map[string]any{"destino": map[string]string{"conversa_id": conv}, "conteudo": map[string]string{"texto": txt}})
		if _, e := s.enviar(ctx, body); e != nil && e.Codigo != CodBloqueado {
			return nil, e
		}
	}
	s.p.d.Portao.Pausas().Assumir(ctx, conv, dominio.PausaHumano, p.DuracaoMin)
	corpo := "Uma automação passou esta conversa para você."
	if p.Motivo != "" {
		corpo = execucoes.Resumo(p.Motivo, 240)
	}
	aut, cv := s.aut.ID, conv
	s.p.d.Barramento.Publicar(eventos.Notificacao, s.c.Alvo.ContaID, dominio.Notificacao{Titulo: "Atendimento humano", Corpo: corpo,
		AutomacaoID: &aut, ConversaID: &cv, Tipo: "humano"})
	s.registrar(ctx, "humano", dominio.AcaoOK, "", corpo)
	return ok(nil)
}

func (s *Sessao) notificar(ctx context.Context, params json.RawMessage) (json.RawMessage, *runner.ErroRPC) {
	var p struct {
		Titulo string `json:"titulo"`
		Texto  string `json:"texto"`
	}
	if e := decodificar(params, &p); e != nil {
		return nil, e
	}
	if strings.TrimSpace(p.Titulo) == "" || strings.TrimSpace(p.Texto) == "" {
		return nil, validacao("Informe título e texto.")
	}
	s.mu.Lock()
	if s.notificacoes >= MaxNotificacoes {
		s.mu.Unlock()
		return nil, erroRPC(CodLimite, "limite", "Máximo de 5 notificações por execução.", nil)
	}
	s.notificacoes++
	s.mu.Unlock()
	titulo, texto := execucoes.Resumo(p.Titulo, modelo.MaxTituloNotificar), execucoes.Resumo(p.Texto, modelo.MaxTextoNotificar)
	if s.c.Simulacao {
		s.registrar(ctx, "notificar", dominio.AcaoSimulada, titulo, "Notificaria: "+texto)
		return ok(nil)
	}
	n := dominio.Notificacao{Titulo: titulo, Corpo: texto, Tipo: "acao"}
	aut := s.aut.ID
	n.AutomacaoID = &aut
	if s.c.Alvo.ConversaID != "" {
		cv := s.c.Alvo.ConversaID
		n.ConversaID = &cv
	}
	s.p.d.Barramento.Publicar(eventos.Notificacao, s.c.Alvo.ContaID, n)
	s.registrar(ctx, "notificar", dominio.AcaoOK, titulo, texto)
	return ok(nil)
}

// ---------------------------------------------------------------------------
// Agendamento
// ---------------------------------------------------------------------------

func (s *Sessao) agendar(ctx context.Context, params json.RawMessage) (json.RawMessage, *runner.ErroRPC) {
	var p struct {
		Em         string          `json:"em"`
		Dados      json.RawMessage `json:"dados"`
		NaConversa bool            `json:"na_conversa"`
	}
	if e := decodificar(params, &p); e != nil {
		return nil, e
	}
	em, err := time.Parse(time.RFC3339Nano, p.Em)
	if err != nil {
		return nil, validacao("Data inválida em 'em' (use ISO 8601).")
	}
	agora := s.p.d.Relogio.Agora()
	if d := em.Sub(agora); d < MinAgendar-time.Second || d > MaxAgendar {
		return nil, validacao("Agende de 60 segundos a 30 dias a partir de agora.")
	}
	if len(p.Dados) > MaxDadosAgendar {
		return nil, erroRPC(CodLimite, "limite", "Dados do agendamento maiores que 16 KB.", nil)
	}
	emTxt := em.Local().Format(time.RFC3339)
	if s.c.Simulacao {
		id := "simulado-" + ids.NovoEm(agora)
		s.registrar(ctx, "agendar", dominio.AcaoSimulada, emTxt, "Agendaria aoAgendar para "+emTxt)
		return ok(map[string]string{"id": id, "em": emTxt})
	}
	n, err := armazenamento.ContarEsperasAgendar(ctx, s.p.d.Banco.L(), s.aut.ID)
	if err != nil {
		return nil, deErro(err)
	}
	if n >= MaxAgendarPorAut {
		return nil, erroRPC(CodLimite, "limite", "Limite de 1.000 agendamentos pendentes por automação.", nil)
	}
	dados, _ := json.Marshal(map[string]json.RawMessage{"dados": nulo(p.Dados)})
	e := dominio.Espera{ID: ids.NovoEm(agora), Tipo: dominio.EsperaAgendar, AutomacaoID: s.aut.ID, RetomarEm: em, Dados: dados, CriadaEm: agora}
	if p.NaConversa && s.c.Alvo.ConversaID != "" {
		cv := s.c.Alvo.ConversaID
		e.ConversaID = &cv
	}
	if err := s.p.d.Esperas.Gravar(ctx, e); err != nil {
		return nil, deErro(err)
	}
	s.registrar(ctx, "agendar", dominio.AcaoOK, emTxt, "")
	return ok(map[string]string{"id": e.ID, "em": emTxt})
}

func nulo(r json.RawMessage) json.RawMessage {
	if len(r) == 0 {
		return json.RawMessage("null")
	}
	return r
}

func (s *Sessao) cancelarAgendamento(ctx context.Context, params json.RawMessage) (json.RawMessage, *runner.ErroRPC) {
	var p struct {
		ID string `json:"id"`
	}
	if e := decodificar(params, &p); e != nil {
		return nil, e
	}
	if s.c.Simulacao {
		s.registrar(ctx, "cancelar_agendamento", dominio.AcaoSimulada, p.ID, "")
		return ok(map[string]bool{"cancelado": strings.HasPrefix(p.ID, "simulado-")})
	}
	r, err := s.p.d.Banco.E().ExecContext(ctx, `DELETE FROM esperas WHERE id = ? AND automacao_id = ? AND tipo = 'agendar'`, p.ID, s.aut.ID)
	if err != nil {
		return nil, deErro(err)
	}
	n, _ := r.RowsAffected()
	return ok(map[string]bool{"cancelado": n == 1})
}
