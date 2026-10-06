// Canais IPC do processo principal (ver src/preload/index.ts).
// O renderer fala com o motor pelo `requisitar` (proxy HTTP no main): assim não depende de CORS
// do motor e o main só aceita URLs `http://127.0.0.1:<porta atual>/v1/...`.
import { execFile } from 'node:child_process';
import { existsSync, statSync } from 'node:fs';
import { writeFile } from 'node:fs/promises';
import { basename, extname, join } from 'node:path';

import { app, BrowserWindow, clipboard, dialog, ipcMain, shell } from 'electron';

import { ClienteMotor } from '@zapdesk/cliente-motor';

import {
  CANAIS,
  type ConexaoMotor,
  type EstadoMotor,
  type InfoMcp,
  type RequisicaoIpc,
  type RespostaIpc,
  type ResultadoSalvar,
  type ResultadoSegredos,
} from '../preload/tipos';
import { pastaAutomacaoPermitida } from './pastas';
import type { CofreSegredos } from './segredos';

export interface ContextoIpc {
  estado(): EstadoMotor;
  reiniciarMotor(): Promise<void>;
  janela(): BrowserWindow | null;
  infoMcp(): InfoMcp;
  /** 002 */
  cofre: CofreSegredos;
  pastaDados: string;
}

function mensagemErro(erro: unknown): string {
  return erro instanceof Error ? erro.message : String(erro);
}

function conexaoAtual(ctx: ContextoIpc): ConexaoMotor {
  const estado = ctx.estado();
  if (estado.fase !== 'pronto') throw new Error('O motor ainda não está pronto.');
  return estado.conexao;
}

/** Só rotas do motor atual. */
export function urlPermitida(url: string, porta: number): boolean {
  try {
    const u = new URL(url);
    return u.protocol === 'http:' && u.hostname === '127.0.0.1' && u.port === String(porta) && u.pathname.startsWith('/v1/');
  } catch {
    return false;
  }
}

