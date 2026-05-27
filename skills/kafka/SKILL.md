---
name: kafka
description: Rapids & Rivers, eventdrevet arkitektur, Kafka-mønstre og schema-design for Nav-applikasjoner
license: MIT
compatibility: Kotlin/JVM application with Kafka on Nais
metadata:
  domain: backend
  tags: kafka rapids-rivers events event-driven nais
---

# Kafka & Rapids & Rivers Skill

Patterns, templates, and procedures for building event-driven systems with Kafka on Nais. Covers Rapids & Rivers framework, event schema design, and consumer/producer patterns.

## When to Use

- Setting up Kafka in a Nais application
- Implementing a Rapids & Rivers consumer (River)
- Designing event schemas
- Testing event-driven code with TestRapid
- Troubleshooting Kafka connectivity or consumer lag

## Commands

```bash
# Check Kafka env vars are present (names only, not values)
kubectl exec -it <pod> -n <namespace> -- env | grep -o '^KAFKA[^=]*'

# Verify Kafka credentials are mounted
kubectl exec -it <pod> -n <namespace> -- ls -la /var/run/secrets/nais.io/kafka/

# View pod logs for Kafka events
kubectl logs -n <namespace> <pod> --tail=50 | grep -i "event\|kafka\|river"
```

## Setting Up Kafka

### Nais Manifest

```yaml
apiVersion: nais.io/v1alpha1
kind: Application
metadata:
  name: my-app
spec:
  kafka:
    pool: nav-dev # or nav-prod
```

This automatically:

- Creates Kafka credentials
- Mounts credentials in `/var/run/secrets/nais.io/kafka/`
- Provides environment variables

### Configuration

```kotlin
// Environment variables automatically set by Nais
val kafkaConfig = mapOf(
    "KAFKA_BROKERS" to System.getenv("KAFKA_BROKERS"),
    "KAFKA_TRUSTSTORE_PATH" to System.getenv("KAFKA_TRUSTSTORE_PATH"),
    "KAFKA_CREDSTORE_PASSWORD" to System.getenv("KAFKA_CREDSTORE_PASSWORD"),
    "KAFKA_KEYSTORE_PATH" to System.getenv("KAFKA_KEYSTORE_PATH"),
    "KAFKA_CONSUMER_GROUP_ID" to "my-app-v1",
    "KAFKA_RAPID_TOPIC" to "teamname.rapid-v1"
)
```

## Rapids & Rivers

### Core Concepts

- **Rapid**: The Kafka topic where all events flow
- **River**: A consumer that listens to specific event types
- **Need**: A request for data/action
- **Demand**: Required fields in an event
- **Require**: Required values in an event
- **Reject**: Conditions that exclude an event
- **Interested In**: Optional fields to capture

### Application Setup

```kotlin
import no.nav.helse.rapids_rivers.RapidApplication
import no.nav.helse.rapids_rivers.RapidsConnection

fun main() {
    val env = System.getenv()

    RapidApplication.create(env).apply {
        UserCreatedRiver(this, userRepository)
        PaymentProcessedRiver(this, paymentService)
    }.start()
}
```

### Creating a River

```kotlin
import no.nav.helse.rapids_rivers.*

class UserCreatedRiver(
    rapidsConnection: RapidsConnection,
    private val userRepository: UserRepository
) : River.PacketListener {

    init {
        River(rapidsConnection).apply {
            precondition { it.requireValue("@event_name", "user_created") }
            validate { it.requireKey("user_id", "email", "name") }
            validate { it.require("@created_at", JsonNode::asLocalDateTime) }
            validate { it.interestedIn("phone_number") }
        }.register(this)
    }

    override fun onPacket(packet: JsonMessage, context: MessageContext) {
        val userId = packet["user_id"].asText()
        val email = packet["email"].asText()
        val name = packet["name"].asText()
        val createdAt = packet["created_at"].asLocalDateTime()

        userRepository.save(User(id = userId, email = email, name = name, createdAt = createdAt))
    }

    override fun onError(problems: MessageProblems, context: MessageContext) {
        logger.error("Failed to validate message: ${problems.toExtendedReport()}")
    }

    companion object {
        private val logger = KotlinLogging.logger {}
    }
}
```

### Validation Options

```kotlin
// Preconditions — "does this message concern me at all?"
// Failures → onPreconditionError() (silent, not logged — high volume)
precondition { packet ->
    packet.requireValue("@event_name", "payment_processed")
    packet.forbid("@cancelled")
    packet.forbidValue("status", "cancelled")
}

// Validations — "is the message I care about well-formed?"
// Failures → onError() (logged — indicates contract violation)
validate { packet ->
    packet.requireKey("transaction_id", "amount")
    packet.require("amount", JsonNode::asDouble)
    packet.require("processed_at", JsonNode::asLocalDateTime)
    packet.requireAny("user_id", "session_id")
    packet.interestedIn("metadata", "correlation_id")
}
```

