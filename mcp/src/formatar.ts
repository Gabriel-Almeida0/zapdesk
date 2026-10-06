// Formatação das respostas das ferramentas: `structuredContent` + resumo curto em português, e
// conversão de erros (do motor, do app fechado ou da própria ferramenta) em `isError: true` com
// texto útil para a IA decidir o próximo passo. Nunca lança.
import type { CallToolResult } from '@modelcontextprotocol/server';
import {
  type ErroCompilacao,
  type ErroDefinicao,
  ErroMotor,
  MENSAGEM_MOTOR_FORA,
  type VariavelFaltando,
} from '@zapdesk/cliente-motor';

import { ErroAppFechado } from './garantir-motor.js';

/** Erro de uso da ferramenta (entrada incoerente, conta ambígua…); vira `isError` com a mensagem. */
export class ErroFerramenta extends Error {
  constructor(mensagem: string) {
    super(mensagem);
    this.name = 'ErroFerramenta';
  }
}

/** Máximo de linhas listadas em textos (o resto vai resumido em "… e mais N"). */
export const MAX_LINHAS = 50;

export function responder(estruturado: Record<string, unknown>, texto: string): CallToolResult {
  return { content: [{ type: 'text', text: texto }], structuredContent: estruturado };
}

function falhar(texto: string): CallToolResult {
  return { content: [{ type: 'text', text: texto }], isError: true };
}

/** "2026-09-27T20:10:00-03:00" → "27/09/2026 20:10" (no fuso do próprio texto). */
export function formatarData(iso: string | null | undefined): string {
  if (!iso) return '—';
  const partes = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/.exec(iso);
  if (!partes) return iso;
  const [, ano, mes, dia, hora, minuto] = partes;
  return `${dia}/${mes}/${ano} ${hora}:${minuto}`;
}

/** Lista em linhas "- …", limitada a `MAX_LINHAS`. */
export function listar<T>(itens: readonly T[], linha: (item: T) => string, maximo = MAX_LINHAS): string {
  const visiveis = itens.slice(0, maximo).map((item) => `- ${linha(item)}`);
  if (itens.length > maximo) visiveis.push(`- … e mais ${itens.length - maximo}`);
  return visiveis.join('\n');
}

export function plural(n: number, singular: string, pluralTexto: string): string {
  return `${n} ${n === 1 ? singular : pluralTexto}`;
}

/** Linha final sobre paginação. */
export function textoPaginacao(proximoCursor: string | null): string {
  return proximoCursor
    ? `Há mais resultados: chame de novo com cursor "${proximoCursor}".`
    : 'Não há mais resultados.';
}

/** "3 destinatários sem {nome}, 1 sem {empresa}" + lista das linhas. */
export function textoVariaveisFaltando(faltando: readonly VariavelFaltando[], total?: number): string {
  const contagem = new Map<string, number>();
  for (const item of faltando) {
    for (const variavel of item.variaveis) contagem.set(variavel, (contagem.get(variavel) ?? 0) + 1);
  }
  const resumo = [...contagem.entries()]
    .sort((a, b) => b[1] - a[1])
    .map(([variavel, n], i) =>
      i === 0 ? `${plural(n, 'destinatário', 'destinatários')} sem {${variavel}}` : `${n} sem {${variavel}}`,
    )
    .join(', ');
  const totalLinhas = total ?? faltando.length;
  const linhas = listar(
    faltando,
    (item) => `linha ${item.destinatario_linha}: ${item.telefone} — falta ${item.variaveis.map((v) => `{${v}}`).join(', ')}`,
  );
  const extra = totalLinhas > faltando.length ? `\n(${totalLinhas} linhas incompletas no total)` : '';
  return `${resumo || `${totalLinhas} destinatários com variáveis faltando`}.\n${linhas}${extra}`;
}

/** "definicao.acoes[2].etapa_id (ação a2): mensagem" — um por linha. */
export function textoErrosDefinicao(erros: readonly ErroDefinicao[]): string {
  return listar(erros, (e) => {
    const onde = [e.no_id ? `nó ${e.no_id}` : null, e.acao_id ? `ação ${e.acao_id}` : null].filter(Boolean).join(', ');
    return `${e.caminho || '(raiz)'}${onde ? ` (${onde})` : ''}: ${e.mensagem}`;
  });
}

