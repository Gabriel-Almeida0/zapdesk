package api

import (
	"errors"
	"mime"
	"net/http"
	"os"
	"strings"

	"zapdesk/motor/internal/arquivos"
	"zapdesk/motor/internal/erros"
)

func (s *Servidor) registrarRotasArquivos() {
	s.rota("POST /v1/arquivos", s.criarArquivo, false)
	s.rota("GET /v1/arquivos/{id}", s.obterArquivo, false)
	s.rota("GET /v1/arquivos/{id}/conteudo", s.conteudoArquivo, true)
}

// LimiteUpload cobre o maior anexo (documento de 100 MB) mais a sobrecarga do multipart.
const LimiteUpload = arquivos.LimiteDocumento + (1 << 20)

func (s *Servidor) criarArquivo(w http.ResponseWriter, r *http.Request) error {
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, LimiteUpload+(1<<20))
		leitor, err := r.MultipartReader()
		if err != nil {
			return erros.Campo("arquivo", "Envie o arquivo no campo \"arquivo\".")
		}
		for {
			parte, err := leitor.NextPart()
			if err != nil {
				return erros.Campo("arquivo", "Envie o arquivo no campo \"arquivo\".")
			}
			if parte.FormName() != "arquivo" {
				parte.Close()
				continue
			}
			a, err := s.o.Servicos.Arquivos.Criar(r.Context(), parte.FileName(), parte.Header.Get("Content-Type"), parte)
			parte.Close()
			if err != nil {
				var mbe *http.MaxBytesError
				if errors.As(err, &mbe) {
					return erros.ComDetalhes(erros.AnexoGrandeDemais, "Arquivo grande demais para o WhatsApp.",
						map[string]any{"limite_bytes": arquivos.LimiteDocumento, "tipo_midia": "documento"})
				}
				return err
			}
			return responderJSON(w, 201, a)
		}
	}
	var c struct {
		Caminho string `json:"caminho"`
	}
	if err := lerJSON(w, r, &c); err != nil {
		return err
	}
	if c.Caminho == "" {
		return erros.Campo("caminho", "Envie um arquivo (multipart, campo \"arquivo\") ou o caminho absoluto.")
	}
	a, err := s.o.Servicos.Arquivos.CriarDeCaminho(r.Context(), c.Caminho)
	if err != nil {
		return err
	}
	return responderJSON(w, 201, a)
}

func (s *Servidor) obterArquivo(w http.ResponseWriter, r *http.Request) error {
	a, err := s.o.Servicos.Arquivos.Obter(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return responderJSON(w, 200, a)
}

func (s *Servidor) conteudoArquivo(w http.ResponseWriter, r *http.Request) error {
	a, err := s.o.Servicos.Arquivos.Obter(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return servirArquivo(w, r, a.Caminho, a.Mimetype, a.Nome)
}

// servirArquivo envia um arquivo local com suporte a Range (players de áudio/vídeo).
func servirArquivo(w http.ResponseWriter, r *http.Request, caminho, mimetype, nome string) error {
	f, err := os.Open(caminho)
	if err != nil {
		return erros.Novo(erros.NaoEncontrado, "Arquivo não encontrado.")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if mimetype != "" {
		w.Header().Set("Content-Type", mimetype)
	}
	if nome != "" {
		w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": nome}))
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeContent(w, r, "", info.ModTime(), f)
	return nil
}
