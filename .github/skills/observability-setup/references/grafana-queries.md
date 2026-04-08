# Grafana Queries Reference

## Grafana Dashboard Panels

**Panel 1: Request Rate**

```promql
sum(rate(http_requests_total{app="my-app"}[5m])) by (endpoint)
```

**Panel 2: Error Rate**

```promql
sum(rate(http_requests_total{app="my-app",status=~"5.."}[5m]))
/ sum(rate(http_requests_total{app="my-app"}[5m])) * 100
```

**Panel 3: Response Time (p50, p95, p99)**

```promql
histogram_quantile(0.50, rate(http_request_duration_seconds_bucket{app="my-app"}[5m]))
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket{app="my-app"}[5m]))
histogram_quantile(0.99, rate(http_request_duration_seconds_bucket{app="my-app"}[5m]))
```

**Panel 4: Memory Usage**

```promql
container_memory_working_set_bytes{app="my-app"}
/ container_spec_memory_limit_bytes{app="my-app"} * 100
```

**Panel 5: Database Connections**

```promql
hikaricp_connections_active{app="my-app"}
hikaricp_connections_max{app="my-app"}
```

**Panel 6: Kafka Consumer Lag**

```promql
kafka_consumer_lag{app="my-app"}
```

## Loki Query Examples

View logs in Grafana Loki Explorer:

```logql
# All logs from your app
{app="my-app", namespace="myteam"}

# Only errors
{app="my-app"} |= "ERROR"

# JSON logs with specific field
{app="my-app"} | json | event_type="payment_processed"

# Logs correlated with trace
{app="my-app"} | json | trace_id="abc123def456"

# Count errors per minute
sum(rate({app="my-app"} |= "ERROR" [1m])) by (pod)
```

## Tempo Trace Search

View traces in Grafana Tempo:

1. Open Grafana → Explore
2. Select Tempo data source
3. Query by:
   - Service name: `my-app`
   - Operation: `getUsersRequest`
   - Duration: `> 1s`
   - Status: `error`

Or link from logs by clicking trace_id in Loki.
