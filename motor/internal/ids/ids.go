// Pacote ids gera identificadores ULID (texto, ordenáveis por criação).
package ids

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	mu       sync.Mutex
	entropia = ulid.Monotonic(rand.Reader, 0)
)

// Novo devolve um ULID novo baseado no relógio do sistema.
func Novo() string {
	return NovoEm(time.Now())
}

// NovoEm devolve um ULID com o instante informado (monotônico dentro do mesmo milissegundo).
func NovoEm(t time.Time) string {
	mu.Lock()
	defer mu.Unlock()
	return ulid.MustNew(ulid.Timestamp(t), entropia).String()
}
