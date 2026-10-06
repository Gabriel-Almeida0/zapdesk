// Ajustes (T063, T145): contas (renomear, reconectar, remover), versão, pasta de dados e logs, e
// "Usar com Claude" com os comandos prontos para Claude Code e Claude Desktop.
// 003 (T039, US5): "Aparência" — Sistema / Claro / Escuro, troca na hora e lembrada (util/tema.ts).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Bot, Check, Copy, FolderOpen, Monitor, Moon, Palette, Plus, Sparkles, Sun, Workflow } from 'lucide-react';
import { useState } from 'react';
import { Link, useNavigate } from 'react-router';

import type { Conta } from '@zapdesk/cliente-motor';

import type { InfoMcp } from '../../preload/tipos';
import { chaves } from '../api/chaves';
import { useCliente } from '../api/motor';
import { Avatar } from '../componentes/Avatar';
import { BotaoIcone } from '../componentes/BotaoIcone';
import { Esqueleto } from '../componentes/Esqueleto';
import { FaixaAviso } from '../componentes/FaixaAviso';
import { Confirmar } from '../componentes/Modal';
import { ROTULO_ESTADO_CONTA, useContaAtual } from '../estado/conta';
import { telefone, textoErro } from '../util/formatar';
import { aplicarTema, lerPreferencia, salvarPreferencia, type PreferenciaTema } from '../util/tema';
import { AjustesAutomacoes } from './secoes/AjustesAutomacoes';
import { AjustesIA } from './secoes/AjustesIA';

/** Aspas simples para o shell (caminhos com espaço, ex.: "Application Support"). */
function aspas(valor: string): string {
  return `"${valor.replace(/(["\\$`])/g, '\\$1')}"`;
}

export function comandoClaudeCode(info: Pick<InfoMcp, 'executavel' | 'script'>): string {
  return `claude mcp add -s user -e ELECTRON_RUN_AS_NODE=1 zapdesk -- ${aspas(info.executavel)} ${aspas(info.script)}`;
}

export function configClaudeDesktop(info: Pick<InfoMcp, 'executavel' | 'script'>): string {
  return JSON.stringify(
    {
      mcpServers: {
        zapdesk: { command: info.executavel, args: [info.script], env: { ELECTRON_RUN_AS_NODE: '1' } },
      },
    },
    null,
    2,
  );
}

function BotaoCopiar({ texto, rotulo }: { texto: string; rotulo: string }) {
  const [copiado, setCopiado] = useState(false);
  return (
    <button
      type="button"
      className="botao secundario pequeno"
      aria-label={rotulo}
      onClick={() => {
        void window.zapdesk.copiar(texto).then(() => {
          setCopiado(true);
          setTimeout(() => setCopiado(false), 1800);
        });
      }}
    >
      {copiado ? <Check size={14} aria-hidden="true" /> : <Copy size={14} aria-hidden="true" />} {copiado ? 'Copiado' : 'Copiar'}
    </button>
  );
}

function LinhaConta({ conta }: { conta: Conta }) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const navegar = useNavigate();
  const [editando, setEditando] = useState(false);
  const [nome, setNome] = useState(conta.nome);
  const [removendo, setRemovendo] = useState(false);

  const renomear = useMutation({
    mutationFn: () => cliente.renomearConta(conta.id, nome.trim()),
    onSuccess: (c) => {
      qc.setQueryData<Conta[]>(chaves.contas, (l) => l?.map((x) => (x.id === c.id ? c : x)));
      setEditando(false);
    },
  });
  const reconectar = useMutation({
    mutationFn: () => cliente.reconectarConta(conta.id),
    onSuccess: () => navegar(`/contas/conectar?conta=${encodeURIComponent(conta.id)}`),
  });
  const remover = useMutation({
    mutationFn: () => cliente.removerConta(conta.id),
    onSuccess: () => {
      qc.setQueryData<Conta[]>(chaves.contas, (l) => l?.filter((x) => x.id !== conta.id));
      setRemovendo(false);
    },
  });
  const erro = renomear.error ?? reconectar.error ?? remover.error;

  return (
    <li className="linha-conta">
      <Avatar nome={conta.nome} chave={conta.id} tamanho={40} />
      <div className="linha-conta-textos">
        {editando ? (
          <form
            className="linha-form"
            onSubmit={(e) => {
              e.preventDefault();
              renomear.mutate();
            }}
          >
            <input aria-label="Nome da conta" value={nome} maxLength={60} onChange={(e) => setNome(e.target.value)} autoFocus />
            <button type="submit" className="botao pequeno" disabled={!nome.trim() || renomear.isPending}>
              Salvar
            </button>
            <button type="button" className="botao secundario pequeno" onClick={() => setEditando(false)}>
              Cancelar
            </button>
          </form>
        ) : (
          <strong>{conta.nome}</strong>
        )}
        <small>
          {conta.telefone ? `${telefone(conta.telefone)} · ` : ''}
          <span className={`estado-conta estado-${conta.estado}`}>{ROTULO_ESTADO_CONTA[conta.estado]}</span>
        </small>
        {erro ? <FaixaAviso tipo="erro">{textoErro(erro)}</FaixaAviso> : null}
      </div>
      <div className="linha-conta-acoes">
        {!editando ? (
          <button type="button" className="botao secundario pequeno" onClick={() => setEditando(true)}>
            Renomear
          </button>
        ) : null}
        {conta.estado === 'desconectada' ? (
          <button type="button" className="botao pequeno" disabled={reconectar.isPending} onClick={() => reconectar.mutate()}>
            Reconectar
          </button>
        ) : null}
        <button type="button" className="botao secundario pequeno perigo-texto" onClick={() => setRemovendo(true)}>
          Remover conta
        </button>
      </div>
      {removendo ? (
        <Confirmar
          titulo={`Remover ${conta.nome}?`}
          texto="O ZapDesk desconecta o número e apaga deste Mac as conversas, mensagens, contatos e mídias desta conta. Disparos da conta são cancelados. Os leads continuam na base."
          confirmar="Remover conta"
          perigo
          ocupado={remover.isPending}
          aoConfirmar={() => remover.mutate()}
          aoFechar={() => setRemovendo(false)}
        />
      ) : null}
    </li>
  );
}

