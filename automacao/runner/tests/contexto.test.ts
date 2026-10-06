// Contexto por execução (unitário, sem processo): conversões, erros 1001–1010, fim/cancelamento,
// segredos, http, historicoParaIA e agendar.
import {
  ErroAutomacao,
  ErroBloqueado,
  ErroExecucaoEncerrada,
  ErroIA,
  ErroNaoEncontrado,
  ErroPermissao,
  ErroSegredo,
  ErroValidacao,
  type Mensagem,
} from '@zapdesk/automacao';
import { describe, expect, it, vi } from 'vitest';
import { criarContexto, erroDoMotor, type OpcoesContexto } from '../src/contexto.js';
import { historicoParaIA } from '../src/historico-ia.js';
import type { ParamsExecutar } from '../src/protocolo.js';
import { ErroRpc } from '../src/rpc.js';

const CONVERSA = {
  id: 'c1',
  conta_id: 'k1',
  tipo: 'individual' as const,
  nome: 'Ana',
  telefone: '+5511999990000',
  lead_id: 'l1',
  contato: { id: 'ct1', nome: 'Ana', nome_push: 'Aninha', telefone: '+5511999990000', notas: null, etiquetas: [] },
};

function montar(extra: Partial<ParamsExecutar> = {}, op: Partial<OpcoesContexto> = {}) {
  const chamadas: { metodo: string; params: Record<string, unknown> }[] = [];
  const respostas = new Map<string, (p: Record<string, unknown>) => unknown>();
  const notificacoes: { metodo: string; params: unknown }[] = [];
  let agora = 1_000_000;
  const controle = criarContexto({
    rpc: {
      requisitar: async (metodo, params) => {
        chamadas.push({ metodo, params: params as Record<string, unknown> });
        const r = respostas.get(metodo);
        return r ? r(params as Record<string, unknown>) : {};
      },
    },
    execucao: {
      execucao_id: 'e1',
      handler: 'aoReceberMensagem',
      simulacao: false,
      prazo_ms: 60_000,
      info: { automacaoId: 'a1', automacaoNome: 'Bot', gatilho: { tipo: 'mensagem_recebida', dados: { x_y: 1 } }, origem: 'gatilho', iniciadaEm: '2026-09-27T10:00:00Z' },
      conversa: CONVERSA,
      argumento: { id: 'm9', conversa_id: 'c1', de_mim: false, texto: 'oi', primeira: true, midia: null },
      ...extra,
    },
    automacao: { id: 'a1', nome: 'Bot' },
    permissoes: ['enviar'],
    segredos: { OPENAI_KEY: 'sk-1' },
    fetchInterno: vi.fn() as unknown as typeof fetch,
    notificar: ((metodo: string, params: unknown) => notificacoes.push({ metodo, params })) as never,
    agora: () => agora,
    ...op,
  });
  return { controle, ctx: controle.ctx, chamadas, respostas, notificacoes, avancar: (ms: number) => (agora += ms) };
}

describe('montagem do ctx', () => {
  it('info, conversa e argumento no formato da SDK (camelCase); prazoMs decresce', () => {
    const { ctx, controle, avancar } = montar();
    expect(ctx.execucao).toMatchObject({
      id: 'e1', automacaoId: 'a1', automacaoNome: 'Bot', origem: 'gatilho', simulacao: false,
      gatilho: { tipo: 'mensagem_recebida', dados: { x_y: 1 } }, iniciadaEm: '2026-09-27T10:00:00Z',
    });
    expect(ctx.execucao.prazoMs).toBe(60_000);
    avancar(1500);
    expect(ctx.execucao.prazoMs).toBe(58_500);
    expect(ctx.conversa).toMatchObject({
      id: 'c1', contaId: 'k1', leadId: 'l1', tipo: 'individual',
      contato: { id: 'ct1', nomePush: 'Aninha', etiquetas: [] },
    });
    expect(controle.argumento).toEqual({ id: 'm9', conversaId: 'c1', deMim: false, texto: 'oi', primeira: true, midia: null });
  });

  it('aceita info em snake_case e não converte a entrada de aoExecutar', () => {
    const { ctx, controle } = montar({
      handler: 'aoExecutar',
      info: { automacao_id: 'a2', automacao_nome: 'Outro', iniciada_em: 'x' },
      argumento: { chave_livre: { outra_chave: 1 } },
      conversa: null,
    });
    expect(ctx.execucao.automacaoId).toBe('a2');
    expect(ctx.execucao.automacaoNome).toBe('Outro');
    expect(controle.argumento).toEqual({ chave_livre: { outra_chave: 1 } });
    expect(ctx.conversa).toBeNull();
  });
});

