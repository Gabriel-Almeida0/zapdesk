package chat

import (
	"context"
	"strings"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/pagina"
)

// ResultadoBusca do contrato.
type ResultadoBusca struct {
	Mensagem dominio.Mensagem `json:"mensagem"`
	Conversa struct {
		ID   string `json:"id"`
		Nome string `json:"nome"`
	} `json:"conversa"`
	Trecho string `json:"trecho"`
}

// Buscar faz a busca FTS (sem diferenciar acentos) nas mensagens da conta.
func (s *Servico) Buscar(ctx context.Context, contaID, q string, p pagina.Params) ([]ResultadoBusca, string, error) {
	if strings.TrimSpace(q) == "" {
		return nil, "", erros.Campo("q", "Digite o que procurar.")
	}
	if _, err := armazenamento.ObterConta(ctx, s.banco.L(), contaID); err != nil {
		return nil, "", err
	}
	brutos, prox, err := armazenamento.BuscarMensagens(ctx, s.banco.L(), contaID, q, p)
	if err != nil {
		return nil, "", err
	}
	res := make([]ResultadoBusca, len(brutos))
	for i, b := range brutos {
		res[i].Mensagem = s.API(b.Mensagem)
		res[i].Conversa.ID = b.ConversaID
		res[i].Conversa.Nome = b.ConversaNome
		res[i].Trecho = b.Trecho
	}
	return res, prox, nil
}

// ListarMensagens devolve uma página (mais recentes primeiro).
func (s *Servico) ListarMensagens(ctx context.Context, conversaID, antes string, limite int) ([]dominio.Mensagem, string, error) {
	if _, err := armazenamento.ObterConversa(ctx, s.banco.L(), conversaID); err != nil {
		return nil, "", err
	}
	brutos, prox, err := armazenamento.ListarMensagens(ctx, s.banco.L(), conversaID, antes, limite)
	if err != nil {
		return nil, "", err
	}
	lista := make([]dominio.Mensagem, len(brutos))
	for i, b := range brutos {
		lista[i] = s.API(b)
	}
	return lista, prox, nil
}
