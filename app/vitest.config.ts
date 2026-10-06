import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

// Dois projetos: main/preload em node (tests/main/**) e renderer em jsdom (tests/renderer/**).
export default defineConfig({
  plugins: [react()],
  test: {
    projects: [
      {
        extends: true,
        test: { name: 'main', include: ['tests/main/**/*.test.ts'], environment: 'node' },
      },
      {
        extends: true,
        test: {
          name: 'renderer',
          include: ['tests/renderer/**/*.test.tsx'],
          environment: 'jsdom',
          setupFiles: ['tests/renderer/configuracao.ts'],
        },
      },
    ],
  },
});
