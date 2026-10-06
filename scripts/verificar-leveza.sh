#!/usr/bin/env bash
# Verificação de leveza (T149, SC-002/SC-003 — quickstart.md cenários 2 e 3):
#   1. abre o app EMPACOTADO (ZapDesk.app);
#   2. espera o app ficar ocioso (padrão 5 min) medindo a memória (RSS) do motor e do app todo;
#   3. fecha o app (SIGTERM no processo principal, o mesmo encerramento de Cmd+Q sem disparo ativo;
#      com SAIR_APPLESCRIPT=1 manda o evento "quit" do macOS, que pode pedir permissão de Automação)
#      e confere que NENHUM processo do ZapDesk sobrou (`pgrep -fl -i zapdesk` vazio, inclui
#      `zapdesk-runner`) em até 5 s e que o runtime.json foi apagado.
# 002 (T128), com FALSO=1: antes da espera ativa um fluxo e um chatbot (a medição é com eles
# ativos e nenhum runner) e, com AUTOMACOES_IA=1 (padrão), roda uma automação de IA uma vez (IA
# falsa: ZAPDESK_IA=falsa) com ociosidade de 1 min e confere que o runner sobe, responde e é
# encerrado sozinho por ociosidade. `pronto` (runtime.json) tem de sair em < 2 s.
#
# Uso:
#   scripts/verificar-leveza.sh [caminho/ZapDesk.app]
# Variáveis:
#   ESPERA_S=300        segundos ociosos antes de medir (padrão 300 = 5 min)
#   LIMITE_MB=80        limite de RSS do motor ocioso
#   FALSO=1             abre com o WhatsApp falso e conecta 2 contas simuladas (padrão; 0 = real)
#   PASTA_DADOS=<dir>   pasta de dados (padrão: temporária, apagada no fim)
#   AUTOMACOES_IA=1     roda o ciclo do runner (só com FALSO=1; 0 pula)
# Sai com 0 se tudo passou, 1 se alguma verificação falhou.
set -uo pipefail

raiz="$(cd "$(dirname "$0")/.." && pwd)"
pacote="${1:-$raiz/app/dist/mac-arm64/ZapDesk.app}"
espera="${ESPERA_S:-300}"
limite_mb="${LIMITE_MB:-80}"
falso="${FALSO:-1}"
pasta_temporaria=0
if [[ -z "${PASTA_DADOS:-}" ]]; then
  PASTA_DADOS="$(mktemp -d -t zapdesk-leveza)"
  pasta_temporaria=1
fi
executavel="$pacote/Contents/MacOS/ZapDesk"
falhas=0

falhar() {
  echo "FALHOU: $*" >&2
  falhas=$((falhas + 1))
}

# Processos do ZapDesk, sem contar este script (o caminho dele também contém "zapdesk").
processos_zapdesk() {
  pgrep -fl -i zapdesk | grep -v -e "verificar-leveza" -e "^$$ " -e "^$PPID " || true
}

rss_kb() { ps -o rss= -p "$1" 2>/dev/null | tr -d ' ' || true; }

agora_ms() { python3 -c 'import time;print(int(time.time()*1000))'; }

# Processos do runner (o motor também tem "zapdesk-runner" na linha de comando: --runner-script).
runners() { pgrep -fl zapdesk-runner | grep -v -e "verificar-leveza" -e "zapdesk-motor" || true; }

json_campo() { python3 -c 'import json,sys;print(json.load(sys.stdin)[sys.argv[1]])' "$1"; }

# Soma o RSS do processo principal e de todos os descendentes (helpers do Chromium e o motor).
rss_total_kb() {
  local total=0 pid
  for pid in $(descendentes "$1") "$1"; do
    total=$((total + $(rss_kb "$pid" || echo 0)))
  done
  echo "$total"
}

descendentes() {
  local filho
  for filho in $(pgrep -P "$1"); do
    echo "$filho"
    descendentes "$filho"
  done
}

api() {
  # api MÉTODO caminho [json]
  local porta token
  porta="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["porta"])' "$PASTA_DADOS/runtime.json")"
  token="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["token"])' "$PASTA_DADOS/runtime.json")"
  curl -sf -X "$1" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' \
    "http://127.0.0.1:$porta$2" ${3:+-d "$3"}
}

[[ -x "$executavel" ]] || { echo "App empacotado não encontrado: $pacote (rode scripts/empacotar.sh)" >&2; exit 1; }

