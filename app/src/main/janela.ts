// Janela única do ZapDesk, com as opções de segurança da constituição, e permissão de microfone
// (gravar áudio no chat) concedida só para a própria janela.
import { join } from 'node:path';

import { app, BrowserWindow, nativeTheme, session, shell, systemPreferences } from 'electron';

const PERMISSOES_PERMITIDAS = new Set(['media', 'clipboard-sanitized-write', 'notifications']);

function origemPropria(url: string): boolean {
  return url.startsWith('file://') || url.startsWith('http://localhost:') || url.startsWith('http://127.0.0.1:');
}

/** Microfone: só `audio` (nunca câmera), só para a janela do app. */
export function configurarPermissoes(): void {
  const sessao = session.defaultSession;
  sessao.setPermissionRequestHandler((conteudo, permissao, responder, detalhes) => {
    if (!origemPropria(conteudo.getURL()) || !PERMISSOES_PERMITIDAS.has(permissao)) {
      responder(false);
      return;
    }
    if (permissao === 'media') {
      const tipos = 'mediaTypes' in detalhes ? (detalhes.mediaTypes ?? []) : [];
      if (tipos.some((t) => t !== 'audio')) {
        responder(false);
        return;
      }
      // macOS: pede a permissão do sistema (NSMicrophoneUsageDescription no Info.plist).
      if (process.platform === 'darwin' && systemPreferences.getMediaAccessStatus('microphone') !== 'granted') {
        void systemPreferences.askForMediaAccess('microphone').then((ok) => responder(ok));
        return;
      }
    }
    responder(true);
  });
  sessao.setPermissionCheckHandler((_conteudo, permissao, origem) => {
    return PERMISSOES_PERMITIDAS.has(permissao) && origemPropria(origem);
  });
}

export function criarJanela(): BrowserWindow {
  const janela = new BrowserWindow({
    width: 1280,
    height: 820,
    minWidth: 960,
    minHeight: 620,
    show: false,
    title: 'ZapDesk',
    // 003: cor antes do primeiro paint = --fundo de tema.css (escuro/claro). Exceção 3 de
    // specs/003-identidade-visual/contracts/tokens.md §4 (o processo principal não lê variáveis CSS).
    backgroundColor: nativeTheme.shouldUseDarkColors ? '#0E1013' : '#F4F1E8',
    titleBarStyle: 'hiddenInset',
    trafficLightPosition: { x: 18, y: 18 },
    webPreferences: {
      preload: join(__dirname, '../preload/index.js'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
      spellcheck: true,
    },
  });

  janela.once('ready-to-show', () => janela.show());

  // Links externos abrem no navegador padrão; nunca dentro do app.
  janela.webContents.setWindowOpenHandler(({ url }) => {
    if (url.startsWith('https://') || url.startsWith('http://')) void shell.openExternal(url);
    return { action: 'deny' };
  });
  janela.webContents.on('will-navigate', (evento, url) => {
    if (!origemPropria(url)) evento.preventDefault();
  });

  const urlDev = process.env['ELECTRON_RENDERER_URL'];
  if (!app.isPackaged && urlDev) {
    void janela.loadURL(urlDev);
  } else {
    void janela.loadFile(join(__dirname, '../renderer/index.html'));
  }
  return janela;
}
