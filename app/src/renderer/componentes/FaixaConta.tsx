// Faixa de estado da conta atual: desconectada (amarela, "Reconectar"), banida, sem rede.
import { useMutation } from '@tanstack/react-query';
import { Ban, TriangleAlert, WifiOff } from 'lucide-react';
import { useNavigate } from 'react-router';

import type { Conta } from '@zapdesk/cliente-motor';

import { useCliente } from '../api/motor';
import { FaixaAviso } from './FaixaAviso';

export function FaixaConta({ conta }: { conta: Conta | null }) {
  const cliente = useCliente();
  const navegar = useNavigate();
  const reconectar = useMutation({
    mutationFn: (id: string) => cliente.reconectarConta(id),
    onSuccess: (c) => navegar(`/contas/conectar?conta=${encodeURIComponent(c.id)}`),
  });

  if (!conta) return null;
  if (conta.estado === 'desconectada') {
    return (
      <FaixaAviso
        tipo="aviso"
        icone={<TriangleAlert size={18} />}
        acao={
          <button
            type="button"
            className="botao pequeno"
            disabled={reconectar.isPending}
            onClick={() => reconectar.mutate(conta.id)}
          >
            {reconectar.isPending ? 'Reconectando…' : 'Reconectar'}
          </button>
        }
      >
        Conta desconectada. Reconecte para continuar.
      </FaixaAviso>
    );
  }
  if (conta.estado === 'banida') {
    return (
      <FaixaAviso tipo="erro" icone={<Ban size={18} />}>
        O WhatsApp bloqueou este número.
      </FaixaAviso>
    );
  }
  if (conta.estado === 'conectada' && !conta.online) {
    return (
      <FaixaAviso tipo="info" icone={<WifiOff size={18} />}>
        Sem conexão. Tentando reconectar…
      </FaixaAviso>
    );
  }
  if (conta.sincronizando) {
    return <FaixaAviso tipo="info">Sincronizando histórico…</FaixaAviso>;
  }
  return null;
}
