---

description: "Lista de tarefas da feature 003 — Identidade visual"
---

# Tasks: Identidade visual — o app com a cara da landing

**Input**: Documentos de design em `/specs/003-identidade-visual/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/ (tokens,
vocabulario-visual, capturas), quickstart.md

**Tests**: a constituição só exige testes para regras de domínio, e esta feature não cria nenhuma.
Os testes existentes do app DEVEM continuar passando **sem mudar expectativas**; o único teste novo
cobre a preferência de tema (T021). A validação visual é por `contraste.mjs`, `verificar-tokens.mjs`
e capturas `capturar.mjs` (antes × depois × mockup), em cada bloco e na Fase 5.

**Organization**: fundação (sequencial, um agente) → **8 blocos [P] por área** (B1–B8), cada um
dono exclusivo de **um arquivo CSS de área** e dos componentes TSX da área → integração (um agente)
→ verificação → entrega. Cada tarefa de bloco leva a user story que mais atende (US1 conversas,
US2 demais telas, US3 logotipo, US4 conforto/AA, US5 tema manual).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: pode rodar em paralelo (arquivos diferentes, sem dependência de tarefa incompleta)
- **[Story]**: US1–US5 da spec (só nas fases de bloco)
- Caminhos relativos à raiz do repositório `~/projetos/zapdesk`

## Regras para TODOS os agentes (ler antes de começar)

1. **Só visual.** Nenhuma mudança de comportamento, fluxo, texto visível, `aria-*`, rota, atalho,
   ordem de foco ou disposição funcional. Em TSX só se mexe em: `className`, marcação puramente
   decorativa (span de lâmpada, ponto, wrapper sem semântica), ícone/logotipo, e `style` de cor que
   vira token. Exceção única: seletor de tema (T039) e leitura do tema efetivo (T047, T050).
2. **Fonte da verdade do visual**: `contracts/tokens.md` (nomes/valores), `contracts/vocabulario-visual.md`
   (receitas) e os mockups da landing em
   `../zapdesk-site/src/components/mockups/` + capturas em
   `../zapdesk-site/docs/verificacao/final/`. **Nenhuma cor
   literal** fora de `app/src/renderer/estilos/tema.css` (exceções em tokens.md §4). **Nada verde.**
   `--sinal` nunca como cor de texto/ícone no claro — use `--sinal-texto`.
3. **Use só nomes novos** de token no seu arquivo (os aliases antigos existem só para não quebrar
   quem ainda não terminou; somem em T053).
4. **Blocos não editam arquivos compartilhados**: `estilos/{tema,fontes,base,controles,estrutura}.css`,
   `main.tsx`, `util/tema.ts`, `componentes/Logo.tsx`, `App.tsx`, `rotas.tsx`, `app/src/main/*`,
   `app/src/preload/*`, `app/scripts/*`, `package.json`. Faltou token/variação de primitivo? Uma linha
   em `specs/003-identidade-visual/pendencias.md` com o prefixo do bloco (ex.: `B3: …`), um
   ajuste local **no seu arquivo** e seguir; a integração (T052) resolve.
5. **Não sobrescrever primitivos** (`.botao`, campos, `.cartao`, `.selo-estado`, `.tabela`, `.aba`,
   `.modal`…) no seu arquivo; só compor/posicionar.
6. **Alturas de itens de listas virtualizadas não mudam** (research R5). `line-height` em px nesses
   itens.
7. **Capturas** só com `node app/scripts/capturar.mjs` (modo falso, dados fictícios, pasta
   temporária). **Nunca** abrir o app com a conta real nem ler `~/Library/Application Support/ZapDesk`.
   Não usar o Playwright MCP; não instalar browsers. Antes de capturar: `npm run compilar -w @zapdesk/app`.
   Dois agentes não compilam ao mesmo tempo: quem for capturar compila e captura em sequência
   (o `out/` é compartilhado; se outro bloco estiver no meio de uma edição, a captura dele pode
   aparecer — avaliar só as telas do seu bloco).
8. Checagem por bloco: `npm run tipos -w @zapdesk/app`, `npm test -w @zapdesk/app`,
   `npm run tokens -w @zapdesk/app -- --arquivos <seus arquivos>`, `npm run contraste -w @zapdesk/app`.
9. **Sem git**: o repositório não tem commits; não rodar `git commit`, `git checkout`, `git branch`,
   `git stash`. O dono faz os commits.
10. Português em tudo (comentários, nomes de classe novos, mensagens de script).

---

## Phase 1: Setup — base de comparação, dependências e captura "antes"

**Purpose**: congelar o estado atual (código e telas) e montar o ferramental de captura sem
WhatsApp real, **antes de qualquer mudança visual**.

- [X] T001 Criar `specs/003-identidade-visual/pendencias.md` (cabeçalho "append-only, prefixo do bloco") e a pasta `docs/verificacao/003/`; copiar o estado atual de `app/src/` e `app/scripts/` para `~/.cache/zapdesk-003-base/` (base para o diff "só visual" de T060, já que não há commits)
- [X] T002 Rodar `npm install` na raiz e compilar tudo (quickstart §0: `motor:compilar`, `compartilhado:compilar`, `runner:compilar`, `npm run compilar -w @zapdesk/app`); anotar em pendencias.md qualquer falha pré-existente de `npm run tipos -w @zapdesk/app` / `npm test -w @zapdesk/app`
- [X] T003 [P] Em `app/package.json`: devDependencies exatas `@fontsource-variable/bricolage-grotesque@5.3.0`, `@fontsource-variable/instrument-sans@5.3.0`, `@fontsource-variable/jetbrains-mono@5.3.0`; scripts `"contraste": "node scripts/contraste.mjs"`, `"tokens": "node scripts/verificar-tokens.mjs"`, `"capturar": "node scripts/capturar.mjs"`; `npm install` de novo
- [X] T004 [P] Modo falso isolado da instância instalada: em `app/src/main/pastas.ts` função pura `resolverPastaInterface({ env, modoFalso, padrao, tmp })` (usa `ZAPDESK_PASTA_INTERFACE`; no modo falso sem env usa `mkdtempSync(tmp/zapdesk-interface-*)`; senão `padrao`) e usá-la em `app/src/main/index.ts` no `app.setPath('userData', …)` **antes** de `requestSingleInstanceLock()`; teste em `app/tests/main/pastas.test.ts` (padrão inalterado no modo real)
- [X] T005 [P] Criar `app/scripts/semente-falsa.mjs` conforme `contracts/capturas.md` → "Semente fictícia": `export async function semear({ base, token })` + CLI `--runtime <runtime.json>`; dados fictícios espelhando `zapdesk-site/src/content/mockups.ts`; telefones `+5511900000100…`; grava e devolve os ids (`semente.json`)
- [X] T006 [P] Criar `app/scripts/capturar.mjs` conforme `contracts/capturas.md` (CLI `--saida/--telas/--temas/--tamanhos/--comparar/--medir/--manter`): `_electron` de `$PLAYWRIGHT_PATH`; Electron via `require.resolve('electron', { paths: ['app'] })`; env `ZAPDESK_DEV_FALSO=1`, `ZAPDESK_PASTA_DADOS`/`ZAPDESK_PASTA_INTERFACE` em `mkdtemp`; espera `runtime.json`; chama `semear`; `PUT /v1/falso/relogio` fixo; tema por `electronApp.evaluate(({ nativeTheme }, t) => { nativeTheme.themeSource = t })`; navegação por `location.hash`; `setViewportSize`/`BrowserWindow.setContentSize`; telas `carregando` (wrapper de motor atrasado em tmp) e `erro-motor` (`ZAPDESK_MOTOR_BIN=/usr/bin/false`); comparação pixel a pixel via `nativeImage.toBitmap()` com máscara `[data-captura-mascara]` + horas relativas; `--medir` (alturas de itens virtuais, layout-shift, `document.fonts`, requisições ≠ 127.0.0.1); limpeza garantida (finally + SIGINT)
- [X] T007 Capturar o **antes** (todas as telas × claro/escuro × 1280x820/960x620) e medidas: `node app/scripts/capturar.mjs --saida docs/verificacao/003/antes --medir`; conferir 2–3 PNGs com Read; rodar 2× e confirmar que `--comparar` dá 0 entre as duas rodadas (determinismo) antes de seguir

**Checkpoint**: telas do app atual capturadas com dados fictícios; nenhuma linha de CSS mudou.

---

## Phase 2: Foundational — divisão do CSS, tokens, fontes, primitivos, marca e checagens

**⚠️ Nenhum bloco começa antes do checkpoint T022.**

### Divisão mecânica (sem mudança visual)

- [X] T008 Dividir `app/src/renderer/estilos/app.css` e `automacoes.css` em `base.css`, `controles.css`, `estrutura.css`, `shell.css`, `conversas.css`, `conversa.css`, `disparos.css`, `crm.css`, `ajustes.css`, `funil.css`, `automacoes.css` (reduzido), `editor-ia.css` seguindo o "Mapa da divisão" do plan.md (conteúdo copiado sem alteração; `.ponto-etiqueta` e `.abas` para `controles.css`; `.selo-estado`, `.tabela*`, `.lista-definicoes` para `estrutura.css`); apagar `app.css`; atualizar os imports em `app/src/renderer/main.tsx` (tema → base → controles → estrutura → shell → conversas → conversa → crm → disparos → ajustes → funil → automacoes → editor-ia) e registrar no topo de cada arquivo o comentário de origem (linhas antigas) e o bloco dono
- [X] T009 Provar a divisão: `npm run compilar -w @zapdesk/app` + `node app/scripts/capturar.mjs --saida docs/verificacao/003/divisao --comparar docs/verificacao/003/antes` = 0 pixels diferentes; se houver diferença, corrigir a ordem/posição das regras concorrentes (não o conteúdo) até zerar

### Tokens, fontes, tema

- [X] T010 Reescrever `app/src/renderer/estilos/tema.css`: tokens §1 e §2 de `contracts/tokens.md` (claro em `:root`; escuro em `@media (prefers-color-scheme: dark) { :root:not([data-tema="claro"]) }` e em `:root[data-tema="escuro"]`, com `color-scheme` em cada bloco) + bloco "Transição" com os aliases da §3 (`--verde: var(--sinal)` etc.); comentário de cabeçalho apontando para `contracts/tokens.md` e `zapdesk-site/src/app/globals.css`
- [X] T011 [P] Criar `app/src/renderer/estilos/fontes.css` (research R4): `@font-face` de `'Bricolage Grotesque Variable'` (`bricolage-grotesque-latin-standard-normal.woff2`, `font-weight 200 800`, `font-stretch 75% 100%`), `'Instrument Sans Variable'` (`instrument-sans-latin-standard-normal.woff2` + `…-standard-italic.woff2`, `font-weight 400 700`, `font-stretch 75% 100%`), `'JetBrains Mono Variable'` (`jetbrains-mono-latin-wght-normal.woff2`, `font-weight 100 800`), todos `font-display: block` e `unicode-range` latin, URLs `@fontsource-variable/<pacote>/files/<arquivo>`; e `app/src/renderer/util/fontes.ts` com `aguardarFontes(limiteMs = 500)` (`document.fonts.load` das 3 famílias, `Promise.race` com o limite)
- [X] T012 Criar `app/src/renderer/util/tema.ts` (data-model §3, research R6): `PreferenciaTema`, `lerPreferencia()`, `salvarPreferencia()`, `aplicarTema()`, `temaEfetivo()`, `useTemaEfetivo()` (`useSyncExternalStore` com `matchMedia` + evento próprio); em `app/src/renderer/main.tsx` importar `fontes.css` logo após `tema.css`, chamar `aplicarTema(lerPreferencia())` e `await aguardarFontes()` antes do `createRoot().render()`

### Primitivos compartilhados (receitas de `contracts/vocabulario-visual.md` → "Primitivos")

- [X] T013 Restilizar `app/src/renderer/estilos/base.css` só com tokens novos: `body` `--fonte` 14 px/1.5 `--fundo` `--texto` com antialias; `h1–h3` `--fonte-titulo` (pesos/`font-variation-settings` da landing); `.mono`/`code`/`pre` `--fonte-mono` sem ligaduras; utilitárias novas `.rotulo`, `.lampada`, `.terminal-cabo`, `.terminal-ativo`; `:focus-visible` `--foco` 2 px offset 2 px; `::selection`/`mark`; barras de rolagem finas em `--linha-forte`; `@media (prefers-reduced-motion: reduce)` zerando animações/transições
- [X] T014 [P] Restilizar `app/src/renderer/estilos/controles.css`: botões (primário âmbar + `--tinta` + `--borda-primario`, secundário, perigo, pequeno/grande/largo, ícone), campos (`input/textarea/select`, foco, erro em `--coral`, números/horas em mono), `.grupo-campos`, `.interruptor`, `.abas/.aba`, `.ponto-etiqueta` (contorno inset `--linha-forte`), utilitários de automações — sem cor literal
- [X] T015 [P] Restilizar `app/src/renderer/estilos/estrutura.css`: `.tela`, `.cabecalho-tela` (título `--fonte-titulo`), `.barra-ferramentas`, `.cartao`, `.tela-dividida`, `.estado-vazio` (ícone em quadrado com borda `--linha-forte`, ícone `--sinal-texto`), `.tela-erro`, faixas (`--sinal-suave`/`--coral-suave`/`--ciano-suave` + borda esquerda), esqueleto (sem brilho em reduced motion), menus flutuantes, modais (`--sobreposicao`, `--raio-painel`, `--sombra-painel`), visualizador (tela escura), `.tabela*`/`.linha-tabela` (telefone/data mono), `.selo-estado.estado-*` (mapa de estados research R8, "ativo" com lâmpada), `.lista-definicoes`, `.fundo-escuro` → tokens `--tela*`

### Marca

- [X] T016 [P] [US3] Criar `app/src/renderer/componentes/Logo.tsx`: `Tecla({ tamanho })` (SVG viewBox 32 de `zapdesk-site/src/app/icon.svg`, `fill`/`stroke` por `var(--tecla-sombra)`, `var(--sinal)`, `var(--tinta)`, `aria-hidden`) e `Logo({ variante: 'completo' | 'so-tecla', tamanho })` com wordmark "zapdesk" (`--fonte-titulo` 750, `font-variation-settings: "opsz" 24, "wdth" 80`, `letter-spacing: -0.04em`); estilos do wordmark em `base.css` (classe `.wordmark`) — coordenar com T013 (mesmo agente da fundação)
- [X] T017 [P] [US3] Reescrever `app/scripts/gerar-icones.mjs` (research R7, sem dependências): `icone.png` 1024 (tecla com profundidade `#C98500`, corpo `#FFB020`, "Z" `#121417` traço 3/32, corpo 824×824 centrado, transparente fora) e `bandejaTemplate.png` 16 / `bandejaTemplate@2x.png` 32 (silhueta preta da tecla, "Z" vazado); rodar `npm run icones -w @zapdesk/app` e conferir os 3 PNGs com Read
- [X] T018 [P] Em `app/src/main/janela.ts` trocar `backgroundColor` para `#0E1013` (escuro) / `#F4F1E8` (claro) com comentário apontando para `contracts/tokens.md` §4 (exceção 3)

### Checagens

- [X] T019 [P] Criar `app/scripts/contraste.mjs` (research R12, molde `zapdesk-site/scripts/contraste.mjs`): lê `app/src/renderer/estilos/tema.css`, extrai claro/escuro/tela, resolve `color-mix(in srgb, A p%, B)` e `rgb(r g b / a)` sobre o fundo, checa os pares de `data-model.md` §4 (incl. paletas de dados importadas de `telas/Etiquetas.tsx`/`telas/Funis.tsx` por regex e 12 matizes do remetente), imprime tabela e sai 1 em falha
- [X] T020 [P] Criar `app/scripts/verificar-tokens.mjs` (research R13): varre `app/src/renderer/**/*.{css,ts,tsx}`, falha em hex/`rgb(`/`rgba(`/`hsl(` literais fora de `estilos/tema.css` exceto exceções de `contracts/tokens.md` §4 (marcador `// cores-de-dados`, `hsl(` com `var(--remetente-l)`, Status), falha em matiz verde em `tema.css`; opção `--arquivos <lista>` (checar só os arquivos de um bloco) e `--relatorio` (lista sem falhar)
- [X] T021 [P] [US5] Teste `app/tests/renderer/tema.test.tsx` para `util/tema.ts`: padrão `sistema`; salvar `escuro` põe `data-tema="escuro"`; voltar a `sistema` apaga a chave e o atributo; valor inválido = `sistema`; `localStorage` lançando erro não quebra; `temaEfetivo` com `matchMedia` simulado
- [X] T022 Checkpoint da fundação: `npm run tipos -w @zapdesk/app`, `npm test -w @zapdesk/app`, `npm run contraste -w @zapdesk/app` (0 falhas), `npm run tokens -w @zapdesk/app -- --arquivos estilos/tema.css,estilos/fontes.css,estilos/base.css,estilos/controles.css,estilos/estrutura.css,componentes/Logo.tsx` (0) e `--relatorio` geral (quantidade por arquivo de bloco, anotada em pendencias.md); `npm run compilar -w @zapdesk/app` + `node app/scripts/capturar.mjs --saida docs/verificacao/003/fundacao --medir` (alturas de listas iguais ao "antes"; fontes `loaded`; rede externa vazia)

**Checkpoint**: o app inteiro já está na paleta nova (via aliases) com fontes e primitivos finais;
blocos podem começar em paralelo.

---

## Phase 3: Blocos por área em paralelo (B1–B8)

Cada bloco = um agente, dono exclusivo dos arquivos listados no título. Implementação é `[P]`
entre blocos (e entre arquivos diferentes do mesmo bloco). Toda verificação de bloco: checagens da
regra 8 + `npm run compilar -w @zapdesk/app` + `node app/scripts/capturar.mjs --telas <telas do
bloco> --saida docs/verificacao/003/<bloco>` + comparação com o "antes" (mesma disposição e
textos) e com o mockup (identidade).

### Bloco B1 — Shell e lista de conversas (US1) · arquivos: `estilos/shell.css`, `estilos/conversas.css`, `componentes/{BarraLateral,Avatar,SeletorConta,FaixaConta,NovaConversa,ListaVirtual}.tsx`, `telas/{Conversas,BuscaMensagens}.tsx`

- [X] T023 [P] [US1] Restilizar `app/src/renderer/estilos/shell.css` (trilho `--superficie-2` com divisores `--linha`, item ativo `--fundo-2` + `.lampada` à esquerda, `.selo-trilho` âmbar `--tinta` mono com borda `--borda-primario`, `.ponto-estado` conectada `--ciano`/conectando lâmpada/banida `--coral`, `.avatar` neutro) conforme `ChatMockup` → `Trilho`
- [X] T024 [P] [US1] Restilizar `app/src/renderer/estilos/conversas.css` (cabeçalho da lista com seletor de conta, busca `--fundo`, filtros em pílula com ativo invertido, item de conversa: ativo `--fundo-2` + barra âmbar 3 px, nome 13 px, hora mono 10–11 px `--sinal-texto` 600 quando não lida, pílula de não lidas `--sinal`/`--tinta`, divisória `--linha`; busca de mensagens com `mark`) — **alturas dos itens inalteradas**
- [X] T025 [P] [US1] Ajustes decorativos em `app/src/renderer/componentes/Avatar.tsx` (remover `hsl()`: iniciais neutras, foto mantida) e, se o mockup pedir, `<span className="lampada" aria-hidden="true" />` no item ativo de `componentes/BarraLateral.tsx`; nada de texto/aria/ordem
- [X] T026 [US1] Verificar B1: capturas `conversas,busca,nova-conversa,contatos,vazio` → `docs/verificacao/003/b1/`; comparar com `zapdesk-site/docs/verificacao/final/topo-1440-pt-{claro,escuro}.png` (lista/trilho) e com o "antes"; `--medir` alturas dos itens iguais

### Bloco B2 — Conversa aberta (US1) · arquivos: `estilos/conversa.css`, `componentes/{Bolha,Composer,GravadorAudio,MenuMensagem,SeletorFigurinha,SugestoesTemplate,PainelContato,FaixaAutomacoes}.tsx`, `componentes/bolhas/Midia.tsx`, `telas/Chat.tsx`

- [X] T027 [P] [US1] Restilizar a parte "chat" e "menu da mensagem" de `app/src/renderer/estilos/conversa.css`: cabeçalho da conversa (nome 13 px 600, subtítulo mono, chip de etiqueta, botão Info), fundo `--fundo` sem papel de parede, `.linha-mensagem(.minha) .bolha` (minha `--superficie-2` + borda `--linha-forte` + canto sup. dir. 3 px; dele `--superficie` + borda `--linha` + canto sup. esq. 3 px; sem sombra), hora mono 10 px `--texto-2`, vistos (lida `--ciano`), citação `--fundo-2` com barra `--linha-forte`, separador de dia em pílula mono, mensagem de sistema, "editada", menu da mensagem
- [X] T028 [US1] Restilizar o restante de `app/src/renderer/estilos/conversa.css` (depois de T027, mesmo arquivo): mídia (áudio com picos `--texto`/`--linha-forte` e botão play, documento em cartão com extensão mono, imagem/vídeo/figurinha sem filtro, visualizador na tela escura), compositor (barra `--superficie`, campo `--fundo` borda `--linha`, ícones `--texto-2`, gravação com ponto `--coral`), sugestões de template, painel do contato (seções com títulos 11 px 600 `--texto-2`, chips, notas), faixa de automações na conversa (lâmpada + `--sinal-suave`)
- [X] T029 [P] [US1] Em `app/src/renderer/componentes/Bolha.tsx` trocar a cor do remetente para `hsl(${matiz} 55% var(--remetente-l))` (tokens.md §4); conferir `Midia.tsx`, `Composer.tsx`, `PainelContato.tsx` e `Chat.tsx` por cores inline/classes verdes e trocar só a marcação decorativa
- [X] T030 [US1] Verificar B2: capturas `conversa,conversa-menu,status` (só o visualizador) → `docs/verificacao/003/b2/`; lado a lado com `ChatMockup` (topo da landing); fluxo manual no app falso: enviar texto, anexar documento, gravar áudio, responder, apagar, abrir painel do contato — tudo igual ao antes

### Bloco B3 — Disparos e relatório (US2) · arquivos: `estilos/disparos.css`, `telas/{Disparos,DetalheDisparo}.tsx`, `telas/NovoDisparo/*.tsx`

- [X] T031 [P] [US2] Restilizar lista e assistente em `app/src/renderer/estilos/disparos.css`: `.lista-disparos`/`.cartao-disparo` (nome 600, conta mono, selo), stepper `.etapas` (atual: sublinhado 3 px `--sinal`, número sobre `--sinal` `--tinta`, lâmpada; feita: `--ciano` com visto; futura: `--linha`), `.rodape-assistente` (estimativa com valor mono), `.passo`, campos de ritmo (número + unidade), prévia da mensagem (bolha "minha" como B2, sem verde), resumo/fontes/faltando — conforme `DisparoMockup`
- [X] T032 [US2] Restilizar detalhe/relatório em `app/src/renderer/estilos/disparos.css` (depois de T031): `.progresso-topo`, `.percentual` (`--fonte-titulo` 30 px 600), `.barra-progresso` (trilho `--linha`, `.barra-ok` `--sinal`, `.barra-falhou` `--coral`), `.contadores`/`.contador*` (cartões `--superficie` borda `--linha`, número `--fonte-titulo`; lido/respondeu `--ciano`, falhou `--coral`), `.selo-destinatario.estado-*` (R8), `.estado-linha`, `.mensagem-disparo` — conforme `RelatorioMockup`
- [X] T033 [US2] Verificar B3: capturas `disparos,disparo-novo-1,disparo-novo-2,disparo-novo-3,disparo-novo-4,disparo-detalhe` → `docs/verificacao/003/b3/`; lado a lado com `zapdesk-site/docs/verificacao/final/secoes/tour-1440-pt-{claro,escuro}*.png`; percorrer os 4 passos com teclado (foco visível) sem iniciar disparo novo

### Bloco B4 — Leads, contatos, etiquetas, templates, importação (US2) · arquivos: `estilos/crm.css`, `telas/{Leads,Contatos,Etiquetas,Templates,ImportarLeads}.tsx`, `componentes/RelatorioImportacao.tsx`

- [X] T034 [P] [US2] Restilizar `app/src/renderer/estilos/crm.css`: `.lista-simples/.lista-cartoes`, `.item-contato/.item-status` (telefone mono), `.anel-status` (`--sinal`), `.item-template(.ativo)` (ativo com barra âmbar), `.linha-etiqueta`, `.paleta/.amostra-cor(.escolhida)` (anel `--foco`), `.previa-etiqueta` (chip neutro + ponto), editor de template e `.variaveis` (pílulas mono), `.anexo-escolhido`, `.area-arquivo` (borda tracejada `--linha-forte`, hover `--fundo-2`), `.relatorio-importacao/.relatorio-grupo` (ícones: ok `--ciano`, erro `--coral`, info `--sinal-texto`, neutro `--texto-2`), `.leads-grade`
- [X] T035 [P] [US2] Em `app/src/renderer/telas/Etiquetas.tsx` trocar a paleta sugerida `CORES_ETIQUETA` pela de research R9 (sem verde; marcador `// cores-de-dados`; padrão inicial = primeira cor da nova paleta; `input type=color` com fallback = primeira cor) — cores já salvas não mudam; conferir `Leads.tsx`, `Contatos.tsx`, `Templates.tsx`, `ImportarLeads.tsx`, `RelatorioImportacao.tsx` por cor inline/classe verde (só marcação decorativa)
- [X] T036 [US2] Verificar B4: capturas `leads,leads-importar,templates,etiquetas,contatos` → `docs/verificacao/003/b4/`; `npm run contraste` (paleta de dados ≥ 3:1 nos 2 temas); etiqueta verde antiga da semente continua verde

### Bloco B5 — Ajustes, contas, QR, status, carregando/erro, tema manual (US2, US3, US5) · arquivos: `estilos/ajustes.css`, `telas/{Ajustes,ConectarConta,Carregando,ErroMotor,Status}.tsx`, `telas/secoes/{AjustesIA,AjustesAutomacoes}.tsx`, `componentes/TelaErro.tsx`

- [X] T037 [P] [US2] Restilizar `app/src/renderer/estilos/ajustes.css`: `.tela-cheia` (fundo `--fundo` + grade pontilhada `--pontos`, título `--fonte-titulo`), `.logo-grande`, `.barra-indeterminada` (`--sinal`, estática em reduced motion), `.rodape-seguro`, `.erro-motor`; conectar conta (`.conectar-*`, passos numerados mono, `.qr-moldura` `--superficie` borda `--linha-forte` raio 14, `.qr` em `--qr-tinta` sobre `--qr-fundo` com margem — nunca invertido, `.qr-expirado`, `.icone-ok` `--ciano`); ajustes (`.lista-contas/.linha-conta`, `.bloco-codigo` na tela escura com `--tela*`); status (`.visualizador-status`, barras `--tela-texto`); ajustes de automações/IA
- [X] T038 [P] [US3] Em `app/src/renderer/telas/Carregando.tsx` trocar o SVG do balão por `<Logo variante="completo" tamanho={56} />` (o `<h1>ZapDesk</h1>` e os textos ficam; o wordmark do Logo é `aria-hidden`) e conferir `ErroMotor.tsx`/`componentes/TelaErro.tsx` (ícone `--coral`, sem mudança de texto)
- [X] T039 [P] [US5] Em `app/src/renderer/telas/Ajustes.tsx` acrescentar a seção "Aparência" (`<h2>Aparência</h2>`, `role="radiogroup"` com "Sistema", "Claro", "Escuro"; usa `lerPreferencia/salvarPreferencia/aplicarTema` de `util/tema.ts`; troca imediata) depois das seções existentes, sem mudar as outras; teste em `app/tests/renderer/ajustes-aparencia.test.tsx`
- [X] T040 [P] [US2] Em `app/src/renderer/telas/Status.tsx` trocar o fundo do status de texto para `hsl(${matiz} 35% 30%)` sobre `--tela-texto` (tokens.md §4) e conferir `ConectarConta.tsx`, `secoes/AjustesIA.tsx`, `secoes/AjustesAutomacoes.tsx` por cor inline/classe verde
- [X] T041 [US2] Verificar B5: capturas `ajustes,conectar-conta,status,carregando,erro-motor,modal-confirmar` → `docs/verificacao/003/b5/`; QR legível (preto sobre branco) nos 2 temas; trocar Aparência para "Escuro"/"Claro"/"Sistema" no app falso (`--manter`) e conferir troca imediata e persistência após reabrir

### Bloco B6 — Funis e Kanban (US2) · arquivos: `estilos/funil.css`, `telas/Funis.tsx`, `telas/Kanban/*.tsx`

- [X] T042 [P] [US2] Restilizar `app/src/renderer/estilos/funil.css` conforme `KanbanMockup`: lista de funis (`.etapa-resumo` com borda na cor da etapa), `.quadro-kanban`, `.coluna-kanban*` (`--superficie-2`, borda `--linha`, raio 12; cabeçalho com borda superior 4 px na cor inline, ponto, nome 13 px 600, contagem mono, "+"), `.cartao-kanban` (`--superficie`, borda `--linha`, `box-shadow: 0 1px 0 var(--linha)`, nome 600, telefone mono, chips, "há 2 h" `--texto-2`, ações ⓘ/⋮), arrastando (`--sombra-painel`, borda `--linha-forte`), alvo de soltura (`--sinal-suave` tracejado), painel do cartão e editar etapas
- [X] T043 [P] [US2] Em `app/src/renderer/telas/Funis.tsx` trocar `CORES_ETAPA` pela paleta de research R9 (`// cores-de-dados`; fallback = primeira cor) — `Kanban/EditarEtapas.tsx` já usa a constante; conferir `Kanban/*.tsx` por cor inline/classe verde (cores de etapa do usuário continuam inline)
- [X] T044 [US2] Verificar B6: capturas `funis,kanban` → `docs/verificacao/003/b6/`; lado a lado com `secoes/tour-*` (aba Funil); arrastar um cartão no app falso e usar "Mover para…" pelo teclado — comportamento igual

### Bloco B7 — Automações: lista, editor de fluxo, execuções e chatbot (US2) · arquivos: `estilos/automacoes.css`, `telas/{Automacoes,NovaAutomacao,EditorAutomacao,Execucoes,DetalheExecucao}.tsx`, `telas/EditorFluxo/*`, `telas/EditorChatbot/**`, `componentes/ResultadoExecucao.tsx`

- [X] T045 [P] [US2] Restilizar lista, editores e execuções em `app/src/renderer/estilos/automacoes.css`: cartões de automação (tipo em rótulo mono, ativa com lâmpada/interruptor, ícones de tipo — trocar `#c98a00`/`#8b64d6`/`#b07800` por `--sinal-texto`/`--ciano`/`--texto-2`), editor de fluxo (gatilho → condições → ações como blocos ligados por cabo vertical `--linha-forte` com `.terminal-cabo`), execuções (selos R8, log em mono na tela escura)
- [X] T046 [US2] Restilizar o chatbot em `app/src/renderer/estilos/automacoes.css` (depois de T045) conforme `ChatbotMockup`: variáveis do React Flow (`--xy-background-color` `--fundo`, `--xy-background-pattern-color` `--pontos` gap 24, `--xy-edge-stroke` `--linha-forte`, `--xy-edge-stroke-selected` `--sinal`, `--xy-handle-background-color` `--fundo`, `--xy-handle-border-color` `--linha-forte`, `--xy-node-*`, controles e minimapa), nós (cartão `--superficie` raio 10, cabeçalho `--superficie-2` mono 11 px com ícone; selecionado borda `--linha-forte` + ícone `--sinal-texto`), terminais redondos 7 px, painel do nó e chat simulado (bolhas iguais às de B2)
- [X] T047 [P] [US2] Em `app/src/renderer/telas/EditorChatbot/EditorChatbot.tsx` passar `colorMode={useTemaEfetivo() === 'escuro' ? 'dark' : 'light'}` ao `<ReactFlow>` e `<Background gap={24} size={1} />`; conferir `EditorChatbot/nos/*`, `EditorFluxo/*`, `Automacoes.tsx`, `ResultadoExecucao.tsx` por cor inline/classe verde (só marcação decorativa)
- [X] T048 [US2] Verificar B7: capturas `automacoes,editor-fluxo,editor-chatbot,execucoes` → `docs/verificacao/003/b7/`; lado a lado com `secoes/automations-1440-pt-{claro,escuro}*.png`; no app falso: ligar/desligar automação, mover nó, ligar cabo, simular conversa — igual ao antes

### Bloco B8 — Editor de IA (US2, US5) · arquivos: `estilos/editor-ia.css`, `telas/EditorIA/*`, `monaco/configurar.ts`

- [X] T049 [P] [US2] Restilizar `app/src/renderer/estilos/editor-ia.css`: árvore de arquivos (`--superficie-2`, arquivo ativo `--fundo-2` + barra âmbar, nomes mono), abas de arquivo mono, área do editor na tela escura (`--tela`, borda `--tela-linha`), painel de teste (entrada como campo, saída/log em mono na tela escura com `--tela-ciano`/`--tela-coral`), barra de compilação (ok `--ciano`, falhou `--coral`), permissões/gatilhos como pílulas mono (como a landing em "automacao.json"); trocar `#fff` literal
- [X] T050 [P] [US5] Em `app/src/renderer/monaco/configurar.ts` definir o tema Monaco `zapdesk-tela` (research R14: base `vs-dark`, cores lidas de `getComputedStyle(document.documentElement)` — `--tela`, `--tela-2`, `--tela-linha`, `--tela-texto`, `--tela-texto-2`, `--tela-sinal`, `--tela-ciano`, `--tela-coral`; keyword/storage sinal, string ciano, number coral, comment texto-2 itálico; fonte `JetBrains Mono Variable`) e em `telas/EditorIA/Codigo.tsx` usar `theme="zapdesk-tela"` reaplicando com `useTemaEfetivo()` (remover `escuro()` por `matchMedia`)
- [X] T051 [US2] Verificar B8: captura `editor-ia` → `docs/verificacao/003/b8/`; lado a lado com o bloco de código de `secoes/automations-*`; no app falso: abrir arquivo, provocar erro de compilação (sublinhado visível), rodar "Testar" com IA simulada

**Checkpoint**: todos os blocos verificados individualmente; pendências anotadas.

---

## Phase 4: Integração (sequencial, um agente)

- [X] T052 Resolver `specs/003-identidade-visual/pendencias.md`: tokens/variações faltantes vão para `tema.css`/primitivos; remover os ajustes locais que os blocos fizeram para contornar; registrar a resolução ao lado de cada linha
- [X] T053 Remover a camada de aliases de `app/src/renderer/estilos/tema.css` depois de confirmar com `grep -rnE "var\(--(verde|verde-forte|verde-suave|azul-lido|perigo|aviso-|erro-fundo|erro-texto|info-|fundo-app|fundo-painel|fundo-cabecalho|fundo-trilho|fundo-hover|fundo-selecionado|fundo-campo|fundo-chat|bolha-|sombra-bolha|sombra-menu|texto-3|texto-inverso|borda|esqueleto|raio\b)" app/src/renderer` = 0 (trocar no arquivo dono o que restar)
- [X] T054 Passada de consistência entre áreas (mesmo selo, mesmo cabeçalho de tela, mesmo raio e espaçamento de cartões, mesmos rótulos mono): corrigir no arquivo dono; nenhuma regra nova fora dele
- [X] T055 Capturas **depois** completas e medidas: `npm run compilar -w @zapdesk/app` + `node app/scripts/capturar.mjs --saida docs/verificacao/003/depois --medir`

---

## Phase 5: Verificação

- [X] T056 [P] Rodar `npm run tipos -w @zapdesk/app`, `npm test -w @zapdesk/app` (todos os testes antigos sem mudar expectativa + `tema.test.tsx` + `ajustes-aparencia.test.tsx` + `pastas.test.ts`), `npm run contraste -w @zapdesk/app` (0 falhas, SC-003) e `npm run tokens -w @zapdesk/app` sem `--arquivos` (0 literais, 0 verdes, SC-002)
- [X] T057 [P] Revisão visual tela a tela (SC-001, SC-008): para cada PNG de `docs/verificacao/003/depois/` abrir com Read o par do "antes" e a referência da landing (`zapdesk-site/docs/verificacao/final/topo-*`, `secoes/tour-*`, `secoes/automations-*`); conferir mesma disposição e textos, nada verde, âmbar só como preenchimento/lâmpada/foco, estados com as cores certas; achados em pendencias.md com prefixo `V:` e o bloco dono
- [X] T058 [P] Acessibilidade (US4, SC-007): com `capturar.mjs --manter` no app falso, Tab por conversas, novo disparo (4 passos), Kanban, editor de chatbot e Ajustes conferindo o anel de foco; `reducedMotion: 'reduce'` sem animação decorativa; capturas extras do foco em `docs/verificacao/003/foco/`
- [X] T059 [P] Desempenho e offline (SC-005, SC-006, FR-025): `medidas.json` de "depois" com `layoutShift == 0`, fontes `loaded`, `redeExterna == []`, alturas de itens virtuais iguais às do "antes"; `ls -la app/out/renderer/assets/*.woff2` = só os 4 arquivos latin (~290 KB); nenhum `fonts.googleapis`/`gstatic` em `app/out/`
- [X] T060 [P] Prova de "só visual" (FR-026, SC-004): `diff -ru ~/.cache/zapdesk-003-base/src app/src` e revisar cada hunk em `.ts/.tsx` — permitido só className, marcação decorativa, Logo, cores→tokens, paletas de dados, `util/tema.ts`/`util/fontes.ts`/seção Aparência/`colorMode`/tema Monaco, `pastas.ts`/`index.ts` (userData no modo falso) e `janela.ts`; qualquer outra mudança volta ao original
- [X] T061 Corrigir tudo o que T056–T060 apontarem (cada correção no arquivo do bloco dono; compartilhados na integração) e repetir a verificação afetada

---

## Phase 6: Entrega

- [X] T062 Empacotar: `npm run empacotar` (raiz → `scripts/empacotar.sh`, que já roda `gerar-icones.mjs`) → `app/dist/ZapDesk-0.1.0-arm64.dmg` e `app/dist/mac-arm64/ZapDesk.app`; conferir o ícone novo no `.app` (Quick Look do `Contents/Resources/icon.icns`) e que `Contents/Resources/app.asar` contém os 4 woff2
- [X] T063 Instalar só se fechado: `pgrep -x ZapDesk` — se **não** houver processo, `rm -rf /Applications/ZapDesk.app && ditto app/dist/mac-arm64/ZapDesk.app /Applications/ZapDesk.app` e abrir uma vez para conferir ícone no Dock e visual (sem mexer em dados); se **houver**, NÃO fechar nem substituir: avisar o usuário que o `.dmg` está em `app/dist/` e que basta fechar o app e arrastar o novo para Aplicativos
- [X] T064 Fechar a feature: marcar tarefas, resumo final em `pendencias.md` (o que ficou fora, decisões tomadas na implementação) e atualizar `quickstart.md` se algum comando mudou

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Fase 1)**: T001 → T002 → (T003, T004, T005, T006 em paralelo) → **T007 (captura "antes") obrigatório antes de qualquer mudança de CSS**.
- **Fundação (Fase 2)**: T008 → T009 (prova da divisão) → T010 → (T011, T014, T015, T016, T017, T018, T019, T020, T021 em paralelo; T012 depois de T011; T013 depois de T010+T011) → **T022 fecha a fase e bloqueia todos os blocos**.
- **Blocos B1–B8 (Fase 3)**: dependem só de T022; **independentes entre si**. Dentro do bloco, tarefas no mesmo arquivo são sequenciais (T027→T028, T031→T032, T045→T046); a verificação do bloco depende só das tarefas do próprio bloco.
- **Integração (Fase 4)**: depois de todos os blocos; T052 → T053 → T054 → T055.
- **Verificação (Fase 5)**: depois de T055; T056–T060 em paralelo; T061 por último (repete o que falhou).
- **Entrega (Fase 6)**: depois de T061; T062 → T063 → T064.

### Mapa bloco → arquivos → user story

| Bloco | Tarefas | Arquivos (dono exclusivo) | Telas de captura | US |
|---|---|---|---|---|
| B1 Shell + lista | T023–T026 | `estilos/shell.css`, `estilos/conversas.css`, `componentes/{BarraLateral,Avatar,SeletorConta,FaixaConta,NovaConversa,ListaVirtual}.tsx`, `telas/{Conversas,BuscaMensagens}.tsx` | conversas, busca, nova-conversa, contatos, vazio | US1 |
| B2 Conversa | T027–T030 | `estilos/conversa.css`, `componentes/{Bolha,Composer,GravadorAudio,MenuMensagem,SeletorFigurinha,SugestoesTemplate,PainelContato,FaixaAutomacoes}.tsx`, `componentes/bolhas/Midia.tsx`, `telas/Chat.tsx` | conversa, conversa-menu | US1 |
| B3 Disparos | T031–T033 | `estilos/disparos.css`, `telas/{Disparos,DetalheDisparo}.tsx`, `telas/NovoDisparo/*` | disparos, disparo-novo-1..4, disparo-detalhe | US2 |
| B4 CRM | T034–T036 | `estilos/crm.css`, `telas/{Leads,Contatos,Etiquetas,Templates,ImportarLeads}.tsx`, `componentes/RelatorioImportacao.tsx` | leads, leads-importar, templates, etiquetas | US2 |
| B5 Ajustes/contas/telas cheias | T037–T041 | `estilos/ajustes.css`, `telas/{Ajustes,ConectarConta,Carregando,ErroMotor,Status}.tsx`, `telas/secoes/*`, `componentes/TelaErro.tsx` | ajustes, conectar-conta, status, carregando, erro-motor, modal-confirmar | US2, US3, US5 |
| B6 Funil | T042–T044 | `estilos/funil.css`, `telas/Funis.tsx`, `telas/Kanban/*` | funis, kanban | US2 |
| B7 Automações + chatbot | T045–T048 | `estilos/automacoes.css`, `telas/{Automacoes,NovaAutomacao,EditorAutomacao,Execucoes,DetalheExecucao}.tsx`, `telas/EditorFluxo/*`, `telas/EditorChatbot/**`, `componentes/ResultadoExecucao.tsx` | automacoes, editor-fluxo, editor-chatbot, execucoes | US2 |
| B8 Editor de IA | T049–T051 | `estilos/editor-ia.css`, `telas/EditorIA/*`, `monaco/configurar.ts` | editor-ia | US2, US5 |

Compartilhados (só leitura para blocos): `estilos/{tema,fontes,base,controles,estrutura}.css`,
`main.tsx`, `App.tsx`, `rotas.tsx`, `util/*`, `componentes/{Logo,Modal,EstadoVazio,Esqueleto,FaixaAviso,BotaoIcone}.tsx`,
`app/src/main/*`, `app/src/preload/*`, `app/scripts/*`, `package.json`.
Recado comum: `specs/003-identidade-visual/pendencias.md` (append-only, prefixo do bloco).

### User Story Dependencies

- **US1** (B1, B2) e **US2** (B3–B8) dependem só da fundação; podem ser entregues separadas, mas a
  feature só é entregue (Fase 6) com as duas — identidade aplicada pela metade parece quebra.
- **US3** (logotipo): T016/T017 na fundação + T038 em B5.
- **US4** (AA, foco, reduced motion, CLS): fundação (T013, T019) + verificação T056/T058/T059.
- **US5** (tema manual): T012/T021 na fundação + T039 (B5) + T047 (B7) + T050 (B8).

### Parallel Opportunities

- Fase 1: T003, T004, T005, T006.
- Fase 2: depois de T010 — T011, T014, T015, T016, T017, T018, T019, T020, T021.
- Fase 3: **8 agentes** (B1–B8) ao mesmo tempo; dentro de cada bloco, CSS e TSX em paralelo.
- Fase 5: T056–T060.

## Parallel Example: Fase 3

```text
# depois do checkpoint T022, um agente por bloco:
Agente B1: T023 shell.css + T024 conversas.css + T025 Avatar/BarraLateral → T026
Agente B2: T027 → T028 conversa.css + T029 Bolha.tsx → T030
Agente B3: T031 → T032 disparos.css → T033
Agente B4: T034 crm.css + T035 Etiquetas.tsx → T036
Agente B5: T037 ajustes.css + T038 Carregando + T039 Aparência + T040 Status → T041
Agente B6: T042 funil.css + T043 Funis.tsx → T044
Agente B7: T045 → T046 automacoes.css + T047 EditorChatbot.tsx → T048
Agente B8: T049 editor-ia.css + T050 Monaco → T051
```

## Implementation Strategy

### MVP (US1)

Fase 1 → Fase 2 → B1 + B2 → capturas de conversas aprovadas pelo dono. Mesmo no MVP o resto do
app já está na paleta nova pelos aliases (consistente, só não refinado).

### Paralelo (recomendado)

Fundação com um agente (é o caminho crítico: captura "antes", divisão provada, tokens, primitivos).
Depois 8 agentes (B1–B8) com arquivos disjuntos, integração com um agente, verificação em paralelo,
entrega com um agente (é ela que decide reinstalar ou só deixar o `.dmg`).

## Notes

- O "antes" (T007) é insubstituível: se alguém mexer no CSS antes dele, refazer a partir da base
  em `~/.cache/zapdesk-003-base/`.
- Captura/compilação compartilham `app/out/`: compilar e capturar em sequência, sem outro agente
  compilando ao mesmo tempo (combinar pela pendencias.md se preciso).
- Nada de commit; o dono revisa e comita.
