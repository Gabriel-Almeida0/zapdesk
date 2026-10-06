package dominio

import (
	"encoding/json"
	"time"
)

// Tipos da feature 002 no formato JSON do contrato (specs/002-automacoes/contracts/api-http.md).
// Gatilhos e definições ficam como JSON bruto aqui; o pacote automacoes/modelo os interpreta.

// Funil de vendas.
type Funil struct {
	ID           string    `json:"id"`
	Nome         string    `json:"nome"`
	Ordem        int       `json:"ordem"`
	Etapas       []Etapa   `json:"etapas"`
	TotalCards   int       `json:"total_cards"`
	CriadoEm     time.Time `json:"criado_em"`
	AtualizadoEm time.Time `json:"atualizado_em"`
}

// Etapa de um funil.
type Etapa struct {
	ID         string `json:"id"`
	FunilID    string `json:"funil_id"`
	Nome       string `json:"nome"`
	Cor        string `json:"cor"`
	Ordem      int    `json:"ordem"`
	TotalCards int    `json:"total_cards"`
}

// CardLead é o resumo do lead dentro do card.
type CardLead struct {
	ID       string            `json:"id"`
	Telefone string            `json:"telefone"`
	Nome     *string           `json:"nome"`
	Campos   map[string]string `json:"campos"`
}

// Card é a posição de um lead num funil.
type Card struct {
	LeadID     string     `json:"lead_id"`
	FunilID    string     `json:"funil_id"`
	EtapaID    string     `json:"etapa_id"`
	Desde      time.Time  `json:"desde"`
	Lead       CardLead   `json:"lead"`
	Etiquetas  []Etiqueta `json:"etiquetas"`
	ConversaID *string    `json:"conversa_id"`
	ContaID    *string    `json:"conta_id"`
}

// Origens de movimento no funil.
const (
	OrigemApp       = "app"
	OrigemMCPFunil  = "mcp"
	OrigemAutomacao = "automacao"
)

// MovimentoFunil é uma linha do histórico do funil.
type MovimentoFunil struct {
	ID               string    `json:"id"`
	LeadID           string    `json:"lead_id"`
	FunilID          string    `json:"funil_id"`
	EtapaOrigemID    *string   `json:"etapa_origem_id"`
	EtapaOrigemNome  *string   `json:"etapa_origem_nome"`
	EtapaDestinoID   *string   `json:"etapa_destino_id"`
	EtapaDestinoNome *string   `json:"etapa_destino_nome"`
	Origem           string    `json:"origem"`
	AutomacaoID      *string   `json:"automacao_id"`
	ExecucaoID       *string   `json:"execucao_id"`
	Em               time.Time `json:"em"`
}

// AntiLoop é o limite de mensagens automáticas por janela.
type AntiLoop struct {
	Mensagens int `json:"mensagens"`
	JanelaMin int `json:"janela_min"`
}

// Limites de uma automação.
type Limites struct {
	AntiLoop  *AntiLoop `json:"anti_loop"`
	TempoS    *int      `json:"tempo_s"`
	MemoriaMB *int      `json:"memoria_mb"`
}

// ErroDefinicao aponta um problema num campo da definição.
type ErroDefinicao struct {
	Caminho  string  `json:"caminho"`
	NoID     *string `json:"no_id"`
	AcaoID   *string `json:"acao_id"`
	Mensagem string  `json:"mensagem"`
}

// ErroCompilacao de um projeto de automação de IA.
type ErroCompilacao struct {
	Arquivo  string `json:"arquivo"`
	Linha    int    `json:"linha"`
	Coluna   int    `json:"coluna"`
	Mensagem string `json:"mensagem"`
	Tipo     string `json:"tipo"` // sintaxe | importacao | manifesto | resolucao | outro
}

// AutomacaoIA são os dados próprios de uma automação de IA.
type AutomacaoIA struct {
	Pasta                 string           `json:"pasta"`
	Permissoes            []string         `json:"permissoes"`
	Segredos              []string         `json:"segredos"`
	HashCompilado         *string          `json:"hash_compilado"`
	CompilacaoOK          bool             `json:"compilacao_ok"`
	ErrosCompilacao       []ErroCompilacao `json:"erros_compilacao"`
	CompiladoEm           *time.Time       `json:"compilado_em"`
	RodandoVersaoAnterior bool             `json:"rodando_versao_anterior"`
}

// Estatisticas24h de uma automação.
type Estatisticas24h struct {
	OK             int  `json:"ok"`
	Erro           int  `json:"erro"`
	Abortada       int  `json:"abortada"`
	DuracaoMediaMs *int `json:"duracao_media_ms"`
}

