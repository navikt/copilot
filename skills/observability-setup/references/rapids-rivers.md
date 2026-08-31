# Rapids & Rivers observability

## Event Metrics

```kotlin
class PaymentRiver(
    rapidsConnection: RapidsConnection,
    private val meterRegistry: PrometheusMeterRegistry
) : River.PacketListener {

    private val eventsReceived = Counter.builder("rapids_events_received_total")
        .description("Total events received")
        .tag("event_type", "payment_created")
        .register(meterRegistry)

    private val eventsProcessed = Counter.builder("rapids_events_processed_total")
        .description("Total events processed successfully")
        .tag("event_type", "payment_created")
        .register(meterRegistry)

    private val eventsFailed = Counter.builder("rapids_events_failed_total")
        .description("Total events that failed processing")
        .tag("event_type", "payment_created")
        .register(meterRegistry)

    private val processingDuration = Timer.builder("rapids_event_processing_duration_seconds")
        .description("Event processing duration")
        .tag("event_type", "payment_created")
        .register(meterRegistry)

    init {
        River(rapidsConnection).apply {
            validate { it.requireValue("@event_name", "payment_created") }
            validate { it.requireKey("payment_id", "amount") }
        }.register(this)
    }

    override fun onPacket(packet: JsonMessage, context: MessageContext) {
        eventsReceived.increment()

        processingDuration.record {
            try {
                processPayment(packet)
                eventsProcessed.increment()
            } catch (e: Exception) {
                eventsFailed.increment()
                throw e
            }
        }
    }

    override fun onError(problems: MessageProblems, context: MessageContext) {
        eventsFailed.increment()
        logger.error(
            "Failed to validate event",
            kv("validation_errors", problems.toString())
        )
    }
}
```

## Kafka Lag Monitoring

```kotlin
val consumerLag = Gauge.builder("kafka_consumer_lag") {
    // Calculate lag from Kafka metrics
    kafkaConsumer.metrics()
        .filter { it.key.name() == "records-lag" }
        .values
        .sumOf { (it.metricValue() as? Number)?.toDouble() ?: 0.0 }
}
    .description("Current Kafka consumer lag")
    .tag("consumer_group", "my-app")
    .register(meterRegistry)
```

## Event Tracing

```kotlin
override fun onPacket(packet: JsonMessage, context: MessageContext) {
    val span = tracer.spanBuilder("processPaymentEvent")
        .setAttribute("event.type", "payment_created")
        .setAttribute("payment.id", packet["payment_id"].asText())
        .setAttribute("messaging.system", "kafka")
        .setAttribute("messaging.destination", "teamdagpenger.rapid.v1")
        .startSpan()

    try {
        processPayment(packet)
        span.setStatus(StatusCode.OK)
    } catch (e: Exception) {
        span.setStatus(StatusCode.ERROR, "Event processing failed")
        span.recordException(e)
        throw e
    } finally {
        span.end()
    }
}
```
