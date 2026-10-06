// Comando zapdesk-motor: processo local que é a fonte única da verdade do ZapDesk
// (contracts/runtime.md). Montagem: config → logs → banco/migração → barramento → fábrica
// (real | falso) → API → ciclo.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"zapdesk/motor/internal/api"
	"zapdesk/motor/internal/aplicacao"
	"zapdesk/motor/internal/ciclo"
	"zapdesk/motor/internal/config"
	"zapdesk/motor/internal/logs"
	"zapdesk/motor/internal/whatsapp"
	"zapdesk/motor/internal/whatsapp/whatsmeow"
)

// versao do motor; pode ser sobrescrita no build com -ldflags "-X main.versao=...".
var versao = "0.1.0"

// TempoEncerramento é o prazo do encerramento gracioso.
const TempoEncerramento = 5 * time.Second

func main() {
	os.Exit(executar(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func executar(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	saida := ciclo.NovaSaida(stdout)

	cfg, err := config.Carregar(args, os.Getenv, stderr)
	if errors.Is(err, config.ErrAjuda) {
		return ciclo.SaidaNormal
	}
	if err != nil {
		var ec *config.ErroConfig
		codigo := "config_invalida"
		if errors.As(err, &ec) {
			codigo = ec.Codigo
		}
		if codigo == "token_ausente" {
			saida.ErroFatal(ciclo.FatalTokenAusente, err.Error())
		} else {
			fmt.Fprintln(stderr, "zapdesk-motor:", err)
		}
		return ciclo.SaidaConfigInvalida
	}
	if cfg.MostrarVersao {
		fmt.Fprintln(stdout, versao)
		return ciclo.SaidaNormal
	}

	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(64 << 20)
	}

	if err := os.MkdirAll(cfg.PastaDados, 0o700); err != nil {
		saida.ErroFatal(ciclo.FatalPastaInacessivel, err.Error())
		return ciclo.SaidaErroInesperado
	}
	trava, err := ciclo.Travar(cfg.PastaDados)
	if errors.Is(err, ciclo.ErrInstanciaDuplicada) {
		saida.ErroFatal(ciclo.FatalInstanciaDuplicada, err.Error())
		return ciclo.SaidaInstanciaDuplicada
	}
	if err != nil {
		saida.ErroFatal(ciclo.FatalPastaInacessivel, err.Error())
		return ciclo.SaidaErroInesperado
	}
	defer trava.Liberar()

	log, err := logs.Novo(logs.Opcoes{PastaDados: cfg.PastaDados, Nivel: cfg.NivelLog, LogConteudo: cfg.LogConteudo, Stderr: stderr})
	if err != nil {
		saida.ErroFatal(ciclo.FatalPastaInacessivel, err.Error())
		return ciclo.SaidaErroInesperado
	}
	defer log.Fechar()

	pedido := make(chan struct{})
	var pedirUma sync.Once
	pedirEncerrar := func() { pedirUma.Do(func() { close(pedido) }) }

	ctx := context.Background()
	app, err := aplicacao.Montar(ctx, aplicacao.Opcoes{
		Versao:       versao,
		PastaDados:   cfg.PastaDados,
		CaminhoLogs:  log.Caminho,
		ModoWhatsApp: cfg.WhatsApp,
		Token:        cfg.Token,
		Religado:     cfg.Religado,
		Log:          log.Logger,
		FabricaReal: func(pasta string, l zerolog.Logger) (whatsapp.Fabrica, error) {
			return whatsmeow.NovaFabrica(pasta, l)
		},
		Encerrar:         pedirEncerrar,
		RunnerExec:       cfg.RunnerExec,
		RunnerScript:     cfg.RunnerScript,
		IA:               cfg.IA,
		AguardarSegredos: cfg.AguardarSegredos,
	})
	switch {
	case errors.Is(err, aplicacao.ErrMigracao):
		saida.ErroFatal(ciclo.FatalMigracaoFalhou, err.Error())
		return ciclo.SaidaErroInesperado
	case errors.Is(err, aplicacao.ErrPasta):
		saida.ErroFatal(ciclo.FatalPastaInacessivel, err.Error())
		return ciclo.SaidaErroInesperado
	case err != nil:
		saida.ErroFatal(ciclo.FatalPastaInacessivel, err.Error())
		return ciclo.SaidaErroInesperado
	}

	if err := app.Servidor.Escutar(cfg.Porta); err != nil {
		codigo := ciclo.FatalPortaOcupada
		if !errors.Is(err, api.ErrPortaOcupada) {
			codigo = ciclo.FatalPortaOcupada
		}
		saida.ErroFatal(codigo, err.Error())
		app.Encerrar(ctx)
		return ciclo.SaidaErroInesperado
	}
	erroServir := make(chan error, 1)
	go func() { erroServir <- app.Servidor.Servir() }()

	saida.Pronto(app.Servidor.Porta(), versao, os.Getpid(), cfg.WhatsApp)
	log.Info().Int("porta", app.Servidor.Porta()).Str("whatsapp", cfg.WhatsApp).Msg("motor pronto")
	app.Iniciar()

	var stdinFim <-chan struct{}
	if !cfg.SemStdin {
		// Linhas de controle do app (contracts/runtime.md › Canal de controle): só "segredos".
		// Nunca logar o conteúdo.
		stdinFim = ciclo.VigiarStdinControle(stdin, func(c ciclo.ComandoControle) {
			if c.Comando == ciclo.ComandoSegredos {
				app.Cofre().Substituir(c.Valores)
			}
		}, func() { log.Warn().Msg("linha de controle inválida no stdin (ignorada)") })
	}
	fimServir := make(chan struct{})
	go func() {
		if err := <-erroServir; err != nil {
			log.Error().Err(err).Msg("servidor HTTP parou")
		}
		close(fimServir)
	}()
	motivo := ciclo.AguardarFim(ctx, stdinFim, mesclar(pedido, fimServir))

	saida.Encerrando(motivo)
	log.Info().Str("motivo", motivo).Msg("encerrando")
	ctxFim, cancelar := context.WithTimeout(ctx, TempoEncerramento)
	defer cancelar()
	feito := make(chan struct{})
	go func() {
		app.Encerrar(ctxFim)
		close(feito)
	}()
	select {
	case <-feito:
	case <-ctxFim.Done():
		log.Warn().Msg("encerramento passou de 5 s; saindo mesmo assim")
	}
	return ciclo.SaidaNormal
}

func mesclar(a, b <-chan struct{}) <-chan struct{} {
	c := make(chan struct{})
	go func() {
		select {
		case <-a:
		case <-b:
		}
		close(c)
	}()
	return c
}