// Automacao (fluxo, chatbot ou IA).
type Automacao struct {
	ID               string          `json:"id"`
	Tipo             string          `json:"tipo"`
	Nome             string          `json:"nome"`
	Descricao        *string         `json:"descricao"`
	Ativa            bool            `json:"ativa"`
	Contas           []string        `json:"contas"`
	IncluirGrupos    bool            `json:"incluir_grupos"`
	Prioridade       int             `json:"prioridade"`
	ContaEnvioID     *string         `json:"conta_envio_id"`
	Gatilhos         json.RawMessage `json:"gatilhos"`
	Definicao        json.RawMessage `json:"definicao"`
	Limites          Limites         `json:"limites"`
	Versao           int             `json:"versao"`
	IA               *AutomacaoIA    `json:"ia"`
	Avisos           []ErroDefinicao `json:"avisos"`
	ErrosSeguidos    int             `json:"erros_seguidos"`
	DesativadaMotivo *string         `json:"desativada_motivo"`
	Estatisticas24h  Estatisticas24h `json:"estatisticas_24h"`
	SessoesAtivas    int             `json:"sessoes_ativas"`
	CriadaEm         time.Time       `json:"criada_em"`
	AtualizadaEm     time.Time       `json:"atualizada_em"`

	// Internos (IA).
	Permissoes      []string         `json:"-"`
	Segredos        []string         `json:"-"`
	HashFontes      *string          `json:"-"`
	HashCompilado   *string          `json:"-"`
	ErrosCompilacao []ErroCompilacao `json:"-"`
	CompiladoEm     *time.Time       `json:"-"`
}

// Estados de execução.
const (
	ExecNaFila     = "na_fila"
	ExecRodando    = "rodando"
	ExecAguardando = "aguardando"
	ExecOK         = "ok"
	ExecErro       = "erro"
	ExecSimulacao  = "simulacao"
	ExecAbortada   = "abortada"
)

// FinalExec indica se o estado é final.
func FinalExec(e string) bool {
	return e == ExecOK || e == ExecErro || e == ExecSimulacao || e == ExecAbortada
}

// Resultados de ação.
const (
	AcaoOK        = "ok"
	AcaoFalhou    = "falhou"
	AcaoBloqueada = "bloqueada"
	AcaoSimulada  = "simulada"
)

// AcaoRegistrada numa execução.
type AcaoRegistrada struct {
	Tipo      string    `json:"tipo"`
	Alvo      *string   `json:"alvo"`
	Resultado string    `json:"resultado"`
	Detalhe   *string   `json:"detalhe"`
	Em        time.Time `json:"em"`
}

// TokensModelo por modelo.
type TokensModelo struct {
	Entrada  int `json:"entrada"`
	Saida    int `json:"saida"`
	Chamadas int `json:"chamadas"`
}

// Tokens somados na execução.
type Tokens struct {
	Entrada   int                     `json:"entrada"`
	Saida     int                     `json:"saida"`
	PorModelo map[string]TokensModelo `json:"por_modelo"`
}

// Somar acrescenta uma chamada.
func (t *Tokens) Somar(modelo string, entrada, saida int) {
	if t.PorModelo == nil {
		t.PorModelo = map[string]TokensModelo{}
	}
	t.Entrada += entrada
	t.Saida += saida
	m := t.PorModelo[modelo]
	m.Entrada += entrada
	m.Saida += saida
	m.Chamadas++
	t.PorModelo[modelo] = m
}

// GatilhoExecucao é o fato que iniciou a execução.
type GatilhoExecucao struct {
	Tipo  string         `json:"tipo"`
	Dados map[string]any `json:"dados"`
}

// Execucao de uma automação.
type Execucao struct {
	ID               string           `json:"id"`
	AutomacaoID      string           `json:"automacao_id"`
	AutomacaoNome    string           `json:"automacao_nome"`
	TipoAutomacao    string           `json:"tipo_automacao"`
	AutomacaoVersao  int              `json:"automacao_versao"`
	Gatilho          GatilhoExecucao  `json:"gatilho"`
	Origem           string           `json:"origem"`
	OrigemExecucaoID *string          `json:"origem_execucao_id"`
	ContaID          *string          `json:"conta_id"`
	ConversaID       *string          `json:"conversa_id"`
	ContatoID        *string          `json:"contato_id"`
	LeadID           *string          `json:"lead_id"`
	Estado           string           `json:"estado"`
	Simulacao        bool             `json:"simulacao"`
	Motivo           *string          `json:"motivo"`
	Erro             *string          `json:"erro"`
	Acoes            []AcaoRegistrada `json:"acoes"`
	Tokens           Tokens           `json:"tokens"`
	Retorno          json.RawMessage  `json:"retorno"`
	RetomarEm        *time.Time       `json:"retomar_em"`
	IniciadaEm       time.Time        `json:"iniciada_em"`
	FinalizadaEm     *time.Time       `json:"finalizada_em"`
	DuracaoMs        *int64           `json:"duracao_ms"`

	// Internos.
	Cadeia     []string `json:"-"`
	PassoAtual *int     `json:"-"`
}

