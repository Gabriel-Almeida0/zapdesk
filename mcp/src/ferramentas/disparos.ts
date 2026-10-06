// Ferramentas de disparos: `criar_disparo`, `listar_disparos`, `ver_disparo`, `iniciar_disparo`,
// `pausar_disparo`, `retomar_disparo`, `cancelar_disparo`, `exportar_relatorio`.
import { mkdir, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';

import type { ClienteMotor, Disparo, NovoDisparo } from '@zapdesk/cliente-motor';
import { z } from 'zod';

import { contaPadrao } from '../conta-padrao.js';
import {
  entradaCaminhoArquivo,
  entradaContaId,
  entradaCursor,
  entradaLeadNovo,
  entradaLimite,
  entradaValoresPadrao,
  saidaDestinatario,
  saidaDisparo,
  saidaPagina,
  saidaRelatorioImportacao,
} from '../esquemas.js';
import {
  ENVIO_WHATSAPP,
  ESCRITA_LOCAL,
  LEITURA,
  type Registrador,
  caminhoAbsoluto,
  registrarFerramenta,
} from '../ferramenta.js';
import { ErroFerramenta, formatarData, listar, plural, responder, textoPaginacao } from '../formatar.js';
import { textoRelatorioImportacao } from './leads.js';

const ESTADOS_DISPARO = [
  'rascunho',
  'agendado',
  'enviando',
  'fora_da_janela',
  'pausado',
  'concluido',
  'cancelado',
] as const;
const ESTADOS_DESTINATARIO = [
  'pendente',
  'enviando',
  'enviado',
  'entregue',
  'lido',
  'falhou',
  'respondeu',
] as const;

const ROTULO_ESTADO: Record<string, string> = {
  rascunho: 'rascunho',
  agendado: 'agendado',
  enviando: 'enviando',
  fora_da_janela: 'fora da janela de horário (retoma sozinho)',
  pausado: 'pausado',
  concluido: 'concluído',
  cancelado: 'cancelado',
};

const entradaDisparoId = z.string().min(1).describe('ID do disparo (veja listar_disparos).');
const horario = z
  .string()
  .regex(/^([01]\d|2[0-3]):[0-5]\d$/, 'Use HH:MM, ex.: "09:00".');

function textoContadores(d: Disparo): string {
  const c = d.contadores;
  return (
    `${plural(c.total, 'destinatário', 'destinatários')}: ${plural(c.enviado + c.entregue + c.lido + c.respondeu, 'enviado', 'enviados')} ` +
    `(${plural(c.entregue, 'entregue', 'entregues')}, ${plural(c.lido, 'lido', 'lidos')}, ${plural(c.respondeu, 'respondeu', 'responderam')}), ` +
    `${plural(c.pendente + c.enviando, 'pendente', 'pendentes')}, ${plural(c.falhou, 'falhou', 'falharam')}`
  );
}

export function estadoLegivel(d: Disparo, agora: Date = new Date()): string {
  if (d.estado === 'agendado' && d.na_fila) return 'na fila';
  // "agendado" com início já passado é o disparo que acabou de ser iniciado (o motor passa por
  // `agendado` até o primeiro envio): dizer "agendado" faria a IA achar que ele ainda vai esperar.
  if (d.estado === 'agendado') {
    return d.inicio_em && new Date(d.inicio_em) > agora
      ? `agendado para ${formatarData(d.inicio_em)}`
      : 'começando agora';
  }
  const base = ROTULO_ESTADO[d.estado] ?? d.estado;
  return d.motivo_pausa && d.estado === 'pausado' ? `${base} (motivo: ${d.motivo_pausa})` : base;
}

export function descreverDisparo(d: Disparo): string {
  const partes = [`"${d.nome}" — ${estadoLegivel(d)} · ${textoContadores(d)}`];
  if (d.proximo_envio_em && !['concluido', 'cancelado', 'pausado'].includes(d.estado)) {
    partes.push(`próximo envio ${formatarData(d.proximo_envio_em)}`);
  }
  if (d.estimativa_termino_em && !['concluido', 'cancelado'].includes(d.estado)) {
    partes.push(`término estimado ${formatarData(d.estimativa_termino_em)}`);
  }
  partes.push(`origem ${d.origem}`);
  return `${partes.join(' · ')} (disparo_id: ${d.id})`;
}

/** Frase "Na fila: começa quando <nome> terminar." para disparos que ficaram na fila da conta. */
async function textoFila(cliente: ClienteMotor, disparo: Disparo): Promise<string | null> {
  if (!disparo.na_fila) return null;
  let ativo: Disparo | undefined;
  for (const estado of ['enviando', 'fora_da_janela'] as const) {
    const pagina = await cliente.listarDisparos({ conta_id: disparo.conta_id, estado, limite: 5 });
    ativo = pagina.itens.find((d) => d.id !== disparo.id);
    if (ativo) break;
  }
  return ativo
    ? `Na fila: começa quando "${ativo.nome}" terminar.`
    : 'Na fila: começa quando o disparo ativo desta conta terminar.';
}

/** Conta registros de um CSV (respeitando aspas), sem o cabeçalho. */
export function contarLinhasCsv(csv: string): number {
  const texto = csv.replace(/^﻿/, '');
  let registros = 0;
  let entreAspas = false;
  let registroTemConteudo = false;
  for (let i = 0; i < texto.length; i++) {
    const ch = texto[i];
    if (ch === '"') entreAspas = !entreAspas;
    if (!entreAspas && (ch === '\n' || ch === '\r')) {
      if (registroTemConteudo) registros++;
      registroTemConteudo = false;
      continue;
    }
    registroTemConteudo = true;
  }
  if (registroTemConteudo) registros++;
  return Math.max(0, registros - 1);
}

function nomeArquivoSeguro(nome: string): string {
  const limpo = nome
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 60);
  return limpo || 'disparo';
}

