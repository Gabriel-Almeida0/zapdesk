// Ações (T080): lista ordenada (com "aguardar") e formulário por tipo; variáveis `{nome}` com
// valores padrão. `FormAcao` é reutilizado pelo nó "ação" do chatbot.
import { ArrowDown, ArrowUp, Plus, Trash } from 'lucide-react';
import { useState } from 'react';

import { TIPOS_ACAO, type Acao, type ErroDefinicao, type TipoAcao } from '@zapdesk/cliente-motor';

import { BotaoIcone } from '../../componentes/BotaoIcone';
import { acaoPadrao, errosEm, novoId, ROTULO_ACAO } from '../../util/automacoes';
import { extrairVariaveis } from '../../util/formatar';
import { Campo, CampoDuracao, ErrosCampo, SeletorAutomacao, SeletorDisparo, SeletorEtiqueta, SeletorFunilEtapa, SeletorTemplate, useTemplate } from './campos';

export const VARIAVEIS_AUTOMACAO = ['nome', 'primeiro_nome', 'telefone', 'ultima_mensagem', 'conta'];
const VARIAVEIS_SEMPRE = new Set(['nome', 'primeiro_nome', 'telefone', 'conta']);

function ValoresPadrao(props: { variaveis: string[]; valores: Record<string, string>; aoMudar: (v: Record<string, string>) => void }) {
  const variaveis = props.variaveis.filter((v) => !VARIAVEIS_SEMPRE.has(v));
  if (variaveis.length === 0) return null;
  return (
    <fieldset className="campo">
      <legend>Se faltar a variável, usar</legend>
      <div className="grade-campos">
        {variaveis.map((v) => (
          <Campo key={v} rotulo={`{${v}}`}>
            <input
              value={props.valores[v] ?? ''}
              placeholder="(sem padrão: a ação falha)"
              onChange={(e) => {
                const novos = { ...props.valores };
                if (e.target.value) novos[v] = e.target.value;
                else delete novos[v];
                props.aoMudar(novos);
              }}
            />
          </Campo>
        ))}
      </div>
    </fieldset>
  );
}

function EntradaJson(props: { valor: Record<string, unknown> | undefined; aoMudar: (v: Record<string, unknown>) => void }) {
  const [texto, setTexto] = useState(() => JSON.stringify(props.valor ?? {}, null, 2));
  const [erro, setErro] = useState<string | null>(null);
  return (
    <Campo rotulo="Entrada (JSON)" dica="Textos aceitam variáveis, ex.: {&quot;pergunta&quot;: &quot;{ultima_mensagem}&quot;}.">
      <textarea
        className="campo-codigo"
        rows={3}
        value={texto}
        aria-invalid={erro ? true : undefined}
        onChange={(e) => {
          setTexto(e.target.value);
          try {
            const v: unknown = JSON.parse(e.target.value || '{}');
            if (!v || typeof v !== 'object' || Array.isArray(v)) throw new Error('objeto');
            setErro(null);
            props.aoMudar(v as Record<string, unknown>);
          } catch {
            setErro('JSON inválido: use um objeto, ex.: {"chave": "valor"}.');
          }
        }}
      />
      {erro ? <span className="erro-campo">{erro}</span> : null}
    </Campo>
  );
}

function TemplateComPadroes({ acao, aoMudar }: { acao: Extract<Acao, { tipo: 'enviar_template' }>; aoMudar: (a: Acao) => void }) {
  const template = useTemplate(acao.template_id);
  return (
    <>
      <SeletorTemplate valor={acao.template_id} aoMudar={(id) => aoMudar({ ...acao, template_id: id })} />
      {template ? <p className="previa-template texto-secundario">{template.texto}</p> : null}
      <ValoresPadrao variaveis={template?.variaveis ?? []} valores={acao.valores_padrao ?? {}} aoMudar={(v) => aoMudar({ ...acao, valores_padrao: v })} />
    </>
  );
}

