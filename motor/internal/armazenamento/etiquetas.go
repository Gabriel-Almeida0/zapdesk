package armazenamento

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
)

const selectEtiqueta = `SELECT e.id, e.nome, e.cor, (SELECT count(*) FROM contato_etiquetas ce WHERE ce.etiqueta_id = e.id) FROM etiquetas e`

// ListarEtiquetas por nome.
func ListarEtiquetas(ctx context.Context, ex Executor) ([]dominio.Etiqueta, error) {
	linhas, err := ex.QueryContext(ctx, selectEtiqueta+` ORDER BY e.nome COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	lista := []dominio.Etiqueta{}
	for linhas.Next() {
		var e dominio.Etiqueta
		if err := linhas.Scan(&e.ID, &e.Nome, &e.Cor, &e.TotalContatos); err != nil {
			return nil, err
		}
		lista = append(lista, e)
	}
	return lista, linhas.Err()
}

// ObterEtiqueta por id.
func ObterEtiqueta(ctx context.Context, ex Executor, id string) (dominio.Etiqueta, error) {
	var e dominio.Etiqueta
	err := ex.QueryRowContext(ctx, selectEtiqueta+` WHERE e.id = ?`, id).Scan(&e.ID, &e.Nome, &e.Cor, &e.TotalContatos)
	if errors.Is(err, sql.ErrNoRows) {
		return e, erros.NaoAchada("Etiqueta")
	}
	return e, err
}

// InserirEtiqueta cria (conflito se o nome existe, sem diferenciar maiúsculas).
func InserirEtiqueta(ctx context.Context, ex Executor, e dominio.Etiqueta, em time.Time) error {
	_, err := ex.ExecContext(ctx, `INSERT INTO etiquetas (id, nome, cor, criada_em) VALUES (?, ?, ?, ?)`, e.ID, e.Nome, e.Cor, Ms(em))
	return traduzirUnico(err, "Já existe uma etiqueta com esse nome.")
}

// AtualizarEtiqueta grava nome e cor.
func AtualizarEtiqueta(ctx context.Context, ex Executor, e dominio.Etiqueta) error {
	_, err := ex.ExecContext(ctx, `UPDATE etiquetas SET nome = ?, cor = ? WHERE id = ?`, e.Nome, e.Cor, e.ID)
	return traduzirUnico(err, "Já existe uma etiqueta com esse nome.")
}

// ExcluirEtiqueta apaga (e, em cascata, dos contatos).
func ExcluirEtiqueta(ctx context.Context, ex Executor, id string) (bool, error) {
	r, err := ex.ExecContext(ctx, `DELETE FROM etiquetas WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}

// ContatosDaEtiqueta devolve os ids de contatos com a etiqueta.
func ContatosDaEtiqueta(ctx context.Context, ex Executor, id string) ([]string, error) {
	return idsAfetados(ctx, ex, `SELECT contato_id FROM contato_etiquetas WHERE etiqueta_id = ?`, id)
}

// DefinirEtiquetasContato substitui o conjunto de etiquetas do contato.
func DefinirEtiquetasContato(ctx context.Context, tx *sql.Tx, contatoID string, etiquetaIDs []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM contato_etiquetas WHERE contato_id = ?`, contatoID); err != nil {
		return err
	}
	for _, id := range etiquetaIDs {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO contato_etiquetas (contato_id, etiqueta_id) VALUES (?, ?)`, contatoID, id); err != nil {
			return err
		}
	}
	return nil
}

// DefinirNotas grava as notas do contato.
func DefinirNotas(ctx context.Context, ex Executor, contatoID string, notas *string, agora time.Time) error {
	_, err := ex.ExecContext(ctx, `UPDATE contatos SET notas = ?, atualizado_em = ? WHERE id = ?`, strOuNil(notas), Ms(agora), contatoID)
	return err
}
