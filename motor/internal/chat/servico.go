// Pacote chat grava e expõe conversas, mensagens, contatos, recibos, busca, mídia e status
// (US1, US4, US6). Recebe os eventos normalizados de cada conta pelo gerenciador de contas.
package chat

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/fatos"
	"zapdesk/motor/internal/relogio"
	"zapdesk/motor/internal/whatsapp"
)

// Contas é o que o chat precisa do gerenciador de contas.
type Contas interface {
	Cliente(ctx context.Context, contaID string) (whatsapp.Cliente, error)
	Proprio(contaID string) (whatsapp.JID, bool)
	PublicarConta(ctx context.Context, contaID string)
}

// Arquivos dá acesso aos anexos enviados pelo usuário.
type Arquivos interface {
	Obter(ctx context.Context, id string) (dominio.Arquivo, error)
	Abrir(a dominio.Arquivo) (io.ReadCloser, error)
}

// Ouvinte recebe fatos relevantes para os disparos.
type Ouvinte interface {
	// MensagemDoContato: mensagem recebida (não minha) de um telefone numa conta.
	MensagemDoContato(contaID, telefone string, em time.Time)
	// Recibo de uma mensagem enviada por mim.
	Recibo(contaID, waID, tipo string, em time.Time)
}

// Servico de chat.
type Servico struct {
	banco      *armazenamento.Banco
	barramento *eventos.Barramento
	relogio    relogio.Relogio
	log        zerolog.Logger
	pasta      string
	contas     Contas
	arquivos   Arquivos
	ouvintes   []Ouvinte
	fatos      fatos.Emissor

	mu          sync.Mutex
	filas       map[string]chan string
	sincronia   map[string]*progresso
	ctx         context.Context
	wg          sync.WaitGroup
	ultimaLimpa time.Time

	pendentesEnvio int           // mensagens enfileiradas ainda não tentadas (protegido por mu)
	envioOcioso    chan struct{} // fechado quando pendentesEnvio volta a 0
}

type progresso struct {
	conversas map[string]bool
	mensagens int
}

// Novo cria o serviço.
func Novo(ctx context.Context, banco *armazenamento.Banco, bar *eventos.Barramento, rel relogio.Relogio,
	log zerolog.Logger, pastaDados string, contas Contas) *Servico {
	return &Servico{banco: banco, barramento: bar, relogio: rel, log: log, pasta: pastaDados, contas: contas,
		filas: map[string]chan string{}, sincronia: map[string]*progresso{}, ctx: ctx}
}

// Banco devolve o pool de leitura (rotas que só consultam).
func (s *Servico) Banco() armazenamento.Executor { return s.banco.L() }

// DefinirArquivos liga o serviço de anexos.
func (s *Servico) DefinirArquivos(a Arquivos) { s.arquivos = a }

// AdicionarOuvinte registra um ouvinte (disparos).
func (s *Servico) AdicionarOuvinte(o Ouvinte) { s.ouvintes = append(s.ouvintes, o) }

// DefinirReceptorFatos liga o despachante de automações (fatos de mensagem recebida/enviada).
func (s *Servico) DefinirReceptorFatos(r fatos.Receptor) { s.fatos.DefinirReceptor(r) }

// Iniciar: mensagens que ficaram "pendente" numa execução anterior viram "falhou" (reenviáveis).
func (s *Servico) Iniciar(ctx context.Context) error {
	_, err := s.banco.E().ExecContext(ctx, `UPDATE mensagens SET estado = 'falhou', erro = 'Envio interrompido. Tente reenviar.'
		WHERE estado = 'pendente' AND disparo_id IS NULL`)
	return err
}

