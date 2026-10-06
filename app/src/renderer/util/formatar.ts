// Formatação pt-BR (datas, telefones, tamanhos) e utilidades de texto da interface.
import { ErroMotor, type TipoMidiaArquivo } from '@zapdesk/cliente-motor';

const LOCALE = 'pt-BR';

function data(iso: string | Date): Date {
  return iso instanceof Date ? iso : new Date(iso);
}

function mesmoDia(a: Date, b: Date): boolean {
  return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
}

function diasAtras(d: Date, agora: Date): number {
  const inicio = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  return Math.round((inicio(agora) - inicio(d)) / 86_400_000);
}

export function hora(iso: string | Date): string {
  return data(iso).toLocaleTimeString(LOCALE, { hour: '2-digit', minute: '2-digit' });
}

/** Lista de conversas: "14:05", "Ontem", "segunda-feira", "27/09/2026". */
export function rotuloDataLista(iso: string | null, agora: Date = new Date()): string {
  if (!iso) return '';
  const d = data(iso);
  const dias = diasAtras(d, agora);
  if (dias <= 0) return hora(d);
  if (dias === 1) return 'Ontem';
  if (dias < 7) return d.toLocaleDateString(LOCALE, { weekday: 'long' });
  return d.toLocaleDateString(LOCALE, { day: '2-digit', month: '2-digit', year: 'numeric' });
}

/** Separador de dia no chat: "Hoje", "Ontem", "segunda-feira", "27 de setembro de 2026". */
export function rotuloDia(iso: string, agora: Date = new Date()): string {
  const d = data(iso);
  const dias = diasAtras(d, agora);
  if (dias <= 0) return 'Hoje';
  if (dias === 1) return 'Ontem';
  if (dias < 7) return d.toLocaleDateString(LOCALE, { weekday: 'long' });
  return d.toLocaleDateString(LOCALE, { day: 'numeric', month: 'long', year: 'numeric' });
}

export function chaveDia(iso: string): string {
  const d = data(iso);
  return `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;
}

export function dataHora(iso: string | null | undefined): string {
  if (!iso) return '—';
  const d = data(iso);
  return `${d.toLocaleDateString(LOCALE, { day: '2-digit', month: '2-digit', year: 'numeric' })} ${hora(d)}`;
}

export function dataCurta(iso: string | null | undefined): string {
  if (!iso) return '—';
  return data(iso).toLocaleDateString(LOCALE, { day: '2-digit', month: '2-digit', year: 'numeric' });
}

export function ehHoje(iso: string, agora: Date = new Date()): boolean {
  return mesmoDia(data(iso), agora);
}

/** "+5511999990000" → "+55 11 99999-0000". Outros países: mantém o E.164. */
export function telefone(e164: string | null | undefined): string {
  if (!e164) return '';
  const m = /^\+55(\d{2})(\d{4,5})(\d{4})$/.exec(e164);
  if (m) return `+55 ${m[1]} ${m[2]}-${m[3]}`;
  return e164;
}

/** Nome para exibir: quando o "nome" é só o número (contato sem nome), mostra o telefone formatado. */
export function nomeExibicao(nome: string | null | undefined, tel?: string | null): string {
  const n = (nome ?? '').trim();
  if (n && !/^\+?[\d\s()-]+$/.test(n)) return n;
  return telefone(tel ?? n) || n;
}

export function tamanho(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const kb = bytes / 1024;
  if (kb < 1024) return `${kb.toLocaleString(LOCALE, { maximumFractionDigits: 0 })} KB`;
  const mb = kb / 1024;
  return `${mb.toLocaleString(LOCALE, { maximumFractionDigits: 1 })} MB`;
}

export function duracao(segundos: number | null | undefined): string {
  if (segundos === null || segundos === undefined || !Number.isFinite(segundos)) return '0:00';
  const s = Math.max(0, Math.round(segundos));
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}

export function numero(n: number): string {
  return n.toLocaleString(LOCALE);
}

export function plural(n: number, singular: string, pluralTexto: string): string {
  return `${numero(n)} ${n === 1 ? singular : pluralTexto}`;
}

export function iniciais(nome: string | null | undefined): string {
  const partes = (nome ?? '').replace(/[^\p{L}\p{N} ]/gu, ' ').trim().split(/\s+/).filter(Boolean);
  if (partes.length === 0) return '#';
  const primeira = partes[0]?.[0] ?? '';
  const ultima = partes.length > 1 ? (partes[partes.length - 1]?.[0] ?? '') : '';
  return (primeira + ultima).toUpperCase();
}

/** Cor estável (matiz) a partir de um texto — avatares e nomes em grupos. */
export function matiz(texto: string): number {
  let h = 0;
  for (let i = 0; i < texto.length; i++) h = (h * 31 + texto.charCodeAt(i)) % 360;
  return h;
}

/** Mensagem pt-BR de um erro qualquer (erros do motor já vêm prontas para exibir). */
export function textoErro(erro: unknown): string {
  if (erro instanceof ErroMotor) {
    const campos = erro.detalhes.campos;
    if (erro.codigo === 'validacao' && campos && Object.keys(campos).length > 0) {
      return Object.values(campos).join(' ');
    }
    return erro.mensagem;
  }
  if (erro instanceof Error) return erro.message;
  return 'Algo deu errado.';
}

// ---------------------------------------------------------------------------
// Variáveis `{nome}` (mesma regra do motor: `{{`/`}}` são escapes; chave normalizada)
// ---------------------------------------------------------------------------

export function normalizarChave(chave: string): string {
  return chave
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .trim()
    .toLowerCase()
    .replace(/\s+/g, '_');
}

export function extrairVariaveis(texto: string): string[] {
  const semEscapes = texto.replace(/\{\{|\}\}/g, '');
  const vistas = new Set<string>();
  for (const m of semEscapes.matchAll(/\{([^{}\n]+)\}/g)) {
    const chave = normalizarChave(m[1] ?? '');
    if (chave) vistas.add(chave);
  }
  return [...vistas];
}

// ---------------------------------------------------------------------------
// Limites de anexo (contracts/api-http.md › Arquivos)
// ---------------------------------------------------------------------------

const MB = 1024 * 1024;
export const LIMITES_ANEXO: Record<TipoMidiaArquivo, number> = {
  imagem: 16 * MB,
  video: 16 * MB,
  audio: 16 * MB,
  documento: 100 * MB,
  figurinha: 1 * MB,
};

const NOMES_TIPO: Record<TipoMidiaArquivo, string> = {
  imagem: 'imagem',
  video: 'vídeo',
  audio: 'áudio',
  documento: 'documento',
  figurinha: 'figurinha',
};

export function tipoMidiaDoArquivo(mimetype: string): TipoMidiaArquivo {
  if (mimetype === 'image/webp') return 'figurinha';
  if (mimetype.startsWith('image/')) return 'imagem';
  if (mimetype.startsWith('video/')) return 'video';
  if (mimetype.startsWith('audio/')) return 'audio';
  return 'documento';
}

/** `null` se couber; senão a mensagem de recusa. */
export function checarLimiteAnexo(arquivo: { size: number; type: string }, comoDocumento = false): string | null {
  const tipo = comoDocumento ? 'documento' : tipoMidiaDoArquivo(arquivo.type);
  const limite = LIMITES_ANEXO[tipo];
  if (arquivo.size <= limite) return null;
  return `Arquivo grande demais: o limite para ${NOMES_TIPO[tipo]} é ${Math.round(limite / MB)} MB.`;
}
