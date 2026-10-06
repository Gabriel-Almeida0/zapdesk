// Cliente HTTP do motor do ZapDesk: um método por rota de contracts/api-http.md
// (e /v1/falso/* de contracts/runtime.md), das features 001 e 002. Funciona no Node 22 e no navegador (fetch nativo).
import { ErroMotor, MENSAGEM_MOTOR_FORA, parsearErro } from './erros.js';
import type {
  AlteracaoAutomacao,
  AlteracaoDisparo,
  AlteracaoEtapa,
  AlteracaoFunil,
  AlteracaoLead,
  AlteracaoTemplate,
  Arquivo,
  ArquivoProjeto,
  Automacao,
  Card,
  ConfiguracaoAutomacoes,
  ConfiguracaoIA,
  Conta,
  Contato,
  ConteudoArquivo,
  Conversa,
  CorpoImportacao,
  Destinatario,
  Disparo,
  EnvioMensagem,
  EstadoConversaAutomacoes,
  Etapa,
  Etiqueta,
  EventoEnergia,
  Execucao,
  ExecucaoDetalhe,
  FalsoChamadaIA,
  FalsoEnviada,
  FalsoEscanearQr,
  FalsoEventoEstado,
  FalsoFalhasEnvio,
  FalsoHistorico,
  FalsoIA,
  FalsoInjetada,
  FalsoMensagemRecebida,
  FalsoRecibo,
  FalsoRelogio,
  FalsoStatus,
  Figurinha,
  FiltroAutomacoes,
  FiltroCards,
  FiltroContatos,
  FiltroConversas,
  FiltroDestinatarios,
  FiltroDisparos,
  FiltroExecucoes,
  FiltroHistoricoFunil,
  FiltroLeads,
  FiltroSessoes,
  Funil,
  FunilDoLead,
  Id,
  ImportacaoContatos,
  InicioSimulacao,
  Lead,
  Mensagem,
  ModeloProjeto,
  MotivoPausa,
  MovimentoFunil,
  NovaAutomacao,
  NovaAutomacaoIA,
  NovaEtapa,
  NovaEtiqueta,
  NovoDisparo,
  NovoFunil,
  NovoTemplate,
  OpcoesExcluirEtapa,
  OrigemCliente,
  Pagina,
  ParametrosMensagens,
  ParametrosPagina,
  Pausa,
  PedidoCard,
  PedidoExecucao,
  PedidoPausa,
  PedidoSimulador,
  PedidoTeste,
  PreviaImportacao,
  QrConta,
  RelatorioImportacao,
  ResultadoAdicionarDestinatarios,
  ResultadoBuscaMensagem,
  ResultadoCompilacao,
  ResultadoTeste,
  ResultadoTesteChave,
  ResultadoValidacaoAutomacao,
  RodadaSimulacao,
  Saude,
  SdkAutomacao,
  Segredo,
  SessaoChatbot,
  Sistema,
  StatusPorContato,
  Template,
  ValidacaoDisparo,
} from './tipos.js';

export type Fetch = typeof globalThis.fetch;

export interface OpcoesClienteMotor {
  /** Porta do motor em 127.0.0.1. */
  porta: number;
  token: string;
  /** Host; padrão "127.0.0.1" (o motor rejeita qualquer outro Host). */
  host?: string;
  /** fetch injetável (testes). Padrão: `globalThis.fetch`. */
  fetch?: Fetch;
}

/** Arquivo para upload multipart: `Blob`/`File` (navegador ou Node 22) + nome. */
export interface ArquivoUpload {
  dados: Blob;
  nome: string;
}

type Metodo = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
type ValorQuery = string | number | boolean | null | undefined;

interface Requisicao {
  metodo: Metodo;
  caminho: string;
  query?: Record<string, ValorQuery> | object;
  json?: unknown;
  formulario?: FormData;
  /** Resposta esperada: JSON (padrão), texto ou nenhuma (204/202 sem corpo). */
  resposta?: 'json' | 'texto' | 'nada';
}

const seg = encodeURIComponent;

export class ClienteMotor {
  readonly porta: number;
  readonly token: string;
  /** Ex.: "http://127.0.0.1:51234". */
  readonly base: string;
  private readonly fetch: Fetch;

  constructor(opcoes: OpcoesClienteMotor) {
    this.porta = opcoes.porta;
    this.token = opcoes.token;
    this.base = `http://${opcoes.host ?? '127.0.0.1'}:${opcoes.porta}`;
    this.fetch = opcoes.fetch ?? globalThis.fetch.bind(globalThis);
  }

  // -------------------------------------------------------------------------
  // Utilidades
  // -------------------------------------------------------------------------

  /**
   * URL absoluta com `?token=` para rotas GET binárias (`<img>`, `<audio>`, download) e para o
   * relatório CSV. Aceita o caminho como vem da API (ex.: "/v1/mensagens/{id}/midia").
   */
  urlBinaria(caminho: string): string {
    const url = new URL(this.normalizarCaminho(caminho), this.base);
    url.searchParams.set('token', this.token);
    return url.toString();
  }

