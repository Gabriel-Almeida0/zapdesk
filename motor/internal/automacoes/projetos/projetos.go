// Pacote projetos guarda os arquivos das automações de IA na pasta de dados
// (<pasta-dados>/automacoes/<id>/; data-model.md › Armazenamento em arquivos; research.md › R14):
// validação de caminhos e limites, conflito por hash, arquivos gerados (.zapdesk/ e
// tsconfig.json, ocultos na API), modelos de projeto e tipos da SDK embutidos.
package projetos

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/relogio"
)

// Pastas dentro da pasta de dados.
const (
	DirProjetos   = "automacoes"
	DirCompiladas = "automacoes-compiladas"
	DirGerados    = ".zapdesk"
	ArquivoManif  = "automacao.json"
	ArquivoTSConf = "tsconfig.json"
)

// VersaoHash entra no hash das fontes (muda quando o compilador muda de comportamento).
const VersaoHash = "esbuild-0.28.2/1"

// Limites dos projetos.
const (
	MaxArquivos     = 50
	MaxTamanho      = 1 << 20
	MaxProfundidade = 4
)

//go:embed sdk/automacao.d.ts
var tiposSDK string

//go:embed all:modelos
var modelosFS embed.FS

// Arquivo do projeto (ArquivoProjeto do contrato).
type Arquivo struct {
	Caminho      string    `json:"caminho"`
	Tamanho      int64     `json:"tamanho"`
	Hash         string    `json:"hash"`
	AtualizadoEm time.Time `json:"atualizado_em"`
}

// Conteudo de um arquivo (ConteudoArquivo do contrato).
type Conteudo struct {
	Caminho      string    `json:"caminho"`
	Conteudo     string    `json:"conteudo"`
	Hash         string    `json:"hash"`
	AtualizadoEm time.Time `json:"atualizado_em"`
}

// Modelo de projeto (ModeloProjeto do contrato).
type Modelo struct {
	ID        string `json:"id"`
	Nome      string `json:"nome"`
	Descricao string `json:"descricao"`
}

var modelos = []Modelo{
	{ID: "responder_historico", Nome: "Responder com IA usando o histórico", Descricao: "Responde dúvidas do contato com a Claude, usando as últimas mensagens da conversa. Chama um atendente quando não sabe."},
	{ID: "classificar_funil", Nome: "Classificar lead no funil", Descricao: "Classifica o interesse do contato (quente, morno, frio), move o lead no funil e aplica uma etiqueta."},
	{ID: "extrair_dados", Nome: "Extrair dados do lead", Descricao: "Lê a conversa e grava no lead nome, e-mail, cidade e interesse informados pelo contato."},
	{ID: "em_branco", Nome: "Em branco", Descricao: "Projeto vazio com execução manual, para programar do zero."},
}

// Modelos devolve os modelos de projeto.
func Modelos() []Modelo { return append([]Modelo{}, modelos...) }

// TiposSDK devolve o .d.ts da SDK embutido.
func TiposSDK() string { return tiposSDK }

var reVersao = regexp.MustCompile(`VERSAO_SDK\s*=\s*"([^"]+)"`)

// VersaoSDK lê a versão declarada no .d.ts (padrão 0.1.0).
func VersaoSDK() string {
	if m := reVersao.FindStringSubmatch(tiposSDK); m != nil {
		return m[1]
	}
	return "0.1.0"
}

// Projetos gerencia as pastas dos projetos.
type Projetos struct {
	pastaDados string
	relogio    relogio.Relogio
}

// Novo cria o gerenciador.
func Novo(pastaDados string, r relogio.Relogio) *Projetos {
	return &Projetos{pastaDados: pastaDados, relogio: r}
}

// Pasta devolve <pasta-dados>/automacoes/<id>.
func (p *Projetos) Pasta(id string) string { return filepath.Join(p.pastaDados, DirProjetos, id) }

// PastaCompiladas devolve <pasta-dados>/automacoes-compiladas/<id>.
func (p *Projetos) PastaCompiladas(id string) string {
	return filepath.Join(p.pastaDados, DirCompiladas, id)
}

var (
	reSegmento = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)
	extensoes  = map[string]bool{".ts": true, ".json": true, ".md": true, ".txt": true}
)

