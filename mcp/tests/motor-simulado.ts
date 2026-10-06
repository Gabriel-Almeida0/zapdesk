// Motor simulado em memória: implementa, como um `fetch`, as rotas de contracts/api-http.md usadas
// pelas ferramentas MCP (com os mesmos códigos de erro). Serve para testar cada ferramenta sem o
// motor Go; o teste de integração (integracao.test.ts) roda contra o motor real em modo falso.
import { existsSync, readFileSync } from 'node:fs';
import { basename, extname } from 'node:path';

import {
  ClienteMotor,
  type Arquivo,
  type Conta,
  type Contato,
  type Conversa,
  type Destinatario,
  type Disparo,
  type Etiqueta,
  type Lead,
  type Mensagem,
  type OrigemLead,
  type RelatorioImportacao,
  type StatusPorContato,
  type Template,
  type VariavelFaltando,
} from '@zapdesk/cliente-motor';

import { AutomacoesSimuladas, ErroHttp, NAO_TRATADA } from './motor-simulado-automacoes.js';

type Json = Record<string, unknown>;

interface Requisicao {
  metodo: string;
  caminho: string;
  query: URLSearchParams;
  corpo: Json;
}

export interface RequisicaoRegistrada {
  metodo: string;
  caminho: string;
  corpo: Json;
}

export const TOKEN_SIMULADO = 'token-de-teste-com-mais-de-32-caracteres!!';

/** Normalização simplificada (a real usa libphonenumber no motor). */
export function normalizarTelefone(bruto: string, ddi = '55'): { e164?: string; motivo?: string } {
  const valor = bruto.trim();
  if (valor === '') return { motivo: 'vazio' };
  if (/[a-zA-Z]/.test(valor)) return { motivo: 'formato_invalido' };
  let digitos = valor.replace(/\D/g, '');
  if (valor.startsWith('00')) digitos = digitos.slice(2);
  else if (!valor.startsWith('+') && (digitos.length === 10 || digitos.length === 11)) digitos = ddi + digitos;
  if (digitos.length < 12 || digitos.length > 15) return { motivo: 'numero_invalido' };
  return { e164: `+${digitos}` };
}

function variaveisDe(texto: string): string[] {
  return [...new Set([...texto.matchAll(/\{(\w+)\}/g)].map((m) => m[1] as string))];
}

function paginar<T>(itens: T[], query: URLSearchParams): { itens: T[]; proximo_cursor: string | null } {
  const limite = Number(query.get('limite') ?? 50);
  const inicio = Number(query.get('cursor') ?? 0);
  const fatia = itens.slice(inicio, inicio + limite);
  return { itens: fatia, proximo_cursor: inicio + limite < itens.length ? String(inicio + limite) : null };
}

export class MotorSimulado {
  readonly requisicoes: RequisicaoRegistrada[] = [];
  contas: Conta[] = [];
  conversas: Conversa[] = [];
  mensagens: Mensagem[] = [];
  contatos: Contato[] = [];
  etiquetas: Etiqueta[] = [];
  templates: Template[] = [];
  leads: Lead[] = [];
  arquivos: Arquivo[] = [];
  disparos: Disparo[] = [];
  destinatarios = new Map<string, Destinatario[]>();
  status: StatusPorContato[] = [];
  semWhatsApp = new Set<string>();
  reacoes: { mensagem_id: string; emoji: string }[] = [];
  /** Rotas da feature 002 (funis, automações, execuções, pausas…). */
  readonly automacoes: AutomacoesSimuladas = new AutomacoesSimuladas(this);
  private importacoes = new Map<string, { colunas: string[]; linhas: string[][]; nome: string }>();
  private seq = 0;
  private minuto = 0;

  /** Relógio determinístico: cada chamada avança 1 minuto a partir de 27/09/2026 10:00 (-03:00). */
  agora(): string {
    const base = Date.UTC(2026, 8, 27, 13, 0) + this.minuto++ * 60_000;
    const d = new Date(base - 3 * 3_600_000);
    return `${d.toISOString().slice(0, 19)}-03:00`;
  }

  novoId(prefixo: string): string {
    this.seq += 1;
    return `${prefixo}${String(this.seq).padStart(4, '0')}`;
  }

  normalizar(telefone: string): string | undefined {
    return normalizarTelefone(telefone).e164;
  }

  cliente(): ClienteMotor {
    return new ClienteMotor({ porta: 7788, token: TOKEN_SIMULADO, fetch: this.fetch });
  }

