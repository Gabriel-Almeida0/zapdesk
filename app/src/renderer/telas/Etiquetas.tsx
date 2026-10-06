// Etiquetas (T121): CRUD com seletor de cor; nome único (1–30), cor #RRGGBB.
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Pencil, Tag, Trash } from 'lucide-react';
import { useState } from 'react';

import type { Etiqueta } from '@zapdesk/cliente-motor';

import { chaves } from '../api/chaves';
import { useCliente } from '../api/motor';
import { BotaoIcone } from '../componentes/BotaoIcone';
import { EsqueletoLista } from '../componentes/Esqueleto';
import { EstadoVazio } from '../componentes/EstadoVazio';
import { FaixaAviso } from '../componentes/FaixaAviso';
import { Confirmar, Modal } from '../componentes/Modal';
import { TelaErro } from '../componentes/TelaErro';
import { numero, textoErro } from '../util/formatar';

// Paleta SUGERIDA para etiquetas novas (research R9): tons da identidade, sem verde, todos ≥ 3:1
// sobre --superficie nos dois temas (npm run contraste). Minúsculas: a comparação usa toLowerCase.
// Cores já salvas pelo usuário não mudam (vêm do motor e são aplicadas inline).
// cores-de-dados
export const CORES_ETIQUETA = [
  '#c48200', // âmbar
  '#2a9bb0', // ciano
  '#e2603c', // coral
  '#737b88', // grafite
  '#a86b3a', // ocre
  '#5b6f9a', // ardósia
  '#b04a6e', // vinho
  '#8a8530', // oliva
] as const;

const COR_PADRAO: string = CORES_ETIQUETA[0];

const COR_VALIDA = /^#[0-9a-fA-F]{6}$/;

export function validarEtiqueta(nome: string, cor: string): string | null {
  const n = nome.trim();
  if (n.length < 1 || n.length > 30) return 'O nome deve ter de 1 a 30 caracteres.';
  if (!COR_VALIDA.test(cor)) return 'Escolha uma cor válida (#RRGGBB).';
  return null;
}

function FormEtiqueta(props: { inicial?: Etiqueta; aoFechar: () => void }) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const [nome, setNome] = useState(props.inicial?.nome ?? '');
  const [cor, setCor] = useState(props.inicial?.cor ?? COR_PADRAO);
  const [tentou, setTentou] = useState(false);
  const erroLocal = validarEtiqueta(nome, cor);
  const salvar = useMutation({
    mutationFn: () =>
      props.inicial
        ? cliente.editarEtiqueta(props.inicial.id, { nome: nome.trim(), cor })
        : cliente.criarEtiqueta({ nome: nome.trim(), cor }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: chaves.etiquetas });
      props.aoFechar();
    },
  });
  return (
    <Modal titulo={props.inicial ? 'Editar etiqueta' : 'Nova etiqueta'} aoFechar={props.aoFechar}>
      <form
        className="formulario"
        onSubmit={(e) => {
          e.preventDefault();
          setTentou(true);
          if (!erroLocal) salvar.mutate();
        }}
      >
        <label className="campo">
          <span>Nome</span>
          <input value={nome} maxLength={30} placeholder="Ex.: Quente" onChange={(e) => setNome(e.target.value)} />
        </label>
        <fieldset className="campo">
          <legend>Cor</legend>
          <div className="paleta" role="radiogroup" aria-label="Cor da etiqueta">
            {CORES_ETIQUETA.map((c) => (
              <button
                key={c}
                type="button"
                role="radio"
                aria-checked={cor.toLowerCase() === c}
                aria-label={`Cor ${c}`}
                className={`amostra-cor${cor.toLowerCase() === c ? ' escolhida' : ''}`}
                style={{ background: c }}
                onClick={() => setCor(c)}
              />
            ))}
            <label className="cor-livre">
              <input type="color" aria-label="Outra cor" value={COR_VALIDA.test(cor) ? cor : COR_PADRAO} onChange={(e) => setCor(e.target.value)} />
            </label>
          </div>
        </fieldset>
        <div className="previa-etiqueta">
          Prévia:{' '}
          <span className="chip-etiqueta">
            <span className="ponto-etiqueta" style={{ background: cor }} aria-hidden="true" />
            {nome.trim() || 'Etiqueta'}
          </span>
        </div>
        {tentou && erroLocal ? <FaixaAviso tipo="erro">{erroLocal}</FaixaAviso> : null}
        {salvar.error ? <FaixaAviso tipo="erro">{textoErro(salvar.error)}</FaixaAviso> : null}
        <div className="acoes-formulario">
          <button type="button" className="botao secundario" onClick={props.aoFechar}>
            Cancelar
          </button>
          <button type="submit" className="botao" disabled={salvar.isPending}>
            Salvar
          </button>
        </div>
      </form>
    </Modal>
  );
}

export function Etiquetas() {
  const cliente = useCliente();
  const qc = useQueryClient();
  const consulta = useQuery({ queryKey: chaves.etiquetas, queryFn: () => cliente.listarEtiquetas() });
  const [editando, setEditando] = useState<Etiqueta | 'nova' | null>(null);
  const [excluindo, setExcluindo] = useState<Etiqueta | null>(null);
  const excluir = useMutation({
    mutationFn: (id: string) => cliente.excluirEtiqueta(id),
    onSuccess: () => {
      setExcluindo(null);
      void qc.invalidateQueries({ queryKey: chaves.etiquetas });
    },
  });

  return (
    <section className="tela tela-rolavel">
      <header className="cabecalho-tela">
        <h1>Etiquetas</h1>
        <button type="button" className="botao" onClick={() => setEditando('nova')}>
          Nova etiqueta
        </button>
      </header>
      {excluir.error ? <FaixaAviso tipo="erro">{textoErro(excluir.error)}</FaixaAviso> : null}
      {consulta.isPending ? (
        <EsqueletoLista linhas={4} />
      ) : consulta.isError ? (
        <TelaErro erro={consulta.error} aoTentar={() => void consulta.refetch()} />
      ) : consulta.data.length === 0 ? (
        <EstadoVazio
          icone={<Tag size={56} />}
          titulo="Nenhuma etiqueta"
          texto="Use etiquetas para organizar contatos (ex.: Quente, Cliente) e filtrar conversas."
          acao={
            <button type="button" className="botao" onClick={() => setEditando('nova')}>
              Nova etiqueta
            </button>
          }
        />
      ) : (
        <ul className="lista-cartoes">
          {consulta.data.map((e) => (
            <li key={e.id} className="linha-etiqueta">
              <span className="ponto-etiqueta grande" style={{ background: e.cor }} aria-hidden="true" />
              <span className="linha-etiqueta-textos">
                <strong>{e.nome}</strong>
                <small>{e.total_contatos === 1 ? '1 contato' : `${numero(e.total_contatos)} contatos`}</small>
              </span>
              <BotaoIcone rotulo={`Editar etiqueta ${e.nome}`} onClick={() => setEditando(e)}>
                <Pencil size={16} />
              </BotaoIcone>
              <BotaoIcone rotulo={`Excluir etiqueta ${e.nome}`} onClick={() => setExcluindo(e)}>
                <Trash size={16} />
              </BotaoIcone>
            </li>
          ))}
        </ul>
      )}
      {editando ? <FormEtiqueta inicial={editando === 'nova' ? undefined : editando} aoFechar={() => setEditando(null)} /> : null}
      {excluindo ? (
        <Confirmar
          titulo={`Excluir "${excluindo.nome}"?`}
          texto="A etiqueta sai de todos os contatos. As conversas não são apagadas."
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
