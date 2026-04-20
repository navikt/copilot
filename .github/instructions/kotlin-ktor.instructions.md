---
applyTo: "**/*.kt"
---

Ktor- og Rapids & Rivers-mønstre for Nav-backends: ApplicationBuilder, sealed config, Kotliquery, Koin og feilhåndtering.

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

## Database Access (Kotliquery)

Use Kotliquery with HikariCP connection pooling. This is Nav's standard — not JPA/Hibernate.

```kotlin
object PostgresDataSourceBuilder {
    val dataSource by lazy {
        HikariDataSource().apply {
            jdbcUrl = getOrThrow(DB_URL_KEY)
            maximumPoolSize = 5 // Start low in K8s; scale up if needed
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

### Kotliquery Patterns

```kotlin
// ✅ Batch insert
fun saveAll(entities: List<Entity>) = using(sessionOf(dataSource)) { session ->
    session.batchPreparedNamedStatement(
        "INSERT INTO table (col1, col2) VALUES (:col1, :col2)",
        entities.map { mapOf("col1" to it.col1, "col2" to it.col2) }
    )
}

// ✅ Transaction
fun transferWithinTransaction() = using(sessionOf(dataSource)) { session ->
    session.transaction { tx ->
        tx.run(queryOf("UPDATE accounts SET balance = balance - ? WHERE id = ?", amount, fromId).asUpdate)
        tx.run(queryOf("UPDATE accounts SET balance = balance + ? WHERE id = ?", amount, toId).asUpdate)
    }
}

// ✅ Row mapper as extension function
private fun Row.toEntity() = Entity(
    id = long("id"),
    name = string("name"),
    description = stringOrNull("description"),
    createdAt = localDateTime("created_at"),
)
```

## Transaction Patterns

JDBC connections are thread-bound. `ThreadLocal` values do not propagate to new coroutines.
Never use `launch`, `async`, or other coroutine builders inside a transaction block.

### Simple single-block transaction

Works when all operations happen in the same place:

```kotlin
fun transferFunds(fromId: Long, toId: Long, amount: BigDecimal) =
    using(sessionOf(dataSource)) { session ->
        session.transaction { tx ->
            tx.run(queryOf("UPDATE accounts SET balance = balance - ? WHERE id = ?", amount, fromId).asUpdate)
            tx.run(queryOf("UPDATE accounts SET balance = balance + ? WHERE id = ?", amount, toId).asUpdate)
        }
    }
```

### Explicit transaction parameter

Recommended when the transaction spans multiple layers. Type-safe and easy to follow:

```kotlin
class DbTransaction(val session: TransactionalSession)

fun <T> transaction(dataSource: DataSource, block: DbTransaction.() -> T): T =
    sessionOf(dataSource).use { session ->
        session.transaction { tx -> DbTransaction(tx).block() }
    }

// Repository methods take DbTransaction as receiver
fun DbTransaction.saveOrder(order: Order): Long =
    session.run(
        queryOf("INSERT INTO orders (product, amount) VALUES (?, ?)", order.product, order.amount)
            .asUpdateAndReturnGeneratedKey
    ) ?: error("Failed to insert order")

fun DbTransaction.updateInventory(productId: Long, delta: Int) =
    session.run(
        queryOf("UPDATE inventory SET stock = stock + ? WHERE product_id = ?", delta, productId)
            .asUpdate
    )

// Usage — everything runs in the same transaction without ThreadLocal
transaction(dataSource) {
    val orderId = saveOrder(order)
    updateInventory(order.productId, -order.quantity)
}
```

### ThreadLocal-based transaction

Pragmatic for existing layered architectures where many service methods already call repositories.
Repositories automatically reuse the active transaction:

```kotlin
object Database {
    private lateinit var dataSource: DataSource
    private val transactionalSession = ThreadLocal<TransactionalSession?>()

    fun <T> query(block: (Session) -> T): T {
        val tx = transactionalSession.get()
        return if (tx != null) block(tx) else using(sessionOf(dataSource)) { block(it) }
    }

    fun <T> transaction(block: () -> T): T {
        check(transactionalSession.get() == null) { "Nested transactions are not supported" }
        return sessionOf(dataSource).use { session ->
            session.transaction { tx ->
                transactionalSession.set(tx)
                try { block() } finally { transactionalSession.remove() }
            }
        }
    }
}
```

> **⚠️** ThreadLocal does not propagate to new coroutines. This approach only works
> when all code in the transaction block runs on the same thread without suspend calls.

## Dependency Injection

For small apps, constructor injection without a framework is simplest — pass dependencies directly.
For larger apps with many services and repositories, Koin keeps the wiring manageable:

```kotlin
// Module definition
fun appModule(config: ApplicationConfig) = module {
    single { Database.dataSource(config.database) }
    single { ResourceRepository(get()) }
    single { ResourceService(get()) }
}

// Install in Application
fun Application.main() {
    install(Koin) {
        slf4jLogger()
        modules(appModule(config))
    }
}

// Inject in routes
fun Route.resourceRoutes() {
    val service by inject<ResourceService>()
    get("/api/resources") { call.respond(service.findAll()) }
}
```

## When Using Arrow-kt (Functional Patterns)

Arrow-kt is increasingly adopted for error handling. Use when the project already depends on it.

```kotlin
import arrow.core.Either
import arrow.core.raise.either

// ✅ Either for domain errors
sealed class DomainError {
    data class NotFound(val id: String) : DomainError()
    data class Validation(val errors: List<String>) : DomainError()
}

