// Ciclo de vida e execução no runner real (runner-protocolo.md): pronto, inicializar, ping,
// executar (handlers, erros 2002/2003, retorno, logs, cancelar), encerrar e stdin EOF.
import { afterEach, describe, expect, it } from 'vitest';
import { compilarProjeto, erroDe, gravarBundle, MotorFalso, pastaTemporaria } from './apoio/motor-falso.js';

const motores: MotorFalso[] = [];
afterEach(() => {
  for (const m of motores.splice(0)) m.matar();
});

function motorCom(bundle: string): MotorFalso {
  const m = new MotorFalso({ permitirLeitura: [bundle] });
  motores.push(m);
  return m;
}

const SDK = `import { definirAutomacao } from '@zapdesk/automacao';`;

describe('ciclo de vida', () => {
  it('envia pronto com versão, protocolo e node; responde ping', async () => {
    const pasta = pastaTemporaria();
    const bundle = gravarBundle(pasta, 'b.mjs', `${SDK} export default definirAutomacao({});`);
    const m = motorCom(bundle);
    const pronto = await m.pronto;
    expect(pronto).toMatchObject({ versao_runner: '1.0.0', protocolo: 1 });
    expect(pronto.node).toMatch(/^\d+\.\d+\.\d+$/);
    expect(await m.requisitar('ping')).toEqual({ ok: true });
  });

  it('inicializar importa o bundle e lista os handlers', async () => {
    const pasta = pastaTemporaria();
    const bundle = gravarBundle(
      pasta,
      'b.mjs',
      `${SDK} export default definirAutomacao({ aoEvento() {}, aoReceberMensagem() {}, aoExecutar() {} });`,
    );
    const m = motorCom(bundle);
    expect(await m.inicializar(bundle)).toEqual({ handlers: ['aoReceberMensagem', 'aoExecutar', 'aoEvento'] });
    // segundo inicializar é recusado
    expect((await erroDe(m.inicializar(bundle))).code).toBe(2001);
  });

  it('export default sem definirAutomacao → 2001 bundle_invalido', async () => {
    const pasta = pastaTemporaria();
    const bundle = gravarBundle(pasta, 'b.mjs', `export default { aoExecutar() {} };`);
    const m = motorCom(bundle);
    const e = await erroDe(m.inicializar(bundle));
    expect(e.code).toBe(2001);
    expect(e.message).toContain('export default definirAutomacao');
  });

  it('erro no topo do módulo → 2001 com stack; definirAutomacao inválido também', async () => {
    const pasta = pastaTemporaria();
    const b1 = gravarBundle(pasta, 'b1.mjs', `${SDK} throw new Error('quebrou no topo');`);
    const m1 = motorCom(b1);
    const e1 = await erroDe(m1.inicializar(b1));
    expect(e1.code).toBe(2001);
    expect(e1.message).toContain('quebrou no topo');
    expect((e1.data as { stack: string }).stack).toContain('quebrou no topo');

    const b2 = gravarBundle(pasta, 'b2.mjs', `${SDK} export default definirAutomacao({ aoRecebe() {} });`);
    const m2 = motorCom(b2);
    const e2 = await erroDe(m2.inicializar(b2));
    expect(e2.code).toBe(2001);
    expect(e2.message).toContain('Handler desconhecido "aoRecebe"');
  });

  it('bundle fora do --allow-fs-read não carrega (Permission Model)', async () => {
    const pasta = pastaTemporaria();
    const permitido = gravarBundle(pasta, 'ok.mjs', `${SDK} export default definirAutomacao({});`);
    const outro = gravarBundle(pasta, 'outro.mjs', `${SDK} export default definirAutomacao({});`);
    const m = motorCom(permitido);
    const e = await erroDe(m.inicializar(outro));
    expect(e.code).toBe(2001);
    expect(e.message).toMatch(/ERR_ACCESS_DENIED|Access to this API has been restricted|permission/i);
  });

  it('método desconhecido → -32601; executar antes de inicializar → erro', async () => {
    const pasta = pastaTemporaria();
    const bundle = gravarBundle(pasta, 'b.mjs', `${SDK} export default definirAutomacao({});`);
    const m = motorCom(bundle);
    await m.pronto;
    expect((await erroDe(m.requisitar('xyz'))).code).toBe(-32601);
    expect((await erroDe(m.executar({ execucao_id: 'e', handler: 'aoExecutar' }))).message).toContain('não inicializado');
  });

  it('encerrar responde {} e sai com código 0', async () => {
    const pasta = pastaTemporaria();
    const bundle = gravarBundle(pasta, 'b.mjs', `${SDK} export default definirAutomacao({});`);
    const m = motorCom(bundle);
    await m.inicializar(bundle);
    expect(await m.requisitar('encerrar')).toEqual({});
    expect(await m.saida).toEqual({ codigo: 0, sinal: null });
  });

  it('stdin EOF → sai com código 0 imediatamente (mesmo com execução pendurada)', async () => {
    const pasta = pastaTemporaria();
    const bundle = gravarBundle(
      pasta,
      'b.mjs',
      `${SDK} export default definirAutomacao({ aoExecutar() { return new Promise(() => {}); } });`,
    );
    const m = motorCom(bundle);
    await m.inicializar(bundle);
    void m.executar({ execucao_id: 'e1', handler: 'aoExecutar' }).catch(() => {});
    await new Promise((r) => setTimeout(r, 50));
    const inicio = Date.now();
    m.proc.stdin.end();
    expect(await m.saida).toEqual({ codigo: 0, sinal: null });
    expect(Date.now() - inicio).toBeLessThan(1000);
  });
});

