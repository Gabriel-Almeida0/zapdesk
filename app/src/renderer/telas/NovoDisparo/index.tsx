// Assistente de Novo disparo (T098): Lista → Mensagem → Ritmo e agendamento → Revisão.
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Check, Megaphone } from 'lucide-react';
import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router';

import { ErroMotor, type Arquivo } from '@zapdesk/cliente-motor';

import { chaves } from '../../api/chaves';
import { useCliente } from '../../api/motor';
import { BotaoIcone } from '../../componentes/BotaoIcone';
import { EstadoVazio } from '../../componentes/EstadoVazio';
import { useContaAtual } from '../../estado/conta';
import {
  faltandoPorVariavel,
  formInicial,
  montarNovoDisparo,
  validarMensagemForm,
  validarRitmoForm,
  type FormDisparo,
} from '../../util/disparos';
import { numero } from '../../util/formatar';
import { PassoLista, type FonteLista } from './PassoLista';
import { PassoMensagem } from './PassoMensagem';
import { PassoRevisao, textoFaltando } from './PassoRevisao';
import { PassoRitmo } from './PassoRitmo';
import { useValidacaoDisparo } from './validacao';

export const PASSOS = ['Lista', 'Mensagem', 'Ritmo e agendamento', 'Revisão'] as const;

/** Por que não dá para avançar do passo (null = pode). */
export function bloqueioDoPasso(passo: number, form: FormDisparo): string | null {
  if (passo === 0) {
    if (!form.contaId) return 'Escolha a conta.';
    if (form.leadIds.length === 0) return 'Adicione pelo menos 1 destinatário.';
  }
  if (passo === 1) return validarMensagemForm(form);
  if (passo === 2) {
    const erros = Object.values(validarRitmoForm(form));
    return erros[0] ?? null;
  }
  return null;
}

export function NovoDisparo() {
  const cliente = useCliente();
  const qc = useQueryClient();
  const navegar = useNavigate();
  const { contas, contaId } = useContaAtual();
  const [passo, setPasso] = useState(0);
  const [form, setForm] = useState<FormDisparo>(() => formInicial(contaId ?? ''));
  const [fontes, setFontes] = useState<FonteLista[]>([]);
  const [arquivo, setArquivo] = useState<Arquivo | null>(null);
  const mudar = (parcial: Partial<FormDisparo>) => setForm((f) => ({ ...f, ...parcial }));
  // As contas podem chegar depois da montagem: assume a conta atual se nenhuma foi escolhida.
  useEffect(() => {
    if (contaId) setForm((f) => (f.contaId ? f : { ...f, contaId }));
  }, [contaId]);

  const validacao = useValidacaoDisparo({ ...form, valoresPadrao: {} }, passo === 3);
  const faltando = faltandoPorVariavel(validacao.data?.faltando ?? []);
  const semValor = [...faltando.entries()].filter(([v]) => !form.valoresPadrao[v]?.trim());

  const criar = useMutation({
    mutationFn: (iniciar: boolean) => cliente.criarDisparo(montarNovoDisparo(form, iniciar)),
    onSuccess: (d) => {
      qc.setQueryData(chaves.disparo(d.id), d);
      void qc.invalidateQueries({ queryKey: chaves.disparos });
      navegar(`/disparos/${d.id}`, { replace: true });
    },
  });

  if (contas.length === 0) {
    return (
      <EstadoVazio
        icone={<Megaphone size={56} />}
        titulo="Nenhuma conta conectada"
        texto="Conecte um número para disparar."
        acao={
          <Link to="/contas/conectar" className="botao">
            Conectar conta
          </Link>
        }
      />
    );
  }

  const bloqueio = bloqueioDoPasso(passo, form);
  const erroVariaveis =
    criar.error instanceof ErroMotor && criar.error.codigo === 'variaveis_faltando' ? criar.error : null;

  return (
    <section className="tela tela-rolavel novo-disparo">
      <header className="cabecalho-tela">
        <BotaoIcone rotulo="Voltar para Disparos" onClick={() => navegar('/disparos')}>
          <ArrowLeft size={20} />
        </BotaoIcone>
        <h1>Novo disparo</h1>
      </header>
      <ol className="etapas" aria-label="Etapas">
        {PASSOS.map((nome, i) => (
          <li key={nome} className={i === passo ? 'atual' : i < passo ? 'feita' : undefined} aria-current={i === passo ? 'step' : undefined}>
            <button type="button" disabled={i > passo} onClick={() => setPasso(i)}>
              <span className="etapa-numero">
                {i < passo ? (
                  <>
                    <Check size={12} strokeWidth={3} aria-hidden="true" />
                    <span className="etapa-numero-texto so-leitor">{i + 1}</span>
                  </>
                ) : (
                  i + 1
                )}
              </span>{' '}
              <span className="etapa-nome">{nome}</span>
              {i === passo ? <span className="lampada" aria-hidden="true" /> : null}
            </button>
          </li>
        ))}
      </ol>

      <div className="cartao cartao-recuado">
        {passo === 0 ? <PassoLista form={form} fontes={fontes} aoMudarForm={mudar} aoMudarFontes={setFontes} /> : null}
        {passo === 1 ? (
          <PassoMensagem
            form={form}
            arquivo={arquivo}
            aoMudarForm={mudar}
            aoMudarArquivo={(a) => {
              setArquivo(a);
              mudar({ arquivoId: a?.id ?? null });
            }}
          />
        ) : null}
        {passo === 2 ? <PassoRitmo form={form} aoMudarForm={mudar} /> : null}
        {passo === 3 ? (
          <PassoRevisao form={form} arquivo={arquivo} aoMudarForm={mudar} erroEnvio={erroVariaveis ? null : criar.error} />
        ) : null}
      </div>

      <footer className="rodape-assistente">
        <span className="texto-secundario" role="status">
          {passo < 3 ? (bloqueio ?? `${numero(form.leadIds.length)} destinatários`) : semValor.length > 0 ? semValor.map(([v, l]) => textoFaltando(l.length, v)).join(' · ') : ''}
        </span>
        <div className="acoes-formulario">
          {passo > 0 ? (
            <button type="button" className="botao secundario" onClick={() => setPasso((p) => p - 1)}>
              Voltar
            </button>
          ) : null}
          {passo < 3 ? (
            <button type="button" className="botao" disabled={Boolean(bloqueio)} onClick={() => setPasso((p) => p + 1)}>
              Continuar
            </button>
          ) : (
            <>
              <button type="button" className="botao secundario" disabled={criar.isPending} onClick={() => criar.mutate(false)}>
                Salvar rascunho
              </button>
              <button
                type="button"
                className="botao"
                disabled={criar.isPending || semValor.length > 0 || validacao.isPending}
                onClick={() => criar.mutate(true)}
              >
                {criar.isPending ? 'Iniciando…' : 'Iniciar'}
              </button>
            </>
          )}
        </div>
      </footer>
      {erroVariaveis ? (
        <p className="erro-campo" role="alert">
          {erroVariaveis.mensagem}
        </p>
      ) : null}
    </section>
  );
}
