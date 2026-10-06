package armazenamento

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
)

func scanTemplate(sc interface{ Scan(...any) error }) (dominio.Template, error) {
	var t dominio.Template
	var arq sql.NullString
	var criado, atualizado int64
	if err := sc.Scan(&t.ID, &t.Nome, &t.Texto, &arq, &criado, &atualizado); err != nil {
		return t, err
	}
	t.ArquivoID, t.CriadoEm, t.AtualizadoEm = deNullStr(arq), DeMs(criado), DeMs(atualizado)
	return t, nil
}

// ObterTemplate busca por id (sem o arquivo resolvido).
func ObterTemplate(ctx context.Context, ex Executor, id string) (dominio.Template, error) {
	t, err := scanTemplate(ex.QueryRowContext(ctx, `SELECT id, nome, texto, arquivo_id, criado_em, atualizado_em FROM templates WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return t, erros.NaoAchado("Template")
	}
	return t, err
}

// ListarTemplates lista por nome; `busca` filtra por trecho do nome.
func ListarTemplates(ctx context.Context, ex Executor, busca string) ([]dominio.Template, error) {
	q := `SELECT id, nome, texto, arquivo_id, criado_em, atualizado_em FROM templates`
	var args []any
	if strings.TrimSpace(busca) != "" {
		q += ` WHERE nome LIKE ? ESCAPE '\'`
		args = append(args, escaparLike(strings.TrimSpace(busca)))
	}
	q += ` ORDER BY nome COLLATE NOCASE`
	linhas, err := ex.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	lista := []dominio.Template{}
	for linhas.Next() {
		t, err := scanTemplate(linhas)
		if err != nil {
			return nil, err
		}
		lista = append(lista, t)
	}
	return lista, linhas.Err()
}

// InserirTemplate cria (conflito se o nome já existe).
func InserirTemplate(ctx context.Context, ex Executor, t dominio.Template) error {
	_, err := ex.ExecContext(ctx, `INSERT INTO templates (id, nome, texto, arquivo_id, criado_em, atualizado_em) VALUES (?, ?, ?, ?, ?, ?)`,
		t.ID, t.Nome, t.Texto, strOuNil(t.ArquivoID), Ms(t.CriadoEm), Ms(t.AtualizadoEm))
	return traduzirUnico(err, "Já existe um template com esse nome.")
}

// AtualizarTemplate grava nome, texto e arquivo.
func AtualizarTemplate(ctx context.Context, ex Executor, t dominio.Template, agora time.Time) error {
	_, err := ex.ExecContext(ctx, `UPDATE templates SET nome = ?, texto = ?, arquivo_id = ?, atualizado_em = ? WHERE id = ?`,
		t.Nome, t.Texto, strOuNil(t.ArquivoID), Ms(agora), t.ID)
	return traduzirUnico(err, "Já existe um template com esse nome.")
}

// ExcluirTemplate apaga.
func ExcluirTemplate(ctx context.Context, ex Executor, id string) (bool, error) {
	r, err := ex.ExecContext(ctx, `DELETE FROM templates WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}

// traduzirUnico converte violação de UNIQUE em erro de conflito.
func traduzirUnico(err error, mensagem string) error {
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return erros.Novo(erros.Conflito, mensagem)
	}
	return err
}
