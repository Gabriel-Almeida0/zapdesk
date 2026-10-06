package runner

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"zapdesk/motor/internal/relogio"
)

// Padrões (data-model.md › Configuração).
const (
	PadraoMaxProcessos = 4
	PadraoOciosidade   = 5 * time.Minute
	PadraoPrazo        = 60 * time.Second
)

// Opcoes do pool.
type Opcoes struct {
	Exec, Script string
	Relogio      relogio.Relogio
	Log          zerolog.Logger
	MaxProcessos int
	Ociosidade   time.Duration
	Fuso         string // vazio = fuso local do motor
}

// Automacao é o que o pool precisa para subir e inicializar o processo.
type Automacao struct {
	ID, Nome      string
	Versao        int
	Hash, Bundle  string
	Permissoes    []string
	Segredos      map[string]string // só os declarados e existentes (nunca ANTHROPIC_API_KEY)
	NomesSegredos []string          // nomes declarados (para reiniciar quando mudarem)
	MemoriaMB     int
}

// Pedido de execução de um handler.
type Pedido struct {
	ExecucaoID, Handler string
	Simulacao           bool
	Prazo               time.Duration
	Info, Conversa      json.RawMessage
	Argumento           json.RawMessage
}

// Ponte atende as chamadas ctx.* e as notificações de uma execução.
type Ponte interface {
	Chamar(ctx context.Context, execucaoID, metodo string, params json.RawMessage) (json.RawMessage, *ErroRPC)
	Log(execucaoID *string, nivel, texto string, em time.Time)
	HTTP(execucaoID string, metodo, urlSemQuery string, status *int, duracaoMs int64, erro *string)
}

// Pool de processos runner.
type Pool struct {
	o Opcoes

	mu        sync.Mutex
	procs     map[string]*processo   // processos utilizáveis por chave (automação|hash)
	todos     map[*processo]struct{} // processos com PID vivo (inclui os que estão saindo)
	execProc  map[string]*processo
	fila      []chan struct{}
	encerrado bool
	vivos     int
}

// NovoPool cria o pool (nada é iniciado até a primeira execução).
func NovoPool(o Opcoes) *Pool {
	if o.Relogio == nil {
		o.Relogio = relogio.Real{}
	}
	if o.MaxProcessos <= 0 {
		o.MaxProcessos = PadraoMaxProcessos
	}
	if o.Ociosidade <= 0 {
		o.Ociosidade = PadraoOciosidade
	}
	return &Pool{o: o, procs: map[string]*processo{}, todos: map[*processo]struct{}{}, execProc: map[string]*processo{}}
}

// Disponivel indica se há executável e script do runner.
func (p *Pool) Disponivel() bool { return p.o.Exec != "" && p.o.Script != "" }

// Configurar muda limite de processos e ociosidade (Ajustes → Automações).
func (p *Pool) Configurar(maxProcessos int, ociosidade time.Duration) {
	p.mu.Lock()
	if maxProcessos > 0 {
		p.o.MaxProcessos = maxProcessos
	}
	if ociosidade > 0 {
		p.o.Ociosidade = ociosidade
	}
	p.acordarFilaSemTrava()
	p.mu.Unlock()
}

// Vivos devolve quantos processos runner estão vivos.
func (p *Pool) Vivos() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.vivos
}

func (p *Pool) registrarVivo(pr *processo) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.todos[pr] = struct{}{}
	p.vivos++
	return !p.encerrado
}

// aoMorrer é chamado quando o processo terminou (depois do Wait).
func (p *Pool) aoMorrer(pr *processo) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.todos[pr]; ok {
		delete(p.todos, pr)
		p.vivos--
	}
	if p.procs[pr.chave] == pr {
		delete(p.procs, pr.chave)
	}
	if pr.cancelOcioso != nil {
		pr.cancelOcioso()
		pr.cancelOcioso = nil
	}
	p.acordarFilaSemTrava()
}

func (p *Pool) acordarFilaSemTrava() {
	if len(p.fila) > 0 {
		close(p.fila[0])
		p.fila = p.fila[1:]
	}
}

// retirarSemTrava tira o processo do uso: fecha já se ocioso, senão ao ficar ocioso.
func (p *Pool) retirarSemTrava(pr *processo) {
	if p.procs[pr.chave] == pr {
		delete(p.procs, pr.chave)
	}
	pr.obsoleto = true
	if pr.ocupado == 0 && !pr.fechando {
		pr.fechando = true
		if pr.cancelOcioso != nil {
			pr.cancelOcioso()
			pr.cancelOcioso = nil
		}
		go pr.encerrar()
	}
}

var errEncerrando = &ErroRunner{Mensagem: "O motor está encerrando."}

