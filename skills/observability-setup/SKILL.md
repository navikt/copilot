---
name: observability-setup
description: Sett opp Prometheus-metrikker, OpenTelemetry-tracing og health check-endepunkter for Nais-applikasjoner
license: MIT
compatibility: Application deployed on Nais
metadata:
  domain: observability
  tags: prometheus opentelemetry health metrics
---

# Observability Setup Skill

This skill provides patterns for setting up observability in Nais applications.

## Required Health Endpoints

```kotlin
import io.ktor.server.application.*
import io.ktor.server.response.*
import io.ktor.server.routing.*
import io.ktor.http.*

fun Application.configureHealthEndpoints(
    dataSource: HikariDataSource,
    kafkaProducer: KafkaProducer<String, String>
) {
    routing {
        get("/isalive") {
            call.respondText("Alive", ContentType.Text.Plain)
        }

        get("/isready") {
            val databaseHealthy = checkDatabase(dataSource)
            val kafkaHealthy = checkKafka(kafkaProducer)

            if (databaseHealthy && kafkaHealthy) {
                call.respondText("Ready", ContentType.Text.Plain)
            } else {
                call.respondText(
                    "Not ready",
                    ContentType.Text.Plain,
                    HttpStatusCode.ServiceUnavailable
                )
            }
        }
    }
}

fun checkDatabase(dataSource: HikariDataSource): Boolean {
    return try {
        dataSource.connection.use { it.isValid(1) }
    } catch (e: Exception) {
        false
    }
}

fun checkKafka(producer: KafkaProducer<String, String>): Boolean {
    return try {
        producer.partitionsFor("health-check-topic").isNotEmpty()
    } catch (e: Exception) {
        false
    }
}
```

## Prometheus Metrics Setup

```kotlin
import io.micrometer.core.instrument.Clock
import io.micrometer.core.instrument.binder.jvm.*
import io.micrometer.prometheus.PrometheusConfig
import io.micrometer.prometheus.PrometheusMeterRegistry
import io.prometheus.client.CollectorRegistry
import io.ktor.server.metrics.micrometer.*
import io.ktor.server.response.*
import io.ktor.http.*

val meterRegistry = PrometheusMeterRegistry(
    PrometheusConfig.DEFAULT,
    CollectorRegistry.defaultRegistry,
    Clock.SYSTEM
)

fun Application.configureMetrics() {
    install(MicrometerMetrics) {
        registry = meterRegistry
        // Production pattern from navikt/ao-oppfolgingskontor
        meterBinders = listOf(
            JvmMemoryMetrics(),        // Heap, non-heap memory
            JvmGcMetrics(),            // Garbage collection
            ProcessorMetrics(),        // CPU usage
            UptimeMetrics()            // Application uptime
        )
    }

    routing {
        get("/metrics") {
            call.respondText(
                meterRegistry.scrape(),
                ContentType.parse("text/plain; version=0.0.4")
            )
        }
    }
}
```

## Business Metrics

```kotlin
import io.micrometer.core.instrument.Counter
import io.micrometer.core.instrument.Timer

class UserService(private val meterRegistry: PrometheusMeterRegistry) {
    private val userCreatedCounter = Counter.builder("users_created_total")
        .description("Total users created")
        .register(meterRegistry)

    private val userCreationTimer = Timer.builder("user_creation_duration_seconds")
        .description("User creation duration")
        .register(meterRegistry)

    fun createUser(user: User) {
        userCreationTimer.record {
            repository.save(user)
        }
        userCreatedCounter.increment()
    }
}
```

## OpenTelemetry Tracing

Nais enables OpenTelemetry auto-instrumentation by default. For manual spans:

```kotlin
import io.opentelemetry.api.GlobalOpenTelemetry
import io.opentelemetry.api.trace.Span
import io.opentelemetry.api.trace.StatusCode

val tracer = GlobalOpenTelemetry.getTracer("my-app")

fun processPayment(paymentId: String) {
    val span = tracer.spanBuilder("processPayment")
        .setAttribute("payment.id", paymentId)
        .startSpan()

    try {
        // Business logic
        val payment = repository.findPayment(paymentId)
        span.setAttribute("payment.amount", payment.amount)

        processPaymentInternal(payment)
        span.setStatus(StatusCode.OK)
    } catch (e: Exception) {
        span.setStatus(StatusCode.ERROR, "Payment processing failed")
        span.recordException(e)
        throw e
    } finally {
        span.end()
    }
}
```

## Structured Logging

```kotlin
import mu.KotlinLogging
import net.logstash.logback.argument.StructuredArguments.kv

private val logger = KotlinLogging.logger {}

fun processOrder(orderId: String) {
    logger.info(
        "Processing order",
        kv("order_id", orderId),
        kv("timestamp", LocalDateTime.now())
    )

    try {
        orderService.process(orderId)

        logger.info(
            "Order processed successfully",
            kv("order_id", orderId)
        )
    } catch (e: Exception) {
        logger.error(
            "Order processing failed",
            kv("order_id", orderId),
            kv("error", e.message),
            e
        )
        throw e
    }
}
```

## Nais Manifest

```yaml
apiVersion: nais.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: myteam
  labels:
    team: myteam
spec:
  image: ghcr.io/navikt/my-app:latest
  port: 8080

  # Health checks
  liveness:
    path: /isalive
    initialDelay: 10
    timeout: 1
    periodSeconds: 10
    failureThreshold: 3

  readiness:
    path: /isready
    initialDelay: 10
    timeout: 1
    periodSeconds: 10
    failureThreshold: 3

  # Prometheus scraping
  prometheus:
    enabled: true
    path: /metrics

  # OpenTelemetry auto-instrumentation
  observability:
    autoInstrumentation:
      enabled: true
      runtime: java # Instruments Ktor, JDBC, Kafka automatically
    logging:
      destinations:
        - id: loki # Automatic Loki shipping
        - id: team-logs # Optional: private team logs

  # Resources (for metrics alerting)
  resources:
    limits:
      memory: 512Mi
    requests:
      cpu: 50m
      memory: 256Mi
```

