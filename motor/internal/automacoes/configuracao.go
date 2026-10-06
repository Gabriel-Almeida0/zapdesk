package automacoes

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"zapdesk/motor/internal/armazenamento"
	"zapdesk/motor/internal/dominio"
	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/eventos"
)

// Configuração de Ajustes → Automações (data-model.md › Configuração), guardada na tabela
// configuracoes com as chaves "automacoes.*".

type faixa struct {
	chave    string
	min, max int
	ler      func(c *dominio.ConfiguracaoAutomacoes) *int
}

var faixas = []faixa{
	{"anti_loop_mensagens", 1, 100, func(c *dominio.ConfiguracaoAutomacoes) *int { return &c.AntiLoopMensagens }},
	{"anti_loop_janela_min", 1, 1440, func(c *dominio.ConfiguracaoAutomacoes) *int { return &c.AntiLoopJanelaMin }},
	{"pausa_anti_loop_min", 1, 10080, func(c *dominio.ConfiguracaoAutomacoes) *int { return &c.PausaAntiLoopMin }},
	{"pausa_humana_min", 1, 10080, func(c *dominio.ConfiguracaoAutomacoes) *int { return &c.PausaHumanaMin }},
	{"primeiros_contatos_hora", 0, 500, func(c *dominio.ConfiguracaoAutomacoes) *int { return &c.PrimeirosContatosHora }},
	{"tempo_ia_s", 5, 300, func(c *dominio.ConfiguracaoAutomacoes) *int { return &c.TempoIAS }},
	{"memoria_ia_mb", 64, 2048, func(c *dominio.ConfiguracaoAutomacoes) *int { return &c.MemoriaIAMB }},
	{"processos_ia_max", 1, 16, func(c *dominio.ConfiguracaoAutomacoes) *int { return &c.ProcessosIAMax }},
	{"ociosidade_ia_min", 1, 60, func(c *dominio.ConfiguracaoAutomacoes) *int { return &c.OciosidadeIAMin }},
}

// Config guarda a configuração em memória (lida do banco na montagem).
type Config struct {
	banco      *armazenamento.Banco
	barramento *eventos.Barramento
	mu         sync.RWMutex
	c          dominio.ConfiguracaoAutomacoes
	aoMudar    []func(dominio.ConfiguracaoAutomacoes)
}

// CarregarConfig lê as chaves gravadas sobre os padrões.
func CarregarConfig(ctx context.Context, b *armazenamento.Banco, bar *eventos.Barramento) *Config {
	c := dominio.ConfiguracaoPadrao()
	for _, f := range faixas {
		var v int
		if ok, err := armazenamento.LerConfiguracao(ctx, b.L(), "automacoes."+f.chave, &v); ok && err == nil && v >= f.min && v <= f.max {
			*f.ler(&c) = v
		}
	}
	var pg bool
	if ok, _ := armazenamento.LerConfiguracao(ctx, b.L(), "automacoes.pausa_geral", &pg); ok {
		c.PausaGeral = pg
	}
	return &Config{banco: b, barramento: bar, c: c}
}

// Configuracao atual (implementa seguranca.Configuracao).
func (c *Config) Configuracao() dominio.ConfiguracaoAutomacoes {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.c
}

// AoMudar registra quem precisa reagir (pool de runners, agendador).
func (c *Config) AoMudar(fn func(dominio.ConfiguracaoAutomacoes)) {
	c.mu.Lock()
	c.aoMudar = append(c.aoMudar, fn)
	c.mu.Unlock()
}

// Atualizar aplica um PATCH parcial (JSON) validando as faixas.
func (c *Config) Atualizar(ctx context.Context, corpo map[string]json.RawMessage) (dominio.ConfiguracaoAutomacoes, error) {
	c.mu.Lock()
	nova := c.c
	c.mu.Unlock()
	campos := map[string]string{}
	gravar := map[string]any{}
	conhecidas := map[string]bool{"pausa_geral": true}
	for _, f := range faixas {
		conhecidas[f.chave] = true
		bruto, ok := corpo[f.chave]
		if !ok {
			continue
		}
		var v int
		if err := json.Unmarshal(bruto, &v); err != nil || v < f.min || v > f.max {
			campos[f.chave] = fmt.Sprintf("Use um número de %d a %d.", f.min, f.max)
			continue
		}
		*f.ler(&nova) = v
		gravar[f.chave] = v
	}
	if bruto, ok := corpo["pausa_geral"]; ok {
		var v bool
		if err := json.Unmarshal(bruto, &v); err != nil {
			campos["pausa_geral"] = "Use verdadeiro ou falso."
		} else {
			nova.PausaGeral = v
			gravar["pausa_geral"] = v
		}
	}
	for k := range corpo {
		if !conhecidas[k] {
			campos[k] = "Campo desconhecido."
		}
	}
	if len(campos) > 0 {
		return c.Configuracao(), erros.Campos(campos)
	}
	for k, v := range gravar {
		if err := armazenamento.GravarConfiguracao(ctx, c.banco.E(), "automacoes."+k, v); err != nil {
			return c.Configuracao(), err
		}
	}
	c.mu.Lock()
	c.c = nova
	fs := append([]func(dominio.ConfiguracaoAutomacoes){}, c.aoMudar...)
	c.mu.Unlock()
	c.barramento.Publicar(eventos.AutomacoesConfiguracao, "", nova)
	for _, f := range fs {
		f(nova)
	}
	return nova, nil
}
