#!/usr/bin/env bash
#
# nav-pilot golden-prompt harness — behavioural regression test for the persona.
#
# WHY THIS EXISTS
#   `mise run nav-pilot:check` tests the nav-pilot *binary* (build, vet, tests).
#   Nothing tests the *persona* — agents/nav-pilot.agent.md. Trimming that file
#   for token cost is exactly the kind of change that silently breaks behaviour:
#   the CLI still builds, the agent still answers, but it stops emitting phase
#   checkpoints or starts recommending the wrong auth mechanism.
#
#   This harness runs a small set of prompts through the persona and asserts
#   *behavioural invariants* with regexes — never text equality. Each assertion
#   maps to a rule under `## Boundaries → ✅ Always` in the persona, which is the
#   closest thing the persona has to a spec.
#
#   ⚠️  Test 5 (TokenX vs Azure client_credentials) is the assertion most likely
#   to catch an over-aggressive cut to the authentication decision tree in
#   `### Fase 2: Plan`. If that table is trimmed or reworded, test 5 fails first.
#   Treat a test 5 failure as "the auth knowledge was load-bearing", not as a
#   flaky assertion to relax.
#
# NOT PART OF CI
#   This makes live model calls. It costs money, it is non-deterministic, and it
#   needs an authenticated Copilot CLI. It is deliberately NOT wired into
#   `mise run nav-pilot:check` and must not be. Run it by hand when you change
#   agents/nav-pilot.agent.md, and paste the result into the PR.
#
# SAFETY
#   Every prompt runs inside a throwaway workspace (mktemp -d) seeded with a
#   minimal fake Nav repo — never against this checkout. The Copilot CLI needs
#   --allow-all-tools for non-interactive mode, so the agent can read/write/run
#   inside that scratch directory. It is removed on exit unless you pass --keep.
#
# USAGE
#   ./scripts/nav-pilot-golden.sh                 # run all tests
#   ./scripts/nav-pilot-golden.sh --only 2,5      # run selected tests
#   ./scripts/nav-pilot-golden.sh --keep          # keep transcripts for inspection
#   ./scripts/nav-pilot-golden.sh --model <model> # pin a model (default: CLI default)
#   ./scripts/nav-pilot-golden.sh --json          # machine-readable summary (needs jq)
#   ./scripts/nav-pilot-golden.sh -v              # echo each transcript as it lands
#
# EXIT CODES
#   0  all selected assertions passed
#   1  at least one assertion failed
#   2  preflight failed (no client, not authenticated, persona missing)
#   3  no assertion failed, but at least one test could not be evaluated
#      (empty transcript, or the response never reached the phase under test).
#      This is deliberately NOT 0: a test that never ran has proven nothing.
#
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PERSONA="$REPO_ROOT/agents/nav-pilot.agent.md"
AGENT_NAME="nav-pilot"
TIMEOUT_SECS="${NAV_PILOT_GOLDEN_TIMEOUT:-300}"

# Per-prompt wall-clock guard, so one hung call cannot stall the suite. macOS
# has no coreutils `timeout` by default; fall back to running unguarded.
TIMEOUT_BIN="$(command -v timeout || command -v gtimeout || true)"

ONLY=""
KEEP=false
VERBOSE=false
JSON=false
MODEL=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --only)    ONLY="${2:-}"; shift 2 ;;
    --keep)    KEEP=true; shift ;;
    --model)   MODEL="${2:-}"; shift 2 ;;
    --json)    JSON=true; shift ;;
    -v|--verbose) VERBOSE=true; shift ;;
    -h|--help) sed -n '2,/^set -uo/p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//;$d'; exit 0 ;;
    *) echo "unknown flag: $1 (try --help)" >&2; exit 2 ;;
  esac
done

if [[ -t 1 ]]; then
  BOLD=$'\033[1m'; DIM=$'\033[2m'; RED=$'\033[31m'; GREEN=$'\033[32m'
  YELLOW=$'\033[33m'; RESET=$'\033[0m'
else
  BOLD=""; DIM=""; RED=""; GREEN=""; YELLOW=""; RESET=""
fi

fail_preflight() {
  echo "${RED}✗ preflight:${RESET} $1" >&2
  [[ -n "${2:-}" ]] && echo "  ${DIM}→ $2${RESET}" >&2
  exit 2
}

# ─── Preflight ───────────────────────────────────────────────────────────────

