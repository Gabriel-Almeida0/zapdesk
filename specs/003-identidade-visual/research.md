# Research — 003 Identidade visual

Fontes lidas: `zapdesk-site/src/app/globals.css`, `src/lib/fontes.ts`, `src/app/icon.svg`,
`src/components/ui/{Logo,MockWindow,Etiqueta}.tsx`, `src/components/mockups/*.tsx`,
`scripts/{contraste,gerar-icones,capturar}.mjs`, `specs/001-landing-zapdesk/tasks.md`, capturas em
`docs/verificacao/final/` (topo, tour, automations); no app: `estilos/{tema,app,automacoes}.css`,
`main.tsx`, `index.html` (CSP), `main/{index,janela,bandeja}.ts`, `scripts/gerar-icones.mjs`,
`scripts/empacotar.sh`, `electron-builder.yml`, `specs/001-zapdesk-mvp/contracts/runtime.md` (modo
falso), `scripts/quickstart-002.mjs`.

## R1. Tokens: copiar os valores da landing, acrescentar só derivados

- **Decision**: `tema.css` passa a ter exatamente os tokens da landing (mesmos nomes e valores, claro
  em `:root`, escuro em `@media (prefers-color-scheme: dark) { :root:not([data-tema="claro"]) }` e
  em `:root[data-tema="escuro"]`), mais derivados do app via `color-mix()` dos próprios tokens
  (`--sinal-suave`, `--coral-suave`, `--ciano-suave`, `--sobreposicao`, `--sombra-painel`,
  `--sombra-menu`) e tokens de forma/tipo (`--raio-tecla: 10px`, `--raio-painel: 14px`,
  `--raio-pilula: 999px`, `--fonte-titulo`, `--fonte`, `--fonte-mono`). Tabela completa em
  `contracts/tokens.md`.
- **Rationale**: "mesma identidade" = mesmos valores; derivados por `color-mix` não criam cores
  novas e seguem o tema sozinhos.
- **Alternatives**: paleta própria "inspirada" (diverge da landing com o tempo); `light-dark()`
  (exigiria `color-scheme` por elemento e complica o tema manual).
- **Nota**: atributo `data-tema` (pt-BR) no `<html>` em vez do `data-theme` da landing (Princípio
  VII); valores `claro`/`escuro`.

## R2. Camada de transição: aliases dos tokens antigos

- **Decision**: na fundação, os 40 nomes antigos (`--verde`, `--fundo-painel`, `--bolha-minha`,
  `--texto-3`, `--borda`…) viram aliases `var(--novo)` (mapa em `contracts/tokens.md` §3). O app
  inteiro troca de paleta de uma vez sem quebrar nada; cada bloco reescreve o seu arquivo usando só
  nomes novos; a integração apaga os aliases quando `grep` não encontrar mais uso.
- **Rationale**: permite blocos em paralelo sem estado intermediário quebrado.
- **Alternatives**: busca-e-troca global na fundação (conflitaria com o trabalho de cada bloco).

## R3. Divisão mecânica do CSS antes de qualquer mudança visual

- **Decision**: a fundação divide `app.css` e `automacoes.css` em 13 arquivos por área (mapa no
  plan.md) e `main.tsx` os importa numa ordem que preserva a ordem relativa original das regras
  (base → controles → estrutura → shell → conversas → conversa → crm → disparos → ajustes → funil
  → automacoes → editor-ia; regras movidas de posição são conferidas uma a uma quanto a seletores
  concorrentes). Prova: `capturar.mjs --comparar docs/verificacao/003/antes` com 0 pixels
  diferentes (tolerância só nas áreas de relógio, ver R11).
- **Rationale**: blocos paralelos sem sobreposição de arquivo; prova objetiva de que a divisão não
  mudou nada.
- **Alternatives**: CSS Modules por componente (refatoração grande de TSX, viola "só visual").

## R4. Fontes empacotadas (offline), subset latin, sem layout shift

