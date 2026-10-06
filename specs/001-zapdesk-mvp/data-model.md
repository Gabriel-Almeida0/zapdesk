# Modelo de dados — ZapDesk MVP

Fonte: seção 6 de `docs/features/zapdesk.md` + clarificações de `spec.md`.

## Visão geral do armazenamento

Pasta de dados: `~/Library/Application Support/ZapDesk/` (sobrescrevível por `--pasta-dados` para
testes).

```text
ZapDesk/
├── zapdesk.db              # SQLite do app (dados de domínio) — dono: motor
├── zapdesk.db.bak-<versao> # backup automático antes de cada migração
├── sessoes/<conta_id>.db   # sqlstore do whatsmeow, um banco por conta
├── midia/<conta_id>/...    # cache de mídia baixada sob demanda
├── anexos/<arquivo_id>     # arquivos enviados pelo usuário/MCP (anexos, áudios gravados)
├── logs/motor.log          # log estruturado rotativo (sem conteúdo de mensagens)
├── motor.lock              # trava de instância única do motor
└── runtime.json            # {porta, token, pid_app, pid_motor, versao, iniciado_em} — 0600
```

Convenções:

- IDs internos: ULID em texto (ordenáveis por criação).
- Datas no banco: inteiro, milissegundos Unix UTC. Na API: string RFC 3339 com fuso.
- Horas de janela: texto `HH:MM` no fuso local do Mac.
- Booleanos: inteiro 0/1. JSON: texto.
- `PRAGMA journal_mode=WAL`, `foreign_keys=ON`, `busy_timeout=5000`.
- Migrações versionadas em `motor/internal/armazenamento/migracoes/NNNN_nome.sql`, registradas
  em `schema_migracoes`; backup do arquivo antes de aplicar qualquer migração pendente.

## Entidades

### Conta (`contas`)

| Campo | Tipo | Obrigatório | Default | Validação |
|-------|------|-------------|---------|-----------|
| id | texto (ULID) | sim | gerado | — |
| jid | texto | após conectar | nulo | JID do WhatsApp (`<numero>@s.whatsapp.net`) |
| telefone | texto | após conectar | nulo | E.164 |
| nome | texto | sim | pushname ou "Nova conta" | 1–60 caracteres |
| estado | enum | sim | `desconectada` | `conectando`, `conectada`, `desconectada`, `banida` |
| sincronizando | bool | sim | 0 | verdadeiro durante history sync |
| criada_em | data | sim | agora | — |
| atualizada_em | data | sim | agora | — |

Transições: `desconectada → conectando` (conectar/reconectar) `→ conectada` (QR aceito ou sessão
restaurada); `conectada → desconectada` (logout remoto, sessão expirada); `conectada/conectando →
banida` (ban temporário/permanente detectado); `banida → conectando` (reconectar manual).
Queda de rede NÃO muda para `desconectada` (continua `conectada` com indicador "Sem conexão").
Efeitos: `→ desconectada` pausa os disparos da conta (motivo `conta_desconectada`); `→ banida`
pausa os disparos com motivo `conta_banida` e não permite retomar enquanto banida.

### Contato (`contatos`)

| Campo | Tipo | Obrigatório | Validação |
|-------|------|-------------|-----------|
| id | texto | sim | — |
| conta_id | texto | sim | FK contas, `ON DELETE CASCADE` |
| jid | texto | sim | único por conta |
| telefone | texto | não | E.164 (nulo para JIDs sem telefone conhecido, ex.: LID) |
| nome | texto | não | nome salvo na agenda do celular |
| nome_push | texto | não | pushname |
| notas | texto | não | livre, até 10.000 caracteres |
| lead_id | texto | não | FK leads (`ON DELETE SET NULL`), ligado por telefone igual |
| criado_em / atualizado_em | data | sim | — |

Único: `(conta_id, jid)`. Índice: `(conta_id, telefone)`.

### Etiqueta (`etiquetas`) e `contato_etiquetas`

| Campo | Tipo | Obrigatório | Validação |
|-------|------|-------------|-----------|
| id | texto | sim | — |
| nome | texto | sim | único (sem diferenciar maiúsculas), 1–30 caracteres |
| cor | texto | sim | hex `#RRGGBB` |
| criada_em | data | sim | — |

`contato_etiquetas(contato_id, etiqueta_id)` PK composta, ambas FK com cascade. Etiquetas são
globais (valem para contatos de todas as contas).

### Conversa (`conversas`)

