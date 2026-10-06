import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import type { Fetch, Runtime } from '../src/index.js';
import { lerRuntime, motorVivo, processoVivo } from '../src/runtime.js';

const runtime: Runtime = {
  versao_contrato: 1,
  porta: 51234,
  token: 't'.repeat(43),
  pid_app: 1,
  pid_motor: 2,
  versao: '0.1.0',
  iniciado_em: '2026-09-27T20:10:00-03:00',
};

describe('lerRuntime', () => {
  let pasta: string;
  beforeEach(async () => {
    pasta = await mkdtemp(join(tmpdir(), 'zapdesk-runtime-'));
  });
  afterEach(async () => {
    await rm(pasta, { recursive: true, force: true });
  });

  it('retorna null se o arquivo não existe', async () => {
    expect(await lerRuntime(pasta)).toBeNull();
  });

  it('lê um runtime.json válido', async () => {
    await writeFile(join(pasta, 'runtime.json'), JSON.stringify(runtime));
    expect(await lerRuntime(pasta)).toEqual(runtime);
  });

  it('retorna null para JSON corrompido ou outra versão de contrato', async () => {
    await writeFile(join(pasta, 'runtime.json'), '{quebrado');
    expect(await lerRuntime(pasta)).toBeNull();
    await writeFile(join(pasta, 'runtime.json'), JSON.stringify({ ...runtime, versao_contrato: 2 }));
    expect(await lerRuntime(pasta)).toBeNull();
  });
});

describe('motorVivo', () => {
  const saudeOk = (async () =>
    new Response(JSON.stringify({ ok: true, versao: '0.1.0', whatsapp: 'falso' }), { status: 200 })) as unknown as Fetch;

  it('processo morto → falso sem chamar a API', async () => {
    let chamou = false;
    const fetch = (async () => {
      chamou = true;
      return new Response('{}');
    }) as unknown as Fetch;
    expect(await motorVivo(runtime, { fetch, processoVivo: () => false })).toBe(false);
    expect(chamou).toBe(false);
  });

  it('processo vivo + /v1/saude 200 → verdadeiro', async () => {
    expect(await motorVivo(runtime, { fetch: saudeOk, processoVivo: () => true })).toBe(true);
  });

  it('processo vivo + /v1/saude com erro ou fora do ar → falso', async () => {
    const erro401 = (async () => new Response('', { status: 401 })) as unknown as Fetch;
    const foraDoAr = (async () => {
      throw new TypeError('fetch failed');
    }) as unknown as Fetch;
    expect(await motorVivo(runtime, { fetch: erro401, processoVivo: () => true })).toBe(false);
    expect(await motorVivo(runtime, { fetch: foraDoAr, processoVivo: () => true })).toBe(false);
  });

  it('processoVivo usa kill -0', () => {
    expect(processoVivo(process.pid)).toBe(true);
    expect(processoVivo(-1)).toBe(false);
    expect(processoVivo(2 ** 22 + 12345)).toBe(false);
  });
});
