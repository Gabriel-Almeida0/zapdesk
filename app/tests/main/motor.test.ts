// T036 — motor com binário simulado (script Node) e runtime.json 0600.
import { mkdtempSync, readFileSync, statSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { afterEach, describe, expect, it } from 'vitest';

import { gerarToken, montarArgs, ProcessoMotor, SupervisorMotor, type MotorLigado } from '../../src/main/motor';
import { apagarRuntime, caminhoRuntime, gravarRuntime } from '../../src/main/runtime-json';

const pasta = mkdtempSync(join(tmpdir(), 'zapdesk-teste-motor-'));

/** Cria um "motor" simulado em Node com o comportamento pedido. */
function scriptMotor(nome: string, corpo: string): string {
  const caminho = join(pasta, `${nome}.cjs`);
  writeFileSync(caminho, corpo);
  return caminho;
}

const PRONTO = `
const token = process.env.ZAPDESK_TOKEN || '';
if (token.length < 32) { console.log(JSON.stringify({evento:'erro_fatal',codigo:'token_ausente',mensagem:'x'})); process.exit(2); }
console.error('log no stderr');
process.on('SIGTERM', () => { console.log(JSON.stringify({evento:'encerrando',motivo:'sinal'})); process.exit(0); });
console.log(JSON.stringify({evento:'pronto',porta:51234,versao:'0.1.0',pid:process.pid,whatsapp:'falso', args: process.argv.slice(2)}));
setInterval(() => {}, 1000);
`;

// O handler vem ANTES do "pronto": sob carga, o parar() do teste chegava antes de o handler existir
// e o SIGTERM padrão matava o processo na hora (teste intermitente "expected 3 >= 250").
const IGNORA_SIGTERM = `
process.on('SIGTERM', () => {});
console.log(JSON.stringify({evento:'pronto',porta:1,versao:'0.1.0',pid:process.pid,whatsapp:'real'}));
setInterval(() => {}, 1000);
`;

const MUDO = `setInterval(() => {}, 1000);`;

const FATAL = `console.log(JSON.stringify({evento:'erro_fatal',codigo:'porta_ocupada',mensagem:'porta em uso'})); process.exit(1);`;

const vivos: ProcessoMotor[] = [];
afterEach(async () => {
  await Promise.all(vivos.splice(0).map((m) => m.parar()));
});

function iniciar(script: string, extra: Partial<Parameters<typeof ProcessoMotor.iniciar>[0]> = {}) {
  return ProcessoMotor.iniciar({
    binario: process.execPath,
    argsIniciais: [script],
    pastaDados: pasta,
    env: { ...process.env, ELECTRON_RUN_AS_NODE: '1' },
    ...extra,
  });
}

describe('ProcessoMotor', () => {
  it('gera token base64url de 32 bytes', () => {
    const token = gerarToken();
    expect(token).toMatch(/^[A-Za-z0-9_-]{43}$/);
    expect(gerarToken()).not.toBe(token);
  });

  it('monta as flags do contrato (token nunca vai por argumento)', () => {
    const args = montarArgs({ binario: 'x', pastaDados: '/dados', falso: true, religado: true, token: 'segredo' });
    expect(args).toEqual(['--pasta-dados', '/dados', '--whatsapp=falso', '--religado']);
    expect(args.join(' ')).not.toContain('segredo');
  });

  it('resolve no "pronto" com porta, versão e pid; token só pelo ambiente', async () => {
    const motor = await iniciar(scriptMotor('pronto', PRONTO), { falso: true });
    vivos.push(motor);
    expect(motor.info.porta).toBe(51234);
    expect(motor.info.versao).toBe('0.1.0');
    expect(motor.info.whatsapp).toBe('falso');
    expect(motor.pid).toBeGreaterThan(0);
    expect(motor.token.length).toBeGreaterThanOrEqual(32);
    expect(motor.vivo).toBe(true);
  });

  it('rejeita com timeout se o "pronto" não chega e não deixa processo', async () => {
    await expect(iniciar(scriptMotor('mudo', MUDO), { timeoutProntoMs: 400 })).rejects.toMatchObject({
      codigo: 'timeout',
    });
  });

  it('rejeita com o detalhe do erro_fatal', async () => {
    await expect(iniciar(scriptMotor('fatal', FATAL))).rejects.toMatchObject({
      codigo: 'porta_ocupada',
      detalhe: 'A porta local escolhida já está em uso.',
    });
  });

  it('rejeita quando o binário não existe', async () => {
    await expect(
      ProcessoMotor.iniciar({ binario: join(pasta, 'nao-existe'), pastaDados: pasta }),
    ).rejects.toMatchObject({ codigo: 'binario_ausente' });
  });

  it('parar(): SIGTERM encerra; saída marcada como esperada', async () => {
    const motor = await iniciar(scriptMotor('pronto2', PRONTO));
    const saida = new Promise<boolean>((r) => motor.on('saida', (_c, _s, esperada) => r(esperada)));
    await motor.parar();
    expect(await saida).toBe(true);
    expect(motor.vivo).toBe(false);
  });

  it('parar(): SIGKILL depois do prazo quando o motor ignora SIGTERM', async () => {
    const motor = await iniciar(scriptMotor('teimoso', IGNORA_SIGTERM), { prazoEncerrarMs: 300 });
    const inicio = Date.now();
    await motor.parar();
    expect(Date.now() - inicio).toBeGreaterThanOrEqual(250);
    expect(motor.vivo).toBe(false);
  });

  it('queda inesperada emite saida com esperada=false', async () => {
    const motor = await iniciar(scriptMotor('pronto3', PRONTO));
    const saida = new Promise<boolean>((r) => motor.on('saida', (_c, _s, esperada) => r(esperada)));
    process.kill(motor.pid, 'SIGKILL');
    expect(await saida).toBe(false);
  });
});

describe('runtime.json', () => {
  it('grava com permissão 0600 no formato do contrato e apaga ao encerrar', async () => {
    const dir = mkdtempSync(join(tmpdir(), 'zapdesk-teste-runtime-'));
    const caminho = await gravarRuntime(dir, {
      porta: 51234,
      token: 'a'.repeat(43),
      pidApp: 100,
      pidMotor: 200,
      versao: '0.1.0',
      iniciadoEm: new Date('2026-09-27T23:10:00Z'),
    });
    expect(caminho).toBe(caminhoRuntime(dir));
    expect(statSync(caminho).mode & 0o777).toBe(0o600);
    const conteudo = JSON.parse(readFileSync(caminho, 'utf8')) as Record<string, unknown>;
    expect(conteudo).toMatchObject({
      versao_contrato: 1,
      porta: 51234,
      token: 'a'.repeat(43),
      pid_app: 100,
      pid_motor: 200,
      versao: '0.1.0',
    });
    expect(String(conteudo['iniciado_em'])).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{2}:\d{2}$/);
    await apagarRuntime(dir);
    expect(() => statSync(caminho)).toThrow();
  });

  it('regravar mantém 0600 mesmo com umask permissiva', async () => {
    const dir = mkdtempSync(join(tmpdir(), 'zapdesk-teste-runtime-'));
    const anterior = process.umask(0o000);
    try {
      await gravarRuntime(dir, { porta: 1, token: 't', pidApp: 1, pidMotor: 2, versao: 'x' });
      await gravarRuntime(dir, { porta: 2, token: 't', pidApp: 1, pidMotor: 2, versao: 'x' });
    } finally {
      process.umask(anterior);
    }
    expect(statSync(caminhoRuntime(dir)).mode & 0o777).toBe(0o600);
  });
});

