// Monaco offline (T104): o `@monaco-editor/react` usa o monaco local (sem jsDelivr), com
// TypeScript configurado como no motor (research.md R12), tipos da SDK `@zapdesk/automacao`
// vindos de `GET /v1/automacoes/sdk` e o JSON Schema do `automacao.json`.
import './workers';

import { loader } from '@monaco-editor/react';
import * as monaco from 'monaco-editor';

import type { ErroCompilacao, Id, SdkAutomacao } from '@zapdesk/cliente-motor';

import { posicaoNoEditor } from './posicao';

// Cópia dos tipos da SDK gerada no build (mesma fonte que o motor embute): usada só se
// `GET /v1/automacoes/sdk` falhar, para o editor não ficar sem autocompletar.
import tiposSdkEmbutidos from '../../../../compartilhado/automacao-sdk/dist/index.d.ts?raw';

loader.config({ monaco });

export { monaco };

const URI_SDK = 'file:///node_modules/@zapdesk/automacao/index.d.ts';
const URI_ESQUEMA = 'file:///zapdesk/automacao.schema.json';

let versaoTipos: string | null = null;

/** Opções do TypeScript no editor (mesmas regras do compilador do motor). */
export function configurarTypeScript(): void {
  const ts = monaco.typescript;
  ts.typescriptDefaults.setCompilerOptions({
    target: ts.ScriptTarget.ESNext,
    module: ts.ModuleKind.ESNext,
    moduleResolution: ts.ModuleResolutionKind.NodeJs,
    strict: true,
    noEmit: true,
    isolatedModules: true,
    resolveJsonModule: true,
    allowNonTsExtensions: true,
    esModuleInterop: true,
    skipLibCheck: true,
    lib: ['esnext'],
  });
  ts.typescriptDefaults.setDiagnosticsOptions({ noSemanticValidation: false, noSyntaxValidation: false });
  ts.typescriptDefaults.setEagerModelSync(true);
  // O `$schema` do manifesto aponta para `.zapdesk/` (VS Code); aqui o esquema vem do motor.
  monaco.json.jsonDefaults.setDiagnosticsOptions({ validate: true, allowComments: false, enableSchemaRequest: false, schemaRequest: 'ignore' });
}

/** Tipos embutidos no app (quando o motor não entrega os dele). */
export function configurarSdkEmbutida(): void {
  configurarSdk({ versao: 'embutida', tipos: tiposSdkEmbutidos, esquema_manifesto: {} });
}

/** Carrega (uma vez por versão) os tipos da SDK e o esquema do manifesto. */
export function configurarSdk(sdk: SdkAutomacao): void {
  if (versaoTipos === `${sdk.versao}:${sdk.tipos.length}`) return;
  versaoTipos = `${sdk.versao}:${sdk.tipos.length}`;
  configurarTypeScript();
  monaco.typescript.typescriptDefaults.setExtraLibs([{ content: sdk.tipos, filePath: URI_SDK }]);
  monaco.json.jsonDefaults.setDiagnosticsOptions({
    validate: true,
    allowComments: false,
    enableSchemaRequest: false,
    schemaRequest: 'ignore',
    schemas: [{ uri: URI_ESQUEMA, fileMatch: ['**/automacao.json'], schema: sdk.esquema_manifesto }],
  });
}

// ---------------------------------------------------------------- tema "tela" (research R14)
// O Monaco não lê variáveis CSS: as cores saem dos tokens `--tela*` de tema.css em runtime (nenhum
// literal aqui). Mesmo tema nos dois modos, como o bloco de código da landing (tela escura sempre).

export const TEMA_MONACO = 'zapdesk-tela';

