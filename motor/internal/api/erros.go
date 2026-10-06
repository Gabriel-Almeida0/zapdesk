package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"zapdesk/motor/internal/erros"
)

type corpoErro struct {
	Erro struct {
		Codigo   string         `json:"codigo"`
		Mensagem string         `json:"mensagem"`
		Detalhes map[string]any `json:"detalhes,omitempty"`
	} `json:"erro"`
}

// Mensagens padrão (pt-BR) por código, usadas quando o erro não traz uma.
var mensagensPadrao = map[string]string{
	erros.NaoAutorizado:     "Acesso não autorizado.",
	erros.HostInvalido:      "Endereço de acesso inválido.",
	erros.NaoEncontrado:     "Item não encontrado.",
	erros.Validacao:         "Confira os campos destacados.",
	erros.Conflito:          "Já existe um item com esse nome.",
	erros.TransicaoInvalida: "Essa ação não é permitida agora.",
	erros.VariaveisFaltando: "Há contatos sem valor para as variáveis da mensagem.",
	erros.ContaIndisponivel: "Conta desconectada. Reconecte para continuar.",
	erros.SemWhatsApp:       "Este número não tem WhatsApp.",
	erros.ForaDoPrazo:       "O prazo do WhatsApp para essa ação já passou.",
	erros.AnexoGrandeDemais: "Arquivo grande demais para o WhatsApp.",
	erros.TipoNaoSuportado:  "Tipo de arquivo não suportado.",
	erros.WhatsAppErro:      "O WhatsApp não respondeu. Tente de novo.",
	erros.Interno:           "Algo deu errado no ZapDesk. Tente de novo.",

	erros.DefinicaoInvalida:  "A automação tem erros. Confira os campos destacados.",
	erros.CompilacaoFalhou:   "O código da automação tem erros de compilação.",
	erros.RunnerIndisponivel: "As automações de IA não podem ser executadas nesta instalação.",
	erros.IANaoConfigurada:   "Configure a chave da Anthropic em Ajustes → IA.",
	erros.IAErro:             "A Claude API respondeu com erro. Tente de novo em instantes.",
}

func responderErroCodigo(w http.ResponseWriter, codigo, mensagem string, detalhes map[string]any) {
	status, ok := erros.Status[codigo]
	if !ok {
		status, codigo = 500, erros.Interno
	}
	if mensagem == "" {
		mensagem = mensagensPadrao[codigo]
	}
	var c corpoErro
	c.Erro.Codigo = codigo
	c.Erro.Mensagem = mensagem
	c.Erro.Detalhes = detalhes
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(c)
}

// responderErro traduz qualquer erro; erros não-domínio viram `interno` (e são logados).
func (s *Servidor) responderErro(w http.ResponseWriter, r *http.Request, err error) {
	if e, ok := erros.Como(err); ok {
		responderErroCodigo(w, e.Codigo, e.Mensagem, e.Detalhes)
		return
	}
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		responderErroCodigo(w, erros.AnexoGrandeDemais, "", map[string]any{"limite_bytes": mbe.Limit})
		return
	}
	s.o.Log.Error().Err(err).Str("metodo", r.Method).Str("caminho", r.URL.Path).Msg("erro interno na API")
	responderErroCodigo(w, erros.Interno, "", nil)
}
