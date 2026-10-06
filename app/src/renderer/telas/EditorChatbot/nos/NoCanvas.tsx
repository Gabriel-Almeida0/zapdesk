// Nó do canvas (T114): um componente para os 9 tipos, com entrada à esquerda e uma saída por
// ramo à direita (opções do menu, ramos da condição, "ao esgotar", "em erro").
import { Handle, Position, type NodeProps } from '@xyflow/react';
import { Bot, CircleStop, Flag, GitFork, ListOrdered, MessageSquare, MessageSquareText, Play, UserRound, Zap } from 'lucide-react';
import type { ReactNode } from 'react';

import type { NoChatbot, TipoNoChatbot } from '@zapdesk/cliente-motor';

import { ROTULO_ACAO } from '../../../util/automacoes';
import { ROTULO_NO, saidas } from '../grafo';
import { useCanvasBot } from './contexto';

export const ICONE_NO: Record<TipoNoChatbot, ReactNode> = {
  inicio: <Play size={14} aria-hidden="true" />,
  mensagem: <MessageSquare size={14} aria-hidden="true" />,
  menu: <ListOrdered size={14} aria-hidden="true" />,
  pergunta: <MessageSquareText size={14} aria-hidden="true" />,
  condicao: <GitFork size={14} aria-hidden="true" />,
  acao: <Zap size={14} aria-hidden="true" />,
  ia: <Bot size={14} aria-hidden="true" />,
  humano: <UserRound size={14} aria-hidden="true" />,
  fim: <Flag size={14} aria-hidden="true" />,
};

/** Texto curto mostrado dentro do nó. */
export function resumoNo(no: NoChatbot): string {
  switch (no.tipo) {
    case 'inicio':
      return 'Começo da conversa';
    case 'mensagem':
      return no.texto || '(sem texto)';
    case 'menu':
      return no.texto || '(sem texto)';
    case 'pergunta':
      return `${no.texto || '(sem texto)'} → {${no.variavel}}`;
    case 'condicao':
      return `${no.ramos.length} ramo(s) + senão`;
    case 'acao':
      return ROTULO_ACAO[no.acao.tipo];
    case 'ia':
      return no.modo === 'responder' ? 'Responde com o retorno' : `Guarda em {${no.variavel ?? ''}}`;
    case 'humano':
      return no.mensagem || 'Transfere para você';
    case 'fim':
      return no.mensagem || 'Encerra a conversa';
  }
}

export function NoCanvas({ id, selected }: NodeProps) {
  const { nos, erros, noAtualSimulacao, inicio } = useCanvasBot();
  const no = nos.get(id);
  if (!no) return null;
  const errosNo = erros.get(id) ?? [];
  const lista = saidas(no);
  return (
    <div
      className={`no-bot tipo-${no.tipo}${selected ? ' selecionado' : ''}${errosNo.length > 0 ? ' com-erro' : ''}${noAtualSimulacao === id ? ' atual' : ''}`}
      title={errosNo.map((e) => e.mensagem).join('\n') || undefined}
      data-testid={`no-${id}`}
    >
      {id !== inicio ? <Handle type="target" position={Position.Left} id="entrada" /> : null}
      <div className="no-bot-cabecalho">
        {ICONE_NO[no.tipo]}
        <span>{ROTULO_NO[no.tipo]}</span>
        {errosNo.length > 0 ? <CircleStop size={13} className="no-bot-erro" aria-label={`${errosNo.length} erro(s)`} /> : null}
      </div>
      <div className="no-bot-corpo">{resumoNo(no)}</div>
      {lista.length > 0 ? (
        <div className="no-bot-saidas">
          {lista.map((s) => (
            <div key={s.id} className={`no-bot-saida${s.destino ? '' : ' livre'}${s.opcional ? ' opcional' : ''}`}>
              {s.rotulo ? <span>{s.rotulo}</span> : <span className="texto-secundario">próximo</span>}
              <Handle type="source" position={Position.Right} id={s.id} />
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}
