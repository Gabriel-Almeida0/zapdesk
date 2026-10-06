package armazenamento

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/pagina"
)

const colunasDisparo = `d.id, d.conta_id, d.nome, d.mensagem, d.arquivo_id, d.intervalo_min_s, d.intervalo_max_s, d.limite_por_hora,
	d.limite_por_dia, d.pausa_a_cada, d.pausa_duracao_s, d.inicio_em, d.janela_inicio, d.janela_fim, d.falhas_seguidas_max,
	d.valores_padrao, d.estado, d.motivo_pausa, d.falhas_seguidas, d.origem, d.proximo_envio_em, d.ultimo_envio_em,
	d.enviados_desde_pausa, d.fila_desde, d.criado_em, d.iniciado_em, d.concluido_em, d.cancelado_em`

func scanDisparo(sc interface{ Scan(...any) error }) (dominio.Disparo, error) {
	var d dominio.Disparo
	var arq, jIni, jFim, motivo sql.NullString
	var lh, ld, pc, pd, inicio, prox, ultimo, fila, iniciado, concluido, cancelado sql.NullInt64
	var valores string
	var criado int64
	err := sc.Scan(&d.ID, &d.ContaID, &d.Nome, &d.Mensagem, &arq, &d.Ritmo.IntervaloMinS, &d.Ritmo.IntervaloMaxS, &lh, &ld, &pc, &pd,
		&inicio, &jIni, &jFim, &d.FalhasSeguidasMax, &valores, &d.Estado, &motivo, &d.FalhasSeguidas, &d.Origem, &prox, &ultimo,
		&d.EnviadosDesdePausa, &fila, &criado, &iniciado, &concluido, &cancelado)
	if err != nil {
		return d, err
	}
	d.ArquivoID = deNullStr(arq)
	d.Ritmo.LimitePorHora, d.Ritmo.LimitePorDia = deNullInt(lh), deNullInt(ld)
	d.Ritmo.PausaACada, d.Ritmo.PausaDuracaoS = deNullInt(pc), deNullInt(pd)
	d.InicioEm, d.ProximoEnvioEm, d.UltimoEnvioEm, d.FilaDesde = deNullMs(inicio), deNullMs(prox), deNullMs(ultimo), deNullMs(fila)
	d.IniciadoEm, d.ConcluidoEm, d.CanceladoEm, d.CriadoEm = deNullMs(iniciado), deNullMs(concluido), deNullMs(cancelado), DeMs(criado)
	if jIni.Valid && jFim.Valid {
		d.Janela = &dominio.Janela{Inicio: jIni.String, Fim: jFim.String}
	}
	d.MotivoPausa = deNullStr(motivo)
	d.ValoresPadrao = map[string]string{}
	json.Unmarshal([]byte(valores), &d.ValoresPadrao)
	return d, nil
}

// InserirDisparo cria o disparo (estado rascunho ou já agendado).
func InserirDisparo(ctx context.Context, ex Executor, d dominio.Disparo) error {
	var jIni, jFim any
	if d.Janela != nil {
		jIni, jFim = d.Janela.Inicio, d.Janela.Fim
	}
	_, err := ex.ExecContext(ctx, `INSERT INTO disparos (id, conta_id, nome, mensagem, arquivo_id, intervalo_min_s, intervalo_max_s,
		limite_por_hora, limite_por_dia, pausa_a_cada, pausa_duracao_s, inicio_em, janela_inicio, janela_fim, falhas_seguidas_max,
		valores_padrao, estado, origem, criado_em) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.ContaID, d.Nome, d.Mensagem, strOuNil(d.ArquivoID), d.Ritmo.IntervaloMinS, d.Ritmo.IntervaloMaxS,
		intOuNil(d.Ritmo.LimitePorHora), intOuNil(d.Ritmo.LimitePorDia), intOuNil(d.Ritmo.PausaACada), intOuNil(d.Ritmo.PausaDuracaoS),
		MsPtr(d.InicioEm), jIni, jFim, d.FalhasSeguidasMax, jsonTexto(d.ValoresPadrao), d.Estado, d.Origem, Ms(d.CriadoEm))
	return err
}

// AtualizarRascunho grava os campos editáveis de um rascunho.
func AtualizarRascunho(ctx context.Context, ex Executor, d dominio.Disparo) error {
	var jIni, jFim any
	if d.Janela != nil {
		jIni, jFim = d.Janela.Inicio, d.Janela.Fim
	}
	r, err := ex.ExecContext(ctx, `UPDATE disparos SET nome = ?, mensagem = ?, arquivo_id = ?, intervalo_min_s = ?, intervalo_max_s = ?,
		limite_por_hora = ?, limite_por_dia = ?, pausa_a_cada = ?, pausa_duracao_s = ?, inicio_em = ?, janela_inicio = ?, janela_fim = ?,
		falhas_seguidas_max = ?, valores_padrao = ? WHERE id = ? AND estado = 'rascunho'`,
		d.Nome, d.Mensagem, strOuNil(d.ArquivoID), d.Ritmo.IntervaloMinS, d.Ritmo.IntervaloMaxS, intOuNil(d.Ritmo.LimitePorHora),
		intOuNil(d.Ritmo.LimitePorDia), intOuNil(d.Ritmo.PausaACada), intOuNil(d.Ritmo.PausaDuracaoS), MsPtr(d.InicioEm), jIni, jFim,
		d.FalhasSeguidasMax, jsonTexto(d.ValoresPadrao), d.ID)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return erros.Transicao("Só é possível editar um disparo em rascunho.", "")
	}
	return nil
}

// DefinirValoresPadrao grava os valores padrão.
func DefinirValoresPadrao(ctx context.Context, ex Executor, id string, v map[string]string) error {
	_, err := ex.ExecContext(ctx, `UPDATE disparos SET valores_padrao = ? WHERE id = ?`, jsonTexto(v), id)
	return err
}

// ObterDisparo busca a linha (sem contadores).
func ObterDisparo(ctx context.Context, ex Executor, id string) (dominio.Disparo, error) {
	d, err := scanDisparo(ex.QueryRowContext(ctx, `SELECT `+colunasDisparo+` FROM disparos d WHERE d.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return d, erros.NaoAchado("Disparo")
	}
	return d, err
}

