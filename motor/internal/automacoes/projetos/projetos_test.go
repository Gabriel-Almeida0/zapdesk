package projetos_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"zapdesk/motor/internal/automacoes/compilador"
	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/automacoes/projetos"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/relogio"
)

func novo(t *testing.T) (*projetos.Projetos, string) {
	t.Helper()
	pasta := t.TempDir()
	return projetos.Novo(pasta, relogio.NovoCongelado(time.Now())), pasta
}

func codigo(err error) string {
	if e, ok := erros.Como(err); ok {
		return e.Codigo
	}
	return fmt.Sprint(err)
}

func TestValidarCaminho(t *testing.T) {
	validos := []string{"index.ts", "lib/util.ts", "a/b/c/d.json", "prompt.md", "notas_v2.txt", "x-y.z.ts"}
	for _, c := range validos {
		if err := projetos.ValidarCaminho(c); err != nil {
			t.Errorf("%q deveria valer: %v", c, err)
		}
	}
	invalidos := []string{"", "/abs.ts", "../x.ts", "a/../x.ts", ".oculto.ts", "a/.b/c.ts", "a/b/c/d/e.ts", "x.js",
		"x.tsx", `a\b.ts`, "com espaço.ts", strings.Repeat("a", 62) + ".ts", "tsconfig.json", "./x.ts", "a//b.ts"}
	for _, c := range invalidos {
		if err := projetos.ValidarCaminho(c); codigo(err) != erros.Validacao {
			t.Errorf("%q deveria falhar: %v", c, err)
		}
	}
}

func TestCriarModelosEArquivosGerados(t *testing.T) {
	p, pasta := novo(t)
	desc := "Minha descrição"
	if err := p.Criar("a1", "responder_historico", "Meu robô", &desc); err != nil {
		t.Fatal(err)
	}
	if err := p.Criar("a2", "inexistente", "x", nil); codigo(err) != erros.Validacao {
		t.Fatalf("modelo inválido: %v", err)
	}
	lista, err := p.Listar("a1")
	if err != nil {
		t.Fatal(err)
	}
	var nomes []string
	for _, a := range lista {
		nomes = append(nomes, a.Caminho)
		if len(a.Hash) != 64 || a.Tamanho == 0 {
			t.Fatalf("arquivo: %+v", a)
		}
	}
	if strings.Join(nomes, ",") != "automacao.json,index.ts,prompt.md" {
		t.Fatalf("arquivos visíveis: %v", nomes)
	}
	base := filepath.Join(pasta, "automacoes", "a1")
	for _, g := range []string{".zapdesk/automacao.d.ts", ".zapdesk/automacao.schema.json", "tsconfig.json"} {
		if _, err := os.Stat(filepath.Join(base, g)); err != nil {
			t.Fatalf("gerado ausente: %s", g)
		}
	}
	var tsc map[string]any
	b, _ := os.ReadFile(filepath.Join(base, "tsconfig.json"))
	json.Unmarshal(b, &tsc)
	if !strings.Contains(string(b), `"@zapdesk/automacao"`) || !strings.Contains(string(b), "./.zapdesk/automacao.d.ts") {
		t.Fatalf("tsconfig: %s", b)
	}
	c, err := p.Ler("a1", "automacao.json")
	if err != nil {
		t.Fatal(err)
	}
	m, errs := modelo.DecodificarManifesto([]byte(c.Conteudo))
	if errs != nil || m.Nome != "Meu robô" || m.Descricao == nil || *m.Descricao != desc || m.Schema != "./.zapdesk/automacao.schema.json" {
		t.Fatalf("manifesto: %+v %v", m, errs)
	}
	if _, err := p.Ler("a1", ".zapdesk/automacao.d.ts"); codigo(err) != erros.NaoEncontrado {
		t.Fatal("gerado não pode ser lido pela API")
	}
	if _, err := p.Ler("a1", "nada.ts"); codigo(err) != erros.NaoEncontrado {
		t.Fatal("inexistente")
	}
	if _, err := p.Listar("zz"); codigo(err) != erros.NaoEncontrado {
		t.Fatal("projeto inexistente")
	}
	if len(projetos.Modelos()) != 4 || projetos.TiposSDK() == "" || projetos.VersaoSDK() == "" {
		t.Fatal("modelos/sdk")
	}
	if e := projetos.EsquemaManifesto(); e["$schema"] == nil || e["properties"] == nil {
		t.Fatal("esquema")
	}
}

