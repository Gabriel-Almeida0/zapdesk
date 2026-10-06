// Conversas (T058, T124): lista virtualizada da conta atual (busca por nome, "Não lidas", filtro por
// etiqueta, esqueleto, atualização por eventos) + painel do chat + busca de mensagens.
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { MessageCircle, MessageSquarePlus, Search, X } from 'lucide-react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router';

import type { Conversa, Id } from '@zapdesk/cliente-motor';

import { chaves } from '../api/chaves';
import { useCliente } from '../api/motor';
import { useAtalhos, vizinho } from '../atalhos';
import { Avatar } from '../componentes/Avatar';
import { BotaoIcone } from '../componentes/BotaoIcone';
import { EsqueletoLista } from '../componentes/Esqueleto';
import { EstadoVazio } from '../componentes/EstadoVazio';
import { FaixaConta } from '../componentes/FaixaConta';
import { ListaVirtual } from '../componentes/ListaVirtual';
import { NovaConversa } from '../componentes/NovaConversa';
import { SeletorConta } from '../componentes/SeletorConta';
import { TelaErro } from '../componentes/TelaErro';
import { useContaAtual } from '../estado/conta';
import { nomeExibicao, rotuloDataLista } from '../util/formatar';
import { BuscaMensagens } from './BuscaMensagens';
import { Chat } from './Chat';

export function useAtraso<T>(valor: T, ms = 250): T {
  const [atrasado, setAtrasado] = useState(valor);
  useEffect(() => {
    const t = setTimeout(() => setAtrasado(valor), ms);
    return () => clearTimeout(t);
  }, [valor, ms]);
  return atrasado;
}

function ItemConversa({ conversa, ativa, aoAbrir }: { conversa: Conversa; ativa: boolean; aoAbrir: () => void }) {
  return (
    <button
      type="button"
      className={`item-conversa${ativa ? ' ativa' : ''}${conversa.nao_lidas > 0 ? ' nao-lida' : ''}`}
      aria-current={ativa ? 'true' : undefined}
      onClick={aoAbrir}
    >
      <Avatar nome={conversa.nome} chave={conversa.jid} grupo={conversa.tipo === 'grupo'} />
      <span className="item-conversa-corpo">
        <span className="item-conversa-linha">
          <span className="item-conversa-nome">{nomeExibicao(conversa.nome, conversa.telefone)}</span>
          <span className="item-conversa-hora">{rotuloDataLista(conversa.ultima_mensagem_em)}</span>
        </span>
        <span className="item-conversa-linha">
          <span className="item-conversa-resumo">{conversa.ultima_mensagem_resumo ?? ''}</span>
          <span className="item-conversa-marcas">
            {conversa.etiquetas.slice(0, 3).map((e) => (
              <span key={e.id} className="ponto-etiqueta" style={{ background: e.cor }} title={e.nome} />
            ))}
            {conversa.nao_lidas > 0 ? (
              <span className="selo-nao-lidas" aria-label={`${conversa.nao_lidas} não lidas`}>
                {conversa.nao_lidas}
              </span>
            ) : null}
          </span>
        </span>
      </span>
    </button>
  );
}

