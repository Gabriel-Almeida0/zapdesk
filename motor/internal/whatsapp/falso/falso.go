// Pacote falso implementa um ClienteWhatsApp em memória, determinístico (Constituição IV), para
// rodar o motor, a API e o MCP sem conta real. O pareamento e o registro de tudo que foi
// "enviado" ficam em <pasta-dados>/falso/ para sobreviver a reinícios do motor (testes de
// sessão persistida e de idempotência dos disparos).
package falso

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/relogio"
	"zapdesk/motor/internal/whatsapp"
)

// ErrContaNaoAberta indica que não há cliente aberto para a conta.
var ErrContaNaoAberta = errors.New("conta não está aberta no cliente falso")

// ErrSemQR indica que a conta não está esperando leitura de QR.
var ErrSemQR = errors.New("a conta não está aguardando QR")

// Enviada é um registro de algo "enviado" pelo cliente falso (GET /v1/falso/enviadas).
type Enviada struct {
	ContaID  string    `json:"conta_id"`
	WaID     string    `json:"wa_id"`
	Tipo     string    `json:"tipo"` // texto|imagem|video|audio|documento|figurinha|reacao|edicao|apagar
	Para     string    `json:"para"` // JID
	Telefone string    `json:"telefone,omitempty"`
	Texto    string    `json:"texto,omitempty"`
	Mimetype string    `json:"mimetype,omitempty"`
	Tamanho  int64     `json:"tamanho,omitempty"`
	NomeArq  string    `json:"nome_arquivo,omitempty"`
	Voz      bool      `json:"voz,omitempty"`
	Citacao  string    `json:"citacao_wa_id,omitempty"`
	Alvo     string    `json:"alvo_wa_id,omitempty"`
	Emoji    string    `json:"emoji,omitempty"`
	Em       time.Time `json:"em"`
}

type sessao struct {
	Telefone string `json:"telefone"`
	Nome     string `json:"nome"`
}

// Controle é a Fabrica falsa e o ponto de controle usado pelas rotas /v1/falso/*.
type Controle struct {
	mu          sync.Mutex
	pasta       string
	relogio     relogio.Relogio
	sessoes     map[string]sessao
	clientes    map[string]*Cliente
	semWhatsApp map[string]bool
	falhas      map[string]string
	enviadas    []Enviada
	porWaID     map[string]int
	grupos      map[string]map[string]string // conta → jid → nome
	contatos    map[string]map[string]whatsapp.InfoContato
	qrSeq       int
}

// Novo cria o controle com persistência em <pastaDados>/falso.
func Novo(pastaDados string, r relogio.Relogio) (*Controle, error) {
	if r == nil {
		r = relogio.Real{}
	}
	c := &Controle{
		pasta:       filepath.Join(pastaDados, "falso"),
		relogio:     r,
		sessoes:     map[string]sessao{},
		clientes:    map[string]*Cliente{},
		semWhatsApp: map[string]bool{},
		falhas:      map[string]string{},
		porWaID:     map[string]int{},
		grupos:      map[string]map[string]string{},
		contatos:    map[string]map[string]whatsapp.InfoContato{},
	}
	if err := os.MkdirAll(filepath.Join(c.pasta, "midia"), 0o700); err != nil {
		return nil, err
	}
	if dados, err := os.ReadFile(c.caminhoSessoes()); err == nil {
		json.Unmarshal(dados, &c.sessoes)
	}
	if arq, err := os.Open(c.caminhoEnviadas()); err == nil {
		sc := bufio.NewScanner(arq)
		sc.Buffer(make([]byte, 1<<20), 1<<24)
		for sc.Scan() {
			var e Enviada
			if json.Unmarshal(sc.Bytes(), &e) == nil {
				c.porWaID[e.WaID] = len(c.enviadas)
				c.enviadas = append(c.enviadas, e)
			}
		}
		arq.Close()
	}
	return c, nil
}

func (c *Controle) caminhoSessoes() string  { return filepath.Join(c.pasta, "sessoes.json") }
func (c *Controle) caminhoEnviadas() string { return filepath.Join(c.pasta, "enviadas.jsonl") }

func (c *Controle) salvarSessoesSemTrava() {
	dados, _ := json.MarshalIndent(c.sessoes, "", "  ")
	tmp := c.caminhoSessoes() + ".tmp"
	if os.WriteFile(tmp, dados, 0o600) == nil {
		os.Rename(tmp, c.caminhoSessoes())
	}
}