// T065 — religar uma vez após queda.
describe('SupervisorMotor', () => {
  class MotorFalso implements MotorLigado {
    readonly token = 't'.repeat(43);
    readonly info;
    private ouvinte: ((c: number | null, s: NodeJS.Signals | null, e: boolean) => void) | null = null;
    constructor(porta: number) {
      this.info = { porta, versao: '0.1.0', pid: 1000 + porta, whatsapp: 'falso' as const };
    }
    on(_evento: 'saida', ouvinte: (c: number | null, s: NodeJS.Signals | null, e: boolean) => void) {
      this.ouvinte = ouvinte;
      return this;
    }
    cair() {
      this.ouvinte?.(null, 'SIGKILL', false);
    }
    parar() {
      this.ouvinte?.(0, null, true);
      return Promise.resolve();
    }
    matarAgora() {}
  }

  it('liga, religa uma vez com --religado e na segunda queda mostra erro', async () => {
    const chamadas: boolean[] = [];
    const motores: MotorFalso[] = [];
    const supervisor = new SupervisorMotor({
      iniciar: (religado) => {
        chamadas.push(religado);
        const m = new MotorFalso(motores.length + 1);
        motores.push(m);
        return Promise.resolve(m);
      },
    });
    const fases: string[] = [];
    supervisor.on('estado', (e) => fases.push(e.fase));

    await supervisor.ligar();
    expect(supervisor.estado.fase).toBe('pronto');

    motores[0]?.cair();
    await new Promise((r) => setTimeout(r, 0));
    expect(chamadas).toEqual([false, true]);
    expect(supervisor.conexao?.porta).toBe(2);

    motores[1]?.cair();
    await new Promise((r) => setTimeout(r, 0));
    expect(chamadas).toEqual([false, true]);
    expect(supervisor.estado).toMatchObject({ fase: 'erro', mensagem: 'Não consegui ligar o WhatsApp.' });
    expect(fases).toEqual(['ligando', 'pronto', 'religando', 'pronto', 'erro']);

    // "Tentar de novo" liga do zero e volta a permitir um religamento.
    await supervisor.reiniciar();
    expect(supervisor.estado.fase).toBe('pronto');
    expect(chamadas).toEqual([false, true, false]);
  });

  it('parar() não religa', async () => {
    const m = new MotorFalso(1);
    let vezes = 0;
    const supervisor = new SupervisorMotor({
      iniciar: () => {
        vezes += 1;
        return Promise.resolve(m);
      },
    });
    await supervisor.ligar();
    await supervisor.parar();
    expect(vezes).toBe(1);
  });

  it('falha ao iniciar vira estado de erro com detalhe', async () => {
    const supervisor = new SupervisorMotor({ iniciar: () => Promise.reject(new Error('porta ocupada')) });
    await supervisor.ligar();
    expect(supervisor.estado).toEqual({
      fase: 'erro',
      mensagem: 'Não consegui ligar o WhatsApp.',
      detalhe: 'porta ocupada',
    });
  });
});

