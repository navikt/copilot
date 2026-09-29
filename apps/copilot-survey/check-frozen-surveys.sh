#!/usr/bin/env bash
# Refuses a change to the id or questions of a survey that has opened
# (active on the base commit and starts on or before today, UTC). Answers
# store the server's question versions, not what the respondent saw, and
# nav-pilot caches definitions for up to a day, so a question cannot change
# mid-wave, bumped version or not. Change it in a new wave (a new id).
# The title, intro, nudge, active and ends may still change.
#
# Usage: check-frozen-surveys.sh <base commit>
set -euo pipefail
base=${1:?usage: $0 <base commit>}
today=$(date -u +%F)
fail=0
for f in $(git diff --name-only "$base" HEAD -- ':(top)apps/copilot-survey/surveys/*.json'); do
  old=$(git show "$base:$f" 2>/dev/null) || continue # new file
  jq -e --arg today "$today" '.active == true and .starts <= $today' <<<"$old" >/dev/null || continue
  new=$(git show "HEAD:$f" 2>/dev/null || echo '{}')
  if [[ "$(jq -cS '[.id, .questions]' <<<"$old")" != "$(jq -cS '[.id, .questions]' <<<"$new")" ]]; then
    echo "::error file=$f::$f has opened: its id and questions are frozen. Change them in a new wave (new id), see surveys/README.md."
    fail=1
  fi
done
exit $fail
