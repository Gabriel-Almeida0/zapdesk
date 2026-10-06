// Motor simulado em TypeScript para testar o runner como processo real (runner-protocolo.md):
// sobe `node` com o Permission Model e as mesmas flags do motor (runtime.md › Runner), fala
// JSON-RPC pelo stdio e responde `ctx.*` com tratadores configuráveis.
import { spawn, type ChildProcessWithoutNullStreams } from 'node:child_process';
import { mkdtempSync, realpathSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
import { ConexaoRpc, ErroRpc } from '../../src/rpc.js';

export const RUNNER = join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'dist', 'zapdesk-runner.mjs');

export function pastaTemporaria(): string {
  return realpathSync(mkdtempSync(join(tmpdir(), 'zapdesk-runner-')));
}

/** Grava um bundle ESM já pronto (JS) e devolve o caminho. */
export function gravarBundle(pasta: string, nome: string, codigo: string): string {
  const caminho = join(pasta, nome);
  writeFileSync(caminho, codigo);
  return caminho;
}

/** Compila um projeto como o motor faz (esbuild; .md/.txt como texto; sourcemap inline;
 *  `@zapdesk/automacao` e `node:*` externos). */
export async function compilarProjeto(pasta: string, arquivos: Record<string, string>, saida = 'bundle.mjs'): Promise<string> {
  const fontes = join(pasta, 'fontes');
  for (const [nome, conteudo] of Object.entries(arquivos)) {
    const destino = join(fontes, nome);
    const { mkdirSync } = await import('node:fs');
    mkdirSync(dirname(destino), { recursive: true });
    writeFileSync(destino, conteudo);
  }
  const caminho = join(pasta, saida);
  await build({
    entryPoints: [join(fontes, 'index.ts')],
    outfile: caminho,
    bundle: true,
    format: 'esm',
    platform: 'node',
    target: 'node22',
    sourcemap: 'inline',
    external: ['@zapdesk/automacao', 'node:*'],
    loader: { '.md': 'text', '.txt': 'text', '.json': 'json' },
    logLevel: 'silent',
  });
  return caminho;
}

export type Tratador = (params: Record<string, unknown>) => unknown;

export interface Notificacao {
  metodo: string;
  params: Record<string, unknown>;
}

export class MotorFalso {
  readonly proc: ChildProcessWithoutNullStreams;
  readonly conexao: ConexaoRpc;
  readonly tratadores = new Map<string, Tratador>();
  readonly notificacoes: Notificacao[] = [];
  /** Requisições ctx.* recebidas, em ordem. */
  readonly chamadas: Notificacao[] = [];
  readonly pronto: Promise<Record<string, unknown>>;
  readonly saida: Promise<{ codigo: number | null; sinal: NodeJS.Signals | null }>;
  stderr = '';
  private ouvintes: ((n: Notificacao) => void)[] = [];

  constructor(opcoes: { permitirLeitura?: string[]; memoriaMb?: number; exec?: string } = {}) {
    const args = ['--permission', `--allow-fs-read=${RUNNER}`];
    for (const p of opcoes.permitirLeitura ?? []) args.push(`--allow-fs-read=${p}`);
    args.push(`--max-old-space-size=${opcoes.memoriaMb ?? 256}`, RUNNER);
    // ZAPDESK_RUNNER_EXEC=<binário do Electron> roda a mesma bateria com ELECTRON_RUN_AS_NODE
    // (Node 24 do app), como o motor faz em produção.
    const exec = opcoes.exec ?? process.env.ZAPDESK_RUNNER_EXEC ?? process.execPath;
    const env: Record<string, string> = { TZ: 'America/Sao_Paulo', LANG: 'pt_BR.UTF-8' };
    if (exec !== process.execPath) env.ELECTRON_RUN_AS_NODE = '1';
    this.proc = spawn(exec, args, {
      env,
      stdio: ['pipe', 'pipe', 'pipe'],
    });
    this.proc.stderr.on('data', (d: Buffer) => {
      this.stderr += d.toString('utf8');
    });
    let resolverPronto!: (p: Record<string, unknown>) => void;
    this.pronto = new Promise((r) => (resolverPronto = r));
    this.saida = new Promise((r) => this.proc.on('exit', (codigo, sinal) => r({ codigo, sinal })));
    this.conexao = new ConexaoRpc({
      escrever: (linha) => {
        if (!this.proc.stdin.destroyed) this.proc.stdin.write(linha);
      },
      aoNotificacao: (metodo, params) => {
        const n = { metodo, params: (params ?? {}) as Record<string, unknown> };
        this.notificacoes.push(n);
        if (metodo === 'pronto') resolverPronto(n.params);
        for (const o of this.ouvintes) o(n);
      },
      aoRequisicao: async (metodo, params) => {
        const p = (params ?? {}) as Record<string, unknown>;
        this.chamadas.push({ metodo, params: p });
        const t = this.tratadores.get(metodo);
        if (!t) throw new ErroRpc(-32601, `Método desconhecido: ${metodo}`);
        const r = await t(p);
        return r === undefined ? {} : r;
      },
    });
    this.proc.stdout.on('data', (d: Buffer) => this.conexao.receber(d));
    this.proc.stdout.on('end', () => this.conexao.fim());
  }

  tratar(metodo: string, t: Tratador): this {
    this.tratadores.set(metodo, t);
    return this;
  }

  requisitar(metodo: string, params?: unknown): Promise<unknown> {
    return this.conexao.requisitar(metodo, params);
  }

  notificar(metodo: string, params?: unknown): void {
    this.conexao.notificar(metodo, params);
  }

  /** Espera a primeira notificação que satisfaça o filtro. */
  esperarNotificacao(filtro: (n: Notificacao) => boolean, prazoMs = 5000): Promise<Notificacao> {
    const ja = this.notificacoes.find(filtro);
    if (ja) return Promise.resolve(ja);
    return new Promise((resolver, rejeitar) => {
      const t = setTimeout(() => rejeitar(new Error('notificação não chegou')), prazoMs);
      this.ouvintes.push((n) => {
        if (filtro(n)) {
          clearTimeout(t);
          resolver(n);
        }
      });
    });
  }

  async inicializar(bundle: string, extra: Record<string, unknown> = {}): Promise<unknown> {
    await this.pronto;
    return this.requisitar('inicializar', {
      protocolo: 1,
      automacao: { id: 'aut1', nome: 'Teste', versao: 1 },
      bundle,
      hash: 'h1',
      permissoes: [],
      segredos: {},
      fuso: 'America/Sao_Paulo',
      ...extra,
    });
  }

  executar(params: Partial<Record<string, unknown>> & { execucao_id: string; handler: string }): Promise<unknown> {
    return this.requisitar('executar', {
      simulacao: false,
      prazo_ms: 10_000,
      info: {},
      conversa: null,
      argumento: null,
      ...params,
    });
  }

  matar(): void {
    if (this.proc.exitCode === null) this.proc.kill('SIGKILL');
  }
}

/** Captura a rejeição de uma promessa (para `expect`). */
export async function erroDe(p: Promise<unknown>): Promise<ErroRpc> {
  try {
    await p;
  } catch (e) {
    return e as ErroRpc;
  }
  throw new Error('esperava erro');
}
