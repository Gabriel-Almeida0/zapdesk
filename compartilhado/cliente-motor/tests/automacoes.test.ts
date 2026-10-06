// Métodos da feature 002 (specs/002-automacoes/contracts/api-http.md e runtime.md) com fetch
// simulado: caminhos, métodos, query, corpos e erros novos.
import { describe, expect, it, vi } from 'vitest';

import {
  ClienteMotor,
  ErroMotor,
  type Fetch,
  type NovaAutomacao,
  parsearErro,
  TIPOS_EVENTO,
  CODIGOS_ERRO,
} from '../src/index.js';

const TOKEN = 'x'.repeat(43);

interface Chamada {
  url: URL;
  init: RequestInit;
}

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
const vazio = () => new Response(null, { status: 204 });
const erro = (status: number, codigo: string, mensagem: string, detalhes: unknown = {}) =>
  json({ erro: { codigo, mensagem, detalhes } }, status);

const novoCliente = (fetch: Fetch) => new ClienteMotor({ porta: 51234, token: TOKEN, fetch });

const corpo = (c: Chamada) => (c.init.body === undefined ? undefined : JSON.parse(String(c.init.body)));
const resumo = (c: Chamada) => `${c.init.method} ${c.url.pathname}${c.url.search}`;

const FLUXO: NovaAutomacao = {
  tipo: 'fluxo',
  nome: 'Respondeu → quente',
  gatilhos: [{ tipo: 'disparo_respondeu' }],
  definicao: {
    versao: 1,
    condicoes: null,
    acoes: [
      { tipo: 'adicionar_etiqueta', etiqueta_id: 'E1' },
      { tipo: 'aguardar', duracao_s: 7200 },
      { tipo: 'enviar_texto', texto: 'Oi {primeiro_nome}!', valores_padrao: { primeiro_nome: 'tudo bem' } },
    ],
  },
  limites: { anti_loop: null },
};

describe('Funis', () => {
  it('rotas de funil e etapas', async () => {
    const { fetch, chamadas } = fetchSimulado(
      json([]),
      json({}, 201),
      json({}),
      json({}),
      vazio(),
      json({}, 201),
      json({}),
      json({}),
      vazio(),
      vazio(),
      vazio(),
    );
    const c = novoCliente(fetch);
    await c.listarFunis();
    await c.criarFunil({ nome: 'Vendas', etapas: [{ nome: 'Novo' }, { nome: 'Ganho', cor: '#00AA00' }] });
    await c.obterFunil('F1');
    await c.editarFunil('F1', { nome: 'Vendas B2B', ordem: 2 });
    await c.excluirFunil('F1');
    await c.criarEtapa('F1', { nome: 'Qualificando', cor: '#FFAA00', posicao: 1 });
    await c.reordenarEtapas('F1', ['S2', 'S1']);
    await c.editarEtapa('S1', { cor: '#112233' });
    await c.excluirEtapa('S1', { destino_etapa_id: 'S2' });
    await c.excluirEtapa('S3', { remover_cards: true });
    await c.excluirEtapa('S4');

    expect(chamadas.map(resumo)).toEqual([
      'GET /v1/funis',
      'POST /v1/funis',
      'GET /v1/funis/F1',
      'PATCH /v1/funis/F1',
      'DELETE /v1/funis/F1',
      'POST /v1/funis/F1/etapas',
      'PUT /v1/funis/F1/etapas/ordem',
      'PATCH /v1/etapas/S1',
      'DELETE /v1/etapas/S1?destino_etapa_id=S2',
      'DELETE /v1/etapas/S3?remover_cards=true',
      'DELETE /v1/etapas/S4',
    ]);
    expect(corpo(chamadas[1]!)).toEqual({ nome: 'Vendas', etapas: [{ nome: 'Novo' }, { nome: 'Ganho', cor: '#00AA00' }] });
    expect(corpo(chamadas[6]!)).toEqual({ etapa_ids: ['S2', 'S1'] });
  });

  it('cards, histórico e funis do lead', async () => {
    const { fetch, chamadas } = fetchSimulado(
      json({ itens: [], proximo_cursor: null }),
      json({}, 201),
      json({}),
      vazio(),
      json({ itens: [], proximo_cursor: null }),
      json([]),
    );
    const c = novoCliente(fetch);
    await c.listarCards('F1', { etapa_id: 'S1', busca: 'ana', limite: 20 });
    await c.moverCard('F1', { telefone: '+5511999990000', etapa_id: 'S1', origem: 'mcp' });
    await c.moverCard('F1', { lead_id: 'L1', etapa_id: 'S2' });
    await c.removerCard('F1', 'L1', 'mcp');
    await c.historicoFunil('F1', { lead_id: 'L1', cursor: 'abc' });
    await c.funisDoLead('L1');

    expect(chamadas.map(resumo)).toEqual([
      'GET /v1/funis/F1/cards?etapa_id=S1&busca=ana&limite=20',
      'PUT /v1/funis/F1/cards',
      'PUT /v1/funis/F1/cards',
      'DELETE /v1/funis/F1/cards/L1?origem=mcp',
      'GET /v1/funis/F1/historico?lead_id=L1&cursor=abc',
      'GET /v1/leads/L1/funis',
    ]);
    expect(corpo(chamadas[1]!)).toEqual({ telefone: '+5511999990000', etapa_id: 'S1', origem: 'mcp' });
  });

  it('acréscimos de leads e disparos', async () => {
    const { fetch, chamadas } = fetchSimulado(json({}), json({ adicionados: 2, ja_existiam: 0, disparo: {} }));
    const c = novoCliente(fetch);
    await c.editarLead('L1', { nome: null, campos: { empresa: 'ACME', antigo: null } });
    const r = await c.adicionarDestinatarios('D1', ['L1', 'L2']);

    expect(chamadas.map(resumo)).toEqual(['PATCH /v1/leads/L1', 'POST /v1/disparos/D1/destinatarios']);
    expect(corpo(chamadas[0]!)).toEqual({ nome: null, campos: { empresa: 'ACME', antigo: null } });
    expect(corpo(chamadas[1]!)).toEqual({ lead_ids: ['L1', 'L2'] });
    expect(r.adicionados).toBe(2);
  });
});

