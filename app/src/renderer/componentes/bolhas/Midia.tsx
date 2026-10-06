// Conteúdo de mídia das bolhas (T111): imagem (miniatura → completa), vídeo, áudio (voz e comum),
// documento, figurinha e o estado "Não foi possível baixar." + "Tentar baixar de novo".
// A mídia é baixada sob demanda pelo motor (`GET /v1/mensagens/{id}/midia?token=`).
import { Download, FileText, Headphones, Mic, Pause, Play, RefreshCw, X } from 'lucide-react';
import { useEffect, useRef, useState, type CSSProperties } from 'react';

import type { Mensagem, Midia } from '@zapdesk/cliente-motor';

import { useCliente } from '../../api/motor';
import { duracao, tamanho } from '../../util/formatar';

export const TEXTO_FALHA_DOWNLOAD = 'Não foi possível baixar.';
export const TEXTO_TENTAR_BAIXAR = 'Tentar baixar de novo';

/** URL da mídia com "tentativa" para forçar novo download após erro. */
function useUrlMidia(mensagem: Mensagem) {
  const cliente = useCliente();
  const [tentativa, setTentativa] = useState(0);
  const [erro, setErro] = useState(false);
  const base = cliente.urlMidiaMensagem(mensagem.id);
  const url = tentativa > 0 ? `${base}&tentativa=${tentativa}` : base;
  return {
    url,
    erro,
    falhou: () => setErro(true),
    tentarDeNovo: () => {
      setErro(false);
      setTentativa((t) => t + 1);
    },
  };
}

export function ErroDownload({ aoTentar }: { aoTentar: () => void }) {
  return (
    <div className="midia-erro" role="alert">
      <span>{TEXTO_FALHA_DOWNLOAD}</span>
      <button type="button" className="botao-link" onClick={aoTentar}>
        <RefreshCw size={14} aria-hidden="true" /> {TEXTO_TENTAR_BAIXAR}
      </button>
    </div>
  );
}

function miniatura(midia: Midia): string | undefined {
  return midia.miniatura_b64 ? `data:image/jpeg;base64,${midia.miniatura_b64}` : undefined;
}

function proporcao(midia: Midia, larguraMax = 330, alturaMax = 360) {
  const l = midia.largura ?? 300;
  const a = midia.altura ?? 220;
  const escala = Math.min(1, larguraMax / l, alturaMax / a);
  return { width: Math.max(120, Math.round(l * escala)), height: Math.max(80, Math.round(a * escala)) };
}

export function Visualizador(props: { url: string; alt: string; aoFechar: () => void }) {
  const { aoFechar } = props;
  useEffect(() => {
    const tecla = (e: KeyboardEvent) => e.key === 'Escape' && aoFechar();
    window.addEventListener('keydown', tecla);
    return () => window.removeEventListener('keydown', tecla);
  }, [aoFechar]);
  return (
    <div className="visualizador" role="dialog" aria-modal="true" aria-label={props.alt} onClick={aoFechar}>
      <button type="button" className="botao-icone visualizador-fechar" aria-label="Fechar" onClick={aoFechar}>
        <X size={24} />
      </button>
      <img src={props.url} alt={props.alt} onClick={(e) => e.stopPropagation()} />
    </div>
  );
}

export function MidiaImagem({ mensagem }: { mensagem: Mensagem }) {
  const midia = mensagem.midia as Midia;
  const { url, erro, falhou, tentarDeNovo } = useUrlMidia(mensagem);
  const [carregou, setCarregou] = useState(false);
  const [aberto, setAberto] = useState(false);
  const dimensoes = proporcao(midia);
  return (
    <div className="midia-imagem" style={dimensoes}>
      {miniatura(midia) && !carregou ? <img className="midia-miniatura" src={miniatura(midia)} alt="" aria-hidden="true" /> : null}
      {erro ? (
        <ErroDownload aoTentar={tentarDeNovo} />
      ) : (
        <button type="button" className="midia-botao" aria-label="Abrir imagem" onClick={() => setAberto(true)}>
          <img
            src={url}
            alt={mensagem.texto ?? 'Imagem'}
            loading="lazy"
            onLoad={() => setCarregou(true)}
            onError={falhou}
            style={{ opacity: carregou ? 1 : 0 }}
          />
        </button>
      )}
      {aberto ? <Visualizador url={url} alt={mensagem.texto ?? 'Imagem'} aoFechar={() => setAberto(false)} /> : null}
    </div>
  );
}

export function MidiaVideo({ mensagem }: { mensagem: Mensagem }) {
  const midia = mensagem.midia as Midia;
  const { url, erro, falhou, tentarDeNovo } = useUrlMidia(mensagem);
  const [ligado, setLigado] = useState(false);
  const dimensoes = proporcao(midia);
  if (erro) return <ErroDownload aoTentar={tentarDeNovo} />;
  if (!ligado) {
    return (
      <button type="button" className="midia-video-capa" style={dimensoes} aria-label="Reproduzir vídeo" onClick={() => setLigado(true)}>
        {miniatura(midia) ? <img src={miniatura(midia)} alt="" /> : null}
        <span className="midia-play">
          <Play size={28} fill="currentColor" />
        </span>
        {midia.duracao_s ? <span className="midia-duracao">{duracao(midia.duracao_s)}</span> : null}
      </button>
    );
  }
  return <video className="midia-video" style={dimensoes} src={url} controls autoPlay onError={falhou} />;
}