fun findResource(id: String): Either<DomainError, Resource> = either {
    val resource = repository.findById(id)
        ?: raise(DomainError.NotFound(id))
    resource
}

// ✅ Map Either to HTTP response
get("/{id}") {
    val id = call.parameters["id"] ?: return@get call.respond(HttpStatusCode.BadRequest)
    when (val result = service.findResource(id)) {
        is Either.Right -> call.respond(HttpStatusCode.OK, result.value)
        is Either.Left -> when (result.value) {
            is DomainError.NotFound -> call.respond(HttpStatusCode.NotFound)
            is DomainError.Validation -> call.respond(HttpStatusCode.BadRequest, result.value)
        }
    }
}
```

Do not introduce Arrow-kt into projects that don't already use it without discussion.

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

## Graceful Shutdown

> **NAIS pod lifecycle:** NAIS injects a `sleep 5` preStop hook before your app receives SIGTERM. By then, the load balancer has already stopped routing new traffic. Your app does **not** need to manipulate readiness probes — just finish in-flight requests and exit.

For standalone Ktor servers (non-Rapids & Rivers):

```kotlin
fun main() {
    val server = embeddedServer(Netty, port = 8080) {
        api()
    }

    server.start(wait = false)

    Runtime.getRuntime().addShutdownHook(Thread {
        logger.info { "SIGTERM received, draining connections" }
        server.stop(
            gracePeriodMillis = 5_000,  // wait for in-flight requests
            timeoutMillis = 10_000      // hard deadline
        )
    })

    Thread.currentThread().join()
}
```

For Rapids & Rivers apps, `RapidApplication` handles shutdown automatically.

Common anti-patterns:
- ❌ Setting `/isready` to return 503 on SIGTERM — unnecessary on NAIS
- ❌ Adding a preStop hook — NAIS already injects `sleep 5`
- ✅ `server.stop(gracePeriod, timeout)` drains in-flight requests — this is all you need

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

### Testing the Ktor application

Use `testApplication` to test the same modules as production — this tests `src/main` code directly:

```kotlin
class RoutesTest {
    @Test
    fun `should return resources`() = testApplication {
        application {
            configureSerialization()
            configureRouting(testRepository)
        }
        client.get("/api/resources").apply {
            status shouldBe HttpStatusCode.OK
        }
    }

    @Test
    fun `should return 401 without token`() = testApplication {
        application {
            configureAuth(mockOAuth2Server)
            configureRouting(testRepository)
        }
        client.get("/api/resources").apply {
            status shouldBe HttpStatusCode.Unauthorized
        }
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

## Dependency Injection (Koin)

Use Koin for lightweight, Kotlin-native DI:

```kotlin
// Module definition
val appModule = module {
    single<DataSource> { PostgresDataSourceBuilder.dataSource }
    single<UserRepository> { UserRepositoryPostgres(get()) }
    single<UserService> { UserService(get()) }
    factory<SomeClient> { SomeClient(get<AppConfig>().clientUrl) }
}

// Bootstrap with Koin
fun main() {
    startKoin { modules(appModule) }

    embeddedServer(Netty, port = 8080) {
        val service: UserService by inject()
        routing { userRoutes(service) }
    }.start(wait = true)
}
```

### Test modules

```kotlin
@Test
fun `should process user`() {
    startKoin {
        modules(module {
            single<UserRepository> { FakeUserRepository() }
            single<UserService> { UserService(get()) }
        })
    }

    val service: UserService by inject()
    service.create(testUser) shouldNotBe null

    stopKoin()
}
```

## Functional Error Handling (Arrow-kt)

Use Arrow's `Either` and `Raise` for typed error handling without exceptions:

```kotlin
// Define domain errors as sealed hierarchy
sealed class UserError {
    data class NotFound(val id: UserId) : UserError()
    data class ValidationFailed(val reason: String) : UserError()
    data object Unauthorized : UserError()
}

// Either-based service methods
suspend fun findUser(id: UserId): Either<UserError, User> =
    either {
        val entity = repository.findById(id)
            ?: raise(UserError.NotFound(id))
        entity.toDomain()
    }

// Compose multiple Either operations
suspend fun processApplication(request: Request): Either<AppError, Receipt> =
    either {
        val user = findUser(request.userId).bind()
        val validated = validate(request).bind()
        submit(user, validated).bind()
    }
```

### Integrating Arrow with Ktor routing

```kotlin
fun Route.userRoutes(service: UserService) {
    get("/api/users/{id}") {
        val id = UserId(call.parameters["id"]!!)
        service.findUser(id).fold(
            ifLeft = { error ->
                when (error) {
                    is UserError.NotFound -> call.respond(HttpStatusCode.NotFound)
                    is UserError.Unauthorized -> call.respond(HttpStatusCode.Forbidden)
                    is UserError.ValidationFailed ->
                        call.respond(HttpStatusCode.BadRequest, error.reason)
                }
            },
            ifRight = { user -> call.respond(HttpStatusCode.OK, user) }
        )
    }
}
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

## Related

| Resource | Use For |
|----------|---------|
| `kotlin-app-config` skill | Sealed class configuration pattern (Dev/Prod/Local) |
| `ktor-scaffold` skill | Scaffolding new Ktor services with full stack |
| `@auth-agent` | JWT validation, TokenX, ID-porten implementation |
| `@nais-agent` | Nais manifest, accessPolicy, secrets |
| `@observability-agent` | Prometheus metrics, Grafana, tracing |
| `flyway-migration` skill | Database migration patterns |
| `api-design` skill | REST API conventions (RFC 7807, versioning) |
