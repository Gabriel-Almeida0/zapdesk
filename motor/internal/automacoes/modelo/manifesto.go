package modelo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Permissões do manifesto.
const (
	PermEnviar       = "enviar"
	PermLerConversas = "ler_conversas"
	PermEtiquetas    = "etiquetas"
	PermFunil        = "funil"
	PermLeads        = "leads"
	PermIA           = "ia"
	PermRede         = "rede"
	PermAgendar      = "agendar"
)

// Permissoes válidas (na ordem do contrato).
var Permissoes = []string{PermEnviar, PermLerConversas, PermEtiquetas, PermFunil, PermLeads, PermIA, PermRede, PermAgendar}

// Handlers da SDK.
const (
	HandlerMensagem = "aoReceberMensagem"
	HandlerAgendar  = "aoAgendar"
	HandlerExecutar = "aoExecutar"
	HandlerEvento   = "aoEvento"
)

// HandlerDoGatilho devolve o handler exigido por um gatilho (formatos.md › Mapa gatilho → handler).
func HandlerDoGatilho(tipo string) string {
	switch tipo {
	case GatilhoMensagemRecebida, GatilhoPalavraChave:
		return HandlerMensagem
	case GatilhoAgendamento, GatilhoAgendar:
		return HandlerAgendar
	case GatilhoManual:
		return HandlerExecutar
	case GatilhoLeadImportado, GatilhoEtiqueta, GatilhoEntrouEtapa, GatilhoDisparoRespondeu, GatilhoSemResposta:
		return HandlerEvento
	}
	return ""
}

// ContasManifesto é "todas" (nil) ou uma lista de conta_id.
type ContasManifesto struct{ Lista []string }

// UnmarshalJSON aceita "todas" ou uma lista.
func (c *ContasManifesto) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if bytes.Equal(b, []byte("null")) {
		c.Lista = nil
		return nil
	}
	var s string
	if json.Unmarshal(b, &s) == nil {
		if s != "todas" {
			return errors.New(`use "todas" ou uma lista de contas`)
		}
		c.Lista = nil
		return nil
	}
	var l []string
	if err := json.Unmarshal(b, &l); err != nil {
		return errors.New(`use "todas" ou uma lista de contas`)
	}
	if l == nil {
		l = []string{}
	}
	c.Lista = l
	return nil
}

// MarshalJSON devolve "todas" ou a lista.
func (c ContasManifesto) MarshalJSON() ([]byte, error) {
	if c.Lista == nil {
		return []byte(`"todas"`), nil
	}
	return json.Marshal(c.Lista)
}

// LimitesManifesto do automacao.json.
type LimitesManifesto struct {
	TempoS    *int      `json:"tempo_s"`
	MemoriaMB *int      `json:"memoria_mb"`
	AntiLoop  *AntiLoop `json:"anti_loop"`
}

// IAManifesto do automacao.json.
type IAManifesto struct {
	Modelo *string `json:"modelo"`
}

// Manifesto é o `automacao.json` de uma automação de IA.
type Manifesto struct {
	Schema          string           `json:"$schema,omitempty"`
	VersaoManifesto int              `json:"versao_manifesto"`
	Nome            string           `json:"nome"`
	Descricao       *string          `json:"descricao"`
	Entrada         string           `json:"entrada,omitempty"`
	Gatilhos        []Gatilho        `json:"gatilhos"`
	Permissoes      []string         `json:"permissoes"`
	Segredos        []string         `json:"segredos"`
	Contas          *ContasManifesto `json:"contas,omitempty"`
	IncluirGrupos   bool             `json:"incluir_grupos"`
	Prioridade      *int             `json:"prioridade,omitempty"`
	ContaEnvio      *string          `json:"conta_envio"`
	Limites         LimitesManifesto `json:"limites"`
	IA              IAManifesto      `json:"ia"`
}

// ArquivoEntrada devolve a entrada (padrão index.ts).
func (m Manifesto) ArquivoEntrada() string {
	if m.Entrada == "" {
		return "index.ts"
	}
	return m.Entrada
}

// PrioridadeOuPadrao devolve a prioridade (padrão 100).
func (m Manifesto) PrioridadeOuPadrao() int {
	if m.Prioridade == nil {
		return 100
	}
	return *m.Prioridade
}

// TemPermissao indica se o manifesto declara a permissão.
func (m Manifesto) TemPermissao(p string) bool {
	for _, x := range m.Permissoes {
		if x == p {
			return true
		}
	}
	return false
}

// DecodificarManifesto lê o JSON do manifesto (campos desconhecidos são erro, para pegar erros
// de digitação). Erros de tipo viram ErroDefinicao com o caminho do campo.
func DecodificarManifesto(dados []byte) (Manifesto, []ErroDefinicao) {
	var m Manifesto
	dec := json.NewDecoder(bytes.NewReader(dados))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return m, []ErroDefinicao{erroDeJSON(err, "")}
	}
	return m, nil
}