function ListaConversas(props: { contaId: Id; conversaId?: Id; aoMudarOrdem: (ids: Id[]) => void }) {
  const cliente = useCliente();
  const navegar = useNavigate();
  const [busca, setBusca] = useState('');
  const [naoLidas, setNaoLidas] = useState(false);
  const [etiquetaId, setEtiquetaId] = useState('');
  const buscaAtrasada = useAtraso(busca.trim());

  const etiquetas = useQuery({ queryKey: chaves.etiquetas, queryFn: () => cliente.listarEtiquetas() });

  const consulta = useInfiniteQuery({
    queryKey: [...chaves.conversas(props.contaId), { busca: buscaAtrasada, naoLidas, etiquetaId }],
    queryFn: ({ pageParam }) =>
      cliente.listarConversas(props.contaId, {
        limite: 50,
        ...(pageParam ? { cursor: pageParam } : {}),
        ...(buscaAtrasada ? { busca: buscaAtrasada } : {}),
        ...(naoLidas ? { nao_lidas: true } : {}),
        ...(etiquetaId ? { etiqueta_id: etiquetaId } : {}),
      }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (ultima) => ultima.proximo_cursor ?? undefined,
  });

  const conversas = useMemo(() => consulta.data?.pages.flatMap((p) => p.itens) ?? [], [consulta.data]);
  const { aoMudarOrdem } = props;
  useEffect(() => aoMudarOrdem(conversas.map((c) => c.id)), [conversas, aoMudarOrdem]);

  const { hasNextPage, isFetchingNextPage, fetchNextPage } = consulta;
  const carregarMais = useCallback(() => {
    if (hasNextPage && !isFetchingNextPage) void fetchNextPage();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  const filtrando = Boolean(buscaAtrasada || naoLidas || etiquetaId);

  return (
    <>
      <div className="busca-lista">
        <label className="campo-busca">
          <Search size={16} aria-hidden="true" />
          <input
            type="search"
            aria-label="Pesquisar conversa pelo nome"
            placeholder="Pesquisar conversa"
            value={busca}
            onChange={(e) => setBusca(e.target.value)}
          />
        </label>
      </div>
      <div className="filtros" role="group" aria-label="Filtros">
        <button type="button" className={`chip${!naoLidas ? ' ativo' : ''}`} aria-pressed={!naoLidas} onClick={() => setNaoLidas(false)}>
          Todas
        </button>
        <button type="button" className={`chip${naoLidas ? ' ativo' : ''}`} aria-pressed={naoLidas} onClick={() => setNaoLidas(true)}>
          Não lidas
        </button>
        <select
          className={`chip chip-select${etiquetaId ? ' ativo' : ''}`}
          aria-label="Filtrar por etiqueta"
          value={etiquetaId}
          onChange={(e) => setEtiquetaId(e.target.value)}
        >
          <option value="">Etiqueta</option>
          {(etiquetas.data ?? []).map((e) => (
            <option key={e.id} value={e.id}>
              {e.nome}
            </option>
          ))}
        </select>
        {etiquetaId ? (
          <BotaoIcone rotulo="Limpar filtro de etiqueta" className="pequeno" onClick={() => setEtiquetaId('')}>
            <X size={14} />
          </BotaoIcone>
        ) : null}
      </div>
      {consulta.isPending ? (
        <EsqueletoLista />
      ) : consulta.isError ? (
        <TelaErro erro={consulta.error} aoTentar={() => void consulta.refetch()} />
      ) : conversas.length === 0 ? (
        <EstadoVazio
          titulo={filtrando ? 'Nenhuma conversa encontrada' : 'Nenhuma conversa ainda'}
          texto={filtrando ? 'Tente outro filtro.' : 'As conversas aparecem aqui assim que chegarem mensagens.'}
        />
      ) : (
        <ListaVirtual
          rotulo="Conversas"
          itens={conversas}
          estimativa={72}
          chave={(c) => c.id}
          aoChegarNoFim={carregarMais}
          indiceVisivel={props.conversaId ? conversas.findIndex((c) => c.id === props.conversaId) : undefined}
          renderizar={(c) => (
            <ItemConversa
              conversa={c}
              ativa={c.id === props.conversaId}
              aoAbrir={() => navegar(`/conversas/${c.id}`)}
            />
          )}
        />
      )}
    </>
  );
}

export function Conversas() {
  const { conversaId } = useParams();
  const navegar = useNavigate();
  const { conta, contas, carregando, erro, recarregar } = useContaAtual();
  const [modoBusca, setModoBusca] = useState(false);
  const [novaConversa, setNovaConversa] = useState(false);
  const ordem = useRef<Id[]>([]);
  const aoMudarOrdem = useCallback((ids: Id[]) => {
    ordem.current = ids;
  }, []);

  useAtalhos({
    buscar: () => setModoBusca(true),
    nova_conversa: () => setNovaConversa(true),
    conversa_anterior: () => {
      const id = vizinho(ordem.current, conversaId, -1);
      if (id) navegar(`/conversas/${id}`);
    },
    proxima_conversa: () => {
      const id = vizinho(ordem.current, conversaId, 1);
      if (id) navegar(`/conversas/${id}`);
    },
  });

  if (carregando) {
    return (
      <div className="conversas">
        <aside className="painel-lista">
          <EsqueletoLista />
        </aside>
        <main className="painel-chat vazio" />
      </div>
    );
  }
  if (erro) return <TelaErro erro={erro} aoTentar={recarregar} />;
  if (contas.length === 0 || !conta) {
    return (
      <EstadoVazio
        icone={<MessageCircle size={56} />}
        titulo="Nenhuma conta conectada"
        texto="Conecte um número de WhatsApp para ver suas conversas."
        acao={
          <Link to="/contas/conectar" className="botao">
            Conectar conta
          </Link>
        }
      />
    );
  }

  return (
    <div className="conversas">
      <aside className="painel-lista" aria-label="Lista de conversas">
        <header className="cabecalho-lista">
          <SeletorConta />
          <div className="acoes-cabecalho">
            <BotaoIcone rotulo="Nova conversa (⌘N)" onClick={() => setNovaConversa(true)}>
              <MessageSquarePlus size={20} />
            </BotaoIcone>
            <BotaoIcone rotulo="Buscar mensagens (⌘F)" ativo={modoBusca} onClick={() => setModoBusca((m) => !m)}>
              <Search size={20} />
            </BotaoIcone>
          </div>
        </header>
        <FaixaConta conta={conta} />
        {modoBusca ? (
          <BuscaMensagens contaId={conta.id} aoFechar={() => setModoBusca(false)} />
        ) : (
          <ListaConversas key={conta.id} contaId={conta.id} conversaId={conversaId} aoMudarOrdem={aoMudarOrdem} />
        )}
      </aside>
      <main className={`painel-chat${conversaId ? '' : ' vazio'}`}>
        {conversaId ? (
          <Chat key={conversaId} conversaId={conversaId} />
        ) : (
          <EstadoVazio
            icone={<MessageCircle size={64} />}
            titulo="ZapDesk"
            texto="Escolha uma conversa ou comece uma nova. Suas mensagens ficam só neste Mac."
          />
        )}
      </main>
      {novaConversa ? (
        <NovaConversa
          contaId={conta.id}
          aoFechar={() => setNovaConversa(false)}
          aoAbrir={(id) => {
            setNovaConversa(false);
            navegar(`/conversas/${id}`);
          }}
        />
      ) : null}
    </div>
  );
}