describe('proxies ctx.* → JSON-RPC', () => {
  it('responder/enviar convertem destino e conteúdo; citar usa a mensagem do gatilho', async () => {
    const { ctx, chamadas, respostas } = montar();
    respostas.set('ctx.enviar', () => ({ id: null, conversa_id: 'c1', simulada: true, texto: 'olá' }));
    expect(await ctx.responder('olá', { citar: true })).toEqual({ id: null, conversaId: 'c1', simulada: true, texto: 'olá' });
    await ctx.enviar({ telefone: '+55119', contaId: 'k2' }, { arquivoId: 'f1', legenda: 'x', como: 'voz' });
    expect(chamadas).toEqual([
      { metodo: 'ctx.enviar', params: { execucao_id: 'e1', destino: { conversa_id: 'c1' }, conteudo: { texto: 'olá' }, citar_mensagem_id: 'm9' } },
      { metodo: 'ctx.enviar', params: { execucao_id: 'e1', destino: { telefone: '+55119', conta_id: 'k2' }, conteudo: { arquivo_id: 'f1', legenda: 'x', como: 'voz' } } },
    ]);
  });

  it('responder sem conversa → ErroValidacao sem chamar o motor', async () => {
    const { ctx, chamadas } = montar({ conversa: null });
    await expect(ctx.responder('x')).rejects.toBeInstanceOf(ErroValidacao);
    expect(chamadas).toEqual([]);
  });

  it('template mantém as variáveis do usuário; alvo vira snake_case', async () => {
    const { ctx, chamadas, respostas } = montar();
    respostas.set('ctx.funil.mover', () => ({ funil_id: 'f', funil: 'Vendas', etapa_id: 'e', etapa: 'Quente', desde: 'd' }));
    await ctx.enviar({ conversaId: 'c2' }, { template: 'boas', variaveis: { primeiro_nome: 'Ana' } });
    expect(await ctx.funil.mover('Vendas', 'Quente', { leadId: 'l7' })).toEqual({ funilId: 'f', funil: 'Vendas', etapaId: 'e', etapa: 'Quente', desde: 'd' });
    await ctx.etiquetas.adicionar('vip', { contatoId: 'ct9' });
    await ctx.etiquetas.doContato();
    expect(chamadas.map((c) => c.params)).toEqual([
      { execucao_id: 'e1', destino: { conversa_id: 'c2' }, conteudo: { template: 'boas', variaveis: { primeiro_nome: 'Ana' } } },
      { execucao_id: 'e1', funil: 'Vendas', etapa: 'Quente', alvo: { lead_id: 'l7' } },
      { execucao_id: 'e1', etiqueta: 'vip', alvo: { contato_id: 'ct9' } },
      { execucao_id: 'e1' },
    ]);
  });

  it('memória não converte valores; escopo padrão global', async () => {
    const { ctx, chamadas, respostas } = montar();
    respostas.set('ctx.memoria.obter', () => ({ valor: { minha_chave: [1] } }));
    respostas.set('ctx.memoria.remover', () => ({ removida: true }));
    respostas.set('ctx.memoria.listar', () => [{ chave: 'a', valor: { b_c: 1 } }]);
    expect(await ctx.memoria.obter('k')).toEqual({ minha_chave: [1] });
    await ctx.memoria.definir('k', { x_y: 2 }, { escopo: 'contato' });
    expect(await ctx.memoria.remover('k')).toBe(true);
    expect(await ctx.memoria.listar({ prefixo: 'a' })).toEqual([{ chave: 'a', valor: { b_c: 1 } }]);
    expect(chamadas.map((c) => [c.metodo, c.params])).toEqual([
      ['ctx.memoria.obter', { execucao_id: 'e1', chave: 'k', escopo: 'global' }],
      ['ctx.memoria.definir', { execucao_id: 'e1', chave: 'k', escopo: 'contato', valor: { x_y: 2 } }],
      ['ctx.memoria.remover', { execucao_id: 'e1', chave: 'k', escopo: 'global' }],
      ['ctx.memoria.listar', { execucao_id: 'e1', escopo: 'global', prefixo: 'a' }],
    ]);
  });

  it('leads: métodos e conversão; campos do usuário preservados', async () => {
    const { ctx, chamadas, respostas } = montar();
    const lead = { id: 'l1', telefone: '+55', nome: 'Ana', campos: { cidade_natal: 'SP' }, origem: 'csv', importado_em: 't' };
    respostas.set('ctx.leads.atualizar', () => lead);
    respostas.set('ctx.leads.buscar_telefone', () => null);
    expect(await ctx.leads.atualizar({ campos: { cidade_natal: 'SP', velho: null } })).toStrictEqual({
      id: 'l1', telefone: '+55', nome: 'Ana', campos: { cidade_natal: 'SP' }, origem: 'csv', importadoEm: 't',
    });
    expect(await ctx.leads.buscarPorTelefone('+55')).toBeNull();
    expect(chamadas[0]!.params).toEqual({ execucao_id: 'e1', campos: { cidade_natal: 'SP', velho: null } });
    expect(chamadas[1]!.metodo).toBe('ctx.leads.buscar_telefone');
  });

  it('IA: gerar, classificar (lista e mapa) e extrair', async () => {
    const { ctx, chamadas, respostas } = montar();
    respostas.set('ctx.ia.gerar', () => ({ texto: 't', modelo: 'claude-sonnet-5', motivo_parada: 'end_turn', tokens: { entrada: 3, saida: 4 } }));
    respostas.set('ctx.ia.classificar', () => ({ categoria: 'frio', tokens: { entrada: 1, saida: 1 } }));
    respostas.set('ctx.ia.extrair', () => ({ dados: { nome_completo: 'Ana' }, tokens: { entrada: 1, saida: 1 } }));
    expect(await ctx.ia.gerar({ mensagens: [{ papel: 'user', texto: 'oi' }], sistema: 's', maxTokens: 100 })).toEqual({
      texto: 't', modelo: 'claude-sonnet-5', motivoParada: 'end_turn', tokens: { entrada: 3, saida: 4 },
    });
    expect((await ctx.ia.classificar('x', ['quente', 'frio'])).categoria).toBe('frio');
    await ctx.ia.classificar('x', { quente: 'quer comprar', frio: 'sem interesse' }, { instrucoes: 'i', modelo: 'claude-opus-5-5' });
    expect((await ctx.ia.extrair('x', { type: 'object' })).dados).toEqual({ nome_completo: 'Ana' });
    expect(chamadas.map((c) => c.params)).toEqual([
      { execucao_id: 'e1', mensagens: [{ papel: 'user', texto: 'oi' }], sistema: 's', max_tokens: 100 },
      { execucao_id: 'e1', texto: 'x', categorias: { quente: null, frio: null } },
      { execucao_id: 'e1', texto: 'x', categorias: { quente: 'quer comprar', frio: 'sem interesse' }, instrucoes: 'i', modelo: 'claude-opus-5-5' },
      { execucao_id: 'e1', texto: 'x', esquema: { type: 'object' } },
    ]);
  });

  it('agendar: daquiSegundos vira data; exatamente um de em/daquiSegundos; naConversa padrão', async () => {
    const { ctx, chamadas, respostas } = montar();
    respostas.set('ctx.agendar', (p) => ({ id: 'ag1', em: p.em }));
    expect(await ctx.agendar({ daquiSegundos: 3600, dados: { a: 1 } })).toEqual({ id: 'ag1', em: new Date(1_000_000 + 3_600_000).toISOString() });
    await ctx.agendar({ em: new Date('2026-10-01T12:00:00Z'), naConversa: false });
    await expect(ctx.agendar({})).rejects.toBeInstanceOf(ErroValidacao);
    await expect(ctx.agendar({ em: 'x', daquiSegundos: 1 })).rejects.toBeInstanceOf(ErroValidacao);
    await expect(ctx.agendar({ em: 'não é data' })).rejects.toBeInstanceOf(ErroValidacao);
    expect(chamadas.map((c) => c.params)).toEqual([
      { execucao_id: 'e1', em: new Date(1_000_000 + 3_600_000).toISOString(), dados: { a: 1 }, na_conversa: true },
      { execucao_id: 'e1', em: '2026-10-01T12:00:00.000Z', na_conversa: false },
    ]);
    const semConversa = montar({ conversa: null });
    await semConversa.ctx.agendar({ daquiSegundos: 60 });
    expect(semConversa.chamadas[0]!.params.na_conversa).toBe(false);
  });

  it('humano, notificar, reagir e cancelarAgendamento', async () => {
    const { ctx, chamadas, respostas } = montar();
    respostas.set('ctx.cancelar_agendamento', () => ({ cancelado: false }));
    await ctx.humano.transferir({ motivo: 'pediu', duracaoMin: 30 });
    await ctx.notificar('T', 'x');
    await ctx.reagir('m1', '👍');
    expect(await ctx.cancelarAgendamento('ag1')).toBe(false);
    expect(chamadas.map((c) => [c.metodo, c.params])).toEqual([
      ['ctx.humano.transferir', { execucao_id: 'e1', motivo: 'pediu', duracao_min: 30 }],
      ['ctx.notificar', { execucao_id: 'e1', titulo: 'T', texto: 'x' }],
      ['ctx.reagir', { execucao_id: 'e1', mensagem_id: 'm1', emoji: '👍' }],
      ['ctx.cancelar_agendamento', { execucao_id: 'e1', id: 'ag1' }],
    ]);
  });
});

