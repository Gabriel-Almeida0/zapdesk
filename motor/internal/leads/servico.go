package leads

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/fatos"
	"zapdesk/motor/internal/importacao"
	"zapdesk/motor/internal/pagina"
	"zapdesk/motor/internal/relogio"
)

// Servico de leads e importação.
type Servico struct {
	banco      *armazenamento.Banco
	barramento *eventos.Barramento
	relogio    relogio.Relogio
	Importador *Importador
	previas    *importacao.Previas
	fatos      fatos.Emissor
}

// DefinirReceptorFatos liga o despachante de automações (gatilho lead_importado).
func (s *Servico) DefinirReceptorFatos(r fatos.Receptor) { s.fatos.DefinirReceptor(r) }

// NovoServico cria o serviço.
func NovoServico(b *armazenamento.Banco, bar *eventos.Barramento, r relogio.Relogio) *Servico {
	return &Servico{banco: b, barramento: bar, relogio: r, Importador: NovoImportador(b, r), previas: importacao.NovasPrevias(r)}
}

// RespostaPrevia de POST /v1/importacoes/previa.
type RespostaPrevia struct {
	ImportacaoID           string     `json:"importacao_id"`
	NomeArquivo            string     `json:"nome_arquivo"`
	Colunas                []string   `json:"colunas"`
	Amostra                [][]string `json:"amostra"`
	TotalLinhas            int        `json:"total_linhas"`
	ColunaTelefoneSugerida *string    `json:"coluna_telefone_sugerida"`
	ColunaNomeSugerida     *string    `json:"coluna_nome_sugerida"`
	ExpiraEm               time.Time  `json:"expira_em"`
}

// Previa lê a planilha e guarda para a importação.
func (s *Servico) Previa(nome string, r io.Reader) (RespostaPrevia, error) {
	pl, err := importacao.Ler(nome, r)
	if err != nil {
		return RespostaPrevia{}, err
	}
	pv := s.previas.Guardar(nome, pl)
	tel, nomeCol := importacao.SugerirColunas(pl.Colunas)
	resp := RespostaPrevia{ImportacaoID: pv.ID, NomeArquivo: nome, Colunas: pl.Colunas, Amostra: [][]string{},
		TotalLinhas: len(pl.Linhas), ColunaTelefoneSugerida: dominio.Str(tel), ColunaNomeSugerida: dominio.Str(nomeCol), ExpiraEm: pv.ExpiraEm}
	for i := 0; i < len(pl.Linhas) && i < 5; i++ {
		resp.Amostra = append(resp.Amostra, pl.Linhas[i].Valores)
	}
	return resp, nil
}

// PreviaCaminho lê uma planilha local (caminho absoluto; usado pelo MCP).
func (s *Servico) PreviaCaminho(caminho string) (RespostaPrevia, error) {
	if !filepath.IsAbs(caminho) {
		return RespostaPrevia{}, erros.Campo("caminho", "Informe o caminho absoluto da planilha.")
	}
	f, err := os.Open(caminho)
	if err != nil {
		return RespostaPrevia{}, erros.Campo("caminho", "Arquivo não encontrado.")
	}
	defer f.Close()
	return s.Previa(filepath.Base(caminho), f)
}

// Mapeamento da forma A.
type Mapeamento struct {
	Telefone string    `json:"telefone"`
	Nome     *string   `json:"nome"`
	Extras   *[]string `json:"extras"`
}

// LeadEntrada da forma C.
type LeadEntrada struct {
	Telefone string            `json:"telefone"`
	Nome     *string           `json:"nome"`
	Campos   map[string]string `json:"campos"`
}

// CorpoImportacao é o corpo de POST /v1/leads/importar (exatamente uma forma).
type CorpoImportacao struct {
	ImportacaoID *string        `json:"importacao_id"`
	Mapeamento   *Mapeamento    `json:"mapeamento"`
	TextoColado  *string        `json:"texto_colado"`
	Leads        *[]LeadEntrada `json:"leads"`
	Origem       string         `json:"origem"`
	DDIPadrao    string         `json:"ddi_padrao"`
}

// OrigemValida confere a origem.
func OrigemValida(o string) bool {
	switch o {
	case dominio.OrigemCSV, dominio.OrigemColado, dominio.OrigemContatos, dominio.OrigemMCP:
		return true
	}
	return false
}

