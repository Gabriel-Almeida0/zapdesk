# Formatos — gatilhos, condições, ações, fluxo, chatbot e `automacao.json`

JSON com campos em `snake_case` português. Validação no motor (`definicao_invalida` com
`ErroDefinicao.caminho` = caminho JSON do campo). O motor serve o JSON Schema do manifesto em
`GET /v1/automacoes/sdk` (`esquema_manifesto`) e o grava em `.zapdesk/automacao.schema.json`.

**Referências**: em fluxos e chatbots (criados pela UI/MCP) as referências são por id
(`etiqueta_id`, `funil_id`, `etapa_id`, `template_id`, `automacao_id`, `disparo_id`,
`conta_id`). No `automacao.json` também se aceitam nomes (`etiqueta`, `funil`, `etapa`,
`template`) — resolvidos sem diferenciar maiúsculas/acentos na compilação; nome inexistente é
erro de manifesto. Referência quebrada **depois** de gravada vira `aviso` (não impede salvar) e
falha a ação em tempo de execução (FR-021).

## Gatilho

Uma automação dispara se **qualquer** de seus gatilhos casar (e a conta estiver em `contas`, e a
conversa não for grupo — salvo `incluir_grupos`).

```jsonc
{ "tipo": "mensagem_recebida",
  "contem": "preço",                 // opcional; sem diferenciar maiúsculas/acentos
  "regex": "^\\d{5}-?\\d{3}$",       // opcional; RE2 (sintaxe Go), 1–500 chars; flag (?i) permitida
  "primeira_mensagem": false,        // opcional; só a 1ª mensagem recebida do contato nesta conta
  "tipo_conversa": "individual" }    // "individual" (padrão) | "grupo" | "qualquer" — grupo exige incluir_grupos
{ "tipo": "palavra_chave", "palavras": ["orçamento", "preço"], "modo": "palavra" }  // "palavra" (padrão) | "mensagem_inteira"
{ "tipo": "lead_importado", "origens": ["csv", "colado", "contatos", "mcp"] }      // origens opcional (padrão todas)
{ "tipo": "etiqueta", "evento": "adicionada", "etiqueta_id": "01J..." }           // "adicionada" | "removida"
{ "tipo": "entrou_etapa", "funil_id": "01J...", "etapa_id": "01J..." }
{ "tipo": "disparo_respondeu", "disparo_id": null }                                // null = qualquer disparo
{ "tipo": "sem_resposta", "apos_s": 7200, "origem_mensagem": "qualquer" }          // 60–2592000; "qualquer"|"disparo"|"automacao"|"manual"
{ "tipo": "agendamento", "cron": "0 9 * * 1-5" }                                   // 5 campos, fuso local
{ "tipo": "agendamento", "intervalo_s": 3600 }                                     // ≥ 60; exatamente um de cron/intervalo_s
{ "tipo": "manual" }
```

Normalização de texto para `contem`/`palavra_chave`/menus: minúsculas, sem acentos, espaços
colapsados, pontuação nas bordas removida. `palavra`: casa se alguma palavra da lista aparece como
palavra isolada; `mensagem_inteira`: a mensagem normalizada é igual à palavra.

Dados do fato entregues à execução (`gatilho.dados`): `mensagem_id` (mensagem/palavra),
`lead_ids` (lead importado — **uma execução por lead**), `etiqueta_id` + `contato_id`, `funil_id`
+ `etapa_id` + `lead_id`, `disparo_id` + `destinatario_id`, `mensagem_id` de referência
(sem resposta), `ocorrencia` (agendamento), `pedido_por` (manual).

## Condições

```jsonc
{ "modo": "todas",                 // "todas" (E) | "alguma" (OU)
  "regras": [
    { "tipo": "etiqueta", "operador": "tem", "etiqueta_id": "01J..." },             // tem | nao_tem
    { "tipo": "etapa", "operador": "esta", "funil_id": "01J...", "etapa_id": "01J..." },  // esta | nao_esta; etapa_id null = "em qualquer etapa do funil"
    { "tipo": "campo_lead", "campo": "empresa", "operador": "igual", "valor": "ACME" },
        // igual | diferente | contem | existe | nao_existe | maior | menor (maior/menor: numérico; texto não numérico = falso)
    { "tipo": "horario", "inicio": "09:00", "fim": "18:00", "dias": [1,2,3,4,5] },   // 1=segunda … 7=domingo; fim < inicio = atravessa meia-noite
    { "tipo": "texto", "operador": "contem", "valor": "boleto" },                    // contem | igual | regex — texto da mensagem do gatilho (sem mensagem = falso)
    { "tipo": "conta", "conta_ids": ["01J..."] },
    { "tipo": "variavel", "variavel": "email", "operador": "existe", "valor": null }  // variáveis da execução/sessão; mesmos operadores de campo_lead + regex
  ] }
```

