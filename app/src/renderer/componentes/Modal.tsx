// Diálogo modal simples (Esc fecha; foco vai para o primeiro campo).
import { X } from 'lucide-react';
import { useEffect, useRef, type ReactNode } from 'react';

import { BotaoIcone } from './BotaoIcone';

export function Modal(props: { titulo: string; aoFechar: () => void; children: ReactNode; largura?: number; rodape?: ReactNode }) {
  const ref = useRef<HTMLDivElement>(null);
  const { aoFechar } = props;
  useEffect(() => {
    const tecla = (e: KeyboardEvent) => {
      if (e.key === 'Escape') aoFechar();
    };
    window.addEventListener('keydown', tecla);
    const campo = ref.current?.querySelector<HTMLElement>('input, textarea, select, button.botao');
    campo?.focus();
    return () => window.removeEventListener('keydown', tecla);
  }, [aoFechar]);
  return (
    <div className="modal-fundo" onMouseDown={(e) => e.target === e.currentTarget && aoFechar()}>
      <div className="modal" role="dialog" aria-modal="true" aria-label={props.titulo} ref={ref} style={{ width: props.largura ?? 440 }}>
        <header className="modal-cabecalho">
          <h2>{props.titulo}</h2>
          <BotaoIcone rotulo="Fechar" onClick={aoFechar}>
            <X size={20} />
          </BotaoIcone>
        </header>
        <div className="modal-corpo">{props.children}</div>
        {props.rodape ? <footer className="modal-rodape">{props.rodape}</footer> : null}
      </div>
    </div>
  );
}

/** Confirmação com botão de perigo. */
export function Confirmar(props: {
  titulo: string;
  texto: ReactNode;
  confirmar: string;
  /** Rótulo do botão que fecha sem fazer nada (padrão "Cancelar"). */
  recusar?: string;
  perigo?: boolean;
  ocupado?: boolean;
  aoConfirmar: () => void;
  aoFechar: () => void;
}) {
  return (
    <Modal
      titulo={props.titulo}
      aoFechar={props.aoFechar}
      rodape={
        <>
          <button type="button" className="botao secundario" onClick={props.aoFechar}>
            {props.recusar ?? 'Cancelar'}
          </button>
          <button
            type="button"
            className={`botao${props.perigo ? ' perigo' : ''}`}
            disabled={props.ocupado}
            onClick={props.aoConfirmar}
          >
            {props.confirmar}
          </button>
        </>
      }
    >
      <p className="texto-modal">{props.texto}</p>
    </Modal>
  );
}
