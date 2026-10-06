# Contrato da API HTTP do motor — acréscimos da feature 002 (v1, compatível)

Base, autenticação, formato, paginação e envelope de erro: **iguais ao MVP**
(`specs/001-zapdesk-mvp/contracts/api-http.md`). Tudo aqui é acréscimo compatível à v1.
Tipos de definição (gatilhos, condições, ações, grafo do chatbot, manifesto): `formatos.md`.
Eventos: `eventos-ws.md`. Tipos TypeScript: `compartilhado/cliente-motor/src/tipos.ts`.

## Novos códigos de erro

| Código | HTTP | Uso |
|--------|------|-----|
| `definicao_invalida` | 422 | fluxo/chatbot/manifesto inválido; `detalhes.erros: ErroDefinicao[]` |
| `compilacao_falhou` | 422 | ativar/testar IA com erro de compilação; `detalhes.erros: ErroCompilacao[]` |
| `runner_indisponivel` | 503 | motor sem `--runner-exec`/`--runner-script` (IA não pode executar) |
| `ia_nao_configurada` | 409 | "Testar chave" sem chave; `detalhes.segredo = "ANTHROPIC_API_KEY"` |
| `ia_erro` | 502 | Claude API respondeu erro no "Testar chave"; `detalhes.status`, `detalhes.request_id` |

Existentes reutilizados: `validacao`, `nao_encontrado`, `conflito` (nome duplicado; hash de
arquivo divergente), `transicao_invalida`, `variaveis_faltando`, `conta_indisponivel`.

## Tipos (JSON)

```text
Funil          { id, nome, ordem, etapas: Etapa[], total_cards, criado_em, atualizado_em }
Etapa          { id, funil_id, nome, cor, ordem, total_cards }
Card           { lead_id, funil_id, etapa_id, desde,
                 lead: {id, telefone, nome|null, campos},
                 etiquetas: Etiqueta[], conversa_id|null, conta_id|null }
MovimentoFunil { id, lead_id, funil_id, etapa_origem_id|null, etapa_origem_nome|null,
                 etapa_destino_id|null, etapa_destino_nome|null,
                 origem: "app"|"mcp"|"automacao", automacao_id|null, execucao_id|null, em }

Automacao      { id, tipo: "fluxo"|"chatbot"|"ia", nome, descricao|null, ativa,
                 contas: string[]|null, incluir_grupos, prioridade, conta_envio_id|null,
                 gatilhos: Gatilho[], definicao: DefinicaoFluxo|DefinicaoChatbot|null,
                 limites: Limites, versao,
                 ia: { pasta, permissoes: Permissao[], segredos: string[],
                       hash_compilado|null, compilacao_ok: bool, erros_compilacao: ErroCompilacao[],
                       compilado_em|null, rodando_versao_anterior: bool } | null,
                 avisos: ErroDefinicao[],            // referências quebradas (não bloqueiam)
                 erros_seguidos, desativada_motivo: "erros_seguidos"|"usuario"|null,
                 estatisticas_24h: { ok, erro, abortada, duracao_media_ms|null },
                 sessoes_ativas: number,             // chatbot; 0 nos demais
                 criada_em, atualizada_em }
Limites        { anti_loop: {mensagens, janela_min}|null, tempo_s|null, memoria_mb|null }
ErroDefinicao  { caminho: string, no_id|null, acao_id|null, mensagem }    // caminho ex.: "definicao.acoes[2].etapa_id"
ErroCompilacao { arquivo, linha, coluna, mensagem, tipo: "sintaxe"|"importacao"|"manifesto"|"resolucao"|"outro" }   // linha e coluna 1-base
ResultadoCompilacao { ok, erros: ErroCompilacao[], avisos: ErroCompilacao[], hash|null,
                      handlers: ("aoReceberMensagem"|"aoAgendar"|"aoExecutar"|"aoEvento")[], duracao_ms }

Execucao       { id, automacao_id, automacao_nome, tipo_automacao, automacao_versao,
                 gatilho: {tipo, dados}, origem: "gatilho"|"manual_app"|"manual_mcp"|"teste"|"fluxo"|"chatbot",
                 origem_execucao_id|null, conta_id|null, conversa_id|null, contato_id|null, lead_id|null,
                 estado: "na_fila"|"rodando"|"aguardando"|"ok"|"erro"|"simulacao"|"abortada",
                 simulacao, motivo|null, erro|null,
                 acoes: AcaoRegistrada[], tokens: Tokens, retorno: any|null,
                 retomar_em|null, iniciada_em, finalizada_em|null, duracao_ms|null }
ExecucaoDetalhe = Execucao & { log, log_truncado, erro_stack|null, variaveis }
AcaoRegistrada { tipo, alvo|null, resultado: "ok"|"falhou"|"bloqueada"|"simulada", detalhe|null, em }
Tokens         { entrada, saida, por_modelo: { [modelo]: {entrada, saida, chamadas} } }

SessaoChatbot  { id, automacao_id, automacao_nome, conversa_id, versao, no_atual, variaveis,
                 tentativas, estado: "ativa"|"concluida"|"humano"|"expirada"|"abortada",
                 motivo|null, expira_em, iniciada_em, atualizada_em, finalizada_em|null }
Pausa          { conversa_id, motivo: "humano"|"anti_loop"|"manual", ate|null, automacao_id|null, criada_em }
EstadoConversaAutomacoes { conversa_id, pausa: Pausa|null, sessao: SessaoChatbot|null,
                           pausa_geral: bool }
ConfiguracaoAutomacoes { anti_loop_mensagens, anti_loop_janela_min, pausa_anti_loop_min,
                         pausa_humana_min, primeiros_contatos_hora, tempo_ia_s, memoria_ia_mb,
                         processos_ia_max, ociosidade_ia_min, pausa_geral }
                 // padrões: anti-loop 10 mensagens a cada 10 min (decisão do orquestrador,
                 // era 5), pausa anti-loop 60, pausa humana 30, 20 primeiros contatos/h,
                 // IA 60 s / 256 MB / 4 processos / 5 min; faixas em data-model.md
ArquivoProjeto { caminho, tamanho, hash, atualizado_em }
ConteudoArquivo { caminho, conteudo, hash, atualizado_em }
ModeloProjeto  { id: "responder_historico"|"classificar_funil"|"extrair_dados"|"em_branco", nome, descricao }
SdkAutomacao   { versao, tipos: string /* .d.ts */, esquema_manifesto: object /* JSON Schema */ }
ConfiguracaoIA { modelo_padrao, modelos: [{id, nome, aviso|null}], chave_configurada: bool }
Segredo        { nome, reservado: bool, usado_por: [{automacao_id, nome}] }
ResultadoTeste { execucao: ExecucaoDetalhe }           // estado "simulacao" ou "erro"
```

