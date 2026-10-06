// Pacote automacoes é a fachada do motor de automações usada pela API: CRUD, validação,
// ativação, execução manual e teste, configuração, pausas, execuções e segredos
// (specs/002-automacoes/contracts/api-http.md). Os subpacotes fazem o trabalho.
package automacoes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rs/zerolog"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/automacoes/acoes"
	"zapdesk/motor/internal/automacoes/despacho"
	"zapdesk/motor/internal/automacoes/esperas"
	"zapdesk/motor/internal/automacoes/execucoes"
	"zapdesk/motor/internal/automacoes/fluxo"
	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/automacoes/seguranca"
	"zapdesk/motor/internal/chat"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/pagina"
	"zapdesk/motor/internal/relogio"
	"zapdesk/motor/internal/segredos"
)

// Ganchos de outras histórias (chatbot, IA) chamados pela fachada.
type Ganchos struct {
	// AoDesativar/AoExcluir: sessões de chatbot → abortada; runner encerrado; pasta apagada.
	AoDesativar []func(ctx context.Context, a dominio.Automacao)
	AoExcluir   []func(ctx context.Context, a dominio.Automacao)
}

// Deps da fachada.
type Deps struct {
	Banco      *armazenamento.Banco
	Barramento *eventos.Barramento
	Relogio    relogio.Relogio
	Log        zerolog.Logger
	Config     *Config
	Registro   *execucoes.Registro
	Portao     *seguranca.Portao
	Acoes      *acoes.Executores
	Fluxo      *fluxo.Executor
	Despacho   *despacho.Despachante
	Agendador  *esperas.Agendador
	Cofre      *segredos.Cofre
	Chat       *chat.Servico
}

// Servico é a fachada.
type Servico struct {
	d       Deps
	ganchos Ganchos
	ia      ServicoIA
	testes  map[string]Testador
}

// ServicoIA é a parte das automações de IA (US3), ligada depois.
type ServicoIA interface {
	// PreencherIA completa Automacao.IA (rodando_versao_anterior etc.).
	PreencherIA(ctx context.Context, a *dominio.Automacao)
	// Ativavel confere a compilação antes de ativar (compilacao_falhou).
	Ativavel(ctx context.Context, a dominio.Automacao) error
	// RunnerDisponivel indica se há runner configurado.
	RunnerDisponivel() bool
	// ProcessosVivos para GET /sistema.
	ProcessosVivos() int
}

// Testador testa uma automação de um tipo (fluxo aqui; IA na US3).
type Testador interface {
	Testar(ctx context.Context, a dominio.Automacao, p PedidoTeste) (dominio.ExecucaoDetalhe, error)
}

// Novo cria a fachada.
func Novo(d Deps) *Servico {
	s := &Servico{d: d, testes: map[string]Testador{}}
	s.testes[modelo.TipoFluxo] = testadorFluxo{s}
	d.Registro.AoDesativar(func(ctx context.Context, id string) {
		if a, err := s.Obter(ctx, id); err == nil {
			s.limparAoDesativar(ctx, a)
			s.publicar(a)
		}
	})
	return s
}

// DefinirIA liga o serviço de IA.
func (s *Servico) DefinirIA(ia ServicoIA, t Testador) {
	s.ia = ia
	s.testes[modelo.TipoIA] = t
}

// AdicionarGanchos acrescenta ganchos de desativação/exclusão.
func (s *Servico) AdicionarGanchos(g Ganchos) {
	s.ganchos.AoDesativar = append(s.ganchos.AoDesativar, g.AoDesativar...)
	s.ganchos.AoExcluir = append(s.ganchos.AoExcluir, g.AoExcluir...)
}

// Config devolve a configuração.
func (s *Servico) Config() *Config { return s.d.Config }

// Portao devolve o portão (pausas).
func (s *Servico) Portao() *seguranca.Portao { return s.d.Portao }

// Deps expostas para os serviços das outras histórias.
func (s *Servico) Deps() Deps { return s.d }

// ---------------------------------------------------------------------------
// Leitura
// ---------------------------------------------------------------------------

func (s *Servico) completar(ctx context.Context, lista []dominio.Automacao) {
	est, _ := armazenamento.EstatisticasAutomacoes(ctx, s.d.Banco.L(), s.d.Relogio.Agora().Add(-24*time.Hour))
	sess, _ := armazenamento.SessoesAtivasPorAutomacao(ctx, s.d.Banco.L())
	for i := range lista {
		a := &lista[i]
		a.Estatisticas24h = est[a.ID]
		a.SessoesAtivas = sess[a.ID]
		if a.Contas != nil && len(a.Contas) == 0 {
			a.Contas = []string{}
		}
		a.Avisos = s.avisos(ctx, *a)
		if a.Tipo == modelo.TipoIA {
			a.Definicao = json.RawMessage("null")
			if s.ia != nil {
				s.ia.PreencherIA(ctx, a)
			} else {
				a.IA = iaBasica(*a, "")
			}
		}
	}
}

