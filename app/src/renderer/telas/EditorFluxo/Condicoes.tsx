// Condições (T080): "todas" (E) ou "alguma" (OU) com as regras de contracts/formatos.md.
// Reutilizado pelos ramos do nó "condição" do chatbot.
import { Plus, Trash } from 'lucide-react';

import type { Condicoes as TipoCondicoes, DiaSemana, ErroDefinicao, OperadorCampo, Regra } from '@zapdesk/cliente-motor';

import { BotaoIcone } from '../../componentes/BotaoIcone';
import { useContaAtual } from '../../estado/conta';
import { errosEm, regraPadrao, ROTULO_REGRA } from '../../util/automacoes';
import { Campo, ErrosCampo, SeletorEtiqueta, SeletorFunilEtapa } from './campos';

const OPERADORES_CAMPO: { id: OperadorCampo; rotulo: string }[] = [
  { id: 'igual', rotulo: 'é igual a' },
  { id: 'diferente', rotulo: 'é diferente de' },
  { id: 'contem', rotulo: 'contém' },
  { id: 'existe', rotulo: 'está preenchido' },
  { id: 'nao_existe', rotulo: 'está vazio' },
  { id: 'maior', rotulo: 'é maior que' },
  { id: 'menor', rotulo: 'é menor que' },
];

const DIAS: { id: DiaSemana; rotulo: string }[] = [
  { id: 1, rotulo: 'Seg' },
  { id: 2, rotulo: 'Ter' },
  { id: 3, rotulo: 'Qua' },
  { id: 4, rotulo: 'Qui' },
  { id: 5, rotulo: 'Sex' },
  { id: 6, rotulo: 'Sáb' },
  { id: 7, rotulo: 'Dom' },
];

const TIPOS_REGRA = Object.keys(ROTULO_REGRA) as Regra['tipo'][];

function semValor(op: string): boolean {
  return op === 'existe' || op === 'nao_existe';
}

export function FormRegra({ regra, aoMudar }: { regra: Regra; aoMudar: (r: Regra) => void }) {
  const { contas } = useContaAtual();
  switch (regra.tipo) {
    case 'etiqueta':
      return (
        <div className="grade-campos">
          <Campo rotulo="O contato">
            <select value={regra.operador} onChange={(e) => aoMudar({ ...regra, operador: e.target.value as 'tem' | 'nao_tem' })}>
              <option value="tem">tem a etiqueta</option>
              <option value="nao_tem">não tem a etiqueta</option>
            </select>
          </Campo>
          <SeletorEtiqueta valor={regra.etiqueta_id} aoMudar={(id) => aoMudar({ ...regra, etiqueta_id: id })} />
        </div>
      );
    case 'etapa':
      return (
        <>
          <Campo rotulo="O lead">
            <select value={regra.operador} onChange={(e) => aoMudar({ ...regra, operador: e.target.value as 'esta' | 'nao_esta' })}>
              <option value="esta">está em</option>
              <option value="nao_esta">não está em</option>
            </select>
          </Campo>
          <SeletorFunilEtapa
            qualquerEtapa
            funilId={regra.funil_id}
            etapaId={regra.etapa_id}
            aoMudar={(funil_id, etapa_id) => aoMudar({ ...regra, funil_id, etapa_id })}
          />
        </>
      );
    case 'campo_lead':
    case 'variavel': {
      const nome = regra.tipo === 'campo_lead' ? regra.campo : regra.variavel;
      const operadores = regra.tipo === 'variavel' ? [...OPERADORES_CAMPO, { id: 'regex' as const, rotulo: 'casa com a regex' }] : OPERADORES_CAMPO;
      return (
        <div className="grade-campos">
          <Campo rotulo={regra.tipo === 'campo_lead' ? 'Campo do lead' : 'Variável'}>
            <input
              value={nome}
              placeholder={regra.tipo === 'campo_lead' ? 'empresa' : 'email'}
              onChange={(e) =>
                aoMudar(regra.tipo === 'campo_lead' ? { ...regra, campo: e.target.value } : { ...regra, variavel: e.target.value })
              }
            />
          </Campo>
          <Campo rotulo="Operador">
            <select
              value={regra.operador}
              onChange={(e) => aoMudar({ ...regra, operador: e.target.value as OperadorCampo } as Regra)}
            >
              {operadores.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.rotulo}
                </option>
              ))}
            </select>
          </Campo>
          {semValor(regra.operador) ? null : (
            <Campo rotulo="Valor">
              <input value={regra.valor ?? ''} onChange={(e) => aoMudar({ ...regra, valor: e.target.value })} />
            </Campo>
          )}
        </div>
      );
    }
    case 'horario':
      return (
        <>
          <div className="grade-campos">
            <Campo rotulo="Das">
              <input type="time" value={regra.inicio} onChange={(e) => aoMudar({ ...regra, inicio: e.target.value })} />
            </Campo>
            <Campo rotulo="Até" dica="Se o fim for antes do início, atravessa a meia-noite.">
              <input type="time" value={regra.fim} onChange={(e) => aoMudar({ ...regra, fim: e.target.value })} />
            </Campo>
          </div>
          <fieldset className="campo">
            <legend>Dias (nenhum marcado = todos)</legend>
            <div className="opcoes-radio">
              {DIAS.map((d) => {
                const dias = regra.dias ?? [];
                return (
                  <label key={d.id} className="caixa">
                    <input
                      type="checkbox"
                      checked={dias.includes(d.id)}
                      onChange={(e) =>
                        aoMudar({ ...regra, dias: e.target.checked ? [...dias, d.id].sort() : dias.filter((x) => x !== d.id) })
                      }
                    />
                    {d.rotulo}
                  </label>
                );
              })}
            </div>
          </fieldset>
        </>
      );
    case 'texto':
      return (
        <div className="grade-campos">
          <Campo rotulo="A mensagem">
            <select value={regra.operador} onChange={(e) => aoMudar({ ...regra, operador: e.target.value as 'contem' | 'igual' | 'regex' })}>
              <option value="contem">contém</option>
              <option value="igual">é igual a</option>
              <option value="regex">casa com a regex</option>
            </select>
          </Campo>
          <Campo rotulo="Texto">
            <input value={regra.valor} onChange={(e) => aoMudar({ ...regra, valor: e.target.value })} />
          </Campo>
        </div>
      );
    case 'conta':
      return (
        <fieldset className="campo">
          <legend>Só nas contas</legend>
          <div className="opcoes-radio">
            {contas.map((c) => (
              <label key={c.id} className="caixa">
                <input
                  type="checkbox"
                  checked={regra.conta_ids.includes(c.id)}
                  onChange={(e) =>
                    aoMudar({ ...regra, conta_ids: e.target.checked ? [...regra.conta_ids, c.id] : regra.conta_ids.filter((x) => x !== c.id) })
                  }
                />
                {c.nome}
              </label>
            ))}
          </div>
        </fieldset>
      );
  }
}

