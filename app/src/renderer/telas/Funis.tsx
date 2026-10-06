// Funis (T067): lista de funis — criar com etapas iniciais, renomear, reordenar e excluir com
// confirmação. Clique abre o Kanban.
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { ArrowDown, ArrowUp, Pencil, Plus, SquareKanban, Trash, X } from 'lucide-react';
import { useState } from 'react';
import { Link } from 'react-router';

import type { Funil } from '@zapdesk/cliente-motor';

import { useFunis } from '../api/automacoes';
import { chaves } from '../api/chaves';
import { useCliente } from '../api/motor';
import { BotaoIcone } from '../componentes/BotaoIcone';
import { EsqueletoLista } from '../componentes/Esqueleto';
import { EstadoVazio } from '../componentes/EstadoVazio';
import { FaixaAviso } from '../componentes/FaixaAviso';
import { Confirmar, Modal } from '../componentes/Modal';
import { TelaErro } from '../componentes/TelaErro';
import { numero, plural, textoErro } from '../util/formatar';

// Paleta SUGERIDA para etapas novas (research R9): tons da identidade, sem verde, os mesmos valores
// de CORES_ETIQUETA (todos ≥ 3:1 sobre --superficie nos dois temas; npm run contraste). Começa no
// ciano para o funil sugerido ficar como no KanbanMockup (Novo ciano, Qualificando âmbar, Proposta
// coral). Cores já salvas pelo usuário não mudam (vêm do motor e são aplicadas inline).
// cores-de-dados
export const CORES_ETAPA = [
  '#2a9bb0', // ciano
  '#c48200', // âmbar
  '#e2603c', // coral
  '#737b88', // grafite
  '#a86b3a', // ocre
  '#5b6f9a', // ardósia
  '#b04a6e', // vinho
  '#8a8530', // oliva
] as const;
export const COR_ETAPA_PADRAO: string = CORES_ETAPA[0];
export const ETAPAS_SUGERIDAS = ['Novo', 'Qualificando', 'Proposta', 'Fechado'];

export function validarNomeFunil(nome: string): string | null {
  const n = nome.trim();
  if (n.length < 1 || n.length > 60) return 'O nome do funil deve ter de 1 a 60 caracteres.';
  return null;
}

export function validarEtapas(nomes: string[]): string | null {
  const limpos = nomes.map((n) => n.trim());
  if (limpos.some((n) => n.length < 1 || n.length > 40)) return 'Cada etapa deve ter de 1 a 40 caracteres.';
  if (limpos.length > 30) return 'Um funil pode ter no máximo 30 etapas.';
  const vistos = new Set<string>();
  for (const n of limpos) {
    const chave = n.toLocaleLowerCase('pt-BR');
    if (vistos.has(chave)) return `A etapa "${n}" aparece duas vezes.`;
    vistos.add(chave);
  }
  return null;
}

