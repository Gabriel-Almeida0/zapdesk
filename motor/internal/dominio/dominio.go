// Pacote dominio define as entidades no formato JSON do contrato (contracts/api-http.md ›
// Tipos). Repositórios as preenchem; a API as serializa.
package dominio

import "time"

// Estados de conta.
const (
	ContaConectando   = "conectando"
	ContaConectada    = "conectada"
	ContaDesconectada = "desconectada"
	ContaBanida       = "banida"
)

// Conta do WhatsApp.
type Conta struct {
	ID            string    `json:"id"`
	Nome          string    `json:"nome"`
	Telefone      *string   `json:"telefone"`
	JID           *string   `json:"jid"`
	Estado        string    `json:"estado"`
	Online        bool      `json:"online"`
	Sincronizando bool      `json:"sincronizando"`
	CriadaEm      time.Time `json:"criada_em"`
}

// Etiqueta global.
type Etiqueta struct {
	ID            string `json:"id"`
	Nome          string `json:"nome"`
	Cor           string `json:"cor"`
	TotalContatos int    `json:"total_contatos"`
}

// Conversa individual ou de grupo.
type Conversa struct {
	ID                   string     `json:"id"`
	ContaID              string     `json:"conta_id"`
	JID                  string     `json:"jid"`
	Tipo                 string     `json:"tipo"`
	Nome                 string     `json:"nome"`
	Telefone             *string    `json:"telefone"`
	ContatoID            *string    `json:"contato_id"`
	NaoLidas             int        `json:"nao_lidas"`
	UltimaMensagemEm     *time.Time `json:"ultima_mensagem_em"`
	UltimaMensagemResumo *string    `json:"ultima_mensagem_resumo"`
	Etiquetas            []Etiqueta `json:"etiquetas"`
}

// Midia de mensagem ou status.
type Midia struct {
	Mimetype     string  `json:"mimetype"`
	Tamanho      int64   `json:"tamanho"`
	NomeArquivo  *string `json:"nome_arquivo"`
	DuracaoS     *int    `json:"duracao_s"`
	PTT          bool    `json:"ptt"`
	Largura      *int    `json:"largura"`
	Altura       *int    `json:"altura"`
	MiniaturaB64 *string `json:"miniatura_b64"`
	Baixada      bool    `json:"baixada"`
	URL          string  `json:"url"`
}

// Citacao exibida na bolha.
type Citacao struct {
	WaID          string  `json:"wa_id"`
	Resumo        string  `json:"resumo"`
	RemetenteNome *string `json:"remetente_nome"`
}

// Reacao a uma mensagem.
type Reacao struct {
	RemetenteJID string `json:"remetente_jid"`
	Emoji        string `json:"emoji"`
	DeMim        bool   `json:"de_mim"`
}

// Estados de mensagem.
const (
	MsgPendente = "pendente"
	MsgEnviada  = "enviada"
	MsgEntregue = "entregue"
	MsgLida     = "lida"
	MsgFalhou   = "falhou"
	MsgRecebida = "recebida"
)

// Mensagem de uma conversa.
type Mensagem struct {
	ID            string    `json:"id"`
	ContaID       string    `json:"conta_id"`
	ConversaID    string    `json:"conversa_id"`
	WaID          string    `json:"wa_id"`
	RemetenteJID  string    `json:"remetente_jid"`
	RemetenteNome *string   `json:"remetente_nome"`
	DeMim         bool      `json:"de_mim"`
	Tipo          string    `json:"tipo"`
	Texto         *string   `json:"texto"`
	Midia         *Midia    `json:"midia"`
	Citacao       *Citacao  `json:"citacao"`
	Reacoes       []Reacao  `json:"reacoes"`
	Editada       bool      `json:"editada"`
	Apagada       bool      `json:"apagada"`
	Estado        string    `json:"estado"`
	Erro          *string   `json:"erro"`
	DisparoID     *string   `json:"disparo_id"`
	AutomacaoID   *string   `json:"automacao_id"`
	EnviadaEm     time.Time `json:"enviada_em"`
	PodeEditar    bool      `json:"pode_editar"`
	PodeApagar    bool      `json:"pode_apagar"`
}

// LeadResumo aparece dentro do Contato.
type LeadResumo struct {
	ID          string    `json:"id"`
	Origem      string    `json:"origem"`
	ImportadoEm time.Time `json:"importado_em"`
}

