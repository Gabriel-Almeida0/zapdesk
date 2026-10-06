# Especificação da Feature: ZapDesk MVP — WhatsApp desktop local com disparo em massa e MCP

**Branch da feature**: `001-zapdesk-mvp`

**Criada em**: 2026-09-27

**Status**: Rascunho

**Entrada**: Descrição do usuário: "ZapDesk: WhatsApp desktop local para macOS com chat completo, multi-conta, disparo em massa com agendamento e relatório, base de leads com deduplicação e servidor MCP; conforme docs/features/zapdesk.md" (entrevista completa em `docs/features/zapdesk.md`, fonte da verdade do produto).

## Visão geral

ZapDesk é um aplicativo desktop para macOS com a experiência do WhatsApp (conversas, grupos, mídia,
status) preparado para disparo em massa a listas de leads e controlável por IA (Claude Code e
Claude Desktop) através de um servidor MCP. Tudo roda na máquina do usuário: abrir o app liga o
WhatsApp local, fechar o app desliga tudo. O usuário é único (Gabriel, operador) e a IA tem o
mesmo poder que ele. Não há login no app.

### Glossário

- **Conta**: um número de WhatsApp conectado ao ZapDesk como aparelho conectado.
- **Motor**: o componente local que mantém as contas conectadas, guarda os dados e executa os
  disparos. A janela do app e o MCP apenas conversam com ele.
- **Lead**: pessoa/telefone na base de prospecção, único por telefone normalizado (E.164).
- **Disparo**: envio em massa de uma mensagem para uma lista de destinatários, por uma conta.
- **Destinatário**: um lead dentro de um disparo, com seu próprio estado de envio.
- **Disparo ativo**: disparo já iniciado em `agendado`, `enviando` ou `fora_da_janela`; disparo
  `pausado`, `rascunho`, `concluido` ou `cancelado` não é ativo (não mantém o app aberto).
- **Janela de envio**: faixa de horário diária em que o disparo pode enviar (ex.: 9h–18h).
- **Template**: mensagem salva (texto com variáveis + anexo opcional) reutilizável.
- **Etiqueta**: marcador colorido aplicado a contatos (estilo CRM).

## Clarifications

### Session 2026-09-27

> Clarificações resolvidas pelo agente sem perguntar ao usuário, usando `docs/features/zapdesk.md`.
> Para as "Questões em aberto" (§13 do documento), foi adotada a proposta escrita lá e registrada
> como **decisão assumida**; onde não havia proposta, a decisão tomada está justificada.

- Q: Ao importar um telefone que já existe com campos novos, o lead existente é atualizado? → A:
  Decisão assumida (proposta §13): preencher apenas campos vazios do lead existente (nome e campos
  extras ausentes); nunca sobrescrever valor já preenchido; `origem` e `importado_em` originais
  são mantidos.
- Q: O disparo deve pausar sozinho após muitas falhas seguidas, e com quantas? → A: Decisão
  assumida (proposta §13): pausar automaticamente após 10 falhas seguidas, valor configurável por
  disparo (0 desliga). Falha por "Sem WhatsApp" não conta para a sequência (é característica do
  lead, não do envio); qualquer envio bem-sucedido zera a contagem.
- Q: Dois disparos da mesma conta podem enviar ao mesmo tempo? → A: Decisão assumida (proposta
  §13): não. Cada conta tem no máximo um disparo em `enviando`/`fora_da_janela`; os demais prontos
  ficam em `agendado` exibidos como "Na fila", em ordem de horário de início e depois de criação.
  Pausar ou concluir o disparo ativo libera a vez para o próximo da fila; retomar um disparo
  pausado enquanto outro da mesma conta está ativo o coloca na fila.
- Q: O sistema deve verificar em lote se os números têm WhatsApp antes do disparo? → A: Decisão
  do agente (§13 sem proposta): não no MVP. A verificação acontece individualmente no momento do
  envio de cada destinatário (e ao iniciar conversa com número novo), preenchendo `tem_whatsapp`
  do lead; verificação em lote fica para v2, pois consultas em massa aumentam o risco de ban.
- Q: O que acontece quando a IA cria um disparo via MCP e algum destinatário não tem valor para
  uma variável da mensagem? → A: Decisão do agente: a ferramenta recusa criar o disparo e retorna
  a lista das linhas incompletas, a menos que a chamada informe `valores_padrao` para as
  variáveis; mesma regra da interface (FR-052).
- Nota: o nome "ZapDesk" (§13) permanece provisório e não afeta a especificação.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Conectar contas e conversar por texto (Priority: P1)