`Mensagem` (MVP) ganha `automacao_id: string|null` (acréscimo compatível).

## Funis

| Método | Caminho | Entrada | Saída / erros |
|--------|---------|---------|---------------|
| GET | `/funis` | — | `Funil[]` (ordem: `ordem`) |
| POST | `/funis` | `{"nome":"1–60, único","etapas"?:[{"nome","cor"?}]}` | `201 Funil` / `validacao`, `conflito` |
| GET | `/funis/{id}` | — | `Funil` |
| PATCH | `/funis/{id}` | `{"nome"?, "ordem"?}` | `Funil` / `validacao`, `conflito` |
| DELETE | `/funis/{id}` | — | `204` (apaga etapas, cards e histórico do funil) |
| POST | `/funis/{id}/etapas` | `{"nome":"1–40","cor"?:"#RRGGBB","posicao"?:n}` | `201 Etapa` / `validacao` (máx. 30), `conflito` |
| PUT | `/funis/{id}/etapas/ordem` | `{"etapa_ids":[...todas...]}` | `Funil` / `validacao` (lista incompleta) |
| PATCH | `/etapas/{id}` | `{"nome"?,"cor"?}` | `Etapa` |
| DELETE | `/etapas/{id}` | `?destino_etapa_id=` **ou** `?remover_cards=true` (obrigatório se houver cards) | `204` / `validacao` "Escolha para onde mover os cards." |
| GET | `/funis/{id}/cards` | `?etapa_id=&busca=&limite&cursor` | página de `Card` (ordem: `desde` desc) |
| PUT | `/funis/{id}/cards` | `{"lead_id"?\|"contato_id"?\|"telefone"?, "etapa_id", "origem"?:"app"\|"mcp"}` (exatamente um alvo) | `200 Card` (moveu) / `201 Card` (entrou) / `validacao` (grupo, telefone inválido, etapa de outro funil) |
| DELETE | `/funis/{id}/cards/{lead_id}` | `?origem=app\|mcp` | `204` (idempotente) |
| GET | `/funis/{id}/historico` | `?lead_id=&limite&cursor` | página de `MovimentoFunil` (mais recente primeiro) |
| GET | `/leads/{id}/funis` | — | `[{funil: {id, nome}, card: Card\|null}]` |

`telefone` no `PUT .../cards`: normalizado (E.164); cria o lead com origem `mcp`/`app` se não
existir (origem do lead = `mcp` quando `origem=mcp`, senão `contatos`).

## Leads e disparos (acréscimos)

