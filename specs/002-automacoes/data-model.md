# Modelo de dados — Automações (feature 002)

Fonte: §6 de `docs/features/automacoes.md` + Clarifications de `spec.md`. Convenções do MVP
(`specs/001-zapdesk-mvp/data-model.md`): IDs ULID em texto; datas no banco em milissegundos Unix
UTC e na API em RFC 3339 com fuso; booleanos 0/1; JSON em texto; migração versionada com backup
automático. Esta feature acrescenta **`0002_automacoes.sql`**.

## Armazenamento em arquivos (novos)

```text
ZapDesk/
├── automacoes/<automacao_id>/          # projeto de automação de IA (dono: motor)
│   ├── automacao.json                  # manifesto (contracts/formatos.md)
│   ├── index.ts, *.ts, *.json, *.md, *.txt
│   ├── tsconfig.json                   # gerado; paths → .zapdesk/automacao.d.ts
│   └── .zapdesk/                       # gerenciado pelo motor, oculto na API
│       ├── automacao.d.ts              # tipos da SDK (@zapdesk/automacao)
│       └── automacao.schema.json       # JSON Schema do manifesto
├── automacoes-compiladas/<id>/<hash>.mjs  # bundles do esbuild; só o último ok e o ativo ficam
└── segredos.bin                        # dono: app; safeStorage; 0600; nunca lido pelo motor
```

Regras dos projetos: até 50 arquivos, 1 MB cada, extensões `.ts`, `.json`, `.md`, `.txt`;
caminhos relativos com `/`, sem `..`, sem começar por `.` (reservado), segmentos
`[A-Za-z0-9_.-]{1,64}`, profundidade ≤ 4. `automacao.json` e o arquivo de entrada (padrão
`index.ts`) não podem ser excluídos.

## Entidades novas

### Funil (`funis`)

| Campo | Tipo | Obrig. | Default | Validação |
|-------|------|--------|---------|-----------|
| id | ULID | sim | gerado | — |
| nome | texto | sim | — | 1–60, único sem diferenciar maiúsculas |
| ordem | inteiro | sim | último + 1 | ≥ 0 |
| criado_em, atualizado_em | data | sim | agora | — |

Excluir funil apaga etapas, posições e histórico do funil (cascata) e publica `funil.alterado`.

### Etapa (`etapas`)

| Campo | Tipo | Obrig. | Default | Validação |
|-------|------|--------|---------|-----------|
| id | ULID | sim | gerado | — |
| funil_id | FK funis (cascata) | sim | — | — |
| nome | texto | sim | — | 1–40, único no funil (sem diferenciar maiúsculas) |
| cor | texto | sim | `#8696A0` | `#RRGGBB` |
| ordem | inteiro | sim | último + 1 | ≥ 0 |
| criada_em | data | sim | agora | — |

Máximo 30 etapas por funil. Excluir etapa com posições exige `destino_etapa_id` (mesmo funil) ou
`remover_cards=true`; cada card movido/removido gera histórico com origem da requisição.

### Posição no funil / card (`posicoes_funil`)

| Campo | Tipo | Obrig. | Validação |
|-------|------|--------|-----------|
| lead_id | FK leads (cascata) | sim | — |
| funil_id | FK funis (cascata) | sim | — |
| etapa_id | FK etapas | sim | etapa pertence ao funil |
| desde | data | sim | quando entrou na etapa atual |

Chave primária `(lead_id, funil_id)` → **um lead no máximo em uma etapa por funil**. Índice
`(funil_id, etapa_id, desde DESC)`. Adicionar um contato: se `contatos.lead_id` é nulo, cria o
lead (telefone E.164 do contato, `nome` = nome ou nome_push, origem `contatos`) e liga o contato,
na mesma transação. Contato de grupo → `validacao` "Grupos não entram em funil".

Card na API (junção): `{lead_id, funil_id, etapa_id, desde, lead: {id, telefone, nome, campos},
etiquetas: Etiqueta[] (do contato mais recente do telefone), conversa_id|null (conversa mais
recente do telefone em qualquer conta)}`.

### Histórico do funil (`historico_funil`)

