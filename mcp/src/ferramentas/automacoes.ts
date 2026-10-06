// Ferramentas de automações (fluxos, chatbots e IA) — specs/002-automacoes/contracts/mcp-ferramentas.md ›
// Automações: `listar_automacoes`, `ver_automacao`, `ver_formatos_automacao`, `criar_automacao`,
// `editar_automacao`, `validar_automacao`, `excluir_automacao`, `ativar_automacao`,
// `desativar_automacao`, `executar_automacao`, `testar_automacao`, `simular_chatbot`.
import {
  type AlteracaoAutomacao,
  type AlvoExecucao,
  type Automacao,
  type ClienteMotor,
  type Execucao,
  type ExecucaoDetalhe,
  type NovaAutomacao,
  type PedidoTeste,
  type SaidaSimulada,
  TIPOS_GATILHO,
} from '@zapdesk/cliente-motor';
import { z } from 'zod';

import { contaPadrao } from '../conta-padrao.js';
import {
  entradaAutomacaoId,
  entradaContaId,
  entradaObjetoJson,
  saidaAcaoRegistrada,
  saidaAutomacao,
  saidaErroDefinicao,
  saidaExecucao,
  saidaExecucaoDetalhe,
  saidaOk,
} from '../esquemas.js';
import {
  ENVIO_WHATSAPP,
  ESCRITA_LOCAL,
  EXCLUSAO_LOCAL,
  LEITURA,
  type Registrador,
  registrarFerramenta,
} from '../ferramenta.js';
import { TEXTO_FORMATOS_AUTOMACAO } from '../formatos-automacao.js';
import {
  ErroFerramenta,
  formatarData,
  listar,
  plural,
  responder,
  responderErro,
  textoErrosCompilacao,
  textoErrosDefinicao,
} from '../formatar.js';

// ---------------------------------------------------------------------------
// Entradas
// ---------------------------------------------------------------------------

const entradaGatilho = z
  .looseObject({ tipo: z.enum(TIPOS_GATILHO as [string, ...string[]]) })
  .describe(
    'Gatilho, ex.: {"tipo":"palavra_chave","palavras":["preço"]}, {"tipo":"disparo_respondeu"}, ' +
      '{"tipo":"entrou_etapa","funil_id":"…","etapa_id":"…"}, {"tipo":"agendamento","cron":"0 9 * * 1-5"}. ' +
      'Campos de cada tipo em ver_formatos_automacao.',
  );

const entradaLimites = z
  .object({
    anti_loop: z
      .object({ mensagens: z.number().int().min(1), janela_min: z.number().int().min(1) })
      .nullable()
      .optional()
      .describe('Limite próprio de anti-loop (só pode ser mais restritivo que o global); null = usar o global.'),
    tempo_s: z.number().int().nullable().optional(),
    memoria_mb: z.number().int().nullable().optional(),
  })
  .describe('Limites opcionais da automação.');

const camposAutomacao = {
  nome: z.string().trim().min(1).max(80).describe('Nome da automação (1–80 caracteres).'),
  descricao: z.string().max(500).nullable().optional().describe('Descrição curta (opcional).'),
  gatilhos: z.array(entradaGatilho).min(1).describe('Um ou mais gatilhos; basta um casar para disparar.'),
  definicao: entradaObjetoJson.describe(
    'Fluxo: {"versao":1,"condicoes":null|{modo,regras},"acoes":[…]}. Chatbot: {"versao":1,"inicio":"n1",' +
      '"nao_entendi":"…","max_tentativas":3,"inatividade_min":30,"nos":[…]}. Formato completo com exemplos: ver_formatos_automacao.',
  ),
  contas: z
    .array(z.string().min(1))
    .nullable()
    .optional()
    .describe('Contas onde a automação vale (conta_ids); null/omitido = todas.'),
  incluir_grupos: z.boolean().optional().describe('Também reagir em grupos (padrão false).'),
  prioridade: z.number().int().min(1).max(1000).optional().describe('1–1000, menor roda primeiro (padrão 100).'),
  conta_envio_id: z
    .string()
    .min(1)
    .nullable()
    .optional()
    .describe('Conta usada para enviar quando o evento não tem conversa (ex.: lead importado, agendamento).'),
  limites: entradaLimites.optional(),
};

