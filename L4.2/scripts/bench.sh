#!/usr/bin/env bash
# bench.sh — простой замер времени работы mygrep против системного grep.
#
# Генерирует синтетический файл из ~200k строк, поднимает кластер из 3
# серверов и сравнивает время поиска тяжёлым регуляркой.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/bin"
TMP="$(mktemp -d)"
DATA="$TMP/big.txt"
N=3
BASE_PORT=${BASE_PORT:-9301}

mkdir -p "$BIN"
go build -C "$ROOT" -o "$BIN/mygrep-server" ./cmd/mygrep-server >/dev/null
go build -C "$ROOT" -o "$BIN/mygrep"        ./cmd/mygrep        >/dev/null

echo ">> generating sample data (200000 lines) at $DATA"
awk 'BEGIN {
  srand(42);
  levels[0]="INFO"; levels[1]="WARN"; levels[2]="ERROR"; levels[3]="DEBUG";
  for (i = 0; i < 200000; i++) {
    lvl = levels[int(rand()*4)];
    status = (rand() < 0.05) ? 500 : 200;
    printf "2025-05-25T10:%02d:%02dZ %-5s service=api request=/path/%d status=%d latency_ms=%d\n",
      int(rand()*60), int(rand()*60), lvl, i, status, int(rand()*1500);
  }
}' > "$DATA"

PIDS=()
cleanup() {
  for pid in "${PIDS[@]:-}"; do kill "$pid" 2>/dev/null || true; done
  rm -rf "$TMP"
}
trap cleanup EXIT

SERVERS=""
for i in $(seq 0 $((N - 1))); do
  PORT=$((BASE_PORT + i))
  "$BIN/mygrep-server" -addr ":$PORT" >/dev/null 2>&1 &
  PIDS+=("$!")
  if [[ -n "$SERVERS" ]]; then SERVERS+=","; fi
  SERVERS+="127.0.0.1:$PORT"
done
for addr in ${SERVERS//,/ }; do
  for _ in $(seq 1 50); do
    if curl -fs "http://$addr/healthz" >/dev/null 2>&1; then break; fi
    sleep 0.1
  done
done

PATTERN="status=500"

echo
echo "== system grep =="
time grep -c -- "$PATTERN" "$DATA"

echo
echo "== mygrep (3 servers, quorum=2) =="
time "$BIN/mygrep" --servers "$SERVERS" -F -c -e "$PATTERN" "$DATA"
