package integracao

import (
	"strings"
	"testing"
)

func TestConversasETexto(t *testing.T) {
	m := novoMotor(t)
	c := m.contaConectada("+5511900000001")

	// mensagem recebida cria conversa com nao_lidas=1
	marca := m.marca()
	m.receber(c.ID, map[string]any{"de": "(11) 91111-2222", "texto": "oi, tudo bem?", "nome": "Ana"})
	convs := m.conversas(c.ID, "")
	if len(convs) != 1 {
		t.Fatalf("conversas: %+v", convs)
	}
	cv := convs[0]
	if cv.NaoLidas != 1 || cv.Tipo != "individual" || ptr(cv.Telefone) != "+5511911112222" || cv.Nome != "Ana" ||
		ptr(cv.UltimaMensagemResumo) != "oi, tudo bem?" || cv.ContatoID == nil || cv.Etiquetas == nil {
		t.Fatalf("conversa: %+v", cv)
	}
	m.esperarEvento(marca, "mensagem.nova", nil)
	m.esperarEvento(marca, "conversa.atualizada", nil)
	m.esperarEvento(marca, "contato.atualizado", nil)
	msgs := m.mensagens(cv.ID)
	if len(msgs) != 1 || msgs[0].Estado != "recebida" || msgs[0].DeMim || ptr(msgs[0].Texto) != "oi, tudo bem?" {
		t.Fatalf("mensagens: %+v", msgs)
	}
	var unica Conversa
	m.json("GET", "/v1/conversas/"+cv.ID, nil, 200, &unica)
	if unica.ID != cv.ID {
		t.Fatal("GET conversa")
	}
	if len(m.conversas(c.ID, "?nao_lidas=true")) != 1 || len(m.conversas(c.ID, "?busca=ana")) != 1 || len(m.conversas(c.ID, "?busca=zzz")) != 0 {
		t.Fatal("filtros de conversa")
	}

	// marcar como lida
	if st, _ := m.req("POST", "/v1/conversas/"+cv.ID+"/lida", nil); st != 204 {
		t.Fatalf("lida: %d", st)
	}
	if m.conversas(c.ID, "")[0].NaoLidas != 0 || len(m.conversas(c.ID, "?nao_lidas=true")) != 0 {
		t.Fatal("nao_lidas deveria zerar")
	}

	// envio de texto: pendente → enviada
	marca = m.marca()
	var env Mensagem
	m.json("POST", "/v1/conversas/"+cv.ID+"/mensagens", map[string]any{"texto": "Olá, Ana!"}, 202, &env)
	if env.Estado != "pendente" || !env.DeMim || ptr(env.Texto) != "Olá, Ana!" {
		t.Fatalf("envio: %+v", env)
	}
	enviada := m.esperarMensagem(marca, env.ID, "enviada")
	if strings.HasPrefix(enviada.WaID, "pendente") || !enviada.PodeEditar || !enviada.PodeApagar {
		t.Fatalf("enviada: %+v", enviada)
	}
	var fal []map[string]any
	m.json("GET", "/v1/falso/enviadas", nil, 200, &fal)
	if len(fal) != 1 || fal[0]["telefone"] != "+5511911112222" || fal[0]["texto"] != "Olá, Ana!" || fal[0]["wa_id"] != enviada.WaID {
		t.Fatalf("falso/enviadas: %v", fal)
	}
	m.erro("POST", "/v1/conversas/"+cv.ID+"/mensagens", map[string]any{"texto": "  "}, 422, "validacao")

	// recibos fora de ordem não regridem
	m.json("POST", "/v1/falso/contas/"+c.ID+"/recibo", map[string]string{"wa_id": enviada.WaID, "tipo": "lido"}, 200, nil)
	m.json("POST", "/v1/falso/contas/"+c.ID+"/recibo", map[string]string{"wa_id": enviada.WaID, "tipo": "entregue"}, 200, nil)
	for _, msg := range m.mensagens(cv.ID) {
		if msg.ID == env.ID && msg.Estado != "lida" {
			t.Fatalf("recibo regrediu: %s", msg.Estado)
		}
	}
	m.erro("POST", "/v1/mensagens/"+env.ID+"/reenviar", nil, 409, "transicao_invalida")

	// grupo
	m.receber(c.ID, map[string]any{"de": "+5511933334444", "texto": "bom dia grupo", "grupo_jid": "120363@g.us", "grupo_nome": "Família", "nome": "Beto"})
	var grupo Conversa
	for _, x := range m.conversas(c.ID, "") {
		if x.Tipo == "grupo" {
			grupo = x
		}
	}
	if grupo.Nome != "Família" || grupo.JID != "120363@g.us" || grupo.Telefone != nil {
		t.Fatalf("grupo: %+v", grupo)
	}
	gm := m.mensagens(grupo.ID)
	if len(gm) != 1 || ptr(gm[0].RemetenteNome) != "Beto" || gm[0].RemetenteJID != "5511933334444@s.whatsapp.net" {
		t.Fatalf("mensagem de grupo: %+v", gm)
	}
	// conversa mais recente primeiro
	if m.conversas(c.ID, "")[0].ID != grupo.ID {
		t.Fatal("ordem por última mensagem")
	}

	// nova conversa por telefone
	var nova Conversa
	m.json("POST", "/v1/contas/"+c.ID+"/conversas", map[string]string{"telefone": "(11) 97777-0000"}, 201, &nova)
	if nova.JID != "5511977770000@s.whatsapp.net" || ptr(nova.Telefone) != "+5511977770000" {
		t.Fatalf("nova conversa: %+v", nova)
	}
	var mesma Conversa
	m.json("POST", "/v1/contas/"+c.ID+"/conversas", map[string]string{"telefone": "+55 11 97777-0000"}, 200, &mesma)
	if mesma.ID != nova.ID {
		t.Fatal("deveria reaproveitar a conversa")
	}
	m.json("PUT", "/v1/falso/numeros-sem-whatsapp", map[string]any{"telefones": []string{"+5511966660000"}}, 200, nil)
	e := m.erro("POST", "/v1/contas/"+c.ID+"/conversas", map[string]string{"telefone": "11 96666-0000"}, 422, "sem_whatsapp")
	if e["mensagem"] != "Este número não tem WhatsApp" && e["mensagem"] != "Este número não tem WhatsApp." {
		t.Fatalf("copy sem_whatsapp: %v", e)
	}
	m.erro("POST", "/v1/contas/"+c.ID+"/conversas", map[string]string{"telefone": "abc"}, 422, "validacao")

	// falha de envio → falhou → reenviar → enviada
	m.json("PUT", "/v1/falso/falhas-envio", map[string]any{"telefones": []string{"+5511977770000"}, "erro": "servidor recusou"}, 200, nil)
	marca = m.marca()
	var f Mensagem
	m.json("POST", "/v1/conversas/"+nova.ID+"/mensagens", map[string]any{"texto": "vai falhar"}, 202, &f)
	falhou := m.esperarMensagem(marca, f.ID, "falhou")
	if falhou.Erro == nil || *falhou.Erro == "" {
		t.Fatalf("falhou sem erro: %+v", falhou)
	}
	m.json("PUT", "/v1/falso/falhas-envio", map[string]any{"telefones": []string{}}, 200, nil)
	marca = m.marca()
	var re Mensagem
	m.json("POST", "/v1/mensagens/"+f.ID+"/reenviar", nil, 202, &re)
	if re.Estado != "pendente" || re.ID != f.ID {
		t.Fatalf("reenviar: %+v", re)
	}
	m.esperarMensagem(marca, f.ID, "enviada")

	// conta desconectada → conta_indisponivel
	m.json("POST", "/v1/falso/contas/"+c.ID+"/estado", map[string]string{"evento": "logout"}, 200, nil)
	m.erro("POST", "/v1/conversas/"+cv.ID+"/mensagens", map[string]any{"texto": "x"}, 409, "conta_indisponivel")
	m.erro("POST", "/v1/contas/"+c.ID+"/conversas", map[string]string{"telefone": "11 95555-0000"}, 409, "conta_indisponivel")

	m.erro("GET", "/v1/conversas/inexistente", nil, 404, "nao_encontrado")
	m.erro("GET", "/v1/conversas/inexistente/mensagens", nil, 404, "nao_encontrado")
}

