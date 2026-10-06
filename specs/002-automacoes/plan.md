# Implementation Plan: Automações — funil de vendas, chatbots e automações de IA programáveis

**Branch**: `002-automacoes` | **Date**: 2026-09-27 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/002-automacoes/spec.md`

## Summary

Funil de vendas (Kanban), automações de fluxo, chatbots e automações de IA em TypeScript, sobre um
único motor de eventos **dentro do motor Go** (dono dos dados, gatilhos, esperas, sessões,
anti-loop e execuções). O código TypeScript de cada automação de IA é compilado pelo **esbuild
embutido no motor** (biblioteca Go) e executado num **processo runner Node isolado**
(`automacao/runner`, binário do Electron com `ELECTRON_RUN_AS_NODE`), que fala com o motor só por
JSON-RPC no stdio e recebe apenas a API `ctx` tipada da SDK `@zapdesk/automacao`. As chamadas à
Claude API são feitas **pelo motor** (a chave nunca chega ao código do usuário). App ganha Kanban,
editor de fluxo, canvas de chatbot (React Flow), editor de código (Monaco offline) e Ajustes → IA
(segredos via `safeStorage` + stdin). MCP ganha 38 ferramentas. Constituição emendada para 1.1.0
(exceção 2 de rede no Princípio I, Princípio VIII novo).

## Technical Context

**Language/Version**: Go 1.27 (motor); TypeScript 5.9 sobre Node 24.21 (Electron 44.4.5, runner
via `ELECTRON_RUN_AS_NODE`; runner também testado com Node ≥ 22.15 do PATH); React 19 no app.

**Primary Dependencies** (novas):
- Motor: `github.com/evanw/esbuild` **v0.28.2** (`pkg/api`, versão exata), `github.com/robfig/cron/v3`
  v3.0.1 (só o parser). Claude API por `net/http` (sem SDK). Reusa `relogio`, `eventos`,
  `variaveis`, `erros`, `armazenamento`, `chat`, `organizacao`, `leads`, `disparos`.
- App: `monaco-editor` 0.57.0, `@monaco-editor/react` 4.7.0, `@xyflow/react` (12.x, fixar na
  implementação).
- Runner/SDK: nenhuma dependência de runtime (só Node); esbuild 0.28.2 e Vitest como dev.
- MCP: nenhuma nova.

**Storage**: SQLite do motor — migração `0002_automacoes.sql` (funis, etapas, posicoes_funil,
historico_funil, automacoes, sessoes_chatbot, execucoes, esperas, pausas_conversa,
memoria_automacoes; colunas `mensagens.automacao_id`, `mensagens.primeiro_contato`); projetos em
`<pasta-dados>/automacoes/<id>/`; bundles em `automacoes-compiladas/`; segredos cifrados pelo app
em `segredos.bin` (data-model.md).

**Testing**: `go test` (unitários de gatilhos, condições, executor de fluxo, chatbot, anti-loop,
pausas, esperas/vencimento, cron, permissões, compilador, cliente Claude com servidor HTTP falso;
integração da API com WhatsApp falso + `--ia=falsa` + runner real via `node`); Vitest no runner
(protocolo, bloqueios, ctx) e na SDK; Vitest + Testing Library no app; Vitest no MCP contra motor
falso real.

**Target Platform**: macOS 14+ Apple Silicon.

**Project Type**: desktop-app (Electron) + serviço local (Go) + servidor MCP (stdio) + runner
de código do usuário (Node filho do motor).

**Performance Goals**: compilação de projeto típico < 1 s; erros no editor < 2 s; fluxo ponta a
ponta < 3 s; resposta de IA < 10 s com processo frio (sem contar a Claude API); Kanban 5.000 cards.

**Constraints**: motor ocioso < 80 MB sem runners; `pronto` < 2 s; `ps` limpo em ≤ 5 s ao fechar;
runner: 256 MB de heap, 60 s por execução, 4 processos, 5 min de ociosidade; sem rede além de
WhatsApp, Claude API (automação ativa/teste/testar chave, com chave do usuário) e `ctx.http` com
permissão `rede`.

**Scale/Scope**: dezenas de automações, milhares de execuções/dia, ~55 rotas novas, 15 eventos, 38
ferramentas MCP, ~8 telas novas.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.* — avaliado contra a
constituição **1.1.0** (emendada nesta feature, MINOR; Relatório de Impacto no topo do arquivo).

| Princípio | Como o plano cumpre | Status |
|-----------|---------------------|--------|
| I. Local-first | Dados no SQLite/pasta de dados; Claude API só pelo motor, só com automação ativa/teste/"Testar chave" e chave do usuário (exceção 2); runner sem chave; `ctx.http` só com `rede`; logs do motor sem mensagens nem `ctx.log` (ficam na execução, no banco local); segredos cifrados (safeStorage) e só em memória no motor | ✅ |
| II. Leveza | esbuild como lib (sem RAM ociosa, +5,3 MB de binário); runners sob demanda, ociosidade 5 min, máx. 4; recuperação de esperas após o `pronto`; runners mortos no encerramento e saem sozinhos com stdin EOF; medição de RAM/`ps` no quickstart | ✅ |
| III. Motor fonte única | Motor dono de automações, funil, esperas, sessões, execuções e arquivos dos projetos; app e MCP só via API; runner via stdio com escopo; única exceção (valores de segredos pela janela) está escrita no princípio | ✅ |
| IV. WhatsApp isolado | Nada novo toca whatsmeow; envios usam `chat.Servico`; testes com cliente falso | ✅ |
| V. Disparo idempotente | Ação "adicionar a disparo" usa `UNIQUE(disparo_id, lead_id)` e o executor existente; execuções interrompidas viram `abortada` sem repetir passos (mesmo espírito) | ✅ |
| VI. Testes de domínio | Tarefas de teste antes da implementação para todas as regras novas do princípio; runner com integração contra motor falso + IA simulada; relógio injetável para esperas | ✅ |
| VII. Português | Rotas, campos, eventos, ferramentas, SDK (`definirAutomacao`, `ctx.responder`…), UI e erros em pt-BR | ✅ |
| VIII. Código isolado e automações seguras | Processo por automação, ambiente limpo, imports restritos (compilação + `registerHooks` + Permission Model), prazo/memória, limite documentado; portão único de envio (pausa, grupos, anti-loop, primeiro contato); simulação sem escrita | ✅ |
| Restrições (Electron seguro, fuse `runAsNode`) | Preload mínimo (novos IPC de segredos/pasta), CSP ampliada só com `worker-src 'self' blob:` e `font-src 'self' data:`; fuse mantido | ✅ |

**Re-check pós-design (Fase 1)**: contratos (`contracts/`) e modelo mantêm todos os itens.
Complexidade adicional justificada abaixo.

## Project Structure

### Documentation (this feature)

```text
specs/002-automacoes/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── api-http.md          # rotas novas do motor
│   ├── eventos-ws.md        # eventos novos
│   ├── runtime.md           # flags, stdin de controle, runner, modo falso
│   ├── runner-protocolo.md  # JSON-RPC motor ↔ runner
│   ├── sdk-automacao.md     # API completa do ctx (@zapdesk/automacao)
│   ├── formatos.md          # gatilhos, condições, ações, fluxo, chatbot, automacao.json
│   └── mcp-ferramentas.md   # 38 ferramentas novas
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository root)

