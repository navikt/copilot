# Diagnostic Query Library

Copy-paste query recipes organized by symptom. Replace `$APP` and `$NAMESPACE` with your values.

## PromQL — Mimir Metrics

### Availability & Errors

```promql
# Error rate (5xx per second)
sum(rate(http_server_requests_seconds_count{app="$APP", status=~"5.."}[5m])) by (uri)

# Success ratio (%)
sum(rate(http_server_requests_seconds_count{app="$APP", status=~"2.."}[5m]))
/ sum(rate(http_server_requests_seconds_count{app="$APP"}[5m])) * 100

# Error budget burn rate (SLO 99.9%)
1 - (sum(rate(http_server_requests_seconds_count{app="$APP", status=~"5.."}[1h]))
/ sum(rate(http_server_requests_seconds_count{app="$APP"}[1h])))
```

### Latency

```promql
# p50, p95, p99 response time
histogram_quantile(0.50, sum(rate(http_server_requests_seconds_bucket{app="$APP"}[5m])) by (le))
histogram_quantile(0.95, sum(rate(http_server_requests_seconds_bucket{app="$APP"}[5m])) by (le))
histogram_quantile(0.99, sum(rate(http_server_requests_seconds_bucket{app="$APP"}[5m])) by (le))

# Latency by endpoint
histogram_quantile(0.95, sum(rate(http_server_requests_seconds_bucket{app="$APP"}[5m])) by (le, uri))

# Apdex score (threshold 0.5s, toleration 2s)
(sum(rate(http_server_requests_seconds_bucket{app="$APP", le="0.5"}[5m]))
+ sum(rate(http_server_requests_seconds_bucket{app="$APP", le="2"}[5m]))) / 2
/ sum(rate(http_server_requests_seconds_count{app="$APP"}[5m]))
```

### Saturation

```promql
# Memory usage %
container_memory_working_set_bytes{app="$APP"}
/ container_spec_memory_limit_bytes{app="$APP"} * 100

# CPU usage vs request
rate(container_cpu_usage_seconds_total{app="$APP"}[5m])
/ container_spec_cpu_quota{app="$APP"} * container_spec_cpu_period{app="$APP"} * 100

# CPU throttling %
rate(container_cpu_cfs_throttled_periods_total{app="$APP"}[5m])
/ rate(container_cpu_cfs_periods_total{app="$APP"}[5m]) * 100

# JVM heap usage (Kotlin/Java apps)
jvm_memory_used_bytes{app="$APP", area="heap"}
/ jvm_memory_max_bytes{app="$APP", area="heap"} * 100

# Database connection pool saturation
hikaricp_connections_active{app="$APP"}
/ hikaricp_connections_max{app="$APP"} * 100
```

### Kafka

```promql
# Consumer lag
kafka_consumer_lag{app="$APP"}

# Consumer rate
sum(rate(kafka_consumer_records_consumed_total{app="$APP"}[5m]))

# Producer error rate
sum(rate(kafka_producer_record_error_total{app="$APP"}[5m]))
```

### Restarts & Pod Health

```promql
# Restart count in last hour
increase(kube_pod_container_status_restarts_total{container="$APP"}[1h])

# Pods not ready
kube_pod_status_ready{app="$APP", condition="false"}

# OOMKilled events
kube_pod_container_status_last_terminated_reason{container="$APP", reason="OOMKilled"}
```

## LogQL — Loki Logs

> **Labels vs fields:** Stream selector labels (`{}`) are indexed and fast. Structured metadata
> (`k8s_pod_name`, `k8s_node_name`, `k8s_container_name`, `detected_level`) can be filtered with `|`
> without parsing. Log line fields (`level`, `message`, `trace_id`) require `| json` parsing — slower.
>
> **Available labels:** `service_name`, `service_namespace`, `app_name`, `deployment_environment`,
> `env`, `k8s_cluster_name`, `kind` (`log`/`event`/`exception`/`measurement`), `collector_name`
>
> **Structured metadata:** `k8s_pod_name`, `k8s_node_name`, `k8s_container_name`, `detected_level`

### Error Investigation

```logql
# All errors (fast: label + structured metadata)
{service_name="$APP"} | detected_level="error"

# All errors (with parsed fields for detail)
{service_name="$APP"} | json | level="error"

# Errors with stack traces
{service_name="$APP"} | json | level="error" | line_format "{{.message}}\n{{.stack_trace}}"

# Count errors per message
sum by (message) (count_over_time({service_name="$APP"} | json | level="error" [15m]))

# Errors from a specific pod
{service_name="$APP"} | k8s_pod_name="$POD_NAME" | json | level="error"
```

### Request Debugging

```logql
# Find request by trace_id
{service_name="$APP"} |= "$TRACE_ID"

# Slow requests (parse duration from structured log)
{service_name="$APP"} | json | duration > 2000

# Specific endpoint errors
{service_name="$APP"} | json | uri="/api/something" | status >= 500

# Auth failures
{service_name="$APP"} | json | message=~".*unauthorized.*|.*forbidden.*|.*401.*|.*403.*"
```

### Dependency Failures

```logql
# Downstream HTTP errors
{service_name="$APP"} | json | message=~".*connection refused.*|.*timeout.*|.*ECONNRESET.*"

# Database errors
{service_name="$APP"} | json | message=~".*SQL.*|.*database.*|.*connection pool.*"

# Token/Auth errors to other services
{service_name="$APP"} | json | message=~".*token.*expired.*|.*invalid_grant.*|.*OIDC.*"
```

### Rate & Volume

```logql
# Log volume per level (useful for detecting log storms)
sum by (level) (rate({service_name="$APP"} | json [5m]))

# Error rate over time
sum(rate({service_name="$APP"} | json | level="error" [5m]))
```

## TraceQL — Tempo Traces

### Finding Traces

```traceql
# All traces for app
{resource.service.name = "$APP"}

# Slow traces (>2s)
{resource.service.name = "$APP" && duration > 2s}

# Error traces
{resource.service.name = "$APP" && status = error}

# Traces hitting specific endpoint
{resource.service.name = "$APP" && name = "GET /api/something"}

# Traces with specific HTTP status
{resource.service.name = "$APP" && span.http.status_code >= 500}
```

### Cross-Service Investigation

```traceql
# Traces passing through two services
{resource.service.name = "$APP"} >> {resource.service.name = "$DOWNSTREAM_APP"}

# Slow spans in downstream service
{resource.service.name = "$APP"} >> {resource.service.name = "$DOWNSTREAM_APP" && duration > 1s}

# Database spans
{resource.service.name = "$APP" && span.db.system = "postgresql"}

# External HTTP calls
{resource.service.name = "$APP" && kind = client && span.http.url != ""}
```

### Structural Queries

```traceql
# Root spans only (entry point)
{resource.service.name = "$APP" && nestedSetParent = -1}

# Leaf spans (no children — usually the actual work)
{resource.service.name = "$APP" && nestedSetLeft = nestedSetRight - 1}
```
