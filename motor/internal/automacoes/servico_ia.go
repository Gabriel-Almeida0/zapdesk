package automacoes

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/automacoes/acoes"
	"zapdesk/motor/internal/automacoes/compilador"
	"zapdesk/motor/internal/automacoes/execucoes"
	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/automacoes/ponte"
	"zapdesk/motor/internal/automacoes/projetos"
	"zapdesk/motor/internal/automacoes/runner"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/ids"
)

// IA é o serviço das automações de IA (US3): projetos, compilação, execução no runner e testes.
// Implementa ServicoIA e Testador (fachada), despacho.Executor e acoes.IA.
type IA struct {
	s        *Servico
	projetos *projetos.Projetos
	pool     *runner.Pool
	ponte    *ponte.Ponte

	mu     sync.Mutex
	travas map[string]*sync.Mutex
}

// NovaIA cria o serviço de IA e o liga à fachada.
func NovaIA(s *Servico, p *projetos.Projetos, pool *runner.Pool, pt *ponte.Ponte) *IA {
	ia := &IA{s: s, projetos: p, pool: pool, ponte: pt, travas: map[string]*sync.Mutex{}}
	s.DefinirIA(ia, ia)
	s.AdicionarGanchos(Ganchos{
		AoDesativar: []func(context.Context, dominio.Automacao){func(_ context.Context, a dominio.Automacao) {
			if a.Tipo == modelo.TipoIA {
				go pool.EncerrarAutomacao(a.ID)
			}
		}},
		AoExcluir: []func(context.Context, dominio.Automacao){func(_ context.Context, a dominio.Automacao) {
			if a.Tipo == modelo.TipoIA {
				pool.EncerrarAutomacao(a.ID)
				p.ApagarProjeto(a.ID)
			}
		}},
	})
	return ia
}

func (ia *IA) trava(id string) *sync.Mutex {
	ia.mu.Lock()
	defer ia.mu.Unlock()
	t, ok := ia.travas[id]
	if !ok {
		t = &sync.Mutex{}
		ia.travas[id] = t
	}
	return t
}

// RunnerDisponivel indica se há runner configurado.
func (ia *IA) RunnerDisponivel() bool { return ia.pool.Disponivel() }

// ProcessosVivos do pool.
func (ia *IA) ProcessosVivos() int { return ia.pool.Vivos() }

// Projetos expõe os projetos (rotas).
func (ia *IA) Projetos() *projetos.Projetos { return ia.projetos }

// PreencherIA completa Automacao.IA.
func (ia *IA) PreencherIA(ctx context.Context, a *dominio.Automacao) {
	a.IA = iaBasica(*a, ia.projetos.Pasta(a.ID))
	a.IA.RodandoVersaoAnterior = len(a.IA.ErrosCompilacao) > 0 && a.HashCompilado != nil
}

// ---------------------------------------------------------------------------
// Criação e compilação
// ---------------------------------------------------------------------------

// CriarPeloModelo cria a automação (inativa), a pasta com os arquivos do modelo e compila.
func (ia *IA) CriarPeloModelo(ctx context.Context, nome, idModelo string, descricao *string) (dominio.Automacao, error) {
	nome = strings.TrimSpace(nome)
	if n := utf8.RuneCountInString(nome); n < 1 || n > 80 {
		return dominio.Automacao{}, erros.Campo("nome", "O nome deve ter de 1 a 80 caracteres.")
	}
	if descricao != nil && utf8.RuneCountInString(*descricao) > 500 {
		return dominio.Automacao{}, erros.Campo("descricao", "A descrição pode ter até 500 caracteres.")
	}
	valido := false
	for _, m := range projetos.Modelos() {
		if m.ID == idModelo {
			valido = true
		}
	}
	if !valido {
		return dominio.Automacao{}, erros.Campo("modelo", "Modelo desconhecido (use responder_historico, classificar_funil, extrair_dados ou em_branco).")
	}
	agora := ia.s.d.Relogio.Agora()
	a := dominio.Automacao{ID: ids.NovoEm(agora), Tipo: modelo.TipoIA, Nome: nome, Descricao: descricao, Prioridade: 100, Versao: 1,
		Gatilhos: json.RawMessage("[]"), Permissoes: []string{}, Segredos: []string{}, CriadaEm: agora, AtualizadaEm: agora}
	if err := ia.projetos.Criar(a.ID, idModelo, nome, descricao); err != nil {
		return a, err
	}
	if err := armazenamento.InserirAutomacao(ctx, ia.s.d.Banco.E(), a); err != nil {
		ia.projetos.ApagarProjeto(a.ID)
		return a, err
	}
	ia.Compilar(ctx, a.ID)
	return ia.s.Obter(ctx, a.ID)
}

