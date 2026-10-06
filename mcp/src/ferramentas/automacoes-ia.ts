// Ferramentas de automações de IA (projetos TypeScript) — specs/002-automacoes/contracts/mcp-ferramentas.md ›
// Automações de IA: `ver_tipos_sdk`, `listar_modelos_automacao_ia`, `criar_automacao_ia`,
// `listar_arquivos_automacao`, `ler_arquivo_automacao`, `escrever_arquivo_automacao`,
// `renomear_arquivo_automacao`, `excluir_arquivo_automacao`, `compilar_automacao`.
//
// Ciclo esperado da IA: ver_tipos_sdk → criar_automacao_ia → escrever_arquivo_automacao →
// compilar_automacao (corrigir até compilar) → testar_automacao → ativar_automacao.
import type { ArquivoProjeto, ClienteMotor, ResultadoCompilacao } from '@zapdesk/cliente-motor';
import { z } from 'zod';

import {
  entradaAutomacaoId,
  saidaArquivoProjeto,
  saidaAutomacao,
  saidaConteudoArquivo,
  saidaModeloProjeto,
  saidaOk,
  saidaResultadoCompilacao,
} from '../esquemas.js';
import { ESCRITA_LOCAL, EXCLUSAO_LOCAL, LEITURA, type Registrador, registrarFerramenta } from '../ferramenta.js';
import { listar, plural, responder, textoErrosCompilacao } from '../formatar.js';
import { RESUMO_SDK } from '../referencia-sdk.js';
import { descreverAutomacao } from './automacoes.js';

const entradaCaminhoProjeto = z
  .string()
  .min(1)
  .max(200)
  .describe('Caminho relativo à pasta do projeto, com "/", ex.: "index.ts", "automacao.json", "lib/util.ts", "prompt.md".');

/** Até este total de caracteres o `criar_automacao_ia` inclui o conteúdo dos arquivos no texto. */
const MAX_CONTEUDO_INICIAL = 20_000;

function descreverArquivo(a: ArquivoProjeto): string {
  return `${a.caminho} (${a.tamanho} bytes, hash ${a.hash})`;
}

/** "Compilou sem erros" ou a lista `arquivo:linha:coluna [tipo] mensagem`. */
export function textoCompilacao(r: ResultadoCompilacao): string {
  const partes: string[] = [];
  if (r.ok) {
    partes.push(
      `Compilou sem erros em ${r.duracao_ms} ms. Handlers exportados: ${r.handlers.join(', ') || 'nenhum'}.` +
        '\nPróximo passo: testar_automacao (simulação) e, se estiver bom, ativar_automacao.',
    );
  } else {
    partes.push(
      `NÃO compilou: ${plural(r.erros.length, 'erro', 'erros')}.\n${textoErrosCompilacao(r.erros)}\n` +
        'Corrija com escrever_arquivo_automacao e rode compilar_automacao de novo.',
    );
  }
  if (r.avisos.length > 0) partes.push(`${plural(r.avisos.length, 'aviso', 'avisos')}:\n${textoErrosCompilacao(r.avisos)}`);
  return partes.join('\n');
}

async function conteudoInicial(cliente: ClienteMotor, automacaoId: string, arquivos: ArquivoProjeto[]): Promise<string> {
  const blocos: string[] = [];
  let total = 0;
  for (const arquivo of arquivos) {
    if (total + arquivo.tamanho > MAX_CONTEUDO_INICIAL) {
      blocos.push(`--- ${arquivo.caminho} (leia com ler_arquivo_automacao) ---`);
      continue;
    }
    const conteudo = await cliente.lerArquivo(automacaoId, arquivo.caminho);
    total += conteudo.conteudo.length;
    blocos.push(`--- ${arquivo.caminho} (hash ${conteudo.hash}) ---\n${conteudo.conteudo}`);
  }
  return blocos.join('\n\n');
}