```text
package.json                          # workspaces + "compartilhado/automacao-sdk", "automacao/runner"
scripts/empacotar.sh                  # + bundle do runner
scripts/sincronizar-sdk.sh            # copia automacao-sdk/dist/index.d.ts → motor (embed)

compartilhado/
├── cliente-motor/src/{tipos.ts, cliente.ts}      # + tipos e métodos das rotas/eventos novos
└── automacao-sdk/                                # NOVO "@zapdesk/automacao"
    ├── src/{index.ts, tipos.ts, erros.ts}        # definirAutomacao, API do ctx, classes de erro
    └── tests/

automacao/runner/                     # NOVO "@zapdesk/automacao-runner"
├── build.mjs                         # esbuild → dist/zapdesk-runner.mjs
├── src/
│   ├── index.ts                      # entrada: protocolo, pronto, laço
│   ├── protocolo.ts                  # tipos JSON-RPC (runner-protocolo.md)
│   ├── rpc.ts                        # NDJSON bidirecional, limites de linha
│   ├── bloqueios.ts                  # registerHooks + remoção de globais
│   ├── carregar.ts                   # import do bundle, validação da definição
│   ├── contexto.ts                   # monta ctx por execução (proxies ctx.* → motor)
│   ├── historico-ia.ts               # historicoParaIA (puro)
│   ├── http.ts                       # ctx.http.fetch com permissão e registro
│   ├── console.ts                    # console/stdout → log
│   └── sdk-runtime.ts                # módulo servido como "@zapdesk/automacao"
└── tests/

motor/internal/
├── armazenamento/migracoes/0002_automacoes.sql
├── armazenamento/{funis.go, automacoes.go, execucoes.go, esperas.go, sessoes.go, pausas.go, memoria.go}
├── funil/                            # serviço de funis/etapas/cards/histórico
├── automacoes/
│   ├── modelo/                       # tipos Gatilho, Condicao, Acao, DefinicaoFluxo, DefinicaoChatbot, Manifesto + validação
│   ├── gatilhos/                     # Fato + casamento puro (filtros, normalização, cadeia)
│   ├── condicoes/                    # avaliação pura (relógio injetado)
│   ├── seguranca/                    # Portão (pausa geral/conversa, grupo, anti-loop, 1º contato) + pausas
│   ├── acoes/                        # executores de ação (reais e simulados) sobre chat/organizacao/funil/leads/disparos
│   ├── fluxo/                        # executor de fluxo (passos persistidos, aguardar)
│   ├── chatbot/                      # máquina de sessão (puro) + serviço de sessões
│   ├── esperas/                      # agendador persistente, vencimento 24 h, cron
│   ├── execucoes/                    # registro, log limitado, retenção, erros seguidos
│   ├── despacho/                     # fila de fatos, ordenação por prioridade, fila por conversa, ouvintes
│   ├── projetos/                     # arquivos do projeto, .zapdesk/, modelos (embed), tipos SDK (embed)
│   ├── compilador/                   # esbuild pkg/api + plugin de imports + validação do manifesto
│   ├── runner/                       # pool de processos, JSON-RPC, prazos, falhas
│   ├── ponte/                        # métodos ctx.* (permissões, simulação) → serviços
│   ├── simulador/                    # sessões simuladas do chatbot (memória)
│   └── servico.go                    # fachada usada pela API (CRUD, ativar, executar, testar)
├── claude/                           # cliente Messages API (retentativa, saídas estruturadas) + falso
├── segredos/                         # cofre em memória alimentado pelo stdin
├── ciclo/                            # VigiarStdin passa a ler linhas de controle
├── config/                           # flags --runner-exec, --runner-script, --ia, --aguardar-segredos
├── aplicacao/                        # montagem dos serviços novos, ouvintes, ordem de início/encerramento
└── api/{funis.go, automacoes.go, automacoes_ia.go, execucoes.go, pausas.go, ia.go, falso_automacoes.go}
motor/testes/integracao/{funil_test.go, fluxos_test.go, chatbot_test.go, automacoes_ia_test.go, seguranca_automacoes_test.go}

app/src/
├── main/{segredos.ts, motor.ts (flags+stdin), notificacoes.ts, ipc.ts (novos canais)}
├── preload/{index.ts, tipos.ts}      # segredos.*, abrirPastaExterna, aoNotificacaoClicada
└── renderer/
    ├── telas/Funis.tsx, telas/Kanban/…
    ├── telas/Automacoes.tsx, telas/NovaAutomacao.tsx
    ├── telas/EditorFluxo/…, telas/EditorChatbot/… (React Flow + ChatSimulado)
    ├── telas/EditorIA/… (Monaco, ArvoreArquivos, PainelTeste, Execucoes)
    ├── telas/DetalheExecucao.tsx
    ├── telas/Ajustes.tsx (+ secoes/AjustesIA.tsx, secoes/AjustesAutomacoes.tsx)
    ├── componentes/FaixaAutomacoes.tsx (conversa), bolha com selo "automática"
    └── monaco/{configurar.ts, workers.ts}

mcp/src/ferramentas/{funis.ts, automacoes.ts, automacoes-ia.ts, execucoes.ts}
```

