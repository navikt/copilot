#!/bin/bash
# Collects the delegation baseline into docs/golden-baselines/<dir>.
# D: scratch dir with the kept runs (run.sh / run1.sh); OUT: the docs dir.
D=${D:?}; OUT=${OUT:?}
mkdir -p "$OUT"
cd "$D" || exit 1
cp run.sh run1.sh collect.sh "$OUT/"
for f in d2 d3 d4 d1r1 d1r2 d1r3 d1r4 d1r5; do
  cp "$f.txt" "$f-results.psv" "$f-usage.psv" "$f-attempts.psv" "$OUT/" 2>/dev/null
done
# raw.psv: one row per test run. Credits per model from the usage rows,
# turn lines per model from the debug log (`turn tool surface resolved`).
echo "test|run|status|credits_per_model|debuglog_turns_per_model|subagents_started|fase3_in_d1c" >"$OUT/raw.psv"
for k in nav-pilot-golden.??????; do
  for t in "$k"/d*.run*.txt; do
    case "$t" in *.svar.txt) continue ;; esac
    s=$(basename "$t" .txt); slug=${s%.run*}; run=${s##*.run}
    test=${slug%[abc]}
    [[ "$slug" == d1a || "$slug" == d1b ]] && continue   # folded into d1c's row
    logs=("$k/$slug.run$run.logs"); [[ $test == d1 ]] && logs=("$k"/d1[abc].run$run.logs)
    turns=$(grep -rhoE 'turn tool surface resolved \{"model":"[^"]*"' "${logs[@]}" | sed 's/.*"model":"//;s/"//' | sort | uniq -c | awk '{printf "%s:%s ",$2,$1}')
    subs=$(grep -rh 'kind: subagent_started' "${logs[@]}" | wc -l | tr -d ' ')
    f3=""; [[ $test == d1 ]] && f3=$(grep -ciE 'Fase[[:space:]]*3' "$k/d1c.run$run.svar.txt")
    echo "$k|$test|$run|$turns|$subs|$f3"
  done
done >"$D/logs.tmp"
# Join with results and usage by suite file.
for f in d2 d3 d4 d1r1 d1r2 d1r3 d1r4 d1r5; do
  k=$(sed -n 's/^transcripts kept in .*\/\(nav-pilot-golden\.[^/]*\)$/\1/p' "$f.log")
  grep -v '^#' "$f-results.psv" | grep -v '^run|' | while IFS="|" read -r id run status _; do
    lr=$run; [[ $f == d1r* ]] && run=${f#d1r}
    cred=$(awk -F'|' -v r="$lr" '$2 == r && $1 !~ /^#/ { c[$6] += $13 / 1e9 } END { for (m in c) printf "%s:%.2f ", m, c[m] }' "$f-usage.psv")
    l=$(awk -F'|' -v k="$k" -v t="$id" -v r="$lr" '$1 == k && $2 == t && $3 == r { print $4 "|" $5 "|" $6 }' "$D/logs.tmp")
    echo "$id|$run|$status|$cred|$l"
  done
done >>"$OUT/raw.psv"
# Transcripts (not the debug logs: they carry the full request bodies).
mkdir -p "$OUT/transkripter"
for f in d2 d3 d4 d1r1 d1r2 d1r3 d1r4 d1r5; do
  k=$(sed -n 's/^transcripts kept in .*\/\(nav-pilot-golden\.[^/]*\)$/\1/p' "$f.log")
  for t in "$k"/d*.run*.txt; do
    case "$t" in *.svar.txt) continue ;; esac
    b=$(basename "$t"); [[ $f == d1r* ]] && b=${b/run1/run${f#d1r}}
    cp "$t" "$OUT/transkripter/$b"
  done
done
