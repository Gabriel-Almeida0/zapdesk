// T084/T116 — faixa da conversa: bot ativo + Assumir, atendimento humano + Devolver, anti-loop +
// Retomar, e atualização pelos eventos; selo "automática" na bolha.
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import type { EstadoConversaAutomacoes, SessaoChatbot } from '@zapdesk/cliente-motor';

import { Bolha } from '../../src/renderer/componentes/Bolha';
import { FaixaAutomacoes, textoFaixa } from '../../src/renderer/componentes/FaixaAutomacoes';
import { hora } from '../../src/renderer/util/formatar';
import { clienteSimulado, mensagem, renderizar } from './utilitarios';

const sessao: SessaoChatbot = {
  id: 's1',
  automacao_id: 'a2',
  automacao_nome: 'Qualificação',
  conversa_id: 'conv1',
  versao: 1,
  no_atual: 'menu1',
  variaveis: {},
  tentativas: 0,
  estado: 'ativa',
  motivo: null,
  expira_em: '2026-09-27T11:00:00-03:00',
  iniciada_em: '2026-09-27T10:00:00-03:00',
  atualizada_em: '2026-09-27T10:00:00-03:00',
  finalizada_em: null,
};

function estado(parcial: Partial<EstadoConversaAutomacoes> = {}): EstadoConversaAutomacoes {
  return { conversa_id: 'conv1', pausa: null, sessao: null, pausa_geral: false, ...parcial };
}

describe('FaixaAutomacoes', () => {
  it('bot ativo → "Assumir" cria pausa humana sem prazo', async () => {
    let atual = estado({ sessao });
    const cliente = clienteSimulado({
      estadoAutomacoesConversa: async () => atual,
      pausarConversa: async () => {
        const pausa = { conversa_id: 'conv1', motivo: 'humano' as const, ate: null, automacao_id: null, criada_em: '2026-09-27T10:00:00-03:00' };
        atual = estado({ pausa });
        return pausa;
      },
    });
    renderizar(<FaixaAutomacoes conversaId="conv1" />, { cliente });
    expect(await screen.findByText('Chatbot “Qualificação” ativo nesta conversa')).toBeTruthy();
    await userEvent.click(screen.getByRole('button', { name: 'Assumir' }));
    await waitFor(() => expect(cliente.pausarConversa).toHaveBeenCalledWith('conv1', { motivo: 'humano' }));
    expect(await screen.findByText('Atendimento humano (sem prazo)')).toBeTruthy();
  });

  it('atendimento humano até HH:MM → "Devolver às automações"', async () => {
    const ate = new Date(Date.now() + 30 * 60_000).toISOString();
    const cliente = clienteSimulado({
      estadoAutomacoesConversa: async () =>
        estado({ pausa: { conversa_id: 'conv1', motivo: 'humano', ate, automacao_id: null, criada_em: ate } }),
      retomarConversa: async () => undefined,
    });
    renderizar(<FaixaAutomacoes conversaId="conv1" />, { cliente });
    expect(await screen.findByText(`Atendimento humano até ${hora(ate)}`)).toBeTruthy();
    await userEvent.click(screen.getByRole('button', { name: 'Devolver às automações' }));
    await waitFor(() => expect(cliente.retomarConversa).toHaveBeenCalledWith('conv1'));
  });

  it('evento conversa.pausa (anti-loop) atualiza a faixa sem recarregar', async () => {
    const cliente = clienteSimulado({ estadoAutomacoesConversa: vi.fn(async () => estado()) });
    const { fonte } = renderizar(<FaixaAutomacoes conversaId="conv1" />, { cliente });
    await waitFor(() => expect(cliente.estadoAutomacoesConversa).toHaveBeenCalled());
    const ate = new Date(Date.now() + 60 * 60_000).toISOString();
    fonte.emitir({
      seq: 1,
      tipo: 'conversa.pausa',
      conta_id: 'conta1',
      em: ate,
      dados: { conversa_id: 'conv1', pausa: { conversa_id: 'conv1', motivo: 'anti_loop', ate, automacao_id: 'a1', criada_em: ate } },
    });
    expect(await screen.findByText(`Automações pausadas (anti-loop) até ${hora(ate)}`)).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Retomar' })).toBeTruthy();
  });

  it('motor sem as rotas: nenhuma faixa', async () => {
    const cliente = clienteSimulado();
    const { container } = renderizar(<FaixaAutomacoes conversaId="conv1" />, { cliente });
    await waitFor(() => expect(cliente.estadoAutomacoesConversa).toHaveBeenCalled());
    expect(container.textContent).toBe('');
  });

  it('textoFaixa e selo "automática" na bolha', () => {
    expect(textoFaixa(estado())).toBeNull();
    expect(textoFaixa(estado({ pausa: { conversa_id: 'c', motivo: 'manual', ate: null, automacao_id: null, criada_em: '' } }))).toBe(
      'Automações pausadas nesta conversa (sem prazo)',
    );
    renderizar(
      <Bolha mensagem={mensagem({ automacao_id: 'a1' })} grupo={false} aoReenviar={vi.fn()} aoResponder={vi.fn()} aoReagir={vi.fn()} aoEditar={vi.fn()} aoApagar={vi.fn()} />,
      { cliente: clienteSimulado() },
    );
    expect(screen.getByText('automática')).toBeTruthy();
  });
});
