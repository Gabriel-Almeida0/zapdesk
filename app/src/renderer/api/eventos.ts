// Eventos do motor → cache do TanStack Query.
// Atualiza diretamente o que é barato (mensagens, conta, disparo) e invalida listas com limite de
// frequência (history sync e disparos geram rajadas de eventos).
import type { InfiniteData, QueryClient, QueryKey } from '@tanstack/react-query';
import { useQueryClient } from '@tanstack/react-query';
import { useEffect } from 'react';

import type {
  ConfiguracaoAutomacoes,
  Conta,
  EstadoConversaAutomacoes,
  EventoMotor,
  Execucao,
  ExecucaoDetalhe,
  Mensagem,
  Pagina,
} from '@zapdesk/cliente-motor';

import { chaves } from './chaves';
import type { FonteEventos } from './motor';

const INTERVALO_INVALIDACAO_MS = 400;

/** Invalida no máximo uma vez a cada `ms` por chave (chamada final garantida). */
export function criarInvalidador(qc: QueryClient, ms = INTERVALO_INVALIDACAO_MS) {
  const pendentes = new Map<string, ReturnType<typeof setTimeout>>();
  return (chave: QueryKey): void => {
    const id = JSON.stringify(chave);
    if (pendentes.has(id)) return;
    pendentes.set(
      id,
      setTimeout(() => {
        pendentes.delete(id);
        void qc.invalidateQueries({ queryKey: chave });
      }, ms),
    );
  };
}

type PaginasMensagens = InfiniteData<Pagina<Mensagem>, string | undefined>;

/** Insere (ou substitui) uma mensagem no topo da primeira página (mais recentes primeiro). */
export function inserirMensagem(qc: QueryClient, mensagem: Mensagem): void {
  qc.setQueryData<PaginasMensagens>(chaves.mensagens(mensagem.conversa_id), (dados) => {
    if (!dados || dados.pages.length === 0) return dados;
    const existe = dados.pages.some((p) => p.itens.some((m) => m.id === mensagem.id));
    if (existe) return substituirNasPaginas(dados, mensagem);
    const [primeira, ...resto] = dados.pages;
    if (!primeira) return dados;
    return { ...dados, pages: [{ ...primeira, itens: [mensagem, ...primeira.itens] }, ...resto] };
  });
}

function substituirNasPaginas(dados: PaginasMensagens, mensagem: Mensagem): PaginasMensagens {
  return {
    ...dados,
    pages: dados.pages.map((p) => ({
      ...p,
      itens: p.itens.map((m) => (m.id === mensagem.id ? mensagem : m)),
    })),
  };
}

export function atualizarMensagem(qc: QueryClient, mensagem: Mensagem): void {
  qc.setQueryData<PaginasMensagens>(chaves.mensagens(mensagem.conversa_id), (dados) =>
    dados ? substituirNasPaginas(dados, mensagem) : dados,
  );
}

function atualizarConta(qc: QueryClient, id: string, mudar: (c: Conta) => Conta | null): void {
  qc.setQueryData<Conta[]>(chaves.contas, (contas) => {
    if (!contas) return contas;
    const resultado: Conta[] = [];
    for (const c of contas) {
      if (c.id !== id) resultado.push(c);
      else {
        const nova = mudar(c);
        if (nova) resultado.push(nova);
      }
    }
    return resultado;
  });
}

