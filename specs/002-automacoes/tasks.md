---

description: "Lista de tarefas da feature 002 — Automações"
---

# Tasks: Automações — funil de vendas, chatbots e automações de IA programáveis

**Input**: Documentos de design em `/specs/002-automacoes/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: OBRIGATÓRIOS por constituição (Princípio VI, v1.1.0): casamento de gatilhos e
condições, executor de fluxo e de chatbot, anti-loop/pausas/primeiro contato, esperas e
vencimento, checagem de permissões do `ctx`; integração da API com WhatsApp falso; runner contra
motor falso com IA simulada; MCP contra motor falso. Testes de cada história vêm antes da
implementação e devem falhar primeiro.

**Organization**: fases por história de usuário. Cada cabeçalho de fase diz **quais diretórios a
fase toca**; dentro da fase as tarefas estão agrupadas por diretório para quatro agentes
paralelos: **motor** (`motor/`), **app** (`app/`), **mcp** (`mcp/`) e **automacao**
(`automacao/runner/` + `compartilhado/automacao-sdk/`). A Fase 2 fixa contratos e tipos
compartilhados (`compartilhado/cliente-motor`, `compartilhado/automacao-sdk`, protocolo do runner,
tipos Go do `modelo`, migração) — depois dela os agentes trabalham contra `contracts/`.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: pode rodar em paralelo (arquivos diferentes, sem dependência de tarefa incompleta)
- **[Story]**: história de usuário (US1…US5) de `spec.md`
- Caminhos relativos à raiz do repositório `~/projetos/zapdesk`

## Path Conventions

- Motor Go: `motor/internal/<pacote>/`, `motor/internal/automacoes/<subpacote>/`, `motor/testes/integracao/`
- Cliente TS compartilhado: `compartilhado/cliente-motor/src/`, `.../tests/`
- SDK: `compartilhado/automacao-sdk/src/`, `.../tests/`
- Runner: `automacao/runner/src/`, `automacao/runner/tests/`
- App: `app/src/main/`, `app/src/preload/`, `app/src/renderer/`, `app/tests/`
- MCP: `mcp/src/`, `mcp/tests/`

---

## Notas de implementação (fundação, 2026-09-27)

- **Anti-loop padrão = 10 mensagens automáticas a cada 10 min** (decisão do orquestrador; era 5:
  o chatbot conta e 5 estouraria em bots legítimos). spec.md (Clarifications, FR-031, SC-004),
  data-model.md, quickstart.md, api-http.md e docs/features/automacoes.md (§3.1, §13, §14)
  atualizados; T029/T073 já citam 10.
- Tipos TS de todos os contratos da 002 (api-http, formatos, eventos, modo falso, linha de
  controle `segredos`) estão em `compartilhado/cliente-motor/src/tipos.ts` (seção "Feature 002");
  métodos em `cliente.ts`. `Mensagem.automacao_id` e `Sistema.{automacoes_ativas, processos_ia,
  runner_disponivel}` são obrigatórios nos tipos (o motor DEVE devolvê-los). `primeiro_contato`
  não é exposto na API (fica só no banco; o contrato não o lista).
- SDK: `npm run compilar -w @zapdesk/automacao` = `tsc` + `juntar-dts.mjs`, que embute os
  `export * from './x.js'` locais num `dist/index.d.ts` autossuficiente. Reexporte sempre com
  `export * from`/`export {…} from` (não `export * as ns`).
- `motor/internal/automacoes/dependencias.go` só segura esbuild/cron no go.mod; apague-o quando
  `compilador/` e `esperas/` importarem as libs.
- **Runner/SDK (agente automacao)**: `dist/index.d.ts` da SDK é um *script* de declarações
  (`declare module '@zapdesk/automacao' {…}` + `declare module '*.md'`/`'*.txt'`) com
  `/// <reference lib="dom" preserve="true" />` — funciona via `paths` e como lib extra do Monaco
  (em arquivo-módulo os curingas `*.md` eram ignorados). Runner aceita `info`/`argumento` em
  camelCase **ou** snake_case (esclarecido em runner-protocolo.md); `2002` leva `codigo_sdk`. O
  runner bloqueia por **lista de permissão** (só `@zapdesk/automacao` + os 9 `node:*` do
  compilador; o resto — inclusive `file:`/`data:`/relativos — dá "Módulo não permitido"). Testes do
  runner rodam também no Electron: `ZAPDESK_RUNNER_EXEC=<Electron> npm test -w @zapdesk/automacao-runner`.
  T127: README da SDK e seção "Automações pelo MCP" do `mcp/README.md` completos (coluna 1-base
  documentada; teste de integração do MCP confirmado rodando contra o motor real).

## Phase 1: Setup — pacotes novos, dependências e empacotamento

**Diretórios**: raiz (`package.json`, `scripts/`, `.github/`), `compartilhado/automacao-sdk/`,
`automacao/runner/`, `motor/` (go.mod), `app/` (package.json, CSP, electron-builder)

**Purpose**: criar os dois pacotes novos e acrescentar as dependências fixadas em research.md.

- [X] T001 Acrescentar workspaces `compartilhado/automacao-sdk` e `automacao/runner` e scripts `sdk:sincronizar` (`bash scripts/sincronizar-sdk.sh`), `runner:compilar` e inclusão de ambos em `compartilhado:compilar`/`test`/`tipos` em package.json
- [X] T002 [P] Criar pacote `@zapdesk/automacao` (ESM, `strict`, `isolatedModules`, Vitest, build `tsc` com `dist/index.js` + `dist/index.d.ts` num arquivo só via `tsconfig.build.json`) em compartilhado/automacao-sdk/package.json e compartilhado/automacao-sdk/tsconfig.json
- [X] T003 [P] Criar pacote `@zapdesk/automacao-runner` (ESM, `strict`, Vitest) com build esbuild 0.28.2 para `dist/zapdesk-runner.mjs` (`platform: node`, `format: esm`, `target: node22`, sem dependências externas) em automacao/runner/package.json e automacao/runner/build.mjs
- [X] T004 [P] Adicionar `github.com/evanw/esbuild v0.28.2` (versão exata) e `github.com/robfig/cron/v3 v3.0.1` em motor/go.mod
- [X] T005 [P] Adicionar `monaco-editor@0.57.0`, `@monaco-editor/react@4.7.0` e `@xyflow/react` (12.x, versão exata) e ampliar a CSP com `worker-src 'self' blob:; font-src 'self' data:` em app/package.json e app/src/renderer/index.html
- [X] T006 [P] Script que copia `compartilhado/automacao-sdk/dist/index.d.ts` para `motor/internal/automacoes/projetos/sdk/automacao.d.ts` e, com `--verificar`, falha se diferirem em scripts/sincronizar-sdk.sh
- [X] T007 [P] Empacotar o runner: `npm run runner:compilar` em scripts/empacotar.sh e `extraResources` `../automacao/runner/dist/zapdesk-runner.mjs → runner/zapdesk-runner.mjs` (fuse `runAsNode` mantido) em app/electron-builder.yml
- [X] T008 [P] CI: compilar `compartilhado/*` e o runner **antes** de `go test ./...` (os testes de IA do motor exigem `automacao/runner/dist`), testes/tipos dos novos workspaces e `scripts/sincronizar-sdk.sh --verificar`, com `ZAPDESK_CI=1` em .github/workflows/ci.yml

**Checkpoint**: `npm install` ok; pacotes novos compilam vazios; `go build ./...` com as libs.

---

## Phase 2: Foundational — contratos tipados, migração e núcleo do motor de automações

**Diretórios**: `compartilhado/cliente-motor/`, `compartilhado/automacao-sdk/`,
`automacao/runner/`, `motor/`, `app/`, `mcp/`

