import { describe, expect, it, vi } from 'vitest';

import { ClienteMotor, ErroMotor, type Fetch, parsearErro } from '../src/index.js';

const TOKEN = 'x'.repeat(43);

interface Chamada {
  url: URL;
  init: RequestInit;
}

/** fetch simulado que registra chamadas e devolve respostas enfileiradas. */
function fetchSimulado(...respostas: Response[]) {
  const chamadas: Chamada[] = [];
  const fn = vi.fn(async (entrada: Parameters<Fetch>[0], init?: RequestInit) => {
    chamadas.push({ url: new URL(String(entrada)), init: init ?? {} });
    const r = respostas.shift();
    if (!r) throw new Error('sem resposta simulada');
    return r;
  });
  return { fetch: fn as unknown as Fetch, chamadas };
}

const json = (corpo: unknown, status = 200) =>
  new Response(JSON.stringify(corpo), { status, headers: { 'Content-Type': 'application/json' } });

function novoCliente(fetch: Fetch) {
  return new ClienteMotor({ porta: 51234, token: TOKEN, fetch });
}

describe('ClienteMotor — autenticação e URLs', () => {
  it('envia Authorization: Bearer e usa 127.0.0.1:<porta>/v1', async () => {
    const { fetch, chamadas } = fetchSimulado(json({ ok: true, versao: '0.1.0', whatsapp: 'falso' }));
    const saude = await novoCliente(fetch).saude();

    expect(saude.whatsapp).toBe('falso');
    const c = chamadas[0]!;
    expect(c.url.toString()).toBe('http://127.0.0.1:51234/v1/saude');
    expect((c.init.headers as Record<string, string>)['Authorization']).toBe(`Bearer ${TOKEN}`);
    expect(c.url.searchParams.has('token')).toBe(false);
  });

  it('urlBinaria adiciona ?token= e aceita caminho com ou sem /v1', () => {
    const cliente = novoCliente(fetchSimulado().fetch);
    expect(cliente.urlBinaria('/v1/mensagens/01J/midia')).toBe(
      `http://127.0.0.1:51234/v1/mensagens/01J/midia?token=${TOKEN}`,
    );
    expect(cliente.urlBinaria('/arquivos/A/conteudo')).toBe(
      `http://127.0.0.1:51234/v1/arquivos/A/conteudo?token=${TOKEN}`,
    );
    expect(cliente.urlEventos()).toBe(`ws://127.0.0.1:51234/v1/eventos?token=${TOKEN}`);
  });

  it('envia JSON com Content-Type e método correto', async () => {
    const { fetch, chamadas } = fetchSimulado(json({ id: 'E1', nome: 'Quente', cor: '#FF0000', total_contatos: 0 }, 201));
    await novoCliente(fetch).criarEtiqueta({ nome: 'Quente', cor: '#FF0000' });

    const c = chamadas[0]!;
    expect(c.init.method).toBe('POST');
    expect(c.url.pathname).toBe('/v1/etiquetas');
    expect((c.init.headers as Record<string, string>)['Content-Type']).toBe('application/json');
    expect(JSON.parse(String(c.init.body))).toEqual({ nome: 'Quente', cor: '#FF0000' });
  });

  it('codifica segmentos de caminho', async () => {
    const { fetch, chamadas } = fetchSimulado(json({}));
    await novoCliente(fetch).obterConta('a/b');
    expect(chamadas[0]!.url.pathname).toBe('/v1/contas/a%2Fb');
  });

  it('204 resolve como undefined', async () => {
    const { fetch, chamadas } = fetchSimulado(new Response(null, { status: 204 }));
    await expect(novoCliente(fetch).excluirEtiqueta('E1')).resolves.toBeUndefined();
    expect(chamadas[0]!.init.method).toBe('DELETE');
  });

  it('rotas /v1/falso/* usam o prefixo correto', async () => {
    const { fetch, chamadas } = fetchSimulado(json({}), json([]));
    const cliente = novoCliente(fetch);
    await cliente.falso.escanearQr('C1', { telefone: '+5511900000001', nome: 'Teste' });
    await cliente.falso.enviadas();
    expect(chamadas[0]!.url.pathname).toBe('/v1/falso/contas/C1/escanear-qr');
    expect(chamadas[1]!.url.pathname).toBe('/v1/falso/enviadas');
  });
});

