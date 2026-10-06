// Pacote organizacao cuida de etiquetas, notas e templates (US5) com as validações de
// data-model.md e os eventos etiquetas.alteradas, templates.alterados e contato.atualizado.
package organizacao

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"unicode/utf8"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/fatos"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/relogio"
	"zapdesk/motor/internal/variaveis"
)

// Publicador de contato.atualizado (serviço de chat).
type Publicador interface {
	PublicarContato(ctx context.Context, id string)
}

// Servico de organização.
type Servico struct {
	banco      *armazenamento.Banco
	barramento *eventos.Barramento
	relogio    relogio.Relogio
	contatos   Publicador
	fatos      fatos.Emissor
}

// DefinirReceptorFatos liga o despachante de automações (gatilho etiqueta).
func (s *Servico) DefinirReceptorFatos(r fatos.Receptor) { s.fatos.DefinirReceptor(r) }

// Novo cria o serviço.
func Novo(b *armazenamento.Banco, bar *eventos.Barramento, r relogio.Relogio, p Publicador) *Servico {
	return &Servico{banco: b, barramento: bar, relogio: r, contatos: p}
}

var corHex = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// ---------------------------------------------------------------------------
// Etiquetas
// ---------------------------------------------------------------------------

// ListarEtiquetas lista todas.
func (s *Servico) ListarEtiquetas(ctx context.Context) ([]dominio.Etiqueta, error) {
	return armazenamento.ListarEtiquetas(ctx, s.banco.L())
}

func validarEtiqueta(nome, cor string) map[string]string {
	c := map[string]string{}
	if n := utf8.RuneCountInString(nome); n < 1 || n > 30 {
		c["nome"] = "O nome da etiqueta deve ter de 1 a 30 caracteres."
	}
	if !corHex.MatchString(cor) {
		c["cor"] = "Use uma cor no formato #RRGGBB."
	}
	return c
}

// CriarEtiqueta cria uma etiqueta.
func (s *Servico) CriarEtiqueta(ctx context.Context, nome, cor string) (dominio.Etiqueta, error) {
	nome = strings.TrimSpace(nome)
	if c := validarEtiqueta(nome, cor); len(c) > 0 {
		return dominio.Etiqueta{}, erros.Campos(c)
	}
	agora := s.relogio.Agora()
	e := dominio.Etiqueta{ID: ids.NovoEm(agora), Nome: nome, Cor: cor}
	if err := armazenamento.InserirEtiqueta(ctx, s.banco.E(), e, agora); err != nil {
		return e, err
	}
	s.barramento.Publicar(eventos.EtiquetasAlteradas, "", struct{}{})
	return armazenamento.ObterEtiqueta(ctx, s.banco.L(), e.ID)
}

// EditarEtiqueta altera nome e/ou cor.
func (s *Servico) EditarEtiqueta(ctx context.Context, id string, nome, cor *string) (dominio.Etiqueta, error) {
	e, err := armazenamento.ObterEtiqueta(ctx, s.banco.L(), id)
	if err != nil {
		return e, err
	}
	if nome != nil {
		e.Nome = strings.TrimSpace(*nome)
	}
	if cor != nil {
		e.Cor = *cor
	}
	if c := validarEtiqueta(e.Nome, e.Cor); len(c) > 0 {
		return e, erros.Campos(c)
	}
	if err := armazenamento.AtualizarEtiqueta(ctx, s.banco.E(), e); err != nil {
		return e, err
	}
	s.barramento.Publicar(eventos.EtiquetasAlteradas, "", struct{}{})
	s.publicarContatosDaEtiqueta(ctx, id)
	return armazenamento.ObterEtiqueta(ctx, s.banco.L(), id)
}

func (s *Servico) publicarContatosDaEtiqueta(ctx context.Context, id string) {
	contatos, _ := armazenamento.ContatosDaEtiqueta(ctx, s.banco.L(), id)
	for _, c := range contatos {
		s.contatos.PublicarContato(ctx, c)
	}
}

