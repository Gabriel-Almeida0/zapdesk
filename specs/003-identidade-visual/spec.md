# Especificação da Feature: Identidade visual — o app com a cara da landing

**Branch da feature**: `003-identidade-visual` (sem branch git; o repositório ainda não tem commits)

**Criada em**: 2026-10-05

**Status**: Rascunho

**Entrada**: Descrição do usuário: "o estilo do app deve ficar como na landing page". O app
desktop hoje imita o WhatsApp Desktop (verde, bolhas verdes, fonte do sistema, balão como ícone) e
deve adotar a identidade visual da landing open source do ZapDesk (projeto `zapdesk-site`),
chamada lá de **"mesa de operação"**.

## Visão geral

A landing do ZapDesk já definiu a identidade do produto: papel quente no tema claro e grafite no
escuro, **um único âmbar** como sinal (botões, lâmpadas de "ativo", seleção), coral e ciano só para
estados (falhou / positivo), **nenhum verde**, linhas finas no lugar de sombras, rótulos em fonte
mono, "cabos" e "terminais" redondos nos diagramas, três famílias tipográficas (títulos, texto,
rótulos/código) e o logotipo da **tecla âmbar com um "Z" em raio**. Os mockups da landing desenham
as telas reais do app nessa linguagem — eles são o desenho-alvo.

Esta feature troca **só a aparência** do app: cores, tipografia, linhas, raios, ícone do app, ícone
da barra de menu e logotipo. Nenhum fluxo, texto, comportamento, atalho ou disposição funcional
muda. Claro/escuro continuam seguindo o sistema; uma opção manual em Ajustes é um extra pequeno.

### Glossário

- **Landing**: o site open source do ZapDesk (`zapdesk-site`), fonte da verdade do visual.
- **Mockup**: desenho de uma tela do app feito na landing (conversas, novo disparo, relatório de
  disparo, Kanban, editor de chatbot, terminal MCP). É o desenho-alvo daquela tela.
- **Token**: nome semântico de uma cor, fonte, raio ou sombra (ex.: "fundo", "texto-2",
  "sinal", "coral"). Toda cor da interface vem de um token; os valores mudam por tema.
- **Sinal (âmbar)**: a cor única de destaque. Como **preenchimento** (botão primário, selo de não
  lidas, lâmpada de ativo) é o âmbar claro; como **texto** no tema claro é um âmbar escuro legível.
- **Estados**: positivo (lido, respondeu, concluído, conectado) em **ciano**; falha (falhou,
  erro, banida) em **coral**; em andamento/ativo com a **lâmpada âmbar**; neutro (pendente,
  entregue, pausado, agendado) em texto secundário.
- **Tela escura ("tela")**: superfícies que ficam escuras nos dois temas (editor de código,
  terminal, visualizador de mídia e de status), como na landing.
- **Cabo / terminal**: linha que liga nós (editor de chatbot) e o pequeno círculo onde ela nasce
  ou chega; ativo = âmbar cheio.
- **Captura**: imagem de uma tela do app, gerada automaticamente com dados fictícios, em claro e em
  escuro, para comparar antes/depois.
- **Modo falso**: o app rodando com o WhatsApp simulado (já existente desde a 001), sem conta real
  e com dados numa pasta temporária.

## Clarifications

### Session 2026-10-05

> Clarify pulado por instrução do orquestrador. As decisões abaixo foram tomadas pelo agente a
> partir da landing (`zapdesk-site`: `src/app/globals.css`, `src/components/mockups/`,
> `src/components/ui/Logo.tsx`, capturas em `docs/verificacao/final/`) e do pedido; ficam
> registradas aqui para revisão do usuário.

- Q: O app copia a landing pixel a pixel? → A: Não. Copia o **sistema** (tokens de cor com os
  mesmos valores, tipografia, linhas, raios, estados, logotipo) e usa os **mockups** como desenho
  de cada tela. Medidas, disposição e densidade continuam as do app (é uma ferramenta de trabalho,
  não uma vitrine). Onde o mockup e o app diferem em estrutura, vale a estrutura do app.
- Q: As bolhas de saída ("minhas") continuam verdes? → A: Não. Saída = superfície levemente
  destacada com borda mais forte e canto superior direito reto; entrada = superfície comum com
  borda fina e canto superior esquerdo reto — exatamente como o `ChatMockup`. Nada de verde em
  lugar nenhum.
