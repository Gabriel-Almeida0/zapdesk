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

// Repositório de execuções (data-model.md › Execução).

// RetencaoExecucoes por automação (as mais antigas além disso são apagadas).
const RetencaoExecucoes = 500

const colunasExecucao = `x.id, x.automacao_id, COALESCE(a.nome, ''), x.tipo_automacao, x.automacao_versao, x.gatilho, x.origem,
	x.origem_execucao_id, x.cadeia, x.conta_id, x.conversa_id, x.contato_id, x.lead_id, x.estado, x.simulacao, x.passo_atual,
	x.acoes, x.tokens, x.retorno, x.retomar_em, x.iniciada_em, x.finalizada_em, x.duracao_ms, x.motivo, x.erro`

const colunasDetalhe = colunasExecucao + `, x.log, x.log_truncado, x.erro_stack, x.variaveis`

const deExecucao = ` FROM execucoes x LEFT JOIN automacoes a ON a.id = x.automacao_id`

func scanExecucao(sc interface{ Scan(...any) error }, detalhe bool) (dominio.ExecucaoDetalhe, error) {
	var d dominio.ExecucaoDetalhe
	e := &d.Execucao
	var gatilho, cadeia, acoes, tokens string
	var origemExec, conta, conversa, contato, lead, retorno, motivo, erro, stack sql.NullString
	var passo, retomar, fim, duracao sql.NullInt64
	var sim int
	var ini int64
	destinos := []any{&e.ID, &e.AutomacaoID, &e.AutomacaoNome, &e.TipoAutomacao, &e.AutomacaoVersao, &gatilho, &e.Origem,
		&origemExec, &cadeia, &conta, &conversa, &contato, &lead, &e.Estado, &sim, &passo, &acoes, &tokens, &retorno, &retomar,
		&ini, &fim, &duracao, &motivo, &erro}
	var variaveis string
	var truncado int
	if detalhe {
		destinos = append(destinos, &d.Log, &truncado, &stack, &variaveis)
	}
	if err := sc.Scan(destinos...); err != nil {
		return d, err
	}
	json.Unmarshal([]byte(gatilho), &e.Gatilho)
	if e.Gatilho.Dados == nil {
		e.Gatilho.Dados = map[string]any{}
	}
	json.Unmarshal([]byte(cadeia), &e.Cadeia)
	e.Acoes = []dominio.AcaoRegistrada{}
	json.Unmarshal([]byte(acoes), &e.Acoes)
	json.Unmarshal([]byte(tokens), &e.Tokens)
	if e.Tokens.PorModelo == nil {
		e.Tokens.PorModelo = map[string]dominio.TokensModelo{}
	}
	e.OrigemExecucaoID, e.ContaID, e.ConversaID, e.ContatoID, e.LeadID = deNullStr(origemExec), deNullStr(conta), deNullStr(conversa), deNullStr(contato), deNullStr(lead)
	e.Simulacao = sim == 1
	e.PassoAtual = deNullInt(passo)
	if retorno.Valid {
		e.Retorno = json.RawMessage(retorno.String)
	} else {
		e.Retorno = json.RawMessage("null")
	}
	e.RetomarEm, e.IniciadaEm, e.FinalizadaEm = deNullMs(retomar), DeMs(ini), deNullMs(fim)
	if duracao.Valid {
		v := duracao.Int64
		e.DuracaoMs = &v
	}
	e.Motivo, e.Erro = deNullStr(motivo), deNullStr(erro)
	if detalhe {
		d.LogTruncado = truncado == 1
		d.ErroStack = deNullStr(stack)
		d.Variaveis = map[string]any{}
		json.Unmarshal([]byte(variaveis), &d.Variaveis)
	}
	return d, nil
}

