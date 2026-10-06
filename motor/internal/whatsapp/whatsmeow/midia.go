package whatsmeow

import (
	"bytes"
	"context"
	"errors"
	"io"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	"zapdesk/motor/internal/whatsapp"
)

func tipoMidiaWA(tipo string) (whatsmeow.MediaType, error) {
	switch tipo {
	case whatsapp.TipoImagem, whatsapp.TipoFigurinha:
		return whatsmeow.MediaImage, nil
	case whatsapp.TipoVideo:
		return whatsmeow.MediaVideo, nil
	case whatsapp.TipoAudio:
		return whatsmeow.MediaAudio, nil
	case whatsapp.TipoDocumento:
		return whatsmeow.MediaDocument, nil
	}
	return "", errTipoMidia
}

// montarMidia monta o protobuf da mídia a partir do upload.
func montarMidia(m whatsapp.MidiaEnvio, up whatsmeow.UploadResponse, ci *waE2E.ContextInfo) (*waE2E.Message, error) {
	legenda := func() *string {
		if m.Legenda == "" {
			return nil
		}
		return proto.String(m.Legenda)
	}
	switch m.Tipo {
	case whatsapp.TipoImagem:
		return &waE2E.Message{ImageMessage: &waE2E.ImageMessage{Caption: legenda(), Mimetype: proto.String(m.Mimetype),
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey,
			FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: proto.Uint64(up.FileLength), ContextInfo: ci}}, nil
	case whatsapp.TipoVideo:
		return &waE2E.Message{VideoMessage: &waE2E.VideoMessage{Caption: legenda(), Mimetype: proto.String(m.Mimetype),
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey,
			FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: proto.Uint64(up.FileLength), ContextInfo: ci}}, nil
	case whatsapp.TipoAudio:
		mt := m.Mimetype
		if m.Voz {
			mt = "audio/ogg; codecs=opus"
		}
		return &waE2E.Message{AudioMessage: &waE2E.AudioMessage{Mimetype: proto.String(mt), PTT: proto.Bool(m.Voz),
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey,
			FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: proto.Uint64(up.FileLength), ContextInfo: ci}}, nil
	case whatsapp.TipoDocumento:
		return &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{Caption: legenda(), Mimetype: proto.String(m.Mimetype),
			FileName: proto.String(m.NomeArq), Title: proto.String(m.NomeArq),
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey,
			FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: proto.Uint64(up.FileLength), ContextInfo: ci}}, nil
	case whatsapp.TipoFigurinha:
		return &waE2E.Message{StickerMessage: &waE2E.StickerMessage{Mimetype: proto.String("image/webp"),
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey,
			FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: proto.Uint64(up.FileLength), ContextInfo: ci}}, nil
	}
	return nil, errTipoMidia
}

// EnviarMidia faz o upload (UploadReader: não segura o arquivo inteiro na memória) e envia.
func (c *Cliente) EnviarMidia(ctx context.Context, para whatsapp.JID, m whatsapp.MidiaEnvio, citar *whatsapp.Citacao) (whatsapp.Enviada, error) {
	if !c.cli.IsConnected() {
		return whatsapp.Enviada{}, whatsapp.ErrDesconectado
	}
	tipo, err := tipoMidiaWA(m.Tipo)
	if err != nil {
		return whatsapp.Enviada{}, err
	}
	up, err := c.cli.UploadReader(ctx, m.Leitor, nil, tipo)
	if err != nil {
		return whatsapp.Enviada{}, traduzirErro(err)
	}
	msg, err := montarMidia(m, up, contextoCitacao(citar))
	if err != nil {
		return whatsapp.Enviada{}, err
	}
	return c.enviar(ctx, para, msg)
}

// Baixar baixa a mídia; se o servidor já expirou o arquivo, pede reenvio ao celular
// (SendMediaRetryReceipt) e devolve ErrMidiaExpirada — o usuário tenta de novo depois.
func (c *Cliente) Baixar(ctx context.Context, ch whatsapp.ChaveDownload) (io.ReadCloser, error) {
	if !c.cli.IsConnected() {
		return nil, whatsapp.ErrDesconectado
	}
	tipo, err := tipoMidiaWA(ch.Tipo)
	if err != nil {
		return nil, err
	}
	if ch.DirectPath == "" {
		return nil, whatsapp.ErrMidiaExpirada
	}
	dados, err := c.cli.DownloadMediaWithPath(ctx, ch.DirectPath, ch.FileEncSHA256, ch.FileSHA256, ch.MediaKey, tipo, "", false)
	if err != nil {
		err = traduzirErro(err)
		if errors.Is(err, whatsapp.ErrMidiaExpirada) && ch.WaID != "" {
			info := &types.MessageInfo{ID: ch.WaID}
			info.Chat, _ = types.ParseJID(ch.Chat)
			info.Sender, _ = types.ParseJID(ch.Remetente)
			info.IsFromMe, info.IsGroup = ch.DeMim, ch.Grupo
			if errRetry := c.cli.SendMediaRetryReceipt(ctx, info, ch.MediaKey); errRetry != nil {
				c.log.Debug().Err(errRetry).Msg("pedido de reenvio de mídia")
			}
		}
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(dados)), nil
}
