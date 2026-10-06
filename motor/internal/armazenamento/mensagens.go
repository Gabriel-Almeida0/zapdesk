package armazenamento

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/pagina"
	"zapdesk/motor/internal/whatsapp"
)

// NovaMensagem são os dados para inserir uma mensagem.
type NovaMensagem struct {
	ID, ContaID, ConversaID, WaID string
	RemetenteJID, RemetenteNome   string
	DeMim                         bool
	Tipo                          string
	Texto                         string
	Midia                         *whatsapp.MidiaInfo
	MidiaCaminho                  string
	ArquivoID                     string
	CitacaoWaID, CitacaoResumo    string
	CitacaoRemetenteNome          string
	Estado                        string
	Erro                          string
	DisparoID                     string
	AutomacaoID                   string
	PrimeiroContato               bool
	EnviadaEm                     time.Time
}

// InserirMensagem insere; devolve false se (conversa_id, wa_id) já existia (idempotente).
func InserirMensagem(ctx context.Context, ex Executor, m NovaMensagem) (bool, error) {
	var midia any
	if m.Midia != nil {
		midia = jsonTexto(m.Midia)
	}
	var texto any
	if m.Texto != "" {
		texto = m.Texto
	}
	r, err := ex.ExecContext(ctx, `INSERT INTO mensagens (id, conta_id, conversa_id, wa_id, remetente_jid, remetente_nome, de_mim,
		tipo, texto, midia, midia_caminho, arquivo_id, citacao_wa_id, citacao_resumo, citacao_remetente_nome, estado, erro,
		disparo_id, automacao_id, primeiro_contato, enviada_em)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (conversa_id, wa_id) DO NOTHING`,
		m.ID, m.ContaID, m.ConversaID, m.WaID, m.RemetenteJID, vazioNil(m.RemetenteNome), b2i(m.DeMim), m.Tipo, texto, midia,
		vazioNil(m.MidiaCaminho), vazioNil(m.ArquivoID), vazioNil(m.CitacaoWaID), vazioNil(m.CitacaoResumo),
		vazioNil(m.CitacaoRemetenteNome), m.Estado, vazioNil(m.Erro), vazioNil(m.DisparoID), vazioNil(m.AutomacaoID),
		b2i(m.PrimeiroContato), Ms(m.EnviadaEm))
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}

// MensagemBruta é a linha com os campos internos.
type MensagemBruta struct {
	dominio.Mensagem
	MidiaInfo    *whatsapp.MidiaInfo
	MidiaCaminho string
	ArquivoID    string
	Rid          int64
	ConversaJID  string
}

const colunasMensagem = `m.rid, m.id, m.conta_id, m.conversa_id, m.wa_id, m.remetente_jid, m.remetente_nome, m.de_mim, m.tipo,
	m.texto, m.midia, m.midia_caminho, m.arquivo_id, m.citacao_wa_id, m.citacao_resumo, m.citacao_remetente_nome, m.editada,
	m.apagada, m.estado, m.erro, m.disparo_id, m.automacao_id, m.enviada_em, cv.jid`

const selectMensagem = `SELECT ` + colunasMensagem + ` FROM mensagens m JOIN conversas cv ON cv.id = m.conversa_id`

