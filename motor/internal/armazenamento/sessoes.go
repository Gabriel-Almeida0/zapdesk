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

// Repositório de sessões de chatbot (data-model.md › Sessão de chatbot).

const colunasSessao = `s.id, s.automacao_id, COALESCE(a.nome, ''), s.conversa_id, s.versao, s.definicao, s.no_atual, s.variaveis,
	s.tentativas, s.estado, s.motivo, s.expira_em, s.iniciada_em, s.atualizada_em, s.finalizada_em, COALESCE(cv.conta_id, '')`

const deSessao = ` FROM sessoes_chatbot s LEFT JOIN automacoes a ON a.id = s.automacao_id LEFT JOIN conversas cv ON cv.id = s.conversa_id`

func scanSessao(sc interface{ Scan(...any) error }) (dominio.SessaoChatbot, error) {
	var s dominio.SessaoChatbot
	var def, vars string
	var motivo sql.NullString
	var exp, ini, atu int64
	var fim sql.NullInt64
	if err := sc.Scan(&s.ID, &s.AutomacaoID, &s.AutomacaoNome, &s.ConversaID, &s.Versao, &def, &s.NoAtual, &vars, &s.Tentativas,
		&s.Estado, &motivo, &exp, &ini, &atu, &fim, &s.ContaID); err != nil {
		return s, err
	}
	s.Definicao = json.RawMessage(def)
	s.Variaveis = map[string]string{}
	json.Unmarshal([]byte(vars), &s.Variaveis)
	s.Motivo = deNullStr(motivo)
	s.ExpiraEm, s.IniciadaEm, s.AtualizadaEm, s.FinalizadaEm = DeMs(exp), DeMs(ini), DeMs(atu), deNullMs(fim)
	return s, nil
}

// InserirSessao cria uma sessão ativa (erro de conflito se a conversa já tem uma ativa).
func InserirSessao(ctx context.Context, ex Executor, s dominio.SessaoChatbot) error {
	vars := s.Variaveis
	if vars == nil {
		vars = map[string]string{}
	}
	_, err := ex.ExecContext(ctx, `INSERT INTO sessoes_chatbot (id, automacao_id, conversa_id, versao, definicao, no_atual, variaveis,
		tentativas, estado, motivo, expira_em, iniciada_em, atualizada_em, finalizada_em) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.AutomacaoID, s.ConversaID, s.Versao, string(s.Definicao), s.NoAtual, jsonTexto(vars), s.Tentativas, s.Estado,
		strOuNil(s.Motivo), Ms(s.ExpiraEm), Ms(s.IniciadaEm), Ms(s.AtualizadaEm), MsPtr(s.FinalizadaEm))
	return traduzirUnico(err, "Esta conversa já tem um chatbot ativo.")
}

// GravarSessao atualiza os campos mutáveis.
func GravarSessao(ctx context.Context, ex Executor, s dominio.SessaoChatbot) error {
	vars := s.Variaveis
	if vars == nil {
		vars = map[string]string{}
	}
	_, err := ex.ExecContext(ctx, `UPDATE sessoes_chatbot SET no_atual = ?, variaveis = ?, tentativas = ?, estado = ?, motivo = ?,
		expira_em = ?, atualizada_em = ?, finalizada_em = ? WHERE id = ?`,
		s.NoAtual, jsonTexto(vars), s.Tentativas, s.Estado, strOuNil(s.Motivo), Ms(s.ExpiraEm), Ms(s.AtualizadaEm), MsPtr(s.FinalizadaEm), s.ID)
	return err
}

// ObterSessao por id.
func ObterSessao(ctx context.Context, ex Executor, id string) (dominio.SessaoChatbot, error) {
	s, err := scanSessao(ex.QueryRowContext(ctx, `SELECT `+colunasSessao+deSessao+` WHERE s.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return s, erros.NaoAchada("Sessão")
	}
	return s, err
}

// SessaoAtivaDaConversa devolve a sessão ativa (ok=false se não há).
func SessaoAtivaDaConversa(ctx context.Context, ex Executor, conversaID string) (dominio.SessaoChatbot, bool, error) {
	s, err := scanSessao(ex.QueryRowContext(ctx, `SELECT `+colunasSessao+deSessao+` WHERE s.conversa_id = ? AND s.estado = 'ativa'`, conversaID))
	if errors.Is(err, sql.ErrNoRows) {
		return s, false, nil
	}
	return s, err == nil, err
}

