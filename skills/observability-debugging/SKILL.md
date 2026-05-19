---
name: observability-debugging
description: Feilsøk produksjonsproblemer med Mimir-metrikker, Loki-logger og Tempo-traces — strukturerte debugging-workflows for Nav-utviklere
license: MIT
compatibility: Application deployed on Nais
metadata:
  domain: observability
  tags: debugging mimir loki tempo prometheus traces logs kubectl nais
---

# Observability Debugging — Three-Pillar Diagnostics

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

### Mimir — Metrics API

```bash
# Query metrics directly (replace $QUERY with PromQL)
curl -s -H "User-Agent: nav-pilot/observability-debugging" -H "X-Scope-OrgID: tenant" \
  "https://mimir.nav.cloud.nais.io/prometheus/api/v1/query?query=$QUERY" | jq .

# Example: Error rate for an app
curl -s -H "User-Agent: nav-pilot/observability-debugging" -H "X-Scope-OrgID: tenant" \
  "https://mimir.nav.cloud.nais.io/prometheus/api/v1/query?query=sum(rate(http_server_requests_seconds_count{k8s_cluster_name=\"$CLUSTER\", app=\"$APP\",status=~\"5..\"}[5m]))" | jq .

# Range query (last hour)
curl -s -H "User-Agent: nav-pilot/observability-debugging" -H "X-Scope-OrgID: tenant" \
  "https://mimir.nav.cloud.nais.io/prometheus/api/v1/query_range?query=$QUERY&start=$(date -d '1 hour ago' +%s)&end=$(date +%s)&step=60" | jq .
```

### Loki — Log Queries

> **Labels (indexed, fast):** `service_name`, `service_namespace`, `app_name`, `env`, `deployment_environment`, `k8s_cluster_name`, `kind` (log/event/exception/measurement)
> **Structured metadata (fast filter with `|`):** `k8s_pod_name`, `k8s_node_name`, `k8s_container_name`, `detected_level`
> **Log line fields (require `| json`, slower):** `level`, `message`, `trace_id`, `span_id`, `logger_name`, `thread_name`
> Always narrow with labels first, then filter metadata/fields.

```bash
# Query via Loki API (dev environment)
curl -s -H "User-Agent: nav-pilot/observability-debugging" -H "X-Scope-OrgID: tenant" \
  "https://loki.nav.cloud.nais.io/loki/api/v1/query_range" \
  --data-urlencode "query={k8s_cluster_name=~\"dev.*\",service_name=\"$APP\"} |= \"ERROR\"" \
  --data-urlencode "limit=50" | jq .

# Prod environment
curl -s -H "User-Agent: nav-pilot/observability-debugging" -H "X-Scope-OrgID: tenant" \
  "https://loki.nav.cloud.nais.io/loki/api/v1/query_range" \
  --data-urlencode "query={k8s_cluster_name=~\"prod.*\",service_name=\"$APP\"} | json | level=\"error\"" | jq .
```

### Tempo — Trace Search

> **Gotcha:** Tempo search may return unrelated traces when your service has no spans. Always verify `rootServiceName` matches your app.

```bash
# Search traces by service name (dev)
curl -s -H "User-Agent: nav-pilot/observability-debugging" -H "X-Scope-OrgID: tenant" \
  "https://tempo.dev-gcp.nav.cloud.nais.io/api/search" \
  --data-urlencode "q={resource.service.name=\"$APP\"}" \
  --data-urlencode "limit=20" | jq '.traces[] | select(.rootServiceName == "$APP")'

# Find slow traces (>2s)
curl -s -H "User-Agent: nav-pilot/observability-debugging" -H "X-Scope-OrgID: tenant" \
  "https://tempo.dev-gcp.nav.cloud.nais.io/api/search?q={resource.service.name=\"$APP\" && duration>2s}" | jq .

# Get trace by ID
curl -s -H "User-Agent: nav-pilot/observability-debugging" -H "X-Scope-OrgID: tenant" \
  "https://tempo.dev-gcp.nav.cloud.nais.io/api/traces/$TRACE_ID" | jq .
```

## Correlation Patterns

### Pattern 1: Error spike → Root cause

