---
name: Kotlin Backend Patterns
description: "Kotlin-backendmønstre for Nav: Ktor og Rapids & Rivers, Spring Boot, database, auth, testing og feilhåndtering."
applyTo: "**/*.kt"
---

Kotlin backend patterns for Nav: Ktor and Rapids & Rivers, Spring Boot, database access, auth, testing, and error handling.

> **Pick one half.** A file path cannot tell Ktor from Spring Boot, so this file covers both. Decide from the file itself: `RapidApplication`, `routing`, `River`, `embeddedServer` means Ktor; `@RestController`, `@Service`, `@SpringBootApplication` means Spring Boot. Apply the matching half and ignore the other. Ktor is the most widely used framework for new Kotlin backends in Nav, but Spring Boot is actively used by many teams, and teams choose for themselves. For migrating between them, see [$java-to-kotlin](../skills/java-to-kotlin/).

# Kotlin Backend Standards

## Ktor and Rapids & Rivers

### Application Structure

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

### Configuration Pattern

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

### Database Access (Kotliquery)

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

#### Kotliquery Patterns

```kotlin
// ✅ Batch insert
fun saveAll(entities: List<Entity>) = using(sessionOf(dataSource)) { session ->
    session.batchPreparedNamedStatement(
        "INSERT INTO table (col1, col2) VALUES (:col1, :col2)",
        entities.map { mapOf("col1" to it.col1, "col2" to it.col2) }
    )
}

// ✅ Row mapper as extension function
private fun Row.toEntity() = Entity(
    id = long("id"),
    name = string("name"),
    description = stringOrNull("description"),
    createdAt = localDateTime("created_at"),
)
```

### Transaction Patterns

JDBC connections are thread-bound. `ThreadLocal` values do not propagate to new coroutines.
Never use `launch`, `async`, or other coroutine builders inside a transaction block.

#### Simple single-block transaction

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

#### Explicit transaction parameter

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