**Purpose**: fixar os tipos de todos os contratos e construir o núcleo comum aos três tipos de
automação (portão de segurança, gatilhos, condições, esperas, execuções, despacho). ⚠️ Nenhuma
história começa antes desta fase.

### Contratos compartilhados (agente automacao + agente de contrato)

- [X] T009 [P] Declarar toda a API de `contracts/sdk-automacao.md` (Contexto, ApiConversa, Mensagem, MensagemRecebida, Destino, Conteudo, ApiEtiquetas, ApiFunil, ApiLeads, ApiMemoria, ApiIA, ApiHttp, ApiLog, ApiSegredos, ApiHumano, OpcoesAgendar, Agendamento, EventoAutomacao, Permissao, ModeloClaude, módulos `*.md`/`*.txt`) em compartilhado/automacao-sdk/src/tipos.ts
- [X] T010 [P] Classes `ErroAutomacao`, `ErroPermissao`, `ErroBloqueado`, `ErroIA`, `ErroSegredo`, `ErroValidacao`, `ErroNaoEncontrado`, `ErroExecucaoEncerrada` (com `codigo` = nomes de runner-protocolo.md) em compartilhado/automacao-sdk/src/erros.ts
- [X] T011 `definirAutomacao` (valida que só há handlers conhecidos e funções; marca com símbolo `Symbol.for('zapdesk.automacao')`) e reexports em compartilhado/automacao-sdk/src/index.ts (depende de T009, T010)
- [X] T012 Testes de `definirAutomacao`, das classes de erro e teste de tipos (`expectTypeOf`) do exemplo "Responder com IA" em compartilhado/automacao-sdk/tests/sdk.test.ts; gerar `dist/` e rodar `npm run sdk:sincronizar` para criar `motor/internal/automacoes/projetos/sdk/automacao.d.ts` (exigido pelo `go:embed` da T097)
- [X] T013 [P] Declarar os tipos de `contracts/runner-protocolo.md` (mensagens JSON-RPC, `inicializar`, `executar`, métodos `ctx.*` com parâmetros/resultados, códigos 1001–1010/2001–2003) em automacao/runner/src/protocolo.ts
- [X] T014 [P] Declarar os tipos novos de `contracts/api-http.md` (Funil, Etapa, Card, MovimentoFunil, Automacao, Limites, ErroDefinicao, ErroCompilacao, ResultadoCompilacao, Execucao, ExecucaoDetalhe, AcaoRegistrada, Tokens, SessaoChatbot, Pausa, EstadoConversaAutomacoes, ConfiguracaoAutomacoes, ArquivoProjeto, ConteudoArquivo, ModeloProjeto, SdkAutomacao, ConfiguracaoIA, Segredo, NovaAutomacao, AlvoExecucao, PedidoTeste), os tipos de `contracts/formatos.md` (Gatilho, Condicoes, Acao, DefinicaoFluxo, DefinicaoChatbot) como uniões discriminadas, `Mensagem.automacao_id`, novos códigos de erro e os 15 eventos de `contracts/eventos-ws.md` em `MapaEventos`/`TIPOS_EVENTO` em compartilhado/cliente-motor/src/tipos.ts
- [X] T015 Métodos do `ClienteMotor` para todas as rotas novas de `contracts/api-http.md` (arquivos com `{caminho...}` codificado por segmento) e `/v1/falso/{ia,ia/chamadas,segredos,processar-esperas}` de `contracts/runtime.md` em compartilhado/cliente-motor/src/cliente.ts (depende de T014)
- [X] T016 Testes dos métodos novos com fetch simulado (caminhos, corpos, erros `definicao_invalida`/`compilacao_falhou`/`conflito` de hash) em compartilhado/cliente-motor/tests/automacoes.test.ts

### motor/ (agente motor) — base

- [X] T017 [P] Migração com as tabelas e restrições de data-model.md (`funis` nome 1–60 UNIQUE NOCASE; `etapas` nome 1–40 UNIQUE(funil_id, nome NOCASE), cor `#RRGGBB`; `posicoes_funil` PK(lead_id, funil_id); `historico_funil`; `automacoes` tipo IN (`fluxo`,`chatbot`,`ia`), prioridade 1–1000; `sessoes_chatbot` com índice único parcial `(conversa_id) WHERE estado='ativa'`; `execucoes` estado IN (`na_fila`,`rodando`,`aguardando`,`ok`,`erro`,`simulacao`,`abortada`); `esperas` tipo IN (`aguardar`,`sem_resposta`,`agendamento`,`agendar`,`expirar_sessao`) com `chave` UNIQUE(automacao_id, chave); `pausas_conversa` motivo IN (`humano`,`anti_loop`,`manual`); `memoria_automacoes` PK(automacao_id, escopo, chave); colunas `mensagens.automacao_id` e `mensagens.primeiro_contato` + índices) em motor/internal/armazenamento/migracoes/0002_automacoes.sql
- [X] T018 [P] Teste da migração 0002 sobre um banco 0001 com dados (backup criado, índices, cascatas) em motor/internal/armazenamento/migrador_test.go
- [X] T019 [P] Tipos Go de `contracts/formatos.md` com tags JSON (Gatilho, Condicoes/Regra, Acao, DefinicaoFluxo, DefinicaoChatbot/No, Manifesto) e validação estrutural que devolve `[]ErroDefinicao{caminho, no_id, acao_id, mensagem}` (limites: 50 ações, 20 regras, 2–200 nós, 1–10 opções, ids `[A-Za-z0-9_-]{1,40}`, variável `[a-z_][a-z0-9_]{0,39}`, `apos_s` 60–2592000, `intervalo_s` ≥ 60, regex RE2 ≤ 500) em motor/internal/automacoes/modelo/
- [X] T020 [P] Testes da validação estrutural de formatos (casos válidos de formatos.md e cada erro) em motor/internal/automacoes/modelo/modelo_test.go
- [X] T021 [P] Códigos `definicao_invalida` (422), `compilacao_falhou` (422), `runner_indisponivel` (503), `ia_nao_configurada` (409), `ia_erro` (502) em motor/internal/erros/erros.go
- [X] T022 [P] Constantes e `Todos` dos 15 eventos novos em motor/internal/eventos/barramento.go
- [X] T023 [P] Flags `--runner-exec`, `--runner-script`, `--ia=real|falsa`, `--aguardar-segredos` (+ variáveis de ambiente) com testes em motor/internal/config/config.go e motor/internal/config/config_test.go
- [X] T024 [P] `VigiarStdin` passa a ler linhas JSON de controle (`{"comando":"segredos","valores":{…}}`; inválidas ignoradas sem logar conteúdo; EOF continua encerrando) com testes em motor/internal/ciclo/ciclo.go e motor/internal/ciclo/ciclo_test.go
- [X] T025 [P] Cofre de segredos em memória (substituição do conjunto; nomes `[A-Z][A-Z0-9_]{0,63}`, valores 1–4.096; `ANTHROPIC_API_KEY` reservado; assinatura de mudança; `Nomes()` sem valores; espera do primeiro envio com prazo de 2 s) com testes em motor/internal/segredos/
- [X] T026 Repositórios SQL de funis/etapas/posições/histórico, automações, execuções (log truncado em 64 KB, retenção de 500 por automação), esperas, sessões, pausas e memória (valor ≤ 64 KB, 10.000 chaves) em motor/internal/armazenamento/{funis,automacoes,execucoes,esperas,sessoes,pausas,memoria}.go (depende de T017)
- [X] T027 `automacao_id`/`primeiro_contato` em `dominio.Mensagem`, na gravação e na API; `chat.Enviar` aceita origem automática (`EnvioAutomatico{AutomacaoID, PrimeiroContato}`) e o ouvinte distingue mensagem **manual** (`de_mim` sem `automacao_id`/`disparo_id`, pela API ou eco inédito do celular) em motor/internal/dominio/dominio.go, motor/internal/armazenamento/mensagens.go e motor/internal/chat/{envio.go,recebimento.go,servico.go}
- [X] T028 Ouvintes de fatos para o despachante: `chat` (recebida com `primeira`, enviada com `manual`), `organizacao` (diferença de etiquetas em `DefinirEtiquetas`), `leads` (ids novos da importação), `disparos` (destinatário `respondeu`) — interface `automacoes.Fonte` sem dependência circular em motor/internal/chat/servico.go, motor/internal/organizacao/servico.go, motor/internal/leads/servico.go, motor/internal/disparos/acompanhamento.go

