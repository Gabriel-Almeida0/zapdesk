// T119 — ferramentas de automações (fluxo e chatbot) contra o motor simulado.
import type { Automacao, ExecucaoDetalhe } from '@zapdesk/cliente-motor';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { type Harness, criarHarness } from './harness.js';

const FLUXO = {
  tipo: 'fluxo',
  nome: 'Respondeu → quente',
  gatilhos: [{ tipo: 'palavra_chave', palavras: ['preço'] }],
  definicao: {
    versao: 1,
    condicoes: null,
    acoes: [
      { tipo: 'enviar_texto', texto: 'Oi {primeiro_nome}! Já te mando os preços.' },
      { tipo: 'notificar', titulo: 'Lead quente', texto: '{nome} pediu preço' },
    ],
  },
};

const CHATBOT = {
  tipo: 'chatbot',
  nome: 'Atendimento',
  gatilhos: [{ tipo: 'mensagem_recebida', primeira_mensagem: true }],
  definicao: {
    versao: 1,
    inicio: 'n1',
    nao_entendi: 'Não entendi. Responda com uma das opções.',
    max_tentativas: 3,
    inatividade_min: 30,
    nos: [
      { id: 'n1', tipo: 'inicio', proximo: 'n2' },
      { id: 'n2', tipo: 'mensagem', texto: 'Olá!', proximo: 'n3' },
      {
        id: 'n3',
        tipo: 'menu',
        texto: 'Como posso ajudar?',
        mostrar_numeros: true,
        opcoes: [
          { rotulo: 'Preços', valores: ['preco'], proximo: 'n4' },
          { rotulo: 'Falar com vendedor', valores: [], proximo: 'n8' },
        ],
      },
      {
        id: 'n4',
        tipo: 'pergunta',
        texto: 'Qual seu e-mail?',
        variavel: 'email',
        validacao: { tipo: 'email', mensagem_erro: 'E-mail inválido. Tente de novo.' },
        proximo: 'n9',
      },
      { id: 'n8', tipo: 'humano', mensagem: 'Vou chamar um atendente.' },
      { id: 'n9', tipo: 'fim', mensagem: 'Obrigado, {email}!' },
    ],
  },
};

