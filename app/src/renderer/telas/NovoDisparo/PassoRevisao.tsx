// Passo 4 — Revisão (T098): resumo, "5 contatos sem {nome}" com a lista e o campo de valor padrão,
// nome do disparo, "Salvar rascunho" e "Iniciar".
import { TriangleAlert } from 'lucide-react';

import type { Arquivo } from '@zapdesk/cliente-motor';

import { FaixaAviso } from '../../componentes/FaixaAviso';
import { useContaAtual } from '../../estado/conta';
import { faltandoPorVariavel, montarRitmo, ritmoAgressivo, textoJanela, type FormDisparo } from '../../util/disparos';
import { dataHora, numero, telefone, textoErro } from '../../util/formatar';
import { AVISO_RITMO } from './PassoRitmo';
import { useValidacaoDisparo } from './validacao';

/** "5 contatos sem {nome}". */
export function textoFaltando(total: number, variavel: string): string {
  return `${numero(total)} ${total === 1 ? 'contato' : 'contatos'} sem {${variavel}}`;
}

export function PassoRevisao(props: {
  form: FormDisparo;
  arquivo: Arquivo | null;
  aoMudarForm: (parcial: Partial<FormDisparo>) => void;
  erroEnvio: unknown;
}) {
  const { form } = props;
  const { contas } = useContaAtual();
  // Sem valores padrão: a lista de faltando fica completa e o campo não some enquanto se digita.
  const validacao = useValidacaoDisparo({ ...form, valoresPadrao: {} }, true);
  const ritmo = montarRitmo(form.ritmo);
  const faltando = faltandoPorVariavel(validacao.data?.faltando ?? []);
  const conta = contas.find((c) => c.id === form.contaId);

  return (
    <div className="passo">
      <label className="campo">
        <span>Nome do disparo</span>
        <input
          maxLength={80}
          placeholder="Deixe vazio para usar a data e a hora"
          value={form.nome}
          onChange={(e) => props.aoMudarForm({ nome: e.target.value })}
        />
      </label>

      <dl className="lista-definicoes resumo-disparo">
        <dt>Conta</dt>
        <dd>
          {conta?.nome ?? '—'} {conta?.telefone ? `(${telefone(conta.telefone)})` : ''}
        </dd>
        <dt>Destinatários</dt>
        <dd>{numero(validacao.data?.total_destinatarios ?? form.leadIds.length)}</dd>
        <dt>Mensagem</dt>
        <dd className="pre">{form.mensagem}</dd>
        <dt>Anexo</dt>
        <dd>{props.arquivo ? props.arquivo.nome : 'Nenhum'}</dd>
        <dt>Intervalo</dt>
        <dd>
          {ritmo.intervalo_min_s}–{ritmo.intervalo_max_s} s entre mensagens
        </dd>
        <dt>Limites</dt>
        <dd>
          {[
            ritmo.limite_por_hora ? `${ritmo.limite_por_hora}/hora` : null,
            ritmo.limite_por_dia ? `${ritmo.limite_por_dia}/dia` : null,
            ritmo.pausa_a_cada && ritmo.pausa_duracao_s
              ? `pausa de ${Math.round(ritmo.pausa_duracao_s / 60)} min a cada ${ritmo.pausa_a_cada}`
              : null,
          ]
            .filter(Boolean)
            .join(' · ') || 'Sem limites'}
        </dd>
        <dt>Início</dt>
        <dd>{form.agendar && form.inicioEm ? dataHora(new Date(form.inicioEm).toISOString()) : 'Agora'}</dd>
        <dt>Janela</dt>
        <dd>{form.usarJanela ? textoJanela({ inicio: form.janelaInicio, fim: form.janelaFim }) : 'Qualquer horário'}</dd>
        <dt>Término estimado</dt>
        <dd>{validacao.data?.estimativa_termino_em ? dataHora(validacao.data.estimativa_termino_em) : '—'}</dd>
      </dl>

      {ritmoAgressivo(ritmo) ? (
        <FaixaAviso tipo="aviso" icone={<TriangleAlert size={18} />}>
          {AVISO_RITMO}
        </FaixaAviso>
      ) : null}

      {validacao.data?.previa ? (
        <div className="previa-chat">
          <small className="texto-secundario">Exemplo para {telefone(validacao.data.previa.telefone)}</small>
          <div className="bolha previa-bolha">
            <span className="texto-mensagem">{validacao.data.previa.texto_resolvido}</span>
          </div>
        </div>
      ) : null}

      {[...faltando.entries()].map(([variavel, linhas]) => (
        <div key={variavel} className="faltando">
          <strong>{textoFaltando(linhas.length, variavel)}</strong>
          {form.valoresPadrao[variavel]?.trim() ? (
            <span className="selo ok">Resolvido com o valor padrão</span>
          ) : (
            <span className="texto-secundario">Defina um valor padrão ou corrija esses leads para poder iniciar.</span>
          )}
          <details>
            <summary>Ver linhas</summary>
            <ul className="lista-faltando">
              {linhas.slice(0, 200).map((l) => (
                <li key={`${l.linha}-${l.telefone}`}>
                  Linha {l.linha}: {telefone(l.telefone)}
                </li>
              ))}
            </ul>
          </details>
          <label className="campo">
            <span>Valor padrão para {`{${variavel}}`}</span>
            <input
              placeholder="Ex.: tudo bem"
              value={form.valoresPadrao[variavel] ?? ''}
              onChange={(e) => props.aoMudarForm({ valoresPadrao: { ...form.valoresPadrao, [variavel]: e.target.value } })}
            />
          </label>
        </div>
      ))}

      {validacao.data && Object.keys(validacao.data.erros_campos).length > 0 ? (
        <FaixaAviso tipo="erro">{Object.values(validacao.data.erros_campos).join(' ')}</FaixaAviso>
      ) : null}
      {props.erroEnvio ? <FaixaAviso tipo="erro">{textoErro(props.erroEnvio)}</FaixaAviso> : null}
    </div>
  );
}
