# Metric naming, labels and metric types

## Prometheus Standards (OpenMetrics)

Follow Nais/Prometheus naming conventions:

```kotlin
// ✅ Good - snake_case with unit suffix
val requestDuration = Timer.builder("http_request_duration_seconds")
    .description("HTTP request duration")
    .tag("method", "GET")
    .tag("endpoint", "/api/users")
    .tag("status", "200")
    .register(meterRegistry)

// ✅ Good - counter with _total suffix
val eventsProcessed = Counter.builder("events_processed_total")
    .description("Total events processed")
    .tag("event_type", "user_created")
    .tag("status", "success")
    .register(meterRegistry)

// ❌ Bad - camelCase, no unit
val requestDuration = Timer.builder("requestDuration")

// ❌ Bad - missing _total suffix
val eventsProcessed = Counter.builder("events_processed")
```

## Label Best Practices

**⚠️ CRITICAL: Avoid high-cardinality labels**

```kotlin
// ✅ Good - bounded cardinality
.tag("method", "GET")           // ~10 values
.tag("status", "200")           // ~60 values
.tag("event_type", "payment")   // ~50 values

// ❌ Bad - unbounded cardinality (creates infinite time series)
.tag("user_id", userId)         // Millions of values
.tag("transaction_id", txId)    // Millions of values
.tag("email", email)            // Millions of values
```

Each unique combination of labels creates a new time series. High cardinality = memory exhaustion in Prometheus.

## Metric types

### Gauge (Current Value)

```kotlin
val activeConnections = Gauge.builder("db_connections_active") {
    dataSource.hikariPoolMXBean.activeConnections.toDouble()
}
    .description("Active database connections")
    .register(meterRegistry)
```

### Histogram (Distribution)

```kotlin
val responseSize = DistributionSummary.builder("http_response_size_bytes")
    .description("HTTP response size in bytes")
    .baseUnit("bytes")
    .register(meterRegistry)

responseSize.record(responseBytes.size.toDouble())
```

## Queue depth as a business gauge

```kotlin
val queueSize = Gauge.builder("event_queue_size") {
    eventQueue.size.toDouble()
}
    .description("Current event queue size")
    .register(meterRegistry)
```
