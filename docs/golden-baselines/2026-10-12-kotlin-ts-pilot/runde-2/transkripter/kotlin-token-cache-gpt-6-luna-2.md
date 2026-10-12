# Copilot CLI Session

> [!NOTE]
> - **Session ID:** `73812ef4-f2ad-4864-8d24-6bc07470b521`  
> - **Started:** 10/13/2026, 1:04:33 AM  
> - **Duration:** 2m 18s  
> - **Exported:** 10/13/2026, 1:06:52 AM  

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

<sub>5s</sub>

### `skill`

**security-owasp**

<details>
<summary>Arguments</summary>

```json
{
  "skill": "security-owasp"
}
```

</details>

```
Skill "security-owasp" loaded successfully. Follow the instructions in the skill context.
```


---

<sub>11s</sub>

### Copilot

Jeg sporer tokenutvekslingen fra innkommende kall til caching og videresending, og retter deretter den felles årsaken til 401-feil, tokenblanding og unødvendige samtidige vekslinger.

---

<sub>12s</sub>

### `bash`

**Check repository worktree**

$ git status --short

```
fatal: unable to get current working directory: Operation not permitted
<shellId: 0 completed with exit code 128>
```


---

<sub>12s</sub>

### `glob`

****/*.kt**

"**/*.kt"

<details>
<summary>8 matches</summary>

```
./src/main/kotlin/no/nav/benchmark/TokenCache.kt
./src/main/kotlin/no/nav/benchmark/VedtakClient.kt
./src/main/kotlin/no/nav/benchmark/TokenConfig.kt
./src/main/kotlin/no/nav/benchmark/Token.kt
./src/main/kotlin/no/nav/benchmark/TexasClient.kt
./src/main/kotlin/no/nav/benchmark/ArkivClient.kt
./src/main/kotlin/no/nav/benchmark/Application.kt
./src/test/kotlin/no/nav/benchmark/VedtakProxyTest.kt
```

</details>


---

<sub>12s</sub>

### `glob`

****/*{Token,token,Auth,auth}***

"**/*{Token,token,Auth,auth}*"

```
./src/main/kotlin/no/nav/benchmark/TokenCache.kt
./src/main/kotlin/no/nav/benchmark/TokenConfig.kt
./src/main/kotlin/no/nav/benchmark/Token.kt
./docs/adr/0002-tokenx-via-texas.md
```


---

<sub>12s</sub>

### `view`

**view the file at .github/instructions/kotlin.instructions.md.**

.github/instructions/kotlin.instructions.md

```
File too large to read at once (22.1 KB). Consider using the grep tool to search within the file, or view with view_range to read specific sections.
```


---

<sub>12s</sub>

### `view`

**view the file at .github/instructions/security-owasp.instructions.md.**

.github/instructions/security-owasp.instructions.md

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

<sub>17s</sub>

### `view`

**view the file at src/main/kotlin/no/nav/benchmark/TokenCache.kt.**

src/main/kotlin/no/nav/benchmark/TokenCache.kt

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

<sub>17s</sub>

### `view`

**view the file at src/main/kotlin/no/nav/benchmark/TexasClient.kt.**

src/main/kotlin/no/nav/benchmark/TexasClient.kt

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

<sub>17s</sub>

### `view`

**view the file at src/main/kotlin/no/nav/benchmark/Token.kt.**

src/main/kotlin/no/nav/benchmark/Token.kt

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

<sub>17s</sub>

### `view`

**view the file at src/main/kotlin/no/nav/benchmark/VedtakClient.kt.**

src/main/kotlin/no/nav/benchmark/VedtakClient.kt

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

<sub>17s</sub>

### `view`

**view the file at src/main/kotlin/no/nav/benchmark/TokenConfig.kt.**

src/main/kotlin/no/nav/benchmark/TokenConfig.kt

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

<sub>17s</sub>

### `view`

**view the file at src/main/kotlin/no/nav/benchmark/Application.kt.**

src/main/kotlin/no/nav/benchmark/Application.kt

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

<sub>18s</sub>

### `view`

**view the file at .github/instructions/kotlin.instructions.md.**

.github/instructions/kotlin.instructions.md (lines 1-180)

<details>
<summary>143 lines</summary>

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
````

</details>


---

<sub>18s</sub>

### `view`

**view the file at .github/instructions/security-core.instructions.md.**

.github/instructions/security-core.instructions.md

<details>
<summary>12 lines</summary>

