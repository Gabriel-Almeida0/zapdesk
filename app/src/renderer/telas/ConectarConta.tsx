// Conectar conta por QR (T056): cria a conta (ou reconecta a do `?conta=`), mostra o QR que chega
// por `conta.qr` (renovado sozinho), acompanha "Sincronizando histórico…" e abre as conversas.
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, CheckCircle2, QrCode, Smartphone } from 'lucide-react';
import QRCode from 'qrcode';
import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router';

import { ErroMotor, type Conta, type QrConta } from '@zapdesk/cliente-motor';

import { chaves } from '../api/chaves';
import { useCliente, useEventoMotor, useModoMotor } from '../api/motor';
import { BotaoIcone } from '../componentes/BotaoIcone';
import { Esqueleto } from '../componentes/Esqueleto';
import { FaixaAviso } from '../componentes/FaixaAviso';
import { useContaAtual } from '../estado/conta';
import { numero, textoErro } from '../util/formatar';

export const TEXTO_QR = 'Escaneie o QR code no seu celular em Aparelhos conectados';

interface Progresso {
  conversas: number;
  mensagens: number;
  concluida: boolean;
}

function ImagemQr({ codigo }: { codigo: string }) {
  const [url, setUrl] = useState<string | null>(null);
  useEffect(() => {
    let ativo = true;
    QRCode.toDataURL(codigo, { margin: 1, width: 528, errorCorrectionLevel: 'L' })
      .then((u) => ativo && setUrl(u))
      .catch(() => ativo && setUrl(null));
    return () => {
      ativo = false;
    };
  }, [codigo]);
  if (!url) return <Esqueleto largura={264} altura={264} />;
  return <img className="qr" src={url} width={264} height={264} alt="QR code para conectar o WhatsApp" />;
}

