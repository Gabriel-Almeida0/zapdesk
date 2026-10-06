#!/usr/bin/env bash
# Empacota o ZapDesk (T147; runner na 002/T007): motor (darwin/arm64, sem CGO) → app/resources/bin,
# bundles do MCP e do runner das automações,
# electron-builder (.dmg arm64, bundle id com.gabriel.zapdesk) e conferência do resultado.
set -euo pipefail
raiz="$(cd "$(dirname "$0")/.." && pwd)"
app="$raiz/app"

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "O empacotamento é só para macOS." >&2
  exit 1
fi

echo "==> Compilando o motor (darwin/arm64, sem CGO)"
mkdir -p "$app/resources/bin"
(cd "$raiz/motor" && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
  go build -trimpath -ldflags "-s -w" -o "$app/resources/bin/zapdesk-motor" ./cmd/zapdesk-motor)
"$app/resources/bin/zapdesk-motor" --versao

echo "==> Compilando os pacotes compartilhados (cliente do motor e SDK das automações)"
npm run compartilhado:compilar --prefix "$raiz"
bash "$raiz/scripts/sincronizar-sdk.sh" --verificar

echo "==> Gerando o bundle do runner das automações de IA"
npm run runner:compilar --prefix "$raiz"
test -f "$raiz/automacao/runner/dist/zapdesk-runner.mjs"

echo "==> Gerando o bundle do MCP"
npm run compilar -w @zapdesk/mcp --prefix "$raiz"
test -f "$raiz/mcp/dist/zapdesk-mcp.mjs"

echo "==> Ícones"
node "$app/scripts/gerar-icones.mjs"

echo "==> Empacotando o app (electron-vite + electron-builder)"
rm -rf "$app/dist"
(cd "$app" && CSC_IDENTITY_AUTO_DISCOVERY=false npx electron-vite build && \
  CSC_IDENTITY_AUTO_DISCOVERY=false npx electron-builder --mac --arm64 --config electron-builder.yml --publish never)

pacote="$app/dist/mac-arm64/ZapDesk.app"
dmg="$(ls "$app"/dist/ZapDesk-*-arm64.dmg | head -1)"

echo "==> Conferindo o pacote"
test -x "$pacote/Contents/Resources/bin/zapdesk-motor" || { echo "Motor ausente no pacote." >&2; exit 1; }
test -f "$pacote/Contents/Resources/mcp/zapdesk-mcp.mjs" || { echo "MCP ausente no pacote." >&2; exit 1; }
test -f "$pacote/Contents/Resources/runner/zapdesk-runner.mjs" || { echo "Runner das automações ausente no pacote." >&2; exit 1; }
/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$pacote/Contents/Info.plist"
/usr/libexec/PlistBuddy -c 'Print :NSMicrophoneUsageDescription' "$pacote/Contents/Info.plist"
file "$pacote/Contents/Resources/bin/zapdesk-motor"
echo "--- codesign do motor:"
codesign -dv "$pacote/Contents/Resources/bin/zapdesk-motor" 2>&1 | sed 's/^/    /'
echo "--- codesign do app:"
codesign -dv "$pacote" 2>&1 | sed 's/^/    /'
codesign --verify --deep --strict "$pacote" && echo "    assinatura (ad-hoc) válida"

echo "==> MCP rodando com o executável do app (ELECTRON_RUN_AS_NODE=1)"
ELECTRON_RUN_AS_NODE=1 "$pacote/Contents/MacOS/ZapDesk" -e 'console.log("    RunAsNode ok, node " + process.versions.node)'

echo
echo "Pronto: $dmg"
echo "Primeira abertura (app sem notarização): abra o ZapDesk uma vez, depois vá em Ajustes do Sistema ›"
echo "Privacidade e Segurança e clique em \"Abrir mesmo assim\"."
