// Pacote funil implementa os funis de vendas (US1): funis, etapas, cards (posição do lead) e o
// histórico de movimentação, com os eventos funil.alterado/funil.movido e o fato entrou_etapa
// para as automações (specs/002-automacoes/data-model.md › Funil…Histórico).
package funil

import (
	"context"
	"database/sql"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/fatos"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/leads"
	"zapdesk/motor/internal/pagina"
	"zapdesk/motor/internal/relogio"
)

// Limites.
const (
	MaxEtapas   = 30
	CorPadrao   = "#8696A0"
	MaxNomeFun  = 60
	MaxNomeEtap = 40
)

var corHex = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// Origem de quem mexe no funil.
type Origem struct {
	Tipo        string // app | mcp | automacao
	AutomacaoID string
	ExecucaoID  string
}

// Servico de funis.
type Servico struct {
	banco      *armazenamento.Banco
	barramento *eventos.Barramento
	relogio    relogio.Relogio
	leads      *leads.Servico
	fatos      fatos.Emissor
}

// Novo cria o serviço.
func Novo(b *armazenamento.Banco, bar *eventos.Barramento, r relogio.Relogio, l *leads.Servico) *Servico {
	return &Servico{banco: b, barramento: bar, relogio: r, leads: l}
}

// DefinirReceptorFatos liga o despachante (gatilho entrou_etapa).
func (s *Servico) DefinirReceptorFatos(r fatos.Receptor) { s.fatos.DefinirReceptor(r) }

func (s *Servico) alterado(funilID *string) {
	s.barramento.Publicar(eventos.FunilAlterado, "", map[string]any{"funil_id": funilID})
}

// NovaEtapa na criação do funil.
type NovaEtapa struct {
	Nome string  `json:"nome"`
	Cor  *string `json:"cor"`
}

func validarNomeFunil(nome string) (string, error) {
	nome = strings.TrimSpace(nome)
	if n := utf8.RuneCountInString(nome); n < 1 || n > MaxNomeFun {
		return nome, erros.Campo("nome", "O nome do funil deve ter de 1 a 60 caracteres.")
	}
	return nome, nil
}

func validarEtapa(campo, nome string, cor *string) (string, string, error) {
	nome = strings.TrimSpace(nome)
	if n := utf8.RuneCountInString(nome); n < 1 || n > MaxNomeEtap {
		return nome, "", erros.Campo(campo+"nome", "O nome da etapa deve ter de 1 a 40 caracteres.")
	}
	c := CorPadrao
	if cor != nil && *cor != "" {
		if !corHex.MatchString(*cor) {
			return nome, "", erros.Campo(campo+"cor", "Use uma cor no formato #RRGGBB.")
		}
		c = strings.ToUpper(*cor)
	}
	return nome, c, nil
}

// Listar os funis (ordem) com etapas e contagens.
func (s *Servico) Listar(ctx context.Context) ([]dominio.Funil, error) {
	return armazenamento.ListarFunis(ctx, s.banco.L())
}

// Obter um funil.
func (s *Servico) Obter(ctx context.Context, id string) (dominio.Funil, error) {
	return armazenamento.ObterFunil(ctx, s.banco.L(), id)
}

