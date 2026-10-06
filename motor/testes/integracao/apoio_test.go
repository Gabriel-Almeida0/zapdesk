package integracao

import (
	"encoding/json"
	"testing"
)

// Tipos mínimos do contrato usados nos testes.
type Conta struct {
	ID            string  `json:"id"`
	Nome          string  `json:"nome"`
	Telefone      *string `json:"telefone"`
	JID           *string `json:"jid"`
	Estado        string  `json:"estado"`
	Online        bool    `json:"online"`
	Sincronizando bool    `json:"sincronizando"`
	CriadaEm      string  `json:"criada_em"`
}

type Conversa struct {
	ID                   string     `json:"id"`
	ContaID              string     `json:"conta_id"`
	JID                  string     `json:"jid"`
	Tipo                 string     `json:"tipo"`
	Nome                 string     `json:"nome"`
	Telefone             *string    `json:"telefone"`
	ContatoID            *string    `json:"contato_id"`
	NaoLidas             int        `json:"nao_lidas"`
	UltimaMensagemEm     *string    `json:"ultima_mensagem_em"`
	UltimaMensagemResumo *string    `json:"ultima_mensagem_resumo"`
	Etiquetas            []Etiqueta `json:"etiquetas"`
}

type Etiqueta struct {
	ID            string `json:"id"`
	Nome          string `json:"nome"`
	Cor           string `json:"cor"`
	TotalContatos int    `json:"total_contatos"`
}

type Midia struct {
	Mimetype    string  `json:"mimetype"`
	Tamanho     int64   `json:"tamanho"`
	NomeArquivo *string `json:"nome_arquivo"`
	PTT         bool    `json:"ptt"`
	Baixada     bool    `json:"baixada"`
	URL         string  `json:"url"`
}

type Mensagem struct {
	ID            string  `json:"id"`
	ContaID       string  `json:"conta_id"`
	ConversaID    string  `json:"conversa_id"`
	WaID          string  `json:"wa_id"`
	RemetenteJID  string  `json:"remetente_jid"`
	RemetenteNome *string `json:"remetente_nome"`
	DeMim         bool    `json:"de_mim"`
	Tipo          string  `json:"tipo"`
	Texto         *string `json:"texto"`
	Midia         *Midia  `json:"midia"`
	Citacao       *struct {
		WaID   string `json:"wa_id"`
		Resumo string `json:"resumo"`
	} `json:"citacao"`
	Reacoes []struct {
		RemetenteJID string `json:"remetente_jid"`
		Emoji        string `json:"emoji"`
		DeMim        bool   `json:"de_mim"`
	} `json:"reacoes"`
	Editada    bool    `json:"editada"`
	Apagada    bool    `json:"apagada"`
	Estado     string  `json:"estado"`
	Erro       *string `json:"erro"`
	DisparoID  *string `json:"disparo_id"`
	EnviadaEm  string  `json:"enviada_em"`
	PodeEditar bool    `json:"pode_editar"`
	PodeApagar bool    `json:"pode_apagar"`
}

type Contato struct {
	ID        string     `json:"id"`
	ContaID   string     `json:"conta_id"`
	JID       string     `json:"jid"`
	Telefone  *string    `json:"telefone"`
	Nome      *string    `json:"nome"`
	NomePush  *string    `json:"nome_push"`
	Notas     *string    `json:"notas"`
	Etiquetas []Etiqueta `json:"etiquetas"`
	Lead      *struct {
		ID          string `json:"id"`
		Origem      string `json:"origem"`
		ImportadoEm string `json:"importado_em"`
	} `json:"lead"`
	ConversaID *string `json:"conversa_id"`
}

type Pagina[T any] struct {
	Itens         []T     `json:"itens"`
	ProximoCursor *string `json:"proximo_cursor"`
}

func dados[T any](t *testing.T, ev Evento) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(ev.Dados, &v); err != nil {
		t.Fatalf("dados de %s: %v", ev.Tipo, err)
	}
	return v
}

func ptr(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

// contaConectada cria uma conta e simula a leitura do QR.
func (m *Motor) contaConectada(telefone string) Conta {
	m.t.Helper()
	var c Conta
	marca := m.marca()
	m.json("POST", "/v1/contas", map[string]any{}, 201, &c)
	m.esperarEvento(marca, "conta.qr", func(e Evento) bool { return e.ContaID != nil && *e.ContaID == c.ID })
	m.json("POST", "/v1/falso/contas/"+c.ID+"/escanear-qr", map[string]string{"telefone": telefone, "nome": "Loja Teste"}, 200, nil)
	m.json("GET", "/v1/contas/"+c.ID, nil, 200, &c)
	if c.Estado != "conectada" {
		m.t.Fatalf("conta não conectou: %+v", c)
	}
	return c
}

// receber injeta uma mensagem recebida e devolve o wa_id.
func (m *Motor) receber(contaID string, corpo map[string]any) string {
	m.t.Helper()
	var r struct {
		WaID string `json:"wa_id"`
	}
	m.json("POST", "/v1/falso/contas/"+contaID+"/mensagem-recebida", corpo, 200, &r)
	return r.WaID
}

func (m *Motor) conversas(contaID, query string) []Conversa {
	m.t.Helper()
	var p Pagina[Conversa]
	m.json("GET", "/v1/contas/"+contaID+"/conversas"+query, nil, 200, &p)
	return p.Itens
}

func (m *Motor) mensagens(conversaID string) []Mensagem {
	m.t.Helper()
	var p Pagina[Mensagem]
	m.json("GET", "/v1/conversas/"+conversaID+"/mensagens", nil, 200, &p)
	return p.Itens
}

// esperarMensagem espera um mensagem.atualizada da mensagem com o estado dado.
func (m *Motor) esperarMensagem(desde int, id, estado string) Mensagem {
	m.t.Helper()
	ev := m.esperarEvento(desde, "mensagem.atualizada", func(e Evento) bool {
		var msg Mensagem
		json.Unmarshal(e.Dados, &msg)
		return msg.ID == id && msg.Estado == estado
	})
	return dados[Mensagem](m.t, ev)
}
