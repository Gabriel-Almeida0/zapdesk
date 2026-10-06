package armazenamento

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/pagina"
)

const colunasDest = `id, disparo_id, lead_id, ordem, telefone, nome, jid, variaveis, estado, motivo_falha, mensagem_wa_id,
	enviando_em, enviado_em, entregue_em, lido_em, respondeu_em, falhou_em`

func scanDest(sc interface{ Scan(...any) error }) (dominio.Destinatario, error) {
	var d dominio.Destinatario
	var nome, jid, motivo, wa sql.NullString
	var vars string
	var e1, e2, e3, e4, e5, e6 sql.NullInt64
	if err := sc.Scan(&d.ID, &d.DisparoID, &d.LeadID, &d.Ordem, &d.Telefone, &nome, &jid, &vars, &d.Estado, &motivo, &wa,
		&e1, &e2, &e3, &e4, &e5, &e6); err != nil {
		return d, err
	}
	d.Nome, d.JID, d.MotivoFalha, d.MensagemWaID = deNullStr(nome), deNullStr(jid), deNullStr(motivo), deNullStr(wa)
	d.EnviandoEm, d.EnviadoEm, d.EntregueEm, d.LidoEm, d.RespondeuEm, d.FalhouEm = deNullMs(e1), deNullMs(e2), deNullMs(e3), deNullMs(e4), deNullMs(e5), deNullMs(e6)
	d.Variaveis = map[string]string{}
	json.Unmarshal([]byte(vars), &d.Variaveis)
	return d, nil
}

// InserirDestinatarios grava a lista (ordem já preenchida).
func InserirDestinatarios(ctx context.Context, ex Executor, lista []dominio.Destinatario) error {
	for inicio := 0; inicio < len(lista); inicio += 200 {
		fim := min(inicio+200, len(lista))
		var b strings.Builder
		b.WriteString(`INSERT INTO destinatarios (id, disparo_id, lead_id, ordem, telefone, nome, variaveis) VALUES `)
		args := make([]any, 0, (fim-inicio)*7)
		for i, d := range lista[inicio:fim] {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString("(?, ?, ?, ?, ?, ?, ?)")
			args = append(args, d.ID, d.DisparoID, d.LeadID, d.Ordem, d.Telefone, strOuNil(d.Nome), jsonTexto(d.Variaveis))
		}
		if _, err := ex.ExecContext(ctx, b.String(), args...); err != nil {
			return err
		}
	}
	return nil
}

// ObterDestinatario por id.
func ObterDestinatario(ctx context.Context, ex Executor, id string) (dominio.Destinatario, error) {
	return scanDest(ex.QueryRowContext(ctx, `SELECT `+colunasDest+` FROM destinatarios WHERE id = ?`, id))
}

// ProximoPendente devolve o próximo destinatário pendente (menor ordem).
func ProximoPendente(ctx context.Context, ex Executor, disparoID string) (dominio.Destinatario, bool, error) {
	d, err := scanDest(ex.QueryRowContext(ctx, `SELECT `+colunasDest+` FROM destinatarios WHERE disparo_id = ? AND estado = 'pendente'
		ORDER BY ordem LIMIT 1`, disparoID))
	if errors.Is(err, sql.ErrNoRows) {
		return d, false, nil
	}
	return d, err == nil, err
}

// MarcarEnviando commita "enviando" antes do envio (só se o disparo ainda estiver enviando).
func MarcarEnviando(ctx context.Context, ex Executor, id, disparoID string, em time.Time) (bool, error) {
	r, err := ex.ExecContext(ctx, `UPDATE destinatarios SET estado = 'enviando', enviando_em = ? WHERE id = ? AND estado = 'pendente'
		AND (SELECT estado FROM disparos WHERE id = ?) = 'enviando'`, Ms(em), id, disparoID)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}

// VoltarPendenteDest desfaz "enviando" quando nada saiu (sem conexão antes de enviar).
func VoltarPendenteDest(ctx context.Context, ex Executor, id string) error {
	_, err := ex.ExecContext(ctx, `UPDATE destinatarios SET estado = 'pendente', enviando_em = NULL WHERE id = ? AND estado = 'enviando'`, id)
	return err
}

// MarcarEnviadoDest grava enviado + mensagem_wa_id.
func MarcarEnviadoDest(ctx context.Context, ex Executor, id, waID, jid string, em time.Time) error {
	_, err := ex.ExecContext(ctx, `UPDATE destinatarios SET estado = 'enviado', mensagem_wa_id = ?, jid = ?, enviado_em = ? WHERE id = ? AND estado = 'enviando'`,
		waID, vazioNil(jid), Ms(em), id)
	return err
}

// MarcarFalhouDest grava falhou com motivo (a partir de pendente ou enviando).
func MarcarFalhouDest(ctx context.Context, ex Executor, id, motivo string, em time.Time) error {
	_, err := ex.ExecContext(ctx, `UPDATE destinatarios SET estado = 'falhou', motivo_falha = ?, falhou_em = ? WHERE id = ? AND estado IN ('pendente', 'enviando')`,
		motivo, Ms(em), id)
	return err
}

// DestinatariosEnviando lista todos os destinatários presos em "enviando" (reconciliação).
func DestinatariosEnviando(ctx context.Context, ex Executor) ([]dominio.Destinatario, error) {
	linhas, err := ex.QueryContext(ctx, `SELECT `+colunasDest+` FROM destinatarios WHERE estado = 'enviando'`)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	var lista []dominio.Destinatario
	for linhas.Next() {
		d, err := scanDest(linhas)
		if err != nil {
			return nil, err
		}
		lista = append(lista, d)
	}
	return lista, linhas.Err()
}

