#!/bin/sh
# Én arm, frakoblet: bench-bin/run.sh <navn> <grense i credits> <argumenter til nav-pilot-golden.sh>
# Vaktsløyfen stopper armen ved forbruk over grensen (regel 5), ukjent bruk,
# feil klientversjon (oppgave eller dommer) eller last over 8 to minutter på rad.
set -u
name=$1 limit=$2; shift 2
wt=$(cd "$(dirname "$0")/.." && pwd)
run="$wt/bench-state/$name"
mkdir -p "$run/tmp"
export PATH="$wt/bench-bin:$PATH" TMPDIR="$run/tmp" BENCH_VERSION_LOG="$run/judge-versions.psv"
: >"$BENCH_VERSION_LOG"
cd "$wt" || exit 1

nohup ./scripts/nav-pilot-golden.sh --keep "$@" >"$run/harness.log" 2>&1 &
hp=$!
echo "$hp" >"$run/harness.pid"

(
  high=0
  while kill -0 "$hp" 2>/dev/null; do
    sleep 60
    w=$(ls -d "$run"/tmp/nav-pilot-golden.* 2>/dev/null | head -1)
    reason=""
    if [ -n "$w" ]; then
      # awk gir ingen sum hvis en fil mangler; judge-credits.psv finnes først etter dommerkontrollene.
      spent=$(cat "$w/usage.psv" /dev/null; [ -f "$w/judge-credits.psv" ] && sed 's/^/J|/' "$w/judge-credits.psv")
      spent=$(printf '%s\n' "$spent" | awk -F'|' '$1 == "J" {if ($4 == "unknown") u=1; else s += $4; next} NF >= 13 {if ($13 == "unknown") u=1; else s += $13 / 1e9} END {print u ? "unknown" : s+0}')
      echo "$(date -u +%FT%TZ)|${spent:-0}" >"$run/spent"
      [ "$spent" = unknown ] && reason="ukjent bruk"
      [ -z "$reason" ] && awk -v s="${spent:-0}" -v l="$limit" 'BEGIN{exit !(s > l)}' && reason="forbruk $spent > $limit"
      for sid in $(cut -d'|' -f3 "$w/usage.psv" 2>/dev/null | sort -u); do
        v=$(head -1 "$HOME/.copilot/session-state/$sid/events.jsonl" 2>/dev/null | grep -o '"copilotVersion":"[^"]*"' | cut -d'"' -f4)
        [ "$v" = 1.0.114-0 ] || reason="oppgaveøkt $sid på '${v}'"
      done
    fi
    bad=$(cut -d'|' -f2 "$BENCH_VERSION_LOG" | grep -v -x 1.0.114-0 | head -1)
    [ -n "$bad" ] && reason="dommerøkt på $bad"
    l1=$(sysctl -n vm.loadavg | awk '{print int($2)}')
    if [ "$l1" -gt 8 ]; then high=$((high + 1)); else high=0; fi
    [ "$high" -ge 2 ] && reason="last $l1"
    if [ -n "$reason" ]; then
      echo "$reason" >"$run/stopped"
      pkill -TERM -P "$hp"; kill -TERM "$hp"
      break
    fi
  done
  echo "$(date -u +%FT%TZ)" >"$run/done"
) >"$run/watch.log" 2>&1 &
echo "$!" >"$run/watch.pid"
echo "$run"