// resolvedor traduz nomes do manifesto em ids (sem diferenciar maiúsculas/acentos).
type resolvedor struct{ b *armazenamento.Banco }

func (r resolvedor) Etiqueta(ctx context.Context, nome string) (string, bool) {
	l, _ := armazenamento.ListarEtiquetas(ctx, r.b.L())
	for _, e := range l {
		if modelo.NormalizarTexto(e.Nome) == modelo.NormalizarTexto(nome) {
			return e.ID, true
		}
	}
	return "", false
}

func (r resolvedor) Funil(ctx context.Context, nome string) (string, bool) {
	l, _ := armazenamento.ListarFunis(ctx, r.b.L())
	for _, f := range l {
		if modelo.NormalizarTexto(f.Nome) == modelo.NormalizarTexto(nome) {
			return f.ID, true
		}
	}
	return "", false
}

func (r resolvedor) Etapa(ctx context.Context, funilID, nome string) (string, bool) {
	l, _ := armazenamento.ListarEtapas(ctx, r.b.L(), funilID)
	for _, e := range l {
		if modelo.NormalizarTexto(e.Nome) == modelo.NormalizarTexto(nome) {
			return e.ID, true
		}
	}
	return "", false
}

// Compilar compila o projeto, atualiza a cópia do manifesto e Automacao.ia e publica
// automacao.atualizada.
func (ia *IA) Compilar(ctx context.Context, id string) (compilador.Resultado, error) {
	t := ia.trava(id)
	t.Lock()
	defer t.Unlock()
	return ia.compilarSemTrava(ctx, id)
}

func (ia *IA) compilarSemTrava(ctx context.Context, id string) (compilador.Resultado, error) {
	a, err := armazenamento.ObterAutomacao(ctx, ia.s.d.Banco.L(), id)
	if err != nil {
		return compilador.Resultado{}, err
	}
	if a.Tipo != modelo.TipoIA {
		return compilador.Resultado{}, erros.Campo("tipo", "Só automações de IA são compiladas.")
	}
	ia.projetos.AtualizarGerados(id)
	res := compilador.Compilar(ctx, ia.projetos.Pasta(id), ia.projetos.PastaCompiladas(id), resolvedor{ia.s.d.Banco})
	agora := ia.s.d.Relogio.Agora()
	gatAntes := string(a.Gatilhos)
	if res.OK && res.Manifesto != nil {
		m := res.Manifesto
		novoHash := res.Hash != nil && (a.HashCompilado == nil || *a.HashCompilado != *res.Hash)
		a.Nome, a.Descricao = m.Nome, m.Descricao
		gs := m.Gatilhos
		if gs == nil {
			gs = []modelo.Gatilho{}
		}
		a.Gatilhos, _ = json.Marshal(gs)
		a.Permissoes = append([]string{}, m.Permissoes...)
		a.Segredos = append([]string{}, m.Segredos...)
		if m.Contas != nil && m.Contas.Lista != nil {
			a.Contas = m.Contas.Lista
		} else {
			a.Contas = nil
		}
		a.IncluirGrupos, a.Prioridade, a.ContaEnvioID = m.IncluirGrupos, m.PrioridadeOuPadrao(), m.ContaEnvio
		a.Limites = dominio.Limites{AntiLoop: m.Limites.AntiLoop, TempoS: m.Limites.TempoS, MemoriaMB: m.Limites.MemoriaMB}
		a.HashCompilado = res.Hash
		a.ErrosCompilacao = []dominio.ErroCompilacao{}
		if novoHash && a.CompiladoEm != nil {
			a.Versao++
		}
		compilador.LimparBundles(ia.projetos.PastaCompiladas(id), res.Bundle)
	} else {
		a.ErrosCompilacao = res.Erros
		if a.ErrosCompilacao == nil {
			a.ErrosCompilacao = []dominio.ErroCompilacao{}
		}
	}
	hf := res.HashFontes
	a.HashFontes = &hf
	a.CompiladoEm = &agora
	a.AtualizadaEm = agora
	if err := armazenamento.GravarAutomacao(ctx, ia.s.d.Banco.E(), a); err != nil {
		return res, err
	}
	if a.Ativa && gatAntes != string(a.Gatilhos) {
		ia.s.reprogramarAgendamentos(ctx, a)
	}
	ia.s.PublicarAtualizada(ctx, id)
	return res, nil
}

