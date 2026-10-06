package disparos

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"zapdesk/motor/internal/agendamento"
	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/chat"
	"zapdesk/motor/internal/contas"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/fatos"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/leads"
	"zapdesk/motor/internal/pagina"
	"zapdesk/motor/internal/relogio"
	"zapdesk/motor/internal/variaveis"
)

// Servico de disparos.
type Servico struct {
	banco      *armazenamento.Banco
	barramento *eventos.Barramento
	relogio    relogio.Relogio
	log        zerolog.Logger
	contas     *contas.Gerenciador
	chat       *chat.Servico
	leads      *leads.Servico
	ctx        context.Context

	rndMu sync.Mutex
	rnd   *rand.Rand

	mu           sync.Mutex
	execs        map[string]*executor
	ultimoAtivos int
	publicacoes  map[string]*publicacao
	estimativas  map[string]estimativaCache
	wg           sync.WaitGroup
	encerrando   bool

	fatos fatos.Emissor
}

// DefinirReceptorFatos liga o despachante de automações (gatilho disparo_respondeu).
func (s *Servico) DefinirReceptorFatos(r fatos.Receptor) { s.fatos.DefinirReceptor(r) }

type publicacao struct {
	ultima   time.Time
	agendada bool
}

type estimativaCache struct {
	chave string
	valor *time.Time
}

// Opcoes do serviço.
type Opcoes struct {
	Semente int64 // 0 = aleatória
}

// Novo cria o serviço.
func Novo(ctx context.Context, banco *armazenamento.Banco, bar *eventos.Barramento, rel relogio.Relogio, log zerolog.Logger,
	ct *contas.Gerenciador, ch *chat.Servico, ld *leads.Servico, o Opcoes) *Servico {
	semente := o.Semente
	if semente == 0 {
		semente = time.Now().UnixNano()
	}
	s := &Servico{banco: banco, barramento: bar, relogio: rel, log: log.With().Str("componente", "disparos").Logger(),
		contas: ct, chat: ch, leads: ld, ctx: ctx, rnd: rand.New(rand.NewSource(semente)),
		execs: map[string]*executor{}, ultimoAtivos: -1, publicacoes: map[string]*publicacao{}, estimativas: map[string]estimativaCache{}}
	ct.AoMudarEstado(s.contaMudou)
	ch.AdicionarOuvinte(acompanhamento{s})
	return s
}

// ---------------------------------------------------------------------------
// Montagem do JSON
// ---------------------------------------------------------------------------

// Montar completa contadores, arquivo, fila, estimativa e aviso.
func (s *Servico) Montar(ctx context.Context, d dominio.Disparo) (dominio.Disparo, error) {
	c, err := armazenamento.ContadoresDisparo(ctx, s.banco.L(), d.ID)
	if err != nil {
		return d, err
	}
	d.Contadores = c
	if d.Arquivo, err = armazenamento.ArquivoOpcional(ctx, s.banco.L(), d.ArquivoID); err != nil {
		return d, err
	}
	d.AvisoRitmoAgressivo = AvisoRitmoAgressivo(d.Ritmo)
	d.NaFila = s.naFila(ctx, d)
	d.EstimativaTerminoEm = s.estimativa(ctx, d)
	if d.ValoresPadrao == nil {
		d.ValoresPadrao = map[string]string{}
	}
	return d, nil
}

func (s *Servico) elegivel(d dominio.Disparo, agora time.Time) bool {
	return d.InicioEm == nil || !d.InicioEm.After(agora)
}

func antes(a, b dominio.Disparo) bool {
	fa, fb := a.CriadoEm, b.CriadoEm
	if a.FilaDesde != nil {
		fa = *a.FilaDesde
	}
	if b.FilaDesde != nil {
		fb = *b.FilaDesde
	}
	if !fa.Equal(fb) {
		return fa.Before(fb)
	}
	return a.ID < b.ID
}

func (s *Servico) naFila(ctx context.Context, d dominio.Disparo) bool {
	agora := s.relogio.Agora()
	if d.Estado != dominio.DisparoAgendado || !s.elegivel(d, agora) {
		return false
	}
	outros, err := armazenamento.DisparosDaConta(ctx, s.banco.L(), d.ContaID, dominio.DisparoEnviando, dominio.DisparoForaDaJanela, dominio.DisparoAgendado)
	if err != nil {
		return false
	}
	for _, o := range outros {
		if o.ID == d.ID {
			continue
		}
		if o.Estado != dominio.DisparoAgendado {
			return true
		}
		if s.elegivel(o, agora) && antes(o, d) {
			return true
		}
	}
	return false
}

