// Gera os ícones do app sem dependências externas (PNG escrito à mão com zlib) — 003: logotipo da
// landing (tecla âmbar com "Z"), mesmo desenho de componentes/Logo.tsx:
// - resources/icone.png (1024×1024): ícone do app (electron-builder converte para .icns);
// - resources/bandejaTemplate.png e bandejaTemplate@2x.png: ícone "template" da barra de menu.
// Uso: node app/scripts/gerar-icones.mjs
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { crc32, deflateSync } from 'node:zlib';

const pasta = join(dirname(fileURLToPath(import.meta.url)), '..', 'resources');
mkdirSync(pasta, { recursive: true });

function png(largura, altura, pixels) {
  const linhas = Buffer.alloc((largura * 4 + 1) * altura);
  for (let y = 0; y < altura; y++) {
    linhas[y * (largura * 4 + 1)] = 0;
    pixels.copy(linhas, y * (largura * 4 + 1) + 1, y * largura * 4, (y + 1) * largura * 4);
  }
  const bloco = (tipo, dados) => {
    const tamanho = Buffer.alloc(4);
    tamanho.writeUInt32BE(dados.length);
    const corpo = Buffer.concat([Buffer.from(tipo, 'ascii'), dados]);
    const crc = Buffer.alloc(4);
    crc.writeUInt32BE(crc32(corpo) >>> 0);
    return Buffer.concat([tamanho, corpo, crc]);
  };
  const cabecalho = Buffer.alloc(13);
  cabecalho.writeUInt32BE(largura, 0);
  cabecalho.writeUInt32BE(altura, 4);
  cabecalho[8] = 8; // bits
  cabecalho[9] = 6; // RGBA
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    bloco('IHDR', cabecalho),
    bloco('IDAT', deflateSync(linhas, { level: 9 })),
    bloco('IEND', Buffer.alloc(0)),
  ]);
}

/** Desenha com supersampling: `cor(x, y)` em coordenadas 0..1 → [r,g,b,a] ou null. */
function desenhar(tamanho, cor, amostras = 4) {
  const pixels = Buffer.alloc(tamanho * tamanho * 4);
  for (let y = 0; y < tamanho; y++) {
    for (let x = 0; x < tamanho; x++) {
      let r = 0, g = 0, b = 0, a = 0;
      for (let sy = 0; sy < amostras; sy++) {
        for (let sx = 0; sx < amostras; sx++) {
          const c = cor((x + (sx + 0.5) / amostras) / tamanho, (y + (sy + 0.5) / amostras) / tamanho);
          if (!c) continue;
          const alfa = c[3] / 255;
          r += c[0] * alfa; g += c[1] * alfa; b += c[2] * alfa; a += alfa;
        }
      }
      const n = amostras * amostras;
      const i = (y * tamanho + x) * 4;
      pixels[i] = a > 0 ? Math.round(r / a) : 0;
      pixels[i + 1] = a > 0 ? Math.round(g / a) : 0;
      pixels[i + 2] = a > 0 ? Math.round(b / a) : 0;
      pixels[i + 3] = Math.round((a / n) * 255);
    }
  }
  return pixels;
}

function retanguloArredondado(x, y, x0, y0, x1, y1, raio) {
  if (x < x0 || x > x1 || y < y0 || y > y1) return false;
  const cx = Math.min(Math.max(x, x0 + raio), x1 - raio);
  const cy = Math.min(Math.max(y, y0 + raio), y1 - raio);
  return (x - cx) ** 2 + (y - cy) ** 2 <= raio ** 2;
}

// ---------------------------------------------------------------- logotipo (003, research R7)
// Geometria de zapdesk-site/src/app/icon.svg, viewBox 32:
//   <rect 32×32 rx 8 #C98500/> (profundidade) + <rect 32×29 rx 8 #FFB020/> (corpo)
//   <path d="M9 8h14L14.5 14.5H20L9 21h14" stroke #121417 width 3, junção em quina, pontas retas/>
// Cores literais SÓ aqui (exceção 4 de contracts/tokens.md §4): espelham --tecla-sombra, --sinal, --tinta.
const COR_PROFUNDIDADE = [0xc9, 0x85, 0x00, 255];
const COR_CORPO = [0xff, 0xb0, 0x20, 255];
const COR_Z = [0x12, 0x14, 0x17, 255];
const Z = [
  [9, 8],
  [23, 8],
  [14.5, 14.5],
  [20, 14.5],
  [9, 21],
  [23, 21],
];

