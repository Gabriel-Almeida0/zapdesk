// Editor de uma automação: escolhe o editor pelo tipo e reúne as peças comuns (interruptor
// Ativar/Desativar, abas Editar/Execuções e o diálogo "Executar").
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Trash } from 'lucide-react';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router';

import { ErroMotor, type Automacao, type Execucao, type Id } from '@zapdesk/cliente-motor';

import { useAutomacao } from '../api/automacoes';
import { chaves } from '../api/chaves';
import { useCliente } from '../api/motor';
import { EsqueletoLista } from '../componentes/Esqueleto';
import { FaixaAviso } from '../componentes/FaixaAviso';
import { BotaoIcone } from '../componentes/BotaoIcone';
import { Confirmar, Modal } from '../componentes/Modal';
import { TelaErro } from '../componentes/TelaErro';
import { useContaAtual } from '../estado/conta';
import { ROTULO_ESTADO_EXECUCAO } from '../util/automacoes';
import { nomeExibicao, telefone, textoErro } from '../util/formatar';
import { useAtraso } from './Conversas';
import { EditorChatbot } from './EditorChatbot/EditorChatbot';
import { EditorFluxo } from './EditorFluxo/EditorFluxo';
import { EditorIA } from './EditorIA/EditorIA';

export type AbaAutomacao = 'editar' | 'execucoes';

export function useAbaAutomacao(): [AbaAutomacao, (a: AbaAutomacao) => void] {
  const [parametros, setParametros] = useSearchParams();
  const aba: AbaAutomacao = parametros.get('aba') === 'execucoes' ? 'execucoes' : 'editar';
  return [aba, (a) => setParametros(a === 'editar' ? {} : { aba: a }, { replace: true })];
}

export function AbasAutomacao({ aba, aoMudar, execucoes }: { aba: AbaAutomacao; aoMudar: (a: AbaAutomacao) => void; execucoes?: number }) {
  return (
    <div className="abas abas-editor" role="tablist" aria-label="Seções da automação">
      <button type="button" role="tab" aria-selected={aba === 'editar'} className={`aba${aba === 'editar' ? ' ativa' : ''}`} onClick={() => aoMudar('editar')}>
        Editar
      </button>
      <button
        type="button"
        role="tab"
        aria-selected={aba === 'execucoes'}
        className={`aba${aba === 'execucoes' ? ' ativa' : ''}`}
        onClick={() => aoMudar('execucoes')}
      >
        Execuções{execucoes ? ` (${execucoes})` : ''}
      </button>
    </div>
  );
}

/** Mensagem para quando o motor recusa ativar (compilação, definição inválida). */
export function textoErroAtivar(erro: unknown): string {
  if (erro instanceof ErroMotor && (erro.codigo === 'definicao_invalida' || erro.codigo === 'compilacao_falhou')) {
    const lista = (erro.detalhes.erros ?? []) as { mensagem: string }[];
    const primeiros = lista.slice(0, 3).map((e) => e.mensagem);
    return primeiros.length > 0 ? `${erro.mensagem} ${primeiros.join(' · ')}` : erro.mensagem;
  }
  return textoErro(erro);
}

/** Interruptor Ativar/Desativar (o motor valida/compila antes de ativar). */
export function InterruptorAtiva(props: {
  automacao: Automacao;
  bloqueado?: string | null;
  antesDeAtivar?: () => Promise<boolean>;
  aoErro?: (mensagem: string | null) => void;
}) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const { automacao } = props;
  const alternar = useMutation({
    mutationFn: async () => {
      if (!automacao.ativa && props.antesDeAtivar && !(await props.antesDeAtivar())) return automacao;
      return automacao.ativa ? cliente.desativarAutomacao(automacao.id) : cliente.ativarAutomacao(automacao.id);
    },
    onMutate: () => props.aoErro?.(null),
    onSuccess: (a) => {
      qc.setQueryData(chaves.automacao(a.id), a);
      void qc.invalidateQueries({ queryKey: chaves.automacoes });
    },
    onError: (e) => props.aoErro?.(textoErroAtivar(e)),
  });
  const bloqueado = !automacao.ativa && Boolean(props.bloqueado);
  return (
    <label className={`interruptor${automacao.ativa ? ' ligado' : ''}`} title={bloqueado ? (props.bloqueado ?? undefined) : undefined}>
      <input
        type="checkbox"
        role="switch"
        aria-label={automacao.ativa ? `Desativar ${automacao.nome}` : `Ativar ${automacao.nome}`}
        checked={automacao.ativa}
        disabled={alternar.isPending || bloqueado}
        onChange={() => alternar.mutate()}
      />
      <span className="interruptor-trilho" aria-hidden="true" />
      <span>{automacao.ativa ? 'Ativa' : 'Inativa'}</span>
    </label>
  );
}

