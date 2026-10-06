// Pacote seguranca implementa o portão único de envio das automações (pausa geral → pausa da
// conversa → grupo → anti-loop → primeiro contato; research.md › R10) e o serviço de pausas de
// conversa (humano, anti-loop, manual) com vencimento preguiçoso e o evento conversa.pausa.
package seguranca

import (
	"context"
	"sync"
	"time"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/relogio"
)

// Configuracao fornece os ajustes atuais (Ajustes → Automações).
type Configuracao interface {
	Configuracao() dominio.ConfiguracaoAutomacoes
}

// Pausas de conversa.
type Pausas struct {
	banco      *armazenamento.Banco
	barramento *eventos.Barramento
	relogio    relogio.Relogio
	cfg        Configuracao

	mu       sync.Mutex
	aoHumano []func(ctx context.Context, conversaID, motivo string)
	aoMudar  []func()
}

// NovasPausas cria o serviço.
func NovasPausas(b *armazenamento.Banco, bar *eventos.Barramento, r relogio.Relogio, cfg Configuracao) *Pausas {
	return &Pausas{banco: b, barramento: bar, relogio: r, cfg: cfg}
}

// AoAtendimentoHumano registra quem precisa saber que a conversa passou a atendimento humano
// (o serviço de chatbot encerra a sessão ativa como "humano"). `motivo` é humano ou manual.
func (p *Pausas) AoAtendimentoHumano(fn func(ctx context.Context, conversaID, motivo string)) {
	p.mu.Lock()
	p.aoHumano = append(p.aoHumano, fn)
	p.mu.Unlock()
}

// AoMudar registra quem precisa ser acordado quando uma pausa com prazo é gravada (agendador).
func (p *Pausas) AoMudar(fn func()) {
	p.mu.Lock()
	p.aoMudar = append(p.aoMudar, fn)
	p.mu.Unlock()
}

func (p *Pausas) contaDaConversa(ctx context.Context, conversaID string) string {
	var conta string
	p.banco.L().QueryRowContext(ctx, `SELECT conta_id FROM conversas WHERE id = ?`, conversaID).Scan(&conta)
	return conta
}

func (p *Pausas) publicar(ctx context.Context, conversaID string, pausa *dominio.Pausa) {
	p.barramento.Publicar(eventos.ConversaPausa, p.contaDaConversa(ctx, conversaID), map[string]any{"conversa_id": conversaID, "pausa": pausa})
}

// Obter devolve a pausa válida da conversa. Pausa vencida é apagada (e o fim é publicado).
func (p *Pausas) Obter(ctx context.Context, conversaID string) (dominio.Pausa, bool) {
	pz, ok, err := armazenamento.ObterPausa(ctx, p.banco.L(), conversaID)
	if err != nil || !ok {
		return pz, false
	}
	if pz.Ate != nil && !pz.Ate.After(p.relogio.Agora()) {
		p.vencer(ctx, conversaID)
		return dominio.Pausa{}, false
	}
	return pz, true
}

// vencer apaga uma pausa vencida (se ainda estiver vencida) e publica conversa.pausa null.
func (p *Pausas) vencer(ctx context.Context, conversaID string) {
	r, err := p.banco.E().ExecContext(ctx, `DELETE FROM pausas_conversa WHERE conversa_id = ? AND ate IS NOT NULL AND ate <= ?`,
		conversaID, armazenamento.Ms(p.relogio.Agora()))
	if err != nil {
		return
	}
	if n, _ := r.RowsAffected(); n == 1 {
		p.publicar(ctx, conversaID, nil)
	}
}

// VencerTodas apaga as pausas vencidas (chamado pelo agendador) e devolve o próximo vencimento.
func (p *Pausas) VencerTodas(ctx context.Context) (time.Time, bool) {
	vencidas, _ := armazenamento.PausasVencidas(ctx, p.banco.L(), p.relogio.Agora())
	for _, c := range vencidas {
		p.vencer(ctx, c)
	}
	prox, ok, _ := armazenamento.ProximoVencimentoPausa(ctx, p.banco.L())
	return prox, ok
}