- Q: Quais cores para os estados de mensagem e de disparo? → A: As da landing: **lido** e
  **respondeu** em ciano; **falhou** em coral; **enviado/entregue/pendente** neutros (texto
  secundário); **enviando / em andamento / ativo** com a lâmpada âmbar; o ícone de "lida" (dois
  vistos) fica ciano, os demais neutros.
- Q: As cores escolhidas pelo usuário para etiquetas e etapas do funil mudam? → A: **Não**: são
  dados do usuário e são preservadas. Elas aparecem só como **ponto/borda colorida** ao lado de
  um texto neutro (como nos chips do mockup), nunca como fundo de texto, para o contraste não
  depender delas. Só a **paleta sugerida** ao criar uma etiqueta/etapa nova passa a oferecer tons
  compatíveis com a identidade (sem o verde do WhatsApp); a opção "Outra cor" continua livre.
- Q: Avatares coloridos por contato continuam? → A: Não: avatar neutro (iniciais sobre superfície
  com borda fina), como no mockup. Fotos de perfil, quando existirem, continuam aparecendo.
- Q: Nome do remetente em grupos (hoje colorido por pessoa)? → A: Mantém a variação de cor por
  pessoa (ajuda a ler grupos), mas com luminosidade ajustada por tema para passar contraste AA.
- Q: O QR code de conexão muda de cor? → A: Não. Continua preto sobre branco com margem branca
  nos dois temas (leitura pelo celular); só a moldura em volta segue a identidade.
- Q: Editor de código e terminal? → A: Seguem a "tela" da landing (fundo grafite nos dois temas,
  palavras-chave âmbar, strings ciano, números coral, comentários em itálico secundário).
- Q: Tema manual? → A: Sim, porque é pequeno: Ajustes → Aparência com **Sistema** (padrão),
  **Claro** e **Escuro**. A escolha é uma preferência da janela, guardada só neste Mac, e vale na
  hora, sem reiniciar.
- Q: As fontes vêm da internet? → A: Não. As três famílias vão **dentro do app** (só o conjunto de
  caracteres latino, que cobre pt-BR) e funcionam offline; nenhuma requisição a serviço de fontes.
- Q: Ícone da barra de menu? → A: A mesma tecla com o "Z", desenhada em uma cor só (o macOS a
  inverte no modo escuro), legível em 16 pt.
- Q: Como provar que "nada mudou além do visual"? → A: Capturas de todas as telas antes e depois
  com os mesmos dados fictícios, testes existentes passando e revisão de que as mudanças em
  componentes se limitam a marcação decorativa (classes, ícone, logotipo).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Conversar no app com a identidade da landing (Priority: P1)

O usuário abre o ZapDesk e encontra o mesmo produto que viu na landing: trilho lateral, lista de
conversas e conversa aberta com papel quente (ou grafite, no escuro), âmbar como único destaque,
bolhas sem verde, horários e rótulos em mono, chips de etiqueta com ponto colorido, títulos e
textos nas fontes da landing. Tudo funciona exatamente como antes.

**Why this priority**: conversas são a tela em que o usuário passa o dia; é a primeira impressão e
a tela-símbolo do mockup principal da landing.

**Independent Test**: abrir o app em modo falso com dados fictícios, capturar a tela de conversas
com uma conversa aberta e o painel do contato, em claro e em escuro, e comparar lado a lado com o
`ChatMockup` da landing; enviar uma mensagem, anexar, gravar áudio e abrir o menu de uma mensagem
para conferir que tudo segue funcionando.

**Acceptance Scenarios**:

1. **Given** o app em tema claro, **When** a tela de conversas abre, **Then** o fundo é o papel
   quente, as superfícies são brancas/creme com linhas finas, o item ativo da lista tem a faixa
   âmbar à esquerda e nenhum elemento é verde.
2. **Given** uma conversa com mensagens enviadas e recebidas, **When** ela é exibida, **Then** as
   bolhas de saída têm superfície destacada com borda mais forte e as de entrada superfície comum
   com borda fina, sem verde, com hora em mono e o indicador "lida" em ciano.
3. **Given** conversas com mensagens não lidas, **When** a lista é exibida, **Then** o contador de
   não lidas é uma pílula âmbar com número escuro e a hora fica em âmbar legível.
4. **Given** o sistema em modo escuro, **When** o app está aberto, **Then** tudo passa para o
   grafite da landing sem recarregar e sem perder o estado da tela.
5. **Given** qualquer ação da tela (enviar, anexar, gravar, responder, apagar, buscar, filtrar,
   abrir painel do contato), **When** o usuário a executa, **Then** o resultado é idêntico ao de
   antes da feature.

