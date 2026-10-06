// Processo filho do motor (contracts/runtime.md).
// - `ProcessoMotor.iniciar()`: spawn com o token SÓ em `ZAPDESK_TOKEN`, espera a linha `pronto`
//   (10 s), consome stdout/stderr até o fim (senão o pipe enche e trava o motor) e mantém o stdin
//   aberto — se o app morrer, o stdin fecha e o motor se encerra sozinho.
// - `parar()`: SIGTERM → SIGKILL após 5 s.
// - `SupervisorMotor`: liga, religa UMA vez após queda inesperada (`--religado`) e expõe o estado
//   para a janela ("Ligando o WhatsApp…", "O WhatsApp parou. Religando…", erro).
import { spawn as spawnPadrao, type ChildProcess } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { EventEmitter } from 'node:events';
import { createInterface } from 'node:readline';

import type { LinhaControleApp } from '@zapdesk/cliente-motor';

import type { ConexaoMotor, EstadoMotor } from '../preload/tipos';

export const TIMEOUT_PRONTO_MS = 10_000;
export const PRAZO_ENCERRAR_MS = 5_000;
const MAX_LINHAS_STDERR = 40;

export const MENSAGEM_NAO_LIGOU = 'Não consegui ligar o WhatsApp.';

/** Token de sessão: 32 bytes aleatórios em base64url (43 caracteres). */
export function gerarToken(): string {
  return randomBytes(32).toString('base64url');
}

export interface OpcoesProcessoMotor {
  /** Executável do motor (ou `node` nos testes). */
  binario: string;
  /** Argumentos antes das flags do motor (testes: caminho do script simulado). */
  argsIniciais?: string[];
  pastaDados: string;
  /** `--whatsapp=falso`. */
  falso?: boolean;
  /** `--religado`: disparos ativos voltam com `motivo_pausa=motor_reiniciado`. */
  religado?: boolean;
  /** 002: executável do runner das automações de IA (`process.execPath` do Electron). */
  runnerExec?: string;
  /** 002: `zapdesk-runner.mjs` (só é passado junto com `runnerExec`). */
  runnerScript?: string;
  /** 002: o motor espera a 1ª linha `segredos` no stdin antes de ligar as automações. */
  aguardarSegredos?: boolean;
  /** Flags extras (ex.: `--nivel-log=debug`). */
  argsExtras?: string[];
  timeoutProntoMs?: number;
  prazoEncerrarMs?: number;
  /** Ambiente base (o token é acrescentado aqui). Padrão: `process.env`. */
  env?: NodeJS.ProcessEnv;
  token?: string;
  spawn?: typeof spawnPadrao;
  /** Espelhar stderr do motor (desenvolvimento). */
  aoLogar?: (linha: string) => void;
}

export interface InfoPronto {
  porta: number;
  versao: string;
  pid: number;
  whatsapp: 'real' | 'falso';
}

/** Falha ao iniciar: `codigo` do `erro_fatal`, `timeout`, `saiu` ou `binario_ausente`. */
export class ErroInicioMotor extends Error {
  readonly codigo: string;
  readonly detalhe: string;
  constructor(codigo: string, detalhe: string) {
    super(`${MENSAGEM_NAO_LIGOU} ${detalhe}`);
    this.name = 'ErroInicioMotor';
    this.codigo = codigo;
    this.detalhe = detalhe;
  }
}

const DETALHES_ERRO_FATAL: Record<string, string> = {
  token_ausente: 'O motor não recebeu o token de sessão.',
  pasta_dados_inacessivel: 'Não foi possível acessar a pasta de dados.',
  instancia_duplicada: 'Outro motor do ZapDesk já está rodando com esta pasta de dados.',
  migracao_falhou: 'A atualização do banco de dados falhou (há um backup na pasta de dados).',
  porta_ocupada: 'A porta local escolhida já está em uso.',
};