```
---
applyTo: "**"
---

# Security Core

These rules apply to every change, in every language. They are never optional.

- **No secrets or personal data in logs.** Never log tokens, passwords, keys, fødselsnummer (FNR), name or address.
- **Secrets come from the environment.** Read tokens, passwords and keys from environment variables or the secret store. Never hardcode them.
- **Parameterized queries only.** Never concatenate user input into SQL, shell or any other command. Validate input at trust boundaries.
- **Verify resource ownership.** Check that the caller owns the resource, not only that the caller is authenticated.
- **Validate `azp` for machine-to-machine calls** against `AZURE_APP_PRE_AUTHORIZED_APPS`.
- **TLS 1.2 or newer.** Never set `InsecureSkipVerify: true` or disable certificate validation.

Code-level OWASP Top 10:2025 patterns: the `$security-owasp` skill.
```

</details>


---

<sub>21s</sub>

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

<sub>24s</sub>

### `view`

**view the file at src/test/kotlin/no/nav/benchmark/VedtakProxyTest.kt.**

src/test/kotlin/no/nav/benchmark/VedtakProxyTest.kt

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

<sub>24s</sub>

### `view`

**view the file at build.gradle.kts.**

build.gradle.kts

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

<sub>24s</sub>

### `view`

**view the file at docs/adr/0002-tokenx-via-texas.md.**

docs/adr/0002-tokenx-via-texas.md

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

<sub>24s</sub>

### `view`

**view the file at .github/instructions/testing.instructions.md.**

.github/instructions/testing.instructions.md

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

<sub>24s</sub>

### `view`

**view the file at .github/instructions/testing-kotlin.instructions.md.**

.github/instructions/testing-kotlin.instructions.md

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

<sub>24s</sub>

### `glob`

****/*Token*Test***

"**/*Token*Test*"

```
No files matched the pattern.
```


---

<sub>56s</sub>

### `bash`

**Check repository files and worktree**

$ cd <workspace> && pwd && git status --short && ls -la

```
<workspace>
fatal: unable to get current working directory: Operation not permitted
<shellId: 1 completed with exit code 128>
```


---

<sub>1m 17s</sub>

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

<sub>1m 29s</sub>

### Copilot

Årsaken er bekreftet: cachen bruker bare audience som nøkkel, ignorerer fornyelsesmarginen og slipper parallelle cache-miss rett gjennom til Texas. Jeg retter alle tre i token-cachen og legger til tester for brukerisolasjon, utløpsmargin og samtidige kall.

---

<sub>1m 30s</sub>

### `apply_patch`

<details>
<summary>Arguments</summary>