| Campo | Tipo | Obrigatório | Validação |
|-------|------|-------------|-----------|
| id | texto | sim | — |
| conta_id | texto | sim | FK contas cascade |
| jid | texto | sim | único por conta; individual ou grupo (`@g.us`) |
| tipo | enum | sim | `individual`, `grupo` |
| nome | texto | não | nome do grupo ou do contato |
| contato_id | texto | não | FK contatos (individual) |
| nao_lidas | inteiro | sim | ≥ 0 |
| ultima_mensagem_em | data | não | — |
| ultima_mensagem_resumo | texto | não | até 200 caracteres |

Único: `(conta_id, jid)`. Índice: `(conta_id, ultima_mensagem_em DESC)`.

### Mensagem (`mensagens`)

| Campo | Tipo | Obrigatório | Validação |
|-------|------|-------------|-----------|
| id | texto | sim | — |
| conta_id | texto | sim | FK |
| conversa_id | texto | sim | FK cascade |
| wa_id | texto | sim | id do WhatsApp; único por `(conversa_id, wa_id)` |
| remetente_jid | texto | sim | — |
| de_mim | bool | sim | — |
| tipo | enum | sim | `texto`, `imagem`, `video`, `audio`, `documento`, `figurinha`, `sistema` |
| texto | texto | não | corpo ou legenda |
| midia | JSON | não | `{mimetype, tamanho, nome_arquivo, duracao_s, ptt, largura, altura, miniatura_b64, chave_download}` (`chave_download` = dados opacos para baixar depois) |
| midia_caminho | texto | não | caminho local no cache quando já baixada |
| citacao_wa_id | texto | não | wa_id citado |
| citacao_resumo | texto | não | trecho da citada para exibir |
| editada | bool | sim | 0 |
| apagada | bool | sim | 0 (apagar para todos → texto/mídia limpos, `apagada=1`) |
| estado | enum | sim | `pendente`, `enviada`, `entregue`, `lida`, `falhou` (recebidas: `recebida`) |
| erro | texto | não | motivo quando `falhou` |
| disparo_id | texto | não | preenchido quando enviada por disparo |
| enviada_em | data | sim | timestamp do WhatsApp (ou local ao criar) |

Índices: `(conversa_id, enviada_em DESC)`; `(conta_id, remetente_jid, enviada_em)` (usado para
detectar "respondeu"). Busca: tabela virtual FTS5 `mensagens_fts(texto)` com
`content='mensagens'`, tokenizer `unicode61 remove_diacritics 2`, mantida por triggers.

Reações (`reacoes`): `(mensagem_id, remetente_jid)` PK, `emoji`, `em`. Emoji vazio remove.
Mensagens do tipo reação não viram linha em `mensagens`; atualizam `reacoes`.

Transições de estado (enviadas por mim): `pendente → enviada → entregue → lida`; `pendente →
falhou`; `falhou → pendente` (reenviar). Recibos fora de ordem nunca regridem o estado.

Regras de prazo: editar só se `de_mim` e `agora − enviada_em ≤ 15 min`; apagar para todos só se
`de_mim` e `agora − enviada_em ≤ 48 h` (constantes configuráveis em um único lugar).

### Status (`status_contatos`)

| Campo | Tipo | Obrigatório |
|-------|------|-------------|
| id | texto | sim |
| conta_id | texto | sim |
| wa_id | texto | sim (único por conta) |
| contato_jid | texto | sim |
| tipo | enum | `texto`, `imagem`, `video` |
| texto / midia / midia_caminho | como em Mensagem | não |
| publicado_em | data | sim |

Exibidos apenas se `publicado_em > agora − 24 h`; linhas mais antigas são removidas por limpeza
periódica.

### Lead (`leads`)

| Campo | Tipo | Obrigatório | Default | Validação |
|-------|------|-------------|---------|-----------|
| id | texto | sim | gerado | — |
| telefone | texto | sim | — | normalizado E.164, **único** |
| nome | texto | não | nulo | até 120 caracteres |
| campos | JSON objeto | não | `{}` | chaves = nomes de coluna normalizados (minúsculas, sem acento, espaços → `_`); valores texto |
| origem | enum | sim | — | `csv`, `colado`, `contatos`, `mcp` |
| tem_whatsapp | bool/nulo | não | nulo | preenchido na verificação no envio |
| importado_em | data | sim | agora | nunca alterado após criação |
| atualizado_em | data | sim | agora | — |

Regras (testes obrigatórios — Constituição VI):

1. Normalização: remove tudo que não é dígito ou `+` inicial; `00` inicial vira `+`; sem DDI →
   aplica `+55`; valida com a biblioteca de números de telefone (tipo possível e válido para a
   região); formata E.164. Inválidos entram no relatório com linha e motivo (`vazio`,
   `formato_invalido`, `numero_invalido`).
2. Deduplicação somente por telefone normalizado, contra a base e dentro do lote (primeira
   ocorrência do lote vence; as demais contam como `duplicado_no_lote`).