export const registrarAutomacoesIA: Registrador = (servidor, contexto) => {
  registrarFerramenta(servidor, contexto, 'ver_tipos_sdk', {
    titulo: 'Ver tipos da SDK de automação',
    descricao:
      'Devolve a referência da SDK @zapdesk/automacao usada pelo código das automações de IA: um resumo da API do ctx ' +
      'e o arquivo de tipos (.d.ts) completo (Contexto, ctx.ia, ctx.funil, ctx.memoria, erros…). Leia antes de escrever código.',
    entrada: z.object({}),
    saida: z.looseObject({ versao: z.string(), tipos: z.string() }),
    anotacoes: LEITURA,
    executar: async (_args, cliente) => {
      const sdk = await cliente.obterSdkAutomacao();
      return responder(
        { versao: sdk.versao, tipos: sdk.tipos },
        `SDK @zapdesk/automacao v${sdk.versao}\n\n${RESUMO_SDK}\n\n===== index.d.ts =====\n${sdk.tipos}`,
      );
    },
  });

  registrarFerramenta(servidor, contexto, 'listar_modelos_automacao_ia', {
    titulo: 'Listar modelos de automação de IA',
    descricao:
      'Lista os modelos de projeto para criar_automacao_ia: responder_historico (responde com IA usando o histórico), ' +
      'classificar_funil (classifica a conversa e move no funil), extrair_dados (extrai dados para campos do lead) e em_branco.',
    entrada: z.object({}),
    saida: z.object({ modelos: z.array(saidaModeloProjeto) }),
    anotacoes: LEITURA,
    executar: async (_args, cliente) => {
      const modelos = await cliente.listarModelosProjeto();
      return responder({ modelos }, `Modelos:\n${listar(modelos, (m) => `${m.id} — ${m.nome}: ${m.descricao}`)}`);
    },
  });

  registrarFerramenta(servidor, contexto, 'criar_automacao_ia', {
    titulo: 'Criar automação de IA',
    descricao:
      'Cria uma automação de IA: um projeto TypeScript (automacao.json + index.ts …) a partir de um modelo, já compilado ' +
      'e INATIVA. Devolve a automação e os arquivos com o conteúdo inicial. Depois: ajuste o código com ' +
      'escrever_arquivo_automacao (gatilhos/permissões no automacao.json), compilar_automacao, testar_automacao e ' +
      'ativar_automacao. Modelos: veja listar_modelos_automacao_ia (padrão em_branco).',
    entrada: z.object({
      nome: z.string().trim().min(1).max(80).describe('Nome da automação (1–80).'),
      modelo: z
        .enum(['responder_historico', 'classificar_funil', 'extrair_dados', 'em_branco'])
        .optional()
        .describe('Modelo inicial (padrão em_branco).'),
      descricao: z.string().max(500).optional().describe('Descrição curta.'),
    }),
    saida: saidaAutomacao.extend({ arquivos: z.array(saidaArquivoProjeto) }),
    anotacoes: ESCRITA_LOCAL,
    executar: async ({ nome, modelo, descricao }, cliente) => {
      const automacao = await cliente.criarAutomacaoIA({ nome, modelo: modelo ?? 'em_branco', ...(descricao ? { descricao } : {}) });
      const arquivos = await cliente.listarArquivos(automacao.id);
      const conteudo = await conteudoInicial(cliente, automacao.id, arquivos);
      return responder(
        { ...automacao, arquivos },
        `Automação de IA criada: ${descreverAutomacao(automacao)}\n` +
          `Arquivos (${arquivos.length}):\n${listar(arquivos, descreverArquivo)}\n\n${conteudo}`,
      );
    },
  });

  registrarFerramenta(servidor, contexto, 'listar_arquivos_automacao', {
    titulo: 'Listar arquivos da automação',
    descricao: 'Lista os arquivos do projeto de uma automação de IA (caminho, tamanho e hash).',
    entrada: z.object({ automacao_id: entradaAutomacaoId }),
    saida: z.object({ arquivos: z.array(saidaArquivoProjeto) }),
    anotacoes: LEITURA,
    executar: async ({ automacao_id }, cliente) => {
      const arquivos = await cliente.listarArquivos(automacao_id);
      return responder(
        { arquivos },
        arquivos.length === 0 ? 'O projeto não tem arquivos.' : `${plural(arquivos.length, 'arquivo', 'arquivos')}:\n${listar(arquivos, descreverArquivo, 500)}`,
      );
    },
  });

  registrarFerramenta(servidor, contexto, 'ler_arquivo_automacao', {
    titulo: 'Ler arquivo da automação',
    descricao:
      'Lê um arquivo do projeto de uma automação de IA. Devolve o conteúdo e o hash (passe-o como hash_anterior ao ' +
      'escrever, para não sobrescrever mudanças feitas pelo usuário no editor).',
    entrada: z.object({ automacao_id: entradaAutomacaoId, caminho: entradaCaminhoProjeto }),
    saida: saidaConteudoArquivo,
    anotacoes: LEITURA,
    executar: async ({ automacao_id, caminho }, cliente) => {
      const arquivo = await cliente.lerArquivo(automacao_id, caminho);
      return responder({ ...arquivo }, `--- ${arquivo.caminho} (hash ${arquivo.hash}) ---\n${arquivo.conteudo}`);
    },
  });

  registrarFerramenta(servidor, contexto, 'escrever_arquivo_automacao', {
    titulo: 'Escrever arquivo da automação',
    descricao:
      'Cria ou substitui (conteúdo inteiro) um arquivo do projeto de uma automação de IA: .ts, .json, .md ou .txt. ' +
      'NÃO compila sozinho: chame compilar_automacao em seguida. `hash_anterior` (de ler_arquivo_automacao) evita ' +
      'sobrescrever mudanças alheias (conflito se o arquivo mudou); null = o arquivo precisa ser novo. ' +
      'O manifesto automacao.json define gatilhos, permissões e segredos (formato em ver_formatos_automacao).\n\n' +
      RESUMO_SDK,
    entrada: z.object({
      automacao_id: entradaAutomacaoId,
      caminho: entradaCaminhoProjeto,
      conteudo: z.string().max(1_000_000).describe('Conteúdo completo do arquivo (UTF-8).'),
      hash_anterior: z
        .string()
        .min(1)
        .nullable()
        .optional()
        .describe('Hash lido antes (conflito se mudou); null = deve ser arquivo novo; omitido = sobrescreve sem conferir.'),
    }),
    saida: saidaArquivoProjeto,
    anotacoes: { ...ESCRITA_LOCAL, idempotentHint: true },
    executar: async ({ automacao_id, caminho, conteudo, hash_anterior }, cliente) => {
      const arquivo = await cliente.escreverArquivo(automacao_id, caminho, conteudo, hash_anterior);
      return responder(
        { ...arquivo },
        `Arquivo gravado: ${descreverArquivo(arquivo)}. Rode compilar_automacao para conferir erros.`,
      );
    },
  });

  registrarFerramenta(servidor, contexto, 'renomear_arquivo_automacao', {
    titulo: 'Renomear arquivo da automação',
    descricao: 'Renomeia/move um arquivo do projeto (conflito se o destino existir). Atualize os imports e compile de novo.',
    entrada: z.object({ automacao_id: entradaAutomacaoId, de: entradaCaminhoProjeto, para: entradaCaminhoProjeto }),
    saida: saidaArquivoProjeto,
    anotacoes: ESCRITA_LOCAL,
    executar: async ({ automacao_id, de, para }, cliente) => {
      const arquivo = await cliente.renomearArquivo(automacao_id, de, para);
      return responder({ ...arquivo }, `Arquivo renomeado: ${de} → ${descreverArquivo(arquivo)}. Rode compilar_automacao.`);
    },
  });

  registrarFerramenta(servidor, contexto, 'excluir_arquivo_automacao', {
    titulo: 'Excluir arquivo da automação',
    descricao: 'Exclui um arquivo do projeto (irreversível). O manifesto automacao.json e o arquivo de entrada não podem ser excluídos.',
    entrada: z.object({ automacao_id: entradaAutomacaoId, caminho: entradaCaminhoProjeto }),
    saida: saidaOk,
    anotacoes: EXCLUSAO_LOCAL,
    executar: async ({ automacao_id, caminho }, cliente) => {
      await cliente.excluirArquivo(automacao_id, caminho);
      return responder({ ok: true }, `Arquivo ${caminho} excluído. Rode compilar_automacao.`);
    },
  });

  registrarFerramenta(servidor, contexto, 'compilar_automacao', {
    titulo: 'Compilar automação de IA',
    descricao:
      'Compila o projeto (TypeScript + manifesto) e devolve "Compilou sem erros" com os handlers exportados, ou a lista de ' +
      'erros no formato arquivo:linha:coluna [tipo] mensagem (tipos: sintaxe, importacao, manifesto, resolucao). ' +
      'Confere também se cada gatilho do automacao.json tem o handler exigido. Nada é executado.',
    entrada: z.object({ automacao_id: entradaAutomacaoId }),
    saida: saidaResultadoCompilacao,
    anotacoes: { ...LEITURA, idempotentHint: true },
    executar: async ({ automacao_id }, cliente) => {
      const resultado = await cliente.compilarAutomacao(automacao_id);
      return responder({ ...resultado }, textoCompilacao(resultado));
    },
  });
};
