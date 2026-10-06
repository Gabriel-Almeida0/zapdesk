package chat

import (
	"context"
	"database/sql"
	"runtime/debug"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/fatos"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/logs"
	"zapdesk/motor/internal/whatsapp"
)

// ProcessarEvento trata um evento de conteúdo de uma conta (chamado em ordem, por conta).
func (s *Servico) ProcessarEvento(ctx context.Context, contaID string, ev whatsapp.Evento) {
	var err error
	switch {
	case ev.Mensagem != nil:
		if ev.Mensagem.Chat.Status() {
			err = s.gravarStatus(ctx, contaID, *ev.Mensagem)
		} else {
			_, err = s.receberMensagem(ctx, contaID, *ev.Mensagem)
		}
	case ev.Recibo != nil:
		err = s.aplicarRecibo(ctx, contaID, *ev.Recibo)
	case ev.Reacao != nil:
		err = s.aplicarReacao(ctx, contaID, *ev.Reacao)
	case ev.Edicao != nil:
		err = s.aplicarEdicao(ctx, contaID, *ev.Edicao)
	case ev.Revogacao != nil:
		err = s.aplicarRevogacao(ctx, contaID, *ev.Revogacao)
	case ev.Historico != nil:
		err = s.gravarHistorico(ctx, contaID, *ev.Historico)
	case ev.Contato != nil:
		err = s.gravarContato(ctx, contaID, *ev.Contato)
	}
	if err != nil {
		s.log.Error().Err(err).Str("conta", contaID).Msg("falha ao processar evento do WhatsApp")
	}
}

// SincronizarContatos grava agenda e grupos da conta logo após conectar.
func (s *Servico) SincronizarContatos(ctx context.Context, contaID string, cli whatsapp.Cliente) {
	if contatos, err := cli.Contatos(ctx); err == nil {
		for _, c := range contatos {
			ic := c
			s.gravarContato(ctx, contaID, ic)
		}
	} else {
		s.log.Warn().Err(err).Msg("listar contatos")
	}
	if grupos, err := cli.Grupos(ctx); err == nil {
		agora := s.relogio.Agora()
		for _, g := range grupos {
			armazenamento.GarantirConversa(ctx, s.banco.E(), contaID, g.JID.String(), "grupo", g.Nome, "", agora)
		}
	} else {
		s.log.Warn().Err(err).Msg("listar grupos")
	}
}

func (s *Servico) gravarContato(ctx context.Context, contaID string, ic whatsapp.InfoContato) error {
	if ic.JID.Vazio() || ic.JID.Grupo() || ic.JID.Status() {
		return nil
	}
	tel, _ := ic.JID.Telefone()
	id, mudou, err := armazenamento.GarantirContato(ctx, s.banco.E(), contaID, armazenamento.DadosContato{
		JID: ic.JID.String(), Telefone: tel, Nome: ic.Nome, NomePush: ic.NomePush}, s.relogio.Agora())
	if err == nil && mudou {
		s.publicarContato(ctx, id)
	}
	return err
}

type resultadoGravacao struct {
	mensagemID, conversaID, contatoID    string
	inserida, conversaNova, contatoMudou bool
}

