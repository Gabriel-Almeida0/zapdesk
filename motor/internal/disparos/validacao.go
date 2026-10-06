package disparos

import (
	"strings"
	"time"
	"unicode/utf8"

	"zapdesk/motor/internal/agendamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/leads"
)

// FalhasSeguidasPadrao pausa o disparo após 10 falhas seguidas.
const FalhasSeguidasPadrao = 10

// Destinatarios de um novo disparo (união deduplicada por lead).
type Destinatarios struct {
	LeadIDs     []string               `json:"lead_ids"`
	EtiquetaIDs []string               `json:"etiqueta_ids"`
	ContatoIDs  []string               `json:"contato_ids"`
	Importar    *leads.CorpoImportacao `json:"importar"`
}

// NovoDisparo é o corpo de POST /v1/disparos e /v1/disparos/validar.
type NovoDisparo struct {
	ContaID           string            `json:"conta_id"`
	Nome              *string           `json:"nome"`
	Mensagem          *string           `json:"mensagem"`
	TemplateID        *string           `json:"template_id"`
	ArquivoID         *string           `json:"arquivo_id"`
	Destinatarios     Destinatarios     `json:"destinatarios"`
	Ritmo             *dominio.Ritmo    `json:"ritmo"`
	InicioEm          *time.Time        `json:"inicio_em"`
	Janela            *dominio.Janela   `json:"janela"`
	FalhasSeguidasMax *int              `json:"falhas_seguidas_max"`
	ValoresPadrao     map[string]string `json:"valores_padrao"`
	Iniciar           bool              `json:"iniciar"`
	Origem            string            `json:"origem"`
}

// ValidarCampos aplica as restrições literais de contracts/api-http.md › Disparos. `mensagem`
// já deve estar resolvida (texto do template, se for o caso).
func ValidarCampos(n NovoDisparo, mensagem string) map[string]string {
	c := map[string]string{}
	if strings.TrimSpace(n.ContaID) == "" {
		c["conta_id"] = "Escolha a conta que vai enviar."
	}
	if n.Nome != nil {
		if l := utf8.RuneCountInString(strings.TrimSpace(*n.Nome)); l < 1 || l > 80 {
			c["nome"] = "O nome deve ter de 1 a 80 caracteres."
		}
	}
	if strings.TrimSpace(mensagem) == "" {
		c["mensagem"] = "Escreva a mensagem do disparo."
	} else if utf8.RuneCountInString(mensagem) > 4096 {
		c["mensagem"] = "A mensagem pode ter até 4.096 caracteres."
	}
	if n.Ritmo == nil {
		c["ritmo.intervalo_min_s"] = "Informe o intervalo mínimo entre mensagens (em segundos)."
	} else {
		r := n.Ritmo
		if r.IntervaloMinS < 1 {
			c["ritmo.intervalo_min_s"] = "O intervalo mínimo deve ser de pelo menos 1 segundo."
		}
		if r.IntervaloMaxS < r.IntervaloMinS {
			c["ritmo.intervalo_max_s"] = "O intervalo máximo deve ser maior ou igual ao mínimo."
		}
		if r.LimitePorHora != nil && *r.LimitePorHora <= 0 {
			c["ritmo.limite_por_hora"] = "O limite por hora deve ser maior que zero."
		}
		if r.LimitePorDia != nil && *r.LimitePorDia <= 0 {
			c["ritmo.limite_por_dia"] = "O limite por dia deve ser maior que zero."
		}
		if r.PausaACada != nil && *r.PausaACada <= 0 {
			c["ritmo.pausa_a_cada"] = "A pausa deve acontecer a cada 1 ou mais mensagens."
		}
		if r.PausaDuracaoS != nil && *r.PausaDuracaoS <= 0 {
			c["ritmo.pausa_duracao_s"] = "A duração da pausa deve ser maior que zero."
		}
		if (r.PausaACada == nil) != (r.PausaDuracaoS == nil) {
			if r.PausaACada == nil {
				c["ritmo.pausa_a_cada"] = "Informe a cada quantas mensagens pausar."
			} else {
				c["ritmo.pausa_duracao_s"] = "Informe a duração da pausa."
			}
		}
	}
	if n.Janela != nil {
		if n.Janela.Inicio == "" || n.Janela.Fim == "" {
			c["janela"] = "Informe o início e o fim da janela de envio."
		} else if _, err := agendamento.ParseJanela(n.Janela.Inicio, n.Janela.Fim); err != nil {
			c["janela"] = "Janela inválida: use HH:MM e horários diferentes."
		}
	}
	if n.FalhasSeguidasMax != nil && *n.FalhasSeguidasMax < 0 {
		c["falhas_seguidas_max"] = "Use 0 (desliga) ou um número positivo."
	}
	if n.Origem != "" && n.Origem != "app" && n.Origem != "mcp" {
		c["origem"] = "Origem inválida (use app ou mcp)."
	}
	return c
}

// AvisoRitmoAgressivo: intervalo_min < 10, limite_por_hora > 120, limite_por_dia > 1000 ou nenhum
// limite definido (apenas aviso).
func AvisoRitmoAgressivo(r dominio.Ritmo) bool {
	return r.IntervaloMinS < 10 ||
		(r.LimitePorHora != nil && *r.LimitePorHora > 120) ||
		(r.LimitePorDia != nil && *r.LimitePorDia > 1000) ||
		(r.LimitePorHora == nil && r.LimitePorDia == nil)
}

// ConfigAgenda converte o disparo na configuração do agendador.
func ConfigAgenda(d dominio.Disparo) agendamento.Config {
	c := agendamento.Config{
		IntervaloMin: time.Duration(d.Ritmo.IntervaloMinS) * time.Second,
		IntervaloMax: time.Duration(d.Ritmo.IntervaloMaxS) * time.Second,
	}
	if d.Ritmo.LimitePorHora != nil {
		c.LimitePorHora = *d.Ritmo.LimitePorHora
	}
	if d.Ritmo.LimitePorDia != nil {
		c.LimitePorDia = *d.Ritmo.LimitePorDia
	}
	if d.Ritmo.PausaACada != nil && d.Ritmo.PausaDuracaoS != nil {
		c.PausaACada = *d.Ritmo.PausaACada
		c.PausaDuracao = time.Duration(*d.Ritmo.PausaDuracaoS) * time.Second
	}
	if d.InicioEm != nil {
		c.InicioEm = *d.InicioEm
	}
	if d.Janela != nil {
		c.Janela, _ = agendamento.ParseJanela(d.Janela.Inicio, d.Janela.Fim)
	}
	return c
}
