// Gerador determinístico das fixtures de importação. Rode com:
//
//	go test ./testes/dados -run TestGerarFixtures -gerar
//
// leads-1000.csv: 1.000 linhas = 838 novos + 50 já existentes (telefones em
// leads-1000.existentes.txt, a pré-cadastrar) + 100 duplicados no lote + 12 inválidos
// (4 vazios, 4 formato_invalido, 4 numero_invalido). leads.xlsx: 10 linhas simples.
package dados

import (
	"bytes"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

var gerar = flag.Bool("gerar", false, "regrava as fixtures")

var nomes = []string{"Ana", "Bruno", "Carla", "Diego", "Érica", "Fábio", "Gisele", "Hugo", "Íris", "João", "Kátia", "Lúcio", "Márcia", "Nélson", "Otávio"}
var cidades = []string{"São Paulo", "Rio de Janeiro", "Belo Horizonte", "Curitiba", "Recife", "Porto Alegre", "Salvador"}

func telefone(i int) string { return fmt.Sprintf("+55119%08d", 81000000+i) }

// formatar escreve o telefone num formato variado (como numa planilha real).
func formatar(e164 string, estilo int) string {
	n := strings.TrimPrefix(e164, "+55")
	switch estilo % 4 {
	case 0:
		return fmt.Sprintf("(%s) %s-%s", n[:2], n[2:7], n[7:])
	case 1:
		return n
	case 2:
		return "+55 " + n[:2] + " " + n[2:7] + "-" + n[7:]
	default:
		return "55" + n
	}
}

func TestGerarFixtures(t *testing.T) {
	if !*gerar {
		t.Skip("use -gerar para regravar as fixtures")
	}
	rnd := rand.New(rand.NewSource(42))
	type linha struct{ nome, tel, empresa, cidade string }
	var unicas []linha
	for i := 0; i < 888; i++ {
		unicas = append(unicas, linha{nomes[i%len(nomes)] + fmt.Sprintf(" %d", i), formatar(telefone(i), i),
			fmt.Sprintf("Empresa %d", i%37), cidades[i%len(cidades)]})
	}
	var existentes []string
	for i := 838; i < 888; i++ {
		existentes = append(existentes, telefone(i))
	}
	linhas := append([]linha(nil), unicas...)
	// 100 duplicados: repetem telefones já presentes, em outro formato, sempre depois do original
	var dups []linha
	for i := 0; i < 100; i++ {
		j := rnd.Intn(888)
		dups = append(dups, linha{"Repetido " + fmt.Sprint(i), formatar(telefone(j), j+1), "", ""})
	}
	invalidos := []linha{{"Vazio 1", "", "", ""}, {"Vazio 2", "   ", "", ""}, {"Vazio 3", "", "X", ""}, {"Vazio 4", "", "", "Recife"},
		{"Letras 1", "abc", "", ""}, {"Letras 2", "não tem", "", ""}, {"Letras 3", "11-9999-ABCD", "", ""}, {"Letras 4", "tel: 11", "", ""},
		{"Curto 1", "+55 11 1234", "", ""}, {"Curto 2", "123", "", ""}, {"Curto 3", "11 1234", "", ""}, {"Curto 4", "0000", "", ""}}
	linhas = append(linhas, dups...)
	linhas = append(linhas, invalidos...)
	// embaralha mantendo cada duplicado depois do seu original: embaralha só as 888 únicas + inválidos
	cabeca := append([]linha(nil), linhas[:888]...)
	cabeca = append(cabeca, invalidos...)
	rnd.Shuffle(len(cabeca), func(i, j int) { cabeca[i], cabeca[j] = cabeca[j], cabeca[i] })
	final := append(cabeca, dups...)
	if len(final) != 1000 {
		t.Fatalf("total %d", len(final))
	}

	var buf bytes.Buffer
	buf.WriteString("\ufeffNome;Celular;Empresa;Cidade\n")
	for _, l := range final {
		fmt.Fprintf(&buf, "%s;%s;%s;%s\n", l.nome, l.tel, l.empresa, l.cidade)
	}
	if err := os.WriteFile("leads-1000.csv", buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("leads-1000.existentes.txt", []byte(strings.Join(existentes, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f := excelize.NewFile()
	f.SetSheetRow("Sheet1", "A1", &[]any{"Nome", "Telefone", "Cidade"})
	for i := 0; i < 10; i++ {
		f.SetSheetRow("Sheet1", fmt.Sprintf("A%d", i+2), &[]any{nomes[i], formatar(telefone(2000+i), i), cidades[i%len(cidades)]})
	}
	if err := f.SaveAs("leads.xlsx"); err != nil {
		t.Fatal(err)
	}
}

func TestFixturesExistem(t *testing.T) {
	dados, err := os.ReadFile("leads-1000.csv")
	if err != nil {
		t.Fatalf("fixture ausente (rode com -gerar): %v", err)
	}
	if n := strings.Count(string(dados), "\n"); n != 1001 {
		t.Fatalf("leads-1000.csv deveria ter 1.001 linhas (com cabeçalho): %d", n)
	}
	ex, _ := os.ReadFile("leads-1000.existentes.txt")
	if n := len(strings.Fields(string(ex))); n != 50 {
		t.Fatalf("existentes: %d", n)
	}
	if _, err := os.Stat("leads.xlsx"); err != nil {
		t.Fatal(err)
	}
}
