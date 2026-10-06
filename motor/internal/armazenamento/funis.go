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

// ---------------------------------------------------------------------------
// Funis e etapas (data-model.md › Funil, Etapa)
// ---------------------------------------------------------------------------

// ListarFunis devolve todos os funis (ordem) com etapas e contagens.
func ListarFunis(ctx context.Context, ex Executor) ([]dominio.Funil, error) {
	linhas, err := ex.QueryContext(ctx, `SELECT id, nome, ordem, criado_em, atualizado_em,
		(SELECT count(*) FROM posicoes_funil p WHERE p.funil_id = f.id) FROM funis f ORDER BY ordem, criado_em`)
	if err != nil {
		return nil, err
	}
	lista := []dominio.Funil{}
	for linhas.Next() {
		var f dominio.Funil
		var cri, atu int64
		if err := linhas.Scan(&f.ID, &f.Nome, &f.Ordem, &cri, &atu, &f.TotalCards); err != nil {
			linhas.Close()
			return nil, err
		}
		f.CriadoEm, f.AtualizadoEm = DeMs(cri), DeMs(atu)
		lista = append(lista, f)
	}
	linhas.Close()
	if err := linhas.Err(); err != nil {
		return nil, err
	}
	for i := range lista {
		if lista[i].Etapas, err = ListarEtapas(ctx, ex, lista[i].ID); err != nil {
			return nil, err
		}
	}
	return lista, nil
}

// ObterFunil por id (com etapas).
func ObterFunil(ctx context.Context, ex Executor, id string) (dominio.Funil, error) {
	var f dominio.Funil
	var cri, atu int64
	err := ex.QueryRowContext(ctx, `SELECT id, nome, ordem, criado_em, atualizado_em,
		(SELECT count(*) FROM posicoes_funil p WHERE p.funil_id = f.id) FROM funis f WHERE id = ?`, id).
		Scan(&f.ID, &f.Nome, &f.Ordem, &cri, &atu, &f.TotalCards)
	if errors.Is(err, sql.ErrNoRows) {
		return f, erros.NaoAchado("Funil")
	}
	if err != nil {
		return f, err
	}
	f.CriadoEm, f.AtualizadoEm = DeMs(cri), DeMs(atu)
	f.Etapas, err = ListarEtapas(ctx, ex, id)
	return f, err
}

// FunilPorNome busca sem diferenciar maiúsculas (usado por ctx.funil e pelo manifesto).
func FunilPorNome(ctx context.Context, ex Executor, nome string) (dominio.Funil, error) {
	var id string
	err := ex.QueryRowContext(ctx, `SELECT id FROM funis WHERE nome = ? COLLATE NOCASE`, nome).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return dominio.Funil{}, erros.NaoAchado("Funil")
	}
	if err != nil {
		return dominio.Funil{}, err
	}
	return ObterFunil(ctx, ex, id)
}

// ProximaOrdemFunil devolve MAX(ordem)+1.
func ProximaOrdemFunil(ctx context.Context, ex Executor) (int, error) {
	var n int
	err := ex.QueryRowContext(ctx, `SELECT COALESCE(MAX(ordem) + 1, 0) FROM funis`).Scan(&n)
	return n, err
}

// InserirFunil cria (conflito se o nome existe).
func InserirFunil(ctx context.Context, ex Executor, id, nome string, ordem int, agora time.Time) error {
	_, err := ex.ExecContext(ctx, `INSERT INTO funis (id, nome, ordem, criado_em, atualizado_em) VALUES (?, ?, ?, ?, ?)`,
		id, nome, ordem, Ms(agora), Ms(agora))
	return traduzirUnico(err, "Já existe um funil com esse nome.")
}

// AtualizarFunil grava nome e ordem.
func AtualizarFunil(ctx context.Context, ex Executor, id, nome string, ordem int, agora time.Time) error {
	_, err := ex.ExecContext(ctx, `UPDATE funis SET nome = ?, ordem = ?, atualizado_em = ? WHERE id = ?`, nome, ordem, Ms(agora), id)
	return traduzirUnico(err, "Já existe um funil com esse nome.")
}

// TocarFunil atualiza atualizado_em.
func TocarFunil(ctx context.Context, ex Executor, id string, agora time.Time) error {
	_, err := ex.ExecContext(ctx, `UPDATE funis SET atualizado_em = ? WHERE id = ?`, Ms(agora), id)
	return err
}

