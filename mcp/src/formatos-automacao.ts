// Resumo embutido de specs/002-automacoes/contracts/formatos.md, devolvido por
// `ver_formatos_automacao`. É texto para a IA montar gatilhos, condições, ações, fluxos, chatbots
// e o `automacao.json` sem errar o formato. Ao mudar formatos.md, atualize aqui.

export const TEXTO_FORMATOS_AUTOMACAO = `# Formatos das automações do ZapDesk

JSON com campos snake_case em português. Referências são por id (etiqueta_id, funil_id,
etapa_id, template_id, automacao_id, disparo_id, conta_id) — descubra os ids com listar_etiquetas,
listar_funis, listar_templates, listar_automacoes, listar_disparos e listar_contas. Erros de
formato voltam como "caminho: mensagem" (ex.: definicao.acoes[2].etapa_id). Referência quebrada
depois de gravada vira aviso (não impede salvar) e a ação falha em tempo de execução.
Teste antes de ativar: validar_automacao, testar_automacao (fluxo/IA) ou simular_chatbot.

## Gatilhos (a automação dispara se QUALQUER gatilho casar)

{"tipo":"mensagem_recebida","contem":"preço","regex":null,"primeira_mensagem":false,"tipo_conversa":"individual"}
   contem: sem diferenciar maiúsculas/acentos; regex RE2 (sintaxe Go, até 500 chars, (?i) permitido);
   primeira_mensagem: só a 1ª mensagem do contato nesta conta; tipo_conversa "individual" (padrão) |
   "grupo" | "qualquer" (grupo exige incluir_grupos: true na automação). Todos os campos opcionais.
{"tipo":"palavra_chave","palavras":["orçamento","preço"],"modo":"palavra"}   // "palavra" (padrão) | "mensagem_inteira"
{"tipo":"lead_importado","origens":["csv","colado","contatos","mcp"]}      // origens opcional (padrão: todas)
{"tipo":"etiqueta","evento":"adicionada","etiqueta_id":"..."}               // "adicionada" | "removida"
{"tipo":"entrou_etapa","funil_id":"...","etapa_id":"..."}
{"tipo":"disparo_respondeu","disparo_id":null}                              // null = qualquer disparo
{"tipo":"sem_resposta","apos_s":7200,"origem_mensagem":"qualquer"}         // 60–2592000; qualquer|disparo|automacao|manual
{"tipo":"agendamento","cron":"0 9 * * 1-5"}                                 // cron de 5 campos, fuso local
{"tipo":"agendamento","intervalo_s":3600}                                   // ≥ 60; use cron OU intervalo_s
{"tipo":"manual"}                                                           // só por executar_automacao / botão no app

Texto é normalizado (minúsculas, sem acentos, espaços colapsados, pontuação das bordas removida).
Mensagens enviadas por você nunca disparam gatilhos. Gatilho lead_importado gera uma execução por lead.

## Condições (opcionais; null ou regras [] = sempre verdadeiro; máx. 20 regras)

{"modo":"todas","regras":[ ... ]}   // "todas" (E) | "alguma" (OU)
  {"tipo":"etiqueta","operador":"tem","etiqueta_id":"..."}                   // tem | nao_tem
  {"tipo":"etapa","operador":"esta","funil_id":"...","etapa_id":"..."}       // esta | nao_esta; etapa_id null = qualquer etapa do funil
  {"tipo":"campo_lead","campo":"empresa","operador":"igual","valor":"ACME"}
      // igual | diferente | contem | existe | nao_existe | maior | menor (maior/menor são numéricos)
  {"tipo":"horario","inicio":"09:00","fim":"18:00","dias":[1,2,3,4,5]}       // 1=segunda … 7=domingo; fim<inicio atravessa a meia-noite
  {"tipo":"texto","operador":"contem","valor":"boleto"}                      // contem | igual | regex (texto da mensagem do gatilho)
  {"tipo":"conta","conta_ids":["..."]}
  {"tipo":"variavel","variavel":"email","operador":"existe","valor":null}    // variáveis da execução/sessão; operadores de campo_lead + regex

## Ações (id opcional, único; textos aceitam variáveis {nome}, {primeiro_nome}, {telefone}, {ultima_mensagem}, {conta}, campos do lead e variáveis capturadas)

{"tipo":"enviar_texto","texto":"Oi {primeiro_nome}!","valores_padrao":{"primeiro_nome":"tudo bem"}}   // 1–4096 chars
{"tipo":"enviar_template","template_id":"...","valores_padrao":{}}
{"tipo":"aguardar","duracao_s":7200}                   // 60–2592000; só em fluxo (proibido em chatbot)
{"tipo":"adicionar_etiqueta","etiqueta_id":"..."}
{"tipo":"remover_etiqueta","etiqueta_id":"..."}
{"tipo":"mover_etapa","funil_id":"...","etapa_id":"..."}
{"tipo":"remover_do_funil","funil_id":"..."}
{"tipo":"atualizar_nota","texto":"Interesse: {interesse}","modo":"acrescentar"}    // substituir | acrescentar
{"tipo":"atualizar_campo_lead","campo":"interesse","valor":"{interesse}"}         // campo "nome" muda o nome; valor "" remove
{"tipo":"iniciar_chatbot","automacao_id":"..."}        // proibido dentro de chatbot
{"tipo":"executar_ia","automacao_id":"...","entrada":{"pergunta":"{ultima_mensagem}"},"salvar_em":"resposta"}
      // chama aoExecutar da automação de IA; o retorno vai para a variável salvar_em
{"tipo":"adicionar_a_disparo","disparo_id":"..."}
{"tipo":"pausar_automacoes","duracao_min":60}          // null = sem prazo
{"tipo":"notificar","titulo":"Lead quente","texto":"{nome} respondeu"}            // título 1–60, texto 1–240

Resultado de cada ação: ok, falhou (o fluxo continua) ou bloqueada (portão de segurança: conversa
pausada, pausa geral, grupo, anti-loop de 10 mensagens automáticas a cada 10 min, limite de
primeiros contatos). Todo envio passa por esse portão.

## Fluxo (tipo "fluxo") — campo definicao

{"versao":1,"condicoes":null,"acoes":[ /* 1–50 ações, executadas em ordem */ ]}

Exemplo completo para criar_automacao:
{"tipo":"fluxo","nome":"Respondeu → quente","gatilhos":[{"tipo":"disparo_respondeu"}],
 "definicao":{"versao":1,"condicoes":null,"acoes":[
   {"tipo":"mover_etapa","funil_id":"F1","etapa_id":"E_QUENTE"},
   {"tipo":"notificar","titulo":"Lead quente","texto":"{nome} respondeu ao disparo"}]}}

## Chatbot (tipo "chatbot") — campo definicao (grafo de nós)

{"versao":1,"inicio":"n1","nao_entendi":"Não entendi. Responda com uma das opções.",
 "max_tentativas":3,"inatividade_min":30,"nos":[
  {"id":"n1","tipo":"inicio","proximo":"n2"},
  {"id":"n2","tipo":"mensagem","texto":"Olá, {nome}!","proximo":"n3"},
  {"id":"n3","tipo":"menu","texto":"Como posso ajudar?","mostrar_numeros":true,"ao_esgotar":null,
     "opcoes":[{"rotulo":"Preços","valores":["preco","valores"],"proximo":"n4"},
               {"rotulo":"Falar com vendedor","valores":[],"proximo":"n8"}]},
  {"id":"n4","tipo":"pergunta","texto":"Qual seu e-mail?","variavel":"email",
     "validacao":{"tipo":"email","mensagem_erro":"E-mail inválido. Tente de novo."},"proximo":"n5"},
  {"id":"n5","tipo":"condicao","ramos":[{"condicoes":{"modo":"todas","regras":[
     {"tipo":"variavel","variavel":"email","operador":"contem","valor":"@empresa.com"}]},"proximo":"n6"}],"senao":"n7"},
  {"id":"n6","tipo":"acao","acao":{"tipo":"atualizar_campo_lead","campo":"email","valor":"{email}"},"proximo":"n7"},
  {"id":"n7","tipo":"ia","automacao_id":"<id de automação de IA>","entrada":{"email":"{email}"},
     "modo":"responder","proximo":"n9","em_erro":"n8"},
  {"id":"n8","tipo":"humano","mensagem":"Vou chamar um atendente."},
  {"id":"n9","tipo":"fim","mensagem":"Obrigado!"}]}

Regras: exatamente um nó inicio; 2–200 nós; ids [A-Za-z0-9_-]{1,40} únicos; todo proximo/
ao_esgotar/senao/em_erro aponta para nó existente; todo nó alcançável a partir do início;
mensagem/menu/pergunta/condicao/acao/ia têm saída, humano e fim não; menu tem 1–10 opções
(o contato responde com o número, o rótulo ou um dos valores); pergunta.variavel [a-z_][a-z0-9_]{0,39};
validacao.tipo nenhuma|email|numero|telefone|regex (regex exige "padrao"); variável usada precisa
ser capturada antes por uma pergunta; ações aguardar e iniciar_chatbot proibidas; nó ia modo
"responder" (envia o retorno string) ou "variavel" (salva em "variavel"). ao_esgotar null = humano.
Enquanto um chatbot conversa com alguém, as outras automações de mensagem não disparam naquela conversa.

## Automação de IA (tipo "ia") — projeto TypeScript

Crie com criar_automacao_ia (modelos: responder_historico, classificar_funil, extrair_dados,
em_branco). O projeto tem automacao.json (manifesto) + index.ts (+ arquivos .ts/.md/.txt/.json).
Gatilhos e permissões ficam no automacao.json (editar_automacao não altera automação de IA).

automacao.json:
{"$schema":"./.zapdesk/automacao.schema.json","versao_manifesto":1,
 "nome":"Responder com IA","descricao":"Responde dúvidas usando o histórico.","entrada":"index.ts",
 "gatilhos":[{"tipo":"mensagem_recebida"}],
 "permissoes":["ler_conversas","enviar","ia"],     // enviar | ler_conversas | etiquetas | funil | leads | ia | rede | agendar
 "segredos":[],                                     // nomes de segredos que o código lê (definidos pelo usuário no app)
 "contas":"todas","incluir_grupos":false,"prioridade":100,"conta_envio":null,
 "limites":{"tempo_s":60,"memoria_mb":256,"anti_loop":null},
 "ia":{"modelo":null}}                              // null = modelo padrão de Ajustes
No manifesto, gatilhos de etiqueta/etapa aceitam nomes: {"tipo":"etiqueta","evento":"adicionada","etiqueta":"Quente"},
{"tipo":"entrou_etapa","funil":"Vendas","etapa":"Proposta"}.

Gatilho → handler exigido (ausente = erro de compilação tipo "manifesto"):
  mensagem_recebida, palavra_chave → aoReceberMensagem(ctx, msg)
  agendamento (e ctx.agendar)      → aoAgendar(ctx, agendamento)
  manual, executar_ia, nó ia, executar_automacao → aoExecutar(ctx, entrada)
  lead_importado, etiqueta, entrou_etapa, disparo_respondeu, sem_resposta → aoEvento(ctx, evento)

API do ctx: resumo na descrição de escrever_arquivo_automacao; .d.ts completo em ver_tipos_sdk.
`;
