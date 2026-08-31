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
#   The agent EDITS that workspace: t1 fixes a typo, t4 adds an endpoint, t6
#   renames a variable across three files. So the workspace is rebuilt from a
#   pristine template before EVERY prompt, not once per suite and not once per
#   --repeat pass. Two samples of one prompt have to meet the same repo, or
#   the median --repeat exists to produce is a median over a fixture that
#   drifted: run 2 of t1 would find no typo left to fix, and run 2 of t6 no
#   `maksAntall` left to rename.
#
# WHAT THE SCRATCH WORKSPACE CONTAINS
#   ONE agent (the persona under test, at .github/agents/) plus every
#   instructions/*.instructions.md at .github/instructions/.
#
#   That is NOT what `nav-pilot install --repo` gives a user. A real repo-scope
#   install also brings the other 12 agents, all 32 skills and 7 prompts, and a
#   collection install brings a different subset again. No real flow produces
#   "one agent plus every instruction". The narrow selection is deliberate,
#   because the harness tests the persona and everything else in a real install
#   is context the persona did not write, but it has a consequence:
#
#     ⚠️  The absolute sizes this harness reports are NOT what a user sees. The
#     persona routes to artifacts that are absent here: `## Related skills`,
#     the contextual skill routing and the delegation tables all point at
#     skills and agents this workspace does not contain, so a real answer is
#     grounded in more material than these transcripts are. Test 5 is an auth
#     question, and a real user would have the auth skills in scope. Only the
#     with-versus-without DELTA between two runs made the same way means
#     anything. A single absolute number off this harness is not a claim about
#     what a user's answer costs.
#
#   The layout and the copying are real even though the selection is not: repo
#   scope copies instructions verbatim (see installArtifact() in
#   cli/nav-pilot/internal/cli/install.go and DstPath() in
#   internal/domain/domain.go), so the harness copies them verbatim too. There
#   is nothing to reimplement and therefore nothing to drift.
#
#   An instruction with `applyTo: "**"` (or with no applyTo at all) is in scope
#   for every prompt below. That is what makes it possible to measure such a
#   rule at all: before the workspace carried instructions, the harness measured
#   the persona alone and an always-on rule was invisible to it. Pass
#   --no-instructions for persona-only runs, comparable with pre-2026-08 results.
#
#   The AGENTS.md inlining in internal/artifacts/export.go is deliberately NOT
#   mirrored here. That rule (applyTo empty or "**" → inline; anything else →
#   a lazy reference under "## Context Loading") belongs to the *opencode*
#   export target. The Copilot CLI this harness drives reads
#   .github/instructions/ directly, and `nav-pilot install` writes no AGENTS.md.
#
# MEASURING OUTPUT SIZE
#   Behavioural assertions answer "did it still do the right thing". They cannot
#   answer "did it get shorter", which is the question an always-on output-style
#   instruction raises. So every run also records bytes, lines and words per
#   transcript, and reports the median with its min/max spread. Size is
#   reported, never asserted: a size change never fails the run. The spread is
#   the point: a 5% median delta inside a 40% spread is noise.
#
#   Model output is non-deterministic, so a single sample proves nothing. Use
#   --repeat N (N runs per prompt, median across them), --save-baseline to
#   record a run, and --compare to diff a later run against that record.
#
#   Commit baselines as docs/golden-baselines/<date>-<label>.txt. They are
#   observations, and they belong with the other recorded analyses in docs/,
#   not with the code. The file's own header repeats the date, revision, model
#   and repeat count, because a size baseline means nothing without them, and
#   because nobody should mistake it for a threshold something must meet.
#
# PASS/FAIL ACROSS REPEATS
#   A test passes only if *every* run of it passed. One failure in five runs is
#   a failure, reported as "k/N passed": a canary that fails intermittently has
#   still caught something.
#
#   One un-evaluable run in five is likewise not a pass. A test with no failed
#   run but at least one dead one (empty transcript, timeout, a response that
#   never reached the phase under test) is "not evaluated" (exit 3), whether
#   that is one run of five or all five. Same stance as a single dead run at
#   --repeat 1: a test that did not run has proven nothing, so a flaky CLI must
#   not be able to report green off whichever runs happened to survive. The
#   size row's `n=` says how many transcripts the median was actually taken
#   over, which is the same number.
#
#   At --repeat 1 all of this is exactly the old behaviour.
#
# COST
#   One pass is up to 5 live model calls (tests 2 and 3 share one prompt).
#   --repeat N multiplies that: --repeat 5 is ~25 calls.
#
# USAGE
#   ./scripts/nav-pilot-golden.sh                 # run all tests
#   ./scripts/nav-pilot-golden.sh --only 2,5      # run selected tests
#   ./scripts/nav-pilot-golden.sh --keep          # keep transcripts for inspection
#   ./scripts/nav-pilot-golden.sh --model <model> # pin a model (default: CLI default)
#   ./scripts/nav-pilot-golden.sh --json          # machine-readable summary (needs jq)
#   ./scripts/nav-pilot-golden.sh -v              # echo each transcript as it lands
#   ./scripts/nav-pilot-golden.sh --repeat 5      # 5 samples per prompt, median reported
#   ./scripts/nav-pilot-golden.sh --no-instructions          # persona only
#   ./scripts/nav-pilot-golden.sh --save-baseline <path>     # record sizes
#   ./scripts/nav-pilot-golden.sh --compare <path>           # diff sizes vs a record
#   ./scripts/nav-pilot-golden.sh --dry-run       # print the workspace, call no model
#
# EXIT CODES
#   0  all selected assertions passed
#   1  at least one assertion failed
#   2  preflight failed (no client, not authenticated, persona missing, bad flag)
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
REPEAT=1
WITH_INSTRUCTIONS=true
SAVE_BASELINE=""
COMPARE_TO=""
DRY_RUN=false