  // -------------------------------------------------------------------------
  // Semeadura
  // -------------------------------------------------------------------------

  adicionarConta(dados: Partial<Conta> = {}): Conta {
    const conta: Conta = {
      id: this.novoId('conta'),
      nome: 'Loja',
      telefone: '+5511900000001',
      jid: '5511900000001@s.whatsapp.net',
      estado: 'conectada',
      online: true,
      sincronizando: false,
      criada_em: this.agora(),
      ...dados,
    };
    this.contas.push(conta);
    return conta;
  }

  adicionarLead(telefone: string, nome: string | null = null, importado_em?: string, origem: OrigemLead = 'csv'): Lead {
    const lead: Lead = {
      id: this.novoId('lead'),
      telefone,
      nome,
      campos: {},
      origem,
      tem_whatsapp: null,
      importado_em: importado_em ?? this.agora(),
      ultimo_disparo_em: null,
    };
    this.leads.push(lead);
    return lead;
  }

  adicionarConversa(contaId: string, telefone: string, nome: string, naoLidas = 0): Conversa {
    const conversa: Conversa = {
      id: this.novoId('conv'),
      conta_id: contaId,
      jid: `${telefone.slice(1)}@s.whatsapp.net`,
      tipo: 'individual',
      nome,
      telefone,
      contato_id: null,
      nao_lidas: naoLidas,
      ultima_mensagem_em: null,
      ultima_mensagem_resumo: null,
      etiquetas: [],
    };
    this.conversas.push(conversa);
    return conversa;
  }

  adicionarMensagem(conversa: Conversa, texto: string, deMim = false, extra: Partial<Mensagem> = {}): Mensagem {
    const mensagem: Mensagem = {
      id: this.novoId('msg'),
      conta_id: conversa.conta_id,
      conversa_id: conversa.id,
      wa_id: this.novoId('WA'),
      remetente_jid: deMim ? 'eu@s.whatsapp.net' : conversa.jid,
      remetente_nome: deMim ? null : conversa.nome,
      de_mim: deMim,
      tipo: 'texto',
      texto,
      midia: null,
      citacao: null,
      reacoes: [],
      editada: false,
      apagada: false,
      estado: deMim ? 'enviada' : 'recebida',
      erro: null,
      disparo_id: null,
      automacao_id: null,
      enviada_em: this.agora(),
      pode_editar: deMim,
      pode_apagar: deMim,
      ...extra,
    };
    this.mensagens.push(mensagem);
    conversa.ultima_mensagem_em = mensagem.enviada_em;
    conversa.ultima_mensagem_resumo = texto;
    return mensagem;
  }

  adicionarContato(contaId: string, telefone: string, nome: string): Contato {
    const contato: Contato = {
      id: this.novoId('ctt'),
      conta_id: contaId,
      jid: `${telefone.slice(1)}@s.whatsapp.net`,
      telefone,
      nome,
      nome_push: null,
      notas: null,
      etiquetas: [],
      lead: null,
      conversa_id: null,
    };
    this.contatos.push(contato);
    return contato;
  }

  /** Requisições registradas que batem com método e caminho (regex ou texto exato). */
  chamadas(metodo: string, caminho: string | RegExp): RequisicaoRegistrada[] {
    return this.requisicoes.filter(
      (r) => r.metodo === metodo && (typeof caminho === 'string' ? r.caminho === caminho : caminho.test(r.caminho)),
    );
  }

  // -------------------------------------------------------------------------
  // fetch
  // -------------------------------------------------------------------------

  readonly fetch = async (entrada: string | URL | Request, init?: RequestInit): Promise<Response> => {
    const url = new URL(typeof entrada === 'string' || entrada instanceof URL ? entrada : entrada.url);
    const metodo = (init?.method ?? 'GET').toUpperCase();
    const cabecalhos = new Headers(init?.headers);
    if (cabecalhos.get('Authorization') !== `Bearer ${TOKEN_SIMULADO}`) {
      return this.respostaErro(new ErroHttp(401, 'nao_autorizado', 'Token inválido.'));
    }
    let corpo: Json = {};
    if (typeof init?.body === 'string' && init.body.length > 0) corpo = JSON.parse(init.body) as Json;
    const caminho = url.pathname.replace(/^\/v1/, '');
    this.requisicoes.push({ metodo, caminho, corpo });
    try {
      let resultado = await this.automacoes.rotear(metodo, caminho, url.searchParams, corpo);
      if (resultado === NAO_TRATADA) resultado = this.rotear({ metodo, caminho, query: url.searchParams, corpo });
      if (resultado === undefined) return new Response(null, { status: 204 });
      if (resultado instanceof Response) return resultado;
      return new Response(JSON.stringify(resultado), {
        status: metodo === 'POST' && /^\/(etiquetas|templates|arquivos|disparos|importacoes\/previa)$/.test(caminho) ? 201 : 200,
        headers: { 'Content-Type': 'application/json' },
      });
    } catch (erro) {
      if (erro instanceof ErroHttp) return this.respostaErro(erro);
      throw erro;
    }
  };

