// Pacote contas gerencia as contas do WhatsApp: um cliente por conta, QR, estados
// (data-model.md › Conta), online/offline e remoção.
package contas

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/relogio"
	"zapdesk/motor/internal/whatsapp"
)

// NomePadrao da conta criada sem nome (substituído pelo pushname ao conectar).
const NomePadrao = "Nova conta"

// EstadoRemovida é passado aos ouvintes quando a conta é removida.
const EstadoRemovida = "removida"

// Consumidor recebe os eventos de conteúdo (mensagens, recibos, histórico...).
type Consumidor interface {
	ProcessarEvento(ctx context.Context, contaID string, ev whatsapp.Evento)
	SincronizarContatos(ctx context.Context, contaID string, cli whatsapp.Cliente)
}

// OuvinteEstado é avisado de mudanças de estado da conta (disparos pausam/cancelam).
type OuvinteEstado func(contaID, estado string)

// contadorEventos é implementado pelo cliente falso.
type contadorEventos interface{ EventosEmitidos() int64 }

type qrAtivo struct {
	codigo string
	expira time.Time
}

type sessao struct {
	cli         whatsapp.Cliente
	online      bool
	qr          *qrAtivo
	processados atomic.Int64
	cancelar    context.CancelFunc
}

// Gerenciador das contas.
type Gerenciador struct {
	banco      *armazenamento.Banco
	fabrica    whatsapp.Fabrica
	barramento *eventos.Barramento
	relogio    relogio.Relogio
	log        zerolog.Logger
	pasta      string
	consumidor Consumidor

	mu         sync.Mutex
	encerrando bool
	sessoes    map[string]*sessao
	ouvintes   []OuvinteEstado
	ctx        context.Context
	wg         sync.WaitGroup
}

// Novo cria o gerenciador. `ctx` vive enquanto o motor estiver de pé.
func Novo(ctx context.Context, banco *armazenamento.Banco, fabrica whatsapp.Fabrica, bar *eventos.Barramento,
	rel relogio.Relogio, log zerolog.Logger, pastaDados string) *Gerenciador {
	return &Gerenciador{banco: banco, fabrica: fabrica, barramento: bar, relogio: rel, log: log, pasta: pastaDados,
		sessoes: map[string]*sessao{}, ctx: ctx}
}

// Banco devolve o banco (usado por handlers de leitura simples).
func (g *Gerenciador) Banco() *armazenamento.Banco { return g.banco }

// DefinirConsumidor liga o serviço de chat.
func (g *Gerenciador) DefinirConsumidor(c Consumidor) { g.consumidor = c }

// AoMudarEstado registra um ouvinte.
func (g *Gerenciador) AoMudarEstado(o OuvinteEstado) {
	g.mu.Lock()
	g.ouvintes = append(g.ouvintes, o)
	g.mu.Unlock()
}

func (g *Gerenciador) avisar(contaID, estado string) {
	g.mu.Lock()
	lista := append([]OuvinteEstado(nil), g.ouvintes...)
	g.mu.Unlock()
	for _, o := range lista {
		o(contaID, estado)
	}
}

// ---------------------------------------------------------------------------
// Consultas
// ---------------------------------------------------------------------------

func (g *Gerenciador) completar(c dominio.Conta) dominio.Conta {
	g.mu.Lock()
	defer g.mu.Unlock()
	if s, ok := g.sessoes[c.ID]; ok && c.Estado == dominio.ContaConectada {
		c.Online = s.online
	}
	return c
}

// Listar devolve todas as contas.
func (g *Gerenciador) Listar(ctx context.Context) ([]dominio.Conta, error) {
	lista, err := armazenamento.ListarContas(ctx, g.banco.L())
	if err != nil {
		return nil, err
	}
	for i := range lista {
		lista[i] = g.completar(lista[i])
	}
	return lista, nil
}

// Obter devolve uma conta.
func (g *Gerenciador) Obter(ctx context.Context, id string) (dominio.Conta, error) {
	c, err := armazenamento.ObterConta(ctx, g.banco.L(), id)
	if err != nil {
		return c, err
	}
	return g.completar(c), nil
}

