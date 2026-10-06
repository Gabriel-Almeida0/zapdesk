// Pacote modelo define os tipos Go de specs/002-automacoes/contracts/formatos.md (gatilhos,
// condições, ações, fluxo, chatbot e manifesto `automacao.json`) com as tags JSON do contrato e
// a validação estrutural que devolve []ErroDefinicao. Não acessa banco: referências (ids que não
// existem mais) são checadas pelo serviço e viram avisos.
package modelo

import (
	"encoding/json"

	"zapdesk/motor/internal/dominio"
)

// Tipos de automação.
const (
	TipoFluxo   = "fluxo"
	TipoChatbot = "chatbot"
	TipoIA      = "ia"
)

// ErroDefinicao aponta um problema num campo da definição (contracts/api-http.md › Tipos).
type ErroDefinicao = dominio.ErroDefinicao

// Tipos de gatilho.
const (
	GatilhoMensagemRecebida = "mensagem_recebida"
	GatilhoPalavraChave     = "palavra_chave"
	GatilhoLeadImportado    = "lead_importado"
	GatilhoEtiqueta         = "etiqueta"
	GatilhoEntrouEtapa      = "entrou_etapa"
	GatilhoDisparoRespondeu = "disparo_respondeu"
	GatilhoSemResposta      = "sem_resposta"
	GatilhoAgendamento      = "agendamento"
	GatilhoManual           = "manual"
	// GatilhoAgendar é o tipo do fato de `ctx.agendar` (não configurável em gatilhos).
	GatilhoAgendar = "agendar"
)

// Gatilho é a união de todos os tipos (campos do tipo em uso; os demais vazios).
type Gatilho struct {
	Tipo string `json:"tipo"`
	// mensagem_recebida
	Contem           *string `json:"contem,omitempty"`
	Regex            *string `json:"regex,omitempty"`
	PrimeiraMensagem bool    `json:"primeira_mensagem,omitempty"`
	TipoConversa     string  `json:"tipo_conversa,omitempty"`
	// palavra_chave
	Palavras []string `json:"palavras,omitempty"`
	Modo     string   `json:"modo,omitempty"`
	// lead_importado
	Origens []string `json:"origens,omitempty"`
	// etiqueta
	Evento     string `json:"evento,omitempty"`
	EtiquetaID string `json:"etiqueta_id,omitempty"`
	// entrou_etapa
	FunilID string `json:"funil_id,omitempty"`
	EtapaID string `json:"etapa_id,omitempty"`
	// disparo_respondeu
	DisparoID *string `json:"disparo_id,omitempty"`
	// sem_resposta
	AposS          int    `json:"apos_s,omitempty"`
	OrigemMensagem string `json:"origem_mensagem,omitempty"`
	// agendamento
	Cron       string `json:"cron,omitempty"`
	IntervaloS int    `json:"intervalo_s,omitempty"`
	// só no manifesto (nomes no lugar de ids, resolvidos na compilação)
	Etiqueta string `json:"etiqueta,omitempty"`
	Funil    string `json:"funil,omitempty"`
	Etapa    string `json:"etapa,omitempty"`
}

// MarshalJSON emite os campos anuláveis obrigatórios do tipo.
func (g Gatilho) MarshalJSON() ([]byte, error) {
	type semMetodo Gatilho
	return comNulos(semMetodo(g), g.Tipo, nulosGatilho)
}

var nulosGatilho = map[string][]string{GatilhoDisparoRespondeu: {"disparo_id"}}

// Operadores e modos das condições.
const (
	ModoTodas  = "todas"
	ModoAlguma = "alguma"
)

// Condicoes agrupa regras (null ou regras vazias = verdadeiro).
type Condicoes struct {
	Modo   string  `json:"modo"`
	Regras []Regra `json:"regras"`
}

// Tipos de regra.
const (
	RegraEtiqueta  = "etiqueta"
	RegraEtapa     = "etapa"
	RegraCampoLead = "campo_lead"
	RegraHorario   = "horario"
	RegraTexto     = "texto"
	RegraConta     = "conta"
	RegraVariavel  = "variavel"
)