```json
"*** Begin Patch\n*** Update File: src/main/kotlin/no/nav/benchmark/TokenCache.kt\n@@\n import java.time.Clock\n+import java.time.Duration\n+import java.security.MessageDigest\n+import java.util.Base64\n import java.util.concurrent.ConcurrentHashMap\n+import kotlinx.coroutines.sync.Mutex\n \n /** Gjenbruker TokenX-tokener så vi ikke kaller Texas på hver forespørsel. */\n class TokenCache(\n     private val texas: TexasClient,\n     private val clock: Clock,\n+    private val expiryLeeway: Duration = TokenConfig().expiryLeeway,\n ) {\n-    private val tokens = ConcurrentHashMap<String, OboToken>()\n+    private val tokens = ConcurrentHashMap<CacheKey, OboToken>()\n+    private val locks = ConcurrentHashMap<CacheKey, Mutex>()\n \n     suspend fun token(userToken: String, audience: String): String {\n-        val cached = tokens[audience]\n-        if (cached != null && cached.isValid(clock.instant())) {\n-            return cached.accessToken\n+        val key = CacheKey(tokenHash(userToken), audience)\n+        val lock = locks.computeIfAbsent(key) { Mutex() }\n+        lock.lock()\n+        try {\n+            val cached = tokens[key]\n+            if (cached != null && cached.isValid(clock.instant(), expiryLeeway)) {\n+                return cached.accessToken\n+            }\n+            val fresh = texas.exchange(userToken, audience)\n+            tokens[key] = fresh\n+            return fresh.accessToken\n+        } finally {\n+            lock.unlock()\n         }\n-        val fresh = texas.exchange(userToken, audience)\n-        tokens[audience] = fresh\n-        return fresh.accessToken\n     }\n+\n+    private fun tokenHash(userToken: String): String =\n+        Base64.getEncoder().encodeToString(\n+            MessageDigest.getInstance(\"SHA-256\").digest(userToken.toByteArray(Charsets.UTF_8)),\n+        )\n+\n+    private data class CacheKey(val userTokenHash: String, val audience: String)\n }\n*** Update File: src/main/kotlin/no/nav/benchmark/Token.kt\n@@\n import java.time.Instant\n+import java.time.Duration\n \n data class OboToken(val accessToken: String, val expiresAt: Instant) {\n-    fun isValid(now: Instant): Boolean = now.isBefore(expiresAt)\n+    fun isValid(now: Instant, expiryLeeway: Duration = Duration.ZERO): Boolean =\n+        now.plus(expiryLeeway).isBefore(expiresAt)\n }\n*** Update File: src/main/kotlin/no/nav/benchmark/Application.kt\n@@\n-    val tokens = TokenCache(texas, clock)\n+    val tokens = TokenCache(texas, clock, config.expiryLeeway)\n*** Add File: src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt\n+package no.nav.benchmark\n+\n+import io.ktor.client.HttpClient\n+import io.ktor.client.engine.mock.MockEngine\n+import io.ktor.client.engine.mock.respond\n+import java.time.Clock\n+import java.time.Duration\n+import java.time.Instant\n+import java.time.ZoneOffset\n+import java.util.concurrent.atomic.AtomicInteger\n+import kotlinx.coroutines.CompletableDeferred\n+import kotlinx.coroutines.Dispatchers\n+import kotlinx.coroutines.async\n+import kotlinx.coroutines.awaitAll\n+import kotlinx.coroutines.runBlocking\n+import kotlin.test.Test\n+import kotlin.test.assertEquals\n+\n+class TokenCacheTest {\n+    private val now = Instant.parse(\"2026-03-04T10:00:00Z\")\n+    private val audience = \"dev-gcp:vedtak:vedtak-api\"\n+\n+    @Test\n+    fun `caches tokens separately for each user`() = runBlocking {\n+        val exchangeCount = AtomicInteger()\n+        val texas = TexasClient(\n+            HttpClient(MockEngine {\n+                val number = exchangeCount.incrementAndGet()\n+                respond(\"\"\"{\"access_token\":\"token-$number\",\"expires_in\":3600}\"\"\")\n+            }),\n+            Clock.fixed(now, ZoneOffset.UTC),\n+        )\n+        val cache = TokenCache(texas, Clock.fixed(now, ZoneOffset.UTC))\n+\n+        assertEquals(\"token-1\", cache.token(\"user-a\", audience))\n+        assertEquals(\"token-2\", cache.token(\"user-b\", audience))\n+        assertEquals(\"token-1\", cache.token(\"user-a\", audience))\n+        assertEquals(2, exchangeCount.get())\n+    }\n+\n+    @Test\n+    fun `refreshes token within the configured expiry leeway`() = runBlocking {\n+        val exchangeCount = AtomicInteger()\n+        val texas = TexasClient(\n+            HttpClient(MockEngine {\n+                val number = exchangeCount.incrementAndGet()\n+                respond(\"\"\"{\"access_token\":\"token-$number\",\"expires_in\":3600}\"\"\")\n+            }),\n+            Clock.fixed(now, ZoneOffset.UTC),\n+        )\n+        val cacheClock = Clock.fixed(now.plusSeconds(3590), ZoneOffset.UTC)\n+        val cache = TokenCache(texas, cacheClock, Duration.ofSeconds(30))\n+\n+        cache.token(\"user-a\", audience)\n+        assertEquals(\"token-2\", cache.token(\"user-a\", audience))\n+        assertEquals(2, exchangeCount.get())\n+    }\n+\n+    @Test\n+    fun `shares one exchange between concurrent requests for the same user`() = runBlocking {\n+        val exchangeCount = AtomicInteger()\n+        val exchangeStarted = CompletableDeferred<Unit>()\n+        val finishExchange = CompletableDeferred<Unit>()\n+        val texas = TexasClient(\n+            HttpClient(MockEngine {\n+                exchangeCount.incrementAndGet()\n+                exchangeStarted.complete(Unit)\n+                finishExchange.await()\n+                respond(\"\"\"{\"access_token\":\"shared-token\",\"expires_in\":3600}\"\"\")\n+            }),\n+            Clock.fixed(now, ZoneOffset.UTC),\n+        )\n+        val cache = TokenCache(texas, Clock.fixed(now, ZoneOffset.UTC))\n+        val requests = List(20) {\n+            async(Dispatchers.Default) { cache.token(\"user-a\", audience) }\n+        }\n+\n+        exchangeStarted.await()\n+        finishExchange.complete(Unit)\n+\n+        assertEquals(List(20) { \"shared-token\" }, requests.awaitAll())\n+        assertEquals(1, exchangeCount.get())\n+    }\n+}\n*** End Patch"
```

