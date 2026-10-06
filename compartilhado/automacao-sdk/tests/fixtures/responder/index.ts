// Exemplo "Responder com IA usando histórico" (contracts/sdk-automacao.md), compilado contra o
// dist/index.d.ts pelo teste de tipos.
import { definirAutomacao, ErroBloqueado } from '@zapdesk/automacao';
import prompt from './prompt.md';

export default definirAutomacao({
  async aoReceberMensagem(ctx, msg) {
    if (!msg.texto) return;
    const historico = await ctx.conversa!.historicoParaIA({ limite: 20 });
    const resposta = await ctx.ia.gerar({ sistema: prompt, mensagens: historico });
    if (resposta.texto.includes('[HUMANO]')) {
      await ctx.humano.transferir({ mensagem: 'Vou chamar um atendente.' });
      return;
    }
    try {
      await ctx.responder(resposta.texto);
    } catch (erro) {
      if (erro instanceof ErroBloqueado) ctx.log.aviso('bloqueado', erro.motivo);
      else throw erro;
    }
    const { categoria } = await ctx.ia.classificar(msg.texto, ['quente', 'frio'] as const);
    const c: 'quente' | 'frio' = categoria;
    const r = await ctx.http.fetch('https://exemplo.com', { signal: ctx.sinal });
    ctx.log.info('respondeu', { tokens: resposta.tokens, c, status: r.status });
  },
  aoExecutar(_ctx, entrada) {
    return { eco: entrada };
  },
});