export function FormAcao({ acao, aoMudar, automacaoId }: { acao: Acao; aoMudar: (a: Acao) => void; automacaoId?: string }) {
  switch (acao.tipo) {
    case 'enviar_texto':
      return (
        <>
          <Campo rotulo="Mensagem" dica={`Variáveis: ${VARIAVEIS_AUTOMACAO.map((v) => `{${v}}`).join(' ')} e campos do lead.`}>
            <textarea rows={3} maxLength={4096} value={acao.texto} onChange={(e) => aoMudar({ ...acao, texto: e.target.value })} />
          </Campo>
          <ValoresPadrao variaveis={extrairVariaveis(acao.texto)} valores={acao.valores_padrao ?? {}} aoMudar={(v) => aoMudar({ ...acao, valores_padrao: v })} />
        </>
      );
    case 'enviar_template':
      return <TemplateComPadroes acao={acao} aoMudar={aoMudar} />;
    case 'aguardar':
      return (
        <CampoDuracao
          rotulo="Aguardar"
          segundos={acao.duracao_s}
          aoMudar={(s) => aoMudar({ ...acao, duracao_s: s })}
          dica="De 1 minuto a 30 dias. A espera continua depois de fechar e reabrir o app."
        />
      );
    case 'adicionar_etiqueta':
    case 'remover_etiqueta':
      return <SeletorEtiqueta valor={acao.etiqueta_id} aoMudar={(id) => aoMudar({ ...acao, etiqueta_id: id })} />;
    case 'mover_etapa':
      return (
        <SeletorFunilEtapa
          funilId={acao.funil_id}
          etapaId={acao.etapa_id}
          aoMudar={(funil_id, etapa_id) => aoMudar({ ...acao, funil_id, etapa_id: etapa_id ?? '' })}
        />
      );
    case 'remover_do_funil':
      return <SeletorFunilEtapa semEtapa funilId={acao.funil_id} etapaId={null} aoMudar={(funil_id) => aoMudar({ ...acao, funil_id })} />;
    case 'atualizar_nota':
      return (
        <>
          <Campo rotulo="Texto da nota">
            <textarea rows={2} maxLength={10000} value={acao.texto} onChange={(e) => aoMudar({ ...acao, texto: e.target.value })} />
          </Campo>
          <div className="opcoes-radio" role="radiogroup" aria-label="Como gravar a nota">
            <label className="caixa">
              <input type="radio" checked={acao.modo === 'acrescentar'} onChange={() => aoMudar({ ...acao, modo: 'acrescentar' })} />
              Acrescentar numa nova linha
            </label>
            <label className="caixa">
              <input type="radio" checked={acao.modo === 'substituir'} onChange={() => aoMudar({ ...acao, modo: 'substituir' })} />
              Substituir a nota
            </label>
          </div>
        </>
      );
    case 'atualizar_campo_lead':
      return (
        <div className="grade-campos">
          <Campo rotulo="Campo" dica={'"nome" altera o nome do lead.'}>
            <input value={acao.campo} placeholder="empresa" onChange={(e) => aoMudar({ ...acao, campo: e.target.value })} />
          </Campo>
          <Campo rotulo="Valor" dica="Vazio remove o campo. Aceita variáveis.">
            <input value={acao.valor} onChange={(e) => aoMudar({ ...acao, valor: e.target.value })} />
          </Campo>
        </div>
      );
    case 'iniciar_chatbot':
      return (
        <SeletorAutomacao tipo="chatbot" rotulo="Chatbot" valor={acao.automacao_id} excluir={automacaoId} aoMudar={(id) => aoMudar({ ...acao, automacao_id: id })} />
      );
    case 'executar_ia':
      return (
        <>
          <SeletorAutomacao tipo="ia" rotulo="Automação de IA" valor={acao.automacao_id} aoMudar={(id) => aoMudar({ ...acao, automacao_id: id })} />
          <EntradaJson valor={acao.entrada} aoMudar={(entrada) => aoMudar({ ...acao, entrada })} />
          <Campo rotulo="Guardar o retorno na variável (opcional)" dica="Use depois como {variavel} nas próximas ações.">
            <input
              className="campo-codigo"
              value={acao.salvar_em ?? ''}
              placeholder="resposta_ia"
              onChange={(e) => aoMudar({ ...acao, salvar_em: e.target.value || null })}
            />
          </Campo>
        </>
      );
    case 'adicionar_a_disparo':
      return <SeletorDisparo soAbertos valor={acao.disparo_id || null} aoMudar={(id) => aoMudar({ ...acao, disparo_id: id ?? '' })} />;
    case 'pausar_automacoes':
      return (
        <div className="opcoes-radio">
          <label className="caixa">
            <input type="checkbox" checked={acao.duracao_min === null} onChange={(e) => aoMudar({ ...acao, duracao_min: e.target.checked ? null : 30 })} />
            Sem prazo (até alguém retomar)
          </label>
          {acao.duracao_min !== null ? (
            <label className="caixa">
              Por
              <input
                type="number"
                min={1}
                aria-label="Minutos de pausa"
                className="numero-curto"
                value={acao.duracao_min}
                onChange={(e) => aoMudar({ ...acao, duracao_min: Math.max(1, Number(e.target.value) || 1) })}
              />
              minutos
            </label>
          ) : null}
        </div>
      );
    case 'notificar':
      return (
        <div className="grade-campos">
          <Campo rotulo="Título">
            <input maxLength={60} value={acao.titulo} onChange={(e) => aoMudar({ ...acao, titulo: e.target.value })} />
          </Campo>
          <Campo rotulo="Texto">
            <input maxLength={240} value={acao.texto} onChange={(e) => aoMudar({ ...acao, texto: e.target.value })} />
          </Campo>
        </div>
      );
  }
}

