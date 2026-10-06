// Grafo do chatbot (puro, testável): saídas de cada nó, ligar/desligar saídas, criar/remover nós,
// posições automáticas e a definição inicial de um bot novo.
import type { DefinicaoChatbot, NoChatbot, PosicaoNo, TipoNoChatbot } from '@zapdesk/cliente-motor';

import { novoId } from '../../util/automacoes';

export const ROTULO_NO: Record<TipoNoChatbot, string> = {
  inicio: 'Início',
  mensagem: 'Enviar mensagem',
  menu: 'Menu de opções',
  pergunta: 'Pergunta',
  condicao: 'Condição',
  acao: 'Ação',
  ia: 'Automação de IA',
  humano: 'Transferir para humano',
  fim: 'Fim',
};

export interface Saida {
  /** Id do handle no canvas: "proximo", "opcao:0", "ao_esgotar", "ramo:1", "senao", "em_erro". */
  id: string;
  rotulo: string;
  destino: string;
  /** Saída opcional (vazia = comportamento padrão, ex.: transferir para humano). */
  opcional: boolean;
}

export function saidas(no: NoChatbot): Saida[] {
  switch (no.tipo) {
    case 'inicio':
    case 'mensagem':
    case 'acao':
      return [{ id: 'proximo', rotulo: '', destino: no.proximo, opcional: false }];
    case 'menu':
      return [
        ...no.opcoes.map((o, i) => ({ id: `opcao:${i}`, rotulo: o.rotulo || `Opção ${i + 1}`, destino: o.proximo, opcional: false })),
        { id: 'ao_esgotar', rotulo: 'Ao esgotar', destino: no.ao_esgotar ?? '', opcional: true },
      ];
    case 'pergunta':
      return [
        { id: 'proximo', rotulo: 'Respondeu', destino: no.proximo, opcional: false },
        { id: 'ao_esgotar', rotulo: 'Ao esgotar', destino: no.ao_esgotar ?? '', opcional: true },
      ];
    case 'condicao':
      return [
        ...no.ramos.map((r, i) => ({ id: `ramo:${i}`, rotulo: `Ramo ${i + 1}`, destino: r.proximo, opcional: false })),
        { id: 'senao', rotulo: 'Senão', destino: no.senao, opcional: false },
      ];
    case 'ia':
      return [
        { id: 'proximo', rotulo: 'OK', destino: no.proximo, opcional: false },
        { id: 'em_erro', rotulo: 'Em erro', destino: no.em_erro ?? '', opcional: true },
      ];
    case 'humano':
    case 'fim':
      return [];
  }
}

/** Liga (ou desliga, com destino "") uma saída do nó. */
export function definirSaida(no: NoChatbot, saida: string, destino: string): NoChatbot {
  const [nome, indiceTexto] = saida.split(':');
  const indice = Number(indiceTexto);
  switch (no.tipo) {
    case 'inicio':
    case 'mensagem':
    case 'acao':
      return nome === 'proximo' ? { ...no, proximo: destino } : no;
    case 'menu':
      if (nome === 'opcao') return { ...no, opcoes: no.opcoes.map((o, i) => (i === indice ? { ...o, proximo: destino } : o)) };
      if (nome === 'ao_esgotar') return { ...no, ao_esgotar: destino || null };
      return no;
    case 'pergunta':
      if (nome === 'proximo') return { ...no, proximo: destino };
      if (nome === 'ao_esgotar') return { ...no, ao_esgotar: destino || null };
      return no;
    case 'condicao':
      if (nome === 'ramo') return { ...no, ramos: no.ramos.map((r, i) => (i === indice ? { ...r, proximo: destino } : r)) };
      if (nome === 'senao') return { ...no, senao: destino };
      return no;
    case 'ia':
      if (nome === 'proximo') return { ...no, proximo: destino };
      if (nome === 'em_erro') return { ...no, em_erro: destino || null };
      return no;
    default:
      return no;
  }
}

/** Remove nós (nunca o início) e desliga as saídas que apontavam para eles. */
export function removerNos(def: DefinicaoChatbot, ids: readonly string[]): DefinicaoChatbot {
  const remover = new Set(ids.filter((id) => id !== def.inicio));
  const nos = def.nos
    .filter((n) => !remover.has(n.id))
    .map((n) => saidas(n).reduce((acc, s) => (remover.has(s.destino) ? definirSaida(acc, s.id, '') : acc), n));
  return { ...def, nos };
}

