# Especificação da Feature: Automações — funil de vendas, chatbots e automações de IA programáveis

**Branch da feature**: `002-automacoes`

**Criada em**: 2026-09-27

**Status**: Rascunho

**Entrada**: Descrição do usuário: "Automações: funil de vendas, chatbots e automações de IA
programáveis em TypeScript isoladas" (entrevista e decisões em `docs/features/automacoes.md`,
fonte da verdade desta feature; decisões na §14 e padrões da §13 adotados).

## Visão geral

O ZapDesk (feature 001) conecta contas de WhatsApp, recebe respostas e dispara em massa, mas tudo
o que acontece depois da resposta de um lead é manual. Esta feature acrescenta três tipos de
automação sobre um mesmo motor de eventos, mais um funil de vendas:

1. **Funil de vendas**: funis com etapas ordenadas e um quadro Kanban onde cada lead ocupa no
   máximo uma etapa por funil.
2. **Automações de fluxo** (sem código): gatilho → condições → ações, incluindo esperas que
   sobrevivem ao fechamento do app.
3. **Chatbots** (sem código): fluxos de conversa em nós (menu, pergunta com captura, condição,
   transferência para humano).
4. **Automações de IA** (com código): cada uma é um pequeno projeto TypeScript, programado pelo
   usuário ou pela IA via MCP, executado isoladamente, com uma API tipada (`ctx`) para ler
   conversas, responder, mexer em etiquetas e funil, guardar memória e chamar modelos Claude.

Proteções obrigatórias valem para todas as automações: nunca reagir a mensagens do próprio
número, limite de mensagens automáticas por conversa, pausa automática quando o operador responde
à mão ("atendimento humano") e grupos desligados por padrão.

### Glossário

- **Automação**: regra ativa ou inativa de um dos três tipos: `fluxo`, `chatbot` ou `ia`.
- **Gatilho**: evento que inicia uma execução (mensagem recebida, palavra-chave, lead importado,
  etiqueta, etapa do funil, resposta a disparo, sem resposta há X tempo, agendamento, manual).
- **Execução**: uma rodada de uma automação para um gatilho, com estado, log e ações realizadas.
- **Ação automática**: qualquer efeito produzido por uma execução (enviar mensagem, etiquetar,
  mover no funil…). Mensagens enviadas por execuções são **mensagens automáticas**.
- **Espera**: pausa persistida dentro de uma execução ("aguardar 2 h") ou verificação agendada
  ("sem resposta há 2 h").
- **Sessão de chatbot**: estado de um chatbot numa conversa (nó atual, variáveis, tentativas,
  expiração).
- **Pausa da conversa**: período em que nenhuma automação age numa conversa. Motivos: `humano`
  (operador respondeu à mão ou clicou "Assumir"), `anti_loop` (limite estourado) ou `manual`
  (ação "pausar automações").
- **Atendimento humano**: pausa com motivo `humano`.
- **Modo simulação**: execução de teste que lê dados reais mas **não envia nem altera nada**;
  apenas registra as ações que seriam feitas.
- **Projeto da automação de IA**: pasta própria com `automacao.json` (manifesto: nome, gatilhos,
  permissões, segredos) e arquivos `.ts`.
- **Permissão**: capacidade declarada no manifesto (`enviar`, `ler_conversas`, `etiquetas`,
  `funil`, `leads`, `ia`, `rede`, `agendar`). Sem a permissão, a chamada falha. A memória própria da
  automação, o log e as notificações ao operador não exigem permissão.
- **Segredo**: valor sensível com nome (ex.: chave da Anthropic) guardado pelo app no chaveiro do
  macOS e nunca exibido em código, log ou resposta de API.
- **Card do funil**: um lead posicionado numa etapa de um funil.

## Clarifications

### Session 2026-09-27

> Clarificações resolvidas pelo agente sem perguntar ao usuário (ele está dormindo e proibiu
> perguntas), usando `docs/features/automacoes.md`. Os padrões da §13 do documento foram adotados
> como **decisão assumida**; onde o documento não decidia, a escolha está justificada e marcada
> como **decisão do agente**, para revisão.

- Q: Quais padrões de segurança e limites valem na v1? → A: Decisão assumida (§13), com um ajuste: anti-loop de
  **10** mensagens automáticas por conversa a cada 10 min (o documento dizia 5; ver a decisão do
  orquestrador abaixo); pausa humana de 30 min; esperas vencidas há
  mais de 24 h descartadas (`abortada`, motivo "expirada"); tempo limite de 60 s por execução de
  IA; modelo padrão `claude-sonnet-5`. Todos configuráveis em Ajustes → Automações (os de
  anti-loop também por automação).
- Q: 5 mensagens automáticas a cada 10 min bastam para chatbots legítimos? → A: Decisão do
  orquestrador (2026-09-27, na implementação): **não**. O chatbot conta no anti-loop, e um bot
  normal (boas-vindas, menu, "não entendi", perguntas, confirmação) passa de 5 mensagens numa
  conversa legítima. O padrão global passa a **10 mensagens automáticas a cada 10 min**; a
  automação continua podendo ter limite próprio mais restritivo e o usuário pode mudar em Ajustes.
- Q: O que é o "card" do funil: contato ou lead? → A: Decisão do agente: o card é sempre um
  **lead** (único por telefone, vale para todas as contas). Colocar no funil um contato que ainda
  não é lead cria o lead automaticamente (origem `contatos`), como já faz a importação de
  contatos do MVP. Assim um mesmo telefone nunca aparece duas vezes no mesmo funil, e "campo do
  lead" funciona em condições e ações. Grupos não entram em funil.
- Q: Quais mensagens contam como "o operador respondeu manualmente" e disparam o atendimento
  humano? → A: Decisão do agente: toda mensagem enviada pelo próprio número numa conversa
  individual que **não** foi gerada por uma automação nem por um disparo — enviada pela janela do
  app, pelo MCP ou pelo celular. Cada nova mensagem manual renova a pausa. O botão "Assumir" na
  conversa cria uma pausa `humano` sem prazo, até "Devolver às automações".
- Q: Enquanto uma sessão de chatbot está ativa numa conversa, outras automações reagem às
  mensagens dessa conversa? → A: Decisão do agente: não. A mensagem vai só para a sessão ativa;
  gatilhos de mensagem (mensagem recebida, palavra-chave, primeira mensagem) de outras automações
  não disparam para ela, e nenhum outro chatbot pode iniciar. Gatilhos que não são de mensagem
  (etiqueta, etapa, agendamento, manual) continuam valendo.
