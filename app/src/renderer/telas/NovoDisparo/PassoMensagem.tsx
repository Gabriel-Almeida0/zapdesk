// Passo 2 — Mensagem (T096, T126): editor com chips das variáveis disponíveis, template, anexo via
// `POST /v1/arquivos` (recusa acima do limite) e prévia com uma linha real via `/disparos/validar`.
import { useQueries, useQuery } from '@tanstack/react-query';
import { FileText, Paperclip, X } from 'lucide-react';
import { useMemo, useRef, useState } from 'react';

import type { Arquivo } from '@zapdesk/cliente-motor';

import { chaves } from '../../api/chaves';
import { useCliente } from '../../api/motor';
import { BotaoIcone } from '../../componentes/BotaoIcone';
import { FaixaAviso } from '../../componentes/FaixaAviso';
import { LIMITE_MENSAGEM, type FormDisparo } from '../../util/disparos';
import { checarLimiteAnexo, extrairVariaveis, normalizarChave, tamanho, telefone, textoErro } from '../../util/formatar';
import { useValidacaoDisparo } from './validacao';

const AMOSTRA_LEADS = 15;

export function PassoMensagem(props: {
  form: FormDisparo;
  arquivo: Arquivo | null;
  aoMudarForm: (parcial: Partial<FormDisparo>) => void;
  aoMudarArquivo: (arquivo: Arquivo | null) => void;
}) {
  const cliente = useCliente();
  const campo = useRef<HTMLTextAreaElement>(null);
  const entrada = useRef<HTMLInputElement>(null);
  const [erroAnexo, setErroAnexo] = useState<string | null>(null);
  const [enviando, setEnviando] = useState(false);

  const templates = useQuery({ queryKey: chaves.templates, queryFn: () => cliente.listarTemplates() });

  // Variáveis disponíveis: {nome} + colunas extras de uma amostra dos leads escolhidos.
  const amostra = useQueries({
    queries: props.form.leadIds.slice(0, AMOSTRA_LEADS).map((id) => ({
      queryKey: chaves.lead(id),
      queryFn: () => cliente.obterLead(id),
      staleTime: 60_000,
    })),
  });
  const disponiveis = useMemo(() => {
    const chavesCampos = new Set<string>(['nome']);
    for (const r of amostra) for (const k of Object.keys(r.data?.campos ?? {})) chavesCampos.add(normalizarChave(k));
    return [...chavesCampos];
  }, [amostra]);

  const usadas = extrairVariaveis(props.form.mensagem);
  const validacao = useValidacaoDisparo(props.form, props.form.mensagem.trim().length > 0);

  const inserirVariavel = (v: string) => {
    const el = campo.current;
    const texto = props.form.mensagem;
    const inicio = el?.selectionStart ?? texto.length;
    const fim = el?.selectionEnd ?? texto.length;
    const novo = `${texto.slice(0, inicio)}{${v}}${texto.slice(fim)}`;
    props.aoMudarForm({ mensagem: novo });
    requestAnimationFrame(() => {
      el?.focus();
      const pos = inicio + v.length + 2;
      el?.setSelectionRange(pos, pos);
    });
  };

  const anexar = async (arquivo: File) => {
    setErroAnexo(null);
    const recusa = checarLimiteAnexo(arquivo);
    if (recusa) {
      setErroAnexo(recusa);
      return;
    }
    setEnviando(true);
    try {
      const enviado = await cliente.enviarArquivo({ dados: arquivo, nome: arquivo.name });
      props.aoMudarArquivo(enviado);
    } catch (e) {
      setErroAnexo(textoErro(e));
    } finally {
      setEnviando(false);
    }
  };

  const previa = validacao.data?.previa;

  return (
    <div className="passo passo-mensagem">
      <div className="coluna-editor">
        <label className="campo">
          <span>Usar um template</span>
          <select
            value=""
            onChange={(e) => {
              const t = templates.data?.find((x) => x.id === e.target.value);
              if (!t) return;
              props.aoMudarForm({ mensagem: t.texto });
              if (t.arquivo) props.aoMudarArquivo(t.arquivo);
            }}
          >
            <option value="">{templates.isPending ? 'Carregando…' : 'Escolher template…'}</option>
            {(templates.data ?? []).map((t) => (
              <option key={t.id} value={t.id}>
                {t.nome}
              </option>
            ))}
          </select>
        </label>

        <label className="campo">
          <span>Mensagem</span>
          <textarea
            ref={campo}
            rows={8}
            maxLength={LIMITE_MENSAGEM}
            placeholder="Oi {nome}, tudo bem?"
            value={props.form.mensagem}
            onChange={(e) => props.aoMudarForm({ mensagem: e.target.value })}
          />
          <small className="texto-secundario">
            {props.form.mensagem.length.toLocaleString('pt-BR')}/{LIMITE_MENSAGEM.toLocaleString('pt-BR')} · use {'{{'} e {'}}'} para chaves literais
          </small>
        </label>

        <div className="variaveis" role="group" aria-label="Variáveis disponíveis">
          <span className="texto-secundario">Variáveis:</span>
          {disponiveis.map((v) => (
            <button key={v} type="button" className={`chip${usadas.includes(v) ? ' ativo' : ''}`} onClick={() => inserirVariavel(v)}>
              {`{${v}}`}
            </button>
          ))}
        </div>
        {usadas.filter((v) => !disponiveis.includes(v)).length > 0 ? (
          <FaixaAviso tipo="aviso">
            {usadas
              .filter((v) => !disponiveis.includes(v))
              .map((v) => `{${v}}`)
              .join(', ')}{' '}
            não aparece nos leads de amostra. Você poderá definir um valor padrão na revisão.
          </FaixaAviso>
        ) : null}

        <div className="campo">
          <span>Anexo (opcional)</span>
          {props.arquivo ? (
            <div className="anexo-escolhido">
              <FileText size={20} aria-hidden="true" />
              <span>
                {props.arquivo.nome} · {tamanho(props.arquivo.tamanho)}
              </span>
              <BotaoIcone rotulo="Remover anexo" onClick={() => props.aoMudarArquivo(null)}>
                <X size={16} />
              </BotaoIcone>
            </div>
          ) : (
            <button type="button" className="botao secundario" disabled={enviando} onClick={() => entrada.current?.click()}>
              <Paperclip size={16} aria-hidden="true" /> {enviando ? 'Enviando…' : 'Anexar arquivo'}
            </button>
          )}
          <input
            ref={entrada}
            type="file"
            hidden
            onChange={(e) => {
              const a = e.target.files?.[0];
              if (a) void anexar(a);
              e.target.value = '';
            }}
          />
          <small className="texto-secundario">Imagem, vídeo e áudio até 16 MB; documento até 100 MB. Com anexo, o texto vira legenda.</small>
          {erroAnexo ? <FaixaAviso tipo="erro">{erroAnexo}</FaixaAviso> : null}
        </div>
      </div>

      <aside className="coluna-previa" aria-label="Prévia">
        <h3>Prévia</h3>
        <div className="previa-chat">
          {previa ? (
            <>
              <small className="texto-secundario">
                Para {previa.nome ?? telefone(previa.telefone)} ({telefone(previa.telefone)})
              </small>
              <div className="bolha previa-bolha">
                {props.arquivo ? (
                  <span className="previa-anexo">
                    <Paperclip size={14} aria-hidden="true" /> {props.arquivo.nome}
                  </span>
                ) : null}
                <span className="texto-mensagem">{previa.texto_resolvido}</span>
              </div>
            </>
          ) : validacao.isFetching ? (
            <p role="status">Montando prévia…</p>
          ) : validacao.error ? (
            <FaixaAviso tipo="erro">{textoErro(validacao.error)}</FaixaAviso>
          ) : (
            <p className="texto-secundario">Escreva a mensagem para ver como ela chega a um lead real da lista.</p>
          )}
        </div>
        {validacao.data && validacao.data.faltando.length > 0 ? (
          <FaixaAviso tipo="aviso">
            {validacao.data.faltando.length} {validacao.data.faltando.length === 1 ? 'contato' : 'contatos'} sem valor para alguma
            variável. Resolva na revisão.
          </FaixaAviso>
        ) : null}
      </aside>
    </div>
  );
}
