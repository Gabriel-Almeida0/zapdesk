# Contrato de eventos WebSocket do motor — v1

Conexão: `ws://127.0.0.1:<porta>/v1/eventos?token=<token>`. Somente servidor → cliente (mensagens
do cliente são ignoradas, exceto `ping`). O servidor envia ping de controle a cada 20 s; clientes
reconectam com backoff (0,5 s → 5 s) e, ao reconectar, recarregam o estado via HTTP (não há replay).

## Envelope

```json
{ "seq": 1042, "tipo": "mensagem.nova", "conta_id": "01J...", "em": "2026-09-27T20:11:00-03:00",
  "dados": { } }
```

- `seq`: inteiro crescente por conexão do motor (reinicia quando o motor reinicia). Lacuna em
  `seq` → cliente deve recarregar via HTTP.
- `conta_id`: presente quando o evento é de uma conta; `null` para eventos globais.
- Tipos usam os objetos de `api-http.md` › Tipos.

## Eventos

| `tipo` | `dados` | Quando |
|--------|---------|--------|
| `motor.pronto` | `{versao}` | primeiro evento de toda conexão |
| `conta.atualizada` | `Conta` | mudança de estado, nome, `online`, `sincronizando` |
| `conta.qr` | `{codigo, expira_em}` | novo QR disponível (cada rotação) |
| `conta.qr_expirado` | `{}` | QR expirou sem leitura (conta volta a `desconectada`) |
| `conta.removida` | `{id}` | conta removida |
| `sincronizacao.progresso` | `{conversas, mensagens, concluida: bool}` | durante history sync |
| `conversa.atualizada` | `Conversa` | nova conversa, não lidas, última mensagem, etiquetas |
| `mensagem.nova` | `Mensagem` | recebida ou enviada (inclusive por disparo e pelo MCP) |
| `mensagem.atualizada` | `Mensagem` | estado (recibos), edição, apagada, reação, mídia baixada, falha |
| `contato.atualizado` | `Contato` | nome, notas, etiquetas, lead ligado |
| `status.novo` | `Status` | status publicado por contato |
| `etiquetas.alteradas` | `{}` | criar/editar/excluir etiqueta (recarregar lista) |
| `templates.alterados` | `{}` | criar/editar/excluir template |
| `leads.importados` | `{origem, total_novos, total_ja_existentes, total_invalidos}` | após qualquer importação (app ou MCP) |
| `disparo.atualizado` | `Disparo` | criação e toda mudança de estado ou contadores (no máximo 1 por segundo por disparo durante envio; sempre imediato em mudança de estado) |
| `destinatario.atualizado` | `Destinatario` | cada transição de destinatário |
| `disparo.finalizado` | `{disparo: Disparo, resumo: "Disparo concluído: 480 enviados, 12 falharam"}` | ao entrar em `concluido` ou `cancelado` |
| `disparos.ativos` | `{total}` | sempre que muda o número de disparos em `agendado`/`enviando`/`fora_da_janela` (usado pelo app para decidir se pode encerrar) |
| `conexao.rede` | `{conta_id, online: bool}` | queda/retorno da rede ("Sem conexão. Tentando reconectar…") |

Contagem de "ativos" para a regra da barra de menu: disparos em `agendado` (com `iniciado_em`),
`enviando` ou `fora_da_janela`. Disparos `pausado` NÃO mantêm o app vivo.
