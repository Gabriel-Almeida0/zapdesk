// Sono do Mac: avisa o motor (`POST /v1/sistema/energia`) para recalcular a agenda dos disparos
// sem "compensar" o tempo dormido.
import { powerMonitor } from 'electron';

import type { EventoEnergia } from '@zapdesk/cliente-motor';

export function vigiarEnergia(avisar: (evento: EventoEnergia) => Promise<void>): () => void {
  const suspender = (): void => {
    void avisar('suspender').catch(() => undefined);
  };
  const retomar = (): void => {
    void avisar('retomar').catch(() => undefined);
  };
  powerMonitor.on('suspend', suspender);
  powerMonitor.on('resume', retomar);
  return () => {
    powerMonitor.removeListener('suspend', suspender);
    powerMonitor.removeListener('resume', retomar);
  };
}
