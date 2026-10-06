# Pesquisa (Fase 0) — ZapDesk MVP

Data da pesquisa: 2026-09-27. Método: leitura do código-fonte do `tulir/whatsmeow` em HEAD,
instalação dos pacotes npm e execução de testes pequenos em pasta temporária, `claude mcp add
--help` local. "Não confirmado" indica o que não foi possível verificar em fonte primária.

## 1. whatsmeow (motor WhatsApp)

### Decisão: `go.mau.fi/whatsmeow` em pseudo-versão fixada no `go.mod`

- **Estado atual**: o projeto não publica tags; só pseudo-versões (mais recente em 2026-09-27:
  `v0.0.0-20260927171547-45cfce066cd2`). `go.mod` exige `go 1.26.0` (toolchain 1.27.1) → Go 1.27
  do Homebrew atende. Dependências: `go.mau.fi/util v0.10.1`, `zerolog v1.35.1`,
  `protobuf v1.36.12`.
- **Rationale**: único motor Go maduro do protocolo multi-device; leve e embutível.
- **Alternativas**: Evolution API (descartada: servidor sempre ligado + Docker/Postgres/Redis),
  Baileys (Node, duplicaria o motor).
- **Consequência**: fixar a pseudo-versão e atualizar conscientemente; todo uso fica no pacote
  adaptador (Constituição IV).
- Fonte: https://proxy.golang.org/go.mau.fi/whatsmeow/@latest, https://github.com/tulir/whatsmeow

### Decisão: sessões em `sqlstore` com driver puro Go `modernc.org/sqlite`, um banco por conta

- **Confirmado (execução)**: driver `"sqlite"`, dialect `"sqlite"`; DSN obrigatório com
  `_pragma=foreign_keys(1)` (sem ele `sqlstore.New` falha com "foreign keys are not enabled"),
  mais `_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)`. A sintaxe `_foreign_keys=on` é do
  `mattn/go-sqlite3` e NÃO funciona no modernc. `CGO_ENABLED=0` compila para darwin/arm64
  (binário ~24 MB).
- **API**: `sqlstore.New(ctx, dialect, address, log) (*Container, error)`;
  `container.GetAllDevices(ctx)`, `GetFirstDevice(ctx)`, `GetDevice(ctx, jid)`, `NewDevice()`
  (sem ctx; só persiste após parear), `PutDevice`, `DeleteDevice`, `Close()`.
- **Multi-conta**: um `*whatsmeow.Client` por `*store.Device` (`whatsmeow.NewClient(dev, log)`).
  Seguimos o documento de produto ("sessão do whatsmeow em banco próprio por conta"): um
  `Container` por conta em `sessoes/<conta_id>.db`; na partida, abrir cada arquivo de conta e
  `GetFirstDevice`. Remover conta = `Logout` + `DeleteDevice` + apagar arquivo.
- **Alternativa**: um container único com vários devices (padrão das bridges) — rejeitado apenas
  para seguir o documento e simplificar a remoção de conta; custo extra de memória por conta é
  pequeno (um `*sql.DB` por arquivo).
- Fonte: https://github.com/tulir/whatsmeow/blob/main/store/sqlstore/container.go

### Decisão: login por QR via `GetQRChannel`

- **API**: `GetQRChannel(ctx) (<-chan QRChannelItem, error)` antes de `Connect()` (sem ctx) e só se
  `client.Store.ID == nil`. `QRChannelItem{Event, Code, Timeout, Error}`; eventos `code`,
  `success`, `timeout`, `error`, `err-unexpected-state`, `err-client-outdated`,
  `err-scanned-without-multidevice`, `passkey-request`, `passkey-confirmation`.
- O motor repassa cada `code` como evento WS `conta.qr`; `timeout` → conta volta a `desconectada`
  com opção de gerar novo QR. Pareamento por código de telefone (`PairPhone`) fica fora do MVP.
- Fonte: https://github.com/tulir/whatsmeow/blob/main/qrchan.go

### Decisão: envio com `SendMessage` e protobuf `waE2E`

- **API**: `SendMessage(ctx, to types.JID, msg *waE2E.Message, extra ...SendRequestExtra)
  (SendResponse, error)`; `SendResponse{Timestamp, ID, ServerID, Sender, Chat, ...}`.
  Pacote `go.mau.fi/whatsmeow/proto/waE2E` (o antigo `binary/proto` é alias legado).
