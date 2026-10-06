# Contrato de runtime — processo do motor, `runtime.json` e modo falso

Versão do contrato: `1`. Consumidores: `app/` (processo principal) e `mcp/`.

## Executável

`zapdesk-motor` (no app empacotado: `ZapDesk.app/Contents/Resources/bin/zapdesk-motor`).

### Flags

| Flag | Variável de ambiente | Padrão | Descrição |
|------|----------------------|--------|-----------|
| `--pasta-dados <dir>` | `ZAPDESK_PASTA_DADOS` | `~/Library/Application Support/ZapDesk` | pasta de dados |
| `--porta <n>` | `ZAPDESK_PORTA` | `0` (aleatória) | porta TCP em `127.0.0.1` |
| `--whatsapp <real\|falso>` | `ZAPDESK_WHATSAPP` | `real` | escolhe a implementação de `ClienteWhatsApp` |
| `--nivel-log <debug\|info\|aviso\|erro>` | `ZAPDESK_NIVEL_LOG` | `info` | nível de log |
| `--log-conteudo` | `ZAPDESK_LOG_CONTEUDO=1` | desligado | inclui conteúdo de mensagens no log (depuração) |
| `--sem-stdin` | — | desligado | não encerrar ao fechar stdin (útil em desenvolvimento/testes) |
| `--religado` | — | desligado | o app religou o motor após queda: disparos ativos voltam com `motivo_pausa=motor_reiniciado` (sem a flag: `app_fechado`) |
| `--versao` | — | — | imprime a versão e sai |

Token: **somente** pela variável de ambiente `ZAPDESK_TOKEN` (obrigatória; o motor sai com código
2 se ausente ou com menos de 32 caracteres). Nunca por argumento.

### Protocolo de inicialização (stdout)

O motor escreve no stdout, uma por linha, mensagens JSON de controle. Logs vão para o arquivo e
para o stderr (nunca para o stdout).

```json
{"evento":"pronto","porta":51234,"versao":"0.1.0","pid":12345,"whatsapp":"real"}
{"evento":"erro_fatal","codigo":"porta_ocupada","mensagem":"..."}
{"evento":"encerrando","motivo":"sinal|stdin_fechado|pedido"}
```

- `pronto` só é emitido depois de: banco aberto e migrado, API escutando, disparos ativos
  convertidos para `pausado`. A conexão das contas com o WhatsApp continua em segundo plano
  (não bloqueia o `pronto`). Meta: < 2 s.
- Códigos de `erro_fatal`: `token_ausente`, `pasta_dados_inacessivel`, `instancia_duplicada`
  (outro motor segura `motor.lock`), `migracao_falhou`, `porta_ocupada`.

### Encerramento

- `SIGTERM`/`SIGINT` ou stdin fechado (EOF) → encerramento gracioso: para agendadores (disparos
  ficam `pausado` com `motivo_pausa=app_fechado`), desconecta clientes WhatsApp, fecha bancos,
  sai com código 0 em até 5 s.
- `POST /v1/sistema/encerrar` tem o mesmo efeito.
- Códigos de saída: `0` normal, `1` erro inesperado, `2` configuração inválida, `3` instância
  duplicada.

## `runtime.json`

Escrito **pelo app** em `<pasta-dados>/runtime.json` com permissão `0600` após receber `pronto`;
removido pelo app ao encerrar.

```json
{
  "versao_contrato": 1,
  "porta": 51234,
  "token": "base64url-de-32-bytes",
  "pid_app": 12000,
  "pid_motor": 12345,
  "versao": "0.1.0",
  "iniciado_em": "2026-09-27T20:10:00-03:00"
}
```

Regras para clientes (MCP):

1. Arquivo ausente, `pid_motor` morto (`kill -0`) ou `GET /v1/saude` sem `200` → app considerado
   fechado.
2. App fechado → `open -b com.gabriel.zapdesk` (fallback `open -a ZapDesk`); sondar a cada 250 ms
   por até 30 s até o arquivo existir e `/v1/saude` responder.
3. Nunca escrever nesse arquivo.

## Modo falso (`--whatsapp=falso`)