O usuário abre o ZapDesk, conecta uma ou mais contas de WhatsApp escaneando um QR code, vê o
histórico sincronizado e conversa por texto com contatos e grupos, trocando de conta pelo seletor
no topo da lista de conversas. Ao fechar o app, nada fica rodando; ao reabrir, as contas continuam
conectadas sem pedir QR de novo.

**Why this priority**: é a base de tudo — sem conta conectada não há disparo nem MCP útil, e o
app precisa substituir o WhatsApp no dia a dia.

**Independent Test**: abrir o app, conectar uma conta por QR, enviar e receber mensagens de texto
em conversa individual e em grupo, buscar uma mensagem antiga, fechar o app e verificar que nenhum
processo do ZapDesk sobrou; reabrir e verificar que a conta continua conectada.

**Acceptance Scenarios**:

1. **Given** app aberto pela primeira vez e nenhuma conta, **When** o usuário clica em "Conectar
   conta", **Then** um QR code é exibido com a instrução "Escaneie o QR code no seu celular em
   Aparelhos conectados", renovado automaticamente quando expira.
2. **Given** QR escaneado com sucesso, **When** a conexão é aceita, **Then** o app mostra
   "Sincronizando histórico…" e em seguida a lista de conversas com contagem de não lidas.
3. **Given** conta conectada, **When** o usuário escreve um texto e pressiona Enter, **Then** a
   mensagem aparece na conversa com estado pendente → enviada → entregue → lida conforme os
   recibos chegam, e aparece no celular.
4. **Given** duas contas conectadas, **When** o usuário troca a conta no seletor, **Then** a lista
   mostra apenas as conversas da conta escolhida.
5. **Given** conta conectada, **When** o usuário informa um número novo em "Nova conversa",
   **Then** uma conversa é aberta com esse número (normalizado para E.164) e aceita mensagens; se o
   número não tiver WhatsApp, o app avisa "Este número não tem WhatsApp".
6. **Given** mensagens antigas salvas, **When** o usuário busca um termo, **Then** as mensagens
   que contêm o termo são listadas com conversa e data, e clicar leva à mensagem.
7. **Given** app aberto sem disparo ativo, **When** o usuário fecha o app, **Then** nenhum
   processo do ZapDesk continua rodando; ao reabrir, as contas continuam conectadas sem novo QR.
8. **Given** a sessão de uma conta expirou no celular, **When** o app detecta, **Then** a conta
   aparece como "Desconectada" com faixa amarela e botão "Reconectar" (novo QR).

---

### User Story 2 - Montar a base de leads sem duplicar (Priority: P1)

O usuário importa leads de um CSV/XLSX (mapeando colunas), colando números (um por linha) ou
escolhendo contatos/etiquetas, e recebe um relatório de quantos eram válidos, novos, já existentes
(com a data da importação original) e inválidos. A tela de Leads mostra a base com busca, origem,
data de importação e último disparo.

**Why this priority**: a deduplicação é o que impede mandar a mesma mensagem duas vezes para a
mesma pessoa e é pré-requisito do disparo.

**Independent Test**: importar um CSV de 1.000 linhas com duplicados internos, telefones já
existentes na base e telefones inválidos, e conferir que o relatório bate exatamente com o
esperado e que a base não tem telefone repetido.

**Acceptance Scenarios**:

1. **Given** um CSV com colunas `telefone`, `nome`, `empresa`, **When** o usuário importa e mapeia
   `telefone` como telefone, **Then** cada linha válida vira um lead com o telefone em E.164
   (DDI +55 quando ausente), `nome` preenchido e `empresa` guardada como campo extra.
2. **Given** um CSV sem coluna reconhecida como telefone, **When** o usuário tenta importar,
   **Then** a importação fica bloqueada com "Escolha qual coluna tem o telefone."
3. **Given** a base já contém `+5511999990000` importado em 10/09, **When** um novo lote contém
   `(11) 99999-0000`, **Then** nenhum lead novo é criado e o relatório lista o número como "já
   existia" com a data 10/09.
4. **Given** um lote com o mesmo telefone em três linhas, **When** importado, **Then** conta como
   um único lead.
5. **Given** um lote com 12 telefones impossíveis de normalizar, **When** importado, **Then** o
   relatório mostra "12 números inválidos" e lista as linhas rejeitadas.
6. **Given** a base vazia, **When** o usuário abre Leads, **Then** vê "Nenhum lead — importe um
   CSV ou use o MCP".

---

### User Story 3 - Disparar em massa com ritmo, agendamento e relatório (Priority: P1)