[[ -f "$PERSONA" ]] || fail_preflight \
  "persona not found at $PERSONA" \
  "Run this script from a copilot checkout; it tests the working-tree persona."

CLI_PATH="$(command -v copilot || true)"
CLI_NAME="copilot"
if [[ -z "$CLI_PATH" ]]; then
  CLI_PATH="$(command -v cplt || true)"
  CLI_NAME="cplt"
fi
if [[ -z "$CLI_PATH" ]]; then
  if command -v opencode >/dev/null 2>&1; then
    fail_preflight \
      "only 'opencode' was found on PATH, and this harness does not support it" \
      "opencode reads its persona from the *user* config dir, so a hermetic run is not possible. Install the Copilot CLI: https://github.com/github/copilot-cli"
  fi
  fail_preflight \
    "neither 'copilot' nor 'cplt' found on PATH" \
    "Install the Copilot CLI (https://github.com/github/copilot-cli), or 'brew install navikt/tap/cplt' for the sandboxed wrapper."
fi

if ! "$CLI_PATH" --version >/dev/null 2>&1; then
  fail_preflight \
    "'$CLI_NAME --version' failed — the CLI is on PATH but not runnable" \
    "Try running '$CLI_NAME' once interactively to complete setup."
fi

# Credential check: a trivial non-interactive prompt. An unauthenticated CLI
# fails here with an auth error rather than mid-suite with a confusing timeout.
probe_out="$("$CLI_PATH" -p "svar kun med ordet OK" --no-color --log-level none 2>&1)"
probe_rc=$?
if [[ $probe_rc -ne 0 ]] || grep -qiE "not (logged in|authenticated)|unauthorized|401|please (log ?in|sign in)|GITHUB_TOKEN" <<<"$probe_out"; then
  fail_preflight \
    "$CLI_NAME is not authenticated (probe prompt failed)" \
    "Run '$CLI_NAME' interactively and complete login, or export a valid GITHUB_TOKEN. Probe said: $(head -c 200 <<<"$probe_out")"
fi

if $JSON && ! command -v jq >/dev/null 2>&1; then
  fail_preflight "--json needs jq" "brew install jq"
fi

# ─── Throwaway workspace ─────────────────────────────────────────────────────
# Seeded with just enough of a Nav repo that the prompts are grounded: the
# persona infers archetype from nais.yaml / build.gradle.kts in Fase 1.

WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/nav-pilot-golden.XXXXXX")"
# shellcheck disable=SC2329  # invoked via trap
cleanup() {
  if $KEEP; then
    echo "${DIM}transcripts kept in $WORKDIR${RESET}"
  else
    rm -rf "$WORKDIR"
  fi
}
trap cleanup EXIT

mkdir -p "$WORKDIR/.github/agents" "$WORKDIR/src/main/kotlin/no/nav/demo"
cp "$PERSONA" "$WORKDIR/.github/agents/$AGENT_NAME.agent.md"

cat >"$WORKDIR/README.md" <<'EOF'
# demo-tjeneste

En liten Ktor-tjeneste for demonstrasjon. Tjenesten eksponerer et REST-API
og kjører på Nais. Dokumentasjonen er dessverre ikke helt komplet enda.
EOF

cat >"$WORKDIR/nais.yaml" <<'EOF'
apiVersion: nais.io/v1alpha1
kind: Application
metadata:
  name: demo-tjeneste
  namespace: demo
spec:
  image: {{image}}
  port: 8080
  liveness:
    path: /internal/isalive
  readiness:
    path: /internal/isready
EOF

cat >"$WORKDIR/build.gradle.kts" <<'EOF'
plugins { kotlin("jvm") version "2.1.0" }
dependencies { implementation("io.ktor:ktor-server-netty:3.0.0") }
EOF

for f in App Routes Config; do
  cat >"$WORKDIR/src/main/kotlin/no/nav/demo/$f.kt" <<EOF
package no.nav.demo

// bruker maksAntall flere steder
val maksAntall = 100
EOF
done

# ─── Test runner ─────────────────────────────────────────────────────────────

declare -a RESULTS=()
pass_count=0
fail_count=0
error_count=0

# A transcript shorter than this is treated as "the call did not happen", not as
# a response. Every assertion below is either an absent() — which succeeds
# trivially on an empty file — or a present() on a long structured block, so
# without this floor a crashed or unauthenticated CLI reports green.
MIN_TRANSCRIPT_BYTES=200
LAST_PROMPT_DETAIL=""