func argsExecucaoMutaveis(d dominio.ExecucaoDetalhe) []any {
	e := d.Execucao
	var retorno any
	if len(e.Retorno) > 0 && string(e.Retorno) != "null" {
		retorno = string(e.Retorno)
	}
	acoes := e.Acoes
	if acoes == nil {
		acoes = []dominio.AcaoRegistrada{}
	}
	tokens := e.Tokens
	if tokens.PorModelo == nil {
		tokens.PorModelo = map[string]dominio.TokensModelo{}
	}
	variaveis := d.Variaveis
	if variaveis == nil {
		variaveis = map[string]any{}
	}
	var duracao any
	if e.DuracaoMs != nil {
		duracao = *e.DuracaoMs
	}
	return []any{strOuNil(e.ContaID), strOuNil(e.ConversaID), strOuNil(e.ContatoID), strOuNil(e.LeadID), e.Estado,
		intOuNil(e.PassoAtual), jsonTexto(variaveis), jsonTexto(acoes), d.Log, b2i(d.LogTruncado), strOuNil(e.Erro),
		strOuNil(d.ErroStack), jsonTexto(tokens), retorno, MsPtr(e.RetomarEm), MsPtr(e.FinalizadaEm), duracao, strOuNil(e.Motivo)}
}

// InserirExecucao grava uma execução nova e aplica a retenção (500 por automação, só finais).
func InserirExecucao(ctx context.Context, ex Executor, d dominio.ExecucaoDetalhe) error {
	e := d.Execucao
	cadeia := e.Cadeia
	if cadeia == nil {
		cadeia = []string{}
	}
	gat := e.Gatilho
	if gat.Dados == nil {
		gat.Dados = map[string]any{}
	}
	args := []any{e.ID, e.AutomacaoID, e.AutomacaoVersao, e.TipoAutomacao, jsonTexto(gat), e.Origem, strOuNil(e.OrigemExecucaoID),
		jsonTexto(cadeia), b2i(e.Simulacao), Ms(e.IniciadaEm)}
	args = append(args, argsExecucaoMutaveis(d)...)
	_, err := ex.ExecContext(ctx, `INSERT INTO execucoes (id, automacao_id, automacao_versao, tipo_automacao, gatilho, origem,
		origem_execucao_id, cadeia, simulacao, iniciada_em, conta_id, conversa_id, contato_id, lead_id, estado, passo_atual,
		variaveis, acoes, log, log_truncado, erro, erro_stack, tokens, retorno, retomar_em, finalizada_em, duracao_ms, motivo)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, args...)
	if err != nil {
		return err
	}
	_, err = ex.ExecContext(ctx, `DELETE FROM execucoes WHERE automacao_id = ?
		AND estado IN ('ok', 'erro', 'simulacao', 'abortada')
		AND id NOT IN (SELECT id FROM execucoes WHERE automacao_id = ? ORDER BY iniciada_em DESC, id DESC LIMIT ?)`,
		e.AutomacaoID, e.AutomacaoID, RetencaoExecucoes)
	return err
}

// GravarExecucao atualiza os campos mutáveis.
func GravarExecucao(ctx context.Context, ex Executor, d dominio.ExecucaoDetalhe) error {
	args := append(argsExecucaoMutaveis(d), d.ID)
	_, err := ex.ExecContext(ctx, `UPDATE execucoes SET conta_id = ?, conversa_id = ?, contato_id = ?, lead_id = ?, estado = ?,
		passo_atual = ?, variaveis = ?, acoes = ?, log = ?, log_truncado = ?, erro = ?, erro_stack = ?, tokens = ?, retorno = ?,
		retomar_em = ?, finalizada_em = ?, duracao_ms = ?, motivo = ? WHERE id = ?`, args...)
	return err
}

// ObterExecucao devolve o detalhe.
func ObterExecucao(ctx context.Context, ex Executor, id string) (dominio.ExecucaoDetalhe, error) {
	d, err := scanExecucao(ex.QueryRowContext(ctx, `SELECT `+colunasDetalhe+deExecucao+` WHERE x.id = ?`, id), true)
	if errors.Is(err, sql.ErrNoRows) {
		return d, erros.NaoAchada("Execução")
	}
	return d, err
}

// FiltroExecucoes da listagem.
type FiltroExecucoes struct {
	AutomacaoID string
	Estado      string
	ConversaID  string
}

// ListarExecucoes pagina (iniciada_em desc, id desc).
func ListarExecucoes(ctx context.Context, ex Executor, f FiltroExecucoes, p pagina.Params) ([]dominio.Execucao, string, error) {
	var onde []string
	var args []any
	if f.AutomacaoID != "" {
		onde = append(onde, "x.automacao_id = ?")
		args = append(args, f.AutomacaoID)
	}
	if f.Estado != "" {
		onde = append(onde, "x.estado = ?")
		args = append(args, f.Estado)
	}
	if f.ConversaID != "" {
		onde = append(onde, "x.conversa_id = ?")
		args = append(args, f.ConversaID)
	}
	if p.Cursor != nil {
		onde = append(onde, "(x.iniciada_em < ? OR (x.iniciada_em = ? AND x.id < ?))")
		args = append(args, p.Cursor.N, p.Cursor.N, p.Cursor.S)
	}
	q := `SELECT ` + colunasExecucao + deExecucao
	if len(onde) > 0 {
		q += " WHERE " + strings.Join(onde, " AND ")
	}
	args = append(args, p.Limite+1)
	linhas, err := ex.QueryContext(ctx, q+` ORDER BY x.iniciada_em DESC, x.id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, "", err
	}
	defer linhas.Close()
	var lista []dominio.Execucao
	for linhas.Next() {
		d, err := scanExecucao(linhas, false)
		if err != nil {
			return nil, "", err
		}
		lista = append(lista, d.Execucao)
	}
	if err := linhas.Err(); err != nil {
		return nil, "", err
	}
	prox := ""
	if len(lista) > p.Limite {
		lista = lista[:p.Limite]
		u := lista[len(lista)-1]
		prox = pagina.Codificar(Ms(u.IniciadaEm), u.ID)
	}
	return lista, prox, nil
}