// iaBasica monta Automacao.IA a partir das colunas.
func iaBasica(a dominio.Automacao, pasta string) *dominio.AutomacaoIA {
	perm, seg := a.Permissoes, a.Segredos
	if perm == nil {
		perm = []string{}
	}
	if seg == nil {
		seg = []string{}
	}
	ec := a.ErrosCompilacao
	if ec == nil {
		ec = []dominio.ErroCompilacao{}
	}
	return &dominio.AutomacaoIA{Pasta: pasta, Permissoes: perm, Segredos: seg, HashCompilado: a.HashCompilado,
		CompilacaoOK: a.HashCompilado != nil && len(ec) == 0, ErrosCompilacao: ec, CompiladoEm: a.CompiladoEm}
}

// Obter devolve a automação completa.
func (s *Servico) Obter(ctx context.Context, id string) (dominio.Automacao, error) {
	a, err := armazenamento.ObterAutomacao(ctx, s.d.Banco.L(), id)
	if err != nil {
		return a, err
	}
	l := []dominio.Automacao{a}
	s.completar(ctx, l)
	return l[0], nil
}

// Listar com filtros.
func (s *Servico) Listar(ctx context.Context, f armazenamento.FiltroAutomacoes) ([]dominio.Automacao, error) {
	if f.Tipo != "" && f.Tipo != modelo.TipoFluxo && f.Tipo != modelo.TipoChatbot && f.Tipo != modelo.TipoIA {
		return nil, erros.Campo("tipo", "Use fluxo, chatbot ou ia.")
	}
	l, err := armazenamento.ListarAutomacoes(ctx, s.d.Banco.L(), f)
	if err != nil {
		return nil, err
	}
	s.completar(ctx, l)
	return l, nil
}

func (s *Servico) publicar(a dominio.Automacao) {
	s.d.Barramento.Publicar(eventos.AutomacaoAtualizada, "", a)
}

// PublicarAtualizada relê e publica automacao.atualizada.
func (s *Servico) PublicarAtualizada(ctx context.Context, id string) {
	if a, err := s.Obter(ctx, id); err == nil {
		s.publicar(a)
	}
}

// ---------------------------------------------------------------------------
// Criação, validação e edição (fluxo e chatbot)
// ---------------------------------------------------------------------------

// Entrada é o corpo de NovaAutomacao/PATCH com a presença de cada campo.
type Entrada struct {
	campos map[string]json.RawMessage
}

// LerEntrada decodifica o JSON do corpo.
func LerEntrada(corpo []byte) (Entrada, error) {
	e := Entrada{campos: map[string]json.RawMessage{}}
	if len(bytes.TrimSpace(corpo)) == 0 {
		return e, nil
	}
	if err := json.Unmarshal(corpo, &e.campos); err != nil {
		return e, erros.Novo(erros.Validacao, "JSON inválido: "+err.Error())
	}
	return e, nil
}

func (e Entrada) tem(k string) bool { _, ok := e.campos[k]; return ok }

func (e Entrada) str(k string) (*string, error) {
	b, ok := e.campos[k]
	if !ok || string(b) == "null" {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, erros.Campo(k, "Use um texto.")
	}
	return &s, nil
}

