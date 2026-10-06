// Árvore de arquivos do projeto (T105): pastas pelos caminhos, criar, renomear e excluir
// (`automacao.json` e a entrada não podem ser excluídos).
import { FileCode, Braces, FilePlus, FileText, Folder, Pencil, Trash } from 'lucide-react';
import { useState, type ReactNode } from 'react';

import { BotaoIcone } from '../../componentes/BotaoIcone';
import { FaixaAviso } from '../../componentes/FaixaAviso';
import { Confirmar, Modal } from '../../componentes/Modal';
import { textoErro } from '../../util/formatar';

export const CAMINHO_VALIDO = /^(?!.*\/\/)(?!\/)(?!.*\/$)[A-Za-z0-9_.\-/]{1,300}$/;
const EXTENSOES = ['.ts', '.json', '.md', '.txt'];

/** Regras de caminho de contracts/api-http.md (mensagem pt-BR ou null). */
export function validarCaminho(caminho: string): string | null {
  if (!CAMINHO_VALIDO.test(caminho)) return 'Use letras, números, _ . - e "/" para pastas.';
  const segmentos = caminho.split('/');
  if (segmentos.length > 4) return 'No máximo 3 níveis de pasta.';
  if (segmentos.some((s) => s === '..' || s === '.' || s.startsWith('.') || s.length > 64)) return 'Segmentos não podem começar com "." nem passar de 64 caracteres.';
  if (!EXTENSOES.some((e) => caminho.endsWith(e))) return 'Extensões permitidas: .ts, .json, .md, .txt.';
  return null;
}

function icone(caminho: string): ReactNode {
  if (caminho.endsWith('.ts')) return <FileCode size={15} aria-hidden="true" />;
  if (caminho.endsWith('.json')) return <Braces size={15} aria-hidden="true" />;
  return <FileText size={15} aria-hidden="true" />;
}

interface ItemArvore {
  caminho: string;
  nome: string;
  nivel: number;
  pasta: boolean;
}

/** Lista plana (pastas antes, ordem alfabética) com nível de indentação. */
export function montarArvore(caminhos: readonly string[]): ItemArvore[] {
  const itens: ItemArvore[] = [];
  const pastas = new Set<string>();
  const ordenados = [...caminhos].sort((a, b) => {
    const pa = a.split('/');
    const pb = b.split('/');
    for (let i = 0; i < Math.min(pa.length, pb.length); i++) {
      const fimA = i === pa.length - 1;
      const fimB = i === pb.length - 1;
      if (pa[i] !== pb[i]) {
        if (fimA !== fimB) return fimA ? 1 : -1;
        return (pa[i] ?? '').localeCompare(pb[i] ?? '');
      }
    }
    return pa.length - pb.length;
  });
  for (const c of ordenados) {
    const partes = c.split('/');
    for (let i = 0; i < partes.length - 1; i++) {
      const pasta = partes.slice(0, i + 1).join('/');
      if (!pastas.has(pasta)) {
        pastas.add(pasta);
        itens.push({ caminho: pasta, nome: partes[i] ?? '', nivel: i, pasta: true });
      }
    }
    itens.push({ caminho: c, nome: partes.at(-1) ?? c, nivel: partes.length - 1, pasta: false });
  }
  return itens;
}

function DialogoCaminho(props: { titulo: string; inicial: string; confirmar: string; aoConfirmar: (caminho: string) => Promise<void>; aoFechar: () => void }) {
  const [caminho, setCaminho] = useState(props.inicial);
  const [erro, setErro] = useState<string | null>(null);
  const [ocupado, setOcupado] = useState(false);
  const invalido = validarCaminho(caminho.trim());
  return (
    <Modal titulo={props.titulo} aoFechar={props.aoFechar}>
      <form
        className="formulario"
        onSubmit={(e) => {
          e.preventDefault();
          if (invalido) return;
          setOcupado(true);
          props
            .aoConfirmar(caminho.trim())
            .then(props.aoFechar)
            .catch((x: unknown) => setErro(textoErro(x)))
            .finally(() => setOcupado(false));
        }}
      >
        <label className="campo">
          <span>Caminho</span>
          <input className="campo-codigo" value={caminho} onChange={(e) => setCaminho(e.target.value)} placeholder="lib/util.ts" />
          <small className="texto-secundario">Pastas com "/", ex.: prompts/boas-vindas.md</small>
        </label>
        {caminho && invalido ? <p className="erro-campo">{invalido}</p> : null}
        {erro ? <FaixaAviso tipo="erro">{erro}</FaixaAviso> : null}
        <div className="acoes-formulario">
          <button type="button" className="botao secundario" onClick={props.aoFechar}>
            Cancelar
          </button>
          <button type="submit" className="botao" disabled={Boolean(invalido) || ocupado}>
            {props.confirmar}
          </button>
        </div>
      </form>
    </Modal>
  );
}