run_prompt() {
  # run_prompt <slug> <prompt> → writes transcript to $WORKDIR/<slug>.txt
  # Returns 0 if the transcript is usable, 1 if it is missing/too short to
  # assert against. Callers MUST branch on this — see record_error.
  local slug="$1" prompt="$2" out="$WORKDIR/$1.txt"
  local -a args=(-p "$prompt" --agent "$AGENT_NAME" --allow-all-tools --no-color --log-level none)
  [[ -n "$MODEL" ]] && args+=(--model "$MODEL")

  echo "${DIM}  → prompting ($slug)…${RESET}" >&2
  local -a runner=()
  [[ -n "$TIMEOUT_BIN" ]] && runner=("$TIMEOUT_BIN" "$TIMEOUT_SECS")
  # ${arr[@]+"${arr[@]}"} — bash 3.2 (stock macOS) treats an empty array as an
  # unbound variable under `set -u`, and the no-coreutils fallback above leaves
  # `runner` empty on exactly that platform.
  ( cd "$WORKDIR" && ${runner[@]+"${runner[@]}"} "$CLI_PATH" "${args[@]}" ) >"$out" 2>"$WORKDIR/$slug.err"
  local rc=$?
  if [[ $rc -eq 124 ]]; then
    echo "${YELLOW}⚠ $slug: timed out after ${TIMEOUT_SECS}s (raise NAV_PILOT_GOLDEN_TIMEOUT)${RESET}" >&2
  fi
  $VERBOSE && { echo "${DIM}--- $slug ---${RESET}"; cat "$out"; echo "${DIM}--- end ---${RESET}"; }

  local size=0
  [[ -f "$out" ]] && size="$(wc -c <"$out" | tr -d ' ')"
  if [[ "$size" -lt "$MIN_TRANSCRIPT_BYTES" ]]; then
    echo "${YELLOW}⚠ $slug: CLI exited $rc with ${size}B of output (need ≥${MIN_TRANSCRIPT_BYTES}B)${RESET}" >&2
    head -c 300 "$WORKDIR/$slug.err" >&2; echo >&2
    LAST_PROMPT_DETAIL="CLI exited $rc, transcript was ${size}B (<${MIN_TRANSCRIPT_BYTES}B) — nothing to assert against"
    return 1
  fi
  return 0
}

record() {
  # record <id> <description> <ok:0|1> [detail]
  local id="$1" desc="$2" ok="$3" detail="${4:-}"
  if [[ "$ok" == "0" ]]; then
    echo "  ${GREEN}✓${RESET} ${BOLD}$id${RESET} $desc"
    RESULTS+=("$id|pass|$desc|")
    pass_count=$((pass_count + 1))
  else
    echo "  ${RED}✗${RESET} ${BOLD}$id${RESET} $desc"
    [[ -n "$detail" ]] && echo "      ${DIM}$detail${RESET}"
    RESULTS+=("$id|fail|$desc|$detail")
    fail_count=$((fail_count + 1))
  fi
}

record_error() {
  # record_error <id> <description> <detail>
  # For tests that could not be evaluated at all — a dead transcript, or a
  # response that never reached the phase the assertion is about. Distinct from
  # both pass and fail on purpose: reporting green here is the failure mode that
  # gets the whole harness ignored.
  local id="$1" desc="$2" detail="${3:-}"
  echo "  ${YELLOW}⚠${RESET} ${BOLD}$id${RESET} $desc ${YELLOW}(not evaluated)${RESET}"
  [[ -n "$detail" ]] && echo "      ${DIM}$detail${RESET}"
  RESULTS+=("$id|error|$desc|$detail")
  error_count=$((error_count + 1))
}

# Assertion primitives. All operate on a transcript file with extended regexes;
# never on exact strings — the persona's wording is allowed to drift, its
# behaviour is not.
present() { grep -qiE -- "$2" "$1"; }
absent()  { ! grep -qiE -- "$2" "$1"; }
count_of() { grep -oiE -- "$2" "$1" 2>/dev/null | wc -l | tr -d ' '; }