function dataLocalHoje(): string {
  const agora = new Date();
  const p = (n: number) => String(n).padStart(2, '0');
  return `${agora.getFullYear()}-${p(agora.getMonth() + 1)}-${p(agora.getDate())}`;
}

export const registrarDisparos: Registrador = (servidor, contexto) => {
  registrarFerramenta(servidor, contexto, 'criar_disparo', {
    titulo: 'Criar e iniciar disparo em massa',
    descricao:
      'Cria um disparo em massa de WhatsApp e ELE COMEÇA NA HORA, sem pedir confirmação (respeitando `inicio_em`, ' +
      'a janela de horário, os limites de ritmo e a fila da conta: se outro disparo da conta estiver ativo, este entra ' +
      'na fila). Para interromper depois, use pausar_disparo ou cancelar_disparo. ' +
      'Mensagem: `mensagem` (com variáveis como {nome} ou {empresa}, preenchidas por lead) ou `template_id`. ' +
      'Destinatários (pelo menos uma fonte, unidas sem repetir): `leads` (lista nova, importada sem duplicar), ' +
      '`lead_ids`, `etiqueta_ids` ou `contato_ids`. Se algum destinatário não tiver valor para uma variável e não ' +
      'houver `valores_padrao` para ela, NADA é criado e o erro lista as linhas incompletas. ' +
      'Ritmo: espera aleatória entre `intervalo_min_s` e `intervalo_max_s` segundos entre mensagens; limites por ' +
      'hora/dia e pausas longas são opcionais (ritmos agressivos aumentam o risco de bloqueio do número).',
    entrada: z.object({
      conta_id: entradaContaId.optional(),
      nome: z.string().trim().min(1).max(100).optional().describe('Nome do disparo (padrão "Disparo dd/mm hh:mm").'),
      mensagem: z.string().min(1).max(4096).optional().describe('Texto com variáveis, ex.: "Oi {nome}, tudo bem?". Obrigatório sem template_id.'),
      template_id: z.string().min(1).optional().describe('Usa texto e anexo de um template (veja listar_templates).'),
      caminho_anexo: entradaCaminhoArquivo.optional().describe('Arquivo local enviado junto com cada mensagem.'),
      destinatarios: z
        .object({
          leads: z
            .array(entradaLeadNovo)
            .min(1)
            .max(50_000)
            .optional()
            .describe('Leads novos: importados sem duplicar (origem mcp) e usados neste disparo.'),
          lead_ids: z.array(z.string().min(1)).optional().describe('Leads já existentes (veja listar_leads / importar_leads).'),
          etiqueta_ids: z.array(z.string().min(1)).optional().describe('Todos os contatos com estas etiquetas.'),
          contato_ids: z.array(z.string().min(1)).optional().describe('Contatos específicos (veja listar_contatos).'),
        })
        .describe('Quem recebe. Informe pelo menos uma fonte.'),
      intervalo_min_s: z.number().int().min(1).describe('Espera mínima entre mensagens, em segundos (sugestão: 30).'),
      intervalo_max_s: z.number().int().min(1).describe('Espera máxima entre mensagens, em segundos (≥ mínimo; sugestão: 90).'),
      limite_por_hora: z.number().int().positive().optional().describe('Máximo de mensagens por hora (sugestão: 40).'),
      limite_por_dia: z.number().int().positive().optional().describe('Máximo de mensagens por dia (sugestão: 300).'),
      pausa_a_cada: z.number().int().positive().optional().describe('Faz uma pausa longa a cada N mensagens (exige pausa_duracao_s).'),
      pausa_duracao_s: z.number().int().positive().optional().describe('Duração da pausa longa, em segundos.'),
      inicio_em: z.iso
        .datetime({ offset: true })
        .optional()
        .describe('Quando começar (RFC 3339 com fuso, ex.: "2026-09-28T09:00:00-03:00"). Padrão: agora.'),
      janela_inicio: horario.optional().describe('Só envia a partir deste horário, "HH:MM" (exige janela_fim).'),
      janela_fim: horario.optional().describe('Só envia até este horário, "HH:MM".'),
      falhas_seguidas_max: z
        .number()
        .int()
        .min(0)
        .optional()
        .describe('Pausa o disparo após N falhas seguidas (padrão 10; 0 = nunca pausa por falhas).'),
      valores_padrao: entradaValoresPadrao.optional(),
    }),
    saida: z.object({
      disparo: saidaDisparo,
      relatorio_importacao: saidaRelatorioImportacao.optional(),
    }),
    anotacoes: ENVIO_WHATSAPP,
    executar: async (args, cliente) => {
      if (!args.mensagem && !args.template_id) {
        throw new ErroFerramenta('Informe `mensagem` ou `template_id`.');
      }
      if (args.intervalo_max_s < args.intervalo_min_s) {
        throw new ErroFerramenta('`intervalo_max_s` precisa ser maior ou igual a `intervalo_min_s`.');
      }
      if ((args.pausa_a_cada === undefined) !== (args.pausa_duracao_s === undefined)) {
        throw new ErroFerramenta('Informe `pausa_a_cada` e `pausa_duracao_s` juntos.');
      }
      if ((args.janela_inicio === undefined) !== (args.janela_fim === undefined)) {
        throw new ErroFerramenta('Informe `janela_inicio` e `janela_fim` juntos.');
      }
      if (args.janela_inicio !== undefined && args.janela_inicio === args.janela_fim) {
        throw new ErroFerramenta('`janela_inicio` e `janela_fim` precisam ser diferentes.');
      }
      const d = args.destinatarios;
      const temFonte =
        (d.leads?.length ?? 0) + (d.lead_ids?.length ?? 0) + (d.etiqueta_ids?.length ?? 0) + (d.contato_ids?.length ?? 0) > 0;
      if (!temFonte) {
        throw new ErroFerramenta('Informe pelo menos um destinatário: `leads`, `lead_ids`, `etiqueta_ids` ou `contato_ids`.');
      }

      const contaId = await contaPadrao(cliente, args.conta_id);
      const arquivo = args.caminho_anexo
        ? await cliente.enviarArquivoPorCaminho(caminhoAbsoluto(args.caminho_anexo))
        : null;

      const novo: NovoDisparo = {
        conta_id: contaId,
        ...(args.nome ? { nome: args.nome } : {}),
        ...(args.mensagem ? { mensagem: args.mensagem } : {}),
        template_id: args.template_id ?? null,
        arquivo_id: arquivo?.id ?? null,
        destinatarios: {
          lead_ids: d.lead_ids ?? [],
          etiqueta_ids: d.etiqueta_ids ?? [],
          contato_ids: d.contato_ids ?? [],
          importar: d.leads ? { leads: d.leads, origem: 'mcp' } : null,
        },
        ritmo: {
          intervalo_min_s: args.intervalo_min_s,
          intervalo_max_s: args.intervalo_max_s,
          limite_por_hora: args.limite_por_hora ?? null,
          limite_por_dia: args.limite_por_dia ?? null,
          pausa_a_cada: args.pausa_a_cada ?? null,
          pausa_duracao_s: args.pausa_duracao_s ?? null,
        },
        inicio_em: args.inicio_em ?? null,
        janela:
          args.janela_inicio && args.janela_fim ? { inicio: args.janela_inicio, fim: args.janela_fim } : null,
        falhas_seguidas_max: args.falhas_seguidas_max ?? 10,
        valores_padrao: args.valores_padrao ?? {},
        iniciar: true,
        origem: 'mcp',
      };

      const { relatorio_importacao: relatorio, ...disparo } = await cliente.criarDisparo(novo);

      const partes = [
        `Disparo criado e iniciado: ${descreverDisparo(disparo)}.`,
        disparo.inicio_em ? `Começa em ${formatarData(disparo.inicio_em)}.` : '',
        (await textoFila(cliente, disparo)) ?? '',
        disparo.aviso_ritmo_agressivo
          ? 'Atenção: ritmo agressivo (intervalo curto ou limites altos/ausentes) aumenta o risco de bloqueio do número.'
          : '',
        `Para interromper: pausar_disparo ou cancelar_disparo com disparo_id "${disparo.id}".`,
        relatorio ? `Importação dos leads: ${textoRelatorioImportacao(relatorio)}` : '',
      ].filter((p) => p.length > 0);

      return responder(
        { disparo, ...(relatorio ? { relatorio_importacao: relatorio } : {}) },
        partes.join('\n'),
      );
    },
  });

  registrarFerramenta(servidor, contexto, 'listar_disparos', {
    titulo: 'Listar disparos',
    descricao:
      'Lista os disparos (mais recentes primeiro) com estado, contadores (enviados, entregues, lidos, respostas, ' +
      'falhas), próximo envio e término estimado. Filtros: conta e estado.',
    entrada: z.object({
      conta_id: z.string().min(1).optional().describe('Só disparos desta conta (padrão: todas).'),
      estado: z.enum(ESTADOS_DISPARO).optional().describe('Só disparos neste estado.'),
      limite: entradaLimite.optional(),
      cursor: entradaCursor.optional(),
    }),
    saida: saidaPagina(saidaDisparo),
    anotacoes: LEITURA,
    executar: async (args, cliente) => {
      const pagina = await cliente.listarDisparos(args);
      const texto =
        pagina.itens.length === 0
          ? 'Nenhum disparo encontrado.'
          : `${plural(pagina.itens.length, 'disparo', 'disparos')}:\n${listar(pagina.itens, descreverDisparo, 200)}\n` +
            textoPaginacao(pagina.proximo_cursor);
      return responder({ ...pagina }, texto);
    },
  });

  registrarFerramenta(servidor, contexto, 'ver_disparo', {
    titulo: 'Ver disparo e destinatários',
    descricao:
      'Mostra um disparo com seus contadores e a lista de destinatários com o estado de cada um (pendente, enviado, ' +
      'entregue, lido, respondeu, falhou + motivo). Filtre por `estado_destinatarios` para ver, por exemplo, só as falhas.',
    entrada: z.object({
      disparo_id: entradaDisparoId,
      estado_destinatarios: z.enum(ESTADOS_DESTINATARIO).optional().describe('Só destinatários neste estado.'),
      limite: entradaLimite.optional(),
      cursor: entradaCursor.optional(),
    }),
    saida: z.object({ disparo: saidaDisparo, destinatarios: saidaPagina(saidaDestinatario) }),
    anotacoes: LEITURA,
    executar: async ({ disparo_id, estado_destinatarios, limite, cursor }, cliente) => {
      const [disparo, destinatarios] = await Promise.all([
        cliente.obterDisparo(disparo_id),
        cliente.listarDestinatarios(disparo_id, {
          ...(estado_destinatarios ? { estado: estado_destinatarios } : {}),
          ...(limite ? { limite } : {}),
          ...(cursor ? { cursor } : {}),
        }),
      ]);
      const linhas = listar(
        destinatarios.itens,
        (x) =>
          `${x.ordem}. ${x.telefone}${x.nome ? ` (${x.nome})` : ''} — ${x.estado}${x.motivo_falha ? `: ${x.motivo_falha}` : ''}`,
        200,
      );
      const texto =
        `Disparo ${descreverDisparo(disparo)}\nMensagem: "${disparo.mensagem}"\n` +
        (destinatarios.itens.length === 0
          ? 'Nenhum destinatário nesta página.'
          : `Destinatários:\n${linhas}\n${textoPaginacao(destinatarios.proximo_cursor)}`);
      return responder({ disparo, destinatarios }, texto);
    },
  });

  registrarFerramenta(servidor, contexto, 'iniciar_disparo', {
    titulo: 'Iniciar disparo em rascunho',
    descricao:
      'Inicia AGORA (sem confirmação) um disparo que está em rascunho (criado na interface do app). ' +
      'Se faltar valor de variável para algum destinatário, informe `valores_padrao`.',
    entrada: z.object({ disparo_id: entradaDisparoId, valores_padrao: entradaValoresPadrao.optional() }),
    saida: saidaDisparo,
    anotacoes: ENVIO_WHATSAPP,
    executar: async ({ disparo_id, valores_padrao }, cliente) => {
      const disparo = await cliente.iniciarDisparo(disparo_id, valores_padrao);
      const fila = await textoFila(cliente, disparo);
      return responder({ ...disparo }, [`Disparo iniciado: ${descreverDisparo(disparo)}.`, fila].filter(Boolean).join('\n'));
    },
  });

  registrarFerramenta(servidor, contexto, 'pausar_disparo', {
    titulo: 'Pausar disparo',
    descricao: 'Pausa um disparo em andamento (mesmo efeito do botão Pausar). Retome com retomar_disparo.',
    entrada: z.object({ disparo_id: entradaDisparoId }),
    saida: saidaDisparo,
    anotacoes: { ...ESCRITA_LOCAL, idempotentHint: true },
    executar: async ({ disparo_id }, cliente) => {
      const disparo = await cliente.pausarDisparo(disparo_id);
      return responder({ ...disparo }, `Disparo pausado: ${descreverDisparo(disparo)}.`);
    },
  });

  registrarFerramenta(servidor, contexto, 'retomar_disparo', {
    titulo: 'Retomar disparo',
    descricao:
      'Retoma um disparo pausado, sem reenviar para quem já recebeu. Se outro disparo da mesma conta estiver ativo, ' +
      'este entra na fila.',
    entrada: z.object({ disparo_id: entradaDisparoId }),
    saida: saidaDisparo,
    anotacoes: ENVIO_WHATSAPP,
    executar: async ({ disparo_id }, cliente) => {
      const disparo = await cliente.retomarDisparo(disparo_id);
      const fila = await textoFila(cliente, disparo);
      return responder({ ...disparo }, [`Disparo retomado: ${descreverDisparo(disparo)}.`, fila].filter(Boolean).join('\n'));
    },
  });

  registrarFerramenta(servidor, contexto, 'cancelar_disparo', {
    titulo: 'Cancelar disparo',
    descricao:
      'Cancela um disparo de vez (irreversível): os pendentes não recebem mais nada; o relatório continua disponível.',
    entrada: z.object({ disparo_id: entradaDisparoId }),
    saida: saidaDisparo,
    anotacoes: { readOnlyHint: false, destructiveHint: true, openWorldHint: false, idempotentHint: true },
    executar: async ({ disparo_id }, cliente) => {
      const disparo = await cliente.cancelarDisparo(disparo_id);
      return responder({ ...disparo }, `Disparo cancelado: ${descreverDisparo(disparo)}.`);
    },
  });

  registrarFerramenta(servidor, contexto, 'exportar_relatorio', {
    titulo: 'Exportar relatório do disparo (CSV)',
    descricao:
      'Grava o relatório do disparo em CSV (uma linha por destinatário: telefone, nome, estado, motivo da falha, ' +
      'horários de envio/entrega/leitura/resposta e variáveis). Padrão: ~/Downloads/zapdesk-<nome>-<data>.csv.',
    entrada: z.object({
      disparo_id: entradaDisparoId,
      caminho_destino: entradaCaminhoArquivo.optional().describe('Onde gravar o CSV (sobrescreve se existir).'),
    }),
    saida: z.object({
      caminho: z.string().describe('Caminho absoluto do CSV gravado.'),
      linhas: z.number().describe('Quantidade de destinatários no relatório.'),
    }),
    anotacoes: { ...ESCRITA_LOCAL, idempotentHint: true },
    executar: async ({ disparo_id, caminho_destino }, cliente) => {
      let caminho: string;
      if (caminho_destino) {
        caminho = caminhoAbsoluto(caminho_destino);
      } else {
        const disparo = await cliente.obterDisparo(disparo_id);
        caminho = join(contexto.pastaDownloads, `zapdesk-${nomeArquivoSeguro(disparo.nome)}-${dataLocalHoje()}.csv`);
      }
      const csv = await cliente.relatorioCsv(disparo_id);
      await mkdir(dirname(caminho), { recursive: true });
      await writeFile(caminho, csv, 'utf8');
      const linhas = contarLinhasCsv(csv);
      return responder({ caminho, linhas }, `Relatório gravado em ${caminho} (${plural(linhas, 'linha', 'linhas')}).`);
    },
  });
};
