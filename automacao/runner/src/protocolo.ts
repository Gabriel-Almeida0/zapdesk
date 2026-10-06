// Tipos do protocolo motor ↔ runner v1 (specs/002-automacoes/contracts/runner-protocolo.md).
// JSON-RPC 2.0 em NDJSON sobre stdin/stdout; campos em snake_case. As estruturas que já têm
// forma na SDK (`info`, `argumento`) chegam em camelCase; o runner também aceita snake_case nelas
// (ver `normalizar.ts`), para tolerar as duas grafias vindas do motor.

export const VERSAO_PROTOCOLO = 1 as const;
/** Máximo de bytes por linha NDJSON (acima disso: erro -32600 e a linha é descartada). */
export const LIMITE_LINHA = 1024 * 1024;

// ---------- JSON-RPC ----------

export type IdRpc = number | string;

export interface RequisicaoRpc<P = unknown> {
  jsonrpc: '2.0';
  id: IdRpc;
  method: string;
  params?: P;
}

export interface NotificacaoRpc<P = unknown> {
  jsonrpc: '2.0';
  method: string;
  params?: P;
}

export interface ErroRpcCorpo {
  code: number;
  message: string;
  data?: unknown;
}

export interface RespostaRpcOk<R = unknown> {
  jsonrpc: '2.0';
  id: IdRpc;
  result: R;
}

export interface RespostaRpcErro {
  jsonrpc: '2.0';
  id: IdRpc | null;
  error: ErroRpcCorpo;
}

export type MensagemRpc = RequisicaoRpc | NotificacaoRpc | RespostaRpcOk | RespostaRpcErro;

// ---------- códigos de erro ----------

export const CODIGOS_RPC = {
  parse: -32700,
  requisicao_invalida: -32600,
  metodo_inexistente: -32601,
  parametros_invalidos: -32602,
  interno: -32603,
} as const;

/** Códigos do domínio (tabela "Códigos de erro"). */
export const CODIGOS_ERRO = {
  permissao_negada: 1001,
  validacao: 1002,
  nao_encontrado: 1003,
  bloqueado: 1004,
  ia_nao_configurada: 1005,
  ia_erro: 1006,
  execucao_encerrada: 1007,
  limite: 1008,
  conta_indisponivel: 1009,
  segredo: 1010,
  bundle_invalido: 2001,
  erro_usuario: 2002,
  handler_ausente: 2003,
} as const;

export type NomeCodigoErro = keyof typeof CODIGOS_ERRO;

/** `error.data` dos erros de domínio vindos do motor. */
export interface DadosErroMotor {
  codigo?: NomeCodigoErro | string;
  permissao?: string;
  campos?: Record<string, string>;
  motivo?: string;
  status?: number | null;
  request_id?: string | null;
}

// ---------- motor → runner ----------

export type NomeHandler = 'aoReceberMensagem' | 'aoAgendar' | 'aoExecutar' | 'aoEvento';

export interface ParamsInicializar {
  protocolo: number;
  automacao: { id: string; nome: string; versao: number };
  /** Caminho absoluto do bundle `.mjs` compilado pelo motor. */
  bundle: string;
  hash: string;
  permissoes: string[];
  /** Só os segredos declarados em automacao.json e existentes. */
  segredos: Record<string, string>;
  fuso: string;
}

export interface ResultadoInicializar {
  handlers: NomeHandler[];
}

export interface ContatoProtocolo {
  id: string;
  nome: string | null;
  nome_push: string | null;
  telefone: string | null;
  notas: string | null;
  etiquetas: { id: string; nome: string; cor: string }[];
}

export interface ConversaProtocolo {
  id: string;
  conta_id: string;
  tipo: 'individual' | 'grupo';
  nome: string | null;
  telefone: string | null;
  lead_id: string | null;
  contato: ContatoProtocolo | null;
}

/** `InfoExecucao` da SDK sem `prazoMs` (camelCase; snake_case também aceito). */
export interface InfoProtocolo {
  id?: string;
  automacaoId?: string;
  automacaoNome?: string;
  gatilho?: { tipo: string; dados: Record<string, unknown> };
  origem?: string;
  simulacao?: boolean;
  iniciadaEm?: string;
  [chave: string]: unknown;
}

