// Campos reutilizados pelos editores de fluxo e de chatbot: duração, etiqueta, funil/etapa,
// template, disparo e automação — com os dados do motor carregados sob demanda.
import { useQuery } from '@tanstack/react-query';
import { useEffect, useState, type ReactNode } from 'react';

import type { ErroDefinicao, Id, TipoAutomacao } from '@zapdesk/cliente-motor';

import { useAutomacoes, useFunis } from '../../api/automacoes';
import { chaves } from '../../api/chaves';
import { useCliente } from '../../api/motor';
import { paraUnidade, SEGUNDOS_UNIDADE, type UnidadeTempo } from '../../util/automacoes';

/** Mensagens de erro de validação do motor para um campo. */
export function ErrosCampo({ erros }: { erros: readonly ErroDefinicao[] }) {
  if (erros.length === 0) return null;
  return (
    <ul className="erros-campo" role="alert">
      {erros.map((e, i) => (
        <li key={`${e.caminho}-${i}`} className="erro-campo">
          {e.mensagem}
        </li>
      ))}
    </ul>
  );
}

export function Campo(props: { rotulo: string; children: ReactNode; dica?: ReactNode; className?: string }) {
  return (
    <label className={`campo${props.className ? ` ${props.className}` : ''}`}>
      <span>{props.rotulo}</span>
      {props.children}
      {props.dica ? <small className="texto-secundario">{props.dica}</small> : null}
    </label>
  );
}

