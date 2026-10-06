package integracao

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"zapdesk/motor/internal/api"
	"zapdesk/motor/internal/logs"
)

func TestBindSoEmLoopback(t *testing.T) {
	s := api.Novo(api.Opcoes{Token: tokenTeste})
	if err := s.Escutar(0); err != nil {
		t.Fatal(err)
	}
	go s.Servir()
	defer s.Encerrar(t.Context())
	porta := s.Porta()
	// responde em 127.0.0.1
	c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", itoa(porta)), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	// não responde nas interfaces externas
	ifaces, _ := net.InterfaceAddrs()
	for _, a := range ifaces {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() || ipnet.IP.To4() == nil {
			continue
		}
		if c, err := net.DialTimeout("tcp", net.JoinHostPort(ipnet.IP.String(), itoa(porta)), 300*time.Millisecond); err == nil {
			c.Close()
			t.Fatalf("motor acessível em %s", ipnet.IP)
		}
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestHostETokenSemprePedidos(t *testing.T) {
	m := novoMotor(t)
	rotas := []string{"/v1/saude", "/v1/contas", "/v1/leads", "/v1/disparos", "/v1/etiquetas", "/v1/eventos", "/v1/falso/enviadas", "/v1/qualquer"}
	for _, r := range rotas {
		req, _ := http.NewRequest("GET", m.url(r), nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Fatalf("%s sem token: %d", r, resp.StatusCode)
		}
		req, _ = http.NewRequest("GET", m.url(r), nil)
		req.Header.Set("Authorization", "Bearer "+tokenTeste[:len(tokenTeste)-1])
		resp, _ = http.DefaultClient.Do(req)
		resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Fatalf("%s com token quase certo: %d", r, resp.StatusCode)
		}
		if st, _ := m.req("GET", r, nil, "Host", "zapdesk.attacker.com"); st != 403 {
			t.Fatalf("%s com Host de DNS rebinding: %d", r, st)
		}
	}
	// ?token= em rota que não é binária/WS não vale, nem em POST
	req, _ := http.NewRequest("POST", m.url("/v1/contas?token="+tokenTeste), nil)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("POST com ?token=: %d", resp.StatusCode)
	}
}

func TestLogSemConteudoDeMensagens(t *testing.T) {
	pasta := t.TempDir()
	l, err := logs.Novo(logs.Opcoes{PastaDados: pasta, Nivel: "debug", SemStderr: true})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Fechar()
	m := novoMotor(t, func(o *opcoesMotor) { o.pasta = pasta; o.log = &l.Logger })
	c := m.contaConectada("+5511900000001")
	m.receber(c.ID, map[string]any{"de": "+5511944445555", "texto": "segredo-recebido-123"})
	conv := m.conversas(c.ID, "")[0]
	marca := m.marca()
	var msg Mensagem
	m.json("POST", "/v1/conversas/"+conv.ID+"/mensagens", map[string]any{"texto": "segredo-enviado-456"}, 202, &msg)
	m.esperarMensagem(marca, msg.ID, "enviada")
	m.json("POST", "/v1/disparos", map[string]any{"conta_id": c.ID, "mensagem": "segredo-disparo-789", "iniciar": true,
		"destinatarios": map[string]any{"importar": map[string]any{"texto_colado": "11 98888-1111"}},
		"ritmo":         map[string]any{"intervalo_min_s": 1, "intervalo_max_s": 1}}, 201, nil)
	m.ocioso()
	m.req("GET", "/v1/saude?token=nao-deve-ir-pro-log", nil)
	dados, err := os.ReadFile(logs.CaminhoLog(pasta))
	if err != nil {
		t.Fatal(err)
	}
	texto := string(dados)
	if !strings.Contains(texto, "mensagem recebida") || !strings.Contains(texto, "disparo enviado") {
		t.Fatalf("o log deveria registrar os eventos (sem conteúdo): %s", texto)
	}
	for _, s := range []string{"segredo-recebido-123", "segredo-enviado-456", "segredo-disparo-789", tokenTeste, "nao-deve-ir-pro-log"} {
		if strings.Contains(texto, s) {
			t.Fatalf("log vazou %q", s)
		}
	}
}