// gravarMensagem grava contato, conversa e mensagem numa transação (idempotente por wa_id).
func (s *Servico) gravarMensagem(ctx context.Context, tx *sql.Tx, contaID string, m whatsapp.MensagemRecebida, historico bool) (resultadoGravacao, error) {
	var r resultadoGravacao
	agora := s.relogio.Agora()
	chat := m.Chat.String()
	if m.Grupo || m.Chat.Grupo() {
		id, nova, err := armazenamento.GarantirConversa(ctx, tx, contaID, chat, "grupo", m.NomeChat, "", agora)
		if err != nil {
			return r, err
		}
		r.conversaID, r.conversaNova = id, nova
	} else {
		tel, _ := m.Chat.Telefone()
		push := ""
		if !m.DeMim {
			push = m.NomeRemetente
		}
		cid, mudou, err := armazenamento.GarantirContato(ctx, tx, contaID, armazenamento.DadosContato{JID: chat, Telefone: tel, NomePush: push}, agora)
		if err != nil {
			return r, err
		}
		r.contatoID, r.contatoMudou = cid, mudou
		id, nova, err := armazenamento.GarantirConversa(ctx, tx, contaID, chat, "individual", "", cid, agora)
		if err != nil {
			return r, err
		}
		r.conversaID, r.conversaNova = id, nova
	}

	remetente := m.Remetente.String()
	if m.DeMim {
		if p, ok := s.contas.Proprio(contaID); ok {
			remetente = p.String()
		}
	}
	estado := dominio.MsgRecebida
	if m.DeMim {
		estado = dominio.MsgEnviada
	}
	nova := armazenamento.NovaMensagem{
		ID: ids.NovoEm(m.Em), ContaID: contaID, ConversaID: r.conversaID, WaID: m.WaID, RemetenteJID: remetente,
		DeMim: m.DeMim, Tipo: m.TipoMsg, Texto: m.Texto, Midia: m.Midia, Estado: estado, EnviadaEm: m.Em,
	}
	if !m.DeMim && m.NomeRemetente != "" {
		nova.RemetenteNome = m.NomeRemetente
	}
	if nova.Tipo == "" {
		nova.Tipo = whatsapp.TipoTexto
	}
	if m.Citacao != nil && m.Citacao.WaID != "" {
		nova.CitacaoWaID = m.Citacao.WaID
		nova.CitacaoResumo = m.Citacao.Texto
		if citada, ok, _ := armazenamento.MensagemPorWaID(ctx, tx, contaID, m.Citacao.WaID); ok {
			if citada.Texto != nil {
				nova.CitacaoResumo = *citada.Texto
			} else {
				nova.CitacaoResumo = resumo(citada.Tipo, "")
			}
			if citada.RemetenteNome != nil {
				nova.CitacaoRemetenteNome = *citada.RemetenteNome
			}
		}
		if len([]rune(nova.CitacaoResumo)) > 200 {
			nova.CitacaoResumo = string([]rune(nova.CitacaoResumo)[:200])
		}
	}
	inserida, err := armazenamento.InserirMensagem(ctx, tx, nova)
	if err != nil {
		return r, err
	}
	r.inserida = inserida
	if !inserida {
		return r, nil
	}
	r.mensagemID = nova.ID
	somar := 0
	if !m.DeMim && !historico {
		somar = 1
	}
	return r, armazenamento.AtualizarUltimaMensagem(ctx, tx, r.conversaID, m.Em, resumo(nova.Tipo, m.Texto), somar)
}

func (s *Servico) receberMensagem(ctx context.Context, contaID string, m whatsapp.MensagemRecebida) (resultadoGravacao, error) {
	var r resultadoGravacao
	err := s.banco.Transacao(ctx, func(tx *sql.Tx) error {
		var err error
		r, err = s.gravarMensagem(ctx, tx, contaID, m, false)
		return err
	})
	if err != nil {
		return r, err
	}
	s.log.Debug().Str("conta", contaID).Str("wa_id", m.WaID).Str(logs.CampoConteudo, m.Texto).Msg("mensagem recebida")
	if r.contatoMudou && r.contatoID != "" {
		s.publicarContato(ctx, r.contatoID)
	}
	if r.inserida {
		s.publicarMensagem(ctx, eventos.MensagemNova, r.mensagemID)
		s.publicarConversa(ctx, r.conversaID)
		if !m.DeMim && !m.Grupo {
			if tel, ok := m.Chat.Telefone(); ok {
				for _, o := range s.ouvintes {
					o.MensagemDoContato(contaID, tel, m.Em)
				}
			}
		}
		s.emitirRecebida(ctx, contaID, m, r)
	}
	return r, nil
}

// emitirRecebida publica o fato para as automações: mensagem do contato (com "primeira") ou
// mensagem minha com wa_id inédito vinda do celular (= manual; ecos do que o motor enviou são
// deduplicados pelo UNIQUE(conversa_id, wa_id)).
func (s *Servico) emitirRecebida(ctx context.Context, contaID string, m whatsapp.MensagemRecebida, r resultadoGravacao) {
	grupo := m.Grupo || m.Chat.Grupo()
	f := fatos.Fato{ContaID: contaID, ConversaID: r.conversaID, ContatoID: r.contatoID, MensagemID: r.mensagemID,
		Grupo: grupo, Texto: m.Texto, Em: m.Em, TemMidia: m.Midia != nil}
	if m.DeMim {
		f.Tipo, f.Origem = fatos.MensagemEnviada, fatos.OrigemManual
	} else {
		f.Tipo = fatos.MensagemRecebida
		var n int
		s.banco.L().QueryRowContext(ctx, `SELECT count(*) FROM mensagens WHERE conversa_id = ? AND de_mim = 0`, r.conversaID).Scan(&n)
		f.Primeira = n == 1
	}
	s.fatos.Emitir(ctx, f)
}

