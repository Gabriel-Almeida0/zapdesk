// Passo 1 — Lista (T095): conta; leads existentes (busca/seleção), importar arquivo, colar números,
// contatos/etiquetas; mostra válidos, novos e duplicados com o RelatorioImportacao.
import { useQuery } from '@tanstack/react-query';
import { Search, Trash } from 'lucide-react';
import { useMemo, useState } from 'react';

import type { RelatorioImportacao as Relatorio } from '@zapdesk/cliente-motor';

import { chaves } from '../../api/chaves';
import { useCliente } from '../../api/motor';
import { BotaoIcone } from '../../componentes/BotaoIcone';
import { FaixaAviso } from '../../componentes/FaixaAviso';
import { RelatorioImportacao } from '../../componentes/RelatorioImportacao';
import { ROTULO_ESTADO_CONTA, useContaAtual } from '../../estado/conta';
import type { FormDisparo } from '../../util/disparos';
import { numero, telefone } from '../../util/formatar';
import { useAtraso } from '../Conversas';
import { AbasImportacao } from '../ImportarLeads';

export interface FonteLista {
  id: string;
  rotulo: string;
  leadIds: string[];
  relatorio: Relatorio | null;
}

function uniao(fontes: FonteLista[]): string[] {
  const vistos = new Set<string>();
  const ids: string[] = [];
  for (const f of fontes) for (const id of f.leadIds) if (!vistos.has(id)) (vistos.add(id), ids.push(id));
  return ids;
}

export function PassoLista(props: {
  form: FormDisparo;
  fontes: FonteLista[];
  aoMudarForm: (parcial: Partial<FormDisparo>) => void;
  aoMudarFontes: (fontes: FonteLista[]) => void;
}) {
  const cliente = useCliente();
  const { contas } = useContaAtual();
  const [modo, setModo] = useState<'existentes' | 'importar'>('existentes');
  const [busca, setBusca] = useState('');
  const q = useAtraso(busca.trim());
  const [selecionados, setSelecionados] = useState<Set<string>>(
    () => new Set(props.fontes.find((f) => f.id === 'existentes')?.leadIds ?? []),
  );

  const leads = useQuery({
    queryKey: [...chaves.leads, { q, seletor: true }],
    queryFn: () => cliente.listarLeads({ limite: 200, ...(q ? { busca: q } : {}) }),
  });

  const definirFontes = (fontes: FonteLista[]) => {
    props.aoMudarFontes(fontes);
    props.aoMudarForm({ leadIds: uniao(fontes) });
  };

  const atualizarExistentes = (novos: Set<string>) => {
    setSelecionados(novos);
    const outras = props.fontes.filter((f) => f.id !== 'existentes');
    const fonte: FonteLista = { id: 'existentes', rotulo: 'Leads escolhidos da base', leadIds: [...novos], relatorio: null };
    definirFontes(novos.size > 0 ? [fonte, ...outras] : outras);
  };

  const itens = leads.data?.itens ?? [];
  const todosVisiveis = itens.length > 0 && itens.every((l) => selecionados.has(l.id));
  const total = props.form.leadIds.length;
  const contaEscolhida = contas.find((c) => c.id === props.form.contaId);
  const importadas = useMemo(() => props.fontes.filter((f) => f.relatorio), [props.fontes]);

  return (
    <div className="passo">
      <label className="campo">
        <span>Conta que vai enviar</span>
        <select value={props.form.contaId} onChange={(e) => props.aoMudarForm({ contaId: e.target.value })}>
          {contas.map((c) => (
            <option key={c.id} value={c.id}>
              {c.nome}
              {c.telefone ? ` (${telefone(c.telefone)})` : ''} — {ROTULO_ESTADO_CONTA[c.estado]}
            </option>
          ))}
        </select>
      </label>
      {contaEscolhida && contaEscolhida.estado !== 'conectada' ? (
        <FaixaAviso tipo="aviso">Esta conta não está conectada. O disparo só começa quando ela estiver conectada.</FaixaAviso>
      ) : null}

      <div className="resumo-lista" aria-live="polite">
        <strong>{numero(total)}</strong> {total === 1 ? 'destinatário' : 'destinatários'} na lista (sem repetidos)
      </div>

      <div className="abas" role="tablist" aria-label="Como montar a lista">
        <button type="button" role="tab" aria-selected={modo === 'existentes'} className={`aba${modo === 'existentes' ? ' ativa' : ''}`} onClick={() => setModo('existentes')}>
          Leads da base
        </button>
        <button type="button" role="tab" aria-selected={modo === 'importar'} className={`aba${modo === 'importar' ? ' ativa' : ''}`} onClick={() => setModo('importar')}>
          Importar novos
        </button>
      </div>

      {modo === 'existentes' ? (
        <div className="painel-aba">
          <div className="barra-ferramentas">
            <label className="campo-busca">
              <Search size={16} aria-hidden="true" />
              <input type="search" aria-label="Buscar lead" placeholder="Buscar por telefone ou nome" value={busca} onChange={(e) => setBusca(e.target.value)} />
            </label>
            <label className="caixa">
              <input
                type="checkbox"
                checked={todosVisiveis}
                disabled={itens.length === 0}
                onChange={(e) => {
                  const novos = new Set(selecionados);
                  for (const l of itens) {
                    if (e.target.checked) novos.add(l.id);
                    else novos.delete(l.id);
                  }
                  atualizarExistentes(novos);
                }}
              />
              Selecionar os {numero(itens.length)} exibidos
            </label>
          </div>
          <div className="lista-escolha alta" role="group" aria-label="Leads">
            {leads.isPending ? (
              <p role="status">Carregando leads…</p>
            ) : itens.length === 0 ? (
              <p className="texto-secundario">Nenhum lead encontrado. Use "Importar novos".</p>
            ) : (
              itens.map((l) => (
                <label key={l.id} className="caixa">
                  <input
                    type="checkbox"
                    checked={selecionados.has(l.id)}
                    onChange={() => {
                      const novos = new Set(selecionados);
                      if (novos.has(l.id)) novos.delete(l.id);
                      else novos.add(l.id);
                      atualizarExistentes(novos);
                    }}
                  />
                  <span className="mono">{telefone(l.telefone)}</span> {l.nome ?? ''}
                </label>
              ))
            )}
          </div>
        </div>
      ) : (
        <div className="painel-aba">
          <AbasImportacao
            aoImportar={(relatorio) => {
              const fonte: FonteLista = {
                id: `importacao-${Date.now()}`,
                rotulo: `Importação ${importadas.length + 1}`,
                leadIds: relatorio.lead_ids,
                relatorio,
              };
              definirFontes([...props.fontes, fonte]);
            }}
          />
        </div>
      )}

      {importadas.length > 0 ? (
        <div className="fontes">
          <h3>Importações adicionadas</h3>
          {importadas.map((f) => (
            <div key={f.id} className="fonte">
              <div className="fonte-cabecalho">
                <strong>{f.rotulo}</strong>
                <span>{numero(f.leadIds.length)} válidos</span>
                <BotaoIcone rotulo={`Tirar ${f.rotulo} da lista`} onClick={() => definirFontes(props.fontes.filter((x) => x.id !== f.id))}>
                  <Trash size={16} />
                </BotaoIcone>
              </div>
              {f.relatorio ? <RelatorioImportacao relatorio={f.relatorio} /> : null}
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}
