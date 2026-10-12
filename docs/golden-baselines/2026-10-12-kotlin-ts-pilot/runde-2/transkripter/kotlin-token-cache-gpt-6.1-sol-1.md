# Copilot CLI Session

> [!NOTE]
> - **Session ID:** `40a677ae-52da-4eb4-aca2-4fd80d5983fa`  
> - **Started:** 10/13/2026, 12:57:21 AM  
> - **Duration:** 4m 19s  
> - **Exported:** 10/13/2026, 1:01:41 AM  

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

<sub>10s</sub>

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

<sub>10s</sub>

### `bash`

**Inspect workspace changes**

$ git status --short -- .

```
fatal: unable to get current working directory: Operation not permitted
<shellId: 0 completed with exit code 128>
```


---

<sub>21s</sub>

### `view`

**view the file at <workspace>/.github/instructions/kotlin.instructions.md.**

<workspace>/.github/instructions/kotlin.instructions.md

```
File too large to read at once (22.1 KB). Consider using the grep tool to search within the file, or view with view_range to read specific sections.
```


---

<sub>21s</sub>

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

<sub>21s</sub>

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

<sub>21s</sub>

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

<sub>21s</sub>

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

<sub>21s</sub>

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

<sub>21s</sub>

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

<sub>21s</sub>

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

<sub>33s</sub>

### `view`

**view the file at <workspace>/.github/instructions/kotlin.instructions.md.**

<workspace>/.github/instructions/kotlin.instructions.md (lines 1-520)

<details>
<summary>416 lines</summary>

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
````

</details>


---

<sub>33s</sub>

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

<sub>33s</sub>

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

<sub>33s</sub>

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

<sub>33s</sub>

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

<sub>33s</sub>

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

<sub>33s</sub>

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

<sub>33s</sub>

### `bash`

**Check build tools and repository status**

$ pwd; command -v gradle; command -v java; git --no-pager -C <workspace> status --short -- .

```
<workspace>
~/.local/share/mise/shims/gradle
~/.local/share/mise/shims/java
fatal: unable to get current working directory: Operation not permitted
<shellId: 1 completed with exit code 128>
```


---

<sub>39s</sub>

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

<sub>1m 7s</sub>

### Copilot

Cachen bruker bare audience som nøkkel, ignorerer den konfigurerte utløpsmarginen og lar samtidige cache-miss starte hver sin veksling. Jeg retter dette i den delte cachen, slik at både vedtak- og arkivklienten får samme beskyttelse.


---

<sub>1m 8s</sub>

### `view`

**view the file at <workspace>/.github/instructions/kotlin.instructions.md.**

<workspace>/.github/instructions/kotlin.instructions.md (lines 521-850)

<details>
<summary>174 lines</summary>

````
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

<sub>1m 8s</sub>

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

<sub>1m 12s</sub>

### `bash`

**Run baseline Gradle tests**

$ cd <workspace> && gradle test --console=plain

<details>
<summary>41 lines</summary>

