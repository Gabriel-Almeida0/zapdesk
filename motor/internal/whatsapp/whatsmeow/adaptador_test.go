package whatsmeow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"zapdesk/motor/internal/whatsapp"
)

func abrirTeste(t *testing.T) (*Fabrica, *Cliente, string) {
	t.Helper()
	pasta := t.TempDir()
	f, err := NovaFabrica(pasta, zerolog.Nop())
	if err != nil {
		t.Fatal(err)
	}
	cli, err := f.Abrir(context.Background(), "conta1")
	if err != nil {
		t.Fatalf("abrir sessão (DSN/pragma foreign_keys): %v", err)
	}
	return f, cli.(*Cliente), pasta
}

func TestAbrirSessaoQRERemover(t *testing.T) {
	f, c, pasta := abrirTeste(t)
	if c.Pareado() {
		t.Fatal("sessão nova não deveria estar pareada")
	}
	if _, ok := c.Proprio(); ok {
		t.Fatal("sem JID antes de parear")
	}
	if _, err := os.Stat(filepath.Join(pasta, "sessoes", "conta1.db")); err != nil {
		t.Fatalf("banco da sessão não criado: %v", err)
	}
	qr, err := c.CanalQR(context.Background())
	if err != nil || qr == nil {
		t.Fatalf("GetQRChannel antes de conectar: %v", err)
	}
	// sem conexão os envios falham com ErrDesconectado (o disparo aguarda)
	if _, err := c.EnviarTexto(context.Background(), whatsapp.JIDDeTelefone("+5511999990000"), "x", nil); !errors.Is(err, whatsapp.ErrDesconectado) {
		t.Fatalf("envio sem conexão: %v", err)
	}
	if _, _, err := c.TemWhatsApp(context.Background(), "+5511999990000"); !errors.Is(err, whatsapp.ErrDesconectado) {
		t.Fatalf("TemWhatsApp sem conexão: %v", err)
	}
	c.Desconectar()
	if _, ok := <-c.Eventos(); ok {
		t.Fatal("canal de eventos deveria fechar")
	}
	c.Desconectar() // idempotente
	if err := f.Remover(context.Background(), "conta1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(pasta, "sessoes", "conta1.db")); !os.IsNotExist(err) {
		t.Fatal("sessão deveria ter sido apagada")
	}
}

func TestConverterConteudo(t *testing.T) {
	tipo, texto, midia, cit := converterConteudo(&waE2E.Message{Conversation: proto.String("oi")})
	if tipo != "texto" || texto != "oi" || midia != nil || cit != nil {
		t.Fatal("texto simples")
	}
	tipo, texto, _, cit = converterConteudo(&waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
		Text: proto.String("resposta"),
		ContextInfo: &waE2E.ContextInfo{StanzaID: proto.String("ABC"), Participant: proto.String("5511988887777@s.whatsapp.net"),
			QuotedMessage: &waE2E.Message{Conversation: proto.String("pergunta")}},
	}})
	if tipo != "texto" || texto != "resposta" || cit == nil || cit.WaID != "ABC" || cit.Texto != "pergunta" || cit.Remetente.Usuario != "5511988887777" {
		t.Fatalf("citação: %+v", cit)
	}
	tipo, texto, midia, _ = converterConteudo(&waE2E.Message{ImageMessage: &waE2E.ImageMessage{Caption: proto.String("foto"),
		Mimetype: proto.String("image/jpeg"), FileLength: proto.Uint64(1234), DirectPath: proto.String("/v/t62/x"), MediaKey: []byte{1, 2},
		Width: proto.Uint32(640), Height: proto.Uint32(480)}})
	if tipo != "imagem" || texto != "foto" || midia.Tamanho != 1234 || midia.Chave.DirectPath != "/v/t62/x" || midia.Largura != 640 || midia.Chave.Tipo != "imagem" {
		t.Fatalf("imagem: %+v", midia)
	}
	tipo, _, midia, _ = converterConteudo(&waE2E.Message{AudioMessage: &waE2E.AudioMessage{PTT: proto.Bool(true), Seconds: proto.Uint32(7), Mimetype: proto.String("audio/ogg; codecs=opus")}})
	if tipo != "audio" || !midia.PTT || midia.DuracaoS != 7 {
		t.Fatalf("áudio: %+v", midia)
	}
	tipo, _, midia, _ = converterConteudo(&waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{FileName: proto.String("nota.pdf"), Mimetype: proto.String("application/pdf")}})
	if tipo != "documento" || midia.NomeArquivo != "nota.pdf" {
		t.Fatal("documento")
	}
	if tipo, _, _, _ = converterConteudo(&waE2E.Message{StickerMessage: &waE2E.StickerMessage{Mimetype: proto.String("image/webp")}}); tipo != "figurinha" {
		t.Fatal("figurinha")
	}
	if tipo, _, _, _ = converterConteudo(&waE2E.Message{VideoMessage: &waE2E.VideoMessage{Seconds: proto.Uint32(3)}}); tipo != "video" {
		t.Fatal("vídeo")
	}
	if tipo, _, _, _ = converterConteudo(&waE2E.Message{}); tipo != "" {
		t.Fatal("mensagem desconhecida deveria ser ignorada")
	}
}

