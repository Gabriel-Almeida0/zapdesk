// Editor de fluxo (T080): gatilho → condições → ações (com "aguardar"), opções, validação ao
// digitar (`/automacoes/validar`) com erros por campo, Salvar (Cmd+S), Ativar, Executar e Testar.
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, FlaskConical, Play, Save } from 'lucide-react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Link, useNavigate } from 'react-router';

import {
  ErroMotor,
  type Automacao,
  type DefinicaoFluxo,
  type ErroDefinicao,
  type ExecucaoDetalhe,
  type Gatilho,
  type Id,
  type Limites,
  type NovaAutomacao,
} from '@zapdesk/cliente-motor';

import { chaves } from '../../api/chaves';
import { useCliente } from '../../api/motor';
import { FaixaAviso } from '../../componentes/FaixaAviso';
import { ResultadoExecucao } from '../../componentes/ResultadoExecucao';
import { textoErro } from '../../util/formatar';
import { AbasAutomacao, BotaoExcluirAutomacao, ExecutarAutomacao, InterruptorAtiva, useAbaAutomacao } from '../EditorAutomacao';
import { Execucoes } from '../Execucoes';
import { Acoes } from './Acoes';
import { Condicoes } from './Condicoes';
import { ErrosCampo } from './campos';
import { Gatilhos } from './Gatilhos';
import { OpcoesAutomacao, type ValoresOpcoes } from './Opcoes';

export interface RascunhoFluxo {
  nome: string;
  descricao: string | null;
  opcoes: ValoresOpcoes;
  gatilhos: Gatilho[];
  definicao: DefinicaoFluxo;
  limites: Partial<Limites>;
}

export function rascunhoFluxo(a?: Automacao): RascunhoFluxo {
  if (!a) {
    return {
      nome: '',
      descricao: null,
      opcoes: { contas: null, incluir_grupos: false, prioridade: 100, conta_envio_id: null, anti_loop: null },
      gatilhos: [{ tipo: 'disparo_respondeu', disparo_id: null }],
      definicao: { versao: 1, condicoes: null, acoes: [{ id: 'acao1', tipo: 'adicionar_etiqueta', etiqueta_id: '' }] },
      limites: {},
    };
  }
  const definicao = (a.definicao as DefinicaoFluxo | null) ?? { versao: 1, condicoes: null, acoes: [] };
  return {
    nome: a.nome,
    descricao: a.descricao,
    opcoes: {
      contas: a.contas,
      incluir_grupos: a.incluir_grupos,
      prioridade: a.prioridade,
      conta_envio_id: a.conta_envio_id,
      anti_loop: a.limites.anti_loop,
    },
    gatilhos: a.gatilhos,
    definicao,
    limites: {},
  };
}

export function paraNovaAutomacaoFluxo(r: RascunhoFluxo): NovaAutomacao {
  return {
    tipo: 'fluxo',
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

/** Erros vindos do motor (`definicao_invalida`) ou `null` se o erro é de outro tipo. */
export function errosDoMotor(erro: unknown): ErroDefinicao[] | null {
  if (erro instanceof ErroMotor && erro.codigo === 'definicao_invalida') return (erro.detalhes.erros ?? []) as ErroDefinicao[];
  return null;
}

const ATRASO_VALIDAR_MS = 600;

/** Valida a definição no motor enquanto o usuário edita (erros por campo). */
export function useValidacaoAoDigitar(nova: NovaAutomacao, ativo: boolean): { erros: ErroDefinicao[]; avisos: ErroDefinicao[] } {
  const cliente = useCliente();
  const [resultado, setResultado] = useState<{ erros: ErroDefinicao[]; avisos: ErroDefinicao[] }>({ erros: [], avisos: [] });
  const json = JSON.stringify(nova);
  useEffect(() => {
    if (!ativo) return undefined;
    let vivo = true;
    const t = setTimeout(() => {
      cliente
        .validarAutomacao(JSON.parse(json) as NovaAutomacao)
        .then((r) => {
          if (vivo) setResultado({ erros: r.erros, avisos: r.avisos });
        })
        .catch((e: unknown) => {
          const erros = errosDoMotor(e);
          if (vivo && erros) setResultado({ erros, avisos: [] });
        });
    }, ATRASO_VALIDAR_MS);
    return () => {
      vivo = false;
      clearTimeout(t);
    };
  }, [cliente, json, ativo]);
  return resultado;
}

/** Cmd+S / Ctrl+S. */
export function useAtalhoSalvar(salvar: () => void): void {
  const ref = useRef(salvar);
  ref.current = salvar;
  useEffect(() => {
    const tecla = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') {
        e.preventDefault();
        ref.current();
      }
    };
    window.addEventListener('keydown', tecla);
    return () => window.removeEventListener('keydown', tecla);
  }, []);
}

function PainelTesteFluxo({ salvarAntes, automacaoId }: { salvarAntes: () => Promise<Id | null>; automacaoId: Id | null }) {
  const cliente = useCliente();
  const [texto, setTexto] = useState('Oi, tenho interesse');
  const [resultado, setResultado] = useState<ExecucaoDetalhe | null>(null);
  const testar = useMutation({
    mutationFn: async () => {
      const id = (await salvarAntes()) ?? automacaoId;
      if (!id) throw new Error('Salve a automação para testar.');
      return cliente.testarAutomacao(id, { mensagem: { texto } });
    },
    onSuccess: (r) => setResultado(r.execucao),
  });
  return (
    <section className="secao-editor" aria-labelledby="titulo-testar">
      <h2 id="titulo-testar">Testar (simulação)</h2>
      <p className="texto-secundario">Roda com uma conversa fictícia: nada é enviado nem alterado. Esperas aparecem como "aguardaria".</p>
      <label className="campo">
        <span>Mensagem de teste</span>
        <input value={texto} onChange={(e) => setTexto(e.target.value)} />
      </label>
      <button type="button" className="botao secundario" disabled={testar.isPending} onClick={() => testar.mutate()}>
        <FlaskConical size={16} aria-hidden="true" /> {testar.isPending ? 'Testando…' : 'Testar'}
      </button>
      {testar.error ? <FaixaAviso tipo="erro">{textoErro(testar.error)}</FaixaAviso> : null}
      {resultado ? <ResultadoExecucao execucao={resultado} compacto /> : null}
    </section>
  );
}

