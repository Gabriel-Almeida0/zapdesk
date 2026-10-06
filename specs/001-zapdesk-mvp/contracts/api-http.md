# Contrato da API HTTP do motor — v1

Base: `http://127.0.0.1:<porta>/v1`. Fonte única para `app/`, `mcp/` e
`compartilhado/cliente-motor/`. Eventos em tempo real: `eventos-ws.md`.

## Convenções

- **Autenticação**: `Authorization: Bearer <token>` em todas as rotas. Rotas `GET` de binário
  (`/midia`, `/arquivos/{id}/conteudo`) e o WebSocket também aceitam `?token=<token>`.
  Token ausente/errado → `401 nao_autorizado`. `Host` diferente de `127.0.0.1:<porta>` (ou
  `localhost:<porta>`, aceito para ferramentas como `curl localhost:…`) → `403 host_invalido`.
- **Formato**: JSON UTF-8, campos em `snake_case` português. Datas RFC 3339 com fuso local.
  Telefones em E.164. IDs são ULIDs (texto).
- **Paginação**: `?limite=<1..200, padrão 50>&cursor=<opaco>`; resposta
  `{"itens":[...],"proximo_cursor":"..."|null}`.
- **Erros**: status HTTP + corpo
  `{"erro":{"codigo":"<codigo>","mensagem":"<pt-BR para exibir>","detalhes":{...}}}`.

| Código | HTTP | Uso |
|--------|------|-----|
| `nao_autorizado` | 401 | token |
| `host_invalido` | 403 | Host header |
| `nao_encontrado` | 404 | recurso inexistente |
| `validacao` | 422 | campos inválidos; `detalhes.campos: {campo: mensagem}` |
| `conflito` | 409 | nome duplicado (etiqueta, template) |
| `transicao_invalida` | 409 | ação não permitida no estado atual; `detalhes.estado_atual` |
| `variaveis_faltando` | 422 | `detalhes.faltando: [{destinatario_linha, telefone, variaveis:[...]}]`, `detalhes.total` |
| `conta_indisponivel` | 409 | conta não `conectada` (ou `banida`) |
| `sem_whatsapp` | 422 | número sem WhatsApp |
| `fora_do_prazo` | 409 | editar/apagar após o prazo |
| `anexo_grande_demais` | 413 | `detalhes.limite_bytes`, `detalhes.tipo_midia` |
| `tipo_nao_suportado` | 415 | arquivo/planilha não suportado |
| `whatsapp_erro` | 502 | falha no WhatsApp; `detalhes.motivo` |
| `interno` | 500 | inesperado |

## Tipos (JSON)

