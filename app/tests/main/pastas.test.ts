// 003 T004 — pasta `userData` da interface: modo real inalterado; modo falso isolado do app
// instalado (pasta temporária ou ZAPDESK_PASTA_INTERFACE).
import { existsSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { basename, dirname, join } from 'node:path';

import { describe, expect, it } from 'vitest';

import { resolverPastaInterface } from '../../src/main/pastas';

const PADRAO = '/Users/teste/Library/Application Support/ZapDesk/interface';

describe('resolverPastaInterface', () => {
  it('modo real sem variável: usa a pasta padrão, sem mudança', () => {
    expect(resolverPastaInterface({ env: {}, modoFalso: false, padrao: PADRAO, tmp: '/tmp' })).toBe(PADRAO);
  });

  it('ZAPDESK_PASTA_INTERFACE tem prioridade (modo real e falso)', () => {
    const env = { ZAPDESK_PASTA_INTERFACE: '/tmp/minha-interface' };
    expect(resolverPastaInterface({ env, modoFalso: false, padrao: PADRAO, tmp: '/tmp' })).toBe('/tmp/minha-interface');
    expect(resolverPastaInterface({ env, modoFalso: true, padrao: PADRAO, tmp: '/tmp' })).toBe('/tmp/minha-interface');
  });

  it('variável vazia é ignorada', () => {
    expect(
      resolverPastaInterface({ env: { ZAPDESK_PASTA_INTERFACE: '' }, modoFalso: false, padrao: PADRAO, tmp: '/tmp' }),
    ).toBe(PADRAO);
  });

  it('modo falso sem variável: cria pasta temporária nova, nunca a padrão', () => {
    const pedidos: string[] = [];
    const pasta = resolverPastaInterface({
      env: {},
      modoFalso: true,
      padrao: PADRAO,
      tmp: '/tmp/x',
      criarTemporaria: (prefixo) => {
        pedidos.push(prefixo);
        return `${prefixo}abc123`;
      },
    });
    expect(pedidos).toEqual(['/tmp/x/zapdesk-interface-']);
    expect(pasta).toBe('/tmp/x/zapdesk-interface-abc123');
    expect(pasta).not.toBe(PADRAO);
  });

  it('modo falso com mkdtemp real: a pasta existe dentro do tmp', () => {
    const pasta = resolverPastaInterface({ env: {}, modoFalso: true, padrao: PADRAO, tmp: tmpdir() });
    try {
      expect(existsSync(pasta)).toBe(true);
      expect(dirname(pasta)).toBe(tmpdir());
      expect(basename(pasta).startsWith('zapdesk-interface-')).toBe(true);
    } finally {
      rmSync(pasta, { recursive: true, force: true });
    }
    expect(join(pasta)).not.toBe(PADRAO);
  });
});