func (s *Servico) estadoAgenda(ctx context.Context, d dominio.Disparo) agendamento.Estado {
	e := agendamento.Estado{EnviadosDesdePausa: d.EnviadosDesdePausa}
	if d.UltimoEnvioEm != nil {
		e.UltimoEnvio = *d.UltimoEnvioEm
	}
	e.EnviosRecentes, _ = armazenamento.EnviosRecentes(ctx, s.banco.L(), d.ID, s.relogio.Agora().Add(-25*time.Hour))
	return e
}

func (s *Servico) estimativa(ctx context.Context, d dominio.Disparo) *time.Time {
	switch d.Estado {
	case dominio.DisparoRascunho, dominio.DisparoAgendado, dominio.DisparoEnviando, dominio.DisparoForaDaJanela:
	default:
		return nil
	}
	pend := d.Contadores.Pendente
	if pend == 0 {
		return nil
	}
	agora := s.relogio.Agora()
	chave := fmt.Sprintf("%s|%d|%v|%v|%s|%d", d.Estado, pend, d.UltimoEnvioEm, d.ProximoEnvioEm, d.Janela, agora.Unix()/60)
	s.mu.Lock()
	if c, ok := s.estimativas[d.ID]; ok && c.chave == chave {
		s.mu.Unlock()
		return c.valor
	}
	s.mu.Unlock()
	cfg := ConfigAgenda(d)
	e := s.estadoAgenda(ctx, d)
	if d.ProximoEnvioEm != nil && d.ProximoEnvioEm.After(agora) {
		cfg.InicioEm = *d.ProximoEnvioEm
		e.UltimoEnvio = time.Time{}
	}
	fim := agendamento.Estimativa(e, agora, cfg, pend)
	var v *time.Time
	if !fim.IsZero() {
		v = &fim
	}
	s.mu.Lock()
	s.estimativas[d.ID] = estimativaCache{chave: chave, valor: v}
	s.mu.Unlock()
	return v
}

// Obter devolve o disparo montado.
func (s *Servico) Obter(ctx context.Context, id string) (dominio.Disparo, error) {
	d, err := armazenamento.ObterDisparo(ctx, s.banco.L(), id)
	if err != nil {
		return d, err
	}
	return s.Montar(ctx, d)
}

// Listar pagina os disparos.
func (s *Servico) Listar(ctx context.Context, contaID, estado string, p pagina.Params) ([]dominio.Disparo, string, error) {
	lista, prox, err := armazenamento.ListarDisparos(ctx, s.banco.L(), contaID, estado, p)
	if err != nil {
		return nil, "", err
	}
	for i := range lista {
		if lista[i], err = s.Montar(ctx, lista[i]); err != nil {
			return nil, "", err
		}
	}
	return lista, prox, nil
}

// Destinatarios pagina os destinatários.
func (s *Servico) Destinatarios(ctx context.Context, id string, f armazenamento.FiltroDestinatarios, p pagina.Params) ([]dominio.Destinatario, string, error) {
	if _, err := armazenamento.ObterDisparo(ctx, s.banco.L(), id); err != nil {
		return nil, "", err
	}
	return armazenamento.ListarDestinatarios(ctx, s.banco.L(), id, f, p)
}

// ---------------------------------------------------------------------------
// Publicação
// ---------------------------------------------------------------------------

// publicarDisparo emite disparo.atualizado: imediato em mudança de estado; no máximo 1/s durante
// o envio.
func (s *Servico) publicarDisparo(id string, imediato bool) {
	s.mu.Lock()
	p := s.publicacoes[id]
	if p == nil {
		p = &publicacao{}
		s.publicacoes[id] = p
	}
	if !imediato && time.Since(p.ultima) < time.Second {
		if !p.agendada {
			p.agendada = true
			espera := time.Second - time.Since(p.ultima)
			time.AfterFunc(espera, func() {
				s.mu.Lock()
				p.agendada = false
				s.mu.Unlock()
				s.publicarDisparo(id, true)
			})
		}
		s.mu.Unlock()
		return
	}
	p.ultima = time.Now()
	s.mu.Unlock()
	ctx := context.Background()
	d, err := s.Obter(ctx, id)
	if err != nil {
		return
	}
	s.barramento.Publicar(eventos.DisparoAtualizado, d.ContaID, d)
	if Final(d.Estado) {
		s.mu.Lock()
		delete(s.estimativas, id)
		s.mu.Unlock()
	}
}

func (s *Servico) publicarFinalizado(id string) {
	d, err := s.Obter(context.Background(), id)
	if err != nil {
		return
	}
	resumo := fmt.Sprintf("Disparo concluído: %d enviados, %d falharam",
		d.Contadores.Enviado+d.Contadores.Entregue+d.Contadores.Lido+d.Contadores.Respondeu, d.Contadores.Falhou)
	if d.Estado == dominio.DisparoCancelado {
		resumo = fmt.Sprintf("Disparo cancelado: %d enviados, %d falharam",
			d.Contadores.Enviado+d.Contadores.Entregue+d.Contadores.Lido+d.Contadores.Respondeu, d.Contadores.Falhou)
	}
	s.barramento.Publicar(eventos.DisparoFinalizado, d.ContaID, map[string]any{"disparo": d, "resumo": resumo})
}

