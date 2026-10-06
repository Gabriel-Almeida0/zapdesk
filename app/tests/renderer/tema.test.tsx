// 003 T021 (US5) — preferência de tema da janela (util/tema.ts; data-model §3).
import { act, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { aguardarFontes } from '../../src/renderer/util/fontes';
import {
  aplicarTema,
  CHAVE_TEMA,
  lerPreferencia,
  salvarPreferencia,
  temaEfetivo,
  useTemaEfetivo,
} from '../../src/renderer/util/tema';

/** matchMedia simulado e controlável (jsdom não tem). */
function simularSistema(escuro: boolean) {
  const ouvintes = new Set<() => void>();
  const consulta = {
    get matches() {
      return estado.escuro;
    },
    media: '(prefers-color-scheme: dark)',
    addEventListener: (_: string, f: () => void) => ouvintes.add(f),
    removeEventListener: (_: string, f: () => void) => ouvintes.delete(f),
  };
  const estado = { escuro };
  vi.stubGlobal('matchMedia', vi.fn(() => consulta));
  return {
    trocar(novo: boolean) {
      estado.escuro = novo;
      ouvintes.forEach((f) => f());
    },
    ouvintes,
  };
}

beforeEach(() => {
  document.documentElement.removeAttribute('data-tema');
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('preferência de tema', () => {
  it('padrão é "sistema" (sem chave salva)', () => {
    expect(lerPreferencia()).toBe('sistema');
  });

  it('salvar "escuro" grava a chave e aplicar põe data-tema="escuro"', () => {
    salvarPreferencia('escuro');
    aplicarTema(lerPreferencia());
    expect(localStorage.getItem(CHAVE_TEMA)).toBe('escuro');
    expect(document.documentElement.getAttribute('data-tema')).toBe('escuro');
  });

  it('voltar a "sistema" apaga a chave e o atributo', () => {
    salvarPreferencia('claro');
    aplicarTema('claro');
    salvarPreferencia('sistema');
    aplicarTema(lerPreferencia());
    expect(localStorage.getItem(CHAVE_TEMA)).toBeNull();
    expect(document.documentElement.hasAttribute('data-tema')).toBe(false);
  });

  it('valor inválido salvo vale "sistema"', () => {
    localStorage.setItem(CHAVE_TEMA, 'roxo');
    expect(lerPreferencia()).toBe('sistema');
  });

  it('localStorage lançando erro não quebra (lê "sistema", salvar é ignorado)', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('bloqueado');
    });
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('bloqueado');
    });
    vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => {
      throw new Error('bloqueado');
    });
    expect(lerPreferencia()).toBe('sistema');
    expect(() => salvarPreferencia('escuro')).not.toThrow();
    expect(() => salvarPreferencia('sistema')).not.toThrow();
  });
});

describe('tema efetivo', () => {
  it('segue o sistema quando a preferência é "sistema"', () => {
    const sistema = simularSistema(true);
    expect(temaEfetivo()).toBe('escuro');
    sistema.trocar(false);
    expect(temaEfetivo()).toBe('claro');
  });

  it('preferência manual vence o sistema', () => {
    simularSistema(true);
    aplicarTema('claro');
    expect(temaEfetivo()).toBe('claro');
    aplicarTema('escuro');
    expect(temaEfetivo()).toBe('escuro');
  });

  it('sem matchMedia: claro', () => {
    vi.stubGlobal('matchMedia', undefined);
    expect(temaEfetivo()).toBe('claro');
  });

  it('useTemaEfetivo acompanha ao vivo (sistema e escolha manual)', () => {
    const sistema = simularSistema(false);
    function Sonda() {
      return <span data-testid="tema">{useTemaEfetivo()}</span>;
    }
    render(<Sonda />);
    expect(screen.getByTestId('tema').textContent).toBe('claro');
    act(() => sistema.trocar(true));
    expect(screen.getByTestId('tema').textContent).toBe('escuro');
    act(() => aplicarTema('claro'));
    expect(screen.getByTestId('tema').textContent).toBe('claro');
    act(() => aplicarTema('sistema'));
    expect(screen.getByTestId('tema').textContent).toBe('escuro');
  });
});

describe('aguardarFontes', () => {
  it('sem document.fonts (jsdom) resolve na hora', async () => {
    await expect(aguardarFontes(10)).resolves.toBeUndefined();
  });

  it('não espera além do limite', async () => {
    const carregar = vi.fn(() => new Promise(() => {}));
    vi.stubGlobal('document', Object.assign(Object.create(document), { fonts: { load: carregar } }));
    const inicio = Date.now();
    await aguardarFontes(30);
    expect(Date.now() - inicio).toBeLessThan(1000);
    expect(carregar).toHaveBeenCalled();
  });
});