O usuário cria um disparo pelo assistente (lista → mensagem → ritmo/agendamento → revisão):
escolhe a conta, a lista (importada, colada, contatos/etiquetas ou leads existentes), escreve a
mensagem com variáveis (ou escolhe um template), anexa arquivo opcional, vê a pré-visualização com
uma linha real, configura intervalo aleatório, limites, pausas, janela e início, e clica
"Iniciar". Acompanha em tempo real, pausa, retoma ou cancela, e exporta o relatório em CSV.
Se fechar a janela com disparo ativo, o app fica na barra de menu até terminar.

**Why this priority**: é o diferencial do produto em relação ao WhatsApp oficial.

**Independent Test**: criar um disparo para 5 números próprios com variável `{nome}`, anexo,
intervalo 5–10 s, limite por hora e janela; acompanhar o progresso; matar o app no meio e reabrir;
retomar e verificar que ninguém recebeu duas vezes; exportar o CSV e conferir os estados.

**Acceptance Scenarios**:

1. **Given** uma lista com colunas `nome` e `cidade`, **When** o usuário escreve
   "Oi {nome}, tudo bem em {cidade}?", **Then** a pré-visualização mostra a mensagem resolvida com
   uma linha real da lista.
2. **Given** 5 destinatários sem valor para `{nome}`, **When** o usuário tenta iniciar, **Then** o
   início é bloqueado com "5 contatos sem {nome}" e a lista das linhas, com opção de definir um
   valor padrão para a variável.
3. **Given** intervalo 30–90 s, limite 40/hora e janela 9h–18h, **When** o disparo roda, **Then**
   cada envio acontece em intervalo aleatório dentro da faixa, nunca mais de 40 envios em
   qualquer período de 60 minutos, e nenhum envio fora da janela; fora dela o estado é
   "fora da janela" com "Aguardando janela (9h–18h)" e retoma sozinho.
4. **Given** disparo em andamento, **When** o usuário clica Pausar e depois Retomar, **Then** o
   envio para e recomeça do próximo destinatário pendente sem reenviar nenhum já enviado.
5. **Given** disparo em andamento, **When** o app é encerrado à força e reaberto, **Then** o
   disparo aparece como pausado com "Disparo pausado porque o app foi fechado." e, ao retomar,
   nenhum destinatário já enviado recebe de novo.
6. **Given** disparo em andamento, **When** o usuário fecha a janela, **Then** aparece "Há um
   disparo em andamento. O ZapDesk vai continuar na barra de menu até terminar.", um ícone na
   barra de menu mostra o progresso e, ao concluir, uma notificação resume o resultado
   ("Disparo concluído: 480 enviados, 12 falharam") e todos os processos são encerrados.
7. **Given** um destinatário responde após receber, **When** a resposta chega, **Then** ela
   aparece na conversa e o destinatário é marcado como "respondeu" no relatório.
8. **Given** disparo concluído, **When** o usuário exporta, **Then** recebe um CSV com uma linha
   por destinatário contendo telefone, nome, estado, motivo da falha e datas de cada transição.
9. **Given** um número sem WhatsApp na lista, **When** chega a vez dele, **Then** o destinatário é
   marcado "falhou" com motivo "Sem WhatsApp" e o disparo segue.

---

### User Story 4 - Mídia e interações no chat (Priority: P2)

O usuário vê e envia imagem, vídeo, áudio, documento e figurinha; toca áudios; grava e envia
áudio pelo microfone; responde citando, reage com emoji, edita e apaga para todos.

**Why this priority**: necessário para substituir o WhatsApp no dia a dia, mas o texto (US1) já
entrega valor sozinho.

**Independent Test**: com uma conta real, enviar e receber cada tipo de mídia, gravar um áudio,
responder citando, reagir, editar e apagar uma mensagem, e confirmar no celular.

**Acceptance Scenarios**:

1. **Given** uma conversa aberta, **When** o usuário anexa uma imagem com legenda, **Then** ela é
   enviada e aparece no celular com a legenda.
2. **Given** uma mensagem de mídia recebida, **When** a bolha aparece, **Then** a mídia é baixada
   sob demanda; se falhar, mostra "Não foi possível baixar." com "Tentar baixar de novo".
3. **Given** permissão de microfone concedida, **When** o usuário grava e solta/envia, **Then** o
   áudio chega ao destinatário como mensagem de voz.
4. **Given** uma mensagem própria enviada há menos de 15 minutos, **When** o usuário edita,
   **Then** o texto muda nos dois lados com indicação "editada"; após o prazo o botão fica
   indisponível.
