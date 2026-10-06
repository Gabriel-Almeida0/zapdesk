# Pendências — 003 Identidade visual

> **Append-only.** Cada linha começa com o prefixo de quem escreveu (`F:` fundação, `B1:`…`B8:`
> blocos, `I:` integração, `V:` verificação). Não apague linhas de outros; a integração (T052)
> registra a resolução ao lado de cada uma.

## Fundação (T001–T022)

- F: T002 — baseline sem falhas pré-existentes: `npm run tipos -w @zapdesk/app` ok; `npm test -w @zapdesk/app` 17 arquivos / 110 testes ok.
- F: Disco quase cheio (281 MB livres no início; ~5 GB depois que o macOS liberou espaço). O Electron
  44.4.5 do workspace foi baixado só agora (não havia `node_modules/electron/dist`). Capturas em
  escala 1× (`--escala css`, padrão) para caber: ~9 MB por rodada completa.
- F: T004 — o ZapDesk instalado (pid 84148) estava aberto no início e não estava mais quando a primeira
  captura terminou; saída limpa (sem `runtime.json`), nenhum comando desta fundação o tocou (as
  capturas só matam processos com a pasta temporária `/tmp/zapdesk-captura-*` na linha de comando).
- F: T005/T006 — telas extras em relação a contracts/capturas.md: `execucoes-ok` e `vazio-disparos`;
  `execucoes` = execução com erro (automação de IA "Classificar lead" que lança erro, desligada
  depois). Avatar da conta e fundo do status de texto usam cor derivada de ULID aleatório → mascarados
  na comparação (somem quando B1/B5 neutralizarem). Durações "N ms" mascaradas. Relógio do renderer
  fixo (Playwright clock) em 2026-10-05 10:42 -03:00; relógio do motor recolocado nesse instante a
  cada tela.
- F: T007 — determinismo: 2 rodadas completas × "antes" = 128/128 e 127/128 (4 px de ruído em
  conversa-menu-960-claro). Ruído de antisserrilhado (1–30 px isolados, só em 960) aparece em telas
  diferentes a cada rodada; testado `--force-device-scale-factor=1` e `--disable-gpu` (pior) —
  mantido o modo padrão. O script refotografa 2× antes de acusar.
- F: T009 — divisão do CSS: 3 rodadas completas × "antes": 126/128, 127/128, 127/128; as diferenças
  (editor-chatbot-960-claro 6–7 px, conversa-menu-960-claro 4 px) são o mesmo ruído e cada um dos 128
  PNGs foi idêntico (0 px) ao "antes" em pelo menos uma rodada (comparacao*.json em
  docs/verificacao/003/divisao/). Análise estática das regras que mudaram de ordem: nenhum par com
  mesma especificidade capaz de casar o mesmo elemento. Nenhuma correção de ordem foi necessária.
- F: T019 — `--remetente-l` claro 34% → 28% (34% dava 3,6:1 em matizes amarelos); contracts/tokens.md
  atualizado. contraste: 197 pares · 0 falhas · 2 proibidos documentados · 11 pendentes (paletas
  antigas de Etiquetas/Funis, de B4/B6: contam como falha quando ganharem `// cores-de-dados`).
- F: T022 — `npm run tokens -- --relatorio` (cores literais restantes, por arquivo de bloco):
  automacoes.css 14 (B7) · Etiquetas.tsx 12 (B4) · ajustes.css 10 (B5) · conversa.css 10 (B2) ·
  Funis.tsx 9 (B6) · disparos.css 3 (B3) · shell.css 3 (B1) · Avatar.tsx 1 (B1) · Bolha.tsx 1 (B2) ·
  conversas.css 1 (B1) · editor-ia.css 1 (B8) · Kanban/EditarEtapas.tsx 1 (B6) · Status.tsx 1 (B5)
  = 67. Arquivos da fundação: 0.
