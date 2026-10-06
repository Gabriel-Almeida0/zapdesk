# Contrato de eventos WebSocket — acréscimos da feature 002 (v1, compatível)

Conexão, envelope (`seq`, `tipo`, `conta_id`, `em`, `dados`), reconexão e recarga por lacuna:
**iguais ao MVP** (`specs/001-zapdesk-mvp/contracts/eventos-ws.md`). Clientes DEVEM ignorar
tipos desconhecidos. Tipos de `dados` em `api-http.md` (deste diretório).

| `tipo` | `dados` | `conta_id` | Quando |
|--------|---------|------------|--------|
| `funil.alterado` | `{funil_id: string\|null}` (`null` = lista mudou) | `null` | criar/editar/excluir/reordenar funil ou etapa |
| `funil.movido` | `{movimento: MovimentoFunil, card: Card\|null}` (`card=null` = saiu do funil) | `null` | lead entrou, mudou de etapa ou saiu de um funil (app, MCP ou automação) |
| `automacao.atualizada` | `Automacao` | `null` | criar, editar, ativar, desativar (inclusive por 5 erros), compilar |
| `automacao.removida` | `{id}` | `null` | exclusão |
| `automacao.arquivos_alterados` | `{automacao_id, caminhos: string[], origem: "api"}` | `null` | escrita/renomeio/exclusão de arquivo do projeto pela API (app ou MCP) |
| `automacao.execucao.iniciada` | `Execucao` | conta do alvo | execução entrou em `rodando` (inclui testes) |
| `automacao.execucao.atualizada` | `Execucao` | idem | nova ação registrada, `aguardando`/retomada (no máx. 1 por segundo por execução, exceto mudança de estado) |
| `automacao.execucao.finalizada` | `Execucao` | idem | `ok`, `erro`, `simulacao` ou `abortada` |
| `chatbot.sessao.iniciada` | `SessaoChatbot` | conta da conversa | sessão criada |
| `chatbot.sessao.atualizada` | `SessaoChatbot` | idem | mudou de nó, variável ou tentativas |
| `chatbot.sessao.finalizada` | `SessaoChatbot` | idem | `concluida`, `humano`, `expirada`, `abortada` |
| `conversa.pausa` | `{conversa_id, pausa: Pausa\|null}` | conta da conversa | pausa criada, renovada, removida ou vencida (vencimento publicado pelo agendador) |
| `notificacao` | `{titulo, corpo, automacao_id\|null, conversa_id\|null, tipo: "anti_loop"\|"humano"\|"desativada"\|"acao"\|"erro"}` | conta, se houver | ação "notificar", nó "humano", anti-loop, desativação por erros, `ctx.notificar` |
| `segredos.alterados` | `{nomes: string[]}` | `null` | o motor recebeu novo conjunto de segredos pelo stdin |
| `automacoes.configuracao` | `ConfiguracaoAutomacoes` | `null` | Ajustes → Automações alterado (inclui "Pausar todas") |

Regras:

- `mensagem.nova`/`mensagem.atualizada` do MVP trazem `automacao_id` nas mensagens automáticas.
- Execuções em simulação publicam os mesmos eventos (`simulacao: true`), para o painel de teste
  acompanhar o log ao vivo; clientes de lista podem filtrar por `simulacao`.
- O processo principal do app transforma `notificacao` em notificação do macOS (clique abre a
  conversa, se houver `conversa_id`, ou a automação).
