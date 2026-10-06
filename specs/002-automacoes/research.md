# Pesquisa (Fase 0) — Automações

Fonte do produto: `docs/features/automacoes.md` (§7 aspectos técnicos, §14 decisões). Pesquisa
externa feita em 2026-09-27 (documentação oficial + testes locais no scratchpad com Electron 42 /
Node 24.18.1 como `ELECTRON_RUN_AS_NODE` e esbuild 0.28.2 como biblioteca Go em go1.27.1). Itens
marcados **[testado]** foram verificados localmente; **[não verificado]** ficam como risco.

Cada item: **Decisão**, **Razão**, **Alternativas consideradas**.

---

## R1. Onde fica o motor de eventos

- **Decisão**: no motor Go, pacote `internal/automacoes/...`: gatilhos, condições, executor de
  fluxo, sessões de chatbot, esperas/agendamentos, anti-loop, pausas, execuções e orquestração dos
  runners. Executores determinísticos com o `relogio.Relogio` injetável que o MVP já usa.
- **Razão**: Constituição III (motor dono dos dados e agendadores); app e MCP veem o mesmo estado;
  o relógio controlável do modo falso (`PUT /v1/falso/relogio`) já permite testar esperas sem
  `sleep`.
- **Alternativas**: executor em TypeScript no app (quebraria o MCP com app minimizado e duplicaria
  estado); JS embutido no Go com goja (rejeitado na §14: o usuário quer TypeScript de verdade,
  `async/await` e `fetch`).

## R2. Ganchos internos de eventos (fonte dos gatilhos)

- **Decisão**: ouvintes síncronos e não bloqueantes registrados nos serviços existentes, no mesmo
  padrão do `chat.Ouvinte` usado pelos disparos: `chat` (mensagem recebida/enviada, com indicação
  de manual), `organizacao` (etiqueta adicionada/removida — diferença do conjunto em
  `DefinirEtiquetas`), `leads` (leads novos de uma importação), `disparos` (destinatário
  `respondeu`) e o novo `funil` (entrou na etapa). O ouvinte só enfileira o fato numa fila interna
  do despachante (canal com buffer + fila por conversa); nunca executa ação dentro do handler do
  WhatsApp.
- **Razão**: o `eventos.Barramento` descarta eventos para assinantes lentos (documentado no
  pacote) — aceitável para a UI, inaceitável para gatilhos. O padrão `AdicionarOuvinte` já existe.
- **Alternativas**: assinar o barramento (perde eventos); tabela de eventos persistida (mais
  robusta, mas desnecessária: se o motor cair, a mensagem já está gravada e o gatilho de mensagem
  perdido é aceito; esperas e agendamentos, que importam, são persistidos — R9).

## R3. Compilação do TypeScript das automações de IA

- **Decisão**: esbuild embutido no **motor** como biblioteca Go (`github.com/evanw/esbuild/pkg/api`,
  versão fixada **v0.28.2**, a atual). `api.Build` com `EntryPoints: [index.ts]`, `Bundle: true`,
  `Write: false`, `Platform: node`, `Format: esm`, `Target: es2022`, `Sourcemap: inline` (para stack
  com linha do `.ts`), plugin `OnResolve` com lista de permissões (R6) e `External` para
  `@zapdesk/automacao`. Saída gravada em `<pasta-dados>/automacoes-compiladas/<id>/<hash>.mjs`.
  Hash = SHA-256 do conteúdo de todos os arquivos do projeto + versão do compilador.
- **Razão**: **[testado]** a API Go compila, aceita plugins `OnResolve/OnLoad` e devolve
  `Errors` com arquivo/linha/coluna; `import fs from 'fs'` virou erro "módulo não permitido: fs" e
  `@zapdesk/automacao` foi preservado como externo. Elimina binário nativo dentro do asar,
  `asarUnpack`, `ESBUILD_BINARY_PATH` e a questão arm64/x64 (o motor já é compilado por
  arquitetura). Compila sem o app (MCP e testes Go). Custo: **+5,3 MB** no binário do motor por
  arquitetura (2,48 → 7,80 MB num binário mínimo), sem custo de RAM ociosa. `pkg/api` é a API
  documentada para uso como biblioteca; versões 0.x podem quebrar em minors → versão exata fixada.
