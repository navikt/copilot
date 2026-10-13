#!/bin/bash
# Round 2: one paid attempt. Prepare, preinstall npm deps outside the sandbox,
# run Copilot in cplt with Gradle/npm usable, record usage, evaluate offline.
# Usage: run-attempt.sh <task> <model> <n> <max-credits>
set -euo pipefail
task=$1 model=$2 n=$3 cap=$4
REPO=$REPO
S=$SCRATCH
# cplt denies execute under /tmp and ~/.cache, so attempts live in the home directory.
A=$HOME/nav-benchmark-attempts/r2
DB=$HOME/.copilot/session-store.db
export JAVA_HOME=$HOME/.local/share/mise/installs/java/21.0.2
slug="${task//\//-}-$model-$n"
att="$A/$slug"
mkdir -p "$A"
python3 -B "$REPO/benchmark/realistic/prepare.py" "$task" "$att" --model "$model" --effort medium --direct-worker >/dev/null
if [ -f "$att/workspace/package-lock.json" ]; then
  # GitHub Packages needs a token; install here so the agent never sees it.
  (cd "$att/workspace" && npm ci --ignore-scripts --no-audit --no-fund >/dev/null 2>&1)
fi
prompt="$(cat "$att/evaluator/fixture/prompt.md")"
session="$(uuidgen | tr 'A-Z' 'a-z')"
after="$(python3 "$REPO/scripts/copilot-usage.py" cursor "$DB")"
daemons_before="$(pgrep -f 'GradleDaemon 8.14.5' | sort || true)"
start=$(date +%s)
set +e
(cd "$att/workspace" && cplt --preset strict --no-proxy-forced --with-proxy --default-allowlist --allow-localhost-any \
  --pass-env JAVA_HOME --no-audit --no-brief \
  --project-dir "$att/workspace" --deny-path "$att/evaluator" --deny-path "$att/manifest.json" \
  --deny-path $MAIN_CHECKOUT -- \
  -p "$prompt" --allow-all-tools --no-color --log-level none --model "$model" --reasoning-effort medium \
  --session-id "$session" --max-ai-credits "$cap" --disable-builtin-mcps --no-ask-user --no-auto-update \
  --share "$att/workspace/.session-transcript.md" </dev/null >"$att/stdout.txt" 2>"$att/stderr.txt")
rc=$?
set -e
end=$(date +%s)
mv "$att/workspace/.session-transcript.md" "$att/transcript.md" 2>/dev/null || true
# Stop only Gradle daemons this attempt started, by PID.
for pid in $(comm -13 <(echo "$daemons_before") <(pgrep -f 'GradleDaemon 8.14.5' | sort || true)); do kill "$pid" 2>/dev/null || true; done
python3 "$REPO/scripts/copilot-usage.py" export "$DB" --session "$session" --after "$after" --slug "$slug" --run "$n" >"$att/usage.psv"
if [ ! -s "$att/usage.psv" ]; then
  # Unverified usage must never become a result or a known cost.
  echo "$slug|$session|$rc|$((end-start))||usage-missing" | tee -a "$S/attempts.psv"
  exit 1
fi
credits=$(awk -F'|' '{s+=$13} END {printf "%.6f", s/1e9}' "$att/usage.psv")
python3 -B "$REPO/benchmark/realistic/check.py" --attempt "$att" --output "$att/result.json" >/dev/null
status=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["status"])' "$att/result.json")
echo "$slug|$session|$rc|$((end-start))|$credits|$status" | tee -a "$S/attempts.psv"
