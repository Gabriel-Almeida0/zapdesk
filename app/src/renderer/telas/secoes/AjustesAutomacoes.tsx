// Ajustes → Automações (T083): limites de data-model.md e "Pausar todas as automações".
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';

import type { ConfiguracaoAutomacoes } from '@zapdesk/cliente-motor';

import { useConfiguracaoAutomacoes } from '../../api/automacoes';
import { chaves } from '../../api/chaves';
import { useCliente } from '../../api/motor';
import { Esqueleto } from '../../componentes/Esqueleto';
import { FaixaAviso } from '../../componentes/FaixaAviso';
import { textoErro } from '../../util/formatar';

type CampoNumerico = Exclude<keyof ConfiguracaoAutomacoes, 'pausa_geral'>;

export const FAIXAS: Record<CampoNumerico, { min: number; max: number; rotulo: string; unidade: string }> = {
  anti_loop_mensagens: { min: 1, max: 100, rotulo: 'Anti-loop: mensagens automáticas por conversa', unidade: 'mensagens' },
  anti_loop_janela_min: { min: 1, max: 1440, rotulo: 'Anti-loop: janela', unidade: 'min' },
  pausa_anti_loop_min: { min: 1, max: 10080, rotulo: 'Pausa ao estourar o anti-loop', unidade: 'min' },
  pausa_humana_min: { min: 1, max: 10080, rotulo: 'Atendimento humano (quando você responde à mão)', unidade: 'min' },
  primeiros_contatos_hora: { min: 0, max: 500, rotulo: 'Primeiros contatos automáticos por conta', unidade: 'por hora' },
  tempo_ia_s: { min: 5, max: 300, rotulo: 'Tempo limite de uma execução de IA', unidade: 's' },
  memoria_ia_mb: { min: 64, max: 2048, rotulo: 'Memória de cada automação de IA', unidade: 'MB' },
  processos_ia_max: { min: 1, max: 16, rotulo: 'Automações de IA rodando ao mesmo tempo', unidade: 'processos' },
  ociosidade_ia_min: { min: 1, max: 60, rotulo: 'Encerrar automação de IA ociosa após', unidade: 'min' },
};

export function validarConfiguracao(c: ConfiguracaoAutomacoes): Partial<Record<CampoNumerico, string>> {
  const erros: Partial<Record<CampoNumerico, string>> = {};
  for (const [campo, f] of Object.entries(FAIXAS) as [CampoNumerico, (typeof FAIXAS)[CampoNumerico]][]) {
    const v = c[campo];
    if (!Number.isInteger(v) || v < f.min || v > f.max) erros[campo] = `Use um valor de ${f.min} a ${f.max}.`;
  }
  return erros;
}

export function AjustesAutomacoes() {
  const cliente = useCliente();
  const qc = useQueryClient();
  const consulta = useConfiguracaoAutomacoes();
  const [valores, setValores] = useState<ConfiguracaoAutomacoes | null>(null);
  useEffect(() => {
    if (consulta.data) setValores(consulta.data);
  }, [consulta.data]);
  const salvar = useMutation({
    mutationFn: (dados: Partial<ConfiguracaoAutomacoes>) => cliente.editarConfiguracaoAutomacoes(dados),
    onSuccess: (c) => qc.setQueryData(chaves.configuracaoAutomacoes, c),
  });

  if (consulta.isPending) return <Esqueleto altura={160} />;
  if (consulta.isError || !valores) return <FaixaAviso tipo="erro">{textoErro(consulta.error)}</FaixaAviso>;
  const erros = validarConfiguracao(valores);
  const sujo = JSON.stringify(valores) !== JSON.stringify(consulta.data);

  return (
    <>
      <label className="caixa interruptor-geral">
        <input
          type="checkbox"
          role="switch"
          checked={valores.pausa_geral}
          disabled={salvar.isPending}
          onChange={(e) => {
            setValores({ ...valores, pausa_geral: e.target.checked });
            salvar.mutate({ pausa_geral: e.target.checked });
          }}
        />
        <strong>Pausar todas as automações</strong>
      </label>
      {valores.pausa_geral ? <FaixaAviso tipo="aviso">Nenhuma automação age enquanto estiver pausado (esperas que vencerem são abortadas).</FaixaAviso> : null}
      <form
        className="formulario"
        onSubmit={(e) => {
          e.preventDefault();
          if (Object.keys(erros).length === 0) {
            const { pausa_geral: _p, ...numeros } = valores;
            void _p;
            salvar.mutate(numeros);
          }
        }}
      >
        <div className="grade-campos grade-ajustes">
          {(Object.keys(FAIXAS) as CampoNumerico[]).map((campo) => {
            const f = FAIXAS[campo];
            return (
              <label key={campo} className="campo">
                <span>{f.rotulo}</span>
                <span className="campo-unidade">
                  <input
                    type="number"
                    min={f.min}
                    max={f.max}
                    aria-invalid={erros[campo] ? true : undefined}
                    value={valores[campo]}
                    onChange={(e) => setValores({ ...valores, [campo]: Math.round(Number(e.target.value)) })}
                  />
                  <span>{f.unidade}</span>
                </span>
                {erros[campo] ? <small className="erro-campo">{erros[campo]}</small> : null}
              </label>
            );
          })}
        </div>
        <p className="texto-secundario">
          O anti-loop conta todas as mensagens automáticas (fluxos, chatbots e IA). Uma automação pode ter um limite próprio, só mais
          restritivo.
        </p>
        {salvar.error ? <FaixaAviso tipo="erro">{textoErro(salvar.error)}</FaixaAviso> : null}
        <div className="acoes-formulario">
          {salvar.isSuccess && !sujo ? <span className="salvo">Salvo</span> : null}
          <button type="submit" className="botao" disabled={!sujo || Object.keys(erros).length > 0 || salvar.isPending}>
            Salvar limites
          </button>
        </div>
      </form>
    </>
  );
}