// ValidarCaminho aplica as regras de caminho do projeto.
func ValidarCaminho(caminho string) error {
	falha := func(m string) error { return erros.Campo("caminho", m) }
	if caminho == "" {
		return falha("Informe o caminho do arquivo.")
	}
	if strings.Contains(caminho, `\`) || strings.HasPrefix(caminho, "/") {
		return falha("Use caminhos relativos com \"/\".")
	}
	segs := strings.Split(caminho, "/")
	if len(segs) > MaxProfundidade {
		return falha("No máximo 4 níveis de pasta.")
	}
	for _, s := range segs {
		if s == ".." || s == "." {
			return falha("O caminho não pode conter \"..\" nem \".\".")
		}
		if strings.HasPrefix(s, ".") {
			return falha("Nomes começando com \".\" são reservados.")
		}
		if !reSegmento.MatchString(s) {
			return falha("Use só letras, números, \"_\", \".\" e \"-\" (até 64 por nome).")
		}
	}
	if !extensoes[strings.ToLower(path.Ext(caminho))] {
		return falha("Extensões permitidas: .ts, .json, .md e .txt.")
	}
	if caminho == ArquivoTSConf {
		return falha("tsconfig.json é gerado pelo ZapDesk.")
	}
	return nil
}

func hashDe(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func (p *Projetos) abs(id, caminho string) string {
	return filepath.Join(p.Pasta(id), filepath.FromSlash(caminho))
}

func (p *Projetos) existeProjeto(id string) error {
	if id == "" || strings.ContainsAny(id, `/\.`) {
		return erros.NaoAchada("Automação")
	}
	if st, err := os.Stat(p.Pasta(id)); err != nil || !st.IsDir() {
		return erros.NaoAchado("Projeto da automação")
	}
	return nil
}

// Criar cria a pasta do projeto a partir de um modelo.
func (p *Projetos) Criar(id, idModelo, nome string, descricao *string) error {
	ok := false
	for _, m := range modelos {
		if m.ID == idModelo {
			ok = true
		}
	}
	if !ok {
		return erros.Campo("modelo", "Modelo desconhecido. Use responder_historico, classificar_funil, extrair_dados ou em_branco.")
	}
	pasta := p.Pasta(id)
	if err := os.MkdirAll(pasta, 0o700); err != nil {
		return err
	}
	raiz := "modelos/" + idModelo
	err := fs.WalkDir(modelosFS, raiz, func(caminho string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		dados, err := modelosFS.ReadFile(caminho)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(caminho, raiz+"/")
		if rel == ArquivoManif {
			dados, err = preencherManifesto(dados, nome, descricao)
			if err != nil {
				return err
			}
		}
		destino := filepath.Join(pasta, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(destino), 0o700); err != nil {
			return err
		}
		return os.WriteFile(destino, dados, 0o600)
	})
	if err != nil {
		return err
	}
	return p.AtualizarGerados(id)
}

// preencherManifesto troca nome e descrição preservando a ordem das chaves do modelo.
func preencherManifesto(dados []byte, nome string, descricao *string) ([]byte, error) {
	var m modelo.Manifesto
	if err := json.Unmarshal(dados, &m); err != nil {
		return nil, err
	}
	m.Nome = nome
	m.Descricao = descricao
	if m.Segredos == nil {
		m.Segredos = []string{}
	}
	if m.Permissoes == nil {
		m.Permissoes = []string{}
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// AtualizarGerados reescreve .zapdesk/automacao.d.ts, .zapdesk/automacao.schema.json e tsconfig.json.
func (p *Projetos) AtualizarGerados(id string) error {
	pasta := p.Pasta(id)
	dir := filepath.Join(pasta, DirGerados)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	esquema, _ := json.MarshalIndent(EsquemaManifesto(), "", "  ")
	tsconfig, _ := json.MarshalIndent(map[string]any{
		"compilerOptions": map[string]any{
			"target": "ES2022", "module": "ES2022", "moduleResolution": "bundler", "strict": true,
			"noEmit": true, "isolatedModules": true, "skipLibCheck": true, "lib": []string{"ES2022", "DOM"},
			"paths": map[string]any{"@zapdesk/automacao": []string{"./.zapdesk/automacao.d.ts"}},
		},
		"include": []string{"**/*.ts", ".zapdesk/automacao.d.ts"},
	}, "", "  ")
	arquivos := map[string][]byte{
		filepath.Join(dir, "automacao.d.ts"):        []byte(tiposSDK),
		filepath.Join(dir, "automacao.schema.json"): append(esquema, '\n'),
		filepath.Join(pasta, ArquivoTSConf):         append(tsconfig, '\n'),
	}
	for caminho, dados := range arquivos {
		if atual, err := os.ReadFile(caminho); err == nil && string(atual) == string(dados) {
			continue
		}
		if err := os.WriteFile(caminho, dados, 0o600); err != nil {
			return err
		}
	}
	return nil
}

// oculto indica arquivos gerados que a API não mostra.
func oculto(rel string) bool {
	return rel == ArquivoTSConf || strings.HasPrefix(rel, DirGerados+"/") || strings.HasPrefix(path.Base(rel), ".")
}

// arquivosUsuario lista caminhos relativos (com "/") dos arquivos do usuário.
func (p *Projetos) arquivosUsuario(id string) ([]string, error) { return listarPasta(p.Pasta(id)) }

func (p *Projetos) info(id, rel string) (Arquivo, error) {
	c := p.abs(id, rel)
	dados, err := os.ReadFile(c)
	if err != nil {
		return Arquivo{}, err
	}
	st, err := os.Stat(c)
	if err != nil {
		return Arquivo{}, err
	}
	return Arquivo{Caminho: rel, Tamanho: int64(len(dados)), Hash: hashDe(dados), AtualizadoEm: st.ModTime()}, nil
}

// Listar os arquivos do usuário (sem .zapdesk/ e tsconfig.json), por caminho.
func (p *Projetos) Listar(id string) ([]Arquivo, error) {
	if err := p.existeProjeto(id); err != nil {
		return nil, err
	}
	rels, err := p.arquivosUsuario(id)
	if err != nil {
		return nil, err
	}
	lista := []Arquivo{}
	for _, r := range rels {
		a, err := p.info(id, r)
		if err != nil {
			return nil, err
		}
		lista = append(lista, a)
	}
	return lista, nil
}

// Ler um arquivo do projeto.
func (p *Projetos) Ler(id, caminho string) (Conteudo, error) {
	if err := p.existeProjeto(id); err != nil {
		return Conteudo{}, err
	}
	if err := ValidarCaminho(caminho); err != nil {
		return Conteudo{}, erros.NaoAchado("Arquivo")
	}
	c := p.abs(id, caminho)
	dados, err := os.ReadFile(c)
	if err != nil {
		return Conteudo{}, erros.NaoAchado("Arquivo")
	}
	st, _ := os.Stat(c)
	return Conteudo{Caminho: caminho, Conteudo: string(dados), Hash: hashDe(dados), AtualizadoEm: st.ModTime()}, nil
}

func conflito(hashAtual string) error {
	return erros.ComDetalhes(erros.Conflito, "O arquivo foi alterado fora do app.", map[string]any{"hash_atual": hashAtual})
}

// Escrever grava um arquivo. verificar=false: grava sempre; verificar=true e hashAnterior=nil:
// o arquivo precisa ser novo; verificar=true e hashAnterior≠nil: precisa bater com o hash atual.
func (p *Projetos) Escrever(id, caminho, conteudo string, verificar bool, hashAnterior *string) (Arquivo, bool, error) {
	if err := p.existeProjeto(id); err != nil {
		return Arquivo{}, false, err
	}
	if err := ValidarCaminho(caminho); err != nil {
		return Arquivo{}, false, err
	}
	if len(conteudo) > MaxTamanho {
		return Arquivo{}, false, erros.Campo("conteudo", "Cada arquivo pode ter até 1 MB.")
	}
	c := p.abs(id, caminho)
	atual, errLer := os.ReadFile(c)
	existe := errLer == nil
	if verificar {
		switch {
		case hashAnterior == nil && existe:
			return Arquivo{}, false, conflito(hashDe(atual))
		case hashAnterior != nil && !existe:
			return Arquivo{}, false, conflito("")
		case hashAnterior != nil && hashDe(atual) != *hashAnterior:
			return Arquivo{}, false, conflito(hashDe(atual))
		}
	}
	if !existe {
		rels, err := p.arquivosUsuario(id)
		if err != nil {
			return Arquivo{}, false, err
		}
		if len(rels) >= MaxArquivos {
			return Arquivo{}, false, erros.Campo("caminho", "O projeto pode ter até 50 arquivos.")
		}
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return Arquivo{}, false, erros.Campo("caminho", "Já existe uma pasta com esse nome.")
		}
	}
	if err := os.MkdirAll(filepath.Dir(c), 0o700); err != nil {
		return Arquivo{}, false, erros.Campo("caminho", "Não foi possível criar a pasta do arquivo.")
	}
	if err := os.WriteFile(c, []byte(conteudo), 0o600); err != nil {
		return Arquivo{}, false, err
	}
	a, err := p.info(id, caminho)
	return a, !existe, err
}

// entrada lê o arquivo de entrada do manifesto (padrão index.ts).
func (p *Projetos) entrada(id string) string {
	dados, err := os.ReadFile(p.abs(id, ArquivoManif))
	if err != nil {
		return "index.ts"
	}
	var m struct {
		Entrada string `json:"entrada"`
	}
	if json.Unmarshal(dados, &m) != nil || m.Entrada == "" {
		return "index.ts"
	}
	return m.Entrada
}

func (p *Projetos) protegido(id, caminho string) error {
	if caminho == ArquivoManif {
		return erros.Campo("caminho", "automacao.json não pode ser excluído nem renomeado.")
	}
	if caminho == p.entrada(id) {
		return erros.Campo("caminho", "O arquivo de entrada ("+caminho+") não pode ser excluído nem renomeado.")
	}
	return nil
}

// Excluir apaga um arquivo (nunca o manifesto nem a entrada).
func (p *Projetos) Excluir(id, caminho string) error {
	if err := p.existeProjeto(id); err != nil {
		return err
	}
	if err := ValidarCaminho(caminho); err != nil {
		return err
	}
	if err := p.protegido(id, caminho); err != nil {
		return err
	}
	c := p.abs(id, caminho)
	if st, err := os.Stat(c); err != nil || st.IsDir() {
		return erros.NaoAchado("Arquivo")
	}
	if err := os.Remove(c); err != nil {
		return err
	}
	p.limparPastasVazias(id, filepath.Dir(c))
	return nil
}

func (p *Projetos) limparPastasVazias(id, dir string) {
	raiz := p.Pasta(id)
	for dir != raiz && strings.HasPrefix(dir, raiz) {
		if os.Remove(dir) != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

// Renomear move um arquivo dentro do projeto.
func (p *Projetos) Renomear(id, de, para string) (Arquivo, error) {
	if err := p.existeProjeto(id); err != nil {
		return Arquivo{}, err
	}
	if err := ValidarCaminho(de); err != nil {
		return Arquivo{}, err
	}
	if err := ValidarCaminho(para); err != nil {
		return Arquivo{}, erros.Campo("para", err.(*erros.Erro).Mensagem)
	}
	if err := p.protegido(id, de); err != nil {
		return Arquivo{}, err
	}
	origem, destino := p.abs(id, de), p.abs(id, para)
	if st, err := os.Stat(origem); err != nil || st.IsDir() {
		return Arquivo{}, erros.NaoAchado("Arquivo")
	}
	if _, err := os.Stat(destino); err == nil {
		return Arquivo{}, erros.Novo(erros.Conflito, "Já existe um arquivo com esse nome.")
	}
	if err := os.MkdirAll(filepath.Dir(destino), 0o700); err != nil {
		return Arquivo{}, err
	}
	if err := os.Rename(origem, destino); err != nil {
		return Arquivo{}, err
	}
	p.limparPastasVazias(id, filepath.Dir(origem))
	return p.info(id, para)
}

// ApagarProjeto remove a pasta do projeto e os bundles compilados.
func (p *Projetos) ApagarProjeto(id string) error {
	if id == "" || strings.ContainsAny(id, `/\.`) {
		return errors.New("id inválido")
	}
	err1 := os.RemoveAll(p.Pasta(id))
	err2 := os.RemoveAll(p.PastaCompiladas(id))
	if err1 != nil {
		return err1
	}
	return err2
}

// HashFontes calcula o hash das fontes do projeto (mesmo algoritmo do compilador).
func (p *Projetos) HashFontes(id string) (string, error) {
	if err := p.existeProjeto(id); err != nil {
		return "", err
	}
	return HashPasta(p.Pasta(id))
}

// HashPasta é o SHA-256 dos arquivos do usuário (caminho e conteúdo, em ordem) + VersaoHash.
func HashPasta(pasta string) (string, error) {
	rels, err := listarPasta(pasta)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	fmt.Fprintf(h, "%s\n", VersaoHash)
	for _, r := range rels {
		dados, err := os.ReadFile(filepath.Join(pasta, filepath.FromSlash(r)))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", r, len(dados))
		h.Write(dados)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// listarPasta lista os arquivos do usuário (sem ocultos e gerados), por caminho.
func listarPasta(pasta string) ([]string, error) {
	var lista []string
	err := filepath.WalkDir(pasta, func(c string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(pasta, c)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if oculto(rel) || !d.Type().IsRegular() {
			return nil
		}
		lista = append(lista, rel)
		return nil
	})
	sort.Strings(lista)
	return lista, err
}
