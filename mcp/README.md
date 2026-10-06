# ZapDesk MCP

Servidor MCP (stdio) que deixa o Claude Code e o Claude Desktop controlarem o ZapDesk: importar
leads sem duplicar, criar disparos em massa, ler e responder conversas, organizar contatos,
etiquetas e templates, montar funis de vendas e **criar, programar, testar e operar automações**
(fluxos, chatbots e automações de IA em TypeScript). Contratos:
`specs/001-zapdesk-mvp/contracts/mcp-ferramentas.md` (31 ferramentas) e
`specs/002-automacoes/contracts/mcp-ferramentas.md` (38 ferramentas).

Cada ferramenta é um cliente fino da API local do motor. O servidor descobre o motor pelo
`runtime.json` da pasta de dados; **se o app estiver fechado, abre o app e espera até 30 s**.

## Instalação no Claude Code

Com o app instalado em `/Applications` (o bundle roda com o próprio executável do app, sem
precisar de Node instalado):

```bash
claude mcp add -s user -e ELECTRON_RUN_AS_NODE=1 zapdesk -- \
  "/Applications/ZapDesk.app/Contents/MacOS/ZapDesk" \
  "/Applications/ZapDesk.app/Contents/Resources/mcp/zapdesk-mcp.mjs"
```

Confira com `claude mcp list` (deve aparecer `zapdesk: ... ✓ Connected`). Para remover:
`claude mcp remove -s user zapdesk`.

## Instalação no Claude Desktop

Edite `~/Library/Application Support/Claude/claude_desktop_config.json` (crie se não existir),
acrescente o bloco abaixo dentro de `mcpServers` e reinicie o Claude Desktop:

```json
{
  "mcpServers": {
    "zapdesk": {
      "command": "/Applications/ZapDesk.app/Contents/MacOS/ZapDesk",
      "args": ["/Applications/ZapDesk.app/Contents/Resources/mcp/zapdesk-mcp.mjs"],
      "env": { "ELECTRON_RUN_AS_NODE": "1" }
    }
  }
}
```

A tela Ajustes › "Usar com Claude" do app mostra esses mesmos comandos com os caminhos reais.

## Uso em desenvolvimento (sem empacotar)

```bash
cd zapdesk   # raiz do repositório clonado
npm run compartilhado:compilar && npm run compilar -w @zapdesk/mcp   # gera mcp/dist/zapdesk-mcp.mjs
claude mcp add -s user zapdesk-dev -- node "$PWD/mcp/dist/zapdesk-mcp.mjs"
```

Rodando de dentro do repositório, se o app estiver fechado o servidor executa `npm run dev` na
raiz do monorepo. Variáveis de ambiente úteis:

| Variável | Efeito |
|----------|--------|
| `ZAPDESK_PASTA_DADOS` | Pasta onde procurar o `runtime.json` (padrão `~/Library/Application Support/ZapDesk`). Precisa ser a mesma pasta usada pelo app/motor. |
| `ZAPDESK_COMANDO_ABRIR` | Comando de shell usado para abrir o app quando ele está fechado (substitui `npm run dev` / `open`). |

Exemplo apontando para um motor falso de desenvolvimento (ver `quickstart.md`), com o
`runtime.json` gravado pelo app em `/tmp/zapdesk-dev`:

```bash
claude mcp add -s user zapdesk-dev -e ZAPDESK_PASTA_DADOS=/tmp/zapdesk-dev -- \
  node "$PWD/mcp/dist/zapdesk-mcp.mjs"
```

## Como o app é aberto

1. `runtime.json` ausente, `pid_motor` morto ou `GET /v1/saude` sem 200 → app fechado.
2. `ZAPDESK_COMANDO_ABRIR` definido → roda o comando; senão, fora de um `.app` → `npm run dev`
   na raiz do repositório; senão → `open -b com.gabriel.zapdesk` (fallback `open -a ZapDesk`).