const VELOCIDADES = [1, 1.5, 2] as const;

export function MidiaAudio({ mensagem }: { mensagem: Mensagem }) {
  const midia = mensagem.midia as Midia;
  const { url, erro, falhou, tentarDeNovo } = useUrlMidia(mensagem);
  const audio = useRef<HTMLAudioElement>(null);
  const [tocando, setTocando] = useState(false);
  const [tempo, setTempo] = useState(0);
  const [total, setTotal] = useState(midia.duracao_s ?? 0);
  const [velocidade, setVelocidade] = useState<(typeof VELOCIDADES)[number]>(1);

  if (erro) return <ErroDownload aoTentar={tentarDeNovo} />;

  const alternar = () => {
    const a = audio.current;
    if (!a) return;
    if (a.paused) void a.play().catch(falhou);
    else a.pause();
  };

  return (
    <div className={`midia-audio${midia.ptt ? ' voz' : ''}`}>
      <span className="midia-audio-icone" aria-hidden="true">
        {midia.ptt ? <Mic size={20} /> : <Headphones size={20} />}
      </span>
      <button type="button" className="botao-icone" aria-label={tocando ? 'Pausar áudio' : 'Tocar áudio'} onClick={alternar}>
        {tocando ? <Pause size={22} fill="currentColor" /> : <Play size={22} fill="currentColor" />}
      </button>
      <input
        type="range"
        className="midia-audio-barra"
        aria-label="Posição do áudio"
        min={0}
        max={Math.max(total, 0.1)}
        step={0.1}
        value={Math.min(tempo, total || tempo)}
        style={{ '--progresso': `${total > 0 ? Math.min(100, (tempo / total) * 100) : 0}%` } as CSSProperties}
        onChange={(e) => {
          const valor = Number(e.target.value);
          if (audio.current) audio.current.currentTime = valor;
          setTempo(valor);
        }}
      />
      <span className="midia-audio-tempo">{duracao(tocando || tempo > 0 ? tempo : total)}</span>
      <button
        type="button"
        className="midia-audio-velocidade"
        aria-label={`Velocidade ${velocidade}x`}
        onClick={() => {
          const proxima = VELOCIDADES[(VELOCIDADES.indexOf(velocidade) + 1) % VELOCIDADES.length] ?? 1;
          setVelocidade(proxima);
          if (audio.current) audio.current.playbackRate = proxima;
        }}
      >
        {velocidade}x
      </button>
      <audio
        ref={audio}
        src={url}
        preload="none"
        onPlay={() => setTocando(true)}
        onPause={() => setTocando(false)}
        onEnded={() => {
          setTocando(false);
          setTempo(0);
        }}
        onTimeUpdate={(e) => setTempo(e.currentTarget.currentTime)}
        onLoadedMetadata={(e) => Number.isFinite(e.currentTarget.duration) && setTotal(e.currentTarget.duration)}
        onError={falhou}
      />
    </div>
  );
}

export function MidiaDocumento({ mensagem }: { mensagem: Mensagem }) {
  const midia = mensagem.midia as Midia;
  const [estado, setEstado] = useState<'ocioso' | 'baixando' | 'erro'>('ocioso');
  const nome = midia.nome_arquivo ?? 'documento';
  const extensao = nome.includes('.') ? nome.split('.').pop()?.toUpperCase() : null;
  const abrir = () => {
    setEstado('baixando');
    window.zapdesk
      .abrirArquivoMotor(midia.url, nome)
      .then(() => setEstado('ocioso'))
      .catch(() => setEstado('erro'));
  };
  return (
    <div className="midia-documento">
      <FileText size={30} aria-hidden="true" />
      <span className="midia-documento-textos">
        <span className="midia-documento-nome">{nome}</span>
        <small>
          {extensao ? `${extensao} · ` : ''}
          {tamanho(midia.tamanho)}
        </small>
      </span>
      <button
        type="button"
        className="botao-icone"
        aria-label={`Baixar e abrir ${nome}`}
        disabled={estado === 'baixando'}
        onClick={abrir}
      >
        <Download size={20} />
      </button>
      {estado === 'erro' ? <ErroDownload aoTentar={abrir} /> : null}
    </div>
  );
}

export function MidiaFigurinha({ mensagem }: { mensagem: Mensagem }) {
  const { url, erro, falhou, tentarDeNovo } = useUrlMidia(mensagem);
  if (erro) return <ErroDownload aoTentar={tentarDeNovo} />;
  return <img className="midia-figurinha" src={url} alt="Figurinha" width={160} height={160} loading="lazy" onError={falhou} />;
}
