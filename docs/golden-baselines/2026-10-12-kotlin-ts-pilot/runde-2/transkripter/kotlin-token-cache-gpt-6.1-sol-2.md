# Copilot CLI Session

> [!NOTE]
> - **Session ID:** `49e6a2db-a545-4deb-bf7f-6a86e90a1e31`  
> - **Started:** 10/13/2026, 1:09:49 AM  
> - **Duration:** 5m 2s  
> - **Exported:** 10/13/2026, 1:14:51 AM  

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

<sub>8s</sub>

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
./build.gradle.kts
./settings.gradle.kts
./src/test/kotlin/no/nav/benchmark/VedtakProxyTest.kt
./docs/adr/0002-tokenx-via-texas.md
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
```

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

<sub>21s</sub>

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

<sub>32s</sub>

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

<sub>32s</sub>

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

<sub>32s</sub>

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

<sub>32s</sub>

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

<sub>32s</sub>

### `view`

**view the file at <workspace>/.github/instructions/kotlin.instructions.md.**

<workspace>/.github/instructions/kotlin.instructions.md (lines 1-280)

<details>
<summary>224 lines</summary>

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
````

</details>


---

<sub>32s</sub>

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

<sub>32s</sub>

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

<sub>32s</sub>

### `bash`

**Inspect worktree and available build tools**

$ git --no-pager status --short -- . && ls -a && command -v gradle && java -version

```
fatal: unable to get current working directory: Operation not permitted
<shellId: 0 completed with exit code 128>
```


---

<sub>51s</sub>

### Copilot

Cachen bruker bare audience som nøkkel, så brukere deler token. Den ignorerer også utløpsmarginen og lar samtidige forespørsler starte hver sin veksling. Jeg retter nøkkelen, kobler inn marginen og samordner vekslinger per brukertoken og audience.


---

<sub>51s</sub>

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

<sub>57s</sub>

### `view`

**view the file at <workspace>/.github/instructions/kotlin.instructions.md.**

<workspace>/.github/instructions/kotlin.instructions.md (lines 281-670)

<details>
<summary>315 lines</summary>

````
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
            header("Authorization", "******")
        }.andExpect { status { isOk() } }
    }
}

// Repository-only test — real database, no controllers
@DataJpaTest
@Testcontainers
class ResourceRepositorySliceTest {
````

</details>


---

<sub>57s</sub>

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

<sub>57s</sub>

### `bash`

**Find usable build environment and worktree state**

$ cd <workspace> && pwd && ls -a && git --no-pager status --short -- .; command -v gradle; java -version

<details>
<summary>15 lines</summary>

```
<workspace>
.
..
.github
build.gradle.kts
docs
settings.gradle.kts
src
fatal: unable to get current working directory: Operation not permitted
~/.local/share/mise/shims/gradle
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/43fd5aff2687d2a3193a02d77f5d9c45 -Djansi.tmpdir=~/Library/Caches/cplt/tmp/43fd5aff2687d2a3193a02d77f5d9c45 -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=58231 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=58231 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0
openjdk version "21.0.2" 2024-01-16
OpenJDK Runtime Environment (build 21.0.2+13-58)
OpenJDK 64-Bit Server VM (build 21.0.2+13-58, mixed mode, sharing)
<shellId: 1 completed with exit code 0>
```

</details>


---

<sub>1m 32s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Update File: <workspace>/src/main/kotlin/no/nav/benchmark/TokenCache.kt\n@@\n package no.nav.benchmark\n \n+import kotlinx.coroutines.sync.Mutex\n+import kotlinx.coroutines.sync.withLock\n+import java.security.MessageDigest\n import java.time.Clock\n+import java.time.Duration\n import java.util.concurrent.ConcurrentHashMap\n@@\n     private val texas: TexasClient,\n     private val clock: Clock,\n+    private val expiryLeeway: Duration = TokenConfig().expiryLeeway,\n ) {\n-    private val tokens = ConcurrentHashMap<String, OboToken>()\n+    private data class Key(val userTokenHash: String, val audience: String)\n+\n+    private class Entry {\n+        val mutex = Mutex()\n+        var token: OboToken? = null\n+    }\n+\n+    private val tokens = ConcurrentHashMap<Key, Entry>()\n+\n+    init {\n+        require(!expiryLeeway.isNegative) { \"expiryLeeway must not be negative\" }\n+    }\n \n     suspend fun token(userToken: String, audience: String): String {\n-        val cached = tokens[audience]\n-        if (cached != null && cached.isValid(clock.instant())) {\n-            return cached.accessToken\n+        val hash = MessageDigest.getInstance(\"SHA-256\")\n+            .digest(userToken.toByteArray(Charsets.UTF_8))\n+            .let { java.util.HexFormat.of().formatHex(it) }\n+        val entry = tokens.computeIfAbsent(Key(hash, audience)) { Entry() }\n+        return entry.mutex.withLock {\n+            val cached = entry.token\n+            if (cached != null && cached.isValid(clock.instant(), expiryLeeway)) {\n+                cached.accessToken\n+            } else {\n+                val fresh = texas.exchange(userToken, audience)\n+                entry.token = fresh\n+                fresh.accessToken\n+            }\n         }\n-        val fresh = texas.exchange(userToken, audience)\n-        tokens[audience] = fresh\n-        return fresh.accessToken\n     }\n }\n*** Update File: <workspace>/src/main/kotlin/no/nav/benchmark/Token.kt\n@@\n+import java.time.Duration\n import java.time.Instant\n \n data class OboToken(val accessToken: String, val expiresAt: Instant) {\n-    fun isValid(now: Instant): Boolean = now.isBefore(expiresAt)\n+    fun isValid(now: Instant, expiryLeeway: Duration = Duration.ZERO): Boolean =\n+        now.plus(expiryLeeway).isBefore(expiresAt)\n }\n*** Update File: <workspace>/src/main/kotlin/no/nav/benchmark/Application.kt\n@@\n-    val tokens = TokenCache(texas, clock)\n+    val tokens = TokenCache(texas, clock, config.expiryLeeway)\n*** Update File: <workspace>/src/main/kotlin/no/nav/benchmark/TexasClient.kt\n@@\n     private val endpoint: String = \"http://texas/api/v1/token/exchange\",\n ) {\n     suspend fun exchange(userToken: String, audience: String): OboToken {\n+        val requestedAt = clock.instant()\n         val response = http.submitForm(\n@@\n-        return OboToken(accessToken, clock.instant().plusSeconds(expiresIn))\n+        return OboToken(accessToken, requestedAt.plusSeconds(expiresIn))\n*** Update File: <workspace>/docs/adr/0002-tokenx-via-texas.md\n@@\n - Et vekslet token gjelder én bruker og én audience. Det skal aldri brukes for en\n   annen bruker eller mot et annet API.\n-- Tokener fornyes når tiden som er igjen før de utløper, er kortere enn\n+- Cache-nøkkelen består av SHA-256 av hele innkommende brukertoken og audience.\n+  Et nytt brukertoken får en egen oppføring, også for samme bruker. Selve\n+  brukertokenet lagres ikke i nøkkelen.\n+- Tokener fornyes når tiden som er igjen før de utløper, er mindre enn eller lik\n   `TokenConfig.expiryLeeway`. Det dekker klokkeforskjell og tiden et kall er underveis.\n+- Vi beregner utløpstiden fra starten av Texas-kallet, slik at svartiden ikke\n+  forlenger tokenets levetid i cachen.\n@@\n   samme bruker mot samme API skal derfor dele én veksling.\n+- En coroutine-mutex per cache-nøkkel beskytter sjekk og veksling. Andre nøkler\n+  kan veksle parallelt. Feil og kansellering frigjør låsen uten å lagre et nytt\n+  token, slik at neste forespørsel kan prøve på nytt.\n@@\n-Cachen vokser med antall aktive brukere. Det er akseptabelt med dagens volum.\n+Cachen og låsene vokser med antall brukertokener og audiences som er brukt i\n+podens levetid. Det er akseptabelt med dagens volum.\n*** End Patch\n"
```