describe('erros do motor → classes da SDK', () => {
  const casos: [number, unknown, new (...a: never[]) => Error, Record<string, unknown>][] = [
    [1001, { codigo: 'permissao_negada', permissao: 'ia' }, ErroPermissao, { permissao: 'ia', codigo: 'permissao_negada' }],
    [1002, { codigo: 'validacao', campos: { texto: 'vazio' } }, ErroValidacao, { campos: { texto: 'vazio' }, codigo: 'validacao' }],
    [1003, { codigo: 'nao_encontrado' }, ErroNaoEncontrado, { codigo: 'nao_encontrado' }],
    [1004, { codigo: 'bloqueado', motivo: 'anti_loop' }, ErroBloqueado, { motivo: 'anti_loop', codigo: 'bloqueado' }],
    [1005, { codigo: 'ia_nao_configurada' }, ErroIA, { codigo: 'ia_nao_configurada', status: null }],
    [1006, { codigo: 'ia_erro', status: 529, request_id: 'req_9' }, ErroIA, { codigo: 'ia_erro', status: 529, requestId: 'req_9' }],
    [1007, { codigo: 'execucao_encerrada' }, ErroExecucaoEncerrada, { codigo: 'execucao_encerrada' }],
    [1008, { codigo: 'limite' }, ErroValidacao, { codigo: 'limite' }],
    [1009, { codigo: 'conta_indisponivel' }, ErroAutomacao, { codigo: 'conta_indisponivel' }],
    [1010, { codigo: 'segredo' }, ErroSegredo, { codigo: 'segredo' }],
    [-32602, undefined, ErroAutomacao, { codigo: 'erro' }],
  ];
  for (const [code, data, Classe, props] of casos) {
    it(`${code} → ${Classe.name}`, async () => {
      const { ctx, respostas } = montar();
      respostas.set('ctx.notificar', () => {
        throw new ErroRpc(code, `mensagem ${code}`, data);
      });
      const e = await ctx.notificar('a', 'b').catch((x: unknown) => x);
      expect(e).toBeInstanceOf(Classe);
      expect((e as Error).message).toBe(`mensagem ${code}`);
      expect(e).toMatchObject(props);
    });
  }

  it('permissão sem data.permissao é extraída da mensagem', () => {
    const e = erroDoMotor(new ErroRpc(1001, "Permissão 'funil' não declarada em automacao.json"));
    expect((e as ErroPermissao).permissao).toBe('funil');
  });
});

