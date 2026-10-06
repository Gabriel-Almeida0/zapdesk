// Ferramentas de leads: `importar_leads` e `listar_leads`.
import type { ClienteMotor, RelatorioImportacao } from '@zapdesk/cliente-motor';
import { z } from 'zod';

import {
  entradaCaminhoArquivo,
  entradaCursor,
  entradaLeadNovo,
  entradaLimite,
  saidaLead,
  saidaPagina,
  saidaRelatorioImportacao,
} from '../esquemas.js';
import { ESCRITA_LOCAL, LEITURA, type Registrador, caminhoAbsoluto, registrarFerramenta } from '../ferramenta.js';
import { ErroFerramenta, formatarData, listar, plural, responder, textoPaginacao } from '../formatar.js';

const ROTULO_MOTIVO: Record<string, string> = {
  vazio: 'vazio',
  formato_invalido: 'formato inválido',
  numero_invalido: 'número inválido',
};

/** "7 novos, 3 já existiam, 0 inválidos, 0 duplicados no lote" + listas. */
export function textoRelatorioImportacao(relatorio: RelatorioImportacao): string {
  const partes = [
    `${relatorio.total_novos} ${relatorio.total_novos === 1 ? 'novo' : 'novos'}, ` +
      `${relatorio.total_ja_existentes} já ${relatorio.total_ja_existentes === 1 ? 'existia' : 'existiam'}, ` +
      `${relatorio.total_invalidos} ${relatorio.total_invalidos === 1 ? 'inválido' : 'inválidos'}, ` +
      `${relatorio.total_duplicados_no_lote} ${relatorio.total_duplicados_no_lote === 1 ? 'duplicado' : 'duplicados'} no lote ` +
      `(${plural(relatorio.total_linhas, 'linha', 'linhas')}).`,
  ];
  if (relatorio.ja_existentes.length > 0) {
    partes.push(
      `Já existiam (não foram duplicados; dados vazios foram completados):\n${listar(
        relatorio.ja_existentes,
        (item) =>
          `${item.telefone} (linha ${item.linha}) — importado em ${formatarData(item.importado_em)}` +
          (item.campos_preenchidos.length > 0 ? `; completou: ${item.campos_preenchidos.join(', ')}` : ''),
      )}`,
    );
  }
  if (relatorio.invalidos.length > 0) {
    partes.push(
      `Inválidos (ignorados):\n${listar(
        relatorio.invalidos,
        (item) => `linha ${item.linha}: "${item.valor}" — ${ROTULO_MOTIVO[item.motivo] ?? item.motivo}`,
      )}`,
    );
  }
  if (relatorio.duplicados_no_lote.length > 0) {
    partes.push(
      `Repetidos dentro do próprio lote (considerada só a primeira ocorrência):\n${listar(
        relatorio.duplicados_no_lote,
        (item) => `${item.telefone} (linha ${item.linha}, igual à linha ${item.primeira_linha})`,
      )}`,
    );
  }
  return partes.join('\n\n');
}

function acharColuna(colunas: readonly string[], pedida: string): string | undefined {
  return colunas.find((c) => c === pedida) ?? colunas.find((c) => c.toLowerCase() === pedida.toLowerCase());
}

async function importarDePlanilha(
  cliente: ClienteMotor,
  caminho: string,
  colunaTelefone: string | undefined,
  colunaNome: string | undefined,
  ddiPadrao: string | undefined,
): Promise<RelatorioImportacao> {
  const previa = await cliente.previaImportacaoPorCaminho(caminhoAbsoluto(caminho));
  const colunas = previa.colunas.map((c) => `"${c}"`).join(', ');

  let telefone: string | undefined;
  if (colunaTelefone) {
    telefone = acharColuna(previa.colunas, colunaTelefone);
    if (!telefone) {
      throw new ErroFerramenta(`A planilha não tem a coluna "${colunaTelefone}". Colunas disponíveis: ${colunas}.`);
    }
  } else {
    telefone = previa.coluna_telefone_sugerida ?? undefined;
    if (!telefone) {
      throw new ErroFerramenta(
        `Não consegui descobrir qual coluna tem o telefone. Informe coluna_telefone. Colunas disponíveis: ${colunas}.`,
      );
    }
  }

  let nome: string | null = null;
  if (colunaNome) {
    nome = acharColuna(previa.colunas, colunaNome) ?? null;
    if (!nome) throw new ErroFerramenta(`A planilha não tem a coluna "${colunaNome}". Colunas disponíveis: ${colunas}.`);
  } else {
    nome = previa.coluna_nome_sugerida;
  }

  return cliente.importarLeads({
    importacao_id: previa.importacao_id,
    mapeamento: { telefone, nome },
    origem: 'mcp',
    ...(ddiPadrao ? { ddi_padrao: ddiPadrao } : {}),
  });
}