// Regra é a união das regras de condição.
type Regra struct {
	Tipo       string   `json:"tipo"`
	Operador   string   `json:"operador,omitempty"`
	EtiquetaID string   `json:"etiqueta_id,omitempty"`
	FunilID    string   `json:"funil_id,omitempty"`
	EtapaID    *string  `json:"etapa_id,omitempty"`
	Campo      string   `json:"campo,omitempty"`
	Valor      *string  `json:"valor,omitempty"`
	Inicio     string   `json:"inicio,omitempty"`
	Fim        string   `json:"fim,omitempty"`
	Dias       []int    `json:"dias,omitempty"`
	ContaIDs   []string `json:"conta_ids,omitempty"`
	Variavel   string   `json:"variavel,omitempty"`
}

// MarshalJSON emite os campos anuláveis obrigatórios do tipo.
func (r Regra) MarshalJSON() ([]byte, error) {
	type semMetodo Regra
	return comNulos(semMetodo(r), r.Tipo, nulosRegra)
}

var nulosRegra = map[string][]string{RegraEtapa: {"etapa_id"}, RegraCampoLead: {"valor"}, RegraVariavel: {"valor"}}

// Tipos de ação.
const (
	AcaoEnviarTexto        = "enviar_texto"
	AcaoEnviarTemplate     = "enviar_template"
	AcaoAguardar           = "aguardar"
	AcaoAdicionarEtiqueta  = "adicionar_etiqueta"
	AcaoRemoverEtiqueta    = "remover_etiqueta"
	AcaoMoverEtapa         = "mover_etapa"
	AcaoRemoverDoFunil     = "remover_do_funil"
	AcaoAtualizarNota      = "atualizar_nota"
	AcaoAtualizarCampoLead = "atualizar_campo_lead"
	AcaoIniciarChatbot     = "iniciar_chatbot"
	AcaoExecutarIA         = "executar_ia"
	AcaoAdicionarADisparo  = "adicionar_a_disparo"
	AcaoPausarAutomacoes   = "pausar_automacoes"
	AcaoNotificar          = "notificar"
)

// Acao é a união das ações de fluxo (e do nó "ação" do chatbot).
type Acao struct {
	ID            string            `json:"id,omitempty"`
	Tipo          string            `json:"tipo"`
	Texto         string            `json:"texto,omitempty"`
	ValoresPadrao map[string]string `json:"valores_padrao,omitempty"`
	TemplateID    string            `json:"template_id,omitempty"`
	DuracaoS      int               `json:"duracao_s,omitempty"`
	EtiquetaID    string            `json:"etiqueta_id,omitempty"`
	FunilID       string            `json:"funil_id,omitempty"`
	EtapaID       string            `json:"etapa_id,omitempty"`
	Modo          string            `json:"modo,omitempty"`
	Campo         string            `json:"campo,omitempty"`
	Valor         *string           `json:"valor,omitempty"`
	AutomacaoID   string            `json:"automacao_id,omitempty"`
	Entrada       map[string]any    `json:"entrada,omitempty"`
	SalvarEm      *string           `json:"salvar_em,omitempty"`
	DisparoID     string            `json:"disparo_id,omitempty"`
	DuracaoMin    *int              `json:"duracao_min,omitempty"`
	Titulo        string            `json:"titulo,omitempty"`
}

// MarshalJSON emite os campos anuláveis obrigatórios do tipo.
func (a Acao) MarshalJSON() ([]byte, error) {
	type semMetodo Acao
	return comNulos(semMetodo(a), a.Tipo, nulosAcao)
}

var nulosAcao = map[string][]string{AcaoPausarAutomacoes: {"duracao_min"}, AcaoExecutarIA: {"salvar_em"}}

// DefinicaoFluxo de uma automação do tipo fluxo.
type DefinicaoFluxo struct {
	Versao    int        `json:"versao"`
	Condicoes *Condicoes `json:"condicoes"`
	Acoes     []Acao     `json:"acoes"`
}

// Tipos de nó do chatbot.
const (
	NoInicio   = "inicio"
	NoMensagem = "mensagem"
	NoMenu     = "menu"
	NoPergunta = "pergunta"
	NoCondicao = "condicao"
	NoAcao     = "acao"
	NoIA       = "ia"
	NoHumano   = "humano"
	NoFim      = "fim"
)

// Posicao no canvas (ignorada pelo executor).
type Posicao struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// OpcaoMenu de um nó menu.
type OpcaoMenu struct {
	Rotulo  string   `json:"rotulo"`
	Valores []string `json:"valores"`
	Proximo string   `json:"proximo"`
}

