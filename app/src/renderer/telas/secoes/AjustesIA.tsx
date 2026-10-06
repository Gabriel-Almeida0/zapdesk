// Ajustes → IA (T107): chave da Anthropic (Keychain via safeStorage, mascarada, testar/remover),
// modelo padrão com aviso de aposentadoria e segredos com nome ("usado por"). Os valores entram
// pela janela e vão direto para o processo principal; nunca voltam para a tela.
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { KeyRound, Trash } from 'lucide-react';
import { useState } from 'react';

import type { ResultadoSegredos, SegredoLocal } from '../../../preload/tipos';
import { chaves } from '../../api/chaves';
import { useCliente } from '../../api/motor';
import { BotaoIcone } from '../../componentes/BotaoIcone';
import { Esqueleto } from '../../componentes/Esqueleto';
import { FaixaAviso } from '../../componentes/FaixaAviso';
import { MODELOS_CLAUDE_PADRAO } from '../../util/automacoes';
import { textoErro } from '../../util/formatar';

export const CHAVE_ANTHROPIC = 'ANTHROPIC_API_KEY';
const NOME_SEGREDO = /^[A-Z][A-Z0-9_]{0,63}$/;

/** Ponte de segredos do preload (ausente fora do Electron). */
export interface PonteSegredos {
  listar(): Promise<SegredoLocal[]>;
  definir(nome: string, valor: string): Promise<ResultadoSegredos>;
  remover(nome: string): Promise<ResultadoSegredos>;
}

function ponte(): PonteSegredos {
  const p = typeof window !== 'undefined' ? window.zapdesk?.segredos : undefined;
  if (!p) throw new Error('Os segredos só podem ser guardados pelo app do ZapDesk.');
  return p;
}

export function validarNomeSegredo(nome: string): string | null {
  if (nome === CHAVE_ANTHROPIC) return 'Use o campo "Chave da Anthropic" acima.';
  if (!NOME_SEGREDO.test(nome)) return 'Use letras maiúsculas, números e _ (começando por letra), até 64 caracteres.';
  return null;
}

function useSegredosLocais() {
  return useQuery({ queryKey: chaves.segredosLocais, queryFn: () => ponte().listar() });
}

function useAlterarSegredo() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (d: { nome: string; valor: string | null }) => {
      const r = d.valor === null ? await ponte().remover(d.nome) : await ponte().definir(d.nome, d.valor);
      if (r.erro) throw new Error(r.erro);
      return r.segredos;
    },
    onSuccess: (lista) => {
      qc.setQueryData(chaves.segredosLocais, lista);
      void qc.invalidateQueries({ queryKey: chaves.segredos });
      void qc.invalidateQueries({ queryKey: chaves.configuracaoIA });
    },
  });
}

function ChaveAnthropic({ locais }: { locais: SegredoLocal[] }) {
  const cliente = useCliente();
  const [valor, setValor] = useState('');
  const [trocando, setTrocando] = useState(false);
  const alterar = useAlterarSegredo();
  const testar = useMutation({ mutationFn: () => cliente.testarChaveIA() });
  const atual = locais.find((s) => s.nome === CHAVE_ANTHROPIC);

  const salvar = () => {
    const v = valor.trim();
    if (!v) return;
    alterar.mutate(
      { nome: CHAVE_ANTHROPIC, valor: v },
      {
        onSuccess: () => {
          setValor('');
          setTrocando(false);
          testar.reset();
        },
      },
    );
  };

  return (
    <section className="painel-secao" aria-labelledby="titulo-chave">
      <h3 id="titulo-chave">
        <KeyRound size={16} aria-hidden="true" /> Chave da Anthropic
      </h3>
      {atual && !trocando ? (
        <div className="linha-chave">
          <code aria-label="Chave guardada">{atual.mascara}</code>
          <button type="button" className="botao secundario pequeno" disabled={testar.isPending} onClick={() => testar.mutate()}>
            {testar.isPending ? 'Testando…' : 'Testar chave'}
          </button>
          <button type="button" className="botao secundario pequeno" onClick={() => setTrocando(true)}>
            Trocar
          </button>
          <button
            type="button"
            className="botao secundario pequeno perigo-texto"
            disabled={alterar.isPending}
            onClick={() => alterar.mutate({ nome: CHAVE_ANTHROPIC, valor: null }, { onSuccess: () => testar.reset() })}
          >
            Remover
          </button>
        </div>
      ) : (
        <form
          className="linha-form"
          onSubmit={(e) => {
            e.preventDefault();
            salvar();
          }}
        >
          <input
            type="password"
            aria-label="Valor da chave da Anthropic"
            autoComplete="off"
            spellCheck={false}
            placeholder="sk-ant-…"
            value={valor}
            onChange={(e) => setValor(e.target.value)}
          />
          <button type="submit" className="botao pequeno" disabled={!valor.trim() || alterar.isPending}>
            Salvar chave
          </button>
          {trocando ? (
            <button type="button" className="botao secundario pequeno" onClick={() => setTrocando(false)}>
              Cancelar
            </button>
          ) : null}
        </form>
      )}
      {testar.isSuccess ? (
        <FaixaAviso tipo="info">
          A chave funciona ({testar.data.modelo}, {testar.data.latencia_ms} ms).
        </FaixaAviso>
      ) : null}
      {testar.error ? <FaixaAviso tipo="erro">{textoErro(testar.error)}</FaixaAviso> : null}
      {alterar.error ? <FaixaAviso tipo="erro">{textoErro(alterar.error)}</FaixaAviso> : null}
      <p className="texto-secundario">
        Guardada no chaveiro do macOS. Depois de salva, só aparece mascarada. Usada apenas por automações de IA que chamam{' '}
        <code>ctx.ia</code>; o código da automação nunca vê a chave.
      </p>
    </section>
  );
}

