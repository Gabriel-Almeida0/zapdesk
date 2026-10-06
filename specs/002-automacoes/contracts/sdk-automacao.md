# Contrato da SDK `@zapdesk/automacao` — v1

Pacote: `compartilhado/automacao-sdk` (nome npm `@zapdesk/automacao`). Contém **tipos** (a API do
`ctx` abaixo, fonte da verdade) e um runtime mínimo (`definirAutomacao` e classes de erro). O
**runner** fornece o módulo em tempo de execução (o bundle do usuário importa
`@zapdesk/automacao` como externo; o runner o resolve para sua própria cópia) e constrói o `ctx`.
O `.d.ts` gerado é copiado para o motor (`go:embed`) e servido em `GET /v1/automacoes/sdk` para o
Monaco, o VS Code (`.zapdesk/automacao.d.ts`) e o MCP (`ver_tipos_sdk`).

Regras gerais:

- Toda chamada assíncrona do `ctx` vira uma requisição JSON-RPC ao motor (`runner-protocolo.md`),
  validada contra `permissoes` do manifesto; sem permissão → `ErroPermissao`.
- Em simulação (`ctx.execucao.simulacao === true`), escritas não acontecem: retornam o resultado
  **simulado** (ex.: `MensagemEnviada` com `simulada: true`) e aparecem em "ações que seriam feitas".
- Chamadas feitas depois do fim da execução (promessas soltas) falham com `ErroExecucaoEncerrada`.
- `console.log/info/warn/error` equivalem a `ctx.log.*` da execução corrente (fora de execução:
  descartados).
- **Limite honesto**: o isolamento protege contra erros e acidentes, não contra código malicioso.