// aplicar copia os campos presentes para a automação; devolve erros de validação de campo e
// erros de definição.
func (s *Servico) aplicar(ctx context.Context, a *dominio.Automacao, e Entrada, novo bool) (map[string]string, []dominio.ErroDefinicao, bool) {
	campos := map[string]string{}
	var defs []dominio.ErroDefinicao
	mudouDef := false
	if novo || e.tem("nome") {
		nome, err := e.str("nome")
		if err != nil || nome == nil || strings.TrimSpace(*nome) == "" || utf8.RuneCountInString(strings.TrimSpace(*nome)) > 80 {
			campos["nome"] = "O nome deve ter de 1 a 80 caracteres."
		} else {
			a.Nome = strings.TrimSpace(*nome)
		}
	}
	if e.tem("descricao") {
		d, err := e.str("descricao")
		if err != nil || (d != nil && utf8.RuneCountInString(*d) > 500) {
			campos["descricao"] = "A descrição pode ter até 500 caracteres."
		} else if d != nil && strings.TrimSpace(*d) != "" {
			v := strings.TrimSpace(*d)
			a.Descricao = &v
		} else {
			a.Descricao = nil
		}
	}
	if e.tem("contas") {
		var l []string
		if string(e.campos["contas"]) == "null" {
			a.Contas = nil
		} else if err := json.Unmarshal(e.campos["contas"], &l); err != nil {
			campos["contas"] = "Use uma lista de contas ou null (todas)."
		} else {
			if l == nil {
				l = []string{}
			}
			a.Contas = l
		}
	}
	if e.tem("incluir_grupos") {
		if err := json.Unmarshal(e.campos["incluir_grupos"], &a.IncluirGrupos); err != nil {
			campos["incluir_grupos"] = "Use verdadeiro ou falso."
		}
	}
	if e.tem("prioridade") {
		var p int
		if err := json.Unmarshal(e.campos["prioridade"], &p); err != nil || p < 1 || p > 1000 {
			campos["prioridade"] = "A prioridade deve ficar entre 1 e 1.000."
		} else {
			a.Prioridade = p
		}
	}
	if e.tem("conta_envio_id") {
		v, err := e.str("conta_envio_id")
		if err != nil {
			campos["conta_envio_id"] = "Use o id de uma conta ou null."
		} else if v != nil && *v != "" {
			a.ContaEnvioID = v
		} else {
			a.ContaEnvioID = nil
		}
	}
	if novo || e.tem("gatilhos") {
		gs, errs := modelo.DecodificarGatilhos(e.campos["gatilhos"])
		if errs != nil {
			defs = append(defs, errs...)
		} else {
			b, _ := json.Marshal(gs)
			if !bytes.Equal(b, a.Gatilhos) {
				mudouDef = true
			}
			a.Gatilhos = b
		}
	}
	if novo || e.tem("definicao") {
		b, errs := normalizarDefinicao(a.Tipo, e.campos["definicao"])
		if errs != nil {
			defs = append(defs, errs...)
		} else {
			if !bytes.Equal(b, a.Definicao) {
				mudouDef = true
			}
			a.Definicao = b
		}
	}
	if e.tem("limites") {
		var l dominio.Limites
		if string(e.campos["limites"]) != "null" {
			if err := json.Unmarshal(e.campos["limites"], &l); err != nil {
				defs = append(defs, dominio.ErroDefinicao{Caminho: "limites", Mensagem: "Limites inválidos."})
			}
		}
		b1, _ := json.Marshal(l)
		b0, _ := json.Marshal(a.Limites)
		if !bytes.Equal(b0, b1) {
			mudouDef = true
		}
		a.Limites = l
	}
	return campos, defs, mudouDef
}

func normalizarDefinicao(tipo string, bruto json.RawMessage) (json.RawMessage, []dominio.ErroDefinicao) {
	switch tipo {
	case modelo.TipoFluxo:
		d, errs := modelo.DecodificarFluxo(bruto)
		if errs != nil {
			return nil, errs
		}
		modelo.NormalizarFluxo(d)
		b, _ := json.Marshal(d)
		return b, nil
	case modelo.TipoChatbot:
		d, errs := modelo.DecodificarChatbot(bruto)
		if errs != nil {
			return nil, errs
		}
		modelo.NormalizarChatbot(d)
		b, _ := json.Marshal(d)
		return b, nil
	}
	return nil, []dominio.ErroDefinicao{{Caminho: "tipo", Mensagem: "Tipo inválido."}}
}

// validarEstrutura aplica a validação estrutural completa (gatilhos, definição, limites).
func (s *Servico) validarEstrutura(a dominio.Automacao) []dominio.ErroDefinicao {
	var errs []dominio.ErroDefinicao
	gs, e1 := modelo.DecodificarGatilhos(a.Gatilhos)
	errs = append(errs, e1...)
	if e1 == nil {
		errs = append(errs, modelo.ValidarGatilhos(gs, modelo.OpcoesGatilhos{IncluirGrupos: a.IncluirGrupos})...)
		for i, g := range gs {
			if g.Tipo == modelo.GatilhoManual && a.Tipo == modelo.TipoChatbot {
				errs = append(errs, dominio.ErroDefinicao{Caminho: fmt.Sprintf("gatilhos[%d].tipo", i), Mensagem: "Chatbots começam por mensagem, palavra-chave ou pela ação \"iniciar chatbot\"."})
			}
		}
	}
	switch a.Tipo {
	case modelo.TipoFluxo:
		d, e := modelo.DecodificarFluxo(a.Definicao)
		errs = append(errs, e...)
		if e == nil {
			errs = append(errs, modelo.ValidarFluxo(d)...)
		}
	case modelo.TipoChatbot:
		d, e := modelo.DecodificarChatbot(a.Definicao)
		errs = append(errs, e...)
		if e == nil {
			errs = append(errs, modelo.ValidarChatbot(d)...)
		}
	}
	errs = append(errs, modelo.ValidarLimites(a.Limites)...)
	if al := a.Limites.AntiLoop; al != nil {
		if g := s.d.Config.Configuracao(); al.Mensagens > g.AntiLoopMensagens {
			errs = append(errs, dominio.ErroDefinicao{Caminho: "limites.anti_loop.mensagens",
				Mensagem: fmt.Sprintf("O limite próprio só pode ser mais restritivo que o global (%d mensagens).", g.AntiLoopMensagens)})
		}
	}
	if errs == nil {
		errs = []dominio.ErroDefinicao{}
	}
	return errs
}

