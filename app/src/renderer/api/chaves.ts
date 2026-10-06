// Chaves do TanStack Query — uma fonte única para consultas e para as atualizações por evento.
import type { Id } from '@zapdesk/cliente-motor';

export const chaves = {
  sistema: ['sistema'] as const,
  contas: ['contas'] as const,
  conversas: (contaId: Id) => ['conversas', contaId] as const,
  conversa: (id: Id) => ['conversa', id] as const,
  mensagens: (conversaId: Id) => ['mensagens', conversaId] as const,
  busca: (contaId: Id, q: string) => ['busca', contaId, q] as const,
  contatos: (contaId: Id) => ['contatos', contaId] as const,
  contato: (id: Id) => ['contato', id] as const,
  status: (contaId: Id) => ['status', contaId] as const,
  figurinhas: (contaId: Id) => ['figurinhas', contaId] as const,
  etiquetas: ['etiquetas'] as const,
  templates: ['templates'] as const,
  leads: ['leads'] as const,
  lead: (id: Id) => ['lead', id] as const,
  disparos: ['disparos'] as const,
  disparo: (id: Id) => ['disparo', id] as const,
  destinatarios: (disparoId: Id) => ['destinatarios', disparoId] as const,
  // 002 — funil
  funis: ['funis'] as const,
  funil: (id: Id) => ['funil', id] as const,
  /** Prefixo; a consulta completa é `[...cards(funil), etapa, busca]`. */
  cards: (funilId: Id) => ['cards', funilId] as const,
  historicoFunil: (funilId: Id, leadId?: Id) => (leadId ? (['historico-funil', funilId, leadId] as const) : (['historico-funil', funilId] as const)),
  funisDoLead: (leadId: Id) => ['funis-lead', leadId] as const,
  // 002 — automações
  automacoes: ['automacoes'] as const,
  automacao: (id: Id) => ['automacao', id] as const,
  /** Prefixo de todas as listas de execuções (`['execucoes', automacaoId | 'todas', estado]`). */
  execucoes: ['execucoes'] as const,
  execucoesAutomacao: (automacaoId: Id | null, estado?: string) => ['execucoes', automacaoId ?? 'todas', estado ?? 'todos'] as const,
  execucao: (id: Id) => ['execucao', id] as const,
  sessoes: (automacaoId: Id) => ['sessoes', automacaoId] as const,
  estadoConversaAutomacoes: (conversaId: Id) => ['conversa-automacoes', conversaId] as const,
  pausas: ['pausas'] as const,
  configuracaoAutomacoes: ['configuracao-automacoes'] as const,
  configuracaoIA: ['configuracao-ia'] as const,
  /** Nomes + "usado por" vindos do motor. */
  segredos: ['segredos'] as const,
  /** Nomes + máscaras do cofre do app (IPC). */
  segredosLocais: ['segredos-locais'] as const,
  arquivos: (automacaoId: Id) => ['arquivos', automacaoId] as const,
  sdk: ['sdk-automacao'] as const,
  modelosProjeto: ['modelos-projeto'] as const,
};
