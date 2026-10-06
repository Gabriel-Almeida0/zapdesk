package integracao

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

func parseRFC(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }

func logTeste() zerolog.Logger {
	if os.Getenv("ZAPDESK_LOG_TESTE") != "" {
		return zerolog.New(os.Stderr).Level(zerolog.DebugLevel)
	}
	return zerolog.Nop()
}
