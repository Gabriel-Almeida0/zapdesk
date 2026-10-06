// 003 T006 — capturas do app SEM WhatsApp real (contracts/capturas.md).
//
// Garantias: motor em modo falso (ZAPDESK_DEV_FALSO=1), pasta de dados e pasta da interface em
// mkdtemp(os.tmpdir()/zapdesk-captura-*) — nunca ~/Library/Application Support/ZapDesk —, dados
// fictícios (semente-falsa.mjs), convive com o ZapDesk instalado aberto (userData separado = trava
// de instância única separada), limpeza garantida (finally + SIGINT/SIGTERM: fecha o Electron,
// mata só processos cuja linha de comando contém a pasta temporária e apaga a pasta).
//
// Uso (da raiz do repositório, depois de `npm run compilar -w @zapdesk/app`):
//   node app/scripts/capturar.mjs --saida docs/verificacao/003/antes [--telas a,b] [--temas claro,escuro]
//        [--tamanhos 1280x820,960x620] [--comparar <dir>] [--medir] [--manter] [--escala css|dispositivo]
//        [--compilar]   (compila app/out DENTRO do lock — use sempre com agentes em paralelo)
//        [--roteiro <arquivo.mjs>]  (003 T052: depois da semente, chama o `default` do arquivo com
//                                    { app, page, ids, api, esperar, tema, tamanho, ir, foto } para
//                                    interações — teclado, arrastar, Testar; sem --telas não fotografa
//                                    a tabela)
//        [--app <dir>]  (lança outro build do app — ex.: a base de comparação; padrão: app/)
//
// Saída: <saida>/<tela>-<largura>-<tema>.png (+ <saida>/mascaras.json com as áreas ignoradas na
// comparação; + <saida>/medidas.json com --medir; + <saida>/diferencas/*.png com --comparar).
// Concorrência: um lock em os.tmpdir()/zapdesk-captura.lock impede duas capturas ao mesmo tempo
// (o app/out é compartilhado); quem chega espera até 15 min.
import { spawnSync } from 'node:child_process';
import {
  chmodSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  statSync,
  writeFileSync,
  openSync,
  closeSync,
  unlinkSync,
} from 'node:fs';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { AGORA, semear } from './semente-falsa.mjs';

const APP = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const RAIZ = resolve(APP, '..');
// Playwright não é dependência do projeto: aponte PLAYWRIGHT_PATH para uma instalação local
// (ex.: PLAYWRIGHT_PATH=/caminho/para/node_modules/playwright) ou deixe o Node resolver `playwright`.
const PLAYWRIGHT = process.env.PLAYWRIGHT_PATH ?? 'playwright';
const esperar = (ms) => new Promise((r) => setTimeout(r, ms));

// ------------------------------------------------------------------ argumentos
function lerArgs(argv) {
  const a = { temas: ['claro', 'escuro'], tamanhos: ['1280x820', '960x620'], telas: null };
  for (let i = 0; i < argv.length; i++) {
    const k = argv[i];
    const v = () => argv[++i];
    if (k === '--saida') a.saida = v();
    else if (k === '--telas') a.telas = v().split(',').map((s) => s.trim()).filter(Boolean);
    else if (k === '--temas') a.temas = v().split(',').map((s) => s.trim());
    else if (k === '--tamanhos') a.tamanhos = v().split(',').map((s) => s.trim());
    else if (k === '--comparar') a.comparar = v();
    else if (k === '--medir') a.medir = true;
    else if (k === '--manter') a.manter = true;
    else if (k === '--escala') a.escala = v();
    else if (k === '--compilar') a.compilar = true;
    else if (k === '--roteiro') a.roteiro = v();
    else if (k === '--app') a.app = v();
    else if (k === '--ajuda' || k === '-h') a.ajuda = true;
    else throw new Error(`Opção desconhecida: ${k}`);
  }
  return a;
}

const args = lerArgs(process.argv.slice(2));
if (args.ajuda || !args.saida) {
  console.log(
    'Uso: node app/scripts/capturar.mjs --saida <dir> [--telas id,…] [--temas claro,escuro] ' +
      '[--tamanhos 1280x820,960x620] [--comparar <dir>] [--medir] [--manter] [--escala css|dispositivo] [--compilar]',
  );
  process.exit(args.ajuda ? 0 : 2);
}
for (const t of args.temas) if (!['claro', 'escuro'].includes(t)) throw new Error(`Tema inválido: ${t}`);
const TAMANHOS = args.tamanhos.map((s) => {
  const m = /^(\d+)x(\d+)$/.exec(s);
  if (!m) throw new Error(`Tamanho inválido: ${s}`);
  return { largura: Number(m[1]), altura: Number(m[2]) };
});
const SAIDA = resolve(process.cwd(), args.saida);
// 'css' (padrão: 1 px por px CSS, PNGs ~4× menores — o disco está curto) ou 'dispositivo' (Retina).
const ESCALA = args.escala === 'dispositivo' ? 'device' : 'css';
const COMPARAR = args.comparar ? resolve(process.cwd(), args.comparar) : null;
// App lançado (o build em <dir>/out); a compilação (--compilar) é sempre a de app/.
const APP_LANCADO = args.app ? resolve(process.cwd(), args.app) : APP;

