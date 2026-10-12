#!/bin/bash
# Run attempts "task model n" (triples) under a hard 250-credit total cap.
# Each Sol attempt is budgeted at 40 credits; stop before one could cross the cap.
S=$SCRATCH
while [ $# -ge 3 ]; do
  spent=$(awk -F'|' '{s+=$5} END {printf "%.0f", s+0.999}' "$S/attempts.psv" 2>/dev/null || echo 0)
  left=$((250 - spent))
  need=40; [[ "$2" == *luna* ]] && need=5
  if [ "$left" -lt "$need" ]; then echo "STOP: budget reached ($spent spent, $left left)"; exit 0; fi
  cap=30; [ "$left" -lt 40 ] && cap=$((left - 10))
  bash "$S/run-attempt.sh" "$1" "$2" "$3" "$cap" || { echo "STOP: usage missing for $1 $2 $3"; exit 1; }
  shift 3
done