3. Sonda a cada 250 ms por até 30 s. Sem resposta: a ferramenta devolve
   "Não consegui ligar o ZapDesk. Abra o app manualmente e tente de novo."

## Ferramentas (69)

| Grupo | Ferramentas |
|-------|-------------|
| Contas e sistema | `listar_contas`, `status_zapdesk` |
| Leads | `importar_leads`, `listar_leads` |
| Conversas | `listar_conversas`, `ler_mensagens`, `buscar_mensagens`, `enviar_mensagem`, `reagir_mensagem`, `editar_mensagem`, `apagar_mensagem`, `marcar_como_lida`, `ver_status` |
| Contatos e etiquetas | `listar_contatos`, `atualizar_contato`, `listar_etiquetas`, `criar_etiqueta`, `atualizar_etiqueta`, `excluir_etiqueta` |
| Templates | `listar_templates`, `criar_template`, `atualizar_template`, `excluir_template` |
| Disparos | `criar_disparo`, `listar_disparos`, `ver_disparo`, `iniciar_disparo`, `pausar_disparo`, `retomar_disparo`, `cancelar_disparo`, `exportar_relatorio` |
| Funil | `listar_funis`, `criar_funil`, `editar_funil`, `excluir_etapa`, `excluir_funil`, `listar_cards_funil`, `mover_card_funil`, `remover_card_funil`, `historico_funil`, `atualizar_lead` |
| Automações | `listar_automacoes`, `ver_automacao`, `ver_formatos_automacao`, `criar_automacao`, `editar_automacao`, `validar_automacao`, `excluir_automacao`, `ativar_automacao`, `desativar_automacao`, `executar_automacao`, `testar_automacao`, `simular_chatbot` |
| Automações de IA (código) | `ver_tipos_sdk`, `listar_modelos_automacao_ia`, `criar_automacao_ia`, `listar_arquivos_automacao`, `ler_arquivo_automacao`, `escrever_arquivo_automacao`, `renomear_arquivo_automacao`, `excluir_arquivo_automacao`, `compilar_automacao` |
| Execuções e segurança | `listar_execucoes`, `ver_execucao`, `listar_pausas`, `pausar_conversa`, `retomar_conversa`, `listar_segredos`, `ver_configuracao_automacoes` |

Pontos importantes:

- **`criar_disparo` começa na hora, sem confirmação** (respeita início, janela, limites e a fila
  da conta). Use `pausar_disparo`/`cancelar_disparo` para interromper. Se algum destinatário não
  tiver valor para uma variável (`{nome}`, `{empresa}`…) e não houver `valores_padrao`, nada é
  criado e o erro lista as linhas incompletas.
- `importar_leads` nunca duplica: devolve novos, já existentes (com a data da importação
  original), inválidos e repetidos no próprio lote.
- `conta_id` é opcional quando há exatamente uma conta conectada.
- Conectar, reconectar, renomear e remover contas só pela interface (exige QR code no celular).

## Automações pelo MCP

Três tipos, todos testáveis sem enviar nada antes de ativar:

- **Fluxo** ("quando X, faça Y") e **chatbot** (menu/perguntas em grafo) são JSON. A IA consulta
  `ver_formatos_automacao` (gatilhos, condições, ações, nós do chatbot, manifesto — com exemplos,
  embutido em `src/formatos-automacao.ts`), confere com `validar_automacao` (erros com o caminho
  do campo, ex.: `definicao.acoes[2].etapa_id`), cria com `criar_automacao`, testa com
  `testar_automacao` (fluxo) ou `simular_chatbot` (sequência de respostas do contato) e ativa.
