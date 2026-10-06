package whatsapp

import (
	"io"
	"strings"
	"time"
)

// Servidores de JID.
const (
	ServidorUsuario = "s.whatsapp.net"
	ServidorGrupo   = "g.us"
	ServidorLID     = "lid"
	ServidorStatus  = "broadcast"
)

// JID identifica um usuário, grupo ou o status@broadcast. Sempre telefone quando conhecido
// (o adaptador resolve LID → telefone).
type JID struct {
	Usuario  string `json:"usuario"`
	Servidor string `json:"servidor"`
}

// JIDStatus é o chat dos status.
var JIDStatus = JID{Usuario: "status", Servidor: ServidorStatus}

// String devolve "usuario@servidor".
func (j JID) String() string {
	if j.Usuario == "" && j.Servidor == "" {
		return ""
	}
	return j.Usuario + "@" + j.Servidor
}

// Vazio indica JID não preenchido.
func (j JID) Vazio() bool { return j.Usuario == "" }

// Grupo indica JID de grupo.
func (j JID) Grupo() bool { return j.Servidor == ServidorGrupo }

// Status indica o chat de status.
func (j JID) Status() bool { return j == JIDStatus }

// Telefone devolve o E.164 quando o JID é de usuário comum.
func (j JID) Telefone() (string, bool) {
	if j.Servidor != ServidorUsuario || j.Usuario == "" {
		return "", false
	}
	return "+" + j.Usuario, true
}

// JIDDeTelefone monta o JID de usuário a partir de um E.164.
func JIDDeTelefone(e164 string) JID {
	return JID{Usuario: strings.TrimPrefix(e164, "+"), Servidor: ServidorUsuario}
}

// ParseJID interpreta "usuario@servidor" (ignora sufixo de dispositivo ":N").
func ParseJID(s string) JID {
	u, srv, ok := strings.Cut(s, "@")
	if !ok {
		return JID{}
	}
	if i := strings.IndexByte(u, ':'); i >= 0 {
		u = u[:i]
	}
	if i := strings.IndexByte(u, '.'); i >= 0 && srv == ServidorUsuario {
		u = u[:i]
	}
	return JID{Usuario: u, Servidor: srv}
}

// Tipos de EventoQR.
const (
	QRCodigo   = "codigo"
	QRSucesso  = "sucesso"
	QRExpirado = "expirado"
	QRErro     = "erro"
)

// EventoQR é um item do canal de QR.
type EventoQR struct {
	Tipo   string // codigo | sucesso | expirado | erro
	Codigo string
	Expira time.Duration
	Erro   error
}

// Estados de EventoEstado.
const (
	EstadoConectada    = "conectada"
	EstadoDesconectada = "desconectada" // logout remoto / sessão expirada
	EstadoBanida       = "banida"
	EstadoRedeCaiu     = "rede_caiu"
	EstadoRedeVoltou   = "rede_voltou"
	EstadoSubstituida  = "substituida"
)

// EventoEstado muda o estado de conexão da conta.
type EventoEstado struct {
	Estado   string
	Motivo   string
	Proprio  JID    // preenchido em "conectada"
	PushName string // nome do próprio perfil, quando conhecido
}

// Tipos de mensagem.
const (
	TipoTexto     = "texto"
	TipoImagem    = "imagem"
	TipoVideo     = "video"
	TipoAudio     = "audio"
	TipoDocumento = "documento"
	TipoFigurinha = "figurinha"
	TipoSistema   = "sistema"
)

// Evento é uma união: exatamente um campo não nulo.
type Evento struct {
	Estado    *EventoEstado
	Mensagem  *MensagemRecebida
	Recibo    *Recibo
	Reacao    *Reacao
	Edicao    *Edicao
	Revogacao *Revogacao
	Historico *LoteHistorico
	Contato   *InfoContato
}

// MensagemRecebida inclui as enviadas pelo próprio celular (DeMim) e status (Chat = status@broadcast).
type MensagemRecebida struct {
	WaID, TipoMsg, Texto string
	Chat, Remetente      JID
	NomeRemetente        string // push name do remetente, quando conhecido
	NomeChat             string // assunto do grupo, quando conhecido
	DeMim, Grupo         bool
	Midia                *MidiaInfo
	Citacao              *Citacao
	Em                   time.Time
}

