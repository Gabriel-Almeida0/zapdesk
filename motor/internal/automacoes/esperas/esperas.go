// Pacote esperas é o agendador persistente das automações (research.md › R9): uma goroutine
// acorda na menor `retomar_em` (relogio.Relogio injetável), trata o vencimento (≤ 24 h executa;
// > 24 h aborta "expirada"; conversa pausada aborta; conta desconectada reprograma a cada 1 min
// até completar 24 h) e calcula a próxima ocorrência de cron/intervalo a partir de agora.
package esperas

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/automacoes/execucoes"
	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/relogio"
)

// Regras de vencimento.
const (
	PrazoVencimento   = 24 * time.Hour
	IntervaloReconta  = time.Minute
	LoteProcessamento = 200
)

// Motivos de aborto.
const (
	MotivoExpirada        = "expirada"
	MotivoConversaPausada = "conversa pausada"
	MotivoInterrompida    = "interrompida"
)

// Manipulador executa ou aborta as esperas vencidas (implementado pelo despachante).
type Manipulador interface {
	Executar(ctx context.Context, e dominio.Espera) error
	Abortar(ctx context.Context, e dominio.Espera, motivo string)
}

// Verificador informa o estado da conversa da espera.
type Verificador interface {
	ConversaPausada(ctx context.Context, conversaID string) bool
	ContaConectada(ctx context.Context, conversaID string) bool
}

// Pausas vence as pausas de conversa com prazo (publicando o fim) e devolve o próximo vencimento.
type Pausas interface {
	VencerTodas(ctx context.Context) (time.Time, bool)
}

// Agendador de esperas.
type Agendador struct {
	banco    *armazenamento.Banco
	relogio  relogio.Relogio
	log      zerolog.Logger
	m        Manipulador
	v        Verificador
	pausas   Pausas
	registro *execucoes.Registro

	mu        sync.Mutex
	sujo      bool
	cancelar  context.CancelFunc
	pararLaco context.CancelFunc
	fim       chan struct{}
	passada   sync.Mutex
}

// Novo cria o agendador (pausas e registro podem ser nil).
func Novo(b *armazenamento.Banco, r relogio.Relogio, log zerolog.Logger, m Manipulador, v Verificador, p Pausas, reg *execucoes.Registro) *Agendador {
	return &Agendador{banco: b, relogio: r, log: log, m: m, v: v, pausas: p, registro: reg}
}

// Recuperar trata o que ficou da execução anterior do motor: execuções "rodando"/"na_fila" viram
// "abortada" ("interrompida"); esperas seguem a regra de vencimento na primeira passada.
func (a *Agendador) Recuperar(ctx context.Context) error {
	if a.registro == nil {
		return nil
	}
	_, err := a.registro.AbortarPendentes(ctx, MotivoInterrompida, dominio.ExecRodando, dominio.ExecNaFila)
	return err
}

// Iniciar põe a goroutine para rodar.
func (a *Agendador) Iniciar(ctx context.Context) {
	a.mu.Lock()
	if a.fim != nil {
		a.mu.Unlock()
		return
	}
	a.fim = make(chan struct{})
	ctx, a.pararLaco = context.WithCancel(ctx)
	a.mu.Unlock()
	go a.laco(ctx)
}

// Encerrar para a goroutine e espera ela sair.
func (a *Agendador) Encerrar() {
	a.mu.Lock()
	fim := a.fim
	parar := a.pararLaco
	a.sujo = true
	a.mu.Unlock()
	if fim == nil {
		return
	}
	parar()
	<-fim
}

// Acordar faz o agendador recalcular a próxima espera (nova espera, pausa, relógio).
func (a *Agendador) Acordar() {
	a.mu.Lock()
	a.sujo = true
	if a.cancelar != nil {
		a.cancelar()
	}
	a.mu.Unlock()
}

func (a *Agendador) laco(ctx context.Context) {
	defer close(a.fim)
	for ctx.Err() == nil {
		a.mu.Lock()
		a.sujo = false
		a.mu.Unlock()
		a.Passada(ctx)
		prox, ok := a.proxima(ctx)
		espCtx, cancelar := context.WithCancel(ctx)
		a.mu.Lock()
		if a.sujo {
			a.mu.Unlock()
			cancelar()
			continue
		}
		a.cancelar = cancelar
		a.mu.Unlock()
		if ok {
			a.relogio.Esperar(espCtx, prox)
		} else {
			<-espCtx.Done()
		}
		cancelar()
		a.mu.Lock()
		a.cancelar = nil
		a.mu.Unlock()
	}
}

