#!/usr/bin/env bats
#
# mcp-compliance-next-version.sh picks the release version from commit subjects.

SCRIPT="${BATS_TEST_DIRNAME}/mcp-compliance-next-version.sh"

setup() {
  cd "$BATS_TEST_TMPDIR"
  git init -q -b main r && cd r
  git config user.email t@example.com && git config user.name t && git config commit.gpgsign false && git config tag.gpgsign false
  mkdir -p cli/mcp-compliance other
}

commit() { echo "$RANDOM" >>"$1/f"; git add -A; git commit -q -m "$2"; }

@test "no tag gives v0.1.0" {
  commit cli/mcp-compliance "feat: start"
  run "$SCRIPT" HEAD
  [ "$output" = v0.1.0 ]
}

@test "fix bumps patch, other paths are ignored" {
  commit cli/mcp-compliance "feat: start"; git tag cli/mcp-compliance/v0.1.0
  commit other "feat!: elsewhere"
  commit cli/mcp-compliance "fix(mcp-compliance): x"
  run "$SCRIPT" HEAD
  [ "$output" = v0.1.1 ]
}

@test "feat bumps minor, breaking bumps minor below v1 and is refused from v1" {
  commit cli/mcp-compliance "feat: start"; git tag cli/mcp-compliance/v0.2.3
  commit cli/mcp-compliance "feat(mcp-compliance): y"
  run "$SCRIPT" HEAD
  [ "$output" = v0.3.0 ]
  git tag cli/mcp-compliance/v1.4.2
  commit cli/mcp-compliance "fix!: z"
  run "$SCRIPT" HEAD
  [ "$status" -ne 0 ]
  [[ "$output" == *"module path"* ]]
}

@test "nothing new fails" {
  commit cli/mcp-compliance "feat: start"; git tag cli/mcp-compliance/v0.1.0
  run "$SCRIPT" HEAD
  [ "$status" -ne 0 ]
}
