// Monta o `ctx` de uma execução: cada chamada assíncrona vira uma requisição JSON-RPC `ctx.*` ao
// motor (que valida permissões, simulação, portão e limites). Aqui ficam só as regras do lado
// runner: conversão snake/camel, `AbortSignal`, fim da execução (chamadas depois do fim →
// `ErroExecucaoEncerrada`), segredos síncronos, `ctx.http` e `historicoParaIA`.
import {
  ErroAutomacao,
  ErroBloqueado,
  ErroExecucaoEncerrada,
  ErroIA,
  ErroNaoEncontrado,
  ErroPermissao,
  ErroSegredo,
  ErroValidacao,
  type Alvo,
  type ApiConversa,
  type Conteudo,
  type ContatoResumo,
  type Contexto,
  type Destino,
  type Etiqueta,
  type Funil,
  type InfoExecucao,
  type Json,
  type Lead,
  type Mensagem,
  type MensagemEnviada,
  type MensagemRecebida,
  type MotivoBloqueio,
  type OpcoesAgendar,
  type OpcoesIA,
  type Permissao,
  type PosicaoFunil,
  type RespostaIA,
  type TokensIA,
} from '@zapdesk/automacao';
import { formatarPartes } from './console.js';
import { historicoParaIA } from './historico-ia.js';
import { criarHttp } from './http.js';
import { camelizar, semIndefinidos, snakeizar } from './normalizar.js';
import {
  CODIGOS_ERRO,
  type DadosErroMotor,
  type MetodoCtx,
  type MetodosCtx,
  type NivelLog,
  type ParamsExecutar,
  type ParamsHttp,
  type ParamsLog,
} from './protocolo.js';
import { ErroRpc } from './rpc.js';

export interface Chamador {
  requisitar(metodo: string, params: unknown): Promise<unknown>;
}

export interface OpcoesContexto {
  rpc: Chamador;
  execucao: ParamsExecutar;
  automacao: { id: string; nome: string };
  permissoes: readonly string[];
  segredos: Readonly<Record<string, string>>;
  fetchInterno: typeof fetch;
  /** Envia notificações `log`/`http` ao motor. */
  notificar(metodo: 'log', params: ParamsLog): void;
  notificar(metodo: 'http', params: ParamsHttp): void;
  /** Relógio (ms); injetável nos testes. */
  agora?: () => number;
}

export interface ControleExecucao {
  readonly ctx: Contexto;
  /** Argumento do handler já no formato da SDK. */
  readonly argumento: unknown;
  /** `cancelar` do motor: aborta o sinal e rejeita as chamadas pendentes e futuras. */
  cancelar(motivo?: string): void;
  /** Fim normal da execução (mesmo efeito, sem motivo de cancelamento). */
  encerrar(): void;
  readonly encerrada: boolean;
}

/** Converte um erro JSON-RPC do motor na classe da SDK (tabela "Códigos de erro"). */
export function erroDoMotor(erro: unknown): Error {
  if (!(erro instanceof ErroRpc)) return erro instanceof Error ? erro : new ErroAutomacao(String(erro));
  const d = (erro.data ?? {}) as DadosErroMotor;
  const msg = erro.message;
  switch (erro.code) {
    case CODIGOS_ERRO.permissao_negada: {
      const permissao = (d.permissao ?? /'([a-z_]+)'/.exec(msg)?.[1] ?? 'enviar') as Permissao;
      return new ErroPermissao(permissao, msg);
    }
    case CODIGOS_ERRO.validacao:
      return new ErroValidacao(msg, { campos: d.campos ?? {} });
    case CODIGOS_ERRO.nao_encontrado:
      return new ErroNaoEncontrado(msg);
    case CODIGOS_ERRO.bloqueado:
      return new ErroBloqueado((d.motivo ?? 'pausa') as MotivoBloqueio, msg);
    case CODIGOS_ERRO.ia_nao_configurada:
      return new ErroIA(msg, { codigo: 'ia_nao_configurada', status: d.status ?? null });
    case CODIGOS_ERRO.ia_erro:
      return new ErroIA(msg, { status: d.status ?? null, requestId: d.request_id ?? null });
    case CODIGOS_ERRO.execucao_encerrada:
      return new ErroExecucaoEncerrada(msg);
    case CODIGOS_ERRO.limite:
      return new ErroValidacao(msg, { codigo: 'limite', campos: d.campos ?? {} });
    case CODIGOS_ERRO.segredo:
      return new ErroSegredo(msg);
    case CODIGOS_ERRO.conta_indisponivel:
      return new ErroAutomacao(msg, { codigo: 'conta_indisponivel' });
    default:
      if (d.codigo === 'limite') return new ErroValidacao(msg, { codigo: 'limite' });
      return new ErroAutomacao(msg, { codigo: typeof d.codigo === 'string' ? d.codigo : 'erro' });
  }
}

