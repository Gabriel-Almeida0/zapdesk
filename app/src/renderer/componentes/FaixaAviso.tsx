// Faixa de aviso no topo de um painel (amarela, vermelha ou informativa).
import type { ReactNode } from 'react';

export type TipoFaixa = 'aviso' | 'erro' | 'info';

export function FaixaAviso(props: { tipo?: TipoFaixa; icone?: ReactNode; children: ReactNode; acao?: ReactNode }) {
  const tipo = props.tipo ?? 'aviso';
  return (
    <div className={`faixa faixa-${tipo}`} role={tipo === 'erro' ? 'alert' : 'status'}>
      {props.icone ? <span className="faixa-icone">{props.icone}</span> : null}
      <span className="faixa-texto">{props.children}</span>
      {props.acao ? <span className="faixa-acao">{props.acao}</span> : null}
    </div>
  );
}
