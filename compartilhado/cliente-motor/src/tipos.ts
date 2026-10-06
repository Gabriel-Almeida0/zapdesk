// Tipos do contrato do motor do ZapDesk — v1.
// Fonte: specs/001-zapdesk-mvp/contracts/api-http.md (› Tipos), eventos-ws.md e runtime.md,
// mais os acréscimos de specs/002-automacoes/contracts/ (seção "Feature 002" abaixo).
// Qualquer mudança aqui DEVE acompanhar uma mudança no contrato (Constituição III).

/** ULID em texto. */
export type Id = string;
/** Data RFC 3339 com fuso local, ex.: "2026-09-27T20:11:00-03:00". */
export type DataHora = string;
/** Telefone em E.164, ex.: "+5511999990000". */
export type Telefone = string;
/** Horário "HH:MM". */
export type Horario = string;

// ---------------------------------------------------------------------------
// Paginação e erros
// ---------------------------------------------------------------------------

export interface Pagina<T> {
  itens: T[];
  proximo_cursor: string | null;
}

export interface ParametrosPagina {
  /** 1..200, padrão 50. */
  limite?: number;
  cursor?: string;
}

export type CodigoErro =
  | 'nao_autorizado'
  | 'host_invalido'
  | 'nao_encontrado'
  | 'validacao'
  | 'conflito'
  | 'transicao_invalida'
  | 'variaveis_faltando'
  | 'conta_indisponivel'
  | 'sem_whatsapp'
  | 'fora_do_prazo'
  | 'anexo_grande_demais'
  | 'tipo_nao_suportado'
  | 'whatsapp_erro'
  | 'interno'
  // 002 — automações
  | 'definicao_invalida'
  | 'compilacao_falhou'
  | 'runner_indisponivel'
  | 'ia_nao_configurada'
  | 'ia_erro';

export const CODIGOS_ERRO: readonly CodigoErro[] = [
  'nao_autorizado',
  'host_invalido',
  'nao_encontrado',
  'validacao',
  'conflito',
  'transicao_invalida',
  'variaveis_faltando',
  'conta_indisponivel',
  'sem_whatsapp',
  'fora_do_prazo',
  'anexo_grande_demais',
  'tipo_nao_suportado',
  'whatsapp_erro',
  'interno',
  'definicao_invalida',
  'compilacao_falhou',
  'runner_indisponivel',
  'ia_nao_configurada',
  'ia_erro',
] as const;

/** Item de `detalhes.faltando` no erro `variaveis_faltando` e em `/disparos/validar`. */
export interface VariavelFaltando {
  destinatario_linha: number;
  telefone: Telefone;
  variaveis: string[];
}

/** Corpo de erro da API: `{"erro":{codigo, mensagem, detalhes}}`. */
export interface CorpoErro {
  erro: {
    codigo: CodigoErro;
    mensagem: string;
    detalhes?: DetalhesErro;
  };
}

/** Detalhes conhecidos por código; outros campos podem aparecer. */
export interface DetalhesErro {
  /** `validacao`: {campo: mensagem}. */
  campos?: Record<string, string>;
  /** `transicao_invalida`. */
  estado_atual?: string;
  /** `variaveis_faltando`. */
  faltando?: VariavelFaltando[];
  total?: number;
  /** `anexo_grande_demais`. */
  limite_bytes?: number;
  tipo_midia?: TipoMidiaArquivo;
  /** `whatsapp_erro`. */
  motivo?: string;
  /** `definicao_invalida` (ErroDefinicao[]) e `compilacao_falhou` (ErroCompilacao[]). */
  erros?: ErroDefinicao[] | ErroCompilacao[];
  /** `conflito` de arquivo de projeto (hash divergente). */
  hash_atual?: string;
  /** `ia_nao_configurada`: "ANTHROPIC_API_KEY". */
  segredo?: string;
  /** `ia_erro`: status HTTP e request id da Claude API. */
  status?: number;
  request_id?: string;
  [chave: string]: unknown;
}

// ---------------------------------------------------------------------------
// Entidades
// ---------------------------------------------------------------------------

export type EstadoConta = 'conectando' | 'conectada' | 'desconectada' | 'banida';

export interface Conta {
  id: Id;
  nome: string;
  telefone: Telefone | null;
  jid: string | null;
  estado: EstadoConta;
  online: boolean;
  sincronizando: boolean;
  criada_em: DataHora;
}

export interface Etiqueta {
  id: Id;
  nome: string;
  /** "#RRGGBB". */
  cor: string;
  total_contatos: number;
}

export type TipoConversa = 'individual' | 'grupo';

export interface Conversa {
  id: Id;
  conta_id: Id;
  jid: string;
  tipo: TipoConversa;
  nome: string;
  telefone: Telefone | null;
  contato_id: Id | null;
  nao_lidas: number;
  ultima_mensagem_em: DataHora | null;
  ultima_mensagem_resumo: string | null;
  etiquetas: Etiqueta[];
}

export interface Midia {
  mimetype: string;
  tamanho: number;
  nome_arquivo: string | null;
  duracao_s: number | null;
  ptt: boolean;
  largura: number | null;
  altura: number | null;
  miniatura_b64: string | null;
  baixada: boolean;
  /** Caminho relativo à raiz do motor, ex.: "/v1/mensagens/{id}/midia". Use `urlBinaria`. */
  url: string;
}

export type TipoMensagem =
  | 'texto'
  | 'imagem'
  | 'video'
  | 'audio'
  | 'documento'
  | 'figurinha'
  | 'sistema';

export type EstadoMensagem = 'pendente' | 'enviada' | 'entregue' | 'lida' | 'falhou' | 'recebida';

export interface Citacao {
  wa_id: string;
  resumo: string;
  remetente_nome: string | null;
}

export interface Reacao {
  remetente_jid: string;
  emoji: string;
  de_mim: boolean;
}

export interface Mensagem {
  id: Id;
  conta_id: Id;
  conversa_id: Id;
  wa_id: string;
  remetente_jid: string;
  remetente_nome: string | null;
  de_mim: boolean;
  tipo: TipoMensagem;
  texto: string | null;
  midia: Midia | null;
  citacao: Citacao | null;
  reacoes: Reacao[];
  editada: boolean;
  apagada: boolean;
  estado: EstadoMensagem;
  erro: string | null;
  disparo_id: Id | null;
  /** 002: automação que enviou (mensagens automáticas); null nas demais. */
  automacao_id: Id | null;
  enviada_em: DataHora;
  pode_editar: boolean;
  pode_apagar: boolean;
}

export type OrigemLead = 'csv' | 'colado' | 'contatos' | 'mcp';

export interface LeadResumo {
  id: Id;
  origem: OrigemLead;
  importado_em: DataHora;
}

export interface Contato {
  id: Id;
  conta_id: Id;
  jid: string;
  telefone: Telefone | null;
  nome: string | null;
  nome_push: string | null;
  notas: string | null;
  etiquetas: Etiqueta[];
  lead: LeadResumo | null;
  conversa_id: Id | null;
}

export type TipoStatus = 'texto' | 'imagem' | 'video';

