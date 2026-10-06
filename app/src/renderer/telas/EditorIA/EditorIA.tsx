// Editor da automação de IA (T105): árvore de arquivos, abas, Monaco com tipos da SDK, salvar
// (Cmd+S e automático 800 ms após parar de digitar) + compilação com erros inline, alteração
// externa (poll de 3 s + evento), Ativar bloqueado com erro, "Rodando a versão anterior", nota do
// isolamento, "Abrir pasta no editor externo", Testar e Execuções.
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, FolderOpen, Play, Save, X } from 'lucide-react';
import { Fragment, lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router';

import type { Automacao, ErroCompilacao } from '@zapdesk/cliente-motor';

import { chaves } from '../../api/chaves';
import { useCliente } from '../../api/motor';
import { BotaoIcone } from '../../componentes/BotaoIcone';
import { Esqueleto } from '../../componentes/Esqueleto';
import { FaixaAviso } from '../../componentes/FaixaAviso';
import { Confirmar } from '../../componentes/Modal';
import type { Problema } from '../../monaco/configurar';
import { posicaoNoEditor } from '../../monaco/posicao';
import { ROTULO_PERMISSAO } from '../../util/automacoes';
import { textoErro } from '../../util/formatar';
import { AbasAutomacao, BotaoExcluirAutomacao, ExecutarAutomacao, InterruptorAtiva, useAbaAutomacao } from '../EditorAutomacao';
import { Execucoes } from '../Execucoes';
import { ArvoreArquivos } from './ArvoreArquivos';
import { PainelTeste } from './PainelTeste';
import { arquivoSujo, ATRASO_SALVAR_MS, useProjetoIA } from './projeto';

const Codigo = lazy(() => import('./Codigo'));

export const NOTA_ISOLAMENTO =
  'Cada automação roda num processo separado e só acessa o ZapDesk pela API ctx, com as permissões do automacao.json. Isso protege contra erros e acidentes, não contra código malicioso: rode só código seu ou que você revisou.';

/** Lê `entrada` e `permissoes` do manifesto (tolerante a JSON inválido durante a edição). */
export function lerManifesto(texto: string | undefined): { entrada: string; permissoes: string[] } {
  try {
    const m = JSON.parse(texto ?? '{}') as { entrada?: unknown; permissoes?: unknown };
    return {
      entrada: typeof m.entrada === 'string' && m.entrada ? m.entrada : 'index.ts',
      permissoes: Array.isArray(m.permissoes) ? m.permissoes.filter((p): p is string => typeof p === 'string') : [],
    };
  } catch {
    return { entrada: 'index.ts', permissoes: [] };
  }
}

// O motor já devolve linha e coluna 1-base (contracts/api-http.md › ErroCompilacao), como o Monaco.
function problemasDaCompilacao(erros: readonly ErroCompilacao[]): Problema[] {
  return erros.map((e) => ({ arquivo: e.arquivo, ...posicaoNoEditor(e), mensagem: e.mensagem, origem: 'esbuild', erro: true }));
}

export function EditorIA({ automacao }: { automacao: Automacao }) {
  const cliente = useCliente();
  const [aba, setAba] = useAbaAutomacao();
  const projeto = useProjetoIA(automacao.id);
  const sdk = useQuery({ queryKey: chaves.sdk, queryFn: () => cliente.obterSdkAutomacao(), staleTime: Infinity });
  const configIA = useQuery({ queryKey: chaves.configuracaoIA, queryFn: () => cliente.obterConfiguracaoIA() });
  const sistema = useQuery({ queryKey: chaves.sistema, queryFn: () => cliente.sistema() });
  const [abertos, setAbertos] = useState<string[]>([]);
  const [ativo, setAtivo] = useState<string | null>(null);
  const [problemasEditor, setProblemasEditor] = useState<Problema[] | null>(null);
  const [irPara, setIrPara] = useState<{ caminho: string; linha: number; coluna: number; n: number } | null>(null);
  const [executando, setExecutando] = useState(false);
  const [confirmarSobrescrever, setConfirmarSobrescrever] = useState(false);
  const [erroAtivar, setErroAtivar] = useState<string | null>(null);
  const [erroPasta, setErroPasta] = useState<string | null>(null);

  const { arquivos, carregando, salvar, compilar } = projeto;
  const caminhos = useMemo(() => Object.keys(arquivos).sort(), [arquivos]);
  const conteudos = useMemo(() => Object.fromEntries(Object.entries(arquivos).map(([c, a]) => [c, a.conteudo])), [arquivos]);
  const sujos = useMemo(() => new Set(caminhos.filter((c) => arquivoSujo(arquivos[c]))), [caminhos, arquivos]);
  const conflitos = caminhos.filter((c) => arquivos[c]?.conflito);
  const manifesto = lerManifesto(arquivos['automacao.json']?.conteudo);
  const protegidos = useMemo(() => new Set(['automacao.json', manifesto.entrada]), [manifesto.entrada]);

  // Abre a entrada quando o projeto termina de carregar (antes o manifesto pode não ter chegado).
  useEffect(() => {
    if (carregando || ativo || caminhos.length === 0) return;
    const inicial = caminhos.includes(manifesto.entrada) ? manifesto.entrada : (caminhos[0] ?? null);
    if (inicial) {
      setAtivo(inicial);
      setAbertos([inicial]);
    }
  }, [carregando, ativo, caminhos, manifesto.entrada]);

  // Arquivo aberto que sumiu (excluído/renomeado fora).
  useEffect(() => {
    if (carregando) return;
    setAbertos((l) => l.filter((c) => c in arquivos));
    if (ativo && !(ativo in arquivos)) setAtivo(null);
  }, [arquivos, ativo, carregando]);

  // Compila uma vez ao abrir (marcadores do esbuild).
  const compilouAoAbrir = useRef(false);
  useEffect(() => {
    if (carregando || compilouAoAbrir.current) return;
    compilouAoAbrir.current = true;
    void compilar();
  }, [carregando, compilar]);

  // Salvar automático 800 ms depois de parar de digitar (só sem conflito).
  const temSujoSemConflito = caminhos.some((c) => sujos.has(c) && !arquivos[c]?.conflito);
  useEffect(() => {
    if (!temSujoSemConflito) return undefined;
    const t = setTimeout(() => void salvar(), ATRASO_SALVAR_MS);
    return () => clearTimeout(t);
  }, [conteudos, temSujoSemConflito, salvar]);

  const salvarAgora = useCallback(() => {
    if (conflitos.length > 0) setConfirmarSobrescrever(true);
    else void salvar();
  }, [conflitos.length, salvar]);

  const abrir = (caminho: string) => {
    setAbertos((l) => (l.includes(caminho) ? l : [...l, caminho]));
    setAtivo(caminho);
  };
  const fecharAba = (caminho: string) => {
    setAbertos((l) => {
      const resto = l.filter((c) => c !== caminho);
      if (ativo === caminho) setAtivo(resto.at(-1) ?? null);
      return resto;
    });
  };

  const ia = automacao.ia;
  const errosCompilacao = projeto.compilacao?.erros ?? ia?.erros_compilacao ?? [];
  const avisosCompilacao = projeto.compilacao?.avisos ?? [];
  const problemas = problemasEditor ?? problemasDaCompilacao(errosCompilacao);
  const comErro = new Set(problemas.filter((p) => p.erro).map((p) => p.arquivo));
  const bloqueio = errosCompilacao.length > 0 ? 'Corrija os erros de compilação para ativar.' : null;
  const usaIA = manifesto.permissoes.includes('ia');
  const chaveConfigurada = configIA.data?.chave_configurada ?? false;

  return (
    <section className="tela tela-editor editor-ia">
      <header className="cabecalho-tela cabecalho-editor">
        <Link to="/automacoes" className="botao-icone" aria-label="Voltar para Automações" title="Voltar para Automações">
          <ArrowLeft size={20} />
        </Link>
        <h1 className="nome-automacao-fixo" title="O nome fica no automacao.json">
          {automacao.nome}
        </h1>
        <span className="selo info">IA (código)</span>
        <InterruptorAtiva
          automacao={automacao}
          bloqueado={bloqueio}
          antesDeAtivar={async () => {
            await salvar();
            const r = await compilar();
            return Boolean(r?.ok);
          }}
          aoErro={setErroAtivar}
        />
        <button type="button" className="botao secundario" onClick={() => setExecutando(true)}>
          <Play size={16} aria-hidden="true" /> Executar
        </button>
        <button
          type="button"
          className="botao secundario"
          disabled={!ia?.pasta}
          onClick={() => {
            setErroPasta(null);
            window.zapdesk?.abrirPastaExterna?.(ia?.pasta ?? '').catch((e: unknown) => setErroPasta(textoErro(e).replace(/^Error invoking remote method '[^']+': (Error: )?/, '')));
          }}
        >
          <FolderOpen size={16} aria-hidden="true" /> Abrir pasta no editor externo
        </button>
        <button type="button" className="botao" disabled={sujos.size === 0 || projeto.salvando} onClick={salvarAgora} title="Cmd+S">
          <Save size={16} aria-hidden="true" /> {projeto.salvando ? 'Salvando…' : sujos.size > 0 ? 'Salvar' : 'Salvo'}
        </button>
        <BotaoExcluirAutomacao automacao={automacao} />
      </header>
      <AbasAutomacao aba={aba} aoMudar={setAba} />
      {erroAtivar ? <FaixaAviso tipo="erro">{erroAtivar}</FaixaAviso> : null}
      {erroPasta ? <FaixaAviso tipo="erro">{erroPasta}</FaixaAviso> : null}
      {projeto.erro ? (
        <FaixaAviso
          tipo="erro"
          acao={
            <button type="button" className="botao-link" onClick={projeto.limparErro}>
              Fechar
            </button>
          }
        >
          {projeto.erro}
        </FaixaAviso>
      ) : null}
      {/* Só faz sentido se está ativa: inativa, nada "roda" (os erros já aparecem nos problemas). */}
      {automacao.ativa && ia?.rodando_versao_anterior ? (
        <FaixaAviso tipo="aviso">Rodando a versão anterior — corrija os erros de compilação para a nova versão valer.</FaixaAviso>
      ) : null}
      {automacao.desativada_motivo === 'erros_seguidos' && !automacao.ativa ? (
        <FaixaAviso tipo="aviso">Desativada sozinha depois de 5 erros seguidos. Veja as execuções, corrija e ative de novo.</FaixaAviso>
      ) : null}
      {sistema.data && sistema.data.runner_disponivel === false ? (
        <FaixaAviso tipo="erro">O executor das automações de IA não está disponível nesta instalação. Fluxos e chatbots continuam funcionando.</FaixaAviso>
      ) : null}
      {usaIA && configIA.data && !chaveConfigurada ? (
        <FaixaAviso tipo="aviso" acao={<Link to="/ajustes#ia">Ajustes → IA</Link>}>
          Esta automação usa IA, mas a chave da Anthropic não está configurada.
        </FaixaAviso>
      ) : null}
      {conflitos.map((c) => (
        <FaixaAviso
          key={c}
          tipo="aviso"
          acao={
            <>
              <button type="button" className="botao-link" onClick={() => void projeto.recarregar(c)}>
                Recarregar
              </button>{' '}
              <button type="button" className="botao-link" onClick={() => setConfirmarSobrescrever(true)}>
                Manter o meu
              </button>
            </>
          }
        >
          {c}: arquivo alterado fora do app.
        </FaixaAviso>
      ))}

      {aba === 'execucoes' ? (
        <div className="corpo-editor-rolavel">
          <Execucoes automacaoId={automacao.id} />
        </div>
      ) : (
        <div className="corpo-editor-ia">
          <ArvoreArquivos
            caminhos={caminhos}
            ativo={ativo}
            sujos={sujos}
            comErro={comErro}
            protegidos={protegidos}
            aoAbrir={abrir}
            aoCriar={(c) => projeto.criar(c, c.endsWith('.json') ? '{}\n' : '')}
            aoRenomear={async (de, para) => {
              await projeto.renomear(de, para);
              setAbertos((l) => l.map((x) => (x === de ? para : x)));
              if (ativo === de) setAtivo(para);
            }}
            aoExcluir={async (c) => {
              await projeto.excluir(c);
              fecharAba(c);
            }}
          />
          <div className="area-codigo">
            <div className="abas-arquivos" role="tablist" aria-label="Arquivos abertos">
              {abertos.map((c) => (
                <div key={c} className={`aba-arquivo${ativo === c ? ' ativa' : ''}`}>
                  <button type="button" role="tab" aria-selected={ativo === c} onClick={() => setAtivo(c)}>
                    {c.split('/').at(-1)}
                    {sujos.has(c) ? ' •' : ''}
                  </button>
                  <BotaoIcone rotulo={`Fechar ${c}`} className="pequeno" onClick={() => fecharAba(c)}>
                    <X size={12} />
                  </BotaoIcone>
                </div>
              ))}
              {projeto.compilando ? <span className="status-compilacao">Compilando…</span> : null}
              {!projeto.compilando && projeto.compilacao ? (
                <span className={`status-compilacao${projeto.compilacao.ok ? ' ok' : ' erro'}`}>
                  {projeto.compilacao.ok ? `Compilado (${projeto.compilacao.duracao_ms} ms)` : `${projeto.compilacao.erros.length} erro(s) de compilação`}
                </span>
              ) : null}
            </div>
            <div className="editor-codigo">
              {carregando ? (
                <Esqueleto altura={300} />
              ) : ativo && ativo in conteudos ? (
                <Suspense fallback={<div className="carregando-editor">Carregando o editor…</div>}>
                  <Codigo
                    automacaoId={automacao.id}
                    caminho={ativo}
                    arquivos={conteudos}
                    sdk={sdk.data ?? null}
                    sdkFalhou={sdk.isError}
                    errosCompilacao={errosCompilacao}
                    avisosCompilacao={avisosCompilacao}
                    irPara={irPara}
                    aoMudar={projeto.editar}
                    aoSalvar={salvarAgora}
                    aoProblemas={setProblemasEditor}
                  />
                </Suspense>
              ) : (
                <p className="texto-secundario centro">Escolha um arquivo.</p>
              )}
            </div>
            <div className="painel-problemas" aria-label="Problemas">
              <div className="painel-problemas-cabecalho">
                <strong>Problemas</strong>
                <span className="texto-secundario">
                  {problemas.length === 0 ? 'Nenhum' : `${problemas.filter((p) => p.erro).length} erro(s), ${problemas.filter((p) => !p.erro).length} aviso(s)`}
                </span>
                {sdk.isError ? <span className="texto-secundario">Tipos da API: usando a cópia do app ({textoErro(sdk.error)})</span> : null}
              </div>
              <ul>
                {problemas.slice(0, 100).map((p, i) => (
                  <li key={`${p.arquivo}-${p.linha}-${p.coluna}-${i}`}>
                    <button
                      type="button"
                      className={p.erro ? 'problema erro' : 'problema'}
                      onClick={() => {
                        abrir(p.arquivo);
                        setIrPara({ caminho: p.arquivo, linha: p.linha, coluna: p.coluna, n: Date.now() });
                      }}
                    >
                      <span className="mono">
                        {p.arquivo}:{p.linha}:{p.coluna}
                      </span>{' '}
                      {p.mensagem}
                    </button>
                  </li>
                ))}
              </ul>
            </div>
            <p className="nota-isolamento">
              {NOTA_ISOLAMENTO} Permissões:{' '}
              {manifesto.permissoes.length > 0
                ? manifesto.permissoes.map((p, i) => (
                    <Fragment key={p}>
                      {i > 0 ? ', ' : null}
                      <span className="permissao">{ROTULO_PERMISSAO[p as keyof typeof ROTULO_PERMISSAO] ?? p}</span>
                    </Fragment>
                  ))
                : 'nenhuma'}
              .
            </p>
          </div>
          <PainelTeste
            automacaoId={automacao.id}
            chaveConfigurada={chaveConfigurada}
            salvarAntes={async () => {
              if (sujos.size > 0) return salvar();
              return true;
            }}
          />
        </div>
      )}
      {executando ? <ExecutarAutomacao automacao={automacao} comEntrada aoFechar={() => setExecutando(false)} /> : null}
      {confirmarSobrescrever ? (
        <Confirmar
          titulo="Salvar por cima?"
          texto={`${conflitos.join(', ')} foi alterado fora do app (VS Code ou Claude). Salvar agora substitui essa versão pela sua.`}
          confirmar="Salvar por cima"
          perigo
          aoConfirmar={() => {
            setConfirmarSobrescrever(false);
            void salvar({ forcar: true });
          }}
          aoFechar={() => setConfirmarSobrescrever(false)}
        />
      ) : null}
    </section>
  );
}