// PrepararLinhas converte o corpo nas linhas a importar (sem gravar) e devolve a origem.
func (s *Servico) PrepararLinhas(c CorpoImportacao) ([]Linha, string, string, error) {
	formas := 0
	for _, presente := range []bool{c.ImportacaoID != nil, c.TextoColado != nil, c.Leads != nil} {
		if presente {
			formas++
		}
	}
	if formas != 1 {
		return nil, "", "", erros.Campo("importacao_id", "Envie exatamente uma forma: importacao_id + mapeamento, texto_colado ou leads.")
	}
	if c.Origem != "" && !OrigemValida(c.Origem) {
		return nil, "", "", erros.Campo("origem", "Origem inválida (use csv, colado, contatos ou mcp).")
	}
	var linhas []Linha
	origem := c.Origem
	switch {
	case c.ImportacaoID != nil:
		if c.Mapeamento == nil || strings.TrimSpace(c.Mapeamento.Telefone) == "" {
			return nil, "", "", erros.Campo("mapeamento.telefone", "Escolha qual coluna tem o telefone.")
		}
		pv, err := s.previas.Obter(*c.ImportacaoID)
		if err != nil {
			return nil, "", "", err
		}
		indice := map[string]int{}
		for i, col := range pv.Planilha.Colunas {
			indice[col] = i
		}
		iTel, ok := indice[c.Mapeamento.Telefone]
		if !ok {
			return nil, "", "", erros.Campo("mapeamento.telefone", "A coluna \""+c.Mapeamento.Telefone+"\" não existe na planilha.")
		}
		iNome := -1
		if c.Mapeamento.Nome != nil && *c.Mapeamento.Nome != "" {
			i, ok := indice[*c.Mapeamento.Nome]
			if !ok {
				return nil, "", "", erros.Campo("mapeamento.nome", "A coluna \""+*c.Mapeamento.Nome+"\" não existe na planilha.")
			}
			iNome = i
		}
		var extras []int
		if c.Mapeamento.Extras == nil {
			for i := range pv.Planilha.Colunas {
				if i != iTel && i != iNome {
					extras = append(extras, i)
				}
			}
		} else {
			for _, col := range *c.Mapeamento.Extras {
				i, ok := indice[col]
				if !ok {
					return nil, "", "", erros.Campo("mapeamento.extras", "A coluna \""+col+"\" não existe na planilha.")
				}
				if i != iTel && i != iNome {
					extras = append(extras, i)
				}
			}
		}
		for _, l := range pv.Planilha.Linhas {
			ln := Linha{Numero: l.Numero, Telefone: l.Valores[iTel], Campos: map[string]string{}}
			if iNome >= 0 {
				ln.Nome = l.Valores[iNome]
			}
			for _, i := range extras {
				ln.Campos[pv.Planilha.Colunas[i]] = l.Valores[i]
			}
			linhas = append(linhas, ln)
		}
		if origem == "" {
			origem = dominio.OrigemCSV
		}
		return linhas, origem, pv.ID, nil
	case c.TextoColado != nil:
		for i, l := range strings.Split(strings.ReplaceAll(*c.TextoColado, "\r\n", "\n"), "\n") {
			if strings.TrimSpace(l) == "" {
				continue
			}
			linhas = append(linhas, Linha{Numero: i + 1, Telefone: strings.TrimSpace(l)})
		}
		if len(linhas) == 0 {
			return nil, "", "", erros.Campo("texto_colado", "Cole pelo menos um número.")
		}
		if origem == "" {
			origem = dominio.OrigemColado
		}
	default:
		for i, l := range *c.Leads {
			ln := Linha{Numero: i + 1, Telefone: l.Telefone, Campos: l.Campos}
			if l.Nome != nil {
				ln.Nome = *l.Nome
			}
			linhas = append(linhas, ln)
		}
		if len(linhas) == 0 {
			return nil, "", "", erros.Campo("leads", "Envie pelo menos um lead.")
		}
		if origem == "" {
			origem = dominio.OrigemMCP
		}
	}
	return linhas, origem, "", nil
}

// Importar executa POST /v1/leads/importar.
func (s *Servico) Importar(ctx context.Context, c CorpoImportacao) (dominio.RelatorioImportacao, error) {
	linhas, origem, previaID, err := s.PrepararLinhas(c)
	if err != nil {
		return dominio.RelatorioImportacao{}, err
	}
	rel, err := s.Importador.Importar(ctx, linhas, origem, c.DDIPadrao)
	if err != nil {
		return rel, err
	}
	if previaID != "" {
		s.previas.Remover(previaID)
	}
	s.PublicarImportacao(origem, rel)
	return rel, nil
}

