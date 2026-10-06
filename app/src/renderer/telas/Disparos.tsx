// Lista de disparos (T094): estado ("Na fila" incluso), progresso e contadores; vazio
// "Nenhum disparo ainda — Novo disparo".
import { useInfiniteQuery } from '@tanstack/react-query';
import { Bot, Megaphone, Plus } from 'lucide-react';
import { useMemo } from 'react';
import { Link } from 'react-router';

import type { Disparo } from '@zapdesk/cliente-motor';

import { chaves } from '../api/chaves';
import { useCliente } from '../api/motor';
import { EsqueletoLista } from '../componentes/Esqueleto';
import { EstadoVazio } from '../componentes/EstadoVazio';
import { TelaErro } from '../componentes/TelaErro';
import { useContaAtual } from '../estado/conta';
import { descreverEstado, percentual, processados, ROTULO_ESTADO_DISPARO } from '../util/disparos';
import { dataHora, numero, plural } from '../util/formatar';

export const VAZIO_DISPAROS = 'Nenhum disparo ainda';

export function SeloEstado({ disparo }: { disparo: Disparo }) {
  const rotulo = disparo.na_fila ? 'Na fila' : ROTULO_ESTADO_DISPARO[disparo.estado];
  return <span className={`selo-estado estado-${disparo.na_fila ? 'na_fila' : disparo.estado}`}>{rotulo}</span>;
}

export function BarraProgresso({ disparo }: { disparo: Disparo }) {
  const p = percentual(disparo);
  const falhou = disparo.contadores.total > 0 ? (disparo.contadores.falhou / disparo.contadores.total) * 100 : 0;
  return (
    <div
      className="barra-progresso"
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={p}
      aria-label={`${p}% processado`}
    >
      <span className="barra-ok" style={{ width: `${Math.max(0, p - falhou)}%` }} />
      <span className="barra-falhou" style={{ width: `${falhou}%` }} />
    </div>
  );
}

function CartaoDisparo({ disparo, nomeConta }: { disparo: Disparo; nomeConta: string }) {
  const c = disparo.contadores;
  return (
    <Link to={`/disparos/${disparo.id}`} className="cartao-disparo">
      <div className="cartao-disparo-topo">
        <strong>{disparo.nome}</strong>
        {disparo.origem === 'mcp' ? (
          <span className="selo info" title="Criado pelo Claude (MCP)">
            <Bot size={12} aria-hidden="true" /> MCP
          </span>
        ) : null}
        <SeloEstado disparo={disparo} />
      </div>
      <BarraProgresso disparo={disparo} />
      <div className="cartao-disparo-linha">
        <span>
          {numero(processados(disparo))} de {numero(c.total)} ·{' '}
          {plural(c.enviado + c.entregue + c.lido + c.respondeu, 'enviado', 'enviados')} ·{' '}
          {plural(c.respondeu, 'respondeu', 'responderam')} · {plural(c.falhou, 'falhou', 'falharam')}
        </span>
      </div>
      <div className="cartao-disparo-linha texto-secundario">
        <span className="cartao-disparo-conta">{nomeConta}</span>
        <span>{descreverEstado(disparo)}</span>
        <span>Criado em {dataHora(disparo.criado_em)}</span>
      </div>
    </Link>
  );
}

export function Disparos() {
  const cliente = useCliente();
  const { contas } = useContaAtual();
  const consulta = useInfiniteQuery({
    queryKey: chaves.disparos,
    queryFn: ({ pageParam }) => cliente.listarDisparos({ limite: 50, ...(pageParam ? { cursor: pageParam } : {}) }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (p) => p.proximo_cursor ?? undefined,
  });
  const disparos = useMemo(() => consulta.data?.pages.flatMap((p) => p.itens) ?? [], [consulta.data]);
  const nomeConta = (id: string) => contas.find((c) => c.id === id)?.nome ?? 'Conta removida';

  return (
    <section className="tela tela-rolavel">
      <header className="cabecalho-tela">
        <h1>Disparos</h1>
        <Link to="/disparos/novo" className="botao">
          <Plus size={16} aria-hidden="true" /> Novo disparo
        </Link>
      </header>
      {consulta.isPending ? (
        <EsqueletoLista linhas={4} />
      ) : consulta.isError ? (
        <TelaErro erro={consulta.error} aoTentar={() => void consulta.refetch()} />
      ) : disparos.length === 0 ? (
        <EstadoVazio
          icone={<Megaphone size={56} />}
          titulo={VAZIO_DISPAROS}
          texto="Envie uma mensagem para uma lista de leads com ritmo, janela de horário e relatório."
          acao={
            <Link to="/disparos/novo" className="botao">
              Novo disparo
            </Link>
          }
        />
      ) : (
        <div className="lista-disparos">
          {disparos.map((d) => (
            <CartaoDisparo key={d.id} disparo={d} nomeConta={nomeConta(d.conta_id)} />
          ))}
          {consulta.hasNextPage ? (
            <button type="button" className="botao secundario" onClick={() => void consulta.fetchNextPage()}>
              Carregar mais
            </button>
          ) : null}
        </div>
      )}
    </section>
  );
}
