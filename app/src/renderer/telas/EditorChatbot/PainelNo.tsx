// Painel de propriedades do nó selecionado (T114). Sem nó selecionado mostra as configurações do
// bot (mensagem de "não entendi", tentativas, inatividade).
import { Plus, Trash } from 'lucide-react';

import type { DefinicaoChatbot, ErroDefinicao, NoChatbot, TipoAcao, TipoValidacaoPergunta } from '@zapdesk/cliente-motor';
import { TIPOS_ACAO } from '@zapdesk/cliente-motor';

import { BotaoIcone } from '../../componentes/BotaoIcone';
import { acaoPadrao, ROTULO_ACAO } from '../../util/automacoes';
import { FormAcao } from '../EditorFluxo/Acoes';
import { Campo, ErrosCampo, SeletorAutomacao } from '../EditorFluxo/campos';
import { EditorCondicoes } from '../EditorFluxo/Condicoes';
import { separarPalavras } from '../EditorFluxo/Gatilhos';
import { ROTULO_NO } from './grafo';

const ACOES_NO: TipoAcao[] = TIPOS_ACAO.filter((t) => t !== 'aguardar' && t !== 'iniciar_chatbot');

const VALIDACOES: { id: TipoValidacaoPergunta; rotulo: string }[] = [
  { id: 'nenhuma', rotulo: 'Qualquer resposta' },
  { id: 'email', rotulo: 'E-mail' },
  { id: 'numero', rotulo: 'Número' },
  { id: 'telefone', rotulo: 'Telefone' },
  { id: 'regex', rotulo: 'Expressão regular' },
];

