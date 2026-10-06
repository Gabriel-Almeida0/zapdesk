// Processo principal do ZapDesk.
// Abrir → trava de instância única → motor (processo filho) → runtime.json 0600 → janela.
// Fechar → sem disparo ativo: encerra o motor e sai (nada sobra); com disparo ativo: barra de menu
// até terminar (ciclo-vida.ts). Queda do motor → religa uma vez com `--religado`.
import { existsSync, mkdirSync } from 'node:fs';
import { mkdtemp } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { app, BrowserWindow, dialog, Notification, safeStorage } from 'electron';

import { AssinaturaEventos, ClienteMotor } from '@zapdesk/cliente-motor';

import { CANAIS, type ConexaoMotor, type EstadoMotor, type InfoMcp } from '../preload/tipos';
import { Bandeja } from './bandeja';
import { CicloVida } from './ciclo-vida';
import { vigiarEnergia } from './energia';
import { registrarIpc } from './ipc';
import { configurarPermissoes, criarJanela } from './janela';
import { ProcessoMotor, SupervisorMotor } from './motor';
import { Notificador } from './notificacoes';
import { resolverPastaInterface } from './pastas';
import { apagarRuntime, apagarRuntimeSync, gravarRuntime } from './runtime-json';
import { CofreSegredos } from './segredos';

app.setName('ZapDesk');

const modoFalso =
  process.env['ZAPDESK_DEV_FALSO'] === '1' || process.argv.some((a) => a === '--whatsapp=falso');

// Dados do Chromium ficam numa subpasta, separados dos dados do motor.
// 003: no modo falso (ou com ZAPDESK_PASTA_INTERFACE) a pasta é outra — antes da trava de instância
// única, para a instância de teste conviver com o app instalado aberto sem tocar nos dados dele.
const pastaDadosPadrao = join(app.getPath('appData'), 'ZapDesk');
app.setPath(
  'userData',
  resolverPastaInterface({
    env: process.env,
    modoFalso,
    padrao: join(pastaDadosPadrao, 'interface'),
    tmp: tmpdir(),
  }),
);

function caminhoMotor(): string {
  const env = process.env['ZAPDESK_MOTOR_BIN'];
  if (env) return env;
  return app.isPackaged
    ? join(process.resourcesPath, 'bin', 'zapdesk-motor')
    : join(app.getAppPath(), '..', 'motor', 'bin', 'zapdesk-motor');
}

function caminhoScriptMcp(): string {
  return app.isPackaged
    ? join(process.resourcesPath, 'mcp', 'zapdesk-mcp.mjs')
    : join(app.getAppPath(), '..', 'mcp', 'dist', 'zapdesk-mcp.mjs');
}

/** 002: script do runner das automações de IA (roda com o Node do próprio Electron). */
function caminhoScriptRunner(): string {
  const env = process.env['ZAPDESK_RUNNER_SCRIPT'];
  if (env) return env;
  return app.isPackaged
    ? join(process.resourcesPath, 'runner', 'zapdesk-runner.mjs')
    : join(app.getAppPath(), '..', 'automacao', 'runner', 'dist', 'zapdesk-runner.mjs');
}

function pastaRecursos(): string {
  return join(app.getAppPath(), 'resources');
}

async function resolverPastaDados(): Promise<string> {
  const env = process.env['ZAPDESK_PASTA_DADOS'];
  if (env) return env;
  if (modoFalso) return mkdtemp(join(tmpdir(), 'zapdesk-falso-'));
  return pastaDadosPadrao;
}

if (!app.requestSingleInstanceLock()) {
  // Outra instância já está aberta: ela recebe `second-instance` e mostra a janela.
  app.exit(0);
} else {
  void iniciar();
}