### motor/ — núcleo das automações (testes primeiro) ⚠️

- [X] T029 [P] Testes do portão: ordem pausa geral → pausa da conversa → grupo → anti-loop (10 em 10 min padrão; limite próprio só mais restritivo) → primeiro contato (20/h por conta); estouro cria pausa `anti_loop` de 60 min + notificação; mensagem manual cria/renova pausa `humano` 30 min; pausa vencida ignorada; "Assumir" sem prazo; relógio congelado em motor/internal/automacoes/seguranca/portao_test.go
- [X] T030 [P] Testes de casamento de gatilhos: todos os tipos de formatos.md, normalização (maiúsculas/acentos/pontuação), `palavra` vs `mensagem_inteira`, regex, `primeira_mensagem`, contas, grupos/`incluir_grupos`, nunca `de_mim`, cadeia (auto-disparo proibido, profundidade > 5 interrompe), ordem por prioridade e criação em motor/internal/automacoes/gatilhos/gatilhos_test.go
- [X] T031 [P] Testes de condições: todos os operadores, `todas`/`alguma`, horário atravessando meia-noite e dias 1–7 no fuso local, `maior`/`menor` numéricos, regras vazias = verdadeiro em motor/internal/automacoes/condicoes/condicoes_test.go
- [X] T032 [P] Testes do agendador de esperas: acorda na menor `retomar_em`, reacorda com espera nova menor, vencida ≤ 24 h executa, > 24 h `abortada` "expirada", conversa pausada `abortada`, conta desconectada reprograma 1 min até 24 h, `sem_resposta` substituída a cada envio e apagada na resposta, cron/intervalo calculam a próxima a partir de agora sem recuperar várias, recuperação na subida (`rodando`/`na_fila` → `abortada` "interrompida") em motor/internal/automacoes/esperas/esperas_test.go
- [X] T033 [P] Testes do registro de execuções: transições de estado, log ≤ 64 KB com `log_truncado`, ≤ 200 ações, tokens somados por modelo, retenção 500, `erros_seguidos` (zera em `ok`; 5 → desativa com `desativada_motivo="erros_seguidos"` e evento `notificacao` tipo `desativada`) em motor/internal/automacoes/execucoes/execucoes_test.go
- [X] T034 [P] Testes do despachante: fila por (automação, conversa) em ordem, filas de conversas diferentes independentes, uma execução por lead em `lead_importado`, pausa geral bloqueia tudo, gancho de sessão de chatbot ativa consome a mensagem e bloqueia gatilhos de mensagem das demais, encerramento marca `rodando` como `abortada` "app fechado" em motor/internal/automacoes/despacho/despacho_test.go

### motor/ — núcleo das automações (implementação)

- [X] T035 Portão de envio e serviço de pausas (`Autorizar`, `PausarHumano`, `Assumir`, `Retomar`, vencimento preguiçoso, evento `conversa.pausa`) em motor/internal/automacoes/seguranca/ (depende de T026, T029)
- [X] T036 [P] Casamento de gatilhos (funções puras sobre `Fato`) em motor/internal/automacoes/gatilhos/ (depende de T019, T030)
- [X] T037 [P] Avaliação de condições (pura, relógio injetado) em motor/internal/automacoes/condicoes/ (depende de T019, T031)
- [X] T038 Agendador de esperas persistente com `relogio.Relogio`, parser cron (`robfig/cron/v3` `ParseStandard`), vencimento de 24 h e recuperação na subida em motor/internal/automacoes/esperas/ (depende de T026, T032)
- [X] T039 Registro de execuções, log limitado, retenção e erros seguidos, publicando `automacao.execucao.*` (atualizada no máx. 1/s) em motor/internal/automacoes/execucoes/ (depende de T026, T033)
- [X] T040 Resolução de variáveis de automação (`{nome}` lead → contato → push, `{primeiro_nome}`, `{telefone}`, `{ultima_mensagem}`, `{conta}`, campos do lead, variáveis da execução; sem valor e sem padrão → falha "Variável {x} sem valor") reusando `internal/variaveis`, com testes em motor/internal/automacoes/acoes/variaveis.go e motor/internal/automacoes/acoes/variaveis_test.go
- [X] T041 Executores de ação comuns (reais e simulados, retornando `AcaoRegistrada`): `enviar_texto`, `enviar_template`, `adicionar_etiqueta`, `remover_etiqueta`, `atualizar_nota` (≤ 10.000), `atualizar_campo_lead`, `pausar_automacoes`, `notificar` (título 1–60, texto 1–240), todo envio pelo portão, conversa aberta pela conta de envio quando o evento não tem conversa; interfaces para funil/chatbot/IA/disparo preenchidas nas histórias; com testes em motor/internal/automacoes/acoes/ (depende de T035, T040)
- [X] T042 Despachante (fila de fatos com buffer, ordenação por prioridade, fila por (automação, conversa), cadeia, gancho de sessão de chatbot, registro dos ouvintes da T028, `pausa_geral`) e fachada `automacoes.Servico` (CRUD genérico, ativar/desativar, executar manual, configuração em `configuracoes` com faixas de data-model.md) em motor/internal/automacoes/despacho/ e motor/internal/automacoes/servico.go (depende de T034–T039, T041)
- [X] T043 Montagem na aplicação: serviços novos, ouvintes, `--aguardar-segredos` antes de iniciar despachante/agendador (após o `pronto`), encerramento (despachante → runners → agendador) em motor/internal/aplicacao/{dominio.go,servicos.go,automacoes.go} e motor/cmd/zapdesk-motor/main.go (depende de T042, T025)
- [X] T044 Rotas comuns: `GET/PATCH /v1/automacoes/configuracao`, `GET /v1/conversas/{id}/automacoes`, `POST/DELETE /v1/conversas/{id}/pausa`, `GET /v1/pausas`, `GET /v1/execucoes`, `GET /v1/execucoes/{id}`, `GET /v1/automacoes/{id}/execucoes`, `GET /v1/segredos`, `GET /v1/sistema` com `automacoes_ativas`/`processos_ia`/`runner_disponivel`, e `estatisticas_24h`/`sessoes_ativas`/`avisos` calculados no `GET /v1/automacoes` em motor/internal/api/{automacoes_comum.go,pausas.go,execucoes.go,sistema.go} (depende de T042)
- [X] T045 Rotas do modo falso `POST /v1/falso/segredos` e `POST /v1/falso/processar-esperas`; `PUT /v1/falso/relogio` acorda o agendador em motor/internal/api/falso_automacoes.go (depende de T038, T043)
- [X] T046 Harness de integração com pasta temporária, `--ia=falsa`, runner via `node` + `automacao/runner/dist` quando presente (pula testes de IA se ausente, com aviso; com `ZAPDESK_CI=1` a ausência **falha** o teste), helpers para injetar mensagem e processar esperas; testes de pausas e configuração em motor/testes/integracao/apoio_test.go e motor/testes/integracao/seguranca_automacoes_test.go (depende de T044, T045)

### automacao/runner/ (agente automacao) — base

