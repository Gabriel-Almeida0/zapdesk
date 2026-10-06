// Editor de chatbot (T114): canvas de nós conectáveis (@xyflow/react) — início, mensagem, menu,
// pergunta, condição, ação, IA, humano e fim —, painel de propriedades, posições salvas, nós com
// erro destacados pela validação do motor, gatilho (palavra-chave ou primeira mensagem), Salvar,
// Ativar e "Testar" com o chat simulado ao lado (T115).
import '@xyflow/react/dist/style.css';

import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  applyNodeChanges,
  Background,
  Controls,
  ReactFlow,
  type Connection,
  type Edge,
  type EdgeChange,
  type Node,
  type NodeChange,
} from '@xyflow/react';
import { ArrowLeft, FlaskConical, Play, Plus, Save } from 'lucide-react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router';

import type { Automacao, DefinicaoChatbot, ErroDefinicao, Gatilho, Id, NoChatbot, NovaAutomacao, TipoNoChatbot } from '@zapdesk/cliente-motor';

import { chaves } from '../../api/chaves';
import { useCliente } from '../../api/motor';
import { FaixaAviso } from '../../componentes/FaixaAviso';
import { textoErro } from '../../util/formatar';
import { useTemaEfetivo } from '../../util/tema';
import { AbasAutomacao, BotaoExcluirAutomacao, ExecutarAutomacao, InterruptorAtiva, useAbaAutomacao } from '../EditorAutomacao';
import { errosDoMotor, useAtalhoSalvar, useValidacaoAoDigitar } from '../EditorFluxo/EditorFluxo';
import { Gatilhos } from '../EditorFluxo/Gatilhos';
import { OpcoesAutomacao, type ValoresOpcoes } from '../EditorFluxo/Opcoes';
import { Execucoes } from '../Execucoes';
import { ChatSimulado } from './ChatSimulado';
import { definicaoPadrao, definirSaida, novoNo, posicoesAutomaticas, removerNos, ROTULO_NO, saidas } from './grafo';
import { ContextoCanvasBot } from './nos/contexto';
import { ICONE_NO, NoCanvas } from './nos/NoCanvas';
import { PainelNo } from './PainelNo';

export const GATILHOS_CHATBOT = ['palavra_chave', 'mensagem_recebida', 'manual'] as const;
const TIPOS_NOVOS: TipoNoChatbot[] = ['mensagem', 'menu', 'pergunta', 'condicao', 'acao', 'ia', 'humano', 'fim'];
const TIPOS_NO_CANVAS = { bot: NoCanvas };

export interface RascunhoChatbot {
  nome: string;
  descricao: string | null;
  opcoes: ValoresOpcoes;
  gatilhos: Gatilho[];
  definicao: DefinicaoChatbot;
}

export function rascunhoChatbot(a?: Automacao): RascunhoChatbot {
  if (!a) {
    return {
      nome: '',
      descricao: null,
      opcoes: { contas: null, incluir_grupos: false, prioridade: 100, conta_envio_id: null, anti_loop: null },
      gatilhos: [{ tipo: 'palavra_chave', palavras: ['orçamento'], modo: 'palavra' }],
      definicao: definicaoPadrao(),
    };
  }
  const definicao = (a.definicao as DefinicaoChatbot | null) ?? definicaoPadrao();
  const pos = posicoesAutomaticas(definicao);
  return {
    nome: a.nome,
    descricao: a.descricao,
    opcoes: { contas: a.contas, incluir_grupos: a.incluir_grupos, prioridade: a.prioridade, conta_envio_id: a.conta_envio_id, anti_loop: a.limites.anti_loop },
    gatilhos: a.gatilhos,
    // Posições preenchidas (bots criados pelo MCP podem vir sem): o canvas não fica "sujo" à toa.
    definicao: { ...definicao, nos: definicao.nos.map((n) => ({ ...n, posicao: pos.get(n.id) ?? { x: 0, y: 0 } })) },
  };
}