func erroDefinicao(errs []dominio.ErroDefinicao) error {
	msg := "A automação tem erros. Confira os campos destacados."
	if len(errs) == 1 {
		msg = errs[0].Mensagem
	}
	return erros.ComDetalhes(erros.DefinicaoInvalida, msg, map[string]any{"erros": errs})
}

func (s *Servico) novaDe(e Entrada) (dominio.Automacao, map[string]string, []dominio.ErroDefinicao) {
	agora := s.d.Relogio.Agora()
	a := dominio.Automacao{ID: ids.NovoEm(agora), Prioridade: 100, Versao: 1, CriadaEm: agora, AtualizadaEm: agora}
	tipo, _ := e.str("tipo")
	campos := map[string]string{}
	if tipo == nil || (*tipo != modelo.TipoFluxo && *tipo != modelo.TipoChatbot) {
		campos["tipo"] = "Use fluxo ou chatbot (automações de IA: POST /automacoes/ia)."
		return a, campos, nil
	}
	a.Tipo = *tipo
	c, defs, _ := s.aplicar(context.Background(), &a, e, true)
	for k, v := range c {
		campos[k] = v
	}
	if len(defs) == 0 {
		defs = s.validarEstrutura(a)
	}
	return a, campos, defs
}

// Validar valida uma NovaAutomacao sem gravar (erros e avisos).
func (s *Servico) Validar(ctx context.Context, e Entrada) (erros_, avisos []dominio.ErroDefinicao, err error) {
	a, campos, defs := s.novaDe(e)
	for k, v := range campos {
		defs = append(defs, dominio.ErroDefinicao{Caminho: k, Mensagem: v})
	}
	if defs == nil {
		defs = []dominio.ErroDefinicao{}
	}
	av := []dominio.ErroDefinicao{}
	if len(campos) == 0 {
		av = s.avisos(ctx, a)
	}
	return defs, av, nil
}

// Criar grava um fluxo/chatbot (inativo).
func (s *Servico) Criar(ctx context.Context, e Entrada) (dominio.Automacao, error) {
	a, campos, defs := s.novaDe(e)
	if len(campos) > 0 {
		return a, erros.Campos(campos)
	}
	if len(defs) > 0 {
		return a, erroDefinicao(defs)
	}
	if err := armazenamento.InserirAutomacao(ctx, s.d.Banco.E(), a); err != nil {
		return a, err
	}
	a, err := s.Obter(ctx, a.ID)
	if err == nil {
		s.publicar(a)
	}
	return a, err
}

// Editar aplica um PATCH (versão +1 se gatilhos/definição/limites mudarem). Automação ativa
// continua ativa se a definição continuar válida (senão definicao_invalida).
func (s *Servico) Editar(ctx context.Context, id string, e Entrada) (dominio.Automacao, error) {
	a, err := armazenamento.ObterAutomacao(ctx, s.d.Banco.L(), id)
	if err != nil {
		return a, err
	}
	if a.Tipo == modelo.TipoIA {
		return a, erros.Campo("tipo", "Automações de IA são configuradas no automacao.json. Edite o arquivo do projeto.")
	}
	if e.tem("tipo") {
		if t, _ := e.str("tipo"); t == nil || *t != a.Tipo {
			return a, erros.Campo("tipo", "O tipo da automação não pode mudar.")
		}
	}
	antesGat := string(a.Gatilhos)
	campos, defs, mudou := s.aplicar(ctx, &a, e, false)
	if len(campos) > 0 {
		return a, erros.Campos(campos)
	}
	if len(defs) == 0 {
		defs = s.validarEstrutura(a)
	}
	if len(defs) > 0 {
		return a, erroDefinicao(defs)
	}
	if mudou {
		a.Versao++
	}
	a.AtualizadaEm = s.d.Relogio.Agora()
	if err := armazenamento.GravarAutomacao(ctx, s.d.Banco.E(), a); err != nil {
		return a, err
	}
	if a.Ativa && antesGat != string(a.Gatilhos) {
		s.reprogramarAgendamentos(ctx, a)
	}
	a, err = s.Obter(ctx, id)
	if err == nil {
		s.publicar(a)
	}
	return a, err
}

// Excluir apaga a automação (sessões → abortada, runner encerrado, pasta/memória/execuções).
func (s *Servico) Excluir(ctx context.Context, id string) error {
	a, err := armazenamento.ObterAutomacao(ctx, s.d.Banco.L(), id)
	if err != nil {
		return err
	}
	for _, g := range s.ganchos.AoExcluir {
		g(ctx, a)
	}
	ok, err := armazenamento.ExcluirAutomacao(ctx, s.d.Banco.E(), id)
	if err != nil {
		return err
	}
	if !ok {
		return erros.NaoAchada("Automação")
	}
	s.d.Barramento.Publicar(eventos.AutomacaoRemovida, "", map[string]string{"id": id})
	return nil
}

// ---------------------------------------------------------------------------
// Ativação
// ---------------------------------------------------------------------------

