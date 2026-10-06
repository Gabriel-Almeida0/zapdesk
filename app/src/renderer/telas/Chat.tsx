// Chat (T059): cabeçalho, mensagens com paginação infinita para cima (coluna invertida: abre no
// fim sem pular), separadores de dia, marca como lida ao abrir, abrir na mensagem da busca,
// arrastar e soltar anexos, painel do contato.
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Info } from 'lucide-react';
import { Fragment, useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { useSearchParams } from 'react-router';

import type { Conversa, Id, Mensagem } from '@zapdesk/cliente-motor';

import { chaves } from '../api/chaves';
import { atualizarMensagem } from '../api/eventos';
import { useCliente, useEventoMotor } from '../api/motor';
import { Avatar } from '../componentes/Avatar';
import { Bolha } from '../componentes/Bolha';
import { BotaoIcone } from '../componentes/BotaoIcone';
import { Composer } from '../componentes/Composer';
import { Esqueleto } from '../componentes/Esqueleto';
import { BotaoAssumir, FaixaAutomacoes } from '../componentes/FaixaAutomacoes';
import { FaixaAviso } from '../componentes/FaixaAviso';
import { Confirmar } from '../componentes/Modal';
import { PainelContato } from '../componentes/PainelContato';
import { TelaErro } from '../componentes/TelaErro';
import { chaveDia, nomeExibicao, rotuloDia, telefone, textoErro } from '../util/formatar';

const TAMANHO_PAGINA = 50;
const MAX_PAGINAS_PROCURANDO = 40;

function EsqueletoMensagens() {
  return (
    <div className="esqueleto-mensagens" role="status" aria-label="Carregando mensagens">
      {[60, 35, 50, 25, 45, 30].map((largura, i) => (
        <div key={i} className={`linha-mensagem ${i % 2 ? 'minha' : 'dele'}`}>
          <Esqueleto largura={`${largura}%`} altura={34} />
        </div>
      ))}
    </div>
  );
}

export function Chat({ conversaId }: { conversaId: Id }) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const [parametros, setParametros] = useSearchParams();
  const alvo = parametros.get('mensagem');

  const [citando, setCitando] = useState<Mensagem | null>(null);
  const [editando, setEditando] = useState<Mensagem | null>(null);
  const [apagando, setApagando] = useState<Mensagem | null>(null);
  const [painel, setPainel] = useState(false);
  const [destaque, setDestaque] = useState<string | null>(null);
  const [arrastando, setArrastando] = useState(false);
  const [soltos, setSoltos] = useState<File[]>([]);
  const [erroAcao, setErroAcao] = useState<string | null>(null);
  const rolagem = useRef<HTMLDivElement>(null);

  const conversa = useQuery({ queryKey: chaves.conversa(conversaId), queryFn: () => cliente.obterConversa(conversaId) });

  const mensagens = useInfiniteQuery({
    queryKey: chaves.mensagens(conversaId),
    queryFn: ({ pageParam }) =>
      cliente.listarMensagens(conversaId, { limite: TAMANHO_PAGINA, ...(pageParam ? { antes: pageParam } : {}) }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (pagina) => (pagina.proximo_cursor ? pagina.itens.at(-1)?.id : undefined),
    staleTime: 5 * 60_000,
  });
  const itens = useMemo(() => mensagens.data?.pages.flatMap((p) => p.itens) ?? [], [mensagens.data]);

  // Marcar como lida ao abrir e quando chega mensagem com a conversa aberta.
  const marcar = useMutation({
    mutationFn: () => cliente.marcarLida(conversaId),
    onSuccess: () => {
      qc.setQueryData<Conversa>(chaves.conversa(conversaId), (c) => (c ? { ...c, nao_lidas: 0 } : c));
      const atual = qc.getQueryData<Conversa>(chaves.conversa(conversaId));
      if (atual) void qc.invalidateQueries({ queryKey: chaves.conversas(atual.conta_id) });
    },
  });
  const { mutate: marcarLida } = marcar;
  const naoLidas = conversa.data?.nao_lidas ?? 0;
  useEffect(() => {
    if (naoLidas > 0) marcarLida();
  }, [naoLidas, marcarLida]);
  useEventoMotor((evento) => {
    if (evento.tipo === 'mensagem.nova' && evento.dados.conversa_id === conversaId && !evento.dados.de_mim && document.hasFocus()) {
      marcarLida();
    }
  });

  // Carregar mensagens mais antigas ao chegar perto do topo.
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = mensagens;
  const carregarMais = useCallback(() => {
    if (hasNextPage && !isFetchingNextPage) void fetchNextPage();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);
  const aoRolar = () => {
    const el = rolagem.current;
    if (!el) return;
    // Coluna invertida: scrollTop vai de 0 (fim) a valores negativos (topo).
    const distanciaDoTopo = el.scrollHeight - el.clientHeight - Math.abs(el.scrollTop);
    if (distanciaDoTopo < 400) carregarMais();
  };
  // Se as mensagens não enchem a tela, carregar mais já.
  useEffect(() => {
    const el = rolagem.current;
    if (el && el.scrollHeight > 0 && el.scrollHeight <= el.clientHeight + 10) carregarMais();
  }, [itens.length, carregarMais]);

  // Abrir na mensagem vinda da busca.
  const paginas = mensagens.data?.pages.length ?? 0;
  useEffect(() => {
    if (!alvo || mensagens.isPending) return;
    if (itens.some((m) => m.id === alvo)) {
      document.getElementById(`msg-${alvo}`)?.scrollIntoView?.({ block: 'center' });
      setDestaque(alvo);
      const t = setTimeout(() => setDestaque(null), 2500);
      setParametros({}, { replace: true });
      return () => clearTimeout(t);
    }
    if (hasNextPage && !isFetchingNextPage && paginas < MAX_PAGINAS_PROCURANDO) void fetchNextPage();
    return undefined;
  }, [alvo, itens, mensagens.isPending, hasNextPage, isFetchingNextPage, paginas, fetchNextPage, setParametros]);

  const reagir = useMutation({
    mutationFn: (d: { id: string; emoji: string }) => cliente.reagir(d.id, d.emoji),
    onError: (e) => setErroAcao(textoErro(e)),
  });
  const reenviar = useMutation({
    mutationFn: (id: string) => cliente.reenviarMensagem(id),
    onSuccess: (m) => atualizarMensagem(qc, m),
    onError: (e) => setErroAcao(textoErro(e)),
  });
  const apagar = useMutation({
    mutationFn: (id: string) => cliente.apagarMensagem(id),
    onSuccess: () => setApagando(null),
    onError: (e) => {
      setApagando(null);
      setErroAcao(textoErro(e));
    },
  });

  if (conversa.isError) return <TelaErro erro={conversa.error} aoTentar={() => void conversa.refetch()} />;
  const dados = conversa.data;
  const grupo = dados?.tipo === 'grupo';

  // Mensagens (mais recentes primeiro) com separador de dia depois da mais antiga de cada dia.
  const linhas: ReactNode[] = [];
  itens.forEach((m, i) => {
    linhas.push(
      <Bolha
        key={m.id}
        mensagem={m}
        grupo={grupo}
        destacada={destaque === m.id}
        aoResponder={(msg) => {
          setEditando(null);
          setCitando(msg);
        }}
        aoReagir={(msg, emoji) => reagir.mutate({ id: msg.id, emoji })}
        aoEditar={(msg) => {
          setCitando(null);
          setEditando(msg);
        }}
        aoApagar={(msg) => setApagando(msg)}
        aoReenviar={(msg) => reenviar.mutate(msg.id)}
      />,
    );
    const anterior = itens[i + 1];
    if (!anterior || chaveDia(anterior.enviada_em) !== chaveDia(m.enviada_em)) {
      if (anterior || !hasNextPage) {
        linhas.push(
          <div key={`dia-${m.id}`} className="separador-dia" role="separator">
            <span>{rotuloDia(m.enviada_em)}</span>
          </div>,
        );
      }
    }
  });

  return (
    <div className="chat-com-painel">
      <section
        className={`chat${arrastando ? ' arrastando' : ''}`}
        aria-label={dados ? `Conversa com ${nomeExibicao(dados.nome, dados.telefone)}` : 'Conversa'}
        onDragOver={(e) => {
          if (e.dataTransfer.types.includes('Files')) {
            e.preventDefault();
            setArrastando(true);
          }
        }}
        onDragLeave={(e) => {
          if (e.currentTarget === e.target) setArrastando(false);
        }}
        onDrop={(e) => {
          e.preventDefault();
          setArrastando(false);
          const arquivos = [...e.dataTransfer.files];
          if (arquivos.length > 0) setSoltos(arquivos);
        }}
      >
        <header className="cabecalho-chat">
          {dados ? (
            <button type="button" className="cabecalho-chat-contato" onClick={() => setPainel((p) => !p)}>
              <Avatar nome={dados.nome} chave={dados.jid} tamanho={40} grupo={grupo} />
              <span className="cabecalho-chat-textos">
                <strong>{nomeExibicao(dados.nome, dados.telefone)}</strong>
                <small>{grupo ? 'Grupo' : telefone(dados.telefone)}</small>
              </span>
            </button>
          ) : (
            <div className="cabecalho-chat-contato">
              <Esqueleto largura={40} altura={40} redondo />
              <Esqueleto largura={160} altura={14} />
            </div>
          )}
          <div className="acoes-cabecalho">
            <BotaoAssumir conversaId={conversaId} />
            {dados?.etiquetas.map((e) => (
              <span key={e.id} className="chip-etiqueta mini">
                <span className="ponto-etiqueta" style={{ background: e.cor }} aria-hidden="true" />
                {e.nome}
              </span>
            ))}
            <BotaoIcone rotulo="Dados do contato" ativo={painel} onClick={() => setPainel((p) => !p)}>
              <Info size={20} />
            </BotaoIcone>
          </div>
        </header>

        <FaixaAutomacoes conversaId={conversaId} />

        {erroAcao ? (
          <FaixaAviso
            tipo="erro"
            acao={
              <button type="button" className="botao-link" onClick={() => setErroAcao(null)}>
                Fechar
              </button>
            }
          >
            {erroAcao}
          </FaixaAviso>
        ) : null}

        <div className="mensagens" ref={rolagem} onScroll={aoRolar} role="log" aria-label="Mensagens" aria-live="polite">
          {mensagens.isPending ? (
            <EsqueletoMensagens />
          ) : mensagens.isError ? (
            <TelaErro erro={mensagens.error} aoTentar={() => void mensagens.refetch()} />
          ) : itens.length === 0 ? (
            <div className="chat-vazio">
              <span>Nenhuma mensagem ainda. Diga oi!</span>
            </div>
          ) : (
            <Fragment>
              {linhas}
              {isFetchingNextPage ? (
                <div className="carregando-antigas" role="status">
                  Carregando mensagens antigas…
                </div>
              ) : null}
            </Fragment>
          )}
        </div>

        {dados ? (
          <Composer
            conversa={dados}
            citando={citando}
            aoLimparCitacao={() => setCitando(null)}
            editando={editando}
            aoLimparEdicao={() => setEditando(null)}
            arquivosSoltos={soltos}
            aoConsumirArquivos={() => setSoltos([])}
          />
        ) : null}
        {arrastando ? (
          <div className="soltar-aqui" aria-hidden="true">
            Solte o arquivo para anexar
          </div>
        ) : null}
      </section>
      {painel && dados ? <PainelContato conversa={dados} aoFechar={() => setPainel(false)} /> : null}
      {apagando ? (
        <Confirmar
          titulo="Apagar mensagem?"
          texto="A mensagem será apagada para todos nesta conversa."
          confirmar="Apagar para todos"
          perigo
          ocupado={apagar.isPending}
          aoConfirmar={() => apagar.mutate(apagando.id)}
          aoFechar={() => setApagando(null)}
        />
      ) : null}
    </div>
  );
}
