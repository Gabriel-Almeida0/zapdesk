package ponte

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/automacoes/acoes"
	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/automacoes/runner"
	"zapdesk/motor/internal/claude"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/funil"
	"zapdesk/motor/internal/leads"
	"zapdesk/motor/internal/telefone"
)

func normalizarTel(bruto string) string {
	if e, motivo := telefone.Normalizar(bruto, ""); motivo == "" {
		return e
	}
	return bruto
}

// ---------------------------------------------------------------------------
// Etiquetas
// ---------------------------------------------------------------------------

type etiquetaProt struct {
	ID   string `json:"id"`
	Nome string `json:"nome"`
	Cor  string `json:"cor"`
}

func (s *Sessao) etiquetaPorNomeOuID(ctx context.Context, v string) (dominio.Etiqueta, bool) {
	lista, _ := armazenamento.ListarEtiquetas(ctx, s.p.d.Banco.L())
	for _, e := range lista {
		if e.ID == v {
			return e, true
		}
	}
	for _, e := range lista {
		if modelo.NormalizarTexto(e.Nome) == modelo.NormalizarTexto(v) {
			return e, true
		}
	}
	return dominio.Etiqueta{}, false
}

func (s *Sessao) etiquetas(ctx context.Context, metodo string, params json.RawMessage) (json.RawMessage, *runner.ErroRPC) {
	var p struct {
		Etiqueta string     `json:"etiqueta"`
		Alvo     *alvoParam `json:"alvo"`
	}
	if e := decodificar(params, &p); e != nil {
		return nil, e
	}
	if metodo == "ctx.etiquetas.listar" {
		lista, err := armazenamento.ListarEtiquetas(ctx, s.p.d.Banco.L())
		if err != nil {
			return nil, deErro(err)
		}
		r := make([]etiquetaProt, 0, len(lista))
		for _, e := range lista {
			r = append(r, etiquetaProt{e.ID, e.Nome, e.Cor})
		}
		return ok(r)
	}
	c, e := s.resolverAlvo(ctx, p.Alvo)
	if e != nil {
		return nil, e
	}
	if metodo == "ctx.etiquetas.do_contato" {
		r := []etiquetaProt{}
		if c.Alvo.ContatoID != "" {
			if ct, err := armazenamento.ObterContato(ctx, s.p.d.Banco.L(), c.Alvo.ContatoID); err == nil {
				for _, e := range ct.Etiquetas {
					r = append(r, etiquetaProt{e.ID, e.Nome, e.Cor})
				}
			}
		}
		return ok(r)
	}
	et, achou := s.etiquetaPorNomeOuID(ctx, p.Etiqueta)
	if !achou {
		return nil, naoAchado(fmt.Sprintf("Etiqueta '%s' não encontrada.", p.Etiqueta))
	}
	tipo := modelo.AcaoAdicionarEtiqueta
	if metodo == "ctx.etiquetas.remover" {
		tipo = modelo.AcaoRemoverEtiqueta
	}
	return s.acao(ctx, c, modelo.Acao{Tipo: tipo, EtiquetaID: et.ID})
}

// acao executa uma ação comum e traduz o resultado (falhou → erro; bloqueada → 1004).
func (s *Sessao) acao(ctx context.Context, c *acoes.Contexto, a modelo.Acao) (json.RawMessage, *runner.ErroRPC) {
	r := s.p.d.Acoes.Executar(ctx, c, a)
	if s.exec != nil {
		s.exec.RegistrarAcao(ctx, r.Registro)
	}
	switch r.Registro.Resultado {
	case dominio.AcaoFalhou:
		msg := "A ação falhou."
		if r.Registro.Detalhe != nil {
			msg = *r.Registro.Detalhe
		}
		if strings.Contains(msg, "não encontrad") {
			return nil, naoAchado(msg)
		}
		return nil, validacao(msg)
	case dominio.AcaoBloqueada:
		msg := ""
		if r.Registro.Detalhe != nil {
			msg = *r.Registro.Detalhe
		}
		return nil, erroRPC(CodBloqueado, "bloqueado", msg, map[string]any{"motivo": r.Bloqueio})
	}
	return ok(nil)
}

