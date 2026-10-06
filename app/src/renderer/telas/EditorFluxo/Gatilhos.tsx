// Gatilhos (T080): todos os tipos de contracts/formatos.md com os filtros de cada um.
import { Plus, Trash } from 'lucide-react';

import {
  TIPOS_GATILHO,
  type ErroDefinicao,
  type Gatilho,
  type OrigemLead,
  type OrigemMensagemSemResposta,
  type TipoConversaGatilho,
  type TipoGatilho,
} from '@zapdesk/cliente-motor';

import { BotaoIcone } from '../../componentes/BotaoIcone';
import { errosEm, gatilhoPadrao, ROTULO_GATILHO, SEGUNDOS_UNIDADE } from '../../util/automacoes';
import { Campo, CampoDuracao, ErrosCampo, SeletorDisparo, SeletorEtiqueta, SeletorFunilEtapa } from './campos';

const ORIGENS_LEAD: { id: OrigemLead; rotulo: string }[] = [
  { id: 'csv', rotulo: 'Planilha' },
  { id: 'colado', rotulo: 'Números colados' },
  { id: 'contatos', rotulo: 'Contatos' },
  { id: 'mcp', rotulo: 'Claude (MCP)' },
];

const ORIGENS_SEM_RESPOSTA: { id: OrigemMensagemSemResposta; rotulo: string }[] = [
  { id: 'qualquer', rotulo: 'Qualquer mensagem minha' },
  { id: 'disparo', rotulo: 'Mensagem de disparo' },
  { id: 'automacao', rotulo: 'Mensagem automática' },
  { id: 'manual', rotulo: 'Mensagem manual' },
];

/** "orçamento, preço" → ["orçamento", "preço"]. */
export function separarPalavras(texto: string): string[] {
  return texto
    .split(/[,\n]/)
    .map((p) => p.trim())
    .filter(Boolean);
}

export function FormGatilho({ gatilho, aoMudar, incluirGrupos }: { gatilho: Gatilho; aoMudar: (g: Gatilho) => void; incluirGrupos: boolean }) {
  switch (gatilho.tipo) {
    case 'mensagem_recebida':
      return (
        <>
          <div className="grade-campos">
            <Campo rotulo="Texto contém (opcional)" dica="Sem diferenciar maiúsculas e acentos.">
              <input value={gatilho.contem ?? ''} onChange={(e) => aoMudar({ ...gatilho, contem: e.target.value || null })} />
            </Campo>
            <Campo rotulo="Expressão regular (opcional)" dica="Sintaxe RE2; use (?i) para ignorar maiúsculas.">
              <input className="campo-codigo" value={gatilho.regex ?? ''} maxLength={500} onChange={(e) => aoMudar({ ...gatilho, regex: e.target.value || null })} />
            </Campo>
          </div>
          <div className="opcoes-radio">
            <label className="caixa">
              <input
                type="checkbox"
                checked={gatilho.primeira_mensagem ?? false}
                onChange={(e) => aoMudar({ ...gatilho, primeira_mensagem: e.target.checked })}
              />
              Só a primeira mensagem do contato
            </label>
            <label className="caixa">
              Conversas:
              <select
                value={gatilho.tipo_conversa ?? 'individual'}
                onChange={(e) => aoMudar({ ...gatilho, tipo_conversa: e.target.value as TipoConversaGatilho })}
              >
                <option value="individual">Individuais</option>
                <option value="grupo">Grupos</option>
                <option value="qualquer">Todas</option>
              </select>
            </label>
          </div>
          {gatilho.tipo_conversa && gatilho.tipo_conversa !== 'individual' && !incluirGrupos ? (
            <p className="texto-secundario">Grupos só contam com "Incluir grupos" ligado nas opções.</p>
          ) : null}
        </>
      );
    case 'palavra_chave':
      return (
        <>
          <Campo rotulo="Palavras-chave" dica="Separe por vírgula. Sem diferenciar maiúsculas e acentos.">
            <input
              defaultValue={gatilho.palavras.join(', ')}
              placeholder="orçamento, preço"
              onBlur={(e) => aoMudar({ ...gatilho, palavras: separarPalavras(e.target.value) })}
              onChange={(e) => aoMudar({ ...gatilho, palavras: separarPalavras(e.target.value) })}
            />
          </Campo>
          <div className="opcoes-radio" role="radiogroup" aria-label="Como comparar">
            <label className="caixa">
              <input type="radio" checked={(gatilho.modo ?? 'palavra') === 'palavra'} onChange={() => aoMudar({ ...gatilho, modo: 'palavra' })} />
              Palavra em qualquer parte da mensagem
            </label>
            <label className="caixa">
              <input type="radio" checked={gatilho.modo === 'mensagem_inteira'} onChange={() => aoMudar({ ...gatilho, modo: 'mensagem_inteira' })} />
              Mensagem inteira
            </label>
          </div>
        </>
      );
    case 'lead_importado':
      return (
        <fieldset className="campo">
          <legend>Origem (nenhuma marcada = todas)</legend>
          <div className="opcoes-radio">
            {ORIGENS_LEAD.map((o) => {
              const atuais = gatilho.origens ?? [];
              return (
                <label key={o.id} className="caixa">
                  <input
                    type="checkbox"
                    checked={atuais.includes(o.id)}
                    onChange={(e) => {
                      const novas = e.target.checked ? [...atuais, o.id] : atuais.filter((x) => x !== o.id);
                      aoMudar({ ...gatilho, origens: novas.length > 0 ? novas : null });
                    }}
                  />
                  {o.rotulo}
                </label>
              );
            })}
          </div>
        </fieldset>
      );
    case 'etiqueta':
      return (
        <div className="grade-campos">
          <Campo rotulo="Quando">
            <select value={gatilho.evento} onChange={(e) => aoMudar({ ...gatilho, evento: e.target.value as 'adicionada' | 'removida' })}>
              <option value="adicionada">For adicionada</option>
              <option value="removida">For removida</option>
            </select>
          </Campo>
          <SeletorEtiqueta valor={gatilho.etiqueta_id} aoMudar={(id) => aoMudar({ ...gatilho, etiqueta_id: id })} />
        </div>
      );
    case 'entrou_etapa':
      return (
        <SeletorFunilEtapa
          funilId={gatilho.funil_id}
          etapaId={gatilho.etapa_id}
          aoMudar={(funil_id, etapa_id) => aoMudar({ ...gatilho, funil_id, etapa_id: etapa_id ?? '' })}
        />
      );
    case 'disparo_respondeu':
      return <SeletorDisparo qualquer valor={gatilho.disparo_id ?? null} aoMudar={(id) => aoMudar({ ...gatilho, disparo_id: id })} />;
    case 'sem_resposta':
      return (
        <div className="grade-campos">
          <CampoDuracao
            rotulo="Sem resposta há"
            segundos={gatilho.apos_s}
            aoMudar={(s) => aoMudar({ ...gatilho, apos_s: s })}
            dica="De 1 minuto a 30 dias depois da última mensagem enviada."
          />
          <Campo rotulo="Depois de">
            <select
              value={gatilho.origem_mensagem ?? 'qualquer'}
              onChange={(e) => aoMudar({ ...gatilho, origem_mensagem: e.target.value as OrigemMensagemSemResposta })}
            >
              {ORIGENS_SEM_RESPOSTA.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.rotulo}
                </option>
              ))}
            </select>
          </Campo>
        </div>
      );
    case 'agendamento': {
      const porCron = gatilho.cron !== undefined;
      return (
        <>
          <div className="opcoes-radio" role="radiogroup" aria-label="Tipo de agendamento">
            <label className="caixa">
              <input type="radio" checked={porCron} onChange={() => aoMudar({ tipo: 'agendamento', cron: '0 9 * * 1-5' })} />
              Horário (cron)
            </label>
            <label className="caixa">
              <input type="radio" checked={!porCron} onChange={() => aoMudar({ tipo: 'agendamento', intervalo_s: 3600 })} />
              A cada intervalo
            </label>
          </div>
          {gatilho.cron !== undefined ? (
            <Campo rotulo="Expressão cron (5 campos, horário do Mac)" dica={'Ex.: "0 9 * * 1-5" = dias úteis às 9h; "*/30 * * * *" = a cada 30 min.'}>
              <input className="campo-codigo" value={gatilho.cron} onChange={(e) => aoMudar({ tipo: 'agendamento', cron: e.target.value })} />
            </Campo>
          ) : (
            <CampoDuracao
              rotulo="Intervalo"
              segundos={gatilho.intervalo_s ?? SEGUNDOS_UNIDADE.h}
              aoMudar={(s) => aoMudar({ tipo: 'agendamento', intervalo_s: s })}
              dica="Mínimo de 1 minuto."
            />
          )}
        </>
      );
    }
    case 'manual':
      return <p className="texto-secundario">Roda só pelo botão "Executar" ou pelo Claude (MCP).</p>;
  }
}

