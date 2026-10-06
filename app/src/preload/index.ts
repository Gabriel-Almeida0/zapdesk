// Preload (sandbox): ponte mínima entre o renderer e o processo principal.
// Nada de Node aqui além do IPC; o token só chega ao renderer para montar URLs binárias.
import { contextBridge, ipcRenderer, type IpcRendererEvent } from 'electron';

import {
  CANAIS,
  type EstadoMotor,
  type InfoMcp,
  type NotificacaoClicada,
  type OuvinteEvento,
  type OuvinteRecarga,
  type RequisicaoIpc,
  type RespostaIpc,
  type ResultadoSalvar,
  type ResultadoSegredos,
  type SegredoLocal,
} from './tipos';

function ouvir<T>(canal: string, ouvinte: (valor: T) => void): () => void {
  const manipulador = (_evento: IpcRendererEvent, valor: T): void => ouvinte(valor);
  ipcRenderer.on(canal, manipulador);
  return () => {
    ipcRenderer.removeListener(canal, manipulador);
  };
}

const ponte = {
  plataforma: process.platform,
  /** Estado atual do motor (inclui porta e token quando pronto). */
  obterConexao: (): Promise<EstadoMotor> => ipcRenderer.invoke(CANAIS.obterEstado),
  aoMudarEstadoMotor: (ouvinte: (estado: EstadoMotor) => void): (() => void) =>
    ouvir(CANAIS.estadoMotor, ouvinte),
  /** "Tentar de novo" na tela de erro. */
  reiniciarMotor: (): Promise<void> => ipcRenderer.invoke(CANAIS.reiniciar),
  /** HTTP ao motor pelo processo principal. */
  requisitarMotor: (req: RequisicaoIpc): Promise<RespostaIpc> => ipcRenderer.invoke(CANAIS.requisitar, req),
  aoEventoMotor: (ouvinte: OuvinteEvento): (() => void) => ouvir(CANAIS.evento, ouvinte),
  aoRecarregar: (ouvinte: OuvinteRecarga): (() => void) => ouvir(CANAIS.recarregar, ouvinte),
  /** Diálogo "Salvar" + grava o relatório CSV do disparo. */
  exportarRelatorio: (disparoId: string, nomeSugerido: string): Promise<ResultadoSalvar> =>
    ipcRenderer.invoke(CANAIS.exportarRelatorio, disparoId, nomeSugerido),
  infoMcp: (): Promise<InfoMcp> => ipcRenderer.invoke(CANAIS.infoMcp),
  copiar: (texto: string): Promise<void> => ipcRenderer.invoke(CANAIS.copiar, texto),
  abrirExterno: (url: string): Promise<void> => ipcRenderer.invoke(CANAIS.abrirExterno, url),
  mostrarNoFinder: (caminho: string): Promise<void> => ipcRenderer.invoke(CANAIS.mostrarNoFinder, caminho),
  /** Baixa um binário do motor (documento) para Downloads e abre com o app padrão. */
  abrirArquivoMotor: (caminhoApi: string, nome: string): Promise<void> =>
    ipcRenderer.invoke(CANAIS.abrirArquivoMotor, caminhoApi, nome),
  /** 002: segredos guardados no Keychain (valores nunca voltam para a janela). */
  segredos: {
    listar: (): Promise<SegredoLocal[]> => ipcRenderer.invoke(CANAIS.segredosListar),
    definir: (nome: string, valor: string): Promise<ResultadoSegredos> =>
      ipcRenderer.invoke(CANAIS.segredosDefinir, nome, valor),
    remover: (nome: string): Promise<ResultadoSegredos> => ipcRenderer.invoke(CANAIS.segredosRemover, nome),
  },
  /** 002: abre a pasta de uma automação de IA no editor externo (só dentro de `<pasta-dados>/automacoes/`). */
  abrirPastaExterna: (caminho: string): Promise<void> => ipcRenderer.invoke(CANAIS.abrirPastaExterna, caminho),
  /** 002: clique numa notificação do macOS (abrir a conversa ou a automação). */
  aoNotificacaoClicada: (ouvinte: (alvo: NotificacaoClicada) => void): (() => void) =>
    ouvir(CANAIS.notificacaoClicada, ouvinte),
};

export type PonteZapDesk = typeof ponte;

contextBridge.exposeInMainWorld('zapdesk', ponte);
