// Importar leads (T076): abas Arquivo (.csv/.xlsx → prévia → mapear telefone obrigatório, nome e
// extras), Colar números (um por linha) e Contatos/etiquetas. Os formulários também são usados no
// passo Lista do Novo disparo.
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, ClipboardPaste, FileSpreadsheet, Upload, Users } from 'lucide-react';
import { useMemo, useState, type ReactNode } from 'react';
import { useNavigate } from 'react-router';

import type { Contato, Id, PreviaImportacao, RelatorioImportacao as Relatorio } from '@zapdesk/cliente-motor';

import { chaves } from '../api/chaves';
import { useCliente } from '../api/motor';
import { BotaoIcone } from '../componentes/BotaoIcone';
import { FaixaAviso } from '../componentes/FaixaAviso';
import { RelatorioImportacao } from '../componentes/RelatorioImportacao';
import { useContaAtual } from '../estado/conta';
import { numero, textoErro } from '../util/formatar';
import { nomeContato } from './Contatos';
import { useAtraso } from './Conversas';

export const ERRO_SEM_TELEFONE = 'Escolha qual coluna tem o telefone.';

type AoImportar = (relatorio: Relatorio) => void;

// ---------------------------------------------------------------------------
// Arquivo
// ---------------------------------------------------------------------------

