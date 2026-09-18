#!/usr/bin/env bash
#
# Query Nav's Mimir, Loki and Tempo. Documented in SKILL.md next to this file.
#
# A skill's files install without the executable bit, so run this as:
#
#   bash "$NAV_PILOT_SKILLS_DIR/observability-debugging/obs-query.sh" ...
#
# What it is for: the four hosts below sit behind two cplt gates and a Mimir
# tenant header, and each of those refuses a request in a different way. Piped
# into jq, all of them arrive as `parse error: Invalid numeric literal`, which
# reads like a broken query. This reads the status code instead, keeps a
# refusal off stdout, and names the rule that stopped the request.
#
# Only a 2xx body reaches stdout, so `... | jq .` is always safe.

set -euo pipefail

SELF=obs-query.sh
UA="nav-pilot/observability-debugging"

# The hosts the nav-pilot agentpakke asks cplt to waive. Kept in one string so
# the 403 messages can print the list a user has to paste back.
WAIVED_HOSTS="mimir.nav.cloud.nais.io,loki.nav.cloud.nais.io,tempo.dev-gcp.nav.cloud.nais.io,tempo.prod-gcp.nav.cloud.nais.io"

usage() {
  cat <<'EOF'
Usage:
  bash obs-query.sh mimir <promql> [--range <start> <end> <step>] [--org tenant|nais]
  bash obs-query.sh loki  <logql>  [--range <start> <end> [step]] [--limit <n>] [--org tenant|nais]
  bash obs-query.sh tempo-search <env> <traceql> [--range <start> <end>] [--limit <n>] [--org tenant|nais]
  bash obs-query.sh tempo-trace  <env> <trace-id> [--org tenant|nais]

  <promql>/<logql>/<traceql>
            the expression, quoted so the shell keeps braces, pipes and quotes
  <env>     dev-gcp or prod-gcp. Tempo is the one host that carries it.

  --range <start> <end> [step]
            mimir: start/end as RFC3339 or unix seconds, step like 30s, all three required.
            loki: start/end as RFC3339 or unix nanoseconds, step optional and
              only meaningful for a metric query.
            tempo-search: start/end as unix seconds.
            Without it: mimir runs an instant query, loki and tempo use the
            server's own default window, which is the last hour.
  --limit <n>
            maximum entries (loki) or traces (tempo-search) returned.
  --org tenant|nais
            X-Scope-OrgID. Default: tenant, your team's application workloads.
            Use nais for platform data: nais-system components, node-exporter,
            kube-prometheus-stack rules, alerts. Same endpoint either way, so a
            wrong value returns real data answering a different question, or an
            empty result that looks like an outage.

Prints the API response on stdout and nothing else, so pipe it to jq yourself.
A refusal goes to stderr with the rule that caused it, and exits non-zero.
EOF
}

die() { printf '%s: %s\n' "$SELF" "$1" >&2; exit 2; }

org="tenant"
start=""
end=""
step=""
limit=""
pos=()

while [ $# -gt 0 ]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    --org)
      [ $# -ge 2 ] || die "--org needs a value: tenant or nais"
      org=$2; shift 2 ;;
    --limit)
      [ $# -ge 2 ] || die "--limit needs a number"
      limit=$2; shift 2 ;;
    --range)
      [ $# -ge 3 ] || die "--range needs <start> <end> and, for mimir, <step>"
      start=$2; end=$3; shift 3
      # step is optional for loki and tempo, so take a third value only when
      # the next argument is not another flag.
      if [ $# -ge 1 ]; then
        case "$1" in -*) ;; *) step=$1; shift ;; esac
      fi ;;
    --)
      shift
      pos+=("$@")
      break ;;
    -*) die "unknown option: $1" ;;
    *) pos+=("$1"); shift ;;
  esac
done

case "$org" in
  tenant|nais) ;;
  *) die "--org takes tenant or nais, not '$org'. tenant is your team's workloads (default), nais is the platform's own data." ;;
esac

[ "${#pos[@]}" -gt 0 ] || { usage >&2; die "no backend. Pick mimir, loki, tempo-search or tempo-trace."; }

backend=${pos[0]}
env_name=""
subject=""

case "$backend" in
  mimir|loki)
    [ "${#pos[@]}" -ge 2 ] || die "$backend needs an expression. Quote it so the shell keeps the braces."
    [ "${#pos[@]}" -le 2 ] || die "unexpected argument: ${pos[2]}. Quote the whole expression as one argument."
    subject=${pos[1]} ;;
  tempo-search|tempo-trace)
    [ "${#pos[@]}" -ge 3 ] || die "$backend needs <env> and then the $([ "$backend" = tempo-trace ] && echo "trace id" || echo "TraceQL expression"). env is dev-gcp or prod-gcp."
    [ "${#pos[@]}" -le 3 ] || die "unexpected argument: ${pos[3]}. Quote the whole expression as one argument."
    env_name=${pos[1]}
    subject=${pos[2]}
    case "$env_name" in
      dev-gcp|prod-gcp) ;;
      *) die "env must be dev-gcp or prod-gcp, not '$env_name'. Those are the two Tempo hosts the sandbox allows; a third is a decision taken in navikt/copilot, not here." ;;
    esac ;;
  *) die "unknown backend: $backend. Pick mimir, loki, tempo-search or tempo-trace." ;;
esac

args=()

