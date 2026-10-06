// Painel do cartão (T070): abrir/iniciar conversa, nome e campos do lead, histórico no funil e
// remover do funil.
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { MessageCircle, Plus, X } from 'lucide-react';
import { useEffect, useState } from 'react';

import type { Card, Funil, Lead, MovimentoFunil } from '@zapdesk/cliente-motor';

import { chaves } from '../../api/chaves';
import { useCliente } from '../../api/motor';
import { useAbrirConversa } from '../../componentes/abrirConversa';
import { BotaoIcone } from '../../componentes/BotaoIcone';
import { Esqueleto } from '../../componentes/Esqueleto';
import { FaixaAviso } from '../../componentes/FaixaAviso';
import { useContaAtual } from '../../estado/conta';
import { dataHora, nomeExibicao, telefone, textoErro } from '../../util/formatar';

const ROTULO_ORIGEM: Record<MovimentoFunil['origem'], string> = { app: 'app', mcp: 'Claude (MCP)', automacao: 'automação' };

export function textoMovimento(m: MovimentoFunil): string {
  if (!m.etapa_origem_nome && m.etapa_destino_nome) return `Entrou em ${m.etapa_destino_nome}`;
  if (m.etapa_origem_nome && !m.etapa_destino_nome) return `Saiu de ${m.etapa_origem_nome}`;
  return `${m.etapa_origem_nome ?? '—'} → ${m.etapa_destino_nome ?? '—'}`;
}

function CamposLead({ lead }: { lead: Lead }) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const [nome, setNome] = useState(lead.nome ?? '');
  const [campos, setCampos] = useState<[string, string][]>(Object.entries(lead.campos));
  useEffect(() => {
    setNome(lead.nome ?? '');
    setCampos(Object.entries(lead.campos));
  }, [lead]);
  const salvar = useMutation({
    mutationFn: () => {
      const novos: Record<string, string | null> = {};
      for (const chave of Object.keys(lead.campos)) novos[chave] = null;
      for (const [k, v] of campos) if (k.trim()) novos[k.trim()] = v.slice(0, 1000);
      return cliente.editarLead(lead.id, { nome: nome.trim() || null, campos: novos });
    },
    onSuccess: (l) => {
      qc.setQueryData(chaves.lead(l.id), l);
      void qc.invalidateQueries({ queryKey: ['cards'] });
    },
  });
  return (
    <section className="painel-secao">
      <h3>Lead</h3>
      <label className="campo">
        <span>Nome</span>
        <input value={nome} maxLength={120} onChange={(e) => setNome(e.target.value)} />
      </label>
      {campos.map(([k, v], i) => (
        <div key={i} className="linha-form linha-campo-lead">
          <input aria-label={`Campo ${i + 1}`} value={k} placeholder="campo" onChange={(e) => setCampos((l) => l.map((x, j) => (j === i ? [e.target.value, x[1]] : x)))} />
          <input aria-label={`Valor do campo ${k || i + 1}`} value={v} placeholder="valor" onChange={(e) => setCampos((l) => l.map((x, j) => (j === i ? [x[0], e.target.value] : x)))} />
          <BotaoIcone rotulo={`Remover campo ${k}`} className="pequeno" onClick={() => setCampos((l) => l.filter((_, j) => j !== i))}>
            <X size={14} />
          </BotaoIcone>
        </div>
      ))}
      <div className="linha-acoes">
        <button type="button" className="botao-link" onClick={() => setCampos((l) => [...l, ['', '']])}>
          <Plus size={14} aria-hidden="true" /> Campo
        </button>
        <button type="button" className="botao pequeno" disabled={salvar.isPending} onClick={() => salvar.mutate()}>
          {salvar.isSuccess && !salvar.isPending ? 'Salvo' : 'Salvar'}
        </button>
      </div>
      {salvar.error ? <FaixaAviso tipo="erro">{textoErro(salvar.error)}</FaixaAviso> : null}
    </section>
  );
}