- F: T022 — medidas "fundacao" × "antes": alturas de conversas, contatos, leads e destinatários
  iguais; layoutShift 0; fontes Bricolage/Instrument/JetBrains `loaded` (itálico só sob demanda);
  rede externa vazia. **B6**: cartões do Kanban mudaram de 86,77 → 86,48 px (e 108,86 → 109,88 com
  etiqueta) pelo corpo novo 14 px/1.5 — a fundação pôs `line-height: 1.4` em `.cartao-kanban`; a
  lista mede cada item (measureElement), sem sobreposição, mas B6 deve fixar `line-height` em px nos
  textos do cartão e conferir com `--medir`.
- F: capturar.mjs ganhou `--compilar` (compila dentro do lock `/tmp/zapdesk-captura.lock`): blocos
  devem usar sempre `--compilar` em vez de compilar à parte (ver vocabulario-visual.md → "API
  implementada na fundação").
- B3: T031/T032 — fundo dos cartões em `--superficie-2` como nos mockups, com ajuste local em disparos.css
  (`.novo-disparo > .cartao` e `.cartao.progresso-disparo`, classe nova no TSX); se a integração
  quiser, vira variação de `.cartao` em estrutura.css. TSX só decorativo: visto + número
  visualmente oculto e lâmpada no stepper (nome acessível segue "1 Lista"), `.cartao-disparo-conta`
  (mono) e `.estimativa-valor` (mono) — mesmo texto. Selos de destinatário seguem R8 (entregue
  neutro; o RelatorioMockup pinta entregue de ciano). Linhas de Destinatários seguem 40 px (--medir).
- B3: T033 — capturas finais em docs/verificacao/003/b3 (6 telas × 2 temas × 2 tamanhos) comparadas
  com DisparoMockup/RelatorioMockup e tour-1440-pt-*; FALTA o percurso manual dos 4 passos só com
  teclado (sem Playwright MCP, não houve como dirigir o app): o stepper não sobrescreve o
  `:focus-visible` global. layoutShift 0,00012 igual em todas as telas do bloco (inclusive lista,
  que não tem nada de B3 que se mova) — provavelmente de shell/fontes, não de disparos.css.
- B2: T027–T030 feitos (conversa.css sem nenhum alias antigo; Bolha.tsx `hsl(… 55% var(--remetente-l))`; Midia.tsx põe `--progresso` no range do áudio, que vira picos por máscara; FaixaAutomacoes.tsx ganhou `.lampada` no estado "robô ativo"). Desvio consciente: nome no cabeçalho da conversa 14 px 600 (não 13) para alinhar com os nomes da lista de B1 no tamanho real do app. Capturas finais em docs/verificacao/003/B2/.
- B2: pré-existente (igual na captura "fundacao", não é do restyle): em 960 px com o painel do contato aberto a conversa fica espremida (bolhas com 1–2 palavras por linha, ações do cabeçalho invadem o cabeçalho do painel); e no `conversa-menu` 960 o menu da mensagem perto do topo é cortado pela rolagem de `.mensagens`. Mudar exige mexer em layout (largura do painel / posição do menu) — decisão do dono/integração.
- B2: T030 — o fluxo "enviar texto, anexar documento, gravar áudio, responder, apagar, abrir painel" foi coberto pelos testes do renderer (us1, us4, us5, menu-mensagem, faixa-automacoes: 33 ok) e pelas capturas; não foi clicado à mão no app falso (só CSS/marcação decorativa mudou).
- B4: T035 — paleta R9 ajustada ao critério "ui ≥ 3:1 sobre --superficie nos 2 temas": âmbar `#FFB020`
  dá 1,83:1 no branco → `#c48200`; ocre `#C98500` (≈ âmbar) → `#a86b3a`; vinho `#9B3D5A` dá 2,56:1 no
  escuro → `#b04a6e`; ciano/coral/grafite/ardósia/oliva como no R9. 8 cores minúsculas (a comparação usa
  `toLowerCase`), padrão e fallback do `input type=color` = primeira. Etiqueta verde da semente segue verde.
- B4: crm.css também estiliza classes usadas fora das telas do B4 — `.item-status`/`.anel-status`
  (Status, B5: ativo ganhou a barra âmbar 3 px; anel `--sinal`), `.variaveis .chip` mono e `.anexo-escolhido`
  (PassoMensagem, B3). `.leads-grade` é da fundação (estrutura.css); em crm.css só `justify-self: start`
  no `.selo` (a pílula "Sim" esticava a coluna — já era assim antes) e datas em mono.
- B7: T047 — `<Background gap={24} size={2} />` em vez de `size={1}`: no React Flow `size` é o diâmetro (raio = size/2 × zoom); com 1 e o fitView do seed (~0,5×) os pontos `--pontos` somem. 2 = o ponto de 2 px da `grade-pontos` da landing no zoom 1.
- B7: T048 — capturas e lado a lado feitos (docs/verificacao/003/B7, telas automacoes, editor-fluxo, editor-chatbot, execucoes, execucoes-ok); as interações com mouse no app falso (ligar/desligar, arrastar nó, ligar cabo, simular conversa) NÃO foram clicadas — capturar.mjs não tem roteiro de interação e o Playwright MCP é proibido. Cobertura: editor-chatbot.test.tsx (paleta→ligar saídas→salvar, chat simulado/nó atual) passa. Terminais ficaram com 7 px visuais, mas com área de pega de 21 px (`::before` inset −7 px). A integração/verificação deve testar arrastar e ligar cabo com o mouse.
- B7: `.lista-automacoes` (gap da lista) está em funil.css (B6) desde a divisão; B7 não mexeu. Bordas da lista/seções usam `0 1px 0 var(--linha)` como a landing.

## B1 — Shell + lista de conversas (T023–T026)

- B1: T026 — `--medir` × fundação: alturas de Conversas (72 px) e Contatos (65 px) iguais em todas as telas/temas; layoutShift 0 na rodada final (uma rodada intermediária deu 0,00012 em todas as telas com semente, inclusive fora do B1: ruído ou outro bloco no meio da edição).
- B1: a faixa âmbar na base do `.painel-chat.vazio` (estado "Escolha uma conversa") é de `conversa.css` (B2), não do shell.
- B1: `.chip` (controles.css) no filtro da lista fica `--superficie` e 30 px; o mockup usa `--fundo` e 24 px. Não sobrescrevi (primitivo da fundação; altura mudaria o layout).
- B1: Avatar agora é neutro em todo o app (contatos, seletor de conta, trilho, painel do contato); a prop `chave` ficou só por compatibilidade.

## B5 — Ajustes, contas, QR, status, carregando/erro, tema manual (T037–T041)

- B5: T038 — Carregando mostra `<Logo variante="completo" tamanho={56}>`; o `<h1>ZapDesk</h1>` continua no DOM (leitores de tela) mas fica **visualmente oculto** em `.carregando h1` (ajustes.css), porque o wordmark "zapdesk" ao lado repetiria o nome. Se a integração preferir o h1 visível, é só apagar a regra `.carregando h1`. Sugestão para T053: uma utilitária `.so-leitor` em base.css.
- B5: ilhas escuras sem sobrescrever primitivos — `.bloco-codigo` (Usar com Claude) e `.fundo-escuro .estado-vazio` (Status) **reapontam tokens** no escopo (`--texto: var(--tela-texto)`, `--linha-forte`, `--fundo-2`, `--foco`, `--superficie`, `--sinal-texto` → `--tela*`) para o `.botao.secundario`/estado vazio manterem contraste na tela escura no tema claro. Se a fundação quiser, isso vira uma utilitária `.ilha-escura` em estrutura.css.
- B5: Status.tsx — a cor passou a `hsl(${tom} 35% 30%)` com `const tom = matiz(item.id)`, porque a regex de exceção do verificar-tokens (`hsl\([^)]*35%\s+30%\)`) não aceita `)` dentro do `hsl(` (ex.: `matiz(item.id)`).
- B5: QR — `.qr-moldura` 288 → 312 px e `.qr` ganhou 10 px de margem branca (`--qr-fundo`, content-box) para a zona de silêncio ficar branca também no tema escuro (moldura `--superficie`); QR continua 264 px preto sobre branco.
- B5: T041 — troca de Aparência validada por teste (`tests/renderer/ajustes-aparencia.test.tsx`: aplica `data-tema` na hora, grava/apaga a chave, abre com a salva) + `main.tsx` já chama `aplicarTema(lerPreferencia())` antes do render. **Não** fiz a conferência manual com `--manter` (exige clicar na janela e reabrir; agente sem interação): fica para V (T05x) no app falso.
- B5: rodada de capturas B5 levou ~5 min por esperar o lock de outros blocos; capturas finais em docs/verificacao/003/B5 (24 PNG).
- B6: altura dos cartões do Kanban fixada em px em `funil.css` (linha 20 / 16 / 16,765625 px; chips 14,09375 px; borda 1 px compensada no padding 9 px): `--medir` volta a dar **86,77 / 108,86** (idêntico ao "antes"), layoutShift 0, nos 4 tamanhos/temas. Ajuste provisório `line-height: 1.4` da fundação removido.
- B6: `.lista-virtual` (conversas.css, B1) tem `margin: 0 -14px` e vazava no Kanban (cartões 4 px para fora da coluna e barra de rolagem horizontal no quadro, já no "antes"). Neutralizado só no Kanban com `.coluna-kanban-lista.lista-virtual { margin: 0 }` em funil.css; B1/I podem preferir escopar a margem negativa em conversas.css.
- B6: `CORES_ETAPA` = mesmos 8 valores validados de `CORES_ETIQUETA` (B4), começando no ciano (Novo ciano, Qualificando âmbar, Proposta coral, como no KanbanMockup); `COR_ETAPA_PADRAO` = primeira (fallback em Funis.tsx e EditarEtapas.tsx). Etapas já salvas (semente: #53bdeb…) seguem inline.
- B6: Funis.tsx `.etapa-resumo` passou de `style.borderColor` para `style.borderLeftColor` (contorno neutro `--linha` + borda esquerda na cor da etapa). Cartao.tsx ganhou a classe visual `.arrastando` (dragstart/dragend), sem mudar dados do arraste.
- B6: T044 — arraste e "Mover para…" por teclado conferidos no app falso com script Playwright próprio (scratchpad, mesmo lock e pasta temporária, sem PNG): arrastar Juliana Novo→Proposta e, por teclado, Otávio Novo→Fechado; contagens 3/3/1/1 → 1/3/2/2; foco visível (`--foco`). Não havia captura `secoes/tour-*` da aba Funil na landing (só a aba Disparo); comparado com `KanbanMockup.tsx`. Capturas em docs/verificacao/003/B6 (a tarefa dizia `b6`).
- B8: T049–T051 — permissões do manifesto continuam como texto corrido em `.nota-isolamento` (EditorIA.tsx junta com ", "); virar pílulas mono exigiria mudar a marcação/texto visível — fica para decisão da integração. Painel "Testar": `.bloco-log`/`.resultado-execucao` são da fundação/B7 (não sobrescritos).
- B8: T051 — na captura (motor falso), o botão "Testar" do painel não mudou de estado ao clicar (nem "Executando…" nem resultado), embora `POST /automacoes/<ia>/testar` direto no motor responda 200 em ~45 ms (execução "erro: Handler aoExecutar não exportado"). Comportamento não foi tocado pelo B8 (onClick presente na fibra); investigar fora do restyle. Erro de compilação verificado: sublinhado coral no Monaco, nome coral ondulado na árvore, status coral, painel de problemas (`B8/editor-ia-erro-1280-*.png`, roteiro temporário com o mesmo lock, sem compilar).

## I — Integração, verificação e entrega (T052–T064, + T033/T048 pendentes)

Resolução de cada pendência dos blocos (arquivo dono entre parênteses):

- I: **B8 "Testar" não faz nada → BUG ANTIGO, não é do restyle.** Reproduzido igual no build da
  base pré-restyle (`~/.cache/zapdesk-003-base/src` compilado à parte, mesmo motor): clique chega
  ao botão, `testar.mutate()` roda, a chamada `POST /automacoes/<ia>/testar` com
  `{"ia_simulada":true,"mensagem":…}` **derruba o motor** (panic nil pointer em
  `motor/internal/claude/falsa.go:82`, `(*Falsa).registrar` com receptor nil, via
  `ponte/dados.go:566` `cli.Gerar`). Causa: `motor/internal/aplicacao/ia.go:24`
  `iaSimulada := claude.Cliente(p.iaFalsa)` — sem `--ia=falsa`, `p.iaFalsa` é `*Falsa` nil e a
  interface fica não-nil, então o `if iaSimulada == nil` nunca cria a `NovaFalsa`. O supervisor
  religa o motor, a tela remonta e o erro some (por isso nem "Executando…" aparece). Afeta também o
  app real (qualquer "Testar" com "IA simulada" marcada numa automação que chama `ctx.ia`). O B8
  testou com `entrada` (aoExecutar), que não chama `ctx.ia` — por isso deu 200. **Não corrigido
  aqui** (fora do escopo visual, motor intocado); correção sugerida de 1 linha: `if p.iaFalsa !=
  nil { iaSimulada = p.iaFalsa } else { iaSimulada = claude.NovaFalsa(a.Relogio.Agora) }` + teste
  de integração "testar com IA simulada sem --ia=falsa".
- I: B6/B1 `.lista-virtual` com `margin: 0 -14px` vazando → escopado em conversas.css para
  `.painel-lista > .lista-virtual, .tela-lista > .lista-virtual` (Conversas e Contatos); ajuste
  local do Kanban removido de funil.css. Alturas medidas iguais ao "antes" (44 listas, 128 telas).
- I: B2 960 px com painel do contato → correção só visual em conversa.css: o nome do contato cede
  espaço primeiro (`flex-shrink: 1000`, mínimo = avatar), `.acoes-cabecalho` encolhe dentro do
  cabeçalho e, por container query (cabeçalho < 360 px), os chips de etiqueta — repetidos no painel
  ao lado — saem do cabeçalho. "Assumir" e o botão do painel ficam inteiros; nada invade o painel.
  Em 1280 nada muda (nome já truncava antes). Menu da mensagem cortado no topo em 960 (layout) e
  bolhas estreitas: ficam como estavam.
- I: B3 fundo `--superficie-2` → variação `.cartao.cartao-recuado` em estrutura.css, usada no
  cartão do assistente (NovoDisparo) e no de progresso (DetalheDisparo); ajustes locais removidos de
  disparos.css. "Entregue" fica **neutro** (R8/contrato), não ciano como o RelatorioMockup.
- I: B5 h1 do Carregando → utilitária `.so-leitor` em base.css (h1 continua no DOM, oculto só na
  tela; o wordmark já mostra o nome); regra local removida de ajustes.css. Visto do stepper do B3
  passou a usar a mesma utilitária. Ilhas escuras (`.bloco-codigo`, Status) ficam como reapontamento
  local de tokens (funcionam, contraste ok) — não virou utilitária.
- I: B5 troca manual de tema conferida no app falso (roteiro Playwright, `--roteiro`): macOS claro →
  "Escuro" aplica `data-tema=escuro` na hora (fundo #0E1013), sobrevive a recarregar, editor Monaco
  em `--tela` (#0A0C0F); "Sistema" remove o atributo e volta a seguir o macOS (claro e escuro).
- I: B1 faixa âmbar na base do "Escolha uma conversa" era a listra verde do WhatsApp Web → cor
  removida (borda transparente de 6 px para não mudar a geometria). Pílulas de filtro: `.chip`
  inativo passou a `--fundo` como no ChatMockup (altura 30 px mantida).
- I: B8 permissões → pílulas mono decorativas (`<span class="permissao">` por item, mesmo texto,
  mesmas vírgulas e ponto final; inline sem padding vertical, altura da nota igual).
- I: T053 camada de aliases removida de tema.css (grep exato dos nomes antigos = 0 em
  `app/src/renderer` e `app/tests`).
- I: T061 — abertura com semente tinha layoutShift 0,000118 (o grupo de baixo do trilho crescia 44 px
  quando o avatar da conta chegava): `.trilho-grupo:last-child { min-height: 86px }` em shell.css →
  0 em todas as 128 telas. Foco de campos (borda só mudava de cor) ganhou anel de 2 px
  (`box-shadow 0 0 0 1px var(--foco)` + borda `--foco`) em controles.css (inclui `.formulario` e
  datetime) e no composer/notas (conversa.css).
- I: capturar.mjs ganhou `--roteiro <arquivo.mjs>` (interações com o mesmo lock, pasta temporária e
  semente) e `--app <dir>` (lançar outro build, ex.: a base de comparação).

Verificação (rodadas finais):

- I: T033 — novo disparo só com teclado: Tab até a caixa, Espaço (14 marcados), Continuar por Enter,
  mensagem digitada, ritmo, revisão; foco no "Iniciar" e **não** iniciado (4 disparos antes e
  depois). Anel visível em todos os passos (`docs/verificacao/003/foco/disparo-passo*-teclado-claro.png`).
- I: T048 — no app falso: interruptor ligar/desligar pelo trilho visível (ativa→inativa→ativa, via
  motor), nó n3 arrastado (transform mudou), cabo ligado com o mouse (arestas 5→6, não salvo), chat
  simulado respondeu (3→5 linhas). Capturas em `foco/chatbot-*`.
- I: T058 — Tab em conversas, Kanban, chatbot e Ajustes: todo controle com `:focus-visible` tem anel
  (outline 2 px `--foco` ou anel de campo); buscas mostram o anel no wrapper `.campo-busca`. Com
  `reducedMotion: reduce`, 0 animações rodando.
- I: T055/T059 — `depois/` completo (128 PNG, claro/escuro, 1280/960) com `--medir`: alturas de
  itens virtuais idênticas ao "antes" (44 listas), layoutShift 0 em todas, fontes Bricolage /
  Instrument / JetBrains `loaded` (itálico só sob demanda), rede externa vazia; 4 woff2 latin
  (~290 KB) em app/out e no app.asar; nenhum googleapis/gstatic.
- I: T056 — tipos ok; `npm test` (raiz) verde: app 130, demais workspaces 40+10+69+109; motor:testar
  37 pacotes ok; contraste 193 pares · 0 falhas; tokens 109 arquivos · 0 literais/verdes.
- I: T060 — diff contra a base revisado: só className, marcação decorativa (lâmpadas, visto, spans
  mono, pílulas de permissão), Logo, cores→tokens, paletas de dados, tema/fontes/Aparência,
  colorMode, tema Monaco, pastas/index (userData no modo falso), janela.ts e o arraste visual do
  Kanban (`.arrastando`). Nada a reverter.
- I: verde restante só em **dados salvos** da semente (etiqueta verde antiga, etapa "Fechado"),
  como previsto em tokens.md §4 — nenhum token/cor de interface verde.

Entrega:

- I: T062 `npm run empacotar` → `app/dist/ZapDesk-0.1.0-arm64.dmg` (ícone novo, 4 woff2 no asar,
  assinatura ad-hoc válida). T063: `pgrep -x ZapDesk` vazio → `/Applications/ZapDesk.app`
  substituído por ditto. Não foi aberto (abriria a pasta de dados real); pasta
  `~/Library/Application Support/ZapDesk` intocada.
- I: em aberto para o dono: corrigir o panic da IA simulada no motor (acima); conferência humana
  do ícone no Dock e do visual com a conta real; offline com Wi-Fi desligado no app instalado
  (as fontes já estão empacotadas e a rede externa medida é vazia).
