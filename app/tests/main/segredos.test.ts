// T053 — cofre de segredos com safeStorage simulado: arquivo 0600 cifrado, máscara, validação e
// valores que nunca aparecem na listagem.
import { existsSync, mkdtempSync, readFileSync, statSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { describe, expect, it, vi } from 'vitest';

import { pastaAutomacaoPermitida } from '../../src/main/pastas';
import { ARQUIVO_SEGREDOS, CofreSegredos, mascarar, validarSegredo, type CifradorSegredos } from '../../src/main/segredos';

/** "Cifra" reversível que troca o texto de lugar, para provar que nada vai em claro. */
function cifradorFalso(disponivel = true): CifradorSegredos {
  return {
    isEncryptionAvailable: () => disponivel,
    encryptString: (t) => Buffer.from(Buffer.from(t, 'utf8').toString('base64').split('').reverse().join(''), 'utf8'),
    decryptString: (b) => Buffer.from(b.toString('utf8').split('').reverse().join(''), 'base64').toString('utf8'),
  };
}

const CHAVE = 'sk-ant-api03-abcdefghijklmnopqrstuvwxyz-a1b2';

describe('CofreSegredos', () => {
  it('grava cifrado em segredos.bin 0600 e relê', async () => {
    const pasta = mkdtempSync(join(tmpdir(), 'zapdesk-segredos-'));
    const cofre = new CofreSegredos({ pastaDados: pasta, cifrador: cifradorFalso() });
    await cofre.carregar();
    expect(cofre.listar()).toEqual([]);

    const alterado = vi.fn();
    cofre.on('alterado', alterado);
    const lista = await cofre.definir('ANTHROPIC_API_KEY', CHAVE);
    expect(lista).toEqual([{ nome: 'ANTHROPIC_API_KEY', mascara: 'sk-ant-…a1b2' }]);
    expect(alterado).toHaveBeenCalledWith({ ANTHROPIC_API_KEY: CHAVE });

    const caminho = join(pasta, ARQUIVO_SEGREDOS);
    expect(statSync(caminho).mode & 0o777).toBe(0o600);
    expect(readFileSync(caminho, 'utf8')).not.toContain(CHAVE);

    const outro = new CofreSegredos({ pastaDados: pasta, cifrador: cifradorFalso() });
    await outro.carregar();
    expect(outro.valores()).toEqual({ ANTHROPIC_API_KEY: CHAVE });
  });

  it('lista só nomes e máscaras (chave da Anthropic primeiro) e remove', async () => {
    const pasta = mkdtempSync(join(tmpdir(), 'zapdesk-segredos-'));
    const cofre = new CofreSegredos({ pastaDados: pasta, cifrador: cifradorFalso() });
    await cofre.carregar();
    await cofre.definir('OPENAI_KEY', 'sk-proj-1234567890XYZW');
    await cofre.definir('ANTHROPIC_API_KEY', CHAVE);
    const lista = cofre.listar();
    expect(lista.map((s) => s.nome)).toEqual(['ANTHROPIC_API_KEY', 'OPENAI_KEY']);
    expect(JSON.stringify(lista)).not.toContain('1234567890');
    expect(lista[1]?.mascara).toBe('…XYZW');
    await cofre.remover('OPENAI_KEY');
    expect(cofre.valores()).toEqual({ ANTHROPIC_API_KEY: CHAVE });
  });

  it('recusa nome ou valor inválido sem gravar', async () => {
    const pasta = mkdtempSync(join(tmpdir(), 'zapdesk-segredos-'));
    const cofre = new CofreSegredos({ pastaDados: pasta, cifrador: cifradorFalso() });
    await cofre.carregar();
    await expect(cofre.definir('minha_chave', 'x')).rejects.toThrow(/maiúsculas/);
    await expect(cofre.definir('CHAVE', '')).rejects.toThrow(/1 a 4096/);
    await expect(cofre.definir('CHAVE', 'x'.repeat(4097))).rejects.toThrow(/1 a 4096/);
    expect(existsSync(join(pasta, ARQUIVO_SEGREDOS))).toBe(false);
  });

  it('sem Keychain disponível não grava e avisa', async () => {
    const pasta = mkdtempSync(join(tmpdir(), 'zapdesk-segredos-'));
    const cofre = new CofreSegredos({ pastaDados: pasta, cifrador: cifradorFalso(false) });
    await cofre.carregar();
    await expect(cofre.definir('CHAVE', 'valor')).rejects.toThrow(/chaveiro do macOS/);
    expect(cofre.valores()).toEqual({});
  });

  it('arquivo ilegível vira cofre vazio sem derrubar', async () => {
    const pasta = mkdtempSync(join(tmpdir(), 'zapdesk-segredos-'));
    writeFileSync(join(pasta, ARQUIVO_SEGREDOS), 'lixo');
    const cofre = new CofreSegredos({ pastaDados: pasta, cifrador: cifradorFalso() });
    await cofre.carregar();
    expect(cofre.valores()).toEqual({});
  });

  it('máscara e validação', () => {
    expect(mascarar(CHAVE)).toBe('sk-ant-…a1b2');
    expect(mascarar('curto')).toBe('••••');
    expect(validarSegredo('A', 'v')).toBeNull();
    expect(validarSegredo('1A', 'v')).toBeTruthy();
    expect(validarSegredo(`A${'B'.repeat(64)}`, 'v')).toBeTruthy();
    expect(validarSegredo('A', 'linha\nquebrada')).toBeTruthy();
  });
});

describe('abrirPastaExterna', () => {
  it('só aceita pastas dentro de <pasta-dados>/automacoes/', () => {
    const dados = '/Users/x/Library/Application Support/ZapDesk';
    expect(pastaAutomacaoPermitida(`${dados}/automacoes/01ABC`, dados)).toBe(`${dados}/automacoes/01ABC`);
    expect(pastaAutomacaoPermitida('automacoes/01ABC', dados)).toBe(`${dados}/automacoes/01ABC`);
    expect(pastaAutomacaoPermitida(`${dados}/automacoes`, dados)).toBeNull();
    expect(pastaAutomacaoPermitida(`${dados}/automacoes/../banco`, dados)).toBeNull();
    expect(pastaAutomacaoPermitida('/etc', dados)).toBeNull();
    expect(pastaAutomacaoPermitida(42, dados)).toBeNull();
  });
});
