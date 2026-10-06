// T132 — contas/sistema e leads contra o motor simulado.
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { type Harness, criarHarness } from './harness.js';

describe('contas e sistema', () => {
  let h: Harness;
  beforeEach(async () => {
    h = await criarHarness();
  });
  afterEach(() => h.fechar());

  it('listar_contas devolve as contas e lembra que conectar é só pela interface', async () => {
    h.motor.adicionarConta({ nome: 'Loja' });
    h.motor.adicionarConta({ nome: 'Pessoal', estado: 'desconectada' });
    const r = await h.chamar('listar_contas');
    expect(r.erro).toBe(false);
    expect((r.estruturado?.['contas'] as unknown[]).length).toBe(2);
    expect(r.texto).toContain('Loja');
    expect(r.texto).toContain('desconectada');
    const ferramentas = await h.cliente.listTools();
    const descricao = ferramentas.tools.find((t) => t.name === 'listar_contas')?.description ?? '';
    expect(descricao).toMatch(/só é possível pela interface/);
  });

  it('listar_contas sem contas explica como conectar', async () => {
    const r = await h.chamar('listar_contas');
    expect(r.texto).toMatch(/Nenhuma conta/);
  });

  it('status_zapdesk', async () => {
    h.motor.adicionarConta();
    const r = await h.chamar('status_zapdesk');
    expect(r.erro).toBe(false);
    expect(r.estruturado).toMatchObject({ versao: '0.1.0', contas_conectadas: 1, disparos_ativos: 0 });
    expect(r.texto).toContain('1 conta conectada');
  });
});