- **Mídia**: `Upload(ctx, dados, MediaImage|MediaVideo|MediaAudio|MediaDocument)`
  e `UploadReader` para arquivos grandes; `UploadResponse{URL, DirectPath, MediaKey,
  FileEncSHA256, FileSHA256, FileLength}` preenchem `ImageMessage`/`VideoMessage`/
  `AudioMessage`/`DocumentMessage`/`StickerMessage` (campos ponteiros via `proto.String` etc.).
- **Áudio de voz**: `AudioMessage{PTT: true, Mimetype: "audio/ogg; codecs=opus"}` → o app grava em
  Opus/OGG (MediaRecorder `audio/ogg;codecs=opus` ou `audio/webm;codecs=opus` convertido no
  motor — ver decisão 6).
- **Figurinha**: `StickerMessage` (webp) enviada com `MediaImage`.
- **Citação**: `ExtendedTextMessage{Text, ContextInfo{StanzaID, Participant, QuotedMessage}}`.
- **Reação/edição/revogação**: `BuildReaction(chat, sender, id, emoji)`, `BuildEdit(chat, id,
  novo)`, `BuildRevoke(chat, sender, id)`, enviados com `SendMessage`.
- **Prazos**: `EditWindow = 20 min` na biblioteca; o WhatsApp anuncia 15 min → a UI usa 15 min.
  Apagar para todos: prazo **não confirmado** em fonte oficial (≈ 60 h segundo a comunidade) → a
  UI usa 48 h, conservador, configurável num único lugar.
- Fonte: https://github.com/tulir/whatsmeow/blob/main/send.go, `upload.go`

### Decisão: download de mídia sob demanda

- **API**: `Download(ctx, msg DownloadableMessage)`, `DownloadAny(ctx, *waE2E.Message)`,
  `DownloadMediaWithPath(...)`; mídia expirada → `SendMediaRetryReceipt(ctx, info, mediaKey)`.
- O motor guarda os dados de download (direct path, chaves, hashes) em `mensagens.midia` e baixa
  só quando a interface pede; resultado vai para `midia/<conta_id>/` (cache).

### Decisão: eventos tratados

- `AddEventHandler(func(evt any)) uint32`. Tipos usados (`types/events`): `Message`, `Receipt`
  (`ReceiptTypeDelivered` = `""`, `Read`, `ReadSelf`, `Played`), `HistorySync`, `Connected`,
  `Disconnected`, `LoggedOut`, `TemporaryBan`, `StreamReplaced`, `ConnectFailure`,
  `ClientOutdated`, `KeepAliveTimeout`, `OfflineSyncCompleted`, `PushName`, `Contact`.
- **HistorySync**: `evt.Data.GetConversations()` → `conv.GetID()` → `types.ParseJID` →
  `conv.GetMessages()` → `m.GetMessage()` (`*waWeb.WebMessageInfo`) →
  `cli.ParseWebMessage(chatJID, webMsg)` → mesmo caminho de gravação de `events.Message`.
- **Mapeamento de estados de conta**: `Connected` → `conectada`; `LoggedOut` → `desconectada`;
  `TemporaryBan`/`ConnectFailure` com motivo de ban → `banida`; `Disconnected` (rede) → mantém
  `conectada` + indicador "Sem conexão" (auto-reconnect já vem ligado:
  `EnableAutoReconnect = true`); `StreamReplaced` → `desconectada`.

### Decisão: identidade por telefone apesar de LID

- **Confirmado**: chats podem usar LID (`@lid`) em vez do número. `types.MessageSource` tem
  `AddressingMode`, `SenderAlt`, `RecipientAlt`; `Store.LIDs.GetPNForLID(ctx, lid)` /
  `GetLIDForPN`.
- **Decisão**: o adaptador normaliza todo remetente/conversa individual para o JID de telefone
  (`@s.whatsapp.net`) usando `SenderAlt` ou o `LIDStore`; o domínio nunca vê LID quando o
  telefone é conhecido. Isso é essencial para ligar contato ↔ lead e detectar "respondeu".
- Se o telefone não puder ser resolvido, o contato fica com `telefone = nulo` e não casa com lead.

### Decisão: verificação de WhatsApp individual

