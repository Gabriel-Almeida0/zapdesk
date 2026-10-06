package compilador

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"zapdesk/motor/internal/dominio"
)

const manifestoBase = `{"versao_manifesto":1,"nome":"Teste","gatilhos":[{"tipo":"manual"}],"permissoes":[]}`

func projeto(t *testing.T, arquivos map[string]string) string {
	t.Helper()
	pasta := t.TempDir()
	if _, ok := arquivos["automacao.json"]; !ok {
		arquivos["automacao.json"] = manifestoBase
	}
	for c, v := range arquivos {
		d := filepath.Join(pasta, filepath.FromSlash(c))
		os.MkdirAll(filepath.Dir(d), 0o700)
		if err := os.WriteFile(d, []byte(v), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return pasta
}

func compilar(t *testing.T, pasta string, res Resolvedor) Resultado {
	t.Helper()
	return Compilar(context.Background(), pasta, filepath.Join(t.TempDir(), "saida"), res)
}

func achar(r Resultado, tipo, trecho string) *dominio.ErroCompilacao {
	for i, e := range r.Erros {
		if e.Tipo == tipo && strings.Contains(e.Mensagem, trecho) {
			return &r.Erros[i]
		}
	}
	return nil
}

const indexOK = `import { definirAutomacao } from '@zapdesk/automacao';
import { saudar } from './lib/texto';
import dados from './dados.json';
import prompt from './prompt.md';
import nota from './nota.txt';
import { createHash } from 'node:crypto';
import { setTimeout as esperar } from 'node:timers/promises';
export default definirAutomacao({
  async aoExecutar(ctx, entrada) {
    await esperar(1);
    return { s: saudar(dados.nome), p: prompt, n: nota, h: createHash('sha256').update('x').digest('hex') };
  },
});
`

func TestCompilaImportsPermitidos(t *testing.T) {
	pasta := projeto(t, map[string]string{
		"index.ts":     indexOK,
		"lib/texto.ts": "export function saudar(n: string): string { return `Oi, ${n}`; }\n",
		"dados.json":   `{"nome":"Ana"}`,
		"prompt.md":    "# Prompt\nSeja breve.",
		"nota.txt":     "nota",
	})
	r := compilar(t, pasta, nil)
	if !r.OK {
		t.Fatalf("erros: %+v", r.Erros)
	}
	codigo, err := os.ReadFile(r.Bundle)
	if err != nil {
		t.Fatal(err)
	}
	c := string(codigo)
	for _, trecho := range []string{`from "@zapdesk/automacao"`, `from "node:crypto"`, `from "node:timers/promises"`, "Seja breve.", `"Ana"`, "sourceMappingURL=data:"} {
		if !strings.Contains(c, trecho) {
			t.Errorf("bundle sem %q", trecho)
		}
	}
	if filepath.Base(r.Bundle) != *r.Hash+".mjs" || r.HashFontes != *r.Hash {
		t.Fatalf("hash/bundle: %s %s", r.Bundle, *r.Hash)
	}
	if len(r.Handlers) != 1 || r.Handlers[0] != "aoExecutar" {
		t.Fatalf("handlers: %v", r.Handlers)
	}
	if r.Manifesto == nil || r.Manifesto.Nome != "Teste" {
		t.Fatal("manifesto")
	}
}

func TestModulosNaoPermitidos(t *testing.T) {
	casos := map[string]string{
		"fs":            "import x from 'fs';\n",
		"node:fs":       "import x from 'node:fs';\n",
		"child_process": "import * as x from 'child_process';\n",
		"lodash":        "import x from 'lodash';\n",
		"crypto":        "import x from 'crypto';\n",
	}
	for nome, imp := range casos {
		pasta := projeto(t, map[string]string{"index.ts": "// comentário\n" + imp +
			"import { definirAutomacao } from '@zapdesk/automacao';\nexport default definirAutomacao({ aoExecutar() { return String(x); } });\n"})
		r := compilar(t, pasta, nil)
		e := achar(r, TipoImportacao, "Módulo não permitido: "+nome)
		if r.OK || e == nil {
			t.Fatalf("%s: %+v", nome, r.Erros)
		}
		if e.Arquivo != "index.ts" || e.Linha != 2 || e.Coluna < 1 {
			t.Fatalf("%s: posição %+v", nome, e)
		}
		// Coluna 1-base (como o Monaco e o "arquivo:linha:coluna" dos editores): em
		// `import x from 'fs'` a aspa do caminho é o 15º caractere.
		if nome == "fs" && e.Coluna != 15 {
			t.Fatalf("coluna deveria ser 1-base (15): %+v", e)
		}
	}
}

func TestImportForaDaPasta(t *testing.T) {
	pasta := projeto(t, map[string]string{"index.ts": "import x from '../fora';\nexport default x;\n"})
	r := compilar(t, pasta, nil)
	if r.OK || achar(r, TipoImportacao, "fora da pasta") == nil {
		t.Fatalf("%+v", r.Erros)
	}
	pasta = projeto(t, map[string]string{"index.ts": "import x from '/etc/passwd';\nexport default x;\n"})
	if r := compilar(t, pasta, nil); r.OK || achar(r, TipoImportacao, "fora da pasta") == nil {
		t.Fatalf("absoluto: %+v", r.Erros)
	}
	pasta = projeto(t, map[string]string{"index.ts": "import x from './nao-existe';\nexport default x;\n"})
	if r := compilar(t, pasta, nil); r.OK || achar(r, TipoResolucao, "nao-existe") == nil {
		t.Fatalf("inexistente: %+v", r.Erros)
	}
}

func TestErroDeSintaxe(t *testing.T) {
	pasta := projeto(t, map[string]string{"index.ts": "const a = 1;\nconst b = ;\n"})
	r := compilar(t, pasta, nil)
	if r.OK || len(r.Erros) == 0 || r.Erros[0].Tipo != TipoSintaxe || r.Erros[0].Arquivo != "index.ts" || r.Erros[0].Linha != 2 {
		t.Fatalf("%+v", r.Erros)
	}
	if r.Hash != nil || r.Bundle != "" {
		t.Fatal("sem bundle com erro")
	}
}

func TestManifestoInvalidoEHandlerAusente(t *testing.T) {
	pasta := projeto(t, map[string]string{"index.ts": "export default {};\n",
		"automacao.json": `{"versao_manifesto":1,"nome":"","gatilhos":[],"permissoes":["voar"]}`})
	r := compilar(t, pasta, nil)
	if r.OK || achar(r, TipoManifesto, "nome") == nil || achar(r, TipoManifesto, "permissoes[0]") == nil {
		t.Fatalf("manifesto: %+v", r.Erros)
	}
	for _, e := range r.Erros {
		if e.Arquivo != "automacao.json" {
			t.Fatalf("arquivo: %+v", e)
		}
	}
	pasta = projeto(t, map[string]string{"automacao.json": `{`, "index.ts": ""})
	if r := compilar(t, pasta, nil); r.OK || r.Erros[0].Tipo != TipoManifesto {
		t.Fatalf("json quebrado: %+v", r.Erros)
	}

	pasta = projeto(t, map[string]string{
		"automacao.json": `{"versao_manifesto":1,"nome":"x","gatilhos":[{"tipo":"mensagem_recebida"},{"tipo":"etiqueta","evento":"adicionada","etiqueta_id":"e1"},{"tipo":"manual"}],"permissoes":[]}`,
		"index.ts":       "import { definirAutomacao } from '@zapdesk/automacao';\nexport default definirAutomacao({ async aoReceberMensagem(ctx, m) {} });\n",
	})
	r = compilar(t, pasta, nil)
	if r.OK || achar(r, TipoManifesto, "Handler aoEvento") == nil || achar(r, TipoManifesto, "aoExecutar") != nil {
		t.Fatalf("handler: %+v", r.Erros)
	}
}

type resolvedor struct{}

func (resolvedor) Etiqueta(_ context.Context, n string) (string, bool) {
	return "e-" + strings.ToLower(n), n == "Quente"
}
func (resolvedor) Funil(_ context.Context, n string) (string, bool) { return "f1", n == "Vendas" }
func (resolvedor) Etapa(_ context.Context, f, n string) (string, bool) {
	return "s-" + f, n == "Novo"
}

func TestNomesResolvidos(t *testing.T) {
	idx := "import { definirAutomacao } from '@zapdesk/automacao';\nexport default definirAutomacao({ aoEvento: async () => {} });\n"
	pasta := projeto(t, map[string]string{"index.ts": idx,
		"automacao.json": `{"versao_manifesto":1,"nome":"x","gatilhos":[{"tipo":"etiqueta","evento":"adicionada","etiqueta":"Quente"},{"tipo":"entrou_etapa","funil":"Vendas","etapa":"Novo"}],"permissoes":[]}`})
	r := compilar(t, pasta, resolvedor{})
	if !r.OK {
		t.Fatalf("%+v", r.Erros)
	}
	g := r.Manifesto.Gatilhos
	if g[0].EtiquetaID != "e-quente" || g[0].Etiqueta != "" || g[1].FunilID != "f1" || g[1].EtapaID != "s-f1" || g[1].Funil != "" {
		t.Fatalf("resolvidos: %+v", g)
	}
	pasta = projeto(t, map[string]string{"index.ts": idx,
		"automacao.json": `{"versao_manifesto":1,"nome":"x","gatilhos":[{"tipo":"etiqueta","evento":"adicionada","etiqueta":"Fria"}],"permissoes":[]}`})
	if r := compilar(t, pasta, nil); r.OK || achar(r, TipoManifesto, "Etiqueta 'Fria' não encontrada") == nil {
		t.Fatalf("nome inexistente: %+v", r.Erros)
	}
}

func TestHashEstavelELimpeza(t *testing.T) {
	arqs := map[string]string{"index.ts": indexOK, "lib/texto.ts": "export const saudar = (n: string) => n;\n",
		"dados.json": `{"nome":"A"}`, "prompt.md": "p", "nota.txt": "n"}
	pasta := projeto(t, arqs)
	saida := filepath.Join(t.TempDir(), "saida")
	r1 := Compilar(context.Background(), pasta, saida, nil)
	r2 := Compilar(context.Background(), pasta, saida, nil)
	if !r1.OK || *r1.Hash != *r2.Hash {
		t.Fatalf("hash instável: %v %v", r1.Hash, r2.Hash)
	}
	os.WriteFile(filepath.Join(pasta, "nota.txt"), []byte("outra"), 0o600)
	r3 := Compilar(context.Background(), pasta, saida, nil)
	if *r3.Hash == *r1.Hash {
		t.Fatal("hash não mudou com o conteúdo")
	}
	// Arquivos gerados não entram no hash.
	os.MkdirAll(filepath.Join(pasta, ".zapdesk"), 0o700)
	os.WriteFile(filepath.Join(pasta, ".zapdesk", "x.d.ts"), []byte("x"), 0o600)
	os.WriteFile(filepath.Join(pasta, "tsconfig.json"), []byte("{}"), 0o600)
	if r4 := Compilar(context.Background(), pasta, saida, nil); *r4.Hash != *r3.Hash {
		t.Fatal("gerados mudaram o hash")
	}
	LimparBundles(saida, r3.Bundle)
	ents, _ := os.ReadDir(saida)
	if len(ents) != 1 || ents[0].Name() != *r3.Hash+".mjs" {
		t.Fatalf("limpeza: %v", ents)
	}
}

func TestDezArquivosEmMenosDeUmSegundo(t *testing.T) {
	arqs := map[string]string{}
	imports := ""
	uso := ""
	for i := 0; i < 9; i++ {
		arqs[fmt.Sprintf("mod%d.ts", i)] = fmt.Sprintf("export function f%d(x: number): number {\n  return x * %d;\n}\n", i, i+1)
		imports += fmt.Sprintf("import { f%d } from './mod%d';\n", i, i)
		uso += fmt.Sprintf(" + f%d(1)", i)
	}
	arqs["index.ts"] = imports + "import { definirAutomacao } from '@zapdesk/automacao';\nexport default definirAutomacao({ aoExecutar() { return 0" + uso + "; } });\n"
	pasta := projeto(t, arqs)
	inicio := time.Now()
	r := compilar(t, pasta, nil)
	if !r.OK || time.Since(inicio) > time.Second {
		t.Fatalf("ok=%v em %v: %+v", r.OK, time.Since(inicio), r.Erros)
	}
}