function NovoFunil({ aoFechar }: { aoFechar: () => void }) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const [nome, setNome] = useState('');
  const [etapas, setEtapas] = useState<string[]>(ETAPAS_SUGERIDAS);
  const [tentou, setTentou] = useState(false);
  const erroLocal = validarNomeFunil(nome) ?? validarEtapas(etapas);
  const criar = useMutation({
    mutationFn: () =>
      cliente.criarFunil({
        nome: nome.trim(),
        etapas: etapas.map((e, i) => ({ nome: e.trim(), cor: CORES_ETAPA[i % CORES_ETAPA.length] ?? COR_ETAPA_PADRAO })),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: chaves.funis });
      aoFechar();
    },
  });
  return (
    <Modal titulo="Novo funil" aoFechar={aoFechar} largura={480}>
      <form
        className="formulario"
        onSubmit={(e) => {
          e.preventDefault();
          setTentou(true);
          if (!erroLocal) criar.mutate();
        }}
      >
        <label className="campo">
          <span>Nome</span>
          <input value={nome} maxLength={60} placeholder="Ex.: Prospecção" onChange={(e) => setNome(e.target.value)} />
        </label>
        <fieldset className="campo">
          <legend>Etapas (na ordem do quadro)</legend>
          <ol className="lista-etapas-nova">
            {etapas.map((etapa, i) => (
              <li key={i}>
                <span className="ponto-etiqueta grande" style={{ background: CORES_ETAPA[i % CORES_ETAPA.length] }} aria-hidden="true" />
                <input
                  aria-label={`Etapa ${i + 1}`}
                  value={etapa}
                  maxLength={40}
                  onChange={(e) => setEtapas((l) => l.map((x, j) => (j === i ? e.target.value : x)))}
                />
                <BotaoIcone
                  rotulo={`Remover etapa ${i + 1}`}
                  className="pequeno"
                  disabled={etapas.length <= 1}
                  onClick={() => setEtapas((l) => l.filter((_, j) => j !== i))}
                >
                  <X size={14} />
                </BotaoIcone>
              </li>
            ))}
          </ol>
          <button type="button" className="botao-link" disabled={etapas.length >= 30} onClick={() => setEtapas((l) => [...l, ''])}>
            <Plus size={14} aria-hidden="true" /> Adicionar etapa
          </button>
        </fieldset>
        {tentou && erroLocal ? <FaixaAviso tipo="erro">{erroLocal}</FaixaAviso> : null}
        {criar.error ? <FaixaAviso tipo="erro">{textoErro(criar.error)}</FaixaAviso> : null}
        <div className="acoes-formulario">
          <button type="button" className="botao secundario" onClick={aoFechar}>
            Cancelar
          </button>
          <button type="submit" className="botao" disabled={criar.isPending}>
            Criar funil
          </button>
        </div>
      </form>
    </Modal>
  );
}

function RenomearFunil({ funil, aoFechar }: { funil: Funil; aoFechar: () => void }) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const [nome, setNome] = useState(funil.nome);
  const erroLocal = validarNomeFunil(nome);
  const salvar = useMutation({
    mutationFn: () => cliente.editarFunil(funil.id, { nome: nome.trim() }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: chaves.funis });
      void qc.invalidateQueries({ queryKey: chaves.funil(funil.id) });
      aoFechar();
    },
  });
  return (
    <Modal titulo="Renomear funil" aoFechar={aoFechar}>
      <form
        className="formulario"
        onSubmit={(e) => {
          e.preventDefault();
          if (!erroLocal) salvar.mutate();
        }}
      >
        <label className="campo">
          <span>Nome</span>
          <input value={nome} maxLength={60} onChange={(e) => setNome(e.target.value)} />
        </label>
        {erroLocal ? <p className="erro-campo">{erroLocal}</p> : null}
        {salvar.error ? <FaixaAviso tipo="erro">{textoErro(salvar.error)}</FaixaAviso> : null}
        <div className="acoes-formulario">
          <button type="button" className="botao secundario" onClick={aoFechar}>
            Cancelar
          </button>
          <button type="submit" className="botao" disabled={Boolean(erroLocal) || salvar.isPending}>
            Salvar
          </button>
        </div>
      </form>
    </Modal>
  );
}