describe('fim e cancelamento', () => {
  it('cancelar aborta o sinal, rejeita pendentes e as chamadas seguintes', async () => {
    const { ctx, controle, respostas } = montar();
    respostas.set('ctx.leads.atual', () => new Promise(() => {}));
    const pendente = ctx.leads.atual();
    expect(ctx.sinal.aborted).toBe(false);
    controle.cancelar();
    expect(ctx.sinal.aborted).toBe(true);
    await expect(pendente).rejects.toBeInstanceOf(ErroExecucaoEncerrada);
    await expect(ctx.funil.listar()).rejects.toBeInstanceOf(ErroExecucaoEncerrada);
    expect(controle.encerrada).toBe(true);
  });

  it('após encerrar, promessas soltas falham e logs são descartados', async () => {
    const { ctx, controle, chamadas, notificacoes } = montar();
    controle.encerrar();
    await expect(ctx.enviar({ conversaId: 'c' }, { texto: 'x' })).rejects.toBeInstanceOf(ErroExecucaoEncerrada);
    ctx.log.info('tarde');
    expect(chamadas).toEqual([]);
    expect(notificacoes).toEqual([]);
    await expect(ctx.http.fetch('https://x.com')).rejects.toBeInstanceOf(ErroExecucaoEncerrada);
  });

  it('prazo aborta o sinal', async () => {
    vi.useFakeTimers();
    try {
      const { ctx } = montar({ prazo_ms: 1000 });
      vi.advanceTimersByTime(999);
      expect(ctx.sinal.aborted).toBe(false);
      vi.advanceTimersByTime(1);
      expect(ctx.sinal.aborted).toBe(true);
    } finally {
      vi.useRealTimers();
    }
  });
});

