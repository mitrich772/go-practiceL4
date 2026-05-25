#!/usr/bin/env bash
# compare_with_grep.sh — построчно сравнивает вывод mygrep с системным grep
# на нескольких тестовых паттернах. Скрипт сам поднимает 3 сервера, гоняет
# серию запросов и в конце печатает результат: PASS / FAIL.
#
# Использование: ./scripts/compare_with_grep.sh [-v]
#   -v  печатать diff при расхождении.

set -euo pipefail

VERBOSE=0
[[ "${1:-}" == "-v" ]] && VERBOSE=1

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/bin"
DATA="$ROOT/examples/data/access.log"
WORDS="$ROOT/examples/data/words.txt"
N=3
BASE_PORT=${BASE_PORT:-9201}

mkdir -p "$BIN"
echo ">> building binaries..."
go build -C "$ROOT" -o "$BIN/mygrep-server" ./cmd/mygrep-server >/dev/null
go build -C "$ROOT" -o "$BIN/mygrep"        ./cmd/mygrep        >/dev/null

PIDS=()
cleanup() {
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  done
}
trap cleanup EXIT

SERVERS=""
for i in $(seq 0 $((N - 1))); do
  PORT=$((BASE_PORT + i))
  "$BIN/mygrep-server" -addr ":$PORT" >/tmp/mygrep-compare-$PORT.log 2>&1 &
  PIDS+=("$!")
  if [[ -n "$SERVERS" ]]; then SERVERS+=","; fi
  SERVERS+="127.0.0.1:$PORT"
done

# Дождёмся готовности.
for addr in ${SERVERS//,/ }; do
  for _ in $(seq 1 50); do
    if curl -fs "http://$addr/healthz" >/dev/null 2>&1; then break; fi
    sleep 0.1
  done
done

PASS=0
FAIL=0

# expected — вывод системного grep с теми же флагами.
# actual   — вывод mygrep.
# Сравниваем построчно (mygrep печатает строки в исходном порядке).
run_case() {
  local title="$1"; shift
  local file="$1";  shift
  local grep_flags="$1"; shift
  local mygrep_flags="$1"; shift
  local pattern="$1"

  # Считаем эталон системным grep'ом. Включаем set +e: grep с no-match
  # возвращает 1, и это нормально.
  set +e
  local expected
  expected=$(grep $grep_flags -- "$pattern" "$file")
  local exp_code=$?
  local actual
  actual=$("$BIN/mygrep" --servers "$SERVERS" $mygrep_flags -e "$pattern" "$file")
  local act_code=$?
  set -e

  if [[ "$expected" == "$actual" ]]; then
    printf "  [PASS] %-45s exit=(grep=%d mygrep=%d)\n" "$title" "$exp_code" "$act_code"
    PASS=$((PASS + 1))
  else
    printf "  [FAIL] %-45s\n" "$title"
    if [[ $VERBOSE -eq 1 ]]; then
      echo "    --- expected (grep) ---"
      echo "$expected" | sed 's/^/      /'
      echo "    --- actual (mygrep) ---"
      echo "$actual"   | sed 's/^/      /'
    fi
    FAIL=$((FAIL + 1))
  fi
}

echo
echo "== Сравнение mygrep vs системного grep =="
echo "   data: $DATA"
echo "   servers: $SERVERS"
echo

# Сюита тестов: каждый кейс — (название, файл, флаги-grep, флаги-mygrep, паттерн)
run_case "fixed ERROR (default)"          "$DATA"  "-F"     "-F"     "ERROR"
run_case "fixed ERROR -n"                 "$DATA"  "-Fn"    "-Fn"    "ERROR"
run_case "fixed ERROR -c"                 "$DATA"  "-Fc"    "-Fc"    "ERROR"
run_case "fixed status=200 -v"            "$DATA"  "-Fv"    "-Fv"    "status=200"
run_case "regex ^2025-05-25T10:00:0[1-3]" "$DATA"  "-E"     ""       "^2025-05-25T10:00:0[1-3]"
run_case "regex status=(500|502) -n"      "$DATA"  "-En"    "-n"     "status=(500|502)"
run_case "ignore case 'error' -i"         "$DATA"  "-Fi"    "-Fi"    "error"
run_case "no matches (DOESNOTEXIST)"      "$DATA"  "-F"     "-F"     "DOESNOTEXIST"

run_case "words fixed 'lima' -i"          "$WORDS" "-Fi"    "-Fi"    "lima"
run_case "words regex ^[A-Z]"             "$WORDS" "-E"     ""       "^[A-Z]"
run_case "words count letters -c"         "$WORDS" "-Ec"    "-c"     "."

echo
echo "== Итог: PASS=$PASS  FAIL=$FAIL =="
if [[ $FAIL -gt 0 ]]; then
  exit 1
fi
