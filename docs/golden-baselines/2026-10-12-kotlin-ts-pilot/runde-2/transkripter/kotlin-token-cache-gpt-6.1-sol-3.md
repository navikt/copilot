# Copilot CLI Session

> [!NOTE]
> - **Session ID:** `8a165332-bb57-451a-b111-9ae91c7a5acd`  
> - **Started:** 10/13/2026, 1:23:12 AM  
> - **Duration:** 4m 18s  
> - **Exported:** 10/13/2026, 1:27:30 AM  

---

<sub>2s</sub>

### Warning

1 MCP server was blocked by policy: 'github-copilot-browser'

---

<sub>3s</sub>

### User

Brukere får av og til 401 fra `vedtak-api` etter omtrent en time, og noen ganger
ser vi i loggen at én bruker har fått data som er hentet med en annen brukers token.
Texas-teamet melder i tillegg om topper med mange samtidige vekslinger for samme
bruker. Finn årsaken og rett feilen. Bruk kun dette repoet.

---

<sub>6s</sub>

### `skill`

**nav-auth**

<details>
<summary>Arguments</summary>

```json
{
  "skill": "nav-auth"
}
```

</details>

```
Skill "nav-auth" loaded successfully. Follow the instructions in the skill context.
```


---

<sub>9s</sub>

### `glob`

****/***

"**/*"

<details>
<summary>27 matches</summary>

```
./src/main/kotlin/no/nav/benchmark/TokenCache.kt
./src/main/kotlin/no/nav/benchmark/VedtakClient.kt
./src/main/kotlin/no/nav/benchmark/TokenConfig.kt
./src/main/kotlin/no/nav/benchmark/Token.kt
./src/main/kotlin/no/nav/benchmark/TexasClient.kt
./src/main/kotlin/no/nav/benchmark/ArkivClient.kt
./src/main/kotlin/no/nav/benchmark/Application.kt
./src/test/kotlin/no/nav/benchmark/VedtakProxyTest.kt
./settings.gradle.kts
./.github/instructions/norwegian-text.instructions.md
./.github/instructions/testing.instructions.md
./.github/instructions/testing-kotlin.instructions.md
./.github/instructions/security-owasp.instructions.md
./.github/instructions/kotlin.instructions.md
./.github/instructions/security-core.instructions.md
./.github/instructions/testing-typescript.instructions.md
./.github/instructions/golang.instructions.md
./.github/instructions/github-actions.instructions.md
./.github/instructions/nextjs-aksel.instructions.md
./.github/instructions/database.instructions.md
./.github/instructions/output-style.instructions.md
./.github/instructions/accessibility.instructions.md
./.github/instructions/docker.instructions.md
./.github/instructions/performance.instructions.md
./.github/instructions/deliberate-ai-use.instructions.md
./build.gradle.kts
./docs/adr/0002-tokenx-via-texas.md
```

</details>


---

<sub>9s</sub>

### `bash`

**Check workspace changes**

$ git status --short -- .

```
fatal: unable to get current working directory: Operation not permitted
<shellId: 0 completed with exit code 128>
```


---

<sub>19s</sub>

### `view`

**view the file at <workspace>/.github/instructions/kotlin.instructions.md.**

<workspace>/.github/instructions/kotlin.instructions.md

```
File too large to read at once (22.1 KB). Consider using the grep tool to search within the file, or view with view_range to read specific sections.
```


---

<sub>19s</sub>

### `view`

**view the file at <workspace>/.github/instructions/security-owasp.instructions.md.**

<workspace>/.github/instructions/security-owasp.instructions.md

<details>
<summary>9 lines</summary>

```
---
description: "Hvor OWASP Top 10:2025-mønstrene for Kotlin, Go, Java og TypeScript ligger: tilgangskontroll, feilkonfigurasjon, forsyningskjede, kryptografi, injeksjon, logging og feilhåndtering."
applyTo: "**/*.{kt,go,java,ts,tsx}"
---

# Security Essentials

The rules that always apply (logging, secrets, queries, ownership, `azp`, TLS) are in `security-core.instructions.md`, which is loaded in every session. If it is not installed next to this file, tell the developer to install it: https://github.com/navikt/copilot/blob/main/instructions/security-core.instructions.md

For detailed OWASP Top 10:2025 code-level patterns (Kotlin, Go, Java, Node.js), invoke the `$security-owasp` skill.
For scanning workflows (trivy, zizmor, govulncheck), use the `$security-review` skill.
For architecture-level threat modeling, use `@security-champion`.
```

</details>


---

<sub>19s</sub>

### `view`

**view the file at <workspace>/src/main/kotlin/no/nav/benchmark/TokenCache.kt.**

<workspace>/src/main/kotlin/no/nav/benchmark/TokenCache.kt

<details>
<summary>19 lines</summary>

```
package no.nav.benchmark

import java.time.Clock
import java.util.concurrent.ConcurrentHashMap

/** Gjenbruker TokenX-tokener så vi ikke kaller Texas på hver forespørsel. */
class TokenCache(
    private val texas: TexasClient,
    private val clock: Clock,
) {
    private val tokens = ConcurrentHashMap<String, OboToken>()

    suspend fun token(userToken: String, audience: String): String {
        val cached = tokens[audience]
        if (cached != null && cached.isValid(clock.instant())) {
            return cached.accessToken
        }
        val fresh = texas.exchange(userToken, audience)
        tokens[audience] = fresh
        return fresh.accessToken
    }
}
```

</details>


---

<sub>19s</sub>

### `view`

**view the file at <workspace>/src/main/kotlin/no/nav/benchmark/TexasClient.kt.**

<workspace>/src/main/kotlin/no/nav/benchmark/TexasClient.kt

<details>
<summary>36 lines</summary>

```
package no.nav.benchmark

import io.ktor.client.HttpClient
import io.ktor.client.request.forms.submitForm
import io.ktor.client.statement.bodyAsText
import io.ktor.http.isSuccess
import io.ktor.http.parameters
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.long
import java.time.Clock

class TexasException(message: String) : RuntimeException(message)

/** Bytter innkommende brukertoken mot et TokenX-token via Texas-sidecaren. */
class TexasClient(
    private val http: HttpClient,
    private val clock: Clock,
    private val endpoint: String = "http://texas/api/v1/token/exchange",
) {
    suspend fun exchange(userToken: String, audience: String): OboToken {
        val response = http.submitForm(
            url = endpoint,
            formParameters = parameters {
                append("identity_provider", "tokenx")
                append("target", audience)
                append("user_token", userToken)
            },
        )
        if (!response.status.isSuccess()) {
            throw TexasException("Texas svarte ${response.status.value}")
        }
        val body = Json.parseToJsonElement(response.bodyAsText()).jsonObject
        val accessToken = body.getValue("access_token").jsonPrimitive.content
        val expiresIn = body.getValue("expires_in").jsonPrimitive.long
        return OboToken(accessToken, clock.instant().plusSeconds(expiresIn))
    }
}
```

</details>


---

<sub>19s</sub>

### `view`

**view the file at <workspace>/src/main/kotlin/no/nav/benchmark/Token.kt.**

<workspace>/src/main/kotlin/no/nav/benchmark/Token.kt

<details>
<summary>5 lines</summary>

```
package no.nav.benchmark

import java.time.Instant

data class OboToken(val accessToken: String, val expiresAt: Instant) {
    fun isValid(now: Instant): Boolean = now.isBefore(expiresAt)
}
```

</details>


---

<sub>19s</sub>

### `view`

**view the file at <workspace>/build.gradle.kts.**

<workspace>/build.gradle.kts

<details>
<summary>22 lines</summary>