- **Decision**: `@fontsource-variable/*@5.3.0` como devDependencies; `estilos/fontes.css` declara
  `@font-face` próprios apontando só para os arquivos latin:
  `bricolage-grotesque-latin-standard-normal.woff2` (eixos wght+opsz+wdth, 131 KB; a landing usa
  `opsz` e `wdth`), `instrument-sans-latin-standard-normal.woff2` (wght+wdth, 57 KB) +
  `...-standard-italic.woff2` (62 KB), `jetbrains-mono-latin-wght-normal.woff2` (40 KB), com
  `unicode-range` latin, `font-display: block`. O Vite resolve as URLs (`@fontsource-variable/.../files/...`)
  e copia só esses 4 arquivos para `out/renderer/assets` (servidos por `self`: CSP inalterada).
  `main.tsx` aguarda `document.fonts.load()` das três famílias com teto de 500 ms antes do
  `createRoot().render()` — a janela só aparece no `ready-to-show`, então não há troca visível.
- **Rationale**: latin cobre pt-BR (á, ç, ã, õ…); arquivo local carrega em milissegundos; esperar
  antes do render elimina FOUT/CLS; sem script inline (CSP `script-src 'self'`).
- **Alternatives**: importar `index.css` do fontsource (empacota cirílico/vietnamita/latin-ext —
  peso inútil); Google Fonts (rede, viola Princípio I); `font-display: swap` (troca visível).
- **Pilhas**: `--fonte-titulo: 'Bricolage Grotesque Variable', ui-sans-serif, system-ui, sans-serif`;
  `--fonte: 'Instrument Sans Variable', -apple-system, system-ui, sans-serif`;
  `--fonte-mono: 'JetBrains Mono Variable', ui-monospace, SFMono-Regular, Menlo, monospace`.
  Recursos da landing: títulos `font-variation-settings: "opsz" 96, "wdth" 88–90`, peso 650–750,
  `letter-spacing` negativo; rótulos mono 12 px, `letter-spacing: .08em`, caixa alta,
  `font-variant-ligatures: none`.

## R5. Alturas de listas virtualizadas e métricas de fonte

- **Decision**: toda linha de lista virtualizada (conversas, leads, contatos, destinatários,
  execuções) mantém a **altura fixa atual** em px; `line-height` explícito em px nos itens. A
  fundação mede (em `capturar.mjs --medir`) a altura dos itens antes/depois e falha se mudar.
- **Rationale**: Instrument Sans tem métricas diferentes da SF; `ListaVirtual` usa tamanhos
  estimados — mudar altura sobrepõe itens.

## R6. Tema: sistema por padrão, manual opcional, sem IPC nova

- **Decision**: `util/tema.ts` com `PreferenciaTema = 'sistema' | 'claro' | 'escuro'` em
  `localStorage['zapdesk.tema']` (try/catch); `aplicarTema()` põe/remove `data-tema` no
  `<html>`; `temaEfetivo()` combina preferência + `matchMedia('(prefers-color-scheme: dark)')`;
  `useTemaEfetivo()` (hook com `useSyncExternalStore`) para Monaco e React Flow. Chamado em
  `main.tsx` antes do render. Ajustes ganha a seção "Aparência" (3 opções em `role="radiogroup"`).
- **Rationale**: preferência da janela (Complexity Tracking); não precisa de `nativeTheme` nem IPC
  (preload mínimo). O `backgroundColor` da janela segue o sistema — só é visível antes do primeiro
  paint.
- **Alternatives**: `nativeTheme.themeSource` via IPC (nova ponte no preload, mais superfície);
  guardar no motor (rota + MCP para nada).

## R7. Logotipo, ícone do app e template da barra de menu sem dependências

- **Decision**: reescrever `app/scripts/gerar-icones.mjs` mantendo o rasterizador próprio (PNG com
  zlib, supersampling) e desenhar a geometria de `zapdesk-site/src/app/icon.svg` (viewBox 32):
  retângulo `rx 8` `#C98500` (profundidade) + retângulo 32×29 `rx 8` `#FFB020` + polilinha "Z"
  `M9 8h14L14.5 14.5H20L9 21h14`, traço 3, junção em quina, cor `#121417`. Rasterização do traço
  por distância ao segmento ≤ 1,5 com quinas preenchidas (ou polígono equivalente).
  `icone.png` 1024×1024: tecla ocupando o grid de ícones do macOS (corpo 824×824 centrado, margem
  100 px, sombra suave opcional), transparente fora. `bandejaTemplate.png` 16×16 e `@2x` 32×32:
  silhueta preta da tecla (sem a faixa de profundidade) com o "Z" **vazado** (alfa 0), traço
  ajustado ao pixel (≥ 1,5 px em 16).
  `componentes/Logo.tsx`: `Tecla` (SVG com `fill: var(--tecla-sombra)`, `var(--sinal)`,
  `stroke: var(--tinta)`) + wordmark "zapdesk" (`--fonte-titulo`, peso 750, `wdth` 80,
  `letter-spacing: -0.04em`, minúsculas).
