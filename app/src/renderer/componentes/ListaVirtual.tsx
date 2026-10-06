// Lista virtualizada (@tanstack/react-virtual) com carregamento da próxima página ao chegar perto
// do fim. Sem ResizeObserver (testes em jsdom) renderiza tudo de forma simples.
import { useVirtualizer } from '@tanstack/react-virtual';
import { useEffect, useRef, type ReactNode } from 'react';

export interface PropsListaVirtual<T> {
  itens: T[];
  estimativa: number;
  chave: (item: T) => string;
  renderizar: (item: T, indice: number) => ReactNode;
  aoChegarNoFim?: () => void;
  rotulo: string;
  className?: string;
  rodape?: ReactNode;
  /** Índice a manter visível (navegação por teclado). */
  indiceVisivel?: number;
}

const suportado = typeof ResizeObserver !== 'undefined';

export function ListaVirtual<T>(props: PropsListaVirtual<T>) {
  const { itens, aoChegarNoFim, chave } = props;
  const ref = useRef<HTMLDivElement>(null);
  const virtualizador = useVirtualizer({
    count: itens.length,
    getScrollElement: () => ref.current,
    estimateSize: () => props.estimativa,
    overscan: 10,
    enabled: suportado,
    getItemKey: (i) => {
      const item = itens[i];
      return item === undefined ? i : chave(item);
    },
  });
  const virtuais = virtualizador.getVirtualItems();
  const ultimo = virtuais.at(-1)?.index ?? -1;

  useEffect(() => {
    if (!aoChegarNoFim || itens.length === 0) return;
    if (!suportado || ultimo >= itens.length - 8) aoChegarNoFim();
  }, [ultimo, itens.length, aoChegarNoFim]);

  const { indiceVisivel } = props;
  useEffect(() => {
    if (suportado && indiceVisivel !== undefined && indiceVisivel >= 0) {
      virtualizador.scrollToIndex(indiceVisivel, { align: 'auto' });
    }
  }, [indiceVisivel, virtualizador]);

  const classe = `lista-virtual${props.className ? ` ${props.className}` : ''}`;

  if (!suportado) {
    return (
      <div className={classe} role="list" aria-label={props.rotulo}>
        {itens.map((item, i) => (
          <div role="listitem" key={chave(item)}>
            {props.renderizar(item, i)}
          </div>
        ))}
        {props.rodape}
      </div>
    );
  }

  return (
    <div ref={ref} className={classe} role="list" aria-label={props.rotulo}>
      <div style={{ height: virtualizador.getTotalSize(), position: 'relative', width: '100%' }}>
        {virtuais.map((v) => {
          const item = itens[v.index];
          if (item === undefined) return null;
          return (
            <div
              key={v.key}
              role="listitem"
              data-index={v.index}
              ref={virtualizador.measureElement}
              style={{ position: 'absolute', top: 0, left: 0, width: '100%', transform: `translateY(${v.start}px)` }}
            >
              {props.renderizar(item, v.index)}
            </div>
          );
        })}
      </div>
      {props.rodape}
    </div>
  );
}