// Contato de uma conta.
type Contato struct {
	ID         string      `json:"id"`
	ContaID    string      `json:"conta_id"`
	JID        string      `json:"jid"`
	Telefone   *string     `json:"telefone"`
	Nome       *string     `json:"nome"`
	NomePush   *string     `json:"nome_push"`
	Notas      *string     `json:"notas"`
	Etiquetas  []Etiqueta  `json:"etiquetas"`
	Lead       *LeadResumo `json:"lead"`
	ConversaID *string     `json:"conversa_id"`
}

// Status publicado por um contato.
type Status struct {
	ID          string    `json:"id"`
	ContaID     string    `json:"conta_id"`
	ContatoJID  string    `json:"contato_jid"`
	ContatoNome *string   `json:"contato_nome"`
	Tipo        string    `json:"tipo"`
	Texto       *string   `json:"texto"`
	Midia       *Midia    `json:"midia"`
	PublicadoEm time.Time `json:"publicado_em"`
}

// StatusPorContato agrupa status.
type StatusPorContato struct {
	ContatoJID  string   `json:"contato_jid"`
	ContatoNome *string  `json:"contato_nome"`
	Itens       []Status `json:"itens"`
}

// Origens de lead.
const (
	OrigemCSV      = "csv"
	OrigemColado   = "colado"
	OrigemContatos = "contatos"
	OrigemMCP      = "mcp"
)

// Lead da base de disparos.
type Lead struct {
	ID              string            `json:"id"`
	Telefone        string            `json:"telefone"`
	Nome            *string           `json:"nome"`
	Campos          map[string]string `json:"campos"`
	Origem          string            `json:"origem"`
	TemWhatsApp     *bool             `json:"tem_whatsapp"`
	ImportadoEm     time.Time         `json:"importado_em"`
	UltimoDisparoEm *time.Time        `json:"ultimo_disparo_em"`
}

// Arquivo anexado.
type Arquivo struct {
	ID        string `json:"id"`
	Nome      string `json:"nome"`
	Mimetype  string `json:"mimetype"`
	Tamanho   int64  `json:"tamanho"`
	TipoMidia string `json:"tipo_midia"`
	URL       string `json:"url"`
	Caminho   string `json:"-"`
}

// Template de mensagem.
type Template struct {
	ID           string    `json:"id"`
	Nome         string    `json:"nome"`
	Texto        string    `json:"texto"`
	Variaveis    []string  `json:"variaveis"`
	Arquivo      *Arquivo  `json:"arquivo"`
	CriadoEm     time.Time `json:"criado_em"`
	AtualizadoEm time.Time `json:"atualizado_em"`
	ArquivoID    *string   `json:"-"`
}

// Ritmo de um disparo.
type Ritmo struct {
	IntervaloMinS int  `json:"intervalo_min_s"`
	IntervaloMaxS int  `json:"intervalo_max_s"`
	LimitePorHora *int `json:"limite_por_hora"`
	LimitePorDia  *int `json:"limite_por_dia"`
	PausaACada    *int `json:"pausa_a_cada"`
	PausaDuracaoS *int `json:"pausa_duracao_s"`
}

// Janela de envio "HH:MM".
type Janela struct {
	Inicio string `json:"inicio"`
	Fim    string `json:"fim"`
}

// Contadores de destinatários por estado.
type Contadores struct {
	Total     int `json:"total"`
	Pendente  int `json:"pendente"`
	Enviando  int `json:"enviando"`
	Enviado   int `json:"enviado"`
	Entregue  int `json:"entregue"`
	Lido      int `json:"lido"`
	Respondeu int `json:"respondeu"`
	Falhou    int `json:"falhou"`
}

// Estados de disparo.
const (
	DisparoRascunho     = "rascunho"
	DisparoAgendado     = "agendado"
	DisparoEnviando     = "enviando"
	DisparoForaDaJanela = "fora_da_janela"
	DisparoPausado      = "pausado"
	DisparoConcluido    = "concluido"
	DisparoCancelado    = "cancelado"
)

// Motivos de pausa.
const (
	PausaUsuario           = "usuario"
	PausaAppFechado        = "app_fechado"
	PausaMotorReiniciado   = "motor_reiniciado"
	PausaContaDesconectada = "conta_desconectada"
	PausaContaBanida       = "conta_banida"
	PausaFalhasSeguidas    = "falhas_seguidas"
)