describe('Automações', () => {
  it('CRUD, ativação, execução, teste e listagens', async () => {
    const respostas = Array.from({ length: 15 }, (_, i) => (i === 6 ? vazio() : json({})));
    const { fetch, chamadas } = fetchSimulado(...respostas);
    const c = novoCliente(fetch);
    await c.listarAutomacoes({ tipo: 'fluxo', ativa: false, busca: 'quente' });
    await c.criarAutomacao(FLUXO);
    await c.criarAutomacaoIA({ nome: 'Responder', modelo: 'responder_historico' });
    await c.validarAutomacao(FLUXO);
    await c.obterAutomacao('A1');
    await c.editarAutomacao('A1', { prioridade: 10, limites: { anti_loop: { mensagens: 3, janela_min: 10 } } });
    await c.excluirAutomacao('A1');
    await c.ativarAutomacao('A2');
    await c.desativarAutomacao('A2');
    await c.executarAutomacao('A2', { conversa_id: 'C1', entrada: { x: 1 }, origem: 'manual_mcp' });
    await c.testarAutomacao('A2', { mensagem: { texto: 'Quanto custa?' }, ia_simulada: true });
    await c.listarExecucoesAutomacao('A2', { estado: 'erro', limite: 10 });
    await c.listarExecucoes({ conversa_id: 'C1' });
    await c.obterExecucao('X1');
    await c.listarSessoesChatbot('A3', { estado: 'ativa' });

    expect(chamadas.map(resumo)).toEqual([
      'GET /v1/automacoes?tipo=fluxo&ativa=false&busca=quente',
      'POST /v1/automacoes',
      'POST /v1/automacoes/ia',
      'POST /v1/automacoes/validar',
      'GET /v1/automacoes/A1',
      'PATCH /v1/automacoes/A1',
      'DELETE /v1/automacoes/A1',
      'POST /v1/automacoes/A2/ativar',
      'POST /v1/automacoes/A2/desativar',
      'POST /v1/automacoes/A2/executar',
      'POST /v1/automacoes/A2/testar',
      'GET /v1/automacoes/A2/execucoes?estado=erro&limite=10',
      'GET /v1/execucoes?conversa_id=C1',
      'GET /v1/execucoes/X1',
      'GET /v1/automacoes/A3/sessoes?estado=ativa',
    ]);
    expect(corpo(chamadas[1]!)).toEqual(FLUXO);
    expect(corpo(chamadas[2]!)).toEqual({ nome: 'Responder', modelo: 'responder_historico' });
    expect(corpo(chamadas[9]!)).toEqual({ conversa_id: 'C1', entrada: { x: 1 }, origem: 'manual_mcp' });
    expect(corpo(chamadas[10]!)).toEqual({ mensagem: { texto: 'Quanto custa?' }, ia_simulada: true });
  });

  it('executar sem alvo envia objeto vazio', async () => {
    const { fetch, chamadas } = fetchSimulado(json({ estado: 'na_fila' }, 202));
    const exec = await novoCliente(fetch).executarAutomacao('A1');
    expect(exec.estado).toBe('na_fila');
    expect(corpo(chamadas[0]!)).toEqual({});
  });

  it('simulador de chatbot', async () => {
    const { fetch, chamadas } = fetchSimulado(
      json({ simulacao_id: 'SIM1', saidas: [], no_atual: 'n3', variaveis: {}, estado: 'ativa' }, 201),
      json({ saidas: [], no_atual: 'n4', variaveis: {}, estado: 'ativa', acoes: [] }),
      vazio(),
    );
    const c = novoCliente(fetch);
    const inicio = await c.iniciarSimulador('A3', { ia_simulada: true });
    await c.enviarAoSimulador(inicio.simulacao_id, '2');
    await c.encerrarSimulador('SIM1');

    expect(chamadas.map(resumo)).toEqual([
      'POST /v1/automacoes/A3/simulador',
      'POST /v1/simulador/SIM1/mensagens',
      'DELETE /v1/simulador/SIM1',
    ]);
    expect(corpo(chamadas[1]!)).toEqual({ texto: '2' });
  });
});

