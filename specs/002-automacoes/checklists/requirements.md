# Specification Quality Checklist: Automações — funil de vendas, chatbots e automações de IA programáveis

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-09-27
**Feature**: [spec.md](../spec.md)

## Content Quality

- [x] No implementation details (languages, frameworks, APIs)
- [x] Focused on user value and business needs
- [x] Written for non-technical stakeholders
- [x] All mandatory sections completed

## Requirement Completeness

- [x] No [NEEDS CLARIFICATION] markers remain
- [x] Requirements are testable and unambiguous
- [x] Success criteria are measurable
- [x] Success criteria are technology-agnostic (no implementation details)
- [x] All acceptance scenarios are defined
- [x] Edge cases are identified
- [x] Scope is clearly bounded
- [x] Dependencies and assumptions identified

## Feature Readiness

- [x] All functional requirements have clear acceptance criteria
- [x] User scenarios cover primary flows
- [x] Feature meets measurable outcomes defined in Success Criteria
- [x] No implementation details leak into specification

## Notes

- Iteração 1 (specify): nenhum marcador [NEEDS CLARIFICATION] foi necessário — o documento de
  produto `docs/features/automacoes.md` decide o escopo (§3), os casos de borda (§8) e as decisões
  (§14). Os pontos ambíguos restantes (card do funil, definição de resposta manual, precedência do
  chatbot, IA real no teste, padrões da §13) foram resolvidos no /speckit-clarify (sessão
  2026-09-27) como decisões assumidas/do agente, sem perguntar ao usuário (instrução explícita).
- "TypeScript", "Claude API" e os IDs de modelo, "chaveiro do macOS", "MCP" e a API `ctx` aparecem
  na spec porque são requisitos de produto pedidos pelo usuário (programar em TypeScript, usar
  Claude, guardar a chave com segurança, controlar pela IA), não escolhas de implementação. A
  tecnologia de compilação, execução isolada e comunicação fica no plan.md.
- Iteração 2 (clarify): 5 perguntas resolvidas; checklist reavaliado sem regressões (16/16).
- /speckit-analyze (2026-09-27): 0 CRÍTICO; 4 ALTO corrigidos (permissão `memoria` inexistente no
  glossário vs SDK; ambiguidade do modo barra de menu vs FR-043; `go:embed` dos tipos da SDK sem
  tarefa que gere o arquivo; testes de IA do motor podendo ser pulados em silêncio no CI). MÉDIO
  corrigidos: estado `na_fila` no FR-050, unicidade das esperas no data-model, FR-008 mensurável,
  `testar` × chatbot, regressão do MVP (FR-111), SC-006 e estatísticas 24 h com tarefa. Restantes
  (MÉDIO/BAIXO, sem bloqueio): caminhos de worker do Monaco não verificados no Vite; limite de
  memória aproximado; `@xyflow/react` a fixar; snapshot da conversa expõe notas/etiquetas sem
  permissão; `POST /v1/falso/segredos` só no modo falso; anti-loop padrão vs chatbots longos.