export function montarArgs(opcoes: OpcoesProcessoMotor): string[] {
  const args = [...(opcoes.argsIniciais ?? []), '--pasta-dados', opcoes.pastaDados];
  if (opcoes.falso) args.push('--whatsapp=falso');
  if (opcoes.religado) args.push('--religado');
  if (opcoes.runnerExec && opcoes.runnerScript) {
    args.push('--runner-exec', opcoes.runnerExec, '--runner-script', opcoes.runnerScript);
  }
  if (opcoes.aguardarSegredos) args.push('--aguardar-segredos');
  args.push(...(opcoes.argsExtras ?? []));
  return args;
}

export class ProcessoMotor extends EventEmitter<{
  saida: [codigo: number | null, sinal: NodeJS.Signals | null, esperada: boolean];
}> {
  readonly token: string;
  readonly info: InfoPronto;
  private readonly filho: ChildProcess;
  private readonly prazoEncerrarMs: number;
  private parando: Promise<void> | null = null;
  private terminou = false;

  private constructor(filho: ChildProcess, token: string, info: InfoPronto, prazoEncerrarMs: number) {
    super();
    this.filho = filho;
    this.token = token;
    this.info = info;
    this.prazoEncerrarMs = prazoEncerrarMs;
    filho.once('exit', (codigo, sinal) => {
      this.terminou = true;
      this.emit('saida', codigo, sinal, this.parando !== null);
    });
  }

  get pid(): number {
    return this.info.pid;
  }

  get vivo(): boolean {
    return !this.terminou;
  }

  /** Inicia o motor e resolve quando ele imprime `pronto`. */
  static iniciar(opcoes: OpcoesProcessoMotor): Promise<ProcessoMotor> {
    const token = opcoes.token ?? gerarToken();
    const timeoutMs = opcoes.timeoutProntoMs ?? TIMEOUT_PRONTO_MS;
    const prazo = opcoes.prazoEncerrarMs ?? PRAZO_ENCERRAR_MS;
    const spawn = opcoes.spawn ?? spawnPadrao;
    const env: NodeJS.ProcessEnv = { ...(opcoes.env ?? process.env), ZAPDESK_TOKEN: token };
    // Nunca herdar um modo "rodar como node" do app para o motor.
    delete env['ELECTRON_RUN_AS_NODE'];

    return new Promise<ProcessoMotor>((resolver, rejeitar) => {
      let filho: ChildProcess;
      try {
        filho = spawn(opcoes.binario, montarArgs(opcoes), {
          env,
          stdio: ['pipe', 'pipe', 'pipe'],
          detached: false,
        });
      } catch (causa) {
        rejeitar(new ErroInicioMotor('binario_ausente', `Motor não encontrado em ${opcoes.binario}.`));
        void causa;
        return;
      }

      const stderr: string[] = [];
      let resolvido = false;
      let erroFatal: ErroInicioMotor | null = null;

      const falhar = (erro: ErroInicioMotor): void => {
        if (resolvido) return;
        resolvido = true;
        clearTimeout(relogio);
        // Garante que nada sobra de uma tentativa que falhou.
        if (filho.exitCode === null && filho.signalCode === null) {
          filho.kill('SIGKILL');
        }
        rejeitar(erro);
      };

      const relogio = setTimeout(() => {
        falhar(
          new ErroInicioMotor(
            'timeout',
            `O motor não respondeu em ${Math.round(timeoutMs / 1000)} s.${ultimoStderr(stderr)}`,
          ),
        );
      }, timeoutMs);

      filho.once('error', (erro: NodeJS.ErrnoException) => {
        const detalhe =
          erro.code === 'ENOENT'
            ? `Motor não encontrado em ${opcoes.binario}.`
            : `Falha ao executar o motor: ${erro.message}`;
        falhar(new ErroInicioMotor(erro.code === 'ENOENT' ? 'binario_ausente' : 'spawn', detalhe));
      });

      // O stdin fica aberto de propósito; erros de escrita (EPIPE) são ignorados.
      filho.stdin?.on('error', () => undefined);

      if (filho.stderr) {
        createInterface({ input: filho.stderr }).on('line', (linha) => {
          stderr.push(linha);
          if (stderr.length > MAX_LINHAS_STDERR) stderr.shift();
          opcoes.aoLogar?.(linha);
        });
      }

      if (filho.stdout) {
        createInterface({ input: filho.stdout }).on('line', (linha) => {
          const controle = parsearLinhaControle(linha);
          if (!controle || resolvido) return;
          if (controle['evento'] === 'pronto') {
            resolvido = true;
            clearTimeout(relogio);
            const info: InfoPronto = {
              porta: Number(controle['porta']),
              versao: String(controle['versao'] ?? ''),
              pid: Number(controle['pid'] ?? filho.pid ?? 0),
              whatsapp: controle['whatsapp'] === 'falso' ? 'falso' : 'real',
            };
            resolver(new ProcessoMotor(filho, token, info, prazo));
          } else if (controle['evento'] === 'erro_fatal') {
            const codigo = String(controle['codigo'] ?? 'desconhecido');
            const detalhe =
              DETALHES_ERRO_FATAL[codigo] ?? String(controle['mensagem'] ?? 'Erro fatal no motor.');
            erroFatal = new ErroInicioMotor(codigo, detalhe);
            falhar(erroFatal);
          }
        });
      }

      filho.once('exit', (codigo, sinal) => {
        if (resolvido) return;
        const detalhe =
          codigo === 3
            ? (DETALHES_ERRO_FATAL['instancia_duplicada'] as string)
            : codigo === 2
              ? 'Configuração inválida do motor.'
              : `O motor encerrou antes de ficar pronto (${sinal ?? `código ${String(codigo)}`}).`;
        falhar(erroFatal ?? new ErroInicioMotor('saiu', `${detalhe}${ultimoStderr(stderr)}`));
      });
    });
  }

  /** SIGTERM; se não sair em 5 s, SIGKILL. Idempotente. */
  parar(): Promise<void> {
    if (this.parando) return this.parando;
    this.parando = new Promise<void>((resolver) => {
      if (this.terminou) {
        resolver();
        return;
      }
      const matar = setTimeout(() => {
        if (!this.terminou) this.filho.kill('SIGKILL');
      }, this.prazoEncerrarMs);
      this.filho.once('exit', () => {
        clearTimeout(matar);
        resolver();
      });
      this.filho.kill('SIGTERM');
    });
    return this.parando;
  }

  /** Encerramento síncrono de emergência (saída abrupta do app). */
  matarAgora(): void {
    if (!this.terminou) this.filho.kill('SIGKILL');
  }

  /**
   * 002: escreve uma linha JSON de controle no stdin do motor (contracts/runtime.md › Canal de
   * controle). Devolve `false` se o motor já saiu. O conteúdo nunca é logado.
   */
  enviarControle(linha: LinhaControleApp): boolean {
    const stdin = this.filho.stdin;
    if (this.terminou || !stdin || stdin.destroyed || !stdin.writable) return false;
    stdin.write(`${JSON.stringify(linha)}\n`);
    return true;
  }
}