func (c *Controle) registrarSemTrava(e Enviada) {
	c.porWaID[e.WaID] = len(c.enviadas)
	c.enviadas = append(c.enviadas, e)
	linha, _ := json.Marshal(e)
	arq, err := os.OpenFile(c.caminhoEnviadas(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err == nil {
		arq.Write(append(linha, '\n'))
		arq.Sync()
		arq.Close()
	}
}

// Abrir implementa whatsapp.Fabrica.
func (c *Controle) Abrir(_ context.Context, contaID string) (whatsapp.Cliente, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ant, ok := c.clientes[contaID]; ok {
		ant.fecharSemTravaControle()
	}
	cli := &Cliente{ctrl: c, contaID: contaID, eventos: make(chan whatsapp.Evento, 4096)}
	c.clientes[contaID] = cli
	return cli, nil
}

// Remover implementa whatsapp.Fabrica (logout + apaga sessão).
func (c *Controle) Remover(_ context.Context, contaID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cli, ok := c.clientes[contaID]; ok {
		cli.fecharSemTravaControle()
		delete(c.clientes, contaID)
	}
	delete(c.sessoes, contaID)
	c.salvarSessoesSemTrava()
	return nil
}

func (c *Controle) cliente(contaID string) (*Cliente, error) {
	cli, ok := c.clientes[contaID]
	if !ok || cli.fechado {
		return nil, ErrContaNaoAberta
	}
	return cli, nil
}

func (c *Controle) novoWaID() string { return "FALSO" + ids.NovoEm(c.relogio.Agora()) }

// ---------------------------------------------------------------------------
// Controle (rotas /v1/falso/*)
// ---------------------------------------------------------------------------

// EscanearQR simula a leitura do QR: a conta fica pareada e conectada.
func (c *Controle) EscanearQR(contaID, telefone, nome string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cli, err := c.cliente(contaID)
	if err != nil {
		return err
	}
	if cli.qr == nil {
		return ErrSemQR
	}
	if nome == "" {
		nome = "Conta falsa"
	}
	c.sessoes[contaID] = sessao{Telefone: telefone, Nome: nome}
	c.salvarSessoesSemTrava()
	select {
	case cli.qr <- whatsapp.EventoQR{Tipo: whatsapp.QRSucesso}:
		cli.emitidos.Add(1)
	default:
	}
	close(cli.qr)
	cli.qr = nil
	cli.conectado = true
	cli.emitirSemTrava(whatsapp.Evento{Estado: &whatsapp.EventoEstado{
		Estado: whatsapp.EstadoConectada, Proprio: whatsapp.JIDDeTelefone(telefone), PushName: nome}})
	return nil
}

// ExpirarQR simula QR expirado sem leitura.
func (c *Controle) ExpirarQR(contaID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cli, err := c.cliente(contaID)
	if err != nil {
		return err
	}
	if cli.qr == nil {
		return ErrSemQR
	}
	select {
	case cli.qr <- whatsapp.EventoQR{Tipo: whatsapp.QRExpirado}:
		cli.emitidos.Add(1)
	default:
	}
	close(cli.qr)
	cli.qr = nil
	return nil
}

// Recebida descreve uma mensagem injetada.
type Recebida struct {
	De             string // E.164 do remetente
	Nome           string // push name
	Texto          string
	GrupoJID       string
	GrupoNome      string
	Tipo           string // texto (padrão)|imagem|video|audio|documento|figurinha
	DeMim          bool
	FalharDownload bool
	CitarWaID      string
	WaID           string
	Em             time.Time
}

func (c *Controle) montarMensagem(contaID string, r Recebida) whatsapp.MensagemRecebida {
	remetente := whatsapp.JIDDeTelefone(r.De)
	chat := remetente
	grupo := false
	if r.GrupoJID != "" {
		chat = whatsapp.ParseJID(r.GrupoJID)
		if chat.Servidor == "" {
			chat = whatsapp.JID{Usuario: r.GrupoJID, Servidor: whatsapp.ServidorGrupo}
		}
		grupo = true
		if c.grupos[contaID] == nil {
			c.grupos[contaID] = map[string]string{}
		}
		nome := r.GrupoNome
		if nome == "" {
			nome = c.grupos[contaID][chat.String()]
		}
		if nome == "" {
			nome = "Grupo " + chat.Usuario
		}
		c.grupos[contaID][chat.String()] = nome
	}
	if r.DeMim {
		if s, ok := c.sessoes[contaID]; ok {
			remetente = whatsapp.JIDDeTelefone(s.Telefone)
		}
	}
	tipo := r.Tipo
	if tipo == "" {
		tipo = whatsapp.TipoTexto
	}
	waID := r.WaID
	if waID == "" {
		waID = c.novoWaID()
	}
	em := r.Em
	if em.IsZero() {
		em = c.relogio.Agora()
	}
	m := whatsapp.MensagemRecebida{
		WaID: waID, TipoMsg: tipo, Texto: r.Texto, Chat: chat, Remetente: remetente,
		NomeRemetente: r.Nome, DeMim: r.DeMim, Grupo: grupo, Em: em,
	}
	if grupo {
		m.NomeChat = c.grupos[contaID][chat.String()]
	}
	if r.CitarWaID != "" {
		m.Citacao = &whatsapp.Citacao{WaID: r.CitarWaID}
	}
	if tipo != whatsapp.TipoTexto && tipo != whatsapp.TipoSistema {
		mime, nome, dados := midiaFalsa(tipo)
		ref := ""
		if !r.FalharDownload {
			ref = ids.Novo()
			os.WriteFile(filepath.Join(c.pasta, "midia", ref), dados, 0o600)
		}
		m.Midia = &whatsapp.MidiaInfo{
			Mimetype: mime, Tamanho: int64(len(dados)), NomeArquivo: nome, PTT: tipo == whatsapp.TipoAudio,
			Chave: whatsapp.ChaveDownload{Tipo: tipo, Ref: ref, Mimetype: mime, Tamanho: int64(len(dados))},
		}
	}
	if r.Nome != "" && !r.DeMim {
		if c.contatos[contaID] == nil {
			c.contatos[contaID] = map[string]whatsapp.InfoContato{}
		}
		c.contatos[contaID][remetente.String()] = whatsapp.InfoContato{JID: remetente, NomePush: r.Nome}
	}
	return m
}

// PNG 1×1 transparente.
var pngMinimo = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
	0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82}

// WebP 1×1 válido (VP8L): a figurinha falsa precisa decodificar no navegador, senão o app mostra
// "Não foi possível baixar." para toda figurinha injetada.
var webpMinimo = []byte{0x52, 0x49, 0x46, 0x46, 0x1a, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50, 0x56, 0x50, 0x38, 0x4c,
	0x0d, 0x00, 0x00, 0x00, 0x2f, 0x00, 0x00, 0x00, 0x10, 0x07, 0x10, 0x11, 0x11, 0x88, 0x88, 0xfe, 0x07, 0x00}

func midiaFalsa(tipo string) (mime, nome string, dados []byte) {
	switch tipo {
	case whatsapp.TipoImagem:
		return "image/png", "", pngMinimo
	case whatsapp.TipoVideo:
		return "video/mp4", "", []byte("video-falso")
	case whatsapp.TipoAudio:
		return "audio/ogg; codecs=opus", "", []byte("OggS-audio-falso")
	case whatsapp.TipoFigurinha:
		return "image/webp", "", webpMinimo
	default:
		return "application/pdf", "documento.pdf", []byte("%PDF-1.4 falso")
	}
}

// InjetarMensagem entrega uma mensagem recebida à conta; devolve o wa_id.
func (c *Controle) InjetarMensagem(contaID string, r Recebida) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cli, err := c.cliente(contaID)
	if err != nil {
		return "", err
	}
	m := c.montarMensagem(contaID, r)
	cli.emitirSemTrava(whatsapp.Evento{Mensagem: &m})
	return m.WaID, nil
}

