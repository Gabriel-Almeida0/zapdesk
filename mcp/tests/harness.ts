// Harness de testes do MCP (T042):
// - `criarHarness()`: servidor MCP + `Client` ligados por `InMemoryTransport.createLinkedPair()`,
//   falando com o motor simulado (motor-simulado.ts) ou com qualquer `obterCliente`.
// - `subirMotorReal()`: compila/usa `motor/bin/zapdesk-motor`, sobe com `--whatsapp=falso
//   --sem-stdin` (+ `--ia=falsa` e o runner, quando o motor já os suporta — T059) numa pasta temporária com token, espera a linha `pronto` e grava um
//   `runtime.json` de teste (como o app faria).
import { type ChildProcess, spawn, spawnSync } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { existsSync } from 'node:fs';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { createInterface } from 'node:readline';
import { fileURLToPath } from 'node:url';

import { Client } from '@modelcontextprotocol/client';
import { InMemoryTransport } from '@modelcontextprotocol/server';
import { ClienteMotor, type Runtime } from '@zapdesk/cliente-motor';

import { type OpcoesServidor, criarServidor } from '../src/servidor.js';
import { MotorSimulado } from './motor-simulado.js';

export const RAIZ_REPOSITORIO = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');
export const BINARIO_MOTOR = join(RAIZ_REPOSITORIO, 'motor', 'bin', 'zapdesk-motor');
export const SCRIPT_RUNNER = join(RAIZ_REPOSITORIO, 'automacao', 'runner', 'dist', 'zapdesk-runner.mjs');

let flagsAutomacoes: string[] | undefined;

/**
 * Flags da 002 (specs/002-automacoes/contracts/runtime.md): `--ia=falsa` e, com o runner
 * compilado, `--runner-exec node --runner-script …`. Só são passadas se o binário já as conhece
 * (o motor está em construção em paralelo; flag desconhecida faria o motor não subir).
 */
export function flagsAutomacoesMotor(): string[] {
  if (flagsAutomacoes) return flagsAutomacoes;
  flagsAutomacoes = [];
  if (!existsSync(BINARIO_MOTOR)) return flagsAutomacoes;
  const ajuda = spawnSync(BINARIO_MOTOR, ['-h'], { encoding: 'utf8', timeout: 5_000 });
  const texto = `${ajuda.stdout ?? ''}${ajuda.stderr ?? ''}`;
  if (/^\s+-ia\b/m.test(texto)) flagsAutomacoes.push('--ia=falsa');
  if (/runner-exec/.test(texto) && existsSync(SCRIPT_RUNNER)) {
    flagsAutomacoes.push('--runner-exec', process.execPath, '--runner-script', SCRIPT_RUNNER);
  }
  return flagsAutomacoes;
}

export interface ResultadoChamada {
  texto: string;
  estruturado: Record<string, unknown> | undefined;
  erro: boolean;
}

export interface Harness {
  motor: MotorSimulado;
  cliente: Client;
  chamar: (nome: string, argumentos?: Record<string, unknown>) => Promise<ResultadoChamada>;
  fechar: () => Promise<void>;
}

export async function conectar(opcoes: OpcoesServidor): Promise<Omit<Harness, 'motor'>> {
  const [transporteCliente, transporteServidor] = InMemoryTransport.createLinkedPair();
  const servidor = criarServidor(opcoes);
  await servidor.connect(transporteServidor);
  const cliente = new Client({ name: 'teste-zapdesk', version: '0.0.0' });
  await cliente.connect(transporteCliente);

  const chamar = async (nome: string, argumentos: Record<string, unknown> = {}): Promise<ResultadoChamada> => {
    const resultado = await cliente.callTool({ name: nome, arguments: argumentos });
    const conteudo = (resultado.content ?? []) as { type: string; text?: string }[];
    return {
      texto: conteudo
        .filter((c) => c.type === 'text')
        .map((c) => c.text ?? '')
        .join('\n'),
      estruturado: resultado.structuredContent as Record<string, unknown> | undefined,
      erro: resultado.isError === true,
    };
  };

  return {
    cliente,
    chamar,
    fechar: async () => {
      await cliente.close();
      await servidor.close();
    },
  };
}

/** Servidor MCP ligado ao motor simulado. */
export async function criarHarness(opcoes: Partial<OpcoesServidor> = {}): Promise<Harness> {
  const motor = new MotorSimulado();
  const conexao = await conectar({ obterCliente: async () => motor.cliente(), ...opcoes });
  return { motor, ...conexao };
}

// ---------------------------------------------------------------------------
// Motor real em modo falso
// ---------------------------------------------------------------------------