| Método | Caminho | Entrada | Saída / erros |
|--------|---------|---------|---------------|
| PATCH | `/leads/{id}` | `{"nome"?: string\|null, "campos"?: {chave: valor\|null}}` (`null` remove o campo; merge) | `Lead` / `validacao` |
| POST | `/disparos/{id}/destinatarios` | `{"lead_ids":[...]}` (1–10.000) | `{adicionados, ja_existiam, disparo: Disparo}` / `transicao_invalida` (`concluido`/`cancelado`), `variaveis_faltando` |

## Automações (todas)

| Método | Caminho | Entrada | Saída / erros |
|--------|---------|---------|---------------|
| GET | `/automacoes` | `?tipo=&ativa=true\|false&busca=` | `Automacao[]` (ordem: `prioridade`, `criada_em`) |
| POST | `/automacoes` | `NovaAutomacao` (tipo `fluxo`\|`chatbot`) | `201 Automacao` (inativa) / `validacao`, `definicao_invalida` (só estrutura JSON; referências viram `avisos`) |
| POST | `/automacoes/ia` | `{"nome":"1–80","modelo":"responder_historico"\|"classificar_funil"\|"extrair_dados"\|"em_branco","descricao"?}` | `201 Automacao` (cria pasta, arquivos do modelo e compila) |
| POST | `/automacoes/validar` | `NovaAutomacao` (sem gravar) | `200 {erros: ErroDefinicao[], avisos: ErroDefinicao[]}` |
| GET | `/automacoes/{id}` | — | `Automacao` |
| PATCH | `/automacoes/{id}` | campos de `NovaAutomacao` (fluxo/chatbot) | `Automacao` (versão +1 se `gatilhos`/`definicao`/`limites` mudarem) / `validacao` ("Edite automacao.json" para `ia`), `definicao_invalida` |
| DELETE | `/automacoes/{id}` | — | `204` (encerra runner, apaga pasta/memória/execuções/sessões/esperas; sessões ativas → `abortada` antes) |
| POST | `/automacoes/{id}/ativar` | — | `Automacao` / `definicao_invalida`, `compilacao_falhou`, `validacao` (sem gatilho) |
| POST | `/automacoes/{id}/desativar` | — | `Automacao` (`desativada_motivo="usuario"`; apaga esperas de gatilho) |
| POST | `/automacoes/{id}/executar` | `AlvoExecucao & {"entrada"?: any, "origem"?: "manual_app"\|"manual_mcp"}` | `202 Execucao` (`na_fila`) / `runner_indisponivel`, `nao_encontrado` |
| POST | `/automacoes/{id}/testar` | `PedidoTeste` | `200 ResultadoTeste` (aguarda até `tempo_s` + 5 s) / `compilacao_falhou`, `runner_indisponivel` |
| GET | `/automacoes/{id}/execucoes` | `?estado=&conversa_id=&limite&cursor` | página de `Execucao` (mais recente primeiro) |
| GET | `/execucoes/{id}` | — | `ExecucaoDetalhe` |
| GET | `/execucoes` | `?estado=&conversa_id=&limite&cursor` | página de `Execucao` de todas as automações |
| GET | `/automacoes/{id}/sessoes` | `?estado=&limite&cursor` | página de `SessaoChatbot` |

```json
// NovaAutomacao
{ "tipo": "fluxo", "nome": "Respondeu → quente", "descricao": null,
  "contas": null, "incluir_grupos": false, "prioridade": 100, "conta_envio_id": null,
  "gatilhos": [ { "tipo": "disparo_respondeu" } ],
  "definicao": { "versao": 1, "condicoes": null, "acoes": [ /* Acao[] */ ] },
  "limites": { "anti_loop": null } }

// AlvoExecucao — no máximo um; nenhum = sem alvo
{ "conversa_id": "01J..." } | { "contato_id": "01J..." } | { "lead_id": "01J..." }
| { "telefone": "+5511...", "conta_id": "01J..." }

// PedidoTeste
{ "mensagem": { "texto": "Quanto custa?", "conversa_id": "01J..."? },  // aoReceberMensagem / gatilho de mensagem
  "mensagem_id": "01J..."?,        // usa uma mensagem real recebida como gatilho
  "entrada": { }?,                 // aoExecutar
  "evento": { "tipo": "etiqueta", "dados": {} }?,  // aoEvento / gatilhos não-mensagem
  "alvo": AlvoExecucao?,           // conversa/lead de contexto
  "ia_simulada": false }
```