describe('Automações de IA — arquivos e compilação', () => {
  it('codifica o caminho por segmento (mantém as barras)', async () => {
    const { fetch, chamadas } = fetchSimulado(json({}), json({}), json({}, 201), vazio());
    const c = novoCliente(fetch);
    await c.lerArquivo('A1', 'index.ts');
    await c.lerArquivo('A1', 'lib/prompt de venda.md');
    await c.escreverArquivo('A1', 'lib/ação#1.ts', 'export {}', null);
    await c.excluirArquivo('A1', 'lib/x?.ts');

    expect(chamadas.map((c) => `${c.init.method} ${c.url.pathname}`)).toEqual([
      'GET /v1/automacoes/A1/arquivos/index.ts',
      'GET /v1/automacoes/A1/arquivos/lib/prompt%20de%20venda.md',
      'PUT /v1/automacoes/A1/arquivos/lib/a%C3%A7%C3%A3o%231.ts',
      'DELETE /v1/automacoes/A1/arquivos/lib/x%3F.ts',
    ]);
    expect(chamadas[3]!.url.search).toBe('');
  });

  it('hash_anterior: omitido, null (arquivo novo) ou string', async () => {
    const { fetch, chamadas } = fetchSimulado(json({}), json({}, 201), json({}));
    const c = novoCliente(fetch);
    await c.escreverArquivo('A1', 'index.ts', 'a');
    await c.escreverArquivo('A1', 'novo.ts', 'b', null);
    await c.escreverArquivo('A1', 'index.ts', 'c', 'h1');

    expect(corpo(chamadas[0]!)).toEqual({ conteudo: 'a' });
    expect(corpo(chamadas[1]!)).toEqual({ conteudo: 'b', hash_anterior: null });
    expect(corpo(chamadas[2]!)).toEqual({ conteudo: 'c', hash_anterior: 'h1' });
  });

  it('recusa caminhos com segmentos vazios, "." ou ".." sem chamar o motor', async () => {
    const { fetch, chamadas } = fetchSimulado();
    const c = novoCliente(fetch);
    for (const caminho of ['../fora.ts', 'a/../../b.ts', './index.ts', '/abs.ts', 'a//b.ts', '']) {
      await expect(c.lerArquivo('A1', caminho)).rejects.toBeInstanceOf(ErroMotor);
      await expect(c.excluirArquivo('A1', caminho)).rejects.toMatchObject({ codigo: 'validacao' });
    }
    expect(chamadas).toHaveLength(0);
  });

  it('conflito de hash traz detalhes.hash_atual', async () => {
    const { fetch } = fetchSimulado(
      erro(409, 'conflito', 'O arquivo foi alterado fora do app.', { hash_atual: 'h2' }),
    );
    const e = await novoCliente(fetch)
      .escreverArquivo('A1', 'index.ts', 'x', 'h1')
      .catch((x: unknown) => x);
    expect(e).toBeInstanceOf(ErroMotor);
    expect((e as ErroMotor).codigo).toBe('conflito');
    expect((e as ErroMotor).detalhes.hash_atual).toBe('h2');
  });

  it('demais rotas do projeto', async () => {
    const { fetch, chamadas } = fetchSimulado(json([]), json({}), json([]), json({}), json({ ok: true }));
    const c = novoCliente(fetch);
    await c.listarModelosProjeto();
    await c.obterSdkAutomacao();
    await c.listarArquivos('A1');
    await c.renomearArquivo('A1', 'a.ts', 'lib/b.ts');
    await c.compilarAutomacao('A1');

    expect(chamadas.map(resumo)).toEqual([
      'GET /v1/automacoes/modelos',
      'GET /v1/automacoes/sdk',
      'GET /v1/automacoes/A1/arquivos',
      'POST /v1/automacoes/A1/arquivos/renomear',
      'POST /v1/automacoes/A1/compilar',
    ]);
    expect(corpo(chamadas[3]!)).toEqual({ de: 'a.ts', para: 'lib/b.ts' });
  });
});

