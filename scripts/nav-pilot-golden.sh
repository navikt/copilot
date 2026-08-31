#!/usr/bin/env bash
#
# Golden-prompt harness: behavioural regression test for an agent under test.
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
# WHICH AGENT (--agent, default nav-pilot)
#   Repinning an agent to a cheaper model is the same class of change as
#   trimming a persona: nothing fails, the answers just get quietly worse. So
#   the agent under test is a parameter. `--agent <key>` reads
#   agents/<key>.agent.md, installs that one agent into the scratch workspace,
#   and runs the assertion group written for it:
#
#     nav-pilot      tests 1-6      phase discipline, blind spots, auth, model gate
#     code-review    tests cr1-cr4  findings schema, no auto-fix, teaching, routing
#     accessibility  tests uu1-uu4  WCAG substance, Ask-First, no subagent fan-out
#
#   IDs are prefixed per agent rather than renumbered, so `--only` never means
#   two different things and a baseline row cannot be silently misread. Only one
#   group runs per invocation; an agent with no group is a preflight failure,
#   because a run that selects zero assertions would otherwise report a green
#   "All 0 assertions passed".
#
#   Each group is derived from the agent file it tests, cited by line, the same
#   way tests 1-6 cite `## Boundaries` in the persona. Adding an agent means
#   reading its file and writing a `run_pass_<agent>`, not a second harness.
#
#   BEHAVIOUR VS FORMAT
#   A hard assertion states what the agent must *do*. It must be derived from
#   observed transcripts, not from a template in the persona, and it must hold
#   across models and persona revisions. Anything that pins the agent's exact
#   *wording* is not a hard assertion, however well motivated: the persona's
#   wording is allowed to drift.
#
#   Where a format is genuinely wanted but is not reliably produced, record it
#   with `record_soft` instead. A soft check prints every run and never touches
#   the exit code. It exists so an unmet want stays visible without a permanently
#   red suite, which is the fastest way to teach people that red means nothing.
#   Test 2b is the worked example, and its history is in git: three persona
#   revisions across four models, 18 transcripts, zero checkpoint blocks. cr4 is
#   the second, and the plainer one: it greps for an agent handle in prose.
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
#   The agent EDITS that workspace: t1 fixes a typo, t4 extends a handler, t6
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
#   One live model call per prompt, not per assertion: assertions that can be
#   read off the same transcript share it.
#     nav-pilot      5 calls per pass (tests 2 and 3 share one prompt)
#     code-review    2 calls per pass (cr1, cr2 and cr3 share one)
#     accessibility  3 calls per pass (uu1 and uu2 share one)
#   --repeat N multiplies that: nav-pilot at --repeat 5 is ~25 calls.
#
# USAGE
#   ./scripts/nav-pilot-golden.sh                 # run all tests
#   ./scripts/nav-pilot-golden.sh --agent code-review    # test another agent
#   ./scripts/nav-pilot-golden.sh --agent accessibility  # ditto
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
#   2  preflight failed (no client, not authenticated, agent file missing,
#      an --agent with no assertion group, bad flag)
#   3  nothing failed, but nothing was proven either: at least one test could
#      not be evaluated (empty transcript, or the response never reached the
#      phase under test), or the selection contained no assertion that can fail
#      at all (--only naming only soft IDs, like `--only 2b`).
#      This is deliberately NOT 0: a test that never ran has proven nothing.
#
#   Soft checks (ids like 2b) never change the exit code, in either direction.
#
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TIMEOUT_SECS="${NAV_PILOT_GOLDEN_TIMEOUT:-300}"

# AGENT is the file key: agents/$AGENT.agent.md, and the name every ID, slug,
# baseline header and summary line is written under. AGENT_NAME is what the CLI
# is told to load, and is resolved from the agent file's own frontmatter below.
# The two are not always the same string (accessibility.agent.md declares
# `name: accessibility-agent`), and passing the wrong one silently runs the
# default agent instead of the one under test.
AGENT="nav-pilot"
PERSONA=""
AGENT_NAME=""
VALID_IDS=""

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
    --agent)   need_val "$@"; AGENT="$2"; shift 2 ;;
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

PERSONA="$REPO_ROOT/agents/$AGENT.agent.md"

[[ -f "$PERSONA" ]] || fail_preflight \
  "agent file not found at $PERSONA" \
  "Run this script from a copilot checkout; it tests the working-tree agent file. Available: $(find "$REPO_ROOT/agents" -name '*.agent.md' -exec basename {} .agent.md \; 2>/dev/null | sort | tr '\n' ' ')"

# An --agent with no assertion group must not run. It would select zero tests
# and print "All 0 assertions passed ✓" with exit 0, which is the loudest
# possible vacuous pass: someone repins a model, sees green, ships.
#
# VALID_IDS is that list made explicit, so --only can be checked against it.
# Keep each row in sync with the record_* IDs in the matching run_pass_<agent>:
# an ID added there and not here is rejected by --only.
case "$AGENT" in
  nav-pilot)     VALID_IDS="1 2 2b 3 4 5 6" ;;
  code-review)   VALID_IDS="cr1 cr2 cr3 cr4" ;;
  accessibility) VALID_IDS="uu1 uu2 uu3 uu4" ;;
  *) fail_preflight \
      "no assertion group for agent '$AGENT'" \
      "This harness has prompts and assertions for: nav-pilot, code-review, accessibility. Add a run_pass_<agent> derived from that agent's own file before benchmarking it." ;;
esac

# --only is the same trap one level down. IDs are prefixed per agent, so
# `--agent code-review --only 2` matches no cr* ID: every `selected` call
# returns false, nothing runs, and the summary reads "All 0 assertions
# passed ✓" with exit 0. An ID the selected agent does not have is a typo
# or a stale command line, never a request to assert nothing.
if [[ -n "$ONLY" ]]; then
  IFS=',' read -r -a only_ids <<<"$ONLY"
  for only_id in "${only_ids[@]}"; do
    [[ " $VALID_IDS " == *" $only_id "* ]] || fail_preflight \
      "--only '$only_id' is not an assertion ID for agent '$AGENT'" \
      "Assertion IDs for $AGENT: $VALID_IDS"
  done
fi

# What the CLI is told to load. Read from the agent file's frontmatter, not
# guessed from the filename: accessibility.agent.md declares
# `name: accessibility-agent`, and --agent accessibility would silently load
# nothing. First `---` opens the block, the first `name:` inside it wins.
# The value is only trimmed and unquoted one layer, never squeezed: deleting
# every space would turn `name: a b` into the plausible-looking `ab` and walk
# it straight past the check below, launching an agent the file never declared.
AGENT_NAME="$(awk '/^---$/ {n++; next} n==1 && /^name:[[:space:]]*/ {sub(/^name:[[:space:]]*/, ""); print; exit}' "$PERSONA" | sed -e 's/[[:space:]]*$//' -e 's/^"\(.*\)"$/\1/' -e "s/^'\(.*\)'\$/\1/")"
[[ -n "$AGENT_NAME" ]] || fail_preflight \
  "$PERSONA has no 'name:' in its frontmatter" \
  "The CLI is launched with --agent <that name>; without it the harness cannot know which agent it would actually be measuring."

# The name is frontmatter, and frontmatter is untrusted input on a branch
# nobody has read yet. It is interpolated straight into a destination path
# ($TEMPLATE/.github/agents/$AGENT_NAME.agent.md), so a `name: ../../../etc/x`
# would have cp write outside the scratch workspace. Check it looks like a
# filename before using it as one: letters, digits, '-' and '_', nothing else.
# Every name in agents/ matches; they happen to be lowercase and hyphenated,
# but the check does not require that.
[[ "$AGENT_NAME" =~ ^[A-Za-z0-9_-]+$ ]] || fail_preflight \
  "$PERSONA declares an unusable agent name: '$AGENT_NAME'" \
  "It becomes a filename and a --agent argument, so it must be letters, digits, '-' or '_' only."

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
plugins {
    kotlin("jvm") version "2.1.0"
    kotlin("plugin.serialization") version "2.1.0"
}
repositories {
    mavenCentral()
}
dependencies {
    implementation("io.ktor:ktor-server-netty:3.0.0")
    implementation("io.ktor:ktor-server-content-negotiation:3.0.0")
    implementation("io.ktor:ktor-serialization-kotlinx-json:3.0.0")
}
EOF

