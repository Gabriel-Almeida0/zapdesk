# Contrato do protocolo motor ↔ runner — v1

Transporte: stdin/stdout do processo runner (`runtime.md` › Runner). **JSON-RPC 2.0**, uma
mensagem por linha (NDJSON, UTF-8, `\n`), no máximo **1 MB por linha** (acima disso o receptor
responde erro `-32600` e descarta). Os dois lados fazem requisições (`id` numérico crescente por
lado) e notificações (sem `id`). stdout do runner é **exclusivo** do protocolo: o runner
substitui `process.stdout.write`/`console.*` antes de carregar código do usuário. stderr é livre
(capturado pelo motor).

Implementações: Go em `motor/internal/automacoes/runner/` (lado motor) e TypeScript em
`automacao/runner/src/` (lado runner; tipos em `automacao/runner/src/protocolo.ts`). Campos em
`snake_case`. O runner nunca recebe token HTTP, chave da Anthropic, caminho do banco ou da pasta de
dados (só o caminho do bundle).

## Ciclo de vida

```text
motor spawn → runner envia notificação "pronto" {versao_runner, protocolo: 1, node}
motor → "inicializar" → runner importa o bundle, valida a definição → responde handlers
motor → "executar" (N, concorrentes) ⇄ runner → "ctx.*" (N) → runner responde "executar"
motor → "cancelar" (notificação)             # prazo ou exclusão; best-effort
motor → "encerrar" → runner responde {} e sai com código 0 (motor faz SIGKILL após 2 s)
stdin EOF → runner sai com código 0 imediatamente
```

Se o runner não enviar `pronto` em 5 s ou não responder `inicializar` em 10 s → motor mata o
processo e a execução que aguardava fica `erro` ("O processo da automação não iniciou").

## Motor → runner

### `inicializar` (requisição)

```jsonc
{ "protocolo": 1,
  "automacao": { "id": "01J...", "nome": "Responder com IA", "versao": 7 },
  "bundle": "/…/automacoes-compiladas/01J…/9f2c….mjs",
  "hash": "9f2c…",
  "permissoes": ["ler_conversas", "enviar", "ia"],
  "segredos": { "OPENAI_KEY": "…" },       // só os declarados e existentes
  "fuso": "America/Sao_Paulo" }
→ { "handlers": ["aoReceberMensagem", "aoExecutar"] }
```

Erros: `2001 bundle_invalido` (import falhou, export default não é `definirAutomacao`), com
`data.stack`.

### `executar` (requisição)

```jsonc
{ "execucao_id": "01J...",
  "handler": "aoReceberMensagem",          // | "aoAgendar" | "aoExecutar" | "aoEvento"
  "simulacao": false,
  "prazo_ms": 58000,                       // o motor mata o processo em prazo_ms + 2000
  "info": { /* InfoExecucao da SDK, sem prazoMs */ },
  "conversa": { "id", "conta_id", "tipo", "nome", "telefone", "lead_id",
                "contato": { "id", "nome", "nome_push", "telefone", "notas", "etiquetas": [] } } | null,
  "argumento": { /* MensagemRecebida | Agendamento | entrada Json | EventoAutomacao, em camelCase da SDK */ } }
→ { "retorno": <Json|null> }
```

Grafia (esclarecimento da implementação do runner, 2026-09-27): `info` e `argumento` vão em
camelCase (formato da SDK); o runner também aceita snake_case nas chaves de `info` e do argumento
de `aoReceberMensagem`/`aoAgendar`/`aoEvento` (converte), mas **nunca** mexe no argumento de
`aoExecutar` (JSON livre do usuário). Parâmetros e resultados de `ctx.*` são snake_case dos dois
lados; valores livres (`valor`, `campos`, `dados`, `esquema`, `variaveis`) não são convertidos.
`retorno` é sempre `null` para handlers que não sejam `aoExecutar`.

