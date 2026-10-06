// Textos pt-BR, valores padrão e utilitários das automações (formatos de contracts/formatos.md).
import type {
  Acao,
  EstadoExecucao,
  EstadoSessaoChatbot,
  ErroDefinicao,
  Gatilho,
  OrigemExecucao,
  Permissao,
  Regra,
  ResultadoAcao,
  TipoAcao,
  TipoAutomacao,
  TipoGatilho,
} from '@zapdesk/cliente-motor';

export const ROTULO_TIPO_AUTOMACAO: Record<TipoAutomacao, string> = {
  fluxo: 'Fluxo',
  chatbot: 'Chatbot',
  ia: 'IA (código)',
};

export const ROTULO_GATILHO: Record<TipoGatilho, string> = {
  mensagem_recebida: 'Mensagem recebida',
  palavra_chave: 'Palavra-chave',
  lead_importado: 'Lead importado',
  etiqueta: 'Etiqueta adicionada ou removida',
  entrou_etapa: 'Lead entrou numa etapa do funil',
  disparo_respondeu: 'Destinatário de disparo respondeu',
  sem_resposta: 'Sem resposta há um tempo',
  agendamento: 'Agendamento',
  manual: 'Manual (botão Executar)',
};

export const ROTULO_ACAO: Record<TipoAcao, string> = {
  enviar_texto: 'Enviar texto',
  enviar_template: 'Enviar template',
  aguardar: 'Aguardar',
  adicionar_etiqueta: 'Adicionar etiqueta',
  remover_etiqueta: 'Remover etiqueta',
  mover_etapa: 'Mover para etapa do funil',
  remover_do_funil: 'Remover do funil',
  atualizar_nota: 'Atualizar nota do contato',
  atualizar_campo_lead: 'Atualizar campo do lead',
  iniciar_chatbot: 'Iniciar chatbot',
  executar_ia: 'Executar automação de IA',
  adicionar_a_disparo: 'Adicionar a um disparo',
  pausar_automacoes: 'Pausar automações na conversa',
  notificar: 'Notificar (notificação do Mac)',
};

export const ROTULO_REGRA: Record<Regra['tipo'], string> = {
  etiqueta: 'Etiqueta',
  etapa: 'Etapa do funil',
  campo_lead: 'Campo do lead',
  horario: 'Horário e dias',
  texto: 'Texto da mensagem',
  conta: 'Conta',
  variavel: 'Variável',
};

export const ROTULO_ESTADO_EXECUCAO: Record<EstadoExecucao, string> = {
  na_fila: 'Na fila',
  rodando: 'Rodando',
  aguardando: 'Aguardando',
  ok: 'OK',
  erro: 'Erro',
  simulacao: 'Simulação',
  abortada: 'Abortada',
};

export const ROTULO_ORIGEM_EXECUCAO: Record<OrigemExecucao, string> = {
  gatilho: 'Gatilho',
  manual_app: 'Executar (app)',
  manual_mcp: 'Executar (Claude)',
  teste: 'Teste',
  fluxo: 'Chamada por fluxo',
  chatbot: 'Chamada por chatbot',
};

export const ROTULO_RESULTADO_ACAO: Record<ResultadoAcao, string> = {
  ok: 'feita',
  falhou: 'falhou',
  bloqueada: 'bloqueada',
  simulada: 'simulada',
};

export const ROTULO_ESTADO_SESSAO: Record<EstadoSessaoChatbot, string> = {
  ativa: 'Ativa',
  concluida: 'Concluída',
  humano: 'Transferida para humano',
  expirada: 'Expirada',
  abortada: 'Abortada',
};

export const ROTULO_PERMISSAO: Record<Permissao, string> = {
  enviar: 'enviar mensagens',
  ler_conversas: 'ler conversas',
  etiquetas: 'etiquetas',
  funil: 'funil',
  leads: 'leads',
  ia: 'IA (Claude)',
  rede: 'rede (http)',
  agendar: 'agendar',
};

/** Classe do selo de estado (reaproveita `.selo.ok/.erro/.info`). */
export function classeEstadoExecucao(estado: EstadoExecucao): string {
  switch (estado) {
    case 'ok':
      return 'selo ok';
    case 'erro':
      return 'selo erro';
    case 'abortada':
      return 'selo';
    default:
      return 'selo info';
  }
}

export function gatilhoPadrao(tipo: TipoGatilho): Gatilho {
  switch (tipo) {
    case 'mensagem_recebida':
      return { tipo, contem: null, regex: null, primeira_mensagem: false, tipo_conversa: 'individual' };
    case 'palavra_chave':
      return { tipo, palavras: [], modo: 'palavra' };
    case 'lead_importado':
      return { tipo, origens: null };
    case 'etiqueta':
      return { tipo, evento: 'adicionada', etiqueta_id: '' };
    case 'entrou_etapa':
      return { tipo, funil_id: '', etapa_id: '' };
    case 'disparo_respondeu':
      return { tipo, disparo_id: null };
    case 'sem_resposta':
      return { tipo, apos_s: 7200, origem_mensagem: 'qualquer' };
    case 'agendamento':
      return { tipo, cron: '0 9 * * 1-5' };
    case 'manual':
      return { tipo };
  }
}