// Ativar confere definição/compilação e gatilhos, programa os agendamentos e ativa.
func (s *Servico) Ativar(ctx context.Context, id string) (dominio.Automacao, error) {
	a, err := armazenamento.ObterAutomacao(ctx, s.d.Banco.L(), id)
	if err != nil {
		return a, err
	}
	gs, _ := modelo.DecodificarGatilhos(a.Gatilhos)
	if a.Tipo == modelo.TipoIA {
		if s.ia == nil {
			return a, erros.Novo(erros.RunnerIndisponivel, "As automações de IA não estão disponíveis.")
		}
		if err := s.ia.Ativavel(ctx, a); err != nil {
			return a, err
		}
		a, _ = armazenamento.ObterAutomacao(ctx, s.d.Banco.L(), id)
		gs, _ = modelo.DecodificarGatilhos(a.Gatilhos)
	} else if defs := s.validarEstrutura(a); len(defs) > 0 {
		return a, erroDefinicao(defs)
	}
	if len(gs) == 0 {
		return a, erros.Campo("gatilhos", "Adicione pelo menos um gatilho antes de ativar.")
	}
	a.Ativa, a.DesativadaMotivo, a.ErrosSeguidos = true, nil, 0
	a.AtualizadaEm = s.d.Relogio.Agora()
	if err := armazenamento.GravarAutomacao(ctx, s.d.Banco.E(), a); err != nil {
		return a, err
	}
	s.reprogramarAgendamentos(ctx, a)
	a, err = s.Obter(ctx, id)
	if err == nil {
		s.publicar(a)
	}
	return a, err
}

// reprogramarAgendamentos recria as esperas de gatilhos "agendamento" (uma por gatilho).
func (s *Servico) reprogramarAgendamentos(ctx context.Context, a dominio.Automacao) {
	armazenamento.ApagarEsperasDaAutomacao(ctx, s.d.Banco.E(), a.ID, dominio.EsperaAgendamento)
	gs, _ := modelo.DecodificarGatilhos(a.Gatilhos)
	for i, g := range gs {
		if g.Tipo == modelo.GatilhoAgendamento {
			if err := s.d.Agendador.ProgramarAgendamento(ctx, a.ID, i, g); err != nil {
				s.d.Log.Warn().Err(err).Msg("programar agendamento")
			}
		}
	}
}

// Desativar marca inativa (motivo "usuario") e apaga as esperas de gatilho.
func (s *Servico) Desativar(ctx context.Context, id string) (dominio.Automacao, error) {
	a, err := armazenamento.ObterAutomacao(ctx, s.d.Banco.L(), id)
	if err != nil {
		return a, err
	}
	motivo := "usuario"
	a.Ativa, a.DesativadaMotivo = false, &motivo
	a.AtualizadaEm = s.d.Relogio.Agora()
	if err := armazenamento.GravarAutomacao(ctx, s.d.Banco.E(), a); err != nil {
		return a, err
	}
	s.limparAoDesativar(ctx, a)
	a, err = s.Obter(ctx, id)
	if err == nil {
		s.publicar(a)
	}
	return a, err
}

func (s *Servico) limparAoDesativar(ctx context.Context, a dominio.Automacao) {
	armazenamento.ApagarEsperasDaAutomacao(ctx, s.d.Banco.E(), a.ID, dominio.EsperaAgendamento, dominio.EsperaSemResposta, dominio.EsperaAgendar)
	for _, g := range s.ganchos.AoDesativar {
		g(ctx, a)
	}
}

// ---------------------------------------------------------------------------
// Execução manual e teste
// ---------------------------------------------------------------------------

// AlvoExecucao do contrato (no máximo um).
type AlvoExecucao struct {
	ConversaID string `json:"conversa_id"`
	ContatoID  string `json:"contato_id"`
	LeadID     string `json:"lead_id"`
	Telefone   string `json:"telefone"`
	ContaID    string `json:"conta_id"`
}

// PedidoExecucao do POST /automacoes/{id}/executar.
type PedidoExecucao struct {
	AlvoExecucao
	Entrada json.RawMessage `json:"entrada"`
	Origem  string          `json:"origem"`
}