if [[ -n "$(processos_zapdesk)" ]]; then
  echo "Já há processos do ZapDesk rodando; feche o app antes de verificar:" >&2
  processos_zapdesk >&2
  exit 1
fi

echo "==> Abrindo $pacote"
echo "    pasta de dados: $PASTA_DADOS · WhatsApp: $([[ "$falso" == 1 ]] && echo falso || echo real)"
argumentos=()
[[ "$falso" == 1 ]] && argumentos+=(--whatsapp=falso)
inicio=$(agora_ms)
ZAPDESK_IA=falsa ZAPDESK_PASTA_DADOS="$PASTA_DADOS" "$executavel" "${argumentos[@]+"${argumentos[@]}"}" >/dev/null 2>&1 &
pid_app=$!

for _ in $(seq 1 600); do
  [[ -f "$PASTA_DADOS/runtime.json" ]] && break
  sleep 0.05
done
pronto_ms=$(( $(agora_ms) - inicio ))
[[ -f "$PASTA_DADOS/runtime.json" ]] || { falhar "runtime.json não apareceu em 30 s"; kill "$pid_app" 2>/dev/null; exit 1; }
pid_motor="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["pid_motor"])' "$PASTA_DADOS/runtime.json")"
echo "    app pid $pid_app · motor pid $pid_motor · pronto (runtime.json) em ${pronto_ms} ms"
((pronto_ms < 2000)) || falhar "pronto em ${pronto_ms} ms (limite 2000 ms)"
perm="$(stat -f '%Lp' "$PASTA_DADOS/runtime.json")"
[[ "$perm" == "600" ]] || falhar "runtime.json com permissão $perm (esperado 600)"

if [[ "$falso" == 1 ]]; then
  echo "==> Conectando 2 contas simuladas"
  for n in 1 2; do
    conta="$(api POST /v1/contas "{\"nome\":\"Leveza $n\"}" | python3 -c 'import json,sys;print(json.load(sys.stdin)["id"])')"
    for _ in $(seq 1 40); do
      api POST "/v1/falso/contas/$conta/escanear-qr" "{\"telefone\":\"+551190000000$n\",\"nome\":\"Leveza $n\"}" >/dev/null && break
      sleep 0.25
    done
  done
  api GET /v1/sistema | python3 -c 'import json,sys;print("    contas conectadas:", json.load(sys.stdin)["contas_conectadas"])'

  echo "==> Ativando um fluxo e um chatbot"
  fluxo="$(api POST /v1/automacoes '{"tipo":"fluxo","nome":"Leveza fluxo","gatilhos":[{"tipo":"sem_resposta","apos_s":7200}],"definicao":{"versao":1,"acoes":[{"tipo":"enviar_texto","texto":"Ainda tem interesse?"}]}}' | json_campo id)"
  api POST "/v1/automacoes/$fluxo/ativar" '{}' >/dev/null || falhar "não ativou o fluxo"
  bot="$(api POST /v1/automacoes '{"tipo":"chatbot","nome":"Leveza bot","gatilhos":[{"tipo":"palavra_chave","palavras":["preço"]}],"definicao":{"versao":1,"inicio":"n1","nao_entendi":"Não entendi.","max_tentativas":3,"inatividade_min":30,"nos":[{"id":"n1","tipo":"inicio","proximo":"n2"},{"id":"n2","tipo":"mensagem","texto":"Oi!","proximo":"n3"},{"id":"n3","tipo":"fim","mensagem":"Tchau."}]}}' | json_campo id)"
  api POST "/v1/automacoes/$bot/ativar" '{}' >/dev/null || falhar "não ativou o chatbot"
  echo "    fluxo $fluxo e chatbot $bot ativos"

  if [[ "${AUTOMACOES_IA:-1}" == 1 ]]; then
    echo "==> Automação de IA (IA falsa): runner sobe, responde e encerra por ociosidade (1 min)"
    api PATCH /v1/automacoes/configuracao '{"ociosidade_ia_min":1}' >/dev/null || falhar "não configurou a ociosidade"
    ia="$(api POST /v1/automacoes/ia '{"nome":"Leveza IA","modelo":"responder_historico"}' | json_campo id)"
    api POST "/v1/automacoes/$ia/ativar" '{}' >/dev/null || falhar "não ativou a automação de IA"
    api PUT /v1/falso/ia '{"respostas":[{"contem":"","texto":"Resposta da IA falsa."}]}' >/dev/null
    conta1="$(api GET /v1/contas | python3 -c 'import json,sys;print(json.load(sys.stdin)[0]["id"])')"
    api POST "/v1/falso/contas/$conta1/mensagem-recebida" '{"de":"+5511977770001","nome":"Leveza","texto":"Olá, tudo bem?"}' >/dev/null
    respondeu=0
    for _ in $(seq 1 60); do
      api GET /v1/falso/enviadas | grep -q "Resposta da IA falsa." && { respondeu=1; break; }
      sleep 0.25
    done
    ((respondeu)) && echo "    IA respondeu pela automação" || falhar "a automação de IA não respondeu em 15 s"
    [[ -n "$(runners)" ]] && echo "    runner vivo após a execução" || falhar "runner não apareceu no ps"
    encerrou=0
    t0=$(date +%s)
    for _ in $(seq 1 100); do
      [[ -z "$(runners)" ]] && { encerrou=1; break; }
      sleep 1
    done
    ((encerrou)) && echo "    runner encerrado por ociosidade após $(( $(date +%s) - t0 )) s" || falhar "runner continua vivo 100 s depois (ociosidade 1 min)"
    api POST "/v1/automacoes/$ia/desativar" '{}' >/dev/null
  fi
