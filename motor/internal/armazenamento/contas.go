package armazenamento

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
)

const colunasConta = `id, nome, telefone, jid, estado, sincronizando, criada_em`

func scanConta(sc interface{ Scan(...any) error }) (dominio.Conta, error) {
	var c dominio.Conta
	var tel, jid sql.NullString
	var sinc int
	var criada int64
	if err := sc.Scan(&c.ID, &c.Nome, &tel, &jid, &c.Estado, &sinc, &criada); err != nil {
		return c, err
	}
	c.Telefone, c.JID, c.Sincronizando, c.CriadaEm = deNullStr(tel), deNullStr(jid), sinc == 1, DeMs(criada)
	return c, nil
}

// CriarConta insere uma conta.
func CriarConta(ctx context.Context, ex Executor, c dominio.Conta) error {
	_, err := ex.ExecContext(ctx, `INSERT INTO contas (id, nome, estado, criada_em, atualizada_em) VALUES (?, ?, ?, ?, ?)`,
		c.ID, c.Nome, c.Estado, Ms(c.CriadaEm), Ms(c.CriadaEm))
	return err
}

// ObterConta busca por id (nao_encontrado se não existe).
func ObterConta(ctx context.Context, ex Executor, id string) (dominio.Conta, error) {
	c, err := scanConta(ex.QueryRowContext(ctx, `SELECT `+colunasConta+` FROM contas WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return c, erros.NaoAchada("Conta")
	}
	return c, err
}

// ListarContas devolve todas as contas por ordem de criação.
func ListarContas(ctx context.Context, ex Executor) ([]dominio.Conta, error) {
	linhas, err := ex.QueryContext(ctx, `SELECT `+colunasConta+` FROM contas ORDER BY criada_em, id`)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	lista := []dominio.Conta{}
	for linhas.Next() {
		c, err := scanConta(linhas)
		if err != nil {
			return nil, err
		}
		lista = append(lista, c)
	}
	return lista, linhas.Err()
}

// AtualizarEstadoConta muda o estado.
func AtualizarEstadoConta(ctx context.Context, ex Executor, id, estado string, agora time.Time) error {
	_, err := ex.ExecContext(ctx, `UPDATE contas SET estado = ?, atualizada_em = ? WHERE id = ?`, estado, Ms(agora), id)
	return err
}

// AtualizarIdentidadeConta grava jid/telefone (após conectar).
func AtualizarIdentidadeConta(ctx context.Context, ex Executor, id, jid, telefone string, agora time.Time) error {
	_, err := ex.ExecContext(ctx, `UPDATE contas SET jid = ?, telefone = ?, atualizada_em = ? WHERE id = ?`,
		vazioNil(jid), vazioNil(telefone), Ms(agora), id)
	return err
}

// RenomearConta muda o nome.
func RenomearConta(ctx context.Context, ex Executor, id, nome string, agora time.Time) error {
	_, err := ex.ExecContext(ctx, `UPDATE contas SET nome = ?, atualizada_em = ? WHERE id = ?`, nome, Ms(agora), id)
	return err
}

// DefinirSincronizando liga/desliga o indicador de history sync.
func DefinirSincronizando(ctx context.Context, ex Executor, id string, v bool) error {
	_, err := ex.ExecContext(ctx, `UPDATE contas SET sincronizando = ? WHERE id = ?`, b2i(v), id)
	return err
}

// RemoverConta apaga a conta e (em cascata) conversas, mensagens, contatos, status e disparos.
func RemoverConta(ctx context.Context, ex Executor, id string) error {
	_, err := ex.ExecContext(ctx, `DELETE FROM contas WHERE id = ?`, id)
	return err
}
