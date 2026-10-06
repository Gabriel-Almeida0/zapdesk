// Contatos (T062): lista paginada com busca e filtro por etiqueta; clicar abre a conversa.
import { useInfiniteQuery, useMutation, useQuery } from '@tanstack/react-query';
import { Contact, Search } from 'lucide-react';
import { useCallback, useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router';

import type { Contato } from '@zapdesk/cliente-motor';

import { chaves } from '../api/chaves';
import { useCliente } from '../api/motor';
import { Avatar } from '../componentes/Avatar';
import { EsqueletoLista } from '../componentes/Esqueleto';
import { EstadoVazio } from '../componentes/EstadoVazio';
import { FaixaAviso } from '../componentes/FaixaAviso';
import { ListaVirtual } from '../componentes/ListaVirtual';
import { ChipEtiqueta } from '../componentes/PainelContato';
import { SeletorConta } from '../componentes/SeletorConta';
import { TelaErro } from '../componentes/TelaErro';
import { useContaAtual } from '../estado/conta';
import { telefone, textoErro } from '../util/formatar';
import { useAtraso } from './Conversas';

export function nomeContato(c: Pick<Contato, 'nome' | 'nome_push' | 'telefone' | 'jid'>): string {
  return c.nome || c.nome_push || telefone(c.telefone) || c.jid;
}

export function Contatos() {
  const cliente = useCliente();
  const navegar = useNavigate();
  const { conta } = useContaAtual();
  const [busca, setBusca] = useState('');
  const [etiquetaId, setEtiquetaId] = useState('');
  const q = useAtraso(busca.trim());
  const etiquetas = useQuery({ queryKey: chaves.etiquetas, queryFn: () => cliente.listarEtiquetas() });

  const contaId = conta?.id ?? '';
  const consulta = useInfiniteQuery({
    queryKey: [...chaves.contatos(contaId), { q, etiquetaId }],
    enabled: Boolean(contaId),
    queryFn: ({ pageParam }) =>
      cliente.listarContatos(contaId, {
        limite: 100,
        ...(pageParam ? { cursor: pageParam } : {}),
        ...(q ? { busca: q } : {}),
        ...(etiquetaId ? { etiqueta_id: etiquetaId } : {}),
      }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (p) => p.proximo_cursor ?? undefined,
  });
  const contatos = useMemo(() => consulta.data?.pages.flatMap((p) => p.itens) ?? [], [consulta.data]);
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = consulta;
  const carregarMais = useCallback(() => {
    if (hasNextPage && !isFetchingNextPage) void fetchNextPage();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  const abrir = useMutation({
    mutationFn: async (c: Contato) => {
      if (c.conversa_id) return c.conversa_id;
      if (!c.telefone) throw new Error('Este contato não tem telefone.');
      return (await cliente.abrirConversa(c.conta_id, c.telefone)).id;
    },
    onSuccess: (id) => navegar(`/conversas/${id}`),
  });

  if (!conta) {
    return (
      <EstadoVazio
        icone={<Contact size={56} />}
        titulo="Nenhuma conta conectada"
        acao={
          <Link to="/contas/conectar" className="botao">
            Conectar conta
          </Link>
        }
      />
    );
  }

  return (
    <section className="tela tela-lista">
      <header className="cabecalho-tela">
        <h1>Contatos</h1>
        <SeletorConta />
      </header>
      <div className="barra-ferramentas">
        <label className="campo-busca">
          <Search size={16} aria-hidden="true" />
          <input type="search" aria-label="Buscar contato" placeholder="Buscar por nome ou telefone" value={busca} onChange={(e) => setBusca(e.target.value)} />
        </label>
        <select className="seletor" aria-label="Filtrar por etiqueta" value={etiquetaId} onChange={(e) => setEtiquetaId(e.target.value)}>
          <option value="">Todas as etiquetas</option>
          {(etiquetas.data ?? []).map((e) => (
            <option key={e.id} value={e.id}>
              {e.nome}
            </option>
          ))}
        </select>
      </div>
      {abrir.error ? <FaixaAviso tipo="erro">{textoErro(abrir.error)}</FaixaAviso> : null}
      {consulta.isPending ? (
        <EsqueletoLista />
      ) : consulta.isError ? (
        <TelaErro erro={consulta.error} aoTentar={() => void consulta.refetch()} />
      ) : contatos.length === 0 ? (
        <EstadoVazio
          icone={<Contact size={48} />}
          titulo={q || etiquetaId ? 'Nenhum contato encontrado' : 'Nenhum contato ainda'}
          texto={q || etiquetaId ? 'Tente outra busca.' : 'Os contatos aparecem depois da sincronização do WhatsApp.'}
        />
      ) : (
        <ListaVirtual
          rotulo="Contatos"
          itens={contatos}
          estimativa={64}
          chave={(c) => c.id}
          aoChegarNoFim={carregarMais}
          renderizar={(c) => (
            <button type="button" className="item-contato" disabled={abrir.isPending} onClick={() => abrir.mutate(c)}>
              <Avatar nome={nomeContato(c)} chave={c.jid} tamanho={44} />
              <span className="item-contato-textos">
                <strong>{nomeContato(c)}</strong>
                <small>{telefone(c.telefone)}</small>
              </span>
              <span className="lista-chips">
                {c.etiquetas.map((e) => (
                  <ChipEtiqueta key={e.id} etiqueta={e} />
                ))}
              </span>
            </button>
          )}
        />
      )}
    </section>
  );
}
