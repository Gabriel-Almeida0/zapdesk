import type { EsquemaJson } from '@zapdesk/automacao';

// Dados que a IA procura na conversa. Campos não mencionados voltam como null e não são gravados.
export const ESQUEMA: EsquemaJson = {
  type: 'object',
  properties: {
    nome: { type: ['string', 'null'], description: 'Nome da pessoa, se ela disse.' },
    email: { type: ['string', 'null'], description: 'E-mail informado.' },
    cidade: { type: ['string', 'null'], description: 'Cidade informada.' },
    interesse: { type: ['string', 'null'], description: 'Produto ou serviço de interesse, em poucas palavras.' },
  },
  required: ['nome', 'email', 'cidade', 'interesse'],
};

export interface Dados {
  nome: string | null;
  email: string | null;
  cidade: string | null;
  interesse: string | null;
}