- **Alternativas**: pacote npm `esbuild` no processo principal do Electron (precisa ficar
  `external` no bundle do electron-vite, binário fora do asar via `asarUnpack:
  ["**/node_modules/@esbuild/**"]`, os dois binários para universal e `ESBUILD_BINARY_PATH`
  definido antes do `require`; e o MCP dependeria do app para compilar); `esbuild-wasm` (14,5 MB e
  ~10× mais lento, segundo a própria documentação).

## R4. Verificação de tipos

- **Decisão**: verificação de tipos só no editor do app, pelo worker TypeScript do Monaco (TS 5.9.3
  embutido), com os tipos da SDK carregados via `addExtraLib`. A compilação do motor (e portanto o
  MCP) informa erros de sintaxe, importação, resolução e do manifesto. O MCP oferece
  `ver_tipos_sdk` para a IA consultar a API.
- **Razão**: esbuild não checa tipos ("run `tsc -noEmit`"). Embarcar `typescript` custaria ~24 MB
  (6.0.3, JS) ou ~27 MB por plataforma (7.0.2, nativo) e um processo extra. O Monaco já traz o
  checador no worker.
- **Alternativas**: `typescript` no runner ou no processo principal (peso e latência); `tsgo`
  nativo como binário extra (peso, empacotamento).

## R5. Runner das automações de IA

- **Decisão**: novo pacote `automacao/runner` (`@zapdesk/automacao-runner`), empacotado com esbuild
  em `dist/zapdesk-runner.mjs` (mesmo padrão de `mcp/build.mjs`) e executado **pelo motor** com o
  binário do Electron + `ELECTRON_RUN_AS_NODE=1` (Electron 44.4.5 → **Node 24.21.0**). O app passa
  ao motor `--runner-exec <process.execPath>` e `--runner-script <Resources>/runner/zapdesk-runner.mjs`;
  nos testes Go, `node` do PATH (≥ 22.15). Um processo por (automação, hash compilado), iniciado sob
  demanda, encerrado após 5 min ocioso, no máximo 4 vivos (fila global), várias execuções
  concorrentes por processo (conversas diferentes), uma por conversa.
- **Comunicação**: stdio com JSON-RPC 2.0 em linhas (NDJSON), stdout só para protocolo,
  `console.*` redirecionado para notificações `log`; stderr capturado pelo motor (stack de falha
  fatal/OOM). Nenhum token HTTP: o **próprio canal é o escopo** da automação (Constituição III).
- **Razão**: §7 do documento (processo Node por automação, como o MCP); stdio evita porta, token e
  qualquer acesso à API HTTP completa; o motor mata o processo em timeout e, se o motor morrer, o
  stdin do runner fecha e ele sai sozinho (ps limpo).
- **Alternativas**: HTTP local com token de escopo (mais superfície e um token a proteger); um
  processo único para todas as automações (quebra o isolamento pedido); `worker_threads` no
  processo principal do Electron (derrubaria o app em OOM).

## R6. Restrição de módulos e isolamento em camadas

- **Decisão** (protege contra **erro**, não contra código malicioso — documentado no editor e na
  SDK):
  1. **Compilação**: plugin `OnResolve` do esbuild no motor permite só `@zapdesk/automacao`
     (externo), arquivos relativos do próprio projeto (sem sair da pasta) e os módulos Node
     `node:crypto`, `node:util`, `node:url`, `node:buffer`, `node:events`,
     `node:timers/promises`, `node:path/posix`, `node:string_decoder`, `node:querystring`
     (externos). Qualquer outro → erro "Módulo não permitido: <nome>".
  2. **Processo**: `--permission --allow-fs-read=<runner.mjs> --allow-fs-read=<bundle.mjs>` (sem
     `--allow-child-process`, `--allow-worker`, `--allow-addons`, `--allow-wasi`),
     `--max-old-space-size=<memoria_mb>`; ambiente limpo (apenas `ELECTRON_RUN_AS_NODE=1`, `TZ`,
     `LANG`); **[testado]** no Electron 42/Node 24 como node: `fs` fora da lista, `child_process` e
     `Worker` dão `ERR_ACCESS_DENIED`. O Permission Model **não** bloqueia rede no Node 24
     (`--allow-net` só existe no Node 25; "bad option" no 24) → camada 3.
  3. **Tempo de execução**: `module.registerHooks` (Node ≥ 22.15, RC no 24) com `resolve` que mapeia
     `@zapdesk/automacao` para o módulo interno do runner e lança erro para `fs`, `child_process`,
     `net`, `tls`, `dgram`, `http`, `https`, `http2`, `worker_threads`, `cluster`, `vm`,
     `inspector`, `module`, `v8`, `os`, `process` (**[testado]**: lançar no `resolve` bloqueia
     `require("fs")`); antes de importar o bundle, o runner guarda `fetch` para uso interno e
     remove/substitui `globalThis.fetch`, `WebSocket`, `EventSource`, `process.binding`,
     `process.dlopen`, `process.kill`, `process.chdir`, `process.env` (objeto vazio congelado).
  4. **Capacidade**: dados, envio, IA e agenda só via JSON-RPC ao motor, que valida permissões,
     simulação, pausa e anti-loop.
  5. **Tempo**: o motor aplica o prazo por execução e mata o processo (`SIGKILL`) ao estourar;
     tamanho máximo de linha JSON-RPC 1 MB.