export interface ParamsExecutar {
  execucao_id: string;
  handler: NomeHandler;
  simulacao: boolean;
  /** O motor mata o processo em `prazo_ms + 2000`. */
  prazo_ms: number;
  info: InfoProtocolo;
  conversa: ConversaProtocolo | null;
  argumento: unknown;
}

export interface ResultadoExecutar {
  retorno: unknown;
}

export interface ParamsCancelar {
  execucao_id: string;
}

export interface ResultadoPing {
  ok: true;
}

// ---------- runner → motor: notificações ----------

export interface ParamsPronto {
  versao_runner: string;
  protocolo: number;
  node: string;
}

export type NivelLog = 'debug' | 'info' | 'aviso' | 'erro';

export interface ParamsLog {
  execucao_id: string | null;
  nivel: NivelLog;
  /** ≤ 8 KB. */
  texto: string;
  em: string;
}

export interface ParamsHttp {
  execucao_id: string;
  metodo: string;
  url_sem_query: string;
  status: number | null;
  duracao_ms: number;
  erro: string | null;
}

// ---------- runner → motor: requisições ctx.* ----------

export type AlvoProtocolo =
  | { contato_id: string }
  | { lead_id: string }
  | { telefone: string }
  | { conversa_id: string };

export type DestinoProtocolo = { conversa_id: string } | { telefone: string; conta_id?: string };

export type ConteudoProtocolo =
  | { texto: string }
  | { template: string; variaveis?: Record<string, string> }
  | { arquivo_id: string; legenda?: string; como?: string };

export interface MensagemProtocolo {
  id: string;
  conversa_id: string;
  de_mim: boolean;
  automacao_id: string | null;
  tipo: string;
  texto: string | null;
  remetente_nome: string | null;
  enviada_em: string;
}

export interface MensagemEnviadaProtocolo {
  id: string | null;
  conversa_id: string | null;
  simulada: boolean;
  texto: string | null;
}

export interface EtiquetaProtocolo {
  id: string;
  nome: string;
  cor: string;
}

export interface FunilProtocolo {
  id: string;
  nome: string;
  etapas: { id: string; nome: string; cor: string; ordem: number }[];
}

export interface PosicaoFunilProtocolo {
  funil_id: string;
  funil: string;
  etapa_id: string;
  etapa: string;
  desde: string;
}

export interface LeadProtocolo {
  id: string;
  telefone: string;
  nome: string | null;
  campos: Record<string, string>;
  origem: string;
  importado_em: string;
}

export interface TokensProtocolo {
  entrada: number;
  saida: number;
}

