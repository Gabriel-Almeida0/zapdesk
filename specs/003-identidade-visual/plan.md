# Implementation Plan: Identidade visual — o app com a cara da landing

**Branch**: `003-identidade-visual` (sem branch git) | **Date**: 2026-10-05 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/003-identidade-visual/spec.md`

## Summary

Trocar a pele do renderer do app (`app/src/renderer`) de "WhatsApp Desktop" para o sistema
**"mesa de operação"** da landing (`zapdesk-site/src/app/globals.css`): mesmos tokens claro/escuro,
âmbar único, coral/ciano para estados, linhas finas, rótulos mono, Bricolage Grotesque / Instrument
Sans / JetBrains Mono **empacotadas** (woff2 latin de `@fontsource-variable`, sem rede), logotipo
da tecla âmbar com "Z" (ícone do app, template da barra de menu, tela de carregamento) e um seletor
de tema opcional em Ajustes. Nada de comportamento muda.

Abordagem: **fundação** (captura "antes" com WhatsApp falso e dados fictícios → divisão mecânica de
`app.css`/`automacoes.css` em arquivos por área, provada pixel a pixel → tokens novos + camada de
aliases dos nomes antigos → fontes → primitivos compartilhados → logo/ícones → scripts de
contraste e de tokens) → **8 blocos [P]**, cada um dono exclusivo de um arquivo CSS de área e dos
componentes daquela área → **integração** (remove aliases, resolve pendências) → **verificação**
(contraste, tokens, testes, capturas antes/depois, foco, CLS, offline) → **entrega** (empacotar;
reinstalar só se o app estiver fechado).

## Technical Context

**Language/Version**: TypeScript 5.9, React 19, CSS puro (sem Tailwind no app), Electron 44.4.5
(Chromium recente: `color-mix()`, `:has()`, `@layer`, `light-dark()` disponíveis), Node ≥ 22.12.

**Primary Dependencies** (novas, só `devDependencies` do app, versões exatas):
`@fontsource-variable/bricolage-grotesque@5.3.0`, `@fontsource-variable/instrument-sans@5.3.0`,
`@fontsource-variable/jetbrains-mono@5.3.0` (usados só como fonte dos arquivos woff2 latin; o Vite
copia para `out/renderer/assets`). Verificação: Playwright 1.60 de
`$PLAYWRIGHT_PATH` (`_electron`), sem instalar
no repo. Nenhuma dependência nova no motor, MCP, runner ou compartilhado.

**Storage**: preferência de tema em `localStorage` do renderer (chave `zapdesk.tema`), dentro do
`userData` do Electron (`~/Library/Application Support/ZapDesk/interface`). Nada no SQLite.

**Testing**: Vitest + Testing Library existentes (`npm test -w @zapdesk/app`) continuam passando;
novo teste só para o seletor de tema (`app/tests/renderer/tema.test.tsx`). Checagens novas:
`npm run contraste -w @zapdesk/app` (WCAG, lê `tema.css`), `npm run tokens -w @zapdesk/app`
(nenhuma cor literal fora de `tema.css`), capturas com `app/scripts/capturar.mjs`.

**Target Platform**: macOS 14+ Apple Silicon (app empacotado `.dmg` arm64).

**Project Type**: desktop-app (Electron + React) — só `app/` muda.

**Performance Goals**: CLS = 0 na abertura; troca de tema < 0,5 s; fontes ≈ 290 KB no pacote
(Bricolage "standard" 131 KB, Instrument Sans normal 57 KB + itálico 62 KB, JetBrains Mono 40 KB).

**Constraints**: CSP atual (`font-src 'self' data:`, `style-src 'self' 'unsafe-inline'`,
`script-src 'self'`) **sem mudança** — fontes servidas de `self`; sem script inline (a preferência
de tema é aplicada em `main.tsx` antes do primeiro render). Alturas dos itens das listas
virtualizadas inalteradas. Nada de rede.

**Scale/Scope**: ~4.600 linhas de CSS (app.css 3.238 + automacoes.css 1.356), 72 componentes com
`className`, 28 hex + ~27 `rgba()` soltos no CSS, 3 paletas/`hsl()` em TSX, 1 cor no main
(`janela.ts`); ~25 telas/estados a capturar × 2 temas × 2 tamanhos.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.* — constituição **1.1.0**.

| Princípio | Como o plano cumpre | Status |
|-----------|---------------------|--------|
| I. Local-first e privacidade | Fontes empacotadas, nenhuma requisição a Google Fonts/CDN (verificado offline + CSP inalterada); capturas só com WhatsApp falso, pasta de dados temporária e dados fictícios; nenhum dado do usuário sai da máquina | ✅ |
| II. Leveza e ciclo de vida | Só CSS/arquivos de fonte (~290 KB) no renderer; motor intocado; o script de captura encerra o Electron e o motor e apaga a pasta temporária | ✅ |
| III. Motor fonte única | Nenhuma ação nova de domínio. A preferência de tema é estado da **janela** (como tamanho/posição), não dado de domínio: fica no renderer e não vai à API nem ao MCP — registrado em Complexity Tracking | ✅ (justificado) |
| IV. WhatsApp isolado | Usa o cliente falso existente (`--whatsapp=falso` / `ZAPDESK_DEV_FALSO=1`) para capturas; nenhum código toca whatsmeow | ✅ |
| V. Disparo idempotente | Nada muda no disparo; a semente de captura usa só a API/rotas `/v1/falso/*` do motor falso | ✅ |
| VI. Testes de domínio | Nenhuma regra de domínio nova; testes existentes devem passar sem mudar expectativas; teste novo para o seletor de tema | ✅ |
| VII. Português | Tokens, arquivos, scripts, comentários, rótulos ("Aparência", "Sistema", "Claro", "Escuro") em pt-BR; nomes de fontes são nomes próprios | ✅ |
| VIII. Código isolado | Não toca runner/automações; só o visual do editor (Monaco) muda | ✅ |
| Restrições (Electron seguro) | `contextIsolation`, `sandbox`, preload e CSP inalterados; nenhuma IPC nova | ✅ |

**Re-check pós-design (Fase 1)**: `contracts/tokens.md`, `contracts/vocabulario-visual.md` e
`contracts/capturas.md` mantêm todos os itens. A única mudança no processo principal é a pasta
`userData` temporária **apenas no modo falso** (para o app de teste não disputar a trava de
instância única com o app instalado aberto) e a cor de fundo da janela antes do carregamento.

## Project Structure

### Documentation (this feature)

```text
specs/003-identidade-visual/
├── plan.md
├── research.md                 # decisões R1–R14
├── data-model.md               # tokens, aliases, preferência de tema, pares de contraste, capturas
├── quickstart.md               # rodar, capturar (antes/depois, claro/escuro), verificar, entregar
├── contracts/
│   ├── tokens.md               # tokens novos (valores claro/escuro/tela) + mapa antigo → novo
│   ├── vocabulario-visual.md   # receitas por componente (botão, campo, selo, chip, bolha, stepper…)
│   └── capturas.md             # CLI do capturar.mjs, lista de telas, semente fictícia, saídas
├── checklists/requirements.md
├── pendencias.md               # criado na fundação; recado append-only dos blocos (prefixo Bn:)
└── tasks.md
```

### Source Code (repository root)

```text
app/
├── package.json                      # + devDeps de fontes; scripts contraste, tokens, capturar
├── scripts/
│   ├── gerar-icones.mjs              # REESCRITO: tecla âmbar + "Z" (icone.png 1024, bandejaTemplate 16/32)
│   ├── contraste.mjs                 # NOVO: lê estilos/tema.css e checa pares WCAG (sai 1 se falhar)
│   ├── verificar-tokens.mjs          # NOVO: cor literal fora de tema.css = erro (com lista de exceções)
│   ├── capturar.mjs                  # NOVO: Playwright _electron + motor falso + semente → PNGs
│   └── semente-falsa.mjs             # NOVO: dados fictícios (conta, conversas, leads, funil, disparo…)
├── resources/{icone.png, bandejaTemplate.png, bandejaTemplate@2x.png}   # regenerados
└── src/
    ├── main/index.ts                 # modo falso: userData temporário (ou ZAPDESK_PASTA_INTERFACE)
    ├── main/janela.ts                # backgroundColor = fundo novo (#F4F1E8 / #0E1013)
    └── renderer/
        ├── main.tsx                  # importa os CSS por área; aplica tema salvo; espera fontes (≤ 500 ms)
        ├── util/tema.ts              # NOVO: preferência Sistema/Claro/Escuro + tema efetivo + hook
        ├── componentes/Logo.tsx      # NOVO: Tecla + wordmark (cores por token)
        └── estilos/                  # app.css e o antigo automacoes.css são DIVIDIDOS (fundação):
            ├── tema.css              # tokens novos + aliases antigos (removidos na integração)
            ├── fontes.css            # @font-face das 3 famílias (woff2 latin)
            ├── base.css              # reset, tipografia, foco, seleção, rolagem, reduced motion
            ├── controles.css         # botões, campos, grupo-campos, interruptor, abas, utilitários
            ├── estrutura.css         # telas genéricas, cartão, estado vazio, faixas, esqueleto,
            │                         # menus, modais, visualizador, tabelas, selos, lista-definicoes
            ├── shell.css             # B1: layout, trilho, avatar, ponto de estado
            ├── conversas.css         # B1: lista de conversas, busca de mensagens
            ├── conversa.css          # B2: chat, menu da mensagem, mídia, compositor, painel, faixa automações
            ├── disparos.css          # B3: lista, assistente (stepper), detalhe/relatório
            ├── crm.css               # B4: listas simples, contatos, templates, etiquetas, importação
            ├── ajustes.css           # B5: conectar conta/QR, ajustes, status, tela cheia, ajustes IA/automações
            ├── funil.css             # B6: funis, Kanban
            ├── automacoes.css        # B7: lista de automações, editor de fluxo, execuções, chatbot
            └── editor-ia.css         # B8: editor de IA
```

**Structure Decision**: só `app/` muda. A divisão de `app.css`/`automacoes.css` acontece **na
fundação**, como movimento mecânico (mesmo conteúdo, ordem de importação preservando a ordem
original das regras), provado por captura pixel a pixel antes de qualquer mudança visual. Depois
disso cada bloco é dono exclusivo de um arquivo CSS de área e dos componentes TSX da área — nenhum
arquivo é editado por dois blocos.

### Mapa da divisão (linhas atuais → arquivo novo)

| Origem (linhas) | Seção atual | Arquivo novo | Dono |
|---|---|---|---|
| app.css 1–105 | reset, tipografia, foco, utilitários de texto | `base.css` | fundação |
| app.css 106–512 | botões, campos | `controles.css` | fundação |
| automacoes.css 3–130 | utilitários, interruptor | `controles.css` | fundação |
| app.css 2573–2597 | `.abas`/`.aba` | `controles.css` | fundação |
| app.css 653–885, 968–1097 | telas genéricas, faixas, esqueleto, menus, modais, visualizador | `estrutura.css` | fundação |
| app.css 2483–2570, 2734–2765, 2858–2885 | tabelas, `.selo-estado`, `.lista-definicoes` | `estrutura.css` | fundação |
| app.css 513–652 | layout, trilho, avatar, ponto de estado | `shell.css` | B1 |
| app.css 1098–1347 | conversas, busca de mensagens | `conversas.css` | B1 |
| app.css 1348–2197 | chat, menu da mensagem, mídia, composer, painel do contato | `conversa.css` | B2 |
| automacoes.css 747–786 | faixa de automações na conversa | `conversa.css` | B2 |
| app.css 2690–3083 (menos selo-estado e lista-definicoes) | disparos | `disparos.css` | B3 |
| app.css 2301–2482, 2598–2689 | listas simples, contatos, templates, etiquetas, importação | `crm.css` | B4 |
| app.css 886–967, 2198–2300, 3084–3238 | tela cheia, conectar conta, ajustes, status | `ajustes.css` | B5 |
| automacoes.css 787–835 | ajustes de automações/IA | `ajustes.css` | B5 |
| automacoes.css 131–411 | funis, Kanban | `funil.css` | B6 |
| automacoes.css 412–746, 1075–1356 | lista de automações, editores, execuções, chatbot | `automacoes.css` | B7 |
| automacoes.css 836–1074 | editor de IA | `editor-ia.css` | B8 |

(As faixas exatas são conferidas na tarefa de divisão; regra que não couber num bloco vai para
`estrutura.css`.)

## Complexity Tracking

| Violação / desvio | Por que é necessário | Alternativa mais simples rejeitada porque |
|---|---|---|
| Preferência de tema fora da API do motor (Princípio III: "toda ação da interface exposta na API") | É estado de apresentação da janela, como tamanho e posição; não é dado de negócio nem afeta o que o MCP vê | Guardar no motor exigiria rota, migração e ferramenta MCP para algo que nenhum cliente além da janela usa |
| Mudança no processo principal (userData temporário no modo falso) | Sem isso, o app de teste sai na hora quando o app instalado está aberto (trava de instância única compartilhada) e as capturas não rodam | Pedir ao usuário para fechar o app a cada captura contraria o pedido (ele pode estar usando) |
| Cor literal no processo principal (`janela.ts` backgroundColor) | O processo principal não lê variáveis CSS; é só a cor antes do primeiro paint | Tirar o `backgroundColor` causaria um flash branco no escuro |
