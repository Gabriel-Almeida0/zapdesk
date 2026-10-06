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

// TamanhoMaximoNomeLead (data-model.md › Lead).
const TamanhoMaximoNomeLead = 120

const selectLead = `SELECT l.id, l.telefone, l.nome, l.campos, l.origem, l.tem_whatsapp, l.importado_em,
	(SELECT MAX(d.enviado_em) FROM destinatarios d WHERE d.lead_id = l.id) FROM leads l`

func scanLead(sc interface{ Scan(...any) error }) (dominio.Lead, error) {
	var l dominio.Lead
	var nome sql.NullString
	var campos string
	var tem, ultimo sql.NullInt64
	var imp int64
	if err := sc.Scan(&l.ID, &l.Telefone, &nome, &campos, &l.Origem, &tem, &imp, &ultimo); err != nil {
		return l, err
	}
	l.Nome, l.ImportadoEm, l.UltimoDisparoEm = deNullStr(nome), DeMs(imp), deNullMs(ultimo)
	l.Campos = map[string]string{}
	json.Unmarshal([]byte(campos), &l.Campos)
	if tem.Valid {
		v := tem.Int64 == 1
		l.TemWhatsApp = &v
	}
	return l, nil
}

// ObterLead busca por id.
func ObterLead(ctx context.Context, ex Executor, id string) (dominio.Lead, error) {
	l, err := scanLead(ex.QueryRowContext(ctx, selectLead+` WHERE l.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return l, erros.NaoAchado("Lead")
	}
	return l, err
}

// LeadPorTelefone busca pelo E.164.
func LeadPorTelefone(ctx context.Context, ex Executor, tel string) (dominio.Lead, error) {
	l, err := scanLead(ex.QueryRowContext(ctx, selectLead+` WHERE l.telefone = ?`, tel))
	if errors.Is(err, sql.ErrNoRows) {
		return l, erros.NaoAchado("Lead")
	}
	return l, err
}

// LeadsPorTelefones devolve telefone → lead para os telefones existentes.
func LeadsPorTelefones(ctx context.Context, ex Executor, tels []string) (map[string]dominio.Lead, error) {
	res := map[string]dominio.Lead{}
	for inicio := 0; inicio < len(tels); inicio += 500 {
		fim := min(inicio+500, len(tels))
		m, args := marcadores(tels[inicio:fim])
		linhas, err := ex.QueryContext(ctx, selectLead+` WHERE l.telefone IN (`+m+`)`, args...)
		if err != nil {
			return nil, err
		}
		for linhas.Next() {
			l, err := scanLead(linhas)
			if err != nil {
				linhas.Close()
				return nil, err
			}
			res[l.Telefone] = l
		}
		linhas.Close()
		if err := linhas.Err(); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// LeadsPorIDs devolve os leads na ordem pedida (ignora inexistentes).
func LeadsPorIDs(ctx context.Context, ex Executor, idsL []string) ([]dominio.Lead, error) {
	porID := map[string]dominio.Lead{}
	for inicio := 0; inicio < len(idsL); inicio += 500 {
		fim := min(inicio+500, len(idsL))
		m, args := marcadores(idsL[inicio:fim])
		linhas, err := ex.QueryContext(ctx, selectLead+` WHERE l.id IN (`+m+`)`, args...)
		if err != nil {
			return nil, err
		}
		for linhas.Next() {
			l, err := scanLead(linhas)
			if err != nil {
				linhas.Close()
				return nil, err
			}
			porID[l.ID] = l
		}
		linhas.Close()
	}
	lista := make([]dominio.Lead, 0, len(idsL))
	for _, id := range idsL {
		if l, ok := porID[id]; ok {
			lista = append(lista, l)
		}
	}
	return lista, nil
}

// InserirLead cria um lead.
func InserirLead(ctx context.Context, ex Executor, l dominio.Lead, agora time.Time) error {
	_, err := ex.ExecContext(ctx, `INSERT INTO leads (id, telefone, nome, campos, origem, importado_em, atualizado_em) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		l.ID, l.Telefone, strOuNil(l.Nome), jsonTexto(l.Campos), l.Origem, Ms(l.ImportadoEm), Ms(agora))
	return err
}

// AtualizarLeadDados grava nome e campos (preenchimento de vazios).
func AtualizarLeadDados(ctx context.Context, ex Executor, id string, nome *string, campos map[string]string, agora time.Time) error {
	_, err := ex.ExecContext(ctx, `UPDATE leads SET nome = ?, campos = ?, atualizado_em = ? WHERE id = ?`,
		strOuNil(nome), jsonTexto(campos), Ms(agora), id)
	return err
}

// LigarContatosAoLead liga contatos de mesmo telefone ainda sem lead.
func LigarContatosAoLead(ctx context.Context, ex Executor, leadID, telefone string) error {
	_, err := ex.ExecContext(ctx, `UPDATE contatos SET lead_id = ? WHERE telefone = ? AND lead_id IS NULL`, leadID, telefone)
	return err
}

// DefinirTemWhatsApp grava o resultado da verificação.
func DefinirTemWhatsApp(ctx context.Context, ex Executor, id string, tem bool) error {
	_, err := ex.ExecContext(ctx, `UPDATE leads SET tem_whatsapp = ? WHERE id = ?`, b2i(tem), id)
	return err
}

// FiltroLeads da listagem.
type FiltroLeads struct {
	Busca  string
	Origem string
}

// ListarLeads pagina por (importado_em desc, id desc).
func ListarLeads(ctx context.Context, ex Executor, f FiltroLeads, p pagina.Params) ([]dominio.Lead, string, error) {
	var onde []string
	var args []any
	if f.Busca != "" {
		onde = append(onde, `(l.telefone LIKE ? ESCAPE '\' OR l.nome LIKE ? ESCAPE '\' OR l.campos LIKE ? ESCAPE '\')`)
		b := escaparLike(f.Busca)
		args = append(args, b, b, b)
	}
	if f.Origem != "" {
		onde = append(onde, `l.origem = ?`)
		args = append(args, f.Origem)
	}
	if p.Cursor != nil {
		onde = append(onde, `(l.importado_em < ? OR (l.importado_em = ? AND l.id < ?))`)
		args = append(args, p.Cursor.N, p.Cursor.N, p.Cursor.S)
	}
	q := selectLead
	if len(onde) > 0 {
		q += ` WHERE ` + strings.Join(onde, " AND ")
	}
	q += ` ORDER BY l.importado_em DESC, l.id DESC LIMIT ?`
	args = append(args, p.Limite+1)
	linhas, err := ex.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, "", err
	}
	defer linhas.Close()
	var lista []dominio.Lead
	for linhas.Next() {
		l, err := scanLead(linhas)
		if err != nil {
			return nil, "", err
		}
		lista = append(lista, l)
	}
	if err := linhas.Err(); err != nil {
		return nil, "", err
	}
	prox := ""
	if len(lista) > p.Limite {
		lista = lista[:p.Limite]
		u := lista[len(lista)-1]
		prox = pagina.Codificar(Ms(u.ImportadoEm), u.ID)
	}
	return lista, prox, nil
}
