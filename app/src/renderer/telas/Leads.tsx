// Leads (T075): tabela virtualizada com busca e filtro de origem; colunas telefone, nome, origem,
// importado em, último disparo e tem WhatsApp.
import { useInfiniteQuery } from '@tanstack/react-query';
import { ListChecks, Search, Upload } from 'lucide-react';
import { useCallback, useMemo, useState } from 'react';
import { Link } from 'react-router';

import type { Lead, OrigemLead } from '@zapdesk/cliente-motor';

import { chaves } from '../api/chaves';
import { useCliente } from '../api/motor';
import { EsqueletoLista } from '../componentes/Esqueleto';
import { EstadoVazio } from '../componentes/EstadoVazio';
import { ListaVirtual } from '../componentes/ListaVirtual';
import { TelaErro } from '../componentes/TelaErro';
import { dataCurta, telefone } from '../util/formatar';
import { useAtraso } from './Conversas';

export const VAZIO_LEADS = 'Nenhum lead — importe um CSV ou use o MCP';

export const ROTULO_ORIGEM_LEAD: Record<OrigemLead, string> = {
  csv: 'Planilha',
  colado: 'Colado',
  contatos: 'Contatos',
  mcp: 'IA (MCP)',
};

function TemWhatsApp({ valor }: { valor: boolean | null }) {
  if (valor === null) return <span className="texto-secundario">—</span>;
  return valor ? <span className="selo ok">Sim</span> : <span className="selo erro">Não</span>;
}

function LinhaLead({ lead }: { lead: Lead }) {
  const extras = Object.entries(lead.campos);
  return (
    <div className="linha-tabela leads-grade">
      <span className="mono">{telefone(lead.telefone)}</span>
      <span title={extras.map(([k, v]) => `${k}: ${v}`).join('\n') || undefined}>
        {lead.nome ?? <span className="texto-secundario">—</span>}
        {extras.length > 0 ? <small className="texto-secundario"> +{extras.length} campos</small> : null}
      </span>
      <span>{ROTULO_ORIGEM_LEAD[lead.origem] ?? lead.origem}</span>
      <span>{dataCurta(lead.importado_em)}</span>
      <span>{dataCurta(lead.ultimo_disparo_em)}</span>
      <TemWhatsApp valor={lead.tem_whatsapp} />
    </div>
  );
}

export function Leads() {
  const cliente = useCliente();
  const [busca, setBusca] = useState('');
  const [origem, setOrigem] = useState<OrigemLead | ''>('');
  const q = useAtraso(busca.trim());

  const consulta = useInfiniteQuery({
    queryKey: [...chaves.leads, { q, origem }],
    queryFn: ({ pageParam }) =>
      cliente.listarLeads({
        limite: 200,
        ...(pageParam ? { cursor: pageParam } : {}),
        ...(q ? { busca: q } : {}),
        ...(origem ? { origem } : {}),
      }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (p) => p.proximo_cursor ?? undefined,
  });
  const leads = useMemo(() => consulta.data?.pages.flatMap((p) => p.itens) ?? [], [consulta.data]);
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = consulta;
  const carregarMais = useCallback(() => {
    if (hasNextPage && !isFetchingNextPage) void fetchNextPage();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);
  const filtrando = Boolean(q || origem);

  return (
    <section className="tela tela-lista">
      <header className="cabecalho-tela">
        <h1>Leads</h1>
        <Link to="/leads/importar" className="botao">
          <Upload size={16} aria-hidden="true" /> Importar leads
        </Link>
      </header>
      <div className="barra-ferramentas">
        <label className="campo-busca">
          <Search size={16} aria-hidden="true" />
          <input type="search" aria-label="Buscar lead" placeholder="Buscar por telefone ou nome" value={busca} onChange={(e) => setBusca(e.target.value)} />
        </label>
        <select className="seletor" aria-label="Filtrar por origem" value={origem} onChange={(e) => setOrigem(e.target.value as OrigemLead | '')}>
          <option value="">Todas as origens</option>
          {(Object.keys(ROTULO_ORIGEM_LEAD) as OrigemLead[]).map((o) => (
            <option key={o} value={o}>
              {ROTULO_ORIGEM_LEAD[o]}
            </option>
          ))}
        </select>
      </div>
      {consulta.isPending ? (
        <EsqueletoLista />
      ) : consulta.isError ? (
        <TelaErro erro={consulta.error} aoTentar={() => void consulta.refetch()} />
      ) : leads.length === 0 ? (
        filtrando ? (
          <EstadoVazio titulo="Nenhum lead encontrado" texto="Tente outra busca ou origem." />
        ) : (
          <EstadoVazio
            icone={<ListChecks size={56} />}
            titulo={VAZIO_LEADS}
            texto="Leads são os números para seus disparos. Telefones repetidos nunca viram lead duplicado."
            acao={
              <Link to="/leads/importar" className="botao">
                Importar leads
              </Link>
            }
          />
        )
      ) : (
        <div className="tabela-virtual">
          <div className="linha-tabela leads-grade cabecalho" role="row">
            <span>Telefone</span>
            <span>Nome</span>
            <span>Origem</span>
            <span>Importado em</span>
            <span>Último disparo</span>
            <span>Tem WhatsApp</span>
          </div>
          <ListaVirtual
            rotulo="Leads"
            itens={leads}
            estimativa={44}
            chave={(l) => l.id}
            aoChegarNoFim={carregarMais}
            renderizar={(l) => <LinhaLead lead={l} />}
          />
        </div>
      )}
    </section>
  );
}
