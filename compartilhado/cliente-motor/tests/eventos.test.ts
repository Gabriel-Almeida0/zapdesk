import type { AddressInfo } from 'node:net';

import { afterEach, describe, expect, it } from 'vitest';
import { WebSocketServer, type WebSocket as SoqueteServidor } from 'ws';

import {
  AssinaturaEventos,
  BACKOFF_INICIAL_MS,
  BACKOFF_MAXIMO_MS,
  type EventoMotor,
  type MotivoRecarga,
  type SoqueteMinimo,
} from '../src/index.js';

const evento = (seq: number, tipo = 'disparos.ativos', dados: unknown = { total: 0 }) =>
  JSON.stringify({ seq, tipo, conta_id: null, em: '2026-09-27T20:00:00-03:00', dados });

/** Espera uma condição com sondagem curta (sem sleep fixo). */
async function esperar(condicao: () => boolean, prazoMs = 3000): Promise<void> {
  const inicio = Date.now();
  while (!condicao()) {
    if (Date.now() - inicio > prazoMs) throw new Error('condição não atingida a tempo');
    await new Promise((r) => setTimeout(r, 10));
  }
}

describe('AssinaturaEventos — backoff (temporizador injetado)', () => {
  it('reconecta com atraso 0,5 s → 1 → 2 → 4 → 5 → 5 e volta a 0,5 s ao conectar', () => {
    const soquetes: SoqueteMinimo[] = [];
    const atrasos: number[] = [];
    let pendente: (() => void) | null = null;

    const assinatura = new AssinaturaEventos({
      url: 'ws://127.0.0.1:1/v1/eventos?token=t',
      aoEvento: () => undefined,
      criarSoquete: () => {
        const s: SoqueteMinimo = { onopen: null, onmessage: null, onclose: null, onerror: null, close: () => undefined };
        soquetes.push(s);
        return s;
      },
      agendar: (fn, ms) => {
        atrasos.push(ms);
        pendente = fn;
        return 1;
      },
      cancelarAgendamento: () => undefined,
    }).iniciar();

    for (let i = 0; i < 6; i++) {
      soquetes.at(-1)!.onclose?.({});
      (pendente as unknown as () => void)();
    }
    expect(atrasos).toEqual([500, 1000, 2000, 4000, 5000, 5000]);
    expect(Math.max(...atrasos)).toBe(BACKOFF_MAXIMO_MS);

    soquetes.at(-1)!.onopen?.({});
    expect(assinatura.estado).toBe('conectada');
    expect(assinatura.proximoAtrasoMs).toBe(BACKOFF_INICIAL_MS);
    assinatura.encerrar();
    expect(assinatura.estado).toBe('encerrada');
  });
});

describe('AssinaturaEventos — servidor WS de teste', () => {
  let servidor: WebSocketServer | null = null;
  let assinatura: AssinaturaEventos | null = null;

  afterEach(async () => {
    assinatura?.encerrar();
    assinatura = null;
    if (servidor) {
      for (const c of servidor.clients) c.terminate();
      await new Promise<void>((r) => servidor!.close(() => r()));
      servidor = null;
    }
  });

  async function subirServidor(aoConectar: (s: SoqueteServidor, n: number) => void) {
    let conexoes = 0;
    const urls: string[] = [];
    servidor = new WebSocketServer({ host: '127.0.0.1', port: 0 });
    servidor.on('connection', (s, req) => {
      conexoes += 1;
      urls.push(req.url ?? '');
      aoConectar(s, conexoes);
    });
    await new Promise<void>((r) => servidor!.once('listening', () => r()));
    const porta = (servidor.address() as AddressInfo).port;
    return { url: `ws://127.0.0.1:${porta}/v1/eventos?token=abc`, urls, conexoes: () => conexoes };
  }

  it('entrega eventos e detecta lacuna de seq', async () => {
    const { url, urls } = await subirServidor((s) => {
      s.send(evento(1, 'motor.pronto', { versao: '0.1.0' }));
      s.send(evento(2));
      s.send(evento(4)); // lacuna: faltou o 3
    });
    const recebidos: EventoMotor[] = [];
    const recargas: MotivoRecarga[] = [];
    assinatura = new AssinaturaEventos({
      url,
      aoEvento: (e) => recebidos.push(e),
      aoRecarregar: (m) => recargas.push(m),
    }).iniciar();

    await esperar(() => recebidos.length === 3);
    expect(recebidos.map((e) => e.seq)).toEqual([1, 2, 4]);
    expect(recebidos[0]!.tipo).toBe('motor.pronto');
    expect(recargas).toEqual(['lacuna_seq']);
    expect(urls[0]).toBe('/v1/eventos?token=abc');
  });

  it('reconecta quando o servidor derruba a conexão e pede recarga', async () => {
    const { url, conexoes } = await subirServidor((s, n) => {
      s.send(evento(n === 1 ? 1 : 100, 'motor.pronto', { versao: '0.1.0' }));
      if (n === 1) setTimeout(() => s.close(), 20);
    });
    const recargas: MotivoRecarga[] = [];
    const estados: string[] = [];
    assinatura = new AssinaturaEventos({
      url,
      aoEvento: () => undefined,
      aoRecarregar: (m) => recargas.push(m),
      aoMudarEstado: (e) => estados.push(e),
    }).iniciar();

    await esperar(() => conexoes() === 2 && recargas.length === 1);
    expect(recargas).toEqual(['reconexao']);
    expect(estados).toContain('reconectando');
    await esperar(() => assinatura!.estado === 'conectada');
  });
});