// ------------------------------------------------------------------ telas (ordem canônica)
// Ordem importa: telas que alteram dados (abrir conversa marca como lida, importar, gerar QR)
// ficam no fim. Rodadas só são comparáveis com o MESMO conjunto de --telas.
// `app`: 'semente' (app com dados), 'vazio', 'carregando', 'erro-motor'.
// `rota(ids)`: hash; `preparar(page, ids)`: cliques depois de navegar; `porVariante`: refaz tudo a
// cada tema/tamanho (telas com cronômetro); `mascaras`: seletores extras ignorados na comparação.
const TELAS = [
  { id: 'conversas', app: 'semente', rota: () => '#/conversas' },
  {
    id: 'busca',
    app: 'semente',
    rota: () => '#/conversas',
    preparar: async (page) => {
      await page.getByRole('button', { name: 'Buscar mensagens (⌘F)' }).click();
      await page.locator('input[aria-label="Buscar mensagens"]').fill('catálogo');
      await page.waitForSelector('.resultados-busca li', { timeout: 8000 }).catch(() => {});
    },
  },
  {
    id: 'nova-conversa',
    app: 'semente',
    rota: () => '#/conversas',
    preparar: async (page) => {
      await page.getByRole('button', { name: 'Nova conversa (⌘N)' }).click();
      await page.waitForSelector('[role="dialog"]');
    },
  },
  { id: 'contatos', app: 'semente', rota: () => '#/contatos' },
  {
    id: 'status',
    app: 'semente',
    rota: () => '#/status',
    porVariante: true,
    preparar: async (page) => {
      await page.locator('.item-status').first().click();
      await page.waitForSelector('.visualizador-status');
      await esperar(400);
    },
    rapida: true,
  },
  { id: 'disparos', app: 'semente', rota: () => '#/disparos' },
  { id: 'disparo-novo-1', app: 'semente', rota: () => '#/disparos/novo', preparar: (p) => passosDisparo(p, 0) },
  { id: 'disparo-novo-2', app: 'semente', rota: () => '#/disparos/novo', preparar: (p) => passosDisparo(p, 1) },
  { id: 'disparo-novo-3', app: 'semente', rota: () => '#/disparos/novo', preparar: (p) => passosDisparo(p, 2) },
  { id: 'disparo-novo-4', app: 'semente', rota: () => '#/disparos/novo', preparar: (p) => passosDisparo(p, 3) },
  { id: 'disparo-detalhe', app: 'semente', rota: (ids) => `#/disparos/${ids.disparoAndamento}` },
  { id: 'leads', app: 'semente', rota: () => '#/leads' },
  {
    id: 'templates',
    app: 'semente',
    rota: () => '#/templates',
    preparar: async (page) => {
      await page.locator('.item-template').first().click();
      await page.waitForSelector('.painel-detalhe .cartao');
    },
  },
  {
    id: 'etiquetas',
    app: 'semente',
    rota: () => '#/etiquetas',
    preparar: async (page) => {
      await page.getByRole('button', { name: 'Nova etiqueta' }).first().click();
      await page.waitForSelector('[role="dialog"]');
    },
  },
  { id: 'funis', app: 'semente', rota: () => '#/funis' },
  { id: 'kanban', app: 'semente', rota: (ids) => `#/funis/${ids.funil}` },
  { id: 'automacoes', app: 'semente', rota: () => '#/automacoes' },
  { id: 'editor-fluxo', app: 'semente', rota: (ids) => `#/automacoes/${ids.fluxo}` },
  {
    id: 'editor-chatbot',
    app: 'semente',
    rota: (ids) => `#/automacoes/${ids.chatbot}`,
    preparar: async (page) => {
      const no = page.locator('.react-flow__node[data-id="n3"]');
      await no.waitFor({ timeout: 10000 });
      await no.click();
      await esperar(300);
    },
  },
  {
    id: 'editor-ia',
    app: 'semente',
    rota: (ids) => `#/automacoes/${ids.ia}`,
    preparar: async (page) => {
      await page.waitForSelector('.monaco-editor .view-lines', { timeout: 20000 });
      await page.waitForSelector('.status-compilacao:not(:has-text("Compilando"))', { timeout: 20000 }).catch(() => {});
    },
    mascaras: ['.monaco-editor .cursors-layer', '.monaco-editor .scrollbar'],
  },
  { id: 'execucoes', app: 'semente', rota: (ids) => `#/execucoes/${ids.execucaoErro}` },
  { id: 'execucoes-ok', app: 'semente', rota: (ids) => `#/execucoes/${ids.execucaoOk}` },
  {
    id: 'ajustes',
    app: 'semente',
    rota: () => '#/ajustes',
    preparar: async (page) => {
      const aparencia = page.getByRole('heading', { name: 'Aparência' });
      if ((await aparencia.count()) > 0) await aparencia.first().scrollIntoViewIfNeeded();
    },
  },
  {
    id: 'modal-confirmar',
    app: 'semente',
    rota: () => '#/ajustes',
    preparar: async (page) => {
      await page.getByRole('button', { name: 'Remover conta' }).first().click();
      await page.waitForSelector('[role="dialog"]');
    },
  },
  {
    id: 'conversa',
    app: 'semente',
    rota: (ids) => `#/conversas/${ids.conversaMarina}`,
    preparar: async (page) => {
      await page.waitForSelector('.mensagens .bolha', { timeout: 10000 });
      await page.getByRole('button', { name: 'Dados do contato' }).click();
      await page.waitForSelector('.painel-contato');
    },
  },
  {
    id: 'conversa-menu',
    app: 'semente',
    rota: (ids) => `#/conversas/${ids.conversaMarina}`,
    preparar: async (page) => {
      await page.waitForSelector('.mensagens .bolha', { timeout: 10000 });
      const bolhas = page.locator('.linha-mensagem');
      const alvo = bolhas.nth((await bolhas.count()) - 2);
      await alvo.hover();
      await alvo.getByRole('button', { name: 'Ações da mensagem' }).click({ force: true });
      await page.waitForSelector('.menu-mensagem-lista');
    },
  },
  {
    id: 'leads-importar',
    app: 'semente',
    rota: () => '#/leads/importar',
    preparar: async (page, ids, ctx) => {
      // CSV fictício só com leads que já existem + 1 inválido: o relatório aparece sem criar dados.
      const csv = join(ctx.tmp, 'leads-ficticios.csv');
      writeFileSync(
        csv,
        'telefone,nome,empresa\n+55 11 90000-0101,Juliana Prado,Studio Prado\n+55 11 90000-0117,Carlos Menezes,Menezes & Filhos\n123,Linha inválida,—\n',
      );
      await page.setInputFiles('input[type="file"][aria-label="Planilha de leads"]', csv);
      await page.waitForSelector('.form-importacao select', { timeout: 10000 });
      await page.locator('.form-importacao button.botao').last().click();
      await page.waitForSelector('.relatorio-importacao', { timeout: 10000 }).catch(() => {});
    },
  },
  {
    id: 'conectar-conta',
    app: 'semente',
    rota: () => '#/contas/conectar',
    preparar: async (page) => {
      await page.getByRole('button', { name: /Gerar QR code/ }).click();
      await page.waitForSelector('img.qr', { timeout: 10000 });
    },
    mascaras: ['img.qr'],
  },
  { id: 'vazio', app: 'vazio', rota: () => '#/conversas' },
  { id: 'vazio-disparos', app: 'vazio', rota: () => '#/disparos' },
  { id: 'carregando', app: 'carregando', rota: null },
  { id: 'erro-motor', app: 'erro-motor', rota: null },
];

