#!/bin/bash
# usage: run.sh <test id>   — 5 runs of one delegation test, GPT-6 Sol Low, kept.
D=/private/tmp/claude-501/-Users-hans-go-src-github-com-navikt-copilot/614d7367-cac8-4f51-bd69-362d0a336457/scratchpad/delegering
R=/Users/hans/go/src/github.com/navikt/copilot/.claude/worktrees/agent-ae4b12b560f4efcc0
cd "$R" || exit 1
TMPDIR=$D NAV_PILOT_GOLDEN_TIMEOUT=600 ./scripts/nav-pilot-golden.sh --suite delegation --only "$1" \
  --model gpt-6-sol --effort low --repeat 5 --keep --save-baseline "$D/$1.txt" >"$D/$1.log" 2>&1
echo "rc=$?"
grep -E "^  (✓|✗|⚠)|kept in" "$D/$1.log"
awk -F'|' '{c[$2]+=$13/1e9; m[$2"|"$6]+=$13/1e9} END {for (k in m) printf "run|model %s %.2f\n", k, m[k]; for (r in c) printf "run %s %.2f\n", r, c[r]}' "$D/$1-usage.psv" | sort