- Q: No "Testar" (modo simulação) de uma automação de IA, as chamadas à Claude API são reais? →
  A: Decisão do agente: sim, por padrão, se a chave estiver configurada (o usuário quer ver o que a
  IA responderia); o painel de teste tem a opção "IA simulada" que devolve respostas fixas sem
  rede nem custo, e os testes automatizados sempre usam a IA simulada. Nada é enviado ao WhatsApp
  nem alterado no banco em simulação; gravações em `ctx.memoria` valem só dentro daquela execução.

Outras decisões do agente (registradas também em Assumptions):

- Segredos (inclusive a chave da Anthropic) só podem ser **definidos** pela janela do app
  (Ajustes → IA). A API e o MCP apenas listam os **nomes** dos segredos configurados.
- Verificação de tipos TypeScript aparece no editor do app; a compilação pedida pelo MCP devolve
  erros de sintaxe, de importação e de compilação (não de tipos), e o MCP oferece os tipos da SDK
  para a IA consultar.
- Para reduzir risco de ban, mensagens automáticas que **iniciam** uma conversa (contato sem
  mensagem anterior) têm limite global de 20 por hora por conta (configurável); acima disso a
  ação falha com orientação para usar um disparo.
- O limite anti-loop, ao estourar, pausa a conversa por 60 min (configurável) e notifica.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Organizar leads num funil de vendas com Kanban (Priority: P1)

O usuário cria um funil ("Prospecção") com etapas ordenadas e coloridas ("Novo", "Qualificando",
"Proposta", "Fechado"), abre o quadro Kanban, adiciona leads e contatos às etapas, arrasta cartões
entre etapas e abre a conversa do lead com um clique. Cada movimentação fica no histórico.

**Why this priority**: o funil é o estado de negócio que as automações movimentam; sem ele as
automações de qualificação não têm onde registrar o avanço do lead.

**Independent Test**: criar um funil com 4 etapas, adicionar 3 leads, arrastar um cartão de
"Novo" para "Qualificando" e conferir o histórico do lead e a contagem por etapa.

**Acceptance Scenarios**:

1. **Given** nenhum funil, **When** o usuário cria "Prospecção" com as etapas "Novo",
   "Qualificando", "Proposta" e "Fechado", **Then** o Kanban mostra 4 colunas vazias na ordem
   definida, cada uma com sua cor e contagem 0.
2. **Given** um funil com etapas, **When** o usuário adiciona um contato que ainda não é lead à
   etapa "Novo", **Then** o lead é criado (origem `contatos`) e o cartão aparece em "Novo" com
   nome, telefone e etiquetas.
3. **Given** um cartão em "Novo", **When** o usuário o arrasta para "Qualificando" (ou usa
   "Mover para…" pelo teclado), **Then** o cartão muda de coluna, a contagem das duas colunas é
   atualizada e o histórico registra "Novo → Qualificando", data, hora e origem (`app`).
4. **Given** um lead já presente no funil, **When** alguém tenta adicioná-lo a outra etapa do
   mesmo funil, **Then** o lead é movido (nunca duplicado) e o histórico registra a mudança.
5. **Given** um cartão, **When** o usuário clica nele, **Then** abre a conversa desse telefone na
   conta em que existe conversa (se houver mais de uma, a mais recente); sem conversa, oferece
   "Iniciar conversa".
6. **Given** uma etapa com cartões, **When** o usuário tenta excluí-la, **Then** o app pede para
   onde mover os cartões (outra etapa ou "remover do funil") antes de excluir.
7. **Given** o mesmo funil aberto na janela, **When** a IA move um lead pelo MCP, **Then** o
   cartão muda de coluna na janela sem recarregar.

---

### User Story 2 - Automatizar o funil com gatilhos, condições, ações e esperas (Priority: P1)