```text
Conta        { id, nome, telefone|null, jid|null, estado: "conectando"|"conectada"|"desconectada"|"banida",
               online: bool, sincronizando: bool, criada_em }
Conversa     { id, conta_id, jid, tipo: "individual"|"grupo", nome, telefone|null, contato_id|null,
               nao_lidas, ultima_mensagem_em|null, ultima_mensagem_resumo|null, etiquetas: Etiqueta[] }
Mensagem     { id, conta_id, conversa_id, wa_id, remetente_jid, remetente_nome|null, de_mim,
               tipo: "texto"|"imagem"|"video"|"audio"|"documento"|"figurinha"|"sistema",
               texto|null, midia: Midia|null, citacao: {wa_id, resumo, remetente_nome}|null,
               reacoes: [{remetente_jid, emoji, de_mim}], editada, apagada,
               estado: "pendente"|"enviada"|"entregue"|"lida"|"falhou"|"recebida", erro|null,
               disparo_id|null, enviada_em, pode_editar: bool, pode_apagar: bool }
Midia        { mimetype, tamanho, nome_arquivo|null, duracao_s|null, ptt: bool, largura|null, altura|null,
               miniatura_b64|null, baixada: bool, url: "/v1/mensagens/{id}/midia" }
Contato      { id, conta_id, jid, telefone|null, nome|null, nome_push|null, notas|null,
               etiquetas: Etiqueta[], lead: {id, origem, importado_em}|null, conversa_id|null }
Etiqueta     { id, nome, cor, total_contatos }
Status       { id, conta_id, contato_jid, contato_nome|null, tipo: "texto"|"imagem"|"video", texto|null,
               midia: Midia|null, publicado_em }
Lead         { id, telefone, nome|null, campos: {chave: valor}, origem: "csv"|"colado"|"contatos"|"mcp",
               tem_whatsapp: bool|null, importado_em, ultimo_disparo_em|null }
Arquivo      { id, nome, mimetype, tamanho, tipo_midia: "imagem"|"video"|"audio"|"documento"|"figurinha",
               url: "/v1/arquivos/{id}/conteudo" }
Template     { id, nome, texto, variaveis: string[], arquivo: Arquivo|null, criado_em, atualizado_em }
Ritmo        { intervalo_min_s, intervalo_max_s, limite_por_hora|null, limite_por_dia|null,
               pausa_a_cada|null, pausa_duracao_s|null }
Janela       { inicio: "HH:MM", fim: "HH:MM" } | null
Disparo      { id, conta_id, nome, mensagem, arquivo: Arquivo|null, ritmo: Ritmo, inicio_em|null,
               janela: Janela, falhas_seguidas_max, valores_padrao: {var: valor},
               estado: "rascunho"|"agendado"|"enviando"|"fora_da_janela"|"pausado"|"concluido"|"cancelado",
               na_fila: bool, motivo_pausa|null, origem: "app"|"mcp",
               contadores: {total, pendente, enviando, enviado, entregue, lido, respondeu, falhou},
               proximo_envio_em|null, estimativa_termino_em|null, aviso_ritmo_agressivo: bool,
               criado_em, iniciado_em|null, concluido_em|null, cancelado_em|null }
Destinatario { id, disparo_id, lead_id, ordem, telefone, nome|null, variaveis: {var: valor},
               estado: "pendente"|"enviando"|"enviado"|"entregue"|"lido"|"falhou"|"respondeu",
               motivo_falha|null, mensagem_wa_id|null, enviando_em, enviado_em, entregue_em, lido_em,
               respondeu_em, falhou_em (todos |null) }
RelatorioImportacao {
  total_linhas, total_novos, total_ja_existentes, total_invalidos, total_duplicados_no_lote,
  novos: [{linha, lead_id, telefone}],
  ja_existentes: [{linha, lead_id, telefone, importado_em, campos_preenchidos: string[]}],
  invalidos: [{linha, valor, motivo: "vazio"|"formato_invalido"|"numero_invalido"}],
  duplicados_no_lote: [{linha, telefone, primeira_linha}],
  lead_ids: string[]      // todos os leads válidos do lote (novos + já existentes), na ordem
}
```

`aviso_ritmo_agressivo` = verdadeiro se `intervalo_min_s < 10` ou `limite_por_hora > 120` ou
`limite_por_dia > 1000` ou nenhum limite definido (apenas aviso; não bloqueia).

## Sistema

| Método | Caminho | Entrada | Saída |
|--------|---------|---------|-------|
| GET | `/saude` | — | `200 {"ok":true,"versao":"0.1.0","whatsapp":"real\|falso"}` |
| GET | `/sistema` | — | `{versao, pasta_dados, caminho_logs, whatsapp, disparos_ativos, contas_conectadas}` |
| POST | `/sistema/energia` | `{"evento":"suspender"\|"retomar"}` | `204`; recalcula agendas |
| POST | `/sistema/encerrar` | — | `202`; encerramento gracioso |

## Contas

| Método | Caminho | Entrada | Saída / erros |
|--------|---------|---------|---------------|
| GET | `/contas` | — | `Conta[]` |
| POST | `/contas` | `{"nome"?: string}` | `201 Conta` em `conectando`; QR chega por `conta.qr` |
| GET | `/contas/{id}` | — | `Conta` |
| GET | `/contas/{id}/qr` | — | `{"codigo":"2@...","expira_em"}` ou `404` se não há QR ativo |
| POST | `/contas/{id}/reconectar` | — | `202 Conta` (`conectando`; novo QR se a sessão foi perdida) |
| PATCH | `/contas/{id}` | `{"nome": "1–60"}` | `Conta` / `validacao` |
| DELETE | `/contas/{id}` | — | `204`; logout, apaga sessão, conversas, mensagens, contatos e mídias da conta; disparos da conta são cancelados. |

## Conversas e mensagens