3. Telefone já existente: não cria; preenche apenas `nome` se nulo e chaves de `campos`
   ausentes/vazias; nunca sobrescreve; `origem` e `importado_em` intactos; relatório `ja_existia`
   com `importado_em` original.
4. A importação inteira acontece numa única transação.

Relatório de importação (retorno, não persistido):
`{total_linhas, novos:[{linha, lead_id, telefone}], ja_existentes:[{linha, lead_id, telefone,
importado_em, campos_preenchidos:[...]}], invalidos:[{linha, valor, motivo}],
duplicados_no_lote:[{linha, telefone, primeira_linha}]}` com a invariante
`len(novos)+len(ja_existentes)+len(invalidos)+len(duplicados_no_lote) = total_linhas`.

"Último disparo" do lead é derivado (`MAX(destinatarios.enviado_em)` por lead).

### Arquivo (`arquivos`)

| Campo | Tipo | Obrigatório | Validação |
|-------|------|-------------|-----------|
| id | texto | sim | — |
| nome | texto | sim | nome original |
| mimetype | texto | sim | detectado pelo conteúdo |
| tamanho | inteiro | sim | ≤ limite do tipo (imagem/vídeo/áudio 16 MB; documento 100 MB; figurinha 1 MB `image/webp`) |
| tipo_midia | enum | sim | `imagem`, `video`, `audio`, `documento`, `figurinha` |
| caminho | texto | sim | dentro de `anexos/` |
| criado_em | data | sim | — |

Anexos de templates e disparos referenciam `arquivos.id` (cópia própria, independente do arquivo
original do usuário).

### Template (`templates`)

| Campo | Tipo | Obrigatório | Validação |
|-------|------|-------------|-----------|
| id | texto | sim | — |
| nome | texto | sim | único (sem diferenciar maiúsculas), 1–60 caracteres |
| texto | texto | sim | não vazio, até 4.096 caracteres |
| arquivo_id | texto | não | FK arquivos |
| criado_em / atualizado_em | data | sim | — |

### Disparo (`disparos`)

| Campo | Tipo | Obrigatório | Default | Validação |
|-------|------|-------------|---------|-----------|
| id | texto | sim | gerado | — |
| conta_id | texto | sim | — | conta existente |
| nome | texto | sim | "Disparo <data>" | 1–80 caracteres |
| mensagem | texto | sim | — | não vazio, até 4.096 caracteres (vira legenda se houver anexo) |
| arquivo_id | texto | não | nulo | arquivo existente |
| intervalo_min_s | inteiro | sim | — | ≥ 1 |
| intervalo_max_s | inteiro | sim | — | ≥ intervalo_min_s |
| limite_por_hora | inteiro | não | nulo | > 0 |
| limite_por_dia | inteiro | não | nulo | > 0 |
| pausa_a_cada | inteiro | não | nulo | > 0; exige `pausa_duracao_s` |
| pausa_duracao_s | inteiro | não | nulo | > 0; exige `pausa_a_cada` |
| inicio_em | data | não | agora | — |
| janela_inicio / janela_fim | texto `HH:MM` | não | nulo | ambos ou nenhum; diferentes; `fim < inicio` = janela que cruza meia-noite |
| falhas_seguidas_max | inteiro | sim | 10 | ≥ 0 (0 desliga a pausa automática) |
| valores_padrao | JSON objeto | não | `{}` | valor por variável |
| estado | enum | sim | `rascunho` | ver máquina abaixo |
| motivo_pausa | enum | não | nulo | `usuario`, `app_fechado`, `motor_reiniciado`, `conta_desconectada`, `conta_banida`, `falhas_seguidas` |
| falhas_seguidas | inteiro | sim | 0 | contador corrente |
| origem | enum | sim | — | `app`, `mcp` |
| proximo_envio_em | data | não | nulo | calculado pelo agendador |
| criado_em / iniciado_em / concluido_em / cancelado_em | data | — | — | — |

Índice: `(conta_id, estado)`.

Máquina de estados (testes obrigatórios):

```text
rascunho ──iniciar──▶ agendado
agendado ──(vez da conta e agora ≥ inicio_em e dentro da janela)──▶ enviando
agendado ──(vez da conta e agora ≥ inicio_em e fora da janela)──▶ fora_da_janela
enviando ──(saiu da janela)──▶ fora_da_janela ──(entrou na janela)──▶ enviando
enviando ──(sem pendentes)──▶ concluido
agendado | enviando | fora_da_janela ──pausar──▶ pausado
pausado ──retomar──▶ agendado      (entra na fila da conta; vira enviando/fora_da_janela na vez)
rascunho | agendado | enviando | fora_da_janela | pausado ──cancelar──▶ cancelado
```

