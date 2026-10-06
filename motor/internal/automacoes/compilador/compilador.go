// Pacote compilador compila os projetos TypeScript das automações de IA com o esbuild embutido
// (github.com/evanw/esbuild/pkg/api; research.md › R3) e valida o manifesto automacao.json.
// Imports permitidos (R6, camada 1): "@zapdesk/automacao" (externo, servido pelo runner), arquivos
// relativos dentro da pasta do projeto e os módulos node:crypto, node:util, node:url, node:buffer,
// node:events, node:timers/promises, node:path/posix, node:string_decoder e node:querystring
// (externos). Qualquer outro vira "Módulo não permitido: <nome>".
package compilador

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/evanw/esbuild/pkg/api"

	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/automacoes/projetos"
	"zapdesk/motor/internal/dominio"
)

// Versao do compilador (entra no hash das fontes).
const Versao = projetos.VersaoHash

// Tipos de ErroCompilacao.
const (
	TipoSintaxe    = "sintaxe"
	TipoImportacao = "importacao"
	TipoManifesto  = "manifesto"
	TipoResolucao  = "resolucao"
	TipoOutro      = "outro"
)

// ModuloSDK é o import da SDK.
const ModuloSDK = "@zapdesk/automacao"

// ModulosNode permitidos (externos).
var ModulosNode = map[string]bool{
	"node:crypto": true, "node:util": true, "node:url": true, "node:buffer": true, "node:events": true,
	"node:timers/promises": true, "node:path/posix": true, "node:string_decoder": true, "node:querystring": true,
}

// Resolvedor traduz os nomes do manifesto em ids (sem diferenciar maiúsculas/acentos; o chamador
// consulta o banco).
type Resolvedor interface {
	Etiqueta(ctx context.Context, nome string) (string, bool)
	Funil(ctx context.Context, nome string) (string, bool)
	Etapa(ctx context.Context, funilID, nome string) (string, bool)
}

// Resultado da compilação (ResultadoCompilacao do contrato).
type Resultado struct {
	OK        bool                     `json:"ok"`
	Erros     []dominio.ErroCompilacao `json:"erros"`
	Avisos    []dominio.ErroCompilacao `json:"avisos"`
	Hash      *string                  `json:"hash"`
	Handlers  []string                 `json:"handlers"`
	DuracaoMs int64                    `json:"duracao_ms"`

	// Manifesto com os nomes já traduzidos em ids (nil se o manifesto for inválido).
	Manifesto *modelo.Manifesto `json:"-"`
	// Bundle é o caminho absoluto do .mjs gravado (só com OK).
	Bundle     string `json:"-"`
	HashFontes string `json:"-"`
}

// handlersConhecidos na ordem do contrato.
var handlersConhecidos = []string{modelo.HandlerMensagem, modelo.HandlerAgendar, modelo.HandlerExecutar, modelo.HandlerEvento}

// reHandler detecta handlers definidos no código gerado. Heurística: o esbuild não minifica, então
// o objeto passado a definirAutomacao mantém `aoX(`/`async aoX(` (método) ou `aoX:` (propriedade).
// Um identificador homônimo usado como função ou chave em outro lugar também conta (falso positivo
// aceito: a falta real do handler aparece em execução como "Handler aoX não exportado").
var reHandler = regexp.MustCompile(`\b(aoReceberMensagem|aoAgendar|aoExecutar|aoEvento)\s*[(:]`)

