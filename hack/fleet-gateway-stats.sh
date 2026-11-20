#!/usr/bin/env bash
# fleet-gateway prod summary from Loki and Mimir. Usage: fleet-gateway-stats.sh [hours=24]
# Prints counts only; never prints navident or other personal fields.
# shortcut: Loki returns at most 5000 lines per query, so counts are a floor on busy days; page by time when traffic grows.
set -euo pipefail

HOURS="${1:-24}"
LOKI=https://loki.nav.cloud.nais.io
MIMIR=https://mimir.nav.cloud.nais.io/prometheus
SEL='{service_name="fleet-gateway",k8s_cluster_name="prod"}'
H=(-H 'X-Scope-OrgID: tenant')
END=$(date +%s)
START=$((END - HOURS * 3600))

loki() { # $1 = extra LogQL filter
  curl -fsS -m 120 -G "${H[@]}" "$LOKI/loki/api/v1/query_range" \
    --data-urlencode "query=$SEL $1" --data-urlencode "start=${START}000000000" \
    --data-urlencode "end=${END}000000000" --data-urlencode limit=5000 \
    --data-urlencode direction=forward
}

if ! curl -fsS -m 10 -o /dev/null "${H[@]}" "$LOKI/loki/api/v1/labels"; then
  echo "Loki is unreachable. Connect naisdevice and try again." >&2
  exit 1
fi

echo "== fleet-gateway prod, last ${HOURS}h =="

echo "-- HTTP 4xx/5xx by status method path --"
loki '| json | status >= 400' | python3 -c '
import sys, json, collections
c = collections.Counter()
for r in json.load(sys.stdin)["data"]["result"]:
    for _, line in r["values"]:
        try:
            d = json.loads(line)
            c[(d["status"], d.get("method", "-"), d.get("path", "-"))] += 1
        except (ValueError, KeyError, TypeError):
            pass
for k, n in c.most_common():
    print(n, *k)'

echo "-- audit refresh / audit login by msg outcome client_id --"
loki '|~ "audit (refresh|login)"' | python3 -c '
import sys, json, collections
c = collections.Counter()
for r in json.load(sys.stdin)["data"]["result"]:
    for _, line in r["values"]:
        try:
            d = json.loads(line)
        except ValueError:
            continue
        m = d.get("msg", "")
        if m.startswith(("audit refresh", "audit login")):
            c[(m, d.get("outcome", "-"), str(d.get("client_id", "-"))[:40])] += 1
for k, n in c.most_common():
    print(n, *k)'

echo "-- Mimir: total requests by code --"
NAME=$(curl -fsS -m 60 "${H[@]}" "$MIMIR/api/v1/label/__name__/values" |
  python3 -c '
import sys, json
for n in json.load(sys.stdin)["data"]:
    if "fleet_gateway" in n or "fleetgateway" in n:
        print(n)' | grep -E '(request|http).*total$' | head -1 || true)
if [ -z "$NAME" ]; then echo "no fleet_gateway request counter found"; exit 0; fi
echo "metric: $NAME"
curl -fsS -m 60 -G "${H[@]}" "$MIMIR/api/v1/query" \
  --data-urlencode "query=sum by (code) (increase(${NAME}[${HOURS}h]))" |
  python3 -c '
import sys, json
for r in json.load(sys.stdin)["data"]["result"]:
    print(round(float(r["value"][1])), r["metric"].get("code", "-"))'
