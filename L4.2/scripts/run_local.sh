#!/usr/bin/env bash
# run_local.sh — поднимает локальный кластер из 3 mygrep-server и
# демонстрирует поиск через mygrep.
#
# Использование:
#   ./scripts/run_local.sh                    # стандартный поиск ERROR
#   PATTERN="status=500" ./scripts/run_local.sh
#   N=5 PATTERN="ERROR" ./scripts/run_local.sh
#
# Скрипт самостоятельно собирает бинарники, поднимает сервера, ждёт их
# готовности (/healthz), запускает mygrep и останавливает сервера.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/bin"
DATA="${DATA:-$ROOT/examples/data/access.log}"
PATTERN="${PATTERN:-ERROR}"
N="${N:-3}"
BASE_PORT="${BASE_PORT:-9101}"

mkdir -p "$BIN"
echo ">> building binaries..."
go build -C "$ROOT" -o "$BIN/mygrep-server" ./cmd/mygrep-server
go build -C "$ROOT" -o "$BIN/mygrep"        ./cmd/mygrep

PIDS=()
cleanup() {
  echo
  echo ">> stopping servers..."
  for pid in "${PIDS[@]:-}"; do
    if kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
  done
}
trap cleanup EXIT

SERVERS=""
for i in $(seq 0 $((N - 1))); do
  PORT=$((BASE_PORT + i))
  ADDR="127.0.0.1:$PORT"
  echo ">> starting server $((i + 1))/$N on $ADDR"
  "$BIN/mygrep-server" -addr ":$PORT" >/tmp/mygrep-server-$PORT.log 2>&1 &
  PIDS+=("$!")
  if [[ -n "$SERVERS" ]]; then SERVERS+=","; fi
  SERVERS+="$ADDR"
done

echo ">> waiting for servers to become healthy..."
for addr in ${SERVERS//,/ }; do
  for _ in $(seq 1 50); do
    if curl -fs "http://$addr/healthz" >/dev/null 2>&1; then break; fi
    sleep 0.1
  done
done

echo
echo "=========================================================="
echo " Распределённый поиск: pattern='$PATTERN'  file='$DATA'"
echo " Servers: $SERVERS  (quorum = $(( N / 2 + 1 )))"
echo "=========================================================="
"$BIN/mygrep" --servers "$SERVERS" -e "$PATTERN" -n "$DATA"
echo "=========================================================="
echo "Exit code: $?"