// InjetarStatus simula um status publicado por um contato.
func (c *Controle) InjetarStatus(contaID, de, texto string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cli, err := c.cliente(contaID)
	if err != nil {
		return "", err
	}
	m := c.montarMensagem(contaID, Recebida{De: de, Texto: texto})
	m.Chat = whatsapp.JIDStatus
	cli.emitirSemTrava(whatsapp.Evento{Mensagem: &m})
	return m.WaID, nil
}

// InjetarRecibo entrega um recibo (entregue|lido) de uma mensagem enviada.
func (c *Controle) InjetarRecibo(contaID, waID, tipo string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cli, err := c.cliente(contaID)
	if err != nil {
		return err
	}
	chat := whatsapp.JID{}
	if i, ok := c.porWaID[waID]; ok {
		chat = whatsapp.ParseJID(c.enviadas[i].Para)
	}
	cli.emitirSemTrava(whatsapp.Evento{Recibo: &whatsapp.Recibo{
		Tipo: tipo, Chat: chat, Remetente: chat, WaIDs: []string{waID}, Em: c.relogio.Agora()}})
	return nil
}

// Estados aceitos por InjetarEstado.
const (
	EventoQuedaRede  = "queda_rede"
	EventoReconectou = "reconectou"
	EventoLogout     = "logout"
	EventoBan        = "ban"
)