// T054 — flags do runner/segredos e linha `segredos` no stdin após o `pronto`.
describe('automações (002)', () => {
  const ECO_STDIN = `
const fs = require('node:fs');
const destino = process.argv[2];
console.log(JSON.stringify({evento:'pronto',porta:1,versao:'0.1.0',pid:process.pid,whatsapp:'falso'}));
let buffer = '';
process.stdin.on('data', (b) => {
  buffer += b.toString();
  fs.writeFileSync(destino, buffer);
});
process.on('SIGTERM', () => process.exit(0));
setInterval(() => {}, 1000);
`;

  it('monta --runner-exec/--runner-script (só juntos) e --aguardar-segredos', () => {
    expect(
      montarArgs({ binario: 'x', pastaDados: '/d', runnerExec: '/Electron', runnerScript: '/r.mjs', aguardarSegredos: true }),
    ).toEqual(['--pasta-dados', '/d', '--runner-exec', '/Electron', '--runner-script', '/r.mjs', '--aguardar-segredos']);
    expect(montarArgs({ binario: 'x', pastaDados: '/d', runnerExec: '/Electron' })).toEqual(['--pasta-dados', '/d']);
  });

  it('enviarControle escreve uma linha JSON no stdin do motor', async () => {
    const destino = join(pasta, 'stdin.txt');
    const motor = await ProcessoMotor.iniciar({
      binario: process.execPath,
      argsIniciais: [scriptMotor('eco', ECO_STDIN), destino],
      pastaDados: pasta,
    });
    vivos.push(motor);
    expect(motor.enviarControle({ comando: 'segredos', valores: { ANTHROPIC_API_KEY: 'sk-ant-x' } })).toBe(true);
    let texto = '';
    for (let i = 0; i < 50 && !texto.endsWith('\n'); i++) {
      await new Promise((r) => setTimeout(r, 20));
      try {
        texto = readFileSync(destino, 'utf8');
      } catch {
        // ainda não escreveu
      }
    }
    expect(JSON.parse(texto.trim())).toEqual({ comando: 'segredos', valores: { ANTHROPIC_API_KEY: 'sk-ant-x' } });
    await motor.parar();
    expect(motor.enviarControle({ comando: 'segredos', valores: {} })).toBe(false);
  });

  it('supervisor envia os segredos logo após cada pronto (mesmo vazios) e a cada alteração', async () => {
    const linhas: unknown[] = [];
    const motorFalso: MotorLigado = {
      token: 't',
      info: { porta: 1, versao: '0.1.0', pid: 1, whatsapp: 'falso' },
      on: () => undefined,
      parar: () => Promise.resolve(),
      matarAgora: () => undefined,
      enviarControle: (l) => {
        linhas.push(l);
        return true;
      },
    };
    let valores: Record<string, string> = {};
    const supervisor = new SupervisorMotor({ iniciar: () => Promise.resolve(motorFalso), segredos: () => valores });
    let linhasNoPronto = -1;
    supervisor.on('estado', (e) => {
      if (e.fase === 'pronto') linhasNoPronto = linhas.length;
    });
    await supervisor.ligar();
    expect(linhasNoPronto).toBe(1);
    expect(linhas).toEqual([{ comando: 'segredos', valores: {} }]);
    valores = { OPENAI_KEY: 'x' };
    expect(supervisor.enviarSegredos(valores)).toBe(true);
    expect(linhas.at(-1)).toEqual({ comando: 'segredos', valores: { OPENAI_KEY: 'x' } });
  });
});
