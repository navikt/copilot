#!/bin/bash
# usage: run.sh <label>   (uses isolated COPILOT_HOME; parent on gpt-6-luna)
S=${S:?set S to a scratch dir containing home/ and work/}
cd "$S/work" || exit 1
COPILOT_HOME=$S/home COPILOT_GITHUB_TOKEN=$(cat $S/.tok) timeout 400 copilot --agent probe-parent --model gpt-6-luna \
  -p "start the subagent probe-child" --allow-all-tools --log-level debug \
  --log-dir $S/logs-$1 --share $S/$1.md > $S/$1.out 2>&1
grep -E 'AI Credits' $S/$1.out
grep -rhoE 'turn tool surface resolved \{"model":"[^"]*"' $S/logs-$1 | sort | uniq -c
grep -ho '(model: [^)]*)' $S/$1.md $S/$1.out | sort -u
