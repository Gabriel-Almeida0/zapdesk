// Ferramenta `ver_status`: status (stories) publicados pelos contatos nas últimas 24 h.
import { z } from 'zod';

import { contaPadrao } from '../conta-padrao.js';
import { entradaContaId, saidaStatus } from '../esquemas.js';
import { LEITURA, type Registrador, registrarFerramenta } from '../ferramenta.js';
import { formatarData, listar, plural, responder } from '../formatar.js';

export const registrarStatus: Registrador = (servidor, contexto) => {
  registrarFerramenta(servidor, contexto, 'ver_status', {
    titulo: 'Ver status dos contatos',
    descricao:
      'Lista os status (stories) publicados pelos contatos nas últimas 24 horas, agrupados por contato, ' +
      'mais recentes primeiro. Status de texto trazem o texto; imagem/vídeo trazem legenda e tipo de mídia.',
    entrada: z.object({ conta_id: entradaContaId.optional() }),
    saida: z.object({
      status_por_contato: z.array(
        z.looseObject({
          contato_jid: z.string(),
          contato_nome: z.string().nullable(),
          itens: z.array(saidaStatus),
        }),
      ),
    }),
    anotacoes: LEITURA,
    executar: async ({ conta_id }, cliente) => {
      const contaId = await contaPadrao(cliente, conta_id);
      const grupos = await cliente.listarStatus(contaId);
      const texto =
        grupos.length === 0
          ? 'Nenhum status publicado nas últimas 24 horas.'
          : `${plural(grupos.length, 'contato publicou', 'contatos publicaram')} status:\n` +
            listar(
              grupos,
              (g) =>
                `${g.contato_nome ?? g.contato_jid}: ` +
                g.itens
                  .map((s) => `[${formatarData(s.publicado_em)} ${s.tipo}]${s.texto ? ` ${s.texto}` : ''}`)
                  .join(' | '),
              200,
            );
      return responder({ status_por_contato: grupos }, texto);
    },
  });
};
