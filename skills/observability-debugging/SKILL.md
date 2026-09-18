---
name: observability-debugging
description: Feilsøk produksjonsproblemer med Mimir-metrikker, Loki-logger og Tempo-traces gjennom strukturerte debugging-workflows for Nav-utviklere
license: MIT
compatibility: Application deployed on Nais
metadata:
  domain: observability
  tags: debugging mimir loki tempo prometheus traces logs kubectl nais
---

# Observability debugging with three-pillar diagnostics

Structured debugging workflows using Nav's observability stack. Replaces guesswork with systematic investigation across metrics, logs, and traces.

## Mental Model

| Pillar | Tool | Answers | Endpoint |
|--------|------|---------|----------|
| **Metrics** | Mimir (Prometheus) | *What* is happening? (rates, quantiles, saturation) | `mimir.nav.cloud.nais.io` |
| **Logs** | Loki | *Why* is it happening? (errors, context, messages) | `loki.nav.cloud.nais.io` |
| **Traces** | Tempo | *Where* in the call chain? (latency, dependencies) | `tempo.$env.nav.cloud.nais.io` |

## Debugging Workflow

```
Symptom
  ├── High error rate → Start with Metrics (Mimir)
  │     └── Find failing endpoint → Logs → Traces
  ├── Slow responses → Start with Metrics (latency quantiles)
  │     └── Find slow endpoint → Traces → Identify bottleneck
  ├── Specific user error → Start with Logs (Loki)
  │     └── Find trace_id → Tempo → See full call chain
  ├── Intermittent failures → Start with Traces (Tempo)
  │     └── Filter by error status → Correlate with Metrics/Logs
  └── Resource exhaustion → Start with Metrics (memory/CPU)
        └── kubectl describe → Logs for OOM context
```

## Quick Access

Every query below goes through one wrapper, `obs-query.sh`, which ships next to this file:

```bash
bash "$NAV_PILOT_SKILLS_DIR/observability-debugging/obs-query.sh" --help
```

nav-pilot sets `NAV_PILOT_SKILLS_DIR` at launch to wherever it materialised these skills for the client you are running, which differs per client, so use the variable rather than a literal path. `bash`, because a skill's files install without the executable bit.

Why a wrapper rather than a hand-written `curl`: these four hosts sit behind two cplt gates and a Mimir tenant header, and each refuses a request differently. Piped into `jq`, every one of them arrives as `parse error: Invalid numeric literal`, which reads like a broken query. The wrapper reads the status code, keeps a refusal off stdout, and names the rule that stopped the request. **Only a 2xx body reaches stdout, so `| jq .` is always safe.**

