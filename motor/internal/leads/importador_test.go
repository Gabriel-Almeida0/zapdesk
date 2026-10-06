package leads

import (
	"context"
	"reflect"
	"testing"
	"time"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/relogio"
)

var ctx = context.Background()

func novoImportador(t *testing.T) (*Importador, *armazenamento.Banco, *relogio.Controlavel) {
	t.Helper()
	b, err := armazenamento.Abrir(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Fechar() })
	if _, err := b.Migrar(ctx, armazenamento.MigracoesPadrao()); err != nil {
		t.Fatal(err)
	}
	r := relogio.NovoCongelado(time.Date(2026, 9, 1, 12, 0, 0, 0, time.Local))
	return NovoImportador(b, r), b, r
}

func conferirInvariante(t *testing.T, rel dominio.RelatorioImportacao) {
	t.Helper()
	soma := len(rel.Novos) + len(rel.JaExistentes) + len(rel.Invalidos) + len(rel.DuplicadosNoLote)
	if soma != rel.TotalLinhas {
		t.Fatalf("invariante quebrada: %d != %d", soma, rel.TotalLinhas)
	}
	if rel.TotalNovos != len(rel.Novos) || rel.TotalJaExistentes != len(rel.JaExistentes) ||
		rel.TotalInvalidos != len(rel.Invalidos) || rel.TotalDuplicadosNoLote != len(rel.DuplicadosNoLote) {
		t.Fatal("totais não batem com as listas")
	}
	if len(rel.LeadIDs) != len(rel.Novos)+len(rel.JaExistentes) {
		t.Fatal("lead_ids deve ter novos + já existentes")
	}
}

func TestDuplicadosNoLotePrimeiraVence(t *testing.T) {
	imp, _, _ := novoImportador(t)
	rel, err := imp.Importar(ctx, []Linha{
		{Numero: 1, Telefone: "(11) 99999-0000", Nome: "Primeira"},
		{Numero: 2, Telefone: "+55 11 99999-0000", Nome: "Segunda"},
		{Numero: 3, Telefone: "21 98888-7777"},
		{Numero: 4, Telefone: "5511999990000"},
	}, dominio.OrigemColado, "")
	if err != nil {
		t.Fatal(err)
	}
	conferirInvariante(t, rel)
	if rel.TotalNovos != 2 || rel.TotalDuplicadosNoLote != 2 {
		t.Fatalf("relatório: %+v", rel)
	}
	d := rel.DuplicadosNoLote
	if d[0].Linha != 2 || d[0].PrimeiraLinha != 1 || d[0].Telefone != "+5511999990000" || d[1].Linha != 4 || d[1].PrimeiraLinha != 1 {
		t.Fatalf("duplicados: %+v", d)
	}
	l, err := armazenamento.LeadPorTelefone(ctx, imp.banco.L(), "+5511999990000")
	if err != nil || l.Nome == nil || *l.Nome != "Primeira" {
		t.Fatalf("primeira ocorrência deve vencer: %+v %v", l, err)
	}
}

func TestJaExistentePreencheSoVazios(t *testing.T) {
	imp, _, r := novoImportador(t)
	rel1, err := imp.Importar(ctx, []Linha{
		{Numero: 1, Telefone: "11999990000", Campos: map[string]string{"empresa": "ACME", "cidade": ""}},
	}, dominio.OrigemCSV, "")
	if err != nil {
		t.Fatal(err)
	}
	original := rel1.Novos[0]
	importadoEm := r.Agora()

	r.Avancar(48 * time.Hour)
	rel2, err := imp.Importar(ctx, []Linha{
		{Numero: 1, Telefone: "+5511999990000", Nome: "Ana", Campos: map[string]string{"empresa": "Outra", "cidade": "Recife", "cargo": "CEO"}},
	}, dominio.OrigemMCP, "")
	if err != nil {
		t.Fatal(err)
	}
	conferirInvariante(t, rel2)
	if rel2.TotalJaExistentes != 1 || rel2.TotalNovos != 0 {
		t.Fatalf("relatório: %+v", rel2)
	}
	je := rel2.JaExistentes[0]
	if je.LeadID != original.LeadID || !je.ImportadoEm.Equal(importadoEm) || je.Linha != 1 {
		t.Fatalf("já existente: %+v", je)
	}
	if !reflect.DeepEqual(je.CamposPreenchidos, []string{"cargo", "cidade", "nome"}) {
		t.Fatalf("campos preenchidos: %v", je.CamposPreenchidos)
	}
	l, _ := armazenamento.ObterLead(ctx, imp.banco.L(), original.LeadID)
	if *l.Nome != "Ana" || l.Campos["empresa"] != "ACME" || l.Campos["cidade"] != "Recife" || l.Campos["cargo"] != "CEO" {
		t.Fatalf("nunca sobrescreve / preenche vazios: %+v", l)
	}
	if l.Origem != dominio.OrigemCSV || !l.ImportadoEm.Equal(importadoEm) {
		t.Fatalf("origem e importado_em devem ficar intactos: %+v", l)
	}

	// terceira importação: nada novo a preencher
	rel3, _ := imp.Importar(ctx, []Linha{{Numero: 1, Telefone: "11 99999-0000", Nome: "Outro nome"}}, dominio.OrigemColado, "")
	if len(rel3.JaExistentes[0].CamposPreenchidos) != 0 {
		t.Fatalf("não deveria preencher nada: %v", rel3.JaExistentes[0].CamposPreenchidos)
	}
}

