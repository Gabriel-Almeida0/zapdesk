package armazenamento

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"zapdesk/motor/internal/dominio"
)

// Repositório de esperas (data-model.md › Espera agendada).

const colunasEspera = `id, tipo, automacao_id, execucao_id, sessao_id, conversa_id, referencia, chave, retomar_em, dados, criada_em`

func scanEspera(sc interface{ Scan(...any) error }) (dominio.Espera, error) {
	var e dominio.Espera
	var exec, sessao, conversa, ref, chave sql.NullString
	var dados string
	var ret, cri int64
	if err := sc.Scan(&e.ID, &e.Tipo, &e.AutomacaoID, &exec, &sessao, &conversa, &ref, &chave, &ret, &dados, &cri); err != nil {
		return e, err
	}
	e.ExecucaoID, e.SessaoID, e.ConversaID, e.Referencia, e.Chave = deNullStr(exec), deNullStr(sessao), deNullStr(conversa), deNullStr(ref), deNullStr(chave)
	e.RetomarEm, e.CriadaEm, e.Dados = DeMs(ret), DeMs(cri), json.RawMessage(dados)
	return e, nil
}

func listarEsperas(ctx context.Context, ex Executor, q string, args ...any) ([]dominio.Espera, error) {
	linhas, err := ex.QueryContext(ctx, `SELECT `+colunasEspera+` FROM esperas `+q, args...)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	var l []dominio.Espera
	for linhas.Next() {
		e, err := scanEspera(linhas)
		if err != nil {
			return nil, err
		}
		l = append(l, e)
	}
	return l, linhas.Err()
}

// GravarEspera insere; com chave, substitui a espera existente da mesma (automação, chave).
func GravarEspera(ctx context.Context, ex Executor, e dominio.Espera) error {
	dados := string(e.Dados)
	if dados == "" || dados == "null" {
		dados = "{}"
	}
	_, err := ex.ExecContext(ctx, `INSERT INTO esperas (`+colunasEspera+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (automacao_id, chave) DO UPDATE SET id = excluded.id, tipo = excluded.tipo, execucao_id = excluded.execucao_id,
		sessao_id = excluded.sessao_id, conversa_id = excluded.conversa_id, referencia = excluded.referencia,
		retomar_em = excluded.retomar_em, dados = excluded.dados, criada_em = excluded.criada_em`,
		e.ID, e.Tipo, e.AutomacaoID, strOuNil(e.ExecucaoID), strOuNil(e.SessaoID), strOuNil(e.ConversaID), strOuNil(e.Referencia),
		strOuNil(e.Chave), Ms(e.RetomarEm), dados, Ms(e.CriadaEm))
	return err
}

// ObterEspera por id (ok=false se não existe mais).
func ObterEspera(ctx context.Context, ex Executor, id string) (dominio.Espera, bool, error) {
	e, err := scanEspera(ex.QueryRowContext(ctx, `SELECT `+colunasEspera+` FROM esperas WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return e, false, nil
	}
	return e, err == nil, err
}

// EsperasVencidas devolve as esperas com retomar_em <= ate (ordem de vencimento).
func EsperasVencidas(ctx context.Context, ex Executor, ate time.Time, limite int) ([]dominio.Espera, error) {
	return listarEsperas(ctx, ex, `WHERE retomar_em <= ? ORDER BY retomar_em, id LIMIT ?`, Ms(ate), limite)
}

// ProximaEspera devolve o menor retomar_em (ok=false se não há esperas).
func ProximaEspera(ctx context.Context, ex Executor) (time.Time, bool, error) {
	var ms sql.NullInt64
	if err := ex.QueryRowContext(ctx, `SELECT MIN(retomar_em) FROM esperas`).Scan(&ms); err != nil {
		return time.Time{}, false, err
	}
	if !ms.Valid {
		return time.Time{}, false, nil
	}
	return DeMs(ms.Int64), true, nil
}

// ApagarEspera remove por id; devolve true se existia (quem apaga "pega" a espera).
func ApagarEspera(ctx context.Context, ex Executor, id string) (bool, error) {
	r, err := ex.ExecContext(ctx, `DELETE FROM esperas WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}

// ReprogramarEspera muda retomar_em.
func ReprogramarEspera(ctx context.Context, ex Executor, id string, em time.Time) error {
	_, err := ex.ExecContext(ctx, `UPDATE esperas SET retomar_em = ? WHERE id = ?`, Ms(em), id)
	return err
}

// ReprogramarEsperaComDados muda retomar_em e dados.
func ReprogramarEsperaComDados(ctx context.Context, ex Executor, id string, em time.Time, dados json.RawMessage) error {
	_, err := ex.ExecContext(ctx, `UPDATE esperas SET retomar_em = ?, dados = ? WHERE id = ?`, Ms(em), string(dados), id)
	return err
}

// ApagarEsperasDaAutomacao remove as esperas dos tipos dados (desativar automação).
func ApagarEsperasDaAutomacao(ctx context.Context, ex Executor, automacaoID string, tipos ...string) error {
	m, args := marcadores(tipos)
	_, err := ex.ExecContext(ctx, `DELETE FROM esperas WHERE automacao_id = ? AND tipo IN (`+m+`)`, append([]any{automacaoID}, args...)...)
	return err
}

// ApagarEsperasSemRespostaDaConversa remove as esperas sem_resposta da conversa (contato respondeu).
func ApagarEsperasSemRespostaDaConversa(ctx context.Context, ex Executor, conversaID string) error {
	_, err := ex.ExecContext(ctx, `DELETE FROM esperas WHERE conversa_id = ? AND tipo = 'sem_resposta'`, conversaID)
	return err
}

// EsperasDaAutomacao lista as esperas de uma automação e tipo.
func EsperasDaAutomacao(ctx context.Context, ex Executor, automacaoID, tipo string) ([]dominio.Espera, error) {
	return listarEsperas(ctx, ex, `WHERE automacao_id = ? AND tipo = ? ORDER BY retomar_em`, automacaoID, tipo)
}

// ContarEsperasAgendar conta as esperas `agendar` (limite de 1.000 por automação).
func ContarEsperasAgendar(ctx context.Context, ex Executor, automacaoID string) (int, error) {
	var n int
	err := ex.QueryRowContext(ctx, `SELECT count(*) FROM esperas WHERE automacao_id = ? AND tipo = 'agendar'`, automacaoID).Scan(&n)
	return n, err
}

// EsperaDaExecucao devolve a espera `aguardar` de uma execução.
func EsperaDaExecucao(ctx context.Context, ex Executor, execucaoID string) (dominio.Espera, bool, error) {
	l, err := listarEsperas(ctx, ex, `WHERE execucao_id = ? LIMIT 1`, execucaoID)
	if err != nil || len(l) == 0 {
		return dominio.Espera{}, false, err
	}
	return l[0], true, nil
}
