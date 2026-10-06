<!--
Relatório de Impacto da Sincronização (Sync Impact Report)
- Versão: 1.0.1 → 1.1.0 (MINOR)
- Motivo do bump: feature 002 (automações). Expansão material do Princípio I (segunda exceção de
  rede: Claude API chamada pelo motor só a partir de automação de IA que o usuário ativou ou testou,
  com a chave dele; rede do código do usuário só com a permissão `rede`), do Princípio II (processos
  de automação contam como filhos), do Princípio III (motor dono também de automações, funil,
  esperas e projetos de código; runner como cliente com canal de escopo) e do Princípio VI (novas
  regras de domínio com teste obrigatório); novo Princípio VIII (código do usuário isolado e
  automações seguras); Restrições técnicas e Fluxo de desenvolvimento atualizados para o novo
  diretório `automacao/` e `compartilhado/automacao-sdk/`. Nenhum princípio removido ou
  redefinido de forma incompatível.
- Histórico: 1.0.0 = adoção inicial; 1.0.1 (PATCH) = esclarecimento de dados entregues ao cliente
  de IA via MCP e aceite do SDK MCP v2.
- Princípios modificados:
  I. Local-first e privacidade (NÃO NEGOCIÁVEL) — exceção 2 (Claude API por automação ativada)
  II. Leveza e ciclo de vida limpo — processos de automação de IA
  III. Motor como fonte única da verdade — automações, funil, esperas, runner; exceção dos
     valores de segredos (definidos só pela janela do app, via stdin)
  VI. Testes obrigatórios para regras de domínio (NÃO NEGOCIÁVEL) — novas regras
- Princípio adicionado: VIII. Código do usuário isolado e automações seguras
- Seções modificadas: Restrições técnicas e de segurança; Fluxo de desenvolvimento e portões de
  qualidade.
- Seções removidas: nenhuma.
- Templates: plan-template.md, spec-template.md e tasks-template.md leem a constituição em tempo de
  execução; nenhum ajuste necessário (✅). Plano da 001 continua conforme (nada nele usa IA/rede).
- TODOs pendentes: nenhum.
-->
# Constituição do ZapDesk

## Core Principles

### I. Local-first e privacidade (NÃO NEGOCIÁVEL)

- Todos os dados (sessões do WhatsApp, mensagens, mídia, leads, disparos, logs) DEVEM ficar na
  máquina do usuário, na pasta de dados do app (`~/Library/Application Support/ZapDesk/`).
- Nenhum componente PODE enviar dados a servidores de terceiros além do próprio protocolo do
  WhatsApp. Não há telemetria, analytics, crash reporting remoto nem sincronização em nuvem.
- Exceção explícita 1: o servidor MCP responde, via stdio local, ao cliente de IA que o próprio
  usuário configurou (Claude Code / Claude Desktop). O que esse cliente faz com a resposta é
  escolha do usuário.
- Exceção explícita 2 (automações de IA): o motor PODE chamar a Claude API (Anthropic Messages
  API) **somente** quando uma automação de IA que o próprio usuário ativou (ou mandou testar)
  executa um helper `ctx.ia` (ou quando o usuário clica em "Testar chave" em Ajustes → IA),
  **somente** com a chave de API que o usuário configurou em Ajustes, e enviando apenas o que o
  código dessa automação passou ao helper. Sem chave configurada ou sem
  automação ativa/teste pedido, nenhuma conexão com serviço de IA PODE ocorrer. Nenhuma automação
  vem ativada por padrão.
- O código de uma automação de IA só PODE acessar a rede por `ctx.http.fetch` e apenas se declarar
  a permissão `rede` no seu manifesto; esse tráfego é escolha do código do usuário.
- Fora dessas exceções, o ZapDesk NÃO PODE abrir conexão de rede com serviços de IA nem enviar
  dados a eles por conta própria. A chave da Anthropic e os demais segredos DEVEM ficar no chaveiro
  do macOS (via app) e na memória do motor; NUNCA em arquivo em claro, log, resposta da API local
  ou do MCP, nem no processo que executa o código do usuário (no caso da chave da Anthropic).
- O motor DEVE escutar exclusivamente em `127.0.0.1` (nunca `0.0.0.0` ou interface externa), em
  porta aleatória, e DEVE rejeitar qualquer requisição HTTP ou WebSocket sem o token de sessão
  gerado a cada inicialização.
- O arquivo `runtime.json` (porta, token, pid) DEVE ser gravado com permissão `0600`.
- Logs NÃO PODEM conter o conteúdo de mensagens por padrão, nem o conteúdo de `ctx.log` das
  automações (que fica só no registro de execuções do banco local).

**Razão**: o produto existe para substituir servidores pesados na nuvem; os dados de leads e
conversas são sensíveis (LGPD) e o risco é aceito apenas enquanto permanecerem locais.

### II. Leveza e ciclo de vida limpo

