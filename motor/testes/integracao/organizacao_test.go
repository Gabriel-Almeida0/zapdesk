package integracao

import (
	"strings"
	"testing"
)

type Template struct {
	ID        string   `json:"id"`
	Nome      string   `json:"nome"`
	Texto     string   `json:"texto"`
	Variaveis []string `json:"variaveis"`
	Arquivo   *Arquivo `json:"arquivo"`
}

func TestEtiquetasNotasETemplates(t *testing.T) {
	m := novoMotor(t)
	c := m.contaConectada("+5511900000001")
	m.receber(c.ID, map[string]any{"de": "+5511944445555", "texto": "oi", "nome": "Carla"})
	m.receber(c.ID, map[string]any{"de": "+5511933332222", "texto": "oi", "nome": "Davi"})
	var cts Pagina[Contato]
	m.json("GET", "/v1/contas/"+c.ID+"/contatos", nil, 200, &cts)
	carla := cts.Itens[0]
	if ptr(carla.NomePush) != "Carla" {
		carla = cts.Itens[1]
	}

	// etiquetas
	marca := m.marca()
	var quente Etiqueta
	m.json("POST", "/v1/etiquetas", map[string]string{"nome": "Quente", "cor": "#FF5500"}, 201, &quente)
	m.esperarEvento(marca, "etiquetas.alteradas", nil)
	m.erro("POST", "/v1/etiquetas", map[string]string{"nome": "quente", "cor": "#000000"}, 409, "conflito")
	m.erro("POST", "/v1/etiquetas", map[string]string{"nome": "", "cor": "#000000"}, 422, "validacao")
	m.erro("POST", "/v1/etiquetas", map[string]string{"nome": strings.Repeat("a", 31), "cor": "#000000"}, 422, "validacao")
	m.erro("POST", "/v1/etiquetas", map[string]string{"nome": "Fria", "cor": "azul"}, 422, "validacao")
	var fria Etiqueta
	m.json("POST", "/v1/etiquetas", map[string]string{"nome": "Fria", "cor": "#0000ff"}, 201, &fria)
	m.erro("PATCH", "/v1/etiquetas/"+fria.ID, map[string]string{"nome": "QUENTE"}, 409, "conflito")
	m.json("PATCH", "/v1/etiquetas/"+fria.ID, map[string]string{"nome": "Morna", "cor": "#00FF00"}, 200, &fria)
	if fria.Nome != "Morna" || fria.Cor != "#00FF00" {
		t.Fatalf("editar etiqueta: %+v", fria)
	}

	// etiquetas no contato e filtros
	marca = m.marca()
	var ct Contato
	m.json("PUT", "/v1/contatos/"+carla.ID+"/etiquetas", map[string]any{"etiqueta_ids": []string{quente.ID, fria.ID}}, 200, &ct)
	if len(ct.Etiquetas) != 2 {
		t.Fatalf("etiquetas do contato: %+v", ct.Etiquetas)
	}
	m.esperarEvento(marca, "contato.atualizado", nil)
	m.json("PUT", "/v1/contatos/"+carla.ID+"/etiquetas", map[string]any{"etiqueta_ids": []string{quente.ID}}, 200, &ct)
	if len(ct.Etiquetas) != 1 || ct.Etiquetas[0].ID != quente.ID || ct.Etiquetas[0].TotalContatos != 1 {
		t.Fatalf("substitui o conjunto: %+v", ct.Etiquetas)
	}
	m.erro("PUT", "/v1/contatos/"+carla.ID+"/etiquetas", map[string]any{"etiqueta_ids": []string{"x"}}, 422, "validacao")
	var lista []Etiqueta
	m.json("GET", "/v1/etiquetas", nil, 200, &lista)
	if len(lista) != 2 || lista[0].Nome != "Morna" || lista[1].TotalContatos != 1 {
		t.Fatalf("listar etiquetas: %+v", lista)
	}
	if convs := m.conversas(c.ID, "?etiqueta_id="+quente.ID); len(convs) != 1 || len(convs[0].Etiquetas) != 1 {
		t.Fatalf("filtro de conversas por etiqueta: %+v", convs)
	}
	m.json("GET", "/v1/contas/"+c.ID+"/contatos?etiqueta_id="+quente.ID, nil, 200, &cts)
	if len(cts.Itens) != 1 || cts.Itens[0].ID != carla.ID {
		t.Fatalf("filtro de contatos por etiqueta: %+v", cts.Itens)
	}

	// notas
	m.json("PATCH", "/v1/contatos/"+carla.ID, map[string]string{"notas": "Cliente desde 2020"}, 200, &ct)
	if ptr(ct.Notas) != "Cliente desde 2020" {
		t.Fatalf("notas: %+v", ct)
	}
	m.erro("PATCH", "/v1/contatos/"+carla.ID, map[string]string{"notas": strings.Repeat("x", 10001)}, 422, "validacao")
	m.erro("PATCH", "/v1/contatos/nada", map[string]string{"notas": "x"}, 404, "nao_encontrado")

	// excluir etiqueta remove dos contatos
	if st, _ := m.req("DELETE", "/v1/etiquetas/"+quente.ID, nil); st != 204 {
		t.Fatal("excluir etiqueta")
	}
	m.json("GET", "/v1/contatos/"+carla.ID, nil, 200, &ct)
	if len(ct.Etiquetas) != 0 {
		t.Fatal("etiqueta excluída ainda no contato")
	}
	m.erro("DELETE", "/v1/etiquetas/"+quente.ID, nil, 404, "nao_encontrado")

	// templates
	marca = m.marca()
	anexo := m.upload("catalogo.pdf", []byte("%PDF-1.4 catálogo"))
	var tpl Template
	m.json("POST", "/v1/templates", map[string]any{"nome": "Boas-vindas", "texto": "Oi {nome}, veja {Cidade}!", "arquivo_id": anexo.ID}, 201, &tpl)
	if len(tpl.Variaveis) != 2 || tpl.Variaveis[1] != "cidade" || tpl.Arquivo == nil || tpl.Arquivo.ID != anexo.ID {
		t.Fatalf("template: %+v", tpl)
	}
	m.esperarEvento(marca, "templates.alterados", nil)
	m.erro("POST", "/v1/templates", map[string]any{"nome": "boas-vindas", "texto": "x"}, 409, "conflito")
	m.erro("POST", "/v1/templates", map[string]any{"nome": strings.Repeat("n", 61), "texto": "x"}, 422, "validacao")
	m.erro("POST", "/v1/templates", map[string]any{"nome": "Vazio", "texto": ""}, 422, "validacao")
	m.erro("POST", "/v1/templates", map[string]any{"nome": "Longo", "texto": strings.Repeat("t", 4097)}, 422, "validacao")
	m.erro("POST", "/v1/templates", map[string]any{"nome": "Anexo", "texto": "x", "arquivo_id": "nada"}, 404, "nao_encontrado")
	var outro Template
	m.json("POST", "/v1/templates", map[string]any{"nome": "Cobrança", "texto": "Lembrete"}, 201, &outro)
	var tpls []Template
	m.json("GET", "/v1/templates?busca=boas", nil, 200, &tpls)
	if len(tpls) != 1 || tpls[0].ID != tpl.ID {
		t.Fatalf("busca de templates: %+v", tpls)
	}
	m.json("PATCH", "/v1/templates/"+tpl.ID, map[string]any{"texto": "Olá {empresa}", "arquivo_id": nil}, 200, &tpl)
	if tpl.Arquivo != nil || len(tpl.Variaveis) != 1 || tpl.Variaveis[0] != "empresa" || tpl.Nome != "Boas-vindas" {
		t.Fatalf("editar template: %+v", tpl)
	}
	m.erro("PATCH", "/v1/templates/"+tpl.ID, map[string]any{"nome": "cobrança"}, 409, "conflito")
	m.json("GET", "/v1/templates/"+outro.ID, nil, 200, &outro)
	if st, _ := m.req("DELETE", "/v1/templates/"+outro.ID, nil); st != 204 {
		t.Fatal("excluir template")
	}
	m.erro("GET", "/v1/templates/"+outro.ID, nil, 404, "nao_encontrado")

	// disparo a partir de template copia texto
	var rel Relatorio
	m.json("POST", "/v1/leads/importar", map[string]any{"leads": []map[string]any{{"telefone": "11 95555-0000", "campos": map[string]string{"empresa": "ACME"}}}}, 200, &rel)
	var d Disparo
	m.json("POST", "/v1/disparos", map[string]any{"conta_id": c.ID, "template_id": tpl.ID, "destinatarios": map[string]any{"lead_ids": rel.LeadIDs},
		"ritmo": map[string]any{"intervalo_min_s": 30, "intervalo_max_s": 60}}, 201, &d)
	if d.Mensagem != "Olá {empresa}" {
		t.Fatalf("disparo com template: %+v", d)
	}
}