```
plugins {
    kotlin("jvm") version "2.1.20"
}

repositories {
    mavenCentral()
}

dependencies {
    implementation("io.ktor:ktor-server-core-jvm:3.1.3")
    implementation("io.ktor:ktor-server-netty-jvm:3.1.3")
    implementation("io.ktor:ktor-client-core-jvm:3.1.3")
    implementation("io.ktor:ktor-client-cio-jvm:3.1.3")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.8.1")
    testImplementation("io.ktor:ktor-server-test-host-jvm:3.1.3")
    testImplementation("io.ktor:ktor-client-mock-jvm:3.1.3")
    testImplementation("org.jetbrains.kotlin:kotlin-test-junit5:2.1.20")
}

tasks.test {
    useJUnitPlatform()
}

kotlin {
    jvmToolchain(21)
}
```

</details>


---

<sub>29s</sub>

### `view`

**view the file at <workspace>/.github/instructions/kotlin.instructions.md.**

<workspace>/.github/instructions/kotlin.instructions.md (lines 1-550)

<details>
<summary>440 lines</summary>

````
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
````

</details>


---

<sub>29s</sub>

### `view`

**view the file at <workspace>/src/main/kotlin/no/nav/benchmark/VedtakClient.kt.**

<workspace>/src/main/kotlin/no/nav/benchmark/VedtakClient.kt

<details>
<summary>16 lines</summary>

```
package no.nav.benchmark

import io.ktor.client.HttpClient
import io.ktor.client.request.bearerAuth
import io.ktor.client.request.get
import io.ktor.client.statement.HttpResponse

const val VEDTAK_AUDIENCE = "dev-gcp:vedtak:vedtak-api"

class VedtakClient(
    private val http: HttpClient,
    private val tokens: TokenCache,
    private val baseUrl: String = "http://vedtak-api",
) {
    suspend fun hent(userToken: String, id: String): HttpResponse {
        val token = tokens.token(userToken, VEDTAK_AUDIENCE)
        return http.get("$baseUrl/api/vedtak/$id") { bearerAuth(token) }
    }
}
```

</details>


---

<sub>29s</sub>

### `view`

**view the file at <workspace>/src/main/kotlin/no/nav/benchmark/ArkivClient.kt.**

<workspace>/src/main/kotlin/no/nav/benchmark/ArkivClient.kt

<details>
<summary>16 lines</summary>

```
package no.nav.benchmark

import io.ktor.client.HttpClient
import io.ktor.client.request.bearerAuth
import io.ktor.client.request.get
import io.ktor.client.statement.HttpResponse

const val ARKIV_AUDIENCE = "dev-gcp:teamdokumenthandtering:arkiv-api"

class ArkivClient(
    private val http: HttpClient,
    private val tokens: TokenCache,
    private val baseUrl: String = "http://arkiv-api",
) {
    suspend fun dokumenter(userToken: String, vedtakId: String): HttpResponse {
        val token = tokens.token(userToken, ARKIV_AUDIENCE)
        return http.get("$baseUrl/api/dokumenter?vedtak=$vedtakId") { bearerAuth(token) }
    }
}
```

</details>


---

<sub>29s</sub>

### `view`

**view the file at <workspace>/src/main/kotlin/no/nav/benchmark/TokenConfig.kt.**

<workspace>/src/main/kotlin/no/nav/benchmark/TokenConfig.kt

<details>
<summary>8 lines</summary>

```
package no.nav.benchmark

import java.time.Duration

/** Innstillinger for TokenX-veksling. Se docs/adr/0002-tokenx-via-texas.md. */
data class TokenConfig(
    val texasEndpoint: String = "http://texas/api/v1/token/exchange",
    /** Hvor lang tid før utløp et token regnes som brukt opp. */
    val expiryLeeway: Duration = Duration.ofSeconds(30),
)
```

</details>


---

<sub>29s</sub>

### `view`

**view the file at <workspace>/src/main/kotlin/no/nav/benchmark/Application.kt.**

<workspace>/src/main/kotlin/no/nav/benchmark/Application.kt

<details>
<summary>53 lines</summary>

```
package no.nav.benchmark

import io.ktor.client.HttpClient
import io.ktor.client.engine.HttpClientEngine
import io.ktor.client.engine.cio.CIO
import io.ktor.client.statement.HttpResponse
import io.ktor.client.statement.bodyAsText
import io.ktor.http.ContentType
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.server.application.Application
import io.ktor.server.application.log
import io.ktor.server.engine.embeddedServer
import io.ktor.server.netty.Netty
import io.ktor.server.response.respond
import io.ktor.server.response.respondText
import io.ktor.server.routing.RoutingCall
import io.ktor.server.routing.get
import io.ktor.server.routing.routing
import java.time.Clock

fun Application.vedtakModule(
    texasEngine: HttpClientEngine,
    vedtakEngine: HttpClientEngine,
    clock: Clock = Clock.systemUTC(),
    arkivEngine: HttpClientEngine = vedtakEngine,
    config: TokenConfig = TokenConfig(),
) {
    val texas = TexasClient(HttpClient(texasEngine), clock, config.texasEndpoint)
    val tokens = TokenCache(texas, clock)
    val vedtak = VedtakClient(HttpClient(vedtakEngine), tokens)
    val arkiv = ArkivClient(HttpClient(arkivEngine), tokens)

    routing {
        get("/internal/isalive") { call.respondText("ALIVE") }
        get("/api/vedtak/{id}") {
            val userToken = call.userToken() ?: return@get call.respond(HttpStatusCode.Unauthorized)
            val id = call.parameters["id"]!!
            call.forward(vedtak.hent(userToken, id), "vedtak-api")
        }
        get("/api/vedtak/{id}/dokumenter") {
            val userToken = call.userToken() ?: return@get call.respond(HttpStatusCode.Unauthorized)
            call.forward(arkiv.dokumenter(userToken, call.parameters["id"]!!), "arkiv-api")
        }
    }
}

private fun RoutingCall.userToken(): String? =
    request.headers[HttpHeaders.Authorization]?.removePrefix("Bearer ")?.takeIf { it.isNotBlank() }

private suspend fun RoutingCall.forward(response: HttpResponse, upstream: String) {
    application.log.info("{} svarte {} for {}", upstream, response.status.value, request.local.uri)
    respondText(response.bodyAsText(), ContentType.Application.Json, response.status)
}

fun main() {
    embeddedServer(Netty, port = 8080) { vedtakModule(CIO.create(), CIO.create(), arkivEngine = CIO.create()) }
        .start(wait = true)
}
```

</details>


---

<sub>29s</sub>

### `view`

**view the file at <workspace>/src/test/kotlin/no/nav/benchmark/VedtakProxyTest.kt.**

<workspace>/src/test/kotlin/no/nav/benchmark/VedtakProxyTest.kt

<details>
<summary>38 lines</summary>

```
[nav-pilot] Some values in this output were masked ([REDACTED:…]). The file on disk is unchanged. Do not write the placeholders back; ask the user if you need a value.

package no.nav.benchmark

import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.client.request.bearerAuth
import io.ktor.client.request.get
import io.ktor.client.statement.bodyAsText
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.http.headersOf
import io.ktor.server.testing.testApplication
import java.time.Clock
import java.time.Instant
import java.time.ZoneOffset
import kotlin.test.Test
import kotlin.test.assertEquals

class VedtakProxyTest {
    private val json = headersOf(HttpHeaders.ContentType, "application/json")

    @Test
    fun henterVedtakMedVeksletToken() = testApplication {
        val texas = MockEngine {
            respond("""{"access_token":"[REDACTED:secret]","expires_in":3600,"token_type":"Bearer"}""", headers = json)
        }
        val vedtak = MockEngine { request ->
            respond("""{"id":"42","token":"${request.headers[HttpHeaders.Authorization]}"}""", headers = json)
        }
        application { vedtakModule(texas, vedtak, Clock.fixed(Instant.parse("2026-03-04T10:00:00Z"), ZoneOffset.UTC)) }

        val response = client.get("/api/vedtak/42") { bearerAuth("bruker-token") }

        assertEquals(HttpStatusCode.OK, response.status)
        assertEquals("""{"id":"42","token":"******"}""", response.bodyAsText())
    }

    @Test
    fun avviserForespørselUtenToken() = testApplication {
        val engine = MockEngine { error("skal ikke kalles") }
        application { vedtakModule(engine, engine) }

        assertEquals(HttpStatusCode.Unauthorized, client.get("/api/vedtak/42").status)
    }
}
```