/** Troca os quadros do stack de `erro` pelos de `origem` (mantém a 1ª linha "Nome: mensagem"). */
function comPilha(erro: Error, origem: Error): Error {
  const quadros = (origem.stack ?? '').split('\n').slice(1).join('\n');
  if (!quadros) return erro;
  try {
    Object.defineProperty(erro, 'stack', {
      value: `${erro.name}: ${erro.message}\n${quadros}`,
      writable: true,
      configurable: true,
    });
  } catch {
    // stack não configurável: mantém o original
  }
  return erro;
}

type Params<M extends MetodoCtx> = MetodosCtx[M]['params'];
type Resultado<M extends MetodoCtx> = MetodosCtx[M]['resultado'];

export function criarContexto(op: OpcoesContexto): ControleExecucao {
  const agora = op.agora ?? Date.now;
  const exec = op.execucao;
  const execucaoId = exec.execucao_id;
  const inicio = agora();
  const controlador = new AbortController();
  const pendentes = new Set<(erro: Error) => void>();
  let encerrada = false;

  const prazoTimer =
    exec.prazo_ms > 0
      ? setTimeout(() => controlador.abort(new ErroAutomacao('Tempo limite da execução.', { codigo: 'tempo' })), exec.prazo_ms)
      : null;
  prazoTimer?.unref?.();

  function garantirAtiva(): void {
    if (encerrada) throw new ErroExecucaoEncerrada();
  }

  function chamar<M extends MetodoCtx>(metodo: M, params: Params<M>): Promise<Resultado<M>> {
    // Pilha capturada na chamada: os erros do motor chegam num callback sem os quadros do
    // código do usuário; anexamos os daqui para o stack apontar a linha do .ts que chamou o ctx.
    const origem = new Error();
    if (encerrada) return Promise.reject(comPilha(new ErroExecucaoEncerrada(), origem));
    return new Promise<Resultado<M>>((resolver, rejeitar) => {
      let feito = false;
      const abortar = (erro: Error) => {
        if (feito) return;
        feito = true;
        rejeitar(comPilha(erro, origem));
      };
      pendentes.add(abortar);
      op.rpc
        .requisitar(metodo, { execucao_id: execucaoId, ...semIndefinidos(params as Record<string, unknown>) })
        .then(
          (r) => {
            pendentes.delete(abortar);
            if (feito) return;
            feito = true;
            resolver(r as Resultado<M>);
          },
          (e: unknown) => {
            pendentes.delete(abortar);
            abortar(erroDoMotor(e));
          },
        );
    });
  }

  function finalizar(erro: Error): void {
    if (encerrada) return;
    encerrada = true;
    if (prazoTimer) clearTimeout(prazoTimer);
    for (const abortar of [...pendentes]) abortar(erro);
    pendentes.clear();
    if (!controlador.signal.aborted) controlador.abort(erro);
  }

  const alvoSnake = (alvo?: Alvo) => (alvo ? snakeizar<MetodosCtx['ctx.funil.mover']['params']['alvo']>(alvo) : undefined);

  // ---------- info ----------
  const infoBruta = camelizar<Partial<InfoExecucao>>(exec.info ?? {});
  const info: InfoExecucao = {
    id: infoBruta.id ?? execucaoId,
    automacaoId: infoBruta.automacaoId ?? op.automacao.id,
    automacaoNome: infoBruta.automacaoNome ?? op.automacao.nome,
    gatilho: infoBruta.gatilho ?? { tipo: 'manual', dados: {} },
    origem: infoBruta.origem ?? 'gatilho',
    simulacao: exec.simulacao,
    iniciadaEm: infoBruta.iniciadaEm ?? new Date(inicio).toISOString(),
    prazoMs: 0,
  };
  Object.defineProperty(info, 'prazoMs', {
    enumerable: true,
    get: () => Math.max(0, exec.prazo_ms - (agora() - inicio)),
  });
  Object.freeze(info);

  // ---------- argumento ----------
  const argumento: unknown =
    exec.handler === 'aoExecutar' ? (exec.argumento ?? null) : camelizar(exec.argumento ?? null);
  const mensagemGatilho =
    exec.handler === 'aoReceberMensagem' ? (argumento as Partial<MensagemRecebida> | null) : null;

  // ---------- conversa ----------
  let conversa: ApiConversa | null = null;
  if (exec.conversa) {
    const c = camelizar<{
      id: string;
      contaId: string;
      tipo: 'individual' | 'grupo';
      nome: string | null;
      telefone: string | null;
      leadId: string | null;
      contato: ContatoResumo | null;
    }>(exec.conversa);
    const historico = async (opcoes?: { limite?: number; antes?: string }): Promise<Mensagem[]> => {
      const r = await chamar('ctx.conversa.historico', {
        conversa_id: c.id,
        limite: opcoes?.limite,
        antes: opcoes?.antes,
      });
      return camelizar<Mensagem[]>(r);
    };
    conversa = Object.freeze({
      id: c.id,
      contaId: c.contaId,
      tipo: c.tipo,
      nome: c.nome ?? null,
      telefone: c.telefone ?? null,
      contato: c.contato ?? null,
      leadId: c.leadId ?? null,
      historico,
      async historicoParaIA(opcoes?: { limite?: number }) {
        return historicoParaIA(await historico({ limite: opcoes?.limite ?? 20 }));
      },
    });
  }

  // ---------- log ----------
  const log = (nivel: NivelLog) => (...partes: unknown[]) => {
    if (encerrada) return;
    op.notificar('log', {
      execucao_id: execucaoId,
      nivel,
      texto: formatarPartes(partes),
      em: new Date(agora()).toISOString(),
    });
  };

  const opcoesIA = (o?: OpcoesIA) => ({ modelo: o?.modelo, sistema: o?.sistema, max_tokens: o?.maxTokens });

  const enviar = async (destino: Destino, conteudo: Conteudo, citar?: string): Promise<MensagemEnviada> => {
    const r = await chamar('ctx.enviar', {
      destino: snakeizar(destino),
      conteudo: snakeizar(conteudo),
      citar_mensagem_id: citar,
    });
    return camelizar<MensagemEnviada>(r);
  };

  const ctx: Contexto = {
    execucao: info,
    conversa,

    async responder(texto, opcoes) {
      if (!conversa) throw new ErroValidacao('Esta execução não tem conversa.', { campos: { conversa: 'ausente' } });
      const citar = opcoes?.citar && mensagemGatilho?.id ? mensagemGatilho.id : undefined;
      return enviar({ conversaId: conversa.id }, { texto }, citar);
    },
    enviar: (destino, conteudo) => enviar(destino, conteudo),
    async reagir(mensagemId, emoji) {
      await chamar('ctx.reagir', { mensagem_id: mensagemId, emoji });
    },

    etiquetas: {
      listar: async () => camelizar<Etiqueta[]>(await chamar('ctx.etiquetas.listar', {})),
      doContato: async (alvo) => camelizar<Etiqueta[]>(await chamar('ctx.etiquetas.do_contato', { alvo: alvoSnake(alvo) })),
      async adicionar(etiqueta, alvo) {
        await chamar('ctx.etiquetas.adicionar', { etiqueta, alvo: alvoSnake(alvo) });
      },
      async remover(etiqueta, alvo) {
        await chamar('ctx.etiquetas.remover', { etiqueta, alvo: alvoSnake(alvo) });
      },
    },

    funil: {
      listar: async () => camelizar<Funil[]>(await chamar('ctx.funil.listar', {})),
      posicao: async (funil, alvo) =>
        camelizar<PosicaoFunil | null>(await chamar('ctx.funil.posicao', { funil, alvo: alvoSnake(alvo) })),
      mover: async (funil, etapa, alvo) =>
        camelizar<PosicaoFunil>(await chamar('ctx.funil.mover', { funil, etapa, alvo: alvoSnake(alvo) })),
      async remover(funil, alvo) {
        await chamar('ctx.funil.remover', { funil, alvo: alvoSnake(alvo) });
      },
    },

    leads: {
      atual: async () => camelizar<Lead | null>(await chamar('ctx.leads.atual', {})),
      obter: async (id) => camelizar<Lead | null>(await chamar('ctx.leads.obter', { id })),
      buscarPorTelefone: async (telefone) =>
        camelizar<Lead | null>(await chamar('ctx.leads.buscar_telefone', { telefone })),
      atualizar: async (dados, alvo) =>
        camelizar<Lead>(
          await chamar('ctx.leads.atualizar', { nome: dados.nome, campos: dados.campos, alvo: alvoSnake(alvo) }),
        ),
    },

    memoria: {
      async obter<T extends Json = Json>(chave: string, opcoes?: { escopo?: 'global' | 'contato' }) {
        const r = await chamar('ctx.memoria.obter', { chave, escopo: opcoes?.escopo ?? 'global' });
        return ((r as { valor?: unknown } | null)?.valor ?? null) as T | null;
      },
      async definir(chave, valor, opcoes) {
        await chamar('ctx.memoria.definir', { chave, escopo: opcoes?.escopo ?? 'global', valor });
      },
      async remover(chave, opcoes) {
        const r = await chamar('ctx.memoria.remover', { chave, escopo: opcoes?.escopo ?? 'global' });
        return Boolean(r?.removida);
      },
      async listar(opcoes) {
        const r = await chamar('ctx.memoria.listar', { escopo: opcoes?.escopo ?? 'global', prefixo: opcoes?.prefixo });
        return r as { chave: string; valor: Json }[];
      },
    },

    ia: {
      async gerar(pedido) {
        const r = await chamar('ctx.ia.gerar', {
          prompt: pedido.prompt,
          mensagens: pedido.mensagens?.map((m) => ({ papel: m.papel, texto: m.texto })),
          ...opcoesIA(pedido),
        });
        return camelizar<RespostaIA>(r);
      },
      async classificar<C extends string>(
        texto: string,
        categorias: readonly C[] | Record<C, string>,
        opcoes?: OpcoesIA & { instrucoes?: string },
      ) {
        const mapa: Record<string, string | null> = {};
        if (Array.isArray(categorias)) for (const c of categorias as readonly C[]) mapa[c] = null;
        else Object.assign(mapa, categorias);
        const r = await chamar('ctx.ia.classificar', {
          texto,
          categorias: mapa,
          instrucoes: opcoes?.instrucoes,
          ...opcoesIA(opcoes),
        });
        return { categoria: r.categoria as C, tokens: r.tokens as TokensIA };
      },
      async extrair<T = Json>(texto: string, esquema: Record<string, Json>, opcoes?: OpcoesIA & { instrucoes?: string }) {
        const r = await chamar('ctx.ia.extrair', { texto, esquema, instrucoes: opcoes?.instrucoes, ...opcoesIA(opcoes) });
        return { dados: r.dados as T, tokens: r.tokens as TokensIA };
      },
    },

    http: criarHttp({
      execucaoId,
      permissoes: op.permissoes,
      fetchInterno: op.fetchInterno,
      sinal: controlador.signal,
      notificar: (p) => {
        if (!encerrada) op.notificar('http', p);
      },
      garantirAtiva,
    }),

    log: Object.freeze({ debug: log('debug'), info: log('info'), aviso: log('aviso'), erro: log('erro') }),

    segredos: Object.freeze({
      obter(nome: string): string {
        const v = Object.hasOwn(op.segredos, nome) ? op.segredos[nome] : undefined;
        if (v === undefined) {
          throw new ErroSegredo(
            `Segredo '${nome}' indisponível: declare-o em automacao.json › segredos e defina o valor em Ajustes → IA.`,
          );
        }
        return v;
      },
      tem: (nome: string) => Object.hasOwn(op.segredos, nome),
    }),

    humano: {
      async transferir(opcoes) {
        await chamar('ctx.humano.transferir', {
          motivo: opcoes?.motivo,
          mensagem: opcoes?.mensagem,
          duracao_min: opcoes?.duracaoMin,
        });
      },
    },

    async agendar(opcoes: OpcoesAgendar) {
      const temEm = opcoes?.em !== undefined;
      const temDaqui = opcoes?.daquiSegundos !== undefined;
      if (temEm === temDaqui) {
        throw new ErroValidacao('Informe exatamente um de "em" ou "daquiSegundos".', { campos: { em: 'obrigatório' } });
      }
      let em: string;
      if (temDaqui) {
        const s = Number(opcoes.daquiSegundos);
        if (!Number.isFinite(s)) throw new ErroValidacao('"daquiSegundos" precisa ser um número.', { campos: { daquiSegundos: 'inválido' } });
        em = new Date(agora() + s * 1000).toISOString();
      } else {
        const d = opcoes.em instanceof Date ? opcoes.em : new Date(String(opcoes.em));
        if (Number.isNaN(d.getTime())) throw new ErroValidacao('Data de "em" inválida.', { campos: { em: 'inválido' } });
        em = d.toISOString();
      }
      return chamar('ctx.agendar', {
        em,
        dados: opcoes.dados,
        na_conversa: opcoes.naConversa ?? conversa !== null,
      });
    },
    async cancelarAgendamento(id) {
      const r = await chamar('ctx.cancelar_agendamento', { id });
      return Boolean(r?.cancelado);
    },
    async notificar(titulo, texto) {
      await chamar('ctx.notificar', { titulo, texto });
    },
    get sinal() {
      return controlador.signal;
    },
  };

  return {
    ctx: Object.freeze(ctx),
    argumento,
    cancelar(motivo) {
      finalizar(new ErroExecucaoEncerrada(motivo ?? 'A execução foi cancelada.'));
    },
    encerrar() {
      finalizar(new ErroExecucaoEncerrada());
    },
    get encerrada() {
      return encerrada;
    },
  };
}