func erroCompilacao(res compilador.Resultado) error {
	msg := "O código da automação tem erros de compilação."
	if len(res.Erros) == 1 {
		msg = res.Erros[0].Mensagem
	}
	return erros.ComDetalhes(erros.CompilacaoFalhou, msg, map[string]any{"erros": res.Erros})
}

// Ativavel exige a compilação ok das fontes atuais.
func (ia *IA) Ativavel(ctx context.Context, a dominio.Automacao) error {
	res, err := ia.Compilar(ctx, a.ID)
	if err != nil {
		return err
	}
	if !res.OK {
		return erroCompilacao(res)
	}
	return nil
}

// preparar devolve a automação com um bundle executável. Fontes alteradas → recompila; se a nova
// compilação falhar, usa a anterior (execução real) ou falha (teste).
func (ia *IA) preparar(ctx context.Context, id string, teste bool) (dominio.Automacao, string, error) {
	t := ia.trava(id)
	t.Lock()
	defer t.Unlock()
	a, err := armazenamento.ObterAutomacao(ctx, ia.s.d.Banco.L(), id)
	if err != nil {
		return a, "", err
	}
	hf, errH := ia.projetos.HashFontes(id)
	if errH == nil && (a.HashFontes == nil || *a.HashFontes != hf || (teste && len(a.ErrosCompilacao) > 0)) {
		res, err := ia.compilarSemTrava(ctx, id)
		if err != nil {
			return a, "", err
		}
		if !res.OK && teste {
			return a, "", erroCompilacao(res)
		}
		a, _ = armazenamento.ObterAutomacao(ctx, ia.s.d.Banco.L(), id)
	}
	if teste && len(a.ErrosCompilacao) > 0 {
		return a, "", erroCompilacao(compilador.Resultado{Erros: a.ErrosCompilacao})
	}
	if a.HashCompilado == nil {
		return a, "", erros.ComDetalhes(erros.CompilacaoFalhou, "A automação ainda não compilou sem erros.", map[string]any{"erros": a.ErrosCompilacao})
	}
	bundle := filepath.Join(ia.projetos.PastaCompiladas(id), *a.HashCompilado+".mjs")
	if _, err := os.Stat(bundle); err != nil {
		res, err := ia.compilarSemTrava(ctx, id)
		if err != nil || !res.OK {
			return a, "", erros.ComDetalhes(erros.CompilacaoFalhou, "O código compilado não foi encontrado; corrija os erros e compile de novo.", map[string]any{"erros": res.Erros})
		}
		a, _ = armazenamento.ObterAutomacao(ctx, ia.s.d.Banco.L(), id)
		bundle = res.Bundle
	}
	return a, bundle, nil
}

// ---------------------------------------------------------------------------
// Execução
// ---------------------------------------------------------------------------

func (ia *IA) modeloManifesto(id string) string {
	c, err := ia.projetos.Ler(id, projetos.ArquivoManif)
	if err != nil {
		return ""
	}
	m, errs := modelo.DecodificarManifesto([]byte(c.Conteudo))
	if errs != nil || m.IA.Modelo == nil {
		return ""
	}
	return *m.IA.Modelo
}

func (ia *IA) limites(a dominio.Automacao) (time.Duration, int) {
	cfg := ia.s.d.Config.Configuracao()
	tempo, mem := cfg.TempoIAS, cfg.MemoriaIAMB
	if a.Limites.TempoS != nil {
		tempo = *a.Limites.TempoS
	}
	if a.Limites.MemoriaMB != nil {
		mem = *a.Limites.MemoriaMB
	}
	return time.Duration(tempo) * time.Second, mem
}

// handlerDe escolhe o handler pelo gatilho da execução.
func handlerDe(g dominio.GatilhoExecucao) string {
	if g.Tipo == modelo.GatilhoManual {
		return modelo.HandlerExecutar
	}
	if h := modelo.HandlerDoGatilho(g.Tipo); h != "" {
		return h
	}
	return modelo.HandlerExecutar
}

