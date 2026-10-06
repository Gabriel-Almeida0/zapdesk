package projetos

// EsquemaManifesto devolve o JSON Schema (draft-07) do automacao.json (formatos.md).
func EsquemaManifesto() map[string]any {
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	obj := func(tipo string, props map[string]any, req ...string) map[string]any {
		p := map[string]any{"tipo": map[string]any{"const": tipo}}
		for k, v := range props {
			p[k] = v
		}
		return map[string]any{"type": "object", "properties": p, "required": append([]string{"tipo"}, req...), "additionalProperties": false}
	}
	idOuNome := func(campoID, campoNome string) map[string]any {
		return map[string]any{"anyOf": []any{
			map[string]any{"required": []string{campoID}}, map[string]any{"required": []string{campoNome}},
		}}
	}
	etiqueta := obj("etiqueta", map[string]any{
		"evento":      map[string]any{"enum": []string{"adicionada", "removida"}},
		"etiqueta_id": str("Id da etiqueta."), "etiqueta": str("Nome da etiqueta (resolvido na compilação)."),
	}, "evento")
	for k, v := range idOuNome("etiqueta_id", "etiqueta") {
		etiqueta[k] = v
	}
	etapa := obj("entrou_etapa", map[string]any{
		"funil_id": str("Id do funil."), "funil": str("Nome do funil."),
		"etapa_id": str("Id da etapa."), "etapa": str("Nome da etapa."),
	})
	etapa["allOf"] = []any{idOuNome("funil_id", "funil"), idOuNome("etapa_id", "etapa")}
	gatilho := map[string]any{"oneOf": []any{
		obj("mensagem_recebida", map[string]any{
			"contem":            str("Trecho (sem diferenciar maiúsculas/acentos)."),
			"regex":             map[string]any{"type": "string", "minLength": 1, "maxLength": 500},
			"primeira_mensagem": map[string]any{"type": "boolean"},
			"tipo_conversa":     map[string]any{"enum": []string{"individual", "grupo", "qualquer"}},
		}),
		obj("palavra_chave", map[string]any{
			"palavras": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "minItems": 1},
			"modo":     map[string]any{"enum": []string{"palavra", "mensagem_inteira"}},
		}, "palavras"),
		obj("lead_importado", map[string]any{"origens": map[string]any{"type": "array",
			"items": map[string]any{"enum": []string{"csv", "colado", "contatos", "mcp"}}}}),
		etiqueta,
		etapa,
		obj("disparo_respondeu", map[string]any{"disparo_id": map[string]any{"type": []string{"string", "null"}}}),
		obj("sem_resposta", map[string]any{
			"apos_s":          map[string]any{"type": "integer", "minimum": 60, "maximum": 2592000},
			"origem_mensagem": map[string]any{"enum": []string{"qualquer", "disparo", "automacao", "manual"}},
		}, "apos_s"),
		obj("agendamento", map[string]any{"cron": str("Cron de 5 campos (fuso local).")}, "cron"),
		obj("agendamento", map[string]any{"intervalo_s": map[string]any{"type": "integer", "minimum": 60}}, "intervalo_s"),
		obj("manual", nil),
	}}
	inteiro := func(min, max int) map[string]any {
		return map[string]any{"type": []string{"integer", "null"}, "minimum": min, "maximum": max}
	}
	return map[string]any{
		"$schema":              "http://json-schema.org/draft-07/schema#",
		"title":                "automacao.json — manifesto de automação de IA do ZapDesk",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"versao_manifesto", "nome", "gatilhos", "permissoes"},
		"properties": map[string]any{
			"$schema":          map[string]any{"type": "string"},
			"versao_manifesto": map[string]any{"const": 1},
			"nome":             map[string]any{"type": "string", "minLength": 1, "maxLength": 80},
			"descricao":        map[string]any{"type": []string{"string", "null"}, "maxLength": 500},
			"entrada":          map[string]any{"type": "string", "pattern": `^[^./][^\\]*\.ts$`, "default": "index.ts"},
			"gatilhos":         map[string]any{"type": "array", "items": gatilho, "minItems": 1, "maxItems": 20},
			"permissoes": map[string]any{"type": "array", "uniqueItems": true, "items": map[string]any{
				"enum": []string{"enviar", "ler_conversas", "etiquetas", "funil", "leads", "ia", "rede", "agendar"}}},
			"segredos": map[string]any{"type": "array", "uniqueItems": true, "items": map[string]any{
				"type": "string", "pattern": "^[A-Z][A-Z0-9_]{0,63}$", "not": map[string]any{"const": "ANTHROPIC_API_KEY"}}},
			"contas": map[string]any{"oneOf": []any{
				map[string]any{"const": "todas"},
				map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			}},
			"incluir_grupos": map[string]any{"type": "boolean"},
			"prioridade":     map[string]any{"type": "integer", "minimum": 1, "maximum": 1000},
			"conta_envio":    map[string]any{"type": []string{"string", "null"}},
			"limites": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
				"tempo_s": inteiro(5, 300), "memoria_mb": inteiro(64, 2048),
				"anti_loop": map[string]any{"oneOf": []any{map[string]any{"type": "null"}, map[string]any{
					"type": "object", "additionalProperties": false, "required": []string{"mensagens", "janela_min"},
					"properties": map[string]any{
						"mensagens":  map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
						"janela_min": map[string]any{"type": "integer", "minimum": 1, "maximum": 1440},
					}}}},
			}},
			"ia": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
				"modelo": map[string]any{"type": []string{"string", "null"}, "pattern": "^claude-"},
			}},
		},
	}
}
