#!/bin/bash
# Adds the rev2 runs (d2, d3, d4, planning p1-p3) to docs/golden-baselines/2026-10-08-delegering-steg3.
D=/private/tmp/claude-501/-Users-hans-go-src-github-com-navikt-copilot/614d7367-cac8-4f51-bd69-362d0a336457/scratchpad/steg3
OUT=/Users/hans/go/src/github.com/navikt/copilot/.claude/worktrees/agent-a019b5daec8d9a445/docs/golden-baselines/2026-10-08-delegering-steg3
mkdir -p "$OUT/rev2/transkripter"
cp "$D/run.sh" "$D/collect2.sh" "$OUT/"
for t in d2 d3 d4 p1 p2 p3; do
  cp "$D/$t.txt" "$D/$t-results.psv" "$D/$t-usage.psv" "$D/$t-attempts.psv" "$OUT/rev2/"
  for j in "$D/$t-judge.psv"; do [[ -f $j ]] && cp "$j" "$OUT/rev2/"; done
  grep -v '^#' "$D/$t-results.psv" | while IFS='|' read -r id run status a b; do
    det=$b; [[ -z $b ]] && det=$a
    r=$run; [[ $t == p* ]] && r=${t#p}
    cred=$(awk -F'|' -v r="$run" -v id="$id" '$2 == r && $1 !~ /^#/ && ($1 == id || id !~ /^d/) { c[$6] += $13 / 1e9 } END { for (m in c) printf "%s:%.2f ", m, c[m] }' "$D/$t-usage.psv")
    [[ $t == p* ]] && cred="(pass total) $cred"
    echo "rev2|$id|$r|$status|$cred|$det"
  done >>"$OUT/raw.psv"
  k=$(sed -n 's/^transcripts kept in //p' "$D/$t.log")
  for x in "$k"/*.run*.txt; do
    case "$x" in *.svar.txt) ;; *) b=$(basename "$x"); [[ $t == p* ]] && b=${b/run1/run${t#p}}; cp "$x" "$OUT/rev2/transkripter/$b" ;; esac
  done
done