// gravarSeMaisLonga grava a pausa se não houver outra válida que termine depois (ou sem prazo).
func (p *Pausas) gravarSeMaisLonga(ctx context.Context, nova dominio.Pausa) (dominio.Pausa, bool) {
	if atual, ok := p.Obter(ctx, nova.ConversaID); ok {
		if atual.Ate == nil || (nova.Ate != nil && !nova.Ate.After(*atual.Ate)) {
			return atual, false
		}
	}
	return nova, p.gravar(ctx, nova)
}

func (p *Pausas) gravar(ctx context.Context, pz dominio.Pausa) bool {
	if err := armazenamento.GravarPausa(ctx, p.banco.E(), pz); err != nil {
		return false
	}
	p.publicar(ctx, pz.ConversaID, &pz)
	p.mu.Lock()
	fs := append([]func(){}, p.aoMudar...)
	p.mu.Unlock()
	for _, f := range fs {
		f()
	}
	return true
}

func (p *Pausas) avisarHumano(ctx context.Context, conversaID, motivo string) {
	p.mu.Lock()
	fs := append([]func(context.Context, string, string){}, p.aoHumano...)
	p.mu.Unlock()
	for _, f := range fs {
		f(ctx, conversaID, motivo)
	}
}

// PausarHumano cria/renova a pausa "humano" de pausa_humana_min (mensagem manual na conversa).
func (p *Pausas) PausarHumano(ctx context.Context, conversaID string) dominio.Pausa {
	agora := p.relogio.Agora()
	ate := agora.Add(time.Duration(p.cfg.Configuracao().PausaHumanaMin) * time.Minute)
	pz, _ := p.gravarSeMaisLonga(ctx, dominio.Pausa{ConversaID: conversaID, Motivo: dominio.PausaHumano, Ate: &ate, CriadaEm: agora})
	p.avisarHumano(ctx, conversaID, dominio.PausaHumano)
	return pz
}

// PausarAntiLoop cria a pausa "anti_loop" de `minutos` causada pela automação.
func (p *Pausas) PausarAntiLoop(ctx context.Context, conversaID, automacaoID string, minutos int) dominio.Pausa {
	agora := p.relogio.Agora()
	ate := agora.Add(time.Duration(minutos) * time.Minute)
	aut := automacaoID
	pz, _ := p.gravarSeMaisLonga(ctx, dominio.Pausa{ConversaID: conversaID, Motivo: dominio.PausaAntiLoop, Ate: &ate, AutomacaoID: &aut, CriadaEm: agora})
	return pz
}

// PausarPorAutomacao é a ação "pausar_automacoes" (motivo manual; nil = sem prazo).
func (p *Pausas) PausarPorAutomacao(ctx context.Context, conversaID, automacaoID string, minutos *int) dominio.Pausa {
	agora := p.relogio.Agora()
	pz := dominio.Pausa{ConversaID: conversaID, Motivo: dominio.PausaManual, CriadaEm: agora}
	if automacaoID != "" {
		a := automacaoID
		pz.AutomacaoID = &a
	}
	if minutos != nil {
		ate := agora.Add(time.Duration(*minutos) * time.Minute)
		pz.Ate = &ate
	}
	r, _ := p.gravarSeMaisLonga(ctx, pz)
	return r
}

// Assumir é a ação explícita do operador (POST /conversas/{id}/pausa): substitui a pausa atual
// (nil = sem prazo) e encerra a sessão de chatbot como "humano".
func (p *Pausas) Assumir(ctx context.Context, conversaID, motivo string, duracaoMin *int) dominio.Pausa {
	agora := p.relogio.Agora()
	pz := dominio.Pausa{ConversaID: conversaID, Motivo: motivo, CriadaEm: agora}
	if duracaoMin != nil {
		ate := agora.Add(time.Duration(*duracaoMin) * time.Minute)
		pz.Ate = &ate
	}
	p.gravar(ctx, pz)
	p.avisarHumano(ctx, conversaID, motivo)
	return pz
}

// Retomar remove a pausa ("Devolver às automações"/"Retomar").
func (p *Pausas) Retomar(ctx context.Context, conversaID string) {
	if ok, err := armazenamento.ApagarPausa(ctx, p.banco.E(), conversaID); err == nil && ok {
		p.publicar(ctx, conversaID, nil)
	}
}

// Listar as pausas válidas (opcionalmente de um motivo).
func (p *Pausas) Listar(ctx context.Context, motivo string) ([]dominio.Pausa, error) {
	return armazenamento.ListarPausas(ctx, p.banco.L(), motivo, p.relogio.Agora())
}
