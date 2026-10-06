// `<pasta-dados>/runtime.json` (contracts/runtime.md): escrito pelo app com permissão 0600 depois
// do `pronto` do motor e apagado ao encerrar. O MCP só lê.
import { rmSync } from 'node:fs';
import { chmod, mkdir, rename, rm, writeFile } from 'node:fs/promises';
import { join } from 'node:path';

import type { Runtime } from '@zapdesk/cliente-motor';

export const NOME_RUNTIME = 'runtime.json';

export function caminhoRuntime(pastaDados: string): string {
  return join(pastaDados, NOME_RUNTIME);
}

/** Data RFC 3339 com o fuso local (ex.: 2026-09-27T20:10:00-03:00). */
export function dataLocalRfc3339(data: Date = new Date()): string {
  const d2 = (n: number): string => String(Math.trunc(Math.abs(n))).padStart(2, '0');
  const deslocamento = -data.getTimezoneOffset();
  const sinal = deslocamento >= 0 ? '+' : '-';
  return (
    `${data.getFullYear()}-${d2(data.getMonth() + 1)}-${d2(data.getDate())}` +
    `T${d2(data.getHours())}:${d2(data.getMinutes())}:${d2(data.getSeconds())}` +
    `${sinal}${d2(deslocamento / 60)}:${d2(deslocamento % 60)}`
  );
}

export interface DadosRuntime {
  porta: number;
  token: string;
  pidApp: number;
  pidMotor: number;
  versao: string;
  iniciadoEm?: Date;
}

export function montarRuntime(dados: DadosRuntime): Runtime {
  return {
    versao_contrato: 1,
    porta: dados.porta,
    token: dados.token,
    pid_app: dados.pidApp,
    pid_motor: dados.pidMotor,
    versao: dados.versao,
    iniciado_em: dataLocalRfc3339(dados.iniciadoEm),
  };
}

/**
 * Grava de forma atômica: arquivo temporário criado já com 0600 → chmod (ignora umask) → rename.
 * O token nunca fica legível por outros usuários, nem por um instante.
 */
export async function gravarRuntime(pastaDados: string, dados: DadosRuntime): Promise<string> {
  await mkdir(pastaDados, { recursive: true, mode: 0o700 });
  const destino = caminhoRuntime(pastaDados);
  const temporario = `${destino}.${process.pid}.tmp`;
  await writeFile(temporario, `${JSON.stringify(montarRuntime(dados), null, 2)}\n`, {
    mode: 0o600,
    flag: 'w',
  });
  await chmod(temporario, 0o600);
  await rename(temporario, destino);
  await chmod(destino, 0o600);
  return destino;
}

export async function apagarRuntime(pastaDados: string): Promise<void> {
  await rm(caminhoRuntime(pastaDados), { force: true });
}

/** Versão síncrona para a saída de emergência (`process.on('exit')`). */
export function apagarRuntimeSync(pastaDados: string): void {
  try {
    rmSync(caminhoRuntime(pastaDados), { force: true });
  } catch {
    // nada a fazer na saída
  }
}

