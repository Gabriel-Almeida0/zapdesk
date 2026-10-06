package disparos

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"zapdesk/motor/internal/agendamento"
	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/chat"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/logs"
	"zapdesk/motor/internal/variaveis"
	"zapdesk/motor/internal/whatsapp"
)

// Motivos de falha de destinatário.
const (
	MotivoSemWhatsApp   = "Sem WhatsApp"
	MotivoEstadoIncerto = "Estado incerto após interrupção"
)

// Atrasos do executor.
var (
	// EsperaSemConexao: nova tentativa quando a conta está sem rede (também acorda ao reconectar).
	EsperaSemConexao = 30 * time.Second
	// ToleranciaAtraso: se o envio passou da hora por mais que isso (sono do Mac), recalcula a
	// agenda em vez de enviar em rajada.
	ToleranciaAtraso = time.Minute
)

// executor é a goroutine de uma conta: no máximo um disparo ativo por vez.
type executor struct {
	contaID   string
	acordar   chan struct{}
	cancelar  context.CancelFunc
	esperando bool
	ate       time.Time
}

// acordar sinaliza o executor da conta (criando-o se preciso).
func (s *Servico) acordar(contaID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.encerrando {
		return
	}
	e, ok := s.execs[contaID]
	if !ok {
		ctx, cancelar := context.WithCancel(s.ctx)
		e = &executor{contaID: contaID, acordar: make(chan struct{}, 1), cancelar: cancelar}
		s.execs[contaID] = e
		s.wg.Add(1)
		go s.executar(ctx, e)
	}
	select {
	case e.acordar <- struct{}{}:
	default:
	}
}

func (s *Servico) acordarTodos() {
	s.mu.Lock()
	var contasIDs []string
	for id := range s.execs {
		contasIDs = append(contasIDs, id)
	}
	s.mu.Unlock()
	for _, id := range contasIDs {
		s.acordar(id)
	}
}

func (s *Servico) pararExecutor(contaID string) {
	s.mu.Lock()
	if e, ok := s.execs[contaID]; ok {
		e.cancelar()
		delete(s.execs, contaID)
	}
	s.mu.Unlock()
}

// AguardarOcioso espera todos os executores estarem parados esperando (testes).
func (s *Servico) AguardarOcioso(ctx context.Context) error {
	for {
		s.mu.Lock()
		ocioso := true
		agora := s.relogio.Agora()
		for _, e := range s.execs {
			if !e.esperando || len(e.acordar) > 0 || (!e.ate.IsZero() && !e.ate.After(agora)) {
				ocioso = false
			}
		}
		s.mu.Unlock()
		if ocioso {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond):
		}
	}
}

func (s *Servico) definirEsperando(e *executor, v bool, ate time.Time) {
	s.mu.Lock()
	e.esperando, e.ate = v, ate
	s.mu.Unlock()
}

func (s *Servico) executar(ctx context.Context, e *executor) {
	defer s.wg.Done()
	for ctx.Err() == nil {
		ate, err := s.passo(ctx, e.contaID)
		if err != nil && ctx.Err() == nil {
			s.log.Error().Err(err).Str("conta", e.contaID).Msg("executor de disparos")
			ate = s.relogio.Agora().Add(5 * time.Second)
		}
		s.definirEsperando(e, true, ate)
		s.esperar(ctx, e, ate)
		s.definirEsperando(e, false, time.Time{})
	}
}

// esperar dorme até `ate` (zero = só por sinal) ou até ser acordado.
func (s *Servico) esperar(ctx context.Context, e *executor, ate time.Time) {
	select {
	case <-e.acordar:
		return
	default:
	}
	if !ate.IsZero() && !ate.After(s.relogio.Agora()) {
		return
	}
	ctxE, cancelar := context.WithCancel(ctx)
	defer cancelar()
	go func() {
		select {
		case <-e.acordar:
			cancelar()
		case <-ctxE.Done():
		}
	}()
	if ate.IsZero() {
		<-ctxE.Done()
		return
	}
	s.relogio.Esperar(ctxE, ate)
}

// escolher devolve o disparo ativo da conta (ou o próximo da fila) e quando reavaliar se não há.
func (s *Servico) escolher(ctx context.Context, contaID string) (*dominio.Disparo, time.Time, error) {
	lista, err := armazenamento.DisparosDaConta(ctx, s.banco.L(), contaID, dominio.DisparoEnviando, dominio.DisparoForaDaJanela, dominio.DisparoAgendado)
	if err != nil {
		return nil, time.Time{}, err
	}
	for _, d := range lista {
		if d.Estado != dominio.DisparoAgendado {
			x := d
			return &x, time.Time{}, nil
		}
	}
	agora := s.relogio.Agora()
	var proximo time.Time
	for _, d := range lista {
		if s.elegivel(d, agora) {
			x := d
			return &x, time.Time{}, nil
		}
		if proximo.IsZero() || d.InicioEm.Before(proximo) {
			proximo = *d.InicioEm
		}
	}
	return nil, proximo, nil
}

