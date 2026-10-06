// Nova conversa por número (T060): o motor normaliza o telefone e checa se tem WhatsApp.
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { ErroMotor, type Id } from '@zapdesk/cliente-motor';

import { chaves } from '../api/chaves';
import { useCliente } from '../api/motor';
import { textoErro } from '../util/formatar';
import { FaixaAviso } from './FaixaAviso';
import { Modal } from './Modal';

export const SEM_WHATSAPP = 'Este número não tem WhatsApp';

export function NovaConversa(props: { contaId: Id; aoFechar: () => void; aoAbrir: (conversaId: Id) => void }) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const [telefone, setTelefone] = useState('');
  const abrir = useMutation({
    mutationFn: () => cliente.abrirConversa(props.contaId, telefone.trim()),
    onSuccess: (conversa) => {
      qc.setQueryData(chaves.conversa(conversa.id), conversa);
      void qc.invalidateQueries({ queryKey: chaves.conversas(props.contaId) });
      props.aoAbrir(conversa.id);
    },
  });

  const erro = abrir.error;
  const mensagemErro =
    erro instanceof ErroMotor && erro.codigo === 'sem_whatsapp' ? SEM_WHATSAPP : erro ? textoErro(erro) : null;

  return (
    <Modal titulo="Nova conversa" aoFechar={props.aoFechar}>
      <form
        className="formulario"
        onSubmit={(e) => {
          e.preventDefault();
          if (telefone.trim()) abrir.mutate();
        }}
      >
        <label className="campo">
          <span>Número de telefone</span>
          <input
            inputMode="tel"
            autoComplete="off"
            placeholder="(11) 99999-0000 ou +1 415 555 2671"
            value={telefone}
            onChange={(e) => setTelefone(e.target.value)}
          />
          <small>Sem DDI, o número é tratado como do Brasil (+55).</small>
        </label>
        {mensagemErro ? <FaixaAviso tipo={erro instanceof ErroMotor && erro.codigo === 'sem_whatsapp' ? 'aviso' : 'erro'}>{mensagemErro}</FaixaAviso> : null}
        <div className="acoes-formulario">
          <button type="button" className="botao secundario" onClick={props.aoFechar}>
            Cancelar
          </button>
          <button type="submit" className="botao" disabled={!telefone.trim() || abrir.isPending}>
            {abrir.isPending ? 'Verificando…' : 'Conversar'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
