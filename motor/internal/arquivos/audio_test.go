package arquivos

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

// ebml monta um elemento (id já com marcador; tamanho -1 = desconhecido).
func ebml(id []byte, dados []byte, desconhecido bool) []byte {
	b := append([]byte{}, id...)
	if desconhecido {
		b = append(b, 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)
	} else {
		n := len(dados)
		b = append(b, 0x01, byte(n>>48), byte(n>>40), byte(n>>32), byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	}
	return append(b, dados...)
}

// webmSintetico imita o que o MediaRecorder do Chromium grava: Segment e Cluster com tamanho
// desconhecido, faixa A_OPUS com OpusHead e SimpleBlocks sem lacing (quadros CELT de 20 ms).
func webmSintetico(blocos int) []byte {
	cabecalho := ebml([]byte{0x1A, 0x45, 0xDF, 0xA3}, ebml([]byte{0x42, 0x82}, []byte("webm"), false), false)
	opusHead := cabecalhoOpus(1)
	faixa := ebml([]byte{0xAE}, append(append(append(
		ebml([]byte{0xD7}, []byte{1}, false),
		ebml([]byte{0x86}, []byte("A_OPUS"), false)...),
		ebml([]byte{0x63, 0xA2}, opusHead, false)...),
		ebml([]byte{0xE1}, ebml([]byte{0x9F}, []byte{1}, false), false)...), false)
	tracks := ebml([]byte{0x16, 0x54, 0xAE, 0x6B}, faixa, false)
	info := ebml([]byte{0x15, 0x49, 0xA9, 0x66}, ebml([]byte{0x2A, 0xD7, 0xB1}, []byte{0x0F, 0x42, 0x40}, false), false)
	var cluster []byte
	cluster = append(cluster, ebml([]byte{0xE7}, []byte{0}, false)...)
	for i := 0; i < blocos; i++ {
		pacote := append([]byte{0xF8}, bytes.Repeat([]byte{byte(i)}, 40+i)...) // cfg 31 (CELT 20 ms), 1 quadro
		bloco := append([]byte{0x81, 0x00, byte(i * 20), 0x80}, pacote...)
		cluster = append(cluster, ebml([]byte{0xA3}, bloco, false)...)
	}
	segmento := append(append(info, tracks...), ebml([]byte{0x1F, 0x43, 0xB6, 0x75}, cluster, true)...)
	return append(cabecalho, ebml([]byte{0x18, 0x53, 0x80, 0x67}, segmento, true)...)
}

type pagina struct {
	tipo    byte
	granula int64
	seq     uint32
	dados   []byte
}

func lerPaginas(t *testing.T, ogg []byte) []pagina {
	t.Helper()
	var ps []pagina
	for len(ogg) > 0 {
		if len(ogg) < 27 || string(ogg[:4]) != "OggS" {
			t.Fatalf("página sem OggS")
		}
		nseg := int(ogg[26])
		tam := 0
		for _, l := range ogg[27 : 27+nseg] {
			tam += int(l)
		}
		total := 27 + nseg + tam
		pg := append([]byte{}, ogg[:total]...)
		crc := binary.LittleEndian.Uint32(pg[22:])
		binary.LittleEndian.PutUint32(pg[22:], 0)
		if CRCOgg(pg) != crc {
			t.Fatalf("CRC inválido na página %d", len(ps))
		}
		ps = append(ps, pagina{tipo: ogg[5], granula: int64(binary.LittleEndian.Uint64(ogg[6:])), seq: binary.LittleEndian.Uint32(ogg[18:]),
			dados: ogg[27+nseg : total]})
		ogg = ogg[total:]
	}
	return ps
}

func TestRemuxWebMParaOgg(t *testing.T) {
	webm := webmSintetico(25)
	os.WriteFile(t.TempDir()+"/voz.webm", webm, 0o600)
	var saida bytes.Buffer
	if err := RemuxWebMParaOgg(bytes.NewReader(webm), &saida); err != nil {
		t.Fatal(err)
	}
	ps := lerPaginas(t, saida.Bytes())
	if len(ps) != 27 {
		t.Fatalf("páginas: %d", len(ps))
	}
	if ps[0].tipo != 0x02 || string(ps[0].dados[:8]) != "OpusHead" || string(ps[1].dados[:8]) != "OpusTags" {
		t.Fatal("cabeçalhos Opus")
	}
	if ps[len(ps)-1].tipo != 0x04 || ps[len(ps)-1].granula != 25*960 {
		t.Fatalf("última página: tipo %x grânulo %d", ps[len(ps)-1].tipo, ps[len(ps)-1].granula)
	}
	for i, p := range ps {
		if p.seq != uint32(i) {
			t.Fatal("sequência")
		}
	}
	if ps[2].dados[0] != 0xF8 || len(ps[2].dados) != 41 {
		t.Fatal("pacote Opus copiado sem recodificar")
	}
	if DetectarMimetype(saida.Bytes()[:512], "voz.ogg", "") != "audio/ogg" {
		t.Fatalf("mimetype do resultado: %s", DetectarMimetype(saida.Bytes()[:512], "voz.ogg", ""))
	}
}

func TestRemuxRejeitaOutrosFormatos(t *testing.T) {
	if err := RemuxWebMParaOgg(bytes.NewReader([]byte("não é webm")), &bytes.Buffer{}); err == nil {
		t.Fatal("deveria falhar")
	}
}

func TestAmostrasOpus(t *testing.T) {
	casos := map[byte]int{0xF8: 960, 0x08: 960, 0x00: 480, 0x18: 2880, 0xF9: 1920, 0x80: 120}
	for toc, esperado := range casos {
		if n := AmostrasOpus([]byte{toc, 0}); n != esperado {
			t.Errorf("toc %#x: %d, esperado %d", toc, n, esperado)
		}
	}
	if AmostrasOpus([]byte{0xFB, 0x03}) != 3*960 {
		t.Error("código 3 com 3 quadros")
	}
}
