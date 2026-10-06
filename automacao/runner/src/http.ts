// `ctx.http.fetch`: exige a permissão `rede`, só http(s), respeita `ctx.sinal` e registra cada
// chamada na notificação `http` (método, URL sem query, status, duração, erro — sem corpo).
import { ErroPermissao, ErroValidacao, type ApiHttp } from '@zapdesk/automacao';
import type { ParamsHttp } from './protocolo.js';

export interface OpcoesHttp {
  execucaoId: string;
  permissoes: readonly string[];
  /** `fetch` guardado antes de o código do usuário ser carregado. */
  fetchInterno: typeof fetch;
  sinal: AbortSignal;
  notificar(params: ParamsHttp): void;
  /** Lança `ErroExecucaoEncerrada` se a execução já terminou. */
  garantirAtiva(): void;
}

export function criarHttp(op: OpcoesHttp): ApiHttp {
  return {
    async fetch(url: string | URL, init?: RequestInit): Promise<Response> {
      op.garantirAtiva();
      if (!op.permissoes.includes('rede')) throw new ErroPermissao('rede');
      let alvo: URL;
      try {
        alvo = new URL(String(url));
      } catch {
        throw new ErroValidacao(`URL inválida: ${String(url).slice(0, 200)}`, { campos: { url: 'inválida' } });
      }
      if (alvo.protocol !== 'http:' && alvo.protocol !== 'https:') {
        throw new ErroValidacao('Só URLs http:// ou https:// são permitidas.', { campos: { url: 'protocolo' } });
      }
      const sinal = init?.signal ? AbortSignal.any([init.signal, op.sinal]) : op.sinal;
      const metodo = (init?.method ?? 'GET').toUpperCase();
      const urlSemQuery = `${alvo.origin}${alvo.pathname}`;
      const inicio = performance.now();
      try {
        const resposta = await op.fetchInterno(alvo, { ...init, signal: sinal });
        op.notificar({
          execucao_id: op.execucaoId,
          metodo,
          url_sem_query: urlSemQuery,
          status: resposta.status,
          duracao_ms: Math.round(performance.now() - inicio),
          erro: null,
        });
        return resposta;
      } catch (erro) {
        op.notificar({
          execucao_id: op.execucaoId,
          metodo,
          url_sem_query: urlSemQuery,
          status: null,
          duracao_ms: Math.round(performance.now() - inicio),
          erro: erro instanceof Error ? `${erro.name}: ${erro.message}`.slice(0, 500) : String(erro).slice(0, 500),
        });
        throw erro;
      }
    },
  };
}