const entradaTipoAutomacao = z.enum(['fluxo', 'chatbot']).describe('"fluxo" (lista de ações) ou "chatbot" (grafo de nós). Automações de IA: use criar_automacao_ia.');

// ---------------------------------------------------------------------------
// Textos
// ---------------------------------------------------------------------------

const ROTULO_TIPO: Record<string, string> = { fluxo: 'fluxo', chatbot: 'chatbot', ia: 'automação de IA' };

function resumoGatilho(g: { tipo: string } & Record<string, unknown>): string {
  switch (g.tipo) {
    case 'palavra_chave':
      return `palavra-chave (${((g['palavras'] as string[] | undefined) ?? []).join(', ')})`;
    case 'mensagem_recebida':
      return g['contem'] ? `mensagem recebida contendo "${String(g['contem'])}"` : 'mensagem recebida';
    case 'agendamento':
      return g['cron'] ? `agendamento (${String(g['cron'])})` : `agendamento (a cada ${String(g['intervalo_s'])} s)`;
    case 'sem_resposta':
      return `sem resposta após ${String(g['apos_s'])} s`;
    default:
      return g.tipo.replace(/_/g, ' ');
  }
}

export function descreverAutomacao(a: Automacao): string {
  const partes = [
    `${a.nome} — ${ROTULO_TIPO[a.tipo] ?? a.tipo}, ${a.ativa ? 'ATIVA' : 'inativa'}`,
    `gatilhos: ${a.gatilhos.map((g) => resumoGatilho(g as never)).join('; ') || 'nenhum'}`,
    `prioridade ${a.prioridade}`,
  ];
  if (a.ia) partes.push(a.ia.compilacao_ok ? 'compilação ok' : `compilação com ${plural(a.ia.erros_compilacao.length, 'erro', 'erros')}`);
  if (a.desativada_motivo === 'erros_seguidos') partes.push('desativada por erros seguidos');
  const e = a.estatisticas_24h;
  if (e.ok + e.erro + e.abortada > 0) partes.push(`24 h: ${e.ok} ok, ${e.erro} erro, ${e.abortada} abortada`);
  if (a.tipo === 'chatbot' && a.sessoes_ativas > 0) partes.push(`${plural(a.sessoes_ativas, 'sessão ativa', 'sessões ativas')}`);
  return `${partes.join(' · ')} (automacao_id: ${a.id})`;
}

function textoAvisos(a: { avisos: Automacao['avisos'] }): string {
  return a.avisos.length > 0
    ? `\nAvisos (referências quebradas; a ação falhará em execução):\n${textoErrosDefinicao(a.avisos)}`
    : '';
}

function textoDetalheAutomacao(a: Automacao): string {
  const linhas = [descreverAutomacao(a)];
  if (a.descricao) linhas.push(`Descrição: ${a.descricao}`);
  linhas.push(`Contas: ${a.contas ? a.contas.join(', ') : 'todas'}${a.incluir_grupos ? ' (inclui grupos)' : ''}`);
  if (a.ia) {
    linhas.push(`Permissões: ${a.ia.permissoes.join(', ') || 'nenhuma'}${a.ia.segredos.length > 0 ? ` · segredos: ${a.ia.segredos.join(', ')}` : ''}`);
    if (!a.ia.compilacao_ok && a.ia.erros_compilacao.length > 0) {
      linhas.push(`Erros de compilação:\n${textoErrosCompilacao(a.ia.erros_compilacao)}`);
    }
    if (a.ia.rodando_versao_anterior) linhas.push('Atenção: a última compilação falhou; execuções reais usam a versão anterior.');
    linhas.push('Arquivos: listar_arquivos_automacao / ler_arquivo_automacao.');
  } else if (a.definicao) {
    linhas.push(`Definição: ${JSON.stringify(a.definicao)}`);
  }
  return linhas.join('\n') + textoAvisos(a);
}

