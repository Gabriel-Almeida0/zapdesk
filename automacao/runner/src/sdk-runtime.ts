// Módulo servido como `@zapdesk/automacao` ao código do usuário. O bundle do usuário importa a SDK
// como externa; o hook de resolução (bloqueios.ts) a mapeia para um módulo virtual cujos exports
// apontam para a MESMA cópia da SDK embutida no runner — assim `instanceof ErroPermissao` e a
// marca de `definirAutomacao` funcionam dos dois lados.
import * as sdk from '@zapdesk/automacao';

/** URL do módulo virtual. */
export const URL_SDK = 'zapdesk:automacao';
const CHAVE_GLOBAL = Symbol.for('zapdesk.sdk.runtime');

/** Publica a SDK num símbolo global (não enumerável) e devolve o código-fonte do módulo virtual. */
export function prepararSdkRuntime(): string {
  const exportado = Object.freeze({ ...sdk });
  if (!(CHAVE_GLOBAL in globalThis)) {
    Object.defineProperty(globalThis, CHAVE_GLOBAL, { value: exportado, enumerable: false, writable: false });
  }
  const nomes = Object.keys(exportado).filter((n) => /^[A-Za-z_$][\w$]*$/.test(n));
  const linhas = [`const s = globalThis[Symbol.for(${JSON.stringify(CHAVE_GLOBAL.description)})];`];
  for (const n of nomes) linhas.push(`export const ${n} = s.${n};`);
  return linhas.join('\n');
}