describe('leads', () => {
  let h: Harness;
  let pasta: string;
  beforeEach(async () => {
    h = await criarHarness();
    pasta = await mkdtemp(join(tmpdir(), 'zapdesk-leads-'));
  });
  afterEach(async () => {
    await h.fechar();
    await rm(pasta, { recursive: true, force: true });
  });

  it('importar_leads com 10 telefones (3 existentes) → 7 novos e 3 "já existia" com data', async () => {
    h.motor.adicionarLead('+5511900000001', 'Ana', '2026-09-01T09:30:00-03:00');
    h.motor.adicionarLead('+5511900000002', null, '2026-09-02T10:00:00-03:00');
    h.motor.adicionarLead('+5511900000003', 'Caio', '2026-09-03T11:15:00-03:00');
    const leads = Array.from({ length: 10 }, (_, i) => ({
      telefone: `(11) 90000-00${String(i + 1).padStart(2, '0')}`,
      nome: `Pessoa ${i + 1}`,
    }));

    const r = await h.chamar('importar_leads', { leads });
    expect(r.erro).toBe(false);
    expect(r.estruturado).toMatchObject({ total_novos: 7, total_ja_existentes: 3, total_invalidos: 0 });
    const ja = r.estruturado?.['ja_existentes'] as { telefone: string; importado_em: string }[];
    expect(ja.map((x) => x.importado_em)).toEqual([
      '2026-09-01T09:30:00-03:00',
      '2026-09-02T10:00:00-03:00',
      '2026-09-03T11:15:00-03:00',
    ]);
    expect(r.texto).toContain('7 novos, 3 já existiam, 0 inválidos, 0 duplicados no lote');
    expect(r.texto).toContain('+5511900000001 (linha 1) — importado em 01/09/2026 09:30');
    expect(r.texto).toContain('completou: nome'); // o lead 2 não tinha nome
    expect((r.estruturado?.['lead_ids'] as string[]).length).toBe(10);

    // sempre origem "mcp" na forma C
    const corpo = h.motor.chamadas('POST', '/leads/importar')[0]?.corpo;
    expect(corpo).toMatchObject({ origem: 'mcp' });
  });

  it('importar_leads relata inválidos e duplicados no lote', async () => {
    const r = await h.chamar('importar_leads', {
      leads: [{ telefone: '11 91111-1111' }, { telefone: 'abc' }, { telefone: '+55 11 91111-1111' }, { telefone: '123' }],
    });
    expect(r.estruturado).toMatchObject({ total_novos: 1, total_invalidos: 2, total_duplicados_no_lote: 1 });
    expect(r.texto).toContain('"abc" — formato inválido');
    expect(r.texto).toContain('igual à linha 1');
  });

  it('importar_leads com caminho_arquivo CSV (coluna sugerida) envia origem mcp', async () => {
    const csv = join(pasta, 'leads.csv');
    await writeFile(csv, 'Nome,Celular,Empresa\nAna,11 92222-0001,X\nBeto,11 92222-0002,Y\n');
    const r = await h.chamar('importar_leads', { caminho_arquivo: csv });
    expect(r.erro).toBe(false);
    expect(r.estruturado).toMatchObject({ total_novos: 2 });
    expect(h.motor.chamadas('POST', '/importacoes/previa')[0]?.corpo).toEqual({ caminho: csv });
    expect(h.motor.chamadas('POST', '/leads/importar')[0]?.corpo).toMatchObject({
      origem: 'mcp',
      mapeamento: { telefone: 'Celular', nome: 'Nome' },
    });
  });

  it('importar_leads com coluna_telefone inexistente lista as colunas', async () => {
    const csv = join(pasta, 'leads.csv');
    await writeFile(csv, 'Nome,Numero\nAna,11 92222-0001\n');
    const r = await h.chamar('importar_leads', { caminho_arquivo: csv, coluna_telefone: 'Fone' });
    expect(r.erro).toBe(true);
    expect(r.texto).toContain('"Nome", "Numero"');
  });

  it('importar_leads sem coluna sugerida pede coluna_telefone; com ela, importa', async () => {
    const csv = join(pasta, 'leads.csv');
    await writeFile(csv, 'Quem,Numero\nAna,11 92222-0001\n');
    const semColuna = await h.chamar('importar_leads', { caminho_arquivo: csv });
    expect(semColuna.erro).toBe(true);
    expect(semColuna.texto).toContain('Informe coluna_telefone');
    const comColuna = await h.chamar('importar_leads', { caminho_arquivo: csv, coluna_telefone: 'numero' });
    expect(comColuna.estruturado).toMatchObject({ total_novos: 1 });
  });

  it('importar_leads sem leads nem arquivo, ou com os dois, é erro de uso', async () => {
    expect((await h.chamar('importar_leads', {})).erro).toBe(true);
    const dois = await h.chamar('importar_leads', { leads: [{ telefone: '1' }], caminho_arquivo: '/x.csv' });
    expect(dois.erro).toBe(true);
    expect(dois.texto).toContain('não os dois');
  });

  it('importar_leads com arquivo inexistente devolve o erro do motor', async () => {
    const r = await h.chamar('importar_leads', { caminho_arquivo: join(pasta, 'nao-existe.csv') });
    expect(r.erro).toBe(true);
    expect(r.texto).toContain('Arquivo não encontrado');
  });

  it('listar_leads com filtros e paginação', async () => {
    for (let i = 0; i < 3; i++) h.motor.adicionarLead(`+55119000000${i}0`, `L${i}`, undefined, 'mcp');
    h.motor.adicionarLead('+5511988887777', 'CSV', undefined, 'csv');
    const r = await h.chamar('listar_leads', { origem: 'mcp', limite: 2 });
    expect(r.erro).toBe(false);
    expect((r.estruturado?.['itens'] as unknown[]).length).toBe(2);
    expect(r.estruturado?.['proximo_cursor']).toBe('2');
    expect(r.texto).toContain('cursor "2"');
    expect(h.motor.chamadas('GET', '/leads')[0]).toBeDefined();
  });

  it('listar_leads rejeita limite fora da faixa (validação de entrada)', async () => {
    const r = await h.chamar('listar_leads', { limite: 500 }).catch((e: Error) => ({ erro: true, texto: e.message }));
    expect(r.erro).toBe(true);
  });
});
