// Evento WS `notificacao` (002) → notificação do macOS (T056).
// Clique: mostra a janela e pede ao renderer para abrir a conversa ou a automação.
import type { Notificacao } from '@zapdesk/cliente-motor';

import type { NotificacaoClicada } from '../preload/tipos';

/** O mínimo da classe `Notification` do Electron (simulado nos testes). */
export interface NotificacaoSistema {
  on(evento: 'click' | 'close', ouvinte: () => void): unknown;
  show(): void;
}

export interface FabricaNotificacao {
  suportada(): boolean;
  criar(opcoes: { title: string; body: string; silent?: boolean }): NotificacaoSistema;
}

export class Notificador {
  // Referências vivas: sem elas o GC pode coletar a notificação e o clique se perde.
  private readonly vivas = new Set<NotificacaoSistema>();

  constructor(
    private readonly fabrica: FabricaNotificacao,
    private readonly aoClicar: (alvo: NotificacaoClicada) => void,
  ) {}

  mostrar(dados: Notificacao): boolean {
    if (!this.fabrica.suportada()) return false;
    const titulo = dados.titulo.trim().slice(0, 60) || 'ZapDesk';
    const corpo = dados.corpo.trim().slice(0, 240);
    const notificacao = this.fabrica.criar({ title: titulo, body: corpo, silent: dados.tipo === 'acao' ? false : undefined });
    const alvo: NotificacaoClicada = { conversa_id: dados.conversa_id, automacao_id: dados.automacao_id };
    notificacao.on('click', () => {
      this.vivas.delete(notificacao);
      this.aoClicar(alvo);
    });
    notificacao.on('close', () => this.vivas.delete(notificacao));
    this.vivas.add(notificacao);
    notificacao.show();
    return true;
  }
}