/** Nome de arquivo seguro para gravar em Downloads. */
export function nomeSeguro(nome: string): string {
  const limpo = basename(nome).replace(/[/\\:*?"<>|\u0000-\u001f]/g, '_').trim();
  return limpo.length > 0 ? limpo.slice(0, 180) : 'arquivo';
}

function caminhoLivre(pasta: string, nome: string): string {
  const ext = extname(nome);
  const base = nome.slice(0, nome.length - ext.length);
  let candidato = join(pasta, nome);
  for (let i = 1; existsSync(candidato) && i < 1000; i++) candidato = join(pasta, `${base} (${i})${ext}`);
  return candidato;
}

export function registrarIpc(ctx: ContextoIpc): void {
  ipcMain.handle(CANAIS.obterEstado, () => ctx.estado());

  ipcMain.handle(CANAIS.reiniciar, () => ctx.reiniciarMotor());

  ipcMain.handle(CANAIS.requisitar, async (_e, req: RequisicaoIpc): Promise<RespostaIpc> => {
    const conexao = conexaoAtual(ctx);
    if (!urlPermitida(req.url, conexao.porta)) throw new Error('URL não permitida.');
    const resposta = await fetch(req.url, {
      method: req.metodo,
      headers: req.cabecalhos,
      ...(req.corpo ? { body: req.corpo } : {}),
    });
    const cabecalhos: Record<string, string> = {};
    resposta.headers.forEach((valor, chave) => {
      cabecalhos[chave] = valor;
    });
    return { status: resposta.status, cabecalhos, corpo: new Uint8Array(await resposta.arrayBuffer()) };
  });

  ipcMain.handle(
    CANAIS.exportarRelatorio,
    async (_e, disparoId: string, nomeSugerido: string): Promise<ResultadoSalvar> => {
      const conexao = conexaoAtual(ctx);
      const janela = ctx.janela();
      const opcoes = {
        title: 'Exportar relatório',
        defaultPath: join(app.getPath('downloads'), nomeSeguro(nomeSugerido)),
        filters: [{ name: 'Planilha CSV', extensions: ['csv'] }],
      };
      const escolha = janela ? await dialog.showSaveDialog(janela, opcoes) : await dialog.showSaveDialog(opcoes);
      if (escolha.canceled || !escolha.filePath) return { salvo: false };
      const cliente = new ClienteMotor({ porta: conexao.porta, token: conexao.token });
      const texto = await cliente.relatorioCsv(disparoId);
      // `Response.text()` remove o BOM; o Excel precisa dele para ler UTF-8.
      const conteudo = texto.startsWith('﻿') ? texto : `﻿${texto}`;
      await writeFile(escolha.filePath, conteudo, 'utf8');
      return { salvo: true, caminho: escolha.filePath };
    },
  );

  ipcMain.handle(CANAIS.infoMcp, () => ctx.infoMcp());

  ipcMain.handle(CANAIS.copiar, (_e, texto: string) => {
    clipboard.writeText(String(texto));
  });

  ipcMain.handle(CANAIS.abrirExterno, async (_e, url: string) => {
    if (/^https?:\/\//.test(url)) await shell.openExternal(url);
  });

  ipcMain.handle(CANAIS.mostrarNoFinder, (_e, caminho: string) => {
    if (typeof caminho === 'string' && caminho.startsWith('/')) shell.showItemInFolder(caminho);
  });

  ipcMain.handle(CANAIS.abrirArquivoMotor, async (_e, caminhoApi: string, nome: string) => {
    const conexao = conexaoAtual(ctx);
    const cliente = new ClienteMotor({ porta: conexao.porta, token: conexao.token });
    const url = cliente.urlBinaria(caminhoApi);
    if (!urlPermitida(url, conexao.porta)) throw new Error('URL não permitida.');
    const resposta = await fetch(url);
    if (!resposta.ok) throw new Error('Não foi possível baixar.');
    const destino = caminhoLivre(app.getPath('downloads'), nomeSeguro(nome));
    await writeFile(destino, new Uint8Array(await resposta.arrayBuffer()));
    const erro = await shell.openPath(destino);
    if (erro) shell.showItemInFolder(destino);
  });

  // 002 — segredos (valores só entram; nunca saem para a janela).
  const resultado = async (acao: () => Promise<unknown>): Promise<ResultadoSegredos> => {
    try {
      await acao();
      return { segredos: ctx.cofre.listar(), erro: null };
    } catch (erro) {
      return { segredos: ctx.cofre.listar(), erro: mensagemErro(erro) };
    }
  };
  ipcMain.handle(CANAIS.segredosListar, () => ctx.cofre.listar());
  ipcMain.handle(CANAIS.segredosDefinir, (_e, nome: unknown, valor: unknown) =>
    resultado(() => ctx.cofre.definir(String(nome), String(valor))),
  );
  ipcMain.handle(CANAIS.segredosRemover, (_e, nome: unknown) => resultado(() => ctx.cofre.remover(String(nome))));

  // 002 — "Abrir pasta no editor externo": VS Code se instalado; senão o Finder.
  ipcMain.handle(CANAIS.abrirPastaExterna, async (_e, caminho: unknown) => {
    const pasta = pastaAutomacaoPermitida(caminho, ctx.pastaDados);
    if (!pasta || !existsSync(pasta) || !statSync(pasta).isDirectory()) throw new Error('Pasta da automação não encontrada.');
    const abriuNoVsCode = await new Promise<boolean>((resolver) => {
      if (process.platform !== 'darwin') {
        resolver(false);
        return;
      }
      execFile('/usr/bin/open', ['-b', 'com.microsoft.VSCode', pasta], (erro) => resolver(!erro));
    });
    if (!abriuNoVsCode) {
      const erro = await shell.openPath(pasta);
      if (erro) throw new Error(erro);
    }
  });
}
