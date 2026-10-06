// T103 — US3: validação de ritmo, bloqueio por variáveis faltando, aviso de ritmo agressivo e textos
// de estado do disparo.
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { NovoDisparo } from '../../src/renderer/telas/NovoDisparo';
import { AVISO_RITMO, PassoRitmo } from '../../src/renderer/telas/NovoDisparo/PassoRitmo';
import {
  descreverEstado,
  formInicial,
  montarNovoDisparo,
  ritmoAgressivo,
  validarRitmoForm,
  type FormDisparo,
} from '../../src/renderer/util/disparos';
import { clienteSimulado, disparo, pagina, renderizar } from './utilitarios';

function form(parcial: Partial<FormDisparo> = {}, ritmo: Partial<FormDisparo['ritmo']> = {}): FormDisparo {
  const base = formInicial('conta1');
  return { ...base, ...parcial, ritmo: { ...base.ritmo, ...ritmo } };
}

describe('validação do ritmo (mesmas regras do motor)', () => {
  it('intervalo mínimo ≥ 1 e máximo ≥ mínimo', () => {
    expect(validarRitmoForm(form({}, { intervaloMin: '0' }))['intervalo_min_s']).toBeTruthy();
    expect(validarRitmoForm(form({}, { intervaloMin: '30', intervaloMax: '10' }))['intervalo_max_s']).toBeTruthy();
    expect(validarRitmoForm(form({}, { intervaloMin: '1', intervaloMax: '1' }))).toEqual({});
  });

  it('limites > 0 e pausa em par', () => {
    expect(validarRitmoForm(form({}, { limiteHora: '0' }))['limite_por_hora']).toBeTruthy();
    expect(validarRitmoForm(form({}, { pausaACada: '50' }))['pausa_a_cada']).toBeTruthy();
    expect(validarRitmoForm(form({}, { pausaACada: '50', pausaDuracaoMin: '10' }))).toEqual({});
  });

  it('janela com horários diferentes; falhas seguidas ≥ 0', () => {
    expect(validarRitmoForm(form({ usarJanela: true, janelaInicio: '09:00', janelaFim: '09:00' }))['janela']).toBeTruthy();
    expect(validarRitmoForm(form({ usarJanela: true, janelaInicio: '22:00', janelaFim: '06:00' }))).toEqual({});
    expect(validarRitmoForm(form({ falhasSeguidas: '-1' }))['falhas_seguidas_max']).toBeTruthy();
    expect(validarRitmoForm(form({ falhasSeguidas: '0' }))).toEqual({});
  });

  it('monta o NovoDisparo do contrato (pausa em segundos, janela, origem app)', () => {
    const novo = montarNovoDisparo(
      form({ leadIds: ['a'], mensagem: 'Oi', usarJanela: true, valoresPadrao: { nome: 'tudo bem', cidade: ' ' } }, { pausaACada: '50', pausaDuracaoMin: '10', limiteHora: '40' }),
      true,
    );
    expect(novo).toMatchObject({
      conta_id: 'conta1',
      destinatarios: { lead_ids: ['a'] },
      ritmo: { intervalo_min_s: 30, intervalo_max_s: 90, limite_por_hora: 40, limite_por_dia: null, pausa_a_cada: 50, pausa_duracao_s: 600 },
      janela: { inicio: '09:00', fim: '18:00' },
      valores_padrao: { nome: 'tudo bem' },
      falhas_seguidas_max: 10,
      iniciar: true,
      origem: 'app',
    });
  });

  it('ritmo agressivo = intervalo < 10 s, > 120/h, > 1000/dia ou sem limites', () => {
    const r = { intervalo_min_s: 30, intervalo_max_s: 60, limite_por_hora: 40, limite_por_dia: null, pausa_a_cada: null, pausa_duracao_s: null };
    expect(ritmoAgressivo(r)).toBe(false);
    expect(ritmoAgressivo({ ...r, intervalo_min_s: 5 })).toBe(true);
    expect(ritmoAgressivo({ ...r, limite_por_hora: 200 })).toBe(true);
    expect(ritmoAgressivo({ ...r, limite_por_hora: null })).toBe(true);
  });
});