func TestMensagemDoProprioCelularEContatos(t *testing.T) {
	m := novoMotor(t)
	c := m.contaConectada("+5511900000001")
	m.receber(c.ID, map[string]any{"de": "+5511922223333", "texto": "enviei pelo celular", "de_mim": true})
	convs := m.conversas(c.ID, "")
	if len(convs) != 1 || convs[0].NaoLidas != 0 {
		t.Fatalf("mensagem própria não conta como não lida: %+v", convs)
	}
	msgs := m.mensagens(convs[0].ID)
	if !msgs[0].DeMim || msgs[0].Estado != "enviada" {
		t.Fatalf("de_mim: %+v", msgs[0])
	}

	m.receber(c.ID, map[string]any{"de": "+5511944445555", "texto": "oi", "nome": "Carla"})
	var p Pagina[Contato]
	m.json("GET", "/v1/contas/"+c.ID+"/contatos", nil, 200, &p)
	if len(p.Itens) != 2 {
		t.Fatalf("contatos: %+v", p.Itens)
	}
	m.json("GET", "/v1/contas/"+c.ID+"/contatos?busca=carla", nil, 200, &p)
	if len(p.Itens) != 1 || ptr(p.Itens[0].NomePush) != "Carla" || p.Itens[0].ConversaID == nil || p.Itens[0].Etiquetas == nil {
		t.Fatalf("busca contato: %+v", p.Itens)
	}
	var ct Contato
	m.json("GET", "/v1/contatos/"+p.Itens[0].ID, nil, 200, &ct)
	if ct.ID != p.Itens[0].ID || ptr(ct.Telefone) != "+5511944445555" {
		t.Fatalf("contato: %+v", ct)
	}
	m.erro("GET", "/v1/contatos/x", nil, 404, "nao_encontrado")
}
