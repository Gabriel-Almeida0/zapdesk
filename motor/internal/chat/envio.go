package chat

import (
	"context"
	"strings"
	"time"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/fatos"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/logs"
	"zapdesk/motor/internal/telefone"
	"zapdesk/motor/internal/whatsapp"
)

// TamanhoMaximoTexto de uma mensagem.
const TamanhoMaximoTexto = 4096

// Envio é o corpo de POST /v1/conversas/{id}/mensagens.
type Envio struct {
	Texto           *string `json:"texto"`
	ArquivoID       *string `json:"arquivo_id"`
	Como            string  `json:"como"`
	CitarMensagemID *string `json:"citar_mensagem_id"`

	// Automatico marca o envio feito por uma automação (nunca vem do JSON da API).
	Automatico *EnvioAutomatico `json:"-"`
}

// EnvioAutomatico identifica a automação que enviou; PrimeiroContato marca o envio que iniciou a
// conversa (conta no limite de primeiros contatos por hora).
type EnvioAutomatico struct {
	AutomacaoID     string
	PrimeiroContato bool
}

// Enviar grava a mensagem como pendente e a coloca na fila de envio da conta.
func (s *Servico) Enviar(ctx context.Context, conversaID string, e Envio) (dominio.Mensagem, error) {
	conv, err := armazenamento.ObterConversa(ctx, s.banco.L(), conversaID)
	if err != nil {
		return dominio.Mensagem{}, err
	}
	texto := ""
	if e.Texto != nil {
		texto = strings.TrimSpace(*e.Texto)
	}
	temArquivo := e.ArquivoID != nil && *e.ArquivoID != ""
	if texto == "" && !temArquivo {
		return dominio.Mensagem{}, erros.Campo("texto", "Escreva uma mensagem ou anexe um arquivo.")
	}
	if len([]rune(texto)) > TamanhoMaximoTexto {
		return dominio.Mensagem{}, erros.Campo("texto", "A mensagem pode ter até 4.096 caracteres.")
	}
	como := e.Como
	if como == "" {
		como = "auto"
	}
	if como != "auto" && como != "voz" && como != "figurinha" && como != "documento" {
		return dominio.Mensagem{}, erros.Campo("como", "Use auto, voz, figurinha ou documento.")
	}
	if _, err := s.contas.Cliente(ctx, conv.ContaID); err != nil {
		return dominio.Mensagem{}, err
	}

	agora := s.relogio.Agora()
	id := ids.NovoEm(agora)
	nova := armazenamento.NovaMensagem{
		ID: id, ContaID: conv.ContaID, ConversaID: conv.ID, WaID: "pendente:" + id, DeMim: true,
		Tipo: whatsapp.TipoTexto, Texto: texto, Estado: dominio.MsgPendente, EnviadaEm: agora,
	}
	if p, ok := s.contas.Proprio(conv.ContaID); ok {
		nova.RemetenteJID = p.String()
	}
	if e.Automatico != nil {
		nova.AutomacaoID, nova.PrimeiroContato = e.Automatico.AutomacaoID, e.Automatico.PrimeiroContato
	}
	if temArquivo {
		if err := s.prepararMidia(ctx, &nova, *e.ArquivoID, como); err != nil {
			return dominio.Mensagem{}, err
		}
	}
	if e.CitarMensagemID != nil && *e.CitarMensagemID != "" {
		citada, err := armazenamento.ObterMensagem(ctx, s.banco.L(), *e.CitarMensagemID)
		if err != nil || citada.ConversaID != conv.ID {
			return dominio.Mensagem{}, erros.Campo("citar_mensagem_id", "Mensagem citada não encontrada nesta conversa.")
		}
		nova.CitacaoWaID = citada.WaID
		if citada.Texto != nil {
			nova.CitacaoResumo = *citada.Texto
		} else {
			nova.CitacaoResumo = resumo(citada.Tipo, "")
		}
		if len([]rune(nova.CitacaoResumo)) > 200 {
			nova.CitacaoResumo = string([]rune(nova.CitacaoResumo)[:200])
		}
		if citada.RemetenteNome != nil {
			nova.CitacaoRemetenteNome = *citada.RemetenteNome
		}
	}
	if _, err := armazenamento.InserirMensagem(ctx, s.banco.E(), nova); err != nil {
		return dominio.Mensagem{}, err
	}
	armazenamento.AtualizarUltimaMensagem(ctx, s.banco.E(), conv.ID, agora, resumo(nova.Tipo, texto), 0)
	b, err := armazenamento.ObterMensagem(ctx, s.banco.L(), id)
	if err != nil {
		return dominio.Mensagem{}, err
	}
	msg := s.API(b)
	s.barramento.Publicar(eventos.MensagemNova, conv.ContaID, msg)
	s.publicarConversa(ctx, conv.ID)
	s.enfileirar(conv.ContaID, id)
	f := fatos.Fato{Tipo: fatos.MensagemEnviada, ContaID: conv.ContaID, ConversaID: conv.ID, MensagemID: id,
		Grupo: conv.Tipo == "grupo", Texto: texto, Em: agora, Origem: fatos.OrigemManual}
	if conv.ContatoID != nil {
		f.ContatoID = *conv.ContatoID
	}
	if e.Automatico != nil {
		f.Origem, f.AutomacaoID = fatos.OrigemAutomacao, e.Automatico.AutomacaoID
	}
	s.fatos.Emitir(ctx, f)
	return msg, nil
}

