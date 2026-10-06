// Tipos compartilhados entre o processo principal, o preload e o renderer (IPC).
import type { EventoMotor, MotivoRecarga } from '@zapdesk/cliente-motor';

export interface ConexaoMotor {
  porta: number;
  token: string;
  versao: string;
  whatsapp: 'real' | 'falso';
  pidMotor: number;
}

/** Estado do motor visto pela janela. */
export type EstadoMotor =
  | { fase: 'ligando' }
  | { fase: 'religando' }
  | { fase: 'pronto'; conexao: ConexaoMotor }
  | { fase: 'erro'; mensagem: string; detalhe: string };

/** Requisição HTTP ao motor feita pelo processo principal (evita CORS do renderer). */
export interface RequisicaoIpc {
  url: string;
  metodo: string;
  cabecalhos: Record<string, string>;
  corpo: Uint8Array | null;
}

export interface RespostaIpc {
  status: number;
  cabecalhos: Record<string, string>;
  corpo: Uint8Array;
}

/** Caminhos reais para configurar o MCP (Ajustes › Usar com Claude). */
export interface InfoMcp {
  executavel: string;
  script: string;
  scriptExiste: boolean;
  empacotado: boolean;
  pastaDados: string;
}

export interface ResultadoSalvar {
  salvo: boolean;
  caminho?: string;
}

/** Segredo visto pela janela (002): nunca o valor, só nome e máscara ("sk-ant-…a1b2"). */
export interface SegredoLocal {
  nome: string;
  mascara: string;
}

export interface ResultadoSegredos {
  segredos: SegredoLocal[];
  /** Mensagem pt-BR quando a operação falhou (ex.: nome inválido, Keychain indisponível). */
  erro: string | null;
}

/** Clique numa notificação do macOS gerada pelo evento `notificacao` (002). */
export interface NotificacaoClicada {
  conversa_id: string | null;
  automacao_id: string | null;
}

export type OuvinteEvento = (evento: EventoMotor) => void;
export type OuvinteRecarga = (motivo: MotivoRecarga) => void;

export const CANAIS = {
  estadoMotor: 'motor:estado',
  obterEstado: 'motor:obter-estado',
  reiniciar: 'motor:reiniciar',
  requisitar: 'motor:requisitar',
  evento: 'motor:evento',
  recarregar: 'motor:recarregar',
  exportarRelatorio: 'app:exportar-relatorio',
  infoMcp: 'app:info-mcp',
  copiar: 'app:copiar',
  abrirExterno: 'app:abrir-externo',
  mostrarNoFinder: 'app:mostrar-no-finder',
  abrirArquivoMotor: 'app:abrir-arquivo-motor',
  // 002 — automações
  segredosListar: 'segredos:listar',
  segredosDefinir: 'segredos:definir',
  segredosRemover: 'segredos:remover',
  abrirPastaExterna: 'app:abrir-pasta-externa',
  notificacaoClicada: 'app:notificacao-clicada',
} as const;