function FormNo({ no, aoMudar, automacaoId }: { no: NoChatbot; aoMudar: (n: NoChatbot) => void; automacaoId?: string }) {
  switch (no.tipo) {
    case 'inicio':
      return <p className="texto-secundario">O bot começa aqui quando o gatilho acontece. Ligue a saída ao primeiro passo.</p>;
    case 'mensagem':
      return (
        <Campo rotulo="Mensagem" dica="Aceita variáveis: {nome}, {primeiro_nome} e as capturadas nas perguntas.">
          <textarea rows={4} maxLength={4096} value={no.texto} onChange={(e) => aoMudar({ ...no, texto: e.target.value })} />
        </Campo>
      );
    case 'menu':
      return (
        <>
          <Campo rotulo="Texto do menu">
            <textarea rows={3} value={no.texto} onChange={(e) => aoMudar({ ...no, texto: e.target.value })} />
          </Campo>
          <fieldset className="campo">
            <legend>Opções (o contato responde o número ou o texto)</legend>
            {no.opcoes.map((o, i) => (
              <div key={i} className="opcao-menu">
                <span className="numero-passo">{i + 1}</span>
                <input
                  aria-label={`Opção ${i + 1}`}
                  value={o.rotulo}
                  onChange={(e) => aoMudar({ ...no, opcoes: no.opcoes.map((x, j) => (j === i ? { ...x, rotulo: e.target.value } : x)) })}
                />
                <input
                  aria-label={`Sinônimos da opção ${i + 1}`}
                  placeholder="sinônimos, separados por vírgula"
                  defaultValue={o.valores.join(', ')}
                  onChange={(e) => aoMudar({ ...no, opcoes: no.opcoes.map((x, j) => (j === i ? { ...x, valores: separarPalavras(e.target.value) } : x)) })}
                />
                <BotaoIcone
                  rotulo={`Remover opção ${i + 1}`}
                  className="pequeno"
                  disabled={no.opcoes.length <= 1}
                  onClick={() => aoMudar({ ...no, opcoes: no.opcoes.filter((_, j) => j !== i) })}
                >
                  <Trash size={13} />
                </BotaoIcone>
              </div>
            ))}
            <button
              type="button"
              className="botao-link"
              disabled={no.opcoes.length >= 10}
              onClick={() => aoMudar({ ...no, opcoes: [...no.opcoes, { rotulo: `Opção ${no.opcoes.length + 1}`, valores: [], proximo: '' }] })}
            >
              <Plus size={14} aria-hidden="true" /> Opção
            </button>
          </fieldset>
          <label className="caixa">
            <input type="checkbox" checked={no.mostrar_numeros ?? true} onChange={(e) => aoMudar({ ...no, mostrar_numeros: e.target.checked })} />
            Enviar a lista numerada ("1 - Preços") depois do texto
          </label>
          <p className="texto-secundario">"Ao esgotar" sem ligação transfere para humano.</p>
        </>
      );
    case 'pergunta': {
      const validacao = no.validacao ?? { tipo: 'nenhuma' as const };
      return (
        <>
          <Campo rotulo="Pergunta">
            <textarea rows={3} value={no.texto} onChange={(e) => aoMudar({ ...no, texto: e.target.value })} />
          </Campo>
          <Campo rotulo="Guardar a resposta na variável" dica="Letras minúsculas, números e _. Use depois como {variavel}.">
            <input className="campo-codigo" value={no.variavel} maxLength={40} onChange={(e) => aoMudar({ ...no, variavel: e.target.value.toLowerCase() })} />
          </Campo>
          <Campo rotulo="Validação">
            <select value={validacao.tipo} onChange={(e) => aoMudar({ ...no, validacao: { ...validacao, tipo: e.target.value as TipoValidacaoPergunta } })}>
              {VALIDACOES.map((v) => (
                <option key={v.id} value={v.id}>
                  {v.rotulo}
                </option>
              ))}
            </select>
          </Campo>
          {validacao.tipo === 'regex' ? (
            <Campo rotulo="Padrão (RE2)">
              <input className="campo-codigo" value={validacao.padrao ?? ''} onChange={(e) => aoMudar({ ...no, validacao: { ...validacao, padrao: e.target.value } })} />
            </Campo>
          ) : null}
          {validacao.tipo !== 'nenhuma' ? (
            <Campo rotulo="Mensagem quando inválido (opcional)" dica='Sem ela, usa o "não entendi" do bot.'>
              <input value={validacao.mensagem_erro ?? ''} onChange={(e) => aoMudar({ ...no, validacao: { ...validacao, mensagem_erro: e.target.value || null } })} />
            </Campo>
          ) : null}
        </>
      );
    }
    case 'condicao':
      return (
        <>
          {no.ramos.map((r, i) => (
            <fieldset key={i} className="grupo-campos">
              <legend>
                Ramo {i + 1}{' '}
                <BotaoIcone
                  rotulo={`Remover ramo ${i + 1}`}
                  className="pequeno"
                  disabled={no.ramos.length <= 1}
                  onClick={() => aoMudar({ ...no, ramos: no.ramos.filter((_, j) => j !== i) })}
                >
                  <Trash size={13} />
                </BotaoIcone>
              </legend>
              <EditorCondicoes
                comVariavel
                condicoes={r.condicoes}
                prefixo={`nos.${no.id}.ramos[${i}].condicoes`}
                erros={[]}
                aoMudar={(c) => aoMudar({ ...no, ramos: no.ramos.map((x, j) => (j === i ? { ...x, condicoes: c } : x)) })}
              />
            </fieldset>
          ))}
          <button
            type="button"
            className="botao-link"
            onClick={() => aoMudar({ ...no, ramos: [...no.ramos, { condicoes: { modo: 'todas', regras: [] }, proximo: '' }] })}
          >
            <Plus size={14} aria-hidden="true" /> Ramo
          </button>
          <p className="texto-secundario">O primeiro ramo cujas condições valem é seguido; nenhum → "Senão".</p>
        </>
      );
    case 'acao':
      return (
        <>
          <Campo rotulo="Ação">
            <select value={no.acao.tipo} onChange={(e) => aoMudar({ ...no, acao: acaoPadrao(e.target.value as TipoAcao, no.acao.id ?? `${no.id}_acao`) })}>
              {ACOES_NO.map((t) => (
                <option key={t} value={t}>
                  {ROTULO_ACAO[t]}
                </option>
              ))}
            </select>
          </Campo>
          <FormAcao acao={no.acao} aoMudar={(a) => aoMudar({ ...no, acao: a })} automacaoId={automacaoId} />
        </>
      );
    case 'ia':
      return (
        <>
          <SeletorAutomacao tipo="ia" rotulo="Automação de IA (aoExecutar)" valor={no.automacao_id} aoMudar={(id) => aoMudar({ ...no, automacao_id: id })} />
          <div className="opcoes-radio" role="radiogroup" aria-label="Uso do retorno">
            <label className="caixa">
              <input type="radio" checked={no.modo === 'responder'} onChange={() => aoMudar({ ...no, modo: 'responder', variavel: null })} />
              Enviar o retorno como resposta
            </label>
            <label className="caixa">
              <input type="radio" checked={no.modo === 'variavel'} onChange={() => aoMudar({ ...no, modo: 'variavel', variavel: no.variavel ?? 'resposta_ia' })} />
              Guardar numa variável
            </label>
          </div>
          {no.modo === 'variavel' ? (
            <Campo rotulo="Variável">
              <input className="campo-codigo" value={no.variavel ?? ''} onChange={(e) => aoMudar({ ...no, variavel: e.target.value.toLowerCase() })} />
            </Campo>
          ) : null}
          <p className="texto-secundario">"Em erro" sem ligação transfere para humano.</p>
        </>
      );
    case 'humano':
      return (
        <Campo rotulo="Mensagem antes de transferir (opcional)">
          <textarea rows={2} value={no.mensagem ?? ''} onChange={(e) => aoMudar({ ...no, mensagem: e.target.value || null })} />
        </Campo>
      );
    case 'fim':
      return (
        <Campo rotulo="Mensagem final (opcional)">
          <textarea rows={2} value={no.mensagem ?? ''} onChange={(e) => aoMudar({ ...no, mensagem: e.target.value || null })} />
        </Campo>
      );
  }
}

