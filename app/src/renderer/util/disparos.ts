// Regras e textos de disparo usados pelas telas (espelham contracts/api-http.md › Disparos; o motor
// continua sendo quem valida de verdade).
import type { Disparo, EstadoDestinatario, EstadoDisparo, Janela, NovoDisparo, Ritmo } from '@zapdesk/cliente-motor';

import { dataHora, extrairVariaveis } from './formatar';

export const ROTULO_ESTADO_DISPARO: Record<EstadoDisparo, string> = {
  rascunho: 'Rascunho',
  agendado: 'Agendado',
  enviando: 'Enviando',
  fora_da_janela: 'Fora da janela',
  pausado: 'Pausado',
  concluido: 'Concluído',
  cancelado: 'Cancelado',
};

export const ROTULO_ESTADO_DESTINATARIO: Record<EstadoDestinatario, string> = {
  pendente: 'Pendente',
  enviando: 'Enviando',
  enviado: 'Enviado',
  entregue: 'Entregue',
  lido: 'Lido',
  falhou: 'Falhou',
  respondeu: 'Respondeu',
};

export const MOTIVOS_PAUSA: Record<string, string> = {
  usuario: 'Disparo pausado por você.',
  app_fechado: 'Disparo pausado porque o app foi fechado.',
  motor_reiniciado: 'Disparo pausado porque o WhatsApp parou e foi religado.',
  conta_desconectada: 'Conta desconectada. Reconecte para continuar.',
  conta_banida: 'O WhatsApp bloqueou este número.',
  falhas_seguidas: 'Disparo pausado após muitas falhas seguidas.',
};

/** "09:00" → "9h"; "09:30" → "9h30". */
export function horaJanela(hhmm: string): string {
  const [h, m] = hhmm.split(':');
  const hora = String(Number(h ?? 0));
  return m && m !== '00' ? `${hora}h${m}` : `${hora}h`;
}

export function textoJanela(janela: Janela): string | null {
  return janela ? `${horaJanela(janela.inicio)}–${horaJanela(janela.fim)}` : null;
}

/** Linha de estado do disparo (ex.: "Aguardando janela (9h–18h)", "Na fila", motivo da pausa). */
export function descreverEstado(d: Disparo, agora: Date = new Date()): string {
  if (d.na_fila) return 'Na fila';
  switch (d.estado) {
    case 'fora_da_janela':
      return `Aguardando janela (${textoJanela(d.janela) ?? ''})`;
    case 'pausado':
      return (d.motivo_pausa && MOTIVOS_PAUSA[d.motivo_pausa]) || 'Disparo pausado.';
    case 'agendado':
      if (d.inicio_em && new Date(d.inicio_em) > agora) return `Agendado para ${dataHora(d.inicio_em)}`;
      return 'Começando…';
    default:
      return ROTULO_ESTADO_DISPARO[d.estado];
  }
}

export function processados(d: Pick<Disparo, 'contadores'>): number {
  return d.contadores.total - d.contadores.pendente - d.contadores.enviando;
}

export function percentual(d: Pick<Disparo, 'contadores'>): number {
  return d.contadores.total > 0 ? Math.round((processados(d) / d.contadores.total) * 100) : 0;
}

export function podePausar(d: Disparo): boolean {
  return d.estado === 'agendado' || d.estado === 'enviando' || d.estado === 'fora_da_janela';
}

export function podeRetomar(d: Disparo): boolean {
  return d.estado === 'pausado';
}

export function podeCancelar(d: Disparo): boolean {
  return d.estado !== 'concluido' && d.estado !== 'cancelado';
}

/** Mesma regra do motor: só aviso, nunca bloqueia. */
export function ritmoAgressivo(r: Ritmo): boolean {
  return (
    r.intervalo_min_s < 10 ||
    (r.limite_por_hora !== null && r.limite_por_hora > 120) ||
    (r.limite_por_dia !== null && r.limite_por_dia > 1000) ||
    (r.limite_por_hora === null && r.limite_por_dia === null)
  );
}

// ---------------------------------------------------------------------------
// Formulário do assistente
// ---------------------------------------------------------------------------

export interface FormRitmo {
  intervaloMin: string;
  intervaloMax: string;
  limiteHora: string;
  limiteDia: string;
  pausaACada: string;
  pausaDuracaoMin: string;
}

export interface FormDisparo {
  contaId: string;
  nome: string;
  leadIds: string[];
  mensagem: string;
  arquivoId: string | null;
  ritmo: FormRitmo;
  agendar: boolean;
  inicioEm: string;
  usarJanela: boolean;
  janelaInicio: string;
  janelaFim: string;
  falhasSeguidas: string;
  valoresPadrao: Record<string, string>;
}

export function formInicial(contaId: string): FormDisparo {
  return {
    contaId,
    nome: '',
    leadIds: [],
    mensagem: '',
    arquivoId: null,
    ritmo: { intervaloMin: '30', intervaloMax: '90', limiteHora: '', limiteDia: '', pausaACada: '', pausaDuracaoMin: '' },
    agendar: false,
    inicioEm: '',
    usarJanela: false,
    janelaInicio: '09:00',
    janelaFim: '18:00',
    falhasSeguidas: '10',
    valoresPadrao: {},
  };
}

function inteiro(valor: string): number | null {
  const t = valor.trim();
  if (!t) return null;
  const n = Number(t);
  return Number.isInteger(n) ? n : Number.NaN;
}

