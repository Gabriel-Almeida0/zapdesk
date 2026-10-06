package ponte

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/automacoes/acoes"
	"zapdesk/motor/internal/automacoes/execucoes"
	"zapdesk/motor/internal/automacoes/modelo"
	"zapdesk/motor/internal/automacoes/seguranca"
	"zapdesk/motor/internal/automacoes/testeapoio"
	"zapdesk/motor/internal/claude"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/eventos"
	"zapdesk/motor/internal/funil"
	"zapdesk/motor/internal/leads"
	"zapdesk/motor/internal/organizacao"
	"zapdesk/motor/internal/segredos"
)

type cfg struct {
	c dominio.ConfiguracaoAutomacoes
}

func (c *cfg) Configuracao() dominio.ConfiguracaoAutomacoes { return c.c }

type pub struct{}

func (pub) PublicarContato(context.Context, string) {}

type esperas struct{ l []dominio.Espera }

func (e *esperas) Gravar(_ context.Context, x dominio.Espera) error {
	e.l = append(e.l, x)
	return nil
}

type amb struct {
	*testeapoio.Ambiente
	p     *Ponte
	reg   *execucoes.Registro
	cfg   *cfg
	ia    *claude.Falsa
	esp   *esperas
	funil *funil.Servico
	conta string
	conv  string
	ct    string
	aut   dominio.Automacao
}

func montar(t *testing.T) *amb {
	a := testeapoio.Novo(t)
	c := &cfg{c: dominio.ConfiguracaoPadrao()}
	reg := execucoes.Novo(a.Banco, a.Barramento, a.Relogio)
	portao := seguranca.NovoPortao(a.Banco, a.Barramento, a.Relogio, c)
	ls := leads.NovoServico(a.Banco, a.Barramento, a.Relogio)
	fs := funil.Novo(a.Banco, a.Barramento, a.Relogio, ls)
	org := organizacao.Novo(a.Banco, a.Barramento, a.Relogio, pub{})
	ac := acoes.Novo(acoes.Deps{Banco: a.Banco, Barramento: a.Barramento, Relogio: a.Relogio, Leads: ls, Funil: fs, Portao: portao, Organizacao: org})
	ia := claude.NovaFalsa(a.Relogio.Agora)
	esp := &esperas{}
	p := Nova(Deps{Banco: a.Banco, Barramento: a.Barramento, Relogio: a.Relogio, Acoes: ac, Organizacao: org, Leads: ls, Funil: fs,
		Portao: portao, Claude: ia, IASimulada: ia, Esperas: esp})
	conta := a.Conta("Loja")
	conv, ct := a.Conversa(conta, "+5511944440000")
	id := a.Automacao("ia", "Minha IA", []modelo.Gatilho{{Tipo: "manual"}}, nil)
	au, _ := armazenamento.ObterAutomacao(a.Ctx, a.Banco.L(), id)
	return &amb{Ambiente: a, p: p, reg: reg, cfg: c, ia: ia, esp: esp, funil: fs, conta: conta, conv: conv, ct: ct, aut: au}
}

func (a *amb) sessao(t *testing.T, perms []string, sim bool, semAlvo bool) (*Sessao, *execucoes.Exec) {
	e, err := a.reg.Criar(a.Ctx, execucoes.Nova{Automacao: a.aut, Origem: "teste", Simulacao: sim})
	if err != nil {
		t.Fatal(err)
	}
	e.Iniciar(a.Ctx)
	c := &acoes.Contexto{Exec: e, Automacao: a.aut, Simulacao: sim, Cadeia: []string{a.aut.ID}, Variaveis: map[string]string{}}
	if !semAlvo {
		c.Alvo = acoes.Alvo{ContaID: a.conta, ConversaID: a.conv, ContatoID: a.ct}
	}
	return a.p.Sessao(Opcoes{Exec: e, Automacao: a.aut, Permissoes: perms, Contexto: c}), e
}

func chamar(t *testing.T, s *Sessao, e *execucoes.Exec, metodo string, params any) (json.RawMessage, int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(params)
	r, er := s.Chamar(context.Background(), e.ID(), metodo, b)
	if er != nil {
		d, _ := er.Dados.(map[string]any)
		return nil, er.Codigo, d
	}
	return r, 0, nil
}

func TestPermissoesPorMetodo(t *testing.T) {
	a := montar(t)
	s, e := a.sessao(t, nil, false, false)
	for metodo, perm := range permissaoDoMetodo {
		_, cod, dados := chamar(t, s, e, metodo, map[string]any{"chave": "x", "titulo": "t", "texto": "x"})
		if perm == "" {
			if cod == CodPermissao {
				t.Errorf("%s não exige permissão: %v", metodo, dados)
			}
			continue
		}
		if cod != CodPermissao || dados["permissao"] != perm || dados["codigo"] != "permissao_negada" {
			t.Errorf("%s deveria exigir %s: %d %v", metodo, perm, cod, dados)
		}
	}
	_, cod, _ := chamar(t, s, e, "ctx.inexistente", nil)
	if cod != -32601 {
		t.Fatalf("método desconhecido: %d", cod)
	}
}

