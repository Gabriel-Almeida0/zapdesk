package integracao

import (
	"context"
	"encoding/csv"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

type Disparo struct {
	ID          string  `json:"id"`
	ContaID     string  `json:"conta_id"`
	Nome        string  `json:"nome"`
	Mensagem    string  `json:"mensagem"`
	Estado      string  `json:"estado"`
	NaFila      bool    `json:"na_fila"`
	MotivoPausa *string `json:"motivo_pausa"`
	Origem      string  `json:"origem"`
	Contadores  struct {
		Total, Pendente, Enviando, Enviado, Entregue, Lido, Respondeu, Falhou int
	} `json:"contadores"`
	ProximoEnvioEm      *string    `json:"proximo_envio_em"`
	EstimativaTerminoEm *string    `json:"estimativa_termino_em"`
	AvisoRitmoAgressivo bool       `json:"aviso_ritmo_agressivo"`
	IniciadoEm          *string    `json:"iniciado_em"`
	RelatorioImportacao *Relatorio `json:"relatorio_importacao"`
}

func (m *Motor) ocioso() {
	m.t.Helper()
	ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelar()
	if err := m.App.Servicos().Disparos.AguardarOcioso(ctx); err != nil {
		m.t.Fatal("executor não ficou ocioso")
	}
}

func (m *Motor) avancar(d time.Duration) {
	m.t.Helper()
	m.json("PUT", "/v1/falso/relogio", map[string]any{"avancar_s": d.Seconds()}, 200, nil)
	m.ocioso()
}

func (m *Motor) disparo(id string) Disparo {
	m.t.Helper()
	var d Disparo
	m.json("GET", "/v1/disparos/"+id, nil, 200, &d)
	return d
}

func colar(n, base int) string {
	var l []string
	for i := 0; i < n; i++ {
		l = append(l, fmt.Sprintf("11 9%04d-%04d", base, i))
	}
	return strings.Join(l, "\n")
}

type enviadaFalsa struct {
	Telefone string `json:"telefone"`
	Tipo     string `json:"tipo"`
	Texto    string `json:"texto"`
	Em       string `json:"em"`
}

func (m *Motor) enviadas() []enviadaFalsa {
	var e []enviadaFalsa
	m.json("GET", "/v1/falso/enviadas", nil, 200, &e)
	return e
}

func TestDisparoValidarCriarETransicoes(t *testing.T) {
	m := novoMotor(t)
	c := m.contaConectada("+5511900000001")
	var rel Relatorio
	m.json("POST", "/v1/leads/importar", map[string]any{"leads": []map[string]any{
		{"telefone": "11 91111-0001", "nome": "Ana", "campos": map[string]string{"cidade": "Recife"}},
		{"telefone": "11 91111-0002", "nome": "Beto"},
		{"telefone": "11 91111-0003"},
	}}, 200, &rel)

	base := map[string]any{
		"conta_id":      c.ID,
		"mensagem":      "Oi {nome}, tudo bem em {cidade}?",
		"destinatarios": map[string]any{"lead_ids": rel.LeadIDs},
		"ritmo":         map[string]any{"intervalo_min_s": 5, "intervalo_max_s": 10},
	}
	var v struct {
		TotalDestinatarios int      `json:"total_destinatarios"`
		Variaveis          []string `json:"variaveis"`
		Faltando           []struct {
			DestinatarioLinha int      `json:"destinatario_linha"`
			Telefone          string   `json:"telefone"`
			Variaveis         []string `json:"variaveis"`
		} `json:"faltando"`
		Previa *struct {
			TextoResolvido string `json:"texto_resolvido"`
		} `json:"previa"`
		EstimativaTerminoEm *string           `json:"estimativa_termino_em"`
		AvisoRitmoAgressivo bool              `json:"aviso_ritmo_agressivo"`
		ErrosCampos         map[string]string `json:"erros_campos"`
	}
	m.json("POST", "/v1/disparos/validar", base, 200, &v)
	if v.TotalDestinatarios != 3 || len(v.Variaveis) != 2 || len(v.Faltando) != 2 || v.Faltando[0].DestinatarioLinha != 2 ||
		v.Previa == nil || v.Previa.TextoResolvido != "Oi Ana, tudo bem em Recife?" || !v.AvisoRitmoAgressivo || v.EstimativaTerminoEm == nil || len(v.ErrosCampos) != 0 {
		t.Fatalf("validar: %+v", v)
	}
	ruim := map[string]any{"conta_id": c.ID, "mensagem": "", "destinatarios": map[string]any{}, "ritmo": map[string]any{"intervalo_min_s": 0, "intervalo_max_s": 0}, "janela": map[string]any{"inicio": "09:00"}}
	m.json("POST", "/v1/disparos/validar", ruim, 200, &v)
	for _, campo := range []string{"mensagem", "ritmo.intervalo_min_s", "janela", "destinatarios"} {
		if v.ErrosCampos[campo] == "" {
			t.Fatalf("validar deveria apontar %s: %v", campo, v.ErrosCampos)
		}
	}

	// iniciar=true com variáveis faltando → 422 e nada é criado
	iniciar := map[string]any{}
	for k, x := range base {
		iniciar[k] = x
	}
	iniciar["iniciar"] = true
	e := m.erro("POST", "/v1/disparos", iniciar, 422, "variaveis_faltando")
	det := e["detalhes"].(map[string]any)
	if det["total"] != float64(2) || len(det["faltando"].([]any)) != 2 || !strings.Contains(e["mensagem"].(string), "{cidade}") {
		t.Fatalf("variaveis_faltando: %v", e)
	}
	var pg Pagina[Disparo]
	m.json("GET", "/v1/disparos", nil, 200, &pg)
	if len(pg.Itens) != 0 {
		t.Fatal("não deveria ter criado disparo")
	}

	// rascunho → PATCH → iniciar com valores padrão
	var d Disparo
	m.json("POST", "/v1/disparos", base, 201, &d)
	if d.Estado != "rascunho" || d.Contadores.Total != 3 || d.Origem != "app" || !strings.HasPrefix(d.Nome, "Disparo ") {
		t.Fatalf("rascunho: %+v", d)
	}
	m.json("PATCH", "/v1/disparos/"+d.ID, map[string]any{"nome": "Setembro", "ritmo": map[string]any{"intervalo_min_s": 30, "intervalo_max_s": 60, "limite_por_hora": 40}}, 200, &d)
	if d.Nome != "Setembro" || d.AvisoRitmoAgressivo {
		t.Fatalf("patch: %+v", d)
	}
	m.erro("PATCH", "/v1/disparos/"+d.ID, map[string]any{"ritmo": map[string]any{"intervalo_min_s": 10, "intervalo_max_s": 5}}, 422, "validacao")
	m.erro("POST", "/v1/disparos/"+d.ID+"/pausar", nil, 409, "transicao_invalida")
	m.erro("POST", "/v1/disparos/"+d.ID+"/retomar", nil, 409, "transicao_invalida")
	m.erro("POST", "/v1/disparos/"+d.ID+"/iniciar", map[string]any{}, 422, "variaveis_faltando")
	marca := m.marca()
	m.json("POST", "/v1/disparos/"+d.ID+"/iniciar", map[string]any{"valores_padrao": map[string]string{"cidade": "sua cidade", "nome": "tudo bem"}}, 200, &d)
	m.ocioso()
	d = m.disparo(d.ID)
	if d.Estado != "enviando" || d.Contadores.Enviado != 1 || d.IniciadoEm == nil || d.ProximoEnvioEm == nil || d.EstimativaTerminoEm == nil {
		t.Fatalf("iniciado: %+v", d)
	}
	ativos := m.esperarEvento(marca, "disparos.ativos", nil)
	if dados[map[string]int](t, ativos)["total"] != 1 {
		t.Fatalf("disparos.ativos: %s", ativos.Dados)
	}
	m.esperarEvento(marca, "destinatario.atualizado", nil)
	m.erro("PATCH", "/v1/disparos/"+d.ID, map[string]any{"nome": "x"}, 409, "transicao_invalida")
	m.erro("DELETE", "/v1/disparos/"+d.ID, nil, 409, "transicao_invalida")

	m.json("POST", "/v1/disparos/"+d.ID+"/pausar", nil, 200, &d)
	if d.Estado != "pausado" || ptr(d.MotivoPausa) != "usuario" {
		t.Fatalf("pausar: %+v", d)
	}
	m.json("POST", "/v1/disparos/"+d.ID+"/retomar", nil, 200, &d)
	if d.Estado != "agendado" {
		t.Fatalf("retomar: %+v", d)
	}
	m.json("POST", "/v1/disparos/"+d.ID+"/cancelar", nil, 200, &d)
	if d.Estado != "cancelado" {
		t.Fatalf("cancelar: %+v", d)
	}
	fim := m.esperarEvento(marca, "disparo.finalizado", nil)
	if !strings.HasPrefix(dados[map[string]any](t, fim)["resumo"].(string), "Disparo cancelado:") {
		t.Fatalf("finalizado: %s", fim.Dados)
	}
	m.erro("POST", "/v1/disparos/"+d.ID+"/cancelar", nil, 409, "transicao_invalida")

	// rascunho pode ser excluído
	var r2 Disparo
	m.json("POST", "/v1/disparos", base, 201, &r2)
	if st, _ := m.req("DELETE", "/v1/disparos/"+r2.ID, nil); st != 204 {
		t.Fatal("excluir rascunho")
	}
	m.erro("GET", "/v1/disparos/"+r2.ID, nil, 404, "nao_encontrado")
	m.erro("POST", "/v1/disparos", map[string]any{"conta_id": "x", "mensagem": "a", "destinatarios": map[string]any{"lead_ids": rel.LeadIDs},
		"ritmo": map[string]any{"intervalo_min_s": 5, "intervalo_max_s": 5}}, 404, "nao_encontrado")
	m.erro("POST", "/v1/disparos", map[string]any{"conta_id": c.ID, "mensagem": "a", "destinatarios": map[string]any{"lead_ids": []string{"x"}},
		"ritmo": map[string]any{"intervalo_min_s": 5, "intervalo_max_s": 5}}, 422, "validacao")
}

func TestDisparoIdempotenteAoReiniciar(t *testing.T) {
	m := novoMotor(t)
	c := m.contaConectada("+5511900000001")
	var d Disparo
	m.json("POST", "/v1/disparos", map[string]any{
		"conta_id": c.ID, "mensagem": "Promo", "origem": "mcp", "iniciar": true,
		"destinatarios": map[string]any{"importar": map[string]any{"texto_colado": colar(20, 1234) + "\n11 91234-0000"}},
		"ritmo":         map[string]any{"intervalo_min_s": 1, "intervalo_max_s": 1},
	}, 201, &d)
	if d.RelatorioImportacao == nil || d.RelatorioImportacao.TotalNovos != 20 || d.RelatorioImportacao.TotalDuplicadosNoLote != 1 || d.Origem != "mcp" {
		t.Fatalf("criar com importação: %+v", d.RelatorioImportacao)
	}
	m.ocioso()
	for i := 0; i < 7; i++ {
		m.avancar(time.Second)
	}
	if d = m.disparo(d.ID); d.Contadores.Enviado != 8 {
		t.Fatalf("antes de reiniciar: %+v", d.Contadores)
	}

	m.reiniciar()
	d = m.disparo(d.ID)
	if d.Estado != "pausado" || ptr(d.MotivoPausa) != "app_fechado" {
		t.Fatalf("após reiniciar: %s %v", d.Estado, ptr(d.MotivoPausa))
	}
	m.esperarEvento(0, "conta.atualizada", func(e Evento) bool { return dados[Conta](t, e).Estado == "conectada" })
	m.json("POST", "/v1/disparos/"+d.ID+"/retomar", nil, 200, &d)
	m.ocioso()
	for i := 0; i < 20; i++ {
		m.avancar(time.Second)
	}
	d = m.disparo(d.ID)
	if d.Estado != "concluido" || d.Contadores.Enviado != 20 {
		t.Fatalf("concluído: %s %+v", d.Estado, d.Contadores)
	}
	vistos := map[string]int{}
	for _, e := range m.enviadas() {
		vistos[e.Telefone]++
	}
	if len(vistos) != 20 {
		t.Fatalf("esperava 20 telefones: %d", len(vistos))
	}
	for tel, n := range vistos {
		if n != 1 {
			t.Fatalf("telefone repetido %s: %d envios", tel, n)
		}
	}
	// religado → motor_reiniciado
	var d2 Disparo
	m.json("POST", "/v1/disparos", map[string]any{"conta_id": c.ID, "mensagem": "x", "iniciar": true,
		"destinatarios": map[string]any{"importar": map[string]any{"texto_colado": colar(3, 4321)}},
		"ritmo":         map[string]any{"intervalo_min_s": 60, "intervalo_max_s": 60}}, 201, &d2)
	m.ocioso()
	m.parar()
	m.o.religado = true
	m.subir()
	if d2 = m.disparo(d2.ID); ptr(d2.MotivoPausa) != "app_fechado" {
		// encerramento gracioso grava app_fechado; o --religado vale para quedas (sem encerramento)
		t.Fatalf("encerramento gracioso: %v", ptr(d2.MotivoPausa))
	}
}

func TestDisparoJanelaELimitePorHora(t *testing.T) {
	m := novoMotor(t)
	c := m.contaConectada("+5511900000001")
	m.json("PUT", "/v1/falso/relogio", map[string]any{"agora": time.Date(2026, 9, 28, 17, 50, 0, 0, time.Local).Format(time.RFC3339)}, 200, nil)
	var d Disparo
	m.json("POST", "/v1/disparos", map[string]any{
		"conta_id": c.ID, "mensagem": "Oi", "iniciar": true,
		"destinatarios": map[string]any{"importar": map[string]any{"texto_colado": colar(12, 5555)}},
		"ritmo":         map[string]any{"intervalo_min_s": 60, "intervalo_max_s": 60, "limite_por_hora": 4},
		"janela":        map[string]any{"inicio": "09:00", "fim": "18:00"},
	}, 201, &d)
	m.ocioso()
	// anda o relógio minuto a minuto por ~20 h
	for i := 0; i < 20*60; i++ {
		m.avancar(time.Minute)
		if i%60 == 0 && m.disparo(d.ID).Estado == "concluido" {
			break
		}
	}
	d = m.disparo(d.ID)
	if d.Estado != "concluido" {
		t.Fatalf("deveria concluir no dia seguinte: %s %+v", d.Estado, d.Contadores)
	}
	var horarios []time.Time
	for _, e := range m.enviadas() {
		h, _ := parseRFC(e.Em)
		h = h.Local()
		if h.Hour() < 9 || h.Hour() >= 18 {
			t.Fatalf("envio fora da janela: %v", h)
		}
		horarios = append(horarios, h)
	}
	sort.Slice(horarios, func(i, j int) bool { return horarios[i].Before(horarios[j]) })
	for i := range horarios {
		n := 0
		for _, x := range horarios {
			if !x.Before(horarios[i]) && x.Before(horarios[i].Add(time.Hour)) {
				n++
			}
		}
		if n > 4 {
			t.Fatalf("mais de 4 envios em 60 min a partir de %v", horarios[i])
		}
	}
	if len(horarios) != 12 {
		t.Fatalf("envios: %d", len(horarios))
	}
	if horarios[0].Day() != 28 || horarios[len(horarios)-1].Day() != 29 {
		t.Fatalf("parte hoje (antes das 18h) e o resto amanhã: %v … %v", horarios[0], horarios[len(horarios)-1])
	}

	// relatório CSV com BOM, colunas do contrato e variáveis
	st, b := m.req("GET", "/v1/disparos/"+d.ID+"/relatorio.csv", nil)
	if st != 200 || !strings.HasPrefix(string(b), "\ufeff") {
		t.Fatalf("csv: %d %q", st, b[:10])
	}
	linhas, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(b), "\ufeff"))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(linhas[0], ",") != "telefone,nome,estado,motivo_falha,enviando_em,enviado_em,entregue_em,lido_em,respondeu_em,falhou_em" || len(linhas) != 13 {
		t.Fatalf("cabeçalho/linhas: %v %d", linhas[0], len(linhas))
	}
	st, _ = m.req("GET", "/v1/disparos/"+d.ID+"/relatorio.csv?token="+tokenTeste, nil, "Authorization", "")
	if st != 200 {
		t.Fatalf("csv com ?token=: %d", st)
	}
	var dp Pagina[struct {
		Estado string `json:"estado"`
		Ordem  int    `json:"ordem"`
	}]
	m.json("GET", "/v1/disparos/"+d.ID+"/destinatarios?limite=5", nil, 200, &dp)
	if len(dp.Itens) != 5 || dp.Itens[0].Ordem != 1 || dp.ProximoCursor == nil {
		t.Fatalf("destinatários: %+v", dp)
	}
	m.json("GET", "/v1/disparos/"+d.ID+"/destinatarios?limite=5&cursor="+*dp.ProximoCursor, nil, 200, &dp)
	if dp.Itens[0].Ordem != 6 {
		t.Fatalf("paginação por ordem: %+v", dp.Itens[0])
	}
}
