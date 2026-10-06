package seguranca

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"zapdesk/motor/internal/automacoes/testeapoio"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/eventos"
)

type cfgFixa struct {
	c dominio.ConfiguracaoAutomacoes
}

func (f *cfgFixa) Configuracao() dominio.ConfiguracaoAutomacoes { return f.c }

func montar(t *testing.T) (*testeapoio.Ambiente, *Portao, *cfgFixa) {
	amb := testeapoio.Novo(t)
	cfg := &cfgFixa{c: dominio.ConfiguracaoPadrao()}
	p := NovoPortao(amb.Banco, amb.Barramento, amb.Relogio, cfg)
	return amb, p, cfg
}

func TestOrdemDasVerificacoes(t *testing.T) {
	amb, p, cfg := montar(t)
	conta := amb.Conta("Loja")
	conv, _ := amb.Conversa(conta, "+5511900000001")
	grupo := amb.Grupo(conta, "Turma")
	aut := amb.Automacao("fluxo", "F", []any{}, nil)
	pedido := Pedido{ConversaID: conv, AutomacaoID: aut}

	// Tudo livre.
	if d := p.Autorizar(amb.Ctx, pedido); !d.Permitido {
		t.Fatalf("deveria permitir: %+v", d)
	}
	// Pausa geral vence tudo (mesmo com pausa da conversa e grupo).
	cfg.c.PausaGeral = true
	p.Pausas().Assumir(amb.Ctx, conv, dominio.PausaManual, nil)
	if d := p.Autorizar(amb.Ctx, pedido); d.Permitido || d.Motivo != MotivoPausaGeral {
		t.Fatalf("pausa geral: %+v", d)
	}
	cfg.c.PausaGeral = false
	if d := p.Autorizar(amb.Ctx, pedido); d.Permitido || d.Motivo != MotivoPausa {
		t.Fatalf("pausa da conversa: %+v", d)
	}
	p.Pausas().Retomar(amb.Ctx, conv)
	// Grupo sem incluir_grupos.
	if d := p.Autorizar(amb.Ctx, Pedido{ConversaID: grupo, AutomacaoID: aut}); d.Permitido || d.Motivo != MotivoGrupo {
		t.Fatalf("grupo: %+v", d)
	}
	if d := p.Autorizar(amb.Ctx, Pedido{ConversaID: grupo, AutomacaoID: aut, IncluirGrupos: true}); !d.Permitido {
		t.Fatalf("grupo incluído: %+v", d)
	}
}

func TestAntiLoopPadraoCriaPausaENotifica(t *testing.T) {
	amb, p, _ := montar(t)
	conta := amb.Conta("Loja")
	conv, _ := amb.Conversa(conta, "+5511900000002")
	aut := amb.Automacao("fluxo", "F", []any{}, nil)
	amb.Mensagem(conta, conv, false, "oi", "", false, amb.Relogio.Agora())
	// 10 automáticas nos últimos 10 min (inclusive de outra automação) → a 11ª é barrada.
	outra := amb.Automacao("fluxo", "G", []any{}, nil)
	for i := 0; i < 10; i++ {
		a := aut
		if i%2 == 0 {
			a = outra
		}
		if d := p.Autorizar(amb.Ctx, Pedido{ConversaID: conv, AutomacaoID: a}); !d.Permitido {
			t.Fatalf("envio %d barrado: %+v", i+1, d)
		}
		amb.Mensagem(conta, conv, true, "auto", a, false, amb.Relogio.Agora())
		amb.Relogio.Avancar(30 * time.Second)
	}
	d := p.Autorizar(amb.Ctx, Pedido{ConversaID: conv, AutomacaoID: aut})
	if d.Permitido || d.Motivo != MotivoAntiLoop {
		t.Fatalf("11ª deveria ser barrada por anti-loop: %+v", d)
	}
	pausa, ok := p.Pausas().Obter(amb.Ctx, conv)
	if !ok || pausa.Motivo != dominio.PausaAntiLoop || pausa.Ate == nil || !pausa.Ate.Equal(amb.Relogio.Agora().Add(60*time.Minute)) ||
		pausa.AutomacaoID == nil || *pausa.AutomacaoID != aut {
		t.Fatalf("pausa anti-loop: %+v %v", pausa, ok)
	}
	if n := amb.EventosDoTipo(eventos.Notificacao); len(n) != 1 {
		t.Fatalf("notificação anti-loop: %d", len(n))
	} else if not := n[0].Dados.(dominio.Notificacao); not.Tipo != "anti_loop" || not.ConversaID == nil {
		t.Fatalf("notificação: %+v", not)
	}
	if len(amb.EventosDoTipo(eventos.ConversaPausa)) == 0 {
		t.Fatal("conversa.pausa não publicado")
	}
	// Mensagens fora da janela não contam: 61 min depois a pausa venceu e a janela esvaziou.
	amb.Relogio.Avancar(61 * time.Minute)
	if d := p.Autorizar(amb.Ctx, Pedido{ConversaID: conv, AutomacaoID: aut}); !d.Permitido {
		t.Fatalf("após a pausa e fora da janela: %+v", d)
	}
}