// InjetarEstado simula eventos de conexão.
func (c *Controle) InjetarEstado(contaID, evento string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cli, err := c.cliente(contaID)
	if err != nil {
		return err
	}
	var e whatsapp.EventoEstado
	switch evento {
	case EventoQuedaRede:
		cli.conectado = false
		e.Estado = whatsapp.EstadoRedeCaiu
	case EventoReconectou:
		cli.conectado = true
		e.Estado = whatsapp.EstadoRedeVoltou
	case EventoLogout:
		cli.conectado = false
		delete(c.sessoes, contaID)
		c.salvarSessoesSemTrava()
		e.Estado = whatsapp.EstadoDesconectada
		e.Motivo = "logout"
	case EventoBan:
		cli.conectado = false
		cli.banido = true
		e.Estado = whatsapp.EstadoBanida
		e.Motivo = "ban"
	default:
		return fmt.Errorf("evento desconhecido: %s", evento)
	}
	cli.emitirSemTrava(whatsapp.Evento{Estado: &e})
	return nil
}

// MensagemHistorico é uma mensagem do corpo de /v1/falso/contas/{id}/historico.
type MensagemHistorico struct {
	WaID  string    `json:"wa_id"`
	De    string    `json:"de"`
	Nome  string    `json:"nome"`
	DeMim bool      `json:"de_mim"`
	Texto string    `json:"texto"`
	Tipo  string    `json:"tipo"`
	Em    time.Time `json:"em"`
}

// ConversaHistorico é uma conversa do corpo de /v1/falso/contas/{id}/historico.
type ConversaHistorico struct {
	JID       string              `json:"jid"`
	Nome      string              `json:"nome"`
	Mensagens []MensagemHistorico `json:"mensagens"`
}

// TamanhoLoteHistorico é o número máximo de mensagens por lote de history sync simulado.
const TamanhoLoteHistorico = 500

// InjetarHistorico simula o history sync, dividido em lotes.
func (c *Controle) InjetarHistorico(contaID string, conversas []ConversaHistorico) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cli, err := c.cliente(contaID)
	if err != nil {
		return err
	}
	var lotes []whatsapp.LoteHistorico
	atual := whatsapp.LoteHistorico{}
	n := 0
	base := c.relogio.Agora().Add(-24 * time.Hour)
	for _, conv := range conversas {
		chat := jidFlexivel(conv.JID)
		ch := whatsapp.ConversaHistorico{Chat: chat, Nome: conv.Nome}
		for i, mh := range conv.Mensagens {
			de := mh.De
			if de == "" && !chat.Grupo() {
				de, _ = chat.Telefone()
			}
			r := Recebida{De: de, Nome: mh.Nome, Texto: mh.Texto, Tipo: mh.Tipo, DeMim: mh.DeMim, WaID: mh.WaID, Em: mh.Em}
			if chat.Grupo() {
				r.GrupoJID = chat.String()
				r.GrupoNome = conv.Nome
			}
			if r.Em.IsZero() {
				r.Em = base.Add(time.Duration(i) * time.Second)
			}
			ch.Mensagens = append(ch.Mensagens, c.montarMensagem(contaID, r))
			n++
			if n == TamanhoLoteHistorico {
				atual.Conversas = append(atual.Conversas, ch)
				lotes = append(lotes, atual)
				atual = whatsapp.LoteHistorico{}
				ch = whatsapp.ConversaHistorico{Chat: chat, Nome: conv.Nome}
				n = 0
			}
		}
		if len(ch.Mensagens) > 0 || len(conv.Mensagens) == 0 {
			atual.Conversas = append(atual.Conversas, ch)
		}
	}
	if len(atual.Conversas) > 0 || len(lotes) == 0 {
		lotes = append(lotes, atual)
	}
	for i := range lotes {
		lotes[i].Progresso = (i + 1) * 100 / len(lotes)
		lotes[i].Final = i == len(lotes)-1
		l := lotes[i]
		cli.emitirSemTrava(whatsapp.Evento{Historico: &l})
	}
	return nil
}