---

### User Story 2 - Todas as telas falam a mesma língua visual (Priority: P1)

Disparos (lista, assistente de novo disparo, relatório), leads, contatos, etiquetas, templates,
status, funis e Kanban, automações (lista, editor de fluxo, editor de chatbot, execuções), editor de
IA, ajustes, conectar conta e telas de carregamento/erro seguem a identidade: o stepper do novo
disparo com o passo atual em âmbar e os feitos em ciano, o relatório com percentual em fonte de
título, barra âmbar + coral e selos de estado da landing, o Kanban com borda superior na cor da
etapa, o editor de chatbot com cabos e terminais redondos, o editor de código na "tela" escura.

**Why this priority**: uma identidade aplicada só em parte parece app quebrado; todas as telas
precisam virar juntas para a entrega fazer sentido.

**Independent Test**: capturar cada tela da lista de capturas (quickstart) em claro e escuro e
comparar com o mockup correspondente da landing (`DisparoMockup`, `RelatorioMockup`,
`KanbanMockup`, `ChatbotMockup`, `TerminalMockup`) ou, sem mockup, com o vocabulário comum (botões,
campos, cartões, selos); percorrer o fluxo de criar um disparo e de editar um chatbot.

**Acceptance Scenarios**:

1. **Given** o assistente de novo disparo no passo 3, **When** ele é exibido, **Then** os passos
   feitos aparecem com visto em ciano, o atual com número sobre âmbar e sublinhado âmbar, os
   futuros neutros, e os grupos de campos usam legendas e bordas finas como no mockup.
2. **Given** um disparo em andamento com falhas, **When** o relatório abre, **Then** o percentual
   aparece em fonte de título, a barra mostra a parte enviada em âmbar e a falha em coral, os
   contadores "Lidos/Responderam" ficam em ciano e "Falharam" em coral, e cada destinatário tem um
   selo de estado com a cor da landing.
3. **Given** um funil com etapas coloridas pelo usuário, **When** o Kanban abre, **Then** cada
   coluna tem a borda superior e o ponto na cor escolhida pelo usuário, cartões com linhas finas e
   telefone em mono, e as cores escolhidas não mudaram.
4. **Given** um chatbot com nós ligados, **When** o editor abre, **Then** os nós são cartões com
   cabeçalho mono, as ligações são cabos finos e os pontos de conexão são terminais redondos; o
   caminho selecionado/ativo fica em âmbar.
5. **Given** o editor de IA, **When** um arquivo `.ts` é aberto, **Then** o editor usa a tela
   escura com o realce de código da landing nos dois temas, e erros continuam sublinhados.
6. **Given** qualquer tela, **When** se procura uma cor fora do conjunto de tokens, **Then** não
   há nenhuma (exceto as cores que o usuário escolheu para etiquetas/etapas e o QR code).

---

### User Story 3 - Reconhecer o ZapDesk pelo logotipo (Priority: P2)

O ícone do app no Dock, no Finder e no Launchpad passa a ser a tecla âmbar com o "Z" em raio da
landing; o ícone na barra de menu (quando um disparo segura o app aberto) é a mesma tecla em uma
cor só; a tela de carregamento mostra o logotipo da landing.

**Why this priority**: reforça que app e site são o mesmo produto, mas não bloqueia o uso.

**Independent Test**: empacotar o app, abrir o `.app` e conferir o ícone no Dock e no Finder; com
um disparo ativo, fechar a janela e conferir o ícone da barra de menu em tema claro e escuro do
macOS; abrir o app e ver o logotipo na tela "Ligando o WhatsApp…".

**Acceptance Scenarios**:

1. **Given** o app empacotado, **When** aparece no Dock, **Then** o ícone é a tecla âmbar com
   "Z" escuro, com a margem padrão dos ícones do macOS, sem balão verde.
2. **Given** um disparo ativo e a janela fechada, **When** o usuário olha a barra de menu, **Then**
   vê a tecla com o "Z" monocromática, nítida em 16 pt, que se inverte sozinha no modo escuro.
3. **Given** o motor subindo, **When** a tela de carregamento aparece, **Then** mostra a tecla da
   landing e o nome "zapdesk" no estilo da marca.

---

### User Story 4 - Ler e navegar com conforto e acessibilidade nos dois temas (Priority: P2)

