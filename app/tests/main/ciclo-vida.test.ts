// T102 — lógica de fechar/sair com e sem disparos ativos.
import { describe, expect, it } from 'vitest';

import type { Disparo } from '@zapdesk/cliente-motor';

import {
  AVISO_SEGUNDO_PLANO,
  CicloVida,
  disparoAtivo,
  ESPERA_RESUMO_MS,
  resumirProgresso,
  type AcoesCiclo,
} from '../../src/main/ciclo-vida';

function criar() {
  const registro: string[] = [];
  const agendados: { fn: () => void; ms: number }[] = [];
  const acoes: AcoesCiclo = {
    esconderJanela: () => registro.push('esconderJanela'),
    mostrarJanela: () => registro.push('mostrarJanela'),
    ocultarDock: () => registro.push('ocultarDock'),
    mostrarDock: () => registro.push('mostrarDock'),
    criarBandeja: () => registro.push('criarBandeja'),
    atualizarBandeja: (r) => registro.push(`bandeja:${r.texto}`),
    destruirBandeja: () => registro.push('destruirBandeja'),
    avisar: (m) => registro.push(`avisar:${m}`),
    notificar: (t, c) => registro.push(`notificar:${t}:${c}`),
    encerrar: () => registro.push('encerrar'),
    agendar: (fn, ms) => agendados.push({ fn, ms }),
  };
  const ciclo = new CicloVida(acoes);
  const rodarAgendados = () => agendados.splice(0).forEach((a) => a.fn());
  return { ciclo, registro, agendados, rodarAgendados };
}

function disparo(parcial: Partial<Disparo> & { id: string }): Disparo {
  return {
    conta_id: 'c1',
    nome: 'Teste',
    mensagem: 'Oi',
    arquivo: null,
    ritmo: { intervalo_min_s: 1, intervalo_max_s: 2, limite_por_hora: null, limite_por_dia: null, pausa_a_cada: null, pausa_duracao_s: null },
    inicio_em: null,
    janela: null,
    falhas_seguidas_max: 10,
    valores_padrao: {},
    estado: 'enviando',
    na_fila: false,
    motivo_pausa: null,
    origem: 'app',
    contadores: { total: 10, pendente: 6, enviando: 1, enviado: 3, entregue: 0, lido: 0, respondeu: 0, falhou: 0 },
    proximo_envio_em: null,
    estimativa_termino_em: null,
    aviso_ritmo_agressivo: false,
    criado_em: '2026-09-27T10:00:00-03:00',
    iniciado_em: '2026-09-27T10:00:00-03:00',
    concluido_em: null,
    cancelado_em: null,
    ...parcial,
  };
}

describe('CicloVida', () => {
  it('fechar sem disparo ativo encerra tudo', () => {
    const { ciclo, registro } = criar();
    expect(ciclo.pedirFechar()).toBe('encerrar');
    expect(registro).toContain('encerrar');
    expect(ciclo.estaEncerrando).toBe(true);
    // Cmd+Q repetido durante o encerramento é ignorado.
    expect(ciclo.pedirFechar()).toBe('ignorar');
    expect(registro.filter((r) => r === 'encerrar')).toHaveLength(1);
  });

  it('fechar com disparo ativo: avisa, esconde janela e dock e mostra a bandeja', () => {
    const { ciclo, registro } = criar();
    ciclo.atualizarAtivos(1);
    expect(ciclo.pedirFechar()).toBe('segundo_plano');
    expect(registro).toEqual([
      `avisar:${AVISO_SEGUNDO_PLANO}`,
      'esconderJanela',
      'ocultarDock',
      'criarBandeja',
      'bandeja:Nenhum disparo em andamento',
    ]);
    expect(ciclo.emSegundoPlano).toBe(true);
    expect(registro).not.toContain('encerrar');
  });

  it('fim do disparo em segundo plano: notifica o resumo e encerra', () => {
    const { ciclo, registro, agendados, rodarAgendados } = criar();
    ciclo.atualizarAtivos(1);
    ciclo.pedirFechar();
    ciclo.atualizarDisparo(disparo({ id: 'd1' }));
    expect(registro.at(-1)).toBe('bandeja:Disparo em andamento: 3 de 10');

    ciclo.atualizarAtivos(0);
    ciclo.registrarFinalizado('Disparo concluído: 480 enviados, 12 falharam', 'd1');
    expect(agendados[0]?.ms).toBe(ESPERA_RESUMO_MS);
    rodarAgendados();
    expect(registro).toContain('notificar:ZapDesk:Disparo concluído: 480 enviados, 12 falharam');
    expect(registro.at(-1)).toBe('encerrar');
  });

  it('não encerra se um disparo novo começou durante a espera do resumo', () => {
    const { ciclo, registro, rodarAgendados } = criar();
    ciclo.atualizarAtivos(1);
    ciclo.pedirFechar();
    ciclo.atualizarAtivos(0);
    ciclo.atualizarAtivos(1);
    rodarAgendados();
    expect(registro).not.toContain('encerrar');
  });

  it('reabrir pela bandeja volta ao normal; fechar de novo sem ativos encerra', () => {
    const { ciclo, registro } = criar();
    ciclo.atualizarAtivos(2);
    ciclo.pedirFechar();
    registro.length = 0;
    ciclo.reabrir();
    expect(registro).toEqual(['destruirBandeja', 'mostrarDock', 'mostrarJanela']);
    expect(ciclo.emSegundoPlano).toBe(false);
    // Ativos chegando a 0 com a janela aberta não fecham o app.
    ciclo.atualizarAtivos(0);
    expect(registro).not.toContain('encerrar');
    expect(ciclo.pedirFechar()).toBe('encerrar');
  });

  it('"Sair agora" na bandeja encerra mesmo com disparo ativo', () => {
    const { ciclo, registro } = criar();
    ciclo.atualizarAtivos(1);
    ciclo.pedirFechar();
    ciclo.forcarSaida();
    expect(registro.at(-1)).toBe('encerrar');
  });

  it('motor falhou de vez em segundo plano → notifica e sai', () => {
    const { ciclo, registro } = criar();
    ciclo.atualizarAtivos(1);
    ciclo.pedirFechar();
    ciclo.motorFalhou('Motor parou.');
    expect(registro.some((r) => r.startsWith('notificar:ZapDesk:O WhatsApp parou'))).toBe(true);
    expect(registro.at(-1)).toBe('encerrar');
  });
});

describe('progresso', () => {
  it('pausado e rascunho não contam como ativos', () => {
    expect(disparoAtivo(disparo({ id: 'a', estado: 'pausado' }))).toBe(false);
    expect(disparoAtivo(disparo({ id: 'a', estado: 'agendado', iniciado_em: null }))).toBe(false);
    expect(disparoAtivo(disparo({ id: 'a', estado: 'agendado' }))).toBe(true);
    expect(disparoAtivo(disparo({ id: 'a', estado: 'fora_da_janela' }))).toBe(true);
  });

  it('soma vários disparos', () => {
    const r = resumirProgresso([disparo({ id: 'a' }), disparo({ id: 'b' })]);
    expect(r).toMatchObject({ disparos: 2, processados: 6, total: 20, percentual: 30 });
    expect(r.texto).toBe('2 disparos em andamento: 6 de 20');
  });
});