// Executar roda uma execução criada pelo despachante (gatilho ou manual).
func (ia *IA) Executar(ctx context.Context, e *execucoes.Exec, a dominio.Automacao, c *acoes.Contexto) error {
	if err := e.Iniciar(ctx); err != nil {
		return err
	}
	d := e.Detalhe()
	var entrada json.RawMessage
	if v, ok := d.Gatilho.Dados["entrada"]; ok {
		entrada, _ = json.Marshal(v)
	}
	_, err := ia.rodar(ctx, e, a.ID, c, handlerDe(d.Gatilho), entrada, false, false)
	return err
}

// ExecutarPorAcao roda aoExecutar como execução filha (ação executar_ia, nó ia) e devolve o retorno.
func (ia *IA) ExecutarPorAcao(ctx context.Context, automacaoID string, entrada map[string]any, c *acoes.Contexto) (json.RawMessage, error) {
	a, err := armazenamento.ObterAutomacao(ctx, ia.s.d.Banco.L(), automacaoID)
	if err != nil || a.Tipo != modelo.TipoIA {
		return nil, erros.Novo(erros.NaoEncontrado, "Automação de IA não encontrada.")
	}
	if !ia.pool.Disponivel() {
		return nil, erros.Novo(erros.Validacao, "As automações de IA não podem ser executadas nesta instalação.")
	}
	origem := c.Origem
	if origem != "chatbot" {
		origem = "fluxo"
	}
	pai := ""
	if c.Exec != nil {
		pai = c.Exec.ID()
	}
	bruto, _ := json.Marshal(entrada)
	e, err := ia.s.d.Registro.Criar(ctx, execucoes.Nova{Automacao: a, Origem: origem, OrigemExecucaoID: pai, Cadeia: c.Cadeia,
		Simulacao: c.Simulacao, Alvo: execucoes.Alvo{ContaID: c.Alvo.ContaID, ConversaID: c.Alvo.ConversaID, ContatoID: c.Alvo.ContatoID, LeadID: c.Alvo.LeadID},
		Gatilho: dominio.GatilhoExecucao{Tipo: modelo.GatilhoManual, Dados: map[string]any{"pedido_por": origem, "entrada": json.RawMessage(bruto)}}})
	if err != nil {
		return nil, err
	}
	e.Iniciar(ctx)
	filho := *c
	filho.Exec, filho.Automacao = e, a
	filho.Cadeia = append(append([]string{}, c.Cadeia...), a.ID)
	ret, err := ia.rodar(ctx, e, a.ID, &filho, modelo.HandlerExecutar, bruto, c.Simulacao, c.IASimulada)
	if err != nil {
		return nil, err
	}
	d := e.Detalhe()
	if d.Estado == dominio.ExecErro {
		msg := "A automação de IA falhou."
		if d.Erro != nil {
			msg = *d.Erro
		}
		return nil, erros.Novo(erros.Validacao, msg)
	}
	return ret, nil
}

