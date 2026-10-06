package chat

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/whatsapp"
)

// ConversorVoz converte um áudio gravado (WebM/Opus) em OGG/Opus para mensagem de voz.
type ConversorVoz interface {
	ConverterVoz(ctx context.Context, a dominio.Arquivo) (dominio.Arquivo, error)
}

// prepararMidia completa a mensagem nova com o anexo, conforme `como`.
func (s *Servico) prepararMidia(ctx context.Context, nova *armazenamento.NovaMensagem, arquivoID, como string) error {
	if s.arquivos == nil {
		return erros.Novo(erros.Interno, "Anexos indisponíveis.")
	}
	a, err := s.arquivos.Obter(ctx, arquivoID)
	if err != nil {
		if erros.Eh(err, erros.NaoEncontrado) {
			return erros.Campo("arquivo_id", "Arquivo não encontrado.")
		}
		return err
	}
	tipo := a.TipoMidia
	ptt := false
	switch como {
	case "documento":
		tipo = whatsapp.TipoDocumento
	case "figurinha":
		if a.Mimetype != "image/webp" {
			return erros.ComDetalhes(erros.TipoNaoSuportado, "Figurinhas precisam ser imagens .webp.", map[string]any{"mimetype": a.Mimetype})
		}
		tipo = whatsapp.TipoFigurinha
	case "voz":
		if a.TipoMidia != whatsapp.TipoAudio {
			return erros.ComDetalhes(erros.TipoNaoSuportado, "Só é possível enviar áudio como mensagem de voz.", map[string]any{"mimetype": a.Mimetype})
		}
		tipo = whatsapp.TipoAudio
		ptt = true
		if !strings.HasPrefix(a.Mimetype, "audio/ogg") {
			if conv, ok := s.arquivos.(ConversorVoz); ok {
				if ogg, err := conv.ConverterVoz(ctx, a); err == nil {
					a = ogg
				} else {
					s.log.Warn().Err(err).Msg("não consegui converter o áudio para voz; enviando como áudio comum")
					ptt = false
				}
			} else {
				ptt = false
			}
		}
	}
	if tipo == whatsapp.TipoFigurinha && como != "figurinha" && como != "auto" {
		tipo = whatsapp.TipoImagem
	}
	nova.Tipo = tipo
	nova.ArquivoID = a.ID
	info := &whatsapp.MidiaInfo{Mimetype: a.Mimetype, Tamanho: a.Tamanho, PTT: ptt}
	if tipo == whatsapp.TipoDocumento {
		info.NomeArquivo = a.Nome
	}
	nova.Midia = info
	if tipo == whatsapp.TipoAudio || tipo == whatsapp.TipoFigurinha {
		nova.Texto = "" // o WhatsApp não aceita legenda nesses tipos
	}
	return nil
}

func (s *Servico) enviarMidiaBruta(ctx context.Context, cli whatsapp.Cliente, para whatsapp.JID, b armazenamento.MensagemBruta, citar *whatsapp.Citacao) (whatsapp.Enviada, error) {
	a, err := s.arquivos.Obter(ctx, b.ArquivoID)
	if err != nil {
		return whatsapp.Enviada{}, err
	}
	r, err := s.arquivos.Abrir(a)
	if err != nil {
		return whatsapp.Enviada{}, err
	}
	defer r.Close()
	m := whatsapp.MidiaEnvio{Tipo: b.Tipo, Leitor: r, Tamanho: a.Tamanho, Mimetype: a.Mimetype, NomeArq: a.Nome, Legenda: ptrStr(b.Texto)}
	if b.MidiaInfo != nil {
		m.Voz = b.MidiaInfo.PTT
	}
	return cli.EnviarMidia(ctx, para, m, citar)
}

