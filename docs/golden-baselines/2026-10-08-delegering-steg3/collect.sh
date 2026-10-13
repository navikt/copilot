#!/bin/bash
# Copies the steg-3 runs into docs/golden-baselines/<dir>. rev0 = first persona, rev1 = after the one revision.
D=/private/tmp/claude-501/-Users-hans-go-src-github-com-navikt-copilot/614d7367-cac8-4f51-bd69-362d0a336457/scratchpad/steg3
OUT=/Users/hans/go/src/github.com/navikt/copilot/.claude/worktrees/agent-a019b5daec8d9a445/docs/golden-baselines/2026-10-08-delegering-steg3
mkdir -p "$OUT"
cp "$D/run.sh" "$D/collect.sh" "$OUT/"
echo "revision|test|run|status|credits_per_model|detail" >"$OUT/raw.psv"
for rev in rev0 rev1; do
  src=$D; [[ $rev == rev0 ]] && src=$D/rev0
  mkdir -p "$OUT/$rev/transkripter"
  for f in "$src"/d?-results.psv; do
    t=$(basename "$f" -results.psv)
    cp "$src/$t.txt" "$src/$t-results.psv" "$src/$t-usage.psv" "$src/$t-attempts.psv" "$OUT/$rev/"
    grep -v '^#' "$f" | while IFS='|' read -r id run status a b; do
      det=$b; [[ -z $b ]] && det=$a
      cred=$(awk -F'|' -v r="$run" '$2 == r && $1 !~ /^#/ { c[$6] += $13 / 1e9 } END { for (m in c) printf "%s:%.2f ", m, c[m] }' "$src/$t-usage.psv")
      echo "$rev|$id|$run|$status|$cred|$det"
    done >>"$OUT/raw.psv"
    k=$(sed -n 's/^transcripts kept in //p' "$src/$t.log")
    [[ $rev == rev0 ]] && k=$src/$(basename "$k")
    for x in "$k"/d*.run*.txt; do case "$x" in *.svar.txt) ;; *) cp "$x" "$OUT/$rev/transkripter/" ;; esac; done
  done
done