function ultimoStderr(linhas: string[]): string {
  const ultima = linhas.filter((l) => l.trim().length > 0).at(-1);
  return ultima ? ` Último log: ${ultima.slice(0, 300)}` : '';
}

export function parsearLinhaControle(linha: string): Record<string, unknown> | null {
  const texto = linha.trim();
  if (!texto.startsWith('{')) return null;
  try {
    const valor: unknown = JSON.parse(texto);
    if (valor && typeof valor === 'object' && 'evento' in valor) return valor as Record<string, unknown>;
  } catch {
    // linha não é de controle
  }
  return null;
}

// ---------------------------------------------------------------------------
// Supervisor: liga, religa uma vez, informa o estado
// ---------------------------------------------------------------------------

/** O mínimo de `ProcessoMotor` que o supervisor usa (permite simular nos testes). */
export interface MotorLigado {
  readonly token: string;
  readonly info: InfoPronto;
  on(evento: 'saida', ouvinte: (codigo: number | null, sinal: NodeJS.Signals | null, esperada: boolean) => void): unknown;
  parar(): Promise<void>;
  matarAgora(): void;
  enviarControle?(linha: LinhaControleApp): boolean;
}

export interface OpcoesSupervisor {
  /** Inicia uma instância do motor (`religado` = após queda). */
  iniciar: (religado: boolean) => Promise<MotorLigado>;
  /**
   * 002: valores dos segredos entregues ao motor logo após cada `pronto` (sempre, mesmo vazio),
   * antes de a janela saber que o motor está pronto.
   */
  segredos?: () => Record<string, string>;
}

