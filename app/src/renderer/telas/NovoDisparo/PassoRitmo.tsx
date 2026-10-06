// Passo 3 — Ritmo e agendamento (T097): intervalo mín./máx., limites por hora/dia, pausa a cada N,
// início, janela, falhas seguidas (padrão 10; 0 desliga), aviso de risco de banimento (sem
// bloquear) e estimativa de término.
import { TriangleAlert } from 'lucide-react';
import type { ReactNode } from 'react';

import { FaixaAviso } from '../../componentes/FaixaAviso';
import { montarRitmo, ritmoAgressivo, validarRitmoForm, type FormDisparo, type FormRitmo } from '../../util/disparos';
import { dataHora } from '../../util/formatar';
import { useValidacaoDisparo } from './validacao';

export const AVISO_RITMO =
  'Ritmo agressivo: intervalos curtos, limites altos ou sem limite aumentam o risco de o WhatsApp bloquear este número. Você pode continuar mesmo assim.';

function CampoNumero(props: {
  rotulo: string;
  valor: string;
  aoMudar: (v: string) => void;
  unidade?: string;
  erro?: string | undefined;
  opcional?: boolean;
  dica?: ReactNode;
}) {
  return (
    <label className="campo">
      <span>
        {props.rotulo}
        {props.opcional ? <small className="texto-secundario"> (opcional)</small> : null}
      </span>
      <span className="campo-unidade">
        <input
          type="number"
          inputMode="numeric"
          min={0}
          step={1}
          value={props.valor}
          aria-invalid={Boolean(props.erro)}
          onChange={(e) => props.aoMudar(e.target.value)}
        />
        {props.unidade ? <span>{props.unidade}</span> : null}
      </span>
      {props.erro ? <small className="erro-campo">{props.erro}</small> : props.dica ? <small className="texto-secundario">{props.dica}</small> : null}
    </label>
  );
}

export function PassoRitmo(props: { form: FormDisparo; aoMudarForm: (parcial: Partial<FormDisparo>) => void }) {
  const { form } = props;
  const erros = validarRitmoForm(form);
  const validacao = useValidacaoDisparo(form, Object.keys(erros).length === 0 && form.mensagem.trim().length > 0);
  const ritmo = (parcial: Partial<FormRitmo>) => props.aoMudarForm({ ritmo: { ...form.ritmo, ...parcial } });
  const agressivo = Object.keys(erros).length === 0 && ritmoAgressivo(montarRitmo(form.ritmo));

  return (
    <div className="passo">
      <fieldset className="grupo-campos">
        <legend>Intervalo entre mensagens (sorteado a cada envio)</legend>
        <div className="grade-campos">
          <CampoNumero rotulo="Mínimo" unidade="segundos" valor={form.ritmo.intervaloMin} erro={erros['intervalo_min_s']} aoMudar={(v) => ritmo({ intervaloMin: v })} />
          <CampoNumero rotulo="Máximo" unidade="segundos" valor={form.ritmo.intervaloMax} erro={erros['intervalo_max_s']} aoMudar={(v) => ritmo({ intervaloMax: v })} />
        </div>
      </fieldset>

      <fieldset className="grupo-campos">
        <legend>Limites</legend>
        <div className="grade-campos">
          <CampoNumero rotulo="Por hora" opcional unidade="mensagens" valor={form.ritmo.limiteHora} erro={erros['limite_por_hora']} aoMudar={(v) => ritmo({ limiteHora: v })} dica="Vale para qualquer período de 60 minutos." />
          <CampoNumero rotulo="Por dia" opcional unidade="mensagens" valor={form.ritmo.limiteDia} erro={erros['limite_por_dia']} aoMudar={(v) => ritmo({ limiteDia: v })} dica="Zera à meia-noite." />
          <CampoNumero rotulo="Pausar a cada" opcional unidade="mensagens" valor={form.ritmo.pausaACada} erro={erros['pausa_a_cada']} aoMudar={(v) => ritmo({ pausaACada: v })} />
          <CampoNumero rotulo="Por quanto tempo" opcional unidade="minutos" valor={form.ritmo.pausaDuracaoMin} erro={erros['pausa_duracao_s']} aoMudar={(v) => ritmo({ pausaDuracaoMin: v })} />
        </div>
      </fieldset>

      <fieldset className="grupo-campos">
        <legend>Quando enviar</legend>
        <div className="opcoes-radio">
          <label className="caixa">
            <input type="radio" name="inicio" checked={!form.agendar} onChange={() => props.aoMudarForm({ agendar: false })} />
            Começar agora
          </label>
          <label className="caixa">
            <input type="radio" name="inicio" checked={form.agendar} onChange={() => props.aoMudarForm({ agendar: true })} />
            Agendar início
          </label>
          {form.agendar ? (
            <input
              type="datetime-local"
              aria-label="Data e hora de início"
              value={form.inicioEm}
              aria-invalid={Boolean(erros['inicio_em'])}
              onChange={(e) => props.aoMudarForm({ inicioEm: e.target.value })}
            />
          ) : null}
        </div>
        {erros['inicio_em'] ? <small className="erro-campo">{erros['inicio_em']}</small> : null}
        <label className="caixa">
          <input type="checkbox" checked={form.usarJanela} onChange={(e) => props.aoMudarForm({ usarJanela: e.target.checked })} />
          Só enviar dentro de uma janela de horário
        </label>
        {form.usarJanela ? (
          <div className="grade-campos">
            <label className="campo">
              <span>Das</span>
              <input type="time" value={form.janelaInicio} onChange={(e) => props.aoMudarForm({ janelaInicio: e.target.value })} />
            </label>
            <label className="campo">
              <span>Até</span>
              <input type="time" value={form.janelaFim} onChange={(e) => props.aoMudarForm({ janelaFim: e.target.value })} />
            </label>
          </div>
        ) : null}
        {erros['janela'] ? <small className="erro-campo">{erros['janela']}</small> : null}
      </fieldset>

      <fieldset className="grupo-campos">
        <legend>Segurança</legend>
        <CampoNumero
          rotulo="Pausar após falhas seguidas"
          unidade="falhas"
          valor={form.falhasSeguidas}
          erro={erros['falhas_seguidas_max']}
          aoMudar={(v) => props.aoMudarForm({ falhasSeguidas: v })}
          dica={'0 desliga. "Sem WhatsApp" não conta; qualquer envio bem-sucedido zera a contagem.'}
        />
      </fieldset>

      {agressivo || validacao.data?.aviso_ritmo_agressivo ? (
        <FaixaAviso tipo="aviso" icone={<TriangleAlert size={18} />}>
          {AVISO_RITMO}
        </FaixaAviso>
      ) : null}
      <p className="estimativa" aria-live="polite">
        {validacao.data?.estimativa_termino_em
          ? (
              <>
                Término estimado: <span className="estimativa-valor">{dataHora(validacao.data.estimativa_termino_em)}</span>
              </>
            )
          : validacao.isFetching
            ? 'Calculando estimativa…'
            : ''}
      </p>
    </div>
  );
}
