// Integração do runner (processo real, Permission Model) com um motor simulado em TS: projeto no
// formato do modelo "Responder com IA usando histórico" compilado como o motor compila (esbuild,
// prompt.md como texto, sourcemap inline) e IA simulada respondendo `ctx.ia.gerar`.
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { ErroRpc } from '../src/rpc.js';
import { compilarProjeto, erroDe, MotorFalso, pastaTemporaria } from './apoio/motor-falso.js';

const PROJETO = {
  'automacao.json': JSON.stringify({ nome: 'Responder com IA', permissoes: ['ler_conversas', 'enviar', 'ia'] }),
  'prompt.md': `Você atende clientes pelo WhatsApp. Responda em português, curto e cordial.
Nunca invente preços, prazos ou promessas. Se não souber ou o contato pedir uma pessoa,
responda apenas [HUMANO].
`,
  'index.ts': `import { definirAutomacao } from '@zapdesk/automacao';
import prompt from './prompt.md';

export default definirAutomacao({
  async aoReceberMensagem(ctx, msg) {
    if (!msg.texto) return;
    const historico = await ctx.conversa!.historicoParaIA({ limite: 20 });
    const resposta = await ctx.ia.gerar({ sistema: prompt, mensagens: historico });
    if (resposta.texto.includes('[HUMANO]')) {
      await ctx.humano.transferir({ motivo: 'IA pediu humano', mensagem: 'Vou chamar um atendente.' });
      return;
    }
    await ctx.responder(resposta.texto);
    ctx.log.info('respondeu', { tokens: resposta.tokens });
  },
});
`,
};

const CONVERSA = {
  id: 'c1', conta_id: 'k1', tipo: 'individual', nome: 'Ana', telefone: '+5511999990000', lead_id: null,
  contato: { id: 'ct1', nome: 'Ana', nome_push: 'Ana', telefone: '+5511999990000', notas: null, etiquetas: [] },
};

function mensagem(texto: string) {
  return {
    id: 'm-' + texto.length, conversaId: 'c1', deMim: false, automacaoId: null, tipo: 'texto', texto,
    remetenteNome: 'Ana', enviadaEm: '2026-09-27T12:00:03Z', primeira: false, midia: null,
  };
}

let motor: MotorFalso;
let simulacaoIA: (params: Record<string, unknown>) => string;
const enviadas: unknown[] = [];

beforeAll(async () => {
  const pasta = pastaTemporaria();
  const bundle = await compilarProjeto(pasta, PROJETO, '9f2c.mjs');
  motor = new MotorFalso({ permitirLeitura: [bundle] });
  motor
    .tratar('ctx.conversa.historico', () => {
      return [
        { id: '3', conversa_id: 'c1', de_mim: false, automacao_id: null, tipo: 'texto', texto: 'qual o preço?', remetente_nome: 'Ana', enviada_em: '2026-09-27T12:00:03Z' },
        { id: '2', conversa_id: 'c1', de_mim: true, automacao_id: 'aut1', tipo: 'texto', texto: 'Olá! Como posso ajudar?', remetente_nome: null, enviada_em: '2026-09-27T12:00:02Z' },
        { id: '1', conversa_id: 'c1', de_mim: false, automacao_id: null, tipo: 'texto', texto: 'oi', remetente_nome: 'Ana', enviada_em: '2026-09-27T12:00:01Z' },
      ];
    })
    // IA simulada do motor (--ia=falsa): determinística, sem rede
    .tratar('ctx.ia.gerar', (p) => ({ texto: simulacaoIA(p), modelo: 'claude-sonnet-5', motivo_parada: 'end_turn', tokens: { entrada: 0, saida: 0 } }))
    .tratar('ctx.enviar', (p) => {
      enviadas.push(p);
      return { id: null, conversa_id: 'c1', simulada: true, texto: (p.conteudo as { texto: string }).texto };
    })
    .tratar('ctx.humano.transferir', () => ({}));
  const r = await motor.inicializar(bundle, {
    automacao: { id: 'aut1', nome: 'Responder com IA', versao: 7 },
    hash: '9f2c',
    permissoes: ['ler_conversas', 'enviar', 'ia'],
  });
  expect(r).toEqual({ handlers: ['aoReceberMensagem'] });
});

afterAll(async () => {
  await motor.requisitar('encerrar');
  expect(await motor.saida).toEqual({ codigo: 0, sinal: null });
});

