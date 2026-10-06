// T043 — garantirMotor: runtime ausente → abre o app; pid morto; timeout de 30 s com relógio falso.
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import type { Runtime } from '@zapdesk/cliente-motor';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ErroAppFechado, MENSAGEM_NAO_LIGOU, criarGarantirMotor } from '../src/garantir-motor.js';

const runtime: Runtime = {
  versao_contrato: 1,
  porta: 51234,
  token: 't'.repeat(43),
  pid_app: 1,
  pid_motor: 2,
  versao: '0.1.0',
  iniciado_em: '2026-09-27T20:10:00-03:00',
};

/** Relógio falso: `esperar` avança o tempo instantaneamente. */
function relogioFalso() {
  let agora = 0;
  return {
    agora: () => agora,
    esperar: vi.fn(async (ms: number) => {
      agora += ms;
    }),
  };
}

describe('garantirMotor', () => {
  let pasta: string;
  beforeEach(async () => {
    pasta = await mkdtemp(join(tmpdir(), 'zapdesk-garantir-'));
  });
  afterEach(async () => {
    await rm(pasta, { recursive: true, force: true });
  });

  it('usa o runtime.json quando o motor está vivo, sem abrir o app', async () => {
    await writeFile(join(pasta, 'runtime.json'), JSON.stringify(runtime));
    const abrirApp = vi.fn(async () => undefined);
    const garantir = criarGarantirMotor({ pastaDados: pasta, abrirApp, motorVivo: async () => true });
    const cliente = await garantir();
    expect(cliente.porta).toBe(51234);
    expect(cliente.token).toBe(runtime.token);
    expect(abrirApp).not.toHaveBeenCalled();
  });

  it('runtime ausente → chama abrir injetado e espera o runtime aparecer', async () => {
    const relogio = relogioFalso();
    const abrirApp = vi.fn(async () => {
      // o "app" grava o runtime.json um pouco depois de aberto
      setTimeout(() => void writeFile(join(pasta, 'runtime.json'), JSON.stringify(runtime)), 0);
    });
    let sondagens = 0;
    const garantir = criarGarantirMotor({
      pastaDados: pasta,
      abrirApp,
      motorVivo: async () => true,
      agora: relogio.agora,
      esperar: async (ms) => {
        sondagens++;
        await relogio.esperar(ms);
        await new Promise((r) => setTimeout(r, 5));
      },
    });
    const cliente = await garantir();
    expect(abrirApp).toHaveBeenCalledTimes(1);
    expect(cliente.porta).toBe(51234);
    expect(sondagens).toBeGreaterThanOrEqual(1);
    expect(relogio.esperar).toHaveBeenCalledWith(250);
  });

  it('pid do motor morto → considera fechado e abre o app', async () => {
    await writeFile(join(pasta, 'runtime.json'), JSON.stringify({ ...runtime, pid_motor: 999_999_999 }));
    const relogio = relogioFalso();
    let aberto = false;
    const abrirApp = vi.fn(async () => {
      aberto = true;
    });
    const garantir = criarGarantirMotor({
      pastaDados: pasta,
      abrirApp,
      // pid morto até o app abrir; depois o novo motor responde
      motorVivo: async () => aberto,
      ...relogio,
    });
    await garantir();
    expect(abrirApp).toHaveBeenCalledTimes(1);
  });

  it('usa o motorVivo padrão (kill -0) e detecta pid morto', async () => {
    await writeFile(join(pasta, 'runtime.json'), JSON.stringify({ ...runtime, pid_motor: 999_999_999 }));
    const relogio = relogioFalso();
    const abrirApp = vi.fn(async () => undefined);
    const garantir = criarGarantirMotor({ pastaDados: pasta, abrirApp, ...relogio, prazoMs: 1_000 });
    await expect(garantir()).rejects.toBeInstanceOf(ErroAppFechado);
    expect(abrirApp).toHaveBeenCalledTimes(1);
  });

  it('sem subida em 30 s → ErroAppFechado com a mensagem do contrato, sondando a cada 250 ms', async () => {
    const relogio = relogioFalso();
    const abrirApp = vi.fn(async () => undefined);
    const garantir = criarGarantirMotor({ pastaDados: pasta, abrirApp, motorVivo: async () => true, ...relogio });
    const erro = await garantir().catch((e: unknown) => e);
    expect(erro).toBeInstanceOf(ErroAppFechado);
    expect((erro as Error).message).toBe(MENSAGEM_NAO_LIGOU);
    expect(relogio.agora()).toBe(30_000);
    expect(relogio.esperar).toHaveBeenCalledTimes(120);
  });

  it('falha ao abrir o app vira ErroAppFechado com detalhe', async () => {
    const garantir = criarGarantirMotor({
      pastaDados: pasta,
      abrirApp: async () => {
        throw new Error('Unable to find application named ZapDesk');
      },
    });
    const erro = (await garantir().catch((e: unknown) => e)) as ErroAppFechado;
    expect(erro).toBeInstanceOf(ErroAppFechado);
    expect(erro.detalhe).toContain('ZapDesk');
  });

  it('chamadas simultâneas com o app fechado abrem o app uma vez só', async () => {
    let aberto = false;
    const abrirApp = vi.fn(async () => {
      aberto = true;
      await writeFile(join(pasta, 'runtime.json'), JSON.stringify(runtime));
    });
    const garantir = criarGarantirMotor({
      pastaDados: pasta,
      abrirApp,
      motorVivo: async () => aberto,
      esperar: async () => undefined,
    });
    await Promise.all([garantir(), garantir(), garantir()]);
    expect(abrirApp).toHaveBeenCalledTimes(1);
  });
});