export function PainelNo(props: {
  no: NoChatbot | null;
  definicao: DefinicaoChatbot;
  erros: readonly ErroDefinicao[];
  automacaoId?: string;
  aoMudarNo: (n: NoChatbot) => void;
  aoMudarDefinicao: (d: DefinicaoChatbot) => void;
  aoExcluirNo: (id: string) => void;
}) {
  const { no, definicao } = props;
  if (!no) {
    return (
      <div className="painel-no">
        <h2>Configurações do bot</h2>
        <Campo rotulo='Mensagem de "não entendi"'>
          <textarea rows={2} maxLength={500} value={definicao.nao_entendi} onChange={(e) => props.aoMudarDefinicao({ ...definicao, nao_entendi: e.target.value })} />
        </Campo>
        <div className="grade-campos">
          <Campo rotulo="Tentativas antes de desistir">
            <input
              type="number"
              min={1}
              max={10}
              value={definicao.max_tentativas}
              onChange={(e) => props.aoMudarDefinicao({ ...definicao, max_tentativas: Math.min(10, Math.max(1, Number(e.target.value) || 1)) })}
            />
          </Campo>
          <Campo rotulo="Encerrar sem resposta após (min)">
            <input
              type="number"
              min={1}
              max={1440}
              value={definicao.inatividade_min}
              onChange={(e) => props.aoMudarDefinicao({ ...definicao, inatividade_min: Math.min(1440, Math.max(1, Number(e.target.value) || 1)) })}
            />
          </Campo>
        </div>
        <ErrosCampo erros={props.erros.filter((e) => !e.no_id)} />
        <p className="texto-secundario">Clique num nó para editar. Arraste da bolinha de saída até outro nó para ligar; selecione uma ligação e aperte Delete para removê-la.</p>
      </div>
    );
  }
  return (
    <div className="painel-no">
      <div className="painel-no-cabecalho">
        <h2>{ROTULO_NO[no.tipo]}</h2>
        <code className="texto-secundario">{no.id}</code>
        {no.id !== definicao.inicio ? (
          <BotaoIcone rotulo={`Excluir nó ${no.id}`} className="pequeno" onClick={() => props.aoExcluirNo(no.id)}>
            <Trash size={14} />
          </BotaoIcone>
        ) : null}
      </div>
      <ErrosCampo erros={props.erros.filter((e) => e.no_id === no.id)} />
      <FormNo no={no} aoMudar={props.aoMudarNo} automacaoId={props.automacaoId} />
    </div>
  );
}
