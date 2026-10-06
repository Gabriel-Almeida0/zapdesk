// T056 — evento `notificacao` vira notificação do macOS; clique leva à conversa/automação.
import { describe, expect, it, vi } from 'vitest';

import { Notificador, type NotificacaoSistema } from '../../src/main/notificacoes';

describe('Notificador', () => {
  it('mostra e repassa o alvo no clique', () => {
    const ouvintes: Record<string, () => void> = {};
    const criada: NotificacaoSistema = {
      on: (evento, ouvinte) => {
        ouvintes[evento] = ouvinte;
      },
      show: vi.fn(),
    };
    const criar = vi.fn(() => criada);
    const aoClicar = vi.fn();
    const notificador = new Notificador({ suportada: () => true, criar }, aoClicar);
    expect(
      notificador.mostrar({ titulo: 'Anti-loop', corpo: 'Automações pausadas na conversa com Ana', automacao_id: 'a1', conversa_id: 'c1', tipo: 'anti_loop' }),
    ).toBe(true);
    expect(criar).toHaveBeenCalledWith(expect.objectContaining({ title: 'Anti-loop', body: 'Automações pausadas na conversa com Ana' }));
    expect(criada.show).toHaveBeenCalled();
    ouvintes['click']?.();
    expect(aoClicar).toHaveBeenCalledWith({ conversa_id: 'c1', automacao_id: 'a1' });
  });

  it('sem suporte a notificações não faz nada', () => {
    const criar = vi.fn();
    const notificador = new Notificador({ suportada: () => false, criar }, vi.fn());
    expect(notificador.mostrar({ titulo: 'x', corpo: 'y', automacao_id: null, conversa_id: null, tipo: 'erro' })).toBe(false);
    expect(criar).not.toHaveBeenCalled();
  });
});
