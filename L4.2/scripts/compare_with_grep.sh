#!/usr/bin/env bash
# compare_with_grep.sh — построчно сравнивает вывод mygrep с системным grep
# на нескольких тестовых паттернах.
#
# Скрипт сам:
#   - собирает бинарь mygrep,
#   - поднимает 3 сервера через docker compose и ждёт их healthy,
#   - гоняет суит запросов и сверяет каждый с системным grep,
#   - в конце гасит сервера.
#
# Использование:
#   ./scripts/compare_with_grep.sh        # короткий отчёт PASS/FAIL
#   ./scripts/compare_with_grep.sh -v     # дополнительно показывает diff

set -euo pipefail

VERBOSE=0
[[ "${1:-}" == "-v" ]] && VERBOSE=1

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/bin"
DATA="$ROOT/examples/data/access.log"
WORDS="$ROOT/examples/data/words.txt"
SERVERS="127.0.0.1:9101,127.0.0.1:9102,127.0.0.1:9103"

mkdir -p "$BIN"
echo ">> building mygrep client..."
go build -C "$ROOT" -o "$BIN/mygrep" ./cmd/mygrep >/dev/null

cleanup() {
  echo ">> stopping docker compose stack"
  (cd "$ROOT" && docker compose down -v >/dev/null 2>&1 || true)
}
trap cleanup EXIT

echo ">> starting 3 servers via docker compose..."
(cd "$ROOT" && docker compose up -d --wait --build server1 server2 server3 >/dev/null)

PASS=0
FAIL=0

run_case() {
  local title="$1"; shift
  local file="$1";  shift
  local grep_flags="$1"; shift
  local mygrep_flags="$1"; shift
  local pattern="$1"

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
echo "   servers: $SERVERS"
echo

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
