// Gravar áudio pelo microfone (T113): MediaRecorder em `audio/webm;codecs=opus`; o motor remuxa
// para OGG/Opus ao enviar como voz. UI: tempo, cancelar e enviar.
import { Send, Trash } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';

import { duracao } from '../util/formatar';
import { BotaoIcone } from './BotaoIcone';

export const MIME_GRAVACAO = 'audio/webm;codecs=opus';

export function GravadorAudio(props: { aoEnviar: (audio: Blob, segundos: number) => void; aoCancelar: () => void }) {
  const [segundos, setSegundos] = useState(0);
  const [erro, setErro] = useState<string | null>(null);
  const gravador = useRef<MediaRecorder | null>(null);
  const pedacos = useRef<Blob[]>([]);
  const destino = useRef<'enviar' | 'cancelar'>('cancelar');
  const inicio = useRef(0);
  const { aoCancelar } = props;
  // Em ref: o efeito que abre o microfone roda uma vez só, mesmo se o pai renderizar de novo.
  const aoEnviar = useRef(props.aoEnviar);
  aoEnviar.current = props.aoEnviar;

  useEffect(() => {
    let fluxo: MediaStream | null = null;
    let relogio: ReturnType<typeof setInterval> | null = null;
    let cancelado = false;
    navigator.mediaDevices
      .getUserMedia({ audio: { echoCancellation: true, noiseSuppression: true } })
      .then((f) => {
        if (cancelado) {
          f.getTracks().forEach((t) => t.stop());
          return;
        }
        fluxo = f;
        const tipo = MediaRecorder.isTypeSupported(MIME_GRAVACAO) ? MIME_GRAVACAO : 'audio/webm';
        const r = new MediaRecorder(f, { mimeType: tipo, audioBitsPerSecond: 32_000 });
        gravador.current = r;
        r.ondataavailable = (e) => e.data.size > 0 && pedacos.current.push(e.data);
        r.onstop = () => {
          fluxo?.getTracks().forEach((t) => t.stop());
          const total = (Date.now() - inicio.current) / 1000;
          if (destino.current === 'enviar' && pedacos.current.length > 0) {
            aoEnviar.current(new Blob(pedacos.current, { type: 'audio/webm' }), total);
          }
        };
        inicio.current = Date.now();
        r.start(250);
        relogio = setInterval(() => setSegundos((Date.now() - inicio.current) / 1000), 250);
      })
      .catch(() => setErro('Não consegui acessar o microfone. Permita o acesso em Ajustes do Sistema › Privacidade › Microfone.'));
    return () => {
      cancelado = true;
      if (relogio) clearInterval(relogio);
      if (gravador.current && gravador.current.state !== 'inactive') gravador.current.stop();
      fluxo?.getTracks().forEach((t) => t.stop());
    };
  }, []);

  const parar = (acao: 'enviar' | 'cancelar') => {
    destino.current = acao;
    if (gravador.current && gravador.current.state !== 'inactive') gravador.current.stop();
    if (acao === 'cancelar') aoCancelar();
  };

  if (erro) {
    return (
      <div className="gravador" role="alert">
        <span className="gravador-erro">{erro}</span>
        <button type="button" className="botao secundario pequeno" onClick={aoCancelar}>
          Fechar
        </button>
      </div>
    );
  }

  return (
    <div className="gravador" aria-live="polite">
      <BotaoIcone rotulo="Cancelar gravação" onClick={() => parar('cancelar')}>
        <Trash size={20} />
      </BotaoIcone>
      <span className="gravador-ponto" aria-hidden="true" />
      <span className="gravador-tempo" aria-label="Tempo de gravação">
        {duracao(segundos)}
      </span>
      <span className="gravador-texto">Gravando…</span>
      <BotaoIcone rotulo="Enviar áudio" className="botao-enviar" onClick={() => parar('enviar')}>
        <Send size={20} />
      </BotaoIcone>
    </div>
  );
}
