// Rotas do app (HashRouter) e o layout com a barra lateral.
import { useEffect } from 'react';
import { Navigate, Outlet, Route, Routes, useNavigate } from 'react-router';

import { BarraLateral } from './componentes/BarraLateral';
import { useAbrirConversa } from './componentes/abrirConversa';
import { Ajustes } from './telas/Ajustes';
import { Automacoes } from './telas/Automacoes';
import { ConectarConta } from './telas/ConectarConta';
import { Contatos } from './telas/Contatos';
import { Conversas } from './telas/Conversas';
import { DetalheDisparo } from './telas/DetalheDisparo';
import { DetalheExecucao } from './telas/DetalheExecucao';
import { Disparos } from './telas/Disparos';
import { EditorAutomacao } from './telas/EditorAutomacao';
import { Etiquetas } from './telas/Etiquetas';
import { Funis } from './telas/Funis';
import { ImportarLeads } from './telas/ImportarLeads';
import { Kanban } from './telas/Kanban/Kanban';
import { Leads } from './telas/Leads';
import { NovaAutomacao } from './telas/NovaAutomacao';
import { NovoDisparo } from './telas/NovoDisparo';
import { Status } from './telas/Status';
import { Templates } from './telas/Templates';

/** 002: clique numa notificação do macOS → conversa ou automação. */
function OuvirNotificacoes() {
  const navegar = useNavigate();
  const abrirConversa = useAbrirConversa();
  useEffect(() => {
    const ponte = typeof window !== 'undefined' ? window.zapdesk : undefined;
    if (!ponte?.aoNotificacaoClicada) return undefined;
    return ponte.aoNotificacaoClicada((alvo) => {
      if (alvo.conversa_id) void abrirConversa(alvo.conversa_id);
      else if (alvo.automacao_id) void navegar(`/automacoes/${alvo.automacao_id}`);
    });
  }, [navegar, abrirConversa]);
  return null;
}

function Layout() {
  return (
    <div className="layout">
      <OuvirNotificacoes />
      <BarraLateral />
      <div className="conteudo">
        <Outlet />
      </div>
    </div>
  );
}

export function Rotas() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<Navigate to="/conversas" replace />} />
        <Route path="conversas" element={<Conversas />} />
        <Route path="conversas/:conversaId" element={<Conversas />} />
        <Route path="contatos" element={<Contatos />} />
        <Route path="status" element={<Status />} />
        <Route path="disparos" element={<Disparos />} />
        <Route path="disparos/novo" element={<NovoDisparo />} />
        <Route path="disparos/:disparoId" element={<DetalheDisparo />} />
        <Route path="leads" element={<Leads />} />
        <Route path="leads/importar" element={<ImportarLeads />} />
        <Route path="templates" element={<Templates />} />
        <Route path="etiquetas" element={<Etiquetas />} />
        <Route path="funis" element={<Funis />} />
        <Route path="funis/:funilId" element={<Kanban />} />
        <Route path="automacoes" element={<Automacoes />} />
        <Route path="automacoes/nova" element={<NovaAutomacao />} />
        <Route path="automacoes/:automacaoId" element={<EditorAutomacao />} />
        <Route path="execucoes/:execucaoId" element={<DetalheExecucao />} />
        <Route path="ajustes" element={<Ajustes />} />
        <Route path="contas/conectar" element={<ConectarConta />} />
        <Route path="*" element={<Navigate to="/conversas" replace />} />
      </Route>
    </Routes>
  );
}
