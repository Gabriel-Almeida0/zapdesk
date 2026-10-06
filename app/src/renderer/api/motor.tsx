// Provider do `ClienteMotor` + barramento local de eventos do motor.
// Os eventos WS chegam pelo processo principal (IPC) e são repassados aqui para:
// (1) atualizar o cache do TanStack Query (eventos.ts) e (2) telas que precisam de eventos
// pontuais (QR, progresso da sincronização) via `useEventoMotor`.
import { createContext, useContext, useEffect, useMemo, useRef, type ReactNode } from 'react';

import type { ClienteMotor, EventoMotor, MotivoRecarga, ModoWhatsApp } from '@zapdesk/cliente-motor';

export type OuvinteEventoMotor = (evento: EventoMotor) => void;

/** Fonte de eventos (em produção: `window.zapdesk`; nos testes: um emissor manual). */
export interface FonteEventos {
  aoEventoMotor(ouvinte: OuvinteEventoMotor): () => void;
  aoRecarregar(ouvinte: (motivo: MotivoRecarga) => void): () => void;
}

interface ContextoMotor {
  cliente: ClienteMotor;
  whatsapp: ModoWhatsApp;
  versao: string;
  fonte: FonteEventos;
}

const Contexto = createContext<ContextoMotor | null>(null);

export function ProvedorMotor(props: {
  cliente: ClienteMotor;
  fonte: FonteEventos;
  whatsapp?: ModoWhatsApp;
  versao?: string;
  children: ReactNode;
}) {
  const valor = useMemo<ContextoMotor>(
    () => ({
      cliente: props.cliente,
      fonte: props.fonte,
      whatsapp: props.whatsapp ?? 'real',
      versao: props.versao ?? '',
    }),
    [props.cliente, props.fonte, props.whatsapp, props.versao],
  );
  return <Contexto.Provider value={valor}>{props.children}</Contexto.Provider>;
}

function useContextoMotor(): ContextoMotor {
  const ctx = useContext(Contexto);
  if (!ctx) throw new Error('ProvedorMotor ausente.');
  return ctx;
}

export function useCliente(): ClienteMotor {
  return useContextoMotor().cliente;
}

export function useModoMotor(): { whatsapp: ModoWhatsApp; versao: string } {
  const { whatsapp, versao } = useContextoMotor();
  return { whatsapp, versao };
}

/** Assina eventos do motor enquanto o componente estiver montado. */
export function useEventoMotor(ouvinte: OuvinteEventoMotor): void {
  const { fonte } = useContextoMotor();
  const ref = useRef(ouvinte);
  ref.current = ouvinte;
  useEffect(() => fonte.aoEventoMotor((e) => ref.current(e)), [fonte]);
}

/** Emissor simples usado nos testes (e útil para depuração). */
export function criarFonteManual(): FonteEventos & {
  emitir(evento: EventoMotor): void;
  recarregar(motivo?: MotivoRecarga): void;
} {
  const ouvintes = new Set<OuvinteEventoMotor>();
  const recargas = new Set<(m: MotivoRecarga) => void>();
  return {
    aoEventoMotor(o) {
      ouvintes.add(o);
      return () => ouvintes.delete(o);
    },
    aoRecarregar(o) {
      recargas.add(o);
      return () => recargas.delete(o);
    },
    emitir(evento) {
      for (const o of [...ouvintes]) o(evento);
    },
    recarregar(motivo = 'reconexao') {
      for (const o of [...recargas]) o(motivo);
    },
  };
}
