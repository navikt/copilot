#!/bin/sh
# Test av kjørevakten uten modellkall: falsk harness, falsk session-store.db og falsk HOME.
# Kjør: sh bench-bin/run_test.sh
set -u
wt=$(cd "$(dirname "$0")/.." && pwd)
scratch=$(mktemp -d)
trap 'rm -rf "$scratch" "$wt"/bench-state/test-*' EXIT
export HOME="$scratch/home" BENCH_INTERVAL=1 BENCH_USAGE_DB="$scratch/store.db"
mkdir -p "$HOME/.copilot/session-state/s1"
echo '{"type":"session.start","data":{"copilotVersion":"1.0.114-0"}}' >"$HOME/.copilot/session-state/s1/events.jsonl"

# Falsk harness: arbeidsmappe med én bruksrad (2 credits), et barnebarn som må drepes, så venting.
cat >"$scratch/harness" <<'EOF'
#!/bin/sh
w="$TMPDIR/nav-pilot-golden.test"; mkdir -p "$w"
echo "t2|1|s1|1|0|m|low|0|0|0|0|0|2000000000|0|0|stop||" >"$w/usage.psv"
echo "t2|1|0|pass|1|1|${FAKE_USAGE_COMPLETE:-true}|" >"$w/attempts.psv"
sh -c 'sleep 300' & echo $! >"$TMPDIR/grandchild.pid"
sleep "${FAKE_SECONDS:-300}"
EOF
chmod +x "$scratch/harness"
export BENCH_HARNESS="$scratch/harness"

db() {
  rm -f "$BENCH_USAGE_DB"
  sqlite3 "$BENCH_USAGE_DB" "create table sessions (id text, cwd text, created_at text); create table assistant_usage_events (session_id text, total_nano_aiu integer);"
  [ "$1" = probe ] && sqlite3 "$BENCH_USAGE_DB" "insert into sessions values ('p', '$wt', '2999-01-01T00:00:00.000Z'); insert into assistant_usage_events values ('p', 3170000000);"
}

wait_done() { i=0; while [ ! -f "$1/done" ] && [ "$i" -lt 60 ]; do sleep 1; i=$((i + 1)); done; }
fail=0
check() { if eval "$2"; then echo "ok   $1"; else echo "FEIL $1"; fail=1; fi; }

db probe
r=$(FAKE_SECONDS=4 sh "$wt/bench-bin/run.sh" test-ok 100); wait_done "$r"
check "fullført arm stoppes ikke" '[ ! -f "$r/stopped" ]'
check "summen tar med proben (2 + 3,17)" 'grep -q "|5.17$" "$r/spent"'
check "barnebarnet er borte når armen er ferdig" '! kill -0 "$(cat "$r/tmp/grandchild.pid")" 2>/dev/null'

db none
r=$(sh "$wt/bench-bin/run.sh" test-noprobe 100); wait_done "$r"
check "manglende probe gir stopp" 'grep -q "ukjent bruk" "$r/stopped"'
check "stopp dreper hele gruppen" '! kill -0 "$(cat "$r/tmp/grandchild.pid")" 2>/dev/null && ! kill -0 "$(cat "$r/harness.pid")" 2>/dev/null'

db probe
r=$(FAKE_USAGE_COMPLETE=false sh "$wt/bench-bin/run.sh" test-incomplete 100); wait_done "$r"
check "usage_complete=false gir stopp" 'grep -q "usage_complete=false" "$r/stopped"'

db probe
r=$(sh "$wt/bench-bin/run.sh" test-limit 4); wait_done "$r"
check "forbruk over grensen gir stopp" 'grep -q "forbruk 5.17 > 4" "$r/stopped"'

exit "$fail"