func jidFlexivel(s string) whatsapp.JID {
	if strings.Contains(s, "@") {
		return whatsapp.ParseJID(s)
	}
	return whatsapp.JIDDeTelefone(s)
}

// DefinirSemWhatsApp substitui a lista de números sem WhatsApp.
func (c *Controle) DefinirSemWhatsApp(telefones []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.semWhatsApp = map[string]bool{}
	for _, t := range telefones {
		c.semWhatsApp[t] = true
	}
}

// DefinirFalhas substitui a lista de números cujo envio falha com `erro`.
func (c *Controle) DefinirFalhas(telefones []string, erro string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.falhas = map[string]string{}
	if erro == "" {
		erro = "falha simulada de envio"
	}
	for _, t := range telefones {
		c.falhas[t] = erro
	}
}

// Enviadas devolve tudo que foi "enviado" (inclusive em execuções anteriores).
func (c *Controle) Enviadas() []Enviada {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Enviada(nil), c.enviadas...)
}

// ---------------------------------------------------------------------------
// Cliente
// ---------------------------------------------------------------------------

// Cliente é o cliente falso de uma conta.
type Cliente struct {
	ctrl      *Controle
	contaID   string
	eventos   chan whatsapp.Evento
	qr        chan whatsapp.EventoQR
	conectado bool
	banido    bool
	fechado   bool
	emitidos  atomic.Int64
}

// EventosEmitidos conta eventos e itens de QR já entregues (usado para o motor esperar o
// processamento nas rotas /v1/falso/*).
func (cli *Cliente) EventosEmitidos() int64 { return cli.emitidos.Load() }

var _ whatsapp.Cliente = (*Cliente)(nil)

func (cli *Cliente) emitirSemTrava(e whatsapp.Evento) {
	if cli.fechado {
		return
	}
	cli.emitidos.Add(1)
	select {
	case cli.eventos <- e:
	default:
		// buffer cheio: bloqueia de forma limitada para não perder eventos em testes de carga
		t := time.NewTimer(5 * time.Second)
		select {
		case cli.eventos <- e:
		case <-t.C:
			cli.emitidos.Add(-1)
		}
		t.Stop()
	}
}

func (cli *Cliente) fecharSemTravaControle() {
	if cli.fechado {
		return
	}
	cli.fechado = true
	cli.conectado = false
	if cli.qr != nil {
		close(cli.qr)
		cli.qr = nil
	}
	close(cli.eventos)
}

// Conectar restaura a sessão (se pareada) ou emite um QR no canal obtido por CanalQR.
func (cli *Cliente) Conectar(ctx context.Context) error {
	c := cli.ctrl
	c.mu.Lock()
	defer c.mu.Unlock()
	if cli.fechado {
		return whatsapp.ErrDesconectado
	}
	if s, ok := c.sessoes[cli.contaID]; ok {
		cli.conectado = true
		cli.banido = false
		cli.emitirSemTrava(whatsapp.Evento{Estado: &whatsapp.EventoEstado{
			Estado: whatsapp.EstadoConectada, Proprio: whatsapp.JIDDeTelefone(s.Telefone), PushName: s.Nome}})
		return nil
	}
	if cli.qr == nil {
		return errors.New("sem sessão: chame CanalQR antes de Conectar")
	}
	c.qrSeq++
	cli.emitidos.Add(1)
	cli.qr <- whatsapp.EventoQR{Tipo: whatsapp.QRCodigo, Codigo: fmt.Sprintf("2@falso-%s-%d", cli.contaID, c.qrSeq), Expira: 60 * time.Second}
	return nil
}