async function iniciar(): Promise<void> {
  const pastaDados = await resolverPastaDados();
  mkdirSync(pastaDados, { recursive: true, mode: 0o700 });

  let janela: BrowserWindow | null = null;
  let pararEventos: (() => void) | null = null;
  let saindo = false;

  const enviar = (canal: string, valor: unknown): void => {
    if (janela && !janela.isDestroyed()) janela.webContents.send(canal, valor);
  };

  // 002: segredos cifrados no Keychain; o motor recebe os valores pelo stdin após cada `pronto`.
  const cofre = new CofreSegredos({ pastaDados, cifrador: safeStorage });
  const scriptRunner = caminhoScriptRunner();
  const runnerExiste = existsSync(scriptRunner);
  if (!runnerExiste) process.stderr.write(`Runner das automações de IA não encontrado em ${scriptRunner}.\n`);

  const supervisor = new SupervisorMotor({
    iniciar: (religado) =>
      ProcessoMotor.iniciar({
        binario: caminhoMotor(),
        pastaDados,
        falso: modoFalso,
        religado,
        ...(runnerExiste ? { runnerExec: process.execPath, runnerScript: scriptRunner } : {}),
        aguardarSegredos: true,
        ...(app.isPackaged ? {} : { aoLogar: (linha: string) => process.stderr.write(`[motor] ${linha}\n`) }),
      }),
    segredos: () => cofre.valores(),
  });
  cofre.on('alterado', (valores) => supervisor.enviarSegredos(valores));

  const notificador = new Notificador(
    {
      suportada: () => Notification.isSupported(),
      criar: (opcoes) => new Notification(opcoes),
    },
    (alvo) => {
      ciclo.reabrir();
      enviar(CANAIS.notificacaoClicada, alvo);
    },
  );

  const bandeja = new Bandeja(pastaRecursos(), {
    abrir: () => ciclo.reabrir(),
    sairAgora: () => ciclo.forcarSaida(),
  });

  const encerrarTudo = async (): Promise<void> => {
    if (saindo) return;
    saindo = true;
    pararEventos?.();
    pararEventos = null;
    try {
      await supervisor.parar();
    } finally {
      await apagarRuntime(pastaDados).catch(() => undefined);
      bandeja.destruir();
      app.exit(0);
    }
  };

  const ciclo = new CicloVida({
    // A janela pode já ter sido destruída (ex.: `window.close()` vindo do renderer): chamar
    // `hide()` nela lança "Object has been destroyed" e o Electron abre um alerta modal que trava o
    // processo principal — o app nunca mais encerraria sozinho.
    esconderJanela: () => {
      if (janela && !janela.isDestroyed()) janela.hide();
    },
    mostrarJanela: () => {
      if (!janela || janela.isDestroyed()) janela = abrirJanela();
      if (janela.isMinimized()) janela.restore();
      janela.show();
      janela.focus();
    },
    ocultarDock: () => app.dock?.hide(),
    mostrarDock: () => void app.dock?.show(),
    criarBandeja: () => bandeja.criar(),
    atualizarBandeja: (resumo) => bandeja.atualizar(resumo),
    destruirBandeja: () => bandeja.destruir(),
    avisar: (mensagem) => {
      if (Notification.isSupported()) {
        new Notification({ title: 'ZapDesk', body: mensagem, silent: true }).show();
      } else {
        void dialog.showMessageBox({ type: 'info', message: mensagem });
      }
    },
    notificar: (titulo, corpo) => {
      if (Notification.isSupported()) new Notification({ title: titulo, body: corpo }).show();
    },
    encerrar: () => void encerrarTudo(),
  });

  const conectarEventos = (conexao: ConexaoMotor): void => {
    pararEventos?.();
    const cliente = new ClienteMotor({ porta: conexao.porta, token: conexao.token });
    const sincronizarAtivos = (): void => {
      void cliente
        .sistema()
        .then((s) => ciclo.atualizarAtivos(s.disparos_ativos))
        .catch(() => undefined);
    };
    const assinatura = new AssinaturaEventos({
      url: cliente.urlEventos(),
      aoEvento: (evento) => {
        switch (evento.tipo) {
          case 'disparos.ativos':
            ciclo.atualizarAtivos(evento.dados.total);
            break;
          case 'disparo.atualizado':
            ciclo.atualizarDisparo(evento.dados);
            break;
          case 'disparo.finalizado':
            ciclo.registrarFinalizado(evento.dados.resumo, evento.dados.disparo.id);
            break;
          case 'notificacao':
            notificador.mostrar(evento.dados);
            break;
          default:
            break;
        }
        enviar(CANAIS.evento, evento);
      },
      aoRecarregar: (motivo) => {
        sincronizarAtivos();
        enviar(CANAIS.recarregar, motivo);
      },
    }).iniciar();
    const pararEnergia = vigiarEnergia((ev) => cliente.energia(ev));
    sincronizarAtivos();
    pararEventos = () => {
      assinatura.encerrar();
      pararEnergia();
    };
  };

  supervisor.on('estado', (estado: EstadoMotor) => {
    enviar(CANAIS.estadoMotor, estado);
    if (estado.fase === 'pronto') {
      const { conexao } = estado;
      void gravarRuntime(pastaDados, {
        porta: conexao.porta,
        token: conexao.token,
        pidApp: process.pid,
        pidMotor: conexao.pidMotor,
        versao: conexao.versao,
      }).catch((erro: unknown) => {
        process.stderr.write(`Falha ao gravar runtime.json: ${String(erro)}\n`);
      });
      conectarEventos(conexao);
    } else {
      pararEventos?.();
      pararEventos = null;
      void apagarRuntime(pastaDados).catch(() => undefined);
      if (estado.fase === 'erro') ciclo.motorFalhou(estado.detalhe);
    }
  });

  const infoMcp = (): InfoMcp => {
    const script = caminhoScriptMcp();
    return {
      executavel: process.execPath,
      script,
      scriptExiste: existsSync(script),
      empacotado: app.isPackaged,
      pastaDados,
    };
  };

  registrarIpc({
    estado: () => supervisor.estado,
    reiniciarMotor: () => supervisor.reiniciar(),
    janela: () => janela,
    infoMcp,
    cofre,
    pastaDados,
  });

  function abrirJanela(): BrowserWindow {
    const nova = criarJanela();
    nova.on('close', (evento) => {
      if (saindo) return;
      evento.preventDefault();
      ciclo.pedirFechar();
    });
    return nova;
  }

  // Sem este ouvinte o Electron encerra o app quando a última janela some. Quem decide é o ciclo:
  // se a janela foi destruída sem passar pelo `close` acima, trata como "fechar a janela".
  app.on('window-all-closed', () => {
    if (!saindo) ciclo.pedirFechar();
  });

  // Cmd+Q / menu "Sair".
  app.on('before-quit', (evento) => {
    if (saindo) return;
    evento.preventDefault();
    ciclo.pedirFechar();
  });

  // Segunda instância, clique no Dock ou `open -b com.gabriel.zapdesk` (MCP) → mostrar a janela.
  app.on('second-instance', () => ciclo.reabrir());
  app.on('activate', () => ciclo.reabrir());

  // Sem ouvinte, o Electron mostra um alerta modal para exceções não tratadas, que bloqueia o
  // processo principal inteiro (bandeja, eventos do motor e o encerramento automático).
  process.on('uncaughtException', (erro) => {
    process.stderr.write(`Erro não tratado no processo principal: ${erro.stack ?? String(erro)}\n`);
  });

  // Saída abrupta do processo principal: não deixar o motor órfão nem um runtime.json velho.
  process.on('exit', () => {
    supervisor.matarAgora();
    apagarRuntimeSync(pastaDados);
  });
  for (const sinal of ['SIGINT', 'SIGTERM', 'SIGHUP'] as const) {
    process.on(sinal, () => void encerrarTudo());
  }

  await app.whenReady();
  configurarPermissoes();
  // safeStorage só funciona depois do `ready`; o motor só liga depois de ler os segredos.
  await cofre.carregar();
  janela = abrirJanela();
  await supervisor.ligar();
}
