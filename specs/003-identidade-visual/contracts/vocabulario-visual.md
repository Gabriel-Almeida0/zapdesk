# Contrato — vocabulário visual (receitas por componente)

Tirado dos mockups da landing (`zapdesk-site/src/components/mockups/*.tsx`,
`src/components/ui/{MockWindow,Etiqueta,Logo}.tsx`) e traduzido para as classes que o app **já
tem** (nenhuma classe é renomeada; blocos só mudam as declarações; nomes com `*` são famílias —
conferir os seletores exatos no arquivo da área). "Dono" = arquivo que define a
receita; os outros usam a classe pronta e **não a sobrescrevem** (faltou variação → `pendencias.md`).

## Primitivos (fundação)

| Receita | Classes do app | Como fica | Dono |
|---|---|---|---|
| Tipografia | `body`, `h1–h3`, `.texto-secundario`, `.mono`, `.pre`, `code` | corpo `--fonte` 14 px/1.5; `h1` de tela `--fonte-titulo` 650, `opsz` 24, −0,01em; `.mono`/horas/telefones `--fonte-mono` sem ligaduras | `base.css` |
| Rótulo mono | (nova utilitária `.rotulo`) | 11–12 px, caixa alta, `letter-spacing .08em`, `--texto-2` | `base.css` |
| Foco | `:focus-visible` | `outline: 2px solid var(--foco); outline-offset: 2px` (3 px fora de listas) | `base.css` |
| Seleção | `::selection`, `mark` | `--sinal` + `--tinta`; `mark` = `--sinal-suave` | `base.css` |
| Botão primário | `.botao` (padrão/primário) | `--sinal`, texto `--tinta` 600, borda 1,5 px `--borda-primario`, raio `--raio-tecla`, hover `--sinal-hover` | `controles.css` |
| Botão secundário | `.botao.secundario` | transparente, borda `--linha-forte`, texto `--texto` | `controles.css` |
| Botão perigo | `.botao.perigo` | texto e borda `--coral`; hover `--coral-suave` | `controles.css` |
| Botão ícone | `.botao-icone` | 32 px, `--texto-2`, hover `--fundo-2`, ativo `--texto` | `controles.css` |
| Campo | `input`, `textarea`, `select`, `.campo` | `--superficie`, borda 1 px `--linha-forte`, raio 8, placeholder `--texto-2`; números/horas em mono | `controles.css` |
| Grupo de campos | `.grupo-campos` (`fieldset/legend`) | borda `--linha`, raio 10, legenda 12 px 600 `--texto` | `controles.css` |
| Interruptor | `.interruptor` | trilho `--linha-forte`; ligado `--sinal` com borda `--borda-primario`; bolinha `--superficie` | `controles.css` |
| Abas | `.abas`, `.aba`, `.aba.ativa` | sublinhado 2–3 px; ativa `--texto` 600 + sublinhado `--sinal` + lâmpada; inativa `--texto-2` | `controles.css` |
| Cartão | `.cartao` | `--superficie-2` ou `--superficie`, borda `--linha`, raio 10, sem sombra | `estrutura.css` |
| Cabeçalho de tela | `.cabecalho-tela` | título `--fonte-titulo`; voltar = botão ícone; ações à direita | `estrutura.css` |
| Selo de estado | `.selo-estado.estado-*` | pílula h 22, 11–12 px 500; tom por estado (R8); "ativo" com lâmpada | `estrutura.css` |
| Tabela | `.tabela`, `.linha-tabela`, `.tabela-virtual` | cabeçalho `--superficie-2` 11 px 600 `--texto-2`; linhas com `--linha`; telefone/data mono | `estrutura.css` |
| Lista de definições | `.lista-definicoes` | `dt` `--texto-2` 12 px; `dd` `--texto` | `estrutura.css` |
| Faixas | `.faixa-aviso/-erro/-info` | fundo `--sinal-suave`/`--coral-suave`/`--ciano-suave`, borda esquerda 2 px no tom, texto `--texto` | `estrutura.css` |
| Estado vazio | `.estado-vazio` | ícone em quadrado 40 px borda `--linha-forte`, ícone `--sinal-texto`; título `--fonte-titulo` | `estrutura.css` |
| Menu flutuante | `.menu-flutuante`, `.menu-item` | `--superficie`, borda `--linha`, `--sombra-painel`, raio 10; hover `--fundo-2` | `estrutura.css` |
| Modal | `.modal-fundo`, `.modal` | `--sobreposicao`; modal `--superficie`, borda `--linha`, raio `--raio-painel`, `--sombra-painel`; cabeçalho como barra do `MockWindow` | `estrutura.css` |
| Esqueleto | `.esqueleto` | `--fundo-2` → `--superficie-2`; sem brilho em reduced motion | `estrutura.css` |
| Lâmpada | (nova utilitária `.lampada`) | 8 px redondo `--sinal` | `base.css` |
| Terminal de cabo | (nova utilitária `.terminal-cabo`, `.terminal-ativo`) | 7 px, `--fundo`, borda 1,5 `--linha-forte`; ativo `--sinal` | `base.css` |
| Chip de etiqueta | `.ponto-etiqueta` (+ texto ao lado) | ponto 6–8 px na cor do usuário + `box-shadow: inset 0 0 0 1px` `--linha-forte`; pílula neutra `--fundo` borda `--linha` | `controles.css` (ponto; hoje em app.css 266–280) |
| Logotipo | `componentes/Logo.tsx` | tecla + "zapdesk" (research R7) | fundação |

