#!/usr/bin/env bats
#
# Regresjonssjekk for auth-preflighten i nav-pilot-golden.sh.
#
# Preflighten leser hele stdout fra probe-kallet, og der ligger statusbanneret
# med `--resume`-sesjons-iden. Et bart `401` i regexen traff en id som
# `f313d1ee-401a-49a3-...`, og kjøringa døde på «is not authenticated» etter at
# modellkallet var betalt. Testen feiler hvis det treffet kommer tilbake, eller
# hvis en ekte 401 slutter å bli fanget.

SCRIPT="${BATS_TEST_DIRNAME}/nav-pilot-golden.sh"

setup() {
  SHIM="$(mktemp -d "$BATS_TEST_DIRNAME/.nav-pilot-golden.bats.XXXXXX")"
}

teardown() {
  rm -rf "$SHIM"
}

# Lager en copilot-shim som svarer med $1 på -p og ellers later som den virker.
make_shim() {
  cat >"$SHIM/copilot" <<EOF
#!/bin/bash
if [[ "\$1" == "--version" ]]; then echo "GitHub Copilot CLI 1.0.83."; exit 0; fi
cat <<'PROBE'
$1
PROBE
exit 0
EOF
  chmod +x "$SHIM/copilot"
}

make_prompt_failure_shim() {
  local exit_code="$1"
  cat >"$SHIM/copilot" <<EOF
#!/bin/bash
if [[ "\$1" == "--version" ]]; then echo "GitHub Copilot CLI 1.0.83."; exit 0; fi
if [[ "\$1" == "-p" ]]; then
  if [[ ! -f "$SHIM/preflight-complete" ]]; then
    touch "$SHIM/preflight-complete"
    echo "OK"
    exit 0
  fi
  echo "This transcript is long enough to prove that process failures are not ignored."
  exit $exit_code
fi
exit 0
EOF
  chmod +x "$SHIM/copilot"
}

make_mixed_shim() {
  local exit_code="$1"
  cat >"$SHIM/copilot" <<EOF
#!/bin/bash
if [[ "\$1" == "--version" ]]; then echo "GitHub Copilot CLI 1.0.83."; exit 0; fi
if [[ ! -f "$SHIM/preflight-complete" ]]; then
  touch "$SHIM/preflight-complete"
  echo "OK"
elif [[ "\$2" == *"ny tjeneste"* ]]; then
  if [[ ! -f "$SHIM/first-run-complete" ]]; then
    touch "$SHIM/first-run-complete"
    echo "A long enough answer to evaluate, without any blind-spot audit count."
  else
    echo "This transcript is long enough to prove a soft result cannot mask a failed repeat."
    exit $exit_code
  fi
else
  echo "Use TokenX to retain the user's context when calling the second service."
fi
EOF
  chmod +x "$SHIM/copilot"
}

make_timeout_shim() {
  cat >"$SHIM/timeout" <<'EOF'
#!/bin/bash
shift
"$@"
EOF
  chmod +x "$SHIM/timeout"
}

run_preflight() {
  PATH="$SHIM:/usr/bin:/bin" run /bin/bash "$SCRIPT" --only 2 --repeat 1
}

@test "sesjons-id med 401 i seg feiler ikke preflighten" {
  make_shim 'OK

Changes    +0 -0
AI Credits 3.16 (2s)
Tokens     ↑ 29.5k (18.3k cached, 11.2k written) • ↓ 4
Resume     copilot --resume=f313d1ee-401a-49a3-8434-6ebc7e35b464'
  run_preflight
  [[ "$output" != *"is not authenticated"* ]]
  [ "$status" -ne 2 ]
}

@test "ekte 401 feiler preflighten" {
  make_shim 'Error: request failed with HTTP 401'
  run_preflight
  [[ "$output" == *"is not authenticated"* ]]
  [ "$status" -eq 2 ]
}

@test "ekte innloggingsfeil feiler preflighten" {
  make_shim 'You are not logged in. Run copilot to sign in.'
  run_preflight
  [[ "$output" == *"is not authenticated"* ]]
  [ "$status" -eq 2 ]
}

@test "ukjent reasoning effort avvises før modellkall" {
  run bash "$SCRIPT" --dry-run --effort impossible
  [[ "$output" == *"--effort has an invalid value"* ]]
  [ "$status" -eq 2 ]
}

@test "ukjent context tier avvises før modellkall" {
  run bash "$SCRIPT" --dry-run --context huge
  [[ "$output" == *"--context has an invalid value"* ]]
  [ "$status" -eq 2 ]
}

@test "timeout is recorded as a failed attempt even with a long transcript" {
  make_prompt_failure_shim 124
  make_timeout_shim
  PATH="$SHIM:/usr/bin:/bin" run /bin/bash "$SCRIPT" --only 2 --save-baseline "$SHIM/baseline.txt"

  [ "$status" -eq 1 ]
  [[ "$output" == *"timed out after"* ]]
  grep -q '^t2|1|124|timeout|' "$SHIM/baseline-attempts.psv"
}

@test "CLI failure is recorded as a failed attempt even with a long transcript" {
  make_prompt_failure_shim 42
  PATH="$SHIM:/usr/bin:/bin" run /bin/bash "$SCRIPT" --only 2 --save-baseline "$SHIM/baseline.txt"

  [ "$status" -eq 1 ]
  grep -q '^t2|1|42|cli_failure|' "$SHIM/baseline-attempts.psv"
}

