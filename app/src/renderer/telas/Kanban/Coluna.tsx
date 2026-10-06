// Coluna do Kanban (T068): uma etapa, com cor, contagem, cartões virtualizados e paginados, e área
// de soltar (arrastar e soltar nativo).
import { useInfiniteQuery } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { useMemo, useState } from 'react';

import type { Card, Etapa, Id } from '@zapdesk/cliente-motor';

import { chaves } from '../../api/chaves';
import { useCliente } from '../../api/motor';
import { BotaoIcone } from '../../componentes/BotaoIcone';
import { Esqueleto } from '../../componentes/Esqueleto';
import { ListaVirtual } from '../../componentes/ListaVirtual';
import { numero, textoErro } from '../../util/formatar';
import { Cartao, TIPO_ARRASTE } from './Cartao';

export const TAMANHO_PAGINA_CARDS = 100;

export function chaveCards(funilId: Id, etapaId: Id, busca: string) {
  return [...chaves.cards(funilId), etapaId, busca] as const;
}

export interface PropsColuna {
  funilId: Id;
  etapa: Etapa;
  etapas: Etapa[];
  busca: string;
  aoSoltar: (leadId: Id, etapaOrigem: Id, etapaDestino: Id) => void;
  aoAbrir: (card: Card) => void;
  aoDetalhes: (card: Card) => void;
  aoMover: (card: Card, etapaId: Id) => void;
  aoAdicionar: (etapa: Etapa) => void;
}

export function Coluna(props: PropsColuna) {
  const { funilId, etapa, busca } = props;
  const cliente = useCliente();
  const [sobre, setSobre] = useState(false);
  const consulta = useInfiniteQuery({
    queryKey: chaveCards(funilId, etapa.id, busca),
    queryFn: ({ pageParam }) =>
      cliente.listarCards(funilId, {
        etapa_id: etapa.id,
        limite: TAMANHO_PAGINA_CARDS,
        ...(busca ? { busca } : {}),
        ...(pageParam ? { cursor: pageParam } : {}),
      }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (p) => p.proximo_cursor ?? undefined,
  });
  const cards = useMemo(() => consulta.data?.pages.flatMap((p) => p.itens) ?? [], [consulta.data]);
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = consulta;

  return (
    <section
      className={`coluna-kanban${sobre ? ' soltar' : ''}`}
      aria-label={`Etapa ${etapa.nome}`}
      onDragOver={(e) => {
        if (!e.dataTransfer.types.includes(TIPO_ARRASTE)) return;
        e.preventDefault();
        e.dataTransfer.dropEffect = 'move';
        setSobre(true);
      }}
      onDragLeave={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setSobre(false);
      }}
      onDrop={(e) => {
        setSobre(false);
        const bruto = e.dataTransfer.getData(TIPO_ARRASTE);
        if (!bruto) return;
        e.preventDefault();
        try {
          const dados = JSON.parse(bruto) as { lead_id: string; etapa_id: string };
          if (dados.etapa_id !== etapa.id) props.aoSoltar(dados.lead_id, dados.etapa_id, etapa.id);
        } catch {
          // arraste de outra origem
        }
      }}
    >
      <header className="coluna-kanban-cabecalho" style={{ borderTopColor: etapa.cor }}>
        <span className="ponto-etiqueta" style={{ background: etapa.cor }} aria-hidden="true" />
        <h2>{etapa.nome}</h2>
        <span className="contagem-coluna" aria-label={`${etapa.total_cards} leads`}>
          {numero(etapa.total_cards)}
        </span>
        <BotaoIcone rotulo={`Adicionar lead em ${etapa.nome}`} className="pequeno" onClick={() => props.aoAdicionar(etapa)}>
          <Plus size={14} />
        </BotaoIcone>
      </header>
      {consulta.isPending ? (
        <div className="coluna-kanban-lista" role="status" aria-label="Carregando">
          <Esqueleto altura={72} />
          <Esqueleto altura={72} />
        </div>
      ) : consulta.isError ? (
        <p className="erro-campo coluna-kanban-lista">{textoErro(consulta.error)}</p>
      ) : cards.length === 0 ? (
        <p className="coluna-kanban-vazia">{busca ? 'Nada encontrado.' : 'Arraste leads para cá.'}</p>
      ) : (
        <ListaVirtual
          className="coluna-kanban-lista"
          rotulo={`Leads em ${etapa.nome}`}
          itens={cards}
          estimativa={92}
          chave={(c) => c.lead_id}
          aoChegarNoFim={() => {
            if (hasNextPage && !isFetchingNextPage) void fetchNextPage();
          }}
          renderizar={(c) => (
            <Cartao card={c} etapas={props.etapas} aoAbrir={props.aoAbrir} aoDetalhes={props.aoDetalhes} aoMover={props.aoMover} />
          )}
        />
      )}
    </section>
  );
}
