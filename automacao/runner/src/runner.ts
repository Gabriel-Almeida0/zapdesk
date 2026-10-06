// Laço de métodos do runner (lado runner de runner-protocolo.md): `inicializar`, `executar`,
// `cancelar`, `ping`, `encerrar`. Independente de stdin/stdout — index.ts liga ao processo e os
// testes ligam a uma conexão em memória.
import type { DefinicaoAutomacao, NomeHandler } from '@zapdesk/automacao';
import { carregarBundle, type AutomacaoCarregada } from './carregar.js';
import { execucaoAtual } from './console.js';
import { criarContexto, type ControleExecucao } from './contexto.js';
import {
  CODIGOS_ERRO,
  CODIGOS_RPC,
  VERSAO_PROTOCOLO,
  type ParamsCancelar,
  type ParamsExecutar,
  type ParamsHttp,
  type ParamsInicializar,
  type ParamsLog,
  type ResultadoExecutar,
  type ResultadoInicializar,
} from './protocolo.js';
import { ErroRpc } from './rpc.js';

/** Tamanho máximo do retorno de `aoExecutar` (JSON). */
export const LIMITE_RETORNO = 64 * 1024;
const HANDLERS: readonly NomeHandler[] = ['aoReceberMensagem', 'aoAgendar', 'aoExecutar', 'aoEvento'];

export interface OpcoesRunner {
  requisitar(metodo: string, params: unknown): Promise<unknown>;
  notificar(metodo: 'log', params: ParamsLog): void;
  notificar(metodo: 'http', params: ParamsHttp): void;
  fetchInterno: typeof fetch;
  /** Chamado depois de responder `encerrar`. */
  aoEncerrar(): void;
  /** Filtro de linhas do stack (remove quadros internos do runner). */
  limparStack?(stack: string): string;
  carregar?(caminho: string): Promise<AutomacaoCarregada>;
  agora?: () => number;
}

export interface Runner {
  tratarRequisicao(metodo: string, params: unknown): Promise<unknown> | undefined;
  tratarNotificacao(metodo: string, params: unknown): void;
  /** Execuções em andamento (testes). */
  readonly execucoes: ReadonlyMap<string, ControleExecucao>;
}

interface Estado {
  automacao: ParamsInicializar['automacao'];
  permissoes: string[];
  segredos: Record<string, string>;
  definicao: DefinicaoAutomacao;
}

