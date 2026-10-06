package modelo

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// NormalizarTexto aplica a normalização de formatos.md (gatilhos `contem`/`palavra_chave` e
// respostas de menu): minúsculas, sem acentos, espaços colapsados e pontuação das bordas removida.
func NormalizarTexto(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	sem, _, err := transform.String(t, s)
	if err != nil {
		sem = s
	}
	sem = strings.ToLower(sem)
	sem = strings.Join(strings.Fields(sem), " ")
	return strings.TrimFunc(sem, func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSymbol(r) || unicode.IsSpace(r) })
}

// Palavras quebra o texto normalizado em palavras isoladas (separadas por espaço ou pontuação).
func Palavras(s string) []string {
	return strings.FieldsFunc(NormalizarTexto(s), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
}

func reNome() *regexp.Regexp { return regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`) }
