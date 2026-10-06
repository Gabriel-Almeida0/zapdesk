# Data model — 003 Identidade visual

Nenhuma tabela, migração, rota ou evento do motor muda. As "entidades" desta feature são de
apresentação e de verificação.

## 1. Token visual

| Campo | Tipo | Regra |
|---|---|---|
| nome | `--kebab-case` pt-BR | único em `tema.css`; lista fechada em `contracts/tokens.md` §1–§2 |
| claro | cor CSS | obrigatório |
| escuro | cor CSS | obrigatório, exceto tokens `--tela*`, `--sinal*`, `--tinta`, `--tecla-sombra`, `--qr-*` (fixos) |
| uso | `texto` \| `preenchimento` \| `borda` \| `foco` \| `decorativo` | `--sinal` nunca `texto` no claro |

Definido em três blocos de `tema.css`: `:root` (claro), `@media (prefers-color-scheme: dark) {
:root:not([data-tema="claro"]) {…} }` e `:root[data-tema="escuro"] {…}` (os dois últimos com os
mesmos valores). Proibido: matiz verde (90°–170°, saturação > 25%).

## 2. Alias de token antigo (transitório)

`antigo → novo` (tabela em `contracts/tokens.md` §3). Vive da fundação (T010) até a integração
(T053). Estado: `ativo` → `sem uso` (grep = 0 fora de `tema.css`) → `removido`.

## 3. Preferência de tema

| Campo | Tipo | Regra |
|---|---|---|
| valor | `'sistema' \| 'claro' \| 'escuro'` | padrão `'sistema'`; valor desconhecido = `'sistema'` |
| onde | `localStorage['zapdesk.tema']` do renderer | leitura/escrita em try/catch; falha = `'sistema'` |

Transições: `sistema → claro|escuro` grava a chave e põe `data-tema` no `<html>`;
`claro|escuro → sistema` **apaga** a chave e remove `data-tema`. Tema efetivo =
preferência ≠ sistema ? preferência : (`prefers-color-scheme: dark` ? escuro : claro); muda ao
vivo com o evento `change` do `matchMedia`.

## 4. Par de contraste (`app/scripts/contraste.mjs`)

| Campo | Regra |
|---|---|
| tema | `claro` \| `escuro` \| `tela` \| `dados` |
| frente / fundo | nomes de token (ou cor de dados) |
| tipo | `texto` (≥ 4,5) \| `grande` (≥ 3) \| `ui` (≥ 3) |
| proibido | par documentado que não pode ser usado (não conta como falha) |

Pares mínimos (para `claro` e `escuro`, sobre `fundo`, `fundo-2`, `superficie`, `superficie-2`):
`texto`, `texto-2`, `sinal-texto`, `coral`, `ciano` (texto); `linha-forte`, `foco` (ui);
`tinta` sobre `sinal` e `sinal-hover` (texto); `texto` sobre `sinal-suave`, `coral-suave`,
`ciano-suave` (texto); `coral`/`ciano`/`sinal-texto` sobre os três suaves (texto).
Tela: `tela-texto`, `tela-texto-2`, `tela-sinal`, `tela-ciano`, `tela-coral` sobre `tela` e
`tela-2`. Dados: cada cor da paleta sugerida sobre `superficie` (ui) nos dois temas; 12 matizes do
remetente (`hsl(h 55% var(--remetente-l))`) sobre `superficie` e `superficie-2` (texto); status
`tela-texto` sobre `hsl(h 35% 30%)` (texto). Proibidos: `sinal` como texto no claro; `foco` sobre
`sinal` no escuro (o offset cai no fundo).

## 5. Captura

| Campo | Regra |
|---|---|
| tela | id da tabela de `contracts/capturas.md` |
| tema | `claro` \| `escuro` |
| tamanho | `1280x820` \| `960x620` |
| rótulo | `antes`, `divisao`, `fundacao`, `b1`…`b8`, `depois` |
| caminho | `docs/verificacao/003/<rotulo>/<tela>-<largura>-<tema>.png` |

## 6. Ícones gerados

| Arquivo | Tamanho | Conteúdo |
|---|---|---|
| `app/resources/icone.png` | 1024×1024 RGBA | tecla âmbar com profundidade + "Z" `--tinta`, corpo 824×824 centrado |
| `app/resources/bandejaTemplate.png` | 16×16 RGBA | silhueta preta da tecla, "Z" vazado |
| `app/resources/bandejaTemplate@2x.png` | 32×32 RGBA | idem |