  private respostaErro(erro: ErroHttp): Response {
    return new Response(
      JSON.stringify({ erro: { codigo: erro.codigo, mensagem: erro.message, detalhes: erro.detalhes } }),
      { status: erro.status, headers: { 'Content-Type': 'application/json' } },
    );
  }

  private achar<T extends { id: string }>(lista: T[], id: string | undefined, rotulo: string): T {
    const item = lista.find((x) => x.id === id);
    if (!item) throw new ErroHttp(404, 'nao_encontrado', `${rotulo} não encontrado(a).`);
    return item;
  }

  private rotear(r: Requisicao): unknown {
    const { metodo: m, caminho: c, query: q, corpo: b } = r;
    let x: RegExpExecArray | null;

    // Sistema
    if (m === 'GET' && c === '/saude') return { ok: true, versao: '0.1.0', whatsapp: 'falso' };
    if (m === 'GET' && c === '/sistema') {
      return {
        versao: '0.1.0',
        pasta_dados: '/tmp/zapdesk-teste',
        caminho_logs: '/tmp/zapdesk-teste/logs',
        whatsapp: 'falso',
        disparos_ativos: this.disparos.filter((d) => ['enviando', 'fora_da_janela'].includes(d.estado)).length,
        contas_conectadas: this.contas.filter((k) => k.estado === 'conectada').length,
        automacoes_ativas: 0,
        processos_ia: 0,
        runner_disponivel: false,
      };
    }

    // Contas e conversas
    if (m === 'GET' && c === '/contas') return this.contas;
    if ((x = /^\/contas\/([^/]+)\/conversas$/.exec(c))) {
      const conta = this.achar(this.contas, x[1], 'Conta');
      if (m === 'GET') {
        let lista = this.conversas.filter((k) => k.conta_id === conta.id);
        if (q.get('nao_lidas') === 'true') lista = lista.filter((k) => k.nao_lidas > 0);
        const busca = q.get('busca')?.toLowerCase();
        if (busca) lista = lista.filter((k) => k.nome.toLowerCase().includes(busca) || (k.telefone ?? '').includes(busca));
        const etiqueta = q.get('etiqueta_id');
        if (etiqueta) lista = lista.filter((k) => k.etiquetas.some((e) => e.id === etiqueta));
        return paginar(lista, q);
      }
      if (m === 'POST') {
        const { e164 } = normalizarTelefone(String(b['telefone'] ?? ''));
        if (!e164) throw new ErroHttp(422, 'validacao', 'Telefone inválido.', { campos: { telefone: 'Telefone inválido.' } });
        if (conta.estado !== 'conectada') throw new ErroHttp(409, 'conta_indisponivel', 'A conta não está conectada.');
        if (this.semWhatsApp.has(e164)) throw new ErroHttp(422, 'sem_whatsapp', 'Esse número não tem WhatsApp.');
        return (
          this.conversas.find((k) => k.conta_id === conta.id && k.telefone === e164) ??
          this.adicionarConversa(conta.id, e164, e164)
        );
      }
    }
    if ((x = /^\/conversas\/([^/]+)$/.exec(c)) && m === 'GET') return this.achar(this.conversas, x[1], 'Conversa');
    if ((x = /^\/conversas\/([^/]+)\/lida$/.exec(c)) && m === 'POST') {
      this.achar(this.conversas, x[1], 'Conversa').nao_lidas = 0;
      return undefined;
    }
    if ((x = /^\/conversas\/([^/]+)\/mensagens$/.exec(c))) {
      const conversa = this.achar(this.conversas, x[1], 'Conversa');
      if (m === 'GET') {
        let lista = this.mensagens.filter((k) => k.conversa_id === conversa.id).reverse();
        const antes = q.get('antes');
        if (antes) lista = lista.slice(lista.findIndex((k) => k.id === antes) + 1);
        return paginar(lista, new URLSearchParams({ limite: q.get('limite') ?? '50' }));
      }
      if (m === 'POST') {
        const arquivoId = b['arquivo_id'] as string | null | undefined;
        if (!b['texto'] && !arquivoId) throw new ErroHttp(422, 'validacao', 'Escreva uma mensagem ou anexe um arquivo.');
        const arquivo = arquivoId ? this.achar(this.arquivos, arquivoId, 'Arquivo') : null;
        return this.adicionarMensagem(conversa, String(b['texto'] ?? ''), true, {
          estado: 'pendente',
          tipo: arquivo ? (arquivo.tipo_midia as Mensagem['tipo']) : 'texto',
          texto: (b['texto'] as string | null | undefined) ?? null,
          midia: arquivo
            ? {
                mimetype: arquivo.mimetype,
                tamanho: arquivo.tamanho,
                nome_arquivo: arquivo.nome,
                duracao_s: null,
                ptt: b['como'] === 'voz',
                largura: null,
                altura: null,
                miniatura_b64: null,
                baixada: true,
                url: '/v1/mensagens/x/midia',
              }
            : null,
        });
      }
    }
    if ((x = /^\/contas\/([^/]+)\/mensagens\/busca$/.exec(c)) && m === 'GET') {
      const termo = (q.get('q') ?? '').toLowerCase();
      const itens = this.mensagens
        .filter((k) => k.conta_id === x![1] && (k.texto ?? '').toLowerCase().includes(termo))
        .map((k) => {
          const conversa = this.achar(this.conversas, k.conversa_id, 'Conversa');
          return { mensagem: k, conversa: { id: conversa.id, nome: conversa.nome }, trecho: k.texto ?? '' };
        });
      return paginar(itens, q);
    }
    if ((x = /^\/mensagens\/([^/]+)\/reacao$/.exec(c)) && m === 'POST') {
      this.achar(this.mensagens, x[1], 'Mensagem');
      this.reacoes.push({ mensagem_id: x[1] as string, emoji: String(b['emoji']) });
      return undefined;
    }
    if ((x = /^\/mensagens\/([^/]+)$/.exec(c))) {
      const mensagem = this.achar(this.mensagens, x[1], 'Mensagem');
      if (!mensagem.de_mim) throw new ErroHttp(409, 'transicao_invalida', 'Só dá para mudar mensagens suas.');
      if (m === 'PATCH') {
        if (!mensagem.pode_editar) throw new ErroHttp(409, 'fora_do_prazo', 'Passou o prazo para editar.');
        mensagem.texto = String(b['texto']);
        mensagem.editada = true;
        return mensagem;
      }
      if (m === 'DELETE') {
        if (!mensagem.pode_apagar) throw new ErroHttp(409, 'fora_do_prazo', 'Passou o prazo para apagar.');
        mensagem.apagada = true;
        return undefined;
      }
    }

    // Status
    if ((x = /^\/contas\/([^/]+)\/status$/.exec(c)) && m === 'GET') {
      this.achar(this.contas, x[1], 'Conta');
      return this.status;
    }

    // Contatos
    if ((x = /^\/contas\/([^/]+)\/contatos$/.exec(c)) && m === 'GET') {
      let lista = this.contatos.filter((k) => k.conta_id === x![1]);
      const busca = q.get('busca')?.toLowerCase();
      if (busca) lista = lista.filter((k) => (k.nome ?? '').toLowerCase().includes(busca) || (k.telefone ?? '').includes(busca));
      const etiqueta = q.get('etiqueta_id');
      if (etiqueta) lista = lista.filter((k) => k.etiquetas.some((e) => e.id === etiqueta));
      return paginar(lista, q);
    }
    if ((x = /^\/contatos\/([^/]+)$/.exec(c)) && m === 'PATCH') {
      const contato = this.achar(this.contatos, x[1], 'Contato');
      contato.notas = String(b['notas']);
      return contato;
    }
    if ((x = /^\/contatos\/([^/]+)\/etiquetas$/.exec(c)) && m === 'PUT') {
      const contato = this.achar(this.contatos, x[1], 'Contato');
      contato.etiquetas = (b['etiqueta_ids'] as string[]).map((id) => this.achar(this.etiquetas, id, 'Etiqueta'));
      return contato;
    }

    // Etiquetas
    if (c === '/etiquetas' && m === 'GET') return this.etiquetas;
    if (c === '/etiquetas' && m === 'POST') {
      this.checarNomeUnico(this.etiquetas, String(b['nome']), 'Já existe uma etiqueta com esse nome.');
      const etiqueta: Etiqueta = { id: this.novoId('etq'), nome: String(b['nome']), cor: String(b['cor']), total_contatos: 0 };
      this.etiquetas.push(etiqueta);
      return etiqueta;
    }
    if ((x = /^\/etiquetas\/([^/]+)$/.exec(c))) {
      const etiqueta = this.achar(this.etiquetas, x[1], 'Etiqueta');
      if (m === 'PATCH') {
        if (b['nome'] !== undefined) {
          this.checarNomeUnico(this.etiquetas.filter((e) => e.id !== etiqueta.id), String(b['nome']), 'Já existe uma etiqueta com esse nome.');
          etiqueta.nome = String(b['nome']);
        }
        if (b['cor'] !== undefined) etiqueta.cor = String(b['cor']);
        return etiqueta;
      }
      if (m === 'DELETE') {
        this.etiquetas = this.etiquetas.filter((e) => e.id !== etiqueta.id);
        return undefined;
      }
    }

    // Arquivos
    if (c === '/arquivos' && m === 'POST') {
      const caminho = String(b['caminho'] ?? '');
      if (!caminho.startsWith('/') || !existsSync(caminho)) {
        throw new ErroHttp(422, 'validacao', 'Arquivo não encontrado.', { campos: { caminho: 'Arquivo não encontrado.' } });
      }
      const ext = extname(caminho).toLowerCase();
      const tipo = ['.jpg', '.jpeg', '.png'].includes(ext)
        ? 'imagem'
        : ext === '.webp'
          ? 'figurinha'
          : ['.ogg', '.mp3', '.m4a'].includes(ext)
            ? 'audio'
            : ext === '.mp4'
              ? 'video'
              : 'documento';
      const arquivo: Arquivo = {
        id: this.novoId('arq'),
        nome: basename(caminho),
        mimetype: tipo === 'imagem' ? 'image/png' : 'application/octet-stream',
        tamanho: readFileSync(caminho).length,
        tipo_midia: tipo,
        url: '/v1/arquivos/x/conteudo',
      };
      this.arquivos.push(arquivo);
      return arquivo;
    }

    // Templates
    if (c === '/templates' && m === 'GET') {
      const busca = q.get('busca')?.toLowerCase();
      return busca ? this.templates.filter((t) => t.nome.toLowerCase().includes(busca)) : this.templates;
    }
    if (c === '/templates' && m === 'POST') {
      this.checarNomeUnico(this.templates, String(b['nome']), 'Já existe um template com esse nome.');
      const arquivoId = b['arquivo_id'] as string | null | undefined;
      const agora = this.agora();
      const template: Template = {
        id: this.novoId('tpl'),
        nome: String(b['nome']),
        texto: String(b['texto']),
        variaveis: variaveisDe(String(b['texto'])),
        arquivo: arquivoId ? this.achar(this.arquivos, arquivoId, 'Arquivo') : null,
        criado_em: agora,
        atualizado_em: agora,
      };
      this.templates.push(template);
      return template;
    }
    if ((x = /^\/templates\/([^/]+)$/.exec(c))) {
      const template = this.achar(this.templates, x[1], 'Template');
      if (m === 'PATCH') {
        if (b['nome'] !== undefined) template.nome = String(b['nome']);
        if (b['texto'] !== undefined) {
          template.texto = String(b['texto']);
          template.variaveis = variaveisDe(template.texto);
        }
        if (b['arquivo_id'] === null) template.arquivo = null;
        else if (typeof b['arquivo_id'] === 'string') template.arquivo = this.achar(this.arquivos, b['arquivo_id'], 'Arquivo');
        template.atualizado_em = this.agora();
        return template;
      }
      if (m === 'DELETE') {
        this.templates = this.templates.filter((t) => t.id !== template.id);
        return undefined;
      }
    }

    // Leads e importação
    if (c === '/importacoes/previa' && m === 'POST') return this.previa(String(b['caminho'] ?? ''));
    if (c === '/leads/importar' && m === 'POST') return this.importar(b);
    if (c === '/leads' && m === 'GET') {
      let lista = [...this.leads].reverse();
      const origem = q.get('origem');
      if (origem) lista = lista.filter((l) => l.origem === origem);
      const busca = q.get('busca')?.toLowerCase();
      if (busca) lista = lista.filter((l) => l.telefone.includes(busca) || (l.nome ?? '').toLowerCase().includes(busca));
      return paginar(lista, q);
    }

    // Disparos
    if (c === '/disparos' && m === 'POST') return this.criarDisparo(b);
    if (c === '/disparos' && m === 'GET') {
      let lista = [...this.disparos].reverse();
      if (q.get('conta_id')) lista = lista.filter((d) => d.conta_id === q.get('conta_id'));
      if (q.get('estado')) lista = lista.filter((d) => d.estado === q.get('estado'));
      return paginar(lista, q);
    }
    if ((x = /^\/disparos\/([^/]+)$/.exec(c)) && m === 'GET') return this.achar(this.disparos, x[1], 'Disparo');
    if ((x = /^\/disparos\/([^/]+)\/destinatarios$/.exec(c)) && m === 'GET') {
      this.achar(this.disparos, x[1], 'Disparo');
      let lista = this.destinatarios.get(x[1] as string) ?? [];
      if (q.get('estado')) lista = lista.filter((d) => d.estado === q.get('estado'));
      return paginar(lista, q);
    }
    if ((x = /^\/disparos\/([^/]+)\/(iniciar|pausar|retomar|cancelar)$/.exec(c)) && m === 'POST') {
      return this.acaoDisparo(this.achar(this.disparos, x[1], 'Disparo'), x[2] as string, b);
    }
    if ((x = /^\/disparos\/([^/]+)\/relatorio\.csv$/.exec(c)) && m === 'GET') {
      const disparo = this.achar(this.disparos, x[1], 'Disparo');
      const linhas = (this.destinatarios.get(disparo.id) ?? []).map(
        (d) => `${d.telefone},"${d.nome ?? ''}",${d.estado},,,,,,,`,
      );
      const csv = `﻿telefone,nome,estado,motivo_falha,enviando_em,enviado_em,entregue_em,lido_em,respondeu_em,falhou_em\n${linhas.join('\n')}\n`;
      return new Response(csv, { status: 200, headers: { 'Content-Type': 'text/csv; charset=utf-8' } });
    }

    throw new ErroHttp(404, 'nao_encontrado', `Rota não simulada: ${m} ${c}`);
  }