- `IsOnWhatsApp(ctx, phones []string) ([]types.IsOnWhatsAppResponse, error)` com `+55...`;
  retorno `{Query, JID, IsIn, PhoneNumber, VerifiedName}`. Limite de lote **não confirmado**.
- Usado com 1 número por vez, imediatamente antes de cada envio do disparo e em "Nova conversa"
  (clarificação da spec). Sem verificação em lote no MVP.

### Outras APIs usadas

- Status: mensagens com `Info.Chat == types.StatusBroadcastJID` (`status@broadcast`).
- Grupos: `GetJoinedGroups(ctx)`, `GetGroupInfo(ctx, jid)`. Contatos:
  `cli.Store.Contacts.GetAllContacts(ctx)`.
- Leitura: `MarkRead(ctx, ids, ts, chat, sender)`. Presença: `SendPresence(ctx, types.
  PresenceAvailable)` logo após conectar (senão o nome aparece como "-").
- Logs: `waLog.Zerolog(logger)` para unificar com o log do motor.
- Quase todos os métodos recebem `context.Context` (exceto `Connect`/`Disconnect`).
- Consumo de memória: **não confirmado** → medir na fase de polimento (meta < 80 MB ocioso);
  mitigação: não manter mídia em memória, `UploadReader` para arquivos grandes, `GOMEMLIMIT`
  configurado (ex.: 64 MiB) e `debug.FreeOSMemory` após history sync.

## 2. Banco de dados do app

### Decisão: SQLite puro Go (`modernc.org/sqlite`) + `database/sql`, SQL escrito à mão

- **Rationale**: build sem CGO, mesmo driver das sessões, migrações simples em `.sql` embutidas
  via `embed`. FTS5 disponível no modernc para a busca de mensagens.
- **Alternativas**: `mattn/go-sqlite3` (exige CGO, complica cross-compile), ORMs (peso e magia
  desnecessários).
- Pragmas: `foreign_keys(1)`, `journal_mode(WAL)`, `busy_timeout(5000)`; uma conexão de escrita
  (`SetMaxOpenConns` separado para leitura/escrita) para evitar `SQLITE_BUSY`.

## 3. Telefones

### Decisão: `github.com/nyaruka/phonenumbers` v1.8.1

- `phonenumbers.Parse(n, "BR")` + `IsValidNumber` + `Format(num, phonenumbers.E164)`.
- Pré-limpeza própria (dígitos, `00` → `+`) antes do parse; DDI padrão configurável (padrão BR).
- Alternativa: regex própria (rejeitada: erra nono dígito e DDDs; biblioteca é a referência da
  libphonenumber).

## 4. Importação CSV/XLSX

### Decisão: parse no motor (`encoding/csv` + `github.com/xuri/excelize/v2` v2.11.0)

- **Rationale**: normalização e deduplicação ficam num único lugar (motor = fonte da verdade);
  o MCP pode importar arquivo por caminho com a mesma lógica; evita o SheetJS, que não é mais
  publicado no npm (npm parado em 0.18.5).
- CSV: detecção de separador (`,` `;` `\t`) e de BOM UTF-8; XLSX: primeira planilha
  (`OpenReader` + `GetRows`). O arquivo é lido sob demanda e descartado após a importação; o
  custo de memória do excelize só existe durante a importação.
- Fluxo: `previa` (colunas, amostra, sugestão da coluna de telefone) → `importar` com
  mapeamento. Ver `contracts/api-http.md`.

## 5. API local do motor

### Decisão: HTTP/JSON (`net/http` com o roteador padrão do Go 1.22+) + WebSocket

- Roteamento com padrões `GET /v1/contas/{id}` do `net/http` (sem framework).
- WebSocket: `github.com/coder/websocket` (sucessor do nhooyr.io/websocket; sem dependências,
  suporta `context`). Alternativa `gorilla/websocket` (arquivado/manutenção irregular).
- Bind `127.0.0.1:0` (porta aleatória); o motor informa a porta ao app numa linha JSON no stdout
  (`{"evento":"pronto","porta":N,"versao":"..."}`).
- Token: gerado pelo app (32 bytes aleatórios, base64url) e passado ao motor pela variável de
  ambiente `ZAPDESK_TOKEN` (não aparece em `ps`). Requisições com `Authorization: Bearer <token>`;
  WebSocket e URLs de mídia aceitam `?token=` (o navegador não envia cabeçalho em `<img>`/WS).
  Comparação em tempo constante. Checagem de `Host` = `127.0.0.1:<porta>` contra DNS rebinding.
