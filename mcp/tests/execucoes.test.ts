// T121 — execuções, pausas, segredos (só nomes; nenhuma ferramenta define valores) e configuração.
import type { Automacao } from '@zapdesk/cliente-motor';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { type Harness, criarHarness } from './harness.js';

describe('execuções, pausas, segredos e configuração', () => {
  let h: Harness;
  beforeEach(async () => {
    h = await criarHarness();
  });
  afterEach(() => h.fechar());

  async function fluxoComExecucoes(): Promise<Automacao> {
    const a = (
      await h.chamar('criar_automacao', {
        tipo: 'fluxo',
        nome: 'Boas-vindas',
        gatilhos: [{ tipo: 'mensagem_recebida' }],
        definicao: { versao: 1, condicoes: null, acoes: [{ tipo: 'enviar_texto', texto: 'Oi!' }] },
        ativar: true,
      })
    ).estruturado as unknown as Automacao;
    h.motor.automacoes.receberMensagem('conv-a', 'olá');
    h.motor.automacoes.receberMensagem('conv-b', 'bom dia');
    await h.chamar('testar_automacao', { automacao_id: a.id, texto: 'teste' });
    return a;
  }

  it('listar_execucoes filtra por automação, estado e conversa (rotas certas)', async () => {
    const a = await fluxoComExecucoes();
    const todas = await h.chamar('listar_execucoes');
    expect((todas.estruturado?.['itens'] as unknown[]).length).toBe(3);
    expect(h.motor.chamadas('GET', '/execucoes')).toHaveLength(1);

    const daAutomacao = await h.chamar('listar_execucoes', { automacao_id: a.id, estado: 'simulacao' });
    expect((daAutomacao.estruturado?.['itens'] as unknown[]).length).toBe(1);
    expect(daAutomacao.texto).toContain('(simulação)');
    expect(h.motor.chamadas('GET', `/automacoes/${a.id}/execucoes`)).toHaveLength(1);

    const daConversa = await h.chamar('listar_execucoes', { conversa_id: 'conv-b' });
    expect((daConversa.estruturado?.['itens'] as unknown[]).length).toBe(1);

    const pagina = await h.chamar('listar_execucoes', { limite: 2 });
    expect(pagina.texto).toContain('cursor "2"');
  });

  it('ver_execucao mostra ações, log e erro com stack', async () => {
    const a = await fluxoComExecucoes();
    const exec = h.motor.automacoes.execucoes.find((e) => e.conversa_id === 'conv-a')!;
    exec.estado = 'erro';
    exec.erro = 'TypeError: x is undefined';
    exec.erro_stack = 'TypeError: x is undefined\n    at index.ts:4:10';
    const r = await h.chamar('ver_execucao', { execucao_id: exec.id });
    expect(r.erro).toBe(false);
    expect(r.estruturado).toMatchObject({ automacao_id: a.id, estado: 'erro' });
    expect(r.texto).toContain('Erro: TypeError: x is undefined');
    expect(r.texto).toContain('at index.ts:4:10');
    expect(r.texto).toContain('Log:');
    const inexistente = await h.chamar('ver_execucao', { execucao_id: 'nao-existe' });
    expect(inexistente.erro).toBe(true);
  });

  it('pausar_conversa (manual, com e sem prazo), listar_pausas e retomar_conversa', async () => {
    const conta = h.motor.adicionarConta();
    const conversa = h.motor.adicionarConversa(conta.id, '+5511911110000', 'Ana');
    const semPrazo = await h.chamar('pausar_conversa', { conversa_id: conversa.id });
    expect(semPrazo.erro, semPrazo.texto).toBe(false);
    expect(h.motor.chamadas('POST', `/conversas/${conversa.id}/pausa`)[0]?.corpo).toEqual({ motivo: 'manual', duracao_min: null });
    expect(semPrazo.texto).toContain('sem prazo');

    await h.chamar('pausar_conversa', { conversa_id: conversa.id, duracao_min: 60 });
    expect(h.motor.chamadas('POST', `/conversas/${conversa.id}/pausa`)[1]?.corpo).toEqual({ motivo: 'manual', duracao_min: 60 });

    const lista = await h.chamar('listar_pausas', { motivo: 'manual' });
    expect((lista.estruturado?.['pausas'] as unknown[]).length).toBe(1);
    expect(lista.texto).toContain(`conversa ${conversa.id}: manual`);

    const retomar = await h.chamar('retomar_conversa', { conversa_id: conversa.id });
    expect(retomar.erro).toBe(false);
    expect((await h.chamar('listar_pausas')).texto).toContain('Nenhuma conversa pausada');
  });

  it('listar_segredos devolve só nomes — nenhum valor aparece', async () => {
    h.motor.automacoes.segredos.push({ nome: 'CRM_TOKEN', reservado: false, usado_por: [{ automacao_id: 'a1', nome: 'Sincronizar CRM' }] });
    const r = await h.chamar('listar_segredos');
    expect(r.erro).toBe(false);
    expect(r.texto).toContain('ANTHROPIC_API_KEY (reservado');
    expect(r.texto).toContain('CRM_TOKEN — usado por Sincronizar CRM');
    const bruto = JSON.stringify(r);
    expect(bruto).not.toContain('sk-ant-segredo-que-nunca-sai');
    for (const s of r.estruturado?.['segredos'] as Record<string, unknown>[]) {
      expect(Object.keys(s).sort()).toEqual(['nome', 'reservado', 'usado_por']);
    }
  });

  it('nenhuma ferramenta define segredos nem altera a configuração global', async () => {
    const { tools } = await h.cliente.listTools();
    const nomes = tools.map((t) => t.name);
    expect(nomes.filter((n) => /segredo/.test(n))).toEqual(['listar_segredos']);
    expect(nomes.filter((n) => /configuracao/.test(n))).toEqual(['ver_configuracao_automacoes']);
    expect(h.motor.requisicoes.filter((r) => r.metodo !== 'GET' && /segredos|configuracao/.test(r.caminho))).toHaveLength(0);
  });

  it('ver_configuracao_automacoes junta limites e configuração de IA', async () => {
    const r = await h.chamar('ver_configuracao_automacoes');
    expect(r.erro, r.texto).toBe(false);
    expect(r.estruturado).toMatchObject({ anti_loop_mensagens: 10, pausa_geral: false, ia: { modelo_padrao: 'claude-sonnet-5', chave_configurada: true } });
    expect(r.texto).toContain('Anti-loop: 10 mensagens automáticas a cada 10 min');
    expect(r.texto).toContain('chave da Anthropic configurada');
  });
});
