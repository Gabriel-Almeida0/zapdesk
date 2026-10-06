package disparos

import (
	"testing"

	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
)

func TestTransicoesDisparo(t *testing.T) {
	validas := []struct{ de, acao, para string }{
		{dominio.DisparoRascunho, AcaoIniciar, dominio.DisparoAgendado},
		{dominio.DisparoAgendado, AcaoEntrarNaJanela, dominio.DisparoEnviando},
		{dominio.DisparoAgendado, AcaoSairDaJanela, dominio.DisparoForaDaJanela},
		{dominio.DisparoEnviando, AcaoSairDaJanela, dominio.DisparoForaDaJanela},
		{dominio.DisparoForaDaJanela, AcaoEntrarNaJanela, dominio.DisparoEnviando},
		{dominio.DisparoEnviando, AcaoConcluir, dominio.DisparoConcluido},
		{dominio.DisparoForaDaJanela, AcaoConcluir, dominio.DisparoConcluido},
		{dominio.DisparoAgendado, AcaoPausar, dominio.DisparoPausado},
		{dominio.DisparoEnviando, AcaoPausar, dominio.DisparoPausado},
		{dominio.DisparoForaDaJanela, AcaoPausar, dominio.DisparoPausado},
		{dominio.DisparoPausado, AcaoRetomar, dominio.DisparoAgendado},
		{dominio.DisparoRascunho, AcaoCancelar, dominio.DisparoCancelado},
		{dominio.DisparoAgendado, AcaoCancelar, dominio.DisparoCancelado},
		{dominio.DisparoEnviando, AcaoCancelar, dominio.DisparoCancelado},
		{dominio.DisparoForaDaJanela, AcaoCancelar, dominio.DisparoCancelado},
		{dominio.DisparoPausado, AcaoCancelar, dominio.DisparoCancelado},
	}
	for _, v := range validas {
		para, err := Transicao(v.de, v.acao)
		if err != nil || para != v.para {
			t.Errorf("%s --%s--> esperado %s, veio %s %v", v.de, v.acao, v.para, para, err)
		}
	}
	invalidas := []struct{ de, acao string }{
		{dominio.DisparoRascunho, AcaoPausar}, {dominio.DisparoRascunho, AcaoRetomar},
		{dominio.DisparoAgendado, AcaoIniciar}, {dominio.DisparoAgendado, AcaoRetomar},
		{dominio.DisparoEnviando, AcaoIniciar}, {dominio.DisparoPausado, AcaoPausar},
		{dominio.DisparoPausado, AcaoIniciar}, {dominio.DisparoConcluido, AcaoCancelar},
		{dominio.DisparoConcluido, AcaoRetomar}, {dominio.DisparoCancelado, AcaoRetomar},
		{dominio.DisparoCancelado, AcaoCancelar}, {dominio.DisparoRascunho, AcaoConcluir},
		{dominio.DisparoPausado, AcaoConcluir},
	}
	for _, v := range invalidas {
		if _, err := Transicao(v.de, v.acao); !erros.Eh(err, erros.TransicaoInvalida) {
			t.Errorf("%s --%s--> deveria ser transicao_invalida, veio %v", v.de, v.acao, err)
		}
	}
	e, _ := erros.Como(func() error { _, err := Transicao(dominio.DisparoConcluido, AcaoPausar); return err }())
	if e.Detalhes["estado_atual"] != dominio.DisparoConcluido {
		t.Fatalf("detalhes.estado_atual: %v", e.Detalhes)
	}
}

func TestTransicoesDestinatario(t *testing.T) {
	validas := [][2]string{
		{dominio.DestPendente, dominio.DestEnviando}, {dominio.DestEnviando, dominio.DestEnviado},
		{dominio.DestEnviado, dominio.DestEntregue}, {dominio.DestEntregue, dominio.DestLido},
		{dominio.DestEnviado, dominio.DestLido}, {dominio.DestPendente, dominio.DestFalhou},
		{dominio.DestEnviando, dominio.DestFalhou}, {dominio.DestEnviado, dominio.DestRespondeu},
		{dominio.DestEntregue, dominio.DestRespondeu}, {dominio.DestLido, dominio.DestRespondeu},
	}
	for _, v := range validas {
		if !TransicaoDestinatarioValida(v[0], v[1]) {
			t.Errorf("%s → %s deveria ser válida", v[0], v[1])
		}
	}
	invalidas := [][2]string{
		{dominio.DestPendente, dominio.DestEnviado}, {dominio.DestPendente, dominio.DestRespondeu},
		{dominio.DestEnviando, dominio.DestRespondeu}, {dominio.DestLido, dominio.DestEntregue},
		{dominio.DestEntregue, dominio.DestEnviado}, {dominio.DestFalhou, dominio.DestEnviado},
		{dominio.DestFalhou, dominio.DestPendente}, {dominio.DestRespondeu, dominio.DestLido},
		{dominio.DestEnviado, dominio.DestFalhou}, {dominio.DestEnviado, dominio.DestEnviando},
	}
	for _, v := range invalidas {
		if TransicaoDestinatarioValida(v[0], v[1]) {
			t.Errorf("%s → %s deveria ser inválida", v[0], v[1])
		}
	}
}

func TestAtivo(t *testing.T) {
	iniciado := true
	if !Ativo(dominio.DisparoEnviando, iniciado) || !Ativo(dominio.DisparoForaDaJanela, iniciado) || !Ativo(dominio.DisparoAgendado, iniciado) {
		t.Fatal("ativos")
	}
	if Ativo(dominio.DisparoPausado, iniciado) || Ativo(dominio.DisparoAgendado, false) || Ativo(dominio.DisparoRascunho, false) {
		t.Fatal("pausado e rascunho não mantêm o app vivo")
	}
}
