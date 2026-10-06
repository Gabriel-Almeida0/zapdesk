// Abre uma conversa de qualquer conta: seleciona a conta dela e navega (Kanban, notificações).
import { useCallback } from 'react';
import { useNavigate } from 'react-router';

import type { Id } from '@zapdesk/cliente-motor';

import { useCliente } from '../api/motor';
import { useContaAtual } from '../estado/conta';

export function useAbrirConversa(): (conversaId: Id, contaId?: Id | null) => Promise<void> {
  const cliente = useCliente();
  const { selecionar, contaId: atual } = useContaAtual();
  const navegar = useNavigate();
  return useCallback(
    async (conversaId: Id, contaId?: Id | null) => {
      let conta = contaId ?? null;
      if (!conta) {
        try {
          conta = (await cliente.obterConversa(conversaId)).conta_id;
        } catch {
          conta = null;
        }
      }
      if (conta && conta !== atual) selecionar(conta);
      await navegar(`/conversas/${conversaId}`);
    },
    [cliente, selecionar, atual, navegar],
  );
}
