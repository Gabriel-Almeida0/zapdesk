// Ferramentas de funil (Kanban de leads) — specs/002-automacoes/contracts/mcp-ferramentas.md › Funil:
// `listar_funis`, `criar_funil`, `editar_funil`, `excluir_etapa`, `excluir_funil`,
// `listar_cards_funil`, `mover_card_funil`, `remover_card_funil`, `historico_funil`, `atualizar_lead`.
import type { Card, ClienteMotor, Funil, MovimentoFunil, PedidoCard } from '@zapdesk/cliente-motor';
import { z } from 'zod';

import {
  entradaCor,
  entradaCursor,
  entradaEtapaId,
  entradaFunilId,
  entradaLeadId,
  entradaLimite,
  saidaCard,
  saidaFunil,
  saidaLead,
  saidaMovimentoFunil,
  saidaOk,
  saidaPagina,
} from '../esquemas.js';
import { ESCRITA_LOCAL, EXCLUSAO_LOCAL, LEITURA, type Registrador, registrarFerramenta } from '../ferramenta.js';
import { ErroFerramenta, formatarData, listar, plural, responder, textoPaginacao } from '../formatar.js';

const entradaNomeFunil = z.string().trim().min(1).max(60).describe('Nome do funil (1–60 caracteres, único).');
const entradaNomeEtapa = z.string().trim().min(1).max(40).describe('Nome da etapa (1–40 caracteres, único no funil).');

/** "Vendas (funil_id X, 5 cards): Novo [etapa_id a · 3] → Proposta [etapa_id b · 2]". */
export function descreverFunil(funil: Funil): string {
  const etapas = [...funil.etapas]
    .sort((a, b) => a.ordem - b.ordem)
    .map((e) => `${e.nome} [etapa_id ${e.id} · ${plural(e.total_cards, 'card', 'cards')}]`)
    .join(' → ');
  return `${funil.nome} (funil_id ${funil.id}, ${plural(funil.total_cards, 'card', 'cards')}): ${etapas || 'sem etapas'}`;
}

function descreverCard(card: Card, funil?: Funil): string {
  const etapa = funil?.etapas.find((e) => e.id === card.etapa_id)?.nome ?? card.etapa_id;
  const nome = card.lead.nome ? ` — ${card.lead.nome}` : '';
  const etiquetas = card.etiquetas.length > 0 ? ` · etiquetas: ${card.etiquetas.map((e) => e.nome).join(', ')}` : '';
  return `${card.lead.telefone}${nome} · etapa ${etapa} desde ${formatarData(card.desde)}${etiquetas} (lead_id: ${card.lead_id})`;
}

function descreverMovimento(m: MovimentoFunil): string {
  const de = m.etapa_origem_nome ?? (m.etapa_origem_id ? m.etapa_origem_id : 'fora do funil');
  const para = m.etapa_destino_nome ?? (m.etapa_destino_id ? m.etapa_destino_id : 'fora do funil');
  const quem = m.origem === 'automacao' ? `automação ${m.automacao_id ?? ''}`.trim() : m.origem;
  return `${formatarData(m.em)} · lead ${m.lead_id}: ${de} → ${para} (por ${quem})`;
}

/**
 * Aplica a lista completa de etapas desejada: ids existentes são editados, sem id são criados
 * (ou reaproveitados se já houver etapa com o mesmo nome — mantém a chamada idempotente), e a
 * ordem final segue a lista; etapas omitidas não são apagadas e vão para o fim, na ordem atual.
 */