// EnviosRecentes devolve os instantes de tentativa (enviando_em) do disparo desde `desde`.
func EnviosRecentes(ctx context.Context, ex Executor, disparoID string, desde time.Time) ([]time.Time, error) {
	linhas, err := ex.QueryContext(ctx, `SELECT enviando_em FROM destinatarios WHERE disparo_id = ? AND enviando_em IS NOT NULL AND enviando_em > ?
		ORDER BY enviando_em`, disparoID, Ms(desde))
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	var lista []time.Time
	for linhas.Next() {
		var ms int64
		if err := linhas.Scan(&ms); err != nil {
			return nil, err
		}
		lista = append(lista, DeMs(ms))
	}
	return lista, linhas.Err()
}

// AplicarReciboDest aplica entregue/lido sem regredir; respondeu só ganha datas. Devolve os ids alterados.
func AplicarReciboDest(ctx context.Context, ex Executor, contaID, waID, tipo string, em time.Time) ([]string, error) {
	var q string
	switch tipo {
	case "entregue":
		q = `UPDATE destinatarios SET
			estado = CASE WHEN estado = 'enviado' THEN 'entregue' ELSE estado END,
			entregue_em = COALESCE(entregue_em, ?)
			WHERE mensagem_wa_id = ? AND estado IN ('enviado', 'entregue', 'lido', 'respondeu') AND entregue_em IS NULL
			AND disparo_id IN (SELECT id FROM disparos WHERE conta_id = ?) RETURNING id`
	case "lido":
		q = `UPDATE destinatarios SET
			estado = CASE WHEN estado IN ('enviado', 'entregue') THEN 'lido' ELSE estado END,
			entregue_em = COALESCE(entregue_em, ?1), lido_em = COALESCE(lido_em, ?1)
			WHERE mensagem_wa_id = ?2 AND estado IN ('enviado', 'entregue', 'lido', 'respondeu') AND lido_em IS NULL
			AND disparo_id IN (SELECT id FROM disparos WHERE conta_id = ?3) RETURNING id`
	default:
		return nil, nil
	}
	return idsAfetados(ctx, ex, q, Ms(em), waID, contaID)
}

// MarcarRespondeu marca como respondeu os destinatários daquele telefone na conta cujo envio
// foi antes da mensagem recebida.
func MarcarRespondeu(ctx context.Context, ex Executor, contaID, telefone string, em time.Time) ([]string, error) {
	return idsAfetados(ctx, ex, `UPDATE destinatarios SET estado = 'respondeu', respondeu_em = ?1
		WHERE telefone = ?2 AND estado IN ('enviado', 'entregue', 'lido') AND enviado_em <= ?1
		AND disparo_id IN (SELECT id FROM disparos WHERE conta_id = ?3) RETURNING id`, Ms(em), telefone, contaID)
}

func idsAfetados(ctx context.Context, ex Executor, q string, args ...any) ([]string, error) {
	linhas, err := ex.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	var lista []string
	for linhas.Next() {
		var id string
		if err := linhas.Scan(&id); err != nil {
			return nil, err
		}
		lista = append(lista, id)
	}
	return lista, linhas.Err()
}

// FiltroDestinatarios da listagem.
type FiltroDestinatarios struct {
	Estado string
	Busca  string
}

// ListarDestinatarios pagina por ordem.
func ListarDestinatarios(ctx context.Context, ex Executor, disparoID string, f FiltroDestinatarios, p pagina.Params) ([]dominio.Destinatario, string, error) {
	onde := []string{"disparo_id = ?"}
	args := []any{disparoID}
	if f.Estado != "" {
		onde = append(onde, "estado = ?")
		args = append(args, f.Estado)
	}
	if f.Busca != "" {
		onde = append(onde, `(telefone LIKE ? ESCAPE '\' OR nome LIKE ? ESCAPE '\')`)
		b := escaparLike(f.Busca)
		args = append(args, b, b)
	}
	if p.Cursor != nil {
		onde = append(onde, "ordem > ?")
		args = append(args, p.Cursor.N)
	}
	args = append(args, p.Limite+1)
	linhas, err := ex.QueryContext(ctx, `SELECT `+colunasDest+` FROM destinatarios WHERE `+strings.Join(onde, " AND ")+` ORDER BY ordem LIMIT ?`, args...)
	if err != nil {
		return nil, "", err
	}
	defer linhas.Close()
	var lista []dominio.Destinatario
	for linhas.Next() {
		d, err := scanDest(linhas)
		if err != nil {
			return nil, "", err
		}
		lista = append(lista, d)
	}
	if err := linhas.Err(); err != nil {
		return nil, "", err
	}
	prox := ""
	if len(lista) > p.Limite {
		lista = lista[:p.Limite]
		prox = pagina.Codificar(int64(lista[len(lista)-1].Ordem), "")
	}
	return lista, prox, nil
}

// PercorrerDestinatarios chama fn para cada destinatário na ordem (relatório CSV).
func PercorrerDestinatarios(ctx context.Context, ex Executor, disparoID string, fn func(dominio.Destinatario) error) error {
	linhas, err := ex.QueryContext(ctx, `SELECT `+colunasDest+` FROM destinatarios WHERE disparo_id = ? ORDER BY ordem`, disparoID)
	if err != nil {
		return err
	}
	defer linhas.Close()
	for linhas.Next() {
		d, err := scanDest(linhas)
		if err != nil {
			return err
		}
		if err := fn(d); err != nil {
			return err
		}
	}
	return linhas.Err()
}

// AtualizarVariaveisDest grava os valores resolvidos.
func AtualizarVariaveisDest(ctx context.Context, ex Executor, id string, v map[string]string) error {
	_, err := ex.ExecContext(ctx, `UPDATE destinatarios SET variaveis = ? WHERE id = ?`, jsonTexto(v), id)
	return err
}