# Recommendation verbs, word-bounded on both sides. The boundaries matter:
# an unanchored /bruk/ also matches "brukerkontekst", which is exactly the word
# a *correct* answer uses, so it would fail every passing run.
#
# The stem must carry its inflections: `bruk` alone is the *imperative*, and the
# most idiomatic wrong answers are the infinitive and present ("kan du bruke
# client_credentials", "client_credentials brukes her"), which a bare stem with
# a right boundary silently misses. The right boundary still rejects
# "brukerkontekst" and "brukstilfelle" — a letter follows the stem there.
_BL='(^|[^a-zæøåA-ZÆØÅ])'
_BR='([^a-zæøåA-ZÆØÅ]|$)'
RE_RECOMMEND_VERB="${_BL}(bruk(e|er|es|t|te)?|bør|velg|anbefal(er|es|t)?|benytt(er)?|sett(e|er)?[[:space:]]+opp|konfigurer(e|er)?|gå[[:space:]]+for|riktig[[:space:]]+valg)${_BR}"

# recommends_client_credentials <transcript> → 0 if the response appears to
# *recommend* Azure client_credentials for a user-context call.
#
# A correct answer is expected to mention client_credentials in order to reject
# it ("ikke bruk client_credentials") or to scope it to batch, so a bare
# substring match would false-positive. Instead: isolate the clauses that
# mention it, drop the ones carrying a negation / contrast / batch marker, and
# fail only if a recommending verb survives in a remaining clause.
recommends_client_credentials() {
  # Hard fail: explicitly preferring client_credentials over TokenX. Checked
  # first because the phrasing ("i stedet for TokenX") carries a contrast
  # marker that the clause filter below would otherwise treat as a rejection.
  if grep -qiE 'client_credentials[^.!?]{0,80}(i[[:space:]]+stedet[[:space:]]+for|istedenfor|fremfor|framfor)[[:space:]]+TokenX' "$1"; then
    return 0
  fi
  grep -oiE '[^.!?]*client_credentials[^.!?]*' "$1" 2>/dev/null \
    | grep -viE 'ikke|aldri|unngå|unngaa|i[[:space:]]+stedet|istedenfor|fremfor|framfor|feil|ugyldig|mister|taper|sporbarhet|batch|maskin-til-maskin|🚫|❌|uten[[:space:]]+brukerkontekst' \
    | grep -qiE "$RE_RECOMMEND_VERB"
}

selected() {
  [[ -z "$ONLY" ]] && return 0
  [[ ",$ONLY," == *",$1,"* ]]
}

echo "${BOLD}nav-pilot golden-prompt harness${RESET}"
echo "${DIM}client: $CLI_NAME ($CLI_PATH)${RESET}"
echo "${DIM}persona: $PERSONA${RESET}"
echo "${DIM}workspace: $WORKDIR${RESET}"
echo

# Regex vocabulary shared by several assertions.
RE_CHECKPOINT='Fase[[:space:]]+[0-9]+[[:space:]]+ferdig'
RE_CONFIRM='Bekreft for å fortsette'
# Fase 2 artifacts that do NOT appear in the Fase 1 checkpoint template.
# (Note: "🔴 Rød sone" alone is a poor discriminator — the Fase 1 checkpoint
# block lists it as a summary line — so we key on the green-zone block and on
# accessPolicy, which the persona mandates in the Fase 2 Nais manifest.)
#
# Deliberately NOT "apiVersion: nais": the seeded workspace ships a nais.yaml
# whose first line is `apiVersion: nais.io/v1alpha1`, and Fase 1 reads that file
# to infer the archetype. Echoing it back is correct behaviour, so keying on it
# would false-fail test 2 on its own fixture.
#
# Used in both directions: as a *leak* detector in test 2 (must be absent after
# Fase 1) and as a *presence* gate in test 4 (must be present before the
# red-zone assertion means anything).
RE_PHASE2_ARTIFACT='Grønn sone|accessPolicy'
RE_OPUS='nav-pilot-opus'

# ── Test 1 — trivial tier emits no phase checkpoint ──────────────────────────
# Invariant: "Classify scope tier before responding" + the Trivial row of
# `## Request scope classification` (single-pass, no phase stops).
if selected 1; then
  DESC1="trivial tier: no phase checkpoint emitted"
  if ! run_prompt t1 "fiks en skrivefeil i README"; then
    record_error 1 "$DESC1" "$LAST_PROMPT_DETAIL"
  elif absent "$WORKDIR/t1.txt" "$RE_CHECKPOINT"; then
    record 1 "$DESC1" 0
  else
    record 1 "$DESC1" 1 \
      "found a checkpoint in a trivial request — tier classification regressed"
  fi
