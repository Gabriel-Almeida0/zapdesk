# Feature: Automações — funil de vendas, chatbots e automações de IA programáveis

> **Status:** Especificação pronta para implementação
> **Slug:** `automacoes`
> **Criado em:** 2026-09-27
> **Autor da spec:** Claude. O usuário pediu a feature e foi dormir, autorizando decidir sem perguntar ("não me pergunte… confio em você"). Toda decisão tomada no lugar dele está na §14 com a razão, para revisão de manhã.

---

## 1. Resumo executivo

O ZapDesk ganha três tipos de automação sobre o mesmo motor de eventos:

1. **Automações de funil** (sem código): gatilho → condições → ações, ligadas a um funil de vendas com etapas.
2. **Chatbots** (sem código): fluxos de conversa em nós, como menu, pergunta, captura de resposta, condição e transferência para humano.
3. **Automações de IA** (com código, que o usuário programa de verdade): cada uma é um **projeto TypeScript isolado**, com arquivos próprios, rodando no seu próprio processo, com uma API tipada (`ctx`) para ler conversas, responder, mexer em etiquetas e funil, guardar memória e chamar modelos de IA. O editor fica dentro do app e cada automação também é uma pasta que pode ser aberta no VS Code.

O pedido do usuário foi explícito: automações de IA são para **programar, não configurar**, e cada uma é isolada das outras.

---

## 2. Contexto e motivação

### 2.1 Problema
O MVP dispara em massa e recebe respostas, mas tudo depois da resposta é manual: qualificar o lead, responder perguntas comuns, mover no funil, fazer follow-up. Com leads frios em volume, responder à mão não escala.

### 2.2 Quem é afetado
O operador (o usuário) e a IA via MCP.

### 2.3 O que existe hoje
Etiquetas, notas, disparos com o estado "respondeu" e o MCP. Não há funil, bot nem execução de código.

### 2.4 Resultado esperado
- A resposta de um lead dispara um bot de qualificação.
- Leads qualificados andam no funil sozinhos.
- Follow-ups saem sozinhos depois de X horas sem resposta.
- Automações de IA escritas em TypeScript respondem, classificam e extraem dados com Claude.
- A IA, via MCP, também consegue criar e editar essas automações.

---

## 3. Escopo

### 3.1 Dentro do escopo
**Funil de vendas**
- Funis com etapas ordenadas (nome, cor). Vários funis.
- Um contato ou lead fica em no máximo uma etapa por funil.
- Tela Kanban: arrastar cartões entre etapas.
- Histórico de movimentação.

**Motor de eventos (comum aos três tipos)**
- Gatilhos:
  - mensagem recebida, com filtros opcionais: conta, conversa individual ou grupo, texto contém ou regex, primeira mensagem do contato;
  - palavra-chave;
  - lead importado;
  - etiqueta adicionada ou removida;
  - contato entrou numa etapa do funil;
  - destinatário de disparo respondeu;
  - sem resposta há X tempo, depois de uma mensagem enviada;
  - agendamento (cron ou intervalo);
  - manual (botão "Executar" e ferramenta MCP).
- Condições: etiqueta tem ou não tem, etapa do funil, campo do lead, horário/dia da semana, texto da mensagem, conta.
- Ações:
  - enviar texto (com variáveis) ou template;
  - aguardar (duração), que é persistido e sobrevive ao fechar o app;
  - adicionar ou remover etiqueta;
  - mover de etapa;
  - atualizar nota ou campo do lead;
  - iniciar chatbot;
  - executar automação de IA;
  - adicionar a um disparo;
  - pausar automações na conversa (atendimento humano);
  - notificar o operador (notificação do macOS).

**Chatbots**
- Editor visual de fluxo em nós:
  - início;
  - enviar mensagem;
  - menu de opções (o usuário responde número ou texto);
  - pergunta com captura para variável, com validação opcional (e-mail, número, regex);
  - condição (ramifica);
  - ação (qualquer ação da lista acima);
  - chamar automação de IA (usa o retorno como resposta ou variável);
  - transferir para humano;
  - fim.