- **Razão**: a documentação do Node diz que o Permission Model "does not protect against malicious
  code" e que `node:vm` "is not a security mechanism". Camadas baratas pegam os erros comuns.
- **Alternativas**: `node:vm` (não é segurança e complica `import`); `isolated-vm` (addon nativo,
  exigiria rebuild para Electron e `--allow-addons`); só a lista na compilação (fácil de contornar
  por acidente com `globalThis.process`).
- **Riscos**: `--max-old-space-size` limita o heap do V8, não o RSS exato (**[testado]**: 64 →
  `heap_size_limit` 160 MB); o limite é "aproximado" na UI. `module.registerHooks` é RC no Node 24.

## R7. Claude API (helpers `ctx.ia`)

- **Decisão**: a chamada HTTP é feita **pelo motor Go**, com `net/http` direto (sem SDK), quando o
  runner pede `ctx.ia.*` por JSON-RPC. A chave nunca entra no runner.
  - `POST https://api.anthropic.com/v1/messages`, cabeçalhos `x-api-key`,
    `anthropic-version: 2023-06-01`, `content-type: application/json`.
  - Corpo: `model`, `max_tokens` (padrão 1024, máx. 8192), `system`, `messages`. **Sem
    `temperature`/`top_p`/`top_k`**: Sonnet 5 e Opus 5/5.5 devolvem 400 com valores fora do padrão.
  - `extrair` e `classificar` usam saídas estruturadas GA:
    `output_config: {format: {type: "json_schema", schema}}` — o motor força
    `additionalProperties: false` em todo objeto e rejeita `minimum/maximum/minLength/maxLength` e
    esquemas recursivos com erro claro; `classificar` usa `{"categoria": {"enum": [...]}}`.
  - Resposta: concatena blocos `text`; checa `stop_reason` (`refusal` e `max_tokens` viram erro
    ou aviso claros antes de interpretar JSON); registra `usage.input_tokens`/`output_tokens` e o
    cabeçalho `request-id` no log da execução (não o conteúdo).
  - Retentativas: até 3 em 429 (respeitando `retry-after`), 500, 504, 529 (`overloaded_error`),
    com espera 1 s → 2 s → 4 s limitada pelo prazo da execução; 400/401/403/404/413 não repetem;
    401 → "Chave da Anthropic inválida. Configure em Ajustes → IA".
  - Modelos: padrão `claude-sonnet-5`; opções `claude-opus-5-5` e `claude-haiku-4-5-20251001`
    (alias `claude-haiku-4-5`). O motor aceita qualquer id que comece com `claude-` para não
    travar com lançamentos novos; a UI oferece os três.
  - **IA simulada** (`--ia=falsa`, testes e "IA simulada" no Testar): respostas determinísticas sem
    rede (`gerar` → "[IA simulada] …", `classificar` → primeira categoria, `extrair` → valores
    vazios/nulos válidos para o esquema), tokens 0.
- **Razão**: o Permission Model não bloqueia rede (R6), então chave no runner = chave ao alcance do
  código do usuário (viola FR-086 e a Constituição I). No motor, a chave fica no Go e os tokens são
  contabilizados num só lugar. Cliente HTTP próprio = ~200 linhas, sem dependência nova.
