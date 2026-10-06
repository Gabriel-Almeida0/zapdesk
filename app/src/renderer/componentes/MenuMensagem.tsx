// Menu da mensagem (T115): responder, reagir (emojis rápidos), editar (se `pode_editar`),
// apagar para todos (se `pode_apagar`; fora do prazo aparece indisponível).
import { ChevronDown, Pencil, Reply, Trash } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';

import type { Mensagem } from '@zapdesk/cliente-motor';

export const EMOJIS_RAPIDOS = ['👍', '❤️', '😂', '😮', '😢', '🙏'] as const;

export interface AcoesMensagem {
  aoResponder: (m: Mensagem) => void;
  aoReagir: (m: Mensagem, emoji: string) => void;
  aoEditar: (m: Mensagem) => void;
  aoApagar: (m: Mensagem) => void;
}

/** Regras de disponibilidade vindas do motor (prazos do WhatsApp em constantes do motor). */
export function acoesDisponiveis(m: Mensagem) {
  const ativa = !m.apagada && m.tipo !== 'sistema' && m.estado !== 'pendente' && m.estado !== 'falhou';
  return {
    responder: ativa,
    reagir: ativa,
    editar: ativa && m.de_mim && m.tipo === 'texto' && m.pode_editar,
    apagar: ativa && m.de_mim && m.pode_apagar,
    /** Mostrar a ação desabilitada ("Fora do prazo") quando é minha mas o prazo passou. */
    editarForaDoPrazo: ativa && m.de_mim && m.tipo === 'texto' && !m.pode_editar,
    apagarForaDoPrazo: ativa && m.de_mim && !m.pode_apagar,
  };
}

/** Altura aproximada do menu aberto (emojis + 3 ações). */
const ALTURA_MENU = 190;

/** Ancestral que rola (a lista de mensagens); o menu não pode passar da borda dele. */
function areaVisivel(el: HTMLElement): DOMRect | { top: number; bottom: number } {
  for (let atual = el.parentElement; atual; atual = atual.parentElement) {
    const overflow = getComputedStyle(atual).overflowY;
    if (overflow === 'auto' || overflow === 'scroll' || overflow === 'hidden') return atual.getBoundingClientRect();
  }
  return { top: 0, bottom: window.innerHeight };
}

/** Abre para cima quando não cabe abaixo do gatilho (mensagens no fim da conversa). */
export function abrirParaCima(gatilho: { top: number; bottom: number }, area: { top: number; bottom: number }): boolean {
  const abaixo = area.bottom - gatilho.bottom;
  const acima = gatilho.top - area.top;
  return abaixo < ALTURA_MENU && acima > abaixo;
}

export function MenuMensagem({ mensagem, ...acoes }: { mensagem: Mensagem } & AcoesMensagem) {
  const [aberto, setAberto] = useState(false);
  const [paraCima, setParaCima] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const disp = acoesDisponiveis(mensagem);

  useEffect(() => {
    if (!aberto) return;
    const fora = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setAberto(false);
    };
    const esc = (e: KeyboardEvent) => e.key === 'Escape' && setAberto(false);
    document.addEventListener('mousedown', fora);
    document.addEventListener('keydown', esc);
    return () => {
      document.removeEventListener('mousedown', fora);
      document.removeEventListener('keydown', esc);
    };
  }, [aberto]);

  if (!disp.responder) return null;
  const minhaReacao = mensagem.reacoes.find((r) => r.de_mim)?.emoji;
  const fazer = (fn: () => void) => () => {
    setAberto(false);
    fn();
  };

  return (
    <div className={`menu-mensagem${aberto ? ' aberto' : ''}`} ref={ref}>
      <button
        type="button"
        className="menu-mensagem-gatilho"
        aria-label="Ações da mensagem"
        aria-haspopup="menu"
        aria-expanded={aberto}
        onClick={(e) => {
          if (!aberto) setParaCima(abrirParaCima(e.currentTarget.getBoundingClientRect(), areaVisivel(e.currentTarget)));
          setAberto((a) => !a);
        }}
      >
        <ChevronDown size={18} />
      </button>
      {aberto ? (
        <div className={`menu-flutuante menu-mensagem-lista${mensagem.de_mim ? ' direita' : ''}${paraCima ? ' acima' : ''}`} role="menu">
          <div className="reacoes-rapidas" role="group" aria-label="Reagir">
            {EMOJIS_RAPIDOS.map((emoji) => (
              <button
                key={emoji}
                type="button"
                role="menuitem"
                className={`reacao-rapida${minhaReacao === emoji ? ' escolhida' : ''}`}
                aria-label={minhaReacao === emoji ? `Remover reação ${emoji}` : `Reagir com ${emoji}`}
                onClick={fazer(() => acoes.aoReagir(mensagem, minhaReacao === emoji ? '' : emoji))}
              >
                {emoji}
              </button>
            ))}
          </div>
          <button type="button" role="menuitem" className="menu-item" onClick={fazer(() => acoes.aoResponder(mensagem))}>
            <Reply size={16} aria-hidden="true" /> Responder
          </button>
          {disp.editar || disp.editarForaDoPrazo ? (
            <button
              type="button"
              role="menuitem"
              className="menu-item"
              disabled={!disp.editar}
              title={disp.editar ? undefined : 'Fora do prazo para editar'}
              onClick={fazer(() => acoes.aoEditar(mensagem))}
            >
              <Pencil size={16} aria-hidden="true" /> Editar
            </button>
          ) : null}
          {disp.apagar || disp.apagarForaDoPrazo ? (
            <button
              type="button"
              role="menuitem"
              className="menu-item perigo"
              disabled={!disp.apagar}
              title={disp.apagar ? undefined : 'Fora do prazo para apagar'}
              onClick={fazer(() => acoes.aoApagar(mensagem))}
            >
              <Trash size={16} aria-hidden="true" /> Apagar para todos
            </button>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
