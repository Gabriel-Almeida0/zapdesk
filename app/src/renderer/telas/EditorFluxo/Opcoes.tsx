// Opções comuns de fluxo e chatbot: contas, grupos, prioridade, conta de envio e anti-loop próprio.
import type { Id, LimiteAntiLoop } from '@zapdesk/cliente-motor';

import { useContaAtual } from '../../estado/conta';
import { Campo } from './campos';

export interface ValoresOpcoes {
  contas: Id[] | null;
  incluir_grupos: boolean;
  prioridade: number;
  conta_envio_id: Id | null;
  anti_loop: LimiteAntiLoop | null;
}

export function OpcoesAutomacao({ valor, aoMudar }: { valor: ValoresOpcoes; aoMudar: (v: ValoresOpcoes) => void }) {
  const { contas } = useContaAtual();
  const todas = valor.contas === null;
  return (
    <section className="secao-editor" aria-labelledby="titulo-opcoes">
      <h2 id="titulo-opcoes">Opções</h2>
      <fieldset className="campo">
        <legend>Vale nas contas</legend>
        <label className="caixa">
          <input type="checkbox" checked={todas} onChange={(e) => aoMudar({ ...valor, contas: e.target.checked ? null : contas.map((c) => c.id) })} />
          Todas as contas
        </label>
        {todas
          ? null
          : contas.map((c) => (
              <label key={c.id} className="caixa">
                <input
                  type="checkbox"
                  checked={(valor.contas ?? []).includes(c.id)}
                  onChange={(e) => {
                    const atuais = valor.contas ?? [];
                    aoMudar({ ...valor, contas: e.target.checked ? [...atuais, c.id] : atuais.filter((x) => x !== c.id) });
                  }}
                />
                {c.nome}
              </label>
            ))}
      </fieldset>
      <label className="caixa">
        <input type="checkbox" checked={valor.incluir_grupos} onChange={(e) => aoMudar({ ...valor, incluir_grupos: e.target.checked })} />
        Incluir grupos (desligado por padrão)
      </label>
      <div className="grade-campos">
        <Campo rotulo="Prioridade" dica="1 a 1.000; menor roda primeiro.">
          <input
            type="number"
            min={1}
            max={1000}
            value={valor.prioridade}
            onChange={(e) => aoMudar({ ...valor, prioridade: Math.min(1000, Math.max(1, Math.round(Number(e.target.value) || 1))) })}
          />
        </Campo>
        <Campo rotulo="Conta de envio" dica="Usada quando o evento não tem conversa (ex.: lead importado).">
          <select value={valor.conta_envio_id ?? ''} onChange={(e) => aoMudar({ ...valor, conta_envio_id: e.target.value || null })}>
            <option value="">Automática</option>
            {contas.map((c) => (
              <option key={c.id} value={c.id}>
                {c.nome}
              </option>
            ))}
          </select>
        </Campo>
      </div>
      <fieldset className="campo">
        <legend>Anti-loop próprio (só mais restritivo que o de Ajustes)</legend>
        <label className="caixa">
          <input
            type="checkbox"
            checked={valor.anti_loop !== null}
            onChange={(e) => aoMudar({ ...valor, anti_loop: e.target.checked ? { mensagens: 5, janela_min: 10 } : null })}
          />
          Limitar esta automação
        </label>
        {valor.anti_loop ? (
          <div className="opcoes-radio">
            <label className="caixa">
              No máximo
              <input
                type="number"
                min={1}
                max={100}
                className="numero-curto"
                aria-label="Mensagens automáticas"
                value={valor.anti_loop.mensagens}
                onChange={(e) => aoMudar({ ...valor, anti_loop: { mensagens: Math.max(1, Number(e.target.value) || 1), janela_min: valor.anti_loop?.janela_min ?? 10 } })}
              />
              mensagens a cada
              <input
                type="number"
                min={1}
                max={1440}
                className="numero-curto"
                aria-label="Janela em minutos"
                value={valor.anti_loop.janela_min}
                onChange={(e) => aoMudar({ ...valor, anti_loop: { mensagens: valor.anti_loop?.mensagens ?? 5, janela_min: Math.max(1, Number(e.target.value) || 1) } })}
              />
              min por conversa
            </label>
          </div>
        ) : null}
      </fieldset>
    </section>
  );
}