- **Alternativas**: `@anthropic-ai/sdk` 0.128.0 no runner (235 KB empacotado, funciona no
  Electron-as-Node **[testado]**, retries embutidos — mas expõe a chave); `anthropic-sdk-go`
  v1.75.0 no motor (bom, porém dependência grande para 1 endpoint); tool use com `tool_choice`
  forçado para JSON (400 no Opus 5.5).
- **Riscos**: `claude-haiku-4-5-20251001` tem aposentadoria "não antes de 15/10/2026" — a UI mostra
  aviso e o erro 404 de modelo vira mensagem "Modelo indisponível; escolha outro em Ajustes → IA".
  Teto de gasto do tier gera 429 **sem** `retry-after` e não se resolve com retentativa: após a 1ª
  repetição sem `retry-after`, falha com "Limite de gasto da Anthropic atingido".

## R8. Segredos e chave da Anthropic

- **Decisão**: o processo principal do app guarda os segredos cifrados com `safeStorage` (chave no
  Keychain) em `<pasta-dados>/segredos.bin` (0600) e os entrega ao motor pelo **stdin**, numa linha
  JSON de controle, logo após o `pronto` e a cada alteração. O motor os mantém só em memória e
  espera o primeiro envio (até 2 s) antes de iniciar o agendador de automações quando recebe
  `--aguardar-segredos`. A API expõe só nomes (`GET /v1/segredos`); definir/remover só pela janela
  (IPC do app). A chave da Anthropic é o segredo reservado `ANTHROPIC_API_KEY`, usado apenas pelo
  motor. Outros segredos só chegam ao runner se declarados em `automacao.json`.
- **Razão**: documento §7 (Keychain + stdin); stdin não aparece em `ps` nem em variáveis de
  ambiente herdáveis; MCP não consegue exfiltrar valores.
- **Alternativas**: rota HTTP para definir segredos (o MCP e qualquer cliente com o token
  poderiam escrever/ler); variável de ambiente (visível a filhos e em `ps eww`); motor lendo o
  Keychain direto (precisaria de CGO/Security.framework, proibido pelo build `CGO_ENABLED=0`).

## R9. Esperas, agendamentos e vencimento

- **Decisão**: tabela `esperas` com `retomar_em`; um único agendador (goroutine) acorda na menor
  `retomar_em` via `Relogio.Esperar`, é reacordado quando uma espera nova/menor é gravada, e
  despacha cada espera vencida. Na subida: vencidas há ≤ 24 h executam; > 24 h → execução
  `abortada` ("expirada"); execuções encontradas em `rodando` → `abortada` ("interrompida"), sem
  repetir passos (mesmo espírito conservador da Constituição V). Agendamentos (`cron`/intervalo)
  guardam só a próxima ocorrência; ao vencer, executam uma vez e calculam a próxima **a partir de
  agora** (nunca recuperam várias). Cron: `github.com/robfig/cron/v3` (só o parser
  `ParseStandard`, 5 campos, fuso local).
- **Razão**: FR-040–FR-043; relógio controlável permite os testes de "sem resposta há 2 h".
- **Alternativas**: timer por espera (milhares de goroutines/timers); parser de cron próprio (mais
  código e testes para algo resolvido por uma lib pequena, pura Go e sem dependências).

## R10. Anti-loop, atendimento humano e origem das mensagens

- **Decisão**: nova coluna `mensagens.automacao_id` (e `primeiro_contato`). Todo envio automático
  passa por `seguranca.Portao.Autorizar(conversa, automacao)` na ordem: pausa geral → pausa da
  conversa → grupo → anti-loop (conta mensagens com `automacao_id` na janela deslizante) → primeiro
  contato (conta por conta/hora). Mensagem `de_mim` inserida sem `automacao_id` e sem `disparo_id`
  (enviada pela janela, pelo MCP ou eco do celular com `wa_id` novo) = **manual** → pausa `humano`.
  Ecos de mensagens que o próprio motor enviou são deduplicados pelo `UNIQUE(conversa_id, wa_id)`
  do MVP e não contam.
- **Razão**: um ponto único de verificação (Constituição VIII) e contagem a partir de dados já
  persistidos (sobrevive a reinício).
- **Alternativas**: contadores em memória (perdem estado no reinício).

## R11. Cadeia de gatilhos

- **Decisão**: cada fato enfileirado carrega `cadeia` (ids de automações que o causaram); o
  despachante ignora automações presentes na cadeia e para em profundidade 5.