- O motor ocioso (contas conectadas, sem disparo) DEVE usar menos de 80 MB de RAM.
- O motor DEVE ficar pronto para receber requisições em menos de 2 s após ser iniciado.
- Fechar o app sem disparo ativo DEVE encerrar todos os processos do ZapDesk (app, motor, filhos,
  inclusive os processos das automações de IA); nenhum processo PODE sobrar (`ps` limpo). A única exceção é o app permanecer na barra de menu
  enquanto um disparo ativo termina, encerrando tudo sozinho ao final.
- Processos de automação de IA DEVEM ser iniciados sob demanda, encerrados após ociosidade e
  limitados em quantidade simultânea; automações NÃO PODEM manter o app aberto nem atrasar o
  `pronto` do motor. A meta de RAM ociosa do motor vale sem processos de automação vivos.
- É proibido Docker, banco de dados em servidor (Postgres, Redis) ou qualquer serviço residente
  em qualquer parte do runtime.
- Novas dependências pesadas no motor DEVEM ser justificadas na seção de complexidade do plano.

**Razão**: a leveza é o motivo de existir do produto frente à Evolution API.

### III. Motor como fonte única da verdade

- O motor Go (`motor/`) é o único dono dos dados (SQLite), do agendador de disparos e do motor
  de automações (gatilhos, esperas, agendamentos, sessões de chatbot, funil, execuções) e dos
  arquivos dos projetos de automação de IA na pasta de dados.
- A interface Electron (`app/`) e o servidor MCP (`mcp/`) são clientes: DEVEM ler e alterar
  estado exclusivamente pela API HTTP/WebSocket do motor. Nenhum deles PODE abrir o SQLite
  diretamente nem falar com o WhatsApp.
- O processo que executa código do usuário (runner) é um cliente ainda mais restrito: fala com o
  motor apenas por um canal próprio (stdio) com escopo daquela automação, sem token da API local,
  sem SQLite e sem arquivos; o motor valida cada chamada contra as permissões declaradas.
- Toda ação disponível na interface DEVE estar exposta na API do motor (e, portanto, poder ser
  exposta no MCP), com o mesmo comportamento e as mesmas validações. Única exceção: definir ou
  remover **valores** de segredos (chave da Anthropic e afins) é feito só pela janela do app, que
  os guarda no chaveiro e os entrega ao motor pelo stdin; a API e o MCP expõem apenas os nomes.
- A API do motor é um contrato versionado e documentado em `specs/<feature>/contracts/`;
  mudanças incompatíveis exigem atualização do contrato e dos dois clientes na mesma entrega.

**Razão**: usuário e IA precisam ver o mesmo estado, e o disparo precisa continuar independente
da janela.

### IV. WhatsApp isolado atrás de interface

- Todo uso de `go.mau.fi/whatsmeow` DEVE ficar confinado a um único pacote adaptador do motor que
  implementa uma interface Go própria (ex.: `ClienteWhatsApp`) definida no domínio.
- O restante do motor (domínio, agendador, API) DEVE depender apenas dessa interface.
- DEVE existir um cliente WhatsApp falso (em memória, determinístico) que implemente a mesma
  interface e permita rodar o motor inteiro, a API e o MCP sem conta real. Esse modo DEVE ser
  ativável por flag/variável de ambiente para desenvolvimento e testes.

**Razão**: whatsmeow é não oficial e pode quebrar; isolar reduz o impacto e torna testes possíveis
sem banimento de número.

### V. Disparo idempotente e retomável

- Cada transição de estado de um disparo e de cada destinatário DEVE ser persistida antes do
  próximo passo (antes de enviar: `enviando`; após confirmação do servidor: `enviado` + id da
  mensagem).
- Retomar um disparo (após pausa, queda do motor, fechamento forçado, sono do Mac ou perda de
  conexão) NUNCA PODE reenviar a um destinatário já `enviado` ou posterior.
- Um destinatário encontrado em `enviando` na retomada DEVE ser tratado de forma conservadora
  (verificado contra o histórico local da conversa; na dúvida, marcado `falhou` com motivo
  "estado incerto após interrupção" em vez de reenviar).
- Disparo ativo encontrado na inicialização do motor DEVE voltar como `pausado`.
- Queda curta de conexão NÃO PODE marcar destinatários como `falhou`.

**Razão**: reenviar mensagens para leads frios aumenta o risco de ban e é irreversível.

### VI. Testes obrigatórios para regras de domínio (NÃO NEGOCIÁVEL)

- As seguintes regras DEVEM ter testes unitários automatizados escritos antes ou junto da
  implementação, e o código NÃO PODE ser integrado sem eles passando:
  normalização de telefone para E.164; deduplicação de leads (entre lotes e dentro do lote);
  cálculo de agendamento (intervalo aleatório, limites por hora/dia, pausas, janela de envio);
  resolução de variáveis de mensagem; máquina de estados do disparo e do destinatário;
  casamento de gatilhos e avaliação de condições das automações; executor de fluxo e de chatbot;
  anti-loop, pausas de conversa e limite de primeiros contatos; esperas, agendamentos e regra de
  vencimento; checagem de permissões do `ctx`.
- O runner de automações de IA DEVE ter testes de integração contra o motor com WhatsApp falso e
  IA simulada (sem rede real).
