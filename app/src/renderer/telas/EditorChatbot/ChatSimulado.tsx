// Chat simulado ao lado do canvas (T115): conversa com o bot usando a definição em edição, sem
// enviar nada; mostra o nó atual (destacado no canvas), as variáveis e permite reiniciar.
import { RotateCcw, Send, X } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';

import type { DefinicaoChatbot, EstadoSessaoChatbot, Id, SaidaSimulada } from '@zapdesk/cliente-motor';

import { useCliente } from '../../api/motor';
import { BotaoIcone } from '../../componentes/BotaoIcone';
import { FaixaAviso } from '../../componentes/FaixaAviso';
import { ROTULO_ESTADO_SESSAO } from '../../util/automacoes';
import { textoErro } from '../../util/formatar';

interface Linha {
  de: 'bot' | 'contato' | 'sistema';
  texto: string;
}

function linhasDasSaidas(saidas: SaidaSimulada[]): Linha[] {
  return saidas.map((s) => ({ de: s.tipo === 'mensagem' ? 'bot' : 'sistema', texto: s.tipo === 'acao' ? `Ação: ${s.texto}` : s.texto }));
}

export function ChatSimulado(props: {
  obterAutomacaoId: () => Promise<Id | null>;
  definicao: DefinicaoChatbot;
  aoNoAtual: (id: string | null) => void;
  aoFechar: () => void;
}) {
  const cliente = useCliente();
  const [simulacao, setSimulacao] = useState<Id | null>(null);
  const [linhas, setLinhas] = useState<Linha[]>([]);
  const [variaveis, setVariaveis] = useState<Record<string, string>>({});
  const [estado, setEstado] = useState<EstadoSessaoChatbot>('ativa');
  const [noAtual, setNoAtual] = useState<string | null>(null);
  const [texto, setTexto] = useState('');
  const [ocupado, setOcupado] = useState(false);
  const [erro, setErro] = useState<string | null>(null);
  const idAtual = useRef<Id | null>(null);
  const fim = useRef<HTMLDivElement>(null);
  const { aoNoAtual, obterAutomacaoId } = props;
  const definicaoRef = useRef(props.definicao);
  definicaoRef.current = props.definicao;

  const encerrar = useCallback(() => {
    const id = idAtual.current;
    idAtual.current = null;
    if (id) void cliente.encerrarSimulador(id).catch(() => undefined);
  }, [cliente]);

  const iniciar = useCallback(async () => {
    encerrar();
    setOcupado(true);
    setErro(null);
    setLinhas([]);
    try {
      const automacaoId = await obterAutomacaoId();
      if (!automacaoId) throw new Error('Salve o chatbot para testar.');
      const r = await cliente.iniciarSimulador(automacaoId, { definicao: definicaoRef.current, ia_simulada: true });
      idAtual.current = r.simulacao_id;
      setSimulacao(r.simulacao_id);
      setLinhas(linhasDasSaidas(r.saidas));
      setVariaveis(r.variaveis);
      setEstado(r.estado);
      setNoAtual(r.no_atual);
      aoNoAtual(r.no_atual);
    } catch (e) {
      setErro(textoErro(e));
    } finally {
      setOcupado(false);
    }
  }, [cliente, encerrar, obterAutomacaoId, aoNoAtual]);

  useEffect(() => {
    void iniciar();
    return () => {
      encerrar();
      aoNoAtual(null);
    };
    // Só ao abrir o painel; "Reiniciar" usa a definição mais recente.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    fim.current?.scrollIntoView?.({ block: 'end' });
  }, [linhas]);

  const enviar = async () => {
    const t = texto.trim();
    if (!t || !simulacao || estado !== 'ativa') return;
    setTexto('');
    setLinhas((l) => [...l, { de: 'contato', texto: t }]);
    setOcupado(true);
    try {
      const r = await cliente.enviarAoSimulador(simulacao, t);
      setLinhas((l) => [...l, ...linhasDasSaidas(r.saidas)]);
      setVariaveis(r.variaveis);
      setEstado(r.estado);
      setNoAtual(r.no_atual);
      aoNoAtual(r.no_atual);
    } catch (e) {
      setErro(textoErro(e));
    } finally {
      setOcupado(false);
    }
  };

  return (
    <aside className="chat-simulado" aria-label="Chat simulado">
      <header className="chat-simulado-cabecalho">
        <strong>Testar o bot</strong>
        <span className="selo info">simulação</span>
        <BotaoIcone rotulo="Reiniciar conversa" className="pequeno" disabled={ocupado} onClick={() => void iniciar()}>
          <RotateCcw size={14} />
        </BotaoIcone>
        <BotaoIcone rotulo="Fechar teste" className="pequeno" onClick={props.aoFechar}>
          <X size={14} />
        </BotaoIcone>
      </header>
      <div className="chat-simulado-mensagens" role="log" aria-label="Mensagens simuladas" aria-live="polite">
        {linhas.map((l, i) =>
          l.de === 'sistema' ? (
            <div key={i} className="mensagem-sistema">
              <span>{l.texto}</span>
            </div>
          ) : (
            <div key={i} className={`linha-mensagem ${l.de === 'contato' ? 'minha' : 'dele'}`}>
              <div className="bolha">
                <span className="texto-mensagem">{l.texto}</span>
              </div>
            </div>
          ),
        )}
        {estado !== 'ativa' ? (
          <div className="mensagem-sistema">
            <span>Sessão encerrada: {ROTULO_ESTADO_SESSAO[estado].toLowerCase()}</span>
          </div>
        ) : null}
        <div ref={fim} />
      </div>
      {erro ? <FaixaAviso tipo="erro">{erro}</FaixaAviso> : null}
      <form
        className="chat-simulado-entrada"
        onSubmit={(e) => {
          e.preventDefault();
          void enviar();
        }}
      >
        <input
          aria-label="Mensagem do contato"
          placeholder={estado === 'ativa' ? 'Responda como o contato' : 'Reinicie para testar de novo'}
          value={texto}
          disabled={!simulacao || estado !== 'ativa'}
          onChange={(e) => setTexto(e.target.value)}
        />
        <BotaoIcone rotulo="Enviar" type="submit" disabled={ocupado || !texto.trim() || estado !== 'ativa'}>
          <Send size={16} />
        </BotaoIcone>
      </form>
      <details className="chat-simulado-estado" open>
        <summary>Estado</summary>
        <dl className="lista-definicoes em-linha">
          <dt>Nó atual</dt>
          <dd>
            <code>{noAtual ?? '—'}</code>
          </dd>
          {Object.entries(variaveis).map(([k, v]) => (
            <div key={k} className="variavel-simulada">
              <dt>
                <code>{`{${k}}`}</code>
              </dt>
              <dd>{v}</dd>
            </div>
          ))}
        </dl>
      </details>
    </aside>
  );
}