async function aplicarEtapas(
  cliente: ClienteMotor,
  funil: Funil,
  etapas: { etapa_id?: string | undefined; nome: string; cor?: string | undefined }[],
): Promise<void> {
  const existentes = [...funil.etapas].sort((a, b) => a.ordem - b.ordem);
  const referenciadas = new Set(etapas.map((e) => e.etapa_id).filter((id): id is string => Boolean(id)));
  const ordemDesejada: string[] = [];

  for (const pedida of etapas) {
    if (pedida.etapa_id) {
      const atual = existentes.find((e) => e.id === pedida.etapa_id);
      if (!atual) {
        throw new ErroFerramenta(
          `A etapa ${pedida.etapa_id} não pertence ao funil "${funil.nome}". Etapas atuais: ${existentes.map((e) => `${e.nome} (${e.id})`).join(', ') || 'nenhuma'}.`,
        );
      }
      const mudou = atual.nome !== pedida.nome || (pedida.cor !== undefined && pedida.cor.toLowerCase() !== atual.cor.toLowerCase());
      if (mudou) {
        await cliente.editarEtapa(atual.id, { nome: pedida.nome, ...(pedida.cor !== undefined ? { cor: pedida.cor } : {}) });
      }
      ordemDesejada.push(atual.id);
      continue;
    }
    const mesmoNome = existentes.find(
      (e) => !referenciadas.has(e.id) && !ordemDesejada.includes(e.id) && e.nome.toLowerCase() === pedida.nome.toLowerCase(),
    );
    if (mesmoNome) {
      if (pedida.cor !== undefined && pedida.cor.toLowerCase() !== mesmoNome.cor.toLowerCase()) {
        await cliente.editarEtapa(mesmoNome.id, { cor: pedida.cor });
      }
      ordemDesejada.push(mesmoNome.id);
      continue;
    }
    const nova = await cliente.criarEtapa(funil.id, { nome: pedida.nome, ...(pedida.cor !== undefined ? { cor: pedida.cor } : {}) });
    ordemDesejada.push(nova.id);
  }

  const omitidas = existentes.filter((e) => !ordemDesejada.includes(e.id)).map((e) => e.id);
  const ordemFinal = [...ordemDesejada, ...omitidas];
  const depois = await cliente.obterFunil(funil.id);
  const ordemAtual = [...depois.etapas].sort((a, b) => a.ordem - b.ordem).map((e) => e.id);
  if (ordemAtual.join(',') !== ordemFinal.join(',')) await cliente.reordenarEtapas(funil.id, ordemFinal);
}

