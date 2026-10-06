package armazenamento

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed migracoes/*.sql
var migracoesEmbutidas embed.FS

// MigracoesPadrao é o FS com as migrações do motor.
func MigracoesPadrao() fs.FS {
	sub, _ := fs.Sub(migracoesEmbutidas, "migracoes")
	return sub
}

// Migracao é um arquivo NNNN_nome.sql.
type Migracao struct {
	Versao int
	Nome   string
	SQL    string
}

// ListarMigracoes lê e ordena as migrações do FS.
func ListarMigracoes(fsys fs.FS) ([]Migracao, error) {
	entradas, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	var lista []Migracao
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".sql")
		num, nome, ok := strings.Cut(base, "_")
		if !ok {
			return nil, fmt.Errorf("migração com nome inválido: %s", e.Name())
		}
		v, err := strconv.Atoi(num)
		if err != nil {
			return nil, fmt.Errorf("migração com número inválido: %s", e.Name())
		}
		conteudo, err := fs.ReadFile(fsys, path.Clean(e.Name()))
		if err != nil {
			return nil, err
		}
		lista = append(lista, Migracao{Versao: v, Nome: nome, SQL: string(conteudo)})
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].Versao < lista[j].Versao })
	for i := 1; i < len(lista); i++ {
		if lista[i].Versao == lista[i-1].Versao {
			return nil, fmt.Errorf("migração duplicada: %d", lista[i].Versao)
		}
	}
	return lista, nil
}

// VersaoAtual devolve a maior versão aplicada (0 se nenhuma).
func (b *Banco) VersaoAtual(ctx context.Context) (int, error) {
	if _, err := b.escrita.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migracoes (
		versao INTEGER PRIMARY KEY, nome TEXT NOT NULL, aplicada_em INTEGER NOT NULL)`); err != nil {
		return 0, err
	}
	var v int
	err := b.escrita.QueryRowContext(ctx, `SELECT COALESCE(MAX(versao), 0) FROM schema_migracoes`).Scan(&v)
	return v, err
}

// Migrar aplica as migrações pendentes do FS. Antes de aplicar qualquer uma num banco que já tem
// esquema, faz backup em <banco>.bak-<versao atual> (VACUUM INTO, seguro com WAL). Devolve as
// versões aplicadas.
func (b *Banco) Migrar(ctx context.Context, fsys fs.FS) ([]int, error) {
	migracoes, err := ListarMigracoes(fsys)
	if err != nil {
		return nil, err
	}
	atual, err := b.VersaoAtual(ctx)
	if err != nil {
		return nil, err
	}
	var pendentes []Migracao
	for _, m := range migracoes {
		if m.Versao > atual {
			pendentes = append(pendentes, m)
		}
	}
	if len(pendentes) == 0 {
		return nil, nil
	}
	if atual > 0 {
		destino := fmt.Sprintf("%s.bak-%d", b.Caminho, atual)
		os.Remove(destino)
		if _, err := b.escrita.ExecContext(ctx, `VACUUM INTO ?`, destino); err != nil {
			return nil, fmt.Errorf("backup antes da migração: %w", err)
		}
	}
	var aplicadas []int
	for _, m := range pendentes {
		tx, err := b.escrita.BeginTx(ctx, nil)
		if err != nil {
			return aplicadas, err
		}
		if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
			tx.Rollback()
			return aplicadas, fmt.Errorf("migração %04d_%s: %w", m.Versao, m.Nome, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migracoes (versao, nome, aplicada_em) VALUES (?, ?, ?)`,
			m.Versao, m.Nome, time.Now().UnixMilli()); err != nil {
			tx.Rollback()
			return aplicadas, err
		}
		if err := tx.Commit(); err != nil {
			return aplicadas, err
		}
		aplicadas = append(aplicadas, m.Versao)
	}
	return aplicadas, nil
}