// obter reserva um processo para a automação (criando, despejando um ocioso ou esperando na fila).
func (p *Pool) obter(ctx context.Context, a Automacao) (*processo, error) {
	chave := chaveDe(a)
	for {
		p.mu.Lock()
		if p.encerrado {
			p.mu.Unlock()
			return nil, errEncerrando
		}
		if pr := p.procs[chave]; pr != nil && !pr.obsoleto {
			p.reservarSemTrava(pr)
			p.mu.Unlock()
			<-pr.iniciado
			if pr.erroInit != nil {
				p.liberar(pr)
				return nil, pr.erroInit
			}
			return pr, nil
		}
		// Processos da mesma automação com outro hash ficam obsoletos.
		for _, q := range p.procs {
			if q.aut.ID == a.ID && q.aut.Hash != a.Hash {
				p.retirarSemTrava(q)
			}
		}
		if len(p.procs) >= p.o.MaxProcessos {
			for _, q := range p.procs {
				if q.ocupado == 0 {
					p.retirarSemTrava(q)
					break
				}
			}
		}
		if len(p.procs) < p.o.MaxProcessos {
			pr := novoProcesso(p, a)
			p.procs[chave] = pr
			p.reservarSemTrava(pr)
			p.mu.Unlock()
			pr.erroInit = pr.iniciar()
			close(pr.iniciado)
			if pr.erroInit != nil {
				p.mu.Lock()
				if p.procs[chave] == pr {
					delete(p.procs, chave)
				}
				p.mu.Unlock()
				p.liberar(pr)
				return nil, pr.erroInit
			}
			return pr, nil
		}
		espera := make(chan struct{})
		p.fila = append(p.fila, espera)
		p.mu.Unlock()
		select {
		case <-espera:
		case <-ctx.Done():
			p.mu.Lock()
			for i, c := range p.fila {
				if c == espera {
					p.fila = append(p.fila[:i], p.fila[i+1:]...)
					break
				}
			}
			p.mu.Unlock()
			return nil, ctx.Err()
		}
	}
}

func (p *Pool) reservarSemTrava(pr *processo) {
	pr.ocupado++
	pr.geracaoOcioso++
	if pr.cancelOcioso != nil {
		pr.cancelOcioso()
		pr.cancelOcioso = nil
	}
}

// liberar devolve o processo; ocioso, ele é fechado após a ociosidade (relógio injetado).
func (p *Pool) liberar(pr *processo) {
	p.mu.Lock()
	defer p.mu.Unlock()
	pr.ocupado--
	if pr.ocupado > 0 {
		return
	}
	select {
	case <-pr.morto:
		p.acordarFilaSemTrava()
		return
	default:
	}
	if pr.obsoleto || p.procs[pr.chave] != pr || pr.erroInit != nil {
		if !pr.fechando {
			pr.fechando = true
			go pr.encerrar()
		}
		p.acordarFilaSemTrava()
		return
	}
	pr.geracaoOcioso++
	gen := pr.geracaoOcioso
	ctx, cancelar := context.WithCancel(context.Background())
	pr.cancelOcioso = cancelar
	limite := p.o.Relogio.Agora().Add(p.o.Ociosidade)
	go func() {
		if p.o.Relogio.Esperar(ctx, limite) != nil {
			return
		}
		p.mu.Lock()
		if pr.geracaoOcioso != gen || pr.ocupado > 0 || pr.fechando {
			p.mu.Unlock()
			return
		}
		p.retirarSemTrava(pr)
		p.mu.Unlock()
	}()
	p.acordarFilaSemTrava()
}

// Executar roda um handler no processo da automação e devolve o retorno JSON.
func (p *Pool) Executar(ctx context.Context, a Automacao, pd Pedido, ponte Ponte) (json.RawMessage, error) {
	if !p.Disponivel() {
		return nil, &ErroRunner{Mensagem: "O executor das automações de IA não está disponível nesta instalação."}
	}
	if pd.Prazo <= 0 {
		pd.Prazo = PadraoPrazo
	}
	pr, err := p.obter(ctx, a)
	if err != nil {
		return nil, err
	}
	defer p.liberar(pr)

	ctxExec, cancelar := context.WithCancel(ctx)
	defer cancelar()
	pr.mu.Lock()
	pr.execs[pd.ExecucaoID] = &execucao{id: pd.ExecucaoID, ponte: ponte, ctx: ctxExec}
	pr.mu.Unlock()
	p.mu.Lock()
	p.execProc[pd.ExecucaoID] = pr
	p.mu.Unlock()
	defer func() {
		pr.mu.Lock()
		delete(pr.execs, pd.ExecucaoID)
		pr.mu.Unlock()
		p.mu.Lock()
		delete(p.execProc, pd.ExecucaoID)
		p.mu.Unlock()
	}()

	params := map[string]any{"execucao_id": pd.ExecucaoID, "handler": pd.Handler, "simulacao": pd.Simulacao,
		"prazo_ms": pd.Prazo.Milliseconds(), "info": bruto(pd.Info), "conversa": bruto(pd.Conversa), "argumento": bruto(pd.Argumento)}
	id, ch, err := pr.requisitar("executar", params)
	if err != nil {
		select {
		case <-pr.morto:
			return nil, pr.erroMorte(pd.ExecucaoID, pd.Prazo)
		default:
		}
		return nil, &ErroRunner{Mensagem: "Falha ao falar com o processo da automação: " + err.Error()}
	}
	defer pr.esquecer(id)
	aviso := time.NewTimer(pd.Prazo)
	morte := time.NewTimer(pd.Prazo + FolgaMorte)
	defer aviso.Stop()
	defer morte.Stop()
	for {
		select {
		case m := <-ch:
			return interpretar(m)
		case <-aviso.C:
			pr.notificar("cancelar", map[string]any{"execucao_id": pd.ExecucaoID})
		case <-morte.C:
			pr.matarPorTempo(pd.ExecucaoID)
		case <-pr.morto:
			return nil, pr.erroMorte(pd.ExecucaoID, pd.Prazo)
		case <-ctx.Done():
			pr.notificar("cancelar", map[string]any{"execucao_id": pd.ExecucaoID})
			return nil, ctx.Err()
		}
	}
}