export function PainelCartao({ funil, card, aoFechar }: { funil: Funil; card: Card; aoFechar: () => void }) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const { contaId } = useContaAtual();
  const abrirConversa = useAbrirConversa();
  const lead = useQuery({ queryKey: chaves.lead(card.lead_id), queryFn: () => cliente.obterLead(card.lead_id) });
  const historico = useQuery({
    queryKey: chaves.historicoFunil(funil.id, card.lead_id),
    queryFn: () => cliente.historicoFunil(funil.id, { lead_id: card.lead_id, limite: 50 }),
  });
  const iniciar = useMutation({
    mutationFn: () => cliente.abrirConversa(contaId ?? '', card.lead.telefone),
    onSuccess: (c) => void abrirConversa(c.id, c.conta_id),
  });
  const remover = useMutation({
    mutationFn: () => cliente.removerCard(funil.id, card.lead_id, 'app'),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: chaves.cards(funil.id) });
      void qc.invalidateQueries({ queryKey: chaves.funil(funil.id) });
      aoFechar();
    },
  });
  const nome = nomeExibicao(card.lead.nome, card.lead.telefone);
  const etapa = funil.etapas.find((e) => e.id === card.etapa_id);

  return (
    <aside className="painel-contato painel-cartao" aria-label={`Detalhes de ${nome}`}>
      <header className="cabecalho-painel">
        <BotaoIcone rotulo="Fechar" onClick={aoFechar}>
          <X size={20} />
        </BotaoIcone>
        <h2>Lead no funil</h2>
      </header>
      <div className="painel-contato-topo">
        <strong>{nome}</strong>
        {card.lead.nome ? <small>{telefone(card.lead.telefone)}</small> : null}
        {etapa ? (
          <span className="chip-etiqueta">
            <span className="ponto-etiqueta" style={{ background: etapa.cor }} aria-hidden="true" />
            {etapa.nome} · desde {dataHora(card.desde)}
          </span>
        ) : null}
        {card.conversa_id ? (
          <button type="button" className="botao" onClick={() => void abrirConversa(card.conversa_id ?? '', card.conta_id)}>
            <MessageCircle size={16} aria-hidden="true" /> Abrir conversa
          </button>
        ) : (
          <button type="button" className="botao" disabled={!contaId || iniciar.isPending} onClick={() => iniciar.mutate()}>
            <MessageCircle size={16} aria-hidden="true" /> Iniciar conversa
          </button>
        )}
        {iniciar.error ? <FaixaAviso tipo="erro">{textoErro(iniciar.error)}</FaixaAviso> : null}
      </div>
      {lead.data ? <CamposLead lead={lead.data} /> : lead.isError ? <FaixaAviso tipo="erro">{textoErro(lead.error)}</FaixaAviso> : <Esqueleto altura={80} />}
      <section className="painel-secao">
        <h3>Histórico neste funil</h3>
        {historico.isPending ? (
          <Esqueleto altura={60} />
        ) : historico.isError ? (
          <FaixaAviso tipo="erro">{textoErro(historico.error)}</FaixaAviso>
        ) : historico.data.itens.length === 0 ? (
          <p className="texto-secundario">Sem movimentações.</p>
        ) : (
          <ol className="historico-funil">
            {historico.data.itens.map((m) => (
              <li key={m.id}>
                <strong>{textoMovimento(m)}</strong>
                <small>
                  {dataHora(m.em)} · {ROTULO_ORIGEM[m.origem]}
                </small>
              </li>
            ))}
          </ol>
        )}
      </section>
      <section className="painel-secao">
        <button type="button" className="botao secundario perigo-texto" disabled={remover.isPending} onClick={() => remover.mutate()}>
          Remover do funil
        </button>
        {remover.error ? <FaixaAviso tipo="erro">{textoErro(remover.error)}</FaixaAviso> : null}
      </section>
    </aside>
  );
}
