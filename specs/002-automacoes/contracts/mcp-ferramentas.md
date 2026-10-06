# Contrato do servidor MCP `zapdesk` — acréscimos da feature 002

Regras gerais (garantir app, retorno estruturado + resumo em português, erros `isError`,
paginação, annotations): **iguais ao MVP** (`specs/001-zapdesk-mvp/contracts/mcp-ferramentas.md`).
Cada ferramenta é cliente fino da API (`api-http.md` deste diretório) via
`@zapdesk/cliente-motor`. Código em `mcp/src/ferramentas/{funis,automacoes,automacoes-ia,execucoes}.ts`.
O teste de paridade passa a unir as tabelas das duas features (31 + 38 = 69 ferramentas).

A instrução do servidor ganha: "Automações: fluxos e chatbots são JSON (veja `ver_formatos_automacao`);
automações de IA são projetos TypeScript — use `ver_tipos_sdk`, escreva arquivos, `compilar_automacao`,
`testar_automacao` (simulação, nada é enviado) e só então `ativar_automacao`. Segredos só podem ser
definidos pelo usuário no app."

## Ferramentas

### Funil

| Ferramenta | Entrada | Saída | API | Annotations |
|------------|---------|-------|-----|-------------|
| `listar_funis` | — | `{funis: Funil[]}` | `GET /funis` | leitura |
| `criar_funil` | `nome`, `etapas?: {nome, cor?}[]` | `Funil` | `POST /funis` | escrita local |
| `editar_funil` | `funil_id`, `nome?`, `etapas?: {etapa_id?, nome, cor?}[]` (lista completa na ordem desejada: ids existentes são editados/reordenados, sem id são criados; etapas omitidas **não** são apagadas) | `Funil` | `PATCH /funis/{id}`, `POST /funis/{id}/etapas`, `PATCH /etapas/{id}`, `PUT /funis/{id}/etapas/ordem` | escrita local, idempotente |
| `excluir_etapa` | `etapa_id`, `destino_etapa_id?` **ou** `remover_cards?: true` | `{ok}` | `DELETE /etapas/{id}` | exclusão |
| `excluir_funil` | `funil_id` | `{ok}` | `DELETE /funis/{id}` | exclusão |
| `listar_cards_funil` | `funil_id`, `etapa_id?`, `busca?`, `limite?`, `cursor?` | página de `Card` | `GET /funis/{id}/cards` | leitura |
| `mover_card_funil` | `funil_id`, `etapa_id`, `lead_id?` **ou** `contato_id?` **ou** `telefone?` | `Card` | `PUT /funis/{id}/cards` (`origem:"mcp"`) | escrita local, idempotente |
| `remover_card_funil` | `funil_id`, `lead_id` | `{ok}` | `DELETE /funis/{id}/cards/{lead_id}?origem=mcp` | escrita local |
| `historico_funil` | `funil_id`, `lead_id?`, `limite?`, `cursor?` | página de `MovimentoFunil` | `GET /funis/{id}/historico` | leitura |
| `atualizar_lead` | `lead_id`, `nome?`, `campos?: Record<string, string\|null>` | `Lead` | `PATCH /leads/{id}` | escrita local |

### Automações (todos os tipos)

| Ferramenta | Entrada | Saída | API | Annotations |
|------------|---------|-------|-----|-------------|
| `listar_automacoes` | `tipo?`, `ativa?` | `{automacoes: Automacao[]}` | `GET /automacoes` | leitura |
| `ver_automacao` | `automacao_id` | `Automacao` | `GET /automacoes/{id}` | leitura |
| `ver_formatos_automacao` | — | `{texto}` (resumo de `formatos.md`: gatilhos, condições, ações, chatbot, manifesto, com exemplos) | — (embutido) | leitura |
| `criar_automacao` | `tipo: "fluxo"\|"chatbot"`, `nome`, `gatilhos`, `definicao`, `contas?`, `incluir_grupos?`, `prioridade?`, `conta_envio_id?`, `limites?`, `ativar?: bool` | `Automacao` (+ `avisos`) | `POST /automacoes` (+ `/ativar`) | escrita local |
| `editar_automacao` | `automacao_id` + campos de `criar_automacao` (parciais) | `Automacao` | `PATCH /automacoes/{id}` | escrita local |
| `validar_automacao` | campos de `criar_automacao` | `{erros, avisos}` | `POST /automacoes/validar` | leitura |
| `excluir_automacao` | `automacao_id` | `{ok}` | `DELETE /automacoes/{id}` | exclusão |
| `ativar_automacao` | `automacao_id` | `Automacao` | `POST /automacoes/{id}/ativar` | escrita local (envia mensagens reais a partir daí → `openWorldHint: true`) |
| `desativar_automacao` | `automacao_id` | `Automacao` | `POST /automacoes/{id}/desativar` | escrita local |
| `executar_automacao` | `automacao_id`, `conversa_id?` \| `contato_id?` \| `lead_id?` \| (`telefone?` + `conta_id?`), `entrada?: object` | `Execucao` | `POST /automacoes/{id}/executar` (`origem:"manual_mcp"`) | envio WhatsApp (destrutivo, mundo aberto) |
| `testar_automacao` | `automacao_id`, `texto?` (mensagem simulada), `mensagem_id?`, `conversa_id?`, `entrada?`, `evento?`, `ia_simulada?: bool` (padrão false) | `ExecucaoDetalhe` (estado `simulacao`/`erro`, log, ações que seriam feitas) | `POST /automacoes/{id}/testar` | leitura (nada é enviado; `openWorldHint` true se IA real) |
| `simular_chatbot` | `automacao_id`, `mensagens: string[]` (sequência de respostas do contato), `ia_simulada?` | `{rodadas: [{entrada, saidas, no_atual, variaveis, estado}]}` | `POST /automacoes/{id}/simulador` + `POST /simulador/{sid}/mensagens` × N + `DELETE` | leitura |

