// T117 — editor de chatbot: grafo (saídas, ligar, remover), canvas com nós e erros destacados,
// painel de propriedades montando menu → pergunta → humano, e chat simulado.
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeAll, describe, expect, it } from 'vitest';

import type { DefinicaoChatbot, NovaAutomacao } from '@zapdesk/cliente-motor';

import { arestas, EditorChatbot } from '../../src/renderer/telas/EditorChatbot/EditorChatbot';
import { definicaoPadrao, definirSaida, novoNo, posicoesAutomaticas, removerNos, saidas } from '../../src/renderer/telas/EditorChatbot/grafo';
import { automacao } from './fabricas-automacoes';
import { clienteSimulado, renderizar } from './utilitarios';

beforeAll(() => {
  // O React Flow mede os nós: jsdom não tem ResizeObserver nem DOMMatrixReadOnly.
  const g = globalThis as unknown as Record<string, unknown>;
  g['ResizeObserver'] ??= class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
  g['DOMMatrixReadOnly'] ??= class {
    m22 = 1;
    constructor() {}
  };
});

function cliente(extra: Record<string, (...a: never[]) => unknown> = {}) {
  return clienteSimulado({
    listarEtiquetas: async () => [],
    listarFunis: async () => [],
    listarAutomacoes: async () => [],
    validarAutomacao: async () => ({ erros: [], avisos: [] }),
    ...extra,
  });
}

describe('grafo do chatbot', () => {
  it('saídas por tipo, ligar e desligar', () => {
    const def = definicaoPadrao();
    const menu = def.nos.find((n) => n.id === 'menu1');
    expect(menu && saidas(menu).map((s) => s.id)).toEqual(['opcao:0', 'opcao:1', 'ao_esgotar']);
    const ligado = definirSaida(menu as NonNullable<typeof menu>, 'ao_esgotar', 'fim1');
    expect(ligado).toMatchObject({ ao_esgotar: 'fim1' });
    expect(definirSaida(ligado, 'ao_esgotar', '')).toMatchObject({ ao_esgotar: null });
    expect(definirSaida(menu as NonNullable<typeof menu>, 'opcao:1', 'fim1')).toMatchObject({ opcoes: [{ proximo: 'pergunta_email' }, { proximo: 'fim1' }] });
  });

  it('remover nó desliga as saídas que apontavam para ele e nunca remove o início', () => {
    const def = removerNos(definicaoPadrao(), ['humano1', 'inicio']);
    expect(def.nos.some((n) => n.id === 'humano1')).toBe(false);
    expect(def.nos.some((n) => n.id === 'inicio')).toBe(true);
    const menu = def.nos.find((n) => n.id === 'menu1');
    expect(menu && menu.tipo === 'menu' ? menu.opcoes[1]?.proximo : null).toBe('');
  });

  it('arestas só das saídas ligadas e posições automáticas por nível', () => {
    const def: DefinicaoChatbot = {
      ...definicaoPadrao(),
      nos: definicaoPadrao().nos.map((n) => ({ ...n, posicao: undefined })),
    };
    expect(arestas(def, null).map((a) => a.id)).toContain('menu1|opcao:1');
    const pos = posicoesAutomaticas(def);
    expect(pos.get('inicio')).toEqual({ x: 0, y: 0 });
    expect((pos.get('menu1')?.x ?? 0) > (pos.get('boas_vindas')?.x ?? 0)).toBe(true);
    expect(novoNo('pergunta', ['pergunta1'], { x: 1, y: 2 })).toMatchObject({ id: 'pergunta2', variavel: 'pergunta2' });
  });
});