func (s *Servico) publicarDestinatario(contaID, id string) {
	d, err := armazenamento.ObterDestinatario(context.Background(), s.banco.L(), id)
	if err != nil {
		return
	}
	s.barramento.Publicar(eventos.DestinatarioAtualizado, contaID, d)
}

// publicarAtivos emite disparos.ativos quando o total muda.
func (s *Servico) publicarAtivos() {
	n, err := armazenamento.ContarAtivos(context.Background(), s.banco.L())
	if err != nil {
		return
	}
	s.mu.Lock()
	mudou := n != s.ultimoAtivos
	s.ultimoAtivos = n
	s.mu.Unlock()
	if mudou {
		s.barramento.Publicar(eventos.DisparosAtivos, "", map[string]int{"total": n})
	}
}

// DisparosAtivos devolve o total para GET /v1/sistema.
func (s *Servico) DisparosAtivos() int {
	n, _ := armazenamento.ContarAtivos(context.Background(), s.banco.L())
	return n
}

// mudar aplica uma ação e publica.
func (s *Servico) mudar(ctx context.Context, d dominio.Disparo, acao string, motivo *string) (bool, error) {
	para, err := Transicao(d.Estado, acao)
	if err != nil {
		return false, err
	}
	ok, err := armazenamento.MudarEstadoDisparo(ctx, s.banco.E(), d.ID, d.Estado, para, motivo, s.relogio.Agora())
	if err != nil || !ok {
		return ok, err
	}
	s.publicarDisparo(d.ID, true)
	if Final(para) {
		s.publicarFinalizado(d.ID)
	}
	s.publicarAtivos()
	return true, nil
}

// ---------------------------------------------------------------------------
// Criação e validação
// ---------------------------------------------------------------------------

type resolvido struct {
	mensagem   string
	arquivoID  *string
	leads      []dominio.Lead
	relatorio  *dominio.RelatorioImportacao
	previaID   string
	importados []importacaoFeita
}

type importacaoFeita struct {
	origem string
	rel    dominio.RelatorioImportacao
}

// mensagemEArquivo resolve texto/anexo (template copia texto e anexo).
func (s *Servico) mensagemEArquivo(ctx context.Context, n NovoDisparo) (string, *string, error) {
	mensagem := ""
	if n.Mensagem != nil {
		mensagem = *n.Mensagem
	}
	arquivo := n.ArquivoID
	if arquivo != nil && *arquivo == "" {
		arquivo = nil
	}
	if n.TemplateID != nil && *n.TemplateID != "" {
		t, err := armazenamento.ObterTemplate(ctx, s.banco.L(), *n.TemplateID)
		if err != nil {
			return "", nil, erros.Novo(erros.NaoEncontrado, "Template não encontrado.")
		}
		if strings.TrimSpace(mensagem) == "" {
			mensagem = t.Texto
		}
		if arquivo == nil {
			arquivo = t.ArquivoID
		}
	}
	if arquivo != nil {
		if _, err := armazenamento.ObterArquivo(ctx, s.banco.L(), *arquivo); err != nil {
			return "", nil, erros.Novo(erros.NaoEncontrado, "Arquivo não encontrado.")
		}
	}
	return mensagem, arquivo, nil
}

// resolverDestinatarios importa/une os leads dentro da transação.
func (s *Servico) resolverDestinatarios(ctx context.Context, tx *sql.Tx, n NovoDisparo) (*resolvido, error) {
	r := &resolvido{}
	var idsL []string
	idsL = append(idsL, n.Destinatarios.LeadIDs...)
	if len(n.Destinatarios.ContatoIDs) > 0 || len(n.Destinatarios.EtiquetaIDs) > 0 {
		linhas, err := s.leads.LinhasDeContatos(ctx, n.ContaID, n.Destinatarios.ContatoIDs, n.Destinatarios.EtiquetaIDs)
		if err != nil {
			return nil, err
		}
		rel, err := s.leads.Importador.ImportarTx(ctx, tx, linhas, dominio.OrigemContatos, "")
		if err != nil {
			return nil, err
		}
		r.importados = append(r.importados, importacaoFeita{dominio.OrigemContatos, rel})
		idsL = append(idsL, rel.LeadIDs...)
	}
	if n.Destinatarios.Importar != nil {
		corpo := *n.Destinatarios.Importar
		linhas, origem, previaID, err := s.leads.PrepararLinhas(corpo)
		if err != nil {
			return nil, err
		}
		rel, err := s.leads.Importador.ImportarTx(ctx, tx, linhas, origem, corpo.DDIPadrao)
		if err != nil {
			return nil, err
		}
		r.relatorio, r.previaID = &rel, previaID
		r.importados = append(r.importados, importacaoFeita{origem, rel})
		idsL = append(idsL, rel.LeadIDs...)
	}
	vistos := map[string]bool{}
	unicos := make([]string, 0, len(idsL))
	for _, id := range idsL {
		if !vistos[id] {
			vistos[id] = true
			unicos = append(unicos, id)
		}
	}
	lista, err := armazenamento.LeadsPorIDs(ctx, tx, unicos)
	if err != nil {
		return nil, err
	}
	r.leads = lista
	return r, nil
}

