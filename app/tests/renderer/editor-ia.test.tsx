// T108 — editor de IA: salvar (automático e com hash), conflito com alteração externa, erros de
// compilação na lista de problemas e "Ativar" bloqueado com erro. O Monaco é trocado por um
// textarea (jsdom não roda o editor).
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { ErroMotor, type ArquivoProjeto, type ResultadoCompilacao } from '@zapdesk/cliente-motor';

import { validarCaminho, montarArvore } from '../../src/renderer/telas/EditorIA/ArvoreArquivos';
import { EditorIA, lerManifesto } from '../../src/renderer/telas/EditorIA/EditorIA';
import { montarPedidoTeste } from '../../src/renderer/telas/EditorIA/PainelTeste';
import { automacao, execucao } from './fabricas-automacoes';
import { clienteSimulado, renderizar } from './utilitarios';

import { posicaoNoEditor } from '../../src/renderer/monaco/posicao';
import { textoDuracaoMs } from '../../src/renderer/util/automacoes';

vi.mock('../../src/renderer/telas/EditorIA/Codigo', () => ({
  default: (props: { caminho: string; arquivos: Record<string, string>; aoMudar: (c: string, v: string) => void }) => (
    <textarea aria-label={`Código de ${props.caminho}`} value={props.arquivos[props.caminho] ?? ''} onChange={(e) => props.aoMudar(props.caminho, e.target.value)} />
  ),
}));

const MANIFESTO = JSON.stringify({ versao_manifesto: 1, nome: 'Responder', gatilhos: [{ tipo: 'mensagem_recebida' }], permissoes: ['enviar', 'ia'] });
const INDEX = "import { definirAutomacao } from '@zapdesk/automacao';\nexport default definirAutomacao({});\n";

function arquivo(caminho: string, hash: string): ArquivoProjeto {
  return { caminho, tamanho: 10, hash, atualizado_em: '2026-09-27T10:00:00-03:00' };
}

function compilacao(parcial: Partial<ResultadoCompilacao> = {}): ResultadoCompilacao {
  return { ok: true, erros: [], avisos: [], hash: 'h', handlers: ['aoReceberMensagem'], duracao_ms: 42, ...parcial };
}

function iaAutomacao(compilacaoOk = true) {
  return automacao({
    id: 'ia1',
    tipo: 'ia',
    nome: 'Responder com IA',
    definicao: null,
    ia: {
      pasta: '/tmp/zapdesk/automacoes/ia1',
      permissoes: ['enviar', 'ia'],
      segredos: [],
      hash_compilado: 'h',
      compilacao_ok: compilacaoOk,
      erros_compilacao: [],
      compilado_em: null,
      rodando_versao_anterior: false,
    },
  });
}

function base(extra: Record<string, (...a: never[]) => unknown> = {}) {
  const hashes: Record<string, string> = { 'automacao.json': 'm1', 'index.ts': 'i1' };
  const conteudos: Record<string, string> = { 'automacao.json': MANIFESTO, 'index.ts': INDEX };
  const cliente = clienteSimulado({
    listarArquivos: async () => Object.keys(hashes).map((c) => arquivo(c, hashes[c] ?? '')),
    lerArquivo: async (_id: string, c: string) => ({ caminho: c, conteudo: conteudos[c] ?? '', hash: hashes[c] ?? '', atualizado_em: '' }),
    escreverArquivo: async (_id: string, c: string, conteudo: string) => {
      hashes[c] = `${hashes[c] ?? 'x'}+`;
      conteudos[c] = conteudo;
      return arquivo(c, hashes[c] ?? '');
    },
    compilarAutomacao: async () => compilacao(),
    obterSdkAutomacao: async () => ({ versao: '1.0.0', tipos: 'declare module "@zapdesk/automacao" {}', esquema_manifesto: {} }),
    obterConfiguracaoIA: async () => ({ modelo_padrao: 'claude-sonnet-5', modelos: [], chave_configurada: true }),
    ...extra,
  });
  return { cliente, hashes, conteudos };
}