# A flag that takes a value must be given one. Without this guard `shift 2`
# fails when the flag is the last argument ($# is 1), and with no `set -e` it
# fails *silently* without shifting, so the loop below spins forever on the
# same argument. One guard covers every value-taking flag; add it to any new
# one. Called as `need_val "$@"`: $1 is the flag, $# the arguments left.
need_val() {
  [[ $# -ge 2 ]] || { echo "$1 needs a value (try --help)" >&2; exit 2; }
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --only)    need_val "$@"; ONLY="$2"; shift 2 ;;
    --keep)    KEEP=true; shift ;;
    --model)   need_val "$@"; MODEL="$2"; shift 2 ;;
    --json)    JSON=true; shift ;;
    --repeat)  need_val "$@"; REPEAT="$2"; shift 2 ;;
    --no-instructions) WITH_INSTRUCTIONS=false; shift ;;
    --save-baseline)   need_val "$@"; SAVE_BASELINE="$2"; shift 2 ;;
    --compare)         need_val "$@"; COMPARE_TO="$2"; shift 2 ;;
    --dry-run) DRY_RUN=true; shift ;;
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

[[ "$REPEAT" =~ ^[1-9][0-9]*$ ]] || fail_preflight \
  "--repeat takes a positive integer, got '$REPEAT'" \
  "Each repeat costs another full set of live model calls."

[[ -z "$COMPARE_TO" || -f "$COMPARE_TO" ]] || fail_preflight \
  "--compare: no baseline file at $COMPARE_TO" \
  "Record one first: ./scripts/nav-pilot-golden.sh --repeat 5 --save-baseline $COMPARE_TO"

CLI_PATH=""
CLI_NAME="(dry run, no client)"

# preflight_client resolves and probes the CLI. Skipped by --dry-run, which
# builds and prints the scratch workspace and stops: that path exists so the
# workspace materialization can be checked without a client, an account, or a
# bill, so it must not require any of them.
preflight_client() {
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
  local probe_out probe_rc
  probe_out="$("$CLI_PATH" -p "svar kun med ordet OK" --no-color --log-level none 2>&1)"
  probe_rc=$?
  if [[ $probe_rc -ne 0 ]] || grep -qiE "not (logged in|authenticated)|unauthorized|401|please (log ?in|sign in)|GITHUB_TOKEN" <<<"$probe_out"; then
    fail_preflight \
      "$CLI_NAME is not authenticated (probe prompt failed)" \
      "Run '$CLI_NAME' interactively and complete login, or export a valid GITHUB_TOKEN. Probe said: $(head -c 200 <<<"$probe_out")"
  fi
}

