// Validação/prévia no motor (`POST /v1/disparos/validar`) com atraso, para mostrar a mensagem
// resolvida com uma linha real, a estimativa de término e as variáveis faltando.
import { useQuery } from '@tanstack/react-query';

import { useCliente } from '../../api/motor';
import { montarNovoDisparo, type FormDisparo } from '../../util/disparos';
import { useAtraso } from '../Conversas';

export function useValidacaoDisparo(form: FormDisparo, habilitado: boolean) {
  const cliente = useCliente();
  const corpo = useAtraso(JSON.stringify(montarNovoDisparo(form, false)), 450);
  return useQuery({
    queryKey: ['validar-disparo', corpo],
    enabled: habilitado && form.leadIds.length > 0 && Boolean(form.contaId),
    queryFn: () => cliente.validarDisparo(JSON.parse(corpo) as ReturnType<typeof montarNovoDisparo>),
    staleTime: 10_000,
    placeholderData: (anterior) => anterior,
  });
}
