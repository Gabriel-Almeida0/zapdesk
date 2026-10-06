package importacao

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"zapdesk/motor/internal/erros"
)

func TestCSVSeparadores(t *testing.T) {
	for _, sep := range []string{",", ";", "\t"} {
		conteudo := "Nome" + sep + "Celular" + sep + "Cidade\nAna" + sep + "11 99999-0000" + sep + "São Paulo\n"
		p, err := Ler("leads.csv", strings.NewReader(conteudo))
		if err != nil {
			t.Fatalf("sep %q: %v", sep, err)
		}
		if !reflect.DeepEqual(p.Colunas, []string{"Nome", "Celular", "Cidade"}) || len(p.Linhas) != 1 || p.Linhas[0].Valores[2] != "São Paulo" {
			t.Fatalf("sep %q: %+v", sep, p)
		}
	}
}

func TestCSVBOMAspasELinhasVazias(t *testing.T) {
	conteudo := "\ufeffnome;telefone;obs\n\"Silva; João\";\"(11) 98888-7777\";\"disse \"\"oi\"\"\"\n;;\n\nMaria;21 97777-6666;\n"
	p, err := Ler("x.csv", strings.NewReader(conteudo))
	if err != nil {
		t.Fatal(err)
	}
	if p.Colunas[0] != "nome" {
		t.Fatalf("BOM não removido: %q", p.Colunas[0])
	}
	if len(p.Linhas) != 2 {
		t.Fatalf("linhas vazias deveriam ser ignoradas: %+v", p.Linhas)
	}
	if p.Linhas[0].Valores[0] != "Silva; João" || p.Linhas[0].Valores[2] != `disse "oi"` || p.Linhas[0].Numero != 1 {
		t.Fatalf("aspas: %+v", p.Linhas[0])
	}
	if p.Linhas[1].Numero != 4 || p.Linhas[1].Valores[0] != "Maria" {
		t.Fatalf("numeração deve seguir a posição na planilha: %+v", p.Linhas[1])
	}
}

func TestCSVLatin1(t *testing.T) {
	// "Nome;Cidade\nJoão;São Paulo" em Windows-1252
	conteudo := []byte("Nome;Cidade\nJo\xe3o;S\xe3o Paulo\n")
	p, err := Ler("x.csv", bytes.NewReader(conteudo))
	if err != nil {
		t.Fatal(err)
	}
	if p.Linhas[0].Valores[0] != "João" || p.Linhas[0].Valores[1] != "São Paulo" {
		t.Fatalf("latin-1: %+v", p.Linhas[0])
	}
}

func TestXLSXPrimeiraPlanilha(t *testing.T) {
	f := excelize.NewFile()
	f.SetSheetRow("Sheet1", "A1", &[]any{"Nome", "WhatsApp", "Empresa"})
	f.SetSheetRow("Sheet1", "A2", &[]any{"Ana", "11999990000", "ACME"})
	f.SetSheetRow("Sheet1", "A3", &[]any{"Beto", 5521988887777, ""})
	f.NewSheet("Outra")
	f.SetSheetRow("Outra", "A1", &[]any{"ignorar"})
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	p, err := Ler("leads.xlsx", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Colunas, []string{"Nome", "WhatsApp", "Empresa"}) || len(p.Linhas) != 2 {
		t.Fatalf("xlsx: %+v", p)
	}
	if p.Linhas[1].Valores[1] != "5521988887777" {
		t.Fatalf("número em célula numérica: %q", p.Linhas[1].Valores[1])
	}
}

func TestTipoNaoSuportado(t *testing.T) {
	_, err := Ler("foto.png", strings.NewReader("\x89PNG\r\n\x1a\n"))
	if !erros.Eh(err, erros.TipoNaoSuportado) {
		t.Fatalf("esperado tipo_nao_suportado: %v", err)
	}
	_, err = Ler("vazio.csv", strings.NewReader(""))
	if err == nil {
		t.Fatal("planilha vazia deveria falhar")
	}
	var e *erros.Erro
	if !errors.As(err, &e) {
		t.Fatalf("erro de domínio esperado: %v", err)
	}
}

func TestSugestoes(t *testing.T) {
	casos := []struct {
		colunas   []string
		tel, nome string
	}{
		{[]string{"Nome", "Celular", "Cidade"}, "Celular", "Nome"},
		{[]string{"Cliente", "WhatsApp"}, "WhatsApp", "Cliente"},
		{[]string{"name", "phone"}, "phone", "name"},
		{[]string{"Nome completo", "Telefone (WhatsApp)"}, "Telefone (WhatsApp)", "Nome completo"},
		{[]string{"Fone", "Empresa"}, "Fone", ""},
		{[]string{"a", "b"}, "", ""},
	}
	for _, c := range casos {
		tel, nome := SugerirColunas(c.colunas)
		if tel != c.tel || nome != c.nome {
			t.Errorf("%v: (%q, %q), esperado (%q, %q)", c.colunas, tel, nome, c.tel, c.nome)
		}
	}
}

func TestNormalizarChave(t *testing.T) {
	casos := map[string]string{
		"Cidade":           "cidade",
		"Razão Social":     "razao_social",
		"  Data de Nasc. ": "data_de_nasc",
		"E-mail":           "e_mail",
		"ÁREA  de atuação": "area_de_atuacao",
	}
	for entrada, esperado := range casos {
		if v := NormalizarChave(entrada); v != esperado {
			t.Errorf("NormalizarChave(%q) = %q; esperado %q", entrada, v, esperado)
		}
	}
}