func TestAntiLoopProprioSoMaisRestritivo(t *testing.T) {
	amb, p, _ := montar(t)
	conta := amb.Conta("Loja")
	conv, _ := amb.Conversa(conta, "+5511900000003")
	aut := amb.Automacao("fluxo", "F", []any{}, nil)
	amb.Mensagem(conta, conv, false, "oi", "", false, amb.Relogio.Agora())
	for i := 0; i < 3; i++ {
		amb.Mensagem(conta, conv, true, "auto", aut, false, amb.Relogio.Agora())
	}
	proprio := &dominio.AntiLoop{Mensagens: 3, JanelaMin: 10}
	if d := p.Autorizar(amb.Ctx, Pedido{ConversaID: conv, AutomacaoID: aut, AntiLoop: proprio}); d.Permitido || d.Motivo != MotivoAntiLoop {
		t.Fatalf("limite próprio de 3: %+v", d)
	}
	p.Pausas().Retomar(amb.Ctx, conv)
	// Limite próprio mais frouxo (50) não afrouxa o global (10).
	for i := 0; i < 7; i++ {
		amb.Mensagem(conta, conv, true, "auto", aut, false, amb.Relogio.Agora())
	}
	frouxo := &dominio.AntiLoop{Mensagens: 50, JanelaMin: 10}
	if d := p.Autorizar(amb.Ctx, Pedido{ConversaID: conv, AutomacaoID: aut, AntiLoop: frouxo}); d.Permitido || d.Motivo != MotivoAntiLoop {
		t.Fatalf("limite frouxo não pode afrouxar o global: %+v", d)
	}
}

func TestPrimeiroContatoPorHora(t *testing.T) {
	amb, p, cfg := montar(t)
	conta := amb.Conta("Loja")
	aut := amb.Automacao("fluxo", "F", []any{}, nil)
	cfg.c.PrimeirosContatosHora = 2
	for i := 0; i < 2; i++ {
		conv, _ := amb.Conversa(conta, "+55119000001"+string(rune('0'+i))+"0")
		d := p.Autorizar(amb.Ctx, Pedido{ConversaID: conv, AutomacaoID: aut})
		if !d.Permitido || !d.PrimeiroContato {
			t.Fatalf("primeiro contato %d: %+v", i, d)
		}
		amb.Mensagem(conta, conv, true, "oi", aut, true, amb.Relogio.Agora())
	}
	conv3, _ := amb.Conversa(conta, "+5511900000999")
	if d := p.Autorizar(amb.Ctx, Pedido{ConversaID: conv3, AutomacaoID: aut}); d.Permitido || d.Motivo != MotivoPrimeiroContato {
		t.Fatalf("3º primeiro contato na hora: %+v", d)
	}
	// Conversa que já tem mensagens não é primeiro contato.
	conv4, _ := amb.Conversa(conta, "+5511900000998")
	amb.Mensagem(conta, conv4, false, "oi", "", false, amb.Relogio.Agora())
	if d := p.Autorizar(amb.Ctx, Pedido{ConversaID: conv4, AutomacaoID: aut}); !d.Permitido || d.PrimeiroContato {
		t.Fatalf("conversa existente: %+v", d)
	}
	// Uma hora depois o limite libera; 0 bloqueia sempre.
	amb.Relogio.Avancar(61 * time.Minute)
	if d := p.Autorizar(amb.Ctx, Pedido{ConversaID: conv3, AutomacaoID: aut}); !d.Permitido {
		t.Fatalf("depois de 1 h: %+v", d)
	}
	cfg.c.PrimeirosContatosHora = 0
	if d := p.Autorizar(amb.Ctx, Pedido{ConversaID: conv3, AutomacaoID: aut}); d.Permitido || d.Motivo != MotivoPrimeiroContato {
		t.Fatalf("0 bloqueia: %+v", d)
	}
}

