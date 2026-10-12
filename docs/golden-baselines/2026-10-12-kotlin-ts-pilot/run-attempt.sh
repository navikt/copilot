#!/bin/bash
# One paid attempt: prepare, run Copilot in cplt, record usage, evaluate offline.
# Usage: run-attempt.sh <task> <model> <n> <max-credits>
set -euo pipefail
task=$1 model=$2 n=$3 cap=$4
REPO=$REPO
S=$SCRATCH
DB=$HOME/.copilot/session-store.db
slug="${task//\//-}-$model-$n"
att="$S/attempts/$slug"
python3 -B "$REPO/benchmark/realistic/prepare.py" "$task" "$att" --model "$model" --effort medium --direct-worker >/dev/null
prompt="$(cat "$att/evaluator/fixture/prompt.md")"
session="$(uuidgen | tr 'A-Z' 'a-z')"
after="$(python3 "$REPO/scripts/copilot-usage.py" cursor "$DB")"
start=$(date +%s)
set +e
(cd "$att/workspace" && cplt --preset strict --proxy-forced --no-audit --no-brief --no-allow-localhost-any \
  --project-dir "$att/workspace" --deny-path "$att/evaluator" --deny-path "$att/manifest.json" \
  --deny-path $MAIN_CHECKOUT -- \
  -p "$prompt" --allow-all-tools --no-color --log-level none --model "$model" --reasoning-effort medium \
  --session-id "$session" --max-ai-credits "$cap" --disable-builtin-mcps --no-ask-user --no-auto-update \
  --share "$att/workspace/.session-transcript.md" </dev/null >"$att/stdout.txt" 2>"$att/stderr.txt")
rc=$?
mv "$att/workspace/.session-transcript.md" "$att/transcript.md" 2>/dev/null || true
set -e
end=$(date +%s)
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
