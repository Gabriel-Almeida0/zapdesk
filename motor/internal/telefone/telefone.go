// Pacote telefone normaliza números para E.164 (data-model.md › Lead, regra 1).
package telefone

import (
	"strconv"
	"strings"

	"github.com/nyaruka/phonenumbers"
)

// Motivos de número inválido (relatório de importação).
const (
	MotivoVazio           = "vazio"
	MotivoFormatoInvalido = "formato_invalido"
	MotivoNumeroInvalido  = "numero_invalido"
)

// DDIPadrao é aplicado quando o número não tem DDI.
const DDIPadrao = "55"

// Normalizar converte `bruto` em E.164. Sem DDI aplica `ddiPadrao` (vazio = 55). Devolve o número
// ou um motivo (vazio | formato_invalido | numero_invalido).
func Normalizar(bruto, ddiPadrao string) (e164 string, motivo string) {
	s := strings.TrimSpace(bruto)
	if s == "" {
		return "", MotivoVazio
	}
	ddi := strings.TrimPrefix(strings.TrimSpace(ddiPadrao), "+")
	if ddi == "" {
		ddi = DDIPadrao
	}

	// Pré-limpeza: aceita dígitos, espaços, ( ) - . / e um + inicial.
	var b strings.Builder
	mais := false
	for i, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && i == 0:
			mais = true
		case r == ' ' || r == '(' || r == ')' || r == '-' || r == '.' || r == '/' || r == ' ' || r == '\t':
		default:
			return "", MotivoFormatoInvalido
		}
	}
	digitos := b.String()
	if digitos == "" {
		return "", MotivoFormatoInvalido
	}
	if !mais && strings.HasPrefix(digitos, "00") {
		digitos = digitos[2:]
		mais = true
	}
	if len(digitos) < 4 || len(digitos) > 17 {
		return "", MotivoNumeroInvalido
	}

	var num *phonenumbers.PhoneNumber
	var err error
	if mais {
		num, err = phonenumbers.Parse("+"+digitos, "ZZ")
	} else {
		cod, errConv := strconv.Atoi(ddi)
		regiao := phonenumbers.GetRegionCodeForCountryCode(cod)
		if errConv != nil || regiao == "" || regiao == "ZZ" {
			return "", MotivoFormatoInvalido
		}
		num, err = phonenumbers.Parse(digitos, regiao)
		// Número nacional inválido que já começa com o DDI (ex.: 5511999990000).
		if (err != nil || !phonenumbers.IsValidNumber(num)) && strings.HasPrefix(digitos, ddi) {
			if alt, errAlt := phonenumbers.Parse("+"+digitos, "ZZ"); errAlt == nil && phonenumbers.IsValidNumber(alt) {
				num, err = alt, nil
			}
		}
	}
	if err != nil || num == nil || !phonenumbers.IsValidNumber(num) {
		return "", MotivoNumeroInvalido
	}
	return phonenumbers.Format(num, phonenumbers.E164), ""
}
