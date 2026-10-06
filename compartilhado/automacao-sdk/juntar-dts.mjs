// Junta as declarações emitidas pelo tsc num `dist/index.d.ts` autossuficiente (um arquivo só,
// sem imports relativos): é o arquivo que o motor embute (.zapdesk/automacao.d.ts) e que o
// Monaco/VS Code resolvem via `paths`. Os demais .d.ts de dist/ continuam lá (inofensivos).
//
// Regras: cada módulo local é embutido uma única vez, na ordem em que aparece; linhas
// `import ... from './x.js'` (tipos locais) são removidas; `export * from './x.js'` vira o
// conteúdo de x.d.ts; `export { A, B } from './x.js'` embute x.d.ts inteiro (seus exports já
// ficam visíveis). `export {};` vazios são removidos. Imports não relativos são mantidos.
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const dist = join(dirname(fileURLToPath(import.meta.url)), 'dist');
const entrada = join(dist, 'index.d.ts');
const vistos = new Set();

const RE_LOCAL = /^\s*(import|export)\b[^;]*?\bfrom\s+['"](\.{1,2}\/[^'"]+)['"];?\s*$/;

function caminhoDts(base, especificador) {
  const semExt = especificador.replace(/\.(m?js|ts)$/, '');
  const candidato = resolve(dirname(base), `${semExt}.d.ts`);
  if (!existsSync(candidato)) throw new Error(`juntar-dts: não achei ${candidato} (de ${base})`);
  return candidato;
}

function embutir(arquivo) {
  if (vistos.has(arquivo)) return '';
  vistos.add(arquivo);
  const saida = [];
  // Junta instruções de várias linhas (ex.: `export {\n A,\n B\n} from './x.js';`).
  const texto = readFileSync(arquivo, 'utf8').replace(/\{[^{}]*\}\s*from/g, (m) => m.replace(/\s*\n\s*/g, ' '));
  for (const linha of texto.split('\n')) {
    if (/^\s*export\s*\{\s*\}\s*;?\s*$/.test(linha)) continue;
    if (linha.startsWith('//# sourceMappingURL')) continue;
    const m = RE_LOCAL.exec(linha);
    if (!m) {
      saida.push(linha);
      continue;
    }
    if (m[1] === 'export') saida.push(embutir(caminhoDts(arquivo, m[2])));
    // `import` local: os nomes já estão no mesmo escopo depois de embutidos.
  }
  return saida.join('\n');
}

const cabecalho =
  '// Tipos do SDK @zapdesk/automacao — gerado por compartilhado/automacao-sdk/juntar-dts.mjs.\n' +
  '// NÃO EDITE: altere compartilhado/automacao-sdk/src e rode `npm run sdk:sincronizar`.\n';

// O arquivo final é um SCRIPT de declarações (sem import/export no topo) com
// `declare module '@zapdesk/automacao' { … }` e os módulos de texto `*.md`/`*.txt`. Assim ele
// funciona tanto via `paths` (VS Code, tsconfig gerado pelo motor) quanto como lib extra do
// Monaco — e as declarações curinga `*.md`/`*.txt` valem (dentro de um arquivo-módulo elas
// virariam "augmentation" e seriam ignoradas).
const bruto = embutir(entrada);
const diretivas = [];
const linhas = [];
for (const linha of bruto.split('\n')) {
  if (/^\s*\/\/\/\s*<reference\b/.test(linha)) {
    if (!diretivas.includes(linha.trim())) diretivas.push(linha.trim());
    continue;
  }
  // Dentro de `declare module` o contexto já é ambiente: `declare` é proibido.
  linhas.push(linha.replace(/^(\s*)(export\s+)?declare\s+(?!global\b|module\b)/, '$1$2'));
}
const corpo = linhas
  .join('\n')
  .replace(/\n{3,}/g, '\n\n')
  .trim()
  .split('\n')
  .map((l) => (l.length > 0 ? `    ${l}` : l))
  .join('\n');

const modulosTexto = [
  '/** Arquivos `.md` importados como texto (carregador "text" do esbuild). */',
  "declare module '*.md' {",
  '    const texto: string;',
  '    export default texto;',
  '}',
  '/** Arquivos `.txt` importados como texto (carregador "text" do esbuild). */',
  "declare module '*.txt' {",
  '    const texto: string;',
  '    export default texto;',
  '}',
].join('\n');

writeFileSync(
  entrada,
  `${diretivas.join('\n')}${diretivas.length ? '\n' : ''}${cabecalho}` +
    `declare module '@zapdesk/automacao' {\n${corpo}\n}\n\n${modulosTexto}\n`,
);
