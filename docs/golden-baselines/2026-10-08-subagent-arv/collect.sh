#!/bin/bash
# Extracts the per-run evidence from the debug logs and session exports.
# Columns: run|ai_credits|turns per model (debug log)|subagent label (session export)|fallback warnings
S=/private/tmp/claude-501/-Users-hans-go-src-github-com-navikt-copilot/614d7367-cac8-4f51-bd69-362d0a336457/scratchpad/subarv
D=/Users/hans/go/src/github.com/navikt/copilot/.claude/worktrees/agent-ace943931b86f4cde/docs/golden-baselines/2026-10-08-subagent-arv
cd $S
mkdir -p $D/agents
cp home/agents/probe-parent.agent.md $D/agents/
cp probe-child-opus.agent.md $D/agents/probe-child.opus.agent.md
cp home/agents/probe-child.agent.md $D/agents/probe-child.luna.agent.md
cp run.sh collect.sh $D/
echo "run|ai_credits|turns_per_model_debuglog|subagent_label_export|fallback_warnings" > $D/raw.psv
for r in direct A1 B1 B2 B3 C1 C2 D1 D2 E1 E2 C3 C4 D3 D4 E3 E4; do
  c=$(grep -ho 'AI Credits [0-9.]*' $r.out 2>/dev/null | awk '{print $3}')
  [ $r = direct ] && c=12.19
  t=$(grep -rhoE 'turn tool surface resolved \{"model":"[^"]*"' logs-$r | sed 's/.*"model":"//;s/"//' | sort | uniq -c | awk '{printf "%sx%s ",$1,$2}')
  u=$(grep -ho "(model: [^)]*)" $r.md $r.out 2>/dev/null | head -1)
  w=$(cat logs-$r/* | grep -c 'did not commit a new selection')
  echo "$r|$c|$t|$u|$w" >> $D/raw.psv
done
grep -rh 'did not commit' logs-D1 | sed 's/^.*\[WARNING\]/[WARNING]/' > $D/D1-fallback-warning.txt
