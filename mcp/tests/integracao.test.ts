// Integração real via stdio: o bundle `dist/zapdesk-mcp.mjs` como processo filho (igual ao Claude
// Code/Desktop), falando com o motor Go real em `--whatsapp=falso`, achado pelo `runtime.json`.
// Pulado se o bundle não foi compilado (`npm run compilar -w @zapdesk/mcp`) ou se o motor ainda
// não sobe em modo falso (`npm run motor:compilar`).
import { existsSync } from 'node:fs';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { Client } from '@modelcontextprotocol/client';
import { StdioClientTransport, getDefaultEnvironment } from '@modelcontextprotocol/client/stdio';
import type { Conta, Conversa, Disparo, Mensagem, RelatorioImportacao } from '@zapdesk/cliente-motor';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';

import { type MotorReal, RAIZ_REPOSITORIO, motorRealDisponivel, subirMotorReal } from './harness.js';

const BUNDLE = join(RAIZ_REPOSITORIO, 'mcp', 'dist', 'zapdesk-mcp.mjs');
const temBundle = existsSync(BUNDLE);
// Cada bloco só roda quando o motor já implementa as rotas que ele usa.
const temChat =
  temBundle &&
  (await motorRealDisponivel(async (c) => {
    const conta = await c.criarConta({ nome: 'sonda' });
    await c.listarConversas(conta.id, { limite: 1 });
  }));
const temLeadsEDisparos =
  temChat &&
  (await motorRealDisponivel(async (c) => {
    await c.listarLeads({ limite: 1 });
    await c.listarDisparos({ limite: 1 });
  }));

async function conectarStdio(pastaDados: string): Promise<Client> {
  const transporte = new StdioClientTransport({
    command: process.execPath,
    args: [BUNDLE],
    env: {
      ...getDefaultEnvironment(),
      ZAPDESK_PASTA_DADOS: pastaDados,
      // nunca abrir o app de verdade durante o teste
      ZAPDESK_COMANDO_ABRIR: 'true',
    },
    stderr: 'pipe',
  });
  const cliente = new Client({ name: 'teste-integracao', version: '0.0.0' });
  await cliente.connect(transporte);
  return cliente;
}

async function chamar(cliente: Client, nome: string, argumentos: Record<string, unknown> = {}) {
  const r = await cliente.callTool({ name: nome, arguments: argumentos }, { timeout: 60_000 });
  const texto = ((r.content ?? []) as { type: string; text?: string }[]).map((c) => c.text ?? '').join('\n');
  return { texto, estruturado: r.structuredContent as Record<string, unknown> | undefined, erro: r.isError === true };
}

describe.skipIf(!temBundle)('bundle via stdio', () => {
  it('sobe pelo stdio e lista as 69 ferramentas', async () => {
    const pasta = await mkdtemp(join(tmpdir(), 'zapdesk-stdio-'));
    const cliente = await conectarStdio(pasta);
    const { tools } = await cliente.listTools();
    expect(tools).toHaveLength(69);
    expect(cliente.getServerVersion()?.name).toBe('zapdesk');
    await cliente.close();
    await rm(pasta, { recursive: true, force: true });
  });
});

interface Ambiente {
  motor: MotorReal;
  cliente: Client;
  conta: Conta;
  pasta: string;
}

/** Motor real numa pasta temporária + conta falsa conectada + MCP via stdio. */
async function prepararAmbiente(): Promise<Ambiente> {
  const pasta = await mkdtemp(join(tmpdir(), 'zapdesk-integracao-'));
  const motor = await subirMotorReal({ pastaDados: pasta });
  let conta = await motor.cliente.criarConta({ nome: 'Loja' });
  await motor.cliente.falso.escanearQr(conta.id, { telefone: '+5511900000001', nome: 'Loja' });
  for (let i = 0; i < 40; i++) {
    conta = await motor.cliente.obterConta(conta.id);
    if (conta.estado === 'conectada') break;
    await new Promise((r) => setTimeout(r, 100));
  }
  const cliente = await conectarStdio(pasta);
  return { motor, cliente, conta, pasta };
}

async function desmontar(a: Ambiente | undefined): Promise<void> {
  await a?.cliente.close();
  await a?.motor.encerrar();
  if (a) await rm(a.pasta, { recursive: true, force: true });
}