- **Razão**: loops sem mensagens (etiqueta ↔ etapa) não seriam pegos pelo anti-loop (FR-023).

## R12. Editor de código no app (Monaco offline)

- **Decisão**: `monaco-editor` **0.57.0** local + `@monaco-editor/react` **4.7.0** com
  `loader.config({ monaco })` (sem CDN: o loader padrão aponta para jsDelivr). Workers pelo Vite
  com os caminhos novos do `exports` do 0.57:
  `monaco-editor/editor/editor.worker?worker` e
  `monaco-editor/language/typescript/ts.worker?worker` (o caminho antigo `esm/vs/...` não resolve
  **[testado com resolução ESM do Node; não verificado no Vite]**), `self.MonacoEnvironment.getWorker`.
  API TS no namespace novo `monaco.typescript` (o `monaco.languages.typescript` está depreciado
  desde 0.55): `typescriptDefaults.addExtraLib(dts, 'file:///node_modules/@zapdesk/automacao/index.d.ts')`,
  `setCompilerOptions({target: ES2022, module: ESNext, moduleResolution: NodeJs, strict: true,
  noEmit: true, isolatedModules: true})`, um model por arquivo em
  `file:///automacoes/<id>/<caminho>`, marcadores via `getModelMarkers`. `automacao.json` validado
  pelo JSON schema servido pelo motor (`jsonDefaults.setDiagnosticsOptions`). Entradas enxutas do
  0.56+ (`monaco-editor/editor` + registro só de TS/JSON/Markdown) para reduzir o bundle.
- **CSP**: acrescentar `worker-src 'self' blob:` e `font-src 'self' data:` à CSP atual
  (`style-src 'unsafe-inline'` já existe e é necessário ao Monaco — issue #4927)
  **[não verificado por teste]**.
- **Razão**: FR-080 offline, sem CDN (Constituição I).
- **Alternativas**: CodeMirror 6 (sem checador TS embutido); editor externo apenas (o usuário pediu
  programar no app).

## R13. Canvas do chatbot e Kanban

- **Decisão**: `@xyflow/react` (React Flow, MIT, offline; versão estável atual fixada na
  implementação) para o canvas de nós; Kanban com drag-and-drop nativo do HTML5 + menu "Mover
  para…" por teclado, colunas virtualizadas com `@tanstack/react-virtual` (já no app).
- **Razão**: canvas de nós conectáveis é exatamente o caso do React Flow; Kanban simples não
  justifica dependência.
- **Alternativas**: canvas próprio em SVG (muito trabalho); `@dnd-kit` (dependência a mais).

## R14. Projetos das automações de IA na pasta de dados

- **Decisão**: `<pasta-dados>/automacoes/<id>/` com os arquivos do usuário e uma subpasta gerenciada
  `.zapdesk/` (oculta na árvore e na API) contendo `automacao.d.ts` (tipos da SDK),
  `automacao.schema.json` (esquema do manifesto) e um `tsconfig.json` na raiz do projeto com
  `paths: {"@zapdesk/automacao": ["./.zapdesk/automacao.d.ts"]}` — assim o VS Code autocompleta. O
  motor embute (`go:embed`) os tipos da SDK copiados de `compartilhado/automacao-sdk/dist/` por
  `scripts/sincronizar-sdk.sh` (CI falha se divergir) e os modelos de projeto. O editor detecta
  alteração externa comparando hashes (`GET .../arquivos` a cada 3 s enquanto aberto); escritas
  pela API/MCP publicam `automacao.arquivos_alterados`.
- **Razão**: §3.1 do documento (pasta abrível no VS Code); Constituição III (motor dono dos
  arquivos; app e MCP só pela API); tipos servidos pelo motor garantem que editor, VS Code e MCP
  veem a mesma versão da SDK que o runner executa.
- **Alternativas**: `node_modules/@zapdesk/automacao` na pasta (parece convite a npm arbitrário);
  `fsnotify` no motor (dependência e watchers permanentes para um caso raro).

## R15. Notificações ao operador

- **Decisão**: evento WS `notificacao` `{titulo, corpo, automacao_id?, conversa_id?}` publicado
  pelo motor; o processo principal do app mostra `Notification` do Electron (padrão já usado no
  fim de disparo) e, ao clicar, abre a conversa.
- **Razão**: o motor não tem UI; o app já assina o WS no processo principal.
