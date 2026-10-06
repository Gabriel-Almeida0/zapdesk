// T133 — conversas, mensagens e status contra o motor simulado.
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import type { Conta, Conversa } from '@zapdesk/cliente-motor';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { type Harness, criarHarness } from './harness.js';

describe('conversas e mensagens', () => {
  let h: Harness;
  let conta: Conta;
  let ana: Conversa;
  let pasta: string;

  beforeEach(async () => {
    h = await criarHarness();
    conta = h.motor.adicionarConta();
    ana = h.motor.adicionarConversa(conta.id, '+5511911112222', 'Ana', 2);
    h.motor.adicionarMensagem(ana, 'Oi, tudo bem?');
    h.motor.adicionarMensagem(ana, 'Tudo! Quero saber o preço', true);
    h.motor.adicionarMensagem(ana, 'Qual o preço da ação?');
    h.motor.adicionarConversa(conta.id, '+5511933334444', 'Beto');
    pasta = await mkdtemp(join(tmpdir(), 'zapdesk-conv-'));
  });
  afterEach(async () => {
    await h.fechar();
    await rm(pasta, { recursive: true, force: true });
  });

  it('listar_conversas usa a conta padrão e aplica filtros', async () => {
    const todas = await h.chamar('listar_conversas');
    expect(todas.erro).toBe(false);
    expect((todas.estruturado?.['itens'] as unknown[]).length).toBe(2);
    expect(todas.texto).toContain(`conversa_id: ${ana.id}`);

    const naoLidas = await h.chamar('listar_conversas', { nao_lidas: true });
    expect((naoLidas.estruturado?.['itens'] as Conversa[]).map((c) => c.nome)).toEqual(['Ana']);
    expect(h.motor.chamadas('GET', `/contas/${conta.id}/conversas`).length).toBe(2);
  });

  it('conta padrão: com duas contas conectadas pede conta_id listando as contas', async () => {
    h.motor.adicionarConta({ nome: 'Pessoal', telefone: '+5511900000009' });
    const r = await h.chamar('listar_conversas');
    expect(r.erro).toBe(true);
    expect(r.texto).toContain('informe conta_id');
    expect(r.texto).toContain('Pessoal');
    const comConta = await h.chamar('listar_conversas', { conta_id: conta.id });
    expect(comConta.erro).toBe(false);
  });

  it('conta padrão: sem conta conectada explica e lista', async () => {
    conta.estado = 'desconectada';
    const r = await h.chamar('listar_conversas');
    expect(r.erro).toBe(true);
    expect(r.texto).toContain('Nenhuma conta está conectada');
  });

  it('ler_mensagens por conversa_id devolve ordem cronológica', async () => {
    const r = await h.chamar('ler_mensagens', { conversa_id: ana.id });
    expect(r.erro).toBe(false);
    const mensagens = r.estruturado?.['mensagens'] as { texto: string; de_mim: boolean }[];
    expect(mensagens.map((m) => m.texto)).toEqual(['Oi, tudo bem?', 'Tudo! Quero saber o preço', 'Qual o preço da ação?']);
    expect(r.texto).toMatch(/Ana: Oi, tudo bem\?/);
    expect(r.texto).toMatch(/Você: Tudo! Quero saber o preço \(enviada\)/);
    expect(r.estruturado?.['mais_antigas_antes']).toBeNull();
    // padrão de 30 mensagens
    expect(h.motor.chamadas('GET', `/conversas/${ana.id}/mensagens`)).toHaveLength(1);
  });

  it('ler_mensagens por telefone (qualquer formato) e paginação com antes', async () => {
    const r = await h.chamar('ler_mensagens', { telefone: '(11) 91111-2222', limite: 2 });
    expect(r.erro).toBe(false);
    expect((r.estruturado?.['conversa'] as Conversa).id).toBe(ana.id);
    const mensagens = r.estruturado?.['mensagens'] as { id: string; texto: string }[];
    expect(mensagens.map((m) => m.texto)).toEqual(['Tudo! Quero saber o preço', 'Qual o preço da ação?']);
    const antes = r.estruturado?.['mais_antigas_antes'] as string;
    expect(antes).toBe(mensagens[0]?.id);

    const anteriores = await h.chamar('ler_mensagens', { conversa_id: ana.id, antes, limite: 2 });
    expect((anteriores.estruturado?.['mensagens'] as { texto: string }[]).map((m) => m.texto)).toEqual(['Oi, tudo bem?']);
  });

  it('ler_mensagens por telefone acha a conversa existente sem criar (funciona com a conta desconectada)', async () => {
    conta.estado = 'desconectada';
    const r = await h.chamar('ler_mensagens', { telefone: '+55 11 91111-2222', conta_id: conta.id });
    expect(r.erro).toBe(false);
    expect((r.estruturado?.['conversa'] as Conversa).id).toBe(ana.id);
    expect(h.motor.chamadas('POST', `/contas/${conta.id}/conversas`)).toHaveLength(0);
  });

  it('ler_mensagens sem conversa_id nem telefone é erro de uso; conversa inexistente vem do motor', async () => {
    expect((await h.chamar('ler_mensagens', {})).erro).toBe(true);
    const r = await h.chamar('ler_mensagens', { conversa_id: 'nao-existe' });
    expect(r.erro).toBe(true);
    expect(r.texto).toContain('não encontrado');
  });

  it('buscar_mensagens', async () => {
    const r = await h.chamar('buscar_mensagens', { termo: 'preço' });
    expect(r.erro).toBe(false);
    expect((r.estruturado?.['itens'] as unknown[]).length).toBe(2);
    expect(r.texto).toContain('2 resultados para "preço"');
    expect(h.motor.chamadas('GET', `/contas/${conta.id}/mensagens/busca`)).toHaveLength(1);
  });

  it('enviar_mensagem de texto para um telefone novo abre a conversa e envia', async () => {
    const r = await h.chamar('enviar_mensagem', { telefone: '+55 11 95555-6666', texto: 'Olá!' });
    expect(r.erro).toBe(false);
    expect(r.estruturado).toMatchObject({ texto: 'Olá!', de_mim: true, estado: 'pendente' });
    expect(h.motor.chamadas('POST', `/contas/${conta.id}/conversas`)[0]?.corpo).toEqual({ telefone: '+55 11 95555-6666' });
    const envio = h.motor.chamadas('POST', /^\/conversas\/[^/]+\/mensagens$/)[0]?.corpo;
    expect(envio).toMatchObject({ texto: 'Olá!', arquivo_id: null, como: 'auto' });
    expect(r.texto).toContain('Enviei mensagem');
  });

  it('enviar_mensagem com anexo sobe o arquivo pelo caminho e envia como voz', async () => {
    const audio = join(pasta, 'recado.ogg');
    await writeFile(audio, 'fake');
    const r = await h.chamar('enviar_mensagem', { conversa_id: ana.id, caminho_anexo: audio, como: 'voz' });
    expect(r.erro).toBe(false);
    expect(h.motor.chamadas('POST', '/arquivos')[0]?.corpo).toEqual({ caminho: audio });
    const envio = h.motor.chamadas('POST', `/conversas/${ana.id}/mensagens`)[0]?.corpo;
    expect(envio).toMatchObject({ como: 'voz', texto: null });
    expect(envio?.['arquivo_id']).toBeTruthy();
    expect(r.texto).toContain('audio "recado.ogg"');
  });

  it('enviar_mensagem: sem conteúdo é erro; número sem WhatsApp e conta desconectada vêm do motor', async () => {
    expect((await h.chamar('enviar_mensagem', { telefone: '11 95555-6666' })).erro).toBe(true);

    h.motor.semWhatsApp.add('+5511977778888');
    const sem = await h.chamar('enviar_mensagem', { telefone: '11 97777-8888', texto: 'oi' });
    expect(sem.erro).toBe(true);
    expect(sem.texto).toContain('não tem WhatsApp');

    const anexoInexistente = await h.chamar('enviar_mensagem', { conversa_id: ana.id, caminho_anexo: join(pasta, 'x.pdf') });
    expect(anexoInexistente.erro).toBe(true);
    expect(anexoInexistente.texto).toContain('caminho: Arquivo não encontrado.');

    conta.estado = 'desconectada';
    const desconectada = await h.chamar('enviar_mensagem', { telefone: '11 95555-6666', texto: 'oi', conta_id: conta.id });
    expect(desconectada.erro).toBe(true);
    expect(desconectada.texto).toContain('reconecte-a pela interface');
  });

  it('reagir_mensagem, editar_mensagem e apagar_mensagem', async () => {
    const minha = h.motor.adicionarMensagem(ana, 'Texto com erro', true);
    const deles = h.motor.mensagens[0]!;

    const reagir = await h.chamar('reagir_mensagem', { mensagem_id: deles.id, emoji: '👍' });
    expect(reagir).toMatchObject({ erro: false, estruturado: { ok: true } });
    expect(h.motor.reacoes).toEqual([{ mensagem_id: deles.id, emoji: '👍' }]);
    const remover = await h.chamar('reagir_mensagem', { mensagem_id: deles.id, emoji: '' });
    expect(remover.texto).toBe('Reação removida.');

    const editar = await h.chamar('editar_mensagem', { mensagem_id: minha.id, texto: 'Texto certo' });
    expect(editar.erro).toBe(false);
    expect(editar.estruturado).toMatchObject({ texto: 'Texto certo', editada: true });

    const editarDeles = await h.chamar('editar_mensagem', { mensagem_id: deles.id, texto: 'x' });
    expect(editarDeles.erro).toBe(true);

    minha.pode_apagar = false;
    const foraDoPrazo = await h.chamar('apagar_mensagem', { mensagem_id: minha.id });
    expect(foraDoPrazo.erro).toBe(true);
    expect(foraDoPrazo.texto).toContain('prazo');
    minha.pode_apagar = true;
    const apagar = await h.chamar('apagar_mensagem', { mensagem_id: minha.id });
    expect(apagar).toMatchObject({ erro: false, estruturado: { ok: true } });
    expect(minha.apagada).toBe(true);
  });

  it('marcar_como_lida', async () => {
    const r = await h.chamar('marcar_como_lida', { conversa_id: ana.id });
    expect(r).toMatchObject({ erro: false, estruturado: { ok: true } });
    expect(ana.nao_lidas).toBe(0);
  });

  it('ver_status agrupa por contato', async () => {
    h.motor.status = [
      {
        contato_jid: ana.jid,
        contato_nome: 'Ana',
        itens: [
          {
            id: 's1',
            conta_id: conta.id,
            contato_jid: ana.jid,
            contato_nome: 'Ana',
            tipo: 'texto',
            texto: 'Promoção hoje!',
            midia: null,
            publicado_em: '2026-09-27T08:00:00-03:00',
          },
        ],
      },
    ];
    const r = await h.chamar('ver_status');
    expect(r.erro).toBe(false);
    expect((r.estruturado?.['status_por_contato'] as unknown[]).length).toBe(1);
    expect(r.texto).toContain('Ana: [27/09/2026 08:00 texto] Promoção hoje!');

    h.motor.status = [];
    expect((await h.chamar('ver_status')).texto).toContain('Nenhum status');
  });
});
