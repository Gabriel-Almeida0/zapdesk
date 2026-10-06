// O menu da mensagem abre para cima quando não cabe abaixo (mensagens no fim da conversa).
import { describe, expect, it } from 'vitest';

import { abrirParaCima } from '../../src/renderer/componentes/MenuMensagem';

describe('abrirParaCima', () => {
  const area = { top: 64, bottom: 760 };
  it('abre para baixo com espaço sobrando', () => {
    expect(abrirParaCima({ top: 200, bottom: 220 }, area)).toBe(false);
  });
  it('abre para cima na última mensagem da lista', () => {
    expect(abrirParaCima({ top: 690, bottom: 710 }, area)).toBe(true);
  });
  it('lista curta sem espaço acima: continua para baixo', () => {
    expect(abrirParaCima({ top: 80, bottom: 100 }, { top: 64, bottom: 200 })).toBe(false);
  });
});
