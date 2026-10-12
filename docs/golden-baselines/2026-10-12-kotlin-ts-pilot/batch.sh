#!/bin/bash
# Run attempts "task model n" from args (triples) with a hard 250-credit total cap.
S=$SCRATCH
while [ $# -ge 3 ]; do
  spent=$(awk -F'|' '{s+=$5} END {printf "%.0f", s+0.999}' "$S/attempts.psv" 2>/dev/null || echo 0)
  left=$((250 - spent))
  cap=30; [ "$left" -lt 40 ] && cap=$((left - 10))   # soft per-session limit can overshoot ~10
  if [ "$cap" -lt 3 ]; then echo "STOP: budget reached ($spent spent)"; exit 0; fi
  bash "$S/run-attempt.sh" "$1" "$2" "$3" "$cap" || { echo "STOP: usage missing for $1 $2 $3"; exit 1; }
  shift 3
done