// QR devolve o QR ativo da conta.
func (g *Gerenciador) QR(ctx context.Context, id string) (string, time.Time, error) {
	if _, err := armazenamento.ObterConta(ctx, g.banco.L(), id); err != nil {
		return "", time.Time{}, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if s, ok := g.sessoes[id]; ok && s.qr != nil {
		return s.qr.codigo, s.qr.expira, nil
	}
	return "", time.Time{}, erros.Novo(erros.NaoEncontrado, "Não há QR code ativo para esta conta.")
}

// Cliente devolve o cliente da conta se ela estiver conectada.
func (g *Gerenciador) Cliente(ctx context.Context, contaID string) (whatsapp.Cliente, error) {
	c, err := armazenamento.ObterConta(ctx, g.banco.L(), contaID)
	if err != nil {
		return nil, err
	}
	if c.Estado == dominio.ContaBanida {
		return nil, erros.Novo(erros.ContaIndisponivel, "O WhatsApp bloqueou este número.")
	}
	if c.Estado != dominio.ContaConectada {
		return nil, erros.Novo(erros.ContaIndisponivel, "Conta desconectada. Reconecte para continuar.")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	s, ok := g.sessoes[contaID]
	if !ok {
		return nil, erros.Novo(erros.ContaIndisponivel, "Conta desconectada. Reconecte para continuar.")
	}
	return s.cli, nil
}

// Proprio devolve o JID da conta (se conhecido).
func (g *Gerenciador) Proprio(contaID string) (whatsapp.JID, bool) {
	g.mu.Lock()
	s, ok := g.sessoes[contaID]
	g.mu.Unlock()
	if !ok {
		return whatsapp.JID{}, false
	}
	return s.cli.Proprio()
}

// ContasConectadas conta contas no estado conectada.
func (g *Gerenciador) ContasConectadas() int {
	lista, err := armazenamento.ListarContas(context.Background(), g.banco.L())
	if err != nil {
		return 0
	}
	n := 0
	for _, c := range lista {
		if c.Estado == dominio.ContaConectada {
			n++
		}
	}
	return n
}

// AguardarEventos espera o processamento de tudo que o cliente (falso) já emitiu.
func (g *Gerenciador) AguardarEventos(ctx context.Context, contaID string) error {
	for {
		g.mu.Lock()
		s, ok := g.sessoes[contaID]
		g.mu.Unlock()
		if !ok {
			return nil
		}
		ce, ok := s.cli.(contadorEventos)
		if !ok || s.processados.Load() >= ce.EventosEmitidos() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Millisecond):
		}
	}
}

// ---------------------------------------------------------------------------
// Ações
// ---------------------------------------------------------------------------

// ValidarNome aplica a regra "1–60 caracteres".
func ValidarNome(nome string) (string, error) {
	nome = strings.TrimSpace(nome)
	n := len([]rune(nome))
	if n < 1 || n > 60 {
		return "", erros.Campo("nome", "O nome deve ter de 1 a 60 caracteres.")
	}
	return nome, nil
}

// Criar cria a conta e inicia o QR.
func (g *Gerenciador) Criar(ctx context.Context, nome *string) (dominio.Conta, error) {
	n := NomePadrao
	if nome != nil {
		v, err := ValidarNome(*nome)
		if err != nil {
			return dominio.Conta{}, err
		}
		n = v
	}
	agora := g.relogio.Agora()
	c := dominio.Conta{ID: ids.NovoEm(agora), Nome: n, Estado: dominio.ContaConectando, CriadaEm: agora}
	if err := armazenamento.CriarConta(ctx, g.banco.E(), c); err != nil {
		return c, err
	}
	g.publicarConta(ctx, c.ID)
	if err := g.abrirEConectar(c.ID); err != nil {
		g.log.Error().Err(err).Str("conta", c.ID).Msg("falha ao abrir a conta")
		g.mudarEstado(c.ID, dominio.ContaDesconectada)
	}
	return g.Obter(ctx, c.ID)
}