export function acaoPadrao(tipo: TipoAcao, id: string): Acao {
  switch (tipo) {
    case 'enviar_texto':
      return { id, tipo, texto: '' };
    case 'enviar_template':
      return { id, tipo, template_id: '' };
    case 'aguardar':
      return { id, tipo, duracao_s: 3600 };
    case 'adicionar_etiqueta':
    case 'remover_etiqueta':
      return { id, tipo, etiqueta_id: '' };
    case 'mover_etapa':
      return { id, tipo, funil_id: '', etapa_id: '' };
    case 'remover_do_funil':
      return { id, tipo, funil_id: '' };
    case 'atualizar_nota':
      return { id, tipo, texto: '', modo: 'acrescentar' };
    case 'atualizar_campo_lead':
      return { id, tipo, campo: '', valor: '' };
    case 'iniciar_chatbot':
      return { id, tipo, automacao_id: '' };
    case 'executar_ia':
      return { id, tipo, automacao_id: '', entrada: {}, salvar_em: null };
    case 'adicionar_a_disparo':
      return { id, tipo, disparo_id: '' };
    case 'pausar_automacoes':
      return { id, tipo, duracao_min: 30 };
    case 'notificar':
      return { id, tipo, titulo: '', texto: '' };
  }
}

export function regraPadrao(tipo: Regra['tipo']): Regra {
  switch (tipo) {
    case 'etiqueta':
      return { tipo, operador: 'tem', etiqueta_id: '' };
    case 'etapa':
      return { tipo, operador: 'esta', funil_id: '', etapa_id: null };
    case 'campo_lead':
      return { tipo, campo: '', operador: 'igual', valor: '' };
    case 'horario':
      return { tipo, inicio: '09:00', fim: '18:00', dias: [1, 2, 3, 4, 5] };
    case 'texto':
      return { tipo, operador: 'contem', valor: '' };
    case 'conta':
      return { tipo, conta_ids: [] };
    case 'variavel':
      return { tipo, variavel: '', operador: 'igual', valor: '' };
  }
}

/** Id curto e estável para ações/nós (`[A-Za-z0-9_-]{1,40}`). */
export function novoId(prefixo: string, existentes: Iterable<string>): string {
  const usados = new Set(existentes);
  for (let i = 1; ; i++) {
    const id = `${prefixo}${i}`;
    if (!usados.has(id)) return id;
  }
}

export type UnidadeTempo = 'min' | 'h' | 'd';

export const SEGUNDOS_UNIDADE: Record<UnidadeTempo, number> = { min: 60, h: 3600, d: 86400 };

/** 7200 → {valor: 2, unidade: 'h'} (maior unidade exata). */
export function paraUnidade(segundos: number): { valor: number; unidade: UnidadeTempo } {
  if (segundos > 0 && segundos % 86400 === 0) return { valor: segundos / 86400, unidade: 'd' };
  if (segundos > 0 && segundos % 3600 === 0) return { valor: segundos / 3600, unidade: 'h' };
  return { valor: Math.round(segundos / 60), unidade: 'min' };
}

/** "2 h", "30 min", "1 dia". */
export function textoDuracao(segundos: number): string {
  const { valor, unidade } = paraUnidade(segundos);
  if (unidade === 'd') return valor === 1 ? '1 dia' : `${valor} dias`;
  return `${valor} ${unidade}`;
}

/** Duração de execução: "850 ms", "2,3 s", "1 min 5 s". */
export function textoDuracaoMs(ms: number | null | undefined): string {
  if (ms === null || ms === undefined) return '—';
  if (ms < 1000) return `${Math.round(ms)} ms`;
  // Arredonda antes de separar minutos e segundos (59.960 ms virava "60 s"; 119.600 ms, "1 min 60 s").
  const decimos = Math.round(ms / 100) / 10;
  if (decimos < 60) return `${decimos.toLocaleString('pt-BR', { maximumFractionDigits: 1 })} s`;
  const s = Math.round(ms / 1000);
  return `${Math.floor(s / 60)} min ${s % 60} s`;
}

/** Erros cujo `caminho` começa com o prefixo (ex.: "definicao.acoes[2]"). */
export function errosEm(erros: readonly ErroDefinicao[], prefixo: string): ErroDefinicao[] {
  return erros.filter((e) => e.caminho === prefixo || e.caminho.startsWith(`${prefixo}.`) || e.caminho.startsWith(`${prefixo}[`));
}

export const MODELOS_CLAUDE_PADRAO = [
  { id: 'claude-sonnet-5', nome: 'Claude Sonnet 5 (padrão)', aviso: null },
  { id: 'claude-opus-5-5', nome: 'Claude Opus 5.5', aviso: null },
  { id: 'claude-haiku-4-5-20251001', nome: 'Claude Haiku 4.5', aviso: 'Modelo mais antigo, com aposentadoria prevista pela Anthropic.' },
] as const;
