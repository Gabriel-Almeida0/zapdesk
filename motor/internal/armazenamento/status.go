package armazenamento

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/whatsapp"
)

// NovoStatus são os dados de um status publicado.
type NovoStatus struct {
	ID, ContaID, WaID, ContatoJID, ContatoNome, Tipo, Texto string
	Midia                                                   *whatsapp.MidiaInfo
	PublicadoEm                                             time.Time
}

// InserirStatus grava (idempotente por conta + wa_id).
func InserirStatus(ctx context.Context, ex Executor, s NovoStatus) (bool, error) {
	var midia any
	if s.Midia != nil {
		midia = jsonTexto(s.Midia)
	}
	r, err := ex.ExecContext(ctx, `INSERT INTO status_contatos (id, conta_id, wa_id, contato_jid, contato_nome, tipo, texto, midia, publicado_em)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (conta_id, wa_id) DO NOTHING`,
		s.ID, s.ContaID, s.WaID, s.ContatoJID, vazioNil(s.ContatoNome), s.Tipo, vazioNil(s.Texto), midia, Ms(s.PublicadoEm))
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}

// StatusBruto inclui os dados internos de mídia.
type StatusBruto struct {
	dominio.Status
	MidiaInfo    *whatsapp.MidiaInfo
	MidiaCaminho string
}

const selectStatus = `SELECT s.id, s.conta_id, s.contato_jid,
	COALESCE(NULLIF(ct.nome, ''), NULLIF(ct.nome_push, ''), s.contato_nome), s.tipo, s.texto, s.midia, s.midia_caminho, s.publicado_em
	FROM status_contatos s LEFT JOIN contatos ct ON ct.conta_id = s.conta_id AND ct.jid = s.contato_jid`

func scanStatus(sc interface{ Scan(...any) error }) (StatusBruto, error) {
	var b StatusBruto
	var nome, texto, midia, caminho sql.NullString
	var em int64
	if err := sc.Scan(&b.ID, &b.ContaID, &b.ContatoJID, &nome, &b.Tipo, &texto, &midia, &caminho, &em); err != nil {
		return b, err
	}
	b.ContatoNome, b.Texto, b.PublicadoEm, b.MidiaCaminho = deNullStr(nome), deNullStr(texto), DeMs(em), caminho.String
	if midia.Valid {
		var info whatsapp.MidiaInfo
		if json.Unmarshal([]byte(midia.String), &info) == nil {
			b.MidiaInfo = &info
			b.Midia = MidiaParaAPI(&info, b.MidiaCaminho != "", "/v1/status/"+b.ID+"/midia")
		}
	}
	return b, nil
}

// ObterStatus busca por id.
func ObterStatus(ctx context.Context, ex Executor, id string) (StatusBruto, error) {
	b, err := scanStatus(ex.QueryRowContext(ctx, selectStatus+` WHERE s.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return b, erros.NaoAchado("Status")
	}
	return b, err
}

// ListarStatusRecentes devolve os status publicados depois de `desde`, mais recentes primeiro.
func ListarStatusRecentes(ctx context.Context, ex Executor, contaID string, desde time.Time) ([]StatusBruto, error) {
	linhas, err := ex.QueryContext(ctx, selectStatus+` WHERE s.conta_id = ? AND s.publicado_em > ? ORDER BY s.publicado_em DESC, s.id DESC`,
		contaID, Ms(desde))
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	var lista []StatusBruto
	for linhas.Next() {
		b, err := scanStatus(linhas)
		if err != nil {
			return nil, err
		}
		lista = append(lista, b)
	}
	return lista, linhas.Err()
}

// ApagarStatusAntigos remove status publicados antes de `limite`; devolve os caminhos de mídia.
func ApagarStatusAntigos(ctx context.Context, ex Executor, limite time.Time) ([]string, error) {
	linhas, err := ex.QueryContext(ctx, `SELECT midia_caminho FROM status_contatos WHERE publicado_em <= ? AND midia_caminho IS NOT NULL`, Ms(limite))
	if err != nil {
		return nil, err
	}
	var caminhos []string
	for linhas.Next() {
		var c string
		linhas.Scan(&c)
		caminhos = append(caminhos, c)
	}
	linhas.Close()
	_, err = ex.ExecContext(ctx, `DELETE FROM status_contatos WHERE publicado_em <= ?`, Ms(limite))
	return caminhos, err
}

// DefinirMidiaStatus grava o caminho da mídia baixada.
func DefinirMidiaStatus(ctx context.Context, ex Executor, id, caminho string) error {
	_, err := ex.ExecContext(ctx, `UPDATE status_contatos SET midia_caminho = ? WHERE id = ?`, caminho, id)
	return err
}
