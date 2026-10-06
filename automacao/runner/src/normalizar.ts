// Conversões entre o formato do protocolo (snake_case) e o da SDK (camelCase).
//
// Decisão (registrada no relatório da feature): o contrato diz que `info` e `argumento` de
// `executar` já chegam em camelCase ("formato da SDK"), mas o resto do protocolo é snake_case.
// Para não quebrar se o motor mandar snake_case, o runner converte as chaves conhecidas nas duas
// grafias. Valores livres do usuário (memória, campos de lead, dados de agendamento, esquemas,
// entrada de `aoExecutar`) nunca são convertidos.

/** Chaves cujo valor é dado livre (não tem as chaves convertidas). */
const CHAVES_LIVRES = new Set(['valor', 'campos', 'dados', 'esquema', 'variaveis', 'entrada', 'retorno']);

export function snakeParaCamel(chave: string): string {
  return chave.replace(/_([a-z0-9])/g, (_, c: string) => c.toUpperCase());
}

export function camelParaSnake(chave: string): string {
  return chave.replace(/[A-Z]/g, (c) => `_${c.toLowerCase()}`);
}

/** Converte recursivamente as chaves snake_case para camelCase, sem entrar em dados livres. */
export function camelizar<T = unknown>(valor: unknown): T {
  return converter(valor, snakeParaCamel) as T;
}

/** Converte recursivamente as chaves camelCase para snake_case, sem entrar em dados livres. */
export function snakeizar<T = unknown>(valor: unknown): T {
  return converter(valor, camelParaSnake) as T;
}

function converter(valor: unknown, f: (c: string) => string): unknown {
  if (Array.isArray(valor)) return valor.map((v) => converter(v, f));
  if (valor === null || typeof valor !== 'object') return valor;
  if (Object.getPrototypeOf(valor) !== Object.prototype && Object.getPrototypeOf(valor) !== null) {
    return valor;
  }
  const saida: Record<string, unknown> = {};
  for (const [chave, v] of Object.entries(valor)) {
    const nova = f(chave);
    saida[nova] = CHAVES_LIVRES.has(chave) || CHAVES_LIVRES.has(nova) ? v : converter(v, f);
  }
  return saida;
}

/** Remove chaves com valor `undefined` (não viajam no JSON de qualquer forma). */
export function semIndefinidos<T extends Record<string, unknown>>(obj: T): T {
  const saida: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(obj)) if (v !== undefined) saida[k] = v;
  return saida as T;
}
