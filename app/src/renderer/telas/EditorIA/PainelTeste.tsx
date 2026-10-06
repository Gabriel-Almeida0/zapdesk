// Painel "Testar" da automação de IA (T106): mensagem digitada, mensagem de uma conversa real,
// entrada JSON (aoExecutar) ou evento; "IA simulada"; ações ao vivo, log, retorno, duração e tokens.
// Sempre em modo simulação: nada é enviado nem alterado.
import { useMutation, useQuery } from '@tanstack/react-query';
import { FlaskConical } from 'lucide-react';
import { useEffect, useState } from 'react';

import type { AcaoRegistrada, ExecucaoDetalhe, Id, PedidoTeste } from '@zapdesk/cliente-motor';

import { chaves } from '../../api/chaves';
import { useCliente, useEventoMotor } from '../../api/motor';
import { FaixaAviso } from '../../componentes/FaixaAviso';
import { ListaAcoesRegistradas, ResultadoExecucao } from '../../componentes/ResultadoExecucao';
import { useContaAtual } from '../../estado/conta';
import { hora, nomeExibicao, textoErro } from '../../util/formatar';

export type ModoTeste = 'mensagem' | 'conversa' | 'entrada' | 'evento';

const ROTULO_MODO: Record<ModoTeste, string> = {
  mensagem: 'Mensagem',
  conversa: 'Conversa real',
  entrada: 'Entrada JSON',
  evento: 'Evento',
};

function lerJson(texto: string, rotulo: string): unknown {
  try {
    return texto.trim() ? (JSON.parse(texto) as unknown) : {};
  } catch {
    throw new Error(`${rotulo}: JSON inválido.`);
  }
}

/** Monta o pedido de teste de acordo com o modo (exportado para testes). */
export function montarPedidoTeste(
  modo: ModoTeste,
  campos: { texto: string; conversaId: Id | null; mensagemId: Id | null; entrada: string; eventoTipo: string; eventoDados: string; iaSimulada: boolean },
): PedidoTeste {
  const base = { ia_simulada: campos.iaSimulada };
  switch (modo) {
    case 'mensagem':
      if (!campos.texto.trim()) throw new Error('Escreva a mensagem de teste.');
      return { ...base, mensagem: { texto: campos.texto } };
    case 'conversa':
      if (!campos.mensagemId) throw new Error('Escolha uma mensagem recebida.');
      return { ...base, mensagem_id: campos.mensagemId, ...(campos.conversaId ? { alvo: { conversa_id: campos.conversaId } } : {}) };
    case 'entrada':
      return { ...base, entrada: lerJson(campos.entrada, 'Entrada') };
    case 'evento': {
      const dados = lerJson(campos.eventoDados, 'Dados do evento');
      if (!dados || typeof dados !== 'object' || Array.isArray(dados)) throw new Error('Os dados do evento devem ser um objeto JSON.');
      return { ...base, evento: { tipo: campos.eventoTipo.trim() || 'manual', dados: dados as Record<string, unknown> } };
    }
  }
}

function EscolherMensagem(props: { conversaId: Id | null; mensagemId: Id | null; aoConversa: (id: Id | null) => void; aoMensagem: (id: Id | null) => void }) {
  const cliente = useCliente();
  const { contaId } = useContaAtual();
  const conversas = useQuery({
    queryKey: [...chaves.conversas(contaId ?? ''), 'teste'],
    queryFn: () => cliente.listarConversas(contaId ?? '', { limite: 30 }),
    enabled: Boolean(contaId),
  });
  const mensagens = useQuery({
    queryKey: [...chaves.mensagens(props.conversaId ?? ''), 'teste'],
    queryFn: () => cliente.listarMensagens(props.conversaId ?? '', { limite: 30 }),
    enabled: Boolean(props.conversaId),
  });
  const recebidas = (mensagens.data?.itens ?? []).filter((m) => !m.de_mim && m.texto);
  return (
    <>
      <label className="campo">
        <span>Conversa</span>
        <select
          value={props.conversaId ?? ''}
          onChange={(e) => {
            props.aoConversa(e.target.value || null);
            props.aoMensagem(null);
          }}
        >
          <option value="">Escolha…</option>
          {(conversas.data?.itens ?? []).map((c) => (
            <option key={c.id} value={c.id}>
              {nomeExibicao(c.nome, c.telefone)}
            </option>
          ))}
        </select>
      </label>
      {props.conversaId ? (
        <label className="campo">
          <span>Mensagem recebida</span>
          <select value={props.mensagemId ?? ''} onChange={(e) => props.aoMensagem(e.target.value || null)}>
            <option value="">Escolha…</option>
            {recebidas.map((m) => (
              <option key={m.id} value={m.id}>
                {hora(m.enviada_em)} · {(m.texto ?? '').slice(0, 60)}
              </option>
            ))}
          </select>
          {mensagens.data && recebidas.length === 0 ? <small className="texto-secundario">Nenhuma mensagem de texto recebida nesta conversa.</small> : null}
        </label>
      ) : null}
    </>
  );
}