// Desconectar fecha o canal de eventos.
func (cli *Cliente) Desconectar() {
	cli.ctrl.mu.Lock()
	defer cli.ctrl.mu.Unlock()
	cli.fecharSemTravaControle()
	if atual, ok := cli.ctrl.clientes[cli.contaID]; ok && atual == cli {
		delete(cli.ctrl.clientes, cli.contaID)
	}
}

// Pareado indica sessão salva.
func (cli *Cliente) Pareado() bool {
	cli.ctrl.mu.Lock()
	defer cli.ctrl.mu.Unlock()
	_, ok := cli.ctrl.sessoes[cli.contaID]
	return ok
}

// CanalQR prepara o canal de QR (antes de Conectar).
func (cli *Cliente) CanalQR(ctx context.Context) (<-chan whatsapp.EventoQR, error) {
	cli.ctrl.mu.Lock()
	defer cli.ctrl.mu.Unlock()
	if _, ok := cli.ctrl.sessoes[cli.contaID]; ok {
		return nil, errors.New("conta já pareada")
	}
	if cli.qr != nil {
		close(cli.qr)
	}
	cli.qr = make(chan whatsapp.EventoQR, 4)
	return cli.qr, nil
}

// Eventos devolve o canal de eventos.
func (cli *Cliente) Eventos() <-chan whatsapp.Evento { return cli.eventos }

// Proprio devolve o JID da conta pareada.
func (cli *Cliente) Proprio() (whatsapp.JID, bool) {
	cli.ctrl.mu.Lock()
	defer cli.ctrl.mu.Unlock()
	s, ok := cli.ctrl.sessoes[cli.contaID]
	if !ok {
		return whatsapp.JID{}, false
	}
	return whatsapp.JIDDeTelefone(s.Telefone), true
}

func (cli *Cliente) verificarConexaoSemTrava() error {
	if cli.banido {
		return whatsapp.ErrBanido
	}
	if !cli.conectado || cli.fechado {
		return whatsapp.ErrDesconectado
	}
	return nil
}

// TemWhatsApp consulta a lista de números sem WhatsApp.
func (cli *Cliente) TemWhatsApp(ctx context.Context, telefoneE164 string) (whatsapp.JID, bool, error) {
	cli.ctrl.mu.Lock()
	defer cli.ctrl.mu.Unlock()
	if err := cli.verificarConexaoSemTrava(); err != nil {
		return whatsapp.JID{}, false, err
	}
	if cli.ctrl.semWhatsApp[telefoneE164] {
		return whatsapp.JID{}, false, nil
	}
	return whatsapp.JIDDeTelefone(telefoneE164), true, nil
}

// Grupos devolve os grupos vistos em mensagens injetadas.
func (cli *Cliente) Grupos(ctx context.Context) ([]whatsapp.InfoGrupo, error) {
	cli.ctrl.mu.Lock()
	defer cli.ctrl.mu.Unlock()
	var lista []whatsapp.InfoGrupo
	for jid, nome := range cli.ctrl.grupos[cli.contaID] {
		lista = append(lista, whatsapp.InfoGrupo{JID: whatsapp.ParseJID(jid), Nome: nome})
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].JID.String() < lista[j].JID.String() })
	return lista, nil
}

// Contatos devolve os contatos vistos em mensagens injetadas.
func (cli *Cliente) Contatos(ctx context.Context) ([]whatsapp.InfoContato, error) {
	cli.ctrl.mu.Lock()
	defer cli.ctrl.mu.Unlock()
	var lista []whatsapp.InfoContato
	for _, ic := range cli.ctrl.contatos[cli.contaID] {
		lista = append(lista, ic)
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].JID.String() < lista[j].JID.String() })
	return lista, nil
}

func (cli *Cliente) checarEnvioSemTrava(para whatsapp.JID) error {
	if err := cli.verificarConexaoSemTrava(); err != nil {
		return err
	}
	if tel, ok := para.Telefone(); ok {
		if cli.ctrl.semWhatsApp[tel] {
			return whatsapp.ErrSemWhatsApp
		}
		if msg, ok := cli.ctrl.falhas[tel]; ok {
			return errors.New(msg)
		}
	}
	return nil
}

func (cli *Cliente) registrarSemTrava(e Enviada) whatsapp.Enviada {
	c := cli.ctrl
	e.ContaID = cli.contaID
	if e.WaID == "" {
		e.WaID = c.novoWaID()
	}
	e.Em = c.relogio.Agora()
	if tel, ok := whatsapp.ParseJID(e.Para).Telefone(); ok {
		e.Telefone = tel
	}
	c.registrarSemTrava(e)
	return whatsapp.Enviada{WaID: e.WaID, Em: e.Em}
}