export function Gatilhos(props: {
  gatilhos: Gatilho[];
  aoMudar: (g: Gatilho[]) => void;
  erros: readonly ErroDefinicao[];
  incluirGrupos: boolean;
  /** Tipos oferecidos (o chatbot aceita menos). */
  tipos?: readonly TipoGatilho[];
}) {
  const tipos = props.tipos ?? TIPOS_GATILHO;
  const mudar = (i: number, g: Gatilho) => props.aoMudar(props.gatilhos.map((x, j) => (j === i ? g : x)));
  return (
    <section className="secao-editor" aria-labelledby="titulo-gatilhos">
      <h2 id="titulo-gatilhos">1. Quando</h2>
      <ErrosCampo erros={props.erros.filter((e) => e.caminho === 'gatilhos')} />
      {props.gatilhos.map((g, i) => (
        <div key={i} className="bloco-editor">
          <div className="bloco-editor-cabecalho">
            <select aria-label={`Gatilho ${i + 1}`} value={g.tipo} onChange={(e) => mudar(i, gatilhoPadrao(e.target.value as TipoGatilho))}>
              {tipos.map((t) => (
                <option key={t} value={t}>
                  {ROTULO_GATILHO[t]}
                </option>
              ))}
            </select>
            <BotaoIcone
              rotulo={`Remover gatilho ${i + 1}`}
              className="pequeno"
              disabled={props.gatilhos.length <= 1}
              onClick={() => props.aoMudar(props.gatilhos.filter((_, j) => j !== i))}
            >
              <Trash size={14} />
            </BotaoIcone>
          </div>
          <FormGatilho gatilho={g} aoMudar={(novo) => mudar(i, novo)} incluirGrupos={props.incluirGrupos} />
          <ErrosCampo erros={errosEm(props.erros, `gatilhos[${i}]`)} />
        </div>
      ))}
      <button
        type="button"
        className="botao-link"
        onClick={() => props.aoMudar([...props.gatilhos, gatilhoPadrao(tipos[0] ?? 'mensagem_recebida')])}
      >
        <Plus size={14} aria-hidden="true" /> Outro gatilho
      </button>
    </section>
  );
}