// Renomear muda o nome.
func (g *Gerenciador) Renomear(ctx context.Context, id, nome string) (dominio.Conta, error) {
	v, err := ValidarNome(nome)
	if err != nil {
		return dominio.Conta{}, err
	}
	if _, err := armazenamento.ObterConta(ctx, g.banco.L(), id); err != nil {
		return dominio.Conta{}, err
	}
	if err := armazenamento.RenomearConta(ctx, g.banco.E(), id, v, g.relogio.Agora()); err != nil {
		return dominio.Conta{}, err
	}
	g.publicarConta(ctx, id)
	return g.Obter(ctx, id)
}

// Reconectar fecha o cliente atual e abre outro (novo QR se a sessão foi perdida).
func (g *Gerenciador) Reconectar(ctx context.Context, id string) (dominio.Conta, error) {
	if _, err := armazenamento.ObterConta(ctx, g.banco.L(), id); err != nil {
		return dominio.Conta{}, err
	}
	g.fecharSessao(id)
	g.mudarEstado(id, dominio.ContaConectando)
	if err := g.abrirEConectar(id); err != nil {
		g.log.Error().Err(err).Str("conta", id).Msg("falha ao reconectar")
		g.mudarEstado(id, dominio.ContaDesconectada)
	}
	return g.Obter(ctx, id)
}

// Remover faz logout, apaga a sessão, os dados e as mídias da conta.
func (g *Gerenciador) Remover(ctx context.Context, id string) error {
	if _, err := armazenamento.ObterConta(ctx, g.banco.L(), id); err != nil {
		return err
	}
	g.avisar(id, EstadoRemovida)
	g.fecharSessao(id)
	if err := g.fabrica.Remover(ctx, id); err != nil {
		g.log.Warn().Err(err).Str("conta", id).Msg("falha no logout ao remover conta")
	}
	if err := armazenamento.RemoverConta(ctx, g.banco.E(), id); err != nil {
		return err
	}
	os.RemoveAll(filepath.Join(g.pasta, "midia", id))
	g.barramento.Publicar(eventos.ContaRemovida, id, map[string]string{"id": id})
	return nil
}

// Iniciar abre, em segundo plano, os clientes das contas existentes (não atrasa o "pronto").
// A lista é lida antes de retornar, para não disputar com contas criadas logo depois.
func (g *Gerenciador) Iniciar() {
	lista, err := armazenamento.ListarContas(g.ctx, g.banco.L())
	if err != nil {
		g.log.Error().Err(err).Msg("listar contas no início")
		return
	}
	// O estado "conectada" gravado é da execução anterior: até a sessão reabrir, a conta está
	// "conectando". Sem isto, GET /v1/contas dizia "conectada" sem cliente (envios e esperas
	// falhavam com "Conta desconectada" logo após reiniciar).
	for i, c := range lista {
		if c.Estado == dominio.ContaConectada {
			if err := armazenamento.AtualizarEstadoConta(g.ctx, g.banco.E(), c.ID, dominio.ContaConectando, g.relogio.Agora()); err == nil {
				lista[i].Estado = dominio.ContaConectando
			}
		}
	}
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		for _, c := range lista {
			if c.Estado == dominio.ContaBanida {
				continue
			}
			g.mu.Lock()
			_, jaAberta := g.sessoes[c.ID]
			g.mu.Unlock()
			if jaAberta {
				continue
			}
			cli, err := g.fabrica.Abrir(g.ctx, c.ID)
			if err != nil {
				g.log.Error().Err(err).Str("conta", c.ID).Msg("abrir conta no início")
				g.mudarEstado(c.ID, dominio.ContaDesconectada)
				continue
			}
			if !cli.Pareado() {
				cli.Desconectar()
				g.mudarEstado(c.ID, dominio.ContaDesconectada)
				continue
			}
			g.mudarEstado(c.ID, dominio.ContaConectando)
			s := g.registrarSessao(c.ID, cli)
			g.conectarComRetentativa(c.ID, s)
		}
	}()
}