export const registrarLeads: Registrador = (servidor, contexto) => {
  registrarFerramenta(servidor, contexto, 'importar_leads', {
    titulo: 'Importar leads',
    descricao:
      'Adiciona leads (telefones) à base do ZapDesk sem duplicar. Envie `leads` (lista de {telefone, nome?, campos?}) ' +
      'OU `caminho_arquivo` de uma planilha local .csv/.xlsx. Telefones em qualquer formato; o ZapDesk normaliza. ' +
      'Retorna quais foram inseridos, quais já existiam (com a data da importação original), quais são inválidos ' +
      'e quais se repetiam no próprio lote. Leads que já existiam não são duplicados; campos vazios deles são completados. ' +
      'Os `lead_ids` devolvidos podem ser usados em criar_disparo.',
    entrada: z.object({
      leads: z
        .array(entradaLeadNovo)
        .min(1)
        .max(50_000)
        .optional()
        .describe('Lista de leads (1 a 50.000). Não use junto com caminho_arquivo.'),
      caminho_arquivo: entradaCaminhoArquivo
        .optional()
        .describe('Planilha local .csv ou .xlsx com os leads. Não use junto com leads.'),
      coluna_telefone: z
        .string()
        .optional()
        .describe('Só com caminho_arquivo: nome da coluna com o telefone (se omitido, o ZapDesk tenta adivinhar).'),
      coluna_nome: z
        .string()
        .optional()
        .describe('Só com caminho_arquivo: nome da coluna com o nome. As demais colunas viram campos extras.'),
      ddi_padrao: z
        .string()
        .regex(/^\d{1,3}$/, 'Só dígitos, ex.: "55".')
        .optional()
        .describe('DDI usado em números sem código do país (padrão "55", Brasil).'),
    }),
    saida: saidaRelatorioImportacao,
    anotacoes: { ...ESCRITA_LOCAL, idempotentHint: true },
    executar: async (args, cliente) => {
      if (args.leads && args.caminho_arquivo) {
        throw new ErroFerramenta('Use `leads` OU `caminho_arquivo`, não os dois na mesma chamada.');
      }
      let relatorio: RelatorioImportacao;
      if (args.caminho_arquivo) {
        relatorio = await importarDePlanilha(
          cliente,
          args.caminho_arquivo,
          args.coluna_telefone,
          args.coluna_nome,
          args.ddi_padrao,
        );
      } else if (args.leads) {
        relatorio = await cliente.importarLeads({
          leads: args.leads,
          origem: 'mcp',
          ...(args.ddi_padrao ? { ddi_padrao: args.ddi_padrao } : {}),
        });
      } else {
        throw new ErroFerramenta('Informe `leads` (lista de telefones) ou `caminho_arquivo` (planilha .csv/.xlsx).');
      }
      return responder({ ...relatorio }, textoRelatorioImportacao(relatorio));
    },
  });

  registrarFerramenta(servidor, contexto, 'listar_leads', {
    titulo: 'Listar leads',
    descricao:
      'Lista a base de leads (mais recentes primeiro), com telefone, nome, campos extras, origem ' +
      '(csv, colado, contatos, mcp), se tem WhatsApp, data de importação e do último disparo. Aceita busca e filtro por origem.',
    entrada: z.object({
      busca: z.string().optional().describe('Trecho do telefone ou do nome.'),
      origem: z.enum(['csv', 'colado', 'contatos', 'mcp']).optional().describe('Filtra pela origem do lead.'),
      limite: entradaLimite.optional(),
      cursor: entradaCursor.optional(),
    }),
    saida: saidaPagina(saidaLead),
    anotacoes: LEITURA,
    executar: async (args, cliente) => {
      const pagina = await cliente.listarLeads(args);
      const linhas = listar(
        pagina.itens,
        (lead) =>
          `${lead.telefone}${lead.nome ? ` — ${lead.nome}` : ''} · origem ${lead.origem} · importado em ` +
          `${formatarData(lead.importado_em)}${lead.ultimo_disparo_em ? ` · último disparo ${formatarData(lead.ultimo_disparo_em)}` : ''}` +
          ` (lead_id: ${lead.id})`,
        200,
      );
      const texto =
        pagina.itens.length === 0
          ? 'Nenhum lead encontrado.'
          : `${plural(pagina.itens.length, 'lead', 'leads')} nesta página:\n${linhas}\n${textoPaginacao(pagina.proximo_cursor)}`;
      return responder({ ...pagina }, texto);
    },
  });
};
