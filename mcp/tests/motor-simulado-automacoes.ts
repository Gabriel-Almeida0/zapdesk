// Rotas da feature 002 (specs/002-automacoes/contracts/api-http.md) no motor simulado em memória:
// funis, automações (fluxo/chatbot/IA), projeto e compilação de automações de IA, simulador de
// chatbot, execuções, pausas, segredos e configuração. A compilação usa o esbuild de verdade (erros
// reais com arquivo:linha:coluna); a execução de código de IA é simulada (não há runner aqui).
import { createHash } from 'node:crypto';
import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

import type {
  AcaoRegistrada,
  Automacao,
  Card,
  ConfiguracaoAutomacoes,
  ConfiguracaoIA,
  ErroCompilacao,
  ErroDefinicao,
  Etapa,
  ExecucaoDetalhe,
  Funil,
  HandlerAutomacao,
  IdModeloProjeto,
  Lead,
  MovimentoFunil,
  NoChatbot,
  DefinicaoChatbot,
  Pausa,
  ResultadoCompilacao,
  SaidaSimulada,
  Segredo,
} from '@zapdesk/cliente-motor';
import { TIPOS_ACAO, TIPOS_GATILHO } from '@zapdesk/cliente-motor';
import { build } from 'esbuild';

type Json = Record<string, unknown>;

/** Erro HTTP da API (mesmo envelope do motor). */
export class ErroHttp extends Error {
  constructor(
    readonly status: number,
    readonly codigo: string,
    mensagem: string,
    readonly detalhes: Json = {},
  ) {
    super(mensagem);
  }
}

/** O que o motor simulado principal empresta a este módulo. */
export interface BaseSimulada {
  agora(): string;
  novoId(prefixo: string): string;
  leads: Lead[];
  contatos: { id: string; telefone: string | null; nome: string | null; conversa_id: string | null; etiquetas: { id: string }[] }[];
  conversas: { id: string; conta_id: string; telefone: string | null }[];
  etiquetas: { id: string }[];
  templates: { id: string }[];
  adicionarLead(telefone: string, nome?: string | null, importado_em?: string, origem?: Lead['origem']): Lead;
  normalizar(telefone: string): string | undefined;
}

interface ArquivoMem {
  conteudo: string;
  hash: string;
  atualizado_em: string;
}

interface Simulacao {
  automacao: Automacao;
  definicao: DefinicaoChatbot;
  no_atual: string | null;
  variaveis: Record<string, string>;
  tentativas: number;
  estado: 'ativa' | 'concluida' | 'humano' | 'expirada' | 'abortada';
}

export const NAO_TRATADA = Symbol('nao_tratada');

const RAIZ = join(import.meta.dirname, '..', '..');
const DTS_SDK = join(RAIZ, 'compartilhado', 'automacao-sdk', 'dist', 'index.d.ts');

const HANDLER_DO_GATILHO: Record<string, HandlerAutomacao> = {
  mensagem_recebida: 'aoReceberMensagem',
  palavra_chave: 'aoReceberMensagem',
  agendamento: 'aoAgendar',
  lead_importado: 'aoEvento',
  etiqueta: 'aoEvento',
  entrou_etapa: 'aoEvento',
  disparo_respondeu: 'aoEvento',
  sem_resposta: 'aoEvento',
};

const hash = (texto: string) => createHash('sha256').update(texto).digest('hex').slice(0, 16);

const normalizarTexto = (t: string) =>
  t
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/^[^\p{L}\p{N}]+|[^\p{L}\p{N}]+$/gu, '');

function modeloArquivos(modelo: IdModeloProjeto, nome: string): Record<string, string> {
  const manifesto = (gatilhos: unknown[], permissoes: string[]) =>
    JSON.stringify(
      {
        $schema: './.zapdesk/automacao.schema.json',
        versao_manifesto: 1,
        nome,
        entrada: 'index.ts',
        gatilhos,
        permissoes,
        segredos: [],
        contas: 'todas',
        incluir_grupos: false,
        prioridade: 100,
        conta_envio: null,
        limites: { tempo_s: 60, memoria_mb: 256, anti_loop: null },
        ia: { modelo: null },
      },
      null,
      2,
    );
  if (modelo === 'responder_historico') {
    return {
      'automacao.json': manifesto([{ tipo: 'mensagem_recebida' }], ['ler_conversas', 'enviar', 'ia']),
      'index.ts':
        "import { definirAutomacao } from '@zapdesk/automacao';\nimport prompt from './prompt.md';\n\n" +
        'export default definirAutomacao({\n  async aoReceberMensagem(ctx, msg) {\n    if (!msg.texto) return;\n' +
        '    const historico = await ctx.conversa!.historicoParaIA({ limite: 20 });\n' +
        '    const resposta = await ctx.ia.gerar({ sistema: prompt, mensagens: historico });\n' +
        '    await ctx.responder(resposta.texto);\n  },\n});\n',
      'prompt.md': 'Responda em português, curto. Nunca invente preços, prazos ou promessas.\n',
    };
  }
  return {
    'automacao.json': manifesto([{ tipo: 'manual' }], []),
    'index.ts':
      "import { definirAutomacao } from '@zapdesk/automacao';\n\nexport default definirAutomacao({\n" +
      '  async aoExecutar(ctx, entrada) {\n    ctx.log.info(\'entrada\', entrada);\n    return { ok: true };\n  },\n});\n',
  };
}

export class AutomacoesSimuladas {
  funis: Funil[] = [];
  cards: Card[] = [];
  historico: MovimentoFunil[] = [];
  automacoes: Automacao[] = [];
  arquivos = new Map<string, Map<string, ArquivoMem>>();
  execucoes: ExecucaoDetalhe[] = [];
  pausas: Pausa[] = [];
  segredos: Segredo[] = [{ nome: 'ANTHROPIC_API_KEY', reservado: true, usado_por: [] }];
  /** Valores só para provar que nada os expõe (o motor real também nunca os devolve). */
  valoresSegredos: Record<string, string> = { ANTHROPIC_API_KEY: 'sk-ant-segredo-que-nunca-sai' };
  configuracao: ConfiguracaoAutomacoes = {
    anti_loop_mensagens: 10,
    anti_loop_janela_min: 10,
    pausa_anti_loop_min: 60,
    pausa_humana_min: 30,
    primeiros_contatos_hora: 20,
    tempo_ia_s: 60,
    memoria_ia_mb: 256,
    processos_ia_max: 4,
    ociosidade_ia_min: 5,
    pausa_geral: false,
  };
  configuracaoIA: ConfiguracaoIA = {
    modelo_padrao: 'claude-sonnet-5',
    modelos: [
      { id: 'claude-sonnet-5', nome: 'Claude Sonnet 5', aviso: null },
      { id: 'claude-haiku-4-5', nome: 'Claude Haiku 4.5', aviso: null },
    ],
    chave_configurada: true,
  };
  /** Mensagens "enviadas" de verdade (o teste confere que simulações não enviam nada). */
  enviadas: { automacao_id: string; texto: string }[] = [];
  private simulacoes = new Map<string, Simulacao>();

