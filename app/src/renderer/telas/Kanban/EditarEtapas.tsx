// Editor de etapas (T069): nome, cor e ordem; nova etapa; excluir etapa com cards pede o destino
// (outra etapa ou remover do funil).
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { ArrowDown, ArrowUp, Plus, Trash } from 'lucide-react';
import { useState } from 'react';

import type { Etapa, Funil, OpcoesExcluirEtapa } from '@zapdesk/cliente-motor';

import { chaves } from '../../api/chaves';
import { useCliente } from '../../api/motor';
import { BotaoIcone } from '../../componentes/BotaoIcone';
import { FaixaAviso } from '../../componentes/FaixaAviso';
import { Modal } from '../../componentes/Modal';
import { numero, textoErro } from '../../util/formatar';
import { COR_ETAPA_PADRAO, CORES_ETAPA } from '../Funis';

const REMOVER = '__remover__';

function ExcluirEtapa(props: { funil: Funil; etapa: Etapa; aoFechar: () => void }) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const outras = props.funil.etapas.filter((e) => e.id !== props.etapa.id).sort((a, b) => a.ordem - b.ordem);
  const [destino, setDestino] = useState<string>(outras[0]?.id ?? REMOVER);
  const temCards = props.etapa.total_cards > 0;
  const excluir = useMutation({
    mutationFn: () => {
      const opcoes: OpcoesExcluirEtapa = !temCards ? {} : destino === REMOVER ? { remover_cards: true } : { destino_etapa_id: destino };
      return cliente.excluirEtapa(props.etapa.id, opcoes);
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: chaves.funil(props.funil.id) });
      void qc.invalidateQueries({ queryKey: chaves.cards(props.funil.id) });
      void qc.invalidateQueries({ queryKey: chaves.funis });
      props.aoFechar();
    },
  });
  return (
    <Modal
      titulo={`Excluir a etapa "${props.etapa.nome}"?`}
      aoFechar={props.aoFechar}
      rodape={
        <>
          <button type="button" className="botao secundario" onClick={props.aoFechar}>
            Cancelar
          </button>
          <button type="button" className="botao perigo" disabled={excluir.isPending} onClick={() => excluir.mutate()}>
            Excluir etapa
          </button>
        </>
      }
    >
      {temCards ? (
        <label className="campo">
          <span>
            Esta etapa tem {numero(props.etapa.total_cards)} {props.etapa.total_cards === 1 ? 'lead' : 'leads'}. Para onde eles vão?
          </span>
          <select aria-label="Destino dos leads" value={destino} onChange={(e) => setDestino(e.target.value)}>
            {outras.map((e) => (
              <option key={e.id} value={e.id}>
                Mover para "{e.nome}"
              </option>
            ))}
            <option value={REMOVER}>Remover os leads do funil</option>
          </select>
        </label>
      ) : (
        <p className="texto-modal">A etapa está vazia.</p>
      )}
      <p className="texto-secundario">Automações que usam esta etapa passam a mostrar um aviso.</p>
      {excluir.error ? <FaixaAviso tipo="erro">{textoErro(excluir.error)}</FaixaAviso> : null}
    </Modal>
  );
}