describe.skipIf(!temChat)('integração real (stdio + motor falso): contas e conversas', () => {
  let a: Ambiente;
  beforeAll(async () => {
    a = await prepararAmbiente();
  }, 30_000);
  afterAll(() => desmontar(a));

  it('a conta falsa está conectada', async () => {
    const r = await chamar(a.cliente, 'listar_contas');
    expect(r.erro).toBe(false);
    expect((r.estruturado?.['contas'] as Conta[])[0]?.estado).toBe('conectada');
  });

  it('lê mensagens recebidas e responde', async () => {
    await a.motor.cliente.falso.mensagemRecebida(a.conta.id, { de: '+5511970000001', texto: 'Quero saber mais' });
    let conversas: Conversa[] = [];
    for (let i = 0; i < 30 && conversas.length === 0; i++) {
      const r = await chamar(a.cliente, 'listar_conversas', { nao_lidas: true });
      conversas = (r.estruturado?.['itens'] as Conversa[] | undefined) ?? [];
      if (conversas.length === 0) await new Promise((res) => setTimeout(res, 100));
    }
    expect(conversas.length).toBeGreaterThan(0);

    const ler = await chamar(a.cliente, 'ler_mensagens', { telefone: '(11) 97000-0001' });
    expect(ler.erro, ler.texto).toBe(false);
    const mensagens = ler.estruturado?.['mensagens'] as Mensagem[];
    expect(mensagens.some((m) => m.texto === 'Quero saber mais' && !m.de_mim)).toBe(true);
    expect(ler.texto).toContain('Quero saber mais');

    const enviar = await chamar(a.cliente, 'enviar_mensagem', { telefone: '+55 11 97000-0001', texto: 'Claro! Te mando agora.' });
    expect(enviar.erro, enviar.texto).toBe(false);

    const buscar = await chamar(a.cliente, 'buscar_mensagens', { termo: 'saber' });
    expect(buscar.erro, buscar.texto).toBe(false);
  }, 30_000);
});

describe.skipIf(!temLeadsEDisparos)('integração real (stdio + motor falso): leads e disparos', () => {
  let a: Ambiente;
  beforeAll(async () => {
    a = await prepararAmbiente();
  }, 30_000);
  afterAll(() => desmontar(a));

  it('importa leads, reimporta com duplicados e recebe "já existia" com data', async () => {
    const primeira = await chamar(a.cliente, 'importar_leads', {
      leads: [
        { telefone: '(11) 98000-0001', nome: 'Ana' },
        { telefone: '(11) 98000-0002', nome: 'Beto' },
        { telefone: '(11) 98000-0003' },
      ],
    });
    expect(primeira.erro, primeira.texto).toBe(false);
    expect(primeira.estruturado).toMatchObject({ total_novos: 3, total_ja_existentes: 0 });

    const segunda = await chamar(a.cliente, 'importar_leads', {
      leads: [
        { telefone: '+55 11 98000-0001' },
        { telefone: '11980000002' },
        { telefone: '(11) 98000-0004', nome: 'Duda' },
        { telefone: '11 98000-0004' },
        { telefone: 'abc' },
      ],
    });
    expect(segunda.erro, segunda.texto).toBe(false);
    const rel = segunda.estruturado as unknown as RelatorioImportacao;
    expect(rel).toMatchObject({
      total_novos: 1,
      total_ja_existentes: 2,
      total_duplicados_no_lote: 1,
      total_invalidos: 1,
    });
    expect(rel.ja_existentes.every((x) => x.importado_em.length > 0)).toBe(true);
    expect(segunda.texto).toContain('já existiam');
    expect(segunda.texto).toMatch(/importado em \d{2}\/\d{2}\/\d{4} \d{2}:\d{2}/);

    const leads = await chamar(a.cliente, 'listar_leads', { origem: 'mcp' });
    expect((leads.estruturado?.['itens'] as unknown[]).length).toBe(4);
  });

  it('recusa disparo com variável faltando e cria disparo que começa na hora', async () => {
    const faltando = await chamar(a.cliente, 'criar_disparo', {
      mensagem: 'Oi {nome}!',
      destinatarios: { leads: [{ telefone: '(11) 98000-0003' }] },
      intervalo_min_s: 1,
      intervalo_max_s: 2,
    });
    expect(faltando.erro).toBe(true);
    expect(faltando.texto).toContain('{nome}');

    const r = await chamar(a.cliente, 'criar_disparo', {
      nome: 'Teste integração',
      mensagem: 'Oi {nome}, tudo bem?',
      destinatarios: { leads: [{ telefone: '(11) 98000-0001', nome: 'Ana' }, { telefone: '(11) 98000-0005' }] },
      valores_padrao: { nome: 'tudo bem' },
      intervalo_min_s: 1,
      intervalo_max_s: 2,
      limite_por_hora: 100,
    });
    expect(r.erro, r.texto).toBe(false);
    const disparo = r.estruturado?.['disparo'] as Disparo;
    expect(disparo.origem).toBe('mcp');
    expect(['agendado', 'enviando']).toContain(disparo.estado);
    expect(disparo.contadores.total).toBe(2);
    expect(r.estruturado?.['relatorio_importacao']).toMatchObject({ total_ja_existentes: 1, total_novos: 1 });

    const ver = await chamar(a.cliente, 'ver_disparo', { disparo_id: disparo.id });
    expect(ver.erro, ver.texto).toBe(false);

    const pausar = await chamar(a.cliente, 'pausar_disparo', { disparo_id: disparo.id });
    expect(pausar.erro, pausar.texto).toBe(false);
    expect(pausar.estruturado).toMatchObject({ estado: 'pausado' });
    const cancelar = await chamar(a.cliente, 'cancelar_disparo', { disparo_id: disparo.id });
    expect(cancelar.erro, cancelar.texto).toBe(false);
    expect(cancelar.estruturado).toMatchObject({ estado: 'cancelado' });
  }, 30_000);
});