export function Acoes(props: { acoes: Acao[]; aoMudar: (a: Acao[]) => void; erros: readonly ErroDefinicao[]; automacaoId?: string }) {
  const [nova, setNova] = useState<TipoAcao>('enviar_texto');
  const { acoes } = props;
  const mudar = (i: number, a: Acao) => props.aoMudar(acoes.map((x, j) => (j === i ? a : x)));
  const mover = (i: number, delta: number) => {
    const lista = [...acoes];
    const a = lista[i];
    const b = lista[i + delta];
    if (!a || !b) return;
    lista[i] = b;
    lista[i + delta] = a;
    props.aoMudar(lista);
  };
  return (
    <section className="secao-editor" aria-labelledby="titulo-acoes">
      <h2 id="titulo-acoes">3. Faça</h2>
      <ErrosCampo erros={props.erros.filter((e) => e.caminho === 'definicao.acoes')} />
      <ol className="lista-acoes">
        {acoes.map((a, i) => (
          <li key={a.id ?? i} className="bloco-editor">
            <div className="bloco-editor-cabecalho">
              <span className="numero-passo">{i + 1}</span>
              <select
                aria-label={`Ação ${i + 1}`}
                value={a.tipo}
                onChange={(e) => mudar(i, acaoPadrao(e.target.value as TipoAcao, a.id ?? novoId('acao', acoes.map((x) => x.id ?? ''))))}
              >
                {TIPOS_ACAO.map((t) => (
                  <option key={t} value={t}>
                    {ROTULO_ACAO[t]}
                  </option>
                ))}
              </select>
              <BotaoIcone rotulo={`Subir ação ${i + 1}`} className="pequeno" disabled={i === 0} onClick={() => mover(i, -1)}>
                <ArrowUp size={14} />
              </BotaoIcone>
              <BotaoIcone rotulo={`Descer ação ${i + 1}`} className="pequeno" disabled={i === acoes.length - 1} onClick={() => mover(i, 1)}>
                <ArrowDown size={14} />
              </BotaoIcone>
              <BotaoIcone rotulo={`Remover ação ${i + 1}`} className="pequeno" onClick={() => props.aoMudar(acoes.filter((_, j) => j !== i))}>
                <Trash size={14} />
              </BotaoIcone>
            </div>
            <FormAcao acao={a} aoMudar={(novo) => mudar(i, novo)} automacaoId={props.automacaoId} />
            <ErrosCampo erros={errosEm(props.erros, `definicao.acoes[${i}]`)} />
          </li>
        ))}
      </ol>
      <div className="linha-form">
        <select aria-label="Tipo da nova ação" value={nova} onChange={(e) => setNova(e.target.value as TipoAcao)}>
          {TIPOS_ACAO.map((t) => (
            <option key={t} value={t}>
              {ROTULO_ACAO[t]}
            </option>
          ))}
        </select>
        <button
          type="button"
          className="botao secundario pequeno"
          disabled={acoes.length >= 50}
          onClick={() => props.aoMudar([...acoes, acaoPadrao(nova, novoId('acao', acoes.map((x) => x.id ?? '')))])}
        >
          <Plus size={14} aria-hidden="true" /> Adicionar ação
        </button>
      </div>
    </section>
  );
}
