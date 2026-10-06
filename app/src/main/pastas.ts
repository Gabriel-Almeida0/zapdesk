// Regras de caminhos do processo principal (sem Electron, testáveis).
import { mkdtempSync } from 'node:fs';
import { isAbsolute, join, relative, resolve } from 'node:path';

/**
 * 002: caminho da pasta de uma automação de IA, só se estiver DENTRO de `<pasta-dados>/automacoes/`
 * (aceita absoluto ou relativo à pasta de dados). `null` = recusado.
 */
export function pastaAutomacaoPermitida(caminho: unknown, pastaDados: string): string | null {
  if (typeof caminho !== 'string' || caminho.length === 0 || caminho.includes('\0')) return null;
  const raiz = resolve(pastaDados, 'automacoes');
  const alvo = isAbsolute(caminho) ? resolve(caminho) : resolve(pastaDados, caminho);
  const rel = relative(raiz, alvo);
  if (!rel || rel.startsWith('..') || isAbsolute(rel)) return null;
  return alvo;
}

/**
 * 003: pasta `userData` do Chromium (preferências da janela, localStorage, trava de instância
 * única). `ZAPDESK_PASTA_INTERFACE` tem prioridade; no modo falso sem a variável usa uma pasta
 * temporária nova (para a instância de teste não disputar a trava nem ler a pasta do app
 * instalado); no modo real devolve `padrao` sem mudança.
 */
export function resolverPastaInterface(opcoes: {
  env: Record<string, string | undefined>;
  modoFalso: boolean;
  padrao: string;
  tmp: string;
  criarTemporaria?: (prefixo: string) => string;
}): string {
  const definida = opcoes.env['ZAPDESK_PASTA_INTERFACE'];
  if (definida) return definida;
  if (opcoes.modoFalso) {
    const criar = opcoes.criarTemporaria ?? ((prefixo: string) => mkdtempSync(prefixo));
    return criar(join(opcoes.tmp, 'zapdesk-interface-'));
  }
  return opcoes.padrao;
}