function UsarComClaude() {
  const info = useQuery({ queryKey: ['info-mcp'], queryFn: () => window.zapdesk.infoMcp(), staleTime: Infinity });
  if (info.isPending) return <Esqueleto altura={120} />;
  if (info.isError || !info.data) return <FaixaAviso tipo="erro">{textoErro(info.error)}</FaixaAviso>;
  const dados = info.data;
  const comando = comandoClaudeCode(dados);
  const json = configClaudeDesktop(dados);
  return (
    <>
      <p>
        O ZapDesk tem um servidor MCP: o Claude pode importar leads, ler e enviar mensagens, criar disparos e gerenciar
        templates e etiquetas. Se o app estiver fechado, o Claude abre o ZapDesk sozinho.
      </p>
      <FaixaAviso tipo="aviso">Disparos criados pelo Claude começam na hora, sem confirmação. Você pode pausar ou cancelar em Disparos.</FaixaAviso>
      {!dados.scriptExiste ? (
        <FaixaAviso tipo="erro">
          O arquivo do MCP não foi encontrado em {dados.script}.
          {dados.empacotado ? ' Reinstale o ZapDesk.' : ' Em desenvolvimento, rode: npm run compilar -w @zapdesk/mcp'}
        </FaixaAviso>
      ) : null}
      <h3>Claude Code</h3>
      <p className="texto-secundario">Rode no Terminal (vale para todos os projetos):</p>
      <div className="bloco-codigo">
        <pre>
          <code>{comando}</code>
        </pre>
        <BotaoCopiar texto={comando} rotulo="Copiar comando do Claude Code" />
      </div>
      <h3>Claude Desktop</h3>
      <p className="texto-secundario">
        Abra <code>~/Library/Application Support/Claude/claude_desktop_config.json</code> (Claude › Ajustes › Desenvolvedor ›
        Editar configuração), junte o bloco abaixo em <code>mcpServers</code> e reinicie o Claude Desktop:
      </p>
      <div className="bloco-codigo">
        <pre>
          <code>{json}</code>
        </pre>
        <BotaoCopiar texto={json} rotulo="Copiar configuração do Claude Desktop" />
      </div>
      {!dados.empacotado ? (
        <p className="texto-secundario">
          Você está rodando em modo de desenvolvimento: os caminhos acima apontam para o Electron de desenvolvimento. Depois de
          instalar o .dmg, volte aqui para copiar os caminhos definitivos.
        </p>
      ) : null}
    </>
  );
}

const OPCOES_TEMA: { valor: PreferenciaTema; rotulo: string; icone: typeof Monitor }[] = [
  { valor: 'sistema', rotulo: 'Sistema', icone: Monitor },
  { valor: 'claro', rotulo: 'Claro', icone: Sun },
  { valor: 'escuro', rotulo: 'Escuro', icone: Moon },
];