  /** URL do WebSocket de eventos, com `?token=`. */
  urlEventos(): string {
    const url = new URL('/v1/eventos', this.base);
    url.protocol = 'ws:';
    url.searchParams.set('token', this.token);
    return url.toString();
  }

  private normalizarCaminho(caminho: string): string {
    const comBarra = caminho.startsWith('/') ? caminho : `/${caminho}`;
    return comBarra.startsWith('/v1/') || comBarra === '/v1' ? comBarra : `/v1${comBarra}`;
  }

  private montarUrl(caminho: string, query?: Requisicao['query']): string {
    const url = new URL(this.normalizarCaminho(caminho), this.base);
    if (query) {
      for (const [chave, valor] of Object.entries(query as Record<string, ValorQuery>)) {
        if (valor === undefined || valor === null || valor === '') continue;
        url.searchParams.set(chave, String(valor));
      }
    }
    return url.toString();
  }

  private async requisitar<T>(req: Requisicao): Promise<T> {
    const cabecalhos: Record<string, string> = { Authorization: `Bearer ${this.token}` };
    let corpo: BodyInit | undefined;
    if (req.formulario) {
      corpo = req.formulario; // o fetch define o boundary do multipart
    } else if (req.json !== undefined) {
      cabecalhos['Content-Type'] = 'application/json';
      corpo = JSON.stringify(req.json);
    }

    let resposta: Response;
    try {
      resposta = await this.fetch(this.montarUrl(req.caminho, req.query), {
        method: req.metodo,
        headers: cabecalhos,
        ...(corpo !== undefined ? { body: corpo } : {}),
      });
    } catch (causa) {
      const erro = new ErroMotor('interno', MENSAGEM_MOTOR_FORA, 0, {
        motivo: causa instanceof Error ? causa.message : String(causa),
      });
      throw erro;
    }

    if (!resposta.ok) {
      throw parsearErro(resposta.status, await resposta.text().catch(() => ''));
    }

    const tipo = req.resposta ?? 'json';
    if (tipo === 'nada' || resposta.status === 204) {
      await resposta.body?.cancel().catch(() => undefined);
      return undefined as T;
    }
    if (tipo === 'texto') return (await resposta.text()) as T;
    const texto = await resposta.text();
    return (texto.length > 0 ? JSON.parse(texto) : undefined) as T;
  }

  private formularioArquivo(arquivo: ArquivoUpload): FormData {
    const formulario = new FormData();
    formulario.append('arquivo', arquivo.dados, arquivo.nome);
    return formulario;
  }

  // -------------------------------------------------------------------------
  // Sistema
  // -------------------------------------------------------------------------

  saude(): Promise<Saude> {
    return this.requisitar({ metodo: 'GET', caminho: '/saude' });
  }

  sistema(): Promise<Sistema> {
    return this.requisitar({ metodo: 'GET', caminho: '/sistema' });
  }

  energia(evento: EventoEnergia): Promise<void> {
    return this.requisitar({ metodo: 'POST', caminho: '/sistema/energia', json: { evento }, resposta: 'nada' });
  }

  encerrar(): Promise<void> {
    return this.requisitar({ metodo: 'POST', caminho: '/sistema/encerrar', resposta: 'nada' });
  }

  // -------------------------------------------------------------------------
  // Contas
  // -------------------------------------------------------------------------

  listarContas(): Promise<Conta[]> {
    return this.requisitar({ metodo: 'GET', caminho: '/contas' });
  }

  criarConta(dados: { nome?: string } = {}): Promise<Conta> {
    return this.requisitar({ metodo: 'POST', caminho: '/contas', json: dados });
  }

  obterConta(id: Id): Promise<Conta> {
    return this.requisitar({ metodo: 'GET', caminho: `/contas/${seg(id)}` });
  }

  /** `404 nao_encontrado` se não há QR ativo. */
  obterQr(contaId: Id): Promise<QrConta> {
    return this.requisitar({ metodo: 'GET', caminho: `/contas/${seg(contaId)}/qr` });
  }

  reconectarConta(id: Id): Promise<Conta> {
    return this.requisitar({ metodo: 'POST', caminho: `/contas/${seg(id)}/reconectar` });
  }

  renomearConta(id: Id, nome: string): Promise<Conta> {
    return this.requisitar({ metodo: 'PATCH', caminho: `/contas/${seg(id)}`, json: { nome } });
  }

  removerConta(id: Id): Promise<void> {
    return this.requisitar({ metodo: 'DELETE', caminho: `/contas/${seg(id)}`, resposta: 'nada' });
  }

  // -------------------------------------------------------------------------
  // Conversas e mensagens
  // -------------------------------------------------------------------------

  listarConversas(contaId: Id, filtro: FiltroConversas = {}): Promise<Pagina<Conversa>> {
    return this.requisitar({ metodo: 'GET', caminho: `/contas/${seg(contaId)}/conversas`, query: filtro });
  }

  /** Abre (ou cria) conversa com um telefone em qualquer formato. */
  abrirConversa(contaId: Id, telefone: string): Promise<Conversa> {
    return this.requisitar({
      metodo: 'POST',
      caminho: `/contas/${seg(contaId)}/conversas`,
      json: { telefone },
    });
  }

