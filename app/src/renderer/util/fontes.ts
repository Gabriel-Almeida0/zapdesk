// 003: espera as 3 famílias empacotadas antes do primeiro render (research R4): sem troca visível
// de fonte nem deslocamento de layout. Nunca bloqueia mais que `limiteMs` (fonte ausente ou
// ambiente sem `document.fonts`, como nos testes, segue na hora).
export const FAMILIAS = ['Bricolage Grotesque Variable', 'Instrument Sans Variable', 'JetBrains Mono Variable'] as const;

export async function aguardarFontes(limiteMs = 500): Promise<void> {
  const fontes = typeof document !== 'undefined' ? document.fonts : undefined;
  if (!fontes?.load) return;
  const carregar = Promise.all([
    fontes.load(`400 14px '${FAMILIAS[1]}'`),
    fontes.load(`600 14px '${FAMILIAS[1]}'`),
    fontes.load(`650 20px '${FAMILIAS[0]}'`),
    fontes.load(`400 12px '${FAMILIAS[2]}'`),
  ]).then(() => undefined);
  let relogio: ReturnType<typeof setTimeout> | undefined;
  const limite = new Promise<void>((r) => {
    relogio = setTimeout(r, limiteMs);
  });
  try {
    await Promise.race([carregar.catch(() => undefined), limite]);
  } finally {
    clearTimeout(relogio);
  }
}