$DRY_RUN || preflight_client

if $JSON && ! command -v jq >/dev/null 2>&1; then
  fail_preflight "--json needs jq" "brew install jq"
fi

# ─── Throwaway workspace ─────────────────────────────────────────────────────
# WORKDIR holds the harness's own files (transcripts, per-run rows) and the
# pristine $TEMPLATE. $WS is the fake repo the agent is pointed at, and holds
# nothing else: the agent explores it in Fase 1, so it must find neither run
# 1's transcript nor the pristine copy of itself lying there, or run 2 is
# measuring a different repo than run 1.
#
# The template is seeded with just enough of a Nav repo that the prompts are
# grounded: the persona infers archetype from nais.yaml / build.gradle.kts in
# Fase 1. $WS is a fresh copy of it before every prompt (see seed_ws below).

WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/nav-pilot-golden.XXXXXX")"
TEMPLATE="$WORKDIR/template"
WS="$WORKDIR/repo"
# shellcheck disable=SC2329  # invoked via trap
cleanup() {
  if $KEEP; then
    echo "${DIM}transcripts kept in $WORKDIR${RESET}"
  else
    rm -rf "$WORKDIR"
  fi
}
trap cleanup EXIT

mkdir -p "$TEMPLATE/.github/agents" "$TEMPLATE/src/main/kotlin/no/nav/demo"
cp "$PERSONA" "$TEMPLATE/.github/agents/$AGENT_NAME.agent.md"

# Instructions, laid out the way `nav-pilot install --repo` lays them out:
# .github/instructions/<name>.instructions.md, byte-for-byte from the checkout.
# Repo scope applies no transformation (install.go → installArtifact copies the
# file; domain.go → DstPath picks the path), so neither does this.
#
# ALWAYS_ON_COUNT counts what is in scope for *every* prompt, by export.go's
# rule (collectInstructionData: applyTo empty or "**" is global, anything else
# is glob-scoped). It is printed because it is the number that decides whether
# a before/after size measurement can see anything at all: with zero always-on
# instructions, an output-style rule cannot have moved the numbers.