export interface Status {
  id: Id;
  conta_id: Id;
  contato_jid: string;
  contato_nome: string | null;
  tipo: TipoStatus;
  texto: string | null;
  midia: Midia | null;
  publicado_em: DataHora;
}

/** Item de `GET /contas/{id}/status` (últimas 24 h, agrupado por contato). */
export interface StatusPorContato {
  contato_jid: string;
  contato_nome: string | null;
  itens: Status[];
}

export interface Lead {
  id: Id;
  telefone: Telefone;
  nome: string | null;
  campos: Record<string, string>;
  origem: OrigemLead;
  tem_whatsapp: boolean | null;
  importado_em: DataHora;
  ultimo_disparo_em: DataHora | null;
}

export type TipoMidiaArquivo = 'imagem' | 'video' | 'audio' | 'documento' | 'figurinha';

export interface Arquivo {
  id: Id;
  nome: string;
  mimetype: string;
  tamanho: number;
  tipo_midia: TipoMidiaArquivo;
  /** "/v1/arquivos/{id}/conteudo". Use `urlBinaria`. */
  url: string;
}

export interface Template {
  id: Id;
  nome: string;
  texto: string;
  variaveis: string[];
  arquivo: Arquivo | null;
  criado_em: DataHora;
  atualizado_em: DataHora;
}

export interface Ritmo {
  intervalo_min_s: number;
  intervalo_max_s: number;
  limite_por_hora: number | null;
  limite_por_dia: number | null;
  pausa_a_cada: number | null;
  pausa_duracao_s: number | null;
}

export type Janela = { inicio: Horario; fim: Horario } | null;

export type EstadoDisparo =
  | 'rascunho'
  | 'agendado'
  | 'enviando'
  | 'fora_da_janela'
  | 'pausado'
  | 'concluido'
  | 'cancelado';

export type OrigemDisparo = 'app' | 'mcp';

export interface ContadoresDisparo {
  total: number;
  pendente: number;
  enviando: number;
  enviado: number;
  entregue: number;
  lido: number;
  respondeu: number;
  falhou: number;
}

export interface Disparo {
  id: Id;
  conta_id: Id;
  nome: string;
  mensagem: string;
  arquivo: Arquivo | null;
  ritmo: Ritmo;
  inicio_em: DataHora | null;
  janela: Janela;
  falhas_seguidas_max: number;
  valores_padrao: Record<string, string>;
  estado: EstadoDisparo;
  na_fila: boolean;
  motivo_pausa: string | null;
  origem: OrigemDisparo;
  contadores: ContadoresDisparo;
  proximo_envio_em: DataHora | null;
  estimativa_termino_em: DataHora | null;
  aviso_ritmo_agressivo: boolean;
  criado_em: DataHora;
  iniciado_em: DataHora | null;
  concluido_em: DataHora | null;
  cancelado_em: DataHora | null;
  /** Presente na resposta de `POST /disparos` quando `destinatarios.importar` foi usado. */
  relatorio_importacao?: RelatorioImportacao;
}

export type EstadoDestinatario =
  | 'pendente'
  | 'enviando'
  | 'enviado'
  | 'entregue'
  | 'lido'
  | 'falhou'
  | 'respondeu';

export interface Destinatario {
  id: Id;
  disparo_id: Id;
  lead_id: Id;
  ordem: number;
  telefone: Telefone;
  nome: string | null;
  variaveis: Record<string, string>;
  estado: EstadoDestinatario;
  motivo_falha: string | null;
  mensagem_wa_id: string | null;
  enviando_em: DataHora | null;
  enviado_em: DataHora | null;
  entregue_em: DataHora | null;
  lido_em: DataHora | null;
  respondeu_em: DataHora | null;
  falhou_em: DataHora | null;
}

export type MotivoInvalido = 'vazio' | 'formato_invalido' | 'numero_invalido';

export interface RelatorioImportacao {
  total_linhas: number;
  total_novos: number;
  total_ja_existentes: number;
  total_invalidos: number;
  total_duplicados_no_lote: number;
  novos: { linha: number; lead_id: Id; telefone: Telefone }[];
  ja_existentes: {
    linha: number;
    lead_id: Id;
    telefone: Telefone;
    importado_em: DataHora;
    campos_preenchidos: string[];
  }[];
  invalidos: { linha: number; valor: string; motivo: MotivoInvalido }[];
  duplicados_no_lote: { linha: number; telefone: Telefone; primeira_linha: number }[];
  /** Todos os leads válidos do lote (novos + já existentes), na ordem. */
  lead_ids: Id[];
}

// ---------------------------------------------------------------------------
// Entradas e respostas de rotas
// ---------------------------------------------------------------------------

export interface Saude {
  ok: boolean;
  versao: string;
  whatsapp: ModoWhatsApp;
}

export type ModoWhatsApp = 'real' | 'falso';

export interface Sistema {
  versao: string;
  pasta_dados: string;
  caminho_logs: string;
  whatsapp: ModoWhatsApp;
  disparos_ativos: number;
  contas_conectadas: number;
  /** 002. */
  automacoes_ativas: number;
  /** 002: processos de runner vivos. */
  processos_ia: number;
  /** 002: false sem --runner-exec/--runner-script (IA não executa). */
  runner_disponivel: boolean;
}

export type EventoEnergia = 'suspender' | 'retomar';

export interface QrConta {
  codigo: string;
  expira_em: DataHora;
}

export interface FiltroConversas extends ParametrosPagina {
  busca?: string;
  etiqueta_id?: Id;
  nao_lidas?: boolean;
}

export interface ParametrosMensagens {
  /** ID da mensagem mais antiga já carregada. */
  antes?: Id;
  limite?: number;
}

export interface ResultadoBuscaMensagem {
  mensagem: Mensagem;
  conversa: { id: Id; nome: string };
  trecho: string;
}

export type ComoEnviar = 'auto' | 'voz' | 'figurinha' | 'documento';

export interface EnvioMensagem {
  /** Obrigatório se não houver `arquivo_id`; vira legenda com imagem/vídeo/documento. */
  texto?: string | null;
  arquivo_id?: Id | null;
  como?: ComoEnviar;
  citar_mensagem_id?: Id | null;
}

export interface Figurinha {
  mensagem_id: Id | null;
  arquivo_id: Id | null;
  url: string;
}

export interface FiltroContatos extends ParametrosPagina {
  busca?: string;
  etiqueta_id?: Id;
}

export interface NovaEtiqueta {
  nome: string;
  cor: string;
}

export interface NovoTemplate {
  nome: string;
  texto: string;
  arquivo_id?: Id | null;
}

export interface AlteracaoTemplate {
  nome?: string;
  texto?: string;
  arquivo_id?: Id | null;
}

export interface PreviaImportacao {
  importacao_id: Id;
  nome_arquivo: string;
  colunas: string[];
  /** Até 5 linhas. */
  amostra: string[][];
  total_linhas: number;
  coluna_telefone_sugerida: string | null;
  coluna_nome_sugerida: string | null;
  expira_em: DataHora;
}

