// Pacote eventos implementa o barramento pub/sub interno cujos eventos são retransmitidos pelo
// WebSocket (contracts/eventos-ws.md). O `seq` é crescente por execução do motor; assinantes
// lentos perdem eventos (o cliente percebe a lacuna em `seq` e recarrega via HTTP).
package eventos

import (
	"sync"
	"sync/atomic"
	"time"
)

// Tipos de evento de contracts/eventos-ws.md.
const (
	MotorPronto            = "motor.pronto"
	ContaAtualizada        = "conta.atualizada"
	ContaQR                = "conta.qr"
	ContaQRExpirado        = "conta.qr_expirado"
	ContaRemovida          = "conta.removida"
	SincronizacaoProgresso = "sincronizacao.progresso"
	ConversaAtualizada     = "conversa.atualizada"
	MensagemNova           = "mensagem.nova"
	MensagemAtualizada     = "mensagem.atualizada"
	ContatoAtualizado      = "contato.atualizado"
	StatusNovo             = "status.novo"
	EtiquetasAlteradas     = "etiquetas.alteradas"
	TemplatesAlterados     = "templates.alterados"
	LeadsImportados        = "leads.importados"
	DisparoAtualizado      = "disparo.atualizado"
	DestinatarioAtualizado = "destinatario.atualizado"
	DisparoFinalizado      = "disparo.finalizado"
	DisparosAtivos         = "disparos.ativos"
	ConexaoRede            = "conexao.rede"

	// Feature 002 — automações (specs/002-automacoes/contracts/eventos-ws.md).
	FunilAlterado               = "funil.alterado"
	FunilMovido                 = "funil.movido"
	AutomacaoAtualizada         = "automacao.atualizada"
	AutomacaoRemovida           = "automacao.removida"
	AutomacaoArquivosAlterados  = "automacao.arquivos_alterados"
	AutomacaoExecucaoIniciada   = "automacao.execucao.iniciada"
	AutomacaoExecucaoAtualizada = "automacao.execucao.atualizada"
	AutomacaoExecucaoFinalizada = "automacao.execucao.finalizada"
	ChatbotSessaoIniciada       = "chatbot.sessao.iniciada"
	ChatbotSessaoAtualizada     = "chatbot.sessao.atualizada"
	ChatbotSessaoFinalizada     = "chatbot.sessao.finalizada"
	ConversaPausa               = "conversa.pausa"
	Notificacao                 = "notificacao"
	SegredosAlterados           = "segredos.alterados"
	AutomacoesConfiguracao      = "automacoes.configuracao"
)

// Todos lista os tipos públicos (na ordem do contrato).
var Todos = []string{
	MotorPronto, ContaAtualizada, ContaQR, ContaQRExpirado, ContaRemovida, SincronizacaoProgresso,
	ConversaAtualizada, MensagemNova, MensagemAtualizada, ContatoAtualizado, StatusNovo,
	EtiquetasAlteradas, TemplatesAlterados, LeadsImportados, DisparoAtualizado,
	DestinatarioAtualizado, DisparoFinalizado, DisparosAtivos, ConexaoRede,
	FunilAlterado, FunilMovido, AutomacaoAtualizada, AutomacaoRemovida, AutomacaoArquivosAlterados,
	AutomacaoExecucaoIniciada, AutomacaoExecucaoAtualizada, AutomacaoExecucaoFinalizada,
	ChatbotSessaoIniciada, ChatbotSessaoAtualizada, ChatbotSessaoFinalizada, ConversaPausa,
	Notificacao, SegredosAlterados, AutomacoesConfiguracao,
}

// Evento é o envelope do contrato.
type Evento struct {
	Seq     int64     `json:"seq"`
	Tipo    string    `json:"tipo"`
	ContaID *string   `json:"conta_id"`
	Em      time.Time `json:"em"`
	Dados   any       `json:"dados"`
}

// Barramento distribui eventos para assinantes.
type Barramento struct {
	mu         sync.Mutex
	seq        int64
	assinantes map[*Assinatura]struct{}
	agora      func() time.Time
}

// Assinatura recebe eventos em C. Perdidos conta quantos foram descartados por lentidão.
type Assinatura struct {
	C        chan Evento
	perdidos atomic.Int64
	b        *Barramento
	fechada  bool
}

// Perdidos devolve quantos eventos foram descartados para esta assinatura.
func (a *Assinatura) Perdidos() int64 { return a.perdidos.Load() }

// Cancelar remove a assinatura e fecha C.
func (a *Assinatura) Cancelar() {
	a.b.mu.Lock()
	defer a.b.mu.Unlock()
	if a.fechada {
		return
	}
	a.fechada = true
	delete(a.b.assinantes, a)
	close(a.C)
}

// Novo cria um barramento. `agora` pode ser nil (time.Now).
func Novo(agora func() time.Time) *Barramento {
	if agora == nil {
		agora = time.Now
	}
	return &Barramento{assinantes: map[*Assinatura]struct{}{}, agora: agora}
}

// Assinar cria uma assinatura com o buffer informado.
func (b *Barramento) Assinar(buffer int) *Assinatura {
	if buffer <= 0 {
		buffer = 256
	}
	a := &Assinatura{C: make(chan Evento, buffer), b: b}
	b.mu.Lock()
	b.assinantes[a] = struct{}{}
	b.mu.Unlock()
	return a
}

// AssinarComSeq cria uma assinatura e devolve o último seq já emitido, atomicamente: o próximo
// evento recebido terá seq+1 (usado no motor.pronto de cada conexão WS, sem criar lacuna).
func (b *Barramento) AssinarComSeq(buffer int) (*Assinatura, int64) {
	if buffer <= 0 {
		buffer = 256
	}
	a := &Assinatura{C: make(chan Evento, buffer), b: b}
	b.mu.Lock()
	b.assinantes[a] = struct{}{}
	seq := b.seq
	b.mu.Unlock()
	return a, seq
}

// FecharTodas encerra todas as assinaturas (encerramento do motor).
func (b *Barramento) FecharTodas() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for a := range b.assinantes {
		a.fechada = true
		close(a.C)
		delete(b.assinantes, a)
	}
}

// Publicar emite um evento. contaID vazio = evento global.
func (b *Barramento) Publicar(tipo, contaID string, dados any) Evento {
	if dados == nil {
		dados = struct{}{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seq++
	ev := Evento{Seq: b.seq, Tipo: tipo, Em: b.agora(), Dados: dados}
	if contaID != "" {
		id := contaID
		ev.ContaID = &id
	}
	for a := range b.assinantes {
		select {
		case a.C <- ev:
		default:
			a.perdidos.Add(1)
		}
	}
	return ev
}

// Seq devolve o último seq emitido.
func (b *Barramento) Seq() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.seq
}
