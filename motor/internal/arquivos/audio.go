package arquivos

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"zapdesk/motor/internal/dominio"
)

// Remux WebM/Opus (MediaRecorder do Chromium) → OGG/Opus (mensagem de voz do WhatsApp) sem
// recodificar: lê os blocos Opus do Matroska e os grava em páginas Ogg (RFC 7845). Implementação
// própria e enxuta no lugar de ebml-go + oggwriter (research.md §6): evita puxar o módulo
// inteiro do pion/webrtc para ~250 linhas de formato.

// ErrWebMNaoSuportado indica WebM fora do que o remux entende (outro codec, lacing etc.).
var ErrWebMNaoSuportado = errors.New("webm não suportado para mensagem de voz")

const (
	idEBML        = 0x1A45DFA3
	idSegmento    = 0x18538067
	idTracks      = 0x1654AE6B
	idTrackEntry  = 0xAE
	idTrackNumber = 0xD7
	idCodecID     = 0x86
	idCodecPriv   = 0x63A2
	idAudio       = 0xE1
	idCanais      = 0x9F
	idCluster     = 0x1F43B675
	idSimpleBlock = 0xA3
	idBlockGroup  = 0xA0
	idBlock       = 0xA1
)

// contêineres que o leitor percorre por dentro.
var conteineres = map[uint64]bool{idSegmento: true, idTracks: true, idTrackEntry: true, idCluster: true, idBlockGroup: true, idAudio: true}

func lerVint(r *bufio.Reader, manterMarcador bool) (uint64, int, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, 0, err
	}
	tam := 1
	for mascara := byte(0x80); tam <= 8 && b&mascara == 0; mascara >>= 1 {
		tam++
	}
	if tam > 8 {
		return 0, 0, ErrWebMNaoSuportado
	}
	v := uint64(b)
	if !manterMarcador {
		v &= uint64(0xFF >> tam)
	}
	for i := 1; i < tam; i++ {
		c, err := r.ReadByte()
		if err != nil {
			return 0, 0, err
		}
		v = v<<8 | uint64(c)
	}
	return v, tam, nil
}

type faixa struct {
	numero  uint64
	codec   string
	privado []byte
	canais  int
}

// RemuxWebMParaOgg converte o áudio Opus de um WebM num OGG/Opus.
func RemuxWebMParaOgg(entrada io.Reader, saida io.Writer) error {
	r := bufio.NewReader(entrada)
	var faixas []*faixa
	var atual *faixa
	var pacotes [][]byte
	numeroOpus := uint64(0)
	for {
		id, _, err := lerVint(r, true)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		tam, n, err := lerVint(r, false)
		if err != nil {
			return err
		}
		desconhecido := tam == (uint64(1)<<(7*n))-1
		if conteineres[id] {
			if id == idTrackEntry {
				atual = &faixa{canais: 1}
				faixas = append(faixas, atual)
			}
			continue // entra no contêiner
		}
		if desconhecido {
			return ErrWebMNaoSuportado
		}
		if tam > 64<<20 {
			return ErrWebMNaoSuportado
		}
		dados := make([]byte, tam)
		if _, err := io.ReadFull(r, dados); err != nil {
			return err
		}
		switch id {
		case idTrackNumber:
			if atual != nil {
				atual.numero = uintBE(dados)
			}
		case idCodecID:
			if atual != nil {
				atual.codec = string(bytes.TrimRight(dados, "\x00"))
			}
		case idCodecPriv:
			if atual != nil {
				atual.privado = dados
			}
		case idCanais:
			if atual != nil {
				atual.canais = int(uintBE(dados))
			}
		case idSimpleBlock, idBlock:
			if numeroOpus == 0 {
				for _, f := range faixas {
					if f.codec == "A_OPUS" {
						numeroOpus = f.numero
					}
				}
				if numeroOpus == 0 {
					return ErrWebMNaoSuportado
				}
			}
			br := bufio.NewReader(bytes.NewReader(dados))
			trilha, nt, err := lerVint(br, false)
			if err != nil || len(dados) < nt+3 {
				return ErrWebMNaoSuportado
			}
			if trilha != numeroOpus {
				continue
			}
			flags := dados[nt+2]
			if flags&0x06 != 0 { // lacing
				return ErrWebMNaoSuportado
			}
			pacotes = append(pacotes, dados[nt+3:])
		}
	}
	var opus *faixa
	for _, f := range faixas {
		if f.codec == "A_OPUS" {
			opus = f
		}
	}
	if opus == nil || len(pacotes) == 0 {
		return ErrWebMNaoSuportado
	}
	cabecalho := opus.privado
	if len(cabecalho) < 19 || string(cabecalho[:8]) != "OpusHead" {
		cabecalho = cabecalhoOpus(opus.canais)
	}
	return escreverOgg(saida, cabecalho, pacotes)
}

