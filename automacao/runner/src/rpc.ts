// JSON-RPC 2.0 bidirecional em NDJSON (uma mensagem por linha, UTF-8, `\n`), no máximo 1 MB por
// linha (runner-protocolo.md). Independente de stdin/stdout: quem usa entrega os pedaços lidos
// em `receber()` e fornece `escrever()` — assim os testes ligam duas conexões em memória.
import {
  CODIGOS_RPC,
  LIMITE_LINHA,
  type ErroRpcCorpo,
  type IdRpc,
  type MensagemRpc,
} from './protocolo.js';

/** Erro JSON-RPC com código e dados (vira `error` na resposta). */
export class ErroRpc extends Error {
  readonly code: number;
  readonly data: unknown;

  constructor(code: number, mensagem: string, data?: unknown) {
    super(mensagem);
    this.name = 'ErroRpc';
    this.code = code;
    this.data = data;
  }

  corpo(): ErroRpcCorpo {
    return this.data === undefined
      ? { code: this.code, message: this.message }
      : { code: this.code, message: this.message, data: this.data };
  }
}

export interface OpcoesConexao {
  /** Escreve uma linha completa (já com `\n`). */
  escrever(linha: string): void;
  /** Trata requisições recebidas; lance `ErroRpc` para responder erro. `undefined` → método inexistente. */
  aoRequisicao?(metodo: string, params: unknown, id: IdRpc): unknown;
  /** Trata notificações recebidas (erros são ignorados). */
  aoNotificacao?(metodo: string, params: unknown): void;
  /** Chamado no fim da entrada (EOF). */
  aoFim?(): void;
  /** Limite por linha, em bytes (padrão 1 MB). */
  limiteLinha?: number;
}

interface Pendente {
  resolver(valor: unknown): void;
  rejeitar(erro: unknown): void;
}

const NL = 0x0a;

export class ConexaoRpc {
  private readonly op: OpcoesConexao;
  private readonly limite: number;
  private pedacos: Buffer[] = [];
  private tamanho = 0;
  private descartando = false;
  private proximoId = 1;
  private readonly pendentes = new Map<IdRpc, Pendente>();
  private encerrada = false;

  constructor(opcoes: OpcoesConexao) {
    this.op = opcoes;
    this.limite = opcoes.limiteLinha ?? LIMITE_LINHA;
  }

  /** Entrega bytes lidos da entrada. */
  receber(pedaco: Buffer | string): void {
    let buf = typeof pedaco === 'string' ? Buffer.from(pedaco, 'utf8') : pedaco;
    while (buf.length > 0) {
      const nl = buf.indexOf(NL);
      const parte = nl === -1 ? buf : buf.subarray(0, nl);
      if (!this.descartando) {
        if (this.tamanho + parte.length > this.limite) {
          // Linha grande demais: descarta tudo até o próximo `\n`.
          this.descartando = true;
          this.pedacos = [];
          this.tamanho = 0;
        } else if (parte.length > 0) {
          this.pedacos.push(Buffer.from(parte));
          this.tamanho += parte.length;
        }
      }
      if (nl === -1) return;
      buf = buf.subarray(nl + 1);
      if (this.descartando) {
        this.descartando = false;
        this.enviarErro(null, CODIGOS_RPC.requisicao_invalida, 'Linha maior que 1 MB descartada.');
        continue;
      }
      const linha = Buffer.concat(this.pedacos, this.tamanho).toString('utf8');
      this.pedacos = [];
      this.tamanho = 0;
      if (linha.trim().length > 0) this.processarLinha(linha);
    }
  }

  /** Fim da entrada (EOF): rejeita pendentes e avisa. */
  fim(): void {
    if (this.encerrada) return;
    this.encerrada = true;
    this.rejeitarPendentes(new ErroRpc(CODIGOS_RPC.interno, 'Conexão encerrada.'));
    this.op.aoFim?.();
  }

  /** Faz uma requisição e espera a resposta. */
  requisitar(metodo: string, params?: unknown): Promise<unknown> {
    if (this.encerrada) return Promise.reject(new ErroRpc(CODIGOS_RPC.interno, 'Conexão encerrada.'));
    const id = this.proximoId++;
    const linha = serializar({ jsonrpc: '2.0', id, method: metodo, params });
    if (Buffer.byteLength(linha) > this.limite) {
      return Promise.reject(
        new ErroRpc(CODIGOS_RPC.requisicao_invalida, 'Dados maiores que 1 MB.', { codigo: 'limite' }),
      );
    }
    return new Promise((resolver, rejeitar) => {
      this.pendentes.set(id, { resolver, rejeitar });
      this.op.escrever(linha);
    });
  }

