// Cartão do Kanban (T068): arrastável (HTML5), abre a conversa no clique e tem "Mover para…"
// acessível pelo teclado.
import { EllipsisVertical, Info } from 'lucide-react';
import { useEffect, useRef, useState, type DragEvent } from 'react';

import type { Card, Etapa } from '@zapdesk/cliente-motor';

import { BotaoIcone } from '../../componentes/BotaoIcone';
import { nomeExibicao, telefone } from '../../util/formatar';

export const TIPO_ARRASTE = 'application/x-zapdesk-card';

/** "agora", "há 5 min", "há 3 h", "há 2 dias". */
export function tempoDesde(iso: string, agora: Date = new Date()): string {
  const s = Math.max(0, (agora.getTime() - new Date(iso).getTime()) / 1000);
  if (s < 60) return 'agora';
  if (s < 3600) return `há ${Math.floor(s / 60)} min`;
  if (s < 86400) return `há ${Math.floor(s / 3600)} h`;
  const dias = Math.floor(s / 86400);
  return dias === 1 ? 'há 1 dia' : `há ${dias} dias`;
}

export interface PropsCartao {
  card: Card;
  etapas: Etapa[];
  aoAbrir: (card: Card) => void;
  aoDetalhes: (card: Card) => void;
  aoMover: (card: Card, etapaId: string) => void;
}

export function Cartao({ card, etapas, aoAbrir, aoDetalhes, aoMover }: PropsCartao) {
  // Posição fixa: a coluna rola (overflow) e cortaria um menu absoluto.
  const [menu, setMenu] = useState<{ top: number; left: number } | null>(null);
  // Só visual: marca a origem do arraste (o navegador já tirou a imagem do fantasma no dragstart).
  const [arrastando, setArrastando] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const nome = nomeExibicao(card.lead.nome, card.lead.telefone);

  useEffect(() => {
    if (!menu) return undefined;
    const fechar = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setMenu(null);
    };
    document.addEventListener('mousedown', fechar);
    return () => document.removeEventListener('mousedown', fechar);
  }, [menu]);

  const aoArrastar = (e: DragEvent<HTMLDivElement>) => {
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData(TIPO_ARRASTE, JSON.stringify({ lead_id: card.lead_id, etapa_id: card.etapa_id }));
    e.dataTransfer.setData('text/plain', nome);
    setArrastando(true);
  };

  return (
    <div
      className={`cartao-kanban${arrastando ? ' arrastando' : ''}`}
      ref={ref}
      draggable
      onDragStart={aoArrastar}
      onDragEnd={() => setArrastando(false)}
      data-lead={card.lead_id}
    >
      <button type="button" className="cartao-kanban-corpo" onClick={() => aoAbrir(card)} aria-label={`Abrir conversa com ${nome}`}>
        <strong>{nome}</strong>
        {card.lead.nome ? <small>{telefone(card.lead.telefone)}</small> : null}
        {card.etiquetas.length > 0 ? (
          <span className="lista-chips">
            {card.etiquetas.slice(0, 3).map((e) => (
              <span key={e.id} className="chip-etiqueta mini">
                <span className="ponto-etiqueta" style={{ background: e.cor }} aria-hidden="true" />
                {e.nome}
              </span>
            ))}
          </span>
        ) : null}
        <small className="cartao-kanban-tempo" title={`Nesta etapa desde ${new Date(card.desde).toLocaleString('pt-BR')}`}>
          {tempoDesde(card.desde)}
        </small>
      </button>
      <div className="cartao-kanban-acoes">
        <BotaoIcone rotulo={`Detalhes de ${nome}`} className="pequeno" onClick={() => aoDetalhes(card)}>
          <Info size={14} />
        </BotaoIcone>
        <BotaoIcone
          rotulo={`Mover ${nome} para…`}
          className="pequeno"
          aria-haspopup="menu"
          aria-expanded={Boolean(menu)}
          onClick={(e) => {
            const r = e.currentTarget.getBoundingClientRect();
            setMenu((m) => (m ? null : { top: r.bottom + 4, left: Math.max(8, r.right - 200) }));
          }}
        >
          <EllipsisVertical size={14} />
        </BotaoIcone>
      </div>
      {menu ? (
        <div className="menu-flutuante menu-mover" role="menu" aria-label="Mover para" style={{ top: menu.top, left: menu.left }}>
          {etapas.map((e) => (
            <button
              key={e.id}
              type="button"
              role="menuitem"
              className="menu-item"
              disabled={e.id === card.etapa_id}
              onClick={() => {
                setMenu(null);
                aoMover(card, e.id);
              }}
              onKeyDown={(ev) => {
                if (ev.key === 'Escape') setMenu(null);
              }}
            >
              <span className="ponto-etiqueta" style={{ background: e.cor }} aria-hidden="true" />
              {e.nome}
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}
