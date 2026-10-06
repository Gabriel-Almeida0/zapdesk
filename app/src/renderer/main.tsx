// 003: CSS por área (ordem de importação = ordem das regras; não reordenar sem nova prova T009).
import './estilos/tema.css';
import './estilos/fontes.css';
import './estilos/base.css';
import './estilos/controles.css';
import './estilos/estrutura.css';
import './estilos/shell.css';
import './estilos/conversas.css';
import './estilos/conversa.css';
import './estilos/crm.css';
import './estilos/disparos.css';
import './estilos/ajustes.css';
import './estilos/funil.css';
import './estilos/automacoes.css';
import './estilos/editor-ia.css';

import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import { App } from './App';
import { aguardarFontes } from './util/fontes';
import { aplicarTema, lerPreferencia } from './util/tema';

const raiz = document.getElementById('raiz');
if (!raiz) throw new Error('Elemento #raiz não encontrado.');

// 003: tema salvo antes do primeiro paint (sem script inline: CSP script-src 'self') e fontes
// empacotadas carregadas (≤ 500 ms) antes do render — sem troca visível nem layout shift.
aplicarTema(lerPreferencia());
void aguardarFontes().then(() => {
  createRoot(raiz).render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
});