func TestInvalidosComMotivo(t *testing.T) {
	imp, _, _ := novoImportador(t)
	rel, err := imp.Importar(ctx, []Linha{
		{Numero: 1, Telefone: ""},
		{Numero: 2, Telefone: "abc"},
		{Numero: 3, Telefone: "+55 11 1234"},
		{Numero: 4, Telefone: "11 99999-0000"},
	}, dominio.OrigemColado, "")
	if err != nil {
		t.Fatal(err)
	}
	conferirInvariante(t, rel)
	motivos := []string{rel.Invalidos[0].Motivo, rel.Invalidos[1].Motivo, rel.Invalidos[2].Motivo}
	if !reflect.DeepEqual(motivos, []string{"vazio", "formato_invalido", "numero_invalido"}) || rel.Invalidos[1].Valor != "abc" || rel.Invalidos[2].Linha != 3 {
		t.Fatalf("inválidos: %+v", rel.Invalidos)
	}
}

func TestDDIPadrao(t *testing.T) {
	imp, _, _ := novoImportador(t)
	rel, _ := imp.Importar(ctx, []Linha{{Numero: 1, Telefone: "415 555 2671"}}, dominio.OrigemColado, "1")
	if rel.TotalNovos != 1 || rel.Novos[0].Telefone != "+14155552671" {
		t.Fatalf("DDI padrão: %+v", rel)
	}
}

func TestTudoNumaTransacao(t *testing.T) {
	imp, b, _ := novoImportador(t)
	// um nome longo demais (> 120) faz a gravação falhar no meio: nada pode ficar gravado
	longo := make([]byte, 200)
	for i := range longo {
		longo[i] = 'a'
	}
	_, err := imp.Importar(ctx, []Linha{
		{Numero: 1, Telefone: "11 99999-0000", Nome: "ok"},
		{Numero: 2, Telefone: "11 98888-0000", Nome: string(longo)},
	}, dominio.OrigemColado, "")
	if err != nil {
		// nome longo é truncado, não é erro; força o erro por outro caminho abaixo
	}
	var n int
	b.L().QueryRow(`SELECT count(*) FROM leads`).Scan(&n)
	if n != 2 {
		t.Fatalf("nome longo deve ser truncado e gravado: %d", n)
	}

	falhar = func(i int) bool { return i == 1 }
	defer func() { falhar = nil }()
	_, err = imp.Importar(ctx, []Linha{
		{Numero: 1, Telefone: "11 97777-0000"},
		{Numero: 2, Telefone: "11 96666-0000"},
	}, dominio.OrigemColado, "")
	if err == nil {
		t.Fatal("esperava erro simulado")
	}
	b.L().QueryRow(`SELECT count(*) FROM leads`).Scan(&n)
	if n != 2 {
		t.Fatalf("importação com erro deve ser desfeita por inteiro: %d leads", n)
	}
}

func TestLigaContatoAoLead(t *testing.T) {
	imp, b, r := novoImportador(t)
	agora := r.Agora()
	armazenamento.CriarConta(ctx, b.E(), dominio.Conta{ID: "c1", Nome: "Conta", Estado: "conectada", CriadaEm: agora})
	cid, _, _ := armazenamento.GarantirContato(ctx, b.E(), "c1", armazenamento.DadosContato{JID: "5511999990000@s.whatsapp.net", Telefone: "+5511999990000"}, agora)
	rel, _ := imp.Importar(ctx, []Linha{{Numero: 1, Telefone: "11999990000"}}, dominio.OrigemColado, "")
	c, _ := armazenamento.ObterContato(ctx, b.L(), cid)
	if c.Lead == nil || c.Lead.ID != rel.Novos[0].LeadID {
		t.Fatalf("contato não ligado ao lead: %+v", c.Lead)
	}
}
