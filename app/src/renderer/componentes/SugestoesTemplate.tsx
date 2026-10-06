// Sugestões do atalho `/` no composer (T125): lista de templates filtrada pelo que vem depois da
// barra; setas navegam, Enter/Tab inserem.
import { FileText, Paperclip } from 'lucide-react';

import type { Template } from '@zapdesk/cliente-motor';

/** `"/apre"` → `"apre"`; `null` se o texto não é um atalho de template. */
export function termoAtalho(texto: string): string | null {
  const m = /^\/(\S*)$/.exec(texto);
  return m ? (m[1] ?? '') : null;
}

export function SugestoesTemplate(props: {
  templates: Template[];
  indice: number;
  carregando: boolean;
  aoEscolher: (t: Template) => void;
  aoPassar: (indice: number) => void;
}) {
  return (
    <div className="sugestoes-template" role="listbox" aria-label="Templates">
      {props.carregando && props.templates.length === 0 ? (
        <div className="sugestao-vazia">Carregando templates…</div>
      ) : props.templates.length === 0 ? (
        <div className="sugestao-vazia">Nenhum template encontrado. Crie em Templates.</div>
      ) : (
        props.templates.map((t, i) => (
          <button
            key={t.id}
            type="button"
            role="option"
            aria-selected={i === props.indice}
            className={`sugestao${i === props.indice ? ' ativa' : ''}`}
            onMouseEnter={() => props.aoPassar(i)}
            onMouseDown={(e) => {
              e.preventDefault();
              props.aoEscolher(t);
            }}
          >
            <FileText size={16} aria-hidden="true" />
            <span className="sugestao-textos">
              <strong>/{t.nome}</strong>
              <span>{t.texto}</span>
            </span>
            {t.arquivo ? <Paperclip size={14} aria-label="Com anexo" /> : null}
          </button>
        ))
      )}
    </div>
  );
}