## Receitas de área (blocos)

| Receita | Classes | Como fica (mockup) | Dono |
|---|---|---|---|
| Trilho | `.trilho`, `.item-trilho(.ativo)`, `.selo-trilho` | `--superficie-2`, borda direita `--linha`; ícone 18 px `--texto-2`; ativo `--fundo-2` + lâmpada à esquerda; selo âmbar `--tinta` mono 10 px com borda `--borda-primario` | B1 `shell.css` |
| Avatar | `.avatar` | neutro: `--superficie-2`, borda `--linha`, iniciais `--fonte-titulo` `--texto-2` | B1 |
| Item de conversa | `.item-conversa*` | ativo `--fundo-2` + barra âmbar 3 px à esquerda; hora mono 10–11 px (`--sinal-texto` 600 se não lida); pílula não lidas `--sinal`/`--tinta` mono; divisória `--linha` | B1 `conversas.css` |
| Filtros da lista | `.filtros`, chips | pílula h 24; ativo `--texto` sobre `--texto` invertido (`--fundo`) como no mockup | B1 |
| Bolhas | `.linha-mensagem(.minha) .bolha` | minha: `--superficie-2`, borda `--linha-forte`, canto sup. dir. 3 px; dele: `--superficie`, borda `--linha`, canto sup. esq. 3 px; hora mono 10 px `--texto-2`; "lida" `--ciano` | B2 `conversa.css` |
| Separador de dia | `.separador-dia` | pílula `--superficie` borda `--linha`, mono 10 px | B2 |
| Compositor | `.composer` | barra `--superficie` borda topo `--linha`; campo `--fundo` borda `--linha`; ícones `--texto-2` | B2 |
| Mídia | `.midia-*`, áudio, documento | áudio: picos `--texto` (tocado) / `--linha-forte`; documento: cartão `--superficie` borda `--linha`, extensão mono | B2 |
| Stepper | `.etapas li(.atual/.feita)`, `.etapa-numero` | sublinhado 3 px; atual `--sinal` + número sobre `--sinal` `--tinta` + lâmpada; feita `--ciano` com visto; futura `--linha` | B3 `disparos.css` |
| Progresso/contadores | `.barra-progresso`, `.barra-ok/-falhou`, `.percentual`, `.contador*` | trilho `--linha`; ok `--sinal`; falhou `--coral`; percentual `--fonte-titulo` 30 px 600; contadores em cartões `--superficie` borda `--linha`, número `--fonte-titulo` (lido/respondeu `--ciano`, falhou `--coral`) | B3 |
| Selo de destinatário | `.selo-destinatario.estado-*` | igual ao selo de estado, tom R8 | B3 |
| Coluna Kanban | `.coluna-kanban*` | `--superficie-2` borda `--linha` raio 12; cabeçalho com borda superior 4 px na cor da etapa (inline), ponto, nome 13 px 600, contagem mono | B6 `funil.css` |
| Cartão Kanban | `.cartao-kanban` | `--superficie`, borda `--linha`, `box-shadow: 0 1px 0 var(--linha)`; telefone mono; arrastando = `--sombra-painel` | B6 |
| Nó de chatbot | `.no-*` (React Flow) | cartão `--superficie` raio 10, cabeçalho `--superficie-2` mono 11 px com ícone (ativo/selecionado `--sinal-texto` + borda `--linha-forte`); cabos 1,5 px `--linha-forte`, selecionado `--sinal`; terminais = `.terminal-cabo`; fundo `--fundo` com `--pontos` 24 px | B7 `automacoes.css` |
| Editor de código | Monaco + `.editor-ia*` | tela escura (`--tela*`), árvore de arquivos `--superficie-2`; abas mono | B8 `editor-ia.css` |
| QR | `.qr-moldura`, `.qr` | QR `--qr-tinta` sobre `--qr-fundo` com margem; moldura `--superficie` borda `--linha-forte` raio 14 | B5 `ajustes.css` |
| Tela cheia | `.tela-cheia`, `.logo-grande` | `--fundo` + grade pontilhada; `Logo` 56–72 px; barra indeterminada `--sinal` (estática com reduced motion) | B5 |