- [X] T047 [P] Testes do NDJSON bidirecional (requisições concorrentes, notificações, linha > 1 MB → `-32600`, EOF encerra) em automacao/runner/tests/rpc.test.ts
- [X] T048 [P] Testes dos bloqueios: `import('node:fs')`, `require('child_process')`, `net`, `worker_threads` etc. lançam "Módulo não permitido"; `globalThis.fetch`/`WebSocket` ausentes; `process.env` vazio; `@zapdesk/automacao` resolve para o runtime interno em automacao/runner/tests/bloqueios.test.ts
- [X] T049 NDJSON JSON-RPC (ids por lado, limite de 1 MB, erros padrão) em automacao/runner/src/rpc.ts (depende de T013, T047)
- [X] T050 Bloqueios com `module.registerHooks` (lista de research.md R6) e remoção de globais, guardando `fetch` interno; runtime `@zapdesk/automacao` servido pelo hook a partir de compartilhado/automacao-sdk em automacao/runner/src/bloqueios.ts e automacao/runner/src/sdk-runtime.ts (depende de T011, T048)
- [X] T051 Redirecionamento de `console.*`/`process.stdout.write` para notificações `log` (texto ≤ 8 KB, `util.inspect` profundidade 4) em automacao/runner/src/console.ts (depende de T049)
- [X] T052 Ciclo de vida: notificação `pronto`, `inicializar` (import do bundle, `definirAutomacao` obrigatório → `2001 bundle_invalido`, lista de handlers), `ping`, `encerrar`, saída em stdin EOF, com testes usando um bundle de exemplo em automacao/runner/src/{index.ts,carregar.ts} e automacao/runner/tests/ciclo.test.ts (depende de T050, T051)

### app/ (agente app) — base

- [X] T053 [P] Cofre de segredos do app: `safeStorage` → `<pasta-dados>/segredos.bin` (0600), listar com máscara (`sk-ant-…` + 4 últimos), definir (nome `[A-Z][A-Z0-9_]{0,63}`, valor 1–4.096), remover, com testes (safeStorage simulado) em app/src/main/segredos.ts e app/tests/main/segredos.test.ts
- [X] T054 Motor com `--runner-exec process.execPath`, `--runner-script` (empacotado: `Resources/runner/zapdesk-runner.mjs`; dev: `automacao/runner/dist/zapdesk-runner.mjs`), `--aguardar-segredos`, e escrita da linha `segredos` no stdin após `pronto` e a cada alteração, com testes em app/src/main/motor.ts, app/src/main/index.ts e app/tests/main/motor.test.ts (depende de T053)
- [X] T055 [P] IPC/preload: `segredos.listar/definir/remover`, `abrirPastaExterna(caminho)` (só dentro de `<pasta-dados>/automacoes/`), `aoNotificacaoClicada` em app/src/main/ipc.ts, app/src/preload/index.ts e app/src/preload/tipos.ts
- [X] T056 [P] Evento WS `notificacao` → `Notification` do macOS; clique abre a conversa ou a automação na janela em app/src/main/notificacoes.ts e app/src/main/index.ts
- [X] T057 [P] Chaves de cache e invalidação pelos 15 eventos novos (TanStack Query) e hooks base (`useAutomacoes`, `useExecucoes`, `useFunis`, `useEstadoConversaAutomacoes`) em app/src/renderer/api/chaves.ts, app/src/renderer/api/eventos.ts e app/src/renderer/api/automacoes.ts
- [X] T058 [P] Itens "Funis" e "Automações" na barra lateral e rotas `funis`, `funis/:funilId`, `automacoes`, `automacoes/nova`, `automacoes/:automacaoId`, `execucoes/:execucaoId` (telas vazias com estado de carregamento/erro do MVP) em app/src/renderer/componentes/BarraLateral.tsx e app/src/renderer/rotas.tsx

### mcp/ (agente mcp) — base

- [X] T059 [P] Paridade passa a unir as tabelas de `specs/001-zapdesk-mvp/contracts/mcp-ferramentas.md` e `specs/002-automacoes/contracts/mcp-ferramentas.md` (69 ferramentas) e harness sobe o motor com `--ia=falsa` e flags do runner em mcp/tests/paridade.test.ts e mcp/tests/harness.ts
- [X] T060 [P] Instruções do servidor com o parágrafo de automações de `contracts/mcp-ferramentas.md` e registro dos quatro módulos novos (vazios) em mcp/src/servidor.ts

**Checkpoint**: `go test ./...` verde com o núcleo; cliente-motor, SDK e runner base testados;
app liga o motor com as flags novas e entrega segredos; paridade do MCP falha só pelas
ferramentas ainda não implementadas.

---

## Phase 3: User Story 1 - Organizar leads num funil de vendas com Kanban (Priority: P1) 🎯 MVP

**Diretórios**: `motor/`, `app/`

**Goal**: funis com etapas, Kanban com arrastar e soltar, histórico de movimentação.

**Independent Test**: criar funil com 4 etapas, adicionar 3 leads (um a partir de contato sem
lead), mover um cartão e conferir histórico e contagens (quickstart §2.2).

### Tests for User Story 1 (motor/) ⚠️ escrever primeiro

- [X] T061 [P] [US1] Testes do serviço de funil: nome de funil 1–60 único sem diferenciar maiúsculas; etapa 1–40 única no funil, cor `#RRGGBB`, máx. 30; lead no máximo em uma etapa por funil (mover, nunca duplicar); mover para a mesma etapa não gera histórico; contato sem lead cria lead origem `contatos`; grupo → "Grupos não entram em funil"; excluir etapa com cards exige destino ou remoção; histórico com origem `app`/`mcp`/`automacao` e nomes copiados em motor/internal/funil/servico_test.go
- [X] T062 [P] [US1] Testes de integração das rotas de funis, cards (por `lead_id`, `contato_id`, `telefone`), histórico, `GET /leads/{id}/funis`, `PATCH /leads/{id}` (merge, `null` remove, chaves normalizadas, valor ≤ 1.000) e eventos `funil.alterado`/`funil.movido` em motor/testes/integracao/funil_test.go

### Implementation for User Story 1 — motor/

- [X] T063 [US1] Serviço de funil (CRUD de funis/etapas, reordenação, cards, histórico, entrada via contato/telefone, publicação de eventos, fato `entrou_etapa` para o despachante) em motor/internal/funil/servico.go (depende de T026, T061)
- [X] T064 [US1] `PATCH /leads/{id}` e atualização de nome/campos no serviço de leads em motor/internal/leads/servico.go e motor/internal/api/leads.go
- [X] T065 [US1] Rotas de funis, etapas, cards e histórico de `contracts/api-http.md` › Funis em motor/internal/api/funis.go (depende de T063)
- [X] T066 [US1] Ações `mover_etapa` e `remover_do_funil` ligadas ao serviço de funil (origem `automacao` no histórico) em motor/internal/automacoes/acoes/funil.go (depende de T041, T063)

### Implementation for User Story 1 — app/

- [X] T067 [P] [US1] Lista de funis (criar com etapas iniciais, renomear, reordenar, excluir com confirmação; estados vazio/carregando/erro) em app/src/renderer/telas/Funis.tsx
- [X] T068 [US1] Kanban: colunas por etapa com cor e contagem, cartões virtualizados (nome, telefone, etiquetas, tempo na etapa), busca, arrastar e soltar nativo, "Mover para…" por teclado, adicionar lead/contato à etapa, atualização ao vivo por `funil.movido` em app/src/renderer/telas/Kanban/{Kanban.tsx,Coluna.tsx,Cartao.tsx}
- [X] T069 [US1] Editor de etapas (nome, cor, ordem) e diálogo de exclusão de etapa com destino em app/src/renderer/telas/Kanban/EditarEtapas.tsx
- [X] T070 [US1] Clique no cartão abre a conversa mais recente ou oferece "Iniciar conversa"; painel do cartão com histórico do lead e edição de nome/campos em app/src/renderer/telas/Kanban/PainelCartao.tsx
- [X] T071 [P] [US1] Testes do Kanban (mover por arrastar e por teclado, contagens, evento externo move o cartão, exclusão de etapa) em app/tests/renderer/kanban.test.tsx

