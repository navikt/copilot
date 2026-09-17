#!/usr/bin/env bats
#
# skills/observability-debugging/obs-query.sh builds requests a model used to
# hand-write, and the two things it has to get right are invisible until they
# are wrong: a LogQL selector must survive the shell as one argument with its
# braces, quotes and pipes intact, and a refusal must never reach stdout, where
# a `| jq` turns it back into `parse error: Invalid numeric literal`.
#
# Both are checked here against a fake curl that records its argv and answers
# with whatever status the test asks for.

bats_require_minimum_version 1.5.0

SCRIPT="${BATS_TEST_DIRNAME}/../skills/observability-debugging/obs-query.sh"

setup() {
  BIN="$BATS_TEST_TMPDIR/bin"
  ARGV="$BATS_TEST_TMPDIR/argv"
  mkdir -p "$BIN"
  cat >"$BIN/curl" <<'EOF'
#!/usr/bin/env bash
: >"$ARGV"
for a in "$@"; do printf '%s\n' "$a" >>"$ARGV"; done
[ "${FAKE_EXIT:-0}" -eq 0 ] || exit "$FAKE_EXIT"
printf '%s\n%s' "${FAKE_BODY:-{\}}" "${FAKE_CODE:-200}"
EOF
  chmod +x "$BIN/curl"
  PATH="$BIN:$PATH"
  export PATH ARGV
  export FAKE_BODY='{"status":"success"}'
  export FAKE_CODE=200
  export FAKE_EXIT=0
}

# argv_has asserts an exact argument was passed, whitespace and all.
argv_has() { grep -qxF -- "$1" "$ARGV"; }

# out_of runs the script and returns only stdout, so a test can prove a refusal
# stayed off it.
out_of() { bash "$SCRIPT" "$@" 2>/dev/null; }

# ── request construction ────────────────────────────────────────────────────

@test "mimir without --range is an instant query, urlencoded, org tenant" {
  run bash "$SCRIPT" mimir 'up{namespace="nais-system"}'
  [ "$status" -eq 0 ]
  argv_has "https://mimir.nav.cloud.nais.io/prometheus/api/v1/query"
  argv_has 'query=up{namespace="nais-system"}'
  argv_has "X-Scope-OrgID: tenant"
  argv_has "User-Agent: nav-pilot/observability-debugging"
  argv_has "-G"
}

@test "mimir --range becomes query_range with start, end and step" {
  run bash "$SCRIPT" mimir 'up' --range 100 200 30s
  [ "$status" -eq 0 ]
  argv_has "https://mimir.nav.cloud.nais.io/prometheus/api/v1/query_range"
  argv_has "start=100"
  argv_has "end=200"
  argv_has "step=30s"
}

@test "mimir --range without a step is refused, not sent" {
  run bash "$SCRIPT" mimir 'up' --range 100 200
  [ "$status" -eq 2 ]
  [[ "$output" == *"needs <start> <end> <step>"* ]]
  [ ! -f "$ARGV" ]
}

@test "a LogQL selector survives as one argument, braces pipes and quotes intact" {
  sel='{k8s_cluster_name="prod-gcp",service_name="min-app"} |= "ERROR" | json | level="error"'
  run bash "$SCRIPT" loki "$sel" --limit 50
  [ "$status" -eq 0 ]
  argv_has "query=$sel"
  argv_has "limit=50"
  argv_has "https://loki.nav.cloud.nais.io/loki/api/v1/query_range"
}

@test "loki uses query_range even without --range" {
  run bash "$SCRIPT" loki '{service_name="a"}'
  [ "$status" -eq 0 ]
  argv_has "https://loki.nav.cloud.nais.io/loki/api/v1/query_range"
  run ! grep -q "^start=" "$ARGV"
}

@test "tempo-search puts the env in the host and the expression in q" {
  run bash "$SCRIPT" tempo-search prod-gcp '{resource.service.name="min-app" && duration>2s}' --limit 10
  [ "$status" -eq 0 ]
  argv_has "https://tempo.prod-gcp.nav.cloud.nais.io/api/search"
  argv_has 'q={resource.service.name="min-app" && duration>2s}'
  argv_has "limit=10"
}

@test "tempo-trace puts the id in the path and sends no query parameters" {
  run bash "$SCRIPT" tempo-trace dev-gcp abc123
  [ "$status" -eq 0 ]
  argv_has "https://tempo.dev-gcp.nav.cloud.nais.io/api/traces/abc123"
  run ! grep -q -- "--data-urlencode" "$ARGV"
}

