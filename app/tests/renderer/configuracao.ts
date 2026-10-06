// Limpa o DOM entre testes (Testing Library sem `globals: true`).
import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';

afterEach(() => {
  cleanup();
  localStorage.clear();
});
