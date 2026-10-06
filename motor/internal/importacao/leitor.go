// Pacote importacao lê planilhas CSV/XLSX para a importação de leads (research.md §4):
// detecção de separador e BOM, primeira planilha do XLSX, sugestão das colunas de telefone e
// nome e normalização das chaves de campos.
package importacao

import (
	"bytes"
	"encoding/csv"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"zapdesk/motor/internal/erros"
)

// TamanhoMaximo de uma planilha.
const TamanhoMaximo = 50 << 20

// LinhaPlanilha é uma linha de dados (Numero conta a partir da primeira linha de dados).
type LinhaPlanilha struct {
	Numero  int
	Valores []string // alinhados a Colunas
}

// Planilha lida.
type Planilha struct {
	Colunas []string
	Linhas  []LinhaPlanilha
}

var errNaoSuportado = erros.Novo(erros.TipoNaoSuportado, "Use uma planilha .csv ou .xlsx.")

// Ler interpreta o arquivo pelo conteúdo (zip = XLSX) e pela extensão.
func Ler(nome string, r io.Reader) (Planilha, error) {
	dados, err := io.ReadAll(io.LimitReader(r, TamanhoMaximo+1))
	if err != nil {
		return Planilha{}, err
	}
	if len(dados) > TamanhoMaximo {
		return Planilha{}, erros.ComDetalhes(erros.AnexoGrandeDemais, "Planilha grande demais (limite de 50 MB).", map[string]any{"limite_bytes": TamanhoMaximo})
	}
	ext := strings.ToLower(filepath.Ext(nome))
	var linhas [][]string
	var posicoes []int
	switch {
	case bytes.HasPrefix(dados, []byte("PK\x03\x04")):
		linhas, posicoes, err = lerXLSX(dados)
	case ext == ".csv" || ext == ".txt" || ext == ".tsv" || ext == "":
		linhas, posicoes, err = lerCSV(dados)
	default:
		return Planilha{}, errNaoSuportado
	}
	if err != nil {
		return Planilha{}, err
	}
	return montar(linhas, posicoes)
}

func vazia(l []string) bool {
	for _, v := range l {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

func montar(linhas [][]string, posicoes []int) (Planilha, error) {
	cab := -1
	for i, l := range linhas {
		if !vazia(l) {
			cab = i
			break
		}
	}
	if cab < 0 {
		return Planilha{}, erros.Campo("arquivo", "A planilha está vazia.")
	}
	p := Planilha{}
	vistas := map[string]int{}
	for i, c := range linhas[cab] {
		c = strings.TrimSpace(c)
		if c == "" {
			c = "Coluna " + strconv.Itoa(i+1)
		}
		if n := vistas[c]; n > 0 {
			vistas[c] = n + 1
			c = c + " (" + strconv.Itoa(n+1) + ")"
		} else {
			vistas[c] = 1
		}
		p.Colunas = append(p.Colunas, c)
	}
	base := posicoes[cab]
	for i := cab + 1; i < len(linhas); i++ {
		if vazia(linhas[i]) {
			continue
		}
		valores := make([]string, len(p.Colunas))
		for j := range valores {
			if j < len(linhas[i]) {
				valores[j] = strings.TrimSpace(linhas[i][j])
			}
		}
		p.Linhas = append(p.Linhas, LinhaPlanilha{Numero: posicoes[i] - base, Valores: valores})
	}
	return p, nil
}

// latin1 converte bytes ISO-8859-1/Windows-1252 (planilhas antigas do Excel) para UTF-8.
func latin1(dados []byte) []byte {
	var b strings.Builder
	for _, c := range dados {
		b.WriteRune(rune(c))
	}
	return []byte(b.String())
}

func detectarSeparador(dados []byte) rune {
	primeira := dados
	if i := bytes.IndexByte(dados, '\n'); i >= 0 {
		primeira = dados[:i]
	}
	contagem := map[rune]int{}
	aspas := false
	for _, c := range string(primeira) {
		switch {
		case c == '"':
			aspas = !aspas
		case !aspas && (c == ';' || c == ',' || c == '\t'):
			contagem[c]++
		}
	}
	sep, max := ',', 0
	for _, c := range []rune{';', ',', '\t'} {
		if contagem[c] > max {
			sep, max = c, contagem[c]
		}
	}
	return sep
}

func lerCSV(dados []byte) ([][]string, []int, error) {
	dados = bytes.TrimPrefix(dados, []byte("\xef\xbb\xbf"))
	if !utf8.Valid(dados) {
		dados = latin1(dados)
	}
	if bytes.IndexByte(dados, 0) >= 0 {
		return nil, nil, errNaoSuportado
	}
	leitor := csv.NewReader(bytes.NewReader(dados))
	leitor.Comma = detectarSeparador(dados)
	leitor.FieldsPerRecord = -1
	leitor.LazyQuotes = true
	var linhas [][]string
	var posicoes []int
	for {
		reg, err := leitor.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, erros.Campo("arquivo", "Não consegui ler o CSV: "+err.Error())
		}
		linha, _ := leitor.FieldPos(0)
		linhas = append(linhas, reg)
		posicoes = append(posicoes, linha)
	}
	return linhas, posicoes, nil
}

func lerXLSX(dados []byte) ([][]string, []int, error) {
	f, err := excelize.OpenReader(bytes.NewReader(dados))
	if err != nil {
		return nil, nil, errNaoSuportado
	}
	defer f.Close()
	planilhas := f.GetSheetList()
	if len(planilhas) == 0 {
		return nil, nil, erros.Campo("arquivo", "A planilha está vazia.")
	}
	linhas, err := f.GetRows(planilhas[0], excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, nil, erros.Campo("arquivo", "Não consegui ler o XLSX: "+err.Error())
	}
	posicoes := make([]int, len(linhas))
	for i := range linhas {
		posicoes[i] = i + 1
	}
	return linhas, posicoes, nil
}

var semAcento = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// NormalizarChave: minúsculas, sem acento, qualquer outro caractere vira "_".
func NormalizarChave(s string) string {
	s, _, _ = transform.String(semAcento, strings.ToLower(strings.TrimSpace(s)))
	var b strings.Builder
	sub := false
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
			sub = false
		} else if !sub && b.Len() > 0 {
			b.WriteByte('_')
			sub = true
		}
	}
	return strings.TrimSuffix(b.String(), "_")
}

var (
	chavesTelefone = []string{"telefone", "tel", "celular", "cel", "whatsapp", "whats", "zap", "fone", "phone", "numero", "mobile"}
	chavesNome     = []string{"nome", "name", "cliente", "contato"}
)

func combina(coluna string, chaves []string) bool {
	for _, token := range strings.Split(NormalizarChave(coluna), "_") {
		for _, k := range chaves {
			if strings.HasPrefix(token, k) {
				return true
			}
		}
	}
	return false
}

// SugerirColunas sugere as colunas de telefone e de nome ("" se nenhuma).
func SugerirColunas(colunas []string) (tel, nome string) {
	for _, c := range colunas {
		if combina(c, chavesTelefone) {
			tel = c
			break
		}
	}
	for _, c := range colunas {
		if c != tel && combina(c, chavesNome) {
			nome = c
			break
		}
	}
	return tel, nome
}
