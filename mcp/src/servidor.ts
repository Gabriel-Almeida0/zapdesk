// Montagem do servidor MCP `zapdesk`: registra as 31 ferramentas do MVP
// (specs/001-zapdesk-mvp/contracts/mcp-ferramentas.md) e as 38 de automações
// (specs/002-automacoes/contracts/mcp-ferramentas.md), cada uma um cliente fino da API do motor.
import { homedir } from 'node:os';
import { join } from 'node:path';

import { McpServer } from '@modelcontextprotocol/server';
import type { ClienteMotor } from '@zapdesk/cliente-motor';

import type { Contexto, Registrador } from './ferramenta.js';
import { registrarAutomacoes } from './ferramentas/automacoes.js';
import { registrarAutomacoesIA } from './ferramentas/automacoes-ia.js';
import { registrarContas } from './ferramentas/contas.js';
import { registrarContatos } from './ferramentas/contatos.js';
import { registrarConversas } from './ferramentas/conversas.js';
import { registrarDisparos } from './ferramentas/disparos.js';
import { registrarEtiquetas } from './ferramentas/etiquetas.js';
import { registrarExecucoes } from './ferramentas/execucoes.js';
import { registrarFunis } from './ferramentas/funis.js';
import { registrarLeads } from './ferramentas/leads.js';
import { registrarStatus } from './ferramentas/status.js';
import { registrarTemplates } from './ferramentas/templates.js';
import { criarGarantirMotor } from './garantir-motor.js';

export const NOME_SERVIDOR = 'zapdesk';
export const VERSAO_SERVIDOR = '0.1.0';

const INSTRUCOES =
  'ZapDesk é um WhatsApp desktop local (macOS) com disparo em massa. Estas ferramentas controlam o app do usuário: ' +
  'importar leads sem duplicar, criar disparos (começam na hora, sem confirmação — use pausar_disparo/cancelar_disparo ' +
  'para interromper), ler e responder conversas, e organizar contatos, etiquetas e templates. Se o app estiver fechado, ' +
  'a primeira chamada o abre e espera até 30 s. Telefones podem vir em qualquer formato. Quando houver mais de uma ' +
  'conta conectada, informe conta_id (veja listar_contas). ' +
  'Funil: leads em etapas de um Kanban (listar_funis, mover_card_funil). ' +
  'Automações: fluxos e chatbots são JSON (veja `ver_formatos_automacao`); automações de IA são projetos TypeScript — ' +
  'use `ver_tipos_sdk`, escreva arquivos, `compilar_automacao`, `testar_automacao` (simulação, nada é enviado) e só então ' +
  '`ativar_automacao`. Acompanhe com listar_execucoes/ver_execucao. Segredos só podem ser definidos pelo usuário no app.';

const REGISTRADORES: readonly Registrador[] = [
  registrarContas,
  registrarLeads,
  registrarConversas,
  registrarContatos,
  registrarEtiquetas,
  registrarTemplates,
  registrarDisparos,
  registrarStatus,
  // 002 — automações
  registrarFunis,
  registrarAutomacoes,
  registrarAutomacoesIA,
  registrarExecucoes,
];

export interface OpcoesServidor {
  /** Fornece o cliente do motor. Padrão: `garantirMotor()` (abre o app se fechado). */
  obterCliente?: () => Promise<ClienteMotor>;
  /** Pasta padrão do `exportar_relatorio`. Padrão: `~/Downloads`. */
  pastaDownloads?: string;
}

export function criarServidor(opcoes: OpcoesServidor = {}): McpServer {
  const servidor = new McpServer(
    { name: NOME_SERVIDOR, version: VERSAO_SERVIDOR },
    { capabilities: { tools: {} }, instructions: INSTRUCOES },
  );
  const contexto: Contexto = {
    obterCliente: opcoes.obterCliente ?? criarGarantirMotor(),
    pastaDownloads: opcoes.pastaDownloads ?? join(homedir(), 'Downloads'),
  };
  for (const registrar of REGISTRADORES) registrar(servidor, contexto);
  return servidor;
}
