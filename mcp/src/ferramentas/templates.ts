// Ferramentas de templates: `listar_templates`, `criar_template`, `atualizar_template`, `excluir_template`.
import type { Template } from '@zapdesk/cliente-motor';
import { z } from 'zod';

import { entradaCaminhoArquivo, saidaOk, saidaTemplate } from '../esquemas.js';
import {
  ESCRITA_LOCAL,
  EXCLUSAO_LOCAL,
  LEITURA,
  type Registrador,
  caminhoAbsoluto,
  registrarFerramenta,
} from '../ferramenta.js';
import { ErroFerramenta, listar, plural, responder } from '../formatar.js';

const entradaTemplateId = z.string().min(1).describe('ID do template (veja listar_templates).');
const entradaNome = z.string().trim().min(1).max(60).describe('Nome do template (1–60 caracteres, único).');
const entradaTexto = z
  .string()
  .min(1)
  .max(4096)
  .describe('Texto da mensagem (até 4.096). Variáveis entre chaves, ex.: "Oi {nome}, tudo bem?".');

function descrever(t: Template): string {
  const texto = t.texto.length > 120 ? `${t.texto.slice(0, 120)}…` : t.texto;
  const variaveis = t.variaveis.length > 0 ? ` · variáveis: ${t.variaveis.map((v) => `{${v}}`).join(', ')}` : '';
  const anexo = t.arquivo ? ` · anexo: ${t.arquivo.nome}` : '';
  return `${t.nome}: "${texto}"${variaveis}${anexo} (template_id: ${t.id})`;
}

export const registrarTemplates: Registrador = (servidor, contexto) => {
  registrarFerramenta(servidor, contexto, 'listar_templates', {
    titulo: 'Listar templates',
    descricao:
      'Lista os templates de mensagem (texto com variáveis como {nome} e anexo opcional). Use o template_id em criar_disparo.',
    entrada: z.object({ busca: z.string().optional().describe('Trecho do nome do template.') }),
    saida: z.object({ templates: z.array(saidaTemplate) }),
    anotacoes: LEITURA,
    executar: async ({ busca }, cliente) => {
      const templates = await cliente.listarTemplates(busca);
      const texto =
        templates.length === 0
          ? 'Nenhum template encontrado.'
          : `${plural(templates.length, 'template', 'templates')}:\n${listar(templates, descrever, 500)}`;
      return responder({ templates }, texto);
    },
  });

  registrarFerramenta(servidor, contexto, 'criar_template', {
    titulo: 'Criar template',
    descricao:
      'Cria um template de mensagem reutilizável. Variáveis entre chaves ({nome}, {empresa}…) são preenchidas com ' +
      'os dados de cada lead no disparo. `caminho_anexo` opcional: arquivo local enviado junto.',
    entrada: z.object({
      nome: entradaNome,
      texto: entradaTexto,
      caminho_anexo: entradaCaminhoArquivo.optional().describe('Arquivo local a anexar ao template.'),
    }),
    saida: saidaTemplate,
    anotacoes: ESCRITA_LOCAL,
    executar: async ({ nome, texto, caminho_anexo }, cliente) => {
      const arquivo = caminho_anexo ? await cliente.enviarArquivoPorCaminho(caminhoAbsoluto(caminho_anexo)) : null;
      const template = await cliente.criarTemplate({ nome, texto, arquivo_id: arquivo?.id ?? null });
      return responder({ ...template }, `Template criado: ${descrever(template)}`);
    },
  });

  registrarFerramenta(servidor, contexto, 'atualizar_template', {
    titulo: 'Atualizar template',
    descricao:
      'Altera nome, texto e/ou anexo de um template. `caminho_anexo: null` remove o anexo; omitido mantém o atual.',
    entrada: z.object({
      template_id: entradaTemplateId,
      nome: entradaNome.optional(),
      texto: entradaTexto.optional(),
      caminho_anexo: entradaCaminhoArquivo
        .nullable()
        .optional()
        .describe('Novo arquivo local para o anexo, ou null para remover o anexo.'),
    }),
    saida: saidaTemplate,
    anotacoes: { ...ESCRITA_LOCAL, idempotentHint: true },
    executar: async ({ template_id, nome, texto, caminho_anexo }, cliente) => {
      if (nome === undefined && texto === undefined && caminho_anexo === undefined) {
        throw new ErroFerramenta('Informe `nome`, `texto` e/ou `caminho_anexo`.');
      }
      let arquivoId: string | null | undefined;
      if (caminho_anexo === null) arquivoId = null;
      else if (caminho_anexo !== undefined) {
        arquivoId = (await cliente.enviarArquivoPorCaminho(caminhoAbsoluto(caminho_anexo))).id;
      }
      const template = await cliente.editarTemplate(template_id, {
        ...(nome !== undefined ? { nome } : {}),
        ...(texto !== undefined ? { texto } : {}),
        ...(arquivoId !== undefined ? { arquivo_id: arquivoId } : {}),
      });
      return responder({ ...template }, `Template atualizado: ${descrever(template)}`);
    },
  });

  registrarFerramenta(servidor, contexto, 'excluir_template', {
    titulo: 'Excluir template',
    descricao: 'Exclui um template (irreversível). Disparos já criados com ele não mudam.',
    entrada: z.object({ template_id: entradaTemplateId }),
    saida: saidaOk,
    anotacoes: EXCLUSAO_LOCAL,
    executar: async ({ template_id }, cliente) => {
      await cliente.excluirTemplate(template_id);
      return responder({ ok: true }, 'Template excluído.');
    },
  });
};
