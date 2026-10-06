// Posição de um erro de compilação do motor no editor. O motor devolve linha e coluna 1-base
// (contracts/api-http.md › ErroCompilacao), a mesma base do Monaco: nada de somar 1.
import type { ErroCompilacao } from '@zapdesk/cliente-motor';

export function posicaoNoEditor(e: Pick<ErroCompilacao, 'linha' | 'coluna'>): { linha: number; coluna: number } {
  return { linha: Math.max(1, e.linha), coluna: Math.max(1, e.coluna) };
}
