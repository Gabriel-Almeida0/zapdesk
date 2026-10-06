---

description: "Lista de tarefas do ZapDesk MVP"
---

# Tasks: ZapDesk MVP — WhatsApp desktop local com disparo em massa e MCP

**Input**: Documentos de design em `/specs/001-zapdesk-mvp/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: OBRIGATÓRIOS por constituição (Princípio VI): testes de domínio (telefone,
deduplicação, agendamento/janela/limites, variáveis, máquinas de estado), integração da API com
cliente falso e testes das ferramentas MCP contra motor falso. Testes de cada história vêm
antes da implementação e devem falhar primeiro.

**Organization**: fases por história de usuário. Cada cabeçalho de fase diz **quais diretórios a
fase toca**; dentro da fase as tarefas estão agrupadas por diretório para que agentes paralelos
(um por diretório: `motor/`, `app/`, `mcp/`) trabalhem sem conflito de arquivo. O contrato em
`contracts/` é o ponto de sincronização: `app/` e `mcp/` desenvolvem contra ele e contra o motor
em modo falso enquanto `motor/` implementa as rotas.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: pode rodar em paralelo (arquivos diferentes, sem dependência de tarefa incompleta)
- **[Story]**: história de usuário (US1…US7) de `spec.md`
- Caminhos relativos à raiz do repositório `~/projetos/zapdesk`

## Path Conventions

- Motor Go: `motor/cmd/`, `motor/internal/<pacote>/`, `motor/testes/`
- Cliente TS compartilhado: `compartilhado/cliente-motor/src/`, `.../tests/`
- App: `app/src/main/`, `app/src/preload/`, `app/src/renderer/`, `app/tests/`
- MCP: `mcp/src/`, `mcp/tests/`

---

## Phase 1: Setup — esqueleto do monorepo e contrato tipado

**Diretórios**: raiz (`package.json`, `.github/`), `motor/`, `compartilhado/cliente-motor/`,
`app/`, `mcp/`

**Purpose**: criar os quatro projetos e o cliente TypeScript completo do contrato, para que `app/`
e `mcp/` possam ser escritos contra a API antes de o motor implementá-la.

- [X] T001 Criar `package.json` raiz com workspaces `["app", "mcp", "compartilhado/cliente-motor"]` e scripts `test`, `tipos`, `dev`, `empacotar`; `.gitignore` (node_modules, dist, out, motor/bin, *.db, runtime.json), `.editorconfig` e `.nvmrc` (22) em /package.json
- [X] T002 [P] Inicializar módulo Go `zapdesk/motor` (`go 1.27`) com as dependências de research.md §1–§5 (whatsmeow em pseudo-versão fixada, modernc.org/sqlite, nyaruka/phonenumbers v1.8.1, excelize/v2 v2.11.0, coder/websocket, zerolog, lumberjack, oklog/ulid/v2) e `main` mínimo que atende `--versao` em motor/go.mod e motor/cmd/zapdesk-motor/main.go
- [X] T003 [P] Criar pacote `@zapdesk/cliente-motor` (TypeScript ESM, `strict`, Vitest) em compartilhado/cliente-motor/package.json e compartilhado/cliente-motor/tsconfig.json
- [X] T004 [P] Criar app Electron 44 com electron-vite, React 19, TypeScript `strict` e Vitest; janela única com `contextIsolation: true`, `nodeIntegration: false`, `sandbox: true` em app/package.json, app/electron.vite.config.ts, app/src/main/index.ts, app/src/preload/index.ts e app/src/renderer/main.tsx
- [X] T005 [P] Criar projeto MCP TypeScript ESM com `@modelcontextprotocol/server` 2.x, `@modelcontextprotocol/client` (dev), zod `^4.2`, Vitest e build esbuild para `dist/zapdesk-mcp.mjs` em mcp/package.json e mcp/build.mjs
- [X] T006 [P] Configurar CI (GitHub Actions, runner macOS): `go test ./...` em motor/, `npm test --workspaces`, `npm run tipos --workspaces` em .github/workflows/ci.yml
- [X] T007 [P] Declarar todos os tipos de contracts/api-http.md › Tipos (Conta, Conversa, Mensagem, Midia, Contato, Etiqueta, Status, Lead, Arquivo, Template, Ritmo, Janela, Disparo, Destinatario, RelatorioImportacao, NovoDisparo, Pagina<T>), a união de códigos de erro e o envelope + união discriminada de todos os `tipo` de contracts/eventos-ws.md em compartilhado/cliente-motor/src/tipos.ts
- [X] T008 [P] Classe `ErroMotor` (`codigo`, `mensagem`, `detalhes`, `status`) e parser do corpo `{"erro":{...}}` em compartilhado/cliente-motor/src/erros.ts
- [X] T009 Implementar `ClienteMotor` com um método por rota de contracts/api-http.md (inclusive `/v1/falso/*` de contracts/runtime.md), `Authorization: Bearer`, multipart para `/arquivos` e `/importacoes/previa`, helper `urlBinaria(caminho)` com `?token=` em compartilhado/cliente-motor/src/cliente.ts (depende de T007, T008)
- [X] T010 Implementar `AssinaturaEventos` WebSocket (`/v1/eventos?token=`), reconexão com backoff 0,5 s → 5 s, detecção de lacuna de `seq` e callback `aoRecarregar` em compartilhado/cliente-motor/src/eventos.ts (depende de T007)
- [X] T011 [P] Implementar `lerRuntime(pastaDados)` e `motorVivo(runtime)` (pid com `kill -0` + `GET /v1/saude`) para Node e exportar tudo em compartilhado/cliente-motor/src/runtime.ts e compartilhado/cliente-motor/src/index.ts
- [X] T012 Testes do cliente com fetch simulado (auth, parse de erro, paginação, multipart) e servidor WS de teste (reconexão, lacuna de `seq`) em compartilhado/cliente-motor/tests/cliente.test.ts

**Checkpoint**: `npm test -w @zapdesk/cliente-motor` verde; os três projetos compilam vazios.

---

## Phase 2: Foundational — motor mínimo com WhatsApp falso, ciclo do app e base do MCP

**Diretórios**: `motor/` (T013–T032), `app/` (T033–T039), `mcp/` (T040–T043). Os três blocos
rodam em paralelo por agentes diferentes; dependem só dos contratos `runtime.md` e da Fase 1.

**⚠️ CRITICAL**: nenhuma história começa antes desta fase. Ao final existe um motor que sobe em
modo falso, autentica, emite eventos WS e aceita os comandos `/v1/falso/*`.

### motor/ (agente motor)

- [X] T013 [P] Pacote de configuração com todas as flags/variáveis da tabela "Flags" de contracts/runtime.md (inclusive `--religado`); `ZAPDESK_TOKEN` obrigatório "com menos de 32 caracteres → sai com código 2" em motor/internal/config/config.go (+ config_test.go)
- [X] T014 [P] Interface `Relogio { Agora() time.Time; Esperar(ctx, ate time.Time) error }` com implementação real e controlável (`Definir`, `Avancar`) em motor/internal/relogio/relogio.go (+ relogio_test.go)
- [X] T015 [P] Gerador de IDs ULID em motor/internal/ids/ids.go
- [X] T016 [P] Logs zerolog em `<pasta-dados>/logs/motor.log` com rotação lumberjack (10 MB × 5) + stderr, removendo o campo `conteudo` salvo com `--log-conteudo` em motor/internal/logs/logs.go
- [X] T017 Abrir SQLite (`modernc.org/sqlite`, DSN com `_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)`, uma conexão de escrita), migrador com arquivos embutidos, tabela `schema_migracoes` e backup `zapdesk.db.bak-<versao>` antes de aplicar migração pendente em motor/internal/armazenamento/banco.go e motor/internal/armazenamento/migrador.go
- [X] T018 Migração inicial com todas as tabelas, colunas, restrições (`UNIQUE(conta_id, jid)`, `leads.telefone` único, `UNIQUE(disparo_id, lead_id)`, etiquetas/templates com nome único `COLLATE NOCASE`), índices e FTS5 `mensagens_fts` (`unicode61 remove_diacritics 2`) com triggers, exatamente como data-model.md, em motor/internal/armazenamento/migracoes/0001_inicial.sql
- [X] T019 Testes do migrador: banco novo, reexecução idempotente, backup criado, FTS encontra "acao" em "ação" em motor/internal/armazenamento/migrador_test.go
- [X] T020 [P] Barramento de eventos pub/sub com `seq` crescente, constantes para todos os `tipo` de contracts/eventos-ws.md e assinantes com buffer (descarta e marca lacuna para assinante lento) em motor/internal/eventos/barramento.go (+ barramento_test.go)
- [X] T021 [P] Interface `Fabrica`/`Cliente`, tipos e erros sentinela (`ErrSemWhatsApp`, `ErrDesconectado`, `ErrBanido`, `ErrForaDoPrazo`, `ErrMidiaExpirada`) exatamente como contracts/cliente-whatsapp.md em motor/internal/whatsapp/cliente.go, tipos.go e erros.go
- [X] T022 Cliente WhatsApp falso determinístico: QR simulado, injeção de mensagem/recibo/estado/histórico/status, números sem WhatsApp, falhas de envio configuráveis e registro de tudo que foi "enviado"; pareamento e registro de enviadas persistidos em `<pasta-dados>/falso/` para sobreviver a reinício do motor (sessão persistida e testes de idempotência) em motor/internal/whatsapp/falso/falso.go (+ falso_test.go) (depende de T021)
- [X] T023 [P] Testes de normalização de telefone (tabela): `(11) 99999-0000`, `11999990000`, `5511999990000`, `+55 11 99999-0000`, `0055 11 99999-0000` → `+5511999990000`; fixo `1133334444` → `+551133334444`; `+1 415 555 2671` mantém DDI; `""` → motivo `vazio`; `abc` → `formato_invalido`; `+55 11 1234` → `numero_invalido`; DDI padrão configurável em motor/internal/telefone/telefone_test.go
- [X] T024 Implementar `Normalizar(bruto, ddiPadrao string) (e164 string, motivo string)` com pré-limpeza (dígitos, `00` → `+`) e `nyaruka/phonenumbers` (`Parse`, `IsValidNumber`, `Format E164`) em motor/internal/telefone/telefone.go (depende de T023)
- [X] T025 Servidor HTTP `net/http` em `127.0.0.1:<porta>`; middleware de token (Bearer; `?token=` só em GET binário e WS; comparação em tempo constante) e de Host (`127.0.0.1:<porta>` → senão `403 host_invalido`); resposta de erro padrão com todos os códigos da tabela de contracts/api-http.md; helpers de paginação por cursor (`limite` 1..200, padrão 50); log de acesso sem query string (o `?token=` nunca vai para o log) em motor/internal/api/servidor.go, middleware.go, erros.go e paginacao.go
- [X] T026 Rotas `GET /v1/saude`, `GET /v1/sistema`, `POST /v1/sistema/encerrar`, `POST /v1/sistema/energia` (publica evento interno de energia) em motor/internal/api/sistema.go
- [X] T027 WebSocket `GET /v1/eventos` (coder/websocket): primeiro evento `motor.pronto`, retransmissão do barramento, ping a cada 20 s em motor/internal/api/ws.go
- [X] T028 Rotas `/v1/falso/*` de contracts/runtime.md (incluindo `PUT /v1/falso/relogio`), registradas somente com `--whatsapp=falso` (404 no modo real) em motor/internal/api/falso.go
- [X] T029 Ciclo de vida: linhas JSON `pronto`/`erro_fatal`/`encerrando` no stdout, `motor.lock` com flock (código 3), vigia de stdin (exceto `--sem-stdin`), SIGTERM/SIGINT, encerramento gracioso em até 5 s em motor/internal/ciclo/ciclo.go
- [X] T030 Montagem em `main`: config → logs → banco/migração → barramento → fábrica (`real` | `falso`) → API → ciclo; `GOMEMLIMIT=64MiB` se não definido; `pronto` emitido só após migração e API escutando em motor/cmd/zapdesk-motor/main.go
- [X] T031 Harness de integração reutilizável: pasta temporária, motor montado com cliente falso e relógio controlável via `httptest`, cliente HTTP com token, helper `esperarEvento(tipo)` via WS e helper `reiniciarMotor()` na mesma pasta em motor/testes/integracao/harness_test.go
- [X] T032 Teste de integração da fundação: 401 sem token, 403 com Host errado, `/v1/saude` ok, WS recebe `motor.pronto`, `/v1/falso/*` retorna 404 no modo real em motor/testes/integracao/fundacao_test.go

### app/ (agente app)

- [X] T033 [P] Iniciar o motor como processo filho: binário `process.resourcesPath/bin/zapdesk-motor` (empacotado) ou `motor/bin/zapdesk-motor` (dev); token de 32 bytes base64url passado só por `ZAPDESK_TOKEN`; `ZAPDESK_DEV_FALSO=1` → `--whatsapp=falso` e pasta temporária; aguardar linha `pronto` (timeout 10 s); SIGTERM → SIGKILL após 5 s em app/src/main/motor.ts
- [X] T034 [P] Gravar `runtime.json` no formato de contracts/runtime.md com permissão `0600` após `pronto` e apagar ao encerrar em app/src/main/runtime-json.ts
- [X] T035 Ciclo básico: `app.requestSingleInstanceLock()`, abrir → motor → janela; fechar janela ou Cmd+Q sem disparo ativo → encerrar motor e sair; IPC `obterConexao()` exposto pelo preload em app/src/main/index.ts, app/src/main/ipc.ts e app/src/preload/index.ts (depende de T033, T034)
- [X] T036 [P] Testes do main: motor com binário simulado (pronto, timeout, kill em 5 s) e `runtime.json` com modo 0600 em app/tests/main/motor.test.ts
- [X] T037 Base do renderer: provider do `ClienteMotor` (conexão via preload), TanStack Query, assinatura WS que invalida/atualiza queries por tipo de evento, roteamento, tokens de tema claro/escuro por `prefers-color-scheme`, barra lateral com Conversas, Contatos, Status, Disparos, Leads, Templates, Etiquetas e Ajustes (botões com `aria-label`) em app/src/renderer/App.tsx, app/src/renderer/rotas.tsx, app/src/renderer/api/, app/src/renderer/componentes/BarraLateral.tsx e app/src/renderer/estilos/tema.css
- [X] T038 [P] Componentes base `Esqueleto`, `EstadoVazio`, `FaixaAviso`, `TelaErro`, `BotaoIcone` (rótulo acessível obrigatório) em app/src/renderer/componentes/
- [X] T039 Telas "Ligando o WhatsApp…" e erro do motor ("Não consegui ligar o WhatsApp." + detalhe + "Tentar de novo" que reinicia o motor via IPC) em app/src/renderer/telas/Carregando.tsx e app/src/renderer/telas/ErroMotor.tsx

### mcp/ (agente mcp)

- [X] T040 [P] `garantirMotor()`: `lerRuntime` + `motorVivo`; se fechado, `open -b com.gabriel.zapdesk` (fallback `open -a ZapDesk`), sondar a cada 250 ms por até 30 s; erro "Não consegui ligar o ZapDesk. Abra o app manualmente e tente de novo."; função de abrir e relógio injetáveis em mcp/src/garantir-motor.ts
- [X] T041 [P] Servidor MCP `zapdesk` com `McpServer` + `serveStdio`, registro modular de ferramentas, helpers `responder(estruturado, texto)` e `responderErro(ErroMotor)`, e `contaPadrao()` (única conta `conectada` ou erro listando contas) em mcp/src/index.ts, mcp/src/servidor.ts, mcp/src/formatar.ts e mcp/src/conta-padrao.ts
- [X] T042 Harness de testes MCP: compilar/subir o motor real com `--whatsapp=falso --sem-stdin`, pasta temporária e token; `runtime.json` de teste; `Client` conectado via `InMemoryTransport.createLinkedPair()` em mcp/tests/harness.ts
- [X] T043 Testes de `garantirMotor` (runtime ausente → chama abrir injetado; pid morto; timeout de 30 s com relógio falso) em mcp/tests/garantir-motor.test.ts

**Checkpoint**: `go run ./cmd/zapdesk-motor --whatsapp=falso` sobe em < 2 s e responde; o app
abre, liga o motor falso, mostra a barra lateral e encerra sem deixar processo; o harness MCP
conecta.

---

## Phase 3: User Story 1 - Conectar contas e conversar por texto (Priority: P1) 🎯 MVP

**Diretórios**: `motor/` (T044–T055), `app/` (T056–T066). `mcp/`: nenhum.

**Goal**: contas por QR com sessão persistida, lista de conversas, chat de texto, grupos, busca,
contatos e ciclo abrir/fechar limpo.

**Independent Test**: conectar conta (falsa ou real), enviar/receber texto em individual e grupo,
buscar mensagem antiga, fechar o app (`ps` limpo), reabrir sem QR.

### Tests for User Story 1 (motor/) ⚠️ escrever primeiro

- [X] T044 [P] [US1] Teste de integração de contas: criar → evento `conta.qr` → `/falso/escanear-qr` → `conectada`; `reiniciarMotor()` → volta `conectada` sem QR; `logout` → `desconectada`; `ban` → `banida`; renomear valida "1–60 caracteres"; remover apaga dados em motor/testes/integracao/contas_test.go
- [X] T045 [P] [US1] Teste de integração de conversas e texto: mensagem recebida cria conversa com `nao_lidas=1`; `/lida` zera; envio `pendente → enviada`; recibos `entregue → lida` sem regressão fora de ordem; grupo; nova conversa com número sem WhatsApp → `sem_whatsapp`; falha de envio → `falhou` → `/reenviar` em motor/testes/integracao/conversas_test.go
- [X] T046 [P] [US1] Teste de integração de histórico e busca: `/falso/historico` com 1.000 mensagens → eventos `sincronizacao.progresso`; paginação `antes`; busca "acao" encontra "ação" com trecho em motor/testes/integracao/busca_test.go

### Implementation for User Story 1 — motor/

- [X] T047 [P] [US1] Repositórios de contas, conversas, mensagens (inclui reações) e contatos com consultas paginadas pelos índices de data-model.md em motor/internal/armazenamento/contas.go, conversas.go, mensagens.go e contatos.go
- [X] T048 [US1] Gerenciador multi-conta: abrir clientes das contas existentes em segundo plano (sem atrasar `pronto`); criar conta (nome "1–60 caracteres", padrão pushname); QR → `conta.qr`/`conta.qr_expirado`; estados `conectando`, `conectada`, `desconectada`, `banida` e transições de data-model.md › Conta; `online` via `conexao.rede`; reconectar; remover (logout + apagar sessão, conversas, mensagens, contatos, mídias; cancelar disparos da conta) em motor/internal/contas/gerenciador.go
- [X] T049 [US1] Adaptador whatsmeow — conexão: `sqlstore.New` por conta em `sessoes/<conta_id>.db` (DSN de research.md §1), `GetFirstDevice`/`NewDevice`, `NewClient`, `GetQRChannel` antes de `Connect`, mapeamento de `Connected`/`LoggedOut`/`TemporaryBan`/`ConnectFailure`/`StreamReplaced`/`Disconnected` para `EventoEstado`, `SendPresence` após conectar, `waLog.Zerolog` em motor/internal/whatsapp/whatsmeow/conexao.go
- [X] T050 [US1] Adaptador whatsmeow — recebimento: `events.Message` → `MensagemRecebida` (texto, extended text, metadados de mídia com `ChaveDownload`, `status@broadcast`), normalização LID → telefone via `SenderAlt`/`Store.LIDs`, `events.Receipt` → `Recibo`, `HistorySync` via `ParseWebMessage`, contatos (`GetAllContacts`, `PushName`) e grupos (`GetJoinedGroups`) em motor/internal/whatsapp/whatsmeow/eventos.go e historico.go
- [X] T051 [US1] Adaptador whatsmeow — `EnviarTexto` (com citação opcional), `TemWhatsApp` (`IsOnWhatsApp` com 1 número), `MarcarLida`, tradução de erros para sentinelas em motor/internal/whatsapp/whatsmeow/envio.go
- [X] T052 [US1] Serviço de chat — recebimento: consumir `Eventos()` de cada cliente, gravar mensagens (idempotente por `(conversa_id, wa_id)`), conversas, contatos, não lidas, resumo, recibos sem regressão, history sync em lotes com `sincronizacao.progresso`; publicar `mensagem.nova`, `mensagem.atualizada`, `conversa.atualizada`, `contato.atualizado` em motor/internal/chat/servico.go e motor/internal/chat/recebimento.go
- [X] T053 [US1] Serviço de chat — envio de texto (grava `pendente` → envia → `enviada`/`falhou`), reenviar, nova conversa por telefone (`telefone.Normalizar` + `TemWhatsApp` → `sem_whatsapp`), marcar lida, busca FTS com trecho em motor/internal/chat/envio.go e motor/internal/chat/busca.go
- [X] T054 [US1] Handlers `GET/POST /v1/contas`, `GET/PATCH/DELETE /v1/contas/{id}`, `GET /v1/contas/{id}/qr`, `POST /v1/contas/{id}/reconectar` em motor/internal/api/contas.go
- [X] T055 [US1] Handlers de conversas (`GET/POST /v1/contas/{id}/conversas`, `GET /v1/conversas/{id}`, `POST /v1/conversas/{id}/lida`), mensagens de texto (`GET/POST /v1/conversas/{id}/mensagens`, `POST /v1/mensagens/{id}/reenviar`), busca (`GET /v1/contas/{id}/mensagens/busca`) e contatos (`GET /v1/contas/{id}/contatos`, `GET /v1/contatos/{id}`) em motor/internal/api/conversas.go, mensagens.go e contatos.go

### Implementation for User Story 1 — app/

- [X] T056 [P] [US1] Tela Conectar conta: QR renderizado (`qrcode`) a partir de `conta.qr`, texto "Escaneie o QR code no seu celular em Aparelhos conectados", renovação automática, "Sincronizando histórico…" com progresso, estado vazio "Nenhuma conta conectada — Conectar conta" em app/src/renderer/telas/ConectarConta.tsx
- [X] T057 [P] [US1] `SeletorConta` no topo da lista (estado de cada conta, troca, "Conectar conta") e `FaixaConta` amarela com "Conta desconectada. Reconecte para continuar." + "Reconectar", "O WhatsApp bloqueou este número." e "Sem conexão. Tentando reconectar…" em app/src/renderer/componentes/SeletorConta.tsx e app/src/renderer/componentes/FaixaConta.tsx
- [X] T058 [US1] Lista de conversas virtualizada (`@tanstack/react-virtual`) com esqueleto, não lidas, filtro "Não lidas", busca por nome e atualização por eventos WS em app/src/renderer/telas/Conversas.tsx
- [X] T059 [US1] Chat de texto: bolhas com estado (pendente, enviada, entregue, lida, falhou + "Reenviar"), nome do remetente em grupos, paginação infinita para cima, marca lida ao abrir, composer com Enter envia e Shift+Enter quebra linha em app/src/renderer/telas/Chat.tsx, app/src/renderer/componentes/Bolha.tsx e app/src/renderer/componentes/Composer.tsx
- [X] T060 [P] [US1] Nova conversa por número com aviso "Este número não tem WhatsApp" em app/src/renderer/componentes/NovaConversa.tsx
- [X] T061 [P] [US1] Busca de mensagens (resultados com conversa, data e trecho; clicar abre a conversa na mensagem) em app/src/renderer/telas/BuscaMensagens.tsx
- [X] T062 [P] [US1] Tela Contatos (lista paginada com busca; clicar abre conversa) em app/src/renderer/telas/Contatos.tsx
- [X] T063 [P] [US1] Ajustes — contas (renomear, reconectar, remover com confirmação), versão, pasta de dados e caminho dos logs (`GET /v1/sistema`) em app/src/renderer/telas/Ajustes.tsx
- [X] T064 [US1] Atalhos de teclado (Cmd+F busca, Cmd+↑/↓ troca de conversa, Cmd+N nova conversa) e gestão de foco em app/src/renderer/atalhos.ts
- [X] T065 [US1] Religar o motor uma vez após queda inesperada, passando `--religado` (disparos voltam com `motivo_pausa=motor_reiniciado`) e mostrando "O WhatsApp parou. Religando…" via IPC; segunda queda → `ErroMotor` em app/src/main/motor.ts
- [X] T066 [P] [US1] Testes de componentes com `ClienteMotor` simulado: Conversas (não lidas, filtro), Chat (estados da bolha, Reenviar), ConectarConta (QR e sucesso) em app/tests/renderer/us1.test.tsx

**Checkpoint**: US1 funcional e testável sozinha (quickstart cenários 1, 2, 4).

---

## Phase 4: User Story 2 - Montar a base de leads sem duplicar (Priority: P1)

**Diretórios**: `motor/` (T067–T074), `app/` (T075–T078). `mcp/`: nenhum.

**Goal**: importar CSV/XLSX, colados e contatos com relatório exato de novos, já existentes,
inválidos e duplicados no lote; tela de Leads.

**Independent Test**: importar a fixture de 1.000 linhas e conferir o relatório e a ausência de
telefone repetido.

### Tests for User Story 2 (motor/) ⚠️ escrever primeiro

- [X] T067 [P] [US2] Testes do importador: duplicados no lote (primeira ocorrência vence; `duplicados_no_lote` com `primeira_linha`); telefone existente → `ja_existentes` com `importado_em` original; preenche só campos vazios ("nunca sobrescreve"; `origem` e `importado_em` intactos; `campos_preenchidos` listados); inválidos com motivo; invariante `novos + ja_existentes + invalidos + duplicados_no_lote = total_linhas`; tudo numa transação em motor/internal/leads/importador_test.go
- [X] T068 [P] [US2] Testes do leitor: CSV com `,` `;` `\t`, BOM UTF-8, aspas; XLSX primeira planilha; sugestão de coluna de telefone (telefone, celular, whatsapp, fone, phone) e nome; chaves de `campos` normalizadas (minúsculas, sem acento, espaços → `_`) em motor/internal/importacao/leitor_test.go
- [X] T069 [P] [US2] Fixtures `leads-1000.csv` (100 duplicados no lote, 50 telefones a pré-cadastrar, 12 inválidos), `leads.xlsx` e gerador em motor/testes/dados/
- [X] T070 [P] [US2] Teste de integração: prévia → importar com mapeamento → relatório exato da fixture em < 5 s; forma colada; forma estruturada (`origem: "mcp"`); `importar-contatos`; `GET /v1/leads` com busca e origem; erro "Escolha qual coluna tem o telefone."; evento `leads.importados` em motor/testes/integracao/leads_test.go

### Implementation for User Story 2 — motor/

- [X] T071 [US2] Repositório de leads (`telefone` "normalizado E.164, único", `campos` JSON, busca, `ultimo_disparo_em` derivado) em motor/internal/armazenamento/leads.go
- [X] T072 [US2] Importador: normalização com `telefone.Normalizar`, deduplicação no lote e na base, preenchimento de vazios, relatório de data-model.md › Lead, transação única, ligação `contatos.lead_id` por telefone igual em motor/internal/leads/importador.go
- [X] T073 [US2] Leitor CSV (`encoding/csv`) e XLSX (`excelize` `OpenReader`/`GetRows`) com cache de prévias que expira em 30 min em motor/internal/importacao/leitor.go e motor/internal/importacao/previas.go
- [X] T074 [US2] Handlers `POST /v1/importacoes/previa` (multipart ou caminho), `POST /v1/leads/importar` (formas A/B/C, `origem`, `ddi_padrao`), `POST /v1/leads/importar-contatos`, `GET /v1/leads`, `GET /v1/leads/{id}` em motor/internal/api/leads.go

### Implementation for User Story 2 — app/

- [X] T075 [US2] Tela Leads: tabela virtualizada com busca, filtro de origem, colunas telefone, nome, origem, importado em, último disparo e tem WhatsApp; vazio "Nenhum lead — importe um CSV ou use o MCP" em app/src/renderer/telas/Leads.tsx
- [X] T076 [US2] Assistente Importar leads: aba Arquivo (.csv/.xlsx → prévia → mapear telefone obrigatório, nome e extras), aba Colar números (um por linha), aba Contatos/etiquetas em app/src/renderer/telas/ImportarLeads.tsx
- [X] T077 [P] [US2] Componente `RelatorioImportacao` reutilizável ("32 números já estavam na sua base", "12 números inválidos", listas expansíveis com linha, data original e motivo) em app/src/renderer/componentes/RelatorioImportacao.tsx
- [X] T078 [P] [US2] Testes do assistente (bloqueio sem coluna de telefone; renderização do relatório) em app/tests/renderer/us2.test.tsx

**Checkpoint**: US1 e US2 funcionam de forma independente (quickstart cenário 5).

---

## Phase 5: User Story 3 - Disparar em massa com ritmo, agendamento e relatório (Priority: P1)

**Diretórios**: `motor/` (T079–T093), `app/` (T094–T103). `mcp/`: nenhum.

**Goal**: assistente de disparo, agendador idempotente com fila por conta, progresso em tempo real,
pausar/retomar/cancelar, relatório CSV e barra de menu enquanto houver disparo ativo.

**Independent Test**: disparo para 5 números próprios com `{nome}`, anexo, intervalo 5–10 s,
limite por hora e janela; matar o app no meio, reabrir, retomar sem duplicar; exportar CSV.

### Tests for User Story 3 (motor/) ⚠️ escrever primeiro

- [X] T079 [P] [US3] Testes de variáveis: extrair `{nome}` e `{cidade}`; escape `{{`/`}}`; chave normalizada; `valores_padrao`; destinatários incompletos listados; variável ausente em todos os leads bloqueia em motor/internal/variaveis/variaveis_test.go
- [X] T080 [P] [US3] Testes de agendamento com relógio fixo e aleatório semeado: intervalo uniforme em [min, max]; pausa a cada N; limite por hora em janela móvel de 60 min; limite por dia no dia civil local (zera à meia-noite); janela normal e que cruza meia-noite; início futuro; retomada após sono sem rajada (regra 6); estimativa de término em motor/internal/agendamento/agendamento_test.go
- [X] T081 [P] [US3] Testes das máquinas de estado: todas as transições válidas e inválidas de disparo (data-model.md › Disparo) e de destinatário (sem regressão; `respondeu` só após `enviado`); boot converte ativos em `pausado`; reconciliação de `enviando` (achado no histórico → `enviado`; senão `falhou` "Estado incerto após interrupção") em motor/internal/disparos/estados_test.go
- [X] T082 [P] [US3] Testes do executor com cliente falso e relógio controlável: fila por conta (um ativo; pausar libera; retomar entra na fila); pausa após 10 falhas seguidas ("Sem WhatsApp" não conta, sucesso zera; 0 desliga); conta desconectada pausa e banida para; queda de rede não marca `falhou`; `respondeu` por mensagem recebida em motor/internal/disparos/executor_test.go
- [X] T083 [P] [US3] Teste de integração: validar/criar/iniciar/pausar/retomar/cancelar via HTTP; `variaveis_faltando`; idempotência (`reiniciarMotor()` no meio → `pausado` com `motivo_pausa=app_fechado` → retomar → `/v1/falso/enviadas` sem telefone repetido); janela e limite por hora com `/v1/falso/relogio`; `relatorio.csv` com BOM e colunas do contrato em motor/testes/integracao/disparos_test.go

### Implementation for User Story 3 — motor/

- [X] T084 [P] [US3] Pacote de variáveis (extrair, resolver com `lead.nome`, `lead.campos`, `valores_padrao`, listar incompletos) em motor/internal/variaveis/variaveis.go
- [X] T085 [P] [US3] Funções puras `ProximoEnvio(estado, agora, config)` e `Estimativa(...)` conforme data-model.md › Cálculo do agendamento em motor/internal/agendamento/agendamento.go
- [X] T086 [US3] Repositórios de disparos e destinatários (contadores por estado, índices `(disparo_id, estado)` e `(disparo_id, ordem)`, paginação por `ordem`) em motor/internal/armazenamento/disparos.go e motor/internal/armazenamento/destinatarios.go
- [X] T087 [US3] Máquinas de estado e validação de `NovoDisparo` com as restrições literais: `intervalo_min_s ≥ 1`; `intervalo_max_s ≥ intervalo_min_s`; `limite_por_hora`, `limite_por_dia`, `pausa_a_cada`, `pausa_duracao_s` `> 0`; pausa em par; janela "ambos ou nenhum", `HH:MM`, diferentes; mensagem "não vazio, até 4.096 caracteres"; nome "1–80 caracteres"; `falhas_seguidas_max ≥ 0` (padrão 10); cálculo de `aviso_ritmo_agressivo` em motor/internal/disparos/estados.go e motor/internal/disparos/validacao.go
- [X] T088 [US3] Serviço de disparos: criar (resolve `lead_ids`, `etiqueta_ids`, `contato_ids` e `importar` via importador de leads; dedup por lead; ordem), validar/prévia, iniciar/pausar/retomar/cancelar, fila por conta (`na_fila`), boot → `pausado` + reconciliação de `enviando` em motor/internal/disparos/servico.go
- [X] T089 [US3] Executor por conta: goroutine; `Relogio.Esperar` até `ProximoEnvio`; commit `enviando` → `TemWhatsApp` (grava `tem_whatsapp` do lead) → envio → commit `enviado` + `mensagem_wa_id` (mensagem gravada com `disparo_id`); falhas e falhas seguidas; reage a eventos de conta e de energia; `disparo.atualizado` limitado a 1/s; `disparo.finalizado`; `disparos.ativos` em motor/internal/disparos/executor.go
- [X] T090 [US3] Acompanhamento: recibos → `entregue`/`lido`; mensagem do número na mesma conta após `enviado_em` → `respondeu` em motor/internal/disparos/acompanhamento.go
- [X] T091 [US3] Arquivos: `POST /v1/arquivos` (multipart ou `caminho`), limites "imagem, vídeo e áudio 16 MB; documento 100 MB; figurinha 1 MB (`image/webp`)", mimetype pelo conteúdo, `GET /v1/arquivos/{id}` e `/conteudo` em motor/internal/arquivos/arquivos.go e motor/internal/api/arquivos.go
- [X] T092 [US3] Adaptador whatsmeow — `EnviarMidia` (`Upload`/`UploadReader` + `ImageMessage`/`VideoMessage`/`AudioMessage`/`DocumentMessage`/`StickerMessage` com legenda) em motor/internal/whatsapp/whatsmeow/midia.go
- [X] T093 [US3] Handlers de todas as rotas da seção Disparos de contracts/api-http.md (incluindo `relatorio.csv` com BOM) em motor/internal/api/disparos.go

### Implementation for User Story 3 — app/

- [X] T094 [US3] Lista de Disparos (estado, "Na fila", progresso; vazio "Nenhum disparo ainda — Novo disparo") em app/src/renderer/telas/Disparos.tsx
- [X] T095 [US3] Novo disparo, passo Lista: conta; leads existentes (busca/seleção), importar arquivo, colar números, contatos/etiquetas; mostra válidos, novos e duplicados com `RelatorioImportacao` em app/src/renderer/telas/NovoDisparo/PassoLista.tsx
- [X] T096 [US3] Passo Mensagem: editor com chips das variáveis disponíveis, anexo via `POST /v1/arquivos` (recusa acima do limite), prévia com linha real via `/v1/disparos/validar` em app/src/renderer/telas/NovoDisparo/PassoMensagem.tsx
- [X] T097 [US3] Passo Ritmo e agendamento: intervalo mín./máx., limites por hora/dia, pausa a cada N, início, janela, falhas seguidas (padrão 10), aviso de risco de banimento sem bloquear, estimativa de término em app/src/renderer/telas/NovoDisparo/PassoRitmo.tsx
- [X] T098 [US3] Passo Revisão: resumo, "5 contatos sem {nome}" com lista e campo de valor padrão, "Salvar rascunho" e "Iniciar" em app/src/renderer/telas/NovoDisparo/PassoRevisao.tsx e app/src/renderer/telas/NovoDisparo/index.tsx
- [X] T099 [US3] Detalhe do disparo: barra de progresso, contadores por estado, próximo envio, estimativa de término, estado ("Aguardando janela (9h–18h)", "Na fila", pausado com motivo — ex.: "Disparo pausado porque o app foi fechado.", "Disparo pausado após muitas falhas seguidas."), Pausar/Retomar/Cancelar, tabela de destinatários filtrável, Exportar CSV (diálogo salvar via preload) em app/src/renderer/telas/DetalheDisparo.tsx
- [X] T100 [US3] Ciclo com disparo ativo: interceptar fechar/Cmd+Q quando `disparos.ativos > 0`, avisar "Há um disparo em andamento. O ZapDesk vai continuar na barra de menu até terminar.", esconder janela e dock, `Tray` com progresso e "Abrir ZapDesk"; quando ativos = 0, `Notification` "Disparo concluído: N enviados, M falharam" e encerramento total em app/src/main/ciclo-vida.ts e app/src/main/bandeja.ts
- [X] T101 [US3] `powerMonitor` (`suspend`/`resume`) → `POST /v1/sistema/energia` em app/src/main/energia.ts
- [X] T102 [P] [US3] Testes da lógica de ciclo de vida (fechar com/sem ativos; fim do disparo → notificar e sair; reabrir pela bandeja) em app/tests/main/ciclo-vida.test.ts
- [X] T103 [P] [US3] Testes do assistente (validação de ritmo, bloqueio por variáveis faltando, aviso de ritmo agressivo) em app/tests/renderer/us3.test.tsx

**Checkpoint**: MVP P1 completo (quickstart cenários 6, 7, 8).

---

## Phase 6: User Story 4 - Mídia e interações no chat (Priority: P2)

**Diretórios**: `motor/` (T104–T110), `app/` (T111–T116). `mcp/`: nenhum.

**Goal**: ver/enviar imagem, vídeo, áudio (inclusive gravado), documento e figurinha; citar,
reagir, editar e apagar para todos.

**Independent Test**: enviar e receber cada tipo, gravar áudio, citar, reagir, editar e apagar,
conferindo no celular.

### Tests for User Story 4 (motor/) ⚠️ escrever primeiro

- [X] T104 [P] [US4] Testes de prazos: editar só `de_mim` e até 15 min; apagar para todos só `de_mim` e até 48 h; `pode_editar`/`pode_apagar` em motor/internal/chat/prazos_test.go
- [X] T105 [P] [US4] Teste de integração com falso: envio de cada tipo via `arquivo_id` e `como`; citação; reação e remoção; edição; apagar; `fora_do_prazo`; download sob demanda com cache; falha de download → `whatsapp_erro` em motor/testes/integracao/midia_test.go
- [X] T106 [P] [US4] Teste do remux WebM → OGG (fixture curta → OGG válido com páginas Opus) em motor/internal/arquivos/audio_test.go

### Implementation for User Story 4 — motor/

- [X] T107 [US4] Remux de áudio WebM/Opus → `audio/ogg; codecs=opus` (ebml-go + oggwriter) com fallback para áudio não-PTT em motor/internal/arquivos/audio.go
- [X] T108 [US4] Adaptador whatsmeow — `Reagir` (`BuildReaction`), `Editar` (`BuildEdit`), `Apagar` (`BuildRevoke`), citação em mídia, `Baixar` (`Download`/`DownloadMediaWithPath`, `SendMediaRetryReceipt` se expirada) e eventos de reação/edição/revogação em motor/internal/whatsapp/whatsmeow/interacoes.go
- [X] T109 [US4] Chat: envio de mídia (`como`: auto, voz, figurinha, documento), reações, edição, apagar, prazos em constantes únicas, download sob demanda com cache em `midia/<conta_id>/`, figurinhas recentes em motor/internal/chat/midia.go, motor/internal/chat/interacoes.go e motor/internal/chat/prazos.go
- [X] T110 [US4] Handlers: `POST /v1/conversas/{id}/mensagens` com `arquivo_id`/`como`/`citar_mensagem_id`, `POST /v1/mensagens/{id}/reacao`, `PATCH`/`DELETE /v1/mensagens/{id}`, `GET /v1/mensagens/{id}/midia`, `GET /v1/contas/{id}/figurinhas` em motor/internal/api/mensagens.go

### Implementation for User Story 4 — app/

- [X] T111 [P] [US4] Bolhas de mídia: imagem (miniatura → completa), vídeo, áudio com player (voz e comum), documento (nome, tamanho, abrir), figurinha, "Não foi possível baixar." + "Tentar baixar de novo", apagada, editada, reações em app/src/renderer/componentes/bolhas/
- [X] T112 [US4] Anexar no composer (botão, arrastar e soltar, colar imagem), upload multipart, legenda, recusa acima do limite em app/src/renderer/componentes/Composer.tsx
- [X] T113 [US4] Gravar áudio com `MediaRecorder` (`audio/webm;codecs=opus`), permissão de microfone no main (`setPermissionRequestHandler`) e `NSMicrophoneUsageDescription`, UI gravar/cancelar/enviar como voz em app/src/renderer/componentes/GravadorAudio.tsx, app/src/main/janela.ts e app/electron-builder.yml
- [X] T114 [P] [US4] Seletor de figurinhas (recentes + enviar arquivo .webp) em app/src/renderer/componentes/SeletorFigurinha.tsx
- [X] T115 [US4] Menu da mensagem: responder (citação no composer), reagir (emojis rápidos), editar (se `pode_editar`), apagar para todos (se `pode_apagar`; indisponível fora do prazo) em app/src/renderer/componentes/MenuMensagem.tsx
- [X] T116 [P] [US4] Testes (menu respeita prazos; bolha com erro de download) em app/tests/renderer/us4.test.tsx

**Checkpoint**: chat completo (quickstart cenário 10).

---

## Phase 7: User Story 5 - Organizar contatos com etiquetas, notas e templates (Priority: P2)

**Diretórios**: `motor/` (T117–T120), `app/` (T121–T127). `mcp/`: nenhum.

**Goal**: etiquetas coloridas, notas, filtro por etiqueta, templates com atalho `/`.

**Independent Test**: criar etiqueta "Quente", aplicar a 3 contatos, filtrar, escrever nota, criar
template e inseri-lo via `/`.

### Tests for User Story 5 (motor/) ⚠️ escrever primeiro

- [X] T117 [P] [US5] Teste de integração: CRUD de etiquetas (nome "único, 1–30 chars" sem diferenciar maiúsculas → `conflito`; cor `#RRGGBB`), notas "até 10.000 caracteres", etiquetas em contatos, filtro `etiqueta_id` em conversas e contatos, CRUD de templates (nome "1–60", texto "1–4.096", `variaveis` extraídas, anexo) em motor/testes/integracao/organizacao_test.go

### Implementation for User Story 5 — motor/

- [X] T118 [US5] Repositórios de etiquetas, `contato_etiquetas` e templates em motor/internal/armazenamento/etiquetas.go e motor/internal/armazenamento/templates.go
- [X] T119 [US5] Serviço de organização com as validações de data-model.md e eventos `etiquetas.alteradas`, `templates.alterados`, `contato.atualizado` em motor/internal/organizacao/servico.go
- [X] T120 [US5] Handlers `/v1/etiquetas`, `/v1/templates`, `PATCH /v1/contatos/{id}`, `PUT /v1/contatos/{id}/etiquetas` e filtro `etiqueta_id` em listagens de conversas e contatos em motor/internal/api/organizacao.go, motor/internal/api/conversas.go e motor/internal/api/contatos.go

### Implementation for User Story 5 — app/

- [X] T121 [P] [US5] Tela Etiquetas (CRUD com seletor de cor) em app/src/renderer/telas/Etiquetas.tsx
- [X] T122 [P] [US5] Tela Templates (CRUD, anexo opcional, variáveis detectadas) em app/src/renderer/telas/Templates.tsx
- [X] T123 [US5] Painel lateral do contato (etiquetas, notas com salvamento, lead de origem e data) em app/src/renderer/componentes/PainelContato.tsx
- [X] T124 [US5] Filtro por etiqueta na lista de conversas em app/src/renderer/telas/Conversas.tsx
- [X] T125 [US5] Atalho `/` no composer: sugestões por nome, inserir texto e anexo para revisão antes do envio em app/src/renderer/componentes/SugestoesTemplate.tsx e app/src/renderer/componentes/Composer.tsx
- [X] T126 [US5] Escolher template no passo Mensagem do Novo disparo em app/src/renderer/telas/NovoDisparo/PassoMensagem.tsx
- [X] T127 [P] [US5] Testes (atalho `/`, filtro por etiqueta) em app/tests/renderer/us5.test.tsx

**Checkpoint**: organização completa.

---

## Phase 8: User Story 6 - Ver status dos contatos (Priority: P2)

**Diretórios**: `motor/` (T128–T130), `app/` (T131). `mcp/`: nenhum.

**Goal**: ver status publicados nas últimas 24 h.

**Independent Test**: publicar status em outro número e vê-lo na tela Status.

- [X] T128 [P] [US6] Teste de integração: `/v1/falso/contas/{id}/status` → `GET /v1/contas/{id}/status` agrupado por contato; some após 24 h (relógio falso); evento `status.novo` em motor/testes/integracao/status_test.go
- [X] T129 [US6] Repositório e serviço de status (grava mensagens de `status@broadcast`, limpeza de itens com mais de 24 h) em motor/internal/armazenamento/status.go e motor/internal/chat/status.go
- [X] T130 [US6] Handlers `GET /v1/contas/{id}/status` e `GET /v1/status/{id}/midia` em motor/internal/api/status.go
- [X] T131 [US6] Tela Status (lista por contato, visualizador de texto/imagem/vídeo, estado vazio) em app/src/renderer/telas/Status.tsx

**Checkpoint**: todas as histórias de interface completas.

---

## Phase 9: User Story 7 - IA controla o ZapDesk pelo MCP (Priority: P2)

**Diretórios**: `mcp/` (T132–T144), `app/` (T145–T146). `motor/`: nenhum.

**Nota de paralelismo**: o agente `mcp/` pode começar esta fase logo após a Fase 2, escrevendo as
ferramentas contra `contracts/mcp-ferramentas.md` e o `ClienteMotor`; os testes de cada grupo só
passam quando as rotas correspondentes do motor existirem (US1 → conversas; US2 → leads; US3 →
disparos; US4 → reagir/editar/apagar; US5 → contatos/etiquetas/templates; US6 → status).

**Goal**: todas as ações do usuário expostas como ferramentas MCP; abrir o app quando fechado.

**Independent Test**: com o app fechado, pedir ao Claude Code para importar 10 leads (3 já
existentes) e disparar; o app abre, o retorno lista os duplicados com data e o disparo começa.

### Tests for User Story 7 (mcp/) ⚠️ escrever primeiro

- [X] T132 [P] [US7] Testes de contas/sistema e leads: `listar_contas`, `status_zapdesk`; `importar_leads` com 10 telefones (3 existentes) → 7 novos e 3 "já existia" com data; `caminho_arquivo` CSV; `listar_leads` em mcp/tests/leads.test.ts
- [X] T133 [P] [US7] Testes de conversas e mensagens: `listar_conversas`, `ler_mensagens` por telefone, `buscar_mensagens`, `enviar_mensagem` (texto e anexo), `reagir_mensagem`, `editar_mensagem`, `apagar_mensagem`, `marcar_como_lida`, `ver_status` em mcp/tests/conversas.test.ts
- [X] T134 [P] [US7] Testes de contatos, etiquetas e templates (CRUD, `atualizar_contato` com notas e etiquetas, erro de conflito com `isError`) em mcp/tests/organizacao.test.ts
- [X] T135 [P] [US7] Testes de disparos: `criar_disparo` começa na hora com `origem: "mcp"`; variáveis faltando → `isError` com lista; `valores_padrao`; fila ("Na fila"); `pausar_disparo`/`retomar_disparo`/`cancelar_disparo`; `ver_disparo`; `exportar_relatorio` grava CSV em mcp/tests/disparos.test.ts
- [X] T136 [P] [US7] Teste "app fechado": sem `runtime.json` → abrir injetado sobe o motor falso → ferramenta conclui; sem subida em 30 s → "Não consegui ligar o ZapDesk" em mcp/tests/app-fechado.test.ts

### Implementation for User Story 7 — mcp/

- [X] T137 [P] [US7] Ferramentas `listar_contas` e `status_zapdesk` em mcp/src/ferramentas/contas.ts
- [X] T138 [P] [US7] Ferramentas `importar_leads` (lista ou `caminho_arquivo`, sempre `origem: "mcp"`, texto-resumo "N novos, M já existiam…") e `listar_leads` em mcp/src/ferramentas/leads.ts
- [X] T139 [P] [US7] Ferramentas `listar_conversas`, `ler_mensagens`, `buscar_mensagens`, `enviar_mensagem`, `reagir_mensagem`, `editar_mensagem`, `apagar_mensagem`, `marcar_como_lida` em mcp/src/ferramentas/conversas.ts
- [X] T140 [P] [US7] Ferramentas `listar_contatos`, `atualizar_contato`, `listar_etiquetas`, `criar_etiqueta`, `atualizar_etiqueta`, `excluir_etiqueta` em mcp/src/ferramentas/contatos.ts e mcp/src/ferramentas/etiquetas.ts
- [X] T141 [P] [US7] Ferramentas `listar_templates`, `criar_template`, `atualizar_template`, `excluir_template` em mcp/src/ferramentas/templates.ts
- [X] T142 [P] [US7] Ferramentas `criar_disparo` (descrição avisa que começa na hora, sem confirmação, e cita `pausar_disparo`/`cancelar_disparo`), `listar_disparos`, `ver_disparo`, `iniciar_disparo`, `pausar_disparo`, `retomar_disparo`, `cancelar_disparo`, `exportar_relatorio` em mcp/src/ferramentas/disparos.ts
- [X] T143 [P] [US7] Ferramenta `ver_status` em mcp/src/ferramentas/status.ts
- [X] T144 [US7] Registrar todas as ferramentas com `annotations` (`readOnlyHint` nas leituras; `destructiveHint`/`openWorldHint` em envio, disparo e apagar) e teste de paridade que compara `listTools()` com a lista de contracts/mcp-ferramentas.md em mcp/src/servidor.ts e mcp/tests/paridade.test.ts

### Implementation for User Story 7 — app/

- [X] T145 [US7] Ajustes › "Usar com Claude": comando `claude mcp add -s user …` e bloco JSON do Claude Desktop com os caminhos reais do app (via IPC), botões copiar em app/src/renderer/telas/Ajustes.tsx e app/src/main/ipc.ts
- [X] T146 [US7] Incluir `mcp/dist/zapdesk-mcp.mjs` em `extraResources` (`mcp/`) e manter o fuse `RunAsNode` ligado em app/electron-builder.yml

**Checkpoint**: ciclo IA completo (quickstart cenário 9).

---

## Phase 10: Polish & Cross-Cutting Concerns

**Diretórios**: raiz (`scripts/`), `motor/`, `app/`, `mcp/`, `specs/`

- [X] T147 Script de empacotamento: motor `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64` → `app/resources/bin/zapdesk-motor`, build do MCP, electron-builder `dmg` arm64 com `extraResources` (`bin/`, `mcp/`), bundle id `com.gabriel.zapdesk`; conferir `codesign -dv` do binário extra em scripts/empacotar.sh e app/electron-builder.yml
- [X] T148 [P] Testes de desempenho com motor falso: subida até `pronto` < 2 s; abrir conversa com 50.000 mensagens < 500 ms; importação de 1.000 linhas < 5 s; criar e listar disparo com 50.000 destinatários em motor/testes/desempenho/desempenho_test.go
- [X] T149 [P] Script de verificação de `ps` limpo e memória ociosa (abre o app empacotado, espera 5 min, mede RSS do motor < 80 MB, fecha e confere `pgrep -fl -i zapdesk` vazio) em scripts/verificar-leveza.sh
- [X] T150 [P] Testes de segurança: bind só em `127.0.0.1`, Host, token em tempo constante, `runtime.json` 0600 (lado app), log sem conteúdo de mensagens após envio/recebimento em motor/testes/integracao/seguranca_test.go
- [X] T151 [P] Revisão de textos pt-BR da interface e das mensagens de erro da API (copy crítico de spec.md) em app/src/renderer/ e motor/internal/api/erros.go
- [ ] T152 Executar quickstart.md completo (cenários 1–11) com conta real e registrar resultados na seção "Cenários de validação" de specs/001-zapdesk-mvp/quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Fase 1 (Setup)**: sem dependências.
- **Fase 2 (Fundação)**: depende da Fase 1; bloqueia todas as histórias. Blocos `motor/`,
  `app/` e `mcp/` em paralelo.
- **Fases 3–9 (histórias)**: dependem da Fase 2. Ordem de prioridade: US1 → US2 → US3 (P1) →
  US4 → US5 → US6 → US7 (P2).
- **Fase 10 (Polimento)**: depois das histórias desejadas.

### User Story Dependencies

- **US1**: só Fundação.
- **US2**: só Fundação (usa `telefone`, da Fundação). Importar por etiquetas só tem efeito
  quando existirem etiquetas (US5); sem elas a aba lista apenas contatos.
- **US3**: depende de US2 (destinatários são leads; reutiliza importador e `RelatorioImportacao`)
  e de US1 (contas conectadas e envio de texto). Escolha de template no assistente chega em US5
  (T126).
- **US4**: depende de US1 (chat) e reutiliza `arquivos` e `EnviarMidia` de US3 (T091, T092).
- **US5**: depende de US1 (contatos, composer).
- **US6**: depende de US1 (recebimento de mensagens).
- **US7**: código pode começar após a Fase 2; testes de cada grupo dependem das rotas da
  história correspondente; fecha por último.

### Within Each User Story

- Testes primeiro (devem falhar) → repositórios → serviços/adaptador → handlers → telas.
- No `app/`, telas podem ser feitas contra o contrato com `ClienteMotor` simulado antes de o
  motor terminar; a integração real acontece no checkpoint da fase.

### Divisão por agente (diretório)

| Fase | motor/ | app/ | mcp/ | raiz/compartilhado |
|------|--------|------|------|--------------------|
| 1 Setup | 1 (T002) | 1 (T004) | 1 (T005) | 9 (T001, T003, T006–T012) |
| 2 Fundação | 20 (T013–T032) | 7 (T033–T039) | 4 (T040–T043) | 0 |
| 3 US1 | 12 (T044–T055) | 11 (T056–T066) | 0 | 0 |
| 4 US2 | 8 (T067–T074) | 4 (T075–T078) | 0 | 0 |
| 5 US3 | 15 (T079–T093) | 10 (T094–T103) | 0 | 0 |
| 6 US4 | 7 (T104–T110) | 6 (T111–T116) | 0 | 0 |
| 7 US5 | 4 (T117–T120) | 7 (T121–T127) | 0 | 0 |
| 8 US6 | 3 (T128–T130) | 1 (T131) | 0 | 0 |
| 9 US7 | 0 | 2 (T145–T146) | 13 (T132–T144) | 0 |
| 10 Polimento | 2 (T148, T150) | 0 | 0 | 4 (T147, T149, T151, T152)* |
| **Total** | **72** | **49** | **18** | **13** |

\* T147 também edita `app/electron-builder.yml`; T151 também edita `app/` e `motor/`; T152 é
validação manual.

### Parallel Opportunities

- Fase 1: T002–T008 e T011 em paralelo.
- Fase 2: os três blocos por diretório em paralelo; dentro do motor, T013–T016, T020, T021, T023
  em paralelo.
- Em cada história: todos os testes `[P]` juntos; no `app/`, telas `[P]` juntas.
- Entre histórias: com o contrato fixo, `app/` pode adiantar telas de US2/US3 enquanto `motor/`
  termina US1; `mcp/` pode fazer toda a US7 em paralelo às fases 3–8.

---

## Parallel Example: User Story 3

```bash
# motor/ — testes de domínio juntos:
Task: "T079 Testes de variáveis em motor/internal/variaveis/variaveis_test.go"
Task: "T080 Testes de agendamento em motor/internal/agendamento/agendamento_test.go"
Task: "T081 Testes das máquinas de estado em motor/internal/disparos/estados_test.go"
Task: "T082 Testes do executor em motor/internal/disparos/executor_test.go"

# motor/ — funções puras juntas:
Task: "T084 Pacote de variáveis em motor/internal/variaveis/variaveis.go"
Task: "T085 ProximoEnvio/Estimativa em motor/internal/agendamento/agendamento.go"

# app/ — em paralelo ao motor, contra o contrato:
Task: "T094 Lista de Disparos em app/src/renderer/telas/Disparos.tsx"
Task: "T102 Testes do ciclo de vida em app/tests/main/ciclo-vida.test.ts"
```

---

## Implementation Strategy

### MVP First (US1 → US3)

1. Fase 1 + Fase 2 (contrato tipado + motor falso + ciclo do app + base do MCP).
2. US1 → validar (conectar, conversar, `ps` limpo).
3. US2 → validar importação de 1.000 linhas.
4. US3 → validar disparo idempotente e barra de menu. **Parar e validar o MVP.**

### Incremental Delivery

US4 (mídia) → US5 (organização) → US6 (status) → US7 (MCP, desenvolvido em paralelo desde a
Fase 2 e fechado por último) → Polimento e `.dmg`.

### Parallel Team Strategy (agentes por diretório)

1. Agente raiz/compartilhado faz a Fase 1; em seguida, três agentes: `motor/`, `app/`, `mcp/`.
2. Cada agente segue as fases na ordem, pegando apenas as tarefas do seu diretório.
3. Mudança de contrato: atualizar `contracts/` e `compartilhado/cliente-motor` primeiro, avisar
   os outros agentes, depois implementar.

---

## Notes

- `[P]` = arquivos diferentes, sem dependência pendente.
- Tarefas que tocam o mesmo arquivo em fases diferentes (ex.: `Composer.tsx` em T059, T112, T125;
  `mensagens.go` em T055, T110) são sequenciais por fase.
- Verificar que testes falham antes de implementar; commit por tarefa ou grupo lógico.
