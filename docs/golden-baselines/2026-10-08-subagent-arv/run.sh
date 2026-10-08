#!/bin/bash
# usage: run.sh <label>   (uses isolated COPILOT_HOME; parent on gpt-6-luna)
S=/private/tmp/claude-501/-Users-hans-go-src-github-com-navikt-copilot/614d7367-cac8-4f51-bd69-362d0a336457/scratchpad/subarv
cd $S/work
COPILOT_HOME=$S/home COPILOT_GITHUB_TOKEN=$(cat $S/.tok) timeout 400 copilot --agent probe-parent --model gpt-6-luna \
  -p "start the subagent probe-child" --allow-all-tools --log-level debug \
  --log-dir $S/logs-$1 --share $S/$1.md > $S/$1.out 2>&1
grep -E 'AI Credits' $S/$1.out
grep -rhoE 'turn tool surface resolved \{"model":"[^"]*"' $S/logs-$1 | sort | uniq -c
grep -ho '(model: [^)]*)' $S/$1.md $S/$1.out | sort -u
