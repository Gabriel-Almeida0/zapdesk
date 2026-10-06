# Contrato do servidor MCP `zapdesk` — v1

Transporte: stdio. SDK: `@modelcontextprotocol/server` v2 + zod 4. Nome do servidor: `zapdesk`.
Cada ferramenta é um cliente fino da API HTTP (`api-http.md`) via `@zapdesk/cliente-motor`.

## Regras gerais

- **Garantir app**: antes de cada chamada, `garantirMotor()` (regras de `runtime.md` ›
  `runtime.json`): abre o app se fechado e espera até 30 s. Falha → `isError: true` com texto
  "Não consegui ligar o ZapDesk. Abra o app manualmente e tente de novo."
- **Retorno**: `structuredContent` com o objeto da API (conforme `outputSchema`) +
  `content: [{type:"text", text}]` com um resumo curto em português (ex.: "7 leads novos, 3 já
  existiam").
- **Erros da API** → `isError: true`, texto = `erro.mensagem` + detalhes relevantes (ex.: lista de
  variáveis faltando). Nunca lançar exceção não tratada.
- **Conta padrão**: parâmetro `conta_id` é opcional quando existe exatamente uma conta
  `conectada`; com zero ou várias, erro pedindo `conta_id` e listando as contas.
- **Paginação**: `limite` (padrão 50, máx 200) e `cursor`; retorno inclui `proximo_cursor`.
- **Annotations**: ferramentas de leitura com `readOnlyHint: true`; `criar_disparo`,
  `enviar_mensagem`, `apagar_mensagem` com `destructiveHint: true` / `openWorldHint: true`.
- Telefones aceitos em qualquer formato; o motor normaliza.

## Ferramentas

Notação dos esquemas: `campo: tipo` (`?` = opcional). Todas as saídas seguem os tipos de
`api-http.md`.

### Contas e sistema

| Ferramenta | Entrada | Saída | API |
|------------|---------|-------|-----|
| `listar_contas` | — | `{contas: Conta[]}` | `GET /contas` |
| `status_zapdesk` | — | `{versao, contas_conectadas, disparos_ativos, pasta_dados}` | `GET /sistema` |

Conectar conta nova (QR) só pela interface; a descrição de `listar_contas` informa isso.

### Leads

| Ferramenta | Entrada | Saída | API |
|------------|---------|-------|-----|
| `importar_leads` | `leads: {telefone: string, nome?: string, campos?: Record<string,string>}[]` (1–50.000) **ou** `caminho_arquivo: string` (CSV/XLSX local) + `coluna_telefone?: string`, `coluna_nome?: string`; `ddi_padrao?: string` | `RelatorioImportacao` | `POST /leads/importar` (forma C, `origem:"mcp"`) ou `POST /importacoes/previa` + `POST /leads/importar` (forma A; coluna sugerida se omitida) |
| `listar_leads` | `busca?: string`, `origem?: enum`, `limite?`, `cursor?` | página de `Lead` | `GET /leads` |

Texto de retorno de `importar_leads`: "N novos, M já existiam (lista com data de importação), K
inválidos, D duplicados no lote".

### Conversas e mensagens

| Ferramenta | Entrada | Saída | API |
|------------|---------|-------|-----|
| `listar_conversas` | `conta_id?`, `nao_lidas?: bool`, `etiqueta_id?`, `busca?`, `limite?`, `cursor?` | página de `Conversa` | `GET /contas/{id}/conversas` |
| `ler_mensagens` | `conversa_id?` **ou** `telefone?` (+ `conta_id?`), `limite?` (padrão 30), `antes?` | `{conversa: Conversa, mensagens: Mensagem[]}` (ordem cronológica) | `GET /conversas/{id}/mensagens` |
| `buscar_mensagens` | `termo: string`, `conta_id?`, `limite?` | página de resultados | `GET /contas/{id}/mensagens/busca` |
| `enviar_mensagem` | `telefone?` **ou** `conversa_id?`; `texto?`; `caminho_anexo?: string`; `como?: "auto"\|"voz"\|"figurinha"\|"documento"`; `citar_mensagem_id?`; `conta_id?` (texto ou anexo obrigatório) | `Mensagem` | `POST /contas/{id}/conversas` (se telefone) + `POST /arquivos` (se anexo) + `POST /conversas/{id}/mensagens` |
| `reagir_mensagem` | `mensagem_id`, `emoji` (`""` remove) | `{ok: true}` | `POST /mensagens/{id}/reacao` |
| `editar_mensagem` | `mensagem_id`, `texto` | `Mensagem` | `PATCH /mensagens/{id}` |
| `apagar_mensagem` | `mensagem_id` | `{ok: true}` | `DELETE /mensagens/{id}` |
| `marcar_como_lida` | `conversa_id` | `{ok: true}` | `POST /conversas/{id}/lida` |
| `ver_status` | `conta_id?` | lista de status por contato | `GET /contas/{id}/status` |

### Contatos, etiquetas e notas

| Ferramenta | Entrada | Saída | API |
|------------|---------|-------|-----|
| `listar_contatos` | `conta_id?`, `busca?`, `etiqueta_id?`, `limite?`, `cursor?` | página de `Contato` | `GET /contas/{id}/contatos` |
| `atualizar_contato` | `contato_id`, `notas?: string`, `etiqueta_ids?: string[]` | `Contato` | `PATCH /contatos/{id}`, `PUT /contatos/{id}/etiquetas` |
| `listar_etiquetas` | — | `{etiquetas: Etiqueta[]}` | `GET /etiquetas` |
| `criar_etiqueta` | `nome` (1–30), `cor` (`#RRGGBB`) | `Etiqueta` | `POST /etiquetas` |
| `atualizar_etiqueta` | `etiqueta_id`, `nome?`, `cor?` | `Etiqueta` | `PATCH /etiquetas/{id}` |
| `excluir_etiqueta` | `etiqueta_id` | `{ok: true}` | `DELETE /etiquetas/{id}` |

### Templates

| Ferramenta | Entrada | Saída | API |
|------------|---------|-------|-----|
| `listar_templates` | `busca?` | `{templates: Template[]}` | `GET /templates` |
| `criar_template` | `nome` (1–60), `texto` (1–4096), `caminho_anexo?` | `Template` | `POST /arquivos` + `POST /templates` |
| `atualizar_template` | `template_id`, `nome?`, `texto?`, `caminho_anexo?: string\|null` | `Template` | `PATCH /templates/{id}` |
| `excluir_template` | `template_id` | `{ok: true}` | `DELETE /templates/{id}` |

### Disparos

| Ferramenta | Entrada | Saída | API |
|------------|---------|-------|-----|
| `criar_disparo` | ver abaixo | `{disparo: Disparo, relatorio_importacao?: RelatorioImportacao}` | `POST /arquivos` (se anexo) + `POST /disparos` com `iniciar: true`, `origem: "mcp"` |
| `listar_disparos` | `conta_id?`, `estado?`, `limite?`, `cursor?` | página de `Disparo` | `GET /disparos` |
| `ver_disparo` | `disparo_id`, `estado_destinatarios?`, `limite?`, `cursor?` | `{disparo: Disparo, destinatarios: página de Destinatario}` | `GET /disparos/{id}` + `/destinatarios` |
| `iniciar_disparo` | `disparo_id`, `valores_padrao?` | `Disparo` | `POST /disparos/{id}/iniciar` (rascunhos criados no app) |
| `pausar_disparo` | `disparo_id` | `Disparo` | `POST /disparos/{id}/pausar` |
| `retomar_disparo` | `disparo_id` | `Disparo` | `POST /disparos/{id}/retomar` |
| `cancelar_disparo` | `disparo_id` | `Disparo` | `POST /disparos/{id}/cancelar` |
| `exportar_relatorio` | `disparo_id`, `caminho_destino?: string` (padrão `~/Downloads/zapdesk-<nome>-<data>.csv`) | `{caminho, linhas}` | `GET /disparos/{id}/relatorio.csv` |

Entrada de `criar_disparo`:

```text
conta_id?: string
nome?: string
mensagem?: string            // obrigatório se não houver template_id
template_id?: string
caminho_anexo?: string
destinatarios: {             // pelo menos uma fonte
  leads?: {telefone, nome?, campos?}[]   // importados com origem "mcp" (dedup) e usados
  lead_ids?: string[]
  etiqueta_ids?: string[]
  contato_ids?: string[]
}
intervalo_min_s: int ≥ 1
intervalo_max_s: int ≥ intervalo_min_s
limite_por_hora?: int > 0
limite_por_dia?: int > 0
pausa_a_cada?: int > 0        // exige pausa_duracao_s
pausa_duracao_s?: int > 0
inicio_em?: string (RFC 3339) // padrão: agora
janela_inicio?: "HH:MM"       // exige janela_fim
janela_fim?: "HH:MM"
falhas_seguidas_max?: int ≥ 0 (padrão 10)
valores_padrao?: Record<string,string>
```

Comportamento: cria e inicia imediatamente, sem confirmação (FR-071). Se faltar valor de
variável e não houver `valores_padrao` → `isError: true` com "N destinatários sem {var}" e a
lista (até 50 linhas). Se outro disparo da conta estiver ativo, o novo entra na fila e o texto de
retorno diz "Na fila: começa quando <nome do ativo> terminar". A descrição da ferramenta DEVE
avisar que o envio começa na hora e que `pausar_disparo`/`cancelar_disparo` existem.

## Testes (obrigatórios — Constituição VI)

`mcp/tests/` sobe um motor real com `--whatsapp=falso` e `--pasta-dados` temporária, conecta o
servidor MCP com `InMemoryTransport.createLinkedPair()` e verifica cada ferramenta: sucesso,
erro de validação e o caminho "app fechado" (com `open` substituído por função injetável que sobe
o motor de teste).