// ListarDisparos pagina por (criado_em desc, id desc).
func ListarDisparos(ctx context.Context, ex Executor, contaID, estado string, p pagina.Params) ([]dominio.Disparo, string, error) {
	var onde []string
	var args []any
	if contaID != "" {
		onde = append(onde, "d.conta_id = ?")
		args = append(args, contaID)
	}
	if estado != "" {
		onde = append(onde, "d.estado = ?")
		args = append(args, estado)
	}
	if p.Cursor != nil {
		onde = append(onde, "(d.criado_em < ? OR (d.criado_em = ? AND d.id < ?))")
		args = append(args, p.Cursor.N, p.Cursor.N, p.Cursor.S)
	}
	q := `SELECT ` + colunasDisparo + ` FROM disparos d`
	if len(onde) > 0 {
		q += " WHERE " + strings.Join(onde, " AND ")
	}
	q += " ORDER BY d.criado_em DESC, d.id DESC LIMIT ?"
	args = append(args, p.Limite+1)
	linhas, err := ex.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, "", err
	}
	defer linhas.Close()
	var lista []dominio.Disparo
	for linhas.Next() {
		d, err := scanDisparo(linhas)
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
		u := lista[len(lista)-1]
		prox = pagina.Codificar(Ms(u.CriadoEm), u.ID)
	}
	return lista, prox, nil
}

// DisparosDaConta devolve os disparos da conta nos estados dados, pela ordem de fila.
func DisparosDaConta(ctx context.Context, ex Executor, contaID string, estados ...string) ([]dominio.Disparo, error) {
	m, args := marcadores(estados)
	args = append([]any{contaID}, args...)
	linhas, err := ex.QueryContext(ctx, `SELECT `+colunasDisparo+` FROM disparos d WHERE d.conta_id = ? AND d.estado IN (`+m+`)
		ORDER BY COALESCE(d.fila_desde, d.criado_em), d.criado_em, d.id`, args...)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	var lista []dominio.Disparo
	for linhas.Next() {
		d, err := scanDisparo(linhas)
		if err != nil {
			return nil, err
		}
		lista = append(lista, d)
	}
	return lista, linhas.Err()
}

// DisparosNosEstados devolve os disparos (todas as contas) nos estados dados.
func DisparosNosEstados(ctx context.Context, ex Executor, estados ...string) ([]dominio.Disparo, error) {
	m, args := marcadores(estados)
	linhas, err := ex.QueryContext(ctx, `SELECT `+colunasDisparo+` FROM disparos d WHERE d.estado IN (`+m+`) ORDER BY d.criado_em`, args...)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	var lista []dominio.Disparo
	for linhas.Next() {
		d, err := scanDisparo(linhas)
		if err != nil {
			return nil, err
		}
		lista = append(lista, d)
	}
	return lista, linhas.Err()
}