// ---------------------------------------------------------------------------
// Funil
// ---------------------------------------------------------------------------

type funilProt struct {
	ID     string        `json:"id"`
	Nome   string        `json:"nome"`
	Etapas []etapaResumo `json:"etapas"`
}

type etapaResumo struct {
	ID    string `json:"id"`
	Nome  string `json:"nome"`
	Cor   string `json:"cor"`
	Ordem int    `json:"ordem"`
}

type posicaoProt struct {
	FunilID string `json:"funil_id"`
	Funil   string `json:"funil"`
	EtapaID string `json:"etapa_id"`
	Etapa   string `json:"etapa"`
	Desde   string `json:"desde"`
}

func (s *Sessao) funilPorNomeOuID(ctx context.Context, v string) (dominio.Funil, *runner.ErroRPC) {
	lista, err := armazenamento.ListarFunis(ctx, s.p.d.Banco.L())
	if err != nil {
		return dominio.Funil{}, deErro(err)
	}
	for _, f := range lista {
		if f.ID == v || modelo.NormalizarTexto(f.Nome) == modelo.NormalizarTexto(v) {
			return f, nil
		}
	}
	return dominio.Funil{}, naoAchado(fmt.Sprintf("Funil '%s' não encontrado.", v))
}

func (s *Sessao) funil(ctx context.Context, metodo string, params json.RawMessage) (json.RawMessage, *runner.ErroRPC) {
	var p struct {
		Funil string     `json:"funil"`
		Etapa string     `json:"etapa"`
		Alvo  *alvoParam `json:"alvo"`
	}
	if e := decodificar(params, &p); e != nil {
		return nil, e
	}
	if metodo == "ctx.funil.listar" {
		lista, err := armazenamento.ListarFunis(ctx, s.p.d.Banco.L())
		if err != nil {
			return nil, deErro(err)
		}
		r := make([]funilProt, 0, len(lista))
		for _, f := range lista {
			fp := funilProt{ID: f.ID, Nome: f.Nome, Etapas: []etapaResumo{}}
			for _, e := range f.Etapas {
				fp.Etapas = append(fp.Etapas, etapaResumo{e.ID, e.Nome, e.Cor, e.Ordem})
			}
			r = append(r, fp)
		}
		return ok(r)
	}
	f, e := s.funilPorNomeOuID(ctx, p.Funil)
	if e != nil {
		return nil, e
	}
	c, e := s.resolverAlvo(ctx, p.Alvo)
	if e != nil {
		return nil, e
	}
	posicao := func(leadID string) (json.RawMessage, *runner.ErroRPC) {
		card, achou, err := armazenamento.ObterCard(ctx, s.p.d.Banco.L(), leadID, f.ID)
		if err != nil {
			return nil, deErro(err)
		}
		if !achou {
			return json.RawMessage("null"), nil
		}
		nome := ""
		for _, et := range f.Etapas {
			if et.ID == card.EtapaID {
				nome = et.Nome
			}
		}
		return ok(posicaoProt{FunilID: f.ID, Funil: f.Nome, EtapaID: card.EtapaID, Etapa: nome, Desde: card.Desde.Format(time.RFC3339)})
	}
	switch metodo {
	case "ctx.funil.posicao":
		if c.Alvo.LeadID == "" {
			return json.RawMessage("null"), nil
		}
		return posicao(c.Alvo.LeadID)
	case "ctx.funil.remover":
		if c.Alvo.LeadID == "" {
			return ok(nil)
		}
		return s.acao(ctx, c, modelo.Acao{Tipo: modelo.AcaoRemoverDoFunil, FunilID: f.ID})
	}
	var etapa *dominio.Etapa
	for i := range f.Etapas {
		if f.Etapas[i].ID == p.Etapa || modelo.NormalizarTexto(f.Etapas[i].Nome) == modelo.NormalizarTexto(p.Etapa) {
			etapa = &f.Etapas[i]
		}
	}
	if etapa == nil {
		return nil, naoAchado(fmt.Sprintf("Etapa '%s' não encontrada no funil '%s'.", p.Etapa, f.Nome))
	}
	if s.c.Simulacao {
		s.registrar(ctx, modelo.AcaoMoverEtapa, dominio.AcaoSimulada, f.Nome+" › "+etapa.Nome, "Moveria o lead para "+f.Nome+" › "+etapa.Nome+".")
		return ok(posicaoProt{FunilID: f.ID, Funil: f.Nome, EtapaID: etapa.ID, Etapa: etapa.Nome, Desde: s.p.d.Relogio.Agora().Format(time.RFC3339)})
	}
	if c.Alvo.LeadID == "" {
		if c.Alvo.ContatoID == "" {
			return nil, validacao("Esta execução não tem contato.")
		}
		l, _, err := s.p.d.Leads.GarantirLeadDoContato(ctx, c.Alvo.ContatoID)
		if err != nil {
			return nil, deErro(err)
		}
		c.Alvo.LeadID = l.ID
	}
	o := funil.Origem{Tipo: dominio.OrigemAutomacao, AutomacaoID: s.aut.ID}
	if s.exec != nil {
		o.ExecucaoID = s.exec.ID()
	}
	if _, _, err := s.p.d.Funil.Mover(ctx, f.ID, funil.AlvoCard{LeadID: c.Alvo.LeadID}, etapa.ID, o); err != nil {
		return nil, deErro(err)
	}
	s.registrar(ctx, modelo.AcaoMoverEtapa, dominio.AcaoOK, f.Nome+" › "+etapa.Nome, "")
	return posicao(c.Alvo.LeadID)
}