// Encerrar desconecta todos os clientes.
func (g *Gerenciador) Encerrar() {
	g.mu.Lock()
	g.encerrando = true
	idsS := make([]string, 0, len(g.sessoes))
	for id := range g.sessoes {
		idsS = append(idsS, id)
	}
	g.mu.Unlock()
	for _, id := range idsS {
		g.fecharSessao(id)
	}
	g.wg.Wait()
}

// ---------------------------------------------------------------------------
// Internos
// ---------------------------------------------------------------------------

func (g *Gerenciador) abrirEConectar(id string) error {
	cli, err := g.fabrica.Abrir(g.ctx, id)
	if err != nil {
		return err
	}
	s := g.registrarSessao(id, cli)
	if cli.Pareado() {
		g.conectarComRetentativa(id, s)
		return nil
	}
	qr, err := cli.CanalQR(g.ctx)
	if err != nil {
		return err
	}
	g.wg.Add(1)
	go g.consumirQR(id, s, qr)
	return cli.Conectar(g.ctx)
}

// conectarComRetentativa tenta conectar; sem rede, a conta fica conectada e offline e o motor
// tenta de novo (queda de rede não desconecta a conta).
func (g *Gerenciador) conectarComRetentativa(id string, s *sessao) {
	if !g.sessaoAtual(id, s) {
		return // encerrando ou sessão já substituída
	}
	if err := s.cli.Conectar(g.ctx); err == nil {
		return
	} else {
		g.log.Warn().Err(err).Str("conta", id).Msg("sem conexão ao restaurar sessão; tentando de novo")
	}
	g.mudarEstado(id, dominio.ContaConectada)
	ctx, cancelar := context.WithCancel(g.ctx)
	g.mu.Lock()
	s.online = false
	anterior := s.cancelar
	s.cancelar = func() {
		cancelar()
		if anterior != nil {
			anterior()
		}
	}
	g.mu.Unlock()
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		espera := 5 * time.Second
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(espera):
			}
			if !g.sessaoAtual(id, s) {
				return
			}
			if err := s.cli.Conectar(ctx); err == nil {
				return
			}
			if espera < time.Minute {
				espera *= 2
			}
		}
	}()
}

func (g *Gerenciador) registrarSessao(id string, cli whatsapp.Cliente) *sessao {
	ctx, cancelar := context.WithCancel(g.ctx)
	s := &sessao{cli: cli, cancelar: cancelar}
	g.mu.Lock()
	if g.encerrando {
		g.mu.Unlock()
		cancelar()
		cli.Desconectar()
		return s
	}
	g.sessoes[id] = s
	g.mu.Unlock()
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		for ev := range cli.Eventos() {
			g.tratar(ctx, id, s, ev)
			s.processados.Add(1)
		}
	}()
	return s
}

func (g *Gerenciador) fecharSessao(id string) {
	g.mu.Lock()
	s, ok := g.sessoes[id]
	delete(g.sessoes, id)
	g.mu.Unlock()
	if ok {
		if s.cancelar != nil {
			s.cancelar()
		}
		s.cli.Desconectar()
	}
}

func (g *Gerenciador) sessaoAtual(id string, s *sessao) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.sessoes[id] == s
}

func (g *Gerenciador) consumirQR(id string, s *sessao, qr <-chan whatsapp.EventoQR) {
	defer g.wg.Done()
	for item := range qr {
		if !g.sessaoAtual(id, s) {
			s.processados.Add(1)
			continue
		}
		switch item.Tipo {
		case whatsapp.QRCodigo:
			expira := g.relogio.Agora().Add(item.Expira)
			g.mu.Lock()
			s.qr = &qrAtivo{codigo: item.Codigo, expira: expira}
			g.mu.Unlock()
			g.barramento.Publicar(eventos.ContaQR, id, map[string]any{"codigo": item.Codigo, "expira_em": expira})
		case whatsapp.QRSucesso:
			g.mu.Lock()
			s.qr = nil
			g.mu.Unlock()
		case whatsapp.QRExpirado, whatsapp.QRErro:
			g.mu.Lock()
			s.qr = nil
			g.mu.Unlock()
			if item.Erro != nil {
				g.log.Warn().Err(item.Erro).Str("conta", id).Msg("erro no QR")
			}
			g.mudarEstado(id, dominio.ContaDesconectada)
			g.barramento.Publicar(eventos.ContaQRExpirado, id, struct{}{})
		}
		s.processados.Add(1)
	}
}