// Compilar valida o manifesto e compila o projeto em `pastaProjeto`, gravando
// `<dirSaida>/<hash>.mjs` quando não há erros. Não apaga bundles antigos: o chamador usa
// LimparBundles mantendo o novo e o que estiver ativo.
func Compilar(ctx context.Context, pastaProjeto, dirSaida string, res Resolvedor) Resultado {
	inicio := time.Now()
	r := Resultado{Erros: []dominio.ErroCompilacao{}, Avisos: []dominio.ErroCompilacao{}, Handlers: []string{}}
	fim := func() Resultado {
		r.DuracaoMs = time.Since(inicio).Milliseconds()
		r.OK = len(r.Erros) == 0
		if !r.OK {
			r.Hash, r.Bundle = nil, ""
		}
		return r
	}

	hash, err := projetos.HashPasta(pastaProjeto)
	if err != nil {
		r.Erros = append(r.Erros, erroSimples("", "Não foi possível ler o projeto: "+err.Error(), TipoOutro))
		return fim()
	}
	r.HashFontes = hash

	man, ok := lerManifesto(ctx, pastaProjeto, res, &r)
	if !ok {
		return fim()
	}
	r.Manifesto = man
	entrada := man.ArquivoEntrada()
	if st, err := os.Stat(filepath.Join(pastaProjeto, filepath.FromSlash(entrada))); err != nil || st.IsDir() {
		r.Erros = append(r.Erros, dominio.ErroCompilacao{Arquivo: projetos.ArquivoManif, Linha: linhaDaChave(pastaProjeto, "entrada"),
			Coluna: 1, Mensagem: fmt.Sprintf("Arquivo de entrada %q não encontrado.", entrada), Tipo: TipoManifesto})
		return fim()
	}

	absProj, _ := filepath.Abs(pastaProjeto)
	if real, err := filepath.EvalSymlinks(absProj); err == nil {
		absProj = real // o esbuild entrega ResolveDir já sem links simbólicos (ex.: /var → /private/var)
	}
	saida := api.Build(api.BuildOptions{
		EntryPoints:   []string{"./" + entrada},
		Bundle:        true,
		Write:         false,
		Platform:      api.PlatformNode,
		Format:        api.FormatESModule,
		Target:        api.ES2022,
		Sourcemap:     api.SourceMapInline,
		AbsWorkingDir: absProj,
		Outfile:       filepath.Join(absProj, hash+".mjs"), // só para os caminhos do sourcemap (sources = "index.ts")
		LogLevel:      api.LogLevelSilent,
		TsconfigRaw:   `{"compilerOptions":{}}`,
		Loader: map[string]api.Loader{
			".ts": api.LoaderTS, ".json": api.LoaderJSON, ".md": api.LoaderText, ".txt": api.LoaderText,
		},
		ResolveExtensions: []string{".ts", ".json"},
		Plugins:           []api.Plugin{pluginImports(absProj)},
	})
	for _, m := range saida.Errors {
		r.Erros = append(r.Erros, converter(m, false))
	}
	for _, m := range saida.Warnings {
		r.Avisos = append(r.Avisos, converter(m, true))
	}
	if len(r.Erros) > 0 {
		return fim()
	}
	var codigo []byte
	for _, f := range saida.OutputFiles {
		if strings.HasSuffix(f.Path, ".mjs") {
			codigo = f.Contents
		}
	}
	if codigo == nil {
		r.Erros = append(r.Erros, erroSimples(entrada, "O esbuild não gerou o bundle.", TipoOutro))
		return fim()
	}

	// Handlers definidos e exigidos pelos gatilhos.
	semSourcemap := string(codigo)
	if i := strings.LastIndex(semSourcemap, "//# sourceMappingURL="); i >= 0 {
		semSourcemap = semSourcemap[:i]
	}
	achados := map[string]bool{}
	for _, m := range reHandler.FindAllStringSubmatch(semSourcemap, -1) {
		achados[m[1]] = true
	}
	for _, h := range handlersConhecidos {
		if achados[h] {
			r.Handlers = append(r.Handlers, h)
		}
	}
	for i, g := range man.Gatilhos {
		h := modelo.HandlerDoGatilho(g.Tipo)
		if h == "" || h == modelo.HandlerExecutar || achados[h] {
			continue
		}
		r.Erros = append(r.Erros, dominio.ErroCompilacao{Arquivo: projetos.ArquivoManif, Linha: linhaDaChave(pastaProjeto, "gatilhos"),
			Coluna: 1, Mensagem: fmt.Sprintf("Handler %s exigido pelo gatilho %s (gatilhos[%d]) não exportado em %s.", h, g.Tipo, i, entrada),
			Tipo: TipoManifesto})
	}
	if len(r.Erros) > 0 {
		return fim()
	}

	if err := os.MkdirAll(dirSaida, 0o700); err != nil {
		r.Erros = append(r.Erros, erroSimples("", "Não foi possível gravar o bundle: "+err.Error(), TipoOutro))
		return fim()
	}
	destino := filepath.Join(dirSaida, hash+".mjs")
	tmp := destino + ".tmp"
	if err := os.WriteFile(tmp, codigo, 0o600); err != nil || os.Rename(tmp, destino) != nil {
		os.Remove(tmp)
		r.Erros = append(r.Erros, erroSimples("", "Não foi possível gravar o bundle.", TipoOutro))
		return fim()
	}
	abs, _ := filepath.Abs(destino)
	r.Bundle = abs
	r.Hash = &hash
	return fim()
}

