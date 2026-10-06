// Ponto de entrada do servidor MCP do ZapDesk (stdio).
import { serveStdio } from '@modelcontextprotocol/server/stdio';

import { criarServidor } from './servidor.js';

serveStdio(() => criarServidor(), {
  onerror: (erro) => {
    // stdout é do protocolo MCP; diagnósticos vão para o stderr.
    process.stderr.write(`zapdesk-mcp: ${erro.message}\n`);
  },
});
