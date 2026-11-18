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
# A dev-only survey (environments without prod-gcp on the base, see
# forCluster) is a test fixture and is not frozen. environments is frozen
# for the rest, so a pull request cannot make a prod survey dev-only first.
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
  new=$(git show "HEAD:$f" 2>/dev/null || echo '{}')
  # jq keeps the last of two equal keys and Go merges them (decodeStrict),
  # so the check below would compare something other than what is served.
  # Equal keys show as a stream that differs from the parsed value's; keys
  # equal but for case (Go folds them) as a clash after ascii_downcase.
  if [[ "$(jq -c --stream . <<<"$new")" != "$(jq -c . <<<"$new" | jq -c --stream .)" ]] ||
    ! jq -e '[.. | objects | keys | map(ascii_downcase) | length == (unique | length)] | all' <<<"$new" >/dev/null ||
    ! jq -e '[.. | objects | keys[] | test("^[ -~]*$")] | all' <<<"$new" >/dev/null; then
    echo "::error file=$f::$f has a duplicate key (also one that differs only in case) or a non-ASCII key; definitions may have neither."
    fail=1
    continue
  fi
  old=$(git show "$base:$f" 2>/dev/null) || continue # new file
  jq -e --arg today "$today" '.starts <= $today and ((.environments // ["prod-gcp"]) | index("prod-gcp"))' <<<"$old" >/dev/null || continue
  if [[ "$(jq -cS '[.id, .questions, .starts, .environments]' <<<"$old")" != "$(jq -cS '[.id, .questions, .starts, .environments]' <<<"$new")" ]]; then
    echo "::error file=$f::$f has started: its id, questions, starts and environments are frozen. Change them in a new wave (new id), see surveys/README.md."
    fail=1
  fi
done
exit $fail