Erro do handler → resposta de erro `2002 erro_usuario` com
`data: {codigo: "erro_usuario", nome, mensagem, stack, codigo_sdk?}` (stack já mapeado para o
`.ts` via sourcemap inline; `codigo_sdk` = `ErroAutomacao.codigo` quando o erro é da SDK, ex.:
`permissao_negada`). Retorno de `aoExecutar` que não é JSON ou passa de 64 KB também vira `2002`.
Handler ausente → `2003 handler_ausente`.

### `cancelar` (notificação)

`{ "execucao_id" }` → o runner aborta o `AbortSignal` do `ctx` e passa a rejeitar chamadas `ctx.*`
dessa execução com `ErroExecucaoEncerrada`. Não garante parar laços síncronos (o motor mata o
processo no prazo).

### `encerrar` (requisição) → `{}`; `ping` (requisição) → `{ "ok": true }`

## Runner → motor

### Notificações

| Método | Parâmetros | Uso |
|--------|------------|-----|
| `pronto` | `{versao_runner, protocolo, node}` | processo pronto para `inicializar` |
| `log` | `{execucao_id\|null, nivel: "debug"\|"info"\|"aviso"\|"erro", texto, em}` | `ctx.log.*` e `console.*`; `texto` ≤ 8 KB por linha (objetos via `util.inspect`, profundidade 4) |
| `http` | `{execucao_id, metodo, url_sem_query, status\|null, duracao_ms, erro\|null}` | registro de cada `ctx.http.fetch` (vira `AcaoRegistrada` tipo `http`) |

### Requisições `ctx.*`

Todas levam `execucao_id`. O motor responde erro `1007 execucao_encerrada` se a execução já
terminou, e `1001 permissao_negada` se o manifesto não declara a permissão exigida.

| Método | Parâmetros (além de `execucao_id`) | Resultado | Permissão |
|--------|------------------------------------|-----------|-----------|
| `ctx.conversa.historico` | `{conversa_id?, limite?, antes?}` | `Mensagem[]` | `ler_conversas` |
| `ctx.enviar` | `{destino: {conversa_id}\|{telefone, conta_id?}, conteudo: {texto}\|{template, variaveis?}\|{arquivo_id, legenda?, como?}, citar_mensagem_id?}` | `MensagemEnviada` | `enviar` |
| `ctx.reagir` | `{mensagem_id, emoji}` | `{}` | `enviar` |
| `ctx.etiquetas.listar` | `{}` | `Etiqueta[]` | `etiquetas` |
| `ctx.etiquetas.do_contato` | `{alvo?}` | `Etiqueta[]` | `etiquetas` |
| `ctx.etiquetas.adicionar` / `.remover` | `{etiqueta, alvo?}` | `{}` | `etiquetas` |
| `ctx.funil.listar` | `{}` | `Funil[]` | `funil` |
| `ctx.funil.posicao` | `{funil, alvo?}` | `PosicaoFunil\|null` | `funil` |
| `ctx.funil.mover` | `{funil, etapa, alvo?}` | `PosicaoFunil` | `funil` |
| `ctx.funil.remover` | `{funil, alvo?}` | `{}` | `funil` |
| `ctx.leads.atual` | `{}` | `Lead\|null` | `leads` |
| `ctx.leads.obter` | `{id}` | `Lead\|null` | `leads` |
| `ctx.leads.buscar_telefone` | `{telefone}` | `Lead\|null` | `leads` |
| `ctx.leads.atualizar` | `{nome?, campos?, alvo?}` | `Lead` | `leads` |
| `ctx.memoria.obter` | `{chave, escopo}` | `{valor: Json\|null}` | — |
| `ctx.memoria.definir` | `{chave, escopo, valor}` | `{}` | — |
| `ctx.memoria.remover` | `{chave, escopo}` | `{removida: bool}` | — |
| `ctx.memoria.listar` | `{escopo, prefixo?}` | `[{chave, valor}]` | — |
| `ctx.ia.gerar` | `{prompt?, mensagens?, sistema?, modelo?, max_tokens?}` | `{texto, modelo, motivo_parada, tokens}` | `ia` |
| `ctx.ia.classificar` | `{texto, categorias: {nome: descricao\|null}, instrucoes?, modelo?, sistema?, max_tokens?}` | `{categoria, tokens}` | `ia` |
| `ctx.ia.extrair` | `{texto, esquema, instrucoes?, modelo?, sistema?, max_tokens?}` | `{dados, tokens}` | `ia` |
| `ctx.agendar` | `{em, dados?, na_conversa}` | `{id, em}` | `agendar` |
| `ctx.cancelar_agendamento` | `{id}` | `{cancelado: bool}` | `agendar` |
| `ctx.humano.transferir` | `{motivo?, mensagem?, duracao_min?}` | `{}` | `enviar` |
| `ctx.notificar` | `{titulo, texto}` | `{}` | — |

