// Seletor de conta no topo da lista de conversas: estado de cada conta, troca e "Conectar conta".
import { Check, ChevronDown, Plus } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router';

import { ROTULO_ESTADO_CONTA, useContaAtual } from '../estado/conta';
import { telefone } from '../util/formatar';
import { Avatar } from './Avatar';

export function SeletorConta() {
  const { contas, conta, selecionar } = useContaAtual();
  const [aberto, setAberto] = useState(false);
  const navegar = useNavigate();
  const ref = useRef<HTMLDivElement>(null);

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

  if (!conta) return null;

  return (
    <div className="seletor-conta" ref={ref}>
      <button
        type="button"
        className="seletor-conta-botao"
        aria-haspopup="menu"
        aria-expanded={aberto}
        aria-label={`Conta: ${conta.nome} (${ROTULO_ESTADO_CONTA[conta.estado]}). Trocar conta`}
        onClick={() => setAberto((a) => !a)}
      >
        <Avatar nome={conta.nome} chave={conta.id} tamanho={32} />
        <span className="seletor-conta-textos">
          <strong>{conta.nome}</strong>
          <small className={`estado-conta estado-${conta.estado}`}>
            {conta.estado === 'conectada' && !conta.online ? 'Sem conexão' : ROTULO_ESTADO_CONTA[conta.estado]}
          </small>
        </span>
        <ChevronDown size={18} aria-hidden="true" />
      </button>
      {aberto ? (
        <div className="menu-flutuante seletor-conta-menu" role="menu">
          {contas.map((c) => (
            <button
              key={c.id}
              type="button"
              role="menuitemradio"
              aria-checked={c.id === conta.id}
              className="menu-item"
              onClick={() => {
                selecionar(c.id);
                setAberto(false);
                navegar('/conversas');
              }}
            >
              <Avatar nome={c.nome} chave={c.id} tamanho={28} />
              <span className="menu-item-textos">
                <span>{c.nome}</span>
                <small>
                  {c.telefone ? `${telefone(c.telefone)} · ` : ''}
                  {ROTULO_ESTADO_CONTA[c.estado]}
                </small>
              </span>
              {c.id === conta.id ? <Check size={16} aria-hidden="true" /> : null}
            </button>
          ))}
          <div className="menu-separador" />
          <button
            type="button"
            role="menuitem"
            className="menu-item"
            onClick={() => {
              setAberto(false);
              navegar('/contas/conectar');
            }}
          >
            <span className="menu-item-icone">
              <Plus size={18} />
            </span>
            <span>Conectar conta</span>
          </button>
        </div>
      ) : null}
    </div>
  );
}
