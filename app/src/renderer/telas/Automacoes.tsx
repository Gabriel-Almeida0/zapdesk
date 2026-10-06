// Lista de automações (T079): tipo, ativa (interruptor), prioridade, ok/erro nas últimas 24 h,
// duração média e avisos; filtro por tipo e busca.
import { Bot, Code, Plus, Search, TriangleAlert, Workflow } from 'lucide-react';
import { useState, type ReactNode } from 'react';
import { Link, useNavigate } from 'react-router';

import type { Automacao, TipoAutomacao } from '@zapdesk/cliente-motor';

import { useAutomacoes, useConfiguracaoAutomacoes } from '../api/automacoes';
import { EsqueletoLista } from '../componentes/Esqueleto';
import { EstadoVazio } from '../componentes/EstadoVazio';
import { FaixaAviso } from '../componentes/FaixaAviso';
import { TelaErro } from '../componentes/TelaErro';
import { ROTULO_TIPO_AUTOMACAO, textoDuracaoMs } from '../util/automacoes';
import { numero } from '../util/formatar';
import { useAtraso } from './Conversas';
import { InterruptorAtiva } from './EditorAutomacao';

export const ICONE_TIPO: Record<TipoAutomacao, ReactNode> = {
  fluxo: <Workflow size={18} aria-hidden="true" />,
  chatbot: <Bot size={18} aria-hidden="true" />,
  ia: <Code size={18} aria-hidden="true" />,
};

function LinhaAutomacao({ automacao }: { automacao: Automacao }) {
  const navegar = useNavigate();
  const [erro, setErro] = useState<string | null>(null);
  const e = automacao.estatisticas_24h;
  const bloqueio = automacao.tipo === 'ia' && automacao.ia && !automacao.ia.compilacao_ok ? 'Corrija os erros de compilação para ativar.' : null;
  return (
    <li className="linha-automacao">
      <button type="button" className="linha-automacao-corpo" onClick={() => void navegar(`/automacoes/${automacao.id}`)}>
        <span className={`icone-tipo tipo-${automacao.tipo}`}>{ICONE_TIPO[automacao.tipo]}</span>
        <span className="linha-automacao-textos">
          <strong>{automacao.nome}</strong>
          <small>
            {ROTULO_TIPO_AUTOMACAO[automacao.tipo]} · prioridade {automacao.prioridade}
            {automacao.tipo === 'chatbot' && automacao.sessoes_ativas > 0 ? ` · ${numero(automacao.sessoes_ativas)} conversas no bot` : ''}
            {automacao.descricao ? ` · ${automacao.descricao}` : ''}
          </small>
        </span>
        <span className="linha-automacao-numeros" aria-label="Últimas 24 horas">
          <span className="selo ok" title="Execuções ok nas últimas 24 h">
            {numero(e.ok)} ok
          </span>
          {e.erro > 0 ? (
            <span className="selo erro" title="Execuções com erro nas últimas 24 h">
              {numero(e.erro)} erro
            </span>
          ) : null}
          <small className="texto-secundario" title="Duração média">
            {textoDuracaoMs(e.duracao_media_ms)}
          </small>
          {automacao.avisos.length > 0 || automacao.desativada_motivo === 'erros_seguidos' ? (
            <span
              className="aviso-automacao"
              title={
                automacao.desativada_motivo === 'erros_seguidos'
                  ? 'Desativada depois de 5 erros seguidos'
                  : automacao.avisos.map((a) => a.mensagem).join('\n')
              }
            >
              <TriangleAlert size={16} aria-label="Avisos" role="img" />
            </span>
          ) : null}
        </span>
      </button>
      <InterruptorAtiva automacao={automacao} bloqueado={bloqueio} aoErro={setErro} />
      {erro ? <p className="erro-campo linha-automacao-erro">{erro}</p> : null}
    </li>
  );
}

export function Automacoes() {
  const [tipo, setTipo] = useState<TipoAutomacao | 'todas'>('todas');
  const [busca, setBusca] = useState('');
  const termo = useAtraso(busca.trim(), 250);
  const consulta = useAutomacoes({ ...(tipo === 'todas' ? {} : { tipo }), ...(termo ? { busca: termo } : {}) });
  const configuracao = useConfiguracaoAutomacoes();
  const lista = [...(consulta.data ?? [])].sort((a, b) => a.prioridade - b.prioridade || a.criada_em.localeCompare(b.criada_em));

  return (
    <section className="tela tela-rolavel">
      <header className="cabecalho-tela">
        <h1>Automações</h1>
        <Link to="/automacoes/nova" className="botao">
          <Plus size={16} aria-hidden="true" /> Nova automação
        </Link>
      </header>
      {configuracao.data?.pausa_geral ? (
        <FaixaAviso tipo="aviso" acao={<Link to="/ajustes#automacoes">Ajustes</Link>}>
          Todas as automações estão pausadas.
        </FaixaAviso>
      ) : null}
      <div className="barra-ferramentas">
        <div className="filtros" role="group" aria-label="Filtrar por tipo">
          {(['todas', 'fluxo', 'chatbot', 'ia'] as const).map((t) => (
            <button key={t} type="button" className={`chip${tipo === t ? ' ativo' : ''}`} aria-pressed={tipo === t} onClick={() => setTipo(t)}>
              {t === 'todas' ? 'Todas' : ROTULO_TIPO_AUTOMACAO[t]}
            </button>
          ))}
        </div>
        <label className="campo-busca">
          <Search size={16} aria-hidden="true" />
          <input type="search" aria-label="Buscar automações" placeholder="Buscar" value={busca} onChange={(e) => setBusca(e.target.value)} />
        </label>
      </div>
      {consulta.isPending ? (
        <EsqueletoLista linhas={4} />
      ) : consulta.isError ? (
        <TelaErro erro={consulta.error} aoTentar={() => void consulta.refetch()} />
      ) : lista.length === 0 ? (
        <EstadoVazio
          icone={<Workflow size={56} />}
          titulo={termo || tipo !== 'todas' ? 'Nada encontrado' : 'Nenhuma automação ainda'}
          texto="Fluxos movem leads no funil e mandam follow-ups; chatbots qualificam por menus e perguntas; automações de IA são programadas em TypeScript."
          acao={
            <Link to="/automacoes/nova" className="botao">
              Criar automação
            </Link>
          }
        />
      ) : (
        <ul className="lista-automacoes">
          {lista.map((a) => (
            <LinhaAutomacao key={a.id} automacao={a} />
          ))}
        </ul>
      )}
    </section>
  );
}