- **Rationale**: mantém o script sem dependência (o app não tem `sharp`); cores literais do ícone
  ficam só no script gerador (exceção listada no verificador de tokens).
- **Alternatives**: `sharp`/`resvg` (dependência nova só para isso); `nativeImage` com SVG (não
  rasteriza SVG de forma confiável no macOS).

## R8. Estados e vocabulário (receitas)

- **Decision**: receitas por componente em `contracts/vocabulario-visual.md`, tiradas dos
  mockups: botão primário (`--sinal` + `--tinta` + contorno `--borda-primario`), secundário (borda
  `--linha-forte`), perigo (texto/borda `--coral`); campo (fundo `--superficie`, borda
  `--linha-forte`, raio 6–10); selo/pílula (h 20–24, `--raio-pilula`, borda por tom); chip de
  etiqueta (pílula neutra + ponto 6 px na cor do usuário com contorno `--linha-forte`); lâmpada (8 px
  `--sinal`); terminal de cabo (7 px, borda 1,5 `--linha-forte`, ativo `--sinal`); stepper; bolhas;
  contadores.
- **Mapa de estados**: positivo (lido, respondeu, concluído, conectado, ok) → `--ciano`; falha
  (falhou, erro, banida, desconectada, compilação falhou) → `--coral`; ativo/andamento (enviando,
  em andamento, ativa, conectando) → lâmpada `--sinal` + texto `--texto`; neutro (pendente,
  enviado, entregue, agendado, na fila, fora da janela, pausado, cancelado, inativa) → `--texto-2`
  + borda `--linha`.

## R9. Cores de dados do usuário

- **Decision**: `etiqueta.cor` / `etapa.cor` continuam inline (`style={{ background }}`) só em
  pontos/bordas; pontos ganham `box-shadow: 0 0 0 1px var(--linha-forte)` (inset) para não sumir.
  Paletas sugeridas (`CORES_ETIQUETA` em `Etiquetas.tsx`, `CORES_ETAPA` em `Funis.tsx`) trocam o
  verde por tons da identidade, nesta ordem (candidatos): âmbar `#FFB020`, ciano `#2A9BB0`,
  coral `#E2603C`, grafite `#737B88`, ocre `#C98500`, ardósia `#5B6F9A`, vinho `#9B3D5A`, oliva
  `#8A8530` (valores finais validados como "ui ≥ 3:1" sobre `--superficie` nos dois
  temas pelo `contraste.mjs`, seção "paleta de dados"). Ficam marcadas como **cores de dados** (exceção
  do verificador de tokens). Cores já salvas não mudam.
- **Rationale**: preserva dados (FR-016) e garante contraste independente da escolha do usuário.

## R10. Avatares, remetente em grupo, status

- **Decision**: `Avatar.tsx` sem `hsl()`: iniciais `--texto-2` em `--fonte-titulo` sobre
  `--superficie-2` com borda `--linha` (foto, quando houver, continua). Remetente em grupo
  (`Bolha.tsx`): mantém matiz por pessoa, `hsl(h 55% var(--remetente-l))` com `--remetente-l: 34%`
  no claro e `72%` no escuro (checado no `contraste.mjs` para 12 matizes). Status (texto colorido em
  `Status.tsx`): fundo `hsl(h 35% 30%)` na tela escura com texto `--tela-texto` (≥ 4,5:1 checado).
- **Rationale**: mockup usa avatar neutro; remetentes coloridos ajudam a ler grupos.

## R11. Capturas sem WhatsApp real (Playwright `_electron`)