## Publishing Events

```kotlin
fun publishUserCreatedEvent(user: User, context: MessageContext) {
    val event = JsonMessage.newMessage(
        mapOf(
            "@event_name" to "user_created",
            "@id" to UUID.randomUUID().toString(),
            "@created_at" to LocalDateTime.now(),
            "@produced_by" to "my-service",
            "user_id" to user.id,
            "email" to user.email,
            "name" to user.name
        )
    )
    context.publish(event.toJson())
}
```

### Event Metadata

Always include standard metadata:

```kotlin
"@event_name" to "payment_processed",  // Event type
"@id" to UUID.randomUUID().toString(), // Unique event ID
"@created_at" to LocalDateTime.now(),  // When event was created
"@produced_by" to "payment-service",   // Service that created it
"@correlation_id" to correlationId     // Request correlation ID (optional)
```

## Event Schema Design

```kotlin
// ✅ Good - past tense, specific, immutable facts
"user_created", "payment_processed", "application_approved"

// ❌ Bad - imperative, vague
"create_user", "process", "handle_application"
```

### Event Versioning

```kotlin
// Option 1: Version in event name
"@event_name" to "user_created_v2"

// Option 2: Version field
"@event_name" to "user_created",
"@version" to 2
```

## Testing with TestRapid

```kotlin
import no.nav.helse.rapids_rivers.testsupport.TestRapid

class UserCreatedRiverTest {
    private lateinit var testRapid: TestRapid
    private lateinit var userRepository: UserRepository

    @BeforeEach
    fun setup() {
        testRapid = TestRapid()
        userRepository = InMemoryUserRepository()
        UserCreatedRiver(testRapid, userRepository)
    }

    @Test
    fun `processes user_created event`() {
        testRapid.sendTestMessage("""
            {
                "@event_name": "user_created",
                "@id": "550e8400-e29b-41d4-a716-446655440000",
                "@created_at": "2024-01-15T10:30:00",
                "user_id": "12345",
                "email": "user@nav.no",
                "name": "Test User"
            }
        """)

        val user = userRepository.findById("12345")
        assertEquals("user@nav.no", user.email)
    }

    @Test
    fun `publishes downstream event`() {
        testRapid.sendTestMessage(/* ... */)

        val published = testRapid.inspektør.message(0)
        assertEquals("need_user_permissions", published["@event_name"].asText())
    }
}
```

## Error Handling

### Retries and DLQ

```kotlin
override fun onPacket(packet: JsonMessage, context: MessageContext) {
    try {
        processEvent(packet)
    } catch (e: TemporaryException) {
        throw e // Let Kafka retry
    } catch (e: PermanentException) {
        logger.error("Permanent error processing event", e)
        dlqProducer.send(eventName = "user_created", originalMessage = packet.toJson(), error = e.message)
    }
}
```

### Idempotency

```kotlin
override fun onPacket(packet: JsonMessage, context: MessageContext) {
    val eventId = packet["@id"].asText()
    if (eventRepository.exists(eventId)) {
        logger.info("Event $eventId already processed, skipping")
        return
    }
    processEvent(packet)
    eventRepository.markProcessed(eventId)
}
```

## Monitoring

```kotlin
private val eventsProcessed = meterRegistry.counter("events_processed_total", "event_name", "user_created")
private val processingDuration = meterRegistry.timer("event_processing_duration_seconds", "event_name", "user_created")

override fun onPacket(packet: JsonMessage, context: MessageContext) {
    processingDuration.record {
        processEvent(packet)
        eventsProcessed.increment()
    }
}
```

## Gotchas

- Changing `KAFKA_CONSUMER_GROUP_ID` causes reprocessing of all messages
- `precondition` failures are silent (high volume) — use for event type filtering
- `validate` failures call `onError()` — use for schema validation
- Always include `@id` for idempotency
- Don't publish PII in event payloads without encryption
- Test with `TestRapid` — never mock Kafka directly

## Boundaries

### ✅ Always

- Use past tense for event names (`user_created`, not `create_user`)
- Include standard metadata (`@event_name`, `@id`, `@created_at`)
- Implement idempotency (check `@id` before processing)
- Write TestRapid tests for all Rivers
- Use `precondition` for event type filtering
- Log with `event_id` for traceability

### ⚠️ Ask First

- Creating new Kafka topics
- Changing consumer group IDs
- Publishing high-volume events (> 1000/sec)
- Modifying event schemas (breaking changes)

### 🚫 Never

- Use imperative event names
- Skip the `@id` field
- Change consumer group without migration plan
- Publish PII in event payloads without encryption
- Ignore `onError` handler in Rivers