- **Automação de IA** é um projeto TypeScript (`automacao.json` + `index.ts` …) que usa a SDK
  `@zapdesk/automacao`. Ciclo esperado:

  ```text
  ver_tipos_sdk → criar_automacao_ia → escrever_arquivo_automacao (manifesto + código)
    → compilar_automacao  (erros "index.ts:6:5 [sintaxe] …", linha e coluna 1-base como no
                           editor; repetir até "Compilou sem erros")
    → testar_automacao    (simulação: log, ações que seriam feitas, retorno; ia_simulada: true
                           dispensa a chave da Anthropic)
    → ativar_automacao    (a partir daqui age em mensagens reais)
    → listar_execucoes / ver_execucao (log, stack, ações, tokens)
  ```

  A descrição de `escrever_arquivo_automacao` já traz um resumo da API do `ctx`
  (`src/referencia-sdk.ts`: handlers, `ctx.responder`, `ctx.ia.gerar/classificar/extrair`,
  `ctx.funil`, `ctx.etiquetas`, `ctx.leads`, `ctx.memoria`, `ctx.http`, `ctx.humano`,
  `ctx.agendar`, permissões exigidas); `ver_tipos_sdk` devolve esse resumo e o `.d.ts` completo
  servido pelo motor. Escrever **não** compila sozinho; `hash_anterior` (de
  `ler_arquivo_automacao`) evita sobrescrever o que o usuário mudou no editor.

Regras de segurança refletidas nas ferramentas:

- `ativar_automacao` e `executar_automacao` têm `openWorldHint: true` (efeito real no WhatsApp);
  `executar_automacao` espera até ~15 s e devolve o resultado da execução.
- Todo envio automático passa pelo portão do motor (pausas, anti-loop de 10 mensagens a cada
  10 min, grupos, primeiros contatos); `pausar_conversa`/`retomar_conversa`/`listar_pausas`
  controlam as pausas por conversa.
- **Segredos**: `listar_segredos` devolve só nomes. Não existe ferramenta para definir valores
  de segredos nem para mudar limites globais — isso é feito pelo usuário no app.
- **Limite honesto do isolamento**: o código das automações de IA roda num processo Node
  separado, sem acesso a arquivos, rede direta (só `ctx.http` com a permissão `rede`) ou
  variáveis de ambiente, e cada chamada do `ctx` é checada contra as permissões do
  `automacao.json`. Isso protege contra erros e acidentes, **não contra código malicioso**: só
  ative código que você (ou a IA, sob sua supervisão) escreveu e testou.

## Testes

```bash
npm test -w @zapdesk/mcp
```

- Cada ferramenta é testada com `InMemoryTransport` + `Client` contra um motor simulado em
  memória (`tests/motor-simulado.ts`; rotas da 002 em `tests/motor-simulado-automacoes.ts`,
  que compila os projetos de IA com o esbuild de verdade para produzir erros reais
  `arquivo:linha:coluna`). `tests/paridade.test.ts` confere as 69 ferramentas contra as tabelas
  dos dois contratos, com annotations.
- `tests/integracao.test.ts` roda o bundle via stdio contra o motor Go real em
  `--whatsapp=falso` (`npm run motor:compilar` + `npm run compilar -w @zapdesk/mcp`); é pulado
  enquanto o binário não suportar as rotas usadas. `ZAPDESK_PULAR_INTEGRACAO=1` força pular.
- `tests/integracao-automacoes.test.ts` faz o ciclo completo de uma automação de IA via stdio
  contra o motor real com `--ia=falsa` e o runner (`npm run runner:compilar`): criar, escrever
  `index.ts`, compilar com e sem erro, testar (nada em `/v1/falso/enviadas`), ativar, injetar
  mensagem e ver a execução `ok`. O harness só passa `--ia`/`--runner-*` se o binário já conhece
  essas flags; com `ZAPDESK_CI=1` a falta de suporte falha em vez de pular. Antes de rodar:
  `npm run motor:compilar && npm run runner:compilar && npm run compilar -w @zapdesk/mcp`
  (conferido com o motor da 002 completo: os 3 casos rodam, nenhum é pulado).
