#!/bin/sh
# Én arm, frakoblet: bench-bin/run.sh <navn> <grense i credits> <argumenter til nav-pilot-golden.sh>
# Vaktsløyfen stopper armen ved forbruk over grensen (regel 5), ukjent eller ufullstendig bruk
# (også proben), feil klientversjon (oppgave eller dommer) eller 5-minutters last over 8
# (eierbeslutning 20. november).
# BENCH_HARNESS, BENCH_INTERVAL og BENCH_USAGE_DB finnes for testen (bench-bin/run_test.sh).
set -u
name=$1 limit=$2; shift 2
wt=$(cd "$(dirname "$0")/.." && pwd)
run="$wt/bench-state/$name"
db="${BENCH_USAGE_DB:-$HOME/.copilot/session-store.db}"
interval="${BENCH_INTERVAL:-60}"
mkdir -p "$run/tmp"
export PATH="$wt/bench-bin:$PATH" TMPDIR="$run/tmp" BENCH_VERSION_LOG="$run/judge-versions.psv"
: >"$BENCH_VERSION_LOG"
rm -f "$run/stopped" "$run/done" "$run/spent"
cd "$wt" || exit 1
start=$(date -u +%Y-%m-%dT%H:%M:%S)

# Egen prosessgruppe (macOS har ikke setsid(1)), så stopp når også dommeren og copilot-barna.
nohup python3 -c 'import os, sys; os.setsid(); os.execvp(sys.argv[1], sys.argv[1:])' \
  "${BENCH_HARNESS:-./scripts/nav-pilot-golden.sh}" --keep "$@" >"$run/harness.log" 2>&1 &
hp=$!
echo "$hp" >"$run/harness.pid"

(
  while kill -0 "$hp" 2>/dev/null; do
    sleep "$interval"
    w=$(ls -d "$run"/tmp/nav-pilot-golden.* 2>/dev/null | head -1)
    reason=""
    if [ -n "$w" ]; then
      # Proben kjøres fra worktreen før arbeidsmappa finnes og står bare i session-store.db.
      probe=$(sqlite3 -readonly "file:$db?mode=ro" "select count(*), sum(total_nano_aiu is null), coalesce(sum(total_nano_aiu), 0) from assistant_usage_events u join sessions s on s.id = u.session_id where s.cwd = '$wt' and s.created_at >= '$start'" 2>/dev/null)
      # awk gir ingen sum hvis en fil mangler; judge-credits.psv finnes først etter dommerkontrollene.
      spent=$(echo "P|$probe"; cat "$w/usage.psv" /dev/null 2>/dev/null; [ -f "$w/judge-credits.psv" ] && sed 's/^/J|/' "$w/judge-credits.psv")
      spent=$(printf '%s\n' "$spent" | awk -F'|' '
        $1 == "P" {if ($2 == "" || $2 == 0 || $3 > 0) u=1; else s += $4 / 1e9; next}
        $1 == "J" {if ($4 == "unknown") u=1; else s += $4; next}
        NF >= 13 {if ($13 == "unknown") u=1; else s += $13 / 1e9}
        END {print u ? "unknown" : s+0}')
      echo "$(date -u +%FT%TZ)|${spent:-unknown}" >"$run/spent"
      [ "${spent:-unknown}" = unknown ] && reason="ukjent bruk (probe, oppgave eller dommer)"
      awk -F'|' '$7 == "false" {f=1} END {exit !f}' "$w/attempts.psv" 2>/dev/null && reason="usage_complete=false i attempts.psv"
      [ -z "$reason" ] && awk -v s="$spent" -v l="$limit" 'BEGIN{exit !(s > l)}' && reason="forbruk $spent > $limit"
      for sid in $(cut -d'|' -f3 "$w/usage.psv" 2>/dev/null | sort -u); do
        v=$(head -1 "$HOME/.copilot/session-state/$sid/events.jsonl" 2>/dev/null | grep -o '"copilotVersion":"[^"]*"' | cut -d'"' -f4)
        [ "$v" = 1.0.114-0 ] || reason="oppgaveøkt $sid på '${v}'"
      done
    fi
    bad=$(cut -d'|' -f2 "$BENCH_VERSION_LOG" | grep -v -x 1.0.114-0 | head -1)
    [ -n "$bad" ] && reason="dommerøkt på $bad"
    l5=$(sysctl -n vm.loadavg | awk '{print $3}')
    awk -v l="$l5" 'BEGIN{exit !(l > 8)}' && reason="5-minutters last $l5"
    if [ -n "$reason" ]; then
      echo "$reason" >"$run/stopped"
      kill -TERM -- "-$hp" 2>/dev/null
      break
    fi
  done
  # Ferdig først når hele gruppen er borte, så ingen dommer eller copilot bruker credits etterpå.
  i=0
  while pgrep -g "$hp" >/dev/null 2>&1; do
    i=$((i + 1))
    [ "$i" -eq 30 ] && kill -KILL -- "-$hp" 2>/dev/null
    sleep 1
  done
  echo "$(date -u +%FT%TZ)" >"$run/done"
) >"$run/watch.log" 2>&1 &
echo "$!" >"$run/watch.pid"
echo "$run"