func TestConverterMensagemComLID(t *testing.T) {
	_, c, _ := abrirTeste(t)
	defer c.Desconectar()
	ctx := context.Background()
	lid := types.NewJID("123456789", types.HiddenUserServer)
	pn := types.NewJID("5511977776666", types.DefaultUserServer)
	// o LIDStore só é ligado ao dispositivo após o pareamento; no teste ligamos direto
	c.cli.Store.LIDs = c.container.LIDMap
	if err := c.cli.Store.LIDs.PutLIDMapping(ctx, lid, pn); err != nil {
		t.Fatal(err)
	}

	em := time.Now()
	info := types.MessageInfo{ID: "M1", Timestamp: em, PushName: "Ana",
		MessageSource: types.MessageSource{Chat: lid, Sender: lid, AddressingMode: types.AddressingModeLID}}
	evs := c.converterMensagem(ctx, &events.Message{Info: info, Message: &waE2E.Message{Conversation: proto.String("olá")}})
	if len(evs) != 1 || evs[0].Mensagem == nil {
		t.Fatalf("eventos: %+v", evs)
	}
	m := evs[0].Mensagem
	if m.Chat.Usuario != "5511977776666" || m.Chat.Servidor != "s.whatsapp.net" || m.Remetente.Usuario != "5511977776666" || m.NomeRemetente != "Ana" {
		t.Fatalf("LID não resolvido para telefone: %+v", m)
	}

	// SenderAlt tem prioridade
	info2 := types.MessageInfo{ID: "M2", Timestamp: em, MessageSource: types.MessageSource{
		Chat: types.NewJID("999", types.HiddenUserServer), Sender: types.NewJID("999", types.HiddenUserServer),
		SenderAlt: types.NewJID("5521988887777", types.DefaultUserServer)}}
	evs = c.converterMensagem(ctx, &events.Message{Info: info2, Message: &waE2E.Message{Conversation: proto.String("x")}})
	if evs[0].Mensagem.Chat.Usuario != "5521988887777" {
		t.Fatalf("SenderAlt: %+v", evs[0].Mensagem)
	}

	// reação, revogação e edição
	chat := types.NewJID("5511977776666", types.DefaultUserServer)
	base := types.MessageInfo{ID: "R1", Timestamp: em, MessageSource: types.MessageSource{Chat: chat, Sender: chat}}
	evs = c.converterMensagem(ctx, &events.Message{Info: base, Message: &waE2E.Message{ReactionMessage: &waE2E.ReactionMessage{
		Key: &waCommon.MessageKey{ID: proto.String("ALVO")}, Text: proto.String("👍")}}})
	if evs[0].Reacao == nil || evs[0].Reacao.WaIDAlvo != "ALVO" || evs[0].Reacao.Emoji != "👍" {
		t.Fatalf("reação: %+v", evs)
	}
	evs = c.converterMensagem(ctx, &events.Message{Info: base, Message: &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{
		Type: waE2E.ProtocolMessage_REVOKE.Enum(), Key: &waCommon.MessageKey{ID: proto.String("ALVO")}}}})
	if evs[0].Revogacao == nil || evs[0].Revogacao.WaIDAlvo != "ALVO" {
		t.Fatalf("revogação: %+v", evs)
	}
	evs = c.converterMensagem(ctx, &events.Message{Info: base, Message: &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{
		Type: waE2E.ProtocolMessage_MESSAGE_EDIT.Enum(), Key: &waCommon.MessageKey{ID: proto.String("ALVO")},
		EditedMessage: &waE2E.Message{Conversation: proto.String("novo texto")}}}})
	if evs[0].Edicao == nil || evs[0].Edicao.NovoTexto != "novo texto" {
		t.Fatalf("edição: %+v", evs)
	}
	// status
	st := types.MessageInfo{ID: "S1", Timestamp: em, MessageSource: types.MessageSource{Chat: types.StatusBroadcastJID, Sender: chat}}
	evs = c.converterMensagem(ctx, &events.Message{Info: st, Message: &waE2E.Message{Conversation: proto.String("meu status")}})
	if !evs[0].Mensagem.Chat.Status() {
		t.Fatal("status@broadcast")
	}
}