export interface OpcoesImportacao {
  ddi_padrao?: string;
  origem?: OrigemLead;
}

/** Forma A: planilha já enviada em `/importacoes/previa`. */
export interface ImportacaoPlanilha extends OpcoesImportacao {
  importacao_id: Id;
  mapeamento: {
    telefone: string;
    nome?: string | null;
    /** Ausente = todas as outras colunas. */
    extras?: string[];
  };
}

/** Forma B: números colados, um por linha. */
export interface ImportacaoColada extends OpcoesImportacao {
  texto_colado: string;
}

/** Forma C: lista estruturada (usada pelo MCP). */
export interface ImportacaoLista extends OpcoesImportacao {
  leads: { telefone: string; nome?: string | null; campos?: Record<string, string> }[];
}

export type CorpoImportacao = ImportacaoPlanilha | ImportacaoColada | ImportacaoLista;

export interface ImportacaoContatos {
  conta_id: Id;
  contato_ids?: Id[];
  etiqueta_ids?: Id[];
}

export interface FiltroLeads extends ParametrosPagina {
  busca?: string;
  origem?: OrigemLead;
}

export interface DestinatariosNovoDisparo {
  lead_ids?: Id[];
  etiqueta_ids?: Id[];
  contato_ids?: Id[];
  importar?: CorpoImportacao | null;
}

export interface NovoDisparo {
  conta_id: Id;
  /** Padrão "Disparo dd/mm hh:mm". */
  nome?: string;
  mensagem?: string;
  template_id?: Id | null;
  arquivo_id?: Id | null;
  destinatarios: DestinatariosNovoDisparo;
  ritmo: Ritmo;
  /** null = agora. */
  inicio_em?: DataHora | null;
  janela?: Janela;
  falhas_seguidas_max?: number;
  valores_padrao?: Record<string, string>;
  /** true = cria e já inicia (MCP sempre true). */
  iniciar?: boolean;
  origem?: OrigemDisparo;
}

/** Campos editáveis de um disparo em `rascunho` (sem destinatários). */
export type AlteracaoDisparo = Partial<Omit<NovoDisparo, 'destinatarios'>>;

export interface ValidacaoDisparo {
  total_destinatarios: number;
  variaveis: string[];
  faltando: VariavelFaltando[];
  previa: { telefone: Telefone; nome: string | null; texto_resolvido: string } | null;
  estimativa_termino_em: DataHora | null;
  aviso_ritmo_agressivo: boolean;
  erros_campos: Record<string, string>;
}

export interface FiltroDisparos extends ParametrosPagina {
  conta_id?: Id;
  estado?: EstadoDisparo;
}

export interface FiltroDestinatarios extends ParametrosPagina {
  estado?: EstadoDestinatario;
  busca?: string;
}

// ---------------------------------------------------------------------------
// Modo falso (/v1/falso/*) — contracts/runtime.md
// ---------------------------------------------------------------------------

export interface FalsoEscanearQr {
  telefone: string;
  nome?: string;
}

export interface FalsoMensagemRecebida {
  de: string;
  texto?: string;
  grupo_jid?: string | null;
  tipo?: TipoMensagem;
  /** Push name do remetente. */
  nome?: string;
  grupo_nome?: string;
  /** Mensagem enviada pelo próprio celular da conta. */
  de_mim?: boolean;
  /** A mídia injetada não poderá ser baixada (testa "Não foi possível baixar."). */
  falhar_download?: boolean;
  citar_wa_id?: string;
  em?: DataHora;
}

/** Resposta de `mensagem-recebida` e `status` no modo falso. */
export interface FalsoInjetada {
  wa_id: string;
}

export interface FalsoMensagemHistorico {
  texto?: string;
  de?: string;
  nome?: string;
  de_mim?: boolean;
  tipo?: TipoMensagem;
  wa_id?: string;
  em?: DataHora;
}

export interface FalsoRecibo {
  wa_id: string;
  tipo: 'entregue' | 'lido';
}

export type FalsoEventoEstado = 'queda_rede' | 'reconectou' | 'logout' | 'ban';

export interface FalsoHistorico {
  /** `jid`: "+5511…", "5511…@s.whatsapp.net" ou "123@g.us". */
  conversas: { jid: string; nome?: string; mensagens: FalsoMensagemHistorico[] }[];
}

export interface FalsoStatus {
  de: string;
  texto: string;
}

export interface FalsoFalhasEnvio {
  telefones: string[];
  erro: string;
}

export type FalsoRelogio = { agora: DataHora } | { avancar_s: number };

/** Registro de algo "enviado" pelo WhatsApp falso (campos vazios omitidos). */
export interface FalsoEnviada {
  conta_id: Id;
  wa_id: string;
  tipo: TipoMensagem | 'reacao' | 'edicao' | 'apagar';
  para: string;
  telefone?: Telefone;
  texto?: string;
  mimetype?: string;
  tamanho?: number;
  nome_arquivo?: string;
  voz?: boolean;
  citacao_wa_id?: string;
  alvo_wa_id?: string;
  emoji?: string;
  em: DataHora;
  [chave: string]: unknown;
}

// ---------------------------------------------------------------------------
// Eventos WebSocket — contracts/eventos-ws.md
// ---------------------------------------------------------------------------

export interface MapaEventos {
  'motor.pronto': { versao: string };
  'conta.atualizada': Conta;
  'conta.qr': QrConta;
  'conta.qr_expirado': Record<string, never>;
  'conta.removida': { id: Id };
  'sincronizacao.progresso': { conversas: number; mensagens: number; concluida: boolean };
  'conversa.atualizada': Conversa;
  'mensagem.nova': Mensagem;
  'mensagem.atualizada': Mensagem;
  'contato.atualizado': Contato;
  'status.novo': Status;
  'etiquetas.alteradas': Record<string, never>;
  'templates.alterados': Record<string, never>;
  'leads.importados': {
    origem: OrigemLead;
    total_novos: number;
    total_ja_existentes: number;
    total_invalidos: number;
  };
  'disparo.atualizado': Disparo;
  'destinatario.atualizado': Destinatario;
  'disparo.finalizado': { disparo: Disparo; resumo: string };
  'disparos.ativos': { total: number };
  'conexao.rede': { conta_id: Id; online: boolean };
  // 002 — automações (specs/002-automacoes/contracts/eventos-ws.md)
  /** `funil_id: null` = a lista de funis mudou. */
  'funil.alterado': { funil_id: Id | null };
  /** `card: null` = saiu do funil. */
  'funil.movido': { movimento: MovimentoFunil; card: Card | null };
  'automacao.atualizada': Automacao;
  'automacao.removida': { id: Id };
  'automacao.arquivos_alterados': { automacao_id: Id; caminhos: string[]; origem: 'api' };
  'automacao.execucao.iniciada': Execucao;
  'automacao.execucao.atualizada': Execucao;
  'automacao.execucao.finalizada': Execucao;
  'chatbot.sessao.iniciada': SessaoChatbot;
  'chatbot.sessao.atualizada': SessaoChatbot;
  'chatbot.sessao.finalizada': SessaoChatbot;
  'conversa.pausa': { conversa_id: Id; pausa: Pausa | null };
  notificacao: Notificacao;
  'segredos.alterados': { nomes: string[] };
  'automacoes.configuracao': ConfiguracaoAutomacoes;
}

