// Ferramentas de execuções, pausas, segredos e configuração — specs/002-automacoes/contracts/mcp-ferramentas.md ›
// Execuções: `listar_execucoes`, `ver_execucao`, `listar_pausas`, `pausar_conversa`, `retomar_conversa`,
// `listar_segredos`, `ver_configuracao_automacoes`.
// Não há ferramenta para definir valores de segredos nem para mudar limites de segurança: só pelo app.
import type { Execucao, Pausa } from '@zapdesk/cliente-motor';
import { z } from 'zod';

import {
  entradaConversaId,
  entradaCursor,
  entradaLimite,
  saidaConfiguracaoAutomacoes,
  saidaConfiguracaoIA,
  saidaExecucao,
  saidaExecucaoDetalhe,
  saidaOk,
  saidaPagina,
  saidaPausa,
  saidaSegredo,
} from '../esquemas.js';
import { ESCRITA_LOCAL, LEITURA, type Registrador, registrarFerramenta } from '../ferramenta.js';
import { formatarData, listar, plural, responder, textoPaginacao } from '../formatar.js';
import { textoExecucao } from './automacoes.js';

const ESTADOS_EXECUCAO = ['na_fila', 'rodando', 'aguardando', 'ok', 'erro', 'simulacao', 'abortada'] as const;

function linhaExecucao(e: Execucao): string {
  const extra = e.erro ? ` — ${e.erro}` : e.motivo ? ` — ${e.motivo}` : '';
  return (
    `${formatarData(e.iniciada_em)} · ${e.automacao_nome} · ${e.estado}${e.simulacao ? ' (simulação)' : ''} · ` +
    `gatilho ${e.gatilho.tipo} · ${plural(e.acoes.length, 'ação', 'ações')}${extra} (execucao_id: ${e.id})`
  );
}

const ROTULO_MOTIVO: Record<string, string> = {
  humano: 'atendimento humano',
  anti_loop: 'anti-loop',
  manual: 'manual',
};

function linhaPausa(p: Pausa): string {
  return (
    `conversa ${p.conversa_id}: ${ROTULO_MOTIVO[p.motivo] ?? p.motivo}, ` +
    `${p.ate ? `até ${formatarData(p.ate)}` : 'sem prazo'}${p.automacao_id ? ` (automação ${p.automacao_id})` : ''}`
  );
}