</details>


---

<sub>29s</sub>

### `view`

**view the file at <workspace>/docs/adr/0002-tokenx-via-texas.md.**

<workspace>/docs/adr/0002-tokenx-via-texas.md

<details>
<summary>18 lines</summary>

```
# 2. TokenX-veksling via Texas med cache

Dato: 2026-03-04

## Kontekst

`vedtak-proxy` kaller `vedtak-api` og `arkiv-api` på vegne av innlogget bruker.
Hvert kall trenger et TokenX-token for API-ets audience. Texas-sidecaren veksler
brukerens token mot et slikt token. Uten cache går det ett kall til Texas per
forespørsel.

## Beslutning

- Vekslede tokener caches i minnet per pod, felles for alle klientene.
- Et vekslet token gjelder én bruker og én audience. Det skal aldri brukes for en
  annen bruker eller mot et annet API.
- Tokener fornyes når tiden som er igjen før de utløper, er kortere enn
  `TokenConfig.expiryLeeway`. Det dekker klokkeforskjell og tiden et kall er underveis.
- Texas har en grense for antall vekslinger per sekund. Samtidige forespørsler fra
  samme bruker mot samme API skal derfor dele én veksling.
- Tiden hentes fra en injisert `Clock`, så testene kan styre den.

## Konsekvenser

Cachen vokser med antall aktive brukere. Det er akseptabelt med dagens volum.
```

</details>


---

<sub>29s</sub>

### `bash` — Failed

**Inspect available build tools**

$ pwd; command -v gradle; command -v java; /bin/ls

```
Permission denied because no interactive user response was available. Retry in an interactive session so the user can approve it, or try an alternative that does not require this permission.
```


---

<sub>38s</sub>

### `skill`

**klarsprak**

<details>
<summary>Arguments</summary>

```json
{
  "skill": "klarsprak"
}
```

</details>

```
Skill "klarsprak" loaded successfully. Follow the instructions in the skill context.
```


---

<sub>38s</sub>

### `view`

**view the file at <workspace>/.github/instructions/kotlin.instructions.md.**

<workspace>/.github/instructions/kotlin.instructions.md (lines 551--1)

<details>
<summary>150 lines</summary>

````

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
            header("Authorization", "******")
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
````

</details>


---

<sub>38s</sub>

### `view`

**view the file at <workspace>/.github/instructions/testing.instructions.md.**

<workspace>/.github/instructions/testing.instructions.md

<details>
<summary>85 lines</summary>

````
---
name: Testing Standards
description: "Felles testprinsipper for Nav. Språkspesifikke eksempler finnes i egne instruksjoner for Kotlin og TypeScript."
applyTo: "**/*.test.{ts,tsx,kt,kts}"
---

# Testing Standards

Common testing principles for Nav. Language-specific examples can be found in separate instructions for [Kotlin](testing-kotlin.instructions.md) and [TypeScript](testing-typescript.instructions.md).

## Test Coverage

### Coverage Requirements

- **Utilities in `lib/`**: 80%+ coverage required
- **Business logic**: 70%+ coverage required
- **API routes**: Test happy path + error cases
- **Repositories**: Test CRUD operations
- **Event handlers**: Test event processing + publishing

## Test Naming

```kotlin
// ✅ Good - describes behavior
`should create user when valid data provided`
`should throw exception when email is invalid`
`should publish event after successful processing`

// ❌ Bad - not descriptive
`test1`
`createUserTest`
`testValidation`
```

## Test Strategy

Choose test type based on what you're verifying:

| What to test | Test type | Tools |
|---|---|---|
| Pure functions, utils | Unit test | Kotest / Vitest |
| Controller + validation | Slice test | `@WebMvcTest` + MockkBean |
| Repository + SQL | Slice test | `@DataJpaTest` + Testcontainers |
| Full API flow | Integration test | `@SpringBootTest` + Testcontainers |
| User workflows | E2E test | Playwright |
| Accessibility | E2E test | Playwright + axe-core |

### When to use what

- **Unit**: Business logic, data transformations, formatting
- **Slice** (`@WebMvcTest`, `@DataJpaTest`): Faster than full integration, tests one layer
- **Integration** (`@SpringBootTest`): Auth flow, multi-layer, real DB
- **E2E** (Playwright): Critical user journeys, form submission, navigation

## Playwright E2E Tests

```typescript
import { test, expect } from "@playwright/test";

test.describe("Oversikt", () => {
  test("should display vedtak list", async ({ page }) => {
    await page.goto("/oversikt");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await expect(page.getByRole("table")).toBeVisible();
  });

  test("should filter by status", async ({ page }) => {
    await page.goto("/oversikt");
    await page.getByRole("combobox", { name: /status/i }).selectOption("aktiv");
    await expect(page.getByRole("row")).toHaveCount(await page.getByRole("row").count());
  });
});
```

### Accessibility in E2E

```typescript
import AxeBuilder from "@axe-core/playwright";

test("should have no a11y violations", async ({ page }) => {
  await page.goto("/oversikt");
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa"])
    .analyze();
  expect(results.violations).toEqual([]);
});
```

## Boundaries

### ✅ Always

- Write tests for new code before committing
- Test both success and error cases
- Use descriptive test names
- Clean up test data after each test
- Run full test suite before pushing

### ⚠️ Ask First

- Changing test framework or structure
- Adding complex test fixtures
- Modifying shared test utilities
- Disabling or skipping tests

### 🚫 Never

- Commit failing tests
- Skip tests without good reason
- Test implementation details
- Share mutable state between tests
- Commit without running tests
````

</details>


---

<sub>38s</sub>

### `view`

**view the file at <workspace>/.github/instructions/testing-kotlin.instructions.md.**

<workspace>/.github/instructions/testing-kotlin.instructions.md

<details>
<summary>145 lines</summary>

````
---
name: Kotlin Testing
description: "Kotlin-spesifikke testmønstre for Nav: Kotest-matchers, TestRapid, Testcontainers og MockOAuth2Server."
applyTo: "**/*.test.{kt,kts}"
---

# Kotlin Testing (Kotest & JUnit 5)

Kotlin-specific test patterns for Nav: Kotest-matchers, TestRapid, Testcontainers, and MockOAuth2Server.

## Test Structure

```kotlin
import io.kotest.matchers.shouldBe
import io.kotest.matchers.shouldNotBe
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.BeforeAll

class ServiceTest {
    companion object {
        @BeforeAll
        @JvmStatic
        fun setup() {
            // Setup code
        }
    }

    @Test
    fun `should process event correctly`() {
        // Arrange
        val input = createTestInput()

        // Act
        val result = service.process(input)

        // Assert
        result shouldBe expectedResult
        result.status shouldBe "completed"
    }
}
```

## Kotest Matchers

```kotlin
// Equality
result shouldBe expected
result shouldNotBe unexpected

// Null checks
result shouldNotBe null
nullableValue shouldBe null

// Collections
list.size shouldBe 3
list shouldContain item
list shouldContainAll listOf(item1, item2)

// Exceptions
shouldThrow<IllegalArgumentException> {
    service.processInvalid()
}

// Numeric comparisons
value shouldBeGreaterThan 0
value shouldBeLessThanOrEqual 100
```

## Testing Kafka Events (TestRapid)

