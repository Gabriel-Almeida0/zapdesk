// Trilho de navegação à esquerda (estilo WhatsApp Desktop). Todos os itens têm rótulo acessível.
import { useQuery } from '@tanstack/react-query';
import {
  CircleDashed,
  Contact,
  FileText,
  ListChecks,
  Megaphone,
  MessageCircle,
  Settings,
  SquareKanban,
  Tag,
  Workflow,
} from 'lucide-react';
import type { ReactNode } from 'react';
import { NavLink } from 'react-router';

import { chaves } from '../api/chaves';
import { useCliente } from '../api/motor';
import { useContaAtual } from '../estado/conta';
import { Avatar } from './Avatar';

interface Item {
  para: string;
  rotulo: string;
  icone: ReactNode;
  selo?: number;
}

function ItemNavegacao({ item }: { item: Item }) {
  return (
    <NavLink
      to={item.para}
      aria-label={item.rotulo}
      title={item.rotulo}
      className={({ isActive }) => `item-trilho${isActive ? ' ativo' : ''}`}
    >
      {item.icone}
      {item.selo ? <span className="selo-trilho" aria-label={`${item.selo} em andamento`}>{item.selo}</span> : null}
    </NavLink>
  );
}

export function BarraLateral() {
  const cliente = useCliente();
  const { conta } = useContaAtual();
  const sistema = useQuery({ queryKey: chaves.sistema, queryFn: () => cliente.sistema(), refetchInterval: 60_000 });
  const ativos = sistema.data?.disparos_ativos ?? 0;

  const principais: Item[] = [
    { para: '/conversas', rotulo: 'Conversas', icone: <MessageCircle size={22} /> },
    { para: '/status', rotulo: 'Status', icone: <CircleDashed size={22} /> },
    { para: '/contatos', rotulo: 'Contatos', icone: <Contact size={22} /> },
  ];
  const crm: Item[] = [
    { para: '/disparos', rotulo: 'Disparos', icone: <Megaphone size={22} />, selo: ativos },
    { para: '/leads', rotulo: 'Leads', icone: <ListChecks size={22} /> },
    { para: '/templates', rotulo: 'Templates', icone: <FileText size={22} /> },
    { para: '/etiquetas', rotulo: 'Etiquetas', icone: <Tag size={22} /> },
  ];
  // 002
  const automacoes: Item[] = [
    { para: '/funis', rotulo: 'Funis', icone: <SquareKanban size={22} /> },
    { para: '/automacoes', rotulo: 'Automações', icone: <Workflow size={22} /> },
  ];

  return (
    <nav className="trilho" aria-label="Navegação principal">
      <div className="trilho-grupo">
        {principais.map((i) => (
          <ItemNavegacao key={i.para} item={i} />
        ))}
        <span className="trilho-divisor" aria-hidden="true" />
        {crm.map((i) => (
          <ItemNavegacao key={i.para} item={i} />
        ))}
        <span className="trilho-divisor" aria-hidden="true" />
        {automacoes.map((i) => (
          <ItemNavegacao key={i.para} item={i} />
        ))}
      </div>
      <div className="trilho-grupo">
        <ItemNavegacao item={{ para: '/ajustes', rotulo: 'Ajustes', icone: <Settings size={22} /> }} />
        {conta ? (
          <NavLink to="/ajustes" className="trilho-conta" aria-label={`Conta atual: ${conta.nome}`} title={conta.nome}>
            <Avatar nome={conta.nome} chave={conta.id} tamanho={30} />
            <span className={`ponto-estado estado-${conta.estado}${conta.online ? '' : ' offline'}`} aria-hidden="true" />
          </NavLink>
        ) : null}
      </div>
    </nav>
  );
}