// ValidacaoPergunta de um nó pergunta.
type ValidacaoPergunta struct {
	Tipo         string  `json:"tipo"`
	Padrao       *string `json:"padrao"`
	MensagemErro *string `json:"mensagem_erro"`
}

// RamoCondicao de um nó condição.
type RamoCondicao struct {
	Condicoes Condicoes `json:"condicoes"`
	Proximo   string    `json:"proximo"`
}

// No é a união dos nós do chatbot.
type No struct {
	ID             string             `json:"id"`
	Tipo           string             `json:"tipo"`
	Posicao        *Posicao           `json:"posicao,omitempty"`
	Proximo        string             `json:"proximo,omitempty"`
	Texto          string             `json:"texto,omitempty"`
	TemplateID     *string            `json:"template_id,omitempty"`
	Opcoes         []OpcaoMenu        `json:"opcoes,omitempty"`
	MostrarNumeros *bool              `json:"mostrar_numeros,omitempty"`
	AoEsgotar      *string            `json:"ao_esgotar,omitempty"`
	Variavel       *string            `json:"variavel,omitempty"`
	Validacao      *ValidacaoPergunta `json:"validacao,omitempty"`
	Ramos          []RamoCondicao     `json:"ramos,omitempty"`
	Senao          string             `json:"senao,omitempty"`
	Acao           *Acao              `json:"acao,omitempty"`
	AutomacaoID    string             `json:"automacao_id,omitempty"`
	Entrada        map[string]any     `json:"entrada,omitempty"`
	ModoIA         string             `json:"modo,omitempty"`
	EmErro         *string            `json:"em_erro,omitempty"`
	MensagemHumFim *string            `json:"mensagem,omitempty"`
}

// MarshalJSON emite os campos anuláveis obrigatórios do tipo.
func (n No) MarshalJSON() ([]byte, error) {
	type semMetodo No
	return comNulos(semMetodo(n), n.Tipo, nulosNo)
}

var nulosNo = map[string][]string{
	NoMensagem: {"template_id"}, NoMenu: {"ao_esgotar"}, NoPergunta: {"ao_esgotar", "validacao"},
	NoIA: {"variavel", "em_erro"}, NoHumano: {"mensagem"}, NoFim: {"mensagem"},
}

// MostraNumeros devolve mostrar_numeros (padrão true).
func (n No) MostraNumeros() bool { return n.MostrarNumeros == nil || *n.MostrarNumeros }

// Saidas devolve os ids de destino do nó (na ordem).
func (n No) Saidas() []string {
	var s []string
	add := func(id string) {
		if id != "" {
			s = append(s, id)
		}
	}
	add(n.Proximo)
	for _, o := range n.Opcoes {
		add(o.Proximo)
	}
	for _, r := range n.Ramos {
		add(r.Proximo)
	}
	add(n.Senao)
	if n.AoEsgotar != nil {
		add(*n.AoEsgotar)
	}
	if n.EmErro != nil {
		add(*n.EmErro)
	}
	return s
}

// DefinicaoChatbot de uma automação do tipo chatbot.
type DefinicaoChatbot struct {
	Versao         int    `json:"versao"`
	Inicio         string `json:"inicio"`
	NaoEntendi     string `json:"nao_entendi"`
	MaxTentativas  int    `json:"max_tentativas"`
	InatividadeMin int    `json:"inatividade_min"`
	Nos            []No   `json:"nos"`
}

// No devolve o nó pelo id.
func (d DefinicaoChatbot) No(id string) (No, bool) {
	for _, n := range d.Nos {
		if n.ID == id {
			return n, true
		}
	}
	return No{}, false
}

// AntiLoop é o limite próprio de uma automação.
type AntiLoop = dominio.AntiLoop

// Limites de uma automação.
type Limites = dominio.Limites

// comNulos serializa v e acrescenta `null` para as chaves anuláveis do tipo que ficaram de fora
// por `omitempty` (o contrato TS as declara como `T | null`).
func comNulos(v any, tipo string, tabela map[string][]string) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	chaves := tabela[tipo]
	if len(chaves) == 0 {
		return b, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	for _, k := range chaves {
		if _, ok := m[k]; !ok {
			m[k] = json.RawMessage("null")
		}
	}
	return json.Marshal(m)
}