// Execuções concorrentes (ex.: lead_importado com vários leads) não podem ler a mesma contagem
// antes de qualquer envio gravado: AutorizarEEnviar serializa decisão + gravação por conta.
func TestPrimeiroContatoConcorrenteNaoEstouraLimite(t *testing.T) {
	amb, p, cfg := montar(t)
	conta := amb.Conta("Loja")
	aut := amb.Automacao("fluxo", "F", []any{}, nil)
	cfg.c.PrimeirosContatosHora = 2
	convs := make([]string, 8)
	for i := range convs {
		convs[i], _ = amb.Conversa(conta, fmt.Sprintf("+55119000020%02d", i))
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	permitidos := 0
	for _, conv := range convs {
		wg.Add(1)
		go func(conv string) {
			defer wg.Done()
			d, err := p.AutorizarEEnviar(amb.Ctx, Pedido{ConversaID: conv, AutomacaoID: aut}, func(d Decisao) error {
				time.Sleep(5 * time.Millisecond) // alarga a janela da corrida
				amb.Mensagem(conta, conv, true, "oi", aut, d.PrimeiroContato, amb.Relogio.Agora())
				return nil
			})
			if err != nil {
				t.Error(err)
			}
			if d.Permitido {
				mu.Lock()
				permitidos++
				mu.Unlock()
			}
		}(conv)
	}
	wg.Wait()
	if permitidos != 2 {
		t.Fatalf("permitidos: %d (limite 2)", permitidos)
	}
}

func TestPausasHumanoAssumirEVencimento(t *testing.T) {
	amb, p, _ := montar(t)
	conta := amb.Conta("Loja")
	conv, _ := amb.Conversa(conta, "+5511900000004")
	ps := p.Pausas()
	agora := amb.Relogio.Agora()

	// Mensagem manual → humano 30 min; nova manual renova.
	ps.PausarHumano(amb.Ctx, conv)
	pz, ok := ps.Obter(amb.Ctx, conv)
	if !ok || pz.Motivo != dominio.PausaHumano || !pz.Ate.Equal(agora.Add(30*time.Minute)) {
		t.Fatalf("humano: %+v", pz)
	}
	amb.Relogio.Avancar(10 * time.Minute)
	ps.PausarHumano(amb.Ctx, conv)
	pz, _ = ps.Obter(amb.Ctx, conv)
	if !pz.Ate.Equal(agora.Add(40 * time.Minute)) {
		t.Fatalf("renovação: %+v", pz)
	}
	// Vencida é ignorada (e apagada preguiçosamente, publicando conversa.pausa null).
	amb.Relogio.Avancar(31 * time.Minute)
	amb.Eventos()
	if _, ok := ps.Obter(amb.Ctx, conv); ok {
		t.Fatal("pausa vencida continua valendo")
	}
	var n int
	amb.Banco.L().QueryRow(`SELECT count(*) FROM pausas_conversa`).Scan(&n)
	if n != 0 {
		t.Fatal("pausa vencida não foi apagada")
	}
	evs := amb.EventosDoTipo(eventos.ConversaPausa)
	ult, _ := json.Marshal(evs[len(evs)-1].Dados)
	if string(ult) != `{"conversa_id":"`+conv+`","pausa":null}` {
		t.Fatalf("evento de vencimento: %s", ult)
	}

	// Assumir: sem prazo; mensagem manual depois não encurta.
	ps.Assumir(amb.Ctx, conv, dominio.PausaHumano, nil)
	ps.PausarHumano(amb.Ctx, conv)
	pz, _ = ps.Obter(amb.Ctx, conv)
	if pz.Ate != nil || pz.Motivo != dominio.PausaHumano {
		t.Fatalf("assumir sem prazo: %+v", pz)
	}
	amb.Relogio.Avancar(1000 * time.Hour)
	if _, ok := ps.Obter(amb.Ctx, conv); !ok {
		t.Fatal("assumir sem prazo venceu")
	}
	ps.Retomar(amb.Ctx, conv)
	if _, ok := ps.Obter(amb.Ctx, conv); ok {
		t.Fatal("retomar não removeu")
	}
	// Pausa anti-loop mais longa não é substituída por uma humana mais curta.
	ps.PausarAntiLoop(amb.Ctx, conv, "a1", 60)
	ps.PausarHumano(amb.Ctx, conv)
	pz, _ = ps.Obter(amb.Ctx, conv)
	if pz.Motivo != dominio.PausaAntiLoop {
		t.Fatalf("pausa mais longa substituída: %+v", pz)
	}
	// Assumir com prazo explícito substitui sempre.
	d := 5
	ps.Assumir(amb.Ctx, conv, dominio.PausaManual, &d)
	pz, _ = ps.Obter(amb.Ctx, conv)
	if pz.Motivo != dominio.PausaManual || !pz.Ate.Equal(amb.Relogio.Agora().Add(5*time.Minute)) {
		t.Fatalf("assumir com prazo: %+v", pz)
	}
}
