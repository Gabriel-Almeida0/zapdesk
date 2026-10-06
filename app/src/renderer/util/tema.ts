// 003: preferência de tema da JANELA (Sistema / Claro / Escuro) — data-model §3, research R6.
// Estado de apresentação, como tamanho e posição: fica no localStorage do renderer (userData do
// Electron), não vai ao motor nem ao MCP (plan.md → Complexity Tracking).
//   'sistema' → sem chave no localStorage e sem `data-tema` no <html> (segue o macOS ao vivo)
//   'claro' | 'escuro' → chave gravada e `data-tema` no <html> (tema.css tem os blocos)
import { useSyncExternalStore } from 'react';

export type PreferenciaTema = 'sistema' | 'claro' | 'escuro';
export type TemaEfetivo = 'claro' | 'escuro';

export const CHAVE_TEMA = 'zapdesk.tema';
/** Evento disparado em `window` quando a preferência muda (para o hook reagir sem recarregar). */
export const EVENTO_TEMA = 'zapdesk:tema';
const CONSULTA_ESCURO = '(prefers-color-scheme: dark)';

function valida(v: unknown): v is PreferenciaTema {
  return v === 'sistema' || v === 'claro' || v === 'escuro';
}

/** Preferência salva; valor ausente, inválido ou localStorage indisponível = 'sistema'. */
export function lerPreferencia(): PreferenciaTema {
  try {
    const v = window.localStorage.getItem(CHAVE_TEMA);
    return valida(v) ? v : 'sistema';
  } catch {
    return 'sistema';
  }
}

/** Grava ('claro'/'escuro') ou apaga ('sistema') a chave. Falha de armazenamento não quebra. */
export function salvarPreferencia(preferencia: PreferenciaTema): void {
  const p: PreferenciaTema = valida(preferencia) ? preferencia : 'sistema';
  try {
    if (p === 'sistema') window.localStorage.removeItem(CHAVE_TEMA);
    else window.localStorage.setItem(CHAVE_TEMA, p);
  } catch {
    // sem armazenamento (modo privado/erro): a escolha vale só nesta sessão
  }
}

/** Põe/remove `data-tema` no <html> e avisa os ouvintes (useTemaEfetivo). Troca imediata. */
export function aplicarTema(preferencia: PreferenciaTema): void {
  const raiz = document.documentElement;
  if (preferencia === 'claro' || preferencia === 'escuro') raiz.setAttribute('data-tema', preferencia);
  else raiz.removeAttribute('data-tema');
  window.dispatchEvent(new Event(EVENTO_TEMA));
}

function sistemaEscuro(): boolean {
  try {
    return typeof window.matchMedia === 'function' && window.matchMedia(CONSULTA_ESCURO).matches;
  } catch {
    return false;
  }
}

/** Tema que está valendo agora: o atributo manual, senão o do sistema. */
export function temaEfetivo(): TemaEfetivo {
  const manual = typeof document !== 'undefined' ? document.documentElement.getAttribute('data-tema') : null;
  if (manual === 'claro' || manual === 'escuro') return manual;
  return sistemaEscuro() ? 'escuro' : 'claro';
}

function assinar(aoMudar: () => void): () => void {
  window.addEventListener(EVENTO_TEMA, aoMudar);
  let consulta: MediaQueryList | null = null;
  try {
    consulta = typeof window.matchMedia === 'function' ? window.matchMedia(CONSULTA_ESCURO) : null;
  } catch {
    consulta = null;
  }
  consulta?.addEventListener?.('change', aoMudar);
  return () => {
    window.removeEventListener(EVENTO_TEMA, aoMudar);
    consulta?.removeEventListener?.('change', aoMudar);
  };
}

/** Hook: tema efetivo, atualizado ao vivo (troca no macOS ou em Ajustes → Aparência). */
export function useTemaEfetivo(): TemaEfetivo {
  return useSyncExternalStore(assinar, temaEfetivo, () => 'claro');
}