- Sessão de bot por conversa: nó atual, variáveis e tempo limite de inatividade configurável, depois do qual a sessão encerra.
- Mensagem de "não entendi" com número máximo de tentativas.

**Automações de IA programáveis**
- Cada automação é uma pasta própria: `automacoes/<id>/` dentro da pasta de dados, com `automacao.json` (nome, gatilhos, permissões), `index.ts` e outros arquivos `.ts` que o usuário quiser.
- O código exporta handlers, por exemplo:
  ```ts
  import { definirAutomacao } from "@zapdesk/automacao";
  export default definirAutomacao({
    async aoReceberMensagem(ctx, msg) { … },
    async aoAgendar(ctx) { … },
    async aoExecutar(ctx, entrada) { … },  // chamada por fluxo, bot ou MCP
  });
  ```
- SDK `@zapdesk/automacao` com tipos completos. O `ctx` dá acesso a:
  - `ctx.conversa`: histórico e contato;
  - `ctx.responder()` e `ctx.enviar()`: texto, mídia, reação;
  - `ctx.etiquetas`;
  - `ctx.funil`;
  - `ctx.leads`;
  - `ctx.memoria`: chave-valor persistente, isolada por automação e, opcionalmente, por contato;
  - `ctx.ia`: helpers para a Claude API (`gerar`, `classificar`, `extrair` com esquema) e, via `ctx.http.fetch`, qualquer outro provedor;
  - `ctx.log`;
  - `ctx.agendar()`;
  - `ctx.segredos`: chaves de API configuradas no app, nunca no código.
- **Isolamento:**
  - cada automação roda num **processo próprio**, executado pelo runtime Node embutido no Electron;
  - só enxerga a API do motor através de um token com **escopo daquela automação**, com as permissões declaradas em `automacao.json` (ex.: `enviar`, `ler_conversas`, `etiquetas`, `funil`, `rede`);
  - tem limites de tempo por execução e de memória;
  - se falhar, não derruba as outras nem o motor.
- **Editor no app:**
  - editor de código (Monaco) com autocompletar da SDK;
  - "Testar" com mensagem simulada e visualização do log e das ações que seriam feitas (modo **simulação**, que não envia nada de verdade);
  - "Ativar" e "Desativar";
  - lista de execuções com logs e erros;
  - botão "Abrir pasta no editor externo".
- **Compilação:** o TypeScript é compilado com esbuild (embutido), e erros de tipo e compilação aparecem no editor.
- **Modelos de projeto** ao criar uma automação: "Responder com IA usando histórico", "Classificar lead e mover no funil", "Extrair dados (nome, empresa, interesse)", "Em branco".

**MCP:** ferramentas para listar, criar e editar funis, etapas, automações de funil, chatbots e automações de IA (incluindo **ler e escrever os arquivos de código** de uma automação de IA, compilar, testar em simulação, ativar e desativar) e para ver execuções e logs.

**Segurança anti-loop, em todas as automações**
- Nunca disparam com mensagens enviadas pelo próprio número.
- Limite de mensagens automáticas por conversa por janela de tempo (padrão 10 a cada 10 min, configurável; o chatbot conta, e 5 estouraria em bots legítimos). Ao estourar, pausa as automações naquela conversa e notifica.
- Quando o operador responde manualmente numa conversa, as automações daquela conversa pausam por um tempo configurável (padrão 30 min). É o "atendimento humano".
- Grupos ficam fora por padrão; precisam ser habilitados explicitamente em cada automação.

### 3.2 Fora do escopo
- Marketplace ou compartilhamento de automações.
- Execução de automações com o app fechado (vale o ciclo de vida do MVP: o app liga e desliga tudo). Aguardas e agendamentos que venceram com o app fechado executam ao reabrir, se ainda fizerem sentido (ver §8).
- Pacotes npm arbitrários nas automações de IA. A v1 permite só a SDK e módulos Node que não mexem em arquivos nem em processos; a rede passa por `ctx.http`.
- Modelos de IA locais (Ollama). É possível via `ctx.http.fetch` para `localhost`, mas sem helper dedicado.
- Transcrição de áudio e visão (imagem) nos helpers de IA. O código pode fazer isso via `fetch`, mas não há helper na v1.