describe('EditorIA', () => {
  it('abre a entrada, salva sozinho depois de digitar (com o hash) e compila', async () => {
    const { cliente } = base();
    renderizar(<EditorIA automacao={iaAutomacao()} />, { cliente, rota: '/automacoes/ia1', caminho: '/automacoes/:automacaoId' });
    const codigo = (await screen.findByLabelText('Código de index.ts')) as HTMLTextAreaElement;
    expect(codigo.value).toContain('definirAutomacao');
    await userEvent.type(codigo, '// oi');
    await waitFor(() => expect(cliente.escreverArquivo).toHaveBeenCalledWith('ia1', 'index.ts', `${INDEX}// oi`, 'i1'), { timeout: 3000 });
    await waitFor(() => expect(cliente.compilarAutomacao).toHaveBeenCalledTimes(2));
    expect(await screen.findByText('Compilado (42 ms)')).toBeTruthy();
  });

  it('conflito: arquivo alterado fora do app avisa e só sobrescreve com confirmação', async () => {
    const { cliente } = base({
      escreverArquivo: vi.fn(async (_id: string, _c: string, _conteudo: string, hash?: string | null) => {
        if (hash !== undefined) throw new ErroMotor('conflito', 'O arquivo foi alterado fora do app.', 409, { hash_atual: 'externo' });
        return arquivo('index.ts', 'novo');
      }) as never,
    });
    renderizar(<EditorIA automacao={iaAutomacao()} />, { cliente, rota: '/automacoes/ia1', caminho: '/automacoes/:automacaoId' });
    const codigo = await screen.findByLabelText('Código de index.ts');
    await userEvent.type(codigo, 'x');
    expect(await screen.findByText('index.ts: arquivo alterado fora do app.', {}, { timeout: 3000 })).toBeTruthy();
    await userEvent.click(screen.getByRole('button', { name: 'Manter o meu' }));
    await userEvent.click(screen.getByRole('button', { name: 'Salvar por cima' }));
    await waitFor(() => expect(cliente.escreverArquivo).toHaveBeenLastCalledWith('ia1', 'index.ts', `${INDEX}x`, undefined));
  });

  it('erros de compilação aparecem nos problemas e bloqueiam "Ativar"', async () => {
    const { cliente } = base({
      compilarAutomacao: async () =>
        compilacao({ ok: false, hash: null, erros: [{ arquivo: 'index.ts', linha: 3, coluna: 7, mensagem: 'Módulo não permitido: fs', tipo: 'importacao' }] }),
    });
    renderizar(<EditorIA automacao={iaAutomacao(false)} />, { cliente, rota: '/automacoes/ia1', caminho: '/automacoes/:automacaoId' });
    expect(await screen.findByText('Módulo não permitido: fs')).toBeTruthy();
    expect(screen.getByText('index.ts:3:7')).toBeTruthy(); // coluna 1-base, sem somar 1
    expect((screen.getByRole('switch', { name: 'Ativar Responder com IA' }) as HTMLInputElement).disabled).toBe(true);
    expect(screen.getByText('1 erro(s) de compilação')).toBeTruthy();
  });

  it('Testar em simulação mostra ações, log e tokens', async () => {
    const { cliente } = base({ testarAutomacao: async () => ({ execucao: execucao() }) });
    renderizar(<EditorIA automacao={iaAutomacao()} />, { cliente, rota: '/automacoes/ia1', caminho: '/automacoes/:automacaoId' });
    await screen.findByLabelText('Código de index.ts');
    await userEvent.click(screen.getByRole('button', { name: 'Testar' }));
    await waitFor(() => expect(cliente.testarAutomacao).toHaveBeenCalledWith('ia1', { ia_simulada: false, mensagem: { texto: 'Quanto custa?' } }));
    expect(await screen.findByText("'O valor é R$ 10'")).toBeTruthy();
    expect(screen.getByText('olá do log')).toBeTruthy();
    expect(screen.getByText('120 de entrada · 30 de saída')).toBeTruthy();
  });

  it('"Rodando a versão anterior" só aparece com a automação ativa', async () => {
    const { cliente } = base();
    const inativa = iaAutomacao(false);
    inativa.ia = { ...inativa.ia!, rodando_versao_anterior: true };
    const { unmount } = renderizar(<EditorIA automacao={{ ...inativa, ativa: false }} />, { cliente, rota: '/automacoes/ia1', caminho: '/automacoes/:automacaoId' });
    await screen.findByLabelText('Código de index.ts');
    expect(screen.queryByText(/Rodando a versão anterior/)).toBeNull();
    unmount();
    renderizar(<EditorIA automacao={{ ...inativa, ativa: true }} />, { cliente, rota: '/automacoes/ia1', caminho: '/automacoes/:automacaoId' });
    expect(await screen.findByText(/Rodando a versão anterior/)).toBeTruthy();
  });

  it('duração: arredonda antes de separar minutos e segundos', () => {
    expect(textoDuracaoMs(850)).toBe('850 ms');
    expect(textoDuracaoMs(59_960)).toBe('1 min 0 s');
    expect(textoDuracaoMs(119_600)).toBe('2 min 0 s');
    expect(textoDuracaoMs(65_000)).toBe('1 min 5 s');
    expect(textoDuracaoMs(1_500)).toBe('1,5 s');
  });

  it('posição do erro do motor no editor é 1-base, sem somar 1 (marcador e lista iguais)', () => {
    expect(posicaoNoEditor({ linha: 1, coluna: 15 })).toEqual({ linha: 1, coluna: 15 });
    expect(posicaoNoEditor({ linha: 0, coluna: 0 })).toEqual({ linha: 1, coluna: 1 });
  });

  it('utilitários: manifesto, caminhos, árvore e pedido de teste', () => {
    expect(lerManifesto(MANIFESTO)).toEqual({ entrada: 'index.ts', permissoes: ['enviar', 'ia'] });
    expect(lerManifesto('{ quebrado')).toEqual({ entrada: 'index.ts', permissoes: [] });
    expect(validarCaminho('lib/util.ts')).toBeNull();
    expect(validarCaminho('../fora.ts')).toBeTruthy();
    expect(validarCaminho('.zapdesk/x.ts')).toBeTruthy();
    expect(validarCaminho('a.js')).toMatch(/Extensões/);
    expect(validarCaminho('a/b/c/d/e.ts')).toMatch(/níveis/);
    expect(montarArvore(['index.ts', 'lib/b.ts', 'automacao.json', 'lib/a.ts']).map((i) => i.caminho)).toEqual([
      'lib',
      'lib/a.ts',
      'lib/b.ts',
      'automacao.json',
      'index.ts',
    ]);
    const campos = { texto: 'oi', conversaId: null, mensagemId: null, entrada: '{"a":1}', eventoTipo: 'etiqueta', eventoDados: '{"x":1}', iaSimulada: true };
    expect(montarPedidoTeste('entrada', campos)).toEqual({ ia_simulada: true, entrada: { a: 1 } });
    expect(montarPedidoTeste('evento', campos)).toEqual({ ia_simulada: true, evento: { tipo: 'etiqueta', dados: { x: 1 } } });
    expect(() => montarPedidoTeste('conversa', campos)).toThrow(/Escolha/);
    expect(() => montarPedidoTeste('entrada', { ...campos, entrada: '{' })).toThrow(/JSON inválido/);
  });
});