// Reenviar volta uma mensagem falhou para pendente e a reenvia.
func (s *Servico) Reenviar(ctx context.Context, id string) (dominio.Mensagem, error) {
	b, err := armazenamento.ObterMensagem(ctx, s.banco.L(), id)
	if err != nil {
		return dominio.Mensagem{}, err
	}
	if b.Estado != dominio.MsgFalhou {
		return dominio.Mensagem{}, erros.Transicao("Só é possível reenviar mensagens que falharam.", b.Estado)
	}
	if _, err := s.contas.Cliente(ctx, b.ContaID); err != nil {
		return dominio.Mensagem{}, err
	}
	ok, err := armazenamento.VoltarPendente(ctx, s.banco.E(), id)
	if err != nil {
		return dominio.Mensagem{}, err
	}
	if !ok {
		return dominio.Mensagem{}, erros.Transicao("Só é possível reenviar mensagens que falharam.", b.Estado)
	}
	b, err = armazenamento.ObterMensagem(ctx, s.banco.L(), id)
	if err != nil {
		return dominio.Mensagem{}, err
	}
	msg := s.API(b)
	s.barramento.Publicar(eventos.MensagemAtualizada, b.ContaID, msg)
	s.enfileirar(b.ContaID, id)
	return msg, nil
}

// enfileirar põe a mensagem na fila (uma goroutine por conta preserva a ordem).
func (s *Servico) enfileirar(contaID, mensagemID string) {
	s.mu.Lock()
	fila, ok := s.filas[contaID]
	if !ok {
		fila = make(chan string, 1024)
		s.filas[contaID] = fila
		s.wg.Add(1)
		go s.trabalhar(contaID, fila)
	}
	if s.pendentesEnvio == 0 {
		s.envioOcioso = make(chan struct{})
	}
	s.pendentesEnvio++
	s.mu.Unlock()
	fila <- mensagemID
}

func (s *Servico) trabalhar(contaID string, fila chan string) {
	defer s.wg.Done()
	for id := range fila {
		s.enviarAgora(s.ctx, id)
		s.mu.Lock()
		s.pendentesEnvio--
		if s.pendentesEnvio == 0 && s.envioOcioso != nil {
			close(s.envioOcioso)
		}
		s.mu.Unlock()
	}
}

func (s *Servico) enviarAgora(ctx context.Context, id string) {
	b, err := armazenamento.ObterMensagem(ctx, s.banco.L(), id)
	if err != nil || b.Estado != dominio.MsgPendente {
		return
	}
	ctxEnvio, cancelar := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelar()
	env, err := s.enviarBruta(ctxEnvio, b)
	if err != nil {
		s.log.Warn().Err(err).Str("mensagem", id).Msg("falha no envio")
		armazenamento.MarcarFalha(ctx, s.banco.E(), id, motivoErro(err))
	} else {
		armazenamento.MarcarEnviada(ctx, s.banco.E(), id, env.WaID, env.Em)
		s.log.Debug().Str("mensagem", id).Str(logs.CampoConteudo, ptrStr(b.Texto)).Msg("mensagem enviada")
	}
	s.publicarMensagem(ctx, eventos.MensagemAtualizada, id)
}

