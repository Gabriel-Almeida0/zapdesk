// `historicoParaIA` (puro): converte o histórico da conversa (mais recentes primeiro, como
// `ctx.conversa.historico`) em mensagens para a Claude, em ordem cronológica.
import type { Mensagem, MensagemIA } from '@zapdesk/automacao';

const ROTULOS_MIDIA: Record<string, string> = {
  imagem: '[imagem]',
  video: '[vídeo]',
  audio: '[áudio]',
  documento: '[documento]',
  figurinha: '[figurinha]',
};

/**
 * Regras (contrato da SDK):
 * - contato → `user`; eu/automação (`deMim`) → `assistant`;
 * - mensagens seguidas do mesmo lado são unidas com quebra de linha;
 * - mídia vira "[imagem]", "[áudio]"… (com a legenda depois, se houver);
 * - mensagens de sistema e textos vazios são ignorados.
 * Decisão: mensagens `assistant` no começo são descartadas, para a lista começar pelo contato
 * (formato esperado pela Messages API).
 */
export function historicoParaIA(maisRecentesPrimeiro: readonly Mensagem[]): MensagemIA[] {
  const cronologico = [...maisRecentesPrimeiro].reverse().sort(ordenar);
  const saida: MensagemIA[] = [];
  for (const m of cronologico) {
    const texto = textoDe(m);
    if (texto === null) continue;
    const papel: MensagemIA['papel'] = m.deMim ? 'assistant' : 'user';
    if (saida.length === 0 && papel === 'assistant') continue;
    const ultima = saida[saida.length - 1];
    if (ultima && ultima.papel === papel) ultima.texto = `${ultima.texto}\n${texto}`;
    else saida.push({ papel, texto });
  }
  return saida;
}

function textoDe(m: Mensagem): string | null {
  if (m.tipo === 'sistema') return null;
  const texto = (m.texto ?? '').trim();
  const rotulo = ROTULOS_MIDIA[m.tipo];
  if (rotulo) return texto ? `${rotulo} ${texto}` : rotulo;
  return texto.length > 0 ? texto : null;
}

function ordenar(a: Mensagem, b: Mensagem): number {
  const ta = Date.parse(a.enviadaEm);
  const tb = Date.parse(b.enviadaEm);
  if (Number.isNaN(ta) || Number.isNaN(tb) || ta === tb) return 0; // sort estável: mantém a ordem
  return ta - tb;
}