export function FormArquivo({ aoImportar }: { aoImportar: AoImportar }) {
  const cliente = useCliente();
  const [previa, setPrevia] = useState<PreviaImportacao | null>(null);
  const [telefoneCol, setTelefoneCol] = useState('');
  const [nomeCol, setNomeCol] = useState('');
  const [extras, setExtras] = useState<string[]>([]);
  const [tentouSemTelefone, setTentouSemTelefone] = useState(false);

  const enviar = useMutation({
    mutationFn: (arquivo: File) => cliente.previaImportacao({ dados: arquivo, nome: arquivo.name }),
    onSuccess: (p) => {
      setPrevia(p);
      setTelefoneCol(p.coluna_telefone_sugerida ?? '');
      setNomeCol(p.coluna_nome_sugerida ?? '');
      setExtras(p.colunas.filter((c) => c !== p.coluna_telefone_sugerida && c !== p.coluna_nome_sugerida));
      setTentouSemTelefone(false);
    },
  });

  const importar = useMutation({
    mutationFn: (p: PreviaImportacao) =>
      cliente.importarLeads({
        importacao_id: p.importacao_id,
        mapeamento: { telefone: telefoneCol, nome: nomeCol || null, extras },
      }),
    onSuccess: aoImportar,
  });

  const outras = previa?.colunas.filter((c) => c !== telefoneCol && c !== nomeCol) ?? [];

  return (
    <div className="form-importacao">
      <label className="area-arquivo">
        <Upload size={28} aria-hidden="true" />
        <span>{previa ? `${previa.nome_arquivo} · ${numero(previa.total_linhas)} linhas` : 'Escolher planilha .csv ou .xlsx'}</span>
        <small>A primeira linha deve ter os nomes das colunas.</small>
        <input
          type="file"
          accept=".csv,.xlsx,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
          aria-label="Planilha de leads"
          onChange={(e) => {
            const arquivo = e.target.files?.[0];
            if (arquivo) enviar.mutate(arquivo);
            e.target.value = '';
          }}
        />
      </label>
      {enviar.isPending ? <p role="status">Lendo planilha…</p> : null}
      {enviar.error ? <FaixaAviso tipo="erro">{textoErro(enviar.error)}</FaixaAviso> : null}

      {previa ? (
        <>
          <div className="grade-campos">
            <label className="campo">
              <span>Coluna do telefone *</span>
              <select
                value={telefoneCol}
                aria-invalid={tentouSemTelefone && !telefoneCol}
                onChange={(e) => {
                  setTelefoneCol(e.target.value);
                  setExtras((x) => x.filter((c) => c !== e.target.value));
                }}
              >
                <option value="">Escolha…</option>
                {previa.colunas.map((c) => (
                  <option key={c} value={c}>
                    {c}
                  </option>
                ))}
              </select>
            </label>
            <label className="campo">
              <span>Coluna do nome</span>
              <select
                value={nomeCol}
                onChange={(e) => {
                  setNomeCol(e.target.value);
                  setExtras((x) => x.filter((c) => c !== e.target.value));
                }}
              >
                <option value="">Nenhuma</option>
                {previa.colunas
                  .filter((c) => c !== telefoneCol)
                  .map((c) => (
                    <option key={c} value={c}>
                      {c}
                    </option>
                  ))}
              </select>
            </label>
          </div>
          {outras.length > 0 ? (
            <fieldset className="campo">
              <legend>Outras colunas (viram variáveis, ex.: {'{empresa}'})</legend>
              <div className="lista-chips">
                {outras.map((c) => (
                  <label key={c} className="caixa">
                    <input
                      type="checkbox"
                      checked={extras.includes(c)}
                      onChange={(e) => setExtras((x) => (e.target.checked ? [...x, c] : x.filter((y) => y !== c)))}
                    />
                    {c}
                  </label>
                ))}
              </div>
            </fieldset>
          ) : null}
          <div className="tabela-rolavel">
            <table className="tabela compacta" aria-label="Amostra da planilha">
              <thead>
                <tr>
                  {previa.colunas.map((c) => (
                    <th key={c} className={c === telefoneCol ? 'coluna-telefone' : c === nomeCol ? 'coluna-nome' : undefined}>
                      {c}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {previa.amostra.map((linha, i) => (
                  <tr key={i}>
                    {previa.colunas.map((_, j) => (
                      <td key={j}>{linha[j] ?? ''}</td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {tentouSemTelefone && !telefoneCol ? <FaixaAviso tipo="erro">{ERRO_SEM_TELEFONE}</FaixaAviso> : null}
          {importar.error ? <FaixaAviso tipo="erro">{textoErro(importar.error)}</FaixaAviso> : null}
          <button
            type="button"
            className="botao"
            disabled={importar.isPending}
            onClick={() => {
              if (!telefoneCol) {
                setTentouSemTelefone(true);
                return;
              }
              importar.mutate(previa);
            }}
          >
            {importar.isPending ? 'Importando…' : `Importar ${numero(previa.total_linhas)} linhas`}
          </button>
        </>
      ) : null}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Colar números
// ---------------------------------------------------------------------------

export function FormColar({ aoImportar }: { aoImportar: AoImportar }) {
  const cliente = useCliente();
  const [texto, setTexto] = useState('');
  const linhas = texto.split('\n').filter((l) => l.trim()).length;
  const importar = useMutation({
    mutationFn: () => cliente.importarLeads({ texto_colado: texto }),
    onSuccess: (r) => {
      aoImportar(r);
    },
  });
  return (
    <div className="form-importacao">
      <label className="campo">
        <span>Um número por linha (sem DDI, vale +55)</span>
        <textarea
          rows={10}
          className="campo-codigo"
          placeholder={'11 99999-0000\n+55 21 98888-7777'}
          value={texto}
          onChange={(e) => setTexto(e.target.value)}
        />
      </label>
      {importar.error ? <FaixaAviso tipo="erro">{textoErro(importar.error)}</FaixaAviso> : null}
      <button type="button" className="botao" disabled={linhas === 0 || importar.isPending} onClick={() => importar.mutate()}>
        {importar.isPending ? 'Importando…' : `Importar ${numero(linhas)} ${linhas === 1 ? 'número' : 'números'}`}
      </button>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Contatos e etiquetas
// ---------------------------------------------------------------------------

export function FormContatos({ aoImportar, contaId }: { aoImportar: AoImportar; contaId: Id | null }) {
  const cliente = useCliente();
  const [etiquetaIds, setEtiquetaIds] = useState<Id[]>([]);
  const [contatoIds, setContatoIds] = useState<Id[]>([]);
  const [busca, setBusca] = useState('');
  const q = useAtraso(busca.trim());
  const etiquetas = useQuery({ queryKey: chaves.etiquetas, queryFn: () => cliente.listarEtiquetas() });
  const contatos = useQuery({
    queryKey: [...chaves.contatos(contaId ?? '-'), { q, formulario: true }],
    enabled: Boolean(contaId),
    queryFn: () => cliente.listarContatos(contaId as string, { limite: 100, ...(q ? { busca: q } : {}) }),
  });
  const escolhidos = useMemo(() => new Set(contatoIds), [contatoIds]);
  const importar = useMutation({
    mutationFn: () =>
      cliente.importarContatosComoLeads({
        conta_id: contaId as string,
        ...(contatoIds.length ? { contato_ids: contatoIds } : {}),
        ...(etiquetaIds.length ? { etiqueta_ids: etiquetaIds } : {}),
      }),
    onSuccess: aoImportar,
  });

  if (!contaId) return <FaixaAviso tipo="aviso">Conecte uma conta para importar os contatos dela.</FaixaAviso>;

  const alternarContato = (c: Contato) =>
    setContatoIds((ids) => (ids.includes(c.id) ? ids.filter((x) => x !== c.id) : [...ids, c.id]));

  return (
    <div className="form-importacao">
      <fieldset className="campo">
        <legend>Por etiqueta</legend>
        {(etiquetas.data ?? []).length === 0 ? (
          <p className="texto-secundario">Nenhuma etiqueta ainda.</p>
        ) : (
          <div className="lista-chips">
            {(etiquetas.data ?? []).map((e) => (
              <label key={e.id} className="caixa">
                <input
                  type="checkbox"
                  checked={etiquetaIds.includes(e.id)}
                  onChange={(ev) => setEtiquetaIds((ids) => (ev.target.checked ? [...ids, e.id] : ids.filter((x) => x !== e.id)))}
                />
                <span className="ponto-etiqueta" style={{ background: e.cor }} aria-hidden="true" />
                {e.nome} <small>({numero(e.total_contatos)})</small>
              </label>
            ))}
          </div>
        )}
      </fieldset>
      <fieldset className="campo">
        <legend>Contatos escolhidos ({numero(contatoIds.length)})</legend>
        <input type="search" aria-label="Buscar contato para importar" placeholder="Buscar contato" value={busca} onChange={(e) => setBusca(e.target.value)} />
        <div className="lista-escolha alta">
          {(contatos.data?.itens ?? []).map((c) => (
            <label key={c.id} className="caixa">
              <input type="checkbox" checked={escolhidos.has(c.id)} disabled={!c.telefone} onChange={() => alternarContato(c)} />
              {nomeContato(c)}
            </label>
          ))}
        </div>
      </fieldset>
      {importar.error ? <FaixaAviso tipo="erro">{textoErro(importar.error)}</FaixaAviso> : null}
      <button
        type="button"
        className="botao"
        disabled={(contatoIds.length === 0 && etiquetaIds.length === 0) || importar.isPending}
        onClick={() => importar.mutate()}
      >
        {importar.isPending ? 'Importando…' : 'Importar como leads'}
      </button>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Abas
// ---------------------------------------------------------------------------

export type AbaImportacao = 'arquivo' | 'colar' | 'contatos';

export function AbasImportacao({ aoImportar }: { aoImportar: AoImportar }) {
  const { contaId } = useContaAtual();
  const [aba, setAba] = useState<AbaImportacao>('arquivo');
  const abas: { id: AbaImportacao; rotulo: string; icone: ReactNode }[] = [
    { id: 'arquivo', rotulo: 'Arquivo', icone: <FileSpreadsheet size={16} aria-hidden="true" /> },
    { id: 'colar', rotulo: 'Colar números', icone: <ClipboardPaste size={16} aria-hidden="true" /> },
    { id: 'contatos', rotulo: 'Contatos e etiquetas', icone: <Users size={16} aria-hidden="true" /> },
  ];
  return (
    <div>
      <div className="abas" role="tablist" aria-label="Forma de importar">
        {abas.map((a) => (
          <button
            key={a.id}
            type="button"
            role="tab"
            aria-selected={aba === a.id}
            className={`aba${aba === a.id ? ' ativa' : ''}`}
            onClick={() => setAba(a.id)}
          >
            {a.icone} {a.rotulo}
          </button>
        ))}
      </div>
      <div role="tabpanel" className="painel-aba">
        {aba === 'arquivo' ? <FormArquivo aoImportar={aoImportar} /> : null}
        {aba === 'colar' ? <FormColar aoImportar={aoImportar} /> : null}
        {aba === 'contatos' ? <FormContatos aoImportar={aoImportar} contaId={contaId} /> : null}
      </div>
    </div>
  );
}

export function ImportarLeads() {
  const navegar = useNavigate();
  const qc = useQueryClient();
  const [relatorio, setRelatorio] = useState<Relatorio | null>(null);
  return (
    <section className="tela tela-rolavel">
      <header className="cabecalho-tela">
        <BotaoIcone rotulo="Voltar para Leads" onClick={() => navegar('/leads')}>
          <ArrowLeft size={20} />
        </BotaoIcone>
        <h1>Importar leads</h1>
      </header>
      <div className="cartao">
        {relatorio ? (
          <>
            <h2>Importação concluída</h2>
            <RelatorioImportacao relatorio={relatorio} />
            <div className="acoes-formulario">
              <button type="button" className="botao secundario" onClick={() => setRelatorio(null)}>
                Importar mais
              </button>
              <button type="button" className="botao" onClick={() => navegar('/leads')}>
                Ver leads
              </button>
            </div>
          </>
        ) : (
          <AbasImportacao
            aoImportar={(r) => {
              setRelatorio(r);
              void qc.invalidateQueries({ queryKey: chaves.leads });
            }}
          />
        )}
      </div>
    </section>
  );
}