func erroSimples(arquivo, msg, tipo string) dominio.ErroCompilacao {
	return dominio.ErroCompilacao{Arquivo: arquivo, Linha: 1, Coluna: 1, Mensagem: msg, Tipo: tipo}
}

// lerManifesto decodifica, valida e resolve os nomes do automacao.json.
func lerManifesto(ctx context.Context, pasta string, res Resolvedor, r *Resultado) (*modelo.Manifesto, bool) {
	dados, err := os.ReadFile(filepath.Join(pasta, projetos.ArquivoManif))
	if err != nil {
		r.Erros = append(r.Erros, erroSimples(projetos.ArquivoManif, "automacao.json não encontrado.", TipoManifesto))
		return nil, false
	}
	m, errs := modelo.DecodificarManifesto(dados)
	if errs == nil {
		errs = modelo.ValidarManifesto(m)
	}
	texto := string(dados)
	for _, e := range errs {
		r.Erros = append(r.Erros, dominio.ErroCompilacao{Arquivo: projetos.ArquivoManif, Linha: linhaDoCaminho(texto, e.Caminho),
			Coluna: 1, Mensagem: e.Caminho + ": " + e.Mensagem, Tipo: TipoManifesto})
	}
	if len(errs) > 0 {
		return nil, false
	}
	for i := range m.Gatilhos {
		g := &m.Gatilhos[i]
		caminho := fmt.Sprintf("gatilhos[%d]", i)
		falha := func(msg string) {
			r.Erros = append(r.Erros, dominio.ErroCompilacao{Arquivo: projetos.ArquivoManif, Linha: linhaDoCaminho(texto, "gatilhos"),
				Coluna: 1, Mensagem: caminho + ": " + msg, Tipo: TipoManifesto})
		}
		switch g.Tipo {
		case modelo.GatilhoEtiqueta:
			if g.EtiquetaID == "" && g.Etiqueta != "" {
				if id, ok := resolver(res, func() (string, bool) { return res.Etiqueta(ctx, g.Etiqueta) }); ok {
					g.EtiquetaID, g.Etiqueta = id, ""
				} else {
					falha(fmt.Sprintf("Etiqueta '%s' não encontrada.", g.Etiqueta))
				}
			}
		case modelo.GatilhoEntrouEtapa:
			if g.FunilID == "" && g.Funil != "" {
				if id, ok := resolver(res, func() (string, bool) { return res.Funil(ctx, g.Funil) }); ok {
					g.FunilID, g.Funil = id, ""
				} else {
					falha(fmt.Sprintf("Funil '%s' não encontrado.", g.Funil))
					continue
				}
			}
			if g.EtapaID == "" && g.Etapa != "" {
				if id, ok := resolver(res, func() (string, bool) { return res.Etapa(ctx, g.FunilID, g.Etapa) }); ok {
					g.EtapaID, g.Etapa = id, ""
				} else {
					falha(fmt.Sprintf("Etapa '%s' não encontrada no funil.", g.Etapa))
				}
			}
		}
	}
	return &m, len(r.Erros) == 0
}

func resolver(res Resolvedor, f func() (string, bool)) (string, bool) {
	if res == nil {
		return "", false
	}
	return f()
}

// linhaDoCaminho acha (aproximadamente) a linha da última chave do caminho no JSON.
func linhaDoCaminho(texto, caminho string) int {
	segs := strings.FieldsFunc(caminho, func(r rune) bool { return r == '.' || r == '[' || r == ']' })
	for i := len(segs) - 1; i >= 0; i-- {
		if _, err := fmt.Sscanf(segs[i], "%d", new(int)); err == nil {
			continue
		}
		if n := linhaDe(texto, `"`+segs[i]+`"`); n > 0 {
			return n
		}
	}
	return 1
}

