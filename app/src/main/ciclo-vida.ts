// Regras de fechar/sair (spec FR-001/FR-038, research.md › ciclo de vida), sem dependência do
// Electron para poder ser testada:
// - fechar janela ou Cmd+Q SEM disparo ativo → encerra motor e sai;
// - COM disparo ativo → avisa, esconde janela e dock, mostra a bandeja com o progresso;
// - quando os ativos chegam a 0 em segundo plano → notificação com o resumo e encerramento total;
// - "Abrir ZapDesk" na bandeja → volta ao normal.
import type { Disparo } from '@zapdesk/cliente-motor';

export const AVISO_SEGUNDO_PLANO =
  'Há um disparo em andamento. O ZapDesk vai continuar na barra de menu até terminar.';

/** Espera curta por eventos `disparo.finalizado` que chegam logo depois de `ativos = 0`. */
export const ESPERA_RESUMO_MS = 1500;

export interface AcoesCiclo {
  esconderJanela(): void;
  mostrarJanela(): void;
  ocultarDock(): void;
  mostrarDock(): void;
  criarBandeja(): void;
  atualizarBandeja(resumo: ResumoProgresso): void;
  destruirBandeja(): void;
  avisar(mensagem: string): void;
  notificar(titulo: string, corpo: string): void;
  /** Encerra o motor, apaga o runtime.json e sai do app. */
  encerrar(): void;
  agendar?: (fn: () => void, ms: number) => unknown;
}

export interface ResumoProgresso {
  disparos: number;
  processados: number;
  total: number;
  /** 0–100. */
  percentual: number;
  texto: string;
}

export type DecisaoFechar = 'encerrar' | 'segundo_plano' | 'ignorar';

const ESTADOS_ATIVOS = new Set(['agendado', 'enviando', 'fora_da_janela']);

export function disparoAtivo(d: Disparo): boolean {
  if (d.estado === 'agendado') return d.iniciado_em !== null;
  return ESTADOS_ATIVOS.has(d.estado);
}

export function resumirProgresso(disparos: Iterable<Disparo>): ResumoProgresso {
  let total = 0;
  let processados = 0;
  let quantidade = 0;
  for (const d of disparos) {
    quantidade += 1;
    total += d.contadores.total;
    processados += d.contadores.total - d.contadores.pendente - d.contadores.enviando;
  }
  const percentual = total > 0 ? Math.floor((processados / total) * 100) : 0;
  const texto =
    quantidade === 0
      ? 'Nenhum disparo em andamento'
      : quantidade === 1
        ? `Disparo em andamento: ${processados} de ${total}`
        : `${quantidade} disparos em andamento: ${processados} de ${total}`;
  return { disparos: quantidade, processados, total, percentual, texto };
}

export class CicloVida {
  private readonly acoes: AcoesCiclo;
  private readonly agendar: (fn: () => void, ms: number) => unknown;
  private ativos = 0;
  private segundoPlano = false;
  private encerrando = false;
  private finalizando = false;
  private readonly resumos: string[] = [];
  private readonly progresso = new Map<string, Disparo>();

  constructor(acoes: AcoesCiclo) {
    this.acoes = acoes;
    this.agendar = acoes.agendar ?? ((fn, ms) => setTimeout(fn, ms));
  }

  get emSegundoPlano(): boolean {
    return this.segundoPlano;
  }

  get disparosAtivos(): number {
    return this.ativos;
  }

  get estaEncerrando(): boolean {
    return this.encerrando;
  }

  /** Evento `disparos.ativos` (ou `GET /sistema` ao conectar). */
  atualizarAtivos(total: number): void {
    this.ativos = Math.max(0, total);
    if (this.segundoPlano && this.ativos === 0) this.finalizarSegundoPlano();
  }

  /** Evento `disparo.atualizado`: mantém o progresso mostrado na bandeja. */
  atualizarDisparo(disparo: Disparo): void {
    if (disparoAtivo(disparo)) this.progresso.set(disparo.id, disparo);
    else this.progresso.delete(disparo.id);
    if (this.segundoPlano) this.acoes.atualizarBandeja(this.resumo());
  }

  /** Evento `disparo.finalizado`. */
  registrarFinalizado(resumo: string, disparoId?: string): void {
    if (disparoId) this.progresso.delete(disparoId);
    if (this.segundoPlano) {
      this.resumos.push(resumo);
      this.acoes.atualizarBandeja(this.resumo());
    }
  }

  resumo(): ResumoProgresso {
    return resumirProgresso(this.progresso.values());
  }

  /** Fechar a janela ou Cmd+Q. */
  pedirFechar(): DecisaoFechar {
    if (this.encerrando) return 'ignorar';
    if (this.ativos > 0) {
      if (!this.segundoPlano) this.entrarSegundoPlano();
      return 'segundo_plano';
    }
    this.encerrar();
    return 'encerrar';
  }

  /** "Abrir ZapDesk" na bandeja, clique no Dock, segunda instância ou `open -b` do MCP. */
  reabrir(): void {
    if (this.encerrando) return;
    if (this.segundoPlano) {
      this.segundoPlano = false;
      this.resumos.length = 0;
      this.acoes.destruirBandeja();
      this.acoes.mostrarDock();
    }
    this.acoes.mostrarJanela();
  }

  /** "Sair agora" na bandeja: o motor pausa os disparos (`app_fechado`). */
  forcarSaida(): void {
    this.encerrar();
  }

  /** O motor caiu de vez enquanto o app estava só na bandeja: avisa e sai. */
  motorFalhou(detalhe: string): void {
    if (!this.segundoPlano || this.encerrando) return;
    this.acoes.notificar('ZapDesk', `O WhatsApp parou e o disparo ficou pausado. ${detalhe}`.trim());
    this.encerrar();
  }

  private entrarSegundoPlano(): void {
    this.segundoPlano = true;
    this.resumos.length = 0;
    this.acoes.avisar(AVISO_SEGUNDO_PLANO);
    this.acoes.esconderJanela();
    this.acoes.ocultarDock();
    this.acoes.criarBandeja();
    this.acoes.atualizarBandeja(this.resumo());
  }

  private finalizarSegundoPlano(): void {
    if (this.finalizando) return;
    this.finalizando = true;
    this.agendar(() => {
      this.finalizando = false;
      // Um disparo novo pode ter começado (ex.: pelo MCP) durante a espera.
      if (!this.segundoPlano || this.ativos > 0 || this.encerrando) return;
      const corpo = this.resumos.length > 0 ? this.resumos.join('\n') : 'Disparo concluído.';
      this.acoes.notificar('ZapDesk', corpo);
      this.encerrar();
    }, ESPERA_RESUMO_MS);
  }

  private encerrar(): void {
    if (this.encerrando) return;
    this.encerrando = true;
    this.acoes.destruirBandeja();
    this.acoes.encerrar();
  }
}