| Método | Caminho | Entrada | Saída / erros |
|--------|---------|---------|---------------|
| GET | `/contas/{id}/conversas` | `?busca=&etiqueta_id=&nao_lidas=true&limite&cursor` | página de `Conversa` (ordem: `ultima_mensagem_em` desc) |
| POST | `/contas/{id}/conversas` | `{"telefone":"(11) 99999-0000"}` | `200/201 Conversa` / `validacao` (telefone), `sem_whatsapp`, `conta_indisponivel` |
| GET | `/conversas/{id}` | — | `Conversa` |
| POST | `/conversas/{id}/lida` | — | `204`; zera `nao_lidas`, envia recibo de leitura |
| GET | `/conversas/{id}/mensagens` | `?antes=<mensagem_id>&limite` | página de `Mensagem` (mais recentes primeiro) |
| GET | `/contas/{id}/mensagens/busca` | `?q=<termo>&limite&cursor` | página de `{mensagem: Mensagem, conversa: {id, nome}, trecho}` |
| POST | `/conversas/{id}/mensagens` | ver abaixo | `202 Mensagem` (`pendente`) / `validacao`, `conta_indisponivel`, `anexo_grande_demais` |
| POST | `/mensagens/{id}/reenviar` | — | `202 Mensagem` / `transicao_invalida` (se não `falhou`) |
| POST | `/mensagens/{id}/reacao` | `{"emoji":"👍"}` (`""` remove) | `204` |
| PATCH | `/mensagens/{id}` | `{"texto":"..."}` | `Mensagem` (`editada=true`) / `fora_do_prazo`, `transicao_invalida` (não é minha / não é texto) |
| DELETE | `/mensagens/{id}` | — | `204` (apagar para todos) / `fora_do_prazo` |
| GET | `/mensagens/{id}/midia` | `?token=` | binário (baixa sob demanda e cacheia) / `502 whatsapp_erro` "Não foi possível baixar." |
| GET | `/contas/{id}/figurinhas` | `?limite` | `[{mensagem_id|null, arquivo_id|null, url}]` — figurinhas recentes (recebidas/enviadas) |

Envio (`POST /conversas/{id}/mensagens`):

```json
{
  "texto": "Oi!",                 // obrigatório se não houver arquivo_id; vira legenda com imagem/vídeo/documento
  "arquivo_id": null,             // de POST /arquivos
  "como": "auto",                 // "auto" | "voz" (áudio PTT) | "figurinha" | "documento"
  "citar_mensagem_id": null
}
```

Enviar para um telefone sem conversa: `POST /contas/{id}/conversas` e depois o envio.

## Contatos

| Método | Caminho | Entrada | Saída |
|--------|---------|---------|-------|
| GET | `/contas/{id}/contatos` | `?busca=&etiqueta_id=&limite&cursor` | página de `Contato` |
| GET | `/contatos/{id}` | — | `Contato` |
| PATCH | `/contatos/{id}` | `{"notas": "até 10.000"}` | `Contato` |
| PUT | `/contatos/{id}/etiquetas` | `{"etiqueta_ids": [...]}` | `Contato` (substitui o conjunto) |

## Status

| Método | Caminho | Entrada | Saída |
|--------|---------|---------|-------|
| GET | `/contas/{id}/status` | — | `[{contato_jid, contato_nome, itens: Status[]}]` (últimas 24 h, mais recentes primeiro) |
| GET | `/status/{id}/midia` | `?token=` | binário |

## Etiquetas

| Método | Caminho | Entrada | Saída / erros |
|--------|---------|---------|---------------|
| GET | `/etiquetas` | — | `Etiqueta[]` |
| POST | `/etiquetas` | `{"nome":"1–30, único","cor":"#RRGGBB"}` | `201 Etiqueta` / `validacao`, `conflito` |
| PATCH | `/etiquetas/{id}` | `{"nome"?,"cor"?}` | `Etiqueta` / `validacao`, `conflito` |
| DELETE | `/etiquetas/{id}` | — | `204` (remove dos contatos) |

## Arquivos (anexos)

| Método | Caminho | Entrada | Saída / erros |
|--------|---------|---------|---------------|
| POST | `/arquivos` | `multipart/form-data` campo `arquivo` **ou** JSON `{"caminho":"/abs/arquivo.pdf"}` (cópia de arquivo local) | `201 Arquivo` / `anexo_grande_demais`, `tipo_nao_suportado`, `validacao` (caminho inexistente) |
| GET | `/arquivos/{id}` | — | `Arquivo` |
| GET | `/arquivos/{id}/conteudo` | `?token=` | binário |

