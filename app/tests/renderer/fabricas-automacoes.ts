// Fábricas de objetos da feature 002 para os testes do renderer.
import type { Automacao, Card, Etapa, Execucao, ExecucaoDetalhe, Funil } from '@zapdesk/cliente-motor';

export function etapa(parcial: Partial<Etapa> = {}): Etapa {
  return { id: 'e1', funil_id: 'f1', nome: 'Novo', cor: '#53bdeb', ordem: 0, total_cards: 0, ...parcial };
}

export function funil(parcial: Partial<Funil> = {}): Funil {
  return {
    id: 'f1',
    nome: 'Prospecção',
    ordem: 0,
    etapas: [
      etapa({ id: 'e1', nome: 'Novo', ordem: 0, total_cards: 2 }),
      etapa({ id: 'e2', nome: 'Qualificando', ordem: 1, cor: '#ffbc38', total_cards: 0 }),
      etapa({ id: 'e3', nome: 'Proposta', ordem: 2, cor: '#8b64d6', total_cards: 1 }),
    ],
    total_cards: 3,
    criado_em: '2026-09-27T10:00:00-03:00',
    atualizado_em: '2026-09-27T10:00:00-03:00',
    ...parcial,
  };
}

export function card(parcial: Partial<Card> & { nome?: string } = {}): Card {
  const { nome, ...resto } = parcial;
  const leadId = resto.lead_id ?? 'l1';
  return {
    lead_id: leadId,
    funil_id: 'f1',
    etapa_id: 'e1',
    desde: new Date(Date.now() - 2 * 86400_000).toISOString(),
    lead: { id: leadId, telefone: '+5511911112222', nome: nome ?? 'Ana', campos: {} },
    etiquetas: [],
    conversa_id: null,
    conta_id: null,
    ...resto,
  };
}

export function automacao(parcial: Partial<Automacao> = {}): Automacao {
  return {
    id: 'a1',
    tipo: 'fluxo',
    nome: 'Respondeu → quente',
    descricao: null,
    ativa: false,
    contas: null,
    incluir_grupos: false,
    prioridade: 100,
    conta_envio_id: null,
    gatilhos: [{ tipo: 'disparo_respondeu', disparo_id: null }],
    definicao: { versao: 1, condicoes: null, acoes: [{ id: 'acao1', tipo: 'adicionar_etiqueta', etiqueta_id: 'et1' }] },
    limites: { anti_loop: null, tempo_s: null, memoria_mb: null },
    versao: 1,
    ia: null,
    avisos: [],
    erros_seguidos: 0,
    desativada_motivo: null,
    estatisticas_24h: { ok: 3, erro: 1, abortada: 0, duracao_media_ms: 120 },
    sessoes_ativas: 0,
    criada_em: '2026-09-27T10:00:00-03:00',
    atualizada_em: '2026-09-27T10:00:00-03:00',
    ...parcial,
  };
}

export function execucao(parcial: Partial<ExecucaoDetalhe> = {}): ExecucaoDetalhe {
  const base: Execucao = {
    id: 'x1',
    automacao_id: 'a1',
    automacao_nome: 'Respondeu → quente',
    tipo_automacao: 'fluxo',
    automacao_versao: 1,
    gatilho: { tipo: 'mensagem_recebida', dados: {} },
    origem: 'teste',
    origem_execucao_id: null,
    conta_id: null,
    conversa_id: null,
    contato_id: null,
    lead_id: null,
    estado: 'simulacao',
    simulacao: true,
    motivo: null,
    erro: null,
    acoes: [{ tipo: 'enviar', alvo: 'Ana', resultado: 'simulada', detalhe: "'O valor é R$ 10'", em: '2026-09-27T10:00:00-03:00' }],
    tokens: { entrada: 120, saida: 30, por_modelo: { 'claude-sonnet-5': { entrada: 120, saida: 30, chamadas: 1 } } },
    retorno: null,
    retomar_em: null,
    iniciada_em: '2026-09-27T10:00:00-03:00',
    finalizada_em: '2026-09-27T10:00:01-03:00',
    duracao_ms: 850,
  };
  return { ...base, log: 'olá do log', log_truncado: false, erro_stack: null, variaveis: {}, ...parcial };
}