describe('executar', () => {
  it('passa ctx e argumento; retorno só de aoExecutar; undefined vira null', async () => {
    const pasta = pastaTemporaria();
    const bundle = gravarBundle(
      pasta,
      'b.mjs',
      `${SDK} export default definirAutomacao({
        aoExecutar(ctx, entrada) { return { entrada, id: ctx.execucao.id, sim: ctx.execucao.simulacao, nome: ctx.execucao.automacaoNome, conversa: ctx.conversa, prazo: ctx.execucao.prazoMs > 0 }; },
        aoEvento(ctx, ev) { return 'ignorado'; },
        aoAgendar() {},
      });`,
    );
    const m = motorCom(bundle);
    await m.inicializar(bundle);
    expect(
      await m.executar({ execucao_id: 'e1', handler: 'aoExecutar', simulacao: true, argumento: { a_b: [1, { c_d: 2 }] } }),
    ).toEqual({
      retorno: { entrada: { a_b: [1, { c_d: 2 }] }, id: 'e1', sim: true, nome: 'Teste', conversa: null, prazo: true },
    });
    expect(await m.executar({ execucao_id: 'e2', handler: 'aoEvento', argumento: { tipo: 'etiqueta' } })).toEqual({ retorno: null });
    expect(await m.executar({ execucao_id: 'e3', handler: 'aoAgendar' })).toEqual({ retorno: null });
  });

  it('handler ausente → 2003 handler_ausente', async () => {
    const pasta = pastaTemporaria();
    const bundle = gravarBundle(pasta, 'b.mjs', `${SDK} export default definirAutomacao({ aoExecutar() {} });`);
    const m = motorCom(bundle);
    await m.inicializar(bundle);
    const e = await erroDe(m.executar({ execucao_id: 'e1', handler: 'aoEvento' }));
    expect(e.code).toBe(2003);
    expect(e.message).toBe('Handler aoEvento não exportado.');
  });

  it('erro do usuário → 2002 com nome, mensagem e stack mapeado para o .ts; processo continua', async () => {
    const pasta = pastaTemporaria();
    const bundle = await compilarProjeto(pasta, {
      'index.ts': `import { definirAutomacao } from '@zapdesk/automacao';
import { validar } from './util/validar';

export default definirAutomacao({
  aoExecutar(_ctx, entrada) {
    validar(entrada);
    return 'ok';
  },
});
`,
      'util/validar.ts': `export function validar(valor: unknown): void {
  if (valor === null) {
    throw new TypeError('entrada obrigatória');
  }
}
`,
    });
    const m = motorCom(bundle);
    await m.inicializar(bundle);
    const e = await erroDe(m.executar({ execucao_id: 'e1', handler: 'aoExecutar', argumento: null }));
    expect(e.code).toBe(2002);
    const d = e.data as { codigo: string; nome: string; mensagem: string; stack: string };
    expect(d.codigo).toBe('erro_usuario');
    expect(d.nome).toBe('TypeError');
    expect(d.mensagem).toBe('entrada obrigatória');
    expect(d.stack).toMatch(/util\/validar\.ts:3:\d+/);
    expect(d.stack).toMatch(/index\.ts:6:\d+/);
    expect(d.stack).not.toContain('zapdesk-runner.mjs');
    // o processo segue atendendo
    expect(await m.executar({ execucao_id: 'e2', handler: 'aoExecutar', argumento: 1 })).toEqual({ retorno: 'ok' });
  });

  it('retorno > 64 KB ou não-JSON → 2002', async () => {
    const pasta = pastaTemporaria();
    const bundle = gravarBundle(
      pasta,
      'b.mjs',
      `${SDK} export default definirAutomacao({ aoExecutar(_c, e) {
        if (e === 'grande') return 'x'.repeat(70 * 1024);
        const o = {}; o.o = o; return o; } });`,
    );
    const m = motorCom(bundle);
    await m.inicializar(bundle);
    expect((await erroDe(m.executar({ execucao_id: 'e1', handler: 'aoExecutar', argumento: 'grande' }))).message).toContain('64 KB');
    const e2 = await erroDe(m.executar({ execucao_id: 'e2', handler: 'aoExecutar', argumento: 'circular' }));
    expect(e2.code).toBe(2002);
    expect(e2.message).toContain('não é JSON válido');
  });

  it('console.* e ctx.log viram notificações log da execução certa (concorrentes); fora de execução descarta', async () => {
    const pasta = pastaTemporaria();
    const bundle = gravarBundle(
      pasta,
      'b.mjs',
      `${SDK}
       console.log('no topo');
       export default definirAutomacao({ async aoExecutar(ctx, e) {
         await new Promise((r) => setTimeout(r, e.atraso));
         console.log('console', e.n, { profundo: { a: { b: { c: { d: { e: 1 } } } } } });
         console.warn('aviso', e.n);
         console.error(new Error('erro ' + e.n).message);
         process.stdout.write('stdout ' + e.n + '\\n');
         ctx.log.debug('dbg', e.n);
         ctx.log.info('x'.repeat(10000));
         return e.n;
       } });`,
    );
    const m = motorCom(bundle);
    await m.inicializar(bundle);
    const [a, b] = await Promise.all([
      m.executar({ execucao_id: 'A', handler: 'aoExecutar', argumento: { n: 'A', atraso: 30 } }),
      m.executar({ execucao_id: 'B', handler: 'aoExecutar', argumento: { n: 'B', atraso: 1 } }),
    ]);
    expect([a, b]).toEqual([{ retorno: 'A' }, { retorno: 'B' }]);
    const logs = m.notificacoes.filter((n) => n.metodo === 'log').map((n) => n.params as { execucao_id: string; nivel: string; texto: string; em: string });
    expect(logs.some((l) => l.texto.includes('no topo'))).toBe(false);
    for (const id of ['A', 'B']) {
      const meus = logs.filter((l) => l.execucao_id === id);
      expect(meus.map((l) => l.nivel)).toEqual(['info', 'aviso', 'erro', 'info', 'debug', 'info']);
      expect(meus[0]!.texto).toBe(`console ${id} { profundo: { a: { b: { c: { d: [Object] } } } } }`);
      expect(meus[3]!.texto).toBe(`stdout ${id}`);
      expect(Buffer.byteLength(meus[5]!.texto)).toBeLessThanOrEqual(8 * 1024);
      expect(meus[5]!.texto.endsWith('…')).toBe(true);
      expect(Number.isNaN(Date.parse(meus[0]!.em))).toBe(false);
    }
  });

  it('cancelar aborta ctx.sinal e chamadas ctx.* passam a falhar com ErroExecucaoEncerrada', async () => {
    const pasta = pastaTemporaria();
    const bundle = gravarBundle(
      pasta,
      'b.mjs',
      `import { definirAutomacao, ErroExecucaoEncerrada } from '@zapdesk/automacao';
       export default definirAutomacao({ async aoExecutar(ctx) {
         const pendente = ctx.memoria.obter('x').then(() => 'resolveu', (e) => e.constructor.name + ':' + (e instanceof ErroExecucaoEncerrada));
         await new Promise((r) => ctx.sinal.addEventListener('abort', r));
         const depois = await ctx.notificar('a', 'b').then(() => 'ok', (e) => e.name);
         return { pendente: await pendente, depois, abortado: ctx.sinal.aborted };
       } });`,
    );
    const m = motorCom(bundle);
    m.tratar('ctx.memoria.obter', () => new Promise(() => {})); // o motor nunca responde
    await m.inicializar(bundle);
    const p = m.executar({ execucao_id: 'e1', handler: 'aoExecutar' });
    await new Promise((r) => setTimeout(r, 50));
    m.notificar('cancelar', { execucao_id: 'e1' });
    expect(await p).toEqual({
      retorno: { pendente: 'ErroExecucaoEncerrada:true', depois: 'ErroExecucaoEncerrada', abortado: true },
    });
  });

  it('promessa solta chamando ctx depois do fim falha com ErroExecucaoEncerrada; erro solto não derruba o processo', async () => {
    const pasta = pastaTemporaria();
    const bundle = gravarBundle(
      pasta,
      'b.mjs',
      `${SDK} let solta;
       export default definirAutomacao({ async aoExecutar(ctx, e) {
         if (e === 'primeira') {
           solta = new Promise((r) => setTimeout(r, 20)).then(() => ctx.log.info('tarde') ?? ctx.notificar('a', 'b')).then(() => 'ok', (x) => x.name);
           setTimeout(() => { throw new Error('erro solto em timer'); }, 5);
           Promise.reject(new Error('rejeição solta'));
           return 1;
         }
         await new Promise((r) => setTimeout(r, 40));
         return await solta;
       } });`,
    );
    const m = motorCom(bundle);
    await m.inicializar(bundle);
    expect(await m.executar({ execucao_id: 'e1', handler: 'aoExecutar', argumento: 'primeira' })).toEqual({ retorno: 1 });
    expect(await m.executar({ execucao_id: 'e2', handler: 'aoExecutar', argumento: 'segunda' })).toEqual({ retorno: 'ErroExecucaoEncerrada' });
    expect(m.chamadas.filter((c) => c.metodo === 'ctx.notificar')).toEqual([]);
    expect(m.notificacoes.some((n) => n.metodo === 'log' && (n.params as { texto: string }).texto === 'tarde')).toBe(false);
    expect(m.stderr).toContain('erro solto em timer');
    expect(m.stderr).toContain('rejeição solta');
    expect(await m.requisitar('ping')).toEqual({ ok: true });
  });

  it('laço infinito trava o processo (o motor mata no prazo) sem afetar outro processo', async () => {
    const pasta = pastaTemporaria();
    const trava = gravarBundle(pasta, 't.mjs', `${SDK} export default definirAutomacao({ aoExecutar() { while (true) {} } });`);
    const ok = gravarBundle(pasta, 'o.mjs', `${SDK} export default definirAutomacao({ aoExecutar() { return 'vivo'; } });`);
    const m1 = motorCom(trava);
    const m2 = motorCom(ok);
    await Promise.all([m1.inicializar(trava), m2.inicializar(ok)]);
    void m1.executar({ execucao_id: 'e1', handler: 'aoExecutar', prazo_ms: 200 }).catch(() => {});
    await new Promise((r) => setTimeout(r, 100));
    const inicio = Date.now();
    expect(await m2.executar({ execucao_id: 'x', handler: 'aoExecutar' })).toEqual({ retorno: 'vivo' });
    expect(Date.now() - inicio).toBeLessThan(1000);
    // simula o motor: SIGKILL no prazo
    m1.proc.kill('SIGKILL');
    expect((await m1.saida).sinal).toBe('SIGKILL');
  });

  it('memória limitada: estourar o heap mata só o processo (stderr com "heap out of memory")', async () => {
    const pasta = pastaTemporaria();
    const bundle = gravarBundle(
      pasta,
      'b.mjs',
      `${SDK} export default definirAutomacao({ aoExecutar() { const a = []; while (true) a.push(new Array(1e5).fill(Math.random())); } });`,
    );
    const m = new MotorFalso({ permitirLeitura: [bundle], memoriaMb: 32 });
    motores.push(m);
    await m.inicializar(bundle);
    void m.executar({ execucao_id: 'e1', handler: 'aoExecutar' }).catch(() => {});
    const s = await m.saida;
    expect(s.codigo === 0).toBe(false);
    expect(m.stderr).toMatch(/heap out of memory/i);
  }, 20_000);
});
