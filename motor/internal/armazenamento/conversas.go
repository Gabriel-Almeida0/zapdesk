package armazenamento

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/pagina"
)

// GarantirConversa cria a conversa (conta, jid) se não existir; atualiza nome de grupo e
// contato. Devolve id e se foi criada.
func GarantirConversa(ctx context.Context, ex Executor, contaID, jid, tipo, nome, contatoID string, agora time.Time) (string, bool, error) {
	var id string
	var nomeAtual, contatoAtual sql.NullString
	err := ex.QueryRowContext(ctx, `SELECT id, nome, contato_id FROM conversas WHERE conta_id = ? AND jid = ?`, contaID, jid).
		Scan(&id, &nomeAtual, &contatoAtual)
	if errors.Is(err, sql.ErrNoRows) {
		id = ids.NovoEm(agora)
		_, err = ex.ExecContext(ctx, `INSERT INTO conversas (id, conta_id, jid, tipo, nome, contato_id) VALUES (?, ?, ?, ?, ?, ?)`,
			id, contaID, jid, tipo, vazioNil(nome), vazioNil(contatoID))
		return id, true, err
	}
	if err != nil {
		return "", false, err
	}
	if (nome != "" && nome != nomeAtual.String) || (contatoID != "" && contatoID != contatoAtual.String) {
		_, err = ex.ExecContext(ctx, `UPDATE conversas SET nome = COALESCE(?, nome), contato_id = COALESCE(?, contato_id) WHERE id = ?`,
			vazioNil(nome), vazioNil(contatoID), id)
	}
	return id, false, err
}

// AtualizarUltimaMensagem avança a última mensagem (só se for mais nova) e soma não lidas.
func AtualizarUltimaMensagem(ctx context.Context, ex Executor, conversaID string, em time.Time, resumo string, somarNaoLidas int) error {
	if len([]rune(resumo)) > 200 {
		resumo = string([]rune(resumo)[:200])
	}
	_, err := ex.ExecContext(ctx, `UPDATE conversas SET
		ultima_mensagem_resumo = CASE WHEN ultima_mensagem_em IS NULL OR ultima_mensagem_em <= ? THEN ? ELSE ultima_mensagem_resumo END,
		ultima_mensagem_em = CASE WHEN ultima_mensagem_em IS NULL OR ultima_mensagem_em <= ? THEN ? ELSE ultima_mensagem_em END,
		nao_lidas = nao_lidas + ?
		WHERE id = ?`, Ms(em), resumo, Ms(em), Ms(em), somarNaoLidas, conversaID)
	return err
}

// ResumirSeUltima troca o resumo da conversa quando a mensagem `mensagemID` (editada/apagada) é
// a mais recente dela. Devolve true se mudou. Compara com a tabela de mensagens, e não com
// `ultima_mensagem_em`, porque o horário gravado na conversa pode diferir em milissegundos do
// `enviada_em` confirmado depois pelo WhatsApp.
func ResumirSeUltima(ctx context.Context, ex Executor, mensagemID, resumo string) (bool, error) {
	if len([]rune(resumo)) > 200 {
		resumo = string([]rune(resumo)[:200])
	}
	r, err := ex.ExecContext(ctx, `UPDATE conversas SET ultima_mensagem_resumo = ?1
		WHERE id = (SELECT m.conversa_id FROM mensagens m WHERE m.id = ?2)
		AND NOT EXISTS (
			SELECT 1 FROM mensagens m, mensagens a
			WHERE a.id = ?2 AND m.conversa_id = a.conversa_id
			AND (m.enviada_em > a.enviada_em OR (m.enviada_em = a.enviada_em AND m.rid > a.rid)))`,
		resumo, mensagemID)
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	return n > 0, err
}

// DefinirNaoLidas ajusta o contador.
func DefinirNaoLidas(ctx context.Context, ex Executor, conversaID string, n int) error {
	_, err := ex.ExecContext(ctx, `UPDATE conversas SET nao_lidas = ? WHERE id = ?`, n, conversaID)
	return err
}

const selectConversa = `SELECT cv.id, cv.conta_id, cv.jid, cv.tipo,
	CASE WHEN cv.tipo = 'grupo' THEN COALESCE(NULLIF(cv.nome, ''), cv.jid)
	     ELSE COALESCE(NULLIF(ct.nome, ''), NULLIF(ct.nome_push, ''), NULLIF(cv.nome, ''), ct.telefone, cv.jid) END,
	ct.telefone, cv.contato_id, cv.nao_lidas, cv.ultima_mensagem_em, cv.ultima_mensagem_resumo
	FROM conversas cv LEFT JOIN contatos ct ON ct.id = cv.contato_id`