// ---------------------------------------------------------------------------
// Leads
// ---------------------------------------------------------------------------

type leadProt struct {
	ID          string            `json:"id"`
	Telefone    string            `json:"telefone"`
	Nome        *string           `json:"nome"`
	Campos      map[string]string `json:"campos"`
	Origem      string            `json:"origem"`
	ImportadoEm string            `json:"importado_em"`
}

func leadDe(l dominio.Lead) leadProt {
	c := l.Campos
	if c == nil {
		c = map[string]string{}
	}
	return leadProt{ID: l.ID, Telefone: l.Telefone, Nome: l.Nome, Campos: c, Origem: l.Origem, ImportadoEm: l.ImportadoEm.Format(time.RFC3339)}
}

func (s *Sessao) leads(ctx context.Context, metodo string, params json.RawMessage) (json.RawMessage, *runner.ErroRPC) {
	var p struct {
		ID       string             `json:"id"`
		Telefone string             `json:"telefone"`
		Nome     json.RawMessage    `json:"nome"`
		Campos   map[string]*string `json:"campos"`
		Alvo     *alvoParam         `json:"alvo"`
	}
	if e := decodificar(params, &p); e != nil {
		return nil, e
	}
	l := s.p.d.Banco.L()
	switch metodo {
	case "ctx.leads.atual":
		c := *s.c
		s.p.d.Acoes.CompletarAlvo(ctx, &c)
		if c.Alvo.LeadID == "" {
			return json.RawMessage("null"), nil
		}
		ld, err := armazenamento.ObterLead(ctx, l, c.Alvo.LeadID)
		if err != nil {
			return json.RawMessage("null"), nil
		}
		return ok(leadDe(ld))
	case "ctx.leads.obter":
		ld, err := armazenamento.ObterLead(ctx, l, p.ID)
		if err != nil {
			return json.RawMessage("null"), nil
		}
		return ok(leadDe(ld))
	case "ctx.leads.buscar_telefone":
		ld, err := armazenamento.LeadPorTelefone(ctx, l, normalizarTel(p.Telefone))
		if err != nil {
			return json.RawMessage("null"), nil
		}
		return ok(leadDe(ld))
	}
	c, e := s.resolverAlvo(ctx, p.Alvo)
	if e != nil {
		return nil, e
	}
	alt := leads.AlteracaoLead{Campos: p.Campos}
	if len(p.Nome) > 0 {
		alt.NomeDefinido = true
		if string(p.Nome) != "null" {
			var n string
			if err := json.Unmarshal(p.Nome, &n); err != nil {
				return nil, validacao("nome deve ser texto ou null.")
			}
			alt.Nome = &n
		}
	}
	if s.c.Simulacao {
		s.registrar(ctx, "atualizar_lead", dominio.AcaoSimulada, "", "Atualizaria o lead.")
		ld := leadProt{ID: c.Alvo.LeadID, Campos: map[string]string{}, Origem: dominio.OrigemContatos}
		if c.Alvo.LeadID != "" {
			if x, err := armazenamento.ObterLead(ctx, l, c.Alvo.LeadID); err == nil {
				ld = leadDe(x)
			}
		}
		for k, v := range p.Campos {
			if v == nil {
				delete(ld.Campos, k)
			} else {
				ld.Campos[k] = *v
			}
		}
		if alt.NomeDefinido {
			ld.Nome = alt.Nome
		}
		return ok(ld)
	}
	if c.Alvo.LeadID == "" {
		if c.Alvo.ContatoID == "" {
			return nil, validacao("Esta execução não tem contato.")
		}
		ld, _, err := s.p.d.Leads.GarantirLeadDoContato(ctx, c.Alvo.ContatoID)
		if err != nil {
			return nil, deErro(err)
		}
		c.Alvo.LeadID = ld.ID
	}
	ld, err := s.p.d.Leads.Atualizar(ctx, c.Alvo.LeadID, alt)
	if err != nil {
		return nil, deErro(err)
	}
	s.registrar(ctx, "atualizar_lead", dominio.AcaoOK, ld.Telefone, "")
	return ok(leadDe(ld))
}