- **Decision**: `app/scripts/capturar.mjs` importa `_electron` de
  `$PLAYWRIGHT_PATH` e lança o Electron do
  workspace (`require.resolve('electron')` a partir de `app/`) com `args: [<app>]` e
  `env: { ZAPDESK_DEV_FALSO: '1', ZAPDESK_PASTA_DADOS: <tmp>, ZAPDESK_PASTA_INTERFACE: <tmp> }`
  sobre o build `app/out/` (`npm run compilar -w @zapdesk/app`) e o motor `motor/bin/zapdesk-motor`.
  Espera `runtime.json` na pasta temporária (porta + token), roda `semente-falsa.mjs` contra a API
  (`/v1/contas` + `/v1/falso/*`), navega por `location.hash` (HashRouter), troca o tema por
  `electronApp.evaluate(({ nativeTheme }, t) => { nativeTheme.themeSource = t }, 'dark'|'light')`
  (funciona no "antes", que não tem tema manual), fixa o relógio (`PUT /v1/falso/relogio`) e
  mascara áreas de hora relativa na comparação. Tamanhos 1280×820 (padrão) e 960×620 (mínimo).
  Comparação pixel a pixel feita no próprio Electron (`nativeImage.createFromPath().toBitmap()`),
  sem dependência nova.
- **Modo falso e a trava de instância única**: hoje `userData` é sempre
  `~/Library/Application Support/ZapDesk/interface`; com o app instalado aberto, a instância de
  teste perde `requestSingleInstanceLock()` e sai. Mudança: `main/index.ts` usa
  `ZAPDESK_PASTA_INTERFACE` se definida, e no modo falso usa uma subpasta temporária. Dados reais
  nunca são lidos (pasta de dados do motor também temporária).
- **Tela de erro do motor**: `ZAPDESK_MOTOR_BIN=/usr/bin/false` → captura de `ErroMotor`.
  Carregando: capturada com `ZAPDESK_MOTOR_BIN` apontando para um wrapper que atrasa o `pronto`
  (script de 3 linhas criado pelo `capturar.mjs` em tmp).
- **Alternatives**: renderer no navegador via Vite (o renderer depende de `window.zapdesk` do
  preload e do IPC `fetch-ipc` — exigiria simular a ponte inteira); Playwright MCP (proibido pelo
  padrão da landing; não pilota Electron).

## R12. Script de contraste lendo o próprio `tema.css`

- **Decision**: `app/scripts/contraste.mjs` (molde do da landing) **lê `tema.css`** e extrai os
  blocos claro/escuro/tela (sem duplicar valores à mão), resolve `color-mix` simples
  (`color-mix(in srgb, A p%, B)`), e checa os pares de `data-model.md` §4 (texto 4,5; grande/ui 3),
  com pares "proibidos" documentados (âmbar como texto no claro; foco âmbar sobre botão âmbar no
  escuro). Sai 1 em falha.
- **Rationale**: um token mudou → a checagem acompanha sozinha.

## R13. Verificador de tokens

- **Decision**: `app/scripts/verificar-tokens.mjs` varre `app/src/renderer/**/*.{css,ts,tsx}` e
  falha em `#rgb/#rrggbb/#rrggbbaa`, `rgb(`/`rgba(`/`hsl(` literais fora de `estilos/tema.css`.
  Exceções por arquivo+marcador: paletas de dados (`// cores-de-dados`), `hsl()` com
  `var(--remetente-l)`, QR (`--qr-*` definidos em tema.css), `monaco/configurar.ts` só lendo
  `getComputedStyle` (sem literais). Também falha em qualquer verde (matiz 90°–170° com saturação
  > 25%) dentro de `tema.css`.

## R14. Monaco e React Flow seguindo os tokens

- **Decision**: `monaco/configurar.ts` define `zapdesk-tela` (base `vs-dark`) com cores lidas de
  `getComputedStyle(document.documentElement)` (`--tela`, `--tela-2`, `--tela-texto`,
  `--tela-texto-2`, `--tela-sinal`, `--tela-ciano`, `--tela-coral`, `--tela-linha`) — mesmo tema nos
  dois modos (a landing usa tela escura em ambos); regras de token: keyword/storage → sinal,
  string → ciano, number → coral, comment → texto-2 itálico. `Codigo.tsx` passa a usar
  `useTemaEfetivo()` só para reaplicar o tema quando os tokens mudarem. React Flow: variáveis
  `--xy-edge-stroke`, `--xy-handle-*`, `--xy-background-pattern-color`, `--xy-node-*` mapeadas para
  tokens em `automacoes.css`; `colorMode` do `<ReactFlow>` ligado ao tema efetivo.
- **Rationale**: Monaco não lê variáveis CSS (precisa de hex no `defineTheme`), mas lê-las em
  runtime evita literais e acompanha o tema.