async function passosDisparo(page, alvo) {
  await page.waitForSelector('.lista-escolha .caixa input[type="checkbox"]', { timeout: 10000 });
  if (alvo === 0) {
    // passo 1 com 3 leads marcados
    const caixas = page.locator('.lista-escolha .caixa input[type="checkbox"]');
    for (let i = 0; i < 3; i++) await caixas.nth(i).check();
    return;
  }
  await page.locator('.barra-ferramentas .caixa input[type="checkbox"]').first().check();
  const continuar = page.getByRole('button', { name: 'Continuar', exact: true });
  await continuar.click();
  if (alvo === 1) {
    await page.locator('textarea').first().fill('Oi {nome}! Chegou a coleção de outubro. Quer ver o catálogo?');
    await esperar(900); // prévia validada no motor (atraso de 450 ms)
    return;
  }
  await page.locator('textarea').first().fill('Oi {nome}! Chegou a coleção de outubro. Quer ver o catálogo?');
  await esperar(700);
  await continuar.click();
  if (alvo === 2) return;
  await continuar.click();
  await esperar(900);
}

// Cores derivadas de ids aleatórios (ULID) mudam a cada semente: avatar da conta e fundo do status
// de texto. Só a comparação ignora essas áreas; os PNGs continuam com elas.
const MASCARAS_GLOBAIS = [
  '[data-captura-mascara]',
  '.trilho-conta .avatar',
  '.seletor-conta-botao .avatar',
  '.seletor-conta-menu .avatar',
  '.linha-conta .avatar',
  '.status-texto',
];