// MudarEstadoDisparo aplica a transição se o estado atual for `de` (condicional contra
// corridas). Ajusta datas e motivo conforme o novo estado. Devolve false se não mudou.
func MudarEstadoDisparo(ctx context.Context, ex Executor, id, de, para string, motivo *string, agora time.Time) (bool, error) {
	sets := []string{"estado = ?"}
	args := []any{para}
	switch para {
	case dominio.DisparoAgendado:
		sets = append(sets, "iniciado_em = COALESCE(iniciado_em, ?)", "fila_desde = ?", "motivo_pausa = NULL", "proximo_envio_em = NULL", "falhas_seguidas = 0")
		args = append(args, Ms(agora), Ms(agora))
	case dominio.DisparoPausado:
		sets = append(sets, "motivo_pausa = ?", "proximo_envio_em = NULL")
		args = append(args, strOuNil(motivo))
	case dominio.DisparoConcluido:
		sets = append(sets, "concluido_em = ?", "proximo_envio_em = NULL", "motivo_pausa = NULL")
		args = append(args, Ms(agora))
	case dominio.DisparoCancelado:
		sets = append(sets, "cancelado_em = ?", "proximo_envio_em = NULL")
		args = append(args, Ms(agora))
	}
	args = append(args, id, de)
	r, err := ex.ExecContext(ctx, `UPDATE disparos SET `+strings.Join(sets, ", ")+` WHERE id = ? AND estado = ?`, args...)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}

// DefinirProximoEnvio grava o próximo envio calculado (nil = recalcular).
func DefinirProximoEnvio(ctx context.Context, ex Executor, id string, em *time.Time) error {
	_, err := ex.ExecContext(ctx, `UPDATE disparos SET proximo_envio_em = ? WHERE id = ?`, MsPtr(em), id)
	return err
}

// LimparProximoEnvioAtivos força o recálculo da agenda (acordou do sono).
func LimparProximoEnvioAtivos(ctx context.Context, ex Executor) error {
	_, err := ex.ExecContext(ctx, `UPDATE disparos SET proximo_envio_em = NULL WHERE estado IN ('agendado', 'enviando', 'fora_da_janela')`)
	return err
}

// RegistrarTentativa grava o instante do último envio e o contador de pausa.
func RegistrarTentativa(ctx context.Context, ex Executor, id string, em time.Time, enviadosDesdePausa int) error {
	_, err := ex.ExecContext(ctx, `UPDATE disparos SET ultimo_envio_em = ?, enviados_desde_pausa = ? WHERE id = ?`, Ms(em), enviadosDesdePausa, id)
	return err
}

// DefinirFalhasSeguidas grava o contador corrente.
func DefinirFalhasSeguidas(ctx context.Context, ex Executor, id string, n int) error {
	_, err := ex.ExecContext(ctx, `UPDATE disparos SET falhas_seguidas = ? WHERE id = ?`, n, id)
	return err
}

// ExcluirDisparo apaga um rascunho.
func ExcluirDisparo(ctx context.Context, ex Executor, id string) (bool, error) {
	r, err := ex.ExecContext(ctx, `DELETE FROM disparos WHERE id = ? AND estado = 'rascunho'`, id)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}

// ContadoresDisparo conta destinatários por estado.
func ContadoresDisparo(ctx context.Context, ex Executor, id string) (dominio.Contadores, error) {
	var c dominio.Contadores
	linhas, err := ex.QueryContext(ctx, `SELECT estado, count(*) FROM destinatarios WHERE disparo_id = ? GROUP BY estado`, id)
	if err != nil {
		return c, err
	}
	defer linhas.Close()
	for linhas.Next() {
		var estado string
		var n int
		if err := linhas.Scan(&estado, &n); err != nil {
			return c, err
		}
		c.Total += n
		switch estado {
		case dominio.DestPendente:
			c.Pendente = n
		case dominio.DestEnviando:
			c.Enviando = n
		case dominio.DestEnviado:
			c.Enviado = n
		case dominio.DestEntregue:
			c.Entregue = n
		case dominio.DestLido:
			c.Lido = n
		case dominio.DestRespondeu:
			c.Respondeu = n
		case dominio.DestFalhou:
			c.Falhou = n
		}
	}
	return c, linhas.Err()
}

// ContarAtivos conta disparos em agendado (iniciado), enviando ou fora_da_janela.
func ContarAtivos(ctx context.Context, ex Executor) (int, error) {
	var n int
	err := ex.QueryRowContext(ctx, `SELECT count(*) FROM disparos WHERE estado IN ('enviando', 'fora_da_janela')
		OR (estado = 'agendado' AND iniciado_em IS NOT NULL)`).Scan(&n)
	return n, err
}