O usuário cria uma automação do tipo "Fluxo": escolhe um gatilho (ex.: "destinatário de disparo
respondeu"), condições opcionais (ex.: "não tem a etiqueta 'cliente'") e uma lista ordenada de
ações (etiquetar "quente", mover para "Qualificando", aguardar 2 h, enviar template…). Ativa e
acompanha as últimas execuções. As proteções anti-loop e de atendimento humano agem sozinhas.

**Why this priority**: é o ganho direto de escala pedido: leads que respondem andam no funil e
recebem follow-up sem trabalho manual.

**Independent Test**: com o WhatsApp simulado, criar o fluxo "respondeu disparo → etiqueta
'quente' → mover para 'Qualificando'", injetar a resposta de um destinatário e conferir etiqueta,
etapa e a execução `ok`; criar o fluxo "sem resposta há 2 h → enviar template", avançar o relógio
controlável e conferir o envio, inclusive após reiniciar o motor no meio da espera.

**Acceptance Scenarios**:

1. **Given** um fluxo ativo com gatilho "destinatário de disparo respondeu" e ações "adicionar
   etiqueta 'quente'" e "mover para 'Qualificando'", **When** um destinatário responde, **Then** o
   contato recebe a etiqueta, o lead vai para "Qualificando" e a execução aparece como `ok` com as
   duas ações listadas.
2. **Given** um fluxo "sem resposta há 2 h → enviar template 'Follow-up'", **When** o usuário
   envia uma mensagem a um lead e passam 2 h sem resposta, **Then** o template é enviado uma vez e
   a execução registra o envio; **When** o lead responde antes das 2 h, **Then** nada é enviado.
3. **Given** um fluxo com "aguardar 1 h" em andamento, **When** o app é fechado e reaberto 30 min
   depois, **Then** a espera continua e a ação seguinte acontece 1 h após o início; **When** o app
   fica fechado 30 h, **Then** a execução é marcada `abortada` (motivo "expirada") sem enviar.
4. **Given** qualquer automação ativa que envia mensagens, **When** uma conversa recebe 10
   mensagens automáticas em 10 min e uma 11ª seria enviada, **Then** a 11ª não é enviada, as
   automações da conversa pausam (motivo `anti_loop`) e o operador recebe notificação.
5. **Given** uma automação ativa numa conversa, **When** o operador responde à mão (pelo app, pelo
   MCP ou pelo celular), **Then** as automações dessa conversa pausam por 30 min, a conversa mostra
   "Atendimento humano até HH:MM" e esperas em andamento nessa conversa são `abortada` ao vencer.
6. **Given** um fluxo com gatilho "mensagem recebida", **When** chega mensagem em grupo, **Then**
   nada executa, a não ser que o fluxo tenha "incluir grupos" ligado.
7. **Given** um fluxo com gatilho "agendamento" (todo dia útil às 9h), **When** chega o horário,
   **Then** o fluxo executa uma vez; horários perdidos com o app fechado executam uma única vez na
   reabertura se venceram há menos de 24 h.
8. **Given** um fluxo com o botão "Executar", **When** o usuário escolhe um contato e executa,
   **Then** o fluxo roda para esse contato ignorando o gatilho, respeitando anti-loop e pausas.
9. **Given** um fluxo com condições "tem etiqueta 'lead'" e "horário entre 9h e 18h", **When** o
   gatilho ocorre fora do horário, **Then** a execução termina `ok` com "condições não atendidas" e
   nenhuma ação.

---

### User Story 3 - Programar automações de IA em TypeScript, testar e ativar (Priority: P1)

O usuário cria uma automação "IA (código)" a partir de um modelo ("Responder com IA usando
histórico", "Classificar lead e mover no funil", "Extrair dados", "Em branco"). O editor do app
abre o projeto com `index.ts`, `automacao.json` e a árvore de arquivos, com autocompletar da API da
automação e erros exibidos enquanto digita. "Testar" roda o código em modo simulação com uma
mensagem digitada ou uma conversa real e mostra log e ações que seriam feitas. "Ativar" liga a
automação; a aba "Execuções" mostra logs, duração, ações, tokens de IA e erros. Cada automação é
isolada: erro, lentidão ou laço infinito numa delas não afeta as outras nem o app. A pasta do
projeto pode ser aberta no editor externo (ex.: VS Code). Em Ajustes → IA o usuário guarda a chave
da Anthropic, escolhe o modelo padrão e cadastra outros segredos com nome.

**Why this priority**: é o pedido central do usuário ("programar, não configurar"), e o que
permite responder, classificar e extrair dados de leads com Claude.

**Independent Test**: criar pelo modelo "Responder com IA usando histórico", conferir que compila
sem erros, testar em simulação com IA simulada (log e ação "enviar" exibidos, nada enviado),
ativar e injetar uma mensagem real no WhatsApp simulado: a resposta é enviada. Criar outra
automação com `while(true){}` e conferir que termina por tempo limite, não afeta a primeira e é
desativada após 5 erros seguidos.

**Acceptance Scenarios**:

1. **Given** Ajustes → IA sem chave, **When** o usuário cola a chave da Anthropic e salva,
   **Then** a chave fica guardada no chaveiro do macOS, aparece mascarada ("sk-ant-…a1b2") e nunca
   é exibida por inteiro de novo; "Testar chave" confirma se a chave funciona.
2. **Given** "Nova automação → IA (código) → Responder com IA usando histórico", **When** o
   usuário confirma um nome, **Then** o editor abre com `automacao.json`, `index.ts` e
   `prompt.md`, sem erros, com autocompletar ao digitar `ctx.`.
3. **Given** um erro de digitação no código, **When** o usuário edita, **Then** o erro aparece
   sublinhado com mensagem em até 2 s, e "Ativar" fica bloqueado enquanto houver erro de
   compilação.
4. **Given** um projeto válido, **When** o usuário clica "Testar", escreve "Quanto custa?" e
   executa, **Then** o painel mostra o log, as ações que seriam feitas (ex.: "enviar: 'O valor
   é…'") e a duração, e nada é enviado ao WhatsApp nem alterado no banco.
5. **Given** a automação ativa com gatilho "mensagem recebida", **When** um contato manda
   mensagem, **Then** a automação responde na conversa, e a execução aparece em "Execuções" com
   estado `ok`, duração, ações e tokens de entrada/saída.
6. **Given** um manifesto sem a permissão `enviar`, **When** o código chama `ctx.responder()`,
   **Then** a chamada falha com "Permissão 'enviar' não declarada em automacao.json" e a execução
   fica `erro`.
7. **Given** uma automação com laço infinito, **When** é acionada, **Then** é interrompida no tempo
   limite (60 s), a execução fica `erro` ("Tempo limite de 60 s excedido") e outras automações
   continuam respondendo normalmente; após 5 erros seguidos ela é desativada sozinha e o operador
   é notificado.
8. **Given** chave da Anthropic ausente ou inválida, **When** o código chama `ctx.ia.gerar()`,
   **Then** a execução fica `erro` com "Configure a chave da Anthropic em Ajustes → IA".
9. **Given** um projeto, **When** o usuário clica "Abrir pasta no editor externo", **Then** a pasta
   abre no editor padrão com tipos da API disponíveis para autocompletar; alterações salvas lá
   aparecem no editor do app e são compiladas antes da próxima execução.
10. **Given** o app sendo fechado com automações de IA em execução, **When** o app encerra,
    **Then** nenhum processo das automações continua rodando.

---

### User Story 4 - Criar chatbots de qualificação em nós (Priority: P2)

O usuário cria um "Chatbot" num canvas de nós conectáveis (início, enviar mensagem, menu de
opções, pergunta com captura para variável e validação, condição, ação, chamar automação de IA,
transferir para humano, fim), testa num chat simulado ao lado e ativa com um gatilho (palavra-chave
ou primeira mensagem do contato). Na conversa, uma faixa mostra "Bot X ativo nesta conversa ·
Assumir".

**Why this priority**: qualificação estruturada (menus, captura de e-mail) sem programar é o
complemento das automações de IA; depende do motor de eventos e das proteções das US2/US3.

**Independent Test**: montar um bot com menu (1-Preços, 2-Falar com vendedor), pergunta de e-mail
validado, condição e transferência para humano; percorrer no chat simulado e, depois, com
mensagens injetadas no WhatsApp simulado.

**Acceptance Scenarios**:

1. **Given** um chatbot ativo com gatilho "palavra-chave: orçamento", **When** um contato envia
   "Orçamento", **Then** o bot inicia a sessão, envia a mensagem de boas-vindas e o menu.
2. **Given** um menu com opções "1 - Preços" e "2 - Falar com vendedor", **When** o contato
   responde "2" ou "falar com vendedor", **Then** o bot segue o ramo correspondente.
3. **Given** um nó de menu, **When** o contato responde algo que não é opção, **Then** o bot envia
   "Não entendi" (texto configurável) e repete; após o máximo de tentativas (padrão 3) segue o
   caminho "ao esgotar" (padrão: transferir para humano).
4. **Given** uma pergunta "Qual seu e-mail?" com validação de e-mail, **When** o contato responde
   "ana@exemplo.com", **Then** o valor fica na variável `email` da sessão e pode ser usado em
   mensagens (`{email}`) e salvo em campo do lead por um nó de ação.
5. **Given** um nó "transferir para humano", **When** a sessão chega nele, **Then** a sessão
   termina com estado `humano`, a conversa entra em atendimento humano e o operador é notificado.
6. **Given** uma sessão ativa, **When** o contato fica 30 min (configurável por bot) sem
   responder, **Then** a sessão termina com estado `expirada` sem enviar nada.
7. **Given** uma sessão ativa, **When** o operador clica "Assumir" na faixa da conversa ou
   responde à mão, **Then** a sessão termina com estado `humano`.
8. **Given** um chatbot com sessões ativas, **When** o usuário edita e salva o fluxo, **Then** as
   sessões em andamento terminam na versão antiga e novas sessões usam a nova.
9. **Given** o editor do chatbot, **When** o usuário clica "Testar", **Then** um chat simulado ao
   lado permite conversar com o bot, mostrando variáveis e nó atual, sem enviar nada ao WhatsApp.
10. **Given** um fluxo inválido (nó sem saída, opção de menu sem destino), **When** o usuário
    tenta ativar, **Then** os nós com problema ficam destacados com a mensagem do erro.

---

### User Story 5 - IA cria, programa e opera automações pelo MCP (Priority: P2)

Pelo Claude Code ou Claude Desktop, a IA lista e edita funis, etapas e cards, cria fluxos e
chatbots, cria automações de IA escrevendo os arquivos do projeto, compila, testa em simulação,
ativa, desativa e consulta execuções e logs.

**Why this priority**: o MVP garante que a IA tem o mesmo poder que o operador; automações não
podem ser exceção.

**Independent Test**: via cliente MCP contra o motor com WhatsApp simulado: criar automação de IA,
escrever `index.ts`, compilar, testar em simulação, ativar, injetar mensagem e ler a execução.

**Acceptance Scenarios**:

1. **Given** o app aberto, **When** a IA chama `criar_automacao_ia` com nome e modelo "em branco",
   depois `escrever_arquivo_automacao` com um `index.ts`, **Then** os arquivos ficam na pasta do
   projeto e aparecem no editor do app.
2. **Given** código com erro, **When** a IA chama `compilar_automacao`, **Then** recebe a lista de
   erros com arquivo, linha, coluna e mensagem.
3. **Given** código válido, **When** a IA chama `testar_automacao` com uma mensagem simulada,
   **Then** recebe log, ações que seriam feitas e resultado, sem nada enviado.
4. **Given** a automação testada, **When** a IA chama `ativar_automacao`, **Then** ela passa a
   responder mensagens reais; `listar_execucoes` e `ver_execucao` mostram o resultado.
5. **Given** um funil, **When** a IA chama `mover_card_funil` com telefone e etapa, **Then** o lead
   é movido (criado se preciso) e o histórico registra origem `mcp`.
6. **Given** a IA, **When** ela pede os segredos, **Then** recebe apenas os nomes configurados,
   nunca os valores, e não existe ferramenta para definir segredos.

---

### Edge Cases

- **Dois gatilhos para a mesma mensagem**: todas as automações que casam executam, em ordem de
  `prioridade` (menor número primeiro; empate: mais antiga primeiro). Sessão de chatbot ativa tem
  precedência e bloqueia gatilhos de mensagem das demais (ver Clarifications).
- **Automações que se disparam em cadeia sem mensagens** (etiqueta → etapa → etiqueta…): a cadeia
  para no 5º nível e uma automação nunca é redisparada por efeito dela mesma (FR-023).
- **Bot conversa com outro bot**: o anti-loop corta na 11ª mensagem automática em 10 min e pausa a
  conversa por 60 min.
- **Mensagem do próprio número**: nunca dispara automação (inclusive mensagens das próprias
  automações e dos disparos).
- **Operador responde manualmente**: pausa de 30 min, sessão de bot encerrada como `humano`,
  esperas da conversa abortadas ao vencer.
- **App fechado durante "aguardar" ou "sem resposta"**: vencidas há menos de 24 h executam ao
  reabrir; mais antigas viram `abortada` ("expirada").
- **App ou motor cai no meio de uma execução** (fora de uma espera): ao reabrir, a execução fica
  `abortada` (motivo "interrompida") e nenhum passo é repetido, para nunca reenviar mensagem.
- **Agendamento perdido com o app fechado**: executa uma única vez na reabertura se venceu há
  menos de 24 h (nunca recupera várias ocorrências).
- **Automação de IA lança erro ou excede o tempo**: execução `erro` com stack; 5 erros seguidos
  desativam a automação e notificam. Uma execução `ok` zera a contagem.
- **Erro de compilação**: a automação não pode ser ativada; se o código de uma automação ativa for
  alterado e passar a ter erro, ela continua rodando a última versão compilada com sucesso e o
  editor avisa "Rodando a versão anterior — corrija os erros".
- **Chave da Anthropic ausente/inválida**: `ctx.ia` falha com "Configure a chave da Anthropic em
  Ajustes → IA".
- **Claude API 429/5xx/sobrecarga**: até 3 tentativas com espera crescente, dentro do tempo limite
  da execução; depois, erro claro.
- **Conta desconectada**: ações de envio falham com motivo "Conta desconectada"; esperas aguardam
  a reconexão até vencer o prazo de 24 h.
- **Contato em disparo e em bot ao mesmo tempo**: permitido; o anti-loop limita.
- **Automação editada com sessões ativas**: sessões continuam na versão guardada na sessão.
- **Mensagem em grupo**: ignorada, salvo `incluir_grupos`.
- **Envio automático que inicia conversa nova**: sujeito ao limite de 20 por hora por conta; acima
  disso a ação falha com "Muitos primeiros contatos automáticos. Use um disparo."
- **Etapa ou funil excluído com automação apontando para ele**: a automação continua ativa; a ação
  falha com "Etapa não encontrada" e o editor mostra o aviso.
- **Etiqueta, template ou chatbot referenciado e depois excluído**: mesma regra (falha da ação,
  aviso no editor).
- **Várias execuções para a mesma conversa**: executadas uma por vez, em fila, por automação e
  conversa; a fila de uma conversa não bloqueia outras.
- **Muitas automações de IA acionadas ao mesmo tempo**: no máximo 4 em execução simultânea (padrão);
  as demais aguardam na fila e o tempo limite só começa quando a execução começa.
- **Arquivo do projeto alterado fora do app enquanto o editor está aberto**: o editor avisa
  "Arquivo alterado fora do app" e oferece recarregar; salvar por cima exige confirmação.
- **Import proibido no código** (`fs`, `child_process`, `net`, pacotes npm): erro de compilação
  "Módulo não permitido: fs".
- **Log de execução muito grande**: truncado em 64 KB com a marca "[log truncado]".
- **Chatbot recebe mídia numa pergunta de texto**: conta como resposta inválida (tentativa).
- **Lead importado sem conversa e ação "enviar"**: a automação usa a conta configurada em "conta
  de envio"; sem ela (e com mais de uma conta conectada), a ação falha com motivo claro.

## Requirements *(mandatory)*

### Functional Requirements

#### Funil de vendas

- **FR-001**: Usuários DEVEM poder criar, renomear, reordenar e excluir funis (nome 1–60,
  único) e, em cada funil, etapas ordenadas com nome (1–40, único no funil) e cor.
- **FR-002**: Cada lead DEVE ocupar no máximo uma etapa por funil; adicionar um lead que já está
  no funil a outra etapa DEVE movê-lo, nunca duplicá-lo. Um lead pode estar em vários funis.
- **FR-003**: Adicionar ao funil um contato individual sem lead DEVE criar o lead (origem
  `contatos`) e ligá-lo ao contato. Grupos NÃO PODEM entrar em funil.
- **FR-004**: O sistema DEVE oferecer quadro Kanban por funil com colunas por etapa, contagem por
  coluna, cartões com nome, telefone, etiquetas e tempo na etapa, arrastar e soltar e alternativa
  por teclado ("Mover para…").
- **FR-005**: Toda entrada, movimentação e saída de funil DEVE gravar histórico (etapa de origem,
  etapa de destino, data/hora, origem: `app`, `mcp` ou `automacao:<id>`), visível no cartão.
- **FR-006**: Excluir uma etapa com cartões DEVE exigir destino (outra etapa ou remover do funil);
  excluir um funil DEVE pedir confirmação e remover posições e histórico daquele funil.
- **FR-007**: Clicar num cartão DEVE abrir a conversa mais recente do telefone (qualquer conta) ou
  oferecer "Iniciar conversa" quando não houver.
- **FR-008**: O Kanban DEVE suportar ao menos 5.000 cartões por funil, abrindo em até 1 s, com
  rolagem sem travar e busca
  por nome/telefone.

#### Motor de eventos e gatilhos (comum aos três tipos)

- **FR-010**: Cada automação DEVE ter: nome (1–80), tipo (`fluxo` | `chatbot` | `ia`), ativa,
  contas em que vale (lista ou todas), `incluir_grupos` (padrão falso), prioridade (1–1000, padrão
  100), conta de envio opcional, gatilhos e limites anti-loop próprios opcionais.
- **FR-011**: O sistema DEVE suportar os gatilhos: (a) mensagem recebida, com filtros opcionais de
  conta, tipo de conversa (individual/grupo), texto contém, regex e "primeira mensagem do
  contato"; (b) palavra-chave (lista; comparação sem diferenciar maiúsculas e acentos, mensagem
  inteira ou palavra isolada); (c) lead importado (somente leads novos); (d) etiqueta adicionada
  ou removida (etiqueta específica); (e) lead entrou numa etapa do funil; (f) destinatário de
  disparo respondeu (disparo específico ou qualquer); (g) sem resposta há X tempo após uma mensagem
  enviada pelo próprio número (1 min a 30 dias, filtro opcional por origem da mensagem: qualquer,
  disparo ou automação); (h) agendamento por expressão cron de 5 campos ou intervalo (mínimo 1
  min), no fuso local; (i) manual (botão "Executar" e ferramenta MCP), com contato/conversa opcional.
- **FR-012**: Gatilhos NUNCA PODEM disparar com mensagens enviadas pelo próprio número, incluindo
  mensagens das próprias automações e dos disparos.
- **FR-013**: Mensagens de grupo DEVEM ser ignoradas por automações sem `incluir_grupos`; envios
  automáticos a grupos também exigem `incluir_grupos`.
- **FR-014**: Quando várias automações casam com o mesmo evento, TODAS DEVEM executar, em ordem de
  prioridade (menor primeiro; empate pela criação mais antiga). Com sessão de chatbot ativa na
  conversa, a mensagem DEVE ir só para a sessão (ver Clarifications).
- **FR-015**: Execuções da mesma automação para a mesma conversa DEVEM rodar uma por vez, em fila
  na ordem dos eventos; filas de conversas diferentes são independentes.
- **FR-016**: O gatilho "sem resposta há X" DEVE ser reprogramado a cada nova mensagem enviada na
  conversa e cancelado quando o contato envia mensagem; dispara no máximo uma vez por mensagem
  enviada.
- **FR-017**: Condições DEVEM suportar: etiqueta tem/não tem; lead está/não está em etapa X;
  campo do lead (igual, diferente, contém, existe, não existe, maior, menor); horário e dias da
  semana (faixa HH:MM–HH:MM, no fuso local); texto da mensagem (contém, igual, regex); conta; com
  combinação "todas" (E) ou "alguma" (OU). Condições não atendidas terminam a execução como `ok`
  com o registro "condições não atendidas" e sem ações.
- **FR-018**: Ações DEVEM suportar: enviar texto com variáveis (`{nome}`, `{telefone}`, campos do
  lead, variáveis da execução/sessão) ou template (com anexo); aguardar (1 min a 30 dias);
  adicionar/remover etiqueta; mover de etapa (e remover do funil); atualizar nota do contato
  (substituir ou acrescentar) ou campo do lead; iniciar chatbot; executar automação de IA
  (entrada configurável, resultado salvo opcionalmente numa variável); adicionar o lead a um
  disparo existente não finalizado; pausar automações na conversa (duração opcional); notificar o
  operador (notificação do macOS com título e texto).
- **FR-019**: Variável sem valor numa mensagem automática DEVE usar o valor padrão configurado na
  ação; sem padrão, a ação DEVE falhar com "Variável {x} sem valor" sem enviar nada.
- **FR-020**: Ações que dependem de conversa num evento sem conversa (lead importado, agendamento)
  DEVEM usar a conversa existente do telefone na conta de envio ou abri-la; sem conta de envio
  resolvível, a ação falha com motivo.
- **FR-021**: Referências quebradas (etapa, etiqueta, template, chatbot, disparo ou automação
  excluídos) DEVEM fazer a ação falhar com motivo e ser sinalizadas no editor da automação.
- **FR-022**: O botão "Executar" e a ferramenta MCP DEVEM executar uma automação para um contato ou
  conversa escolhido (ou sem alvo, para automações que não precisam), ignorando gatilhos e
  condições de gatilho, mas respeitando pausa, anti-loop e permissões.
- **FR-023**: Eventos causados por ações automáticas (ex.: etiqueta adicionada por um fluxo, que
  dispara outro fluxo) DEVEM carregar a cadeia de origem; uma automação NÃO PODE ser disparada por
  um evento causado por ela mesma na mesma cadeia, e cadeias com mais de 5 níveis DEVEM ser
  interrompidas com registro "cadeia de automações longa demais" na execução que a geraria.

#### Segurança anti-loop e atendimento humano

- **FR-030**: Todo envio automático DEVE passar por um único ponto de verificação que aplica, na
  ordem: pausa da conversa, permissão de grupo, limite anti-loop e limite de primeiros contatos.
  Envio bloqueado não sai e a ação registra o motivo.
- **FR-031**: Limite anti-loop: no máximo N mensagens automáticas por conversa numa janela
  deslizante de M minutos (padrão 10 em 10 min, global configurável; a automação pode ter limite
  mais restritivo). Ao estourar, a conversa DEVE entrar em pausa `anti_loop` por 60 min
  (configurável) e o operador DEVE ser notificado.
- **FR-032**: Mensagem manual do operador (definição nas Clarifications) DEVE criar ou renovar a
  pausa `humano` da conversa por 30 min (configurável) e encerrar a sessão de chatbot ativa com
  estado `humano`.
- **FR-033**: A conversa DEVE mostrar faixa de estado: "Bot X ativo nesta conversa · Assumir",
  "Atendimento humano até HH:MM · Devolver às automações" ou "Automações pausadas (anti-loop) até
  HH:MM · Retomar". "Assumir" cria pausa `humano` sem prazo; "Devolver"/"Retomar" removem a pausa.
- **FR-034**: Durante a pausa, nenhuma automação PODE agir na conversa: gatilhos da conversa não
  executam, esperas que vencem são `abortada` (motivo "conversa pausada") e envios são bloqueados.
- **FR-035**: Mensagens automáticas que iniciam conversa (sem nenhuma mensagem anterior no
  histórico local) DEVEM respeitar o limite de 20 por hora por conta (configurável).
- **FR-036**: Toda mensagem automática DEVE ser identificável como tal (automação de origem) no
  histórico da conversa e na API.

#### Esperas, agendamentos e ciclo de vida

- **FR-040**: Esperas ("aguardar", "sem resposta há X") e próximas ocorrências de agendamento DEVEM
  ser persistidas antes de a execução ceder a vez, e sobreviver a fechamento do app, queda do
  motor e sono do Mac.
- **FR-041**: Na abertura, esperas e agendamentos vencidos há menos de 24 h DEVEM executar (cada
  agendamento no máximo uma vez); vencidos há mais DEVEM ficar `abortada` com motivo "expirada".
- **FR-042**: Com a conta desconectada, ações de envio DEVEM falhar com motivo e esperas DEVEM
  aguardar a reconexão até o limite de 24 h depois do vencimento.
- **FR-043**: Automações NÃO PODEM executar com o app fechado nem manter o app aberto; fechar o app
  DEVE encerrar todas as execuções (as em andamento ficam `abortada`, motivo "app fechado") e todos
  os processos das automações de IA. Enquanto o app permanece na barra de menu terminando um
  disparo ativo (regra do MVP), ele ainda está aberto e as automações continuam funcionando; ao
  sair de vez, vale a regra acima.

#### Execuções e observabilidade

- **FR-050**: Cada execução DEVE registrar: automação, gatilho, conversa/lead, início, fim,
  duração, estado (`na_fila`, `rodando`, `aguardando`, `ok`, `erro`, `simulacao`, `abortada`), ações realizadas
  (tipo, alvo, resultado), log (até 64 KB, truncado com aviso), erro com stack (IA), tokens de IA
  (entrada/saída por modelo) e versão da automação.
- **FR-051**: Cada automação DEVE mostrar lista de execuções filtrável por estado, com detalhe;
  a lista de automações mostra contagem de execuções ok/erro nas últimas 24 h e duração média.
- **FR-052**: O sistema DEVE manter as 500 execuções mais recentes por automação e apagar as mais
  antigas automaticamente.
- **FR-053**: O arquivo de log do motor NÃO PODE conter texto de mensagens nem o conteúdo de
  `ctx.log` por padrão (vale o modo de depuração do MVP); o log da execução fica no banco local.
- **FR-054**: Mudanças de estado de execução, sessão de chatbot, pausa de conversa e movimentações
  de funil DEVEM ser publicadas em tempo real para a janela e ferramentas.

#### Chatbots

- **FR-060**: O editor de chatbot DEVE ser um canvas de nós conectáveis dos tipos: início, enviar
  mensagem (texto com variáveis ou template), menu de opções (número ou texto da opção), pergunta
  com captura para variável e validação opcional (e-mail, número, telefone, regex), condição
  (ramos por regra, com "senão"), ação (qualquer ação do FR-018 exceto "aguardar" e "iniciar
  chatbot"), chamar automação de IA (resultado como resposta enviada ou em variável), transferir
  para humano e fim.
- **FR-061**: O chatbot DEVE ser validado antes de ativar: exatamente um início, todo nó
  alcançável, toda saída ligada, variáveis usadas definidas antes; erros destacados nos nós.
- **FR-062**: Cada conversa DEVE ter no máximo uma sessão de chatbot ativa, com nó atual,
  variáveis, tentativas, expiração por inatividade (padrão 30 min, configurável por bot) e a
  definição do bot congelada no início da sessão.
- **FR-063**: Resposta inválida em menu ou pergunta DEVE enviar a mensagem "não entendi"
  (configurável) e repetir, até o máximo de tentativas (padrão 3); ao esgotar, seguir a saída "ao
  esgotar" (padrão: transferir para humano).
- **FR-064**: Estados finais da sessão: `concluida` (nó fim), `humano` (transferência, "Assumir" ou
  resposta manual), `expirada` (inatividade), `abortada` (erro, anti-loop, bot excluído ou app
  fechado com sessão já vencida).
- **FR-065**: Gatilhos de chatbot: palavra-chave, primeira mensagem do contato, mensagem recebida
  com filtros, manual e ação "iniciar chatbot" de outro fluxo.
- **FR-066**: "Testar" DEVE abrir um chat simulado ao lado do canvas, mostrando o nó atual e as
  variáveis, sem enviar nada nem alterar dados.

#### Automações de IA programáveis

- **FR-070**: Cada automação de IA DEVE ser um projeto numa pasta própria dentro da pasta de dados,
  com `automacao.json` (manifesto) e arquivos `.ts` (e `.json`, `.md`, `.txt` auxiliares), até 50
  arquivos de até 1 MB cada; caminhos só dentro da pasta.
- **FR-071**: O código DEVE exportar, por padrão, uma definição com handlers opcionais
  `aoReceberMensagem`, `aoAgendar` e `aoExecutar`, criada por `definirAutomacao` da API da
  automação; o handler chamado depende do gatilho (mensagem → `aoReceberMensagem`; agendamento e
  `ctx.agendar` → `aoAgendar`; manual, fluxo, bot e MCP → `aoExecutar` com entrada).
- **FR-072**: A API tipada `ctx` DEVE oferecer: conversa atual (histórico paginado, contato,
  lead); responder/enviar texto, mídia (arquivo existente) e reação; etiquetas (listar, adicionar,
  remover); funil (posição, mover, remover); leads (obter, atualizar campos, buscar por telefone);
  memória chave-valor persistente isolada por automação, com escopo global ou por contato (sem
  permissão); IA
  (`gerar`, `classificar`, `extrair` com esquema) via Claude; HTTP (`ctx.http.fetch`); log;
  agendar uma chamada futura (`ctx.agendar`); segredos declarados; e informações da execução
  (gatilho, simulação, ids).
- **FR-073**: Cada chamada do `ctx` DEVE ser validada contra as permissões declaradas no manifesto;
  sem a permissão, a chamada falha com "Permissão '<p>' não declarada em automacao.json". Segredos
  só são entregues se listados no manifesto.
- **FR-074**: Cada automação de IA DEVE rodar em processo próprio, separado do app, do motor e das
  outras automações, sem acesso ao token da API local, ao banco de dados ou a arquivos, e se
  comunicar apenas pela API `ctx`.
- **FR-075**: Limites por automação: tempo por execução (padrão 60 s, 5–300 s), memória do processo
  (padrão 256 MB), uma execução por conversa por vez com fila, no máximo 4 processos de automação
  vivos ao mesmo tempo (padrão) e encerramento do processo após 5 min ocioso.
- **FR-076**: Estourar tempo ou memória, lançar erro ou o processo morrer DEVE marcar a execução
  como `erro` com a causa, sem afetar outras automações, o motor ou o app. 5 erros seguidos DEVEM
  desativar a automação e notificar o operador.
- **FR-077**: Imports DEVEM ser restritos à API da automação, a arquivos do próprio projeto e a
  uma lista de módulos Node sem acesso a arquivos, processos ou rede (ex.: `node:crypto`,
  `node:util`, `node:url`, `node:buffer`, `node:events`, `node:timers/promises`, `node:path/posix`);
  qualquer outro import é erro de compilação. Rede só por `ctx.http.fetch` com permissão `rede`.
- **FR-078**: O sistema DEVE documentar, no editor e na documentação da API, que o isolamento
  protege contra erros e acidentes, não contra código malicioso, pois o código é do próprio
  usuário.
- **FR-079**: O TypeScript DEVE ser compilado pelo próprio ZapDesk (sem Node/npm instalados pelo
  usuário); erros de sintaxe, importação e compilação DEVEM voltar com arquivo, linha, coluna e
  mensagem; a compilação de um projeto típico DEVE levar menos de 1 s.
- **FR-080**: O editor do app DEVE oferecer: árvore de arquivos (criar, renomear, excluir),
  editor de código com realce, autocompletar e verificação de tipos da API da automação, erros
  inline, salvar (Cmd+S), "Testar", "Ativar/Desativar", aba "Execuções" e "Abrir pasta no editor
  externo".
- **FR-081**: "Testar" DEVE executar em modo simulação com: mensagem digitada, mensagem de uma
  conversa real escolhida, ou entrada JSON (para `aoExecutar`); mostrar log, ações que seriam
  feitas, retorno, duração e tokens; oferecer "IA simulada".
- **FR-082**: Em modo simulação, NENHUMA escrita pode ocorrer: envios, etiquetas, funil, leads e
  agendamentos viram registros de "ação simulada"; gravações em memória valem só dentro da
  execução; leituras usam dados reais.
- **FR-083**: Ativar DEVE exigir compilação sem erros; a execução usa sempre a última versão
  compilada com sucesso (identificada por hash do conteúdo). Arquivos alterados fora do app DEVEM
  ser detectados e recompilados antes da execução seguinte.
- **FR-084**: Ao criar, o usuário DEVE escolher um modelo de projeto: "Responder com IA usando
  histórico", "Classificar lead e mover no funil", "Extrair dados (nome, empresa, interesse)" ou
  "Em branco"; os modelos com IA usam prompt conservador (não inventar preços ou promessas;
  transferir para humano quando não souber).
- **FR-085**: `ctx.ia` DEVE usar a Claude API com o modelo padrão configurável (`claude-sonnet-5`;
  opções `claude-opus-5-5` e `claude-haiku-4-5-20251001`), permitir escolher o modelo por chamada,
  repetir até 3 vezes em sobrecarga/limite de taxa/erro de servidor dentro do tempo limite, e
  registrar tokens de entrada e saída por execução.
- **FR-086**: A chave da Anthropic e os demais segredos NUNCA PODEM aparecer no código, em logs,
  em respostas da API local ou do MCP, nem ser acessíveis ao processo da automação de IA (no caso
  da chave da Anthropic); segredos declarados no manifesto são entregues à automação que os
  declarou, apenas em memória.
- **FR-087**: "Abrir pasta no editor externo" DEVE abrir a pasta do projeto no editor padrão do
  sistema, com um `tsconfig.json` e os tipos da API presentes na pasta para autocompletar.
- **FR-088**: Excluir uma automação de IA DEVE pedir confirmação, encerrar o processo, apagar a
  pasta, a memória e as execuções.

#### Ajustes

- **FR-090**: Ajustes → IA DEVE permitir definir, testar e remover a chave da Anthropic (exibida
  mascarada), escolher o modelo padrão e gerenciar segredos com nome (`[A-Z][A-Z0-9_]{0,63}`),
  guardados no chaveiro do macOS; valores nunca são reexibidos.
- **FR-091**: Ajustes → Automações DEVE permitir configurar: limite anti-loop (mensagens e janela),
  duração da pausa anti-loop, duração da pausa humana, limite de primeiros contatos por hora,
  tempo limite padrão de IA, processos simultâneos e tempo de ociosidade; e um interruptor geral
  "Pausar todas as automações".

#### MCP

- **FR-100**: O MCP DEVE expor ferramentas para: listar/criar/editar/excluir funis e etapas;
  listar cards, adicionar/mover/remover lead do funil e ver histórico; listar/criar/editar/excluir
  automações de fluxo e chatbots (definição completa em JSON); criar automação de IA, listar, ler,
  escrever, renomear e excluir arquivos do projeto, compilar, testar em simulação; ativar,
  desativar e executar manualmente qualquer automação; listar e ver execuções com logs; ver e
  remover pausas de conversa; listar nomes de segredos; obter os tipos da API da automação.
- **FR-101**: Não PODE existir ferramenta MCP para definir ou ler valores de segredos.
- **FR-102**: Toda ferramenta nova DEVE ter o mesmo comportamento e validações da interface (é um
  cliente fino da API local), com saída estruturada e resumo em português.

#### Não funcionais e compatibilidade

- **FR-110**: Sem automações de IA em execução, o motor ocioso DEVE continuar abaixo de 80 MB de
  RAM e pronto em menos de 2 s; nenhuma automação ativa pode atrasar o "pronto".
- **FR-111**: Todos os critérios do MVP DEVEM continuar passando.
- **FR-112**: Toda a interface e as mensagens de erro DEVEM estar em português do Brasil.
- **FR-113**: Nenhuma conexão com serviço de IA PODE ocorrer sem uma automação de IA ativa (ou um
  teste pedido pelo usuário) que chame `ctx.ia`, e sem a chave configurada pelo usuário.

### Key Entities *(include if feature involves data)*

- **Funil**: nome, ordem; tem etapas.
- **Etapa**: funil, nome, cor, ordem.
- **Posição no funil (card)**: lead, funil, etapa, desde quando; única por (lead, funil).
- **Histórico do funil**: lead, funil, etapa de origem e destino, quando, origem da mudança.
- **Automação**: nome, tipo, ativa, contas, incluir grupos, prioridade, conta de envio, gatilhos,
  condições, definição (fluxo ou grafo do bot), limites próprios, versão; para IA: pasta,
  manifesto, hash e erros da última compilação, erros seguidos.
- **Sessão de chatbot**: automação, conversa, nó atual, variáveis, tentativas, expira em, estado,
  definição congelada.
- **Execução**: automação, gatilho, conversa/lead, início/fim, estado, ações, log, erro, tokens,
  simulação.
- **Espera agendada**: execução ou automação, tipo (aguardar, sem resposta, agendamento,
  `ctx.agendar`), retomar em, passo, dados.
- **Pausa de conversa**: conversa, até (ou sem prazo), motivo.
- **Memória da automação**: automação, escopo (global ou contato), chave, valor.
- **Segredo**: nome, valor (somente no chaveiro do macOS e na memória do motor).
- **Mensagem (existente)**: ganha a automação de origem, quando automática.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: O usuário cria um funil com 4 etapas e move um cartão em menos de 1 minuto; o
  histórico registra 100% das movimentações (app, MCP e automações).
- **SC-002**: O fluxo "lead respondeu disparo → etiqueta 'quente' → mover para 'Qualificando' →
  iniciar bot" completa de ponta a ponta em menos de 3 s após a resposta, com o WhatsApp simulado.
- **SC-003**: Follow-ups "sem resposta há X" saem no minuto previsto (tolerância de 1 min) e
  nenhum sai em duplicidade, inclusive com o app fechado e reaberto durante a espera.
- **SC-004**: Em 100% dos testes de loop (bot contra bot), no máximo 10 mensagens automáticas saem
  por conversa em 10 min.
- **SC-005**: Em 100% dos casos de resposta manual do operador, nenhuma mensagem automática sai na
  conversa durante a pausa.
- **SC-006**: Uma automação de IA criada pelo modelo "Responder com IA" responde uma mensagem
  real em até 10 s após a chegada (excluído o tempo da Claude API), com o processo já frio.
- **SC-007**: Uma automação com erro ou laço infinito não aumenta em mais de 1 s o tempo de
  resposta de outra automação ativa e é interrompida em até 60 s (+2 s).
- **SC-008**: Uma chamada sem permissão declarada falha em 100% dos casos, sem efeito colateral.
- **SC-009**: A IA, só pelo MCP, cria, programa, compila, testa e ativa uma automação de IA sem
  intervenção do usuário.
- **SC-010**: Ao fechar o app, nenhum processo do ZapDesk (inclusive de automações) sobra em até
  5 s.
- **SC-011**: Com automações de fluxo e chatbots ativos e nenhuma automação de IA rodando, o motor
  ocioso fica abaixo de 80 MB de RAM e pronto em menos de 2 s.
- **SC-012**: Erros de compilação aparecem no editor em até 2 s após parar de digitar.
- **SC-013**: O chatbot de exemplo (menu, e-mail validado, condição, transferência) é percorrido
  corretamente em 100% dos caminhos testados, no chat simulado e com mensagens injetadas.

## Assumptions

- Usuário único (Gabriel, operador) no Mac; a IA via MCP tem o mesmo poder do operador, exceto
  definir segredos.
- O código das automações de IA é do próprio usuário (ou escrito pela IA a pedido dele); o
  isolamento protege contra erro, não contra código malicioso (documentado).
- A chave da Anthropic é do usuário; custo das chamadas é dele e fica visível por execução (tokens).
- Pacotes npm arbitrários, modelos locais com helper próprio, transcrição de áudio e visão nos
  helpers de IA, marketplace, métricas de conversão, versionamento e teste A/B ficam fora da v1
  (`docs/features/automacoes.md` §3.2–3.3).
- Execução com o app fechado está fora do escopo; o ciclo de vida do MVP prevalece (fechar o app
  desliga tudo, exceto o disparo ativo já previsto no MVP; automações não mantêm o app aberto).
- Relógio e fuso: horário local do Mac para condições de horário e agendamentos.
- Os padrões numéricos (anti-loop 10/10 min, pausa humana 30 min, pausa anti-loop 60 min, 24 h de
  validade de esperas, 60 s de tempo limite, 256 MB, 4 processos, 5 min de ociosidade, 500
  execuções guardadas, 20 primeiros contatos por hora, 3 tentativas de bot, 30 min de inatividade
  de bot) são configuráveis e devem ser revisados pelo usuário (§13 do documento).
- Segredos são definidos apenas pela janela do app; MCP e API só listam nomes.
- Risco tratado: o anti-loop também conta mensagens de chatbots; com o padrão original do
  documento (5 em 10 min) um bot longo (boas-vindas, menu, "não entendi", perguntas) estouraria
  em conversas legítimas. Decisão do orquestrador: padrão elevado para 10 em 10 min (ver
  Clarifications); o usuário pode ajustar em Ajustes.
- Verificação de tipos TypeScript acontece no editor do app; a compilação via MCP informa erros de
  sintaxe, importação e compilação.
- Reutiliza do MVP: etiquetas, notas, templates, leads, disparos, conversas, WhatsApp simulado,
  relógio controlável e o padrão de notificações do macOS.