func TestSimulacaoNaoEscreve(t *testing.T) {
	a := montar(t)
	f, _ := a.funil.Criar(a.Ctx, "Vendas", []funil.NovaEtapa{{Nome: "Novo"}, {Nome: "Quente"}})
	a.Exec(`INSERT INTO etiquetas (id, nome, cor, criada_em) VALUES ('e1', 'Quente', '#FF0000', 0)`)
	todas := modelo.Permissoes
	s, e := a.sessao(t, todas, true, false)
	r, cod, _ := chamar(t, s, e, "ctx.enviar", map[string]any{"destino": map[string]any{"conversa_id": a.conv}, "conteudo": map[string]any{"texto": "Olá!"}})
	var m mensagemEnviada
	json.Unmarshal(r, &m)
	if cod != 0 || m.ID != nil || !m.Simulada || *m.Texto != "Olá!" {
		t.Fatalf("enviar simulado: %s %d", r, cod)
	}
	if _, cod, _ := chamar(t, s, e, "ctx.etiquetas.adicionar", map[string]any{"etiqueta": "quente"}); cod != 0 {
		t.Fatalf("etiqueta simulada: %d", cod)
	}
	r, cod, _ = chamar(t, s, e, "ctx.funil.mover", map[string]any{"funil": "vendas", "etapa": "QUENTE"})
	if cod != 0 || !strings.Contains(string(r), f.Etapas[1].ID) {
		t.Fatalf("funil simulado: %s %d", r, cod)
	}
	chamar(t, s, e, "ctx.notificar", map[string]any{"titulo": "t", "texto": "x"})
	chamar(t, s, e, "ctx.humano.transferir", map[string]any{"mensagem": "Vou chamar alguém"})
	r, _, _ = chamar(t, s, e, "ctx.agendar", map[string]any{"em": a.Relogio.Agora().Add(10 * 60 * 1e9).Format("2006-01-02T15:04:05Z07:00"), "na_conversa": true})
	if !strings.Contains(string(r), "simulado-") {
		t.Fatalf("agendar simulado: %s", r)
	}
	// Memória simulada: lida pela própria execução, nunca gravada.
	chamar(t, s, e, "ctx.memoria.definir", map[string]any{"chave": "k", "escopo": "global", "valor": map[string]int{"n": 1}})
	r, _, _ = chamar(t, s, e, "ctx.memoria.obter", map[string]any{"chave": "k", "escopo": "global"})
	if string(r) != `{"valor":{"n":1}}` {
		t.Fatalf("memória simulada: %s", r)
	}
	r, _, _ = chamar(t, s, e, "ctx.memoria.listar", map[string]any{"escopo": "global"})
	if !strings.Contains(string(r), `"chave":"k"`) {
		t.Fatalf("listar simulado: %s", r)
	}
	var n int
	a.Banco.L().QueryRow(`SELECT (SELECT count(*) FROM memoria_automacoes) + (SELECT count(*) FROM contato_etiquetas) +
		(SELECT count(*) FROM posicoes_funil) + (SELECT count(*) FROM pausas_conversa) + (SELECT count(*) FROM mensagens) + (SELECT count(*) FROM leads)`).Scan(&n)
	if n != 0 || len(a.esp.l) != 0 || len(a.EventosDoTipo(eventos.Notificacao)) != 0 {
		t.Fatalf("simulação escreveu: %d linhas, %d esperas", n, len(a.esp.l))
	}
	d := e.Detalhe()
	for _, ac := range d.Acoes {
		if ac.Resultado != dominio.AcaoSimulada {
			t.Fatalf("ação não simulada: %+v", ac)
		}
	}
	if len(d.Acoes) < 5 {
		t.Fatalf("ações simuladas: %+v", d.Acoes)
	}
	// Outra execução não enxerga a memória simulada da primeira.
	s2, e2 := a.sessao(t, todas, true, false)
	r, _, _ = chamar(t, s2, e2, "ctx.memoria.obter", map[string]any{"chave": "k", "escopo": "global"})
	if string(r) != `{"valor":null}` {
		t.Fatalf("memória vazou entre execuções: %s", r)
	}
}

