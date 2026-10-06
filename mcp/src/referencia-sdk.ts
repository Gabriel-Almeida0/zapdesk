// Resumo da API do `ctx` da SDK `@zapdesk/automacao` (specs/002-automacoes/contracts/sdk-automacao.md),
// embutido na descrição de `escrever_arquivo_automacao` e devolvido junto com o .d.ts em
// `ver_tipos_sdk`. A fonte da verdade é o .d.ts servido pelo motor; ao mudar a SDK, atualize aqui.

export const RESUMO_SDK = `Resumo da SDK (import { definirAutomacao } from '@zapdesk/automacao'):
export default definirAutomacao({
  async aoReceberMensagem(ctx, msg) {},   // gatilhos mensagem_recebida/palavra_chave; msg: {id, texto|null, tipo, primeira, midia|null, remetenteNome, enviadaEm}
  async aoAgendar(ctx, ag) {},            // gatilho agendamento e ctx.agendar; ag: {origem, id|null, previstoPara, dados}
  async aoExecutar(ctx, entrada) { return {...} }, // manual/executar_automacao/fluxo executar_ia/nó ia (string retornada a nó ia "responder" é enviada)
  async aoEvento(ctx, ev) {},             // lead_importado | etiqueta | entrou_etapa | disparo_respondeu | sem_resposta
});
ctx (cada chamada exige a permissão entre [] declarada em automacao.json › permissoes):
- ctx.execucao {id, automacaoId, automacaoNome, gatilho:{tipo,dados}, origem, simulacao, prazoMs}; ctx.sinal (AbortSignal)
- ctx.conversa (null sem conversa): {id, contaId, tipo, nome, telefone, contato, leadId,
  historico({limite?,antes?}) [ler_conversas], historicoParaIA({limite?}) → MensagemIA[] [ler_conversas]}
- ctx.responder(texto, {citar?}) [enviar]; ctx.enviar({conversaId}|{telefone,contaId?}, {texto}|{template,variaveis?}|{arquivoId,legenda?}) [enviar];
  ctx.reagir(mensagemId, emoji) [enviar]. Envios passam pelo portão (pausas, anti-loop…) → ErroBloqueado.
- ctx.ia.gerar({prompt? | mensagens?: {papel:'user'|'assistant', texto}[], sistema?, modelo?, maxTokens?}) → {texto, tokens} [ia];
  ctx.ia.classificar(texto, ['a','b'] | {a:'descrição'}, {instrucoes?}) → {categoria} [ia];
  ctx.ia.extrair(texto, jsonSchema, {instrucoes?}) → {dados} [ia]. Sem temperatura.
- ctx.etiquetas.listar/doContato(alvo?)/adicionar(nomeOuId, alvo?)/remover(nomeOuId, alvo?) [etiquetas]
- ctx.funil.listar()/posicao(funil, alvo?)/mover(funil, etapa, alvo?)/remover(funil, alvo?) — por nome ou id [funil]
- ctx.leads.atual()/obter(id)/buscarPorTelefone(tel)/atualizar({nome?, campos?}, alvo?) [leads]
  (alvo = {contatoId}|{leadId}|{telefone}|{conversaId}; padrão: contato/lead da conversa atual)
- ctx.memoria.obter/definir/remover/listar(chave, valor?, {escopo:'global'|'contato'}) (sem permissão; JSON ≤ 64 KB)
- ctx.http.fetch(url, init) [rede] (o fetch global não existe); ctx.segredos.obter(nome)/tem(nome) (só nomes declarados em "segredos")
- ctx.humano.transferir({motivo?, mensagem?, duracaoMin?}) [enviar]; ctx.agendar({daquiSegundos|em, dados?}) [agendar];
  ctx.cancelarAgendamento(id); ctx.notificar(titulo, texto); ctx.log.debug/info/aviso/erro(...) (console.* também vai para o log)
Regras: só ESM/TypeScript; sem node:fs, child_process, net etc. ("Módulo não permitido"); imports relativos
de .ts/.json e .md/.txt (importados como texto: import prompt from './prompt.md'). Em simulação
(testar_automacao) escritas não acontecem e viram "ações que seriam feitas". Nunca logue segredos.
Tipos completos: ver_tipos_sdk.`;
