package modelo

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/robfig/cron/v3"
)

// Limites estruturais de formatos.md.
const (
	MaxAcoesFluxo      = 50
	MaxRegras          = 20
	MinNos, MaxNos     = 2, 200
	MaxOpcoesMenu      = 10
	MinAposS, MaxAposS = 60, 2592000
	MinIntervaloS      = 60
	MaxRegex           = 500
	MaxTextoMensagem   = 4096
	MaxNota            = 10000
	MaxTituloNotificar = 60
	MaxTextoNotificar  = 240
	MaxGatilhos        = 20
)

var (
	reID       = regexp.MustCompile(`^[A-Za-z0-9_-]{1,40}$`)
	reVariavel = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,39}$`)
	reHorario  = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
)

// IDValido indica se o id de ação/nó segue `[A-Za-z0-9_-]{1,40}`.
func IDValido(id string) bool { return reID.MatchString(id) }

// VariavelValida indica se o nome de variável segue `[a-z_][a-z0-9_]{0,39}`.
func VariavelValida(v string) bool { return reVariavel.MatchString(v) }

// ParserCron é o parser de 5 campos (ParseStandard do robfig/cron).
var ParserCron = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// coletor acumula erros com o contexto (nó/ação) atual.
type coletor struct {
	erros  []ErroDefinicao
	noID   *string
	acaoID *string
}

func (c *coletor) add(caminho, mensagem string, args ...any) {
	if len(args) > 0 {
		mensagem = fmt.Sprintf(mensagem, args...)
	}
	c.erros = append(c.erros, ErroDefinicao{Caminho: caminho, NoID: c.noID, AcaoID: c.acaoID, Mensagem: mensagem})
}

func tamanho(s string) int { return utf8.RuneCountInString(s) }

func vazio(s string) bool { return strings.TrimSpace(s) == "" }

func validarRegex(c *coletor, caminho, padrao string) {
	if padrao == "" || tamanho(padrao) > MaxRegex {
		c.add(caminho, "A expressão regular deve ter de 1 a 500 caracteres.")
		return
	}
	if _, err := regexp.Compile(padrao); err != nil {
		c.add(caminho, "Expressão regular inválida: %s", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Gatilhos
// ---------------------------------------------------------------------------

// OpcoesGatilhos ajusta a validação ao contexto.
type OpcoesGatilhos struct {
	IncluirGrupos bool
	Manifesto     bool // aceita nomes no lugar de ids
}

// ValidarGatilhos valida a lista de gatilhos (caminho base "gatilhos").
func ValidarGatilhos(gs []Gatilho, o OpcoesGatilhos) []ErroDefinicao {
	c := &coletor{}
	if len(gs) > MaxGatilhos {
		c.add("gatilhos", "No máximo %d gatilhos.", MaxGatilhos)
	}
	for i, g := range gs {
		validarGatilho(c, fmt.Sprintf("gatilhos[%d]", i), g, o)
	}
	return c.erros
}

func validarGatilho(c *coletor, p string, g Gatilho, o OpcoesGatilhos) {
	switch g.Tipo {
	case GatilhoMensagemRecebida:
		if g.Contem != nil && vazio(*g.Contem) {
			c.add(p+".contem", "Informe o trecho ou deixe em branco.")
		}
		if g.Regex != nil {
			validarRegex(c, p+".regex", *g.Regex)
		}
		switch g.TipoConversa {
		case "", "individual", "qualquer":
		case "grupo":
			if !o.IncluirGrupos {
				c.add(p+".tipo_conversa", "Gatilho de grupo exige \"Incluir grupos\".")
			}
		default:
			c.add(p+".tipo_conversa", "Use individual, grupo ou qualquer.")
		}
	case GatilhoPalavraChave:
		if len(g.Palavras) == 0 {
			c.add(p+".palavras", "Informe pelo menos uma palavra.")
		}
		for j, w := range g.Palavras {
			if NormalizarTexto(w) == "" || tamanho(w) > 100 {
				c.add(fmt.Sprintf("%s.palavras[%d]", p, j), "Palavra vazia ou longa demais (até 100 caracteres).")
			}
		}
		if g.Modo != "" && g.Modo != "palavra" && g.Modo != "mensagem_inteira" {
			c.add(p+".modo", "Use palavra ou mensagem_inteira.")
		}
	case GatilhoLeadImportado:
		for j, org := range g.Origens {
			switch org {
			case "csv", "colado", "contatos", "mcp":
			default:
				c.add(fmt.Sprintf("%s.origens[%d]", p, j), "Origem inválida (use csv, colado, contatos ou mcp).")
			}
		}
	case GatilhoEtiqueta:
		if g.Evento != "adicionada" && g.Evento != "removida" {
			c.add(p+".evento", "Use adicionada ou removida.")
		}
		if g.EtiquetaID == "" && !(o.Manifesto && g.Etiqueta != "") {
			c.add(p+".etiqueta_id", "Escolha a etiqueta.")
		}
	case GatilhoEntrouEtapa:
		if g.FunilID == "" && !(o.Manifesto && g.Funil != "") {
			c.add(p+".funil_id", "Escolha o funil.")
		}
		if g.EtapaID == "" && !(o.Manifesto && g.Etapa != "") {
			c.add(p+".etapa_id", "Escolha a etapa.")
		}
	case GatilhoDisparoRespondeu:
	case GatilhoSemResposta:
		if g.AposS < MinAposS || g.AposS > MaxAposS {
			c.add(p+".apos_s", "O tempo deve ficar entre 1 minuto e 30 dias.")
		}
		switch g.OrigemMensagem {
		case "", "qualquer", "disparo", "automacao", "manual":
		default:
			c.add(p+".origem_mensagem", "Use qualquer, disparo, automacao ou manual.")
		}
	case GatilhoAgendamento:
		temCron, temIntervalo := g.Cron != "", g.IntervaloS != 0
		switch {
		case temCron == temIntervalo:
			c.add(p, "Informe exatamente um: cron ou intervalo_s.")
		case temCron:
			if len(strings.Fields(g.Cron)) != 5 {
				c.add(p+".cron", "O cron deve ter 5 campos (minuto hora dia mês dia-da-semana).")
			} else if _, err := ParserCron.Parse(g.Cron); err != nil {
				c.add(p+".cron", "Cron inválido: %s", err.Error())
			}
		default:
			if g.IntervaloS < MinIntervaloS {
				c.add(p+".intervalo_s", "O intervalo mínimo é de 60 segundos.")
			}
		}
	case GatilhoManual:
	case "":
		c.add(p+".tipo", "Informe o tipo do gatilho.")
	default:
		c.add(p+".tipo", "Tipo de gatilho desconhecido: %s.", g.Tipo)
	}
}

// ---------------------------------------------------------------------------
// Condições
// ---------------------------------------------------------------------------

var operadoresCampo = map[string]bool{"igual": true, "diferente": true, "contem": true, "existe": true,
	"nao_existe": true, "maior": true, "menor": true}

func validarCondicoes(c *coletor, p string, cd *Condicoes) {
	if cd == nil {
		return
	}
	if cd.Modo != ModoTodas && cd.Modo != ModoAlguma {
		c.add(p+".modo", "Use todas ou alguma.")
	}
	if len(cd.Regras) > MaxRegras {
		c.add(p+".regras", "No máximo %d regras.", MaxRegras)
	}
	for i, r := range cd.Regras {
		validarRegra(c, fmt.Sprintf("%s.regras[%d]", p, i), r)
	}
}

func precisaValor(op string) bool { return op != "existe" && op != "nao_existe" }

func validarRegra(c *coletor, p string, r Regra) {
	switch r.Tipo {
	case RegraEtiqueta:
		if r.Operador != "tem" && r.Operador != "nao_tem" {
			c.add(p+".operador", "Use tem ou nao_tem.")
		}
		if r.EtiquetaID == "" {
			c.add(p+".etiqueta_id", "Escolha a etiqueta.")
		}
	case RegraEtapa:
		if r.Operador != "esta" && r.Operador != "nao_esta" {
			c.add(p+".operador", "Use esta ou nao_esta.")
		}
		if r.FunilID == "" {
			c.add(p+".funil_id", "Escolha o funil.")
		}
	case RegraCampoLead:
		if vazio(r.Campo) {
			c.add(p+".campo", "Informe o campo.")
		}
		if !operadoresCampo[r.Operador] {
			c.add(p+".operador", "Operador inválido.")
		} else if precisaValor(r.Operador) && r.Valor == nil {
			c.add(p+".valor", "Informe o valor.")
		}
	case RegraHorario:
		if !reHorario.MatchString(r.Inicio) {
			c.add(p+".inicio", "Use o formato HH:MM.")
		}
		if !reHorario.MatchString(r.Fim) {
			c.add(p+".fim", "Use o formato HH:MM.")
		}
		for j, d := range r.Dias {
			if d < 1 || d > 7 {
				c.add(fmt.Sprintf("%s.dias[%d]", p, j), "Dia deve ser de 1 (segunda) a 7 (domingo).")
			}
		}
	case RegraTexto:
		switch r.Operador {
		case "contem", "igual":
			if r.Valor == nil || *r.Valor == "" {
				c.add(p+".valor", "Informe o texto.")
			}
		case "regex":
			v := ""
			if r.Valor != nil {
				v = *r.Valor
			}
			validarRegex(c, p+".valor", v)
		default:
			c.add(p+".operador", "Use contem, igual ou regex.")
		}
	case RegraConta:
		if len(r.ContaIDs) == 0 {
			c.add(p+".conta_ids", "Escolha pelo menos uma conta.")
		}
	case RegraVariavel:
		if !VariavelValida(r.Variavel) {
			c.add(p+".variavel", "Nome de variável inválido.")
		}
		if r.Operador == "regex" {
			v := ""
			if r.Valor != nil {
				v = *r.Valor
			}
			validarRegex(c, p+".valor", v)
		} else if !operadoresCampo[r.Operador] {
			c.add(p+".operador", "Operador inválido.")
		} else if precisaValor(r.Operador) && r.Valor == nil {
			c.add(p+".valor", "Informe o valor.")
		}
	case "":
		c.add(p+".tipo", "Informe o tipo da regra.")
	default:
		c.add(p+".tipo", "Tipo de regra desconhecido: %s.", r.Tipo)
	}
}

// ---------------------------------------------------------------------------
// Ações
// ---------------------------------------------------------------------------

// contextoAcao diz onde a ação está (chatbot proíbe aguardar e iniciar_chatbot).
type contextoAcao struct{ chatbot bool }

func validarAcao(c *coletor, p string, a Acao, ctx contextoAcao) {
	if a.ID != "" && !IDValido(a.ID) {
		c.add(p+".id", "Id inválido (use letras, números, _ e -, até 40).")
	}
	switch a.Tipo {
	case AcaoEnviarTexto:
		if vazio(a.Texto) || tamanho(a.Texto) > MaxTextoMensagem {
			c.add(p+".texto", "O texto deve ter de 1 a 4.096 caracteres.")
		}
	case AcaoEnviarTemplate:
		if a.TemplateID == "" {
			c.add(p+".template_id", "Escolha o template.")
		}
	case AcaoAguardar:
		if ctx.chatbot {
			c.add(p+".tipo", "Aguardar não é permitido em chatbot.")
		} else if a.DuracaoS < MinAposS || a.DuracaoS > MaxAposS {
			c.add(p+".duracao_s", "A espera deve ficar entre 1 minuto e 30 dias.")
		}
	case AcaoAdicionarEtiqueta, AcaoRemoverEtiqueta:
		if a.EtiquetaID == "" {
			c.add(p+".etiqueta_id", "Escolha a etiqueta.")
		}
	case AcaoMoverEtapa:
		if a.FunilID == "" {
			c.add(p+".funil_id", "Escolha o funil.")
		}
		if a.EtapaID == "" {
			c.add(p+".etapa_id", "Escolha a etapa.")
		}
	case AcaoRemoverDoFunil:
		if a.FunilID == "" {
			c.add(p+".funil_id", "Escolha o funil.")
		}
	case AcaoAtualizarNota:
		if vazio(a.Texto) || tamanho(a.Texto) > MaxNota {
			c.add(p+".texto", "O texto deve ter de 1 a 10.000 caracteres.")
		}
		if a.Modo != "substituir" && a.Modo != "acrescentar" {
			c.add(p+".modo", "Use substituir ou acrescentar.")
		}
	case AcaoAtualizarCampoLead:
		if vazio(a.Campo) || tamanho(a.Campo) > 60 {
			c.add(p+".campo", "Informe o campo (até 60 caracteres).")
		}
		if a.Valor == nil {
			c.add(p+".valor", "Informe o valor (vazio remove o campo).")
		} else if tamanho(*a.Valor) > 1000 {
			c.add(p+".valor", "O valor pode ter até 1.000 caracteres.")
		}
	case AcaoIniciarChatbot:
		if ctx.chatbot {
			c.add(p+".tipo", "Iniciar chatbot não é permitido dentro de um chatbot.")
		} else if a.AutomacaoID == "" {
			c.add(p+".automacao_id", "Escolha o chatbot.")
		}
	case AcaoExecutarIA:
		if a.AutomacaoID == "" {
			c.add(p+".automacao_id", "Escolha a automação de IA.")
		}
		if a.SalvarEm != nil && !VariavelValida(*a.SalvarEm) {
			c.add(p+".salvar_em", "Nome de variável inválido.")
		}
	case AcaoAdicionarADisparo:
		if a.DisparoID == "" {
			c.add(p+".disparo_id", "Escolha o disparo.")
		}
	case AcaoPausarAutomacoes:
		if a.DuracaoMin != nil && (*a.DuracaoMin < 1 || *a.DuracaoMin > 10080) {
			c.add(p+".duracao_min", "A pausa deve ter de 1 a 10.080 minutos (ou sem prazo).")
		}
	case AcaoNotificar:
		if vazio(a.Titulo) || tamanho(a.Titulo) > MaxTituloNotificar {
			c.add(p+".titulo", "O título deve ter de 1 a 60 caracteres.")
		}
		if vazio(a.Texto) || tamanho(a.Texto) > MaxTextoNotificar {
			c.add(p+".texto", "O texto deve ter de 1 a 240 caracteres.")
		}
	case "":
		c.add(p+".tipo", "Informe o tipo da ação.")
	default:
		c.add(p+".tipo", "Tipo de ação desconhecido: %s.", a.Tipo)
	}
}

// ---------------------------------------------------------------------------
// Fluxo
// ---------------------------------------------------------------------------

// NormalizarFluxo gera ids ausentes das ações (a1, a2… sem colidir) e fixa versao=1.
func NormalizarFluxo(d *DefinicaoFluxo) {
	if d.Versao == 0 {
		d.Versao = 1
	}
	usados := map[string]bool{}
	for _, a := range d.Acoes {
		if a.ID != "" {
			usados[a.ID] = true
		}
	}
	n := 1
	for i := range d.Acoes {
		if d.Acoes[i].ID != "" {
			continue
		}
		for usados[fmt.Sprintf("a%d", n)] {
			n++
		}
		d.Acoes[i].ID = fmt.Sprintf("a%d", n)
		usados[d.Acoes[i].ID] = true
	}
}

// ValidarFluxo valida a definição de fluxo (caminho base "definicao").
func ValidarFluxo(d *DefinicaoFluxo) []ErroDefinicao {
	c := &coletor{}
	if d == nil {
		c.add("definicao", "Informe a definição do fluxo.")
		return c.erros
	}
	if d.Versao != 1 {
		c.add("definicao.versao", "Versão de definição não suportada (use 1).")
	}
	validarCondicoes(c, "definicao.condicoes", d.Condicoes)
	if len(d.Acoes) == 0 || len(d.Acoes) > MaxAcoesFluxo {
		c.add("definicao.acoes", "O fluxo deve ter de 1 a 50 ações.")
	}
	ids := map[string]bool{}
	for i, a := range d.Acoes {
		id := a.ID
		if id != "" {
			c.acaoID = &id
			if ids[id] {
				c.add(fmt.Sprintf("definicao.acoes[%d].id", i), "Id de ação repetido: %s.", id)
			}
			ids[id] = true
		}
		validarAcao(c, fmt.Sprintf("definicao.acoes[%d]", i), a, contextoAcao{})
		c.acaoID = nil
	}
	return c.erros
}

// ValidarLimites valida `limites` (anti-loop 1–100 msgs / 1–1440 min; tempo 5–300 s; memória
// 64–2048 MB). A regra "só mais restritivo que o global" é do serviço (depende da configuração).
func ValidarLimites(l Limites) []ErroDefinicao {
	c := &coletor{}
	if l.AntiLoop != nil {
		if l.AntiLoop.Mensagens < 1 || l.AntiLoop.Mensagens > 100 {
			c.add("limites.anti_loop.mensagens", "Use de 1 a 100 mensagens.")
		}
		if l.AntiLoop.JanelaMin < 1 || l.AntiLoop.JanelaMin > 1440 {
			c.add("limites.anti_loop.janela_min", "Use uma janela de 1 a 1.440 minutos.")
		}
	}
	if l.TempoS != nil && (*l.TempoS < 5 || *l.TempoS > 300) {
		c.add("limites.tempo_s", "O tempo limite deve ficar entre 5 e 300 segundos.")
	}
	if l.MemoriaMB != nil && (*l.MemoriaMB < 64 || *l.MemoriaMB > 2048) {
		c.add("limites.memoria_mb", "A memória deve ficar entre 64 e 2.048 MB.")
	}
	return c.erros
}
