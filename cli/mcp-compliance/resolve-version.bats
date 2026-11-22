#!/usr/bin/env bats

r() { "$BATS_TEST_DIRNAME/resolve-version.sh" "$@" 2>/dev/null; }

@test "empty input uses the pinned release tag" { [ "$(r "" cli/mcp-compliance/v1.2.3)" = v1.2.3 ]; }
@test "branch ref falls back to latest" { [ "$(r "" main)" = latest ]; }
@test "empty ref (local path) falls back to latest" { [ "$(r "" "")" = latest ]; }
@test "SHA ref falls back to latest" { [ "$(r "" 0123456789abcdef0123456789abcdef01234567)" = latest ]; }
@test "pre-release or foreign tag falls back to latest" {
  [ "$(r "" cli/mcp-compliance/v1.2.3-rc1)" = latest ]
  [ "$(r "" cli/other/v1.2.3)" = latest ]
}
@test "fallback logs a notice" { run "$BATS_TEST_DIRNAME/resolve-version.sh" "" main; [[ "$output" == *"::notice::"* ]]; }
@test "explicit input wins" {
  [ "$(r latest cli/mcp-compliance/v1.2.3)" = latest ]
  [ "$(r v0.2.0 cli/mcp-compliance/v1.2.3)" = v0.2.0 ]
}