export function paraNovaAutomacaoChatbot(r: RascunhoChatbot): NovaAutomacao {
  return {
    tipo: 'chatbot',
    nome: r.nome.trim(),
    descricao: r.descricao,
    contas: r.opcoes.contas,
    incluir_grupos: r.opcoes.incluir_grupos,
    prioridade: r.opcoes.prioridade,
    conta_envio_id: r.opcoes.conta_envio_id,
    gatilhos: r.gatilhos,
    definicao: r.definicao,
    limites: { anti_loop: r.opcoes.anti_loop },
  };
}

function nodesDaDefinicao(def: DefinicaoChatbot): Node[] {
  const pos = posicoesAutomaticas(def);
  return def.nos.map((n) => ({ id: n.id, type: 'bot', position: pos.get(n.id) ?? { x: 0, y: 0 }, data: {}, deletable: n.id !== def.inicio }));
}

/** Arestas a partir das saídas ligadas. */
export function arestas(def: DefinicaoChatbot, noAtual: string | null): Edge[] {
  const ids = new Set(def.nos.map((n) => n.id));
  return def.nos.flatMap((n) =>
    saidas(n)
      .filter((s) => s.destino && ids.has(s.destino))
      .map((s) => ({
        id: `${n.id}|${s.id}`,
        source: n.id,
        sourceHandle: s.id,
        target: s.destino,
        targetHandle: 'entrada',
        animated: noAtual === n.id,
        className: s.opcional ? 'aresta-opcional' : undefined,
      })),
  );
}

