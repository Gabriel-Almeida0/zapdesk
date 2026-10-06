package integracao

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type Arquivo struct {
	ID        string `json:"id"`
	Nome      string `json:"nome"`
	Mimetype  string `json:"mimetype"`
	Tamanho   int64  `json:"tamanho"`
	TipoMidia string `json:"tipo_midia"`
	URL       string `json:"url"`
}

func (m *Motor) upload(nome string, conteudo []byte) Arquivo {
	m.t.Helper()
	st, b := m.multipart("/v1/arquivos", nome, conteudo)
	if st != 201 {
		m.t.Fatalf("upload %s: %d %s", nome, st, b)
	}
	var a Arquivo
	json.Unmarshal(b, &a)
	return a
}

var pngMinimo = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0x0d, 0x49, 0x48, 0x44, 0x52, 0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0, 0x1f, 0x15, 0xc4, 0x89}

// webmVoz imita a gravação do MediaRecorder (Opus em WebM).
func webmVoz() []byte {
	el := func(id []byte, d []byte, desconhecido bool) []byte {
		b := append([]byte{}, id...)
		if desconhecido {
			b = append(b, 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)
		} else {
			n := len(d)
			b = append(b, 0x01, 0, 0, 0, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
		}
		return append(b, d...)
	}
	cab := el([]byte{0x1A, 0x45, 0xDF, 0xA3}, el([]byte{0x42, 0x82}, []byte("webm"), false), false)
	head := append([]byte("OpusHead"), 1, 1, 0x38, 0x01, 0x80, 0xBB, 0, 0, 0, 0, 0)
	faixa := el([]byte{0xAE}, append(append(el([]byte{0xD7}, []byte{1}, false), el([]byte{0x86}, []byte("A_OPUS"), false)...), el([]byte{0x63, 0xA2}, head, false)...), false)
	var cl []byte
	for i := 0; i < 10; i++ {
		cl = append(cl, el([]byte{0xA3}, append([]byte{0x81, 0, byte(i), 0x80, 0xF8}, bytes.Repeat([]byte{1}, 30)...), false)...)
	}
	seg := append(el([]byte{0x16, 0x54, 0xAE, 0x6B}, faixa, false), el([]byte{0x1F, 0x43, 0xB6, 0x75}, cl, true)...)
	return append(cab, el([]byte{0x18, 0x53, 0x80, 0x67}, seg, true)...)
}

func TestMidiaEInteracoes(t *testing.T) {
	m := novoMotor(t)
	c := m.contaConectada("+5511900000001")
	var conv Conversa
	m.json("POST", "/v1/contas/"+c.ID+"/conversas", map[string]string{"telefone": "11 97777-0000"}, 201, &conv)

	img := m.upload("foto.png", pngMinimo)
	if img.TipoMidia != "imagem" || img.Mimetype != "image/png" || img.URL != "/v1/arquivos/"+img.ID+"/conteudo" {
		t.Fatalf("upload imagem: %+v", img)
	}
	pdf := m.upload("contrato.pdf", []byte("%PDF-1.4\nconteudo"))
	web := m.upload("fig.webp", append([]byte("RIFF\x10\x00\x00\x00WEBPVP8 "), make([]byte, 20)...))
	voz := m.upload("gravacao.webm", webmVoz())
	if pdf.TipoMidia != "documento" || web.TipoMidia != "figurinha" {
		t.Fatalf("tipos: %+v %+v", pdf, web)
	}
	var obt Arquivo
	m.json("GET", "/v1/arquivos/"+img.ID, nil, 200, &obt)
	st, conteudo := m.req("GET", "/v1/arquivos/"+img.ID+"/conteudo?token="+tokenTeste, nil, "Authorization", "")
	if st != 200 || !bytes.Equal(conteudo, pngMinimo) {
		t.Fatalf("conteúdo: %d", st)
	}

	enviar := func(corpo map[string]any) Mensagem {
		t.Helper()
		marca := m.marca()
		var msg Mensagem
		m.json("POST", "/v1/conversas/"+conv.ID+"/mensagens", corpo, 202, &msg)
		return m.esperarMensagem(marca, msg.ID, "enviada")
	}
	mi := enviar(map[string]any{"arquivo_id": img.ID, "texto": "olha a foto"})
	if mi.Tipo != "imagem" || ptr(mi.Texto) != "olha a foto" || mi.Midia == nil || !mi.Midia.Baixada {
		t.Fatalf("imagem: %+v", mi)
	}
	md := enviar(map[string]any{"arquivo_id": pdf.ID})
	if md.Tipo != "documento" || md.Midia == nil || ptr(md.Midia.NomeArquivo) != "contrato.pdf" {
		t.Fatalf("documento: %+v", md.Midia)
	}
	mf := enviar(map[string]any{"arquivo_id": web.ID, "como": "figurinha"})
	mv := enviar(map[string]any{"arquivo_id": voz.ID, "como": "voz"})
	if mf.Tipo != "figurinha" || mv.Tipo != "audio" || !mv.Midia.PTT || !strings.HasPrefix(mv.Midia.Mimetype, "audio/ogg") {
		t.Fatalf("figurinha/voz: %+v / %+v", mf, mv.Midia)
	}
	mdoc := enviar(map[string]any{"arquivo_id": img.ID, "como": "documento"})
	if mdoc.Tipo != "documento" {
		t.Fatalf("como=documento: %s", mdoc.Tipo)
	}
	m.erro("POST", "/v1/conversas/"+conv.ID+"/mensagens", map[string]any{"arquivo_id": pdf.ID, "como": "figurinha"}, 415, "tipo_nao_suportado")
	m.erro("POST", "/v1/conversas/"+conv.ID+"/mensagens", map[string]any{"arquivo_id": "nao-existe"}, 422, "validacao")

	var env []map[string]any
	m.json("GET", "/v1/falso/enviadas", nil, 200, &env)
	tipos := map[string]map[string]any{}
	for _, e := range env {
		tipos[e["tipo"].(string)] = e
	}
	if tipos["imagem"]["texto"] != "olha a foto" || tipos["audio"]["voz"] != true || tipos["audio"]["mimetype"] != "audio/ogg; codecs=opus" || tipos["figurinha"] == nil {
		t.Fatalf("enviadas: %v", tipos)
	}

	var figs []map[string]any
	m.json("GET", "/v1/contas/"+c.ID+"/figurinhas", nil, 200, &figs)
	if len(figs) != 1 || figs[0]["arquivo_id"] != web.ID {
		t.Fatalf("figurinhas recentes: %v", figs)
	}

	// anexo acima do limite
	grande := make([]byte, 16<<20+10)
	copy(grande, pngMinimo)
	st, b := m.multipart("/v1/arquivos", "enorme.png", grande)
	if st != 413 || !strings.Contains(string(b), "anexo_grande_demais") || !strings.Contains(string(b), `"limite_bytes":16777216`) {
		t.Fatalf("limite: %d %s", st, b)
	}

	// citação, reação, edição, apagar
	waRec := m.receber(c.ID, map[string]any{"de": "+5511977770000", "texto": "qual o preço?", "nome": "Cli"})
	var recebida Mensagem
	for _, x := range m.mensagens(conv.ID) {
		if x.WaID == waRec {
			recebida = x
		}
	}
	marca := m.marca()
	var resp Mensagem
	m.json("POST", "/v1/conversas/"+conv.ID+"/mensagens", map[string]any{"texto": "R$ 10", "citar_mensagem_id": recebida.ID}, 202, &resp)
	if resp.Citacao == nil || resp.Citacao.WaID != waRec || resp.Citacao.Resumo != "qual o preço?" {
		t.Fatalf("citação: %+v", resp.Citacao)
	}
	resp = m.esperarMensagem(marca, resp.ID, "enviada")

	if st, _ := m.req("POST", "/v1/mensagens/"+recebida.ID+"/reacao", map[string]string{"emoji": "👍"}); st != 204 {
		t.Fatal("reagir")
	}
	var r1 Mensagem
	for _, x := range m.mensagens(conv.ID) {
		if x.ID == recebida.ID {
			r1 = x
		}
	}
	if len(r1.Reacoes) != 1 || r1.Reacoes[0].Emoji != "👍" || !r1.Reacoes[0].DeMim {
		t.Fatalf("reação: %+v", r1.Reacoes)
	}
	m.req("POST", "/v1/mensagens/"+recebida.ID+"/reacao", map[string]string{"emoji": ""})
	for _, x := range m.mensagens(conv.ID) {
		if x.ID == recebida.ID && len(x.Reacoes) != 0 {
			t.Fatal("reação deveria ser removida")
		}
	}

	var ed Mensagem
	m.json("PATCH", "/v1/mensagens/"+resp.ID, map[string]string{"texto": "R$ 9,90"}, 200, &ed)
	if !ed.Editada || ptr(ed.Texto) != "R$ 9,90" {
		t.Fatalf("editar: %+v", ed)
	}
	resumoDaConversa := func() string {
		for _, cv := range m.conversas(c.ID, "") {
			if cv.ID == conv.ID {
				return ptr(cv.UltimaMensagemResumo)
			}
		}
		return ""
	}
	// A prévia da lista acompanha a edição da última mensagem.
	if r := resumoDaConversa(); r != "R$ 9,90" {
		t.Fatalf("resumo após editar: %q", r)
	}
	m.erro("PATCH", "/v1/mensagens/"+recebida.ID, map[string]string{"texto": "x"}, 409, "transicao_invalida")
	m.erro("PATCH", "/v1/mensagens/"+mi.ID, map[string]string{"texto": "x"}, 409, "transicao_invalida")

	m.Relogio.Avancar(16 * time.Minute)
	m.erro("PATCH", "/v1/mensagens/"+resp.ID, map[string]string{"texto": "tarde"}, 409, "fora_do_prazo")
	if st, _ := m.req("DELETE", "/v1/mensagens/"+resp.ID, nil); st != 204 {
		t.Fatal("apagar dentro de 48 h")
	}
	for _, x := range m.mensagens(conv.ID) {
		if x.ID == resp.ID && (!x.Apagada || x.Texto != nil || x.PodeApagar) {
			t.Fatalf("apagada: %+v", x)
		}
	}
	// O texto apagado não pode continuar na prévia da lista de conversas.
	if r := resumoDaConversa(); r != "Você apagou esta mensagem" {
		t.Fatalf("resumo após apagar: %q", r)
	}
	m.Relogio.Avancar(49 * time.Hour)
	m.erro("DELETE", "/v1/mensagens/"+md.ID, nil, 409, "fora_do_prazo")
	m.erro("DELETE", "/v1/mensagens/"+recebida.ID, nil, 409, "transicao_invalida")

	var fal []map[string]any
	m.json("GET", "/v1/falso/enviadas", nil, 200, &fal)
	contagem := map[string]int{}
	for _, e := range fal {
		contagem[e["tipo"].(string)]++
	}
	if contagem["reacao"] != 2 || contagem["edicao"] != 1 || contagem["apagar"] != 1 {
		t.Fatalf("interações registradas: %v", contagem)
	}

	// download sob demanda com cache; falha → whatsapp_erro
	waImg := m.receber(c.ID, map[string]any{"de": "+5511977770000", "tipo": "imagem", "texto": "veja"})
	waRuim := m.receber(c.ID, map[string]any{"de": "+5511977770000", "tipo": "documento", "falhar_download": true})
	var comMidia, semMidia Mensagem
	for _, x := range m.mensagens(conv.ID) {
		switch x.WaID {
		case waImg:
			comMidia = x
		case waRuim:
			semMidia = x
		}
	}
	if comMidia.Midia == nil || comMidia.Midia.Baixada || comMidia.Midia.URL != "/v1/mensagens/"+comMidia.ID+"/midia" {
		t.Fatalf("mídia recebida: %+v", comMidia.Midia)
	}
	marca = m.marca()
	baixar := func(id string) (int, []byte, string) {
		r, _ := http.NewRequest("GET", m.url("/v1/mensagens/"+id+"/midia?token="+tokenTeste), nil)
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, b, resp.Header.Get("Content-Type")
	}
	st, b, ct := baixar(comMidia.ID)
	if st != 200 || len(b) == 0 || ct != "image/png" {
		t.Fatalf("download: %d %s", st, ct)
	}
	ev := m.esperarEvento(marca, "mensagem.atualizada", func(e Evento) bool { return dados[Mensagem](t, e).ID == comMidia.ID })
	if !dados[Mensagem](t, ev).Midia.Baixada {
		t.Fatal("deveria marcar baixada")
	}
	m.json("POST", "/v1/falso/contas/"+c.ID+"/estado", map[string]string{"evento": "queda_rede"}, 200, nil)
	if st, _, _ = baixar(comMidia.ID); st != 200 {
		t.Fatal("segunda vez vem do cache, mesmo sem rede")
	}
	m.json("POST", "/v1/falso/contas/"+c.ID+"/estado", map[string]string{"evento": "reconectou"}, 200, nil)
	st, b, _ = baixar(semMidia.ID)
	if st != 502 || !strings.Contains(string(b), "whatsapp_erro") || !strings.Contains(string(b), "Não foi possível baixar.") {
		t.Fatalf("falha de download: %d %s", st, b)
	}
}