// passo faz no máximo um envio e devolve até quando dormir.
func (s *Servico) passo(ctx context.Context, contaID string) (time.Time, error) {
	d, reavaliar, err := s.escolher(ctx, contaID)
	if err != nil || d == nil {
		return reavaliar, err
	}
	agora := s.relogio.Agora()
	cfg := ConfigAgenda(*d)

	// Sem pendentes: conclui (depois que ninguém estiver em "enviando").
	c, err := armazenamento.ContadoresDisparo(ctx, s.banco.L(), d.ID)
	if err != nil {
		return time.Time{}, err
	}
	if c.Pendente == 0 && c.Enviando == 0 {
		de := d.Estado
		if de == dominio.DisparoAgendado {
			if _, err := s.mudar(ctx, *d, AcaoEntrarNaJanela, nil); err != nil {
				return time.Time{}, err
			}
			de = dominio.DisparoEnviando
			d.Estado = de
		}
		_, err := s.mudar(ctx, *d, AcaoConcluir, nil)
		return agora, err
	}

	// Próximo envio: calculado uma vez e guardado (o aleatório não muda a cada acordar).
	if d.ProximoEnvioEm == nil || agora.Sub(*d.ProximoEnvioEm) > ToleranciaAtraso {
		s.rndMu.Lock()
		prox, _ := agendamento.ProximoEnvio(s.estadoAgenda(ctx, *d), agora, cfg, s.rnd)
		s.rndMu.Unlock()
		if err := armazenamento.DefinirProximoEnvio(ctx, s.banco.E(), d.ID, &prox); err != nil {
			return time.Time{}, err
		}
		d.ProximoEnvioEm = &prox
		s.publicarDisparo(d.ID, false)
	}
	prox := *d.ProximoEnvioEm

	// Estado visível: fora_da_janela enquanto espera a janela; enviando no resto.
	acao := AcaoEntrarNaJanela
	if !agendamento.DentroDaJanela(agora, cfg.Janela) {
		acao = AcaoSairDaJanela
	}
	if para, _ := Transicao(d.Estado, acao); para != "" && para != d.Estado {
		if ok, err := s.mudar(ctx, *d, acao, nil); err != nil || !ok {
			return agora, err
		}
		d.Estado = para
	}
	if prox.After(agora) {
		return prox, nil
	}
	if d.Estado != dominio.DisparoEnviando {
		// passou da hora mas estamos fora da janela: recalcula já
		if err := armazenamento.DefinirProximoEnvio(ctx, s.banco.E(), d.ID, nil); err != nil {
			return time.Time{}, err
		}
		return agora, nil
	}
	return s.enviarUm(ctx, *d, cfg)
}

