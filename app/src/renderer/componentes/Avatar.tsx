// Avatar com iniciais neutras (sem foto de perfil no MVP). Cores só por token (.avatar em shell.css);
// `chave` fica na API para quem já passa um id estável.
import { User, Users } from 'lucide-react';

import { iniciais } from '../util/formatar';

export function Avatar(props: { nome: string | null | undefined; chave?: string; tamanho?: number; grupo?: boolean }) {
  const tamanho = props.tamanho ?? 49;
  return (
    <span
      className="avatar"
      aria-hidden="true"
      style={{
        width: tamanho,
        height: tamanho,
        fontSize: Math.max(11, Math.round(tamanho * 0.32)),
      }}
    >
      {props.grupo ? (
        <Users size={Math.round(tamanho * 0.45)} strokeWidth={1.75} />
      ) : /\p{L}/u.test(props.nome ?? '') ? (
        iniciais(props.nome)
      ) : (
        <User size={Math.round(tamanho * 0.45)} strokeWidth={1.75} />
      )}
    </span>
  );
}