case "$backend" in
  mimir)
    host="mimir.nav.cloud.nais.io"
    lang="PromQL"
    args+=(--data-urlencode "query=$subject")
    if [ -n "$start" ]; then
      [ -n "$step" ] || die "mimir --range needs <start> <end> <step>. Without a step the server has no resolution to sample at."
      url="https://$host/prometheus/api/v1/query_range"
      args+=(--data-urlencode "start=$start" --data-urlencode "end=$end" --data-urlencode "step=$step")
    else
      url="https://$host/prometheus/api/v1/query"
    fi ;;
  loki)
    host="loki.nav.cloud.nais.io"
    lang="LogQL"
    # query_range even without --range: a log search wants a window, and Loki's
    # own default is the last hour. The instant endpoint answers at a single
    # point in time, which is almost never the log question being asked.
    url="https://$host/loki/api/v1/query_range"
    args+=(--data-urlencode "query=$subject")
    if [ -n "$start" ]; then
      args+=(--data-urlencode "start=$start" --data-urlencode "end=$end")
      [ -z "$step" ] || args+=(--data-urlencode "step=$step")
    fi ;;
  tempo-search)
    host="tempo.$env_name.nav.cloud.nais.io"
    lang="TraceQL"
    url="https://$host/api/search"
    args+=(--data-urlencode "q=$subject")
    if [ -n "$start" ]; then
      args+=(--data-urlencode "start=$start" --data-urlencode "end=$end")
    fi ;;
  tempo-trace)
    host="tempo.$env_name.nav.cloud.nais.io"
    lang="trace id"
    url="https://$host/api/traces/$subject" ;;
esac

[ -z "$limit" ] || case "$backend" in
  loki|tempo-search) args+=(--data-urlencode "limit=$limit") ;;
  *) die "--limit does not apply to $backend." ;;
esac

# -G puts every --data-urlencode in the query string, which is the whole point
# of building the request here: the expression is encoded once, correctly,
# instead of being pasted into a URL where a brace or a pipe decides the
# outcome. --max-time as well as --connect-timeout, because a route the
# naisdevice gateway does not carry connects and then never answers. 120s is
# well past any query worth running interactively, so a timeout here is much
# more likely to be the route than the query; the message says both anyway,
# because telling someone to reconnect naisdevice when the query is simply too
# wide is the same wrong-place answer this script exists to stop.
rc=0
response=$(curl -sS -G \
  --connect-timeout 5 --max-time 120 \
  -H "User-Agent: $UA" \
  -H "X-Scope-OrgID: $org" \
  ${args[@]+"${args[@]}"} \
  -w '\n%{http_code}' \
  "$url") || rc=$?

if [ "$rc" -ne 0 ]; then
  printf '%s: no answer from %s (curl exit %s).\n' "$SELF" "$host" "$rc" >&2
  if [ "$rc" -eq 28 ]; then
    printf 'Exit 28 is a timeout at 120s: either the naisdevice gateway is not carrying this route, or the query is too wide. Narrow the window before you assume the network.\n' >&2
  fi
  printf 'These hosts route over naisdevice only. Connect it:\n' >&2
  printf '  nais device connect      (check with: nais device status)\n' >&2
  printf 'This is not a query problem. Do not rewrite the %s.\n' "$lang" >&2
  exit 1
fi

code=${response##*$'\n'}
body=${response%$'\n'*}

case "$code" in
  2*) printf '%s\n' "$body"; exit 0 ;;
esac

# Everything below is a refusal, and it goes to stderr. Putting a non-JSON body
# on stdout is how the original curl examples turned every one of these into
# `parse error: Invalid numeric literal` one pipe later.
case "$code" in
  401)
    printf '%s: 401 from %s. X-Scope-OrgID did not reach the server, and there is no default org.\n' "$SELF" "$host" >&2
    printf 'This run sent: X-Scope-OrgID: %s\n' "$org" >&2
    printf 'Valid values are tenant (the workloads your team deploys) and nais (platform data).\n' >&2
    printf 'If both look right, something between you and the host strips the header.\n' >&2
    ;;
  403)
    case "$body" in
      *[Pp]rivate*)
        printf '%s: 403 from %s, and the request never left the sandbox.\n' "$SELF" "$host" >&2
        printf '%s resolves to a private address over naisdevice, and cplt refuses a private target unless proxy.allow_private_domains names it.\n' "$host" >&2
        printf 'Grant it once, either way:\n' >&2
        printf '  nav-pilot sync --apply        and answer the sandbox question\n' >&2
        printf '  cplt config set proxy.allow_private_domains %s\n' "$WAIVED_HOSTS" >&2
        printf 'nav-pilot doctor says whether it is in force. This is not a query problem: do not rewrite the %s.\n' "$lang" >&2
        ;;
      *[Aa]llowlist*)
        printf '%s: 403 from %s: the host is outside proxy.allowed_domains, so cplt refused it before DNS.\n' "$SELF" "$host" >&2
        printf 'These four are listed: %s\n' "$WAIVED_HOSTS" >&2
        printf 'A fifth host is a decision somebody takes on purpose. Raise it in navikt/copilot rather than widening the allowlist locally.\n' >&2
        ;;
      *)
        printf '%s: 403 from %s, and the body names no cplt rule, so the server refused rather than the sandbox.\n' "$SELF" "$host" >&2
        printf '%s\n' "$body" >&2
        ;;
    esac ;;
  *)
    printf '%s: HTTP %s from %s\n' "$SELF" "$code" "$url" >&2
    printf '%s\n' "$body" >&2
    ;;
esac

exit 1