# always_on <file> → 0 if the instruction is in scope for every prompt.
#
# Mirrors source.ExtractFrontmatterValue as export.go calls it: the value is
# read from the *frontmatter block only* (a mention of applyTo down in prose
# does not count), the first match wins, and one layer of surrounding quotes is
# stripped, single as well as double, so `applyTo: '**'` counts as global too.
# No frontmatter, or frontmatter without applyTo, means empty, means global.
always_on() {
  local in_fm=0 line val
  while IFS= read -r line; do
    if [[ "$line" == "---" ]]; then
      [[ $in_fm -eq 1 ]] && break
      in_fm=1
      continue
    fi
    [[ $in_fm -eq 1 ]] || break
    [[ "$line" == applyTo:* ]] || continue
    val="$(printf '%s' "${line#applyTo:}" \
      | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' \
            -e 's/^"\(.*\)"$/\1/' -e "s/^'\(.*\)'\$/\1/")"
    [[ -z "$val" || "$val" == '**' ]] && return 0
    return 1
  done <"$1"
  return 0
}

INSTR_COUNT=0
ALWAYS_ON_COUNT=0
if $WITH_INSTRUCTIONS && [[ -d "$REPO_ROOT/instructions" ]]; then
  mkdir -p "$TEMPLATE/.github/instructions"
  for instr in "$REPO_ROOT"/instructions/*.instructions.md; do
    [[ -f "$instr" ]] || continue
    cp "$instr" "$TEMPLATE/.github/instructions/"
    INSTR_COUNT=$((INSTR_COUNT + 1))
    always_on "$instr" && ALWAYS_ON_COUNT=$((ALWAYS_ON_COUNT + 1))
  done
fi

# One string, used both in the --save-baseline header and in the --compare
# compatibility check. They must be the same string or the check is a lie.
if $WITH_INSTRUCTIONS; then
  INSTR_DESC="$INSTR_COUNT installed, $ALWAYS_ON_COUNT always-on"
else
  INSTR_DESC="none (--no-instructions)"
fi

cat >"$TEMPLATE/README.md" <<'EOF'
# demo-tjeneste

En liten Ktor-tjeneste for demonstrasjon. Tjenesten eksponerer et REST-API
og kjører på Nais. Dokumentasjonen er dessverre ikke helt komplet enda.
EOF

cat >"$TEMPLATE/nais.yaml" <<'EOF'
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

cat >"$TEMPLATE/build.gradle.kts" <<'EOF'
plugins { kotlin("jvm") version "2.1.0" }
dependencies { implementation("io.ktor:ktor-server-netty:3.0.0") }
EOF

for f in App Routes Config; do
  cat >"$TEMPLATE/src/main/kotlin/no/nav/demo/$f.kt" <<EOF
package no.nav.demo

// bruker maksAntall flere steder
val maksAntall = 100
EOF
done

# The agent writes to $WS (t1 fixes the README typo, t4 adds an endpoint, t6
# renames maksAntall), so $WS is thrown away and rebuilt from $TEMPLATE before
# every prompt. Per prompt, not per --repeat pass: within one pass the prompts
# also touch each other's files, and re-copying is cheap enough that the
# stronger guarantee costs nothing.
#
# rm -rf, never a copy over the top: the agent may have *created* files, and a
# file the template does not contain has nothing to overwrite it.
seed_ws() {
  rm -rf "$WS"
  cp -R "$TEMPLATE" "$WS"
}
seed_ws

# ─── Test runner ─────────────────────────────────────────────────────────────

# Per-run rows, one file each, aggregated after the last run:
#   RESULTS_FILE  id|status|assertion|detail          (one row per test per run)
#   MEASURES      slug|bytes|lines|words              (one row per transcript)
# Files rather than arrays because the aggregation reads them repeatedly, and
# because bash 3.2 (stock macOS) makes an empty array an unbound-variable error
# under `set -u`, while an empty file just reads as nothing.
RESULTS_FILE="$WORKDIR/results.psv"
MEASURES="$WORKDIR/measures.psv"
: >"$RESULTS_FILE"
: >"$MEASURES"

pass_count=0
fail_count=0
error_count=0
RUN=1

# Transcript path for a prompt in the current run. Repeats must not overwrite
# each other: --keep has to leave all N samples behind for inspection.
tx() { printf '%s/%s.run%s.txt' "$WORKDIR" "$1" "$RUN"; }

run_tag() { [[ "$REPEAT" -gt 1 ]] && printf '%s[run %s/%s]%s ' "$DIM" "$RUN" "$REPEAT" "$RESET"; return 0; }

# A transcript shorter than this is treated as "the call did not happen", not as
# a response. Every assertion below is either an absent() — which succeeds
# trivially on an empty file — or a present() on a long structured block, so
# without this floor a crashed or unauthenticated CLI reports green.
MIN_TRANSCRIPT_BYTES=200
LAST_PROMPT_DETAIL=""

run_prompt() {
  # run_prompt <slug> <prompt> → writes transcript to $(tx <slug>)
  # Returns 0 if the transcript is usable, 1 if it is missing/too short to
  # assert against. Callers MUST branch on this — see record_error.
  local slug="$1" prompt="$2" out
  out="$(tx "$slug")"
  local -a args=(-p "$prompt" --agent "$AGENT_NAME" --allow-all-tools --no-color --log-level none)
  [[ -n "$MODEL" ]] && args+=(--model "$MODEL")

  # Fresh repo per prompt. Three of the prompts below tell the agent to edit
  # this workspace, so without this the second sample of t1 finds no typo.
  seed_ws

  echo "${DIM}  → $(run_tag)prompting ($slug)…${RESET}" >&2
  local -a runner=()
  [[ -n "$TIMEOUT_BIN" ]] && runner=("$TIMEOUT_BIN" "$TIMEOUT_SECS")
  # ${arr[@]+"${arr[@]}"} — bash 3.2 (stock macOS) treats an empty array as an
  # unbound variable under `set -u`, and the no-coreutils fallback above leaves
  # `runner` empty on exactly that platform.
  ( cd "$WS" && ${runner[@]+"${runner[@]}"} "$CLI_PATH" "${args[@]}" ) >"$out" 2>"${out%.txt}.err"
  local rc=$?
  if [[ $rc -eq 124 ]]; then
    echo "${YELLOW}⚠ $slug: timed out after ${TIMEOUT_SECS}s (raise NAV_PILOT_GOLDEN_TIMEOUT)${RESET}" >&2
  fi
  $VERBOSE && { echo "${DIM}--- $slug ---${RESET}"; cat "$out"; echo "${DIM}--- end ---${RESET}"; }

  local size=0
  [[ -f "$out" ]] && size="$(wc -c <"$out" | tr -d ' ')"
  if [[ "$size" -lt "$MIN_TRANSCRIPT_BYTES" ]]; then
    echo "${YELLOW}⚠ $slug: CLI exited $rc with ${size}B of output (need ≥${MIN_TRANSCRIPT_BYTES}B)${RESET}" >&2
    head -c 300 "${out%.txt}.err" >&2; echo >&2
    LAST_PROMPT_DETAIL="CLI exited $rc, transcript was ${size}B (<${MIN_TRANSCRIPT_BYTES}B) — nothing to assert against"
    return 1
  fi

  # Size is measured for every usable transcript, whatever the assertions then
  # say about it. A dead transcript is deliberately not measured: its length
  # describes the failure, not the persona.
  printf '%s|%s|%s|%s\n' "$slug" "$size" \
    "$(wc -l <"$out" | tr -d ' ')" "$(wc -w <"$out" | tr -d ' ')" >>"$MEASURES"
  return 0
}

record() {
  # record <id> <description> <ok:0|1> [detail]
  # Records ONE run. The pass/fail of the test as a whole is decided in the
  # aggregation below, once every repeat has been recorded.
  local id="$1" desc="$2" ok="$3" detail="${4:-}"
  if [[ "$ok" == "0" ]]; then
    echo "  ${GREEN}✓${RESET} ${BOLD}$id${RESET} $(run_tag)$desc"
    printf '%s|pass|%s|\n' "$id" "$desc" >>"$RESULTS_FILE"
  else
    echo "  ${RED}✗${RESET} ${BOLD}$id${RESET} $(run_tag)$desc"
    [[ -n "$detail" ]] && echo "      ${DIM}$detail${RESET}"
    printf '%s|fail|%s|%s\n' "$id" "$desc" "$detail" >>"$RESULTS_FILE"
  fi
}

record_error() {
  # record_error <id> <description> <detail>
  # For tests that could not be evaluated at all — a dead transcript, or a
  # response that never reached the phase the assertion is about. Distinct from
  # both pass and fail on purpose: reporting green here is the failure mode that
  # gets the whole harness ignored.
  local id="$1" desc="$2" detail="${3:-}"
  echo "  ${YELLOW}⚠${RESET} ${BOLD}$id${RESET} $(run_tag)$desc ${YELLOW}(not evaluated)${RESET}"
  [[ -n "$detail" ]] && echo "      ${DIM}$detail${RESET}"
  printf '%s|error|%s|%s\n' "$id" "$desc" "$detail" >>"$RESULTS_FILE"
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
echo "${DIM}client: $CLI_NAME${CLI_PATH:+ ($CLI_PATH)}${RESET}"
echo "${DIM}persona: $PERSONA${RESET}"
if $WITH_INSTRUCTIONS; then
  echo "${DIM}instructions: $INSTR_COUNT in .github/instructions/, $ALWAYS_ON_COUNT always-on (applyTo \"**\")${RESET}"
else
  echo "${DIM}instructions: none (--no-instructions), persona only${RESET}"
fi
echo "${DIM}repeats: $REPEAT${RESET}"
echo "${DIM}workspace: $WS${RESET}"
echo

if $DRY_RUN; then
  echo "${BOLD}scratch workspace (no model calls made)${RESET}"
  ( cd "$WS" && find . -type f | sort | sed 's|^\./|  |' )
  echo
  echo "${DIM}--dry-run stops here. Drop it to make live calls.${RESET}"
  exit 0
fi

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

# One pass over the selected prompts. Called once per --repeat, so every
# assertion below sees a fresh sample; `tx` keeps the transcripts apart.
run_pass() {
  # ── Test 1 — trivial tier emits no phase checkpoint ──────────────────────────
  # Invariant: "Classify scope tier before responding" + the Trivial row of
  # `## Request scope classification` (single-pass, no phase stops).
  if selected 1; then
    DESC1="trivial tier: no phase checkpoint emitted"
    if ! run_prompt t1 "fiks en skrivefeil i README"; then
      record_error 1 "$DESC1" "$LAST_PROMPT_DETAIL"
    elif absent "$(tx t1)" "$RE_CHECKPOINT"; then
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
    T2="$(tx t2)"
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
    T4="$(tx t4)"
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
    T5="$(tx t5)"
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
    elif absent "$(tx t6)" "$RE_OPUS"; then
      record 6 "$DESC6" 0
    else
      record 6 "$DESC6" 1 \
        "escalated to Opus for a small refactor — the model gate regressed"
    fi
  fi
}

for ((RUN = 1; RUN <= REPEAT; RUN++)); do
  run_pass
done

# ─── Aggregate ───────────────────────────────────────────────────────────────
# Everything below reads the per-run rows and collapses them to one row per
# test and one row per prompt. With --repeat 1 the collapse is the identity,
# so the reported result is byte-for-byte what the harness reported before.

AGG_TESTS="$WORKDIR/agg-tests.psv"
AGG_SIZES="$WORKDIR/agg-sizes.psv"
: >"$AGG_TESTS"
: >"$AGG_SIZES"

# stats: reads one integer per line, prints "median min max".
# Median, not mean: one 12kB response where the others are 3kB should not drag
# the number it is compared against. An even sample count averages the two
# middle values and truncates, which is close enough for byte counts.
stats() {
  sort -n | awk '
    { v[NR] = $1 }
    END {
      if (NR == 0) { print "0 0 0"; exit }
      m = (NR % 2) ? v[(NR + 1) / 2] : (v[NR / 2] + v[NR / 2 + 1]) / 2
      printf "%d %d %d\n", m, v[1], v[NR]
    }'
}

# uniq_field <file> <field> → distinct values, in first-seen order.
uniq_field() { cut -d'|' -f"$2" "$1" | awk '!seen[$0]++'; }

for id in $(uniq_field "$RESULTS_FILE" 1); do
  np="$(grep -c "^$id|pass|" "$RESULTS_FILE")"
  nf="$(grep -c "^$id|fail|" "$RESULTS_FILE")"
  ne="$(grep -c "^$id|error|" "$RESULTS_FILE")"
  desc="$(grep -m1 "^$id|" "$RESULTS_FILE" | cut -d'|' -f3)"
  # -f4- , not -f4: a detail can itself contain a pipe. Test 4 quotes the
  # regex "Grønn sone|accessPolicy" in its message, and cutting that at the
  # pipe silently changes what the failure says. Detail is the last field in
  # every row below for the same reason.
  detail="$(grep -m1 "^$id|fail|" "$RESULTS_FILE" | cut -d'|' -f4-)"
  [[ -z "$detail" ]] && detail="$(grep -m1 "^$id|error|" "$RESULTS_FILE" | cut -d'|' -f4-)"

  # Any failing run fails the test. A model that emits the right answer four
  # times in five has still lost the invariant; hiding that behind a majority
  # vote would make the canary quieter exactly as it starts to matter.
  #
  # Any dead run is likewise not a pass. Two passes and one dead transcript is
  # "not evaluated" (exit 3), not green. Same stance as a single dead run at
  # --repeat 1: a test that did not run has proven nothing, and a CLI timing
  # out four times in five must not report a green suite off the fifth.
  if [[ "$nf" -gt 0 ]]; then
    status="fail"; fail_count=$((fail_count + 1))
  elif [[ "$ne" -gt 0 || "$np" -eq 0 ]]; then
    status="error"; error_count=$((error_count + 1))
  else
    status="pass"; pass_count=$((pass_count + 1))
  fi
  printf '%s|%s|%s|%s|%s|%s|%s\n' "$id" "$status" "$np" "$nf" "$ne" "$desc" "$detail" >>"$AGG_TESTS"
done

for slug in $(uniq_field "$MEASURES" 1); do
  rows="$(grep "^$slug|" "$MEASURES")"
  n="$(wc -l <<<"$rows" | tr -d ' ')"
  read -r b_med b_min b_max <<<"$(cut -d'|' -f2 <<<"$rows" | stats)"
  read -r l_med l_min l_max <<<"$(cut -d'|' -f3 <<<"$rows" | stats)"
  read -r w_med w_min w_max <<<"$(cut -d'|' -f4 <<<"$rows" | stats)"
  printf '%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s\n' "$slug" "$n" \
    "$b_med" "$b_min" "$b_max" "$l_med" "$l_min" "$l_max" "$w_med" "$w_min" "$w_max" >>"$AGG_SIZES"
done

# ─── Summary ─────────────────────────────────────────────────────────────────

echo
if [[ "$REPEAT" -gt 1 && -s "$AGG_TESTS" ]]; then
  echo "${BOLD}Across $REPEAT runs${RESET}"
  # detail last: `read` puts every remaining field, pipes and all, in the last
  # variable, so a detail containing a pipe survives intact.
  while IFS='|' read -r id status np nf ne desc detail; do
    case "$status" in
      pass) mark="${GREEN}✓${RESET}" ;;
      fail) mark="${RED}✗${RESET}" ;;
      *)    mark="${YELLOW}⚠${RESET}" ;;
    esac
    echo "  $mark ${BOLD}$id${RESET} $desc ${DIM}($np/$REPEAT passed, $nf failed, $ne not evaluated)${RESET}"
    [[ "$status" != "pass" && -n "$detail" ]] && echo "      ${DIM}$detail${RESET}"
  done <"$AGG_TESTS"
  echo
fi

if [[ -s "$AGG_SIZES" ]]; then
  echo "${BOLD}Response size${RESET} ${DIM}(median per prompt over the n usable transcripts of $REPEAT, spread in brackets)${RESET}"
  printf '  %-6s %9s %8s %8s   %s\n' "prompt" "bytes" "lines" "words" "bytes min-max"
  while IFS='|' read -r slug n b_med b_min b_max l_med l_min l_max w_med w_min w_max; do
    printf '  %-6s %9s %8s %8s   %s\n' "$slug" "$b_med" "$l_med" "$w_med" "${DIM}$b_min-$b_max (n=$n)${RESET}"
  done <"$AGG_SIZES"
  echo "${DIM}Sizes are reported, never asserted. A wide spread means a small median delta is noise.${RESET}"
  echo
fi

if [[ -n "$SAVE_BASELINE" ]]; then
  # The header is the point of the file: a size baseline is only meaningful
  # next to the run conditions that produced it. Anyone reading it must see
  # immediately that these are recorded numbers, not targets to hit.
  {
    echo "# nav-pilot golden-prompt SIZE MEASUREMENT: RECORDED, NOT A TARGET"
    echo "# Nothing asserts against these numbers, and no run fails because it"
    echo "# missed them. They describe one revision, on one model, on one day."
    echo "# Only comparable with another run made the same way (--compare)."
    echo "# date:         $(date -u +%Y-%m-%d)"
    echo "# revision:     $(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"
    echo "# client:       $CLI_NAME"
    echo "# model:        ${MODEL:-CLI default}"
    echo "# repeats:      $REPEAT"
    echo "# instructions: $INSTR_DESC"
    echo "# prompts:      ${ONLY:-all}"
    echo "#"
    echo "# slug|runs|bytes_median|bytes_min|bytes_max|lines_median|lines_min|lines_max|words_median|words_min|words_max"
    cat "$AGG_SIZES"
  } >"$SAVE_BASELINE"
  echo "${DIM}size baseline written to $SAVE_BASELINE${RESET}"
  echo
fi

# compat_warn <header field> <this run's value>: shout when the baseline was
# recorded under different conditions. Printing both headers next to each other
# is disclosure, not a check. A baseline from another model, or with
# instructions off, is not comparable, and nothing before this noticed. It warns
# rather than refuses, and it deliberately does not touch the exit code: no
# size path may decide whether the run is green.
compat_warn() {
  local field="$1" want="$2" got
  got="$(sed -n "s/^# $field:[[:space:]]*//p" "$COMPARE_TO" | head -1)"
  if [[ -n "$got" && "$got" != "$want" ]]; then
    echo "  ${YELLOW}⚠ baseline $field: '$got', this run: '$want'. Not comparable.${RESET}"
  fi
  return 0
}