5. **Given** uma mensagem própria dentro do prazo do WhatsApp, **When** o usuário escolhe
   "Apagar para todos", **Then** ela some nos dois lados com "Mensagem apagada".
6. **Given** uma mensagem qualquer, **When** o usuário reage ou responde citando, **Then** a
   reação/citação aparece nos dois lados.
7. **Given** um envio falhou, **When** a bolha mostra o ícone de erro, **Then** o usuário pode
   clicar "Reenviar".

---

### User Story 5 - Organizar contatos com etiquetas, notas e templates (Priority: P2)

O usuário cria etiquetas coloridas, aplica a contatos, escreve notas, filtra a lista de conversas
por etiqueta, salva templates (texto + anexo opcional) e os insere numa conversa digitando `/` no
campo de mensagem. O painel lateral do contato mostra etiquetas, notas e o lead de origem.

**Why this priority**: melhora a organização e é pré-requisito das ferramentas MCP de templates,
etiquetas e notas (US7); o fluxo principal funciona sem isso.

**Independent Test**: criar a etiqueta "Quente", aplicar a 3 contatos, filtrar a lista por ela,
escrever uma nota, criar um template e inseri-lo numa conversa via `/`.

**Acceptance Scenarios**:

1. **Given** etiqueta "Quente" aplicada a 3 contatos, **When** o usuário filtra a lista de
   conversas por "Quente", **Then** apenas essas conversas aparecem.
2. **Given** um template "Apresentação", **When** o usuário digita `/apre` no campo de mensagem,
   **Then** o template aparece na lista de sugestões e, ao escolher, o texto (e anexo, se houver)
   é inserido para revisão antes do envio.
3. **Given** uma etiqueta com nome já existente, **When** o usuário tenta criar outra igual,
   **Then** a criação é recusada com mensagem de nome duplicado.

---

### User Story 6 - Ver status dos contatos (Priority: P2)

O usuário abre a tela Status e vê os status (stories) publicados pelos contatos nas últimas 24
horas, com imagem, vídeo ou texto, agrupados por contato.

**Why this priority**: completa a experiência do WhatsApp; é pequeno e vem antes do MCP (US7)
porque a IA também pode consultar status.

**Independent Test**: publicar um status no celular de outro número e verificar que aparece na
tela Status do ZapDesk.

**Acceptance Scenarios**:

1. **Given** um contato publicou um status, **When** o usuário abre Status, **Then** vê o contato
   na lista e consegue visualizar o conteúdo.
2. **Given** nenhum status nas últimas 24 h, **When** abre Status, **Then** vê um estado vazio.

---

### User Story 7 - IA controla o ZapDesk pelo MCP (Priority: P2)

Pelo Claude Code ou Claude Desktop, a IA importa leads (recebendo quais eram novos e quais já
existiam, com data), cria disparos que começam imediatamente sem confirmação, acompanha
relatórios, pausa/cancela disparos, lê conversas e respostas, envia mensagens individuais e
gerencia templates, etiquetas e contatos. Se o app estiver fechado, a chamada MCP abre o app e
espera ele ficar pronto.

**Why this priority**: fecha o ciclo "IA gera leads → importa → dispara → lê respostas" sem passo
manual; expõe todas as ações das histórias 1–6, por isso é a última a fechar (o MCP pode
ser desenvolvido em paralelo desde a fundação, contra o contrato da API).

**Independent Test**: com o app fechado, pedir ao Claude Code para importar 10 leads (3 já
existentes) e disparar para eles; verificar que o app abriu, que o retorno listou os 3 duplicados
com data, que o disparo começou na hora e aparece na tela de Disparos, e que a IA consegue ler as
respostas.

**Acceptance Scenarios**:

1. **Given** app aberto, **When** a IA chama `importar_leads` com 10 telefones dos quais 3 já
   existem, **Then** recebe 7 inseridos e 3 "já existia" com a data de importação original.
2. **Given** app aberto, **When** a IA chama `criar_disparo` com lista, mensagem e ritmo válidos,
   **Then** o disparo é criado com origem `mcp` e começa imediatamente (sem confirmação),
   aparecendo em tempo real na tela de Disparos.
3. **Given** app fechado, **When** a IA chama qualquer ferramenta, **Then** o app é aberto, a
   ferramenta espera o app ficar pronto (até ~30 s) e conclui; se não ficar pronto, retorna erro
   claro "Não consegui ligar o ZapDesk".
4. **Given** disparo em andamento, **When** a IA chama `pausar_disparo` ou `cancelar_disparo`,
   **Then** o efeito é o mesmo que o botão da interface.
