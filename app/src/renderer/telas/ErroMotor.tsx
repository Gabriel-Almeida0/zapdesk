// Motor não subiu (ou caiu duas vezes): motivo + "Tentar de novo" (reinicia o motor via IPC).
import { CircleAlert } from 'lucide-react';
import { useState } from 'react';

export function ErroMotorTela(props: { mensagem: string; detalhe: string; aoTentar: () => Promise<void> }) {
  const [tentando, setTentando] = useState(false);
  return (
    <main className="tela-cheia erro-motor" role="alert">
      <CircleAlert size={56} className="icone-erro" aria-hidden="true" />
      <h1>{props.mensagem}</h1>
      <p className="detalhe">{props.detalhe}</p>
      <button
        type="button"
        className="botao"
        disabled={tentando}
        onClick={() => {
          setTentando(true);
          void props.aoTentar().finally(() => setTentando(false));
        }}
      >
        {tentando ? 'Tentando…' : 'Tentar de novo'}
      </button>
    </main>
  );
}
