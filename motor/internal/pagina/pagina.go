// Pacote pagina define o cursor opaco de paginação por chave (keyset) usado pelos repositórios.
package pagina

import (
	"encoding/base64"
	"encoding/json"

	"zapdesk/motor/internal/erros"
)

// Chave é a posição de ordenação do último item entregue (ex.: data em ms + id).
type Chave struct {
	N int64  `json:"n"`
	S string `json:"s"`
}

// Codificar transforma a chave num cursor opaco.
func Codificar(n int64, s string) string {
	j, _ := json.Marshal(Chave{N: n, S: s})
	return base64.RawURLEncoding.EncodeToString(j)
}

// Decodificar faz o inverso. Cursor vazio → (nil, nil).
func Decodificar(cursor string) (*Chave, error) {
	if cursor == "" {
		return nil, nil
	}
	j, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, erros.Campo("cursor", "Cursor inválido.")
	}
	var c Chave
	if err := json.Unmarshal(j, &c); err != nil {
		return nil, erros.Campo("cursor", "Cursor inválido.")
	}
	return &c, nil
}

// Params são os parâmetros de uma página.
type Params struct {
	Limite int
	Cursor *Chave
}
