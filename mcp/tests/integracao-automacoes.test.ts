// Integração real das ferramentas de automações (specs/002-automacoes/contracts/mcp-ferramentas.md ›
// Testes): o bundle `dist/zapdesk-mcp.mjs` via stdio, falando com o motor Go real em
// `--whatsapp=falso --ia=falsa` e com o runner compilado. Cada bloco só roda quando o motor já
// implementa as rotas que ele usa (o motor da 002 está sendo construído em paralelo); com
// `ZAPDESK_CI=1` a ausência do runner/rotas de IA falha em vez de pular.
import { existsSync } from 'node:fs';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { Client } from '@modelcontextprotocol/client';
import { StdioClientTransport, getDefaultEnvironment } from '@modelcontextprotocol/client/stdio';
import type { Automacao, Card, Conta, Execucao, Funil, Segredo } from '@zapdesk/cliente-motor';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';

import { type MotorReal, RAIZ_REPOSITORIO, flagsAutomacoesMotor, motorRealDisponivel, subirMotorReal } from './harness.js';
import { CODIGO_COM_ERRO, CODIGO_OK, MANIFESTO_RESPONDER } from './projeto-ia-exemplo.js';

const BUNDLE = join(RAIZ_REPOSITORIO, 'mcp', 'dist', 'zapdesk-mcp.mjs');
const temBundle = existsSync(BUNDLE);

const temFunis =
  temBundle &&
  (await motorRealDisponivel(async (c) => {
    await c.listarFunis();
    await c.listarSegredos();
  }));
const temIA =
  temBundle &&
  flagsAutomacoesMotor().includes('--runner-exec') &&
  (await motorRealDisponivel(async (c) => {
    const sistema = await c.sistema();
    if (!sistema.runner_disponivel) throw new Error('runner indisponível');
    await c.obterSdkAutomacao();
    await c.listarAutomacoes();
    await c.falso.ia({ respostas: [] });
  }));

if (process.env['ZAPDESK_CI'] === '1' && temBundle && !temIA) {
  throw new Error('ZAPDESK_CI=1: motor/runner sem suporte às automações de IA — o teste de integração do MCP não pode ser pulado.');
}

async function conectarStdio(pastaDados: string): Promise<Client> {
  const transporte = new StdioClientTransport({
    command: process.execPath,
    args: [BUNDLE],
    env: { ...getDefaultEnvironment(), ZAPDESK_PASTA_DADOS: pastaDados, ZAPDESK_COMANDO_ABRIR: 'true' },
    stderr: 'pipe',
  });
  const cliente = new Client({ name: 'teste-integracao-automacoes', version: '0.0.0' });
  await cliente.connect(transporte);
  return cliente;
}

async function chamar(cliente: Client, nome: string, argumentos: Record<string, unknown> = {}) {
  const r = await cliente.callTool({ name: nome, arguments: argumentos }, { timeout: 90_000 });
  const texto = ((r.content ?? []) as { type: string; text?: string }[]).map((c) => c.text ?? '').join('\n');
  return { texto, estruturado: r.structuredContent as Record<string, unknown> | undefined, erro: r.isError === true };
}

interface Ambiente {
  motor: MotorReal;
  cliente: Client;
  conta: Conta;
  pasta: string;
}

async function prepararAmbiente(): Promise<Ambiente> {
  const pasta = await mkdtemp(join(tmpdir(), 'zapdesk-integracao-aut-'));
  const motor = await subirMotorReal({ pastaDados: pasta });
  let conta = await motor.cliente.criarConta({ nome: 'Loja' });
  await motor.cliente.falso.escanearQr(conta.id, { telefone: '+5511900000001', nome: 'Loja' });
  for (let i = 0; i < 40 && conta.estado !== 'conectada'; i++) {
    await new Promise((r) => setTimeout(r, 100));
    conta = await motor.cliente.obterConta(conta.id);
  }
  const cliente = await conectarStdio(pasta);
  return { motor, cliente, conta, pasta };
}

async function desmontar(a: Ambiente | undefined): Promise<void> {
  await a?.cliente.close();
  await a?.motor.encerrar();
  if (a) await rm(a.pasta, { recursive: true, force: true });
}