// ResolverAlvo valida e completa o alvo.
func (s *Servico) ResolverAlvo(ctx context.Context, p AlvoExecucao, campo string) (acoes.Alvo, error) {
	n := 0
	for _, v := range []string{p.ConversaID, p.ContatoID, p.LeadID, p.Telefone} {
		if v != "" {
			n++
		}
	}
	if n > 1 {
		return acoes.Alvo{}, erros.Campo(campo, "Informe no máximo um alvo: conversa, contato, lead ou telefone.")
	}
	l := s.d.Banco.L()
	switch {
	case p.ConversaID != "":
		cv, err := armazenamento.ObterConversa(ctx, l, p.ConversaID)
		if err != nil {
			return acoes.Alvo{}, err
		}
		a := acoes.Alvo{ContaID: cv.ContaID, ConversaID: cv.ID}
		if cv.ContatoID != nil {
			a.ContatoID = *cv.ContatoID
		}
		return a, nil
	case p.ContatoID != "":
		ct, err := armazenamento.ObterContato(ctx, l, p.ContatoID)
		if err != nil {
			return acoes.Alvo{}, err
		}
		a := acoes.Alvo{ContaID: ct.ContaID, ContatoID: ct.ID}
		if ct.ConversaID != nil {
			a.ConversaID = *ct.ConversaID
		}
		return a, nil
	case p.LeadID != "":
		if _, err := armazenamento.ObterLead(ctx, l, p.LeadID); err != nil {
			return acoes.Alvo{}, err
		}
		return acoes.Alvo{LeadID: p.LeadID}, nil
	case p.Telefone != "":
		if p.ContaID == "" {
			return acoes.Alvo{}, erros.Campo("conta_id", "Informe a conta para enviar ao telefone.")
		}
		cv, _, err := s.d.Chat.NovaConversa(ctx, p.ContaID, p.Telefone)
		if err != nil {
			return acoes.Alvo{}, err
		}
		a := acoes.Alvo{ContaID: cv.ContaID, ConversaID: cv.ID}
		if cv.ContatoID != nil {
			a.ContatoID = *cv.ContatoID
		}
		return a, nil
	}
	return acoes.Alvo{}, nil
}

// Executar dispara a automação manualmente (202 Execucao na_fila).
func (s *Servico) Executar(ctx context.Context, id string, p PedidoExecucao) (dominio.Execucao, error) {
	a, err := armazenamento.ObterAutomacao(ctx, s.d.Banco.L(), id)
	if err != nil {
		return dominio.Execucao{}, err
	}
	origem := p.Origem
	if origem == "" {
		origem = "manual_app"
	}
	if origem != "manual_app" && origem != "manual_mcp" {
		return dominio.Execucao{}, erros.Campo("origem", "Use manual_app ou manual_mcp.")
	}
	if a.Tipo == modelo.TipoIA && (s.ia == nil || !s.ia.RunnerDisponivel()) {
		return dominio.Execucao{}, erros.Novo(erros.RunnerIndisponivel, "As automações de IA não podem ser executadas nesta instalação.")
	}
	alvo, err := s.ResolverAlvo(ctx, p.AlvoExecucao, "conversa_id")
	if err != nil {
		return dominio.Execucao{}, err
	}
	if a.Tipo == modelo.TipoChatbot && alvo.ConversaID == "" {
		return dominio.Execucao{}, erros.Campo("conversa_id", "Escolha a conversa onde o chatbot vai começar.")
	}
	dados := map[string]any{"pedido_por": origem}
	var vars map[string]any
	if len(p.Entrada) > 0 && string(p.Entrada) != "null" {
		dados["entrada"] = p.Entrada
	}
	e, err := s.d.Despacho.CriarEEnfileirar(ctx, a, dominio.GatilhoExecucao{Tipo: modelo.GatilhoManual, Dados: dados}, origem, "", nil, alvo, nil, vars)
	if err != nil {
		return dominio.Execucao{}, err
	}
	return e.Detalhe().Execucao, nil
}

// PedidoTeste do POST /automacoes/{id}/testar.
type PedidoTeste struct {
	Mensagem *struct {
		Texto      string `json:"texto"`
		ConversaID string `json:"conversa_id"`
	} `json:"mensagem"`
	MensagemID string          `json:"mensagem_id"`
	Entrada    json.RawMessage `json:"entrada"`
	Evento     *struct {
		Tipo  string         `json:"tipo"`
		Dados map[string]any `json:"dados"`
	} `json:"evento"`
	Alvo       *AlvoExecucao `json:"alvo"`
	IASimulada bool          `json:"ia_simulada"`
}

// Testar roda a automação em simulação e devolve a execução (estado simulacao ou erro).
func (s *Servico) Testar(ctx context.Context, id string, p PedidoTeste) (dominio.ExecucaoDetalhe, error) {
	a, err := armazenamento.ObterAutomacao(ctx, s.d.Banco.L(), id)
	if err != nil {
		return dominio.ExecucaoDetalhe{}, err
	}
	if a.Tipo == modelo.TipoChatbot {
		return dominio.ExecucaoDetalhe{}, erros.Campo("tipo", "Use o chat simulado para testar chatbots.")
	}
	t := s.testes[a.Tipo]
	if t == nil {
		return dominio.ExecucaoDetalhe{}, erros.Novo(erros.RunnerIndisponivel, "As automações de IA não podem ser executadas nesta instalação.")
	}
	return t.Testar(ctx, a, p)
}

