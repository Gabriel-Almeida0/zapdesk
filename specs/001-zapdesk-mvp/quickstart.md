# Quickstart — validação do ZapDesk MVP

Guia de execução e validação ponta a ponta. Contratos: `contracts/`. Modelo: `data-model.md`.

## Pré-requisitos

- macOS Apple Silicon, Go 1.27 (`brew install go`), Node 22 + npm 10.
- Para E2E real: um número de WhatsApp de teste no celular e 2–5 números próprios para receber.

## Instalação e testes automatizados

```bash
cd ~/projetos/zapdesk
npm install                                  # workspaces: app, mcp, compartilhado/cliente-motor
(cd motor && go test ./...)                  # domínio + integração da API com cliente falso
npm test --workspaces                        # cliente-motor, app (Vitest) e mcp (contra motor falso)
npm run tipos --workspaces                   # tsc --noEmit
```

Esperado: todos verdes. Testes de domínio obrigatórios em `motor/internal/{telefone,leads,
variaveis,agendamento,disparos}`.

## Desenvolvimento com WhatsApp falso (sem conta real)

```bash
# Terminal 1 — motor falso numa pasta temporária
export ZAPDESK_TOKEN=$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=')
(cd motor && go run ./cmd/zapdesk-motor --whatsapp=falso --pasta-dados /tmp/zapdesk-dev --porta 7788 --sem-stdin)
# stdout: {"evento":"pronto","porta":7788,...}

# Terminal 2 — conectar conta falsa e mandar mensagem
H="Authorization: Bearer $ZAPDESK_TOKEN"
CONTA=$(curl -s -H "$H" -X POST localhost:7788/v1/contas -d '{}' | jq -r .id)
curl -s -H "$H" -X POST localhost:7788/v1/falso/contas/$CONTA/escanear-qr -d '{"telefone":"+5511900000001","nome":"Teste"}'
curl -s -H "$H" -X POST localhost:7788/v1/falso/contas/$CONTA/mensagem-recebida -d '{"de":"+5511911112222","texto":"oi"}'
curl -s -H "$H" "localhost:7788/v1/contas/$CONTA/conversas" | jq '.itens[0].nao_lidas'   # 1
```

App contra motor falso: `npm run dev -w app -- --whatsapp=falso` (o processo principal inicia o
motor em modo falso com pasta de dados temporária).

## Cenários de validação

| # | Cenário | Como | Resultado esperado |
|---|---------|------|--------------------|
| 1 | Subida rápida | abrir o app com 1 conta pareada; medir até sumir "Ligando o WhatsApp…" | < 2 s (SC-001) |
| 2 | `ps` limpo | fechar o app sem disparo; `pgrep -fl -i zapdesk` | nenhuma linha (SC-002) |
| 3 | Memória ociosa | 2 contas conectadas, 5 min parado; `ps -o rss= -p <pid_motor>` | < 80 MB (SC-003) |
| 4 | Sessão persistida | fechar/abrir 10 vezes | nunca pede QR (SC-004) |
| 5 | Importação | `motor/testes/dados/leads-1000.csv` (fixture com 100 duplicados no lote, 50 já existentes, 12 inválidos) | relatório exato; soma = 1000; < 5 s (SC-005) |
| 6 | Idempotência | disparo falso de 200 destinatários, intervalo 1–1 s; matar o motor (`kill -9`) no meio; reabrir; retomar; `GET /v1/falso/enviadas` | nenhum telefone repetido (SC-006) |
| 7 | Janela/limites | disparo falso com janela 09:00–18:00 e 40/h; avançar relógio via `/v1/falso/relogio` | 0 envios fora da janela; nenhum período de 60 min > 40 (SC-007) |
| 8 | Barra de menu | disparo ativo, fechar janela | ícone na barra de menu; ao fim, notificação "Disparo concluído: …" e `ps` limpo |
| 9 | MCP com app fechado | fechar app; no Claude Code: "importe estes 10 leads e dispare 'Oi {nome}'" | app abre; retorno lista duplicados com data; disparo em `enviando` (SC-009) |
| 10 | Chat completo (real) | texto, imagem, vídeo, áudio gravado, documento, figurinha, citar, reagir, editar, apagar | tudo aparece no celular |
| 11 | Fluidez | motor falso com 50.000 mensagens geradas (`/v1/falso/historico`) | rolagem sem travar; abrir conversa < 500 ms (SC-008) |

## Configurar o MCP

Ajustes → "Usar com Claude" mostra os comandos prontos (ver `research.md` §8):

```bash
claude mcp add -s user -e ELECTRON_RUN_AS_NODE=1 zapdesk -- \
  "/Applications/ZapDesk.app/Contents/MacOS/ZapDesk" \
  "/Applications/ZapDesk.app/Contents/Resources/mcp/zapdesk-mcp.mjs"
```

Claude Desktop: adicionar o bloco `mcpServers.zapdesk` equivalente em
`~/Library/Application Support/Claude/claude_desktop_config.json` e reiniciar o Claude Desktop.

## Empacotar

```bash
npm run empacotar            # compila motor (CGO_ENABLED=0 darwin/arm64), bundle do MCP e .dmg
open app/dist/ZapDesk-*.dmg  # primeira abertura (app não notarizado): Ajustes do Sistema › Privacidade e Segurança › "Abrir mesmo assim"
```
