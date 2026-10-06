// "Nova automação" (T079): escolher Fluxo, Chatbot ou IA (código). Fluxo e chatbot abrem o editor
// em branco (salvos no primeiro "Salvar"); IA escolhe um modelo de projeto e já cria a pasta.
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft } from 'lucide-react';
import { useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router';

import type { IdModeloProjeto, ModeloProjeto, TipoAutomacao } from '@zapdesk/cliente-motor';

import { chaves } from '../api/chaves';
import { useCliente } from '../api/motor';
import { FaixaAviso } from '../componentes/FaixaAviso';
import { textoErro } from '../util/formatar';
import { ICONE_TIPO } from './Automacoes';
import { EditorChatbot } from './EditorChatbot/EditorChatbot';
import { EditorFluxo } from './EditorFluxo/EditorFluxo';

const DESCRICAO_TIPO: Record<TipoAutomacao, string> = {
  fluxo: 'Gatilho → condições → ações, com esperas. Ex.: lead respondeu o disparo → etiqueta "quente" → mover para "Qualificando".',
  chatbot: 'Conversa em nós: menu de opções, perguntas com validação, condições e transferência para humano.',
  ia: 'Projeto TypeScript com a API da automação: responder com Claude, classificar, extrair dados. Programe no app ou no VS Code.',
};

/** Modelos de projeto (fallback se o motor ainda não responder a lista). */
export const MODELOS_PADRAO: ModeloProjeto[] = [
  { id: 'responder_historico', nome: 'Responder com IA usando histórico', descricao: 'Lê a conversa e responde com Claude, com prompt conservador.' },
  { id: 'classificar_funil', nome: 'Classificar lead e mover no funil', descricao: 'Classifica o interesse do lead e move para a etapa certa.' },
  { id: 'extrair_dados', nome: 'Extrair dados (nome, empresa, interesse)', descricao: 'Extrai dados da conversa e grava nos campos do lead.' },
  { id: 'em_branco', nome: 'Em branco', descricao: 'Só o esqueleto com definirAutomacao.' },
];

function NovaIA() {
  const cliente = useCliente();
  const qc = useQueryClient();
  const navegar = useNavigate();
  const modelos = useQuery({ queryKey: chaves.modelosProjeto, queryFn: () => cliente.listarModelosProjeto(), staleTime: Infinity });
  const lista = modelos.data && modelos.data.length > 0 ? modelos.data : MODELOS_PADRAO;
  const [modelo, setModelo] = useState<IdModeloProjeto>('responder_historico');
  const [nome, setNome] = useState('');
  const criar = useMutation({
    mutationFn: () => cliente.criarAutomacaoIA({ nome: nome.trim(), modelo }),
    onSuccess: (a) => {
      qc.setQueryData(chaves.automacao(a.id), a);
      void qc.invalidateQueries({ queryKey: chaves.automacoes });
      void navegar(`/automacoes/${a.id}`, { replace: true });
    },
  });
  return (
    <form
      className="cartao formulario"
      onSubmit={(e) => {
        e.preventDefault();
        if (nome.trim()) criar.mutate();
      }}
    >
      <h2>Automação de IA</h2>
      <fieldset className="campo">
        <legend>Modelo de projeto</legend>
        <div className="grade-modelos" role="radiogroup" aria-label="Modelo de projeto">
          {lista.map((m) => (
            <label key={m.id} className={`opcao-modelo${modelo === m.id ? ' escolhida' : ''}`}>
              <input type="radio" name="modelo" checked={modelo === m.id} onChange={() => setModelo(m.id)} />
              <strong>{m.nome}</strong>
              <small>{m.descricao}</small>
            </label>
          ))}
        </div>
      </fieldset>
      <label className="campo">
        <span>Nome</span>
        <input value={nome} maxLength={80} placeholder="Ex.: Responder dúvidas de preço" onChange={(e) => setNome(e.target.value)} autoFocus />
      </label>
      {criar.error ? <FaixaAviso tipo="erro">{textoErro(criar.error)}</FaixaAviso> : null}
      <div className="acoes-formulario">
        <Link to="/automacoes/nova" className="botao secundario">
          Voltar
        </Link>
        <button type="submit" className="botao" disabled={!nome.trim() || criar.isPending}>
          {criar.isPending ? 'Criando e compilando…' : 'Criar e abrir o editor'}
        </button>
      </div>
    </form>
  );
}

export function NovaAutomacao() {
  const [parametros] = useSearchParams();
  const tipo = parametros.get('tipo');
  if (tipo === 'fluxo') return <EditorFluxo />;
  if (tipo === 'chatbot') return <EditorChatbot />;
  return (
    <section className="tela tela-rolavel">
      <header className="cabecalho-tela">
        <Link to="/automacoes" className="botao-icone" aria-label="Voltar para Automações" title="Voltar para Automações">
          <ArrowLeft size={20} />
        </Link>
        <h1>Nova automação</h1>
      </header>
      {tipo === 'ia' ? (
        <NovaIA />
      ) : (
        <div className="grade-tipos">
          {(['fluxo', 'chatbot', 'ia'] as const).map((t) => (
            <Link key={t} to={`/automacoes/nova?tipo=${t}`} className="cartao opcao-tipo">
              <span className={`icone-tipo tipo-${t}`}>{ICONE_TIPO[t]}</span>
              <strong>{t === 'ia' ? 'IA (código)' : t === 'fluxo' ? 'Fluxo' : 'Chatbot'}</strong>
              <small>{DESCRICAO_TIPO[t]}</small>
            </Link>
          ))}
        </div>
      )}
    </section>
  );
}