export interface PropsArvore {
  caminhos: string[];
  ativo: string | null;
  sujos: ReadonlySet<string>;
  comErro: ReadonlySet<string>;
  protegidos: ReadonlySet<string>;
  aoAbrir: (caminho: string) => void;
  aoCriar: (caminho: string) => Promise<void>;
  aoRenomear: (de: string, para: string) => Promise<void>;
  aoExcluir: (caminho: string) => Promise<void>;
}

export function ArvoreArquivos(props: PropsArvore) {
  const [criando, setCriando] = useState(false);
  const [renomeando, setRenomeando] = useState<string | null>(null);
  const [excluindo, setExcluindo] = useState<string | null>(null);
  const [erro, setErro] = useState<string | null>(null);
  const itens = montarArvore(props.caminhos);
  return (
    <nav className="arvore-arquivos" aria-label="Arquivos do projeto">
      <div className="arvore-cabecalho">
        <span>Arquivos</span>
        <BotaoIcone rotulo="Novo arquivo" className="pequeno" disabled={props.caminhos.length >= 50} onClick={() => setCriando(true)}>
          <FilePlus size={15} />
        </BotaoIcone>
      </div>
      <ul role="tree">
        {itens.map((i) =>
          i.pasta ? (
            <li key={`pasta:${i.caminho}`} className="arvore-pasta" style={{ paddingLeft: 8 + i.nivel * 12 }} role="treeitem" aria-expanded="true">
              <Folder size={15} aria-hidden="true" /> {i.nome}
            </li>
          ) : (
            <li
              key={i.caminho}
              role="treeitem"
              aria-selected={props.ativo === i.caminho}
              className={`arvore-arquivo${props.ativo === i.caminho ? ' ativo' : ''}${props.comErro.has(i.caminho) ? ' com-erro' : ''}`}
            >
              <button type="button" style={{ paddingLeft: 8 + i.nivel * 12 }} onClick={() => props.aoAbrir(i.caminho)}>
                {icone(i.caminho)}
                <span className="arvore-nome">{i.nome}</span>
                {props.sujos.has(i.caminho) ? <span className="ponto-sujo" aria-label="não salvo" /> : null}
              </button>
              <span className="arvore-acoes">
                <BotaoIcone rotulo={`Renomear ${i.caminho}`} className="pequeno" disabled={props.protegidos.has(i.caminho)} onClick={() => setRenomeando(i.caminho)}>
                  <Pencil size={13} />
                </BotaoIcone>
                <BotaoIcone rotulo={`Excluir ${i.caminho}`} className="pequeno" disabled={props.protegidos.has(i.caminho)} onClick={() => setExcluindo(i.caminho)}>
                  <Trash size={13} />
                </BotaoIcone>
              </span>
            </li>
          ),
        )}
      </ul>
      {erro ? <FaixaAviso tipo="erro">{erro}</FaixaAviso> : null}
      {criando ? (
        <DialogoCaminho
          titulo="Novo arquivo"
          inicial=""
          confirmar="Criar"
          aoConfirmar={async (c) => {
            await props.aoCriar(c);
            props.aoAbrir(c);
          }}
          aoFechar={() => setCriando(false)}
        />
      ) : null}
      {renomeando ? (
        <DialogoCaminho
          titulo={`Renomear ${renomeando}`}
          inicial={renomeando}
          confirmar="Renomear"
          aoConfirmar={(c) => props.aoRenomear(renomeando, c)}
          aoFechar={() => setRenomeando(null)}
        />
      ) : null}
      {excluindo ? (
        <Confirmar
          titulo={`Excluir ${excluindo}?`}
          texto="O arquivo é apagado da pasta do projeto."
          confirmar="Excluir"
          perigo
          aoConfirmar={() => {
            const alvo = excluindo;
            setExcluindo(null);
            props.aoExcluir(alvo).catch((x: unknown) => setErro(textoErro(x)));
          }}
          aoFechar={() => setExcluindo(null)}
        />
      ) : null}
    </nav>
  );
}
