// Estado do projeto de uma automação de IA no editor: conteúdo de cada arquivo, o hash que o motor
// conhece (para detectar alteração externa e conflito ao salvar), salvar e compilar.
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useRef, useState } from 'react';

import { ErroMotor, type ArquivoProjeto, type Id, type ResultadoCompilacao } from '@zapdesk/cliente-motor';

import { chaves } from '../../api/chaves';
import { useCliente, useEventoMotor } from '../../api/motor';
import { textoErro } from '../../util/formatar';

export interface EstadoArquivo {
  /** Texto no editor. */
  conteudo: string;
  /** Último texto salvo/lido do disco. */
  salvo: string;
  /** Hash do `salvo` no motor. */
  hash: string;
  /** O arquivo mudou fora do app enquanto havia edição local. */
  conflito: boolean;
}

export const INTERVALO_VERIFICAR_MS = 3000;
export const ATRASO_SALVAR_MS = 800;

export function arquivoSujo(a: EstadoArquivo | undefined): boolean {
  return Boolean(a && a.conteudo !== a.salvo);
}

export function useProjetoIA(automacaoId: Id) {
  const cliente = useCliente();
  const qc = useQueryClient();
  const [arquivos, setArquivos] = useState<Record<string, EstadoArquivo>>({});
  const [carregando, setCarregando] = useState(true);
  const [erro, setErro] = useState<string | null>(null);
  const [compilacao, setCompilacao] = useState<ResultadoCompilacao | null>(null);
  const [compilando, setCompilando] = useState(false);
  const [salvando, setSalvando] = useState(false);
  const ref = useRef(arquivos);
  ref.current = arquivos;

  // Lista de arquivos com hash; consultada a cada 3 s enquanto o editor está aberto (VS Code).
  const lista = useQuery({
    queryKey: chaves.arquivos(automacaoId),
    queryFn: () => cliente.listarArquivos(automacaoId),
    refetchInterval: INTERVALO_VERIFICAR_MS,
    staleTime: 0,
  });

  const ler = useCallback(
    async (caminho: string) => {
      const c = await cliente.lerArquivo(automacaoId, caminho);
      setArquivos((atual) => ({ ...atual, [caminho]: { conteudo: c.conteudo, salvo: c.conteudo, hash: c.hash, conflito: false } }));
    },
    [cliente, automacaoId],
  );

  const compilar = useCallback(async () => {
    setCompilando(true);
    try {
      const r = await cliente.compilarAutomacao(automacaoId);
      setCompilacao(r);
      void qc.invalidateQueries({ queryKey: chaves.automacao(automacaoId) });
      return r;
    } catch (e) {
      setErro(textoErro(e));
      return null;
    } finally {
      setCompilando(false);
    }
  }, [cliente, automacaoId, qc]);

  // Sincroniza com a lista do motor: carrega novos, recarrega alterados fora (sem edição local),
  // marca conflito (com edição local) e remove os apagados.
  const dadosLista = lista.data;
  useEffect(() => {
    if (!dadosLista) return;
    const atuais = ref.current;
    const noMotor = new Map<string, ArquivoProjeto>(dadosLista.map((a) => [a.caminho, a]));
    const pendentes: Promise<void>[] = [];
    for (const a of dadosLista) {
      const local = atuais[a.caminho];
      if (!local) pendentes.push(ler(a.caminho));
      else if (local.hash !== a.hash) {
        if (arquivoSujo(local)) {
          if (!local.conflito) setArquivos((x) => (x[a.caminho] ? { ...x, [a.caminho]: { ...local, conflito: true } } : x));
        } else pendentes.push(ler(a.caminho));
      }
    }
    const removidos = Object.keys(atuais).filter((c) => !noMotor.has(c));
    if (removidos.length > 0) {
      setArquivos((x) => {
        const novo = { ...x };
        for (const c of removidos) delete novo[c];
        return novo;
      });
    }
    if (pendentes.length > 0 || carregando) {
      void Promise.all(pendentes)
        .catch((e: unknown) => setErro(textoErro(e)))
        .finally(() => setCarregando(false));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dadosLista, ler]);

  useEffect(() => {
    if (lista.isError) {
      setErro(textoErro(lista.error));
      setCarregando(false);
    }
  }, [lista.isError, lista.error]);

  useEventoMotor((evento) => {
    if (evento.tipo === 'automacao.arquivos_alterados' && evento.dados.automacao_id === automacaoId) void lista.refetch();
  });

  const editar = useCallback((caminho: string, conteudo: string) => {
    setArquivos((x) => {
      const a = x[caminho];
      return a ? { ...x, [caminho]: { ...a, conteudo } } : x;
    });
  }, []);

  /** Salva os arquivos alterados. `forcar` = sobrescreve mesmo com conflito. Devolve true se tudo salvou. */
  const salvar = useCallback(
    async (opcoes: { forcar?: boolean; compilar?: boolean } = {}): Promise<boolean> => {
      const sujos = Object.entries(ref.current).filter(([, a]) => arquivoSujo(a));
      if (sujos.length === 0) {
        if (opcoes.compilar) await compilar();
        return true;
      }
      setSalvando(true);
      let ok = true;
      try {
        for (const [caminho, a] of sujos) {
          if (a.conflito && !opcoes.forcar) {
            ok = false;
            continue;
          }
          try {
            const r = await cliente.escreverArquivo(automacaoId, caminho, a.conteudo, opcoes.forcar ? undefined : a.hash);
            setArquivos((x) => {
              const atual = x[caminho];
              return atual ? { ...x, [caminho]: { ...atual, salvo: a.conteudo, hash: r.hash, conflito: false } } : x;
            });
          } catch (e) {
            ok = false;
            if (e instanceof ErroMotor && e.codigo === 'conflito') {
              setArquivos((x) => {
                const atual = x[caminho];
                return atual ? { ...x, [caminho]: { ...atual, conflito: true } } : x;
              });
            } else setErro(textoErro(e));
          }
        }
      } finally {
        setSalvando(false);
      }
      if (opcoes.compilar !== false) await compilar();
      return ok;
    },
    [cliente, automacaoId, compilar],
  );

  const recarregar = useCallback((caminho: string) => ler(caminho), [ler]);

  const criar = useCallback(
    async (caminho: string, conteudo = '') => {
      const r = await cliente.escreverArquivo(automacaoId, caminho, conteudo, null);
      setArquivos((x) => ({ ...x, [caminho]: { conteudo, salvo: conteudo, hash: r.hash, conflito: false } }));
      void lista.refetch();
      await compilar();
    },
    [cliente, automacaoId, lista, compilar],
  );

  const renomear = useCallback(
    async (de: string, para: string) => {
      const r = await cliente.renomearArquivo(automacaoId, de, para);
      setArquivos((x) => {
        const novo = { ...x };
        const a = novo[de];
        delete novo[de];
        if (a) novo[para] = { ...a, hash: r.hash };
        return novo;
      });
      void lista.refetch();
      await compilar();
    },
    [cliente, automacaoId, lista, compilar],
  );

  const excluir = useCallback(
    async (caminho: string) => {
      await cliente.excluirArquivo(automacaoId, caminho);
      setArquivos((x) => {
        const novo = { ...x };
        delete novo[caminho];
        return novo;
      });
      void lista.refetch();
      await compilar();
    },
    [cliente, automacaoId, lista, compilar],
  );

  return {
    arquivos,
    lista: dadosLista ?? [],
    carregando,
    erro,
    limparErro: () => setErro(null),
    compilacao,
    compilando,
    salvando,
    editar,
    salvar,
    compilar,
    recarregar,
    criar,
    renomear,
    excluir,
  };
}