// ExcluirEtiqueta apaga a etiqueta (e a remove dos contatos).
func (s *Servico) ExcluirEtiqueta(ctx context.Context, id string) error {
	contatos, _ := armazenamento.ContatosDaEtiqueta(ctx, s.banco.L(), id)
	ok, err := armazenamento.ExcluirEtiqueta(ctx, s.banco.E(), id)
	if err != nil {
		return err
	}
	if !ok {
		return erros.NaoAchada("Etiqueta")
	}
	s.barramento.Publicar(eventos.EtiquetasAlteradas, "", struct{}{})
	for _, c := range contatos {
		s.contatos.PublicarContato(ctx, c)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Contatos
// ---------------------------------------------------------------------------

// TamanhoMaximoNotas do contato.
const TamanhoMaximoNotas = 10000

// DefinirNotas grava as notas.
func (s *Servico) DefinirNotas(ctx context.Context, contatoID string, notas *string) (dominio.Contato, error) {
	if _, err := armazenamento.ObterContato(ctx, s.banco.L(), contatoID); err != nil {
		return dominio.Contato{}, err
	}
	if notas != nil && utf8.RuneCountInString(*notas) > TamanhoMaximoNotas {
		return dominio.Contato{}, erros.Campo("notas", "As notas podem ter até 10.000 caracteres.")
	}
	if notas != nil && *notas == "" {
		notas = nil
	}
	if err := armazenamento.DefinirNotas(ctx, s.banco.E(), contatoID, notas, s.relogio.Agora()); err != nil {
		return dominio.Contato{}, err
	}
	s.contatos.PublicarContato(ctx, contatoID)
	return armazenamento.ObterContato(ctx, s.banco.L(), contatoID)
}

// DefinirEtiquetas substitui as etiquetas do contato. A diferença (adicionadas/removidas) vira
// fato para as automações (gatilho "etiqueta"), com a cadeia causal do contexto.
func (s *Servico) DefinirEtiquetas(ctx context.Context, contatoID string, etiquetaIDs []string) (dominio.Contato, error) {
	antes, err := armazenamento.ObterContato(ctx, s.banco.L(), contatoID)
	if err != nil {
		return dominio.Contato{}, err
	}
	for _, id := range etiquetaIDs {
		if _, err := armazenamento.ObterEtiqueta(ctx, s.banco.L(), id); err != nil {
			return dominio.Contato{}, erros.Campo("etiqueta_ids", "Etiqueta não encontrada: "+id)
		}
	}
	err = s.banco.Transacao(ctx, func(tx *sql.Tx) error {
		return armazenamento.DefinirEtiquetasContato(ctx, tx, contatoID, etiquetaIDs)
	})
	if err != nil {
		return dominio.Contato{}, err
	}
	s.contatos.PublicarContato(ctx, contatoID)
	s.barramento.Publicar(eventos.EtiquetasAlteradas, "", struct{}{})
	depois, err := armazenamento.ObterContato(ctx, s.banco.L(), contatoID)
	if err == nil {
		s.emitirDiferenca(ctx, antes, depois)
	}
	return depois, err
}

func (s *Servico) emitirDiferenca(ctx context.Context, antes, depois dominio.Contato) {
	tinha, tem := map[string]bool{}, map[string]bool{}
	for _, e := range antes.Etiquetas {
		tinha[e.ID] = true
	}
	for _, e := range depois.Etiquetas {
		tem[e.ID] = true
	}
	base := fatos.Fato{Tipo: fatos.Etiqueta, ContaID: depois.ContaID, ContatoID: depois.ID, Em: s.relogio.Agora()}
	if depois.ConversaID != nil {
		base.ConversaID = *depois.ConversaID
	}
	if depois.Lead != nil {
		base.LeadID = depois.Lead.ID
	}
	for _, e := range depois.Etiquetas {
		if !tinha[e.ID] {
			f := base
			f.EtiquetaID, f.Evento = e.ID, "adicionada"
			s.fatos.Emitir(ctx, f)
		}
	}
	for _, e := range antes.Etiquetas {
		if !tem[e.ID] {
			f := base
			f.EtiquetaID, f.Evento = e.ID, "removida"
			s.fatos.Emitir(ctx, f)
		}
	}
}

// ---------------------------------------------------------------------------
// Templates
// ---------------------------------------------------------------------------

func (s *Servico) montarTemplate(ctx context.Context, t dominio.Template) (dominio.Template, error) {
	t.Variaveis = variaveis.Extrair(t.Texto)
	a, err := armazenamento.ArquivoOpcional(ctx, s.banco.L(), t.ArquivoID)
	t.Arquivo = a
	return t, err
}

// ListarTemplates lista (busca por trecho do nome).
func (s *Servico) ListarTemplates(ctx context.Context, busca string) ([]dominio.Template, error) {
	lista, err := armazenamento.ListarTemplates(ctx, s.banco.L(), busca)
	if err != nil {
		return nil, err
	}
	for i := range lista {
		if lista[i], err = s.montarTemplate(ctx, lista[i]); err != nil {
			return nil, err
		}
	}
	return lista, nil
}

// ObterTemplate devolve um template.
func (s *Servico) ObterTemplate(ctx context.Context, id string) (dominio.Template, error) {
	t, err := armazenamento.ObterTemplate(ctx, s.banco.L(), id)
	if err != nil {
		return t, err
	}
	return s.montarTemplate(ctx, t)
}

func (s *Servico) validarTemplate(ctx context.Context, t dominio.Template) error {
	c := map[string]string{}
	if n := utf8.RuneCountInString(t.Nome); n < 1 || n > 60 {
		c["nome"] = "O nome do template deve ter de 1 a 60 caracteres."
	}
	if n := utf8.RuneCountInString(t.Texto); strings.TrimSpace(t.Texto) == "" || n > 4096 {
		c["texto"] = "O texto deve ter de 1 a 4.096 caracteres."
	}
	if len(c) > 0 {
		return erros.Campos(c)
	}
	if t.ArquivoID != nil {
		if _, err := armazenamento.ObterArquivo(ctx, s.banco.L(), *t.ArquivoID); err != nil {
			return erros.Novo(erros.NaoEncontrado, "Arquivo não encontrado.")
		}
	}
	return nil
}

// CriarTemplate cria.
func (s *Servico) CriarTemplate(ctx context.Context, nome, texto string, arquivoID *string) (dominio.Template, error) {
	agora := s.relogio.Agora()
	if arquivoID != nil && *arquivoID == "" {
		arquivoID = nil
	}
	t := dominio.Template{ID: ids.NovoEm(agora), Nome: strings.TrimSpace(nome), Texto: texto, ArquivoID: arquivoID, CriadoEm: agora, AtualizadoEm: agora}
	if err := s.validarTemplate(ctx, t); err != nil {
		return t, err
	}
	if err := armazenamento.InserirTemplate(ctx, s.banco.E(), t); err != nil {
		return t, err
	}
	s.barramento.Publicar(eventos.TemplatesAlterados, "", struct{}{})
	return s.ObterTemplate(ctx, t.ID)
}

// AlteracaoTemplate: campos presentes no PATCH (ArquivoDefinido distingue null de ausente).
type AlteracaoTemplate struct {
	Nome            *string
	Texto           *string
	ArquivoID       *string
	ArquivoDefinido bool
}

// EditarTemplate altera.
func (s *Servico) EditarTemplate(ctx context.Context, id string, a AlteracaoTemplate) (dominio.Template, error) {
	t, err := armazenamento.ObterTemplate(ctx, s.banco.L(), id)
	if err != nil {
		return t, err
	}
	if a.Nome != nil {
		t.Nome = strings.TrimSpace(*a.Nome)
	}
	if a.Texto != nil {
		t.Texto = *a.Texto
	}
	if a.ArquivoDefinido {
		t.ArquivoID = a.ArquivoID
		if t.ArquivoID != nil && *t.ArquivoID == "" {
			t.ArquivoID = nil
		}
	}
	if err := s.validarTemplate(ctx, t); err != nil {
		return t, err
	}
	if err := armazenamento.AtualizarTemplate(ctx, s.banco.E(), t, s.relogio.Agora()); err != nil {
		return t, err
	}
	s.barramento.Publicar(eventos.TemplatesAlterados, "", struct{}{})
	return s.ObterTemplate(ctx, id)
}

// ExcluirTemplate apaga.
func (s *Servico) ExcluirTemplate(ctx context.Context, id string) error {
	ok, err := armazenamento.ExcluirTemplate(ctx, s.banco.E(), id)
	if err != nil {
		return err
	}
	if !ok {
		return erros.NaoAchado("Template")
	}
	s.barramento.Publicar(eventos.TemplatesAlterados, "", struct{}{})
	return nil
}