// ---------------------------------------------------------------------------
// Memória
// ---------------------------------------------------------------------------

func (s *Sessao) escopo(ctx context.Context, e string) (string, *runner.ErroRPC) {
	switch e {
	case "", "global":
		return "global", nil
	case "contato":
		c := *s.c
		s.p.d.Acoes.CompletarAlvo(ctx, &c)
		if c.Alvo.ContatoID == "" {
			if s.c.ConversaFicticia {
				return "contato:contato-ficticio", nil
			}
			return "", validacao("Esta execução não tem contato para a memória de escopo 'contato'.")
		}
		return "contato:" + c.Alvo.ContatoID, nil
	}
	return "", validacao("Escopo inválido (use global ou contato).")
}

func (s *Sessao) memoria(ctx context.Context, metodo string, params json.RawMessage) (json.RawMessage, *runner.ErroRPC) {
	var p struct {
		Chave   string          `json:"chave"`
		Escopo  string          `json:"escopo"`
		Valor   json.RawMessage `json:"valor"`
		Prefixo string          `json:"prefixo"`
	}
	if e := decodificar(params, &p); e != nil {
		return nil, e
	}
	esc, e := s.escopo(ctx, p.Escopo)
	if e != nil {
		return nil, e
	}
	if metodo != "ctx.memoria.listar" {
		if n := len([]rune(p.Chave)); n < 1 || n > 200 {
			return nil, validacao("A chave deve ter de 1 a 200 caracteres.")
		}
	}
	chaveSim := esc + "\x00" + p.Chave
	b := s.p.d.Banco
	switch metodo {
	case "ctx.memoria.obter":
		if s.c.Simulacao {
			s.mu.Lock()
			v, tem := s.memSim[chaveSim]
			s.mu.Unlock()
			if tem {
				return ok(map[string]json.RawMessage{"valor": nulo(v)})
			}
		}
		v, _, err := armazenamento.LerMemoria(ctx, b.L(), s.aut.ID, esc, p.Chave)
		if err != nil {
			return nil, deErro(err)
		}
		return ok(map[string]json.RawMessage{"valor": nulo(v)})
	case "ctx.memoria.definir":
		if len(p.Valor) == 0 {
			p.Valor = json.RawMessage("null")
		}
		if len(p.Valor) > MaxValorMemoria {
			return nil, erroRPC(CodLimite, "limite", "Valor maior que 64 KB.", nil)
		}
		if s.c.Simulacao {
			s.mu.Lock()
			s.memSim[chaveSim] = p.Valor
			s.mu.Unlock()
			return ok(nil)
		}
		if err := armazenamento.GravarMemoria(ctx, b.E(), s.aut.ID, esc, p.Chave, p.Valor, s.p.d.Relogio.Agora()); err != nil {
			if strings.Contains(err.Error(), "10.000") {
				return nil, erroRPC(CodLimite, "limite", "Limite de 10.000 chaves de memória atingido.", nil)
			}
			return nil, deErro(err)
		}
		return ok(nil)
	case "ctx.memoria.remover":
		if s.c.Simulacao {
			s.mu.Lock()
			v, tem := s.memSim[chaveSim]
			s.memSim[chaveSim] = nil
			s.mu.Unlock()
			if tem {
				return ok(map[string]bool{"removida": v != nil})
			}
			_, existia, _ := armazenamento.LerMemoria(ctx, b.L(), s.aut.ID, esc, p.Chave)
			return ok(map[string]bool{"removida": existia})
		}
		r, err := armazenamento.RemoverMemoria(ctx, b.E(), s.aut.ID, esc, p.Chave)
		if err != nil {
			return nil, deErro(err)
		}
		return ok(map[string]bool{"removida": r})
	}
	lista, err := armazenamento.ListarMemoria(ctx, b.L(), s.aut.ID, esc, p.Prefixo)
	if err != nil {
		return nil, deErro(err)
	}
	if s.c.Simulacao {
		m := map[string]json.RawMessage{}
		for _, it := range lista {
			m[it.Chave] = it.Valor
		}
		s.mu.Lock()
		for k, v := range s.memSim {
			partes := strings.SplitN(k, "\x00", 2)
			if partes[0] != esc || !strings.HasPrefix(partes[1], p.Prefixo) {
				continue
			}
			if v == nil {
				delete(m, partes[1])
			} else {
				m[partes[1]] = v
			}
		}
		s.mu.Unlock()
		lista = lista[:0]
		for k, v := range m {
			lista = append(lista, armazenamento.ItemMemoria{Chave: k, Valor: v})
		}
		sort.Slice(lista, func(i, j int) bool { return lista[i].Chave < lista[j].Chave })
	}
	if lista == nil {
		lista = []armazenamento.ItemMemoria{}
	}
	return ok(lista)
}