```bash
# 1. Confirm error rate in Mimir
curl -s -H "User-Agent: nav-pilot/observability-debugging" -H "X-Scope-OrgID: tenant" \
  "https://mimir.nav.cloud.nais.io/prometheus/api/v1/query?query=sum(rate(http_server_requests_seconds_count{k8s_cluster_name=\"$CLUSTER\",app=\"$APP\",status=~\"5..\"}[5m]))by(uri)" | jq .

# 2. Find error logs in Loki (last 15 min)
curl -s -H "User-Agent: nav-pilot/observability-debugging" -H "X-Scope-OrgID: tenant" \
  "https://loki.nav.cloud.nais.io/loki/api/v1/query_range" \
  --data-urlencode "query={k8s_cluster_name=\"$CLUSTER\",service_name=\"$APP\"} | json | level=\"error\"" \
  --data-urlencode "limit=20" \
  --data-urlencode "start=$(date -d '15 minutes ago' +%s)000000000" | jq '.data.result[].values[][1]' | head -10

# 3. Extract trace_id from error log, then look up in Tempo
curl -s -H "User-Agent: nav-pilot/observability-debugging" -H "X-Scope-OrgID: tenant" \
  "https://tempo.prod-gcp.nav.cloud.nais.io/api/traces/$TRACE_ID" | jq '.batches[].scopeSpans[].spans[] | {name, status, duration: (.endTimeUnixNano - .startTimeUnixNano) / 1000000}'
```

### Pattern 2: Slow endpoint → Bottleneck

```bash
# 1. Find slow endpoints in Mimir (p95 latency)
curl -s -H "User-Agent: nav-pilot/observability-debugging" -H "X-Scope-OrgID: tenant" \
  "https://mimir.nav.cloud.nais.io/prometheus/api/v1/query?query=histogram_quantile(0.95,sum(rate(http_server_requests_seconds_bucket{k8s_cluster_name=\"$CLUSTER\",app=\"$APP\"}[5m]))by(le,uri))" | jq '.data.result[] | {endpoint: .metric.uri, p95_seconds: .value[1]}'

# 2. Find slow traces in Tempo
curl -s -H "User-Agent: nav-pilot/observability-debugging" -H "X-Scope-OrgID: tenant" \
  "https://tempo.prod-gcp.nav.cloud.nais.io/api/search?q={resource.service.name=\"$APP\" && duration>1s}&limit=10" | jq .

# 3. Inspect trace for bottleneck span
curl -s -H "User-Agent: nav-pilot/observability-debugging" -H "X-Scope-OrgID: tenant" \
  "https://tempo.prod-gcp.nav.cloud.nais.io/api/traces/$TRACE_ID" | jq '.batches[].scopeSpans[].spans[] | select((.endTimeUnixNano - .startTimeUnixNano) > 500000000) | {name, duration_ms: (.endTimeUnixNano - .startTimeUnixNano) / 1000000}'
```

### Pattern 3: Pod issues → Resource context

```bash
# 1. Check pod status
kubectl get pods -n $NAMESPACE -l app=$APP

# 2. Memory usage (% of limit)
curl -s -H "User-Agent: nav-pilot/observability-debugging" -H "X-Scope-OrgID: tenant" \
  "https://mimir.nav.cloud.nais.io/prometheus/api/v1/query?query=container_memory_working_set_bytes{k8s_cluster_name=\"$CLUSTER\",app=\"$APP\"}/container_spec_memory_limit_bytes{k8s_cluster_name=\"$CLUSTER\",app=\"$APP\"}*100" | jq '.data.result[] | {pod: .metric.pod, memory_pct: .value[1]}'

# 3. CPU throttling
curl -s -H "User-Agent: nav-pilot/observability-debugging" -H "X-Scope-OrgID: tenant" \
  "https://mimir.nav.cloud.nais.io/prometheus/api/v1/query?query=rate(container_cpu_cfs_throttled_periods_total{k8s_cluster_name=\"$CLUSTER\",app=\"$APP\"}[5m])/rate(container_cpu_cfs_periods_total{k8s_cluster_name=\"$CLUSTER\",app=\"$APP\"}[5m])*100" | jq .

# 4. Recent OOM kills in logs
kubectl get events -n $NAMESPACE --field-selector reason=OOMKilling --sort-by='.lastTimestamp' | tail -5
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

All API responses return JSON. Master these jq patterns to extract signal from noise.

### Basics — Mimir & Loki Responses

```bash
# Extract metric values from Mimir instant query
curl -s ... | jq '.data.result[] | {metric: .metric, value: .value[1]}'

# Extract log lines from Loki response
curl -s ... | jq -r '.data.result[].values[][1]'

