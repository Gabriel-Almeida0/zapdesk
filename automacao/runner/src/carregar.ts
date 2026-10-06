// Import do bundle compilado pelo motor e validação da definição (`inicializar`).
import { pathToFileURL } from 'node:url';
import { ehAutomacao, handlersDe, type DefinicaoAutomacao, type NomeHandler } from '@zapdesk/automacao';
import { CODIGOS_ERRO } from './protocolo.js';
import { ErroRpc } from './rpc.js';

export interface AutomacaoCarregada {
  definicao: DefinicaoAutomacao;
  handlers: NomeHandler[];
}

/** Importa o bundle; `2001 bundle_invalido` se falhar ou se o export default não for `definirAutomacao`. */
export async function carregarBundle(caminho: string): Promise<AutomacaoCarregada> {
  let modulo: { default?: unknown };
  try {
    modulo = (await import(pathToFileURL(caminho).href)) as { default?: unknown };
  } catch (erro) {
    const e = erro instanceof Error ? erro : new Error(String(erro));
    throw new ErroRpc(CODIGOS_ERRO.bundle_invalido, `Falha ao carregar a automação: ${e.message}`, {
      codigo: 'bundle_invalido',
      nome: e.name,
      mensagem: e.message,
      stack: e.stack ?? null,
    });
  }
  const definicao = modulo.default;
  if (!ehAutomacao(definicao)) {
    throw new ErroRpc(
      CODIGOS_ERRO.bundle_invalido,
      'O index.ts precisa fazer `export default definirAutomacao({ ... })`.',
      { codigo: 'bundle_invalido', stack: null },
    );
  }
  return { definicao, handlers: handlersDe(definicao) };
}
