import { definirAutomacao } from '@zapdesk/automacao';
import { ACOES, CATEGORIAS, FUNIL } from './categorias';

// Classifica o contato pelas últimas mensagens e move o lead no funil, com uma etiqueta.
// Não envia nada ao contato.
export default definirAutomacao({
  async aoReceberMensagem(ctx, msg) {
    if (!msg.texto || !ctx.conversa) return;
    const historico = await ctx.conversa.historicoParaIA({ limite: 10 });
    const texto = historico.map((m) => `${m.papel === 'user' ? 'Contato' : 'Loja'}: ${m.texto}`).join('\n');
    const { categoria } = await ctx.ia.classificar(texto, CATEGORIAS, {
      instrucoes: 'Classifique o interesse de compra do contato. Na dúvida, escolha "morno".',
    });
    const acao = ACOES[categoria];
    await ctx.funil.mover(FUNIL, acao.etapa);
    await ctx.etiquetas.adicionar(acao.etiqueta);
    ctx.log.info('classificado como', categoria);
  },
});
