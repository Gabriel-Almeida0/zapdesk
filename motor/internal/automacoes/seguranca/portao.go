package seguranca

import (
	"context"
	"fmt"
	"sync"
	"time"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/relogio"
)

// Motivos de bloqueio (iguais a ErroBloqueado.motivo da SDK).
const (
	MotivoPausaGeral      = "pausa_geral"
	MotivoPausa           = "pausa"
	MotivoGrupo           = "grupo"
	MotivoAntiLoop        = "anti_loop"
	MotivoPrimeiroContato = "primeiro_contato"
)

// Pedido de envio automático numa conversa.
type Pedido struct {
	ConversaID    string
	AutomacaoID   string
	AutomacaoNome string
	IncluirGrupos bool
	AntiLoop      *dominio.AntiLoop // limite próprio (só vale se for mais restritivo)
}

// Decisao do portão.
type Decisao struct {
	Permitido       bool
	Motivo          string
	Mensagem        string
	PrimeiroContato bool // o envio iniciará a conversa (marcar a mensagem)
}

// Portao é o ponto único de verificação dos envios automáticos (Constituição VIII).
type Portao struct {
	banco      *armazenamento.Banco
	barramento *eventos.Barramento
	relogio    relogio.Relogio
	cfg        Configuracao
	pausas     *Pausas

	mu     sync.Mutex
	travas map[string]*sync.Mutex // uma por conta: autorizar + gravar o envio sem corrida
}

// NovoPortao cria o portão com o seu serviço de pausas.
func NovoPortao(b *armazenamento.Banco, bar *eventos.Barramento, r relogio.Relogio, cfg Configuracao) *Portao {
	return &Portao{banco: b, barramento: bar, relogio: r, cfg: cfg, pausas: NovasPausas(b, bar, r, cfg)}
}

// Pausas devolve o serviço de pausas.
func (p *Portao) Pausas() *Pausas { return p.pausas }

func negar(motivo, msg string) Decisao { return Decisao{Motivo: motivo, Mensagem: msg} }

// travaDaConta devolve a trava da conta dona da conversa ("" se a conversa não existe).
func (p *Portao) travaDaConta(ctx context.Context, conversaID string) *sync.Mutex {
	var conta string
	p.banco.L().QueryRowContext(ctx, `SELECT conta_id FROM conversas WHERE id = ?`, conversaID).Scan(&conta)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.travas == nil {
		p.travas = map[string]*sync.Mutex{}
	}
	t, ok := p.travas[conta]
	if !ok {
		t = &sync.Mutex{}
		p.travas[conta] = t
	}
	return t
}

// AutorizarEEnviar decide o envio e, se permitido, chama `enviar` (que grava a mensagem) sob a
// trava da conta. Sem a trava, execuções concorrentes liam a mesma contagem antes de qualquer
// gravação e estouravam o limite de primeiros contatos por hora e o anti-loop.
func (p *Portao) AutorizarEEnviar(ctx context.Context, pd Pedido, enviar func(Decisao) error) (Decisao, error) {
	t := p.travaDaConta(ctx, pd.ConversaID)
	t.Lock()
	defer t.Unlock()
	d := p.Autorizar(ctx, pd)
	if !d.Permitido {
		return d, nil
	}
	return d, enviar(d)
}

// Autorizar decide um envio automático (só a decisão; para enviar use AutorizarEEnviar), na ordem: pausa geral → pausa da conversa → grupo →
// anti-loop → primeiro contato. O estouro do anti-loop cria a pausa anti_loop e notifica.
func (p *Portao) Autorizar(ctx context.Context, pd Pedido) Decisao {
	cfg := p.cfg.Configuracao()
	if cfg.PausaGeral {
		return negar(MotivoPausaGeral, "Todas as automações estão pausadas.")
	}
	var conta, tipo string
	if err := p.banco.L().QueryRowContext(ctx, `SELECT conta_id, tipo FROM conversas WHERE id = ?`, pd.ConversaID).Scan(&conta, &tipo); err != nil {
		return negar(MotivoPausa, "Conversa não encontrada.")
	}
	if pz, ok := p.pausas.Obter(ctx, pd.ConversaID); ok {
		return negar(MotivoPausa, textoPausa(pz))
	}
	if tipo == "grupo" && !pd.IncluirGrupos {
		return negar(MotivoGrupo, "Automações não enviam em grupos (ative \"Incluir grupos\").")
	}
	agora := p.relogio.Agora()
	limites := []dominio.AntiLoop{{Mensagens: cfg.AntiLoopMensagens, JanelaMin: cfg.AntiLoopJanelaMin}}
	if pd.AntiLoop != nil {
		limites = append(limites, *pd.AntiLoop)
	}
	for _, l := range limites {
		n, err := armazenamento.ContarEnviosAutomaticos(ctx, p.banco.L(), pd.ConversaID, "", agora.Add(-time.Duration(l.JanelaMin)*time.Minute))
		if err == nil && n >= l.Mensagens {
			p.estourarAntiLoop(ctx, conta, pd, cfg.PausaAntiLoopMin)
			return negar(MotivoAntiLoop, fmt.Sprintf("Anti-loop: %d mensagens automáticas em %d min nesta conversa.", l.Mensagens, l.JanelaMin))
		}
	}
	var total int
	p.banco.L().QueryRowContext(ctx, `SELECT count(*) FROM mensagens WHERE conversa_id = ?`, pd.ConversaID).Scan(&total)
	if total == 0 {
		n, err := armazenamento.ContarPrimeirosContatos(ctx, p.banco.L(), conta, agora.Add(-time.Hour))
		if err != nil || n >= cfg.PrimeirosContatosHora {
			return negar(MotivoPrimeiroContato, fmt.Sprintf("Limite de %d primeiros contatos por hora atingido nesta conta.", cfg.PrimeirosContatosHora))
		}
		return Decisao{Permitido: true, PrimeiroContato: true}
	}
	return Decisao{Permitido: true}
}

func (p *Portao) estourarAntiLoop(ctx context.Context, conta string, pd Pedido, minutos int) {
	p.pausas.PausarAntiLoop(ctx, pd.ConversaID, pd.AutomacaoID, minutos)
	aut, conv := pd.AutomacaoID, pd.ConversaID
	nome := pd.AutomacaoNome
	if nome == "" {
		nome = "Uma automação"
	}
	p.barramento.Publicar(eventos.Notificacao, conta, dominio.Notificacao{
		Titulo:      "Automações pausadas nesta conversa",
		Corpo:       fmt.Sprintf("%s enviou mensagens demais em sequência. Pausei as automações por %d min.", nome, minutos),
		AutomacaoID: &aut, ConversaID: &conv, Tipo: "anti_loop"})
}

func textoPausa(pz dominio.Pausa) string {
	ate := "sem prazo"
	if pz.Ate != nil {
		ate = "até " + pz.Ate.Format("15:04")
	}
	switch pz.Motivo {
	case dominio.PausaHumano:
		return "Conversa em atendimento humano " + ate + "."
	case dominio.PausaAntiLoop:
		return "Automações pausadas nesta conversa (anti-loop) " + ate + "."
	}
	return "Automações pausadas nesta conversa " + ate + "."
}
