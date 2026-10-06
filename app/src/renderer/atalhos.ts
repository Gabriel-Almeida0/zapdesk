// Atalhos de teclado das conversas (T064): Cmd+F busca, Cmd+↑/↓ troca de conversa, Cmd+N nova.
import { useEffect, useRef } from 'react';

export type Atalho = 'buscar' | 'nova_conversa' | 'conversa_anterior' | 'proxima_conversa';

interface TeclaMinima {
  key: string;
  metaKey: boolean;
  ctrlKey: boolean;
  altKey: boolean;
  shiftKey: boolean;
}

/** No macOS usa Cmd; em outras plataformas (testes/dev) aceita Ctrl. */
export function interpretarAtalho(e: TeclaMinima, mac = true): Atalho | null {
  const modificador = mac ? e.metaKey && !e.ctrlKey : e.ctrlKey && !e.metaKey;
  if (!modificador || e.altKey) return null;
  const tecla = e.key.toLowerCase();
  if (tecla === 'f' && !e.shiftKey) return 'buscar';
  if (tecla === 'n' && !e.shiftKey) return 'nova_conversa';
  if (tecla === 'arrowup') return 'conversa_anterior';
  if (tecla === 'arrowdown') return 'proxima_conversa';
  return null;
}

/** Próximo id numa lista ordenada (sem dar a volta). */
export function vizinho(ids: readonly string[], atual: string | undefined, direcao: 1 | -1): string | null {
  if (ids.length === 0) return null;
  const i = atual ? ids.indexOf(atual) : -1;
  if (i === -1) return direcao === 1 ? (ids[0] ?? null) : (ids[ids.length - 1] ?? null);
  return ids[Math.min(ids.length - 1, Math.max(0, i + direcao))] ?? null;
}

export function useAtalhos(acoes: Partial<Record<Atalho, () => void>>): void {
  const ref = useRef(acoes);
  ref.current = acoes;
  useEffect(() => {
    const mac = navigator.platform.toLowerCase().includes('mac');
    const tecla = (e: KeyboardEvent) => {
      const atalho = interpretarAtalho(e, mac);
      const acao = atalho ? ref.current[atalho] : undefined;
      if (!acao) return;
      e.preventDefault();
      acao();
    };
    window.addEventListener('keydown', tecla);
    return () => window.removeEventListener('keydown', tecla);
  }, []);
}