```ts
// ===================== index.d.ts — @zapdesk/automacao =====================

export type Json = null | boolean | number | string | Json[] | { [chave: string]: Json };
export type Id = string;
export type DataIso = string; // RFC 3339

export type Permissao =
  | 'enviar' | 'ler_conversas' | 'etiquetas' | 'funil' | 'leads' | 'ia' | 'rede' | 'agendar';

export type ModeloClaude =
  | 'claude-sonnet-5' | 'claude-opus-5-5' | 'claude-haiku-4-5-20251001' | 'claude-haiku-4-5'
  | (string & {}); // qualquer id "claude-…" aceito pelo motor

// ---------- definição ----------

export interface DefinicaoAutomacao {
  /** Gatilhos `mensagem_recebida` e `palavra_chave`. */
  aoReceberMensagem?(ctx: Contexto, mensagem: MensagemRecebida): Promise<void> | void;
  /** Gatilho `agendamento` e chamadas marcadas com `ctx.agendar`. */
  aoAgendar?(ctx: Contexto, agendamento: Agendamento): Promise<void> | void;
  /** Execução manual (app/MCP), ação `executar_ia` de fluxo e nó `ia` de chatbot. O retorno
   *  (JSON, ≤ 64 KB) volta a quem chamou; string retornada a um nó `ia` em modo `responder` é enviada. */
  aoExecutar?(ctx: Contexto, entrada: Json): Promise<Json | void> | Json | void;
  /** Gatilhos `lead_importado`, `etiqueta`, `entrou_etapa`, `disparo_respondeu`, `sem_resposta`. */
  aoEvento?(ctx: Contexto, evento: EventoAutomacao): Promise<void> | void;
}

/** Marca e valida (em tempo de execução) a definição. Uso: `export default definirAutomacao({...})`. */
export declare function definirAutomacao<D extends DefinicaoAutomacao>(definicao: D): D;

// ---------- contexto ----------

export interface Contexto {
  readonly execucao: InfoExecucao;
  /** Conversa do gatilho (ou do alvo manual); `null` em eventos sem conversa (ex.: agendamento). */
  readonly conversa: ApiConversa | null;

  /** Responde na conversa atual. Permissão `enviar`. Erro se `conversa === null`. */
  responder(texto: string, opcoes?: { citar?: boolean }): Promise<MensagemEnviada>;
  /** Envia para uma conversa ou telefone. Permissão `enviar`. Passa pelo portão
   *  (pausa, grupos, anti-loop, primeiro contato) → `ErroBloqueado` quando barrado. */
  enviar(destino: Destino, conteudo: Conteudo): Promise<MensagemEnviada>;
  /** Reage a uma mensagem ("" remove). Permissão `enviar`. */
  reagir(mensagemId: Id, emoji: string): Promise<void>;

  readonly etiquetas: ApiEtiquetas;
  readonly funil: ApiFunil;
  readonly leads: ApiLeads;
  readonly memoria: ApiMemoria;
  readonly ia: ApiIA;
  readonly http: ApiHttp;
  readonly log: ApiLog;
  readonly segredos: ApiSegredos;
  readonly humano: ApiHumano;

  /** Agenda uma chamada futura de `aoAgendar` (permissão `agendar`). Mínimo 60 s, máximo 30 dias. */
  agendar(opcoes: OpcoesAgendar): Promise<{ id: Id; em: DataIso }>;
  cancelarAgendamento(id: Id): Promise<boolean>;
  /** Notificação do macOS ao operador (sem permissão; máx. 5 por execução). */
  notificar(titulo: string, texto: string): Promise<void>;
  /** Sinal de cancelamento (tempo limite próximo ou execução cancelada). */
  readonly sinal: AbortSignal;
}

export interface InfoExecucao {
  id: Id;
  automacaoId: Id;
  automacaoNome: string;
  gatilho: { tipo: TipoGatilho; dados: Record<string, Json> };
  origem: 'gatilho' | 'manual_app' | 'manual_mcp' | 'teste' | 'fluxo' | 'chatbot';
  simulacao: boolean;
  iniciadaEm: DataIso;
  /** Milissegundos até o tempo limite. */
  prazoMs: number;
}

export type TipoGatilho =
  | 'mensagem_recebida' | 'palavra_chave' | 'lead_importado' | 'etiqueta' | 'entrou_etapa'
  | 'disparo_respondeu' | 'sem_resposta' | 'agendamento' | 'agendar' | 'manual';

// ---------- conversa ----------

export interface ApiConversa {
  readonly id: Id;
  readonly contaId: Id;
  readonly tipo: 'individual' | 'grupo';
  readonly nome: string | null;
  readonly telefone: string | null;       // E.164; null em grupo
  readonly contato: ContatoResumo | null;
  readonly leadId: Id | null;
  /** Permissão `ler_conversas`. Mais recentes primeiro; `limite` 1–200 (padrão 30). */
  historico(opcoes?: { limite?: number; antes?: Id }): Promise<Mensagem[]>;
  /** Permissão `ler_conversas`. Histórico em ordem cronológica no formato de mensagens da IA
   *  (contato → "user", eu/automação → "assistant"; mensagens seguidas do mesmo lado unidas;
   *  mídia vira "[imagem]", "[áudio]"…). `limite` padrão 20. */
  historicoParaIA(opcoes?: { limite?: number }): Promise<MensagemIA[]>;
}

export interface ContatoResumo {
  id: Id;
  nome: string | null;
  nomePush: string | null;
  telefone: string | null;
  notas: string | null;
  etiquetas: Etiqueta[];
}

export interface Mensagem {
  id: Id;
  conversaId: Id;
  deMim: boolean;
  automacaoId: Id | null;
  tipo: 'texto' | 'imagem' | 'video' | 'audio' | 'documento' | 'figurinha' | 'sistema';
  texto: string | null;
  remetenteNome: string | null;
  enviadaEm: DataIso;
}

export interface MensagemRecebida extends Mensagem {
  deMim: false;
  /** Primeira mensagem recebida desse contato nesta conta. */
  primeira: boolean;
  midia: { mimetype: string; nomeArquivo: string | null; tamanho: number } | null;
}

export type Destino = { conversaId: Id } | { telefone: string; contaId?: Id };

export type Conteudo =
  | { texto: string }                                                    // 1–4096
  | { template: string; variaveis?: Record<string, string> }             // nome ou id do template
  | { arquivoId: Id; legenda?: string; como?: 'auto' | 'voz' | 'figurinha' | 'documento' };

export interface MensagemEnviada {
  id: Id | null;            // null em simulação
  conversaId: Id | null;
  simulada: boolean;
  texto: string | null;
}

// ---------- etiquetas, funil, leads ----------

export interface Etiqueta { id: Id; nome: string; cor: string }

/** Permissão `etiquetas`. `alvo` padrão: contato da conversa atual. Etiqueta por nome ou id. */
export interface ApiEtiquetas {
  listar(): Promise<Etiqueta[]>;
  doContato(alvo?: Alvo): Promise<Etiqueta[]>;
  adicionar(etiqueta: string, alvo?: Alvo): Promise<void>;
  remover(etiqueta: string, alvo?: Alvo): Promise<void>;
}

export type Alvo = { contatoId: Id } | { leadId: Id } | { telefone: string } | { conversaId: Id };

export interface Funil { id: Id; nome: string; etapas: { id: Id; nome: string; cor: string; ordem: number }[] }
export interface PosicaoFunil { funilId: Id; funil: string; etapaId: Id; etapa: string; desde: DataIso }

/** Permissão `funil`. Funil e etapa por nome ou id. `alvo` padrão: lead da conversa atual
 *  (criado se preciso, como no app). */
export interface ApiFunil {
  listar(): Promise<Funil[]>;
  posicao(funil: string, alvo?: Alvo): Promise<PosicaoFunil | null>;
  mover(funil: string, etapa: string, alvo?: Alvo): Promise<PosicaoFunil>;
  remover(funil: string, alvo?: Alvo): Promise<void>;
}

export interface Lead {
  id: Id;
  telefone: string;
  nome: string | null;
  campos: Record<string, string>;
  origem: 'csv' | 'colado' | 'contatos' | 'mcp';
  importadoEm: DataIso;
}

/** Permissão `leads`. */
export interface ApiLeads {
  /** Lead da conversa atual (ou do evento); null se o contato não é lead. */
  atual(): Promise<Lead | null>;
  obter(id: Id): Promise<Lead | null>;
  buscarPorTelefone(telefone: string): Promise<Lead | null>;
  /** Merge de campos (`null` remove); `nome` opcional. Padrão: lead atual (criado se preciso). */
  atualizar(dados: { nome?: string | null; campos?: Record<string, string | null> }, alvo?: Alvo): Promise<Lead>;
}

// ---------- memória ----------

/** Sem permissão. Isolada por automação. `escopo: 'contato'` usa o contato atual (erro se não houver).
 *  Chave 1–200; valor JSON ≤ 64 KB; até 10.000 chaves. Em simulação, escritas valem só na execução. */
export interface ApiMemoria {
  obter<T extends Json = Json>(chave: string, opcoes?: OpcoesMemoria): Promise<T | null>;
  definir(chave: string, valor: Json, opcoes?: OpcoesMemoria): Promise<void>;
  remover(chave: string, opcoes?: OpcoesMemoria): Promise<boolean>;
  listar(opcoes?: OpcoesMemoria & { prefixo?: string }): Promise<{ chave: string; valor: Json }[]>;
}
export interface OpcoesMemoria { escopo?: 'global' | 'contato' }

// ---------- IA (Claude, chamada pelo motor) ----------

export interface MensagemIA { papel: 'user' | 'assistant'; texto: string }

export interface OpcoesIA {
  modelo?: ModeloClaude;          // padrão: manifesto `ia.modelo` → Ajustes
  sistema?: string;               // prompt de sistema
  maxTokens?: number;             // 1–8192, padrão 1024
}

/** Permissão `ia`. Sem temperatura (os modelos atuais a recusam). Retentativas automáticas
 *  (429/5xx/529, até 3) dentro do prazo. Chave ausente/inválida → `ErroIA` com
 *  "Configure a chave da Anthropic em Ajustes → IA". Tokens somados na execução. */
export interface ApiIA {
  gerar(pedido: { prompt?: string; mensagens?: MensagemIA[] } & OpcoesIA): Promise<RespostaIA>;
  /** Escolhe exatamente uma categoria (saída estruturada com enum). */
  classificar<C extends string>(
    texto: string,
    categorias: readonly C[] | Record<C, string>,   // lista ou {categoria: descrição}
    opcoes?: OpcoesIA & { instrucoes?: string },
  ): Promise<{ categoria: C; tokens: TokensIA }>;
  /** Extrai dados conforme um JSON Schema (subconjunto: object/array/string/number/integer/
   *  boolean/enum/null/required/description; sem recursão nem min/max). */
  extrair<T = Json>(texto: string, esquema: EsquemaJson, opcoes?: OpcoesIA & { instrucoes?: string }): Promise<{ dados: T; tokens: TokensIA }>;
}

export interface RespostaIA {
  texto: string;
  modelo: string;
  motivoParada: 'end_turn' | 'max_tokens' | 'stop_sequence' | 'refusal' | (string & {});
  tokens: TokensIA;
}
export interface TokensIA { entrada: number; saida: number }
export type EsquemaJson = { [chave: string]: Json };

// ---------- HTTP (rede) ----------

/** Permissão `rede`. Mesma assinatura do fetch global (que é removido dentro das automações).
 *  URL http(s) apenas; registrado no log (método, host+caminho, status, duração — sem query/corpo).
 *  Respeita `ctx.sinal`. */
export interface ApiHttp {
  fetch(url: string | URL, init?: RequestInit): Promise<Response>;
}

// ---------- log, segredos, humano ----------

export interface ApiLog {
  debug(...partes: unknown[]): void;
  info(...partes: unknown[]): void;
  aviso(...partes: unknown[]): void;
  erro(...partes: unknown[]): void;
}

/** Só segredos declarados em `automacao.json` › `segredos`. Síncrono (entregues na inicialização
 *  do processo). Não declarado ou ausente → `ErroSegredo`. Nunca logue o valor. */
export interface ApiSegredos {
  obter(nome: string): string;
  tem(nome: string): boolean;
}

/** Permissão `enviar`. Coloca a conversa atual em atendimento humano (pausa sem prazo ou
 *  `duracaoMin`), envia `mensagem` se informada (pelo portão) e notifica o operador. */
export interface ApiHumano {
  transferir(opcoes?: { motivo?: string; mensagem?: string; duracaoMin?: number }): Promise<void>;
}

// ---------- agendamento e eventos ----------

export interface OpcoesAgendar {
  em?: Date | DataIso;            // exatamente um de `em` / `daquiSegundos`
  daquiSegundos?: number;
  dados?: Json;                   // ≤ 16 KB
  /** Mantém a conversa atual como contexto da chamada futura (padrão true se houver conversa). */
  naConversa?: boolean;
}

export interface Agendamento {
  origem: 'cron' | 'intervalo' | 'agendar';
  id: Id | null;                  // id de ctx.agendar
  previstoPara: DataIso;
  dados: Json | null;
}

export type EventoAutomacao =
  | { tipo: 'lead_importado'; lead: Lead }
  | { tipo: 'etiqueta'; evento: 'adicionada' | 'removida'; etiqueta: Etiqueta; contatoId: Id }
  | { tipo: 'entrou_etapa'; funilId: Id; etapaId: Id; leadId: Id; etapaAnteriorId: Id | null }
  | { tipo: 'disparo_respondeu'; disparoId: Id; destinatarioId: Id; mensagemId: Id | null }
  | { tipo: 'sem_resposta'; aposSegundos: number; mensagemReferenciaId: Id };

// ---------- erros ----------

export declare class ErroAutomacao extends Error { readonly codigo: string; }
export declare class ErroPermissao extends ErroAutomacao { readonly permissao: Permissao; }   // "Permissão 'enviar' não declarada em automacao.json"
export declare class ErroBloqueado extends ErroAutomacao {                                     // envio barrado pelo portão
  readonly motivo: 'pausa' | 'pausa_geral' | 'grupo' | 'anti_loop' | 'primeiro_contato';
}
export declare class ErroIA extends ErroAutomacao { readonly status: number | null; readonly requestId: string | null; }
export declare class ErroSegredo extends ErroAutomacao {}
export declare class ErroValidacao extends ErroAutomacao { readonly campos: Record<string, string>; }
export declare class ErroNaoEncontrado extends ErroAutomacao {}
export declare class ErroExecucaoEncerrada extends ErroAutomacao {}

// ---------- módulos de texto (carregador "text" do esbuild) ----------
declare module '*.md' { const texto: string; export default texto; }
declare module '*.txt' { const texto: string; export default texto; }
```

## Exemplo (modelo "Responder com IA usando histórico")

```ts
import { definirAutomacao } from '@zapdesk/automacao';
import prompt from './prompt.md';            // arquivos .md/.txt importados como texto

export default definirAutomacao({
  async aoReceberMensagem(ctx, msg) {
    if (!msg.texto) return;
    const historico = await ctx.conversa!.historicoParaIA({ limite: 20 });
    const resposta = await ctx.ia.gerar({ sistema: prompt, mensagens: historico });
    if (resposta.texto.includes('[HUMANO]')) {
      await ctx.humano.transferir({ mensagem: 'Vou chamar um atendente.' });
      return;
    }
    await ctx.responder(resposta.texto);
    ctx.log.info('respondeu', { tokens: resposta.tokens });
  },
});
```

Carregadores do esbuild na compilação: `.ts` (TypeScript), `.json` (JSON), `.md`/`.txt` (texto).
