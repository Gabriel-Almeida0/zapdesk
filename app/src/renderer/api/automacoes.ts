// Hooks base das automações e do funil (T057). As listas se atualizam pelos eventos WS
// (api/eventos.ts); aqui ficam só as consultas.
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';

import type { EstadoExecucao, FiltroAutomacoes, Id } from '@zapdesk/cliente-motor';

import { chaves } from './chaves';
import { useCliente } from './motor';

export function useFunis() {
  const cliente = useCliente();
  return useQuery({ queryKey: chaves.funis, queryFn: () => cliente.listarFunis() });
}

export function useFunil(id: Id | undefined) {
  const cliente = useCliente();
  return useQuery({
    queryKey: chaves.funil(id ?? ''),
    queryFn: () => cliente.obterFunil(id ?? ''),
    enabled: Boolean(id),
  });
}

export function useAutomacoes(filtro: FiltroAutomacoes = {}) {
  const cliente = useCliente();
  return useQuery({
    queryKey: [...chaves.automacoes, filtro],
    queryFn: () => cliente.listarAutomacoes(filtro),
  });
}

export function useAutomacao(id: Id | undefined) {
  const cliente = useCliente();
  return useQuery({
    queryKey: chaves.automacao(id ?? ''),
    queryFn: () => cliente.obterAutomacao(id ?? ''),
    enabled: Boolean(id),
  });
}

const TAMANHO_PAGINA_EXECUCOES = 50;

/** Execuções de uma automação (ou de todas, com `automacaoId = null`), mais recentes primeiro. */
export function useExecucoes(automacaoId: Id | null, estado?: EstadoExecucao) {
  const cliente = useCliente();
  return useInfiniteQuery({
    queryKey: chaves.execucoesAutomacao(automacaoId, estado),
    queryFn: ({ pageParam }) => {
      const filtro = { limite: TAMANHO_PAGINA_EXECUCOES, ...(estado ? { estado } : {}), ...(pageParam ? { cursor: pageParam } : {}) };
      return automacaoId ? cliente.listarExecucoesAutomacao(automacaoId, filtro) : cliente.listarExecucoes(filtro);
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (pagina) => pagina.proximo_cursor ?? undefined,
  });
}

export function useExecucao(id: Id | undefined) {
  const cliente = useCliente();
  return useQuery({
    queryKey: chaves.execucao(id ?? ''),
    queryFn: () => cliente.obterExecucao(id ?? ''),
    enabled: Boolean(id),
  });
}

export function useEstadoConversaAutomacoes(conversaId: Id | undefined) {
  const cliente = useCliente();
  return useQuery({
    queryKey: chaves.estadoConversaAutomacoes(conversaId ?? ''),
    queryFn: () => cliente.estadoAutomacoesConversa(conversaId ?? ''),
    enabled: Boolean(conversaId),
    // A pausa vence sozinha no motor (sem evento): reconsulta de vez em quando.
    refetchInterval: 60_000,
  });
}

export function useConfiguracaoAutomacoes() {
  const cliente = useCliente();
  return useQuery({ queryKey: chaves.configuracaoAutomacoes, queryFn: () => cliente.obterConfiguracaoAutomacoes() });
}