describe('EditorChatbot', () => {
  it('mostra os nós do bot novo e destaca os nós com erro da validação', async () => {
    const c = cliente({
      validarAutomacao: async () => ({
        erros: [{ caminho: 'definicao.nos[2].opcoes[0].proximo', no_id: 'menu1', acao_id: null, mensagem: 'Opção sem destino.' }],
        avisos: [],
      }),
    });
    renderizar(<EditorChatbot />, { cliente: c, rota: '/automacoes/nova?tipo=chatbot', caminho: '/automacoes/nova' });
    expect(await screen.findByTestId('no-menu1')).toBeTruthy();
    expect(screen.getByTestId('no-pergunta_email')).toBeTruthy();
    await userEvent.type(screen.getByLabelText('Nome do chatbot'), 'Qualificação');
    await waitFor(() => expect(screen.getByTestId('no-menu1').className).toContain('com-erro'), { timeout: 2000 });
    expect(screen.getByText(/1 nó com problema/)).toBeTruthy();
  });

  it('monta menu → pergunta → humano pela paleta e salva a definição', async () => {
    let criado: NovaAutomacao | null = null;
    const c = cliente({
      criarAutomacao: async (d: NovaAutomacao) => {
        criado = d;
        return automacao({ id: 'b1', tipo: 'chatbot', nome: d.nome, definicao: d.definicao, gatilhos: d.gatilhos });
      },
    });
    renderizar(<EditorChatbot />, { cliente: c, rota: '/automacoes/nova?tipo=chatbot', caminho: '/automacoes/nova' });
    await screen.findByTestId('no-menu1');
    await userEvent.type(screen.getByLabelText('Nome do chatbot'), 'Bot de teste');
    // Adiciona um nó "Pergunta": vira o selecionado e abre o painel dele.
    const paleta = screen.getByRole('toolbar', { name: 'Adicionar nó' });
    await userEvent.click(within(paleta).getByRole('button', { name: /Pergunta/ }));
    expect(await screen.findByTestId('no-pergunta1')).toBeTruthy();
    await userEvent.type(screen.getByLabelText('Pergunta'), 'Qual sua empresa?');
    const variavel = screen.getByLabelText(/Guardar a resposta na variável/);
    await userEvent.clear(variavel);
    await userEvent.type(variavel, 'empresa');
    await userEvent.click(screen.getByRole('button', { name: /Salvar/ }));
    await waitFor(() => expect(criado).not.toBeNull());
    const def = (criado as unknown as { definicao: DefinicaoChatbot }).definicao;
    expect(def.nos.find((n) => n.id === 'pergunta1')).toMatchObject({ tipo: 'pergunta', texto: 'Qual sua empresa?', variavel: 'empresa' });
    expect(def.nos.every((n) => n.posicao)).toBe(true);
    expect((criado as unknown as NovaAutomacao).gatilhos).toEqual([{ tipo: 'palavra_chave', palavras: ['orçamento'], modo: 'palavra' }]);
  });

  it('chat simulado: conversa com o bot, mostra variáveis e o nó atual', async () => {
    const def = definicaoPadrao();
    const c = cliente({
      iniciarSimulador: async () => ({
        simulacao_id: 'sim1',
        saidas: [
          { tipo: 'mensagem', texto: 'Olá, Ana! Aqui é o atendimento automático.', no_id: 'boas_vindas' },
          { tipo: 'mensagem', texto: 'Como posso ajudar?\n1 - Preços\n2 - Falar com vendedor', no_id: 'menu1' },
        ],
        no_atual: 'menu1',
        variaveis: {},
        estado: 'ativa',
      }),
      enviarAoSimulador: async (_id: string, texto: string) =>
        texto === '1'
          ? { saidas: [{ tipo: 'mensagem', texto: 'Qual é o seu e-mail?', no_id: 'pergunta_email' }], no_atual: 'pergunta_email', variaveis: {}, estado: 'ativa', acoes: [] }
          : {
              saidas: [{ tipo: 'mensagem', texto: `Obrigado! Enviamos a tabela para ${texto}.`, no_id: 'fim1' }],
              no_atual: null,
              variaveis: { email: texto },
              estado: 'concluida',
              acoes: [],
            },
      encerrarSimulador: async () => undefined,
    });
    renderizar(<EditorChatbot automacao={automacao({ id: 'b1', tipo: 'chatbot', nome: 'Qualificação', definicao: def, gatilhos: [] })} />, {
      cliente: c,
      rota: '/automacoes/b1',
      caminho: '/automacoes/:automacaoId',
    });
    await userEvent.click(screen.getByRole('button', { name: /Testar/ }));
    expect(await screen.findByText(/Olá, Ana!/)).toBeTruthy();
    await waitFor(() => expect(c.iniciarSimulador).toHaveBeenCalledWith('b1', expect.objectContaining({ ia_simulada: true })));
    await waitFor(() => expect(screen.getByTestId('no-menu1').className).toContain('atual'));
    const entrada = screen.getByLabelText('Mensagem do contato');
    await userEvent.type(entrada, '1{Enter}');
    expect(await screen.findByText('Qual é o seu e-mail?')).toBeTruthy();
    await userEvent.type(entrada, 'ana@exemplo.com{Enter}');
    expect(await screen.findByText('Obrigado! Enviamos a tabela para ana@exemplo.com.')).toBeTruthy();
    expect(screen.getByText('{email}')).toBeTruthy();
    expect(screen.getByText('Sessão encerrada: concluída')).toBeTruthy();
  });
});