// Criar um funil com as etapas iniciais.
func (s *Servico) Criar(ctx context.Context, nome string, etapas []NovaEtapa) (dominio.Funil, error) {
	nome, err := validarNomeFunil(nome)
	if err != nil {
		return dominio.Funil{}, err
	}
	if len(etapas) > MaxEtapas {
		return dominio.Funil{}, erros.Campo("etapas", "Um funil pode ter até 30 etapas.")
	}
	vistos := map[string]bool{}
	type etp struct{ nome, cor string }
	var lista []etp
	for i, e := range etapas {
		n, c, err := validarEtapa("etapas["+itoa(i)+"].", e.Nome, e.Cor)
		if err != nil {
			return dominio.Funil{}, err
		}
		chave := strings.ToLower(n)
		if vistos[chave] {
			return dominio.Funil{}, erros.Campo("etapas["+itoa(i)+"].nome", "Já existe uma etapa com esse nome neste funil.")
		}
		vistos[chave] = true
		lista = append(lista, etp{n, c})
	}
	agora := s.relogio.Agora()
	id := ids.NovoEm(agora)
	err = s.banco.Transacao(ctx, func(tx *sql.Tx) error {
		ordem, err := armazenamento.ProximaOrdemFunil(ctx, tx)
		if err != nil {
			return err
		}
		if err := armazenamento.InserirFunil(ctx, tx, id, nome, ordem, agora); err != nil {
			return err
		}
		for i, e := range lista {
			if err := armazenamento.InserirEtapa(ctx, tx, dominio.Etapa{ID: ids.NovoEm(agora), FunilID: id, Nome: e.nome, Cor: e.cor, Ordem: i}, agora); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return dominio.Funil{}, err
	}
	s.alterado(nil)
	return s.Obter(ctx, id)
}

// Editar nome e/ou ordem.
func (s *Servico) Editar(ctx context.Context, id string, nome *string, ordem *int) (dominio.Funil, error) {
	f, err := s.Obter(ctx, id)
	if err != nil {
		return f, err
	}
	if nome != nil {
		if f.Nome, err = validarNomeFunil(*nome); err != nil {
			return f, err
		}
	}
	if ordem != nil {
		if *ordem < 0 {
			return f, erros.Campo("ordem", "A ordem deve ser 0 ou maior.")
		}
		f.Ordem = *ordem
	}
	if err := armazenamento.AtualizarFunil(ctx, s.banco.E(), id, f.Nome, f.Ordem, s.relogio.Agora()); err != nil {
		return f, err
	}
	s.alterado(nil)
	return s.Obter(ctx, id)
}

// Excluir apaga o funil (etapas, cards e histórico).
func (s *Servico) Excluir(ctx context.Context, id string) error {
	ok, err := armazenamento.ExcluirFunil(ctx, s.banco.E(), id)
	if err != nil {
		return err
	}
	if !ok {
		return erros.NaoAchado("Funil")
	}
	s.alterado(nil)
	return nil
}

// CriarEtapa acrescenta uma etapa na posição (padrão: última).
func (s *Servico) CriarEtapa(ctx context.Context, funilID, nome string, cor *string, posicao *int) (dominio.Etapa, error) {
	f, err := s.Obter(ctx, funilID)
	if err != nil {
		return dominio.Etapa{}, err
	}
	if len(f.Etapas) >= MaxEtapas {
		return dominio.Etapa{}, erros.Campo("nome", "Um funil pode ter até 30 etapas.")
	}
	nome, c, err := validarEtapa("", nome, cor)
	if err != nil {
		return dominio.Etapa{}, err
	}
	ordem := len(f.Etapas)
	if posicao != nil {
		if *posicao < 0 || *posicao > len(f.Etapas) {
			return dominio.Etapa{}, erros.Campo("posicao", "Posição fora do funil.")
		}
		ordem = *posicao
	}
	agora := s.relogio.Agora()
	e := dominio.Etapa{ID: ids.NovoEm(agora), FunilID: funilID, Nome: nome, Cor: c, Ordem: ordem}
	err = s.banco.Transacao(ctx, func(tx *sql.Tx) error {
		if err := normalizarOrdem(ctx, tx, f.Etapas); err != nil {
			return err
		}
		if err := armazenamento.InserirEtapa(ctx, tx, e, agora); err != nil {
			return err
		}
		return armazenamento.TocarFunil(ctx, tx, funilID, agora)
	})
	if err != nil {
		return dominio.Etapa{}, err
	}
	s.alterado(&funilID)
	return armazenamento.ObterEtapa(ctx, s.banco.L(), e.ID)
}

func normalizarOrdem(ctx context.Context, ex armazenamento.Executor, etapas []dominio.Etapa) error {
	for i, e := range etapas {
		if e.Ordem != i {
			if err := armazenamento.DefinirOrdemEtapa(ctx, ex, e.ID, i); err != nil {
				return err
			}
		}
	}
	return nil
}

// ReordenarEtapas recebe todas as etapas na nova ordem.
func (s *Servico) ReordenarEtapas(ctx context.Context, funilID string, etapaIDs []string) (dominio.Funil, error) {
	f, err := s.Obter(ctx, funilID)
	if err != nil {
		return f, err
	}
	atuais := map[string]bool{}
	for _, e := range f.Etapas {
		atuais[e.ID] = true
	}
	vistos := map[string]bool{}
	for _, id := range etapaIDs {
		if !atuais[id] || vistos[id] {
			return f, erros.Campo("etapa_ids", "Envie todas as etapas do funil, cada uma uma vez.")
		}
		vistos[id] = true
	}
	if len(vistos) != len(atuais) {
		return f, erros.Campo("etapa_ids", "Envie todas as etapas do funil, cada uma uma vez.")
	}
	agora := s.relogio.Agora()
	err = s.banco.Transacao(ctx, func(tx *sql.Tx) error {
		for i, id := range etapaIDs {
			if err := armazenamento.DefinirOrdemEtapa(ctx, tx, id, i); err != nil {
				return err
			}
		}
		return armazenamento.TocarFunil(ctx, tx, funilID, agora)
	})
	if err != nil {
		return f, err
	}
	s.alterado(&funilID)
	return s.Obter(ctx, funilID)
}

// EditarEtapa altera nome e/ou cor.
func (s *Servico) EditarEtapa(ctx context.Context, id string, nome, cor *string) (dominio.Etapa, error) {
	e, err := armazenamento.ObterEtapa(ctx, s.banco.L(), id)
	if err != nil {
		return e, err
	}
	n := e.Nome
	if nome != nil {
		n = *nome
	}
	c := &e.Cor
	if cor != nil {
		c = cor
	}
	if e.Nome, e.Cor, err = validarEtapa("", n, c); err != nil {
		return e, err
	}
	if err := armazenamento.AtualizarEtapa(ctx, s.banco.E(), e); err != nil {
		return e, err
	}
	armazenamento.TocarFunil(ctx, s.banco.E(), e.FunilID, s.relogio.Agora())
	s.alterado(&e.FunilID)
	return armazenamento.ObterEtapa(ctx, s.banco.L(), id)
}

// ExcluirEtapa apaga a etapa; se houver cards, exige destino (mesmo funil) ou remover_cards.
func (s *Servico) ExcluirEtapa(ctx context.Context, id string, destinoID string, removerCards bool, o Origem) error {
	e, err := armazenamento.ObterEtapa(ctx, s.banco.L(), id)
	if err != nil {
		return err
	}
	leadsNaEtapa, err := armazenamento.LeadsNaEtapa(ctx, s.banco.L(), id)
	if err != nil {
		return err
	}
	var destino dominio.Etapa
	if len(leadsNaEtapa) > 0 {
		switch {
		case destinoID != "":
			destino, err = armazenamento.ObterEtapa(ctx, s.banco.L(), destinoID)
			if err != nil || destino.FunilID != e.FunilID || destino.ID == e.ID {
				return erros.Campo("destino_etapa_id", "Escolha outra etapa deste funil.")
			}
		case !removerCards:
			return erros.Campo("destino_etapa_id", "Escolha para onde mover os cards.")
		}
	}
	for _, lead := range leadsNaEtapa {
		if destino.ID != "" {
			if _, _, err := s.mover(ctx, e.FunilID, lead, destino.ID, o); err != nil {
				return err
			}
		} else if err := s.Remover(ctx, e.FunilID, lead, o); err != nil {
			return err
		}
	}
	f, _ := s.Obter(ctx, e.FunilID)
	err = s.banco.Transacao(ctx, func(tx *sql.Tx) error {
		if err := armazenamento.ExcluirEtapa(ctx, tx, e); err != nil {
			return err
		}
		var restantes []dominio.Etapa
		for _, x := range f.Etapas {
			if x.ID != e.ID {
				restantes = append(restantes, x)
			}
		}
		for i := range restantes {
			restantes[i].Ordem = -1
		}
		if err := normalizarOrdem(ctx, tx, restantes); err != nil {
			return err
		}
		return armazenamento.TocarFunil(ctx, tx, e.FunilID, s.relogio.Agora())
	})
	if err != nil {
		return err
	}
	s.alterado(&e.FunilID)
	return nil
}

// AlvoCard identifica o lead por exatamente um: lead_id, contato_id ou telefone.
type AlvoCard struct {
	LeadID    string
	ContatoID string
	Telefone  string
}

// ResolverLead encontra (ou cria) o lead do alvo.
func (s *Servico) ResolverLead(ctx context.Context, a AlvoCard, o Origem) (dominio.Lead, error) {
	n := 0
	for _, v := range []string{a.LeadID, a.ContatoID, a.Telefone} {
		if v != "" {
			n++
		}
	}
	if n != 1 {
		return dominio.Lead{}, erros.Campo("lead_id", "Informe exatamente um: lead_id, contato_id ou telefone.")
	}
	switch {
	case a.LeadID != "":
		return armazenamento.ObterLead(ctx, s.banco.L(), a.LeadID)
	case a.ContatoID != "":
		l, _, err := s.leads.GarantirLeadDoContato(ctx, a.ContatoID)
		return l, err
	default:
		origem := dominio.OrigemContatos
		if o.Tipo == dominio.OrigemMCPFunil {
			origem = dominio.OrigemMCP
		}
		l, _, err := s.leads.GarantirLeadTelefone(ctx, a.Telefone, nil, origem)
		return l, err
	}
}

// Mover coloca o lead na etapa (entra no funil ou muda de etapa). Devolve o card e se entrou
// (true) ou só mudou/manteve (false). Mesma etapa: sem efeito e sem histórico.
func (s *Servico) Mover(ctx context.Context, funilID string, a AlvoCard, etapaID string, o Origem) (dominio.Card, bool, error) {
	lead, err := s.ResolverLead(ctx, a, o)
	if err != nil {
		return dominio.Card{}, false, err
	}
	return s.mover(ctx, funilID, lead.ID, etapaID, o)
}

func (s *Servico) mover(ctx context.Context, funilID, leadID, etapaID string, o Origem) (dominio.Card, bool, error) {
	if _, err := armazenamento.ObterFunil(ctx, s.banco.L(), funilID); err != nil {
		return dominio.Card{}, false, err
	}
	destino, err := armazenamento.ObterEtapa(ctx, s.banco.L(), etapaID)
	if err != nil {
		return dominio.Card{}, false, erros.Campo("etapa_id", "Etapa não encontrada.")
	}
	if destino.FunilID != funilID {
		return dominio.Card{}, false, erros.Campo("etapa_id", "A etapa é de outro funil.")
	}
	atual, estava, err := armazenamento.ObterPosicao(ctx, s.banco.L(), leadID, funilID)
	if err != nil {
		return dominio.Card{}, false, err
	}
	if estava && atual.EtapaID == etapaID {
		c, _, err := armazenamento.ObterCard(ctx, s.banco.L(), leadID, funilID)
		return c, false, err
	}
	agora := s.relogio.Agora()
	mov := novoMovimento(leadID, funilID, o, agora)
	if estava {
		if origem, err := armazenamento.ObterEtapa(ctx, s.banco.L(), atual.EtapaID); err == nil {
			mov.EtapaOrigemID, mov.EtapaOrigemNome = &origem.ID, &origem.Nome
		}
	}
	mov.EtapaDestinoID, mov.EtapaDestinoNome = &destino.ID, &destino.Nome
	err = s.banco.Transacao(ctx, func(tx *sql.Tx) error {
		if err := armazenamento.DefinirPosicao(ctx, tx, armazenamento.Posicao{LeadID: leadID, FunilID: funilID, EtapaID: etapaID, Desde: agora}); err != nil {
			return err
		}
		return armazenamento.InserirMovimento(ctx, tx, mov)
	})
	if err != nil {
		return dominio.Card{}, false, err
	}
	card, _, err := armazenamento.ObterCard(ctx, s.banco.L(), leadID, funilID)
	if err != nil {
		return card, false, err
	}
	s.barramento.Publicar(eventos.FunilMovido, "", map[string]any{"movimento": mov, "card": card})
	f := fatos.Fato{Tipo: fatos.EntrouEtapa, FunilID: funilID, EtapaID: etapaID, LeadID: leadID, Em: agora}
	if estava {
		f.EtapaAnteriorID = atual.EtapaID
	}
	if card.ConversaID != nil {
		f.ConversaID = *card.ConversaID
	}
	if card.ContaID != nil {
		f.ContaID = *card.ContaID
	}
	s.fatos.Emitir(ctx, f)
	return card, !estava, nil
}

func novoMovimento(leadID, funilID string, o Origem, agora time.Time) dominio.MovimentoFunil {
	m := dominio.MovimentoFunil{ID: ids.NovoEm(agora), LeadID: leadID, FunilID: funilID, Origem: o.Tipo, Em: agora}
	if m.Origem == "" {
		m.Origem = dominio.OrigemApp
	}
	if o.AutomacaoID != "" {
		a := o.AutomacaoID
		m.AutomacaoID = &a
	}
	if o.ExecucaoID != "" {
		e := o.ExecucaoID
		m.ExecucaoID = &e
	}
	return m
}

// Remover tira o lead do funil (idempotente).
func (s *Servico) Remover(ctx context.Context, funilID, leadID string, o Origem) error {
	if _, err := armazenamento.ObterFunil(ctx, s.banco.L(), funilID); err != nil {
		return err
	}
	atual, estava, err := armazenamento.ObterPosicao(ctx, s.banco.L(), leadID, funilID)
	if err != nil || !estava {
		return err
	}
	agora := s.relogio.Agora()
	mov := novoMovimento(leadID, funilID, o, agora)
	if origem, err := armazenamento.ObterEtapa(ctx, s.banco.L(), atual.EtapaID); err == nil {
		mov.EtapaOrigemID, mov.EtapaOrigemNome = &origem.ID, &origem.Nome
	}
	err = s.banco.Transacao(ctx, func(tx *sql.Tx) error {
		if _, err := armazenamento.RemoverPosicao(ctx, tx, leadID, funilID); err != nil {
			return err
		}
		return armazenamento.InserirMovimento(ctx, tx, mov)
	})
	if err != nil {
		return err
	}
	s.barramento.Publicar(eventos.FunilMovido, "", map[string]any{"movimento": mov, "card": nil})
	return nil
}

// Posicao devolve o card do lead no funil (ok=false se fora).
func (s *Servico) Posicao(ctx context.Context, funilID, leadID string) (dominio.Card, bool, error) {
	return armazenamento.ObterCard(ctx, s.banco.L(), leadID, funilID)
}

// Cards pagina os cards de um funil.
func (s *Servico) Cards(ctx context.Context, funilID string, f armazenamento.FiltroCards, p pagina.Params) ([]dominio.Card, string, error) {
	if _, err := armazenamento.ObterFunil(ctx, s.banco.L(), funilID); err != nil {
		return nil, "", err
	}
	return armazenamento.ListarCards(ctx, s.banco.L(), funilID, f, p)
}

// Historico pagina o histórico de um funil.
func (s *Servico) Historico(ctx context.Context, funilID, leadID string, p pagina.Params) ([]dominio.MovimentoFunil, string, error) {
	if _, err := armazenamento.ObterFunil(ctx, s.banco.L(), funilID); err != nil {
		return nil, "", err
	}
	return armazenamento.ListarHistoricoFunil(ctx, s.banco.L(), funilID, leadID, p)
}

// FunilDoLead é um item de GET /leads/{id}/funis.
type FunilDoLead struct {
	Funil struct {
		ID   string `json:"id"`
		Nome string `json:"nome"`
	} `json:"funil"`
	Card *dominio.Card `json:"card"`
}

// FunisDoLead devolve todos os funis com o card do lead (ou null).
func (s *Servico) FunisDoLead(ctx context.Context, leadID string) ([]FunilDoLead, error) {
	if _, err := armazenamento.ObterLead(ctx, s.banco.L(), leadID); err != nil {
		return nil, err
	}
	funis, cards, err := armazenamento.FunisDoLead(ctx, s.banco.L(), leadID)
	if err != nil {
		return nil, err
	}
	l := []FunilDoLead{}
	for _, f := range funis {
		var it FunilDoLead
		it.Funil.ID, it.Funil.Nome = f.ID, f.Nome
		if c, ok := cards[f.ID]; ok {
			cc := c
			it.Card = &cc
		}
		l = append(l, it)
	}
	return l, nil
}

func itoa(i int) string { return strconv.Itoa(i) }
