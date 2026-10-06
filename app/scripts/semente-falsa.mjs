// 003 T005 — semente FICTÍCIA para capturas do app (contracts/capturas.md → "Semente fictícia").
// Usa só a API do motor e as rotas /v1/falso/* (motor em modo falso). Nomes e telefones espelham
// os mockups da landing (zapdesk-site/src/content/mockups.ts); telefones +55 11 90000-01xx.
// Determinística: cada passo fixa o relógio do motor antes de agir, e o disparo anda por saltos
// do relógio (intervalo 60 s), então duas rodadas produzem os mesmos dados.
//
// Uso como módulo: `import { semear } from './semente-falsa.mjs'; await semear({ base, token })`
//   base = "http://127.0.0.1:<porta>" (com ou sem "/v1").
// Uso pela linha de comando: node app/scripts/semente-falsa.mjs --runtime <pasta>/runtime.json
//   (grava <pasta>/semente.json com os ids).
import { readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

export const AGORA = '2026-10-05T10:42:00-03:00';

const esperar = (ms) => new Promise((r) => setTimeout(r, ms));

function criarApi(base, token) {
  const raiz = base.replace(/\/+$/, '').replace(/\/v1$/, '') + '/v1';
  return async function api(metodo, caminho, corpo, { aceitar = [] } = {}) {
    const r = await fetch(raiz + caminho, {
      method: metodo,
      headers: { authorization: `Bearer ${token}`, 'content-type': 'application/json' },
      body: corpo === undefined ? undefined : JSON.stringify(corpo),
    });
    const texto = await r.text();
    let json = null;
    try {
      json = texto ? JSON.parse(texto) : null;
    } catch {
      json = texto;
    }
    if (!r.ok && !aceitar.includes(r.status)) {
      throw new Error(`semente: ${metodo} ${caminho} → ${r.status} ${texto.slice(0, 300)}`);
    }
    return json;
  };
}

async function ate(fn, ms = 10000, passo = 50) {
  const fim = Date.now() + ms;
  for (;;) {
    const v = await fn();
    if (v) return v;
    if (Date.now() > fim) throw new Error('semente: tempo esgotado esperando o motor');
    await esperar(passo);
  }
}

const em = (dia, hora) => `2026-10-${dia}T${hora}:00-03:00`;
const RITMO_60 = {
  intervalo_min_s: 60,
  intervalo_max_s: 60,
  limite_por_hora: null,
  limite_por_dia: null,
  pausa_a_cada: null,
  pausa_duracao_s: null,
};

// Leads do disparo (nomes do RelatorioMockup/KanbanMockup).
const LEADS = [
  ['+5511900000101', 'Juliana Prado', 'Studio Prado', 'Campinas'],
  ['+5511900000117', 'Carlos Menezes', 'Menezes & Filhos', 'São Paulo'],
  ['+5511900000123', 'Padaria Trigo Bom', 'Trigo Bom', 'Santos'],
  ['+5511900000138', 'Beatriz Lima', 'Lima Arquitetura', 'Jundiaí'],
  ['+5511900000144', 'Otávio Reis', 'Reis Transportes', 'Sorocaba'],
  ['+5511900000159', 'Studio Ponto Fixo', 'Ponto Fixo', 'São Paulo'],
  ['+5511900000102', 'Fernanda Alves', 'Alves Doces', 'Campinas'],
  ['+5511900000103', 'Gustavo Rocha', 'Rocha Auto', 'Guarulhos'],
  ['+5511900000104', 'Helena Duarte', 'Duarte Moda', 'São Paulo'],
  ['+5511900000105', 'Igor Martins', 'Martins TI', 'Osasco'],
  ['+5511900000106', 'Larissa Gomes', 'Gomes Pet', 'Santo André'],
  ['+5511900000107', 'Mateus Freitas', 'Freitas Obras', 'Barueri'],
];
const FALHAM = ['+5511900000138', '+5511900000103'];

/**
 * Semeia o motor (modo falso) com dados fictícios. Devolve os ids usados pelas capturas.
 * @param {{ base: string, token: string, log?: (m: string) => void }} opcoes
 */
export async function semear({ base, token, log = () => {} }) {
  const api = criarApi(base, token);
  const relogio = (agora) => api('PUT', '/falso/relogio', { agora });
  const ids = {};

  // --------------------------------------------------------------- conta
  await relogio(em('03', '08:00'));
  const conta = await api('POST', '/contas', { nome: 'Comercial' });
  ids.conta = conta.id;
  await api('POST', `/falso/contas/${conta.id}/escanear-qr`, { telefone: '+5511900000001', nome: 'Comercial' });
  await ate(async () => (await api('GET', `/contas/${conta.id}`)).estado === 'conectada');
  log('conta conectada');

  // --------------------------------------------------------------- etiquetas, templates, leads
  const etq = {};
  for (const [nome, cor] of [
    ['Quente', '#fc7e7e'],
    ['Orçamento', '#ffbc38'],
    ['Morno', '#53bdeb'],
    ['Cliente', '#25d366'], // verde antigo: cor salva pelo usuário não muda (FR-016)
  ]) {
    etq[nome] = (await api('POST', '/etiquetas', { nome, cor })).id;
  }
  ids.etiquetas = etq;

  const tpl = {};
  for (const [nome, texto] of [
    ['Boas-vindas', 'Oi {primeiro_nome}! Aqui é da {empresa}. Posso te mandar o catálogo?'],
    ['Follow-up', 'Ainda tem interesse, {primeiro_nome}? Separei uma condição especial para você.'],
    ['Orçamento', 'Segue o orçamento, {nome}. Qualquer dúvida é só chamar por aqui.'],
  ]) {
    tpl[nome] = (await api('POST', '/templates', { nome, texto })).id;
  }
  ids.templates = tpl;

  const importacao = await api('POST', '/leads/importar', {
    origem: 'csv',
    leads: LEADS.map(([telefone, nome, empresa, cidade]) => ({ telefone, nome, campos: { empresa, cidade } })),
  });
  const leadPorTelefone = Object.fromEntries(importacao.novos.map((n) => [n.telefone, n.lead_id]));
  ids.leads = importacao.novos.map((n) => n.lead_id);
  log(`${ids.leads.length} leads`);

  // --------------------------------------------------------------- disparos antigos
  // concluído (03/10)
  await relogio(em('03', '09:00'));
  const concluido = await api('POST', '/disparos', {
    conta_id: conta.id,
    nome: 'Boas-vindas de setembro',
    mensagem: 'Oi {nome}! Obrigado pelo contato.',
    iniciar: true,
    destinatarios: { lead_ids: [leadPorTelefone['+5511900000104'], leadPorTelefone['+5511900000105']] },
    ritmo: RITMO_60,
  });
  for (let i = 0; i < 4; i++) {
    const d = await api('GET', `/disparos/${concluido.id}`);
    if (d.estado === 'concluido') break;
    await api('PUT', '/falso/relogio', { avancar_s: 60 });
    await esperar(150);
  }
  await ate(async () => (await api('GET', `/disparos/${concluido.id}`)).estado === 'concluido');
  ids.disparoConcluido = concluido.id;

  // cancelado (03/10)
  await relogio(em('03', '11:00'));
  const cancelado = await api('POST', '/disparos', {
    conta_id: conta.id,
    nome: 'Promoção relâmpago',
    mensagem: 'Só hoje: frete grátis, {nome}!',
    destinatarios: { lead_ids: [leadPorTelefone['+5511900000106']] },
    ritmo: RITMO_60,
    inicio_em: em('04', '09:00'),
  });
  await api('POST', `/disparos/${cancelado.id}/cancelar`);
  ids.disparoCancelado = cancelado.id;

  // --------------------------------------------------------------- disparo em andamento (pausado)
  await api('PUT', '/falso/falhas-envio', { telefones: FALHAM, erro: 'Número não recebe mensagens' });
  await relogio(em('04', '09:00'));
  const andamento = await api('POST', '/disparos', {
    conta_id: conta.id,
    nome: 'Retorno de outubro',
    mensagem: 'Oi {nome}! Chegou a coleção de outubro. Quer ver o catálogo?',
    iniciar: true,
    destinatarios: { lead_ids: LEADS.map(([t]) => leadPorTelefone[t]) },
    ritmo: RITMO_60,
  });
  ids.disparoAndamento = andamento.id;
  const processados = async () => {
    const c = (await api('GET', `/disparos/${andamento.id}`)).contadores;
    return c.total - c.pendente - c.enviando;
  };
  await ate(async () => (await processados()) >= 1);
  while ((await processados()) < 9) {
    const antes = await processados();
    await api('PUT', '/falso/relogio', { avancar_s: 60 });
    await ate(async () => (await processados()) > antes);
  }
  await api('POST', `/disparos/${andamento.id}/pausar`);
  await ate(async () => (await api('GET', `/disparos/${andamento.id}`)).estado === 'pausado');
  const destinatarios = (await api('GET', `/disparos/${andamento.id}/destinatarios?limite=50`)).itens;
  const porTelefone = Object.fromEntries(destinatarios.map((d) => [d.telefone, d]));
  // recibos: entregue / lido; respostas
  await relogio(em('04', '09:30'));
  for (const [tel, tipo] of [
    ['+5511900000101', 'lido'],
    ['+5511900000117', 'lido'],
    ['+5511900000123', 'entregue'],
    ['+5511900000102', 'lido'],
    ['+5511900000104', 'entregue'],
  ]) {
    const wa = porTelefone[tel]?.mensagem_wa_id;
    if (wa) {
      await api('POST', `/falso/contas/${conta.id}/recibo`, { wa_id: wa, tipo: 'entregue' });
      if (tipo === 'lido') await api('POST', `/falso/contas/${conta.id}/recibo`, { wa_id: wa, tipo: 'lido' });
    }
  }
  await relogio(em('04', '09:40'));
  await api('POST', `/falso/contas/${conta.id}/mensagem-recebida`, {
    de: '+5511900000117',
    nome: 'Carlos Menezes',
    texto: 'Quero ver sim! Manda o catálogo.',
    em: em('04', '09:40'),
  });

  // agendado (amanhã)
  await relogio(em('04', '12:00'));
  const agendado = await api('POST', '/disparos', {
    conta_id: conta.id,
    nome: 'Lembrete de visita',
    mensagem: 'Oi {nome}, lembrando da visita amanhã às 14h.',
    iniciar: true,
    destinatarios: { lead_ids: [leadPorTelefone['+5511900000159'], leadPorTelefone['+5511900000144']] },
    ritmo: RITMO_60,
    inicio_em: em('06', '09:00'),
  });
  ids.disparoAgendado = agendado.id;
  log('disparos prontos');

  // --------------------------------------------------------------- funil
  await relogio(em('04', '14:00'));
  const funil = await api('POST', '/funis', {
    nome: 'Prospecção',
    etapas: [
      { nome: 'Novo', cor: '#53bdeb' },
      { nome: 'Qualificando', cor: '#ffbc38' },
      { nome: 'Proposta', cor: '#8b64d6' },
      { nome: 'Fechado', cor: '#25d366' },
    ],
  });
  ids.funil = funil.id;
  const [eNovo, eQual, eProp, eFech] = funil.etapas.map((e) => e.id);
  const cartoes = [
    ['+5511900000101', eNovo, '04', '14:00'],
    ['+5511900000144', eNovo, '05', '08:40'],
    ['+5511900000159', eNovo, '05', '10:20'],
    ['+5511900000117', eQual, '04', '16:00'],
    ['+5511900000123', eQual, '05', '09:10'],
    ['+5511900000102', eProp, '05', '07:30'],
    ['+5511900000104', eFech, '03', '17:00'],
  ];
  for (const [tel, etapa, dia, hora] of cartoes) {
    // :30 s — longe da virada do minuto, para "há 21 min" não oscilar com milissegundos.
    await relogio(em(dia, hora).replace(':00-03:00', ':30-03:00'));
    await api('PUT', `/funis/${funil.id}/cards`, { lead_id: leadPorTelefone[tel], etapa_id: etapa });
  }
  log('funil pronto');

  // --------------------------------------------------------------- automações
  await relogio(em('04', '15:00'));
  const fluxo = await api('POST', '/automacoes', {
    tipo: 'fluxo',
    nome: 'Pediu preço → Qualificando',
    descricao: 'Marca como quente e move no funil quando o lead pergunta preço.',
    gatilhos: [{ tipo: 'palavra_chave', palavras: ['preço', 'tabela'] }],
    definicao: {
      versao: 1,
      condicoes: {
        modo: 'todas',
        regras: [
          { tipo: 'horario', inicio: '08:00', fim: '20:00', dias: [1, 2, 3, 4, 5, 6, 7] },
          { tipo: 'etiqueta', operador: 'nao_tem', etiqueta_id: etq['Cliente'] },
        ],
      },
      acoes: [
        { tipo: 'adicionar_etiqueta', etiqueta_id: etq['Quente'] },
        { tipo: 'mover_etapa', funil_id: funil.id, etapa_id: eQual },
        { tipo: 'enviar_texto', texto: 'Oi {primeiro_nome}! Já te mando a tabela de preços.' },
      ],
    },
  });
  await api('POST', `/automacoes/${fluxo.id}/ativar`);
  ids.fluxo = fluxo.id;

  const chatbot = await api('POST', '/automacoes', {
    tipo: 'chatbot',
    nome: 'Atendimento inicial',
    gatilhos: [{ tipo: 'palavra_chave', palavras: ['atendimento'] }],
    definicao: {
      versao: 1,
      inicio: 'n1',
      nao_entendi: 'Não entendi. Responda com uma das opções.',
      max_tentativas: 3,
      inatividade_min: 30,
      nos: [
        { id: 'n1', tipo: 'inicio', proximo: 'n2', posicao: { x: 0, y: 0 } },
        { id: 'n2', tipo: 'mensagem', texto: 'Olá, {nome}!', template_id: null, proximo: 'n3', posicao: { x: 0, y: 140 } },
        {
          id: 'n3',
          tipo: 'menu',
          texto: 'Como posso ajudar?',
          opcoes: [
            { rotulo: 'Preços', valores: ['preco', 'valores'], proximo: 'n4' },
            { rotulo: 'Falar com vendedor', valores: [], proximo: 'n8' },
          ],
          mostrar_numeros: true,
          ao_esgotar: null,
          posicao: { x: 0, y: 300 },
        },
        {
          id: 'n4',
          tipo: 'pergunta',
          texto: 'Qual seu e-mail?',
          variavel: 'email',
          validacao: { tipo: 'email', padrao: null, mensagem_erro: 'E-mail inválido. Tente de novo.' },
          proximo: 'n9',
          ao_esgotar: null,
          posicao: { x: -200, y: 480 },
        },
        { id: 'n8', tipo: 'humano', mensagem: 'Vou chamar um atendente, só um instante.', posicao: { x: 220, y: 480 } },
        { id: 'n9', tipo: 'fim', mensagem: 'Obrigado, {email}!', posicao: { x: -200, y: 660 } },
      ],
    },
  });
  await api('POST', `/automacoes/${chatbot.id}/ativar`);
  ids.chatbot = chatbot.id;

  const ia = await api('POST', '/automacoes/ia', {
    nome: 'Responder com histórico',
    modelo: 'responder_historico',
    descricao: 'Rascunho de resposta com IA (desligada).',
  });
  ids.ia = ia.id;

  // execução ok: lead pergunta preço
  await relogio(em('04', '15:10'));
  await api('POST', `/falso/contas/${conta.id}/mensagem-recebida`, {
    de: '+5511900000180',
    nome: 'Bruno Tavares',
    texto: 'Boa tarde! Qual o preço do kit com 40 peças?',
    em: em('04', '15:10'),
  });
  const execOk = await ate(async () => {
    const e = (await api('GET', `/automacoes/${fluxo.id}/execucoes?limite=10`)).itens;
    return e.find((x) => x.estado !== 'rodando' && x.estado !== 'na_fila');
  });
  ids.execucaoOk = execOk.id;
  // execução com erro: automação de IA (em branco) cujo código lança erro; executada uma vez e
  // desligada de novo (a lista mostra a IA "inativa").
  await relogio(em('04', '15:20'));
  const classificar = await api('POST', '/automacoes/ia', { nome: 'Classificar lead', modelo: 'em_branco' });
  const manifesto = JSON.parse((await api('GET', `/automacoes/${classificar.id}/arquivos/automacao.json`)).conteudo);
  manifesto.gatilhos = [{ tipo: 'manual' }];
  await api('PUT', `/automacoes/${classificar.id}/arquivos/automacao.json`, {
    conteudo: JSON.stringify(manifesto, null, 2),
  });
  await api('PUT', `/automacoes/${classificar.id}/arquivos/index.ts`, {
    conteudo:
      "import { definirAutomacao } from '@zapdesk/automacao';\n\n" +
      'export default definirAutomacao({\n' +
      '  async aoExecutar(ctx, entrada) {\n' +
      "    ctx.log.info('classificando', entrada);\n" +
      "    throw new Error('Planilha de preços não encontrada.');\n" +
      '  },\n});\n',
  });
  await api('POST', `/automacoes/${classificar.id}/ativar`);
  await api('POST', `/automacoes/${classificar.id}/executar`, { entrada: { lead: 'Bruno Tavares' } });
  const execErro = await ate(async () => {
    const e = (await api('GET', `/automacoes/${classificar.id}/execucoes?limite=10`)).itens;
    return e.find((x) => x.estado !== 'rodando' && x.estado !== 'na_fila');
  }, 30000);
  await api('POST', `/automacoes/${classificar.id}/desativar`);
  ids.classificar = classificar.id;
  ids.execucaoErro = execErro.id;
  log(`execuções: ${execOk.estado}, ${execErro.estado}`);

  // --------------------------------------------------------------- conversas de ontem e de hoje
  const MARINA = '+5511900000142';
  const historico = [
    {
      jid: '+5511900000171',
      nome: 'Clínica Bem Viver',
      mensagens: [
        { texto: 'Bom dia! Vocês fazem uniforme bordado?', de: '+5511900000171', em: em('04', '16:02') },
        { texto: 'Fazemos sim! Te mando as opções.', de_mim: true, em: em('04', '16:20'), wa_id: 'SEMENTE-CBV-1' },
        { texto: 'Obrigada pelo retorno!', de: '+5511900000171', em: em('04', '17:45') },
      ],
    },
    {
      jid: '+5511900000168',
      nome: 'Rafael Nunes',
      mensagens: [
        { texto: 'Rafael, chegou o lote 32?', de_mim: true, em: em('04', '18:10'), wa_id: 'SEMENTE-RN-1' },
        { tipo: 'audio', de: '+5511900000168', em: em('04', '18:30') },
      ],
    },
    {
      jid: '120363000000000001@g.us',
      nome: 'Grupo — Fornecedores',
      mensagens: [
        { texto: 'Bom dia, pessoal!', de: '+5511900000168', nome: 'Rafael Nunes', em: em('05', '08:01') },
        { texto: 'Bom dia! A entrega de hoje sai às 11h.', de: '+5511900000155', nome: 'Ateliê Fio de Ouro', em: em('05', '08:15') },
      ],
    },
    {
      jid: '+5511900000155',
      nome: 'Ateliê Fio de Ouro',
      mensagens: [
        { texto: 'Bom dia! Consigo retirar o pedido na sexta?', de: '+5511900000155', em: em('05', '09:40') },
        { texto: 'Consegue sim, deixo separado no balcão.', de_mim: true, em: em('05', '10:02'), wa_id: 'SEMENTE-AFO-1' },
        { texto: 'Combinado, sexta eu passo aí.', de: '+5511900000155', em: em('05', '10:15') },
      ],
    },
    {
      jid: MARINA,
      nome: 'Marina Couto',
      mensagens: [
        { texto: 'Oi, tudo bem? Vocês têm pronta entrega?', de: MARINA, em: em('04', '17:05'), wa_id: 'SEMENTE-MC-0' },
        { texto: 'Temos sim! Amanhã te mando o catálogo.', de_mim: true, em: em('04', '17:12'), wa_id: 'SEMENTE-MC-00' },
        { texto: 'Oi! Vi o catálogo de vocês. Fazem entrega em Campinas?', de: MARINA, em: em('05', '10:31'), wa_id: 'SEMENTE-MC-1' },
        {
          texto: 'Oi, Marina! Fazemos sim, com frete fixo. Quantas peças você precisa?',
          de_mim: true,
          em: em('05', '10:33'),
          wa_id: 'SEMENTE-MC-2',
        },
        { tipo: 'audio', de: MARINA, em: em('05', '10:36'), wa_id: 'SEMENTE-MC-3' },
        { tipo: 'documento', de_mim: true, texto: 'orcamento-out.pdf', em: em('05', '10:39'), wa_id: 'SEMENTE-MC-4' },
      ],
    },
  ];
  // Uma conversa por chamada, com o relógio andando 1 s: a ordem de criação (desempate de listas)
  // fica fixa em vez de depender dos ids aleatórios.
  for (const [i, conversa] of historico.entries()) {
    await relogio(`2026-10-05T10:40:0${i}-03:00`);
    await api('POST', `/falso/contas/${conta.id}/historico`, { conversas: [conversa] });
  }
  for (const wa of ['SEMENTE-CBV-1', 'SEMENTE-RN-1', 'SEMENTE-AFO-1', 'SEMENTE-MC-00', 'SEMENTE-MC-2']) {
    await api('POST', `/falso/contas/${conta.id}/recibo`, { wa_id: wa, tipo: 'lido' }, { aceitar: [404, 409] });
  }
  await api('POST', `/falso/contas/${conta.id}/recibo`, { wa_id: 'SEMENTE-MC-4', tipo: 'entregue' }, { aceitar: [404, 409] });

  // não lidas (mensagens novas chegando agora)
  for (const [texto, autor, de, hora] of [
    ['Chegou a nota do lote 31?', 'Rafael Nunes', '+5511900000168', '09:50'],
    ['Chegou sim, conferindo agora.', 'Ateliê Fio de Ouro', '+5511900000155', '09:52'],
    ['Alguém confirma a coleta das 14h?', 'Rafael Nunes', '+5511900000168', '09:55'],
    ['Confirmo.', 'Ateliê Fio de Ouro', '+5511900000155', '09:57'],
    ['Rafael: chegou a nota do lote 32', 'Rafael Nunes', '+5511900000168', '09:58'],
  ]) {
    await relogio(em('05', hora));
    await api('POST', `/falso/contas/${conta.id}/mensagem-recebida`, {
      de,
      nome: autor,
      texto: texto.replace(/^Rafael: /, ''),
      grupo_jid: '120363000000000001@g.us',
      grupo_nome: 'Grupo — Fornecedores',
      em: em('05', hora),
    });
  }
  await relogio(em('05', '10:41'));
  await api('POST', `/falso/contas/${conta.id}/mensagem-recebida`, {
    de: MARINA,
    nome: 'Marina Couto',
    texto: 'Recebi, obrigada!',
    citar_wa_id: 'SEMENTE-MC-4',
    em: em('05', '10:41'),
  });
  await relogio(em('05', '10:42'));
  await api('POST', `/falso/contas/${conta.id}/mensagem-recebida`, {
    de: MARINA,
    nome: 'Marina Couto',
    texto: 'Pode me mandar o orçamento com 40 peças?',
    em: em('05', '10:42'),
  });

  const conversas = (await api('GET', `/contas/${conta.id}/conversas?limite=100`)).itens;
  const marina = conversas.find((c) => c.telefone === MARINA);
  ids.conversaMarina = marina.id;
  ids.conversaGrupo = conversas.find((c) => c.tipo !== 'individual')?.id ?? null;

  // etiquetas e notas de contatos
  const contatos = (await api('GET', `/contas/${conta.id}/contatos?limite=100`)).itens;
  const contatoDe = (tel) => contatos.find((c) => c.telefone === tel);
  await api('PUT', `/contatos/${marina.contato_id}/etiquetas`, { etiqueta_ids: [etq['Quente'], etq['Orçamento']] });
  await api('PATCH', `/contatos/${marina.contato_id}`, { notas: 'Prefere contato à tarde.' });
  const clinica = contatoDe('+5511900000171');
  if (clinica) await api('PUT', `/contatos/${clinica.id}/etiquetas`, { etiqueta_ids: [etq['Cliente']] });
  const atelie = contatoDe('+5511900000155');
  if (atelie) await api('PUT', `/contatos/${atelie.id}/etiquetas`, { etiqueta_ids: [etq['Morno']] });
  ids.contatoMarina = marina.contato_id;

  // --------------------------------------------------------------- status (texto)
  await relogio(em('05', '08:30'));
  await api('POST', `/falso/contas/${conta.id}/status`, {
    de: '+5511900000155',
    texto: 'Coleção nova chegando sexta! 🧵',
  });
  await relogio(em('05', '09:15'));
  await api('POST', `/falso/contas/${conta.id}/status`, {
    de: '+5511900000171',
    texto: 'Agenda de outubro aberta.',
  });

  // relógio final: "agora" das capturas
  await relogio(AGORA);
  log('semente pronta');
  return ids;
}

// --------------------------------------------------------------- linha de comando
const ehCli = process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1];
if (ehCli) {
  const i = process.argv.indexOf('--runtime');
  if (i < 0 || !process.argv[i + 1]) {
    console.error('Uso: node app/scripts/semente-falsa.mjs --runtime <pasta>/runtime.json');
    process.exit(2);
  }
  const caminho = process.argv[i + 1];
  const rt = JSON.parse(readFileSync(caminho, 'utf8'));
  const ids = await semear({
    base: `http://127.0.0.1:${rt.porta}`,
    token: rt.token,
    log: (m) => console.log(`· ${m}`),
  });
  const saida = join(dirname(caminho), 'semente.json');
  writeFileSync(saida, JSON.stringify(ids, null, 2));
  console.log(`ids gravados em ${saida}`);
}