fi

# ── Tests 2 + 3 — full tier stops after Fase 1, raises blind spots #1 and #2 ──
# One model call, two independent assertions (the prompt is identical, so
# re-running it would only pay twice for the same sample).
if selected 2 || selected 3; then
  DESC2="full tier: exactly one checkpoint, response stops after Fase 1"
  DESC3="full tier: blind spots #1 (personvern) and #2 (tilgangskontroll) both raised"
  T2="$WORKDIR/t2.txt"
  if ! run_prompt t2 "ny tjeneste som leser fnr fra ID-porten"; then
    if selected 2; then record_error 2 "$DESC2" "$LAST_PROMPT_DETAIL"; fi
    if selected 3; then record_error 3 "$DESC3" "$LAST_PROMPT_DETAIL"; fi
  else
    if selected 2; then
      n="$(count_of "$T2" "$RE_CHECKPOINT")"
      detail=""
      ok=0
      if [[ "$n" != "1" ]]; then
        ok=1; detail="expected exactly 1 phase checkpoint, found $n"
      elif ! present "$T2" 'Fase[[:space:]]+1[[:space:]]+ferdig'; then
        ok=1; detail="the single checkpoint was not the Fase 1 checkpoint"
      elif ! present "$T2" "$RE_CONFIRM"; then
        ok=1; detail="checkpoint did not ask the user to confirm before continuing"
      elif ! absent "$T2" "$RE_PHASE2_ARTIFACT"; then
        ok=1; detail="response leaked Fase 2 content (matched: $RE_PHASE2_ARTIFACT) — PHASE INTEGRITY rule regressed"
      fi
      record 2 "$DESC2" "$ok" "$detail"
    fi

    if selected 3; then
      # Blind spot #1 = Privacy, #2 = Access control. Assert the *topic* is
      # raised, in any phrasing the agent chooses.
      RE_BS1='personopplysning|persondata|personvern|GDPR|datakategori|behandlingsgrunnlag'
      RE_BS2='tilgangskontroll|hvem[[:space:]]+(skal[[:space:]]+)?kalle|hvem[[:space:]]+bruker|innbygger|saksbehandler|autorisasjon'
      ok=0; detail=""
      if ! present "$T2" "$RE_BS1"; then
        ok=1; detail="blind spot #1 (personvern) not raised"
      elif ! present "$T2" "$RE_BS2"; then
        ok=1; detail="blind spot #2 (tilgangskontroll) not raised"
      fi
      record 3 "$DESC3" "$ok" "$detail"
    fi
  fi
fi

# ── Test 4 — every Fase 2 plan declares a red zone ───────────────────────────
# Invariant: Boundaries → ✅ Always, "Include 🔴 Rød-sone-deklarasjon in every
# Phase 2 plan". A compressed-tier prompt is used because compressed tier
# traverses all phases in one response by design, so Fase 2 output is reachable
# in a single non-interactive call. "Rød sone: ingen" is a valid declaration —
# the invariant is that the declaration exists, not that anything is red.
#
# ⚠️  The red-zone assertion MUST be gated on Fase 2 output actually existing.
# The Fase 1 checkpoint template itself contains the line
# `• 🔴 Rød sone: [liste, eller «ingen»]`, so a bare present 'Rød sone' is
# satisfied by a response that stops at the Fase 1 checkpoint and never plans at
# all — i.e. it passes vacuously on exactly the regression it exists to catch.
# No Fase 2 output is "could not evaluate", never a pass.
if selected 4; then
  DESC4="Fase 2 output contains a 🔴 Rød sone declaration"
  T4="$WORKDIR/t4.txt"
  if ! run_prompt t4 "legg til et nytt REST-endepunkt i den eksisterende Ktor-tjenesten — flere filer, kjent mønster, ingen nye datastrømmer"; then
    record_error 4 "$DESC4" "$LAST_PROMPT_DETAIL"
  elif ! present "$T4" "$RE_PHASE2_ARTIFACT"; then
    record_error 4 "$DESC4" \
      "no Fase 2 content in the response (no match for: $RE_PHASE2_ARTIFACT) — the red-zone assertion would pass vacuously off the Fase 1 checkpoint template, so it was not evaluated. Re-run with --keep and check whether compressed-tier traversal regressed."
  elif present "$T4" 'Rød sone'; then
    record 4 "$DESC4" 0
  else
    record 4 "$DESC4" 1 \
      "no red-zone declaration in a Fase 2 plan — mandatory per Boundaries → ✅ Always"
  fi
