// Redireciona `console.*` e `process.stdout.write` para notificações `log` do protocolo: o
// stdout do runner é exclusivo do JSON-RPC. A execução corrente vem do AsyncLocalStorage
// (várias execuções concorrentes no mesmo processo); fora de execução as linhas são descartadas
// (contrato da SDK).
import { AsyncLocalStorage } from 'node:async_hooks';
import { inspect } from 'node:util';
import type { NivelLog } from './protocolo.js';

/** Tamanho máximo do texto de uma notificação `log`. */
export const LIMITE_TEXTO_LOG = 8 * 1024;

/** Execução corrente (para atribuir logs de `console.*`). */
export const execucaoAtual = new AsyncLocalStorage<{ id: string }>();

/** Formata as partes como `console.log` (objetos via `util.inspect`, profundidade 4) e corta em 8 KB. */
export function formatarPartes(partes: readonly unknown[]): string {
  const texto = partes
    .map((p) => (typeof p === 'string' ? p : inspect(p, { depth: 4, breakLength: Infinity, compact: true, colors: false })))
    .join(' ');
  return cortar(texto, LIMITE_TEXTO_LOG);
}

/** Corta o texto para caber em `limite` bytes UTF-8 (acrescenta "…"). */
export function cortar(texto: string, limite: number): string {
  if (Buffer.byteLength(texto) <= limite) return texto;
  const buf = Buffer.from(texto).subarray(0, limite - 3);
  // remove um possível caractere cortado ao meio (U+FFFD no fim)
  return `${buf.toString('utf8').replace(/�+$/, '')}…`;
}

export type EmitirLog = (execucaoId: string, nivel: NivelLog, texto: string) => void;

/** Substitui `console.*` e `process.stdout.write`. Devolve a escrita original do stdout. */
export function instalarConsole(emitir: EmitirLog): (linha: string, cb?: () => void) => boolean {
  const escritaOriginal = process.stdout.write.bind(process.stdout) as (
    dados: string,
    cb?: () => void,
  ) => boolean;

  const registrar = (nivel: NivelLog) => (...partes: unknown[]) => {
    const atual = execucaoAtual.getStore();
    if (!atual) return;
    emitir(atual.id, nivel, formatarPartes(partes));
  };

  const c = console as unknown as Record<string, unknown>;
  c.log = registrar('info');
  c.info = registrar('info');
  c.debug = registrar('debug');
  c.trace = registrar('debug');
  c.warn = registrar('aviso');
  c.error = registrar('erro');
  c.dir = (obj: unknown) => registrar('info')(obj);
  c.table = (obj: unknown) => registrar('info')(obj);

  const falsoWrite = (dados: unknown, ...resto: unknown[]): boolean => {
    const atual = execucaoAtual.getStore();
    const texto = typeof dados === 'string' ? dados : Buffer.isBuffer(dados) ? dados.toString('utf8') : String(dados);
    if (atual && texto.trim().length > 0) emitir(atual.id, 'info', cortar(texto.replace(/\n$/, ''), LIMITE_TEXTO_LOG));
    const cb = resto.find((r) => typeof r === 'function') as (() => void) | undefined;
    if (cb) queueMicrotask(cb);
    return true;
  };
  (process.stdout as unknown as { write: unknown }).write = falsoWrite;

  return escritaOriginal;
}
