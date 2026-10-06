package chat

import (
	"context"
	"strings"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/whatsapp"
)

func (s *Servico) clienteDaMensagem(ctx context.Context, id string) (armazenamento.MensagemBruta, whatsapp.Cliente, error) {
	b, err := armazenamento.ObterMensagem(ctx, s.banco.L(), id)
	if err != nil {
		return b, nil, err
	}
	cli, err := s.contas.Cliente(ctx, b.ContaID)
	return b, cli, err
}

func erroWhatsApp(err error) error {
	if err == nil {
		return nil
	}
	if erros.Eh(err, erros.ContaIndisponivel) {
		return err
	}
	return erros.ComDetalhes(erros.WhatsAppErro, "O WhatsApp não respondeu. Tente de novo.", map[string]any{"motivo": motivoErro(err)})
}

// Reagir envia (ou remove, com emoji vazio) a minha reação.
func (s *Servico) Reagir(ctx context.Context, id, emoji string) error {
	b, cli, err := s.clienteDaMensagem(ctx, id)
	if err != nil {
		return err
	}
	if b.Apagada || strings.HasPrefix(b.WaID, "pendente:") {
		return erros.Transicao("Não é possível reagir a esta mensagem.", b.Estado)
	}
	if len([]rune(emoji)) > 10 {
		return erros.Campo("emoji", "Use um único emoji.")
	}
	proprio, _ := s.contas.Proprio(b.ContaID)
	if err := cli.Reagir(ctx, whatsapp.ParseJID(b.ConversaJID), whatsapp.ParseJID(b.RemetenteJID), b.WaID, emoji); err != nil {
		return erroWhatsApp(err)
	}
	if err := armazenamento.DefinirReacao(ctx, s.banco.E(), id, proprio.String(), emoji, true, s.relogio.Agora()); err != nil {
		return err
	}
	s.publicarMensagem(ctx, eventos.MensagemAtualizada, id)
	return nil
}

// Editar troca o texto de uma mensagem minha (até 15 min).
func (s *Servico) Editar(ctx context.Context, id, texto string) (dominio.Mensagem, error) {
	texto = strings.TrimSpace(texto)
	if texto == "" || len([]rune(texto)) > TamanhoMaximoTexto {
		return dominio.Mensagem{}, erros.Campo("texto", "O texto deve ter de 1 a 4.096 caracteres.")
	}
	b, err := armazenamento.ObterMensagem(ctx, s.banco.L(), id)
	if err != nil {
		return dominio.Mensagem{}, err
	}
	if !b.DeMim || b.Tipo != whatsapp.TipoTexto || b.Apagada || !enviadaComSucesso(b.Mensagem) {
		return dominio.Mensagem{}, erros.Transicao("Só é possível editar mensagens de texto enviadas por você.", b.Estado)
	}
	if s.relogio.Agora().Sub(b.EnviadaEm) > PrazoEditar {
		return dominio.Mensagem{}, erros.Novo(erros.ForaDoPrazo, "Só é possível editar até 15 minutos após o envio.")
	}
	cli, err := s.contas.Cliente(ctx, b.ContaID)
	if err != nil {
		return dominio.Mensagem{}, err
	}
	if err := cli.Editar(ctx, whatsapp.ParseJID(b.ConversaJID), b.WaID, texto); err != nil {
		return dominio.Mensagem{}, erroWhatsApp(err)
	}
	if err := armazenamento.EditarTexto(ctx, s.banco.E(), id, texto); err != nil {
		return dominio.Mensagem{}, err
	}
	b, err = armazenamento.ObterMensagem(ctx, s.banco.L(), id)
	if err != nil {
		return dominio.Mensagem{}, err
	}
	msg := s.API(b)
	s.barramento.Publicar(eventos.MensagemAtualizada, b.ContaID, msg)
	s.reresumir(ctx, id)
	return msg, nil
}

// Apagar apaga para todos uma mensagem minha (até 48 h).
func (s *Servico) Apagar(ctx context.Context, id string) error {
	b, err := armazenamento.ObterMensagem(ctx, s.banco.L(), id)
	if err != nil {
		return err
	}
	if !b.DeMim || b.Apagada || !enviadaComSucesso(b.Mensagem) {
		return erros.Transicao("Só é possível apagar mensagens enviadas por você.", b.Estado)
	}
	if s.relogio.Agora().Sub(b.EnviadaEm) > PrazoApagar {
		return erros.Novo(erros.ForaDoPrazo, "Só é possível apagar para todos até 48 horas após o envio.")
	}
	cli, err := s.contas.Cliente(ctx, b.ContaID)
	if err != nil {
		return err
	}
	if err := cli.Apagar(ctx, whatsapp.ParseJID(b.ConversaJID), whatsapp.ParseJID(b.RemetenteJID), b.WaID); err != nil {
		return erroWhatsApp(err)
	}
	if err := armazenamento.MarcarApagada(ctx, s.banco.E(), id); err != nil {
		return err
	}
	s.publicarMensagem(ctx, eventos.MensagemAtualizada, id)
	s.reresumir(ctx, id)
	return nil
}

func (s *Servico) aplicarReacao(ctx context.Context, contaID string, r whatsapp.Reacao) error {
	b, ok, err := armazenamento.MensagemPorWaID(ctx, s.banco.L(), contaID, r.WaIDAlvo)
	if err != nil || !ok {
		return err
	}
	rem := r.Remetente.String()
	if r.DeMim {
		if p, ok := s.contas.Proprio(contaID); ok {
			rem = p.String()
		}
	}
	if err := armazenamento.DefinirReacao(ctx, s.banco.E(), b.ID, rem, r.Emoji, r.DeMim, r.Em); err != nil {
		return err
	}
	s.publicarMensagem(ctx, eventos.MensagemAtualizada, b.ID)
	return nil
}

func (s *Servico) aplicarEdicao(ctx context.Context, contaID string, e whatsapp.Edicao) error {
	b, ok, err := armazenamento.MensagemPorWaID(ctx, s.banco.L(), contaID, e.WaIDAlvo)
	if err != nil || !ok {
		return err
	}
	if err := armazenamento.EditarTexto(ctx, s.banco.E(), b.ID, e.NovoTexto); err != nil {
		return err
	}
	s.publicarMensagem(ctx, eventos.MensagemAtualizada, b.ID)
	s.reresumir(ctx, b.ID)
	return nil
}

func (s *Servico) aplicarRevogacao(ctx context.Context, contaID string, r whatsapp.Revogacao) error {
	b, ok, err := armazenamento.MensagemPorWaID(ctx, s.banco.L(), contaID, r.WaIDAlvo)
	if err != nil || !ok {
		return err
	}
	if err := armazenamento.MarcarApagada(ctx, s.banco.E(), b.ID); err != nil {
		return err
	}
	s.publicarMensagem(ctx, eventos.MensagemAtualizada, b.ID)
	s.reresumir(ctx, b.ID)
	return nil
}