/** "index.ts:3:10 [sintaxe] mensagem" — um por linha, no formato que a IA usa para corrigir. */
export function textoErrosCompilacao(erros: readonly ErroCompilacao[]): string {
  return listar(erros, (e) => `${e.arquivo}:${e.linha}:${e.coluna} [${e.tipo}] ${e.mensagem}`);
}

function textoErroMotor(erro: ErroMotor): string {
  const d = erro.detalhes;
  switch (erro.codigo) {
    case 'validacao': {
      const campos = d.campos ? Object.entries(d.campos) : [];
      return campos.length > 0
        ? `${erro.mensagem}\nCampos com problema:\n${listar(campos, ([campo, msg]) => `${campo}: ${msg}`)}`
        : erro.mensagem;
    }
    case 'variaveis_faltando':
      return (
        `${erro.mensagem}\n${textoVariaveisFaltando(d.faltando ?? [], d.total)}\n` +
        'Nada foi enviado. Informe `valores_padrao` para essas variáveis (ex.: {"nome": "tudo bem"}) ' +
        'ou complete os dados dos leads e tente de novo.'
      );
    case 'transicao_invalida':
      return d.estado_atual ? `${erro.mensagem} (estado atual: ${d.estado_atual})` : erro.mensagem;
    case 'conta_indisponivel':
      return `${erro.mensagem}\nA conta precisa estar conectada; reconecte-a pela interface do ZapDesk.`;
    case 'anexo_grande_demais':
      return d.limite_bytes
        ? `${erro.mensagem} (limite para ${d.tipo_midia ?? 'esse tipo'}: ${Math.round(d.limite_bytes / 1_048_576)} MB)`
        : erro.mensagem;
    case 'whatsapp_erro':
      return d.motivo ? `${erro.mensagem} (motivo: ${d.motivo})` : erro.mensagem;
    case 'nao_autorizado':
      return `${erro.mensagem}\nO token do runtime.json não foi aceito; feche e abra o ZapDesk e tente de novo.`;
    case 'definicao_invalida': {
      const erros = (d.erros ?? []) as ErroDefinicao[];
      return erros.length > 0
        ? `${erro.mensagem}\nCorrija estes campos (veja ver_formatos_automacao):\n${textoErrosDefinicao(erros)}`
        : erro.mensagem;
    }
    case 'compilacao_falhou': {
      const erros = (d.erros ?? []) as ErroCompilacao[];
      return (
        `${erro.mensagem}\n` +
        (erros.length > 0 ? `${textoErrosCompilacao(erros)}\n` : '') +
        'Corrija os arquivos (escrever_arquivo_automacao), rode compilar_automacao até "Compilou sem erros" e tente de novo.'
      );
    }
    case 'conflito':
      return d.hash_atual
        ? `${erro.mensagem}\nO arquivo mudou desde a sua leitura (hash atual: ${d.hash_atual}). Leia de novo com ler_arquivo_automacao, reaplique a mudança e escreva com o hash novo.`
        : erro.mensagem;
    case 'runner_indisponivel':
      return `${erro.mensagem}\nAutomações de IA não podem executar neste ZapDesk (runner ausente). Fluxos e chatbots continuam funcionando.`;
    case 'ia_nao_configurada':
      return `${erro.mensagem}\nO usuário precisa configurar a chave da Anthropic em Ajustes → IA no app (segredos só podem ser definidos pela interface). Para testar sem chave, use ia_simulada: true.`;
    case 'ia_erro':
      return `${erro.mensagem}${d.status ? ` (status ${d.status}${d.request_id ? `, request_id ${d.request_id}` : ''})` : ''}`;
    case 'interno':
      if (erro.status === 0) {
        return `${MENSAGEM_MOTOR_FORA} O app pode ter sido fechado agora há pouco; tente de novo.`;
      }
      return erro.mensagem;
    default:
      return erro.mensagem;
  }
}

/** Converte qualquer erro em resultado `isError: true` com texto em português. */
export function responderErro(erro: unknown): CallToolResult {
  if (erro instanceof ErroMotor) return falhar(textoErroMotor(erro));
  if (erro instanceof ErroAppFechado) {
    return falhar(erro.detalhe ? `${erro.message} (detalhe: ${erro.detalhe})` : erro.message);
  }
  if (erro instanceof ErroFerramenta) return falhar(erro.message);
  const mensagem = erro instanceof Error ? erro.message : String(erro);
  return falhar(`Erro inesperado no servidor MCP do ZapDesk: ${mensagem}`);
}
