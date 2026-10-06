// Assinatura do WebSocket de eventos do motor (contracts/eventos-ws.md).
// - Reconecta com backoff 0,5 s → 5 s (dobra a cada tentativa, volta a 0,5 s ao conectar).
// - Não há replay: ao REconectar, ou ao detectar lacuna em `seq`, chama `aoRecarregar` para o
//   consumidor recarregar o estado via HTTP.
// Funciona com o WebSocket nativo do navegador e do Node 22.
import { TIPOS_EVENTO, type EventoMotor, type TipoEvento } from './tipos.js';

export const BACKOFF_INICIAL_MS = 500;
export const BACKOFF_MAXIMO_MS = 5000;

/** Mínimo do WebSocket que usamos (compatível com o nativo e com o pacote `ws`). */
export interface SoqueteMinimo {
  onopen: ((ev: unknown) => void) | null;
  onmessage: ((ev: { data: unknown }) => void) | null;
  onclose: ((ev: unknown) => void) | null;
  onerror: ((ev: unknown) => void) | null;
  close(codigo?: number, motivo?: string): void;
}

export type FabricaSoquete = (url: string) => SoqueteMinimo;

export type EstadoAssinatura = 'conectando' | 'conectada' | 'reconectando' | 'encerrada';

export type MotivoRecarga = 'reconexao' | 'lacuna_seq';

export interface OpcoesAssinaturaEventos {
  /** URL completa `ws://127.0.0.1:<porta>/v1/eventos?token=...` (ver `ClienteMotor.urlEventos()`). */
  url: string;
  aoEvento: (evento: EventoMotor) => void;
  /** Recarregar o estado via HTTP (reconexão ou lacuna em `seq`). */
  aoRecarregar?: (motivo: MotivoRecarga) => void;
  aoMudarEstado?: (estado: EstadoAssinatura) => void;
  /** Padrão: `new WebSocket(url)` global. */
  criarSoquete?: FabricaSoquete;
  /** Temporizadores injetáveis (testes). */
  agendar?: (fn: () => void, ms: number) => unknown;
  cancelarAgendamento?: (id: unknown) => void;
}

function fabricaPadrao(url: string): SoqueteMinimo {
  const Construtor = (globalThis as { WebSocket?: new (url: string) => SoqueteMinimo }).WebSocket;
  if (!Construtor) throw new Error('WebSocket indisponível neste ambiente.');
  return new Construtor(url);
}

function ehEvento(valor: unknown): valor is EventoMotor {
  if (valor === null || typeof valor !== 'object') return false;
  const v = valor as { seq?: unknown; tipo?: unknown };
  return typeof v.seq === 'number' && typeof v.tipo === 'string';
}

export class AssinaturaEventos {
  private readonly opcoes: OpcoesAssinaturaEventos;
  private readonly criarSoquete: FabricaSoquete;
  private readonly agendar: (fn: () => void, ms: number) => unknown;
  private readonly cancelarAgendamento: (id: unknown) => void;

  private soquete: SoqueteMinimo | null = null;
  private temporizador: unknown = null;
  private atrasoMs = BACKOFF_INICIAL_MS;
  private ultimoSeq: number | null = null;
  private jaConectou = false;
  private encerrada = false;
  private estadoAtual: EstadoAssinatura = 'conectando';

  constructor(opcoes: OpcoesAssinaturaEventos) {
    this.opcoes = opcoes;
    this.criarSoquete = opcoes.criarSoquete ?? fabricaPadrao;
    this.agendar = opcoes.agendar ?? ((fn, ms) => setTimeout(fn, ms));
    this.cancelarAgendamento =
      opcoes.cancelarAgendamento ?? ((id) => clearTimeout(id as ReturnType<typeof setTimeout>));
  }

  get estado(): EstadoAssinatura {
    return this.estadoAtual;
  }

  /** Próximo atraso de reconexão (exposto para testes). */
  get proximoAtrasoMs(): number {
    return this.atrasoMs;
  }

  iniciar(): this {
    this.encerrada = false;
    this.conectar();
    return this;
  }

  encerrar(): void {
    this.encerrada = true;
    if (this.temporizador !== null) {
      this.cancelarAgendamento(this.temporizador);
      this.temporizador = null;
    }
    const s = this.soquete;
    this.soquete = null;
    if (s) {
      s.onopen = s.onmessage = s.onclose = s.onerror = null;
      try {
        s.close(1000, 'encerrada');
      } catch {
        // ignorar
      }
    }
    this.mudarEstado('encerrada');
  }

  private mudarEstado(estado: EstadoAssinatura): void {
    if (this.estadoAtual === estado) return;
    this.estadoAtual = estado;
    this.opcoes.aoMudarEstado?.(estado);
  }

  private conectar(): void {
    if (this.encerrada) return;
    this.mudarEstado(this.jaConectou ? 'reconectando' : 'conectando');

    let s: SoqueteMinimo;
    try {
      s = this.criarSoquete(this.opcoes.url);
    } catch {
      this.agendarReconexao();
      return;
    }
    this.soquete = s;

    s.onopen = () => {
      const eraReconexao = this.jaConectou;
      this.jaConectou = true;
      this.atrasoMs = BACKOFF_INICIAL_MS;
      this.ultimoSeq = null; // `seq` recomeça a ser acompanhado nesta conexão
      this.mudarEstado('conectada');
      if (eraReconexao) this.opcoes.aoRecarregar?.('reconexao');
    };

    s.onmessage = (ev) => this.receber(ev.data);

    s.onerror = () => {
      // o `close` vem em seguida e cuida da reconexão
    };

    s.onclose = () => {
      if (this.soquete !== s) return;
      this.soquete = null;
      this.agendarReconexao();
    };
  }

  private agendarReconexao(): void {
    if (this.encerrada) return;
    this.mudarEstado('reconectando');
    const atraso = this.atrasoMs;
    this.atrasoMs = Math.min(this.atrasoMs * 2, BACKOFF_MAXIMO_MS);
    this.temporizador = this.agendar(() => {
      this.temporizador = null;
      this.conectar();
    }, atraso);
  }

  private receber(dados: unknown): void {
    if (typeof dados !== 'string') return;
    let valor: unknown;
    try {
      valor = JSON.parse(dados);
    } catch {
      return;
    }
    if (!ehEvento(valor)) return;

    const anterior = this.ultimoSeq;
    this.ultimoSeq = valor.seq;
    if (anterior !== null && valor.seq !== anterior + 1) {
      this.opcoes.aoRecarregar?.('lacuna_seq');
    }

    if ((TIPOS_EVENTO as readonly string[]).includes(valor.tipo)) {
      this.opcoes.aoEvento(valor);
    }
  }
}

/** Atalho para filtrar eventos por tipo com o tipo de `dados` correto. */
export function ehTipo<T extends TipoEvento>(
  evento: EventoMotor,
  tipo: T,
): evento is Extract<EventoMotor, { tipo: T }> {
  return evento.tipo === tipo;
}
