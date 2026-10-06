// Pacote leads importa leads com normalização E.164 e deduplicação (data-model.md › Lead):
// dentro do lote (primeira ocorrência vence) e contra a base (preenche só campos vazios,
// nunca sobrescreve), tudo numa única transação.
package leads

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/importacao"
	"zapdesk/motor/internal/relogio"
	"zapdesk/motor/internal/telefone"
)

// Linha é uma linha a importar (Numero é a posição mostrada no relatório).
type Linha struct {
	Numero   int
	Telefone string
	Nome     string
	Campos   map[string]string
}

// Importador aplica as regras de importação.
type Importador struct {
	banco   *armazenamento.Banco
	relogio relogio.Relogio
}

// NovoImportador cria o importador.
func NovoImportador(b *armazenamento.Banco, r relogio.Relogio) *Importador {
	return &Importador{banco: b, relogio: r}
}

// falhar é um gancho de teste que força erro na gravação do i-ésimo telefone válido.
var falhar func(i int) bool

func truncar(s string, n int) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) > n {
		return string([]rune(s)[:n])
	}
	return s
}

// Importar importa as linhas numa transação e devolve o relatório.
func (im *Importador) Importar(ctx context.Context, linhas []Linha, origem, ddi string) (dominio.RelatorioImportacao, error) {
	var rel dominio.RelatorioImportacao
	err := im.banco.Transacao(ctx, func(tx *sql.Tx) error {
		var err error
		rel, err = im.ImportarTx(ctx, tx, linhas, origem, ddi)
		return err
	})
	return rel, err
}

type valida struct {
	linha Linha
	e164  string
}

// ImportarTx importa dentro de uma transação existente.
func (im *Importador) ImportarTx(ctx context.Context, tx *sql.Tx, linhas []Linha, origem, ddi string) (dominio.RelatorioImportacao, error) {
	rel := dominio.RelatorioImportacao{
		TotalLinhas: len(linhas), Novos: []dominio.ItemNovo{}, JaExistentes: []dominio.ItemJaExistente{},
		Invalidos: []dominio.ItemInvalido{}, DuplicadosNoLote: []dominio.ItemDuplicado{}, LeadIDs: []string{},
	}
	primeira := map[string]int{}
	var validas []valida
	for _, l := range linhas {
		e164, motivo := telefone.Normalizar(l.Telefone, ddi)
		if motivo != "" {
			rel.Invalidos = append(rel.Invalidos, dominio.ItemInvalido{Linha: l.Numero, Valor: l.Telefone, Motivo: motivo})
			continue
		}
		if p, ok := primeira[e164]; ok {
			rel.DuplicadosNoLote = append(rel.DuplicadosNoLote, dominio.ItemDuplicado{Linha: l.Numero, Telefone: e164, PrimeiraLinha: p})
			continue
		}
		primeira[e164] = l.Numero
		validas = append(validas, valida{l, e164})
	}

	tels := make([]string, len(validas))
	for i, v := range validas {
		tels[i] = v.e164
	}
	existentes, err := armazenamento.LeadsPorTelefones(ctx, tx, tels)
	if err != nil {
		return rel, err
	}
	agora := im.relogio.Agora()
	for i, v := range validas {
		if falhar != nil && falhar(i) {
			return rel, errors.New("falha simulada na importação")
		}
		nome := truncar(v.linha.Nome, armazenamento.TamanhoMaximoNomeLead)
		campos := map[string]string{}
		for k, val := range v.linha.Campos {
			k = importacao.NormalizarChave(k)
			val = strings.TrimSpace(val)
			if k != "" && val != "" {
				campos[k] = val
			}
		}
		if ex, ok := existentes[v.e164]; ok {
			var preenchidos []string
			novoNome := ex.Nome
			if (ex.Nome == nil || *ex.Nome == "") && nome != "" {
				novoNome = &nome
				preenchidos = append(preenchidos, "nome")
			}
			novosCampos := ex.Campos
			if novosCampos == nil {
				novosCampos = map[string]string{}
			}
			for k, val := range campos {
				if strings.TrimSpace(novosCampos[k]) == "" {
					novosCampos[k] = val
					preenchidos = append(preenchidos, k)
				}
			}
			sort.Strings(preenchidos)
			if preenchidos == nil {
				preenchidos = []string{}
			} else if err := armazenamento.AtualizarLeadDados(ctx, tx, ex.ID, novoNome, novosCampos, agora); err != nil {
				return rel, err
			}
			if err := armazenamento.LigarContatosAoLead(ctx, tx, ex.ID, v.e164); err != nil {
				return rel, err
			}
			rel.JaExistentes = append(rel.JaExistentes, dominio.ItemJaExistente{Linha: v.linha.Numero, LeadID: ex.ID, Telefone: v.e164,
				ImportadoEm: ex.ImportadoEm, CamposPreenchidos: preenchidos})
			rel.LeadIDs = append(rel.LeadIDs, ex.ID)
			continue
		}
		l := dominio.Lead{ID: ids.NovoEm(agora), Telefone: v.e164, Campos: campos, Origem: origem, ImportadoEm: agora}
		if nome != "" {
			l.Nome = &nome
		}
		if err := armazenamento.InserirLead(ctx, tx, l, agora); err != nil {
			return rel, err
		}
		if err := armazenamento.LigarContatosAoLead(ctx, tx, l.ID, v.e164); err != nil {
			return rel, err
		}
		existentes[v.e164] = l
		rel.Novos = append(rel.Novos, dominio.ItemNovo{Linha: v.linha.Numero, LeadID: l.ID, Telefone: v.e164})
		rel.LeadIDs = append(rel.LeadIDs, l.ID)
	}
	rel.TotalNovos, rel.TotalJaExistentes = len(rel.Novos), len(rel.JaExistentes)
	rel.TotalInvalidos, rel.TotalDuplicadosNoLote = len(rel.Invalidos), len(rel.DuplicadosNoLote)
	return rel, nil
}
