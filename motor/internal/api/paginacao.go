package api

import (
	"net/http"
	"strconv"

	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/pagina"
)

// Limites de paginação do contrato.
const (
	LimitePadrao = 50
	LimiteMaximo = 200
)

// Pagina é a resposta paginada `{"itens":[...],"proximo_cursor":...}`.
type Pagina[T any] struct {
	Itens         []T     `json:"itens"`
	ProximoCursor *string `json:"proximo_cursor"`
}

// NovaPagina garante `itens: []` em vez de null.
func NovaPagina[T any](itens []T, proximo string) Pagina[T] {
	if itens == nil {
		itens = []T{}
	}
	p := Pagina[T]{Itens: itens}
	if proximo != "" {
		p.ProximoCursor = &proximo
	}
	return p
}

// lerLimite lê ?limite (1..200, padrão 50).
func lerLimite(r *http.Request) (int, error) {
	v := r.URL.Query().Get("limite")
	if v == "" {
		return LimitePadrao, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > LimiteMaximo {
		return 0, erros.Campo("limite", "O limite deve ser um número de 1 a 200.")
	}
	return n, nil
}

// lerPagina lê ?limite e ?cursor.
func lerPagina(r *http.Request) (pagina.Params, error) {
	limite, err := lerLimite(r)
	if err != nil {
		return pagina.Params{}, err
	}
	cur, err := pagina.Decodificar(r.URL.Query().Get("cursor"))
	if err != nil {
		return pagina.Params{}, err
	}
	return pagina.Params{Limite: limite, Cursor: cur}, nil
}
