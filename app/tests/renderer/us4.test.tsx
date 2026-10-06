// T116 — US4: menu da mensagem respeita prazos; bolha com erro de download; reações e apagada.
import { fireEvent, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { Bolha } from '../../src/renderer/componentes/Bolha';
import { TEXTO_FALHA_DOWNLOAD, TEXTO_TENTAR_BAIXAR } from '../../src/renderer/componentes/bolhas/Midia';
import { acoesDisponiveis } from '../../src/renderer/componentes/MenuMensagem';
import { checarLimiteAnexo } from '../../src/renderer/util/formatar';
import { clienteSimulado, mensagem, renderizar } from './utilitarios';

const acoes = () => ({
  aoResponder: vi.fn(),
  aoReagir: vi.fn(),
  aoEditar: vi.fn(),
  aoApagar: vi.fn(),
  aoReenviar: vi.fn(),
});

async function abrirMenu() {
  await userEvent.click(screen.getByRole('button', { name: 'Ações da mensagem' }));
}

describe('menu da mensagem', () => {
  it('dentro do prazo: editar e apagar disponíveis', async () => {
    const a = acoes();
    renderizar(<Bolha mensagem={mensagem()} grupo={false} {...a} />, { cliente: clienteSimulado() });
    await abrirMenu();
    await userEvent.click(screen.getByRole('menuitem', { name: /Editar/ }));
    expect(a.aoEditar).toHaveBeenCalled();
    await abrirMenu();
    await userEvent.click(screen.getByRole('menuitem', { name: /Apagar para todos/ }));
    expect(a.aoApagar).toHaveBeenCalled();
  });

  it('fora do prazo: editar e apagar ficam indisponíveis', async () => {
    renderizar(<Bolha mensagem={mensagem({ pode_editar: false, pode_apagar: false })} grupo={false} {...acoes()} />, {
      cliente: clienteSimulado(),
    });
    await abrirMenu();
    expect((screen.getByRole('menuitem', { name: /Editar/ }) as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByRole('menuitem', { name: /Apagar para todos/ }) as HTMLButtonElement).disabled).toBe(true);
    // Responder e reagir continuam.
    expect((screen.getByRole('menuitem', { name: /Responder/ }) as HTMLButtonElement).disabled).toBe(false);
  });

  it('mensagem de outra pessoa não tem editar nem apagar; reagir envia o emoji', async () => {
    const a = acoes();
    const m = mensagem({ de_mim: false, estado: 'recebida' });
    renderizar(<Bolha mensagem={m} grupo={false} {...a} />, { cliente: clienteSimulado() });
    await abrirMenu();
    expect(screen.queryByRole('menuitem', { name: /Editar/ })).toBeNull();
    expect(screen.queryByRole('menuitem', { name: /Apagar/ })).toBeNull();
    await userEvent.click(screen.getByRole('menuitem', { name: 'Reagir com 👍' }));
    expect(a.aoReagir).toHaveBeenCalledWith(m, '👍');
  });

  it('regras: só texto meu edita; apagada/pendente sem ações', () => {
    expect(acoesDisponiveis(mensagem({ tipo: 'imagem' })).editar).toBe(false);
    expect(acoesDisponiveis(mensagem({ apagada: true })).responder).toBe(false);
    expect(acoesDisponiveis(mensagem({ estado: 'pendente' })).reagir).toBe(false);
  });
});

describe('bolhas de mídia', () => {
  const imagem = mensagem({
    id: 'img1',
    tipo: 'imagem',
    texto: 'foto',
    midia: {
      mimetype: 'image/jpeg',
      tamanho: 1000,
      nome_arquivo: null,
      duracao_s: null,
      ptt: false,
      largura: 400,
      altura: 300,
      miniatura_b64: null,
      baixada: false,
      url: '/v1/mensagens/img1/midia',
    },
  });

  it('falha no download mostra "Não foi possível baixar." e "Tentar baixar de novo"', async () => {
    renderizar(<Bolha mensagem={imagem} grupo={false} {...acoes()} />, { cliente: clienteSimulado() });
    const img = screen.getByAltText('foto');
    expect(img.getAttribute('src')).toBe('http://127.0.0.1:1/v1/mensagens/img1/midia?token=t');
    fireEvent.error(img);
    expect(await screen.findByText(TEXTO_FALHA_DOWNLOAD)).toBeTruthy();
    await userEvent.click(screen.getByRole('button', { name: new RegExp(TEXTO_TENTAR_BAIXAR) }));
    expect(screen.getByAltText('foto').getAttribute('src')).toContain('tentativa=1');
  });

  it('apagada, editada, reações e citação', () => {
    renderizar(
      <>
        <Bolha mensagem={mensagem({ id: 'x', apagada: true, de_mim: false })} grupo={false} {...acoes()} />
        <Bolha
          mensagem={mensagem({
            id: 'y',
            texto: 'nova versão',
            editada: true,
            reacoes: [
              { remetente_jid: 'a', emoji: '❤️', de_mim: false },
              { remetente_jid: 'b', emoji: '❤️', de_mim: true },
            ],
            citacao: { wa_id: 'W0', resumo: 'mensagem antiga', remetente_nome: 'Ana' },
          })}
          grupo={false}
          {...acoes()}
        />
      </>,
      { cliente: clienteSimulado() },
    );
    expect(screen.getByText('Mensagem apagada')).toBeTruthy();
    expect(screen.getByText('editada')).toBeTruthy();
    expect(screen.getByText('mensagem antiga')).toBeTruthy();
    expect(screen.getByLabelText('Reações').textContent).toContain('❤️2');
  });
});

describe('limites de anexo', () => {
  it('recusa acima do limite do WhatsApp', () => {
    const mb = 1024 * 1024;
    expect(checarLimiteAnexo({ size: 17 * mb, type: 'video/mp4' })).toBe('Arquivo grande demais: o limite para vídeo é 16 MB.');
    expect(checarLimiteAnexo({ size: 17 * mb, type: 'application/pdf' })).toBeNull();
    expect(checarLimiteAnexo({ size: 2 * mb, type: 'image/webp' })).toContain('figurinha é 1 MB');
  });
});
