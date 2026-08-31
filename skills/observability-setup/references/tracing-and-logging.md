# Trace context, log levels and log correlation

## Trace Context Propagation

OpenTelemetry automatically propagates trace context through:

- HTTP headers (W3C Trace Context)
- Kafka message headers
- Database connections

## Log Levels

```kotlin
logger.trace { "Detailed trace information" }
logger.debug { "Debug information" }
logger.info { "Informational message" }
logger.warn { "Warning message" }
logger.error(exception) { "Error occurred" }
```

## Logging Best Practices

1. **Log to stdout/stderr** (not files)
2. **Use structured logging** (JSON format)
3. **Include correlation IDs**
4. **Log at appropriate levels**
5. **Never log sensitive data** (PII, secrets)

```kotlin
// ✅ Good - structured with context
logger.info(
    "Payment processed",
    kv("transaction_id", txId),
    kv("amount", amount),
    kv("currency", "NOK")
)

// ❌ Bad - unstructured, hard to query
logger.info("Payment $txId processed for $amount NOK")
```

## Log Correlation with Traces

Nais auto-instrumentation automatically injects `trace_id` and `span_id` into MDC. If you use `LogstashEncoder` (standard for Nav apps), these fields are included in every log line — no manual code needed.

**Verify it works:** Find a trace in APM → click "View logs" → logs should appear correlated.

If correlation is missing, check that your logback config uses `LogstashEncoder` or includes `%X{trace_id}` in the pattern.
