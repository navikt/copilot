#!/usr/bin/env bash
# Refuses a change to the id, questions or starts of a survey that has
# started (starts on or before today, UTC, on the base commit), active or not.
# Answers store the server's question versions, not what the respondent saw,
# and nav-pilot caches definitions for up to a day, so a question cannot
# change mid-wave, bumped version or not. Change it in a new wave (a new id).
# active and starts are not a way out: a pull request that only sets
# active: false, or moves starts later, would let the next one change the
# questions. Every other field (title, intro, nudge, series, min_cli_version,
# active, ends) may still change.
#
# Usage: check-frozen-surveys.sh <base commit>
set -euo pipefail
base=${1:?usage: $0 <base commit>}
if ! git rev-parse --quiet --verify "$base^{commit}" >/dev/null; then
  echo "::error::base commit $base cannot be resolved; fetch it (fetch-depth: 0) before the check."
  exit 1
fi
today=$(date -u +%F)
fail=0
for f in $(git diff --no-renames --name-only "$base" HEAD -- ':(top)apps/copilot-survey/surveys/*.json'); do
  old=$(git show "$base:$f" 2>/dev/null) || continue # new file
  jq -e --arg today "$today" '.starts <= $today' <<<"$old" >/dev/null || continue
  new=$(git show "HEAD:$f" 2>/dev/null || echo '{}')
  if [[ "$(jq -cS '[.id, .questions, .starts]' <<<"$old")" != "$(jq -cS '[.id, .questions, .starts]' <<<"$new")" ]]; then
    echo "::error file=$f::$f has started: its id, questions and starts are frozen. Change them in a new wave (new id), see surveys/README.md."
    fail=1
  fi
done
exit $fail