// ValidarManifesto valida o manifesto (caminhos relativos ao automacao.json).
func ValidarManifesto(m Manifesto) []ErroDefinicao {
	c := &coletor{}
	if m.VersaoManifesto != 1 {
		c.add("versao_manifesto", "Use versao_manifesto 1.")
	}
	if vazio(m.Nome) || tamanho(m.Nome) > 80 {
		c.add("nome", "O nome deve ter de 1 a 80 caracteres.")
	}
	if m.Descricao != nil && tamanho(*m.Descricao) > 500 {
		c.add("descricao", "A descrição pode ter até 500 caracteres.")
	}
	e := m.ArquivoEntrada()
	if !strings.HasSuffix(e, ".ts") || strings.Contains(e, "..") || strings.HasPrefix(e, "/") || strings.HasPrefix(e, ".") {
		c.add("entrada", "A entrada deve ser um arquivo .ts do projeto.")
	}
	if len(m.Gatilhos) == 0 {
		c.add("gatilhos", "Declare pelo menos um gatilho.")
	}
	for _, er := range ValidarGatilhos(m.Gatilhos, OpcoesGatilhos{IncluirGrupos: m.IncluirGrupos, Manifesto: true}) {
		c.erros = append(c.erros, er)
	}
	vistas := map[string]bool{}
	for i, p := range m.Permissoes {
		ok := false
		for _, v := range Permissoes {
			if p == v {
				ok = true
			}
		}
		if !ok {
			c.add(fmt.Sprintf("permissoes[%d]", i), "Permissão desconhecida: %q (use %s).", p, strings.Join(Permissoes, ", "))
		}
		if vistas[p] {
			c.add(fmt.Sprintf("permissoes[%d]", i), "Permissão repetida: %q.", p)
		}
		vistas[p] = true
	}
	for i, s := range m.Segredos {
		switch {
		case s == "ANTHROPIC_API_KEY":
			c.add(fmt.Sprintf("segredos[%d]", i), "ANTHROPIC_API_KEY é reservado: use ctx.ia.")
		case !reSegredo.MatchString(s):
			c.add(fmt.Sprintf("segredos[%d]", i), "Nome de segredo inválido (maiúsculas, números e _).")
		}
	}
	if m.Prioridade != nil && (*m.Prioridade < 1 || *m.Prioridade > 1000) {
		c.add("prioridade", "A prioridade deve ficar entre 1 e 1.000.")
	}
	for _, er := range ValidarLimites(Limites{AntiLoop: m.Limites.AntiLoop, TempoS: m.Limites.TempoS, MemoriaMB: m.Limites.MemoriaMB}) {
		c.erros = append(c.erros, er)
	}
	if m.IA.Modelo != nil && !strings.HasPrefix(*m.IA.Modelo, "claude-") {
		c.add("ia.modelo", "O modelo deve começar com \"claude-\" (ou null para o padrão).")
	}
	return c.erros
}

var reSegredo = reNome()

// erroDeJSON converte erros do encoding/json em ErroDefinicao.
func erroDeJSON(err error, prefixo string) ErroDefinicao {
	var te *json.UnmarshalTypeError
	var se *json.SyntaxError
	caminho := prefixo
	msg := "JSON inválido: " + err.Error()
	switch {
	case errors.As(err, &te):
		if te.Field != "" {
			if caminho != "" {
				caminho += "."
			}
			caminho += te.Field
		}
		msg = fmt.Sprintf("Tipo inválido (esperado %s).", nomeTipo(te.Type.Kind().String()))
	case errors.As(err, &se):
		msg = fmt.Sprintf("JSON inválido na posição %d.", se.Offset)
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		campo := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
		msg = fmt.Sprintf("Campo desconhecido: %q.", campo)
		if caminho != "" {
			caminho += "."
		}
		caminho += campo
	}
	if caminho == "" {
		caminho = "$"
	}
	return ErroDefinicao{Caminho: caminho, Mensagem: msg}
}

func nomeTipo(k string) string {
	switch k {
	case "string":
		return "texto"
	case "int", "int64", "float64":
		return "número"
	case "bool":
		return "verdadeiro/falso"
	case "slice":
		return "lista"
	case "map", "struct", "ptr":
		return "objeto"
	}
	return k
}

// DecodificarFluxo lê uma DefinicaoFluxo de JSON bruto (caminho "definicao").
func DecodificarFluxo(bruto json.RawMessage) (*DefinicaoFluxo, []ErroDefinicao) {
	if len(bytes.TrimSpace(bruto)) == 0 || bytes.Equal(bytes.TrimSpace(bruto), []byte("null")) {
		return nil, []ErroDefinicao{{Caminho: "definicao", Mensagem: "Informe a definição."}}
	}
	var d DefinicaoFluxo
	if err := json.Unmarshal(bruto, &d); err != nil {
		return nil, []ErroDefinicao{erroDeJSON(err, "definicao")}
	}
	return &d, nil
}

// DecodificarChatbot lê uma DefinicaoChatbot de JSON bruto (caminho "definicao").
func DecodificarChatbot(bruto json.RawMessage) (*DefinicaoChatbot, []ErroDefinicao) {
	if len(bytes.TrimSpace(bruto)) == 0 || bytes.Equal(bytes.TrimSpace(bruto), []byte("null")) {
		return nil, []ErroDefinicao{{Caminho: "definicao", Mensagem: "Informe a definição."}}
	}
	var d DefinicaoChatbot
	if err := json.Unmarshal(bruto, &d); err != nil {
		return nil, []ErroDefinicao{erroDeJSON(err, "definicao")}
	}
	return &d, nil
}

// DecodificarGatilhos lê a lista de gatilhos de JSON bruto (caminho "gatilhos").
func DecodificarGatilhos(bruto json.RawMessage) ([]Gatilho, []ErroDefinicao) {
	if len(bytes.TrimSpace(bruto)) == 0 || bytes.Equal(bytes.TrimSpace(bruto), []byte("null")) {
		return []Gatilho{}, nil
	}
	var g []Gatilho
	if err := json.Unmarshal(bruto, &g); err != nil {
		return nil, []ErroDefinicao{erroDeJSON(err, "gatilhos")}
	}
	return g, nil
}