export function PainelTeste(props: { automacaoId: Id; salvarAntes: () => Promise<boolean>; chaveConfigurada: boolean }) {
  const cliente = useCliente();
  const [modo, setModo] = useState<ModoTeste>('mensagem');
  const [texto, setTexto] = useState('Quanto custa?');
  const [conversaId, setConversaId] = useState<Id | null>(null);
  const [mensagemId, setMensagemId] = useState<Id | null>(null);
  const [entrada, setEntrada] = useState('{\n  "pergunta": "Quanto custa?"\n}');
  const [eventoTipo, setEventoTipo] = useState('etiqueta');
  const [eventoDados, setEventoDados] = useState('{}');
  const [iaSimulada, setIaSimulada] = useState(!props.chaveConfigurada);
  const [escolheuIA, setEscolheuIA] = useState(false);
  // Padrão: IA real se há chave; simulada se não há (até o usuário escolher).
  useEffect(() => {
    if (!escolheuIA) setIaSimulada(!props.chaveConfigurada);
  }, [props.chaveConfigurada, escolheuIA]);
  const [aoVivo, setAoVivo] = useState<AcaoRegistrada[]>([]);
  const [resultado, setResultado] = useState<ExecucaoDetalhe | null>(null);

  const testar = useMutation({
    mutationFn: async () => {
      const pedido = montarPedidoTeste(modo, { texto, conversaId, mensagemId, entrada, eventoTipo, eventoDados, iaSimulada });
      await props.salvarAntes();
      return cliente.testarAutomacao(props.automacaoId, pedido);
    },
    onMutate: () => {
      setAoVivo([]);
      setResultado(null);
    },
    onSuccess: (r) => setResultado(r.execucao),
  });

  // Ações da simulação chegando enquanto roda (eventos `automacao.execucao.*`).
  useEventoMotor((evento) => {
    if (!testar.isPending) return;
    if (
      (evento.tipo === 'automacao.execucao.iniciada' || evento.tipo === 'automacao.execucao.atualizada') &&
      evento.dados.automacao_id === props.automacaoId &&
      evento.dados.origem === 'teste'
    ) {
      setAoVivo(evento.dados.acoes);
    }
  });

  return (
    <aside className="painel-teste" aria-label="Testar">
      <h2>
        <FlaskConical size={16} aria-hidden="true" /> Testar
      </h2>
      <p className="texto-secundario">Modo simulação: lê dados reais, mas nada é enviado nem alterado.</p>
      <div className="filtros" role="tablist" aria-label="Tipo de teste">
        {(Object.keys(ROTULO_MODO) as ModoTeste[]).map((m) => (
          <button key={m} type="button" role="tab" aria-selected={modo === m} className={`chip${modo === m ? ' ativo' : ''}`} onClick={() => setModo(m)}>
            {ROTULO_MODO[m]}
          </button>
        ))}
      </div>
      {modo === 'mensagem' ? (
        <label className="campo">
          <span>Mensagem recebida (aoReceberMensagem)</span>
          <textarea rows={3} value={texto} onChange={(e) => setTexto(e.target.value)} />
        </label>
      ) : modo === 'conversa' ? (
        <EscolherMensagem conversaId={conversaId} mensagemId={mensagemId} aoConversa={setConversaId} aoMensagem={setMensagemId} />
      ) : modo === 'entrada' ? (
        <label className="campo">
          <span>Entrada (aoExecutar)</span>
          <textarea className="campo-codigo" rows={5} value={entrada} onChange={(e) => setEntrada(e.target.value)} />
        </label>
      ) : (
        <>
          <label className="campo">
            <span>Tipo do evento (aoEvento)</span>
            <input className="campo-codigo" value={eventoTipo} onChange={(e) => setEventoTipo(e.target.value)} />
          </label>
          <label className="campo">
            <span>Dados (JSON)</span>
            <textarea className="campo-codigo" rows={4} value={eventoDados} onChange={(e) => setEventoDados(e.target.value)} />
          </label>
        </>
      )}
      <label className="caixa">
        <input
          type="checkbox"
          checked={iaSimulada}
          onChange={(e) => {
            setEscolheuIA(true);
            setIaSimulada(e.target.checked);
          }}
        />
        IA simulada (respostas fixas, sem custo nem rede)
      </label>
      {!iaSimulada && !props.chaveConfigurada ? (
        <FaixaAviso tipo="aviso">Sem chave da Anthropic, chamadas a ctx.ia falham. Configure em Ajustes → IA ou use a IA simulada.</FaixaAviso>
      ) : null}
      <button type="button" className="botao" disabled={testar.isPending} onClick={() => testar.mutate()}>
        {testar.isPending ? 'Executando…' : 'Testar'}
      </button>
      {testar.error ? <FaixaAviso tipo="erro">{textoErro(testar.error)}</FaixaAviso> : null}
      {testar.isPending ? (
        <div role="status" aria-live="polite">
          <h3>Ações até agora</h3>
          <ListaAcoesRegistradas acoes={aoVivo} />
        </div>
      ) : null}
      {resultado ? <ResultadoExecucao execucao={resultado} compacto /> : null}
    </aside>
  );
}