describe('runner ↔ motor simulado com o modelo "Responder com IA"', () => {
  it('lê o histórico, chama a IA com o prompt e responde (simulação: nada enviado de verdade)', async () => {
    simulacaoIA = () => '[IA simulada] O valor depende do plano; posso chamar um atendente?';
    motor.chamadas.length = 0;
    const r = await motor.executar({
      execucao_id: 'e1', handler: 'aoReceberMensagem', simulacao: true, prazo_ms: 58_000,
      info: { automacaoId: 'aut1', automacaoNome: 'Responder com IA', gatilho: { tipo: 'mensagem_recebida', dados: {} }, origem: 'teste', iniciadaEm: '2026-09-27T12:00:04Z' },
      conversa: CONVERSA, argumento: mensagem('qual o preço?'),
    });
    expect(r).toEqual({ retorno: null });
    expect(motor.chamadas.map((c) => c.metodo)).toEqual(['ctx.conversa.historico', 'ctx.ia.gerar', 'ctx.enviar']);
    expect(motor.chamadas[0]!.params).toEqual({ execucao_id: 'e1', conversa_id: 'c1', limite: 20 });
    const ia = motor.chamadas[1]!.params;
    expect(ia.sistema).toBe(PROJETO['prompt.md']);
    expect(ia.mensagens).toEqual([
      { papel: 'user', texto: 'oi' },
      { papel: 'assistant', texto: 'Olá! Como posso ajudar?' },
      { papel: 'user', texto: 'qual o preço?' },
    ]);
    expect(motor.chamadas[2]!.params).toEqual({
      execucao_id: 'e1', destino: { conversa_id: 'c1' },
      conteudo: { texto: '[IA simulada] O valor depende do plano; posso chamar um atendente?' },
    });
    const log = await motor.esperarNotificacao((n) => n.metodo === 'log' && n.params.execucao_id === 'e1');
    expect(log.params).toMatchObject({ nivel: 'info', texto: 'respondeu { tokens: { entrada: 0, saida: 0 } }' });
  });

  it('[HUMANO] transfere para atendimento humano sem responder', async () => {
    simulacaoIA = () => '[HUMANO]';
    motor.chamadas.length = 0;
    await motor.executar({ execucao_id: 'e2', handler: 'aoReceberMensagem', conversa: CONVERSA, argumento: mensagem('quero falar com alguém') });
    expect(motor.chamadas.map((c) => c.metodo)).toEqual(['ctx.conversa.historico', 'ctx.ia.gerar', 'ctx.humano.transferir']);
    expect(motor.chamadas[2]!.params).toEqual({ execucao_id: 'e2', motivo: 'IA pediu humano', mensagem: 'Vou chamar um atendente.' });
  });

  it('mensagem sem texto não chama o motor', async () => {
    motor.chamadas.length = 0;
    await motor.executar({ execucao_id: 'e3', handler: 'aoReceberMensagem', conversa: CONVERSA, argumento: { ...mensagem(''), texto: null, tipo: 'imagem' } });
    expect(motor.chamadas).toEqual([]);
  });

  it('erros do motor (permissão, portão, IA) viram erro da execução com o código da SDK', async () => {
    const casos: [string, ErroRpc, string][] = [
      ['ctx.ia.gerar', new ErroRpc(1001, "Permissão 'ia' não declarada em automacao.json", { codigo: 'permissao_negada', permissao: 'ia' }), 'permissao_negada'],
      ['ctx.ia.gerar', new ErroRpc(1005, 'Configure a chave da Anthropic em Ajustes → IA', { codigo: 'ia_nao_configurada' }), 'ia_nao_configurada'],
      ['ctx.enviar', new ErroRpc(1004, 'Conversa em atendimento humano até 14:30.', { codigo: 'bloqueado', motivo: 'pausa' }), 'bloqueado'],
    ];
    simulacaoIA = () => 'ok';
    for (const [i, [metodo, erro, codigoSdk]] of casos.entries()) {
      const original = motor.tratadores.get(metodo)!;
      motor.tratar(metodo, () => {
        throw erro;
      });
      const e = await erroDe(motor.executar({ execucao_id: `x${i}`, handler: 'aoReceberMensagem', conversa: CONVERSA, argumento: mensagem('oi') }));
      motor.tratar(metodo, original);
      expect(e.code).toBe(2002);
      expect(e.data).toMatchObject({ codigo: 'erro_usuario', mensagem: erro.message, codigo_sdk: codigoSdk });
      expect((e.data as { stack: string }).stack).toMatch(/index\.ts:\d+:\d+/);
    }
  });

  it('execuções concorrentes de conversas diferentes no mesmo processo', async () => {
    simulacaoIA = (p) => `eco: ${(p.mensagens as { texto: string }[]).at(-1)!.texto}`;
    enviadas.length = 0;
    await Promise.all(
      ['c1', 'c2', 'c3'].map((id, i) =>
        motor.executar({ execucao_id: `p${i}`, handler: 'aoReceberMensagem', conversa: { ...CONVERSA, id }, argumento: mensagem('oi') }),
      ),
    );
    expect(enviadas).toHaveLength(3);
    expect(new Set(enviadas.map((p) => (p as { execucao_id: string }).execucao_id))).toEqual(new Set(['p0', 'p1', 'p2']));
  });
});
