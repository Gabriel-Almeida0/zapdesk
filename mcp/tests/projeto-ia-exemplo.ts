// Projeto de automação de IA de exemplo usado nos testes do ciclo (simulado e real).

export const MANIFESTO_RESPONDER = JSON.stringify(
  {
    versao_manifesto: 1,
    nome: 'Responder dúvidas',
    entrada: 'index.ts',
    gatilhos: [{ tipo: 'mensagem_recebida' }],
    permissoes: ['ler_conversas', 'enviar', 'ia'],
    segredos: [],
    contas: 'todas',
  },
  null,
  2,
);

export const CODIGO_COM_ERRO = `import { definirAutomacao } from '@zapdesk/automacao';

export default definirAutomacao({
  async aoReceberMensagem(ctx, msg) {
    const resposta = await ctx.ia.gerar({ prompt: msg.texto ?? '' }
    await ctx.responder(resposta.texto);
  },
});
`;

export const CODIGO_OK = `import { definirAutomacao } from '@zapdesk/automacao';
import prompt from './prompt.md';

export default definirAutomacao({
  async aoReceberMensagem(ctx, msg) {
    if (!msg.texto) return;
    const historico = await ctx.conversa!.historicoParaIA({ limite: 10 });
    const resposta = await ctx.ia.gerar({ sistema: prompt, mensagens: historico });
    console.log('resposta gerada', resposta.tokens);
    await ctx.responder(resposta.texto);
  },
});
`;
