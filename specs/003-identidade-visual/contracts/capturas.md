# Contrato — capturas sem WhatsApp real (`app/scripts/capturar.mjs`)

## Garantias

- Nunca usa a conta nem os dados reais: motor com `--whatsapp=falso` (via `ZAPDESK_DEV_FALSO=1`),
  `ZAPDESK_PASTA_DADOS` e `ZAPDESK_PASTA_INTERFACE` em `mkdtemp(os.tmpdir()/zapdesk-captura-*)`,
  apagadas no fim (também em erro/SIGINT).
- Convive com o ZapDesk instalado aberto (trava de instância única em `userData` separado).
- Não abre rede além de `127.0.0.1` (motor). Sem chave de IA.
- Determinístico: mesma semente, relógio fixado (`PUT /v1/falso/relogio {"agora":"2026-10-05T10:42:00-03:00"}`),
  animações desligadas na captura (`reducedMotion: 'reduce'` + `caret-color: transparent`).

## Pré-requisitos

`npm install` (raiz), `npm run motor:compilar`, `npm run compartilhado:compilar`,
`npm run compilar -w @zapdesk/app` (gera `app/out/`). Playwright de
`$PLAYWRIGHT_PATH` (`_electron`); Electron do
workspace (`require.resolve('electron', { paths: [app] })`).

## CLI

```text
node app/scripts/capturar.mjs [opções]
  --saida <dir>            obrigatório; ex.: docs/verificacao/003/antes
  --telas <id,id,…>        padrão: todas da tabela abaixo
  --temas claro,escuro     padrão: ambos (nativeTheme.themeSource = 'light' | 'dark')
  --tamanhos 1280x820,960x620   padrão: ambos
  --comparar <dir>         compara cada PNG com o de mesmo nome em <dir>; imprime % de pixels
                           diferentes (máscara nas áreas [data-captura-mascara] e horas relativas);
                           sai 1 se algum > 0 (use após a divisão do CSS)
  --medir                  grava <saida>/medidas.json: altura dos itens de listas virtuais,
                           layout-shift acumulado na abertura (PerformanceObserver), fontes
                           carregadas (document.fonts) e requisições de rede não-127.0.0.1
  --manter                 não fecha o app no fim (inspeção manual)
```

Nome do arquivo: `<tela>-<largura>-<tema>.png` (ex.: `conversa-1280-escuro.png`).

## Telas (`--telas`)

| id | Como chega | Estado da semente | Mockup de referência |
|---|---|---|---|
| `conversas` | `#/conversas` | lista com 6 conversas (não lidas, grupo, etiquetas) | `ChatMockup` (lista) |
| `conversa` | `#/conversas/<Marina>` + painel do contato aberto | texto, áudio, documento, imagem, citação, "lida", data | `ChatMockup` |
| `conversa-menu` | idem + menu da mensagem aberto | | — |
| `busca` | busca de mensagens aberta com termo | `mark` | — |
| `nova-conversa` | modal "Nova conversa" | | — |
| `contatos` | `#/contatos` | 8 contatos | — |
| `status` | `#/status` + visualizador aberto | 2 status de texto | — |
| `disparos` | `#/disparos` | 1 em andamento, 1 agendado, 1 concluído, 1 cancelado | — |
| `disparo-novo-1`…`-4` | `#/disparos/novo` e "Avançar" | lista, mensagem, ritmo, revisão | `DisparoMockup` (passo 3) |
| `disparo-detalhe` | `#/disparos/<emAndamento>` | 12 destinatários: enviado, entregue, lido, respondeu, falhou, pendente; pausado | `RelatorioMockup` |
| `leads` | `#/leads` | 12 leads | — |
| `leads-importar` | `#/leads/importar` com CSV fictício + relatório | | — |
| `templates` | `#/templates` | 3 templates, 1 selecionado | — |
| `etiquetas` | `#/etiquetas` + modal nova etiqueta | 4 etiquetas (incl. uma verde antiga) | — |
| `funis` | `#/funis` | 1 funil | — |
| `kanban` | `#/funis/<Prospecção>` | 4 etapas, 7 cartões | `KanbanMockup` |
| `automacoes` | `#/automacoes` | fluxo ativo, chatbot ativo, IA inativa | — |
| `editor-fluxo` | `#/automacoes/<fluxo>` | gatilho, 2 condições, 3 ações | — |
| `editor-chatbot` | `#/automacoes/<chatbot>` + um nó selecionado | 5 nós como no mockup | `ChatbotMockup` |
| `editor-ia` | `#/automacoes/<ia>` | projeto do modelo `responder_historico`, arquivo `index.ts` aberto | `TerminalMockup`/blocos de código |
| `execucoes` | `#/execucoes/<id>` | 1 ok, 1 erro | — |
| `ajustes` | `#/ajustes` (rolagem até Aparência quando existir) | 1 conta conectada | — |
| `conectar-conta` | `#/contas/conectar` | QR visível | — |
| `modal-confirmar` | ação "Remover conta" (cancelada) | | — |
| `carregando` | app lançado com motor atrasado (wrapper em tmp) | | — |
| `erro-motor` | app lançado com `ZAPDESK_MOTOR_BIN=/usr/bin/false` | | — |
| `vazio` | app novo sem semente, `#/conversas` e `#/disparos` | estados vazios | — |

