# Quickstart — validação da feature 002 (Automações)

Guia de verificação ponta a ponta. Contratos: `contracts/`; entidades: `data-model.md`.

## Pré-requisitos

- Go 1.27, Node 22.15+ (`.nvmrc`), dependências instaladas (`npm install` na raiz).
- Compilar: `npm run compartilhado:compilar` (cliente-motor + automacao-sdk),
  `npm run sdk:sincronizar` (tipos da SDK → motor), `npm run compilar -w @zapdesk/automacao-runner`,
  `npm run motor:compilar`.

## 1. Testes automatizados (portões da constituição)

```bash
cd motor && go test ./...                 # domínio + integração (WhatsApp falso, --ia=falsa, runner via node)
npm test --workspaces                     # cliente-motor, automacao-sdk, runner, app, mcp
npm run tipos --workspaces
bash scripts/sincronizar-sdk.sh --verificar   # tipos embutidos no motor == SDK
```

Esperado: tudo verde; nenhum teste abre rede (IA simulada; servidor HTTP falso no teste do
cliente Claude).

## 2. Motor falso manual

```bash
export ZAPDESK_TOKEN=$(openssl rand -base64 32)
motor/bin/zapdesk-motor --pasta-dados /tmp/zd --whatsapp=falso --ia=falsa --sem-stdin \
  --runner-exec "$(which node)" --runner-script automacao/runner/dist/zapdesk-runner.mjs
# A=http://127.0.0.1:<porta>/v1 ; H="Authorization: Bearer $ZAPDESK_TOKEN"
```

1. Conta: `POST $A/contas` + `POST $A/falso/contas/{id}/escanear-qr`.
2. **Funil (US1)**: `POST $A/funis {"nome":"Prospecção","etapas":[{"nome":"Novo"},{"nome":"Qualificando"},{"nome":"Proposta"},{"nome":"Fechado"}]}`;
   `PUT $A/funis/{id}/cards {"telefone":"+5511900000002","etapa_id":<Novo>}` → 201; de novo para
   "Qualificando" → 200; `GET .../historico` mostra as duas entradas.
3. **Fluxo (US2)**: criar fluxo `disparo_respondeu` → `adicionar_etiqueta quente` →
   `mover_etapa Qualificando` → `iniciar_chatbot`; ativar; criar e iniciar disparo para
   `+5511900000002`; `POST $A/falso/contas/{id}/mensagem-recebida {"de":"+5511900000002","texto":"oi"}`
   → execução `ok` com 3 ações; etiqueta e etapa conferidas.
4. **Sem resposta**: fluxo `sem_resposta apos_s=7200` → `enviar_template`; enviar mensagem manual;
   `PUT $A/falso/relogio {"avancar_s":7300}` + `POST $A/falso/processar-esperas` → template em
   `GET $A/falso/enviadas`. Repetir parando o motor no meio (reiniciar com a mesma pasta) → sai uma
   única vez. Avançar 30 h com o motor parado → execução `abortada` ("expirada").
5. **Anti-loop / humano**: fluxo que responde toda mensagem; injetar 12 mensagens em sequência →
   só 10 automáticas; `conversa.pausa` `anti_loop`; enviar mensagem pela API → pausa `humano`.
6. **IA (US3)**: `POST $A/automacoes/ia {"nome":"Resp","modelo":"responder_historico"}` →
   compilação ok; `POST .../testar {"mensagem":{"texto":"Quanto custa?"},"ia_simulada":true}` →
   `simulacao`, ação `enviar` simulada, `GET $A/falso/enviadas` sem nada novo; ativar; injetar
   mensagem → resposta "[IA simulada] …" enviada.
7. **Isolamento**: projeto em branco com `while(true){}` em `aoReceberMensagem`, ativar, injetar
   mensagem → `erro` em ~60 s; a automação do passo 6 continua respondendo; após 5 erros,
   `automacao.atualizada` com `ativa=false`. Manifesto sem `enviar` chamando `ctx.responder` →
   `erro` com "Permissão 'enviar' não declarada em automacao.json". `import fs from 'fs'` →
   `compilacao_falhou` "Módulo não permitido: fs".
8. **Chatbot (US4)**: criar o bot de `formatos.md`; `POST .../simulador` + mensagens "2",
   "x", "x", "x" → transferência para humano; com e-mail válido → variável `email` preenchida.
9. **MCP (US5)**: `npm test -w @zapdesk/mcp` (paridade de 69 ferramentas e fluxo IA completo).

## 3. App real (manual)

1. `npm run dev`; Ajustes → IA: colar chave → "Testar chave" ok; valor mascarado.
2. Funis → Kanban: arrastar cartão; "Mover para…" pelo teclado.
3. Automações → Nova → IA (código) → "Responder com IA": autocompletar em `ctx.`; erro de
   digitação sublinhado em < 2 s; "Testar" com IA real; "Abrir pasta no editor externo" abre o VS
   Code sem erros de tipo.
4. Conversa com bot ativo mostra a faixa "Chatbot “X” ativo nesta conversa · Assumir"; "Assumir" encerra a sessão.
5. Fechar o app → em ≤ 5 s `ps aux | grep -i -E "zapdesk|runner"` vazio.

## 4. Leveza

`scripts/verificar-leveza.sh` com fluxos e chatbots ativos e nenhum runner vivo: RAM do motor
< 80 MB, `pronto` < 2 s; após usar uma IA, esperar 5 min → processo runner encerrado.