**Checkpoint**: US1 funcional e testável sozinha (quickstart §2.2).

---

## Phase 4: User Story 2 - Automatizar o funil com gatilhos, condições, ações e esperas (Priority: P1)

**Diretórios**: `motor/`, `app/`

**Goal**: automações do tipo fluxo ponta a ponta, com esperas persistidas e proteções visíveis.

**Independent Test**: fluxos "respondeu → quente → Qualificando" e "sem resposta há 2 h →
template", com relógio controlável e reinício do motor (quickstart §2.3–2.5).

### Tests for User Story 2 (motor/) ⚠️ escrever primeiro

- [X] T072 [P] [US2] Testes do executor de fluxo: condições não atendidas → `ok` "condições não atendidas"; passos persistidos antes de executar; `aguardar` grava espera e retoma no passo seguinte; falha não fatal continua; bloqueio pelo portão registrado `bloqueada`; simulação registra "aguardaria 2 h" e segue sem escrever; `salvar_em` em motor/internal/automacoes/fluxo/fluxo_test.go
- [X] T073 [P] [US2] Testes de integração dos fluxos: respondeu disparo → etiqueta + etapa; sem resposta 2 h com reinício do motor no meio (envia uma vez) e com 30 h parado (`abortada` "expirada"); anti-loop (12 mensagens → 10 automáticas + pausa + `notificacao`); resposta manual pela API e pelo celular falso → pausa `humano`; grupo ignorado sem `incluir_grupos`; agendamento cron; executar manual; cadeia etiqueta↔etapa interrompida; primeiro contato > 20/h bloqueado em motor/testes/integracao/fluxos_test.go
- [X] T074 [P] [US2] Testes de `POST /disparos/{id}/destinatarios` (estados permitidos, idempotência por `UNIQUE(disparo_id, lead_id)`, `variaveis_faltando`, `transicao_invalida` em concluído/cancelado) em motor/testes/integracao/disparos_automacao_test.go

### Implementation for User Story 2 — motor/

- [X] T075 [US2] Executor de fluxo (passos, esperas, simulação, variáveis de execução, ações via registro de executores) em motor/internal/automacoes/fluxo/ (depende de T041, T072)
- [X] T076 [US2] `AdicionarDestinatarios` no serviço de disparos e rota `POST /v1/disparos/{id}/destinatarios`; ação `adicionar_a_disparo` em motor/internal/disparos/servico.go, motor/internal/api/disparos.go e motor/internal/automacoes/acoes/disparo.go (depende de T074)
- [X] T077 [US2] CRUD/validação/ativação de fluxos no serviço (avisos de referência quebrada, versão +1, esperas de gatilho `agendamento`/`sem_resposta` criadas ao ativar e apagadas ao desativar) e `testar` de fluxo (conversa fictícia quando sem alvo) em motor/internal/automacoes/servico.go (depende de T075)
- [X] T078 [US2] Rotas `GET/POST /v1/automacoes`, `POST /v1/automacoes/validar`, `GET/PATCH/DELETE /v1/automacoes/{id}`, `ativar`, `desativar`, `executar`, `testar` em motor/internal/api/automacoes.go (depende de T077)

### Implementation for User Story 2 — app/

- [X] T079 [P] [US2] Lista de automações (tipo, ativa com interruptor, prioridade, ok/erro 24 h, duração média, avisos) e "Nova automação" com escolha Fluxo/Chatbot/IA (código) em app/src/renderer/telas/Automacoes.tsx e app/src/renderer/telas/NovaAutomacao.tsx
- [X] T080 [US2] Editor de fluxo: seletor de gatilhos (todos de formatos.md com filtros), condições (todas/alguma), lista ordenada de ações com formulário por tipo e variáveis, contas/grupos/prioridade/conta de envio/limites, validar ao digitar (`/automacoes/validar`) com erros por campo, Salvar, Ativar, Executar (escolher contato), Testar em app/src/renderer/telas/EditorFluxo/{EditorFluxo.tsx,Gatilhos.tsx,Condicoes.tsx,Acoes.tsx}
- [X] T081 [P] [US2] Aba de execuções e detalhe da execução (estado, gatilho, ações com resultado, log, erro, tokens, duração, `retomar_em`) em app/src/renderer/telas/Execucoes.tsx e app/src/renderer/telas/DetalheExecucao.tsx
- [X] T082 [US2] Faixa de estado na conversa ("Atendimento humano até HH:MM · Devolver às automações", "Automações pausadas (anti-loop) até HH:MM · Retomar", botão "Assumir") e selo "automática" na bolha de mensagens com `automacao_id` em app/src/renderer/componentes/FaixaAutomacoes.tsx, app/src/renderer/telas/Chat.tsx e app/src/renderer/componentes/Bolha.tsx
- [X] T083 [P] [US2] Ajustes → Automações (faixas de data-model.md; interruptor "Pausar todas as automações") em app/src/renderer/telas/secoes/AjustesAutomacoes.tsx e app/src/renderer/telas/Ajustes.tsx
- [X] T084 [P] [US2] Testes do editor de fluxo (montar o fluxo do quickstart, erros de validação exibidos) e da faixa da conversa em app/tests/renderer/editor-fluxo.test.tsx e app/tests/renderer/faixa-automacoes.test.tsx

**Checkpoint**: US1 + US2 funcionando; fluxos cobrem os critérios de follow-up e anti-loop.

---

## Phase 5: User Story 3 - Programar automações de IA em TypeScript, testar e ativar (Priority: P1)

**Diretórios**: `motor/`, `automacao/runner/`, `app/`

**Goal**: projetos TypeScript compilados no motor, executados isolados no runner, com editor
Monaco, simulação, execuções e Ajustes → IA.

**Independent Test**: modelo "Responder com IA" compila, testa em simulação (IA simulada, nada
enviado), ativa e responde; automação com `while(true){}` não afeta a outra e é desativada após 5
erros; sem permissão `enviar` falha (quickstart §2.6–2.7).

### Tests for User Story 3 ⚠️ escrever primeiro