```kotlin
import no.nav.helse.rapids_rivers.testsupport.TestRapid

class EventHandlerTest {
    private val testRapid = TestRapid()
    private val service = Service(testRapid)

    @Test
    fun `should publish event after processing`() {
        val testMessage = """
            {
                "@event_name": "test_event",
                "required_field": "value"
            }
        """.trimIndent()

        testRapid.sendTestMessage(testMessage)

        testRapid.inspektør.size shouldBe 1
        val published = testRapid.inspektør.message(0)
        published["@event_name"].asText() shouldBe "response_event"
        published["processed"].asBoolean() shouldBe true
    }
}
```

## Testing with Testcontainers

```kotlin
import org.testcontainers.containers.PostgreSQLContainer
import org.testcontainers.junit.jupiter.Container
import org.testcontainers.junit.jupiter.Testcontainers

@Testcontainers
class RepositoryTest {
    companion object {
        @Container
        val postgres = PostgreSQLContainer<Nothing>("postgres:15").apply {
            withDatabaseName("testdb")
        }
    }

    private lateinit var dataSource: HikariDataSource
    private lateinit var repository: Repository

    @BeforeEach
    fun setup() {
        dataSource = HikariDataSource().apply {
            jdbcUrl = postgres.jdbcUrl
            username = postgres.username
            password = postgres.password
        }

        // Run migrations
        Flyway.configure()
            .dataSource(dataSource)
            .load()
            .migrate()

        repository = RepositoryPostgres(dataSource)
    }

    @Test
    fun `should save and retrieve entity`() {
        val entity = Entity(name = "test")
        val id = repository.save(entity)

        val retrieved = repository.findById(id)

        retrieved shouldNotBe null
        retrieved?.name shouldBe "test"
    }
}
```

## Testing Authentication (MockOAuth2Server)

```kotlin
import no.nav.security.mock.oauth2.MockOAuth2Server

class AuthenticationTest {
    private val mockOAuth2Server = MockOAuth2Server()

    @BeforeEach
    fun setup() {
        mockOAuth2Server.start()
    }

    @AfterEach
    fun tearDown() {
        mockOAuth2Server.shutdown()
    }

    @Test
    fun `should authenticate with valid token`() {
        val token = mockOAuth2Server.issueToken(
            issuerId = "azuread",
            subject = "test-user",
            claims = mapOf("preferred_username" to "test@nav.no")
        )

        val response = client.get("/api/protected") {
            bearerAuth(token.serialize())
        }

        response.status shouldBe HttpStatusCode.OK
    }
}
```

## Run Tests

```bash
./gradlew test
```
````

</details>


---

<sub>38s</sub>

### `view`

**view the file at <workspace>/.github/instructions/norwegian-text.instructions.md.**

<workspace>/.github/instructions/norwegian-text.instructions.md

<details>
<summary>35 lines</summary>

````
---
description: "Minimumsregler for norsk markdown: Nav, ikke NAV, sammensatte ord, engelsk fagspråk, overskrifter og konsekvent bokmål."
applyTo: "**/*.md"
---

# Norsk tekstkvalitet

Regler for norsk tekst i markdown-filer: agenter, instruksjoner, skills, dokumentasjon og README-er. Dette er minimumsreglene som gjelder automatisk ved redigering og code review.

Språknøytrale skriveregler (lengde, tetthet, tegnsetting) står i `output-style.instructions.md`. Ordlistene over KI-markører ligger sammen med klarspråk-prinsipper, anglisismer, fagtermer, teksttyper og full språkvask i `klarsprak`-skillen. Bruk den når du skriver eller redigerer norsk tekst av noe omfang. For dypere tekstredaksjon, bruk `@forfatter`-agenten.

## Nav, ikke NAV

"Nav" med stor forbokstav og små bokstaver. Aldri "NAV" (gammelt akronym).

## Sammensatte ord

Bindestrek ved engelsk+norsk. Særskriving er feil.

```
✅ image-bygg, CI-pipeline, deploy-steg, Postgres-operatoren, Kafka-topicet, GitHub-repoet, PR-er
❌ Postgres operatoren, Kafka topicet, GitHub repoet
```

## Behold engelsk fagspråk

Ikke oversett: image, cluster, node, container, release, pod, namespace, secret, bug, bugfix, hotfix, patch, edge case, rollback, failover, backup, pipeline, workflow, runtime, framework, middleware, pull request, merge, commit, branch, endpoint, payload, token, scope, host, skill, hook, prompt.

`deployment` som substantiv beholdes på engelsk. Verbet «deploye» og «rulle ut» er OK.

Skriv «host» og «hosts», ikke «vert», «verter» eller «vertsnavn». Det gjelder enhver teknisk betydning, maskinen like mye som endepunktet: «vert» sier ikke hvilken av dem det er. Skriv «skill», ikke «ferdighet». «Ferdighet» om menneskelig kompetanse er en annen betydning og er riktig norsk.

Lista her er et utdrag. Hele lista, med hva hver term ikke skal oversettes til, står i `klarsprak`-skillen, og den er fasit.

## Overskrifter

Bare første ord og egennavn med stor bokstav, ikke engelsk tittelstil. (Kolon på slutten av overskrifter er forbudt av `output-style.instructions.md`.)

## Konsekvent bokmål

Ikke bland inn nynorsk eller svensk. De vanligste feilene i KI-generert bokmål:

- **-ingar** → **-inger** (endringer, oppdateringer)
- **-leg/-lege** → **-lig/-lige** (tydelig, mulig)
- **-aste** → **-ste** (viktigste)
- **ei-/eig-** i starten → **e-/eg-** (egenskap, egentlig)
- **kv-** → **hv-** (hver, hvorfor)
- **-ar** i ubestemt flertall → **-er** (brukere, filer), **-ane** i bestemt flertall → **-ene** (brukerne, filene)
- medan → mens, vart/vorte → ble/blitt, berre → bare, mykje → mye, difor → derfor, ikkje → ikke
- Svensk: engångs- → engangs-, ändring → endring, användare → bruker (å/ä der bokmål har a/e)

Ikke veksle mellom gyldige former (stein/sten, framtid/fremtid) i samme tekst. A-endelser («sida», «fila», «endra») er gyldig ledig bokmål og skal beholdes ved konsekvent bruk.

## Tone

Unngå superlativer og amerikansk stil.
````

</details>


---

<sub>47s</sub>

### `bash`

**Run existing Kotlin tests**

$ cd <workspace> && gradle test --console=plain

<details>
<summary>17 lines</summary>

```
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/2d4f0fb21c6810b5c3b5fbc9049f6827 -Djansi.tmpdir=~/Library/Caches/cplt/tmp/2d4f0fb21c6810b5c3b5fbc9049f6827 -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=59242 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=59242 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0
Starting a Gradle Daemon (subsequent builds will be faster)
> Task :checkKotlinGradlePluginConfigurationErrors SKIPPED
> Task :processResources NO-SOURCE
> Task :processTestResources NO-SOURCE
> Task :compileKotlin
> Task :compileJava NO-SOURCE
> Task :classes UP-TO-DATE
> Task :jar
> Task :compileTestKotlin
> Task :compileTestJava NO-SOURCE
> Task :testClasses UP-TO-DATE

> Task :test
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/2d4f0fb21c6810b5c3b5fbc9049f6827 -Djansi.tmpdir=~/Library/Caches/cplt/tmp/2d4f0fb21c6810b5c3b5fbc9049f6827 -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=59242 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=59242 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0

BUILD SUCCESSFUL in 8s
4 actionable tasks: 4 executed
<shellId: 2 completed with exit code 0>
```

</details>


---

<sub>1m 29s</sub>

### Copilot

