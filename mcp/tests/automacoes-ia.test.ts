// T120 — ciclo de programação de uma automação de IA só pelo MCP (motor simulado; a versão contra o
// motor real via stdio está em integracao-automacoes.test.ts):
// criar_automacao_ia → escrever_arquivo_automacao → compilar_automacao com erro (arquivo:linha:coluna)
// e sem erro → testar_automacao (nada enviado) → ativar_automacao → mensagem recebida →
// listar_execucoes/ver_execucao ok; ver_tipos_sdk devolve o .d.ts.
import type { ArquivoProjeto, Automacao, ConteudoArquivo, ExecucaoDetalhe, ResultadoCompilacao } from '@zapdesk/cliente-motor';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { type Harness, criarHarness } from './harness.js';
import { CODIGO_COM_ERRO, CODIGO_OK, MANIFESTO_RESPONDER } from './projeto-ia-exemplo.js';

describe('automações de IA: ciclo completo pelo MCP', () => {
  let h: Harness;
  beforeEach(async () => {
    h = await criarHarness();
  });
  afterEach(() => h.fechar());

  it('ver_tipos_sdk devolve o resumo do ctx e o .d.ts completo', async () => {
    const r = await h.chamar('ver_tipos_sdk');
    expect(r.erro, r.texto).toBe(false);
    expect(r.estruturado?.['tipos']).toContain("declare module '@zapdesk/automacao'");
    expect(r.texto).toContain('definirAutomacao');
    expect(r.texto).toContain('ctx.ia.gerar');
    expect(r.texto).toContain('===== index.d.ts =====');
  });

  it('listar_modelos_automacao_ia lista os quatro modelos', async () => {
    const r = await h.chamar('listar_modelos_automacao_ia');
    expect((r.estruturado?.['modelos'] as unknown[]).length).toBe(4);
    expect(r.texto).toContain('responder_historico');
  });

  it('cria, programa, compila com erro e sem erro, testa sem enviar, ativa e vê a execução', async () => {
    // 1. criar
    const criada = await h.chamar('criar_automacao_ia', { nome: 'Responder dúvidas' });
    expect(criada.erro, criada.texto).toBe(false);
    const automacao = criada.estruturado as unknown as Automacao & { arquivos: ArquivoProjeto[] };
    expect(automacao.tipo).toBe('ia');
    expect(automacao.ativa).toBe(false);
    expect(automacao.arquivos.map((a) => a.caminho).sort()).toEqual(['automacao.json', 'index.ts']);
    expect(criada.texto).toContain('--- index.ts');
    expect(criada.texto).toContain('definirAutomacao');
    expect(h.motor.chamadas('POST', '/automacoes/ia')[0]?.corpo).toEqual({ nome: 'Responder dúvidas', modelo: 'em_branco' });
    const id = automacao.id;

    // 2. escrever manifesto, prompt e código com erro de sintaxe
    const lido = (await h.chamar('ler_arquivo_automacao', { automacao_id: id, caminho: 'automacao.json' }))
      .estruturado as unknown as ConteudoArquivo;
    const manifesto = await h.chamar('escrever_arquivo_automacao', {
      automacao_id: id,
      caminho: 'automacao.json',
      conteudo: MANIFESTO_RESPONDER,
      hash_anterior: lido.hash,
    });
    expect(manifesto.erro, manifesto.texto).toBe(false);
    expect(manifesto.texto).toContain('compilar_automacao');
    await h.chamar('escrever_arquivo_automacao', {
      automacao_id: id,
      caminho: 'prompt.md',
      conteudo: 'Responda curto, em português. Nunca invente preços.',
      hash_anterior: null,
    });
    await h.chamar('escrever_arquivo_automacao', { automacao_id: id, caminho: 'index.ts', conteudo: CODIGO_COM_ERRO });

    // 3. compilar com erro → arquivo:linha:coluna
    const comErro = await h.chamar('compilar_automacao', { automacao_id: id });
    expect(comErro.erro).toBe(false);
    const resultadoErro = comErro.estruturado as unknown as ResultadoCompilacao;
    expect(resultadoErro.ok).toBe(false);
    expect(resultadoErro.erros[0]).toMatchObject({ arquivo: 'index.ts', linha: 6, tipo: 'sintaxe' });
    expect(comErro.texto).toMatch(/NÃO compilou: 1 erro\.\n- index\.ts:6:\d+ \[sintaxe\] /);

    // ativar com erro de compilação é recusado com a mesma lista
    const ativarErro = await h.chamar('ativar_automacao', { automacao_id: id });
    expect(ativarErro.erro).toBe(true);
    expect(ativarErro.texto).toMatch(/index\.ts:6:\d+ \[sintaxe\]/);

    // 4. corrigir e compilar sem erro
    await h.chamar('escrever_arquivo_automacao', { automacao_id: id, caminho: 'index.ts', conteudo: CODIGO_OK });
    const ok = await h.chamar('compilar_automacao', { automacao_id: id });
    expect(ok.estruturado).toMatchObject({ ok: true, handlers: ['aoReceberMensagem'] });
    expect(ok.texto).toContain('Compilou sem erros');

    // 5. testar em simulação: nada é enviado
    const teste = await h.chamar('testar_automacao', { automacao_id: id, texto: 'Vocês entregam no sábado?', ia_simulada: true });
    expect(teste.erro, teste.texto).toBe(false);
    expect((teste.estruturado as unknown as ExecucaoDetalhe).estado).toBe('simulacao');
    expect(teste.texto).toContain('nada foi enviado');
    expect(h.motor.automacoes.enviadas).toHaveLength(0);

    // 6. ativar, receber mensagem e ver a execução
    const ativa = await h.chamar('ativar_automacao', { automacao_id: id });
    expect(ativa.erro, ativa.texto).toBe(false);
    expect(ativa.estruturado).toMatchObject({ ativa: true });
    h.motor.automacoes.receberMensagem('conv-1', 'Vocês entregam no sábado?');

    const execucoes = await h.chamar('listar_execucoes', { automacao_id: id, estado: 'ok' });
    const itens = execucoes.estruturado?.['itens'] as { id: string; estado: string }[];
    expect(itens).toHaveLength(1);
    expect(execucoes.texto).toContain('mensagem_recebida');
    const ver = await h.chamar('ver_execucao', { execucao_id: itens[0]!.id });
    expect(ver.erro).toBe(false);
    expect(ver.estruturado).toMatchObject({ estado: 'ok' });
    expect(ver.texto).toContain('responder → conv-1: ok');
    expect(ver.texto).toContain('Log:');
  });

  it('manifesto com gatilho sem handler vira erro de manifesto', async () => {
    const { id } = (await h.chamar('criar_automacao_ia', { nome: 'Sem handler' })).estruturado as unknown as Automacao;
    await h.chamar('escrever_arquivo_automacao', { automacao_id: id, caminho: 'automacao.json', conteudo: MANIFESTO_RESPONDER });
    const r = await h.chamar('compilar_automacao', { automacao_id: id });
    expect(r.texto).toContain('[manifesto] O gatilho mensagem_recebida exige o handler aoReceberMensagem');
  });

  it('import de módulo proibido vira erro de importação', async () => {
    const { id } = (await h.chamar('criar_automacao_ia', { nome: 'Proibido' })).estruturado as unknown as Automacao;
    await h.chamar('escrever_arquivo_automacao', {
      automacao_id: id,
      caminho: 'index.ts',
      conteudo: "import { readFileSync } from 'node:fs';\nexport default { readFileSync };\n",
    });
    const r = await h.chamar('compilar_automacao', { automacao_id: id });
    expect(r.texto).toMatch(/index\.ts:1:\d+ \[importacao\] Módulo não permitido: node:fs/);
  });

  it('escrever com hash desatualizado devolve conflito com orientação', async () => {
    const { id } = (await h.chamar('criar_automacao_ia', { nome: 'Conflito' })).estruturado as unknown as Automacao;
    const r = await h.chamar('escrever_arquivo_automacao', {
      automacao_id: id,
      caminho: 'index.ts',
      conteudo: '// novo',
      hash_anterior: 'hash-velho',
    });
    expect(r.erro).toBe(true);
    expect(r.texto).toContain('alterado fora do app');
    expect(r.texto).toContain('ler_arquivo_automacao');
  });

  it('listar, renomear e excluir arquivos; o manifesto não pode ser excluído', async () => {
    const { id } = (await h.chamar('criar_automacao_ia', { nome: 'Arquivos' })).estruturado as unknown as Automacao;
    await h.chamar('escrever_arquivo_automacao', { automacao_id: id, caminho: 'lib/util.ts', conteudo: 'export const x = 1;\n' });
    const ren = await h.chamar('renomear_arquivo_automacao', { automacao_id: id, de: 'lib/util.ts', para: 'lib/ajuda.ts' });
    expect(ren.erro, ren.texto).toBe(false);
    const lista = await h.chamar('listar_arquivos_automacao', { automacao_id: id });
    expect((lista.estruturado?.['arquivos'] as ArquivoProjeto[]).map((a) => a.caminho)).toEqual(['automacao.json', 'index.ts', 'lib/ajuda.ts']);
    expect(h.motor.chamadas('GET', `/automacoes/${id}/arquivos/lib/ajuda.ts`)).toHaveLength(0);
    const ex = await h.chamar('excluir_arquivo_automacao', { automacao_id: id, caminho: 'lib/ajuda.ts' });
    expect(ex.erro).toBe(false);
    const manifesto = await h.chamar('excluir_arquivo_automacao', { automacao_id: id, caminho: 'automacao.json' });
    expect(manifesto.erro).toBe(true);
  });

  it('editar_automacao numa automação de IA orienta a editar automacao.json', async () => {
    const { id } = (await h.chamar('criar_automacao_ia', { nome: 'Editar' })).estruturado as unknown as Automacao;
    const r = await h.chamar('editar_automacao', { automacao_id: id, nome: 'Outro' });
    expect(r.erro).toBe(true);
    expect(r.texto).toContain('automacao.json');
  });

  it('executar_automacao de IA entrega a entrada a aoExecutar e devolve o retorno', async () => {
    const { id } = (await h.chamar('criar_automacao_ia', { nome: 'Manual' })).estruturado as unknown as Automacao;
    const r = await h.chamar('executar_automacao', { automacao_id: id, entrada: { pergunta: 'oi' } });
    expect(r.erro, r.texto).toBe(false);
    expect(h.motor.chamadas('POST', `/automacoes/${id}/executar`)[0]?.corpo).toEqual({ entrada: { pergunta: 'oi' }, origem: 'manual_mcp' });
    expect(r.texto).toContain('Retorno: {"recebido":{"pergunta":"oi"}}');
  });
});