func TestMemoriaRealEscoposELimites(t *testing.T) {
	a := montar(t)
	s, e := a.sessao(t, nil, false, false)
	chamar(t, s, e, "ctx.memoria.definir", map[string]any{"chave": "visitas", "escopo": "contato", "valor": 3})
	chamar(t, s, e, "ctx.memoria.definir", map[string]any{"chave": "total", "escopo": "global", "valor": 10})
	var esc string
	a.Banco.L().QueryRow(`SELECT escopo FROM memoria_automacoes WHERE chave = 'visitas'`).Scan(&esc)
	if esc != "contato:"+a.ct {
		t.Fatalf("escopo contato: %q", esc)
	}
	r, _, _ := chamar(t, s, e, "ctx.memoria.remover", map[string]any{"chave": "total", "escopo": "global"})
	if string(r) != `{"removida":true}` {
		t.Fatal(string(r))
	}
	grande := strings.Repeat("x", MaxValorMemoria)
	if _, cod, _ := chamar(t, s, e, "ctx.memoria.definir", map[string]any{"chave": "g", "escopo": "global", "valor": grande}); cod != CodLimite {
		t.Fatalf("valor > 64 KB: %d", cod)
	}
	if _, cod, _ := chamar(t, s, e, "ctx.memoria.obter", map[string]any{"chave": "", "escopo": "global"}); cod != CodValidacao {
		t.Fatalf("chave vazia: %d", cod)
	}
	// Sem contato: escopo contato é validação; alvo ausente → 1002.
	s2, e2 := a.sessao(t, modelo.Permissoes, false, true)
	if _, cod, _ := chamar(t, s2, e2, "ctx.memoria.obter", map[string]any{"chave": "k", "escopo": "contato"}); cod != CodValidacao {
		t.Fatalf("escopo contato sem contato: %d", cod)
	}
	if _, cod, _ := chamar(t, s2, e2, "ctx.etiquetas.do_contato", map[string]any{}); cod != CodValidacao {
		t.Fatalf("alvo ausente: %d", cod)
	}
}

func TestEncerradaNotificacoesBloqueioEIA(t *testing.T) {
	a := montar(t)
	s, e := a.sessao(t, []string{"enviar", "ia"}, false, false)
	for i := 0; i < MaxNotificacoes; i++ {
		if _, cod, _ := chamar(t, s, e, "ctx.notificar", map[string]any{"titulo": "t", "texto": "x"}); cod != 0 {
			t.Fatalf("notificação %d: %d", i, cod)
		}
	}
	if _, cod, _ := chamar(t, s, e, "ctx.notificar", map[string]any{"titulo": "t", "texto": "x"}); cod != CodLimite {
		t.Fatalf("6ª notificação: %d", cod)
	}
	// Portão: pausa geral → 1004 com o motivo.
	a.cfg.c.PausaGeral = true
	_, cod, dados := chamar(t, s, e, "ctx.enviar", map[string]any{"destino": map[string]any{"conversa_id": a.conv}, "conteudo": map[string]any{"texto": "oi"}})
	if cod != CodBloqueado || dados["motivo"] != "pausa_geral" {
		t.Fatalf("bloqueio: %d %v", cod, dados)
	}
	a.cfg.c.PausaGeral = false
	// IA: sem chave no cliente real → 1005; simulada soma tokens (0) e registra a chamada.
	real := claude.NovoReal(claude.OpcoesReal{Chave: segredos.Novo(), URLBase: "http://127.0.0.1:1"})
	a.p.d.Claude = real
	if _, cod, _ := chamar(t, s, e, "ctx.ia.gerar", map[string]any{"prompt": "oi"}); cod != CodIANaoConfigurada {
		t.Fatalf("sem chave: %d", cod)
	}
	a.p.d.Claude = a.ia
	r, cod, _ := chamar(t, s, e, "ctx.ia.classificar", map[string]any{"texto": "quero comprar", "categorias": map[string]any{"quente": nil, "frio": "sem interesse"}})
	if cod != 0 || !strings.Contains(string(r), `"categoria"`) {
		t.Fatalf("classificar: %s %d", r, cod)
	}
	if len(a.ia.Chamadas()) != 1 {
		t.Fatal("chamada à IA não registrada")
	}
	// Depois do fim: execucao_encerrada; outro id de execução também.
	s.Encerrar()
	if _, cod, _ := chamar(t, s, e, "ctx.memoria.obter", map[string]any{"chave": "k"}); cod != CodExecucaoEncerrada {
		t.Fatalf("encerrada: %d", cod)
	}
	s3, _ := a.sessao(t, nil, false, false)
	if _, er := s3.Chamar(context.Background(), "outra", "ctx.memoria.obter", json.RawMessage(`{"chave":"k"}`)); er == nil || er.Codigo != CodExecucaoEncerrada {
		t.Fatalf("execução de outra sessão: %v", er)
	}
}
