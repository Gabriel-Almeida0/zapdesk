// Painel lateral do contato (T123): etiquetas, notas com salvamento e lead de origem.
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, Plus, X } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';

import type { Contato, Conversa, Etiqueta } from '@zapdesk/cliente-motor';

import { chaves } from '../api/chaves';
import { useCliente } from '../api/motor';
import { dataCurta, nomeExibicao, telefone, textoErro } from '../util/formatar';
import { Avatar } from './Avatar';
import { BotaoIcone } from './BotaoIcone';
import { Esqueleto } from './Esqueleto';
import { FaixaAviso } from './FaixaAviso';

export const LIMITE_NOTAS = 10_000;

const ROTULO_ORIGEM: Record<string, string> = {
  csv: 'planilha (CSV/XLSX)',
  colado: 'números colados',
  contatos: 'contatos/etiquetas',
  mcp: 'IA (MCP)',
};

export function ChipEtiqueta({ etiqueta, aoRemover }: { etiqueta: Pick<Etiqueta, 'nome' | 'cor'>; aoRemover?: () => void }) {
  return (
    <span className="chip-etiqueta">
      <span className="ponto-etiqueta" style={{ background: etiqueta.cor }} aria-hidden="true" />
      {etiqueta.nome}
      {aoRemover ? (
        <button type="button" className="chip-remover" aria-label={`Remover etiqueta ${etiqueta.nome}`} onClick={aoRemover}>
          <X size={12} />
        </button>
      ) : null}
    </span>
  );
}

function EtiquetasContato({ contato }: { contato: Contato }) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const [aberto, setAberto] = useState(false);
  const todas = useQuery({ queryKey: chaves.etiquetas, queryFn: () => cliente.listarEtiquetas() });
  const definir = useMutation({
    mutationFn: (ids: string[]) => cliente.definirEtiquetasContato(contato.id, ids),
    onSuccess: (c) => {
      qc.setQueryData(chaves.contato(c.id), c);
      void qc.invalidateQueries({ queryKey: chaves.conversas(c.conta_id) });
      void qc.invalidateQueries({ queryKey: chaves.etiquetas });
    },
  });
  const atuais = contato.etiquetas.map((e) => e.id);
  const alternar = (id: string) =>
    definir.mutate(atuais.includes(id) ? atuais.filter((x) => x !== id) : [...atuais, id]);

  return (
    <section className="painel-secao">
      <h3>Etiquetas</h3>
      <div className="lista-chips">
        {contato.etiquetas.map((e) => (
          <ChipEtiqueta key={e.id} etiqueta={e} aoRemover={() => alternar(e.id)} />
        ))}
        <button type="button" className="chip" aria-expanded={aberto} onClick={() => setAberto((a) => !a)}>
          <Plus size={14} aria-hidden="true" /> Etiqueta
        </button>
      </div>
      {aberto ? (
        <div className="lista-escolha" role="group" aria-label="Escolher etiquetas">
          {(todas.data ?? []).length === 0 ? (
            <p className="texto-secundario">Nenhuma etiqueta. Crie em Etiquetas.</p>
          ) : (
            (todas.data ?? []).map((e) => (
              <label key={e.id} className="caixa">
                <input type="checkbox" checked={atuais.includes(e.id)} disabled={definir.isPending} onChange={() => alternar(e.id)} />
                <span className="ponto-etiqueta" style={{ background: e.cor }} aria-hidden="true" />
                {e.nome}
              </label>
            ))
          )}
        </div>
      ) : null}
      {definir.error ? <FaixaAviso tipo="erro">{textoErro(definir.error)}</FaixaAviso> : null}
    </section>
  );
}