function LinhaEtapa(props: { funil: Funil; etapa: Etapa; indice: number; total: number; aoMover: (delta: number) => void; aoExcluir: () => void; ocupado: boolean }) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const [nome, setNome] = useState(props.etapa.nome);
  const editar = useMutation({
    mutationFn: (dados: { nome?: string; cor?: string }) => cliente.editarEtapa(props.etapa.id, dados),
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: chaves.funil(props.funil.id) });
      void qc.invalidateQueries({ queryKey: chaves.funis });
    },
  });
  const salvarNome = () => {
    const n = nome.trim();
    if (n && n !== props.etapa.nome && n.length <= 40) editar.mutate({ nome: n });
    else setNome(props.etapa.nome);
  };
  return (
    <li className="linha-etapa">
      <input
        type="color"
        className="cor-etapa"
        aria-label={`Cor da etapa ${props.etapa.nome}`}
        value={props.etapa.cor}
        onChange={(e) => editar.mutate({ cor: e.target.value })}
      />
      <input
        aria-label={`Nome da etapa ${props.indice + 1}`}
        value={nome}
        maxLength={40}
        onChange={(e) => setNome(e.target.value)}
        onBlur={salvarNome}
        onKeyDown={(e) => {
          if (e.key === 'Enter') (e.target as HTMLInputElement).blur();
        }}
      />
      <small className="texto-secundario">{numero(props.etapa.total_cards)}</small>
      <BotaoIcone rotulo={`Subir ${props.etapa.nome}`} className="pequeno" disabled={props.indice === 0 || props.ocupado} onClick={() => props.aoMover(-1)}>
        <ArrowUp size={14} />
      </BotaoIcone>
      <BotaoIcone rotulo={`Descer ${props.etapa.nome}`} className="pequeno" disabled={props.indice === props.total - 1 || props.ocupado} onClick={() => props.aoMover(1)}>
        <ArrowDown size={14} />
      </BotaoIcone>
      <BotaoIcone rotulo={`Excluir etapa ${props.etapa.nome}`} className="pequeno" onClick={props.aoExcluir}>
        <Trash size={14} />
      </BotaoIcone>
      {editar.error ? <span className="erro-campo">{textoErro(editar.error)}</span> : null}
    </li>
  );
}

export function EditarEtapas({ funil, aoFechar }: { funil: Funil; aoFechar: () => void }) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const etapas = [...funil.etapas].sort((a, b) => a.ordem - b.ordem);
  const [nova, setNova] = useState('');
  const [excluindo, setExcluindo] = useState<Etapa | null>(null);
  const invalidar = () => {
    void qc.invalidateQueries({ queryKey: chaves.funil(funil.id) });
    void qc.invalidateQueries({ queryKey: chaves.funis });
  };
  const reordenar = useMutation({
    mutationFn: (ids: string[]) => cliente.reordenarEtapas(funil.id, ids),
    onSuccess: (f) => qc.setQueryData(chaves.funil(funil.id), f),
    onSettled: invalidar,
  });
  const criar = useMutation({
    mutationFn: () => cliente.criarEtapa(funil.id, { nome: nova.trim(), cor: CORES_ETAPA[etapas.length % CORES_ETAPA.length] ?? COR_ETAPA_PADRAO }),
    onSuccess: () => setNova(''),
    onSettled: invalidar,
  });
  const mover = (i: number, delta: number) => {
    const ids = etapas.map((e) => e.id);
    const j = i + delta;
    const a = ids[i];
    const b = ids[j];
    if (a === undefined || b === undefined) return;
    ids[i] = b;
    ids[j] = a;
    reordenar.mutate(ids);
  };
  const erro = reordenar.error ?? criar.error;

  return (
    <Modal titulo={`Etapas de "${funil.nome}"`} aoFechar={aoFechar} largura={560}>
      <ol className="lista-etapas-edicao">
        {etapas.map((e, i) => (
          <LinhaEtapa
            key={e.id}
            funil={funil}
            etapa={e}
            indice={i}
            total={etapas.length}
            ocupado={reordenar.isPending}
            aoMover={(d) => mover(i, d)}
            aoExcluir={() => setExcluindo(e)}
          />
        ))}
      </ol>
      <form
        className="linha-form"
        onSubmit={(e) => {
          e.preventDefault();
          if (nova.trim()) criar.mutate();
        }}
      >
        <input aria-label="Nova etapa" placeholder="Nova etapa" value={nova} maxLength={40} onChange={(e) => setNova(e.target.value)} />
        <button type="submit" className="botao pequeno" disabled={!nova.trim() || criar.isPending || etapas.length >= 30}>
          <Plus size={14} aria-hidden="true" /> Adicionar
        </button>
      </form>
      {erro ? <FaixaAviso tipo="erro">{textoErro(erro)}</FaixaAviso> : null}
      {excluindo ? <ExcluirEtapa funil={funil} etapa={excluindo} aoFechar={() => setExcluindo(null)} /> : null}
    </Modal>
  );
}
