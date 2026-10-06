// Pacote disparos implementa os disparos em massa: máquinas de estado (data-model.md ›
// Disparo/Destinatário), validação, fila por conta, executor idempotente e acompanhamento.
package disparos

import (
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
)

// Ações sobre um disparo.
const (
	AcaoIniciar        = "iniciar"
	AcaoPausar         = "pausar"
	AcaoRetomar        = "retomar"
	AcaoCancelar       = "cancelar"
	AcaoEntrarNaJanela = "entrar_na_janela"
	AcaoSairDaJanela   = "sair_da_janela"
	AcaoConcluir       = "concluir"
)

var transicoes = map[string]map[string]string{
	dominio.DisparoRascunho: {AcaoIniciar: dominio.DisparoAgendado, AcaoCancelar: dominio.DisparoCancelado},
	dominio.DisparoAgendado: {AcaoEntrarNaJanela: dominio.DisparoEnviando, AcaoSairDaJanela: dominio.DisparoForaDaJanela,
		AcaoPausar: dominio.DisparoPausado, AcaoCancelar: dominio.DisparoCancelado},
	dominio.DisparoEnviando: {AcaoSairDaJanela: dominio.DisparoForaDaJanela, AcaoConcluir: dominio.DisparoConcluido,
		AcaoPausar: dominio.DisparoPausado, AcaoCancelar: dominio.DisparoCancelado, AcaoEntrarNaJanela: dominio.DisparoEnviando},
	dominio.DisparoForaDaJanela: {AcaoEntrarNaJanela: dominio.DisparoEnviando, AcaoConcluir: dominio.DisparoConcluido,
		AcaoPausar: dominio.DisparoPausado, AcaoCancelar: dominio.DisparoCancelado, AcaoSairDaJanela: dominio.DisparoForaDaJanela},
	dominio.DisparoPausado: {AcaoRetomar: dominio.DisparoAgendado, AcaoCancelar: dominio.DisparoCancelado},
}

var mensagensAcao = map[string]string{
	AcaoIniciar:  "Só é possível iniciar um disparo em rascunho.",
	AcaoPausar:   "Só é possível pausar um disparo agendado ou em andamento.",
	AcaoRetomar:  "Só é possível retomar um disparo pausado.",
	AcaoCancelar: "Este disparo já terminou.",
}

// Transicao devolve o próximo estado ou transicao_invalida (com detalhes.estado_atual).
func Transicao(de, acao string) (string, error) {
	if para, ok := transicoes[de][acao]; ok {
		return para, nil
	}
	msg := mensagensAcao[acao]
	if msg == "" {
		msg = "Essa ação não é permitida no estado atual do disparo."
	}
	return "", erros.Transicao(msg, de)
}

// Origens válidas por transição de destinatário.
var transicoesDest = map[string][]string{
	dominio.DestEnviando:  {dominio.DestPendente},
	dominio.DestEnviado:   {dominio.DestEnviando},
	dominio.DestEntregue:  {dominio.DestEnviado},
	dominio.DestLido:      {dominio.DestEnviado, dominio.DestEntregue},
	dominio.DestFalhou:    {dominio.DestPendente, dominio.DestEnviando},
	dominio.DestRespondeu: {dominio.DestEnviado, dominio.DestEntregue, dominio.DestLido},
}

// TransicaoDestinatarioValida confere a máquina do destinatário (sem regressão).
func TransicaoDestinatarioValida(de, para string) bool {
	for _, d := range transicoesDest[para] {
		if d == de {
			return true
		}
	}
	return false
}

// Ativo: estados que mantêm o app vivo na barra de menu (contracts/eventos-ws.md).
func Ativo(estado string, iniciado bool) bool {
	switch estado {
	case dominio.DisparoEnviando, dominio.DisparoForaDaJanela:
		return true
	case dominio.DisparoAgendado:
		return iniciado
	}
	return false
}

// Final indica estado terminal.
func Final(estado string) bool {
	return estado == dominio.DisparoConcluido || estado == dominio.DisparoCancelado
}