- A API HTTP/WebSocket do motor DEVE ter testes de integração contra o cliente WhatsApp falso.
- As ferramentas MCP DEVEM ter testes contra um motor rodando com cliente falso.
- Testes que dependem de tempo DEVEM usar relógio injetável (nada de `sleep` real).

**Razão**: são as regras cujo erro causa dano real (mensagem duplicada, disparo fora de hora,
lead perdido).

### VII. Português em tudo

- Interface do usuário exclusivamente em português do Brasil.
- Identificadores de domínio, comentários, mensagens de log, mensagens de erro da API, nomes de
  ferramentas MCP e documentação DEVEM estar em português. Palavras-chave da linguagem, nomes de
  bibliotecas e termos técnicos consagrados (ex.: `handler`, `WebSocket`) são permitidos.

**Razão**: o único usuário e mantenedor trabalha em português; consistência reduz atrito.

### VIII. Código do usuário isolado e automações seguras

- Código escrito pelo usuário (ou pela IA a pedido dele) DEVE rodar em processo separado do app,
  do motor e das demais automações, com ambiente limpo (sem token, sem chave da Anthropic), acesso
  apenas à API `ctx`, imports restritos na compilação e bloqueios em tempo de execução para
  arquivos, processos e rede direta.
- Cada execução DEVE ter tempo limite e o processo limite de memória; falha, laço infinito ou
  morte do processo NÃO PODE afetar o motor, o app ou outras automações.
- O limite do isolamento DEVE estar documentado com honestidade: ele protege contra erro e
  acidente, não contra código malicioso.
- Todo envio automático DEVE passar por um único portão no motor que aplica pausa da conversa
  (atendimento humano), grupos desligados por padrão, limite anti-loop e limite de primeiros
  contatos; automações NUNCA PODEM reagir a mensagens do próprio número.
- O modo simulação NUNCA PODE enviar mensagens nem alterar dados.

**Razão**: o usuário quer programar de verdade, e automações que respondem leads frios sem freio
geram loops, spam e ban — riscos irreversíveis como o reenvio em disparos.

## Restrições técnicas e de segurança

- Plataforma alvo: macOS (Apple Silicon obrigatório; Intel desejável).
- Monorepo com quatro partes: `motor/` (Go + whatsmeow + SQLite puro Go; compila o TypeScript das
  automações com o esbuild embutido como biblioteca Go), `app/` (Electron + React + TypeScript +
  Vite), `mcp/` (TypeScript + SDK MCP oficial — `@modelcontextprotocol/sdk` ou a sucessora v2
  `@modelcontextprotocol/server` —, stdio) e `automacao/` (runner Node das automações de IA,
  executado pelo binário do Electron com `ELECTRON_RUN_AS_NODE=1`), mais os pacotes de contrato em
  `compartilhado/` (`cliente-motor`, `automacao-sdk`).
- O fuse `runAsNode` do Electron DEVE permanecer ligado (MCP e runner dependem dele).
- SQLite com migrações versionadas; backup automático do banco antes de aplicar migração.
- Proibido: Evolution API, Baileys (motor duplicado), Docker, qualquer servidor remoto próprio.
- O app Electron DEVE usar `contextIsolation: true`, `nodeIntegration: false` e expor ao renderer
  apenas uma ponte (preload) mínima; o token do motor não é persistido fora de `runtime.json`.
- Disparos iniciados pelo MCP começam sem confirmação (decisão do usuário); por isso as
  ferramentas de pausar e cancelar DEVEM existir no MCP e na interface.

## Fluxo de desenvolvimento e portões de qualidade

- Todo trabalho segue o fluxo Spec Kit: spec → clarify → plan → tasks → analyze → implement.
- Portões antes de integrar qualquer mudança:
  1. `go test ./...` em `motor/` passando (inclui testes de domínio e de integração da API).
  2. Testes e checagem de tipos de `app/`, `mcp/`, `automacao/` e `compartilhado/` passando.
  3. Contrato da API atualizado se algum endpoint ou evento mudou.
- Critérios de leveza (RAM ociosa, tempo de subida, `ps` limpo) DEVEM ser verificados antes de
  cada versão `.dmg`.
- A implementação pode ser dividida entre agentes por diretório (`motor/`, `app/`, `mcp/`,
  `automacao/` + `compartilhado/automacao-sdk/`); os contratos da feature (API, eventos, protocolo
  motor↔runner, SDK) são o ponto de sincronização entre eles.

## Governance

- Esta constituição prevalece sobre outras práticas do projeto. Planos e tarefas que conflitem
  com um princípio DEVEM ser ajustados, ou o princípio emendado explicitamente antes.
- Emendas: alteração neste arquivo com Relatório de Impacto no topo, nova versão e data.
- Versionamento semântico: MAJOR para remoção/redefinição de princípio; MINOR para princípio ou
  seção nova ou expansão material; PATCH para esclarecimentos de redação.
- Revisão de conformidade: a seção "Constitution Check" de cada `plan.md` e o `/speckit-analyze`
  verificam aderência; violações justificadas vão para "Complexity Tracking".

**Version**: 1.1.0 | **Ratified**: 2026-09-27 | **Last Amended**: 2026-09-27