export type TipoNotificacao = 'anti_loop' | 'humano' | 'desativada' | 'acao' | 'erro';

/** Dados do evento `notificacao` (o app transforma em notificação do macOS). */
export interface Notificacao {
  titulo: string;
  corpo: string;
  automacao_id: Id | null;
  conversa_id: Id | null;
  tipo: TipoNotificacao;
}

export type TipoEvento = keyof MapaEventos;

export const TIPOS_EVENTO: readonly TipoEvento[] = [
  'motor.pronto',
  'conta.atualizada',
  'conta.qr',
  'conta.qr_expirado',
  'conta.removida',
  'sincronizacao.progresso',
  'conversa.atualizada',
  'mensagem.nova',
  'mensagem.atualizada',
  'contato.atualizado',
  'status.novo',
  'etiquetas.alteradas',
  'templates.alterados',
  'leads.importados',
  'disparo.atualizado',
  'destinatario.atualizado',
  'disparo.finalizado',
  'disparos.ativos',
  'conexao.rede',
  'funil.alterado',
  'funil.movido',
  'automacao.atualizada',
  'automacao.removida',
  'automacao.arquivos_alterados',
  'automacao.execucao.iniciada',
  'automacao.execucao.atualizada',
  'automacao.execucao.finalizada',
  'chatbot.sessao.iniciada',
  'chatbot.sessao.atualizada',
  'chatbot.sessao.finalizada',
  'conversa.pausa',
  'notificacao',
  'segredos.alterados',
  'automacoes.configuracao',
] as const;

/** Envelope de um evento de tipo `T`. */
export interface EnvelopeEvento<T extends TipoEvento = TipoEvento> {
  /** Crescente por execução do motor; lacuna → recarregar via HTTP. */
  seq: number;
  tipo: T;
  /** `null` para eventos globais. */
  conta_id: Id | null;
  em: DataHora;
  dados: MapaEventos[T];
}

/** União discriminada por `tipo` de todos os eventos. */
export type EventoMotor = { [T in TipoEvento]: EnvelopeEvento<T> }[TipoEvento];

// ===========================================================================
// Feature 002 — Automações (funil, fluxos, chatbots e automações de IA)
// Fontes: specs/002-automacoes/contracts/api-http.md (› Tipos), formatos.md, eventos-ws.md e
// runtime.md. Tudo aqui é acréscimo compatível à v1.
// ===========================================================================

// ---------------------------------------------------------------------------
// Formatos — gatilhos (contracts/formatos.md › Gatilho)
// ---------------------------------------------------------------------------

export type TipoConversaGatilho = 'individual' | 'grupo' | 'qualquer';
export type ModoPalavraChave = 'palavra' | 'mensagem_inteira';
export type OrigemMensagemSemResposta = 'qualquer' | 'disparo' | 'automacao' | 'manual';

export interface GatilhoMensagemRecebida {
  tipo: 'mensagem_recebida';
  /** Sem diferenciar maiúsculas/acentos. */
  contem?: string | null;
  /** RE2 (sintaxe Go), 1–500 caracteres; `(?i)` permitido. */
  regex?: string | null;
  /** Só a 1ª mensagem recebida do contato nesta conta. */
  primeira_mensagem?: boolean;
  /** Padrão "individual"; "grupo" exige `incluir_grupos`. */
  tipo_conversa?: TipoConversaGatilho;
}

export interface GatilhoPalavraChave {
  tipo: 'palavra_chave';
  palavras: string[];
  /** Padrão "palavra". */
  modo?: ModoPalavraChave;
}

export interface GatilhoLeadImportado {
  tipo: 'lead_importado';
  /** Ausente/null = todas. */
  origens?: OrigemLead[] | null;
}

export interface GatilhoEtiqueta {
  tipo: 'etiqueta';
  evento: 'adicionada' | 'removida';
  etiqueta_id: Id;
}

export interface GatilhoEntrouEtapa {
  tipo: 'entrou_etapa';
  funil_id: Id;
  etapa_id: Id;
}

export interface GatilhoDisparoRespondeu {
  tipo: 'disparo_respondeu';
  /** null/ausente = qualquer disparo. */
  disparo_id?: Id | null;
}

export interface GatilhoSemResposta {
  tipo: 'sem_resposta';
  /** 60–2.592.000. */
  apos_s: number;
  /** Padrão "qualquer". */
  origem_mensagem?: OrigemMensagemSemResposta;
}

/** Exatamente um de `cron` (5 campos, fuso local) ou `intervalo_s` (≥ 60). */
export type GatilhoAgendamento =
  | { tipo: 'agendamento'; cron: string; intervalo_s?: never }
  | { tipo: 'agendamento'; intervalo_s: number; cron?: never };

export interface GatilhoManual {
  tipo: 'manual';
}

export type Gatilho =
  | GatilhoMensagemRecebida
  | GatilhoPalavraChave
  | GatilhoLeadImportado
  | GatilhoEtiqueta
  | GatilhoEntrouEtapa
  | GatilhoDisparoRespondeu
  | GatilhoSemResposta
  | GatilhoAgendamento
  | GatilhoManual;

export type TipoGatilho = Gatilho['tipo'];

export const TIPOS_GATILHO: readonly TipoGatilho[] = [
  'mensagem_recebida',
  'palavra_chave',
  'lead_importado',
  'etiqueta',
  'entrou_etapa',
  'disparo_respondeu',
  'sem_resposta',
  'agendamento',
  'manual',
] as const;

// ---------------------------------------------------------------------------
// Formatos — condições (contracts/formatos.md › Condições)
// ---------------------------------------------------------------------------

/** `maior`/`menor` são numéricos (texto não numérico = falso). */
export type OperadorCampo =
  | 'igual'
  | 'diferente'
  | 'contem'
  | 'existe'
  | 'nao_existe'
  | 'maior'
  | 'menor';

/** 1 = segunda … 7 = domingo. */
export type DiaSemana = 1 | 2 | 3 | 4 | 5 | 6 | 7;

export interface RegraEtiqueta {
  tipo: 'etiqueta';
  operador: 'tem' | 'nao_tem';
  etiqueta_id: Id;
}

export interface RegraEtapa {
  tipo: 'etapa';
  operador: 'esta' | 'nao_esta';
  funil_id: Id;
  /** null = "em qualquer etapa do funil". */
  etapa_id: Id | null;
}

export interface RegraCampoLead {
  tipo: 'campo_lead';
  campo: string;
  operador: OperadorCampo;
  valor?: string | null;
}

export interface RegraHorario {
  tipo: 'horario';
  /** "HH:MM"; `fim < inicio` atravessa a meia-noite. */
  inicio: Horario;
  fim: Horario;
  dias?: DiaSemana[];
}

export interface RegraTexto {
  tipo: 'texto';
  operador: 'contem' | 'igual' | 'regex';
  valor: string;
}

