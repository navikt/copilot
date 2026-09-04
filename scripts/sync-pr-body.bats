#!/usr/bin/env bats
#
# The sync workflow opens pull requests in other teams' repositories, and the
# only thing that has ever described such a PR is the jq filter in its check
# step. That filter cannot be exercised by opening real PRs, so it is lifted
# out of the workflow verbatim here — one source of truth, no copy to drift —
# and run against the JSON `nav-pilot sync --json` actually emits.

WORKFLOW="${BATS_TEST_DIRNAME}/../.github/workflows/copilot-customization-sync.yml"

setup() {
  # Lift CHANGE_FILTER out of the workflow between its marker comments.
  eval "$(sed -n '/sync-pr-body-filter-start/,/sync-pr-body-filter-end/p' "$WORKFLOW" | sed '1d;$d')"
  [ -n "$CHANGE_FILTER" ]
}

changes() { # $1 = sync JSON
  jq -r "$CHANGE_FILTER" <<<"$1"
}

@test "lists updated files" {
  run changes '{"up_to_date":false,"updates":[{"path":".github/agents/a.agent.md"},{"path":".github/skills/s/SKILL.md"}]}'
  [ "$status" -eq 0 ]
  [ "${lines[0]}" = ".github/agents/a.agent.md" ]
  [ "${lines[1]}" = ".github/skills/s/SKILL.md" ]
  [ "${#lines[@]}" -eq 2 ]
}

@test "names the lock file when only the pinned revision moved" {
  run changes '{"up_to_date":false,"pin_bump":{"path":".nav-pilot/agentpakke.lock.json","from":"aaaaaaabbbbbbbccccccc0000000000000000000","to":"ddddddd1111111222222233333330000000000f0"}}'
  [ "$status" -eq 0 ]
  [ "${#lines[@]}" -eq 1 ]
  [[ "${lines[0]}" == ".nav-pilot/agentpakke.lock.json (pinned revision aaaaaaa → ddddddd)" ]]
}

@test "lists files, deletions and the pin together" {
  run changes '{"up_to_date":false,"updates":[{"path":"a.md"}],"deletions":["b.md"],"pin_bump":{"path":".nav-pilot/agentpakke.lock.json","from":"1111111000000000000000000000000000000000","to":"2222222000000000000000000000000000000000"}}'
  [ "$status" -eq 0 ]
  [ "${lines[0]}" = "a.md" ]
  [ "${lines[1]}" = "b.md (deleted upstream)" ]
  [[ "${lines[2]}" == ".nav-pilot/agentpakke.lock.json"* ]]
  [ "${#lines[@]}" -eq 3 ]
}

@test "an up-to-date sync produces no lines" {
  run changes '{"up_to_date":true,"source":"1111111000000000000000000000000000000000"}'
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}