// Tipos de recibo.
const (
	ReciboEntregue = "entregue"
	ReciboLido     = "lido"
	ReciboTocado   = "tocado"
)

// Recibo de entrega/leitura de mensagens enviadas por mim.
type Recibo struct {
	Tipo      string
	Chat      JID
	Remetente JID
	WaIDs     []string
	Em        time.Time
}

// Reacao recebida (emoji vazio remove).
type Reacao struct {
	Chat, Remetente JID
	DeMim           bool
	WaIDAlvo        string
	Emoji           string
	Em              time.Time
}

// Edicao de mensagem recebida.
type Edicao struct {
	Chat, Remetente JID
	WaIDAlvo        string
	NovoTexto       string
	Em              time.Time
}

// Revogacao (apagar para todos).
type Revogacao struct {
	Chat, Remetente JID
	WaIDAlvo        string
	Em              time.Time
}

// ConversaHistorico é uma conversa do history sync.
type ConversaHistorico struct {
	Chat      JID
	Nome      string
	NaoLidas  int
	Mensagens []MensagemRecebida
}

// LoteHistorico é um lote do history sync.
type LoteHistorico struct {
	Conversas []ConversaHistorico
	Progresso int  // 0..100 quando conhecido
	Final     bool // último lote
}

// InfoGrupo descreve um grupo.
type InfoGrupo struct {
	JID  JID
	Nome string
}

// InfoContato descreve um contato (agenda/push name).
type InfoContato struct {
	JID      JID
	Nome     string // nome na agenda do celular
	NomePush string
}

// MidiaInfo são os metadados de mídia recebida.
type MidiaInfo struct {
	Mimetype    string        `json:"mimetype"`
	Tamanho     int64         `json:"tamanho"`
	NomeArquivo string        `json:"nome_arquivo,omitempty"`
	DuracaoS    int           `json:"duracao_s,omitempty"`
	PTT         bool          `json:"ptt,omitempty"`
	Largura     int           `json:"largura,omitempty"`
	Altura      int           `json:"altura,omitempty"`
	Miniatura   []byte        `json:"miniatura,omitempty"`
	Chave       ChaveDownload `json:"chave_download"`
}

// ChaveDownload guarda o necessário para baixar a mídia depois (serializável em JSON).
type ChaveDownload struct {
	Tipo          string `json:"tipo"` // imagem|video|audio|documento|figurinha
	URL           string `json:"url,omitempty"`
	DirectPath    string `json:"direct_path,omitempty"`
	MediaKey      []byte `json:"media_key,omitempty"`
	FileSHA256    []byte `json:"file_sha256,omitempty"`
	FileEncSHA256 []byte `json:"file_enc_sha256,omitempty"`
	Tamanho       int64  `json:"tamanho,omitempty"`
	Mimetype      string `json:"mimetype,omitempty"`
	// Dados para pedir reenvio de mídia expirada (SendMediaRetryReceipt).
	WaID      string `json:"wa_id,omitempty"`
	Chat      string `json:"chat,omitempty"`
	Remetente string `json:"remetente,omitempty"`
	DeMim     bool   `json:"de_mim,omitempty"`
	Grupo     bool   `json:"grupo,omitempty"`
	// Ref é usado pelo cliente falso (conteúdo guardado em memória/disco).
	Ref string `json:"ref,omitempty"`
}

// Citacao referencia uma mensagem citada.
type Citacao struct {
	WaID      string
	Remetente JID
	Texto     string // resumo da citada (para exibir e para o protobuf)
}

// MidiaEnvio é uma mídia a enviar.
type MidiaEnvio struct {
	Tipo     string // imagem|video|audio|documento|figurinha
	Leitor   io.Reader
	Tamanho  int64
	Mimetype string
	NomeArq  string
	Legenda  string
	Voz      bool // PTT (áudio ogg/opus)
}

// Enviada é a confirmação do servidor.
type Enviada struct {
	WaID string
	Em   time.Time
}
