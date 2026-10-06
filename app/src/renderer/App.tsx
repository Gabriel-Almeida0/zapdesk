// Raiz do renderer: acompanha o estado do motor (preload) e, quando pronto, monta o cliente,
// o cache (TanStack Query) alimentado pelos eventos WS e as rotas.
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { HashRouter } from 'react-router';

import { ClienteMotor, ErroMotor } from '@zapdesk/cliente-motor';

import type { EstadoMotor } from '../preload/tipos';
import type { PonteZapDesk } from '../preload/index';
import { useSincronizarCache } from './api/eventos';
import { criarFetchIpc } from './api/fetch-ipc';
import { ProvedorMotor, type FonteEventos } from './api/motor';
import { ProvedorConta } from './estado/conta';
import { Rotas } from './rotas';
import { Carregando } from './telas/Carregando';
import { ErroMotorTela } from './telas/ErroMotor';

export function criarQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        refetchOnWindowFocus: false,
        // 4xx do motor não melhora tentando de novo.
        retry: (tentativas, erro) =>
          tentativas < 2 && !(erro instanceof ErroMotor && erro.status >= 400 && erro.status < 500),
      },
      mutations: { retry: false },
    },
  });
}

function SincronizarCache({ fonte }: { fonte: FonteEventos }) {
  useSincronizarCache(fonte);
  return null;
}

type PonteApp = Pick<
  PonteZapDesk,
  'obterConexao' | 'aoMudarEstadoMotor' | 'reiniciarMotor' | 'requisitarMotor' | 'aoEventoMotor' | 'aoRecarregar'
>;

function useEstadoMotor(ponte: PonteApp): EstadoMotor {
  const [estado, setEstado] = useState<EstadoMotor>({ fase: 'ligando' });
  useEffect(() => {
    let ativo = true;
    const parar = ponte.aoMudarEstadoMotor((e) => setEstado(e));
    void ponte.obterConexao().then((e) => {
      if (ativo) setEstado(e);
    });
    return () => {
      ativo = false;
      parar();
    };
  }, [ponte]);
  return estado;
}

export function App({ ponte = window.zapdesk }: { ponte?: PonteApp }) {
  const estado = useEstadoMotor(ponte);
  const conexao = estado.fase === 'pronto' ? estado.conexao : null;

  // Um cliente e um cache novos a cada conexão (o motor religado muda porta e token).
  const cliente = useMemo(
    () =>
      conexao
        ? new ClienteMotor({ porta: conexao.porta, token: conexao.token, fetch: criarFetchIpc(ponte) })
        : null,
    [conexao, ponte],
  );
  const queryClient = useMemo(() => (conexao ? criarQueryClient() : null), [conexao]);

  if (estado.fase === 'ligando') return <Carregando texto="Ligando o WhatsApp…" />;
  if (estado.fase === 'religando') return <Carregando texto="O WhatsApp parou. Religando…" />;
  if (estado.fase === 'erro') {
    return <ErroMotorTela mensagem={estado.mensagem} detalhe={estado.detalhe} aoTentar={() => ponte.reiniciarMotor()} />;
  }
  if (!cliente || !queryClient || !conexao) return <Carregando texto="Ligando o WhatsApp…" />;

  return (
    <QueryClientProvider client={queryClient}>
      <ProvedorMotor cliente={cliente} fonte={ponte} whatsapp={conexao.whatsapp} versao={conexao.versao}>
        <SincronizarCache fonte={ponte} />
        <ProvedorConta>
          <HashRouter>
            <Rotas />
          </HashRouter>
        </ProvedorConta>
      </ProvedorMotor>
    </QueryClientProvider>
  );
}