export const registrarFunis: Registrador = (servidor, contexto) => {
  registrarFerramenta(servidor, contexto, 'listar_funis', {
    titulo: 'Listar funis',
    descricao:
      'Lista os funis de vendas (Kanban de leads) com suas etapas em ordem, ids e quantos cards (leads) há em cada etapa. ' +
      'Use os funil_id/etapa_id em mover_card_funil, em gatilhos entrou_etapa e em ações mover_etapa de automações.',
    entrada: z.object({}),
    saida: z.object({ funis: z.array(saidaFunil) }),
    anotacoes: LEITURA,
    executar: async (_args, cliente) => {
      const funis = await cliente.listarFunis();
      const texto =
        funis.length === 0
          ? 'Nenhum funil criado. Use criar_funil.'
          : `${plural(funis.length, 'funil', 'funis')}:\n${listar(funis, descreverFunil)}`;
      return responder({ funis }, texto);
    },
  });

  registrarFerramenta(servidor, contexto, 'criar_funil', {
    titulo: 'Criar funil',
    descricao:
      'Cria um funil de vendas (Kanban de leads). Informe `etapas` (recomendado) com nomes e cores na ordem desejada, ' +
      'ex.: Novo → Em conversa → Proposta → Fechado. Depois ajuste com editar_funil.',
    entrada: z.object({
      nome: entradaNomeFunil,
      etapas: z
        .array(z.object({ nome: entradaNomeEtapa, cor: entradaCor.optional() }))
        .min(1)
        .max(30)
        .optional()
        .describe('Etapas na ordem desejada (máx. 30).'),
    }),
    saida: saidaFunil,
    anotacoes: ESCRITA_LOCAL,
    executar: async ({ nome, etapas }, cliente) => {
      const funil = await cliente.criarFunil({ nome, ...(etapas ? { etapas } : {}) });
      return responder({ ...funil }, `Funil criado: ${descreverFunil(funil)}`);
    },
  });

  registrarFerramenta(servidor, contexto, 'editar_funil', {
    titulo: 'Editar funil',
    descricao:
      'Renomeia o funil e/ou ajusta as etapas. `etapas` é a lista COMPLETA na ordem desejada: itens com etapa_id são ' +
      'renomeados/recoloridos/reordenados, itens sem etapa_id são criados. Etapas omitidas NÃO são apagadas (ficam no fim); ' +
      'para apagar use excluir_etapa. Pode ser repetida sem duplicar etapas.',
    entrada: z.object({
      funil_id: entradaFunilId,
      nome: entradaNomeFunil.optional(),
      etapas: z
        .array(
          z.object({
            etapa_id: z.string().min(1).optional().describe('ID de etapa existente; omita para criar uma nova.'),
            nome: entradaNomeEtapa,
            cor: entradaCor.optional(),
          }),
        )
        .min(1)
        .max(30)
        .optional(),
    }),
    saida: saidaFunil,
    anotacoes: { ...ESCRITA_LOCAL, idempotentHint: true },
    executar: async ({ funil_id, nome, etapas }, cliente) => {
      if (nome === undefined && etapas === undefined) throw new ErroFerramenta('Informe `nome` e/ou `etapas`.');
      let funil = await cliente.obterFunil(funil_id);
      if (nome !== undefined && nome !== funil.nome) funil = await cliente.editarFunil(funil_id, { nome });
      if (etapas) await aplicarEtapas(cliente, funil, etapas);
      funil = await cliente.obterFunil(funil_id);
      return responder({ ...funil }, `Funil atualizado: ${descreverFunil(funil)}`);
    },
  });

  registrarFerramenta(servidor, contexto, 'excluir_etapa', {
    titulo: 'Excluir etapa',
    descricao:
      'Exclui uma etapa do funil. Se a etapa tiver cards, informe `destino_etapa_id` (move os cards para outra etapa ' +
      'do mesmo funil) OU `remover_cards: true` (tira esses leads do funil).',
    entrada: z.object({
      etapa_id: entradaEtapaId,
      destino_etapa_id: z.string().min(1).optional().describe('Etapa que recebe os cards da etapa excluída.'),
      remover_cards: z.literal(true).optional().describe('true = remove do funil os leads que estavam na etapa.'),
    }),
    saida: saidaOk,
    anotacoes: EXCLUSAO_LOCAL,
    executar: async ({ etapa_id, destino_etapa_id, remover_cards }, cliente) => {
      if (destino_etapa_id && remover_cards) {
        throw new ErroFerramenta('Use `destino_etapa_id` OU `remover_cards`, não os dois.');
      }
      await cliente.excluirEtapa(
        etapa_id,
        destino_etapa_id ? { destino_etapa_id } : remover_cards ? { remover_cards: true } : {},
      );
      return responder(
        { ok: true },
        destino_etapa_id
          ? `Etapa excluída; os cards foram movidos para a etapa ${destino_etapa_id}.`
          : remover_cards
            ? 'Etapa excluída; os leads dela saíram do funil.'
            : 'Etapa excluída.',
      );
    },
  });

  registrarFerramenta(servidor, contexto, 'excluir_funil', {
    titulo: 'Excluir funil',
    descricao: 'Exclui o funil com todas as etapas, cards e histórico (irreversível). Os leads continuam na base.',
    entrada: z.object({ funil_id: entradaFunilId }),
    saida: saidaOk,
    anotacoes: EXCLUSAO_LOCAL,
    executar: async ({ funil_id }, cliente) => {
      await cliente.excluirFunil(funil_id);
      return responder({ ok: true }, 'Funil excluído.');
    },
  });

  registrarFerramenta(servidor, contexto, 'listar_cards_funil', {
    titulo: 'Listar cards do funil',
    descricao:
      'Lista os leads (cards) de um funil, mais recentes na etapa primeiro, com telefone, nome, etapa, desde quando e etiquetas. ' +
      'Filtre por etapa_id ou busca (nome/telefone). Paginado.',
    entrada: z.object({
      funil_id: entradaFunilId,
      etapa_id: entradaEtapaId.optional(),
      busca: z.string().optional().describe('Trecho do nome ou telefone.'),
      limite: entradaLimite.optional(),
      cursor: entradaCursor.optional(),
    }),
    saida: saidaPagina(saidaCard),
    anotacoes: LEITURA,
    executar: async ({ funil_id, ...filtro }, cliente) => {
      const [pagina, funil] = await Promise.all([cliente.listarCards(funil_id, filtro), cliente.obterFunil(funil_id)]);
      const texto =
        pagina.itens.length === 0
          ? `Nenhum card encontrado no funil "${funil.nome}".`
          : `${plural(pagina.itens.length, 'card', 'cards')} no funil "${funil.nome}":\n` +
            `${listar(pagina.itens, (c) => descreverCard(c, funil), 200)}\n${textoPaginacao(pagina.proximo_cursor)}`;
      return responder({ ...pagina }, texto);
    },
  });

  registrarFerramenta(servidor, contexto, 'mover_card_funil', {
    titulo: 'Mover card no funil',
    descricao:
      'Coloca um lead no funil ou o move para outra etapa. Identifique o lead por UM de: lead_id, contato_id ou telefone ' +
      '(qualquer formato; se o telefone não for lead ainda, ele é criado com origem mcp). Dispara automações com gatilho ' +
      'entrou_etapa. Idempotente: mover para a etapa em que já está não muda nada.',
    entrada: z.object({
      funil_id: entradaFunilId,
      etapa_id: entradaEtapaId,
      lead_id: entradaLeadId.optional(),
      contato_id: z.string().min(1).optional().describe('ID do contato (veja listar_contatos).'),
      telefone: z.string().min(1).optional().describe('Telefone em qualquer formato, ex.: "(11) 99999-0000".'),
    }),
    saida: saidaCard,
    anotacoes: { ...ESCRITA_LOCAL, idempotentHint: true },
    executar: async ({ funil_id, etapa_id, lead_id, contato_id, telefone }, cliente) => {
      const alvos = [lead_id, contato_id, telefone].filter((v) => v !== undefined);
      if (alvos.length !== 1) {
        throw new ErroFerramenta('Informe exatamente um de `lead_id`, `contato_id` ou `telefone`.');
      }
      const pedido: PedidoCard = lead_id
        ? { etapa_id, lead_id, origem: 'mcp' }
        : contato_id
          ? { etapa_id, contato_id, origem: 'mcp' }
          : { etapa_id, telefone: telefone as string, origem: 'mcp' };
      const card = await cliente.moverCard(funil_id, pedido);
      const funil = await cliente.obterFunil(funil_id).catch(() => undefined);
      return responder({ ...card }, `Card no funil${funil ? ` "${funil.nome}"` : ''}: ${descreverCard(card, funil)}`);
    },
  });

  registrarFerramenta(servidor, contexto, 'remover_card_funil', {
    titulo: 'Remover card do funil',
    descricao: 'Tira o lead do funil (o lead continua na base). Idempotente.',
    entrada: z.object({ funil_id: entradaFunilId, lead_id: entradaLeadId }),
    saida: saidaOk,
    anotacoes: ESCRITA_LOCAL,
    executar: async ({ funil_id, lead_id }, cliente) => {
      await cliente.removerCard(funil_id, lead_id, 'mcp');
      return responder({ ok: true }, 'Lead removido do funil.');
    },
  });

  registrarFerramenta(servidor, contexto, 'historico_funil', {
    titulo: 'Histórico do funil',
    descricao:
      'Mostra as movimentações de cards no funil (mais recentes primeiro): de qual etapa para qual, quando e quem moveu ' +
      '(app, mcp ou automação). Filtre por lead_id para ver a trajetória de um lead.',
    entrada: z.object({
      funil_id: entradaFunilId,
      lead_id: entradaLeadId.optional(),
      limite: entradaLimite.optional(),
      cursor: entradaCursor.optional(),
    }),
    saida: saidaPagina(saidaMovimentoFunil),
    anotacoes: LEITURA,
    executar: async ({ funil_id, ...filtro }, cliente) => {
      const pagina = await cliente.historicoFunil(funil_id, filtro);
      const texto =
        pagina.itens.length === 0
          ? 'Nenhuma movimentação registrada.'
          : `${plural(pagina.itens.length, 'movimentação', 'movimentações')}:\n${listar(pagina.itens, descreverMovimento, 200)}\n${textoPaginacao(pagina.proximo_cursor)}`;
      return responder({ ...pagina }, texto);
    },
  });

  registrarFerramenta(servidor, contexto, 'atualizar_lead', {
    titulo: 'Atualizar lead',
    descricao:
      'Altera o nome e/ou os campos extras de um lead (viram variáveis {campo} em mensagens e condições campo_lead). ' +
      '`campos` é mesclado: chaves informadas são gravadas, valor null remove o campo, as demais ficam como estão.',
    entrada: z.object({
      lead_id: entradaLeadId,
      nome: z.string().max(200).nullable().optional().describe('Novo nome (null apaga o nome).'),
      campos: z
        .record(z.string().min(1), z.string().nullable())
        .optional()
        .describe('Ex.: {"empresa": "ACME", "cidade": null} (null remove).'),
    }),
    saida: saidaLead,
    anotacoes: ESCRITA_LOCAL,
    executar: async ({ lead_id, nome, campos }, cliente) => {
      if (nome === undefined && campos === undefined) throw new ErroFerramenta('Informe `nome` e/ou `campos`.');
      const lead = await cliente.editarLead(lead_id, {
        ...(nome !== undefined ? { nome } : {}),
        ...(campos !== undefined ? { campos } : {}),
      });
      const extras = Object.entries(lead.campos);
      return responder(
        { ...lead },
        `Lead atualizado: ${lead.telefone}${lead.nome ? ` — ${lead.nome}` : ''}` +
          (extras.length > 0 ? `\nCampos: ${extras.map(([k, v]) => `${k}=${v}`).join(', ')}` : '\nSem campos extras.'),
      );
    },
  });
};