### 3.3 Roadmap futuro
- Pacotes npm permitidos por lista.
- Métricas de conversão do funil.
- Versionamento e histórico das automações.
- Teste A/B de mensagens.

---

## 4. Personas
Operador (programa e configura) e IA via MCP (cria, edita, programa e testa automações).

---

## 5. Fluxos UX (principais)

1. **Funil:** Funis → Novo funil → etapas → Kanban. Cartões arrastáveis, com clique abrindo a conversa.
2. **Automação de funil:** Automações → Nova → "Fluxo" → escolher gatilho → condições → ações (lista ordenada com "aguardar") → Salvar → Ativar. Mostra as últimas execuções.
3. **Chatbot:** Automações → Novo → "Chatbot" → canvas de nós conectáveis → "Testar" abre um chat simulado ao lado → Ativar com gatilho (palavra-chave ou primeira mensagem).
4. **Automação de IA:**
   1. Automações → Nova → "IA (código)" → escolher modelo de projeto.
   2. Abre o editor com `index.ts`, `automacao.json` e a árvore de arquivos.
   3. Programar; a compilação roda contínua e os erros aparecem inline.
   4. "Testar" com uma mensagem ou conversa real em modo simulação.
   5. Ativar.
   6. A aba "Execuções" mostra logs, duração, ações e erros.
5. **Ajustes → IA:** chave da Anthropic (guardada no Keychain via `safeStorage`), modelo padrão e demais segredos com nome.
6. **Conversa:** faixa "Bot X ativo nesta conversa · Assumir" e indicador de "atendimento humano".

Estados de vazio, carregando e erro seguem o padrão do MVP. Textos em pt-BR.

---

## 6. Modelo de dados (novas entidades no SQLite do motor)

- `Funil`: id, nome, ordem.
- `Etapa`: id, funil_id, nome, cor, ordem.
- `PosicaoFunil`: contato/lead, funil, etapa, movido_em. Único por (contato, funil).
- `HistoricoFunil`.
- `Automacao`:
  - id, nome, tipo (`fluxo` | `chatbot` | `ia`), ativa, contas (lista ou todas), incluir_grupos, gatilhos (JSON), definição (JSON do fluxo ou do grafo do bot);
  - para `ia`: pasta, permissões, hash da última compilação e erro de compilação;
  - limites anti-loop próprios, criada/atualizada.
- `SessaoChatbot`: automacao_id, conversa, nó atual, variáveis, tentativas, expira_em, estado.
- `ExecucaoAutomacao`: id, automacao_id, gatilho, conversa, início/fim, estado (`rodando` | `ok` | `erro` | `simulacao` | `abortada`), ações realizadas (JSON), log (texto limitado), erro.
- `EsperaAgendada`: execução, retomar_em, passo. Persistida para "aguardar" e "sem resposta há X".
- `PausaConversa`: conversa, até, motivo (`humano` | `anti_loop` | `manual`).
- `MemoriaAutomacao`: automacao_id, escopo (`global` | `contato:<id>`), chave, valor JSON.
- `Segredo`: nome, valor cifrado (o valor fica no Keychain do app e o motor recebe na subida; ver §7).

---

## 7. Aspectos técnicos

- **Motor (Go):**
  - dono dos dados, dos gatilhos, do agendador (esperas e cron), das sessões de chatbot, do executor de fluxos e bots, do anti-loop e da orquestração dos processos de automação de IA;
  - o executor de fluxo e de bot é Go puro, determinístico e testável com relógio controlável.
