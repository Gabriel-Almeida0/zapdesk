// Esqueletos de carregamento (listas e blocos).
export function Esqueleto({ largura, altura = 12, redondo }: { largura?: number | string; altura?: number; redondo?: boolean }) {
  return (
    <span
      className={`esqueleto${redondo ? ' redondo' : ''}`}
      style={{ width: largura ?? '100%', height: altura }}
      aria-hidden="true"
    />
  );
}

/** Linhas no formato da lista de conversas/contatos. */
export function EsqueletoLista({ linhas = 8 }: { linhas?: number }) {
  return (
    <div className="esqueleto-lista" role="status" aria-label="Carregando">
      {Array.from({ length: linhas }, (_, i) => (
        <div className="esqueleto-item" key={i}>
          <Esqueleto largura={49} altura={49} redondo />
          <div className="esqueleto-textos">
            <Esqueleto largura={`${45 + ((i * 17) % 30)}%`} altura={13} />
            <Esqueleto largura={`${60 + ((i * 11) % 30)}%`} altura={11} />
          </div>
        </div>
      ))}
    </div>
  );
}