func (s *Servico) aplicarRecibo(ctx context.Context, contaID string, r whatsapp.Recibo) error {
	novo := ""
	switch r.Tipo {
	case whatsapp.ReciboEntregue:
		novo = dominio.MsgEntregue
	case whatsapp.ReciboLido, whatsapp.ReciboTocado:
		novo = dominio.MsgLida
	default:
		return nil
	}
	em := r.Em
	if em.IsZero() {
		em = s.relogio.Agora()
	}
	for _, waID := range r.WaIDs {
		b, ok, err := armazenamento.MensagemPorWaID(ctx, s.banco.L(), contaID, waID)
		if err != nil {
			return err
		}
		if ok && b.DeMim {
			mudou, err := armazenamento.AvancarEstadoMensagem(ctx, s.banco.E(), b.ID, b.Estado, novo)
			if err != nil {
				return err
			}
			if mudou {
				s.publicarMensagem(ctx, eventos.MensagemAtualizada, b.ID)
			}
		}
		tipo := r.Tipo
		if tipo == whatsapp.ReciboTocado {
			tipo = whatsapp.ReciboLido
		}
		for _, o := range s.ouvintes {
			o.Recibo(contaID, waID, tipo, em)
		}
	}
	return nil
}

func (s *Servico) gravarHistorico(ctx context.Context, contaID string, lote whatsapp.LoteHistorico) error {
	s.mu.Lock()
	p, ok := s.sincronia[contaID]
	if !ok {
		p = &progresso{conversas: map[string]bool{}}
		s.sincronia[contaID] = p
	}
	s.mu.Unlock()
	if !ok {
		armazenamento.DefinirSincronizando(ctx, s.banco.E(), contaID, true)
		s.contas.PublicarConta(ctx, contaID)
	}

	conversasTocadas := map[string]bool{}
	err := s.banco.Transacao(ctx, func(tx *sql.Tx) error {
		for _, conv := range lote.Conversas {
			p.conversas[conv.Chat.String()] = true
			if conv.Chat.Grupo() && conv.Nome != "" {
				if _, _, err := armazenamento.GarantirConversa(ctx, tx, contaID, conv.Chat.String(), "grupo", conv.Nome, "", s.relogio.Agora()); err != nil {
					return err
				}
			}
			for _, m := range conv.Mensagens {
				if m.Chat.Vazio() {
					m.Chat = conv.Chat
				}
				if m.Chat.Status() {
					continue
				}
				if m.Grupo && m.NomeChat == "" {
					m.NomeChat = conv.Nome
				}
				r, err := s.gravarMensagem(ctx, tx, contaID, m, true)
				if err != nil {
					return err
				}
				if r.conversaID != "" {
					conversasTocadas[r.conversaID] = true
				}
				p.mensagens++
			}
			if conv.NaoLidas > 0 {
				if c, ok, _ := armazenamento.ConversaPorJID(ctx, tx, contaID, conv.Chat.String()); ok {
					armazenamento.DefinirNaoLidas(ctx, tx, c.ID, conv.NaoLidas)
				}
			}
			if !conv.Chat.Grupo() && conv.Nome != "" {
				tel, _ := conv.Chat.Telefone()
				armazenamento.GarantirContato(ctx, tx, contaID, armazenamento.DadosContato{JID: conv.Chat.String(), Telefone: tel, Nome: conv.Nome}, s.relogio.Agora())
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for id := range conversasTocadas {
		s.publicarConversa(ctx, id)
	}
	s.barramento.Publicar(eventos.SincronizacaoProgresso, contaID, map[string]any{
		"conversas": len(p.conversas), "mensagens": p.mensagens, "concluida": lote.Final})
	if lote.Final {
		s.mu.Lock()
		delete(s.sincronia, contaID)
		s.mu.Unlock()
		armazenamento.DefinirSincronizando(ctx, s.banco.E(), contaID, false)
		s.contas.PublicarConta(ctx, contaID)
		debug.FreeOSMemory()
	}
	return nil
}
