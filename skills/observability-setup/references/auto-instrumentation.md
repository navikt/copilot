# What Nais auto-instrumentation does

## What Auto-Instrumentation Provides

**Automatic Tracing For**:

- HTTP server requests (Ktor, Spring Boot)
- HTTP client requests (Ktor client, OkHttp)
- Database queries (JDBC, PostgreSQL driver)
- Kafka producer/consumer
- Redis/Valkey operations

**Automatic Metrics**:

- JVM metrics (heap, GC, threads)
- HTTP request metrics
- Database connection pool metrics

**No Code Changes Required** for basic instrumentation!

## Manual Instrumentation (Advanced)

For custom spans:

```yaml
spec:
  observability:
    autoInstrumentation:
      enabled: true
      runtime: sdk # Enables SDK without auto-instrumentation
```

Then use OpenTelemetry SDK in code (as shown earlier).

## Sensitive Data Masking

Nais auto-masks these fields in traces:

- `db.statement` (SQL queries)
- `messaging.kafka.message.key`
- `url.path` (Norwegian personal numbers)

**Always verify** your application traces in Grafana Tempo to ensure no sensitive data is exposed!

## Noisy Traces (Filtered)

Nais automatically filters these paths from tracing:

- `*/isAlive`
- `*/isReady`
- `*/prometheus`
- `*/metrics`
- `*/actuator/*`
- `*/internal/health*`
- `*/internal/status*`
