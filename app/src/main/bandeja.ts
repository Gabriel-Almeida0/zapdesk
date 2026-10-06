// Ícone da barra de menu — existe só enquanto um disparo mantém o app vivo com a janela fechada.
import { join } from 'node:path';

import { Menu, nativeImage, Tray } from 'electron';

import type { ResumoProgresso } from './ciclo-vida';

export interface AcoesBandeja {
  abrir(): void;
  sairAgora(): void;
}

export class Bandeja {
  private tray: Tray | null = null;
  private readonly acoes: AcoesBandeja;
  private readonly pastaRecursos: string;

  constructor(pastaRecursos: string, acoes: AcoesBandeja) {
    this.pastaRecursos = pastaRecursos;
    this.acoes = acoes;
  }

  criar(): void {
    if (this.tray) return;
    const imagem = nativeImage.createFromPath(join(this.pastaRecursos, 'bandejaTemplate.png'));
    imagem.setTemplateImage(true);
    this.tray = new Tray(imagem);
    this.tray.setToolTip('ZapDesk — disparo em andamento');
    this.tray.on('click', () => this.tray?.popUpContextMenu());
  }

  atualizar(resumo: ResumoProgresso): void {
    if (!this.tray) return;
    this.tray.setTitle(resumo.disparos > 0 ? ` ${resumo.percentual}%` : '');
    this.tray.setContextMenu(
      Menu.buildFromTemplate([
        { label: resumo.texto, enabled: false },
        { type: 'separator' },
        { label: 'Abrir ZapDesk', click: () => this.acoes.abrir() },
        { type: 'separator' },
        { label: 'Sair agora (o disparo fica pausado)', click: () => this.acoes.sairAgora() },
      ]),
    );
  }

  destruir(): void {
    this.tray?.destroy();
    this.tray = null;
  }
}
