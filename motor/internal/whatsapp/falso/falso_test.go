package falso

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"zapdesk/motor/internal/relogio"
	"zapdesk/motor/internal/whatsapp"
)

var ctx = context.Background()

func receber(t *testing.T, cli whatsapp.Cliente) whatsapp.Evento {
	t.Helper()
	select {
	case e := <-cli.Eventos():
		return e
	case <-time.After(time.Second):
		t.Fatal("sem evento")
	}
	return whatsapp.Evento{}
}

func parear(t *testing.T, c *Controle, conta, tel string) whatsapp.Cliente {
	t.Helper()
	cli, _ := c.Abrir(ctx, conta)
	qr, err := cli.CanalQR(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := cli.Conectar(ctx); err != nil {
		t.Fatal(err)
	}
	if e := <-qr; e.Tipo != whatsapp.QRCodigo || e.Codigo == "" {
		t.Fatalf("qr: %+v", e)
	}
	if err := c.EscanearQR(conta, tel, "Teste"); err != nil {
		t.Fatal(err)
	}
	if e := <-qr; e.Tipo != whatsapp.QRSucesso {
		t.Fatalf("esperado sucesso: %+v", e)
	}
	if e := receber(t, cli); e.Estado == nil || e.Estado.Estado != whatsapp.EstadoConectada {
		t.Fatalf("esperado conectada: %+v", e)
	}
	return cli
}

func TestPareamentoPersisteEntreReinicios(t *testing.T) {
	pasta := t.TempDir()
	c, _ := Novo(pasta, relogio.NovoCongelado(time.Now()))
	cli := parear(t, c, "c1", "+5511900000001")
	if !cli.Pareado() {
		t.Fatal("deveria estar pareado")
	}
	cli.Desconectar()
	if _, ok := <-cli.Eventos(); ok {
		t.Fatal("canal deveria fechar")
	}

	c2, _ := Novo(pasta, nil)
	cli2, _ := c2.Abrir(ctx, "c1")
	if !cli2.Pareado() {
		t.Fatal("sessão não persistiu")
	}
	cli2.Conectar(ctx)
	e := receber(t, cli2)
	if e.Estado == nil || e.Estado.Estado != whatsapp.EstadoConectada || e.Estado.Proprio.Usuario != "5511900000001" {
		t.Fatalf("restauração: %+v", e.Estado)
	}
}

func TestEnviadasPersistemEFalhas(t *testing.T) {
	pasta := t.TempDir()
	c, _ := Novo(pasta, nil)
	cli := parear(t, c, "c1", "+5511900000001")
	para := whatsapp.JIDDeTelefone("+5511911112222")
	env, err := cli.EnviarTexto(ctx, para, "oi", nil)
	if err != nil || env.WaID == "" {
		t.Fatal(err)
	}
	c.DefinirSemWhatsApp([]string{"+5511933334444"})
	if _, ok, _ := cli.TemWhatsApp(ctx, "+5511933334444"); ok {
		t.Fatal("deveria não ter WhatsApp")
	}
	if _, err := cli.EnviarTexto(ctx, whatsapp.JIDDeTelefone("+5511933334444"), "x", nil); !errors.Is(err, whatsapp.ErrSemWhatsApp) {
		t.Fatalf("esperado ErrSemWhatsApp: %v", err)
	}
	c.DefinirFalhas([]string{"+5511955556666"}, "servidor recusou")
	if _, err := cli.EnviarTexto(ctx, whatsapp.JIDDeTelefone("+5511955556666"), "x", nil); err == nil || err.Error() != "servidor recusou" {
		t.Fatalf("falha configurada: %v", err)
	}
	c.InjetarEstado("c1", EventoQuedaRede)
	receber(t, cli)
	if _, err := cli.EnviarTexto(ctx, para, "x", nil); !errors.Is(err, whatsapp.ErrDesconectado) {
		t.Fatalf("esperado ErrDesconectado: %v", err)
	}

	c2, _ := Novo(pasta, nil)
	env2 := c2.Enviadas()
	if len(env2) != 1 || env2[0].Telefone != "+5511911112222" || env2[0].Texto != "oi" {
		t.Fatalf("enviadas após reinício: %+v", env2)
	}
}

func TestInjecoes(t *testing.T) {
	c, _ := Novo(t.TempDir(), nil)
	cli := parear(t, c, "c1", "+5511900000001")
	waID, err := c.InjetarMensagem("c1", Recebida{De: "+5511911112222", Texto: "oi", Nome: "Ana"})
	if err != nil {
		t.Fatal(err)
	}
	e := receber(t, cli)
	if e.Mensagem == nil || e.Mensagem.WaID != waID || e.Mensagem.Texto != "oi" || e.Mensagem.Chat.Usuario != "5511911112222" {
		t.Fatalf("mensagem: %+v", e.Mensagem)
	}
	c.InjetarMensagem("c1", Recebida{De: "+5511911112222", Texto: "no grupo", GrupoJID: "123@g.us"})
	e = receber(t, cli)
	if !e.Mensagem.Grupo || e.Mensagem.Chat.String() != "123@g.us" {
		t.Fatalf("grupo: %+v", e.Mensagem)
	}
	c.InjetarMensagem("c1", Recebida{De: "+5511911112222", Tipo: "imagem"})
	e = receber(t, cli)
	rc, err := cli.Baixar(ctx, e.Mensagem.Midia.Chave)
	if err != nil {
		t.Fatal(err)
	}
	dados, _ := io.ReadAll(rc)
	if len(dados) == 0 {
		t.Fatal("mídia vazia")
	}
	c.InjetarMensagem("c1", Recebida{De: "+5511911112222", Tipo: "imagem", FalharDownload: true})
	e = receber(t, cli)
	if _, err := cli.Baixar(ctx, e.Mensagem.Midia.Chave); !errors.Is(err, whatsapp.ErrMidiaExpirada) {
		t.Fatalf("download deveria falhar: %v", err)
	}
	c.InjetarStatus("c1", "+5511911112222", "status!")
	e = receber(t, cli)
	if !e.Mensagem.Chat.Status() {
		t.Fatal("status")
	}
	c.InjetarRecibo("c1", "X", "lido")
	if e = receber(t, cli); e.Recibo == nil || e.Recibo.Tipo != "lido" {
		t.Fatal("recibo")
	}

	var msgs []MensagemHistorico
	for i := 0; i < 1200; i++ {
		msgs = append(msgs, MensagemHistorico{Texto: "h"})
	}
	c.InjetarHistorico("c1", []ConversaHistorico{{JID: "+5511977778888", Mensagens: msgs}})
	total, lotes := 0, 0
	for {
		e = receber(t, cli)
		lotes++
		for _, conv := range e.Historico.Conversas {
			total += len(conv.Mensagens)
		}
		if e.Historico.Final {
			break
		}
	}
	if total != 1200 || lotes != 3 {
		t.Fatalf("histórico: %d mensagens em %d lotes", total, lotes)
	}

	c.InjetarEstado("c1", EventoLogout)
	if e = receber(t, cli); e.Estado.Estado != whatsapp.EstadoDesconectada || cli.Pareado() {
		t.Fatal("logout")
	}
}

// A figurinha falsa precisa ser um WebP bem formado (o app a exibe com <img>).
func TestMidiaFalsaFigurinhaEhWebPValido(t *testing.T) {
	mime, _, dados := midiaFalsa(whatsapp.TipoFigurinha)
	if mime != "image/webp" {
		t.Fatalf("mimetype = %q", mime)
	}
	if len(dados) < 20 || string(dados[0:4]) != "RIFF" || string(dados[8:12]) != "WEBP" || string(dados[12:15]) != "VP8" {
		t.Fatalf("cabeçalho WebP inválido: %q", dados)
	}
	tamanho := int(dados[4]) | int(dados[5])<<8 | int(dados[6])<<16 | int(dados[7])<<24
	if tamanho != len(dados)-8 {
		t.Fatalf("tamanho RIFF = %d, esperado %d", tamanho, len(dados)-8)
	}
}