export class SupervisorMotor extends EventEmitter<{ estado: [EstadoMotor] }> {
  private readonly opcoes: OpcoesSupervisor;
  private motor: MotorLigado | null = null;
  private estadoAtual: EstadoMotor = { fase: 'ligando' };
  private jaReligou = false;
  private encerrado = false;
  private ligando: Promise<void> | null = null;

  constructor(opcoes: OpcoesSupervisor) {
    super();
    this.opcoes = opcoes;
  }

  get estado(): EstadoMotor {
    return this.estadoAtual;
  }

  get processo(): MotorLigado | null {
    return this.motor;
  }

  get conexao(): ConexaoMotor | null {
    return this.estadoAtual.fase === 'pronto' ? this.estadoAtual.conexao : null;
  }

  private mudar(estado: EstadoMotor): void {
    this.estadoAtual = estado;
    this.emit('estado', estado);
  }

  /** Primeira subida (ou "Tentar de novo" depois de um erro). */
  ligar(): Promise<void> {
    if (this.ligando) return this.ligando;
    this.encerrado = false;
    this.jaReligou = false;
    this.ligando = this.subir(false).finally(() => {
      this.ligando = null;
    });
    return this.ligando;
  }

  /** "Tentar de novo": derruba o que houver e liga do zero. */
  async reiniciar(): Promise<void> {
    const anterior = this.motor;
    this.motor = null;
    if (anterior) await anterior.parar();
    await this.ligar();
  }

  private async subir(religado: boolean): Promise<void> {
    this.mudar(religado ? { fase: 'religando' } : { fase: 'ligando' });
    try {
      const motor = await this.opcoes.iniciar(religado);
      if (this.encerrado) {
        await motor.parar();
        return;
      }
      this.motor = motor;
      if (this.opcoes.segredos) motor.enviarControle?.({ comando: 'segredos', valores: this.opcoes.segredos() });
      motor.on('saida', (codigo, sinal, esperada) => {
        if (this.motor !== motor) return;
        this.motor = null;
        if (esperada || this.encerrado) return;
        void this.aoCair(codigo, sinal);
      });
      this.mudar({
        fase: 'pronto',
        conexao: {
          porta: motor.info.porta,
          token: motor.token,
          versao: motor.info.versao,
          whatsapp: motor.info.whatsapp,
          pidMotor: motor.info.pid,
        },
      });
    } catch (erro) {
      const detalhe =
        erro instanceof ErroInicioMotor ? erro.detalhe : erro instanceof Error ? erro.message : String(erro);
      this.mudar({ fase: 'erro', mensagem: MENSAGEM_NAO_LIGOU, detalhe });
    }
  }

  private async aoCair(codigo: number | null, sinal: NodeJS.Signals | null): Promise<void> {
    if (this.jaReligou) {
      this.mudar({
        fase: 'erro',
        mensagem: MENSAGEM_NAO_LIGOU,
        detalhe: `O motor parou de novo (${sinal ?? `código ${String(codigo)}`}).`,
      });
      return;
    }
    this.jaReligou = true;
    await this.subir(true);
  }

  /** 002: reenvia o conjunto de segredos (a cada alteração em Ajustes → IA). */
  enviarSegredos(valores: Record<string, string>): boolean {
    return this.motor?.enviarControle?.({ comando: 'segredos', valores }) ?? false;
  }

  /** Encerra o motor (fechar o app). */
  async parar(): Promise<void> {
    this.encerrado = true;
    const motor = this.motor;
    this.motor = null;
    if (motor) await motor.parar();
  }

  matarAgora(): void {
    this.encerrado = true;
    this.motor?.matarAgora();
  }
}
