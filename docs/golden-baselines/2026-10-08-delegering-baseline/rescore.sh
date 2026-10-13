#!/bin/bash
# Rescores the baseline with the agent_id rule from the usage rows (column 17
# non-empty = a subagent turn). Run from this directory; writes rescore.psv.
# d1 counts only subagent rows in d1b/d1c; d3 also needs at least one row.
cd "$(dirname "$0")" || exit 1
echo "test|run|usage_rows|subagent_rows|status" >rescore.psv
for f in d1r1 d1r2 d1r3 d1r4 d1r5 d2 d3 d4; do
  grep -v '^#' "$f-usage.psv" | awk -F'|' -v f="$f" '
    { t = ($1 ~ /^d1/) ? "d1" : $1; run = (f ~ /^d1r/) ? substr(f, 4) : $2
      k = t "|" run; n[k]++
      if ($17 != "" && !(t == "d1" && $1 == "d1a")) s[k]++ }
    END { for (k in n) {
      split(k, a, "|"); st = (a[1] == "d3") ? ((s[k] + 0 == 0 && n[k] >= 1) ? "pass" : "fail") : ((s[k] + 0 >= 1) ? "check-names" : "fail")
      print k "|" n[k] "|" s[k] + 0 "|" st } }'
done | sort >>rescore.psv
cat rescore.psv