| Campo | Tipo | Obrig. | Observação |
|-------|------|--------|------------|
| id | ULID | sim | ordenável por tempo |
| lead_id | FK leads (cascata) | sim | — |
| funil_id | FK funis (cascata) | sim | — |
| etapa_origem_id, etapa_destino_id | texto | não | nulo = fora do funil (entrada/saída) |
| etapa_origem_nome, etapa_destino_nome | texto | não | cópia do nome (sobrevive à exclusão da etapa) |
| origem | texto | sim | `app` \| `mcp` \| `automacao` |
| automacao_id | texto | não | quando `origem = automacao` |
| execucao_id | texto | não | idem |
| em | data | sim | — |

Mover para a mesma etapa em que já está: sem efeito e sem histórico (idempotente).

### Automação (`automacoes`)

| Campo | Tipo | Obrig. | Default | Validação |
|-------|------|--------|---------|-----------|
| id | ULID | sim | gerado | — |
| tipo | texto | sim | — | `fluxo` \| `chatbot` \| `ia` |
| nome | texto | sim | — | 1–80 (para `ia`, espelha `automacao.json`) |
| descricao | texto | não | nulo | ≤ 500 |
| ativa | bool | sim | 0 | só ativa se definição válida (fluxo/chatbot) ou compilação ok (ia) |
| contas | JSON | não | nulo | nulo = todas; senão lista de `conta_id` existentes |
| incluir_grupos | bool | sim | 0 | — |
| prioridade | inteiro | sim | 100 | 1–1000; menor executa primeiro |
| conta_envio_id | FK contas (set null) | não | nulo | conta usada quando o evento não tem conversa |
| gatilhos | JSON | sim | `[]` | lista de `Gatilho` (contracts/formatos.md); ≥ 1 para ativar |
| definicao | JSON | não | nulo | `DefinicaoFluxo` ou `DefinicaoChatbot`; nulo para `ia` |
| limites | JSON | sim | `{}` | `{anti_loop?: {mensagens, janela_min}, tempo_s?, memoria_mb?}`; anti-loop só pode ser **mais restritivo** que o global |
| versao | inteiro | sim | 1 | +1 a cada gravação de definição/manifesto; guardada nas sessões e execuções |
| permissoes | JSON | não | nulo | (`ia`) cópia validada do manifesto |
| segredos | JSON | não | nulo | (`ia`) nomes declarados |
| hash_fontes | texto | não | nulo | (`ia`) hash dos arquivos na última compilação |
| hash_compilado | texto | não | nulo | (`ia`) hash da **última compilação bem-sucedida** (a que executa) |
| erros_compilacao | JSON | não | `[]` | (`ia`) `ErroCompilacao[]` da última tentativa |
| compilado_em | data | não | nulo | — |
| erros_seguidos | inteiro | sim | 0 | zera em execução `ok`; 5 → `ativa=0`, `desativada_motivo="erros_seguidos"` |
| desativada_motivo | texto | não | nulo | `erros_seguidos` \| `usuario` \| nulo |
| criada_em, atualizada_em | data | sim | agora | — |

Para `ia`, a fonte da verdade de nome, gatilhos, permissões, segredos, contas, grupos, prioridade,
conta de envio e limites é o `automacao.json`; a linha guarda a cópia validada na última
compilação (usada pelo despachante). `PATCH /automacoes/{id}` recusa esses campos para `ia`.

Estados/transições: `inativa → ativa` (validação/compilação ok) → `inativa` (usuário, 5 erros,
exclusão). Referências quebradas não desativam; a ação falha e o editor mostra aviso.

### Sessão de chatbot (`sessoes_chatbot`)