# Parse JSON log lines (Loki returns strings — double-decode)
curl -s ... | jq -r '.data.result[].values[][1]' | jq -s '.' | jq '.[] | fromjson | {time: .timestamp, msg: .message, level: .level}'
```

### Trace Data — Tempo Responses

Tempo returns OpenTelemetry-format traces with deeply nested spans. Key jq recipes:

```bash
# List all spans with timing (flat overview)
curl -s -H "User-Agent: nav-pilot/observability-debugging" -H "X-Scope-OrgID: tenant" \
  "https://tempo.$env.nav.cloud.nais.io/api/traces/$TRACE_ID" | \
  jq '[.batches[].scopeSpans[].spans[] | {
    name,
    service: (.attributes // [] | map(select(.key == "service.name")) | .[0].value.stringValue // "unknown"),
    duration_ms: ((.endTimeUnixNano | tonumber) - (.startTimeUnixNano | tonumber)) / 1000000,
    status: (.status.code // "ok")
  }] | sort_by(-.duration_ms)'

# Find the slowest span (bottleneck)
curl -s ... | jq '[.batches[].scopeSpans[].spans[] | {
  name,
  duration_ms: ((.endTimeUnixNano | tonumber) - (.startTimeUnixNano | tonumber)) / 1000000
}] | sort_by(-.duration_ms) | .[0]'

# Show only error spans
curl -s ... | jq '[.batches[].scopeSpans[].spans[] | select(.status.code == 2)] | .[] | {
  name,
  status_message: .status.message,
  duration_ms: ((.endTimeUnixNano | tonumber) - (.startTimeUnixNano | tonumber)) / 1000000
}'

# Extract span attributes (e.g., HTTP details)
curl -s ... | jq '.batches[].scopeSpans[].spans[] | {
  name,
  http_method: (.attributes | map(select(.key == "http.method")) | .[0].value.stringValue),
  http_url: (.attributes | map(select(.key == "http.url")) | .[0].value.stringValue),
  http_status: (.attributes | map(select(.key == "http.status_code")) | .[0].value.intValue)
}'

# Build a call tree (parent → child relationships)
curl -s ... | jq '[.batches[].scopeSpans[].spans[] | {
  span_id: .spanId,
  parent: .parentSpanId,
  name,
  duration_ms: ((.endTimeUnixNano | tonumber) - (.startTimeUnixNano | tonumber)) / 1000000
}] | group_by(.parent) | .[] | {parent: .[0].parent, children: [.[] | {name, duration_ms}]}'
```

### Tempo Search Results — Processing Multiple Traces

```bash
# Search results → summary table
curl -s -H "User-Agent: nav-pilot/observability-debugging" -H "X-Scope-OrgID: tenant" \
  "https://tempo.$env.nav.cloud.nais.io/api/search?q={resource.k8s.cluster.name=\"$CLUSTER\" && resource.service.name=\"$APP\"}&limit=20" | \
  jq '.traces[] | {
    traceID,
    rootServiceName,
    rootTraceName,
    duration_ms: (.durationMs // 0),
    startTime: (.startTimeUnixNano / 1000000000 | todate)
  }'

# Find traces with errors in search results
curl -s ... | jq '[.traces[] | select(.spanSets[].spans[].attributes[] | select(.key == "status" and .value.stringValue == "error"))] | length'
```

### Utility Patterns

```bash
# Pretty-print with timestamp conversion (Unix nanos → ISO)
jq '.batches[].scopeSpans[].spans[] | .startTimeUnixNano |= (tonumber / 1000000000 | todate)'

# Count spans per service in a trace
curl -s ... | jq '[.batches[] | {service: .resource.attributes[] | select(.key == "service.name") | .value.stringValue, span_count: (.scopeSpans[].spans | length)}]'

# Filter spans by minimum duration (e.g., >100ms)
curl -s ... | jq '[.batches[].scopeSpans[].spans[] | select(((.endTimeUnixNano | tonumber) - (.startTimeUnixNano | tonumber)) > 100000000)]'

# Pipe Loki JSON logs through jq for analysis
kubectl logs -n $NAMESPACE -l app=$APP --since=5m | jq -s 'group_by(.level) | .[] | {level: .[0].level, count: length}'
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

## Boundaries

### ✅ Always

- Start with the symptom and pick the right starting pillar
- Correlate across at least two pillars before concluding
- Include the time window in queries (avoid querying "all time")
- Use `X-Scope-OrgID: tenant` and `User-Agent: nav-pilot/observability-debugging` headers for all API calls
- Replace `$CLUSTER`, `$APP`, `$NAMESPACE`, `$env` with actual values

### ⚠️ Ask First

- Querying production data for sensitive applications
- Running resource-intensive range queries spanning days
- Changing alert rules based on debugging findings

### 🚫 Never

- Share trace data containing PII outside the team
- Run unbounded queries without time limits (`start`/`end`)
- Assume a single trace represents the general case — check rates first
- Delete or modify logs/traces (they're immutable)
