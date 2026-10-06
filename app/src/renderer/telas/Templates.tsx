// Templates (T122): CRUD com anexo opcional e variáveis detectadas; inseridos no chat com `/`.
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { FileText, Paperclip, Plus, Trash, X } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';

import type { Arquivo, Template } from '@zapdesk/cliente-motor';

import { chaves } from '../api/chaves';
import { useCliente } from '../api/motor';
import { BotaoIcone } from '../componentes/BotaoIcone';
import { EsqueletoLista } from '../componentes/Esqueleto';
import { EstadoVazio } from '../componentes/EstadoVazio';
import { FaixaAviso } from '../componentes/FaixaAviso';
import { Confirmar } from '../componentes/Modal';
import { TelaErro } from '../componentes/TelaErro';
import { checarLimiteAnexo, extrairVariaveis, tamanho, textoErro } from '../util/formatar';

export function validarTemplate(nome: string, texto: string): string | null {
  if (nome.trim().length < 1 || nome.trim().length > 60) return 'O nome deve ter de 1 a 60 caracteres.';
  if (texto.trim().length < 1) return 'Escreva o texto do template.';
  if (texto.length > 4096) return 'O texto passou de 4.096 caracteres.';
  return null;
}

function Editor(props: { template: Template | null; aoSalvo: (t: Template) => void; aoExcluir: (t: Template) => void }) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const [nome, setNome] = useState(props.template?.nome ?? '');
  const [texto, setTexto] = useState(props.template?.texto ?? '');
  const [arquivo, setArquivo] = useState<Arquivo | null>(props.template?.arquivo ?? null);
  const [erroAnexo, setErroAnexo] = useState<string | null>(null);
  const [tentou, setTentou] = useState(false);
  const entrada = useRef<HTMLInputElement>(null);

  useEffect(() => {
    setNome(props.template?.nome ?? '');
    setTexto(props.template?.texto ?? '');
    setArquivo(props.template?.arquivo ?? null);
    setTentou(false);
  }, [props.template]);

  const salvar = useMutation({
    mutationFn: () => {
      const dados = { nome: nome.trim(), texto, arquivo_id: arquivo?.id ?? null };
      return props.template ? cliente.editarTemplate(props.template.id, dados) : cliente.criarTemplate(dados);
    },
    onSuccess: (t) => {
      void qc.invalidateQueries({ queryKey: chaves.templates });
      props.aoSalvo(t);
    },
  });
  const anexar = useMutation({
    mutationFn: (a: File) => cliente.enviarArquivo({ dados: a, nome: a.name }),
    onSuccess: setArquivo,
  });

  const variaveis = extrairVariaveis(texto);
  const erroLocal = validarTemplate(nome, texto);

  return (
    <form
      className="formulario editor-template"
      onSubmit={(e) => {
        e.preventDefault();
        setTentou(true);
        if (!erroLocal) salvar.mutate();
      }}
    >
      <label className="campo">
        <span>Nome (use no chat digitando /nome)</span>
        <input value={nome} maxLength={60} placeholder="Ex.: Apresentação" onChange={(e) => setNome(e.target.value)} />
      </label>
      <label className="campo">
        <span>Texto</span>
        <textarea rows={10} maxLength={4096} value={texto} placeholder="Oi {nome}! Aqui é o Gabriel…" onChange={(e) => setTexto(e.target.value)} />
      </label>
      <div className="variaveis">
        <span className="texto-secundario">Variáveis detectadas:</span>
        {variaveis.length === 0 ? <span className="texto-secundario">nenhuma</span> : variaveis.map((v) => <span key={v} className="chip ativo">{`{${v}}`}</span>)}
      </div>
      <div className="campo">
        <span>Anexo (opcional)</span>
        {arquivo ? (
          <div className="anexo-escolhido">
            <FileText size={20} aria-hidden="true" />
            <span>
              {arquivo.nome} · {tamanho(arquivo.tamanho)}
            </span>
            <BotaoIcone rotulo="Remover anexo" onClick={() => setArquivo(null)}>
              <X size={16} />
            </BotaoIcone>
          </div>
        ) : (
          <button type="button" className="botao secundario" disabled={anexar.isPending} onClick={() => entrada.current?.click()}>
            <Paperclip size={16} aria-hidden="true" /> {anexar.isPending ? 'Enviando…' : 'Anexar arquivo'}
          </button>
        )}
        <input
          ref={entrada}
          type="file"
          hidden
          onChange={(e) => {
            const a = e.target.files?.[0];
            e.target.value = '';
            if (!a) return;
            const recusa = checarLimiteAnexo(a);
            setErroAnexo(recusa);
            if (!recusa) anexar.mutate(a);
          }}
        />
        {erroAnexo ? <FaixaAviso tipo="erro">{erroAnexo}</FaixaAviso> : null}
        {anexar.error ? <FaixaAviso tipo="erro">{textoErro(anexar.error)}</FaixaAviso> : null}
      </div>
      {tentou && erroLocal ? <FaixaAviso tipo="erro">{erroLocal}</FaixaAviso> : null}
      {salvar.error ? <FaixaAviso tipo="erro">{textoErro(salvar.error)}</FaixaAviso> : null}
      <div className="acoes-formulario">
        {props.template ? (
          <button type="button" className="botao secundario perigo-texto" onClick={() => props.template && props.aoExcluir(props.template)}>
            <Trash size={16} aria-hidden="true" /> Excluir
          </button>
        ) : null}
        <button type="submit" className="botao" disabled={salvar.isPending}>
          {salvar.isPending ? 'Salvando…' : 'Salvar template'}
        </button>
      </div>
    </form>
  );
}