describe('Pausas, configuração, IA e segredos', () => {
  it('rotas', async () => {
    const { fetch, chamadas } = fetchSimulado(
      json({}),
      json({}),
      vazio(),
      json([]),
      json({}),
      json({}),
      json({}),
      json({}),
      json({ ok: true, modelo: 'claude-sonnet-5', latencia_ms: 300 }),
      json({}),
      json([]),
    );
    const c = novoCliente(fetch);
    await c.estadoAutomacoesConversa('C1');
    await c.pausarConversa('C1', { motivo: 'humano', duracao_min: null });
    await c.retomarConversa('C1');
    await c.listarPausas('anti_loop');
    await c.obterConfiguracaoAutomacoes();
    await c.editarConfiguracaoAutomacoes({ anti_loop_mensagens: 12, pausa_geral: true });
    await c.obterConfiguracaoIA();
    await c.editarConfiguracaoIA({ modelo_padrao: 'claude-opus-5-5' });
    await c.testarChaveIA();
    await c.testarChaveIA('claude-haiku-4-5');
    await c.listarSegredos();

    expect(chamadas.map(resumo)).toEqual([
      'GET /v1/conversas/C1/automacoes',
      'POST /v1/conversas/C1/pausa',
      'DELETE /v1/conversas/C1/pausa',
      'GET /v1/pausas?motivo=anti_loop',
      'GET /v1/automacoes/configuracao',
      'PATCH /v1/automacoes/configuracao',
      'GET /v1/ia/configuracao',
      'PATCH /v1/ia/configuracao',
      'POST /v1/ia/testar-chave',
      'POST /v1/ia/testar-chave',
      'GET /v1/segredos',
    ]);
    expect(corpo(chamadas[1]!)).toEqual({ motivo: 'humano', duracao_min: null });
    expect(corpo(chamadas[5]!)).toEqual({ anti_loop_mensagens: 12, pausa_geral: true });
    expect(corpo(chamadas[8]!)).toEqual({});
    expect(corpo(chamadas[9]!)).toEqual({ modelo: 'claude-haiku-4-5' });
  });

  it('modo falso: ia, chamadas, segredos e processar-esperas', async () => {
    const { fetch, chamadas } = fetchSimulado(json({}), json([]), json({}), json({}));
    const c = novoCliente(fetch);
    await c.falso.ia({ respostas: [{ contem: 'preço', texto: 'O valor é R$ 10' }], erro: { status: 529, vezes: 2 } });
    await c.falso.iaChamadas();
    await c.falso.segredos({ ANTHROPIC_API_KEY: 'sk-ant-teste' });
    await c.falso.processarEsperas();

    expect(chamadas.map(resumo)).toEqual([
      'PUT /v1/falso/ia',
      'GET /v1/falso/ia/chamadas',
      'POST /v1/falso/segredos',
      'POST /v1/falso/processar-esperas',
    ]);
    expect(corpo(chamadas[2]!)).toEqual({ valores: { ANTHROPIC_API_KEY: 'sk-ant-teste' } });
  });
});

