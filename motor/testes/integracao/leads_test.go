package integracao

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type Relatorio struct {
	TotalLinhas           int `json:"total_linhas"`
	TotalNovos            int `json:"total_novos"`
	TotalJaExistentes     int `json:"total_ja_existentes"`
	TotalInvalidos        int `json:"total_invalidos"`
	TotalDuplicadosNoLote int `json:"total_duplicados_no_lote"`
	Novos                 []struct {
		Linha    int    `json:"linha"`
		LeadID   string `json:"lead_id"`
		Telefone string `json:"telefone"`
	} `json:"novos"`
	JaExistentes []struct {
		Linha             int      `json:"linha"`
		LeadID            string   `json:"lead_id"`
		ImportadoEm       string   `json:"importado_em"`
		CamposPreenchidos []string `json:"campos_preenchidos"`
	} `json:"ja_existentes"`
	Invalidos []struct {
		Linha  int    `json:"linha"`
		Valor  string `json:"valor"`
		Motivo string `json:"motivo"`
	} `json:"invalidos"`
	DuplicadosNoLote []struct {
		Linha         int    `json:"linha"`
		Telefone      string `json:"telefone"`
		PrimeiraLinha int    `json:"primeira_linha"`
	} `json:"duplicados_no_lote"`
	LeadIDs []string `json:"lead_ids"`
}

type Lead struct {
	ID              string            `json:"id"`
	Telefone        string            `json:"telefone"`
	Nome            *string           `json:"nome"`
	Campos          map[string]string `json:"campos"`
	Origem          string            `json:"origem"`
	TemWhatsApp     *bool             `json:"tem_whatsapp"`
	ImportadoEm     string            `json:"importado_em"`
	UltimoDisparoEm *string           `json:"ultimo_disparo_em"`
}