describe('segredos', () => {
  it('obter é síncrono; não declarado → ErroSegredo; tem() sem lançar', () => {
    const { ctx } = montar();
    expect(ctx.segredos.obter('OPENAI_KEY')).toBe('sk-1');
    expect(ctx.segredos.tem('OPENAI_KEY')).toBe(true);
    expect(ctx.segredos.tem('ANTHROPIC_API_KEY')).toBe(false);
    expect(() => ctx.segredos.obter('ANTHROPIC_API_KEY')).toThrow(ErroSegredo);
    expect(() => ctx.segredos.obter('toString')).toThrow(ErroSegredo);
  });
});

describe('ctx.http.fetch', () => {
  it('sem permissão rede → ErroPermissao sem chamar fetch', async () => {
    const fetchInterno = vi.fn();
    const { ctx } = montar({}, { fetchInterno: fetchInterno as never });
    const e = await ctx.http.fetch('https://api.exemplo.com').catch((x: unknown) => x);
    expect(e).toBeInstanceOf(ErroPermissao);
    expect((e as ErroPermissao).permissao).toBe('rede');
    expect(fetchInterno).not.toHaveBeenCalled();
  });

  it('com rede: registra notificação http sem query; só http(s); repassa o sinal', async () => {
    const fetchInterno = vi.fn(async (_u: URL, init: RequestInit) => {
      expect(init.signal).toBeInstanceOf(AbortSignal);
      return new Response('{}', { status: 201 });
    });
    const { ctx, notificacoes } = montar({}, { fetchInterno: fetchInterno as never, permissoes: ['rede'] });
    const r = await ctx.http.fetch('https://api.exemplo.com/v1/x?token=segredo#h', { method: 'post', body: 'corpo' });
    expect(r.status).toBe(201);
    expect(notificacoes).toHaveLength(1);
    expect(notificacoes[0]).toMatchObject({
      metodo: 'http',
      params: { execucao_id: 'e1', metodo: 'POST', url_sem_query: 'https://api.exemplo.com/v1/x', status: 201, erro: null },
    });
    expect(JSON.stringify(notificacoes)).not.toContain('segredo');
    expect(JSON.stringify(notificacoes)).not.toContain('corpo');
    await expect(ctx.http.fetch('file:///etc/passwd')).rejects.toBeInstanceOf(ErroValidacao);
    await expect(ctx.http.fetch('não é url')).rejects.toBeInstanceOf(ErroValidacao);
  });

  it('falha de rede é registrada com erro e status null', async () => {
    const fetchInterno = vi.fn(async () => {
      throw new TypeError('fetch failed');
    });
    const { ctx, notificacoes } = montar({}, { fetchInterno: fetchInterno as never, permissoes: ['rede'] });
    await expect(ctx.http.fetch('http://localhost:1/a?b=c')).rejects.toThrow('fetch failed');
    expect(notificacoes[0]).toMatchObject({ params: { status: null, erro: 'TypeError: fetch failed', url_sem_query: 'http://localhost:1/a' } });
  });
});

