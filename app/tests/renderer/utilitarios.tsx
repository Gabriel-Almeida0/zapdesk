// Utilitários dos testes do renderer: ClienteMotor simulado, fábricas de objetos do contrato e
// render com todos os providers.
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render } from '@testing-library/react';
import type { ReactElement } from 'react';
import { MemoryRouter, Route, Routes } from 'react-router';
import { vi } from 'vitest';

import type {
  ClienteMotor,
  Conta,
  Conversa,
  Disparo,
  Mensagem,
  Pagina,
  RelatorioImportacao,
  Template,
} from '@zapdesk/cliente-motor';

import { useSincronizarCache } from '../../src/renderer/api/eventos';
import { criarFonteManual, ProvedorMotor, type FonteEventos } from '../../src/renderer/api/motor';
import { ProvedorConta } from '../../src/renderer/estado/conta';

export type ClienteSimulado = ClienteMotor & Record<string, ReturnType<typeof vi.fn>>;

export function pagina<T>(itens: T[], proximo: string | null = null): Pagina<T> {
  return { itens, proximo_cursor: proximo };
}

/** Todo método não informado rejeita (o teste falha se usar algo inesperado sem querer). */
export function clienteSimulado(metodos: Record<string, (...args: never[]) => unknown> = {}): ClienteSimulado {
  const base: Record<string, unknown> = {
    porta: 1,
    token: 't',
    base: 'http://127.0.0.1:1',
    urlBinaria: (c: string) => `http://127.0.0.1:1${c}?token=t`,
    urlMidiaMensagem: (id: string) => `http://127.0.0.1:1/v1/mensagens/${id}/midia?token=t`,
    urlMidiaStatus: (id: string) => `http://127.0.0.1:1/v1/status/${id}/midia?token=t`,
    urlConteudoArquivo: (id: string) => `http://127.0.0.1:1/v1/arquivos/${id}/conteudo?token=t`,
    listarContas: vi.fn(async () => [conta()]),
    listarEtiquetas: vi.fn(async () => []),
    sistema: vi.fn(async () => ({
      versao: '0.1.0',
      pasta_dados: '/tmp/zapdesk',
      caminho_logs: '/tmp/zapdesk/logs/motor.log',
      whatsapp: 'falso',
      disparos_ativos: 0,
      contas_conectadas: 1,
      automacoes_ativas: 0,
      processos_ia: 0,
      runner_disponivel: false,
    })),
  };
  for (const [nome, fn] of Object.entries(metodos)) base[nome] = vi.fn(fn);
  return new Proxy(base, {
    get(alvo, chave: string) {
      if (!(chave in alvo)) {
        if (chave === 'then') return undefined;
        alvo[chave] = vi.fn(async () => {
          throw new Error(`método ${chave} não simulado`);
        });
      }
      return alvo[chave];
    },
  }) as ClienteSimulado;
}

function Sincronizar({ fonte }: { fonte: FonteEventos }) {
  useSincronizarCache(fonte);
  return null;
}

export function renderizar(
  ui: ReactElement,
  opcoes: { cliente: ClienteMotor; rota?: string; caminho?: string; fonte?: ReturnType<typeof criarFonteManual> },
) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity }, mutations: { retry: false } } });
  const fonte = opcoes.fonte ?? criarFonteManual();
  const resultado = render(
    <QueryClientProvider client={qc}>
      <ProvedorMotor cliente={opcoes.cliente} fonte={fonte} whatsapp="falso" versao="0.1.0">
        <Sincronizar fonte={fonte} />
        <ProvedorConta>
          <MemoryRouter initialEntries={[opcoes.rota ?? '/']}>
            <Routes>
              <Route path={opcoes.caminho ?? '*'} element={ui} />
            </Routes>
          </MemoryRouter>
        </ProvedorConta>
      </ProvedorMotor>
    </QueryClientProvider>,
  );
  return { ...resultado, qc, fonte };
}

// ---------------------------------------------------------------------------
// Fábricas
// ---------------------------------------------------------------------------

export function conta(parcial: Partial<Conta> = {}): Conta {
  return {
    id: 'conta1',
    nome: 'Comercial',
    telefone: '+5511900000001',
    jid: '5511900000001@s.whatsapp.net',
    estado: 'conectada',
    online: true,
    sincronizando: false,
    criada_em: '2026-09-27T10:00:00-03:00',
    ...parcial,
  };
}

export function conversa(parcial: Partial<Conversa> = {}): Conversa {
  return {
    id: 'conv1',
    conta_id: 'conta1',
    jid: '5511911112222@s.whatsapp.net',
    tipo: 'individual',
    nome: 'Ana',
    telefone: '+5511911112222',
    contato_id: 'contato1',
    nao_lidas: 0,
    ultima_mensagem_em: '2026-09-27T10:00:00-03:00',
    ultima_mensagem_resumo: 'oi',
    etiquetas: [],
    ...parcial,
  };
}

export function mensagem(parcial: Partial<Mensagem> = {}): Mensagem {
  return {
    id: 'm1',
    conta_id: 'conta1',
    conversa_id: 'conv1',
    wa_id: 'WA1',
    remetente_jid: '5511900000001@s.whatsapp.net',
    remetente_nome: null,
    de_mim: true,
    tipo: 'texto',
    texto: 'Olá',
    midia: null,
    citacao: null,
    reacoes: [],
    editada: false,
    apagada: false,
    estado: 'enviada',
    erro: null,
    disparo_id: null,
    automacao_id: null,
    enviada_em: '2026-09-27T10:00:00-03:00',
    pode_editar: true,
    pode_apagar: true,
    ...parcial,
  };
}

export function template(parcial: Partial<Template> = {}): Template {
  return {
    id: 't1',
    nome: 'Apresentação',
    texto: 'Oi {nome}, aqui é o Gabriel.',
    variaveis: ['nome'],
    arquivo: null,
    criado_em: '2026-09-27T10:00:00-03:00',
    atualizado_em: '2026-09-27T10:00:00-03:00',
    ...parcial,
  };
}

export function disparo(parcial: Partial<Disparo> = {}): Disparo {
  return {
    id: 'd1',
    conta_id: 'conta1',
    nome: 'Prospecção',
    mensagem: 'Oi {nome}',
    arquivo: null,
    ritmo: { intervalo_min_s: 30, intervalo_max_s: 90, limite_por_hora: 40, limite_por_dia: 300, pausa_a_cada: null, pausa_duracao_s: null },
    inicio_em: null,
    janela: null,
    falhas_seguidas_max: 10,
    valores_padrao: {},
    estado: 'enviando',
    na_fila: false,
    motivo_pausa: null,
    origem: 'app',
    contadores: { total: 10, pendente: 5, enviando: 0, enviado: 3, entregue: 1, lido: 0, respondeu: 0, falhou: 1 },
    proximo_envio_em: null,
    estimativa_termino_em: null,
    aviso_ritmo_agressivo: false,
    criado_em: '2026-09-27T10:00:00-03:00',
    iniciado_em: '2026-09-27T10:00:00-03:00',
    concluido_em: null,
    cancelado_em: null,
    ...parcial,
  };
}

export function relatorio(parcial: Partial<RelatorioImportacao> = {}): RelatorioImportacao {
  return {
    total_linhas: 0,
    total_novos: 0,
    total_ja_existentes: 0,
    total_invalidos: 0,
    total_duplicados_no_lote: 0,
    novos: [],
    ja_existentes: [],
    invalidos: [],
    duplicados_no_lote: [],
    lead_ids: [],
    ...parcial,
  };
}
