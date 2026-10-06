// Leitura do `runtime.json` e checagem de vida do motor (contracts/runtime.md › runtime.json).
// Só para Node (app/main e mcp/). Importe por `@zapdesk/cliente-motor/runtime` — o ponto de
// entrada principal não inclui este módulo para poder ser usado no renderer (navegador).
import { readFile } from 'node:fs/promises';
import { homedir } from 'node:os';
import { join } from 'node:path';

import { ClienteMotor, type Fetch } from './cliente.js';
import type { Runtime } from './tipos.js';

export const NOME_RUNTIME = 'runtime.json';
export const VERSAO_CONTRATO_RUNTIME = 1;

/** Pasta de dados padrão do macOS (`~/Library/Application Support/ZapDesk`). */
export function pastaDadosPadrao(): string {
  return process.env['ZAPDESK_PASTA_DADOS'] ?? join(homedir(), 'Library', 'Application Support', 'ZapDesk');
}

export function caminhoRuntime(pastaDados: string): string {
  return join(pastaDados, NOME_RUNTIME);
}

function ehRuntime(valor: unknown): valor is Runtime {
  if (valor === null || typeof valor !== 'object') return false;
  const v = valor as Record<string, unknown>;
  return (
    v['versao_contrato'] === VERSAO_CONTRATO_RUNTIME &&
    typeof v['porta'] === 'number' &&
    typeof v['token'] === 'string' &&
    typeof v['pid_motor'] === 'number' &&
    typeof v['pid_app'] === 'number'
  );
}

/**
 * Lê `<pastaDados>/runtime.json`. Retorna `null` se o arquivo não existe, está incompleto,
 * corrompido ou é de outra versão de contrato (app considerado fechado). Nunca escreve.
 */
export async function lerRuntime(pastaDados: string = pastaDadosPadrao()): Promise<Runtime | null> {
  let texto: string;
  try {
    texto = await readFile(caminhoRuntime(pastaDados), 'utf8');
  } catch {
    return null;
  }
  try {
    const valor: unknown = JSON.parse(texto);
    return ehRuntime(valor) ? valor : null;
  } catch {
    return null;
  }
}

/** `kill -0 pid`: verdadeiro se o processo existe (EPERM também conta como vivo). */
export function processoVivo(pid: number): boolean {
  if (!Number.isInteger(pid) || pid <= 0) return false;
  try {
    process.kill(pid, 0);
    return true;
  } catch (erro) {
    return (erro as NodeJS.ErrnoException).code === 'EPERM';
  }
}

export interface OpcoesMotorVivo {
  fetch?: Fetch;
  /** Tempo máximo do `GET /v1/saude`. Padrão 2 s. */
  timeoutMs?: number;
  /** Injetável para testes. */
  processoVivo?: (pid: number) => boolean;
}

/** Motor vivo = `pid_motor` existe **e** `GET /v1/saude` responde 200. */
export async function motorVivo(runtime: Runtime, opcoes: OpcoesMotorVivo = {}): Promise<boolean> {
  const vivo = opcoes.processoVivo ?? processoVivo;
  if (!vivo(runtime.pid_motor)) return false;

  const fetchBase = opcoes.fetch ?? globalThis.fetch.bind(globalThis);
  const timeoutMs = opcoes.timeoutMs ?? 2000;
  const fetchComPrazo: Fetch = (entrada, init) =>
    fetchBase(entrada, { ...init, signal: AbortSignal.timeout(timeoutMs) });

  const cliente = new ClienteMotor({ porta: runtime.porta, token: runtime.token, fetch: fetchComPrazo });
  try {
    const saude = await cliente.saude();
    return saude.ok === true;
  } catch {
    return false;
  }
}

/** Cria um `ClienteMotor` a partir do runtime. */
export function clienteDoRuntime(runtime: Runtime, fetch?: Fetch): ClienteMotor {
  return new ClienteMotor({ porta: runtime.porta, token: runtime.token, ...(fetch ? { fetch } : {}) });
}