export function CampoDuracao(props: {
  rotulo: string;
  segundos: number;
  aoMudar: (segundos: number) => void;
  unidades?: UnidadeTempo[];
  dica?: ReactNode;
}) {
  // A unidade fica no estado local: apagar o número não pode trocar "horas" por "minutos".
  const [unidade, setUnidade] = useState<UnidadeTempo>(() => paraUnidade(props.segundos).unidade);
  const [texto, setTexto] = useState(() => String(props.segundos / SEGUNDOS_UNIDADE[paraUnidade(props.segundos).unidade]));
  const unidades = props.unidades ?? ['min', 'h', 'd'];
  const valorAtual = Number(texto.replace(',', '.'));
  useEffect(() => {
    // Mudança vinda de fora (ex.: recarregar a automação).
    if (Math.round((Number.isFinite(valorAtual) ? valorAtual : 0) * SEGUNDOS_UNIDADE[unidade]) !== props.segundos) {
      const p = paraUnidade(props.segundos);
      setUnidade(p.unidade);
      setTexto(String(p.valor));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [props.segundos]);
  return (
    <fieldset className="campo">
      <legend>{props.rotulo}</legend>
      <div className="campo-unidade">
        <input
          type="number"
          min={1}
          aria-label={`${props.rotulo} (valor)`}
          value={texto}
          onChange={(e) => {
            setTexto(e.target.value);
            const v = Number(e.target.value);
            props.aoMudar(Number.isFinite(v) ? Math.max(0, Math.round(v * SEGUNDOS_UNIDADE[unidade])) : 0);
          }}
        />
        <select
          aria-label={`${props.rotulo} (unidade)`}
          value={unidade}
          onChange={(e) => {
            const nova = e.target.value as UnidadeTempo;
            setUnidade(nova);
            props.aoMudar(Math.max(0, Math.round((Number.isFinite(valorAtual) ? valorAtual : 0) * SEGUNDOS_UNIDADE[nova])));
          }}
        >
          {unidades.map((u) => (
            <option key={u} value={u}>
              {u === 'min' ? 'minutos' : u === 'h' ? 'horas' : 'dias'}
            </option>
          ))}
        </select>
      </div>
      {props.dica ? <small className="texto-secundario">{props.dica}</small> : null}
    </fieldset>
  );
}

export function SeletorEtiqueta(props: { valor: Id; aoMudar: (id: Id) => void; rotulo?: string }) {
  const cliente = useCliente();
  const etiquetas = useQuery({ queryKey: chaves.etiquetas, queryFn: () => cliente.listarEtiquetas() });
  const lista = etiquetas.data ?? [];
  const quebrada = props.valor && etiquetas.data && !lista.some((e) => e.id === props.valor);
  return (
    <Campo rotulo={props.rotulo ?? 'Etiqueta'}>
      <select value={props.valor} onChange={(e) => props.aoMudar(e.target.value)}>
        <option value="">Escolha…</option>
        {lista.map((e) => (
          <option key={e.id} value={e.id}>
            {e.nome}
          </option>
        ))}
        {quebrada ? <option value={props.valor}>(etiqueta excluída)</option> : null}
      </select>
    </Campo>
  );
}

export function SeletorFunilEtapa(props: {
  funilId: Id;
  etapaId: Id | null;
  aoMudar: (funilId: Id, etapaId: Id | null) => void;
  /** Mostra a opção "Qualquer etapa" (etapa null). */
  qualquerEtapa?: boolean;
  semEtapa?: boolean;
}) {
  const funis = useFunis();
  const lista = [...(funis.data ?? [])].sort((a, b) => a.ordem - b.ordem);
  const funil = lista.find((f) => f.id === props.funilId);
  const etapas = [...(funil?.etapas ?? [])].sort((a, b) => a.ordem - b.ordem);
  return (
    <div className="grade-campos">
      <Campo rotulo="Funil">
        <select value={props.funilId} onChange={(e) => props.aoMudar(e.target.value, props.qualquerEtapa ? null : '')}>
          <option value="">Escolha…</option>
          {lista.map((f) => (
            <option key={f.id} value={f.id}>
              {f.nome}
            </option>
          ))}
          {props.funilId && funis.data && !funil ? <option value={props.funilId}>(funil excluído)</option> : null}
        </select>
      </Campo>
      {props.semEtapa ? null : (
        <Campo rotulo="Etapa">
          <select value={props.etapaId ?? ''} onChange={(e) => props.aoMudar(props.funilId, e.target.value || (props.qualquerEtapa ? null : ''))}>
            <option value="">{props.qualquerEtapa ? 'Qualquer etapa' : 'Escolha…'}</option>
            {etapas.map((e) => (
              <option key={e.id} value={e.id}>
                {e.nome}
              </option>
            ))}
            {props.etapaId && funil && !etapas.some((e) => e.id === props.etapaId) ? <option value={props.etapaId}>(etapa excluída)</option> : null}
          </select>
        </Campo>
      )}
    </div>
  );
}

export function SeletorTemplate(props: { valor: Id; aoMudar: (id: Id) => void }) {
  const cliente = useCliente();
  const templates = useQuery({ queryKey: chaves.templates, queryFn: () => cliente.listarTemplates() });
  return (
    <Campo rotulo="Template">
      <select value={props.valor} onChange={(e) => props.aoMudar(e.target.value)}>
        <option value="">Escolha…</option>
        {(templates.data ?? []).map((t) => (
          <option key={t.id} value={t.id}>
            {t.nome}
          </option>
        ))}
        {props.valor && templates.data && !templates.data.some((t) => t.id === props.valor) ? (
          <option value={props.valor}>(template excluído)</option>
        ) : null}
      </select>
    </Campo>
  );
}

export function useTemplate(id: Id) {
  const cliente = useCliente();
  const templates = useQuery({ queryKey: chaves.templates, queryFn: () => cliente.listarTemplates() });
  return templates.data?.find((t) => t.id === id) ?? null;
}

export function SeletorDisparo(props: { valor: Id | null; aoMudar: (id: Id | null) => void; qualquer?: boolean; soAbertos?: boolean }) {
  const cliente = useCliente();
  const disparos = useQuery({ queryKey: [...chaves.disparos, 'seletor'], queryFn: () => cliente.listarDisparos({ limite: 100 }) });
  const lista = (disparos.data?.itens ?? []).filter((d) => !props.soAbertos || (d.estado !== 'concluido' && d.estado !== 'cancelado'));
  return (
    <Campo rotulo="Disparo">
      <select value={props.valor ?? ''} onChange={(e) => props.aoMudar(e.target.value || null)}>
        <option value="">{props.qualquer ? 'Qualquer disparo' : 'Escolha…'}</option>
        {lista.map((d) => (
          <option key={d.id} value={d.id}>
            {d.nome}
          </option>
        ))}
      </select>
    </Campo>
  );
}

export function SeletorAutomacao(props: { tipo: TipoAutomacao; valor: Id; aoMudar: (id: Id) => void; rotulo: string; excluir?: Id }) {
  const automacoes = useAutomacoes({ tipo: props.tipo });
  const lista = (automacoes.data ?? []).filter((a) => a.id !== props.excluir);
  return (
    <Campo rotulo={props.rotulo}>
      <select value={props.valor} onChange={(e) => props.aoMudar(e.target.value)}>
        <option value="">Escolha…</option>
        {lista.map((a) => (
          <option key={a.id} value={a.id}>
            {a.nome}
            {a.ativa ? '' : ' (inativa)'}
          </option>
        ))}
        {props.valor && automacoes.data && !lista.some((a) => a.id === props.valor) ? <option value={props.valor}>(automação excluída)</option> : null}
      </select>
    </Campo>
  );
}