// PublicarImportacao emite leads.importados.
func (s *Servico) PublicarImportacao(origem string, rel dominio.RelatorioImportacao) {
	s.barramento.Publicar(eventos.LeadsImportados, "", map[string]any{"origem": origem, "total_novos": rel.TotalNovos,
		"total_ja_existentes": rel.TotalJaExistentes, "total_invalidos": rel.TotalInvalidos})
	if len(rel.Novos) > 0 {
		novos := make([]string, len(rel.Novos))
		for i, n := range rel.Novos {
			novos[i] = n.LeadID
		}
		s.fatos.Emitir(context.Background(), fatos.Fato{Tipo: fatos.LeadImportado, LeadIDs: novos, OrigemLead: origem, Em: s.relogio.Agora()})
	}
}

// LinhasDeContatos monta as linhas a partir de contatos e/ou etiquetas de uma conta.
func (s *Servico) LinhasDeContatos(ctx context.Context, contaID string, contatoIDs, etiquetaIDs []string) ([]Linha, error) {
	if _, err := armazenamento.ObterConta(ctx, s.banco.L(), contaID); err != nil {
		return nil, err
	}
	if len(contatoIDs) == 0 && len(etiquetaIDs) == 0 {
		return nil, erros.Campo("contato_ids", "Escolha contatos ou etiquetas.")
	}
	var conds []string
	args := []any{contaID}
	if len(contatoIDs) > 0 {
		m := strings.TrimSuffix(strings.Repeat("?, ", len(contatoIDs)), ", ")
		conds = append(conds, "ct.id IN ("+m+")")
		for _, id := range contatoIDs {
			args = append(args, id)
		}
	}
	if len(etiquetaIDs) > 0 {
		m := strings.TrimSuffix(strings.Repeat("?, ", len(etiquetaIDs)), ", ")
		conds = append(conds, "EXISTS (SELECT 1 FROM contato_etiquetas ce WHERE ce.contato_id = ct.id AND ce.etiqueta_id IN ("+m+"))")
		for _, id := range etiquetaIDs {
			args = append(args, id)
		}
	}
	linhasSQL, err := s.banco.L().QueryContext(ctx, `SELECT ct.telefone, COALESCE(NULLIF(ct.nome, ''), ct.nome_push, '')
		FROM contatos ct WHERE ct.conta_id = ? AND ct.telefone IS NOT NULL AND (`+strings.Join(conds, " OR ")+`) ORDER BY ct.id`, args...)
	if err != nil {
		return nil, err
	}
	defer linhasSQL.Close()
	var linhas []Linha
	for linhasSQL.Next() {
		var tel, nome string
		if err := linhasSQL.Scan(&tel, &nome); err != nil {
			return nil, err
		}
		linhas = append(linhas, Linha{Numero: len(linhas) + 1, Telefone: tel, Nome: nome})
	}
	return linhas, linhasSQL.Err()
}

// ImportarContatos executa POST /v1/leads/importar-contatos.
func (s *Servico) ImportarContatos(ctx context.Context, contaID string, contatoIDs, etiquetaIDs []string) (dominio.RelatorioImportacao, error) {
	linhas, err := s.LinhasDeContatos(ctx, contaID, contatoIDs, etiquetaIDs)
	if err != nil {
		return dominio.RelatorioImportacao{}, err
	}
	rel, err := s.Importador.Importar(ctx, linhas, dominio.OrigemContatos, "")
	if err != nil {
		return rel, err
	}
	s.PublicarImportacao(dominio.OrigemContatos, rel)
	return rel, nil
}

// Listar leads.
func (s *Servico) Listar(ctx context.Context, busca, origem string, p pagina.Params) ([]dominio.Lead, string, error) {
	if origem != "" && !OrigemValida(origem) {
		return nil, "", erros.Campo("origem", "Origem inválida (use csv, colado, contatos ou mcp).")
	}
	return armazenamento.ListarLeads(ctx, s.banco.L(), armazenamento.FiltroLeads{Busca: busca, Origem: origem}, p)
}

// Obter um lead.
func (s *Servico) Obter(ctx context.Context, id string) (dominio.Lead, error) {
	return armazenamento.ObterLead(ctx, s.banco.L(), id)
}
