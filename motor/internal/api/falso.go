package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/telefone"
	"zapdesk/motor/internal/whatsapp/falso"
)

// Sincronizador espera o motor processar os eventos já emitidos por uma conta (as rotas de
// injeção do modo falso só respondem depois disso, para que o estado já esteja visível na API).
type Sincronizador interface {
	AguardarEventos(ctx context.Context, contaID string) error
}

// registrarRotasFalso registra /v1/falso/* (somente com --whatsapp=falso; 404 no modo real).
func (s *Servidor) registrarRotasFalso() {
	s.rota("POST /v1/falso/contas/{id}/escanear-qr", s.falsoEscanear, false)
	s.rota("POST /v1/falso/contas/{id}/expirar-qr", s.falsoExpirarQR, false)
	s.rota("POST /v1/falso/contas/{id}/mensagem-recebida", s.falsoMensagem, false)
	s.rota("POST /v1/falso/contas/{id}/recibo", s.falsoRecibo, false)
	s.rota("POST /v1/falso/contas/{id}/estado", s.falsoEstado, false)
	s.rota("POST /v1/falso/contas/{id}/historico", s.falsoHistorico, false)
	s.rota("POST /v1/falso/contas/{id}/status", s.falsoStatus, false)
	s.rota("PUT /v1/falso/numeros-sem-whatsapp", s.falsoSemWhatsApp, false)
	s.rota("PUT /v1/falso/falhas-envio", s.falsoFalhas, false)
	s.rota("GET /v1/falso/enviadas", s.falsoEnviadas, false)
	s.rota("PUT /v1/falso/relogio", s.falsoRelogio, false)
	if s.o.Servicos.Automacoes != nil {
		s.registrarRotasFalsoAutomacoes()
	}
}

func erroFalso(err error) error {
	switch {
	case errors.Is(err, falso.ErrContaNaoAberta):
		return erros.Novo(erros.NaoEncontrado, "Conta não encontrada ou não aberta no WhatsApp falso.")
	case errors.Is(err, falso.ErrSemQR):
		return erros.Transicao("A conta não está aguardando a leitura do QR.", "")
	case err != nil:
		return erros.Novo(erros.Validacao, err.Error())
	}
	return nil
}

func (s *Servidor) aguardar(r *http.Request, contaID string) {
	if sinc, ok := s.o.Servicos.Contadores.(Sincronizador); ok {
		ctx, cancelar := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancelar()
		sinc.AguardarEventos(ctx, contaID)
	}
}

func normalizarObrigatorio(campo, bruto string) (string, error) {
	e164, motivo := telefone.Normalizar(bruto, "")
	if motivo != "" {
		return "", erros.Campo(campo, "Telefone inválido ("+motivo+").")
	}
	return e164, nil
}

func (s *Servidor) falsoEscanear(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Telefone string `json:"telefone"`
		Nome     string `json:"nome"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	tel, err := normalizarObrigatorio("telefone", c.Telefone)
	if err != nil {
		return err
	}
	id := r.PathValue("id")
	if err := erroFalso(s.o.Falso.EscanearQR(id, tel, c.Nome)); err != nil {
		return err
	}
	s.aguardar(r, id)
	return responderJSON(w, 200, map[string]any{"ok": true})
}

func (s *Servidor) falsoExpirarQR(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if err := erroFalso(s.o.Falso.ExpirarQR(id)); err != nil {
		return err
	}
	s.aguardar(r, id)
	return responderJSON(w, 200, map[string]any{"ok": true})
}

func (s *Servidor) falsoMensagem(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		De             string     `json:"de"`
		Nome           string     `json:"nome"`
		Texto          string     `json:"texto"`
		GrupoJID       *string    `json:"grupo_jid"`
		GrupoNome      string     `json:"grupo_nome"`
		Tipo           string     `json:"tipo"`
		DeMim          bool       `json:"de_mim"`
		FalharDownload bool       `json:"falhar_download"`
		CitarWaID      string     `json:"citar_wa_id"`
		Em             *time.Time `json:"em"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	tel, err := normalizarObrigatorio("de", c.De)
	if err != nil {
		return err
	}
	rec := falso.Recebida{De: tel, Nome: c.Nome, Texto: c.Texto, GrupoNome: c.GrupoNome, Tipo: c.Tipo,
		DeMim: c.DeMim, FalharDownload: c.FalharDownload, CitarWaID: c.CitarWaID}
	if c.GrupoJID != nil {
		rec.GrupoJID = *c.GrupoJID
	}
	if c.Em != nil {
		rec.Em = *c.Em
	}
	id := r.PathValue("id")
	waID, err := s.o.Falso.InjetarMensagem(id, rec)
	if err := erroFalso(err); err != nil {
		return err
	}
	s.aguardar(r, id)
	return responderJSON(w, 200, map[string]any{"wa_id": waID})
}

