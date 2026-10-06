// Busca de mensagens da conta (T061): resultados com conversa, data e trecho; clicar abre a
// conversa já posicionada na mensagem.
import { useInfiniteQuery } from '@tanstack/react-query';
import { ArrowLeft, Search } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router';

import type { Id } from '@zapdesk/cliente-motor';

import { chaves } from '../api/chaves';
import { useCliente } from '../api/motor';
import { BotaoIcone } from '../componentes/BotaoIcone';
import { EsqueletoLista } from '../componentes/Esqueleto';
import { EstadoVazio } from '../componentes/EstadoVazio';
import { TelaErro } from '../componentes/TelaErro';
import { rotuloDataLista } from '../util/formatar';
import { useAtraso } from './Conversas';

/** Destaca o termo no trecho (sem HTML vindo do motor). */
function Trecho({ texto, termo }: { texto: string; termo: string }) {
  if (!termo) return <>{texto}</>;
  const normalizar = (s: string) => s.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();
  const base = normalizar(texto);
  const alvo = normalizar(termo);
  const partes: { t: string; marcado: boolean }[] = [];
  let i = 0;
  while (alvo && i < texto.length) {
    const j = base.indexOf(alvo, i);
    if (j === -1) break;
    if (j > i) partes.push({ t: texto.slice(i, j), marcado: false });
    partes.push({ t: texto.slice(j, j + alvo.length), marcado: true });
    i = j + alvo.length;
  }
  if (i < texto.length) partes.push({ t: texto.slice(i), marcado: false });
  return (
    <>
      {partes.map((p, k) => (p.marcado ? <mark key={k}>{p.t}</mark> : <span key={k}>{p.t}</span>))}
    </>
  );
}

export function BuscaMensagens(props: { contaId: Id; aoFechar: () => void }) {
  const cliente = useCliente();
  const navegar = useNavigate();
  const [termo, setTermo] = useState('');
  const q = useAtraso(termo.trim(), 300);
  const campo = useRef<HTMLInputElement>(null);
  useEffect(() => campo.current?.focus(), []);

  const consulta = useInfiniteQuery({
    queryKey: chaves.busca(props.contaId, q),
    enabled: q.length >= 2,
    queryFn: ({ pageParam }) => cliente.buscarMensagens(props.contaId, q, { limite: 30, ...(pageParam ? { cursor: pageParam } : {}) }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (p) => p.proximo_cursor ?? undefined,
  });
  const resultados = consulta.data?.pages.flatMap((p) => p.itens) ?? [];

  return (
    <div className="busca-mensagens">
      <div className="busca-lista">
        <BotaoIcone rotulo="Voltar para as conversas" onClick={props.aoFechar}>
          <ArrowLeft size={18} />
        </BotaoIcone>
        <label className="campo-busca">
          <Search size={16} aria-hidden="true" />
          <input
            ref={campo}
            type="search"
            aria-label="Buscar mensagens"
            placeholder="Buscar mensagens"
            value={termo}
            onChange={(e) => setTermo(e.target.value)}
            onKeyDown={(e) => e.key === 'Escape' && props.aoFechar()}
          />
        </label>
      </div>
      {q.length < 2 ? (
        <p className="dica-busca">Digite ao menos 2 letras. A busca ignora acentos.</p>
      ) : consulta.isPending ? (
        <EsqueletoLista linhas={5} />
      ) : consulta.isError ? (
        <TelaErro erro={consulta.error} aoTentar={() => void consulta.refetch()} />
      ) : resultados.length === 0 ? (
        <EstadoVazio titulo="Nenhuma mensagem encontrada" />
      ) : (
        <ul className="resultados-busca" aria-label="Resultados da busca">
          {resultados.map((r) => (
            <li key={r.mensagem.id}>
              <button
                type="button"
                className="resultado-busca"
                onClick={() => navegar(`/conversas/${r.conversa.id}?mensagem=${encodeURIComponent(r.mensagem.id)}`)}
              >
                <span className="item-conversa-linha">
                  <strong>{r.conversa.nome}</strong>
                  <small>{rotuloDataLista(r.mensagem.enviada_em)}</small>
                </span>
                <span className="resultado-trecho">
                  {r.mensagem.de_mim ? 'Você: ' : ''}
                  <Trecho texto={r.trecho} termo={q} />
                </span>
              </button>
            </li>
          ))}
          {consulta.hasNextPage ? (
            <li>
              <button type="button" className="botao secundario largo" onClick={() => void consulta.fetchNextPage()}>
                Mais resultados
              </button>
            </li>
          ) : null}
        </ul>
      )}
    </div>
  );
}