`testar` vale para `fluxo` e `ia`; para `chatbot` use o simulador abaixo (`testar` num chatbot →
`validacao` "Use o chat simulado"). Sem `conversa_id` no teste de mensagem, o motor usa uma **conversa fictícia** (contato "Contato de
teste", `+5500000000000`, sem histórico) que só existe na execução.

### Simulador de chatbot (chat simulado do editor)

Estado em memória do motor; sessões simuladas expiram após 30 min ociosas; nada é gravado além da
execução `simulacao` de cada rodada.

| Método | Caminho | Entrada | Saída |
|--------|---------|---------|-------|
| POST | `/automacoes/{id}/simulador` | `{"definicao"?: DefinicaoChatbot, "alvo"?: AlvoExecucao, "ia_simulada"?: bool}` (sem `definicao` usa a gravada) | `201 {simulacao_id, saidas: SaidaSimulada[], no_atual, variaveis, estado}` |
| POST | `/simulador/{simulacao_id}/mensagens` | `{"texto":"2"}` | `{saidas, no_atual, variaveis, estado, acoes: AcaoRegistrada[]}` |
| DELETE | `/simulador/{simulacao_id}` | — | `204` |

`SaidaSimulada = {tipo: "mensagem"|"acao"|"aviso", texto, no_id}`.

## Automações de IA — projeto e compilação

| Método | Caminho | Entrada | Saída / erros |
|--------|---------|---------|---------------|
| GET | `/automacoes/modelos` | — | `ModeloProjeto[]` |
| GET | `/automacoes/sdk` | — | `SdkAutomacao` |
| GET | `/automacoes/{id}/arquivos` | — | `ArquivoProjeto[]` (sem `.zapdesk/` e `tsconfig.json`) |
| GET | `/automacoes/{id}/arquivos/{caminho...}` | — | `ConteudoArquivo` / `nao_encontrado` |
| PUT | `/automacoes/{id}/arquivos/{caminho...}` | `{"conteudo": string, "hash_anterior"?: string\|null}` (`null` = deve ser novo) | `200/201 ArquivoProjeto` / `conflito` (`detalhes.hash_atual`) "O arquivo foi alterado fora do app.", `validacao` (caminho, extensão, tamanho, limite de arquivos) |
| DELETE | `/automacoes/{id}/arquivos/{caminho...}` | — | `204` / `validacao` (manifesto ou entrada) |
| POST | `/automacoes/{id}/arquivos/renomear` | `{"de","para"}` | `ArquivoProjeto` / `validacao`, `conflito` (destino existe) |
| POST | `/automacoes/{id}/compilar` | — | `200 ResultadoCompilacao` (também atualiza `Automacao.ia` e publica `automacao.atualizada`) |

Regras:

- Toda escrita de arquivo publica `automacao.arquivos_alterados` e **não** compila sozinha (o
  editor chama `compilar` após 800 ms sem digitar; o MCP chama explicitamente).
- Execução real e teste: se `hash_fontes` difere do conteúdo atual, o motor recompila antes;
  execução real usa a última compilação **ok** (se a nova falhar, roda a anterior e marca
  `rodando_versao_anterior`); teste usa a nova e falha com `compilacao_falhou`.
- Compilar valida também o manifesto (`automacao.json`, `tipo:"manifesto"` nos erros) e o
  handler exigido por cada gatilho declarado.

## Conversas — estado das automações e pausas

| Método | Caminho | Entrada | Saída |
|--------|---------|---------|-------|
| GET | `/conversas/{id}/automacoes` | — | `EstadoConversaAutomacoes` |
| POST | `/conversas/{id}/pausa` | `{"motivo":"humano"\|"manual","duracao_min"?: number\|null}` (`null`/ausente = sem prazo) | `Pausa` ("Assumir"; encerra sessão de bot como `humano`) |
| DELETE | `/conversas/{id}/pausa` | — | `204` ("Devolver às automações"/"Retomar") |
| GET | `/pausas` | `?motivo=` | `Pausa[]` ativas |

## Configuração, IA e segredos

| Método | Caminho | Entrada | Saída / erros |
|--------|---------|---------|---------------|
| GET | `/automacoes/configuracao` | — | `ConfiguracaoAutomacoes` |
| PATCH | `/automacoes/configuracao` | campos parciais | `ConfiguracaoAutomacoes` / `validacao` (faixas do data-model) |
| GET | `/ia/configuracao` | — | `ConfiguracaoIA` |
| PATCH | `/ia/configuracao` | `{"modelo_padrao": "claude-…"}` | `ConfiguracaoIA` / `validacao` |
| POST | `/ia/testar-chave` | `{"modelo"?}` | `{ok: true, modelo, latencia_ms}` / `ia_nao_configurada`, `ia_erro` (401 → "Chave da Anthropic inválida.") |
| GET | `/segredos` | — | `Segredo[]` (**somente nomes**; valores nunca saem do motor) |

Não existe rota para definir, ler ou apagar valores de segredos: isso é feito pelo app via stdin
(`runtime.md`).

## Sistema (acréscimo)

`GET /sistema` ganha `automacoes_ativas`, `processos_ia` (vivos) e `runner_disponivel: bool`.