// rodar prepara o bundle, monta info/conversa/argumento e executa no pool; finaliza a execução.
func (ia *IA) rodar(ctx context.Context, e *execucoes.Exec, id string, c *acoes.Contexto, handler string, entrada json.RawMessage,
	simulacao, iaSimulada bool) (json.RawMessage, error) {
	fim := func(f execucoes.Fim) (json.RawMessage, error) {
		if simulacao && f.Estado == dominio.ExecOK {
			f.Estado = dominio.ExecSimulacao
		}
		return f.Retorno, e.Finalizar(context.WithoutCancel(ctx), f)
	}
	if !ia.pool.Disponivel() {
		return fim(execucoes.Fim{Estado: dominio.ExecErro, Erro: "As automações de IA não podem ser executadas nesta instalação (runner indisponível)."})
	}
	a, bundle, err := ia.preparar(ctx, id, simulacao && c.Origem == "teste")
	if err != nil {
		msg := err.Error()
		if ee, ok := erros.Como(err); ok {
			msg = ee.Mensagem
		}
		return fim(execucoes.Fim{Estado: dominio.ExecErro, Erro: msg})
	}
	if len(a.ErrosCompilacao) > 0 {
		e.Logar("aviso", "O código atual tem erros de compilação; rodando a versão anterior.")
	}
	c.Automacao = a
	prazo, mem := ia.limites(a)
	sessao := ia.ponte.Sessao(ponte.Opcoes{Exec: e, Automacao: a, Permissoes: a.Permissoes, Contexto: c, IASimulada: iaSimulada,
		ModeloManifesto: ia.modeloManifesto(id)})
	defer sessao.Encerrar()
	d := e.Detalhe()
	info, _ := json.Marshal(map[string]any{"id": d.ID, "automacaoId": a.ID, "automacaoNome": a.Nome, "gatilho": d.Gatilho,
		"origem": d.Origem, "simulacao": d.Simulacao, "iniciadaEm": d.IniciadaEm.Format(time.RFC3339)})
	conversa := ia.conversaProt(ctx, c)
	arg, err := ia.argumento(ctx, handler, d, c, entrada)
	if err != nil {
		return fim(execucoes.Fim{Estado: dominio.ExecErro, Erro: err.Error()})
	}
	ret, err := ia.pool.Executar(ctx, runner.Automacao{ID: a.ID, Nome: a.Nome, Versao: a.Versao, Hash: *a.HashCompilado, Bundle: bundle,
		Permissoes: a.Permissoes, Segredos: ia.s.d.Cofre.Declarados(a.Segredos), NomesSegredos: a.Segredos, MemoriaMB: mem},
		runner.Pedido{ExecucaoID: e.ID(), Handler: handler, Simulacao: simulacao, Prazo: prazo, Info: info, Conversa: conversa, Argumento: arg}, sessao)
	if err != nil {
		return fim(ia.fimDeErro(e, err, ctx))
	}
	return fim(execucoes.Fim{Estado: dominio.ExecOK, Retorno: ret})
}

func (ia *IA) fimDeErro(e *execucoes.Exec, err error, ctx context.Context) execucoes.Fim {
	var eu *runner.ErroUsuario
	var er *runner.ErroRunner
	switch {
	case errors.As(err, &eu):
		nome := eu.Nome
		if nome == "" {
			nome = "Error"
		}
		return execucoes.Fim{Estado: dominio.ExecErro, Erro: nome + ": " + eu.Mensagem, ErroStack: eu.Stack}
	case errors.As(err, &er):
		for _, l := range er.Stderr {
			e.Logar("erro", l)
		}
		if ctx.Err() != nil {
			return execucoes.Fim{Estado: dominio.ExecAbortada, Motivo: "app fechado"}
		}
		return execucoes.Fim{Estado: dominio.ExecErro, Erro: er.Mensagem}
	}
	if ctx.Err() != nil {
		return execucoes.Fim{Estado: dominio.ExecAbortada, Motivo: "app fechado"}
	}
	return execucoes.Fim{Estado: dominio.ExecErro, Erro: err.Error()}
}

// conversaProt monta `conversa` do pedido executar (null sem conversa; fictícia no teste).
func (ia *IA) conversaProt(ctx context.Context, c *acoes.Contexto) json.RawMessage {
	if c.ConversaFicticia {
		b, _ := json.Marshal(map[string]any{"id": ponte.ConversaFicticiaID, "conta_id": "", "tipo": "individual", "nome": "Contato de teste",
			"telefone": "+5500000000000", "lead_id": nil, "contato": map[string]any{"id": "contato-ficticio", "nome": "Contato de teste",
				"nome_push": nil, "telefone": "+5500000000000", "notas": nil, "etiquetas": []any{}}})
		return b
	}
	if c.Alvo.ConversaID == "" {
		return json.RawMessage("null")
	}
	l := ia.s.d.Banco.L()
	cv, err := armazenamento.ObterConversa(ctx, l, c.Alvo.ConversaID)
	if err != nil {
		return json.RawMessage("null")
	}
	conv := map[string]any{"id": cv.ID, "conta_id": cv.ContaID, "tipo": cv.Tipo, "nome": nilSeVazio(cv.Nome), "telefone": cv.Telefone,
		"lead_id": nil, "contato": nil}
	if cv.ContatoID != nil {
		if ct, err := armazenamento.ObterContato(ctx, l, *cv.ContatoID); err == nil {
			ets := []map[string]string{}
			for _, e := range ct.Etiquetas {
				ets = append(ets, map[string]string{"id": e.ID, "nome": e.Nome, "cor": e.Cor})
			}
			conv["contato"] = map[string]any{"id": ct.ID, "nome": ct.Nome, "nome_push": ct.NomePush, "telefone": ct.Telefone,
				"notas": ct.Notas, "etiquetas": ets}
			if ct.Lead != nil {
				conv["lead_id"] = ct.Lead.ID
			}
		}
	}
	b, _ := json.Marshal(conv)
	return b
}

