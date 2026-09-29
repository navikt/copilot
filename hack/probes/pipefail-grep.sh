#!/usr/bin/env bash
# pipefail-grep counts how often `echo "$body" | grep -q X` reports no match
# under `set -o pipefail` when X is there, against `grep -q X <<< "$body"`.
# The body is skills/security-review/SKILL.md, the check the one in
# scripts/lint-skills.sh. It keeps every CPU busy with `yes` while it runs,
# since the early-exit race shows up when the writer gets descheduled.
#
# REPEAT=n uses the body n times over, to see the race with a body larger
# than the pipe buffer.
#
#   [REPEAT=n] bash hack/probes/pipefail-grep.sh [runs-per-worker] [workers]
set -uo pipefail
runs=${1:-2000}
workers=${2:-6}
one=$(awk 'BEGIN{fm=0} /^---$/{fm++; next} fm>=2{print}' skills/security-review/SKILL.md)
body=$one
for (( i = 1; i < ${REPEAT:-1}; i++ )); do body+=$'\n'$one; done
echo "body: ${#body} bytes"
export body runs

hogs=()
for _ in $(seq 1 $(( $(getconf _NPROCESSORS_ONLN) + 6 ))); do
  yes >/dev/null &
  hogs+=($!)
done
trap 'kill "${hogs[@]}" 2>/dev/null' EXIT

for form in pipe herestring; do
  total=0
  for _ in $(seq 1 "$workers"); do
    bash -c '
      set -uo pipefail
      f=0
      for _ in $(seq 1 "$runs"); do
        if [[ $0 == pipe ]]; then
          echo "$body" | grep -q "^## Review contract$" || f=$((f+1))
        else
          grep -q "^## Review contract$" <<< "$body" || f=$((f+1))
        fi
      done
      echo "$f"' "$form" > "${TMPDIR:-/tmp}/pipefail-grep.$form.$_" &
  done
  # shellcheck disable=SC2046 # one pid per word
  wait $(jobs -p | grep -vxF "$(printf '%s\n' "${hogs[@]}")")
  for f in "${TMPDIR:-/tmp}"/pipefail-grep."$form".*; do
    total=$(( total + $(cat "$f") )); rm -f "$f"
  done
  echo "$form: $total false negatives in $(( runs * workers )) runs"
done