Telas que dependem de ids leem `<pasta-temporária>/semente.json` escrito por
`semente-falsa.mjs`. Uma tela que não puder ser alcançada no "antes" (ex.: Aparência) é pulada com
aviso, não erro.

## Semente fictícia (`app/scripts/semente-falsa.mjs`)

Exporta `semear({ base, token })`. Usa só a API do motor e `/v1/falso/*`
(`specs/001-zapdesk-mvp/contracts/runtime.md`, `specs/002-automacoes/contracts/api-http.md`). Dados
fictícios espelhando `zapdesk-site/src/content/mockups.ts` (Marina Couto, Ateliê Fio de Ouro,
Rafael Nunes, Clínica Bem Viver, grupo "Fornecedores"…), telefones `+55 11 90000-01xx`, conta
"Comercial". Passos: conta + `escanear-qr`; `historico` com as conversas; `mensagem-recebida`
para não lidas; `recibo` lido/entregue; etiquetas, leads, contatos, templates; funil com 4
etapas e cartões; automações (fluxo, chatbot de `formatos.md`, IA de modelo); disparo com
`falhas-envio` para 2 números, iniciado e pausado após progresso; `status` de 2 contatos. Devolve os
ids (gravados em `semente.json`).

## Acréscimos da implementação (fundação)

- Opções extras: `--compilar` (compila `app/out` dentro do lock `/tmp/zapdesk-captura.lock`; use
  sempre com agentes em paralelo) e `--escala css|dispositivo` (padrão `css`: PNG 1×).
- Telas extras: `execucoes-ok` (execução com sucesso); `vazio-disparos` (Disparos sem dados).
  `execucoes` mostra a execução com **erro**.
- Saídas extras: `<saida>/mascaras.json` (áreas ignoradas), `<saida>/comparacao.json` e
  `<saida>/diferencas/*.png` (pixels diferentes em vermelho) com `--comparar`.
- Máscaras fixas: avatar da conta e fundo do status de texto (cor derivada de id aleatório), QR,
  textos "N ms"; a captura desliga transições e esconde cursor/barras do Monaco.

## Acréscimos da integração (T052)

- `--roteiro <arquivo.mjs>`: depois de abrir o app com semente, chama o `default` do arquivo com
  `{ app, page, ids, tmp, esperar, api, tema, tamanho, ir, foto, medir }` (interações: teclado,
  arrastar, Testar). Sem `--telas`, não fotografa a tabela. Mesmo lock, pasta temporária e limpeza.
- `--app <dir>`: lança outro build (`<dir>/out`), ex.: a base pré-restyle para comparar
  comportamento. A compilação (`--compilar`) continua sendo a de `app/`.