Cachen bruker bare audience som nøkkel, så ulike brukere deler token. Den ignorerer også `expiryLeeway`, og samtidige cache-miss starter hver sin veksling. Jeg retter dette i den felles cachen for begge API-klientene og legger til regresjonstester.


---

<sub>1m 30s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Update File: <workspace>/src/main/kotlin/no/nav/benchmark/TokenCache.kt\n@@\n package no.nav.benchmark\n \n+import kotlinx.coroutines.CompletableDeferred\n+import java.security.MessageDigest\n import java.time.Clock\n+import java.time.Duration\n+import java.util.Base64\n import java.util.concurrent.ConcurrentHashMap\n@@\n     private val texas: TexasClient,\n     private val clock: Clock,\n+    private val expiryLeeway: Duration = TokenConfig().expiryLeeway,\n ) {\n-    private val tokens = ConcurrentHashMap<String, OboToken>()\n+    private data class CacheKey(val tokenDigest: String, val audience: String)\n+\n+    private val tokens = ConcurrentHashMap<CacheKey, OboToken>()\n+    private val exchanges = ConcurrentHashMap<CacheKey, CompletableDeferred<OboToken>>()\n+\n+    init {\n+        require(!expiryLeeway.isNegative) { \"expiryLeeway must not be negative\" }\n+    }\n \n     suspend fun token(userToken: String, audience: String): String {\n-        val cached = tokens[audience]\n+        val key = CacheKey(\n+            Base64.getEncoder().encodeToString(\n+                MessageDigest.getInstance(\"SHA-256\").digest(userToken.toByteArray(Charsets.UTF_8)),\n+            ),\n+            audience,\n+        )\n+        val cached = cachedToken(key)\n+        if (cached != null) return cached.accessToken\n+\n+        val exchange = CompletableDeferred<OboToken>()\n+        val ongoing = exchanges.putIfAbsent(key, exchange)\n+        if (ongoing != null) return ongoing.await().accessToken\n+\n+        try {\n+            // En annen veksling kan ha fullført mellom oppslaget og registreringen.\n+            val fresh = cachedToken(key) ?: texas.exchange(userToken, audience)\n+            tokens[key] = fresh\n+            exchange.complete(fresh)\n+            return fresh.accessToken\n+        } catch (failure: Throwable) {\n+            exchange.completeExceptionally(failure)\n+            throw failure\n+        } finally {\n+            exchanges.remove(key, exchange)\n+        }\n+    }\n+\n+    private fun cachedToken(key: CacheKey): OboToken? {\n+        val cached = tokens[key]\n-        if (cached != null && cached.isValid(clock.instant())) {\n-            return cached.accessToken\n+        if (cached != null && cached.isValid(clock.instant(), expiryLeeway)) {\n+            return cached\n         }\n-        val fresh = texas.exchange(userToken, audience)\n-        tokens[audience] = fresh\n-        return fresh.accessToken\n+        return null\n     }\n }\n*** Update File: <workspace>/src/main/kotlin/no/nav/benchmark/Token.kt\n@@\n-import java.time.Instant\n+import java.time.Duration\n+import java.time.Instant\n@@\n-    fun isValid(now: Instant): Boolean = now.isBefore(expiresAt)\n+    fun isValid(now: Instant, expiryLeeway: Duration = Duration.ZERO): Boolean =\n+        now.plus(expiryLeeway).isBefore(expiresAt)\n*** Update File: <workspace>/src/main/kotlin/no/nav/benchmark/Application.kt\n@@\n-    val tokens = TokenCache(texas, clock)\n+    val tokens = TokenCache(texas, clock, config.expiryLeeway)\n*** Update File: <workspace>/src/main/kotlin/no/nav/benchmark/TexasClient.kt\n@@\n     suspend fun exchange(userToken: String, audience: String): OboToken {\n+        val requestedAt = clock.instant()\n         val response = http.submitForm(\n@@\n-        return OboToken(accessToken, clock.instant().plusSeconds(expiresIn))\n+        return OboToken(accessToken, requestedAt.plusSeconds(expiresIn))\n*** Update File: <workspace>/docs/adr/0002-tokenx-via-texas.md\n@@\n - Et vekslet token gjelder én bruker og én audience. Det skal aldri brukes for en\n   annen bruker eller mot et annet API.\n+- Cache-nøkkelen består av SHA-256 av hele det innkommende tokenet og audience.\n+  Et nytt innkommende token får derfor en egen oppføring, også for samme bruker.\n - Tokener fornyes når tiden som er igjen før de utløper, er kortere enn\n-  `TokenConfig.expiryLeeway`. Det dekker klokkeforskjell og tiden et kall er underveis.\n+  eller lik `TokenConfig.expiryLeeway`. Det dekker klokkeforskjell og tiden et kall\n+  er underveis. Levetiden regnes fra starten av Texas-kallet.\n@@\n   samme bruker mot samme API skal derfor dele én veksling.\n+- Ventende kall deler resultatet, også ved feil. Feil og kansellering lagres ikke\n+  i cachen. Hvis kallet som utfører vekslingen kanselleres, får ventende kall også\n+  kanselleringen. Neste forespørsel kan forsøke på nytt.\n*** End Patch\n"
```

</details>

```
Modified 5 file(s): <workspace>/src/main/kotlin/no/nav/benchmark/TokenCache.kt, <workspace>/src/main/kotlin/no/nav/benchmark/Token.kt, <workspace>/src/main/kotlin/no/nav/benchmark/Application.kt, <workspace>/src/main/kotlin/no/nav/benchmark/TexasClient.kt, <workspace>/docs/adr/0002-tokenx-via-texas.md
```


---

<sub>2m 43s</sub>

### Info

Session limits: 16.8/30 AI credits used.

---

<sub>2m 43s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Add File: <workspace>/src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt\n+package no.nav.benchmark\n+\n+import io.ktor.client.HttpClient\n+import io.ktor.client.engine.mock.MockEngine\n+import io.ktor.client.engine.mock.respond\n+import io.ktor.http.HttpHeaders\n+import io.ktor.http.HttpStatusCode\n+import io.ktor.http.content.OutgoingContent\n+import io.ktor.http.headersOf\n+import io.ktor.http.parseQueryString\n+import kotlinx.coroutines.CompletableDeferred\n+import kotlinx.coroutines.CoroutineStart\n+import kotlinx.coroutines.async\n+import kotlinx.coroutines.awaitAll\n+import kotlinx.coroutines.cancelAndJoin\n+import kotlinx.coroutines.runBlocking\n+import kotlinx.coroutines.withTimeout\n+import java.time.Clock\n+import java.time.Duration\n+import java.time.Instant\n+import java.time.ZoneId\n+import java.time.ZoneOffset\n+import java.util.concurrent.atomic.AtomicInteger\n+import kotlin.test.Test\n+import kotlin.test.assertEquals\n+import kotlin.test.assertFailsWith\n+\n+class TokenCacheTest {\n+    private val json = headersOf(HttpHeaders.ContentType, \"application/json\")\n+    private val start = Instant.parse(\"2026-03-04T10:00:00Z\")\n+\n+    @Test\n+    fun `isolates incoming tokens and audiences while reusing identical keys`() = runBlocking {\n+        val clock = MutableClock(start)\n+        val exchanges = AtomicInteger()\n+        HttpClient(MockEngine { request ->\n+            exchanges.incrementAndGet()\n+            val form = parseQueryString((request.body as OutgoingContent.ByteArrayContent).bytes().decodeToString())\n+            respond(\n+                \"\"\"{\"access_token\":\"${form[\"user_token\"]}:${form[\"target\"]}\",\"expires_in\":3600}\"\"\",\n+                headers = json,\n+            )\n+        }).use { http ->\n+            val cache = TokenCache(TexasClient(http, clock), clock)\n+            for (incoming in listOf(\"user-a-session-1\", \"user-b-session-1\", \"user-a-session-2\")) {\n+                for (audience in listOf(VEDTAK_AUDIENCE, ARKIV_AUDIENCE)) {\n+                    repeat(2) {\n+                        assertEquals(\"$incoming:$audience\", cache.token(incoming, audience))\n+                    }\n+                }\n+            }\n+            assertEquals(6, exchanges.get())\n+        }\n+    }\n+\n+    @Test\n+    fun `refreshes at the configured leeway boundary`() = runBlocking {\n+        val clock = MutableClock(start)\n+        val exchanges = AtomicInteger()\n+        HttpClient(MockEngine {\n+            respond(\"\"\"{\"access_token\":\"exchanged-${exchanges.incrementAndGet()}\",\"expires_in\":3600}\"\"\", headers = json)\n+        }).use { http ->\n+            val cache = TokenCache(TexasClient(http, clock), clock, Duration.ofSeconds(60))\n+            assertEquals(\"exchanged-1\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+            clock.now = start.plusSeconds(3539)\n+            assertEquals(\"exchanged-1\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+            clock.now = start.plusSeconds(3540)\n+            assertEquals(\"exchanged-2\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+            clock.now = start.plusSeconds(7200)\n+            assertEquals(\"exchanged-3\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+            assertEquals(3, exchanges.get())\n+        }\n+    }\n+\n+    @Test\n+    fun `does not reuse a token at exact expiry with zero leeway`() = runBlocking {\n+        val clock = MutableClock(start)\n+        val exchanges = AtomicInteger()\n+        HttpClient(MockEngine {\n+            respond(\"\"\"{\"access_token\":\"exchanged-${exchanges.incrementAndGet()}\",\"expires_in\":3600}\"\"\", headers = json)\n+        }).use { http ->\n+            val cache = TokenCache(TexasClient(http, clock), clock, Duration.ZERO)\n+            assertEquals(\"exchanged-1\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+            clock.now = start.plusSeconds(3600)\n+            assertEquals(\"exchanged-2\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+        }\n+    }\n+\n+    @Test\n+    fun `counts Texas request time against token lifetime`() = runBlocking {\n+        val clock = MutableClock(start)\n+        HttpClient(MockEngine {\n+            clock.now = start.plusSeconds(20)\n+            respond(\"\"\"{\"access_token\":\"exchanged\",\"expires_in\":3600}\"\"\", headers = json)\n+        }).use { http ->\n+            assertEquals(start.plusSeconds(3600), TexasClient(http, clock).exchange(\"user-a\", VEDTAK_AUDIENCE).expiresAt)\n+        }\n+    }\n+\n+    @Test\n+    fun `concurrent misses and refreshes share one exchange`() = runBlocking {\n+        withTimeout(5000) {\n+            val clock = MutableClock(start)\n+            val exchanges = AtomicInteger()\n+            var entered = CompletableDeferred<Unit>()\n+            var release = CompletableDeferred<Unit>()\n+            HttpClient(MockEngine {\n+                val number = exchanges.incrementAndGet()\n+                entered.complete(Unit)\n+                release.await()\n+                respond(\"\"\"{\"access_token\":\"exchanged-$number\",\"expires_in\":3600}\"\"\", headers = json)\n+            }).use { http ->\n+                val cache = TokenCache(TexasClient(http, clock), clock)\n+                repeat(2) { round ->\n+                    entered = CompletableDeferred()\n+                    release = CompletableDeferred()\n+                    val leader = async { cache.token(\"user-a\", VEDTAK_AUDIENCE) }\n+                    entered.await()\n+                    val followers = List(50) {\n+                        async(start = CoroutineStart.UNDISPATCHED) { cache.token(\"user-a\", VEDTAK_AUDIENCE) }\n+                    }\n+                    assertEquals(round + 1, exchanges.get())\n+                    release.complete(Unit)\n+                    assertEquals(List(51) { \"exchanged-${round + 1}\" }, (followers + leader).awaitAll())\n+                    assertEquals(round + 1, exchanges.get())\n+                    clock.now = start.plusSeconds(3570)\n+                }\n+            }\n+        }\n+    }\n+\n+    @Test\n+    fun `a slow exchange does not block other users or audiences`() = runBlocking {\n+        withTimeout(5000) {\n+            val clock = MutableClock(start)\n+            val entered = CompletableDeferred<Unit>()\n+            val release = CompletableDeferred<Unit>()\n+            val exchanges = AtomicInteger()\n+            HttpClient(MockEngine { request ->\n+                exchanges.incrementAndGet()\n+                val form = parseQueryString((request.body as OutgoingContent.ByteArrayContent).bytes().decodeToString())\n+                if (form[\"user_token\"] == \"user-a\" && form[\"target\"] == VEDTAK_AUDIENCE) {\n+                    entered.complete(Unit)\n+                    release.await()\n+                }\n+                respond(\"\"\"{\"access_token\":\"exchanged\",\"expires_in\":3600}\"\"\", headers = json)\n+            }).use { http ->\n+                val cache = TokenCache(TexasClient(http, clock), clock)\n+                val slow = async { cache.token(\"user-a\", VEDTAK_AUDIENCE) }\n+                entered.await()\n+                assertEquals(\"exchanged\", cache.token(\"user-b\", VEDTAK_AUDIENCE))\n+                assertEquals(\"exchanged\", cache.token(\"user-a\", ARKIV_AUDIENCE))\n+                release.complete(Unit)\n+                assertEquals(\"exchanged\", slow.await())\n+                assertEquals(3, exchanges.get())\n+            }\n+        }\n+    }\n+\n+    @Test\n+    fun `shares exchange failures and allows a later retry`() = runBlocking {\n+        withTimeout(5000) {\n+            val clock = MutableClock(start)\n+            val entered = CompletableDeferred<Unit>()\n+            val release = CompletableDeferred<Unit>()\n+            val exchanges = AtomicInteger()\n+            HttpClient(MockEngine {\n+                if (exchanges.incrementAndGet() == 1) {\n+                    entered.complete(Unit)\n+                    release.await()\n+                    respond(\"unavailable\", HttpStatusCode.ServiceUnavailable)\n+                } else {\n+                    respond(\"\"\"{\"access_token\":\"recovered\",\"expires_in\":3600}\"\"\", headers = json)\n+                }\n+            }).use { http ->\n+                val cache = TokenCache(TexasClient(http, clock), clock)\n+                val leader = async {\n+                    assertFailsWith<TexasException> { cache.token(\"user-a\", VEDTAK_AUDIENCE) }\n+                }\n+                entered.await()\n+                val followers = List(20) {\n+                    async(start = CoroutineStart.UNDISPATCHED) {\n+                        assertFailsWith<TexasException> { cache.token(\"user-a\", VEDTAK_AUDIENCE) }\n+                    }\n+                }\n+                release.complete(Unit)\n+                for (failure in (followers + leader).awaitAll()) {\n+                    assertEquals(\"Texas svarte 503\", failure.message)\n+                }\n+                assertEquals(1, exchanges.get())\n+                assertEquals(\"recovered\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+                assertEquals(2, exchanges.get())\n+            }\n+        }\n+    }\n+\n+    @Test\n+    fun `cancelling a waiter does not cancel the shared exchange`() = runBlocking {\n+        withTimeout(5000) {\n+            val clock = MutableClock(start)\n+            val entered = CompletableDeferred<Unit>()\n+            val release = CompletableDeferred<Unit>()\n+            val exchanges = AtomicInteger()\n+            HttpClient(MockEngine {\n+                exchanges.incrementAndGet()\n+                entered.complete(Unit)\n+                release.await()\n+                respond(\"\"\"{\"access_token\":\"exchanged\",\"expires_in\":3600}\"\"\", headers = json)\n+            }).use { http ->\n+                val cache = TokenCache(TexasClient(http, clock), clock)\n+                val leader = async { cache.token(\"user-a\", VEDTAK_AUDIENCE) }\n+                entered.await()\n+                val waiter = async(start = CoroutineStart.UNDISPATCHED) { cache.token(\"user-a\", VEDTAK_AUDIENCE) }\n+                waiter.cancelAndJoin()\n+                release.complete(Unit)\n+                assertEquals(\"exchanged\", leader.await())\n+                assertEquals(\"exchanged\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+                assertEquals(1, exchanges.get())\n+            }\n+        }\n+    }\n+\n+    private class MutableClock(@Volatile var now: Instant) : Clock() {\n+        override fun instant(): Instant = now\n+        override fun getZone(): ZoneId = ZoneOffset.UTC\n+        override fun withZone(zone: ZoneId): Clock = this\n+    }\n+}\n*** Update File: <workspace>/src/test/kotlin/no/nav/benchmark/VedtakProxyTest.kt\n@@\n import io.ktor.http.headersOf\n+import io.ktor.http.content.OutgoingContent\n+import io.ktor.http.parseQueryString\n import io.ktor.server.testing.testApplication\n import java.time.Clock\n+import java.time.Duration\n import java.time.Instant\n+import java.time.ZoneId\n@@\n     fun avviserForespørselUtenToken() = testApplication {\n@@\n         assertEquals(HttpStatusCode.Unauthorized, client.get(\"/api/vedtak/42\").status)\n     }\n+\n+    @Test\n+    fun `begge API klienter isolerer brukere og bruker konfigurert fornyelsesmargin`() = testApplication {\n+        val start = Instant.parse(\"2026-03-04T10:00:00Z\")\n+        var now = start\n+        val clock = object : Clock() {\n+            override fun instant(): Instant = now\n+            override fun getZone(): ZoneId = ZoneOffset.UTC\n+            override fun withZone(zone: ZoneId): Clock = this\n+        }\n+        var exchanges = 0\n+        val texas = MockEngine { request ->\n+            val form = parseQueryString((request.body as OutgoingContent.ByteArrayContent).bytes().decodeToString())\n+            val token = \"${form[\"user_token\"]}:${form[\"target\"]}:${++exchanges}\"\n+            respond(\"\"\"{\"access_token\":\"$token\",\"expires_in\":3600}\"\"\", headers = json)\n+        }\n+        val upstream = MockEngine { request ->\n+            respond(request.headers[HttpHeaders.Authorization].orEmpty(), headers = json)\n+        }\n+        application {\n+            vedtakModule(\n+                texas,\n+                upstream,\n+                clock,\n+                config = TokenConfig(expiryLeeway = Duration.ofSeconds(60)),\n+            )\n+        }\n+\n+        for ((path, audience) in listOf(\n+            \"/api/vedtak/42\" to VEDTAK_AUDIENCE,\n+            \"/api/vedtak/42/dokumenter\" to ARKIV_AUDIENCE,\n+        )) {\n+            val a = client.get(path) { bearerAuth(\"user-a\") }\n+            assertEquals(HttpStatusCode.OK, a.status)\n+            assertEquals(\"Bearer user-a:$audience:$exchanges\", a.bodyAsText())\n+            val b = client.get(path) { bearerAuth(\"user-b\") }\n+            assertEquals(\"Bearer user-b:$audience:$exchanges\", b.bodyAsText())\n+            assertEquals(a.bodyAsText(), client.get(path) { bearerAuth(\"user-a\") }.bodyAsText())\n+        }\n+        assertEquals(4, exchanges)\n+\n+        now = start.plusSeconds(3540)\n+        assertEquals(\n+            \"Bearer user-a:$VEDTAK_AUDIENCE:5\",\n+            client.get(\"/api/vedtak/42\") { bearerAuth(\"user-a\") }.bodyAsText(),\n+        )\n+        assertEquals(\n+            \"Bearer user-a:$ARKIV_AUDIENCE:6\",\n+            client.get(\"/api/vedtak/42/dokumenter\") { bearerAuth(\"user-a\") }.bodyAsText(),\n+        )\n+        assertEquals(6, exchanges)\n+    }\n }\n*** End Patch\n"
```

</details>

```
Added 1 file(s): <workspace>/src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt
Modified 1 file(s): <workspace>/src/test/kotlin/no/nav/benchmark/VedtakProxyTest.kt
```


---

<sub>2m 57s</sub>

### `bash`

**Validate token cache changes and workspace diff**

$ cd <workspace> && gradle test --console=plain && git --no-pager diff --check -- . && git status --short -- .

<details>
<summary>18 lines</summary>

```
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/2d4f0fb21c6810b5c3b5fbc9049f6827 -Djansi.tmpdir=~/Library/Caches/cplt/tmp/2d4f0fb21c6810b5c3b5fbc9049f6827 -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=59242 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=59242 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0
Starting a Gradle Daemon (subsequent builds will be faster)
> Task :checkKotlinGradlePluginConfigurationErrors SKIPPED
> Task :processResources NO-SOURCE
> Task :processTestResources NO-SOURCE
> Task :compileKotlin
> Task :compileJava NO-SOURCE
> Task :classes UP-TO-DATE
> Task :jar
> Task :compileTestKotlin
> Task :compileTestJava NO-SOURCE
> Task :testClasses UP-TO-DATE

> Task :test
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/2d4f0fb21c6810b5c3b5fbc9049f6827 -Djansi.tmpdir=~/Library/Caches/cplt/tmp/2d4f0fb21c6810b5c3b5fbc9049f6827 -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=59242 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=59242 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0

BUILD SUCCESSFUL in 9s
4 actionable tasks: 4 executed
fatal: unable to get current working directory: Operation not permitted
<shellId: 3 completed with exit code 128>
```

</details>


---

<sub>3m 30s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Update File: <workspace>/src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt\n@@\n import kotlinx.coroutines.CompletableDeferred\n+import kotlinx.coroutines.CancellationException\n@@\n     fun `cancelling a waiter does not cancel the shared exchange`() = runBlocking {\n@@\n         }\n     }\n \n+    @Test\n+    fun `cancelling the exchange owner releases waiters and allows retry`() = runBlocking {\n+        withTimeout(5000) {\n+            val clock = MutableClock(start)\n+            val entered = CompletableDeferred<Unit>()\n+            val release = CompletableDeferred<Unit>()\n+            val exchanges = AtomicInteger()\n+            HttpClient(MockEngine {\n+                if (exchanges.incrementAndGet() == 1) {\n+                    entered.complete(Unit)\n+                    release.await()\n+                }\n+                respond(\"\"\"{\"access_token\":\"exchanged\",\"expires_in\":3600}\"\"\", headers = json)\n+            }).use { http ->\n+                val cache = TokenCache(TexasClient(http, clock), clock)\n+                val leader = async { cache.token(\"user-a\", VEDTAK_AUDIENCE) }\n+                entered.await()\n+                val waiter = async(start = CoroutineStart.UNDISPATCHED) {\n+                    assertFailsWith<CancellationException> { cache.token(\"user-a\", VEDTAK_AUDIENCE) }\n+                }\n+                leader.cancelAndJoin()\n+                release.complete(Unit)\n+                waiter.await()\n+                assertEquals(\"exchanged\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+                assertEquals(2, exchanges.get())\n+            }\n+        }\n+    }\n+\n+    @Test\n+    fun `failed refresh does not return the previous token`() = runBlocking {\n+        val clock = MutableClock(start)\n+        val exchanges = AtomicInteger()\n+        HttpClient(MockEngine {\n+            when (exchanges.incrementAndGet()) {\n+                1 -> respond(\"\"\"{\"access_token\":\"original\",\"expires_in\":3600}\"\"\", headers = json)\n+                2 -> respond(\"unavailable\", HttpStatusCode.ServiceUnavailable)\n+                else -> respond(\"\"\"{\"access_token\":\"recovered\",\"expires_in\":3600}\"\"\", headers = json)\n+            }\n+        }).use { http ->\n+            val cache = TokenCache(TexasClient(http, clock), clock)\n+            assertEquals(\"original\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+            clock.now = start.plusSeconds(3570)\n+            assertFailsWith<TexasException> { cache.token(\"user-a\", VEDTAK_AUDIENCE) }\n+            assertEquals(\"recovered\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+            assertEquals(3, exchanges.get())\n+        }\n+    }\n+\n     private class MutableClock(@Volatile var now: Instant) : Clock() {\n*** End Patch\n"
```

</details>

```
Modified 1 file(s): <workspace>/src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt
```


---

<sub>3m 39s</sub>

### `bash`

**Run full regression suite**

$ cd <workspace> && gradle test --console=plain

<details>
<summary>16 lines</summary>

```
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/2d4f0fb21c6810b5c3b5fbc9049f6827 -Djansi.tmpdir=~/Library/Caches/cplt/tmp/2d4f0fb21c6810b5c3b5fbc9049f6827 -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=59242 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=59242 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0
> Task :checkKotlinGradlePluginConfigurationErrors SKIPPED
> Task :compileKotlin UP-TO-DATE
> Task :compileJava NO-SOURCE
> Task :processResources NO-SOURCE
> Task :classes UP-TO-DATE
> Task :jar UP-TO-DATE
> Task :processTestResources NO-SOURCE
> Task :compileTestKotlin
> Task :compileTestJava NO-SOURCE
> Task :testClasses UP-TO-DATE

> Task :test
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/2d4f0fb21c6810b5c3b5fbc9049f6827 -Djansi.tmpdir=~/Library/Caches/cplt/tmp/2d4f0fb21c6810b5c3b5fbc9049f6827 -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=59242 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=59242 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0

BUILD SUCCESSFUL in 2s
4 actionable tasks: 2 executed, 2 up-to-date
<shellId: 4 completed with exit code 0>
```

</details>


---

<sub>3m 37s</sub>

### `view`

**view the file at <workspace>/src/main/kotlin/no/nav/benchmark/TokenCache.kt.**

<workspace>/src/main/kotlin/no/nav/benchmark/TokenCache.kt

<details>
<summary>52 lines</summary>

```
package no.nav.benchmark

import kotlinx.coroutines.CompletableDeferred
import java.security.MessageDigest
import java.time.Clock
import java.time.Duration
import java.util.Base64
import java.util.concurrent.ConcurrentHashMap

/** Gjenbruker TokenX-tokener så vi ikke kaller Texas på hver forespørsel. */
class TokenCache(
    private val texas: TexasClient,
    private val clock: Clock,
    private val expiryLeeway: Duration = TokenConfig().expiryLeeway,
) {
    private data class CacheKey(val tokenDigest: String, val audience: String)

    private val tokens = ConcurrentHashMap<CacheKey, OboToken>()
    private val exchanges = ConcurrentHashMap<CacheKey, CompletableDeferred<OboToken>>()

    init {
        require(!expiryLeeway.isNegative) { "expiryLeeway must not be negative" }
    }

    suspend fun token(userToken: String, audience: String): String {
        val key = CacheKey(
            Base64.getEncoder().encodeToString(
                MessageDigest.getInstance("SHA-256").digest(userToken.toByteArray(Charsets.UTF_8)),
            ),
            audience,
        )
        val cached = cachedToken(key)
        if (cached != null) return cached.accessToken

        val exchange = CompletableDeferred<OboToken>()
        val ongoing = exchanges.putIfAbsent(key, exchange)
        if (ongoing != null) return ongoing.await().accessToken

        try {
            // En annen veksling kan ha fullført mellom oppslaget og registreringen.
            val fresh = cachedToken(key) ?: texas.exchange(userToken, audience)
            tokens[key] = fresh
            exchange.complete(fresh)
            return fresh.accessToken
        } catch (failure: Throwable) {
            exchange.completeExceptionally(failure)
            throw failure
        } finally {
            exchanges.remove(key, exchange)
        }
    }

    private fun cachedToken(key: CacheKey): OboToken? {
        val cached = tokens[key]
        if (cached != null && cached.isValid(clock.instant(), expiryLeeway)) {
            return cached
        }
        return null
    }
}
```

</details>


---

<sub>3m 37s</sub>

### `view`

**view the file at <workspace>/docs/adr/0002-tokenx-via-texas.md.**

<workspace>/docs/adr/0002-tokenx-via-texas.md

<details>
<summary>24 lines</summary>

```
# 2. TokenX-veksling via Texas med cache

Dato: 2026-03-04

## Kontekst

`vedtak-proxy` kaller `vedtak-api` og `arkiv-api` på vegne av innlogget bruker.
Hvert kall trenger et TokenX-token for API-ets audience. Texas-sidecaren veksler
brukerens token mot et slikt token. Uten cache går det ett kall til Texas per
forespørsel.

## Beslutning

- Vekslede tokener caches i minnet per pod, felles for alle klientene.
- Et vekslet token gjelder én bruker og én audience. Det skal aldri brukes for en
  annen bruker eller mot et annet API.
- Cache-nøkkelen består av SHA-256 av hele det innkommende tokenet og audience.
  Et nytt innkommende token får derfor en egen oppføring, også for samme bruker.
- Tokener fornyes når tiden som er igjen før de utløper, er kortere enn
  eller lik `TokenConfig.expiryLeeway`. Det dekker klokkeforskjell og tiden et kall
  er underveis. Levetiden regnes fra starten av Texas-kallet.
- Texas har en grense for antall vekslinger per sekund. Samtidige forespørsler fra
  samme bruker mot samme API skal derfor dele én veksling.
- Ventende kall deler resultatet, også ved feil. Feil og kansellering lagres ikke
  i cachen. Hvis kallet som utfører vekslingen kanselleres, får ventende kall også
  kanselleringen. Neste forespørsel kan forsøke på nytt.
- Tiden hentes fra en injisert `Clock`, så testene kan styre den.

## Konsekvenser

Cachen vokser med antall aktive brukere. Det er akseptabelt med dagens volum.
```

</details>


---

<sub>3m 37s</sub>

### `bash`

**Check changed files for whitespace errors**

$ git -C <workspace> --no-pager diff --check -- <workspace>

```
fatal: unable to get current working directory: Operation not permitted
<shellId: 5 completed with exit code 128>
```


---

<sub>4m 18s</sub>

### Copilot

**Rettet i den felles token-cachen for `vedtak-api` og `arkiv-api`.** Tre feil forklarer symptomene: audience var eneste cache-nøkkel, `expiryLeeway` ble ignorert, og samtidige cache-miss startet separate Texas-vekslinger.

Cachen bruker nå SHA-256 av hele innkommende token sammen med audience. Det skiller brukere og token-fornyelser, men gir flere cache-oppføringer enn en nøkkel basert på brukeridentitet. Samtidige kall med samme nøkkel deler resultat eller feil, uten en global lås som blokkerer andre brukere. Levetiden regnes fra starten av Texas-kallet, og konfigurasjonen styrer fornyelsesmarginen.

🔴 Rød sone: token-håndtering, forstå dette grundig.

**Sjekkliste for rød sone**
- [ ] Cache-nøkkelen hindrer gjenbruk på tvers av brukere og API-er.
- [ ] Tokener fornyes ved eller innenfor utløpsmarginen.
- [ ] Delt veksling rydder opp ved feil og kansellering, slik at senere kall kan prøve på nytt.

---

<sub>4m 18s</sub>

### Info

Session limits: 23.18/30 AI credits used.

---

<sub>Generated by [GitHub Copilot CLI](https://github.com/features/copilot/cli)</sub>