Limites: imagem, vídeo e áudio 16 MB; documento 100 MB; figurinha 1 MB (`image/webp`). Áudio
`audio/webm` é remuxado para `audio/ogg; codecs=opus` quando enviado `como: "voz"`.

## Templates

| Método | Caminho | Entrada | Saída / erros |
|--------|---------|---------|---------------|
| GET | `/templates` | `?busca=` | `Template[]` (busca por prefixo/trecho do nome, para o atalho `/`) |
| POST | `/templates` | `{"nome":"1–60, único","texto":"1–4096","arquivo_id"?}` | `201 Template` / `validacao`, `conflito` |
| GET | `/templates/{id}` | — | `Template` |
| PATCH | `/templates/{id}` | `{"nome"?,"texto"?,"arquivo_id"?: string\|null}` | `Template` |
| DELETE | `/templates/{id}` | — | `204` |

## Leads e importação

| Método | Caminho | Entrada | Saída / erros |
|--------|---------|---------|---------------|
| POST | `/importacoes/previa` | `multipart` campo `arquivo` (`.csv`, `.xlsx`) **ou** `{"caminho":"/abs/leads.csv"}` | `201 {importacao_id, nome_arquivo, colunas: string[], amostra: string[][] (até 5 linhas), total_linhas, coluna_telefone_sugerida|null, coluna_nome_sugerida|null, expira_em}` / `tipo_nao_suportado` |
| POST | `/leads/importar` | ver abaixo | `200 RelatorioImportacao` / `validacao` (ex.: `"Escolha qual coluna tem o telefone."`) |
| POST | `/leads/importar-contatos` | `{"conta_id","contato_ids"?: [], "etiqueta_ids"?: []}` | `200 RelatorioImportacao` (origem `contatos`) |
| GET | `/leads` | `?busca=&origem=&limite&cursor` | página de `Lead` (ordem: `importado_em` desc) |
| GET | `/leads/{id}` | — | `Lead` |

Corpo de `POST /leads/importar` — exatamente uma das formas:

```json
// A) de planilha (origem "csv")
{ "importacao_id": "01J...", "mapeamento": { "telefone": "Celular", "nome": "Nome",
  "extras": ["Empresa", "Cidade"] } }          // extras ausente = todas as outras colunas

// B) números colados (origem "colado"): um por linha
{ "texto_colado": "11 99999-0000\n+55 21 98888-7777" }

// C) lista estruturada (usada pelo MCP)
{ "leads": [ { "telefone": "+5511999990000", "nome": "Ana", "campos": { "empresa": "X" } } ],
  "origem": "mcp" }
```

Parâmetros comuns opcionais: `"ddi_padrao": "55"` e `"origem"` (padrão: A=`csv`, B=`colado`,
C=`mcp`; o MCP sempre envia `"origem": "mcp"`, inclusive na forma A). Regras: `data-model.md` › Lead.
`linha` no relatório é 1-based e conta a partir da primeira linha de dados (cabeçalho excluído).

## Disparos

| Método | Caminho | Entrada | Saída / erros |
|--------|---------|---------|---------------|
| POST | `/disparos/validar` | `NovoDisparo` (sem gravar) | `200 {total_destinatarios, variaveis: string[], faltando: [...] , previa: {telefone, nome, texto_resolvido}, estimativa_termino_em, aviso_ritmo_agressivo, erros_campos: {}}` |
| POST | `/disparos` | `NovoDisparo` | `201 Disparo` / `validacao`, `variaveis_faltando` (só se `iniciar=true`), `nao_encontrado` (conta/arquivo/template) |
| GET | `/disparos` | `?conta_id=&estado=&limite&cursor` | página de `Disparo` (ordem: `criado_em` desc) |
| GET | `/disparos/{id}` | — | `Disparo` |
| PATCH | `/disparos/{id}` | campos de `NovoDisparo` (sem destinatários) | `Disparo` / `transicao_invalida` (só em `rascunho`) |
| DELETE | `/disparos/{id}` | — | `204` / `transicao_invalida` (só em `rascunho`) |
| POST | `/disparos/{id}/iniciar` | `{"valores_padrao"?: {}}` | `Disparo` (`agendado`) / `variaveis_faltando`, `conta_indisponivel`, `transicao_invalida` |
| POST | `/disparos/{id}/pausar` | — | `Disparo` (`pausado`, `motivo_pausa=usuario`) / `transicao_invalida` |
| POST | `/disparos/{id}/retomar` | — | `Disparo` (`agendado`, entra na fila) / `transicao_invalida`, `conta_indisponivel` |
| POST | `/disparos/{id}/cancelar` | — | `Disparo` (`cancelado`) / `transicao_invalida` |
| GET | `/disparos/{id}/destinatarios` | `?estado=&busca=&limite&cursor` | página de `Destinatario` (ordem: `ordem`) |
| GET | `/disparos/{id}/relatorio.csv` | `?token=` | `text/csv; charset=utf-8` com BOM; colunas: `telefone,nome,estado,motivo_falha,enviando_em,enviado_em,entregue_em,lido_em,respondeu_em,falhou_em` + uma coluna por variável, com o nome entre chaves (ex.: `{empresa}`) |