// Usage — everything runs in the same transaction without ThreadLocal
transaction(dataSource) {
    val orderId = saveOrder(order)
    updateInventory(order.productId, -order.quantity)
}
```

#### ThreadLocal-based transaction

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

### Dependency Injection (Koin)

For small apps, constructor injection without a framework is simplest — pass dependencies directly.
For larger apps with many services and repositories, Koin keeps the wiring manageable:

```kotlin
// Module definition
fun appModule(config: ApplicationConfig) = module {
    single<DataSource> { Database.dataSource(config.database) }
    single<ResourceRepository> { ResourceRepositoryPostgres(get()) }
    single<ResourceService> { ResourceService(get()) }
    factory<SomeClient> { SomeClient(get<AppConfig>().clientUrl) }
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

Swap the modules for fakes in tests, and call `stopKoin()` afterwards:

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

### Functional Error Handling (Arrow-kt)

Arrow-kt is increasingly adopted for error handling. Use it when the project already depends on it, and do not introduce it elsewhere without discussion.

```kotlin
import arrow.core.Either
import arrow.core.raise.either

// Domain errors as a sealed hierarchy
sealed class UserError {
    data class NotFound(val id: UserId) : UserError()
    data class ValidationFailed(val reason: String) : UserError()
    data object Unauthorized : UserError()
}

// Either-based service methods, composed with bind()
suspend fun findUser(id: UserId): Either<UserError, User> = either {
    val entity = repository.findById(id) ?: raise(UserError.NotFound(id))
    entity.toDomain()
}

suspend fun processApplication(request: Request): Either<AppError, Receipt> = either {
    val user = findUser(request.userId).bind()
    val validated = validate(request).bind()
    submit(user, validated).bind()
}

// Map the result to an HTTP response at the route
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

### Ktor Routing

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

### Graceful Shutdown

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

### Kafka Rapids & Rivers

Use the Rapids & Rivers pattern for event-driven architecture:

```kotlin
class MyEventRiver(rapidsConnection: RapidsConnection) : River.PacketListener {
    init {
        River(rapidsConnection).apply {
            precondition { it.requireValue("@event_name", "my_event") }
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

### Observability

Implement Prometheus metrics using Micrometer, and log with KotlinLogging:

```kotlin
val meterRegistry = PrometheusMeterRegistry(
    PrometheusConfig.DEFAULT,
    PrometheusRegistry.defaultRegistry,
    Clock.SYSTEM
)

val requestCounter = Counter.builder("http_requests_total")
    .description("Total HTTP requests")
    .tag("method", "GET")
    .register(meterRegistry)

private val logger = KotlinLogging.logger {}
logger.info { "Processing event: ${event.id}" }
logger.error(exception) { "Failed to process event" }
```

### Testing

Use Kotest for test structure and assertions, and `TestRapid` for rivers:

```kotlin
class ServiceTest {
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

        repository.findById(saved) shouldNotBe null
    }
}
```

### Ktor boundaries

**✅ Always**

- Use sealed classes for state and configuration
- Implement Repository pattern for database access
- Add Prometheus metrics for business operations
- Implement all three health endpoints

**⚠️ Ask First**

- Modifying Kafka event schemas
- Adding new Rapids & Rivers dependencies

**🚫 Never**

- Use `!!` without a null check

## Spring Boot

### Controller Layer

```kotlin
@RestController
@RequestMapping("/api")
class ResourceController(
    private val service: ResourceService
) {
    @GetMapping("/resources/{id}")
    fun getResource(@PathVariable id: UUID): ResponseEntity<ResourceDTO> {
        val resource = service.findById(id)
        return ResponseEntity.ok(resource)
    }

    @PostMapping("/resources")
    fun createResource(@RequestBody @Valid request: CreateResourceRequest): ResponseEntity<ResourceDTO> {
        val created = service.create(request)
        return ResponseEntity.status(HttpStatus.CREATED).body(created)
    }
}
```

### Service Layer

```kotlin
@Service
class ResourceService(
    private val repository: ResourceRepository
) {
    @Transactional
    fun create(request: CreateResourceRequest): ResourceDTO {
        val entity = request.toEntity()
        return repository.save(entity).toDTO()
    }
}
```

### Database Access

Check existing repository implementations in the codebase — patterns vary:

```kotlin
// Option A: CrudRepository interface
@Repository
interface ResourceRepository : CrudRepository<ResourceEntity, UUID> {
    fun findByIdent(ident: String): List<ResourceEntity>

    @Query("SELECT * FROM resource WHERE status = :status")
    fun findByStatus(status: String): List<ResourceEntity>
}

// Option B: NamedParameterJdbcTemplate (raw SQL)
@Repository
class JdbcResourceRepository(
    private val namedParameterJdbcTemplate: NamedParameterJdbcTemplate
) {
    fun findById(id: UUID): ResourceEntity? {
        val sql = "SELECT * FROM resource WHERE id = :id"
        return namedParameterJdbcTemplate.query(sql, mapOf("id" to id)) { rs, _ ->
            ResourceEntity(id = rs.getObject("id", UUID::class.java))
        }.firstOrNull()
    }
}
```

### Auth (token-validation-spring)

```kotlin
@ProtectedWithClaims(issuer = "azuread")
@RestController
class ProtectedController {
    @GetMapping("/api/protected")
    fun protectedEndpoint(): ResponseEntity<Any> {
        // Token validation is handled automatically by the filter
        return ResponseEntity.ok(mapOf("status" to "ok"))
    }
}
```

### Configuration

Use `application.yml` / `application-{profile}.yml` for Spring configuration:

```yaml
spring:
  datasource:
    url: jdbc:postgresql://${DB_HOST}:${DB_PORT}/${DB_DATABASE}
    username: ${DB_USERNAME}
    password: ${DB_PASSWORD}
  flyway:
    enabled: true
```

Bind typed configuration with `@ConfigurationProperties`:

```kotlin
@ConfigurationProperties(prefix = "app")
data class AppProperties(
    val externalApiUrl: String,
    val maxRetries: Int = 3,
    val featureFlags: FeatureFlags = FeatureFlags(),
) {
    data class FeatureFlags(
        val nyFunksjon: Boolean = false,
    )
}

// Enable in Application.kt
@SpringBootApplication
@EnableConfigurationProperties(AppProperties::class)
class Application
```

### Structured Logging

```kotlin
// Check existing log statements in the repo to match the established pattern
// SLF4J placeholder format (always available)
logger.info("Processing event: eventId={}", eventId)

// If logstash-logback-encoder is on the classpath:
// logger.info("Processing event {}", kv("event_id", eventId))

// Spring request-scoped MDC via filter
MDC.put("x_request_id", request.getHeader("X-Request-ID"))
```

### Error Handling (ProblemDetail)

```kotlin
@RestControllerAdvice
class ErrorHandler {
    @ExceptionHandler(ResourceNotFoundException::class)
    fun handleNotFound(ex: ResourceNotFoundException): ProblemDetail =
        ProblemDetail.forStatusAndDetail(HttpStatus.NOT_FOUND, ex.message ?: "Ressurs ikke funnet")

    @ExceptionHandler(MethodArgumentNotValidException::class)
    fun handleValidation(ex: MethodArgumentNotValidException): ProblemDetail =
        ProblemDetail.forStatusAndDetail(HttpStatus.BAD_REQUEST, "Validering feilet").apply {
            setProperty("feil", ex.bindingResult.fieldErrors.map {
                mapOf("felt" to it.field, "melding" to it.defaultMessage)
            })
        }
}
```

### Testing

Use `@SpringBootTest` with Testcontainers and MockOAuth2Server for full integration tests. Prefer test slices when the whole context is not needed:

```kotlin
// Controller-only test — no database, no service
@WebMvcTest(ResourceController::class)
class ResourceControllerSliceTest {
    @Autowired lateinit var mockMvc: MockMvc
    @MockkBean lateinit var service: ResourceService

    @Test
    fun `should return 200`() {
        every { service.findAll() } returns listOf(testResource())
        mockMvc.get("/api/resources") {
            header("Authorization", "Bearer ${token()}")
        }.andExpect { status { isOk() } }
    }
}

// Repository-only test — real database, no controllers
@DataJpaTest
@Testcontainers
class ResourceRepositorySliceTest {
    @Autowired lateinit var repository: ResourceRepository

    companion object {
        @Container val postgres = PostgreSQLContainer("postgres:15")

        @DynamicPropertySource @JvmStatic
        fun configure(registry: DynamicPropertyRegistry) {
            registry.add("spring.datasource.url") { postgres.jdbcUrl }
            registry.add("spring.datasource.username") { postgres.username }
            registry.add("spring.datasource.password") { postgres.password }
        }
    }

    @Test
    fun `should save and find`() {
        val saved = repository.save(ResourceEntity(name = "test"))
        repository.findById(saved.id!!) shouldNotBe null
    }
}
```

`@MockkBean` needs `com.ninja-squad:springmockk` — verify it is in `build.gradle.kts` before using it.

### Spring boundaries

**✅ Always**

- Use constructor injection, never field injection (`@Autowired` on fields)
- Annotate transactional boundaries explicitly, on the service layer and not the controller
- Follow the existing repository pattern in the codebase — don't mix Spring Data JPA and JDBC in the same layer

**⚠️ Ask First**

- Introducing new Spring modules or starters
- Changing transaction isolation levels

**🚫 Never**

- Put business logic in controllers

## Boundaries for both

### ✅ Always

- Use Flyway for database migrations, with versioned scripts
- Preserve existing code structure when making targeted fixes — don't rename, restructure, or refactor working code beyond the task at hand

### ⚠️ Ask First

- Changing database schema
- Changing authentication configuration

### 🚫 Never

- Skip database migration versioning
- Bypass authentication checks
- Commit configuration secrets

## Related

| Type | Name | When to use |
|------|------|-------------|
| Skill | [$kotlin-app-config](../skills/kotlin-app-config/) | Sealed class configuration pattern (Dev/Prod/Local) |
| Skill | [$ktor-scaffold](../skills/ktor-scaffold/) | Scaffolding a new Ktor service with full stack |
| Skill | [$spring-boot-scaffold](../skills/spring-boot-scaffold/) | Scaffolding a new Spring Boot Kotlin project |
| Skill | [$java-to-kotlin](../skills/java-to-kotlin/) | Migrating Java or Spring Boot code to Kotlin |
| Skill | [$flyway-migration](../skills/flyway-migration/) | Database migration patterns |
| Skill | [$nav-auth](../skills/nav-auth/) | JWT validation, TokenX, ID-porten, Azure AD |
| Skill | [$nais](../skills/nais/) | Nais manifest, accessPolicy, secrets |
| Skill | [$observability-setup](../skills/observability-setup/) | Prometheus metrics, Grafana, tracing |
| Skill | [$api-design](../skills/api-design/) | REST API conventions (RFC 7807, versioning) |
