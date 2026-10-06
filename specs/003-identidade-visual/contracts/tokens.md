# Contrato — tokens visuais (`app/src/renderer/estilos/tema.css`)

Fonte dos valores: `zapdesk-site/src/app/globals.css` (sistema "mesa de operação"). Este arquivo é
o contrato entre a fundação e os blocos: **blocos só usam os nomes da §1 e §2**; nenhum bloco cria
token (faltou? `pendencias.md`, prefixo do bloco). Atributo de tema manual: `data-tema="claro" |
"escuro"` no `<html>` (ausente = segue o sistema).

## 1. Tokens base (iguais à landing)

| Token | Claro | Escuro | Uso permitido |
|---|---|---|---|
| `--fundo` | `#F4F1E8` | `#0E1013` | fundo da janela, área da conversa, campo de busca |
| `--fundo-2` | `#EAE5D7` | `#14171B` | item selecionado/hover, faixas de seção, esqueleto |
| `--superficie` | `#FFFFFF` | `#1A1E23` | painéis, cartões, bolha recebida, campos |
| `--superficie-2` | `#F9F7F1` | `#222730` | cabeçalhos de painel, trilho, bolha enviada, cartão de passo |
| `--texto` | `#15171A` | `#ECE8DF` | texto principal e títulos |
| `--texto-2` | `#545A63` | `#A3A9B3` | texto de apoio, legendas, ícones neutros, estados neutros |
| `--linha` | `#D6CFBF` | `#2C323B` | divisórias e bordas de cartões (decorativas) |
| `--linha-forte` | `#857E70` | `#737B88` | bordas de controles (campos, botões secundários), bolha enviada |
| `--sinal` | `#FFB020` | `#FFB020` | **preenchimento** âmbar: botão primário, pílula de não lidas, lâmpada, item ativo, progresso |
| `--sinal-hover` | `#FFC24D` | `#FFC24D` | hover do preenchimento âmbar |
| `--sinal-texto` | `#8A5300` | `#FFB020` | âmbar como **texto/ícone/link** |
| `--coral` | `#B23C1C` | `#FF8A6B` | estado de falha, ação destrutiva, erro |
| `--ciano` | `#0F6676` | `#6CC8DC` | estado positivo (lido, respondeu, concluído, conectado) |
| `--tinta` | `#121417` | `#121417` | texto sobre âmbar (2 temas) |
| `--borda-primario` | `#15171A` | `transparent` | contorno do botão primário |
| `--foco` | `#8A5300` | `#FFB020` | anel de foco |
| `--pontos` | `rgb(21 23 26 / .1)` | `rgb(236 232 223 / .07)` | grade pontilhada (canvas do chatbot) |
| `--tecla-sombra` | `#C98500` | `#C98500` | profundidade da tecla do logotipo |
| `--tela` | `#0A0C0F` | (igual) | editor de código, terminal, visualizador de mídia/status |
| `--tela-2` | `#14171B` | (igual) | barras/cabeçalhos da tela escura |
| `--tela-linha` | `#262B33` | (igual) | linhas na tela escura |
| `--tela-texto` | `#ECE8DF` | (igual) | texto na tela escura |
| `--tela-texto-2` | `#A3A9B3` | (igual) | comentários/apoio na tela escura |
| `--tela-sinal` | `#FFB020` | (igual) | keyword / prompt |
| `--tela-ciano` | `#6CC8DC` | (igual) | string / resultado |
| `--tela-coral` | `#FF8A6B` | (igual) | número / erro |

**Proibido**: `--sinal` como cor de texto no tema claro (1,9:1); usar `--sinal-texto`. Verde em
qualquer token.

## 2. Tokens derivados do app (novos; só a partir dos base)