  constructor(private readonly base: BaseSimulada) {}

  // -------------------------------------------------------------------------
  // Utilidades
  // -------------------------------------------------------------------------

  private achar<T>(lista: T[], pred: (x: T) => boolean, rotulo: string): T {
    const item = lista.find(pred);
    if (!item) throw new ErroHttp(404, 'nao_encontrado', `${rotulo} não encontrado(a).`);
    return item;
  }

  private funil(id: string | undefined): Funil {
    return this.achar(this.funis, (f) => f.id === id, 'Funil');
  }

  private automacao(id: string | undefined): Automacao {
    return this.achar(this.automacoes, (a) => a.id === id, 'Automação');
  }

  private recontar(funil: Funil): void {
    for (const etapa of funil.etapas) etapa.total_cards = this.cards.filter((c) => c.etapa_id === etapa.id).length;
    funil.total_cards = this.cards.filter((c) => c.funil_id === funil.id).length;
    funil.etapas.sort((a, b) => a.ordem - b.ordem);
  }

  private novaEtapa(funil: Funil, nome: string, cor = '#8696A0'): Etapa {
    if (funil.etapas.some((e) => e.nome.toLowerCase() === nome.toLowerCase())) {
      throw new ErroHttp(409, 'conflito', 'Já existe uma etapa com esse nome neste funil.');
    }
    const etapa: Etapa = { id: this.base.novoId('etp'), funil_id: funil.id, nome, cor, ordem: funil.etapas.length, total_cards: 0 };
    funil.etapas.push(etapa);
    return etapa;
  }

  private novaExecucao(a: Automacao, dados: Partial<ExecucaoDetalhe>): ExecucaoDetalhe {
    const agora = this.base.agora();
    const exec: ExecucaoDetalhe = {
      id: this.base.novoId('exe'),
      automacao_id: a.id,
      automacao_nome: a.nome,
      tipo_automacao: a.tipo,
      automacao_versao: a.versao,
      gatilho: { tipo: 'manual', dados: {} },
      origem: 'gatilho',
      origem_execucao_id: null,
      conta_id: null,
      conversa_id: null,
      contato_id: null,
      lead_id: null,
      estado: 'ok',
      simulacao: false,
      motivo: null,
      erro: null,
      acoes: [],
      tokens: { entrada: 0, saida: 0, por_modelo: {} },
      retorno: null,
      retomar_em: null,
      iniciada_em: agora,
      finalizada_em: agora,
      duracao_ms: 12,
      log: '',
      log_truncado: false,
      erro_stack: null,
      variaveis: {},
      ...dados,
    };
    this.execucoes.unshift(exec);
    return exec;
  }

  private resumoExecucao(e: ExecucaoDetalhe): Json {
    const { log: _l, log_truncado: _t, erro_stack: _s, variaveis: _v, ...resto } = e;
    return resto;
  }

  /** Estrutura do fluxo/chatbot → erros; referências inexistentes → avisos. */
  private validarDefinicao(b: Json): { erros: ErroDefinicao[]; avisos: ErroDefinicao[] } {
    const erros: ErroDefinicao[] = [];
    const avisos: ErroDefinicao[] = [];
    const erro = (caminho: string, mensagem: string, extra: Partial<ErroDefinicao> = {}) =>
      erros.push({ caminho, no_id: null, acao_id: null, mensagem, ...extra });
    const nome = b['nome'];
    if (typeof nome !== 'string' || nome.length < 1 || nome.length > 80) erro('nome', 'Informe um nome de 1 a 80 caracteres.');
    const gatilhos = b['gatilhos'];
    if (!Array.isArray(gatilhos)) erro('gatilhos', 'Informe a lista de gatilhos.');
    else
      gatilhos.forEach((g: Json, i) => {
        if (!TIPOS_GATILHO.includes(g['tipo'] as never)) erro(`gatilhos[${i}].tipo`, `Tipo de gatilho desconhecido: ${String(g['tipo'])}.`);
        if (g['tipo'] === 'palavra_chave' && (!Array.isArray(g['palavras']) || (g['palavras'] as unknown[]).length === 0)) {
          erro(`gatilhos[${i}].palavras`, 'Informe pelo menos uma palavra.');
        }
      });
    const def = b['definicao'] as Json | undefined;
    const referencia = (caminho: string, valor: unknown, lista: { id: string }[], rotulo: string, extra: Partial<ErroDefinicao> = {}) => {
      if (typeof valor === 'string' && !lista.some((x) => x.id === valor)) {
        avisos.push({ caminho, no_id: null, acao_id: null, mensagem: `${rotulo} ${valor} não existe.`, ...extra });
      }
    };
    const etapas = this.funis.flatMap((f) => f.etapas);
    const conferirAcao = (acao: Json, caminho: string, extra: Partial<ErroDefinicao>) => {
      if (!TIPOS_ACAO.includes(acao['tipo'] as never)) erro(`${caminho}.tipo`, `Tipo de ação desconhecido: ${String(acao['tipo'])}.`, extra);
      if (acao['tipo'] === 'enviar_texto' && (typeof acao['texto'] !== 'string' || acao['texto'] === '')) {
        erro(`${caminho}.texto`, 'Escreva o texto (1–4096 caracteres).', extra);
      }
      referencia(`${caminho}.etiqueta_id`, acao['etiqueta_id'], this.base.etiquetas, 'Etiqueta', extra);
      referencia(`${caminho}.funil_id`, acao['funil_id'], this.funis, 'Funil', extra);
      referencia(`${caminho}.etapa_id`, acao['etapa_id'], etapas, 'Etapa', extra);
      referencia(`${caminho}.template_id`, acao['template_id'], this.base.templates, 'Template', extra);
    };
    if (!def || typeof def !== 'object') {
      erro('definicao', 'Informe a definição.');
    } else if (b['tipo'] === 'fluxo') {
      const acoes = def['acoes'];
      if (!Array.isArray(acoes) || acoes.length === 0 || acoes.length > 50) erro('definicao.acoes', 'Informe de 1 a 50 ações.');
      else acoes.forEach((a: Json, i) => conferirAcao(a, `definicao.acoes[${i}]`, { acao_id: (a['id'] as string | undefined) ?? null }));
    } else if (b['tipo'] === 'chatbot') {
      const nos = def['nos'] as Json[] | undefined;
      if (!Array.isArray(nos) || nos.length < 2 || nos.length > 200) {
        erro('definicao.nos', 'O chatbot precisa de 2 a 200 nós.');
      } else {
        const ids = new Set(nos.map((n) => n['id'] as string));
        if (nos.filter((n) => n['tipo'] === 'inicio').length !== 1) erro('definicao.nos', 'O chatbot precisa de exatamente um nó "inicio".');
        nos.forEach((n, i) => {
          for (const campo of ['proximo', 'senao', 'em_erro', 'ao_esgotar']) {
            const alvo = n[campo];
            if (typeof alvo === 'string' && !ids.has(alvo)) {
              erro(`definicao.nos[${i}].${campo}`, `O nó "${alvo}" não existe.`, { no_id: n['id'] as string });
            }
          }
          if (n['tipo'] === 'acao' && n['acao']) {
            const acao = n['acao'] as Json;
            if (acao['tipo'] === 'aguardar' || acao['tipo'] === 'iniciar_chatbot') {
              erro(`definicao.nos[${i}].acao.tipo`, `A ação ${String(acao['tipo'])} não é permitida em chatbot.`, { no_id: n['id'] as string });
            } else conferirAcao(acao, `definicao.nos[${i}].acao`, { no_id: n['id'] as string });
          }
        });
      }
    }
    return { erros, avisos };
  }

