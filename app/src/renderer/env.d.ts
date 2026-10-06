/// <reference types="vite/client" />

import type { PonteZapDesk } from '../preload/index';

declare global {
  interface Window {
    zapdesk: PonteZapDesk;
  }
}