- [X] T085 [P] [US3] Testes do compilador: `index.ts` com imports relativos, `.json`, `.md`/`.txt` como texto; `@zapdesk/automacao` e `node:crypto|util|url|buffer|events|timers/promises|path/posix|string_decoder|querystring` externos; `fs`/`child_process`/pacote npm → "Módulo não permitido: <nome>" com arquivo/linha/coluna; import fora da pasta proibido; manifesto inválido e handler exigido ausente (`tipo:"manifesto"`); hash estável; < 1 s num projeto de 10 arquivos em motor/internal/automacoes/compilador/compilador_test.go
- [X] T086 [P] [US3] Testes dos projetos: caminhos (`/`, sem `..`, sem `.` inicial, segmentos `[A-Za-z0-9_.-]{1,64}`, profundidade ≤ 4), extensões `.ts .json .md .txt`, até 50 arquivos de 1 MB, `automacao.json` e entrada não excluíveis, `.zapdesk/` e `tsconfig.json` gerados e ocultos, conflito por `hash_anterior`, os 4 modelos compilam sem erro em motor/internal/automacoes/projetos/projetos_test.go
- [X] T087 [P] [US3] Testes do cliente Claude com `httptest`: cabeçalhos (`x-api-key`, `anthropic-version: 2023-06-01`), sem `temperature`, `output_config.format` json_schema com `additionalProperties:false` forçado e rejeição de min/max/recursão, retentativa em 429 (com `retry-after`)/500/504/529 até 3 dentro do prazo, 401 → "Chave da Anthropic inválida…", 429 sem `retry-after` repetido → "Limite de gasto da Anthropic atingido", `refusal`/`max_tokens`, tokens e `request-id`; IA falsa determinística em motor/internal/claude/claude_test.go
- [X] T088 [P] [US3] Testes da ponte `ctx.*`: cada método exige a permissão da tabela de runner-protocolo.md (1001 com nome da permissão), simulação não escreve e registra `simulada`, memória simulada por execução, `execucao_encerrada` após o fim, alvo ausente → 1002, segredos só declarados, máx. 5 notificações, `ErroBloqueado` com motivo do portão em motor/internal/automacoes/ponte/ponte_test.go
- [X] T089 [P] [US3] Testes do pool de runners com um runner de teste em Node: spawn com `--permission`/`--allow-fs-read`/`--max-old-space-size` e ambiente limpo (sem `ZAPDESK_TOKEN`/chave), prazo → SIGKILL e mensagens de erro de runner-protocolo.md, OOM detectado pelo stderr, máx. 4 processos com fila, ociosidade 5 min (relógio controlável), hash novo/segredo alterado reinicia, `EncerrarTodos` sem processos restantes em motor/internal/automacoes/runner/pool_test.go
- [X] T090 [P] [US3] Testes do contexto do runner: montagem do `ctx` por execução, mapeamento de erros 1001–1010 para as classes da SDK, `AbortSignal` no `cancelar`, promessas após o fim rejeitadas, `segredos.obter` síncrono, `historicoParaIA` (papéis, união de mensagens seguidas, mídia como "[imagem]"), `ctx.http.fetch` sem `rede` → `ErroPermissao` e com `rede` registrando notificação `http` sem query em automacao/runner/tests/contexto.test.ts
- [X] T091 [P] [US3] Teste de integração do runner com um motor simulado em TS (processo real, bundle compilado do modelo "Responder com IA", IA simulada) em automacao/runner/tests/integracao.test.ts
- [X] T092 [P] [US3] Testes de integração de IA no motor real: criar pelo modelo → compila; testar com IA simulada → `simulacao`, nada em `/v1/falso/enviadas`; ativar bloqueado com erro de compilação; ativar e responder mensagem injetada; `while(true){}` → `erro` no prazo sem atrasar outra automação (> 1 s) e desativada após 5 erros; sem `enviar` → erro de permissão; código alterado com erro → roda a versão anterior (`rodando_versao_anterior`); chave ausente (`--ia=real` sem segredo) → "Configure a chave da Anthropic em Ajustes → IA"; após `Encerrar`, nenhum processo filho em motor/testes/integracao/automacoes_ia_test.go

### Implementation for User Story 3 — automacao/runner/

- [X] T093 [US3] Contexto por execução (proxies `ctx.*` → JSON-RPC, simulação, `AbortSignal`, encerramento de execução, segredos, `notificar`, `humano`, `agendar`) em automacao/runner/src/contexto.ts (depende de T052, T090)
- [X] T094 [P] [US3] `historicoParaIA` (puro) em automacao/runner/src/historico-ia.ts
- [X] T095 [P] [US3] `ctx.http.fetch` com checagem de `rede`, só http(s), `ctx.sinal` e notificação `http` em automacao/runner/src/http.ts
- [X] T096 [US3] `executar`/`cancelar` no laço principal (handler certo, retorno JSON ≤ 64 KB, `2002 erro_usuario` com stack mapeado por sourcemap, `2003 handler_ausente`) em automacao/runner/src/index.ts (depende de T093–T095, T091)

### Implementation for User Story 3 — motor/

- [X] T097 [P] [US3] Projetos: arquivos na pasta de dados, `.zapdesk/automacao.d.ts` e `automacao.schema.json`, `tsconfig.json` com `paths`, hash de fontes, modelos embutidos (`responder_historico`, `classificar_funil`, `extrair_dados`, `em_branco` com prompts conservadores de formatos.md) e tipos da SDK embutidos (`go:embed` do arquivo sincronizado pela T006) em motor/internal/automacoes/projetos/ (depende de T086)
- [X] T098 [P] [US3] Compilador com `esbuild/pkg/api` (plugin `OnResolve` de permissões, carregadores, sourcemap inline, saída em `automacoes-compiladas/<id>/<hash>.mjs`, limpeza de bundles antigos), validação do manifesto (nomes → ids) e handlers exigidos em motor/internal/automacoes/compilador/ (depende de T085, T097)
- [X] T099 [P] [US3] Cliente Claude (`gerar`, `classificar` com enum, `extrair` com esquema; retentativa; modelos; `ia.modelo_padrao`) e IA falsa (`--ia=falsa`, `PUT /v1/falso/ia`, `GET /v1/falso/ia/chamadas`) em motor/internal/claude/ (depende de T025, T087)
- [X] T100 [US3] Ponte `ctx.*` (permissões, simulação, memória, funil/etiquetas/leads/envio via serviços e portão, IA via cliente Claude com tokens na execução) em motor/internal/automacoes/ponte/ (depende de T041, T066, T088, T099)
- [X] T101 [US3] Pool de runners (processo por automação+hash, `inicializar` com segredos declarados, `executar` com prazo, falhas, ociosidade, limite de processos, `EncerrarTodos`) em motor/internal/automacoes/runner/ (depende de T089, T100)
- [X] T102 [US3] Serviço de automações de IA: criar pelo modelo, compilar, recompilar antes de executar se as fontes mudaram, ativar exige compilação ok, executar/testar (IA simulada opcional), excluir apagando pasta/memória/execuções, erros seguidos, ação `executar_ia` com `salvar_em` em motor/internal/automacoes/servico_ia.go e motor/internal/automacoes/acoes/ia.go (depende de T098, T101)
- [X] T103 [US3] Rotas `POST /v1/automacoes/ia`, `GET /v1/automacoes/modelos`, `GET /v1/automacoes/sdk`, arquivos (`GET/PUT/DELETE .../arquivos/{caminho...}`, `renomear`), `compilar`, `GET/PATCH /v1/ia/configuracao`, `POST /v1/ia/testar-chave`; evento `automacao.arquivos_alterados` em motor/internal/api/automacoes_ia.go e motor/internal/api/ia.go (depende de T102)

### Implementation for User Story 3 — app/

- [X] T104 [P] [US3] Monaco offline: `loader.config({ monaco })`, workers pelos caminhos do 0.57 (`monaco-editor/editor/editor.worker?worker`, `monaco-editor/language/typescript/ts.worker?worker`), `monaco.typescript.typescriptDefaults` com opções de research.md R12, tipos da SDK de `GET /v1/automacoes/sdk` via `addExtraLib`, JSON schema do manifesto em app/src/renderer/monaco/{configurar.ts,workers.ts}
- [X] T105 [US3] Editor de IA: árvore de arquivos (criar, renomear, excluir), abas, Cmd+S, compilar 800 ms após parar de digitar com marcadores de esbuild + TS, detecção de alteração externa (poll de hashes a cada 3 s + `automacao.arquivos_alterados`, aviso e conflito ao salvar), Ativar bloqueado com erro, aviso "Rodando a versão anterior", nota do limite do isolamento, "Abrir pasta no editor externo" em app/src/renderer/telas/EditorIA/{EditorIA.tsx,ArvoreArquivos.tsx,Codigo.tsx} (depende de T104)
- [X] T106 [US3] Painel "Testar" (mensagem digitada, mensagem de conversa real, entrada JSON, evento; "IA simulada"; log ao vivo, ações que seriam feitas, retorno, duração, tokens) em app/src/renderer/telas/EditorIA/PainelTeste.tsx
- [X] T107 [P] [US3] Ajustes → IA: chave da Anthropic mascarada (definir, testar, remover), modelo padrão com aviso de aposentadoria do Haiku 4.5, segredos com nome (lista com "usado por") em app/src/renderer/telas/secoes/AjustesIA.tsx e app/src/renderer/telas/Ajustes.tsx
- [X] T108 [P] [US3] Testes do editor de IA (salvar, conflito, marcadores de erro, Ativar bloqueado) e de Ajustes → IA (máscara, valor nunca reexibido) em app/tests/renderer/editor-ia.test.tsx e app/tests/renderer/ajustes-ia.test.tsx