// RespostaValidacao de POST /v1/disparos/validar.
type RespostaValidacao struct {
	TotalDestinatarios  int                  `json:"total_destinatarios"`
	Variaveis           []string             `json:"variaveis"`
	Faltando            []variaveis.Faltante `json:"faltando"`
	Previa              *Previa              `json:"previa"`
	EstimativaTerminoEm *time.Time           `json:"estimativa_termino_em"`
	AvisoRitmoAgressivo bool                 `json:"aviso_ritmo_agressivo"`
	ErrosCampos         map[string]string    `json:"erros_campos"`
}

// Previa do primeiro destinatário.
type Previa struct {
	Telefone       string  `json:"telefone"`
	Nome           *string `json:"nome"`
	TextoResolvido string  `json:"texto_resolvido"`
}

// Validar resolve tudo sem gravar (importações são desfeitas).
func (s *Servico) Validar(ctx context.Context, n NovoDisparo) (RespostaValidacao, error) {
	resp := RespostaValidacao{Variaveis: []string{}, Faltando: []variaveis.Faltante{}, ErrosCampos: map[string]string{}}
	mensagem, _, err := s.mensagemEArquivo(ctx, n)
	if err != nil {
		if e, ok := erros.Como(err); ok && e.Codigo == erros.NaoEncontrado {
			resp.ErrosCampos["template_id"] = e.Mensagem
		} else {
			return resp, err
		}
	}
	for k, v := range ValidarCampos(n, mensagem) {
		resp.ErrosCampos[k] = v
	}
	if n.ContaID != "" {
		if _, err := armazenamento.ObterConta(ctx, s.banco.L(), n.ContaID); err != nil {
			resp.ErrosCampos["conta_id"] = "Conta não encontrada."
		}
	}
	resp.Variaveis = variaveis.Extrair(mensagem)
	if n.Ritmo != nil {
		resp.AvisoRitmoAgressivo = AvisoRitmoAgressivo(*n.Ritmo)
	}
	tx, err := s.banco.E().BeginTx(ctx, nil)
	if err != nil {
		return resp, err
	}
	defer tx.Rollback()
	r, err := s.resolverDestinatarios(ctx, tx, n)
	if err != nil {
		if e, ok := erros.Como(err); ok {
			for k, v := range camposDoErro(e) {
				resp.ErrosCampos[k] = v
			}
			return resp, nil
		}
		return resp, err
	}
	resp.TotalDestinatarios = len(r.leads)
	if len(r.leads) == 0 {
		resp.ErrosCampos["destinatarios"] = "Escolha pelo menos um destinatário válido."
	}
	resp.Faltando = variaveis.Faltando(mensagem, r.leads, n.ValoresPadrao)
	if len(r.leads) > 0 {
		l := r.leads[0]
		vals, _ := variaveis.ValoresDoLead(resp.Variaveis, l, n.ValoresPadrao)
		resp.Previa = &Previa{Telefone: l.Telefone, Nome: l.Nome, TextoResolvido: variaveis.Resolver(mensagem, vals)}
	}
	if n.Ritmo != nil && len(resp.ErrosCampos) == 0 && len(r.leads) > 0 {
		d := dominio.Disparo{Ritmo: *n.Ritmo, InicioEm: n.InicioEm, Janela: n.Janela}
		fim := agendamento.Estimativa(agendamento.Estado{}, s.relogio.Agora(), ConfigAgenda(d), len(r.leads))
		resp.EstimativaTerminoEm = &fim
	}
	return resp, nil
}

func camposDoErro(e *erros.Erro) map[string]string {
	res := map[string]string{}
	if c, ok := e.Detalhes["campos"].(map[string]string); ok {
		for k, v := range c {
			res["destinatarios."+k] = v
		}
	}
	if len(res) == 0 {
		res["destinatarios"] = e.Mensagem
	}
	return res
}

const maxFaltandoDetalhes = 100

func erroFaltando(f []variaveis.Faltante) error {
	contagem := map[string]int{}
	for _, x := range f {
		for _, v := range x.Variaveis {
			contagem[v]++
		}
	}
	var partes []string
	for v, n := range contagem {
		if n == 1 {
			partes = append(partes, fmt.Sprintf("1 contato sem {%s}", v))
		} else {
			partes = append(partes, fmt.Sprintf("%d contatos sem {%s}", n, v))
		}
	}
	sort.Strings(partes)
	lista := f
	if len(lista) > maxFaltandoDetalhes {
		lista = lista[:maxFaltandoDetalhes]
	}
	return erros.ComDetalhes(erros.VariaveisFaltando, strings.Join(partes, "; ")+". Informe um valor padrão ou corrija os contatos.",
		map[string]any{"faltando": lista, "total": len(f)})
}

