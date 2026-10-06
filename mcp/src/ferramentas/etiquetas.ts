// Ferramentas de etiquetas: `listar_etiquetas`, `criar_etiqueta`, `atualizar_etiqueta`, `excluir_etiqueta`.
import type { Etiqueta } from '@zapdesk/cliente-motor';
import { z } from 'zod';

import { entradaCor, saidaEtiqueta, saidaOk } from '../esquemas.js';
import { ESCRITA_LOCAL, EXCLUSAO_LOCAL, LEITURA, type Registrador, registrarFerramenta } from '../ferramenta.js';
import { ErroFerramenta, listar, plural, responder } from '../formatar.js';

const entradaEtiquetaId = z.string().min(1).describe('ID da etiqueta (veja listar_etiquetas).');
const entradaNome = z.string().trim().min(1).max(30).describe('Nome da etiqueta (1–30 caracteres, único).');

const descrever = (e: Etiqueta) =>
  `${e.nome} ${e.cor} · ${plural(e.total_contatos, 'contato', 'contatos')} (etiqueta_id: ${e.id})`;

export const registrarEtiquetas: Registrador = (servidor, contexto) => {
  registrarFerramenta(servidor, contexto, 'listar_etiquetas', {
    titulo: 'Listar etiquetas',
    descricao: 'Lista as etiquetas (nome, cor e quantos contatos têm cada uma). Etiquetas valem para todas as contas.',
    entrada: z.object({}),
    saida: z.object({ etiquetas: z.array(saidaEtiqueta) }),
    anotacoes: LEITURA,
    executar: async (_args, cliente) => {
      const etiquetas = await cliente.listarEtiquetas();
      const texto =
        etiquetas.length === 0
          ? 'Nenhuma etiqueta criada.'
          : `${plural(etiquetas.length, 'etiqueta', 'etiquetas')}:\n${listar(etiquetas, descrever, 500)}`;
      return responder({ etiquetas }, texto);
    },
  });

  registrarFerramenta(servidor, contexto, 'criar_etiqueta', {
    titulo: 'Criar etiqueta',
    descricao: 'Cria uma etiqueta para organizar contatos. O nome é único (sem diferenciar maiúsculas).',
    entrada: z.object({ nome: entradaNome, cor: entradaCor }),
    saida: saidaEtiqueta,
    anotacoes: ESCRITA_LOCAL,
    executar: async ({ nome, cor }, cliente) => {
      const etiqueta = await cliente.criarEtiqueta({ nome, cor });
      return responder({ ...etiqueta }, `Etiqueta criada: ${descrever(etiqueta)}`);
    },
  });

  registrarFerramenta(servidor, contexto, 'atualizar_etiqueta', {
    titulo: 'Atualizar etiqueta',
    descricao: 'Renomeia e/ou muda a cor de uma etiqueta.',
    entrada: z.object({
      etiqueta_id: entradaEtiquetaId,
      nome: entradaNome.optional(),
      cor: entradaCor.optional(),
    }),
    saida: saidaEtiqueta,
    anotacoes: { ...ESCRITA_LOCAL, idempotentHint: true },
    executar: async ({ etiqueta_id, nome, cor }, cliente) => {
      if (nome === undefined && cor === undefined) throw new ErroFerramenta('Informe `nome` e/ou `cor`.');
      const etiqueta = await cliente.editarEtiqueta(etiqueta_id, {
        ...(nome !== undefined ? { nome } : {}),
        ...(cor !== undefined ? { cor } : {}),
      });
      return responder({ ...etiqueta }, `Etiqueta atualizada: ${descrever(etiqueta)}`);
    },
  });

  registrarFerramenta(servidor, contexto, 'excluir_etiqueta', {
    titulo: 'Excluir etiqueta',
    descricao: 'Exclui uma etiqueta e a remove de todos os contatos (irreversível).',
    entrada: z.object({ etiqueta_id: entradaEtiquetaId }),
    saida: saidaOk,
    anotacoes: EXCLUSAO_LOCAL,
    executar: async ({ etiqueta_id }, cliente) => {
      await cliente.excluirEtiqueta(etiqueta_id);
      return responder({ ok: true }, 'Etiqueta excluída.');
    },
  });
};