**Checkpoint**: US1–US3 (P1) completas; critérios de isolamento, permissão e tempo limite
verificados.

---

## Phase 6: User Story 4 - Criar chatbots de qualificação em nós (Priority: P2)

**Diretórios**: `motor/`, `app/`

**Goal**: chatbots com canvas, sessões por conversa, chat simulado e faixa "Bot ativo".

**Independent Test**: bot de `formatos.md` percorrido no simulador e com mensagens injetadas
(quickstart §2.8).

### Tests for User Story 4 (motor/) ⚠️ escrever primeiro

- [X] T109 [P] [US4] Testes da máquina de chatbot (pura): menu por número/rótulo/valores normalizados; validação `email`/`numero`/`telefone` (E.164)/`regex`; "não entendi" e `max_tentativas` → `ao_esgotar` (padrão humano); mídia inválida; condição com ramos e `senao`; nó ação; nó IA (`responder`/`variavel`, `em_erro`); humano; fim; variáveis em textos; validação do grafo (um início, alcançável, saídas, variáveis definidas antes, `aguardar`/`iniciar_chatbot` proibidos) em motor/internal/automacoes/chatbot/maquina_test.go
- [X] T110 [P] [US4] Testes de integração do chatbot: início por palavra-chave e por primeira mensagem; sessão ativa consome a mensagem e bloqueia gatilhos de mensagem das demais automações; resposta manual e "Assumir" → `humano`; inatividade 30 min → `expirada`; edição com sessão ativa mantém a versão congelada; anti-loop encerra `abortada`; simulador (`/automacoes/{id}/simulador`) sem escrita em motor/testes/integracao/chatbot_test.go

### Implementation for User Story 4 — motor/

- [X] T111 [US4] Máquina de sessão e validação do grafo em motor/internal/automacoes/chatbot/maquina.go (depende de T019, T109)
- [X] T112 [US4] Serviço de sessões (criar com definição congelada, entregar mensagem via gancho do despachante, espera `expirar_sessao`, estados finais, eventos `chatbot.sessao.*`), ação `iniciar_chatbot` e nó IA via serviço de IA em motor/internal/automacoes/chatbot/sessoes.go e motor/internal/automacoes/acoes/chatbot.go (depende de T042, T102, T111)
- [X] T113 [US4] Simulador em memória (expira em 30 min ocioso) e rotas `POST /v1/automacoes/{id}/simulador`, `POST /v1/simulador/{id}/mensagens`, `DELETE /v1/simulador/{id}`, `GET /v1/automacoes/{id}/sessoes` em motor/internal/automacoes/simulador/ e motor/internal/api/chatbot.go (depende de T112)

### Implementation for User Story 4 — app/

- [X] T114 [US4] Canvas do chatbot com `@xyflow/react`: tipos de nó (início, mensagem, menu, pergunta, condição, ação, IA, humano, fim), conexões por saída, painel de propriedades, posições salvas, nós com erro destacados pela validação em app/src/renderer/telas/EditorChatbot/{EditorChatbot.tsx,nos/,PainelNo.tsx}
- [X] T115 [US4] Chat simulado ao lado do canvas (nó atual destacado, variáveis, reiniciar) em app/src/renderer/telas/EditorChatbot/ChatSimulado.tsx
- [X] T116 [US4] Faixa "Bot X ativo nesta conversa · Assumir" com base em `EstadoConversaAutomacoes.sessao` e eventos `chatbot.sessao.*` em app/src/renderer/componentes/FaixaAutomacoes.tsx
- [X] T117 [P] [US4] Testes do editor de chatbot (montar menu→pergunta→humano, erro destacado) e do chat simulado em app/tests/renderer/editor-chatbot.test.tsx

**Checkpoint**: chatbot de exemplo aprovado em todos os caminhos.

---

## Phase 7: User Story 5 - IA cria, programa e opera automações pelo MCP (Priority: P2)

**Diretórios**: `mcp/`

**Goal**: 38 ferramentas novas como clientes finos da API.

**Independent Test**: ciclo completo de automação de IA só pelo MCP contra o motor falso
(quickstart §2.9).

> O código das ferramentas pode começar logo após a Fase 2 (contra `contracts/` e o
> `ClienteMotor`); os testes de cada grupo dependem das rotas da história correspondente.

### Tests for User Story 5 (mcp/) ⚠️ escrever primeiro

- [X] T118 [P] [US5] Testes das ferramentas de funil (criar, `editar_funil` com etapas novas/reordenadas sem apagar omitidas, `mover_card_funil` por telefone com origem `mcp`, histórico, `atualizar_lead`) em mcp/tests/funis.test.ts
- [X] T119 [P] [US5] Testes das ferramentas de automação (criar fluxo com `ativar`, validar com erros, testar sem envio, `simular_chatbot` com sequência, executar manual) em mcp/tests/automacoes.test.ts
- [X] T120 [P] [US5] Teste do ciclo de IA: `criar_automacao_ia` → `escrever_arquivo_automacao` → `compilar_automacao` com erro (lista `arquivo:linha:coluna`) e sem erro → `testar_automacao` (nada em `/v1/falso/enviadas`) → `ativar_automacao` → mensagem injetada → `listar_execucoes`/`ver_execucao` `ok`; `ver_tipos_sdk` devolve o .d.ts em mcp/tests/automacoes-ia.test.ts
- [X] T121 [P] [US5] Testes de execuções, pausas, segredos (só nomes; nenhuma ferramenta define valores) e configuração em mcp/tests/execucoes.test.ts

### Implementation for User Story 5 — mcp/

- [X] T122 [P] [US5] Ferramentas de funil (10) com resumos em português em mcp/src/ferramentas/funis.ts
- [X] T123 [P] [US5] Ferramentas de automações (12) com `ver_formatos_automacao` embutindo o resumo de `contracts/formatos.md` em mcp/src/ferramentas/automacoes.ts e mcp/src/formatos-automacao.ts
- [X] T124 [P] [US5] Ferramentas de automações de IA (9) com erros de compilação formatados em mcp/src/ferramentas/automacoes-ia.ts
- [X] T125 [P] [US5] Ferramentas de execuções, pausas, segredos e configuração (7) em mcp/src/ferramentas/execucoes.ts
- [X] T126 [US5] Esquemas zod de saída reutilizáveis (Funil, Card, Automacao, Execucao, Pausa…) e annotations conforme o contrato em mcp/src/esquemas.ts (depende de T122–T125; paridade T059 verde)

**Checkpoint**: 69 ferramentas no `listTools()`; ciclo de IA pelo MCP verde.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Diretórios**: raiz (`scripts/`), `motor/`, `app/`, `compartilhado/automacao-sdk/`, `mcp/`