export function ConectarConta() {
  const cliente = useCliente();
  const qc = useQueryClient();
  const navegar = useNavigate();
  const { whatsapp } = useModoMotor();
  const [parametros] = useSearchParams();
  const { contas, selecionar } = useContaAtual();

  const [contaId, setContaId] = useState<string | null>(parametros.get('conta'));
  const [nome, setNome] = useState('');
  const [qr, setQr] = useState<QrConta | null>(null);
  const [expirou, setExpirou] = useState(false);
  const [progresso, setProgresso] = useState<Progresso | null>(null);

  const conta: Conta | null = contas.find((c) => c.id === contaId) ?? null;

  const criar = useMutation({
    mutationFn: () => cliente.criarConta(nome.trim() ? { nome: nome.trim() } : {}),
    onSuccess: (nova) => {
      qc.setQueryData<Conta[]>(chaves.contas, (lista) =>
        lista ? [...lista.filter((c) => c.id !== nova.id), nova] : [nova],
      );
      setContaId(nova.id);
    },
  });

  const novoQr = useMutation({
    mutationFn: (id: string) => cliente.reconectarConta(id),
    onSuccess: () => {
      setExpirou(false);
      setQr(null);
    },
  });

  const simular = useMutation({
    mutationFn: (id: string) =>
      cliente.falso.escanearQr(id, {
        telefone: `+55119${String(Math.floor(10_000_000 + Math.random() * 89_999_999))}`,
        nome: nome.trim() || 'Conta de teste',
      }),
  });

  // QR já ativo (ex.: tela reaberta durante o pareamento).
  useEffect(() => {
    if (!contaId) return;
    let ativo = true;
    cliente
      .obterQr(contaId)
      .then((q) => ativo && setQr(q))
      .catch((e: unknown) => {
        if (!(e instanceof ErroMotor && e.codigo === 'nao_encontrado')) return;
      });
    return () => {
      ativo = false;
    };
  }, [cliente, contaId]);

  useEventoMotor((evento) => {
    if (!contaId || evento.conta_id !== contaId) return;
    if (evento.tipo === 'conta.qr') {
      setQr(evento.dados);
      setExpirou(false);
    } else if (evento.tipo === 'conta.qr_expirado') {
      setQr(null);
      setExpirou(true);
    } else if (evento.tipo === 'sincronizacao.progresso') {
      setProgresso(evento.dados);
    }
  });

  const conectada = conta?.estado === 'conectada';
  const sincronizado = conectada && (progresso?.concluida === true || (!conta.sincronizando && progresso === null));

  const abrirConversas = () => {
    if (contaId) selecionar(contaId);
    navegar('/conversas');
  };

  // Ao terminar a sincronização, vai direto para as conversas.
  useEffect(() => {
    if (!conectada || progresso?.concluida !== true) return;
    const t = setTimeout(abrirConversas, 1200);
    return () => clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [conectada, progresso?.concluida]);

  return (
    <section className="tela tela-centro conectar-conta">
      <header className="cabecalho-tela">
        <BotaoIcone rotulo="Voltar" onClick={() => navegar(-1)}>
          <ArrowLeft size={20} />
        </BotaoIcone>
        <h1>Conectar conta</h1>
      </header>

      <div className="cartao conectar-cartao">
        {!contaId ? (
          <>
            {contas.length === 0 ? <p className="destaque">Nenhuma conta conectada</p> : null}
            <div className="conectar-passos">
              <Smartphone size={40} aria-hidden="true" />
              <ol>
                <li>Abra o WhatsApp no celular.</li>
                <li>
                  Toque em <strong>Mais opções</strong> ou <strong>Configurações</strong> e depois em{' '}
                  <strong>Aparelhos conectados</strong>.
                </li>
                <li>
                  Toque em <strong>Conectar um aparelho</strong> e aponte o celular para o QR code.
                </li>
              </ol>
            </div>
            <label className="campo">
              <span>Nome da conta (opcional)</span>
              <input
                value={nome}
                maxLength={60}
                placeholder="Ex.: Comercial"
                onChange={(e) => setNome(e.target.value)}
              />
            </label>
            {criar.error ? <FaixaAviso tipo="erro">{textoErro(criar.error)}</FaixaAviso> : null}
            <button type="button" className="botao grande" disabled={criar.isPending} onClick={() => criar.mutate()}>
              <QrCode size={18} aria-hidden="true" /> {criar.isPending ? 'Gerando…' : 'Gerar QR code'}
            </button>
          </>
        ) : conta?.estado === 'banida' ? (
          <FaixaAviso tipo="erro">O WhatsApp bloqueou este número.</FaixaAviso>
        ) : conectada ? (
          <div className="conectar-sincronizando" role="status">
            {sincronizado ? (
              <>
                <CheckCircle2 size={48} className="icone-ok" aria-hidden="true" />
                <h2>Conta conectada</h2>
                <p>{conta.nome}</p>
              </>
            ) : (
              <>
                <div className="barra-indeterminada" aria-hidden="true">
                  <span />
                </div>
                <h2>Sincronizando histórico…</h2>
                {progresso ? (
                  <p>
                    {numero(progresso.conversas)} conversas · {numero(progresso.mensagens)} mensagens
                  </p>
                ) : (
                  <p>Isso pode levar alguns minutos na primeira vez.</p>
                )}
              </>
            )}
            <button type="button" className="botao" onClick={abrirConversas}>
              Abrir conversas
            </button>
          </div>
        ) : (
          <div className="conectar-qr">
            <div className="qr-moldura" aria-live="polite">
              {expirou ? (
                <div className="qr-expirado">
                  <p>O QR code expirou.</p>
                  <button
                    type="button"
                    className="botao"
                    disabled={novoQr.isPending}
                    onClick={() => contaId && novoQr.mutate(contaId)}
                  >
                    Gerar novo QR
                  </button>
                </div>
              ) : qr ? (
                <ImagemQr codigo={qr.codigo} />
              ) : (
                <div className="qr-carregando">
                  <Esqueleto largura={264} altura={264} />
                  <span>Gerando QR code…</span>
                </div>
              )}
            </div>
            <p className="texto-qr">{TEXTO_QR}</p>
            <p className="texto-secundario">O QR code é renovado automaticamente.</p>
            {whatsapp === 'falso' && contaId ? (
              <button
                type="button"
                className="botao secundario"
                disabled={simular.isPending}
                onClick={() => simular.mutate(contaId)}
              >
                Simular leitura do QR (modo falso)
              </button>
            ) : null}
            {novoQr.error ? <FaixaAviso tipo="erro">{textoErro(novoQr.error)}</FaixaAviso> : null}
          </div>
        )}
      </div>
    </section>
  );
}
