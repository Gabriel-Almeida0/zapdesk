// Pacote aplicacao monta o motor inteiro (banco, barramento, WhatsApp, serviços e API) a partir
// da configuração. É usado pelo main e pelo harness de integração (motor/testes/integracao).
package aplicacao

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"zapdesk/motor/internal/api"
	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/relogio"
	"zapdesk/motor/internal/whatsapp"
	"zapdesk/motor/internal/whatsapp/falso"
)

// ErrMigracao envolve falhas de migração (erro_fatal migracao_falhou).
var ErrMigracao = errors.New("migração falhou")

// ErrPasta envolve falhas de acesso à pasta de dados.
var ErrPasta = errors.New("pasta de dados inacessível")

// Opcoes da montagem.
type Opcoes struct {
	Versao       string
	PastaDados   string
	CaminhoLogs  string
	ModoWhatsApp string // real | falso
	Token        string
	Religado     bool
	Log          zerolog.Logger

	// Relogio do domínio. No modo falso, se nil, usa um Controlavel corrente.
	Relogio relogio.Relogio
	// FabricaReal cria a fábrica do whatsmeow (modo real). Injetada pelo main para que só o
	// binário importe o adaptador.
	FabricaReal func(pastaDados string, log zerolog.Logger) (whatsapp.Fabrica, error)
	// Migracoes (testes); nil = embutidas.
	Migracoes fs.FS
	// Encerrar é chamado quando a API pede encerramento.
	Encerrar func()

	// Feature 002 (contracts/runtime.md › Novas flags).
	RunnerExec, RunnerScript string
	IA                       string // real | falsa
	AguardarSegredos         bool
	// URLClaude troca a URL da Claude API (só testes: servidor falso que conta conexões).
	URLClaude string
}

// App é o motor montado.
type App struct {
	o          Opcoes
	Banco      *armazenamento.Banco
	Barramento *eventos.Barramento
	Relogio    relogio.Relogio
	Fabrica    whatsapp.Fabrica
	Falso      *falso.Controle
	Servidor   *api.Servidor

	fecharUma sync.Once
	cancelar  context.CancelFunc
	ctxApp    context.Context
	sv        servicos
	aut       pecasAutomacoes

	apiServicos api.Servicos
}

// Montar abre o banco, migra e monta os serviços. Não escuta rede (use Escutar ou Handler).
func Montar(ctx context.Context, o Opcoes) (*App, error) {
	if err := os.MkdirAll(o.PastaDados, 0o700); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPasta, err)
	}
	a := &App{o: o, Relogio: o.Relogio}
	var controlavel *relogio.Controlavel
	if o.ModoWhatsApp == "falso" {
		if a.Relogio == nil {
			a.Relogio = relogio.NovoControlavel(time.Now())
		}
		controlavel, _ = a.Relogio.(*relogio.Controlavel)
	} else if a.Relogio == nil {
		a.Relogio = relogio.Real{}
	}

	banco, err := armazenamento.Abrir(ctx, o.PastaDados)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPasta, err)
	}
	a.Banco = banco
	migs := o.Migracoes
	if migs == nil {
		migs = armazenamento.MigracoesPadrao()
	}
	if _, err := banco.Migrar(ctx, migs); err != nil {
		banco.Fechar()
		return nil, fmt.Errorf("%w: %v", ErrMigracao, err)
	}

	a.Barramento = eventos.Novo(a.Relogio.Agora)

	if o.ModoWhatsApp == "falso" {
		ctrl, err := falso.Novo(o.PastaDados, a.Relogio)
		if err != nil {
			banco.Fechar()
			return nil, err
		}
		a.Falso = ctrl
		a.Fabrica = ctrl
	} else {
		if o.FabricaReal == nil {
			banco.Fechar()
			return nil, errors.New("fábrica do WhatsApp real não configurada")
		}
		f, err := o.FabricaReal(o.PastaDados, o.Log)
		if err != nil {
			banco.Fechar()
			return nil, err
		}
		a.Fabrica = f
	}

	ctxApp, cancelar := context.WithCancel(context.Background())
	a.cancelar, a.ctxApp = cancelar, ctxApp
	servicos, err := a.montarServicos(ctxApp)
	if err != nil {
		cancelar()
		banco.Fechar()
		return nil, err
	}

	a.Servidor = api.Novo(api.Opcoes{
		Versao:             o.Versao,
		PastaDados:         o.PastaDados,
		CaminhoLogs:        o.CaminhoLogs,
		ModoWhatsApp:       o.ModoWhatsApp,
		Token:              o.Token,
		Log:                o.Log,
		Barramento:         a.Barramento,
		Relogio:            a.Relogio,
		Falso:              a.Falso,
		RelogioControlavel: controlavel,
		Encerrar:           o.Encerrar,
		Energia:            a.energia,
		Servicos:           servicos,
	})
	return a, nil
}

// Handler devolve o handler HTTP completo (para httptest).
func (a *App) Handler() http.Handler { return a.Servidor.Handler() }

// Iniciar põe o motor para trabalhar em segundo plano (contas, agendadores). Chamado depois que a
// API está escutando; não bloqueia o "pronto".
func (a *App) Iniciar() { a.iniciarServicos() }

// Encerrar para agendadores, desconecta clientes e fecha o banco (idempotente).
func (a *App) Encerrar(ctx context.Context) {
	a.fecharUma.Do(func() {
		if a.Servidor != nil {
			a.Servidor.Encerrar(ctx)
		}
		a.encerrarServicos(ctx)
		a.cancelar()
		a.Barramento.FecharTodas()
		a.Banco.Fechar()
	})
}
