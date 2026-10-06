// Garante que o motor do ZapDesk está no ar antes de cada ferramenta (contracts/runtime.md ›
// "Regras para clientes (MCP)"): lê `runtime.json`, confere o pid e `GET /v1/saude`; se o app
// estiver fechado, abre o app e sonda a cada 250 ms por até 30 s.
import { ClienteMotor, type Fetch, type Runtime } from '@zapdesk/cliente-motor';
import {
  lerRuntime as lerRuntimePadrao,
  motorVivo as motorVivoPadrao,
  pastaDadosPadrao,
} from '@zapdesk/cliente-motor/runtime';

import { abrirAppPadrao } from './abrir-app.js';

export const MENSAGEM_NAO_LIGOU =
  'Não consegui ligar o ZapDesk. Abra o app manualmente e tente de novo.';

/** O app estava fechado e não ficou pronto a tempo (ou não deu para abri-lo). */
export class ErroAppFechado extends Error {
  readonly detalhe: string | undefined;

  constructor(detalhe?: string) {
    super(MENSAGEM_NAO_LIGOU);
    this.name = 'ErroAppFechado';
    this.detalhe = detalhe;
  }
}

export interface OpcoesGarantirMotor {
  /** Pasta onde o app grava `runtime.json`. Padrão: `ZAPDESK_PASTA_DADOS` ou a pasta do macOS. */
  pastaDados?: string;
  /** Abre o app (injetável em testes). Padrão: `abrirAppPadrao`. */
  abrirApp?: () => Promise<void>;
  lerRuntime?: (pastaDados: string) => Promise<Runtime | null>;
  motorVivo?: (runtime: Runtime) => Promise<boolean>;
  /** Relógio injetável (ms). */
  agora?: () => number;
  esperar?: (ms: number) => Promise<void>;
  /** Intervalo entre sondagens. Padrão 250 ms. */
  intervaloMs?: number;
  /** Prazo total de espera após abrir o app. Padrão 30 s. */
  prazoMs?: number;
  /** fetch usado pelo `ClienteMotor` devolvido (testes). */
  fetch?: Fetch;
}

export type GarantirMotor = () => Promise<ClienteMotor>;

const esperarReal = (ms: number) => new Promise<void>((resolver) => setTimeout(resolver, ms));

/**
 * Cria a função `garantirMotor()`. Chamadas simultâneas com o app fechado compartilham a mesma
 * tentativa de abertura (o app é aberto uma única vez).
 */
export function criarGarantirMotor(opcoes: OpcoesGarantirMotor = {}): GarantirMotor {
  const pastaDados = opcoes.pastaDados ?? pastaDadosPadrao();
  const ler = opcoes.lerRuntime ?? lerRuntimePadrao;
  const vivo = opcoes.motorVivo ?? ((runtime: Runtime) => motorVivoPadrao(runtime));
  const abrir = opcoes.abrirApp ?? abrirAppPadrao;
  const agora = opcoes.agora ?? Date.now;
  const esperar = opcoes.esperar ?? esperarReal;
  const intervaloMs = opcoes.intervaloMs ?? 250;
  const prazoMs = opcoes.prazoMs ?? 30_000;

  const cliente = (runtime: Runtime) =>
    new ClienteMotor({
      porta: runtime.porta,
      token: runtime.token,
      ...(opcoes.fetch ? { fetch: opcoes.fetch } : {}),
    });

  const runtimeVivo = async (): Promise<Runtime | null> => {
    const runtime = await ler(pastaDados);
    if (runtime === null) return null;
    return (await vivo(runtime)) ? runtime : null;
  };

  let abrindo: Promise<ClienteMotor> | null = null;

  const abrirEEsperar = async (): Promise<ClienteMotor> => {
    try {
      await abrir();
    } catch (erro) {
      throw new ErroAppFechado(erro instanceof Error ? erro.message : String(erro));
    }
    const inicio = agora();
    while (agora() - inicio < prazoMs) {
      await esperar(intervaloMs);
      const runtime = await runtimeVivo();
      if (runtime) return cliente(runtime);
    }
    throw new ErroAppFechado(`o motor não respondeu em ${Math.round(prazoMs / 1000)} s`);
  };

  return async () => {
    const runtime = await runtimeVivo();
    if (runtime) return cliente(runtime);
    if (!abrindo) {
      abrindo = abrirEEsperar().finally(() => {
        abrindo = null;
      });
    }
    return abrindo;
  };
}
