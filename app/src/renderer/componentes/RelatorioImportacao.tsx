// Relatório de importação (T077), usado em Leads e no Novo disparo: novos, já existentes (com data
// original), inválidos (com motivo) e duplicados no lote, com listas expansíveis.
import { CircleCheck, Clock, CopyX, TriangleAlert } from 'lucide-react';
import type { ReactNode } from 'react';

import type { MotivoInvalido, RelatorioImportacao as Relatorio } from '@zapdesk/cliente-motor';

import { dataCurta, numero, telefone } from '../util/formatar';

export const MOTIVOS_INVALIDO: Record<MotivoInvalido, string> = {
  vazio: 'vazio',
  formato_invalido: 'formato inválido',
  numero_invalido: 'número inválido',
};

/** Frases do relatório (spec: "32 números já estavam na sua base", "12 números inválidos"). */
export function frasesRelatorio(r: Relatorio) {
  const n = (valor: number, um: string, varios: string) => `${numero(valor)} ${valor === 1 ? um : varios}`;
  return {
    novos: n(r.total_novos, 'lead novo', 'leads novos'),
    jaExistentes: n(r.total_ja_existentes, 'número já estava na sua base', 'números já estavam na sua base'),
    invalidos: n(r.total_invalidos, 'número inválido', 'números inválidos'),
    duplicados: n(r.total_duplicados_no_lote, 'número repetido na lista', 'números repetidos na lista'),
  };
}

function Grupo(props: { icone: ReactNode; titulo: string; tipo: string; total: number; children: ReactNode }) {
  if (props.total === 0) {
    return (
      <div className={`relatorio-grupo ${props.tipo} zerado`}>
        <span className="relatorio-icone">{props.icone}</span>
        <span>{props.titulo}</span>
      </div>
    );
  }
  return (
    <details className={`relatorio-grupo ${props.tipo}`}>
      <summary>
        <span className="relatorio-icone">{props.icone}</span>
        <span>{props.titulo}</span>
        <small>ver lista</small>
      </summary>
      <div className="relatorio-lista">{props.children}</div>
    </details>
  );
}

const MAX_LINHAS = 500;

function Tabela(props: { cabecalho: string[]; linhas: ReactNode[][]; total: number }) {
  return (
    <table className="tabela compacta">
      <thead>
        <tr>
          {props.cabecalho.map((c) => (
            <th key={c}>{c}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {props.linhas.slice(0, MAX_LINHAS).map((l, i) => (
          <tr key={i}>
            {l.map((c, j) => (
              <td key={j}>{c}</td>
            ))}
          </tr>
        ))}
        {props.total > MAX_LINHAS ? (
          <tr>
            <td colSpan={props.cabecalho.length} className="texto-secundario">
              … e mais {numero(props.total - MAX_LINHAS)}
            </td>
          </tr>
        ) : null}
      </tbody>
    </table>
  );
}

export function RelatorioImportacao({ relatorio }: { relatorio: Relatorio }) {
  const f = frasesRelatorio(relatorio);
  return (
    <div className="relatorio-importacao" aria-label="Relatório da importação">
      <p className="relatorio-resumo">
        {numero(relatorio.total_linhas)} {relatorio.total_linhas === 1 ? 'linha lida' : 'linhas lidas'} ·{' '}
        {numero(relatorio.lead_ids.length)} {relatorio.lead_ids.length === 1 ? 'número válido' : 'números válidos'}
      </p>
      <Grupo icone={<CircleCheck size={18} />} titulo={f.novos} tipo="ok" total={relatorio.total_novos}>
        <Tabela
          cabecalho={['Linha', 'Telefone']}
          total={relatorio.novos.length}
          linhas={relatorio.novos.map((x) => [x.linha, telefone(x.telefone)])}
        />
      </Grupo>
      <Grupo icone={<Clock size={18} />} titulo={f.jaExistentes} tipo="info" total={relatorio.total_ja_existentes}>
        <Tabela
          cabecalho={['Linha', 'Telefone', 'Já existia desde', 'Campos preenchidos']}
          total={relatorio.ja_existentes.length}
          linhas={relatorio.ja_existentes.map((x) => [
            x.linha,
            telefone(x.telefone),
            dataCurta(x.importado_em),
            x.campos_preenchidos.length > 0 ? x.campos_preenchidos.join(', ') : '—',
          ])}
        />
      </Grupo>
      <Grupo icone={<TriangleAlert size={18} />} titulo={f.invalidos} tipo="erro" total={relatorio.total_invalidos}>
        <Tabela
          cabecalho={['Linha', 'Valor', 'Motivo']}
          total={relatorio.invalidos.length}
          linhas={relatorio.invalidos.map((x) => [x.linha, x.valor || '(vazio)', MOTIVOS_INVALIDO[x.motivo] ?? x.motivo])}
        />
      </Grupo>
      <Grupo icone={<CopyX size={18} />} titulo={f.duplicados} tipo="neutro" total={relatorio.total_duplicados_no_lote}>
        <Tabela
          cabecalho={['Linha', 'Telefone', 'Primeira vez na linha']}
          total={relatorio.duplicados_no_lote.length}
          linhas={relatorio.duplicados_no_lote.map((x) => [x.linha, telefone(x.telefone), x.primeira_linha])}
        />
      </Grupo>
    </div>
  );
}