// ExecucaoDetalhe = Execucao + log, stack e variáveis.
type ExecucaoDetalhe struct {
	Execucao
	Log         string         `json:"log"`
	LogTruncado bool           `json:"log_truncado"`
	ErroStack   *string        `json:"erro_stack"`
	Variaveis   map[string]any `json:"variaveis"`
}

// Estados de sessão de chatbot.
const (
	SessaoAtiva     = "ativa"
	SessaoConcluida = "concluida"
	SessaoHumano    = "humano"
	SessaoExpirada  = "expirada"
	SessaoAbortada  = "abortada"
)

// SessaoChatbot numa conversa.
type SessaoChatbot struct {
	ID            string            `json:"id"`
	AutomacaoID   string            `json:"automacao_id"`
	AutomacaoNome string            `json:"automacao_nome"`
	ConversaID    string            `json:"conversa_id"`
	Versao        int               `json:"versao"`
	NoAtual       string            `json:"no_atual"`
	Variaveis     map[string]string `json:"variaveis"`
	Tentativas    int               `json:"tentativas"`
	Estado        string            `json:"estado"`
	Motivo        *string           `json:"motivo"`
	ExpiraEm      time.Time         `json:"expira_em"`
	IniciadaEm    time.Time         `json:"iniciada_em"`
	AtualizadaEm  time.Time         `json:"atualizada_em"`
	FinalizadaEm  *time.Time        `json:"finalizada_em"`

	// Interno: cópia congelada do grafo.
	Definicao json.RawMessage `json:"-"`
	ContaID   string          `json:"-"`
}

// Motivos de pausa de conversa.
const (
	PausaHumano   = "humano"
	PausaAntiLoop = "anti_loop"
	PausaManual   = "manual"
)

// Pausa de automações numa conversa.
type Pausa struct {
	ConversaID  string     `json:"conversa_id"`
	Motivo      string     `json:"motivo"`
	Ate         *time.Time `json:"ate"`
	AutomacaoID *string    `json:"automacao_id"`
	CriadaEm    time.Time  `json:"criada_em"`
}

// EstadoConversaAutomacoes de GET /conversas/{id}/automacoes.
type EstadoConversaAutomacoes struct {
	ConversaID string         `json:"conversa_id"`
	Pausa      *Pausa         `json:"pausa"`
	Sessao     *SessaoChatbot `json:"sessao"`
	PausaGeral bool           `json:"pausa_geral"`
}

// Tipos de espera.
const (
	EsperaAguardar      = "aguardar"
	EsperaSemResposta   = "sem_resposta"
	EsperaAgendamento   = "agendamento"
	EsperaAgendar       = "agendar"
	EsperaExpirarSessao = "expirar_sessao"
)

// Espera agendada (interna).
type Espera struct {
	ID          string
	Tipo        string
	AutomacaoID string
	ExecucaoID  *string
	SessaoID    *string
	ConversaID  *string
	Referencia  *string
	Chave       *string
	RetomarEm   time.Time
	Dados       json.RawMessage
	CriadaEm    time.Time
}

// ConfiguracaoAutomacoes (Ajustes → Automações).
type ConfiguracaoAutomacoes struct {
	AntiLoopMensagens     int  `json:"anti_loop_mensagens"`
	AntiLoopJanelaMin     int  `json:"anti_loop_janela_min"`
	PausaAntiLoopMin      int  `json:"pausa_anti_loop_min"`
	PausaHumanaMin        int  `json:"pausa_humana_min"`
	PrimeirosContatosHora int  `json:"primeiros_contatos_hora"`
	TempoIAS              int  `json:"tempo_ia_s"`
	MemoriaIAMB           int  `json:"memoria_ia_mb"`
	ProcessosIAMax        int  `json:"processos_ia_max"`
	OciosidadeIAMin       int  `json:"ociosidade_ia_min"`
	PausaGeral            bool `json:"pausa_geral"`
}

// ConfiguracaoPadrao de data-model.md (anti-loop 10 em 10 min — decisão do orquestrador).
func ConfiguracaoPadrao() ConfiguracaoAutomacoes {
	return ConfiguracaoAutomacoes{AntiLoopMensagens: 10, AntiLoopJanelaMin: 10, PausaAntiLoopMin: 60, PausaHumanaMin: 30,
		PrimeirosContatosHora: 20, TempoIAS: 60, MemoriaIAMB: 256, ProcessosIAMax: 4, OciosidadeIAMin: 5}
}

// Notificacao ao operador (evento WS "notificacao").
type Notificacao struct {
	Titulo      string  `json:"titulo"`
	Corpo       string  `json:"corpo"`
	AutomacaoID *string `json:"automacao_id"`
	ConversaID  *string `json:"conversa_id"`
	Tipo        string  `json:"tipo"` // anti_loop | humano | desativada | acao | erro
}