// ------------------------------------------------------------------ lock entre capturas
const LOCK = join(tmpdir(), 'zapdesk-captura.lock');
async function pegarLock() {
  const limite = Date.now() + 15 * 60 * 1000;
  for (;;) {
    try {
      const fd = openSync(LOCK, 'wx');
      writeFileSync(fd, String(process.pid));
      closeSync(fd);
      return;
    } catch {
      let dono = 0;
      try {
        dono = Number(readFileSync(LOCK, 'utf8'));
      } catch {}
      let vivo = false;
      try {
        if (dono) {
          process.kill(dono, 0);
          vivo = true;
        }
      } catch {}
      if (!vivo) {
        try {
          unlinkSync(LOCK);
        } catch {}
        continue;
      }
      if (Date.now() > limite) throw new Error(`Outra captura (pid ${dono}) segura ${LOCK} há 15 min.`);
      console.log(`… aguardando outra captura (pid ${dono}) terminar`);
      await esperar(5000);
    }
  }
}
function soltarLock() {
  try {
    if (Number(readFileSync(LOCK, 'utf8')) === process.pid) unlinkSync(LOCK);
  } catch {}
}

// ------------------------------------------------------------------ dependências (antes do tmp)
const requerer = createRequire(join(APP, 'package.json'));
const { _electron } = createRequire(import.meta.url)(PLAYWRIGHT);
const ELECTRON = requerer('electron');

// ------------------------------------------------------------------ limpeza
const TMP = mkdtempSync(join(tmpdir(), 'zapdesk-captura-'));
const abertos = new Set();
let limpo = false;
async function limpar() {
  if (limpo) return;
  limpo = true;
  for (const app of abertos) await fecharApp(app);
  // Só processos deste script: a linha de comando contém a pasta temporária única.
  spawnSync('pkill', ['-f', TMP]);
  await esperar(300);
  spawnSync('pkill', ['-9', '-f', TMP]);
  rmSync(TMP, { recursive: true, force: true });
  soltarLock();
}
for (const sinal of ['SIGINT', 'SIGTERM']) {
  process.on(sinal, () => {
    console.log(`\n${sinal}: limpando…`);
    void limpar().then(() => process.exit(130));
  });
}

async function fecharApp(app) {
  abertos.delete(app);
  const proc = app.process();
  await Promise.race([app.close().catch(() => {}), esperar(10000)]);
  if (proc.exitCode === null && proc.signalCode === null) {
    try {
      proc.kill('SIGKILL');
    } catch {}
  }
}

// ------------------------------------------------------------------ app

const CSS_CAPTURA = `
  *, *::before, *::after { caret-color: transparent !important; transition: none !important; }
  .monaco-editor .cursors-layer { visibility: hidden !important; }
  /* barras do Monaco aparecem/somem com atraso próprio (não determinístico) */
  .monaco-editor .scrollbar, .monaco-editor .decorationsOverviewRuler { opacity: 0 !important; }
`;

// Sem flags extras de Chromium: o "antes" foi capturado assim (GPU, fator do monitor, foto
// reduzida para 1×). Testado: --force-device-scale-factor=1 e --disable-gpu não eliminam o ruído de
// antisserrilhado entre rodadas (1–30 px isolados em telas de 960) e --disable-gpu o piora.
const ARGS_CHROMIUM = [];

