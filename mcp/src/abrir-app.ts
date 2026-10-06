// Como abrir o ZapDesk quando ele está fechado (contracts/runtime.md › regra 2).
//
// Ordem de decisão:
// 1. `ZAPDESK_COMANDO_ABRIR` definido → roda esse comando no shell (útil em desenvolvimento).
// 2. Este script está fora de um `.app` (rodando de `mcp/dist/` no repositório) → `npm run dev`
//    na raiz do monorepo.
// 3. App empacotado → `open -b com.gabriel.zapdesk`, com fallback `open -a ZapDesk`.
import { execFile, spawn } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

export const BUNDLE_ID = 'com.gabriel.zapdesk';
export const NOME_APP = 'ZapDesk';

/** Ambiente para processos filhos: sem `ELECTRON_RUN_AS_NODE`, senão o Electron vira Node. */
function ambienteFilho(): NodeJS.ProcessEnv {
  const env = { ...process.env };
  delete env['ELECTRON_RUN_AS_NODE'];
  // Garante `node`/`npm` no PATH mesmo quando o cliente MCP inicia com PATH mínimo.
  env['PATH'] = [dirname(process.execPath), env['PATH'] ?? '/usr/bin:/bin'].join(':');
  return env;
}

function executar(comando: string, argumentos: string[]): Promise<void> {
  return new Promise((resolver, rejeitar) => {
    execFile(comando, argumentos, { env: ambienteFilho(), timeout: 10_000 }, (erro, _saida, saidaErro) => {
      if (erro) rejeitar(new Error(saidaErro.trim() || erro.message));
      else resolver();
    });
  });
}

/** Inicia um processo desanexado (sobrevive ao fim do servidor MCP). */
function iniciarDesanexado(comando: string, argumentos: string[], cwd?: string): void {
  const filho = spawn(comando, argumentos, {
    cwd,
    env: ambienteFilho(),
    detached: true,
    stdio: 'ignore',
  });
  filho.on('error', (erro) => process.stderr.write(`zapdesk-mcp: falha ao abrir o app: ${erro.message}\n`));
  filho.unref();
}

/** Raiz do monorepo (package.json com `"name": "zapdesk"`) acima deste arquivo, se houver. */
export function raizRepositorio(inicio: string = dirname(fileURLToPath(import.meta.url))): string | null {
  if (inicio.includes('.app/Contents/')) return null;
  let atual = inicio;
  for (;;) {
    const pacote = join(atual, 'package.json');
    if (existsSync(pacote)) {
      try {
        const conteudo = JSON.parse(readFileSync(pacote, 'utf8')) as { name?: unknown };
        if (conteudo.name === 'zapdesk') return atual;
      } catch {
        // package.json ilegível: continua subindo
      }
    }
    const pai = dirname(atual);
    if (pai === atual) return null;
    atual = pai;
  }
}

export async function abrirAppPadrao(): Promise<void> {
  const comando = process.env['ZAPDESK_COMANDO_ABRIR'];
  if (comando && comando.trim().length > 0) {
    iniciarDesanexado('/bin/sh', ['-c', comando]);
    return;
  }

  const raiz = raizRepositorio();
  if (raiz) {
    process.stderr.write(`zapdesk-mcp: app fechado; iniciando "npm run dev" em ${raiz}\n`);
    iniciarDesanexado('npm', ['run', 'dev'], raiz);
    return;
  }

  try {
    await executar('open', ['-b', BUNDLE_ID]);
  } catch {
    await executar('open', ['-a', NOME_APP]);
  }
}