describe('Erros novos', () => {
  it('definicao_invalida traz detalhes.erros (ErroDefinicao[])', async () => {
    const erros = [{ caminho: 'definicao.acoes[2].etapa_id', no_id: null, acao_id: 'a3', mensagem: 'Etapa inexistente.' }];
    const { fetch } = fetchSimulado(erro(422, 'definicao_invalida', 'A automação tem erros.', { erros }));
    const e = (await novoCliente(fetch)
      .criarAutomacao(FLUXO)
      .catch((x: unknown) => x)) as ErroMotor;
    expect(e.codigo).toBe('definicao_invalida');
    expect(e.status).toBe(422);
    expect(e.detalhes.erros).toEqual(erros);
  });

  it('compilacao_falhou traz detalhes.erros (ErroCompilacao[])', async () => {
    const erros = [{ arquivo: 'index.ts', linha: 3, coluna: 5, mensagem: 'Módulo não permitido: fs', tipo: 'importacao' }];
    const { fetch } = fetchSimulado(erro(422, 'compilacao_falhou', 'Não compilou.', { erros }));
    const e = (await novoCliente(fetch)
      .ativarAutomacao('A1')
      .catch((x: unknown) => x)) as ErroMotor;
    expect(e.codigo).toBe('compilacao_falhou');
    expect(e.detalhes.erros).toEqual(erros);
  });

  it('runner_indisponivel, ia_nao_configurada e ia_erro são reconhecidos', () => {
    expect(parsearErro(503, { erro: { codigo: 'runner_indisponivel', mensagem: 'x' } }).codigo).toBe('runner_indisponivel');
    expect(parsearErro(409, { erro: { codigo: 'ia_nao_configurada', mensagem: 'x', detalhes: { segredo: 'ANTHROPIC_API_KEY' } } }).detalhes.segredo).toBe('ANTHROPIC_API_KEY');
    const e = parsearErro(502, { erro: { codigo: 'ia_erro', mensagem: 'Chave da Anthropic inválida.', detalhes: { status: 401, request_id: 'req_1' } } });
    expect(e.codigo).toBe('ia_erro');
    expect(e.detalhes.status).toBe(401);
    expect(parsearErro(503, '').codigo).toBe('runner_indisponivel');
  });

  it('lista de códigos e de eventos inclui os da 002', () => {
    for (const c of ['definicao_invalida', 'compilacao_falhou', 'runner_indisponivel', 'ia_nao_configurada', 'ia_erro']) {
      expect(CODIGOS_ERRO).toContain(c);
    }
    const novos = [
      'funil.alterado',
      'funil.movido',
      'automacao.atualizada',
      'automacao.removida',
      'automacao.arquivos_alterados',
      'automacao.execucao.iniciada',
      'automacao.execucao.atualizada',
      'automacao.execucao.finalizada',
      'chatbot.sessao.iniciada',
      'chatbot.sessao.atualizada',
      'chatbot.sessao.finalizada',
      'conversa.pausa',
      'notificacao',
      'segredos.alterados',
      'automacoes.configuracao',
    ];
    expect(novos).toHaveLength(15);
    for (const t of novos) expect(TIPOS_EVENTO).toContain(t);
    expect(new Set(TIPOS_EVENTO).size).toBe(TIPOS_EVENTO.length);
  });
});