</details>

```
Modified 5 file(s): <workspace>/src/main/kotlin/no/nav/benchmark/TokenCache.kt, <workspace>/src/main/kotlin/no/nav/benchmark/Token.kt, <workspace>/src/main/kotlin/no/nav/benchmark/Application.kt, <workspace>/src/main/kotlin/no/nav/benchmark/TexasClient.kt, <workspace>/docs/adr/0002-tokenx-via-texas.md
```


---

<sub>2m 41s</sub>

### Info

Session limits: 16.22/30 AI credits used.

---

<sub>2m 41s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Add File: <workspace>/src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt\n+package no.nav.benchmark\n+\n+import io.ktor.client.HttpClient\n+import io.ktor.client.engine.mock.MockEngine\n+import io.ktor.client.engine.mock.respond\n+import io.ktor.client.request.forms.FormDataContent\n+import io.ktor.http.HttpHeaders\n+import io.ktor.http.HttpStatusCode\n+import io.ktor.http.headersOf\n+import kotlinx.coroutines.CompletableDeferred\n+import kotlinx.coroutines.CoroutineStart\n+import kotlinx.coroutines.async\n+import kotlinx.coroutines.awaitAll\n+import kotlinx.coroutines.cancelAndJoin\n+import kotlinx.coroutines.runBlocking\n+import kotlinx.coroutines.withTimeout\n+import java.time.Clock\n+import java.time.Duration\n+import java.time.Instant\n+import java.time.ZoneId\n+import java.time.ZoneOffset\n+import java.util.concurrent.atomic.AtomicInteger\n+import kotlin.test.Test\n+import kotlin.test.assertEquals\n+import kotlin.test.assertFailsWith\n+import kotlin.test.assertIs\n+import kotlin.test.assertTrue\n+\n+class TokenCacheTest {\n+    private val json = headersOf(HttpHeaders.ContentType, \"application/json\")\n+    private val start = Instant.parse(\"2026-03-04T10:00:00Z\")\n+\n+    @Test\n+    fun `cache isolates users audiences and renewed input tokens`() = runBlocking {\n+        val calls = mutableListOf<Pair<String, String>>()\n+        val engine = MockEngine { request ->\n+            val form = assertIs<FormDataContent>(request.body).formData\n+            calls.add(form.getValue(\"user_token\") to form.getValue(\"target\"))\n+            respond(\"\"\"{\"access_token\":\"exchanged-${calls.size}\",\"expires_in\":3600}\"\"\", headers = json)\n+        }\n+        HttpClient(engine).use { http ->\n+            val clock = MutableClock(start)\n+            val cache = TokenCache(TexasClient(http, clock), clock)\n+            assertEquals(\"exchanged-1\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+            assertEquals(\"exchanged-2\", cache.token(\"user-b\", VEDTAK_AUDIENCE))\n+            assertEquals(\"exchanged-3\", cache.token(\"user-a\", ARKIV_AUDIENCE))\n+            assertEquals(\"exchanged-4\", cache.token(\"user-a-renewed\", VEDTAK_AUDIENCE))\n+            assertEquals(\"exchanged-1\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+            assertEquals(\"exchanged-2\", cache.token(\"user-b\", VEDTAK_AUDIENCE))\n+            assertEquals(\"exchanged-3\", cache.token(\"user-a\", ARKIV_AUDIENCE))\n+            assertEquals(\"exchanged-4\", cache.token(\"user-a-renewed\", VEDTAK_AUDIENCE))\n+            assertEquals(\n+                listOf(\n+                    \"user-a\" to VEDTAK_AUDIENCE,\n+                    \"user-b\" to VEDTAK_AUDIENCE,\n+                    \"user-a\" to ARKIV_AUDIENCE,\n+                    \"user-a-renewed\" to VEDTAK_AUDIENCE,\n+                ),\n+                calls,\n+            )\n+        }\n+    }\n+\n+    @Test\n+    fun `refreshes at leeway boundary and never reuses expired tokens`() = runBlocking {\n+        for (leeway in listOf(Duration.ZERO, Duration.ofSeconds(30), Duration.ofSeconds(120))) {\n+            for (age in listOf(3600L - leeway.seconds, 3600L, 3601L)) {\n+                val calls = AtomicInteger()\n+                val engine = MockEngine {\n+                    respond(\n+                        \"\"\"{\"access_token\":\"exchanged-${calls.incrementAndGet()}\",\"expires_in\":3600}\"\"\",\n+                        headers = json,\n+                    )\n+                }\n+                HttpClient(engine).use { http ->\n+                    val clock = MutableClock(start)\n+                    val cache = TokenCache(TexasClient(http, clock), clock, leeway)\n+                    assertEquals(\"exchanged-1\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+                    clock.now = start.plusSeconds(3600L - leeway.seconds).minusNanos(1)\n+                    assertEquals(\"exchanged-1\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+                    clock.now = start.plusSeconds(age)\n+                    assertEquals(\"exchanged-2\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+                    assertEquals(2, calls.get())\n+                }\n+            }\n+        }\n+    }\n+\n+    @Test\n+    fun `concurrent requests share one exchange on miss and refresh`() = runBlocking {\n+        withTimeout(5_000) {\n+            for (warmCache in listOf(false, true)) {\n+                val calls = AtomicInteger()\n+                val entered = CompletableDeferred<Unit>()\n+                val release = CompletableDeferred<Unit>()\n+                val engine = MockEngine {\n+                    val number = calls.incrementAndGet()\n+                    if (!warmCache || number > 1) {\n+                        entered.complete(Unit)\n+                        release.await()\n+                    }\n+                    respond(\"\"\"{\"access_token\":\"exchanged-$number\",\"expires_in\":3600}\"\"\", headers = json)\n+                }\n+                HttpClient(engine).use { http ->\n+                    val clock = MutableClock(start)\n+                    val cache = TokenCache(TexasClient(http, clock), clock)\n+                    if (warmCache) {\n+                        cache.token(\"user-a\", VEDTAK_AUDIENCE)\n+                        clock.now = start.plusSeconds(3570)\n+                    }\n+                    val requests = List(100) {\n+                        async(start = CoroutineStart.UNDISPATCHED) {\n+                            cache.token(\"user-a\", VEDTAK_AUDIENCE)\n+                        }\n+                    }\n+                    entered.await()\n+                    val expectedCalls = if (warmCache) 2 else 1\n+                    assertEquals(expectedCalls, calls.get())\n+                    release.complete(Unit)\n+                    assertEquals(List(100) { \"exchanged-$expectedCalls\" }, requests.awaitAll())\n+                    assertEquals(expectedCalls, calls.get())\n+                }\n+            }\n+        }\n+    }\n+\n+    @Test\n+    fun `different keys exchange while another key is blocked`() = runBlocking {\n+        withTimeout(5_000) {\n+            val entered = CompletableDeferred<Unit>()\n+            val release = CompletableDeferred<Unit>()\n+            val calls = AtomicInteger()\n+            val engine = MockEngine { request ->\n+                calls.incrementAndGet()\n+                val form = assertIs<FormDataContent>(request.body).formData\n+                if (form[\"user_token\"] == \"user-a\" && form[\"target\"] == VEDTAK_AUDIENCE) {\n+                    entered.complete(Unit)\n+                    release.await()\n+                }\n+                respond(\"\"\"{\"access_token\":\"exchanged\",\"expires_in\":3600}\"\"\", headers = json)\n+            }\n+            HttpClient(engine).use { http ->\n+                val clock = MutableClock(start)\n+                val cache = TokenCache(TexasClient(http, clock), clock)\n+                val blocked = async { cache.token(\"user-a\", VEDTAK_AUDIENCE) }\n+                entered.await()\n+                assertEquals(\"exchanged\", cache.token(\"user-b\", VEDTAK_AUDIENCE))\n+                assertEquals(\"exchanged\", cache.token(\"user-a\", ARKIV_AUDIENCE))\n+                assertEquals(3, calls.get())\n+                release.complete(Unit)\n+                assertEquals(\"exchanged\", blocked.await())\n+            }\n+        }\n+    }\n+\n+    @Test\n+    fun `failed refresh propagates and next request retries without stale fallback`() = runBlocking {\n+        val calls = AtomicInteger()\n+        val engine = MockEngine {\n+            val number = calls.incrementAndGet()\n+            if (number == 2) {\n+                respond(\"unavailable\", HttpStatusCode.ServiceUnavailable)\n+            } else {\n+                respond(\"\"\"{\"access_token\":\"exchanged-$number\",\"expires_in\":3600}\"\"\", headers = json)\n+            }\n+        }\n+        HttpClient(engine).use { http ->\n+            val clock = MutableClock(start)\n+            val cache = TokenCache(TexasClient(http, clock), clock)\n+            assertEquals(\"exchanged-1\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+            clock.now = start.plusSeconds(3570)\n+            val failure = assertFailsWith<TexasException> { cache.token(\"user-a\", VEDTAK_AUDIENCE) }\n+            assertEquals(\"Texas svarte 503\", failure.message)\n+            assertEquals(\"exchanged-3\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+            assertEquals(\"exchanged-3\", cache.token(\"user-a\", VEDTAK_AUDIENCE))\n+            assertEquals(3, calls.get())\n+        }\n+    }\n+\n+    @Test\n+    fun `cancelled exchange releases the key for waiting requests`() = runBlocking {\n+        withTimeout(5_000) {\n+            val calls = AtomicInteger()\n+            val entered = CompletableDeferred<Unit>()\n+            val engine = MockEngine {\n+                if (calls.incrementAndGet() == 1) {\n+                    entered.complete(Unit)\n+                    CompletableDeferred<Unit>().await()\n+                }\n+                respond(\"\"\"{\"access_token\":\"exchanged\",\"expires_in\":3600}\"\"\", headers = json)\n+            }\n+            HttpClient(engine).use { http ->\n+                val clock = MutableClock(start)\n+                val cache = TokenCache(TexasClient(http, clock), clock)\n+                val cancelled = async { cache.token(\"user-a\", VEDTAK_AUDIENCE) }\n+                entered.await()\n+                val waiting = async(start = CoroutineStart.UNDISPATCHED) {\n+                    cache.token(\"user-a\", VEDTAK_AUDIENCE)\n+                }\n+                cancelled.cancelAndJoin()\n+                assertTrue(cancelled.isCancelled)\n+                assertEquals(\"exchanged\", waiting.await())\n+                assertEquals(2, calls.get())\n+            }\n+        }\n+    }\n+\n+    @Test\n+    fun `Texas response latency does not extend token lifetime`() = runBlocking {\n+        val clock = MutableClock(start)\n+        val engine = MockEngine {\n+            clock.now = start.plusSeconds(10)\n+            respond(\"\"\"{\"access_token\":\"exchanged\",\"expires_in\":3600}\"\"\", headers = json)\n+        }\n+        HttpClient(engine).use { http ->\n+            assertEquals(start.plusSeconds(3600), TexasClient(http, clock).exchange(\"user-a\", VEDTAK_AUDIENCE).expiresAt)\n+        }\n+    }\n+\n+    private class MutableClock(@Volatile var now: Instant) : Clock() {\n+        override fun instant(): Instant = now\n+        override fun getZone(): ZoneId = ZoneOffset.UTC\n+        override fun withZone(zone: ZoneId): Clock = Clock.fixed(now, zone)\n+    }\n+}\n*** End Patch\n"
```

</details>

```
Added 1 file(s): <workspace>/src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt
```


---

<sub>3m 9s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Update File: <workspace>/src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt\n@@\n-            calls.add(form.getValue(\"user_token\") to form.getValue(\"target\"))\n+            calls.add(requireNotNull(form[\"user_token\"]) to requireNotNull(form[\"target\"]))\n*** Update File: <workspace>/src/test/kotlin/no/nav/benchmark/VedtakProxyTest.kt\n@@\n import io.ktor.client.request.bearerAuth\n+import io.ktor.client.request.forms.FormDataContent\n@@\n import java.time.Clock\n+import java.time.Duration\n import java.time.Instant\n+import java.time.ZoneId\n import java.time.ZoneOffset\n+import java.util.concurrent.atomic.AtomicInteger\n@@\n import kotlin.test.assertEquals\n+import kotlin.test.assertIs\n@@\n     fun avviserForespørselUtenToken() = testApplication {\n@@\n         assertEquals(HttpStatusCode.Unauthorized, client.get(\"/api/vedtak/42\").status)\n     }\n+\n+    @Test\n+    fun klienteneIsolererBrukereOgAudienceOgBrukerKonfigurertMargin() = testApplication {\n+        val start = Instant.parse(\"2026-03-04T10:00:00Z\")\n+        var now = start\n+        val clock = object : Clock() {\n+            override fun instant(): Instant = now\n+            override fun getZone(): ZoneId = ZoneOffset.UTC\n+            override fun withZone(zone: ZoneId): Clock = Clock.fixed(now, zone)\n+        }\n+        val calls = AtomicInteger()\n+        val texas = MockEngine { request ->\n+            assertEquals(\"http://texas/custom/exchange\", request.url.toString())\n+            val form = assertIs<FormDataContent>(request.body).formData\n+            val token = \"${form[\"user_token\"]}-${form[\"target\"]}-${calls.incrementAndGet()}\"\n+            respond(\"\"\"{\"access_token\":\"$token\",\"expires_in\":3600}\"\"\", headers = json)\n+        }\n+        val upstream = MockEngine { request ->\n+            respond(requireNotNull(request.headers[HttpHeaders.Authorization]), headers = json)\n+        }\n+        application {\n+            vedtakModule(\n+                texas, upstream, clock,\n+                config = TokenConfig(\n+                    texasEndpoint = \"http://texas/custom/exchange\",\n+                    expiryLeeway = Duration.ofSeconds(120),\n+                ),\n+            )\n+        }\n+\n+        suspend fun hent(path: String, userToken: String): String {\n+            val response = client.get(path) { bearerAuth(userToken) }\n+            assertEquals(HttpStatusCode.OK, response.status)\n+            return response.bodyAsText()\n+        }\n+\n+        assertEquals(\"Bearer user-a-$VEDTAK_AUDIENCE-1\", hent(\"/api/vedtak/42\", \"user-a\"))\n+        assertEquals(\"Bearer user-b-$VEDTAK_AUDIENCE-2\", hent(\"/api/vedtak/42\", \"user-b\"))\n+        assertEquals(\"Bearer user-a-$ARKIV_AUDIENCE-3\", hent(\"/api/vedtak/42/dokumenter\", \"user-a\"))\n+        assertEquals(\"Bearer user-b-$ARKIV_AUDIENCE-4\", hent(\"/api/vedtak/42/dokumenter\", \"user-b\"))\n+        assertEquals(\"Bearer user-a-$VEDTAK_AUDIENCE-1\", hent(\"/api/vedtak/42\", \"user-a\"))\n+        assertEquals(\"Bearer user-b-$ARKIV_AUDIENCE-4\", hent(\"/api/vedtak/42/dokumenter\", \"user-b\"))\n+        assertEquals(4, calls.get())\n+\n+        now = start.plusSeconds(3480)\n+        assertEquals(\"Bearer user-a-$VEDTAK_AUDIENCE-5\", hent(\"/api/vedtak/42\", \"user-a\"))\n+        assertEquals(\"Bearer user-a-$ARKIV_AUDIENCE-6\", hent(\"/api/vedtak/42/dokumenter\", \"user-a\"))\n+        assertEquals(6, calls.get())\n+    }\n }\n*** End Patch\n"
```

</details>

```
Modified 2 file(s): <workspace>/src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt, <workspace>/src/test/kotlin/no/nav/benchmark/VedtakProxyTest.kt
```


---

<sub>3m 27s</sub>

### `bash`

**Run Kotlin regression and proxy integration tests**

$ cd <workspace> && gradle --no-daemon --console=plain test

<details>
<summary>18 lines</summary>

```
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/43fd5aff2687d2a3193a02d77f5d9c45 -Djansi.tmpdir=~/Library/Caches/cplt/tmp/43fd5aff2687d2a3193a02d77f5d9c45 -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=58231 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=58231 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0
To honour the JVM settings for this build a single-use Daemon process will be forked. For more on this, please refer to https://docs.gradle.org/8.14.5/userguide/gradle_daemon.html#sec:disabling_the_daemon in the Gradle documentation.
Daemon will be stopped at the end of the build 
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
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/43fd5aff2687d2a3193a02d77f5d9c45 -Djansi.tmpdir=~/Library/Caches/cplt/tmp/43fd5aff2687d2a3193a02d77f5d9c45 -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=58231 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=58231 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0

BUILD SUCCESSFUL in 9s
4 actionable tasks: 4 executed
<shellId: 2 completed with exit code 0>
```

</details>


---

<sub>4m 16s</sub>

### Info

Response was interrupted due to a server error. Retrying...

---

<sub>4m 42s</sub>

### `rg`

**<testsuite**

"<testsuite" in *.xml (build/test-results/test)

```
build/test-results/test/TEST-no.nav.benchmark.VedtakProxyTest.xml:<testsuite name="no.nav.benchmark.VedtakProxyTest" tests="3" skipped="0" failures="0" errors="0" timestamp="2026-10-12T23:13:15.815Z" hostname="Mac.local" time="0.109">
build/test-results/test/TEST-no.nav.benchmark.TokenCacheTest.xml:<testsuite name="no.nav.benchmark.TokenCacheTest" tests="7" skipped="0" failures="0" errors="0" timestamp="2026-10-12T23:13:15.503Z" hostname="Mac.local" time="0.31">
```


---

<sub>4m 42s</sub>

### `view`

**view the file at <workspace>/src/main/kotlin/no/nav/benchmark/TokenCache.kt.**

<workspace>/src/main/kotlin/no/nav/benchmark/TokenCache.kt

<details>
<summary>39 lines</summary>

```
package no.nav.benchmark

import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import java.security.MessageDigest
import java.time.Clock
import java.time.Duration
import java.util.concurrent.ConcurrentHashMap

/** Gjenbruker TokenX-tokener så vi ikke kaller Texas på hver forespørsel. */
class TokenCache(
    private val texas: TexasClient,
    private val clock: Clock,
    private val expiryLeeway: Duration = TokenConfig().expiryLeeway,
) {
    private data class Key(val userTokenHash: String, val audience: String)

    private class Entry {
        val mutex = Mutex()
        var token: OboToken? = null
    }

    private val tokens = ConcurrentHashMap<Key, Entry>()

    init {
        require(!expiryLeeway.isNegative) { "expiryLeeway must not be negative" }
    }

    suspend fun token(userToken: String, audience: String): String {
        val hash = MessageDigest.getInstance("SHA-256")
            .digest(userToken.toByteArray(Charsets.UTF_8))
            .let { java.util.HexFormat.of().formatHex(it) }
        val entry = tokens.computeIfAbsent(Key(hash, audience)) { Entry() }
        return entry.mutex.withLock {
            val cached = entry.token
            if (cached != null && cached.isValid(clock.instant(), expiryLeeway)) {
                cached.accessToken
            } else {
                val fresh = texas.exchange(userToken, audience)
                entry.token = fresh
                fresh.accessToken
            }
        }
    }
}
```

</details>


---

<sub>4m 42s</sub>

### `view`

**view the file at <workspace>/docs/adr/0002-tokenx-via-texas.md.**

<workspace>/docs/adr/0002-tokenx-via-texas.md

<details>
<summary>27 lines</summary>

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
- Cache-nøkkelen består av SHA-256 av hele innkommende brukertoken og audience.
  Et nytt brukertoken får en egen oppføring, også for samme bruker. Selve
  brukertokenet lagres ikke i nøkkelen.
- Tokener fornyes når tiden som er igjen før de utløper, er mindre enn eller lik
  `TokenConfig.expiryLeeway`. Det dekker klokkeforskjell og tiden et kall er underveis.
- Vi beregner utløpstiden fra starten av Texas-kallet, slik at svartiden ikke
  forlenger tokenets levetid i cachen.
- Texas har en grense for antall vekslinger per sekund. Samtidige forespørsler fra
  samme bruker mot samme API skal derfor dele én veksling.
- En coroutine-mutex per cache-nøkkel beskytter sjekk og veksling. Andre nøkler
  kan veksle parallelt. Feil og kansellering frigjør låsen uten å lagre et nytt
  token, slik at neste forespørsel kan prøve på nytt.
- Tiden hentes fra en injisert `Clock`, så testene kan styre den.

## Konsekvenser

Cachen og låsene vokser med antall brukertokener og audiences som er brukt i
podens levetid. Det er akseptabelt med dagens volum.
```

</details>


---

<sub>5m 2s</sub>

### Copilot

**Rettet token-cachen for både `vedtak-api` og `arkiv-api`.** Årsakene var:
- Cachen var nøkkelt på bare audience, slik at ulike brukere fikk samme token.
- `expiryLeeway` var ikke koblet inn. Tokener kunne utløpe mens downstream-kallet var underveis.
- Cache-sjekk og veksling var ikke samordnet, så samtidige forespørsler startet hver sin Texas-veksling.

Nøkkelen består nå av SHA-256 av hele brukertokenet og audience. En coroutine-mutex per nøkkel samordner vekslinger uten å blokkere andre brukere, i stedet for én global lås. Kostnaden er flere cache-oppføringer og låser i minnet.

Utløpsmarginen brukes nå fra konfigurasjonen, og Texas-kallets svartid forlenger ikke beregnet levetid. Feil og kansellering frigjør låsen uten å lagre et nytt token. ADR-en er oppdatert.

🔴 Rød sone: token-isolasjon og gjenbruk, forstå dette grundig.

**Sjekkliste for rød sone**
- [ ] Gjennomgå nøkkelen for brukertoken og audience.
- [ ] Gjennomgå utløpsmargin og levetidsberegning.
- [ ] Gjennomgå låsing ved veksling, feil og kansellering.

Kan du forklare hvorfor audience alene ikke er en trygg cache-nøkkel? Still gjerne spørsmål om valgene over.

---

<sub>Generated by [GitHub Copilot CLI](https://github.com/features/copilot/cli)</sub>
