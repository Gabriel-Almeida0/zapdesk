// `fetch` compatível que passa pelo processo principal (preload → IPC → main → motor).
// O `ClienteMotor` recebe esta função; assim o renderer não depende de CORS no motor.
import type { RequisicaoIpc, RespostaIpc } from '../../preload/tipos';

export interface PonteRequisicao {
  requisitarMotor(req: RequisicaoIpc): Promise<RespostaIpc>;
}

const SEM_CORPO = new Set([101, 204, 205, 304]);

export function criarFetchIpc(ponte: PonteRequisicao): typeof fetch {
  return async (entrada, init) => {
    // `Request` serializa FormData (multipart com boundary) e JSON do mesmo jeito que o fetch.
    const requisicao = new Request(entrada, init);
    const corpo =
      requisicao.method === 'GET' || requisicao.method === 'HEAD'
        ? null
        : new Uint8Array(await requisicao.arrayBuffer());
    const cabecalhos: Record<string, string> = {};
    requisicao.headers.forEach((valor, chave) => {
      cabecalhos[chave] = valor;
    });
    const resposta = await ponte.requisitarMotor({
      url: requisicao.url,
      metodo: requisicao.method,
      cabecalhos,
      corpo: corpo && corpo.byteLength > 0 ? corpo : null,
    });
    return new Response(SEM_CORPO.has(resposta.status) ? null : (resposta.corpo as Uint8Array<ArrayBuffer>), {
      status: resposta.status,
      headers: resposta.cabecalhos,
    });
  };
}