export function montarRitmo(f: FormRitmo): Ritmo {
  const pausaMin = inteiro(f.pausaDuracaoMin);
  return {
    intervalo_min_s: inteiro(f.intervaloMin) ?? 0,
    intervalo_max_s: inteiro(f.intervaloMax) ?? 0,
    limite_por_hora: inteiro(f.limiteHora),
    limite_por_dia: inteiro(f.limiteDia),
    pausa_a_cada: inteiro(f.pausaACada),
    pausa_duracao_s: pausaMin === null ? null : pausaMin * 60,
  };
}

/** `datetime-local` ("2026-09-28T09:00") → RFC 3339 com o fuso local. */
export function dataLocalParaRfc3339(valor: string): string | null {
  if (!valor) return null;
  const d = new Date(valor);
  if (Number.isNaN(d.getTime())) return null;
  const d2 = (n: number) => String(Math.trunc(Math.abs(n))).padStart(2, '0');
  const off = -d.getTimezoneOffset();
  return (
    `${d.getFullYear()}-${d2(d.getMonth() + 1)}-${d2(d.getDate())}T${d2(d.getHours())}:${d2(d.getMinutes())}:00` +
    `${off >= 0 ? '+' : '-'}${d2(off / 60)}:${d2(off % 60)}`
  );
}

const HHMM = /^([01]\d|2[0-3]):[0-5]\d$/;
export const LIMITE_MENSAGEM = 4096;

/** Erros por campo (mesmas restrições do motor). Vazio = ok. */
export function validarRitmoForm(f: FormDisparo): Record<string, string> {
  const erros: Record<string, string> = {};
  const r = montarRitmo(f.ritmo);
  if (!Number.isInteger(r.intervalo_min_s) || r.intervalo_min_s < 1) erros['intervalo_min_s'] = 'O intervalo mínimo deve ser de pelo menos 1 segundo.';
  if (!Number.isInteger(r.intervalo_max_s) || r.intervalo_max_s < r.intervalo_min_s)
    erros['intervalo_max_s'] = 'O intervalo máximo deve ser maior ou igual ao mínimo.';
  for (const [campo, valor, nome] of [
    ['limite_por_hora', r.limite_por_hora, 'O limite por hora'],
    ['limite_por_dia', r.limite_por_dia, 'O limite por dia'],
    ['pausa_a_cada', r.pausa_a_cada, 'A pausa a cada N mensagens'],
    ['pausa_duracao_s', r.pausa_duracao_s, 'A duração da pausa'],
  ] as const) {
    if (valor !== null && (!Number.isInteger(valor) || valor <= 0)) erros[campo] = `${nome} deve ser maior que zero.`;
  }
  if ((r.pausa_a_cada === null) !== (r.pausa_duracao_s === null)) erros['pausa_a_cada'] = 'Preencha "pausar a cada" e "por quanto tempo" juntos.';
  if (f.usarJanela) {
    if (!HHMM.test(f.janelaInicio) || !HHMM.test(f.janelaFim)) erros['janela'] = 'Use horários no formato HH:MM.';
    else if (f.janelaInicio === f.janelaFim) erros['janela'] = 'O início e o fim da janela precisam ser diferentes.';
  }
  const falhas = inteiro(f.falhasSeguidas);
  if (falhas === null || !Number.isInteger(falhas) || falhas < 0) erros['falhas_seguidas_max'] = 'Use 0 ou mais (0 desliga).';
  if (f.agendar && !dataLocalParaRfc3339(f.inicioEm)) erros['inicio_em'] = 'Escolha a data e a hora de início.';
  return erros;
}

export function validarMensagemForm(f: FormDisparo): string | null {
  if (!f.mensagem.trim()) return 'Escreva a mensagem.';
  if (f.mensagem.length > LIMITE_MENSAGEM) return `A mensagem passou de ${LIMITE_MENSAGEM.toLocaleString('pt-BR')} caracteres.`;
  return null;
}

export function montarNovoDisparo(f: FormDisparo, iniciar: boolean): NovoDisparo {
  const valores = Object.fromEntries(Object.entries(f.valoresPadrao).filter(([, v]) => v.trim() !== ''));
  const falhas = inteiro(f.falhasSeguidas);
  return {
    conta_id: f.contaId,
    ...(f.nome.trim() ? { nome: f.nome.trim() } : {}),
    mensagem: f.mensagem,
    arquivo_id: f.arquivoId,
    destinatarios: { lead_ids: f.leadIds },
    ritmo: montarRitmo(f.ritmo),
    inicio_em: f.agendar ? dataLocalParaRfc3339(f.inicioEm) : null,
    janela: f.usarJanela ? { inicio: f.janelaInicio, fim: f.janelaFim } : null,
    falhas_seguidas_max: falhas !== null && Number.isInteger(falhas) ? falhas : 10,
    valores_padrao: valores,
    iniciar,
    origem: 'app',
  };
}

/** Agrupa `faltando` por variável: `{nome: [{linha, telefone}]}`. */
export function faltandoPorVariavel(faltando: { destinatario_linha: number; telefone: string; variaveis: string[] }[]) {
  const mapa = new Map<string, { linha: number; telefone: string }[]>();
  for (const f of faltando) {
    for (const v of f.variaveis) {
      const lista = mapa.get(v) ?? [];
      lista.push({ linha: f.destinatario_linha, telefone: f.telefone });
      mapa.set(v, lista);
    }
  }
  return mapa;
}

export { extrairVariaveis };