  obterConversa(id: Id): Promise<Conversa> {
    return this.requisitar({ metodo: 'GET', caminho: `/conversas/${seg(id)}` });
  }

  marcarLida(conversaId: Id): Promise<void> {
    return this.requisitar({ metodo: 'POST', caminho: `/conversas/${seg(conversaId)}/lida`, resposta: 'nada' });
  }

  /** Mais recentes primeiro. */
  listarMensagens(conversaId: Id, parametros: ParametrosMensagens = {}): Promise<Pagina<Mensagem>> {
    return this.requisitar({
      metodo: 'GET',
      caminho: `/conversas/${seg(conversaId)}/mensagens`,
      query: parametros,
    });
  }

  buscarMensagens(
    contaId: Id,
    q: string,
    pagina: ParametrosPagina = {},
  ): Promise<Pagina<ResultadoBuscaMensagem>> {
    return this.requisitar({
      metodo: 'GET',
      caminho: `/contas/${seg(contaId)}/mensagens/busca`,
      query: { q, ...pagina },
    });
  }

  enviarMensagem(conversaId: Id, envio: EnvioMensagem): Promise<Mensagem> {
    return this.requisitar({ metodo: 'POST', caminho: `/conversas/${seg(conversaId)}/mensagens`, json: envio });
  }

  reenviarMensagem(id: Id): Promise<Mensagem> {
    return this.requisitar({ metodo: 'POST', caminho: `/mensagens/${seg(id)}/reenviar` });
  }

  /** `emoji = ""` remove a reação. */
  reagir(mensagemId: Id, emoji: string): Promise<void> {
    return this.requisitar({
      metodo: 'POST',
      caminho: `/mensagens/${seg(mensagemId)}/reacao`,
      json: { emoji },
      resposta: 'nada',
    });
  }

  editarMensagem(id: Id, texto: string): Promise<Mensagem> {
    return this.requisitar({ metodo: 'PATCH', caminho: `/mensagens/${seg(id)}`, json: { texto } });
  }

  /** Apagar para todos. */
  apagarMensagem(id: Id): Promise<void> {
    return this.requisitar({ metodo: 'DELETE', caminho: `/mensagens/${seg(id)}`, resposta: 'nada' });
  }

  /** URL (com token) da mídia da mensagem. */
  urlMidiaMensagem(id: Id): string {
    return this.urlBinaria(`/v1/mensagens/${seg(id)}/midia`);
  }

  /** Baixa a mídia da mensagem (Node/MCP). */
  async baixarMidiaMensagem(id: Id): Promise<Blob> {
    return this.baixarBinario(`/v1/mensagens/${seg(id)}/midia`);
  }

  listarFigurinhas(contaId: Id, limite?: number): Promise<Figurinha[]> {
    return this.requisitar({ metodo: 'GET', caminho: `/contas/${seg(contaId)}/figurinhas`, query: { limite } });
  }

  // -------------------------------------------------------------------------
  // Contatos
  // -------------------------------------------------------------------------

  listarContatos(contaId: Id, filtro: FiltroContatos = {}): Promise<Pagina<Contato>> {
    return this.requisitar({ metodo: 'GET', caminho: `/contas/${seg(contaId)}/contatos`, query: filtro });
  }

  obterContato(id: Id): Promise<Contato> {
    return this.requisitar({ metodo: 'GET', caminho: `/contatos/${seg(id)}` });
  }

  salvarNotas(contatoId: Id, notas: string): Promise<Contato> {
    return this.requisitar({ metodo: 'PATCH', caminho: `/contatos/${seg(contatoId)}`, json: { notas } });
  }

  /** Substitui o conjunto de etiquetas do contato. */
  definirEtiquetasContato(contatoId: Id, etiquetaIds: Id[]): Promise<Contato> {
    return this.requisitar({
      metodo: 'PUT',
      caminho: `/contatos/${seg(contatoId)}/etiquetas`,
      json: { etiqueta_ids: etiquetaIds },
    });
  }

  // -------------------------------------------------------------------------
  // Status
  // -------------------------------------------------------------------------

  listarStatus(contaId: Id): Promise<StatusPorContato[]> {
    return this.requisitar({ metodo: 'GET', caminho: `/contas/${seg(contaId)}/status` });
  }

  urlMidiaStatus(id: Id): string {
    return this.urlBinaria(`/v1/status/${seg(id)}/midia`);
  }

  // -------------------------------------------------------------------------
  // Etiquetas
  // -------------------------------------------------------------------------

  listarEtiquetas(): Promise<Etiqueta[]> {
    return this.requisitar({ metodo: 'GET', caminho: '/etiquetas' });
  }

  criarEtiqueta(dados: NovaEtiqueta): Promise<Etiqueta> {
    return this.requisitar({ metodo: 'POST', caminho: '/etiquetas', json: dados });
  }

  editarEtiqueta(id: Id, dados: Partial<NovaEtiqueta>): Promise<Etiqueta> {
    return this.requisitar({ metodo: 'PATCH', caminho: `/etiquetas/${seg(id)}`, json: dados });
  }

