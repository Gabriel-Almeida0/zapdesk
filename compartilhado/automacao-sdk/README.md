# @zapdesk/automacao — SDK das automações de IA

Automações de IA do ZapDesk são projetos TypeScript (pasta
`<pasta-dados>/automacoes/<id>/`) com um `automacao.json` (manifesto) e um `index.ts` que
exporta `definirAutomacao({...})`. O motor compila o projeto com esbuild e o executa num processo
Node isolado (o *runner*), que fala com o motor só por JSON-RPC no stdio. Contrato completo:
`specs/002-automacoes/contracts/sdk-automacao.md` (API) e `runner-protocolo.md` (protocolo).

## Exemplo

```ts
import { definirAutomacao, ErroBloqueado } from '@zapdesk/automacao';
import prompt from './prompt.md'; // .md e .txt são importados como texto

export default definirAutomacao({
  async aoReceberMensagem(ctx, msg) {
    if (!msg.texto) return;
    const historico = await ctx.conversa!.historicoParaIA({ limite: 20 });
    const resposta = await ctx.ia.gerar({ sistema: prompt, mensagens: historico });
    if (resposta.texto.includes('[HUMANO]')) {
      await ctx.humano.transferir({ mensagem: 'Vou chamar um atendente.' });
      return;
    }
    try {
      await ctx.responder(resposta.texto);
    } catch (erro) {
      if (erro instanceof ErroBloqueado) ctx.log.aviso('envio barrado:', erro.motivo);
      else throw erro;
    }
  },
});
```

## Handlers

| Handler | Quando roda | Argumento |
|---------|-------------|-----------|
| `aoReceberMensagem(ctx, mensagem)` | gatilhos `mensagem_recebida`, `palavra_chave` | `MensagemRecebida` |
| `aoAgendar(ctx, agendamento)` | gatilho `agendamento` e chamadas de `ctx.agendar` | `Agendamento` |
| `aoExecutar(ctx, entrada)` | manual (app/MCP), ação `executar_ia` de fluxo, nó `ia` de chatbot | JSON; o retorno (JSON ≤ 64 KB) volta a quem chamou |
| `aoEvento(ctx, evento)` | `lead_importado`, `etiqueta`, `entrou_etapa`, `disparo_respondeu`, `sem_resposta` | `EventoAutomacao` |

## O `ctx`

| API | Permissão | O que faz |
|-----|-----------|-----------|
| `ctx.execucao` | — | id, automação, gatilho, origem, `simulacao`, `prazoMs` restante |
| `ctx.conversa.historico()` / `historicoParaIA()` | `ler_conversas` | mensagens da conversa (a segunda já no formato da IA) |
| `ctx.responder()`, `ctx.enviar()`, `ctx.reagir()` | `enviar` | envia pelo portão (pausa, grupos, anti-loop, primeiro contato) |
| `ctx.humano.transferir()` | `enviar` | pausa a conversa para atendimento humano e avisa o operador |
| `ctx.etiquetas.*` | `etiquetas` | listar, do contato, adicionar, remover (por nome ou id) |
| `ctx.funil.*` | `funil` | listar, posição, mover, remover (por nome ou id) |
| `ctx.leads.*` | `leads` | atual, obter, buscar por telefone, atualizar campos |
| `ctx.ia.gerar/classificar/extrair` | `ia` | Claude, chamada **pelo motor** (a chave nunca entra na automação) |
| `ctx.http.fetch()` | `rede` | `fetch` de http(s), registrado no log sem query nem corpo |
| `ctx.agendar()` / `cancelarAgendamento()` | `agendar` | chamada futura de `aoAgendar` (60 s a 30 dias) |
| `ctx.memoria.*` | — | chave/valor JSON por automação (`global` ou por `contato`) |
| `ctx.segredos.obter()` / `tem()` | — | só segredos declarados em `automacao.json` › `segredos` |
| `ctx.notificar()` | — | notificação do macOS (máx. 5 por execução) |
| `ctx.log.*`, `console.*` | — | log da execução (aparece em Execuções) |
| `ctx.sinal` | — | `AbortSignal` abortado no tempo limite ou cancelamento |

Sem a permissão declarada, a chamada falha com `ErroPermissao`
("Permissão 'enviar' não declarada em automacao.json"). Em **simulação** (Testar), escritas não
acontecem: voltam resultados simulados (`MensagemEnviada.simulada = true`) e aparecem em "ações
que seriam feitas". Chamadas feitas depois do fim da execução (promessas soltas) falham com
`ErroExecucaoEncerrada`.

## Erros

`ErroAutomacao` (base, com `codigo`), `ErroPermissao` (`permissao`), `ErroBloqueado` (`motivo`:
`pausa`, `pausa_geral`, `grupo`, `anti_loop`, `primeiro_contato`), `ErroIA` (`status`,
`requestId`), `ErroSegredo`, `ErroValidacao` (`campos`), `ErroNaoEncontrado`,
`ErroExecucaoEncerrada`. Erro não tratado num handler marca a execução como `erro` (com o stack
apontando para o `.ts`); 5 erros seguidos desativam a automação.

## Módulos disponíveis

Só `@zapdesk/automacao`, arquivos do próprio projeto (`.ts`, `.json`, `.md`, `.txt`) e os módulos
Node `node:crypto`, `node:util`, `node:url`, `node:buffer`, `node:events`,
`node:timers/promises`, `node:path/posix`, `node:string_decoder` e `node:querystring`. Qualquer
outro (`fs`, `child_process`, `net`, `http`, `os`, pacotes npm…) dá "Módulo não permitido: <nome>"
na compilação e também em tempo de execução (`import()` dinâmico, `process.getBuiltinModule`).
O `fetch` global, `WebSocket` e `EventSource` não existem (use `ctx.http.fetch`) e `process.env`
é vazio.

## Limites

60 s por execução (o processo é morto no prazo), cerca de 256 MB de heap por processo, retorno de
`aoExecutar` ≤ 64 KB, texto de log ≤ 8 KB por linha, mensagens JSON-RPC ≤ 1 MB.

## Limite honesto do isolamento

O isolamento **protege contra erros e acidentes, não contra código malicioso**. Cada automação
roda num processo separado (um laço infinito ou estouro de memória derruba só ele) com o Permission
Model do Node (sem leitura de arquivos além do próprio bundle, sem processos filhos nem workers),
lista de módulos na compilação e no carregamento, e sem token da API nem chave da Anthropic. Mas o
Permission Model do Node não bloqueia rede e a própria documentação do Node avisa que ele não é
uma barreira contra código mal-intencionado. Só rode automações cujo código você escreveu ou
revisou.

## Desenvolvimento

```bash
npm run compilar -w @zapdesk/automacao   # dist/index.js + dist/index.d.ts (arquivo único)
npm test -w @zapdesk/automacao
npm run sdk:sincronizar                  # copia o .d.ts para o motor (go:embed); a CI confere
```

O `dist/index.d.ts` é um arquivo de declarações único (`declare module '@zapdesk/automacao'` +
módulos `*.md`/`*.txt`), gerado por `juntar-dts.mjs`; funciona via `paths` (VS Code) e como lib
extra do Monaco. Reexporte módulos em `src/index.ts` sempre com `export * from './x.js'`.
