// `definirAutomacao`: marca e valida a definição exportada pelo `index.ts` do projeto.
import { ErroValidacao } from './erros.js';
import type { DefinicaoAutomacao, NomeHandler } from './tipos.js';

/** Handlers aceitos por `definirAutomacao`, na ordem do contrato. */
export const HANDLERS: readonly NomeHandler[] = [
  'aoReceberMensagem',
  'aoAgendar',
  'aoExecutar',
  'aoEvento',
];

/** Símbolo global que marca um objeto criado por `definirAutomacao`. */
export const MARCA_AUTOMACAO: unique symbol = Symbol.for('zapdesk.automacao') as never;

/** Marca e valida (em tempo de execução) a definição. Uso: `export default definirAutomacao({...})`.
 *  Só aceita os handlers `aoReceberMensagem`, `aoAgendar`, `aoExecutar` e `aoEvento`, todos funções. */
export function definirAutomacao<D extends DefinicaoAutomacao>(definicao: D): D {
  if (definicao === null || typeof definicao !== 'object' || Array.isArray(definicao)) {
    throw new ErroValidacao('definirAutomacao espera um objeto com os handlers da automação.');
  }
  const campos: Record<string, string> = {};
  for (const chave of Object.keys(definicao)) {
    if (!(HANDLERS as readonly string[]).includes(chave)) {
      campos[chave] = `Handler desconhecido "${chave}". Use: ${HANDLERS.join(', ')}.`;
    } else if (typeof (definicao as Record<string, unknown>)[chave] !== 'function') {
      campos[chave] = `"${chave}" precisa ser uma função.`;
    }
  }
  const erros = Object.values(campos);
  if (erros.length > 0) {
    throw new ErroValidacao(erros.join(' '), { campos });
  }
  Object.defineProperty(definicao, MARCA_AUTOMACAO, { value: true, enumerable: false });
  return definicao;
}

/** `true` se o valor foi criado por `definirAutomacao` (usado pelo runner). */
export function ehAutomacao(valor: unknown): valor is DefinicaoAutomacao {
  return (
    valor !== null &&
    typeof valor === 'object' &&
    (valor as Record<symbol, unknown>)[MARCA_AUTOMACAO] === true
  );
}

/** Handlers presentes numa definição válida. */
export function handlersDe(definicao: DefinicaoAutomacao): NomeHandler[] {
  return HANDLERS.filter((h) => typeof definicao[h] === 'function');
}