  private checarNomeUnico(lista: { nome: string }[], nome: string, mensagem: string): void {
    if (lista.some((x) => x.nome.toLowerCase() === nome.toLowerCase())) throw new ErroHttp(409, 'conflito', mensagem);
  }

  // -------------------------------------------------------------------------
  // Leads
  // -------------------------------------------------------------------------

  private previa(caminho: string): unknown {
    if (!existsSync(caminho)) throw new ErroHttp(422, 'validacao', 'Arquivo não encontrado.');
    if (!caminho.endsWith('.csv')) throw new ErroHttp(415, 'tipo_nao_suportado', 'Use .csv ou .xlsx.');
    const [cabecalho, ...linhas] = readFileSync(caminho, 'utf8')
      .trim()
      .split(/\r?\n/)
      .map((l) => l.split(','));
    const colunas = cabecalho ?? [];
    const id = this.novoId('imp');
    this.importacoes.set(id, { colunas, linhas, nome: basename(caminho) });
    return {
      importacao_id: id,
      nome_arquivo: basename(caminho),
      colunas,
      amostra: linhas.slice(0, 5),
      total_linhas: linhas.length,
      coluna_telefone_sugerida: colunas.find((k) => /telefone|celular|whats|fone/i.test(k)) ?? null,
      coluna_nome_sugerida: colunas.find((k) => /nome/i.test(k)) ?? null,
      expira_em: this.agora(),
    };
  }