function noTriangulo(px, py, a, b, c) {
  const s = (p1, p2) => (px - p2[0]) * (p1[1] - p2[1]) - (p1[0] - p2[0]) * (py - p2[1]);
  const d1 = s(a, b), d2 = s(b, c), d3 = s(c, a);
  return !((d1 < 0 || d2 < 0 || d3 < 0) && (d1 > 0 || d2 > 0 || d3 > 0));
}

/** Ponto (em unidades do viewBox) dentro do traço do "Z": segmentos com pontas retas + quinas em miter. */
function noZ(x, y, largura = 3) {
  const m = largura / 2;
  for (let i = 0; i < Z.length - 1; i++) {
    const [ax, ay] = Z[i];
    const [bx, by] = Z[i + 1];
    const dx = bx - ax, dy = by - ay;
    const l2 = dx * dx + dy * dy;
    const t = ((x - ax) * dx + (y - ay) * dy) / l2;
    if (t < 0 || t > 1) continue;
    const dist = Math.abs((x - ax) * dy - (y - ay) * dx) / Math.sqrt(l2);
    if (dist <= m) return true;
  }
  // quinas (miter): triângulos entre o vértice, as bordas externas e a ponta do miter
  for (let i = 1; i < Z.length - 1; i++) {
    const p = Z[i], a = Z[i - 1], b = Z[i + 1];
    const d1 = [p[0] - a[0], p[1] - a[1]], d2 = [b[0] - p[0], b[1] - p[1]];
    const n = (d) => { const l = Math.hypot(d[0], d[1]); return [-d[1] / l, d[0] / l]; };
    let n1 = n(d1), n2 = n(d2);
    const cruz = d1[0] * d2[1] - d1[1] * d2[0];
    const lado = cruz > 0 ? -1 : 1; // lado externo da curva
    n1 = [n1[0] * lado, n1[1] * lado];
    n2 = [n2[0] * lado, n2[1] * lado];
    const bis = [n1[0] + n2[0], n1[1] + n2[1]];
    const lb = Math.hypot(bis[0], bis[1]);
    const cosMeio = (n1[0] * bis[0] + n1[1] * bis[1]) / lb;
    const comp = m / cosMeio;
    if (comp / m > 10) continue; // miterlimit 10 (não acontece neste desenho)
    const ponta = [p[0] + (bis[0] / lb) * comp, p[1] + (bis[1] / lb) * comp];
    const e1 = [p[0] + n1[0] * m, p[1] + n1[1] * m];
    const e2 = [p[0] + n2[0] * m, p[1] + n2[1] * m];
    if (noTriangulo(x, y, p, e1, ponta) || noTriangulo(x, y, p, ponta, e2)) return true;
  }
  return false;
}

/** Tecla colorida em coordenadas do viewBox (0..32). */
function tecla(u, v) {
  if (!retanguloArredondado(u, v, 0, 0, 32, 32, 8)) return null;
  if (noZ(u, v)) return COR_Z;
  if (retanguloArredondado(u, v, 0, 0, 32, 29, 8)) return COR_CORPO;
  return COR_PROFUNDIDADE;
}

// Ícone do app: grade de ícones do macOS — corpo 824×824 centrado num quadro 1024 (margem 100),
// transparente fora.
const MARGEM = 100 / 1024;
const LADO = 824 / 1024;
const icone = desenhar(
  1024,
  (x, y) => tecla(((x - MARGEM) / LADO) * 32, ((y - MARGEM) / LADO) * 32),
  4,
);
writeFileSync(join(pasta, 'icone.png'), png(1024, 1024, icone));

// Ícone template da barra de menu: silhueta preta da tecla (sem a faixa de profundidade) com o "Z"
// vazado (alfa 0); o macOS pinta no claro/escuro. Em 16 px o traço engrossa um pouco (≥ 1,6 px).
for (const [tamanho, nome, traco] of [
  [16, 'bandejaTemplate.png', 3.4],
  [32, 'bandejaTemplate@2x.png', 3.2],
]) {
  const pixels = desenhar(
    tamanho,
    (x, y) => {
      const u = x * 32, v = y * 32;
      if (!retanguloArredondado(u, v, 1, 1, 31, 31, 7)) return null;
      if (noZ(u, v, traco)) return null;
      return [0, 0, 0, 255];
    },
    8,
  );
  writeFileSync(join(pasta, nome), png(tamanho, tamanho, pixels));
}

console.log(`Ícones gerados em ${pasta}`);
