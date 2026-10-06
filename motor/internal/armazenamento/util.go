package armazenamento

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

// Ms converte para milissegundos Unix.
func Ms(t time.Time) int64 { return t.UnixMilli() }

// MsPtr converte ponteiro (nil → NULL).
func MsPtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UnixMilli()
}

// DeMs converte milissegundos em horário local.
func DeMs(ms int64) time.Time { return time.UnixMilli(ms) }

func deNullMs(n sql.NullInt64) *time.Time {
	if !n.Valid {
		return nil
	}
	t := time.UnixMilli(n.Int64)
	return &t
}

func deNullStr(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	s := n.String
	return &s
}

func deNullInt(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

func strOuNil(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func intOuNil(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func vazioNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func jsonTexto(v any) string {
	j, _ := json.Marshal(v)
	return string(j)
}

// marcadores devolve "?, ?, ?" e os argumentos.
func marcadores(lista []string) (string, []any) {
	args := make([]any, len(lista))
	for i, v := range lista {
		args[i] = v
	}
	return strings.TrimSuffix(strings.Repeat("?, ", len(lista)), ", "), args
}

// escaparLike escapa % e _ para LIKE ... ESCAPE '\'.
func escaparLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(s) + "%"
}