  importar(b: Json): RelatorioImportacao {
    const origem = (b['origem'] as OrigemLead | undefined) ?? 'mcp';
    const ddi = (b['ddi_padrao'] as string | undefined) ?? '55';
    let entradas: { telefone: string; nome?: string | null; campos?: Record<string, string> }[];
    if (Array.isArray(b['leads'])) {
      entradas = b['leads'] as typeof entradas;
    } else if (typeof b['importacao_id'] === 'string') {
      const imp = this.importacoes.get(b['importacao_id']);
      if (!imp) throw new ErroHttp(404, 'nao_encontrado', 'Importação expirada.');
      const mapa = b['mapeamento'] as { telefone: string; nome?: string | null };
      const iTel = imp.colunas.indexOf(mapa.telefone);
      if (iTel < 0) throw new ErroHttp(422, 'validacao', 'Escolha qual coluna tem o telefone.');
      const iNome = mapa.nome ? imp.colunas.indexOf(mapa.nome) : -1;
      entradas = imp.linhas.map((l) => ({ telefone: l[iTel] ?? '', nome: iNome >= 0 ? (l[iNome] ?? null) : null }));
    } else {
      throw new ErroHttp(422, 'validacao', 'Corpo de importação inválido.');
    }

    const rel: RelatorioImportacao = {
      total_linhas: entradas.length,
      total_novos: 0,
      total_ja_existentes: 0,
      total_invalidos: 0,
      total_duplicados_no_lote: 0,
      novos: [],
      ja_existentes: [],
      invalidos: [],
      duplicados_no_lote: [],
      lead_ids: [],
    };
    const vistos = new Map<string, number>();
    entradas.forEach((entrada, i) => {
      const linha = i + 1;
      const { e164, motivo } = normalizarTelefone(entrada.telefone, ddi);
      if (!e164) {
        rel.invalidos.push({ linha, valor: entrada.telefone, motivo: motivo as 'vazio' });
        return;
      }
      const primeira = vistos.get(e164);
      if (primeira !== undefined) {
        rel.duplicados_no_lote.push({ linha, telefone: e164, primeira_linha: primeira });
        return;
      }
      vistos.set(e164, linha);
      const existente = this.leads.find((l) => l.telefone === e164);
      if (existente) {
        const preenchidos: string[] = [];
        if (!existente.nome && entrada.nome) {
          existente.nome = entrada.nome;
          preenchidos.push('nome');
        }
        rel.ja_existentes.push({
          linha,
          lead_id: existente.id,
          telefone: e164,
          importado_em: existente.importado_em,
          campos_preenchidos: preenchidos,
        });
        rel.lead_ids.push(existente.id);
        return;
      }
      const lead = this.adicionarLead(e164, entrada.nome ?? null, undefined, origem);
      lead.campos = entrada.campos ?? {};
      rel.novos.push({ linha, lead_id: lead.id, telefone: e164 });
      rel.lead_ids.push(lead.id);
    });
    rel.total_novos = rel.novos.length;
    rel.total_ja_existentes = rel.ja_existentes.length;
    rel.total_invalidos = rel.invalidos.length;
    rel.total_duplicados_no_lote = rel.duplicados_no_lote.length;
    return rel;
  }