| Campo | Tipo | Obrig. | Observação |
|-------|------|--------|------------|
| id | ULID | sim | — |
| automacao_id | FK automacoes (cascata) | sim | — |
| conversa_id | FK conversas (cascata) | sim | — |
| versao | inteiro | sim | versão da automação no início |
| definicao | JSON | sim | **cópia congelada** do grafo no início (edições não afetam a sessão) |
| no_atual | texto | sim | id do nó aguardando resposta |
| variaveis | JSON | sim | `{nome: valor}` capturadas (≤ 64 KB) |
| tentativas | inteiro | sim | respostas inválidas no nó atual |
| estado | texto | sim | `ativa` \| `concluida` \| `humano` \| `expirada` \| `abortada` |
| motivo | texto | não | texto do fim (ex.: "anti-loop", "chatbot excluído") |
| expira_em | data | sim | última interação + inatividade do bot |
| iniciada_em, atualizada_em, finalizada_em | data | — | — |

Índice único parcial `(conversa_id) WHERE estado = 'ativa'` → no máximo uma sessão ativa por
conversa. Transições: `ativa → concluida` (nó fim) | `humano` (nó humano, "Assumir", resposta
manual, tentativas esgotadas sem destino) | `expirada` (agendador, `expira_em` vencido) | `abortada`
(erro, anti-loop, bot excluído/desativado, app reaberto com sessão vencida há > 24 h). Sessões
`ativa` sobrevivem ao reinício; a expiração é uma espera do tipo `expirar_sessao`.

### Execução (`execucoes`)

| Campo | Tipo | Obrig. | Observação |
|-------|------|--------|------------|
| id | ULID | sim | — |
| automacao_id | FK automacoes (cascata) | sim | — |
| automacao_versao | inteiro | sim | — |
| tipo_automacao | texto | sim | `fluxo` \| `chatbot` \| `ia` |
| gatilho | JSON | sim | `{tipo, dados}` do fato que iniciou (ex.: `mensagem_id`, `etiqueta_id`) |
| origem | texto | sim | `gatilho` \| `manual_app` \| `manual_mcp` \| `teste` \| `fluxo` \| `chatbot` |
| origem_execucao_id | texto | não | execução que chamou (fluxo/bot → IA) |
| cadeia | JSON | sim | ids de automações na cadeia causal (R11); profundidade = tamanho |
| conta_id, conversa_id, contato_id, lead_id | texto | não | alvo |
| estado | texto | sim | ver transições |
| simulacao | bool | sim | 1 = modo simulação |
| passo_atual | inteiro | não | (fluxo) índice da próxima ação |
| variaveis | JSON | sim | variáveis da execução (fluxo: `salvar_em`; bot: cópia) |
| acoes | JSON | sim | `AcaoRegistrada[]` (≤ 200 itens): `{tipo, alvo, resultado: "ok"\|"falhou"\|"bloqueada"\|"simulada", detalhe, em}` |
| log | texto | sim | ≤ 64 KB; excedente descartado e `log_truncado=1` |
| log_truncado | bool | sim | — |
| erro | texto | não | mensagem pt-BR |
| erro_stack | texto | não | (ia) stack com linhas do `.ts` (sourcemap) |
| tokens | JSON | sim | `{entrada, saida, por_modelo: {modelo: {entrada, saida, chamadas}}}` |
| retorno | JSON | não | retorno de `aoExecutar` (≤ 64 KB) |
| iniciada_em | data | sim | — |
| finalizada_em | data | não | — |
| duracao_ms | inteiro | não | tempo ativo (sem contar esperas) |
| motivo | texto | não | motivo de `abortada` (`expirada`, `interrompida`, `conversa pausada`, `app fechado`, `condições não atendidas` em `ok`) |

Estados: `na_fila → rodando → ok | erro | abortada`; `rodando ↔ aguardando` (fluxo com espera);
testes terminam como `simulacao` (ou `erro`). Na subida: `rodando`/`na_fila` → `abortada`
("interrompida"); `aguardando` segue a regra das esperas. Retenção: após cada inserção, apagar as
execuções mais antigas além das **500** por automação. Índices `(automacao_id, iniciada_em DESC)`,
`(conversa_id, iniciada_em DESC)`, `(estado)`.

### Espera agendada (`esperas`)