// enviarUm envia ao próximo destinatário pendente com commits antes e depois (Constituição V).
func (s *Servico) enviarUm(ctx context.Context, d dominio.Disparo, cfg agendamento.Config) (time.Time, error) {
	dest, ok, err := armazenamento.ProximoPendente(ctx, s.banco.L(), d.ID)
	if err != nil || !ok {
		return s.relogio.Agora(), err
	}
	cli, err := s.contas.Cliente(ctx, d.ContaID)
	if err != nil {
		// a conta caiu: o ouvinte de contas pausa o disparo; aguarda sinal
		return s.relogio.Agora().Add(EsperaSemConexao), nil
	}
	agora := s.relogio.Agora()
	marcado, err := armazenamento.MarcarEnviando(ctx, s.banco.E(), dest.ID, d.ID, agora)
	if err != nil || !marcado {
		return agora, err
	}
	s.publicarDestinatario(d.ContaID, dest.ID)

	voltar := func() (time.Time, error) {
		armazenamento.VoltarPendenteDest(ctx, s.banco.E(), dest.ID)
		s.publicarDestinatario(d.ContaID, dest.ID)
		return s.relogio.Agora().Add(EsperaSemConexao), nil
	}

	jid, tem, err := cli.TemWhatsApp(ctx, dest.Telefone)
	if err != nil {
		s.log.Warn().Err(err).Str("disparo", d.ID).Msg("verificação de WhatsApp falhou; tentando depois")
		return voltar()
	}
	armazenamento.DefinirTemWhatsApp(ctx, s.banco.E(), dest.LeadID, tem)

	registrar := func() error {
		e := s.estadoAgenda(ctx, d)
		e = agendamento.Registrar(e, agora, cfg)
		return armazenamento.RegistrarTentativa(ctx, s.banco.E(), d.ID, agora, e.EnviadosDesdePausa)
	}

	if !tem {
		armazenamento.MarcarFalhouDest(ctx, s.banco.E(), dest.ID, MotivoSemWhatsApp, agora)
		if err := registrar(); err != nil {
			return agora, err
		}
		return s.aposEnvio(ctx, d, dest.ID, false, cfg)
	}

	vals := map[string]string{}
	for k, v := range d.ValoresPadrao {
		vals[k] = v
	}
	for k, v := range dest.Variaveis {
		vals[k] = v
	}
	texto := variaveis.Resolver(d.Mensagem, vals)
	var env whatsapp.Enviada
	var midia *whatsapp.MidiaInfo
	tipo := whatsapp.TipoTexto
	arquivoID := ""
	if d.ArquivoID != nil {
		arquivoID = *d.ArquivoID
		env, midia, tipo, err = s.chat.EnviarMidiaDireta(ctx, cli, jid, arquivoID, texto)
	} else {
		env, err = cli.EnviarTexto(ctx, jid, texto, nil)
	}
	switch {
	case err == nil:
	case errors.Is(err, whatsapp.ErrEstadoIncerto):
		armazenamento.MarcarFalhouDest(ctx, s.banco.E(), dest.ID, MotivoEstadoIncerto, agora)
		registrar()
		return s.aposEnvio(ctx, d, dest.ID, true, cfg)
	case errors.Is(err, whatsapp.ErrDesconectado), errors.Is(err, whatsapp.ErrBanido):
		// nada saiu: volta a pendente (queda de rede nunca marca falhou)
		return voltar()
	case errors.Is(err, whatsapp.ErrSemWhatsApp):
		armazenamento.MarcarFalhouDest(ctx, s.banco.E(), dest.ID, MotivoSemWhatsApp, agora)
		registrar()
		return s.aposEnvio(ctx, d, dest.ID, false, cfg)
	default:
		armazenamento.MarcarFalhouDest(ctx, s.banco.E(), dest.ID, chat.MotivoErro(err), agora)
		registrar()
		return s.aposEnvio(ctx, d, dest.ID, true, cfg)
	}

	// sucesso: "enviado" + mensagem na conversa numa só transação
	var msgID, convID string
	err = s.banco.Transacao(ctx, func(tx *sql.Tx) error {
		if err := armazenamento.MarcarEnviadoDest(ctx, tx, dest.ID, env.WaID, jid.String(), env.Em); err != nil {
			return err
		}
		var err error
		msgID, convID, err = s.chat.GravarEnviadaTx(ctx, tx, chat.EnviadaDisparo{ContaID: d.ContaID, Para: jid, Envio: env,
			Texto: texto, Tipo: tipo, Midia: midia, ArquivoID: arquivoID, DisparoID: d.ID})
		return err
	})
	if err != nil {
		return agora, err
	}
	s.log.Debug().Str("disparo", d.ID).Str("destinatario", dest.ID).Str(logs.CampoConteudo, texto).Msg("disparo enviado")
	if err := registrar(); err != nil {
		return agora, err
	}
	armazenamento.DefinirFalhasSeguidas(ctx, s.banco.E(), d.ID, 0)
	s.chat.PublicarEnviada(ctx, msgID, convID)
	return s.aposEnvio(ctx, d, dest.ID, false, cfg)
}

// aposEnvio atualiza falhas seguidas, publica e agenda o próximo.
func (s *Servico) aposEnvio(ctx context.Context, d dominio.Disparo, destID string, falhou bool, cfg agendamento.Config) (time.Time, error) {
	s.publicarDestinatario(d.ContaID, destID)
	atual, err := armazenamento.ObterDisparo(ctx, s.banco.L(), d.ID)
	if err != nil {
		return time.Time{}, err
	}
	if falhou {
		n := atual.FalhasSeguidas + 1
		armazenamento.DefinirFalhasSeguidas(ctx, s.banco.E(), d.ID, n)
		if atual.FalhasSeguidasMax > 0 && n >= atual.FalhasSeguidasMax {
			m := dominio.PausaFalhasSeguidas
			s.mudar(ctx, atual, AcaoPausar, &m)
			return s.relogio.Agora(), nil
		}
	}
	c, err := armazenamento.ContadoresDisparo(ctx, s.banco.L(), d.ID)
	if err != nil {
		return time.Time{}, err
	}
	agora := s.relogio.Agora()
	if c.Pendente == 0 && c.Enviando == 0 {
		_, err := s.mudar(ctx, atual, AcaoConcluir, nil)
		return agora, err
	}
	s.rndMu.Lock()
	prox, _ := agendamento.ProximoEnvio(s.estadoAgenda(ctx, atual), agora, cfg, s.rnd)
	s.rndMu.Unlock()
	if err := armazenamento.DefinirProximoEnvio(ctx, s.banco.E(), d.ID, &prox); err != nil {
		return time.Time{}, err
	}
	s.publicarDisparo(d.ID, false)
	return prox, nil
}