  // -------------------------------------------------------------------------
  // Disparos
  // -------------------------------------------------------------------------

  private criarDisparo(b: Json): Disparo {
    const conta = this.achar(this.contas, b['conta_id'] as string, 'Conta');
    let mensagem = b['mensagem'] as string | undefined;
    let arquivo: Arquivo | null = b['arquivo_id'] ? this.achar(this.arquivos, b['arquivo_id'] as string, 'Arquivo') : null;
    if (b['template_id']) {
      const template = this.achar(this.templates, b['template_id'] as string, 'Template');
      mensagem = template.texto;
      arquivo = arquivo ?? template.arquivo;
    }
    if (!mensagem) throw new ErroHttp(422, 'validacao', 'Escreva a mensagem.', { campos: { mensagem: 'obrigatória' } });
    const ritmo = b['ritmo'] as Disparo['ritmo'];
    if (ritmo.intervalo_max_s < ritmo.intervalo_min_s) {
      throw new ErroHttp(422, 'validacao', 'Revise o ritmo.', { campos: { intervalo_max_s: 'deve ser ≥ mínimo' } });
    }

    const dest = b['destinatarios'] as {
      lead_ids?: string[];
      etiqueta_ids?: string[];
      contato_ids?: string[];
      importar?: Json | null;
    };
    let relatorio: RelatorioImportacao | undefined;
    const ids: string[] = [...(dest.lead_ids ?? [])];
    if (dest.importar) {
      relatorio = this.importar(dest.importar);
      ids.push(...relatorio.lead_ids);
    }
    const contatos = this.contatos.filter(
      (k) =>
        (dest.contato_ids ?? []).includes(k.id) || k.etiquetas.some((e) => (dest.etiqueta_ids ?? []).includes(e.id)),
    );
    for (const contato of contatos) {
      const lead =
        this.leads.find((l) => l.telefone === contato.telefone) ??
        this.adicionarLead(contato.telefone ?? '', contato.nome, undefined, 'contatos');
      ids.push(lead.id);
    }
    const leads = [...new Set(ids)].map((id) => this.achar(this.leads, id, 'Lead'));
    if (leads.length === 0) throw new ErroHttp(422, 'validacao', 'Escolha pelo menos um destinatário.');

    const variaveis = variaveisDe(mensagem);
    const padrao = (b['valores_padrao'] as Record<string, string> | undefined) ?? {};
    const valor = (lead: Lead, v: string) => (v === 'nome' ? lead.nome : lead.campos[v]) || padrao[v];
    const faltando: VariavelFaltando[] = [];
    leads.forEach((lead, i) => {
      const falta = variaveis.filter((v) => !valor(lead, v));
      if (falta.length > 0) faltando.push({ destinatario_linha: i + 1, telefone: lead.telefone, variaveis: falta });
    });
    if (faltando.length > 0 && b['iniciar'] === true) {
      throw new ErroHttp(422, 'variaveis_faltando', 'Há destinatários sem valor para variáveis da mensagem.', {
        faltando: faltando.slice(0, 50),
        total: faltando.length,
      });
    }

    const ativo = this.disparos.some(
      (d) => d.conta_id === conta.id && ['enviando', 'fora_da_janela'].includes(d.estado),
    );
    const iniciar = b['iniciar'] === true;
    const agora = this.agora();
    const disparo: Disparo = {
      id: this.novoId('disp'),
      conta_id: conta.id,
      nome: (b['nome'] as string | undefined) ?? 'Disparo 27/09 10:00',
      mensagem,
      arquivo,
      ritmo,
      inicio_em: (b['inicio_em'] as string | null | undefined) ?? null,
      janela: (b['janela'] as Disparo['janela'] | undefined) ?? null,
      falhas_seguidas_max: (b['falhas_seguidas_max'] as number | undefined) ?? 10,
      valores_padrao: padrao,
      estado: !iniciar ? 'rascunho' : ativo ? 'agendado' : 'enviando',
      na_fila: iniciar && ativo,
      motivo_pausa: null,
      origem: (b['origem'] as Disparo['origem'] | undefined) ?? 'app',
      contadores: { total: leads.length, pendente: leads.length, enviando: 0, enviado: 0, entregue: 0, lido: 0, respondeu: 0, falhou: 0 },
      proximo_envio_em: iniciar ? agora : null,
      estimativa_termino_em: iniciar ? agora : null,
      aviso_ritmo_agressivo: ritmo.intervalo_min_s < 10 || (ritmo.limite_por_hora === null && ritmo.limite_por_dia === null),
      criado_em: agora,
      iniciado_em: iniciar ? agora : null,
      concluido_em: null,
      cancelado_em: null,
    };
    this.disparos.push(disparo);
    this.destinatarios.set(
      disparo.id,
      leads.map((lead, i) => ({
        id: this.novoId('dst'),
        disparo_id: disparo.id,
        lead_id: lead.id,
        ordem: i + 1,
        telefone: lead.telefone,
        nome: lead.nome,
        variaveis: Object.fromEntries(variaveis.map((v) => [v, valor(lead, v) ?? ''])),
        estado: 'pendente',
        motivo_falha: null,
        mensagem_wa_id: null,
        enviando_em: null,
        enviado_em: null,
        entregue_em: null,
        lido_em: null,
        respondeu_em: null,
        falhou_em: null,
      })),
    );
    return relatorio ? { ...disparo, relatorio_importacao: relatorio } : disparo;
  }

