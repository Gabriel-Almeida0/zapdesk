// T127 — US5: atalho `/` no composer e filtro por etiqueta na lista de conversas.
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { Composer } from '../../src/renderer/componentes/Composer';
import { termoAtalho } from '../../src/renderer/componentes/SugestoesTemplate';
import { Conversas } from '../../src/renderer/telas/Conversas';
import { validarEtiqueta } from '../../src/renderer/telas/Etiquetas';
import { extrairVariaveis } from '../../src/renderer/util/formatar';
import { clienteSimulado, conversa, pagina, renderizar, template } from './utilitarios';

function composer() {
  return (
    <Composer
      conversa={conversa()}
      citando={null}
      aoLimparCitacao={vi.fn()}
      editando={null}
      aoLimparEdicao={vi.fn()}
      arquivosSoltos={[]}
      aoConsumirArquivos={vi.fn()}
    />
  );
}

describe('atalho / no composer', () => {
  it('sugere templates pelo nome e insere texto e anexo para revisão', async () => {
    const cliente = clienteSimulado({
      listarTemplates: async () => [
        template({
          arquivo: { id: 'arq1', nome: 'tabela.pdf', mimetype: 'application/pdf', tamanho: 2048, tipo_midia: 'documento', url: '/v1/arquivos/arq1/conteudo' },
        }),
      ],
    });
    renderizar(composer(), { cliente });
    const campo = screen.getByRole('textbox', { name: 'Mensagem' }) as HTMLTextAreaElement;
    await userEvent.type(campo, '/apre');
    expect(await screen.findByRole('option', { name: /\/Apresentação/ })).toBeTruthy();
    await waitFor(() => expect(cliente.listarTemplates).toHaveBeenLastCalledWith('apre'));
    await userEvent.keyboard('{Enter}');
    expect(campo.value).toBe('Oi {nome}, aqui é o Gabriel.');
    expect(screen.getByText('tabela.pdf')).toBeTruthy();
    // Nada foi enviado: fica para revisão.
    expect(cliente.enviarMensagem).not.toHaveBeenCalled();
  });

  it('termoAtalho só reconhece "/" no começo e sem espaço', () => {
    expect(termoAtalho('/apre')).toBe('apre');
    expect(termoAtalho('/')).toBe('');
    expect(termoAtalho('oi /apre')).toBeNull();
    expect(termoAtalho('/apre x')).toBeNull();
  });
});

describe('filtro por etiqueta', () => {
  it('filtra a lista de conversas pela etiqueta escolhida', async () => {
    const quente = { id: 'e1', nome: 'Quente', cor: '#ff0000', total_contatos: 3 };
    const cliente = clienteSimulado({
      listarEtiquetas: async () => [quente],
      listarConversas: async () => pagina([conversa({ etiquetas: [quente] })]),
    });
    renderizar(<Conversas />, { cliente, rota: '/conversas', caminho: '/conversas' });
    await screen.findByText('Ana');
    await screen.findByRole('option', { name: 'Quente' });
    await userEvent.selectOptions(screen.getByLabelText('Filtrar por etiqueta'), 'e1');
    await waitFor(() =>
      expect(cliente.listarConversas).toHaveBeenLastCalledWith('conta1', expect.objectContaining({ etiqueta_id: 'e1' })),
    );
  });
});

describe('validações de organização', () => {
  it('etiqueta: nome 1–30 e cor #RRGGBB', () => {
    expect(validarEtiqueta('', '#ffffff')).toBeTruthy();
    expect(validarEtiqueta('x'.repeat(31), '#ffffff')).toBeTruthy();
    expect(validarEtiqueta('Quente', 'vermelho')).toBeTruthy();
    expect(validarEtiqueta('Quente', '#FF0000')).toBeNull();
  });

  it('variáveis detectadas nos templates (escapes e acentos)', () => {
    expect(extrairVariaveis('Oi {Nome}, {{literal}} de {Cidade Natal} {ação}')).toEqual(['nome', 'cidade_natal', 'acao']);
  });
});