Quem usa o app o dia todo, inclusive só pelo teclado, enxerga bem: textos com contraste AA nos dois
temas, anel de foco âmbar sempre visível, nada pisca nem se mexe quando o sistema pede menos
movimento, e a troca de fontes não faz a tela "pular" na abertura nem quebra listas e textos.

**Why this priority**: identidade nova não pode piorar a leitura nem a acessibilidade que o app já
tem.

**Independent Test**: rodar a checagem de contraste de todos os pares de cor (claro, escuro e tela
escura); navegar por Tab em conversas, novo disparo, Kanban e ajustes conferindo o foco; ligar
"Reduzir movimento" no macOS e abrir o app; medir deslocamento de layout na abertura.

**Acceptance Scenarios**:

1. **Given** os dois temas, **When** a checagem de contraste roda, **Then** todo par de texto
   permitido tem razão ≥ 4,5:1 (≥ 3:1 para texto grande, ícones, bordas de controle e foco).
2. **Given** qualquer elemento interativo, **When** recebe foco pelo teclado, **Then** mostra um
   anel âmbar (âmbar escuro no tema claro) com ≥ 3:1 contra o fundo ao redor.
3. **Given** "Reduzir movimento" ligado, **When** o app abre e telas carregam, **Then** não há
   animações de brilho, deslizamento ou barra indeterminada animada além do mínimo informativo.
4. **Given** a abertura do app, **When** as fontes carregam, **Then** não há troca visível de
   fonte nem deslocamento do conteúdo; listas longas continuam rolando sem sobreposição de itens.

---

### User Story 5 - Escolher o tema manualmente (Priority: P3)

Em Ajustes → Aparência o usuário escolhe **Sistema** (padrão), **Claro** ou **Escuro**; a troca é
imediata e a escolha é lembrada na próxima abertura.

**Why this priority**: pedido como "se fácil"; o padrão (seguir o sistema) já atende a maioria.

**Independent Test**: com o macOS em claro, escolher "Escuro" em Ajustes → o app fica escuro na
hora; fechar e abrir → continua escuro; voltar a "Sistema" → acompanha o macOS de novo.

**Acceptance Scenarios**:

1. **Given** a preferência "Sistema", **When** o macOS troca de tema, **Then** o app acompanha.
2. **Given** a preferência "Escuro" com o macOS claro, **When** o app abre, **Then** a janela já
   nasce escura, inclusive o editor de código e o editor de chatbot.
3. **Given** uma preferência salva, **When** o usuário volta para "Sistema", **Then** a escolha
   manual é descartada e o app segue o macOS.

---

### Edge Cases

- Etiquetas/etapas com cor escolhida pelo usuário muito clara ou muito escura: a cor aparece só
  como ponto/borda (com contorno fino para não sumir no fundo); o nome fica em texto neutro.
- Etiquetas antigas verdes (cor padrão anterior): continuam verdes — são dados do usuário.
- Fotos de perfil e mídia (imagem, vídeo, figurinha): não recebem filtro; o visualizador de mídia
  e o de status ficam na tela escura nos dois temas.
- QR code: sempre preto sobre branco com margem branca, mesmo no tema escuro.
- Nomes, prévias e etiquetas longas: truncam como hoje; a troca de fonte não pode fazer texto
  vazar de cartões, linhas de lista ou colunas da tabela.
- Listas virtualizadas (conversas, leads, contatos, destinatários): a altura de cada item continua
  a mesma de antes, para a rolagem não sobrepor itens.
- Tamanho mínimo da janela (960×620): nada corta ou quebra em relação ao comportamento atual.
- Troca de tema do sistema com um modal, menu ou o editor de código aberto: tudo acompanha sem
  fechar nem perder conteúdo.
- "Aumentar contraste" do macOS: no mínimo não piora; bordas finas podem ficar mais fortes.
- App aberto pelo usuário durante a entrega: o app instalado não é substituído (o pacote novo fica
  pronto e o usuário é avisado).
- Fonte não carregou (arquivo corrompido): a interface cai para a fonte do sistema sem quebrar.

## Requirements *(mandatory)*

### Functional Requirements

**Sistema visual**

- **FR-001**: O app DEVE usar o mesmo conjunto de tokens de cor da landing, com os mesmos valores
  em claro e em escuro (fundo, fundo-2, superfície, superfície-2, texto, texto-2, linha,
  linha-forte, sinal, sinal-hover, sinal-texto, coral, ciano, tinta, borda do primário, foco e os
  tokens da tela escura), acrescentando só tokens derivados necessários ao app (ex.: fundos suaves
  de aviso/erro/informação, sobreposição de modal), documentados junto dos demais.
