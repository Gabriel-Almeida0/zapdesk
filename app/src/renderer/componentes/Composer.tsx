// Campo de mensagem (T059, T112, T113, T114, T125):
// - Enter envia, Shift+Enter quebra linha;
// - anexo por botão, arrastar e soltar (vem do Chat) ou colar imagem; legenda; limites recusados
//   ao anexar; "Enviar como documento";
// - gravar áudio (voz), figurinhas, responder citando e editar;
// - atalho `/`: sugere templates e insere texto + anexo para revisão antes do envio.
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { FileText, Mic, Paperclip, Pencil, Reply, Send, Sticker, X } from 'lucide-react';
import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from 'react';

import type { Arquivo, ComoEnviar, Conversa, Figurinha, Mensagem, Template } from '@zapdesk/cliente-motor';

import { chaves } from '../api/chaves';
import { atualizarMensagem, inserirMensagem } from '../api/eventos';
import { useCliente } from '../api/motor';
import { checarLimiteAnexo, tamanho, textoErro } from '../util/formatar';
import { BotaoIcone } from './BotaoIcone';
import { GravadorAudio } from './GravadorAudio';
import { SeletorFigurinha } from './SeletorFigurinha';
import { SugestoesTemplate, termoAtalho } from './SugestoesTemplate';

interface Anexo {
  arquivo: Arquivo;
  previa: string | null;
  comoDocumento: boolean;
}

export const LIMITE_TEXTO = 4096;

export interface PropsComposer {
  conversa: Conversa;
  citando: Mensagem | null;
  aoLimparCitacao: () => void;
  editando: Mensagem | null;
  aoLimparEdicao: () => void;
  /** Arquivos soltos sobre o chat. */
  arquivosSoltos: File[];
  aoConsumirArquivos: () => void;
}

function resumoMensagem(m: Mensagem): string {
  if (m.texto) return m.texto;
  const rotulos: Record<string, string> = {
    imagem: 'Imagem',
    video: 'Vídeo',
    audio: 'Áudio',
    documento: m.midia?.nome_arquivo ?? 'Documento',
    figurinha: 'Figurinha',
  };
  return rotulos[m.tipo] ?? 'Mensagem';
}