`NovoDisparo`:

```json
{
  "conta_id": "01J...",
  "nome": "Prospecção setembro",            // opcional; padrão "Disparo dd/mm hh:mm"
  "mensagem": "Oi {nome}, tudo bem?",       // ou "template_id" (copia texto e anexo)
  "template_id": null,
  "arquivo_id": null,
  "destinatarios": {                        // união deduplicada por lead; pelo menos uma fonte
    "lead_ids": [],
    "etiqueta_ids": [],                     // contatos com essas etiquetas → leads (importados com origem "contatos")
    "contato_ids": [],
    "importar": null                        // mesmo corpo de POST /leads/importar (A, B ou C); importa e usa os leads válidos
  },
  "ritmo": { "intervalo_min_s": 30, "intervalo_max_s": 90, "limite_por_hora": 40,
             "limite_por_dia": 300, "pausa_a_cada": 50, "pausa_duracao_s": 600 },
  "inicio_em": null,                        // null = agora
  "janela": { "inicio": "09:00", "fim": "18:00" },
  "falhas_seguidas_max": 10,
  "valores_padrao": { "nome": "tudo bem" },
  "iniciar": false,                         // true = cria e já inicia (MCP sempre true)
  "origem": "app"                           // "app" | "mcp"
}
```

Quando `destinatarios.importar` é usado, a resposta de `POST /disparos` inclui
`"relatorio_importacao": RelatorioImportacao`.

Validações (`validacao`, com `detalhes.campos`): `intervalo_min_s ≥ 1`; `intervalo_max_s ≥
intervalo_min_s`; limites e pausa `> 0`; `pausa_a_cada` e `pausa_duracao_s` juntos; janela com
ambos os horários, formato `HH:MM`, diferentes; mensagem não vazia (até 4.096); pelo menos 1
destinatário válido; conta existente.

## Notas de implementação do motor (v0.1.0)

Acréscimos compatíveis e esclarecimentos descobertos na implementação:

- `validacao`: as chaves de `detalhes.campos` usam o caminho do campo com ponto
  (ex.: `ritmo.intervalo_min_s`, `mapeamento.telefone`, `destinatarios`).
- `GET /contas/{id}/qr` → `codigo` é opaco: com o WhatsApp real (whatsmeow atual) vem no formato
  `https://wa.me/settings/linked_devices#2@…`; no modo falso, `2@falso-…`. Renderize a string
  inteira no QR.
- `GET /conversas/{id}/mensagens`: `proximo_cursor` é o id da mensagem mais antiga da página e
  pode ser passado como `antes` (ou como `cursor`).
- `POST /disparos` com `iniciar: true` também pode responder `409 conta_indisponivel` (conta
  desconectada/banida), como `/iniciar`. Com `variaveis_faltando` nada é gravado.
  `detalhes.faltando` traz no máximo 100 itens; `detalhes.total` traz o total.
- `DELETE /contas/{id}`: os disparos da conta são cancelados (eventos `disparo.finalizado`) e,
  em seguida, apagados junto com a conta.
- `POST /conversas/{id}/mensagens`: legenda é ignorada em áudio e figurinha (o WhatsApp não
  aceita); `como: "figurinha"` exige `image/webp` (`415 tipo_nao_suportado`); `como: "voz"` com
  `audio/webm` é remuxado para OGG/Opus — se falhar, vai como áudio comum.
- `GET /contas/{id}/figurinhas` aceita `?limite` (padrão 30).
- Motivos de falha de destinatário usados pelo motor: `Sem WhatsApp`, `Estado incerto após
  interrupção` e a mensagem do servidor nos demais casos.
