// T078 — US2: bloqueio sem coluna de telefone, renderização do relatório e tela de Leads.
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { RelatorioImportacao } from '../../src/renderer/componentes/RelatorioImportacao';
import { ERRO_SEM_TELEFONE, FormArquivo, FormColar } from '../../src/renderer/telas/ImportarLeads';
import { Leads, VAZIO_LEADS } from '../../src/renderer/telas/Leads';
import { clienteSimulado, pagina, relatorio, renderizar } from './utilitarios';

const PREVIA = {
  importacao_id: 'imp1',
  nome_arquivo: 'leads.csv',
  colunas: ['Contato', 'Empresa', 'Cidade'],
  amostra: [['Ana', 'X', 'SP']],
  total_linhas: 3,
  coluna_telefone_sugerida: null,
  coluna_nome_sugerida: 'Contato',
  expira_em: '2026-09-27T11:00:00-03:00',
};

describe('Importar leads — arquivo', () => {
  it('bloqueia a importação sem coluna de telefone', async () => {
    const cliente = clienteSimulado({ previaImportacao: async () => PREVIA, importarLeads: async () => relatorio() });
    renderizar(<FormArquivo aoImportar={() => undefined} />, { cliente });
    const arquivo = new File(['Contato;Empresa;Cidade\nAna;X;SP'], 'leads.csv', { type: 'text/csv' });
    await userEvent.upload(screen.getByLabelText('Planilha de leads'), arquivo);
    await userEvent.click(await screen.findByRole('button', { name: 'Importar 3 linhas' }));
    expect(await screen.findByText(ERRO_SEM_TELEFONE)).toBeTruthy();
    expect(cliente.importarLeads).not.toHaveBeenCalled();
  });

  it('com o telefone mapeado envia o mapeamento com nome e extras', async () => {
    const cliente = clienteSimulado({
      previaImportacao: async () => ({ ...PREVIA, colunas: ['Celular', 'Contato', 'Empresa'], coluna_telefone_sugerida: 'Celular' }),
      importarLeads: async () => relatorio({ total_linhas: 3, total_novos: 3 }),
    });
    let recebido = false;
    renderizar(<FormArquivo aoImportar={() => (recebido = true)} />, { cliente });
    await userEvent.upload(screen.getByLabelText('Planilha de leads'), new File(['x'], 'leads.csv', { type: 'text/csv' }));
    await userEvent.click(await screen.findByRole('button', { name: 'Importar 3 linhas' }));
    await waitFor(() =>
      expect(cliente.importarLeads).toHaveBeenCalledWith({
        importacao_id: 'imp1',
        mapeamento: { telefone: 'Celular', nome: 'Contato', extras: ['Empresa'] },
      }),
    );
    await waitFor(() => expect(recebido).toBe(true));
  });

  it('colar números conta as linhas e envia o texto', async () => {
    const cliente = clienteSimulado({ importarLeads: async () => relatorio() });
    renderizar(<FormColar aoImportar={() => undefined} />, { cliente });
    await userEvent.type(screen.getByRole('textbox'), '11 99999-0000{Enter}{Enter}21 98888-7777');
    await userEvent.click(screen.getByRole('button', { name: 'Importar 2 números' }));
    await waitFor(() => expect(cliente.importarLeads).toHaveBeenCalledWith({ texto_colado: '11 99999-0000\n\n21 98888-7777' }));
  });
});

describe('RelatorioImportacao', () => {
  it('mostra novos, já existentes com data original, inválidos com motivo e duplicados', async () => {
    const r = relatorio({
      total_linhas: 1000,
      total_novos: 856,
      total_ja_existentes: 32,
      total_invalidos: 12,
      total_duplicados_no_lote: 100,
      ja_existentes: [
        { linha: 4, lead_id: 'l4', telefone: '+5511999990000', importado_em: '2026-09-10T10:00:00-03:00', campos_preenchidos: ['empresa'] },
      ],
      invalidos: [{ linha: 7, valor: 'abc', motivo: 'formato_invalido' }],
      duplicados_no_lote: [{ linha: 9, telefone: '+5511988887777', primeira_linha: 2 }],
      lead_ids: Array.from({ length: 888 }, (_, i) => `l${i}`),
    });
    renderizar(<RelatorioImportacao relatorio={r} />, { cliente: clienteSimulado() });
    expect(screen.getByText('32 números já estavam na sua base')).toBeTruthy();
    expect(screen.getByText('12 números inválidos')).toBeTruthy();
    expect(screen.getByText('856 leads novos')).toBeTruthy();
    expect(screen.getByText('100 números repetidos na lista')).toBeTruthy();
    expect(screen.getByText('10/09/2026')).toBeTruthy();
    expect(screen.getByText('formato inválido')).toBeTruthy();
    expect(screen.getByText(/888 números válidos/)).toBeTruthy();
  });
});

describe('Leads', () => {
  it('base vazia: "Nenhum lead — importe um CSV ou use o MCP"', async () => {
    const cliente = clienteSimulado({ listarLeads: async () => pagina([]) });
    renderizar(<Leads />, { cliente });
    expect(await screen.findByText(VAZIO_LEADS)).toBeTruthy();
  });

  it('lista telefone, origem e tem WhatsApp', async () => {
    const cliente = clienteSimulado({
      listarLeads: async () =>
        pagina([
          {
            id: 'l1',
            telefone: '+5511999990000',
            nome: 'Ana',
            campos: { empresa: 'X' },
            origem: 'mcp',
            tem_whatsapp: false,
            importado_em: '2026-09-10T10:00:00-03:00',
            ultimo_disparo_em: null,
          },
        ]),
    });
    renderizar(<Leads />, { cliente });
    expect(await screen.findByText('+55 11 99999-0000')).toBeTruthy();
    expect(screen.getByText('IA (MCP)', { selector: 'span' })).toBeTruthy();
    expect(screen.getByText('Não')).toBeTruthy();
  });
});