function ModeloPadrao() {
  const cliente = useCliente();
  const qc = useQueryClient();
  const config = useQuery({ queryKey: chaves.configuracaoIA, queryFn: () => cliente.obterConfiguracaoIA() });
  const salvar = useMutation({
    mutationFn: (modelo: string) => cliente.editarConfiguracaoIA({ modelo_padrao: modelo }),
    onSuccess: (c) => qc.setQueryData(chaves.configuracaoIA, c),
  });
  if (config.isPending) return <Esqueleto altura={40} />;
  if (config.isError) return <FaixaAviso tipo="erro">{textoErro(config.error)}</FaixaAviso>;
  const modelos = config.data.modelos.length > 0 ? config.data.modelos : MODELOS_CLAUDE_PADRAO.map((m) => ({ ...m }));
  const escolhido = modelos.find((m) => m.id === config.data.modelo_padrao);
  return (
    <section className="painel-secao">
      <label className="campo">
        <span>Modelo padrão</span>
        <select value={config.data.modelo_padrao} disabled={salvar.isPending} onChange={(e) => salvar.mutate(e.target.value)}>
          {modelos.map((m) => (
            <option key={m.id} value={m.id}>
              {m.nome}
            </option>
          ))}
        </select>
        <small className="texto-secundario">O código pode escolher outro modelo por chamada.</small>
      </label>
      {escolhido?.aviso ? <FaixaAviso tipo="aviso">{escolhido.aviso}</FaixaAviso> : null}
      {salvar.error ? <FaixaAviso tipo="erro">{textoErro(salvar.error)}</FaixaAviso> : null}
    </section>
  );
}

function SegredosComNome({ locais }: { locais: SegredoLocal[] }) {
  const cliente = useCliente();
  const doMotor = useQuery({ queryKey: chaves.segredos, queryFn: () => cliente.listarSegredos() });
  const alterar = useAlterarSegredo();
  const [nome, setNome] = useState('');
  const [valor, setValor] = useState('');
  const [tentou, setTentou] = useState(false);
  const outros = locais.filter((s) => s.nome !== CHAVE_ANTHROPIC);
  const erroNome = validarNomeSegredo(nome.trim());

  return (
    <section className="painel-secao" aria-labelledby="titulo-segredos">
      <h3 id="titulo-segredos">Outros segredos</h3>
      <p className="texto-secundario">
        Chaves de outros serviços que o código lê com <code>ctx.segredos.obter("NOME")</code>. A automação só recebe os segredos que declara em{' '}
        <code>automacao.json</code>.
      </p>
      {outros.length === 0 ? (
        <p className="texto-secundario">Nenhum segredo.</p>
      ) : (
        <ul className="lista-segredos">
          {outros.map((s) => {
            const uso = doMotor.data?.find((m) => m.nome === s.nome)?.usado_por ?? [];
            return (
              <li key={s.nome}>
                <code>{s.nome}</code>
                <span className="texto-secundario">{s.mascara}</span>
                <small className="texto-secundario">{uso.length > 0 ? `usado por ${uso.map((u) => u.nome).join(', ')}` : 'não usado'}</small>
                <BotaoIcone rotulo={`Remover segredo ${s.nome}`} className="pequeno" disabled={alterar.isPending} onClick={() => alterar.mutate({ nome: s.nome, valor: null })}>
                  <Trash size={14} />
                </BotaoIcone>
              </li>
            );
          })}
        </ul>
      )}
      <form
        className="linha-form"
        onSubmit={(e) => {
          e.preventDefault();
          setTentou(true);
          if (erroNome || !valor) return;
          alterar.mutate(
            { nome: nome.trim(), valor },
            {
              onSuccess: () => {
                setNome('');
                setValor('');
                setTentou(false);
              },
            },
          );
        }}
      >
        <input aria-label="Nome do segredo" className="campo-codigo" placeholder="OPENAI_API_KEY" value={nome} onChange={(e) => setNome(e.target.value.toUpperCase())} />
        <input type="password" aria-label="Valor do segredo" autoComplete="off" placeholder="valor" value={valor} onChange={(e) => setValor(e.target.value)} />
        <button type="submit" className="botao pequeno" disabled={alterar.isPending}>
          Adicionar
        </button>
      </form>
      {tentou && erroNome ? <p className="erro-campo">{erroNome}</p> : null}
      {tentou && !valor ? <p className="erro-campo">Informe o valor.</p> : null}
    </section>
  );
}

export function AjustesIA() {
  const locais = useSegredosLocais();
  if (locais.isPending) return <Esqueleto altura={120} />;
  if (locais.isError) return <FaixaAviso tipo="erro">{textoErro(locais.error)}</FaixaAviso>;
  return (
    <>
      <ChaveAnthropic locais={locais.data} />
      <ModeloPadrao />
      <SegredosComNome locais={locais.data} />
    </>
  );
}
