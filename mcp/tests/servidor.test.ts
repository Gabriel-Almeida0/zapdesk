import { Client } from '@modelcontextprotocol/client';
import { InMemoryTransport } from '@modelcontextprotocol/server';
import { describe, expect, it } from 'vitest';

import { NOME_SERVIDOR, criarServidor } from '../src/servidor.js';

describe('servidor MCP (esqueleto)', () => {
  it('conecta por InMemoryTransport e se identifica como zapdesk', async () => {
    const [transporteCliente, transporteServidor] = InMemoryTransport.createLinkedPair();
    const servidor = criarServidor();
    await servidor.connect(transporteServidor);

    const cliente = new Client({ name: 'teste', version: '0.0.0' });
    await cliente.connect(transporteCliente);

    expect(cliente.getServerVersion()?.name).toBe(NOME_SERVIDOR);

    await cliente.close();
    await servidor.close();
  });
});