fi

# ── Test 5 — user context means TokenX, never Azure client_credentials ───────
# ⚠️  THE CANARY. This is the assertion most likely to catch an over-aggressive
# cut to the authentication decision tree in `### Fase 2: Plan`, and to the
# `## Critical patterns` row "Azure client_credentials with user context →
# loses user audit trail → use TokenX". Both must survive for this to pass.
#
# Mentioning client_credentials is fine — the persona is expected to say "not
# client_credentials". So we fail only on a *recommending* phrasing.
if selected 5; then
  DESC5="user context → TokenX, not Azure client_credentials  ${YELLOW}(canary)${RESET}"
  T5="$WORKDIR/t5.txt"
  if ! run_prompt t5 "tjeneste A kaller tjeneste B med brukerkontekst — hvilken auth?"; then
    record_error 5 "$DESC5" "$LAST_PROMPT_DETAIL"
  else
    ok=0; detail=""
    if ! present "$T5" 'TokenX'; then
      ok=1; detail="TokenX never mentioned — the auth decision tree may have been cut too far"
    elif recommends_client_credentials "$T5"; then
      ok=1; detail="response appears to recommend Azure client_credentials for a user-context call"
    fi
    record 5 "$DESC5" "$ok" "$detail"
  fi
fi

# ── Test 6 — routine refactor does not escalate to Opus ──────────────────────
# Invariant: the model gate in `### Cost guardrails` — "Never use Opus for
# routine tasks, repo exploration, boilerplate, formatting, simple wiring,
# lint/test interpretation, or small refactors."
if selected 6; then
  DESC6="routine rename: no escalation to @nav-pilot-opus"
  if ! run_prompt t6 "rename variabelen maksAntall i tre filer"; then
    record_error 6 "$DESC6" "$LAST_PROMPT_DETAIL"
  elif absent "$WORKDIR/t6.txt" "$RE_OPUS"; then
    record 6 "$DESC6" 0
  else
    record 6 "$DESC6" 1 \
      "escalated to Opus for a small refactor — the model gate regressed"
  fi
fi

# ─── Summary ─────────────────────────────────────────────────────────────────

echo
if $JSON; then
  # `--only` can select nothing, leaving RESULTS empty. On bash 3.2 (stock
  # macOS) expanding an empty array under `set -u` is an unbound-variable error,
  # so guard the expansion and still emit a well-formed empty summary.
  if [[ ${#RESULTS[@]} -gt 0 ]]; then
    printf '%s\n' "${RESULTS[@]}"
  fi | jq -R -s 'split("\n") | map(select(length>0) | split("|")
    | {id: .[0], status: .[1], assertion: .[2], detail: .[3]})
    | {passed: (map(select(.status=="pass")) | length),
       failed: (map(select(.status=="fail")) | length),
       errored: (map(select(.status=="error")) | length),
       tests: .}'
fi

echo "─────────────────────────────────────────────"
if [[ $fail_count -eq 0 && $error_count -eq 0 ]]; then
  echo "${GREEN}${BOLD}All $pass_count assertions passed ✓${RESET}"
  exit 0
fi
if [[ $fail_count -eq 0 ]]; then
  echo "${YELLOW}${BOLD}$pass_count passed, $error_count not evaluated ⚠${RESET}"
  echo "${DIM}A test that could not run has proven nothing — this is not a green run.${RESET}"
  echo "${DIM}Re-run with --keep -v to see what the CLI actually returned.${RESET}"
  exit 3
fi
echo "${RED}${BOLD}$fail_count of $((pass_count + fail_count)) evaluated assertions failed ✗${RESET}"
[[ $error_count -gt 0 ]] && echo "${YELLOW}$error_count further test(s) could not be evaluated ⚠${RESET}"
echo "${DIM}Re-run with --keep to inspect the transcripts before relaxing an assertion.${RESET}"
echo "${DIM}These are live model calls: confirm a failure reproduces before treating it as a regression.${RESET}"
exit 1