- **FR-002**: Nenhuma cor da interface PODE ser definida fora do arquivo de tokens; toda regra de
  estilo e todo componente DEVEM referenciar tokens. Exceções explícitas: cores escolhidas pelo
  usuário (etiquetas, etapas), a paleta sugerida dessas cores, o QR code e o fundo da janela
  nativa antes do carregamento.
- **FR-003**: O app NÃO PODE exibir verde em nenhum elemento próprio (botões, bolhas, seleção,
  ícone, estados, links, interruptores).
- **FR-004**: Os nomes de tokens antigos do app DEVEM continuar funcionando durante a transição
  (apontando para os novos valores), e ao fim da feature nenhum estilo PODE depender deles.
- **FR-005**: A tipografia DEVE usar as três famílias da landing: uma de **títulos**
  (títulos de tela, nome do app, números de destaque como percentual e contadores), uma de
  **texto** (todo o resto da interface) e uma **mono** (horários, telefones, rótulos, código,
  contadores pequenos, caminhos).
- **FR-006**: As fontes DEVEM vir empacotadas no app, só com o conjunto latino, funcionar sem
  internet e NÃO PODEM provocar requisição de rede.
- **FR-007**: Linhas finas DEVEM substituir as sombras como separação principal; sombras ficam só
  em menus flutuantes, modais e cartões arrastados, no estilo da landing.
- **FR-008**: Estados DEVEM usar as cores da landing: positivo (lido, respondeu, concluído,
  conectado) em ciano; falha (falhou, erro, banida, desconectada) em coral; ativo/em andamento com
  a lâmpada âmbar; neutros (pendente, enviado, entregue, pausado, agendado, cancelado) em texto
  secundário com borda fina.
- **FR-009**: Botão primário DEVE ser âmbar com texto escuro e, no tema claro, contorno escuro fino
  (como na landing); botões secundários com borda; ações destrutivas em coral.

**Telas (desenho-alvo = mockups da landing)**

- **FR-010**: Trilho lateral, lista de conversas, conversa (bolhas, mídia, compositor, menu da
  mensagem) e painel do contato DEVEM seguir o `ChatMockup` (bolhas sem verde, hora em mono, faixa
  âmbar no item ativo, pílula âmbar de não lidas, chips de etiqueta com ponto colorido, avatar
  neutro).
- **FR-011**: Assistente de novo disparo e relatório de disparo DEVEM seguir `DisparoMockup` e
  `RelatorioMockup` (stepper, grupos de campos, percentual em fonte de título, barra âmbar+coral,
  contadores coloridos, selos de destinatário).
- **FR-012**: Funis e Kanban DEVEM seguir o `KanbanMockup` (borda superior e ponto na cor da etapa,
  contagem em mono, cartões com linha fina).
- **FR-013**: Editor de chatbot DEVE seguir o `ChatbotMockup` (cartões de nó com cabeçalho mono e
  ícone, cabos finos, terminais redondos, caminho ativo/selecionado em âmbar, fundo pontilhado).
- **FR-014**: Editor de IA (código), blocos de código e saídas de execução DEVEM usar a tela escura
  e o realce de código da landing (`TerminalMockup`/blocos de código) nos dois temas.
- **FR-015**: Leads, contatos, etiquetas, templates, status, execuções, ajustes, conectar conta,
  estados vazios, faixas de aviso, modais, menus e telas de carregamento/erro DEVEM usar o mesmo
  vocabulário (cartões com linha fina, rótulos mono, botões e campos da landing, selos de estado).
- **FR-016**: A paleta sugerida para novas etiquetas e etapas DEVE oferecer tons compatíveis com a
  identidade e sem o verde do WhatsApp; cores já salvas NÃO PODEM ser alteradas.

**Marca**

- **FR-017**: O ícone do app DEVE ser a tecla âmbar com o "Z" em raio da landing, no grid padrão de
  ícones do macOS.
- **FR-018**: O ícone da barra de menu DEVE ser a mesma tecla em versão monocromática "template"
  (16 e 32 px), nítida e invertida automaticamente pelo macOS.
- **FR-019**: A tela de carregamento DEVE mostrar o logotipo (tecla + "zapdesk") da landing.

**Temas e acessibilidade**

- **FR-020**: Por padrão o tema DEVE seguir o do macOS e acompanhar trocas ao vivo.
- **FR-021**: Ajustes DEVE oferecer Aparência = Sistema / Claro / Escuro, aplicada na hora,
  lembrada entre aberturas, guardada só neste Mac, valendo também para o editor de código e o
  editor de chatbot.
