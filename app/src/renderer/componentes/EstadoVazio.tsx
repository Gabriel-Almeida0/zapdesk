// Estado vazio padrão: ícone, título, texto e ação opcional.
import type { ReactNode } from 'react';

export function EstadoVazio(props: { icone?: ReactNode; titulo: string; texto?: ReactNode; acao?: ReactNode }) {
  return (
    <div className="estado-vazio">
      {props.icone ? <div className="estado-vazio-icone">{props.icone}</div> : null}
      <h2>{props.titulo}</h2>
      {props.texto ? <p>{props.texto}</p> : null}
      {props.acao ? <div className="estado-vazio-acao">{props.acao}</div> : null}
    </div>
  );
}
