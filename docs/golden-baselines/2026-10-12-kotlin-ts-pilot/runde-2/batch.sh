#!/bin/bash
# Run attempts "task model n" (triples) under a 250-credit budget. Not a hard cap:
# --max-ai-credits is soft, so an attempt can overshoot its reservation
# (40 for Sol, 5 for Luna). The batch stops before starting one that would not fit.
S=$SCRATCH
while [ $# -ge 3 ]; do
  spent=$(awk -F'|' '{s+=$5} END {printf "%.0f", s+0.999}' "$S/attempts.psv" 2>/dev/null || echo 0)
  left=$((250 - spent))
  need=40; [[ "$2" == *luna* ]] && need=5
  if [ "$left" -lt "$need" ]; then echo "STOP: budget reached ($spent spent, $left left)"; exit 0; fi
  cap=30; [ "$left" -lt 40 ] && cap=$((left - 10))
  if [ "$cap" -lt 3 ]; then echo "STOP: budget reached ($spent spent, $left left)"; exit 0; fi
  bash "$S/run-attempt.sh" "$1" "$2" "$3" "$cap" || { echo "STOP: usage missing for $1 $2 $3"; exit 1; }
  shift 3
done