describe.skipIf(!temFunis)('integração real (stdio + motor falso): funil e segredos', () => {
  let a: Ambiente;
  beforeAll(async () => {
    a = await prepararAmbiente();
  }, 30_000);
  afterAll(() => desmontar(a));

  it('cria funil e move card por telefone com origem mcp', async () => {
    const criar = await chamar(a.cliente, 'criar_funil', { nome: 'Vendas', etapas: [{ nome: 'Novo' }, { nome: 'Proposta' }] });
    expect(criar.erro, criar.texto).toBe(false);
    const funil = criar.estruturado as unknown as Funil;
    const etapa = [...funil.etapas].sort((x, y) => x.ordem - y.ordem)[1]!;
    const mover = await chamar(a.cliente, 'mover_card_funil', { funil_id: funil.id, etapa_id: etapa.id, telefone: '(11) 96666-0001' });
    expect(mover.erro, mover.texto).toBe(false);
    const card = mover.estruturado as unknown as Card;
    expect(card.lead.telefone).toBe('+5511966660001');
    const hist = await chamar(a.cliente, 'historico_funil', { funil_id: funil.id });
    expect(hist.erro, hist.texto).toBe(false);
    expect((hist.estruturado?.['itens'] as { origem: string }[])[0]?.origem).toBe('mcp');
  }, 30_000);

  it('listar_segredos não contém valores', async () => {
    await a.motor.cliente.falso.segredos({ CRM_TOKEN: 'valor-super-secreto-123' }).catch(() => undefined);
    const r = await chamar(a.cliente, 'listar_segredos');
    expect(r.erro, r.texto).toBe(false);
    expect(JSON.stringify(r)).not.toContain('valor-super-secreto-123');
    for (const s of r.estruturado?.['segredos'] as Segredo[]) expect(Object.keys(s).sort()).toEqual(['nome', 'reservado', 'usado_por']);
  });
});

describe.skipIf(!temIA)('integração real (stdio + motor falso + runner): ciclo de automação de IA', () => {
  let a: Ambiente;
  beforeAll(async () => {
    a = await prepararAmbiente();
    await a.motor.cliente.falso.ia({ respostas: [{ contem: 'sábado', texto: 'Sim, entregamos no sábado até 12h.' }] });
  }, 30_000);
  afterAll(() => desmontar(a));

  it('cria, escreve index.ts, compila com e sem erro, testa sem enviar, ativa, recebe mensagem e vê a execução', async () => {
    const c = a.cliente;
    const criada = await chamar(c, 'criar_automacao_ia', { nome: 'Responder dúvidas' });
    expect(criada.erro, criada.texto).toBe(false);
    const id = (criada.estruturado as unknown as Automacao).id;

    const sdk = await chamar(c, 'ver_tipos_sdk');
    expect(sdk.erro, sdk.texto).toBe(false);
    expect(String(sdk.estruturado?.['tipos'])).toContain('definirAutomacao');

    for (const [caminho, conteudo] of [
      ['automacao.json', MANIFESTO_RESPONDER],
      ['prompt.md', 'Responda curto, em português. Nunca invente preços.'],
      ['index.ts', CODIGO_COM_ERRO],
    ] as const) {
      const w = await chamar(c, 'escrever_arquivo_automacao', { automacao_id: id, caminho, conteudo });
      expect(w.erro, `${caminho}: ${w.texto}`).toBe(false);
    }

    const comErro = await chamar(c, 'compilar_automacao', { automacao_id: id });
    expect(comErro.estruturado?.['ok']).toBe(false);
    expect(comErro.texto).toMatch(/index\.ts:\d+:\d+ /);

    await chamar(c, 'escrever_arquivo_automacao', { automacao_id: id, caminho: 'index.ts', conteudo: CODIGO_OK });
    const ok = await chamar(c, 'compilar_automacao', { automacao_id: id });
    expect(ok.estruturado?.['ok'], ok.texto).toBe(true);

    const teste = await chamar(c, 'testar_automacao', { automacao_id: id, texto: 'Vocês entregam no sábado?', ia_simulada: true });
    expect(teste.erro, teste.texto).toBe(false);
    expect(teste.estruturado?.['estado'], teste.texto).toBe('simulacao');
    expect(await a.motor.cliente.falso.enviadas()).toHaveLength(0);

    const ativar = await chamar(c, 'ativar_automacao', { automacao_id: id });
    expect(ativar.erro, ativar.texto).toBe(false);

    await a.motor.cliente.falso.mensagemRecebida(a.conta.id, { de: '+5511955550001', texto: 'Vocês entregam no sábado?' });
    let execucoes: Execucao[] = [];
    for (let i = 0; i < 80; i++) {
      const r = await chamar(c, 'listar_execucoes', { automacao_id: id });
      execucoes = (r.estruturado?.['itens'] as Execucao[] | undefined) ?? [];
      if (execucoes.some((e) => ['ok', 'erro', 'abortada'].includes(e.estado))) break;
      await new Promise((res) => setTimeout(res, 250));
    }
    const final = execucoes.find((e) => !e.simulacao);
    expect(final?.estado, JSON.stringify(execucoes)).toBe('ok');
    const ver = await chamar(c, 'ver_execucao', { execucao_id: final!.id });
    expect(ver.erro, ver.texto).toBe(false);
    expect(ver.estruturado?.['estado']).toBe('ok');
    const enviadas = await a.motor.cliente.falso.enviadas();
    expect(enviadas.some((e) => (e.texto ?? '').includes('sábado'))).toBe(true);
  }, 90_000);
});