  // -------------------------------------------------------------------------
  // Compilação (esbuild em memória)
  // -------------------------------------------------------------------------

  async compilar(a: Automacao): Promise<ResultadoCompilacao> {
    const arquivos = this.arquivos.get(a.id) ?? new Map<string, ArquivoMem>();
    const erros: ErroCompilacao[] = [];
    const inicio = Date.now();
    let manifesto: Json = {};
    const bruto = arquivos.get('automacao.json')?.conteudo;
    if (bruto === undefined) {
      erros.push({ arquivo: 'automacao.json', linha: 1, coluna: 1, mensagem: 'automacao.json não encontrado.', tipo: 'manifesto' });
    } else {
      try {
        manifesto = JSON.parse(bruto) as Json;
      } catch (e) {
        erros.push({ arquivo: 'automacao.json', linha: 1, coluna: 1, mensagem: `JSON inválido: ${(e as Error).message}`, tipo: 'manifesto' });
      }
    }
    const entrada = (manifesto['entrada'] as string | undefined) ?? 'index.ts';
    let saida = '';
    if (erros.length === 0) {
      try {
        const r = await build({
          entryPoints: [entrada],
          bundle: true,
          write: false,
          format: 'esm',
          platform: 'node',
          logLevel: 'silent',
          external: ['@zapdesk/automacao'],
          plugins: [
            {
              name: 'memoria',
              setup(b) {
                b.onResolve({ filter: /.*/ }, (args) => {
                  if (args.path === '@zapdesk/automacao') return { path: args.path, external: true };
                  if (args.kind === 'entry-point') return { path: args.path, namespace: 'mem' };
                  if (!args.path.startsWith('.')) return { errors: [{ text: `Módulo não permitido: ${args.path}` }] };
                  const baseDir = args.importer.includes('/') ? args.importer.slice(0, args.importer.lastIndexOf('/') + 1) : '';
                  const partes: string[] = [];
                  for (const p of (baseDir + args.path).split('/')) {
                    if (p === '..') partes.pop();
                    else if (p !== '.' && p !== '') partes.push(p);
                  }
                  let caminho = partes.join('/');
                  if (!arquivos.has(caminho) && arquivos.has(`${caminho}.ts`)) caminho = `${caminho}.ts`;
                  return { path: caminho, namespace: 'mem' };
                });
                b.onLoad({ filter: /.*/, namespace: 'mem' }, (args) => {
                  const arquivo = arquivos.get(args.path);
                  if (!arquivo) return { errors: [{ text: `Arquivo não encontrado: ${args.path}` }] };
                  const ext = args.path.slice(args.path.lastIndexOf('.'));
                  const loader = ext === '.json' ? 'json' : ext === '.md' || ext === '.txt' ? 'text' : 'ts';
                  return { contents: arquivo.conteudo, loader };
                });
              },
            },
          ],
        });
        saida = r.outputFiles[0]?.text ?? '';
      } catch (e) {
        const falhas = (e as { errors?: { text: string; location: { file: string; line: number; column: number } | null }[] }).errors ?? [];
        for (const f of falhas) {
          erros.push({
            arquivo: f.location?.file.replace(/^mem:/, '') ?? entrada,
            linha: f.location?.line ?? 1,
            coluna: (f.location?.column ?? 0) + 1,
            mensagem: f.text,
            tipo: /não encontrado|Could not resolve|não permitido/.test(f.text) ? 'importacao' : 'sintaxe',
          });
        }
      }
    }
    const handlers = (['aoReceberMensagem', 'aoAgendar', 'aoExecutar', 'aoEvento'] as const).filter((h) =>
      new RegExp(`\\b${h}\\b`).test(saida),
    );
    if (erros.length === 0) {
      for (const g of (manifesto['gatilhos'] as Json[] | undefined) ?? []) {
        const exigido = HANDLER_DO_GATILHO[g['tipo'] as string];
        if (exigido && !handlers.includes(exigido)) {
          erros.push({
            arquivo: 'automacao.json',
            linha: 1,
            coluna: 1,
            mensagem: `O gatilho ${String(g['tipo'])} exige o handler ${exigido}, que não foi exportado.`,
            tipo: 'manifesto',
          });
        }
      }
    }
    const ok = erros.length === 0;
    const resultado: ResultadoCompilacao = {
      ok,
      erros,
      avisos: [],
      hash: ok ? hash(saida) : null,
      handlers,
      duracao_ms: Date.now() - inicio,
    };
    if (ok) {
      a.nome = (manifesto['nome'] as string | undefined) ?? a.nome;
      a.gatilhos = ((manifesto['gatilhos'] as Automacao['gatilhos'] | undefined) ?? []);
    }
    if (a.ia) {
      a.ia.compilacao_ok = ok;
      a.ia.erros_compilacao = erros;
      a.ia.hash_compilado = ok ? resultado.hash : a.ia.hash_compilado;
      a.ia.compilado_em = this.base.agora();
      if (ok) a.ia.permissoes = (manifesto['permissoes'] as never) ?? [];
    }
    return resultado;
  }

  // -------------------------------------------------------------------------
  // Gatilhos simulados (para os testes de ciclo)
  // -------------------------------------------------------------------------

