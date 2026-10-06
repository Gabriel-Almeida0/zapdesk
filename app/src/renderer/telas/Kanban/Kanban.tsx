// Kanban do funil (T068): colunas por etapa, busca, arrastar e soltar, "Mover para…" pelo teclado,
// adicionar lead/contato à etapa e atualização ao vivo por `funil.movido` (via cache).
import { useMutation, useQueryClient, type InfiniteData, type QueryClient } from '@tanstack/react-query';
import { ArrowLeft, Search, Settings2 } from 'lucide-react';
import { useState } from 'react';
import { Link, useParams } from 'react-router';

import type { Card, Etapa, Funil, Id, Pagina } from '@zapdesk/cliente-motor';

import { useFunil } from '../../api/automacoes';
import { chaves } from '../../api/chaves';
import { useCliente } from '../../api/motor';
import { useAbrirConversa } from '../../componentes/abrirConversa';
import { Esqueleto } from '../../componentes/Esqueleto';
import { FaixaAviso } from '../../componentes/FaixaAviso';
import { TelaErro } from '../../componentes/TelaErro';
import { textoErro } from '../../util/formatar';
import { useAtraso } from '../Conversas';
import { AdicionarLead } from './AdicionarLead';
import { chaveCards, Coluna } from './Coluna';
import { EditarEtapas } from './EditarEtapas';
import { PainelCartao } from './PainelCartao';

type PaginasCards = InfiniteData<Pagina<Card>, string | undefined>;

/** Move o cartão no cache já (a resposta do motor e o evento confirmam depois). */
export function moverNoCache(qc: QueryClient, funilId: Id, leadId: Id, origem: Id, destino: Id, busca: string): void {
  let movido: Card | undefined;
  qc.setQueryData<PaginasCards>(chaveCards(funilId, origem, busca), (dados) => {
    if (!dados) return dados;
    return {
      ...dados,
      pages: dados.pages.map((p) => ({
        ...p,
        itens: p.itens.filter((c) => {
          if (c.lead_id !== leadId) return true;
          movido = c;
          return false;
        }),
      })),
    };
  });
  if (movido) {
    const novo: Card = { ...movido, etapa_id: destino, desde: new Date().toISOString() };
    qc.setQueryData<PaginasCards>(chaveCards(funilId, destino, busca), (dados) => {
      if (!dados || dados.pages.length === 0) return dados;
      const [primeira, ...resto] = dados.pages;
      if (!primeira) return dados;
      return { ...dados, pages: [{ ...primeira, itens: [novo, ...primeira.itens] }, ...resto] };
    });
  }
  qc.setQueryData<Funil>(chaves.funil(funilId), (f) =>
    f
      ? {
          ...f,
          etapas: f.etapas.map((e) =>
            e.id === origem ? { ...e, total_cards: Math.max(0, e.total_cards - 1) } : e.id === destino ? { ...e, total_cards: e.total_cards + 1 } : e,
          ),
        }
      : f,
  );
}

export function Kanban() {
  const { funilId = '' } = useParams();
  const cliente = useCliente();
  const qc = useQueryClient();
  const abrirConversa = useAbrirConversa();
  const funil = useFunil(funilId);
  const [busca, setBusca] = useState('');
  const buscaAtrasada = useAtraso(busca.trim(), 300);
  const [editandoEtapas, setEditandoEtapas] = useState(false);
  const [adicionando, setAdicionando] = useState<Etapa | null>(null);
  const [detalhe, setDetalhe] = useState<Card | null>(null);

  const mover = useMutation({
    mutationFn: (d: { leadId: Id; origem: Id; destino: Id }) =>
      cliente.moverCard(funilId, { lead_id: d.leadId, etapa_id: d.destino, origem: 'app' }),
    onMutate: (d) => moverNoCache(qc, funilId, d.leadId, d.origem, d.destino, buscaAtrasada),
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: chaves.cards(funilId) });
      void qc.invalidateQueries({ queryKey: chaves.funil(funilId) });
      void qc.invalidateQueries({ queryKey: chaves.historicoFunil(funilId) });
    },
  });

  if (funil.isError) return <TelaErro erro={funil.error} aoTentar={() => void funil.refetch()} />;
  const etapas = [...(funil.data?.etapas ?? [])].sort((a, b) => a.ordem - b.ordem);

  const abrir = (card: Card) => {
    if (card.conversa_id) void abrirConversa(card.conversa_id, card.conta_id);
    else setDetalhe(card);
  };

  return (
    <div className="kanban-com-painel">
      <section className="tela kanban">
        <header className="cabecalho-tela">
          <Link to="/funis" className="botao-icone" aria-label="Voltar para Funis" title="Voltar para Funis">
            <ArrowLeft size={20} />
          </Link>
          <h1>{funil.data ? funil.data.nome : <Esqueleto largura={180} altura={20} />}</h1>
          <label className="campo-busca busca-kanban">
            <Search size={16} aria-hidden="true" />
            <input type="search" aria-label="Buscar no funil" placeholder="Buscar por nome ou telefone" value={busca} onChange={(e) => setBusca(e.target.value)} />
          </label>
          <button type="button" className="botao secundario" onClick={() => setEditandoEtapas(true)} disabled={!funil.data}>
            <Settings2 size={16} aria-hidden="true" /> Etapas
          </button>
        </header>
        {mover.error ? <FaixaAviso tipo="erro">{textoErro(mover.error)}</FaixaAviso> : null}
        <div className="quadro-kanban">
          {funil.isPending ? (
            [1, 2, 3].map((i) => (
              <div key={i} className="coluna-kanban">
                <Esqueleto altura={36} />
                <Esqueleto altura={72} />
              </div>
            ))
          ) : etapas.length === 0 ? (
            <p className="texto-secundario">Este funil não tem etapas. Use "Etapas" para criar.</p>
          ) : (
            etapas.map((etapa) => (
              <Coluna
                key={etapa.id}
                funilId={funilId}
                etapa={etapa}
                etapas={etapas}
                busca={buscaAtrasada}
                aoSoltar={(leadId, origem, destino) => mover.mutate({ leadId, origem, destino })}
                aoMover={(card, destino) => mover.mutate({ leadId: card.lead_id, origem: card.etapa_id, destino })}
                aoAbrir={abrir}
                aoDetalhes={setDetalhe}
                aoAdicionar={setAdicionando}
              />
            ))
          )}
        </div>
      </section>
      {detalhe && funil.data ? (
        <PainelCartao key={detalhe.lead_id} funil={funil.data} card={detalhe} aoFechar={() => setDetalhe(null)} />
      ) : null}
      {editandoEtapas && funil.data ? <EditarEtapas funil={funil.data} aoFechar={() => setEditandoEtapas(false)} /> : null}
      {adicionando ? <AdicionarLead funilId={funilId} etapa={adicionando} aoFechar={() => setAdicionando(null)} /> : null}
    </div>
  );
}
