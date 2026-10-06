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
)

// Repositório de automações (data-model.md › Automação).

const colunasAutomacao = `id, tipo, nome, descricao, ativa, contas, incluir_grupos, prioridade, conta_envio_id, gatilhos,
	definicao, limites, versao, permissoes, segredos, hash_fontes, hash_compilado, erros_compilacao, compilado_em,
	erros_seguidos, desativada_motivo, criada_em, atualizada_em`

func scanAutomacao(sc interface{ Scan(...any) error }) (dominio.Automacao, error) {
	var a dominio.Automacao
	var desc, contas, contaEnvio, definicao, permissoes, segredos, hf, hc, motivo sql.NullString
	var gatilhos, limites, errosComp string
	var ativa, grupos int
	var compilado sql.NullInt64
	var cri, atu int64
	if err := sc.Scan(&a.ID, &a.Tipo, &a.Nome, &desc, &ativa, &contas, &grupos, &a.Prioridade, &contaEnvio, &gatilhos,
		&definicao, &limites, &a.Versao, &permissoes, &segredos, &hf, &hc, &errosComp, &compilado, &a.ErrosSeguidos,
		&motivo, &cri, &atu); err != nil {
		return a, err
	}
	a.Descricao, a.ContaEnvioID, a.HashFontes, a.HashCompilado, a.DesativadaMotivo = deNullStr(desc), deNullStr(contaEnvio), deNullStr(hf), deNullStr(hc), deNullStr(motivo)
	a.Ativa, a.IncluirGrupos = ativa == 1, grupos == 1
	if contas.Valid {
		a.Contas = []string{}
		json.Unmarshal([]byte(contas.String), &a.Contas)
	}
	a.Gatilhos = json.RawMessage(gatilhos)
	if definicao.Valid {
		a.Definicao = json.RawMessage(definicao.String)
	} else {
		a.Definicao = json.RawMessage("null")
	}
	json.Unmarshal([]byte(limites), &a.Limites)
	if permissoes.Valid {
		json.Unmarshal([]byte(permissoes.String), &a.Permissoes)
	}
	if segredos.Valid {
		json.Unmarshal([]byte(segredos.String), &a.Segredos)
	}
	a.ErrosCompilacao = []dominio.ErroCompilacao{}
	json.Unmarshal([]byte(errosComp), &a.ErrosCompilacao)
	a.CompiladoEm = deNullMs(compilado)
	a.CriadaEm, a.AtualizadaEm = DeMs(cri), DeMs(atu)
	a.Avisos = []dominio.ErroDefinicao{}
	return a, nil
}

func argsAutomacao(a dominio.Automacao) []any {
	var contas, definicao, permissoes, segredos any
	if a.Contas != nil {
		contas = jsonTexto(a.Contas)
	}
	if len(a.Definicao) > 0 && string(a.Definicao) != "null" {
		definicao = string(a.Definicao)
	}
	if a.Permissoes != nil {
		permissoes = jsonTexto(a.Permissoes)
	}
	if a.Segredos != nil {
		segredos = jsonTexto(a.Segredos)
	}
	gatilhos := string(a.Gatilhos)
	if gatilhos == "" || gatilhos == "null" {
		gatilhos = "[]"
	}
	errosComp := a.ErrosCompilacao
	if errosComp == nil {
		errosComp = []dominio.ErroCompilacao{}
	}
	return []any{a.ID, a.Tipo, a.Nome, strOuNil(a.Descricao), b2i(a.Ativa), contas, b2i(a.IncluirGrupos), a.Prioridade,
		strOuNil(a.ContaEnvioID), gatilhos, definicao, jsonTexto(a.Limites), a.Versao, permissoes, segredos,
		strOuNil(a.HashFontes), strOuNil(a.HashCompilado), jsonTexto(errosComp), MsPtr(a.CompiladoEm), a.ErrosSeguidos,
		strOuNil(a.DesativadaMotivo), Ms(a.CriadaEm), Ms(a.AtualizadaEm)}
}

// InserirAutomacao grava uma automação nova.
func InserirAutomacao(ctx context.Context, ex Executor, a dominio.Automacao) error {
	_, err := ex.ExecContext(ctx, `INSERT INTO automacoes (`+colunasAutomacao+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		argsAutomacao(a)...)
	return err
}

// GravarAutomacao sobrescreve todos os campos.
func GravarAutomacao(ctx context.Context, ex Executor, a dominio.Automacao) error {
	args := argsAutomacao(a)
	_, err := ex.ExecContext(ctx, `UPDATE automacoes SET tipo = ?, nome = ?, descricao = ?, ativa = ?, contas = ?, incluir_grupos = ?,
		prioridade = ?, conta_envio_id = ?, gatilhos = ?, definicao = ?, limites = ?, versao = ?, permissoes = ?, segredos = ?,
		hash_fontes = ?, hash_compilado = ?, erros_compilacao = ?, compilado_em = ?, erros_seguidos = ?, desativada_motivo = ?,
		criada_em = ?, atualizada_em = ? WHERE id = ?`, append(args[1:], a.ID)...)
	return err
}

