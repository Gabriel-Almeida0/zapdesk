// Seletor de figurinhas (T114): recentes (recebidas/enviadas) + enviar arquivo .webp.
import { useQuery } from '@tanstack/react-query';
import { Upload } from 'lucide-react';
import { useRef } from 'react';

import type { Figurinha, Id } from '@zapdesk/cliente-motor';

import { chaves } from '../api/chaves';
import { useCliente } from '../api/motor';
import { Esqueleto } from './Esqueleto';

export function SeletorFigurinha(props: { contaId: Id; aoEscolher: (f: Figurinha) => void; aoArquivo: (arquivo: File) => void }) {
  const cliente = useCliente();
  const entrada = useRef<HTMLInputElement>(null);
  const consulta = useQuery({
    queryKey: chaves.figurinhas(props.contaId),
    queryFn: () => cliente.listarFigurinhas(props.contaId, 40),
  });
  const figurinhas = consulta.data ?? [];
  return (
    <div className="menu-flutuante seletor-figurinha" role="dialog" aria-label="Figurinhas">
      <div className="seletor-figurinha-cabecalho">
        <strong>Figurinhas recentes</strong>
        <button type="button" className="botao secundario pequeno" onClick={() => entrada.current?.click()}>
          <Upload size={14} aria-hidden="true" /> Enviar .webp
        </button>
        <input
          ref={entrada}
          type="file"
          accept="image/webp"
          hidden
          onChange={(e) => {
            const arquivo = e.target.files?.[0];
            if (arquivo) props.aoArquivo(arquivo);
            e.target.value = '';
          }}
        />
      </div>
      {consulta.isPending ? (
        <div className="grade-figurinhas">
          {Array.from({ length: 8 }, (_, i) => (
            <Esqueleto key={i} largura={72} altura={72} />
          ))}
        </div>
      ) : figurinhas.length === 0 ? (
        <p className="texto-secundario">Nenhuma figurinha recente. As figurinhas recebidas e enviadas aparecem aqui.</p>
      ) : (
        <div className="grade-figurinhas">
          {figurinhas.map((f, i) => (
            <button
              key={f.arquivo_id ?? f.mensagem_id ?? i}
              type="button"
              className="figurinha"
              aria-label="Enviar figurinha"
              onClick={() => props.aoEscolher(f)}
            >
              <img src={cliente.urlBinaria(f.url)} alt="" width={72} height={72} loading="lazy" />
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