export function EditorChatbot({ automacao }: { automacao?: Automacao }) {
  const cliente = useCliente();
  const tema = useTemaEfetivo();
  const qc = useQueryClient();
  const navegar = useNavigate();
  const [aba, setAba] = useAbaAutomacao();
  const [base, setBase] = useState(() => rascunhoChatbot(automacao));
  const [rascunho, setRascunho] = useState(base);
  const [nodes, setNodes] = useState<Node[]>(() => nodesDaDefinicao(base.definicao));
  const [selecionado, setSelecionado] = useState<string | null>(null);
  const [painel, setPainel] = useState<'no' | 'gatilho'>('gatilho');
  const [parametros] = useSearchParams();
  const [testando, setTestando] = useState(parametros.get('testar') === '1');
  const [noAtual, setNoAtual] = useState<string | null>(null);
  const [errosSalvar, setErrosSalvar] = useState<ErroDefinicao[] | null>(null);
  const [erroAtivar, setErroAtivar] = useState<string | null>(null);
  const [executando, setExecutando] = useState(false);

  // Definição com as posições atuais do canvas (é o que se salva e valida).
  const definicao = useMemo<DefinicaoChatbot>(() => {
    const pos = new Map(nodes.map((n) => [n.id, { x: Math.round(n.position.x), y: Math.round(n.position.y) }]));
    return { ...rascunho.definicao, nos: rascunho.definicao.nos.map((n) => ({ ...n, posicao: pos.get(n.id) ?? n.posicao })) };
  }, [rascunho.definicao, nodes]);
  const completo = useMemo(() => ({ ...rascunho, definicao }), [rascunho, definicao]);
  const sujo = JSON.stringify(completo) !== JSON.stringify(base);

  const versao = automacao?.versao;
  useEffect(() => {
    if (!automacao || sujo) return;
    const novo = rascunhoChatbot(automacao);
    setBase(novo);
    setRascunho(novo);
    setNodes(nodesDaDefinicao(novo.definicao));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [versao]);

  const nova = useMemo(() => paraNovaAutomacaoChatbot(completo), [completo]);
  const validacao = useValidacaoAoDigitar(nova, rascunho.nome.trim().length > 0);
  const erros = errosSalvar ?? validacao.erros;
  const errosPorNo = useMemo(() => {
    const m = new Map<string, ErroDefinicao[]>();
    for (const e of erros) if (e.no_id) m.set(e.no_id, [...(m.get(e.no_id) ?? []), e]);
    return m;
  }, [erros]);

  const salvar = useMutation({
    mutationFn: async (): Promise<Automacao> => {
      if (!automacao) return cliente.criarAutomacao(nova);
      const { tipo: _tipo, ...alteracao } = nova;
      void _tipo;
      return cliente.editarAutomacao(automacao.id, alteracao);
    },
    onMutate: () => setErrosSalvar(null),
    onSuccess: (a) => {
      qc.setQueryData(chaves.automacao(a.id), a);
      void qc.invalidateQueries({ queryKey: chaves.automacoes });
      const novo = rascunhoChatbot(a);
      setBase(novo);
      setRascunho(novo);
      if (!automacao) void navegar(`/automacoes/${a.id}${testando ? '?testar=1' : ''}`, { replace: true });
    },
    onError: (e) => setErrosSalvar(errosDoMotor(e)),
  });
  useAtalhoSalvar(() => {
    if (rascunho.nome.trim() && !salvar.isPending) salvar.mutate();
  });

  const obterAutomacaoId = useCallback(async (): Promise<Id | null> => {
    if (automacao) return automacao.id;
    if (!rascunho.nome.trim()) throw new Error('Dê um nome ao chatbot para testar.');
    return (await salvar.mutateAsync()).id;
  }, [automacao, rascunho.nome, salvar]);

  const mudarDefinicao = (d: DefinicaoChatbot) => setRascunho((r) => ({ ...r, definicao: d }));
  const mudarNo = (no: NoChatbot) => mudarDefinicao({ ...rascunho.definicao, nos: rascunho.definicao.nos.map((n) => (n.id === no.id ? no : n)) });

  const onNodesChange = useCallback(
    (mudancas: NodeChange[]) => {
      const removidos = mudancas.filter((m) => m.type === 'remove').map((m) => m.id);
      if (removidos.length > 0) {
        setRascunho((r) => ({ ...r, definicao: removerNos(r.definicao, removidos) }));
        if (selecionado && removidos.includes(selecionado)) setSelecionado(null);
      }
      setNodes((ns) => applyNodeChanges(mudancas, ns));
    },
    [selecionado],
  );

  const onEdgesChange = useCallback((mudancas: EdgeChange[]) => {
    const removidas = mudancas.filter((m) => m.type === 'remove').map((m) => m.id);
    if (removidas.length === 0) return;
    setRascunho((r) => ({
      ...r,
      definicao: {
        ...r.definicao,
        nos: r.definicao.nos.map((n) =>
          removidas.reduce((acc, id) => {
            const [origem, saida] = id.split('|');
            return origem === n.id && saida ? definirSaida(acc, saida, '') : acc;
          }, n),
        ),
      },
    }));
  }, []);

  const onConnect = useCallback((c: Connection) => {
    if (!c.sourceHandle || !c.target) return;
    setRascunho((r) => ({
      ...r,
      definicao: { ...r.definicao, nos: r.definicao.nos.map((n) => (n.id === c.source ? definirSaida(n, c.sourceHandle ?? '', c.target) : n)) },
    }));
  }, []);

  const adicionarNo = (tipo: TipoNoChatbot) => {
    const ref = nodes.find((n) => n.id === selecionado) ?? nodes.at(-1);
    const posicao = ref ? { x: ref.position.x + 320, y: ref.position.y + 40 } : { x: 0, y: 0 };
    const no = novoNo(tipo, rascunho.definicao.nos.map((n) => n.id), posicao);
    let nos = [...rascunho.definicao.nos, no];
    // Liga a primeira saída livre do nó selecionado ao novo nó.
    const anterior = rascunho.definicao.nos.find((n) => n.id === selecionado);
    const livre = anterior ? saidas(anterior).find((s) => !s.destino && !s.opcional) : undefined;
    if (anterior && livre) nos = nos.map((n) => (n.id === anterior.id ? definirSaida(n, livre.id, no.id) : n));
    mudarDefinicao({ ...rascunho.definicao, nos });
    setNodes((ns) => [...ns.map((n) => ({ ...n, selected: false })), { id: no.id, type: 'bot', position: posicao, data: {}, selected: true }]);
    setSelecionado(no.id);
    setPainel('no');
  };

  const excluirNo = (id: string) => {
    mudarDefinicao(removerNos(rascunho.definicao, [id]));
    setNodes((ns) => ns.filter((n) => n.id !== id));
    setSelecionado(null);
  };

  const contexto = useMemo(
    () => ({ nos: new Map(rascunho.definicao.nos.map((n) => [n.id, n])), erros: errosPorNo, noAtualSimulacao: noAtual, inicio: rascunho.definicao.inicio }),
    [rascunho.definicao, errosPorNo, noAtual],
  );
  const edges = useMemo(() => arestas(rascunho.definicao, noAtual), [rascunho.definicao, noAtual]);
  const noSelecionado = rascunho.definicao.nos.find((n) => n.id === selecionado) ?? null;
  const errosGerais = erros.filter((e) => !e.no_id);

  return (
    <section className="tela tela-editor editor-chatbot">
      <header className="cabecalho-tela cabecalho-editor">
        <Link to="/automacoes" className="botao-icone" aria-label="Voltar para Automações" title="Voltar para Automações">
          <ArrowLeft size={20} />
        </Link>
        <input
          className="nome-automacao"
          aria-label="Nome do chatbot"
          placeholder="Nome do chatbot"
          maxLength={80}
          value={rascunho.nome}
          onChange={(e) => setRascunho((r) => ({ ...r, nome: e.target.value }))}
        />
        <span className="selo info">Chatbot</span>
        {automacao ? (
          <InterruptorAtiva
            automacao={automacao}
            antesDeAtivar={async () => {
              if (sujo) await salvar.mutateAsync();
              return true;
            }}
            aoErro={setErroAtivar}
          />
        ) : null}
        <button type="button" className={`botao secundario${testando ? ' ativo' : ''}`} aria-pressed={testando} onClick={() => setTestando((t) => !t)}>
          <FlaskConical size={16} aria-hidden="true" /> Testar
        </button>
        {automacao ? (
          <button type="button" className="botao secundario" onClick={() => setExecutando(true)}>
            <Play size={16} aria-hidden="true" /> Executar
          </button>
        ) : null}
        <button type="button" className="botao" disabled={!rascunho.nome.trim() || salvar.isPending || (!sujo && Boolean(automacao))} onClick={() => salvar.mutate()}>
          <Save size={16} aria-hidden="true" /> {salvar.isPending ? 'Salvando…' : sujo || !automacao ? 'Salvar' : 'Salvo'}
        </button>
        {automacao ? <BotaoExcluirAutomacao automacao={automacao} /> : null}
      </header>
      {automacao ? <AbasAutomacao aba={aba} aoMudar={setAba} /> : null}
      {erroAtivar ? <FaixaAviso tipo="erro">{erroAtivar}</FaixaAviso> : null}
      {salvar.error && !errosSalvar ? <FaixaAviso tipo="erro">{textoErro(salvar.error)}</FaixaAviso> : null}
      {errosPorNo.size > 0 ? (
        <FaixaAviso tipo="erro">
          {errosPorNo.size === 1 ? '1 nó com problema' : `${errosPorNo.size} nós com problema`} (destacados em vermelho). Corrija antes de ativar.
        </FaixaAviso>
      ) : null}
      {errosGerais.length > 0 ? <FaixaAviso tipo="erro">{errosGerais.map((e) => e.mensagem).join(' · ')}</FaixaAviso> : null}
      {automacao && automacao.avisos.length > 0 ? <FaixaAviso tipo="aviso">{automacao.avisos.map((a) => a.mensagem).join(' · ')}</FaixaAviso> : null}

      {aba === 'execucoes' && automacao ? (
        <div className="corpo-editor-rolavel">
          <Execucoes automacaoId={automacao.id} />
        </div>
      ) : (
        <div className="corpo-chatbot">
          <div className="paleta-nos" role="toolbar" aria-label="Adicionar nó">
            {TIPOS_NOVOS.map((t) => (
              <button key={t} type="button" className="chip" onClick={() => adicionarNo(t)} title={`Adicionar ${ROTULO_NO[t].toLowerCase()}`}>
                <Plus size={12} aria-hidden="true" />
                {ICONE_NO[t]} {ROTULO_NO[t]}
              </button>
            ))}
          </div>
          <div className="area-chatbot">
            <div className="canvas-bot" aria-label="Fluxo do chatbot">
              <ContextoCanvasBot.Provider value={contexto}>
                <ReactFlow
                  colorMode={tema === 'escuro' ? 'dark' : 'light'}
                  nodes={nodes}
                  edges={edges}
                  nodeTypes={TIPOS_NO_CANVAS}
                  onNodesChange={onNodesChange}
                  onEdgesChange={onEdgesChange}
                  onConnect={onConnect}
                  onSelectionChange={({ nodes: sel }) => {
                    const id = sel[0]?.id ?? null;
                    setSelecionado(id);
                    if (id) setPainel('no');
                  }}
                  onPaneClick={() => setSelecionado(null)}
                  deleteKeyCode={['Backspace', 'Delete']}
                  fitView
                  fitViewOptions={{ padding: 0.2, maxZoom: 1 }}
                  minZoom={0.3}
                  proOptions={{ hideAttribution: true }}
                >
                  <Background gap={24} size={2} />
                  <Controls showInteractive={false} />
                </ReactFlow>
              </ContextoCanvasBot.Provider>
            </div>
            <aside className="lateral-chatbot">
              <div className="abas" role="tablist">
                <button type="button" role="tab" aria-selected={painel === 'no'} className={`aba${painel === 'no' ? ' ativa' : ''}`} onClick={() => setPainel('no')}>
                  {noSelecionado ? 'Nó' : 'Bot'}
                </button>
                <button
                  type="button"
                  role="tab"
                  aria-selected={painel === 'gatilho'}
                  className={`aba${painel === 'gatilho' ? ' ativa' : ''}`}
                  onClick={() => setPainel('gatilho')}
                >
                  Gatilho e opções
                </button>
              </div>
              {painel === 'no' ? (
                <PainelNo
                  no={noSelecionado}
                  definicao={rascunho.definicao}
                  erros={erros}
                  automacaoId={automacao?.id}
                  aoMudarNo={mudarNo}
                  aoMudarDefinicao={mudarDefinicao}
                  aoExcluirNo={excluirNo}
                />
              ) : (
                <>
                  <Gatilhos
                    gatilhos={rascunho.gatilhos}
                    tipos={GATILHOS_CHATBOT}
                    aoMudar={(g) => setRascunho((r) => ({ ...r, gatilhos: g }))}
                    erros={erros}
                    incluirGrupos={rascunho.opcoes.incluir_grupos}
                  />
                  <p className="texto-secundario">
                    Para iniciar só na primeira mensagem do contato, use "Mensagem recebida" com "Só a primeira mensagem". Fluxos também podem iniciar o bot.
                  </p>
                  <OpcoesAutomacao valor={rascunho.opcoes} aoMudar={(o) => setRascunho((r) => ({ ...r, opcoes: o }))} />
                </>
              )}
            </aside>
            {testando ? (
              <ChatSimulado obterAutomacaoId={obterAutomacaoId} definicao={definicao} aoNoAtual={setNoAtual} aoFechar={() => setTestando(false)} />
            ) : null}
          </div>
        </div>
      )}
      {executando && automacao ? <ExecutarAutomacao automacao={automacao} aoFechar={() => setExecutando(false)} /> : null}
    </section>
  );
}
