package disparos

import (
	"context"
	"database/sql"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/variaveis"
)

// ResultadoAdicionar de POST /disparos/{id}/destinatarios.
type ResultadoAdicionar struct {
	Adicionados int             `json:"adicionados"`
	JaExistiam  int             `json:"ja_existiam"`
	Disparo     dominio.Disparo `json:"disparo"`
}

// MaxLeadsAdicionar por chamada.
const MaxLeadsAdicionar = 10000

// AdicionarDestinatarios acrescenta leads a um disparo não finalizado (feature 002, ação
// "adicionar a um disparo"): idempotente por UNIQUE(disparo_id, lead_id), ordem continua a
// sequência e as variáveis são validadas com os valores_padrao do disparo.
func (s *Servico) AdicionarDestinatarios(ctx context.Context, id string, leadIDs []string) (ResultadoAdicionar, error) {
	d, err := armazenamento.ObterDisparo(ctx, s.banco.L(), id)
	if err != nil {
		return ResultadoAdicionar{}, err
	}
	switch d.Estado {
	case dominio.DisparoRascunho, dominio.DisparoAgendado, dominio.DisparoEnviando, dominio.DisparoForaDaJanela, dominio.DisparoPausado:
	default:
		return ResultadoAdicionar{}, erros.Transicao("Não é possível acrescentar contatos a um disparo concluído ou cancelado.", d.Estado)
	}
	if len(leadIDs) < 1 || len(leadIDs) > MaxLeadsAdicionar {
		return ResultadoAdicionar{}, erros.Campo("lead_ids", "Envie de 1 a 10.000 leads.")
	}
	vistos := map[string]bool{}
	var unicos []string
	for _, l := range leadIDs {
		if !vistos[l] {
			vistos[l] = true
			unicos = append(unicos, l)
		}
	}
	lista, err := armazenamento.LeadsPorIDs(ctx, s.banco.L(), unicos)
	if err != nil {
		return ResultadoAdicionar{}, err
	}
	if len(lista) != len(unicos) {
		return ResultadoAdicionar{}, erros.Campo("lead_ids", "Algum lead não foi encontrado.")
	}
	existentes := map[string]bool{}
	armazenamento.PercorrerDestinatarios(ctx, s.banco.L(), id, func(x dominio.Destinatario) error {
		existentes[x.LeadID] = true
		return nil
	})
	var novos []dominio.Lead
	for _, l := range lista {
		if !existentes[l.ID] {
			novos = append(novos, l)
		}
	}
	res := ResultadoAdicionar{JaExistiam: len(lista) - len(novos)}
	if len(novos) > 0 {
		if f := variaveis.Faltando(d.Mensagem, novos, d.ValoresPadrao); len(f) > 0 {
			return ResultadoAdicionar{}, erroFaltando(f)
		}
		agora := s.relogio.Agora()
		vars := variaveis.Extrair(d.Mensagem)
		err = s.banco.Transacao(ctx, func(tx *sql.Tx) error {
			var ordem int
			if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(ordem), 0) FROM destinatarios WHERE disparo_id = ?`, id).Scan(&ordem); err != nil {
				return err
			}
			dests := make([]dominio.Destinatario, len(novos))
			for i, l := range novos {
				vals, _ := variaveis.ValoresDoLead(vars, l, d.ValoresPadrao)
				dests[i] = dominio.Destinatario{ID: ids.NovoEm(agora), DisparoID: id, LeadID: l.ID, Ordem: ordem + i + 1,
					Telefone: l.Telefone, Nome: l.Nome, Variaveis: vals}
			}
			return armazenamento.InserirDestinatarios(ctx, tx, dests)
		})
		if err != nil {
			return ResultadoAdicionar{}, err
		}
		res.Adicionados = len(novos)
		s.publicarDisparo(id, true)
		s.acordar(d.ContaID)
	}
	res.Disparo, err = s.Obter(ctx, id)
	return res, err
}

// AdicionarLeads é o adaptador usado pela ação adicionar_a_disparo.
func (s *Servico) AdicionarLeads(ctx context.Context, disparoID string, leadIDs []string) (int, int, error) {
	r, err := s.AdicionarDestinatarios(ctx, disparoID, leadIDs)
	return r.Adicionados, r.JaExistiam, err
}
