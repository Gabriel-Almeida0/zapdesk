// Bloqueios de módulos e globais (research.md R6, camada 3) no runner real, com o Permission
// Model ligado como o motor faz.
import { afterEach, describe, expect, it } from 'vitest';
import { ErroRpc } from '../src/rpc.js';
import { gravarBundle, MotorFalso, pastaTemporaria } from './apoio/motor-falso.js';

const motores: MotorFalso[] = [];
afterEach(() => {
  for (const m of motores.splice(0)) m.matar();
});

async function rodar(corpo: string, extraTopo = ""): Promise<any> {
  const pasta = pastaTemporaria();
  const bundle = gravarBundle(
    pasta,
    'b.mjs',
    `import { definirAutomacao, ErroPermissao } from '@zapdesk/automacao';
${extraTopo}
async function tentar(f) { try { const r = await f(); return r === undefined ? 'ok' : r; } catch (e) { return 'ERRO: ' + e.message; } }
export default definirAutomacao({ async aoExecutar(ctx) { ${corpo} } });`,
  );
  const m = new MotorFalso({ permitirLeitura: [bundle] });
  motores.push(m);
  await m.inicializar(bundle);
  const r = (await m.executar({ execucao_id: 'e1', handler: 'aoExecutar' })) as { retorno: Record<string, unknown> };
  return r.retorno;
}

describe('bloqueios do runner', () => {
  it('import() de módulos proibidos lança "Módulo não permitido"', async () => {
    const proibidos = [
      'node:fs', 'fs', 'fs/promises', 'node:child_process', 'child_process', 'net', 'node:tls', 'dgram',
      'http', 'https', 'http2', 'worker_threads', 'cluster', 'vm', 'inspector', 'module', 'v8', 'os',
      'process', 'node:path', 'pacote-npm-qualquer', '../fora.mjs', 'data:text/javascript,export default 1',
    ];
    const r = await rodar(`
      const saida = {};
      for (const m of ${JSON.stringify(proibidos)}) saida[m] = await tentar(() => import(m).then(() => 'importou'));
      return saida;`);
    for (const m of proibidos) expect(r[m], m).toBe(`ERRO: Módulo não permitido: ${m}`);
  });

  it('require via createRequire/getBuiltinModule também é barrado', async () => {
    const r = await rodar(`return {
      getBuiltin: await tentar(() => process.getBuiltinModule('child_process')),
      getBuiltinNode: await tentar(() => process.getBuiltinModule('node:fs')),
      permitido: typeof process.getBuiltinModule('node:crypto').randomUUID,
    };`);
    expect(r).toEqual({
      getBuiltin: 'ERRO: Módulo não permitido: child_process',
      getBuiltinNode: 'ERRO: Módulo não permitido: node:fs',
      permitido: 'function',
    });
  });

  it('import estático proibido no topo do bundle falha o inicializar (2001)', async () => {
    const pasta = pastaTemporaria();
    const bundle = gravarBundle(
      pasta,
      'b.mjs',
      `import { definirAutomacao } from '@zapdesk/automacao'; import { readFileSync } from 'node:fs';
       export default definirAutomacao({ aoExecutar() { return typeof readFileSync; } });`,
    );
    const m = new MotorFalso({ permitirLeitura: [bundle] });
    motores.push(m);
    const e = (await m.inicializar(bundle).catch((x) => x)) as { code: number; message: string; data: { stack: string } };
    expect(e.code).toBe(2001);
    expect(e.message).toContain('Módulo não permitido: node:fs');
  });

  it('módulos permitidos funcionam (com e sem node:)', async () => {
    const r = await rodar(`
      const { randomUUID } = await import('node:crypto');
      const { format } = await import('util');
      const { setTimeout: dormir } = await import('node:timers/promises');
      const pp = await import('node:path/posix');
      const qs = await import('node:querystring');
      await dormir(1);
      return { uuid: randomUUID().length, f: format('%s-%d', 'a', 1), j: pp.join('a', 'b'), q: qs.stringify({ a: 1 }),
               url: typeof URL, buf: (await import('node:buffer')).Buffer.from('oi').toString('base64') };`);
    expect(r).toEqual({ uuid: 36, f: 'a-1', j: 'a/b', q: 'a=1', url: 'function', buf: 'b2k=' });
  });

  it('globais de rede removidos, process.env vazio e operações de processo neutralizadas', async () => {
    const r = await rodar(`return {
      fetch: typeof globalThis.fetch, ws: typeof globalThis.WebSocket, es: typeof globalThis.EventSource,
      envChaves: Object.keys(process.env).length, envCongelado: Object.isFrozen(process.env),
      path: process.env.PATH ?? null,
      kill: await tentar(() => process.kill(process.pid, 'SIGTERM')),
      exit: await tentar(() => process.exit(3)),
      chdir: await tentar(() => process.chdir('/')),
      binding: await tentar(() => process.binding('fs')),
      dlopen: await tentar(() => process.dlopen({}, '/tmp/x.node')),
      fsLeitura: await tentar(async () => (await import('node:fs')).readFileSync('/etc/hosts')),
    };`);
    expect(r).toEqual({
      fetch: 'undefined', ws: 'undefined', es: 'undefined',
      envChaves: 0, envCongelado: true, path: null,
      kill: 'ERRO: Operação não permitida em automações: process.kill',
      exit: 'ERRO: Operação não permitida em automações: process.exit',
      chdir: 'ERRO: Operação não permitida em automações: process.chdir',
      binding: 'ERRO: Operação não permitida em automações: process.binding',
      dlopen: 'ERRO: Operação não permitida em automações: process.dlopen',
      fsLeitura: 'ERRO: Módulo não permitido: node:fs',
    });
  });

  it('@zapdesk/automacao resolve para o runtime interno (mesmas classes do runner)', async () => {
    const pasta = pastaTemporaria();
    const bundle = gravarBundle(
      pasta,
      'b.mjs',
      `import * as sdk from '@zapdesk/automacao';
       export default sdk.definirAutomacao({ async aoExecutar(ctx) {
         try { await ctx.enviar({ conversaId: 'c1' }, { texto: 'oi' }); return 'enviou'; }
         catch (e) { return { permissao: e instanceof sdk.ErroPermissao, base: e instanceof sdk.ErroAutomacao,
                             nome: e.name, qual: e.permissao, versao: sdk.VERSAO_SDK }; }
       } });`,
    );
    const m = new MotorFalso({ permitirLeitura: [bundle] });
    motores.push(m);
    // o motor responde 1001 com data.permissao
    m.tratar('ctx.enviar', () => {
      throw new ErroRpc(1001, "Permissão 'enviar' não declarada em automacao.json", { codigo: 'permissao_negada', permissao: 'enviar' });
    });
    await m.inicializar(bundle);
    const r = (await m.executar({ execucao_id: 'e1', handler: 'aoExecutar' })) as { retorno: unknown };
    expect(r.retorno).toEqual({ permissao: true, base: true, nome: 'ErroPermissao', qual: 'enviar', versao: '1.0.0' });
  });

  it('o Permission Model impede ler arquivos fora do bundle mesmo por caminhos permitidos', async () => {
    // Sem node:fs disponível, o único jeito "acidental" seria import de arquivo: barrado no hook.
    const r = await rodar(`return await tentar(() => import('file:///etc/hosts'));`);
    expect(r).toBe('ERRO: Módulo não permitido: file:///etc/hosts');
  });
});
