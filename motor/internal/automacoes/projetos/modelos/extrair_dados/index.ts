import { definirAutomacao, type Contexto } from '@zapdesk/automacao';
import { ESQUEMA, type Dados } from './esquema';

const INSTRUCOES = 'Extraia só o que o contato disse explicitamente. Nunca invente valores; use null.';

async function extrairEGravar(ctx: Contexto): Promise<Dados | null> {
  if (!ctx.conversa) return null;
  const historico = await ctx.conversa.historicoParaIA({ limite: 20 });
  const texto = historico.filter((m) => m.papel === 'user').map((m) => m.texto).join('\n');
  if (!texto) return null;
  const { dados } = await ctx.ia.extrair<Dados>(texto, ESQUEMA, { instrucoes: INSTRUCOES });
  const campos: Record<string, string> = {};
  if (dados.email) campos.email = dados.email;
  if (dados.cidade) campos.cidade = dados.cidade;
  if (dados.interesse) campos.interesse = dados.interesse;
  if (Object.keys(campos).length > 0 || dados.nome) {
    await ctx.leads.atualizar(dados.nome ? { nome: dados.nome, campos } : { campos });
  }
  ctx.log.info('dados extraídos', dados);
  return dados;
}

// Grava no lead os dados que o contato informou na conversa.
export default definirAutomacao({
  async aoReceberMensagem(ctx) {
    await extrairEGravar(ctx);
  },
  async aoExecutar(ctx) {
    const dados = await extrairEGravar(ctx);
    return dados ? { ...dados } : null;
  },
});
