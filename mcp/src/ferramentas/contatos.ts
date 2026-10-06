// Ferramentas de contatos: `listar_contatos`, `atualizar_contato` (notas e etiquetas).
import type { Contato } from '@zapdesk/cliente-motor';
import { z } from 'zod';

import { contaPadrao } from '../conta-padrao.js';
import { entradaContaId, entradaCursor, entradaLimite, saidaContato, saidaPagina } from '../esquemas.js';
import { ESCRITA_LOCAL, LEITURA, type Registrador, registrarFerramenta } from '../ferramenta.js';
import { ErroFerramenta, formatarData, listar, plural, responder, textoPaginacao } from '../formatar.js';

export function descreverContato(contato: Contato): string {
  const nome = contato.nome ?? contato.nome_push ?? contato.telefone ?? contato.jid;
  const partes = [nome];
  if (contato.telefone && contato.telefone !== nome) partes.push(`(${contato.telefone})`);
  if (contato.etiquetas.length > 0) partes.push(`· etiquetas: ${contato.etiquetas.map((e) => e.nome).join(', ')}`);
  if (contato.lead) partes.push(`· lead (${contato.lead.origem}, ${formatarData(contato.lead.importado_em)})`);
  if (contato.notas) partes.push(`· notas: ${contato.notas.length > 80 ? `${contato.notas.slice(0, 80)}…` : contato.notas}`);
  partes.push(`(contato_id: ${contato.id})`);
  return partes.join(' ');
}

export const registrarContatos: Registrador = (servidor, contexto) => {
  registrarFerramenta(servidor, contexto, 'listar_contatos', {
    titulo: 'Listar contatos',
    descricao:
      'Lista os contatos de uma conta com nome, telefone, etiquetas, notas e se já é lead. Filtros: busca por ' +
      'nome/telefone e etiqueta.',
    entrada: z.object({
      conta_id: entradaContaId.optional(),
      busca: z.string().optional().describe('Trecho do nome ou do telefone.'),
      etiqueta_id: z.string().optional().describe('Só contatos com esta etiqueta (veja listar_etiquetas).'),
      limite: entradaLimite.optional(),
      cursor: entradaCursor.optional(),
    }),
    saida: saidaPagina(saidaContato),
    anotacoes: LEITURA,
    executar: async ({ conta_id, ...filtro }, cliente) => {
      const contaId = await contaPadrao(cliente, conta_id);
      const pagina = await cliente.listarContatos(contaId, filtro);
      const texto =
        pagina.itens.length === 0
          ? 'Nenhum contato encontrado.'
          : `${plural(pagina.itens.length, 'contato', 'contatos')}:\n${listar(pagina.itens, descreverContato, 200)}\n` +
            textoPaginacao(pagina.proximo_cursor);
      return responder({ ...pagina }, texto);
    },
  });

  registrarFerramenta(servidor, contexto, 'atualizar_contato', {
    titulo: 'Atualizar contato',
    descricao:
      'Atualiza as notas e/ou as etiquetas de um contato. `etiqueta_ids` SUBSTITUI o conjunto inteiro de etiquetas ' +
      '(para adicionar uma, envie as atuais + a nova; [] remove todas). `notas` substitui o texto das notas ("" apaga).',
    entrada: z.object({
      contato_id: z.string().min(1).describe('ID do contato (veja listar_contatos).'),
      notas: z.string().max(10_000).optional().describe('Novo texto das notas (até 10.000 caracteres).'),
      etiqueta_ids: z.array(z.string().min(1)).optional().describe('Conjunto completo de etiquetas do contato.'),
    }),
    saida: saidaContato,
    anotacoes: { ...ESCRITA_LOCAL, idempotentHint: true },
    executar: async ({ contato_id, notas, etiqueta_ids }, cliente) => {
      if (notas === undefined && etiqueta_ids === undefined) {
        throw new ErroFerramenta('Informe `notas` e/ou `etiqueta_ids`.');
      }
      let contato: Contato | undefined;
      if (notas !== undefined) contato = await cliente.salvarNotas(contato_id, notas);
      if (etiqueta_ids !== undefined) contato = await cliente.definirEtiquetasContato(contato_id, etiqueta_ids);
      if (!contato) throw new ErroFerramenta('Nada foi alterado.');
      return responder({ ...contato }, `Contato atualizado: ${descreverContato(contato)}`);
    },
  });
};
