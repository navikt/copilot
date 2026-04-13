#!/bin/bash
set -euo pipefail

# E2E test for nav-pilot agent phase headers.
# Runs the Copilot CLI with --agent nav-pilot and checks that
# phase headers appear in the output.
#
# Usage:
#   ./scripts/test/test-agent-phases.sh              # structural tests only
#   ./scripts/test/test-agent-phases.sh --e2e         # include E2E tests
#   ./scripts/test/test-agent-phases.sh --e2e -v      # verbose E2E output
#
# Prerequisites:
#   - copilot CLI installed and authenticated
#   - Run from the navikt/copilot repo root

RUN_E2E=false
VERBOSE=false
for arg in "$@"; do
  case "$arg" in
    --e2e) RUN_E2E=true ;;
    -v|--verbose) VERBOSE=true ;;
  esac
done

PASS=0
FAIL=0
SKIP=0
RESULTS=()
OUTPUT_DIR=$(mktemp -d)
trap 'rm -rf "$OUTPUT_DIR"' EXIT

# ─── Helpers ─────────────────────────────────────────────────────────────────

log() { echo "  $*"; }
pass() { PASS=$((PASS + 1)); RESULTS+=("✅ $1"); log "✅ $1"; }
fail() { FAIL=$((FAIL + 1)); RESULTS+=("❌ $1: $2"); log "❌ $1: $2"; }
skip() { SKIP=$((SKIP + 1)); RESULTS+=("⏭ $1: $2"); log "⏭ $1: $2"; }

run_agent() {
  local name="$1"
  local agent="$2"
  local prompt="$3"
  local output_file="$OUTPUT_DIR/${name}.txt"

  log "Running: copilot --agent $agent -p '${prompt:0:60}..'" >&2

  local exit_code=0
  copilot \
    --agent "$agent" \
    -p "$prompt" \
    --allow-all \
    > "$output_file" 2>&1 || exit_code=$?

  local lines
  lines=$(wc -l < "$output_file" | tr -d ' ')
  log "  → exit=$exit_code, lines=$lines, saved to $output_file" >&2

  if [[ "$VERBOSE" == "true" ]]; then
    log "  --- First 80 lines ---" >&2
    head -80 "$output_file" >&2
    log "  --- End ---" >&2
  fi

  if [[ "$lines" -lt 2 ]]; then
    log "  ⚠ Output looks empty. Contents:" >&2
    cat "$output_file" >&2
  fi

  echo "$output_file"
}

check_file() {
  local test_name="$1"
  local output_file="$2"
  local pattern="$3"

  if [[ ! -f "$output_file" ]]; then
    fail "$test_name" "Output file not found"
    return
  fi

  if grep -qE "$pattern" "$output_file"; then
    pass "$test_name"
  else
    fail "$test_name" "Pattern not found: $pattern"
    if [[ "$VERBOSE" == "true" ]]; then
      log "  File: $output_file" >&2
    fi
  fi
}

# ─── Structural Tests (no CLI needed) ───────────────────────────────────────

echo ""
echo "═══ Agent File Structural Tests ═══"
echo ""

AGENT_FILE=".github/agents/nav-pilot.agent.md"

if [[ ! -f "$AGENT_FILE" ]]; then
  fail "Agent file exists" "$AGENT_FILE not found"
else
  pass "Agent file exists"

  # Check response_format tag
  if grep -q "<response_format>" "$AGENT_FILE"; then
    pass "Has <response_format> tag"
  else
    fail "Has <response_format> tag" "Missing <response_format> section"
  fi

  # Check all 4 phases defined
  for phase in "Fase 1: Intervju" "Fase 2: Plan" "Fase 3: Review" "Fase 4: Lever"; do
    if grep -q "$phase" "$AGENT_FILE"; then
      pass "Phase defined: $phase"
    else
      fail "Phase defined: $phase" "Not found in agent file"
    fi
  done

  # Check phase emojis
  for emoji in "🔍" "📐" "🔎" "🚀"; do
    if grep -q "$emoji" "$AGENT_FILE"; then
      pass "Phase emoji: $emoji"
    else
      fail "Phase emoji: $emoji" "Not found in agent file"
    fi
  done

  # Check phase stop separator
  if grep -q "─────" "$AGENT_FILE"; then
    pass "Has phase stop separator"
  else
    fail "Has phase stop separator" "Missing separator pattern"
  fi

  # Check mandatory language
  if grep -qi "MUST\|mandatory\|REGEL\|SKAL" "$AGENT_FILE"; then
    pass "Uses mandatory language"
  else
    fail "Uses mandatory language" "Phase instructions should be imperative"
  fi
fi

# Check specialist agents have progress indicators
for agent_file in .github/agents/{auth,security-champion,nais,observability,code-review}.agent.md; do
  name=$(basename "$agent_file" .agent.md)
  if [[ ! -f "$agent_file" ]]; then
    fail "Specialist agent: $name" "File not found"
    continue
  fi
  if grep -q "fremdrift\|progress\|Kartlegger\|Analyserer" "$agent_file"; then
    pass "Specialist progress: $name"
  else
    fail "Specialist progress: $name" "Missing progress indicators"
  fi
done

# ─── E2E Tests (requires copilot CLI + --e2e flag) ──────────────────────────

echo ""
echo "═══ E2E Agent Output Tests ═══"
echo ""

if [[ "$RUN_E2E" != "true" ]]; then
  log "Skipped — pass --e2e to run (takes ~2-5 min per test)"
  log "Example: ./scripts/test/test-agent-phases.sh --e2e -v"
elif ! command -v copilot &>/dev/null; then
  log "⚠ copilot CLI not found — skipping E2E tests"
else
  # Test 1: nav-pilot should show a phase header
  log ""
  log "Test 1: nav-pilot phase header on planning prompt"
  FILE=$(run_agent "phase-header" "nav-pilot" \
    "Jeg trenger en ny Kotlin/Ktor backend-tjeneste som mottar dagpengesøknader via Kafka. Ikke generer kode, bare kartlegg og planlegg.")
  check_file "nav-pilot shows phase header" "$FILE" \
    "(Fase [1-4]:|🔍 Fase|📐 Fase|🔎 Fase|🚀 Fase)"

  # Test 2: planning prompt should show early phases (1 or 2)
  log ""
  log "Test 2: nav-pilot shows planning phase (Fase 1 or 2)"
  check_file "nav-pilot shows planning phase" "$FILE" \
    "(Fase [12]|Intervju|Plan|kartlegg|planlegg)"

  # Test 3: auth-agent should produce auth-related output
  log ""
  log "Test 3: auth-agent produces auth content"
  FILE=$(run_agent "auth-check" "auth-agent" \
    "Gjør en rask sjekk av auth-oppsettet i dette repoet. Bare oppsummer, ikke endre noe.")
  check_file "auth-agent shows auth content" "$FILE" \
    "(auth|Auth|token|Token|Azure|azureAd|OAuth|JWT|🔐)"

  log ""
  log "Output files preserved in: $OUTPUT_DIR"
  # Don't clean up if E2E ran — user might want to inspect
  trap - EXIT
  log "  Inspect: ls $OUTPUT_DIR/"
fi

# ─── Summary ────────────────────────────────────────────────────────────────

echo ""
echo "═══ Results ═══"
echo ""
for r in "${RESULTS[@]}"; do
  echo "  $r"
done
echo ""
echo "  Total: $((PASS + FAIL + SKIP)) | ✅ $PASS passed | ❌ $FAIL failed | ⏭ $SKIP skipped"
echo ""

if [[ $FAIL -gt 0 ]]; then
  exit 1
fi
