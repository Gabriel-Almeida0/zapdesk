// 003 T039 (US5) — Ajustes → Aparência: Sistema / Claro / Escuro, troca na hora e lembrada.
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { Aparencia } from '../../src/renderer/telas/Ajustes';
import { CHAVE_TEMA, EVENTO_TEMA } from '../../src/renderer/util/tema';

beforeEach(() => {
  localStorage.clear();
  document.documentElement.removeAttribute('data-tema');
  vi.stubGlobal(
    'matchMedia',
    vi.fn(() => ({ matches: false, media: '', addEventListener: () => {}, removeEventListener: () => {} })),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('Ajustes → Aparência', () => {
  it('mostra o grupo com as 3 opções e "Sistema" marcado por padrão', () => {
    render(<Aparencia />);
    expect(screen.getByRole('heading', { name: 'Aparência' })).toBeTruthy();
    const grupo = screen.getByRole('radiogroup', { name: 'Aparência' });
    expect(grupo).toBeTruthy();
    expect((screen.getByRole('radio', { name: 'Sistema' }) as HTMLInputElement).checked).toBe(true);
    expect((screen.getByRole('radio', { name: 'Claro' }) as HTMLInputElement).checked).toBe(false);
    expect((screen.getByRole('radio', { name: 'Escuro' }) as HTMLInputElement).checked).toBe(false);
  });

  it('escolher "Escuro" aplica na hora, avisa e grava a preferência', async () => {
    const aviso = vi.fn();
    window.addEventListener(EVENTO_TEMA, aviso);
    render(<Aparencia />);
    await userEvent.click(screen.getByRole('radio', { name: 'Escuro' }));
    expect(document.documentElement.getAttribute('data-tema')).toBe('escuro');
    expect(localStorage.getItem(CHAVE_TEMA)).toBe('escuro');
    expect(aviso).toHaveBeenCalled();
    expect((screen.getByRole('radio', { name: 'Escuro' }) as HTMLInputElement).checked).toBe(true);
    window.removeEventListener(EVENTO_TEMA, aviso);
  });

  it('abre com a preferência salva marcada', () => {
    localStorage.setItem(CHAVE_TEMA, 'claro');
    render(<Aparencia />);
    expect((screen.getByRole('radio', { name: 'Claro' }) as HTMLInputElement).checked).toBe(true);
  });

  it('voltar a "Sistema" apaga a chave e o atributo', async () => {
    localStorage.setItem(CHAVE_TEMA, 'escuro');
    document.documentElement.setAttribute('data-tema', 'escuro');
    render(<Aparencia />);
    await userEvent.click(screen.getByRole('radio', { name: 'Sistema' }));
    expect(localStorage.getItem(CHAVE_TEMA)).toBeNull();
    expect(document.documentElement.hasAttribute('data-tema')).toBe(false);
  });
});