func scanConversa(sc interface{ Scan(...any) error }) (dominio.Conversa, error) {
	var c dominio.Conversa
	var tel, contato, resumo sql.NullString
	var ult sql.NullInt64
	if err := sc.Scan(&c.ID, &c.ContaID, &c.JID, &c.Tipo, &c.Nome, &tel, &contato, &c.NaoLidas, &ult, &resumo); err != nil {
		return c, err
	}
	c.Telefone, c.ContatoID, c.UltimaMensagemEm, c.UltimaMensagemResumo = deNullStr(tel), deNullStr(contato), deNullMs(ult), deNullStr(resumo)
	c.Etiquetas = []dominio.Etiqueta{}
	return c, nil
}

// ObterConversa busca por id com etiquetas.
func ObterConversa(ctx context.Context, ex Executor, id string) (dominio.Conversa, error) {
	c, err := scanConversa(ex.QueryRowContext(ctx, selectConversa+` WHERE cv.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return c, erros.NaoAchada("Conversa")
	}
	if err != nil {
		return c, err
	}
	lista := []dominio.Conversa{c}
	if err := preencherEtiquetasConversas(ctx, ex, lista); err != nil {
		return c, err
	}
	return lista[0], nil
}

// ConversaPorJID busca (conta, jid); ok=false se não existe.
func ConversaPorJID(ctx context.Context, ex Executor, contaID, jid string) (dominio.Conversa, bool, error) {
	var id string
	err := ex.QueryRowContext(ctx, `SELECT id FROM conversas WHERE conta_id = ? AND jid = ?`, contaID, jid).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return dominio.Conversa{}, false, nil
	}
	if err != nil {
		return dominio.Conversa{}, false, err
	}
	c, err := ObterConversa(ctx, ex, id)
	return c, err == nil, err
}

// FiltroConversas da listagem.
type FiltroConversas struct {
	Busca      string
	EtiquetaID string
	NaoLidas   bool
}

// ListarConversas pagina por (ultima_mensagem_em desc, id desc).
func ListarConversas(ctx context.Context, ex Executor, contaID string, f FiltroConversas, p pagina.Params) ([]dominio.Conversa, string, error) {
	onde := []string{"cv.conta_id = ?"}
	args := []any{contaID}
	if f.Busca != "" {
		onde = append(onde, `(cv.nome LIKE ? ESCAPE '\' OR ct.nome LIKE ? ESCAPE '\' OR ct.nome_push LIKE ? ESCAPE '\' OR ct.telefone LIKE ? ESCAPE '\')`)
		l := escaparLike(f.Busca)
		args = append(args, l, l, l, l)
	}
	if f.NaoLidas {
		onde = append(onde, "cv.nao_lidas > 0")
	}
	if f.EtiquetaID != "" {
		onde = append(onde, `EXISTS (SELECT 1 FROM contato_etiquetas ce WHERE ce.contato_id = cv.contato_id AND ce.etiqueta_id = ?)`)
		args = append(args, f.EtiquetaID)
	}
	if p.Cursor != nil {
		onde = append(onde, `(COALESCE(cv.ultima_mensagem_em, -1) < ? OR (COALESCE(cv.ultima_mensagem_em, -1) = ? AND cv.id < ?))`)
		args = append(args, p.Cursor.N, p.Cursor.N, p.Cursor.S)
	}
	q := selectConversa + ` WHERE ` + strings.Join(onde, " AND ") +
		` ORDER BY COALESCE(cv.ultima_mensagem_em, -1) DESC, cv.id DESC LIMIT ?`
	args = append(args, p.Limite+1)
	linhas, err := ex.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, "", err
	}
	defer linhas.Close()
	var lista []dominio.Conversa
	for linhas.Next() {
		c, err := scanConversa(linhas)
		if err != nil {
			return nil, "", err
		}
		lista = append(lista, c)
	}
	if err := linhas.Err(); err != nil {
		return nil, "", err
	}
	proximo := ""
	if len(lista) > p.Limite {
		lista = lista[:p.Limite]
		u := lista[len(lista)-1]
		n := int64(-1)
		if u.UltimaMensagemEm != nil {
			n = Ms(*u.UltimaMensagemEm)
		}
		proximo = pagina.Codificar(n, u.ID)
	}
	if err := preencherEtiquetasConversas(ctx, ex, lista); err != nil {
		return nil, "", err
	}
	return lista, proximo, nil
}

func preencherEtiquetasConversas(ctx context.Context, ex Executor, lista []dominio.Conversa) error {
	var idsC []string
	for _, c := range lista {
		if c.ContatoID != nil {
			idsC = append(idsC, *c.ContatoID)
		}
	}
	et, err := EtiquetasDeContatos(ctx, ex, idsC)
	if err != nil {
		return err
	}
	for i := range lista {
		if lista[i].ContatoID != nil {
			if l, ok := et[*lista[i].ContatoID]; ok {
				lista[i].Etiquetas = l
			}
		}
	}
	return nil
}
