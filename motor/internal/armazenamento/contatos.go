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

// DadosContato são os campos atualizáveis por eventos do WhatsApp.
type DadosContato struct {
	JID      string
	Telefone string
	Nome     string // agenda
	NomePush string
}

// GarantirContato cria ou atualiza (sem apagar nomes já conhecidos) o contato (conta, jid).
// Liga ao lead de mesmo telefone. Devolve id e se houve mudança.
func GarantirContato(ctx context.Context, ex Executor, contaID string, d DadosContato, agora time.Time) (string, bool, error) {
	var id string
	var tel, nome, push, lead sql.NullString
	err := ex.QueryRowContext(ctx, `SELECT id, telefone, nome, nome_push, lead_id FROM contatos WHERE conta_id = ? AND jid = ?`,
		contaID, d.JID).Scan(&id, &tel, &nome, &push, &lead)
	if errors.Is(err, sql.ErrNoRows) {
		id = ids.NovoEm(agora)
		_, err = ex.ExecContext(ctx, `INSERT INTO contatos (id, conta_id, jid, telefone, nome, nome_push, lead_id, criado_em, atualizado_em)
			VALUES (?, ?, ?, ?, ?, ?, (SELECT id FROM leads WHERE telefone = ?), ?, ?)`,
			id, contaID, d.JID, vazioNil(d.Telefone), vazioNil(d.Nome), vazioNil(d.NomePush), d.Telefone, Ms(agora), Ms(agora))
		return id, true, err
	}
	if err != nil {
		return "", false, err
	}
	mudou := false
	novoTel, novoNome, novoPush := tel.String, nome.String, push.String
	if d.Telefone != "" && d.Telefone != tel.String {
		novoTel, mudou = d.Telefone, true
	}
	if d.Nome != "" && d.Nome != nome.String {
		novoNome, mudou = d.Nome, true
	}
	if d.NomePush != "" && d.NomePush != push.String {
		novoPush, mudou = d.NomePush, true
	}
	if !lead.Valid && novoTel != "" {
		var leadID string
		if ex.QueryRowContext(ctx, `SELECT id FROM leads WHERE telefone = ?`, novoTel).Scan(&leadID) == nil {
			mudou = true
		}
	}
	if !mudou {
		return id, false, nil
	}
	_, err = ex.ExecContext(ctx, `UPDATE contatos SET telefone = ?, nome = ?, nome_push = ?,
		lead_id = COALESCE(lead_id, (SELECT id FROM leads WHERE telefone = ?)), atualizado_em = ? WHERE id = ?`,
		vazioNil(novoTel), vazioNil(novoNome), vazioNil(novoPush), novoTel, Ms(agora), id)
	return id, true, err
}

const selectContato = `SELECT ct.id, ct.conta_id, ct.jid, ct.telefone, ct.nome, ct.nome_push, ct.notas,
	l.id, l.origem, l.importado_em,
	(SELECT cv.id FROM conversas cv WHERE cv.conta_id = ct.conta_id AND cv.jid = ct.jid)
	FROM contatos ct LEFT JOIN leads l ON l.id = ct.lead_id`

func scanContato(sc interface{ Scan(...any) error }) (dominio.Contato, int64, error) {
	var c dominio.Contato
	var tel, nome, push, notas, lid, lorigem, conv sql.NullString
	var limp sql.NullInt64
	if err := sc.Scan(&c.ID, &c.ContaID, &c.JID, &tel, &nome, &push, &notas, &lid, &lorigem, &limp, &conv); err != nil {
		return c, 0, err
	}
	c.Telefone, c.Nome, c.NomePush, c.Notas, c.ConversaID = deNullStr(tel), deNullStr(nome), deNullStr(push), deNullStr(notas), deNullStr(conv)
	if lid.Valid {
		c.Lead = &dominio.LeadResumo{ID: lid.String, Origem: lorigem.String, ImportadoEm: DeMs(limp.Int64)}
	}
	c.Etiquetas = []dominio.Etiqueta{}
	return c, 0, nil
}

