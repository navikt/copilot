#!/usr/bin/env bash
#
# Tests for drift detection in lint-skills.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
LINT_SCRIPT="$SCRIPT_DIR/../scripts/lint-skills.sh"

pass=0
fail=0

assert_exit_code() {
  local expected=$1 actual=$2 name=$3
  if [[ "$actual" -eq "$expected" ]]; then
    pass=$(( pass + 1 ))
    printf '  ✅ %s\n' "$name"
  else
    fail=$(( fail + 1 ))
    printf '  ❌ %s (expected exit %d, got %d)\n' "$name" "$expected" "$actual"
  fi
}

assert_output_contains() {
  local pattern=$1 output=$2 name=$3
  if echo "$output" | grep -q "$pattern"; then
    pass=$(( pass + 1 ))
    printf '  ✅ %s\n' "$name"
  else
    fail=$(( fail + 1 ))
    printf '  ❌ %s (expected output to contain "%s")\n' "$name" "$pattern"
  fi
}

assert_output_not_contains() {
  local pattern=$1 output=$2 name=$3
  if ! echo "$output" | grep -q "$pattern"; then
    pass=$(( pass + 1 ))
    printf '  ✅ %s\n' "$name"
  else
    fail=$(( fail + 1 ))
    printf '  ❌ %s (expected output NOT to contain "%s")\n' "$name" "$pattern"
  fi
}

# Set up a temp repo structure so lint-skills.sh can find its root
setup_test_repo() {
  local tmpdir
  tmpdir=$(mktemp -d)
  git -C "$tmpdir" init --quiet
  mkdir -p "$tmpdir/.github/skills"
  mkdir -p "$tmpdir/scripts"
  cp "$LINT_SCRIPT" "$tmpdir/scripts/lint-skills.sh"
  echo "$tmpdir"
}

create_skill() {
  local repo=$1 name=$2 skill_content=$3 metadata=$4
  local dir="$repo/.github/skills/$name"
  mkdir -p "$dir"
  echo "$skill_content" > "$dir/SKILL.md"
  echo "$metadata" > "$dir/metadata.json"
}

echo "🧪 Drift detection tests"
echo ""

# ── Test 1: All in sync ───────────────────────────────────────────────
echo "Test 1: References in sync (metadata ↔ disk ↔ SKILL.md)"
repo=$(setup_test_repo)
create_skill "$repo" "test-skill" \
'---
name: test-skill
description: Test
---
# Test

See [queries](references/queries.md) for details.
' \
'{"description": "Test", "references": ["references/queries.md"]}'
mkdir -p "$repo/.github/skills/test-skill/references"
echo "# Queries" > "$repo/.github/skills/test-skill/references/queries.md"

exit_code=0
output=$(cd "$repo" && bash scripts/lint-skills.sh test-skill 2>&1) || exit_code=$?
assert_exit_code 0 "$exit_code" "exits 0 when in sync"
assert_output_contains "references in sync" "$output" "reports sync status"
rm -rf "$repo"

# ── Test 2: File on disk not in metadata ──────────────────────────────
echo ""
echo "Test 2: File on disk not in metadata (drift)"
repo=$(setup_test_repo)
create_skill "$repo" "drift-skill" \
'---
name: drift-skill
description: Test
---
# Test
' \
'{"description": "Test"}'
mkdir -p "$repo/.github/skills/drift-skill/references"
echo "# Orphan" > "$repo/.github/skills/drift-skill/references/orphan.md"

exit_code=0
output=$(cd "$repo" && bash scripts/lint-skills.sh drift-skill 2>&1) || exit_code=$?
assert_exit_code 1 "$exit_code" "exits 1 when file on disk not in metadata"
assert_output_contains "exists on disk but not in metadata" "$output" "reports disk-only file"
rm -rf "$repo"

# ── Test 3: Metadata lists file that doesn't exist ────────────────────
echo ""
echo "Test 3: Metadata references missing file"
repo=$(setup_test_repo)
create_skill "$repo" "missing-skill" \
'---
name: missing-skill
description: Test
---
# Test
' \
'{"description": "Test", "references": ["references/missing.md"]}'

exit_code=0
output=$(cd "$repo" && bash scripts/lint-skills.sh missing-skill 2>&1) || exit_code=$?
assert_exit_code 1 "$exit_code" "exits 1 when metadata file missing from disk"
assert_output_contains "file does not exist" "$output" "reports missing file"
rm -rf "$repo"

# ── Test 4: SKILL.md links to file not in metadata ────────────────────
echo ""
echo "Test 4: SKILL.md links to unreferenced file"
repo=$(setup_test_repo)
create_skill "$repo" "link-skill" \
'---
name: link-skill
description: Test
---
# Test

See [extra](references/extra.md) for more.
' \
'{"description": "Test"}'

exit_code=0
output=$(cd "$repo" && bash scripts/lint-skills.sh link-skill 2>&1) || exit_code=$?
# This is a warning, not an error, so exit 0
assert_exit_code 0 "$exit_code" "exits 0 (warning only for SKILL.md link not in metadata)"
assert_output_contains "not in metadata.json references" "$output" "warns about untracked link"
rm -rf "$repo"

# ── Test 5: Skill without references (no drift check needed) ──────────
echo ""
echo "Test 5: Skill without references (skip drift check)"
repo=$(setup_test_repo)
create_skill "$repo" "plain-skill" \
'---
name: plain-skill
description: Test
---
# Test

Just a plain skill with no references.
' \
'{"description": "Test"}'

exit_code=0
output=$(cd "$repo" && bash scripts/lint-skills.sh plain-skill 2>&1) || exit_code=$?
assert_exit_code 0 "$exit_code" "exits 0 for plain skill"
assert_output_not_contains "drift" "$output" "no drift messages for plain skill"
assert_output_not_contains "references" "$output" "no reference messages for plain skill"
rm -rf "$repo"

# ── Summary ───────────────────────────────────────────────────────────
echo ""
echo "─────────────────────────────────────────────"
echo "Passed: $pass  Failed: $fail"
if (( fail > 0 )); then
  exit 1
fi