5. **Given** mensagens recebidas, **When** a IA chama `listar_conversas` e `ler_mensagens`,
   **Then** recebe as conversas e mensagens com remetente, texto, tipo e data.

---

### Edge Cases

- Motor não sobe (porta ocupada, binário ausente): tela "Não consegui ligar o WhatsApp." com
  detalhe e "Tentar de novo".
- Motor cai com app aberto: o app religa uma vez ("O WhatsApp parou. Religando…"); se cair de
  novo, tela de erro; disparos ativos ficam pausados.
- App fechado à força com disparo ativo: na próxima abertura o disparo aparece pausado; retomar
  não reenvia.
- Sessão expirada: conta "desconectada"; disparos dessa conta pausam ("Conta desconectada.
  Reconecte para continuar.").
- Conta banida: conta "banida"; disparos dessa conta param ("O WhatsApp bloqueou este número.").
- Número sem WhatsApp num disparo: destinatário "falhou" com "Sem WhatsApp"; disparo segue.
- Muitas falhas seguidas num disparo: pausa automática após 10 falhas seguidas (configurável;
  "Sem WhatsApp" não conta) com "Disparo pausado após muitas falhas seguidas."
- Dois disparos na mesma conta: não enviam ao mesmo tempo; o segundo fica "Na fila" (estado
  `agendado`) até o ativo terminar, ser pausado ou cancelado.
- CSV sem coluna de telefone: bloqueia e pede mapeamento.
- Telefone inválido: linha rejeitada e listada no relatório.
- Lead duplicado (app ou MCP): não cria; retorna "já existia" + data.
- Variável sem valor: bloqueia início e lista linhas (ou valor padrão).
- Fora da janela: estado "fora da janela"; retoma sozinho.
- Mac dorme durante disparo: ao acordar, recalcula a agenda e segue, sem "compensar" envios
  atrasados em rajada.
- Sem internet: reconexão automática; envios aguardam; nada é marcado "falhou" por queda curta
  ("Sem conexão. Tentando reconectar…").
- MCP chamado com app fechado: abre o app e espera até ~30 s; erro claro se não subir.
- Mídia não baixa: bolha com "Tentar baixar de novo".
- Editar/apagar fora do prazo do WhatsApp: botão indisponível.
- Anexo acima do limite: recusado ao anexar, com o limite exibido.
- Importar lead existente com campos novos: preenche só os campos vazios do lead existente; nunca
  sobrescreve; o relatório continua informando "já existia" com a data original.
- Destinatário encontrado em `enviando` após interrupção (não se sabe se o envio saiu): é
  conferido no histórico local da conversa; se não houver prova do envio, vira "falhou" com motivo
  "Estado incerto após interrupção" em vez de ser reenviado.

## Requirements *(mandatory)*

### Functional Requirements

**Ciclo de vida e contas**

- **FR-001**: Abrir o app DEVE ligar o motor local e mostrar "Ligando o WhatsApp…" até ele ficar
  pronto; o motor DEVE ficar pronto em menos de 2 segundos.
- **FR-002**: Fechar o app sem disparo ativo DEVE encerrar todos os processos do ZapDesk.
- **FR-003**: Fechar a janela com disparo ativo DEVE manter o app vivo apenas como ícone na barra
  de menu (com progresso), e ao término DEVE exibir notificação com o resumo e encerrar tudo.
- **FR-004**: O sistema DEVE permitir conectar várias contas por QR code e persistir as sessões,
  sem pedir QR de novo ao reabrir.
- **FR-005**: O sistema DEVE exibir o estado de cada conta (conectando, conectada, desconectada,
  banida) e oferecer "Reconectar" para contas desconectadas e "Remover conta" em Ajustes.
- **FR-006**: Se o motor cair com o app aberto, o app DEVE religá-lo uma vez automaticamente e,
  se falhar de novo, mostrar tela de erro com "Tentar de novo".

**Chat**

- **FR-010**: O sistema DEVE listar as conversas (individuais e grupos) da conta selecionada,
  ordenadas pela última mensagem, com contagem de não lidas e filtros por não lidas e etiqueta.
- **FR-011**: O sistema DEVE importar o histórico enviado pelo WhatsApp na conexão e, daí em
  diante, salvar localmente toda mensagem enviada e recebida.
- **FR-012**: O usuário DEVE poder ler e enviar texto em conversas individuais e grupos, com
  estado da mensagem (pendente, enviada, entregue, lida, falhou) e "Reenviar" em falhas.
- **FR-013**: O usuário DEVE poder iniciar conversa com um número novo.
- **FR-014**: O usuário DEVE poder buscar mensagens por texto em todas as conversas da conta.
- **FR-015**: O sistema DEVE exibir a lista de contatos da conta.
- **FR-016**: O usuário DEVE poder ver e enviar imagem, vídeo, áudio, documento e figurinha, com
  legenda quando aplicável; mídia recebida é baixada sob demanda e mantida em cache local.
- **FR-017**: O usuário DEVE poder gravar áudio pelo microfone e enviá-lo como mensagem de voz, e
  tocar áudios recebidos.
- **FR-018**: O usuário DEVE poder responder citando, reagir, editar e apagar para todos,
  respeitando os prazos do WhatsApp (botão indisponível fora do prazo).
- **FR-019**: O sistema DEVE marcar a conversa como lida ao abri-la.
- **FR-020**: O usuário DEVE poder ver os status dos contatos publicados nas últimas 24 horas.

**Leads**

- **FR-030**: O sistema DEVE normalizar telefones para E.164, assumindo DDI +55 quando ausente, e
  rejeitar números inválidos listando-os no relatório.
- **FR-031**: O sistema DEVE deduplicar leads somente pelo telefone normalizado; importar telefone
  existente NÃO cria lead novo e o relatório informa "já existia" com a data de importação
  original; duplicados dentro do mesmo lote contam uma vez. Ao reimportar um telefone
  existente, apenas campos vazios do lead (nome e campos extras ausentes) são preenchidos; valores
  já preenchidos, origem e data de importação nunca são sobrescritos.
- **FR-032**: O usuário DEVE poder importar leads de CSV e XLSX (mapeando a coluna de telefone,
  obrigatória, e opcionalmente nome e demais colunas como campos extras), colando números um por
  linha, ou escolhendo contatos/etiquetas.
- **FR-033**: Toda importação DEVE retornar um relatório com totais e listas de: novos, já
  existentes (com data), inválidos (com linha e motivo) e duplicados no lote.
- **FR-034**: O usuário DEVE ver a base de leads em tabela com busca, origem, data de importação e
  último disparo.

**Templates, etiquetas e notas**

- **FR-040**: O usuário DEVE poder criar, editar e excluir templates (nome único, texto com
  variáveis, anexo opcional) e inseri-los numa conversa digitando `/` no campo de mensagem.
- **FR-041**: O usuário DEVE poder criar, editar e excluir etiquetas (nome único de 1–30
  caracteres, cor) e aplicá-las a contatos.
- **FR-042**: O usuário DEVE poder escrever notas livres em contatos e ver, no painel lateral do
  contato, etiquetas, notas e o lead de origem.

**Disparos**

- **FR-050**: O usuário DEVE poder criar um disparo ligado a uma conta, com lista de
  destinatários vinda de importação CSV/XLSX, números colados, contatos/etiquetas ou leads
  existentes; os destinatários são sempre leads da base (a lista é importada pelas mesmas regras
  de FR-030/FR-031).
- **FR-051**: A mensagem do disparo DEVE aceitar variáveis `{nome}` e `{<coluna>}` para qualquer
  campo extra do lead, e anexo opcional com legenda; o usuário DEVE ver pré-visualização com uma
  linha real.
- **FR-052**: O início DEVE ser bloqueado se algum destinatário não tiver valor para uma variável
  usada, listando as linhas incompletas, a menos que um valor padrão seja definido para a
  variável.
- **FR-053**: O usuário DEVE configurar, em cada disparo, intervalo mínimo e máximo (segundos,
  mínimo ≥ 1, máximo ≥ mínimo), e opcionalmente limite por hora, limite por dia, pausa a cada N
  mensagens com duração, data/hora de início e janela diária de envio (início e fim juntos). Não
  há preset imposto; o app mostra aviso de risco de banimento para ritmos agressivos sem
  bloquear.
- **FR-054**: O intervalo entre envios DEVE ser aleatório uniforme entre mínimo e máximo; os
  limites, pausas e janela DEVEM ser sempre respeitados.
- **FR-055**: O sistema DEVE exibir a estimativa de término antes de iniciar e durante o disparo.
- **FR-056**: O usuário DEVE poder pausar, retomar e cancelar um disparo.
- **FR-057**: O disparo DEVE seguir a máquina de estados: `rascunho → agendado → enviando ⇄
  fora_da_janela → concluido`; `agendado/enviando/fora_da_janela → pausado`; `pausado → agendado`
  (retomar; volta a `enviando`/`fora_da_janela` quando for a vez da conta — FR-065); qualquer
  estado não final → `cancelado`.
- **FR-058**: Cada destinatário DEVE seguir: `pendente → enviando → enviado → entregue → lido`;
  `pendente/enviando → falhou` (com motivo; uma mensagem já aceita pelo WhatsApp não volta a
  falhar); `enviado/entregue/lido → respondeu` quando
  chega mensagem do número na mesma conta após o envio.
- **FR-059**: Retomar um disparo (após pausa, queda, fechamento forçado, sono do Mac ou perda de
  conexão) NUNCA DEVE reenviar para destinatário já enviado; disparos ativos encontrados ao
  reabrir o app DEVEM aparecer como `pausado`.
- **FR-060**: Conta desconectada DEVE pausar os disparos dela; conta banida DEVE pará-los.
- **FR-061**: O progresso DEVE ser exibido em tempo real: barra, contadores por estado, próximo
  envio previsto e estado do disparo.
- **FR-062**: O relatório DEVE mostrar cada destinatário com estado, motivo de falha e datas, e
  ser exportável em CSV.
- **FR-063**: Anexos acima do limite do WhatsApp DEVEM ser recusados no momento de anexar.
- **FR-064**: O disparo DEVE pausar automaticamente após N falhas seguidas (N configurável por
  disparo, padrão 10, 0 desliga), com a mensagem "Disparo pausado após muitas falhas seguidas.";
  falhas por "Sem WhatsApp" não contam e qualquer envio bem-sucedido zera a contagem.
- **FR-065**: Cada conta DEVE ter no máximo um disparo em `enviando`/`fora_da_janela`; outros
  disparos prontos da mesma conta ficam em `agendado`, exibidos como "Na fila", e começam em ordem
  (horário de início, depois criação) quando o ativo conclui, é pausado ou cancelado.
- **FR-066**: Antes de enviar a cada destinatário, o sistema DEVE verificar se o número tem
  WhatsApp e registrar o resultado no lead (`tem_whatsapp`); não há verificação em lote no MVP.
- **FR-067**: Um destinatário encontrado em `enviando` após interrupção DEVE ser conferido no
  histórico local; sem prova de envio, é marcado `falhou` com motivo "Estado incerto após
  interrupção" e nunca reenviado automaticamente.

**MCP**

- **FR-070**: O sistema DEVE oferecer um servidor MCP utilizável pelo Claude Code e pelo Claude
  Desktop que exponha todas as ações do usuário: listar contas; importar leads (com relatório de
  duplicados) e listar/buscar leads; listar conversas, ler mensagens, buscar mensagens; enviar
  mensagem individual (texto e anexo); criar, iniciar, pausar, retomar, cancelar e listar
  disparos e ver relatório; gerenciar templates, etiquetas, notas e contatos; reagir, editar e
  apagar mensagens; ver status. Exceção: conectar, reconectar, renomear e remover contas ficam só
  na interface (conectar exige escanear QR no celular).
- **FR-071**: Disparo criado via MCP DEVE começar imediatamente, sem confirmação, respeitando o
  agendamento informado e a fila da conta (FR-065); se faltar valor de variável para algum
  destinatário e a chamada não trouxer `valores_padrao`, a criação é recusada com a lista das
  linhas incompletas.
- **FR-072**: Se o app estiver fechado, a chamada MCP DEVE abrir o app e esperar até ~30 s pelo
  motor; se falhar, retornar erro claro.
- **FR-073**: A tela Ajustes DEVE mostrar as instruções para configurar o MCP no Claude Code e no
  Claude Desktop, além de versão, pasta de dados e caminho dos logs.

**Privacidade, segurança e dados**

- **FR-080**: Todos os dados DEVEM ficar na pasta de dados local; nenhum dado é enviado a
  terceiros além do próprio WhatsApp; sem telemetria.
- **FR-081**: A interface local do motor DEVE aceitar pedidos apenas da própria máquina e com o
  token gerado a cada abertura.
- **FR-082**: Logs NÃO DEVEM conter o conteúdo das mensagens por padrão.
- **FR-083**: Antes de atualizar a estrutura do banco de dados, o sistema DEVE fazer backup
  automático do banco.

**Interface**

- **FR-090**: Toda a interface DEVE estar em português do Brasil, seguir o tema claro/escuro do
  sistema e ter visual inspirado no WhatsApp Desktop.
- **FR-091**: As ações principais (enviar, buscar, trocar de conversa) DEVEM ser acessíveis por
  teclado e todo botão de ícone DEVE ter rótulo acessível.
- **FR-092**: Cada tela DEVE ter estados de carregando, vazio e erro conforme a seção 5.4 do
  documento de produto.

### Key Entities *(include if feature involves data)*

- **Conta**: número conectado; nome (1–60 caracteres, padrão pushname), JID após conectar,
  estado (conectando, conectada, desconectada, banida), data de criação.
- **Contato**: pessoa conhecida por uma conta; nome, telefone/JID, notas, etiquetas (N:N), lead
  de origem (se houver).
- **Conversa**: por conta e por JID (individual ou grupo); não lidas, última mensagem.
- **Mensagem**: id do WhatsApp, remetente, tipo (texto, imagem, vídeo, áudio, documento,
  figurinha, reação, sistema), texto, mídia local, citação, reações, editada/apagada, estado,
  data.
- **Status**: publicação de um contato visível por 24 h.
- **Etiqueta**: nome único (1–30), cor hex.
- **Lead**: telefone E.164 único, nome opcional, campos extras, origem (csv, colado, contatos,
  mcp), tem WhatsApp (desconhecido/sim/não), data de importação.
- **Template**: nome único, texto com variáveis, anexo opcional.
- **Disparo**: conta, nome, mensagem, anexo, ritmo (intervalos, limites, pausas), início, janela,
  estado, origem (app ou mcp).
- **Destinatário**: disparo, lead, telefone, variáveis resolvidas, estado, motivo de falha, id da
  mensagem enviada, datas de cada transição.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: O app fica utilizável (conversas visíveis) em até 2 segundos após abrir, com contas
  já conectadas, medido do clique ao fim do "Ligando o WhatsApp…".
- **SC-002**: Após fechar o app sem disparo ativo, 0 processos do ZapDesk permanecem em execução.
- **SC-003**: Com contas conectadas e sem disparo, o componente local de WhatsApp ocupa menos de
  80 MB de memória.
- **SC-004**: Reabrir o app 10 vezes seguidas nunca pede QR code para contas já conectadas.
- **SC-005**: Uma importação de 1.000 linhas com duplicados e inválidos gera relatório 100%
  correto (novos + já existentes + inválidos + duplicados no lote = total de linhas) em menos de
  5 segundos.
- **SC-006**: Em 100% dos testes de interrupção (pausa, fechamento forçado, queda de conexão,
  sono), nenhum destinatário recebe a mensagem do mesmo disparo duas vezes.
- **SC-007**: Em um disparo com janela e limites configurados, 0 envios ocorrem fora da janela e
  0 períodos de 60 minutos excedem o limite por hora.
- **SC-008**: A lista de conversas e o chat continuam fluidos (rolagem sem travar, abrir conversa
  em menos de 500 ms) com dezenas de milhares de mensagens armazenadas.
- **SC-009**: Pelo Claude Code ou Claude Desktop, o ciclo "importar leads → disparar → ler
  respostas" é concluído sem nenhuma ação manual na interface, inclusive com o app fechado no
  início.
- **SC-010**: Todas as ações da interface listadas em FR-070 estão disponíveis via MCP.
- **SC-011**: Disparos de até 50.000 destinatários são criados e acompanhados sem degradação da
  interface.

## Assumptions

- Usuário único local (Gabriel); a IA via MCP tem exatamente o mesmo poder; não há login.
- Plataforma: macOS com Apple Silicon (Intel desejável, não obrigatório no MVP); distribuição
  local sem assinatura/notarização Apple (instrução "Abrir mesmo assim").
- O usuário assume os riscos de banimento e LGPD de disparar para leads frios sem opt-out; não há
  lista de opt-out nem rodízio de números ou variação automática de texto (fora do escopo).
- Fuso horário da janela de envio, do agendamento e do "limite por dia" é o fuso local do Mac;
  "dia" é o dia civil local (zera à meia-noite); "limite por hora" vale para qualquer período
  móvel de 60 minutos.
- Um destinatário "respondeu" quando chega qualquer mensagem do mesmo número, na mesma conta,
  depois do envio.
- Edição permitida até 15 minutos após o envio e apagar para todos até o prazo atual do WhatsApp
  (cerca de 2 dias); prazos podem mudar com o WhatsApp.
- Limites de anexo padrão: 16 MB para imagem, vídeo e áudio; 100 MB para documento.
- Fora do escopo: chamadas de voz/vídeo, Windows/Linux, API oficial, sincronização em nuvem,
  publicar status próprio, criar/administrar grupos e comunidades, canais, chatbot.
- Nome "ZapDesk" é provisório.