export interface RegraConta {
  tipo: 'conta';
  conta_ids: Id[];
}

export interface RegraVariavel {
  tipo: 'variavel';
  variavel: string;
  operador: OperadorCampo | 'regex';
  valor: string | null;
}

export type Regra =
  | RegraEtiqueta
  | RegraEtapa
  | RegraCampoLead
  | RegraHorario
  | RegraTexto
  | RegraConta
  | RegraVariavel;

export interface Condicoes {
  /** "todas" = E; "alguma" = OU. */
  modo: 'todas' | 'alguma';
  /** Máx. 20; vazio = sempre verdadeiro. */
  regras: Regra[];
}

// ---------------------------------------------------------------------------
// Formatos — ações (contracts/formatos.md › Ação)
// ---------------------------------------------------------------------------

interface AcaoBase {
  /** Opcional (gerado se ausente); único na definição. `[A-Za-z0-9_-]{1,40}`. */
  id?: string;
}

export interface AcaoEnviarTexto extends AcaoBase {
  tipo: 'enviar_texto';
  /** 1–4.096; aceita variáveis `{nome}`. */
  texto: string;
  valores_padrao?: Record<string, string>;
}

export interface AcaoEnviarTemplate extends AcaoBase {
  tipo: 'enviar_template';
  template_id: Id;
  valores_padrao?: Record<string, string>;
}

/** Não permitida em chatbot. */
export interface AcaoAguardar extends AcaoBase {
  tipo: 'aguardar';
  /** 60–2.592.000. */
  duracao_s: number;
}

export interface AcaoAdicionarEtiqueta extends AcaoBase {
  tipo: 'adicionar_etiqueta';
  etiqueta_id: Id;
}

export interface AcaoRemoverEtiqueta extends AcaoBase {
  tipo: 'remover_etiqueta';
  etiqueta_id: Id;
}

export interface AcaoMoverEtapa extends AcaoBase {
  tipo: 'mover_etapa';
  funil_id: Id;
  etapa_id: Id;
}

export interface AcaoRemoverDoFunil extends AcaoBase {
  tipo: 'remover_do_funil';
  funil_id: Id;
}

export interface AcaoAtualizarNota extends AcaoBase {
  tipo: 'atualizar_nota';
  texto: string;
  /** "acrescentar" = nova linha; nota final ≤ 10.000. */
  modo: 'substituir' | 'acrescentar';
}

export interface AcaoAtualizarCampoLead extends AcaoBase {
  tipo: 'atualizar_campo_lead';
  /** "nome" altera o nome do lead. */
  campo: string;
  /** "" remove o campo. */
  valor: string;
}

/** Não permitida dentro de chatbot; ignorada se já há sessão ativa. */
export interface AcaoIniciarChatbot extends AcaoBase {
  tipo: 'iniciar_chatbot';
  automacao_id: Id;
}

export interface AcaoExecutarIA extends AcaoBase {
  tipo: 'executar_ia';
  automacao_id: Id;
  /** Strings passam por variáveis. */
  entrada?: Record<string, unknown>;
  /** Variável que recebe o retorno (string → texto; objeto → JSON). */
  salvar_em?: string | null;
}

export interface AcaoAdicionarADisparo extends AcaoBase {
  tipo: 'adicionar_a_disparo';
  disparo_id: Id;
}

export interface AcaoPausarAutomacoes extends AcaoBase {
  tipo: 'pausar_automacoes';
  /** null = sem prazo; motivo "manual". */
  duracao_min: number | null;
}

export interface AcaoNotificar extends AcaoBase {
  tipo: 'notificar';
  /** 1–60. */
  titulo: string;
  /** 1–240. */
  texto: string;
}

export type Acao =
  | AcaoEnviarTexto
  | AcaoEnviarTemplate
  | AcaoAguardar
  | AcaoAdicionarEtiqueta
  | AcaoRemoverEtiqueta
  | AcaoMoverEtapa
  | AcaoRemoverDoFunil
  | AcaoAtualizarNota
  | AcaoAtualizarCampoLead
  | AcaoIniciarChatbot
  | AcaoExecutarIA
  | AcaoAdicionarADisparo
  | AcaoPausarAutomacoes
  | AcaoNotificar;

export type TipoAcao = Acao['tipo'];

export const TIPOS_ACAO: readonly TipoAcao[] = [
  'enviar_texto',
  'enviar_template',
  'aguardar',
  'adicionar_etiqueta',
  'remover_etiqueta',
  'mover_etapa',
  'remover_do_funil',
  'atualizar_nota',
  'atualizar_campo_lead',
  'iniciar_chatbot',
  'executar_ia',
  'adicionar_a_disparo',
  'pausar_automacoes',
  'notificar',
] as const;

// ---------------------------------------------------------------------------
// Formatos — fluxo e chatbot
// ---------------------------------------------------------------------------

export interface DefinicaoFluxo {
  versao: 1;
  condicoes: Condicoes | null;
  /** 1–50. */
  acoes: Acao[];
}

/** Só do canvas (ignorada pelo executor). */
export interface PosicaoNo {
  x: number;
  y: number;
}

interface NoBase {
  /** `[A-Za-z0-9_-]{1,40}`, único. */
  id: string;
  posicao?: PosicaoNo;
}

export interface NoInicio extends NoBase {
  tipo: 'inicio';
  proximo: string;
}

export interface NoMensagem extends NoBase {
  tipo: 'mensagem';
  texto: string;
  template_id?: Id | null;
  proximo: string;
}

export interface OpcaoMenu {
  rotulo: string;
  /** Sinônimos normalizados aceitos como resposta. */
  valores: string[];
  proximo: string;
}

export interface NoMenu extends NoBase {
  tipo: 'menu';
  texto: string;
  /** 1–10. */
  opcoes: OpcaoMenu[];
  /** Envia "1 - Preços\n2 - …" após o texto. */
  mostrar_numeros?: boolean;
  /** null = transferir para humano. */
  ao_esgotar?: string | null;
}

export type TipoValidacaoPergunta = 'nenhuma' | 'email' | 'numero' | 'telefone' | 'regex';

export interface ValidacaoPergunta {
  tipo: TipoValidacaoPergunta;
  /** Obrigatório com `regex`. */
  padrao?: string | null;
  mensagem_erro?: string | null;
}

export interface NoPergunta extends NoBase {
  tipo: 'pergunta';
  texto: string;
  /** `[a-z_][a-z0-9_]{0,39}`. */
  variavel: string;
  validacao?: ValidacaoPergunta | null;
  proximo: string;
  ao_esgotar?: string | null;
}

export interface RamoCondicao {
  condicoes: Condicoes;
  proximo: string;
}

export interface NoCondicao extends NoBase {
  tipo: 'condicao';
  ramos: RamoCondicao[];
  senao: string;
}

export interface NoAcao extends NoBase {
  tipo: 'acao';
  /** `aguardar` e `iniciar_chatbot` proibidas. */
  acao: Acao;
  proximo: string;
}