export function aplicarEvento(qc: QueryClient, invalidar: (chave: QueryKey) => void, evento: EventoMotor): void {
  switch (evento.tipo) {
    case 'conta.atualizada': {
      const conta = evento.dados;
      const contas = qc.getQueryData<Conta[]>(chaves.contas);
      if (contas && !contas.some((c) => c.id === conta.id)) {
        qc.setQueryData<Conta[]>(chaves.contas, [...contas, conta]);
      } else {
        atualizarConta(qc, conta.id, () => conta);
      }
      break;
    }
    case 'conta.removida':
      atualizarConta(qc, evento.dados.id, () => null);
      break;
    case 'conexao.rede':
      atualizarConta(qc, evento.dados.conta_id, (c) => ({ ...c, online: evento.dados.online }));
      break;
    case 'conversa.atualizada':
      qc.setQueryData(chaves.conversa(evento.dados.id), evento.dados);
      invalidar(chaves.conversas(evento.dados.conta_id));
      break;
    case 'mensagem.nova':
      inserirMensagem(qc, evento.dados);
      invalidar(chaves.conversas(evento.dados.conta_id));
      break;
    case 'mensagem.atualizada':
      atualizarMensagem(qc, evento.dados);
      break;
    case 'contato.atualizado':
      qc.setQueryData(chaves.contato(evento.dados.id), evento.dados);
      invalidar(chaves.contatos(evento.dados.conta_id));
      invalidar(chaves.conversas(evento.dados.conta_id));
      break;
    case 'status.novo':
      invalidar(chaves.status(evento.dados.conta_id));
      break;
    case 'etiquetas.alteradas':
      invalidar(chaves.etiquetas);
      invalidar(['conversas']);
      invalidar(['contatos']);
      invalidar(['contato']);
      break;
    case 'templates.alterados':
      invalidar(chaves.templates);
      break;
    case 'leads.importados':
      invalidar(chaves.leads);
      break;
    case 'disparo.atualizado':
      qc.setQueryData(chaves.disparo(evento.dados.id), evento.dados);
      invalidar(chaves.disparos);
      break;
    case 'disparo.finalizado':
      qc.setQueryData(chaves.disparo(evento.dados.disparo.id), evento.dados.disparo);
      invalidar(chaves.disparos);
      invalidar(chaves.destinatarios(evento.dados.disparo.id));
      break;
    case 'destinatario.atualizado':
      invalidar(chaves.destinatarios(evento.dados.disparo_id));
      break;
    case 'disparos.ativos':
      invalidar(chaves.sistema);
      break;
    case 'sincronizacao.progresso':
      if (evento.conta_id && evento.dados.concluida) invalidar(chaves.conversas(evento.conta_id));
      break;
    // 002 — funil
    case 'funil.alterado':
      invalidar(chaves.funis);
      if (evento.dados.funil_id) {
        invalidar(chaves.funil(evento.dados.funil_id));
        invalidar(chaves.cards(evento.dados.funil_id));
      }
      break;
    case 'funil.movido': {
      const { movimento } = evento.dados;
      invalidar(chaves.cards(movimento.funil_id));
      invalidar(chaves.funil(movimento.funil_id));
      invalidar(chaves.funis);
      invalidar(chaves.historicoFunil(movimento.funil_id));
      invalidar(chaves.funisDoLead(movimento.lead_id));
      break;
    }
    // 002 — automações
    case 'automacao.atualizada':
      qc.setQueryData(chaves.automacao(evento.dados.id), evento.dados);
      invalidar(chaves.automacoes);
      break;
    case 'automacao.removida':
      qc.removeQueries({ queryKey: chaves.automacao(evento.dados.id) });
      invalidar(chaves.automacoes);
      break;
    case 'automacao.arquivos_alterados':
      invalidar(chaves.arquivos(evento.dados.automacao_id));
      break;
    case 'automacao.execucao.iniciada':
    case 'automacao.execucao.atualizada':
    case 'automacao.execucao.finalizada': {
      const execucao: Execucao = evento.dados;
      qc.setQueryData<ExecucaoDetalhe>(chaves.execucao(execucao.id), (antes) => (antes ? { ...antes, ...execucao } : antes));
      invalidar(chaves.execucoes);
      if (evento.tipo === 'automacao.execucao.finalizada') {
        // O detalhe (log, stack) só vem pela rota.
        invalidar(chaves.execucao(execucao.id));
        invalidar(chaves.automacao(execucao.automacao_id));
        invalidar(chaves.automacoes);
      }
      break;
    }
    case 'chatbot.sessao.iniciada':
    case 'chatbot.sessao.atualizada':
    case 'chatbot.sessao.finalizada': {
      const sessao = evento.dados;
      qc.setQueryData<EstadoConversaAutomacoes>(chaves.estadoConversaAutomacoes(sessao.conversa_id), (antes) => {
        if (!antes) return antes;
        if (sessao.estado === 'ativa') return { ...antes, sessao };
        return antes.sessao?.id === sessao.id ? { ...antes, sessao: null } : antes;
      });
      invalidar(chaves.sessoes(sessao.automacao_id));
      if (evento.tipo !== 'chatbot.sessao.atualizada') invalidar(chaves.automacoes);
      break;
    }
    case 'conversa.pausa':
      qc.setQueryData<EstadoConversaAutomacoes>(chaves.estadoConversaAutomacoes(evento.dados.conversa_id), (antes) =>
        antes ? { ...antes, pausa: evento.dados.pausa } : antes,
      );
      invalidar(chaves.pausas);
      break;
    case 'segredos.alterados':
      invalidar(chaves.segredos);
      invalidar(chaves.configuracaoIA);
      break;
    case 'automacoes.configuracao': {
      const configuracao: ConfiguracaoAutomacoes = evento.dados;
      qc.setQueryData(chaves.configuracaoAutomacoes, configuracao);
      // `pausa_geral` aparece na faixa de todas as conversas.
      qc.setQueriesData<EstadoConversaAutomacoes>({ queryKey: ['conversa-automacoes'] }, (antes) =>
        antes ? { ...antes, pausa_geral: configuracao.pausa_geral } : antes,
      );
      break;
    }
    default:
      break;
  }
}

/** Liga a fonte de eventos ao cache enquanto o app estiver montado. */
export function useSincronizarCache(fonte: FonteEventos): void {
  const qc = useQueryClient();
  useEffect(() => {
    const invalidar = criarInvalidador(qc);
    const pararEventos = fonte.aoEventoMotor((evento) => aplicarEvento(qc, invalidar, evento));
    // Reconexão do WS ou lacuna de `seq`: não há replay → recarregar tudo via HTTP.
    const pararRecarga = fonte.aoRecarregar(() => {
      void qc.invalidateQueries();
    });
    return () => {
      pararEventos();
      pararRecarga();
    };
  }, [qc, fonte]);
}
