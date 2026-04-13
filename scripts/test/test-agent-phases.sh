#!/bin/bash
set -euo pipefail

# E2E test for nav-pilot agent phase headers.
# Runs the Copilot CLI with --agent nav-pilot and checks that
# phase headers appear in the output.
#
# Usage:
#   ./scripts/test/test-agent-phases.sh
#   ./scripts/test/test-agent-phases.sh --verbose
#
# Prerequisites:
#   - copilot CLI installed and authenticated
#   - Run from the navikt/copilot repo root

VERBOSE="${1:-}"
PASS=0
FAIL=0
RESULTS=()

# ─── Helpers ─────────────────────────────────────────────────────────────────

log() { echo "  $*"; }
pass() { PASS=$((PASS + 1)); RESULTS+=("✅ $1"); log "✅ $1"; }
fail() { FAIL=$((FAIL + 1)); RESULTS+=("❌ $1: $2"); log "❌ $1: $2"; }

run_agent() {
  local agent="$1"
  local prompt="$2"
  local output_file
  output_file=$(mktemp)

  log "Running: copilot --agent $agent -p \"$prompt\" ..."

  # Run with --quiet to minimize noise, --allow-all for non-interactive
  if ! copilot \
    --agent "$agent" \
    -p "$prompt" \
    --quiet \
    --allow-all \
    --stream off \
    > "$output_file" 2>&1; then
    log "⚠ copilot exited with non-zero status"
  fi

  if [[ "$VERBOSE" == "--verbose" ]]; then
    log "--- Output ---"
    head -50 "$output_file"
    log "--- End ---"
  fi

  cat "$output_file"
  rm -f "$output_file"
}

check_output() {
  local test_name="$1"
  local output="$2"
  local pattern="$3"

  if echo "$output" | grep -qE "$pattern"; then
    pass "$test_name"
  else
    fail "$test_name" "Pattern not found: $pattern"
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

# ─── E2E Tests (requires copilot CLI) ───────────────────────────────────────

echo ""
echo "═══ E2E Agent Output Tests ═══"
echo ""

if ! command -v copilot &>/dev/null; then
  log "⚠ copilot CLI not found — skipping E2E tests"
  log "  Install: https://docs.github.com/en/copilot/github-copilot-in-the-cli"
else
  # Test 1: nav-pilot should show a phase header
  log ""
  log "Test: nav-pilot phase header on planning prompt"
  OUTPUT=$(run_agent "nav-pilot" "Jeg trenger en ny Kotlin/Ktor backend-tjeneste som mottar dagpengesøknader via Kafka. Ikke generer kode, bare kartlegg.")
  PHASE_PATTERN="(Fase [1-4]:|🔍|📐|🔎|🚀)"
  check_output "nav-pilot shows phase header" "$OUTPUT" "$PHASE_PATTERN"

  # Test 2: nav-pilot should show phase stop
  log ""
  log "Test: nav-pilot phase stop separator"
  OUTPUT=$(run_agent "nav-pilot" "Planlegg en ny tjeneste for å behandle vedtak. Kun fase 1, stopp etter intervju.")
  STOP_PATTERN="(Fase .* ferdig|⏸|Bekreft|fortsette|────)"
  check_output "nav-pilot shows phase stop" "$OUTPUT" "$STOP_PATTERN"

  # Test 3: auth-agent should show progress indicator
  log ""
  log "Test: auth-agent progress indicator"
  OUTPUT=$(run_agent "auth-agent" "Gjør en rask sjekk av auth-oppsettet i dette repoet. Bare oppsummer, ikke endre noe.")
  AUTH_PATTERN="(🔐|Kartlegger|Analyserer|Funn|Auth)"
  check_output "auth-agent shows progress" "$OUTPUT" "$AUTH_PATTERN"
fi

# ─── Summary ────────────────────────────────────────────────────────────────

echo ""
echo "═══ Results ═══"
echo ""
for r in "${RESULTS[@]}"; do
  echo "  $r"
done
echo ""
echo "  Total: $((PASS + FAIL)) | ✅ $PASS passed | ❌ $FAIL failed"
echo ""

if [[ $FAIL -gt 0 ]]; then
  exit 1
fi