let contador = 0;
async function abrirApp(tipo) {
  const n = ++contador;
  const pastaDados = join(TMP, `dados-${n}`);
  const pastaInterface = join(TMP, `interface-${n}`);
  mkdirSync(pastaDados, { recursive: true });
  mkdirSync(pastaInterface, { recursive: true });
  const env = { ...process.env };
  delete env.ELECTRON_RUN_AS_NODE;
  delete env.ELECTRON_RENDERER_URL;
  Object.assign(env, {
    ZAPDESK_DEV_FALSO: '1',
    ZAPDESK_PASTA_DADOS: pastaDados,
    ZAPDESK_PASTA_INTERFACE: pastaInterface,
  });
  if (tipo === 'erro-motor') env.ZAPDESK_MOTOR_BIN = '/usr/bin/false';
  if (tipo === 'carregando') {
    const motor = join(RAIZ, 'motor', 'bin', 'zapdesk-motor');
    const wrapper = join(TMP, `motor-atrasado-${n}.sh`);
    writeFileSync(
      wrapper,
      `#!/bin/sh\n# ${TMP}\ntrap 'kill $P 2>/dev/null; exit 0' TERM INT\nsleep 120 & P=$!\nwait $P\nexec "${motor}" "$@"\n`,
    );
    chmodSync(wrapper, 0o755);
    env.ZAPDESK_MOTOR_BIN = wrapper;
  }

  const app = await _electron.launch({ executablePath: ELECTRON, args: [APP_LANCADO, `--zapdesk-captura=${TMP}`, ...ARGS_CHROMIUM], env, cwd: APP_LANCADO });
  abertos.add(app);
  const contexto = app.context();
  // Relógio do renderer fixo (horas relativas, "Ontem", "há 2 h") — só Date; timers continuam.
  await contexto.clock.setFixedTime(new Date(AGORA));
  await contexto.addInitScript((css) => {
    window.__zdCls = 0;
    try {
      new PerformanceObserver((l) => {
        for (const e of l.getEntries()) if (!e.hadRecentInput) window.__zdCls += e.value;
      }).observe({ type: 'layout-shift', buffered: true });
    } catch {}
    const por = () => {
      const s = document.createElement('style');
      s.setAttribute('data-captura', '');
      s.textContent = css;
      document.head.appendChild(s);
    };
    if (document.head) por();
    else document.addEventListener('DOMContentLoaded', por);
  }, CSS_CAPTURA);

  const redeExterna = [];
  contexto.on('request', (r) => {
    const u = r.url();
    if (!/^(file:|data:|blob:|devtools:|chrome-extension:|http:\/\/127\.0\.0\.1[:/]|http:\/\/localhost[:/])/.test(u)) redeExterna.push(u);
  });

  const page = await app.firstWindow();
  // colorScheme: null = sem emulação (o tema vem do nativeTheme, como no app de verdade).
  await page.emulateMedia({ reducedMotion: 'reduce', colorScheme: null });
  const ctx = { app, page, tipo, pastaDados, ids: {}, redeExterna, tmp: TMP, rt: null };

  if (tipo === 'semente' || tipo === 'vazio') {
    const rtCaminho = join(pastaDados, 'runtime.json');
    const fim = Date.now() + 30000;
    while (!existsSync(rtCaminho) || statSync(rtCaminho).size === 0) {
      if (Date.now() > fim) throw new Error('runtime.json não apareceu em 30 s (motor não subiu?)');
      await esperar(100);
    }
    await esperar(100);
    ctx.rt = JSON.parse(readFileSync(rtCaminho, 'utf8'));
    ctx.base = `http://127.0.0.1:${ctx.rt.porta}/v1`;
    if (tipo === 'semente') {
      ctx.ids = await semear({ base: ctx.base, token: ctx.rt.token, log: (m) => console.log(`  · ${m}`) });
      writeFileSync(join(pastaDados, 'semente.json'), JSON.stringify(ctx.ids, null, 2));
    } else {
      await apiMotor(ctx, 'PUT', '/falso/relogio', { agora: AGORA });
    }
  }
  // Recarrega: o relógio fixo, o CSS de captura e o observador de layout valem desde o início.
  await page.reload();
  await page.waitForLoadState('domcontentloaded');
  if (tipo === 'semente' || tipo === 'vazio') {
    await page.waitForSelector('.layout', { timeout: 30000 });
  }
  return ctx;
}

async function apiMotor(ctx, metodo, caminho, corpo) {
  const r = await fetch(ctx.base + caminho, {
    method: metodo,
    headers: { authorization: `Bearer ${ctx.rt.token}`, 'content-type': 'application/json' },
    body: corpo === undefined ? undefined : JSON.stringify(corpo),
  });
  if (!r.ok) throw new Error(`${metodo} ${caminho} → ${r.status}`);
  return r.json().catch(() => null);
}

async function definirTema(ctx, tema) {
  await ctx.app.evaluate(({ nativeTheme }, t) => {
    nativeTheme.themeSource = t;
  }, tema === 'escuro' ? 'dark' : 'light');
  await ctx.page.waitForFunction(
    (escuro) => matchMedia('(prefers-color-scheme: dark)').matches === escuro,
    tema === 'escuro',
    { timeout: 5000 },
  );
}

async function definirTamanho(ctx, { largura, altura }) {
  await ctx.app.evaluate(({ BrowserWindow }, [l, a]) => {
    const j = BrowserWindow.getAllWindows()[0];
    j.setContentSize(l, a);
  }, [largura, altura]);
  await ctx.page
    .waitForFunction(([l, a]) => window.innerWidth === l && window.innerHeight === a, [largura, altura], { timeout: 5000 })
    .catch(() => console.warn(`  ! janela não chegou a ${largura}x${altura}`));
}