- `runtime.json` escrito pelo app com `0600`, apagado ao encerrar.

## 6. Ciclo de vida

### Decisão: motor como processo filho com vigia de stdin

- O app (processo principal do Electron) inicia `zapdesk-motor` com `stdio: ['pipe', 'pipe',
  'pipe']`. O motor encerra graciosamente se o stdin fechar (app morreu) ou ao receber SIGTERM;
  o app manda SIGTERM ao sair e SIGKILL após 5 s. Assim nenhum processo sobra (Constituição II).
- Trava de instância única: `app.requestSingleInstanceLock()` no app e `motor.lock` (flock) no
  motor.
- Queda do motor: o app religa uma vez; disparos voltam como `pausado` (`motor_reiniciado`).
- Disparo ativo ao fechar: `window-all-closed`/`before-quit` interceptados; janela escondida,
  `app.dock.hide()`, `Tray` com progresso; ao evento `disparo.atualizado` final sem outros ativos
  → `Notification` com resumo → `app.quit()`.
- Sono do Mac: `powerMonitor` (`suspend`/`resume`) no app avisa o motor
  (`POST /v1/sistema/energia`), que recalcula a agenda; o agendador também é robusto por conta
  própria (usa relógio de parede e regra 6 do cálculo em `data-model.md`).
- Conversão de áudio gravado: o Chromium grava `audio/webm;codecs=opus`; o WhatsApp espera
  `audio/ogg; codecs=opus` para mensagem de voz. Decisão: o renderer grava com `MediaRecorder`
  (WebM/Opus) e o motor faz remux (sem recodificar) WebM→OGG em Go puro, lendo os blocos Opus com
  `github.com/at-wat/ebml-go` e escrevendo com `github.com/pion/webrtc/v4/pkg/media/oggwriter`.
  Alternativa: encoder WASM no renderer (`opus-media-recorder`) — rejeitada por peso e manutenção.
  Risco (não confirmado na prática): validar na tarefa de áudio; fallback = enviar como áudio
  comum (não-PTT).
  **Atualização (implementação)**: o remux foi escrito em Go puro no próprio motor
  (`motor/internal/arquivos/audio.go`, ~250 linhas: leitor EBML + páginas Ogg com CRC) em vez de
  `ebml-go` + `oggwriter`, que exigiria o módulo `pion/webrtc` e pacotes RTP. Testado com WebM
  sintético no formato do MediaRecorder; não validado com áudio real no celular.

## 7. App desktop

### Decisão: Electron 44 + React + TypeScript + Vite + electron-builder 26

- Versões atuais: Electron 44.4.5, electron-builder 26.15.3.
- Empacotamento: `mac.target = [{target: "dmg", arch: ["arm64"]}]`; `extraResources`: binário
  do motor (`bin/zapdesk-motor`) e o bundle do MCP (`mcp/zapdesk-mcp.mjs`); caminho em runtime via
  `process.resourcesPath`. Assinatura/notarização: fora do MVP (ad-hoc; "Abrir mesmo assim").
- Segurança: `contextIsolation: true`, `nodeIntegration: false`, `sandbox: true`, preload mínimo.
- Estrutura de build: `electron-vite` (config única para main/preload/renderer) — alternativa
  Vite puro + tsc para o main (mais configuração manual).
- UI: React 19 + React Router; estado de servidor via TanStack Query alimentado por eventos WS
  (invalidação/atualização otimista); listas longas com `@tanstack/react-virtual`; ícones
  `lucide-react`; tema claro/escuro por `prefers-color-scheme`. Sem biblioteca de componentes
  pesada.
- Testes: Vitest + Testing Library no renderer; testes do main com Vitest (lógica de ciclo de
  vida isolada em módulos puros).

## 8. Servidor MCP

### Decisão: SDK MCP TypeScript v2 (`@modelcontextprotocol/server` 2.1.0) + zod ^4.2

- **Estado atual**: `@modelcontextprotocol/sdk` 1.30.1 é a linha v1 (legado, correções por ≥ 6
  meses); a v2 (estável desde 2026-07-27, spec MCP 2026-07-28) foi dividida em
  `@modelcontextprotocol/server`, `/client`, `/core`. v2 exige zod `^4.2.0`, Node ≥ 20, ESM.