export function criarRunner(op: OpcoesRunner): Runner {
  let estado: Estado | null = null;
  let inicializando: Promise<ResultadoInicializar> | null = null;
  const execucoes = new Map<string, ControleExecucao>();
  const carregar = op.carregar ?? carregarBundle;
  const limparStack = op.limparStack ?? ((s: string) => s);

  async function inicializar(p: ParamsInicializar): Promise<ResultadoInicializar> {
    if (!p || typeof p.bundle !== 'string' || !p.automacao) {
      throw new ErroRpc(CODIGOS_RPC.parametros_invalidos, 'Parâmetros de inicializar inválidos.');
    }
    if (p.protocolo !== VERSAO_PROTOCOLO) {
      throw new ErroRpc(CODIGOS_RPC.parametros_invalidos, `Protocolo ${p.protocolo} não suportado (runner: ${VERSAO_PROTOCOLO}).`);
    }
    if (estado || inicializando) {
      throw new ErroRpc(CODIGOS_ERRO.bundle_invalido, 'Runner já inicializado.', { codigo: 'bundle_invalido' });
    }
    inicializando = (async () => {
      const carregada = await carregar(p.bundle);
      estado = {
        automacao: p.automacao,
        permissoes: Array.isArray(p.permissoes) ? p.permissoes : [],
        segredos: Object.freeze({ ...(p.segredos ?? {}) }) as Record<string, string>,
        definicao: carregada.definicao,
      };
      return { handlers: carregada.handlers };
    })();
    try {
      return await inicializando;
    } finally {
      inicializando = null;
    }
  }

  async function executar(p: ParamsExecutar): Promise<ResultadoExecutar> {
    if (!estado) throw new ErroRpc(CODIGOS_RPC.requisicao_invalida, 'Runner não inicializado.');
    if (!p || typeof p.execucao_id !== 'string' || !HANDLERS.includes(p.handler)) {
      throw new ErroRpc(CODIGOS_RPC.parametros_invalidos, 'Parâmetros de executar inválidos.');
    }
    if (execucoes.has(p.execucao_id)) {
      throw new ErroRpc(CODIGOS_RPC.parametros_invalidos, `Execução ${p.execucao_id} já está em andamento.`);
    }
    const handler = estado.definicao[p.handler] as ((...a: unknown[]) => unknown) | undefined;
    if (typeof handler !== 'function') {
      throw new ErroRpc(CODIGOS_ERRO.handler_ausente, `Handler ${p.handler} não exportado.`, {
        codigo: 'handler_ausente',
      });
    }
    const controle = criarContexto({
      rpc: { requisitar: op.requisitar },
      execucao: p,
      automacao: estado.automacao,
      permissoes: estado.permissoes,
      segredos: estado.segredos,
      fetchInterno: op.fetchInterno,
      notificar: op.notificar as never,
      agora: op.agora,
    });
    execucoes.set(p.execucao_id, controle);
    const definicao = estado.definicao;
    try {
      const retorno = await execucaoAtual.run({ id: p.execucao_id }, async () =>
        handler.call(definicao, controle.ctx, controle.argumento),
      );
      return { retorno: p.handler === 'aoExecutar' ? validarRetorno(retorno) : null };
    } catch (erro) {
      if (erro instanceof ErroRpc) throw erro;
      throw erroUsuario(erro, limparStack);
    } finally {
      controle.encerrar();
      execucoes.delete(p.execucao_id);
    }
  }

  return {
    execucoes,
    tratarRequisicao(metodo, params) {
      switch (metodo) {
        case 'inicializar':
          return inicializar(params as ParamsInicializar);
        case 'executar':
          return executar(params as ParamsExecutar);
        case 'ping':
          return Promise.resolve({ ok: true });
        case 'encerrar':
          for (const c of execucoes.values()) c.cancelar('O processo da automação foi encerrado.');
          setImmediate(() => op.aoEncerrar());
          return Promise.resolve({});
        default:
          return undefined;
      }
    },
    tratarNotificacao(metodo, params) {
      if (metodo === 'cancelar') {
        const id = (params as ParamsCancelar | undefined)?.execucao_id;
        if (typeof id === 'string') execucoes.get(id)?.cancelar();
      }
    },
  };
}

/** Garante que o retorno é JSON (≤ 64 KB); `undefined` vira `null`. */
function validarRetorno(valor: unknown): unknown {
  if (valor === undefined) return null;
  let json: string | undefined;
  try {
    json = JSON.stringify(valor);
  } catch (erro) {
    throw new ErroRpc(CODIGOS_ERRO.erro_usuario, 'O retorno de aoExecutar não é JSON válido.', {
      codigo: 'erro_usuario',
      nome: 'ErroValidacao',
      mensagem: `O retorno de aoExecutar não é JSON válido: ${erro instanceof Error ? erro.message : String(erro)}`,
      stack: null,
    });
  }
  if (json === undefined) return null; // função, símbolo…
  if (Buffer.byteLength(json) > LIMITE_RETORNO) {
    throw new ErroRpc(CODIGOS_ERRO.erro_usuario, 'O retorno de aoExecutar é maior que 64 KB.', {
      codigo: 'erro_usuario',
      nome: 'ErroValidacao',
      mensagem: 'O retorno de aoExecutar é maior que 64 KB.',
      stack: null,
    });
  }
  return JSON.parse(json) as unknown;
}

function erroUsuario(erro: unknown, limparStack: (s: string) => string): ErroRpc {
  const e = erro instanceof Error ? erro : null;
  const nome = e?.name ?? 'Error';
  const mensagem = e ? e.message : typeof erro === 'string' ? erro : safeString(erro);
  const stack = e?.stack ? limparStack(e.stack) : null;
  const dados: Record<string, unknown> = { codigo: 'erro_usuario', nome, mensagem, stack };
  const codigoSdk = (e as { codigo?: unknown } | null)?.codigo;
  if (typeof codigoSdk === 'string') dados.codigo_sdk = codigoSdk;
  return new ErroRpc(CODIGOS_ERRO.erro_usuario, `${nome}: ${mensagem}`, dados);
}

function safeString(v: unknown): string {
  try {
    return typeof v === 'object' ? JSON.stringify(v) : String(v);
  } catch {
    return String(v);
  }
}