  private acaoDisparo(d: Disparo, acao: string, b: Json): Disparo {
    const invalida = () => {
      throw new ErroHttp(409, 'transicao_invalida', 'Essa ação não é permitida agora.', { estado_atual: d.estado });
    };
    const ativoOutro = () =>
      this.disparos.some((o) => o.id !== d.id && o.conta_id === d.conta_id && ['enviando', 'fora_da_janela'].includes(o.estado));
    switch (acao) {
      case 'iniciar':
        if (d.estado !== 'rascunho') invalida();
        if (b['valores_padrao']) d.valores_padrao = b['valores_padrao'] as Record<string, string>;
        d.na_fila = ativoOutro();
        d.estado = d.na_fila ? 'agendado' : 'enviando';
        break;
      case 'pausar':
        if (!['agendado', 'enviando', 'fora_da_janela'].includes(d.estado)) invalida();
        d.estado = 'pausado';
        d.na_fila = false;
        d.motivo_pausa = 'usuario';
        break;
      case 'retomar':
        if (d.estado !== 'pausado') invalida();
        d.motivo_pausa = null;
        d.na_fila = ativoOutro();
        d.estado = d.na_fila ? 'agendado' : 'enviando';
        break;
      case 'cancelar':
        if (['concluido', 'cancelado'].includes(d.estado)) invalida();
        d.estado = 'cancelado';
        d.na_fila = false;
        d.cancelado_em = this.agora();
        break;
    }
    return d;
  }
}