/** Resumo de uma execução/teste para a IA: estado, erro, ações, retorno, tokens e log. */
export function textoExecucao(e: Execucao | ExecucaoDetalhe, maxLog = 6000): string {
  const linhas = [
    `Execução ${e.id} de "${e.automacao_nome}": ${e.estado}${e.simulacao ? ' (simulação — nada foi enviado)' : ''}` +
      ` · origem ${e.origem} · início ${formatarData(e.iniciada_em)}${e.duracao_ms !== null ? ` · ${e.duracao_ms} ms` : ''}`,
  ];
  if (e.motivo) linhas.push(`Motivo: ${e.motivo}`);
  if (e.erro) linhas.push(`Erro: ${e.erro}`);
  if ('erro_stack' in e && e.erro_stack) linhas.push(`Stack:\n${e.erro_stack}`);
  if (e.acoes.length > 0) {
    linhas.push(
      `${e.simulacao ? 'Ações que seriam feitas' : 'Ações'}:\n${listar(
        e.acoes,
        (a) => `${a.tipo}${a.alvo ? ` → ${a.alvo}` : ''}: ${a.resultado}${a.detalhe ? ` — ${a.detalhe}` : ''}`,
        200,
      )}`,
    );
  } else {
    linhas.push('Nenhuma ação registrada.');
  }
  if (e.retorno !== null && e.retorno !== undefined) linhas.push(`Retorno: ${JSON.stringify(e.retorno)}`);
  if (e.tokens.entrada + e.tokens.saida > 0) linhas.push(`Tokens de IA: ${e.tokens.entrada} entrada, ${e.tokens.saida} saída`);
  if (e.retomar_em) linhas.push(`Aguardando até ${formatarData(e.retomar_em)}.`);
  if ('variaveis' in e && Object.keys(e.variaveis).length > 0) linhas.push(`Variáveis: ${JSON.stringify(e.variaveis)}`);
  if ('log' in e && e.log) {
    const log = e.log.length > maxLog ? `…${e.log.slice(-maxLog)}` : e.log;
    linhas.push(`Log${e.log_truncado ? ' (truncado)' : ''}:\n${log}`);
  }
  return linhas.join('\n');
}

function textoSaidas(saidas: readonly SaidaSimulada[]): string {
  if (saidas.length === 0) return '  (sem resposta)';
  return saidas.map((s) => `  ${s.tipo === 'mensagem' ? 'Bot' : s.tipo === 'acao' ? 'Ação' : 'Aviso'}: ${s.texto}`).join('\n');
}

/** Monta o alvo de execução/teste (no máximo um). */
async function montarAlvo(
  cliente: ClienteMotor,
  args: { conversa_id?: string | undefined; contato_id?: string | undefined; lead_id?: string | undefined; telefone?: string | undefined; conta_id?: string | undefined },
): Promise<AlvoExecucao> {
  const informados = [args.conversa_id, args.contato_id, args.lead_id, args.telefone].filter((v) => v !== undefined);
  if (informados.length > 1) {
    throw new ErroFerramenta('Informe no máximo um alvo: conversa_id, contato_id, lead_id ou telefone (+ conta_id).');
  }
  if (args.conversa_id) return { conversa_id: args.conversa_id };
  if (args.contato_id) return { contato_id: args.contato_id };
  if (args.lead_id) return { lead_id: args.lead_id };
  if (args.telefone) return { telefone: args.telefone, conta_id: await contaPadrao(cliente, args.conta_id) };
  return {};
}

const ESTADOS_FINAIS = new Set(['ok', 'erro', 'simulacao', 'abortada', 'aguardando']);
const esperar = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));

/** Acompanha a execução até terminar (ou `prazoMs`), para devolver o resultado útil de uma vez. */
async function acompanhar(cliente: ClienteMotor, execucao: Execucao, prazoMs: number): Promise<Execucao | ExecucaoDetalhe> {
  if (ESTADOS_FINAIS.has(execucao.estado)) return cliente.obterExecucao(execucao.id).catch(() => execucao);
  const inicio = Date.now();
  let atual: Execucao | ExecucaoDetalhe = execucao;
  while (Date.now() - inicio < prazoMs) {
    await esperar(250);
    atual = await cliente.obterExecucao(execucao.id);
    if (ESTADOS_FINAIS.has(atual.estado)) break;
  }
  return atual;
}

