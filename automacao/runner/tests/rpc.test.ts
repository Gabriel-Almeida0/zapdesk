import { describe, expect, it, vi } from 'vitest';
import { ConexaoRpc, ErroRpc } from '../src/rpc.js';

/** Liga duas conexões em memória (A escreve → B recebe e vice-versa), em pedaços pequenos. */
function par(opA: Partial<ConstructorParameters<typeof ConexaoRpc>[0]> = {}, opB: typeof opA = {}) {
  const saidaA: string[] = [];
  const saidaB: string[] = [];
  // eslint-disable-next-line prefer-const
  let a: ConexaoRpc, b: ConexaoRpc;
  a = new ConexaoRpc({
    ...opA,
    escrever: (l) => {
      saidaA.push(l);
      // entrega em pedaços de 7 bytes para exercitar a remontagem de linhas
      const buf = Buffer.from(l);
      queueMicrotask(() => {
        for (let i = 0; i < buf.length; i += 7) b.receber(buf.subarray(i, i + 7));
      });
    },
  });
  b = new ConexaoRpc({
    ...opB,
    escrever: (l) => {
      saidaB.push(l);
      queueMicrotask(() => a.receber(l));
    },
  });
  return { a, b, saidaA, saidaB };
}

describe('NDJSON JSON-RPC bidirecional', () => {
  it('requisições concorrentes nos dois sentidos, com ids por lado', async () => {
    const { a, b, saidaA } = par(
      { aoRequisicao: async (m, p) => ({ de: 'a', m, p }) },
      {
        aoRequisicao: async (m, p) => {
          await new Promise((r) => setTimeout(r, (p as { atraso: number }).atraso));
          return { de: 'b', m, p };
        },
      },
    );
    const [r1, r2, r3] = await Promise.all([
      a.requisitar('lento', { atraso: 30 }),
      a.requisitar('rapido', { atraso: 1 }),
      b.requisitar('ctx.x', { ç: 'ãé✓' }),
    ]);
    expect(r1).toEqual({ de: 'b', m: 'lento', p: { atraso: 30 } });
    expect(r2).toEqual({ de: 'b', m: 'rapido', p: { atraso: 1 } });
    expect(r3).toEqual({ de: 'a', m: 'ctx.x', p: { ç: 'ãé✓' } });
    const ids = saidaA.filter((l) => l.includes('"method"')).map((l) => JSON.parse(l).id);
    expect(ids).toEqual([1, 2]);
    expect(a.emAberto).toBe(0);
  });

  it('notificações não têm id nem resposta', async () => {
    const recebidas: unknown[] = [];
    const { a, saidaA, saidaB } = par({}, { aoNotificacao: (m, p) => recebidas.push([m, p]) });
    a.notificar('log', { texto: 'oi' });
    await new Promise((r) => setTimeout(r, 5));
    expect(recebidas).toEqual([['log', { texto: 'oi' }]]);
    expect(JSON.parse(saidaA[0]!)).toEqual({ jsonrpc: '2.0', method: 'log', params: { texto: 'oi' } });
    expect(saidaB).toEqual([]);
  });

  it('erros: ErroRpc vira error com code/data; exceção comum vira -32603; método inexistente -32601', async () => {
    const { a } = par(
      {},
      {
        aoRequisicao: (m) => {
          if (m === 'dom') throw new ErroRpc(1001, "Permissão 'enviar' não declarada", { codigo: 'permissao_negada', permissao: 'enviar' });
          if (m === 'quebra') throw new Error('boom');
          return undefined;
        },
      },
    );
    const e1 = (await a.requisitar('dom').catch((e: ErroRpc) => e)) as ErroRpc;
    expect(e1).toBeInstanceOf(ErroRpc);
    expect([e1.code, e1.data]).toEqual([1001, { codigo: 'permissao_negada', permissao: 'enviar' }]);
    expect((await a.requisitar('quebra').catch((e: ErroRpc) => e) as ErroRpc).code).toBe(-32603);
    const e3 = (await a.requisitar('nada').catch((e: ErroRpc) => e)) as ErroRpc;
    expect(e3.code).toBe(-32601);
    expect(e3.message).toContain('nada');
  });

  it('linha > 1 MB → -32600 e é descartada; a conexão continua', async () => {
    const escritas: string[] = [];
    const c = new ConexaoRpc({
      escrever: (l) => escritas.push(l),
      aoRequisicao: () => ({ ok: true }),
    });
    const grande = JSON.stringify({ jsonrpc: '2.0', id: 9, method: 'x', params: { t: 'a'.repeat(1024 * 1024) } });
    // chega em pedaços: estoura no meio
    for (let i = 0; i < grande.length; i += 65536) c.receber(grande.slice(i, i + 65536));
    c.receber('\n');
    c.receber('{"jsonrpc":"2.0","id":10,"method":"ping"}\n');
    await new Promise((r) => setTimeout(r, 5));
    expect(escritas.map((l) => JSON.parse(l))).toEqual([
      { jsonrpc: '2.0', id: null, error: { code: -32600, message: 'Linha maior que 1 MB descartada.' } },
      { jsonrpc: '2.0', id: 10, result: { ok: true } },
    ]);
  });

  it('JSON inválido → -32700; mensagem sem jsonrpc → -32600; resposta desconhecida é ignorada', async () => {
    const escritas: string[] = [];
    const c = new ConexaoRpc({ escrever: (l) => escritas.push(l) });
    c.receber('{nao json\n{"id":3,"method":"x"}\n{"jsonrpc":"2.0","id":77,"result":1}\n\n');
    expect(escritas.map((l) => JSON.parse(l).error.code)).toEqual([-32700, -32600]);
    expect(JSON.parse(escritas[1]!).id).toBe(3);
  });

  it('requisição própria acima de 1 MB é rejeitada sem escrever', async () => {
    const escrever = vi.fn();
    const c = new ConexaoRpc({ escrever });
    const e = await c.requisitar('ctx.enviar', { texto: 'x'.repeat(1024 * 1024) }).catch((x: ErroRpc) => x) as ErroRpc;
    expect(e.code).toBe(-32600);
    expect(e.data).toEqual({ codigo: 'limite' });
    expect(escrever).not.toHaveBeenCalled();
  });

  it('EOF encerra: rejeita pendentes, chama aoFim e recusa novas requisições', async () => {
    const aoFim = vi.fn();
    const c = new ConexaoRpc({ escrever: () => {}, aoFim });
    const p = c.requisitar('x');
    c.fim();
    await expect(p).rejects.toThrow('Conexão encerrada.');
    await expect(c.requisitar('y')).rejects.toThrow('Conexão encerrada.');
    c.fim();
    expect(aoFim).toHaveBeenCalledTimes(1);
  });

  it('caractere multibyte partido entre pedaços é remontado', () => {
    const recebidas: unknown[] = [];
    const c = new ConexaoRpc({ escrever: () => {}, aoNotificacao: (_m, p) => recebidas.push(p) });
    const buf = Buffer.from('{"jsonrpc":"2.0","method":"log","params":"ação ✓"}\n');
    const meio = buf.indexOf(0xc3) + 1; // parte no meio do "ç"
    c.receber(buf.subarray(0, meio));
    c.receber(buf.subarray(meio));
    expect(recebidas).toEqual(['ação ✓']);
  });
});