// ---------------------------------------------------------------------------
// IA
// ---------------------------------------------------------------------------

type tokensProt struct {
	Entrada int `json:"entrada"`
	Saida   int `json:"saida"`
}

func (s *Sessao) cliente() claude.Cliente {
	if s.iaSimulada && s.p.d.IASimulada != nil {
		return s.p.d.IASimulada
	}
	return s.p.d.Claude
}

func (s *Sessao) modeloDe(ctx context.Context, pedido string) string {
	switch {
	case pedido != "":
		return pedido
	case s.modelo != "":
		return s.modelo
	case s.p.d.ModeloPadrao != nil:
		return s.p.d.ModeloPadrao(ctx)
	}
	return claude.ModeloPadrao
}

func (s *Sessao) somar(modelo string, u claude.Uso, requestID string) {
	if s.exec == nil {
		return
	}
	s.exec.SomarTokens(modelo, u.Entrada, u.Saida)
	msg := fmt.Sprintf("IA %s: %d tokens de entrada, %d de saída", modelo, u.Entrada, u.Saida)
	if requestID != "" {
		msg += " (request-id " + requestID + ")"
	}
	s.exec.Logar("debug", msg)
}

func (s *Sessao) ia(ctx context.Context, metodo string, params json.RawMessage) (json.RawMessage, *runner.ErroRPC) {
	var p struct {
		Prompt     string             `json:"prompt"`
		Mensagens  []claude.Mensagem  `json:"mensagens"`
		Sistema    string             `json:"sistema"`
		Modelo     string             `json:"modelo"`
		MaxTokens  int                `json:"max_tokens"`
		Texto      string             `json:"texto"`
		Categorias map[string]*string `json:"categorias"`
		Instrucoes string             `json:"instrucoes"`
		Esquema    map[string]any     `json:"esquema"`
	}
	if e := decodificar(params, &p); e != nil {
		return nil, e
	}
	if p.MaxTokens != 0 && (p.MaxTokens < 1 || p.MaxTokens > 8192) {
		return nil, validacao("maxTokens deve ficar entre 1 e 8192.")
	}
	modelo := s.modeloDe(ctx, p.Modelo)
	if !strings.HasPrefix(modelo, "claude-") {
		return nil, validacao("O modelo deve começar com \"claude-\".")
	}
	cli := s.cliente()
	switch metodo {
	case "ctx.ia.gerar":
		if p.Prompt == "" && len(p.Mensagens) == 0 {
			return nil, validacao("Informe prompt ou mensagens.")
		}
		r, err := cli.Gerar(ctx, claude.PedidoGerar{Modelo: modelo, Sistema: p.Sistema, Prompt: p.Prompt, Mensagens: p.Mensagens, MaxTokens: p.MaxTokens})
		if err != nil {
			return nil, deErro(err)
		}
		s.somar(r.Modelo, r.Uso, r.RequestID)
		return ok(map[string]any{"texto": r.Texto, "modelo": r.Modelo, "motivo_parada": r.MotivoParada, "tokens": tokensProt{r.Uso.Entrada, r.Uso.Saida}})
	case "ctx.ia.classificar":
		if len(p.Categorias) == 0 {
			return nil, validacao("Informe as categorias.")
		}
		cats := make([]string, 0, len(p.Categorias))
		desc := map[string]string{}
		for k, v := range p.Categorias {
			cats = append(cats, k)
			if v != nil {
				desc[k] = *v
			}
		}
		sort.Strings(cats)
		r, err := cli.Classificar(ctx, claude.PedidoClassificar{Modelo: modelo, Sistema: p.Sistema, Texto: p.Texto, Instrucoes: p.Instrucoes,
			Categorias: cats, Descricoes: desc, MaxTokens: p.MaxTokens})
		if err != nil {
			return nil, deErro(err)
		}
		s.somar(r.Modelo, r.Uso, r.RequestID)
		return ok(map[string]any{"categoria": r.Categoria, "tokens": tokensProt{r.Uso.Entrada, r.Uso.Saida}})
	}
	if p.Esquema == nil {
		return nil, validacao("Informe o esquema.")
	}
	r, err := cli.Extrair(ctx, claude.PedidoExtrair{Modelo: modelo, Sistema: p.Sistema, Texto: p.Texto, Instrucoes: p.Instrucoes, Esquema: p.Esquema, MaxTokens: p.MaxTokens})
	if err != nil {
		return nil, deErro(err)
	}
	s.somar(r.Modelo, r.Uso, r.RequestID)
	return ok(map[string]any{"dados": nulo(r.Dados), "tokens": tokensProt{r.Uso.Entrada, r.Uso.Saida}})
}