## API implementada na fundação — use isto

Estado entregue em T001–T022. Tudo abaixo já existe e está em uso; blocos **só leem** estes
arquivos (regra 4 de tasks.md). Faltou algo? Uma linha em `specs/003-identidade-visual/pendencias.md`
com o prefixo do bloco e um ajuste local no **seu** arquivo.

### Arquivos CSS por área (ordem de importação em `main.tsx` = ordem das regras)

`tema → fontes → base → controles → estrutura → shell → conversas → conversa → crm → disparos →
ajustes → funil → automacoes → editor-ia`. A divisão foi provada pixel a pixel (T009): **não
reordene imports** nem mova regras entre arquivos. Cada arquivo tem no topo a origem (linhas do
antigo `app.css`/`automacoes.css`) e o dono. Seu arquivo herda as regras antigas da sua área: troque
as declarações; não renomeie classes.

| Dono | Arquivo |
|---|---|
| B1 | `estilos/shell.css`, `estilos/conversas.css` |
| B2 | `estilos/conversa.css` (inclui a faixa de automações na conversa) |
| B3 | `estilos/disparos.css` |
| B4 | `estilos/crm.css` |
| B5 | `estilos/ajustes.css` (tela cheia, conectar conta, ajustes, status, ajustes de IA/automações) |
| B6 | `estilos/funil.css` (já tem `line-height: 1.4` em `.cartao-kanban` — ver pendências) |
| B7 | `estilos/automacoes.css` (inclui o `@media (max-width: 1180px)` de `.corpo-editor`) |
| B8 | `estilos/editor-ia.css` (inclui o `@media` de `.corpo-editor-ia`) |

### Tokens (`estilos/tema.css`)

Exatamente os nomes de `contracts/tokens.md` §1–§2. Valor alterado na fundação: `--remetente-l`
claro **28%** (34% reprovava matizes amarelos no AA). Os aliases antigos (§3: `--verde`,
`--fundo-painel`, `--texto-3`, `--borda`, `--raio`…) estão num bloco "Transição" no fim do arquivo,
só para o app não quebrar enquanto os blocos trabalham: **no seu arquivo use só nomes novos**
(T053 apaga os aliases). Mapa rápido antigo → novo para a sua busca-e-troca:

```
--verde→--sinal (preenchimento)        --verde-forte→--sinal-texto (texto/ícone/link)
--verde-suave→--sinal-suave            --azul-lido→--ciano          --perigo→--coral
--aviso-fundo/-borda/-texto→--sinal-suave/--sinal/--texto
--erro-fundo/-texto→--coral-suave/--coral   --info-fundo/-texto→--ciano-suave/--ciano
--fundo-app→--fundo   --fundo-painel→--superficie   --fundo-cabecalho/-trilho→--superficie-2
--fundo-hover/-selecionado→--fundo-2   --fundo-campo→--fundo   --fundo-chat→--fundo
--bolha-minha→--superficie-2 (+borda --linha-forte)   --bolha-dele→--superficie (+borda --linha)
--bolha-citacao→--fundo-2   --sombra-bolha→(nada; use linhas)   --sombra-menu→--sombra-painel
--texto-3→--texto-2   --texto-inverso→--tinta   --borda→--linha   --borda-forte→--linha-forte
--esqueleto/-brilho→--fundo-2/--superficie-2   --raio→--raio-tecla   (--raio-bolha agora 10px)
#fff sobre verde → --tinta sobre --sinal    rgba(0,168,132,.x) → --sinal-suave
rgba(0,0,0,.x) de sombra → --sombra-painel ou borda --linha    fundo preto de mídia → --tela
```