Estados finais: `concluido`, `cancelado`. Transição inválida → erro `transicao_invalida`.
Na inicialização do motor, todo disparo em `enviando`/`fora_da_janela`/`agendado` com
`iniciado_em` preenchido vai para `pausado` (`motivo_pausa=app_fechado`, ou `motor_reiniciado`
quando o motor foi iniciado com `--religado` pelo app após queda — contracts/runtime.md).
Fila por conta: no máximo um disparo da conta em `enviando`/`fora_da_janela`; "Na fila" = estado
`agendado` com `inicio_em ≤ agora` enquanto outro da conta está ativo (derivado, não persistido).

### Destinatário (`destinatarios`)

| Campo | Tipo | Obrigatório | Validação |
|-------|------|-------------|-----------|
| id | texto | sim | — |
| disparo_id | texto | sim | FK disparos cascade |
| lead_id | texto | sim | FK leads; único por `(disparo_id, lead_id)` |
| ordem | inteiro | sim | ordem de envio |
| telefone | texto | sim | E.164 (cópia no momento da criação) |
| jid | texto | não | JID resolvido na verificação |
| variaveis | JSON | sim | valores resolvidos para cada variável usada |
| estado | enum | sim | `pendente`, `enviando`, `enviado`, `entregue`, `lido`, `falhou`, `respondeu` |
| motivo_falha | texto | não | ex.: `Sem WhatsApp`, `Estado incerto após interrupção`, erro do servidor |
| mensagem_wa_id | texto | não | id da mensagem enviada |
| enviando_em / enviado_em / entregue_em / lido_em / respondeu_em / falhou_em | data | não | — |

Índices: `(disparo_id, estado)`, `(disparo_id, ordem)`, `(telefone, estado)`.

Máquina de estados (testes obrigatórios):

```text
pendente ──▶ enviando ──▶ enviado ──▶ entregue ──▶ lido
pendente | enviando ──▶ falhou
enviado | entregue | lido ──(mensagem do número na mesma conta após enviado_em)──▶ respondeu
```

Recibos fora de ordem não regridem (`lido` antes de `entregue` preenche ambos). `respondeu` é
final para fins de relatório (recibos posteriores só preenchem datas).
Idempotência (Constituição V): gravar `enviando` (commit) → enviar → gravar `enviado` +
`mensagem_wa_id` (commit). Na retomada, `enviando` sem `mensagem_wa_id` é conferido no histórico
local (mensagem `de_mim` para o JID com `disparo_id` igual); se achada → `enviado`; senão →
`falhou` com `Estado incerto após interrupção`. Nunca reenviar.

Resolução de variáveis (testes obrigatórios): `{nome}` → `lead.nome`; `{x}` → `lead.campos[x]`
(chave normalizada); valor ausente ou vazio → `valores_padrao[x]`; se ainda ausente → destinatário
incompleto, o que bloqueia `iniciar` (erro `variaveis_faltando` com a lista). `{{` e `}}` escapam
chaves literais. Variável desconhecida em todos os leads também bloqueia.

### Configuração (`configuracoes`)

`chave` texto PK, `valor` JSON. Usos: conta selecionada por último, preferências de interface.

## Relações

- Conta 1:N Conversa, Conta 1:N Contato, Conta 1:N Disparo, Conta 1:N Status.
- Conversa 1:N Mensagem; Mensagem 1:N Reação.
- Contato N:N Etiqueta; Contato N:1 Lead (opcional).
- Disparo 1:N Destinatário; Lead 1:N Destinatário.
- Template N:1 Arquivo; Disparo N:1 Arquivo.

## Cálculo do agendamento (testes obrigatórios)

Função pura `ProximoEnvio(estado, agora, config) → (instante, motivo)` com relógio injetado:

1. `base = ultimo_envio + aleatorio_uniforme(intervalo_min_s, intervalo_max_s)` (primeiro envio:
   `max(agora, inicio_em)`).
2. Se `pausa_a_cada` e `enviados_desde_ultima_pausa == pausa_a_cada` → `base = max(base,
   ultimo_envio + pausa_duracao_s)`.
3. Limite por hora: se os envios nos últimos 60 min ≥ limite → `base = max(base, envio_mais_antigo
   _da_janela_movel + 60 min)`.
4. Limite por dia: se envios no dia civil local ≥ limite → `base = max(base, próxima meia-noite
   local)`.
5. Janela: se `base` fora da janela → próximo início de janela (considerando janela que cruza a
   meia-noite).
6. Se `agora > base` (ex.: Mac acordou do sono), `base = agora + aleatorio(intervalo)` — nunca
   rajada de envios atrasados.

Estimativa de término: simulação determinística da mesma função com o intervalo médio para os
pendentes restantes.