func bruto(r json.RawMessage) json.RawMessage {
	if len(r) == 0 {
		return json.RawMessage("null")
	}
	return r
}

func interpretar(m *mensagem) (json.RawMessage, error) {
	if m.Error == nil {
		var r struct {
			Retorno json.RawMessage `json:"retorno"`
		}
		json.Unmarshal(m.Result, &r)
		return bruto(r.Retorno), nil
	}
	switch m.Error.Codigo {
	case CodigoErroUsuario:
		var d struct {
			Nome     string `json:"nome"`
			Mensagem string `json:"mensagem"`
			Stack    string `json:"stack"`
		}
		b, _ := json.Marshal(m.Error.Dados)
		json.Unmarshal(b, &d)
		if d.Mensagem == "" {
			d.Mensagem = m.Error.Mensagem
		}
		return nil, &ErroUsuario{Nome: d.Nome, Mensagem: d.Mensagem, Stack: d.Stack}
	}
	return nil, &ErroRunner{Mensagem: m.Error.Mensagem}
}

// Cancelar avisa o runner (best effort) que a execução deve parar.
func (p *Pool) Cancelar(execucaoID string) {
	p.mu.Lock()
	pr := p.execProc[execucaoID]
	p.mu.Unlock()
	if pr != nil {
		pr.notificar("cancelar", map[string]any{"execucao_id": execucaoID})
	}
}

func (p *Pool) processosDe(filtro func(*processo) bool) []*processo {
	var l []*processo
	for pr := range p.todos {
		if filtro(pr) {
			l = append(l, pr)
		}
	}
	for _, pr := range p.procs {
		if _, ok := p.todos[pr]; !ok && filtro(pr) {
			l = append(l, pr)
		}
	}
	return l
}

// EncerrarAutomacao fecha os processos da automação (exclusão/desativação); espera saírem.
func (p *Pool) EncerrarAutomacao(id string) {
	p.mu.Lock()
	l := p.processosDe(func(pr *processo) bool { return pr.aut.ID == id })
	for _, pr := range l {
		if p.procs[pr.chave] == pr {
			delete(p.procs, pr.chave)
		}
		pr.obsoleto, pr.fechando = true, true
		if pr.cancelOcioso != nil {
			pr.cancelOcioso()
			pr.cancelOcioso = nil
		}
	}
	p.mu.Unlock()
	esperarTodos(l)
}

// SegredosAlterados fecha (já, ou quando ficarem ociosos) os processos das automações que
// declaram algum dos segredos alterados; reiniciam na próxima execução.
func (p *Pool) SegredosAlterados(nomes []string) {
	alterado := map[string]bool{}
	for _, n := range nomes {
		alterado[n] = true
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, pr := range p.processosDe(func(pr *processo) bool {
		for _, n := range pr.aut.NomesSegredos {
			if alterado[n] {
				return true
			}
		}
		return false
	}) {
		p.retirarSemTrava(pr)
	}
}

// EncerrarTodos fecha todos os processos ("encerrar" + SIGKILL após 2 s) e só volta quando
// nenhum processo runner estiver vivo (ou o contexto acabar).
func (p *Pool) EncerrarTodos(ctx context.Context) {
	p.mu.Lock()
	p.encerrado = true
	l := p.processosDe(func(*processo) bool { return true })
	for _, pr := range l {
		pr.fechando = true
		if pr.cancelOcioso != nil {
			pr.cancelOcioso()
			pr.cancelOcioso = nil
		}
	}
	p.procs = map[string]*processo{}
	for _, c := range p.fila {
		close(c)
	}
	p.fila = nil
	p.mu.Unlock()
	feito := make(chan struct{})
	go func() { esperarTodos(l); close(feito) }()
	select {
	case <-feito:
	case <-ctx.Done():
		for _, pr := range l {
			pr.matar()
		}
	}
}

func esperarTodos(l []*processo) {
	var wg sync.WaitGroup
	for _, pr := range l {
		wg.Add(1)
		go func(pr *processo) { defer wg.Done(); pr.encerrar() }(pr)
	}
	wg.Wait()
}
