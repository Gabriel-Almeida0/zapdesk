/// <reference lib="dom" preserve="true" />
// Ponto de entrada do SDK `@zapdesk/automacao` (specs/002-automacoes/contracts/sdk-automacao.md).
//
// Regras do build:
// - `npm run compilar` gera `dist/index.js` e um `dist/index.d.ts` AUTOSSUFICIENTE (juntar-dts.mjs
//   embute os `export * from './x.js'` locais) — é esse arquivo que `npm run sdk:sincronizar`
//   copia para o motor (go:embed) e que o editor/VS Code usam.
// - Reexporte módulos locais sempre com `export * from './x.js'` (nunca `export * as`).
// - Sem dependências de runtime. A referência à lib "dom" traz `AbortSignal`, `RequestInit` e
//   `Response` para quem usa o .d.ts sem tipos do Node (Monaco, VS Code).

export * from './tipos.js';
export * from './erros.js';
export * from './definir.js';

/** Versão do SDK; o motor a publica em `GET /v1/automacoes/sdk` (`SdkAutomacao.versao`). */
export const VERSAO_SDK = '1.0.0';
