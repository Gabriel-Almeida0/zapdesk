// T071 — Kanban: mover por teclado e por arrastar, contagens, evento externo e exclusão de etapa.
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { Kanban } from '../../src/renderer/telas/Kanban/Kanban';
import { tempoDesde } from '../../src/renderer/telas/Kanban/Cartao';
import { validarEtapas, validarNomeFunil } from '../../src/renderer/telas/Funis';
import { card, funil } from './fabricas-automacoes';
import { clienteSimulado, pagina, renderizar } from './utilitarios';

function montar(extra: Record<string, (...a: never[]) => unknown> = {}) {
  const cards: Record<string, ReturnType<typeof card>[]> = {
    e1: [card({ lead_id: 'l1', nome: 'Ana' }), card({ lead_id: 'l2', nome: 'Bruno', lead: { id: 'l2', telefone: '+5511933334444', nome: 'Bruno', campos: {} } })],
    e2: [],
    e3: [card({ lead_id: 'l3', etapa_id: 'e3', nome: 'Carla', lead: { id: 'l3', telefone: '+5511955556666', nome: 'Carla', campos: {} } })],
  };
  const cliente = clienteSimulado({
    obterFunil: async () => funil(),
    listarCards: async (_f: string, filtro: { etapa_id?: string }) => pagina(cards[filtro.etapa_id ?? ''] ?? []),
    // O "motor" move de verdade: o refetch depois da mutação vê o novo estado.
    moverCard: async (_f: string, pedido: { lead_id: string; etapa_id: string }) => {
      let movido: ReturnType<typeof card> | undefined;
      for (const k of Object.keys(cards)) {
        movido ??= cards[k]?.find((c) => c.lead_id === pedido.lead_id);
        cards[k] = (cards[k] ?? []).filter((c) => c.lead_id !== pedido.lead_id);
      }
      const novo = { ...(movido ?? card({ lead_id: pedido.lead_id })), etapa_id: pedido.etapa_id };
      cards[pedido.etapa_id] = [novo, ...(cards[pedido.etapa_id] ?? [])];
      return novo;
    },
    ...extra,
  });
  const r = renderizar(<Kanban />, { cliente, rota: '/funis/f1', caminho: '/funis/:funilId' });
  return { ...r, cliente, cards };
}

describe('Kanban', () => {
  it('mostra colunas na ordem com cor e contagem', async () => {
    montar();
    const colunas = await screen.findAllByRole('region', { name: /^Etapa / });
    expect(colunas.map((c) => c.getAttribute('aria-label'))).toEqual(['Etapa Novo', 'Etapa Qualificando', 'Etapa Proposta']);
    expect(within(colunas[0] as HTMLElement).getByLabelText('2 leads')).toBeTruthy();
    expect(await screen.findByText('Ana')).toBeTruthy();
    expect(screen.getByText('Carla')).toBeTruthy();
  });

  it('"Mover para…" pelo teclado move o lead com origem app e atualiza as contagens', async () => {
    const { cliente } = montar();
    await screen.findByText('Ana');
    await userEvent.click(screen.getByRole('button', { name: 'Mover Ana para…' }));
    await userEvent.click(screen.getByRole('menuitem', { name: 'Qualificando' }));
    await waitFor(() => expect(cliente.moverCard).toHaveBeenCalledWith('f1', { lead_id: 'l1', etapa_id: 'e2', origem: 'app' }));
    // Otimista: "Ana" já aparece em Qualificando.
    const qualificando = screen.getByRole('region', { name: 'Etapa Qualificando' });
    await waitFor(() => expect(within(qualificando).getByText('Ana')).toBeTruthy());
    expect(within(screen.getByRole('region', { name: 'Etapa Novo' })).queryByText('Ana')).toBeNull();
  });

  it('arrastar e soltar noutra coluna move o lead', async () => {
    const { cliente } = montar();
    await screen.findByText('Bruno');
    const dados = new Map<string, string>();
    const dataTransfer = {
      setData: (t: string, v: string) => dados.set(t, v),
      getData: (t: string) => dados.get(t) ?? '',
      get types() {
        return [...dados.keys()];
      },
      effectAllowed: 'move',
      dropEffect: 'move',
    };
    const cartao = screen.getByText('Bruno').closest('.cartao-kanban') as HTMLElement;
    fireEvent.dragStart(cartao, { dataTransfer });
    const proposta = screen.getByRole('region', { name: 'Etapa Proposta' });
    fireEvent.dragOver(proposta, { dataTransfer });
    fireEvent.drop(proposta, { dataTransfer });
    await waitFor(() => expect(cliente.moverCard).toHaveBeenCalledWith('f1', { lead_id: 'l2', etapa_id: 'e3', origem: 'app' }));
  });

  it('evento funil.movido (ex.: MCP) recarrega as colunas', async () => {
    const { fonte, cards, cliente } = montar();
    await screen.findByText('Ana');
    cards.e1 = [cards.e1?.[1] as ReturnType<typeof card>];
    cards.e2 = [card({ lead_id: 'l1', etapa_id: 'e2', nome: 'Ana' })];
    const chamadasAntes = (cliente.listarCards as unknown as { mock: { calls: unknown[] } }).mock.calls.length;
    fonte.emitir({
      seq: 1,
      tipo: 'funil.movido',
      conta_id: null,
      em: '2026-09-27T10:00:00-03:00',
      dados: {
        movimento: {
          id: 'm1',
          lead_id: 'l1',
          funil_id: 'f1',
          etapa_origem_id: 'e1',
          etapa_origem_nome: 'Novo',
          etapa_destino_id: 'e2',
          etapa_destino_nome: 'Qualificando',
          origem: 'mcp',
          automacao_id: null,
          execucao_id: null,
          em: '2026-09-27T10:00:00-03:00',
        },
        card: card({ lead_id: 'l1', etapa_id: 'e2' }),
      },
    });
    await waitFor(() => expect((cliente.listarCards as unknown as { mock: { calls: unknown[] } }).mock.calls.length).toBeGreaterThan(chamadasAntes));
    const qualificando = screen.getByRole('region', { name: 'Etapa Qualificando' });
    await waitFor(() => expect(within(qualificando).getByText('Ana')).toBeTruthy());
  });

  it('excluir etapa com leads pede o destino', async () => {
    const { cliente } = montar({ excluirEtapa: async () => undefined });
    await screen.findByText('Ana');
    await userEvent.click(screen.getByRole('button', { name: /Etapas/ }));
    await userEvent.click(screen.getByRole('button', { name: 'Excluir etapa Novo' }));
    const destino = await screen.findByLabelText('Destino dos leads');
    await userEvent.selectOptions(destino, 'e3');
    await userEvent.click(screen.getByRole('button', { name: 'Excluir etapa' }));
    await waitFor(() => expect(cliente.excluirEtapa).toHaveBeenCalledWith('e1', { destino_etapa_id: 'e3' }));
  });

  it('validações e tempo na etapa', () => {
    expect(validarNomeFunil('')).toBeTruthy();
    expect(validarNomeFunil('Prospecção')).toBeNull();
    expect(validarEtapas(['Novo', 'novo'])).toMatch(/duas vezes/);
    expect(validarEtapas(['Novo', ''])).toBeTruthy();
    const agora = new Date('2026-09-27T12:00:00Z');
    expect(tempoDesde('2026-09-27T11:59:30Z', agora)).toBe('agora');
    expect(tempoDesde('2026-09-27T09:00:00Z', agora)).toBe('há 3 h');
    expect(tempoDesde('2026-09-25T12:00:00Z', agora)).toBe('há 2 dias');
  });
});