// EnviarMidiaDireta envia um anexo (usado pelos disparos).
func (s *Servico) EnviarMidiaDireta(ctx context.Context, cli whatsapp.Cliente, para whatsapp.JID, arquivoID, legenda string) (whatsapp.Enviada, *whatsapp.MidiaInfo, string, error) {
	a, err := s.arquivos.Obter(ctx, arquivoID)
	if err != nil {
		return whatsapp.Enviada{}, nil, "", err
	}
	r, err := s.arquivos.Abrir(a)
	if err != nil {
		return whatsapp.Enviada{}, nil, "", err
	}
	defer r.Close()
	tipo := a.TipoMidia
	if tipo == whatsapp.TipoAudio || tipo == whatsapp.TipoFigurinha {
		legenda = ""
	}
	info := &whatsapp.MidiaInfo{Mimetype: a.Mimetype, Tamanho: a.Tamanho}
	if tipo == whatsapp.TipoDocumento {
		info.NomeArquivo = a.Nome
	}
	env, err := cli.EnviarMidia(ctx, para, whatsapp.MidiaEnvio{Tipo: tipo, Leitor: r, Tamanho: a.Tamanho, Mimetype: a.Mimetype, NomeArq: a.Nome, Legenda: legenda}, nil)
	return env, info, tipo, err
}

// ConteudoMidia devolve o caminho local da mídia da mensagem, baixando sob demanda.
func (s *Servico) ConteudoMidia(ctx context.Context, mensagemID string) (string, string, error) {
	b, err := armazenamento.ObterMensagem(ctx, s.banco.L(), mensagemID)
	if err != nil {
		return "", "", err
	}
	if b.Apagada || (b.MidiaInfo == nil && b.ArquivoID == "") {
		return "", "", erros.Novo(erros.NaoEncontrado, "Esta mensagem não tem mídia.")
	}
	mt := ""
	if b.MidiaInfo != nil {
		mt = b.MidiaInfo.Mimetype
	}
	if b.ArquivoID != "" && s.arquivos != nil {
		if a, err := s.arquivos.Obter(ctx, b.ArquivoID); err == nil {
			return a.Caminho, a.Mimetype, nil
		}
	}
	if b.MidiaCaminho != "" {
		if _, err := os.Stat(b.MidiaCaminho); err == nil {
			return b.MidiaCaminho, mt, nil
		}
	}
	destino := filepath.Join(s.pasta, "midia", b.ContaID, b.ID)
	if err := s.baixar(ctx, b.ContaID, b.MidiaInfo.Chave, destino); err != nil {
		return "", "", err
	}
	armazenamento.DefinirMidiaCaminho(ctx, s.banco.E(), b.ID, destino)
	s.publicarMensagem(ctx, eventos.MensagemAtualizada, b.ID)
	return destino, mt, nil
}

func (s *Servico) baixar(ctx context.Context, contaID string, chave whatsapp.ChaveDownload, destino string) error {
	falha := func(motivo string) error {
		return erros.ComDetalhes(erros.WhatsAppErro, "Não foi possível baixar.", map[string]any{"motivo": motivo})
	}
	cli, err := s.contas.Cliente(ctx, contaID)
	if err != nil {
		return falha("conta desconectada")
	}
	r, err := cli.Baixar(ctx, chave)
	if err != nil {
		return falha(motivoErro(err))
	}
	defer r.Close()
	if err := os.MkdirAll(filepath.Dir(destino), 0o700); err != nil {
		return err
	}
	tmp := destino + ".parcial"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		os.Remove(tmp)
		return falha(err.Error())
	}
	f.Close()
	return os.Rename(tmp, destino)
}

// Figurinhas recentes da conta.
func (s *Servico) Figurinhas(ctx context.Context, contaID string, limite int) ([]armazenamento.Figurinha, error) {
	if _, err := armazenamento.ObterConta(ctx, s.banco.L(), contaID); err != nil {
		return nil, err
	}
	return armazenamento.FigurinhasRecentes(ctx, s.banco.L(), contaID, limite)
}