```
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/b9f3fb9a97fab7017db27808cdf4dc1b -Djansi.tmpdir=~/Library/Caches/cplt/tmp/b9f3fb9a97fab7017db27808cdf4dc1b -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=54756 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=54756 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0
Starting a Gradle Daemon (subsequent builds will be faster)
> Task :checkKotlinGradlePluginConfigurationErrors SKIPPED
> Task :processResources NO-SOURCE
> Task :processTestResources NO-SOURCE
The message received from the daemon indicates that the daemon has disappeared.
Build request sent: Build{id=d6e6dc0f-3bf4-4301-9b42-b12d11d5106c, currentDir=<workspace>}
Attempting to read last messages from the daemon log...
Daemon pid: 39485
  log file: ~/.gradle/daemon/8.14.5/daemon-39485.out.log
----- Last 20 lines from daemon log file - daemon-39485.out.log -----
2026-10-13T00:58:30.440+0200 [DEBUG] [org.gradle.launcher.daemon.server.DefaultDaemonConnection] thread 40: Received non-IO message from client: Build{id=d6e6dc0f-3bf4-4301-9b42-b12d11d5106c, currentDir=<workspace>}
2026-10-13T00:58:30.440+0200 [INFO] [org.gradle.launcher.daemon.server.DefaultIncomingConnectionHandler] Received command: Build{id=d6e6dc0f-3bf4-4301-9b42-b12d11d5106c, currentDir=<workspace>}.
2026-10-13T00:58:30.441+0200 [DEBUG] [org.gradle.launcher.daemon.server.DefaultIncomingConnectionHandler] Starting executing command: Build{id=d6e6dc0f-3bf4-4301-9b42-b12d11d5106c, currentDir=<workspace>} with connection: socket connection from /127.0.0.1:54899 to /127.0.0.1:54903.
2026-10-13T00:58:30.442+0200 [DEBUG] [org.gradle.launcher.daemon.server.DaemonStateCoordinator] Command execution: started DaemonCommandExecution[command = Build{id=d6e6dc0f-3bf4-4301-9b42-b12d11d5106c, currentDir=<workspace>}, connection = DefaultDaemonConnection: socket connection from /127.0.0.1:54899 to /127.0.0.1:54903] after 6.166666666666666E-4 minutes of idle
2026-10-13T00:58:30.442+0200 [INFO] [org.gradle.launcher.daemon.server.DaemonRegistryUpdater] Marking the daemon as busy, address: [e3bd3e23-f0e0-4a8a-a289-c2aaccdc5f8e port:54899, addresses:[localhost/127.0.0.1]]
2026-10-13T00:58:30.442+0200 [DEBUG] [org.gradle.launcher.daemon.registry.PersistentDaemonRegistry] Marking busy by address: [e3bd3e23-f0e0-4a8a-a289-c2aaccdc5f8e port:54899, addresses:[localhost/127.0.0.1]]
2026-10-13T00:58:30.443+0200 [DEBUG] [org.gradle.cache.internal.DefaultFileLockManager] Waiting to acquire exclusive lock on daemon addresses registry.
2026-10-13T00:58:30.443+0200 [DEBUG] [org.gradle.cache.internal.DefaultFileLockManager] Lock acquired on daemon addresses registry.
2026-10-13T00:58:30.443+0200 [DEBUG] [org.gradle.cache.internal.DefaultFileLockManager] Releasing lock on daemon addresses registry.
2026-10-13T00:58:30.443+0200 [DEBUG] [org.gradle.launcher.daemon.server.DaemonStateCoordinator] resetting idle timer
2026-10-13T00:58:30.444+0200 [DEBUG] [org.gradle.launcher.daemon.server.DaemonStateCoordinator] daemon is running. Sleeping until state changes.
2026-10-13T00:58:30.444+0200 [INFO] [org.gradle.launcher.daemon.server.exec.StartBuildOrRespondWithBusy] Daemon is about to start building Build{id=d6e6dc0f-3bf4-4301-9b42-b12d11d5106c, currentDir=<workspace>}. Dispatching build started information...
2026-10-13T00:58:30.444+0200 [DEBUG] [org.gradle.launcher.daemon.server.SynchronizedDispatchConnection] thread 42: dispatching org.gradle.launcher.daemon.protocol.BuildStarted@78d0e7a0
2026-10-13T00:58:30.445+0200 [DEBUG] [org.gradle.launcher.daemon.server.exec.EstablishBuildEnvironment] Configuring env variables: [PATH, DOTNET_CLI_DO_NOT_USE_MSBUILD_SERVER, npm_config_ignore_scripts, NO_PROXY, https_proxy, KUBECONFIG, MISE_SHELL, GIT_CONFIG_COUNT, TURBO_TELEMETRY_DISABLED, GOCACHE, GCM_INTERACTIVE, pnpm_config_ignore_scripts, COPILOT_AGENT_SESSION_ID, PNPM_HOME, COREPACK_ENABLE_AUTO_PIN, COLORTERM, RUSTUP_HOME, NODE_USE_ENV_PROXY, LOGNAME, GIT_CONFIG_VALUE_0, GIT_CONFIG_VALUE_1, GIT_OPTIONAL_LOCKS, TERM_PROGRAM_VERSION, GIT_CONFIG_VALUE_2, PWD, GIT_CONFIG_VALUE_3, CARGO_HOME, GIT_ASKPASS, LANGUAGE, __CPLT_WRAPPED, DO_NOT_TRACK, SHELL, __MISE_SHIM, OMO_DISABLE_POSTHOG, COPILOT_CLI_BINARY_VERSION, GIT_TERMINAL_PROMPT, GOPATH, COPILOT_CLI_SANDBOX_SUPPORT_OVERRIDE, NPM_CONFIG_USERCONFIG, GATSBY_TELEMETRY_DISABLED, TMPDIR, PWTEST_SOCKETS_DIR, SSH_ASKPASS, LC_ALL, __CF_USER_TEXT_ENCODING, LC_CTYPE, SHLVL, TMUX_PANE, DOCKER_HOST, GOTMPDIR, YARN_ENABLE_SCRIPTS, JAVA_HOME, NODE_OPTIONS, TERM, HTTP_PROXY, LANG, __CPLT_TRUST_LOCKED, JAVA_TOOL_OPTIONS, HTTPS_PROXY, DOTNET_CLI_HOME, GOFLAGS, CLOUDSDK_ROOT_DIR, NEXT_TELEMETRY_DISABLED, GIT_CONFIG_KEY_1, no_proxy, http_proxy, GIT_CONFIG_KEY_0, GRADLE_MACOS_SANDBOX, GIT_CONFIG_KEY_3, GIT_CONFIG_KEY_2, CHECKPOINT_DISABLE, FZF_DEFAULT_COMMAND, TERM_PROGRAM, COPILOT_CLI_RESOLVED_DIST_DIR, RUSTUP_TOOLCHAIN, GIT_CONFIG_NOSYSTEM, GPG_TTY, USER, GOROOT, PLAYWRIGHT_MCP_SANDBOX, GH_TOKEN, GOBIN, TEMP, TMP, DISABLE_AUTOUPDATER, __CPLT_SCRATCH_DIR, KUBE_EDITOR, TMUX, COPILOT_CLI, HOME, __MISE_DIFF]
2026-10-13T00:58:30.447+0200 [DEBUG] [org.gradle.launcher.daemon.server.exec.LogToClient] About to start relaying all logs to the client via the connection.
2026-10-13T00:58:30.447+0200 [INFO] [org.gradle.launcher.daemon.server.exec.LogToClient] The client will now receive all logging from the daemon (pid: 39485). The daemon log file: ~/.gradle/daemon/8.14.5/daemon-39485.out.log
2026-10-13T00:58:30.447+0200 [INFO] [org.gradle.launcher.daemon.server.exec.LogAndCheckHealth] Starting build in new daemon [memory: 512 MiB]
2026-10-13T00:58:30.448+0200 [DEBUG] [org.gradle.launcher.daemon.server.exec.ExecuteBuild] The daemon has started executing the build.
2026-10-13T00:58:30.448+0200 [DEBUG] [org.gradle.launcher.daemon.server.exec.ExecuteBuild] Executing build with daemon context: DefaultDaemonContext[uid=99d7f6b4-e81c-49b4-b25a-84802b3b81db,javaHome=~/.local/share/mise/installs/java/21.0.2,javaVersion=21,javaVendor=Oracle Corporation,daemonRegistryDir=~/.gradle/daemon,pid=39485,idleTimeout=10800000,priority=NORMAL,applyInstrumentationAgent=true,nativeServicesMode=ENABLED,daemonOpts=-XX:MaxMetaspaceSize=384m,-XX:+HeapDumpOnOutOfMemoryError,-Xms256m,-Xmx512m,-Dfile.encoding=UTF-8,-Djava.io.tmpdir=~/Library/Caches/cplt/tmp/b9f3fb9a97fab7017db27808cdf4dc1b,-Duser.country=US,-Duser.language=en,-Duser.variant]
Daemon vm is shutting down... The daemon has exited normally or was terminated in response to a user interrupt.----- End of the daemon log -----


FAILURE: Build failed with an exception.

* What went wrong:
Gradle build daemon disappeared unexpectedly (it may have been killed or may have crashed)

* Try:
> Run with --stacktrace option to get the stack trace.
> Run with --info or --debug option to get more log output.
> Run with --scan to get full insights.
> Get more help at https://help.gradle.org.
> Task :compileKotlin
<shellId: 2 completed with exit code 1>
```

