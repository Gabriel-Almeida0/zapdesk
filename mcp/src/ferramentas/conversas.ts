// Ferramentas de conversas e mensagens: `listar_conversas`, `ler_mensagens`, `buscar_mensagens`,
// `enviar_mensagem`, `reagir_mensagem`, `editar_mensagem`, `apagar_mensagem`, `marcar_como_lida`.
import type { ClienteMotor, Conversa, Mensagem } from '@zapdesk/cliente-motor';
import { z } from 'zod';

import { contaPadrao } from '../conta-padrao.js';
import {
  entradaCaminhoArquivo,
  entradaContaId,
  entradaCursor,
  entradaLimite,
  saidaConversa,
  saidaMensagem,
  saidaOk,
  saidaPagina,
} from '../esquemas.js';
import {
  ENVIO_WHATSAPP,
  LEITURA,
  type Registrador,
  caminhoAbsoluto,
  registrarFerramenta,
} from '../ferramenta.js';
import { ErroFerramenta, formatarData, listar, plural, responder, textoPaginacao } from '../formatar.js';

const entradaMensagemId = z.string().min(1).describe('ID da mensagem (veja ler_mensagens ou buscar_mensagens).');
const entradaConversaId = z.string().min(1).describe('ID da conversa (veja listar_conversas).');

export function descreverConversa(conversa: Conversa): string {
  const partes = [conversa.nome];
  if (conversa.telefone && conversa.telefone !== conversa.nome) partes.push(`(${conversa.telefone})`);
  if (conversa.tipo === 'grupo') partes.push('[grupo]');
  if (conversa.nao_lidas > 0) partes.push(`· ${plural(conversa.nao_lidas, 'não lida', 'não lidas')}`);
  if (conversa.ultima_mensagem_em) {
    partes.push(`· última ${formatarData(conversa.ultima_mensagem_em)}: ${conversa.ultima_mensagem_resumo ?? ''}`.trimEnd());
  }
  if (conversa.etiquetas.length > 0) partes.push(`· etiquetas: ${conversa.etiquetas.map((e) => e.nome).join(', ')}`);
  partes.push(`(conversa_id: ${conversa.id})`);
  return partes.join(' ');
}

/** "[27/09/2026 20:10] Ana: [imagem] legenda (lida) (id: X)". */
export function descreverMensagem(mensagem: Mensagem): string {
  const autor = mensagem.de_mim ? 'Você' : (mensagem.remetente_nome ?? mensagem.remetente_jid);
  const conteudo: string[] = [];
  if (mensagem.citacao) conteudo.push(`↪ respondendo "${mensagem.citacao.resumo}"`);
  if (mensagem.tipo !== 'texto') conteudo.push(`[${mensagem.tipo}${mensagem.midia?.nome_arquivo ? `: ${mensagem.midia.nome_arquivo}` : ''}]`);
  if (mensagem.apagada) conteudo.push('(mensagem apagada)');
  else if (mensagem.texto) conteudo.push(mensagem.texto);
  if (mensagem.editada) conteudo.push('(editada)');
  if (mensagem.reacoes.length > 0) conteudo.push(`reações: ${mensagem.reacoes.map((r) => r.emoji).join(' ')}`);
  if (mensagem.de_mim) conteudo.push(`(${mensagem.estado}${mensagem.erro ? `: ${mensagem.erro}` : ''})`);
  return `[${formatarData(mensagem.enviada_em)}] ${autor}: ${conteudo.join(' ')} (mensagem_id: ${mensagem.id})`;
}

const digitos = (texto: string) => texto.replace(/\D/g, '');

/** Procura uma conversa existente pelo telefone (sem criar). */
async function acharConversaPorTelefone(
  cliente: ClienteMotor,
  contaId: string,
  telefone: string,
): Promise<Conversa | undefined> {
  const procurado = digitos(telefone);
  if (procurado.length < 8) return undefined;
  const pagina = await cliente.listarConversas(contaId, { busca: procurado, limite: 50 });
  return pagina.itens.find((c) => {
    const d = c.telefone ? digitos(c.telefone) : '';
    return d.length > 0 && (d.endsWith(procurado) || procurado.endsWith(d));
  });
}