export function EditorFluxo({ automacao }: { automacao?: Automacao }) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const navegar = useNavigate();
  const [aba, setAba] = useAbaAutomacao();
  const [base, setBase] = useState(() => rascunhoFluxo(automacao));
  const [rascunho, setRascunho] = useState(base);
  const [errosSalvar, setErrosSalvar] = useState<ErroDefinicao[] | null>(null);
  const [erroAtivar, setErroAtivar] = useState<string | null>(null);
  const [executando, setExecutando] = useState(false);

  // Mudanças vindas de fora (MCP, outra janela) enquanto não há edição local.
  const versao = automacao?.versao;
  const sujo = JSON.stringify(rascunho) !== JSON.stringify(base);
  useEffect(() => {
    if (!automacao || sujo) return;
    const novo = rascunhoFluxo(automacao);
    setBase(novo);
    setRascunho(novo);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [versao]);

  const nova = useMemo(() => paraNovaAutomacaoFluxo(rascunho), [rascunho]);
  const validacao = useValidacaoAoDigitar(nova, rascunho.nome.trim().length > 0);
  const erros = errosSalvar ?? validacao.erros;
  const avisos = [...validacao.avisos, ...(automacao?.avisos ?? [])];

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
      const novo = rascunhoFluxo(a);
      setBase(novo);
      setRascunho(novo);
      if (!automacao) void navegar(`/automacoes/${a.id}`, { replace: true });
    },
    onError: (e) => setErrosSalvar(errosDoMotor(e)),
  });

  const salvarSePreciso = useCallback(async (): Promise<Id | null> => {
    if (!automacao || sujo) return (await salvar.mutateAsync()).id;
    return automacao.id;
  }, [automacao, sujo, salvar]);

  useAtalhoSalvar(() => {
    if (rascunho.nome.trim() && !salvar.isPending) salvar.mutate();
  });

  const mudar = <K extends keyof RascunhoFluxo>(chave: K, valor: RascunhoFluxo[K]) => setRascunho((r) => ({ ...r, [chave]: valor }));

  return (
    <section className="tela tela-editor">
      <header className="cabecalho-tela cabecalho-editor">
        <Link to="/automacoes" className="botao-icone" aria-label="Voltar para Automações" title="Voltar para Automações">
          <ArrowLeft size={20} />
        </Link>
        <input
          className="nome-automacao"
          aria-label="Nome da automação"
          placeholder="Nome do fluxo"
          maxLength={80}
          value={rascunho.nome}
          onChange={(e) => mudar('nome', e.target.value)}
        />
        <span className="selo info">Fluxo</span>
        {automacao ? <InterruptorAtiva automacao={automacao} antesDeAtivar={async () => Boolean(await salvarSePreciso())} aoErro={setErroAtivar} /> : null}
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
      {automacao?.desativada_motivo === 'erros_seguidos' && !automacao.ativa ? (
        <FaixaAviso tipo="aviso">Desativada sozinha depois de 5 erros seguidos. Veja as execuções, corrija e ative de novo.</FaixaAviso>
      ) : null}
      {salvar.error && !errosSalvar ? <FaixaAviso tipo="erro">{textoErro(salvar.error)}</FaixaAviso> : null}
      {errosSalvar && errosSalvar.length > 0 ? <FaixaAviso tipo="erro">Corrija os campos destacados antes de salvar.</FaixaAviso> : null}
      {aba === 'execucoes' && automacao ? (
        <div className="corpo-editor-rolavel">
          <Execucoes automacaoId={automacao.id} />
        </div>
      ) : (
        <div className="corpo-editor">
          <div className="coluna-principal">
            {avisos.length > 0 ? (
              <FaixaAviso tipo="aviso">
                {avisos.map((a) => a.mensagem).filter((m, i, l) => l.indexOf(m) === i).join(' · ')}
              </FaixaAviso>
            ) : null}
            <ErrosCampo erros={erros.filter((e) => e.caminho === 'nome' || e.caminho === 'definicao')} />
            <Gatilhos gatilhos={rascunho.gatilhos} aoMudar={(g) => mudar('gatilhos', g)} erros={erros} incluirGrupos={rascunho.opcoes.incluir_grupos} />
            <Condicoes
              condicoes={rascunho.definicao.condicoes}
              aoMudar={(c) => mudar('definicao', { ...rascunho.definicao, condicoes: c })}
              erros={erros}
            />
            <Acoes
              acoes={rascunho.definicao.acoes}
              aoMudar={(a) => mudar('definicao', { ...rascunho.definicao, acoes: a })}
              erros={erros}
              automacaoId={automacao?.id}
            />
          </div>
          <aside className="coluna-lateral">
            <OpcoesAutomacao valor={rascunho.opcoes} aoMudar={(o) => mudar('opcoes', o)} />
            <PainelTesteFluxo salvarAntes={salvarSePreciso} automacaoId={automacao?.id ?? null} />
          </aside>
        </div>
      )}
      {executando && automacao ? <ExecutarAutomacao automacao={automacao} aoFechar={() => setExecutando(false)} /> : null}
    </section>
  );
}
