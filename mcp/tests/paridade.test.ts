// T144/T059 — paridade: `listTools()` = união das tabelas de specs/001-zapdesk-mvp e
// specs/002-automacoes (contracts/mcp-ferramentas.md), com annotations.
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import { afterAll, beforeAll, describe, expect, it } from 'vitest';

import { type Harness, RAIZ_REPOSITORIO, criarHarness } from './harness.js';

const CONTRATO_MVP = join(RAIZ_REPOSITORIO, 'specs', '001-zapdesk-mvp', 'contracts', 'mcp-ferramentas.md');
const CONTRATO_AUTOMACOES = join(RAIZ_REPOSITORIO, 'specs', '002-automacoes', 'contracts', 'mcp-ferramentas.md');

/** Nomes da primeira coluna das tabelas da seção "## Ferramentas". */
function ferramentasDoContrato(caminho: string): string[] {
  const texto = readFileSync(caminho, 'utf8');
  const secao = texto.slice(texto.indexOf('## Ferramentas'), texto.indexOf('## Testes'));
  return [...secao.matchAll(/^\| `([a-z_]+)` \|/gm)].map((m) => m[1] as string);
}

const LEITURAS = [
  'listar_contas',
  'status_zapdesk',
  'listar_leads',
  'listar_conversas',
  'ler_mensagens',
  'buscar_mensagens',
  'ver_status',
  'listar_contatos',
  'listar_etiquetas',
  'listar_templates',
  'listar_disparos',
  'ver_disparo',
  // 002
  'listar_funis',
  'listar_cards_funil',
  'historico_funil',
  'listar_automacoes',
  'ver_automacao',
  'ver_formatos_automacao',
  'validar_automacao',
  'testar_automacao',
  'simular_chatbot',
  'ver_tipos_sdk',
  'listar_modelos_automacao_ia',
  'listar_arquivos_automacao',
  'ler_arquivo_automacao',
  'compilar_automacao',
  'listar_execucoes',
  'ver_execucao',
  'listar_pausas',
  'listar_segredos',
  'ver_configuracao_automacoes',
];

/** Exclusões locais (002): destrutivas, sem efeito no WhatsApp. */
const EXCLUSOES = ['excluir_etapa', 'excluir_funil', 'excluir_automacao', 'excluir_arquivo_automacao'];

describe('paridade com o contrato', () => {
  let h: Harness;
  beforeAll(async () => {
    h = await criarHarness();
  });
  afterAll(() => h.fechar());

  it('expõe exatamente as 69 ferramentas dos contratos (31 do MVP + 38 de automações)', async () => {
    const mvp = ferramentasDoContrato(CONTRATO_MVP);
    const automacoes = ferramentasDoContrato(CONTRATO_AUTOMACOES);
    expect(mvp).toHaveLength(31);
    expect(automacoes).toHaveLength(38);
    const contrato = [...mvp, ...automacoes];
    expect(new Set(contrato).size).toBe(69);
    const { tools } = await h.cliente.listTools();
    expect(tools.map((t) => t.name).sort()).toEqual([...contrato].sort());
  });

  it('toda ferramenta tem título, descrição, esquemas e annotations', async () => {
    const { tools } = await h.cliente.listTools();
    for (const t of tools) {
      expect(t.title, t.name).toBeTruthy();
      expect(t.description?.length ?? 0, t.name).toBeGreaterThan(30);
      expect(t.inputSchema.type, t.name).toBe('object');
      expect(t.outputSchema?.type, t.name).toBe('object');
      expect(t.annotations, t.name).toBeDefined();
    }
  });

  it('leituras com readOnlyHint; envio, disparo e apagar com destructiveHint/openWorldHint', async () => {
    const { tools } = await h.cliente.listTools();
    const porNome = new Map(tools.map((t) => [t.name, t]));
    for (const nome of LEITURAS) expect(porNome.get(nome)?.annotations?.readOnlyHint, nome).toBe(true);
    for (const nome of ['criar_disparo', 'enviar_mensagem', 'apagar_mensagem']) {
      expect(porNome.get(nome)?.annotations, nome).toMatchObject({ destructiveHint: true, openWorldHint: true });
    }
    const escritas = tools.filter((t) => !LEITURAS.includes(t.name));
    for (const t of escritas) expect(t.annotations?.readOnlyHint, t.name).toBe(false);
  });

  it('annotations das automações conforme o contrato 002', async () => {
    const { tools } = await h.cliente.listTools();
    const porNome = new Map(tools.map((t) => [t.name, t]));
    for (const nome of EXCLUSOES) {
      expect(porNome.get(nome)?.annotations, nome).toMatchObject({ destructiveHint: true, openWorldHint: false });
    }
    expect(porNome.get('executar_automacao')?.annotations).toMatchObject({ destructiveHint: true, openWorldHint: true });
    expect(porNome.get('ativar_automacao')?.annotations).toMatchObject({ readOnlyHint: false, openWorldHint: true });
    expect(porNome.get('testar_automacao')?.annotations).toMatchObject({ readOnlyHint: true, openWorldHint: true });
    for (const nome of ['editar_funil', 'mover_card_funil', 'escrever_arquivo_automacao']) {
      expect(porNome.get(nome)?.annotations?.idempotentHint, nome).toBe(true);
    }
  });

  it('a descrição de escrever_arquivo_automacao traz o resumo da API do ctx', async () => {
    const { tools } = await h.cliente.listTools();
    const descricao = tools.find((t) => t.name === 'escrever_arquivo_automacao')?.description ?? '';
    for (const trecho of ['definirAutomacao', 'aoReceberMensagem', 'ctx.responder', 'ctx.ia.gerar', 'ctx.funil', 'ctx.memoria', 'compilar_automacao']) {
      expect(descricao, trecho).toContain(trecho);
    }
  });

  it('as instruções do servidor explicam o ciclo das automações', () => {
    const instrucoes = h.cliente.getInstructions() ?? '';
    expect(instrucoes).toContain('ver_formatos_automacao');
    expect(instrucoes).toContain('ver_tipos_sdk');
    expect(instrucoes).toContain('testar_automacao');
    expect(instrucoes).toContain('Segredos só podem ser definidos pelo usuário no app');
  });
});