async function navegar(ctx, tela) {
  const { page } = ctx;
  await page.keyboard.press('Escape').catch(() => {});
  await page.mouse.move(2, 2);
  if (ctx.base && ctx.tipo === 'semente') await apiMotor(ctx, 'PUT', '/falso/relogio', { agora: AGORA });
  const hash = tela.rota(ctx.ids);
  await page.evaluate((h) => {
    if (location.hash === h) location.hash = '#/__captura';
    location.hash = h;
  }, '#/__vazio');
  await esperar(50);
  await page.evaluate((h) => {
    location.hash = h;
  }, hash);
  await esperar(250);
  await esperarQuieto(page);
  if (tela.preparar) await tela.preparar(page, ctx.ids, ctx);
  // Hover não entra na foto (botão sob o mouse muda de cor). Menus abertos por clique continuam.
  await page.mouse.move(2, 2);
}

async function esperarQuieto(page) {
  await page.evaluate(() => document.fonts.ready).catch(() => {});
  await page
    .waitForFunction(
      () =>
        !document.querySelector('.esqueleto, [aria-busy="true"]') &&
        !Array.from(document.querySelectorAll('[role="status"]')).some((e) => /Carregando/.test(e.textContent ?? '')),
      undefined,
      { timeout: 8000 },
    )
    .catch(() => {});
}

async function areasMascaradas(page, seletores) {
  return page.evaluate(([sel, css]) => {
    const r = [];
    for (const s of sel) {
      for (const el of document.querySelectorAll(s)) {
        const b = el.getBoundingClientRect();
        if (b.width > 0 && b.height > 0) r.push({ x: b.x, y: b.y, w: b.width, h: b.height, s });
      }
    }
    // Durações medidas em tempo real (ex.: "65 ms", "Compilado (2 ms)") variam entre rodadas.
    const andar = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    for (let n = andar.nextNode(); n; n = andar.nextNode()) {
      if (!/\d+(?:[.,]\d+)?\s?ms\b/.test(n.textContent ?? '')) continue;
      // o pai inteiro: a largura variável do número desloca os vizinhos (ex.: "1 ok · 3 ms")
      const el = n.parentElement?.parentElement ?? n.parentElement;
      if (!el) continue;
      const b = el.getBoundingClientRect();
      if (b.width > 0 && b.height > 0) r.push({ x: b.x, y: b.y, w: b.width, h: b.height, s: 'texto:ms' });
    }
    return { areas: r, escala: css ? 1 : window.devicePixelRatio };
  }, [seletores, ESCALA === 'css']);
}

async function fotografar(page, rapida) {
  const tirar = () => page.screenshot({ animations: 'disabled', caret: 'hide', scale: ESCALA });
  let anterior = await tirar();
  if (rapida) return anterior;
  // Estável = 3 fotos seguidas iguais (barras de rolagem e imagens assíncronas assentam).
  let iguais = 0;
  for (let i = 0; i < 15; i++) {
    await esperar(250);
    const atual = await tirar();
    iguais = atual.equals(anterior) ? iguais + 1 : 0;
    anterior = atual;
    if (iguais >= 2) return atual;
  }
  console.warn('  ! tela não estabilizou em 10 tentativas; usando a última');
  return anterior;
}