`condicoes: null` ou `regras: []` = sempre verdadeiro. Máx. 20 regras.

## Ação

Todas têm `id` opcional (gerado se ausente; único na definição). Textos aceitam variáveis
`{nome}` (data-model › Resolução de variáveis).

```jsonc
{ "tipo": "enviar_texto", "texto": "Oi {primeiro_nome}!", "valores_padrao": { "primeiro_nome": "tudo bem" } }  // 1–4096
{ "tipo": "enviar_template", "template_id": "01J...", "valores_padrao": {} }        // texto + anexo do template
{ "tipo": "aguardar", "duracao_s": 7200 }                                           // 60–2592000 (não permitido em chatbot)
{ "tipo": "adicionar_etiqueta", "etiqueta_id": "01J..." }
{ "tipo": "remover_etiqueta", "etiqueta_id": "01J..." }
{ "tipo": "mover_etapa", "funil_id": "01J...", "etapa_id": "01J..." }
{ "tipo": "remover_do_funil", "funil_id": "01J..." }
{ "tipo": "atualizar_nota", "texto": "Interesse: {interesse}", "modo": "acrescentar" }   // substituir | acrescentar (nova linha); nota ≤ 10.000
{ "tipo": "atualizar_campo_lead", "campo": "interesse", "valor": "{interesse}" }     // campo "nome" altera o nome do lead; valor "" remove
{ "tipo": "iniciar_chatbot", "automacao_id": "01J..." }                             // não permitido dentro de chatbot; ignora se já há sessão ativa
{ "tipo": "executar_ia", "automacao_id": "01J...", "entrada": { "pergunta": "{ultima_mensagem}" }, "salvar_em": "resposta" }
      // chama aoExecutar; strings da entrada passam por variáveis; retorno salvo em variável (string → texto; objeto → JSON)
{ "tipo": "adicionar_a_disparo", "disparo_id": "01J..." }
{ "tipo": "pausar_automacoes", "duracao_min": 60 }                                  // null = sem prazo; motivo "manual"
{ "tipo": "notificar", "titulo": "Lead quente", "texto": "{nome} respondeu" }       // título 1–60, texto 1–240
```

Variáveis especiais sempre disponíveis: `{ultima_mensagem}` (texto da mensagem do gatilho ou a
última recebida na conversa), `{conta}` (nome da conta).

Resultado de cada ação: `ok`, `falhou` (motivo; o fluxo continua) ou `bloqueada` (portão:
pausa/anti-loop/grupo/primeiro contato; o fluxo continua; anti-loop encerra sessões de bot).
Máx. 50 ações por fluxo.

## DefinicaoFluxo

```jsonc
{ "versao": 1,
  "condicoes": { "modo": "todas", "regras": [] },   // ou null
  "acoes": [ /* Acao[] , 1–50 */ ] }
```

## DefinicaoChatbot

```jsonc
{ "versao": 1,
  "inicio": "n1",
  "nao_entendi": "Não entendi. Responda com uma das opções.",   // 1–500
  "max_tentativas": 3,          // 1–10
  "inatividade_min": 30,        // 1–1440
  "nos": [
    { "id": "n1", "tipo": "inicio", "proximo": "n2", "posicao": { "x": 0, "y": 0 } },
    { "id": "n2", "tipo": "mensagem", "texto": "Olá, {nome}!", "template_id": null, "proximo": "n3" },
    { "id": "n3", "tipo": "menu", "texto": "Como posso ajudar?",
      "opcoes": [ { "rotulo": "Preços", "valores": ["preco", "valores"], "proximo": "n4" },
                  { "rotulo": "Falar com vendedor", "valores": [], "proximo": "n8" } ],   // 1–10 opções
      "mostrar_numeros": true,      // envia "1 - Preços\n2 - Falar com vendedor" após o texto
      "ao_esgotar": null },         // null = transferir para humano
    { "id": "n4", "tipo": "pergunta", "texto": "Qual seu e-mail?", "variavel": "email",   // [a-z_][a-z0-9_]{0,39}
      "validacao": { "tipo": "email", "padrao": null, "mensagem_erro": "E-mail inválido. Tente de novo." },
          // tipo: nenhuma | email | numero | telefone (normaliza E.164) | regex (padrao obrigatório)
      "proximo": "n5", "ao_esgotar": null },
    { "id": "n5", "tipo": "condicao",
      "ramos": [ { "condicoes": { "modo": "todas", "regras": [ { "tipo": "variavel", "variavel": "email", "operador": "contem", "valor": "@empresa.com" } ] }, "proximo": "n6" } ],
      "senao": "n7" },
    { "id": "n6", "tipo": "acao", "acao": { "tipo": "atualizar_campo_lead", "campo": "email", "valor": "{email}" }, "proximo": "n7" },
    { "id": "n7", "tipo": "ia", "automacao_id": "01J...", "entrada": { "email": "{email}" },
      "modo": "responder",          // responder (retorno string é enviado) | variavel
      "variavel": null, "proximo": "n9", "em_erro": "n8" },   // em_erro opcional (padrão: humano)
    { "id": "n8", "tipo": "humano", "mensagem": "Vou chamar um atendente, só um instante." },
    { "id": "n9", "tipo": "fim", "mensagem": "Obrigado!" }
  ] }
```

