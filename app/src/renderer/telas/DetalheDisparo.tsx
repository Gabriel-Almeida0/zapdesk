// Detalhe do disparo (T099): progresso, contadores por estado, próximo envio, estimativa, estado com
// motivo, Pausar/Retomar/Cancelar (e Iniciar/Excluir em rascunho), destinatários filtráveis e
// Exportar CSV (diálogo salvar via preload).
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Bot, Download, Pause, Play, Search, Square, Trash } from 'lucide-react';
import { useCallback, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router';

import type { Disparo, EstadoDestinatario } from '@zapdesk/cliente-motor';

import { chaves } from '../api/chaves';
import { useCliente } from '../api/motor';
import { BotaoIcone } from '../componentes/BotaoIcone';
import { Esqueleto, EsqueletoLista } from '../componentes/Esqueleto';
import { EstadoVazio } from '../componentes/EstadoVazio';
import { FaixaAviso } from '../componentes/FaixaAviso';
import { ListaVirtual } from '../componentes/ListaVirtual';
import { Confirmar } from '../componentes/Modal';
import { TelaErro } from '../componentes/TelaErro';
import { useContaAtual } from '../estado/conta';
import {
  descreverEstado,
  podeCancelar,
  podePausar,
  podeRetomar,
  processados,
  percentual,
  ROTULO_ESTADO_DESTINATARIO,
  textoJanela,
} from '../util/disparos';
import { dataHora, numero, telefone, textoErro } from '../util/formatar';
import { BarraProgresso, SeloEstado } from './Disparos';

const CONTADORES: { chave: keyof Disparo['contadores']; rotulo: string }[] = [
  { chave: 'pendente', rotulo: 'Pendentes' },
  { chave: 'enviado', rotulo: 'Enviados' },
  { chave: 'entregue', rotulo: 'Entregues' },
  { chave: 'lido', rotulo: 'Lidos' },
  { chave: 'respondeu', rotulo: 'Responderam' },
  { chave: 'falhou', rotulo: 'Falharam' },
];

function nomeArquivoRelatorio(d: Disparo): string {
  const base = d.nome.normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/[^\w-]+/g, '-').replace(/^-|-$/g, '');
  return `relatorio-${base || 'disparo'}.csv`;
}

