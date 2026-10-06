import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

import { describe, expect, it } from 'vitest';

// Garante as opções de segurança exigidas pela constituição (Restrições técnicas).
describe('janela principal', () => {
  const fonte = readFileSync(fileURLToPath(new URL('../../src/main/janela.ts', import.meta.url)), 'utf8');

  it('usa contextIsolation, sem nodeIntegration e com sandbox', () => {
    expect(fonte).toMatch(/contextIsolation:\s*true/);
    expect(fonte).toMatch(/nodeIntegration:\s*false/);
    expect(fonte).toMatch(/sandbox:\s*true/);
  });
});
