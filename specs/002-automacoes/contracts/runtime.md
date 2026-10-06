# Contrato de runtime — acréscimos da feature 002

Base: `specs/001-zapdesk-mvp/contracts/runtime.md` (flags, `pronto`, encerramento,
`runtime.json`, modo falso). Versão do contrato de runtime continua `1` (acréscimos compatíveis).

## Novas flags do motor

| Flag | Variável | Padrão | Descrição |
|------|----------|--------|-----------|
| `--runner-exec <caminho>` | `ZAPDESK_RUNNER_EXEC` | vazio | executável que roda o runner (app: `process.execPath` do Electron; testes: `node`) |
| `--runner-script <caminho>` | `ZAPDESK_RUNNER_SCRIPT` | vazio | `zapdesk-runner.mjs` (empacotado: `Contents/Resources/runner/zapdesk-runner.mjs`; dev: `automacao/runner/dist/zapdesk-runner.mjs`) |
| `--ia <real\|falsa>` | `ZAPDESK_IA` | `real` | `falsa` = IA simulada determinística (testes, CI); nunca abre rede |
| `--aguardar-segredos` | — | desligado | não inicia o agendador/despachante de automações até receber o primeiro comando `segredos` no stdin (máx. 2 s; depois segue sem segredos) |

Sem `--runner-exec`/`--runner-script`: automações de IA não executam (`runner_indisponivel`;
`GET /sistema` → `runner_disponivel: false`); fluxos e chatbots funcionam normalmente.
Com `--runner-exec` apontando para o Electron, o motor define `ELECTRON_RUN_AS_NODE=1` só no
ambiente do runner.

O `pronto` continua sendo emitido antes de qualquer automação executar; recuperação de
execuções/esperas (vencimento de 24 h) roda depois do `pronto`, em segundo plano.

## Canal de controle no stdin (app → motor)

O stdin, que no MVP só servia para detectar o fim do app (EOF), passa a aceitar **linhas JSON**
de controle. Linhas inválidas são ignoradas (log `aviso`, sem conteúdo). EOF continua encerrando o
motor.

```json
{"comando":"segredos","valores":{"ANTHROPIC_API_KEY":"sk-ant-...","OPENAI_KEY":"..."}}
```

- `segredos`: **substitui** o conjunto inteiro em memória (ausente = removido). Nomes
  `[A-Z][A-Z0-9_]{0,63}`, valores 1–4.096. O motor: publica `segredos.alterados` (só nomes);
  encerra os processos de runner de automações que declaram um segredo alterado (reiniciam na
  próxima execução). Nunca loga valores.
- O app envia `segredos` logo após o `pronto` (sempre, mesmo vazio) e a cada alteração em Ajustes.

## Runner (processo filho do motor)

```text
<runner-exec> --permission --allow-fs-read=<runner-script> --allow-fs-read=<bundle.mjs>
              --max-old-space-size=<memoria_mb> <runner-script>
env: ELECTRON_RUN_AS_NODE=1 (se Electron), TZ=<fuso do motor>, LANG=pt_BR.UTF-8 — nada mais
stdio: stdin/stdout = JSON-RPC (runner-protocolo.md), stderr = capturado pelo motor (últimas 200 linhas)
```

- Um processo por (automação, hash compilado). Ociosidade → `encerrar`; motor encerrando →
  `encerrar` + `SIGKILL` após 2 s; stdin fechado (motor morreu) → o runner sai sozinho.
- Nenhum runner sobrevive ao motor (Constituição II); o motor os encerra antes de sair do
  encerramento gracioso de 5 s.

## App (processo principal) — acréscimos

- Passa ao motor `--runner-exec process.execPath --runner-script <caminho>` e
  `--aguardar-segredos`.
- Guarda segredos com `safeStorage` em `<pasta-dados>/segredos.bin` (0600, JSON cifrado) e os envia
  pelo stdin. IPC novos (preload): `segredos.listar()` → `[{nome, mascara}]` (`mascara` =
  `sk-ant-…` + 4 últimos), `segredos.definir(nome, valor)`, `segredos.remover(nome)`,
  `abrirPastaExterna(caminho)` (só caminhos dentro de `<pasta-dados>/automacoes/`),
  `aoNotificacaoClicada`.
- `electron-builder.yml`: `extraResources` ganha `../automacao/runner/dist/zapdesk-runner.mjs →
  runner/zapdesk-runner.mjs`; fuse `runAsNode` continua ligado.

## Modo falso — acréscimos (`--whatsapp=falso`)

| Método e caminho | Corpo | Efeito |
|------------------|-------|--------|
| `PUT /v1/falso/ia` | `{"respostas":[{"contem":"preço","texto":"O valor é R$ 10"}],"erro"?:{"status":529,"vezes":2}}` | respostas da IA simulada por trecho do prompt (primeira que casar; senão padrão) e erro injetado nas próximas N chamadas (testa retentativa); só com `--ia=falsa` |
| `GET /v1/falso/ia/chamadas` | — | `[{modelo, sistema: string\|null, mensagens_resumo: string, esquema?, em}]` (asserts; `mensagens_resumo` = texto das mensagens enviadas à IA, resumido) |
| `POST /v1/falso/segredos` | `{"valores":{...}}` | mesmo efeito do comando `segredos` do stdin (testes de integração sem app) |
| `POST /v1/falso/processar-esperas` | — | força uma passada do agendador de esperas e só responde quando os despachos terminarem (determinismo nos testes) |

O `PUT /v1/falso/relogio` do MVP acorda também o agendador de automações.