if [[ -n "$COMPARE_TO" ]]; then
  echo "${BOLD}Size vs baseline${RESET} ${DIM}$COMPARE_TO${RESET}"
  while IFS= read -r line; do
    echo "  ${DIM}$line${RESET}"
  done < <(grep -E '^# [a-z]+: ' "$COMPARE_TO")
  compat_warn model "${MODEL:-CLI default}"
  compat_warn instructions "$INSTR_DESC"
  compat_warn repeats "$REPEAT"
  compat_warn prompts "${ONLY:-all}"
  printf '  %-6s %10s %10s %10s %8s\n' "prompt" "baseline" "current" "delta" "pct"
  while IFS='|' read -r slug n b_med rest; do
    base="$(grep -m1 "^$slug|" "$COMPARE_TO" | cut -d'|' -f3)"
    if [[ -z "$base" ]]; then
      printf '  %-6s %10s %10s %10s %8s\n' "$slug" "-" "$b_med" "-" "-"
      continue
    fi
    printf '  %-6s %10s %10s %10s %8s\n' "$slug" "$base" "$b_med" \
      "$((b_med - base))" \
      "$(awk -v b="$base" -v c="$b_med" 'BEGIN { if (b == 0) print "n/a"; else printf "%+.1f%%", (c - b) * 100 / b }')"
  done <"$AGG_SIZES"
  echo "${DIM}Bytes, median. A size change never fails the run.${RESET}"
  echo