func (s *Servidor) falsoRecibo(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		WaID string `json:"wa_id"`
		Tipo string `json:"tipo"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	if c.Tipo != "entregue" && c.Tipo != "lido" {
		return erros.Campo("tipo", "Use \"entregue\" ou \"lido\".")
	}
	id := r.PathValue("id")
	if err := erroFalso(s.o.Falso.InjetarRecibo(id, c.WaID, c.Tipo)); err != nil {
		return err
	}
	s.aguardar(r, id)
	return responderJSON(w, 200, map[string]any{"ok": true})
}

func (s *Servidor) falsoEstado(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Evento string `json:"evento"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	id := r.PathValue("id")
	if err := erroFalso(s.o.Falso.InjetarEstado(id, c.Evento)); err != nil {
		return err
	}
	s.aguardar(r, id)
	return responderJSON(w, 200, map[string]any{"ok": true})
}

func (s *Servidor) falsoHistorico(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Conversas []falso.ConversaHistorico `json:"conversas"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	id := r.PathValue("id")
	if err := erroFalso(s.o.Falso.InjetarHistorico(id, c.Conversas)); err != nil {
		return err
	}
	s.aguardar(r, id)
	return responderJSON(w, 200, map[string]any{"ok": true})
}

func (s *Servidor) falsoStatus(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		De    string `json:"de"`
		Texto string `json:"texto"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	tel, err := normalizarObrigatorio("de", c.De)
	if err != nil {
		return err
	}
	id := r.PathValue("id")
	waID, err := s.o.Falso.InjetarStatus(id, tel, c.Texto)
	if err := erroFalso(err); err != nil {
		return err
	}
	s.aguardar(r, id)
	return responderJSON(w, 200, map[string]any{"wa_id": waID})
}

func normalizarLista(brutos []string) []string {
	var lista []string
	for _, b := range brutos {
		if e164, motivo := telefone.Normalizar(b, ""); motivo == "" {
			lista = append(lista, e164)
		}
	}
	return lista
}

func (s *Servidor) falsoSemWhatsApp(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Telefones []string `json:"telefones"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	s.o.Falso.DefinirSemWhatsApp(normalizarLista(c.Telefones))
	return responderJSON(w, 200, map[string]any{"ok": true})
}

func (s *Servidor) falsoFalhas(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Telefones []string `json:"telefones"`
		Erro      string   `json:"erro"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	s.o.Falso.DefinirFalhas(normalizarLista(c.Telefones), c.Erro)
	return responderJSON(w, 200, map[string]any{"ok": true})
}

func (s *Servidor) falsoEnviadas(w http.ResponseWriter, r *http.Request) error {
	lista := s.o.Falso.Enviadas()
	if lista == nil {
		lista = []falso.Enviada{}
	}
	return responderJSON(w, 200, lista)
}

func (s *Servidor) falsoRelogio(w http.ResponseWriter, r *http.Request) error {
	var c struct {
		Agora    *time.Time `json:"agora"`
		AvancarS *float64   `json:"avancar_s"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	rc := s.o.RelogioControlavel
	if rc == nil {
		return erros.Novo(erros.NaoEncontrado, "Relógio controlável indisponível.")
	}
	switch {
	case c.Agora != nil && c.AvancarS == nil:
		rc.Definir(*c.Agora)
	case c.AvancarS != nil && c.Agora == nil:
		if *c.AvancarS < 0 {
			return erros.Campo("avancar_s", "Use um número de segundos maior ou igual a zero.")
		}
		rc.Avancar(time.Duration(*c.AvancarS * float64(time.Second)))
	default:
		return erros.Campo("agora", "Envie \"agora\" ou \"avancar_s\".")
	}
	return responderJSON(w, 200, map[string]any{"agora": rc.Agora().Format(time.RFC3339Nano)})
}