fi

echo "==> Ocioso por $espera s (amostra a cada 30 s)"
max_motor=0
max_total=0
decorrido=0
while ((decorrido < espera)); do
  passo=$((espera - decorrido < 30 ? espera - decorrido : 30))
  sleep "$passo"
  decorrido=$((decorrido + passo))
  kill -0 "$pid_motor" 2>/dev/null || { falhar "o motor morreu durante a espera"; break; }
  motor_kb=$(rss_kb "$pid_motor")
  total_kb=$(rss_total_kb "$pid_app")
  cpu_motor=$(ps -o %cpu= -p "$pid_motor" | tr -d ' ')
  ((motor_kb > max_motor)) && max_motor=$motor_kb
  ((total_kb > max_total)) && max_total=$total_kb
  printf '    %4ds  motor %6.1f MB (CPU %s%%)  ·  app inteiro %7.1f MB\n' "$decorrido" \
    "$(echo "$motor_kb/1024" | bc -l)" "$cpu_motor" "$(echo "$total_kb/1024" | bc -l)"
done
motor_final_kb=$(rss_kb "$pid_motor")
motor_final_mb=$((motor_final_kb / 1024))
echo "    motor ocioso ao fim: ${motor_final_mb} MB (pico ${max_motor} KB) · app inteiro pico $((max_total / 1024)) MB"
((motor_final_mb < limite_mb)) || falhar "motor ocioso com ${motor_final_mb} MB (limite ${limite_mb} MB)"

echo "==> Fechando o app"
fechou_em=$(agora_ms)
if [[ "${SAIR_APPLESCRIPT:-0}" == 1 ]] && osascript -e 'tell application id "com.gabriel.zapdesk" to quit' >/dev/null 2>&1; then
  echo "    evento quit enviado (como Cmd+Q)"
else
  kill -TERM "$pid_app" 2>/dev/null
  echo "    SIGTERM no processo principal"
fi
for _ in $(seq 1 50); do
  [[ -z "$(processos_zapdesk)" ]] && break
  sleep 0.1
done
restantes="$(processos_zapdesk)"
if [[ -n "$restantes" ]]; then
  falhar "processos do ZapDesk (inclui zapdesk-runner) continuam vivos 5 s depois de fechar:"
  echo "$restantes" | sed 's/^/    /' >&2
  pkill -9 -f "$pacote" 2>/dev/null
else
  echo "    pgrep -fl -i zapdesk: vazio ($(( $(agora_ms) - fechou_em )) ms para encerrar)"
fi
[[ -f "$PASTA_DADOS/runtime.json" ]] && falhar "runtime.json não foi apagado ao fechar"

((pasta_temporaria)) && rm -rf "$PASTA_DADOS"

echo
if ((falhas == 0)); then
  echo "OK: motor ocioso ${motor_final_mb} MB < ${limite_mb} MB e nenhum processo após fechar."
  exit 0
fi
echo "$falhas verificação(ões) falharam." >&2
exit 1