  /** Simula o motor recebendo uma mensagem: executa as automações ativas com gatilho de mensagem. */
  receberMensagem(conversaId: string, texto: string): ExecucaoDetalhe[] {
    const criadas: ExecucaoDetalhe[] = [];
    for (const a of this.automacoes.filter((x) => x.ativa)) {
      const casa = a.gatilhos.some((g) => {
        if (g.tipo === 'mensagem_recebida') return !g.contem || normalizarTexto(texto).includes(normalizarTexto(g.contem));
        if (g.tipo === 'palavra_chave') return g.palavras.some((p) => normalizarTexto(texto).split(' ').includes(normalizarTexto(p)));
        return false;
      });
      if (!casa) continue;
      const resposta = a.tipo === 'ia' ? `Resposta da IA para: ${texto}` : 'ok';
      this.enviadas.push({ automacao_id: a.id, texto: resposta });
      criadas.push(
        this.novaExecucao(a, {
          gatilho: { tipo: 'mensagem_recebida', dados: { mensagem_id: this.base.novoId('msg') } },
          conversa_id: conversaId,
          acoes: [{ tipo: 'responder', alvo: conversaId, resultado: 'ok', detalhe: resposta, em: this.base.agora() }],
          log: '[info] respondeu\n',
        }),
      );
    }
    return criadas;
  }

  // -------------------------------------------------------------------------
  // Simulador de chatbot (interpretador mínimo)
  // -------------------------------------------------------------------------

  private no(s: Simulacao, id: string | null | undefined): NoChatbot | undefined {
    return s.definicao.nos.find((n) => n.id === id);
  }

  private avancar(s: Simulacao, desde: string | null, saidas: SaidaSimulada[], acoes: AcaoRegistrada[]): void {
    let atual = desde;
    for (let passos = 0; atual && passos < 200; passos++) {
      const no = this.no(s, atual);
      if (!no) break;
      const sub = (t: string) => t.replace(/\{(\w+)\}/g, (m, v: string) => s.variaveis[v] ?? (v === 'nome' ? 'Contato de teste' : m));
      switch (no.tipo) {
        case 'inicio':
          atual = no.proximo;
          continue;
        case 'mensagem':
          saidas.push({ tipo: 'mensagem', texto: sub(no.texto), no_id: no.id });
          atual = no.proximo;
          continue;
        case 'menu': {
          const numeros = no.mostrar_numeros ? `\n${no.opcoes.map((o, i) => `${i + 1} - ${o.rotulo}`).join('\n')}` : '';
          saidas.push({ tipo: 'mensagem', texto: sub(no.texto) + numeros, no_id: no.id });
          s.no_atual = no.id;
          return;
        }
        case 'pergunta':
          saidas.push({ tipo: 'mensagem', texto: sub(no.texto), no_id: no.id });
          s.no_atual = no.id;
          return;
        case 'condicao': {
          const ramo = no.ramos.find((r) =>
            r.condicoes.regras.every((regra) => {
              if (regra.tipo !== 'variavel') return true;
              const valor = s.variaveis[regra.variavel];
              if (regra.operador === 'existe') return valor !== undefined;
              if (regra.operador === 'contem') return (valor ?? '').includes(regra.valor ?? '');
              if (regra.operador === 'igual') return valor === regra.valor;
              return false;
            }),
          );
          atual = ramo ? ramo.proximo : no.senao;
          continue;
        }
        case 'acao':
          saidas.push({ tipo: 'acao', texto: `Ação ${no.acao.tipo}`, no_id: no.id });
          acoes.push({ tipo: no.acao.tipo, alvo: null, resultado: 'simulada', detalhe: null, em: this.base.agora() });
          atual = no.proximo;
          continue;
        case 'ia':
          saidas.push({ tipo: 'mensagem', texto: 'Resposta simulada da IA.', no_id: no.id });
          atual = no.proximo;
          continue;
        case 'humano':
          if (no.mensagem) saidas.push({ tipo: 'mensagem', texto: sub(no.mensagem), no_id: no.id });
          saidas.push({ tipo: 'aviso', texto: 'Conversa transferida para atendimento humano.', no_id: no.id });
          s.no_atual = no.id;
          s.estado = 'humano';
          return;
        case 'fim':
          if (no.mensagem) saidas.push({ tipo: 'mensagem', texto: sub(no.mensagem), no_id: no.id });
          s.no_atual = no.id;
          s.estado = 'concluida';
          return;
      }
    }
  }

  private responderSimulacao(s: Simulacao, texto: string): { saidas: SaidaSimulada[]; acoes: AcaoRegistrada[] } {
    const saidas: SaidaSimulada[] = [];
    const acoes: AcaoRegistrada[] = [];
    const no = this.no(s, s.no_atual);
    const esgotar = (aoEsgotar: string | null | undefined) => {
      s.tentativas += 1;
      if (s.tentativas >= s.definicao.max_tentativas) {
        s.tentativas = 0;
        if (aoEsgotar) this.avancar(s, aoEsgotar, saidas, acoes);
        else {
          saidas.push({ tipo: 'aviso', texto: 'Tentativas esgotadas: transferido para humano.', no_id: no?.id ?? null });
          s.estado = 'humano';
        }
      } else {
        saidas.push({ tipo: 'mensagem', texto: s.definicao.nao_entendi, no_id: no?.id ?? null });
      }
    };
    if (no?.tipo === 'menu') {
      const n = normalizarTexto(texto);
      const indice = Number.parseInt(n, 10);
      const opcao =
        (Number.isInteger(indice) ? no.opcoes[indice - 1] : undefined) ??
        no.opcoes.find((o) => normalizarTexto(o.rotulo) === n || o.valores.some((v) => normalizarTexto(v) === n));
      if (opcao) {
        s.tentativas = 0;
        this.avancar(s, opcao.proximo, saidas, acoes);
      } else esgotar(no.ao_esgotar);
    } else if (no?.tipo === 'pergunta') {
      const v = no.validacao;
      const valido =
        !v || v.tipo === 'nenhuma'
          ? true
          : v.tipo === 'email'
            ? /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(texto.trim())
            : v.tipo === 'numero'
              ? !Number.isNaN(Number(texto.replace(',', '.')))
              : v.tipo === 'regex'
                ? new RegExp(v.padrao ?? '').test(texto)
                : true;
      if (valido) {
        s.variaveis[no.variavel] = texto.trim();
        s.tentativas = 0;
        this.avancar(s, no.proximo, saidas, acoes);
      } else {
        s.tentativas += 1;
        saidas.push({ tipo: 'mensagem', texto: v?.mensagem_erro ?? s.definicao.nao_entendi, no_id: no.id });
      }
    }
    return { saidas, acoes };
  }

