#!/bin/bash
# usage: run.sh <suite> <only> <name> — 5 runs, GPT-6 Sol Low, kept.
D=/private/tmp/claude-501/-Users-hans-go-src-github-com-navikt-copilot/614d7367-cac8-4f51-bd69-362d0a336457/scratchpad/steg3
R=/Users/hans/go/src/github.com/navikt/copilot/.claude/worktrees/agent-a019b5daec8d9a445
export GRADLE_OPTS="-Dorg.gradle.daemon=false -Dkotlin.compiler.execution.strategy=in-process"
export KOTLIN_COMPILER_EXECUTION_STRATEGY=in-process
cd "$R" || exit 1
s=(); [[ $1 != planning ]] && s=(--suite "$1")
TMPDIR=$D NAV_PILOT_GOLDEN_TIMEOUT=600 ./scripts/nav-pilot-golden.sh "${s[@]}" --only "$2" \
  --model gpt-6-sol --effort low --repeat 5 --keep --save-baseline "$D/$3.txt" >"$D/$3.log" 2>&1
echo "$3 rc=$?"
grep -E "^  (✓|✗|⚠)|kept in" "$D/$3.log"
awk -F'|' '!/^#/{c[$2]+=$13/1e9; m[$2"|"$6]+=$13/1e9} END {for (k in m) printf "run|model %s %.2f\n", k, m[k]; for (r in c) printf "run %s %.2f\n", r, c[r]}' "$D/$3-usage.psv" | sort
