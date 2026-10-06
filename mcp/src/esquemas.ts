// Esquemas zod compartilhados pelas ferramentas MCP: pedaços de entrada reutilizáveis e as
// saídas (`outputSchema`) espelhando os tipos de contracts/api-http.md › Tipos.
// As saídas usam `looseObject`: campos extras do motor não quebram a validação.
import { z } from 'zod';

// ---------------------------------------------------------------------------
// Entradas reutilizáveis
// ---------------------------------------------------------------------------

export const entradaContaId = z
  .string()
  .min(1)
  .describe(
    'ID da conta de WhatsApp (veja listar_contas). Opcional quando existe exatamente uma conta conectada.',
  );

export const entradaLimite = z
  .number()
  .int()
  .min(1)
  .max(200)
  .describe('Quantos itens trazer por página (1–200, padrão 50).');

export const entradaCursor = z
  .string()
  .min(1)
  .describe('Cursor de paginação: use o `proximo_cursor` devolvido pela chamada anterior.');

export const entradaCaminhoArquivo = z
  .string()
  .min(1)
  .describe('Caminho de um arquivo local nesta máquina (absoluto, relativo ou começando com ~/).');

export const entradaCor = z
  .string()
  .regex(/^#[0-9A-Fa-f]{6}$/, 'Use o formato #RRGGBB.')
  .describe('Cor no formato #RRGGBB, ex.: "#25D366".');

export const entradaValoresPadrao = z
  .record(z.string(), z.string())
  .describe(
    'Valor usado quando um destinatário não tem a variável, ex.: {"nome": "tudo bem"} para "Oi {nome}".',
  );

export const entradaLeadNovo = z.object({
  telefone: z
    .string()
    .min(1)
    .describe('Telefone em qualquer formato, ex.: "(11) 99999-0000" ou "+55 11 99999-0000".'),
  nome: z.string().optional().describe('Nome do lead (vira a variável {nome}).'),
  campos: z
    .record(z.string(), z.string())
    .optional()
    .describe('Campos extras (cada chave vira uma variável {chave} nas mensagens), ex.: {"empresa": "X"}.'),
});

// ---------------------------------------------------------------------------
// Saídas (tipos da API)
// ---------------------------------------------------------------------------

const nulo = <T extends z.ZodType>(esquema: T) => esquema.nullable();

export const saidaEtiqueta = z.looseObject({
  id: z.string(),
  nome: z.string(),
  cor: z.string(),
  total_contatos: z.number(),
});

export const saidaConta = z.looseObject({
  id: z.string(),
  nome: z.string(),
  telefone: nulo(z.string()),
  jid: nulo(z.string()),
  estado: z.string().describe('conectando | conectada | desconectada | banida'),
  online: z.boolean(),
  sincronizando: z.boolean(),
  criada_em: z.string(),
});

export const saidaConversa = z.looseObject({
  id: z.string(),
  conta_id: z.string(),
  jid: z.string(),
  tipo: z.string().describe('individual | grupo'),
  nome: z.string(),
  telefone: nulo(z.string()),
  contato_id: nulo(z.string()),
  nao_lidas: z.number(),
  ultima_mensagem_em: nulo(z.string()),
  ultima_mensagem_resumo: nulo(z.string()),
  etiquetas: z.array(saidaEtiqueta),
});

export const saidaMidia = z.looseObject({
  mimetype: z.string(),
  tamanho: z.number(),
  nome_arquivo: nulo(z.string()),
  duracao_s: nulo(z.number()),
  ptt: z.boolean(),
  url: z.string(),
});

export const saidaMensagem = z.looseObject({
  id: z.string(),
  conta_id: z.string(),
  conversa_id: z.string(),
  wa_id: z.string(),
  remetente_jid: z.string(),
  remetente_nome: nulo(z.string()),
  de_mim: z.boolean(),
  tipo: z.string().describe('texto | imagem | video | audio | documento | figurinha | sistema'),
  texto: nulo(z.string()),
  midia: nulo(saidaMidia),
  citacao: nulo(z.looseObject({ wa_id: z.string(), resumo: z.string(), remetente_nome: nulo(z.string()) })),
  reacoes: z.array(z.looseObject({ remetente_jid: z.string(), emoji: z.string(), de_mim: z.boolean() })),
  editada: z.boolean(),
  apagada: z.boolean(),
  estado: z.string().describe('pendente | enviada | entregue | lida | falhou | recebida'),
  erro: nulo(z.string()),
  disparo_id: nulo(z.string()),
  enviada_em: z.string(),
  pode_editar: z.boolean(),
  pode_apagar: z.boolean(),
});

export const saidaContato = z.looseObject({
  id: z.string(),
  conta_id: z.string(),
  jid: z.string(),
  telefone: nulo(z.string()),
  nome: nulo(z.string()),
  nome_push: nulo(z.string()),
  notas: nulo(z.string()),
  etiquetas: z.array(saidaEtiqueta),
  lead: nulo(z.looseObject({ id: z.string(), origem: z.string(), importado_em: z.string() })),
  conversa_id: nulo(z.string()),
});

export const saidaStatus = z.looseObject({
  id: z.string(),
  conta_id: z.string(),
  contato_jid: z.string(),
  contato_nome: nulo(z.string()),
  tipo: z.string().describe('texto | imagem | video'),
  texto: nulo(z.string()),
  midia: nulo(saidaMidia),
  publicado_em: z.string(),
});

export const saidaLead = z.looseObject({
  id: z.string(),
  telefone: z.string(),
  nome: nulo(z.string()),
  campos: z.record(z.string(), z.string()),
  origem: z.string().describe('csv | colado | contatos | mcp'),
  tem_whatsapp: nulo(z.boolean()),
  importado_em: z.string(),
  ultimo_disparo_em: nulo(z.string()),
});

export const saidaArquivo = z.looseObject({
  id: z.string(),
  nome: z.string(),
  mimetype: z.string(),
  tamanho: z.number(),
  tipo_midia: z.string(),
  url: z.string(),
});

export const saidaTemplate = z.looseObject({
  id: z.string(),
  nome: z.string(),
  texto: z.string(),
  variaveis: z.array(z.string()),
  arquivo: nulo(saidaArquivo),
  criado_em: z.string(),
  atualizado_em: z.string(),
});

export const saidaRelatorioImportacao = z.looseObject({
  total_linhas: z.number(),
  total_novos: z.number(),
  total_ja_existentes: z.number(),
  total_invalidos: z.number(),
  total_duplicados_no_lote: z.number(),
  novos: z.array(z.looseObject({ linha: z.number(), lead_id: z.string(), telefone: z.string() })),
  ja_existentes: z.array(
    z.looseObject({
      linha: z.number(),
      lead_id: z.string(),
      telefone: z.string(),
      importado_em: z.string().describe('Data da importação original do lead.'),
      campos_preenchidos: z.array(z.string()),
    }),
  ),
  invalidos: z.array(z.looseObject({ linha: z.number(), valor: z.string(), motivo: z.string() })),
  duplicados_no_lote: z.array(
    z.looseObject({ linha: z.number(), telefone: z.string(), primeira_linha: z.number() }),
  ),
  lead_ids: z.array(z.string()),
});

export const saidaDisparo = z.looseObject({
  id: z.string(),
  conta_id: z.string(),
  nome: z.string(),
  mensagem: z.string(),
  arquivo: nulo(saidaArquivo),
  ritmo: z.looseObject({
    intervalo_min_s: z.number(),
    intervalo_max_s: z.number(),
    limite_por_hora: nulo(z.number()),
    limite_por_dia: nulo(z.number()),
    pausa_a_cada: nulo(z.number()),
    pausa_duracao_s: nulo(z.number()),
  }),
  inicio_em: nulo(z.string()),
  janela: nulo(z.looseObject({ inicio: z.string(), fim: z.string() })),
  falhas_seguidas_max: z.number(),
  valores_padrao: z.record(z.string(), z.string()),
  estado: z
    .string()
    .describe('rascunho | agendado | enviando | fora_da_janela | pausado | concluido | cancelado'),
  na_fila: z.boolean(),
  motivo_pausa: nulo(z.string()),
  origem: z.string().describe('app | mcp'),
  contadores: z.looseObject({
    total: z.number(),
    pendente: z.number(),
    enviando: z.number(),
    enviado: z.number(),
    entregue: z.number(),
    lido: z.number(),
    respondeu: z.number(),
    falhou: z.number(),
  }),
  proximo_envio_em: nulo(z.string()),
  estimativa_termino_em: nulo(z.string()),
  aviso_ritmo_agressivo: z.boolean(),
  criado_em: z.string(),
  iniciado_em: nulo(z.string()),
  concluido_em: nulo(z.string()),
  cancelado_em: nulo(z.string()),
});

export const saidaDestinatario = z.looseObject({
  id: z.string(),
  disparo_id: z.string(),
  lead_id: z.string(),
  ordem: z.number(),
  telefone: z.string(),
  nome: nulo(z.string()),
  variaveis: z.record(z.string(), z.string()),
  estado: z.string().describe('pendente | enviando | enviado | entregue | lido | falhou | respondeu'),
  motivo_falha: nulo(z.string()),
});

export const saidaOk = z.object({ ok: z.literal(true) });

/** Página da API: `{itens, proximo_cursor}`. */
export function saidaPagina<T extends z.ZodType>(item: T) {
  return z.object({
    itens: z.array(item),
    proximo_cursor: z
      .string()
      .nullable()
      .describe('Passe como `cursor` para buscar a próxima página; null = não há mais.'),
  });
}

// ===========================================================================
// Feature 002 — Automações (specs/002-automacoes/contracts/api-http.md › Tipos)
// ===========================================================================

// ---------------------------------------------------------------------------
// Entradas reutilizáveis
// ---------------------------------------------------------------------------

export const entradaFunilId = z.string().min(1).describe('ID do funil (veja listar_funis).');
export const entradaEtapaId = z.string().min(1).describe('ID da etapa (veja listar_funis: cada funil traz suas etapas).');
export const entradaLeadId = z.string().min(1).describe('ID do lead (veja listar_leads ou listar_cards_funil).');
export const entradaAutomacaoId = z.string().min(1).describe('ID da automação (veja listar_automacoes).');
export const entradaConversaId = z.string().min(1).describe('ID da conversa (veja listar_conversas).');

/** Objeto JSON livre (gatilhos, definições…): o motor valida e devolve erros com o caminho do campo. */
export const entradaObjetoJson = z.record(z.string(), z.unknown());

// ---------------------------------------------------------------------------
// Saídas
// ---------------------------------------------------------------------------

export const saidaEtapa = z.looseObject({
  id: z.string(),
  funil_id: z.string(),
  nome: z.string(),
  cor: z.string(),
  ordem: z.number(),
  total_cards: z.number(),
});

export const saidaFunil = z.looseObject({
  id: z.string(),
  nome: z.string(),
  ordem: z.number(),
  etapas: z.array(saidaEtapa),
  total_cards: z.number(),
  criado_em: z.string(),
  atualizado_em: z.string(),
});

export const saidaCard = z.looseObject({
  lead_id: z.string(),
  funil_id: z.string(),
  etapa_id: z.string(),
  desde: z.string().describe('Desde quando o lead está nesta etapa.'),
  lead: z.looseObject({
    id: z.string(),
    telefone: z.string(),
    nome: nulo(z.string()),
    campos: z.record(z.string(), z.string()),
  }),
  etiquetas: z.array(saidaEtiqueta),
  conversa_id: nulo(z.string()),
  conta_id: nulo(z.string()),
});

export const saidaMovimentoFunil = z.looseObject({
  id: z.string(),
  lead_id: z.string(),
  funil_id: z.string(),
  etapa_origem_id: nulo(z.string()),
  etapa_origem_nome: nulo(z.string()),
  etapa_destino_id: nulo(z.string()),
  etapa_destino_nome: nulo(z.string()),
  origem: z.string().describe('app | mcp | automacao'),
  automacao_id: nulo(z.string()),
  execucao_id: nulo(z.string()),
  em: z.string(),
});

export const saidaErroDefinicao = z.looseObject({
  caminho: z.string().describe('Caminho JSON do campo, ex.: "definicao.acoes[2].etapa_id".'),
  no_id: nulo(z.string()),
  acao_id: nulo(z.string()),
  mensagem: z.string(),
});

export const saidaErroCompilacao = z.looseObject({
  arquivo: z.string(),
  linha: z.number(),
  coluna: z.number(),
  mensagem: z.string(),
  tipo: z.string().describe('sintaxe | importacao | manifesto | resolucao | outro'),
});

export const saidaAutomacao = z.looseObject({
  id: z.string(),
  tipo: z.string().describe('fluxo | chatbot | ia'),
  nome: z.string(),
  descricao: nulo(z.string()),
  ativa: z.boolean(),
  contas: nulo(z.array(z.string())),
  incluir_grupos: z.boolean(),
  prioridade: z.number(),
  conta_envio_id: nulo(z.string()),
  gatilhos: z.array(z.looseObject({ tipo: z.string() })),
  definicao: nulo(z.looseObject({ versao: z.number() })),
  limites: z.looseObject({}),
  versao: z.number(),
  ia: nulo(
    z.looseObject({
      pasta: z.string(),
      permissoes: z.array(z.string()),
      segredos: z.array(z.string()),
      hash_compilado: nulo(z.string()),
      compilacao_ok: z.boolean(),
      erros_compilacao: z.array(saidaErroCompilacao),
      compilado_em: nulo(z.string()),
      rodando_versao_anterior: z.boolean(),
    }),
  ),
  avisos: z.array(saidaErroDefinicao),
  erros_seguidos: z.number(),
  desativada_motivo: nulo(z.string()),
  estatisticas_24h: z.looseObject({
    ok: z.number(),
    erro: z.number(),
    abortada: z.number(),
    duracao_media_ms: nulo(z.number()),
  }),
  sessoes_ativas: z.number(),
  criada_em: z.string(),
  atualizada_em: z.string(),
});

export const saidaAcaoRegistrada = z.looseObject({
  tipo: z.string(),
  alvo: nulo(z.string()),
  resultado: z.string().describe('ok | falhou | bloqueada | simulada'),
  detalhe: nulo(z.string()),
  em: z.string(),
});

export const saidaExecucao = z.looseObject({
  id: z.string(),
  automacao_id: z.string(),
  automacao_nome: z.string(),
  tipo_automacao: z.string(),
  automacao_versao: z.number(),
  gatilho: z.looseObject({ tipo: z.string(), dados: z.record(z.string(), z.unknown()) }),
  origem: z.string().describe('gatilho | manual_app | manual_mcp | teste | fluxo | chatbot'),
  origem_execucao_id: nulo(z.string()),
  conta_id: nulo(z.string()),
  conversa_id: nulo(z.string()),
  contato_id: nulo(z.string()),
  lead_id: nulo(z.string()),
  estado: z.string().describe('na_fila | rodando | aguardando | ok | erro | simulacao | abortada'),
  simulacao: z.boolean(),
  motivo: nulo(z.string()),
  erro: nulo(z.string()),
  acoes: z.array(saidaAcaoRegistrada),
  tokens: z.looseObject({ entrada: z.number(), saida: z.number() }),
  retorno: z.unknown(),
  retomar_em: nulo(z.string()),
  iniciada_em: z.string(),
  finalizada_em: nulo(z.string()),
  duracao_ms: nulo(z.number()),
});

export const saidaExecucaoDetalhe = saidaExecucao.extend({
  log: z.string(),
  log_truncado: z.boolean(),
  erro_stack: nulo(z.string()),
  variaveis: z.record(z.string(), z.unknown()),
});

export const saidaPausa = z.looseObject({
  conversa_id: z.string(),
  motivo: z.string().describe('humano | anti_loop | manual'),
  ate: nulo(z.string()).describe('null = sem prazo.'),
  automacao_id: nulo(z.string()),
  criada_em: z.string(),
});

export const saidaArquivoProjeto = z.looseObject({
  caminho: z.string(),
  tamanho: z.number(),
  hash: z.string(),
  atualizado_em: z.string(),
});

export const saidaConteudoArquivo = z.looseObject({
  caminho: z.string(),
  conteudo: z.string(),
  hash: z.string().describe('Passe como hash_anterior em escrever_arquivo_automacao para não sobrescrever mudanças alheias.'),
  atualizado_em: z.string(),
});

export const saidaResultadoCompilacao = z.looseObject({
  ok: z.boolean(),
  erros: z.array(saidaErroCompilacao),
  avisos: z.array(saidaErroCompilacao),
  hash: nulo(z.string()),
  handlers: z.array(z.string()),
  duracao_ms: z.number(),
});

export const saidaModeloProjeto = z.looseObject({
  id: z.string(),
  nome: z.string(),
  descricao: z.string(),
});

export const saidaSegredo = z.looseObject({
  nome: z.string(),
  reservado: z.boolean(),
  usado_por: z.array(z.looseObject({ automacao_id: z.string(), nome: z.string() })),
});

export const saidaConfiguracaoAutomacoes = z.looseObject({
  anti_loop_mensagens: z.number(),
  anti_loop_janela_min: z.number(),
  pausa_anti_loop_min: z.number(),
  pausa_humana_min: z.number(),
  primeiros_contatos_hora: z.number(),
  tempo_ia_s: z.number(),
  memoria_ia_mb: z.number(),
  processos_ia_max: z.number(),
  ociosidade_ia_min: z.number(),
  pausa_geral: z.boolean(),
});

export const saidaConfiguracaoIA = z.looseObject({
  modelo_padrao: z.string(),
  modelos: z.array(z.looseObject({ id: z.string(), nome: z.string(), aviso: nulo(z.string()) })),
  chave_configurada: z.boolean(),
});
