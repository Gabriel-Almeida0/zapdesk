import { definirAutomacao } from '@zapdesk/automacao';
import prompt from './prompt.md';

// Responde dúvidas do contato usando o histórico da conversa. Quando a IA não sabe responder ou o
// contato pede uma pessoa, a resposta traz [HUMANO] e a conversa vai para atendimento humano.
export default definirAutomacao({
  async aoReceberMensagem(ctx, msg) {
    if (!msg.texto || !ctx.conversa) return;
    const historico = await ctx.conversa.historicoParaIA({ limite: 20 });
    const resposta = await ctx.ia.gerar({ sistema: prompt, mensagens: historico });
    if (resposta.texto.includes('[HUMANO]')) {
      await ctx.humano.transferir({ mensagem: 'Vou chamar um atendente, só um instante.' });
      return;
    }
    await ctx.responder(resposta.texto.trim());
    ctx.log.info('respondeu', { tokens: resposta.tokens });
  },
});