// ExcluirFunil apaga (cascata: etapas, posições, histórico).
func ExcluirFunil(ctx context.Context, ex Executor, id string) (bool, error) {
	r, err := ex.ExecContext(ctx, `DELETE FROM funis WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}

const selectEtapa = `SELECT e.id, e.funil_id, e.nome, e.cor, e.ordem,
	(SELECT count(*) FROM posicoes_funil p WHERE p.etapa_id = e.id) FROM etapas e`

func scanEtapa(sc interface{ Scan(...any) error }) (dominio.Etapa, error) {
	var e dominio.Etapa
	err := sc.Scan(&e.ID, &e.FunilID, &e.Nome, &e.Cor, &e.Ordem, &e.TotalCards)
	return e, err
}

// ListarEtapas de um funil na ordem.
func ListarEtapas(ctx context.Context, ex Executor, funilID string) ([]dominio.Etapa, error) {
	linhas, err := ex.QueryContext(ctx, selectEtapa+` WHERE e.funil_id = ? ORDER BY e.ordem, e.criada_em`, funilID)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	lista := []dominio.Etapa{}
	for linhas.Next() {
		e, err := scanEtapa(linhas)
		if err != nil {
			return nil, err
		}
		lista = append(lista, e)
	}
	return lista, linhas.Err()
}

// ObterEtapa por id.
func ObterEtapa(ctx context.Context, ex Executor, id string) (dominio.Etapa, error) {
	e, err := scanEtapa(ex.QueryRowContext(ctx, selectEtapa+` WHERE e.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return e, erros.NaoAchada("Etapa")
	}
	return e, err
}

// EtapaPorNome busca a etapa de um funil sem diferenciar maiúsculas.
func EtapaPorNome(ctx context.Context, ex Executor, funilID, nome string) (dominio.Etapa, error) {
	e, err := scanEtapa(ex.QueryRowContext(ctx, selectEtapa+` WHERE e.funil_id = ? AND e.nome = ? COLLATE NOCASE`, funilID, nome))
	if errors.Is(err, sql.ErrNoRows) {
		return e, erros.NaoAchada("Etapa")
	}
	return e, err
}

// InserirEtapa cria a etapa na ordem dada (as seguintes são deslocadas).
func InserirEtapa(ctx context.Context, ex Executor, e dominio.Etapa, agora time.Time) error {
	if _, err := ex.ExecContext(ctx, `UPDATE etapas SET ordem = ordem + 1 WHERE funil_id = ? AND ordem >= ?`, e.FunilID, e.Ordem); err != nil {
		return err
	}
	_, err := ex.ExecContext(ctx, `INSERT INTO etapas (id, funil_id, nome, cor, ordem, criada_em) VALUES (?, ?, ?, ?, ?, ?)`,
		e.ID, e.FunilID, e.Nome, e.Cor, e.Ordem, Ms(agora))
	return traduzirUnico(err, "Já existe uma etapa com esse nome neste funil.")
}

// AtualizarEtapa grava nome e cor.
func AtualizarEtapa(ctx context.Context, ex Executor, e dominio.Etapa) error {
	_, err := ex.ExecContext(ctx, `UPDATE etapas SET nome = ?, cor = ? WHERE id = ?`, e.Nome, e.Cor, e.ID)
	return traduzirUnico(err, "Já existe uma etapa com esse nome neste funil.")
}

// DefinirOrdemEtapa grava a ordem.
func DefinirOrdemEtapa(ctx context.Context, ex Executor, id string, ordem int) error {
	_, err := ex.ExecContext(ctx, `UPDATE etapas SET ordem = ? WHERE id = ?`, ordem, id)
	return err
}

// ExcluirEtapa apaga a etapa (posições restantes em cascata) e compacta a ordem.
func ExcluirEtapa(ctx context.Context, ex Executor, e dominio.Etapa) error {
	if _, err := ex.ExecContext(ctx, `DELETE FROM etapas WHERE id = ?`, e.ID); err != nil {
		return err
	}
	_, err := ex.ExecContext(ctx, `UPDATE etapas SET ordem = ordem - 1 WHERE funil_id = ? AND ordem > ?`, e.FunilID, e.Ordem)
	return err
}

// ---------------------------------------------------------------------------
// Posições (cards) e histórico
// ---------------------------------------------------------------------------

// Posicao de um lead num funil.
type Posicao struct {
	LeadID, FunilID, EtapaID string
	Desde                    time.Time
}

// ObterPosicao devolve a posição (ok=false se o lead não está no funil).
func ObterPosicao(ctx context.Context, ex Executor, leadID, funilID string) (Posicao, bool, error) {
	var p Posicao
	var desde int64
	err := ex.QueryRowContext(ctx, `SELECT lead_id, funil_id, etapa_id, desde FROM posicoes_funil WHERE lead_id = ? AND funil_id = ?`,
		leadID, funilID).Scan(&p.LeadID, &p.FunilID, &p.EtapaID, &desde)
	if errors.Is(err, sql.ErrNoRows) {
		return p, false, nil
	}
	p.Desde = DeMs(desde)
	return p, err == nil, err
}

