// Camada 3 do isolamento (research.md R6): antes de carregar o código do usuário, o runner
// - instala `module.registerHooks` para que o bundle só importe `@zapdesk/automacao` (servido pelo
//   próprio runner) e os módulos Node permitidos na compilação; qualquer outro → "Módulo não
//   permitido: <nome>";
// - troca `process.getBuiltinModule` pela mesma regra;
// - guarda `fetch` para uso interno (ctx.http) e remove `fetch`, `WebSocket`, `EventSource`;
// - neutraliza `process.binding`, `dlopen`, `kill`, `chdir`, `exit`… e deixa `process.env` vazio
//   (`process.reallyExit` fica: o `process.exit` original do runner depende dele).
// Protege contra ERRO e acidente, não contra código malicioso (limite honesto, sdk-automacao.md).
import { registerHooks } from 'node:module';
import { URL_SDK } from './sdk-runtime.js';

/** Módulos Node permitidos às automações (mesma lista do compilador do motor). */
export const MODULOS_PERMITIDOS: readonly string[] = [
  'crypto',
  'util',
  'url',
  'buffer',
  'events',
  'timers/promises',
  'path/posix',
  'string_decoder',
  'querystring',
];

export const NOME_SDK = '@zapdesk/automacao';

/** Erro lançado ao importar um módulo fora da lista. */
export class ErroModuloNaoPermitido extends Error {
  readonly code = 'ERR_MODULO_NAO_PERMITIDO';

  constructor(especificador: string) {
    super(`Módulo não permitido: ${especificador}`);
    this.name = 'ErroModuloNaoPermitido';
  }
}

/** `true` se o especificador (com ou sem `node:`) é um módulo Node permitido. */
export function moduloPermitido(especificador: string): boolean {
  const nome = especificador.startsWith('node:') ? especificador.slice(5) : especificador;
  return MODULOS_PERMITIDOS.includes(nome);
}

export interface Originais {
  fetch: typeof fetch;
  exit: (codigo?: number) => never;
}

export interface OpcoesBloqueios {
  /** URL (file://) do próprio runner: os imports dele não são filtrados. */
  urlRunner: string;
  /** Código-fonte do módulo virtual da SDK (sdk-runtime.ts). */
  fonteSdk: string;
}

let aplicado: Originais | null = null;

/** Aplica os bloqueios (idempotente). Devolve as funções originais que o runner ainda usa. */
export function aplicarBloqueios(op: OpcoesBloqueios): Originais {
  if (aplicado) return aplicado;
  const originais: Originais = {
    fetch: globalThis.fetch.bind(globalThis),
    exit: process.exit.bind(process) as (codigo?: number) => never,
  };

  registerHooks({
    resolve(especificador, contexto, proximo) {
      if (especificador === NOME_SDK) return { url: URL_SDK, format: 'module', shortCircuit: true };
      if (contexto.parentURL === undefined || contexto.parentURL === op.urlRunner) {
        return proximo(especificador, contexto);
      }
      if (moduloPermitido(especificador)) {
        const nome = especificador.startsWith('node:') ? especificador : `node:${especificador}`;
        return proximo(nome, contexto);
      }
      throw new ErroModuloNaoPermitido(especificador);
    },
    load(url, contexto, proximo) {
      if (url === URL_SDK) return { format: 'module', source: op.fonteSdk, shortCircuit: true };
      return proximo(url, contexto);
    },
  });

  const getBuiltin = process.getBuiltinModule?.bind(process);
  definir(process, 'getBuiltinModule', (id: string) => {
    if (!moduloPermitido(id)) throw new ErroModuloNaoPermitido(id);
    return getBuiltin?.(id.startsWith('node:') ? id : `node:${id}`);
  });

  for (const nome of ['fetch', 'WebSocket', 'EventSource']) {
    Reflect.deleteProperty(globalThis, nome);
  }

  for (const nome of [
    'binding',
    '_linkedBinding',
    'dlopen',
    'kill',
    'chdir',
    'exit',
    'abort',
    'setuid',
    'setgid',
    'seteuid',
    'setegid',
    'setgroups',
    'initgroups',
    'umask',
    'loadEnvFile',
  ]) {
    if (nome in process) {
      definir(process, nome, () => {
        throw new Error(`Operação não permitida em automações: process.${nome}`);
      });
    }
  }
  definir(process, 'env', Object.freeze(Object.create(null) as Record<string, string>));

  aplicado = originais;
  return originais;
}

function definir(alvo: object, nome: string, valor: unknown): void {
  try {
    Object.defineProperty(alvo, nome, { value: valor, writable: false, configurable: false, enumerable: false });
  } catch {
    try {
      (alvo as Record<string, unknown>)[nome] = valor;
    } catch {
      // propriedade não configurável: fica como está
    }
  }
}