- **Runner de IA:**
  - novo pacote `automacao/runner`: um script Node empacotado com o app e executado com `ELECTRON_RUN_AS_NODE`, como o MCP;
  - o motor inicia um processo por automação de IA ativa quando ela é acionada (ou mantém vivo com timeout de ociosidade) e encerra tudo ao sair;
  - comunicação por stdio (JSON-RPC) ou HTTP local com token de escopo;
  - limites: tempo por execução (padrão 60 s), memória (`--max-old-space-size`) e uma execução por conversa por vez, com fila.
- **Isolamento de código:**
  - o código do usuário roda num processo separado, sem acesso ao token principal;
  - a API disponível é só o `ctx`, validado no motor contra as permissões declaradas;
  - `fs`, `child_process` e `net` ficam bloqueados por política do runner, com `import` restrito por um resolver do esbuild e checagens em tempo de execução;
  - **limite honesto do isolamento:** isso protege contra erro, não contra código malicioso. O código é do próprio usuário. Isso fica documentado.
- **SDK:** `compartilhado/automacao-sdk` (`@zapdesk/automacao`), com tipos e `definirAutomacao`. É distribuído junto com o app para o editor ter autocompletar.
- **Compilação:** esbuild (binário nativo embutido no app ou no motor).
- **Editor:** Monaco no renderer, com os tipos da SDK carregados.
- **Claude API:**
  - helpers em `ctx.ia` usam a Messages API com modelo padrão configurável, hoje `claude-sonnet-5`, com opção de `claude-opus-5-5` e `claude-haiku-4-5-20251001`;
  - a chave vem de Ajustes e nunca vai para o código nem para os logs.
- **Segredos:** guardados pelo app no Keychain (Electron `safeStorage`). O motor recebe os segredos em memória na subida, via stdin, e os entrega ao runner só para as automações que declaram usá-los.
- **Eventos WS** novos: automacao.execucao.*, funil.movido, chatbot.sessao.*, conversa.pausa.
- **Testes:** unitários do executor de fluxo e bot, do anti-loop, das esperas e dos gatilhos; integração do runner com o cliente WhatsApp falso (automação de IA de exemplo com `ctx.ia` falso); MCP contra o motor falso.

---

## 8. Edge cases

| Cenário | Comportamento |
|---|---|
| Dois gatilhos disparam para a mesma mensagem | Todas as automações que casam executam, em ordem de prioridade (campo `prioridade`). Chatbot com sessão ativa na conversa tem precedência e bloqueia outros bots. |
| Bot conversa com outro bot (loop) | O anti-loop corta e pausa a conversa |
| Operador responde manualmente | Pausa as automações na conversa (padrão 30 min) e encerra a sessão do bot com o estado "humano" |
| App fechado durante um "aguardar" | Ao reabrir, esperas vencidas executam se venceram há menos de 24 h. Senão são marcadas `abortada` (motivo: expirada), para não mandar mensagem velha. |
| Automação de IA lança erro ou passa do tempo | Execução `erro` com stack. Após 5 erros seguidos, desativa sozinha e notifica. |
| Erro de compilação | Não ativa, e o editor mostra os erros |
| Chave da Anthropic ausente ou inválida | `ctx.ia` lança um erro claro, e a execução fica `erro` com a mensagem "Configure a chave da Anthropic em Ajustes → IA" |
| Resposta 429/5xx da Claude API | Retentativa com backoff (até 3 tentativas) dentro do tempo limite |
| Conta desconectada | Execuções que enviam falham com motivo; as esperas aguardam a reconexão até vencer |
| Contato em disparo e em bot ao mesmo tempo | Permitido; o anti-loop limita |
| Automação editada com sessões ativas | As sessões de chatbot continuam na versão antiga até terminar (a definição fica guardada na sessão) |
| Mensagem em grupo | Ignorada, a não ser que a automação tenha `incluir_grupos` |

---

## 9. Plano de implementação
1. Spec Kit (feature 002).
2. Motor: modelo, migrações, motor de eventos, executor de fluxo, anti-loop, esperas, funil, chatbot, orquestração do runner.
3. Runner e SDK.
4. App: Kanban, editor de fluxo, canvas do chatbot, editor de código (Monaco) com testes e execuções, Ajustes → IA.
5. MCP.
6. Verificação E2E com o WhatsApp falso.