describe('PassoRitmo', () => {
  it('avisa sobre ritmo agressivo sem bloquear', () => {
    renderizar(<PassoRitmo form={form({}, { intervaloMin: '2', intervaloMax: '5' })} aoMudarForm={() => undefined} />, {
      cliente: clienteSimulado({ validarDisparo: async () => ({}) }),
    });
    expect(screen.getByText(AVISO_RITMO)).toBeTruthy();
  });
});

describe('estado do disparo', () => {
  it('textos de janela, fila e pausa', () => {
    expect(descreverEstado(disparo({ estado: 'fora_da_janela', janela: { inicio: '09:00', fim: '18:00' } }))).toBe('Aguardando janela (9h–18h)');
    expect(descreverEstado(disparo({ estado: 'agendado', na_fila: true }))).toBe('Na fila');
    expect(descreverEstado(disparo({ estado: 'pausado', motivo_pausa: 'app_fechado' }))).toBe('Disparo pausado porque o app foi fechado.');
    expect(descreverEstado(disparo({ estado: 'pausado', motivo_pausa: 'falhas_seguidas' }))).toBe('Disparo pausado após muitas falhas seguidas.');
  });
});

describe('Novo disparo', () => {
  it('bloqueia "Iniciar" com "5 contatos sem {nome}" até definir um valor padrão', async () => {
    const faltando = Array.from({ length: 5 }, (_, i) => ({ destinatario_linha: i + 1, telefone: `+551199999000${i}`, variaveis: ['nome'] }));
    const cliente = clienteSimulado({
      listarLeads: async () =>
        pagina([
          { id: 'l1', telefone: '+5511999990001', nome: null, campos: {}, origem: 'csv', tem_whatsapp: null, importado_em: '2026-09-10T10:00:00-03:00', ultimo_disparo_em: null },
        ]),
      obterLead: async () => ({ id: 'l1', telefone: '+5511999990001', nome: null, campos: { cidade: 'SP' }, origem: 'csv', tem_whatsapp: null, importado_em: '', ultimo_disparo_em: null }),
      listarTemplates: async () => [],
      validarDisparo: async () => ({
        total_destinatarios: 5,
        variaveis: ['nome'],
        faltando,
        previa: { telefone: '+5511999990001', nome: null, texto_resolvido: 'Oi , tudo bem?' },
        estimativa_termino_em: null,
        aviso_ritmo_agressivo: false,
        erros_campos: {},
      }),
      criarDisparo: async () => disparo(),
    });
    renderizar(<NovoDisparo />, { cliente });

    await userEvent.click(await screen.findByRole('checkbox', { name: /99999-0001/ }));
    await userEvent.click(screen.getByRole('button', { name: 'Continuar' }));
    await userEvent.type(await screen.findByPlaceholderText('Oi {nome}, tudo bem?'), 'Oi {{nome}, tudo bem?');
    await userEvent.click(screen.getByRole('button', { name: 'Continuar' }));
    await userEvent.click(screen.getByRole('button', { name: 'Continuar' }));

    expect(await screen.findByText('5 contatos sem {nome}', { selector: 'strong' }, { timeout: 3000 })).toBeTruthy();
    const iniciar = screen.getByRole('button', { name: 'Iniciar' });
    expect((iniciar as HTMLButtonElement).disabled).toBe(true);

    await userEvent.type(screen.getByLabelText('Valor padrão para {nome}'), 'tudo bem');
    await waitFor(() => expect((screen.getByRole('button', { name: 'Iniciar' }) as HTMLButtonElement).disabled).toBe(false));
    await userEvent.click(screen.getByRole('button', { name: 'Iniciar' }));
    await waitFor(() =>
      expect(cliente.criarDisparo).toHaveBeenCalledWith(
        expect.objectContaining({ iniciar: true, valores_padrao: { nome: 'tudo bem' }, destinatarios: { lead_ids: ['l1'] }, mensagem: 'Oi {nome}, tudo bem?' }),
      ),
    );
  });
});