// ObterAutomacao por id.
func ObterAutomacao(ctx context.Context, ex Executor, id string) (dominio.Automacao, error) {
	a, err := scanAutomacao(ex.QueryRowContext(ctx, `SELECT `+colunasAutomacao+` FROM automacoes WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return a, erros.NaoAchada("Automação")
	}
	return a, err
}

// FiltroAutomacoes da listagem.
type FiltroAutomacoes struct {
	Tipo  string
	Ativa *bool
	Busca string
}

// ListarAutomacoes (ordem: prioridade, criada_em).
func ListarAutomacoes(ctx context.Context, ex Executor, f FiltroAutomacoes) ([]dominio.Automacao, error) {
	var onde []string
	var args []any
	if f.Tipo != "" {
		onde = append(onde, "tipo = ?")
		args = append(args, f.Tipo)
	}
	if f.Ativa != nil {
		onde = append(onde, "ativa = ?")
		args = append(args, b2i(*f.Ativa))
	}
	if f.Busca != "" {
		onde = append(onde, `(nome LIKE ? ESCAPE '\' OR descricao LIKE ? ESCAPE '\')`)
		b := escaparLike(f.Busca)
		args = append(args, b, b)
	}
	q := `SELECT ` + colunasAutomacao + ` FROM automacoes`
	if len(onde) > 0 {
		q += " WHERE " + strings.Join(onde, " AND ")
	}
	linhas, err := ex.QueryContext(ctx, q+` ORDER BY prioridade, criada_em, id`, args...)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	lista := []dominio.Automacao{}
	for linhas.Next() {
		a, err := scanAutomacao(linhas)
		if err != nil {
			return nil, err
		}
		lista = append(lista, a)
	}
	return lista, linhas.Err()
}

// ExcluirAutomacao apaga (cascata: sessões, execuções, esperas, memória).
func ExcluirAutomacao(ctx context.Context, ex Executor, id string) (bool, error) {
	r, err := ex.ExecContext(ctx, `DELETE FROM automacoes WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}

// ContarAutomacoesAtivas para GET /sistema.
func ContarAutomacoesAtivas(ctx context.Context, ex Executor) (int, error) {
	var n int
	err := ex.QueryRowContext(ctx, `SELECT count(*) FROM automacoes WHERE ativa = 1`).Scan(&n)
	return n, err
}

// RegistrarResultadoErros zera (ok) ou incrementa (erro) erros_seguidos; devolve o valor novo.
func RegistrarResultadoErros(ctx context.Context, ex Executor, id string, erro bool) (int, error) {
	q := `UPDATE automacoes SET erros_seguidos = 0 WHERE id = ?`
	if erro {
		q = `UPDATE automacoes SET erros_seguidos = erros_seguidos + 1 WHERE id = ?`
	}
	if _, err := ex.ExecContext(ctx, q, id); err != nil {
		return 0, err
	}
	var n int
	err := ex.QueryRowContext(ctx, `SELECT erros_seguidos FROM automacoes WHERE id = ?`, id).Scan(&n)
	return n, err
}

// DesativarAutomacao marca inativa com o motivo; devolve true se estava ativa.
func DesativarAutomacao(ctx context.Context, ex Executor, id, motivo string, agora time.Time) (bool, error) {
	r, err := ex.ExecContext(ctx, `UPDATE automacoes SET ativa = 0, desativada_motivo = ?, atualizada_em = ? WHERE id = ? AND ativa = 1`,
		motivo, Ms(agora), id)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}

// EstatisticasAutomacoes devolve contagens das execuções não simuladas desde `desde`.
func EstatisticasAutomacoes(ctx context.Context, ex Executor, desde time.Time) (map[string]dominio.Estatisticas24h, error) {
	linhas, err := ex.QueryContext(ctx, `SELECT automacao_id,
		SUM(estado = 'ok'), SUM(estado = 'erro'), SUM(estado = 'abortada'),
		AVG(CASE WHEN estado IN ('ok', 'erro') THEN duracao_ms END)
		FROM execucoes WHERE simulacao = 0 AND iniciada_em >= ? GROUP BY automacao_id`, Ms(desde))
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	m := map[string]dominio.Estatisticas24h{}
	for linhas.Next() {
		var id string
		var e dominio.Estatisticas24h
		var media sql.NullFloat64
		if err := linhas.Scan(&id, &e.OK, &e.Erro, &e.Abortada, &media); err != nil {
			return nil, err
		}
		if media.Valid {
			v := int(media.Float64 + 0.5)
			e.DuracaoMediaMs = &v
		}
		m[id] = e
	}
	return m, linhas.Err()
}

// SessoesAtivasPorAutomacao conta as sessões ativas.
func SessoesAtivasPorAutomacao(ctx context.Context, ex Executor) (map[string]int, error) {
	linhas, err := ex.QueryContext(ctx, `SELECT automacao_id, count(*) FROM sessoes_chatbot WHERE estado = 'ativa' GROUP BY automacao_id`)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	m := map[string]int{}
	for linhas.Next() {
		var id string
		var n int
		if err := linhas.Scan(&id, &n); err != nil {
			return nil, err
		}
		m[id] = n
	}
	return m, linhas.Err()
}

// ---------------------------------------------------------------------------
// Configuração (tabela configuracoes, chaves "automacoes.*" e "ia.*")
// ---------------------------------------------------------------------------

// LerConfiguracao devolve o valor JSON de uma chave (ok=false se ausente).
func LerConfiguracao(ctx context.Context, ex Executor, chave string, destino any) (bool, error) {
	var v string
	err := ex.QueryRowContext(ctx, `SELECT valor FROM configuracoes WHERE chave = ?`, chave).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal([]byte(v), destino)
}

// GravarConfiguracao grava o valor (JSON) de uma chave.
func GravarConfiguracao(ctx context.Context, ex Executor, chave string, valor any) error {
	_, err := ex.ExecContext(ctx, `INSERT INTO configuracoes (chave, valor) VALUES (?, ?)
		ON CONFLICT (chave) DO UPDATE SET valor = excluded.valor`, chave, jsonTexto(valor))
	return err
}