// multipart envia um arquivo no campo "arquivo".
func (m *Motor) multipart(caminho, nome string, conteudo []byte) (int, []byte) {
	m.t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	parte, _ := w.CreateFormFile("arquivo", nome)
	parte.Write(conteudo)
	w.Close()
	r, _ := http.NewRequest("POST", m.Base+caminho, &buf)
	r.Header.Set("Authorization", "Bearer "+tokenTeste)
	r.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := m.cli.Do(r)
	if err != nil {
		m.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func TestImportacaoDaFixture1000(t *testing.T) {
	m := novoMotor(t)
	existentes, _ := os.ReadFile("../dados/leads-1000.existentes.txt")
	var pre []map[string]string
	for _, tel := range strings.Fields(string(existentes)) {
		pre = append(pre, map[string]string{"telefone": tel})
	}
	var r0 Relatorio
	m.json("POST", "/v1/leads/importar", map[string]any{"leads": pre, "origem": "mcp"}, 200, &r0)
	if r0.TotalNovos != 50 {
		t.Fatalf("pré-cadastro: %+v", r0.TotalNovos)
	}

	csv, err := os.ReadFile("../dados/leads-1000.csv")
	if err != nil {
		t.Fatal(err)
	}
	inicio := time.Now()
	st, b := m.multipart("/v1/importacoes/previa", "leads-1000.csv", csv)
	if st != 201 {
		t.Fatalf("prévia: %d %s", st, b)
	}
	var previa struct {
		ImportacaoID           string     `json:"importacao_id"`
		NomeArquivo            string     `json:"nome_arquivo"`
		Colunas                []string   `json:"colunas"`
		Amostra                [][]string `json:"amostra"`
		TotalLinhas            int        `json:"total_linhas"`
		ColunaTelefoneSugerida *string    `json:"coluna_telefone_sugerida"`
		ColunaNomeSugerida     *string    `json:"coluna_nome_sugerida"`
		ExpiraEm               string     `json:"expira_em"`
	}
	json.Unmarshal(b, &previa)
	if previa.TotalLinhas != 1000 || len(previa.Amostra) != 5 || ptr(previa.ColunaTelefoneSugerida) != "Celular" ||
		ptr(previa.ColunaNomeSugerida) != "Nome" || previa.NomeArquivo != "leads-1000.csv" || previa.ExpiraEm == "" {
		t.Fatalf("prévia: %+v", previa)
	}

	m.erro("POST", "/v1/leads/importar", map[string]any{"importacao_id": previa.ImportacaoID, "mapeamento": map[string]any{}}, 422, "validacao")
	e := m.erro("POST", "/v1/leads/importar", map[string]any{"importacao_id": previa.ImportacaoID, "mapeamento": map[string]any{"nome": "Nome"}}, 422, "validacao")
	if e["mensagem"] != "Escolha qual coluna tem o telefone." {
		t.Fatalf("copy: %v", e["mensagem"])
	}

	marca := m.marca()
	var rel Relatorio
	m.json("POST", "/v1/leads/importar", map[string]any{
		"importacao_id": previa.ImportacaoID,
		"mapeamento":    map[string]any{"telefone": "Celular", "nome": "Nome"},
	}, 200, &rel)
	if d := time.Since(inicio); d > 5*time.Second {
		t.Fatalf("importação de 1.000 linhas levou %v (meta < 5 s)", d)
	}
	t.Logf("prévia + importação de 1.000 linhas: %v", time.Since(inicio))
	if rel.TotalLinhas != 1000 || rel.TotalNovos != 838 || rel.TotalJaExistentes != 50 || rel.TotalDuplicadosNoLote != 100 || rel.TotalInvalidos != 12 {
		t.Fatalf("relatório: linhas=%d novos=%d ja=%d dup=%d inv=%d", rel.TotalLinhas, rel.TotalNovos, rel.TotalJaExistentes, rel.TotalDuplicadosNoLote, rel.TotalInvalidos)
	}
	if len(rel.LeadIDs) != 888 {
		t.Fatalf("lead_ids: %d", len(rel.LeadIDs))
	}
	motivos := map[string]int{}
	for _, inv := range rel.Invalidos {
		motivos[inv.Motivo]++
	}
	if motivos["vazio"] != 4 || motivos["formato_invalido"] != 4 || motivos["numero_invalido"] != 4 {
		t.Fatalf("motivos dos inválidos: %v", motivos)
	}
	ev := m.esperarEvento(marca, "leads.importados", nil)
	d := dados[map[string]any](t, ev)
	if d["origem"] != "csv" || d["total_novos"] != float64(838) || d["total_ja_existentes"] != float64(50) || d["total_invalidos"] != float64(12) {
		t.Fatalf("evento: %v", d)
	}

	// extras = todas as outras colunas, com chave normalizada
	var p Pagina[Lead]
	m.json("GET", "/v1/leads?limite=200", nil, 200, &p)
	if len(p.Itens) != 200 || p.ProximoCursor == nil {
		t.Fatalf("paginação de leads: %d", len(p.Itens))
	}
	var novo Lead
	m.json("GET", "/v1/leads/"+rel.Novos[0].LeadID, nil, 200, &novo)
	if novo.Origem != "csv" || novo.Campos["empresa"] == "" || novo.Campos["cidade"] == "" || novo.Nome == nil {
		t.Fatalf("lead importado: %+v", novo)
	}
	if _, ok := novo.Campos["celular"]; ok {
		t.Fatal("a coluna de telefone não deve ir para campos")
	}
	total := 0
	cursor := ""
	for {
		url := "/v1/leads?limite=200&origem=csv"
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		var pg Pagina[Lead]
		m.json("GET", url, nil, 200, &pg)
		total += len(pg.Itens)
		if pg.ProximoCursor == nil {
			break
		}
		cursor = *pg.ProximoCursor
	}
	if total != 838 {
		t.Fatalf("filtro origem=csv: %d", total)
	}
	m.json("GET", "/v1/leads?busca=Empresa%2036", nil, 200, &p)
	if len(p.Itens) == 0 {
		t.Fatal("busca em campos")
	}
	m.erro("GET", "/v1/leads/inexistente", nil, 404, "nao_encontrado")
	m.erro("GET", "/v1/leads?origem=outra", nil, 422, "validacao")

	// a prévia é consumida pela importação
	m.erro("POST", "/v1/leads/importar", map[string]any{"importacao_id": previa.ImportacaoID, "mapeamento": map[string]any{"telefone": "Celular"}}, 404, "nao_encontrado")
}

func TestImportacaoFormasEXLSX(t *testing.T) {
	m := novoMotor(t)

	var rel Relatorio
	m.json("POST", "/v1/leads/importar", map[string]any{"texto_colado": "11 99999-0000\n\n+55 21 98888-7777\n11999990000\nxyz"}, 200, &rel)
	if rel.TotalLinhas != 4 || rel.TotalNovos != 2 || rel.TotalDuplicadosNoLote != 1 || rel.TotalInvalidos != 1 {
		t.Fatalf("colado: %+v", rel)
	}
	var l Lead
	m.json("GET", "/v1/leads/"+rel.Novos[0].LeadID, nil, 200, &l)
	if l.Origem != "colado" {
		t.Fatalf("origem colado: %+v", l)
	}

	m.json("POST", "/v1/leads/importar", map[string]any{
		"leads": []map[string]any{{"telefone": "+5511977770000", "nome": "Zé", "campos": map[string]string{"Razão Social": "Zé LTDA"}}},
	}, 200, &rel)
	m.json("GET", "/v1/leads/"+rel.Novos[0].LeadID, nil, 200, &l)
	if l.Origem != "mcp" || l.Campos["razao_social"] != "Zé LTDA" {
		t.Fatalf("estruturada: %+v", l)
	}
	m.erro("POST", "/v1/leads/importar", map[string]any{"texto_colado": "x", "leads": []any{}}, 422, "validacao")
	m.erro("POST", "/v1/leads/importar", map[string]any{}, 422, "validacao")

	xlsx, _ := os.ReadFile("../dados/leads.xlsx")
	st, b := m.multipart("/v1/importacoes/previa", "leads.xlsx", xlsx)
	if st != 201 {
		t.Fatalf("prévia xlsx: %d %s", st, b)
	}
	var pv struct {
		ImportacaoID string `json:"importacao_id"`
		TotalLinhas  int    `json:"total_linhas"`
	}
	json.Unmarshal(b, &pv)
	m.json("POST", "/v1/leads/importar", map[string]any{"importacao_id": pv.ImportacaoID, "mapeamento": map[string]any{"telefone": "Telefone", "extras": []string{}}}, 200, &rel)
	if pv.TotalLinhas != 10 || rel.TotalNovos != 10 {
		t.Fatalf("xlsx: %d %+v", pv.TotalLinhas, rel)
	}
	var lx Lead
	m.json("GET", "/v1/leads/"+rel.Novos[0].LeadID, nil, 200, &lx)
	if l = lx; len(l.Campos) != 0 || l.Nome != nil {
		t.Fatalf("extras vazio = nenhum campo, sem nome mapeado: %+v", l)
	}

	// prévia por caminho e tipo não suportado
	var pc struct {
		TotalLinhas int `json:"total_linhas"`
	}
	abs, _ := os.Getwd()
	m.json("POST", "/v1/importacoes/previa", map[string]string{"caminho": abs + "/../dados/leads-1000.csv"}, 201, &pc)
	if pc.TotalLinhas != 1000 {
		t.Fatalf("prévia por caminho: %+v", pc)
	}
	st, b = m.multipart("/v1/importacoes/previa", "foto.png", []byte("\x89PNG\r\n\x1a\nxxxx"))
	if st != 415 || !strings.Contains(string(b), "tipo_nao_suportado") {
		t.Fatalf("tipo não suportado: %d %s", st, b)
	}
	m.erro("POST", "/v1/importacoes/previa", map[string]string{"caminho": "/nao/existe.csv"}, 422, "validacao")
}

func TestImportarContatos(t *testing.T) {
	m := novoMotor(t)
	c := m.contaConectada("+5511900000001")
	m.receber(c.ID, map[string]any{"de": "+5511944445555", "texto": "oi", "nome": "Carla"})
	m.receber(c.ID, map[string]any{"de": "+5511933332222", "texto": "oi", "nome": "Davi"})
	var p Pagina[Contato]
	m.json("GET", "/v1/contas/"+c.ID+"/contatos", nil, 200, &p)
	var ids []string
	for _, ct := range p.Itens {
		ids = append(ids, ct.ID)
	}
	var rel Relatorio
	m.json("POST", "/v1/leads/importar-contatos", map[string]any{"conta_id": c.ID, "contato_ids": ids}, 200, &rel)
	if rel.TotalNovos != 2 {
		t.Fatalf("importar contatos: %+v", rel)
	}
	var l Lead
	m.json("GET", "/v1/leads/"+rel.Novos[0].LeadID, nil, 200, &l)
	if l.Origem != "contatos" || l.Nome == nil {
		t.Fatalf("lead de contato: %+v", l)
	}
	var ct Contato
	m.json("GET", "/v1/contatos/"+ids[0], nil, 200, &ct)
	if ct.Lead == nil || ct.Lead.Origem != "contatos" {
		t.Fatalf("contato deveria apontar para o lead: %+v", ct.Lead)
	}
	m.erro("POST", "/v1/leads/importar-contatos", map[string]any{"conta_id": c.ID}, 422, "validacao")
	m.erro("POST", "/v1/leads/importar-contatos", map[string]any{"conta_id": "x", "contato_ids": ids}, 404, "nao_encontrado")
}
