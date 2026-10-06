// Botão só com ícone: o rótulo acessível é obrigatório (vira aria-label e dica).
import type { ButtonHTMLAttributes, ReactNode } from 'react';

export interface PropsBotaoIcone extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'aria-label' | 'children'> {
  rotulo: string;
  children: ReactNode;
  ativo?: boolean;
}

export function BotaoIcone({ rotulo, children, ativo, className, type, ...resto }: PropsBotaoIcone) {
  return (
    <button
      type={type ?? 'button'}
      aria-label={rotulo}
      title={rotulo}
      aria-pressed={ativo}
      className={`botao-icone${ativo ? ' ativo' : ''}${className ? ` ${className}` : ''}`}
      {...resto}
    >
      {children}
    </button>
  );
}