  excluirEtiqueta(id: Id): Promise<void> {
    return this.requisitar({ metodo: 'DELETE', caminho: `/etiquetas/${seg(id)}`, resposta: 'nada' });
  }

  // -------------------------------------------------------------------------
  // Arquivos (anexos)
  // -------------------------------------------------------------------------

  /** Upload multipart (campo `arquivo`). */
  enviarArquivo(arquivo: ArquivoUpload): Promise<Arquivo> {
    return this.requisitar({ metodo: 'POST', caminho: '/arquivos', formulario: this.formularioArquivo(arquivo) });
  }

  /** O motor copia um arquivo local pelo caminho absoluto (usado pelo MCP). */
  enviarArquivoPorCaminho(caminho: string): Promise<Arquivo> {
    return this.requisitar({ metodo: 'POST', caminho: '/arquivos', json: { caminho } });
  }

  obterArquivo(id: Id): Promise<Arquivo> {
    return this.requisitar({ metodo: 'GET', caminho: `/arquivos/${seg(id)}` });
  }

  urlConteudoArquivo(id: Id): string {
    return this.urlBinaria(`/v1/arquivos/${seg(id)}/conteudo`);
  }

  // -------------------------------------------------------------------------
  // Templates
  // -------------------------------------------------------------------------

  listarTemplates(busca?: string): Promise<Template[]> {
    return this.requisitar({ metodo: 'GET', caminho: '/templates', query: { busca } });
  }

  criarTemplate(dados: NovoTemplate): Promise<Template> {
    return this.requisitar({ metodo: 'POST', caminho: '/templates', json: dados });
  }

  obterTemplate(id: Id): Promise<Template> {
    return this.requisitar({ metodo: 'GET', caminho: `/templates/${seg(id)}` });
  }

  editarTemplate(id: Id, dados: AlteracaoTemplate): Promise<Template> {
    return this.requisitar({ metodo: 'PATCH', caminho: `/templates/${seg(id)}`, json: dados });
  }

  excluirTemplate(id: Id): Promise<void> {
    return this.requisitar({ metodo: 'DELETE', caminho: `/templates/${seg(id)}`, resposta: 'nada' });
  }

  // -------------------------------------------------------------------------
  // Leads e importação
  // -------------------------------------------------------------------------

  /** Prévia de planilha `.csv`/`.xlsx` enviada por multipart. */
  previaImportacao(arquivo: ArquivoUpload): Promise<PreviaImportacao> {
    return this.requisitar({
      metodo: 'POST',
      caminho: '/importacoes/previa',
      formulario: this.formularioArquivo(arquivo),
    });
  }

  /** Prévia de planilha lida pelo motor a partir de um caminho absoluto (MCP). */
  previaImportacaoPorCaminho(caminho: string): Promise<PreviaImportacao> {
    return this.requisitar({ metodo: 'POST', caminho: '/importacoes/previa', json: { caminho } });
  }

  importarLeads(corpo: CorpoImportacao): Promise<RelatorioImportacao> {
    return this.requisitar({ metodo: 'POST', caminho: '/leads/importar', json: corpo });
  }

  importarContatosComoLeads(dados: ImportacaoContatos): Promise<RelatorioImportacao> {
    return this.requisitar({ metodo: 'POST', caminho: '/leads/importar-contatos', json: dados });
  }

  listarLeads(filtro: FiltroLeads = {}): Promise<Pagina<Lead>> {
    return this.requisitar({ metodo: 'GET', caminho: '/leads', query: filtro });
  }

  obterLead(id: Id): Promise<Lead> {
    return this.requisitar({ metodo: 'GET', caminho: `/leads/${seg(id)}` });
  }

  // -------------------------------------------------------------------------
  // Disparos
  // -------------------------------------------------------------------------

  validarDisparo(dados: NovoDisparo): Promise<ValidacaoDisparo> {
    return this.requisitar({ metodo: 'POST', caminho: '/disparos/validar', json: dados });
  }

  criarDisparo(dados: NovoDisparo): Promise<Disparo> {
    return this.requisitar({ metodo: 'POST', caminho: '/disparos', json: dados });
  }

  listarDisparos(filtro: FiltroDisparos = {}): Promise<Pagina<Disparo>> {
    return this.requisitar({ metodo: 'GET', caminho: '/disparos', query: filtro });
  }

  obterDisparo(id: Id): Promise<Disparo> {
    return this.requisitar({ metodo: 'GET', caminho: `/disparos/${seg(id)}` });
  }

  /** Só em `rascunho`. */
  editarDisparo(id: Id, dados: AlteracaoDisparo): Promise<Disparo> {
    return this.requisitar({ metodo: 'PATCH', caminho: `/disparos/${seg(id)}`, json: dados });
  }

  /** Só em `rascunho`. */
  excluirDisparo(id: Id): Promise<void> {
    return this.requisitar({ metodo: 'DELETE', caminho: `/disparos/${seg(id)}`, resposta: 'nada' });
  }

