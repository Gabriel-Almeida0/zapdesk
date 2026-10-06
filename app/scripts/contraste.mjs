#!/usr/bin/env node
// 003 T019 — contraste WCAG 2.1 dos pares de cor do app (data-model.md §4, research R12).
// Lê os valores DIRETO de src/renderer/estilos/tema.css (nada duplicado à mão): bloco claro
// (primeiro `:root {`), escuro pelo sistema (`@media (prefers-color-scheme: dark)`) e escuro manual
// (`:root[data-tema='escuro']`) — que precisam ser idênticos — e a "tela" escura.
// Resolve var(), color-mix(in srgb, A p%, B), rgb(r g b / a) e hsl(). Mínimos: texto 4,5 · grande 3 · ui 3.
// Uso: npm run contraste -w @zapdesk/app   (sai 1 se algum par permitido falhar)
//      node app/scripts/contraste.mjs --tudo   (lista também os pares que passaram)
import { existsSync, readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const APP = join(dirname(fileURLToPath(import.meta.url)), '..');
const RENDERER = join(APP, 'src', 'renderer');
const TEMA = join(RENDERER, 'estilos', 'tema.css');
const tudo = process.argv.includes('--tudo');

// ------------------------------------------------------------------ leitura do tema.css
const css = readFileSync(TEMA, 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');

function bloco(inicioRegex) {
  const m = inicioRegex.exec(css);
  if (!m) return null;
  let i = m.index + m[0].length;
  let prof = 1;
  const ini = i;
  while (prof > 0 && i < css.length) {
    if (css[i] === '{') prof++;
    else if (css[i] === '}') prof--;
    i++;
  }
  return css.slice(ini, i - 1);
}
function declaracoes(texto) {
  const d = {};
  for (const m of texto.matchAll(/--([\w-]+)\s*:\s*([^;]+);/g)) d[m[1]] = m[2].trim().replace(/\s+/g, ' ');
  return d;
}

const claroBruto = declaracoes(bloco(/:root\s*\{/) ?? '');
const media = bloco(/@media\s*\(prefers-color-scheme:\s*dark\)\s*\{/) ?? '';
const escuroSistema = declaracoes(/:root:not\(\[data-tema=['"]?claro['"]?\]\)\s*\{([\s\S]*?)\}/.exec(media)?.[1] ?? '');
const escuroManual = declaracoes(bloco(/:root\[data-tema=['"]?escuro['"]?\]\s*\{/) ?? '');

const problemas = [];
for (const k of new Set([...Object.keys(escuroSistema), ...Object.keys(escuroManual)])) {
  if (escuroSistema[k] !== escuroManual[k]) {
    problemas.push(`--${k}: escuro pelo sistema (${escuroSistema[k] ?? '—'}) ≠ escuro manual (${escuroManual[k] ?? '—'})`);
  }
}
if (Object.keys(claroBruto).length < 20) problemas.push('bloco claro (:root) não encontrado ou incompleto');
if (Object.keys(escuroSistema).length < 10) problemas.push('bloco escuro (@media prefers-color-scheme) não encontrado');

// ------------------------------------------------------------------ cores
function hex(h) {
  let n = h.replace('#', '');
  if (n.length === 3 || n.length === 4) n = [...n].map((c) => c + c).join('');
  const a = n.length === 8 ? parseInt(n.slice(6, 8), 16) / 255 : 1;
  return [parseInt(n.slice(0, 2), 16), parseInt(n.slice(2, 4), 16), parseInt(n.slice(4, 6), 16), a];
}
function hsl(h, s, l) {
  s /= 100;
  l /= 100;
  const k = (n) => (n + h / 30) % 12;
  const a = s * Math.min(l, 1 - l);
  const f = (n) => l - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)));
  return [f(0) * 255, f(8) * 255, f(4) * 255, 1];
}
function sobre(c, fundo) {
  const a = c[3];
  return [c[0] * a + fundo[0] * (1 - a), c[1] * a + fundo[1] * (1 - a), c[2] * a + fundo[2] * (1 - a), 1];
}
function separarArgs(s) {
  const r = [];
  let prof = 0;
  let atual = '';
  for (const ch of s) {
    if (ch === '(') prof++;
    if (ch === ')') prof--;
    if (ch === ',' && prof === 0) {
      r.push(atual.trim());
      atual = '';
    } else atual += ch;
  }
  if (atual.trim()) r.push(atual.trim());
  return r;
}

function criarResolvedor(decl) {
  const cache = {};
  const resolver = (valor, pilha = []) => {
    valor = valor.trim();
    let m;
    if ((m = /^var\(--([\w-]+)\)$/.exec(valor))) {
      const nome = m[1];
      if (pilha.includes(nome)) throw new Error(`ciclo em --${nome}`);
      if (cache[nome]) return cache[nome];
      if (!(nome in decl)) throw new Error(`token ausente: --${nome}`);
      return (cache[nome] = resolver(decl[nome], [...pilha, nome]));
    }
    if (valor.startsWith('#')) return hex(valor);
    if ((m = /^rgba?\(\s*([\d.]+)[ ,]+([\d.]+)[ ,]+([\d.]+)\s*(?:[/,]\s*([\d.]+%?))?\s*\)$/.exec(valor))) {
      const a = m[4] === undefined ? 1 : m[4].endsWith('%') ? parseFloat(m[4]) / 100 : parseFloat(m[4]);
      return [+m[1], +m[2], +m[3], a];
    }
    if ((m = /^hsla?\(\s*([\d.]+)(?:deg)?[ ,]+([\d.]+)%[ ,]+([\d.]+)%\s*\)$/.exec(valor))) return hsl(+m[1], +m[2], +m[3]);
    if ((m = /^color-mix\(\s*in srgb\s*,(.*)\)$/.exec(valor))) {
      const [a, b] = separarArgs(m[1]);
      const pa = /\s([\d.]+)%$/.exec(a);
      const pb = /\s([\d.]+)%$/.exec(b);
      const ca = resolver(a.replace(/\s[\d.]+%$/, ''), pilha);
      const cb = resolver(b.replace(/\s[\d.]+%$/, ''), pilha);
      let p = pa ? parseFloat(pa[1]) / 100 : pb ? 1 - parseFloat(pb[1]) / 100 : 0.5;
      if (b === 'transparent') return [ca[0], ca[1], ca[2], ca[3] * p];
      return [0, 1, 2, 3].map((i) => ca[i] * p + cb[i] * (1 - p));
    }
    if (valor === 'transparent') return [0, 0, 0, 0];
    throw new Error(`valor de cor não suportado: ${valor}`);
  };
  return (nome) => resolver(`var(--${nome})`);
}

const decl = {
  claro: claroBruto,
  escuro: { ...claroBruto, ...escuroSistema },
};
const cor = { claro: criarResolvedor(decl.claro), escuro: criarResolvedor(decl.escuro) };

function lum([r, g, b]) {
  const f = (c) => {
    c /= 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b);
}
function razao(a, b) {
  const [l1, l2] = [lum(a), lum(b)].sort((x, y) => y - x);
  return (l1 + 0.05) / (l2 + 0.05);
}
const MINIMO = { texto: 4.5, grande: 3, ui: 3 };
const paraHex = (c) => '#' + c.slice(0, 3).map((v) => Math.round(v).toString(16).padStart(2, '0')).join('').toUpperCase();

// ------------------------------------------------------------------ pares (data-model.md §4)
const pares = [];
const FUNDOS = ['fundo', 'fundo-2', 'superficie', 'superficie-2'];
const SUAVES = ['sinal-suave', 'coral-suave', 'ciano-suave'];
const add = (tema, frente, fundos, tipo, nota, extra = {}) => fundos.forEach((fundo) => pares.push({ tema, frente, fundo, tipo, nota, ...extra }));

for (const t of ['claro', 'escuro']) {
  add(t, 'texto', FUNDOS, 'texto', 'corpo e títulos');
  add(t, 'texto-2', FUNDOS, 'texto', 'apoio, legendas, estados neutros');
  add(t, 'sinal-texto', FUNDOS, 'texto', 'âmbar como texto/ícone/link');
  add(t, 'coral', FUNDOS, 'texto', 'falha, destrutivo');
  add(t, 'ciano', FUNDOS, 'texto', 'positivo (lido, respondeu, concluído)');
  add(t, 'linha-forte', FUNDOS, 'ui', 'borda de controle');
  add(t, 'foco', FUNDOS, 'ui', 'anel de foco');
  add(t, 'tinta', ['sinal', 'sinal-hover'], 'texto', 'texto do botão primário');
  add(t, 'texto', SUAVES, 'texto', 'texto sobre faixa suave');
  add(t, 'coral', SUAVES, 'texto', 'coral sobre faixa suave');
  add(t, 'ciano', SUAVES, 'texto', 'ciano sobre faixa suave');
  add(t, 'sinal-texto', SUAVES, 'texto', 'âmbar-texto sobre faixa suave');
  add(t, 'fundo', ['texto'], 'texto', 'chip/filtro ativo invertido');
}
add('claro', 'sinal', ['fundo'], 'texto', '--sinal como TEXTO no claro (use --sinal-texto)', { proibido: true });
add('claro', 'borda-primario', ['fundo', 'superficie'], 'ui', 'contorno do botão primário (identifica o âmbar no claro)');
add('claro', 'foco', ['sinal'], 'ui', 'anel de foco sobre botão âmbar (claro)');
add('escuro', 'foco', ['sinal'], 'ui', 'foco âmbar sobre botão âmbar no escuro (o offset cai no fundo)', { proibido: true });
for (const f of ['tela', 'tela-2']) {
  for (const [frente, nota] of [
    ['tela-texto', 'texto do código/terminal'],
    ['tela-texto-2', 'comentários'],
    ['tela-sinal', 'keyword/prompt'],
    ['tela-ciano', 'string/resultado'],
    ['tela-coral', 'número/erro'],
  ]) {
    pares.push({ tema: 'tela', frente, fundo: f, tipo: 'texto', nota });
  }
}

// ------------------------------------------------------------------ cores de dados
function paleta(arquivo, nome) {
  const caminho = join(RENDERER, arquivo);
  if (!existsSync(caminho)) return null;
  const fonte = readFileSync(caminho, 'utf8');
  const m = new RegExp(`${nome}\\s*=\\s*\\[([^\\]]*)\\]`).exec(fonte);
  if (!m) return null;
  return {
    cores: [...m[1].matchAll(/['"](#[0-9a-fA-F]{6})['"]/g)].map((x) => x[1]),
    // Antes de T035/T043 a paleta ainda é a antiga (com verde): só informativa até ganhar o marcador.
    migrada: /cores-de-dados/.test(fonte),
  };
}
const paletas = [
  ['telas/Etiquetas.tsx', 'CORES_ETIQUETA', 'B4 (T035)'],
  ['telas/Funis.tsx', 'CORES_ETAPA', 'B6 (T043)'],
];
const dados = [];
for (const [arquivo, nome, dono] of paletas) {
  const p = paleta(arquivo, nome);
  if (!p) {
    problemas.push(`${nome} não encontrada em ${arquivo}`);
    continue;
  }
  for (const c of p.cores) {
    for (const t of ['claro', 'escuro']) {
      dados.push({ tema: t, frente: c, fundo: 'superficie', tipo: 'ui', nota: `${nome} (${arquivo})`, informativo: !p.migrada, dono });
    }
  }
}
// Remetente em grupo: hsl(h 55% var(--remetente-l)) — 12 matizes.
for (const t of ['claro', 'escuro']) {
  const l = parseFloat(decl[t]['remetente-l'] ?? 'NaN');
  for (let h = 0; h < 360; h += 30) {
    for (const f of ['superficie', 'superficie-2']) {
      dados.push({ tema: t, frente: `hsl(${h} 55% ${l}%)`, cor: hsl(h, 55, l), fundo: f, tipo: 'texto', nota: 'nome do remetente em grupo' });
    }
  }
}
// Status de texto: --tela-texto sobre hsl(h 35% 30%).
for (let h = 0; h < 360; h += 30) {
  dados.push({ tema: 'tela', frente: 'tela-texto', fundoCor: hsl(h, 35, 30), fundo: `hsl(${h} 35% 30%)`, tipo: 'texto', nota: 'status de texto' });
}

// ------------------------------------------------------------------ avaliação
const resolverTema = (tema) => cor[tema === 'tela' ? 'claro' : tema];
let falhas = 0;
let informativas = 0;
const linhas = [];
for (const p of [...pares, ...dados]) {
  const r = resolverTema(p.tema);
  let fundo;
  let frente;
  try {
    fundo = p.fundoCor ?? r(p.fundo);
    frente = p.cor ?? (p.frente.startsWith('#') ? hex(p.frente) : r(p.frente));
  } catch (e) {
    problemas.push(`${p.tema} ${p.frente}/${p.fundo}: ${e.message}`);
    continue;
  }
  const fundoOpaco = sobre(fundo, resolverTema(p.tema)('fundo'));
  const frenteOpaca = sobre(frente, fundoOpaco);
  const q = razao(frenteOpaca, fundoOpaco);
  const ok = q >= MINIMO[p.tipo];
  let status = ok ? 'ok' : 'FALHA';
  if (p.proibido) status = 'proibido';
  else if (!ok && p.informativo) {
    status = `pendente ${p.dono}`;
    informativas++;
  } else if (!ok) falhas++;
  linhas.push({ ...p, q, status, frenteHex: paraHex(frenteOpaca), fundoHex: paraHex(fundoOpaco) });
}

const larg = (s, n) => String(s).padEnd(n);
console.log(`Contraste WCAG — ${TEMA.replace(APP + '/', 'app/')}\n`);
console.log(`${larg('tema', 7)}${larg('frente', 22)}${larg('fundo', 18)}${larg('tipo', 7)}${larg('razão', 8)}${larg('status', 22)}uso`);
for (const l of linhas) {
  if (!tudo && l.status === 'ok') continue;
  console.log(
    `${larg(l.tema, 7)}${larg(l.frente, 22)}${larg(l.fundo, 18)}${larg(l.tipo, 7)}${larg(l.q.toFixed(2), 8)}${larg(l.status, 22)}${l.nota}`,
  );
}
for (const p of problemas) console.log(`PROBLEMA: ${p}`);
const proibidos = linhas.filter((l) => l.status === 'proibido').length;
console.log(
  `\n${linhas.length} pares · ${falhas} falha(s) · ${proibidos} proibido(s) documentado(s) · ${informativas} pendente(s) de bloco` +
    (problemas.length ? ` · ${problemas.length} problema(s) de leitura` : ''),
);
if (!tudo) console.log('(use --tudo para listar também os pares que passaram)');
process.exit(falhas > 0 || problemas.length > 0 ? 1 : 0);