export interface MotorReal {
  porta: number;
  token: string;
  pid: number;
  pastaDados: string;
  runtime: Runtime;
  cliente: ClienteMotor;
  processo: ChildProcess;
  /** Grava `<pastaDados>/runtime.json` (o app faz isso em produção). */
  gravarRuntime: () => Promise<void>;
  encerrar: () => Promise<void>;
}

export async function subirMotorReal(
  opcoes: { pastaDados?: string; gravarRuntime?: boolean; prazoMs?: number } = {},
): Promise<MotorReal> {
  const pastaDados = opcoes.pastaDados ?? (await mkdtemp(join(tmpdir(), 'zapdesk-mcp-teste-')));
  const token = randomBytes(32).toString('base64url');
  const processo = spawn(
    BINARIO_MOTOR,
    [
      '--whatsapp=falso',
      '--sem-stdin',
      '--pasta-dados',
      pastaDados,
      '--porta',
      '0',
      '--nivel-log',
      'erro',
      ...flagsAutomacoesMotor(),
    ],
    { env: { ...process.env, ZAPDESK_TOKEN: token }, stdio: ['pipe', 'pipe', 'pipe'] },
  );
  let saidaErro = '';
  processo.stderr?.on('data', (parte: Buffer) => {
    saidaErro += parte.toString();
  });

  const pronto = await new Promise<{ porta: number; pid: number; versao: string }>((resolver, rejeitar) => {
    const prazo = setTimeout(() => {
      processo.kill('SIGKILL');
      rejeitar(new Error(`motor não emitiu "pronto" a tempo. stderr: ${saidaErro.slice(-500)}`));
    }, opcoes.prazoMs ?? 10_000);
    processo.once('exit', (codigo) => {
      clearTimeout(prazo);
      rejeitar(new Error(`motor saiu com código ${codigo}. stderr: ${saidaErro.slice(-500)}`));
    });
    const linhas = createInterface({ input: processo.stdout! });
    linhas.on('line', (linha) => {
      try {
        const msg = JSON.parse(linha) as { evento?: string; porta?: number; pid?: number; versao?: string; mensagem?: string };
        if (msg.evento === 'pronto' && msg.porta) {
          clearTimeout(prazo);
          resolver({ porta: msg.porta, pid: msg.pid ?? processo.pid ?? 0, versao: msg.versao ?? '0.0.0' });
        } else if (msg.evento === 'erro_fatal') {
          clearTimeout(prazo);
          rejeitar(new Error(`erro_fatal do motor: ${msg.mensagem ?? linha}`));
        }
      } catch {
        // linha não-JSON: ignora
      }
    });
  });

  const runtime: Runtime = {
    versao_contrato: 1,
    porta: pronto.porta,
    token,
    pid_app: process.pid,
    pid_motor: pronto.pid,
    versao: pronto.versao,
    iniciado_em: new Date().toISOString(),
  };
  const gravarRuntime = () =>
    writeFile(join(pastaDados, 'runtime.json'), JSON.stringify(runtime), { mode: 0o600 });
  if (opcoes.gravarRuntime !== false) await gravarRuntime();

  const encerrar = async () => {
    if (processo.exitCode === null) {
      const saiu = new Promise<void>((r) => processo.once('exit', () => r()));
      processo.kill('SIGTERM');
      const prazo = setTimeout(() => processo.kill('SIGKILL'), 5_000);
      await saiu;
      clearTimeout(prazo);
    }
    if (!opcoes.pastaDados) await rm(pastaDados, { recursive: true, force: true });
  };

  return {
    porta: pronto.porta,
    token,
    pid: pronto.pid,
    pastaDados,
    runtime,
    cliente: new ClienteMotor({ porta: pronto.porta, token }),
    processo,
    gravarRuntime,
    encerrar,
  };
}

/**
 * O motor real já suporta o necessário para os testes de integração? (binário existe, sobe em
 * modo falso, responde `/v1/saude`, `/v1/falso/enviadas` e a `sonda` passada — rotas que o
 * teste usa). Enquanto o motor estiver em
 * construção, os testes de integração são pulados.
 */
export async function motorRealDisponivel(
  sonda: (cliente: ClienteMotor) => Promise<unknown> = (c) => c.sistema(),
): Promise<boolean> {
  if (process.env['ZAPDESK_PULAR_INTEGRACAO'] === '1' || !existsSync(BINARIO_MOTOR)) return false;
  let motor: MotorReal | undefined;
  try {
    motor = await subirMotorReal({ prazoMs: 5_000, gravarRuntime: false });
    const saude = await motor.cliente.saude();
    await motor.cliente.falso.enviadas();
    await sonda(motor.cliente);
    return saude.ok === true;
  } catch {
    return false;
  } finally {
    await motor?.encerrar().catch(() => undefined);
  }
}