// DefinirPosicao grava (insere ou move) a posição.
func DefinirPosicao(ctx context.Context, ex Executor, p Posicao) error {
	_, err := ex.ExecContext(ctx, `INSERT INTO posicoes_funil (lead_id, funil_id, etapa_id, desde) VALUES (?, ?, ?, ?)
		ON CONFLICT (lead_id, funil_id) DO UPDATE SET etapa_id = excluded.etapa_id, desde = excluded.desde`,
		p.LeadID, p.FunilID, p.EtapaID, Ms(p.Desde))
	return err
}

// RemoverPosicao tira o lead do funil (true se estava).
func RemoverPosicao(ctx context.Context, ex Executor, leadID, funilID string) (bool, error) {
	r, err := ex.ExecContext(ctx, `DELETE FROM posicoes_funil WHERE lead_id = ? AND funil_id = ?`, leadID, funilID)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}

// LeadsNaEtapa lista os leads de uma etapa.
func LeadsNaEtapa(ctx context.Context, ex Executor, etapaID string) ([]string, error) {
	linhas, err := ex.QueryContext(ctx, `SELECT lead_id FROM posicoes_funil WHERE etapa_id = ?`, etapaID)
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

// InserirMovimento grava uma linha do histórico.
func InserirMovimento(ctx context.Context, ex Executor, m dominio.MovimentoFunil) error {
	_, err := ex.ExecContext(ctx, `INSERT INTO historico_funil (id, lead_id, funil_id, etapa_origem_id, etapa_origem_nome,
		etapa_destino_id, etapa_destino_nome, origem, automacao_id, execucao_id, em) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.LeadID, m.FunilID, strOuNil(m.EtapaOrigemID), strOuNil(m.EtapaOrigemNome), strOuNil(m.EtapaDestinoID),
		strOuNil(m.EtapaDestinoNome), m.Origem, strOuNil(m.AutomacaoID), strOuNil(m.ExecucaoID), Ms(m.Em))
	return err
}

// ListarHistoricoFunil pagina o histórico (mais recente primeiro; id ULID é ordenável).
func ListarHistoricoFunil(ctx context.Context, ex Executor, funilID, leadID string, p pagina.Params) ([]dominio.MovimentoFunil, string, error) {
	onde := []string{"funil_id = ?"}
	args := []any{funilID}
	if leadID != "" {
		onde = append(onde, "lead_id = ?")
		args = append(args, leadID)
	}
	if p.Cursor != nil {
		onde = append(onde, "id < ?")
		args = append(args, p.Cursor.S)
	}
	args = append(args, p.Limite+1)
	linhas, err := ex.QueryContext(ctx, `SELECT id, lead_id, funil_id, etapa_origem_id, etapa_origem_nome, etapa_destino_id,
		etapa_destino_nome, origem, automacao_id, execucao_id, em FROM historico_funil WHERE `+strings.Join(onde, " AND ")+
		` ORDER BY id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, "", err
	}
	defer linhas.Close()
	var lista []dominio.MovimentoFunil
	for linhas.Next() {
		var m dominio.MovimentoFunil
		var oi, on, di, dn, ai, ei sql.NullString
		var em int64
		if err := linhas.Scan(&m.ID, &m.LeadID, &m.FunilID, &oi, &on, &di, &dn, &m.Origem, &ai, &ei, &em); err != nil {
			return nil, "", err
		}
		m.EtapaOrigemID, m.EtapaOrigemNome, m.EtapaDestinoID, m.EtapaDestinoNome = deNullStr(oi), deNullStr(on), deNullStr(di), deNullStr(dn)
		m.AutomacaoID, m.ExecucaoID, m.Em = deNullStr(ai), deNullStr(ei), DeMs(em)
		lista = append(lista, m)
	}
	if err := linhas.Err(); err != nil {
		return nil, "", err
	}
	prox := ""
	if len(lista) > p.Limite {
		lista = lista[:p.Limite]
		prox = pagina.Codificar(0, lista[len(lista)-1].ID)
	}
	return lista, prox, nil
}

// selectCard junta lead, contato mais recente do telefone (etiquetas) e conversa mais recente.
const selectCard = `SELECT p.lead_id, p.funil_id, p.etapa_id, p.desde, l.telefone, l.nome, l.campos,
	(SELECT ct.id FROM contatos ct WHERE ct.telefone = l.telefone ORDER BY ct.atualizado_em DESC LIMIT 1),
	(SELECT cv.id FROM conversas cv JOIN contatos ct ON ct.id = cv.contato_id WHERE ct.telefone = l.telefone
	   ORDER BY COALESCE(cv.ultima_mensagem_em, 0) DESC LIMIT 1),
	(SELECT cv.conta_id FROM conversas cv JOIN contatos ct ON ct.id = cv.contato_id WHERE ct.telefone = l.telefone
	   ORDER BY COALESCE(cv.ultima_mensagem_em, 0) DESC LIMIT 1)
	FROM posicoes_funil p JOIN leads l ON l.id = p.lead_id`

func scanCards(ctx context.Context, ex Executor, linhas *sql.Rows) ([]dominio.Card, error) {
	defer linhas.Close()
	lista := []dominio.Card{}
	var contatos []string
	var idxContato []int
	for linhas.Next() {
		var c dominio.Card
		var desde int64
		var nome, contato, conversa, conta sql.NullString
		var campos string
		if err := linhas.Scan(&c.LeadID, &c.FunilID, &c.EtapaID, &desde, &c.Lead.Telefone, &nome, &campos, &contato, &conversa, &conta); err != nil {
			return nil, err
		}
		c.Desde = DeMs(desde)
		c.Lead.ID, c.Lead.Nome = c.LeadID, deNullStr(nome)
		c.Lead.Campos = map[string]string{}
		json.Unmarshal([]byte(campos), &c.Lead.Campos)
		c.ConversaID, c.ContaID = deNullStr(conversa), deNullStr(conta)
		c.Etiquetas = []dominio.Etiqueta{}
		if contato.Valid {
			contatos = append(contatos, contato.String)
			idxContato = append(idxContato, len(lista))
		}
		lista = append(lista, c)
	}
	if err := linhas.Err(); err != nil {
		return nil, err
	}
	if len(contatos) > 0 {
		porContato, err := EtiquetasDeContatos(ctx, ex, contatos)
		if err != nil {
			return nil, err
		}
		for i, ct := range contatos {
			if e := porContato[ct]; e != nil {
				lista[idxContato[i]].Etiquetas = e
			}
		}
	}
	return lista, nil
}

// ObterCard devolve o card do lead no funil.
func ObterCard(ctx context.Context, ex Executor, leadID, funilID string) (dominio.Card, bool, error) {
	linhas, err := ex.QueryContext(ctx, selectCard+` WHERE p.lead_id = ? AND p.funil_id = ?`, leadID, funilID)
	if err != nil {
		return dominio.Card{}, false, err
	}
	lista, err := scanCards(ctx, ex, linhas)
	if err != nil || len(lista) == 0 {
		return dominio.Card{}, false, err
	}
	return lista[0], true, nil
}

// FiltroCards da listagem do Kanban.
type FiltroCards struct {
	EtapaID string
	Busca   string
}

// ListarCards pagina os cards de um funil (desde desc, lead_id desc).
func ListarCards(ctx context.Context, ex Executor, funilID string, f FiltroCards, p pagina.Params) ([]dominio.Card, string, error) {
	onde := []string{"p.funil_id = ?"}
	args := []any{funilID}
	if f.EtapaID != "" {
		onde = append(onde, "p.etapa_id = ?")
		args = append(args, f.EtapaID)
	}
	if f.Busca != "" {
		b := escaparLike(f.Busca)
		onde = append(onde, `(l.telefone LIKE ? ESCAPE '\' OR l.nome LIKE ? ESCAPE '\')`)
		args = append(args, b, b)
	}
	if p.Cursor != nil {
		onde = append(onde, "(p.desde < ? OR (p.desde = ? AND p.lead_id < ?))")
		args = append(args, p.Cursor.N, p.Cursor.N, p.Cursor.S)
	}
	args = append(args, p.Limite+1)
	linhas, err := ex.QueryContext(ctx, selectCard+` WHERE `+strings.Join(onde, " AND ")+` ORDER BY p.desde DESC, p.lead_id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, "", err
	}
	lista, err := scanCards(ctx, ex, linhas)
	if err != nil {
		return nil, "", err
	}
	prox := ""
	if len(lista) > p.Limite {
		lista = lista[:p.Limite]
		u := lista[len(lista)-1]
		prox = pagina.Codificar(Ms(u.Desde), u.LeadID)
	}
	return lista, prox, nil
}

// FunisDoLead devolve (funil, card|nil) para todos os funis.
func FunisDoLead(ctx context.Context, ex Executor, leadID string) ([]dominio.Funil, map[string]dominio.Card, error) {
	funis, err := ListarFunis(ctx, ex)
	if err != nil {
		return nil, nil, err
	}
	linhas, err := ex.QueryContext(ctx, selectCard+` WHERE p.lead_id = ?`, leadID)
	if err != nil {
		return nil, nil, err
	}
	cards, err := scanCards(ctx, ex, linhas)
	if err != nil {
		return nil, nil, err
	}
	m := map[string]dominio.Card{}
	for _, c := range cards {
		m[c.FunilID] = c
	}
	return funis, m, nil
}
