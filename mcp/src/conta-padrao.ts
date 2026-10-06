// Conta padrão: `conta_id` é opcional quando existe exatamente uma conta `conectada`
// (contracts/mcp-ferramentas.md › Regras gerais).
import type { ClienteMotor, Conta } from '@zapdesk/cliente-motor';

import { ErroFerramenta, listar } from './formatar.js';

export function descreverConta(conta: Conta): string {
  return `${conta.nome} (${conta.telefone ?? 'sem número'}, ${conta.estado}) — conta_id: ${conta.id}`;
}

export async function contaPadrao(cliente: ClienteMotor, contaId?: string): Promise<string> {
  if (contaId) return contaId;
  const contas = await cliente.listarContas();
  const conectadas = contas.filter((conta) => conta.estado === 'conectada');
  if (conectadas.length === 1 && conectadas[0]) return conectadas[0].id;

  if (contas.length === 0) {
    throw new ErroFerramenta(
      'Nenhuma conta de WhatsApp cadastrada no ZapDesk. Conecte uma conta pela interface do app (QR code) e tente de novo.',
    );
  }
  const lista = listar(contas, descreverConta);
  if (conectadas.length === 0) {
    throw new ErroFerramenta(
      `Nenhuma conta está conectada agora. Reconecte pela interface do app ou informe conta_id.\nContas:\n${lista}`,
    );
  }
  throw new ErroFerramenta(
    `Há ${conectadas.length} contas conectadas; informe conta_id para escolher uma.\nContas:\n${lista}`,
  );
}