describe('ClienteMotor — erros', () => {
  it('converte o corpo {"erro":{...}} em ErroMotor', async () => {
    const { fetch } = fetchSimulado(
      json(
        {
          erro: {
            codigo: 'variaveis_faltando',
            mensagem: 'Faltam variáveis.',
            detalhes: {
              faltando: [{ destinatario_linha: 2, telefone: '+5511999990000', variaveis: ['empresa'] }],
              total: 1,
            },
          },
        },
        422,
      ),
    );
    const promessa = novoCliente(fetch).iniciarDisparo('D1');
    await expect(promessa).rejects.toBeInstanceOf(ErroMotor);
    const erro = (await promessa.catch((e: unknown) => e)) as ErroMotor;
    expect(erro.codigo).toBe('variaveis_faltando');
    expect(erro.status).toBe(422);
    expect(erro.mensagem).toBe('Faltam variáveis.');
    expect(erro.detalhes.faltando?.[0]?.variaveis).toEqual(['empresa']);
  });

  it('corpo inválido cai no código pelo status', () => {
    expect(parsearErro(401, '').codigo).toBe('nao_autorizado');
    expect(parsearErro(404, 'not found').codigo).toBe('nao_encontrado');
    expect(parsearErro(500, '{"x":1}').codigo).toBe('interno');
    expect(parsearErro(409, { erro: { codigo: 'desconhecido', mensagem: 'M' } }).codigo).toBe('conflito');
  });

  it('falha de rede vira ErroMotor com status 0', async () => {
    const fetch = (async () => {
      throw new TypeError('fetch failed');
    }) as unknown as Fetch;
    const erro = (await novoCliente(fetch).listarContas().catch((e: unknown) => e)) as ErroMotor;
    expect(erro).toBeInstanceOf(ErroMotor);
    expect(erro.status).toBe(0);
    expect(erro.detalhes.motivo).toBe('fetch failed');
  });
});

describe('ClienteMotor — paginação', () => {
  it('passa limite/cursor/filtros na query e omite vazios', async () => {
    const { fetch, chamadas } = fetchSimulado(json({ itens: [], proximo_cursor: 'abc' }));
    const pagina = await novoCliente(fetch).listarConversas('C1', {
      limite: 20,
      cursor: 'xyz',
      nao_lidas: true,
      busca: '',
    });

    expect(pagina.proximo_cursor).toBe('abc');
    const q = chamadas[0]!.url.searchParams;
    expect(q.get('limite')).toBe('20');
    expect(q.get('cursor')).toBe('xyz');
    expect(q.get('nao_lidas')).toBe('true');
    expect(q.has('busca')).toBe(false);
  });

  it('busca de mensagens envia q', async () => {
    const { fetch, chamadas } = fetchSimulado(json({ itens: [], proximo_cursor: null }));
    await novoCliente(fetch).buscarMensagens('C1', 'ação', { limite: 10 });
    expect(chamadas[0]!.url.pathname).toBe('/v1/contas/C1/mensagens/busca');
    expect(chamadas[0]!.url.searchParams.get('q')).toBe('ação');
  });
});

describe('ClienteMotor — multipart', () => {
  it('envia /arquivos como multipart no campo "arquivo" sem Content-Type manual', async () => {
    const { fetch, chamadas } = fetchSimulado(
      json({ id: 'A1', nome: 'foto.png', mimetype: 'image/png', tamanho: 3, tipo_midia: 'imagem', url: '/v1/arquivos/A1/conteudo' }, 201),
    );
    const arquivo = await novoCliente(fetch).enviarArquivo({
      dados: new Blob([new Uint8Array([1, 2, 3])], { type: 'image/png' }),
      nome: 'foto.png',
    });

    expect(arquivo.id).toBe('A1');
    const c = chamadas[0]!;
    expect(c.url.pathname).toBe('/v1/arquivos');
    expect((c.init.headers as Record<string, string>)['Content-Type']).toBeUndefined();
    expect(c.init.body).toBeInstanceOf(FormData);
    const parte = (c.init.body as FormData).get('arquivo') as File;
    expect(parte.name).toBe('foto.png');
    expect(parte.size).toBe(3);
  });

  it('prévia de importação por multipart e por caminho', async () => {
    const previa = {
      importacao_id: 'I1',
      nome_arquivo: 'leads.csv',
      colunas: ['Nome', 'Celular'],
      amostra: [],
      total_linhas: 0,
      coluna_telefone_sugerida: 'Celular',
      coluna_nome_sugerida: 'Nome',
      expira_em: '2026-09-27T20:00:00-03:00',
    };
    const { fetch, chamadas } = fetchSimulado(json(previa, 201), json(previa, 201));
    const cliente = novoCliente(fetch);
    await cliente.previaImportacao({ dados: new Blob(['Nome,Celular\n']), nome: 'leads.csv' });
    await cliente.previaImportacaoPorCaminho('/tmp/leads.csv');

    expect(chamadas[0]!.url.pathname).toBe('/v1/importacoes/previa');
    expect(chamadas[0]!.init.body).toBeInstanceOf(FormData);
    expect(JSON.parse(String(chamadas[1]!.init.body))).toEqual({ caminho: '/tmp/leads.csv' });
  });
});