// Disparo em massa.
type Disparo struct {
	ID                  string               `json:"id"`
	ContaID             string               `json:"conta_id"`
	Nome                string               `json:"nome"`
	Mensagem            string               `json:"mensagem"`
	Arquivo             *Arquivo             `json:"arquivo"`
	Ritmo               Ritmo                `json:"ritmo"`
	InicioEm            *time.Time           `json:"inicio_em"`
	Janela              *Janela              `json:"janela"`
	FalhasSeguidasMax   int                  `json:"falhas_seguidas_max"`
	ValoresPadrao       map[string]string    `json:"valores_padrao"`
	Estado              string               `json:"estado"`
	NaFila              bool                 `json:"na_fila"`
	MotivoPausa         *string              `json:"motivo_pausa"`
	Origem              string               `json:"origem"`
	Contadores          Contadores           `json:"contadores"`
	ProximoEnvioEm      *time.Time           `json:"proximo_envio_em"`
	EstimativaTerminoEm *time.Time           `json:"estimativa_termino_em"`
	AvisoRitmoAgressivo bool                 `json:"aviso_ritmo_agressivo"`
	CriadoEm            time.Time            `json:"criado_em"`
	IniciadoEm          *time.Time           `json:"iniciado_em"`
	ConcluidoEm         *time.Time           `json:"concluido_em"`
	CanceladoEm         *time.Time           `json:"cancelado_em"`
	RelatorioImportacao *RelatorioImportacao `json:"relatorio_importacao,omitempty"`

	// Internos (não vão para o JSON).
	ArquivoID          *string    `json:"-"`
	FalhasSeguidas     int        `json:"-"`
	UltimoEnvioEm      *time.Time `json:"-"`
	EnviadosDesdePausa int        `json:"-"`
	FilaDesde          *time.Time `json:"-"`
}

// Estados de destinatário.
const (
	DestPendente  = "pendente"
	DestEnviando  = "enviando"
	DestEnviado   = "enviado"
	DestEntregue  = "entregue"
	DestLido      = "lido"
	DestFalhou    = "falhou"
	DestRespondeu = "respondeu"
)

// Destinatario de um disparo.
type Destinatario struct {
	ID           string            `json:"id"`
	DisparoID    string            `json:"disparo_id"`
	LeadID       string            `json:"lead_id"`
	Ordem        int               `json:"ordem"`
	Telefone     string            `json:"telefone"`
	Nome         *string           `json:"nome"`
	Variaveis    map[string]string `json:"variaveis"`
	Estado       string            `json:"estado"`
	MotivoFalha  *string           `json:"motivo_falha"`
	MensagemWaID *string           `json:"mensagem_wa_id"`
	EnviandoEm   *time.Time        `json:"enviando_em"`
	EnviadoEm    *time.Time        `json:"enviado_em"`
	EntregueEm   *time.Time        `json:"entregue_em"`
	LidoEm       *time.Time        `json:"lido_em"`
	RespondeuEm  *time.Time        `json:"respondeu_em"`
	FalhouEm     *time.Time        `json:"falhou_em"`
	JID          *string           `json:"-"`
}

// RelatorioImportacao (retorno da importação).
type RelatorioImportacao struct {
	TotalLinhas           int               `json:"total_linhas"`
	TotalNovos            int               `json:"total_novos"`
	TotalJaExistentes     int               `json:"total_ja_existentes"`
	TotalInvalidos        int               `json:"total_invalidos"`
	TotalDuplicadosNoLote int               `json:"total_duplicados_no_lote"`
	Novos                 []ItemNovo        `json:"novos"`
	JaExistentes          []ItemJaExistente `json:"ja_existentes"`
	Invalidos             []ItemInvalido    `json:"invalidos"`
	DuplicadosNoLote      []ItemDuplicado   `json:"duplicados_no_lote"`
	LeadIDs               []string          `json:"lead_ids"`
}

// ItemNovo do relatório.
type ItemNovo struct {
	Linha    int    `json:"linha"`
	LeadID   string `json:"lead_id"`
	Telefone string `json:"telefone"`
}

// ItemJaExistente do relatório.
type ItemJaExistente struct {
	Linha             int       `json:"linha"`
	LeadID            string    `json:"lead_id"`
	Telefone          string    `json:"telefone"`
	ImportadoEm       time.Time `json:"importado_em"`
	CamposPreenchidos []string  `json:"campos_preenchidos"`
}

// ItemInvalido do relatório.
type ItemInvalido struct {
	Linha  int    `json:"linha"`
	Valor  string `json:"valor"`
	Motivo string `json:"motivo"`
}

// ItemDuplicado do relatório.
type ItemDuplicado struct {
	Linha         int    `json:"linha"`
	Telefone      string `json:"telefone"`
	PrimeiraLinha int    `json:"primeira_linha"`
}

// Str devolve um ponteiro para s (nil se vazio).
func Str(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