func TestEscreverConflitoELimites(t *testing.T) {
	p, _ := novo(t)
	p.Criar("a", "em_branco", "x", nil)
	a, criado, err := p.Escrever("a", "lib/util.ts", "export const x = 1;\n", true, nil)
	if err != nil || !criado || a.Caminho != "lib/util.ts" {
		t.Fatalf("novo: %+v %v %v", a, criado, err)
	}
	// Deve ser novo, mas já existe.
	if _, _, err := p.Escrever("a", "lib/util.ts", "y", true, nil); codigo(err) != erros.Conflito {
		t.Fatalf("hash_anterior null com arquivo existente: %v", err)
	}
	errado := "abc"
	_, _, err = p.Escrever("a", "lib/util.ts", "y", true, &errado)
	if e, _ := erros.Como(err); e == nil || e.Codigo != erros.Conflito || e.Detalhes["hash_atual"] != a.Hash || e.Mensagem != "O arquivo foi alterado fora do app." {
		t.Fatalf("hash divergente: %v", err)
	}
	a2, criado, err := p.Escrever("a", "lib/util.ts", "export const x = 2;\n", true, &a.Hash)
	if err != nil || criado || a2.Hash == a.Hash {
		t.Fatalf("hash certo: %v", err)
	}
	if _, _, err := p.Escrever("a", "lib/util.ts", "livre", false, nil); err != nil {
		t.Fatalf("sem verificação: %v", err)
	}
	if _, _, err := p.Escrever("a", "grande.md", strings.Repeat("x", projetos.MaxTamanho+1), false, nil); codigo(err) != erros.Validacao {
		t.Fatal("1 MB")
	}
	if _, _, err := p.Escrever("a", "x.js", "", false, nil); codigo(err) != erros.Validacao {
		t.Fatal("extensão")
	}
	// Até 50 arquivos (já há automacao.json, index.ts e lib/util.ts).
	for i := 0; i < projetos.MaxArquivos-3; i++ {
		if _, _, err := p.Escrever("a", fmt.Sprintf("m/f%d.txt", i), "x", false, nil); err != nil {
			t.Fatalf("arquivo %d: %v", i, err)
		}
	}
	if _, _, err := p.Escrever("a", "um-a-mais.txt", "x", false, nil); codigo(err) != erros.Validacao {
		t.Fatalf("51º arquivo: %v", err)
	}
	// Sobrescrever um existente ainda é permitido no limite.
	if _, _, err := p.Escrever("a", "m/f0.txt", "y", false, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExcluirRenomearEProtegidos(t *testing.T) {
	p, pasta := novo(t)
	p.Criar("a", "em_branco", "x", nil)
	for _, c := range []string{"automacao.json", "index.ts"} {
		if err := p.Excluir("a", c); codigo(err) != erros.Validacao {
			t.Fatalf("excluir %s: %v", c, err)
		}
		if _, err := p.Renomear("a", c, "outro.ts"); codigo(err) != erros.Validacao {
			t.Fatalf("renomear %s: %v", c, err)
		}
	}
	p.Escrever("a", "lib/a.ts", "1", false, nil)
	p.Escrever("a", "b.ts", "2", false, nil)
	if _, err := p.Renomear("a", "lib/a.ts", "b.ts"); codigo(err) != erros.Conflito {
		t.Fatalf("destino existe: %v", err)
	}
	r, err := p.Renomear("a", "lib/a.ts", "novo/c.ts")
	if err != nil || r.Caminho != "novo/c.ts" {
		t.Fatalf("renomear: %v", err)
	}
	if _, err := os.Stat(filepath.Join(pasta, "automacoes", "a", "lib")); !os.IsNotExist(err) {
		t.Fatal("pasta vazia ficou")
	}
	if err := p.Excluir("a", "novo/c.ts"); err != nil {
		t.Fatal(err)
	}
	if err := p.Excluir("a", "novo/c.ts"); codigo(err) != erros.NaoEncontrado {
		t.Fatal("excluir de novo")
	}
	// Entrada personalizada também fica protegida.
	p.Escrever("a", "automacao.json", `{"versao_manifesto":1,"nome":"x","entrada":"b.ts","gatilhos":[{"tipo":"manual"}],"permissoes":[]}`, false, nil)
	if err := p.Excluir("a", "b.ts"); codigo(err) != erros.Validacao {
		t.Fatal("entrada personalizada")
	}
	if err := p.Excluir("a", "index.ts"); err != nil {
		t.Fatalf("index.ts deixou de ser entrada: %v", err)
	}
	os.MkdirAll(filepath.Join(pasta, "automacoes-compiladas", "a"), 0o700)
	if err := p.ApagarProjeto("a"); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"automacoes/a", "automacoes-compiladas/a"} {
		if _, err := os.Stat(filepath.Join(pasta, d)); !os.IsNotExist(err) {
			t.Fatalf("%s não apagada", d)
		}
	}
}

func TestHashFontes(t *testing.T) {
	p, _ := novo(t)
	p.Criar("a", "em_branco", "x", nil)
	h1, _ := p.HashFontes("a")
	p.AtualizarGerados("a")
	h2, _ := p.HashFontes("a")
	if h1 != h2 || len(h1) != 64 {
		t.Fatal("gerados mudaram o hash")
	}
	p.Escrever("a", "n.txt", "x", false, nil)
	if h3, _ := p.HashFontes("a"); h3 == h1 {
		t.Fatal("hash não mudou")
	}
}

func TestOsQuatroModelosCompilam(t *testing.T) {
	p, pasta := novo(t)
	for _, m := range projetos.Modelos() {
		if err := p.Criar(m.ID, m.ID, "Modelo "+m.ID, nil); err != nil {
			t.Fatal(err)
		}
		r := compilador.Compilar(context.Background(), p.Pasta(m.ID), filepath.Join(pasta, projetos.DirCompiladas, m.ID), nil)
		if !r.OK {
			t.Fatalf("%s: %+v", m.ID, r.Erros)
		}
		if len(r.Handlers) == 0 {
			t.Fatalf("%s sem handlers", m.ID)
		}
		h, _ := p.HashFontes(m.ID)
		if *r.Hash != h {
			t.Fatalf("%s: hash do compilador ≠ HashFontes", m.ID)
		}
	}
}
