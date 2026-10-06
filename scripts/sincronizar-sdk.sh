#!/usr/bin/env bash
# Copia os tipos do SDK (compartilhado/automacao-sdk/dist/index.d.ts, um arquivo só gerado por
# `npm run compilar -w @zapdesk/automacao`) para o motor, que os embute (go:embed) e os grava em
# `.zapdesk/automacao.d.ts` de cada projeto de automação de IA (research.md R14).
#
#   bash scripts/sincronizar-sdk.sh              # copia
#   bash scripts/sincronizar-sdk.sh --verificar  # só confere; falha se estiverem diferentes (CI)
#
# Não compila o SDK: rode antes `npm run compartilhado:compilar` (o `npm run sdk:sincronizar`
# da raiz só chama este script).
set -euo pipefail
raiz="$(cd "$(dirname "$0")/.." && pwd)"
origem="$raiz/compartilhado/automacao-sdk/dist/index.d.ts"
destino="$raiz/motor/internal/automacoes/projetos/sdk/automacao.d.ts"

if [[ ! -f "$origem" ]]; then
  echo "SDK não compilado: $origem não existe. Rode 'npm run compartilhado:compilar'." >&2
  exit 1
fi

case "${1:-}" in
  --verificar)
    if [[ ! -f "$destino" ]]; then
      echo "Tipos do SDK ausentes no motor ($destino). Rode 'npm run sdk:sincronizar'." >&2
      exit 1
    fi
    if ! cmp -s "$origem" "$destino"; then
      echo "Tipos do SDK no motor estão desatualizados. Rode 'npm run sdk:sincronizar' e comite." >&2
      diff -u "$destino" "$origem" | head -40 >&2 || true
      exit 1
    fi
    echo "Tipos do SDK sincronizados."
    ;;
  "")
    mkdir -p "$(dirname "$destino")"
    cp "$origem" "$destino"
    echo "Tipos do SDK copiados para ${destino#"$raiz/"}"
    ;;
  *)
    echo "Uso: $0 [--verificar]" >&2
    exit 2
    ;;
esac