// ContextoTeste monta alvo, gatilho e mensagem de um PedidoTeste (compartilhado com a IA).
func (s *Servico) ContextoTeste(ctx context.Context, p PedidoTeste) (acoes.Alvo, dominio.GatilhoExecucao, *string, bool, error) {
	var alvo acoes.Alvo
	var err error
	if p.Alvo != nil {
		if alvo, err = s.ResolverAlvo(ctx, *p.Alvo, "alvo"); err != nil {
			return alvo, dominio.GatilhoExecucao{}, nil, false, err
		}
	}
	g := dominio.GatilhoExecucao{Tipo: modelo.GatilhoManual, Dados: map[string]any{"pedido_por": "teste"}}
	var texto *string
	switch {
	case p.MensagemID != "":
		m, err := armazenamento.ObterMensagem(ctx, s.d.Banco.L(), p.MensagemID)
		if err != nil {
			return alvo, g, nil, false, err
		}
		if alvo, err = s.ResolverAlvo(ctx, AlvoExecucao{ConversaID: m.ConversaID}, "mensagem_id"); err != nil {
			return alvo, g, nil, false, err
		}
		g = dominio.GatilhoExecucao{Tipo: modelo.GatilhoMensagemRecebida, Dados: map[string]any{"mensagem_id": m.ID}}
		texto = m.Texto
		if texto == nil {
			v := ""
			texto = &v
		}
	case p.Mensagem != nil:
		if p.Mensagem.ConversaID != "" {
			if alvo, err = s.ResolverAlvo(ctx, AlvoExecucao{ConversaID: p.Mensagem.ConversaID}, "mensagem.conversa_id"); err != nil {
				return alvo, g, nil, false, err
			}
		}
		g = dominio.GatilhoExecucao{Tipo: modelo.GatilhoMensagemRecebida, Dados: map[string]any{"mensagem_id": nil}}
		t := p.Mensagem.Texto
		texto = &t
	case p.Evento != nil:
		dados := p.Evento.Dados
		if dados == nil {
			dados = map[string]any{}
		}
		g = dominio.GatilhoExecucao{Tipo: p.Evento.Tipo, Dados: dados}
	}
	ficticia := alvo.ConversaID == "" && alvo.ContatoID == "" && alvo.LeadID == "" && texto != nil
	return alvo, g, texto, ficticia, nil
}

type testadorFluxo struct{ s *Servico }

func (t testadorFluxo) Testar(ctx context.Context, a dominio.Automacao, p PedidoTeste) (dominio.ExecucaoDetalhe, error) {
	s := t.s
	alvo, g, texto, ficticia, err := s.ContextoTeste(ctx, p)
	if err != nil {
		return dominio.ExecucaoDetalhe{}, err
	}
	e, err := s.d.Registro.Criar(ctx, execucoes.Nova{Automacao: a, Gatilho: g, Origem: "teste", Simulacao: true,
		Alvo: execucoes.Alvo{ContaID: alvo.ContaID, ConversaID: alvo.ConversaID, ContatoID: alvo.ContatoID, LeadID: alvo.LeadID}})
	if err != nil {
		return dominio.ExecucaoDetalhe{}, err
	}
	c := &acoes.Contexto{Exec: e, Automacao: a, Alvo: alvo, Simulacao: true, Cadeia: []string{a.ID}, Variaveis: map[string]string{},
		UltimaMensagem: texto, Origem: modelo.TipoFluxo, ConversaFicticia: ficticia, IASimulada: p.IASimulada}
	if ficticia {
		e.Logar("info", "Conversa fictícia: Contato de teste (+5500000000000), sem histórico.")
	}
	ctxT, cancelar := context.WithTimeout(ctx, 30*time.Second)
	defer cancelar()
	if err := s.d.Fluxo.Executar(ctxT, e, a, c); err != nil {
		return dominio.ExecucaoDetalhe{}, err
	}
	return armazenamento.ObterExecucao(ctx, s.d.Banco.L(), e.ID())
}

// ---------------------------------------------------------------------------
// Execuções, pausas, estado da conversa, segredos
// ---------------------------------------------------------------------------

// Execucoes pagina execuções.
func (s *Servico) Execucoes(ctx context.Context, f armazenamento.FiltroExecucoes, p pagina.Params) ([]dominio.Execucao, string, error) {
	if f.AutomacaoID != "" {
		if _, err := armazenamento.ObterAutomacao(ctx, s.d.Banco.L(), f.AutomacaoID); err != nil {
			return nil, "", err
		}
	}
	if f.Estado != "" {
		switch f.Estado {
		case dominio.ExecNaFila, dominio.ExecRodando, dominio.ExecAguardando, dominio.ExecOK, dominio.ExecErro, dominio.ExecSimulacao, dominio.ExecAbortada:
		default:
			return nil, "", erros.Campo("estado", "Estado de execução inválido.")
		}
	}
	return armazenamento.ListarExecucoes(ctx, s.d.Banco.L(), f, p)
}

// Execucao devolve o detalhe.
func (s *Servico) Execucao(ctx context.Context, id string) (dominio.ExecucaoDetalhe, error) {
	return armazenamento.ObterExecucao(ctx, s.d.Banco.L(), id)
}