function Destinatarios({ disparoId }: { disparoId: string }) {
  const cliente = useCliente();
  const [estado, setEstado] = useState<EstadoDestinatario | ''>('');
  const [busca, setBusca] = useState('');
  const consulta = useInfiniteQuery({
    queryKey: [...chaves.destinatarios(disparoId), { estado, busca: busca.trim() }],
    queryFn: ({ pageParam }) =>
      cliente.listarDestinatarios(disparoId, {
        limite: 200,
        ...(pageParam ? { cursor: pageParam } : {}),
        ...(estado ? { estado } : {}),
        ...(busca.trim() ? { busca: busca.trim() } : {}),
      }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (p) => p.proximo_cursor ?? undefined,
  });
  const itens = useMemo(() => consulta.data?.pages.flatMap((p) => p.itens) ?? [], [consulta.data]);
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = consulta;
  const carregarMais = useCallback(() => {
    if (hasNextPage && !isFetchingNextPage) void fetchNextPage();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  return (
    <div className="cartao destinatarios">
      <div className="cartao-cabecalho">
        <h2>Destinatários</h2>
        <div className="barra-ferramentas">
          <label className="campo-busca">
            <Search size={16} aria-hidden="true" />
            <input type="search" aria-label="Buscar destinatário" placeholder="Telefone ou nome" value={busca} onChange={(e) => setBusca(e.target.value)} />
          </label>
          <select className="seletor" aria-label="Filtrar por estado" value={estado} onChange={(e) => setEstado(e.target.value as EstadoDestinatario | '')}>
            <option value="">Todos</option>
            {(Object.keys(ROTULO_ESTADO_DESTINATARIO) as EstadoDestinatario[]).map((e) => (
              <option key={e} value={e}>
                {ROTULO_ESTADO_DESTINATARIO[e]}
              </option>
            ))}
          </select>
        </div>
      </div>
      {consulta.isPending ? (
        <EsqueletoLista linhas={5} />
      ) : consulta.isError ? (
        <TelaErro erro={consulta.error} aoTentar={() => void consulta.refetch()} />
      ) : itens.length === 0 ? (
        <EstadoVazio titulo="Nenhum destinatário neste filtro" />
      ) : (
        <div className="tabela-virtual altura-fixa">
          <div className="linha-tabela destinatarios-grade cabecalho" role="row">
            <span>#</span>
            <span>Telefone</span>
            <span>Nome</span>
            <span>Estado</span>
            <span>Atualizado</span>
            <span>Motivo</span>
          </div>
          <ListaVirtual
            rotulo="Destinatários"
            itens={itens}
            estimativa={40}
            chave={(d) => d.id}
            aoChegarNoFim={carregarMais}
            renderizar={(d) => (
              <div className="linha-tabela destinatarios-grade">
                <span className="texto-secundario">{d.ordem}</span>
                <span className="mono">{telefone(d.telefone)}</span>
                <span>{d.nome ?? '—'}</span>
                <span>
                  <span className={`selo-destinatario estado-${d.estado}`}>{ROTULO_ESTADO_DESTINATARIO[d.estado]}</span>
                </span>
                <span className="texto-secundario">
                  {dataHora(d.respondeu_em ?? d.lido_em ?? d.entregue_em ?? d.enviado_em ?? d.falhou_em ?? d.enviando_em)}
                </span>
                <span className="texto-secundario">{d.motivo_falha ?? ''}</span>
              </div>
            )}
          />
        </div>
      )}
    </div>
  );
}

export function DetalheDisparo() {
  const { disparoId = '' } = useParams();
  const cliente = useCliente();
  const qc = useQueryClient();
  const navegar = useNavigate();
  const { contas } = useContaAtual();
  const [confirmar, setConfirmar] = useState<'cancelar' | 'excluir' | null>(null);
  const [exportado, setExportado] = useState<string | null>(null);

  const consulta = useQuery({ queryKey: chaves.disparo(disparoId), queryFn: () => cliente.obterDisparo(disparoId) });

  const acao = useMutation({
    mutationFn: async (tipo: 'pausar' | 'retomar' | 'cancelar' | 'iniciar' | 'excluir') => {
      if (tipo === 'pausar') return cliente.pausarDisparo(disparoId);
      if (tipo === 'retomar') return cliente.retomarDisparo(disparoId);
      if (tipo === 'cancelar') return cliente.cancelarDisparo(disparoId);
      if (tipo === 'iniciar') return cliente.iniciarDisparo(disparoId);
      await cliente.excluirDisparo(disparoId);
      return null;
    },
    onSuccess: (d) => {
      setConfirmar(null);
      void qc.invalidateQueries({ queryKey: chaves.disparos });
      if (d) qc.setQueryData(chaves.disparo(d.id), d);
      else navegar('/disparos', { replace: true });
    },
    onError: () => setConfirmar(null),
  });

  const exportar = useMutation({
    mutationFn: (d: Disparo) => window.zapdesk.exportarRelatorio(d.id, nomeArquivoRelatorio(d)),
    onSuccess: (r) => setExportado(r.salvo && r.caminho ? r.caminho : null),
  });

  if (consulta.isPending) {
    return (
      <section className="tela tela-rolavel">
        <Esqueleto altura={28} largura={260} />
        <Esqueleto altura={120} />
      </section>
    );
  }
  if (consulta.isError) return <TelaErro erro={consulta.error} aoTentar={() => void consulta.refetch()} />;
  const d = consulta.data;
  const conta = contas.find((c) => c.id === d.conta_id);
  const pausadoPorProblema = d.estado === 'pausado' && d.motivo_pausa && d.motivo_pausa !== 'usuario';

  return (
    <section className="tela tela-rolavel detalhe-disparo">
      <header className="cabecalho-tela">
        <BotaoIcone rotulo="Voltar para Disparos" onClick={() => navegar('/disparos')}>
          <ArrowLeft size={20} />
        </BotaoIcone>
        <h1>{d.nome}</h1>
        <SeloEstado disparo={d} />
        {d.origem === 'mcp' ? (
          <span className="selo info">
            <Bot size={12} aria-hidden="true" /> Criado pelo Claude
          </span>
        ) : null}
        <div className="acoes-cabecalho">
          {d.estado === 'rascunho' ? (
            <>
              <button type="button" className="botao secundario perigo-texto" onClick={() => setConfirmar('excluir')}>
                <Trash size={16} aria-hidden="true" /> Excluir rascunho
              </button>
              <button type="button" className="botao" disabled={acao.isPending} onClick={() => acao.mutate('iniciar')}>
                <Play size={16} aria-hidden="true" /> Iniciar
              </button>
            </>
          ) : null}
          {podePausar(d) ? (
            <button type="button" className="botao secundario" disabled={acao.isPending} onClick={() => acao.mutate('pausar')}>
              <Pause size={16} aria-hidden="true" /> Pausar
            </button>
          ) : null}
          {podeRetomar(d) ? (
            <button type="button" className="botao" disabled={acao.isPending} onClick={() => acao.mutate('retomar')}>
              <Play size={16} aria-hidden="true" /> Retomar
            </button>
          ) : null}
          {podeCancelar(d) && d.estado !== 'rascunho' ? (
            <button type="button" className="botao secundario perigo-texto" onClick={() => setConfirmar('cancelar')}>
              <Square size={16} aria-hidden="true" /> Cancelar
            </button>
          ) : null}
          <button type="button" className="botao secundario" disabled={exportar.isPending} onClick={() => exportar.mutate(d)}>
            <Download size={16} aria-hidden="true" /> Exportar CSV
          </button>
        </div>
      </header>

      {pausadoPorProblema ? <FaixaAviso tipo="aviso">{descreverEstado(d)}</FaixaAviso> : null}
      {acao.error ? <FaixaAviso tipo="erro">{textoErro(acao.error)}</FaixaAviso> : null}
      {exportar.error ? <FaixaAviso tipo="erro">{textoErro(exportar.error)}</FaixaAviso> : null}
      {exportado ? (
        <FaixaAviso
          tipo="info"
          acao={
            <button type="button" className="botao-link" onClick={() => void window.zapdesk.mostrarNoFinder(exportado)}>
              Mostrar no Finder
            </button>
          }
        >
          Relatório salvo em {exportado}
        </FaixaAviso>
      ) : null}

      <div className="cartao cartao-recuado progresso-disparo">
        <div className="progresso-topo">
          <strong className="percentual">{percentual(d)}%</strong>
          <span>
            {numero(processados(d))} de {numero(d.contadores.total)} processados
          </span>
          <span className="estado-linha" role="status">
            {descreverEstado(d)}
          </span>
        </div>
        <BarraProgresso disparo={d} />
        <div className="contadores">
          {CONTADORES.map((c) => (
            <div key={c.chave} className={`contador contador-${c.chave}`}>
              <strong>{numero(d.contadores[c.chave])}</strong>
              <span>{c.rotulo}</span>
            </div>
          ))}
        </div>
        <dl className="lista-definicoes em-linha">
          <dt>Conta</dt>
          <dd>{conta?.nome ?? 'Conta removida'}</dd>
          <dt>Próximo envio</dt>
          <dd>{d.proximo_envio_em ? dataHora(d.proximo_envio_em) : '—'}</dd>
          <dt>Término estimado</dt>
          <dd>{d.estimativa_termino_em ? dataHora(d.estimativa_termino_em) : '—'}</dd>
          <dt>Ritmo</dt>
          <dd>
            {d.ritmo.intervalo_min_s}–{d.ritmo.intervalo_max_s} s
            {d.ritmo.limite_por_hora ? ` · ${d.ritmo.limite_por_hora}/hora` : ''}
            {d.ritmo.limite_por_dia ? ` · ${d.ritmo.limite_por_dia}/dia` : ''}
          </dd>
          <dt>Janela</dt>
          <dd>{textoJanela(d.janela) ?? 'Qualquer horário'}</dd>
          <dt>Criado em</dt>
          <dd>{dataHora(d.criado_em)}</dd>
        </dl>
        <details className="mensagem-disparo">
          <summary>Mensagem</summary>
          <p className="pre">{d.mensagem}</p>
          {d.arquivo ? <p className="texto-secundario">Anexo: {d.arquivo.nome}</p> : null}
        </details>
      </div>

      <Destinatarios disparoId={d.id} />

      {confirmar === 'cancelar' ? (
        <Confirmar
          titulo="Cancelar disparo?"
          texto="Os destinatários pendentes não vão receber a mensagem. Não dá para desfazer."
          confirmar="Cancelar disparo"
          recusar="Voltar"
          perigo
          ocupado={acao.isPending}
          aoConfirmar={() => acao.mutate('cancelar')}
          aoFechar={() => setConfirmar(null)}
        />
      ) : null}
      {confirmar === 'excluir' ? (
        <Confirmar
          titulo="Excluir rascunho?"
          texto="O rascunho será apagado. Os leads continuam na base."
          confirmar="Excluir"
          perigo
          ocupado={acao.isPending}
          aoConfirmar={() => acao.mutate('excluir')}
          aoFechar={() => setConfirmar(null)}
        />
      ) : null}
    </section>
  );
}