| Campo | Tipo | Obrig. | Observação |
|-------|------|--------|------------|
| id | ULID | sim | — |
| tipo | texto | sim | `aguardar` \| `sem_resposta` \| `agendamento` \| `agendar` (ctx.agendar) \| `expirar_sessao` |
| automacao_id | FK automacoes (cascata) | sim | — |
| execucao_id | FK execucoes (cascata) | não | (`aguardar`) execução a retomar |
| sessao_id | FK sessoes_chatbot (cascata) | não | (`expirar_sessao`) |
| conversa_id | texto | não | (`sem_resposta`, `agendar` com conversa) |
| referencia | texto | não | (`sem_resposta`) id da mensagem enviada que iniciou a contagem |
| retomar_em | data | sim | — |
| dados | JSON | sim | ex.: `{gatilho_indice}` (agendamento), `{dados}` (ctx.agendar) |
| criada_em | data | sim | — |

Coluna extra `chave` (texto, nula) com UNIQUE `(automacao_id, chave)`: `agendamento:<índice do
gatilho>` (uma próxima ocorrência por gatilho) e `sem_resposta:<conversa_id>` (uma espera por
conversa); nula nos demais tipos. Índice `(retomar_em)`. Regras:

- **sem_resposta**: a cada mensagem `de_mim` na conversa, para cada automação ativa com esse
  gatilho e filtro de origem compatível, substitui a espera da conversa (`chave = sem_resposta:<conversa_id>`) com `retomar_em = enviada_em + apos_s`; mensagem recebida do
  contato apaga as esperas `sem_resposta` da conversa.
- **Vencimento**: ao despachar, se `agora - retomar_em > 24 h` → `abortada` ("expirada") sem
  executar; conversa pausada → `abortada` ("conversa pausada"); conta desconectada → reprograma
  para 1 min depois, até completar 24 h do vencimento.
- **agendamento**: ao executar, apaga e grava a próxima ocorrência calculada a partir de agora;
  desativar a automação apaga suas esperas `agendamento`/`sem_resposta`/`agendar`; esperas
  `aguardar` de execuções em andamento continuam (a execução termina) — exceto exclusão da
  automação (cascata → execução some).
- Limite: 1.000 esperas `agendar` por automação.

### Pausa de conversa (`pausas_conversa`)

| Campo | Tipo | Obrig. | Observação |
|-------|------|--------|------------|
| conversa_id | FK conversas (cascata), PK | sim | uma pausa por conversa |
| motivo | texto | sim | `humano` \| `anti_loop` \| `manual` |
| ate | data | não | nulo = sem prazo (até "Devolver às automações") |
| automacao_id | texto | não | quem causou (`anti_loop`/`manual`) |
| criada_em | data | sim | — |

Pausa vencida (`ate < agora`) é tratada como inexistente e apagada preguiçosamente. Nova pausa
substitui a anterior se terminar depois (ou se for sem prazo). `humano` por mensagem manual:
`ate = agora + pausa_humana_min`.

### Memória da automação (`memoria_automacoes`)

| Campo | Tipo | Obrig. | Validação |
|-------|------|--------|-----------|
| automacao_id | FK automacoes (cascata) | sim | — |
| escopo | texto | sim | `global` \| `contato:<contato_id>` |
| chave | texto | sim | 1–200 |
| valor | JSON | sim | ≤ 64 KB serializado |
| atualizada_em | data | sim | — |

PK `(automacao_id, escopo, chave)`. Limite 10.000 chaves por automação. Em simulação, escritas
ficam num mapa da execução (lidas por ela mesma) e nunca vão ao banco.

### Configuração (`configuracoes`, tabela existente — chaves novas)

| Chave | Default | Validação |
|-------|---------|-----------|
| `automacoes.anti_loop_mensagens` | 10 | 1–100 |
| `automacoes.anti_loop_janela_min` | 10 | 1–1440 |
| `automacoes.pausa_anti_loop_min` | 60 | 1–10080 |
| `automacoes.pausa_humana_min` | 30 | 1–10080 |
| `automacoes.primeiros_contatos_hora` | 20 | 0–500 (0 = bloqueia) |
| `automacoes.tempo_ia_s` | 60 | 5–300 |
| `automacoes.memoria_ia_mb` | 256 | 64–2048 |
| `automacoes.processos_ia_max` | 4 | 1–16 |
| `automacoes.ociosidade_ia_min` | 5 | 1–60 |
| `automacoes.pausa_geral` | false | bool — "Pausar todas as automações" |
| `ia.modelo_padrao` | `claude-sonnet-5` | começa com `claude-` |

