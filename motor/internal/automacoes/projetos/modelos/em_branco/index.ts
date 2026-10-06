import { definirAutomacao } from '@zapdesk/automacao';

// Automação em branco: roda quando executada manualmente (app ou MCP) ou por um fluxo/chatbot.
// Declare gatilhos e permissões em automacao.json.
export default definirAutomacao({
  async aoExecutar(ctx, entrada) {
    ctx.log.info('executada com', entrada);
    return { ok: true };
  },
});