| Token | Definição | Uso |
|---|---|---|
| `--sinal-suave` | `color-mix(in srgb, var(--sinal) 16%, var(--superficie))` | fundo de faixa de aviso, destaque de busca (`mark`) |
| `--coral-suave` | `color-mix(in srgb, var(--coral) 12%, var(--superficie))` | fundo de faixa/caixa de erro |
| `--ciano-suave` | `color-mix(in srgb, var(--ciano) 12%, var(--superficie))` | fundo de faixa de informação/sucesso |
| `--sobreposicao` | claro `rgb(21 23 26 / .32)`, escuro `rgb(0 0 0 / .55)` | fundo de modal |
| `--sombra-painel` | `0 1px 0 var(--linha), 0 24px 48px -24px rgb(0 0 0 / .25)` | menus flutuantes, modais, cartão arrastado |
| `--remetente-l` | claro `28%` (era 34%: reprovava matizes amarelos; ajuste da fundação), escuro `72%` | luminosidade do nome do remetente em grupo |
| `--qr-fundo` / `--qr-tinta` | `#FFFFFF` / `#000000` (fixos) | QR code (leitura pelo celular) |
| `--raio-tecla` | `10px` | botões, campos, cartões pequenos |
| `--raio-painel` | `14px` | modais, painéis grandes |
| `--raio-pilula` | `999px` | selos, chips, contadores |
| `--raio-bolha` | `10px` (canto "rabinho" `3px`) | bolhas de mensagem |
| `--fonte-titulo` | `'Bricolage Grotesque Variable', ui-sans-serif, system-ui, sans-serif` | títulos, wordmark, números de destaque |
| `--fonte` | `'Instrument Sans Variable', -apple-system, system-ui, sans-serif` | interface |
| `--fonte-mono` | `'JetBrains Mono Variable', ui-monospace, SFMono-Regular, Menlo, monospace` | horas, telefones, rótulos, código |
| `--ease-mesa` | `cubic-bezier(.2,.7,.2,1)` | transições |

## 3. Camada de transição (aliases antigos → novos)

Existe da fundação até a integração (T053). Bloco que terminar o seu arquivo não usa nenhum destes.

| Antigo | Novo | Observação |
|---|---|---|
| `--verde` | `--sinal` | preenchimentos (botão, selo, interruptor ligado) |
| `--verde-forte` | `--sinal-texto` | links, ícones e texto de destaque |
| `--verde-suave` | `--sinal-suave` | |
| `--azul-lido` | `--ciano` | vistos de "lida" |
| `--perigo` | `--coral` | |
| `--aviso-fundo` / `--aviso-borda` / `--aviso-texto` | `--sinal-suave` / `--sinal` / `--texto` | |
| `--erro-fundo` / `--erro-texto` | `--coral-suave` / `--coral` | |
| `--info-fundo` / `--info-texto` | `--ciano-suave` / `--ciano` | |
| `--fundo-app` | `--fundo` | |
| `--fundo-painel` | `--superficie` | |
| `--fundo-cabecalho` / `--fundo-trilho` | `--superficie-2` | |
| `--fundo-hover` / `--fundo-selecionado` | `--fundo-2` | |
| `--fundo-campo` | `--fundo` | |
| `--fundo-chat` | `--fundo` | |
| `--bolha-minha` | `--superficie-2` | + borda `--linha-forte` (B2) |
| `--bolha-dele` | `--superficie` | + borda `--linha` (B2) |
| `--bolha-citacao` | `--fundo-2` | |
| `--sombra-bolha` | `none` | linhas no lugar de sombra |
| `--sombra-menu` | `--sombra-painel` | |
| `--texto` / `--texto-2` | `--texto` / `--texto-2` | mesmo nome, valores novos |
| `--texto-3` | `--texto-2` | `--texto-3` sumia no AA |
| `--texto-inverso` | `--tinta` | texto sobre âmbar |
| `--borda` / `--borda-forte` | `--linha` / `--linha-forte` | |
| `--esqueleto` / `--esqueleto-brilho` | `--fundo-2` / `--superficie-2` | |
| `--raio` / `--raio-bolha` | `--raio-tecla` / `--raio-bolha` | |
| `--fonte` / `--fonte-mono` | mesmos nomes, pilhas novas | |

## 4. Exceções à regra "nenhuma cor literal fora de tema.css"

1. Paletas sugeridas de dados: `CORES_ETIQUETA` (`telas/Etiquetas.tsx`), `CORES_ETAPA`
   (`telas/Funis.tsx`) — marcadas `// cores-de-dados`; cores salvas pelo usuário (inline).
2. `hsl(<matiz> 55% var(--remetente-l))` em `componentes/Bolha.tsx`; `hsl(<matiz> 35% 30%)` em
   `telas/Status.tsx` (sobre `--tela-texto`).
3. `app/src/main/janela.ts` `backgroundColor` (`#F4F1E8` / `#0E1013`, comentário apontando para
   esta tabela).
4. `app/scripts/gerar-icones.mjs` (cores do logotipo) e `app/scripts/contraste.mjs`.