function corpoNovaAutomacao(tipo: 'fluxo' | 'chatbot', args: Record<string, unknown>): NovaAutomacao {
  const corpo: Record<string, unknown> = { tipo };
  for (const chave of ['nome', 'descricao', 'gatilhos', 'definicao', 'contas', 'incluir_grupos', 'prioridade', 'conta_envio_id', 'limites']) {
    if (args[chave] !== undefined) corpo[chave] = args[chave];
  }
  return corpo as unknown as NovaAutomacao;
}

// ---------------------------------------------------------------------------
// Registro
// ---------------------------------------------------------------------------

export const registrarAutomacoes: Registrador = (servidor, contexto) => {
  registrarFerramenta(servidor, contexto, 'listar_automacoes', {
    titulo: 'Listar automações',
    descricao:
      'Lista as automações (fluxos, chatbots e automações de IA) em ordem de prioridade, com estado (ativa/inativa), ' +
      'gatilhos, estatísticas das últimas 24 h e, nas de IA, se a compilação está ok. Filtre por tipo e ativa.',
    entrada: z.object({
      tipo: z.enum(['fluxo', 'chatbot', 'ia']).optional().describe('Só deste tipo.'),
      ativa: z.boolean().optional().describe('true = só ativas; false = só inativas.'),
    }),
    saida: z.object({ automacoes: z.array(saidaAutomacao) }),
    anotacoes: LEITURA,
    executar: async (args, cliente) => {
      const automacoes = await cliente.listarAutomacoes(args);
      const texto =
        automacoes.length === 0
          ? 'Nenhuma automação encontrada. Crie com criar_automacao (fluxo/chatbot) ou criar_automacao_ia.'
          : `${plural(automacoes.length, 'automação', 'automações')}:\n${listar(automacoes, descreverAutomacao, 200)}`;
      return responder({ automacoes }, texto);
    },
  });

  registrarFerramenta(servidor, contexto, 'ver_automacao', {
    titulo: 'Ver automação',
    descricao:
      'Mostra uma automação completa: gatilhos, definição JSON (fluxo/chatbot), avisos de referências quebradas, ' +
      'permissões e erros de compilação (IA), estatísticas e se está ativa.',
    entrada: z.object({ automacao_id: entradaAutomacaoId }),
    saida: saidaAutomacao,
    anotacoes: LEITURA,
    executar: async ({ automacao_id }, cliente) => {
      const automacao = await cliente.obterAutomacao(automacao_id);
      return responder({ ...automacao }, textoDetalheAutomacao(automacao));
    },
  });

  registrarFerramenta(servidor, contexto, 'ver_formatos_automacao', {
    titulo: 'Ver formatos de automação',
    descricao:
      'Devolve a referência dos formatos JSON das automações, com exemplos: todos os gatilhos, condições, ações, a ' +
      'definição de fluxo, o grafo de chatbot (nós inicio/mensagem/menu/pergunta/condicao/acao/ia/humano/fim), o ' +
      'manifesto automacao.json e o mapa gatilho → handler das automações de IA. Consulte ANTES de criar ou editar.',
    entrada: z.object({}),
    saida: z.object({ texto: z.string() }),
    anotacoes: LEITURA,
    executar: async () => responder({ texto: TEXTO_FORMATOS_AUTOMACAO }, TEXTO_FORMATOS_AUTOMACAO),
  });

  registrarFerramenta(servidor, contexto, 'criar_automacao', {
    titulo: 'Criar automação (fluxo ou chatbot)',
    descricao:
      'Cria um fluxo ("quando X, faça Y") ou um chatbot (menu/perguntas em grafo). Nasce INATIVA, salvo `ativar: true`. ' +
      'Formatos de gatilhos, condições, ações e nós: ver_formatos_automacao. Recomendado: validar_automacao antes, ' +
      'testar_automacao (fluxo) ou simular_chatbot (chatbot) depois, e só então ativar. Erros de formato voltam com o ' +
      'caminho do campo. Automações de IA (código): use criar_automacao_ia.',
    entrada: z.object({
      tipo: entradaTipoAutomacao,
      ...camposAutomacao,
      ativar: z.boolean().optional().describe('true = ativa logo após criar (passa a agir em mensagens reais).'),
    }),
    saida: saidaAutomacao,
    anotacoes: ESCRITA_LOCAL,
    executar: async (args, cliente) => {
      let automacao = await cliente.criarAutomacao(corpoNovaAutomacao(args.tipo, args));
      if (args.ativar) {
        try {
          automacao = await cliente.ativarAutomacao(automacao.id);
        } catch (erro) {
          const falha = responderErro(erro);
          const motivo = (falha.content[0] as { text: string }).text;
          return {
            ...falha,
            content: [
              {
                type: 'text',
                text: `A automação foi criada (automacao_id: ${automacao.id}) mas NÃO foi ativada:\n${motivo}\nCorrija com editar_automacao e ative com ativar_automacao.`,
              },
            ],
          };
        }
      }
      return responder(
        { ...automacao },
        `Automação criada: ${descreverAutomacao(automacao)}${textoAvisos(automacao)}` +
          (automacao.ativa ? '' : `\nEla está inativa. Teste com ${automacao.tipo === 'chatbot' ? 'simular_chatbot' : 'testar_automacao'} e ative com ativar_automacao.`),
      );
    },
  });

  registrarFerramenta(servidor, contexto, 'editar_automacao', {
    titulo: 'Editar automação',
    descricao:
      'Altera um fluxo ou chatbot: informe só os campos a mudar (gatilhos e definicao são substituídos por inteiro). ' +
      'Mudanças em gatilhos/definição/limites criam uma nova versão; execuções em andamento terminam na versão antiga. ' +
      '`ativar` true/false ativa ou desativa após salvar. Automações de IA: edite automacao.json/arquivos com escrever_arquivo_automacao.',
    entrada: z.object({
      automacao_id: entradaAutomacaoId,
      nome: camposAutomacao.nome.optional(),
      descricao: camposAutomacao.descricao,
      gatilhos: camposAutomacao.gatilhos.optional(),
      definicao: camposAutomacao.definicao.optional(),
      contas: camposAutomacao.contas,
      incluir_grupos: camposAutomacao.incluir_grupos,
      prioridade: camposAutomacao.prioridade,
      conta_envio_id: camposAutomacao.conta_envio_id,
      limites: camposAutomacao.limites,
      ativar: z.boolean().optional().describe('true = ativar; false = desativar (após salvar).'),
    }),
    saida: saidaAutomacao,
    anotacoes: { ...ESCRITA_LOCAL, idempotentHint: true },
    executar: async ({ automacao_id, ativar, ...campos }, cliente) => {
      const alteracao: Record<string, unknown> = {};
      for (const [chave, valor] of Object.entries(campos)) if (valor !== undefined) alteracao[chave] = valor;
      if (Object.keys(alteracao).length === 0 && ativar === undefined) {
        throw new ErroFerramenta('Informe pelo menos um campo para alterar (ou `ativar`).');
      }
      let automacao: Automacao;
      if (Object.keys(alteracao).length > 0) {
        const atual = await cliente.obterAutomacao(automacao_id);
        if (atual.tipo === 'ia') {
          throw new ErroFerramenta(
            'Esta é uma automação de IA: nome, gatilhos, permissões, contas e limites ficam no arquivo automacao.json. ' +
              'Leia com ler_arquivo_automacao, altere com escrever_arquivo_automacao e rode compilar_automacao.',
          );
        }
        automacao = await cliente.editarAutomacao(automacao_id, alteracao as AlteracaoAutomacao);
      } else {
        automacao = await cliente.obterAutomacao(automacao_id);
      }
      if (ativar === true && !automacao.ativa) automacao = await cliente.ativarAutomacao(automacao_id);
      if (ativar === false && automacao.ativa) automacao = await cliente.desativarAutomacao(automacao_id);
      return responder({ ...automacao }, `Automação atualizada (versão ${automacao.versao}): ${descreverAutomacao(automacao)}${textoAvisos(automacao)}`);
    },
  });

  registrarFerramenta(servidor, contexto, 'validar_automacao', {
    titulo: 'Validar automação',
    descricao:
      'Confere um fluxo ou chatbot SEM gravar: devolve `erros` (impedem salvar/ativar; cada um com o caminho do campo) ' +
      'e `avisos` (referências a etiquetas/etapas/templates inexistentes). Mesmos campos de criar_automacao.',
    entrada: z.object({ tipo: entradaTipoAutomacao, ...camposAutomacao }),
    saida: z.object({ erros: z.array(saidaErroDefinicao), avisos: z.array(saidaErroDefinicao) }),
    anotacoes: LEITURA,
    executar: async (args, cliente) => {
      const resultado = await cliente.validarAutomacao(corpoNovaAutomacao(args.tipo, args));
      const partes: string[] = [];
      partes.push(
        resultado.erros.length === 0
          ? 'Válida: nenhum erro.'
          : `${plural(resultado.erros.length, 'erro', 'erros')}:\n${textoErrosDefinicao(resultado.erros)}`,
      );
      if (resultado.avisos.length > 0) partes.push(`${plural(resultado.avisos.length, 'aviso', 'avisos')}:\n${textoErrosDefinicao(resultado.avisos)}`);
      return responder({ ...resultado }, partes.join('\n'));
    },
  });

  registrarFerramenta(servidor, contexto, 'excluir_automacao', {
    titulo: 'Excluir automação',
    descricao:
      'Exclui a automação com suas execuções, sessões de chatbot, esperas e memória (irreversível). Nas de IA, apaga ' +
      'também a pasta do projeto. Para só parar, use desativar_automacao.',
    entrada: z.object({ automacao_id: entradaAutomacaoId }),
    saida: saidaOk,
    anotacoes: EXCLUSAO_LOCAL,
    executar: async ({ automacao_id }, cliente) => {
      await cliente.excluirAutomacao(automacao_id);
      return responder({ ok: true }, 'Automação excluída.');
    },
  });

  registrarFerramenta(servidor, contexto, 'ativar_automacao', {
    titulo: 'Ativar automação',
    descricao:
      'Ativa a automação: a partir daqui ela reage a mensagens e eventos REAIS e pode enviar mensagens pelo WhatsApp ' +
      '(sempre pelo portão de segurança: pausas, anti-loop, grupos, primeiro contato). Automação de IA precisa compilar ' +
      'sem erros (os erros voltam como arquivo:linha:coluna). Teste antes com testar_automacao/simular_chatbot.',
    entrada: z.object({ automacao_id: entradaAutomacaoId }),
    saida: saidaAutomacao,
    anotacoes: { readOnlyHint: false, destructiveHint: false, openWorldHint: true },
    executar: async ({ automacao_id }, cliente) => {
      const automacao = await cliente.ativarAutomacao(automacao_id);
      return responder({ ...automacao }, `Automação ativada: ${descreverAutomacao(automacao)}${textoAvisos(automacao)}`);
    },
  });

  registrarFerramenta(servidor, contexto, 'desativar_automacao', {
    titulo: 'Desativar automação',
    descricao: 'Desativa a automação (para de reagir a gatilhos; esperas pendentes de gatilho são descartadas). Pode reativar depois.',
    entrada: z.object({ automacao_id: entradaAutomacaoId }),
    saida: saidaAutomacao,
    anotacoes: { ...ESCRITA_LOCAL, idempotentHint: true },
    executar: async ({ automacao_id }, cliente) => {
      const automacao = await cliente.desativarAutomacao(automacao_id);
      return responder({ ...automacao }, `Automação desativada: ${descreverAutomacao(automacao)}`);
    },
  });

  registrarFerramenta(servidor, contexto, 'executar_automacao', {
    titulo: 'Executar automação agora',
    descricao:
      'Executa a automação AGORA de verdade (pode enviar mensagens reais), sobre um alvo opcional: conversa_id, ' +
      'contato_id, lead_id ou telefone (+ conta_id). Automação de IA recebe `entrada` em aoExecutar e o retorno volta ' +
      'aqui. Espera até ~15 s pelo fim e devolve estado, ações, retorno e log; se ainda estiver rodando, acompanhe com ' +
      'ver_execucao. Para experimentar sem enviar nada, use testar_automacao.',
    entrada: z.object({
      automacao_id: entradaAutomacaoId,
      conversa_id: z.string().min(1).optional().describe('Conversa alvo.'),
      contato_id: z.string().min(1).optional().describe('Contato alvo.'),
      lead_id: z.string().min(1).optional().describe('Lead alvo.'),
      telefone: z.string().min(1).optional().describe('Telefone alvo em qualquer formato (use com conta_id).'),
      conta_id: entradaContaId.optional(),
      entrada: z.unknown().optional().describe('JSON entregue a aoExecutar(ctx, entrada) nas automações de IA.'),
    }),
    saida: saidaExecucao,
    anotacoes: ENVIO_WHATSAPP,
    executar: async (args, cliente) => {
      const alvo = await montarAlvo(cliente, args);
      const execucao = await cliente.executarAutomacao(args.automacao_id, {
        ...alvo,
        ...(args.entrada !== undefined ? { entrada: args.entrada } : {}),
        origem: 'manual_mcp',
      });
      const final = await acompanhar(cliente, execucao, 15_000);
      const aindaRodando = !ESTADOS_FINAIS.has(final.estado);
      return responder(
        { ...final },
        textoExecucao(final) + (aindaRodando ? `\nAinda em andamento: acompanhe com ver_execucao (execucao_id: ${final.id}).` : ''),
      );
    },
  });

  registrarFerramenta(servidor, contexto, 'testar_automacao', {
    titulo: 'Testar automação (simulação)',
    descricao:
      'Roda um fluxo ou automação de IA em SIMULAÇÃO: nada é enviado nem gravado; devolve as ações que seriam feitas, ' +
      'o log (console.log/ctx.log), o erro com stack e o retorno. Informe `texto` para simular uma mensagem recebida ' +
      '(aoReceberMensagem/gatilhos de mensagem; sem conversa_id usa um contato fictício), `mensagem_id` para usar uma ' +
      'mensagem real, `entrada` para aoExecutar ou `evento` ({tipo, dados}) para aoEvento. Automação de IA é compilada ' +
      'antes (erros de compilação voltam como arquivo:linha:coluna). `ia_simulada: true` usa respostas de IA falsas ' +
      '(não gasta tokens nem precisa de chave). Chatbot: use simular_chatbot.',
    entrada: z.object({
      automacao_id: entradaAutomacaoId,
      texto: z.string().min(1).max(4096).optional().describe('Mensagem simulada recebida do contato.'),
      mensagem_id: z.string().min(1).optional().describe('ID de uma mensagem real recebida para usar como gatilho.'),
      conversa_id: z.string().min(1).optional().describe('Conversa real usada como contexto (histórico, contato, lead).'),
      entrada: z.unknown().optional().describe('JSON para aoExecutar.'),
      evento: z
        .object({ tipo: z.string().min(1), dados: z.record(z.string(), z.unknown()) })
        .optional()
        .describe('Evento para aoEvento/gatilhos não-mensagem, ex.: {"tipo":"etiqueta","dados":{…}}.'),
      ia_simulada: z.boolean().optional().describe('true = IA falsa determinística (padrão false: usa a Claude API real, se houver chave).'),
    }),
    saida: saidaExecucaoDetalhe,
    anotacoes: { readOnlyHint: true, openWorldHint: true },
    executar: async (args, cliente) => {
      const pedido: PedidoTeste = {};
      if (args.texto !== undefined) pedido.mensagem = { texto: args.texto, ...(args.conversa_id ? { conversa_id: args.conversa_id } : {}) };
      else if (args.conversa_id) pedido.alvo = { conversa_id: args.conversa_id };
      if (args.mensagem_id) pedido.mensagem_id = args.mensagem_id;
      if (args.entrada !== undefined) pedido.entrada = args.entrada;
      if (args.evento) pedido.evento = args.evento;
      if (args.ia_simulada !== undefined) pedido.ia_simulada = args.ia_simulada;
      const { execucao } = await cliente.testarAutomacao(args.automacao_id, pedido);
      return responder({ ...execucao }, textoExecucao(execucao, 12_000));
    },
  });

  registrarFerramenta(servidor, contexto, 'simular_chatbot', {
    titulo: 'Simular chatbot',
    descricao:
      'Conversa com um chatbot em SIMULAÇÃO (nada é enviado): o bot abre a conversa e cada item de `mensagens` é uma ' +
      'resposta do contato, em sequência. Devolve, por rodada, o que o bot respondeu, o nó atual, as variáveis ' +
      'capturadas e o estado (ativa, concluida, humano…). Use para conferir todos os caminhos antes de ativar.',
    entrada: z.object({
      automacao_id: entradaAutomacaoId,
      mensagens: z.array(z.string().max(4096)).max(50).describe('Respostas do contato, em ordem, ex.: ["1", "ana@x.com"].'),
      ia_simulada: z.boolean().optional().describe('true = nós de IA usam respostas falsas (padrão false).'),
    }),
    saida: z.object({
      rodadas: z.array(
        z.object({
          entrada: z.string().nullable().describe('Mensagem do contato (null = abertura do bot).'),
          saidas: z.array(z.looseObject({ tipo: z.string(), texto: z.string(), no_id: z.string().nullable() })),
          no_atual: z.string().nullable(),
          variaveis: z.record(z.string(), z.string()),
          estado: z.string(),
          acoes: z.array(saidaAcaoRegistrada).optional(),
        }),
      ),
    }),
    anotacoes: LEITURA,
    executar: async ({ automacao_id, mensagens, ia_simulada }, cliente) => {
      const inicio = await cliente.iniciarSimulador(automacao_id, ia_simulada !== undefined ? { ia_simulada } : {});
      const rodadas: {
        entrada: string | null;
        saidas: SaidaSimulada[];
        no_atual: string | null;
        variaveis: Record<string, string>;
        estado: string;
        acoes?: unknown[];
      }[] = [{ entrada: null, saidas: inicio.saidas, no_atual: inicio.no_atual, variaveis: inicio.variaveis, estado: inicio.estado }];
      let naoEnviadas = 0;
      try {
        let estado = inicio.estado;
        for (const [i, texto] of mensagens.entries()) {
          if (estado !== 'ativa') {
            naoEnviadas = mensagens.length - i;
            break;
          }
          const r = await cliente.enviarAoSimulador(inicio.simulacao_id, texto);
          rodadas.push({ entrada: texto, saidas: r.saidas, no_atual: r.no_atual, variaveis: r.variaveis, estado: r.estado, acoes: r.acoes });
          estado = r.estado;
        }
      } finally {
        await cliente.encerrarSimulador(inicio.simulacao_id).catch(() => undefined);
      }
      const texto = rodadas
        .map((r, i) => {
          const cabeca = r.entrada === null ? 'Abertura' : `Rodada ${i} — contato: "${r.entrada}"`;
          const vars = Object.keys(r.variaveis).length > 0 ? ` · variáveis ${JSON.stringify(r.variaveis)}` : '';
          return `${cabeca}\n${textoSaidas(r.saidas)}\n  → nó ${r.no_atual ?? '—'} · estado ${r.estado}${vars}`;
        })
        .join('\n');
      const final = rodadas[rodadas.length - 1];
      const aviso =
        naoEnviadas > 0
          ? `\nA sessão terminou (${final?.estado}) antes de ${plural(naoEnviadas, 'mensagem', 'mensagens')}, que não foram enviadas.`
          : '';
      return responder({ rodadas }, texto + aviso);
    },
  });
};