// enviarBruta envia texto ou mídia de uma mensagem já gravada.
func (s *Servico) enviarBruta(ctx context.Context, b armazenamento.MensagemBruta) (whatsapp.Enviada, error) {
	cli, err := s.contas.Cliente(ctx, b.ContaID)
	if err != nil {
		return whatsapp.Enviada{}, whatsapp.ErrDesconectado
	}
	para := whatsapp.ParseJID(b.ConversaJID)
	var citar *whatsapp.Citacao
	if b.Citacao != nil {
		citar = &whatsapp.Citacao{WaID: b.Citacao.WaID, Texto: b.Citacao.Resumo}
		if citada, ok, _ := armazenamento.MensagemPorWaID(ctx, s.banco.L(), b.ContaID, b.Citacao.WaID); ok {
			citar.Remetente = whatsapp.ParseJID(citada.RemetenteJID)
		}
	}
	if b.ArquivoID != "" {
		return s.enviarMidiaBruta(ctx, cli, para, b, citar)
	}
	return cli.EnviarTexto(ctx, para, ptrStr(b.Texto), citar)
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// NovaConversa abre (ou reaproveita) a conversa com um telefone em qualquer formato.
func (s *Servico) NovaConversa(ctx context.Context, contaID, bruto string) (dominio.Conversa, bool, error) {
	if _, err := armazenamento.ObterConta(ctx, s.banco.L(), contaID); err != nil {
		return dominio.Conversa{}, false, err
	}
	e164, motivo := telefone.Normalizar(bruto, "")
	if motivo != "" {
		return dominio.Conversa{}, false, erros.Campo("telefone", "Telefone inválido. Confira o DDD e o número.")
	}
	cli, err := s.contas.Cliente(ctx, contaID)
	if err != nil {
		return dominio.Conversa{}, false, err
	}
	jid := whatsapp.JIDDeTelefone(e164)
	if c, ok, err := armazenamento.ConversaPorJID(ctx, s.banco.L(), contaID, jid.String()); err != nil {
		return c, false, err
	} else if ok {
		return c, false, nil
	}
	real, tem, err := cli.TemWhatsApp(ctx, e164)
	if err != nil {
		return dominio.Conversa{}, false, erros.ComDetalhes(erros.WhatsAppErro, "O WhatsApp não respondeu. Tente de novo.", map[string]any{"motivo": motivoErro(err)})
	}
	s.banco.E().ExecContext(ctx, `UPDATE leads SET tem_whatsapp = ? WHERE telefone = ?`, tem, e164)
	if !tem {
		return dominio.Conversa{}, false, erros.Novo(erros.SemWhatsApp, "Este número não tem WhatsApp.")
	}
	if !real.Vazio() {
		jid = real
	}
	tel, _ := jid.Telefone()
	if tel == "" {
		tel = e164
	}
	agora := s.relogio.Agora()
	cid, _, err := armazenamento.GarantirContato(ctx, s.banco.E(), contaID, armazenamento.DadosContato{JID: jid.String(), Telefone: tel}, agora)
	if err != nil {
		return dominio.Conversa{}, false, err
	}
	id, criada, err := armazenamento.GarantirConversa(ctx, s.banco.E(), contaID, jid.String(), "individual", "", cid, agora)
	if err != nil {
		return dominio.Conversa{}, false, err
	}
	c, err := armazenamento.ObterConversa(ctx, s.banco.L(), id)
	if err == nil && criada {
		s.barramento.Publicar(eventos.ConversaAtualizada, contaID, c)
	}
	return c, criada, err
}

// MarcarLida zera as não lidas e envia recibo de leitura (se a conta estiver conectada).
func (s *Servico) MarcarLida(ctx context.Context, conversaID string) error {
	c, err := armazenamento.ObterConversa(ctx, s.banco.L(), conversaID)
	if err != nil {
		return err
	}
	if c.NaoLidas == 0 {
		return nil
	}
	was, rems, _ := armazenamento.WaIDsNaoLidas(ctx, s.banco.L(), conversaID, c.NaoLidas)
	if err := armazenamento.DefinirNaoLidas(ctx, s.banco.E(), conversaID, 0); err != nil {
		return err
	}
	s.publicarConversa(ctx, conversaID)
	if cli, err := s.contas.Cliente(ctx, c.ContaID); err == nil && len(was) > 0 {
		chat := whatsapp.ParseJID(c.JID)
		porRemetente := map[string][]string{}
		for i, w := range was {
			porRemetente[rems[i]] = append(porRemetente[rems[i]], w)
		}
		for rem, lista := range porRemetente {
			if err := cli.MarcarLida(ctx, chat, whatsapp.ParseJID(rem), lista, s.relogio.Agora()); err != nil {
				s.log.Debug().Err(err).Msg("recibo de leitura")
			}
		}
	}
	return nil
}
