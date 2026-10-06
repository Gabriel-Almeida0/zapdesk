// Pacote arquivos guarda os anexos enviados pelo usuário/MCP em <pasta-dados>/anexos/ com os
// limites do WhatsApp (contracts/api-http.md › Arquivos) e detecção de tipo pelo conteúdo.
package arquivos

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/relogio"
)

// Limites por tipo de mídia (bytes).
const (
	LimiteMidia     = 16 << 20
	LimiteDocumento = 100 << 20
	LimiteFigurinha = 1 << 20
)

// Tipos de mídia.
const (
	TipoImagem    = "imagem"
	TipoVideo     = "video"
	TipoAudio     = "audio"
	TipoDocumento = "documento"
	TipoFigurinha = "figurinha"
)

// Limite devolve o limite do tipo.
func Limite(tipo string) int64 {
	switch tipo {
	case TipoDocumento:
		return LimiteDocumento
	case TipoFigurinha:
		return LimiteFigurinha
	}
	return LimiteMidia
}

// Servico de anexos.
type Servico struct {
	banco   *armazenamento.Banco
	pasta   string
	relogio relogio.Relogio
}

// Novo cria o serviço (anexos em <pastaDados>/anexos).
func Novo(banco *armazenamento.Banco, pastaDados string, r relogio.Relogio) *Servico {
	return &Servico{banco: banco, pasta: filepath.Join(pastaDados, "anexos"), relogio: r}
}

var porExtensao = map[string]string{
	".ogg": "audio/ogg", ".opus": "audio/ogg; codecs=opus", ".oga": "audio/ogg", ".mp3": "audio/mpeg", ".m4a": "audio/mp4",
	".aac": "audio/aac", ".wav": "audio/wav", ".weba": "audio/webm", ".amr": "audio/amr",
	".mp4": "video/mp4", ".mov": "video/quicktime", ".3gp": "video/3gpp", ".webm": "video/webm",
	".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".gif": "image/gif", ".webp": "image/webp",
	".pdf": "application/pdf", ".csv": "text/csv", ".txt": "text/plain",
	".doc": "application/msword", ".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls": "application/vnd.ms-excel", ".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".ppt": "application/vnd.ms-powerpoint", ".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".zip": "application/zip",
}

// DetectarMimetype decide o mimetype pelo conteúdo, usando a dica (Content-Type do upload) e a
// extensão só quando o conteúdo é ambíguo.
func DetectarMimetype(cabeca []byte, nome, dica string) string {
	detectado := http.DetectContentType(cabeca)
	if i := strings.Index(detectado, ";"); i >= 0 && !strings.HasPrefix(detectado, "text/") {
		detectado = detectado[:i]
	}
	ext := strings.ToLower(filepath.Ext(nome))
	dica = strings.TrimSpace(strings.ToLower(dica))
	porExt := porExtensao[ext]
	soAudio := detectado == "video/webm" && bytes.Contains(cabeca, []byte("A_OPUS")) &&
		!bytes.Contains(cabeca, []byte("V_VP")) && !bytes.Contains(cabeca, []byte("V_AV1")) && !bytes.Contains(cabeca, []byte("V_MPEG"))
	switch {
	case detectado == "video/webm" && (soAudio || strings.HasPrefix(dica, "audio/") || ext == ".weba"):
		if dica != "" && strings.HasPrefix(dica, "audio/webm") {
			return dica
		}
		return "audio/webm"
	case detectado == "application/ogg" || detectado == "audio/ogg":
		if strings.Contains(dica, "opus") || ext == ".opus" {
			return "audio/ogg; codecs=opus"
		}
		return "audio/ogg"
	case detectado == "application/octet-stream" || strings.HasPrefix(detectado, "text/plain") || detectado == "application/zip":
		if porExt != "" {
			return porExt
		}
		if dica != "" && dica != "application/octet-stream" {
			return dica
		}
	}
	if strings.HasPrefix(detectado, "text/plain") && porExt == "" {
		return "text/plain"
	}
	return detectado
}

// TipoDoMimetype classifica o mimetype.
func TipoDoMimetype(mt string) string {
	switch {
	case mt == "image/webp":
		return TipoFigurinha
	case strings.HasPrefix(mt, "image/"):
		return TipoImagem
	case strings.HasPrefix(mt, "video/"):
		return TipoVideo
	case strings.HasPrefix(mt, "audio/"):
		return TipoAudio
	}
	return TipoDocumento
}

