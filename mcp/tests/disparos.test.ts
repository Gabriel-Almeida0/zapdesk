// T135 — disparos contra o motor simulado.
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import type { Conta, Disparo } from '@zapdesk/cliente-motor';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { contarLinhasCsv, estadoLegivel } from '../src/ferramentas/disparos.js';
import { type Harness, criarHarness } from './harness.js';

const ritmo = { intervalo_min_s: 30, intervalo_max_s: 90, limite_por_hora: 40 };

describe('disparos', () => {
  let h: Harness;
  let conta: Conta;
  let pasta: string;

  beforeEach(async () => {
    pasta = await mkdtemp(join(tmpdir(), 'zapdesk-disp-'));
    h = await criarHarness({ pastaDownloads: pasta });
    conta = h.motor.adicionarConta();
  });
  afterEach(async () => {
    await h.fechar();
    await rm(pasta, { recursive: true, force: true });
  });

  it('descrição de criar_disparo avisa que começa na hora e cita pausar/cancelar', async () => {
    const { tools } = await h.cliente.listTools();
    const criar = tools.find((t) => t.name === 'criar_disparo');
    expect(criar?.description).toMatch(/COMEÇA NA HORA/);
    expect(criar?.description).toMatch(/sem pedir confirmação/);
    expect(criar?.description).toContain('pausar_disparo');
    expect(criar?.description).toContain('cancelar_disparo');
    expect(criar?.annotations).toMatchObject({ destructiveHint: true, openWorldHint: true });
  });

  it('criar_disparo com leads novos começa na hora com origem mcp e iniciar true', async () => {
    h.motor.adicionarLead('+5511900000001', 'Ana', '2026-09-01T09:30:00-03:00');
    const r = await h.chamar('criar_disparo', {
      nome: 'Prospecção',
      mensagem: 'Oi {nome}, tudo bem?',
      destinatarios: {
        leads: [
          { telefone: '11 90000-0001', nome: 'Ana' },
          { telefone: '11 90000-0002', nome: 'Beto' },
        ],
      },
      ...ritmo,
    });
    expect(r.erro).toBe(false);
    const disparo = r.estruturado?.['disparo'] as Disparo;
    expect(disparo).toMatchObject({ estado: 'enviando', origem: 'mcp', na_fila: false });
    expect(disparo.contadores.total).toBe(2);
    expect(r.estruturado?.['relatorio_importacao']).toMatchObject({ total_novos: 1, total_ja_existentes: 1 });
    expect(disparo).not.toHaveProperty('relatorio_importacao');

    const corpo = h.motor.chamadas('POST', '/disparos')[0]?.corpo;
    expect(corpo).toMatchObject({
      conta_id: conta.id,
      iniciar: true,
      origem: 'mcp',
      mensagem: 'Oi {nome}, tudo bem?',
      destinatarios: { importar: { origem: 'mcp' } },
      ritmo: { intervalo_min_s: 30, intervalo_max_s: 90, limite_por_hora: 40, limite_por_dia: null },
      falhas_seguidas_max: 10,
      janela: null,
      inicio_em: null,
    });
    expect(r.texto).toContain('Disparo criado e iniciado');
    expect(r.texto).toContain('pausar_disparo ou cancelar_disparo');
    expect(r.texto).toContain('1 já existia');
    expect(r.texto).toContain('importado em 01/09/2026 09:30');
  });

  it('variáveis faltando sem valores_padrao → isError com a lista e nada é criado', async () => {
    const r = await h.chamar('criar_disparo', {
      mensagem: 'Oi {nome}, sua empresa {empresa}',
      destinatarios: {
        leads: [
          { telefone: '11 90000-0001', nome: 'Ana', campos: { empresa: 'X' } },
          { telefone: '11 90000-0002' },
          { telefone: '11 90000-0003', nome: 'Caio' },
        ],
      },
      ...ritmo,
    });
    expect(r.erro).toBe(true);
    expect(r.texto).toContain('2 destinatários sem {empresa}, 1 sem {nome}');
    expect(r.texto).toContain('linha 2: +5511900000002 — falta {nome}, {empresa}');
    expect(r.texto).toContain('valores_padrao');
    expect(h.motor.disparos).toHaveLength(0);
  });

  it('valores_padrao destrava a criação', async () => {
    const r = await h.chamar('criar_disparo', {
      mensagem: 'Oi {nome}!',
      destinatarios: { leads: [{ telefone: '11 90000-0002' }] },
      valores_padrao: { nome: 'tudo bem' },
      ...ritmo,
    });
    expect(r.erro).toBe(false);
    expect(h.motor.chamadas('POST', '/disparos')[0]?.corpo).toMatchObject({ valores_padrao: { nome: 'tudo bem' } });
  });

  it('segundo disparo da conta vai para a fila e o texto diz quando começa', async () => {
    const lead = h.motor.adicionarLead('+5511900000001', 'Ana');
    await h.chamar('criar_disparo', { nome: 'Primeiro', mensagem: 'a', destinatarios: { lead_ids: [lead.id] }, ...ritmo });
    const r = await h.chamar('criar_disparo', { nome: 'Segundo', mensagem: 'b', destinatarios: { lead_ids: [lead.id] }, ...ritmo });
    expect(r.erro).toBe(false);
    expect(r.estruturado?.['disparo']).toMatchObject({ estado: 'agendado', na_fila: true });
    expect(r.texto).toContain('Na fila: começa quando "Primeiro" terminar.');
    expect(r.texto).toContain('"Segundo" — na fila');
  });

  it('validações locais: mensagem/template, intervalos, pausa, janela e destinatários', async () => {
    const base = { destinatarios: { lead_ids: ['l1'] }, ...ritmo };
    expect((await h.chamar('criar_disparo', base)).texto).toContain('`mensagem` ou `template_id`');
    expect(
      (await h.chamar('criar_disparo', { ...base, mensagem: 'x', intervalo_min_s: 90, intervalo_max_s: 30 })).texto,
    ).toContain('intervalo_max_s');
    expect((await h.chamar('criar_disparo', { ...base, mensagem: 'x', pausa_a_cada: 50 })).texto).toContain('juntos');
    expect((await h.chamar('criar_disparo', { ...base, mensagem: 'x', janela_inicio: '09:00' })).texto).toContain('juntos');
    expect(
      (await h.chamar('criar_disparo', { ...base, mensagem: 'x', janela_inicio: '09:00', janela_fim: '09:00' })).texto,
    ).toContain('diferentes');
    expect((await h.chamar('criar_disparo', { ...ritmo, mensagem: 'x', destinatarios: {} })).texto).toContain(
      'pelo menos um destinatário',
    );
    expect(h.motor.chamadas('POST', '/disparos')).toHaveLength(0);
  });

  it('janela, pausas, inicio_em, template e etiqueta vão no corpo do motor', async () => {
    const tpl = (await h.chamar('criar_template', { nome: 'T', texto: 'Oi {nome}' })).estruturado as { id: string };
    const etq = (await h.chamar('criar_etiqueta', { nome: 'VIP', cor: '#111111' })).estruturado as { id: string };
    const contato = h.motor.adicionarContato(conta.id, '+5511955556666', 'Duda');
    contato.etiquetas = [h.motor.etiquetas[0]!];
    const r = await h.chamar('criar_disparo', {
      template_id: tpl.id,
      destinatarios: { etiqueta_ids: [etq.id] },
      ...ritmo,
      pausa_a_cada: 50,
      pausa_duracao_s: 600,
      janela_inicio: '09:00',
      janela_fim: '18:00',
      inicio_em: '2026-09-28T09:00:00-03:00',
      falhas_seguidas_max: 3,
    });
    expect(r.erro).toBe(false);
    expect(h.motor.chamadas('POST', '/disparos')[0]?.corpo).toMatchObject({
      template_id: tpl.id,
      janela: { inicio: '09:00', fim: '18:00' },
      inicio_em: '2026-09-28T09:00:00-03:00',
      falhas_seguidas_max: 3,
      ritmo: { pausa_a_cada: 50, pausa_duracao_s: 600 },
      destinatarios: { etiqueta_ids: [etq.id], importar: null },
    });
    expect(r.texto).toContain('Começa em 28/09/2026 09:00');
  });

  it('pausar, retomar, cancelar e transição inválida', async () => {
    const lead = h.motor.adicionarLead('+5511900000001', 'Ana');
    const criado = await h.chamar('criar_disparo', { mensagem: 'a', destinatarios: { lead_ids: [lead.id] }, ...ritmo });
    const id = (criado.estruturado?.['disparo'] as Disparo).id;

    const pausar = await h.chamar('pausar_disparo', { disparo_id: id });
    expect(pausar.estruturado).toMatchObject({ estado: 'pausado', motivo_pausa: 'usuario' });
    expect(pausar.texto).toContain('pausado (motivo: usuario)');

    const retomar = await h.chamar('retomar_disparo', { disparo_id: id });
    expect(retomar.estruturado).toMatchObject({ estado: 'enviando' });

    const cancelar = await h.chamar('cancelar_disparo', { disparo_id: id });
    expect(cancelar.estruturado).toMatchObject({ estado: 'cancelado' });

    const invalida = await h.chamar('pausar_disparo', { disparo_id: id });
    expect(invalida.erro).toBe(true);
    expect(invalida.texto).toContain('(estado atual: cancelado)');
  });

  it('iniciar_disparo de rascunho criado no app', async () => {
    const lead = h.motor.adicionarLead('+5511900000001', null);
    const rascunho = await h.motor
      .cliente()
      .criarDisparo({ conta_id: conta.id, mensagem: 'Oi {nome}', destinatarios: { lead_ids: [lead.id] }, ritmo: { intervalo_min_s: 30, intervalo_max_s: 60, limite_por_hora: null, limite_por_dia: null, pausa_a_cada: null, pausa_duracao_s: null } });
    expect(rascunho.estado).toBe('rascunho');
    const r = await h.chamar('iniciar_disparo', { disparo_id: rascunho.id, valores_padrao: { nome: 'cliente' } });
    expect(r.erro).toBe(false);
    expect(r.estruturado).toMatchObject({ estado: 'enviando' });
    expect(h.motor.chamadas('POST', `/disparos/${rascunho.id}/iniciar`)[0]?.corpo).toEqual({
      valores_padrao: { nome: 'cliente' },
    });
  });

  it('listar_disparos e ver_disparo com destinatários', async () => {
    const l1 = h.motor.adicionarLead('+5511900000001', 'Ana');
    const l2 = h.motor.adicionarLead('+5511900000002', 'Beto');
    const criado = await h.chamar('criar_disparo', { nome: 'D1', mensagem: 'a', destinatarios: { lead_ids: [l1.id, l2.id] }, ...ritmo });
    const id = (criado.estruturado?.['disparo'] as Disparo).id;
    h.motor.destinatarios.get(id)![1]!.estado = 'falhou';
    h.motor.destinatarios.get(id)![1]!.motivo_falha = 'Número sem WhatsApp';

    const lista = await h.chamar('listar_disparos', { estado: 'enviando' });
    expect((lista.estruturado?.['itens'] as Disparo[]).map((d) => d.nome)).toEqual(['D1']);

    const ver = await h.chamar('ver_disparo', { disparo_id: id });
    expect(ver.erro).toBe(false);
    expect((ver.estruturado?.['destinatarios'] as { itens: unknown[] }).itens).toHaveLength(2);
    expect(ver.texto).toContain('2. +5511900000002 (Beto) — falhou: Número sem WhatsApp');

    const falhas = await h.chamar('ver_disparo', { disparo_id: id, estado_destinatarios: 'falhou' });
    expect((falhas.estruturado?.['destinatarios'] as { itens: unknown[] }).itens).toHaveLength(1);

    const inexistente = await h.chamar('ver_disparo', { disparo_id: 'nao-existe' });
    expect(inexistente.erro).toBe(true);
  });

  it('exportar_relatorio grava CSV no caminho pedido e no padrão ~/Downloads', async () => {
    const lead = h.motor.adicionarLead('+5511900000001', 'Ana');
    const criado = await h.chamar('criar_disparo', { nome: 'Promoção de Setembro', mensagem: 'a', destinatarios: { lead_ids: [lead.id] }, ...ritmo });
    const id = (criado.estruturado?.['disparo'] as Disparo).id;

    const destino = join(pasta, 'sub', 'relatorio.csv');
    const r = await h.chamar('exportar_relatorio', { disparo_id: id, caminho_destino: destino });
    expect(r.erro).toBe(false);
    expect(r.estruturado).toEqual({ caminho: destino, linhas: 1 });
    expect(await readFile(destino, 'utf8')).toContain('+5511900000001');

    const padrao = await h.chamar('exportar_relatorio', { disparo_id: id });
    expect(padrao.estruturado?.['caminho']).toMatch(
      new RegExp(`^${pasta}/zapdesk-promocao-de-setembro-\\d{4}-\\d{2}-\\d{2}\\.csv$`),
    );
  });

  it('contarLinhasCsv respeita BOM, aspas com quebra de linha e linha final', () => {
    expect(contarLinhasCsv('﻿a,b\n1,2\n3,"x\ny"\n')).toBe(2);
    expect(contarLinhasCsv('a,b\r\n')).toBe(0);
    expect(contarLinhasCsv('')).toBe(0);
  });
});

describe('estadoLegivel', () => {
  const agora = new Date('2026-09-28T10:00:00-03:00');
  const base = { estado: 'agendado', na_fila: false, motivo_pausa: null } as unknown as Disparo;
  it('disparo recém-iniciado não aparece como "agendado"', () => {
    expect(estadoLegivel({ ...base, inicio_em: '2026-09-28T09:59:00-03:00' }, agora)).toBe('começando agora');
    expect(estadoLegivel({ ...base, inicio_em: null } as unknown as Disparo, agora)).toBe('começando agora');
  });
  it('início no futuro mostra a data', () => {
    expect(estadoLegivel({ ...base, inicio_em: '2026-09-28T14:00:00-03:00' }, agora)).toMatch(/^agendado para /);
  });
  it('na fila tem prioridade', () => {
    expect(estadoLegivel({ ...base, na_fila: true, inicio_em: null } as unknown as Disparo, agora)).toBe('na fila');
  });
});