</details>

```
Added 1 file(s): <workspace>/src/test/kotlin/no/nav/benchmark/TokenCacheTest.kt
Modified 3 file(s): <workspace>/src/main/kotlin/no/nav/benchmark/TokenCache.kt, <workspace>/src/main/kotlin/no/nav/benchmark/Token.kt, <workspace>/src/main/kotlin/no/nav/benchmark/Application.kt
```


---

<sub>1m 33s</sub>

### `bash`

**Run Kotlin tests**

$ ./gradlew test

```
/bin/bash: ./gradlew: No such file or directory
<shellId: 2 completed with exit code 127>
```


---

<sub>1m 36s</sub>

### `bash`

**Locate Gradle build files and installed Gradle**

$ ls -la <workspace> && command -v gradle || true && find <workspace> -maxdepth 1 -type f -print

<details>
<summary>12 lines</summary>

```
total 16
drwxr-xr-x  7 user  staff  224 Oct 13 01:04 .
drwxr-xr-x  7 user  staff  224 Oct 13 01:04 ..
drwxr-xr-x  3 user  staff   96 Oct 13 01:04 .github
-rw-r--r--  1 user  staff  679 Oct 12 11:31 build.gradle.kts
drwxr-xr-x  3 user  staff   96 Oct 12 11:31 docs
-rw-r--r--  1 user  staff   34 Oct 12 11:31 settings.gradle.kts
drwxr-xr-x  4 user  staff  128 Oct 12 11:31 src
~/.local/share/mise/shims/gradle
<workspace>/build.gradle.kts
<workspace>/settings.gradle.kts
<shellId: 3 completed with exit code 0>
```

</details>


---

<sub>1m 50s</sub>

### `bash`

**Run Kotlin test suite**

$ gradle test

<details>
<summary>17 lines</summary>

```
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/24fa6c89a2fad6edeae67e1f3b196f55 -Djansi.tmpdir=~/Library/Caches/cplt/tmp/24fa6c89a2fad6edeae67e1f3b196f55 -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=56983 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=56983 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0
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
Picked up JAVA_TOOL_OPTIONS: -Djava.io.tmpdir=~/Library/Caches/cplt/tmp/24fa6c89a2fad6edeae67e1f3b196f55 -Djansi.tmpdir=~/Library/Caches/cplt/tmp/24fa6c89a2fad6edeae67e1f3b196f55 -Djava.rmi.server.hostname=localhost -Djava.net.preferIPv4Stack=true -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=56983 -Dhttp.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=56983 -Dhttps.nonProxyHosts=localhost|*.localhost|127.*|[::1]|::1|0.0.0.0

BUILD SUCCESSFUL in 9s
4 actionable tasks: 4 executed
<shellId: 4 completed with exit code 0>
```

</details>


---

<sub>2m 18s</sub>

### Copilot

Feilen lå i `TokenCache`: den brukte bare audience som nøkkel, brukte ikke `expiryLeeway` og lot samtidige cache-miss utløse flere Texas-vekslinger. Cachen skiller nå brukertoken og audience, samler samtidige kall per nøkkel og fornyer token før utløp.

SHA-256 gjør at cachen ikke behøver å lagre råtokenet som nøkkel, med en hash per oppslag som kostnad. En mutex per nøkkel hindrer parallelle vekslinger; `expiryLeeway` reduserer risikoen for 401 når tokenet nærmer seg utløp.

🔴 Rød sone: gå nøye gjennom tokenisolasjonen og fornyingen.

**Sjekkliste for rød sone**
- [ ] Kontroller at cachen isolerer tokens per brukertoken og audience.
- [ ] Kontroller at samtidige kall deler én veksling per nøkkel.
- [ ] Kontroller at `expiryLeeway` fornyer tokens før utløp.

Kan du forklare hvorfor cachen bruker både brukertoken og audience som nøkkel? Still gjerne spørsmål om valgene over.

---

<sub>Generated by [GitHub Copilot CLI](https://github.com/features/copilot/cli)</sub>