### Automações de IA (código)

| Ferramenta | Entrada | Saída | API | Annotations |
|------------|---------|-------|-----|-------------|
| `ver_tipos_sdk` | — | `{versao, tipos}` (.d.ts completo) | `GET /automacoes/sdk` | leitura |
| `listar_modelos_automacao_ia` | — | `{modelos: ModeloProjeto[]}` | `GET /automacoes/modelos` | leitura |
| `criar_automacao_ia` | `nome`, `modelo?` (padrão `em_branco`), `descricao?` | `Automacao` + `{arquivos: ArquivoProjeto[]}` | `POST /automacoes/ia` | escrita local |
| `listar_arquivos_automacao` | `automacao_id` | `{arquivos: ArquivoProjeto[]}` | `GET /automacoes/{id}/arquivos` | leitura |
| `ler_arquivo_automacao` | `automacao_id`, `caminho` | `ConteudoArquivo` | `GET /automacoes/{id}/arquivos/{caminho}` | leitura |
| `escrever_arquivo_automacao` | `automacao_id`, `caminho`, `conteudo`, `hash_anterior?` | `ArquivoProjeto` | `PUT /automacoes/{id}/arquivos/{caminho}` | escrita local, idempotente |
| `renomear_arquivo_automacao` | `automacao_id`, `de`, `para` | `ArquivoProjeto` | `POST /automacoes/{id}/arquivos/renomear` | escrita local |
| `excluir_arquivo_automacao` | `automacao_id`, `caminho` | `{ok}` | `DELETE /automacoes/{id}/arquivos/{caminho}` | exclusão |
| `compilar_automacao` | `automacao_id` | `ResultadoCompilacao` (texto: "Compilou sem erros" ou lista `arquivo:linha:coluna mensagem`) | `POST /automacoes/{id}/compilar` | leitura (efeito: cache de compilação) |

### Execuções, pausas, segredos e configuração

| Ferramenta | Entrada | Saída | API | Annotations |
|------------|---------|-------|-----|-------------|
| `listar_execucoes` | `automacao_id?`, `estado?`, `conversa_id?`, `limite?`, `cursor?` | página de `Execucao` | `GET /automacoes/{id}/execucoes` ou `GET /execucoes` | leitura |
| `ver_execucao` | `execucao_id` | `ExecucaoDetalhe` (log completo, stack, ações, tokens) | `GET /execucoes/{id}` | leitura |
| `listar_pausas` | `motivo?` | `{pausas: Pausa[]}` | `GET /pausas` | leitura |
| `pausar_conversa` | `conversa_id`, `duracao_min?` (ausente = sem prazo) | `Pausa` | `POST /conversas/{id}/pausa` (`motivo:"manual"`) | escrita local |
| `retomar_conversa` | `conversa_id` | `{ok}` | `DELETE /conversas/{id}/pausa` | escrita local |
| `listar_segredos` | — | `{segredos: Segredo[]}` (**só nomes**) | `GET /segredos` | leitura |
| `ver_configuracao_automacoes` | — | `ConfiguracaoAutomacoes & {ia: ConfiguracaoIA}` | `GET /automacoes/configuracao` + `GET /ia/configuracao` | leitura |

Não há ferramentas para definir/ler valores de segredos nem para alterar a configuração global
de anti-loop (decisão: limites de segurança só pela janela; a IA pode apenas ler).

## Testes (mcp/tests)

- Paridade: `listTools()` = união das tabelas de `specs/001-zapdesk-mvp/contracts/mcp-ferramentas.md`
  e deste arquivo, com annotations conferidas.
- Contra motor real em modo falso com `--ia=falsa` e runner compilado: criar automação de IA,
  escrever `index.ts`, compilar com erro e sem erro, testar (nada em `/v1/falso/enviadas`), ativar,
  injetar mensagem, `listar_execucoes` → `ok`; `mover_card_funil` por telefone; `listar_segredos`
  não contém valores.
