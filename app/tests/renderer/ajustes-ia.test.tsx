// T108 — Ajustes → IA: chave mascarada (o valor nunca volta à tela), testar chave, modelo padrão
// com aviso e segredos com nome.
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { ResultadoSegredos, SegredoLocal } from '../../src/preload/tipos';
import { AjustesIA, validarNomeSegredo } from '../../src/renderer/telas/secoes/AjustesIA';
import { clienteSimulado, renderizar } from './utilitarios';

const CHAVE = 'sk-ant-api03-SEGREDO-MUITO-SECRETO-a1b2';

function ponteFalsa() {
  const valores = new Map<string, string>();
  const listar = (): SegredoLocal[] => [...valores.entries()].map(([nome, v]) => ({ nome, mascara: `sk-ant-…${v.slice(-4)}` }));
  const ponte = {
    listar: vi.fn(async () => listar()),
    definir: vi.fn(async (nome: string, valor: string): Promise<ResultadoSegredos> => {
      valores.set(nome, valor);
      return { segredos: listar(), erro: null };
    }),
    remover: vi.fn(async (nome: string): Promise<ResultadoSegredos> => {
      valores.delete(nome);
      return { segredos: listar(), erro: null };
    }),
  };
  (window as unknown as { zapdesk: unknown }).zapdesk = { segredos: ponte };
  return ponte;
}

afterEach(() => {
  delete (window as unknown as { zapdesk?: unknown }).zapdesk;
});

function cliente() {
  return clienteSimulado({
    obterConfiguracaoIA: async () => ({
      modelo_padrao: 'claude-haiku-4-5-20251001',
      modelos: [
        { id: 'claude-sonnet-5', nome: 'Claude Sonnet 5', aviso: null },
        { id: 'claude-haiku-4-5-20251001', nome: 'Claude Haiku 4.5', aviso: 'Aposentadoria prevista.' },
      ],
      chave_configurada: false,
    }),
    listarSegredos: async () => [{ nome: 'OPENAI_KEY', reservado: false, usado_por: [{ automacao_id: 'a1', nome: 'Transcrever' }] }],
    testarChaveIA: async () => ({ ok: true, modelo: 'claude-sonnet-5', latencia_ms: 321 }),
  });
}

describe('Ajustes → IA', () => {
  it('guarda a chave pelo processo principal e só mostra a máscara', async () => {
    const ponte = ponteFalsa();
    const c = cliente();
    const { container } = renderizar(<AjustesIA />, { cliente: c });
    const campo = await screen.findByLabelText('Valor da chave da Anthropic');
    expect((campo as HTMLInputElement).type).toBe('password');
    await userEvent.type(campo, CHAVE);
    await userEvent.click(screen.getByRole('button', { name: 'Salvar chave' }));
    await waitFor(() => expect(ponte.definir).toHaveBeenCalledWith('ANTHROPIC_API_KEY', CHAVE));
    expect(await screen.findByLabelText('Chave guardada')).toHaveProperty('textContent', 'sk-ant-…a1b2');
    // O valor não fica em lugar nenhum da tela (nem em inputs).
    expect(container.innerHTML).not.toContain('SEGREDO-MUITO-SECRETO');
    for (const input of container.querySelectorAll('input')) expect(input.value).not.toContain('SEGREDO');

    await userEvent.click(screen.getByRole('button', { name: 'Testar chave' }));
    expect(await screen.findByText(/A chave funciona \(claude-sonnet-5, 321 ms\)/)).toBeTruthy();
  });

  it('modelo padrão com aviso e segredos com "usado por"', async () => {
    const ponte = ponteFalsa();
    await ponte.definir('OPENAI_KEY', 'sk-proj-xyz-1234');
    renderizar(<AjustesIA />, { cliente: cliente() });
    expect(await screen.findByText('Aposentadoria prevista.')).toBeTruthy();
    expect(await screen.findByText('usado por Transcrever')).toBeTruthy();
    await userEvent.type(screen.getByLabelText('Nome do segredo'), 'minha chave');
    await userEvent.type(screen.getByLabelText('Valor do segredo'), 'v');
    await userEvent.click(screen.getByRole('button', { name: 'Adicionar' }));
    expect(await screen.findByText(/Use letras maiúsculas/)).toBeTruthy();
    expect(ponte.definir).toHaveBeenCalledTimes(1);
  });

  it('valida nomes de segredo', () => {
    expect(validarNomeSegredo('ANTHROPIC_API_KEY')).toMatch(/Chave da Anthropic/);
    expect(validarNomeSegredo('OPENAI_KEY')).toBeNull();
    expect(validarNomeSegredo('1ABC')).toBeTruthy();
  });
});
