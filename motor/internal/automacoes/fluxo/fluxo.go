// Pacote fluxo executa automações do tipo fluxo (data-model.md › Máquina do executor de fluxo):
// condições → ações em ordem, com o passo persistido antes de cada ação, "aguardar" gravado como
// espera (a execução cede e é retomada pelo agendador no passo seguinte), falhas não fatais que
// continuam o fluxo e simulação que registra tudo sem escrever nem esperar.
package fluxo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"zapdesk/motor/internal/automacoes/acoes"
	"zapdesk/motor/internal/automacoes/condicoes"
	"zapdesk/motor/internal/automacoes/execucoes"
	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/relogio"
)

// Esperas grava a espera "aguardar" de uma execução.
type Esperas interface {
	Gravar(ctx context.Context, e dominio.Espera) error
}

// Executor de fluxos.
type Executor struct {
	acoes   *acoes.Executores
	esperas Esperas
	relogio relogio.Relogio
}

// Novo cria o executor.
func Novo(a *acoes.Executores, e Esperas, r relogio.Relogio) *Executor {
	return &Executor{acoes: a, esperas: e, relogio: r}
}

// MotivoCondicoes é o motivo das execuções "ok" que pararam nas condições.
const MotivoCondicoes = "condições não atendidas"

// Executar começa a execução (estado "na_fila" → "rodando").
func (x *Executor) Executar(ctx context.Context, e *execucoes.Exec, a dominio.Automacao, c *acoes.Contexto) error {
	def, errs := modelo.DecodificarFluxo(a.Definicao)
	if errs != nil {
		e.Iniciar(ctx)
		return e.Finalizar(ctx, execucoes.Fim{Estado: dominio.ExecErro, Erro: "Definição do fluxo inválida: " + errs[0].Mensagem})
	}
	if err := e.Iniciar(ctx); err != nil {
		return err
	}
	if !condicoes.Avaliar(def.Condicoes, x.acoes.DadosCondicoes(ctx, c)) {
		e.Logar("info", "Condições não atendidas.")
		return e.Finalizar(ctx, execucoes.Fim{Estado: final(c, dominio.ExecOK), Motivo: MotivoCondicoes})
	}
	return x.rodar(ctx, e, def, c, 0)
}

// Retomar continua uma execução que estava aguardando (chamado pelo agendador).
func (x *Executor) Retomar(ctx context.Context, e *execucoes.Exec, a dominio.Automacao, c *acoes.Contexto) error {
	if e.Detalhe().AutomacaoVersao != a.Versao {
		return e.Finalizar(ctx, execucoes.Fim{Estado: dominio.ExecAbortada, Motivo: "a automação foi alterada durante a espera"})
	}
	def, errs := modelo.DecodificarFluxo(a.Definicao)
	if errs != nil {
		return e.Finalizar(ctx, execucoes.Fim{Estado: dominio.ExecErro, Erro: "Definição do fluxo inválida."})
	}
	if err := e.Retomar(ctx); err != nil {
		return err
	}
	return x.rodar(ctx, e, def, c, e.Passo())
}

func final(c *acoes.Contexto, estado string) string {
	if c.Simulacao && estado == dominio.ExecOK {
		return dominio.ExecSimulacao
	}
	return estado
}

func (x *Executor) rodar(ctx context.Context, e *execucoes.Exec, def *modelo.DefinicaoFluxo, c *acoes.Contexto, inicio int) error {
	for i := inicio; i < len(def.Acoes); i++ {
		if ctx.Err() != nil {
			return e.Finalizar(context.WithoutCancel(ctx), execucoes.Fim{Estado: dominio.ExecAbortada, Motivo: "app fechado"})
		}
		a := def.Acoes[i]
		if !c.Simulacao {
			if err := e.DefinirPasso(ctx, i); err != nil {
				return err
			}
		}
		if a.Tipo == modelo.AcaoAguardar {
			dur := time.Duration(a.DuracaoS) * time.Second
			if c.Simulacao {
				e.RegistrarAcao(ctx, dominio.AcaoRegistrada{Tipo: a.Tipo, Resultado: dominio.AcaoSimulada, Detalhe: ptr("Aguardaria " + Duracao(dur) + ".")})
				continue
			}
			retomar := x.relogio.Agora().Add(dur)
			esp := dominio.Espera{Tipo: dominio.EsperaAguardar, AutomacaoID: c.Automacao.ID, RetomarEm: retomar}
			id := e.ID()
			esp.ExecucaoID = &id
			if c.Alvo.ConversaID != "" {
				cv := c.Alvo.ConversaID
				esp.ConversaID = &cv
			}
			e.RegistrarAcao(ctx, dominio.AcaoRegistrada{Tipo: a.Tipo, Resultado: dominio.AcaoOK, Detalhe: ptr("Aguardando " + Duracao(dur) + " (até " + retomar.Format("02/01 15:04") + ").")})
			if err := e.Aguardar(ctx, retomar, i+1); err != nil {
				return err
			}
			return x.esperas.Gravar(ctx, esp)
		}
		r := x.acoes.Executar(ctx, c, a)
		if a.ID != "" && r.Registro.Alvo == nil {
			r.Registro.Alvo = nil
		}
		e.RegistrarAcao(ctx, r.Registro)
		if r.Fatal != nil {
			return e.Finalizar(ctx, execucoes.Fim{Estado: dominio.ExecErro, Erro: "Erro interno na ação " + a.Tipo + ": " + r.Fatal.Error()})
		}
		if a.Tipo == modelo.AcaoExecutarIA && a.SalvarEm != nil && r.Retorno != nil {
			v := textoDoRetorno(r.Retorno)
			if c.Variaveis == nil {
				c.Variaveis = map[string]string{}
			}
			c.Variaveis[*a.SalvarEm] = v
			e.DefinirVariavel(ctx, *a.SalvarEm, v)
		}
	}
	return e.Finalizar(ctx, execucoes.Fim{Estado: final(c, dominio.ExecOK)})
}

// textoDoRetorno: string → o próprio texto; outros JSON → o JSON.
func textoDoRetorno(r json.RawMessage) string {
	var s string
	if json.Unmarshal(r, &s) == nil {
		return s
	}
	return string(r)
}

// Duracao formata uma duração em pt-BR curto ("2 h", "30 min", "1 h 30 min", "45 s").
func Duracao(d time.Duration) string {
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	s := int((d % time.Minute) / time.Second)
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%d h %d min", h, m)
	case h > 0:
		return fmt.Sprintf("%d h", h)
	case m > 0:
		return fmt.Sprintf("%d min", m)
	}
	return fmt.Sprintf("%d s", s)
}

func ptr(s string) *string { return &s }
