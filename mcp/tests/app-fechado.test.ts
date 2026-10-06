// T136 — "app fechado": sem runtime.json → o `abrir` injetado sobe o motor → a ferramenta conclui;
// sem subida em 30 s → "Não consegui ligar o ZapDesk".
import { existsSync } from 'node:fs';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import type { Runtime } from '@zapdesk/cliente-motor';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { criarGarantirMotor } from '../src/garantir-motor.js';
import { type MotorReal, conectar, motorRealDisponivel, subirMotorReal } from './harness.js';
import { MotorSimulado, TOKEN_SIMULADO } from './motor-simulado.js';

describe('app fechado (motor simulado)', () => {
  let pasta: string;
  beforeEach(async () => {
    pasta = await mkdtemp(join(tmpdir(), 'zapdesk-fechado-'));
  });
  afterEach(async () => {
    await rm(pasta, { recursive: true, force: true });
  });

  it('abre o app, espera o motor e conclui a ferramenta', async () => {
    const motor = new MotorSimulado();
    motor.adicionarConta({ nome: 'Loja' });
    const runtime: Runtime = {
      versao_contrato: 1,
      porta: 7788,
      token: TOKEN_SIMULADO,
      pid_app: process.pid,
      pid_motor: process.pid,
      versao: '0.1.0',
      iniciado_em: '2026-09-27T20:10:00-03:00',
    };
    const abrirApp = vi.fn(async () => {
      // o app sobe e grava o runtime.json depois de um tempo
      setTimeout(() => void writeFile(join(pasta, 'runtime.json'), JSON.stringify(runtime)), 30);
    });
    const garantir = criarGarantirMotor({
      pastaDados: pasta,
      abrirApp,
      intervaloMs: 10,
      fetch: motor.fetch,
      motorVivo: async () => existsSync(join(pasta, 'runtime.json')),
    });
    const h = await conectar({ obterCliente: garantir });
    const r = await h.chamar('listar_contas');
    expect(abrirApp).toHaveBeenCalledTimes(1);
    expect(r.erro).toBe(false);
    expect(r.texto).toContain('Loja');

    // segunda chamada: app já aberto, não abre de novo
    await h.chamar('status_zapdesk');
    expect(abrirApp).toHaveBeenCalledTimes(1);
    await h.fechar();
  });

  it('sem subida em 30 s → isError "Não consegui ligar o ZapDesk"', async () => {
    let agora = 0;
    const garantir = criarGarantirMotor({
      pastaDados: pasta,
      abrirApp: async () => undefined,
      agora: () => agora,
      esperar: async (ms) => {
        agora += ms;
      },
    });
    const h = await conectar({ obterCliente: garantir });
    const r = await h.chamar('importar_leads', { leads: [{ telefone: '11 99999-0000' }] });
    expect(r.erro).toBe(true);
    expect(r.texto).toContain('Não consegui ligar o ZapDesk. Abra o app manualmente e tente de novo.');
    expect(agora).toBe(30_000);
    await h.fechar();
  });
});

const temMotorReal = await motorRealDisponivel();

describe.skipIf(!temMotorReal)('app fechado (motor real em modo falso)', () => {
  let pasta: string;
  let motor: MotorReal | undefined;
  beforeEach(async () => {
    pasta = await mkdtemp(join(tmpdir(), 'zapdesk-fechado-real-'));
  });
  afterEach(async () => {
    await motor?.encerrar();
    await rm(pasta, { recursive: true, force: true });
  });

  it('abrir injetado sobe o motor real e a ferramenta conclui', async () => {
    const garantir = criarGarantirMotor({
      pastaDados: pasta,
      abrirApp: async () => {
        motor = await subirMotorReal({ pastaDados: pasta });
      },
    });
    const h = await conectar({ obterCliente: garantir });
    const r = await h.chamar('status_zapdesk');
    expect(r.erro).toBe(false);
    expect(r.texto).toContain('WhatsApp falso');
    await h.fechar();
  }, 40_000);
});