describe('historicoParaIA', () => {
  const m = (id: string, deMim: boolean, tipo: Mensagem['tipo'], texto: string | null, seg: number): Mensagem => ({
    id, conversaId: 'c1', deMim, automacaoId: null, tipo, texto, remetenteNome: null,
    enviadaEm: new Date(Date.UTC(2026, 8, 27, 10, 0, seg)).toISOString(),
  });

  it('cronológico, papéis, união de seguidas, mídia rotulada, sistema ignorado, começa pelo contato', () => {
    // mais recentes primeiro, como ctx.conversa.historico
    const hist = [
      m('9', false, 'texto', 'e o preço?', 9),
      m('8', true, 'imagem', 'catálogo', 8),
      m('7', true, 'texto', 'Segue:', 7),
      m('6', false, 'sistema', 'mudou o número', 6),
      m('5', false, 'audio', null, 5),
      m('4', false, 'texto', 'oi', 4),
      m('3', false, 'texto', '   ', 3),
      m('2', true, 'texto', 'Olá! (mensagem antiga do bot)', 2),
    ];
    expect(historicoParaIA(hist)).toEqual([
      { papel: 'user', texto: 'oi\n[áudio]' },
      { papel: 'assistant', texto: 'Segue:\n[imagem] catálogo' },
      { papel: 'user', texto: 'e o preço?' },
    ]);
  });

  it('mesmo horário mantém a ordem recebida invertida; vazio → []', () => {
    const hist = [m('b', true, 'texto', 'segunda', 1), m('a', false, 'texto', 'primeira', 1)];
    expect(historicoParaIA(hist)).toEqual([
      { papel: 'user', texto: 'primeira' },
      { papel: 'assistant', texto: 'segunda' },
    ]);
    expect(historicoParaIA([])).toEqual([]);
  });

  it('ctx.conversa.historicoParaIA pede limite 20 por padrão', async () => {
    const { ctx, chamadas, respostas } = montar();
    respostas.set('ctx.conversa.historico', () => [
      { id: '1', conversa_id: 'c1', de_mim: false, automacao_id: null, tipo: 'texto', texto: 'oi', remetente_nome: null, enviada_em: '2026-09-27T10:00:00Z' },
    ]);
    expect(await ctx.conversa!.historicoParaIA()).toEqual([{ papel: 'user', texto: 'oi' }]);
    expect(chamadas[0]).toEqual({ metodo: 'ctx.conversa.historico', params: { execucao_id: 'e1', conversa_id: 'c1', limite: 20 } });
    const h = await ctx.conversa!.historico({ limite: 5, antes: 'm1' });
    expect(h[0]).toMatchObject({ conversaId: 'c1', deMim: false, enviadaEm: '2026-09-27T10:00:00Z' });
  });
});