  iniciarDisparo(id: Id, valoresPadrao?: Record<string, string>): Promise<Disparo> {
    return this.requisitar({
      metodo: 'POST',
      caminho: `/disparos/${seg(id)}/iniciar`,
      json: valoresPadrao ? { valores_padrao: valoresPadrao } : {},
    });
  }

  pausarDisparo(id: Id): Promise<Disparo> {
    return this.requisitar({ metodo: 'POST', caminho: `/disparos/${seg(id)}/pausar` });
  }

  retomarDisparo(id: Id): Promise<Disparo> {
    return this.requisitar({ metodo: 'POST', caminho: `/disparos/${seg(id)}/retomar` });
  }

  cancelarDisparo(id: Id): Promise<Disparo> {
    return this.requisitar({ metodo: 'POST', caminho: `/disparos/${seg(id)}/cancelar` });
  }

  listarDestinatarios(disparoId: Id, filtro: FiltroDestinatarios = {}): Promise<Pagina<Destinatario>> {
    return this.requisitar({
      metodo: 'GET',
      caminho: `/disparos/${seg(disparoId)}/destinatarios`,
      query: filtro,
    });
  }

  /** URL (com token) do relatório CSV — para download direto pelo navegador. */
  urlRelatorioCsv(disparoId: Id): string {
    return this.urlBinaria(`/v1/disparos/${seg(disparoId)}/relatorio.csv`);
  }

  /** Conteúdo do relatório CSV (UTF-8 com BOM). */
  relatorioCsv(disparoId: Id): Promise<string> {
    return this.requisitar({
      metodo: 'GET',
      caminho: `/disparos/${seg(disparoId)}/relatorio.csv`,
      resposta: 'texto',
    });
  }

  // -------------------------------------------------------------------------
  // 002 — Funis (specs/002-automacoes/contracts/api-http.md › Funis)
  // -------------------------------------------------------------------------

  listarFunis(): Promise<Funil[]> {
    return this.requisitar({ metodo: 'GET', caminho: '/funis' });
  }

  criarFunil(dados: NovoFunil): Promise<Funil> {
    return this.requisitar({ metodo: 'POST', caminho: '/funis', json: dados });
  }

  obterFunil(id: Id): Promise<Funil> {
    return this.requisitar({ metodo: 'GET', caminho: `/funis/${seg(id)}` });
  }

  editarFunil(id: Id, dados: AlteracaoFunil): Promise<Funil> {
    return this.requisitar({ metodo: 'PATCH', caminho: `/funis/${seg(id)}`, json: dados });
  }

  /** Apaga etapas, cards e histórico do funil. */
  excluirFunil(id: Id): Promise<void> {
    return this.requisitar({ metodo: 'DELETE', caminho: `/funis/${seg(id)}`, resposta: 'nada' });
  }

  criarEtapa(funilId: Id, dados: NovaEtapa): Promise<Etapa> {
    return this.requisitar({ metodo: 'POST', caminho: `/funis/${seg(funilId)}/etapas`, json: dados });
  }

  /** `etapaIds` = todas as etapas do funil, na ordem desejada. */
  reordenarEtapas(funilId: Id, etapaIds: Id[]): Promise<Funil> {
    return this.requisitar({
      metodo: 'PUT',
      caminho: `/funis/${seg(funilId)}/etapas/ordem`,
      json: { etapa_ids: etapaIds },
    });
  }

  editarEtapa(id: Id, dados: AlteracaoEtapa): Promise<Etapa> {
    return this.requisitar({ metodo: 'PATCH', caminho: `/etapas/${seg(id)}`, json: dados });
  }

  /** Com cards na etapa, informe `destino_etapa_id` **ou** `remover_cards: true`. */
  excluirEtapa(id: Id, opcoes: OpcoesExcluirEtapa = {}): Promise<void> {
    return this.requisitar({ metodo: 'DELETE', caminho: `/etapas/${seg(id)}`, query: opcoes, resposta: 'nada' });
  }

  /** Mais recentes (`desde`) primeiro. */
  listarCards(funilId: Id, filtro: FiltroCards = {}): Promise<Pagina<Card>> {
    return this.requisitar({ metodo: 'GET', caminho: `/funis/${seg(funilId)}/cards`, query: filtro });
  }

  /** Coloca o lead no funil ou o move de etapa (200 = moveu, 201 = entrou). */
  moverCard(funilId: Id, pedido: PedidoCard): Promise<Card> {
    return this.requisitar({ metodo: 'PUT', caminho: `/funis/${seg(funilId)}/cards`, json: pedido });
  }

  /** Idempotente. */
  removerCard(funilId: Id, leadId: Id, origem?: OrigemCliente): Promise<void> {
    return this.requisitar({
      metodo: 'DELETE',
      caminho: `/funis/${seg(funilId)}/cards/${seg(leadId)}`,
      query: { origem },
      resposta: 'nada',
    });
  }

  /** Mais recente primeiro. */
  historicoFunil(funilId: Id, filtro: FiltroHistoricoFunil = {}): Promise<Pagina<MovimentoFunil>> {
    return this.requisitar({ metodo: 'GET', caminho: `/funis/${seg(funilId)}/historico`, query: filtro });
  }