### Segredo (sem tabela)

Valor só no arquivo cifrado do app e na memória do motor. Nome `[A-Z][A-Z0-9_]{0,63}`; valor
1–4.096 caracteres. `ANTHROPIC_API_KEY` é reservado (só o motor usa). A API expõe
`{nome, usado_por: [{automacao_id, nome}]}`.

## Alterações em entidades existentes

### Mensagem (`mensagens`)

| Campo novo | Tipo | Observação |
|------------|------|------------|
| automacao_id | FK automacoes (set null) | não nulo = mensagem automática |
| primeiro_contato | bool | 1 = envio automático que iniciou a conversa (conta no limite por hora) |

Índices `(conversa_id, automacao_id, enviada_em)` e `(conta_id, primeiro_contato, enviada_em)`.
Na API, `Mensagem` ganha `automacao_id|null` (acréscimo compatível).

**Manual** (dispara pausa `humano`): mensagem `de_mim` gravada com `automacao_id IS NULL` e
`disparo_id IS NULL`, em conversa individual — criada por `POST /conversas/{id}/mensagens` (app
ou MCP) ou recebida do celular com `wa_id` inédito.

### Lead

Sem colunas novas. Ganha rota `PATCH /v1/leads/{id}` (nome, campos) usada pelo Kanban, pelas
ações e pelo `ctx.leads`. Campos: chaves normalizadas como na importação (minúsculas, sem
acento, `_`); valores texto ≤ 1.000.

### Destinatário de disparo

Ganha rota `POST /v1/disparos/{id}/destinatarios` (acrescentar leads) usada pela ação
"adicionar a um disparo": permitido em `rascunho`, `agendado`, `enviando`, `fora_da_janela`,
`pausado`; idempotente por `UNIQUE(disparo_id, lead_id)`; `ordem` continua a sequência; valida
variáveis com `valores_padrao` do disparo (faltando → `variaveis_faltando`).

## Resolução de variáveis em automações

Reusa `internal/variaveis` do MVP. Valores disponíveis, em ordem de precedência: variáveis da
execução/sessão (`{email}`), campos do lead, `{nome}` (lead → contato `nome` → `nome_push`),
`{primeiro_nome}` (primeira palavra de `{nome}`), `{telefone}` (E.164). Sem valor e sem
`valores_padrao` → ação falha "Variável {x} sem valor" (FR-019).

## Máquina do executor de fluxo

```text
fato → casar gatilhos (automação ativa, conta, grupo, cadeia) → pausa? → condições
  ├─ não atendidas → execução ok ("condições não atendidas")
  └─ atendidas → para cada ação a partir de passo_atual:
        persistir passo_atual → executar ação (via Portão se envia) → registrar AcaoRegistrada
        aguardar → gravar espera + estado aguardando → (agendador) → retoma no passo seguinte
      → ok | erro (ação falhou com erro fatal) 
```

Falha de uma ação **não fatal** (envio bloqueado, referência quebrada, variável sem valor) é
registrada e o fluxo **continua**; erro interno é fatal (`erro`). Em simulação, `aguardar` é
registrado ("aguardaria 2 h") e o fluxo segue sem esperar.

## Máquina do chatbot

```text
início(gatilho) → [mensagem → …] → nó que espera resposta (menu | pergunta)
resposta do contato → validar
  ├─ válida → variável/ramo → próximo nó …
  └─ inválida → tentativas+1 → < max: "não entendi" + repete ; = max: saída ao_esgotar (padrão humano)
condicao → primeiro ramo verdadeiro | senao ; acao → executa (sem esperar) → próximo
ia → executa automação IA (aoExecutar) → responde ou salva variável → próximo
humano → pausa humano + notifica → sessão humano ; fim → sessão concluida
```

Todo envio do bot passa pelo Portão; envio bloqueado por anti-loop encerra a sessão como
`abortada` ("anti-loop").
