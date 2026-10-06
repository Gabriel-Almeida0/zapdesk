package armazenamento

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
)

// CriarArquivo insere o registro do anexo.
func CriarArquivo(ctx context.Context, ex Executor, a dominio.Arquivo, em time.Time) error {
	_, err := ex.ExecContext(ctx, `INSERT INTO arquivos (id, nome, mimetype, tamanho, tipo_midia, caminho, criado_em) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Nome, a.Mimetype, a.Tamanho, a.TipoMidia, a.Caminho, Ms(em))
	return err
}

// ObterArquivo busca por id.
func ObterArquivo(ctx context.Context, ex Executor, id string) (dominio.Arquivo, error) {
	var a dominio.Arquivo
	err := ex.QueryRowContext(ctx, `SELECT id, nome, mimetype, tamanho, tipo_midia, caminho FROM arquivos WHERE id = ?`, id).
		Scan(&a.ID, &a.Nome, &a.Mimetype, &a.Tamanho, &a.TipoMidia, &a.Caminho)
	if errors.Is(err, sql.ErrNoRows) {
		return a, erros.NaoAchado("Arquivo")
	}
	a.URL = "/v1/arquivos/" + a.ID + "/conteudo"
	return a, err
}

// ArquivoOpcional busca quando id != nil.
func ArquivoOpcional(ctx context.Context, ex Executor, id *string) (*dominio.Arquivo, error) {
	if id == nil || *id == "" {
		return nil, nil
	}
	a, err := ObterArquivo(ctx, ex, *id)
	if err != nil {
		if erros.Eh(err, erros.NaoEncontrado) {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}