**Structure Decision**: mantém `motor/`, `app/`, `mcp/`, `compartilhado/` e cria `automacao/`
(runner). Divisão por agentes: **motor**, **app**, **mcp** e **automacao** (runner +
`compartilhado/automacao-sdk`). A Fase 2 (fundação) fixa contratos e tipos compartilhados
(`cliente-motor`, `automacao-sdk`, tipos Go do `modelo`, protocolo do runner, migração) para os
quatro trabalharem em paralelo; os contratos em `contracts/` são o ponto de sincronização.

## Fluxos-chave de arquitetura

1. **Mensagem recebida → automações**: `chat.receberMensagem` grava → ouvinte `despacho` recebe
   `Fato{mensagem_recebida, conversa, mensagem, cadeia:[]}` → pausa geral/da conversa? → sessão de
   chatbot ativa? (entrega à sessão e para) → automações ativas que casam (gatilhos puros), em
   ordem de prioridade → uma `Execucao` por automação, enfileirada por (automação, conversa) →
   fluxo (Go) | chatbot (Go) | IA (runner) → cada envio passa pelo `seguranca.Portao` →
   `chat.Enviar` com `automacao_id`.
2. **Resposta manual**: mensagem `de_mim` sem `automacao_id`/`disparo_id` → ouvinte → pausa
   `humano` 30 min + sessão de bot `humano` + esperas `sem_resposta` reprogramadas.