export function Composer(props: PropsComposer) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const campo = useRef<HTMLTextAreaElement>(null);
  const entradaArquivo = useRef<HTMLInputElement>(null);

  const [texto, setTexto] = useState('');
  const [anexo, setAnexo] = useState<Anexo | null>(null);
  const [enviandoArquivo, setEnviandoArquivo] = useState(false);
  const [erro, setErro] = useState<string | null>(null);
  const [gravando, setGravando] = useState(false);
  const [figurinhas, setFigurinhas] = useState(false);
  const [indiceSugestao, setIndiceSugestao] = useState(0);
  const [sugestoesFechadas, setSugestoesFechadas] = useState(false);

  const { editando, citando, conversa } = props;
  const termo = editando ? null : termoAtalho(texto);
  const mostrarSugestoes = termo !== null && !sugestoesFechadas;

  const templates = useQuery({
    queryKey: [...chaves.templates, { busca: termo ?? '' }],
    queryFn: () => cliente.listarTemplates(termo || undefined),
    enabled: mostrarSugestoes,
  });
  const listaTemplates = (templates.data ?? []).slice(0, 8);

  // Editar: texto atual no campo.
  useEffect(() => {
    if (editando) {
      setTexto(editando.texto ?? '');
      campo.current?.focus();
    }
  }, [editando]);

  useEffect(() => {
    if (citando) campo.current?.focus();
  }, [citando]);

  useEffect(() => {
    setIndiceSugestao(0);
  }, [termo]);

  // Altura automática do campo.
  useEffect(() => {
    const el = campo.current;
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = `${Math.min(el.scrollHeight, 180)}px`;
  }, [texto]);

  const limparAnexo = useCallback(() => {
    setAnexo((a) => {
      if (a?.previa?.startsWith('blob:')) URL.revokeObjectURL(a.previa);
      return null;
    });
  }, []);

  const anexar = useCallback(
    async (arquivo: File) => {
      setErro(null);
      const recusa = checarLimiteAnexo(arquivo);
      if (recusa) {
        setErro(recusa);
        return;
      }
      setEnviandoArquivo(true);
      try {
        const enviado = await cliente.enviarArquivo({ dados: arquivo, nome: arquivo.name || 'arquivo' });
        const previa = arquivo.type.startsWith('image/') ? URL.createObjectURL(arquivo) : null;
        limparAnexo();
        setAnexo({ arquivo: enviado, previa, comoDocumento: false });
        campo.current?.focus();
      } catch (e) {
        setErro(textoErro(e));
      } finally {
        setEnviandoArquivo(false);
      }
    },
    [cliente, limparAnexo],
  );

  // Arquivos soltos no chat.
  const { arquivosSoltos, aoConsumirArquivos } = props;
  useEffect(() => {
    const primeiro = arquivosSoltos[0];
    if (!primeiro) return;
    aoConsumirArquivos();
    void anexar(primeiro);
  }, [arquivosSoltos, aoConsumirArquivos, anexar]);

  const enviar = useMutation({
    mutationFn: (corpo: { texto: string | null; arquivo_id: string | null; como: ComoEnviar; citar: string | null }) =>
      cliente.enviarMensagem(conversa.id, {
        texto: corpo.texto,
        arquivo_id: corpo.arquivo_id,
        como: corpo.como,
        citar_mensagem_id: corpo.citar,
      }),
    onSuccess: (mensagem) => {
      inserirMensagem(qc, mensagem);
      void qc.invalidateQueries({ queryKey: chaves.conversas(conversa.conta_id) });
    },
    onError: (e) => setErro(textoErro(e)),
  });

  const editar = useMutation({
    mutationFn: (dados: { id: string; texto: string }) => cliente.editarMensagem(dados.id, dados.texto),
    onSuccess: (m) => {
      atualizarMensagem(qc, m);
      props.aoLimparEdicao();
      setTexto('');
    },
    onError: (e) => setErro(textoErro(e)),
  });

  const escolherTemplate = (t: Template) => {
    setTexto(t.texto);
    setSugestoesFechadas(true);
    if (t.arquivo) {
      limparAnexo();
      setAnexo({
        arquivo: t.arquivo,
        previa: t.arquivo.tipo_midia === 'imagem' ? cliente.urlConteudoArquivo(t.arquivo.id) : null,
        comoDocumento: t.arquivo.tipo_midia === 'documento',
      });
    }
    campo.current?.focus();
  };

  const submeter = () => {
    setErro(null);
    const conteudo = texto.trim();
    if (editando) {
      if (!conteudo) return;
      editar.mutate({ id: editando.id, texto: conteudo });
      return;
    }
    if (!conteudo && !anexo) return;
    if (conteudo.length > LIMITE_TEXTO) {
      setErro(`A mensagem passou de ${LIMITE_TEXTO.toLocaleString('pt-BR')} caracteres.`);
      return;
    }
    const como: ComoEnviar = anexo?.comoDocumento ? 'documento' : anexo?.arquivo.tipo_midia === 'figurinha' ? 'figurinha' : 'auto';
    enviar.mutate(
      { texto: conteudo || null, arquivo_id: anexo?.arquivo.id ?? null, como, citar: citando?.id ?? null },
      {
        onSuccess: () => {
          setTexto('');
          limparAnexo();
          setSugestoesFechadas(false);
          props.aoLimparCitacao();
        },
      },
    );
  };

  const enviarAudio = useCallback(
    async (audio: Blob) => {
      setGravando(false);
      setErro(null);
      try {
        const arquivo = await cliente.enviarArquivo({ dados: audio, nome: `audio-${Date.now()}.webm` });
        enviar.mutate({ texto: null, arquivo_id: arquivo.id, como: 'voz', citar: citando?.id ?? null });
        props.aoLimparCitacao();
      } catch (e) {
        setErro(textoErro(e));
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [cliente, citando],
  );

  const enviarFigurinha = async (f: Figurinha) => {
    setFigurinhas(false);
    setErro(null);
    try {
      let arquivoId = f.arquivo_id;
      if (!arquivoId && f.mensagem_id) {
        const dados = await cliente.baixarMidiaMensagem(f.mensagem_id);
        arquivoId = (await cliente.enviarArquivo({ dados, nome: 'figurinha.webp' })).id;
      }
      if (arquivoId) enviar.mutate({ texto: null, arquivo_id: arquivoId, como: 'figurinha', citar: null });
    } catch (e) {
      setErro(textoErro(e));
    }
  };

  const enviarArquivoFigurinha = async (arquivo: File) => {
    setFigurinhas(false);
    const recusa = arquivo.type !== 'image/webp' ? 'Figurinhas precisam ser .webp.' : checarLimiteAnexo(arquivo);
    if (recusa) {
      setErro(recusa);
      return;
    }
    try {
      const enviado = await cliente.enviarArquivo({ dados: arquivo, nome: arquivo.name });
      enviar.mutate({ texto: null, arquivo_id: enviado.id, como: 'figurinha', citar: null });
    } catch (e) {
      setErro(textoErro(e));
    }
  };

  const aoTeclar = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.nativeEvent.isComposing) return;
    if (mostrarSugestoes && listaTemplates.length > 0) {
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault();
        const passo = e.key === 'ArrowDown' ? 1 : -1;
        setIndiceSugestao((i) => (i + passo + listaTemplates.length) % listaTemplates.length);
        return;
      }
      if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault();
        const t = listaTemplates[indiceSugestao];
        if (t) escolherTemplate(t);
        return;
      }
    }
    if (e.key === 'Escape') {
      if (mostrarSugestoes) setSugestoesFechadas(true);
      else if (editando) {
        props.aoLimparEdicao();
        setTexto('');
      } else if (citando) props.aoLimparCitacao();
      return;
    }
    if (e.key === 'Enter' && !e.shiftKey && !e.metaKey) {
      e.preventDefault();
      submeter();
    }
  };

  const ocupado = enviar.isPending || editar.isPending || enviandoArquivo;
  const podeEnviar = Boolean(texto.trim() || anexo) && !enviandoArquivo;
  const tipoAnexo = anexo?.arquivo.tipo_midia;

  if (gravando) {
    return (
      <footer className="composer">
        <GravadorAudio aoEnviar={(audio) => void enviarAudio(audio)} aoCancelar={() => setGravando(false)} />
      </footer>
    );
  }

  return (
    <footer className="composer">
      {mostrarSugestoes ? (
        <SugestoesTemplate
          templates={listaTemplates}
          indice={indiceSugestao}
          carregando={templates.isPending}
          aoEscolher={escolherTemplate}
          aoPassar={setIndiceSugestao}
        />
      ) : null}
      {citando || editando ? (
        <div className="composer-contexto">
          {editando ? <Pencil size={16} aria-hidden="true" /> : <Reply size={16} aria-hidden="true" />}
          <div className="citacao">
            <strong>{editando ? 'Editando mensagem' : citando?.de_mim ? 'Você' : (citando?.remetente_nome ?? 'Mensagem')}</strong>
            <span>{resumoMensagem((editando ?? citando) as Mensagem)}</span>
          </div>
          <BotaoIcone
            rotulo={editando ? 'Cancelar edição' : 'Cancelar resposta'}
            onClick={() => {
              if (editando) {
                props.aoLimparEdicao();
                setTexto('');
              } else props.aoLimparCitacao();
            }}
          >
            <X size={18} />
          </BotaoIcone>
        </div>
      ) : null}
      {anexo ? (
        <div className="composer-anexo">
          {anexo.previa ? (
            <img src={anexo.previa} alt="" className="composer-anexo-previa" />
          ) : (
            <FileText size={28} aria-hidden="true" />
          )}
          <span className="composer-anexo-textos">
            <strong>{anexo.arquivo.nome}</strong>
            <small>{tamanho(anexo.arquivo.tamanho)} · a legenda é o texto abaixo</small>
          </span>
          {tipoAnexo === 'imagem' || tipoAnexo === 'video' ? (
            <label className="caixa">
              <input
                type="checkbox"
                checked={anexo.comoDocumento}
                onChange={(e) => setAnexo({ ...anexo, comoDocumento: e.target.checked })}
              />
              Enviar como documento
            </label>
          ) : null}
          <BotaoIcone rotulo="Remover anexo" onClick={limparAnexo}>
            <X size={18} />
          </BotaoIcone>
        </div>
      ) : null}
      {erro ? (
        <div className="composer-erro" role="alert">
          {erro}
          <button type="button" className="botao-link" onClick={() => setErro(null)}>
            Fechar
          </button>
        </div>
      ) : null}
      <div className="composer-linha">
        {!editando ? (
          <>
            <div className="ancora-figurinha">
              <BotaoIcone rotulo="Figurinhas" ativo={figurinhas} onClick={() => setFigurinhas((f) => !f)}>
                <Sticker size={22} />
              </BotaoIcone>
              {figurinhas ? (
                <SeletorFigurinha
                  contaId={conversa.conta_id}
                  aoEscolher={(f) => void enviarFigurinha(f)}
                  aoArquivo={(a) => void enviarArquivoFigurinha(a)}
                />
              ) : null}
            </div>
            <BotaoIcone rotulo="Anexar arquivo" disabled={enviandoArquivo} onClick={() => entradaArquivo.current?.click()}>
              <Paperclip size={22} />
            </BotaoIcone>
            <input
              ref={entradaArquivo}
              type="file"
              hidden
              onChange={(e) => {
                const arquivo = e.target.files?.[0];
                if (arquivo) void anexar(arquivo);
                e.target.value = '';
              }}
            />
          </>
        ) : null}
        <textarea
          ref={campo}
          className="composer-campo"
          rows={1}
          aria-label={editando ? 'Editar mensagem' : anexo ? 'Legenda' : 'Mensagem'}
          placeholder={enviandoArquivo ? 'Enviando anexo…' : anexo ? 'Adicione uma legenda' : 'Digite uma mensagem ou / para templates'}
          value={texto}
          onChange={(e) => {
            setTexto(e.target.value);
            if (termoAtalho(e.target.value) === null) setSugestoesFechadas(false);
          }}
          onKeyDown={aoTeclar}
          onPaste={(e) => {
            const arquivo = e.clipboardData.files[0];
            if (arquivo) {
              e.preventDefault();
              void anexar(arquivo);
            }
          }}
        />
        {podeEnviar || editando ? (
          <BotaoIcone rotulo={editando ? 'Salvar edição' : 'Enviar'} className="botao-enviar" disabled={ocupado || !podeEnviar} onClick={submeter}>
            <Send size={22} />
          </BotaoIcone>
        ) : (
          <BotaoIcone rotulo="Gravar áudio" onClick={() => setGravando(true)}>
            <Mic size={22} />
          </BotaoIcone>
        )}
      </div>
    </footer>
  );
}