- **FR-022**: Todos os pares de cor permitidos DEVEM atingir contraste WCAG AA nos dois temas e na
  tela escura (texto 4,5:1; texto grande, ícones, bordas de controle e foco 3:1), verificados por
  uma checagem automática que falha quando algum par fica abaixo.
- **FR-023**: Todo elemento focável DEVE mostrar foco visível no âmbar da identidade; nenhum foco
  existente PODE ser removido.
- **FR-024**: Com "Reduzir movimento" ligado, animações decorativas DEVEM ser desligadas.
- **FR-025**: A abertura do app NÃO PODE apresentar troca visível de fonte nem deslocamento de
  layout; alturas de itens de listas virtualizadas NÃO PODEM mudar.

**Escopo e verificação**

- **FR-026**: Nenhum fluxo, texto visível, rótulo acessível, atalho, ordem de foco, rota,
  disposição funcional ou comportamento PODE mudar; mudanças em componentes ficam restritas a
  marcação decorativa (classes, ícones, logotipo), exceto o seletor de tema (FR-021).
- **FR-027**: DEVE existir um jeito automático de capturar todas as telas do app com dados
  fictícios e WhatsApp simulado, em claro e em escuro, sem usar a conta nem os dados reais do
  usuário e sem conflitar com o app instalado aberto.
- **FR-028**: A entrega DEVE gerar o pacote do app e só substituir o app instalado se ele estiver
  fechado; caso contrário, deixa o pacote pronto e avisa.

### Key Entities *(include if feature involves data)*

- **Token visual**: nome semântico + valor no tema claro + valor no tema escuro (ou valor fixo da
  tela escura) + uso permitido (texto, preenchimento, borda, foco).
- **Mapeamento de token antigo**: nome usado hoje no app → token novo equivalente (camada de
  transição).
- **Preferência de tema**: Sistema | Claro | Escuro, por Mac, padrão Sistema.
- **Par de contraste**: cor de primeiro plano + cor de fundo + tipo (texto, grande, interface) +
  mínimo exigido.
- **Captura**: tela + tema + tamanho de janela + rótulo (antes, fundação, bloco, depois).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% das telas da lista de capturas existem em claro e escuro "antes" e "depois", com
  os mesmos dados fictícios, e cada uma passa na revisão lado a lado com o mockup correspondente ou
  com o vocabulário comum.
- **SC-002**: 0 ocorrências de cor literal fora do arquivo de tokens (exceções do FR-002 listadas e
  justificadas) e 0 elementos verdes no app.
- **SC-003**: 100% dos pares de cor permitidos passam AA nos dois temas e na tela escura.
- **SC-004**: 100% dos testes automatizados existentes do app continuam passando sem alteração de
  expectativa (os únicos testes novos cobrem o seletor de tema).
- **SC-005**: 0 requisições de rede para fontes; o app abre e mostra as fontes corretas sem
  internet.
- **SC-006**: Deslocamento de layout acumulado na abertura = 0 e nenhuma troca visível de fonte.
- **SC-007**: Em 100% dos elementos interativos percorridos por Tab nas telas principais
  (conversas, novo disparo, Kanban, editor de chatbot, ajustes) o foco é visível.
- **SC-008**: O usuário reconhece o app como "o mesmo da landing" ao comparar a tela de conversas
  com a primeira dobra da landing (aprovação do dono do produto).
- **SC-009**: A troca de tema (sistema ou manual) acontece em menos de 0,5 s, sem recarregar a tela
  nem perder o que está aberto.

## Assumptions

- A landing (`../zapdesk-site`) está estável; seus tokens e
  mockups atuais são a referência. Se a landing mudar depois, o app não acompanha automaticamente.
- A estrutura das telas do app prevalece sobre a dos mockups quando diferem (os mockups foram
  desenhados a partir do app, mas simplificam).
- Os ícones de interface continuam os mesmos (a landing também usa a mesma família de ícones);
  muda só cor, espessura e tamanho onde o mockup indica.
- A preferência de tema é da janela (como tamanho e posição), não um dado de domínio; por isso não
  passa pelo motor nem pelo MCP.
- O dono do produto aceita o aumento de tamanho do app causado pelas fontes empacotadas (algumas
  centenas de KB).
- Não há usuários além do dono; não é preciso migrar preferências nem avisar sobre a mudança visual.
- O idioma da interface continua só pt-BR; nenhum texto muda.
