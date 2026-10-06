// Editor de código (T105) — o único módulo que carrega o Monaco (importado sob demanda).
// Um model por arquivo (`file:///automacoes/<id>/<caminho>`) para o TypeScript resolver imports
// relativos; tipos da SDK e esquema do manifesto vêm do motor; erros do esbuild viram marcadores.
import Editor from '@monaco-editor/react';
import { useEffect, useRef } from 'react';

import type { ErroCompilacao, Id, SdkAutomacao } from '@zapdesk/cliente-motor';

import {
  configurarSdk,
  configurarSdkEmbutida,
  configurarTypeScript,
  definirTemaMonaco,
  descartarModels,
  fonteMono,
  garantirModel,
  linguagemDoArquivo,
  marcarErrosCompilacao,
  monaco,
  problemas,
  TEMA_MONACO,
  uriArquivo,
  type Problema,
} from '../../monaco/configurar';
import { useTemaEfetivo } from '../../util/tema';

export interface PropsCodigo {
  automacaoId: Id;
  caminho: string;
  /** Conteúdo atual de todos os arquivos (models para o TypeScript). */
  arquivos: Record<string, string>;
  sdk: SdkAutomacao | null;
  /** `GET /automacoes/sdk` falhou: usa os tipos embutidos no app. */
  sdkFalhou?: boolean;
  errosCompilacao: readonly ErroCompilacao[];
  avisosCompilacao: readonly ErroCompilacao[];
  /** Posição a revelar (clique num problema). */
  irPara: { caminho: string; linha: number; coluna: number; n: number } | null;
  aoMudar: (caminho: string, conteudo: string) => void;
  aoSalvar: () => void;
  aoProblemas: (lista: Problema[]) => void;
}

export default function Codigo(props: PropsCodigo) {
  const { automacaoId, arquivos, sdk, errosCompilacao, avisosCompilacao } = props;
  const editorRef = useRef<monaco.editor.IStandaloneCodeEditor | null>(null);
  const salvarRef = useRef(props.aoSalvar);
  salvarRef.current = props.aoSalvar;
  const problemasRef = useRef(props.aoProblemas);
  problemasRef.current = props.aoProblemas;

  useEffect(() => {
    configurarTypeScript();
  }, []);

  // Tema "tela" (tokens --tela*): relido quando o tema efetivo do app muda.
  const tema = useTemaEfetivo();
  useEffect(() => {
    definirTemaMonaco();
    monaco.editor.setTheme(TEMA_MONACO);
  }, [tema]);

  const { sdkFalhou } = props;
  useEffect(() => {
    if (sdk) configurarSdk(sdk);
    else if (sdkFalhou) configurarSdkEmbutida();
  }, [sdk, sdkFalhou]);

  // Models de todos os arquivos (os que não estão abertos também: imports relativos).
  useEffect(() => {
    for (const [caminho, conteudo] of Object.entries(arquivos)) garantirModel(automacaoId, caminho, conteudo);
    descartarModels(automacaoId, Object.keys(arquivos));
  }, [automacaoId, arquivos]);

  useEffect(() => () => descartarModels(automacaoId), [automacaoId]);

  useEffect(() => {
    marcarErrosCompilacao(automacaoId, errosCompilacao, avisosCompilacao);
  }, [automacaoId, errosCompilacao, avisosCompilacao, arquivos]);

  useEffect(() => {
    const aviso = () => problemasRef.current(problemas(automacaoId));
    const d = monaco.editor.onDidChangeMarkers(aviso);
    aviso();
    return () => d.dispose();
  }, [automacaoId]);

  const { irPara } = props;
  useEffect(() => {
    const ed = editorRef.current;
    if (!ed || !irPara || irPara.caminho !== props.caminho) return;
    ed.revealLineInCenter(irPara.linha);
    ed.setPosition({ lineNumber: irPara.linha, column: irPara.coluna });
    ed.focus();
  }, [irPara, props.caminho]);

  const caminho = props.caminho;
  return (
    <Editor
      path={uriArquivo(automacaoId, caminho).toString()}
      defaultLanguage={linguagemDoArquivo(caminho)}
      value={arquivos[caminho] ?? ''}
      theme={TEMA_MONACO}
      beforeMount={() => definirTemaMonaco()}
      loading={<div className="carregando-editor">Carregando o editor…</div>}
      options={{
        fontSize: 13,
        lineHeight: 21,
        fontFamily: fonteMono(),
        fontLigatures: false,
        padding: { top: 12, bottom: 12 },
        renderLineHighlight: 'all',
        minimap: { enabled: false },
        scrollBeyondLastLine: false,
        tabSize: 2,
        automaticLayout: true,
        fixedOverflowWidgets: true,
        wordWrap: caminho.endsWith('.md') || caminho.endsWith('.txt') ? 'on' : 'off',
      }}
      onMount={(ed) => {
        editorRef.current = ed;
        ed.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => salvarRef.current());
      }}
      onChange={(valor) => props.aoMudar(caminho, valor ?? '')}
    />
  );
}