// EnviarTexto registra o envio.
func (cli *Cliente) EnviarTexto(ctx context.Context, para whatsapp.JID, texto string, citar *whatsapp.Citacao) (whatsapp.Enviada, error) {
	cli.ctrl.mu.Lock()
	defer cli.ctrl.mu.Unlock()
	if err := cli.checarEnvioSemTrava(para); err != nil {
		return whatsapp.Enviada{}, err
	}
	e := Enviada{Tipo: whatsapp.TipoTexto, Para: para.String(), Texto: texto}
	if citar != nil {
		e.Citacao = citar.WaID
	}
	return cli.registrarSemTrava(e), nil
}

// EnviarMidia lê todo o conteúdo e registra o envio.
func (cli *Cliente) EnviarMidia(ctx context.Context, para whatsapp.JID, m whatsapp.MidiaEnvio, citar *whatsapp.Citacao) (whatsapp.Enviada, error) {
	var tamanho int64
	if m.Leitor != nil {
		n, err := io.Copy(io.Discard, m.Leitor)
		if err != nil {
			return whatsapp.Enviada{}, err
		}
		tamanho = n
	}
	cli.ctrl.mu.Lock()
	defer cli.ctrl.mu.Unlock()
	if err := cli.checarEnvioSemTrava(para); err != nil {
		return whatsapp.Enviada{}, err
	}
	e := Enviada{Tipo: m.Tipo, Para: para.String(), Texto: m.Legenda, Mimetype: m.Mimetype, Tamanho: tamanho, NomeArq: m.NomeArq, Voz: m.Voz}
	if citar != nil {
		e.Citacao = citar.WaID
	}
	return cli.registrarSemTrava(e), nil
}

// Reagir registra a reação.
func (cli *Cliente) Reagir(ctx context.Context, chat, remetente whatsapp.JID, waID, emoji string) error {
	cli.ctrl.mu.Lock()
	defer cli.ctrl.mu.Unlock()
	if err := cli.verificarConexaoSemTrava(); err != nil {
		return err
	}
	cli.registrarSemTrava(Enviada{Tipo: "reacao", Para: chat.String(), Alvo: waID, Emoji: emoji})
	return nil
}

// Editar registra a edição.
func (cli *Cliente) Editar(ctx context.Context, chat whatsapp.JID, waID, novoTexto string) error {
	cli.ctrl.mu.Lock()
	defer cli.ctrl.mu.Unlock()
	if err := cli.verificarConexaoSemTrava(); err != nil {
		return err
	}
	cli.registrarSemTrava(Enviada{Tipo: "edicao", Para: chat.String(), Alvo: waID, Texto: novoTexto})
	return nil
}

// Apagar registra o "apagar para todos".
func (cli *Cliente) Apagar(ctx context.Context, chat, remetente whatsapp.JID, waID string) error {
	cli.ctrl.mu.Lock()
	defer cli.ctrl.mu.Unlock()
	if err := cli.verificarConexaoSemTrava(); err != nil {
		return err
	}
	cli.registrarSemTrava(Enviada{Tipo: "apagar", Para: chat.String(), Alvo: waID})
	return nil
}

// MarcarLida não faz nada além de checar a conexão.
func (cli *Cliente) MarcarLida(ctx context.Context, chat, remetente whatsapp.JID, waIDs []string, em time.Time) error {
	cli.ctrl.mu.Lock()
	defer cli.ctrl.mu.Unlock()
	return cli.verificarConexaoSemTrava()
}

// Baixar devolve o conteúdo guardado para a mídia injetada.
func (cli *Cliente) Baixar(ctx context.Context, chave whatsapp.ChaveDownload) (io.ReadCloser, error) {
	if chave.Ref == "" {
		return nil, whatsapp.ErrMidiaExpirada
	}
	dados, err := os.ReadFile(filepath.Join(cli.ctrl.pasta, "midia", filepath.Base(chave.Ref)))
	if err != nil {
		return nil, whatsapp.ErrMidiaExpirada
	}
	return io.NopCloser(bytes.NewReader(dados)), nil
}
