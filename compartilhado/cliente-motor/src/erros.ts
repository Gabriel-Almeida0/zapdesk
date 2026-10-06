// Erro padrão das respostas do motor: status HTTP + `{"erro":{codigo, mensagem, detalhes}}`.
import { CODIGOS_ERRO, type CodigoErro, type DetalhesErro } from './tipos.js';

export class ErroMotor extends Error {
  readonly codigo: CodigoErro;
  readonly mensagem: string;
  readonly detalhes: DetalhesErro;
  /** Status HTTP; 0 quando a falha foi de rede (motor fora do ar). */
  readonly status: number;

  constructor(codigo: CodigoErro, mensagem: string, status: number, detalhes: DetalhesErro = {}) {
    super(mensagem);
    this.name = 'ErroMotor';
    this.codigo = codigo;
    this.mensagem = mensagem;
    this.status = status;
    this.detalhes = detalhes;
  }
}

/** Mensagem genérica usada quando o corpo de erro não segue o contrato. */
export const MENSAGEM_ERRO_INESPERADO = 'Algo deu errado no motor do ZapDesk.';

/** Mensagem usada quando não foi possível falar com o motor. */
export const MENSAGEM_MOTOR_FORA = 'Não consegui falar com o motor do ZapDesk.';

function ehCodigoErro(valor: unknown): valor is CodigoErro {
  return typeof valor === 'string' && (CODIGOS_ERRO as readonly string[]).includes(valor);
}

/** Código padrão para um status HTTP sem corpo reconhecível. */
function codigoPorStatus(status: number): CodigoErro {
  switch (status) {
    case 401:
      return 'nao_autorizado';
    case 403:
      return 'host_invalido';
    case 404:
      return 'nao_encontrado';
    case 409:
      return 'conflito';
    case 413:
      return 'anexo_grande_demais';
    case 415:
      return 'tipo_nao_suportado';
    case 422:
      return 'validacao';
    case 502:
      return 'whatsapp_erro';
    case 503:
      return 'runner_indisponivel';
    default:
      return 'interno';
  }
}

/**
 * Converte o corpo (já parseado ou texto) de uma resposta de erro em `ErroMotor`.
 * Tolera corpo vazio, texto não-JSON e códigos desconhecidos.
 */
export function parsearErro(status: number, corpo: unknown): ErroMotor {
  let valor: unknown = corpo;
  if (typeof corpo === 'string') {
    try {
      valor = corpo.length > 0 ? JSON.parse(corpo) : null;
    } catch {
      valor = null;
    }
  }

  const erro =
    valor !== null && typeof valor === 'object' && 'erro' in valor
      ? (valor as { erro: unknown }).erro
      : null;

  if (erro !== null && typeof erro === 'object') {
    const bruto = erro as { codigo?: unknown; mensagem?: unknown; detalhes?: unknown };
    const codigo = ehCodigoErro(bruto.codigo) ? bruto.codigo : codigoPorStatus(status);
    const mensagem =
      typeof bruto.mensagem === 'string' && bruto.mensagem.length > 0
        ? bruto.mensagem
        : MENSAGEM_ERRO_INESPERADO;
    const detalhes =
      bruto.detalhes !== null && typeof bruto.detalhes === 'object'
        ? (bruto.detalhes as DetalhesErro)
        : {};
    return new ErroMotor(codigo, mensagem, status, detalhes);
  }

  return new ErroMotor(codigoPorStatus(status), MENSAGEM_ERRO_INESPERADO, status);
}

export function ehErroMotor(valor: unknown): valor is ErroMotor {
  return valor instanceof ErroMotor;
}
