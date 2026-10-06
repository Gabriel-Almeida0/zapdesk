// Entrada do runner das automações de IA (dist/zapdesk-runner.mjs). Contrato:
// specs/002-automacoes/contracts/runner-protocolo.md e runtime.md › Runner.
//
// Ordem importa: (1) toma o stdout para o protocolo e redireciona console.*; (2) aplica os
// bloqueios de módulos/globais; (3) liga sourcemaps (stack do usuário aponta para o .ts);
// (4) escuta o stdin e envia `pronto`. Só então o motor manda `inicializar`, que importa o bundle.
// Stdin fechado (motor morreu) → sai com código 0 na hora (Constituição II).
import { fileURLToPath } from 'node:url';
import { VERSAO_SDK } from '@zapdesk/automacao';
import { aplicarBloqueios } from './bloqueios.js';
import { cortar, instalarConsole, LIMITE_TEXTO_LOG } from './console.js';
import { VERSAO_PROTOCOLO, type ParamsHttp, type ParamsLog, type ParamsPronto } from './protocolo.js';
import { ConexaoRpc } from './rpc.js';
import { criarRunner } from './runner.js';
import { prepararSdkRuntime } from './sdk-runtime.js';

export const VERSAO_RUNNER = '1.0.0';
/** Versão do SDK embutido neste bundle (o runner serve `@zapdesk/automacao` às automações). */
export const VERSAO_SDK_EMBUTIDA: string = VERSAO_SDK;

const urlRunner = import.meta.url;
const caminhoRunner = fileURLToPath(urlRunner);
const escreverErro = process.stderr.write.bind(process.stderr);

// (1) stdout exclusivo do protocolo
let conexao: ConexaoRpc | null = null;
let ultimaEscrita: Promise<void> = Promise.resolve();
const escreverStdout = instalarConsole((execucaoId, nivel, texto) => {
  conexao?.notificar('log', { execucao_id: execucaoId, nivel, texto, em: new Date().toISOString() } satisfies ParamsLog);
});
function escrever(linha: string): void {
  ultimaEscrita = new Promise((resolver) => {
    escreverStdout(linha, () => resolver());
  });
}

// (2) bloqueios
const originais = aplicarBloqueios({ urlRunner, fonteSdk: prepararSdkRuntime() });
const sair = (codigo: number): never => originais.exit(codigo);

// (3) sourcemaps inline do bundle compilado pelo motor
process.setSourceMapsEnabled(true);

// Erros soltos do código do usuário (timer que lança, promessa rejeitada sem catch) não derrubam
// o processo: vão para o stderr, que o motor captura.
process.on('uncaughtException', (erro) => {
  escreverErro(`[runner] erro não tratado: ${cortar(String((erro as Error)?.stack ?? erro), LIMITE_TEXTO_LOG)}\n`);
});
process.on('unhandledRejection', (motivo) => {
  escreverErro(`[runner] promessa rejeitada sem tratamento: ${cortar(String((motivo as Error)?.stack ?? motivo), LIMITE_TEXTO_LOG)}\n`);
});

function limparStack(stack: string): string {
  return stack
    .split('\n')
    .filter((l) => !l.includes(caminhoRunner) && !l.includes(urlRunner) && !l.includes('node:internal'))
    .join('\n');
}

// (4) protocolo
const runner = criarRunner({
  requisitar: (metodo, params) => conexao!.requisitar(metodo, params),
  notificar: ((metodo: 'log' | 'http', params: ParamsLog | ParamsHttp) => conexao?.notificar(metodo, params)) as never,
  fetchInterno: originais.fetch,
  limparStack,
  aoEncerrar: () => {
    void ultimaEscrita.then(() => sair(0));
    setTimeout(() => sair(0), 500).unref();
  },
});

conexao = new ConexaoRpc({
  escrever,
  aoRequisicao: (metodo, params) => runner.tratarRequisicao(metodo, params),
  aoNotificacao: (metodo, params) => runner.tratarNotificacao(metodo, params),
  aoFim: () => sair(0),
});

process.stdin.on('data', (pedaco: Buffer) => conexao!.receber(pedaco));
process.stdin.on('end', () => conexao!.fim());
process.stdin.on('close', () => conexao!.fim());
process.stdin.on('error', () => conexao!.fim());

conexao.notificar('pronto', {
  versao_runner: VERSAO_RUNNER,
  protocolo: VERSAO_PROTOCOLO,
  node: process.versions.node,
} satisfies ParamsPronto);