func (g *Gerenciador) tratar(ctx context.Context, id string, s *sessao, ev whatsapp.Evento) {
	if ev.Estado == nil {
		if g.consumidor != nil {
			g.consumidor.ProcessarEvento(ctx, id, ev)
		}
		return
	}
	e := ev.Estado
	switch e.Estado {
	case whatsapp.EstadoConectada:
		agora := g.relogio.Agora()
		tel, _ := e.Proprio.Telefone()
		if err := armazenamento.AtualizarIdentidadeConta(ctx, g.banco.E(), id, e.Proprio.String(), tel, agora); err != nil {
			g.log.Error().Err(err).Msg("gravar identidade da conta")
		}
		if e.PushName != "" {
			if c, err := armazenamento.ObterConta(ctx, g.banco.L(), id); err == nil && c.Nome == NomePadrao {
				nome := e.PushName
				if len([]rune(nome)) > 60 {
					nome = string([]rune(nome)[:60])
				}
				armazenamento.RenomearConta(ctx, g.banco.E(), id, nome, agora)
			}
		}
		g.mu.Lock()
		s.online = true
		s.qr = nil
		g.mu.Unlock()
		g.mudarEstado(id, dominio.ContaConectada)
		if g.consumidor != nil {
			g.consumidor.SincronizarContatos(ctx, id, s.cli)
		}
	case whatsapp.EstadoDesconectada, whatsapp.EstadoSubstituida:
		g.mu.Lock()
		s.online = false
		g.mu.Unlock()
		g.mudarEstado(id, dominio.ContaDesconectada)
	case whatsapp.EstadoBanida:
		g.mu.Lock()
		s.online = false
		g.mu.Unlock()
		g.mudarEstado(id, dominio.ContaBanida)
	case whatsapp.EstadoRedeCaiu, whatsapp.EstadoRedeVoltou:
		online := e.Estado == whatsapp.EstadoRedeVoltou
		g.mu.Lock()
		mudou := s.online != online
		s.online = online
		g.mu.Unlock()
		if mudou {
			g.barramento.Publicar(eventos.ConexaoRede, id, map[string]any{"conta_id": id, "online": online})
			g.publicarConta(ctx, id)
			if online {
				g.avisar(id, dominio.ContaConectada)
			}
		}
	}
}

// mudarEstado grava, publica conta.atualizada e avisa os ouvintes.
func (g *Gerenciador) mudarEstado(id, estado string) {
	ctx := context.Background()
	atual, err := armazenamento.ObterConta(ctx, g.banco.L(), id)
	if errors.Is(err, nil) && atual.Estado == estado {
		g.publicarConta(ctx, id)
		g.avisar(id, estado)
		return
	}
	if err != nil {
		return
	}
	if err := armazenamento.AtualizarEstadoConta(ctx, g.banco.E(), id, estado, g.relogio.Agora()); err != nil {
		g.log.Error().Err(err).Msg("gravar estado da conta")
		return
	}
	g.publicarConta(ctx, id)
	g.avisar(id, estado)
}

func (g *Gerenciador) publicarConta(ctx context.Context, id string) {
	if c, err := g.Obter(ctx, id); err == nil {
		g.barramento.Publicar(eventos.ContaAtualizada, id, c)
	}
}

// PublicarConta reemite conta.atualizada (ex.: mudança em sincronizando).
func (g *Gerenciador) PublicarConta(ctx context.Context, id string) { g.publicarConta(ctx, id) }
