#!/usr/bin/env node
// 003 T020 — nenhuma cor literal fora de estilos/tema.css (FR-002, SC-002) e nada verde no tema
// (FR-003). research R13; exceções em specs/003-identidade-visual/contracts/tokens.md §4.
//
// Varre app/src/renderer/**/*.{css,ts,tsx} (comentários ignorados) e acusa:
//   - hex (#rgb, #rgba, #rrggbb, #rrggbbaa) — em CSS só nos valores de declaração; em TS/TSX só
//     dentro de strings/template strings;
//   - rgb( / rgba( / hsl( / hsla( literais.
// Exceções: estilos/tema.css inteiro; paletas de dados marcadas com `// cores-de-dados` (da linha do
// marcador até o `]` que fecha o array); `hsl(… var(--remetente-l))` (Bolha.tsx); `hsl(… 35% 30%)`
// em telas/Status.tsx. Em tema.css acusa qualquer cor de matiz verde (90°–170°, saturação > 25%).
//
// Uso: npm run tokens -w @zapdesk/app                       (tudo; sai 1 se houver ocorrência)
//      npm run tokens -w @zapdesk/app -- --arquivos estilos/crm.css,telas/Leads.tsx   (só esses)
//      npm run tokens -w @zapdesk/app -- --relatorio         (contagem por arquivo, sem falhar)
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { dirname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const APP = join(dirname(fileURLToPath(import.meta.url)), '..');
const RENDERER = join(APP, 'src', 'renderer');
const TEMA = 'estilos/tema.css';

const argv = process.argv.slice(2);
const relatorio = argv.includes('--relatorio');
const iArq = argv.indexOf('--arquivos');
const filtro = iArq >= 0 ? (argv[iArq + 1] ?? '').split(',').map((s) => s.trim().replace(/^\.?\/?(app\/)?(src\/renderer\/)?/, '')).filter(Boolean) : null;

function listar(pasta) {
  const r = [];
  for (const nome of readdirSync(pasta)) {
    const c = join(pasta, nome);
    if (statSync(c).isDirectory()) r.push(...listar(c));
    else if (/\.(css|ts|tsx)$/.test(nome) && !/\.d\.ts$/.test(nome)) r.push(c);
  }
  return r;
}

/** Troca comentários por espaços (mantém quebras de linha e posições). */
function semComentarios(texto, css) {
  let r = '';
  let i = 0;
  let aspas = null;
  while (i < texto.length) {
    const c = texto[i];
    const d = texto[i + 1];
    if (aspas) {
      r += c;
      if (c === '\\') {
        r += d ?? '';
        i += 2;
        continue;
      }
      if (c === aspas) aspas = null;
      i++;
      continue;
    }
    if (c === '"' || c === "'" || (!css && c === '`')) {
      aspas = c;
      r += c;
      i++;
      continue;
    }
    if (c === '/' && d === '*') {
      const fim = texto.indexOf('*/', i + 2);
      const ate = fim < 0 ? texto.length : fim + 2;
      r += texto.slice(i, ate).replace(/[^\n]/g, ' ');
      i = ate;
      continue;
    }
    if (!css && c === '/' && d === '/') {
      const fim = texto.indexOf('\n', i);
      const ate = fim < 0 ? texto.length : fim;
      r += ' '.repeat(ate - i);
      i = ate;
      continue;
    }
    r += c;
    i++;
  }
  return r;
}

/** Linhas cobertas por `// cores-de-dados` (do marcador até o `]` que fecha o array). */
function linhasDeDados(linhas) {
  const s = new Set();
  for (let i = 0; i < linhas.length; i++) {
    if (!/\/\/\s*cores-de-dados/.test(linhas[i])) continue;
    let prof = 0;
    let abriu = false;
    for (let j = i; j < linhas.length; j++) {
      s.add(j);
      for (const ch of linhas[j]) {
        if (ch === '[') {
          prof++;
          abriu = true;
        } else if (ch === ']') prof--;
      }
      if (abriu && prof <= 0) break;
      if (!abriu && j > i + 1) break; // marcador sem array logo abaixo: cobre só 2 linhas
    }
  }
  return s;
}

const RE_FUNCAO = /\b(rgba?|hsla?)\(/g;
const RE_HEX = /#(?:[0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{3,4})\b/g;

function trechosStrings(linha) {
  // posições [ini, fim) de strings simples/duplas/template na linha (aproximação por linha)
  const r = [];
  const re = /(['"`])(?:\\.|(?!\1).)*\1/g;
  let m;
  while ((m = re.exec(linha))) r.push([m.index, m.index + m[0].length]);
  return r;
}

function verificarArquivo(caminho) {
  const rel = relative(RENDERER, caminho);
  const css = caminho.endsWith('.css');
  const bruto = readFileSync(caminho, 'utf8');
  const linhasBrutas = bruto.split('\n');
  const dadosOk = linhasDeDados(linhasBrutas);
  const linhas = semComentarios(bruto, css).split('\n');
  const achados = [];
  let profCss = 0;
  linhas.forEach((linha, i) => {
    if (rel === TEMA) return;
    const n = i + 1;
    const permitidoDados = dadosOk.has(i);
    // funções de cor
    for (const m of linha.matchAll(RE_FUNCAO)) {
      const resto = linha.slice(m.index);
      if (/^hsla?\(/.test(resto) && /var\(--remetente-l\)/.test(resto.slice(0, 80))) continue;
      if (rel === 'telas/Status.tsx' && /^hsl\([^)]*35%\s+30%\)/.test(resto)) continue;
      if (permitidoDados) continue;
      achados.push({ arquivo: rel, linha: n, trecho: resto.slice(0, 40).trim(), tipo: m[1] });
    }
    // hex
    if (css) {
      // só dentro de valores de declaração: entre ":" e ";" com profundidade de bloco > 0
      let prof = profCss;
      const partes = [];
      let emValor = false;
      let ini = 0;
      for (let k = 0; k < linha.length; k++) {
        const ch = linha[k];
        if (ch === '{') {
          prof++;
          emValor = false;
        } else if (ch === '}') {
          if (emValor) partes.push([ini, k]);
          prof--;
          emValor = false;
        } else if (ch === ':' && prof > 0 && !emValor && !/[\w-]\($/.test(linha.slice(0, k))) {
          emValor = true;
          ini = k + 1;
        } else if (ch === ';' && emValor) {
          partes.push([ini, k]);
          emValor = false;
        }
      }
      if (emValor) partes.push([ini, linha.length]);
      // valor que continua na linha seguinte (declaração multilinha)
      if (!partes.length && prof > 0 && /^\s+[^:{}]*[,;]?\s*$/.test(linha) && !/^\s*[.#&:\w-][^:]*\{/.test(linha)) partes.push([0, linha.length]);
      profCss = prof;
      for (const [a, b] of partes) {
        for (const m of linha.slice(a, b).matchAll(RE_HEX)) {
          achados.push({ arquivo: rel, linha: n, trecho: m[0], tipo: 'hex' });
        }
      }
    } else {
      for (const [a, b] of trechosStrings(linha)) {
        for (const m of linha.slice(a, b).matchAll(RE_HEX)) {
          if (permitidoDados) continue;
          // rotas do HashRouter ("#/conversas") e âncoras não são cor
          if (/^#\//.test(linha.slice(a + 1 + (m.index ?? 0) - 1))) continue;
          achados.push({ arquivo: rel, linha: n, trecho: m[0], tipo: 'hex' });
        }
      }
    }
  });
  return achados;
}

// ------------------------------------------------------------------ verde no tema.css
function hexParaHsl(h) {
  let n = h.replace('#', '');
  if (n.length <= 4) n = [...n].map((c) => c + c).join('');
  const [r, g, b] = [0, 2, 4].map((i) => parseInt(n.slice(i, i + 2), 16) / 255);
  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  const l = (max + min) / 2;
  const d = max - min;
  if (d === 0) return { h: 0, s: 0, l };
  const s = d / (1 - Math.abs(2 * l - 1));
  let hh;
  if (max === r) hh = ((g - b) / d) % 6;
  else if (max === g) hh = (b - r) / d + 2;
  else hh = (r - g) / d + 4;
  return { h: (hh * 60 + 360) % 360, s, l };
}
function verdesNoTema() {
  const caminho = join(RENDERER, TEMA);
  const linhas = semComentarios(readFileSync(caminho, 'utf8'), true).split('\n');
  const r = [];
  linhas.forEach((linha, i) => {
    for (const m of linha.matchAll(RE_HEX)) {
      const { h, s } = hexParaHsl(m[0]);
      if (h >= 90 && h <= 170 && s > 0.25) r.push({ arquivo: TEMA, linha: i + 1, trecho: m[0], tipo: 'verde' });
    }
    for (const m of linha.matchAll(/rgba?\(\s*(\d+)[ ,]+(\d+)[ ,]+(\d+)/g)) {
      const hx = '#' + [m[1], m[2], m[3]].map((v) => (+v).toString(16).padStart(2, '0')).join('');
      const { h, s } = hexParaHsl(hx);
      if (h >= 90 && h <= 170 && s > 0.25) r.push({ arquivo: TEMA, linha: i + 1, trecho: m[0], tipo: 'verde' });
    }
    for (const m of linha.matchAll(/hsla?\(\s*([\d.]+)(?:deg)?[ ,]+([\d.]+)%/g)) {
      if (+m[1] >= 90 && +m[1] <= 170 && +m[2] > 25) r.push({ arquivo: TEMA, linha: i + 1, trecho: m[0], tipo: 'verde' });
    }
  });
  return r;
}

// ------------------------------------------------------------------ execução
let arquivos = listar(RENDERER);
if (filtro) {
  const pedidos = new Set(filtro);
  arquivos = arquivos.filter((c) => pedidos.has(relative(RENDERER, c)));
  const achados = new Set(arquivos.map((c) => relative(RENDERER, c)));
  for (const p of pedidos) if (!achados.has(p)) console.warn(`aviso: ${p} não existe em src/renderer`);
}
const todos = arquivos.flatMap(verificarArquivo);
if (!filtro || filtro.includes(TEMA)) todos.push(...verdesNoTema());

if (relatorio) {
  const porArquivo = {};
  for (const a of todos) porArquivo[a.arquivo] = (porArquivo[a.arquivo] ?? 0) + 1;
  const ordem = Object.entries(porArquivo).sort((x, y) => y[1] - x[1]);
  console.log(`Cores literais fora de ${TEMA} (relatório, não falha):\n`);
  for (const [arq, n] of ordem) console.log(`${String(n).padStart(4)}  ${arq}`);
  console.log(`\n${todos.length} ocorrência(s) em ${ordem.length} arquivo(s).`);
  process.exit(0);
}

for (const a of todos) console.log(`${a.arquivo}:${a.linha}  ${a.tipo.padEnd(5)}  ${a.trecho}`);
const escopo = filtro ? `${arquivos.length} arquivo(s) pedidos` : `${arquivos.length} arquivo(s)`;
console.log(`\n${escopo} · ${todos.length} cor(es) literal(is)/verde(s) fora do permitido.`);
process.exit(todos.length ? 1 : 0);
