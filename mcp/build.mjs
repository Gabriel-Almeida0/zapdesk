// Gera o bundle único do servidor MCP: dist/zapdesk-mcp.mjs (vai para extraResources do app e
// roda com o executável do Electron + ELECTRON_RUN_AS_NODE=1).
import { build } from 'esbuild';

await build({
  entryPoints: ['src/index.ts'],
  outfile: 'dist/zapdesk-mcp.mjs',
  bundle: true,
  platform: 'node',
  format: 'esm',
  target: 'node22',
  sourcemap: false,
  legalComments: 'none',
  // Dependências CommonJS dentro de um bundle ESM precisam de `require`.
  banner: {
    js: "import { createRequire as __criarRequire } from 'node:module'; const require = __criarRequire(import.meta.url);",
  },
  logLevel: 'info',
});
