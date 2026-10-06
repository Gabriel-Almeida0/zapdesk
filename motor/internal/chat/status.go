package chat

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/whatsapp"
)

// JanelaStatus: status aparecem por 24 h.
const JanelaStatus = 24 * time.Hour

func (s *Servico) gravarStatus(ctx context.Context, contaID string, m whatsapp.MensagemRecebida) error {
	if m.DeMim {
		return nil
	}
	tipo := m.TipoMsg
	switch tipo {
	case whatsapp.TipoTexto, "":
		tipo = "texto"
	case whatsapp.TipoImagem, whatsapp.TipoVideo:
	default:
		return nil // áudio etc. não são exibidos
	}
	em := m.Em
	if em.IsZero() {
		em = s.relogio.Agora()
	}
	novo := armazenamento.NovoStatus{ID: ids.NovoEm(em), ContaID: contaID, WaID: m.WaID, ContatoJID: m.Remetente.String(),
		ContatoNome: m.NomeRemetente, Tipo: tipo, Texto: m.Texto, Midia: m.Midia, PublicadoEm: em}
	inserido, err := armazenamento.InserirStatus(ctx, s.banco.E(), novo)
	if err != nil || !inserido {
		return err
	}
	b, err := armazenamento.ObterStatus(ctx, s.banco.L(), novo.ID)
	if err != nil {
		return err
	}
	s.barramento.Publicar(eventos.StatusNovo, contaID, b.Status)
	return nil
}

// ListarStatus devolve os status das últimas 24 h agrupados por contato (mais recentes primeiro).
func (s *Servico) ListarStatus(ctx context.Context, contaID string) ([]dominio.StatusPorContato, error) {
	if _, err := armazenamento.ObterConta(ctx, s.banco.L(), contaID); err != nil {
		return nil, err
	}
	agora := s.relogio.Agora()
	s.LimparStatus(ctx)
	lista, err := armazenamento.ListarStatusRecentes(ctx, s.banco.L(), contaID, agora.Add(-JanelaStatus))
	if err != nil {
		return nil, err
	}
	grupos := []dominio.StatusPorContato{}
	pos := map[string]int{}
	for _, b := range lista {
		i, ok := pos[b.ContatoJID]
		if !ok {
			i = len(grupos)
			pos[b.ContatoJID] = i
			grupos = append(grupos, dominio.StatusPorContato{ContatoJID: b.ContatoJID, ContatoNome: b.ContatoNome})
		}
		grupos[i].Itens = append(grupos[i].Itens, b.Status)
	}
	return grupos, nil
}

// LimparStatus remove status com mais de 24 h (no máximo uma vez por minuto).
func (s *Servico) LimparStatus(ctx context.Context) {
	agora := s.relogio.Agora()
	s.mu.Lock()
	if !s.ultimaLimpa.IsZero() && agora.Sub(s.ultimaLimpa) < time.Minute && agora.After(s.ultimaLimpa) {
		s.mu.Unlock()
		return
	}
	s.ultimaLimpa = agora
	s.mu.Unlock()
	caminhos, err := armazenamento.ApagarStatusAntigos(ctx, s.banco.E(), agora.Add(-JanelaStatus))
	if err != nil {
		s.log.Warn().Err(err).Msg("limpar status antigos")
	}
	for _, c := range caminhos {
		os.Remove(c)
	}
}

// ConteudoStatus devolve o caminho local da mídia do status (baixa sob demanda).
func (s *Servico) ConteudoStatus(ctx context.Context, id string) (string, string, error) {
	b, err := armazenamento.ObterStatus(ctx, s.banco.L(), id)
	if err != nil {
		return "", "", err
	}
	if b.MidiaInfo == nil {
		return "", "", erros.Novo(erros.NaoEncontrado, "Este status não tem mídia.")
	}
	if b.MidiaCaminho != "" {
		if _, err := os.Stat(b.MidiaCaminho); err == nil {
			return b.MidiaCaminho, b.MidiaInfo.Mimetype, nil
		}
	}
	destino := filepath.Join(s.pasta, "midia", b.ContaID, "status-"+b.ID)
	if err := s.baixar(ctx, b.ContaID, b.MidiaInfo.Chave, destino); err != nil {
		return "", "", err
	}
	armazenamento.DefinirMidiaStatus(ctx, s.banco.E(), b.ID, destino)
	return destino, b.MidiaInfo.Mimetype, nil
}
