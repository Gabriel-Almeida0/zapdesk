// Bolha de mensagem (T059/T111): texto, mídia, citação, reações, editada/apagada e estado
// (pendente, enviada, entregue, lida, falhou + "Reenviar").
import { Ban, Check, CheckCheck, CircleAlert, Clock } from 'lucide-react';
import type { ReactNode } from 'react';

import type { EstadoMensagem, Mensagem } from '@zapdesk/cliente-motor';

import { hora, matiz } from '../util/formatar';
import { MidiaAudio, MidiaDocumento, MidiaFigurinha, MidiaImagem, MidiaVideo } from './bolhas/Midia';
import { MenuMensagem, type AcoesMensagem } from './MenuMensagem';

export const ROTULO_ESTADO: Record<EstadoMensagem, string> = {
  pendente: 'Enviando',
  enviada: 'Enviada',
  entregue: 'Entregue',
  lida: 'Lida',
  falhou: 'Não enviada',
  recebida: 'Recebida',
};

function IconeEstado({ estado }: { estado: EstadoMensagem }) {
  const rotulo = ROTULO_ESTADO[estado];
  switch (estado) {
    case 'pendente':
      return <Clock size={14} className="estado-msg" aria-label={rotulo} role="img" />;
    case 'enviada':
      return <Check size={16} className="estado-msg" aria-label={rotulo} role="img" />;
    case 'entregue':
      return <CheckCheck size={16} className="estado-msg" aria-label={rotulo} role="img" />;
    case 'lida':
      return <CheckCheck size={16} className="estado-msg lida" aria-label={rotulo} role="img" />;
    case 'falhou':
      return <CircleAlert size={15} className="estado-msg falhou" aria-label={rotulo} role="img" />;
    default:
      return null;
  }
}

/** Texto com links clicáveis (abrem no navegador). */
export function TextoMensagem({ texto }: { texto: string }) {
  const partes: ReactNode[] = [];
  const regex = /\bhttps?:\/\/[^\s<>"']+/g;
  let ultimo = 0;
  for (const m of texto.matchAll(regex)) {
    const inicio = m.index ?? 0;
    if (inicio > ultimo) partes.push(texto.slice(ultimo, inicio));
    const url = m[0];
    partes.push(
      <a key={inicio} href={url} target="_blank" rel="noreferrer">
        {url}
      </a>,
    );
    ultimo = inicio + url.length;
  }
  if (ultimo < texto.length) partes.push(texto.slice(ultimo));
  return <span className="texto-mensagem">{partes}</span>;
}

function Conteudo({ mensagem }: { mensagem: Mensagem }) {
  if (mensagem.apagada) {
    return (
      <span className="mensagem-apagada">
        <Ban size={14} aria-hidden="true" /> {mensagem.de_mim ? 'Você apagou esta mensagem' : 'Mensagem apagada'}
      </span>
    );
  }
  const legenda = mensagem.texto ? <TextoMensagem texto={mensagem.texto} /> : null;
  if (!mensagem.midia) return legenda ?? <span className="texto-secundario">Mensagem não suportada</span>;
  switch (mensagem.tipo) {
    case 'imagem':
      return (
        <>
          <MidiaImagem mensagem={mensagem} />
          {legenda}
        </>
      );
    case 'video':
      return (
        <>
          <MidiaVideo mensagem={mensagem} />
          {legenda}
        </>
      );
    case 'audio':
      return <MidiaAudio mensagem={mensagem} />;
    case 'documento':
      return (
        <>
          <MidiaDocumento mensagem={mensagem} />
          {legenda}
        </>
      );
    case 'figurinha':
      return <MidiaFigurinha mensagem={mensagem} />;
    default:
      return legenda;
  }
}

function agruparReacoes(m: Mensagem): { emoji: string; total: number; minha: boolean }[] {
  const mapa = new Map<string, { emoji: string; total: number; minha: boolean }>();
  for (const r of m.reacoes) {
    if (!r.emoji) continue;
    const item = mapa.get(r.emoji) ?? { emoji: r.emoji, total: 0, minha: false };
    item.total += 1;
    item.minha ||= r.de_mim;
    mapa.set(r.emoji, item);
  }
  return [...mapa.values()];
}

export interface PropsBolha extends AcoesMensagem {
  mensagem: Mensagem;
  grupo: boolean;
  destacada?: boolean;
  aoReenviar: (m: Mensagem) => void;
}

export function Bolha({ mensagem, grupo, destacada, aoReenviar, ...acoes }: PropsBolha) {
  if (mensagem.tipo === 'sistema') {
    return (
      <div className="mensagem-sistema" id={`msg-${mensagem.id}`}>
        <span>{mensagem.texto}</span>
      </div>
    );
  }
  const figurinha = mensagem.tipo === 'figurinha' && !mensagem.apagada;
  const reacoes = agruparReacoes(mensagem);
  const nomeRemetente = grupo && !mensagem.de_mim ? (mensagem.remetente_nome ?? mensagem.remetente_jid.split('@')[0]) : null;

  return (
    <div
      id={`msg-${mensagem.id}`}
      className={`linha-mensagem ${mensagem.de_mim ? 'minha' : 'dele'}${destacada ? ' destacada' : ''}${reacoes.length > 0 ? ' com-reacoes' : ''}`}
    >
      <div
        className={`bolha${figurinha ? ' sem-fundo' : ''}${mensagem.estado === 'falhou' ? ' falhou' : ''}`}
        data-estado={mensagem.estado}
      >
        <MenuMensagem mensagem={mensagem} {...acoes} />
        {nomeRemetente ? (
          <span className="remetente" style={{ color: `hsl(${matiz(mensagem.remetente_jid)} 55% var(--remetente-l))` }}>
            {nomeRemetente}
          </span>
        ) : null}
        {mensagem.citacao ? (
          <div className="citacao">
            <strong>{mensagem.citacao.remetente_nome ?? 'Mensagem'}</strong>
            <span>{mensagem.citacao.resumo}</span>
          </div>
        ) : null}
        {mensagem.disparo_id ? <span className="marca-disparo">Disparo</span> : null}
        {mensagem.automacao_id ? (
          <span className="marca-disparo marca-automacao" title="Enviada por uma automação">
            automática
          </span>
        ) : null}
        <Conteudo mensagem={mensagem} />
        <span className="rodape-bolha">
          {mensagem.editada && !mensagem.apagada ? <span className="editada">editada</span> : null}
          <time dateTime={mensagem.enviada_em}>{hora(mensagem.enviada_em)}</time>
          {mensagem.de_mim ? <IconeEstado estado={mensagem.estado} /> : null}
        </span>
        {reacoes.length > 0 ? (
          <span className="reacoes" aria-label="Reações">
            {reacoes.map((r) => (
              <span key={r.emoji} className={r.minha ? 'minha' : undefined}>
                {r.emoji}
                {r.total > 1 ? <small>{r.total}</small> : null}
              </span>
            ))}
          </span>
        ) : null}
      </div>
      {mensagem.estado === 'falhou' ? (
        <div className="falha-envio" role="alert">
          <span>{mensagem.erro ?? 'Não foi possível enviar.'}</span>
          <button type="button" className="botao-link" onClick={() => aoReenviar(mensagem)}>
            Reenviar
          </button>
        </div>
      ) : null}
    </div>
  );
}
