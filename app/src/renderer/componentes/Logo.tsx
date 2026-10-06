// 003 (US3): logotipo do ZapDesk — a tecla âmbar com profundidade e o "Z" em raio, mais o
// wordmark "zapdesk". Mesmo desenho de zapdesk-site/src/app/icon.svg (viewBox 32) e do ícone do
// app (scripts/gerar-icones.mjs). Cores só por token (--tecla-sombra, --sinal, --tinta: iguais nos
// dois temas); o wordmark usa --texto. Decorativo: aria-hidden — o texto acessível fica na tela
// (ex.: o <h1>ZapDesk</h1> do Carregando).
import type { CSSProperties } from 'react';

export function Tecla({ tamanho = 28 }: { tamanho?: number }) {
  return (
    <svg viewBox="0 0 32 32" width={tamanho} height={tamanho} aria-hidden="true" focusable="false">
      <rect width="32" height="32" rx="8" style={{ fill: 'var(--tecla-sombra)' }} />
      <rect width="32" height="29" rx="8" style={{ fill: 'var(--sinal)' }} />
      <path
        d="M9 8h14L14.5 14.5H20L9 21h14"
        fill="none"
        style={{ stroke: 'var(--tinta)' }}
        strokeWidth="3"
        strokeLinejoin="miter"
        strokeMiterlimit="10"
      />
    </svg>
  );
}

export interface PropsLogo {
  variante?: 'completo' | 'so-tecla';
  /** Lado da tecla em px (padrão 28); o wordmark acompanha (≈ 0,86 × tamanho). */
  tamanho?: number;
  className?: string;
}

export function Logo({ variante = 'completo', tamanho = 28, className }: PropsLogo) {
  const estilo: CSSProperties = { fontSize: Math.round(tamanho * 0.86) };
  return (
    <span className={`logo${className ? ` ${className}` : ''}`} aria-hidden="true" style={estilo}>
      <Tecla tamanho={tamanho} />
      {variante === 'completo' ? <span className="wordmark">zapdesk</span> : null}
    </span>
  );
}
