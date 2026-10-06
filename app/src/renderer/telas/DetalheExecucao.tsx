// Detalhe da execução (T081): estado, gatilho, ações com resultado, log, erro, tokens, duração e
// `retomar_em`. Atualiza ao vivo pelos eventos `automacao.execucao.*`.
import { ArrowLeft } from 'lucide-react';
import { Link, useParams } from 'react-router';

import { useExecucao } from '../api/automacoes';
import { Esqueleto } from '../componentes/Esqueleto';
import { ResultadoExecucao } from '../componentes/ResultadoExecucao';
import { TelaErro } from '../componentes/TelaErro';
import { useAbrirConversa } from '../componentes/abrirConversa';

export function DetalheExecucao() {
  const { execucaoId = '' } = useParams();
  const execucao = useExecucao(execucaoId);
  const abrirConversa = useAbrirConversa();
  if (execucao.isError) return <TelaErro erro={execucao.error} aoTentar={() => void execucao.refetch()} />;
  const dados = execucao.data;
  return (
    <section className="tela tela-rolavel">
      <header className="cabecalho-tela">
        <Link
          to={dados ? `/automacoes/${dados.automacao_id}?aba=execucoes` : '/automacoes'}
          className="botao-icone"
          aria-label="Voltar para a automação"
          title="Voltar para a automação"
        >
          <ArrowLeft size={20} />
        </Link>
        <h1>{dados ? `Execução de "${dados.automacao_nome}"` : <Esqueleto largura={240} altura={20} />}</h1>
        {dados?.conversa_id ? (
          <button type="button" className="botao secundario" onClick={() => void abrirConversa(dados.conversa_id ?? '', dados.conta_id)}>
            Abrir conversa
          </button>
        ) : null}
      </header>
      {dados ? <div className="cartao"><ResultadoExecucao execucao={dados} /></div> : <Esqueleto altura={240} />}
    </section>
  );
}
