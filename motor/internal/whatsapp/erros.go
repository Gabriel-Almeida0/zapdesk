package whatsapp

import "errors"

// Erros sentinela. O adaptador real traduz os erros do whatsmeow para eles; erros de rede
// transitórios são ErrDesconectado (o disparo aguarda e não marca falhou).
var (
	ErrSemWhatsApp   = errors.New("número sem WhatsApp")
	ErrDesconectado  = errors.New("sem conexão com o WhatsApp")
	ErrBanido        = errors.New("o WhatsApp bloqueou este número")
	ErrForaDoPrazo   = errors.New("fora do prazo permitido pelo WhatsApp")
	ErrMidiaExpirada = errors.New("mídia expirada no servidor do WhatsApp")
	// ErrEstadoIncerto: a mensagem pode ter saído (tempo esgotado depois do envio). O disparo
	// marca "falhou" com "Estado incerto após interrupção" em vez de reenviar (Constituição V).
	ErrEstadoIncerto = errors.New("não foi possível confirmar o envio")
)
