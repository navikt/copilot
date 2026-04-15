---
name: kafka-topic
description: Legg til Kafka-topic-konfigurasjon i Nais-manifest og lag Rapids & Rivers event handler
---

You are helping configure Kafka integration for a Nav application using the Rapids & Rivers pattern.

## Step 1: Add Kafka Configuration to Nais Manifest

Update `.nais/app.yaml` to include Kafka:

```yaml
kafka:
  pool: nav-dev # or nav-prod for production
```

## Step 2: Create Event Handler

Ask the user:

1. **Event name**: What event should this handler listen for?
2. **Required fields**: What fields must be present in the event?
3. **Optional fields**: What fields are optional?
4. **Action**: What should happen when this event is received?

### Kotlin Implementation

Create a River for handling the event:

```kotlin
package no.nav.your.package.rivers

import no.nav.helse.rapids_rivers.*

class YourEventRiver(rapidsConnection: RapidsConnection) : River.PacketListener {
    init {
        River(rapidsConnection).apply {
            validate { it.demandValue("@event_name", "your_event_name") }
            validate { it.requireKey("required_field_1", "required_field_2") }
            validate { it.interestedIn("optional_field") }
        }.register(this)
    }

    override fun onPacket(packet: JsonMessage, context: MessageContext) {
        val requiredField = packet["required_field_1"].asText()
        val optionalField = packet["optional_field"].takeIf { !it.isMissingOrNull() }?.asText()

        // Process the event
        logger.info { "Processing event: ${packet["@event_name"].asText()}" }

        // Perform business logic
        val result = processEvent(requiredField, optionalField)

        // Publish response event if needed
        val response = JsonMessage.newNeed(
            listOf("RequiredCapability"),
            mapOf(
                "correlation_id" to packet["@id"].asText(),
                "result" to result,
                "processed_at" to LocalDateTime.now().toString()
            )
        )
        context.publish(requiredField, response.toJson())
    }

    private fun processEvent(field1: String, field2: String?): String {
        // Business logic here
        return "processed"
    }

    companion object {
        private val logger = KotlinLogging.logger {}
    }
}
```

### Register the River

Update your `ApplicationBuilder` to register the river:

```kotlin
class ApplicationBuilder(configuration: Map<String, String>) {
    private val rapidsConnection: RapidsConnection

    init {
        rapidsConnection = RapidApplication.create(configuration)

        // Register event handlers
        YourEventRiver(rapidsConnection)
        // Add more rivers as needed
    }

    fun start() {
        rapidsConnection.start()
    }
}
```

## Step 3: Create Test

Generate a test for the event handler:

```kotlin
package no.nav.your.package.rivers

import io.kotest.matchers.shouldBe
import no.nav.helse.rapids_rivers.testsupport.TestRapid
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test

class YourEventRiverTest {
    private lateinit var testRapid: TestRapid

    @BeforeEach
    fun setup() {
        testRapid = TestRapid()
        YourEventRiver(testRapid)
    }

    @Test
    fun `should process event and publish response`() {
        val testMessage = """
            {
                "@event_name": "your_event_name",
                "@id": "test-correlation-id",
                "required_field_1": "value1",
                "required_field_2": "value2",
                "optional_field": "optional_value"
            }
        """.trimIndent()

        testRapid.sendTestMessage(testMessage)

        testRapid.inspektør.size shouldBe 1
        val published = testRapid.inspektør.message(0)
        published["correlation_id"].asText() shouldBe "test-correlation-id"
        published["result"].asText() shouldBe "processed"
    }

    @Test
    fun `should ignore events with wrong event name`() {
        val testMessage = """
            {
                "@event_name": "different_event",
                "required_field_1": "value1"
            }
        """.trimIndent()

        testRapid.sendTestMessage(testMessage)

        testRapid.inspektør.size shouldBe 0
    }
}
```

## Publishing Events

If you need to **publish** events (not just listen), create a producer:

```kotlin
class EventPublisher(private val rapidsConnection: RapidsConnection) {
    fun publishEvent(identifier: String, data: Map<String, Any>) {
        val message = JsonMessage.newNeed(
            listOf("RequiredCapability"),
            mapOf(
                "@event_name" to "your_event_name",
                "@id" to UUID.randomUUID().toString(),
                "timestamp" to LocalDateTime.now().toString()
            ) + data
        )

        rapidsConnection.publish(identifier, message.toJson())
        logger.info { "Published event: ${message["@event_name"]}" }
    }

    companion object {
        private val logger = KotlinLogging.logger {}
    }
}
```

## Environment Configuration

Ensure Kafka configuration is available in your environment:

```kotlin
private object kafka : PropertyGroup() {
    val brokers by stringType
    val schema_registry by stringType
    val consumer_group_id by stringType
}

val kafkaConfig = KafkaKonfigurasjon(
    serverKonfigurasjon = KafkaServerKonfigurasjon(
        autentisering = "SSL",
        kafkaBrokers = Configuration.properties[kafka.brokers]
    ),
    schemaRegistryKonfigurasjon = KafkaSchemaRegistryConfig(
        url = Configuration.properties[kafka.schema_registry]
    )
)
```

## Prometheus Metrics

Add metrics for event processing:

```kotlin
private val eventsProcessed = Counter.builder("events_processed_total")
    .description("Total events processed")
    .tag("event_name", "your_event_name")
    .register(meterRegistry)

private val processingDuration = Timer.builder("event_processing_duration")
    .description("Event processing duration")
    .tag("event_name", "your_event_name")
    .register(meterRegistry)

override fun onPacket(packet: JsonMessage, context: MessageContext) {
    processingDuration.record {
        // Process event
        eventsProcessed.increment()
    }
}
```

## Documentation

Remind the user to:

1. Document the event schema in their repository
2. Add the new event to the team's event catalog
3. Update the README with event flow diagrams
4. Configure appropriate Kafka topic permissions in Nais

## Forstå koden

After generating the handler, explain:

1. **demandValue vs requireKey vs interestedIn** — Three different validation levels in Rapids & Rivers. What happens when each fails? Why this design?
2. **Idempotens** — What happens if the same event is delivered twice (Kafka guarantees at-least-once)? How should the handler deal with this?
3. **Publiser-mønsteret** — Why `context.publish(ident, ...)` uses an identifier for partitioning. What would happen with random partitioning to ordering guarantees?
4. **Dead-letter-håndtering** — What happens when `onPacket` throws? Where does the failed message go, and how do you recover?

🔴 **Rød sone**: Event-driven architecture has subtle failure modes (ordering, duplication, poison pills) that are hard to debug in production. Understand the semantics before wiring up new rivers.

Still gjerne spørsmål om valgene over.
