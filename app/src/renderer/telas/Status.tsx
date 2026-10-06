// Status (T131): lista por contato (últimas 24 h) e visualizador de texto/imagem/vídeo com avanço
// automático; estado vazio.
import { useQuery } from '@tanstack/react-query';
import { ChevronLeft, ChevronRight, CircleDashed } from 'lucide-react';
import { useEffect, useState } from 'react';
import { Link } from 'react-router';

import type { StatusPorContato } from '@zapdesk/cliente-motor';

import { chaves } from '../api/chaves';
import { useCliente } from '../api/motor';
import { Avatar } from '../componentes/Avatar';
import { BotaoIcone } from '../componentes/BotaoIcone';
import { EsqueletoLista } from '../componentes/Esqueleto';
import { EstadoVazio } from '../componentes/EstadoVazio';
import { SeletorConta } from '../componentes/SeletorConta';
import { TelaErro } from '../componentes/TelaErro';
import { useContaAtual } from '../estado/conta';
import { matiz, rotuloDataLista } from '../util/formatar';

const DURACAO_MS = 6000;

function nome(g: StatusPorContato): string {
  return g.contato_nome ?? g.contato_jid.split('@')[0] ?? 'Contato';
}

function Visualizador({ grupo, aoTerminar }: { grupo: StatusPorContato; aoTerminar: () => void }) {
  const cliente = useCliente();
  // Itens vêm dos mais recentes para os mais antigos; exibe em ordem cronológica.
  const itens = [...grupo.itens].reverse();
  const [indice, setIndice] = useState(0);
  const item = itens[Math.min(indice, itens.length - 1)];

  useEffect(() => setIndice(0), [grupo.contato_jid]);
  useEffect(() => {
    if (!item || item.tipo === 'video') return;
    const t = setTimeout(() => {
      if (indice < itens.length - 1) setIndice((i) => i + 1);
      else aoTerminar();
    }, DURACAO_MS);
    return () => clearTimeout(t);
  }, [item, indice, itens.length, aoTerminar]);

  if (!item) return null;
  // 003 (T040): fundo do status de texto derivado do id, escuro o bastante para --tela-texto (tokens.md §4).
  const tom = matiz(item.id);
  return (
    <div className="visualizador-status">
      <div className="status-barras" aria-hidden="true">
        {itens.map((s, i) => (
          <span key={s.id} className={i < indice ? 'vista' : i === indice ? 'atual' : undefined} />
        ))}
      </div>
      <header className="status-cabecalho">
        <Avatar nome={nome(grupo)} chave={grupo.contato_jid} tamanho={36} />
        <span>
          <strong>{nome(grupo)}</strong>
          <small>{rotuloDataLista(item.publicado_em)}</small>
        </span>
      </header>
      <div className="status-conteudo">
        <BotaoIcone rotulo="Status anterior" disabled={indice === 0} onClick={() => setIndice((i) => Math.max(0, i - 1))}>
          <ChevronLeft size={28} />
        </BotaoIcone>
        {item.tipo === 'texto' ? (
          <div className="status-texto" style={{ background: `hsl(${tom} 35% 30%)` }}>
            {item.texto}
          </div>
        ) : item.tipo === 'imagem' ? (
          <figure className="status-midia">
            <img src={cliente.urlMidiaStatus(item.id)} alt={item.texto ?? 'Status'} />
            {item.texto ? <figcaption>{item.texto}</figcaption> : null}
          </figure>
        ) : (
          <figure className="status-midia">
            <video
              src={cliente.urlMidiaStatus(item.id)}
              autoPlay
              controls
              onEnded={() => (indice < itens.length - 1 ? setIndice((i) => i + 1) : aoTerminar())}
            />
            {item.texto ? <figcaption>{item.texto}</figcaption> : null}
          </figure>
        )}
        <BotaoIcone
          rotulo="Próximo status"
          onClick={() => (indice < itens.length - 1 ? setIndice((i) => i + 1) : aoTerminar())}
        >
          <ChevronRight size={28} />
        </BotaoIcone>
      </div>
    </div>
  );
}

export function Status() {
  const cliente = useCliente();
  const { conta } = useContaAtual();
  const [aberto, setAberto] = useState<string | null>(null);
  const consulta = useQuery({
    queryKey: chaves.status(conta?.id ?? '-'),
    enabled: Boolean(conta),
    queryFn: () => cliente.listarStatus(conta?.id as string),
    refetchInterval: 5 * 60_000,
  });

  if (!conta) {
    return (
      <EstadoVazio
        icone={<CircleDashed size={56} />}
        titulo="Nenhuma conta conectada"
        acao={
          <Link to="/contas/conectar" className="botao">
            Conectar conta
          </Link>
        }
      />
    );
  }

  const grupos = consulta.data ?? [];
  const atual = grupos.find((g) => g.contato_jid === aberto) ?? null;
  const proximo = () => {
    const i = grupos.findIndex((g) => g.contato_jid === aberto);
    setAberto(grupos[i + 1]?.contato_jid ?? null);
  };

  return (
    <section className="tela tela-dividida">
      <aside className="painel-lista">
        <header className="cabecalho-tela">
          <h1>Status</h1>
          <SeletorConta />
        </header>
        {consulta.isPending ? (
          <EsqueletoLista linhas={5} />
        ) : consulta.isError ? (
          <TelaErro erro={consulta.error} aoTentar={() => void consulta.refetch()} />
        ) : grupos.length === 0 ? (
          <EstadoVazio icone={<CircleDashed size={48} />} titulo="Nenhum status nas últimas 24 horas" texto="Os status publicados pelos seus contatos aparecem aqui." />
        ) : (
          <ul className="lista-simples" aria-label="Status recentes">
            {grupos.map((g) => (
              <li key={g.contato_jid}>
                <button type="button" className={`item-status${g.contato_jid === aberto ? ' ativo' : ''}`} onClick={() => setAberto(g.contato_jid)}>
                  <span className="anel-status">
                    <Avatar nome={nome(g)} chave={g.contato_jid} tamanho={44} />
                  </span>
                  <span className="item-contato-textos">
                    <strong>{nome(g)}</strong>
                    <small>
                      {g.itens.length === 1 ? '1 atualização' : `${g.itens.length} atualizações`} · {rotuloDataLista(g.itens[0]?.publicado_em ?? null)}
                    </small>
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </aside>
      <div className="painel-detalhe fundo-escuro">
        {atual ? (
          <Visualizador key={atual.contato_jid} grupo={atual} aoTerminar={proximo} />
        ) : (
          <EstadoVazio icone={<CircleDashed size={56} />} titulo="Clique em um contato para ver o status" />
        )}
      </div>
    </section>
  );
}