/** Cor de um token CSS como `#rrggbb` (o Monaco só aceita hex); `null` se não resolver. */
function corDoToken(nome: string): string | null {
  if (typeof document === 'undefined') return null;
  const sonda = document.createElement('span');
  sonda.style.display = 'none';
  sonda.style.color = `var(${nome})`;
  document.body.appendChild(sonda);
  const valor = getComputedStyle(sonda).color;
  sonda.remove();
  const m = /rgba?\(\s*([\d.]+)[,\s]+([\d.]+)[,\s]+([\d.]+)/.exec(valor);
  if (!m) return null;
  return `#${[m[1], m[2], m[3]].map((n) => Math.round(Number(n)).toString(16).padStart(2, '0')).join('')}`;
}

/** Hex + opacidade (0–1) no formato `#rrggbbaa` do Monaco. */
function comAlfa(hex: string, alfa: number): string {
  return `${hex}${Math.round(alfa * 255).toString(16).padStart(2, '0')}`;
}

/** Pilha mono do app (`--fonte-mono`: JetBrains Mono Variable). */
export function fonteMono(): string | undefined {
  if (typeof document === 'undefined') return undefined;
  return getComputedStyle(document.documentElement).getPropertyValue('--fonte-mono').trim() || undefined;
}

/** (Re)define o tema `zapdesk-tela` a partir dos tokens atuais. Chamar antes de montar e quando o tema mudar. */
export function definirTemaMonaco(): void {
  const tela = corDoToken('--tela');
  const tela2 = corDoToken('--tela-2');
  const linha = corDoToken('--tela-linha');
  const texto = corDoToken('--tela-texto');
  const texto2 = corDoToken('--tela-texto-2');
  const sinal = corDoToken('--tela-sinal');
  const ciano = corDoToken('--tela-ciano');
  const coral = corDoToken('--tela-coral');
  if (!tela || !tela2 || !linha || !texto || !texto2 || !sinal || !ciano || !coral) return;
  const s = (h: string) => h.slice(1);
  monaco.editor.defineTheme(TEMA_MONACO, {
    base: 'vs-dark',
    inherit: true,
    // Mesmas regras do realce da landing (tok-kw/str/num/com/prop).
    rules: [
      { token: '', foreground: s(texto), background: s(tela) },
      { token: 'keyword', foreground: s(sinal) },
      { token: 'storage', foreground: s(sinal) },
      { token: 'string', foreground: s(ciano) },
      { token: 'string.escape', foreground: s(ciano), fontStyle: 'bold' },
      { token: 'regexp', foreground: s(ciano) },
      { token: 'number', foreground: s(coral) },
      { token: 'comment', foreground: s(texto2), fontStyle: 'italic' },
      { token: 'identifier', foreground: s(texto) },
      { token: 'type', foreground: s(texto) },
      { token: 'type.identifier', foreground: s(texto) },
      { token: 'delimiter', foreground: s(texto) },
      { token: 'tag', foreground: s(sinal) },
      { token: 'attribute.name', foreground: s(sinal) },
      { token: 'attribute.value', foreground: s(ciano) },
      // automacao.json: chaves = prop (âmbar), valores = string (ciano), true/false/null = keyword.
      { token: 'string.key.json', foreground: s(sinal) },
      { token: 'string.value.json', foreground: s(ciano) },
      { token: 'keyword.json', foreground: s(coral) },
      { token: 'number.json', foreground: s(coral) },
      // prompt.md: títulos e listas em âmbar, código em ciano, ênfases no texto.
      { token: 'keyword.md', foreground: s(sinal), fontStyle: 'bold' },
      { token: 'variable.md', foreground: s(ciano) },
      { token: 'variable.source.md', foreground: s(ciano) },
      { token: 'string.link.md', foreground: s(ciano) },
      { token: 'strong', foreground: s(texto), fontStyle: 'bold' },
      { token: 'emphasis', foreground: s(texto), fontStyle: 'italic' },
    ],
    colors: {
      'editor.background': tela,
      'editor.foreground': texto,
      'editorCursor.foreground': sinal,
      'editor.lineHighlightBackground': tela2,
      'editor.lineHighlightBorder': tela2,
      'editorLineNumber.foreground': comAlfa(texto2, 0.55),
      'editorLineNumber.activeForeground': texto,
      'editor.selectionBackground': comAlfa(sinal, 0.28),
      'editor.inactiveSelectionBackground': comAlfa(sinal, 0.16),
      'editor.selectionHighlightBackground': comAlfa(sinal, 0.12),
      'editor.wordHighlightBackground': comAlfa(texto2, 0.16),
      'editor.findMatchBackground': comAlfa(sinal, 0.4),
      'editor.findMatchHighlightBackground': comAlfa(sinal, 0.18),
      'editorBracketMatch.background': comAlfa(sinal, 0.14),
      'editorBracketMatch.border': comAlfa(sinal, 0.6),
      // Sem colchetes coloridos: a landing deixa a pontuação na cor do texto.
      'editorBracketHighlight.foreground1': texto,
      'editorBracketHighlight.foreground2': texto,
      'editorBracketHighlight.foreground3': texto,
      'editorBracketHighlight.foreground4': texto,
      'editorBracketHighlight.foreground5': texto,
      'editorBracketHighlight.foreground6': texto,
      'editorBracketHighlight.unexpectedBracket.foreground': coral,
      'editorIndentGuide.background1': linha,
      'editorIndentGuide.activeBackground1': comAlfa(texto2, 0.5),
      'editorWhitespace.foreground': linha,
      'editorRuler.foreground': linha,
      'editorGutter.background': tela,
      'editorError.foreground': coral,
      'editorWarning.foreground': sinal,
      'editorInfo.foreground': ciano,
      'editorOverviewRuler.border': linha,
      'editorOverviewRuler.errorForeground': coral,
      'editorOverviewRuler.warningForeground': sinal,
      'editorWidget.background': tela2,
      'editorWidget.foreground': texto,
      'editorWidget.border': linha,
      'editorHoverWidget.background': tela2,
      'editorHoverWidget.border': linha,
      'editorSuggestWidget.background': tela2,
      'editorSuggestWidget.border': linha,
      'editorSuggestWidget.foreground': texto,
      'editorSuggestWidget.highlightForeground': sinal,
      'editorSuggestWidget.selectedBackground': linha,
      'editorMarkerNavigation.background': tela2,
      'input.background': tela,
      'input.border': linha,
      'focusBorder': comAlfa(sinal, 0.7),
      'scrollbarSlider.background': comAlfa(texto2, 0.18),
      'scrollbarSlider.hoverBackground': comAlfa(texto2, 0.3),
      'scrollbarSlider.activeBackground': comAlfa(texto2, 0.4),
      'scrollbar.shadow': comAlfa(tela, 0),
      'widget.shadow': comAlfa(tela, 0.5),
    },
  });
}

/** URI do model de um arquivo do projeto: `file:///automacoes/<id>/<caminho>`. */
export function uriArquivo(automacaoId: Id, caminho: string): monaco.Uri {
  return monaco.Uri.parse(`file:///automacoes/${automacaoId}/${caminho}`);
}

export function linguagemDoArquivo(caminho: string): string {
  if (caminho.endsWith('.ts')) return 'typescript';
  if (caminho.endsWith('.json')) return 'json';
  if (caminho.endsWith('.md')) return 'markdown';
  return 'plaintext';
}

/** Cria/atualiza o model de um arquivo (o TS enxerga os outros arquivos para imports relativos). */
export function garantirModel(automacaoId: Id, caminho: string, conteudo: string): monaco.editor.ITextModel {
  const uri = uriArquivo(automacaoId, caminho);
  const existente = monaco.editor.getModel(uri);
  if (existente) {
    if (existente.getValue() !== conteudo) existente.setValue(conteudo);
    return existente;
  }
  return monaco.editor.createModel(conteudo, linguagemDoArquivo(caminho), uri);
}

export function descartarModels(automacaoId: Id, manter: readonly string[] = []): void {
  const prefixo = `file:///automacoes/${automacaoId}/`;
  const manterSet = new Set(manter.map((c) => `${prefixo}${c}`));
  for (const m of monaco.editor.getModels()) {
    const uri = m.uri.toString(true);
    if (uri.startsWith(prefixo) && !manterSet.has(uri)) m.dispose();
  }
}

/** Erros do esbuild (compilação no motor) como marcadores no editor. */
export function marcarErrosCompilacao(automacaoId: Id, erros: readonly ErroCompilacao[], avisos: readonly ErroCompilacao[] = []): void {
  const porArquivo = new Map<string, monaco.editor.IMarkerData[]>();
  const adicionar = (e: ErroCompilacao, severidade: monaco.MarkerSeverity) => {
    const lista = porArquivo.get(e.arquivo) ?? [];
    const { linha, coluna } = posicaoNoEditor(e);
    lista.push({ severity: severidade, message: e.mensagem, startLineNumber: linha, startColumn: coluna, endLineNumber: linha, endColumn: coluna + 1, source: 'esbuild' });
    porArquivo.set(e.arquivo, lista);
  };
  for (const e of erros) adicionar(e, monaco.MarkerSeverity.Error);
  for (const e of avisos) adicionar(e, monaco.MarkerSeverity.Warning);
  const prefixo = `file:///automacoes/${automacaoId}/`;
  for (const m of monaco.editor.getModels()) {
    const uri = m.uri.toString(true);
    if (!uri.startsWith(prefixo)) continue;
    monaco.editor.setModelMarkers(m, 'esbuild', porArquivo.get(uri.slice(prefixo.length)) ?? []);
  }
}

export interface Problema {
  arquivo: string;
  linha: number;
  coluna: number;
  mensagem: string;
  origem: 'esbuild' | 'typescript' | 'json';
  erro: boolean;
}

/** Todos os marcadores (TS + esbuild + JSON) dos arquivos da automação. */
export function problemas(automacaoId: Id): Problema[] {
  const prefixo = `file:///automacoes/${automacaoId}/`;
  const lista: Problema[] = [];
  for (const m of monaco.editor.getModelMarkers({})) {
    const uri = m.resource.toString(true);
    if (!uri.startsWith(prefixo)) continue;
    if (m.severity < monaco.MarkerSeverity.Warning) continue;
    lista.push({
      arquivo: uri.slice(prefixo.length),
      linha: m.startLineNumber,
      coluna: m.startColumn,
      mensagem: m.message,
      origem: m.owner === 'esbuild' ? 'esbuild' : m.owner === 'json' ? 'json' : 'typescript',
      erro: m.severity === monaco.MarkerSeverity.Error,
    });
  }
  return lista.sort((a, b) => a.arquivo.localeCompare(b.arquivo) || a.linha - b.linha);
}