func (a *Agendador) proxima(ctx context.Context) (time.Time, bool) {
	prox, ok, err := armazenamento.ProximaEspera(ctx, a.banco.L())
	if err != nil {
		ok = false
	}
	if a.pausas != nil {
		if p, okP := a.pausas.VencerTodas(ctx); okP && (!ok || p.Before(prox)) {
			prox, ok = p, true
		}
	}
	return prox, ok
}

// Passada trata todas as esperas vencidas agora (síncrona). Devolve quantas tratou.
func (a *Agendador) Passada(ctx context.Context) int {
	a.passada.Lock()
	defer a.passada.Unlock()
	if a.pausas != nil {
		a.pausas.VencerTodas(ctx)
	}
	total := 0
	for ctx.Err() == nil {
		vencidas, err := armazenamento.EsperasVencidas(ctx, a.banco.L(), a.relogio.Agora(), LoteProcessamento)
		if err != nil || len(vencidas) == 0 {
			return total
		}
		tratadas := 0
		for _, e := range vencidas {
			if a.tratar(ctx, e) {
				tratadas++
			}
		}
		total += tratadas
		if tratadas == 0 {
			return total // só reprogramações: o resto vence depois
		}
	}
	return total
}

// dadosVencimento guarda o vencimento original quando a espera é reprogramada (conta desconectada).
type dadosVencimento struct {
	VencidaEmMs int64 `json:"_vencida_em_ms,omitempty"`
}

func vencimentoOriginal(e dominio.Espera) time.Time {
	var d dadosVencimento
	json.Unmarshal(e.Dados, &d)
	if d.VencidaEmMs > 0 {
		return armazenamento.DeMs(d.VencidaEmMs)
	}
	return e.RetomarEm
}

// tratar decide uma espera vencida; devolve true se ela saiu da fila (executada/abortada).
func (a *Agendador) tratar(ctx context.Context, e dominio.Espera) bool {
	agora := a.relogio.Agora()
	venc := vencimentoOriginal(e)
	expirada := agora.Sub(venc) > PrazoVencimento
	comConversa := e.ConversaID != nil && *e.ConversaID != "" &&
		(e.Tipo == dominio.EsperaAguardar || e.Tipo == dominio.EsperaSemResposta || e.Tipo == dominio.EsperaAgendar)

	if !expirada && comConversa && !a.v.ConversaPausada(ctx, *e.ConversaID) && !a.v.ContaConectada(ctx, *e.ConversaID) {
		// Conta desconectada: tenta de novo em 1 min até completar 24 h do vencimento.
		novo := agora.Add(IntervaloReconta)
		var m map[string]any
		json.Unmarshal(e.Dados, &m)
		if m == nil {
			m = map[string]any{}
		}
		m["_vencida_em_ms"] = armazenamento.Ms(venc)
		dados, _ := json.Marshal(m)
		if err := armazenamento.ReprogramarEsperaComDados(ctx, a.banco.E(), e.ID, novo, dados); err != nil {
			a.log.Warn().Err(err).Msg("reprogramar espera")
		}
		return false
	}
	if ok, err := armazenamento.ApagarEspera(ctx, a.banco.E(), e.ID); err != nil || !ok {
		return false // outra passada já pegou
	}
	switch {
	case expirada:
		a.m.Abortar(ctx, e, MotivoExpirada)
	case comConversa && a.v.ConversaPausada(ctx, *e.ConversaID):
		a.m.Abortar(ctx, e, MotivoConversaPausada)
	default:
		if err := a.m.Executar(ctx, e); err != nil {
			a.log.Warn().Err(err).Str("espera", e.ID).Str("tipo", e.Tipo).Msg("espera não executou")
		}
	}
	if e.Tipo == dominio.EsperaAgendamento {
		a.reagendar(ctx, e)
	}
	return true
}