---

## 10. Critérios de aceite
- [ ] Criar um funil com 4 etapas e mover um cartão; o histórico registra a mudança.
- [ ] Fluxo "lead respondeu disparo → etiqueta 'quente' → mover para 'Qualificando' → iniciar bot" funciona de ponta a ponta com o WhatsApp falso.
- [ ] Follow-up "sem resposta há 2 h → enviar template" funciona com o relógio controlável e sobrevive ao reinício.
- [ ] Chatbot com menu, captura de e-mail validado, condição e transferência para humano funciona no teste simulado e com mensagens injetadas.
- [ ] Automação de IA criada pelo modelo "Responder com IA" compila, testa em simulação (com `ctx.ia` falso nos testes automatizados) e responde mensagens reais injetadas quando ativa.
- [ ] Uma automação de IA com erro ou loop infinito não afeta outras, termina por tempo limite e é desativada depois de 5 erros.
- [ ] Uma automação sem a permissão `enviar` não consegue enviar (erro de permissão).
- [ ] O anti-loop pausa a conversa após o limite; resposta manual pausa as automações.
- [ ] Pelo MCP: criar uma automação de IA escrevendo `index.ts`, compilar, testar e ativar.
- [ ] Ao fechar o app, nenhum processo de runner sobra.
- [ ] Os critérios do MVP continuam passando (motor ocioso < 80 MB sem automações de IA rodando).

---

## 11. Métricas
Execuções por automação (ok e erro), tempo médio e conversões por etapa (contagem simples no Kanban).

## 12. Riscos
| Risco | Mitigação |
|---|---|
| IA responde algo errado ou inventado a um lead | Modo simulação, logs, pausa em atendimento humano, prompt dos modelos de projeto conservador |
| Custo da Claude API | Log de tokens por execução, visível na tela de execuções |
| Loops e spam gerando ban | Anti-loop obrigatório e grupos desligados por padrão |
| Isolamento por processo é contornável por código malicioso | O código é do próprio usuário; limite documentado |
| Peso: vários processos Node | Processo sob demanda com tempo de ociosidade (padrão 5 min) e no máximo N processos simultâneos (padrão 4) |

## 13. Questões em aberto
- [ ] Revisar os padrões assumidos: anti-loop 10 a cada 10 min (era 5; elevado na implementação, ver §14); pausa humana de 30 min; esperas vencidas há mais de 24 h descartadas; tempo limite de IA de 60 s; modelo padrão `claude-sonnet-5`.

## 14. Decisões registradas (tomadas pelo Claude com o usuário dormindo)
- **Motor Go como dono do motor de eventos.** Segue a constituição: fonte única da verdade, para app e MCP verem o mesmo estado.
- **Automações de IA em TypeScript, num processo Node isolado, e não JS embutido no Go (goja).** O usuário pediu para programar de verdade: TS com tipos, async/await e `fetch` nativo. O runtime Node já vem no Electron (o MCP usa o mesmo), então não entra dependência nova. O isolamento por processo atende o "isoladamente".
- **Pasta própria por automação**, abrível no VS Code. Programar de verdade inclui usar o editor que o usuário quiser e ter vários arquivos.
- **Monaco no app**, para programar sem sair do app, com autocompletar da SDK.
- **Claude API como helper padrão de IA**, com qualquer outro provedor possível via `ctx.http.fetch`.
- **Anti-loop e pausa humana obrigatórios.** Sem eles, automações de IA respondendo leads frios geram loops e aumentam o risco de ban.
- **Anti-loop padrão de 10 mensagens automáticas a cada 10 min (era 5).** Decidido na implementação: o chatbot conta no anti-loop, e boas-vindas + menu + "não entendi" + perguntas passam de 5 numa conversa legítima. Continua configurável em Ajustes e por automação (só mais restritivo).
- **Sem execução com o app fechado.** Preserva a decisão do MVP de desligar tudo ao fechar.