export function EditorCondicoes(props: {
  condicoes: TipoCondicoes;
  aoMudar: (c: TipoCondicoes) => void;
  erros: readonly ErroDefinicao[];
  prefixo: string;
  /** Oferece a regra "variável" (chatbot). */
  comVariavel?: boolean;
}) {
  const { condicoes } = props;
  const tipos = props.comVariavel ? TIPOS_REGRA : TIPOS_REGRA.filter((t) => t !== 'variavel');
  const mudarRegra = (i: number, r: Regra) => props.aoMudar({ ...condicoes, regras: condicoes.regras.map((x, j) => (j === i ? r : x)) });
  return (
    <div className="editor-condicoes">
      {condicoes.regras.length > 1 ? (
        <Campo rotulo="Combinar">
          <select value={condicoes.modo} onChange={(e) => props.aoMudar({ ...condicoes, modo: e.target.value as 'todas' | 'alguma' })}>
            <option value="todas">Todas as regras (E)</option>
            <option value="alguma">Alguma regra (OU)</option>
          </select>
        </Campo>
      ) : null}
      {condicoes.regras.map((r, i) => (
        <div key={i} className="bloco-editor">
          <div className="bloco-editor-cabecalho">
            <select aria-label={`Regra ${i + 1}`} value={r.tipo} onChange={(e) => mudarRegra(i, regraPadrao(e.target.value as Regra['tipo']))}>
              {tipos.map((t) => (
                <option key={t} value={t}>
                  {ROTULO_REGRA[t]}
                </option>
              ))}
            </select>
            <BotaoIcone
              rotulo={`Remover regra ${i + 1}`}
              className="pequeno"
              onClick={() => props.aoMudar({ ...condicoes, regras: condicoes.regras.filter((_, j) => j !== i) })}
            >
              <Trash size={14} />
            </BotaoIcone>
          </div>
          <FormRegra regra={r} aoMudar={(novo) => mudarRegra(i, novo)} />
          <ErrosCampo erros={errosEm(props.erros, `${props.prefixo}.regras[${i}]`)} />
        </div>
      ))}
      <button
        type="button"
        className="botao-link"
        disabled={condicoes.regras.length >= 20}
        onClick={() => props.aoMudar({ ...condicoes, regras: [...condicoes.regras, regraPadrao('etiqueta')] })}
      >
        <Plus size={14} aria-hidden="true" /> Adicionar regra
      </button>
    </div>
  );
}

export function Condicoes(props: { condicoes: TipoCondicoes | null; aoMudar: (c: TipoCondicoes | null) => void; erros: readonly ErroDefinicao[] }) {
  const atuais = props.condicoes ?? { modo: 'todas' as const, regras: [] };
  return (
    <section className="secao-editor" aria-labelledby="titulo-condicoes">
      <h2 id="titulo-condicoes">2. Só se (opcional)</h2>
      {atuais.regras.length === 0 ? <p className="texto-secundario">Sem condições: executa sempre que o gatilho acontecer.</p> : null}
      <EditorCondicoes
        condicoes={atuais}
        prefixo="definicao.condicoes"
        erros={props.erros}
        aoMudar={(c) => props.aoMudar(c.regras.length === 0 ? null : c)}
      />
    </section>
  );
}
