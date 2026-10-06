// Contexto do canvas: os nós da definição, erros por nó e o nó atual do chat simulado.
import { createContext, useContext } from 'react';

import type { ErroDefinicao, NoChatbot } from '@zapdesk/cliente-motor';

export interface ContextoCanvas {
  nos: Map<string, NoChatbot>;
  erros: Map<string, ErroDefinicao[]>;
  noAtualSimulacao: string | null;
  inicio: string;
}

export const ContextoCanvasBot = createContext<ContextoCanvas>({ nos: new Map(), erros: new Map(), noAtualSimulacao: null, inicio: '' });

export function useCanvasBot(): ContextoCanvas {
  return useContext(ContextoCanvasBot);
}
