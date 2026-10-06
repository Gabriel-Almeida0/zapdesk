// Categorias do lead e o que fazer com cada uma. Ajuste os nomes do funil, das etapas e das
// etiquetas para os que existem no seu ZapDesk (sem diferenciar maiúsculas e acentos).
export const FUNIL = 'Vendas';

export const CATEGORIAS = {
  quente: 'Quer comprar agora, pediu orçamento, forma de pagamento ou entrega.',
  morno: 'Tem interesse, mas ainda está pesquisando ou tirando dúvidas.',
  frio: 'Sem interesse de compra, fora do perfil ou só cumprimentou.',
} as const;

export type Categoria = keyof typeof CATEGORIAS;

export const ACOES: Record<Categoria, { etapa: string; etiqueta: string }> = {
  quente: { etapa: 'Qualificando', etiqueta: 'Quente' },
  morno: { etapa: 'Novo', etiqueta: 'Morno' },
  frio: { etapa: 'Novo', etiqueta: 'Frio' },
};
