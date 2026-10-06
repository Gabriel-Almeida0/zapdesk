// T084 — editor de fluxo: montar o fluxo do quickstart (respondeu → etiqueta "quente" → mover para
// "Qualificando" → aguardar 2 h → template) e erros de validação por campo.
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { ErroMotor, type NovaAutomacao } from '@zapdesk/cliente-motor';

import { EditorFluxo, paraNovaAutomacaoFluxo, rascunhoFluxo } from '../../src/renderer/telas/EditorFluxo/EditorFluxo';
import { automacao, funil } from './fabricas-automacoes';
import { clienteSimulado, renderizar, template } from './utilitarios';

const QUENTE = { id: 'et1', nome: 'quente', cor: '#ff0000', total_contatos: 0 };

function cliente(extra: Record<string, (...a: never[]) => unknown> = {}) {
  return clienteSimulado({
    listarEtiquetas: async () => [QUENTE],
    listarFunis: async () => [funil()],
    listarTemplates: async () => [template({ id: 't1', nome: 'Follow-up', texto: 'Oi {nome}, conseguiu ver?' })],
    listarDisparos: async () => ({ itens: [], proximo_cursor: null }),
    listarAutomacoes: async () => [],
    validarAutomacao: async () => ({ erros: [], avisos: [] }),
    ...extra,
  });
}

describe('EditorFluxo', () => {
  it('monta o fluxo do quickstart e salva no formato do contrato', async () => {
    let criado: NovaAutomacao | null = null;
    const c = cliente({
      criarAutomacao: async (d: NovaAutomacao) => {
        criado = d;
        return automacao({ id: 'novo', nome: d.nome });
      },
    });
    renderizar(<EditorFluxo />, { cliente: c, rota: '/automacoes/nova?tipo=fluxo', caminho: '/automacoes/nova' });

    await userEvent.type(screen.getByLabelText('Nome da automação'), 'Respondeu → quente');
    // Gatilho padrão: destinatário de disparo respondeu (qualquer disparo).
    expect((screen.getByLabelText('Gatilho 1') as HTMLSelectElement).value).toBe('disparo_respondeu');

    // Ação 1 (padrão "adicionar etiqueta"): escolher "quente".
    const acoes = screen.getByRole('region', { name: '3. Faça' });
    await screen.findByRole('option', { name: 'quente' });
    await userEvent.selectOptions(within(acoes).getByLabelText('Etiqueta'), 'et1');

    // Ação 2: mover para "Qualificando".
    await userEvent.selectOptions(screen.getByLabelText('Tipo da nova ação'), 'mover_etapa');
    await userEvent.click(screen.getByRole('button', { name: /Adicionar ação/ }));
    await screen.findByRole('option', { name: 'Prospecção' });
    await userEvent.selectOptions(within(acoes).getByLabelText('Funil'), 'f1');
    await userEvent.selectOptions(within(acoes).getByLabelText('Etapa'), 'e2');

    // Ação 3: aguardar 2 h.
    await userEvent.selectOptions(screen.getByLabelText('Tipo da nova ação'), 'aguardar');
    await userEvent.click(screen.getByRole('button', { name: /Adicionar ação/ }));
    const valor = within(acoes).getByLabelText('Aguardar (valor)');
    await userEvent.clear(valor);
    await userEvent.type(valor, '2');

    // Ação 4: template.
    await userEvent.selectOptions(screen.getByLabelText('Tipo da nova ação'), 'enviar_template');
    await userEvent.click(screen.getByRole('button', { name: /Adicionar ação/ }));
    await userEvent.selectOptions(within(acoes).getByLabelText('Template'), 't1');

    await userEvent.click(screen.getByRole('button', { name: /Salvar/ }));
    await waitFor(() => expect(criado).not.toBeNull());
    const d = criado as unknown as NovaAutomacao & { tipo: 'fluxo' };
    expect(d.tipo).toBe('fluxo');
    expect(d.gatilhos).toEqual([{ tipo: 'disparo_respondeu', disparo_id: null }]);
    expect(d.definicao.acoes.map((a) => a.tipo)).toEqual(['adicionar_etiqueta', 'mover_etapa', 'aguardar', 'enviar_template']);
    expect(d.definicao.acoes[0]).toMatchObject({ etiqueta_id: 'et1' });
    expect(d.definicao.acoes[1]).toMatchObject({ funil_id: 'f1', etapa_id: 'e2' });
    expect(d.definicao.acoes[2]).toMatchObject({ duracao_s: 7200 });
    expect(d.definicao.acoes[3]).toMatchObject({ template_id: 't1' });
  });

  it('mostra os erros de validação do motor no campo certo', async () => {
    const c = cliente({
      validarAutomacao: async () => ({
        erros: [{ caminho: 'definicao.acoes[0].etiqueta_id', no_id: null, acao_id: 'acao1', mensagem: 'Etiqueta não encontrada.' }],
        avisos: [],
      }),
    });
    renderizar(<EditorFluxo />, { cliente: c, rota: '/automacoes/nova?tipo=fluxo', caminho: '/automacoes/nova' });
    await userEvent.type(screen.getByLabelText('Nome da automação'), 'Teste');
    const acoes = screen.getByRole('region', { name: '3. Faça' });
    expect(await within(acoes).findByText('Etiqueta não encontrada.', {}, { timeout: 2000 })).toBeTruthy();
    expect(c.validarAutomacao).toHaveBeenCalled();
  });

  it('erro definicao_invalida ao salvar aparece por campo', async () => {
    const c = cliente({
      criarAutomacao: async () => {
        throw new ErroMotor('definicao_invalida', 'Definição inválida.', 422, {
          erros: [{ caminho: 'gatilhos[0].disparo_id', no_id: null, acao_id: null, mensagem: 'Disparo não encontrado.' }],
        });
      },
    });
    renderizar(<EditorFluxo />, { cliente: c, rota: '/automacoes/nova?tipo=fluxo', caminho: '/automacoes/nova' });
    await userEvent.type(screen.getByLabelText('Nome da automação'), 'X');
    await userEvent.click(screen.getByRole('button', { name: /Salvar/ }));
    expect(await screen.findByText('Disparo não encontrado.')).toBeTruthy();
    expect(screen.getByText('Corrija os campos destacados antes de salvar.')).toBeTruthy();
  });

  it('rascunho ida e volta preserva a automação salva', () => {
    const a = automacao();
    const nova = paraNovaAutomacaoFluxo(rascunhoFluxo(a));
    expect(nova).toMatchObject({ tipo: 'fluxo', nome: a.nome, gatilhos: a.gatilhos, definicao: a.definicao, limites: { anti_loop: null } });
  });
});