  // -------------------------------------------------------------------------
  // Rotas
  // -------------------------------------------------------------------------

  async rotear(m: string, c: string, q: URLSearchParams, b: Json): Promise<unknown> {
    let x: RegExpExecArray | null;

    // ---- Funis
    if (c === '/funis' && m === 'GET') {
      this.funis.forEach((f) => this.recontar(f));
      return this.funis;
    }
    if (c === '/funis' && m === 'POST') {
      const nome = String(b['nome'] ?? '').trim();
      if (!nome || nome.length > 60) throw new ErroHttp(422, 'validacao', 'Informe um nome de 1 a 60 caracteres.', { campos: { nome: 'inválido' } });
      if (this.funis.some((f) => f.nome.toLowerCase() === nome.toLowerCase())) throw new ErroHttp(409, 'conflito', 'Já existe um funil com esse nome.');
      const agora = this.base.agora();
      const funil: Funil = { id: this.base.novoId('fun'), nome, ordem: this.funis.length, etapas: [], total_cards: 0, criado_em: agora, atualizado_em: agora };
      for (const e of (b['etapas'] as { nome: string; cor?: string }[] | undefined) ?? []) this.novaEtapa(funil, e.nome, e.cor);
      this.funis.push(funil);
      return funil;
    }
    if ((x = /^\/funis\/([^/]+)$/.exec(c))) {
      const funil = this.funil(x[1]);
      if (m === 'GET') {
        this.recontar(funil);
        return funil;
      }
      if (m === 'PATCH') {
        if (b['nome'] !== undefined) {
          const nome = String(b['nome']);
          if (this.funis.some((f) => f.id !== funil.id && f.nome.toLowerCase() === nome.toLowerCase())) {
            throw new ErroHttp(409, 'conflito', 'Já existe um funil com esse nome.');
          }
          funil.nome = nome;
        }
        funil.atualizado_em = this.base.agora();
        return funil;
      }
      if (m === 'DELETE') {
        this.funis = this.funis.filter((f) => f.id !== funil.id);
        this.cards = this.cards.filter((k) => k.funil_id !== funil.id);
        return undefined;
      }
    }
    if ((x = /^\/funis\/([^/]+)\/etapas$/.exec(c)) && m === 'POST') {
      const funil = this.funil(x[1]);
      if (funil.etapas.length >= 30) throw new ErroHttp(422, 'validacao', 'Máximo de 30 etapas.');
      return this.novaEtapa(funil, String(b['nome']), b['cor'] as string | undefined);
    }
    if ((x = /^\/funis\/([^/]+)\/etapas\/ordem$/.exec(c)) && m === 'PUT') {
      const funil = this.funil(x[1]);
      const ids = b['etapa_ids'] as string[];
      if (ids.length !== funil.etapas.length || !funil.etapas.every((e) => ids.includes(e.id))) {
        throw new ErroHttp(422, 'validacao', 'Informe todas as etapas do funil.');
      }
      ids.forEach((id, i) => {
        const etapa = funil.etapas.find((e) => e.id === id);
        if (etapa) etapa.ordem = i;
      });
      this.recontar(funil);
      return funil;
    }
    if ((x = /^\/etapas\/([^/]+)$/.exec(c))) {
      const funil = this.achar(this.funis, (f) => f.etapas.some((e) => e.id === x![1]), 'Etapa');
      const etapa = funil.etapas.find((e) => e.id === x![1]) as Etapa;
      if (m === 'PATCH') {
        if (b['nome'] !== undefined) {
          if (funil.etapas.some((e) => e.id !== etapa.id && e.nome.toLowerCase() === String(b['nome']).toLowerCase())) {
            throw new ErroHttp(409, 'conflito', 'Já existe uma etapa com esse nome neste funil.');
          }
          etapa.nome = String(b['nome']);
        }
        if (b['cor'] !== undefined) etapa.cor = String(b['cor']);
        return etapa;
      }
      if (m === 'DELETE') {
        const nela = this.cards.filter((k) => k.etapa_id === etapa.id);
        const destino = q.get('destino_etapa_id');
        if (nela.length > 0 && !destino && q.get('remover_cards') !== 'true') {
          throw new ErroHttp(422, 'validacao', 'Escolha para onde mover os cards.');
        }
        if (destino) nela.forEach((k) => (k.etapa_id = destino));
        else this.cards = this.cards.filter((k) => k.etapa_id !== etapa.id);
        funil.etapas = funil.etapas.filter((e) => e.id !== etapa.id);
        funil.etapas.forEach((e, i) => (e.ordem = i));
        return undefined;
      }
    }
    if ((x = /^\/funis\/([^/]+)\/cards$/.exec(c))) {
      const funil = this.funil(x[1]);
      if (m === 'GET') {
        let lista = this.cards.filter((k) => k.funil_id === funil.id).reverse();
        if (q.get('etapa_id')) lista = lista.filter((k) => k.etapa_id === q.get('etapa_id'));
        const busca = q.get('busca')?.toLowerCase();
        if (busca) lista = lista.filter((k) => k.lead.telefone.includes(busca) || (k.lead.nome ?? '').toLowerCase().includes(busca));
        return paginar(lista, q);
      }
      if (m === 'PUT') {
        const etapa = funil.etapas.find((e) => e.id === b['etapa_id']);
        if (!etapa) throw new ErroHttp(422, 'validacao', 'A etapa não pertence a este funil.');
        let lead: Lead | undefined;
        if (b['lead_id']) lead = this.achar(this.base.leads, (l) => l.id === b['lead_id'], 'Lead');
        else if (b['contato_id']) {
          const contato = this.achar(this.base.contatos, (k) => k.id === b['contato_id'], 'Contato');
          lead = this.base.leads.find((l) => l.telefone === contato.telefone) ?? this.base.adicionarLead(contato.telefone ?? '', contato.nome, undefined, 'contatos');
        } else if (b['telefone']) {
          const e164 = this.base.normalizar(String(b['telefone']));
          if (!e164) throw new ErroHttp(422, 'validacao', 'Telefone inválido.', { campos: { telefone: 'Telefone inválido.' } });
          lead = this.base.leads.find((l) => l.telefone === e164) ?? this.base.adicionarLead(e164, null, undefined, b['origem'] === 'mcp' ? 'mcp' : 'contatos');
        } else throw new ErroHttp(422, 'validacao', 'Informe lead_id, contato_id ou telefone.');
        const existente = this.cards.find((k) => k.funil_id === funil.id && k.lead_id === lead!.id);
        const origemEtapa = existente ? funil.etapas.find((e) => e.id === existente.etapa_id) : undefined;
        if (existente && existente.etapa_id === etapa.id) return existente;
        const card: Card = existente ?? {
          lead_id: lead.id,
          funil_id: funil.id,
          etapa_id: etapa.id,
          desde: '',
          lead: { id: lead.id, telefone: lead.telefone, nome: lead.nome, campos: lead.campos },
          etiquetas: [],
          conversa_id: null,
          conta_id: null,
        };
        card.etapa_id = etapa.id;
        card.desde = this.base.agora();
        if (!existente) this.cards.push(card);
        this.historico.unshift({
          id: this.base.novoId('mov'),
          lead_id: lead.id,
          funil_id: funil.id,
          etapa_origem_id: origemEtapa?.id ?? null,
          etapa_origem_nome: origemEtapa?.nome ?? null,
          etapa_destino_id: etapa.id,
          etapa_destino_nome: etapa.nome,
          origem: (b['origem'] as 'app' | 'mcp' | undefined) ?? 'app',
          automacao_id: null,
          execucao_id: null,
          em: card.desde,
        });
        return card;
      }
    }
    if ((x = /^\/funis\/([^/]+)\/cards\/([^/]+)$/.exec(c)) && m === 'DELETE') {
      const funil = this.funil(x[1]);
      const card = this.cards.find((k) => k.funil_id === funil.id && k.lead_id === x![2]);
      if (card) {
        const etapa = funil.etapas.find((e) => e.id === card.etapa_id);
        this.cards = this.cards.filter((k) => k !== card);
        this.historico.unshift({
          id: this.base.novoId('mov'),
          lead_id: card.lead_id,
          funil_id: funil.id,
          etapa_origem_id: etapa?.id ?? null,
          etapa_origem_nome: etapa?.nome ?? null,
          etapa_destino_id: null,
          etapa_destino_nome: null,
          origem: (q.get('origem') as 'app' | 'mcp' | null) ?? 'app',
          automacao_id: null,
          execucao_id: null,
          em: this.base.agora(),
        });
      }
      return undefined;
    }
    if ((x = /^\/funis\/([^/]+)\/historico$/.exec(c)) && m === 'GET') {
      const funil = this.funil(x[1]);
      let lista = this.historico.filter((h) => h.funil_id === funil.id);
      if (q.get('lead_id')) lista = lista.filter((h) => h.lead_id === q.get('lead_id'));
      return paginar(lista, q);
    }
    if ((x = /^\/leads\/([^/]+)$/.exec(c)) && m === 'PATCH') {
      const lead = this.achar(this.base.leads, (l) => l.id === x![1], 'Lead');
      if (b['nome'] !== undefined) lead.nome = b['nome'] as string | null;
      for (const [chave, valor] of Object.entries((b['campos'] as Record<string, string | null> | undefined) ?? {})) {
        if (valor === null) delete lead.campos[chave];
        else lead.campos[chave] = valor;
      }
      return lead;
    }

    // ---- Automações de IA: projeto
    if (c === '/automacoes/modelos' && m === 'GET') {
      return [
        { id: 'responder_historico', nome: 'Responder com IA usando histórico', descricao: 'Responde dúvidas com a IA.' },
        { id: 'classificar_funil', nome: 'Classificar e mover no funil', descricao: 'Classifica a conversa e move no funil.' },
        { id: 'extrair_dados', nome: 'Extrair dados para o lead', descricao: 'Extrai dados para campos do lead.' },
        { id: 'em_branco', nome: 'Em branco', descricao: 'Projeto mínimo com aoExecutar.' },
      ];
    }
    if (c === '/automacoes/sdk' && m === 'GET') {
      const tipos = existsSync(DTS_SDK) ? readFileSync(DTS_SDK, 'utf8') : "declare module '@zapdesk/automacao' { export function definirAutomacao<D>(d: D): D; }";
      return { versao: '1.0.0', tipos, esquema_manifesto: { type: 'object' } };
    }
    if (c === '/automacoes/ia' && m === 'POST') {
      const nome = String(b['nome'] ?? '');
      if (!nome || nome.length > 80) throw new ErroHttp(422, 'validacao', 'Informe um nome de 1 a 80 caracteres.');
      const agora = this.base.agora();
      const a = this.novaAutomacaoBase({ tipo: 'ia', nome, descricao: (b['descricao'] as string | undefined) ?? null }, agora);
      a.ia = {
        pasta: `/tmp/zapdesk-teste/automacoes/${a.id}`,
        permissoes: [],
        segredos: [],
        hash_compilado: null,
        compilacao_ok: false,
        erros_compilacao: [],
        compilado_em: null,
        rodando_versao_anterior: false,
      };
      const arquivos = new Map<string, ArquivoMem>();
      for (const [caminho, conteudo] of Object.entries(modeloArquivos(b['modelo'] as IdModeloProjeto, nome))) {
        arquivos.set(caminho, { conteudo, hash: hash(conteudo), atualizado_em: agora });
      }
      this.arquivos.set(a.id, arquivos);
      this.automacoes.push(a);
      await this.compilar(a);
      return a;
    }
    if ((x = /^\/automacoes\/([^/]+)\/arquivos$/.exec(c)) && m === 'GET') {
      this.automacao(x[1]);
      return [...(this.arquivos.get(x[1] as string) ?? new Map<string, ArquivoMem>()).entries()]
        .sort(([a], [b2]) => a.localeCompare(b2))
        .map(([caminho, arq]) => ({ caminho, tamanho: Buffer.byteLength(arq.conteudo), hash: arq.hash, atualizado_em: arq.atualizado_em }));
    }
    if ((x = /^\/automacoes\/([^/]+)\/arquivos\/renomear$/.exec(c)) && m === 'POST') {
      this.automacao(x[1]);
      const arquivos = this.arquivos.get(x[1] as string)!;
      const de = String(b['de']);
      const para = String(b['para']);
      const arq = arquivos.get(de);
      if (!arq) throw new ErroHttp(404, 'nao_encontrado', 'Arquivo não encontrado.');
      if (arquivos.has(para)) throw new ErroHttp(409, 'conflito', 'Já existe um arquivo com esse nome.');
      arquivos.delete(de);
      arquivos.set(para, arq);
      return { caminho: para, tamanho: Buffer.byteLength(arq.conteudo), hash: arq.hash, atualizado_em: arq.atualizado_em };
    }
    if ((x = /^\/automacoes\/([^/]+)\/arquivos\/(.+)$/.exec(c))) {
      this.automacao(x[1]);
      const arquivos = this.arquivos.get(x[1] as string)!;
      const caminho = x[2]!.split('/').map(decodeURIComponent).join('/');
      if (m === 'GET') {
        const arq = arquivos.get(caminho);
        if (!arq) throw new ErroHttp(404, 'nao_encontrado', 'Arquivo não encontrado.');
        return { caminho, conteudo: arq.conteudo, hash: arq.hash, atualizado_em: arq.atualizado_em };
      }
      if (m === 'PUT') {
        if (!/\.(ts|json|md|txt)$/.test(caminho)) {
          throw new ErroHttp(422, 'validacao', 'Extensão não permitida (use .ts, .json, .md ou .txt).', { campos: { caminho: 'extensão' } });
        }
        const atual = arquivos.get(caminho);
        if ('hash_anterior' in b) {
          const anterior = b['hash_anterior'];
          if (anterior === null && atual) throw new ErroHttp(409, 'conflito', 'O arquivo já existe.', { hash_atual: atual.hash });
          if (typeof anterior === 'string' && atual && atual.hash !== anterior) {
            throw new ErroHttp(409, 'conflito', 'O arquivo foi alterado fora do app.', { hash_atual: atual.hash });
          }
        }
        const conteudo = String(b['conteudo']);
        const arq = { conteudo, hash: hash(conteudo), atualizado_em: this.base.agora() };
        arquivos.set(caminho, arq);
        return { caminho, tamanho: Buffer.byteLength(conteudo), hash: arq.hash, atualizado_em: arq.atualizado_em };
      }
      if (m === 'DELETE') {
        if (caminho === 'automacao.json' || caminho === 'index.ts') {
          throw new ErroHttp(422, 'validacao', 'O manifesto e o arquivo de entrada não podem ser excluídos.');
        }
        if (!arquivos.delete(caminho)) throw new ErroHttp(404, 'nao_encontrado', 'Arquivo não encontrado.');
        return undefined;
      }
    }
    if ((x = /^\/automacoes\/([^/]+)\/compilar$/.exec(c)) && m === 'POST') return this.compilar(this.automacao(x[1]));

    // ---- Configuração
    if (c === '/automacoes/configuracao' && m === 'GET') return this.configuracao;
    if (c === '/ia/configuracao' && m === 'GET') return this.configuracaoIA;
    if (c === '/segredos' && m === 'GET') return this.segredos;

    // ---- Automações (todas)
    if (c === '/automacoes/validar' && m === 'POST') return this.validarDefinicao(b);
    if (c === '/automacoes' && m === 'GET') {
      let lista = [...this.automacoes].sort((a, b2) => a.prioridade - b2.prioridade);
      if (q.get('tipo')) lista = lista.filter((a) => a.tipo === q.get('tipo'));
      if (q.get('ativa')) lista = lista.filter((a) => String(a.ativa) === q.get('ativa'));
      return lista;
    }
    if (c === '/automacoes' && m === 'POST') {
      if (b['tipo'] !== 'fluxo' && b['tipo'] !== 'chatbot') throw new ErroHttp(422, 'validacao', 'Tipo deve ser fluxo ou chatbot.');
      const { erros, avisos } = this.validarDefinicao(b);
      if (erros.length > 0) throw new ErroHttp(422, 'definicao_invalida', 'A definição da automação tem erros.', { erros });
      const a = this.novaAutomacaoBase(b, this.base.agora());
      a.avisos = avisos;
      this.automacoes.push(a);
      return a;
    }
    if ((x = /^\/automacoes\/([^/]+)$/.exec(c))) {
      const a = this.automacao(x[1]);
      if (m === 'GET') return a;
      if (m === 'PATCH') {
        if (a.tipo === 'ia') throw new ErroHttp(422, 'validacao', 'Edite automacao.json.');
        const novo = { ...a, ...b } as unknown as Json;
        const { erros, avisos } = this.validarDefinicao(novo);
        if (erros.length > 0) throw new ErroHttp(422, 'definicao_invalida', 'A definição da automação tem erros.', { erros });
        const muda = ['gatilhos', 'definicao', 'limites'].some((k) => k in b);
        Object.assign(a, b);
        a.avisos = avisos;
        if (muda) a.versao += 1;
        a.atualizada_em = this.base.agora();
        return a;
      }
      if (m === 'DELETE') {
        this.automacoes = this.automacoes.filter((y) => y.id !== a.id);
        this.execucoes = this.execucoes.filter((e) => e.automacao_id !== a.id);
        this.arquivos.delete(a.id);
        return undefined;
      }
    }
    if ((x = /^\/automacoes\/([^/]+)\/(ativar|desativar)$/.exec(c)) && m === 'POST') {
      const a = this.automacao(x[1]);
      if (x[2] === 'desativar') {
        a.ativa = false;
        a.desativada_motivo = 'usuario';
        return a;
      }
      if (a.tipo === 'ia') {
        const r = await this.compilar(a);
        if (!r.ok) throw new ErroHttp(422, 'compilacao_falhou', 'A automação não compila.', { erros: r.erros });
      }
      if (a.gatilhos.length === 0) throw new ErroHttp(422, 'validacao', 'A automação precisa de pelo menos um gatilho.');
      a.ativa = true;
      a.desativada_motivo = null;
      return a;
    }
    if ((x = /^\/automacoes\/([^/]+)\/executar$/.exec(c)) && m === 'POST') {
      const a = this.automacao(x[1]);
      const retorno = a.tipo === 'ia' ? { recebido: b['entrada'] ?? null } : null;
      const exec = this.novaExecucao(a, {
        origem: (b['origem'] as 'manual_mcp' | undefined) ?? 'manual_app',
        gatilho: { tipo: 'manual', dados: { pedido_por: 'mcp' } },
        conversa_id: (b['conversa_id'] as string | undefined) ?? null,
        lead_id: (b['lead_id'] as string | undefined) ?? null,
        retorno,
        log: '[info] execução manual\n',
      });
      return this.resumoExecucao(exec);
    }
    if ((x = /^\/automacoes\/([^/]+)\/testar$/.exec(c)) && m === 'POST') {
      const a = this.automacao(x[1]);
      if (a.tipo === 'chatbot') throw new ErroHttp(422, 'validacao', 'Use o chat simulado para testar chatbots.');
      const mensagem = b['mensagem'] as { texto: string; conversa_id?: string } | undefined;
      const conversa = mensagem?.conversa_id ?? (b['alvo'] as Json | undefined)?.['conversa_id'] ?? null;
      let acoes: AcaoRegistrada[];
      let log = '';
      let retorno: unknown = null;
      if (a.tipo === 'ia') {
        const r = await this.compilar(a);
        if (!r.ok) throw new ErroHttp(422, 'compilacao_falhou', 'A automação não compila.', { erros: r.erros });
        const iaSimulada = b['ia_simulada'] === true;
        acoes = mensagem
          ? [{ tipo: 'responder', alvo: String(conversa ?? 'conversa fictícia'), resultado: 'simulada', detalhe: iaSimulada ? 'Resposta simulada da IA.' : 'Resposta da IA.', em: this.base.agora() }]
          : [];
        log = `[info] aoReceberMensagem: ${mensagem?.texto ?? '(sem mensagem)'}\n`;
        if (b['entrada'] !== undefined) retorno = { ok: true };
      } else {
        const def = a.definicao as { acoes: { tipo: string }[] };
        acoes = def.acoes.map((ac) => ({ tipo: ac.tipo, alvo: null, resultado: 'simulada' as const, detalhe: null, em: this.base.agora() }));
      }
      const exec = this.novaExecucao(a, {
        origem: 'teste',
        estado: 'simulacao',
        simulacao: true,
        gatilho: { tipo: mensagem ? 'mensagem_recebida' : 'manual', dados: {} },
        conversa_id: conversa as string | null,
        acoes,
        log,
        retorno,
      });
      return { execucao: exec };
    }
    if ((x = /^\/automacoes\/([^/]+)\/execucoes$/.exec(c)) && m === 'GET') {
      this.automacao(x[1]);
      let lista = this.execucoes.filter((e) => e.automacao_id === x![1]);
      if (q.get('estado')) lista = lista.filter((e) => e.estado === q.get('estado'));
      return paginar(lista.map((e) => this.resumoExecucao(e)), q);
    }
    if (c === '/execucoes' && m === 'GET') {
      let lista = this.execucoes;
      if (q.get('estado')) lista = lista.filter((e) => e.estado === q.get('estado'));
      if (q.get('conversa_id')) lista = lista.filter((e) => e.conversa_id === q.get('conversa_id'));
      return paginar(lista.map((e) => this.resumoExecucao(e)), q);
    }
    if ((x = /^\/execucoes\/([^/]+)$/.exec(c)) && m === 'GET') return this.achar(this.execucoes, (e) => e.id === x![1], 'Execução');

    // ---- Simulador de chatbot
    if ((x = /^\/automacoes\/([^/]+)\/simulador$/.exec(c)) && m === 'POST') {
      const a = this.automacao(x[1]);
      if (a.tipo !== 'chatbot') throw new ErroHttp(422, 'validacao', 'O simulador é só para chatbots.');
      const definicao = ((b['definicao'] as DefinicaoChatbot | undefined) ?? a.definicao) as DefinicaoChatbot;
      const s: Simulacao = { automacao: a, definicao, no_atual: null, variaveis: {}, tentativas: 0, estado: 'ativa' };
      const saidas: SaidaSimulada[] = [];
      this.avancar(s, definicao.inicio, saidas, []);
      const id = this.base.novoId('sim');
      this.simulacoes.set(id, s);
      return { simulacao_id: id, saidas, no_atual: s.no_atual, variaveis: s.variaveis, estado: s.estado };
    }
    if ((x = /^\/simulador\/([^/]+)\/mensagens$/.exec(c)) && m === 'POST') {
      const s = this.simulacoes.get(x[1] as string);
      if (!s) throw new ErroHttp(404, 'nao_encontrado', 'Simulação não encontrada.');
      if (s.estado !== 'ativa') throw new ErroHttp(409, 'transicao_invalida', 'A sessão simulada terminou.', { estado_atual: s.estado });
      const { saidas, acoes } = this.responderSimulacao(s, String(b['texto'] ?? ''));
      return { saidas, no_atual: s.no_atual, variaveis: { ...s.variaveis }, estado: s.estado, acoes };
    }
    if ((x = /^\/simulador\/([^/]+)$/.exec(c)) && m === 'DELETE') {
      this.simulacoes.delete(x[1] as string);
      return undefined;
    }

    // ---- Pausas
    if ((x = /^\/conversas\/([^/]+)\/pausa$/.exec(c))) {
      const conversaId = x[1] as string;
      this.achar(this.base.conversas, (k) => k.id === conversaId, 'Conversa');
      this.pausas = this.pausas.filter((p) => p.conversa_id !== conversaId);
      if (m === 'DELETE') return undefined;
      if (m === 'POST') {
        const duracao = b['duracao_min'] as number | null | undefined;
        const pausa: Pausa = {
          conversa_id: conversaId,
          motivo: b['motivo'] as Pausa['motivo'],
          ate: duracao ? `ate+${duracao}min` : null,
          automacao_id: null,
          criada_em: this.base.agora(),
        };
        this.pausas.push(pausa);
        return pausa;
      }
    }
    if (c === '/pausas' && m === 'GET') {
      const motivo = q.get('motivo');
      return motivo ? this.pausas.filter((p) => p.motivo === motivo) : this.pausas;
    }

    return NAO_TRATADA;
  }

