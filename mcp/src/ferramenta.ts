// Registro modular de ferramentas: cada ferramenta recebe um `ClienteMotor` já garantido
// (app aberto e motor respondendo) e qualquer erro vira `isError: true` via `responderErro`.
import { homedir } from 'node:os';
import { isAbsolute, join, resolve } from 'node:path';

import type { CallToolResult, McpServer, ToolAnnotations } from '@modelcontextprotocol/server';
import type { ClienteMotor } from '@zapdesk/cliente-motor';
import type { z } from 'zod';

import { responderErro } from './formatar.js';

export interface Contexto {
  /** `garantirMotor()` em produção; cliente simulado nos testes. */
  obterCliente: () => Promise<ClienteMotor>;
  /** Pasta padrão de `exportar_relatorio`. */
  pastaDownloads: string;
}

export interface DefinicaoFerramenta<E extends z.ZodObject, S extends z.ZodType> {
  titulo: string;
  descricao: string;
  entrada: E;
  saida: S;
  anotacoes: ToolAnnotations;
  executar: (args: z.output<E>, cliente: ClienteMotor) => Promise<CallToolResult>;
}

export function registrarFerramenta<E extends z.ZodObject, S extends z.ZodType>(
  servidor: McpServer,
  contexto: Contexto,
  nome: string,
  definicao: DefinicaoFerramenta<E, S>,
): void {
  servidor.registerTool(
    nome,
    {
      title: definicao.titulo,
      description: definicao.descricao,
      inputSchema: definicao.entrada,
      outputSchema: definicao.saida,
      annotations: { title: definicao.titulo, ...definicao.anotacoes },
    },
    (async (args: z.output<E>) => {
      try {
        const cliente = await contexto.obterCliente();
        return await definicao.executar(args, cliente);
      } catch (erro) {
        return responderErro(erro);
      }
    }) as never,
  );
}

export type Registrador = (servidor: McpServer, contexto: Contexto) => void;

/** Resolve "~/x", caminhos relativos (ao diretório atual) e absolutos. O motor exige absoluto. */
export function caminhoAbsoluto(caminho: string): string {
  if (caminho === '~') return homedir();
  if (caminho.startsWith('~/')) return join(homedir(), caminho.slice(2));
  return isAbsolute(caminho) ? caminho : resolve(caminho);
}

/** Anotações prontas. */
export const LEITURA: ToolAnnotations = { readOnlyHint: true, openWorldHint: false };
export const ESCRITA_LOCAL: ToolAnnotations = {
  readOnlyHint: false,
  destructiveHint: false,
  openWorldHint: false,
};
export const EXCLUSAO_LOCAL: ToolAnnotations = {
  readOnlyHint: false,
  destructiveHint: true,
  openWorldHint: false,
};
/** Efeito visível no WhatsApp (envia algo a outras pessoas). */
export const ENVIO_WHATSAPP: ToolAnnotations = {
  readOnlyHint: false,
  destructiveHint: true,
  openWorldHint: true,
};