`alvo`: `{contato_id}|{lead_id}|{telefone}|{conversa_id}`; ausente = alvo da execução (erro
`1002 validacao` "Esta execução não tem contato" se não houver).

Em `simulacao: true`, o motor responde às escritas com o resultado simulado (ex.: `MensagemEnviada`
com `id: null, simulada: true`), registra `AcaoRegistrada{resultado:"simulada"}` e não altera
nada; `ctx.memoria.definir/remover` gravam num mapa da execução; `ctx.agendar` devolve id
simulado.

## Códigos de erro (campo `error.code`)

| Código | Nome (`error.data.codigo`) | Classe na SDK | Mensagem típica |
|--------|----------------------------|---------------|-----------------|
| -32700/-32600/-32601/-32602 | padrão JSON-RPC | `ErroAutomacao` | — |
| 1001 | `permissao_negada` | `ErroPermissao` (`data.permissao`) | "Permissão 'enviar' não declarada em automacao.json" |
| 1002 | `validacao` | `ErroValidacao` (`data.campos`) | "Texto vazio." |
| 1003 | `nao_encontrado` | `ErroNaoEncontrado` | "Etapa 'Proposta' não encontrada no funil 'Vendas'." |
| 1004 | `bloqueado` | `ErroBloqueado` (`data.motivo`) | "Conversa em atendimento humano até 14:30." |
| 1005 | `ia_nao_configurada` | `ErroIA` | "Configure a chave da Anthropic em Ajustes → IA" |
| 1006 | `ia_erro` | `ErroIA` (`data.status`, `data.request_id`) | "A Claude API está sobrecarregada (529). Tente mais tarde." |
| 1007 | `execucao_encerrada` | `ErroExecucaoEncerrada` | "A execução já terminou." |
| 1008 | `limite` | `ErroValidacao` | "Valor maior que 64 KB." / "Máximo de 5 notificações por execução." |
| 1009 | `conta_indisponivel` | `ErroAutomacao` | "Conta desconectada." |
| 1010 | `segredo` | `ErroSegredo` | (lado runner; nunca vai ao motor) |
| 2001 | `bundle_invalido` | — | resposta a `inicializar` |
| 2002 | `erro_usuario` | — | resposta a `executar` |
| 2003 | `handler_ausente` | — | "Handler aoEvento não exportado." |

## Falhas do processo (lado motor)

| Situação | Execução(ões) em andamento no processo | Processo |
|----------|----------------------------------------|----------|
| prazo excedido | a que estourou: `erro` "Tempo limite de 60 s excedido"; demais: `erro` "Interrompida porque outra execução desta automação excedeu o tempo" | SIGKILL, reinicia sob demanda |
| saída inesperada / OOM (stderr contém `heap out of memory`) | `erro` "Memória excedida (256 MB)" ou "O processo da automação terminou inesperadamente (código N)" + últimas linhas do stderr no log | reinicia sob demanda |
| linha inválida / > 1 MB | a execução citada (se houver) `erro` | mantido |
| `encerrar` por ociosidade / hash novo / segredo alterado | nenhuma (só sem execuções) | encerrado |

Cada `erro` conta para `erros_seguidos` (5 → desativa + notificação `desativada`).
