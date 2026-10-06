// Erro de carregamento dentro de uma tela, com "Tentar de novo".
import { CircleAlert } from 'lucide-react';

import { textoErro } from '../util/formatar';

export function TelaErro(props: { erro: unknown; titulo?: string; aoTentar?: () => void }) {
  return (
    <div className="tela-erro" role="alert">
      <CircleAlert size={36} aria-hidden="true" />
      <h2>{props.titulo ?? 'Não foi possível carregar.'}</h2>
      <p>{textoErro(props.erro)}</p>
      {props.aoTentar ? (
        <button type="button" className="botao" onClick={props.aoTentar}>
          Tentar de novo
        </button>
      ) : null}
    </div>
  );
}