/** Tema da janela: "Sistema" segue o macOS; "Claro"/"Escuro" fixam. Vale na hora e fica salvo neste Mac. */
export function Aparencia() {
  const [preferencia, setPreferencia] = useState<PreferenciaTema>(() => lerPreferencia());
  const escolher = (p: PreferenciaTema) => {
    setPreferencia(p);
    salvarPreferencia(p);
    aplicarTema(p);
  };
  return (
    <div className="cartao" id="aparencia">
      <div className="cartao-cabecalho">
        <h2 id="titulo-aparencia">
          <Palette size={20} aria-hidden="true" /> Aparência
        </h2>
      </div>
      <div className="seletor-tema" role="radiogroup" aria-labelledby="titulo-aparencia">
        {OPCOES_TEMA.map(({ valor, rotulo, icone: Icone }) => (
          <label key={valor} className="opcao-tema">
            <input
              type="radio"
              name="tema"
              value={valor}
              checked={preferencia === valor}
              onChange={() => escolher(valor)}
            />
            <Icone size={15} aria-hidden="true" />
            {rotulo}
          </label>
        ))}
      </div>
      <p className="texto-secundario">“Sistema” acompanha o tema do macOS. A escolha fica guardada só neste Mac.</p>
    </div>
  );
}

export function Ajustes() {
  const cliente = useCliente();
  const { contas } = useContaAtual();
  const sistema = useQuery({ queryKey: chaves.sistema, queryFn: () => cliente.sistema() });

  return (
    <section className="tela tela-rolavel ajustes">
      <header className="cabecalho-tela">
        <h1>Ajustes</h1>
      </header>

      <div className="cartao">
        <div className="cartao-cabecalho">
          <h2>Contas</h2>
          <Link to="/contas/conectar" className="botao pequeno">
            <Plus size={14} aria-hidden="true" /> Conectar conta
          </Link>
        </div>
        {contas.length === 0 ? (
          <p className="texto-secundario">Nenhuma conta conectada.</p>
        ) : (
          <ul className="lista-contas">
            {contas.map((c) => (
              <LinhaConta key={c.id} conta={c} />
            ))}
          </ul>
        )}
      </div>

      <div className="cartao" id="ia">
        <div className="cartao-cabecalho">
          <h2>
            <Sparkles size={20} aria-hidden="true" /> IA
          </h2>
        </div>
        <AjustesIA />
      </div>

      <div className="cartao" id="automacoes">
        <div className="cartao-cabecalho">
          <h2>
            <Workflow size={20} aria-hidden="true" /> Automações
          </h2>
        </div>
        <AjustesAutomacoes />
      </div>

      <div className="cartao">
        <div className="cartao-cabecalho">
          <h2>
            <Bot size={20} aria-hidden="true" /> Usar com Claude
          </h2>
        </div>
        <UsarComClaude />
      </div>

      <div className="cartao">
        <h2>Sobre</h2>
        {sistema.isPending ? (
          <Esqueleto altura={60} />
        ) : sistema.isError ? (
          <FaixaAviso tipo="erro">{textoErro(sistema.error)}</FaixaAviso>
        ) : (
          <dl className="lista-definicoes">
            <dt>Versão</dt>
            <dd>
              {sistema.data.versao}
              {sistema.data.whatsapp === 'falso' ? ' · WhatsApp simulado (modo de desenvolvimento)' : ''}
            </dd>
            <dt>Pasta de dados</dt>
            <dd className="com-acao">
              <code>{sistema.data.pasta_dados}</code>
              <BotaoIcone rotulo="Mostrar pasta de dados no Finder" onClick={() => void window.zapdesk.mostrarNoFinder(sistema.data.pasta_dados)}>
                <FolderOpen size={16} />
              </BotaoIcone>
            </dd>
            <dt>Logs</dt>
            <dd className="com-acao">
              <code>{sistema.data.caminho_logs}</code>
              <BotaoIcone rotulo="Mostrar logs no Finder" onClick={() => void window.zapdesk.mostrarNoFinder(sistema.data.caminho_logs)}>
                <FolderOpen size={16} />
              </BotaoIcone>
            </dd>
            <dt>Contas conectadas</dt>
            <dd>{sistema.data.contas_conectadas}</dd>
            <dt>Disparos em andamento</dt>
            <dd>{sistema.data.disparos_ativos}</dd>
            <dt>Automações ativas</dt>
            <dd>
              {sistema.data.automacoes_ativas ?? 0}
              {sistema.data.processos_ia ? ` · ${sistema.data.processos_ia} de IA rodando` : ''}
              {sistema.data.runner_disponivel === false ? ' · automações de IA indisponíveis (runner ausente)' : ''}
            </dd>
          </dl>
        )}
        <p className="texto-secundario">Tudo fica neste Mac. O ZapDesk não envia dados para nenhum servidor além do WhatsApp.</p>
      </div>

      <Aparencia />
    </section>
  );
}