## Alert Configuration

Create `.nais/alert.yml`:

```yaml
apiVersion: nais.io/v1
kind: Alert
metadata:
  name: my-app-alerts
  namespace: myteam
  labels:
    team: myteam
spec:
  receivers:
    slack:
      channel: "#team-alerts"
      prependText: "@here "
  alerts:
    - alert: HighErrorRate
      expr: |
        (sum(rate(http_requests_total{app="my-app",status=~"5.."}[5m]))
        / sum(rate(http_requests_total{app="my-app"}[5m]))) > 0.05
      for: 5m
      description: "Error rate is {{ $value | humanizePercentage }}"
      action: "Check logs in Grafana Loki"
      documentation: https://teamdocs/runbooks/high-error-rate
      sla: "Respond within 15 minutes"
      severity: critical

    - alert: HighResponseTime
      expr: |
        histogram_quantile(0.95,
          rate(http_request_duration_seconds_bucket{app="my-app"}[5m])
        ) > 1
      for: 10m
      description: "95th percentile response time is {{ $value }}s"
      action: "Check Tempo traces for slow requests"
      severity: warning

    - alert: PodCrashLooping
      expr: |
        rate(kube_pod_container_status_restarts_total{
          pod=~"my-app-.*"
        }[15m]) > 0
      for: 5m
      description: "Pod {{ $labels.pod }} is crash looping"
      action: "Check logs: kubectl logs {{ $labels.pod }}"
      severity: critical

    - alert: HighMemoryUsage
      expr: |
        (container_memory_working_set_bytes{app="my-app"}
        / container_spec_memory_limit_bytes{app="my-app"}) > 0.9
      for: 10m
      description: "Memory usage is {{ $value | humanizePercentage }}"
      action: "Check for memory leaks, increase limits if needed"
      severity: warning
```

## Complete Example

```kotlin
import io.ktor.server.application.*
import io.ktor.server.engine.*
import io.ktor.server.netty.*
import io.micrometer.core.instrument.Timer
import io.opentelemetry.api.GlobalOpenTelemetry
import io.opentelemetry.api.trace.StatusCode

fun main() {
    val env = Environment.from(System.getenv())
    val dataSource = createDataSource(env.databaseUrl)

    // Run database migrations
    runMigrations(dataSource)

    // Setup metrics
    val meterRegistry = setupMetrics()

    embeddedServer(Netty, port = 8080) {
        configureHealthEndpoints(dataSource)
        configureMetrics(meterRegistry)
        configureRouting(dataSource, meterRegistry)
    }.start(wait = true)
}

fun Application.configureRouting(
    dataSource: HikariDataSource,
    meterRegistry: PrometheusMeterRegistry
) {
    val tracer = GlobalOpenTelemetry.getTracer("my-app")

    routing {
        get("/api/users") {
            val requestTimer = Timer.sample()
            val requestCounter = meterRegistry.counter(
                "http_requests_total",
                "method", "GET",
                "endpoint", "/api/users"
            )

            val span = tracer.spanBuilder("getUsersRequest")
                .setAttribute("http.method", "GET")
                .setAttribute("http.route", "/api/users")
                .startSpan()

            try {
                val users = userRepository.findAll()
                span.setAttribute("user.count", users.size.toLong())
                span.setStatus(StatusCode.OK)

                requestCounter.increment()
                requestTimer.stop(meterRegistry.timer(
                    "http_request_duration_seconds",
                    "method", "GET",
                    "endpoint", "/api/users",
                    "status", "200"
                ))

                call.respond(users)
            } catch (e: Exception) {
                span.setStatus(StatusCode.ERROR, "Failed to get users")
                span.recordException(e)

                meterRegistry.counter(
                    "http_requests_total",
                    "method", "GET",
                    "endpoint", "/api/users",
                    "status", "500"
                ).increment()

                // trace_id and span_id are auto-injected into MDC by OTel agent
                logger.error("Failed to get users", e)

                throw e
            } finally {
                span.end()
            }
        }
    }
}
```

## Grafana & Loki & Tempo Queries

See [references/grafana-queries.md](references/grafana-queries.md) for PromQL dashboard panels, LogQL query examples, and Tempo trace search patterns.

## Monitoring Checklist

- [ ] `/isalive` endpoint implemented
- [ ] `/isready` endpoint with dependency checks (database, Kafka)
- [ ] `/metrics` endpoint exposing Prometheus metrics
- [ ] Health checks configured in Nais manifest
- [ ] Business metrics instrumented (counters, timers, gauges)
- [ ] Verify trace_id appears in logs (auto-injected by OTel agent via MDC)
- [ ] OpenTelemetry auto-instrumentation enabled in Nais manifest
- [ ] Alert rules created in `.nais/alert.yml`
- [ ] Slack channel configured for alerts
- [ ] Grafana dashboard created
- [ ] No sensitive data in logs or metrics (verify in Grafana)
- [ ] High-cardinality labels avoided (no user_ids, transaction_ids)

## Production Patterns & DORA Metrics

See [references/production-patterns.md](references/production-patterns.md) for real-world patterns from navikt repositories and DORA metric implementation examples.
