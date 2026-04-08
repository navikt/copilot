# Production Patterns

## Production Patterns from navikt

Based on 177+ repositories using observability setup:

### JVM Metrics Binders (navikt/ao-oppfolgingskontor)

```kotlin
import io.micrometer.core.instrument.binder.jvm.*

install(MicrometerMetrics) {
    registry = meterRegistry
    meterBinders = listOf(
        JvmMemoryMetrics(),        // Heap, non-heap, buffer pool metrics
        JvmGcMetrics(),            // GC pause time, count
        ProcessorMetrics(),        // CPU usage
        UptimeMetrics()            // Application uptime
    )
}
```

### Common Counter Patterns

```kotlin
// From dp-rapportering: Track business events
val eventsProcessed = Counter.builder("events_processed_total")
    .description("Total events processed")
    .tag("event_type", "rapportering_innsendt")
    .tag("status", "ok")
    .register(meterRegistry)

// From dp-rapportering: Track API errors
val apiErrors = Counter.builder("api_errors_total")
    .description("Total API errors")
    .tag("endpoint", "/api/rapporteringsperioder")
    .tag("error_type", "validation_error")
    .register(meterRegistry)
```

### Timer Patterns

```kotlin
// From dp-rapportering: Measure HTTP call duration
suspend fun <T> timedAction(navn: String, block: suspend () -> T): T {
    val (result, duration) = measureTimedValue {
        block()
    }
    Timer.builder("http_timer")
        .tag("navn", navn)
        .description("HTTP call duration")
        .register(meterRegistry)
        .record(duration.inWholeMilliseconds, MILLISECONDS)
    return result
}
```

## DORA Metrics Examples

Track DORA metrics for your team:

```kotlin
// Deployment frequency
val deployments = Counter.builder("deployments_total")
    .description("Total deployments")
    .tag("team", "myteam")
    .tag("environment", "production")
    .register(meterRegistry)

// Lead time for changes (commit to deploy)
val leadTime = Timer.builder("deployment_lead_time_seconds")
    .description("Time from commit to deployment")
    .tag("team", "myteam")
    .register(meterRegistry)

// Change failure rate
val failedDeployments = Counter.builder("deployments_failed_total")
    .description("Total failed deployments")
    .tag("team", "myteam")
    .register(meterRegistry)

// Time to restore service
val incidentResolutionTime = Timer.builder("incident_resolution_duration_seconds")
    .description("Time to resolve incidents")
    .tag("team", "myteam")
    .tag("severity", "critical")
    .register(meterRegistry)
```

Alert on DORA metrics:

```yaml
- alert: LowDeploymentFrequency
  expr: |
    sum(increase(deployments_total{team="myteam",environment="production"}[7d]))
    < 5
  description: "Only {{ $value }} deployments in last 7 days (target: >1/day)"
  severity: info

- alert: HighChangeFailureRate
  expr: |
    sum(rate(deployments_failed_total{team="myteam"}[7d]))
    / sum(rate(deployments_total{team="myteam"}[7d]))
    > 0.15
  description: "Change failure rate is {{ $value | humanizePercentage }} (target: <15%)"
  severity: warning
```

See https://dora.dev for benchmarks and best practices.