# A minimal but real Ktor skeleton. It used to be three byte-identical files
# containing only `val maksAntall = 100`, seeded for test 6's rename. That made
# test 4's premises false: the prompt speaks of "den eksisterende Ktor-tjenesten"
# and a "kjent mønster", and neither existed in the fixture (see #519). The
# skeleton below makes both true, and keeps `maksAntall` in exactly three files
# — declared in Config.kt, used in App.kt and Routes.kt — so test 6's "rename
# variabelen maksAntall i tre filer" is still literally true, and is now a
# declaration plus two call sites rather than three copies of one line.
#
# ⚠️  Baselines in docs/golden-baselines/ recorded before 2026-08-31 measured
# the old placeholder fixture. NONE of their sizes are comparable with runs made
# after this change — every prompt explores this repo in Fase 1, so a bigger
# fixture moves t1 and t2 as surely as it moves t4 and t6, and --compare across
# that boundary would report a fixture change as a persona change. This comment
# is not the enforcement: FIXTURE_SUM below is, so that a --compare in six
# months says it without anyone having read this.
cat >"$TEMPLATE/src/main/kotlin/no/nav/demo/Config.kt" <<'EOF'
package no.nav.demo

// Maks antall oppgaver som returneres i én respons.
const val maksAntall = 100
EOF

cat >"$TEMPLATE/src/main/kotlin/no/nav/demo/Oppgave.kt" <<'EOF'
package no.nav.demo

import kotlinx.serialization.Serializable

@Serializable
data class Oppgave(val id: String, val tittel: String)

@Serializable
data class OppgaveRespons(val oppgaver: List<Oppgave>)
EOF

cat >"$TEMPLATE/src/main/kotlin/no/nav/demo/Routes.kt" <<'EOF'
package no.nav.demo

import io.ktor.server.application.Application
import io.ktor.server.response.respond
import io.ktor.server.routing.get
import io.ktor.server.routing.routing

private val oppgaver = listOf(
    Oppgave("1", "Registrer søknad"),
    Oppgave("2", "Send vedtaksbrev"),
)

fun Application.oppgaveRoutes() {
    routing {
        get("/api/oppgaver") {
            call.respond(OppgaveRespons(oppgaver.take(maksAntall)))
        }
    }
}
EOF

cat >"$TEMPLATE/src/main/kotlin/no/nav/demo/App.kt" <<'EOF'
package no.nav.demo

import io.ktor.serialization.kotlinx.json.json
import io.ktor.server.application.install
import io.ktor.server.engine.embeddedServer
import io.ktor.server.netty.Netty
import io.ktor.server.plugins.contentnegotiation.ContentNegotiation

fun main() {
    println("starter demo-tjeneste, maksAntall=$maksAntall")
    embeddedServer(Netty, port = 8080) {
        install(ContentNegotiation) { json() }
        oppgaveRoutes()
    }.start(wait = true)
}
EOF

# ─── Fixtures for the review agents ──────────────────────────────────────────
# Seeded ONLY when the agent under test needs them, and never for nav-pilot: two
# more files for Fase 1 to explore would move the sizes the baselines in
# docs/golden-baselines/ record, and the comparison would report the fixture
# change as a persona change. (The nav-pilot template itself was byte-for-byte
# unchanged from before --agent existed until #519 replaced the three
# placeholder .kt files with the Ktor skeleton above; see the note there.)
#
# The review agents cannot be given a clean repo. code-review asserts on what it
# reports, accessibility on which WCAG rules it names, and both need something
# to be wrong. Each planted defect below is one the agent file names explicitly,
# so a miss is a miss against its own spec and not against our taste:
#
#   UserRepo.kt   SQL string interpolation   code-review.agent.md:104-108
#                 fnr in a log line          code-review.agent.md:112-116, :121
#                 exception swallowed        code-review.agent.md:125-127
#   StatusPanel.tsx
#                 Tailwind p-4/mx-8          code-review.agent.md:178, :186-191
#                 <div onClick> no keyboard  accessibility.agent.md:230, :256
#                 outline: none              accessibility.agent.md:232, :257
#                 tabIndex={5}               accessibility.agent.md:236, :259
#                 colour as the only signal  accessibility.agent.md:233, :260
#                 icon button, no name       accessibility.agent.md:231, :258
if [[ "$AGENT" != "nav-pilot" ]]; then
  mkdir -p "$TEMPLATE/src/app/komponenter"

  cat >"$TEMPLATE/src/main/kotlin/no/nav/demo/UserRepo.kt" <<'EOF'
package no.nav.demo

import org.slf4j.LoggerFactory

private val logger = LoggerFactory.getLogger("UserRepo")

fun finnBruker(fnr: String): String? {
    logger.info("Slår opp bruker fnr=$fnr")
    val query = "SELECT navn FROM bruker WHERE fnr = '$fnr'"
    return try {
        session.run(queryOf(query).map { it.string("navn") }.asSingle)
    } catch (e: Exception) {
        null
    }
}
EOF

  cat >"$TEMPLATE/src/app/komponenter/StatusPanel.tsx" <<'EOF'
import { useState } from "react";
import { TrashIcon } from "@navikt/aksel-icons";

export function StatusPanel({ status, onSlett }) {
  const [apen, setApen] = useState(false);
  return (
    <div className="p-4 mx-8">
      <div onClick={() => setApen(!apen)} style={{ outline: "none" }}>
        Vis detaljer
      </div>
      <span tabIndex={5} style={{ color: status === "feil" ? "red" : "green" }}>
        {status}
      </span>
      <button onClick={onSlett}>
        <TrashIcon />
      </button>
    </div>
  );
}
EOF
fi

# Fixture identity, for the --compare compatibility check below. The sizes this
# harness reports are sizes of answers about this fake repo, so they move when
# the repo moves. .github/ is excluded on purpose: the persona and the
# instructions are the variables the harness exists to move, and folding them in
# here would warn "not comparable" on exactly the comparison it is built to make.
FIXTURE_SUM="$( (cd "$TEMPLATE" && find . -type f -not -path './.github/*' -exec cksum {} + 2>/dev/null) | sort | cksum | awk '{print $1}' )"

# The agent writes to $WS (t1 fixes the README typo, t4 extends a handler, t6
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

# Did the agent write to the repo? Two assertions turn on this (code-review is
# forbidden to auto-fix; accessibility must ask before a custom ARIA role), and
# neither can be read off the transcript: an agent that edits and then says
# "here is what I would change" is indistinguishable in text from one that only
# advised. So the workspace is fingerprinted either side of the call.
#
# Content-addressed, not mtime-based: a created, modified or deleted file all
# show up. `-exec cksum {} +` keeps the path next to the checksum, so the diff
# names the files; the sort makes directory order irrelevant.
ws_fingerprint() { ( cd "$WS" && find . -type f -exec cksum {} + 2>/dev/null | sort ); }

# NOTE ON ORDERING: these two files describe the MOST RECENT run_prompt, and are
# overwritten by the next one. An assertion that uses them must be evaluated
# before the next run_prompt in the same pass. The groups below all do.
FP_BEFORE="$WORKDIR/fp.before"
FP_AFTER="$WORKDIR/fp.after"
: >"$FP_BEFORE"
: >"$FP_AFTER"

ws_wrote() { ! cmp -s "$FP_BEFORE" "$FP_AFTER"; }
ws_written_files() {
  diff "$FP_BEFORE" "$FP_AFTER" | awk '/^[<>]/ { print $NF }' | sort -u | tr '\n' ' '
}

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
soft_count=0
RUN=1

# Transcript path for a prompt in the current run. Repeats must not overwrite
# each other: --keep has to leave all N samples behind for inspection.
tx() { printf '%s/%s.run%s.txt' "$WORKDIR" "$1" "$RUN"; }

run_tag() { [[ "$REPEAT" -gt 1 ]] && printf '%s[run %s/%s]%s ' "$DIM" "$RUN" "$REPEAT" "$RESET"; return 0; }