func linhaDaChave(pasta, chave string) int {
	dados, _ := os.ReadFile(filepath.Join(pasta, projetos.ArquivoManif))
	if n := linhaDe(string(dados), `"`+chave+`"`); n > 0 {
		return n
	}
	return 1
}

func linhaDe(texto, trecho string) int {
	i := strings.Index(texto, trecho)
	if i < 0 {
		return 0
	}
	return strings.Count(texto[:i], "\n") + 1
}

const nomePlugin = "zapdesk-imports"

// pluginImports aplica a lista de módulos permitidos.
func pluginImports(raiz string) api.Plugin {
	return api.Plugin{Name: nomePlugin, Setup: func(b api.PluginBuild) {
		b.OnResolve(api.OnResolveOptions{Filter: ".*"}, func(a api.OnResolveArgs) (api.OnResolveResult, error) {
			if a.Kind == api.ResolveEntryPoint {
				return api.OnResolveResult{}, nil
			}
			p := a.Path
			switch {
			case p == ModuloSDK:
				return api.OnResolveResult{Path: p, External: true}, nil
			case ModulosNode[p]:
				return api.OnResolveResult{Path: p, External: true}, nil
			case strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../") || p == "." || p == "..":
				alvo := filepath.Clean(filepath.Join(a.ResolveDir, filepath.FromSlash(p)))
				if alvo != raiz && !strings.HasPrefix(alvo, raiz+string(filepath.Separator)) {
					return erroImport("Import fora da pasta do projeto: " + p), nil
				}
				rel, _ := filepath.Rel(raiz, alvo)
				if strings.HasPrefix(filepath.ToSlash(rel), ".") && rel != "." {
					return erroImport("Import de arquivo reservado: " + p), nil
				}
				return api.OnResolveResult{}, nil // resolução padrão do esbuild
			case filepath.IsAbs(p) || strings.HasPrefix(p, "/"):
				return erroImport("Import fora da pasta do projeto: " + p), nil
			}
			return erroImport("Módulo não permitido: " + p), nil
		})
	}}
}

func erroImport(msg string) api.OnResolveResult {
	return api.OnResolveResult{Errors: []api.Message{{Text: msg}}}
}

// converter traduz uma mensagem do esbuild.
func converter(m api.Message, aviso bool) dominio.ErroCompilacao {
	e := dominio.ErroCompilacao{Linha: 1, Coluna: 1, Mensagem: m.Text, Tipo: TipoOutro}
	if m.Location != nil {
		e.Arquivo = filepath.ToSlash(m.Location.File)
		e.Linha = m.Location.Line
		e.Coluna = m.Location.Column + 1
		if e.Linha < 1 {
			e.Linha = 1
		}
	}
	switch {
	case m.PluginName == nomePlugin || strings.HasPrefix(m.Text, "Módulo não permitido") || strings.HasPrefix(m.Text, "Import "):
		e.Tipo = TipoImportacao
	case strings.HasPrefix(m.Text, "Could not resolve"):
		e.Tipo = TipoResolucao
		e.Mensagem = strings.Replace(m.Text, "Could not resolve", "Arquivo não encontrado:", 1)
	case m.Location != nil && !aviso:
		e.Tipo = TipoSintaxe
	}
	return e
}

// LimparBundles apaga os .mjs de `dirSaida` exceto os caminhos (ou nomes) em `manter`.
func LimparBundles(dirSaida string, manter ...string) error {
	fica := map[string]bool{}
	for _, m := range manter {
		if m != "" {
			fica[filepath.Base(m)] = true
		}
	}
	entradas, err := os.ReadDir(dirSaida)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entradas {
		if e.IsDir() || fica[e.Name()] {
			continue
		}
		if strings.HasSuffix(e.Name(), ".mjs") || strings.HasSuffix(e.Name(), ".tmp") {
			os.Remove(filepath.Join(dirSaida, e.Name()))
		}
	}
	return nil
}
