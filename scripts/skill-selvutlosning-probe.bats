#!/usr/bin/env bats
#
# Mutasjonssjekk for tokenhentingen i skill-selvutlosning-probe.sh (#660).
#
# cplt sin gh-guard er på som standard og blokkerer `gh auth token`. Guarden
# forklarer hvorfor på stderr. Skriptet kastet den forklaringen (`2>/dev/null`)
# og rådet i stedet «gh auth login» — som guarden også blokkerer. Testen feiler
# hvis forklaringen forsvinner igjen, eller hvis skriptet igjen gir det rådet
# som eneste utvei.

SCRIPT="${BATS_TEST_DIRNAME}/skill-selvutlosning-probe.sh"

GUARD_MSG="⚠️ BLOCKED by sandbox: revealing the GitHub token is not allowed in this environment."

setup() {
  SHIM="$(mktemp -d)"
  cat >"$SHIM/gh" <<EOF
#!/usr/bin/env bash
if [[ "\$1" == "auth" && "\$2" == "token" ]]; then
  printf '%s\n' "$GUARD_MSG" >&2
  printf 'Reason: token exfiltration prevention. Use the GH_TOKEN env var instead.\n' >&2
  exit 1
fi
exit 0
EOF
  printf '#!/usr/bin/env bash\nexit 0\n' >"$SHIM/copilot"
  chmod +x "$SHIM/gh" "$SHIM/copilot"
  OUTDIR="$(mktemp -d)"
}

teardown() {
  rm -rf "$SHIM" "$OUTDIR"
}

run_probe() {
  env -u COPILOT_GITHUB_TOKEN -u GH_TOKEN -u GITHUB_TOKEN \
    HOME="$OUTDIR/home" PATH="$SHIM:/usr/bin:/bin" \
    bash "$SCRIPT" --model dummy --out "$OUTDIR/out"
}

@test "guard-blokkering feiler høylytt med guardens egen melding" {
  run run_probe
  [ "$status" -eq 2 ]
  [[ "$output" == *"BLOCKED by sandbox"* ]]
}

@test "rådet peker på GH_TOKEN, ikke bare gh auth login" {
  run run_probe
  [[ "$output" == *"GH_TOKEN"* ]]
}

@test "eksplisitt token hopper over gh helt" {
  run env -u GH_TOKEN COPILOT_GITHUB_TOKEN=ghp_dummy \
    HOME="$OUTDIR/home" PATH="$SHIM:/usr/bin:/bin" \
    bash "$SCRIPT" --model dummy --out "$OUTDIR/out"
  [[ "$output" != *"ingen GitHub-token"* ]]
}