func TestEventosDeConexao(t *testing.T) {
	_, c, _ := abrirTeste(t)
	defer c.Desconectar()
	c.jaConectou = true
	c.tratar(&events.Disconnected{})
	c.tratar(&events.KeepAliveTimeout{}) // não repete
	if e := <-c.eventos; e.Estado == nil || e.Estado.Estado != whatsapp.EstadoRedeCaiu {
		t.Fatalf("queda: %+v", e)
	}
	c.tratar(&events.KeepAliveRestored{})
	if e := <-c.eventos; e.Estado.Estado != whatsapp.EstadoRedeVoltou {
		t.Fatalf("volta: %+v", e)
	}
	c.tratar(&events.LoggedOut{Reason: events.ConnectFailureLoggedOut})
	if e := <-c.eventos; e.Estado.Estado != whatsapp.EstadoDesconectada {
		t.Fatal("logout")
	}
	c.tratar(&events.TemporaryBan{Code: events.TempBanSentToTooManyPeople})
	if e := <-c.eventos; e.Estado.Estado != whatsapp.EstadoBanida {
		t.Fatal("ban")
	}
	c.tratar(&events.StreamReplaced{})
	if e := <-c.eventos; e.Estado.Estado != whatsapp.EstadoSubstituida {
		t.Fatal("substituída")
	}
	chat := types.NewJID("5511977776666", types.DefaultUserServer)
	c.tratar(&events.Receipt{MessageSource: types.MessageSource{Chat: chat, Sender: chat}, MessageIDs: []string{"A"}, Type: types.ReceiptTypeRead})
	if e := <-c.eventos; e.Recibo == nil || e.Recibo.Tipo != whatsapp.ReciboLido || e.Recibo.WaIDs[0] != "A" {
		t.Fatalf("recibo: %+v", e)
	}
	c.tratar(&events.Receipt{MessageSource: types.MessageSource{Chat: chat, Sender: chat, IsFromMe: true}, MessageIDs: []string{"B"}, Type: types.ReceiptTypeReadSelf})
	select {
	case e := <-c.eventos:
		t.Fatalf("read-self não deveria virar recibo: %+v", e)
	default:
	}
}

func TestTraduzirErro(t *testing.T) {
	casos := map[error]error{
		whatsmeow.ErrNotConnected:               whatsapp.ErrDesconectado,
		whatsmeow.ErrIQTimedOut:                 whatsapp.ErrDesconectado,
		context.DeadlineExceeded:                whatsapp.ErrDesconectado,
		whatsmeow.ErrMediaDownloadFailedWith404: whatsapp.ErrMidiaExpirada,
	}
	for entrada, esperado := range casos {
		if !errors.Is(traduzirErro(entrada), esperado) {
			t.Errorf("%v deveria virar %v", entrada, esperado)
		}
	}
	outro := errors.New("server returned error 479")
	if traduzirErro(outro) != outro || traduzirErro(nil) != nil {
		t.Fatal("erros desconhecidos passam intactos")
	}
}

func TestMontarMidia(t *testing.T) {
	up := whatsmeow.UploadResponse{URL: "u", DirectPath: "/d", MediaKey: []byte{1}, FileLength: 10}
	for _, tipo := range []string{"imagem", "video", "audio", "documento", "figurinha"} {
		msg, err := montarMidia(whatsapp.MidiaEnvio{Tipo: tipo, Mimetype: "x/y", NomeArq: "a.pdf", Legenda: "leg", Voz: true}, up, nil)
		if err != nil || msg == nil {
			t.Fatalf("%s: %v", tipo, err)
		}
		tipoVolta, _, midia, _ := converterConteudo(msg)
		if tipoVolta != tipo || midia.Chave.DirectPath != "/d" {
			t.Fatalf("%s: volta %s %+v", tipo, tipoVolta, midia)
		}
		if tipo == "audio" && (!midia.PTT || midia.Mimetype != "audio/ogg; codecs=opus") {
			t.Fatal("voz deve ir como PTT ogg/opus")
		}
	}
	if _, err := montarMidia(whatsapp.MidiaEnvio{Tipo: "x"}, up, nil); err == nil {
		t.Fatal("tipo inválido")
	}
}
