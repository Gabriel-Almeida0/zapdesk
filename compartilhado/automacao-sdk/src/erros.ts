// Classes de erro das automações de IA (contracts/sdk-automacao.md › erros).
// `codigo` usa os nomes da tabela de contracts/runner-protocolo.md (`error.data.codigo`); o
// runner converte os erros JSON-RPC do motor nestas classes, então `instanceof` funciona no
// código do usuário.
import type { Permissao } from './tipos.js';

/** Motivo pelo qual o portão de envio barrou uma mensagem. */
export type MotivoBloqueio = 'pausa' | 'pausa_geral' | 'grupo' | 'anti_loop' | 'primeiro_contato';

/** Erro base de tudo o que o ZapDesk lança dentro de uma automação. */
export class ErroAutomacao extends Error {
  /** Código estável (ex.: `permissao_negada`, `bloqueado`, `ia_erro`). */
  readonly codigo: string;

  constructor(mensagem: string, opcoes?: { codigo?: string; causa?: unknown }) {
    super(mensagem, opcoes?.causa === undefined ? undefined : { cause: opcoes.causa });
    this.name = 'ErroAutomacao';
    this.codigo = opcoes?.codigo ?? 'erro';
  }
}

/** Operação exige uma permissão não declarada em `automacao.json` › `permissoes`. */
export class ErroPermissao extends ErroAutomacao {
  readonly permissao: Permissao;

  constructor(permissao: Permissao, mensagem?: string) {
    super(mensagem ?? `Permissão '${permissao}' não declarada em automacao.json`, {
      codigo: 'permissao_negada',
    });
    this.name = 'ErroPermissao';
    this.permissao = permissao;
  }
}

/** Envio barrado pelo portão (pausa, grupo, anti-loop, primeiro contato). */
export class ErroBloqueado extends ErroAutomacao {
  readonly motivo: MotivoBloqueio;

  constructor(motivo: MotivoBloqueio, mensagem?: string) {
    super(mensagem ?? `Envio bloqueado (${motivo}).`, { codigo: 'bloqueado' });
    this.name = 'ErroBloqueado';
    this.motivo = motivo;
  }
}

/** Falha da Claude API (ou chave não configurada: `codigo = 'ia_nao_configurada'`). */
export class ErroIA extends ErroAutomacao {
  /** Status HTTP da Claude API, quando houver. */
  readonly status: number | null;
  /** Cabeçalho `request-id` da Claude API, quando houver. */
  readonly requestId: string | null;

  constructor(
    mensagem: string,
    opcoes?: {
      codigo?: 'ia_erro' | 'ia_nao_configurada';
      status?: number | null;
      requestId?: string | null;
    },
  ) {
    super(mensagem, { codigo: opcoes?.codigo ?? 'ia_erro' });
    this.name = 'ErroIA';
    this.status = opcoes?.status ?? null;
    this.requestId = opcoes?.requestId ?? null;
  }
}

/** Segredo não declarado em `automacao.json` ou não definido em Ajustes → IA. */
export class ErroSegredo extends ErroAutomacao {
  constructor(mensagem: string) {
    super(mensagem, { codigo: 'segredo' });
    this.name = 'ErroSegredo';
  }
}

/** Dados inválidos ou acima de um limite (`codigo = 'limite'`). */
export class ErroValidacao extends ErroAutomacao {
  /** Mensagem por campo, quando o motor informar. */
  readonly campos: Record<string, string>;

  constructor(
    mensagem: string,
    opcoes?: { campos?: Record<string, string>; codigo?: 'validacao' | 'limite' },
  ) {
    super(mensagem, { codigo: opcoes?.codigo ?? 'validacao' });
    this.name = 'ErroValidacao';
    this.campos = opcoes?.campos ?? {};
  }
}

/** Etiqueta, funil, etapa, lead ou conversa não encontrados. */
export class ErroNaoEncontrado extends ErroAutomacao {
  constructor(mensagem: string) {
    super(mensagem, { codigo: 'nao_encontrado' });
    this.name = 'ErroNaoEncontrado';
  }
}

/** Chamada ao `ctx` depois do fim (ou cancelamento) da execução. */
export class ErroExecucaoEncerrada extends ErroAutomacao {
  constructor(mensagem = 'A execução já terminou.') {
    super(mensagem, { codigo: 'execucao_encerrada' });
    this.name = 'ErroExecucaoEncerrada';
  }
}