</details>


---

<sub>2m 36s</sub>

### Info

Session limits: 18.26/30 AI credits used.

---

<sub>2m 36s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Update File: <workspace>/src/main/kotlin/no/nav/benchmark/TokenCache.kt\n@@\n package no.nav.benchmark\n \n+import kotlinx.coroutines.CompletableDeferred\n+import java.security.MessageDigest\n import java.time.Clock\n-import java.util.concurrent.ConcurrentHashMap\n+import java.time.Duration\n+import java.util.HexFormat\n@@\n     private val texas: TexasClient,\n     private val clock: Clock,\n+    private val expiryLeeway: Duration = Duration.ofSeconds(30),\n ) {\n-    private val tokens = ConcurrentHashMap<String, OboToken>()\n+    private data class Key(val tokenHash: String, val audience: String)\n+\n+    private val lock = Any()\n+    private val tokens = mutableMapOf<Key, OboToken>()\n+    private val exchanges = mutableMapOf<Key, CompletableDeferred<OboToken>>()\n+\n+    init {\n+        require(!expiryLeeway.isNegative) { \"expiryLeeway must not be negative\" }\n+    }\n \n     suspend fun token(userToken: String, audience: String): String {\n-        val cached = tokens[audience]\n-        if (cached != null && cached.isValid(clock.instant())) {\n-            return cached.accessToken\n+        val hash = MessageDigest.getInstance(\"SHA-256\").digest(userToken.toByteArray(Charsets.UTF_8))\n+        val key = Key(HexFormat.of().formatHex(hash), audience)\n+        val (exchange, owner) = synchronized(lock) {\n+            val cached = tokens[key]\n+            if (cached != null && cached.isValid(clock.instant().plus(expiryLeeway))) {\n+                return cached.accessToken\n+            }\n+            tokens.remove(key)\n+            val pending = exchanges[key]\n+            if (pending != null) {\n+                pending to false\n+            } else {\n+                CompletableDeferred<OboToken>().also { exchanges[key] = it } to true\n+            }\n+        }\n+        if (!owner) return exchange.await().accessToken\n+\n+        try {\n+            val fresh = texas.exchange(userToken, audience)\n+            if (!fresh.isValid(clock.instant().plus(expiryLeeway))) {\n+                throw TexasException(\"Texas returnerte et token med for kort levetid\")\n+            }\n+            synchronized(lock) {\n+                tokens[key] = fresh\n+                exchange.complete(fresh)\n+                exchanges.remove(key)\n+            }\n+            return fresh.accessToken\n+        } catch (failure: Throwable) {\n+            // Alle ventere får samme feil, og neste forespørsel kan prøve igjen.\n+            synchronized(lock) {\n+                exchange.completeExceptionally(failure)\n+                exchanges.remove(key)\n+            }\n+            throw failure\n         }\n-        val fresh = texas.exchange(userToken, audience)\n-        tokens[audience] = fresh\n-        return fresh.accessToken\n     }\n }\n*** Update File: <workspace>/src/main/kotlin/no/nav/benchmark/Application.kt\n@@\n-    val tokens = TokenCache(texas, clock)\n+    val tokens = TokenCache(texas, clock, config.expiryLeeway)\n*** Update File: <workspace>/src/main/kotlin/no/nav/benchmark/TexasClient.kt\n@@\n     suspend fun exchange(userToken: String, audience: String): OboToken {\n+        val requestedAt = clock.instant()\n         val response = http.submitForm(\n@@\n-        return OboToken(accessToken, clock.instant().plusSeconds(expiresIn))\n+        return OboToken(accessToken, requestedAt.plusSeconds(expiresIn))\n*** Update File: <workspace>/docs/adr/0002-tokenx-via-texas.md\n@@\n - Et vekslet token gjelder én bruker og én audience. Det skal aldri brukes for en\n   annen bruker eller mot et annet API.\n-- Tokener fornyes når tiden som er igjen før de utløper, er kortere enn\n+- Cache-nøkkelen består av SHA-256 av hele innkommende token og audience.\n+  Et nytt innkommende token gir en ny nøkkel, også for samme bruker. Vi lagrer ikke\n+  innkommende tokener i cache-nøklene og logger ikke tokener eller nøklene.\n+- Tokener fornyes når tiden som er igjen før de utløper, er kortere enn eller lik\n   `TokenConfig.expiryLeeway`. Det dekker klokkeforskjell og tiden et kall er underveis.\n+  Utløpstiden beregnes fra starten av Texas-kallet, slik at svartiden ikke forlenger\n+  levetiden. Et nytt token med for kort gjenværende levetid gir en feil.\n@@\n   samme bruker mot samme API skal derfor dele én veksling.\n+- Samtidige kall med samme cache-nøkkel deler også feil fra vekslingen.\n+  Ved feil eller kansellering av kallet som veksler, fjernes den pågående vekslingen,\n+  slik at neste forespørsel kan prøve igjen. Kansellering av en venter stopper ikke\n+  vekslingen. Andre cache-nøkler kan veksle samtidig.\n*** Add File: <workspace>/src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt\n+package no.nav.benchmark\n+\n+import io.ktor.client.HttpClient\n+import io.ktor.client.engine.mock.MockEngine\n+import io.ktor.client.engine.mock.respond\n+import io.ktor.http.HttpHeaders\n+import io.ktor.http.HttpStatusCode\n+import io.ktor.http.headersOf\n+import kotlinx.coroutines.CompletableDeferred\n+import kotlinx.coroutines.CoroutineStart\n+import kotlinx.coroutines.async\n+import kotlinx.coroutines.awaitAll\n+import kotlinx.coroutines.cancelAndJoin\n+import kotlinx.coroutines.runBlocking\n+import kotlinx.coroutines.withTimeout\n+import java.time.Clock\n+import java.time.Duration\n+import java.time.Instant\n+import java.time.ZoneId\n+import java.time.ZoneOffset\n+import java.util.concurrent.atomic.AtomicInteger\n+import kotlin.test.Test\n+import kotlin.test.assertEquals\n+import kotlin.test.assertFailsWith\n+import kotlin.test.assertIs\n+\n+class TokenCacheTest {\n+    private val json = headersOf(HttpHeaders.ContentType, \"application/json\")\n+\n+    @Test\n+    fun `isolates incoming tokens and audiences and reuses exact matches`() = runBlocking {\n+        val calls = AtomicInteger()\n+        val clock = MutableClock()\n+        val engine = MockEngine {\n+            respond(tokenBody(\"exchanged-${calls.incrementAndGet()}\"), headers = json)\n+        }\n+        HttpClient(engine).use { http ->\n+            val cache = TokenCache(TexasClient(http, clock), clock)\n+            assertEquals(\"exchanged-1\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+            assertEquals(\"exchanged-2\", cache.token(\"user-b\", VEDTAK_AUDIENCE))\n+            assertEquals(\"exchanged-3\", cache.token(\"user-a\", ARKIV_AUDIENCE))\n+            assertEquals(\"exchanged-4\", cache.token(\"user-a-renewed\", VEDTAK_AUDIENCE))\n+            assertEquals(\"exchanged-1\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+            assertEquals(\"exchanged-2\", cache.token(\"user-b\", VEDTAK_AUDIENCE))\n+            assertEquals(\"exchanged-3\", cache.token(\"user-a\", ARKIV_AUDIENCE))\n+            assertEquals(4, calls.get())\n+        }\n+    }\n+\n+    @Test\n+    fun `refreshes at the exact configured margin and after expiry`() = runBlocking {\n+        val calls = AtomicInteger()\n+        val clock = MutableClock()\n+        val engine = MockEngine {\n+            respond(tokenBody(\"exchanged-${calls.incrementAndGet()}\"), headers = json)\n+        }\n+        HttpClient(engine).use { http ->\n+            val cache = TokenCache(TexasClient(http, clock), clock, Duration.ofSeconds(60))\n+            assertEquals(\"exchanged-1\", cache.token(\"user\", VEDTAK_AUDIENCE))\n+            clock.advance(Duration.ofSeconds(3539))\n+            assertEquals(\"exchanged-1\", cache.token(\"user\", VEDTAK_AUDIENCE))\n+            clock.advance(Duration.ofSeconds(1))\n+            assertEquals(\"exchanged-2\", cache.token(\"user\", VEDTAK_AUDIENCE))\n+            clock.advance(Duration.ofSeconds(3600))\n+            assertEquals(\"exchanged-3\", cache.token(\"user\", VEDTAK_AUDIENCE))\n+            assertEquals(3, calls.get())\n+        }\n+    }\n+\n+    @Test\n+    fun `counts Texas response time against token lifetime`() = runBlocking {\n+        val clock = MutableClock()\n+        val requestedAt = clock.instant()\n+        val engine = MockEngine {\n+            clock.advance(Duration.ofSeconds(20))\n+            respond(tokenBody(\"exchanged\"), headers = json)\n+        }\n+        HttpClient(engine).use { http ->\n+            assertEquals(requestedAt.plusSeconds(3600), TexasClient(http, clock).exchange(\"user\", VEDTAK_AUDIENCE).expiresAt)\n+        }\n+    }\n+\n+    @Test\n+    fun `shares one exchange for concurrent misses and refreshes`() = runBlocking {\n+        withTimeout(5000) {\n+            val calls = AtomicInteger()\n+            val clock = MutableClock()\n+            var started = CompletableDeferred<Unit>()\n+            var release = CompletableDeferred<Unit>()\n+            val engine = MockEngine {\n+                val number = calls.incrementAndGet()\n+                started.complete(Unit)\n+                release.await()\n+                respond(tokenBody(\"exchanged-$number\"), headers = json)\n+            }\n+            HttpClient(engine).use { http ->\n+                val cache = TokenCache(TexasClient(http, clock), clock)\n+                repeat(2) { round ->\n+                    val requests = List(40) {\n+                        async(start = CoroutineStart.UNDISPATCHED) { cache.token(\"user\", VEDTAK_AUDIENCE) }\n+                    }\n+                    started.await()\n+                    assertEquals(round + 1, calls.get())\n+                    release.complete(Unit)\n+                    assertEquals(List(40) { \"exchanged-${round + 1}\" }, requests.awaitAll())\n+                    clock.advance(Duration.ofSeconds(3570))\n+                    started = CompletableDeferred()\n+                    release = CompletableDeferred()\n+                }\n+                assertEquals(2, calls.get())\n+            }\n+        }\n+    }\n+\n+    @Test\n+    fun `does not block other keys behind a slow exchange`() = runBlocking {\n+        withTimeout(5000) {\n+            val calls = AtomicInteger()\n+            val clock = MutableClock()\n+            val started = CompletableDeferred<Unit>()\n+            val release = CompletableDeferred<Unit>()\n+            val engine = MockEngine {\n+                val number = calls.incrementAndGet()\n+                if (number == 1) {\n+                    started.complete(Unit)\n+                    release.await()\n+                }\n+                respond(tokenBody(\"exchanged-$number\"), headers = json)\n+            }\n+            HttpClient(engine).use { http ->\n+                val cache = TokenCache(TexasClient(http, clock), clock)\n+                val slow = async { cache.token(\"user-a\", VEDTAK_AUDIENCE) }\n+                started.await()\n+                assertEquals(\"exchanged-2\", cache.token(\"user-b\", VEDTAK_AUDIENCE))\n+                assertEquals(\"exchanged-3\", cache.token(\"user-a\", ARKIV_AUDIENCE))\n+                release.complete(Unit)\n+                assertEquals(\"exchanged-1\", slow.await())\n+            }\n+        }\n+    }\n+\n+    @Test\n+    fun `shares exchange failures and allows a later retry`() = runBlocking {\n+        withTimeout(5000) {\n+            val calls = AtomicInteger()\n+            val clock = MutableClock()\n+            val started = CompletableDeferred<Unit>()\n+            val release = CompletableDeferred<Unit>()\n+            val engine = MockEngine {\n+                if (calls.incrementAndGet() == 1) {\n+                    started.complete(Unit)\n+                    release.await()\n+                    respond(\"unavailable\", HttpStatusCode.ServiceUnavailable)\n+                } else {\n+                    respond(tokenBody(\"retried\"), headers = json)\n+                }\n+            }\n+            HttpClient(engine).use { http ->\n+                val cache = TokenCache(TexasClient(http, clock), clock)\n+                val requests = List(20) {\n+                    async(start = CoroutineStart.UNDISPATCHED) {\n+                        runCatching { cache.token(\"user\", VEDTAK_AUDIENCE) }\n+                    }\n+                }\n+                started.await()\n+                assertEquals(1, calls.get())\n+                release.complete(Unit)\n+                requests.awaitAll().forEach { assertIs<TexasException>(it.exceptionOrNull()) }\n+                assertEquals(1, calls.get())\n+                assertEquals(\"retried\", cache.token(\"user\", VEDTAK_AUDIENCE))\n+                assertEquals(2, calls.get())\n+            }\n+        }\n+    }\n+\n+    @Test\n+    fun `cancelling a waiter does not cancel the exchange`() = runBlocking {\n+        withTimeout(5000) {\n+            val clock = MutableClock()\n+            val calls = AtomicInteger()\n+            val started = CompletableDeferred<Unit>()\n+            val release = CompletableDeferred<Unit>()\n+            val engine = MockEngine {\n+                calls.incrementAndGet()\n+                started.complete(Unit)\n+                release.await()\n+                respond(tokenBody(\"exchanged\"), headers = json)\n+            }\n+            HttpClient(engine).use { http ->\n+                val cache = TokenCache(TexasClient(http, clock), clock)\n+                val owner = async { cache.token(\"user\", VEDTAK_AUDIENCE) }\n+                started.await()\n+                val waiter = async(start = CoroutineStart.UNDISPATCHED) { cache.token(\"user\", VEDTAK_AUDIENCE) }\n+                waiter.cancelAndJoin()\n+                release.complete(Unit)\n+                assertEquals(\"exchanged\", owner.await())\n+                assertEquals(\"exchanged\", cache.token(\"user\", VEDTAK_AUDIENCE))\n+                assertEquals(1, calls.get())\n+            }\n+        }\n+    }\n+\n+    @Test\n+    fun `cancelling the exchange owner releases waiters and allows retry`() = runBlocking {\n+        withTimeout(5000) {\n+            val clock = MutableClock()\n+            val calls = AtomicInteger()\n+            val started = CompletableDeferred<Unit>()\n+            val engine = MockEngine {\n+                if (calls.incrementAndGet() == 1) {\n+                    started.complete(Unit)\n+                    CompletableDeferred<Unit>().await()\n+                }\n+                respond(tokenBody(\"retried\"), headers = json)\n+            }\n+            HttpClient(engine).use { http ->\n+                val cache = TokenCache(TexasClient(http, clock), clock)\n+                val owner = async { cache.token(\"user\", VEDTAK_AUDIENCE) }\n+                started.await()\n+                val waiter = async(start = CoroutineStart.UNDISPATCHED) {\n+                    runCatching { cache.token(\"user\", VEDTAK_AUDIENCE) }\n+                }\n+                owner.cancelAndJoin()\n+                assertIs<kotlinx.coroutines.CancellationException>(waiter.await().exceptionOrNull())\n+                assertEquals(\"retried\", cache.token(\"user\", VEDTAK_AUDIENCE))\n+                assertEquals(2, calls.get())\n+            }\n+        }\n+    }\n+\n+    @Test\n+    fun `rejects unusable fresh tokens without caching them`() = runBlocking {\n+        val clock = MutableClock()\n+        val calls = AtomicInteger()\n+        val lifetimes = listOf(30, 0, -1, 3600)\n+        val engine = MockEngine {\n+            respond(tokenBody(\"exchanged\", lifetimes[calls.getAndIncrement()]), headers = json)\n+        }\n+        HttpClient(engine).use { http ->\n+            val cache = TokenCache(TexasClient(http, clock), clock)\n+            repeat(3) { assertFailsWith<TexasException> { cache.token(\"user\", VEDTAK_AUDIENCE) } }\n+            assertEquals(\"exchanged\", cache.token(\"user\", VEDTAK_AUDIENCE))\n+            assertEquals(\"exchanged\", cache.token(\"user\", VEDTAK_AUDIENCE))\n+            assertEquals(4, calls.get())\n+        }\n+    }\n+\n+    private fun tokenBody(token: String, lifetime: Int = 3600) =\n+        \"\"\"{\"access_token\":\"$token\",\"expires_in\":$lifetime,\"token_type\":\"Bearer\"}\"\"\"\n+\n+    private class MutableClock : Clock() {\n+        private var now = Instant.parse(\"2026-03-04T10:00:00Z\")\n+        override fun instant(): Instant = now\n+        override fun getZone(): ZoneId = ZoneOffset.UTC\n+        override fun withZone(zone: ZoneId): Clock = Clock.fixed(now, zone)\n+        fun advance(duration: Duration) {\n+            now = now.plus(duration)\n+        }\n+    }\n+}\n*** End Patch\n"
```

</details>

```
Added 1 file(s): <workspace>/src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt
Modified 4 file(s): <workspace>/src/main/kotlin/no/nav/benchmark/TokenCache.kt, <workspace>/src/main/kotlin/no/nav/benchmark/Application.kt, <workspace>/src/main/kotlin/no/nav/benchmark/TexasClient.kt, <workspace>/docs/adr/0002-tokenx-via-texas.md
```


---

<sub>2m 52s</sub>

### `bash`

**Validate tests with in-process Kotlin compiler**

$ cd <workspace> && gradle test --no-daemon --max-workers=1 -Pkotlin.compiler.execution.strategy=in-process --console=plain && git --no-pager diff --check -- .

<details>
<summary>19 lines</summary>

```
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/b9f3fb9a97fab7017db27808cdf4dc1b -Djansi.tmpdir=~/Library/Caches/cplt/tmp/b9f3fb9a97fab7017db27808cdf4dc1b -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=54756 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=54756 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0
To honour the JVM settings for this build a single-use Daemon process will be forked. For more on this, please refer to https://docs.gradle.org/8.14.5/userguide/gradle_daemon.html#sec:disabling_the_daemon in the Gradle documentation.
Daemon will be stopped at the end of the build 
> Task :checkKotlinGradlePluginConfigurationErrors SKIPPED
> Task :compileKotlin
> Task :compileJava NO-SOURCE
> Task :processResources NO-SOURCE
> Task :classes UP-TO-DATE
> Task :jar
> Task :compileTestKotlin
> Task :compileTestJava NO-SOURCE
> Task :processTestResources NO-SOURCE
> Task :testClasses UP-TO-DATE

> Task :test
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/b9f3fb9a97fab7017db27808cdf4dc1b -Djansi.tmpdir=~/Library/Caches/cplt/tmp/b9f3fb9a97fab7017db27808cdf4dc1b -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=54756 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=54756 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0

BUILD SUCCESSFUL in 10s
4 actionable tasks: 4 executed
fatal: unable to get current working directory: Operation not permitted
<shellId: 3 completed with exit code 128>
```

</details>


---

<sub>3m 29s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Update File: <workspace>/src/test/kotlin/no/nav/benchmark/VedtakProxyTest.kt\n@@\n import io.ktor.client.request.get\n+import io.ktor.client.request.forms.FormDataContent\n@@\n import java.time.Clock\n+import java.time.Duration\n import java.time.Instant\n+import java.time.ZoneId\n import java.time.ZoneOffset\n+import java.util.concurrent.atomic.AtomicInteger\n@@\n     fun avviserForespørselUtenToken() = testApplication {\n@@\n         assertEquals(HttpStatusCode.Unauthorized, client.get(\"/api/vedtak/42\").status)\n     }\n+\n+    @Test\n+    fun `both clients isolate users and audiences and honor configured expiry margin`() = testApplication {\n+        var now = Instant.parse(\"2026-03-04T10:00:00Z\")\n+        val clock = object : Clock() {\n+            override fun instant(): Instant = now\n+            override fun getZone(): ZoneId = ZoneOffset.UTC\n+            override fun withZone(zone: ZoneId): Clock = Clock.fixed(now, zone)\n+        }\n+        val calls = AtomicInteger()\n+        val texas = MockEngine { request ->\n+            val form = (request.body as FormDataContent).formData\n+            assertEquals(\"tokenx\", form[\"identity_provider\"])\n+            val token = \"${form[\"user_token\"]}:${form[\"target\"]}:${calls.incrementAndGet()}\"\n+            respond(\"\"\"{\"access_token\":\"$token\",\"expires_in\":3600}\"\"\", headers = json)\n+        }\n+        val vedtak = MockEngine { request ->\n+            respond(request.headers[HttpHeaders.Authorization].orEmpty(), headers = json)\n+        }\n+        val arkiv = MockEngine { request ->\n+            respond(request.headers[HttpHeaders.Authorization].orEmpty(), headers = json)\n+        }\n+        application {\n+            vedtakModule(\n+                texas,\n+                vedtak,\n+                clock,\n+                arkiv,\n+                TokenConfig(expiryLeeway = Duration.ofSeconds(60)),\n+            )\n+        }\n+\n+        suspend fun check(path: String, user: String, audience: String, exchange: Int) {\n+            val response = client.get(path) { bearerAuth(user) }\n+            assertEquals(HttpStatusCode.OK, response.status)\n+            assertEquals(\"Bearer $user:$audience:$exchange\", response.bodyAsText())\n+        }\n+\n+        check(\"/api/vedtak/42\", \"user-a\", VEDTAK_AUDIENCE, 1)\n+        check(\"/api/vedtak/42\", \"user-b\", VEDTAK_AUDIENCE, 2)\n+        check(\"/api/vedtak/42/dokumenter\", \"user-a\", ARKIV_AUDIENCE, 3)\n+        check(\"/api/vedtak/42/dokumenter\", \"user-b\", ARKIV_AUDIENCE, 4)\n+        check(\"/api/vedtak/42\", \"user-a-renewed\", VEDTAK_AUDIENCE, 5)\n+        check(\"/api/vedtak/42\", \"user-a\", VEDTAK_AUDIENCE, 1)\n+        check(\"/api/vedtak/42\", \"user-b\", VEDTAK_AUDIENCE, 2)\n+        check(\"/api/vedtak/42/dokumenter\", \"user-a\", ARKIV_AUDIENCE, 3)\n+        assertEquals(5, calls.get())\n+\n+        now = now.plusSeconds(3539)\n+        check(\"/api/vedtak/42\", \"user-a\", VEDTAK_AUDIENCE, 1)\n+        check(\"/api/vedtak/42/dokumenter\", \"user-a\", ARKIV_AUDIENCE, 3)\n+        now = now.plusSeconds(1)\n+        check(\"/api/vedtak/42\", \"user-a\", VEDTAK_AUDIENCE, 6)\n+        check(\"/api/vedtak/42/dokumenter\", \"user-a\", ARKIV_AUDIENCE, 7)\n+        assertEquals(7, calls.get())\n+    }\n }\n*** End Patch\n"
```

</details>

```
Modified 1 file(s): <workspace>/src/test/kotlin/no/nav/benchmark/VedtakProxyTest.kt
```


---

<sub>3m 36s</sub>

### Info

Session limits: 22.96/30 AI credits used.

---

<sub>3m 45s</sub>

### `bash`

**Run cache and both-client integration regressions**

$ cd <workspace> && gradle test --no-daemon --max-workers=1 -Pkotlin.compiler.execution.strategy=in-process --console=plain

<details>
<summary>18 lines</summary>

```
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/b9f3fb9a97fab7017db27808cdf4dc1b -Djansi.tmpdir=~/Library/Caches/cplt/tmp/b9f3fb9a97fab7017db27808cdf4dc1b -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=54756 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=54756 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0
To honour the JVM settings for this build a single-use Daemon process will be forked. For more on this, please refer to https://docs.gradle.org/8.14.5/userguide/gradle_daemon.html#sec:disabling_the_daemon in the Gradle documentation.
Daemon will be stopped at the end of the build 
> Task :checkKotlinGradlePluginConfigurationErrors SKIPPED
> Task :compileKotlin UP-TO-DATE
> Task :compileJava NO-SOURCE
> Task :processResources NO-SOURCE
> Task :classes UP-TO-DATE
> Task :jar UP-TO-DATE
> Task :compileTestKotlin
> Task :compileTestJava NO-SOURCE
> Task :processTestResources NO-SOURCE
> Task :testClasses UP-TO-DATE

> Task :test
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/b9f3fb9a97fab7017db27808cdf4dc1b -Djansi.tmpdir=~/Library/Caches/cplt/tmp/b9f3fb9a97fab7017db27808cdf4dc1b -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=54756 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=54756 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0

BUILD SUCCESSFUL in 8s
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
<summary>57 lines</summary>

```
package no.nav.benchmark

import kotlinx.coroutines.CompletableDeferred
import java.security.MessageDigest
import java.time.Clock
import java.time.Duration
import java.util.HexFormat

/** Gjenbruker TokenX-tokener så vi ikke kaller Texas på hver forespørsel. */
class TokenCache(
    private val texas: TexasClient,
    private val clock: Clock,
    private val expiryLeeway: Duration = Duration.ofSeconds(30),
) {
    private data class Key(val tokenHash: String, val audience: String)

    private val lock = Any()
    private val tokens = mutableMapOf<Key, OboToken>()
    private val exchanges = mutableMapOf<Key, CompletableDeferred<OboToken>>()

    init {
        require(!expiryLeeway.isNegative) { "expiryLeeway must not be negative" }
    }

    suspend fun token(userToken: String, audience: String): String {
        val hash = MessageDigest.getInstance("SHA-256").digest(userToken.toByteArray(Charsets.UTF_8))
        val key = Key(HexFormat.of().formatHex(hash), audience)
        val (exchange, owner) = synchronized(lock) {
            val cached = tokens[key]
            if (cached != null && cached.isValid(clock.instant().plus(expiryLeeway))) {
                return cached.accessToken
            }
            tokens.remove(key)
            val pending = exchanges[key]
            if (pending != null) {
                pending to false
            } else {
                CompletableDeferred<OboToken>().also { exchanges[key] = it } to true
            }
        }
        if (!owner) return exchange.await().accessToken

        try {
            val fresh = texas.exchange(userToken, audience)
            if (!fresh.isValid(clock.instant().plus(expiryLeeway))) {
                throw TexasException("Texas returnerte et token med for kort levetid")
            }
            synchronized(lock) {
                tokens[key] = fresh
                exchange.complete(fresh)
                exchanges.remove(key)
            }
            return fresh.accessToken
        } catch (failure: Throwable) {
            // Alle ventere får samme feil, og neste forespørsel kan prøve igjen.
            synchronized(lock) {
                exchange.completeExceptionally(failure)
                exchanges.remove(key)
            }
            throw failure
        }
    }
}
```

</details>


---

<sub>3m 37s</sub>

### `bash`

**Check scoped diff whitespace**

$ cd <workspace> && git --no-pager --git-dir=~/.git --work-tree=~ diff --check -- nav-benchmark-attempts/r2/kotlin-token-cache-gpt-6.1-sol-1/workspace

```
fatal: this operation must be run in a work tree
<shellId: 5 completed with exit code 128>
```


---

<sub>4m 19s</sub>

### Copilot

**Rettet i den delte tokencachen**, for både `vedtak-api` og `arkiv-api`. Årsakene var:

- Cache-nøkkelen inneholdt bare audience. Dermed kunne én bruker få en annen brukers token. Nøkkelen bruker nå SHA-256 av hele innkommende token og audience.
- `expiryLeeway` var ikke koblet inn. Tokenet fornyes nå ved den konfigurerte marginen, og Texas-svartiden forlenger ikke beregnet levetid.
- Cache-oppslag og veksling var ikke koordinert. Samtidige kall med samme nøkkel deler nå én veksling, også ved feil. Feil og kansellering rydder opp slik at senere kall kan prøve igjen.

Hele innkommende token brukes fremfor bruker-ID for å unngå gjenbruk på tvers av ulike innlogginger eller rettigheter. Det gir flere cache-oppføringer ved tokenfornyelse. Nettverkskall skjer utenfor låsen, så ulike nøkler kan veksle samtidig.

**Sjekkliste for rød sone**

Tokenhåndtering er rød sone: forstå dette grundig.

- [ ] Gå gjennom isolasjonen mellom tokener og audiences.
- [ ] Gå gjennom utløpsberegningen og marginen.
- [ ] Gå gjennom deling av vekslinger og opprydding ved feil eller kansellering.

---

<sub>Generated by [GitHub Copilot CLI](https://github.com/features/copilot/cli)</sub>
