// Gera o bundle único do runner: dist/zapdesk-runner.mjs. O motor o executa com o binário do
// Electron + ELECTRON_RUN_AS_NODE=1 (app) ou com `node` do PATH (testes Go), com o Permission
// Model ligado (--allow-fs-read só do runner e do bundle): por isso NADA pode ficar externo —
// o SDK `@zapdesk/automacao` e qualquer código compartilhado vão dentro do bundle.
import { build } from 'esbuild';

await build({
  entryPoints: ['src/index.ts'],
  outfile: 'dist/zapdesk-runner.mjs',
  bundle: true,
  platform: 'node',
  format: 'esm',
  target: 'node22',
  sourcemap: false,
  legalComments: 'none',
  // Só módulos embutidos do Node (node:*) ficam fora do bundle.
  external: ['node:*'],
  logLevel: 'info',
});
