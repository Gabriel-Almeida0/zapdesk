// Aba "Execuções" (T081): lista filtrável por estado, com ok/erro, duração, tokens e ações; clique
// abre o detalhe. Ao vivo pelos eventos (invalidação do cache).
import { History } from 'lucide-react';
import { useMemo, useState } from 'react';
import { Link } from 'react-router';

import type { EstadoExecucao, Id } from '@zapdesk/cliente-motor';

import { useExecucoes } from '../api/automacoes';
import { EsqueletoLista } from '../componentes/Esqueleto';
import { EstadoVazio } from '../componentes/EstadoVazio';
import { TelaErro } from '../componentes/TelaErro';
import { textoTokens } from '../componentes/ResultadoExecucao';
import { classeEstadoExecucao, ROTULO_ESTADO_EXECUCAO, ROTULO_GATILHO, ROTULO_ORIGEM_EXECUCAO, textoDuracaoMs } from '../util/automacoes';
import { dataHora } from '../util/formatar';

const FILTROS: (EstadoExecucao | 'todos')[] = ['todos', 'ok', 'erro', 'aguardando', 'rodando', 'simulacao', 'abortada'];

export function Execucoes({ automacaoId }: { automacaoId: Id | null }) {
  const [estado, setEstado] = useState<EstadoExecucao | 'todos'>('todos');
  const consulta = useExecucoes(automacaoId, estado === 'todos' ? undefined : estado);
  const itens = useMemo(() => consulta.data?.pages.flatMap((p) => p.itens) ?? [], [consulta.data]);

  return (
    <div className="execucoes">
      <div className="filtros" role="group" aria-label="Filtrar por estado">
        {FILTROS.map((f) => (
          <button key={f} type="button" className={`chip${estado === f ? ' ativo' : ''}`} aria-pressed={estado === f} onClick={() => setEstado(f)}>
            {f === 'todos' ? 'Todas' : ROTULO_ESTADO_EXECUCAO[f]}
          </button>
        ))}
      </div>
      {consulta.isPending ? (
        <EsqueletoLista linhas={4} />
      ) : consulta.isError ? (
        <TelaErro erro={consulta.error} aoTentar={() => void consulta.refetch()} />
      ) : itens.length === 0 ? (
        <EstadoVazio
          icone={<History size={48} />}
          titulo="Nenhuma execução"
          texto={estado === 'todos' ? 'Quando a automação rodar (ou for testada), as execuções aparecem aqui com log e ações.' : 'Nenhuma execução neste estado.'}
        />
      ) : (
        <table className="tabela tabela-execucoes">
          <thead>
            <tr>
              <th>Início</th>
              {automacaoId ? null : <th>Automação</th>}
              <th>Estado</th>
              <th>Gatilho</th>
              <th>Ações</th>
              <th>Duração</th>
              <th>Tokens</th>
            </tr>
          </thead>
          <tbody>
            {itens.map((e) => (
              <tr key={e.id}>
                <td>
                  <Link to={`/execucoes/${e.id}`}>{dataHora(e.iniciada_em)}</Link>
                </td>
                {automacaoId ? null : <td>{e.automacao_nome}</td>}
                <td>
                  <span className={classeEstadoExecucao(e.estado)}>{ROTULO_ESTADO_EXECUCAO[e.estado]}</span>
                  {e.erro ? <small className="erro-campo linha-erro">{e.erro}</small> : e.motivo ? <small className="texto-secundario linha-erro">{e.motivo}</small> : null}
                </td>
                <td>
                  {ROTULO_GATILHO[e.gatilho.tipo as keyof typeof ROTULO_GATILHO] ?? e.gatilho.tipo}
                  {e.origem !== 'gatilho' ? <small className="texto-secundario"> · {ROTULO_ORIGEM_EXECUCAO[e.origem]}</small> : null}
                </td>
                <td>{e.acoes.length}</td>
                <td>{textoDuracaoMs(e.duracao_ms)}</td>
                <td>{textoTokens(e.tokens)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {consulta.hasNextPage ? (
        <button type="button" className="botao secundario" disabled={consulta.isFetchingNextPage} onClick={() => void consulta.fetchNextPage()}>
          Carregar mais
        </button>
      ) : null}
    </div>
  );
}
