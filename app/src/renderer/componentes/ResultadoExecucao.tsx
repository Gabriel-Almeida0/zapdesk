// Resultado de uma execução (detalhe, "Testar" do fluxo e da IA): estado, ações com resultado,
// log, erro com stack, retorno, tokens e duração.
import type { AcaoRegistrada, Execucao, ExecucaoDetalhe, Tokens } from '@zapdesk/cliente-motor';

import {
  ROTULO_ACAO,
  classeEstadoExecucao,
  ROTULO_ESTADO_EXECUCAO,
  ROTULO_GATILHO,
  ROTULO_ORIGEM_EXECUCAO,
  ROTULO_RESULTADO_ACAO,
  textoDuracaoMs,
} from '../util/automacoes';
import { dataHora, numero } from '../util/formatar';
import { FaixaAviso } from './FaixaAviso';

function ehDetalhe(e: Execucao | ExecucaoDetalhe): e is ExecucaoDetalhe {
  return 'log' in e;
}

export function textoTokens(t: Tokens): string {
  if (t.entrada === 0 && t.saida === 0) return 'Nenhum';
  return `${numero(t.entrada)} de entrada · ${numero(t.saida)} de saída`;
}

export function ListaAcoesRegistradas({ acoes }: { acoes: AcaoRegistrada[] }) {
  if (acoes.length === 0) return <p className="texto-secundario">Nenhuma ação.</p>;
  return (
    <ol className="acoes-registradas">
      {acoes.map((a, i) => (
        <li key={i} className={`acao-registrada resultado-${a.resultado}`}>
          <span className={`selo ${a.resultado === 'ok' ? 'ok' : a.resultado === 'simulada' ? 'info' : 'erro'}`}>{ROTULO_RESULTADO_ACAO[a.resultado]}</span>
          <strong>{ROTULO_ACAO[a.tipo as keyof typeof ROTULO_ACAO] ?? a.tipo}</strong>
          {a.alvo ? <span className="texto-secundario">{a.alvo}</span> : null}
          {a.detalhe ? <span className="acao-detalhe">{a.detalhe}</span> : null}
        </li>
      ))}
    </ol>
  );
}

export function ResultadoExecucao({ execucao, compacto }: { execucao: Execucao | ExecucaoDetalhe; compacto?: boolean }) {
  const detalhe = ehDetalhe(execucao) ? execucao : null;
  const gatilho = ROTULO_GATILHO[execucao.gatilho.tipo as keyof typeof ROTULO_GATILHO] ?? execucao.gatilho.tipo;
  const temRetorno = execucao.retorno !== null && execucao.retorno !== undefined;
  return (
    <div className="resultado-execucao">
      <dl className="lista-definicoes em-linha">
        <dt>Estado</dt>
        <dd>
          <span className={classeEstadoExecucao(execucao.estado)}>{ROTULO_ESTADO_EXECUCAO[execucao.estado]}</span>
          {execucao.simulacao && execucao.estado !== 'simulacao' ? <span className="selo info">simulação</span> : null}
        </dd>
        <dt>Duração</dt>
        <dd>{textoDuracaoMs(execucao.duracao_ms)}</dd>
        {compacto ? null : (
          <>
            <dt>Gatilho</dt>
            <dd>{gatilho}</dd>
            <dt>Origem</dt>
            <dd>{ROTULO_ORIGEM_EXECUCAO[execucao.origem] ?? execucao.origem}</dd>
            <dt>Início</dt>
            <dd>{dataHora(execucao.iniciada_em)}</dd>
            <dt>Versão</dt>
            <dd>{execucao.automacao_versao}</dd>
          </>
        )}
        <dt>Tokens de IA</dt>
        <dd>{textoTokens(execucao.tokens)}</dd>
      </dl>
      {execucao.estado === 'aguardando' && execucao.retomar_em ? (
        <FaixaAviso tipo="info">Aguardando até {dataHora(execucao.retomar_em)}.</FaixaAviso>
      ) : null}
      {execucao.motivo ? <FaixaAviso tipo={execucao.estado === 'erro' ? 'erro' : 'info'}>{execucao.motivo}</FaixaAviso> : null}
      {execucao.erro ? <FaixaAviso tipo="erro">{execucao.erro}</FaixaAviso> : null}
      {detalhe?.erro_stack ? (
        <details>
          <summary>Stack do erro</summary>
          <pre className="bloco-log">{detalhe.erro_stack}</pre>
        </details>
      ) : null}
      <h3>Ações {execucao.simulacao ? 'que seriam feitas' : ''}</h3>
      <ListaAcoesRegistradas acoes={execucao.acoes} />
      {Object.keys(execucao.tokens.por_modelo).length > 1 ? (
        <table className="tabela tabela-tokens">
          <thead>
            <tr>
              <th>Modelo</th>
              <th>Chamadas</th>
              <th>Entrada</th>
              <th>Saída</th>
            </tr>
          </thead>
          <tbody>
            {Object.entries(execucao.tokens.por_modelo).map(([modelo, t]) => (
              <tr key={modelo}>
                <td>{modelo}</td>
                <td>{numero(t.chamadas)}</td>
                <td>{numero(t.entrada)}</td>
                <td>{numero(t.saida)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : null}
      {temRetorno ? (
        <>
          <h3>Retorno</h3>
          <pre className="bloco-log">{typeof execucao.retorno === 'string' ? execucao.retorno : JSON.stringify(execucao.retorno, null, 2)}</pre>
        </>
      ) : null}
      {detalhe ? (
        <>
          <h3>Log</h3>
          {detalhe.log ? <pre className="bloco-log">{detalhe.log}</pre> : <p className="texto-secundario">Sem log.</p>}
          {detalhe.log_truncado ? <p className="texto-secundario">[log truncado em 64 KB]</p> : null}
          {Object.keys(detalhe.variaveis).length > 0 ? (
            <>
              <h3>Variáveis</h3>
              <pre className="bloco-log">{JSON.stringify(detalhe.variaveis, null, 2)}</pre>
            </>
          ) : null}
        </>
      ) : null}
    </div>
  );
}