// Criar grava o conteúdo lido de r como novo anexo.
func (s *Servico) Criar(ctx context.Context, nome, dica string, r io.Reader) (dominio.Arquivo, error) {
	if err := os.MkdirAll(s.pasta, 0o700); err != nil {
		return dominio.Arquivo{}, err
	}
	nome = filepath.Base(strings.TrimSpace(nome))
	if nome == "" || nome == "." || nome == "/" {
		nome = "arquivo"
	}
	agora := s.relogio.Agora()
	id := ids.NovoEm(agora)
	destino := filepath.Join(s.pasta, id)
	arq, err := os.OpenFile(destino, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return dominio.Arquivo{}, err
	}
	cabeca := make([]byte, 512)
	n, _ := io.ReadFull(r, cabeca)
	cabeca = cabeca[:n]
	mt := DetectarMimetype(cabeca, nome, dica)
	tipo := TipoDoMimetype(mt)
	limite := Limite(tipo)
	total, err := io.Copy(arq, io.LimitReader(io.MultiReader(bytes.NewReader(cabeca), r), limite+1))
	arq.Close()
	if err != nil {
		os.Remove(destino)
		return dominio.Arquivo{}, err
	}
	if total > limite {
		os.Remove(destino)
		return dominio.Arquivo{}, erros.ComDetalhes(erros.AnexoGrandeDemais,
			fmt.Sprintf("Arquivo grande demais para o WhatsApp (limite de %s para %s).", humano(limite), nomeTipo(tipo)),
			map[string]any{"limite_bytes": limite, "tipo_midia": tipo})
	}
	if total == 0 {
		os.Remove(destino)
		return dominio.Arquivo{}, erros.Campo("arquivo", "O arquivo está vazio.")
	}
	a := dominio.Arquivo{ID: id, Nome: nome, Mimetype: mt, Tamanho: total, TipoMidia: tipo, Caminho: destino,
		URL: "/v1/arquivos/" + id + "/conteudo"}
	if err := armazenamento.CriarArquivo(ctx, s.banco.E(), a, agora); err != nil {
		os.Remove(destino)
		return dominio.Arquivo{}, err
	}
	return a, nil
}

// CriarDeCaminho copia um arquivo local (caminho absoluto) como anexo.
func (s *Servico) CriarDeCaminho(ctx context.Context, caminho string) (dominio.Arquivo, error) {
	if !filepath.IsAbs(caminho) {
		return dominio.Arquivo{}, erros.Campo("caminho", "Informe o caminho absoluto do arquivo.")
	}
	info, err := os.Stat(caminho)
	if err != nil || info.IsDir() {
		return dominio.Arquivo{}, erros.Campo("caminho", "Arquivo não encontrado.")
	}
	arq, err := os.Open(caminho)
	if err != nil {
		return dominio.Arquivo{}, erros.Campo("caminho", "Não consegui ler o arquivo.")
	}
	defer arq.Close()
	return s.Criar(ctx, filepath.Base(caminho), mime.TypeByExtension(filepath.Ext(caminho)), arq)
}

// Obter devolve o anexo.
func (s *Servico) Obter(ctx context.Context, id string) (dominio.Arquivo, error) {
	return armazenamento.ObterArquivo(ctx, s.banco.L(), id)
}

// Abrir abre o conteúdo do anexo.
func (s *Servico) Abrir(a dominio.Arquivo) (io.ReadCloser, error) {
	f, err := os.Open(a.Caminho)
	if errors.Is(err, os.ErrNotExist) {
		return nil, erros.NaoAchado("Arquivo")
	}
	return f, err
}

func humano(b int64) string {
	if b >= 1<<20 {
		return fmt.Sprintf("%d MB", b>>20)
	}
	return fmt.Sprintf("%d KB", b>>10)
}

func nomeTipo(t string) string {
	switch t {
	case TipoImagem:
		return "imagem"
	case TipoVideo:
		return "vídeo"
	case TipoAudio:
		return "áudio"
	case TipoFigurinha:
		return "figurinha"
	}
	return "documento"
}