export function novoNo(tipo: TipoNoChatbot, existentes: readonly string[], posicao: PosicaoNo): NoChatbot {
  const id = novoId(tipo, existentes);
  switch (tipo) {
    case 'inicio':
      return { id, tipo, proximo: '', posicao };
    case 'mensagem':
      return { id, tipo, texto: '', proximo: '', posicao };
    case 'menu':
      return {
        id,
        tipo,
        texto: 'Escolha uma opção:',
        opcoes: [
          { rotulo: 'Opção 1', valores: [], proximo: '' },
          { rotulo: 'Opção 2', valores: [], proximo: '' },
        ],
        mostrar_numeros: true,
        ao_esgotar: null,
        posicao,
      };
    case 'pergunta':
      return { id, tipo, texto: '', variavel: id.toLowerCase().replace(/[^a-z0-9_]/g, '_'), validacao: { tipo: 'nenhuma' }, proximo: '', ao_esgotar: null, posicao };
    case 'condicao':
      return { id, tipo, ramos: [{ condicoes: { modo: 'todas', regras: [] }, proximo: '' }], senao: '', posicao };
    case 'acao':
      return { id, tipo, acao: { id: `${id}_acao`, tipo: 'adicionar_etiqueta', etiqueta_id: '' }, proximo: '', posicao };
    case 'ia':
      return { id, tipo, automacao_id: '', modo: 'responder', variavel: null, proximo: '', em_erro: null, posicao };
    case 'humano':
      return { id, tipo, mensagem: 'Vou te passar para um atendente.', posicao };
    case 'fim':
      return { id, tipo, mensagem: null, posicao };
  }
}

const LARGURA_NIVEL = 300;
const ALTURA_LINHA = 170;

/** Posições para nós sem `posicao` (níveis pela distância ao início). */
export function posicoesAutomaticas(def: DefinicaoChatbot): Map<string, PosicaoNo> {
  const porId = new Map(def.nos.map((n) => [n.id, n]));
  const nivel = new Map<string, number>();
  const fila: string[] = [def.inicio];
  nivel.set(def.inicio, 0);
  while (fila.length > 0) {
    const id = fila.shift() as string;
    const no = porId.get(id);
    if (!no) continue;
    for (const s of saidas(no)) {
      if (s.destino && porId.has(s.destino) && !nivel.has(s.destino)) {
        nivel.set(s.destino, (nivel.get(id) ?? 0) + 1);
        fila.push(s.destino);
      }
    }
  }
  const linhas = new Map<number, number>();
  const resultado = new Map<string, PosicaoNo>();
  for (const n of def.nos) {
    if (n.posicao) {
      resultado.set(n.id, n.posicao);
      continue;
    }
    const nv = nivel.get(n.id) ?? Math.max(0, ...nivel.values()) + 1;
    const linha = linhas.get(nv) ?? 0;
    linhas.set(nv, linha + 1);
    resultado.set(n.id, { x: nv * LARGURA_NIVEL, y: linha * ALTURA_LINHA });
  }
  return resultado;
}

/** Bot novo: boas-vindas → menu (preços / falar com vendedor) → e-mail validado → humano. */
export function definicaoPadrao(): DefinicaoChatbot {
  return {
    versao: 1,
    inicio: 'inicio',
    nao_entendi: 'Desculpe, não entendi. Pode responder com o número da opção?',
    max_tentativas: 3,
    inatividade_min: 30,
    nos: [
      { id: 'inicio', tipo: 'inicio', proximo: 'boas_vindas', posicao: { x: 0, y: 120 } },
      { id: 'boas_vindas', tipo: 'mensagem', texto: 'Olá, {primeiro_nome}! Aqui é o atendimento automático.', proximo: 'menu1', posicao: { x: 260, y: 100 } },
      {
        id: 'menu1',
        tipo: 'menu',
        texto: 'Como posso ajudar?',
        opcoes: [
          { rotulo: 'Preços', valores: ['preco', 'valor'], proximo: 'pergunta_email' },
          { rotulo: 'Falar com vendedor', valores: ['vendedor', 'humano'], proximo: 'humano1' },
        ],
        mostrar_numeros: true,
        ao_esgotar: null,
        posicao: { x: 560, y: 80 },
      },
      {
        id: 'pergunta_email',
        tipo: 'pergunta',
        texto: 'Qual é o seu e-mail? Envio a tabela por lá.',
        variavel: 'email',
        validacao: { tipo: 'email', mensagem_erro: 'Esse e-mail parece inválido. Pode conferir?' },
        proximo: 'fim1',
        ao_esgotar: null,
        posicao: { x: 900, y: 0 },
      },
      { id: 'humano1', tipo: 'humano', mensagem: 'Certo! Um vendedor vai continuar o atendimento.', posicao: { x: 900, y: 240 } },
      { id: 'fim1', tipo: 'fim', mensagem: 'Obrigado! Enviamos a tabela para {email}.', posicao: { x: 1220, y: 0 } },
    ],
  };
}