Implementação em memória, determinística, de `ClienteWhatsApp` (ver `cliente-whatsapp.md`).
Habilita os endpoints de controle abaixo (retornam `404` no modo real). Todos exigem token.

| Método e caminho | Corpo | Efeito |
|------------------|-------|--------|
| `POST /v1/falso/contas/{id}/escanear-qr` | `{"telefone":"+5511900000001","nome":"Teste"}` | simula QR escaneado → conta `conectada` |
| `POST /v1/falso/contas/{id}/mensagem-recebida` | `{"de":"+5511..","texto":"oi","grupo_jid":null,"tipo":"texto"}` | injeta mensagem recebida |
| `POST /v1/falso/contas/{id}/recibo` | `{"wa_id":"..","tipo":"entregue\|lido"}` | injeta recibo |
| `POST /v1/falso/contas/{id}/estado` | `{"evento":"queda_rede\|reconectou\|logout\|ban"}` | simula eventos de conexão |
| `POST /v1/falso/contas/{id}/historico` | `{"conversas":[{"jid":"..","mensagens":[...]}]}` | simula history sync |
| `POST /v1/falso/contas/{id}/status` | `{"de":"+55..","texto":"..."}` | simula status publicado |
| `PUT /v1/falso/numeros-sem-whatsapp` | `{"telefones":["+55.."]}` | números que respondem `IsIn=false` |
| `PUT /v1/falso/falhas-envio` | `{"telefones":["+55.."],"erro":"..."}` | envios a esses números falham |
| `GET /v1/falso/enviadas` | — | lista tudo que foi "enviado" (para asserts de idempotência) |
| `PUT /v1/falso/relogio` | `{"agora":"2026-09-28T09:00:00-03:00"}` ou `{"avancar_s":3600}` | controla o relógio injetado do motor |

O relógio controlável só existe no modo falso; no modo real o motor usa o relógio do sistema.
No motor falso iniciado por linha de comando o relógio **anda em tempo real** a partir do instante
ajustado (os disparos progridem sozinhos); `PUT /v1/falso/relogio` só desloca esse instante.

Detalhes do modo falso (acréscimos compatíveis, motor v0.1.0):

- As rotas de injeção (`escanear-qr`, `mensagem-recebida`, `recibo`, `estado`, `historico`,
  `status`, `expirar-qr`) só respondem **depois** que o motor processou o evento: ao receber a
  resposta, o estado já está visível na API (os eventos WS já foram publicados).
- `escanear-qr` exige que a conta esteja aguardando QR (senão `409 transicao_invalida`); conta
  inexistente → `404`. Resposta: `{"ok":true}`.
- `POST /v1/falso/contas/{id}/expirar-qr` → simula QR expirado sem leitura (`conta.qr_expirado`).
- `mensagem-recebida` aceita também `nome` (push name), `grupo_nome`, `de_mim` (mensagem enviada
  pelo próprio celular), `falhar_download` (a mídia não poderá ser baixada), `citar_wa_id` e `em`
  (RFC 3339). `tipo`: `texto` (padrão) | `imagem` | `video` | `audio` | `documento` |
  `figurinha`. Resposta: `{"wa_id":"FALSO..."}`.
- `status` → `{"wa_id":"..."}`.
- `historico`: cada conversa é `{"jid":"+5511..." | "5511...@s.whatsapp.net" | "123@g.us",
  "nome"?: "...", "mensagens":[{"texto","de"?,"nome"?,"de_mim"?,"tipo"?,"wa_id"?,"em"?}]}`; é
  entregue em lotes de 500 mensagens (um `sincronizacao.progresso` por lote).
- `GET /v1/falso/enviadas` → `[{conta_id, wa_id, tipo, para, telefone, texto, mimetype, tamanho,
  nome_arquivo, voz, citacao_wa_id, alvo_wa_id, emoji, em}]` (campos vazios omitidos; `tipo`:
  `texto|imagem|video|audio|documento|figurinha|reacao|edicao|apagar`). Persistido em
  `<pasta-dados>/falso/` e preservado entre reinícios do motor, assim como o pareamento.
- `PUT /v1/falso/relogio` → `{"agora":"<RFC 3339>"}`.