func scanMensagem(sc interface{ Scan(...any) error }) (MensagemBruta, error) {
	var b MensagemBruta
	m := &b.Mensagem
	var nome, texto, midia, caminho, arq, cwa, cres, cnome, erro, disp, aut sql.NullString
	var deMim, editada, apagada int
	var em int64
	if err := sc.Scan(&b.Rid, &m.ID, &m.ContaID, &m.ConversaID, &m.WaID, &m.RemetenteJID, &nome, &deMim, &m.Tipo, &texto, &midia,
		&caminho, &arq, &cwa, &cres, &cnome, &editada, &apagada, &m.Estado, &erro, &disp, &aut, &em, &b.ConversaJID); err != nil {
		return b, err
	}
	m.RemetenteNome, m.Texto, m.Erro, m.DisparoID, m.AutomacaoID = deNullStr(nome), deNullStr(texto), deNullStr(erro), deNullStr(disp), deNullStr(aut)
	m.DeMim, m.Editada, m.Apagada, m.EnviadaEm = deMim == 1, editada == 1, apagada == 1, DeMs(em)
	b.MidiaCaminho, b.ArquivoID = caminho.String, arq.String
	if cwa.Valid {
		m.Citacao = &dominio.Citacao{WaID: cwa.String, Resumo: cres.String, RemetenteNome: deNullStr(cnome)}
	}
	if midia.Valid {
		var info whatsapp.MidiaInfo
		if json.Unmarshal([]byte(midia.String), &info) == nil {
			b.MidiaInfo = &info
			m.Midia = MidiaParaAPI(&info, b.MidiaCaminho != "" || b.ArquivoID != "", "/v1/mensagens/"+m.ID+"/midia")
		}
	}
	m.Reacoes = []dominio.Reacao{}
	return b, nil
}

// MidiaParaAPI converte os metadados internos no objeto Midia do contrato.
func MidiaParaAPI(info *whatsapp.MidiaInfo, baixada bool, url string) *dominio.Midia {
	md := &dominio.Midia{Mimetype: info.Mimetype, Tamanho: info.Tamanho, PTT: info.PTT, Baixada: baixada, URL: url}
	md.NomeArquivo = dominio.Str(info.NomeArquivo)
	if info.DuracaoS > 0 {
		d := info.DuracaoS
		md.DuracaoS = &d
	}
	if info.Largura > 0 {
		l, a := info.Largura, info.Altura
		md.Largura, md.Altura = &l, &a
	}
	if len(info.Miniatura) > 0 {
		s := base64.StdEncoding.EncodeToString(info.Miniatura)
		md.MiniaturaB64 = &s
	}
	return md
}