func uintBE(b []byte) uint64 {
	var v uint64
	for _, c := range b {
		v = v<<8 | uint64(c)
	}
	return v
}

func cabecalhoOpus(canais int) []byte {
	if canais < 1 || canais > 2 {
		canais = 1
	}
	h := make([]byte, 19)
	copy(h, "OpusHead")
	h[8] = 1
	h[9] = byte(canais)
	binary.LittleEndian.PutUint16(h[10:], 312)
	binary.LittleEndian.PutUint32(h[12:], 48000)
	return h
}

// AmostrasOpus devolve a duração de um pacote Opus em amostras a 48 kHz (RFC 6716 §3.1).
func AmostrasOpus(p []byte) int {
	if len(p) == 0 {
		return 0
	}
	toc := p[0]
	cfg := int(toc >> 3)
	var decimos int // duração do quadro em décimos de ms
	switch {
	case cfg < 12:
		decimos = []int{100, 200, 400, 600}[cfg%4]
	case cfg < 16:
		decimos = []int{100, 200}[cfg%2]
	default:
		decimos = []int{25, 50, 100, 200}[cfg%4]
	}
	quadros := 1
	switch toc & 3 {
	case 1, 2:
		quadros = 2
	case 3:
		if len(p) < 2 {
			return 0
		}
		quadros = int(p[1] & 0x3F)
	}
	return quadros * decimos * 48 / 10
}

var tabelaCRC = func() [256]uint32 {
	var t [256]uint32
	for i := range t {
		r := uint32(i) << 24
		for j := 0; j < 8; j++ {
			if r&0x80000000 != 0 {
				r = r<<1 ^ 0x04C11DB7
			} else {
				r <<= 1
			}
		}
		t[i] = r
	}
	return t
}()

// CRCOgg calcula o CRC das páginas Ogg (polinômio 0x04C11DB7, sem reflexão).
func CRCOgg(b []byte) uint32 {
	var crc uint32
	for _, c := range b {
		crc = crc<<8 ^ tabelaCRC[byte(crc>>24)^c]
	}
	return crc
}

func escreverPagina(w io.Writer, tipo byte, granula int64, serie, seq uint32, pacote []byte) error {
	var lacos []byte
	resto := len(pacote)
	for resto >= 255 {
		lacos = append(lacos, 255)
		resto -= 255
	}
	lacos = append(lacos, byte(resto))
	if len(lacos) > 255 {
		return ErrWebMNaoSuportado
	}
	p := make([]byte, 27, 27+len(lacos)+len(pacote))
	copy(p, "OggS")
	p[5] = tipo
	binary.LittleEndian.PutUint64(p[6:], uint64(granula))
	binary.LittleEndian.PutUint32(p[14:], serie)
	binary.LittleEndian.PutUint32(p[18:], seq)
	p[26] = byte(len(lacos))
	p = append(p, lacos...)
	p = append(p, pacote...)
	binary.LittleEndian.PutUint32(p[22:], CRCOgg(p))
	_, err := w.Write(p)
	return err
}

func escreverOgg(w io.Writer, cabecalho []byte, pacotes [][]byte) error {
	const serie = 0x5A415044 // "ZAPD"
	tags := []byte("OpusTags")
	fornecedor := "ZapDesk"
	tags = binary.LittleEndian.AppendUint32(tags, uint32(len(fornecedor)))
	tags = append(tags, fornecedor...)
	tags = binary.LittleEndian.AppendUint32(tags, 0)
	if err := escreverPagina(w, 0x02, 0, serie, 0, cabecalho); err != nil {
		return err
	}
	if err := escreverPagina(w, 0, 0, serie, 1, tags); err != nil {
		return err
	}
	var granula int64
	for i, p := range pacotes {
		granula += int64(AmostrasOpus(p))
		tipo := byte(0)
		if i == len(pacotes)-1 {
			tipo = 0x04
		}
		if err := escreverPagina(w, tipo, granula, serie, uint32(i+2), p); err != nil {
			return err
		}
	}
	return nil
}

// ConverterVoz cria um anexo OGG/Opus a partir de um áudio WebM/Opus (chat.ConversorVoz).
func (s *Servico) ConverterVoz(ctx context.Context, a dominio.Arquivo) (dominio.Arquivo, error) {
	if !strings.Contains(a.Mimetype, "webm") {
		return dominio.Arquivo{}, fmt.Errorf("%w: %s", ErrWebMNaoSuportado, a.Mimetype)
	}
	f, err := os.Open(a.Caminho)
	if err != nil {
		return dominio.Arquivo{}, err
	}
	defer f.Close()
	var buf bytes.Buffer
	if err := RemuxWebMParaOgg(f, &buf); err != nil {
		return dominio.Arquivo{}, err
	}
	nome := strings.TrimSuffix(a.Nome, filepath.Ext(a.Nome)) + ".ogg"
	return s.Criar(ctx, nome, "audio/ogg; codecs=opus", &buf)
}
