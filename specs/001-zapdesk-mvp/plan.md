# Implementation Plan: ZapDesk MVP — WhatsApp desktop local com disparo em massa e MCP

**Branch**: `001-zapdesk-mvp` | **Date**: 2026-09-27 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/001-zapdesk-mvp/spec.md`

## Summary

App desktop macOS com experiência de WhatsApp, disparo em massa e controle por IA, 100% local.
Abordagem: monorepo com três partes que conversam apenas por um contrato HTTP/WebSocket local:

- `motor/` — executável Go (whatsmeow + SQLite puro Go) que é a **fonte única da verdade**:
  contas multi-dispositivo, mensagens, leads com deduplicação E.164, templates, etiquetas e o
  agendador de disparos idempotente. Escuta só em `127.0.0.1` (porta aleatória, token).
- `app/` — Electron + React + TypeScript + Vite: inicia/encerra o motor, grava `runtime.json`,
  barra de menu durante disparos, todas as telas em pt-BR.
- `mcp/` — servidor MCP stdio em TypeScript que lê `runtime.json`, abre o app se fechado e expõe
  todas as ações como ferramentas.
- `compartilhado/cliente-motor/` — pacote TS com os tipos e o cliente HTTP/WS do contrato, usado
  por `app/` e `mcp/`.

O whatsmeow fica atrás da interface `ClienteWhatsApp` com uma implementação falsa, o que permite
desenvolver `app/` e `mcp/` e testar toda a API sem conta real (research.md §1, §9).

## Technical Context

**Language/Version**: Go 1.27 (motor); TypeScript 5.x sobre Node 22 (MCP) e Electron 44
(Chromium + Node embutido) para o app.

**Primary Dependencies**:
- Motor: `go.mau.fi/whatsmeow` (pseudo-versão fixada), `modernc.org/sqlite`,
  `github.com/nyaruka/phonenumbers` v1.8.1, `github.com/xuri/excelize/v2` v2.11.0,
  `github.com/coder/websocket`, `github.com/rs/zerolog` + `gopkg.in/natefinch/lumberjack.v2`
  (log rotativo), `github.com/oklog/ulid/v2`, `github.com/at-wat/ebml-go` +
  `github.com/pion/webrtc/v4/pkg/media/oggwriter` (remux de áudio).
- App: Electron 44, electron-vite, electron-builder 26, React 19, React Router, TanStack Query,
  `@tanstack/react-virtual`, `qrcode` (renderizar QR), `lucide-react`.
- MCP: `@modelcontextprotocol/server` 2.x (+ `@modelcontextprotocol/client` nos testes), zod 4,
  esbuild.

**Storage**: SQLite local (`zapdesk.db` do app com FTS5; `sessoes/<conta_id>.db` do whatsmeow por
conta), cache de mídia e anexos em arquivos na pasta de dados (data-model.md).

**Testing**: `go test` (unitários de domínio com relógio injetável + integração da API via
`httptest` e cliente falso); Vitest (+ Testing Library) em `app/` e `compartilhado/`; Vitest em
`mcp/` com `InMemoryTransport` contra motor falso real; E2E manual com conta real (quickstart.md).

**Target Platform**: macOS 14+ Apple Silicon (arm64); Intel desejável (fora do MVP).

**Project Type**: desktop-app (Electron) + serviço local (Go) + servidor MCP (stdio).

**Performance Goals**: motor pronto < 2 s; abrir conversa < 500 ms com dezenas de milhares de
mensagens; importação de 1.000 linhas < 5 s; disparos de até 50.000 destinatários.

**Constraints**: motor ocioso < 80 MB RAM (`GOMEMLIMIT=64MiB`); nenhum processo após fechar
(exceto disparo ativo na barra de menu); só `127.0.0.1` + token; sem rede além do WhatsApp; sem
Docker; tudo em pt-BR.

**Scale/Scope**: 1 usuário, poucas contas (1–5), ~12 telas, ~70 endpoints/eventos, ~31
ferramentas MCP.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Princípio | Como o plano cumpre | Status |
|-----------|---------------------|--------|
| I. Local-first | Pasta de dados local; bind `127.0.0.1:0`; token via env `ZAPDESK_TOKEN`, Bearer/`?token=`, comparação em tempo constante, checagem de `Host`; `runtime.json` 0600; logs sem conteúdo por padrão; sem telemetria (contracts/runtime.md, api-http.md) | ✅ |
| II. Leveza e ciclo limpo | Motor Go estático sem CGO; `GOMEMLIMIT`; `pronto` emitido antes de conectar contas; vigia de stdin + SIGTERM→SIGKILL em 5 s; `ps` limpo verificado no quickstart; sem Docker/serviços | ✅ (medição de RAM é tarefa de polimento) |
| III. Motor fonte única | App e MCP só usam `@zapdesk/cliente-motor`; nenhum acesso ao SQLite fora do motor; toda ação da UI tem endpoint; contrato versionado em `contracts/` | ✅ |
| IV. WhatsApp isolado | Interface `ClienteWhatsApp` em `internal/whatsapp`; whatsmeow só em `internal/whatsapp/whatsmeow`; cliente falso + endpoints `/v1/falso/*` só no modo falso (contracts/cliente-whatsapp.md) | ✅ |
| V. Disparo idempotente | Commit de `enviando` antes de enviar e de `enviado`+`wa_id` depois; reconciliação conservadora de `enviando`; boot converte ativos em `pausado`; queda curta não marca falha (data-model.md › Destinatário) | ✅ |
| VI. Testes de domínio | Tarefas de teste antes da implementação para telefone, dedup, agendamento, variáveis e máquinas de estado; integração da API com falso; MCP contra motor falso; relógio injetável | ✅ |
| VII. Português | Identificadores, pastas, rotas, campos JSON, ferramentas MCP e UI em pt-BR | ✅ |
| Restrições (Electron seguro) | `contextIsolation`, `nodeIntegration: false`, `sandbox`, preload mínimo | ✅ |

**Re-check pós-design (Fase 1)**: contratos e modelo mantêm todos os itens acima. Nenhuma
violação; única complexidade adicional (pacote `compartilhado/`) justificada abaixo.

## Project Structure

### Documentation (this feature)

```text
specs/001-zapdesk-mvp/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── api-http.md           # endpoints REST do motor
│   ├── eventos-ws.md         # eventos WebSocket
│   ├── runtime.md            # processo do motor, runtime.json, modo falso
│   ├── cliente-whatsapp.md   # interface Go interna (motor)
│   └── mcp-ferramentas.md    # ferramentas MCP e esquemas
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository root)

```text
package.json                  # workspaces npm: app, mcp, compartilhado/cliente-motor; scripts raiz
.github/workflows/ci.yml
scripts/empacotar.sh          # motor (darwin/arm64) + bundle MCP + electron-builder

motor/                        # módulo Go "zapdesk/motor"
├── go.mod
├── cmd/zapdesk-motor/main.go # flags, montagem de dependências, protocolo stdout
├── internal/
│   ├── config/               # flags/env, pasta de dados
│   ├── ciclo/                # stdout JSON, vigia de stdin, sinais, motor.lock, encerramento
│   ├── relogio/              # Relogio real e controlável
│   ├── ids/                  # ULID
│   ├── logs/                 # zerolog + lumberjack, filtro de conteúdo
│   ├── armazenamento/        # abrir SQLite, migrações + backup, repositórios por entidade
│   │   └── migracoes/        # 0001_inicial.sql ...
│   ├── eventos/              # barramento pub/sub interno (seq, tipos do eventos-ws.md)
│   ├── telefone/             # normalização E.164 (+ testes)
│   ├── variaveis/            # extrair/resolver variáveis (+ testes)
│   ├── leads/                # importação, deduplicação, relatório (+ testes)
│   ├── importacao/           # leitura CSV/XLSX, prévia, cache temporário (+ testes)
│   ├── agendamento/          # ProximoEnvio, estimativa (função pura + testes)
│   ├── disparos/             # máquinas de estado, fila por conta, executor, retomada (+ testes)
│   ├── whatsapp/             # interface ClienteWhatsApp + tipos + erros sentinela
│   │   ├── whatsmeow/        # adaptador real (único import de whatsmeow)
│   │   └── falso/            # cliente em memória determinístico
│   ├── contas/               # gerenciador multi-conta (clientes, QR, estados)
│   ├── chat/                 # mensagens, conversas, contatos, recibos, busca, status, mídia
│   ├── organizacao/          # etiquetas, notas, templates
│   ├── arquivos/             # anexos, limites, remux de áudio
│   └── api/                  # servidor HTTP, auth/host, erros, paginação, handlers, WS, rotas falso
└── testes/
    ├── integracao/           # API ponta a ponta com cliente falso (httptest)
    └── dados/                # fixtures (leads-1000.csv, leads.xlsx)

compartilhado/cliente-motor/  # pacote "@zapdesk/cliente-motor"
├── src/{tipos.ts, erros.ts, cliente.ts, eventos.ts, runtime.ts, index.ts}
└── tests/

app/
├── electron.vite.config.ts
├── electron-builder.yml
├── src/main/                 # index, motor (spawn/religar), runtime-json, ciclo-vida, bandeja, energia, ipc, janela
├── src/preload/index.ts      # ponte mínima: obterConexao(), salvarArquivo(), abrirExterno()
├── src/renderer/
│   ├── main.tsx, App.tsx, rotas.tsx
│   ├── api/                  # provider do cliente, hooks TanStack Query, assinatura WS
│   ├── componentes/          # BarraLateral, SeletorConta, Bolha, Composer, Esqueleto, Vazio, FaixaAviso...
│   ├── telas/                # Carregando, ErroMotor, ConectarConta, Conversas, Chat, Contatos, Status,
│   │                         # Disparos, NovoDisparo/, DetalheDisparo, Leads, ImportarLeads, Templates,
│   │                         # Etiquetas, Ajustes
│   └── estilos/              # tokens claro/escuro
└── tests/

mcp/
├── build.mjs                 # esbuild → dist/zapdesk-mcp.mjs
├── src/{index.ts, servidor.ts, garantir-motor.ts, formatar.ts, conta-padrao.ts}
├── src/ferramentas/{contas,leads,conversas,contatos,etiquetas,templates,disparos,status}.ts
└── tests/
```

**Structure Decision**: monorepo com `motor/`, `app/`, `mcp/` (pedido pelo usuário) +
`compartilhado/cliente-motor/`. Divisão para agentes paralelos por diretório: um agente de
`motor/`, um de `app/`, um de `mcp/`; `compartilhado/` é criado na Fase 1 a partir do contrato e
depois só muda junto com o contrato. Contrato em `contracts/` é o ponto de sincronização.

## Fluxos-chave de arquitetura

1. **Subida**: app gera token → spawn do motor com `ZAPDESK_TOKEN` → lê `pronto` no stdout →
   grava `runtime.json` → renderer pede `obterConexao()` ao preload → cliente HTTP/WS.
2. **Mensagem recebida**: whatsmeow → adaptador normaliza (LID→telefone) → `chat` grava →
   `eventos` publica `mensagem.nova` → WS → TanStack Query atualiza lista/chat; `disparos` escuta
   para marcar `respondeu`.
3. **Disparo**: `POST /disparos` → valida variáveis → `agendado` → fila da conta → executor
   (goroutine por conta ativa) calcula `ProximoEnvio` → `Relogio.Esperar` → commit `enviando` →
   `TemWhatsApp` → `Enviar*` → commit `enviado` → eventos. Recibos atualizam destinatários.
4. **Fechar com disparo**: evento `disparos.ativos > 0` → `before-quit` interceptado → janela
   escondida, dock oculto, `Tray` com progresso → `disparo.finalizado` + `ativos = 0` →
   `Notification` → encerra motor → `app.exit`.
5. **MCP**: ferramenta → `garantirMotor()` (runtime.json/saúde/`open -b`) → cliente HTTP → resposta
   estruturada.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| 4º diretório `compartilhado/cliente-motor` além de `motor/`, `app/`, `mcp/` | App e MCP consomem o mesmo contrato (~70 rotas/eventos); um único cliente tipado evita divergência entre agentes paralelos | Duplicar tipos em `app/` e `mcp/` geraria drift silencioso do contrato (Princípio III) |
| Um banco whatsmeow por conta (vários `sqlstore.Container`) | Segue o documento de produto; remover conta = apagar arquivo | Container único é o padrão das bridges, mas misturaria sessões e dificultaria remoção limpa |