export const registrarExecucoes: Registrador = (servidor, contexto) => {
  registrarFerramenta(servidor, contexto, 'listar_execucoes', {
    titulo: 'Listar execuções',
    descricao:
      'Lista execuções de automações (mais recentes primeiro): estado (ok, erro, aguardando, abortada, simulacao…), ' +
      'gatilho, ações e erro. Filtre por automacao_id, estado ou conversa_id. Detalhes e log: ver_execucao.',
    entrada: z.object({
      automacao_id: z.string().min(1).optional().describe('Só desta automação.'),
      estado: z.enum(ESTADOS_EXECUCAO).optional(),
      conversa_id: entradaConversaId.optional(),
      limite: entradaLimite.optional(),
      cursor: entradaCursor.optional(),
    }),
    saida: saidaPagina(saidaExecucao),
    anotacoes: LEITURA,
    executar: async ({ automacao_id, ...filtro }, cliente) => {
      const pagina = automacao_id
        ? await cliente.listarExecucoesAutomacao(automacao_id, filtro)
        : await cliente.listarExecucoes(filtro);
      const texto =
        pagina.itens.length === 0
          ? 'Nenhuma execução encontrada.'
          : `${plural(pagina.itens.length, 'execução', 'execuções')}:\n${listar(pagina.itens, linhaExecucao, 200)}\n${textoPaginacao(pagina.proximo_cursor)}`;
      return responder({ ...pagina }, texto);
    },
  });

  registrarFerramenta(servidor, contexto, 'ver_execucao', {
    titulo: 'Ver execução',
    descricao:
      'Mostra uma execução completa: estado, erro com stack, ações feitas (ou que seriam feitas, em simulação) com ' +
      'resultado, variáveis, retorno, tokens de IA e o log (console.log / ctx.log). Use para depurar automações.',
    entrada: z.object({ execucao_id: z.string().min(1).describe('ID da execução (veja listar_execucoes).') }),
    saida: saidaExecucaoDetalhe,
    anotacoes: LEITURA,
    executar: async ({ execucao_id }, cliente) => {
      const execucao = await cliente.obterExecucao(execucao_id);
      return responder({ ...execucao }, textoExecucao(execucao, 20_000));
    },
  });

  registrarFerramenta(servidor, contexto, 'listar_pausas', {
    titulo: 'Listar pausas de automações',
    descricao:
      'Lista as conversas em que as automações estão pausadas agora: por atendimento humano (alguém respondeu à mão), ' +
      'anti-loop (muitas mensagens automáticas seguidas) ou manual, com o prazo.',
    entrada: z.object({ motivo: z.enum(['humano', 'anti_loop', 'manual']).optional() }),
    saida: z.object({ pausas: z.array(saidaPausa) }),
    anotacoes: LEITURA,
    executar: async ({ motivo }, cliente) => {
      const pausas = await cliente.listarPausas(motivo);
      return responder(
        { pausas },
        pausas.length === 0 ? 'Nenhuma conversa pausada.' : `${plural(pausas.length, 'conversa pausada', 'conversas pausadas')}:\n${listar(pausas, linhaPausa, 200)}`,
      );
    },
  });

  registrarFerramenta(servidor, contexto, 'pausar_conversa', {
    titulo: 'Pausar automações na conversa',
    descricao:
      'Pausa todas as automações numa conversa (nenhuma mensagem automática é enviada a ela e um chatbot ativo é ' +
      'encerrado). `duracao_min` omitido = sem prazo até retomar_conversa.',
    entrada: z.object({
      conversa_id: entradaConversaId,
      duracao_min: z.number().int().min(1).max(525_600).optional().describe('Minutos de pausa; omitido = sem prazo.'),
    }),
    saida: saidaPausa,
    anotacoes: ESCRITA_LOCAL,
    executar: async ({ conversa_id, duracao_min }, cliente) => {
      const pausa = await cliente.pausarConversa(conversa_id, { motivo: 'manual', duracao_min: duracao_min ?? null });
      return responder({ ...pausa }, `Automações pausadas na ${linhaPausa(pausa)}.`);
    },
  });

  registrarFerramenta(servidor, contexto, 'retomar_conversa', {
    titulo: 'Retomar automações na conversa',
    descricao: 'Remove a pausa (humano, anti-loop ou manual) da conversa: as automações voltam a agir nela.',
    entrada: z.object({ conversa_id: entradaConversaId }),
    saida: saidaOk,
    anotacoes: { ...ESCRITA_LOCAL, idempotentHint: true },
    executar: async ({ conversa_id }, cliente) => {
      await cliente.retomarConversa(conversa_id);
      return responder({ ok: true }, 'Automações retomadas nesta conversa.');
    },
  });

  registrarFerramenta(servidor, contexto, 'listar_segredos', {
    titulo: 'Listar segredos',
    descricao:
      'Lista os NOMES dos segredos configurados no app (ex.: ANTHROPIC_API_KEY, chaves de APIs externas) e quais ' +
      'automações usam cada um. Valores nunca são mostrados. Segredos só podem ser definidos pelo usuário no app ' +
      '(Ajustes → Segredos); o código os lê com ctx.segredos.obter("NOME") se declarados em automacao.json › segredos.',
    entrada: z.object({}),
    saida: z.object({ segredos: z.array(saidaSegredo) }),
    anotacoes: LEITURA,
    executar: async (_args, cliente) => {
      const segredos = (await cliente.listarSegredos()).map((s) => ({
        nome: s.nome,
        reservado: s.reservado,
        usado_por: s.usado_por,
      }));
      const texto =
        segredos.length === 0
          ? 'Nenhum segredo configurado. O usuário pode cadastrar em Ajustes → Segredos no app.'
          : `${plural(segredos.length, 'segredo', 'segredos')} (só nomes):\n${listar(
              segredos,
              (s) =>
                `${s.nome}${s.reservado ? ' (reservado: chave da IA)' : ''}` +
                (s.usado_por.length > 0 ? ` — usado por ${s.usado_por.map((u) => u.nome).join(', ')}` : ''),
              200,
            )}`;
      return responder({ segredos }, texto);
    },
  });

  registrarFerramenta(servidor, contexto, 'ver_configuracao_automacoes', {
    titulo: 'Ver configuração das automações',
    descricao:
      'Mostra os limites de segurança e de IA: anti-loop (mensagens automáticas por janela), pausas automáticas, ' +
      'primeiros contatos por hora, tempo/memória/processos das automações de IA, pausa geral, modelo de IA padrão e ' +
      'se a chave da Anthropic está configurada. Só leitura: mudanças são feitas pelo usuário no app.',
    entrada: z.object({}),
    saida: saidaConfiguracaoAutomacoes.extend({ ia: saidaConfiguracaoIA }),
    anotacoes: LEITURA,
    executar: async (_args, cliente) => {
      const [config, ia] = await Promise.all([cliente.obterConfiguracaoAutomacoes(), cliente.obterConfiguracaoIA()]);
      const texto = [
        config.pausa_geral ? 'PAUSA GERAL LIGADA: nenhuma automação está agindo.' : 'Automações ligadas (sem pausa geral).',
        `Anti-loop: ${config.anti_loop_mensagens} mensagens automáticas a cada ${config.anti_loop_janela_min} min por conversa; ` +
          `estouro pausa a conversa por ${config.pausa_anti_loop_min} min.`,
        `Resposta manual pausa as automações da conversa por ${config.pausa_humana_min} min.`,
        `Primeiros contatos: ${config.primeiros_contatos_hora} por hora por conta.`,
        `IA: ${config.tempo_ia_s} s e ${config.memoria_ia_mb} MB por execução, até ${config.processos_ia_max} processos, ` +
          `ociosidade ${config.ociosidade_ia_min} min.`,
        `Modelo padrão: ${ia.modelo_padrao} · chave da Anthropic ${ia.chave_configurada ? 'configurada' : 'NÃO configurada (use ia_simulada nos testes)'}.`,
        `Modelos: ${ia.modelos.map((m) => m.id).join(', ')}`,
      ].join('\n');
      return responder({ ...config, ia }, texto);
    },
  });
};
