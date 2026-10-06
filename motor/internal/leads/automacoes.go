package leads

import (
	"context"
	"database/sql"
	"strings"
	"unicode/utf8"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/importacao"
	"zapdesk/motor/internal/telefone"
)

// Acréscimos da feature 002: PATCH /leads/{id} (nome e campos) e criação do lead a partir de um
// contato (Kanban, ações e ctx.leads).

// TamanhoMaximoValorCampo de um campo de lead.
const TamanhoMaximoValorCampo = 1000

// AlteracaoLead do PATCH /leads/{id}: Nome definido (NomeDefinido) com nil remove; Campos é merge
// e valor nil remove o campo.
type AlteracaoLead struct {
	Nome         *string
	NomeDefinido bool
	Campos       map[string]*string
}

// Atualizar aplica a alteração (chaves normalizadas como na importação; valores ≤ 1.000).
func (s *Servico) Atualizar(ctx context.Context, id string, a AlteracaoLead) (dominio.Lead, error) {
	l, err := armazenamento.ObterLead(ctx, s.banco.L(), id)
	if err != nil {
		return l, err
	}
	campos := map[string]string{}
	for k, v := range l.Campos {
		campos[k] = v
	}
	for k, v := range a.Campos {
		chave := importacao.NormalizarChave(k)
		if chave == "" {
			return l, erros.Campo("campos", "Nome de campo inválido: \""+k+"\".")
		}
		if v == nil || strings.TrimSpace(*v) == "" {
			delete(campos, chave)
			continue
		}
		if utf8.RuneCountInString(*v) > TamanhoMaximoValorCampo {
			return l, erros.Campo("campos."+chave, "O valor pode ter até 1.000 caracteres.")
		}
		campos[chave] = strings.TrimSpace(*v)
	}
	nome := l.Nome
	if a.NomeDefinido {
		nome = nil
		if a.Nome != nil && strings.TrimSpace(*a.Nome) != "" {
			n := strings.TrimSpace(*a.Nome)
			if utf8.RuneCountInString(n) > armazenamento.TamanhoMaximoNomeLead {
				return l, erros.Campo("nome", "O nome pode ter até 120 caracteres.")
			}
			nome = &n
		}
	}
	if err := armazenamento.AtualizarLeadDados(ctx, s.banco.E(), id, nome, campos, s.relogio.Agora()); err != nil {
		return l, err
	}
	return armazenamento.ObterLead(ctx, s.banco.L(), id)
}

// GarantirLeadDoContato devolve o lead ligado ao contato; se não houver, cria o lead (telefone
// E.164 do contato, nome = nome ou nome_push, origem "contatos") e liga os contatos do telefone,
// na mesma transação. Contato de grupo/sem telefone → validacao "Grupos não entram em funil".
func (s *Servico) GarantirLeadDoContato(ctx context.Context, contatoID string) (dominio.Lead, bool, error) {
	c, err := armazenamento.ObterContato(ctx, s.banco.L(), contatoID)
	if err != nil {
		return dominio.Lead{}, false, err
	}
	if c.Lead != nil {
		l, err := armazenamento.ObterLead(ctx, s.banco.L(), c.Lead.ID)
		return l, false, err
	}
	if c.Telefone == nil || *c.Telefone == "" || strings.HasSuffix(c.JID, "@g.us") {
		return dominio.Lead{}, false, erros.Campo("contato_id", "Grupos não entram em funil.")
	}
	nome := c.Nome
	if nome == nil || *nome == "" {
		nome = c.NomePush
	}
	return s.GarantirLeadTelefone(ctx, *c.Telefone, nome, dominio.OrigemContatos)
}

// GarantirLeadTelefone devolve (ou cria com a origem dada) o lead do telefone (qualquer formato).
func (s *Servico) GarantirLeadTelefone(ctx context.Context, bruto string, nome *string, origem string) (dominio.Lead, bool, error) {
	e164, motivo := telefone.Normalizar(bruto, "")
	if motivo != "" {
		return dominio.Lead{}, false, erros.Campo("telefone", "Telefone inválido. Confira o DDD e o número.")
	}
	if l, err := armazenamento.LeadPorTelefone(ctx, s.banco.L(), e164); err == nil {
		return l, false, nil
	}
	agora := s.relogio.Agora()
	l := dominio.Lead{ID: ids.NovoEm(agora), Telefone: e164, Nome: nome, Campos: map[string]string{}, Origem: origem, ImportadoEm: agora}
	err := s.banco.Transacao(ctx, func(tx *sql.Tx) error {
		if err := armazenamento.InserirLead(ctx, tx, l, agora); err != nil {
			return err
		}
		return armazenamento.LigarContatosAoLead(ctx, tx, l.ID, e164)
	})
	if err != nil {
		if l2, err2 := armazenamento.LeadPorTelefone(ctx, s.banco.L(), e164); err2 == nil {
			return l2, false, nil
		}
		return dominio.Lead{}, false, err
	}
	l, err = armazenamento.ObterLead(ctx, s.banco.L(), l.ID)
	return l, true, err
}