// ObterContato busca por id com etiquetas.
func ObterContato(ctx context.Context, ex Executor, id string) (dominio.Contato, error) {
	c, _, err := scanContato(ex.QueryRowContext(ctx, selectContato+` WHERE ct.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return c, erros.NaoAchado("Contato")
	}
	if err != nil {
		return c, err
	}
	et, err := EtiquetasDeContatos(ctx, ex, []string{c.ID})
	if err != nil {
		return c, err
	}
	if l, ok := et[c.ID]; ok {
		c.Etiquetas = l
	}
	return c, nil
}

// ContatoPorJID busca (conta, jid).
func ContatoPorJID(ctx context.Context, ex Executor, contaID, jid string) (dominio.Contato, error) {
	var id string
	err := ex.QueryRowContext(ctx, `SELECT id FROM contatos WHERE conta_id = ? AND jid = ?`, contaID, jid).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return dominio.Contato{}, erros.NaoAchado("Contato")
	}
	if err != nil {
		return dominio.Contato{}, err
	}
	return ObterContato(ctx, ex, id)
}

// FiltroContatos da listagem.
type FiltroContatos struct {
	Busca      string
	EtiquetaID string
}

// ListarContatos pagina por (nome exibido, id).
func ListarContatos(ctx context.Context, ex Executor, contaID string, f FiltroContatos, p pagina.Params) ([]dominio.Contato, string, error) {
	onde := []string{"ct.conta_id = ?"}
	args := []any{contaID}
	if f.Busca != "" {
		onde = append(onde, `(ct.nome LIKE ? ESCAPE '\' OR ct.nome_push LIKE ? ESCAPE '\' OR ct.telefone LIKE ? ESCAPE '\')`)
		l := escaparLike(f.Busca)
		args = append(args, l, l, l)
	}
	if f.EtiquetaID != "" {
		onde = append(onde, `EXISTS (SELECT 1 FROM contato_etiquetas ce WHERE ce.contato_id = ct.id AND ce.etiqueta_id = ?)`)
		args = append(args, f.EtiquetaID)
	}
	if p.Cursor != nil {
		onde = append(onde, `ct.id > ?`)
		args = append(args, p.Cursor.S)
	}
	q := selectContato + ` WHERE ` + strings.Join(onde, " AND ") + ` ORDER BY ct.id LIMIT ?`
	args = append(args, p.Limite+1)
	linhas, err := ex.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, "", err
	}
	defer linhas.Close()
	var lista []dominio.Contato
	for linhas.Next() {
		c, _, err := scanContato(linhas)
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
		proximo = pagina.Codificar(0, lista[len(lista)-1].ID)
	}
	if err := preencherEtiquetasContatos(ctx, ex, lista); err != nil {
		return nil, "", err
	}
	return lista, proximo, nil
}

func preencherEtiquetasContatos(ctx context.Context, ex Executor, lista []dominio.Contato) error {
	if len(lista) == 0 {
		return nil
	}
	idsC := make([]string, len(lista))
	for i, c := range lista {
		idsC[i] = c.ID
	}
	et, err := EtiquetasDeContatos(ctx, ex, idsC)
	if err != nil {
		return err
	}
	for i := range lista {
		if l, ok := et[lista[i].ID]; ok {
			lista[i].Etiquetas = l
		}
	}
	return nil
}

// EtiquetasDeContatos devolve contato_id → etiquetas (com total_contatos).
func EtiquetasDeContatos(ctx context.Context, ex Executor, contatoIDs []string) (map[string][]dominio.Etiqueta, error) {
	res := map[string][]dominio.Etiqueta{}
	if len(contatoIDs) == 0 {
		return res, nil
	}
	m, args := marcadores(contatoIDs)
	linhas, err := ex.QueryContext(ctx, `SELECT ce.contato_id, e.id, e.nome, e.cor,
		(SELECT count(*) FROM contato_etiquetas x WHERE x.etiqueta_id = e.id)
		FROM contato_etiquetas ce JOIN etiquetas e ON e.id = ce.etiqueta_id
		WHERE ce.contato_id IN (`+m+`) ORDER BY e.nome COLLATE NOCASE`, args...)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	for linhas.Next() {
		var cid string
		var e dominio.Etiqueta
		if err := linhas.Scan(&cid, &e.ID, &e.Nome, &e.Cor, &e.TotalContatos); err != nil {
			return nil, err
		}
		res[cid] = append(res[cid], e)
	}
	return res, linhas.Err()
}