// EstadoConversa de GET /conversas/{id}/automacoes.
func (s *Servico) EstadoConversa(ctx context.Context, conversaID string) (dominio.EstadoConversaAutomacoes, error) {
	if _, err := armazenamento.ObterConversa(ctx, s.d.Banco.L(), conversaID); err != nil {
		return dominio.EstadoConversaAutomacoes{}, err
	}
	est := dominio.EstadoConversaAutomacoes{ConversaID: conversaID, PausaGeral: s.d.Config.Configuracao().PausaGeral}
	if p, ok := s.d.Portao.Pausas().Obter(ctx, conversaID); ok {
		est.Pausa = &p
	}
	if se, ok, _ := armazenamento.SessaoAtivaDaConversa(ctx, s.d.Banco.L(), conversaID); ok {
		est.Sessao = &se
	}
	return est, nil
}

// Pausar é o "Assumir" (motivo humano|manual; duração nil = sem prazo).
func (s *Servico) Pausar(ctx context.Context, conversaID, motivo string, duracaoMin *int) (dominio.Pausa, error) {
	if _, err := armazenamento.ObterConversa(ctx, s.d.Banco.L(), conversaID); err != nil {
		return dominio.Pausa{}, err
	}
	if motivo != dominio.PausaHumano && motivo != dominio.PausaManual {
		return dominio.Pausa{}, erros.Campo("motivo", "Use humano ou manual.")
	}
	if duracaoMin != nil && (*duracaoMin < 1 || *duracaoMin > 10080) {
		return dominio.Pausa{}, erros.Campo("duracao_min", "Use de 1 a 10.080 minutos (ou sem prazo).")
	}
	return s.d.Portao.Pausas().Assumir(ctx, conversaID, motivo, duracaoMin), nil
}

// Retomar remove a pausa da conversa.
func (s *Servico) Retomar(ctx context.Context, conversaID string) error {
	if _, err := armazenamento.ObterConversa(ctx, s.d.Banco.L(), conversaID); err != nil {
		return err
	}
	s.d.Portao.Pausas().Retomar(ctx, conversaID)
	return nil
}

// Pausas válidas (motivo opcional).
func (s *Servico) Pausas(ctx context.Context, motivo string) ([]dominio.Pausa, error) {
	if motivo != "" && motivo != dominio.PausaHumano && motivo != dominio.PausaAntiLoop && motivo != dominio.PausaManual {
		return nil, erros.Campo("motivo", "Use humano, anti_loop ou manual.")
	}
	return s.d.Portao.Pausas().Listar(ctx, motivo)
}

// Segredo da API (só nomes).
type Segredo struct {
	Nome      string       `json:"nome"`
	Reservado bool         `json:"reservado"`
	UsadoPor  []UsoSegredo `json:"usado_por"`
}

// UsoSegredo por uma automação.
type UsoSegredo struct {
	AutomacaoID string `json:"automacao_id"`
	Nome        string `json:"nome"`
}

// Segredos lista os nomes presentes (e os declarados por automações) com quem os usa.
func (s *Servico) Segredos(ctx context.Context) ([]Segredo, error) {
	auts, err := armazenamento.ListarAutomacoes(ctx, s.d.Banco.L(), armazenamento.FiltroAutomacoes{Tipo: modelo.TipoIA})
	if err != nil {
		return nil, err
	}
	por := map[string]*Segredo{}
	var ordem []string
	add := func(n string) *Segredo {
		if x, ok := por[n]; ok {
			return x
		}
		x := &Segredo{Nome: n, Reservado: segredos.Reservado(n), UsadoPor: []UsoSegredo{}}
		por[n] = x
		ordem = append(ordem, n)
		return x
	}
	for _, n := range s.d.Cofre.Nomes() {
		add(n)
	}
	for _, a := range auts {
		for _, n := range a.Segredos {
			x := add(n)
			x.UsadoPor = append(x.UsadoPor, UsoSegredo{AutomacaoID: a.ID, Nome: a.Nome})
		}
		if a.Permissoes != nil {
			for _, p := range a.Permissoes {
				if p == modelo.PermIA {
					x := add(segredos.ChaveAnthropic)
					x.UsadoPor = append(x.UsadoPor, UsoSegredo{AutomacaoID: a.ID, Nome: a.Nome})
				}
			}
		}
	}
	l := make([]Segredo, 0, len(ordem))
	for _, n := range ordem {
		l = append(l, *por[n])
	}
	return l, nil
}

// AutomacoesAtivas, ProcessosIA e RunnerDisponivel alimentam GET /sistema.
func (s *Servico) AutomacoesAtivas() int {
	n, _ := armazenamento.ContarAutomacoesAtivas(context.Background(), s.d.Banco.L())
	return n
}

// ProcessosIA vivos.
func (s *Servico) ProcessosIA() int {
	if s.ia == nil {
		return 0
	}
	return s.ia.ProcessosVivos()
}

// RunnerDisponivel indica se a IA pode executar.
func (s *Servico) RunnerDisponivel() bool { return s.ia != nil && s.ia.RunnerDisponivel() }