/** Parâmetros e resultados de cada método `ctx.*` (todos também levam `execucao_id`). */
export interface MetodosCtx {
  'ctx.conversa.historico': {
    params: { conversa_id?: string; limite?: number; antes?: string };
    resultado: MensagemProtocolo[];
  };
  'ctx.enviar': {
    params: { destino: DestinoProtocolo; conteudo: ConteudoProtocolo; citar_mensagem_id?: string };
    resultado: MensagemEnviadaProtocolo;
  };
  'ctx.reagir': { params: { mensagem_id: string; emoji: string }; resultado: Record<string, never> };
  'ctx.etiquetas.listar': { params: Record<string, never>; resultado: EtiquetaProtocolo[] };
  'ctx.etiquetas.do_contato': { params: { alvo?: AlvoProtocolo }; resultado: EtiquetaProtocolo[] };
  'ctx.etiquetas.adicionar': {
    params: { etiqueta: string; alvo?: AlvoProtocolo };
    resultado: Record<string, never>;
  };
  'ctx.etiquetas.remover': {
    params: { etiqueta: string; alvo?: AlvoProtocolo };
    resultado: Record<string, never>;
  };
  'ctx.funil.listar': { params: Record<string, never>; resultado: FunilProtocolo[] };
  'ctx.funil.posicao': {
    params: { funil: string; alvo?: AlvoProtocolo };
    resultado: PosicaoFunilProtocolo | null;
  };
  'ctx.funil.mover': {
    params: { funil: string; etapa: string; alvo?: AlvoProtocolo };
    resultado: PosicaoFunilProtocolo;
  };
  'ctx.funil.remover': { params: { funil: string; alvo?: AlvoProtocolo }; resultado: Record<string, never> };
  'ctx.leads.atual': { params: Record<string, never>; resultado: LeadProtocolo | null };
  'ctx.leads.obter': { params: { id: string }; resultado: LeadProtocolo | null };
  'ctx.leads.buscar_telefone': { params: { telefone: string }; resultado: LeadProtocolo | null };
  'ctx.leads.atualizar': {
    params: { nome?: string | null; campos?: Record<string, string | null>; alvo?: AlvoProtocolo };
    resultado: LeadProtocolo;
  };
  'ctx.memoria.obter': { params: { chave: string; escopo: string }; resultado: { valor: unknown } };
  'ctx.memoria.definir': {
    params: { chave: string; escopo: string; valor: unknown };
    resultado: Record<string, never>;
  };
  'ctx.memoria.remover': { params: { chave: string; escopo: string }; resultado: { removida: boolean } };
  'ctx.memoria.listar': {
    params: { escopo: string; prefixo?: string };
    resultado: { chave: string; valor: unknown }[];
  };
  'ctx.ia.gerar': {
    params: {
      prompt?: string;
      mensagens?: { papel: string; texto: string }[];
      sistema?: string;
      modelo?: string;
      max_tokens?: number;
    };
    resultado: { texto: string; modelo: string; motivo_parada: string; tokens: TokensProtocolo };
  };
  'ctx.ia.classificar': {
    params: {
      texto: string;
      categorias: Record<string, string | null>;
      instrucoes?: string;
      modelo?: string;
      sistema?: string;
      max_tokens?: number;
    };
    resultado: { categoria: string; tokens: TokensProtocolo };
  };
  'ctx.ia.extrair': {
    params: {
      texto: string;
      esquema: unknown;
      instrucoes?: string;
      modelo?: string;
      sistema?: string;
      max_tokens?: number;
    };
    resultado: { dados: unknown; tokens: TokensProtocolo };
  };
  'ctx.agendar': {
    params: { em: string; dados?: unknown; na_conversa: boolean };
    resultado: { id: string; em: string };
  };
  'ctx.cancelar_agendamento': { params: { id: string }; resultado: { cancelado: boolean } };
  'ctx.humano.transferir': {
    params: { motivo?: string; mensagem?: string; duracao_min?: number };
    resultado: Record<string, never>;
  };
  'ctx.notificar': { params: { titulo: string; texto: string }; resultado: Record<string, never> };
}

export type MetodoCtx = keyof MetodosCtx;

/** Permissão exigida por método (tabela do contrato; `null` = nenhuma). O motor é quem valida;
 *  o runner usa a tabela só para `ctx.http` (lado runner) e documentação. */
export const PERMISSAO_POR_METODO: Record<MetodoCtx, string | null> = {
  'ctx.conversa.historico': 'ler_conversas',
  'ctx.enviar': 'enviar',
  'ctx.reagir': 'enviar',
  'ctx.etiquetas.listar': 'etiquetas',
  'ctx.etiquetas.do_contato': 'etiquetas',
  'ctx.etiquetas.adicionar': 'etiquetas',
  'ctx.etiquetas.remover': 'etiquetas',
  'ctx.funil.listar': 'funil',
  'ctx.funil.posicao': 'funil',
  'ctx.funil.mover': 'funil',
  'ctx.funil.remover': 'funil',
  'ctx.leads.atual': 'leads',
  'ctx.leads.obter': 'leads',
  'ctx.leads.buscar_telefone': 'leads',
  'ctx.leads.atualizar': 'leads',
  'ctx.memoria.obter': null,
  'ctx.memoria.definir': null,
  'ctx.memoria.remover': null,
  'ctx.memoria.listar': null,
  'ctx.ia.gerar': 'ia',
  'ctx.ia.classificar': 'ia',
  'ctx.ia.extrair': 'ia',
  'ctx.agendar': 'agendar',
  'ctx.cancelar_agendamento': 'agendar',
  'ctx.humano.transferir': 'enviar',
  'ctx.notificar': null,
};