export interface NoIA extends NoBase {
  tipo: 'ia';
  automacao_id: Id;
  entrada?: Record<string, unknown>;
  /** "responder": retorno string é enviado; "variavel": salvo em `variavel`. */
  modo: 'responder' | 'variavel';
  variavel?: string | null;
  proximo: string;
  /** Padrão: humano. */
  em_erro?: string | null;
}

export interface NoHumano extends NoBase {
  tipo: 'humano';
  mensagem?: string | null;
}

export interface NoFim extends NoBase {
  tipo: 'fim';
  mensagem?: string | null;
}

export type NoChatbot =
  | NoInicio
  | NoMensagem
  | NoMenu
  | NoPergunta
  | NoCondicao
  | NoAcao
  | NoIA
  | NoHumano
  | NoFim;

export type TipoNoChatbot = NoChatbot['tipo'];

export interface DefinicaoChatbot {
  versao: 1;
  /** Id do nó `inicio`. */
  inicio: string;
  /** 1–500. */
  nao_entendi: string;
  /** 1–10. */
  max_tentativas: number;
  /** 1–1.440. */
  inatividade_min: number;
  /** 2–200. */
  nos: NoChatbot[];
}

// ---------------------------------------------------------------------------
// Formatos — manifesto `automacao.json` (automação de IA)
// ---------------------------------------------------------------------------

export type Permissao =
  | 'enviar'
  | 'ler_conversas'
  | 'etiquetas'
  | 'funil'
  | 'leads'
  | 'ia'
  | 'rede'
  | 'agendar';

export const PERMISSOES: readonly Permissao[] = [
  'enviar',
  'ler_conversas',
  'etiquetas',
  'funil',
  'leads',
  'ia',
  'rede',
  'agendar',
] as const;

/**
 * Gatilho como escrito no manifesto: além dos ids, aceita nomes (`etiqueta`, `funil`, `etapa`),
 * resolvidos na compilação sem diferenciar maiúsculas/acentos.
 */
export type GatilhoManifesto =
  | Exclude<Gatilho, GatilhoEtiqueta | GatilhoEntrouEtapa>
  | {
      tipo: 'etiqueta';
      evento: 'adicionada' | 'removida';
      etiqueta_id?: Id;
      etiqueta?: string;
    }
  | {
      tipo: 'entrou_etapa';
      funil_id?: Id;
      funil?: string;
      etapa_id?: Id;
      etapa?: string;
    };

export interface LimiteAntiLoop {
  mensagens: number;
  janela_min: number;
}

export interface ManifestoAutomacao {
  $schema?: string;
  versao_manifesto: 1;
  /** 1–80 (espelhado em `Automacao.nome`). */
  nome: string;
  descricao?: string | null;
  /** Padrão "index.ts". */
  entrada?: string;
  gatilhos: GatilhoManifesto[];
  permissoes: Permissao[];
  /** Nomes de segredos que o código pode ler (≠ ANTHROPIC_API_KEY). */
  segredos?: string[];
  contas?: 'todas' | Id[];
  incluir_grupos?: boolean;
  prioridade?: number;
  conta_envio?: Id | null;
  limites?: {
    tempo_s?: number | null;
    memoria_mb?: number | null;
    anti_loop?: LimiteAntiLoop | null;
  };
  /** `modelo: null` = modelo padrão de Ajustes. */
  ia?: { modelo: string | null };
}

// ---------------------------------------------------------------------------
// Funis (contracts/api-http.md › Funis)
// ---------------------------------------------------------------------------

export interface Etapa {
  id: Id;
  funil_id: Id;
  nome: string;
  /** "#RRGGBB". */
  cor: string;
  ordem: number;
  total_cards: number;
}

export interface Funil {
  id: Id;
  nome: string;
  ordem: number;
  etapas: Etapa[];
  total_cards: number;
  criado_em: DataHora;
  atualizado_em: DataHora;
}

export interface Card {
  lead_id: Id;
  funil_id: Id;
  etapa_id: Id;
  desde: DataHora;
  lead: { id: Id; telefone: Telefone; nome: string | null; campos: Record<string, string> };
  etiquetas: Etiqueta[];
  conversa_id: Id | null;
  conta_id: Id | null;
}

export type OrigemMovimento = 'app' | 'mcp' | 'automacao';

export interface MovimentoFunil {
  id: Id;
  lead_id: Id;
  funil_id: Id;
  etapa_origem_id: Id | null;
  etapa_origem_nome: string | null;
  etapa_destino_id: Id | null;
  etapa_destino_nome: string | null;
  origem: OrigemMovimento;
  automacao_id: Id | null;
  execucao_id: Id | null;
  em: DataHora;
}

export interface NovaEtapa {
  /** 1–40. */
  nome: string;
  cor?: string;
  posicao?: number;
}

export interface NovoFunil {
  /** 1–60, único. */
  nome: string;
  etapas?: { nome: string; cor?: string }[];
}

export interface AlteracaoFunil {
  nome?: string;
  ordem?: number;
}

export interface AlteracaoEtapa {
  nome?: string;
  cor?: string;
}

/** Obrigatório se a etapa tiver cards: mover para outra etapa **ou** remover os cards. */
export type OpcoesExcluirEtapa =
  | { destino_etapa_id: Id; remover_cards?: never }
  | { remover_cards: true; destino_etapa_id?: never }
  | { destino_etapa_id?: never; remover_cards?: never };

/** Origem de quem mexe no funil pela API (a automação usa o serviço interno). */
export type OrigemCliente = 'app' | 'mcp';

/** Exatamente um alvo: `lead_id`, `contato_id` ou `telefone`. */
export type PedidoCard = { etapa_id: Id; origem?: OrigemCliente } & (
  | { lead_id: Id; contato_id?: never; telefone?: never }
  | { contato_id: Id; lead_id?: never; telefone?: never }
  | { telefone: string; lead_id?: never; contato_id?: never }
);

export interface FiltroCards extends ParametrosPagina {
  etapa_id?: Id;
  busca?: string;
}

export interface FiltroHistoricoFunil extends ParametrosPagina {
  lead_id?: Id;
}

export interface FunilDoLead {
  funil: { id: Id; nome: string };
  card: Card | null;
}

// ---------------------------------------------------------------------------
// Leads e disparos (acréscimos)
// ---------------------------------------------------------------------------

export interface AlteracaoLead {
  nome?: string | null;
  /** Merge; `null` remove o campo. */
  campos?: Record<string, string | null>;
}

export interface ResultadoAdicionarDestinatarios {
  adicionados: number;
  ja_existiam: number;
  disparo: Disparo;
}

// ---------------------------------------------------------------------------
// Automações (contracts/api-http.md › Automações)
// ---------------------------------------------------------------------------

export type TipoAutomacao = 'fluxo' | 'chatbot' | 'ia';

export interface Limites {
  anti_loop: LimiteAntiLoop | null;
  tempo_s: number | null;
  memoria_mb: number | null;
}

export interface ErroDefinicao {
  /** Caminho JSON do campo, ex.: "definicao.acoes[2].etapa_id". */
  caminho: string;
  no_id: string | null;
  acao_id: string | null;
  mensagem: string;
}

