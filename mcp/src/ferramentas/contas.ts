// Ferramentas de contas e sistema: `listar_contas`, `status_zapdesk`.
import { z } from 'zod';

import { descreverConta } from '../conta-padrao.js';
import { saidaConta } from '../esquemas.js';
import { LEITURA, type Registrador, registrarFerramenta } from '../ferramenta.js';
import { listar, plural, responder } from '../formatar.js';

export const registrarContas: Registrador = (servidor, contexto) => {
  registrarFerramenta(servidor, contexto, 'listar_contas', {
    titulo: 'Listar contas de WhatsApp',
    descricao:
      'Lista as contas de WhatsApp conectadas ao ZapDesk, com estado (conectando, conectada, desconectada, banida). ' +
      'Use o `id` como `conta_id` nas outras ferramentas quando houver mais de uma conta conectada. ' +
      'Conectar uma conta nova (QR code), reconectar, renomear ou remover contas só é possível pela interface do app.',
    entrada: z.object({}),
    saida: z.object({ contas: z.array(saidaConta) }),
    anotacoes: LEITURA,
    executar: async (_args, cliente) => {
      const contas = await cliente.listarContas();
      const texto =
        contas.length === 0
          ? 'Nenhuma conta cadastrada. Conecte uma conta pela interface do ZapDesk (QR code).'
          : `${plural(contas.length, 'conta', 'contas')}:\n${listar(contas, descreverConta)}`;
      return responder({ contas }, texto);
    },
  });

  registrarFerramenta(servidor, contexto, 'status_zapdesk', {
    titulo: 'Status do ZapDesk',
    descricao:
      'Mostra a versão do ZapDesk, quantas contas estão conectadas, quantos disparos estão ativos e a pasta de dados.',
    entrada: z.object({}),
    saida: z.looseObject({
      versao: z.string(),
      contas_conectadas: z.number(),
      disparos_ativos: z.number(),
      pasta_dados: z.string(),
    }),
    anotacoes: LEITURA,
    executar: async (_args, cliente) => {
      const sistema = await cliente.sistema();
      const texto =
        `ZapDesk ${sistema.versao}: ${plural(sistema.contas_conectadas, 'conta conectada', 'contas conectadas')}, ` +
        `${plural(sistema.disparos_ativos, 'disparo ativo', 'disparos ativos')}. Pasta de dados: ${sistema.pasta_dados}` +
        (sistema.whatsapp === 'falso' ? ' (modo de teste: WhatsApp falso)' : '');
      return responder({ ...sistema }, texto);
    },
  });
};