- [X] T127 [P] Documentação da SDK (API do `ctx`, permissões, exemplos, **limite honesto do isolamento**) em compartilhado/automacao-sdk/README.md e seção de automações em mcp/README.md
- [X] T128 [P] `verificar-leveza.sh` com fluxos e chatbots ativos e nenhum runner: RSS do motor < 80 MB, `pronto` < 2 s; runner encerrado após ociosidade; `ps` limpo (inclui `zapdesk-runner`) ≤ 5 s após fechar em scripts/verificar-leveza.sh
- [X] T129 [P] Testes de segurança: log do motor sem texto de mensagens nem `ctx.log`; `GET /v1/segredos` e MCP sem valores; ambiente do runner sem token/chave; nenhuma conexão à Claude API sem chave/automação ativa (servidor falso não recebe chamadas) em motor/testes/integracao/seguranca_test.go
- [X] T130 [P] Desempenho: Kanban com 5.000 cards (consulta paginada < 200 ms), compilação < 1 s, fluxo ponta a ponta < 3 s, automação de IA com processo frio respondendo em < 10 s (IA simulada) em motor/testes/desempenho/automacoes_test.go
- [X] T131 [P] Revisão de textos pt-BR das telas novas e mensagens de erro (spec.md) em app/src/renderer/telas/ e motor/internal/api/erros.go
- [X] T132 Empacotar (`scripts/empacotar.sh`) e conferir no `.app`: runner em `Resources/runner/`, IA respondendo com o motor empacotado, Monaco sem rede (CSP sem violações no console)
- [ ] T133 Executar quickstart.md completo (§1–§4) **e o quickstart do MVP** (`specs/001-zapdesk-mvp/quickstart.md`, regressão FR-111) e registrar resultados em specs/002-automacoes/quickstart.md — *28/09: tudo que roda sem conta/chave real passou (ver "Resultados" no quickstart); pendentes: chave real da Anthropic, chat com conta real, MCP pelo Claude Code, barra de menu*

---

## Dependencies & Execution Order

### Phase Dependencies

- **Fase 1 (Setup)**: sem dependências.
- **Fase 2 (Fundação)**: depende da Fase 1; bloqueia todas as histórias. Blocos de contrato
  (T009–T016) primeiro; em seguida `motor/`, `automacao/runner/`, `app/` e `mcp/` em paralelo.
- **Fases 3–7**: dependem da Fase 2. Ordem: US1 → US2 → US3 (P1) → US4 → US5 (P2).
- **Fase 8**: depois das histórias desejadas.

### User Story Dependencies

- **US1 (Funil)**: só Fundação.
- **US2 (Fluxos)**: Fundação + US1 no motor (ações de funil T066 e gatilho `entrou_etapa`). O app
  pode começar em paralelo à US1.
- **US3 (IA)**: Fundação + T066 (ponte usa funil); independente de US2 exceto a ação `executar_ia`
  (T102), que só é exercitada quando existir um fluxo.
- **US4 (Chatbot)**: Fundação + US3 no motor para o nó IA (T112 depende de T102); os demais nós
  independem.
- **US5 (MCP)**: código após a Fase 2; testes dependem das rotas de US1–US4; fecha por último.

### Within Each User Story

- Testes primeiro (devem falhar) → funções puras → serviços → rotas → telas.
- `app/` trabalha contra `contracts/` e o `ClienteMotor` (T015) antes de o motor terminar.
- `automacao/runner/` trabalha contra `runner-protocolo.md` com um motor simulado em TS (T091)
  antes do pool do motor (T101).

### Divisão por agente (diretório)

| Fase | motor/ | automacao/ (runner + SDK) | app/ | mcp/ | raiz/cliente-motor |
|------|--------|---------------------------|------|------|--------------------|
| 1 Setup | 1 (T004) | 2 (T002, T003) | 1 (T005) | 0 | 4 (T001, T006–T008) |
| 2 Fundação | 30 (T017–T046) | 11 (T009–T013, T047–T052) | 6 (T053–T058) | 2 (T059–T060) | 3 (T014–T016) |
| 3 US1 | 6 (T061–T066) | 0 | 5 (T067–T071) | 0 | 0 |
| 4 US2 | 7 (T072–T078) | 0 | 6 (T079–T084) | 0 | 0 |
| 5 US3 | 13 (T085–T089, T092, T097–T103) | 6 (T090, T091, T093–T096) | 5 (T104–T108) | 0 | 0 |
| 6 US4 | 5 (T109–T113) | 0 | 4 (T114–T117) | 0 | 0 |
| 7 US5 | 0 | 0 | 0 | 9 (T118–T126) | 0 |
| 8 Polimento | 2 (T129, T130) | 1 (T127) | 1 (T131) | 0 | 3 (T128, T132, T133)* |
| **Total (133)** | **64** | **20** | **28** | **11** | **10** |

\* T007 também edita `app/electron-builder.yml`; T127 também edita `mcp/README.md`; T131 também
edita `motor/internal/api/erros.go`; T132–T133 são validação manual.

### Parallel Opportunities

- Fase 1: T002–T008 em paralelo após T001.
- Fase 2: T009/T010/T013/T014 em paralelo; no motor, T017–T025 em paralelo e depois os testes
  T029–T034 juntos; runner (T047–T052) e app (T053–T058) em paralelo ao motor.
- US3: testes T085–T092 juntos; T094/T095 (runner) e T097–T099 (motor) em paralelo; app T104 e
  T107 em paralelo ao motor.
- Entre histórias: app adianta telas de US2–US4 contra o contrato; mcp faz a US5 em paralelo às
  fases 3–6.

---

## Parallel Example: User Story 3

```bash
# motor/ — testes juntos:
Task: "T085 Testes do compilador em motor/internal/automacoes/compilador/compilador_test.go"
Task: "T087 Testes do cliente Claude em motor/internal/claude/claude_test.go"
Task: "T088 Testes da ponte ctx.* em motor/internal/automacoes/ponte/ponte_test.go"
Task: "T089 Testes do pool de runners em motor/internal/automacoes/runner/pool_test.go"

# automacao/ — em paralelo ao motor, contra runner-protocolo.md:
Task: "T090 Testes do contexto em automacao/runner/tests/contexto.test.ts"
Task: "T094 historicoParaIA em automacao/runner/src/historico-ia.ts"
Task: "T095 ctx.http.fetch em automacao/runner/src/http.ts"

# app/ — contra o contrato:
Task: "T104 Monaco offline em app/src/renderer/monaco/"
Task: "T107 Ajustes → IA em app/src/renderer/telas/secoes/AjustesIA.tsx"
```

---

## Implementation Strategy

### MVP First (US1 → US3)

1. Fase 1 + Fase 2 (contratos tipados, núcleo de automações com portão, runner base, app com
   segredos).
2. US1 → validar Kanban e histórico.
3. US2 → validar follow-up com relógio controlável e anti-loop.
4. US3 → validar isolamento, permissões e tempo limite. **Parar e validar (P1 completo).**

### Incremental Delivery

US4 (chatbot) → US5 (MCP, desenvolvido em paralelo desde a Fase 2 e fechado por último) →
Polimento e `.dmg`.

### Parallel Team Strategy (agentes por diretório)

1. Agente raiz/contrato faz a Fase 1 e T014–T016; agente **automacao** faz T009–T013 (SDK e
   protocolo) — com isso os contratos ficam fixos.
2. Quatro agentes seguem em paralelo: **motor**, **automacao**, **app**, **mcp**, cada um pegando
   só as tarefas do seu diretório, fase a fase.
3. Mudança de contrato: atualizar `contracts/` + `compartilhado/` primeiro (e rodar
   `scripts/sincronizar-sdk.sh` se a SDK mudou), avisar os outros agentes, depois implementar.

---

## Notes

- `[P]` = arquivos diferentes, sem dependência pendente.
- Arquivos tocados em várias fases (sequenciais por fase): `motor/internal/automacoes/servico.go`
  (T042, T077), `app/src/renderer/componentes/FaixaAutomacoes.tsx` (T082, T116),
  `app/src/renderer/telas/Ajustes.tsx` (T083, T107), `motor/internal/automacoes/acoes/` (T041,
  T066, T076, T102, T112).
- Verificar que os testes falham antes de implementar; commit por tarefa ou grupo lógico.