export type TipoErroCompilacao = 'sintaxe' | 'importacao' | 'manifesto' | 'resolucao' | 'outro';

export interface ErroCompilacao {
  arquivo: string;
  /** 1-base. */
  linha: number;
  /** 1-base (como o Monaco e a forma `arquivo:linha:coluna`). */
  coluna: number;
  mensagem: string;
  tipo: TipoErroCompilacao;
}

export type HandlerAutomacao = 'aoReceberMensagem' | 'aoAgendar' | 'aoExecutar' | 'aoEvento';

export interface ResultadoCompilacao {
  ok: boolean;
  erros: ErroCompilacao[];
  avisos: ErroCompilacao[];
  hash: string | null;
  handlers: HandlerAutomacao[];
  duracao_ms: number;
}

export interface AutomacaoIADados {
  pasta: string;
  permissoes: Permissao[];
  segredos: string[];
  hash_compilado: string | null;
  compilacao_ok: boolean;
  erros_compilacao: ErroCompilacao[];
  compilado_em: DataHora | null;
  rodando_versao_anterior: boolean;
}

export interface Estatisticas24h {
  ok: number;
  erro: number;
  abortada: number;
  duracao_media_ms: number | null;
}

export interface Automacao {
  id: Id;
  tipo: TipoAutomacao;
  nome: string;
  descricao: string | null;
  ativa: boolean;
  /** null = todas as contas. */
  contas: Id[] | null;
  incluir_grupos: boolean;
  /** 1–1.000; menor primeiro. */
  prioridade: number;
  conta_envio_id: Id | null;
  gatilhos: Gatilho[];
  /** null nas automações de IA (a definição é o projeto). */
  definicao: DefinicaoFluxo | DefinicaoChatbot | null;
  limites: Limites;
  versao: number;
  ia: AutomacaoIADados | null;
  /** Referências quebradas (não bloqueiam). */
  avisos: ErroDefinicao[];
  erros_seguidos: number;
  desativada_motivo: 'erros_seguidos' | 'usuario' | null;
  estatisticas_24h: Estatisticas24h;
  /** Chatbot; 0 nos demais. */
  sessoes_ativas: number;
  criada_em: DataHora;
  atualizada_em: DataHora;
}

interface NovaAutomacaoBase {
  /** 1–80. */
  nome: string;
  descricao?: string | null;
  contas?: Id[] | null;
  incluir_grupos?: boolean;
  prioridade?: number;
  conta_envio_id?: Id | null;
  gatilhos: Gatilho[];
  limites?: Partial<Limites>;
}

export type NovaAutomacao =
  | (NovaAutomacaoBase & { tipo: 'fluxo'; definicao: DefinicaoFluxo })
  | (NovaAutomacaoBase & { tipo: 'chatbot'; definicao: DefinicaoChatbot });

/** PATCH de fluxo/chatbot (automação de IA: edite `automacao.json`). */
export interface AlteracaoAutomacao {
  nome?: string;
  descricao?: string | null;
  contas?: Id[] | null;
  incluir_grupos?: boolean;
  prioridade?: number;
  conta_envio_id?: Id | null;
  gatilhos?: Gatilho[];
  definicao?: DefinicaoFluxo | DefinicaoChatbot;
  limites?: Partial<Limites>;
}

export type IdModeloProjeto = 'responder_historico' | 'classificar_funil' | 'extrair_dados' | 'em_branco';

export interface NovaAutomacaoIA {
  /** 1–80. */
  nome: string;
  modelo: IdModeloProjeto;
  descricao?: string | null;
}

export interface ResultadoValidacaoAutomacao {
  erros: ErroDefinicao[];
  avisos: ErroDefinicao[];
}

export interface FiltroAutomacoes {
  tipo?: TipoAutomacao;
  ativa?: boolean;
  busca?: string;
}

interface SemAlvo {
  conversa_id?: never;
  contato_id?: never;
  lead_id?: never;
  telefone?: never;
  conta_id?: never;
}

/** No máximo um alvo; nenhum (`{}`) = sem alvo. */
export type AlvoExecucao =
  | SemAlvo
  | (Omit<SemAlvo, 'conversa_id'> & { conversa_id: Id })
  | (Omit<SemAlvo, 'contato_id'> & { contato_id: Id })
  | (Omit<SemAlvo, 'lead_id'> & { lead_id: Id })
  | (Omit<SemAlvo, 'telefone' | 'conta_id'> & { telefone: string; conta_id: Id });

export type PedidoExecucao = AlvoExecucao & {
  entrada?: unknown;
  origem?: 'manual_app' | 'manual_mcp';
};

export interface PedidoTeste {
  /** aoReceberMensagem / gatilho de mensagem. Sem `conversa_id` usa a conversa fictícia. */
  mensagem?: { texto: string; conversa_id?: Id };
  /** Usa uma mensagem real recebida como gatilho. */
  mensagem_id?: Id;
  /** aoExecutar. */
  entrada?: unknown;
  /** aoEvento / gatilhos não-mensagem. */
  evento?: { tipo: string; dados: Record<string, unknown> };
  /** Conversa/lead de contexto. */
  alvo?: AlvoExecucao;
  ia_simulada?: boolean;
}

// ---------------------------------------------------------------------------
// Execuções
// ---------------------------------------------------------------------------

export type EstadoExecucao =
  | 'na_fila'
  | 'rodando'
  | 'aguardando'
  | 'ok'
  | 'erro'
  | 'simulacao'
  | 'abortada';

export type OrigemExecucao = 'gatilho' | 'manual_app' | 'manual_mcp' | 'teste' | 'fluxo' | 'chatbot';

export type ResultadoAcao = 'ok' | 'falhou' | 'bloqueada' | 'simulada';

export interface AcaoRegistrada {
  tipo: string;
  alvo: string | null;
  resultado: ResultadoAcao;
  detalhe: string | null;
  em: DataHora;
}

export interface TokensModelo {
  entrada: number;
  saida: number;
  chamadas: number;
}

export interface Tokens {
  entrada: number;
  saida: number;
  por_modelo: Record<string, TokensModelo>;
}

export interface Execucao {
  id: Id;
  automacao_id: Id;
  automacao_nome: string;
  tipo_automacao: TipoAutomacao;
  automacao_versao: number;
  gatilho: { tipo: TipoGatilho | string; dados: Record<string, unknown> };
  origem: OrigemExecucao;
  origem_execucao_id: Id | null;
  conta_id: Id | null;
  conversa_id: Id | null;
  contato_id: Id | null;
  lead_id: Id | null;
  estado: EstadoExecucao;
  simulacao: boolean;
  motivo: string | null;
  erro: string | null;
  acoes: AcaoRegistrada[];
  tokens: Tokens;
  retorno: unknown;
  retomar_em: DataHora | null;
  iniciada_em: DataHora;
  finalizada_em: DataHora | null;
  duracao_ms: number | null;
}

export interface ExecucaoDetalhe extends Execucao {
  log: string;
  log_truncado: boolean;
  erro_stack: string | null;
  variaveis: Record<string, unknown>;
}

