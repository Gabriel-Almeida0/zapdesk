// Faixa de estado das automações na conversa (T082, T116): "Chatbot “X” ativo nesta conversa · Assumir",
// "Atendimento humano até HH:MM · Devolver às automações", "Automações pausadas (anti-loop) até
// HH:MM · Retomar". Atualiza por `conversa.pausa` e `chatbot.sessao.*` (cache).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Bot, Hand, CirclePause, UserRound } from 'lucide-react';
import { Link } from 'react-router';

import type { EstadoConversaAutomacoes, Id, Pausa } from '@zapdesk/cliente-motor';

import { useEstadoConversaAutomacoes } from '../api/automacoes';
import { chaves } from '../api/chaves';
import { useCliente } from '../api/motor';
import { dataHora, ehHoje, hora, textoErro } from '../util/formatar';
import { FaixaAviso } from './FaixaAviso';

function ate(pausa: Pausa): string {
  if (!pausa.ate) return '';
  return ehHoje(pausa.ate) ? ` até ${hora(pausa.ate)}` : ` até ${dataHora(pausa.ate)}`;
}

/** Texto principal da faixa (exportado para testes). */
export function textoFaixa(estado: EstadoConversaAutomacoes): string | null {
  const { pausa, sessao } = estado;
  if (pausa?.motivo === 'humano') return pausa.ate ? `Atendimento humano${ate(pausa)}` : 'Atendimento humano (sem prazo)';
  if (pausa?.motivo === 'anti_loop') return `Automações pausadas (anti-loop)${ate(pausa)}`;
  if (pausa?.motivo === 'manual') return `Automações pausadas nesta conversa${ate(pausa) || ' (sem prazo)'}`;
  // Nome entre aspas: "Bot de preços" virava "Bot Bot de preços ativo…".
  if (sessao && sessao.estado === 'ativa') return `Chatbot “${sessao.automacao_nome}” ativo nesta conversa`;
  return null;
}

function useAcoesPausa(conversaId: Id) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const atualizar = () => void qc.invalidateQueries({ queryKey: chaves.estadoConversaAutomacoes(conversaId) });
  const assumir = useMutation({
    mutationFn: () => cliente.pausarConversa(conversaId, { motivo: 'humano' }),
    onSuccess: (pausa) => {
      qc.setQueryData<EstadoConversaAutomacoes>(chaves.estadoConversaAutomacoes(conversaId), (e) => (e ? { ...e, pausa, sessao: null } : e));
    },
    onSettled: atualizar,
  });
  const retomar = useMutation({
    mutationFn: () => cliente.retomarConversa(conversaId),
    onSuccess: () => {
      qc.setQueryData<EstadoConversaAutomacoes>(chaves.estadoConversaAutomacoes(conversaId), (e) => (e ? { ...e, pausa: null } : e));
    },
    onSettled: atualizar,
  });
  return { assumir, retomar };
}

export function FaixaAutomacoes({ conversaId }: { conversaId: Id }) {
  const estado = useEstadoConversaAutomacoes(conversaId);
  const { assumir, retomar } = useAcoesPausa(conversaId);
  // Motor sem as rotas (ou falha pontual): a conversa segue normal, sem faixa.
  if (!estado.data) return null;
  const dados = estado.data;
  const texto = textoFaixa(dados);
  const erro = assumir.error ?? retomar.error;

  return (
    <>
      {dados.pausa_geral ? (
        <FaixaAviso tipo="info" icone={<CirclePause size={16} />} acao={<Link to="/ajustes#automacoes">Ajustes</Link>}>
          Todas as automações estão pausadas.
        </FaixaAviso>
      ) : null}
      {texto ? (
        <div className={`faixa-automacoes${dados.pausa ? ` motivo-${dados.pausa.motivo}` : ' bot'}`} role="status">
          <span className="faixa-automacoes-icone" aria-hidden="true">
            {dados.pausa ? null : <span className="lampada" />}
            {dados.pausa?.motivo === 'humano' ? <UserRound size={16} /> : dados.pausa ? <CirclePause size={16} /> : <Bot size={16} />}
          </span>
          <span className="faixa-texto">{texto}</span>
          {dados.pausa ? (
            <button type="button" className="botao-link" disabled={retomar.isPending} onClick={() => retomar.mutate()}>
              {dados.pausa.motivo === 'humano' ? 'Devolver às automações' : 'Retomar'}
            </button>
          ) : (
            <button type="button" className="botao-link" disabled={assumir.isPending} onClick={() => assumir.mutate()}>
              Assumir
            </button>
          )}
        </div>
      ) : null}
      {erro ? <FaixaAviso tipo="erro">{textoErro(erro)}</FaixaAviso> : null}
    </>
  );
}

/** Botão "Assumir" no cabeçalho do chat (quando há automações ativas e a conversa não está pausada). */
export function BotaoAssumir({ conversaId }: { conversaId: Id }) {
  const cliente = useCliente();
  const sistema = useQuery({ queryKey: chaves.sistema, queryFn: () => cliente.sistema() });
  const estado = useEstadoConversaAutomacoes(conversaId);
  const { assumir } = useAcoesPausa(conversaId);
  if (!estado.data || estado.data.pausa || estado.data.sessao) return null;
  if (!sistema.data || !sistema.data.automacoes_ativas) return null;
  return (
    <button
      type="button"
      className="botao secundario pequeno"
      title="Pausar as automações nesta conversa até você devolver"
      disabled={assumir.isPending}
      onClick={() => assumir.mutate()}
    >
      <Hand size={14} aria-hidden="true" /> Assumir
    </button>
  );
}