@test "a soft result does not hide a dead repeat" {
  cat >"$SHIM/copilot" <<EOF
#!/bin/bash
if [[ "\$1" == "--version" ]]; then echo "GitHub Copilot CLI 1.0.83."; exit 0; fi
if [[ ! -f "$SHIM/preflight-complete" ]]; then
  touch "$SHIM/preflight-complete"
  echo "OK"
elif [[ ! -f "$SHIM/first-run-complete" ]]; then
  touch "$SHIM/first-run-complete"
  echo "A long enough answer to evaluate, without any blind-spot audit count."
else
  echo "x"
fi
EOF
  chmod +x "$SHIM/copilot"

  PATH="$SHIM:/usr/bin:/bin" run /bin/bash "$SCRIPT" --only 2b --repeat 2 --save-baseline "$SHIM/baseline.txt"

  [ "$status" -eq 3 ]
  [[ "$output" == *"soft: 0/2 met, 1 not met, 1 not evaluated"* ]]
  grep -q '^2b|1|soft-fail|' "$SHIM/baseline-results.psv"
  grep -q '^2b|2|error|' "$SHIM/baseline-results.psv"
  grep -q '^t2|2|0|short_transcript|' "$SHIM/baseline-attempts.psv"
}

@test "a soft result cannot hide a CLI failure beside a passing hard assertion" {
  make_mixed_shim 42
  PATH="$SHIM:/usr/bin:/bin" run /bin/bash "$SCRIPT" --only 2b,5 --repeat 2 --save-baseline "$SHIM/baseline.txt"

  [ "$status" -eq 1 ]
  [[ "$output" == *"1 failed"* ]]
  [[ "$output" == *"soft: 0 met, 1 not met"* ]]
  grep -q '^2b|2|fail|' "$SHIM/baseline-results.psv"
  grep -q '^5|2|pass|' "$SHIM/baseline-results.psv"
}

@test "a soft result cannot hide a timeout beside a passing hard assertion" {
  make_mixed_shim 124
  make_timeout_shim
  PATH="$SHIM:/usr/bin:/bin" run /bin/bash "$SCRIPT" --only 2b,5 --repeat 2

  [ "$status" -eq 1 ]
  [[ "$output" == *"timed out after"* ]]
  [[ "$output" == *"1 failed"* ]]
}

@test "exit 124 without timeout wrapper is a CLI failure" {
  make_prompt_failure_shim 124
  cat >"$SHIM/no-timeout.bash" <<'EOF'
command() {
  if [[ "$1" == "-v" && ( "$2" == "timeout" || "$2" == "gtimeout" ) ]]; then
    return 1
  fi
  builtin command "$@"
}
EOF
  BASH_ENV="$SHIM/no-timeout.bash" PATH="$SHIM:/usr/bin:/bin" \
    run /bin/bash "$SCRIPT" --only 2 --save-baseline "$SHIM/baseline.txt"

  [ "$status" -eq 1 ]
  [[ "$output" != *"timed out after"* ]]
  grep -q '^t2|1|124|cli_failure|' "$SHIM/baseline-attempts.psv"
}

@test "hard failure takes precedence over soft and unevaluated repeats" {
  cat >"$SHIM/copilot" <<EOF
#!/bin/bash
if [[ "\$1" == "--version" ]]; then echo "GitHub Copilot CLI 1.0.83."; exit 0; fi
if [[ ! -f "$SHIM/preflight-complete" ]]; then
  touch "$SHIM/preflight-complete"
  echo "OK"
elif [[ ! -f "$SHIM/first-run-complete" ]]; then
  touch "$SHIM/first-run-complete"
  echo "A long enough answer to evaluate, without any blind-spot audit count."
elif [[ ! -f "$SHIM/second-run-complete" ]]; then
  touch "$SHIM/second-run-complete"
  echo "This transcript is long enough to prove a hard failure was not caused by length."
  exit 42
else
  echo "x"
fi
EOF
  chmod +x "$SHIM/copilot"
  PATH="$SHIM:/usr/bin:/bin" run /bin/bash "$SCRIPT" --only 2b --repeat 3 --save-baseline "$SHIM/baseline.txt"

  [ "$status" -eq 1 ]
  [[ "$output" == *"0/3 passed, 1 failed, 1 not evaluated; soft: 0 met, 1 not met"* ]]
  grep -q '^2b|1|soft-fail|' "$SHIM/baseline-results.psv"
  grep -q '^2b|2|fail|' "$SHIM/baseline-results.psv"
  grep -q '^2b|3|error|' "$SHIM/baseline-results.psv"
}

@test "missing uuidgen after a failed prompt is not misclassified as a CLI failure" {
  make_prompt_failure_shim 42
  cat >"$SHIM/uuidgen" <<'EOF'
#!/bin/bash
exit 1
EOF
  chmod +x "$SHIM/uuidgen"
  PATH="$SHIM:/usr/bin:/bin" run /bin/bash "$SCRIPT" --only 2,4 --save-baseline "$SHIM/baseline.txt"

  [ "$status" -eq 1 ]
  grep -q '^4|1|error|' "$SHIM/baseline-results.psv"
}