export function Funis() {
  const cliente = useCliente();
  const qc = useQueryClient();
  const consulta = useFunis();
  const [criando, setCriando] = useState(false);
  const [renomeando, setRenomeando] = useState<Funil | null>(null);
  const [excluindo, setExcluindo] = useState<Funil | null>(null);

  const ordenar = useMutation({
    mutationFn: (d: { funil: Funil; ordem: number }) => cliente.editarFunil(d.funil.id, { ordem: d.ordem }),
    onSettled: () => void qc.invalidateQueries({ queryKey: chaves.funis }),
  });
  const excluir = useMutation({
    mutationFn: (id: string) => cliente.excluirFunil(id),
    onSuccess: () => {
      setExcluindo(null);
      void qc.invalidateQueries({ queryKey: chaves.funis });
    },
  });
  const funis = [...(consulta.data ?? [])].sort((a, b) => a.ordem - b.ordem);
  const erro = ordenar.error ?? excluir.error;

  return (
    <section className="tela tela-rolavel">
      <header className="cabecalho-tela">
        <h1>Funis</h1>
        <button type="button" className="botao" onClick={() => setCriando(true)}>
          <Plus size={16} aria-hidden="true" /> Novo funil
        </button>
      </header>
      {erro ? <FaixaAviso tipo="erro">{textoErro(erro)}</FaixaAviso> : null}
      {consulta.isPending ? (
        <EsqueletoLista linhas={3} />
      ) : consulta.isError ? (
        <TelaErro erro={consulta.error} aoTentar={() => void consulta.refetch()} />
      ) : funis.length === 0 ? (
        <EstadoVazio
          icone={<SquareKanban size={56} />}
          titulo="Nenhum funil ainda"
          texto="Crie um funil com etapas (ex.: Novo, Qualificando, Proposta, Fechado) e acompanhe cada lead num quadro Kanban."
          acao={
            <button type="button" className="botao" onClick={() => setCriando(true)}>
              Criar funil
            </button>
          }
        />
      ) : (
        <ul className="lista-funis">
          {funis.map((f, i) => (
            <li key={f.id} className="cartao-funil">
              <Link to={`/funis/${f.id}`} className="cartao-funil-link">
                <strong>{f.nome}</strong>
                <small>
                  {plural(f.etapas.length, 'etapa', 'etapas')} · {f.total_cards === 1 ? '1 lead' : `${numero(f.total_cards)} leads`}
                </small>
                <span className="etapas-resumo" aria-hidden="true">
                  {[...f.etapas]
                    .sort((a, b) => a.ordem - b.ordem)
                    .map((e) => (
                      <span key={e.id} className="etapa-resumo" style={{ borderLeftColor: e.cor }}>
                        {e.nome} <b>{numero(e.total_cards)}</b>
                      </span>
                    ))}
                </span>
              </Link>
              <div className="cartao-funil-acoes">
                <BotaoIcone
                  rotulo={`Subir ${f.nome}`}
                  disabled={i === 0 || ordenar.isPending}
                  onClick={() => ordenar.mutate({ funil: f, ordem: (funis[i - 1]?.ordem ?? i) })}
                >
                  <ArrowUp size={16} />
                </BotaoIcone>
                <BotaoIcone
                  rotulo={`Descer ${f.nome}`}
                  disabled={i === funis.length - 1 || ordenar.isPending}
                  onClick={() => ordenar.mutate({ funil: f, ordem: (funis[i + 1]?.ordem ?? i) })}
                >
                  <ArrowDown size={16} />
                </BotaoIcone>
                <BotaoIcone rotulo={`Renomear ${f.nome}`} onClick={() => setRenomeando(f)}>
                  <Pencil size={16} />
                </BotaoIcone>
                <BotaoIcone rotulo={`Excluir ${f.nome}`} onClick={() => setExcluindo(f)}>
                  <Trash size={16} />
                </BotaoIcone>
              </div>
            </li>
          ))}
        </ul>
      )}
      {criando ? <NovoFunil aoFechar={() => setCriando(false)} /> : null}
      {renomeando ? <RenomearFunil funil={renomeando} aoFechar={() => setRenomeando(null)} /> : null}
      {excluindo ? (
        <Confirmar
          titulo={`Excluir "${excluindo.nome}"?`}
          texto={`Os ${numero(excluindo.total_cards)} leads saem deste funil e o histórico dele é apagado. Os leads continuam na base.`}
          confirmar="Excluir funil"
          perigo
          ocupado={excluir.isPending}
          aoConfirmar={() => excluir.mutate(excluindo.id)}
          aoFechar={() => setExcluindo(null)}
        />
      ) : null}
    </section>
  );
}