@test "--org nais reaches the header" {
  run bash "$SCRIPT" mimir 'up' --org nais
  [ "$status" -eq 0 ]
  argv_has "X-Scope-OrgID: nais"
}

# ── arguments refused before anything is sent ───────────────────────────────

@test "an unknown org is refused and names both valid values" {
  run bash "$SCRIPT" mimir 'up' --org nav
  [ "$status" -eq 2 ]
  [[ "$output" == *"tenant"* ]]
  [[ "$output" == *"nais"* ]]
  [ ! -f "$ARGV" ]
}

@test "a tempo env outside the two allowed hosts is refused here, not by cplt" {
  run bash "$SCRIPT" tempo-search dev-fss '{}'
  [ "$status" -eq 2 ]
  [[ "$output" == *"dev-gcp"* ]]
  [[ "$output" == *"navikt/copilot"* ]]
  [ ! -f "$ARGV" ]
}

@test "an unquoted expression that split into two arguments is refused" {
  run bash "$SCRIPT" loki '{service_name="a"}' '|= "ERROR"'
  [ "$status" -eq 2 ]
  [[ "$output" == *"one argument"* ]]
}

@test "an unknown backend names the four that exist" {
  run bash "$SCRIPT" grafana 'up'
  [ "$status" -eq 2 ]
  [[ "$output" == *"mimir, loki, tempo-search or tempo-trace"* ]]
}

# ── the four refusals, told apart ───────────────────────────────────────────

@test "a 2xx body is the only thing that reaches stdout" {
  FAKE_BODY='{"status":"success","data":{"result":[]}}'
  run out_of mimir 'up'
  [ "$status" -eq 0 ]
  [ "$output" = '{"status":"success","data":{"result":[]}}' ]
}

@test "the private-IP 403 names allow_private_domains and keeps stdout clean" {
  export FAKE_CODE=403
  export FAKE_BODY='Resolved to a private IP, blocked by cplt'
  [ -z "$(out_of mimir 'up')" ]
  run bash "$SCRIPT" mimir 'up'
  [ "$status" -eq 1 ]
  [[ "$output" == *"proxy.allow_private_domains"* ]]
  [[ "$output" == *"nav-pilot sync --apply"* ]]
  [[ "$output" == *"do not rewrite the PromQL"* ]]
}

@test "the allowlist 403 names the other key and points at the repo" {
  export FAKE_CODE=403
  export FAKE_BODY='Domain not in allowlist'
  run bash "$SCRIPT" loki '{a="b"}'
  [ "$status" -eq 1 ]
  [[ "$output" == *"proxy.allowed_domains"* ]]
  [[ "$output" == *"navikt/copilot"* ]]
  [[ "$output" != *"allow_private_domains"* ]]
}

@test "a 403 that names no cplt rule is reported as the server refusing" {
  export FAKE_CODE=403
  export FAKE_BODY='no access to this tenant'
  run bash "$SCRIPT" mimir 'up'
  [ "$status" -eq 1 ]
  [[ "$output" == *"server refused rather than the sandbox"* ]]
  [[ "$output" == *"no access to this tenant"* ]]
}

@test "a 401 names the org value that was actually sent" {
  export FAKE_CODE=401
  export FAKE_BODY='no org id'
  run bash "$SCRIPT" mimir 'up' --org nais
  [ "$status" -eq 1 ]
  [[ "$output" == *"X-Scope-OrgID: nais"* ]]
  [[ "$output" == *"no default org"* ]]
}

@test "a connection failure names naisdevice and not the query" {
  export FAKE_EXIT=7
  run bash "$SCRIPT" mimir 'up'
  [ "$status" -eq 1 ]
  [[ "$output" == *"nais device connect"* ]]
  [[ "$output" == *"Do not rewrite the PromQL"* ]]
}

@test "a timeout says so, because it looks like a disconnect and is not" {
  export FAKE_EXIT=28
  run bash "$SCRIPT" tempo-trace prod-gcp abc
  [ "$status" -eq 1 ]
  [[ "$output" == *"Exit 28 is a timeout"* ]]
}

@test "a 400 keeps the server's error off stdout but still shows it" {
  export FAKE_CODE=400
  export FAKE_BODY='{"status":"error","errorType":"bad_data","error":"parse error"}'
  [ -z "$(out_of mimir 'up{')" ]
  run bash "$SCRIPT" mimir 'up{'
  [ "$status" -eq 1 ]
  [[ "$output" == *"HTTP 400"* ]]
  [[ "$output" == *"bad_data"* ]]
}
