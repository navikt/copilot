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
  SHIM="$(mktemp -d)"
}

teardown() {
  rm -rf "$SHIM"
}

# Lager en copilot-shim som svarer med $1 på -p og ellers later som den virker.
make_shim() {
  cat >"$SHIM/copilot" <<EOF
#!/usr/bin/env bash
if [[ "\$1" == "--version" ]]; then echo "GitHub Copilot CLI 1.0.83."; exit 0; fi
cat <<'PROBE'
$1
PROBE
exit 0
EOF
  chmod +x "$SHIM/copilot"
}

run_preflight() {
  PATH="$SHIM:/usr/bin:/bin" run bash "$SCRIPT" --only 2 --repeat 1
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