Where `NAV_PILOT_SKILLS_DIR` is unset, nav-pilot materialised no skills for this client and the script is not on disk. Use `curl`, send `X-Scope-OrgID` yourself, and read [When a query returns nothing useful](#when-a-query-returns-nothing-useful) before you touch the query.

### The org header

`X-Scope-OrgID` takes one of exactly two values, and the wrapper sends it for you:

| `--org` | Data |
|---|---|
| `tenant` (default) | Your team's application workloads: what you deploy |
| `nais` | Platform: `nais-system` components, node-exporter, kube-prometheus-stack rules, alerts |

Same endpoint either way. The header is the only difference, so a wrong value returns real data answering a different question, or an empty result that looks like an outage. Omitting it is a **401**, not a default. Federation is off, so `nais|tenant` is rejected: needing both means two calls.

### Mimir metrics

```bash
OBS="$NAV_PILOT_SKILLS_DIR/observability-debugging/obs-query.sh"

# Instant query: error rate for an app
bash "$OBS" mimir "sum(rate(http_server_requests_seconds_count{k8s_cluster_name=\"$CLUSTER\",app=\"$APP\",status=~\"5..\"}[5m]))" | jq .

# Range query, last hour at one-minute resolution
bash "$OBS" mimir "up{app=\"$APP\"}" --range "$(( $(date +%s) - 3600 ))" "$(date +%s)" 60 | jq .
```

`--range` takes start, end and step, all three. Without it the query is an instant one.

### Loki logs

> **Labels (indexed, fast):** `service_name`, `service_namespace`, `app_name`, `env`, `deployment_environment`, `k8s_cluster_name`, `kind` (log/event/exception/measurement)
> **Structured metadata (fast filter with `|`):** `k8s_pod_name`, `k8s_node_name`, `k8s_container_name`, `detected_level`
> **Log line fields (require `| json`, slower):** `level`, `message`, `trace_id`, `span_id`, `logger_name`, `thread_name`
> Always narrow with labels first, then filter metadata/fields.

> **One global endpoint** (`loki.nav.cloud.nais.io`), like Mimir. Pick the cluster with the
> `k8s_cluster_name="$CLUSTER"` label. For Nav production workloads the label is `prod`, not
> the Tempo hostname segment `prod-gcp`. Use the label value returned by the metrics or logs.

```bash
OBS="$NAV_PILOT_SKILLS_DIR/observability-debugging/obs-query.sh"

# Error lines for one app in one cluster
bash "$OBS" loki "{k8s_cluster_name=\"$CLUSTER\",service_name=\"$APP\"} |= \"ERROR\"" --limit 50 | jq .

# Parse structured fields for level/detail
bash "$OBS" loki "{k8s_cluster_name=\"$CLUSTER\",service_name=\"$APP\"} | json | level=\"error\"" | jq .
```

Loki queries are always range queries; the server's own window is the last hour. Narrow it with `--range <start> <end>`, where both are RFC3339 or unix nanoseconds.

### Tempo traces

> **Gotcha:** Tempo search may return unrelated traces when your service has no spans. Always verify `rootServiceName` matches your app.

Tempo is the one host that carries the environment, so it takes it as an argument: `dev-gcp` or `prod-gcp`. Those are the only two the sandbox allows, and the wrapper refuses a third itself rather than letting it come back as a confusing 403.

```bash
OBS="$NAV_PILOT_SKILLS_DIR/observability-debugging/obs-query.sh"

# Search traces by service name
bash "$OBS" tempo-search dev-gcp "{resource.service.name=\"$APP\"}" --limit 20 | jq --arg app "$APP" '.traces[] | select(.rootServiceName == $app)'

# Find slow traces (>2s)
bash "$OBS" tempo-search dev-gcp "{resource.service.name=\"$APP\" && duration>2s}" | jq .

# Get one trace by ID
bash "$OBS" tempo-trace dev-gcp "$TRACE_ID" | jq .
```

## Correlation Patterns

Every block starts by naming the wrapper once, because each shell it runs in is a fresh one.

### Pattern 1: Error spike → Root cause

```bash
OBS="$NAV_PILOT_SKILLS_DIR/observability-debugging/obs-query.sh"

# 1. Confirm the error rate in Mimir, split by endpoint
bash "$OBS" mimir "sum(rate(http_server_requests_seconds_count{k8s_cluster_name=\"$CLUSTER\",app=\"$APP\",status=~\"5..\"}[5m]))by(uri)" | jq .

# 2. Find the error logs in Loki, last 15 minutes
bash "$OBS" loki "{k8s_cluster_name=\"$CLUSTER\",service_name=\"$APP\"} | json | level=\"error\"" \
  --limit 20 --range "$(( $(date +%s) - 900 ))000000000" "$(date +%s)000000000" \
  | jq -r '.data.result[].values[][1]' | head -10

# 3. Pull the trace_id out of an error log, then look it up
bash "$OBS" tempo-trace prod-gcp "$TRACE_ID" \
  | jq '.batches[].scopeSpans[].spans[] | {name, status, duration: ((.endTimeUnixNano - .startTimeUnixNano) / 1000000)}'
```

### Pattern 2: Slow endpoint → Bottleneck

```bash
OBS="$NAV_PILOT_SKILLS_DIR/observability-debugging/obs-query.sh"

# 1. Find the slow endpoints in Mimir (p95 latency)
bash "$OBS" mimir "histogram_quantile(0.95,sum(rate(http_server_requests_seconds_bucket{k8s_cluster_name=\"$CLUSTER\",app=\"$APP\"}[5m]))by(le,uri))" \
  | jq '.data.result[] | {endpoint: .metric.uri, p95_seconds: .value[1]}'

# 2. Find the slow traces in Tempo
bash "$OBS" tempo-search prod-gcp "{resource.service.name=\"$APP\" && duration>1s}" --limit 10 | jq .

# 3. Inspect the trace for the bottleneck span
bash "$OBS" tempo-trace prod-gcp "$TRACE_ID" \
  | jq '.batches[].scopeSpans[].spans[] | select((.endTimeUnixNano - .startTimeUnixNano) > 500000000) | {name, duration_ms: ((.endTimeUnixNano - .startTimeUnixNano) / 1000000)}'
```

### Pattern 3: Pod issues → Resource context

`kubectl` and the `nais` CLI are not wrapped: they authenticate on their own, answer on the Kubernetes API rather than on these four hosts, and already report their own failures.

```bash
OBS="$NAV_PILOT_SKILLS_DIR/observability-debugging/obs-query.sh"

# 1. Check pod status
kubectl get pods -n "$NAMESPACE" -l app="$APP"

# 2. Memory usage (% of limit)
bash "$OBS" mimir "container_memory_working_set_bytes{k8s_cluster_name=\"$CLUSTER\",app=\"$APP\"}/container_spec_memory_limit_bytes{k8s_cluster_name=\"$CLUSTER\",app=\"$APP\"}*100" \
  | jq '.data.result[] | {pod: .metric.pod, memory_pct: .value[1]}'

# 3. CPU throttling
bash "$OBS" mimir "rate(container_cpu_cfs_throttled_periods_total{k8s_cluster_name=\"$CLUSTER\",app=\"$APP\"}[5m])/rate(container_cpu_cfs_periods_total{k8s_cluster_name=\"$CLUSTER\",app=\"$APP\"}[5m])*100" | jq .

# 4. Recent OOM kills
kubectl get events -n "$NAMESPACE" --field-selector reason=OOMKilling --sort-by='.lastTimestamp' | tail -5
```

## kubectl + nais CLI Integration

```bash
# View app status
nais app status $APP -n $NAMESPACE

# Port-forward to app (useful for /metrics endpoint)
kubectl port-forward -n $NAMESPACE deploy/$APP 8080:8080
curl localhost:8080/metrics | grep -v "^#" | sort

# Get recent pod restarts
kubectl get pods -n $NAMESPACE -l app=$APP -o custom-columns=NAME:.metadata.name,RESTARTS:.status.containerStatuses[0].restartCount,STARTED:.status.startTime

# Tail live logs (last 5 min)
kubectl logs -n $NAMESPACE -l app=$APP --since=5m -f --tail=50

# Check app configuration
nais app get $APP -n $NAMESPACE -o yaml
```

## jq for Observability Data

All API responses return JSON, and the wrapper puts only a 2xx body on stdout, so a pipeline never sees a refusal. Every block below expects `OBS` to be set first:

```bash
OBS="$NAV_PILOT_SKILLS_DIR/observability-debugging/obs-query.sh"
```

### Basics for Mimir and Loki responses

```bash
# Extract metric values from a Mimir instant query
bash "$OBS" mimir "$QUERY" | jq '.data.result[] | {metric: .metric, value: .value[1]}'

# Extract log lines from a Loki response
bash "$OBS" loki "$SELECTOR" | jq -r '.data.result[].values[][1]'

# Parse JSON log lines (Loki returns them as strings, so double-decode)
bash "$OBS" loki "$SELECTOR" | jq -r '.data.result[].values[][1]' \
  | jq -s '.' | jq '.[] | fromjson | {time: .timestamp, msg: .message, level: .level}'
```

### Trace data in Tempo responses

Tempo returns OpenTelemetry-format traces with deeply nested spans. Key recipes:

```bash
# All spans with timing, slowest first
bash "$OBS" tempo-trace "$ENV" "$TRACE_ID" | jq '[.batches[].scopeSpans[].spans[] | {
  name,
  service: (.attributes // [] | map(select(.key == "service.name")) | .[0].value.stringValue // "unknown"),
  duration_ms: (((.endTimeUnixNano | tonumber) - (.startTimeUnixNano | tonumber)) / 1000000),
  status: (.status.code // "ok")
}] | sort_by(-.duration_ms)'

# The slowest span alone (bottleneck)
bash "$OBS" tempo-trace "$ENV" "$TRACE_ID" | jq '[.batches[].scopeSpans[].spans[] | {
  name,
  duration_ms: (((.endTimeUnixNano | tonumber) - (.startTimeUnixNano | tonumber)) / 1000000)
}] | sort_by(-.duration_ms) | .[0]'

# Error spans only
bash "$OBS" tempo-trace "$ENV" "$TRACE_ID" | jq '[.batches[].scopeSpans[].spans[] | select(.status.code == 2)] | .[] | {
  name,
  status_message: .status.message,
  duration_ms: (((.endTimeUnixNano | tonumber) - (.startTimeUnixNano | tonumber)) / 1000000)
}'

# Span attributes (HTTP details)
bash "$OBS" tempo-trace "$ENV" "$TRACE_ID" | jq '.batches[].scopeSpans[].spans[] | {
  name,
  http_method: (.attributes | map(select(.key == "http.method")) | .[0].value.stringValue),
  http_url: (.attributes | map(select(.key == "http.url")) | .[0].value.stringValue),
  http_status: (.attributes | map(select(.key == "http.status_code")) | .[0].value.intValue)
}'

# Call tree (parent → child)
bash "$OBS" tempo-trace "$ENV" "$TRACE_ID" | jq '[.batches[].scopeSpans[].spans[] | {
  span_id: .spanId,
  parent: .parentSpanId,
  name,
  duration_ms: (((.endTimeUnixNano | tonumber) - (.startTimeUnixNano | tonumber)) / 1000000)
}] | group_by(.parent) | .[] | {parent: .[0].parent, children: [.[] | {name, duration_ms}]}'
```

### Tempo search results with several traces

```bash
# Search results → summary table
bash "$OBS" tempo-search "$ENV" "{resource.k8s.cluster.name=\"$CLUSTER\" && resource.service.name=\"$APP\"}" --limit 20 | jq '.traces[] | {
  traceID,
  rootServiceName,
  rootTraceName,
  duration_ms: (.durationMs // 0),
  startTime: (.startTimeUnixNano / 1000000000 | todate)
}'

# How many search results carry an error
bash "$OBS" tempo-search "$ENV" "{resource.service.name=\"$APP\"}" \
  | jq '[.traces[] | select(.spanSets[].spans[].attributes[] | select(.key == "status" and .value.stringValue == "error"))] | length'
```

### Utility patterns

```bash
# Unix nanos → ISO timestamps
bash "$OBS" tempo-trace "$ENV" "$TRACE_ID" | jq '.batches[].scopeSpans[].spans[] | .startTimeUnixNano |= (tonumber / 1000000000 | todate)'

# Spans per service in one trace
bash "$OBS" tempo-trace "$ENV" "$TRACE_ID" | jq '[.batches[] | {service: .resource.attributes[] | select(.key == "service.name") | .value.stringValue, span_count: (.scopeSpans[].spans | length)}]'

# Spans longer than 100ms
bash "$OBS" tempo-trace "$ENV" "$TRACE_ID" | jq '[.batches[].scopeSpans[].spans[] | select(((.endTimeUnixNano | tonumber) - (.startTimeUnixNano | tonumber)) > 100000000)]'

# Group live kubectl logs by level
kubectl logs -n "$NAMESPACE" -l app="$APP" --since=5m | jq -s 'group_by(.level) | .[] | {level: .[0].level, count: length}'
```

## Common Diagnostic Queries

See [references/query-library.md](./references/query-library.md) for a comprehensive library of diagnostic PromQL, LogQL, and TraceQL queries organized by symptom.

## Grafana UI Shortcuts

- **Metrics:** `https://grafana.nav.cloud.nais.io/explore?orgId=1&left={"datasource":"mimir"}`
- **Logs:** `https://grafana.nav.cloud.nais.io/explore?orgId=1&left={"datasource":"loki"}`
- **Traces:** `https://grafana.nav.cloud.nais.io/explore?orgId=1&left={"datasource":"tempo"}`

In Grafana Explore:
1. Paste trace_id from logs → switch to Tempo → see full call chain
2. Click "Logs for this span" on a trace span → jump to Loki with time filter
3. Use "Split" view to correlate metrics and logs side-by-side

## When a query returns nothing useful

`obs-query.sh` tells these apart and names the fix, so read what it wrote on stderr before you touch the query. The table says the same thing, and covers the case where you queried with `curl` instead.

| What comes back | What it is | What fixes it |
|---|---|---|
| curl exits non-zero, or the request hangs until it times out | naisdevice is not connected. These hosts are only reachable through it. | `nais device connect`, then `nais device status` |
| `403` with `Resolved to a private IP, blocked by cplt` | The sandbox's DNS-rebinding guard. `mimir`, `loki` and the two `tempo` hosts resolve to private addresses, and cplt refuses a private-resolving host unless it is waived. | `nav-pilot sync --apply` and answer the sandbox question, or `cplt config set proxy.allow_private_domains mimir.nav.cloud.nais.io,loki.nav.cloud.nais.io,tempo.dev-gcp.nav.cloud.nais.io,tempo.prod-gcp.nav.cloud.nais.io`. `nav-pilot doctor` says whether it is in force. |
| `403` with `Domain not in allowlist` | The host is outside `proxy.allowed_domains`. Only the four hosts above are listed, so this is a fifth host, and adding one is a decision somebody makes on purpose. | Raise it in navikt/copilot rather than widening the allowlist locally |
| `401` | `X-Scope-OrgID` did not reach the server. There is no default org. | Send the header. A proxy that strips it is the other cause. |
| `200` with an empty `data.result` | Nothing refused anything. Either the window holds no data, or `--org` names the wrong half of the dataset. | Widen the window; then try the other `--org` value before rewriting the query |

The first four all look like "the network is down" from inside a `jq` pipeline, and they have four different fixes. With `curl` you have to add `--fail-with-body` and drop the `| jq` yourself, or the body naming which one never reaches you.

## Boundaries

### ✅ Always

- Start with the symptom and pick the right starting pillar
- Correlate across at least two pillars before concluding
- Include the time window in queries (avoid querying "all time")
- Query through `obs-query.sh`, which sends `X-Scope-OrgID` and the `User-Agent` for you
- Pass `--org nais` when the question is about the platform rather than about your own workloads
- Replace `$CLUSTER`, `$APP`, `$NAMESPACE`, `$ENV`, `$TRACE_ID` with actual values

### ⚠️ Ask First

- Querying production data for sensitive applications
- Running resource-intensive range queries spanning days
- Changing alert rules based on debugging findings

### 🚫 Never

- Share trace data containing PII outside the team
- Run unbounded queries without time limits (`start`/`end`)
- Do not assume a single trace represents the general case. Check rates first
- Delete or modify logs/traces (they're immutable)