// ExecucoesNosEstados lista ids de execuções em algum dos estados (recuperação na subida).
func ExecucoesNosEstados(ctx context.Context, ex Executor, estados ...string) ([]string, error) {
	m, args := marcadores(estados)
	linhas, err := ex.QueryContext(ctx, `SELECT id FROM execucoes WHERE estado IN (`+m+`) ORDER BY iniciada_em`, args...)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	var l []string
	for linhas.Next() {
		var id string
		if err := linhas.Scan(&id); err != nil {
			return nil, err
		}
		l = append(l, id)
	}
	return l, linhas.Err()
}

// ContarEnviosAutomaticos conta mensagens automáticas (qualquer automação) na conversa desde `desde`
// (anti-loop, R10) — opcionalmente só de uma automação.
func ContarEnviosAutomaticos(ctx context.Context, ex Executor, conversaID, automacaoID string, desde time.Time) (int, error) {
	q := `SELECT count(*) FROM mensagens WHERE conversa_id = ? AND automacao_id IS NOT NULL AND enviada_em >= ?`
	args := []any{conversaID, Ms(desde)}
	if automacaoID != "" {
		q = `SELECT count(*) FROM mensagens WHERE conversa_id = ? AND automacao_id = ? AND enviada_em >= ?`
		args = []any{conversaID, automacaoID, Ms(desde)}
	}
	var n int
	err := ex.QueryRowContext(ctx, q, args...).Scan(&n)
	return n, err
}

// ContarPrimeirosContatos conta envios automáticos que iniciaram conversa na conta desde `desde`.
func ContarPrimeirosContatos(ctx context.Context, ex Executor, contaID string, desde time.Time) (int, error) {
	var n int
	err := ex.QueryRowContext(ctx, `SELECT count(*) FROM mensagens WHERE conta_id = ? AND primeiro_contato = 1 AND enviada_em >= ?`,
		contaID, Ms(desde)).Scan(&n)
	return n, err
}
