// "Ligando o WhatsApp…" enquanto o motor sobe (e "Religando…" após queda).
// 003 (T038): logotipo da landing (tecla + "zapdesk"); o <h1> continua para leitores de tela.
import { Logo } from '../componentes/Logo';

export function Carregando({ texto }: { texto: string }) {
  return (
    <main className="tela-cheia carregando" aria-busy="true">
      <div className="logo-grande" aria-hidden="true">
        <Logo variante="completo" tamanho={56} />
      </div>
      <h1 className="so-leitor">ZapDesk</h1>
      <div className="barra-indeterminada" aria-hidden="true">
        <span />
      </div>
      <p role="status">{texto}</p>
      <p className="rodape-seguro">Suas mensagens ficam só neste Mac.</p>
    </main>
  );
}