# A transcript shorter than this is treated as "the call did not happen", not as
# a response. Every assertion below is either an absent() — which succeeds
# trivially on an empty file — or a present() on a long structured block, so
# without this floor a crashed or unauthenticated CLI reports green.
#
# It only has to catch an empty or crashed CLI. It must NOT try to judge whether
# an answer is substantial: the persona answers trivial tier in two sentences by
# design, and at 200 this floor sat above that length. It discarded a *passing*
# test 5 canary — a correct 160B TokenX answer, exit 0, 5.84 credits — as an
# error, and t6 came within seven bytes of the same trap (#519). The per-test
# gates below do the real work; this one only asks whether there is a response
# at all.
MIN_TRANSCRIPT_BYTES=40
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
  ws_fingerprint >"$FP_BEFORE"

  echo "${DIM}  → $(run_tag)prompting ($slug)…${RESET}" >&2
  local -a runner=()
  [[ -n "$TIMEOUT_BIN" ]] && runner=("$TIMEOUT_BIN" "$TIMEOUT_SECS")
  # ${arr[@]+"${arr[@]}"} — bash 3.2 (stock macOS) treats an empty array as an
  # unbound variable under `set -u`, and the no-coreutils fallback above leaves
  # `runner` empty on exactly that platform.
  ( cd "$WS" && ${runner[@]+"${runner[@]}"} "$CLI_PATH" "${args[@]}" ) >"$out" 2>"${out%.txt}.err"
  local rc=$?
  # Taken unconditionally, including after a dead call: an agent that wrote and
  # then timed out still wrote, and the no-auto-fix assertions want to say so.
  ws_fingerprint >"$FP_AFTER"
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

