// Conta selecionada (seletor no topo da lista de conversas). Persistida por visualizador.
import { useQuery } from '@tanstack/react-query';
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';

import type { Conta, Id } from '@zapdesk/cliente-motor';

import { chaves } from '../api/chaves';
import { useCliente } from '../api/motor';

const CHAVE_LOCAL = 'zapdesk.conta';

interface ContextoConta {
  contas: Conta[];
  carregando: boolean;
  erro: unknown;
  recarregar: () => void;
  conta: Conta | null;
  contaId: Id | null;
  selecionar: (id: Id) => void;
}

const Contexto = createContext<ContextoConta | null>(null);

function lerLocal(): string | null {
  try {
    return localStorage.getItem(CHAVE_LOCAL);
  } catch {
    return null;
  }
}

function gravarLocal(id: string): void {
  try {
    localStorage.setItem(CHAVE_LOCAL, id);
  } catch {
    // armazenamento indisponível: só não lembra a escolha
  }
}

export function useContas() {
  const cliente = useCliente();
  return useQuery({ queryKey: chaves.contas, queryFn: () => cliente.listarContas() });
}

export function ProvedorConta({ children }: { children: ReactNode }) {
  const consulta = useContas();
  const [escolhida, setEscolhida] = useState<string | null>(() => lerLocal());
  const contas = useMemo(() => consulta.data ?? [], [consulta.data]);

  // Preferência: a escolhida → a primeira conectada → a primeira.
  const conta = useMemo(
    () =>
      contas.find((c) => c.id === escolhida) ??
      contas.find((c) => c.estado === 'conectada') ??
      contas[0] ??
      null,
    [contas, escolhida],
  );

  useEffect(() => {
    if (conta && conta.id !== escolhida) setEscolhida(conta.id);
  }, [conta, escolhida]);

  const selecionar = useCallback((id: Id) => {
    setEscolhida(id);
    gravarLocal(id);
  }, []);

  const { refetch } = consulta;
  const valor = useMemo<ContextoConta>(
    () => ({
      contas,
      carregando: consulta.isPending,
      erro: consulta.error,
      recarregar: () => void refetch(),
      conta,
      contaId: conta?.id ?? null,
      selecionar,
    }),
    [contas, consulta.isPending, consulta.error, refetch, conta, selecionar],
  );
  return <Contexto.Provider value={valor}>{children}</Contexto.Provider>;
}

export function useContaAtual(): ContextoConta {
  const ctx = useContext(Contexto);
  if (!ctx) throw new Error('ProvedorConta ausente.');
  return ctx;
}

export const ROTULO_ESTADO_CONTA: Record<Conta['estado'], string> = {
  conectando: 'Conectando…',
  conectada: 'Conectada',
  desconectada: 'Desconectada',
  banida: 'Bloqueada pelo WhatsApp',
};