  funisDoLead(leadId: Id): Promise<FunilDoLead[]> {
    return this.requisitar({ metodo: 'GET', caminho: `/leads/${seg(leadId)}/funis` });
  }

  // -------------------------------------------------------------------------
  // 002 — Leads e disparos (acréscimos)
  // -------------------------------------------------------------------------

  /** Merge de `campos` (`null` remove o campo). */
  editarLead(id: Id, dados: AlteracaoLead): Promise<Lead> {
    return this.requisitar({ metodo: 'PATCH', caminho: `/leads/${seg(id)}`, json: dados });
  }

  /** 1–10.000 leads. */
  adicionarDestinatarios(disparoId: Id, leadIds: Id[]): Promise<ResultadoAdicionarDestinatarios> {
    return this.requisitar({
      metodo: 'POST',
      caminho: `/disparos/${seg(disparoId)}/destinatarios`,
      json: { lead_ids: leadIds },
    });
  }

  // -------------------------------------------------------------------------
  // 002 — Automações (todas)
  // -------------------------------------------------------------------------

  /** Ordem: `prioridade`, `criada_em`. */
  listarAutomacoes(filtro: FiltroAutomacoes = {}): Promise<Automacao[]> {
    return this.requisitar({ metodo: 'GET', caminho: '/automacoes', query: filtro });
  }

  /** Fluxo ou chatbot; nasce inativa. */
  criarAutomacao(dados: NovaAutomacao): Promise<Automacao> {
    return this.requisitar({ metodo: 'POST', caminho: '/automacoes', json: dados });
  }

  /** Cria a pasta com os arquivos do modelo e compila. */
  criarAutomacaoIA(dados: NovaAutomacaoIA): Promise<Automacao> {
    return this.requisitar({ metodo: 'POST', caminho: '/automacoes/ia', json: dados });
  }

  /** Valida sem gravar. */
  validarAutomacao(dados: NovaAutomacao): Promise<ResultadoValidacaoAutomacao> {
    return this.requisitar({ metodo: 'POST', caminho: '/automacoes/validar', json: dados });
  }

  obterAutomacao(id: Id): Promise<Automacao> {
    return this.requisitar({ metodo: 'GET', caminho: `/automacoes/${seg(id)}` });
  }

  editarAutomacao(id: Id, dados: AlteracaoAutomacao): Promise<Automacao> {
    return this.requisitar({ metodo: 'PATCH', caminho: `/automacoes/${seg(id)}`, json: dados });
  }

  excluirAutomacao(id: Id): Promise<void> {
    return this.requisitar({ metodo: 'DELETE', caminho: `/automacoes/${seg(id)}`, resposta: 'nada' });
  }

  ativarAutomacao(id: Id): Promise<Automacao> {
    return this.requisitar({ metodo: 'POST', caminho: `/automacoes/${seg(id)}/ativar` });
  }

  desativarAutomacao(id: Id): Promise<Automacao> {
    return this.requisitar({ metodo: 'POST', caminho: `/automacoes/${seg(id)}/desativar` });
  }

  /** `202` com a execução `na_fila`. */
  executarAutomacao(id: Id, pedido: PedidoExecucao = {}): Promise<Execucao> {
    return this.requisitar({ metodo: 'POST', caminho: `/automacoes/${seg(id)}/executar`, json: pedido });
  }

  /** Simulação (nada é enviado); aguarda até `tempo_s` + 5 s. Fluxo e IA. */
  testarAutomacao(id: Id, pedido: PedidoTeste): Promise<ResultadoTeste> {
    return this.requisitar({ metodo: 'POST', caminho: `/automacoes/${seg(id)}/testar`, json: pedido });
  }

  /** Mais recente primeiro. */
  listarExecucoesAutomacao(id: Id, filtro: FiltroExecucoes = {}): Promise<Pagina<Execucao>> {
    return this.requisitar({ metodo: 'GET', caminho: `/automacoes/${seg(id)}/execucoes`, query: filtro });
  }

  /** Execuções de todas as automações, mais recente primeiro. */
  listarExecucoes(filtro: FiltroExecucoes = {}): Promise<Pagina<Execucao>> {
    return this.requisitar({ metodo: 'GET', caminho: '/execucoes', query: filtro });
  }

  obterExecucao(id: Id): Promise<ExecucaoDetalhe> {
    return this.requisitar({ metodo: 'GET', caminho: `/execucoes/${seg(id)}` });
  }

  listarSessoesChatbot(automacaoId: Id, filtro: FiltroSessoes = {}): Promise<Pagina<SessaoChatbot>> {
    return this.requisitar({ metodo: 'GET', caminho: `/automacoes/${seg(automacaoId)}/sessoes`, query: filtro });
  }

  // -------------------------------------------------------------------------
  // 002 — Simulador de chatbot
  // -------------------------------------------------------------------------

  iniciarSimulador(automacaoId: Id, pedido: PedidoSimulador = {}): Promise<InicioSimulacao> {
    return this.requisitar({ metodo: 'POST', caminho: `/automacoes/${seg(automacaoId)}/simulador`, json: pedido });
  }

