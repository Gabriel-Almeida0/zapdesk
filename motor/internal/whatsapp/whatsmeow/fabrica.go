// Pacote whatsmeow é o adaptador real de whatsapp.Cliente — o ÚNICO pacote do motor que importa
// go.mau.fi/whatsmeow (Constituição IV). Cada conta tem a sua sessão em
// <pasta-dados>/sessoes/<conta_id>.db (sqlstore com modernc.org/sqlite, research.md §1).
package whatsmeow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/whatsapp"
)

// Fabrica abre um cliente whatsmeow por conta.
type Fabrica struct {
	pasta string
	log   zerolog.Logger
}

var _ whatsapp.Fabrica = (*Fabrica)(nil)

// NovaFabrica cria a fábrica real.
func NovaFabrica(pastaDados string, log zerolog.Logger) (*Fabrica, error) {
	if err := os.MkdirAll(filepath.Join(pastaDados, "sessoes"), 0o700); err != nil {
		return nil, err
	}
	return &Fabrica{pasta: pastaDados, log: log.With().Str("componente", "whatsmeow").Logger()}, nil
}

func (f *Fabrica) caminho(contaID string) string {
	return filepath.Join(f.pasta, "sessoes", filepath.Base(contaID)+".db")
}

// abrirContainer abre o sqlstore da conta. O DSN do modernc PRECISA de _pragma=foreign_keys(1)
// (sem ele o sqlstore.New falha com "foreign keys are not enabled").
func (f *Fabrica) abrirContainer(ctx context.Context, contaID string, log waLog.Logger) (*sqlstore.Container, error) {
	return sqlstore.New(ctx, "sqlite", armazenamento.DSN(f.caminho(contaID), false), log)
}

// Abrir abre (ou cria) a sessão da conta e o cliente.
func (f *Fabrica) Abrir(ctx context.Context, contaID string) (whatsapp.Cliente, error) {
	log := f.log.With().Str("conta", contaID).Logger()
	wlog := waLog.Zerolog(log)
	container, err := f.abrirContainer(ctx, contaID, wlog.Sub("banco"))
	if err != nil {
		return nil, fmt.Errorf("abrir sessão: %w", err)
	}
	dispositivo, err := container.GetFirstDevice(ctx)
	if err != nil {
		container.Close()
		return nil, fmt.Errorf("ler dispositivo: %w", err)
	}
	cli := whatsmeow.NewClient(dispositivo, wlog.Sub("cliente"))
	cli.EnableAutoReconnect = true
	c := &Cliente{
		contaID:   contaID,
		container: container,
		cli:       cli,
		log:       log,
		eventos:   make(chan whatsapp.Evento, 1024),
		grupos:    map[string]string{},
	}
	cli.AddEventHandler(c.tratar)
	return c, nil
}

// Remover faz logout (se possível) e apaga a sessão da conta.
func (f *Fabrica) Remover(ctx context.Context, contaID string) error {
	caminho := f.caminho(contaID)
	if _, err := os.Stat(caminho); err == nil {
		wlog := waLog.Zerolog(f.log.With().Str("conta", contaID).Logger())
		if container, err := f.abrirContainer(ctx, contaID, wlog.Sub("banco")); err == nil {
			if dev, err := container.GetFirstDevice(ctx); err == nil && dev.ID != nil {
				cli := whatsmeow.NewClient(dev, wlog.Sub("cliente"))
				ctxLogout, cancelar := context.WithTimeout(ctx, 15*time.Second)
				if cli.ConnectContext(ctxLogout) == nil {
					if err := cli.Logout(ctxLogout); err != nil {
						f.log.Warn().Err(err).Str("conta", contaID).Msg("logout ao remover conta")
					}
				}
				cancelar()
				cli.Disconnect()
				container.DeleteDevice(ctx, dev)
			}
			container.Close()
		}
	}
	for _, sufixo := range []string{"", "-wal", "-shm"} {
		os.Remove(caminho + sufixo)
	}
	return nil
}
