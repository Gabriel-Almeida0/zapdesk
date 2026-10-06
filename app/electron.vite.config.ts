import { resolve } from 'node:path';

import react from '@vitejs/plugin-react';
import { defineConfig } from 'electron-vite';

// Uma config para os três alvos do Electron.
// - main/preload: dependências do node_modules ficam externas, EXCETO o pacote do workspace
//   @zapdesk/cliente-motor, que é embutido no bundle (não vai para o .app como node_modules).
// - preload roda com `sandbox: true` → precisa sair em CommonJS e sem imports externos.
export default defineConfig({
  main: {
    build: {
      externalizeDeps: { exclude: ['@zapdesk/cliente-motor'] },
    },
  },
  preload: {
    build: {
      externalizeDeps: false,
    },
  },
  renderer: {
    root: resolve(__dirname, 'src/renderer'),
    build: {
      rollupOptions: {
        input: resolve(__dirname, 'src/renderer/index.html'),
      },
    },
    plugins: [react()],
  },
});