// AguardarEnvios espera as filas de envio esvaziarem (testes de integração e simulação).
func (s *Servico) AguardarEnvios(ctx context.Context) error {
	for {
		s.mu.Lock()
		if s.pendentesEnvio == 0 {
			s.mu.Unlock()
			return nil
		}
		c := s.envioOcioso
		s.mu.Unlock()
		select {
		case <-c:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Encerrar espera as filas de envio terminarem o item atual.
func (s *Servico) Encerrar() {
	s.mu.Lock()
	for id, f := range s.filas {
		close(f)
		delete(s.filas, id)
	}
	s.mu.Unlock()
	s.wg.Wait()
}

// API converte a linha bruta na Mensagem do contrato.
func (s *Servico) API(b armazenamento.MensagemBruta) dominio.Mensagem {
	m := b.Mensagem
	agora := s.relogio.Agora()
	m.PodeEditar, m.PodeApagar = PodeEditar(m, agora), PodeApagar(m, agora)
	if m.Reacoes == nil {
		m.Reacoes = []dominio.Reacao{}
	}
	return m
}

func (s *Servico) publicarMensagem(ctx context.Context, tipo, id string) {
	b, err := armazenamento.ObterMensagem(ctx, s.banco.L(), id)
	if err != nil {
		return
	}
	s.barramento.Publicar(tipo, b.ContaID, s.API(b))
}

// Resumos da lista de conversas para mensagens apagadas (iguais ao texto da bolha no app).
const (
	ResumoApagadaPorMim = "Você apagou esta mensagem"
	ResumoApagada       = "Mensagem apagada"
)

// reresumir atualiza a prévia da conversa quando a mensagem mudada (editada ou apagada) é a
// última — senão o texto antigo (inclusive o de uma mensagem apagada) continuaria na lista.
func (s *Servico) reresumir(ctx context.Context, id string) {
	b, err := armazenamento.ObterMensagem(ctx, s.banco.L(), id)
	if err != nil {
		return
	}
	texto := ""
	if b.Texto != nil {
		texto = *b.Texto
	}
	novo := resumo(b.Tipo, texto)
	if b.Apagada {
		novo = ResumoApagada
		if b.DeMim {
			novo = ResumoApagadaPorMim
		}
	}
	if mudou, err := armazenamento.ResumirSeUltima(ctx, s.banco.E(), id, novo); err == nil && mudou {
		s.publicarConversa(ctx, b.ConversaID)
	}
}

func (s *Servico) publicarConversa(ctx context.Context, id string) {
	c, err := armazenamento.ObterConversa(ctx, s.banco.L(), id)
	if err != nil {
		return
	}
	s.barramento.Publicar(eventos.ConversaAtualizada, c.ContaID, c)
}

func (s *Servico) publicarContato(ctx context.Context, id string) {
	c, err := armazenamento.ObterContato(ctx, s.banco.L(), id)
	if err != nil {
		return
	}
	s.barramento.Publicar(eventos.ContatoAtualizado, c.ContaID, c)
}

// PublicarContato reemite contato.atualizado (usado pela organização).
func (s *Servico) PublicarContato(ctx context.Context, id string) { s.publicarContato(ctx, id) }

// motivoErro traduz erros do cliente para texto exibível.
func motivoErro(err error) string {
	switch {
	case errors.Is(err, whatsapp.ErrSemWhatsApp):
		return "Este número não tem WhatsApp."
	case errors.Is(err, whatsapp.ErrDesconectado):
		return "Sem conexão com o WhatsApp."
	case errors.Is(err, whatsapp.ErrBanido):
		return "O WhatsApp bloqueou este número."
	case errors.Is(err, whatsapp.ErrForaDoPrazo):
		return "O prazo do WhatsApp para essa ação já passou."
	case errors.Is(err, whatsapp.ErrMidiaExpirada):
		return "Não foi possível baixar."
	case err == nil:
		return ""
	}
	return err.Error()
}

// MotivoErro exporta a tradução (usada pelos disparos).
func MotivoErro(err error) string { return motivoErro(err) }

// resumo da última mensagem para a lista de conversas.
func resumo(tipo, texto string) string {
	if texto != "" {
		return texto
	}
	switch tipo {
	case whatsapp.TipoImagem:
		return "Imagem"
	case whatsapp.TipoVideo:
		return "Vídeo"
	case whatsapp.TipoAudio:
		return "Áudio"
	case whatsapp.TipoDocumento:
		return "Documento"
	case whatsapp.TipoFigurinha:
		return "Figurinha"
	}
	return ""
}