record_soft() {
  # record_soft <id> <description> <ok:0|1> [detail]
  # A want, not an invariant. Prints its result every run and is counted in the
  # summary, but never contributes to the exit code. Use it for something we
  # would like the persona to emit and have evidence it does not, where a hard
  # assertion would leave the suite permanently red and train people past it.
  #
  # Records ONE run, like record: the soft verdict for the test as a whole is
  # decided in the aggregation below, once every repeat has been recorded, and
  # soft_count is incremented there rather than here.
  local id="$1" desc="$2" ok="$3" detail="${4:-}"
  if [[ "$ok" == "0" ]]; then
    echo "  ${GREEN}✓${RESET} ${BOLD}$id${RESET} $(run_tag)$desc ${DIM}(soft)${RESET}"
    printf '%s|soft-pass|%s|\n' "$id" "$desc" >>"$RESULTS_FILE"
  else
    echo "  ${YELLOW}○${RESET} ${BOLD}$id${RESET} $(run_tag)$desc ${YELLOW}(soft, not met)${RESET}"
    [[ -n "$detail" ]] && echo "      ${DIM}$detail${RESET}"
    printf '%s|soft-fail|%s|%s\n' "$id" "$desc" "$detail" >>"$RESULTS_FILE"
  fi
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

echo "${BOLD}golden-prompt harness, agent under test: $AGENT${RESET}"
echo "${DIM}client: $CLI_NAME${CLI_PATH:+ ($CLI_PATH)}${RESET}"
echo "${DIM}agent file: $PERSONA (launched as --agent $AGENT_NAME)${RESET}"
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

# ── Test 2 vocabulary, derived from transcripts ─────────────────────────────
# Every regex below was measured against the 36 kept transcripts of the three
# persona revisions on this branch (18 t2, 18 t4, four models). The hit rates in
# the comments are those measurements, not estimates.
#
# The response reached Fase 1 at all. Gate, not assertion: no Fase 1 output means
# the stop invariant was never exercised, which is "not evaluated", never a pass.
# Hit rate: t2 18/18, t4 1/18.
RE_FASE1_REACHED='Fase[[:space:]]*1|Intervju'

# Fase 2 or later *work*, the leak the stop invariant forbids. Two markers, both
# with clean separation: t2 0/18, t4 18/18.
#
# Test 2's alone. Test 4 briefly shared it, and that was a mistake worth naming:
# the two tests ask different questions. Test 2 asks "did anything past Fase 1
# happen", for which a file mutation is perfect evidence. Test 4 asks "was a
# Fase 2 plan produced", for which a file mutation is no evidence at all — a
# full-tier Fase 1 turn that leaks one `● Update(...)` line has mutated without
# planning, and every vacuous pass found in review came through that door. Test
# 4 keys on RE_FASE2_PLAN below instead.
#
#   ^● Edit|Create|…  the client renders one line per file mutation. A full-tier
#                     Fase 1 turn that writes files has run past its own gate.
#   Grønn sone        the zone declaration belongs to a Fase 2 plan.
#
# Deliberately NOT in this regex, each for a measured reason:
#
#   accessPolicy   2 of 18 correct Fase 1 responses name it, reporting that the
#                  seeded nais.yaml lacks one. That is Fase 1 reading the repo,
#                  not Fase 2 writing a manifest, and keying on it false-failed
#                  the stop invariant on exactly the runs that honoured it.
#   Fase 2         2 of 18 refer forward to it ("jeg bygger plan i Fase 2").
#                  Naming the next phase is what stopping before it looks like.
#   Rød sone       the Fase 1 checkpoint template lists it as a summary line, so
#                  it would fire on the block test 2b is still hoping to see.
RE_FASE2_WORK='^● (Edit|Create|Write|Delete|Update)|Grønn sone'

# The turn ends with questions outstanding rather than proceeding. Counted, not
# matched, because the closing invitation is pure paraphrase: seventeen of
# eighteen say some form of "svar på det du vet", the eighteenth lays the blind
# spots out as a table and says nothing at all. What every run does have is
# questions. Occurrence counts: t2 8 to 16, t4 0 to 2. The floor sits in that gap.
MIN_OPEN_QUESTIONS=3

# The blind-spot audit line, in any phrasing (reist / adressert / dekket), as long
# as it carries the count. Required by `### Fase 1: Intervju`: "Track which blind
# spots you raise and report the count". Soft: t2 0/18.
RE_BLINDSPOT_AUDIT='Blindsoner[^.]{0,40}[0-9]+[[:space:]]*/[[:space:]]*11'

# ── Test 4 vocabulary ───────────────────────────────────────────────────────
# Test 4's presence gate is RE_FASE2_WORK, the same expression test 2 uses as
# its leak detector. One expression, read in two directions: what test 2 must
# not see is exactly what test 4 must see, and it is measured in both (t2 0/18,
# t4 18/18).
#
# It used to have its own regex, `Grønn sone|accessPolicy`, and the accessPolicy
# half was the bug (#519). Two of eighteen *Fase 1* responses name accessPolicy,
# reporting that the seeded nais.yaml lacks one. On those runs the gate opened
# on a Fase 1 answer, and `present "$T4" 'Rød sone'` then matched the
# `• 🔴 Rød sone: [liste, eller «ingen»]` line of the Fase 1 checkpoint template
# — the vacuous pass the block comment at test 4 says must never happen.
#
# This is not the false failure #491 removed. That one was accessPolicy used as
# test 2's *leak* detector, firing on correct Fase 1 answers. Dropping it here
# removes the same word from the other direction.
# A Fase 2 plan was produced. Test 4's gate, and deliberately independent of the
# zone declarations, because zone presence is the thing under test: using
# `Grønn sone` as the plan detector conflated "a plan exists" with "a green zone
# was declared", so a plan that dropped BOTH zones — the wholesale-trimming
# regression this assertion exists to catch — was filed as "no plan" instead of
# as the failure it is. `Grønn sone` stays as one alternative among several, but
# it is no longer what decides whether a plan exists.
#
# The markers are the persona's own headings, all of which put a colon straight
# after the number or the word «ferdig» after it:
#
#   📐 Fase 2: Plan         `### Delegation format` (agent file :139)
#   ✅ Fase 2 ferdig        `### Phase transition format` (:120), filled for 2
#   🟢 Grønn sone           the zone block of `### Fase 2: Plan` item 10 (:211)
#
# Deliberately NOT a bare `Fase 2`: two of eighteen Fase 1 answers refer forward
# to it in prose («jeg bygger plan i Fase 2», «kommer i Fase 2-planen»), and
# naming the next phase is what stopping before it looks like. Requiring the
# colon or «ferdig» excludes both of those phrasings.
#
# ⚠️  PROVISIONAL, and known to be insufficient on its own. It is derived from
# the persona file rather than from measured output, and every expression written
# that way for this test has turned out to admit some Fase 1 shape: a response can
# quote the Fase 2 zone template forward, pre-declare «🔴 Rød sone: ingen for
# denne oppgaven», or simply name «Fase 2: Plan» in a sentence. That is why test
# 4 cannot report green at all — see the calibration note at the test itself.
# Do not widen this to a bare `Fase 2`, and do not narrow it hoping for a pass.
RE_FASE2_PLAN='Fase[[:space:]]*2[[:space:]]*:|Fase[[:space:]]*2[[:space:]]+ferdig|Grønn sone'
RE_OPUS='nav-pilot-opus'

# One pass over the selected prompts. Called once per --repeat, so every
# assertion below sees a fresh sample; `tx` keeps the transcripts apart.
run_pass_nav_pilot() {
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

  # ── Tests 2 + 2b + 3: full tier stops after Fase 1, raises blind spots 1 and 2 ──
  # One model call, three independent checks (the prompt is identical, so
  # re-running it would only pay three times for the same sample).
  #
  # ⚠️  Test 2 used to assert two things at once: that the response stops after
  # Fase 1, and that it stops by emitting the literal checkpoint template. Those
  # two have opposite track records. The stop held on 18 of 18 transcripts across
  # three persona revisions and four models. The template appeared in none of them,
  # in any form, and three attempts to make the persona emit it all failed live:
  # the ONLY clause was removed, the Fase 1 exit criterion was fixed so it stopped
  # contradicting the block, the checkpoint was added to the phase machine's
  # allowed work, to `✅ Always`, and as a filled-in example at the point of use.
  # Zero for eleven runs. The model emits every field the block would carry, as
  # prose and front-loaded, and omits the container.
  #
  # So they are split. Test 2 keeps its number and asserts the behaviour, on
  # regexes derived from the transcripts. Test 2b carries the format as a soft
  # check that reports and does not fail the suite, because a permanently red test
  # is a test people stop reading. Test 2b is not decoration: `Blindsoner reist:
  # N/11` with justification for the skipped ones is required by `### Fase 1:
  # Intervju`, it is the one field the prose ending does not replace in any run,
  # and it is a real audit loss for as long as it is missing.
  if selected 2 || selected 2b || selected 3; then
    DESC2="full tier: response stops after Fase 1 with questions outstanding"
    DESC2B="full tier: Fase 1 checkpoint block emitted, with the blind-spot count"
    DESC3="full tier: blind spots #1 (personvern) and #2 (tilgangskontroll) both raised"
    T2="$(tx t2)"
    if ! run_prompt t2 "ny tjeneste som leser fnr fra ID-porten"; then
      if selected 2; then record_error 2 "$DESC2" "$LAST_PROMPT_DETAIL"; fi
      if selected 3; then record_error 3 "$DESC3" "$LAST_PROMPT_DETAIL"; fi
    else
      if selected 2; then
        # Invariant: PHASE INTEGRITY, "STOP after each phase […] wait for explicit
        # user confirmation", and Boundaries → 🚫 Never, "Do work belonging to a
        # later phase in the same response when on full-tier".
        # Order matters. The leak is checked before the Fase 1 gate, because the
        # worst regression, a full-tier response that skips the interview and goes
        # straight to writing files, has no Fase 1 output to gate on. Gating first
        # reports that as amber "not evaluated" instead of red. Only a response that
        # did neither Fase 1 nor Fase 2 work is genuinely unevaluable.
        q="$(count_of "$T2" '[?]')"
        ok=0; detail=""
        if ! absent "$T2" "$RE_FASE2_WORK"; then
          record 2 "$DESC2" 1 \
            "response did Fase 2 work (matched: $RE_FASE2_WORK): PHASE INTEGRITY rule regressed"
        elif ! present "$T2" "$RE_FASE1_REACHED"; then
          record_error 2 "$DESC2" \
            "no Fase 1 output and no Fase 2 work (no match for: $RE_FASE1_REACHED): the stop invariant was never exercised, so it was not evaluated. Re-run with --keep and check whether tier classification regressed."
        else
          if [[ "$q" -lt "$MIN_OPEN_QUESTIONS" ]]; then
            ok=1; detail="only $q question mark(s), need ≥$MIN_OPEN_QUESTIONS: the turn did not end with questions outstanding, so it did not stop for the user"
          fi
          record 2 "$DESC2" "$ok" "$detail"
        fi
      fi

      if selected 2 || selected 2b; then
        # SOFT. See the block comment above tests 2 + 2b. `--only 2` keeps
        # reporting both parts of the split, and `--only 2b` asks for this part
        # alone; an ID that preflight accepts has to reach the code that runs it. Each part is reported
        # separately: if the audit count starts showing up as prose while the block
        # still does not, that is progress and the next attempt should see it.
        missing=""
        absent "$T2" "$RE_CHECKPOINT"      && missing="$missing; no checkpoint header (want: $RE_CHECKPOINT)"
        absent "$T2" "$RE_CONFIRM"         && missing="$missing; no confirmation line (want: $RE_CONFIRM)"
        absent "$T2" "$RE_BLINDSPOT_AUDIT" && missing="$missing; no blind-spot audit count (want: $RE_BLINDSPOT_AUDIT)"
        if [[ -z "$missing" ]]; then
          record_soft 2b "$DESC2B" 0
        else
          record_soft 2b "$DESC2B" 1 "${missing#; }"
        fi
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
  # ⚠️  THE PROMPT MUST NOT ASK FOR A NEW ENDPOINT, NEW DATA OR NEW AUTH. It used
  # to ask for "et nytt REST-endepunkt", and that prompt cannot reach Fase 2 by
  # construction (#519): `### Fase 1: Intervju` marks blind spots #1 and #2
  # "required regardless of scope tier if the change touches user data, new API
  # endpoints, or any auth configuration", `### ✅ Always` repeats it, and
  # `## Request scope classification` sends new API contracts to Full. All three
  # samples of that prompt correctly stopped in Fase 1 with questions open.
  #
  # The prompt below extends an existing handler and an existing response model.
  # Its tail restates the compressed-tier criteria from `## Request scope
  # classification` — not verbatim: "no new service boundary, no new data flows"
  # is given as "ingen nye endepunkter, ingen persondata", because those are the
  # two blocking triggers at `### Fase 1: Intervju` that have to be defused for
  # any tier to reach Fase 2 at all. The fixture backs the premise: the Ktor
  # skeleton really does have /api/oppgaver, in Routes.kt, with its response
  # model in Oppgave.kt.
  #
  # ⚠️  The prompt does tell the model most of its own classification criteria,
  # which weakens test 4 as evidence that the persona classifies correctly. It
  # is not evidence of that and never was — the assertion is about the red-zone
  # rule, and the tier is a precondition for reaching it in one non-interactive
  # call. Measuring classification wants its own assertion on a prompt that
  # states only the shape of the change. Not done here; see #519.
  #
  # ⚠️  TEST 4 CANNOT REPORT GREEN. This is deliberate, and it is not a bug.
  #
  # The assertion needs to tell a Fase 2 plan apart from a Fase 1 stop. Four
  # attempts derived that distinction from the persona file, and review broke
  # every one of them, each time with a *different* Fase 1 shape:
  #
  #   present 'Rød sone'          the Fase 1 checkpoint template carries the words
  #   + accessPolicy gate         a Fase 1 answer naming the seeded nais.yaml
  #   + `● Edit` mutation gate    a Fase 1 stop that leaks one mutation line
  #   + `[liste` placeholder      a compliant model FILLS the template in
  #   + `•` bullet exclusion      the persona has TWO zone-bearing templates, and
  #                               a Fase 1 stop may quote the Fase 2 one forward
  #
  # The pattern is not that the regexes were careless. It is that we have no
  # ground truth: there is no measured Fase 2 transcript anywhere in this repo to
  # derive from. All three kept t4 samples stop in Fase 1, and the 18/18 behind
  # RE_FASE2_WORK was measured against the *old* prompt, which this PR replaced.
  # #491 got its expressions right because it derived them from 36 measured
  # transcripts. Guessing from the spec has now failed four times running.
  #
  # So the pass branch is disabled until it can be calibrated. What survives is
  # the half that does not need ground truth: a response that reaches a plan and
  # contains no red-zone wording at all has demonstrably lost the declaration,
  # and that is still RED. Everything else is "not evaluated".
  #
  # An amber is honest when we cannot tell a pass from a vacuous one. A green is
  # not: it launders a regression, and unlike an amber nobody ever reads it.
  #
  # TO CALIBRATE (and re-enable the pass branch):
  #   1. ./scripts/nav-pilot-golden.sh --only 4 --repeat 5 --keep --model <pinned>
  #   2. Read the kept t4 transcripts BY HAND. Do not skim for a keyword.
  #   3. Derive two things from what the model actually emits: what marks a plan,
  #      and what marks the red-zone declaration inside one. Record the hit rates
  #      the way the test 2 vocabulary above does (`t2 0/18, t4 18/18`).
  #   4. Only then restore `record 4 "$DESC4" 0` behind those expressions.
  # Until step 3 has real numbers next to it, leave this branch alone.
  if selected 4; then
    DESC4="Fase 2 output contains a 🔴 Rød sone declaration  ${YELLOW}(uncalibrated)${RESET}"
    T4="$(tx t4)"
    if ! run_prompt t4 "legg til feltet antall i responsen fra det eksisterende /api/oppgaver-endepunktet i Ktor-tjenesten — flere filer, kjent mønster, ingen nye endepunkter, ingen persondata, ingen endringer i auth"; then
      record_error 4 "$DESC4" "$LAST_PROMPT_DETAIL"
    elif ! present "$T4" "$RE_FASE2_PLAN"; then
      record_error 4 "$DESC4" \
        "no Fase 2 plan in the response (no match for: $RE_FASE2_PLAN) — a red-zone declaration is a property of a plan, so with no plan there is nothing to assert and this is not a pass. Either the response stopped in Fase 1, or it took a trivial-tier single pass. Re-run with --keep and read the transcript."
    elif ! present "$T4" 'Rød sone'; then
      record 4 "$DESC4" 1 \
        "a Fase 2 plan with no 🔴 Rød-sone-deklarasjon anywhere in the response — mandatory per \`### Fase 2: Plan\` item 10 and Boundaries → ✅ Always"
    else
      record_error 4 "$DESC4" \
        "UNCALIBRATED — the response reached a plan and mentions a red zone, but this harness cannot yet tell that apart from a Fase 1 stop that quotes the Fase 2 zone template forward, so it will not call it a pass. No measured Fase 2 transcript exists to derive the distinction from. Calibrate with: --only 4 --repeat 5 --keep --model <pinned>, read the transcripts by hand, then restore the pass branch behind expressions with real hit rates. See the note above this test."
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

# ─── code-review ─────────────────────────────────────────────────────────────
# Derived from agents/code-review.agent.md, which is unusually explicit about
# its own contract: a findings table with a fixed schema, a three-level priority
# system, a "report, never fix" boundary and a delegation table. Each assertion
# cites the line it comes from, the same way tests 1-6 cite the persona.
#
# ⚠️  cr2 is not a formality. code-review has `execute` (code-review.agent.md:6)
# and the harness passes --allow-all-tools, so having no `edit` tool does not
# stop `sed -i`. On a cheaper model this is the boundary most likely to go.

# Priority markers, from `## Priority System` (code-review.agent.md:62-64). The
# English words are accepted next to the emoji because the three-level scheme is
# the claim, not the glyph. "Nit" is deliberately not in the list: three letters
# with no boundary matches inside ordinary Norwegian words.
RE_CR_PRIORITY='🔴|🟡|💭|Blocker|Suggestion'
# The planted SQL injection in UserRepo.kt, which mirrors the ❌ example at
# code-review.agent.md:104-108 almost line for line. Used as the gate for the
# whole group: if the review never names it, the transcript says nothing about
# output format or about the no-auto-fix boundary, and reporting either as a
# pass would be inventing a result.
RE_CR_INJECTION='injeksjon|injection|parameteri[sz]|prepared[[:space:]]+statement|string[- ]?interpolasjon|interpolat'
# Causal language. `## Priority System`, code-review.agent.md:66 says "For each
# finding, explain **why** it matters, teach, don't just flag", repeated under
# ✅ Always at :246.
RE_CR_WHY='fordi|because|risik|kan[[:space:]]+føre[[:space:]]+til|fører[[:space:]]+til|angriper|attacker|utnytt|exploit|lekk|konsekvens|derfor|slik[[:space:]]+at'
# The two specialists that own a Next.js/Aksel file, from `## Related agents and
# skills` (code-review.agent.md:40 and :42). Deliberately not the whole table:
# routing a spacing-and-keyboard review to @security-champion-agent would name
# the wrong owner. Read by cr4, which is soft; see the block above it.
RE_CR_DELEGATE='accessibility[-[:space:]]?agent|aksel[-[:space:]]?agent'

run_pass_code_review() {
  # ── cr1 + cr2 + cr3: one review of a Kotlin file with three planted defects ──
  if selected cr1 || selected cr2 || selected cr3; then
    DESC_CR1="findings arrive as table rows carrying a priority and a line"
    DESC_CR2="reports only: the reviewed file is not auto-fixed"
    DESC_CR3="explains why each finding matters, not a bare table"
    TCR="$(tx cr-kotlin)"
    if ! run_prompt cr-kotlin "gjennomgå src/main/kotlin/no/nav/demo/UserRepo.kt"; then
      selected cr1 && record_error cr1 "$DESC_CR1" "$LAST_PROMPT_DETAIL"
      selected cr2 && record_error cr2 "$DESC_CR2" "$LAST_PROMPT_DETAIL"
      selected cr3 && record_error cr3 "$DESC_CR3" "$LAST_PROMPT_DETAIL"
    elif ! present "$TCR" "$RE_CR_INJECTION"; then
      # The gate. UserRepo.kt builds its query by interpolating fnr into a
      # string; a review that does not name that has not reviewed the file, and
      # the three assertions below would each be measuring an empty response.
      cr_gate="the review never named the SQL injection in UserRepo.kt (no match for: $RE_CR_INJECTION), so this transcript says nothing about output format, the no-auto-fix boundary or teaching. Re-run with --keep and read it before touching an assertion."
      selected cr1 && record_error cr1 "$DESC_CR1" "$cr_gate"
      selected cr2 && record_error cr2 "$DESC_CR2" "$cr_gate"
      selected cr3 && record_error cr3 "$DESC_CR3" "$cr_gate"
    else
      # cr1: `## Output Format` (code-review.agent.md:70-86) is a summary, then
      # findings in a table whose columns are File, Line, Priority, Issue.
      # Asserted as "a pipe row carrying a priority marker" and "such a row also
      # carries a number", not as column order: the schema is the promise, the
      # exact layout is allowed to drift.
      if selected cr1; then
        cr_rows="$(grep -E '^[[:space:]]*\|' "$TCR" | grep -cE "$RE_CR_PRIORITY")"
        cr_located="$(grep -E '^[[:space:]]*\|' "$TCR" | grep -E "$RE_CR_PRIORITY" | grep -cE '[0-9]')"
        ok=0; detail=""
        if [[ "$cr_rows" -lt 1 ]]; then
          ok=1; detail="found the injection but reported no table row carrying a priority marker ($RE_CR_PRIORITY). The Output Format and Priority System regressed"
        elif [[ "$cr_located" -lt 1 ]]; then
          ok=1; detail="$cr_rows prioritised row(s), none carrying a line number. Findings are not locatable, per the File/Line columns at code-review.agent.md:78"
        fi
        record cr1 "$DESC_CR1" "$ok" "$detail"
      fi

      # cr2: `## Boundaries → 🚫 Never` (code-review.agent.md:259) says "Auto-fix
      # code, report findings only", stated again in the opening line at :21.
      # Read off the workspace, not the transcript: an agent that edits and then
      # says "here is what I would change" reads identically in text.
      if selected cr2; then
        if ws_wrote; then
          record cr2 "$DESC_CR2" 1 \
            "the agent wrote to the workspace: $(ws_written_files). 🚫 Never auto-fix (code-review.agent.md:259) regressed. It has execute (:6), so no edit tool is not a guardrail."
        else
          record cr2 "$DESC_CR2" 0
        fi
      fi

      # cr3: code-review.agent.md:66 says "explain **why** it matters, teach, don't
      # just flag", and ✅ Always :246. Two conditions, because either alone is
      # cheap to satisfy: causal language must appear, AND there must be prose
      # outside the table for it to appear in. A table with "fordi" wedged into
      # an Issue cell is a flag, not teaching.
      if selected cr3; then
        cr_prose="$(grep -vE '^[[:space:]]*\|' "$TCR" | grep -cE '^.{40,}$')"
        ok=0; detail=""
        if [[ "$cr_prose" -lt 2 ]]; then
          ok=1; detail="only $cr_prose prose line(s) of 40+ chars outside the table. The ### Details section (code-review.agent.md:84-85) is where why lives, and it is missing"
        elif ! present "$TCR" "$RE_CR_WHY"; then
          ok=1; detail="no causal language anywhere in the response (no match for: $RE_CR_WHY). Findings were flagged, not explained"
        fi
        record cr3 "$DESC_CR3" "$ok" "$detail"
      fi
    fi
  fi

  # ── cr4: a Next.js file is a specialist's file ────────────────────────────────
  # SOFT. `## Related agents and skills` (code-review.agent.md:37-43) names the
  # owner of an Aksel/a11y file, and StatusPanel.tsx is squarely two of its rows:
  # Tailwind spacing where Aksel tokens belong (:178, :186-191) and keyboard/ARIA
  # defects. Naming that owner is worth measuring. It cannot be a hard assertion:
  #
  #   1. code-review has no `runSubagent` in its frontmatter (:5-16; compare
  #      accessibility.agent.md:12, which has it). The agent cannot hand a file to
  #      another agent. The only thing it can do is type the handle, so this check
  #      pins wording while describing behaviour, which is what BEHAVIOUR VS FORMAT
  #      above forbids. The ✅ Always bullet that promised delegation was removed
  #      rather than reworded: #484 is three failed rewordings of a sibling rule.
  #   2. One agent is installed per invocation (:402). The delegation target is not
  #      in the workspace where the delegation is measured.
  #
  # Measured 0/5 on GPT-5.3-Codex and 0/5 on GPT-5.6 Luna (#514). All ten runs did
  # reach the accessibility domain anyway: instructions/accessibility.instructions.md
  # is scoped `applyTo: "src/**/*.{tsx,jsx}"`, which covers StatusPanel.tsx, so the
  # user is not underserved by the miss. Hence reported, not failed.
  #
  # Every path below records a soft row, the two that cannot reach a verdict
  # included. `record_error` would aggregate to error and flip an otherwise green
  # run to exit 3, and for the off-domain path that is backwards: a review that
  # ignores both domains is worse behaviour than one that reaches them and names
  # no owner, so letting the worse case harden the suite while the closer miss
  # stays soft inverts the point of demoting this at all. The header above, :186,
  # the `record_soft` doc and the printed summary all say a soft check never moves
  # the exit code in either direction, and cr4 keeps that literally rather than
  # carving out an exception. The two unevaluable paths say "not evaluated:" in
  # their detail so the distinction stays readable, and cr1-cr3 still carry CLI
  # health on their own prompt, which is where a flaky CLI shows up first.
  if selected cr4; then
    DESC_CR4="Next.js review routes on to the accessibility/Aksel specialist"
    TCR4="$(tx cr-tsx)"
    if ! run_prompt cr-tsx "gjennomgå src/app/komponenter/StatusPanel.tsx"; then
      record_soft cr4 "$DESC_CR4" 1 "not evaluated: $LAST_PROMPT_DETAIL"
    elif ! present "$TCR4" 'space-[0-9]|<Box|Aksel|Tailwind|p-4|mx-8|paddingInline|paddingBlock|tastatur|keyboard|aria'; then
      record_soft cr4 "$DESC_CR4" 1 \
        "not evaluated: the response reached neither the spacing nor the a11y domain of StatusPanel.tsx, so there was no domain review to route and nothing here says whether an owner would have been named. Not counted as met: a bare absent() on the handles would pass vacuously."
    elif present "$TCR4" "$RE_CR_DELEGATE"; then
      record_soft cr4 "$DESC_CR4" 0
    else
      record_soft cr4 "$DESC_CR4" 1 \
        "reviewed an Aksel/a11y file without naming @accessibility-agent or @aksel-agent, the owners at code-review.agent.md:40 and :42. Soft: the agent has no runSubagent and the target is not installed, so this is a want, not a regression"
    fi
  fi
}

# ─── accessibility ───────────────────────────────────────────────────────────
# Derived from agents/accessibility.agent.md. Its substance is WCAG: the tables
# at :32-74 map every requirement to a numbered success criterion, and an answer
# that says "add a label" without 3.3.2 has lost exactly what the agent file
# carries. So uu2 counts criteria, not adjectives.
#
# ⚠️  This agent has `edit` (:8) and `runSubagent` (:12) on top of `execute`.
# uu3 and uu4 are about that grant, and are worth more than any assertion about
# how it phrases advice.

# The planted defects in StatusPanel.tsx, one regex per topic. Each is a row of
# `## Vanlige Feil` (accessibility.agent.md:228-236) and a bullet of
# 🚫 Never (:254-260), so a miss is a miss against the agent's own list.
RE_UU_KEYBOARD='tastatur|keyboard|onKeyDown|onKeyPress|role="button"|klikkbar[[:space:]]+div|div[[:space:]]+med[[:space:]]+onClick|<div onClick'   # :230, :256
RE_UU_FOCUS='outline|fokusindikator|fokus-indikator|synlig[[:space:]]+fokus|focus[- ]?visible|2\.4\.7'                                              # :232, :257
RE_UU_TABINDEX='tabindex[^0-9a-zæøå]{0,8}(5|>[[:space:]]*0|større[[:space:]]+enn[[:space:]]+0)|positiv[[:space:]]+tabindex|tabindex[[:space:]]*>[[:space:]]*0'  # :236, :259
RE_UU_COLOUR='farge[^.!?]{0,60}(eneste|alene)|kun[[:space:]]+farge|colou?r[^.!?]{0,40}only|1\.4\.1'                                                 # :233, :260
# Confirmation or redirect. ⚠️ Ask First (:248-250) covers custom ARIA roles and
# deviations from the Aksel pattern; ✅ Always :242 says use Aksel components.
# Either asking or steering back to Aksel is the documented handling.
RE_UU_ASK='vil[[:space:]]+du|skal[[:space:]]+jeg|ønsker[[:space:]]+du|bekreft|foreslår|anbefaler|i[[:space:]]+stedet|istedenfor|<Select|Aksel'
# Subagent fan-out, as the CLI reports it.
# ⚠️  UNVERIFIED SURFACE: this only catches a spawn the client actually prints.
# The positive gate in uu4 is what keeps the assertion from being vacuous: it
# cannot pass off an empty transcript. But if the client turns out to spawn
# silently, uu4 measures nothing and must be replaced, not relaxed.
RE_UU_SUBAGENT='runSubagent|run_subagent|sub-?agent|spawn(ing|ed)?[[:space:]]+(an[[:space:]]+)?agent|delegerer[[:space:]]+til[[:space:]]+@'

run_pass_accessibility() {
  # ── uu1 + uu2: one review of a component with five planted defects ───────────
  if selected uu1 || selected uu2; then
    DESC_UU1="names at least 3 of the 4 planted Vanlige Feil"
    DESC_UU2="cites WCAG success criteria by number, not just by adjective"
    TUU="$(tx uu-review)"
    if ! run_prompt uu-review "gå gjennom src/app/komponenter/StatusPanel.tsx for universell utforming"; then
      selected uu1 && record_error uu1 "$DESC_UU1" "$LAST_PROMPT_DETAIL"
      selected uu2 && record_error uu2 "$DESC_UU2" "$LAST_PROMPT_DETAIL"
    else
      # Count topics rather than assert each one: the fixture plants four and a
      # good review may fold two into one finding, but a review that finds one
      # or none has not reviewed the file. The misses are named in the detail,
      # so a failure says which rule went missing.
      uu_hits=0; uu_missed=""
      present "$TUU" "$RE_UU_KEYBOARD" && uu_hits=$((uu_hits + 1)) || uu_missed="$uu_missed <div onClick> keyboard (:230,:256);"
      present "$TUU" "$RE_UU_FOCUS"    && uu_hits=$((uu_hits + 1)) || uu_missed="$uu_missed outline:none (:232,:257);"
      present "$TUU" "$RE_UU_TABINDEX" && uu_hits=$((uu_hits + 1)) || uu_missed="$uu_missed tabIndex={5} (:236,:259);"
      present "$TUU" "$RE_UU_COLOUR"   && uu_hits=$((uu_hits + 1)) || uu_missed="$uu_missed colour as only signal (:233,:260);"

      if selected uu1; then
        if [[ "$uu_hits" -ge 3 ]]; then
          record uu1 "$DESC_UU1" 0
        else
          record uu1 "$DESC_UU1" 1 \
            "found $uu_hits of 4 planted defects. Missed:$uu_missed each is a row of ## Vanlige Feil and a 🚫 Never bullet in accessibility.agent.md"
        fi
      fi

      # uu2: the WCAG tables (accessibility.agent.md:32-74) are the substance of
      # this agent, and :24 calls AA "lovpålagt i Norge". Distinct criteria, so
      # one number repeated ten times does not count as two.
      #
      # Gated on the review having found anything at all: on a transcript with
      # no findings, "no criteria cited" is not a WCAG regression, it is a dead
      # sample, and calling it a failure would be as wrong as calling it a pass.
      if selected uu2; then
        if [[ "$uu_hits" -lt 1 ]]; then
          record_error uu2 "$DESC_UU2" \
            "the review named none of the 4 planted defects, so there was no finding for a criterion to be attached to, so this is not evaluated rather than failed."
        else
          uu_wcag="$(grep -oE '[1-4]\.[0-9]{1,2}\.[0-9]{1,2}' "$TUU" | sort -u | wc -l | tr -d ' ')"
          if [[ "$uu_wcag" -ge 2 ]]; then
            record uu2 "$DESC_UU2" 0
          else
            record uu2 "$DESC_UU2" 1 \
              "$uu_hits defect(s) found but only $uu_wcag distinct WCAG criterion number(s) cited. The tables at accessibility.agent.md:32-74 are what this agent knows that a generic reviewer does not"
          fi
        fi
      fi
    fi
  fi

  # ── uu3: a custom ARIA role is an Ask-First item ─────────────────────────────
  # ⚠️ Ask First (accessibility.agent.md:248-250): "Custom ARIA-roller",
  # "Avvik fra Aksel-mønster". The prompt asks for both at once. With `edit`
  # granted (:8), the failure mode is that it just writes the file.
  if selected uu3; then
    DESC_UU3="custom ARIA role: asks or redirects to Aksel, does not just write it"
    TUU3="$(tx uu-ask)"
    if ! run_prompt uu-ask "legg til en custom listbox med role=\"listbox\" for statusvalget i src/app/komponenter/StatusPanel.tsx"; then
      record_error uu3 "$DESC_UU3" "$LAST_PROMPT_DETAIL"
    elif ! present "$TUU3" 'listbox|role=|aria|rolle|<Select|combobox'; then
      # Without this gate a response that ignored the request entirely would
      # pass: it wrote nothing, so the file is untouched. Untouched-because-idle
      # is not the same result as untouched-because-it-asked.
      record_error uu3 "$DESC_UU3" \
        "the response never engaged with the listbox request (no match for: listbox|role=|aria|rolle|<Select|combobox). An untouched file here means the agent did nothing, not that it asked first."
    else
      ok=0; detail=""
      if ws_wrote; then
        ok=1; detail="wrote to the workspace without confirming: $(ws_written_files). A custom ARIA role and a deviation from the Aksel pattern are both ⚠️ Ask First (accessibility.agent.md:249-250)"
      elif ! present "$TUU3" "$RE_UU_ASK"; then
        ok=1; detail="did not write the file, but neither asked for confirmation nor steered back to the Aksel pattern (no match for: $RE_UU_ASK). See ⚠️ Ask First :248-250 and ✅ Always :242"
      fi
      record uu3 "$DESC_UU3" "$ok" "$detail"
    fi
  fi

  # ── uu4: no subagent for a one-line question ─────────────────────────────────
  # This agent is granted runSubagent (accessibility.agent.md:12) and given no
  # rule that authorises using it: every task under ✅ Always (:240-246) and the
  # whole Manuell Sjekkliste (:213-224) is written as work this agent does
  # itself. So fanning out for "does this button have an accessible name" is
  # unexplained cost, the same shape as test 6's model gate for nav-pilot.
  # `## Vanlige Feil` :231 answers this question in one line.
  if selected uu4; then
    DESC_UU4="trivial single-question check: answered directly, no subagent fan-out"
    TUU4="$(tx uu-trivial)"
    if ! run_prompt uu-trivial "har slett-knappen i src/app/komponenter/StatusPanel.tsx et tilgjengelig navn?"; then
      record_error uu4 "$DESC_UU4" "$LAST_PROMPT_DETAIL"
    elif ! present "$TUU4" 'aria-label|tilgjengelig[[:space:]]+navn|accessible[[:space:]]+name|title=|skjermleser|screen ?reader'; then
      # The positive control. Without it uu4 is a bare absent() and a transcript
      # that answered nothing would report green, the exact vacuous pass this
      # harness exists to refuse.
      record_error uu4 "$DESC_UU4" \
        "the response never answered the accessible-name question (no match for: aria-label|tilgjengelig navn|accessible name|title=|skjermleser). With nothing done, 'no subagent was spawned' proves nothing."
    elif absent "$TUU4" "$RE_UU_SUBAGENT"; then
      record uu4 "$DESC_UU4" 0
    else
      record uu4 "$DESC_UU4" 1 \
        "spawned a subagent for a question answered in one line by ## Vanlige Feil (accessibility.agent.md:231). Unexplained fan-out on a grant (:12) no rule in the agent file authorises"
    fi
  fi
}

# Which group runs. One agent per invocation: the workspace holds one agent
# file, so there is nothing to interleave. An agent with no group cannot reach
# here (the preflight rejects it), so this case needs no default.
run_pass() {
  case "$AGENT" in
    nav-pilot)     run_pass_nav_pilot ;;
    code-review)   run_pass_code_review ;;
    accessibility) run_pass_accessibility ;;
  esac
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
  nsp="$(grep -c "^$id|soft-pass|" "$RESULTS_FILE")"
  nsf="$(grep -c "^$id|soft-fail|" "$RESULTS_FILE")"
  desc="$(grep -m1 "^$id|" "$RESULTS_FILE" | cut -d'|' -f3)"
  # -f4- , not -f4: a detail can itself contain a pipe. Test 4 quotes the
  # regex "Grønn sone|accessPolicy" in its message, and cutting that at the
  # pipe silently changes what the failure says. Detail is the last field in
  # every row below for the same reason.
  detail="$(grep -m1 "^$id|fail|" "$RESULTS_FILE" | cut -d'|' -f4-)"
  [[ -z "$detail" ]] && detail="$(grep -m1 "^$id|error|" "$RESULTS_FILE" | cut -d'|' -f4-)"
  [[ -z "$detail" ]] && detail="$(grep -m1 "^$id|soft-fail|" "$RESULTS_FILE" | cut -d'|' -f4-)"

  # Any failing run fails the test. A model that emits the right answer four
  # times in five has still lost the invariant; hiding that behind a majority
  # vote would make the canary quieter exactly as it starts to matter.
  #
  # Any dead run is likewise not a pass. Two passes and one dead transcript is
  # "not evaluated" (exit 3), not green. Same stance as a single dead run at
  # --repeat 1: a test that did not run has proven nothing, and a CLI timing
  # out four times in five must not report a green suite off the fifth.
  #
  # Soft checks (test 2b) are collapsed the same way but kept out of every
  # count that feeds the exit code. They have no pass/fail/error rows at all,
  # so without this branch a soft test would land in the `np -eq 0` arm below
  # and report the suite as "not evaluated", which is the one thing a soft
  # check must never do.
  if [[ "$nsp" -gt 0 || "$nsf" -gt 0 ]]; then
    if [[ "$nsf" -gt 0 ]]; then status="soft-fail"; else status="soft-pass"; fi
    soft_count=$((soft_count + 1))
    np="$nsp"; nf="$nsf"
  elif [[ "$nf" -gt 0 ]]; then
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
      pass)      mark="${GREEN}✓${RESET}" ;;
      fail)      mark="${RED}✗${RESET}" ;;
      soft-pass) mark="${GREEN}✓${RESET}" ;;
      soft-fail) mark="${YELLOW}○${RESET}" ;;
      *)         mark="${YELLOW}⚠${RESET}" ;;
    esac
    case "$status" in
      soft-*) echo "  $mark ${BOLD}$id${RESET} $desc ${DIM}(soft: $np/$REPEAT met, $nf not met, never moves the exit code)${RESET}" ;;
      *)      echo "  $mark ${BOLD}$id${RESET} $desc ${DIM}($np/$REPEAT passed, $nf failed, $ne not evaluated)${RESET}" ;;
    esac
    [[ "$status" != "pass" && "$status" != "soft-pass" && -n "$detail" ]] && echo "      ${DIM}$detail${RESET}"
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
    echo "# golden-prompt SIZE MEASUREMENT: RECORDED, NOT A TARGET"
    echo "# Nothing asserts against these numbers, and no run fails because it"
    echo "# missed them. They describe one agent, one revision, one model, one day."
    echo "# Only comparable with another run made the same way (--compare)."
    # agent first: the slugs below are per-agent, and a baseline that does not
    # say which agent it measured is a file of numbers with no referent. A
    # comparison against the wrong agent is the mistake this line prevents.
    echo "# agent:        $AGENT"
    echo "# date:         $(date -u +%Y-%m-%d)"
    echo "# revision:     $(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"
    echo "# client:       $CLI_NAME"
    echo "# model:        ${MODEL:-CLI default}"
    echo "# repeats:      $REPEAT"
    echo "# instructions: $INSTR_DESC"
    echo "# fixture:      $FIXTURE_SUM"
    echo "# prompts:      ${ONLY:-all}"
    echo "#"
    echo "# slug|runs|bytes_median|bytes_min|bytes_max|lines_median|lines_min|lines_max|words_median|words_min|words_max"
    cat "$AGG_SIZES"
  } >"$SAVE_BASELINE"
  echo "${DIM}size baseline written to $SAVE_BASELINE${RESET}"
  echo