Derivados prontos: `--sinal-suave`, `--coral-suave`, `--ciano-suave`, `--sobreposicao`,
`--sombra-painel`, `--qr-fundo`/`--qr-tinta`, `--raio-tecla` (10), `--raio-painel` (14),
`--raio-pilula`, `--raio-bolha` (10), `--fonte-titulo`, `--fonte`, `--fonte-mono`, `--ease-mesa`.
Precisa de transparência de um token? `color-mix(in srgb, var(--token) N%, transparent)` é
permitido (não é cor literal).

### Fontes (`estilos/fontes.css` + `util/fontes.ts`)

Famílias empacotadas (4 woff2 latin, ~290 KB, `font-display: block`): `'Bricolage Grotesque
Variable'` (títulos; eixos `wght` 200–800, `wdth` 75–100, `opsz`), `'Instrument Sans Variable'`
(interface; normal + itálico), `'JetBrains Mono Variable'` (mono). Use **só pelas pilhas**
`var(--fonte-titulo)`, `var(--fonte)`, `var(--fonte-mono)`. Títulos da landing:
`font-variation-settings: 'opsz' 24` (cartão/tela) ou `'opsz' 96, 'wdth' 88–90` (grandes),
peso 650–750, `letter-spacing` −0,01 a −0,035em. `main.tsx` espera as fontes (≤ 500 ms) antes do
primeiro render: nada a fazer nos blocos. Mono: sempre `font-variant-ligatures: none`.

### Utilitárias novas (`base.css`) — classes prontas para TSX decorativo

| Classe | Uso |
|---|---|
| `.rotulo` | rótulo mono 11 px, caixa alta, `.08em`, `--texto-2` |
| `.lampada` | ponto âmbar 8 px "ativo" — `<span className="lampada" aria-hidden="true" />` |
| `.terminal-cabo` / `.terminal-cabo.terminal-ativo` | terminal 7 px de cabo (fluxo/chatbot) |
| `.logo`, `.wordmark` | usados pelo componente `Logo` (não estilize de novo) |
| `.mono` | mono 13 px sem ligaduras |

Base global: `body` Instrument Sans 14 px / 1.5 (era 14,2 / 1.4 — **itens de listas virtuais
precisam de `line-height` em px no seu arquivo se a altura medida mudar**), `h1–h3` Bricolage 650
`opsz` 24, `a` = `--sinal-texto`, `:focus-visible` 2 px `--foco` offset 2, `::selection` âmbar,
`mark` = `--sinal-suave`, barras de rolagem finas (`--linha`, hover `--linha-forte`) e
`prefers-reduced-motion` zerando animações/transições globalmente.

### Primitivos prontos (não sobrescreva; só posicione/componha)

- `controles.css`: `.botao` (primário âmbar + `--tinta` + contorno `--borda-primario`), `.secundario`,
  `.perigo` (contorno coral), `.perigo-texto`, `.pequeno`/`.grande`/`.largo`, `.botao-link`
  (`--sinal-texto`), `.botao-icone` (+`.ativo`, `.pequeno`), `.chip` (+`.ativo` invertido),
  `.chip-etiqueta` (+`.mini`) com `.ponto-etiqueta` (contorno `--linha-forte`), `.selo`
  (+`.ok` ciano, `.erro` coral, `.info` neutro), campos (`.campo`, `.campo-busca`, `.seletor`,
  `.campo-unidade`, `.campo-codigo`; números/horas/tel/datas já em mono), `.grupo-campos`, `.caixa`,
  `.lista-escolha`, `.item-escolha`, `.abas/.aba(.ativa)`, `.interruptor(.ligado)`, `.bloco-log`
  (tela escura), `.numero-curto`. **Alturas iguais às antigas** (botão 28/36/44, campo 38).