// Criar grava o disparo (rascunho, ou já agendado com iniciar=true).
func (s *Servico) Criar(ctx context.Context, n NovoDisparo) (dominio.Disparo, error) {
	if strings.TrimSpace(n.ContaID) == "" {
		return dominio.Disparo{}, erros.Campo("conta_id", "Escolha a conta que vai enviar.")
	}
	if _, err := armazenamento.ObterConta(ctx, s.banco.L(), n.ContaID); err != nil {
		return dominio.Disparo{}, erros.Novo(erros.NaoEncontrado, "Conta não encontrada.")
	}
	mensagem, arquivoID, err := s.mensagemEArquivo(ctx, n)
	if err != nil {
		return dominio.Disparo{}, err
	}
	if c := ValidarCampos(n, mensagem); len(c) > 0 {
		return dominio.Disparo{}, erros.Campos(c)
	}
	if n.Iniciar {
		if _, err := s.contas.Cliente(ctx, n.ContaID); err != nil {
			return dominio.Disparo{}, err
		}
	}
	agora := s.relogio.Agora()
	d := dominio.Disparo{
		ID: ids.NovoEm(agora), ContaID: n.ContaID, Mensagem: mensagem, ArquivoID: arquivoID, Ritmo: *n.Ritmo,
		InicioEm: n.InicioEm, Janela: n.Janela, FalhasSeguidasMax: FalhasSeguidasPadrao, ValoresPadrao: n.ValoresPadrao,
		Estado: dominio.DisparoRascunho, Origem: n.Origem, CriadoEm: agora,
	}
	if d.ValoresPadrao == nil {
		d.ValoresPadrao = map[string]string{}
	}
	if d.Origem == "" {
		d.Origem = "app"
	}
	if n.FalhasSeguidasMax != nil {
		d.FalhasSeguidasMax = *n.FalhasSeguidasMax
	}
	if n.Nome != nil {
		d.Nome = strings.TrimSpace(*n.Nome)
	} else {
		d.Nome = "Disparo " + agora.Format("02/01 15:04")
	}

	var r *resolvido
	err = s.banco.Transacao(ctx, func(tx *sql.Tx) error {
		var err error
		r, err = s.resolverDestinatarios(ctx, tx, n)
		if err != nil {
			return err
		}
		if len(r.leads) == 0 {
			return erros.Campo("destinatarios", "Escolha pelo menos um destinatário válido.")
		}
		vars := variaveis.Extrair(mensagem)
		if n.Iniciar {
			if f := variaveis.Faltando(mensagem, r.leads, d.ValoresPadrao); len(f) > 0 {
				return erroFaltando(f)
			}
		}
		if err := armazenamento.InserirDisparo(ctx, tx, d); err != nil {
			return err
		}
		dests := make([]dominio.Destinatario, len(r.leads))
		for i, l := range r.leads {
			vals, _ := variaveis.ValoresDoLead(vars, l, d.ValoresPadrao)
			dests[i] = dominio.Destinatario{ID: ids.NovoEm(agora), DisparoID: d.ID, LeadID: l.ID, Ordem: i + 1, Telefone: l.Telefone,
				Nome: l.Nome, Variaveis: vals}
		}
		if err := armazenamento.InserirDestinatarios(ctx, tx, dests); err != nil {
			return err
		}
		if n.Iniciar {
			if _, err := armazenamento.MudarEstadoDisparo(ctx, tx, d.ID, dominio.DisparoRascunho, dominio.DisparoAgendado, nil, agora); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return dominio.Disparo{}, err
	}
	for _, imp := range r.importados {
		s.leads.PublicarImportacao(imp.origem, imp.rel)
	}
	s.publicarDisparo(d.ID, true)
	s.publicarAtivos()
	if n.Iniciar {
		s.acordar(d.ContaID)
	}
	criado, err := s.Obter(ctx, d.ID)
	if err != nil {
		return criado, err
	}
	criado.RelatorioImportacao = r.relatorio
	return criado, nil
}

// Editar altera um rascunho (PATCH com os campos de NovoDisparo, sem destinatários).
func (s *Servico) Editar(ctx context.Context, id string, corpo map[string]json.RawMessage) (dominio.Disparo, error) {
	d, err := armazenamento.ObterDisparo(ctx, s.banco.L(), id)
	if err != nil {
		return d, err
	}
	if d.Estado != dominio.DisparoRascunho {
		return d, erros.Transicao("Só é possível editar um disparo em rascunho.", d.Estado)
	}
	if _, ok := corpo["destinatarios"]; ok {
		return d, erros.Campo("destinatarios", "Os destinatários não podem ser alterados; crie outro disparo.")
	}
	atual := NovoDisparo{ContaID: d.ContaID, Nome: &d.Nome, Mensagem: &d.Mensagem, ArquivoID: d.ArquivoID, Ritmo: &d.Ritmo,
		InicioEm: d.InicioEm, Janela: d.Janela, FalhasSeguidasMax: &d.FalhasSeguidasMax, ValoresPadrao: d.ValoresPadrao, Origem: d.Origem}
	base, _ := json.Marshal(atual)
	var mapa map[string]json.RawMessage
	json.Unmarshal(base, &mapa)
	for k, v := range corpo {
		if k == "conta_id" || k == "origem" || k == "iniciar" {
			continue
		}
		mapa[k] = v
	}
	mesclado, _ := json.Marshal(mapa)
	var n NovoDisparo
	if err := json.Unmarshal(mesclado, &n); err != nil {
		return d, erros.Novo(erros.Validacao, "JSON inválido: "+err.Error())
	}
	mensagem, arquivoID, err := s.mensagemEArquivo(ctx, n)
	if err != nil {
		return d, err
	}
	if c := ValidarCampos(n, mensagem); len(c) > 0 {
		return d, erros.Campos(c)
	}
	d.Mensagem, d.ArquivoID, d.Ritmo, d.InicioEm, d.Janela = mensagem, arquivoID, *n.Ritmo, n.InicioEm, n.Janela
	if n.Nome != nil {
		d.Nome = strings.TrimSpace(*n.Nome)
	}
	if n.FalhasSeguidasMax != nil {
		d.FalhasSeguidasMax = *n.FalhasSeguidasMax
	}
	if n.ValoresPadrao != nil {
		d.ValoresPadrao = n.ValoresPadrao
	}
	if err := armazenamento.AtualizarRascunho(ctx, s.banco.E(), d); err != nil {
		return d, err
	}
	if err := s.recalcularVariaveis(ctx, d); err != nil {
		return d, err
	}
	s.publicarDisparo(id, true)
	return s.Obter(ctx, id)
}

// leadsDoDisparo carrega os leads na ordem dos destinatários.
func (s *Servico) leadsDoDisparo(ctx context.Context, id string) ([]dominio.Lead, []dominio.Destinatario, error) {
	var dests []dominio.Destinatario
	err := armazenamento.PercorrerDestinatarios(ctx, s.banco.L(), id, func(d dominio.Destinatario) error {
		dests = append(dests, d)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	idsL := make([]string, len(dests))
	for i, d := range dests {
		idsL[i] = d.LeadID
	}
	lista, err := armazenamento.LeadsPorIDs(ctx, s.banco.L(), idsL)
	return lista, dests, err
}

// recalcularVariaveis refaz os valores resolvidos dos destinatários ainda pendentes.
func (s *Servico) recalcularVariaveis(ctx context.Context, d dominio.Disparo) error {
	lista, dests, err := s.leadsDoDisparo(ctx, d.ID)
	if err != nil {
		return err
	}
	porLead := map[string]dominio.Lead{}
	for _, l := range lista {
		porLead[l.ID] = l
	}
	vars := variaveis.Extrair(d.Mensagem)
	return s.banco.Transacao(ctx, func(tx *sql.Tx) error {
		for _, dest := range dests {
			if dest.Estado != dominio.DestPendente {
				continue
			}
			vals, _ := variaveis.ValoresDoLead(vars, porLead[dest.LeadID], d.ValoresPadrao)
			if err := armazenamento.AtualizarVariaveisDest(ctx, tx, dest.ID, vals); err != nil {
				return err
			}
		}
		return nil
	})
}

// Excluir apaga um rascunho.
func (s *Servico) Excluir(ctx context.Context, id string) error {
	d, err := armazenamento.ObterDisparo(ctx, s.banco.L(), id)
	if err != nil {
		return err
	}
	ok, err := armazenamento.ExcluirDisparo(ctx, s.banco.E(), id)
	if err != nil {
		return err
	}
	if !ok {
		return erros.Transicao("Só é possível excluir um disparo em rascunho.", d.Estado)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Ações
// ---------------------------------------------------------------------------

// Iniciar põe um rascunho na fila da conta.
func (s *Servico) Iniciar(ctx context.Context, id string, valoresPadrao map[string]string) (dominio.Disparo, error) {
	d, err := armazenamento.ObterDisparo(ctx, s.banco.L(), id)
	if err != nil {
		return d, err
	}
	if _, err := Transicao(d.Estado, AcaoIniciar); err != nil {
		return d, err
	}
	if len(valoresPadrao) > 0 {
		if d.ValoresPadrao == nil {
			d.ValoresPadrao = map[string]string{}
		}
		for k, v := range valoresPadrao {
			d.ValoresPadrao[k] = v
		}
		if err := armazenamento.DefinirValoresPadrao(ctx, s.banco.E(), id, d.ValoresPadrao); err != nil {
			return d, err
		}
	}
	lista, _, err := s.leadsDoDisparo(ctx, id)
	if err != nil {
		return d, err
	}
	if f := variaveis.Faltando(d.Mensagem, lista, d.ValoresPadrao); len(f) > 0 {
		return d, erroFaltando(f)
	}
	if _, err := s.contas.Cliente(ctx, d.ContaID); err != nil {
		return d, err
	}
	if len(valoresPadrao) > 0 {
		if err := s.recalcularVariaveis(ctx, d); err != nil {
			return d, err
		}
	}
	if ok, err := s.mudar(ctx, d, AcaoIniciar, nil); err != nil {
		return d, err
	} else if !ok {
		return d, erros.Transicao("O disparo mudou de estado; tente de novo.", d.Estado)
	}
	s.acordar(d.ContaID)
	return s.Obter(ctx, id)
}

// Pausar pausa pelo usuário.
func (s *Servico) Pausar(ctx context.Context, id string) (dominio.Disparo, error) {
	return s.acaoSimples(ctx, id, AcaoPausar, dominio.PausaUsuario)
}

// Cancelar cancela.
func (s *Servico) Cancelar(ctx context.Context, id string) (dominio.Disparo, error) {
	return s.acaoSimples(ctx, id, AcaoCancelar, "")
}

func (s *Servico) acaoSimples(ctx context.Context, id, acao, motivo string) (dominio.Disparo, error) {
	for tentativa := 0; tentativa < 5; tentativa++ {
		d, err := armazenamento.ObterDisparo(ctx, s.banco.L(), id)
		if err != nil {
			return d, err
		}
		var m *string
		if motivo != "" {
			m = &motivo
		}
		ok, err := s.mudar(ctx, d, acao, m)
		if err != nil {
			return d, err
		}
		if ok {
			s.acordar(d.ContaID)
			return s.Obter(ctx, id)
		}
	}
	return dominio.Disparo{}, erros.Novo(erros.Interno, "O disparo está mudando de estado; tente de novo.")
}

// Retomar volta um pausado para a fila (fim da fila).
func (s *Servico) Retomar(ctx context.Context, id string) (dominio.Disparo, error) {
	d, err := armazenamento.ObterDisparo(ctx, s.banco.L(), id)
	if err != nil {
		return d, err
	}
	if _, err := Transicao(d.Estado, AcaoRetomar); err != nil {
		return d, err
	}
	if _, err := s.contas.Cliente(ctx, d.ContaID); err != nil {
		return d, err
	}
	return s.acaoSimples(ctx, id, AcaoRetomar, "")
}

// pausarDaConta pausa os disparos ativos de uma conta (desconectada/banida/app fechado).
func (s *Servico) pausarDaConta(contaID, motivo string) {
	ctx := context.Background()
	lista, err := armazenamento.DisparosDaConta(ctx, s.banco.L(), contaID, dominio.DisparoAgendado, dominio.DisparoEnviando, dominio.DisparoForaDaJanela)
	if err != nil {
		return
	}
	for _, d := range lista {
		m := motivo
		s.mudar(ctx, d, AcaoPausar, &m)
	}
}

// contaMudou reage aos estados da conta.
func (s *Servico) contaMudou(contaID, estado string) {
	switch estado {
	case dominio.ContaDesconectada:
		s.pausarDaConta(contaID, dominio.PausaContaDesconectada)
	case dominio.ContaBanida:
		s.pausarDaConta(contaID, dominio.PausaContaBanida)
	case contas.EstadoRemovida:
		ctx := context.Background()
		lista, _ := armazenamento.DisparosDaConta(ctx, s.banco.L(), contaID, dominio.DisparoRascunho, dominio.DisparoAgendado,
			dominio.DisparoEnviando, dominio.DisparoForaDaJanela, dominio.DisparoPausado)
		for _, d := range lista {
			s.mudar(ctx, d, AcaoCancelar, nil)
		}
		s.pararExecutor(contaID)
	case dominio.ContaConectada:
		s.acordar(contaID)
	}
}

// Energia: "retomar" recalcula as agendas (regra 6: nada de rajada após o sono).
func (s *Servico) Energia(evento string) {
	s.log.Info().Str("evento", evento).Msg("energia")
	if evento != "retomar" {
		return
	}
	armazenamento.LimparProximoEnvioAtivos(context.Background(), s.banco.E())
	s.acordarTodos()
}

// ---------------------------------------------------------------------------
// Ciclo de vida
// ---------------------------------------------------------------------------

// Preparar roda antes do "pronto": ativos → pausado e reconciliação de "enviando".
func (s *Servico) Preparar(ctx context.Context, religado bool) error {
	motivo := dominio.PausaAppFechado
	if religado {
		motivo = dominio.PausaMotorReiniciado
	}
	if _, err := s.banco.E().ExecContext(ctx, `UPDATE disparos SET estado = 'pausado', motivo_pausa = ?, proximo_envio_em = NULL
		WHERE estado IN ('enviando', 'fora_da_janela') OR (estado = 'agendado' AND iniciado_em IS NOT NULL)`, motivo); err != nil {
		return err
	}
	return s.reconciliar(ctx)
}

// reconciliar trata destinatários presos em "enviando": achado no histórico local → enviado;
// senão → falhou "Estado incerto após interrupção". Nunca reenvia (Constituição V).
func (s *Servico) reconciliar(ctx context.Context) error {
	presos, err := armazenamento.DestinatariosEnviando(ctx, s.banco.L())
	if err != nil {
		return err
	}
	agora := s.relogio.Agora()
	for _, d := range presos {
		disp, err := armazenamento.ObterDisparo(ctx, s.banco.L(), d.DisparoID)
		if err != nil {
			continue
		}
		jid := strings.TrimPrefix(d.Telefone, "+") + "@s.whatsapp.net"
		if d.JID != nil {
			jid = *d.JID
		}
		if waID, ok := s.chat.AchadaNoHistorico(ctx, disp.ContaID, disp.ID, jid); ok {
			em := agora
			if d.EnviandoEm != nil {
				em = *d.EnviandoEm
			}
			if err := armazenamento.MarcarEnviadoDest(ctx, s.banco.E(), d.ID, waID, jid, em); err != nil {
				return err
			}
			continue
		}
		if err := armazenamento.MarcarFalhouDest(ctx, s.banco.E(), d.ID, MotivoEstadoIncerto, agora); err != nil {
			return err
		}
	}
	return nil
}

// Encerrar para os executores e deixa os ativos como pausado (app_fechado).
func (s *Servico) Encerrar(ctx context.Context) {
	s.mu.Lock()
	s.encerrando = true
	for _, e := range s.execs {
		e.cancelar()
	}
	s.mu.Unlock()
	s.wg.Wait()
	s.banco.E().ExecContext(ctx, `UPDATE disparos SET estado = 'pausado', motivo_pausa = 'app_fechado', proximo_envio_em = NULL
		WHERE estado IN ('enviando', 'fora_da_janela') OR (estado = 'agendado' AND iniciado_em IS NOT NULL)`)
}

// ---------------------------------------------------------------------------
// Relatório
// ---------------------------------------------------------------------------

// colunasRelatorio são as colunas fixas do relatório (contracts/api-http.md).
var colunasRelatorio = []string{"telefone", "nome", "estado", "motivo_falha", "enviando_em", "enviado_em", "entregue_em", "lido_em", "respondeu_em", "falhou_em"}

// CabecalhoRelatorio devolve as colunas fixas + uma por variável, escrita como `{variavel}`. As
// chaves evitam colunas repetidas (ex.: a variável {nome} ao lado da coluna fixa "nome", o que o
// Excel mostra como duas colunas iguais) e deixam claro qual valor foi usado na mensagem.
func CabecalhoRelatorio(vars []string) []string {
	cab := append([]string{}, colunasRelatorio...)
	for _, v := range vars {
		cab = append(cab, "{"+v+"}")
	}
	return cab
}

// RelatorioCSV escreve o CSV (UTF-8 com BOM) com as colunas do contrato + uma por variável.
func (s *Servico) RelatorioCSV(ctx context.Context, id string, w io.Writer) (string, error) {
	d, err := armazenamento.ObterDisparo(ctx, s.banco.L(), id)
	if err != nil {
		return "", err
	}
	vars := variaveis.Extrair(d.Mensagem)
	if _, err := w.Write([]byte("\xef\xbb\xbf")); err != nil {
		return "", err
	}
	cw := csv.NewWriter(w)
	cw.Write(CabecalhoRelatorio(vars))
	data := func(t *time.Time) string {
		if t == nil {
			return ""
		}
		return t.Format(time.RFC3339)
	}
	txt := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	err = armazenamento.PercorrerDestinatarios(ctx, s.banco.L(), id, func(x dominio.Destinatario) error {
		linha := []string{x.Telefone, txt(x.Nome), x.Estado, txt(x.MotivoFalha), data(x.EnviandoEm), data(x.EnviadoEm),
			data(x.EntregueEm), data(x.LidoEm), data(x.RespondeuEm), data(x.FalhouEm)}
		for _, v := range vars {
			val := x.Variaveis[v]
			if val == "" {
				val = d.ValoresPadrao[v]
			}
			linha = append(linha, val)
		}
		return cw.Write(linha)
	})
	cw.Flush()
	if err == nil {
		err = cw.Error()
	}
	return d.Nome, err
}