export interface ResultadoTeste {
  /** Estado "simulacao" ou "erro". */
  execucao: ExecucaoDetalhe;
}

export interface FiltroExecucoes extends ParametrosPagina {
  estado?: EstadoExecucao;
  conversa_id?: Id;
}

// ---------------------------------------------------------------------------
// Chatbot — sessões e simulador
// ---------------------------------------------------------------------------

export type EstadoSessaoChatbot = 'ativa' | 'concluida' | 'humano' | 'expirada' | 'abortada';

export interface SessaoChatbot {
  id: Id;
  automacao_id: Id;
  automacao_nome: string;
  conversa_id: Id;
  versao: number;
  no_atual: string;
  variaveis: Record<string, string>;
  tentativas: number;
  estado: EstadoSessaoChatbot;
  motivo: string | null;
  expira_em: DataHora;
  iniciada_em: DataHora;
  atualizada_em: DataHora;
  finalizada_em: DataHora | null;
}

export interface FiltroSessoes extends ParametrosPagina {
  estado?: EstadoSessaoChatbot;
}

export interface SaidaSimulada {
  tipo: 'mensagem' | 'acao' | 'aviso';
  texto: string;
  no_id: string | null;
}

export interface PedidoSimulador {
  /** Sem `definicao` usa a gravada. */
  definicao?: DefinicaoChatbot;
  alvo?: AlvoExecucao;
  ia_simulada?: boolean;
}

export interface EstadoSimulacao {
  saidas: SaidaSimulada[];
  no_atual: string | null;
  variaveis: Record<string, string>;
  estado: EstadoSessaoChatbot;
}

export interface InicioSimulacao extends EstadoSimulacao {
  simulacao_id: Id;
}

export interface RodadaSimulacao extends EstadoSimulacao {
  acoes: AcaoRegistrada[];
}

// ---------------------------------------------------------------------------
// Automações de IA — projeto e compilação
// ---------------------------------------------------------------------------

export interface ArquivoProjeto {
  /** Relativo à pasta do projeto, com "/". */
  caminho: string;
  tamanho: number;
  hash: string;
  atualizado_em: DataHora;
}

export interface ConteudoArquivo {
  caminho: string;
  conteudo: string;
  hash: string;
  atualizado_em: DataHora;
}

export interface ModeloProjeto {
  id: IdModeloProjeto;
  nome: string;
  descricao: string;
}

export interface SdkAutomacao {
  versao: string;
  /** Conteúdo do .d.ts do SDK. */
  tipos: string;
  /** JSON Schema do manifesto. */
  esquema_manifesto: Record<string, unknown>;
}

// ---------------------------------------------------------------------------
// Conversas — estado das automações e pausas
// ---------------------------------------------------------------------------

export type MotivoPausa = 'humano' | 'anti_loop' | 'manual';

export interface Pausa {
  conversa_id: Id;
  motivo: MotivoPausa;
  /** null = sem prazo. */
  ate: DataHora | null;
  automacao_id: Id | null;
  criada_em: DataHora;
}

export interface EstadoConversaAutomacoes {
  conversa_id: Id;
  pausa: Pausa | null;
  sessao: SessaoChatbot | null;
  pausa_geral: boolean;
}

export interface PedidoPausa {
  motivo: 'humano' | 'manual';
  /** null/ausente = sem prazo. */
  duracao_min?: number | null;
}

// ---------------------------------------------------------------------------
// Configuração, IA e segredos
// ---------------------------------------------------------------------------

/**
 * Padrões (data-model.md › Configuração): anti-loop 10 mensagens a cada 10 min (decisão do
 * orquestrador; era 5), pausa anti-loop 60 min, pausa humana 30 min, 20 primeiros contatos/h,
 * IA 60 s / 256 MB / 4 processos / 5 min de ociosidade, pausa geral desligada.
 */
export interface ConfiguracaoAutomacoes {
  /** 1–100 (padrão 10). */
  anti_loop_mensagens: number;
  /** 1–1.440 (padrão 10). */
  anti_loop_janela_min: number;
  /** 1–10.080 (padrão 60). */
  pausa_anti_loop_min: number;
  /** 1–10.080 (padrão 30). */
  pausa_humana_min: number;
  /** 0–500 (padrão 20; 0 bloqueia). */
  primeiros_contatos_hora: number;
  /** 5–300 (padrão 60). */
  tempo_ia_s: number;
  /** 64–2.048 (padrão 256). */
  memoria_ia_mb: number;
  /** 1–16 (padrão 4). */
  processos_ia_max: number;
  /** 1–60 (padrão 5). */
  ociosidade_ia_min: number;
  /** "Pausar todas as automações". */
  pausa_geral: boolean;
}

export interface ModeloIA {
  id: string;
  nome: string;
  aviso: string | null;
}

export interface ConfiguracaoIA {
  /** Começa com "claude-" (padrão "claude-sonnet-5"). */
  modelo_padrao: string;
  modelos: ModeloIA[];
  chave_configurada: boolean;
}

export interface ResultadoTesteChave {
  ok: true;
  modelo: string;
  latencia_ms: number;
}

export interface Segredo {
  nome: string;
  reservado: boolean;
  usado_por: { automacao_id: Id; nome: string }[];
}

// ---------------------------------------------------------------------------
// Modo falso — acréscimos da 002 (contracts/runtime.md › Modo falso)
// ---------------------------------------------------------------------------

export interface FalsoIA {
  /** Primeira cujo `contem` aparecer no prompt; senão a resposta padrão. */
  respostas: { contem: string; texto: string }[];
  /** Erro injetado nas próximas `vezes` chamadas (testa retentativa). */
  erro?: { status: number; vezes: number } | null;
}

export interface FalsoChamadaIA {
  modelo: string;
  sistema: string | null;
  /** Texto das mensagens enviadas, resumido. */
  mensagens_resumo: string;
  esquema?: Record<string, unknown>;
  em: DataHora;
}

// ---------------------------------------------------------------------------
// Runtime — contracts/runtime.md
// ---------------------------------------------------------------------------

export interface Runtime {
  versao_contrato: 1;
  porta: number;
  token: string;
  pid_app: number;
  pid_motor: number;
  versao: string;
  iniciado_em: DataHora;
}

/** Linhas JSON de controle que o motor escreve no stdout. */
export type LinhaControleMotor =
  | { evento: 'pronto'; porta: number; versao: string; pid: number; whatsapp: ModoWhatsApp }
  | {
      evento: 'erro_fatal';
      codigo:
        | 'token_ausente'
        | 'pasta_dados_inacessivel'
        | 'instancia_duplicada'
        | 'migracao_falhou'
        | 'porta_ocupada';
      mensagem: string;
    }
  | { evento: 'encerrando'; motivo: 'sinal' | 'stdin_fechado' | 'pedido' };

/** 002: linhas JSON de controle que o app escreve no stdin do motor (runtime.md › Canal de controle). */
export type LinhaControleApp = {
  comando: 'segredos';
  /** Substitui o conjunto inteiro. Nomes `[A-Z][A-Z0-9_]{0,63}`, valores 1–4.096. */
  valores: Record<string, string>;
};
