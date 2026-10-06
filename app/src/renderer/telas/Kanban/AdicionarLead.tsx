// Adicionar à etapa (T068): um lead existente, um contato (vira lead com origem "contatos") ou um
// telefone. Se o lead já está no funil, o motor o move (nunca duplica).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import type { Etapa, Id, PedidoCard } from '@zapdesk/cliente-motor';

import { chaves } from '../../api/chaves';
import { useCliente } from '../../api/motor';
import { FaixaAviso } from '../../componentes/FaixaAviso';
import { Modal } from '../../componentes/Modal';
import { useContaAtual } from '../../estado/conta';
import { nomeExibicao, telefone, textoErro } from '../../util/formatar';
import { useAtraso } from '../Conversas';

type Aba = 'lead' | 'contato' | 'telefone';

export function AdicionarLead(props: { funilId: Id; etapa: Etapa; aoFechar: () => void }) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const { contaId } = useContaAtual();
  const [aba, setAba] = useState<Aba>('lead');
  const [busca, setBusca] = useState('');
  const [numeroTel, setNumeroTel] = useState('');
  const termo = useAtraso(busca.trim(), 250);

  const leads = useQuery({
    queryKey: ['adicionar-lead', 'leads', termo],
    queryFn: () => cliente.listarLeads({ limite: 20, ...(termo ? { busca: termo } : {}) }),
    enabled: aba === 'lead',
  });
  const contatos = useQuery({
    queryKey: ['adicionar-lead', 'contatos', contaId, termo],
    queryFn: () => cliente.listarContatos(contaId ?? '', { limite: 20, ...(termo ? { busca: termo } : {}) }),
    enabled: aba === 'contato' && Boolean(contaId),
  });

  const adicionar = useMutation({
    mutationFn: (alvo: { lead_id: Id } | { contato_id: Id } | { telefone: string }) =>
      cliente.moverCard(props.funilId, { etapa_id: props.etapa.id, origem: 'app', ...alvo } as PedidoCard),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: chaves.cards(props.funilId) });
      void qc.invalidateQueries({ queryKey: chaves.funil(props.funilId) });
      void qc.invalidateQueries({ queryKey: chaves.funis });
      props.aoFechar();
    },
  });

  return (
    <Modal titulo={`Adicionar em "${props.etapa.nome}"`} aoFechar={props.aoFechar} largura={500}>
      <div className="abas" role="tablist">
        {(
          [
            ['lead', 'Lead'],
            ['contato', 'Contato'],
            ['telefone', 'Telefone'],
          ] as const
        ).map(([id, rotulo]) => (
          <button key={id} type="button" role="tab" aria-selected={aba === id} className={`aba${aba === id ? ' ativa' : ''}`} onClick={() => setAba(id)}>
            {rotulo}
          </button>
        ))}
      </div>
      {aba === 'telefone' ? (
        <form
          className="formulario"
          onSubmit={(e) => {
            e.preventDefault();
            if (numeroTel.trim()) adicionar.mutate({ telefone: numeroTel.trim() });
          }}
        >
          <label className="campo">
            <span>Telefone</span>
            <input inputMode="tel" placeholder="(11) 99999-0000" value={numeroTel} onChange={(e) => setNumeroTel(e.target.value)} />
            <small>Se ainda não for lead, o ZapDesk cria o lead.</small>
          </label>
          <div className="acoes-formulario">
            <button type="submit" className="botao" disabled={!numeroTel.trim() || adicionar.isPending}>
              Adicionar
            </button>
          </div>
        </form>
      ) : (
        <>
          <label className="campo">
            <span>{aba === 'lead' ? 'Buscar lead' : 'Buscar contato'}</span>
            <input type="search" value={busca} placeholder="Nome ou telefone" onChange={(e) => setBusca(e.target.value)} />
          </label>
          <ul className="lista-escolha alta" aria-label={aba === 'lead' ? 'Leads' : 'Contatos'}>
            {aba === 'lead'
              ? (leads.data?.itens ?? []).map((l) => (
                  <li key={l.id}>
                    <button type="button" className="item-escolha" disabled={adicionar.isPending} onClick={() => adicionar.mutate({ lead_id: l.id })}>
                      <strong>{nomeExibicao(l.nome, l.telefone)}</strong>
                      <small>{telefone(l.telefone)}</small>
                    </button>
                  </li>
                ))
              : (contatos.data?.itens ?? [])
                  .filter((c) => !c.jid.endsWith('@g.us'))
                  .map((c) => (
                    <li key={c.id}>
                      <button type="button" className="item-escolha" disabled={adicionar.isPending} onClick={() => adicionar.mutate({ contato_id: c.id })}>
                        <strong>{nomeExibicao(c.nome ?? c.nome_push, c.telefone)}</strong>
                        <small>{telefone(c.telefone)}</small>
                      </button>
                    </li>
                  ))}
            {(aba === 'lead' ? leads : contatos).isPending ? <li className="texto-secundario">Carregando…</li> : null}
            {(aba === 'lead' ? leads.data?.itens.length === 0 : contatos.data?.itens.length === 0) ? (
              <li className="texto-secundario">Nada encontrado.</li>
            ) : null}
          </ul>
        </>
      )}
      {adicionar.error ? <FaixaAviso tipo="erro">{textoErro(adicionar.error)}</FaixaAviso> : null}
    </Modal>
  );
}