  /** Envia uma notificação (sem resposta). Linhas acima do limite são descartadas. */
  notificar(metodo: string, params?: unknown): void {
    if (this.encerrada) return;
    const linha = serializar({ jsonrpc: '2.0', method: metodo, params });
    if (Buffer.byteLength(linha) > this.limite) return;
    this.op.escrever(linha);
  }

  /** Quantas requisições nossas aguardam resposta. */
  get emAberto(): number {
    return this.pendentes.size;
  }

  rejeitarPendentes(erro: unknown): void {
    const lista = [...this.pendentes.values()];
    this.pendentes.clear();
    for (const p of lista) p.rejeitar(erro);
  }

  private processarLinha(linha: string): void {
    let msg: MensagemRpc;
    try {
      msg = JSON.parse(linha) as MensagemRpc;
    } catch {
      this.enviarErro(null, CODIGOS_RPC.parse, 'JSON inválido.');
      return;
    }
    if (msg === null || typeof msg !== 'object' || Array.isArray(msg) || msg.jsonrpc !== '2.0') {
      this.enviarErro(idDe(msg), CODIGOS_RPC.requisicao_invalida, 'Mensagem JSON-RPC inválida.');
      return;
    }
    if ('method' in msg && typeof msg.method === 'string') {
      if ('id' in msg && msg.id !== undefined && msg.id !== null) {
        void this.tratarRequisicao(msg.id, msg.method, msg.params);
      } else {
        try {
          this.op.aoNotificacao?.(msg.method, msg.params);
        } catch {
          // notificações não têm resposta
        }
      }
      return;
    }
    if ('id' in msg && ('result' in msg || 'error' in msg)) {
      const id = msg.id;
      if (id === null) return; // erro sem id do outro lado: nada a casar
      const p = this.pendentes.get(id);
      if (!p) return;
      this.pendentes.delete(id);
      if ('error' in msg) {
        const e = msg.error ?? { code: CODIGOS_RPC.interno, message: 'Erro desconhecido.' };
        p.rejeitar(new ErroRpc(e.code, e.message, e.data));
      } else {
        p.resolver(msg.result);
      }
      return;
    }
    this.enviarErro(idDe(msg), CODIGOS_RPC.requisicao_invalida, 'Mensagem JSON-RPC inválida.');
  }

  private async tratarRequisicao(id: IdRpc, metodo: string, params: unknown): Promise<void> {
    try {
      if (!this.op.aoRequisicao) throw metodoInexistente(metodo);
      const resultado = await this.op.aoRequisicao(metodo, params, id);
      if (resultado === undefined) throw metodoInexistente(metodo);
      const linha = serializar({ jsonrpc: '2.0', id, result: resultado });
      if (Buffer.byteLength(linha) > this.limite) {
        this.enviarErro(id, CODIGOS_RPC.requisicao_invalida, 'Resposta maior que 1 MB.');
        return;
      }
      if (!this.encerrada) this.op.escrever(linha);
    } catch (erro) {
      if (erro instanceof ErroRpc) {
        this.escreverErro(id, erro.corpo());
      } else {
        this.enviarErro(id, CODIGOS_RPC.interno, erro instanceof Error ? erro.message : String(erro));
      }
    }
  }

  private enviarErro(id: IdRpc | null, code: number, message: string): void {
    this.escreverErro(id, { code, message });
  }

  private escreverErro(id: IdRpc | null, corpo: ErroRpcCorpo): void {
    if (this.encerrada) return;
    let linha = serializar({ jsonrpc: '2.0', id, error: corpo });
    if (Buffer.byteLength(linha) > this.limite) {
      linha = serializar({ jsonrpc: '2.0', id, error: { code: corpo.code, message: corpo.message.slice(0, 1000) } });
    }
    this.op.escrever(linha);
  }
}

export function metodoInexistente(metodo: string): ErroRpc {
  return new ErroRpc(CODIGOS_RPC.metodo_inexistente, `Método desconhecido: ${metodo}`);
}

function idDe(msg: unknown): IdRpc | null {
  if (msg && typeof msg === 'object' && 'id' in msg) {
    const id = (msg as { id: unknown }).id;
    if (typeof id === 'number' || typeof id === 'string') return id;
  }
  return null;
}

function serializar(msg: unknown): string {
  return `${JSON.stringify(msg)}\n`;
}
