package api

import (
	"net/http"

	"zapdesk/motor/internal/erros"
)

func (s *Servidor) registrarRotas() {
	s.rota("GET /v1/saude", s.saude, false)
	s.rota("GET /v1/sistema", s.sistema, false)
	s.rota("POST /v1/sistema/encerrar", s.encerrar, false)
	s.rota("POST /v1/sistema/energia", s.energia, false)
	s.rota("GET /v1/eventos", s.eventosWS, true)
	if s.o.Falso != nil {
		s.registrarRotasFalso()
	}
	s.registrarRotasDominio()
	s.rota("/", func(w http.ResponseWriter, r *http.Request) error {
		return erros.Novo(erros.NaoEncontrado, "Rota não encontrada.")
	}, true)
}

func (s *Servidor) saude(w http.ResponseWriter, r *http.Request) error {
	return responderJSON(w, 200, map[string]any{"ok": true, "versao": s.o.Versao, "whatsapp": s.o.ModoWhatsApp})
}

func (s *Servidor) sistema(w http.ResponseWriter, r *http.Request) error {
	ativos, conectadas := 0, 0
	if c := s.o.Servicos.Contadores; c != nil {
		ativos, conectadas = c.DisparosAtivos(), c.ContasConectadas()
	}
	resp := map[string]any{
		"versao":            s.o.Versao,
		"pasta_dados":       s.o.PastaDados,
		"caminho_logs":      s.o.CaminhoLogs,
		"whatsapp":          s.o.ModoWhatsApp,
		"disparos_ativos":   ativos,
		"contas_conectadas": conectadas,
		// Feature 002.
		"automacoes_ativas": 0,
		"processos_ia":      0,
		"runner_disponivel": false,
	}
	if c, ok := s.o.Servicos.Contadores.(ContadoresAutomacoes); ok {
		resp["automacoes_ativas"], resp["processos_ia"], resp["runner_disponivel"] = c.AutomacoesAtivas(), c.ProcessosIA(), c.RunnerDisponivel()
	}
	return responderJSON(w, 200, resp)
}

func (s *Servidor) encerrar(w http.ResponseWriter, r *http.Request) error {
	w.WriteHeader(http.StatusAccepted)
	if s.o.Encerrar != nil {
		go s.o.Encerrar()
	}
	return nil
}

func (s *Servidor) energia(w http.ResponseWriter, r *http.Request) error {
	var corpo struct {
		Evento string `json:"evento"`
	}
	if err := lerJSON(w, r, &corpo); err != nil {
		return err
	}
	if corpo.Evento != "suspender" && corpo.Evento != "retomar" {
		return erros.Campo("evento", "Use \"suspender\" ou \"retomar\".")
	}
	if s.o.Energia != nil {
		s.o.Energia(corpo.Evento)
	}
	return semConteudo(w)
}