async function medir(ctx) {
  return ctx.page.evaluate(() => {
    const listas = {};
    for (const l of document.querySelectorAll('.lista-virtual')) {
      const nome = l.getAttribute('aria-label') ?? l.className;
      listas[nome] = Array.from(l.querySelectorAll(':scope > div > [role="listitem"]')).map((i) =>
        Math.round(i.getBoundingClientRect().height * 100) / 100,
      );
    }
    const fontes = [];
    document.fonts.forEach((f) => fontes.push({ familia: f.family.replace(/"/g, ''), peso: f.weight, estilo: f.style, status: f.status }));
    return { alturasItens: listas, layoutShift: window.__zdCls ?? null, fontes };
  });
}

// Comparação no próprio Electron (nativeImage.toBitmap), sem dependência nova.
async function comparar(ctx, arquivoA, arquivoB, mascaras, saidaDiff) {
  const r = await ctx.app.evaluate(({ nativeImage }, { a, b, mascaras: m }) => {
    const ia = nativeImage.createFromPath(a);
    const ib = nativeImage.createFromPath(b);
    if (ia.isEmpty() || ib.isEmpty()) return { erro: 'imagem ausente' };
    const sa = ia.getSize();
    const sb = ib.getSize();
    if (sa.width !== sb.width || sa.height !== sb.height) return { erro: `tamanhos diferentes ${sa.width}x${sa.height} × ${sb.width}x${sb.height}` };
    const ba = ia.toBitmap();
    const bb = ib.toBitmap();
    const { width: w, height: h } = sa;
    const ignorar = new Uint8Array(w * h);
    for (const q of m) {
      const x0 = Math.max(0, Math.floor(q.x * q.escala) - 1);
      const y0 = Math.max(0, Math.floor(q.y * q.escala) - 1);
      const x1 = Math.min(w, Math.ceil((q.x + q.w) * q.escala) + 1);
      const y1 = Math.min(h, Math.ceil((q.y + q.h) * q.escala) + 1);
      for (let y = y0; y < y1; y++) ignorar.fill(1, y * w + x0, y * w + x1);
    }
    let dif = 0;
    const diff = Buffer.alloc(w * h * 4);
    for (let p = 0; p < w * h; p++) {
      const o = p * 4;
      const igual = ba[o] === bb[o] && ba[o + 1] === bb[o + 1] && ba[o + 2] === bb[o + 2] && ba[o + 3] === bb[o + 3];
      if (!igual && !ignorar[p]) {
        dif++;
        diff[o] = 0; diff[o + 1] = 0; diff[o + 2] = 255; diff[o + 3] = 255; // BGRA: vermelho
      } else {
        const cinza = Math.round((ba[o] + ba[o + 1] + ba[o + 2]) / 3 / 3 + 170);
        diff[o] = cinza; diff[o + 1] = cinza; diff[o + 2] = cinza; diff[o + 3] = 255;
      }
    }
    const png = dif > 0 ? nativeImage.createFromBitmap(diff, { width: w, height: h }).toPNG().toString('base64') : null;
    return { total: w * h, dif, png };
  }, { a: arquivoA, b: arquivoB, mascaras });
  if (r.png && saidaDiff) {
    mkdirSync(dirname(saidaDiff), { recursive: true });
    writeFileSync(saidaDiff, Buffer.from(r.png, 'base64'));
  }
  delete r.png;
  return r;
}

// ------------------------------------------------------------------ principal
async function principal() {
  await pegarLock();
  if (args.compilar) {
    // Compila DENTRO do lock: app/out é compartilhado entre os agentes; ninguém compila no meio da
    // captura de outro.
    console.log('▶ npm run compilar -w @zapdesk/app (com o lock)');
    const r = spawnSync('npm', ['run', 'compilar', '-w', '@zapdesk/app'], { cwd: RAIZ, stdio: ['ignore', 'ignore', 'inherit'] });
    if (r.status !== 0) throw new Error('compilação falhou');
  }
  const pedidas = args.telas;
  if (pedidas) {
    const conhecidas = new Set(TELAS.map((t) => t.id));
    for (const t of pedidas) if (!conhecidas.has(t)) throw new Error(`Tela desconhecida: ${t} (veja contracts/capturas.md)`);
  }
  const telas = TELAS.filter((t) => (!pedidas && !args.roteiro) || (pedidas ?? []).includes(t.id));
  mkdirSync(SAIDA, { recursive: true });
  const caminhoMascaras = join(SAIDA, 'mascaras.json');
  const todasMascaras = existsSync(caminhoMascaras) ? JSON.parse(readFileSync(caminhoMascaras, 'utf8')) : {};
  const medidas = { geradoEm: new Date().toISOString(), telas: {}, aberturas: [] };
  const resultadosComparacao = [];
  let ctx = null;
  let tipoAtual = null;

  const fecharAtual = async () => {
    if (!ctx) return;
    if (args.medir) {
      medidas.aberturas.push({ app: tipoAtual, redeExterna: [...new Set(ctx.redeExterna)] });
    }
    await fecharApp(ctx.app);
    ctx = null;
  };

  for (const tela of telas) {
    if (tipoAtual !== tela.app || !ctx) {
      await fecharAtual();
      console.log(`\n▶ abrindo app (${tela.app})`);
      ctx = await abrirApp(tela.app);
      tipoAtual = tela.app;
      if (tela.app === 'carregando') await ctx.page.waitForSelector('.tela-cheia', { timeout: 15000 }).catch(() => {});
      if (tela.app === 'erro-motor') await ctx.page.waitForSelector('.erro-motor, .tela-erro, .tela-cheia', { timeout: 30000 }).catch(() => {});
      if (args.medir && (tela.app === 'semente' || tela.app === 'vazio')) {
        await esperarQuieto(ctx.page);
        const m = await medir(ctx);
        medidas.aberturas.push({ app: tela.app, layoutShiftAbertura: m.layoutShift, fontes: m.fontes });
      }
    }
    console.log(`• ${tela.id}`);
    if (!tela.porVariante && tela.rota) await navegar(ctx, tela);
    for (const tema of args.temas) {
      await definirTema(ctx, tema);
      for (const tam of TAMANHOS) {
        await definirTamanho(ctx, tam);
        if (tela.porVariante && tela.rota) await navegar(ctx, tela);
        else if (tela.app === 'erro-motor' || tela.app === 'carregando') await esperar(200);
        else await esperarQuieto(ctx.page);
        const nome = `${tela.id}-${tam.largura}-${tema}`;
        const arquivo = join(SAIDA, `${nome}.png`);
        const fotografarAqui = async () => {
          const masc = await areasMascaradas(ctx.page, [...MASCARAS_GLOBAIS, ...(tela.mascaras ?? [])]);
          writeFileSync(arquivo, await fotografar(ctx.page, tela.rapida));
          todasMascaras[nome] = masc.areas.map((a) => ({ ...a, escala: masc.escala }));
        };
        await fotografarAqui();
        if (args.medir) {
          const m = await medir(ctx);
          medidas.telas[nome] = { alturasItens: m.alturasItens, layoutShift: m.layoutShift };
        }
        if (COMPARAR) {
          const ref = join(COMPARAR, `${nome}.png`);
          if (!existsSync(ref)) {
            resultadosComparacao.push({ nome, erro: 'sem referência' });
            console.log(`    ${nome}: sem referência em ${args.comparar}`);
          } else {
            let mascRef = [];
            try {
              mascRef = JSON.parse(readFileSync(join(COMPARAR, 'mascaras.json'), 'utf8'))[nome] ?? [];
            } catch {}
            const diff = join(SAIDA, 'diferencas', `${nome}.png`);
            let r = await comparar(ctx, arquivo, ref, [...todasMascaras[nome], ...mascRef], diff);
            // Antisserrilhado/barra de rolagem às vezes oscilam 1 quadro: refotografa até 2× antes de
            // acusar. Diferença real persiste; ruído some. (Sem tolerância de cor: 0 px é 0 px.)
            for (let t = 0; t < 2 && !r.erro && r.dif > 0 && !tela.porVariante; t++) {
              await esperar(500);
              await fotografarAqui();
              r = await comparar(ctx, arquivo, ref, [...todasMascaras[nome], ...mascRef], diff);
              r.refeita = t + 1;
            }
            if (!r.erro && r.dif === 0) rmSync(diff, { force: true });
            resultadosComparacao.push({ nome, ...r });
            const pct = r.total ? ((100 * r.dif) / r.total).toFixed(4) : '?';
            console.log(`    ${nome}: ${r.erro ? r.erro : `${r.dif} px diferentes (${pct}%)`}`);
          }
        }
      }
    }
  }

  if (args.roteiro) {
    if (!ctx || tipoAtual !== 'semente') {
      await fecharAtual();
      console.log('\n▶ abrindo app (semente) para o roteiro');
      ctx = await abrirApp('semente');
      tipoAtual = 'semente';
    }
    const roteiro = (await import(resolve(process.cwd(), args.roteiro))).default;
    const c = ctx;
    await roteiro({
      app: c.app,
      page: c.page,
      ids: c.ids,
      tmp: TMP,
      esperar,
      api: (metodo, caminho, corpo) => apiMotor(c, metodo, caminho, corpo),
      tema: (t) => definirTema(c, t),
      tamanho: (largura, altura) => definirTamanho(c, { largura, altura }),
      ir: (hash) => navegar(c, { rota: () => hash }),
      foto: async (nome) => {
        const arquivo = join(SAIDA, `${nome}.png`);
        writeFileSync(arquivo, await fotografar(c.page, true));
        return arquivo;
      },
      medir: () => medir(c),
    });
  }

  writeFileSync(caminhoMascaras, JSON.stringify(todasMascaras, null, 2));
  if (args.medir) {
    const caminho = join(SAIDA, 'medidas.json');
    if (ctx) medidas.aberturas.push({ app: tipoAtual, redeExterna: [...new Set(ctx.redeExterna)] });
    writeFileSync(caminho, JSON.stringify(medidas, null, 2));
    console.log(`\nmedidas: ${caminho}`);
  }

  if (args.manter && ctx) {
    console.log(`\n--manter: app aberto (pasta ${TMP}). Ctrl+C para fechar e limpar.`);
    await new Promise(() => {});
  }
  await fecharAtual();

  if (COMPARAR) {
    const ruins = resultadosComparacao.filter((r) => r.erro || r.dif > 0);
    writeFileSync(join(SAIDA, 'comparacao.json'), JSON.stringify(resultadosComparacao, null, 2));
    console.log(`\ncomparação com ${args.comparar}: ${resultadosComparacao.length - ruins.length}/${resultadosComparacao.length} idênticas`);
    if (ruins.length) {
      for (const r of ruins) console.log(`  ✗ ${r.nome}: ${r.erro ?? `${r.dif} px`}`);
      process.exitCode = 1;
    }
  }
  console.log(`\nPNGs em ${SAIDA}`);
}

try {
  await principal();
} catch (e) {
  console.error(e);
  process.exitCode = 1;
} finally {
  await limpar();
}