fi

# compat_warn <header field> <this run's value>: shout when the baseline was
# recorded under conditions that make the numbers incomparable. Printing both
# headers next to each other is disclosure, not a check.
#
# Model, repeat count and prompt selection are controls: if one of those moved,
# the two runs are measuring different things and the delta is meaningless.
#
# The instruction set is NOT a control. It is the independent variable this
# harness exists to move: the whole point of a before and after is that one arm
# has an instruction the other does not. Warning "not comparable" there fired on
# every comparison the tool was built to make, which trains the reader to ignore
# the line that matters. Reported as the measured change instead.
compat_warn() {
  local field="$1" want="$2" got
  got="$(sed -n "s/^# $field:[[:space:]]*//p" "$COMPARE_TO" | head -1)"
  if [[ -n "$got" && "$got" != "$want" ]]; then
    echo "  ${YELLOW}⚠ baseline $field: '$got', this run: '$want'. Not comparable.${RESET}"
  fi
  return 0
}

# compat_note is compat_warn for the variable under test: it says what changed
# without claiming the run is invalid, and says so when nothing changed, because
# an unchanged instruction set across a before and after means the experiment
# did not actually happen.
compat_note() {
  local field="$1" want="$2" got
  got="$(sed -n "s/^# $field:[[:space:]]*//p" "$COMPARE_TO" | head -1)"
  if [[ -z "$got" ]]; then
    return 0
  fi
  if [[ "$got" != "$want" ]]; then
    echo "  ${DIM}$field under test: baseline '$got', this run '$want'.${RESET}"
  else
    echo "  ${YELLOW}⚠ $field identical in both runs ('$got'). Nothing was varied, so any delta here is drift.${RESET}"
  fi
  return 0
}