  private novaAutomacaoBase(b: Json, agora: string): Automacao {
    return {
      id: this.base.novoId('aut'),
      tipo: b['tipo'] as Automacao['tipo'],
      nome: String(b['nome']),
      descricao: (b['descricao'] as string | null | undefined) ?? null,
      ativa: false,
      contas: (b['contas'] as string[] | null | undefined) ?? null,
      incluir_grupos: (b['incluir_grupos'] as boolean | undefined) ?? false,
      prioridade: (b['prioridade'] as number | undefined) ?? 100,
      conta_envio_id: (b['conta_envio_id'] as string | null | undefined) ?? null,
      gatilhos: (b['gatilhos'] as Automacao['gatilhos'] | undefined) ?? [],
      definicao: (b['definicao'] as Automacao['definicao'] | undefined) ?? null,
      limites: { anti_loop: null, tempo_s: null, memoria_mb: null, ...((b['limites'] as object | undefined) ?? {}) },
      versao: 1,
      ia: null,
      avisos: [],
      erros_seguidos: 0,
      desativada_motivo: null,
      estatisticas_24h: { ok: 0, erro: 0, abortada: 0, duracao_media_ms: null },
      sessoes_ativas: 0,
      criada_em: agora,
      atualizada_em: agora,
    };
  }
}

function paginar<T>(itens: T[], query: URLSearchParams): { itens: T[]; proximo_cursor: string | null } {
  const limite = Number(query.get('limite') ?? 50);
  const inicio = Number(query.get('cursor') ?? 0);
  const fatia = itens.slice(inicio, inicio + limite);
  return { itens: fatia, proximo_cursor: inicio + limite < itens.length ? String(inicio + limite) : null };
}