// SessoesAtivasDaAutomacao lista as sessões ativas de um bot.
func SessoesAtivasDaAutomacao(ctx context.Context, ex Executor, automacaoID string) ([]dominio.SessaoChatbot, error) {
	linhas, err := ex.QueryContext(ctx, `SELECT `+colunasSessao+deSessao+` WHERE s.automacao_id = ? AND s.estado = 'ativa'`, automacaoID)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	var l []dominio.SessaoChatbot
	for linhas.Next() {
		s, err := scanSessao(linhas)
		if err != nil {
			return nil, err
		}
		l = append(l, s)
	}
	return l, linhas.Err()
}

// ListarSessoes pagina as sessões de um bot (iniciada_em desc).
func ListarSessoes(ctx context.Context, ex Executor, automacaoID, estado string, p pagina.Params) ([]dominio.SessaoChatbot, string, error) {
	onde := []string{"s.automacao_id = ?"}
	args := []any{automacaoID}
	if estado != "" {
		onde = append(onde, "s.estado = ?")
		args = append(args, estado)
	}
	if p.Cursor != nil {
		onde = append(onde, "(s.iniciada_em < ? OR (s.iniciada_em = ? AND s.id < ?))")
		args = append(args, p.Cursor.N, p.Cursor.N, p.Cursor.S)
	}
	args = append(args, p.Limite+1)
	linhas, err := ex.QueryContext(ctx, `SELECT `+colunasSessao+deSessao+` WHERE `+strings.Join(onde, " AND ")+
		` ORDER BY s.iniciada_em DESC, s.id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, "", err
	}
	defer linhas.Close()
	var l []dominio.SessaoChatbot
	for linhas.Next() {
		s, err := scanSessao(linhas)
		if err != nil {
			return nil, "", err
		}
		l = append(l, s)
	}
	if err := linhas.Err(); err != nil {
		return nil, "", err
	}
	prox := ""
	if len(l) > p.Limite {
		l = l[:p.Limite]
		u := l[len(l)-1]
		prox = pagina.Codificar(Ms(u.IniciadaEm), u.ID)
	}
	return l, prox, nil
}

// ---------------------------------------------------------------------------
// Pausas de conversa (data-model.md › Pausa de conversa)
// ---------------------------------------------------------------------------

// ObterPausa devolve a pausa gravada (vencida ou não; ok=false se não há).
func ObterPausa(ctx context.Context, ex Executor, conversaID string) (dominio.Pausa, bool, error) {
	var p dominio.Pausa
	var ate sql.NullInt64
	var aut sql.NullString
	var cri int64
	err := ex.QueryRowContext(ctx, `SELECT conversa_id, motivo, ate, automacao_id, criada_em FROM pausas_conversa WHERE conversa_id = ?`,
		conversaID).Scan(&p.ConversaID, &p.Motivo, &ate, &aut, &cri)
	if errors.Is(err, sql.ErrNoRows) {
		return p, false, nil
	}
	if err != nil {
		return p, false, err
	}
	p.Ate, p.AutomacaoID, p.CriadaEm = deNullMs(ate), deNullStr(aut), DeMs(cri)
	return p, true, nil
}

// GravarPausa insere ou substitui a pausa da conversa.
func GravarPausa(ctx context.Context, ex Executor, p dominio.Pausa) error {
	_, err := ex.ExecContext(ctx, `INSERT INTO pausas_conversa (conversa_id, motivo, ate, automacao_id, criada_em) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (conversa_id) DO UPDATE SET motivo = excluded.motivo, ate = excluded.ate, automacao_id = excluded.automacao_id,
		criada_em = excluded.criada_em`, p.ConversaID, p.Motivo, MsPtr(p.Ate), strOuNil(p.AutomacaoID), Ms(p.CriadaEm))
	return err
}

// ApagarPausa remove a pausa (true se existia).
func ApagarPausa(ctx context.Context, ex Executor, conversaID string) (bool, error) {
	r, err := ex.ExecContext(ctx, `DELETE FROM pausas_conversa WHERE conversa_id = ?`, conversaID)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}

// ListarPausas devolve as pausas ainda válidas em `agora` (opcionalmente de um motivo).
func ListarPausas(ctx context.Context, ex Executor, motivo string, agora time.Time) ([]dominio.Pausa, error) {
	q := `SELECT conversa_id, motivo, ate, automacao_id, criada_em FROM pausas_conversa WHERE (ate IS NULL OR ate > ?)`
	args := []any{Ms(agora)}
	if motivo != "" {
		q += ` AND motivo = ?`
		args = append(args, motivo)
	}
	linhas, err := ex.QueryContext(ctx, q+` ORDER BY criada_em DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	l := []dominio.Pausa{}
	for linhas.Next() {
		var p dominio.Pausa
		var ate sql.NullInt64
		var aut sql.NullString
		var cri int64
		if err := linhas.Scan(&p.ConversaID, &p.Motivo, &ate, &aut, &cri); err != nil {
			return nil, err
		}
		p.Ate, p.AutomacaoID, p.CriadaEm = deNullMs(ate), deNullStr(aut), DeMs(cri)
		l = append(l, p)
	}
	return l, linhas.Err()
}

// PausasVencidas devolve as conversas com pausa vencida até `agora` (o agendador publica o fim).
func PausasVencidas(ctx context.Context, ex Executor, agora time.Time) ([]string, error) {
	linhas, err := ex.QueryContext(ctx, `SELECT conversa_id FROM pausas_conversa WHERE ate IS NOT NULL AND ate <= ?`, Ms(agora))
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

// ProximoVencimentoPausa devolve o menor `ate` de pausas com prazo.
func ProximoVencimentoPausa(ctx context.Context, ex Executor) (time.Time, bool, error) {
	var ms sql.NullInt64
	if err := ex.QueryRowContext(ctx, `SELECT MIN(ate) FROM pausas_conversa WHERE ate IS NOT NULL`).Scan(&ms); err != nil {
		return time.Time{}, false, err
	}
	if !ms.Valid {
		return time.Time{}, false, nil
	}
	return DeMs(ms.Int64), true, nil
}

// ---------------------------------------------------------------------------
// Memória das automações (data-model.md › Memória)
// ---------------------------------------------------------------------------

// LimiteChavesMemoria por automação.
const LimiteChavesMemoria = 10000

// LerMemoria devolve o valor JSON (ok=false se ausente).
func LerMemoria(ctx context.Context, ex Executor, automacaoID, escopo, chave string) (json.RawMessage, bool, error) {
	var v string
	err := ex.QueryRowContext(ctx, `SELECT valor FROM memoria_automacoes WHERE automacao_id = ? AND escopo = ? AND chave = ?`,
		automacaoID, escopo, chave).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	return json.RawMessage(v), err == nil, err
}

// GravarMemoria insere/atualiza; falha com `validacao` ao passar de 10.000 chaves novas.
func GravarMemoria(ctx context.Context, ex Executor, automacaoID, escopo, chave string, valor json.RawMessage, agora time.Time) error {
	if _, ok, err := LerMemoria(ctx, ex, automacaoID, escopo, chave); err != nil {
		return err
	} else if !ok {
		var n int
		if err := ex.QueryRowContext(ctx, `SELECT count(*) FROM memoria_automacoes WHERE automacao_id = ?`, automacaoID).Scan(&n); err != nil {
			return err
		}
		if n >= LimiteChavesMemoria {
			return erros.Novo(erros.Validacao, "Limite de 10.000 chaves de memória atingido.")
		}
	}
	_, err := ex.ExecContext(ctx, `INSERT INTO memoria_automacoes (automacao_id, escopo, chave, valor, atualizada_em) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (automacao_id, escopo, chave) DO UPDATE SET valor = excluded.valor, atualizada_em = excluded.atualizada_em`,
		automacaoID, escopo, chave, string(valor), Ms(agora))
	return err
}

// RemoverMemoria apaga a chave (true se existia).
func RemoverMemoria(ctx context.Context, ex Executor, automacaoID, escopo, chave string) (bool, error) {
	r, err := ex.ExecContext(ctx, `DELETE FROM memoria_automacoes WHERE automacao_id = ? AND escopo = ? AND chave = ?`, automacaoID, escopo, chave)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}

// ItemMemoria de uma listagem.
type ItemMemoria struct {
	Chave string          `json:"chave"`
	Valor json.RawMessage `json:"valor"`
}

// ListarMemoria lista as chaves de um escopo com o prefixo (ordem de chave; máx. 1.000).
func ListarMemoria(ctx context.Context, ex Executor, automacaoID, escopo, prefixo string) ([]ItemMemoria, error) {
	q := `SELECT chave, valor FROM memoria_automacoes WHERE automacao_id = ? AND escopo = ?`
	args := []any{automacaoID, escopo}
	if prefixo != "" {
		q += ` AND substr(chave, 1, ?) = ?`
		args = append(args, len([]rune(prefixo)), prefixo)
	}
	linhas, err := ex.QueryContext(ctx, q+` ORDER BY chave LIMIT 1000`, args...)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	l := []ItemMemoria{}
	for linhas.Next() {
		var it ItemMemoria
		var v string
		if err := linhas.Scan(&it.Chave, &v); err != nil {
			return nil, err
		}
		it.Valor = json.RawMessage(v)
		l = append(l, it)
	}
	return l, linhas.Err()
}