Validação (`definicao_invalida`): exatamente um `inicio`; ids únicos `[A-Za-z0-9_-]{1,40}`; todo
`proximo`/`ao_esgotar`/`senao`/`em_erro` aponta para nó existente; todo nó alcançável a partir do
início; `mensagem`/`acao`/`ia`/`condicao` têm saída; `humano` e `fim` não têm; variável usada em
texto/condição é capturada por algum `pergunta` anterior no caminho (ou é variável padrão); ações
`aguardar` e `iniciar_chatbot` proibidas; 2–200 nós. `posicao` é só do canvas (ignorada pelo
executor).

Correspondência de resposta no menu: número da opção (`"2"`, `"2."`, `"2)"`), rótulo normalizado
ou um dos `valores` normalizados. Mídia sem texto = resposta inválida.

## `automacao.json` (manifesto da automação de IA)

```jsonc
{
  "$schema": "./.zapdesk/automacao.schema.json",
  "versao_manifesto": 1,
  "nome": "Responder com IA",            // 1–80 (espelhado em Automacao.nome)
  "descricao": "Responde dúvidas usando o histórico.",
  "entrada": "index.ts",                 // arquivo de entrada (padrão index.ts)
  "gatilhos": [ { "tipo": "mensagem_recebida" } ],   // mesmos Gatilho acima; nomes aceitos no lugar de ids
  "permissoes": ["ler_conversas", "enviar", "ia"],
      // enviar | ler_conversas | etiquetas | funil | leads | ia | rede | agendar
  "segredos": [],                        // nomes de segredos que o código pode ler (≠ ANTHROPIC_API_KEY)
  "contas": "todas",                     // "todas" | ["<conta_id>", …]
  "incluir_grupos": false,
  "prioridade": 100,
  "conta_envio": null,                   // conta_id para eventos sem conversa
  "limites": { "tempo_s": 60, "memoria_mb": 256, "anti_loop": null },
  "ia": { "modelo": null }               // null = modelo padrão de Ajustes
}
```

Mapa gatilho → handler exigido na compilação:

| Gatilho | Handler |
|---------|---------|
| `mensagem_recebida`, `palavra_chave` | `aoReceberMensagem(ctx, msg)` |
| `agendamento` (e `ctx.agendar`) | `aoAgendar(ctx, agendamento)` |
| `manual`, ação `executar_ia`, nó `ia`, MCP `executar_automacao` | `aoExecutar(ctx, entrada)` |
| `lead_importado`, `etiqueta`, `entrou_etapa`, `disparo_respondeu`, `sem_resposta` | `aoEvento(ctx, evento)` |

`aoExecutar` é sempre opcional na compilação, mas a chamada por fluxo/bot/MCP falha em execução
("Handler aoExecutar não exportado") se ausente. Handler exigido e ausente → erro de compilação
`tipo:"manifesto"`.

## Modelos de projeto (arquivos criados)

| Modelo | Arquivos | Gatilho / permissões |
|--------|----------|----------------------|
| `responder_historico` | `automacao.json`, `index.ts`, `prompt.md` | `mensagem_recebida`; `ler_conversas`, `enviar`, `ia` |
| `classificar_funil` | `automacao.json`, `index.ts`, `categorias.ts` | `mensagem_recebida`; `ler_conversas`, `funil`, `etiquetas`, `ia` |
| `extrair_dados` | `automacao.json`, `index.ts`, `esquema.ts` | `mensagem_recebida` + `manual`; `ler_conversas`, `leads`, `ia` |
| `em_branco` | `automacao.json`, `index.ts` | `manual`; nenhuma |

Prompts dos modelos com IA: responder em português, curto, **nunca inventar preços, prazos ou
promessas**; se não souber ou o contato pedir pessoa, chamar `ctx.humano.transferir()`.
