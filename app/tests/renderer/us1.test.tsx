// T066 — US1: Conversas (não lidas, filtro), Chat (estados da bolha, Reenviar), ConectarConta (QR e
// sucesso), Nova conversa sem WhatsApp, atalhos e telas do motor.
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { ErroMotor } from '@zapdesk/cliente-motor';

import { interpretarAtalho, vizinho } from '../../src/renderer/atalhos';
import { App } from '../../src/renderer/App';
import { NovaConversa, SEM_WHATSAPP } from '../../src/renderer/componentes/NovaConversa';
import { Chat } from '../../src/renderer/telas/Chat';
import { ConectarConta, TEXTO_QR } from '../../src/renderer/telas/ConectarConta';
import { Conversas } from '../../src/renderer/telas/Conversas';
import type { EstadoMotor } from '../../src/preload/tipos';
import { clienteSimulado, conta, conversa, mensagem, pagina, renderizar } from './utilitarios';

describe('Conversas', () => {
  it('mostra a lista com não lidas e filtra por "Não lidas"', async () => {
    const cliente = clienteSimulado({
      listarConversas: async () =>
        pagina([conversa({ id: 'c1', nome: 'Ana', nao_lidas: 3 }), conversa({ id: 'c2', nome: 'Bruno', nao_lidas: 0 })]),
    });
    renderizar(<Conversas />, { cliente, rota: '/conversas', caminho: '/conversas' });

    expect(await screen.findByText('Ana')).toBeTruthy();
    expect(screen.getByText('Bruno')).toBeTruthy();
    expect(screen.getByLabelText('3 não lidas')).toBeTruthy();

    await userEvent.click(screen.getByRole('button', { name: 'Não lidas' }));
    await waitFor(() =>
      expect(cliente.listarConversas).toHaveBeenLastCalledWith('conta1', expect.objectContaining({ nao_lidas: true })),
    );
  });

  it('sem contas mostra "Nenhuma conta conectada" com "Conectar conta"', async () => {
    const cliente = clienteSimulado({ listarContas: async () => [] });
    renderizar(<Conversas />, { cliente, rota: '/conversas', caminho: '/conversas' });
    expect(await screen.findByText('Nenhuma conta conectada')).toBeTruthy();
    expect(screen.getByRole('link', { name: 'Conectar conta' })).toBeTruthy();
  });

  it('conta desconectada mostra a faixa com "Reconectar"', async () => {
    const cliente = clienteSimulado({
      listarContas: async () => [conta({ estado: 'desconectada' })],
      listarConversas: async () => pagina([]),
    });
    renderizar(<Conversas />, { cliente, rota: '/conversas', caminho: '/conversas' });
    expect(await screen.findByText('Conta desconectada. Reconecte para continuar.')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Reconectar' })).toBeTruthy();
  });
});

describe('Chat', () => {
  function clienteChat(mensagens = [mensagem()]) {
    return clienteSimulado({
      obterConversa: async () => conversa({ nao_lidas: 2 }),
      listarMensagens: async () => pagina(mensagens),
      marcarLida: async () => undefined,
      reenviarMensagem: async () => mensagem({ id: 'falha', texto: 'não foi', estado: 'pendente' }),
    });
  }

  it('mostra o estado de cada bolha e marca a conversa como lida ao abrir', async () => {
    const cliente = clienteChat([
      mensagem({ id: 'a', texto: 'pendente', estado: 'pendente' }),
      mensagem({ id: 'b', texto: 'lida', estado: 'lida' }),
      mensagem({ id: 'c', texto: 'entregue', estado: 'entregue' }),
      mensagem({ id: 'd', texto: 'recebida', estado: 'recebida', de_mim: false }),
    ]);
    renderizar(<Chat conversaId="conv1" />, { cliente });
    expect(await screen.findByText('pendente')).toBeTruthy();
    expect(screen.getByRole('img', { name: 'Enviando' })).toBeTruthy();
    expect(screen.getByRole('img', { name: 'Lida' })).toBeTruthy();
    expect(screen.getByRole('img', { name: 'Entregue' })).toBeTruthy();
    await waitFor(() => expect(cliente.marcarLida).toHaveBeenCalledWith('conv1'));
  });

  it('mensagem que falhou mostra "Reenviar" e reenvia', async () => {
    const cliente = clienteChat([mensagem({ id: 'falha', texto: 'não foi', estado: 'falhou', erro: 'Sem conexão' })]);
    renderizar(<Chat conversaId="conv1" />, { cliente });
    expect(await screen.findByText('Sem conexão')).toBeTruthy();
    expect(screen.getByRole('img', { name: 'Não enviada' })).toBeTruthy();
    await userEvent.click(screen.getByRole('button', { name: 'Reenviar' }));
    await waitFor(() => expect(cliente.reenviarMensagem).toHaveBeenCalledWith('falha'));
    expect(await screen.findByRole('img', { name: 'Enviando' })).toBeTruthy();
  });

  it('Enter envia e Shift+Enter quebra linha', async () => {
    const cliente = clienteChat();
    cliente.enviarMensagem = vi.fn(async () => mensagem({ id: 'nova', texto: 'linha 1\nlinha 2', estado: 'pendente' }));
    renderizar(<Chat conversaId="conv1" />, { cliente });
    const campo = await screen.findByRole('textbox', { name: 'Mensagem' });
    await userEvent.type(campo, 'linha 1{Shift>}{Enter}{/Shift}linha 2');
    expect((campo as HTMLTextAreaElement).value).toBe('linha 1\nlinha 2');
    await userEvent.type(campo, '{Enter}');
    await waitFor(() =>
      expect(cliente.enviarMensagem).toHaveBeenCalledWith('conv1', expect.objectContaining({ texto: 'linha 1\nlinha 2' })),
    );
    await waitFor(() => expect((campo as HTMLTextAreaElement).value).toBe(''));
  });
});

describe('ConectarConta', () => {
  it('gera o QR pelo evento conta.qr e mostra o sucesso ao conectar', async () => {
    let contas = [conta({ id: 'antiga' })];
    const cliente = clienteSimulado({
      listarContas: async () => contas,
      criarConta: async () => {
        const nova = conta({ id: 'nova', nome: 'Nova', estado: 'conectando', telefone: null });
        contas = [...contas, nova];
        return nova;
      },
      obterQr: async () => {
        throw new ErroMotor('nao_encontrado', 'Sem QR', 404);
      },
    });
    const { fonte } = renderizar(<ConectarConta />, { cliente, rota: '/contas/conectar' });

    await userEvent.click(await screen.findByRole('button', { name: /Gerar QR code/ }));
    expect(await screen.findByText('Gerando QR code…')).toBeTruthy();

    act(() => {
      fonte.emitir({ seq: 1, tipo: 'conta.qr', conta_id: 'nova', em: '', dados: { codigo: '2@abc', expira_em: '' } });
    });
    expect(await screen.findByAltText('QR code para conectar o WhatsApp')).toBeTruthy();
    expect(screen.getByText(TEXTO_QR)).toBeTruthy();

    act(() => {
      fonte.emitir({ seq: 2, tipo: 'conta.qr_expirado', conta_id: 'nova', em: '', dados: {} });
    });
    expect(await screen.findByText('O QR code expirou.')).toBeTruthy();

    act(() => {
      fonte.emitir({
        seq: 3,
        tipo: 'conta.atualizada',
        conta_id: 'nova',
        em: '',
        dados: conta({ id: 'nova', nome: 'Nova', estado: 'conectada', sincronizando: true }),
      });
      fonte.emitir({ seq: 4, tipo: 'sincronizacao.progresso', conta_id: 'nova', em: '', dados: { conversas: 12, mensagens: 340, concluida: false } });
    });
    expect(await screen.findByText('Sincronizando histórico…')).toBeTruthy();
    expect(screen.getByText('12 conversas · 340 mensagens')).toBeTruthy();

    act(() => {
      fonte.emitir({ seq: 5, tipo: 'sincronizacao.progresso', conta_id: 'nova', em: '', dados: { conversas: 12, mensagens: 400, concluida: true } });
    });
    expect(await screen.findByText('Conta conectada')).toBeTruthy();
  });
});

describe('NovaConversa', () => {
  it('avisa "Este número não tem WhatsApp"', async () => {
    const cliente = clienteSimulado({
      abrirConversa: async () => {
        throw new ErroMotor('sem_whatsapp', 'Número sem WhatsApp', 422);
      },
    });
    renderizar(<NovaConversa contaId="conta1" aoFechar={() => undefined} aoAbrir={() => undefined} />, { cliente });
    await userEvent.type(screen.getByLabelText(/Número de telefone/), '11 99999-0000');
    await userEvent.click(screen.getByRole('button', { name: 'Conversar' }));
    expect(await screen.findByText(SEM_WHATSAPP)).toBeTruthy();
    expect(cliente.abrirConversa).toHaveBeenCalledWith('conta1', '11 99999-0000');
  });
});

describe('atalhos', () => {
  const tecla = (key: string, extra: Partial<Record<'metaKey' | 'ctrlKey' | 'altKey' | 'shiftKey', boolean>> = {}) => ({
    key,
    metaKey: false,
    ctrlKey: false,
    altKey: false,
    shiftKey: false,
    ...extra,
  });
  it('interpreta Cmd+F, Cmd+N e Cmd+↑/↓', () => {
    expect(interpretarAtalho(tecla('f', { metaKey: true }))).toBe('buscar');
    expect(interpretarAtalho(tecla('n', { metaKey: true }))).toBe('nova_conversa');
    expect(interpretarAtalho(tecla('ArrowUp', { metaKey: true }))).toBe('conversa_anterior');
    expect(interpretarAtalho(tecla('ArrowDown', { metaKey: true }))).toBe('proxima_conversa');
    expect(interpretarAtalho(tecla('f'))).toBeNull();
  });
  it('troca de conversa sem dar a volta', () => {
    expect(vizinho(['a', 'b', 'c'], 'b', 1)).toBe('c');
    expect(vizinho(['a', 'b', 'c'], 'c', 1)).toBe('c');
    expect(vizinho(['a', 'b', 'c'], undefined, 1)).toBe('a');
    expect(vizinho([], 'a', 1)).toBeNull();
  });
});

describe('App (estado do motor)', () => {
  function ponte(estado: EstadoMotor) {
    return {
      obterConexao: vi.fn(async () => estado),
      aoMudarEstadoMotor: vi.fn(() => () => undefined),
      reiniciarMotor: vi.fn(async () => undefined),
      requisitarMotor: vi.fn(),
      aoEventoMotor: vi.fn(() => () => undefined),
      aoRecarregar: vi.fn(() => () => undefined),
    };
  }

  it('mostra "Ligando o WhatsApp…" enquanto o motor sobe', async () => {
    render(<App ponte={ponte({ fase: 'ligando' })} />);
    expect(await screen.findByText('Ligando o WhatsApp…')).toBeTruthy();
  });

  it('erro do motor: motivo e "Tentar de novo" reinicia via IPC', async () => {
    const p = ponte({ fase: 'erro', mensagem: 'Não consegui ligar o WhatsApp.', detalhe: 'Motor não encontrado.' });
    render(<App ponte={p} />);
    const alerta = await screen.findByRole('alert');
    expect(within(alerta).getByText('Não consegui ligar o WhatsApp.')).toBeTruthy();
    expect(within(alerta).getByText('Motor não encontrado.')).toBeTruthy();
    fireEvent.click(within(alerta).getByRole('button', { name: 'Tentar de novo' }));
    expect(p.reiniciarMotor).toHaveBeenCalled();
  });

  it('religando mostra "O WhatsApp parou. Religando…"', async () => {
    render(<App ponte={ponte({ fase: 'religando' })} />);
    expect(await screen.findByText('O WhatsApp parou. Religando…')).toBeTruthy();
  });
});