describe('automações: fluxo e chatbot', () => {
  let h: Harness;
  beforeEach(async () => {
    h = await criarHarness();
  });
  afterEach(() => h.fechar());

  it('ver_formatos_automacao devolve a referência com gatilhos, ações, chatbot e manifesto', async () => {
    const r = await h.chamar('ver_formatos_automacao');
    expect(r.erro).toBe(false);
    for (const trecho of ['palavra_chave', 'entrou_etapa', 'mover_etapa', 'executar_ia', '"tipo":"menu"', 'automacao.json', 'aoReceberMensagem']) {
      expect(r.texto, trecho).toContain(trecho);
    }
  });

  it('criar_automacao com ativar: true cria e ativa o fluxo', async () => {
    const r = await h.chamar('criar_automacao', { ...FLUXO, ativar: true });
    expect(r.erro, r.texto).toBe(false);
    const a = r.estruturado as unknown as Automacao;
    expect(a.ativa).toBe(true);
    expect(h.motor.chamadas('POST', '/automacoes')[0]?.corpo).toMatchObject({ tipo: 'fluxo', nome: 'Respondeu → quente' });
    expect(h.motor.chamadas('POST', `/automacoes/${a.id}/ativar`)).toHaveLength(1);
    expect(r.texto).toContain('ATIVA');

    const lista = await h.chamar('listar_automacoes', { ativa: true });
    expect(lista.texto).toContain('palavra-chave (preço)');
    const ver = await h.chamar('ver_automacao', { automacao_id: a.id });
    expect(ver.texto).toContain('"enviar_texto"');
  });

  it('criar_automacao sem ativar nasce inativa e orienta o teste', async () => {
    const r = await h.chamar('criar_automacao', FLUXO);
    expect((r.estruturado as unknown as Automacao).ativa).toBe(false);
    expect(r.texto).toContain('testar_automacao');
  });

  it('criar_automacao com definição inválida devolve os caminhos dos campos', async () => {
    const r = await h.chamar('criar_automacao', {
      ...FLUXO,
      definicao: { versao: 1, condicoes: null, acoes: [{ tipo: 'teletransportar' }] },
    });
    expect(r.erro).toBe(true);
    expect(r.texto).toContain('definicao.acoes[0].tipo: Tipo de ação desconhecido');
    expect(r.texto).toContain('ver_formatos_automacao');
  });

  it('validar_automacao lista erros e avisos sem gravar', async () => {
    const r = await h.chamar('validar_automacao', {
      ...FLUXO,
      definicao: {
        versao: 1,
        condicoes: null,
        acoes: [{ id: 'a1', tipo: 'enviar_texto', texto: '' }, { tipo: 'adicionar_etiqueta', etiqueta_id: 'nao-existe' }],
      },
    });
    expect(r.erro).toBe(false);
    expect((r.estruturado?.['erros'] as unknown[]).length).toBe(1);
    expect(r.texto).toContain('definicao.acoes[0].texto (ação a1): Escreva o texto');
    expect(r.texto).toContain('1 aviso');
    expect(r.texto).toContain('Etiqueta nao-existe não existe');
    expect(h.motor.automacoes.automacoes).toHaveLength(0);
  });

  it('testar_automacao simula o fluxo sem enviar nada', async () => {
    const a = (await h.chamar('criar_automacao', FLUXO)).estruturado as unknown as Automacao;
    const r = await h.chamar('testar_automacao', { automacao_id: a.id, texto: 'Qual o preço?' });
    expect(r.erro, r.texto).toBe(false);
    const exec = r.estruturado as unknown as ExecucaoDetalhe;
    expect(exec.estado).toBe('simulacao');
    expect(exec.acoes.map((x) => x.resultado)).toEqual(['simulada', 'simulada']);
    expect(r.texto).toContain('simulação — nada foi enviado');
    expect(r.texto).toContain('Ações que seriam feitas');
    expect(h.motor.chamadas('POST', `/automacoes/${a.id}/testar`)[0]?.corpo).toEqual({ mensagem: { texto: 'Qual o preço?' } });
    expect(h.motor.automacoes.enviadas).toHaveLength(0);
  });

  it('testar_automacao em chatbot orienta a usar o simulador', async () => {
    const a = (await h.chamar('criar_automacao', CHATBOT)).estruturado as unknown as Automacao;
    const r = await h.chamar('testar_automacao', { automacao_id: a.id, texto: 'oi' });
    expect(r.erro).toBe(true);
    expect(r.texto).toContain('chat simulado');
  });

  it('simular_chatbot percorre a sequência: menu → pergunta inválida → válida → fim', async () => {
    const criado = await h.chamar('criar_automacao', CHATBOT);
    expect(criado.erro, criado.texto).toBe(false);
    const a = criado.estruturado as unknown as Automacao;
    const r = await h.chamar('simular_chatbot', { automacao_id: a.id, mensagens: ['1', 'sem-arroba', 'ana@x.com', 'depois do fim'] });
    expect(r.erro, r.texto).toBe(false);
    const rodadas = r.estruturado?.['rodadas'] as { entrada: string | null; no_atual: string; estado: string; variaveis: Record<string, string> }[];
    expect(rodadas.map((x) => x.no_atual)).toEqual(['n3', 'n4', 'n4', 'n9']);
    expect(rodadas.at(-1)).toMatchObject({ estado: 'concluida', variaveis: { email: 'ana@x.com' } });
    expect(r.texto).toContain('1 - Preços');
    expect(r.texto).toContain('E-mail inválido');
    expect(r.texto).toContain('Obrigado, ana@x.com!');
    expect(r.texto).toContain('antes de 1 mensagem');
    // a sessão simulada sempre é encerrada
    expect(h.motor.chamadas('DELETE', /^\/simulador\//)).toHaveLength(1);
  });

  it('simular_chatbot pelo caminho do vendedor termina em humano', async () => {
    const a = (await h.chamar('criar_automacao', CHATBOT)).estruturado as unknown as Automacao;
    const r = await h.chamar('simular_chatbot', { automacao_id: a.id, mensagens: ['falar com vendedor'] });
    expect(r.texto).toContain('Vou chamar um atendente.');
    expect(r.texto).toContain('estado humano');
  });

  it('executar_automacao manda origem manual_mcp e devolve o resultado', async () => {
    const conta = h.motor.adicionarConta();
    const a = (await h.chamar('criar_automacao', FLUXO)).estruturado as unknown as Automacao;
    const r = await h.chamar('executar_automacao', { automacao_id: a.id, telefone: '11 95555-0000' });
    expect(r.erro, r.texto).toBe(false);
    expect(h.motor.chamadas('POST', `/automacoes/${a.id}/executar`)[0]?.corpo).toEqual({
      telefone: '11 95555-0000',
      conta_id: conta.id,
      origem: 'manual_mcp',
    });
    expect(r.estruturado).toMatchObject({ estado: 'ok', origem: 'manual_mcp' });
    const dois = await h.chamar('executar_automacao', { automacao_id: a.id, lead_id: 'x', conversa_id: 'y' });
    expect(dois.erro).toBe(true);
    expect(dois.texto).toContain('no máximo um alvo');
  });

  it('editar_automacao cria versão nova; ativar/desativar; excluir', async () => {
    const a = (await h.chamar('criar_automacao', FLUXO)).estruturado as unknown as Automacao;
    const r = await h.chamar('editar_automacao', { automacao_id: a.id, gatilhos: [{ tipo: 'manual' }], ativar: true });
    expect(r.erro, r.texto).toBe(false);
    expect(r.estruturado).toMatchObject({ versao: 2, ativa: true });
    const d = await h.chamar('desativar_automacao', { automacao_id: a.id });
    expect(d.estruturado).toMatchObject({ ativa: false });
    const at = await h.chamar('ativar_automacao', { automacao_id: a.id });
    expect(at.estruturado).toMatchObject({ ativa: true });
    const x = await h.chamar('excluir_automacao', { automacao_id: a.id });
    expect(x.erro).toBe(false);
    expect(h.motor.automacoes.automacoes).toHaveLength(0);
  });
});
