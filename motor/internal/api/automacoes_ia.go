package api

import (
	"encoding/json"
	"net/http"

	"zapdesk/motor/internal/automacoes"
	"zapdesk/motor/internal/automacoes/projetos"
	"zapdesk/motor/internal/erros"
)

// Rotas das automações de IA (contracts/api-http.md › Automações de IA — projeto e compilação).
func (s *Servidor) registrarRotasAutomacoesIA() {
	s.rota("POST /v1/automacoes/ia", s.criarAutomacaoIA, false)
	s.rota("GET /v1/automacoes/modelos", s.modelosAutomacao, false)
	s.rota("GET /v1/automacoes/sdk", s.sdkAutomacao, false)
	s.rota("GET /v1/automacoes/{id}/arquivos", s.listarArquivosAutomacao, false)
	s.rota("GET /v1/automacoes/{id}/arquivos/{caminho...}", s.lerArquivoAutomacao, false)
	s.rota("PUT /v1/automacoes/{id}/arquivos/{caminho...}", s.escreverArquivoAutomacao, false)
	s.rota("DELETE /v1/automacoes/{id}/arquivos/{caminho...}", s.excluirArquivoAutomacao, false)
	s.rota("POST /v1/automacoes/{id}/arquivos/renomear", s.renomearArquivoAutomacao, false)
	s.rota("POST /v1/automacoes/{id}/compilar", s.compilarAutomacao, false)
}

func (s *Servidor) ia() *automacoes.IA { return s.o.Servicos.AutomacoesIA }

func (s *Servidor) criarAutomacaoIA(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Nome      string  `json:"nome"`
		Modelo    string  `json:"modelo"`
		Descricao *string `json:"descricao"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	a, err := s.ia().CriarPeloModelo(r.Context(), c.Nome, c.Modelo, c.Descricao)
	if err != nil {
		return err
	}
	return responderJSON(w, 201, a)
}

func (s *Servidor) modelosAutomacao(w http.ResponseWriter, r *http.Request) error {
	return responderJSON(w, 200, projetos.Modelos())
}

func (s *Servidor) sdkAutomacao(w http.ResponseWriter, r *http.Request) error {
	return responderJSON(w, 200, map[string]any{"versao": projetos.VersaoSDK(), "tipos": projetos.TiposSDK(), "esquema_manifesto": projetos.EsquemaManifesto()})
}

func (s *Servidor) listarArquivosAutomacao(w http.ResponseWriter, r *http.Request) error {
	l, err := s.ia().ListarArquivos(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, l)
}

func (s *Servidor) lerArquivoAutomacao(w http.ResponseWriter, r *http.Request) error {
	c, err := s.ia().LerArquivo(r.Context(), r.PathValue("id"), r.PathValue("caminho"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, c)
}

func (s *Servidor) escreverArquivoAutomacao(w http.ResponseWriter, r *http.Request) error {
	var corpo map[string]json.RawMessage
	if err := lerJSON(w, r, &corpo); err != nil {
		return err
	}
	var conteudo string
	if err := json.Unmarshal(corpo["conteudo"], &conteudo); err != nil {
		return erros.Campo("conteudo", "Envie o conteúdo do arquivo como texto.")
	}
	verificar := false
	var hash *string
	if b, ok := corpo["hash_anterior"]; ok {
		verificar = true
		if string(b) != "null" {
			var h string
			if err := json.Unmarshal(b, &h); err != nil {
				return erros.Campo("hash_anterior", "Use o hash (texto) ou null.")
			}
			hash = &h
		}
	}
	a, criado, err := s.ia().EscreverArquivo(r.Context(), r.PathValue("id"), r.PathValue("caminho"), conteudo, verificar, hash)
	if err != nil {
		return err
	}
	status := 200
	if criado {
		status = 201
	}
	return responderJSON(w, status, a)
}

func (s *Servidor) excluirArquivoAutomacao(w http.ResponseWriter, r *http.Request) error {
	if err := s.ia().ExcluirArquivo(r.Context(), r.PathValue("id"), r.PathValue("caminho")); err != nil {
		return err
	}
	return semConteudo(w)
}

func (s *Servidor) renomearArquivoAutomacao(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		De   string `json:"de"`
		Para string `json:"para"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	a, err := s.ia().RenomearArquivo(r.Context(), r.PathValue("id"), c.De, c.Para)
	if err != nil {
		return err
	}
	return responderJSON(w, 200, a)
}

func (s *Servidor) compilarAutomacao(w http.ResponseWriter, r *http.Request) error {
	res, err := s.ia().CompilarAPI(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, res)
}