fi

if $JSON; then
  # One jq pass over both aggregates. Each row is tagged with its kind so a
  # single -R -s slurp can split them; --only can select nothing, and an empty
  # stream still yields a well-formed summary.
  { sed 's/^/test|/' "$AGG_TESTS"; sed 's/^/size|/' "$AGG_SIZES"; } \
    | jq -R -s --argjson repeat "$REPEAT" \
        --argjson instructions "$($WITH_INSTRUCTIONS && echo true || echo false)" '
    (split("\n") | map(select(length > 0) | split("|"))) as $rows
    | ($rows | map(select(.[0] == "test") | {
        id: .[1], status: .[2], assertion: .[6],
        detail: (.[7:] | join("|")),
        runs: {pass: (.[3] | tonumber), fail: (.[4] | tonumber), error: (.[5] | tonumber)}
      })) as $tests
    | {passed:  ($tests | map(select(.status == "pass"))  | length),
       failed:  ($tests | map(select(.status == "fail"))  | length),
       errored: ($tests | map(select(.status == "error")) | length),
       repeat: $repeat,
       instructions: $instructions,
       tests: $tests,
       sizes: ($rows | map(select(.[0] == "size") | {
         slug: .[1], runs: (.[2] | tonumber),
         bytes: {median: (.[3] | tonumber), min: (.[4] | tonumber), max: (.[5] | tonumber)},
         lines: {median: (.[6] | tonumber), min: (.[7] | tonumber), max: (.[8] | tonumber)},
         words: {median: (.[9] | tonumber), min: (.[10] | tonumber), max: (.[11] | tonumber)}
       }))}'
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
if [[ "$REPEAT" -gt 1 ]]; then
  echo "${DIM}A test fails if any of its $REPEAT runs failed. The per-test counts above say how many.${RESET}"
else
  echo "${DIM}These are live model calls: confirm a failure reproduces (--repeat 3) before treating it as a regression.${RESET}"
fi
exit 1