## Resultados (T133, 28/09/2026 — sem conta real de WhatsApp nem chave real da Anthropic)

Tudo abaixo foi executado de verdade nesta máquina (macOS arm64, Go 1.27.1, Node 22.23).

### §1 Testes automatizados — passou

- `go vet ./...` e `ZAPDESK_CI=1 go test -race ./...` (e `-count=3 -race`): 37 pacotes verdes.
- `ZAPDESK_CI=1 npm test` (2 rodadas seguidas): cliente-motor 40, SDK 10, runner 69, app 110,
  mcp 109 — inclusive `tests/integracao-automacoes.test.ts` rodando (não pulado) contra o motor e
  o runner recém-compilados. `npm run tipos` ok; `scripts/sincronizar-sdk.sh --verificar` ok.

### §2 Motor falso manual — passou (15/15), automatizado em `scripts/quickstart-002.mjs`

`node scripts/quickstart-002.mjs` sobe o motor real (`--whatsapp=falso --ia=falsa`, runner via
node) numa pasta temporária e confere: conta; funil (201 → 200, 2 entradas no histórico); fluxo
`disparo_respondeu` com 3 ações `ok` (etiqueta, etapa, chatbot); sem resposta com o motor
reiniciado no meio (sai uma vez) e espera vencida há > 24 h `abortada/expirada`; anti-loop (10
automáticas + pausa `anti_loop`); mensagem manual → pausa `humano`; IA `responder_historico`
testada sem enviar e respondendo "[IA simulada] …" ativa; laço infinito → `erro` por tempo limite
(com `tempo_s` 5 para encurtar) sem afetar a outra IA; 5 erros → desativada
(`erros_seguidos`); `import fs` → `compilacao_falhou`; sem `enviar` → "Permissão 'enviar' não
declarada em automacao.json"; chatbot do `formatos.md` no simulador (3 inválidas → humano;
e-mail válido → `{email}`). Observação: o relógio falso volta ao real quando o motor reinicia, por
isso o script avança 40 h no passo de expiração.

### §3 App real — passou em modo falso; itens com chave/conta real pendentes

Build de produção do app contra o motor real (`ZAPDESK_DEV_FALSO=1 ZAPDESK_IA=falsa`), dirigido
por CDP:

- IA: criar pelo modelo "Responder com IA", editar, erro inline (`import x from 'fs'` →
  `index.ts:1:15` igual no Monaco e na lista, colunas 1-base), "Testar" com IA simulada e com a
  IA falsa do motor, Ativar, mensagem injetada respondida e execução `ok` com ações/log/tokens
  (tokens 0 na IA falsa, por projeto).
- Chatbot: canvas, simulador (menu → pergunta → fim), ativação por palavra-chave (acentos e
  maiúsculas ignorados), sessão em conversa injetada, "2" → transferência para humano, faixa
  "Chatbot “X” ativo nesta conversa · Assumir" e "Assumir" encerrando a sessão.
- Anti-loop: 12 mensagens → 10 automáticas, 1 bloqueada, faixa "Automações pausadas (anti-loop)".
- Sem resposta 2 h com relógio falso e reinício do app/motor: follow-up enviado uma vez.
- Pacote (`.app`): runner em `Contents/Resources/runner/`, IA respondendo com o motor empacotado,
  Monaco com autocompletar em `ctx.` sem nenhuma requisição externa nem violação de CSP.
- **Exige chave real da Anthropic** (não feito): "Testar chave" ok, "Testar" com IA real.
- **Exige ação manual no Mac** (não feito): "Abrir pasta no editor externo" abrindo o VS Code.

### §4 Leveza — passou

`ESPERA_S=300 scripts/verificar-leveza.sh` no `.app` empacotado, com 2 contas, fluxo e chatbot
ativos: `pronto` em 630–724 ms; runner da IA sobe, responde e sai sozinho após 60 s de
ociosidade (1 min configurado); motor ocioso 41–46 MB (limite 80 MB); app inteiro ~410–473 MB;
`pgrep -fl -i zapdesk` vazio 164 ms depois de fechar; runtime.json apagado.

### Quickstart do MVP (regressão FR-111)

| # | Resultado |
|---|-----------|
| Testes automatizados | passou (acima) |
| Dev com WhatsApp falso | passou (conta falsa, mensagem recebida, não lidas = 1) |
| 1 Subida rápida | passou em modo falso: `pronto` < 0,8 s no `.app` |
| 2 `ps` limpo | passou (164 ms) |
| 3 Memória ociosa | passou: 41 MB após 5 min com 2 contas (falsas) |
| 4 Sessão persistida | passou em modo falso: `TestContasCicloCompleto` e 10 reinícios em `TestContaAposReiniciarNaoMenteConectada`; com conta real **não verificado** |
| 5 Importação | passou (`motor/testes/dados`, fixture de 1.000) |
| 6 Idempotência | passou (`TestDisparoIdempotenteAoReiniciar`) |
| 7 Janela/limites | passou (`TestDisparoJanelaELimitePorHora`) |
| 8 Barra de menu | **não verificado** (precisa observar o ícone e a notificação na tela) |
| 9 MCP com app fechado | **não verificado**: precisa do Claude Code/Desktop e abriria o app instalado com os dados reais; a integração stdio do MCP com o motor real passou |
| 10 Chat completo | **exige conta real de WhatsApp** |
| 11 Fluidez | não refeito nesta rodada |

T133 fica aberta até os itens marcados como "exige conta real"/"chave real"/"não verificado".