3. **Espera/agendamento**: executor grava `esperas` e cede → agendador acorda em
   `Relogio.Esperar(min retomar_em)` → vencimento/pausa/conta → retoma a execução.
4. **Automação de IA**: `compilador` (esbuild) → bundle por hash → `runner.Pool` obtém processo
   (automação, hash) → `inicializar` → `executar` → runner chama `ctx.*` → `ponte` valida
   permissão/simulação → serviços do motor → `claude` para `ctx.ia` → resposta → execução `ok`.
5. **Segredos**: Ajustes → IPC → `safeStorage` → `segredos.bin` → stdin `{"comando":"segredos"}`
   → `segredos.Cofre` → `claude` (ANTHROPIC_API_KEY) e `inicializar` (declarados).
6. **Encerramento**: app fecha → motor encerra despachante (execuções `rodando` → `abortada` "app
   fechado"), `runner.Pool.EncerrarTodos` (encerrar + SIGKILL 2 s), agendador para → sai em ≤ 5 s.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| 5º diretório de topo `automacao/` (runner) + `compartilhado/automacao-sdk` | O runner é um executável separado (processo isolado) e a SDK é contrato consumido por runner, app (Monaco), motor (embed dos tipos) e usuário | Colocar o runner em `mcp/` ou `app/` misturaria responsabilidades e empacotamentos; duplicar tipos da SDK causaria drift entre editor e execução |
| esbuild v0.28.2 embutido no motor (+5,3 MB no binário) | Compilar TS sem Node/npm do usuário, também pelo MCP com o app minimizado e nos testes Go | npm `esbuild` no Electron exige binário fora do asar e deixaria o MCP dependente do app; `esbuild-wasm` ~10× mais lento |
| `robfig/cron/v3` no motor | Parser de cron de 5 campos confiável | Parser próprio = mais código e testes para problema resolvido; lib pequena, pura Go, sem dependências |
| Cliente HTTP próprio da Claude API no motor | Chave fora do processo do usuário (Permission Model não bloqueia rede) | SDK TS no runner exporia a chave; `anthropic-sdk-go` é dependência grande para 1 endpoint |
| Stdin do motor vira canal de controle (segredos) | Entregar segredos sem env/argumentos/HTTP | Rota HTTP exporia escrita de segredos ao MCP; env aparece em `ps eww` e é herdada |
