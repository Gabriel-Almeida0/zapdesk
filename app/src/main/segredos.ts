// Cofre de segredos do app (T053, contracts/runtime.md › App).
// Os valores (chave da Anthropic e segredos com nome) ficam cifrados com `safeStorage` (Keychain do
// macOS) em `<pasta-dados>/segredos.bin` (0600). A janela só vê nomes e máscaras; os valores saem
// daqui apenas para o stdin do motor (linha `segredos`), nunca para o renderer nem para logs.
import { EventEmitter } from 'node:events';
import { chmod, readFile, rename, writeFile } from 'node:fs/promises';
import { join } from 'node:path';

import type { SegredoLocal } from '../preload/tipos';

/** O pedaço do `safeStorage` do Electron que o cofre usa (simulado nos testes). */
export interface CifradorSegredos {
  isEncryptionAvailable(): boolean;
  encryptString(texto: string): Buffer;
  decryptString(cifrado: Buffer): string;
}

export const NOME_SEGREDO = /^[A-Z][A-Z0-9_]{0,63}$/;
export const TAMANHO_MAX_SEGREDO = 4096;
export const CHAVE_ANTHROPIC = 'ANTHROPIC_API_KEY';
export const ARQUIVO_SEGREDOS = 'segredos.bin';

export class ErroSegredo extends Error {
  constructor(mensagem: string) {
    super(mensagem);
    this.name = 'ErroSegredo';
  }
}

/** "sk-ant-…a1b2": prefixo conhecido + 4 últimos. Valores curtos não revelam nada. */
export function mascarar(valor: string): string {
  if (valor.length < 12) return '••••';
  const fim = valor.slice(-4);
  if (valor.startsWith('sk-ant-')) return `sk-ant-…${fim}`;
  return `…${fim}`;
}

export function validarSegredo(nome: string, valor: string): string | null {
  if (!NOME_SEGREDO.test(nome)) return 'Use só letras maiúsculas, números e _ (começando por letra), até 64 caracteres.';
  if (valor.length < 1 || valor.length > TAMANHO_MAX_SEGREDO) return `O valor deve ter de 1 a ${TAMANHO_MAX_SEGREDO} caracteres.`;
  if (/[\r\n]/.test(valor)) return 'O valor não pode ter quebra de linha.';
  return null;
}

export class CofreSegredos extends EventEmitter<{ alterado: [valores: Record<string, string>] }> {
  private readonly caminho: string;
  private readonly cifrador: CifradorSegredos;
  private dados: Record<string, string> = {};
  private fila: Promise<void> = Promise.resolve();

  constructor(opcoes: { pastaDados: string; cifrador: CifradorSegredos }) {
    super();
    this.caminho = join(opcoes.pastaDados, ARQUIVO_SEGREDOS);
    this.cifrador = opcoes.cifrador;
  }

  /** Lê o arquivo (ausente ou ilegível = cofre vazio; nunca derruba o app). */
  async carregar(): Promise<void> {
    let bruto: Buffer;
    try {
      bruto = await readFile(this.caminho);
    } catch {
      this.dados = {};
      return;
    }
    try {
      if (!this.cifrador.isEncryptionAvailable()) throw new Error('indisponível');
      const valor: unknown = JSON.parse(this.cifrador.decryptString(bruto));
      const limpo: Record<string, string> = {};
      if (valor && typeof valor === 'object') {
        for (const [nome, v] of Object.entries(valor as Record<string, unknown>)) {
          if (typeof v === 'string' && validarSegredo(nome, v) === null) limpo[nome] = v;
        }
      }
      this.dados = limpo;
    } catch {
      // Arquivo de outro usuário/Mac ou corrompido: começa vazio sem apagar o arquivo.
      this.dados = {};
    }
  }

  /** Cópia dos valores (só para o motor). */
  valores(): Record<string, string> {
    return { ...this.dados };
  }

  listar(): SegredoLocal[] {
    return Object.keys(this.dados)
      .sort((a, b) => (a === CHAVE_ANTHROPIC ? -1 : b === CHAVE_ANTHROPIC ? 1 : a.localeCompare(b)))
      .map((nome) => ({ nome, mascara: mascarar(this.dados[nome] ?? '') }));
  }

  definir(nome: string, valor: string): Promise<SegredoLocal[]> {
    const erro = validarSegredo(nome, valor);
    if (erro) return Promise.reject(new ErroSegredo(erro));
    return this.alterar((d) => {
      d[nome] = valor;
    });
  }

  remover(nome: string): Promise<SegredoLocal[]> {
    return this.alterar((d) => {
      delete d[nome];
    });
  }

  private alterar(mudar: (d: Record<string, string>) => void): Promise<SegredoLocal[]> {
    const passo = this.fila.then(async () => {
      const novo = { ...this.dados };
      mudar(novo);
      await this.gravar(novo);
      this.dados = novo;
      this.emit('alterado', this.valores());
      return this.listar();
    });
    this.fila = passo.then(
      () => undefined,
      () => undefined,
    );
    return passo;
  }

  private async gravar(dados: Record<string, string>): Promise<void> {
    if (!this.cifrador.isEncryptionAvailable()) {
      throw new ErroSegredo('O chaveiro do macOS não está disponível. Não foi possível guardar o segredo.');
    }
    const cifrado = this.cifrador.encryptString(JSON.stringify(dados));
    const temporario = `${this.caminho}.${process.pid}.tmp`;
    await writeFile(temporario, cifrado, { mode: 0o600 });
    await chmod(temporario, 0o600);
    await rename(temporario, this.caminho);
  }
}