  enviarAoSimulador(simulacaoId: Id, texto: string): Promise<RodadaSimulacao> {
    return this.requisitar({ metodo: 'POST', caminho: `/simulador/${seg(simulacaoId)}/mensagens`, json: { texto } });
  }

  encerrarSimulador(simulacaoId: Id): Promise<void> {
    return this.requisitar({ metodo: 'DELETE', caminho: `/simulador/${seg(simulacaoId)}`, resposta: 'nada' });
  }

  // -------------------------------------------------------------------------
  // 002 — Automações de IA: projeto e compilação
  // -------------------------------------------------------------------------

  listarModelosProjeto(): Promise<ModeloProjeto[]> {
    return this.requisitar({ metodo: 'GET', caminho: '/automacoes/modelos' });
  }

  obterSdkAutomacao(): Promise<SdkAutomacao> {
    return this.requisitar({ metodo: 'GET', caminho: '/automacoes/sdk' });
  }

  /** Sem `.zapdesk/` e `tsconfig.json`. */
  listarArquivos(automacaoId: Id): Promise<ArquivoProjeto[]> {
    return this.requisitar({ metodo: 'GET', caminho: `/automacoes/${seg(automacaoId)}/arquivos` });
  }

  async lerArquivo(automacaoId: Id, caminho: string): Promise<ConteudoArquivo> {
    return this.requisitar({ metodo: 'GET', caminho: this.caminhoArquivo(automacaoId, caminho) });
  }

  /**
   * Cria ou substitui um arquivo do projeto. `hashAnterior`: `undefined` = sobrescreve sem
   * conferir; `null` = o arquivo deve ser novo; string = `conflito` se o arquivo mudou.
   */
  async escreverArquivo(
    automacaoId: Id,
    caminho: string,
    conteudo: string,
    hashAnterior?: string | null,
  ): Promise<ArquivoProjeto> {
    return this.requisitar({
      metodo: 'PUT',
      caminho: this.caminhoArquivo(automacaoId, caminho),
      json: hashAnterior === undefined ? { conteudo } : { conteudo, hash_anterior: hashAnterior },
    });
  }

  async excluirArquivo(automacaoId: Id, caminho: string): Promise<void> {
    return this.requisitar({ metodo: 'DELETE', caminho: this.caminhoArquivo(automacaoId, caminho), resposta: 'nada' });
  }

  renomearArquivo(automacaoId: Id, de: string, para: string): Promise<ArquivoProjeto> {
    return this.requisitar({
      metodo: 'POST',
      caminho: `/automacoes/${seg(automacaoId)}/arquivos/renomear`,
      json: { de, para },
    });
  }

  compilarAutomacao(automacaoId: Id): Promise<ResultadoCompilacao> {
    return this.requisitar({ metodo: 'POST', caminho: `/automacoes/${seg(automacaoId)}/compilar` });
  }

  /**
   * `/automacoes/{id}/arquivos/{caminho...}` com cada segmento codificado. Segmentos vazios, `.`
   * e `..` seriam normalizados pela URL (mudando o alvo), então são recusados aqui.
   */
  private caminhoArquivo(automacaoId: Id, caminho: string): string {
    const segmentos = caminho.split('/');
    if (segmentos.some((s) => s === '' || s === '.' || s === '..')) {
      throw new ErroMotor('validacao', 'Caminho de arquivo inválido.', 0, { campos: { caminho: 'inválido' } });
    }
    return `/automacoes/${seg(automacaoId)}/arquivos/${segmentos.map(seg).join('/')}`;
  }

  // -------------------------------------------------------------------------
  // 002 — Conversas: estado das automações e pausas
  // -------------------------------------------------------------------------

  estadoAutomacoesConversa(conversaId: Id): Promise<EstadoConversaAutomacoes> {
    return this.requisitar({ metodo: 'GET', caminho: `/conversas/${seg(conversaId)}/automacoes` });
  }

  /** "Assumir" (encerra a sessão de bot como `humano`). `duracao_min` ausente/null = sem prazo. */
  pausarConversa(conversaId: Id, pedido: PedidoPausa): Promise<Pausa> {
    return this.requisitar({ metodo: 'POST', caminho: `/conversas/${seg(conversaId)}/pausa`, json: pedido });
  }

  /** "Devolver às automações" / "Retomar". */
  retomarConversa(conversaId: Id): Promise<void> {
    return this.requisitar({ metodo: 'DELETE', caminho: `/conversas/${seg(conversaId)}/pausa`, resposta: 'nada' });
  }

  /** Pausas ativas. */
  listarPausas(motivo?: MotivoPausa): Promise<Pausa[]> {
    return this.requisitar({ metodo: 'GET', caminho: '/pausas', query: { motivo } });
  }

  // -------------------------------------------------------------------------
  // 002 — Configuração, IA e segredos
  // -------------------------------------------------------------------------

  obterConfiguracaoAutomacoes(): Promise<ConfiguracaoAutomacoes> {
    return this.requisitar({ metodo: 'GET', caminho: '/automacoes/configuracao' });
  }

  editarConfiguracaoAutomacoes(dados: Partial<ConfiguracaoAutomacoes>): Promise<ConfiguracaoAutomacoes> {
    return this.requisitar({ metodo: 'PATCH', caminho: '/automacoes/configuracao', json: dados });
  }

