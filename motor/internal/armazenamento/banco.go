// Pacote armazenamento abre o SQLite do app (modernc.org/sqlite, sem CGO), aplica as migrações
// e implementa os repositórios de cada entidade de data-model.md.
package armazenamento

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // driver "sqlite"
)

// NomeBanco é o arquivo principal na pasta de dados.
const NomeBanco = "zapdesk.db"

// DSN monta o endereço do modernc com os pragmas exigidos (research.md §1–§2): a sintaxe
// _pragma=foreign_keys(1) é a do modernc (a _foreign_keys=on é do mattn e não funciona aqui).
func DSN(caminho string, somenteLeitura bool) string {
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "synchronous(NORMAL)")
	if somenteLeitura {
		q.Add("_pragma", "query_only(1)")
	}
	q.Set("_txlock", "immediate")
	return "file:" + caminho + "?" + q.Encode()
}

// Banco agrupa uma conexão única de escrita (evita SQLITE_BUSY) e um pool de leitura.
type Banco struct {
	Caminho string
	escrita *sql.DB
	leitura *sql.DB
}

// Abrir abre (criando se preciso) o banco em <pasta>/zapdesk.db.
func Abrir(ctx context.Context, pastaDados string) (*Banco, error) {
	if err := os.MkdirAll(pastaDados, 0o700); err != nil {
		return nil, err
	}
	caminho := filepath.Join(pastaDados, NomeBanco)
	return AbrirArquivo(ctx, caminho)
}

// AbrirArquivo abre um arquivo SQLite específico.
func AbrirArquivo(ctx context.Context, caminho string) (*Banco, error) {
	esc, err := sql.Open("sqlite", DSN(caminho, false))
	if err != nil {
		return nil, err
	}
	esc.SetMaxOpenConns(1)
	esc.SetMaxIdleConns(1)
	esc.SetConnMaxIdleTime(0)
	if err := esc.PingContext(ctx); err != nil {
		esc.Close()
		return nil, fmt.Errorf("abrir %s: %w", caminho, err)
	}
	lei, err := sql.Open("sqlite", DSN(caminho, true))
	if err != nil {
		esc.Close()
		return nil, err
	}
	lei.SetMaxOpenConns(4)
	lei.SetMaxIdleConns(2)
	lei.SetConnMaxIdleTime(time.Minute)
	return &Banco{Caminho: caminho, escrita: esc, leitura: lei}, nil
}

// E devolve o pool de escrita (1 conexão). Nunca faça uma escrita em E dentro de outra
// transação em E (deadlock): passe o *sql.Tx adiante.
func (b *Banco) E() *sql.DB { return b.escrita }

// L devolve o pool de leitura.
func (b *Banco) L() *sql.DB { return b.leitura }

// Fechar fecha as conexões (faz checkpoint do WAL).
func (b *Banco) Fechar() error {
	errL := b.leitura.Close()
	errE := b.escrita.Close()
	if errE != nil {
		return errE
	}
	return errL
}

// Transacao executa fn numa transação de escrita, com commit se fn devolver nil.
func (b *Banco) Transacao(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := b.escrita.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Executor é o subconjunto comum de *sql.DB e *sql.Tx.
type Executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