// DadosAgendamento é o conteúdo de `dados` de uma espera de agendamento.
type DadosAgendamento struct {
	GatilhoIndice int    `json:"gatilho_indice"`
	Cron          string `json:"cron,omitempty"`
	IntervaloS    int    `json:"intervalo_s,omitempty"`
}

func (a *Agendador) reagendar(ctx context.Context, e dominio.Espera) {
	var d DadosAgendamento
	json.Unmarshal(e.Dados, &d)
	g := modelo.Gatilho{Tipo: modelo.GatilhoAgendamento, Cron: d.Cron, IntervaloS: d.IntervaloS}
	if err := a.ProgramarAgendamento(ctx, e.AutomacaoID, d.GatilhoIndice, g); err != nil {
		a.log.Warn().Err(err).Msg("reagendar")
	}
}

// ProximaOcorrencia calcula a próxima execução de um gatilho de agendamento a partir de `agora`
// (cron de 5 campos no fuso local ou intervalo).
func ProximaOcorrencia(g modelo.Gatilho, agora time.Time) (time.Time, error) {
	if g.Cron != "" {
		s, err := modelo.ParserCron.Parse(g.Cron)
		if err != nil {
			return time.Time{}, err
		}
		return s.Next(agora.Local()), nil
	}
	if g.IntervaloS >= modelo.MinIntervaloS {
		return agora.Add(time.Duration(g.IntervaloS) * time.Second), nil
	}
	return time.Time{}, errors.New("agendamento sem cron nem intervalo")
}

// ProgramarAgendamento grava (substituindo) a próxima ocorrência do gatilho `indice`.
func (a *Agendador) ProgramarAgendamento(ctx context.Context, automacaoID string, indice int, g modelo.Gatilho) error {
	agora := a.relogio.Agora()
	prox, err := ProximaOcorrencia(g, agora)
	if err != nil {
		return err
	}
	dados, _ := json.Marshal(DadosAgendamento{GatilhoIndice: indice, Cron: g.Cron, IntervaloS: g.IntervaloS})
	chave := "agendamento:" + itoa(indice)
	err = armazenamento.GravarEspera(ctx, a.banco.E(), dominio.Espera{ID: ids.NovoEm(agora), Tipo: dominio.EsperaAgendamento,
		AutomacaoID: automacaoID, Chave: &chave, RetomarEm: prox, Dados: dados, CriadaEm: agora})
	if err == nil {
		a.Acordar()
	}
	return err
}

// ProgramarSemResposta substitui a espera "sem resposta" da conversa para a automação.
func (a *Agendador) ProgramarSemResposta(ctx context.Context, automacaoID, conversaID, referencia string, enviadaEm time.Time, aposS int, gatilhoIndice int) error {
	agora := a.relogio.Agora()
	chave := "sem_resposta:" + conversaID
	conv, ref := conversaID, referencia
	dados, _ := json.Marshal(map[string]any{"gatilho_indice": gatilhoIndice, "apos_s": aposS})
	err := armazenamento.GravarEspera(ctx, a.banco.E(), dominio.Espera{ID: ids.NovoEm(agora), Tipo: dominio.EsperaSemResposta,
		AutomacaoID: automacaoID, ConversaID: &conv, Referencia: &ref, Chave: &chave,
		RetomarEm: enviadaEm.Add(time.Duration(aposS) * time.Second), Dados: dados, CriadaEm: agora})
	if err == nil {
		a.Acordar()
	}
	return err
}

// CancelarSemResposta apaga as esperas "sem resposta" da conversa (o contato respondeu).
func (a *Agendador) CancelarSemResposta(ctx context.Context, conversaID string) error {
	return armazenamento.ApagarEsperasSemRespostaDaConversa(ctx, a.banco.E(), conversaID)
}

// Gravar grava uma espera qualquer e acorda o agendador.
func (a *Agendador) Gravar(ctx context.Context, e dominio.Espera) error {
	if e.ID == "" {
		e.ID = ids.NovoEm(a.relogio.Agora())
	}
	if e.CriadaEm.IsZero() {
		e.CriadaEm = a.relogio.Agora()
	}
	err := armazenamento.GravarEspera(ctx, a.banco.E(), e)
	if err == nil {
		a.Acordar()
	}
	return err
}

func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}