func nilSeVazio(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// argumento monta o segundo parâmetro do handler (formato camelCase da SDK).
func (ia *IA) argumento(ctx context.Context, handler string, d dominio.ExecucaoDetalhe, c *acoes.Contexto, entrada json.RawMessage) (json.RawMessage, error) {
	l := ia.s.d.Banco.L()
	g := d.Gatilho
	str := func(k string) string {
		v, _ := g.Dados[k].(string)
		return v
	}
	var v any
	switch handler {
	case modelo.HandlerExecutar:
		if len(entrada) == 0 {
			return json.RawMessage("null"), nil
		}
		return entrada, nil
	case modelo.HandlerMensagem:
		texto := ""
		if c.UltimaMensagem != nil {
			texto = *c.UltimaMensagem
		}
		m := map[string]any{"id": "mensagem-teste", "conversaId": c.Alvo.ConversaID, "deMim": false, "automacaoId": nil, "tipo": "texto",
			"texto": texto, "remetenteNome": nil, "enviadaEm": ia.s.d.Relogio.Agora().Format(time.RFC3339), "primeira": false, "midia": nil}
		if c.ConversaFicticia {
			m["conversaId"], m["remetenteNome"] = ponte.ConversaFicticiaID, "Contato de teste"
		}
		if mid := str("mensagem_id"); mid != "" {
			if b, err := armazenamento.ObterMensagem(ctx, l, mid); err == nil {
				m["id"], m["conversaId"], m["tipo"], m["texto"], m["remetenteNome"] = b.ID, b.ConversaID, b.Tipo, b.Texto, b.RemetenteNome
				m["enviadaEm"] = b.EnviadaEm.Format(time.RFC3339)
				var n int
				l.QueryRowContext(ctx, `SELECT count(*) FROM mensagens WHERE conversa_id = ? AND de_mim = 0 AND (enviada_em < ? OR (enviada_em = ? AND rid < ?))`,
					b.ConversaID, armazenamento.Ms(b.EnviadaEm), armazenamento.Ms(b.EnviadaEm), b.Rid).Scan(&n)
				m["primeira"] = n == 0
				if b.Midia != nil {
					m["midia"] = map[string]any{"mimetype": b.Midia.Mimetype, "nomeArquivo": b.Midia.NomeArquivo, "tamanho": b.Midia.Tamanho}
				}
			}
		}
		v = m
	case modelo.HandlerAgendar:
		origem := "agendar"
		if g.Tipo == modelo.GatilhoAgendamento {
			origem = "intervalo"
			if str("cron") != "" {
				origem = "cron"
			}
		}
		var id any
		if x := str("agendamento_id"); x != "" {
			id = x
		}
		previsto := g.Dados["previsto_para"]
		if previsto == nil {
			previsto = g.Dados["ocorrencia"]
		}
		if previsto == nil {
			previsto = ia.s.d.Relogio.Agora().Format(time.RFC3339)
		}
		v = map[string]any{"origem": origem, "id": id, "previstoPara": previsto, "dados": g.Dados["dados"]}
	case modelo.HandlerEvento:
		ev, err := ia.evento(ctx, g)
		if err != nil {
			return nil, err
		}
		v = ev
	}
	return json.Marshal(v)
}

func (ia *IA) evento(ctx context.Context, g dominio.GatilhoExecucao) (map[string]any, error) {
	l := ia.s.d.Banco.L()
	str := func(k string) any {
		if v, ok := g.Dados[k].(string); ok && v != "" {
			return v
		}
		return nil
	}
	switch g.Tipo {
	case modelo.GatilhoLeadImportado:
		var lead any
		if ids, ok := g.Dados["lead_ids"].([]any); ok && len(ids) > 0 {
			if id, _ := ids[0].(string); id != "" {
				if ld, err := armazenamento.ObterLead(ctx, l, id); err == nil {
					lead = map[string]any{"id": ld.ID, "telefone": ld.Telefone, "nome": ld.Nome, "campos": ld.Campos, "origem": ld.Origem,
						"importadoEm": ld.ImportadoEm.Format(time.RFC3339)}
				}
			}
		}
		if ids, ok := g.Dados["lead_ids"].([]string); ok && len(ids) > 0 && lead == nil {
			if ld, err := armazenamento.ObterLead(ctx, l, ids[0]); err == nil {
				lead = map[string]any{"id": ld.ID, "telefone": ld.Telefone, "nome": ld.Nome, "campos": ld.Campos, "origem": ld.Origem,
					"importadoEm": ld.ImportadoEm.Format(time.RFC3339)}
			}
		}
		return map[string]any{"tipo": g.Tipo, "lead": lead}, nil
	case modelo.GatilhoEtiqueta:
		et := map[string]any{"id": str("etiqueta_id"), "nome": "", "cor": ""}
		if id, _ := g.Dados["etiqueta_id"].(string); id != "" {
			if e, err := armazenamento.ObterEtiqueta(ctx, l, id); err == nil {
				et = map[string]any{"id": e.ID, "nome": e.Nome, "cor": e.Cor}
			}
		}
		return map[string]any{"tipo": g.Tipo, "evento": str("evento"), "etiqueta": et, "contatoId": str("contato_id")}, nil
	case modelo.GatilhoEntrouEtapa:
		return map[string]any{"tipo": g.Tipo, "funilId": str("funil_id"), "etapaId": str("etapa_id"), "leadId": str("lead_id"),
			"etapaAnteriorId": str("etapa_anterior_id")}, nil
	case modelo.GatilhoDisparoRespondeu:
		return map[string]any{"tipo": g.Tipo, "disparoId": str("disparo_id"), "destinatarioId": str("destinatario_id"), "mensagemId": str("mensagem_id")}, nil
	case modelo.GatilhoSemResposta:
		return map[string]any{"tipo": g.Tipo, "aposSegundos": g.Dados["apos_s"], "mensagemReferenciaId": str("mensagem_id")}, nil
	}
	d := map[string]any{"tipo": g.Tipo}
	for k, v := range g.Dados {
		d[k] = v
	}
	return d, nil
}

// ---------------------------------------------------------------------------
// Teste
// ---------------------------------------------------------------------------

// Testar roda em simulação (compila a versão atual; erro de compilação → compilacao_falhou).
func (ia *IA) Testar(ctx context.Context, a dominio.Automacao, p PedidoTeste) (dominio.ExecucaoDetalhe, error) {
	if !ia.pool.Disponivel() {
		return dominio.ExecucaoDetalhe{}, erros.Novo(erros.RunnerIndisponivel, "As automações de IA não podem ser executadas nesta instalação.")
	}
	if _, _, err := ia.preparar(ctx, a.ID, true); err != nil {
		return dominio.ExecucaoDetalhe{}, err
	}
	a, _ = armazenamento.ObterAutomacao(ctx, ia.s.d.Banco.L(), a.ID)
	alvo, g, texto, ficticia, err := ia.s.ContextoTeste(ctx, p)
	if err != nil {
		return dominio.ExecucaoDetalhe{}, err
	}
	handler := modelo.HandlerExecutar
	var entrada json.RawMessage
	switch {
	case texto != nil:
		handler = modelo.HandlerMensagem
	case p.Evento != nil:
		handler = handlerDe(g)
		if handler == modelo.HandlerExecutar {
			handler = modelo.HandlerEvento
		}
	default:
		if len(p.Entrada) > 0 {
			entrada = p.Entrada
			g.Dados["entrada"] = p.Entrada
		}
	}
	e, err := ia.s.d.Registro.Criar(ctx, execucoes.Nova{Automacao: a, Gatilho: g, Origem: "teste", Simulacao: true,
		Alvo: execucoes.Alvo{ContaID: alvo.ContaID, ConversaID: alvo.ConversaID, ContatoID: alvo.ContatoID, LeadID: alvo.LeadID}})
	if err != nil {
		return dominio.ExecucaoDetalhe{}, err
	}
	e.Iniciar(ctx)
	if ficticia {
		e.Logar("info", "Conversa fictícia: Contato de teste (+5500000000000), sem histórico.")
	}
	if p.IASimulada {
		e.Logar("info", "IA simulada: nenhuma chamada à Claude API.")
	}
	c := &acoes.Contexto{Exec: e, Automacao: a, Alvo: alvo, Simulacao: true, Cadeia: []string{a.ID}, Variaveis: map[string]string{},
		UltimaMensagem: texto, Origem: "teste", ConversaFicticia: ficticia, IASimulada: p.IASimulada}
	prazo, _ := ia.limites(a)
	ctxT, cancelar := context.WithTimeout(ctx, prazo+5*time.Second)
	defer cancelar()
	ia.rodar(ctxT, e, a.ID, c, handler, entrada, true, p.IASimulada)
	return armazenamento.ObterExecucao(ctx, ia.s.d.Banco.L(), e.ID())
}

// ---------------------------------------------------------------------------
// Arquivos do projeto (rotas)
// ---------------------------------------------------------------------------

func (ia *IA) exigirIA(ctx context.Context, id string) error {
	a, err := armazenamento.ObterAutomacao(ctx, ia.s.d.Banco.L(), id)
	if err != nil {
		return err
	}
	if a.Tipo != modelo.TipoIA {
		return erros.Campo("tipo", "Só automações de IA têm arquivos de projeto.")
	}
	return nil
}

func (ia *IA) alterados(id string, caminhos ...string) {
	ia.s.d.Barramento.Publicar(eventos.AutomacaoArquivosAlterados, "", map[string]any{"automacao_id": id, "caminhos": caminhos, "origem": "api"})
}

// ListarArquivos do projeto.
func (ia *IA) ListarArquivos(ctx context.Context, id string) ([]projetos.Arquivo, error) {
	if err := ia.exigirIA(ctx, id); err != nil {
		return nil, err
	}
	return ia.projetos.Listar(id)
}

// LerArquivo do projeto.
func (ia *IA) LerArquivo(ctx context.Context, id, caminho string) (projetos.Conteudo, error) {
	if err := ia.exigirIA(ctx, id); err != nil {
		return projetos.Conteudo{}, err
	}
	return ia.projetos.Ler(id, caminho)
}

// EscreverArquivo grava (não compila); publica automacao.arquivos_alterados.
func (ia *IA) EscreverArquivo(ctx context.Context, id, caminho, conteudo string, verificar bool, hashAnterior *string) (projetos.Arquivo, bool, error) {
	if err := ia.exigirIA(ctx, id); err != nil {
		return projetos.Arquivo{}, false, err
	}
	a, criado, err := ia.projetos.Escrever(id, caminho, conteudo, verificar, hashAnterior)
	if err == nil {
		ia.alterados(id, a.Caminho)
	}
	return a, criado, err
}

// ExcluirArquivo do projeto.
func (ia *IA) ExcluirArquivo(ctx context.Context, id, caminho string) error {
	if err := ia.exigirIA(ctx, id); err != nil {
		return err
	}
	err := ia.projetos.Excluir(id, caminho)
	if err == nil {
		ia.alterados(id, caminho)
	}
	return err
}

// RenomearArquivo do projeto.
func (ia *IA) RenomearArquivo(ctx context.Context, id, de, para string) (projetos.Arquivo, error) {
	if err := ia.exigirIA(ctx, id); err != nil {
		return projetos.Arquivo{}, err
	}
	a, err := ia.projetos.Renomear(id, de, para)
	if err == nil {
		ia.alterados(id, de, a.Caminho)
	}
	return a, err
}

// CompilarAPI é o POST /automacoes/{id}/compilar.
func (ia *IA) CompilarAPI(ctx context.Context, id string) (compilador.Resultado, error) {
	if err := ia.exigirIA(ctx, id); err != nil {
		return compilador.Resultado{}, err
	}
	res, err := ia.Compilar(ctx, id)
	if res.Erros == nil {
		res.Erros = []dominio.ErroCompilacao{}
	}
	if res.Avisos == nil {
		res.Avisos = []dominio.ErroCompilacao{}
	}
	if res.Handlers == nil {
		res.Handlers = []string{}
	}
	return res, err
}

// Encerrar para os runners (encerramento do motor).
func (ia *IA) Encerrar(ctx context.Context) { ia.pool.EncerrarTodos(ctx) }

// Configurar aplica os ajustes de processos e ociosidade.
func (ia *IA) Configurar(c dominio.ConfiguracaoAutomacoes) {
	ia.pool.Configurar(c.ProcessosIAMax, time.Duration(c.OciosidadeIAMin)*time.Minute)
}