// ObterMensagem busca por id (com reações).
func ObterMensagem(ctx context.Context, ex Executor, id string) (MensagemBruta, error) {
	b, err := scanMensagem(ex.QueryRowContext(ctx, selectMensagem+` WHERE m.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return b, erros.NaoAchada("Mensagem")
	}
	if err != nil {
		return b, err
	}
	lista := []MensagemBruta{b}
	if err := preencherReacoes(ctx, ex, lista); err != nil {
		return b, err
	}
	return lista[0], nil
}

// MensagemPorWaID busca (conta, wa_id). ok=false se não existe.
func MensagemPorWaID(ctx context.Context, ex Executor, contaID, waID string) (MensagemBruta, bool, error) {
	b, err := scanMensagem(ex.QueryRowContext(ctx, selectMensagem+` WHERE m.conta_id = ? AND m.wa_id = ? ORDER BY m.rid DESC LIMIT 1`, contaID, waID))
	if errors.Is(err, sql.ErrNoRows) {
		return b, false, nil
	}
	if err != nil {
		return b, false, err
	}
	lista := []MensagemBruta{b}
	if err := preencherReacoes(ctx, ex, lista); err != nil {
		return b, false, err
	}
	return lista[0], true, nil
}

// ListarMensagens pagina (enviada_em desc, rid desc) antes da mensagem `antesID`.
func ListarMensagens(ctx context.Context, ex Executor, conversaID, antesID string, limite int) ([]MensagemBruta, string, error) {
	onde := `m.conversa_id = ?`
	args := []any{conversaID}
	if antesID != "" {
		var em, rid int64
		err := ex.QueryRowContext(ctx, `SELECT enviada_em, rid FROM mensagens WHERE id = ? AND conversa_id = ?`, antesID, conversaID).Scan(&em, &rid)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", erros.Campo("antes", "Mensagem de referência não encontrada nesta conversa.")
		}
		if err != nil {
			return nil, "", err
		}
		onde += ` AND (m.enviada_em < ? OR (m.enviada_em = ? AND m.rid < ?))`
		args = append(args, em, em, rid)
	}
	args = append(args, limite+1)
	linhas, err := ex.QueryContext(ctx, selectMensagem+` WHERE `+onde+` ORDER BY m.enviada_em DESC, m.rid DESC LIMIT ?`, args...)
	if err != nil {
		return nil, "", err
	}
	defer linhas.Close()
	var lista []MensagemBruta
	for linhas.Next() {
		b, err := scanMensagem(linhas)
		if err != nil {
			return nil, "", err
		}
		lista = append(lista, b)
	}
	if err := linhas.Err(); err != nil {
		return nil, "", err
	}
	proximo := ""
	if len(lista) > limite {
		lista = lista[:limite]
		proximo = lista[len(lista)-1].ID
	}
	return lista, proximo, preencherReacoes(ctx, ex, lista)
}

func preencherReacoes(ctx context.Context, ex Executor, lista []MensagemBruta) error {
	if len(lista) == 0 {
		return nil
	}
	idsM := make([]string, len(lista))
	pos := map[string]int{}
	for i, b := range lista {
		idsM[i] = b.ID
		pos[b.ID] = i
	}
	m, args := marcadores(idsM)
	linhas, err := ex.QueryContext(ctx, `SELECT mensagem_id, remetente_jid, emoji, de_mim FROM reacoes WHERE mensagem_id IN (`+m+`) ORDER BY em`, args...)
	if err != nil {
		return err
	}
	defer linhas.Close()
	for linhas.Next() {
		var mid string
		var r dominio.Reacao
		var deMim int
		if err := linhas.Scan(&mid, &r.RemetenteJID, &r.Emoji, &deMim); err != nil {
			return err
		}
		r.DeMim = deMim == 1
		lista[pos[mid]].Reacoes = append(lista[pos[mid]].Reacoes, r)
	}
	return linhas.Err()
}

// ordemEstado para recibos sem regressão.
var ordemEstado = map[string]int{dominio.MsgPendente: 0, dominio.MsgFalhou: 0, dominio.MsgEnviada: 1, dominio.MsgEntregue: 2, dominio.MsgLida: 3}

// AvancarEstadoMensagem aplica um recibo sem regredir. Devolve true se mudou.
func AvancarEstadoMensagem(ctx context.Context, ex Executor, id, atual, novo string) (bool, error) {
	if ordemEstado[novo] <= ordemEstado[atual] {
		return false, nil
	}
	r, err := ex.ExecContext(ctx, `UPDATE mensagens SET estado = ? WHERE id = ? AND estado = ?`, novo, id, atual)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}

// MarcarEnviada grava wa_id definitivo e estado enviada.
func MarcarEnviada(ctx context.Context, ex Executor, id, waID string, em time.Time) error {
	_, err := ex.ExecContext(ctx, `UPDATE mensagens SET wa_id = ?, estado = 'enviada', erro = NULL, enviada_em = ? WHERE id = ?`, waID, Ms(em), id)
	return err
}

// MarcarFalha grava estado falhou com o motivo.
func MarcarFalha(ctx context.Context, ex Executor, id, motivo string) error {
	_, err := ex.ExecContext(ctx, `UPDATE mensagens SET estado = 'falhou', erro = ? WHERE id = ?`, motivo, id)
	return err
}

// VoltarPendente (reenviar).
func VoltarPendente(ctx context.Context, ex Executor, id string) (bool, error) {
	r, err := ex.ExecContext(ctx, `UPDATE mensagens SET estado = 'pendente', erro = NULL WHERE id = ? AND estado = 'falhou'`, id)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}

// WaIDsNaoLidas devolve os wa_id das últimas `n` mensagens recebidas da conversa.
func WaIDsNaoLidas(ctx context.Context, ex Executor, conversaID string, n int) ([]string, []string, error) {
	if n <= 0 {
		return nil, nil, nil
	}
	linhas, err := ex.QueryContext(ctx, `SELECT wa_id, remetente_jid FROM mensagens WHERE conversa_id = ? AND de_mim = 0
		ORDER BY enviada_em DESC, rid DESC LIMIT ?`, conversaID, n)
	if err != nil {
		return nil, nil, err
	}
	defer linhas.Close()
	var was, rems []string
	for linhas.Next() {
		var w, r string
		if err := linhas.Scan(&w, &r); err != nil {
			return nil, nil, err
		}
		was, rems = append(was, w), append(rems, r)
	}
	return was, rems, linhas.Err()
}

// ResultadoBusca é um item da busca FTS.
type ResultadoBusca struct {
	Mensagem     MensagemBruta
	ConversaID   string
	ConversaNome string
	Trecho       string
}

// termosFTS transforma o texto do usuário numa consulta FTS5 segura (cada termo entre aspas,
// com prefixo; todos obrigatórios).
func termosFTS(q string) string {
	var partes []string
	for _, t := range strings.Fields(q) {
		t = strings.ReplaceAll(t, `"`, "")
		if t == "" {
			continue
		}
		partes = append(partes, `"`+t+`"*`)
	}
	return strings.Join(partes, " ")
}

// BuscarMensagens busca por texto (FTS, sem acento) nas mensagens da conta, mais recentes primeiro.
func BuscarMensagens(ctx context.Context, ex Executor, contaID, q string, p pagina.Params) ([]ResultadoBusca, string, error) {
	consulta := termosFTS(q)
	if consulta == "" {
		return nil, "", nil
	}
	onde := `mensagens_fts MATCH ? AND m.conta_id = ? AND m.apagada = 0`
	args := []any{consulta, contaID}
	if p.Cursor != nil {
		onde += ` AND (m.enviada_em < ? OR (m.enviada_em = ? AND m.rid < ?))`
		var rid int64
		if len(p.Cursor.S) > 0 {
			json.Unmarshal([]byte(p.Cursor.S), &rid)
		}
		args = append(args, p.Cursor.N, p.Cursor.N, rid)
	}
	args = append(args, p.Limite+1)
	linhas, err := ex.QueryContext(ctx, `SELECT `+colunasMensagem+`, snippet(mensagens_fts, 0, '', '', '…', 12),
		CASE WHEN cv.tipo = 'grupo' THEN COALESCE(NULLIF(cv.nome, ''), cv.jid)
		     ELSE COALESCE(NULLIF(ct.nome, ''), NULLIF(ct.nome_push, ''), NULLIF(cv.nome, ''), ct.telefone, cv.jid) END
		FROM mensagens_fts f
		JOIN mensagens m ON m.rid = f.rowid
		JOIN conversas cv ON cv.id = m.conversa_id
		LEFT JOIN contatos ct ON ct.id = cv.contato_id
		WHERE `+onde+` ORDER BY m.enviada_em DESC, m.rid DESC LIMIT ?`, args...)
	if err != nil {
		return nil, "", err
	}
	defer linhas.Close()
	var lista []ResultadoBusca
	for linhas.Next() {
		var r ResultadoBusca
		var trecho, nome string
		b, err := scanMensagem(linhasComExtras{linhas, []any{&trecho, &nome}})
		if err != nil {
			return nil, "", err
		}
		r.Mensagem, r.ConversaID, r.ConversaNome, r.Trecho = b, b.ConversaID, nome, trecho
		lista = append(lista, r)
	}
	if err := linhas.Err(); err != nil {
		return nil, "", err
	}
	proximo := ""
	if len(lista) > p.Limite {
		lista = lista[:p.Limite]
		u := lista[len(lista)-1].Mensagem
		rid, _ := json.Marshal(u.Rid)
		proximo = pagina.Codificar(Ms(u.EnviadaEm), string(rid))
	}
	msgs := make([]MensagemBruta, len(lista))
	for i := range lista {
		msgs[i] = lista[i].Mensagem
	}
	if err := preencherReacoes(ctx, ex, msgs); err != nil {
		return nil, "", err
	}
	for i := range lista {
		lista[i].Mensagem = msgs[i]
	}
	return lista, proximo, nil
}

// linhasComExtras acrescenta destinos extras ao Scan de uma linha.
type linhasComExtras struct {
	r      *sql.Rows
	extras []any
}

func (l linhasComExtras) Scan(dest ...any) error { return l.r.Scan(append(dest, l.extras...)...) }

// DefinirReacao grava (ou remove, se emoji vazio) a reação de um remetente.
func DefinirReacao(ctx context.Context, ex Executor, mensagemID, remetente, emoji string, deMim bool, em time.Time) error {
	if emoji == "" {
		_, err := ex.ExecContext(ctx, `DELETE FROM reacoes WHERE mensagem_id = ? AND remetente_jid = ?`, mensagemID, remetente)
		return err
	}
	_, err := ex.ExecContext(ctx, `INSERT INTO reacoes (mensagem_id, remetente_jid, emoji, de_mim, em) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (mensagem_id, remetente_jid) DO UPDATE SET emoji = excluded.emoji, em = excluded.em`,
		mensagemID, remetente, emoji, b2i(deMim), Ms(em))
	return err
}

// EditarTexto troca o texto e marca editada.
func EditarTexto(ctx context.Context, ex Executor, id, texto string) error {
	_, err := ex.ExecContext(ctx, `UPDATE mensagens SET texto = ?, editada = 1 WHERE id = ?`, texto, id)
	return err
}

// MarcarApagada limpa texto e mídia e marca apagada.
func MarcarApagada(ctx context.Context, ex Executor, id string) error {
	_, err := ex.ExecContext(ctx, `UPDATE mensagens SET texto = NULL, midia = NULL, midia_caminho = NULL, apagada = 1 WHERE id = ?`, id)
	return err
}

// DefinirMidiaCaminho grava o caminho da mídia baixada.
func DefinirMidiaCaminho(ctx context.Context, ex Executor, id, caminho string) error {
	_, err := ex.ExecContext(ctx, `UPDATE mensagens SET midia_caminho = ? WHERE id = ?`, caminho, id)
	return err
}

// Figurinha recente.
type Figurinha struct {
	MensagemID *string `json:"mensagem_id"`
	ArquivoID  *string `json:"arquivo_id"`
	URL        string  `json:"url"`
}

// FigurinhasRecentes devolve figurinhas recebidas/enviadas mais recentes (sem repetir arquivo).
func FigurinhasRecentes(ctx context.Context, ex Executor, contaID string, limite int) ([]Figurinha, error) {
	linhas, err := ex.QueryContext(ctx, `SELECT id, arquivo_id FROM mensagens WHERE conta_id = ? AND tipo = 'figurinha' AND apagada = 0
		ORDER BY enviada_em DESC, rid DESC LIMIT ?`, contaID, limite*3)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	lista := []Figurinha{}
	vistos := map[string]bool{}
	for linhas.Next() && len(lista) < limite {
		var id string
		var arq sql.NullString
		if err := linhas.Scan(&id, &arq); err != nil {
			return nil, err
		}
		f := Figurinha{}
		if arq.Valid {
			if vistos[arq.String] {
				continue
			}
			vistos[arq.String] = true
			a := arq.String
			f.ArquivoID = &a
			f.URL = "/v1/arquivos/" + a + "/conteudo"
		} else {
			m := id
			f.MensagemID = &m
			f.URL = "/v1/mensagens/" + id + "/midia"
		}
		lista = append(lista, f)
	}
	return lista, linhas.Err()
}
