# Quickstart — validar a 003 (identidade visual)

Contratos: [tokens](contracts/tokens.md), [vocabulário](contracts/vocabulario-visual.md),
[capturas](contracts/capturas.md). Entidades: [data-model.md](data-model.md).
Caminhos relativos a `~/projetos/zapdesk`.

## 0. Preparar (uma vez)

```bash
npm install                                   # raiz (workspaces); hoje node_modules não existe
npm run motor:compilar                        # motor/bin/zapdesk-motor
npm run compartilhado:compilar
npm run runner:compilar
npm run compilar -w @zapdesk/app              # app/out/ (as capturas usam o build, não o dev server)
```

Recompile o app (`npm run compilar -w @zapdesk/app`) antes de cada rodada de capturas.

## 1. Capturas sem WhatsApp real

O app sobe em **modo falso** (`ZAPDESK_DEV_FALSO=1` → motor `--whatsapp=falso`) com pasta de dados
**e** pasta da interface temporárias — não lê nada de `~/Library/Application Support/ZapDesk` e
roda mesmo com o ZapDesk instalado aberto. O script lança o Electron com o Playwright de
`../pbn/node_modules/playwright` (`_electron`), espera o `runtime.json` da pasta temporária,
**semeia dados fictícios** pela API e por `/v1/falso/*` (conta "Comercial", conversas da Marina
Couto etc., funil, disparo pausado com falhas, automações), navega pelas rotas (`#/…`) e troca o
tema com `nativeTheme.themeSource` (claro/escuro, inclusive no app antigo).

```bash
# todas as telas, 2 temas, 2 tamanhos
node app/scripts/capturar.mjs --saida docs/verificacao/003/antes
# só algumas telas de um bloco
node app/scripts/capturar.mjs --telas conversas,conversa,conversa-menu --saida docs/verificacao/003/b2
# provar que a divisão do CSS não mudou nada (sai 1 se houver diferença)
node app/scripts/capturar.mjs --saida docs/verificacao/003/divisao --comparar docs/verificacao/003/antes
# medidas: alturas de listas virtuais, CLS, fontes, rede
node app/scripts/capturar.mjs --telas conversas,leads --medir --saida docs/verificacao/003/medidas
# interações no app falso (teclado, arrastar, "Testar") com um roteiro próprio
node app/scripts/capturar.mjs --roteiro caminho/roteiro.mjs --saida /tmp/x
```

Esperado: PNGs `<tela>-<largura>-<tema>.png` (lista de telas em `contracts/capturas.md`); nenhum
processo `zapdesk-motor`/Electron de teste sobrando (`ps aux | grep zapdesk-captura` vazio); pasta
temporária apagada.

Manual (se precisar olhar com calma): `ZAPDESK_DEV_FALSO=1 npm run dev -w @zapdesk/app` sobe o app
em modo falso com pasta temporária; para dados, `node app/scripts/semente-falsa.mjs --runtime
<pasta>/runtime.json`.

## 2. Checagens automáticas

```bash
npm run contraste -w @zapdesk/app     # pares WCAG lidos de estilos/tema.css → "N pares · 0 falha(s)"
npm run tokens -w @zapdesk/app        # 0 cores literais fora de tema.css (exceções listadas) e 0 verdes
npm run tipos -w @zapdesk/app
npm test -w @zapdesk/app              # todos os testes antigos + tema.test.tsx
```

## 3. Revisão visual (antes × depois × mockup)

Para cada tela, abrir lado a lado `docs/verificacao/003/antes/<tela>…`, `…/depois/<tela>…` e a
referência da landing (`zapdesk-site/docs/verificacao/final/topo-1440-pt-{claro,escuro}.png` para
conversas; `secoes/tour-*` para disparo/relatório/Kanban; `secoes/automations-*` para chatbot e
código). Conferir: nada verde; âmbar só como preenchimento/lâmpada/foco (texto âmbar escuro no
claro); estados com as cores da landing; mesma disposição e mesmos textos do "antes".

## 4. Acessibilidade e conforto

1. Tab por conversas, novo disparo (4 passos), Kanban, editor de chatbot e Ajustes: anel âmbar
   sempre visível (SC-007).
2. Ajustes do Sistema → Acessibilidade → Tela → **Reduzir movimento**: abrir o app → sem brilho do
   esqueleto nem barra indeterminada correndo.
3. `--medir`: `layoutShift == 0`, fontes `Bricolage Grotesque Variable`, `Instrument Sans
   Variable`, `JetBrains Mono Variable` com status `loaded`, `redeExterna == []`, alturas dos itens
   iguais às do "antes".
4. Offline: desligar o Wi-Fi, abrir o app empacotado → fontes corretas.

## 5. Tema manual (US5)

Ajustes → Aparência: "Escuro" com o macOS claro → muda na hora (incluindo editor de código e
chatbot); fechar/abrir → continua escuro; "Sistema" → volta a acompanhar o macOS.

## 6. Ícones

`node app/scripts/gerar-icones.mjs` → abrir `app/resources/icone.png` e os dois
`bandejaTemplate*.png` (Quick Look). Depois de empacotar: ícone no Dock/Finder; com um disparo
ativo (no app instalado, com dados reais **só se o usuário quiser**; ou no modo falso) fechar a
janela e olhar a barra de menu em claro e escuro.

## 7. Entrega

```bash
npm run empacotar                       # raiz → scripts/empacotar.sh → app/dist/ZapDesk-0.1.0-arm64.dmg
pgrep -x ZapDesk >/dev/null && echo ABERTO || echo FECHADO
```

- **FECHADO**: `rm -rf /Applications/ZapDesk.app && ditto app/dist/mac-arm64/ZapDesk.app
  /Applications/ZapDesk.app`, abrir uma vez e conferir ícone/visual.
- **ABERTO**: **não** fechar nem substituir; avisar o usuário que o `.dmg` está em
  `app/dist/` e que basta fechar o app e arrastar o novo para Aplicativos.
