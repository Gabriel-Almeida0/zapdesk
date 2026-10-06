// T118 — ferramentas de funil contra o motor simulado.
import type { Card, Funil, Lead, MovimentoFunil } from '@zapdesk/cliente-motor';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { type Harness, criarHarness } from './harness.js';

describe('funis', () => {
  let h: Harness;
  beforeEach(async () => {
    h = await criarHarness();
  });
  afterEach(() => h.fechar());

  async function criarFunil(): Promise<Funil> {
    const r = await h.chamar('criar_funil', {
      nome: 'Vendas',
      etapas: [{ nome: 'Novo' }, { nome: 'Proposta', cor: '#25D366' }, { nome: 'Fechado' }],
    });
    expect(r.erro, r.texto).toBe(false);
    return r.estruturado as unknown as Funil;
  }

  it('criar_funil e listar_funis mostram etapas com ids em ordem', async () => {
    const funil = await criarFunil();
    expect(funil.etapas.map((e) => e.nome)).toEqual(['Novo', 'Proposta', 'Fechado']);
    const r = await h.chamar('listar_funis');
    expect(r.erro).toBe(false);
    expect(r.texto).toContain(`Vendas (funil_id ${funil.id}`);
    expect(r.texto).toMatch(/Novo \[etapa_id \w+ · 0 cards\] → Proposta/);
  });

  it('criar_funil com nome repetido devolve o conflito do motor', async () => {
    await criarFunil();
    const r = await h.chamar('criar_funil', { nome: 'vendas' });
    expect(r.erro).toBe(true);
    expect(r.texto).toContain('Já existe um funil');
  });

  it('editar_funil cria etapas novas, reordena e NÃO apaga as omitidas; repetir não duplica', async () => {
    const funil = await criarFunil();
    const [novo, proposta, fechado] = funil.etapas;
    const pedido = {
      funil_id: funil.id,
      nome: 'Vendas B2B',
      etapas: [
        { etapa_id: proposta!.id, nome: 'Proposta enviada' },
        { nome: 'Negociação', cor: '#FFAA00' },
        { etapa_id: novo!.id, nome: 'Novo' },
      ],
    };
    const r = await h.chamar('editar_funil', pedido);
    expect(r.erro, r.texto).toBe(false);
    const editado = r.estruturado as unknown as Funil;
    expect(editado.nome).toBe('Vendas B2B');
    expect(editado.etapas.map((e) => e.nome)).toEqual(['Proposta enviada', 'Negociação', 'Novo', 'Fechado']);
    expect(editado.etapas.find((e) => e.id === fechado!.id)).toBeDefined();

    // idempotente: a mesma chamada de novo não cria outra "Negociação"
    const r2 = await h.chamar('editar_funil', pedido);
    expect(r2.erro, r2.texto).toBe(false);
    expect((r2.estruturado as unknown as Funil).etapas).toHaveLength(4);
    expect(h.motor.chamadas('POST', `/funis/${funil.id}/etapas`)).toHaveLength(1);
    expect(h.motor.chamadas('PUT', `/funis/${funil.id}/etapas/ordem`)).toHaveLength(1);
  });

  it('editar_funil com etapa de outro funil é erro de uso', async () => {
    const funil = await criarFunil();
    const r = await h.chamar('editar_funil', { funil_id: funil.id, etapas: [{ etapa_id: 'xyz', nome: 'X' }] });
    expect(r.erro).toBe(true);
    expect(r.texto).toContain('não pertence ao funil');
  });

  it('mover_card_funil por telefone cria o lead com origem mcp e registra histórico mcp', async () => {
    const funil = await criarFunil();
    const [novo, proposta] = funil.etapas;
    const r = await h.chamar('mover_card_funil', { funil_id: funil.id, etapa_id: novo!.id, telefone: '(11) 97777-0001' });
    expect(r.erro, r.texto).toBe(false);
    const card = r.estruturado as unknown as Card;
    expect(card.lead.telefone).toBe('+5511977770001');
    expect(h.motor.chamadas('PUT', `/funis/${funil.id}/cards`)[0]?.corpo).toMatchObject({
      telefone: '(11) 97777-0001',
      origem: 'mcp',
    });
    expect(h.motor.leads.find((l) => l.telefone === '+5511977770001')?.origem).toBe('mcp');

    const mover = await h.chamar('mover_card_funil', { funil_id: funil.id, etapa_id: proposta!.id, lead_id: card.lead_id });
    expect(mover.erro).toBe(false);
    expect(mover.texto).toContain('etapa Proposta');

    const hist = await h.chamar('historico_funil', { funil_id: funil.id, lead_id: card.lead_id });
    const itens = hist.estruturado?.['itens'] as MovimentoFunil[];
    expect(itens).toHaveLength(2);
    expect(itens.every((m) => m.origem === 'mcp')).toBe(true);
    expect(hist.texto).toContain('Novo → Proposta (por mcp)');
    expect(hist.texto).toContain('fora do funil → Novo');

    const cards = await h.chamar('listar_cards_funil', { funil_id: funil.id, etapa_id: proposta!.id });
    expect((cards.estruturado?.['itens'] as Card[]).map((c) => c.lead_id)).toEqual([card.lead_id]);
  });

  it('mover_card_funil exige exatamente um alvo', async () => {
    const funil = await criarFunil();
    const nenhum = await h.chamar('mover_card_funil', { funil_id: funil.id, etapa_id: funil.etapas[0]!.id });
    expect(nenhum.erro).toBe(true);
    const dois = await h.chamar('mover_card_funil', {
      funil_id: funil.id,
      etapa_id: funil.etapas[0]!.id,
      lead_id: 'a',
      telefone: '11 90000-0000',
    });
    expect(dois.erro).toBe(true);
    expect(dois.texto).toContain('exatamente um');
  });

  it('remover_card_funil manda origem mcp; excluir_etapa com cards exige destino', async () => {
    const funil = await criarFunil();
    const [novo, proposta] = funil.etapas;
    const r = await h.chamar('mover_card_funil', { funil_id: funil.id, etapa_id: novo!.id, telefone: '11 97777-0002' });
    const leadId = (r.estruturado as unknown as Card).lead_id;

    const semDestino = await h.chamar('excluir_etapa', { etapa_id: novo!.id });
    expect(semDestino.erro).toBe(true);
    expect(semDestino.texto).toContain('Escolha para onde mover os cards');

    const comDestino = await h.chamar('excluir_etapa', { etapa_id: novo!.id, destino_etapa_id: proposta!.id });
    expect(comDestino.erro, comDestino.texto).toBe(false);
    expect(h.motor.automacoes.cards[0]?.etapa_id).toBe(proposta!.id);

    const remover = await h.chamar('remover_card_funil', { funil_id: funil.id, lead_id: leadId });
    expect(remover.erro).toBe(false);
    expect(h.motor.chamadas('DELETE', `/funis/${funil.id}/cards/${leadId}`)).toHaveLength(1);
    expect(h.motor.automacoes.historico[0]?.origem).toBe('mcp');

    const excluir = await h.chamar('excluir_funil', { funil_id: funil.id });
    expect(excluir.erro).toBe(false);
    expect(h.motor.automacoes.funis).toHaveLength(0);
  });

  it('atualizar_lead mescla campos e remove com null', async () => {
    const lead = h.motor.adicionarLead('+5511966660000', 'Ana');
    lead.campos = { empresa: 'X', cidade: 'SP' };
    const r = await h.chamar('atualizar_lead', { lead_id: lead.id, nome: 'Ana Souza', campos: { empresa: 'ACME', cidade: null, cargo: 'CEO' } });
    expect(r.erro, r.texto).toBe(false);
    expect((r.estruturado as unknown as Lead).campos).toEqual({ empresa: 'ACME', cargo: 'CEO' });
    expect(r.texto).toContain('Ana Souza');
    expect(r.texto).toContain('empresa=ACME');
    expect((await h.chamar('atualizar_lead', { lead_id: lead.id })).erro).toBe(true);
  });
});