  obterConfiguracaoIA(): Promise<ConfiguracaoIA> {
    return this.requisitar({ metodo: 'GET', caminho: '/ia/configuracao' });
  }

  editarConfiguracaoIA(dados: { modelo_padrao: string }): Promise<ConfiguracaoIA> {
    return this.requisitar({ metodo: 'PATCH', caminho: '/ia/configuracao', json: dados });
  }

  /** "Testar chave" — `ia_nao_configurada` sem chave; `ia_erro` se a Claude API recusar. */
  testarChaveIA(modelo?: string): Promise<ResultadoTesteChave> {
    return this.requisitar({ metodo: 'POST', caminho: '/ia/testar-chave', json: modelo ? { modelo } : {} });
  }

  /** Somente nomes: valores nunca saem do motor. */
  listarSegredos(): Promise<Segredo[]> {
    return this.requisitar({ metodo: 'GET', caminho: '/segredos' });
  }

  // -------------------------------------------------------------------------
  // Modo falso (/v1/falso/*) — só existem com --whatsapp=falso (404 no modo real)
  // -------------------------------------------------------------------------

  readonly falso = {
    escanearQr: (contaId: Id, dados: FalsoEscanearQr): Promise<unknown> =>
      this.requisitar({ metodo: 'POST', caminho: `/falso/contas/${seg(contaId)}/escanear-qr`, json: dados }),
    expirarQr: (contaId: Id): Promise<unknown> =>
      this.requisitar({ metodo: 'POST', caminho: `/falso/contas/${seg(contaId)}/expirar-qr` }),
    mensagemRecebida: (contaId: Id, dados: FalsoMensagemRecebida): Promise<FalsoInjetada> =>
      this.requisitar({ metodo: 'POST', caminho: `/falso/contas/${seg(contaId)}/mensagem-recebida`, json: dados }),
    recibo: (contaId: Id, dados: FalsoRecibo): Promise<unknown> =>
      this.requisitar({ metodo: 'POST', caminho: `/falso/contas/${seg(contaId)}/recibo`, json: dados }),
    estado: (contaId: Id, evento: FalsoEventoEstado): Promise<unknown> =>
      this.requisitar({ metodo: 'POST', caminho: `/falso/contas/${seg(contaId)}/estado`, json: { evento } }),
    historico: (contaId: Id, dados: FalsoHistorico): Promise<unknown> =>
      this.requisitar({ metodo: 'POST', caminho: `/falso/contas/${seg(contaId)}/historico`, json: dados }),
    status: (contaId: Id, dados: FalsoStatus): Promise<FalsoInjetada> =>
      this.requisitar({ metodo: 'POST', caminho: `/falso/contas/${seg(contaId)}/status`, json: dados }),
    numerosSemWhatsApp: (telefones: string[]): Promise<unknown> =>
      this.requisitar({ metodo: 'PUT', caminho: '/falso/numeros-sem-whatsapp', json: { telefones } }),
    falhasEnvio: (dados: FalsoFalhasEnvio): Promise<unknown> =>
      this.requisitar({ metodo: 'PUT', caminho: '/falso/falhas-envio', json: dados }),
    enviadas: (): Promise<FalsoEnviada[]> =>
      this.requisitar({ metodo: 'GET', caminho: '/falso/enviadas' }),
    relogio: (dados: FalsoRelogio): Promise<unknown> =>
      this.requisitar({ metodo: 'PUT', caminho: '/falso/relogio', json: dados }),
    // 002 — só com --ia=falsa (ia, iaChamadas) / modo falso (segredos, processarEsperas)
    ia: (dados: FalsoIA): Promise<unknown> =>
      this.requisitar({ metodo: 'PUT', caminho: '/falso/ia', json: dados }),
    iaChamadas: (): Promise<FalsoChamadaIA[]> =>
      this.requisitar({ metodo: 'GET', caminho: '/falso/ia/chamadas' }),
    /** Mesmo efeito do comando `segredos` do stdin (substitui o conjunto inteiro). */
    segredos: (valores: Record<string, string>): Promise<unknown> =>
      this.requisitar({ metodo: 'POST', caminho: '/falso/segredos', json: { valores } }),
    /** Uma passada do agendador de esperas; responde quando os despachos terminarem. */
    processarEsperas: (): Promise<unknown> =>
      this.requisitar({ metodo: 'POST', caminho: '/falso/processar-esperas' }),
  };

  // -------------------------------------------------------------------------

  private async baixarBinario(caminho: string): Promise<Blob> {
    let resposta: Response;
    try {
      resposta = await this.fetch(this.montarUrl(caminho), {
        method: 'GET',
        headers: { Authorization: `Bearer ${this.token}` },
      });
    } catch (causa) {
      throw new ErroMotor('interno', MENSAGEM_MOTOR_FORA, 0, {
        motivo: causa instanceof Error ? causa.message : String(causa),
      });
    }
    if (!resposta.ok) throw parsearErro(resposta.status, await resposta.text().catch(() => ''));
    return resposta.blob();
  }
}