export function Templates() {
  const cliente = useCliente();
  const qc = useQueryClient();
  const consulta = useQuery({ queryKey: chaves.templates, queryFn: () => cliente.listarTemplates() });
  const [selecionado, setSelecionado] = useState<string | 'novo' | null>(null);
  const [excluindo, setExcluindo] = useState<Template | null>(null);
  const excluir = useMutation({
    mutationFn: (id: string) => cliente.excluirTemplate(id),
    onSuccess: () => {
      setExcluindo(null);
      setSelecionado(null);
      void qc.invalidateQueries({ queryKey: chaves.templates });
    },
  });

  const lista = consulta.data ?? [];
  const atual = selecionado && selecionado !== 'novo' ? (lista.find((t) => t.id === selecionado) ?? null) : null;

  return (
    <section className="tela tela-dividida">
      <aside className="painel-lista">
        <header className="cabecalho-tela">
          <h1>Templates</h1>
          <BotaoIcone rotulo="Novo template" onClick={() => setSelecionado('novo')}>
            <Plus size={20} />
          </BotaoIcone>
        </header>
        {consulta.isPending ? (
          <EsqueletoLista linhas={5} />
        ) : consulta.isError ? (
          <TelaErro erro={consulta.error} aoTentar={() => void consulta.refetch()} />
        ) : lista.length === 0 ? (
          <EstadoVazio
            icone={<FileText size={48} />}
            titulo="Nenhum template"
            texto="Salve mensagens prontas e use no chat digitando /."
            acao={
              <button type="button" className="botao" onClick={() => setSelecionado('novo')}>
                Novo template
              </button>
            }
          />
        ) : (
          <ul className="lista-simples" aria-label="Templates">
            {lista.map((t) => (
              <li key={t.id}>
                <button type="button" className={`item-template${t.id === selecionado ? ' ativo' : ''}`} onClick={() => setSelecionado(t.id)}>
                  <strong>/{t.nome}</strong>
                  <span>{t.texto}</span>
                  {t.arquivo ? (
                    <small>
                      <Paperclip size={12} aria-hidden="true" /> {t.arquivo.nome}
                    </small>
                  ) : null}
                </button>
              </li>
            ))}
          </ul>
        )}
      </aside>
      <div className="painel-detalhe">
        {selecionado ? (
          <div className="cartao">
            <h2>{atual ? 'Editar template' : 'Novo template'}</h2>
            <Editor template={atual} aoSalvo={(t) => setSelecionado(t.id)} aoExcluir={setExcluindo} />
          </div>
        ) : (
          <EstadoVazio icone={<FileText size={56} />} titulo="Escolha um template" texto="Ou crie um novo com o botão +." />
        )}
        {excluir.error ? <FaixaAviso tipo="erro">{textoErro(excluir.error)}</FaixaAviso> : null}
      </div>
      {excluindo ? (
        <Confirmar
          titulo={`Excluir "${excluindo.nome}"?`}
          texto="O template será apagado. Disparos já criados não mudam."
          confirmar="Excluir"
          perigo
          ocupado={excluir.isPending}
          aoConfirmar={() => excluir.mutate(excluindo.id)}
          aoFechar={() => setExcluindo(null)}
        />
      ) : null}
    </section>
  );
}