- `estrutura.css`: `.tela`, `.cabecalho-tela`, `.barra-ferramentas`, `.cartao`, `.tela-dividida`
  (+`.painel-lista`/`.painel-detalhe`), `.estado-vazio`, `.tela-erro`, `.faixa` + `-aviso/-erro/-info`,
  `.esqueleto`, `.menu-flutuante`/`.menu-item`, `.modal-fundo`/`.modal*`, `.visualizador`,
  `.fundo-escuro` (= `--tela`), `.tabela`, `.tabela-virtual`, `.linha-tabela(.cabecalho)`,
  `.selo-estado.estado-*` (R8: `enviando` com lâmpada, `concluido` ciano, `falhou`/`erro` coral,
  demais neutros), `.lista-definicoes`.

### Componentes compartilhados

- `componentes/Logo.tsx`: `<Logo variante="completo" | "so-tecla" tamanho={56} />` e `<Tecla tamanho />`
  — `aria-hidden`; cores por token. B5 usa no `Carregando` (T038).
- `util/tema.ts`: `type PreferenciaTema = 'sistema'|'claro'|'escuro'`, `lerPreferencia()`,
  `salvarPreferencia(p)`, `aplicarTema(p)` (põe/tira `data-tema` e avisa), `temaEfetivo()`,
  `useTemaEfetivo(): 'claro'|'escuro'` (ao vivo). B5 (Aparência), B7 (`colorMode` do React Flow) e
  B8 (tema Monaco) usam estes; ninguém lê `matchMedia` por conta própria.
- `util/fontes.ts`: `aguardarFontes(limiteMs)` — já chamado em `main.tsx`.
- Ícones do app/bandeja: `npm run icones -w @zapdesk/app` (já regenerados).

### Checagens (rode no fim do seu bloco)

```bash
npm run tipos -w @zapdesk/app
npm test -w @zapdesk/app
npm run tokens -w @zapdesk/app -- --arquivos estilos/<seu>.css,telas/<Sua>.tsx   # 0 literais nos seus
npm run tokens -w @zapdesk/app -- --relatorio                                    # quanto falta no app
npm run contraste -w @zapdesk/app                                                # 0 falhas
```

`contraste.mjs` lê o `tema.css` sozinho; as paletas `CORES_ETIQUETA`/`CORES_ETAPA` aparecem como
"pendente B4/B6" até o arquivo ganhar o marcador `// cores-de-dados` (aí passam a contar como
falha se < 3:1). `verificar-tokens.mjs` aceita: linhas do array sob `// cores-de-dados`,
`hsl(… var(--remetente-l))` e `hsl(… 35% 30%)` em `telas/Status.tsx`.

### Capturas sem conflito entre blocos (fila/lock)

Todos os blocos dividem `app/out/`. Regra: **nunca rode `npm run compilar -w @zapdesk/app` solto**;
compile e capture num comando só, que pega o lock `/tmp/zapdesk-captura.lock` (quem chega espera
até 15 min, mostrando "aguardando outra captura"):

```bash
node app/scripts/capturar.mjs --compilar --telas <telas do bloco> --saida docs/verificacao/003/<bloco>
# referência:
node app/scripts/capturar.mjs --compilar --telas <…> --saida docs/verificacao/003/<bloco> --comparar docs/verificacao/003/fundacao
```

- O build pega o estado de **todos** os arquivos naquele instante: outro bloco no meio de uma edição
  pode aparecer nas telas dele — avalie só as suas telas.
- Comparações só valem entre rodadas com o **mesmo `--telas`** (telas mexem em dados: abrir a
  conversa marca como lida, gerar QR cria conta; a ordem é fixa, a da tabela de `capturas.md`).
  Para "mesma disposição" use `--comparar docs/verificacao/003/fundacao` só como apoio visual
  (vai dar diferença — é o seu restyle); a prova de "nada mudou" foi a T009.
- Mantenha `--escala` padrão (`css`, PNG 1×; o disco está curto). Uma rodada completa leva ~80 s;
  a de um bloco, 15–40 s. Não use `--manter` sem necessidade (segura o lock até Ctrl+C).
- Ruído conhecido: 1–30 px isolados de antisserrilhado em telas 960 (React Flow, ícones) variam
  entre rodadas; o script refotografa 2× antes de acusar. Diferença real persiste.
- Telas extras além da tabela: `execucoes-ok` (execução com sucesso) e `vazio-disparos` (estado
  vazio de Disparos); `execucoes` é a execução com **erro**. Áreas ignoradas na comparação (cores
  derivadas de ids aleatórios): avatar da conta, fundo do status de texto, QR, durações "N ms".
