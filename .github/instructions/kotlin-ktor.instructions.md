---
applyTo: "**/*.kt"
---

> Ktor/Rapids & Rivers patterns for Nav backends. Apply when the file uses Ktor (`RapidApplication`, `routing`, `River`) — for Spring Boot apps, see `kotlin-spring.instructions.md` instead.

# Kotlin/Ktor Development Standards

## Application Structure

Use the ApplicationBuilder pattern for bootstrapping applications:

```kotlin
class ApplicationBuilder(configuration: Map<String, String>) {
    private val meterRegistry = PrometheusMeterRegistry(PrometheusConfig.DEFAULT)
    private val dataSource = PostgresDataSourceBuilder.dataSource
    private val rapidsConnection: RapidsConnection

    init {
        rapidsConnection = RapidApplication.create(configuration)
        // Register rivers and event handlers
    }

    fun start() {
        rapidsConnection.start()
    }
}
```

## Configuration Pattern

Use sealed classes for environment-specific configuration with compile-time safety:

```kotlin
sealed class ApplicationConfig {
    abstract val database: DatabaseConfig
    abstract val kafka: KafkaConfig
    abstract val http: HttpConfig

    data class Dev(
        override val database: DatabaseConfig,
        override val kafka: KafkaConfig,
        override val http: HttpConfig
    ) : ApplicationConfig()

    data class Prod(...) : ApplicationConfig()
    data class Local(...) : ApplicationConfig()
}

// Usage
val config = when (environment) {
    "prod" -> ApplicationConfig.Prod(...)
    "dev" -> ApplicationConfig.Dev(...)
    else -> ApplicationConfig.Local(...)
}
```

Use `konfig` library for typed configuration:

```kotlin
object Configuration {
    private val defaultProperties = ConfigurationMap(...)
    val properties =
        ConfigurationProperties.systemProperties()
        overriding EnvironmentVariables()
        overriding defaultProperties

    val databaseUrl by lazy {
        properties[Key("DB_JDBC_URL", stringType)]
    }
}
```

## Database Access

Use Kotliquery with HikariCP connection pooling:

```kotlin
object PostgresDataSourceBuilder {
    val dataSource by lazy {
        HikariDataSource().apply {
            jdbcUrl = getOrThrow(DB_URL_KEY)
            maximumPoolSize = 40
            minimumIdle = 1
        }
    }
}

// Repository pattern with interface
class RepositoryPostgres(private val dataSource: DataSource) : Repository {
    override fun save(entity: Entity): Long {
        return using(sessionOf(dataSource)) { session ->
            session.run(
                queryOf(
                    "INSERT INTO table (col1, col2) VALUES (?, ?)",
                    entity.col1, entity.col2
                ).asUpdateAndReturnGeneratedKey
            ) ?: throw Exception("Failed to insert")
        }
    }

    override fun findById(id: Long): Entity? {
        return using(sessionOf(dataSource)) { session ->
            session.run(
                queryOf("SELECT * FROM table WHERE id = ?", id)
                    .map { row ->
                        Entity(
                            id = row.long("id"),
                            col1 = row.string("col1")
                        )
                    }.asSingle
            )
        }
    }
}
```

## Ktor Routing

Structure routes using extension functions on `Application`:

```kotlin
fun Application.api() {
    routing {
        authenticate("azureAd") {
            get("/api/resource") {
                val user = call.principal<JWTPrincipal>()
                call.respond(HttpStatusCode.OK, data)
            }

            post("/api/resource") {
                val request = call.receive<RequestDto>()
                call.respond(HttpStatusCode.Created, result)
            }
        }

        // Health endpoints (unauthenticated)
        get("/isalive") { call.respondText("Alive") }
        get("/isready") { call.respondText("Ready") }
        get("/metrics") { call.respondText(meterRegistry.scrape()) }
    }
}
```

## Kafka Rapids & Rivers

Use the Rapids & Rivers pattern for event-driven architecture:

```kotlin
class MyEventRiver(rapidsConnection: RapidsConnection) : River.PacketListener {
    init {
        River(rapidsConnection).apply {
            validate { it.demandValue("@event_name", "my_event") }
            validate { it.requireKey("required_field") }
            validate { it.interestedIn("optional_field") }
        }.register(this)
    }

    override fun onPacket(packet: JsonMessage, context: MessageContext) {
        val requiredField = packet["required_field"].asText()
        // Process event

        // Publish new event if needed
        val response = JsonMessage.newNeed(
            listOf("SomeCapability"),
            mapOf("data" to data)
        )
        context.publish(ident, response.toJson())
    }
}
```

## Testing

Use Kotest for test structure and assertions:

```kotlin
class ServiceTest {
    companion object {
        @BeforeAll
        @JvmStatic
        fun setup() {
            mockkObject(ApplicationBuilder.Companion)
            every { getRapidsConnection() } returns TestRapid()
        }
    }

    @Test
    fun `should process event correctly`() {
        val testRapid = TestRapid()
        val service = Service(testRapid)

        testRapid.sendTestMessage(testEvent)

        val published = testRapid.inspektør.message(0)
        published["field"] shouldBe expectedValue
    }
}
```

Use Testcontainers for database integration tests:

```kotlin
@Testcontainers
class RepositoryTest {
    companion object {
        @Container
        val postgres = PostgreSQLContainer<Nothing>("postgres:15").apply {
            withDatabaseName("testdb")
        }
    }

    @Test
    fun `should save and retrieve entity`() {
        val dataSource = HikariDataSource().apply {
            jdbcUrl = postgres.jdbcUrl
            username = postgres.username
            password = postgres.password
        }

        val repository = RepositoryPostgres(dataSource)
        val saved = repository.save(entity)
        val retrieved = repository.findById(saved)

        retrieved shouldNotBe null
    }
}
```

## Observability

Implement Prometheus metrics using Micrometer:

```kotlin
val meterRegistry = PrometheusMeterRegistry(
    PrometheusConfig.DEFAULT,
    PrometheusRegistry.defaultRegistry,
    Clock.SYSTEM
)

// Counter
val requestCounter = Counter.builder("http_requests_total")
    .description("Total HTTP requests")
    .tag("method", "GET")
    .register(meterRegistry)

requestCounter.increment()

// Timer
val requestTimer = Timer.builder("http_request_duration")
    .description("HTTP request duration")
    .register(meterRegistry)

requestTimer.record {
    // Process request
}
```

Use structured logging with KotlinLogging:

```kotlin
private val logger = KotlinLogging.logger {}

logger.info { "Processing event: ${event.id}" }
logger.warn { "Retrying failed operation" }
logger.error(exception) { "Failed to process event" }
```

## Boundaries

### ✅ Always

- Use sealed classes for state and configuration
- Implement Repository pattern for database access
- Add Prometheus metrics for business operations
- Use Flyway for database migrations
- Implement all three health endpoints

### ⚠️ Ask First

- Changing database schema
- Modifying Kafka event schemas
- Adding new Rapids & Rivers dependencies
- Changing authentication configuration

### 🚫 Never

- Skip database migration versioning
- Bypass authentication checks
- Use `!!` operator without null checks
- Commit configuration secrets
