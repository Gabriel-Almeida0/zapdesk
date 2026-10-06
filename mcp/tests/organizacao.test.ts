// T134 — contatos, etiquetas e templates contra o motor simulado.
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import type { Conta, Contato, Etiqueta, Template } from '@zapdesk/cliente-motor';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { type Harness, criarHarness } from './harness.js';

describe('etiquetas', () => {
  let h: Harness;
  beforeEach(async () => {
    h = await criarHarness();
  });
  afterEach(() => h.fechar());

  it('CRUD completo', async () => {
    const criar = await h.chamar('criar_etiqueta', { nome: 'Cliente', cor: '#25D366' });
    expect(criar.erro).toBe(false);
    const etiqueta = criar.estruturado as unknown as Etiqueta;
    expect(etiqueta).toMatchObject({ nome: 'Cliente', cor: '#25D366', total_contatos: 0 });

    const listar = await h.chamar('listar_etiquetas');
    expect((listar.estruturado?.['etiquetas'] as Etiqueta[]).map((e) => e.nome)).toEqual(['Cliente']);

    const atualizar = await h.chamar('atualizar_etiqueta', { etiqueta_id: etiqueta.id, nome: 'VIP' });
    expect(atualizar.estruturado).toMatchObject({ nome: 'VIP', cor: '#25D366' });
    expect(h.motor.chamadas('PATCH', `/etiquetas/${etiqueta.id}`)[0]?.corpo).toEqual({ nome: 'VIP' });

    const excluir = await h.chamar('excluir_etiqueta', { etiqueta_id: etiqueta.id });
    expect(excluir).toMatchObject({ erro: false, estruturado: { ok: true } });
    expect((await h.chamar('listar_etiquetas')).texto).toContain('Nenhuma etiqueta');
  });

  it('nome duplicado → isError com a mensagem de conflito', async () => {
    await h.chamar('criar_etiqueta', { nome: 'Cliente', cor: '#25D366' });
    const r = await h.chamar('criar_etiqueta', { nome: 'cliente', cor: '#000000' });
    expect(r.erro).toBe(true);
    expect(r.texto).toContain('Já existe uma etiqueta com esse nome.');
  });

  it('cor inválida é rejeitada na validação de entrada; atualizar sem campos é erro de uso', async () => {
    const cor = await h.chamar('criar_etiqueta', { nome: 'X', cor: 'verde' }).catch((e: Error) => ({ erro: true, texto: e.message }));
    expect(cor.erro).toBe(true);
    const vazio = await h.chamar('atualizar_etiqueta', { etiqueta_id: 'e1' });
    expect(vazio.erro).toBe(true);
    expect(vazio.texto).toContain('Informe `nome` e/ou `cor`');
  });
});

describe('contatos', () => {
  let h: Harness;
  let conta: Conta;
  let contato: Contato;
  beforeEach(async () => {
    h = await criarHarness();
    conta = h.motor.adicionarConta();
    contato = h.motor.adicionarContato(conta.id, '+5511911112222', 'Ana');
    h.motor.adicionarContato(conta.id, '+5511933334444', 'Beto');
  });
  afterEach(() => h.fechar());

  it('listar_contatos com busca', async () => {
    const r = await h.chamar('listar_contatos', { busca: 'ana' });
    expect(r.erro).toBe(false);
    expect((r.estruturado?.['itens'] as Contato[]).map((c) => c.nome)).toEqual(['Ana']);
    expect(r.texto).toContain(`contato_id: ${contato.id}`);
  });

  it('atualizar_contato com notas e etiquetas', async () => {
    const etq = (await h.chamar('criar_etiqueta', { nome: 'Quente', cor: '#FF0000' })).estruturado as unknown as Etiqueta;
    const r = await h.chamar('atualizar_contato', {
      contato_id: contato.id,
      notas: 'Pediu orçamento',
      etiqueta_ids: [etq.id],
    });
    expect(r.erro).toBe(false);
    expect(r.estruturado).toMatchObject({ notas: 'Pediu orçamento', etiquetas: [{ nome: 'Quente' }] });
    expect(h.motor.chamadas('PATCH', `/contatos/${contato.id}`)[0]?.corpo).toEqual({ notas: 'Pediu orçamento' });
    expect(h.motor.chamadas('PUT', `/contatos/${contato.id}/etiquetas`)[0]?.corpo).toEqual({ etiqueta_ids: [etq.id] });
    expect(r.texto).toContain('etiquetas: Quente');

    const filtro = await h.chamar('listar_contatos', { etiqueta_id: etq.id });
    expect((filtro.estruturado?.['itens'] as unknown[]).length).toBe(1);
  });

  it('atualizar_contato só com etiquetas não mexe nas notas; sem nada é erro', async () => {
    const r = await h.chamar('atualizar_contato', { contato_id: contato.id, etiqueta_ids: [] });
    expect(r.erro).toBe(false);
    expect(h.motor.chamadas('PATCH', `/contatos/${contato.id}`)).toHaveLength(0);
    expect((await h.chamar('atualizar_contato', { contato_id: contato.id })).erro).toBe(true);
  });

  it('etiqueta inexistente → erro do motor', async () => {
    const r = await h.chamar('atualizar_contato', { contato_id: contato.id, etiqueta_ids: ['nao-existe'] });
    expect(r.erro).toBe(true);
    expect(r.texto).toContain('Etiqueta não encontrado');
  });
});

describe('templates', () => {
  let h: Harness;
  let pasta: string;
  beforeEach(async () => {
    h = await criarHarness();
    pasta = await mkdtemp(join(tmpdir(), 'zapdesk-tpl-'));
  });
  afterEach(async () => {
    await h.fechar();
    await rm(pasta, { recursive: true, force: true });
  });

  it('CRUD com anexo', async () => {
    const pdf = join(pasta, 'catalogo.pdf');
    await writeFile(pdf, '%PDF');
    const criar = await h.chamar('criar_template', {
      nome: 'Boas-vindas',
      texto: 'Oi {nome}, tudo bem? Veja o catálogo da {empresa}.',
      caminho_anexo: pdf,
    });
    expect(criar.erro).toBe(false);
    const template = criar.estruturado as unknown as Template;
    expect(template.variaveis).toEqual(['nome', 'empresa']);
    expect(template.arquivo?.nome).toBe('catalogo.pdf');
    expect(criar.texto).toContain('variáveis: {nome}, {empresa}');

    const listar = await h.chamar('listar_templates', { busca: 'boas' });
    expect((listar.estruturado?.['templates'] as Template[]).length).toBe(1);

    const semAnexo = await h.chamar('atualizar_template', { template_id: template.id, caminho_anexo: null, texto: 'Oi {nome}!' });
    expect(semAnexo.erro).toBe(false);
    expect(semAnexo.estruturado).toMatchObject({ arquivo: null, variaveis: ['nome'] });
    expect(h.motor.chamadas('PATCH', `/templates/${template.id}`)[0]?.corpo).toEqual({ texto: 'Oi {nome}!', arquivo_id: null });

    const excluir = await h.chamar('excluir_template', { template_id: template.id });
    expect(excluir).toMatchObject({ erro: false, estruturado: { ok: true } });
  });

  it('nome duplicado → conflito; atualizar sem campos → erro de uso', async () => {
    await h.chamar('criar_template', { nome: 'A', texto: 'x' });
    const dup = await h.chamar('criar_template', { nome: 'a', texto: 'y' });
    expect(dup.erro).toBe(true);
    expect(dup.texto).toContain('Já existe um template');
    expect((await h.chamar('atualizar_template', { template_id: 'x' })).erro).toBe(true);
  });
});