/** "Executar" (FR-022): escolhe um contato da conta atual (ou nenhum) e roda ignorando o gatilho. */
export function ExecutarAutomacao(props: { automacao: Automacao; aoFechar: () => void; comEntrada?: boolean }) {
  const cliente = useCliente();
  const { contaId } = useContaAtual();
  const [busca, setBusca] = useState('');
  const termo = useAtraso(busca.trim(), 250);
  const [entrada, setEntrada] = useState('{}');
  const [resultado, setResultado] = useState<Execucao | null>(null);
  const contatos = useQuery({
    queryKey: ['executar', 'contatos', contaId, termo],
    queryFn: () => cliente.listarContatos(contaId ?? '', { limite: 20, ...(termo ? { busca: termo } : {}) }),
    enabled: Boolean(contaId),
  });
  const executar = useMutation({
    mutationFn: (contatoId: Id | null) => {
      let valor: unknown = undefined;
      if (props.comEntrada && entrada.trim()) valor = JSON.parse(entrada) as unknown;
      const extra = { ...(valor !== undefined ? { entrada: valor } : {}), origem: 'manual_app' as const };
      return cliente.executarAutomacao(props.automacao.id, contatoId ? { contato_id: contatoId, ...extra } : extra);
    },
    onSuccess: setResultado,
  });
  return (
    <Modal titulo={`Executar "${props.automacao.nome}"`} aoFechar={props.aoFechar} largura={520}>
      {resultado ? (
        <>
          <FaixaAviso tipo={resultado.estado === 'erro' ? 'erro' : 'info'}>
            Execução {ROTULO_ESTADO_EXECUCAO[resultado.estado].toLowerCase()}.{' '}
            <Link to={`/execucoes/${resultado.id}`} onClick={props.aoFechar}>
              Ver detalhes
            </Link>
          </FaixaAviso>
          <div className="acoes-formulario">
            <button type="button" className="botao" onClick={props.aoFechar}>
              Fechar
            </button>
          </div>
        </>
      ) : (
        <>
          <p className="texto-secundario">Roda agora para o contato escolhido, ignorando o gatilho. Pausas e anti-loop continuam valendo.</p>
          {props.comEntrada ? (
            <label className="campo">
              <span>Entrada (JSON, para aoExecutar)</span>
              <textarea className="campo-codigo" rows={3} value={entrada} onChange={(e) => setEntrada(e.target.value)} />
            </label>
          ) : null}
          <label className="campo">
            <span>Contato</span>
            <input type="search" placeholder="Buscar nome ou telefone" value={busca} onChange={(e) => setBusca(e.target.value)} />
          </label>
          <ul className="lista-escolha alta" aria-label="Contatos">
            <li>
              <button type="button" className="item-escolha" disabled={executar.isPending} onClick={() => executar.mutate(null)}>
                <strong>Sem contato</strong>
                <small>Para automações que não precisam de conversa</small>
              </button>
            </li>
            {(contatos.data?.itens ?? []).map((c) => (
              <li key={c.id}>
                <button type="button" className="item-escolha" disabled={executar.isPending} onClick={() => executar.mutate(c.id)}>
                  <strong>{nomeExibicao(c.nome ?? c.nome_push, c.telefone)}</strong>
                  <small>{telefone(c.telefone)}</small>
                </button>
              </li>
            ))}
          </ul>
          {executar.error ? <FaixaAviso tipo="erro">{textoErro(executar.error)}</FaixaAviso> : null}
        </>
      )}
    </Modal>
  );
}

/** Excluir (FR-088): confirma, apaga no motor (IA: pasta, memória e execuções) e volta à lista. */
export function BotaoExcluirAutomacao({ automacao }: { automacao: Automacao }) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const navegar = useNavigate();
  const [confirmando, setConfirmando] = useState(false);
  const excluir = useMutation({
    mutationFn: () => cliente.excluirAutomacao(automacao.id),
    onSuccess: () => {
      qc.removeQueries({ queryKey: chaves.automacao(automacao.id) });
      void qc.invalidateQueries({ queryKey: chaves.automacoes });
      void navegar('/automacoes', { replace: true });
    },
  });
  const texto =
    automacao.tipo === 'ia'
      ? 'A pasta do projeto, a memória e as execuções são apagadas. Isso não pode ser desfeito.'
      : automacao.tipo === 'chatbot'
        ? 'Conversas com o bot em andamento são encerradas. As execuções são apagadas.'
        : 'Esperas em andamento são canceladas. As execuções são apagadas.';
  return (
    <>
      <BotaoIcone rotulo={`Excluir ${automacao.nome}`} onClick={() => setConfirmando(true)}>
        <Trash size={18} />
      </BotaoIcone>
      {confirmando ? (
        <Confirmar
          titulo={`Excluir "${automacao.nome}"?`}
          texto={excluir.error ? `${texto} (${textoErro(excluir.error)})` : texto}
          confirmar="Excluir automação"
          perigo
          ocupado={excluir.isPending}
          aoConfirmar={() => excluir.mutate()}
          aoFechar={() => setConfirmando(false)}
        />
      ) : null}
    </>
  );
}

export function EditorAutomacao() {
  const { automacaoId = '' } = useParams();
  const automacao = useAutomacao(automacaoId);
  if (automacao.isPending) {
    return (
      <section className="tela">
        <EsqueletoLista linhas={4} />
      </section>
    );
  }
  if (automacao.isError) return <TelaErro erro={automacao.error} aoTentar={() => void automacao.refetch()} />;
  const a = automacao.data;
  if (a.tipo === 'fluxo') return <EditorFluxo key={a.id} automacao={a} />;
  if (a.tipo === 'chatbot') return <EditorChatbot key={a.id} automacao={a} />;
  return <EditorIA key={a.id} automacao={a} />;
}