function NotasContato({ contato }: { contato: Contato }) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const [notas, setNotas] = useState(contato.notas ?? '');
  const salvoRef = useRef(contato.notas ?? '');
  const salvar = useMutation({
    mutationFn: (valor: string) => cliente.salvarNotas(contato.id, valor),
    onSuccess: (c) => {
      salvoRef.current = c.notas ?? '';
      qc.setQueryData(chaves.contato(c.id), c);
    },
  });
  // Atualização vinda de fora (MCP) sem sobrescrever o que está sendo digitado.
  useEffect(() => {
    if (notas === salvoRef.current) {
      setNotas(contato.notas ?? '');
      salvoRef.current = contato.notas ?? '';
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [contato.notas]);

  const alterado = notas !== salvoRef.current;
  return (
    <section className="painel-secao">
      <h3>Notas</h3>
      <textarea
        className="campo-notas"
        aria-label="Notas do contato"
        maxLength={LIMITE_NOTAS}
        rows={6}
        placeholder="Anote o que importa sobre este contato"
        value={notas}
        onChange={(e) => setNotas(e.target.value)}
        onBlur={() => alterado && salvar.mutate(notas)}
      />
      <div className="linha-acoes">
        <small className="texto-secundario">
          {notas.length.toLocaleString('pt-BR')}/{LIMITE_NOTAS.toLocaleString('pt-BR')}
        </small>
        {salvar.isPending ? (
          <small>Salvando…</small>
        ) : alterado ? (
          <button type="button" className="botao pequeno" onClick={() => salvar.mutate(notas)}>
            Salvar notas
          </button>
        ) : salvar.isSuccess ? (
          <small className="salvo">
            <Check size={14} aria-hidden="true" /> Salvo
          </small>
        ) : null}
      </div>
      {salvar.error ? <FaixaAviso tipo="erro">{textoErro(salvar.error)}</FaixaAviso> : null}
    </section>
  );
}

export function PainelContato({ conversa, aoFechar }: { conversa: Conversa; aoFechar: () => void }) {
  const cliente = useCliente();
  const contatoId = conversa.contato_id;
  const consulta = useQuery({
    queryKey: chaves.contato(contatoId ?? '-'),
    queryFn: () => cliente.obterContato(contatoId as string),
    enabled: Boolean(contatoId),
  });
  const contato = consulta.data;

  return (
    <aside className="painel-contato" aria-label="Dados do contato">
      <header className="cabecalho-painel">
        <BotaoIcone rotulo="Fechar dados do contato" onClick={aoFechar}>
          <X size={20} />
        </BotaoIcone>
        <h2>{conversa.tipo === 'grupo' ? 'Dados do grupo' : 'Dados do contato'}</h2>
      </header>
      <div className="painel-contato-topo">
        <Avatar nome={conversa.nome} chave={conversa.jid} tamanho={96} grupo={conversa.tipo === 'grupo'} />
        <h2>{nomeExibicao(conversa.nome, conversa.telefone)}</h2>
        {conversa.telefone ? <p>{telefone(conversa.telefone)}</p> : null}
        {contato?.nome_push && contato.nome_push !== conversa.nome ? <p className="texto-secundario">~{contato.nome_push}</p> : null}
      </div>
      {!contatoId ? (
        <p className="texto-secundario painel-secao">Etiquetas e notas ficam disponíveis em conversas individuais.</p>
      ) : consulta.isPending ? (
        <div className="painel-secao">
          <Esqueleto altura={14} />
          <Esqueleto altura={80} />
        </div>
      ) : consulta.isError || !contato ? (
        <FaixaAviso tipo="erro">{textoErro(consulta.error)}</FaixaAviso>
      ) : (
        <>
          <EtiquetasContato contato={contato} />
          <NotasContato key={contato.id} contato={contato} />
          <section className="painel-secao">
            <h3>Lead</h3>
            {contato.lead ? (
              <p>
                Importado via {ROTULO_ORIGEM[contato.lead.origem] ?? contato.lead.origem} em {dataCurta(contato.lead.importado_em)}
              </p>
            ) : (
              <p className="texto-secundario">Este contato não está na base de leads.</p>
            )}
          </section>
        </>
      )}
    </aside>
  );
}