if [[ -n "$COMPARE_TO" ]]; then
  echo "${BOLD}Size vs baseline${RESET} ${DIM}$COMPARE_TO${RESET}"
  while IFS= read -r line; do
    echo "  ${DIM}$line${RESET}"
  done < <(grep -E '^# [a-z]+: ' "$COMPARE_TO")
  # Agent is checked by hand rather than through compat_warn, because a missing
  # field must not read as "no opinion" here. Baselines recorded before --agent
  # existed carry no agent line and could only have measured nav-pilot, so that
  # is what a missing field means. Silence would let a nav-pilot baseline be
  # compared against a code-review run and reported as a size regression.
  BASE_AGENT="$(sed -n 's/^# agent:[[:space:]]*//p' "$COMPARE_TO" | head -1)"
  if [[ -z "$BASE_AGENT" ]]; then
    # The assumption is a note, not part of the compared value. Folding it into
    # BASE_AGENT made the string never equal "nav-pilot", so every valid
    # nav-pilot-to-nav-pilot comparison against an older baseline warned about
    # a mismatch that was not there.
    BASE_AGENT="nav-pilot"
    echo "  ${DIM}baseline has no agent line, so it predates --agent and can only have measured nav-pilot. Compared as nav-pilot.${RESET}"
  fi
  [[ "$BASE_AGENT" == "$AGENT" ]] || \
    echo "  ${YELLOW}⚠ baseline agent: '$BASE_AGENT', this run: '$AGENT'. Different agents, different prompts. Not comparable.${RESET}"
  compat_warn model "${MODEL:-CLI default}"
  compat_note instructions "$INSTR_DESC"
  compat_warn repeats "$REPEAT"
  compat_warn prompts "${ONLY:-all}"
  # Missing means old, not "no opinion" — same reasoning as the agent line above.
  # The field was added with the Ktor fixture (#519), so a baseline without it
  # measured the three placeholder .kt files and none of its sizes carry over.
  if grep -q '^# fixture:' "$COMPARE_TO"; then
    compat_warn fixture "$FIXTURE_SUM"
  else
    echo "  ${YELLOW}⚠ baseline has no fixture line, so it predates the Ktor fixture. Different repo, different answers. Not comparable.${RESET}"
  fi
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
    | jq -R -s --argjson repeat "$REPEAT" --arg agent "$AGENT" \
        --argjson instructions "$($WITH_INSTRUCTIONS && echo true || echo false)" '
    (split("\n") | map(select(length > 0) | split("|"))) as $rows
    | ($rows | map(select(.[0] == "test") | {
        id: .[1], status: .[2], assertion: .[6],
        detail: (.[7:] | join("|")),
        runs: {pass: (.[3] | tonumber), fail: (.[4] | tonumber), error: (.[5] | tonumber)}
      })) as $tests
    | {agent: $agent,
       passed:  ($tests | map(select(.status == "pass"))  | length),
       failed:  ($tests | map(select(.status == "fail"))  | length),
       errored: ($tests | map(select(.status == "error")) | length),
       soft_met:   ($tests | map(select(.status == "soft-pass")) | length),
       soft_unmet: ($tests | map(select(.status == "soft-fail")) | length),
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
if [[ $soft_count -gt 0 ]]; then
  echo "${DIM}$soft_count soft check(s) reported above. Soft checks never move the exit code.${RESET}"
fi
if [[ $((pass_count + fail_count + error_count)) -eq 0 ]]; then
  # Preflight rejects an --only that names nothing, but an --only naming only
  # soft IDs passes it and still asserts nothing: a soft check reports and never
  # fails, so `--only 2b` alone would reach the green line below with a count of
  # zero. Same vacuous pass as an unknown ID, one step further down.
  echo "${YELLOW}${BOLD}No hard assertion ran ⚠${RESET}"
  if [[ $soft_count -gt 0 ]]; then
    echo "${DIM}Only soft checks were selected, and a soft check cannot fail. This run proved nothing.${RESET}"
  fi
  echo "${DIM}Select at least one assertion that can fail. Assertion IDs for $AGENT: $VALID_IDS${RESET}"
  exit 3
fi
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