/**
 * Conversa por `conversa_id` ou por telefone (conta padrão). Na leitura, procura primeiro uma
 * conversa existente (funciona com a conta desconectada); se não achar, abre pelo motor, que
 * normaliza o telefone e cria a conversa se preciso.
 */
async function resolverConversa(
  cliente: ClienteMotor,
  args: { conversa_id?: string | undefined; telefone?: string | undefined; conta_id?: string | undefined },
  leitura = false,
): Promise<Conversa> {
  if (args.conversa_id && args.telefone) {
    throw new ErroFerramenta('Informe `conversa_id` OU `telefone`, não os dois.');
  }
  if (args.conversa_id) return cliente.obterConversa(args.conversa_id);
  if (args.telefone) {
    const contaId = await contaPadrao(cliente, args.conta_id);
    if (leitura) {
      const existente = await acharConversaPorTelefone(cliente, contaId, args.telefone).catch(() => undefined);
      if (existente) return existente;
    }
    return cliente.abrirConversa(contaId, args.telefone);
  }
  throw new ErroFerramenta('Informe `conversa_id` ou `telefone`.');
}

export const registrarConversas: Registrador = (servidor, contexto) => {
  registrarFerramenta(servidor, contexto, 'listar_conversas', {
    titulo: 'Listar conversas',
    descricao:
      'Lista as conversas de uma conta (mais recentes primeiro) com nome, telefone, não lidas, resumo da última ' +
      'mensagem e etiquetas. Filtros: só não lidas, por etiqueta ou busca por nome/telefone.',
    entrada: z.object({
      conta_id: entradaContaId.optional(),
      nao_lidas: z.boolean().optional().describe('true = só conversas com mensagens não lidas.'),
      etiqueta_id: z.string().optional().describe('Só conversas cujo contato tem esta etiqueta.'),
      busca: z.string().optional().describe('Trecho do nome ou do telefone.'),
      limite: entradaLimite.optional(),
      cursor: entradaCursor.optional(),
    }),
    saida: saidaPagina(saidaConversa),
    anotacoes: LEITURA,
    executar: async ({ conta_id, ...filtro }, cliente) => {
      const contaId = await contaPadrao(cliente, conta_id);
      const pagina = await cliente.listarConversas(contaId, filtro);
      const texto =
        pagina.itens.length === 0
          ? 'Nenhuma conversa encontrada.'
          : `${plural(pagina.itens.length, 'conversa', 'conversas')}:\n${listar(pagina.itens, descreverConversa, 200)}\n` +
            textoPaginacao(pagina.proximo_cursor);
      return responder({ ...pagina }, texto);
    },
  });

  registrarFerramenta(servidor, contexto, 'ler_mensagens', {
    titulo: 'Ler mensagens de uma conversa',
    descricao:
      'Lê as mensagens de uma conversa em ordem cronológica (mais antigas primeiro), com remetente, texto, tipo, ' +
      'data, reações e estado de entrega. Informe `conversa_id` ou `telefone` (em qualquer formato). ' +
      'Para ver mensagens mais antigas, chame de novo com `antes` = `mais_antigas_antes` do resultado.',
    entrada: z.object({
      conversa_id: entradaConversaId.optional(),
      telefone: z.string().min(1).optional().describe('Telefone do contato, em qualquer formato (alternativa a conversa_id).'),
      conta_id: entradaContaId.optional(),
      limite: entradaLimite.optional().describe('Quantas mensagens trazer (padrão 30, máx 200).'),
      antes: z.string().min(1).optional().describe('Traz mensagens anteriores a este mensagem_id.'),
    }),
    saida: z.object({
      conversa: saidaConversa,
      mensagens: z.array(saidaMensagem),
      mais_antigas_antes: z
        .string()
        .nullable()
        .describe('Passe como `antes` para ler mensagens mais antigas; null = não há mais.'),
    }),
    anotacoes: LEITURA,
    executar: async (args, cliente) => {
      const conversa = await resolverConversa(cliente, args, true);
      const pagina = await cliente.listarMensagens(conversa.id, {
        limite: args.limite ?? 30,
        ...(args.antes ? { antes: args.antes } : {}),
      });
      const mensagens = [...pagina.itens].reverse();
      const maisAntigas = pagina.proximo_cursor !== null && mensagens[0] ? mensagens[0].id : null;
      const cabecalho = `Conversa ${descreverConversa(conversa)}`;
      const corpo =
        mensagens.length === 0
          ? 'Nenhuma mensagem.'
          : mensagens.map(descreverMensagem).join('\n') +
            (maisAntigas ? `\nHá mensagens mais antigas: chame de novo com antes "${maisAntigas}".` : '');
      return responder({ conversa, mensagens, mais_antigas_antes: maisAntigas }, `${cabecalho}\n${corpo}`);
    },
  });

  registrarFerramenta(servidor, contexto, 'buscar_mensagens', {
    titulo: 'Buscar mensagens',
    descricao:
      'Busca um termo no texto de todas as mensagens de uma conta (ignora acentos e maiúsculas). ' +
      'Retorna a mensagem, a conversa e um trecho com o termo.',
    entrada: z.object({
      termo: z.string().min(1).describe('Palavra ou frase a buscar.'),
      conta_id: entradaContaId.optional(),
      limite: entradaLimite.optional(),
      cursor: entradaCursor.optional(),
    }),
    saida: saidaPagina(
      z.looseObject({
        mensagem: saidaMensagem,
        conversa: z.looseObject({ id: z.string(), nome: z.string() }),
        trecho: z.string(),
      }),
    ),
    anotacoes: LEITURA,
    executar: async ({ termo, conta_id, limite, cursor }, cliente) => {
      const contaId = await contaPadrao(cliente, conta_id);
      const pagina = await cliente.buscarMensagens(contaId, termo, {
        ...(limite ? { limite } : {}),
        ...(cursor ? { cursor } : {}),
      });
      const texto =
        pagina.itens.length === 0
          ? `Nenhuma mensagem com "${termo}".`
          : `${plural(pagina.itens.length, 'resultado', 'resultados')} para "${termo}":\n` +
            listar(
              pagina.itens,
              (r) =>
                `${r.conversa.nome} · ${formatarData(r.mensagem.enviada_em)} · ${r.mensagem.de_mim ? 'Você' : (r.mensagem.remetente_nome ?? r.mensagem.remetente_jid)}: ` +
                `"${r.trecho}" (mensagem_id: ${r.mensagem.id}, conversa_id: ${r.conversa.id})`,
              200,
            ) +
            `\n${textoPaginacao(pagina.proximo_cursor)}`;
      return responder({ ...pagina }, texto);
    },
  });

  registrarFerramenta(servidor, contexto, 'enviar_mensagem', {
    titulo: 'Enviar mensagem',
    descricao:
      'Envia AGORA uma mensagem de WhatsApp para uma pessoa (sem confirmação). Destino: `telefone` (qualquer formato; ' +
      'abre a conversa se ainda não existir) ou `conversa_id`. Conteúdo: `texto` e/ou `caminho_anexo` (arquivo local: ' +
      'imagem, vídeo, áudio, documento ou figurinha .webp; o texto vira legenda). `como` força voz, figurinha ou documento. ' +
      'Limites: imagem/vídeo/áudio 16 MB, documento 100 MB, figurinha 1 MB.',
    entrada: z.object({
      telefone: z.string().min(1).optional().describe('Telefone do destinatário, em qualquer formato.'),
      conversa_id: entradaConversaId.optional().describe('Alternativa a telefone: ID da conversa (inclusive grupos).'),
      texto: z.string().min(1).max(4096).optional().describe('Texto da mensagem (ou legenda do anexo).'),
      caminho_anexo: entradaCaminhoArquivo.optional().describe('Arquivo local a anexar.'),
      como: z
        .enum(['auto', 'voz', 'figurinha', 'documento'])
        .optional()
        .describe('Como enviar o anexo: auto (padrão, pelo tipo), voz (áudio gravado), figurinha ou documento.'),
      citar_mensagem_id: z.string().min(1).optional().describe('Responder citando esta mensagem.'),
      conta_id: entradaContaId.optional(),
    }),
    saida: saidaMensagem,
    anotacoes: ENVIO_WHATSAPP,
    executar: async (args, cliente) => {
      if (!args.texto && !args.caminho_anexo) {
        throw new ErroFerramenta('Informe `texto` e/ou `caminho_anexo`.');
      }
      const conversa = await resolverConversa(cliente, args);
      const arquivo = args.caminho_anexo
        ? await cliente.enviarArquivoPorCaminho(caminhoAbsoluto(args.caminho_anexo))
        : null;
      const mensagem = await cliente.enviarMensagem(conversa.id, {
        texto: args.texto ?? null,
        arquivo_id: arquivo?.id ?? null,
        como: args.como ?? 'auto',
        citar_mensagem_id: args.citar_mensagem_id ?? null,
      });
      const oque = arquivo ? `${arquivo.tipo_midia} "${arquivo.nome}"${args.texto ? ' com legenda' : ''}` : 'mensagem';
      return responder(
        { ...mensagem },
        `Enviei ${oque} para ${conversa.nome}${conversa.telefone && conversa.telefone !== conversa.nome ? ` (${conversa.telefone})` : ''}. ` +
          `Estado: ${mensagem.estado}. (mensagem_id: ${mensagem.id}, conversa_id: ${conversa.id})`,
      );
    },
  });

  registrarFerramenta(servidor, contexto, 'reagir_mensagem', {
    titulo: 'Reagir a mensagem',
    descricao: 'Reage a uma mensagem com um emoji (ex.: "👍"). Envie emoji vazio ("") para remover a sua reação.',
    entrada: z.object({
      mensagem_id: entradaMensagemId,
      emoji: z.string().max(16).describe('Um emoji, ou "" para remover a reação.'),
    }),
    saida: saidaOk,
    anotacoes: { readOnlyHint: false, destructiveHint: false, openWorldHint: true, idempotentHint: true },
    executar: async ({ mensagem_id, emoji }, cliente) => {
      await cliente.reagir(mensagem_id, emoji);
      return responder({ ok: true }, emoji ? `Reagi com ${emoji}.` : 'Reação removida.');
    },
  });

  registrarFerramenta(servidor, contexto, 'editar_mensagem', {
    titulo: 'Editar mensagem',
    descricao:
      'Edita o texto de uma mensagem de texto enviada por você (o WhatsApp só permite editar até 15 minutos após o envio).',
    entrada: z.object({
      mensagem_id: entradaMensagemId,
      texto: z.string().min(1).max(4096).describe('Novo texto.'),
    }),
    saida: saidaMensagem,
    anotacoes: ENVIO_WHATSAPP,
    executar: async ({ mensagem_id, texto }, cliente) => {
      const mensagem = await cliente.editarMensagem(mensagem_id, texto);
      return responder({ ...mensagem }, `Mensagem editada: "${mensagem.texto ?? texto}".`);
    },
  });

  registrarFerramenta(servidor, contexto, 'apagar_mensagem', {
    titulo: 'Apagar mensagem para todos',
    descricao:
      'Apaga para todos uma mensagem enviada por você (irreversível; o WhatsApp tem prazo limite para isso).',
    entrada: z.object({ mensagem_id: entradaMensagemId }),
    saida: saidaOk,
    anotacoes: ENVIO_WHATSAPP,
    executar: async ({ mensagem_id }, cliente) => {
      await cliente.apagarMensagem(mensagem_id);
      return responder({ ok: true }, 'Mensagem apagada para todos.');
    },
  });

  registrarFerramenta(servidor, contexto, 'marcar_como_lida', {
    titulo: 'Marcar conversa como lida',
    descricao: 'Zera as não lidas de uma conversa e envia o recibo de leitura (tique azul) ao contato.',
    entrada: z.object({ conversa_id: entradaConversaId }),
    saida: saidaOk,
    anotacoes: { readOnlyHint: false, destructiveHint: false, openWorldHint: true, idempotentHint: true },
    executar: async ({ conversa_id }, cliente) => {
      await cliente.marcarLida(conversa_id);
      return responder({ ok: true }, 'Conversa marcada como lida.');
    },
  });
};