- **Decisão**: usar a v2 (sucessora direta do `@modelcontextprotocol/sdk` pedido no documento).
  Se houver incompatibilidade com o Claude Desktop instalado, cair para
  `@modelcontextprotocol/sdk@1.30` — a API `registerTool` é equivalente.
- **API confirmada (execução, Node 22)**:
  `import { McpServer, InMemoryTransport } from '@modelcontextprotocol/server'`;
  `import { StdioServerTransport, serveStdio } from '@modelcontextprotocol/server/stdio'`;
  `server.registerTool(nome, {title, description, inputSchema: z.object({...}), outputSchema,
  annotations}, async (args, ctx) => ({content:[{type:'text', text}], structuredContent,
  isError}))`. `.tool()` foi removido na v2.
- **Testes**: `InMemoryTransport.createLinkedPair()` + `Client` de `@modelcontextprotocol/client`
  + `callTool({name, arguments})` contra um motor real rodando com cliente falso.
- **Empacotamento**: esbuild → `zapdesk-mcp.mjs` único em `extraResources`; executado com o
  próprio executável do app e `ELECTRON_RUN_AS_NODE=1` (o fuse `RunAsNode` DEVE continuar ligado).
  Alternativa considerada: MCP em Go dentro do binário do motor (mais robusto, sem Node) —
  rejeitada porque o documento pede MCP em TypeScript; fica como plano B.
- **Configuração** (tela Ajustes mostra os comandos prontos):
  - Claude Code: `claude mcp add -s user -e ELECTRON_RUN_AS_NODE=1 zapdesk --
    "/Applications/ZapDesk.app/Contents/MacOS/ZapDesk"
    "/Applications/ZapDesk.app/Contents/Resources/mcp/zapdesk-mcp.mjs"`.
  - Claude Desktop: `~/Library/Application Support/Claude/claude_desktop_config.json` →
    `{"mcpServers":{"zapdesk":{"command":"<ZapDesk>","args":["<zapdesk-mcp.mjs>"],
    "env":{"ELECTRON_RUN_AS_NODE":"1"}}}}` (formato conhecido; não refetchado hoje).
- **Abrir o app**: se `runtime.json` não existir, o `pid_motor` não estiver vivo ou `GET
  /v1/saude` falhar → `open -b <bundle id>` (fallback `open -a ZapDesk`), sondar `runtime.json` +
  `/v1/saude` a cada 250 ms por até 30 s; senão erro "Não consegui ligar o ZapDesk".

## 9. Monorepo e ferramentas

### Decisão: workspaces npm na raiz + módulo Go independente em `motor/`

- `package.json` raiz com workspaces `app`, `mcp`, `compartilhado/cliente-motor`.
- `compartilhado/cliente-motor` (`@zapdesk/cliente-motor`): tipos TypeScript do contrato + cliente
  HTTP/WS (fetch + WebSocket nativo do Node 22 e do navegador). Usado pelo app e pelo MCP para
  não duplicar o contrato. Justificado em Complexity Tracking do plano.
- Motor: `go 1.27`, `go test ./...`, `golangci-lint` opcional; binário `CGO_ENABLED=0 GOOS=darwin
  GOARCH=arm64`.
- CI: GitHub Actions com `go test`, `npm test` e `tsc --noEmit` (runner macOS para o build .dmg
  opcional/manual).
- Modo falso: `zapdesk-motor --whatsapp=falso` (ou `ZAPDESK_WHATSAPP=falso`) liga o cliente em
  memória com endpoints de controle `/v1/falso/*` (simular mensagem recebida, recibo, QR
  escaneado, ban, queda) — disponíveis SOMENTE nesse modo.

## Itens não confirmados (acompanhar na implementação)

| Item | Risco | Mitigação |
|------|-------|-----------|
| Prazo exato de "apagar para todos" | baixo | 48 h configurável |
| Limite de lote do `IsOnWhatsApp` | baixo | consulta individual |
| Memória ociosa do whatsmeow | médio | medir em T de polimento; `GOMEMLIMIT` |
| Remux WebM→OGG em Go puro | médio | fallback: enviar como documento de áudio |
| Assinatura do binário extra no .dmg | baixo | `codesign -dv` na tarefa de empacotamento |
